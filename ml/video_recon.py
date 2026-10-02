"""Video tier: reconstruct a handheld video into a LiDAR-style export.

Input is an ordinary video (no depth, poses or intrinsics). Output is a
folder in the Stray Scanner layout that the Go pipeline already reads:

    camera_matrix.csv   intrinsics at the extracted frame resolution
    odometry.csv        camera-to-world pose + intrinsics per frame
    depth/NNNNNN.png    uint16 depth, millimetres
    confidence/NNNNNN.png
    meta.json           tier, frame size, scale uncertainty, provenance

Steps (SYSTEM_DESIGN.md §5):
 1. frames: ffmpeg at 2x the target rate, keep the sharper of each pair
 2. poses: COLMAP structure from motion (pycolmap), sequential matching;
    poses are up to an unknown scale
 3. depth: Depth Anything V2 Metric-Indoor per frame, run on upright frames
 4. scale: per frame, ratio of metric depth to SfM depth at the sparse
    points; the median ratio over all frames sets metric scale, its spread
    gives the scale uncertainty; each frame's depth is then aligned to the
    shared SfM geometry so frames fuse consistently
 5. gravity: the camera axis that stays most constant in world space is
    "up" in the image; refined by the floor plane
 6. export

usage: python ml/video_recon.py --video in.mp4 --out export_dir
"""
import argparse
import json
import math
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

import cv2
import numpy as np


def log(*a):
    print("[video]", *a, file=sys.stderr, flush=True)


# ---------------------------------------------------------------- frames

def extract_frames(video, out_dir, fps, max_side):
    """Extract frames at 2*fps, keep the sharper of each consecutive pair."""
    raw = out_dir / "raw"
    raw.mkdir(parents=True, exist_ok=True)
    vf = f"fps={2 * fps},scale='if(gt(iw,ih),min({max_side},iw),-2)':'if(gt(iw,ih),-2,min({max_side},ih))'"
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", str(video), "-vf", vf, "-q:v", "2",
                    str(raw / "%06d.jpg")], check=True)
    files = sorted(raw.glob("*.jpg"))
    keep = out_dir / "images"
    keep.mkdir(exist_ok=True)
    kept = []
    for i in range(0, len(files), 2):
        pair = files[i:i + 2]
        sharp = [cv2.Laplacian(cv2.imread(str(f), cv2.IMREAD_GRAYSCALE), cv2.CV_64F).var() for f in pair]
        best = pair[int(np.argmax(sharp))]
        t = (int(best.stem) - 1) / (2 * fps)
        name = f"{len(kept):06d}.jpg"
        shutil.copy(best, keep / name)
        kept.append((name, t))
    shutil.rmtree(raw)
    return keep, kept


# ---------------------------------------------------------------- SfM

def run_sfm(image_dir, work):
    import pycolmap

    db = work / "database.db"
    if db.exists():
        db.unlink()
    sparse = work / "sparse"
    sparse.mkdir(parents=True, exist_ok=True)
    reader = pycolmap.ImageReaderOptions()
    reader.camera_model = "SIMPLE_RADIAL"
    pycolmap.extract_features(db, image_dir, camera_mode=pycolmap.CameraMode.SINGLE, reader_options=reader)
    seq = pycolmap.SequentialMatchingOptions()
    seq.overlap = 12
    seq.quadratic_overlap = True
    pycolmap.match_sequential(db, matching_options=seq)
    maps = pycolmap.incremental_mapping(db, image_dir, sparse)
    if not maps:
        raise SystemExit("SfM failed: no model reconstructed")
    rec = max(maps.values(), key=lambda r: r.num_reg_images())
    log(f"SfM: {rec.num_reg_images()} of {len(list(image_dir.glob('*.jpg')))} frames registered, "
        f"{rec.num_points3D()} points, {len(maps)} model(s)")
    return rec


# ---------------------------------------------------------------- depth

class DepthModel:
    def __init__(self, name, device):
        import torch
        from transformers import AutoImageProcessor, AutoModelForDepthEstimation

        self.torch = torch
        self.device = device
        self.proc = AutoImageProcessor.from_pretrained(name)
        self.model = AutoModelForDepthEstimation.from_pretrained(name).to(device).eval()
        if device == "cuda":
            self.model = self.model.half()

    def __call__(self, rgb):
        """Metric depth (metres) at the input image's resolution."""
        torch = self.torch
        inp = self.proc(images=rgb, return_tensors="pt").to(self.device)
        if self.device == "cuda":
            inp = {k: v.half() for k, v in inp.items()}
        with torch.no_grad():
            pred = self.model(**inp).predicted_depth
        pred = torch.nn.functional.interpolate(pred[:, None].float(), size=rgb.shape[:2], mode="bilinear",
                                               align_corners=False)
        return pred[0, 0].cpu().numpy()


def upright_rotation(up_cam):
    """Number of 90° clockwise rotations (cv2) that put the image's up
    direction at the top. up_cam is world-up expressed in camera axes
    (x right, y down)."""
    ux, uy = up_cam[0], up_cam[1]
    if abs(uy) >= abs(ux):
        return 0 if uy < 0 else 2          # up is -y (normal) or +y (upside down)
    return 1 if ux < 0 else 3              # up is -x (left): rotate clockwise once; +x: three times


ROT = {1: cv2.ROTATE_90_CLOCKWISE, 2: cv2.ROTATE_180, 3: cv2.ROTATE_90_COUNTERCLOCKWISE}
UNROT = {1: cv2.ROTATE_90_COUNTERCLOCKWISE, 2: cv2.ROTATE_180, 3: cv2.ROTATE_90_CLOCKWISE}


# ---------------------------------------------------------------- gravity

def estimate_up(Rcw, pts_world, cams, unit):
    """World up direction. Start from the camera image axis whose world
    direction is most constant over the walk (the phone is held the same
    way up), then refine with the dominant plane whose normal is close to
    it and which lies at least 0.5 m below the cameras (the floor). unit is
    an estimate of SfM units per metre.

    The sign assumes the constant image axis points down in the world: true
    for upright video (+y down) and for raw sensor-orientation video held
    in portrait as recorded by capture apps (+x down)."""
    best = None
    for k in (0, 1):
        axes = Rcw[:, :, k]
        m = axes.mean(0)
        if best is None or np.linalg.norm(m) > np.linalg.norm(best[1]):
            best = (k, m)
    k, m = best
    up = -m / np.linalg.norm(m)   # the image axis points down (y) or sideways-down (x)

    # Floor refinement: RANSAC planes whose normal is within 15° of up.
    rng = np.random.default_rng(0)
    P = pts_world
    if len(P) > 200000:
        P = P[rng.choice(len(P), 200000, replace=False)]
    h = P @ up
    cam_h = np.median(cams @ up)
    low = P[h < cam_h - 0.5 * unit]
    best_n, best_in = up, 0
    for _ in range(400):
        s = low[rng.choice(len(low), 3, replace=False)]
        n = np.cross(s[1] - s[0], s[2] - s[0])
        if np.linalg.norm(n) < 1e-9:
            continue
        n /= np.linalg.norm(n)
        if n @ up < 0:
            n = -n
        if n @ up < math.cos(math.radians(15)):
            continue
        d = np.abs((low - s[0]) @ n)
        inl = int((d < 0.03 * unit).sum())
        if inl > best_in:
            best_n, best_in = n, inl
    return best_n


def rotation_to_y(up):
    """Rotation matrix taking `up` to +Y."""
    y = np.array([0.0, 1.0, 0.0])
    v = np.cross(up, y)
    c = float(up @ y)
    if np.linalg.norm(v) < 1e-12:
        return np.eye(3) if c > 0 else np.diag([1.0, -1.0, -1.0])
    vx = np.array([[0, -v[2], v[1]], [v[2], 0, -v[0]], [-v[1], v[0], 0]])
    return np.eye(3) + vx + vx @ vx * (1 / (1 + c))


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


# ---------------------------------------------------------------- main

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--video", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--fps", type=float, default=3.0)
    ap.add_argument("--max-side", type=int, default=1280)
    ap.add_argument("--depth-model", default="depth-anything/Depth-Anything-V2-Metric-Indoor-Base-hf")
    ap.add_argument("--depth-width", type=int, default=256, help="long side of the exported depth maps")
    ap.add_argument("--max-depth", type=float, default=6.0)
    args = ap.parse_args()

    import torch
    device = "cuda" if torch.cuda.is_available() else "cpu"
    t0 = time.time()
    out = Path(args.out)
    work = out / "work"
    work.mkdir(parents=True, exist_ok=True)

    image_dir, kept = extract_frames(Path(args.video), work, args.fps, args.max_side)
    log(f"frames: {len(kept)} kept at {args.fps} fps ({time.time() - t0:.0f}s)")

    rec = run_sfm(image_dir, work)
    log(f"SfM done ({time.time() - t0:.0f}s)")
    cam = next(iter(rec.cameras.values()))
    W, H = cam.width, cam.height
    f, cx, cy = cam.params[0], cam.params[1], cam.params[2]

    images = sorted(rec.images.values(), key=lambda im: im.name)
    Rcw, Ccw, sparse = [], [], []
    for im in images:
        T = im.cam_from_world
        R = T.rotation.matrix()
        t = np.asarray(T.translation)
        Rcw.append(R.T)
        Ccw.append(-R.T @ t)
        uv, z = [], []
        for p2 in im.points2D:
            if p2.has_point3D():
                X = rec.points3D[p2.point3D_id].xyz
                d = (R @ X + t)[2]
                if d > 0:
                    uv.append(p2.xy)
                    z.append(d)
        sparse.append((np.array(uv).reshape(-1, 2), np.array(z)))
    Rcw, Ccw = np.array(Rcw), np.array(Ccw)
    pts = np.array([p.xyz for p in rec.points3D.values()])

    # Gravity from camera axes and the floor (in SfM units).
    unit = np.median(np.concatenate([z for _, z in sparse if len(z)])) / 2.0  # indoor views are ~2 m deep
    up = estimate_up(Rcw, pts, Ccw, unit)
    Rw = rotation_to_y(up)
    up_cam = np.median(np.einsum("nji,j->ni", Rcw, up), axis=0)  # world up in camera axes
    rot = upright_rotation(up_cam)
    log(f"gravity: up in camera axes {np.round(up_cam, 2)}, image rotation {rot}x90°")

    # Metric depth per frame, aligned to the SfM geometry.
    model = DepthModel(args.depth_model, device)
    ratios, depths = [], []
    for im, (uv, z) in zip(images, sparse):
        rgb = cv2.cvtColor(cv2.imread(str(image_dir / im.name)), cv2.COLOR_BGR2RGB)
        up_img = cv2.rotate(rgb, ROT[rot]) if rot else rgb
        d = model(up_img)
        d = cv2.rotate(d, UNROT[rot]) if rot else d
        depths.append(d.astype(np.float32))
        if len(z) >= 20:
            u = np.clip(uv[:, 0].astype(int), 0, W - 1)
            v = np.clip(uv[:, 1].astype(int), 0, H - 1)
            r = d[v, u] / z
            r = r[np.isfinite(r) & (r > 0)]
            ratios.append(float(np.median(r)) if len(r) >= 20 else np.nan)
        else:
            ratios.append(np.nan)
    ratios = np.array(ratios)
    good = np.isfinite(ratios)
    s = float(np.median(ratios[good]))  # SfM units -> metres
    logr = np.log(ratios[good] / s)
    mad = float(np.median(np.abs(logr))) * 1.4826
    n_eff = max(1.0, good.sum() / 5.0)  # neighbouring frames are correlated
    scale_sig = math.sqrt((mad / math.sqrt(n_eff)) ** 2 + 0.02 ** 2)  # + 2 % model bias floor until calibrated
    log(f"scale: {s:.4f} m per SfM unit from {good.sum()} frames, per-frame spread {mad*100:.1f}%, "
        f"relative sigma {scale_sig*100:.1f}% ({time.time() - t0:.0f}s)")

    # Export in the Stray Scanner layout.
    (out / "depth").mkdir(exist_ok=True)
    (out / "confidence").mkdir(exist_ok=True)
    dw = args.depth_width
    dh = int(round(dw * H / W)) if W >= H else args.depth_width
    if H > W:
        dw = int(round(args.depth_width * W / H))
    rows = []
    for i, (im, d, r) in enumerate(zip(images, depths, ratios)):
        k = s / r if np.isfinite(r) else 1.0  # align this frame's depth to the shared geometry
        dm = cv2.resize(d * k, (dw, dh), interpolation=cv2.INTER_NEAREST)
        valid = (dm > 0.1) & (dm < args.max_depth)
        mm = np.where(valid, np.round(dm * 1000), 0).astype(np.uint16)
        conf = np.where(valid, 2, 0).astype(np.uint8)
        if not np.isfinite(r):
            conf[:] = 0  # depth not tied to the geometry: excluded from fusion
        cv2.imwrite(str(out / "depth" / f"{i:06d}.png"), mm)
        cv2.imwrite(str(out / "confidence" / f"{i:06d}.png"), conf)
        R = Rw @ Rcw[i]
        C = Rw @ (Ccw[i] * s)
        qx, qy, qz, qw = mat_to_quat(R)
        t = dict(kept)[im.name]
        rows.append(f"{t:.6f}, {i:06d}, {C[0]:.6f}, {C[1]:.6f}, {C[2]:.6f}, {qx:.8f}, {qy:.8f}, {qz:.8f}, {qw:.8f}, "
                    f"{f:.4f}, {f:.4f}, {cx:.4f}, {cy:.4f}, , ")
    with open(out / "odometry.csv", "w", newline="\n") as fh:
        fh.write("timestamp, frame, x, y, z, qx, qy, qz, qw, fx, fy, cx, cy, distortion_center_x, distortion_center_y\n")
        fh.write("\n".join(rows) + "\n")
    with open(out / "camera_matrix.csv", "w", newline="\n") as fh:
        fh.write(f"{f:.4f}, 0.0, {cx:.4f}\n0.0, {f:.4f}, {cy:.4f}\n0.0, 0.0, 1.0")
    meta = {
        "tier": "video", "rgb_width": W, "rgb_height": H,
        "frames_extracted": len(kept), "frames_registered": len(images),
        "scale_m_per_unit": s, "scale_rel_sigma": scale_sig, "scale_frame_spread": mad,
        "image_rotation_quarter_turns": rot,
        "models": [{"name": args.depth_model, "version": "hf"}, {"name": "pycolmap", "version": __import__("pycolmap").__version__}],
        "runtime_s": round(time.time() - t0, 1),
    }
    json.dump(meta, open(out / "meta.json", "w"), indent=2)
    log(f"export written to {out} ({time.time() - t0:.0f}s)")


if __name__ == "__main__":
    main()
