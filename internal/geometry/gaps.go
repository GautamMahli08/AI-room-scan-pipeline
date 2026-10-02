package geometry

import "math/bits"

// Gap is a break in a straight run of wall cells: a candidate opening.
// Along is the wall direction; the gap spans [A0, A1) along it and
// [B0, B1) across it (wall thickness), in cells.
type Gap struct {
	Orient         Orientation // OrientH (wall along x) or OrientV (along y)
	A0, A1, B0, B1 int
	HeaderFill     float64 // fraction of gap columns with points in the header band
	FloorFill      float64 // fraction of gap cells where floor was observed
	LowFill        float64 // fraction of gap columns with points low in the wall band (sills)
}

// Width returns the gap length along the wall in metres.
func (g Gap) Width(res float64) float64 { return float64(g.A1-g.A0) * res }

// Header band: door heads sit at ~2.0-2.1 m; above that a corridor is open
// to the ceiling. Low band: window sills.
const (
	headerLo = 2.05
	headerHi = 2.35
	lowLo    = 0.30
	lowHi    = 0.80
)

// FindGaps scans every row (for H walls) and column (for V walls) for
// breaks between wall cells of the matching orientation whose length is in
// [minGap, maxGap] metres, then merges breaks on adjacent rows into
// rectangles.
func FindGaps(r *Rasters, orient []Orientation, minGap, maxGap float64) []Gap {
	var out []Gap
	for _, o := range []Orientation{OrientH, OrientV} {
		alongN, acrossN := r.W, r.H
		idx := func(a, b int) int { return r.Idx(a, b) }
		if o == OrientV {
			alongN, acrossN = r.H, r.W
			idx = func(a, b int) int { return r.Idx(b, a) }
		}
		lo, hi := int(minGap/r.Res+0.5), int(maxGap/r.Res+0.5)

		type run struct{ a0, a1 int }
		// Thicken walls across by ±3 cells so collinear pieces that are
		// offset by a few cells (noise, slight drift) share rows.
		const thick = 3
		on := make([]bool, alongN*acrossN)
		for b := 0; b < acrossN; b++ {
			for a := 0; a < alongN; a++ {
				if orient[idx(a, b)]&o == 0 {
					continue
				}
				for d := max(0, b-thick); d <= min(acrossN-1, b+thick); d++ {
					on[d*alongN+a] = true
				}
			}
		}

		var open []Gap // rectangles still growing across rows
		for b := 0; b < acrossN; b++ {
			var runs []run
			last := -1
			for a := 0; a < alongN; a++ {
				if !on[b*alongN+a] {
					continue
				}
				if last >= 0 && a-last-1 >= lo && a-last-1 <= hi {
					runs = append(runs, run{last + 1, a})
				}
				last = a
			}
			// Extend open rectangles whose ends match a run on this row
			// within 3 cells; close the others.
			var next []Gap
			used := make([]bool, len(runs))
			for _, g := range open {
				matched := false
				for i, rn := range runs {
					if !used[i] && abs(rn.a0-g.A0) <= 3 && abs(rn.a1-g.A1) <= 3 {
						g.A0, g.A1 = min(g.A0, rn.a0), max(g.A1, rn.a1)
						g.B1 = b + 1
						used[i], matched = true, true
						break
					}
				}
				if matched {
					next = append(next, g)
				} else {
					out = append(out, g)
				}
			}
			for i, rn := range runs {
				if !used[i] {
					next = append(next, Gap{Orient: o, A0: rn.a0, A1: rn.a1, B0: b, B1: b + 1})
				}
			}
			open = next
		}
		out = append(out, open...)
	}
	for i := range out {
		r.scoreGap(&out[i])
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// scoreGap measures header, sill and floor evidence over the gap,
// widening it across the wall by 3 cells each side so the head of a door
// seen only from one side still counts.
func (r *Rasters) scoreGap(g *Gap) {
	head, low := bandMask(headerLo, headerHi), bandMask(lowLo, lowHi)
	var nHead, nLow, nFloor, nCells int
	for a := g.A0; a < g.A1; a++ {
		var h, l bool
		for b := g.B0 - 3; b < g.B1+3; b++ {
			x, y := a, b
			if g.Orient == OrientV {
				x, y = b, a
			}
			if x < 0 || y < 0 || x >= r.W || y >= r.H {
				continue
			}
			i := r.Idx(x, y)
			h = h || bits.OnesCount64(r.Heights[i]&head) > 0
			l = l || bits.OnesCount64(r.Heights[i]&low) >= 3
			if b >= g.B0 && b < g.B1 {
				nCells++
				if r.Floor[i] > 0 {
					nFloor++
				}
			}
		}
		if h {
			nHead++
		}
		if l {
			nLow++
		}
	}
	n := float64(g.A1 - g.A0)
	g.HeaderFill, g.LowFill = float64(nHead)/n, float64(nLow)/n
	if nCells > 0 {
		g.FloorFill = float64(nFloor) / float64(nCells)
	}
}
