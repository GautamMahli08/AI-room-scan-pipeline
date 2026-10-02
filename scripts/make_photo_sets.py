"""Build photo-tier benchmark inputs from a Stray Scanner capture.

The capture protocol for the photo tier is: one folder per room, 2-8
stills covering the room, including one photo through each doorway taken
from both sides. On the sample data there are no separate stills, so this
script plays the photographer: it uses the LiDAR-tier plan of the same
capture to know which room the phone was in, and picks frames from the
video as a person would take photos. This is benchmark *preparation*: the
photo tier itself only ever sees the JPEGs written here (no poses, depth,
intrinsics, order or room geometry).

Per room: up to 6 wide, level frames (median LiDAR range >= 1.8 m, as a person
standing back would take) inside the room, spread over viewing
directions; plus, for every opening to another room, the frame taken
from inside this room that looks most squarely through it (2-4 m away).

usage: python scripts/make_photo_sets.py <stray_capture_dir> <lidar_plan.json> <out_dir>
"""
import csv
import zlib
import json
import math
import os
import shutil
import subprocess
import sys
from pathlib import Path

import cv2
import numpy as np


def quat_to_mat(x, y, z, w):
    n = math.sqrt(x * x + y * y + z * z + w * w)
    x, y, z, w = x / n, y / n, z / n, w / n
    return np.array([[1 - 2 * (y * y + z * z), 2 * (x * y - z * w), 2 * (x * z + y * w)],
                     [2 * (x * y + z * w), 1 - 2 * (x * x + z * z), 2 * (y * z - x * w)],
                     [2 * (x * z - y * w), 2 * (y * z + x * w), 1 - 2 * (x * x + y * y)]])


def inside(p, poly):
    x, y = p
    c = False
    for i in range(len(poly)):
        x1, y1 = poly[i]
        x2, y2 = poly[i - 1]
        if (y1 > y) != (y2 > y) and x < (x2 - x1) * (y - y1) / (y2 - y1) + x1:
            c = not c
    return c


def shrink_ok(p, poly, margin):
    """p inside poly and at least margin from every edge."""
    if not inside(p, poly):
        return False
    for i in range(len(poly)):
        a, b = np.array(poly[i - 1]), np.array(poly[i])
        ab = b - a
        t = np.clip(np.dot(np.array(p) - a, ab) / np.dot(ab, ab), 0, 1)
        if np.linalg.norm(np.array(p) - (a + t * ab)) < margin:
            return False
    return True


def main():
    cap, plan_path, out = Path(sys.argv[1]), Path(sys.argv[2]), Path(sys.argv[3])
    if any(cap.glob("*/odometry.csv")) and not (cap / "odometry.csv").exists():
        cap = next(cap.glob("*/odometry.csv")).parent
    plan = json.load(open(plan_path))
    th = math.radians(plan["diagnostics"]["manhattan_deg"])
    c, s = math.cos(-th), math.sin(-th)

    rows = list(csv.reader(open(cap / "odometry.csv")))[1:]
    frames = []
    for r in rows[::10]:
        idx = int(r[1])
        C = np.array([float(v) for v in r[2:5]])
        R = quat_to_mat(*[float(v) for v in r[5:9]])
        fwd = R @ np.array([0, 0, 1.0])  # OpenCV camera looks along +z
        px, py = c * C[0] - s * (-C[2]), s * C[0] + c * (-C[2])
        fx, fy = c * fwd[0] - s * (-fwd[2]), s * fwd[0] + c * (-fwd[2])
        frames.append({"idx": idx, "pos": (px, py), "dir": (fx, fy), "pitch": fwd[1]})

    # Openings between rooms, as plan-space centres with their rooms.
    rooms = {r["id"]: r for r in plan["rooms"]}
    doors = []
    for adj in plan["adjacency"]:
        for oid in adj["via"]:
            for r in plan["rooms"]:
                for o in r["openings"]:
                    if o["id"] != oid:
                        continue
                    w = next(w for w in r["walls"] if w["id"] == o["wall"])
                    a, b = np.array(w["start"]), np.array(w["end"])
                    u = (b - a) / np.linalg.norm(b - a)
                    mid = a + u * (o["offset_m"]["value"] + o["width_m"]["value"] / 2)
                    doors.append({"rooms": adj["rooms"], "centre": mid, "id": oid})

    # A person photographing a room stands back and takes wide, level
    # shots; a scanning video mostly looks at nearby walls. Score frames by
    # how far away the scene is (median LiDAR depth) so the "photos" are
    # wide views, as the protocol asks for.
    for f in frames:
        d = cv2.imread(str(cap / "depth" / f"{f['idx']:06d}.png"), cv2.IMREAD_UNCHANGED)
        f["range"] = float(np.median(d[d > 0])) / 1000 if d is not None and (d > 0).any() else 0.0

    picks = {rid: set() for rid in rooms}
    for rid, r in rooms.items():
        poly = r["polygon"]
        cand = [f for f in frames if shrink_ok(f["pos"], poly, 0.3) and abs(f["pitch"]) < 0.35 and f["range"] >= 1.8]
        # Spread over 6 viewing-direction sectors; the widest view in each.
        for k in range(6):
            sec = [f for f in cand if int(((math.atan2(f["dir"][1], f["dir"][0]) + math.pi) / (2 * math.pi)) * 6) % 6 == k]
            if sec:
                picks[rid].add(max(sec, key=lambda f: f["range"])["idx"])
    door_picks = {rid: [] for rid in rooms}
    for d in doors:
        for rid in d["rooms"]:
            poly = rooms[rid]["polygon"]
            best, best_score = None, 1e9
            for f in frames:
                if not inside(f["pos"], poly):
                    continue
                v = d["centre"] - np.array(f["pos"])
                dist = np.linalg.norm(v)
                if not 1.5 <= dist <= 4.0:
                    continue
                ang = math.degrees(math.acos(np.clip(np.dot(v / dist, f["dir"]) / (np.linalg.norm(f["dir"]) + 1e-9), -1, 1)))
                if ang < 20 and abs(f["pitch"]) < 0.5 and ang + abs(dist - 2.5) * 5 < best_score:
                    best, best_score = f["idx"], ang + abs(dist - 2.5) * 5
            if best is not None:
                door_picks[rid].append(best)

    if out.exists():
        shutil.rmtree(out)
    manifest = {"source": str(cap), "plan": str(plan_path), "rooms": {}}
    # A photo belongs to one room: a frame (or one within half a second of
    # it) already given to another room is not reused, even where room
    # outlines overlap at a doorway. Real photo sets never share files.
    used = []

    def free(i):
        return all(abs(i - u) > 23 for u in used)

    for rid in rooms:
        sel = []
        for i in list(door_picks[rid]) + sorted(picks[rid]):
            if i not in sel and free(i) and len(sel) < 8:
                sel.append(i)
        used.extend(sel)
        door_picks[rid] = [i for i in door_picks[rid] if i in sel]
        if len(sel) < 2:
            continue
        d = out / rid
        d.mkdir(parents=True)
        # Shuffled names: the photo tier gets no capture order.
        rng = np.random.default_rng(zlib.crc32(rid.encode()))
        names = rng.permutation(len(sel))
        for name, idx in zip(names, sel):
            subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", str(cap / "rgb.mp4"), "-vf",
                            f"select=eq(n\\,{idx})", "-frames:v", "1", "-q:v", "2", str(d / f"IMG_{name:02d}.jpg")],
                           check=True)
        manifest["rooms"][rid] = {"frames": [int(i) for i in sel], "doorway_frames": [int(i) for i in door_picks[rid]]}
        print(f"{rid}: {len(sel)} photos ({len(door_picks[rid])} through doorways)")
    json.dump(manifest, open(out.parent / (out.name + "_manifest.json"), "w"), indent=2)


main()
