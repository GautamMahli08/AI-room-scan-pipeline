"""Photo tier: per-room photo folders -> one stitched, LiDAR-style export.

Input: a folder of room folders, each with 2-8 stills (JPEG/HEIC, any
orientation, no poses or depth). Capture protocol: include one photo
through each doorway, taken from both sides.

Steps (SYSTEM_DESIGN.md §6):
 1. load: EXIF orientation applied; 35 mm-equivalent focal read if present
 2. orientation: photos without EXIF orientation (frames from raw video)
    are rotated upright with the same line + depth cue as the video tier
 3. per room: one VGGT pass over the room's photos (VGGT's design point:
    a handful of views); metric scale from Depth Pro rescaled by the focal
    (EXIF if known, else VGGT's); gravity from the cameras and the floor
 4. links: SIFT + fundamental-matrix inliers between photos of different
    rooms find the doorway photos; for each linked pair of rooms VGGT is
    run on both rooms' photos together, which ties the two room frames
    (gravity is shared, so the link is yaw + translation)
 5. layout: maximum spanning tree over links (weight = inliers), rooms
    placed from the best-connected one; rooms with no link are reported
    as unplaced instead of guessed
 6. export one combined capture (each room's photos get their own time
    block, so the Go drift stage treats rooms as chunks)

usage: python ml/photo_recon.py --photo <rooms_dir> --out <export_dir>
"""
import argparse
import json
import math
import shutil
import sys
import time
from pathlib import Path

import cv2
import numpy as np

from video_recon import (DepthPro, MetricDepth, axis_line_lengths, floor_up, mat_to_quat, rotate,
                         rotation_to_y, unproject, umeyama, vggt_size)
from vggt_runner import VGGTRunner

IMG_EXT = {".jpg", ".jpeg", ".png", ".heic", ".heif"}


def log(*a):
    print("[photo]", *a, file=sys.stderr, flush=True)


def load_photo(path, max_side=1280):
    """RGB uint8 with EXIF orientation applied, plus the 35 mm-equivalent
    focal length (or None) and whether EXIF orientation was present."""
    from PIL import Image, ImageOps
    if path.suffix.lower() in {".heic", ".heif"}:
        import pillow_heif
        pillow_heif.register_heif_opener()
    im = Image.open(path)
    exif = im.getexif()
    has_orient = 0x0112 in exif
    f35 = None
    try:
        f35 = exif.get_ifd(0x8769).get(0xA405)  # FocalLengthIn35mmFilm
    except Exception:
        pass
    im = ImageOps.exif_transpose(im).convert("RGB")
    s = max_side / max(im.size)
    if s < 1:
        im = im.resize((round(im.size[0] * s), round(im.size[1] * s)), Image.LANCZOS)
    return np.asarray(im), (float(f35) if f35 else None), has_orient


def detect_rotation(images, depth):
    """Same cue as the video tier, over all photos without EXIF
    orientation: lines pick the vertical axis, depth ordering the sign."""
    vert = horiz = 1.0
    dep = np.zeros(4)
    for rgb in images:
        v, h = axis_line_lengths(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY))
        vert, horiz = vert + v, horiz + h
        for r in range(4):
            d = depth(rotate(rgb, r))
            k = d.shape[0] // 3
            dep[r] += np.log(np.median(d[:k]) / np.median(d[-k:]))
    pair = (0, 2) if vert > horiz else (1, 3)
    return pair[0] if dep[pair[0]] >= dep[pair[1]] else pair[1]


def to_vggt(rgb):
    W, H = vggt_size(rgb.shape[1], rgb.shape[0])
    return cv2.resize(rgb, (W, H), interpolation=cv2.INTER_AREA)


def run_group(runner, imgs):
    """VGGT on photos that may differ in size: each is padded to the
    group's largest VGGT size (white, as VGGT's pad mode) and cropped back."""
    W = max(i.shape[1] for i in imgs)
    H = max(i.shape[0] for i in imgs)
    side = max(W, H)
    padded, offs = [], []
    for im in imgs:
        py, px = (side - im.shape[0]) // 2, (side - im.shape[1]) // 2
        canvas = np.full((side, side, 3), 255, np.uint8)
        canvas[py:py + im.shape[0], px:px + im.shape[1]] = im
        padded.append(canvas)
        offs.append((py, px))
    ext, K, D, C = runner(padded)
    Ds, Cs, Kc = [], [], K.copy()
    for k, (im, (py, px)) in enumerate(zip(imgs, offs)):
        Ds.append(D[k][py:py + im.shape[0], px:px + im.shape[1]])
        Cs.append(C[k][py:py + im.shape[0], px:px + im.shape[1]])
        Kc[k, 0, 2] -= px
        Kc[k, 1, 2] -= py
    return ext, Kc, Ds, Cs


def metric_scale(Ds, Cs, Ks, metric, focal35, sizes):
    """Median ratio of focal-corrected Depth Pro depth to VGGT depth."""
    ratios = []
    for k, (D, C, K, (dm, fm)) in enumerate(zip(Ds, Cs, Ks, metric)):
        f = K[0, 0]
        if focal35[k]:  # EXIF: true focal in pixels from the 35 mm equivalent (36x24 mm frame diagonal)
            h, w = sizes[k]
            f = focal35[k] / math.hypot(36, 24) * math.hypot(w, h)
            f *= D.shape[1] / w
        dmc = dm * (f / fm)
        ok = (C > np.quantile(C, 0.5)) & (dmc > 0.2) & (dmc < 8)
        if ok.sum() > 300:
            ratios.append(np.median(dmc[ok] / D[ok]))
    return np.array(ratios)


def sift_inliers(a, b, sift):
    ka, da = sift.detectAndCompute(cv2.cvtColor(a, cv2.COLOR_RGB2GRAY), None)
    kb, db = sift.detectAndCompute(cv2.cvtColor(b, cv2.COLOR_RGB2GRAY), None)
    if da is None or db is None or len(ka) < 20 or len(kb) < 20:
        return 0
    m = cv2.BFMatcher().knnMatch(da, db, k=2)
    good = [x for x, y in (p for p in m if len(p) == 2) if x.distance < 0.75 * y.distance]
    if len(good) < 15:
        return 0
    pa = np.float32([ka[g.queryIdx].pt for g in good])
    pb = np.float32([kb[g.trainIdx].pt for g in good])
    _, mask = cv2.findFundamentalMat(pa, pb, cv2.FM_RANSAC, 2.0, 0.999)
    return int(mask.sum()) if mask is not None else 0


def yaw_translation(src, dst):
    """Gravity-preserving (yaw about +Y, translation) fit of dst ~ R src + t."""
    ms, md = src.mean(0), dst.mean(0)
    a, b = src - ms, dst - md
    num = np.sum(a[:, 2] * b[:, 0] - a[:, 0] * b[:, 2])
    den = np.sum(a[:, 0] * b[:, 0] + a[:, 2] * b[:, 2])
    th = math.atan2(num, den)
    c, s = math.cos(th), math.sin(th)
    R = np.array([[c, 0, s], [0, 1, 0], [-s, 0, c]])
    return R, md - R @ ms


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--photo", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--min-inliers", type=int, default=40)
    ap.add_argument("--max-depth", type=float, default=6.0)
    args = ap.parse_args()
    import torch
    device = "cuda" if torch.cuda.is_available() else "cpu"
    t0 = time.time()
    out = Path(args.out)
    if out.exists():
        shutil.rmtree(out)
    (out / "depth").mkdir(parents=True)
    (out / "confidence").mkdir()

    room_dirs = sorted(d for d in Path(args.photo).iterdir() if d.is_dir())
    rooms = []
    for d in room_dirs:
        files = sorted(f for f in d.iterdir() if f.suffix.lower() in IMG_EXT)
        if len(files) < 2:
            log(f"{d.name}: {len(files)} photo(s), need at least 2; skipped")
            continue
        photos = [load_photo(f) for f in files]
        rooms.append({"name": d.name, "files": [f.name for f in files], "rgb": [p[0] for p in photos],
                      "f35": [p[1] for p in photos], "exif_orient": [p[2] for p in photos]})
    log(f"{len(rooms)} rooms, {sum(len(r['rgb']) for r in rooms)} photos")

    # Orientation for photos that carry no EXIF orientation.
    da = MetricDepth("depth-anything/Depth-Anything-V2-Metric-Indoor-Base-hf", device)
    loose = [im for r in rooms for im, o in zip(r["rgb"], r["exif_orient"]) if not o]
    rot = detect_rotation(loose[:: max(1, len(loose) // 12)], da) if loose else 0
    da.unload()
    for r in rooms:
        r["rgb"] = [rotate(im, rot) if not o else im for im, o in zip(r["rgb"], r["exif_orient"])]
        r["vg"] = [to_vggt(im) for im in r["rgb"]]
    log(f"orientation for photos without EXIF orientation: {rot} quarter turn(s)")

    dp = DepthPro(device)
    for r in rooms:
        r["metric"] = [dp(im) for im in r["vg"]]
    dp.unload()
    log(f"Depth Pro on all photos ({time.time() - t0:.0f}s)")

    # Per-room reconstruction in metres, gravity aligned.
    runner = VGGTRunner(device)
    for r in rooms:
        ext, K, D, C = run_group(runner, r["vg"])
        ratios = metric_scale(D, C, K, r["metric"], r["f35"], [im.shape[:2] for im in r["vg"]])
        sc = float(np.median(ratios)) if len(ratios) else 1.0
        r["scale_spread"] = float(np.median(np.abs(np.log(ratios / sc)))) * 1.4826 if len(ratios) > 1 else 0.2
        ext = ext.copy()
        ext[:, :, 3] *= sc
        D = [d * sc for d in D]
        Rcw = np.array([e[:, :3].T for e in ext])
        Ccw = np.array([-e[:, :3].T @ e[:, 3] for e in ext])
        up = -Rcw[:, :, 1].mean(0)
        up /= np.linalg.norm(up)
        pts = np.concatenate([unproject(D[k], K[k], ext[k], step=4, mask=C[k] > np.quantile(C[k], 0.4))
                              for k in range(len(D))])
        up = floor_up(up, pts, Ccw)
        Rw = rotation_to_y(up)
        r["Rcw"] = np.array([Rw @ R for R in Rcw])   # camera-to-world, room frame, Y up
        r["C"] = Ccw @ Rw.T
        r["K"], r["D"], r["conf"] = K, D, C
        r["pts"] = pts @ Rw.T
        log(f"{r['name']}: {len(D)} photos, metric scale x{sc:.3f} (spread {r['scale_spread'] * 100:.0f}%)")

    # Links between rooms through shared views (doorway photos).
    sift = cv2.SIFT_create(4000)
    links = []
    for a in range(len(rooms)):
        for b in range(a + 1, len(rooms)):
            best = 0
            for ia in rooms[a]["vg"]:
                for ib in rooms[b]["vg"]:
                    best = max(best, sift_inliers(ia, ib, sift))
            if best >= args.min_inliers:
                links.append((best, a, b))
    log(f"links: {[(rooms[a]['name'], rooms[b]['name'], n) for n, a, b in links]}")

    # Joint VGGT per linked pair -> relative room transform (yaw + translation).
    rel = {}
    for n, a, b in links:
        imgs = rooms[a]["vg"] + rooms[b]["vg"]
        ext, K, D, C = run_group(runner, imgs)
        na = len(rooms[a]["vg"])
        # Fit each room's own (metric, gravity-aligned) points to the joint
        # reconstruction with a similarity, then compose b -> joint -> a.
        def joint_pts(k, sel):
            return unproject(D[k], K[k], ext[k], step=8, mask=sel)

        fits = []
        for room, ks in ((rooms[a], range(0, na)), (rooms[b], range(na, len(imgs)))):
            src, dst = [], []
            for j, k in enumerate(ks):
                sel = (C[k] > np.quantile(C[k], 0.5)) & (room["conf"][j] > np.quantile(room["conf"][j], 0.5))
                Rcw, Cc = room["Rcw"][j], room["C"][j]
                own = np.hstack([Rcw.T, (-Rcw.T @ Cc)[:, None]])
                src.append(unproject(room["D"][j], room["K"][j], own, step=8, mask=sel))
                dst.append(joint_pts(k, sel))
            fits.append(umeyama(np.concatenate(src), np.concatenate(dst)))
        (sa, Ra, ta), (sb, Rb, tb) = fits
        # point in b's room frame -> joint -> a's room frame
        Rab = Ra.T @ Rb * (sb / sa)
        tab = Ra.T @ (tb - ta) / sa
        # Project onto yaw + translation using b's points.
        pb = rooms[b]["pts"][:: max(1, len(rooms[b]["pts"]) // 5000)]
        R4, t4 = yaw_translation(pb, pb @ Rab.T + tab)
        rel[(a, b)] = (R4, t4, n, sb / sa)
        log(f"  {rooms[a]['name']} <- {rooms[b]['name']}: yaw {math.degrees(math.atan2(R4[0, 2], R4[0, 0])):.1f}°, "
            f"scale agreement {sb / sa:.3f}")

    # Maximum spanning tree from the best-connected room.
    deg = np.zeros(len(rooms))
    for n, a, b in links:
        deg[a] += n
        deg[b] += n
    root = int(np.argmax(deg)) if links else 0
    place = {root: (np.eye(3), np.zeros(3))}
    edges = sorted(links, reverse=True)
    changed = True
    while changed:
        changed = False
        for n, a, b in edges:
            if (a in place) == (b in place):
                continue
            R4, t4 = rel[(a, b)][:2]
            if a in place:
                Ra_, ta_ = place[a]
                place[b] = (Ra_ @ R4, Ra_ @ t4 + ta_)
            else:
                Rb_, tb_ = place[b]
                Ri = R4.T
                place[a] = (Rb_ @ Ri, Rb_ @ (-Ri @ t4) + tb_)
            changed = True
    unplaced = [rooms[i]["name"] for i in range(len(rooms)) if i not in place]
    if unplaced:
        log(f"WARNING: no doorway link found for {unplaced}; these rooms are not part of the stitched plan")

    # Export placed rooms as one capture.
    W = max(im.shape[1] for r in rooms for im in r["vg"])
    H = max(im.shape[0] for r in rooms for im in r["vg"])
    rows, frame = [], 0
    for i, r in enumerate(rooms):
        if i not in place:
            continue
        Rp, tp = place[i]
        for j in range(len(r["vg"])):
            h, w = r["vg"][j].shape[:2]
            K = r["K"][j].copy()
            K[0] *= W / w
            K[1] *= H / h
            dm = cv2.resize(r["D"][j], (W, H), interpolation=cv2.INTER_NEAREST)
            cm = cv2.resize(r["conf"][j], (W, H), interpolation=cv2.INTER_NEAREST)
            valid = (dm > 0.1) & (dm < args.max_depth)
            lo, hi = np.quantile(cm, 0.3), np.quantile(cm, 0.6)
            conf = (np.where(cm >= hi, 2, np.where(cm >= lo, 1, 0)) * valid).astype(np.uint8)
            cv2.imwrite(str(out / "depth" / f"{frame:06d}.png"), np.where(valid, np.round(dm * 1000), 0).astype(np.uint16))
            cv2.imwrite(str(out / "confidence" / f"{frame:06d}.png"), conf)
            Rcw = Rp @ r["Rcw"][j]
            Cw = Rp @ r["C"][j] + tp
            qx, qy, qz, qw = mat_to_quat(Rcw)
            t = i * 100.0 + j  # one time block per room
            rows.append(f"{t:.3f}, {frame:06d}, {Cw[0]:.6f}, {Cw[1]:.6f}, {Cw[2]:.6f}, {qx:.8f}, {qy:.8f}, "
                        f"{qz:.8f}, {qw:.8f}, {K[0, 0]:.4f}, {K[1, 1]:.4f}, {K[0, 2]:.4f}, {K[1, 2]:.4f}, , ")
            frame += 1
    with open(out / "odometry.csv", "w", newline="\n") as fh:
        fh.write("timestamp, frame, x, y, z, qx, qy, qz, qw, fx, fy, cx, cy, distortion_center_x, distortion_center_y\n")
        fh.write("\n".join(rows) + "\n")
    Km = np.median(np.array([r["K"][j] for r in rooms for j in range(len(r["vg"]))]), axis=0)
    with open(out / "camera_matrix.csv", "w", newline="\n") as fh:
        fh.write(f"{Km[0, 0]:.4f}, 0.0, {Km[0, 2]:.4f}\n0.0, {Km[1, 1]:.4f}, {Km[1, 2]:.4f}\n0.0, 0.0, 1.0")
    spreads = [r["scale_spread"] for r in rooms]
    scale_sig = math.sqrt(float(np.median(spreads)) ** 2 / 4 + 0.06 ** 2)
    meta = {
        "tier": "photo", "rgb_width": W, "rgb_height": H, "rooms": [r["name"] for r in rooms],
        "rooms_unplaced": unplaced,
        "links": [{"rooms": [rooms[a]["name"], rooms[b]["name"]], "inliers": n,
                   "scale_agreement": rel[(a, b)][3]} for n, a, b in links],
        "image_rotation_quarter_turns": rot, "scale_rel_sigma": scale_sig,
        "notes": [f"rooms without a doorway link: {', '.join(unplaced)}"] if unplaced else [],
        "models": [{"name": "facebook/VGGT-1B", "version": "hf"}, {"name": "apple/DepthPro-hf", "version": "hf"}],
        "runtime_s": round(time.time() - t0, 1),
    }
    json.dump(meta, open(out / "meta.json", "w"), indent=2)
    log(f"export: {frame} photos from {len(place)} placed rooms ({time.time() - t0:.0f}s)")


if __name__ == "__main__":
    main()
