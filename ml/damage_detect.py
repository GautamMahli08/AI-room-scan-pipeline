"""Damage detection on video frames (open-vocabulary, OWLv2).

Reads the requested frames from a video, rotates them upright (detectors
are trained on upright photos), runs OWLv2 with one text prompt set per
damage class, and writes boxes in the ORIGINAL frame's pixel coordinates.
Projection onto surfaces, multi-view merging, rules and scope are done in
Go, which has the depth and poses.

usage: python ml/damage_detect.py --video rgb.mp4 --frames 0,30,60 --rot 1 --out detections.json
"""
import argparse
import json
import subprocess
import sys
import tempfile
import time
from pathlib import Path

import cv2
import numpy as np

# Several phrasings per class; the best-scoring phrase wins.
PROMPTS = {
    "water_stain": ["a water stain on a wall", "a brown water stain on a ceiling", "water damage"],
    "mould": ["black mould on a wall", "mold growth"],
    # "thin hairline" matters: on the single_room bathroom crack the generic
    # phrases score 0.20 on wall-sized boxes, this one 0.50 on the crack.
    "crack": ["a thin hairline crack in a white wall", "a crack in a wall", "a cracked plaster wall"],
    "peeling_paint": ["peeling paint", "flaking paint on a wall"],
    "hole": ["a hole in a wall", "a damaged hole in drywall"],
}
# Things open-vocabulary detectors confuse with damage on plain walls.
# A box whose best phrase is a distractor is dropped, and so are damage
# boxes overlapping it.
DISTRACTORS = ["a potted plant", "a curtain", "a shadow on a wall", "a light switch", "a picture frame",
               "a piece of furniture", "a door", "a window", "a person",
               # bathroom fixtures: marble veining was read as a water stain in floor_only
               "a marble countertop", "a bathroom sink", "a toilet", "a trash bin", "a mirror"]
ROT = {1: cv2.ROTATE_90_CLOCKWISE, 2: cv2.ROTATE_180, 3: cv2.ROTATE_90_COUNTERCLOCKWISE}


def log(*a):
    print("[damage]", *a, file=sys.stderr, flush=True)


def unrotate_box(box, rot, w, h):
    """Map a box from the rotated (upright) image back to the original
    w x h frame. rot = clockwise quarter turns applied to the original."""
    # Clockwise turn: original (x, y) -> upright (h-1-y, x). Inverse: x = y_up, y = h-1-x_up.
    # Counter-clockwise: original (x, y) -> upright (y, w-1-x). Inverse: x = w-1-y_up, y = x_up.
    x0, y0, x1, y1 = box
    pts = np.array([[x0, y0], [x1, y0], [x0, y1], [x1, y1]], float)
    if rot == 1:
        pts = np.stack([pts[:, 1], h - 1 - pts[:, 0]], 1)
    elif rot == 2:
        pts = np.stack([w - 1 - pts[:, 0], h - 1 - pts[:, 1]], 1)
    elif rot == 3:
        pts = np.stack([w - 1 - pts[:, 1], pts[:, 0]], 1)
    return [float(pts[:, 0].min()), float(pts[:, 1].min()), float(pts[:, 0].max()), float(pts[:, 1].max())]


def _self_test():
    """A box marked on a w x h frame must come back to the same place after
    rotating the frame and mapping the rotated box back."""
    import cv2
    h, w = 1440, 1920
    box = (300, 200, 700, 260)  # x0, y0, x1, y1 in the original
    for rot in (1, 2, 3):
        img = np.zeros((h, w), np.uint8)
        img[box[1]:box[3] + 1, box[0]:box[2] + 1] = 255
        up = cv2.rotate(img, ROT[rot])
        ys, xs = np.nonzero(up)
        back = unrotate_box([xs.min(), ys.min(), xs.max(), ys.max()], rot, w, h)
        assert np.allclose(back, box), (rot, back, box)
    print("unrotate_box self-test passed")


def iou(a, b):
    ix = max(0.0, min(a[2], b[2]) - max(a[0], b[0]))
    iy = max(0.0, min(a[3], b[3]) - max(a[1], b[1]))
    inter = ix * iy
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--video", required=True)
    ap.add_argument("--frames", required=True, help="comma-separated frame indices")
    ap.add_argument("--rot", type=int, default=0, help="clockwise quarter turns to make frames upright")
    ap.add_argument("--out", required=True)
    ap.add_argument("--threshold", type=float, default=0.15, help="minimum detector score kept")
    ap.add_argument("--model", default="google/owlv2-base-patch16-ensemble")
    ap.add_argument("--max-area", type=float, default=0.2, help="largest box, as a fraction of the frame")
    args = ap.parse_args()

    import torch
    from transformers import Owlv2ForObjectDetection, Owlv2Processor
    device = "cuda" if torch.cuda.is_available() else "cpu"
    t0 = time.time()
    proc = Owlv2Processor.from_pretrained(args.model)
    model = Owlv2ForObjectDetection.from_pretrained(args.model).to(device).eval()
    labels, classes = [], []
    for cls, phrases in PROMPTS.items():
        for p in phrases:
            labels.append(p)
            classes.append(cls)
    for p in DISTRACTORS:
        labels.append(p)
        classes.append("distractor")

    idx = [int(x) for x in args.frames.split(",") if x]
    with tempfile.TemporaryDirectory() as tmp:
        # Evenly spaced frames: one short select expression (a long eq() chain
        # overflows ffmpeg's expression parser). Otherwise, batches of 30.
        steps = {b - a for a, b in zip(idx, idx[1:])}
        if len(steps) == 1 and idx[0] % next(iter(steps)) == 0:
            batches = [(idx, f"not(mod(n\\,{next(iter(steps))}))*lte(n\\,{idx[-1]})")]
        else:
            batches = [(idx[k:k + 30], "+".join(f"eq(n\\,{i})" for i in idx[k:k + 30])) for k in range(0, len(idx), 30)]
        files = []
        for b, (_, sel) in enumerate(batches):
            subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", args.video, "-vf", f"select='{sel}'", "-fps_mode",
                            "passthrough", "-q:v", "2", str(Path(tmp) / f"b{b:03d}_%06d.jpg")], check=True)
            files += sorted(Path(tmp).glob(f"b{b:03d}_*.jpg"))
        if len(files) != len(idx):
            raise SystemExit(f"extracted {len(files)} frames, expected {len(idx)}")
        out = []
        for fi, f in zip(idx, files):
            bgr = cv2.imread(str(f))
            h, w = bgr.shape[:2]
            up = cv2.rotate(bgr, ROT[args.rot]) if args.rot else bgr
            rgb = cv2.cvtColor(up, cv2.COLOR_BGR2RGB)
            inp = proc(text=[labels], images=rgb, return_tensors="pt").to(device)
            with torch.no_grad():
                res = model(**inp)
            # OWLv2 pads to a square; boxes are relative to the padded square.
            side = max(rgb.shape[:2])
            r = proc.post_process_object_detection(res, threshold=args.threshold,
                                                   target_sizes=torch.tensor([[side, side]], device=device))[0]
            dets, distract = [], []
            for score, lab, box in zip(r["scores"].tolist(), r["labels"].tolist(), r["boxes"].tolist()):
                box = [max(0.0, box[0]), max(0.0, box[1]), min(up.shape[1] - 1.0, box[2]), min(up.shape[0] - 1.0, box[3])]
                if box[2] <= box[0] or box[3] <= box[1]:
                    continue
                (distract if classes[lab] == "distractor" else dets).append((score, lab, box))
            frame_area = up.shape[0] * up.shape[1]
            for score, lab, box in dets:
                area = (box[2] - box[0]) * (box[3] - box[1])
                if area > args.max_area * frame_area:
                    continue  # damage is local; whole-wall boxes are "a wall"
                if any(iou(box, d[2]) > 0.3 and d[0] >= score * 0.8 for d in distract):
                    continue
                out.append({"frame": fi, "class": classes[lab], "prompt": labels[lab], "score": round(score, 4),
                            "box": [round(v, 1) for v in unrotate_box(box, args.rot, w, h)]})
    json.dump({"model": args.model, "threshold": args.threshold, "frames": idx, "detections": out},
              open(args.out, "w"), indent=1)
    log(f"{len(idx)} frames, {len(out)} detections >= {args.threshold} ({time.time() - t0:.0f}s)")


if __name__ == "__main__":
    main()
