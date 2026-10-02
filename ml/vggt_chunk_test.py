"""Diagnostic: accuracy of single VGGT chunks of consecutive frames
against ARKit (each chunk similarity-aligned on its own). If chunks are
locally accurate, the error in the full reconstruction comes from
chaining them, not from VGGT."""
import csv
import glob
import sys

import cv2
import numpy as np

sys.path.insert(0, "F:/LiDAR/ml")
from video_recon import rotate, vggt_size, unproject  # noqa: E402
from vggt_runner import VGGTRunner  # noqa: E402

frames_dir, stray, rot = sys.argv[1], sys.argv[2], int(sys.argv[3])
files = sorted(glob.glob(frames_dir + "/*.jpg"))
first = rotate(cv2.imread(files[0]), rot)
W, H = vggt_size(first.shape[1], first.shape[0])
rows = list(csv.reader(open(stray + "/odometry.csv")))[1:]
tg = np.array([float(r[0]) for r in rows]); tg -= tg[0]
Cg = np.array([[float(r[2]), float(r[3]), float(r[4])] for r in rows])
runner = VGGTRunner("cuda")


def umeyama(src, dst):
    ms, md = src.mean(0), dst.mean(0)
    xs, xd = src - ms, dst - md
    U, S, Vt = np.linalg.svd(xd.T @ xs / len(src))
    Dm = np.eye(3)
    if np.linalg.det(U @ Vt) < 0:
        Dm[2, 2] = -1
    R = U @ Dm @ Vt
    return np.trace(np.diag(S) @ Dm) / xs.var(0).sum(), R, md - (np.trace(np.diag(S) @ Dm) / xs.var(0).sum()) * R @ ms


for start in range(0, len(files) - 8, 16):
    ids = list(range(start, start + 8))
    imgs = [cv2.cvtColor(cv2.resize(rotate(cv2.imread(files[i]), rot), (W, H), interpolation=cv2.INTER_AREA),
                         cv2.COLOR_BGR2RGB) for i in ids]
    ext, K, D, C = runner(imgs)
    cen = np.array([-e[:, :3].T @ e[:, 3] for e in ext])
    G = Cg[np.clip(np.searchsorted(tg, np.array(ids) / 3.0), 0, len(tg) - 1)]
    extent = np.linalg.norm(G - G.mean(0), axis=1).max()
    s, R, t = umeyama(cen, G)
    err = np.linalg.norm(s * cen @ R.T + t - G, axis=1)
    print(f"chunk {start:3d}: ARKit extent {extent * 100:5.1f} cm, scale {s:.3f}, ATE median {np.median(err) * 100:.1f} cm")
