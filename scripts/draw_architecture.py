"""Draw the software architecture (code modules, processes, files) to
docs/img/architecture.png.

Usage: ml/.venv/Scripts/python.exe scripts/draw_architecture.py
"""
import os
from PIL import Image, ImageDraw, ImageFont

W, H = 2400, 1700
OUT = os.path.join(os.path.dirname(__file__), "..", "docs", "img", "architecture.png")

BG = (250, 250, 247)
INK = (34, 38, 46)
MUTED = (96, 104, 116)
GO = ((219, 234, 254), (59, 110, 190))
GO_REGION = ((238, 244, 253), (59, 110, 190))
PY = ((254, 236, 210), (196, 120, 30))
PY_REGION = ((255, 247, 235), (196, 120, 30))
IO = ((226, 243, 226), (60, 140, 80))
CLI = ((236, 233, 250), (110, 90, 180))
NOTE = ((240, 240, 242), (140, 140, 150))


def font(size, bold=False, mono=False):
    if mono:
        names = ["consolab.ttf" if bold else "consola.ttf", "DejaVuSansMono.ttf"]
    else:
        names = ["segoeuib.ttf" if bold else "segoeui.ttf", "arialbd.ttf" if bold else "arial.ttf"]
    for n in names:
        for d in ["C:/Windows/Fonts", "/usr/share/fonts/truetype/dejavu"]:
            p = os.path.join(d, n)
            if os.path.exists(p):
                return ImageFont.truetype(p, size)
    return ImageFont.load_default()


F_TITLE = font(46, True)
F_SUB = font(24)
F_REGION = font(26, True)
F_HEAD = font(23, True, mono=True)
F_HEAD_TXT = font(24, True)
F_BODY = font(19)
F_LAB = font(18)

img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)


def box(x, y, w, h, title, lines, style, mono=True, radius=14, width=3):
    fill, edge = style
    d.rounded_rectangle([x, y, x + w, y + h], radius=radius, fill=fill, outline=edge, width=width)
    d.text((x + 16, y + 12), title, font=F_HEAD if mono else F_HEAD_TXT, fill=INK)
    ty = y + 48
    for ln in lines:
        d.text((x + 18, ty), ln, font=F_BODY, fill=INK)
        ty += 26


def region(x, y, w, h, label, style, lx=None, bottom=False):
    fill, edge = style
    d.rounded_rectangle([x, y, x + w, y + h], radius=22, fill=fill, outline=edge, width=3)
    tw = d.textlength(label, font=F_REGION)
    lx = x + 24 if lx is None else lx
    ly = y + h if bottom else y
    d.rectangle([lx, ly - 18, lx + 16 + tw, ly + 16], fill=BG)
    d.text((lx + 8, ly - 18), label, font=F_REGION, fill=edge)


def head(px, py, dx, dy, color, size=12):
    lx, ly = -dy, dx
    d.polygon([(px, py),
               (px - dx * size * 1.7 + lx * size * 0.8, py - dy * size * 1.7 + ly * size * 0.8),
               (px - dx * size * 1.7 - lx * size * 0.8, py - dy * size * 1.7 - ly * size * 0.8)], fill=color)


def arrow(points, color=INK, width=4):
    d.line(points, fill=color, width=width, joint="curve")
    (x0, y0), (x1, y1) = points[-2], points[-1]
    n = max(abs(x1 - x0), abs(y1 - y0)) or 1
    head(x1, y1, (x1 - x0) / n, (y1 - y0) / n, color)


def label(x, y, text, color=MUTED):
    tw = d.textlength(text, font=F_LAB)
    d.rectangle([x - 5, y - 1, x + tw + 5, y + 23], fill=BG)
    d.text((x, y), text, font=F_LAB, fill=color)


# Title
d.text((60, 36), "Room Scan Pipeline: software architecture", font=F_TITLE, fill=INK)
d.text((62, 98), "Two programs: a Go pipeline that does all geometry, and Python scripts that run the neural "
       "networks. They talk only through files.", font=F_SUB, fill=MUTED)

# CLI entry points
box(560, 160, 560, 100, "cmd/scan", ["scan run <capture> [-tier] [-damage] [-drift] [-live]"], CLI)
box(1180, 160, 560, 100, "cmd/bench", ["bench repeat | overlay | calibrate"], CLI)

# Inputs
box(60, 340, 380, 620, "Capture on disk", [
    "", "LiDAR scan folder", "(Stray Scanner export)", "  rgb.mp4", "  depth/  confidence/",
    "  odometry.csv  (poses)", "  camera_matrix.csv", "", "Video file", "  .mp4 / .MOV", "",
    "Photo folders", "  one folder per room", "", "Tier is detected from", "what is in the folder."], IO, mono=False)

# Go process
region(480, 330, 1300, 720, "Go process  (roomscan, go build \u2192 bin/scan.exe)", GO_REGION, lx=690)
box(520, 370, 1220, 140, "internal/pipeline   \u2014 the orchestrator", [
    "1. reconstruct (video / photo only: run Python)   2. ingest   3. fuse   4. geometry",
    "5. stitch + drift   6. calibrate intervals   7. damage (optional)   8. write outputs",
    "files: reconstruct.go  lidar.go  damage.go  report.go  debug.go"], GO)
row_b = [
    ("geometry/pointcloud", ["voxel fusion", "integer sums \u2192", "deterministic", "PLY export"]),
    ("geometry", ["floor plane, Manhattan", "walls, gaps \u2192 openings", "rooms: carve + split",
                  "outline, ceiling, raster"]),
    ("stitch", ["drift correction", "plane-anchored,", "per 4 s chunk,", "held-out prior"]),
    ("calib", ["90% intervals", "model + empirical \u03c4", "empirical_lidar.json", "(embedded)"]),
]
row_c = [
    ("ingest/strayscanner", ["reads the export:", "poses, intrinsics,", "depth, confidence"]),
    ("frame  +  geom", ["shared types:", "Frame, Pose, K,", "vectors, planes"]),
    ("output", ["plan.json + schema", "validation, plan.svg", "with damage markers"]),
    ("damage", ["place detections on", "wall / floor / ceiling", "concealed-damage rules"]),
]
cw, gap = 290, 20
xs = [520 + i * (cw + gap) for i in range(4)]
for x, (t, ls) in zip(xs, row_b):
    box(x, 540, cw, 200, t, ls, GO)
for x, (t, ls) in zip(xs, row_c):
    box(x, 790, cw, 200 - 30, t, ls, GO)
label(1000, 1008, "internal/bench: repeat, overlay, calibrate (used by cmd/bench)")

# CLI -> pipeline
arrow([(620, 260), (620, 370)])
arrow([(1680, 260), (1680, 370)])
# Inputs -> pipeline
arrow([(440, 440), (520, 440)])

# Outputs
box(1880, 340, 460, 650, "results/<capture>/", [
    "", "plan.json", "  rooms, walls, openings,", "  ceilings, areas, each", "  with a 90% interval",
    "", "plan.svg", "  dimensioned drawing", "  + damage markers", "", "debug_raster.png",
    "drift.json", "plan_drift_off.json / .svg", "", "damage/detections.json", "export/  (video, photo)"], IO)
arrow([(1740, 440), (1880, 440)])
label(1765, 405, "writes")

# Python process
py_y = 1130
region(480, py_y, 1300, 330, "Python process  (ml/.venv, PyTorch, GPU if present)", PY_REGION, bottom=True)
pyb = [
    ("video_recon.py", ["frames \u2192 upright", "VGGT poses + depth", "Depth Pro metric scale", "writes export/"]),
    ("photo_recon.py", ["VGGT per room", "SIFT links, joint VGGT", "spanning-tree layout", "writes export/"]),
    ("vggt_runner.py", ["shared VGGT loader", "fp16 aggregator,", "fp32 heads (4 GB GPU)", ""]),
    ("damage_detect.py", ["OWLv2 open-vocabulary", "crack / stain prompts", "+ distractor prompts",
                          "writes detections.json"]),
]
for x, (t, ls) in zip(xs, pyb):
    box(x, py_y + 50, cw, 240, t, ls, PY)

# pipeline -> python (subprocess), left channel
arrow([(500, 510), (500, py_y + 2)], color=PY[1])
# python -> go (files)
arrow([(xs[0] + 145, py_y + 50), (xs[0] + 145, 960)], color=GO[1])
arrow([(xs[1] + 145, py_y + 50), (xs[1] + 145, 1080), (xs[0] + 200, 1080), (xs[0] + 200, 960)], color=GO[1])
arrow([(xs[3] + 145, py_y + 50), (xs[3] + 145, 960)], color=GO[1])
label(xs[0] + 215, 1062, "export/ in the LiDAR scan format", GO[1])
label(xs[3] - 112, 1062, "detections.json", GO[1])
# vggt_runner used by both recon scripts
d.line([(xs[2], py_y + 170), (xs[1] + cw, py_y + 170)], fill=PY[1], width=3)
head(xs[1] + cw, py_y + 170, -1, 0, PY[1], 9)

# Weights
box(1880, py_y, 460, 330, "Model weights", [
    "", "VGGT-1B (Meta)", "Depth Pro (Apple)", "OWLv2 (Google)", "Depth Anything V2",
    "  (orientation only)", "", "fetch_weights.py downloads", "them once (Hugging Face)"], NOTE, mono=False)
arrow([(1880, py_y + 165), (1780, py_y + 165)], color=MUTED)

# How they talk
box(60, py_y, 380, 330, "How Go calls Python", [
    "", "Go runs  python ml/<script>.py", "as a subprocess.", "",
    "Python only writes files;", "Go reads them back.", "No server, no shared memory.", "",
    "Cached by an input stamp;", "-live forces a rerun."], NOTE, mono=False)
label(512, 1092, "subprocess", PY[1])

# Key decisions
ky = 1530
d.line([(60, ky - 20), (W - 60, ky - 20)], fill=(210, 212, 215), width=2)
d.text((60, ky), "Key architecture decisions", font=F_HEAD_TXT, fill=INK)
decisions = [
    "1. One data format: video and photo are turned into a LiDAR-style export, so all three tiers share one geometry path.",
    "2. Go for geometry (fast, deterministic, single binary); Python only where the neural networks live.",
    "3. Every number leaves through internal/output with a 90% interval, and plan.json is checked against schema/plan.schema.json.",
]
for i, t in enumerate(decisions):
    d.text((60, ky + 42 + i * 30), t, font=F_BODY, fill=INK)

os.makedirs(os.path.dirname(OUT), exist_ok=True)
img.save(OUT)
print("wrote", os.path.normpath(OUT))
