// Package bench compares plans: two captures of the same space
// (repeatability), and a tier against a reference.
package bench

import (
	"fmt"
	"math"
	"sort"

	"roomscan/internal/geometry"
	"roomscan/internal/output"
)

// Transform maps plan B coordinates into plan A: rotate by Rot·90°, then
// translate.
type Transform struct {
	Rot    int
	Tx, Ty float64
}

func (t Transform) Apply(p geometry.Pt) geometry.Pt {
	x, y := p.X, p.Y
	for i := 0; i < t.Rot; i++ {
		x, y = -y, x
	}
	return geometry.Pt{X: x + t.Tx, Y: y + t.Ty}
}

// RoomMatch pairs a room of A with a room of B.
type RoomMatch struct {
	A, B   string
	IoU    float64
	AreaA  float64
	AreaB  float64
	Walls  []WallMatch
	CeilA  *float64
	CeilB  *float64
	DeltaA float64 // area difference, m²
}

// WallMatch pairs a wall of A with one of B (after transform).
type WallMatch struct {
	A, B           string
	LenA, LenB     float64
	Measured       bool    // both lengths measured (not inferred)
	Delta          float64 // LenB - LenA, metres
	RelDelta       float64 // Delta / LenA
	Pass           bool    // |Δ| <= 1 cm or <= 0.5 %
	OffsetMismatch float64 // distance between the two wall lines, metres
	SignedOffset   float64 // B's face minus A's, along A's outward normal
	// SameCorners: both endpoints coincide within cornerTol, i.e. the two
	// plans agree on where this wall starts and ends (same topology).
	SameCorners bool
}

const cornerTol = 0.06

// Result is a repeatability comparison.
type Result struct {
	Transform              Transform
	Overlap                float64 // IoU of the two footprints after registration
	Rooms                  []RoomMatch
	UnmatchedA, UnmatchedB []string
}

const gridRes = 0.05

func polys(p *output.Plan, t Transform) [][]geometry.Pt {
	var out [][]geometry.Pt
	for _, r := range p.Rooms {
		var poly []geometry.Pt
		for _, v := range r.Polygon {
			poly = append(poly, t.Apply(geometry.Pt{X: v[0], Y: v[1]}))
		}
		out = append(out, poly)
	}
	return out
}

// raster is a sparse set of occupied 5 cm cells.
type raster map[[2]int]bool

func rasterize(polys [][]geometry.Pt) raster {
	m := raster{}
	for _, poly := range polys {
		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, v := range poly {
			minX, minY = math.Min(minX, v.X), math.Min(minY, v.Y)
			maxX, maxY = math.Max(maxX, v.X), math.Max(maxY, v.Y)
		}
		for cy := int(math.Floor(minY / gridRes)); float64(cy)*gridRes <= maxY; cy++ {
			for cx := int(math.Floor(minX / gridRes)); float64(cx)*gridRes <= maxX; cx++ {
				c := geometry.Pt{X: (float64(cx) + 0.5) * gridRes, Y: (float64(cy) + 0.5) * gridRes}
				if geometry.PointInPolygon(c, poly) {
					m[[2]int{cx, cy}] = true
				}
			}
		}
	}
	return m
}

func iou(a, b raster) float64 {
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func centroid(poly []geometry.Pt) geometry.Pt {
	var a, cx, cy float64
	for i := range poly {
		j := (i + 1) % len(poly)
		c := poly[i].Cross(poly[j])
		a += c
		cx += (poly[i].X + poly[j].X) * c
		cy += (poly[i].Y + poly[j].Y) * c
	}
	return geometry.Pt{X: cx / (3 * a), Y: cy / (3 * a)}
}

// Register finds the transform of B into A maximising footprint overlap,
// trying every room-pair centroid alignment under each 90° rotation, then
// refines the translation on matched parallel walls.
func Register(a, b *output.Plan) (Transform, float64) {
	ra := rasterize(polys(a, Transform{}))
	pa := polys(a, Transform{})
	best, bestIoU := Transform{}, -1.0
	for rot := 0; rot < 4; rot++ {
		pb := polys(b, Transform{Rot: rot})
		for _, x := range pa {
			for _, y := range pb {
				ca, cb := centroid(x), centroid(y)
				t := Transform{Rot: rot, Tx: ca.X - cb.X, Ty: ca.Y - cb.Y}
				if v := iou(ra, rasterize(polys(b, t))); v > bestIoU {
					best, bestIoU = t, v
				}
			}
		}
	}
	// Refine: median offset of parallel walls within 30 cm, per axis.
	for it := 0; it < 3; it++ {
		var dx, dy []float64
		for _, wa := range walls(a, Transform{}) {
			for _, wb := range walls(b, best) {
				if wa.n.Dot(wb.n) < 0.99 {
					continue
				}
				d := wa.n.Dot(wb.mid) - wa.c
				if math.Abs(d) > 0.3 || overlap(wa, wb) < 0.3 {
					continue
				}
				if math.Abs(wa.n.X) > 0.9 {
					dx = append(dx, -d*wa.n.X)
				} else if math.Abs(wa.n.Y) > 0.9 {
					dy = append(dy, -d*wa.n.Y)
				}
			}
		}
		best.Tx += median(dx)
		best.Ty += median(dy)
	}
	return best, iou(ra, rasterize(polys(b, best)))
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64{}, v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

type wallLine struct {
	room, id  string
	a, b, mid geometry.Pt
	n         geometry.Pt // outward normal
	c         float64
	length    float64
	measured  bool
}

func walls(p *output.Plan, t Transform) []wallLine {
	var out []wallLine
	for _, r := range p.Rooms {
		for _, w := range r.Walls {
			a := t.Apply(geometry.Pt{X: w.Start[0], Y: w.Start[1]})
			b := t.Apply(geometry.Pt{X: w.End[0], Y: w.End[1]})
			d := b.Sub(a)
			l := d.Norm()
			if l == 0 {
				continue
			}
			n := geometry.Pt{X: d.Y / l, Y: -d.X / l} // CCW polygon: outward is right of travel
			out = append(out, wallLine{room: r.ID, id: w.ID, a: a, b: b, mid: a.Add(b).Scale(0.5), n: n, c: n.Dot(a),
				length: w.LengthM.Value, measured: !w.Inferred && w.LengthM.Source == "measured"})
		}
	}
	return out
}

// overlap is the shared extent of two parallel walls along their
// direction, as a fraction of the shorter one.
func overlap(a, b wallLine) float64 {
	u := a.b.Sub(a.a).Scale(1 / a.b.Sub(a.a).Norm())
	a0, a1 := 0.0, a.b.Sub(a.a).Dot(u)
	b0, b1 := b.a.Sub(a.a).Dot(u), b.b.Sub(a.a).Dot(u)
	if b0 > b1 {
		b0, b1 = b1, b0
	}
	ov := math.Min(a1, b1) - math.Max(a0, b0)
	short := math.Min(a1-a0, b1-b0)
	if short <= 0 {
		return 0
	}
	return math.Max(0, ov) / short
}

// Compare registers b onto a and matches rooms and walls.
func Compare(a, b *output.Plan) *Result {
	t, ov := Register(a, b)
	res := &Result{Transform: t, Overlap: ov}

	pa, pb := polys(a, Transform{}), polys(b, t)
	type cand struct {
		i, j int
		v    float64
	}
	var cands []cand
	for i := range pa {
		ri := rasterize([][]geometry.Pt{pa[i]})
		for j := range pb {
			if v := iou(ri, rasterize([][]geometry.Pt{pb[j]})); v >= 0.3 {
				cands = append(cands, cand{i, j, v})
			}
		}
	}
	sort.Slice(cands, func(x, y int) bool { return cands[x].v > cands[y].v })
	usedA, usedB := map[int]bool{}, map[int]bool{}
	wa, wb := walls(a, Transform{}), walls(b, t)
	for _, c := range cands {
		if usedA[c.i] || usedB[c.j] {
			continue
		}
		usedA[c.i], usedB[c.j] = true, true
		ra, rb := a.Rooms[c.i], b.Rooms[c.j]
		m := RoomMatch{A: ra.ID, B: rb.ID, IoU: c.v, AreaA: ra.FloorAreaM2.Value, AreaB: rb.FloorAreaM2.Value,
			CeilA: ra.CeilingHeightM.Value, CeilB: rb.CeilingHeightM.Value}
		m.DeltaA = m.AreaB - m.AreaA
		m.Walls = matchWalls(filter(wa, ra.ID), filter(wb, rb.ID))
		res.Rooms = append(res.Rooms, m)
	}
	for i, r := range a.Rooms {
		if !usedA[i] {
			res.UnmatchedA = append(res.UnmatchedA, r.ID)
		}
	}
	for j, r := range b.Rooms {
		if !usedB[j] {
			res.UnmatchedB = append(res.UnmatchedB, r.ID)
		}
	}
	return res
}

func filter(ws []wallLine, room string) []wallLine {
	var out []wallLine
	for _, w := range ws {
		if w.room == room {
			out = append(out, w)
		}
	}
	return out
}

// matchWalls pairs same-facing walls within 15 cm of each other that
// overlap by at least half the shorter one, closest first.
func matchWalls(wa, wb []wallLine) []WallMatch {
	type cand struct {
		i, j int
		d    float64
	}
	var cands []cand
	for i, a := range wa {
		for j, b := range wb {
			if a.n.Dot(b.n) < 0.99 {
				continue
			}
			d := math.Abs(a.n.Dot(b.mid) - a.c)
			if d <= 0.15 && overlap(a, b) >= 0.5 {
				cands = append(cands, cand{i, j, d})
			}
		}
	}
	sort.Slice(cands, func(x, y int) bool { return cands[x].d < cands[y].d })
	usedA, usedB := map[int]bool{}, map[int]bool{}
	var out []WallMatch
	for _, c := range cands {
		if usedA[c.i] || usedB[c.j] {
			continue
		}
		usedA[c.i], usedB[c.j] = true, true
		a, b := wa[c.i], wb[c.j]
		m := WallMatch{A: a.id, B: b.id, LenA: a.length, LenB: b.length, Measured: a.measured && b.measured,
			Delta: b.length - a.length, OffsetMismatch: c.d, SignedOffset: a.n.Dot(b.mid) - a.c}
		d1 := math.Min(a.a.Sub(b.a).Norm(), a.a.Sub(b.b).Norm())
		d2 := math.Min(a.b.Sub(b.a).Norm(), a.b.Sub(b.b).Norm())
		m.SameCorners = d1 <= cornerTol && d2 <= cornerTol
		m.RelDelta = m.Delta / a.length
		m.Pass = math.Abs(m.Delta) <= 0.01 || math.Abs(m.RelDelta) <= 0.005
		out = append(out, m)
	}
	sort.Slice(out, func(x, y int) bool { return out[x].A < out[y].A })
	return out
}

// Summary counts measured wall pairs and how many pass the gate.
func (r *Result) Summary() (pairs, pass int, medAbs float64) {
	var abs []float64
	for _, m := range r.Rooms {
		for _, w := range m.Walls {
			if !w.Measured {
				continue
			}
			pairs++
			if w.Pass {
				pass++
			}
			abs = append(abs, math.Abs(w.Delta))
		}
	}
	return pairs, pass, median(abs)
}

// SplitSummary is Summary separately for measured pairs whose corners
// coincide and pairs whose corners differ.
func (r *Result) SplitSummary() (same, sameP, diff, diffP int, sameMed, diffMed float64) {
	var sa, da []float64
	for _, m := range r.Rooms {
		for _, w := range m.Walls {
			if !w.Measured {
				continue
			}
			if w.SameCorners {
				same++
				if w.Pass {
					sameP++
				}
				sa = append(sa, math.Abs(w.Delta))
			} else {
				diff++
				if w.Pass {
					diffP++
				}
				da = append(da, math.Abs(w.Delta))
			}
		}
	}
	return same, sameP, diff, diffP, median(sa), median(da)
}

// Markdown renders the comparison as tables.
func (r *Result) Markdown(nameA, nameB string) string {
	pairs, pass, med := r.Summary()
	s := fmt.Sprintf("### %s vs %s\n\nRegistration: rotation %d×90°, translation (%.3f, %.3f) m, footprint IoU %.3f.  \n",
		nameA, nameB, r.Transform.Rot, r.Transform.Tx, r.Transform.Ty, r.Overlap)
	s += fmt.Sprintf("Measured wall pairs: **%d**, within gate (≤1 cm or ≤0.5%%): **%d** (%.0f%%), median |Δ| %.1f cm.\n\n",
		pairs, pass, 100*float64(pass)/math.Max(1, float64(pairs)), med*100)
	same, sameP, diff, diffP, sameMed, diffMed := r.SplitSummary()
	s += fmt.Sprintf("Same corners in both plans: %d pairs, %d pass, median |Δ| %.1f cm. Different corners: %d pairs, %d pass, median |Δ| %.1f cm.\n\n",
		same, sameP, sameMed*100, diff, diffP, diffMed*100)
	s += "| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |\n|---|---|---|---|---|---|---|\n"
	for _, m := range r.Rooms {
		s += fmt.Sprintf("| %s | %s | %.2f | %.2f | %.2f | %s | %s |\n", m.A, m.B, m.IoU, m.AreaA, m.AreaB, fmtPtr(m.CeilA), fmtPtr(m.CeilB))
	}
	s += "\n| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |\n|---|---|---|---|---|---|---|---|---|\n"
	for _, m := range r.Rooms {
		for _, w := range m.Walls {
			gate := "—"
			if w.Measured {
				gate = map[bool]string{true: "pass", false: "fail"}[w.Pass]
			}
			s += fmt.Sprintf("| %s/%s | %s | %s | %.3f | %.3f | %+.1f | %+.2f | %v | %s |\n",
				m.A, m.B, w.A, w.B, w.LenA, w.LenB, w.Delta*100, w.RelDelta*100, w.Measured, gate)
		}
	}
	if len(r.UnmatchedA)+len(r.UnmatchedB) > 0 {
		s += fmt.Sprintf("\nUnmatched rooms: A %v, B %v\n", r.UnmatchedA, r.UnmatchedB)
	}
	return s
}

func fmtPtr(v *float64) string {
	if v == nil {
		return "unobserved"
	}
	return fmt.Sprintf("%.3f", *v)
}
