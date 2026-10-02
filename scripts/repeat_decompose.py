"""Decompose repeatability failures into face error vs topology error.

A wall's length runs between its two end walls (the previous and next
edges of the room outline). If both plans end the wall at the *same* end
walls, the length difference must equal the outward face shifts of those
two end walls (face error). If either end wall differs, the length
difference comes from where the outline is cut (topology error).

usage: python scripts/repeat_decompose.py <bench.json>   (from bench repeat -json)
"""
import io
import json
import statistics
import sys

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")


def main():
    d = json.load(open(sys.argv[1]))
    a, b = json.load(open(d["plan_a"])), json.load(open(d["plan_b"]))
    order = {}
    for p, tag in ((a, "A"), (b, "B")):
        for r in p["rooms"]:
            ids = [w["id"] for w in r["walls"]]
            for i, w in enumerate(ids):
                order[(tag, r["id"], w)] = (ids[i - 1], ids[(i + 1) % len(ids)])

    same, diff = [], []
    print(f"{'room':<8}{'wall':<11}{'Δ cm':>8}{'face-pred cm':>14}  class")
    for rm in d["result"]["Rooms"]:
        match = {w["A"]: w for w in rm["Walls"]}  # A wall id -> pair
        for w in rm["Walls"]:
            if not w["Measured"]:
                continue
            pa, na = order[("A", rm["A"], w["A"])]
            pb, nb = order[("B", rm["B"], w["B"])]
            ends_match = pa in match and na in match and {match[pa]["B"], match[na]["B"]} == {pb, nb}
            if ends_match:
                pred = match[pa]["SignedOffset"] + match[na]["SignedOffset"]
                same.append((w, pred))
                cls = "same end walls (face error)"
                ptxt = f"{pred*100:+.1f}"
            else:
                diff.append(w)
                cls = "different end walls (topology)"
                ptxt = "—"
            print(f"{rm['A']+'/'+rm['B']:<8}{w['A']+'/'+w['B']:<11}{w['Delta']*100:>+8.1f}{ptxt:>14}  {cls}")

    def stats(ws):
        if not ws:
            return "none"
        return (f"{len(ws)} pairs, {sum(w['Pass'] for w in ws)} pass, "
                f"median |Δ| {statistics.median(abs(w['Delta']) for w in ws)*100:.1f} cm")

    print(f"\nsame end walls:      {stats([w for w, _ in same])}")
    if same:
        resid = [abs(w["Delta"] - p) for w, p in same]
        print(f"  |Δ − face prediction| median {statistics.median(resid)*100:.1f} cm (face shifts explain these)")
    print(f"different end walls: {stats(diff)}")


main()
