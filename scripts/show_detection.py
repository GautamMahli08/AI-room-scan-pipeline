"""Draw the strongest detections of one class on their frames, upright,
side by side (visual check of damage detections).

usage: python scripts/show_detection.py <capture_dir> <results_dir> <class> <out.jpg> [n]
"""
import glob
import json
import subprocess
import sys
import tempfile

import cv2

cap, res, cls, out = sys.argv[1:5]
n = int(sys.argv[5]) if len(sys.argv) > 5 else 3
video = glob.glob(cap + "/*/rgb.mp4")[0] if not glob.glob(cap + "/rgb.mp4") else cap + "/rgb.mp4"
dets = json.load(open(res + "/damage/detections.json"))["detections"]
top = sorted([d for d in dets if d["class"] == cls], key=lambda d: -d["score"])[:n]
tiles = []
with tempfile.TemporaryDirectory() as t:
    for d in top:
        subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", video, "-vf", f"select=eq(n\\,{d['frame']})",
                        "-frames:v", "1", f"{t}/f.jpg"], check=True)
        im = cv2.imread(f"{t}/f.jpg")
        b = [int(v) for v in d["box"]]
        cv2.rectangle(im, (b[0], b[1]), (b[2], b[3]), (0, 0, 255), 12)
        im = cv2.resize(cv2.rotate(im, cv2.ROTATE_90_CLOCKWISE), (300, 400))
        cv2.putText(im, f"f{d['frame']} {d['score']:.2f}", (8, 24), cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 0, 255), 2)
        tiles.append(im)
cv2.imwrite(out, cv2.hconcat(tiles))
print(out, [(d["frame"], d["score"]) for d in top])
