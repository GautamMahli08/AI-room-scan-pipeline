"""Diagnostic: one VGGT pass over N keyframes spread across a video,
scored against the capture's ARKit trajectory (similarity-aligned).
Separates VGGT's own accuracy from errors added by chunk chaining."""
import csv
import sys
import time

import cv2
import numpy as np

sys.path.insert(0, "F:/LiDAR/ml")
from video_recon import rotate, vggt_size  # noqa: E402
from vggt_runner import VGGTRunner  # noqa: E402

frames_dir, stray, n, rot = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
import glob  # noqa: E402

files = sorted(glob.glob(frames_dir + "/*.jpg"))
sel = np.linspace(0, len(files) - 1, n).astype(int)
first = rotate(cv2.imread(files[0]), rot)
W, H = vggt_size(first.shape[1], first.shape[0])
imgs = [cv2.cvtColor(cv2.resize(rotate(cv2.imread(files[i]), rot), (W, H), interpolation=cv2.INTER_AREA),
                     cv2.COLOR_BGR2RGB) for i in sel]
runner = VGGTRunner("cuda")
t = time.time()
ext, K, D, C = runner(imgs)
print(f"VGGT {n} frames {time.time() - t:.0f}s, focal {np.median(K[:, 0, 0]):.1f}px (true ~{1599.7 * H / 1920:.1f})")

centres = np.array([-e[:, :3].T @ e[:, 3] for e in ext])
rows = list(csv.reader(open(stray + "/odometry.csv")))[1:]
tg = np.array([float(r[0]) for r in rows]); tg -= tg[0]
Cg = np.array([[float(r[2]), float(r[3]), float(r[4])] for r in rows])
times = sel / 3.0  # frames were extracted at 3 fps
G = Cg[np.clip(np.searchsorted(tg, times), 0, len(tg) - 1)]


def umeyama(src, dst):
    ms, md = src.mean(0), dst.mean(0)
    xs, xd = src - ms, dst - md
    U, S, Vt = np.linalg.svd(xd.T @ xs / len(src))
    Dm = np.eye(3)
    if np.linalg.det(U @ Vt) < 0:
        Dm[2, 2] = -1
    R = U @ Dm @ Vt
    s = np.trace(np.diag(S) @ Dm) / xs.var(0).sum()
    return s, R, md - s * R @ ms


s, R, tt = umeyama(centres, G)
err = np.linalg.norm(s * centres @ R.T + tt - G, axis=1)
print(f"scale VGGT->metres {s:.3f}; ATE median {np.median(err) * 100:.1f} cm, max {err.max() * 100:.1f} cm")
