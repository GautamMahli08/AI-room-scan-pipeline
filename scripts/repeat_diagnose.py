"""Root-cause evidence for the repeatability gate (fix loop, Part 4).

For each matched wall pair reported by `bench repeat -json`, classify what
terminates the wall in each plan: a short edge (< 0.5 m, a notch or jog),
an inferred edge (open boundary / unobserved), or a full-length measured
wall. Prints the gate pass rate per class and the face-offset agreement.

usage: python scripts/repeat_diagnose.py <bench.json>
"""
import io
import json
import statistics
import sys

SHORT = 0.5


def neighbours(plan):
    out = {}
    for r in plan["rooms"]:
        ws = r["walls"]
        for i, w in enumerate(ws):
            out[(r["id"], w["id"])] = (ws[i - 1], ws[(i + 1) % len(ws)])
    return out


def ends_kind(nb):
    kinds = []
    for n in nb:
        if n["inferred"]:
            kinds.append("inferred")
        elif n["length_m"]["value"] < SHORT:
            kinds.append("short")
        else:
            kinds.append("wall")
    return kinds


def main():
    d = json.load(open(sys.argv[1]))
    a, b = json.load(open(d["plan_a"])), json.load(open(d["plan_b"]))
    na, nb = neighbours(a), neighbours(b)
    rows = []
    for rm in d["result"]["Rooms"]:
        for w in rm["Walls"]:
            if not w["Measured"]:
                continue
            ka = ends_kind(na[(rm["A"], w["A"])])
            kb = ends_kind(nb[(rm["B"], w["B"])])
            clean = all(k == "wall" for k in ka + kb)
            rows.append((clean, w, ka, kb))
    print(f"{'pair':<16}{'Δ cm':>8}  {'gate':<5} ends A / ends B")
    for clean, w, ka, kb in rows:
        print(f"{w['A']+'/'+w['B']:<16}{w['Delta']*100:>8.1f}  {'pass' if w['Pass'] else 'fail':<5} {ka} / {kb}")
    for label, sel in (("both ends on full walls in both plans", True), ("an end on a short or inferred edge", False)):
        g = [w for c, w, _, _ in rows if c == sel]
        if g:
            print(f"\n{label}: {len(g)} pairs, {sum(w['Pass'] for w in g)} pass, "
                  f"median |Δ| {statistics.median(abs(w['Delta']) for w in g)*100:.1f} cm")
    offs = [w["OffsetMismatch"] for _, w, _, _ in rows]
    print(f"\nface position agreement (perpendicular offset between matched walls): "
          f"median {statistics.median(offs)*100:.1f} cm, max {max(offs)*100:.1f} cm")
    shorts = [sum(1 for r in p["rooms"] for x in r["walls"] if x["length_m"]["value"] < SHORT) for p in (a, b)]
    totals = [sum(len(r["walls"]) for r in p["rooms"]) for p in (a, b)]
    print(f"edges shorter than {SHORT} m: A {shorts[0]}/{totals[0]}, B {shorts[1]}/{totals[1]}")


def layer_test(d, a, b):
    """Would taking the outermost well-covered layer make faces agree?"""
    la, lb = a["diagnostics"]["wall_layers"], b["diagnostics"]["wall_layers"]

    def outer(layers):
        ok = [l for l in layers or [] if l["coverage"] >= 0.3]
        return max(l["offset_m"] for l in ok) if ok else 0.0

    now, alt, multi = [], [], 0
    for rm in d["result"]["Rooms"]:
        for w in rm["Walls"]:
            ka, kb = f"{rm['A']}/{w['A']}", f"{rm['B']}/{w['B']}"
            if ka not in la or kb not in lb:
                continue
            oa, ob = outer(la[ka]), outer(lb[kb])
            multi += (oa != 0) + (ob != 0)
            now.append(abs(w["SignedOffset"]))
            alt.append(abs(w["SignedOffset"] + ob - oa))
    print(f"\nlayer test over {len(now)} matched walls ({multi} faces where the densest layer is not the outermost covered one):")
    print(f"  face disagreement now (densest layer):        median {statistics.median(now)*100:.1f} cm, mean {statistics.mean(now)*100:.1f} cm")
    print(f"  if outermost layer with >=30% coverage taken: median {statistics.median(alt)*100:.1f} cm, mean {statistics.mean(alt)*100:.1f} cm")


sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")
if len(sys.argv) > 2 and sys.argv[2] == "--layers":
    dd = json.load(open(sys.argv[1]))
    layer_test(dd, json.load(open(dd["plan_a"])), json.load(open(dd["plan_b"])))
else:
    main()
