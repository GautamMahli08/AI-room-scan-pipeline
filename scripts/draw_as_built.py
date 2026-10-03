"""Draw the as-built picture (what exists today, with status and measured numbers)
to docs/img/as_built.png.

Usage: ml/.venv/Scripts/python.exe scripts/draw_as_built.py
Numbers come from docs/BENCHMARK.md; update both together.
"""
import os
from PIL import Image, ImageDraw, ImageFont

W, H = 2400, 1660
OUT = os.path.join(os.path.dirname(__file__), "..", "docs", "img", "as_built.png")

BG = (250, 250, 247)
INK = (34, 38, 46)
MUTED = (96, 104, 116)
RULE = (210, 212, 215)
OK = ((224, 242, 226), (52, 140, 74))       # working
PART = ((255, 240, 214), (200, 130, 20))    # runs, limited accuracy
NO = ((236, 236, 238), (140, 140, 150))     # not built / not possible
NEUTRAL = ((228, 236, 250), (70, 110, 180))


def font(size, bold=False):
    for n in (["segoeuib.ttf", "arialbd.ttf"] if bold else ["segoeui.ttf", "arial.ttf"]):
        for d in ["C:/Windows/Fonts", "/usr/share/fonts/truetype/dejavu"]:
            p = os.path.join(d, n)
            if os.path.exists(p):
                return ImageFont.truetype(p, size)
    return ImageFont.load_default()


F_TITLE = font(46, True)
F_SUB = font(24)
F_SEC = font(24, True)
F_HEAD = font(25, True)
F_BODY = font(20)
F_BADGE = font(17, True)

img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)


def box(x, y, w, h, title, lines, style, badge=None):
    fill, edge = style
    d.rounded_rectangle([x, y, x + w, y + h], radius=16, fill=fill, outline=edge, width=3)
    d.text((x + 18, y + 12), title, font=F_HEAD, fill=INK)
    if badge:
        tw = d.textlength(badge, font=F_BADGE)
        bx = x + w - tw - 34
        d.rounded_rectangle([bx, y + 14, x + w - 14, y + 42], radius=12, fill=edge)
        d.text((bx + 10, y + 17), badge, font=F_BADGE, fill=(255, 255, 255))
    ty = y + 52
    for ln in lines:
        d.text((x + 20, ty), ln, font=F_BODY, fill=INK)
        ty += 28


def arrow(x0, y, x1, color=INK):
    d.line([(x0, y), (x1 - 4, y)], fill=color, width=4)
    d.polygon([(x1, y), (x1 - 20, y - 11), (x1 - 20, y + 11)], fill=color)


def section(y, text):
    d.text((60, y), text, font=F_SEC, fill=MUTED)
    d.line([(60, y + 38), (W - 60, y + 38)], fill=RULE, width=2)


# Title
d.text((60, 36), "Room Scan Pipeline: what is built (as of 3 Oct 2026)", font=F_TITLE, fill=INK)
d.text((62, 98), "Every box below exists in the repository and runs. Colour = how well it works on the sample "
       "captures; numbers are from docs/BENCHMARK.md.", font=F_SUB, fill=MUTED)

# --- Three tiers -----------------------------------------------------------
section(150, "THREE INPUT TIERS  \u2192  one command:  scan run <capture>")

cols = [(60, 330), (440, 470), (960, 560), (1570, 770)]
lanes = [
    ("LiDAR scan", ["iPhone Pro, Stray Scanner", "depth + poses + confidence"],
     "Native ingest + fusion  (Go)", ["depth and poses used as-is", "voxel fusion, integer sums", "\u2192 same input, same plan"],
     "Plan extraction  (Go)", ["floor, Manhattan walls, rooms", "doors / windows by gap test", "ceiling height per room"],
     "LiDAR result", ["6 rooms + corridor on the property scans", "13\u201316 openings found and classified",
                      "ceilings 2.26\u20133.08 m (\u00b11.2 cm model)", "rerun same capture: 20/20 walls identical"],
     OK, "WORKING"),
    ("Video", ["any phone, .mp4 / .MOV", "rotation tag \u2192 upright"],
     "VGGT + Depth Pro  (Python)", ["VGGT on 8-frame chunks", "Depth Pro gives metric scale", "chunks chained into one path"],
     "Plan extraction  (Go)", ["same code as LiDAR", "walls fitted where seen,", "else rectangle of seen floor"],
     "Video result", ["sample: footprint 13.1 vs 25.0 m\u00b2 (\u221248%)", "90% interval [10.5, 28.8] contains truth",
                      "own iPhone video: 3 rooms, 43.9 m\u00b2", "error = drift when chaining chunks"],
     PART, "RUNS, NOT ACCURATE"),
    ("Photos", ["any phone, folder per room", "overlap between rooms"],
     "Per-room VGGT  (Python)", ["VGGT per room folder", "SIFT links between rooms", "spanning-tree layout"],
     "Plan extraction  (Go)", ["same code as LiDAR", "rooms placed by links", "rectangle fallback"],
     "Photo result", ["one room alone: cameras within 16 cm", "full set: 4 of 6 rooms linked",
                      "footprint \u221248%; interval contains truth", "placement wrong when geometry is"],
     PART, "RUNS, NOT ACCURATE"),
]
lh = 172
for i, (t0, l0, t1, l1, t2, l2, t3, l3, st, badge) in enumerate(lanes):
    y = 210 + i * (lh + 22)
    box(cols[0][0], y, cols[0][1], lh, t0, l0, NEUTRAL)
    box(cols[1][0], y, cols[1][1], lh, t1, l1, OK if i == 0 else PART)
    box(cols[2][0], y, cols[2][1], lh, t2, l2, OK if i == 0 else PART)
    box(cols[3][0], y, cols[3][1], lh, t3, l3, st, badge)
    for (xa, wa), (xb, _) in zip(cols, cols[1:]):
        arrow(xa + wa, y + lh // 2, xb)

# --- Shared stages -----------------------------------------------------------
sy = 210 + 3 * (lh + 22) + 20
section(sy, "SHARED BY EVERY TIER")
sy += 60
sw, sh, gap = 555, 210, 20
shared = [
    ("Drift correction", ["plane-anchored, per 4 s chunk", "strength picked on held-out data",
                          "two scans of the house differ", "4.8% off \u2192 1.2% on"], OK, "DONE"),
    ("Calibrated 90% intervals", ["fitted on repeat captures", "coverage 29% \u2192 93% (88% held out)",
                                 "wall length \u00b116.7 cm, area \u00b115.4%", "on every number in plan.json"], OK, "DONE"),
    ("Damage detection", ["OWLv2, \u2265 2 views, on a surface", "found the real bathroom crack",
                          "2 of 3 captures; 1 partly wrong", "3 concealed-damage rules"], PART, "WORKS, LIMITED"),
    ("Outputs", ["plan.json (schema-checked)", "plan.svg: dimensions + damage", "debug raster + labelled copy",
                 "drift on / off versions"], OK, "DONE"),
]
for i, (t, ls, st, b) in enumerate(shared):
    box(60 + i * (sw + gap), sy, sw, sh, t, ls, st, b)

# --- Evaluation ------------------------------------------------------------
ey = sy + sh + 30
section(ey, "PROOF AND EVALUATION")
ey += 60
ew, eh = 555, 210
evals = [
    ("Benchmark tools  (cmd/bench)", ["repeat: wall-by-wall agreement", "overlay: two plans on one image",
                                      "calibrate: fit the intervals", "scripts/run_samples.sh reruns all"], OK, "DONE"),
    ("Fix loop  (fixloop/)", ["prediction committed before fix", "same-input repeat 8/14 \u2192 20/20",
                              "cross-capture 21%: gate still fails", "honest post-mortem (a curtain)"], OK, "DONE"),
    ("Tests", ["Go unit tests: all pass", "synthetic room: 4.000 \u00d7 3.000 m,",
               "door 0.900 m, angle 20.000\u00b0", "damage rules + schema tests"], OK, "PASS"),
    ("Documentation", ["README, SYSTEM_DESIGN", "capture protocol, device matrix",
                       "benchmark, technical report", "compliance, future scope"], OK, "DONE"),
]
for i, (t, ls, st, b) in enumerate(evals):
    box(60 + i * (ew + gap), ey, ew, eh, t, ls, st, b)

# --- Not built ---------------------------------------------------------------
ny = ey + eh + 30
section(ny, "NOT BUILT / NOT POSSIBLE WITH THE DATA AVAILABLE")
ny += 58
nots = ["laser ground truth (none exists)", "head-to-head vs Polycam / magicplan",
        "staged damage captures", "accurate video / photo walls", "furniture (not in the brief)"]
x = 60
for t in nots:
    tw = d.textlength(t, font=F_BODY)
    d.rounded_rectangle([x, ny, x + tw + 36, ny + 44], radius=14, fill=NO[0], outline=NO[1], width=2)
    d.text((x + 18, ny + 9), t, font=F_BODY, fill=INK)
    x += tw + 36 + 16
d.text((60, ny + 58), "Plans for these are in docs/FUTURE_SCOPE.md.", font=F_BODY, fill=MUTED)

# Legend
ly = H - 60
x = 60
for st, t in [(OK, "works"), (PART, "runs end to end, accuracy limited"), (NO, "not built"),
              (NEUTRAL, "input")]:
    d.rounded_rectangle([x, ly, x + 40, ly + 24], radius=6, fill=st[0], outline=st[1], width=3)
    d.text((x + 52, ly - 1), t, font=F_BODY, fill=INK)
    x += 52 + d.textlength(t, font=F_BODY) + 40
d.text((W - 60 - d.textlength("Design view: docs/img/high_level_design.png", font=F_BODY), ly - 1),
       "Design view: docs/img/high_level_design.png", font=F_BODY, fill=MUTED)

os.makedirs(os.path.dirname(OUT), exist_ok=True)
img.save(OUT)
print("wrote", os.path.normpath(OUT))
