"""Annotate a debug raster (scan run -debug) with numbered callouts and a
legend explaining every colour. Example points are chosen automatically:
for each colour, a pixel of that colour near the middle of where it
occurs (well inside filled areas), kept apart from other callouts.
Colours absent from a capture are listed as "not present here".

usage: python scripts/explain_raster.py <debug_raster.png> <out.png> [title]
"""
import sys

import numpy as np
from PIL import Image, ImageDraw, ImageFont
from scipy import ndimage

src, out = sys.argv[1], sys.argv[2]
title = sys.argv[3] if len(sys.argv) > 3 else src
img = Image.open(src).convert("RGB")
# Video/photo rasters use 5 cm cells and come out small: enlarge them
# (nearest neighbour, pixels stay sharp) so callouts do not pile up.
scale = 1
if img.size[1] < 900:
    scale = -(-900 // img.size[1])
    img = img.resize((img.size[0] * scale, img.size[1] * scale), Image.NEAREST)
px = np.asarray(img).astype(int)
H, W = px.shape[:2]

PASTELS = [(166, 206, 227), (178, 223, 138), (251, 154, 153), (253, 191, 111), (202, 178, 214),
           (255, 255, 153), (141, 211, 199), (252, 205, 229), (204, 235, 197)]


def mask_of(colors):
    m = np.zeros((H, W), bool)
    for c in colors:
        m |= np.all(px == np.array(c), axis=2)
    return m


def inside_plan():
    """Pixels enclosed by the plan: within the room area's bounding rows
    and columns on both sides (a cheap 'surrounded' test)."""
    rooms = mask_of(PASTELS) | mask_of([(0, 0, 0), (60, 60, 60), (0, 120, 0), (0, 60, 200)])
    left = np.maximum.accumulate(rooms, axis=1)
    right = np.maximum.accumulate(rooms[:, ::-1], axis=1)[:, ::-1]
    up = np.maximum.accumulate(rooms, axis=0)
    down = np.maximum.accumulate(rooms[::-1], axis=0)[::-1]
    return left & right & up & down


# (colours, kind, title, explanation); kind: "line" or "fill"
ITEMS = [
    ([(230, 0, 0)], "line", "Red line / dots", "Camera positions: a line along a LiDAR or video walk, separate dots where photos were taken"),
    (PASTELS, "fill", "Pastel fill", "One detected room; each room gets its own colour"),
    ([(0, 0, 0)], "line", "Thin black line", "Room outline snapped onto a measured wall surface"),
    ([(150, 150, 150)], "line", "Thin grey line", "Inferred room edge: no wall measured there (open plan, unscanned, or a fallback rectangle)"),
    ([(60, 60, 60)], "line", "Dark grey", "Wall cell (points fill at least half of the 0.3-2.0 m height band) that is not on a long straight run: thick walls, corners, clutter"),
    ([(0, 120, 0)], "line", "Green", "Wall on a straight left-right run of 30 cm or more"),
    ([(0, 60, 200)], "line", "Blue", "Wall on a straight up-down run of 30 cm or more"),
    ([(220, 0, 220)], "line", "Magenta bar", "Gap in a wall (0.5-1.6 m) WITH wall seen above it (header): a door or window"),
    ([(255, 170, 0)], "line", "Orange bar", "Gap in a wall (0.5-1.6 m) with nothing seen above it: an opening, or not scanned"),
    ([(215, 215, 215)], "fill", "Mid grey", "Furniture / objects between floor and 2 m (also seen outside, through windows)"),
    ([(235, 235, 235)], "fill", "Light grey", "Floor that was seen but is not part of any room"),
    ([(255, 255, 255)], "hole", "White inside the plan", "Nothing observed (e.g. a closed cupboard or a room never entered)"),
]


def pick(colors, kind, taken):
    m = mask_of(colors)
    if kind == "hole":
        m &= inside_plan()
    if kind in ("fill", "hole"):
        d = ndimage.distance_transform_edt(m)
        if d.max() >= 6:
            m = d >= min(12, d.max() / 2)  # well inside the area
    ys, xs = np.nonzero(m)
    if len(xs) < 20:
        return None
    cx, cy = np.median(xs), np.median(ys)
    order = np.argsort((xs - cx) ** 2 + (ys - cy) ** 2)
    for k in order[:: max(1, len(order) // 4000)]:
        x, y = int(xs[k]), int(ys[k])
        if all((x - tx) ** 2 + (y - ty) ** 2 > 70 ** 2 for tx, ty in taken):
            return x, y
    return int(xs[order[0]]), int(ys[order[0]])


def font(size, bold=False):
    for name in (("arialbd.ttf" if bold else "arial.ttf"), "DejaVuSans.ttf"):
        try:
            return ImageFont.truetype(name, size)
        except OSError:
            pass
    return ImageFont.load_default()


PANEL = 760
canvas = Image.new("RGB", (W + PANEL, max(H, 1500)), "white")
canvas.paste(img, (0, 0))
d = ImageDraw.Draw(canvas)
f_title, f_head, f_text, f_num = font(30, True), font(21, True), font(19), font(20, True)
d.text((W + 30, 24), "How to read this picture", font=f_title, fill=(20, 20, 20))
d.text((W + 30, 66), f"Top-down view: {title}", font=f_text, fill=(60, 60, 60))
d.text((W + 30, 92), "Made by:  bin/scan run <capture> -debug", font=f_text, fill=(60, 60, 60))
d.text((W + 30, 118), "Numbers point to an example of each colour." + (f" Enlarged {scale}x." if scale > 1 else ""),
       font=f_text, fill=(60, 60, 60))

y, taken = 170, []
for n, (colors, kind, head, text) in enumerate(ITEMS, 1):
    p = pick(colors, kind, taken)
    swatch = colors[0] if p is None else tuple(px[p[1], p[0]])
    d.ellipse((W + 30, y, W + 62, y + 32), fill=(20, 20, 20) if p else (170, 170, 170))
    d.text((W + 46, y + 16), str(n), font=f_num, fill="white", anchor="mm")
    d.rectangle((W + 76, y + 4, W + 110, y + 28), fill=swatch, outline=(120, 120, 120))
    d.text((W + 124, y), head + ("" if p else "  (not present here)"), font=f_head,
           fill=(20, 20, 20) if p else (150, 150, 150))
    words, line, ly = text.split(), "", y + 28
    for w in words:
        trial = (line + " " + w).strip()
        if d.textlength(trial, font=f_text) > PANEL - 150:
            d.text((W + 124, ly), line, font=f_text, fill=(70, 70, 70))
            line, ly = w, ly + 24
        else:
            line = trial
    d.text((W + 124, ly), line, font=f_text, fill=(70, 70, 70))
    y = ly + 40
    if p is None:
        continue
    taken.append(p)
    x, yy = p
    bx, by = min(x + 28, W - 20), max(yy - 28, 20)
    d.line((x, yy, bx, by), fill=(20, 20, 20), width=3)
    d.ellipse((x - 6, yy - 6, x + 6, yy + 6), outline=(20, 20, 20), width=3)
    d.ellipse((bx - 17, by - 17, bx + 17, by + 17), fill=(255, 230, 0), outline=(20, 20, 20), width=3)
    d.text((bx, by), str(n), font=f_num, fill=(20, 20, 20), anchor="mm")

canvas.save(out)
print("wrote", out, canvas.size)
