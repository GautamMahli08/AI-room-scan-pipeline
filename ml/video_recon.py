"""Video tier: reconstruct a handheld video into a LiDAR-style export.

Input is an ordinary video (no depth, poses or intrinsics). Output is a
folder in the Stray Scanner layout that the Go pipeline already reads:

    camera_matrix.csv   intrinsics at the exported frame resolution
    odometry.csv        camera-to-world pose + intrinsics per frame
    depth/NNNNNN.png    uint16 depth, millimetres
    confidence/NNNNNN.png  0 / 1 / 2 from the model's depth confidence
    meta.json           tier, frame size, scale uncertainty, provenance

Steps (SYSTEM_DESIGN.md §5):
 1. frames: ffmpeg at 2x the target rate, keep the sharper of each pair
 2. orientation: rotate frames upright (raw sensor-orientation video is
    stored sideways); chosen with the metric depth model, since upright
    indoor views have the nearest surface (the floor) at the bottom
 3. poses + depth: VGGT on overlapping chunks of frames (as many as fit in
    GPU memory). Room scans are mostly panning in place, which defeats
    incremental SfM (no baseline: COLMAP registered 49 of 223 frames on
    single_room); VGGT predicts per-frame depth and poses jointly and does
    not need one. Chunks are chained by a similarity transform fitted on
    the frames they share (same pixels in both chunks give dense
    correspondences).
 4. metric scale: Depth Anything V2 Metric-Indoor against VGGT depth on
    confident pixels; the median ratio sets the scale, the spread across
    frames gives the scale uncertainty that widens every interval
 5. gravity: frames are upright, so "up" is image -y averaged over the
    walk, refined with the floor plane
 6. export

usage: python ml/video_recon.py --video in.mp4 --out export_dir
"""
import argparse
import json
import math
import shutil
import subprocess
import sys
import time
from pathlib import Path

import cv2
import numpy as np

from vggt_runner import VGGTRunner


def log(*a):
    print("[video]", *a, file=sys.stderr, flush=True)


# ---------------------------------------------------------------- frames

def extract_frames(video, work, fps, max_side):
    """Extract frames at 2*fps and keep the sharper of each consecutive
    pair. Returns [(path, timestamp)]."""
    raw = work / "raw"
    if raw.exists():
        shutil.rmtree(raw)
    raw.mkdir(parents=True)
    vf = f"fps={2 * fps},scale='if(gt(iw,ih),min({max_side},iw),-2)':'if(gt(iw,ih),-2,min({max_side},ih))'"
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", str(video), "-vf", vf, "-q:v", "2",
                    str(raw / "%06d.jpg")], check=True)
    files = sorted(raw.glob("*.jpg"))
    keep = work / "frames"
    if keep.exists():
        shutil.rmtree(keep)
    keep.mkdir()
    kept = []
    for i in range(0, len(files), 2):
        pair = files[i:i + 2]
        sharp = [cv2.Laplacian(cv2.imread(str(f), cv2.IMREAD_GRAYSCALE), cv2.CV_64F).var() for f in pair]
        best = pair[int(np.argmax(sharp))]
        dst = keep / f"{len(kept):06d}.jpg"
        shutil.move(str(best), dst)
        kept.append((dst, (int(best.stem) - 1) / (2 * fps)))
    shutil.rmtree(raw)
    return kept


ROT = {1: cv2.ROTATE_90_CLOCKWISE, 2: cv2.ROTATE_180, 3: cv2.ROTATE_90_COUNTERCLOCKWISE}


def rotate(img, r):
    return cv2.rotate(img, ROT[r]) if r else img


# ---------------------------------------------------------------- models

class MetricDepth:
    """Depth Anything V2 Metric-Indoor (metres)."""

    def __init__(self, name, device):
        import torch
        from transformers import AutoImageProcessor, AutoModelForDepthEstimation
        self.torch, self.device = torch, device
        self.proc = AutoImageProcessor.from_pretrained(name)
        self.model = AutoModelForDepthEstimation.from_pretrained(name).to(device).eval()
        if device == "cuda":
            self.model = self.model.half()

    def __call__(self, rgb):
        torch = self.torch
        inp = self.proc(images=rgb, return_tensors="pt").to(self.device)
        if self.device == "cuda":
            inp = {k: v.half() for k, v in inp.items()}
        with torch.no_grad():
            pred = self.model(**inp).predicted_depth
        pred = torch.nn.functional.interpolate(pred[:, None].float(), size=rgb.shape[:2], mode="bilinear",
                                               align_corners=False)
        return pred[0, 0].cpu().numpy()

    def unload(self):
        del self.model
        self.torch.cuda.empty_cache()


class DepthPro:
    """Apple Depth Pro: metric depth plus its own focal-length estimate.
    Metric depth from one image scales with the focal length the model
    assumes; on single_room it estimates ~2000 px for a true ~1600 px and
    reads 1.23-1.39x deep, but rescaled by (VGGT focal / its focal) it
    reads 0.97-1.12x LiDAR. VGGT's focal comes from many views at once."""

    def __init__(self, device):
        import torch
        from transformers import DepthProForDepthEstimation, DepthProImageProcessorFast
        self.torch, self.device = torch, device
        self.proc = DepthProImageProcessorFast.from_pretrained("apple/DepthPro-hf")
        dtype = torch.float16 if device == "cuda" else torch.float32
        self.model = DepthProForDepthEstimation.from_pretrained("apple/DepthPro-hf", dtype=dtype).to(device).eval()
        self.dtype = dtype

    def __call__(self, rgb):
        """Returns (depth in metres at the input size, focal in pixels at the input size)."""
        torch = self.torch
        inp = self.proc(images=rgb, return_tensors="pt").to(self.device)
        inp = {k: v.to(self.dtype) for k, v in inp.items()}
        with torch.no_grad():
            out = self.model(**inp)
        post = self.proc.post_process_depth_estimation(out, target_sizes=[rgb.shape[:2]])[0]
        return post["predicted_depth"].float().cpu().numpy(), float(post["focal_length"])

    def unload(self):
        del self.model
        self.torch.cuda.empty_cache()


def axis_line_lengths(gray):
    """Total length of straight segments within 3 degrees of the image's
    vertical and horizontal axes (LSD on a 640 px copy). In an upright
    indoor view vertical edges (wall corners, door frames) stay vertical
    while horizontal ones are tilted by perspective; sideways, the reverse."""
    s = 640 / max(gray.shape)
    g = cv2.resize(gray, None, fx=s, fy=s, interpolation=cv2.INTER_AREA)
    segs = cv2.createLineSegmentDetector().detect(g)[0]
    v = h = 0.0
    if segs is None:
        return v, h
    for x1, y1, x2, y2 in segs[:, 0]:
        length = math.hypot(x2 - x1, y2 - y1)
        if length < 30:
            continue
        ang = math.degrees(math.atan2(abs(y2 - y1), abs(x2 - x1)))  # 0 horizontal, 90 vertical
        if ang > 87:
            v += length
        elif ang < 3:
            h += length
    return v, h


def choose_rotation(frames, depth, n=12):
    """Quarter turns (clockwise) that make frames upright.

    Two cues: straight lines decide whether the image's vertical is its
    y or x axis (rotations {0,2} vs {1,3}); within that pair, depth
    ordering decides the sign, because an upright indoor view has the
    nearest surface (the floor) at the bottom. The depth scores of r and
    r+2 are near mirror images, so the sign is reliable even when the
    depth model is unsure about sideways input."""
    idx = np.linspace(0, len(frames) - 1, n).astype(int)
    vert, horiz, dep = 1.0, 1.0, np.zeros(4)
    for i in idx:
        bgr = cv2.imread(str(frames[i][0]))
        v, h = axis_line_lengths(cv2.cvtColor(bgr, cv2.COLOR_BGR2GRAY))
        vert, horiz = vert + v, horiz + h
        rgb = cv2.cvtColor(bgr, cv2.COLOR_BGR2RGB)
        for r in range(4):
            d = depth(rotate(rgb, r))
            k = d.shape[0] // 3
            dep[r] += np.log(np.median(d[:k]) / np.median(d[-k:]))
    line = math.log(vert / horiz)
    pair = (0, 2) if line > 0 else (1, 3)
    rot = pair[0] if dep[pair[0]] >= dep[pair[1]] else pair[1]
    return rot, line, dep / n


def has_orientation_tag(video):
    """True if the video carries a display-matrix rotation (phone camera
    apps write one; raw sensor exports such as Stray Scanner's do not)."""
    r = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
                        "stream_side_data=rotation:stream_tags=rotate", "-of", "default=nw=1", str(video)],
                       capture_output=True, text=True)
    return "rotation=" in r.stdout or "rotate=" in r.stdout


def vggt_size(w, h, long_side=518):
    """VGGT input size: long side 518, both sides multiples of 14."""
    s = long_side / max(w, h)
    return int(round(w * s / 14)) * 14, int(round(h * s / 14)) * 14


# ---------------------------------------------------------------- geometry

def unproject(depth, K, cam_from_world, step=4, mask=None):
    """World points of every step-th pixel (where mask is true)."""
    H, W = depth.shape
    v, u = np.mgrid[0:H:step, 0:W:step]
    z = depth[v, u]
    ok = z > 0
    if mask is not None:
        ok &= mask[v, u]
    u, v, z = u[ok], v[ok], z[ok]
    xc = np.stack([(u - K[0, 2]) * z / K[0, 0], (v - K[1, 2]) * z / K[1, 1], z], 1)
    R, t = cam_from_world[:, :3], cam_from_world[:, 3]
    return (xc - t) @ R  # R^T (x - t)


def umeyama(src, dst):
    """Similarity (s, R, t) minimising |dst - (s R src + t)|."""
    mu_s, mu_d = src.mean(0), dst.mean(0)
    xs, xd = src - mu_s, dst - mu_d
    U, S, Vt = np.linalg.svd(xd.T @ xs / len(src))
    D = np.eye(3)
    if np.linalg.det(U @ Vt) < 0:
        D[2, 2] = -1
    R = U @ D @ Vt
    s = np.trace(np.diag(S) @ D) / xs.var(0).sum()
    return s, R, mu_d - s * R @ mu_s


def robust_umeyama(src, dst, iters=3):
    """Umeyama with rounds of trimming the worst 20 % residuals."""
    keep = np.ones(len(src), bool)
    for _ in range(iters):
        s, R, t = umeyama(src[keep], dst[keep])
        r = np.linalg.norm(dst - (s * src @ R.T + t), axis=1)
        keep = r <= np.quantile(r, 0.8)
    return s, R, t


def rotation_to_y(up):
    y = np.array([0.0, 1.0, 0.0])
    v, c = np.cross(up, y), float(up @ y)
    if np.linalg.norm(v) < 1e-12:
        return np.eye(3) if c > 0 else np.diag([1.0, -1.0, -1.0])
    vx = np.array([[0, -v[2], v[1]], [v[2], 0, -v[0]], [-v[1], v[0], 0]])
    return np.eye(3) + vx + vx @ vx / (1 + c)


def floor_up(up, pts, cams):
    """Refine up with the floor: the RANSAC plane within 15° of up, at
    least 0.5 m below the median camera, with the most inliers."""
    rng = np.random.default_rng(0)
    low = pts[pts @ up < np.median(cams @ up) - 0.5]
    if len(low) < 100:
        return up
    if len(low) > 200000:
        low = low[rng.choice(len(low), 200000, replace=False)]
    best, best_in = up, 0
    for _ in range(500):
        s = low[rng.choice(len(low), 3, replace=False)]
        n = np.cross(s[1] - s[0], s[2] - s[0])
        if np.linalg.norm(n) < 1e-9:
            continue
        n /= np.linalg.norm(n)
        n = n if n @ up > 0 else -n
        if n @ up < math.cos(math.radians(15)):
            continue
        inl = int((np.abs((low - s[0]) @ n) < 0.03).sum())
        if inl > best_in:
            best, best_in = n, inl
    return best


def mat_to_quat(R):
    """(x, y, z, w) of a rotation matrix."""
    t = np.trace(R)
    if t > 0:
        s = math.sqrt(t + 1.0) * 2
        return ((R[2, 1] - R[1, 2]) / s, (R[0, 2] - R[2, 0]) / s, (R[1, 0] - R[0, 1]) / s, 0.25 * s)
    i = int(np.argmax(np.diag(R)))
    if i == 0:
        s = math.sqrt(1.0 + R[0, 0] - R[1, 1] - R[2, 2]) * 2
        return (0.25 * s, (R[0, 1] + R[1, 0]) / s, (R[0, 2] + R[2, 0]) / s, (R[2, 1] - R[1, 2]) / s)
    if i == 1:
        s = math.sqrt(1.0 + R[1, 1] - R[0, 0] - R[2, 2]) * 2
        return ((R[0, 1] + R[1, 0]) / s, 0.25 * s, (R[1, 2] + R[2, 1]) / s, (R[0, 2] - R[2, 0]) / s)
    s = math.sqrt(1.0 + R[2, 2] - R[0, 0] - R[1, 1]) * 2
    return ((R[0, 2] + R[2, 0]) / s, (R[1, 2] + R[2, 1]) / s, 0.25 * s, (R[1, 0] - R[0, 1]) / s)


# ---------------------------------------------------------------- reconstruction

def kabsch(src, dst):
    """Rigid (R, t) minimising |dst - (R src + t)|, trimming the worst 20 %
    of residuals twice."""
    keep = np.ones(len(src), bool)
    for _ in range(3):
        ms, md = src[keep].mean(0), dst[keep].mean(0)
        U, _, Vt = np.linalg.svd((dst[keep] - md).T @ (src[keep] - ms))
        D = np.diag([1.0, 1.0, np.sign(np.linalg.det(U @ Vt))])
        R = U @ D @ Vt
        t = md - R @ ms
        r = np.linalg.norm(dst - (src @ R.T + t), axis=1)
        keep = r <= np.quantile(r, 0.8)
    return R, t


def reconstruct(images, runner, metric, chunk, overlap):
    """VGGT on overlapping chunks of consecutive frames, chained by a
    similarity transform fitted on the shared frames (same pixels in both
    chunks). VGGT normalises scale per chunk (0.9-2.6x on single_room), so
    relative scale must come from the overlap; within a chunk its shape is
    accurate (2-5 cm over 40 cm of motion against ARKit). One global
    metric scale is then taken from focal-corrected Depth Pro over all
    frames that have it. Returns per-frame cam_from_world (3x4, metres),
    K, depth (metres), conf, and the per-frame metric ratios."""
    n = len(images)
    poses, Ks, depths, confs = [None] * n, [None] * n, [None] * n, [None] * n
    step = chunk - overlap
    start = 0
    while True:
        end = min(start + chunk, n)
        ids = list(range(start, end))
        ext, K, D, C = runner([images[i] for i in ids])
        if start == 0:
            S, Rg, tg = 1.0, np.eye(3), np.zeros(3)
        else:
            src, dst = [], []
            for j, i in enumerate(ids):
                if poses[i] is None:
                    continue
                mask = (C[j] > np.quantile(C[j], 0.5)) & (confs[i] > np.quantile(confs[i], 0.5))
                src.append(unproject(D[j], K[j], ext[j], step=8, mask=mask))
                dst.append(unproject(depths[i], Ks[i], poses[i], step=8, mask=mask))
            S, Rg, tg = robust_umeyama(np.concatenate(src), np.concatenate(dst))
        for j, i in enumerate(ids):
            if poses[i] is not None:
                continue  # shared frames keep the earlier chunk's estimate
            R, t = ext[j][:, :3], ext[j][:, 3]
            Rcw = Rg @ R.T
            Cw = S * (Rg @ (-R.T @ t)) + tg
            poses[i] = np.hstack([Rcw.T, (-Rcw.T @ Cw)[:, None]])
            Ks[i], depths[i], confs[i] = K[j], D[j] * S, C[j]
        if end == n:
            break
        start += step

    ratios = []
    for i, (dm, fm) in metric.items():
        dm = dm * (Ks[i][0, 0] / fm)  # metric depth at VGGT's focal
        ok = (confs[i] > np.quantile(confs[i], 0.5)) & (dm > 0.2) & (dm < 8)
        if ok.sum() > 500:
            ratios.append(np.median(dm[ok] / depths[i][ok]))
    ratios = np.array(ratios)
    m = float(np.median(ratios))
    for i in range(n):
        poses[i] = np.hstack([poses[i][:, :3], poses[i][:, 3:] * m])
        depths[i] = depths[i] * m
    return poses, Ks, depths, confs, ratios / m


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--video", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--fps", type=float, default=3.0)
    ap.add_argument("--chunk", type=int, default=8, help="frames per VGGT pass (8 fits a 4 GB GPU)")
    ap.add_argument("--overlap", type=int, default=3)
    ap.add_argument("--depth-model", default="depth-anything/Depth-Anything-V2-Metric-Indoor-Base-hf")
    ap.add_argument("--max-depth", type=float, default=6.0)
    ap.add_argument("--metric-every", type=int, default=4, help="run Depth Pro on every n-th frame")
    args = ap.parse_args()

    import torch
    device = "cuda" if torch.cuda.is_available() else "cpu"
    t0 = time.time()
    out = Path(args.out)
    work = out / "work"
    work.mkdir(parents=True, exist_ok=True)

    frames = extract_frames(Path(args.video), work, args.fps, 1280)
    log(f"frames: {len(frames)} at {args.fps} fps ({time.time() - t0:.0f}s)")

    metric = MetricDepth(args.depth_model, device)
    if has_orientation_tag(args.video):
        # The phone's camera app recorded which way up the video is, and
        # ffmpeg has already applied it: frames are upright. Image cues are
        # only a fallback (they fail on footage aimed at a tiled floor).
        rot, line = 0, float("nan")
        log("orientation: from the video's own rotation tag (frames already upright)")
    else:
        rot, line, dep = choose_rotation(frames, metric)
        log(f"orientation: {rot} quarter turn(s) clockwise (no rotation tag; line score {line:+.2f}: "
            f"{'upright axis' if line > 0 else 'sideways'}; depth scores {np.round(dep, 2)})")

    first = rotate(cv2.imread(str(frames[0][0])), rot)
    W, H = vggt_size(first.shape[1], first.shape[0])
    images = [cv2.cvtColor(cv2.resize(rotate(cv2.imread(str(f)), rot), (W, H), interpolation=cv2.INTER_AREA),
                           cv2.COLOR_BGR2RGB) for f, _ in frames]

    metric.unload()
    dp = DepthPro(device)
    metric_d = {i: dp(images[i]) for i in range(0, len(images), args.metric_every)}
    dp.unload()
    log(f"Depth Pro on {len(metric_d)} frames ({time.time() - t0:.0f}s)")

    runner = VGGTRunner(device)
    poses, Ks, depths, confs, ratios = reconstruct(images, runner, metric_d, args.chunk, args.overlap)
    s = 1.0  # poses and depth are already in metres
    spread = float(np.median(np.abs(np.log(ratios)))) * 1.4826
    # Per-frame metric noise averages over frames (neighbours correlated);
    # the residual bias of Depth Pro after focal correction (0.97-1.12x
    # LiDAR on single_room) does not.
    scale_sig = math.sqrt((spread / math.sqrt(max(1, len(ratios) / 3))) ** 2 + 0.06 ** 2)
    log(f"VGGT: {len(images)} frames; metric ratio spread across frames {spread * 100:.1f}%, "
        f"relative scale sigma {scale_sig * 100:.1f}% ({time.time() - t0:.0f}s)")

    # Gravity: frames are upright, so camera -y is up on average.
    Rcw = np.array([p[:, :3].T for p in poses])
    Ccw = np.array([-p[:, :3].T @ p[:, 3] for p in poses])
    up = -Rcw[:, :, 1].mean(0)
    up /= np.linalg.norm(up)
    pts = np.concatenate([unproject(depths[i] * s, Ks[i], np.hstack([poses[i][:, :3], poses[i][:, 3:] * s]), step=8,
                                    mask=confs[i] > np.quantile(confs[i], 0.5)) for i in range(0, len(poses), 2)])
    up = floor_up(up, pts, Ccw)
    Rw = rotation_to_y(up)
    # Cross-check: phones are held around 1.4 m above the floor.
    floor_h = np.percentile(pts @ up, 2)
    cam_h = float(np.median(Ccw @ up) - floor_h)
    log(f"camera height above floor {cam_h:.2f} m (expected ~1.2-1.6 m for a handheld phone)")

    # Export.
    for d in ("depth", "confidence"):
        (out / d).mkdir(exist_ok=True)
    rows = []
    for i, ((f, t), p, K, D, C) in enumerate(zip(frames, poses, Ks, depths, confs)):
        dm = D * s
        valid = (dm > 0.1) & (dm < args.max_depth)
        lo, hi = np.quantile(C, 0.3), np.quantile(C, 0.6)
        conf = np.where(C >= hi, 2, np.where(C >= lo, 1, 0)).astype(np.uint8) * valid
        cv2.imwrite(str(out / "depth" / f"{i:06d}.png"), np.where(valid, np.round(dm * 1000), 0).astype(np.uint16))
        cv2.imwrite(str(out / "confidence" / f"{i:06d}.png"), conf.astype(np.uint8))
        R = Rw @ p[:, :3].T
        Cw = Rw @ (-p[:, :3].T @ p[:, 3] * s)
        qx, qy, qz, qw = mat_to_quat(R)
        rows.append(f"{t:.6f}, {i:06d}, {Cw[0]:.6f}, {Cw[1]:.6f}, {Cw[2]:.6f}, {qx:.8f}, {qy:.8f}, {qz:.8f}, {qw:.8f}, "
                    f"{K[0, 0]:.4f}, {K[1, 1]:.4f}, {K[0, 2]:.4f}, {K[1, 2]:.4f}, , ")
    with open(out / "odometry.csv", "w", newline="\n") as fh:
        fh.write("timestamp, frame, x, y, z, qx, qy, qz, qw, fx, fy, cx, cy, distortion_center_x, distortion_center_y\n")
        fh.write("\n".join(rows) + "\n")
    K = np.median(np.array(Ks), axis=0)
    with open(out / "camera_matrix.csv", "w", newline="\n") as fh:
        fh.write(f"{K[0, 0]:.4f}, 0.0, {K[0, 2]:.4f}\n0.0, {K[1, 1]:.4f}, {K[1, 2]:.4f}\n0.0, 0.0, 1.0")
    meta = {
        "tier": "video", "rgb_width": W, "rgb_height": H, "frames": len(frames),
        "image_rotation_quarter_turns": rot, "orientation_line_score": None if math.isnan(line) else line, "scale_to_metres": s,
        "scale_rel_sigma": scale_sig, "scale_frame_spread": spread, "camera_height_m": cam_h,
        "models": [{"name": "facebook/VGGT-1B", "version": "hf"}, {"name": "apple/DepthPro-hf", "version": "hf"},
                   {"name": args.depth_model + " (orientation only)", "version": "hf"}],
        "runtime_s": round(time.time() - t0, 1),
    }
    json.dump(meta, open(out / "meta.json", "w"), indent=2)
    log(f"export written to {out} ({time.time() - t0:.0f}s)")


if __name__ == "__main__":
    main()
