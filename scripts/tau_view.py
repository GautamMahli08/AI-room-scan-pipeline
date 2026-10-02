"""Show the per-pair error term needed for each cross-capture wall pair
(diagnostic for the empirical calibration in internal/calib).

usage: python scripts/tau_view.py <uncal_results_dir>
"""
import io
import json
import math
import os
import subprocess
import sys

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")
U = sys.argv[1]
PAIRS = [("single_scan_floor_only", "single_scan_with_ceiling"), ("single_room", "single_scan_with_ceiling"),
         ("single_room", "single_scan_floor_only")]


def sig(m):
    return (m["ci90"][1] - m["ci90"][0]) / 2 / 1.645


need = []
for a, b in PAIRS:
    out = subprocess.run([os.path.abspath("bin/bench.exe" if os.name == "nt" else "bin/bench"), "repeat", "-json", f"{U}/{a}/plan.json", f"{U}/{b}/plan.json"],
                         capture_output=True, text=True).stdout
    d = json.loads(out)
    pa, pb = json.load(open(f"{U}/{a}/plan.json")), json.load(open(f"{U}/{b}/plan.json"))

    def wall(p, r, w):
        return next(x for R in p["rooms"] if R["id"] == r for x in R["walls"] if x["id"] == w)

    for rm in d["result"]["Rooms"]:
        for w in rm["Walls"]:
            if not w["Measured"]:
                continue
            sa, sb = sig(wall(pa, rm["A"], w["A"])["length_m"]), sig(wall(pb, rm["B"], w["B"])["length_m"])
            t = math.sqrt(max(0, (w["Delta"] / 1.645) ** 2 - sa * sa - sb * sb) / 2)
            need.append((t, w["Delta"], w["LenA"], w["SameCorners"]))
need.sort()
print(f"{'tau cm':>8} {'delta cm':>9} {'len m':>6}  same corners")
for t, dlt, ln, sc in need:
    print(f"{t * 100:8.1f} {dlt * 100:9.1f} {ln:6.2f}  {sc}")
