"""Compare a video/photo-tier camera trajectory with the LiDAR capture's
ARKit trajectory of the same video (diagnostics only: the video tier
never reads the ARKit poses).

Frames are matched by timestamp; a single similarity transform (Umeyama)
aligns the estimate to ARKit. Reports the fitted scale (1.0 = metric scale
right), the absolute trajectory error after alignment, the per-segment
scale drift, and the rotation error.

usage: python scripts/traj_eval.py <export_dir> <stray_scanner_dir>
"""
import csv
import io
import sys

import numpy as np

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")


def read_odo(path):
    rows = list(csv.reader(open(path)))[1:]
    t = np.array([float(r[0]) for r in rows])
    C = np.array([[float(r[2]), float(r[3]), float(r[4])] for r in rows])
    q = np.array([[float(x) for x in r[5:9]] for r in rows])
    return t, C, q


def quat_to_mat(q):
    x, y, z, w = q / np.linalg.norm(q)
    return np.array([[1 - 2 * (y * y + z * z), 2 * (x * y - z * w), 2 * (x * z + y * w)],
                     [2 * (x * y + z * w), 1 - 2 * (x * x + z * z), 2 * (y * z - x * w)],
                     [2 * (x * z - y * w), 2 * (y * z + x * w), 1 - 2 * (x * x + y * y)]])


def umeyama(src, dst):
    mu_s, mu_d = src.mean(0), dst.mean(0)
    xs, xd = src - mu_s, dst - mu_d
    U, S, Vt = np.linalg.svd(xd.T @ xs / len(src))
    D = np.eye(3)
    if np.linalg.det(U @ Vt) < 0:
        D[2, 2] = -1
    R = U @ D @ Vt
    s = np.trace(np.diag(S) @ D) / xs.var(0).sum()
    return s, R, mu_d - s * R @ mu_s


def main():
    te, Ce, qe = read_odo(sys.argv[1] + "/odometry.csv")
    tg, Cg, qg = read_odo(sys.argv[2] + "/odometry.csv")
    tg = tg - tg[0]
    idx = np.clip(np.searchsorted(tg, te), 0, len(tg) - 1)
    G = Cg[idx]
    s, R, t = umeyama(Ce, G)
    A = s * Ce @ R.T + t
    err = np.linalg.norm(A - G, axis=1)
    path = np.linalg.norm(np.diff(G, axis=0), axis=1).sum()
    print(f"frames {len(te)}, ARKit path {path:.1f} m")
    print(f"similarity scale to fit ARKit: {s:.3f} (1.0 = metric scale correct)")
    print(f"ATE after alignment: median {np.median(err) * 100:.1f} cm, max {err.max() * 100:.1f} cm")
    # Scale consistency: fit scale per third of the walk.
    n = len(te)
    for k in range(3):
        sl = slice(k * n // 3, (k + 1) * n // 3)
        if sl.stop - sl.start > 5:
            sk, _, _ = umeyama(Ce[sl], G[sl])
            print(f"  segment {k + 1}: scale {sk:.3f}")
    # Relative rotation error between consecutive frames.
    rot_err = []
    for i in range(len(te) - 1):
        Re = quat_to_mat(qe[i]).T @ quat_to_mat(qe[i + 1])
        Rg = quat_to_mat(qg[idx[i]]).T @ quat_to_mat(qg[idx[i + 1]])
        d = Re.T @ Rg
        rot_err.append(np.degrees(np.arccos(np.clip((np.trace(d) - 1) / 2, -1, 1))))
    print(f"frame-to-frame rotation error: median {np.median(rot_err):.2f}°, p90 {np.percentile(rot_err, 90):.2f}°")


main()


def per_frame(export, stray, chunk=8, overlap=3):
    """Rotation error of each consecutive pair, marking pairs whose second
    frame is the first new frame of a VGGT chunk."""
    te, Ce, qe = read_odo(export + "/odometry.csv")
    tg, Cg, qg = read_odo(stray + "/odometry.csv")
    tg = tg - tg[0]
    idx = np.clip(np.searchsorted(tg, te), 0, len(tg) - 1)
    step = chunk - overlap
    firsts = {0} | {chunk + k * step for k in range(len(te))}
    inside, boundary = [], []
    for i in range(len(te) - 1):
        Re = quat_to_mat(qe[i]).T @ quat_to_mat(qe[i + 1])
        Rg = quat_to_mat(qg[idx[i]]).T @ quat_to_mat(qg[idx[i + 1]])
        ang = np.degrees(np.arccos(np.clip((np.trace(Re.T @ Rg) - 1) / 2, -1, 1)))
        ae = np.degrees(np.arccos(np.clip((np.trace(Re) - 1) / 2, -1, 1)))
        ag = np.degrees(np.arccos(np.clip((np.trace(Rg) - 1) / 2, -1, 1)))
        (boundary if (i + 1) in firsts else inside).append((ang, ae, ag))
    for name, v in (("inside a chunk", inside), ("across chunk boundary", boundary)):
        v = np.array(v)
        print(f"{name}: {len(v)} pairs, error median {np.median(v[:, 0]):.2f}°, "
              f"motion est {np.median(v[:, 1]):.2f}° vs ARKit {np.median(v[:, 2]):.2f}°")
