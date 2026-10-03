"""Annotate a debug raster (scan run -debug) with numbered callouts and a
legend explaining every colour.

usage: python scripts/explain_raster.py <debug_raster.png> <out.png>
"""
import sys

import numpy as np
from PIL import Image, ImageDraw, ImageFont

src, out = sys.argv[1], sys.argv[2]
img = Image.open(src).convert("RGB")
px = np.asarray(img)
W, H = img.size


def font(size, bold=False):
    for name in (("arialbd.ttf" if bold else "arial.ttf"), "DejaVuSans.ttf"):
        try:
            return ImageFont.truetype(name, size)
        except OSError:
            pass
    return ImageFont.load_default()


def near(color, approx, radius=80):
    """Pixel of exactly `color` closest to `approx` (x, y)."""
    ax, ay = approx
    x0, y0 = max(0, ax - radius), max(0, ay - radius)
    win = px[y0:ay + radius, x0:ax + radius]
    ys, xs = np.nonzero(np.all(win == color, axis=2))
    if len(xs) == 0:
        return None
    k = np.argmin((xs + x0 - ax) ** 2 + (ys + y0 - ay) ** 2)
    return int(xs[k] + x0), int(ys[k] + y0)


# (colour in the raster, a spot near a good example, title, explanation)
ITEMS = [
    ((230, 0, 0), (880, 780), "Red line", "The path the person walked with the phone (camera position)"),
    ((251, 154, 153), (760, 760), "Pastel fill", "One detected room; each room gets its own colour"),
    ((0, 0, 0), (900, 606), "Thin black line", "Final room outline, snapped onto a measured wall surface"),
    ((150, 150, 150), (526, 800), "Thin grey line", "Inferred room edge: no wall seen there (open plan / unscanned)"),
    ((60, 60, 60), (400, 1180), "Dark grey", "Wall cell (points fill at least half of the 0.3-2.0 m height band) that is not on a long straight run: thick walls, corners, clutter"),
    ((0, 120, 0), (300, 712), "Green", "Wall on a straight left-right run of 30 cm or more"),
    ((0, 60, 200), (945, 760), "Blue", "Wall on a straight up-down run of 30 cm or more"),
    ((220, 0, 220), (640, 505), "Magenta bar", "Gap in a wall (0.5-1.6 m) WITH wall seen above it (header): a door or window"),
    ((255, 170, 0), (715, 318), "Orange bar", "Gap in a wall (0.5-1.6 m) with nothing seen above it: an opening, or not scanned"),
    ((215, 215, 215), (1250, 470), "Mid grey", "Furniture / objects between floor and 2 m (also seen outside, through windows)"),
    ((235, 235, 235), (1330, 420), "Light grey", "Floor that was seen but is not part of any room"),
    ((255, 255, 255), (650, 1120), "White inside the plan", "Nothing observed (e.g. a closed cupboard or a room never entered)"),
]

PANEL = 760
canvas = Image.new("RGB", (W + PANEL, max(H, 1500)), "white")
canvas.paste(img, (0, 0))
d = ImageDraw.Draw(canvas)
f_title, f_head, f_text, f_num = font(30, True), font(21, True), font(19), font(20, True)

d.text((W + 30, 24), "How to read this picture", font=f_title, fill=(20, 20, 20))
d.text((W + 30, 66), "Top-down view of single_scan_with_ceiling (whole flat),", font=f_text, fill=(60, 60, 60))
d.text((W + 30, 92), "made by:  bin/scan run single_scan_with_ceiling -debug", font=f_text, fill=(60, 60, 60))
d.text((W + 30, 118), "1 pixel = 1 cm. Numbers point to an example of each colour.", font=f_text, fill=(60, 60, 60))

y = 170
for n, (color, approx, title, text) in enumerate(ITEMS, 1):
    p = near(np.array(color), approx)
    # legend row: number badge, colour swatch, title + text
    d.ellipse((W + 30, y, W + 62, y + 32), fill=(20, 20, 20))
    d.text((W + 46, y + 16), str(n), font=f_num, fill="white", anchor="mm")
    d.rectangle((W + 76, y + 4, W + 110, y + 28), fill=color, outline=(120, 120, 120))
    d.text((W + 124, y), title, font=f_head, fill=(20, 20, 20))
    # wrap the explanation to the panel width
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
    # callout on the picture
    if p is not None:
        x, yy = p
        bx, by = x + 28, yy - 28
        d.line((x, yy, bx, by), fill=(20, 20, 20), width=3)
        d.ellipse((x - 6, yy - 6, x + 6, yy + 6), outline=(20, 20, 20), width=3)
        d.ellipse((bx - 17, by - 17, bx + 17, by + 17), fill=(255, 230, 0), outline=(20, 20, 20), width=3)
        d.text((bx, by), str(n), font=f_num, fill=(20, 20, 20), anchor="mm")
    else:
        print(f"no pixel of colour {color} near {approx} for item {n}")

d.text((W + 30, y + 10), "Final plan for comparison: results/single_scan_with_ceiling/plan.svg", font=f_text, fill=(60, 60, 60))
canvas.save(out)
print("wrote", out, canvas.size)
