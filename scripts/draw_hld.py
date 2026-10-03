"""Draw the high-level design diagram to docs/img/high_level_design.png.

Usage: ml/.venv/Scripts/python.exe scripts/draw_hld.py
"""
import os
from PIL import Image, ImageDraw, ImageFont

W, H = 2400, 1480
OUT = os.path.join(os.path.dirname(__file__), "..", "docs", "img", "high_level_design.png")

BG = (250, 250, 247)
INK = (34, 38, 46)
MUTED = (96, 104, 116)
GO = (219, 234, 254)        # Go pipeline stages
GO_EDGE = (59, 110, 190)
PY = (254, 236, 210)        # Python / ML stages
PY_EDGE = (196, 120, 30)
IO = (226, 243, 226)        # inputs and outputs
IO_EDGE = (60, 140, 80)


def font(size, bold=False):
    names = ["segoeuib.ttf" if bold else "segoeui.ttf", "arialbd.ttf" if bold else "arial.ttf"]
    for n in names:
        for d in ["C:/Windows/Fonts", "/usr/share/fonts/truetype/dejavu"]:
            p = os.path.join(d, n)
            if os.path.exists(p):
                return ImageFont.truetype(p, size)
    return ImageFont.load_default()


F_TITLE = font(46, True)
F_SUB = font(24)
F_HEAD = font(28, True)
F_BODY = font(21)
F_COL = font(22, True)
F_LAB = font(19)

img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)


def box(x, y, w, h, title, lines, fill, edge):
    d.rounded_rectangle([x, y, x + w, y + h], radius=18, fill=fill, outline=edge, width=3)
    d.text((x + 20, y + 14), title, font=F_HEAD, fill=INK)
    ty = y + 58
    for ln in lines:
        d.text((x + 22, ty), ln, font=F_BODY, fill=INK)
        ty += 30
    return (x, y, w, h)


def head(px, py, dx, dy, color, size=14):
    # arrow head at (px, py) pointing along (dx, dy) unit direction
    lx, ly = -dy, dx
    d.polygon([(px, py),
               (px - dx * size * 1.6 + lx * size * 0.8, py - dy * size * 1.6 + ly * size * 0.8),
               (px - dx * size * 1.6 - lx * size * 0.8, py - dy * size * 1.6 - ly * size * 0.8)],
              fill=color)


def arrow(points, color=INK, width=4, label=None, label_at=None):
    d.line(points, fill=color, width=width, joint="curve")
    (x0, y0), (x1, y1) = points[-2], points[-1]
    n = max(abs(x1 - x0), abs(y1 - y0)) or 1
    head(x1, y1, (x1 - x0) / n, (y1 - y0) / n, color)
    if label:
        lx, ly = label_at
        tw = d.textlength(label, font=F_LAB)
        d.rectangle([lx - 6, ly - 2, lx + tw + 6, ly + 25], fill=BG)
        d.text((lx, ly), label, font=F_LAB, fill=MUTED)


# Title
d.text((60, 36), "Room Scan Pipeline: high-level design", font=F_TITLE, fill=INK)
d.text((62, 98), "Phone capture (LiDAR scan, video or photos)  \u2192  dimensioned floor plan with 90% intervals, "
       "drift correction and damage flags", font=F_SUB, fill=MUTED)

# Column headings
cols = [(60, "1. CAPTURE"), (520, "2. RECONSTRUCT"), (1000, "3. COMMON FORM"),
        (1380, "4. ANALYSE"), (1910, "5. OUTPUT")]
for x, t in cols:
    d.text((x, 160), t, font=F_COL, fill=MUTED)
d.line([(60, 196), (W - 60, 196)], fill=(210, 212, 215), width=2)

# 1. Inputs
iw, ih = 360, 210
inputs = [
    (240, "LiDAR scan", ["iPhone Pro (Stray Scanner)", "RGB + depth + confidence", "camera poses + intrinsics", "best accuracy"]),
    (560, "Video", ["any phone, .mp4 / .MOV", "walk through every room", "rotation tag read for", "upright frames"]),
    (880, "Photos", ["any phone", "folder per room", "overlap between rooms", "lowest accuracy"]),
]
for y, t, ls in inputs:
    box(60, y, iw, ih, t, ls, IO, IO_EDGE)

# 2. Reconstruction
rx, rw = 520, 400
recon = [
    (240, "Native ingest  (Go)", ["read depth + poses as-is", "metric scale from sensor", "drop low-confidence depth"], GO, GO_EDGE),
    (560, "VGGT + Depth Pro  (Python)", ["VGGT on 8-frame chunks", "\u2192 poses + depth", "Depth Pro fixes metric scale"], PY, PY_EDGE),
    (880, "Per-room VGGT  (Python)", ["VGGT per room folder", "SIFT links between rooms", "spanning-tree layout"], PY, PY_EDGE),
]
for y, t, ls, f, e in recon:
    box(rx, y, rw, ih, t, ls, f, e)
    arrow([(60 + iw, y + ih // 2), (rx, y + ih // 2)])

# 3. Common frame set + fused cloud
cx, cy, cw, ch = 1000, 240, 300, 850
box(cx, cy, cw, ch, "Frame set", [
    "one structure for", "every tier:", "",
    "\u2022 RGB image", "\u2022 depth map", "\u2022 confidence", "\u2022 camera pose", "\u2022 intrinsics", "",
    "fused into a voxel", "point cloud", "(integer sums \u2192", "same input, same plan)",
    "", "later stages never", "ask which tier it was",
], GO, GO_EDGE)
for y, *_ in recon:
    arrow([(rx + rw, y + ih // 2), (cx, y + ih // 2)])

# 4. Analysis stages
ax, aw = 1380, 440
geo = box(ax, 240, aw, 250, "Geometry  (Go)", [
    "floor plane + Manhattan angle", "wall faces, room outlines",
    "doors / windows (gap test)", "rooms split by free space", "ceiling height"], GO, GO_EDGE)
stc = box(ax, 530, aw, 170, "Stitch + drift  (Go)", [
    "plane-anchored correction", "per 4 s chunk; on/off ablation", "rooms placed on one plan"], GO, GO_EDGE)
cal = box(ax, 740, aw, 170, "Calibration  (Go)", [
    "90% intervals from", "cross-capture error", "length \u00b116.7 cm, area \u00b115.4%"], GO, GO_EDGE)
dmg = box(ax, 950, aw, 200, "Damage  (Python + Go)", [
    "OWLv2 detects cracks, stains", "\u2265 2 views, projected onto", "wall / floor / ceiling",
    "\u2192 concealed-damage rules"], PY, PY_EDGE)

arrow([(cx + cw, 365), (ax, 365)], label="cloud", label_at=(cx + cw + 14, 332))
arrow([(cx + cw, 1050), (ax, 1050)], label="frames", label_at=(cx + cw + 14, 1017))
# geometry -> stitch -> calibration
arrow([(ax + aw // 2, 490), (ax + aw // 2, 530)])
arrow([(ax + aw // 2, 700), (ax + aw // 2, 740)])
# geometry surfaces -> damage (left rail)
rail = ax - 30
d.line([(ax, 450), (rail, 450), (rail, 1000)], fill=MUTED, width=3)
head(ax, 1000, 1, 0, MUTED, 11)
d.line([(rail, 1000), (ax, 1000)], fill=MUTED, width=3)

# 5. Output
ox, oy, ow, oh = 1910, 240, 430, 910
box(ox, oy, ow, oh, "Results", [
    "results/<capture>/", "",
    "plan.json", "  rooms, walls, openings,", "  ceiling heights, areas,", "  every number with a", "  90% interval", "",
    "plan.svg", "  dimensioned drawing", "  + damage markers", "",
    "debug_raster.png", "  what the pipeline saw", "",
    "drift.json, plan_drift_off.*", "  drift on/off ablation",
], IO, IO_EDGE)
for y in (365, 615, 825, 1050):
    arrow([(ax + aw, y), (ox, y)])

# Bottom band: evaluation + legend
by = 1230
d.line([(60, by - 30), (W - 60, by - 30)], fill=(210, 212, 215), width=2)
box(60, by, 1240, 150, "Evaluation  (cmd/bench)", [
    "repeat: same property scanned twice \u2192 wall-by-wall agreement      calibrate: fit the 90% intervals",
    "overlay: compare two plans visually      fix loop (fixloop/): declared prediction \u2192 fix \u2192 measured result"],
    (238, 238, 245), (120, 120, 150))

lx, ly = 1380, by + 10
d.text((lx, ly), "Legend", font=F_HEAD, fill=INK)
for i, (f, e, t) in enumerate([(GO, GO_EDGE, "Go pipeline (cmd/scan, internal/)"),
                               (PY, PY_EDGE, "Python ML models (ml/), called by Go"),
                               (IO, IO_EDGE, "Inputs and outputs")]):
    yy = ly + 48 + i * 34
    d.rounded_rectangle([lx, yy, lx + 44, yy + 24], radius=6, fill=f, outline=e, width=3)
    d.text((lx + 60, yy - 2), t, font=F_BODY, fill=INK)

d.text((60, H - 56), "Command:  scan run <capture> [-tier lidar|video|photo] [-damage] [-drift]      "
       "Details: SYSTEM_DESIGN.md, docs/TECHNICAL_REPORT.md", font=F_LAB, fill=MUTED)

os.makedirs(os.path.dirname(OUT), exist_ok=True)
img.save(OUT)
print("wrote", os.path.normpath(OUT))
