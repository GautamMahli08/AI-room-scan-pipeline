"""Diagnostic: compare photo-tier camera poses with the ARKit poses of the
frames the photos were cut from (via the photo-set manifest). Separates
reconstruction error from errors added later (fusion, plan extraction).

usage: python scripts/photo_eval.py <export_dir> <photo_root> <manifest.json> <stray_capture_dir>
"""
import csv
import io
import json
import math
import sys
import zlib
from pathlib import Path

import numpy as np

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")


def quat_to_mat(x, y, z, w):
    n = math.sqrt(x * x + y * y + z * z + w * w)
    x, y, z, w = x / n, y / n, z / n, w / n
    return np.array([[1 - 2 * (y * y + z * z), 2 * (x * y - z * w), 2 * (x * z + y * w)],
                     [2 * (x * y + z * w), 1 - 2 * (x * x + z * z), 2 * (y * z - x * w)],
                     [2 * (x * z - y * w), 2 * (y * z + x * w), 1 - 2 * (x * x + y * y)]])


def umeyama(src, dst):
    ms, md = src.mean(0), dst.mean(0)
    xs, xd = src - ms, dst - md
    U, S, Vt = np.linalg.svd(xd.T @ xs / len(src))
    D = np.eye(3)
    if np.linalg.det(U @ Vt) < 0:
        D[2, 2] = -1
    R = U @ D @ Vt
    s = np.trace(np.diag(S) @ D) / xs.var(0).sum()
    return s, R, md - s * R @ ms


export, root, manifest, stray = Path(sys.argv[1]), Path(sys.argv[2]), json.load(open(sys.argv[3])), Path(sys.argv[4])
if not (stray / "odometry.csv").exists():
    stray = next(stray.glob("*/odometry.csv")).parent
ark = {int(r[1]): r for r in list(csv.reader(open(stray / "odometry.csv")))[1:]}
rows = list(csv.reader(open(export / "odometry.csv")))[1:]
meta = json.load(open(export / "meta.json"))

# Export frame order: rooms in sorted folder order, photos in sorted file order.
k = 0
for room in sorted(d.name for d in root.iterdir() if d.is_dir()):
    files = sorted(f.name for f in (root / room).iterdir())
    if room not in manifest["rooms"] or len(files) < 2:
        continue
    sel = manifest["rooms"][room]["frames"]
    names = np.random.default_rng(zlib.crc32(room.encode())).permutation(len(sel))
    frame_of = {f"IMG_{n:02d}.jpg": idx for n, idx in zip(names, sel)}
    E, G = [], []
    for f in files:
        r = rows[k]
        k += 1
        E.append([float(r[2]), float(r[3]), float(r[4])])
        a = ark[frame_of[f]]
        G.append([float(a[2]), float(a[3]), float(a[4])])
    E, G = np.array(E), np.array(G)
    s, R, t = umeyama(E, G)
    err = np.linalg.norm(s * E @ R.T + t - G, axis=1)
    extent = np.linalg.norm(G - G.mean(0), axis=1).max()
    print(f"{room}: {len(files)} photos, ARKit camera spread {extent:.2f} m; similarity scale {s:.3f} "
          f"(1 = metric right); camera position error after alignment: median {np.median(err) * 100:.1f} cm, "
          f"max {err.max() * 100:.1f} cm")
