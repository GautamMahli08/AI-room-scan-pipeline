"""Diagnostic: OWLv2 crack scores on one frame, whole image vs tiles.

usage: python ml/crack_probe.py <video> <frame> <rot>
"""
import subprocess
import sys
import tempfile

import cv2
import torch
from transformers import Owlv2ForObjectDetection, Owlv2Processor

video, frame, rot = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
ROT = {1: cv2.ROTATE_90_CLOCKWISE, 2: cv2.ROTATE_180, 3: cv2.ROTATE_90_COUNTERCLOCKWISE}
with tempfile.TemporaryDirectory() as t:
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", video, "-vf", f"select=eq(n\\,{frame})", "-frames:v", "1",
                    f"{t}/f.jpg"], check=True)
    img = cv2.imread(f"{t}/f.jpg")
img = cv2.cvtColor(cv2.rotate(img, ROT[rot]) if rot else img, cv2.COLOR_BGR2RGB)

name = "google/owlv2-base-patch16-ensemble"
proc = Owlv2Processor.from_pretrained(name)
model = Owlv2ForObjectDetection.from_pretrained(name).to("cuda").eval()
prompts = ["a crack in a wall", "a thin hairline crack in a white wall", "a cracked plaster wall",
           "a crack line on a painted wall"]


def best(rgb):
    inp = proc(text=[prompts], images=rgb, return_tensors="pt").to("cuda")
    with torch.no_grad():
        out = model(**inp)
    side = max(rgb.shape[:2])
    r = proc.post_process_object_detection(out, threshold=0.0, target_sizes=torch.tensor([[side, side]], device="cuda"))[0]
    rows = []
    for s, l, b in zip(r["scores"].tolist(), r["labels"].tolist(), r["boxes"].tolist()):
        area = (b[2] - b[0]) * (b[3] - b[1]) / (rgb.shape[0] * rgb.shape[1])
        rows.append((s, prompts[l], area, [round(v) for v in b]))
    rows.sort(reverse=True)
    return rows[:3]


H, W = img.shape[:2]
print("WHOLE FRAME", W, "x", H)
for r in best(img):
    print(f"  {r[0]:.3f} {r[1]!r} box {r[3]} ({r[2] * 100:.0f}% of frame)")
for ty in range(2):
    for tx in range(2):
        # 2x2 tiles with 20 % overlap
        x0, y0 = int(tx * W * 0.4), int(ty * H * 0.4)
        tile = img[y0:y0 + int(H * 0.6), x0:x0 + int(W * 0.6)]
        print(f"TILE ({tx},{ty}) at x={x0} y={y0}")
        for r in best(tile):
            print(f"  {r[0]:.3f} {r[1]!r} box {r[3]} ({r[2] * 100:.0f}% of tile)")
