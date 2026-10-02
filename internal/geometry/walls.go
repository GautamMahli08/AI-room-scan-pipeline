package geometry

import (
	"math"
	"sort"

	"roomscan/internal/geom"
)

// Line is an infinite 2D line {p : N·p = C} with unit normal N pointing
// out of the room.
type Line struct {
	N Pt
	C float64
}

func (l Line) intersect(m Line) (Pt, bool) {
	det := l.N.X*m.N.Y - l.N.Y*m.N.X
	if math.Abs(det) < 1e-9 {
		return Pt{}, false
	}
	return Pt{(l.C*m.N.Y - l.N.Y*m.C) / det, (l.N.X*m.C - l.C*m.N.X) / det}, true
}

// WallFit is the evidence behind one polygon edge.
type WallFit struct {
	Line     Line
	Axis     Orientation // OrientH, OrientV, or OrientOther
	Support  int         // wall points in the fitted face layer
	Spread   float64     // std of those points about the face, metres
	Coverage float64     // fraction of edge length with wall points at the face
	Inferred bool        // too little wall evidence: boundary from free space only
	Layers   []Layer     // all point layers near the face (diagnostics)
}

// RoomShape is a room outline with per-edge wall fits. Edge i runs from
// Corners[i] to Corners[i+1].
type RoomShape struct {
	Corners []Pt
	Walls   []WallFit
}

// WallPoints indexes wall-band points by plan raster cell for fast lookup.
type WallPoints struct {
	g     Grid2
	start []int32 // start[i]..start[i+1] are the points of cell i
	xy    []Pt
}

// IndexWallPoints projects points to the plan and keeps those in the wall
// band.
func IndexWallPoints(pts []geom.Vec3, f PlanFrame, g Grid2) *WallPoints {
	type cp struct {
		c int
		p Pt
	}
	var tmp []cp
	for _, p := range pts {
		x, y, h := f.ToPlan(p)
		if h < wallBandLo || h > wallBandHi {
			continue
		}
		if cx, cy, ok := g.Cell(x, y); ok {
			tmp = append(tmp, cp{g.Idx(cx, cy), Pt{x, y}})
		}
	}
	sort.Slice(tmp, func(i, j int) bool { return tmp[i].c < tmp[j].c })
	w := &WallPoints{g: g, start: make([]int32, g.W*g.H+1), xy: make([]Pt, len(tmp))}
	for i, t := range tmp {
		w.xy[i] = t.p
		w.start[t.c+1]++
	}
	for i := 1; i < len(w.start); i++ {
		w.start[i] += w.start[i-1]
	}
	return w
}

// inBox calls fn for every indexed point inside the axis-aligned box.
func (w *WallPoints) inBox(x0, y0, x1, y1 float64, fn func(p Pt)) {
	c0x, c0y, _ := w.g.Cell(x0, y0)
	c1x, c1y, _ := w.g.Cell(x1, y1)
	for cy := max(c0y, 0); cy <= min(c1y, w.g.H-1); cy++ {
		for cx := max(c0x, 0); cx <= min(c1x, w.g.W-1); cx++ {
			i := w.g.Idx(cx, cy)
			for _, p := range w.xy[w.start[i]:w.start[i+1]] {
				if p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
					fn(p)
				}
			}
		}
	}
}

// Wall fitting parameters.
const (
	snapAngle   = 15 * math.Pi / 180 // edges this close to an axis become axis-aligned
	faceSearch  = 0.15               // search this far into the room from the line for the face
	faceOutward = 0.04               // and this far outward (the line may sit just inside the face)
	faceBand    = 0.015              // points within ± this of the face peak define it
	endTrim     = 0.10               // ignore this much at each end (corners, adjacent walls)
	minCoverage = 0.30               // below this the edge is reported as inferred
	coverageBin = 0.05               // metres along the wall per coverage bin
	simplifyTol = 0.06               // Douglas-Peucker tolerance
	openRadius  = 0.15               // morphological opening before tracing
	minEdgeLen  = 0.12               // shorter edges are merged away
)

// FitRoom turns a room into a Manhattan-snapped outline whose edges
// sit on the measured wall faces.
//
// The outline comes from the wall-line arrangement (rectangles owned by the
// room), so every edge already lies on a detected wall line or its
// extension; fitting then moves each edge onto its measured face.
func FitRoom(arr *Arrangement, label int32, wp *WallPoints) *RoomShape {
	poly := removeSmallFeatures(dropCollinear(arr.Outline(label)), minFeature)
	if len(poly) < 3 {
		return nil
	}
	es := regularize(edgeLines(poly))
	if len(es) < 3 {
		return nil
	}
	lines := make([]Line, len(es))
	for i, e := range es {
		lines[i] = e.l
	}

	// First pass corners from the raster lines give each edge its extent.
	corners := intersectAll(lines)
	fits := make([]WallFit, len(lines))
	for i := range lines {
		a, b := corners[i], corners[(i+1)%len(corners)]
		fits[i] = fitFace(wp, lines[i], es[i].axis, a, b)
	}
	for i := range fits {
		lines[i] = fits[i].Line
	}
	axes := make([]Orientation, len(es))
	for i, e := range es {
		axes[i] = e.axis
	}

	// Merge coplanar pieces across short jogs, then re-fit every face over
	// its final extent.
	lines, axes, fits = mergeJogs(lines, axes, fits)
	corners = intersectAll(lines)
	for i := range lines {
		a, b := corners[i], corners[(i+1)%len(corners)]
		fits[i] = fitFace(wp, lines[i], axes[i], a, b)
		lines[i] = fits[i].Line
	}
	corners = intersectAll(lines)
	return &RoomShape{Corners: corners, Walls: fits}
}

// Jog merging: two same-facing edges joined by a short connector whose
// measured faces are nearly coplanar are one wall split by an artefact of
// the arrangement (a wall line from elsewhere in the plan). Real steps
// (a chimney breast, a pillar) are deeper than the tolerance and stay.
const (
	jogMaxConnector = 0.35 // metres
	jogTolMeasured  = 0.04 // face offset between two measured pieces
	jogTolInferred  = 0.10 // when either piece is inferred
)

func mergeJogs(lines []Line, axes []Orientation, fits []WallFit) ([]Line, []Orientation, []WallFit) {
	for changed := true; changed && len(lines) > 4; {
		changed = false
		corners := intersectAll(lines)
		n := len(lines)
		for i := 0; i < n; i++ {
			j, k := (i+1)%n, (i+2)%n
			if lines[i].N.Dot(lines[k].N) < 0.999 {
				continue
			}
			if corners[j].Sub(corners[k]).Norm() > jogMaxConnector {
				continue
			}
			tol := jogTolMeasured
			if fits[i].Inferred || fits[k].Inferred {
				tol = jogTolInferred
			}
			if math.Abs(lines[i].C-lines[k].C) > tol {
				continue
			}
			wi, wk := float64(fits[i].Support)+1, float64(fits[k].Support)+1
			lines[i].C = (lines[i].C*wi + lines[k].C*wk) / (wi + wk)
			fits[i].Support += fits[k].Support
			fits[i].Inferred = fits[i].Inferred && fits[k].Inferred
			// Remove j and k (k may wrap to index 0).
			keep := func(x int) bool { return x != j && x != k }
			var nl []Line
			var na []Orientation
			var nf []WallFit
			for x := 0; x < n; x++ {
				if keep(x) {
					nl, na, nf = append(nl, lines[x]), append(na, axes[x]), append(nf, fits[x])
				}
			}
			lines, axes, fits = nl, na, nf
			changed = true
			break
		}
	}
	return lines, axes, fits
}

// FallbackShape outlines a room whose walls could not be fitted (no wall
// lines bound it, as with sparse or noisy model depth): the cleaned region
// is traced, simplified at 10 cm and snapped to the Manhattan axes, and
// every wall is reported as inferred, so it carries the wide inferred-wall
// interval. A rough, honestly uncertain outline is more useful than
// dropping the room.
func FallbackShape(r *Rasters, cells []int) *RoomShape {
	mask := cleanRegion(r, cells, openRadius)
	poly := douglasPeucker(traceBoundary(r, mask), 0.10)
	if len(poly) < 3 {
		return nil
	}
	es := regularize(edgeLines(poly))
	if len(es) < 3 {
		return nil
	}
	lines := make([]Line, len(es))
	fits := make([]WallFit, len(es))
	for i, e := range es {
		lines[i] = e.l
		fits[i] = WallFit{Line: e.l, Axis: e.axis, Inferred: true}
	}
	corners := intersectAll(lines)
	if math.Abs(PolygonArea(corners)) < 1.0 {
		return nil
	}
	return &RoomShape{Corners: corners, Walls: fits}
}

// edge is a polygon edge as a line plus the vertex it starts at.
type edge struct {
	l    Line
	axis Orientation
	v    Pt
}

// edgeLines converts polygon edges to lines, snapping near-axis edges to
// exact axes through the edge midpoint. Normals point out of the room
// (polygon is counter-clockwise, so outward is to the right of travel).
func edgeLines(p []Pt) []edge {
	var out []edge
	for i := range p {
		a, b := p[i], p[(i+1)%len(p)]
		d := b.Sub(a)
		if d.Norm() < 1e-9 {
			continue
		}
		ang := math.Atan2(d.Y, d.X)
		m := a.Add(b).Scale(0.5)
		e := edge{axis: OrientOther, v: a}
		switch {
		case math.Abs(math.Remainder(ang, math.Pi)) < snapAngle:
			e.axis = OrientH
			e.l.N = Pt{0, -math.Copysign(1, d.X)}
		case math.Abs(math.Remainder(ang-math.Pi/2, math.Pi)) < snapAngle:
			e.axis = OrientV
			e.l.N = Pt{math.Copysign(1, d.Y), 0}
		default:
			u := d.Scale(1 / d.Norm())
			e.l.N = Pt{u.Y, -u.X}
		}
		e.l.C = e.l.N.Dot(m)
		out = append(out, e)
	}
	return out
}

// regularize makes every consecutive pair of edges intersect: consecutive
// edges on the same side within minEdgeLen are one wall with a small jog
// and are merged; parallel edges further apart get a perpendicular
// connector through their shared vertex; reversed edges (spikes) are
// dropped.
func regularize(es []edge) []edge {
	for changed := true; changed && len(es) > 3; {
		changed = false
		for i := 0; i < len(es) && len(es) > 3; i++ {
			j := (i + 1) % len(es)
			dot := es[i].l.N.Dot(es[j].l.N)
			switch {
			case dot > 0.999 && math.Abs(es[i].l.C-es[j].l.C) < minEdgeLen:
				es[i].l.C = (es[i].l.C + es[j].l.C) / 2
			case dot < -0.999:
			default:
				continue
			}
			es = append(es[:j], es[j+1:]...)
			changed = true
		}
	}
	var out []edge
	for i := range es {
		out = append(out, es[i])
		j := (i + 1) % len(es)
		if es[i].l.N.Dot(es[j].l.N) > 0.999 {
			t := es[i].l.N // travel from line i to line j
			if es[j].l.C < es[i].l.C {
				t = t.Scale(-1)
			}
			n := Pt{t.Y, -t.X}
			out = append(out, edge{l: Line{N: n, C: n.Dot(es[j].v)}, axis: axisOf(n), v: es[j].v})
		}
	}
	return out
}

func axisOf(n Pt) Orientation {
	switch {
	case math.Abs(n.X) < 1e-9:
		return OrientH
	case math.Abs(n.Y) < 1e-9:
		return OrientV
	}
	return OrientOther
}

func intersectAll(lines []Line) []Pt {
	out := make([]Pt, len(lines))
	for i := range lines {
		prev := lines[(i-1+len(lines))%len(lines)]
		if p, ok := prev.intersect(lines[i]); ok {
			out[i] = p
		}
	}
	return out
}

// fitFace finds the wall face near edge a→b: the densest layer of wall
// points between faceSearch inside and faceOutward outside the line, refined as the mean of the
// points within ±faceBand of that layer.
func fitFace(wp *WallPoints, l Line, axis Orientation, a, b Pt) WallFit {
	fit := WallFit{Line: l, Axis: axis}
	dir := b.Sub(a)
	length := dir.Norm()
	if length < 2*endTrim+0.05 {
		fit.Inferred = true
		return fit
	}
	u := dir.Scale(1 / length)
	// Box around the trimmed edge, widened by faceSearch along the normal.
	p0 := a.Add(u.Scale(endTrim))
	p1 := b.Sub(u.Scale(endTrim))
	// Only the room's own side of the line is searched, so the far face of
	// a thick wall (seen from the next room) is never picked.
	x0 := math.Min(p0.X, p1.X) - faceSearch
	x1 := math.Max(p0.X, p1.X) + faceSearch
	y0 := math.Min(p0.Y, p1.Y) - faceSearch
	y1 := math.Max(p0.Y, p1.Y) + faceSearch

	const bin = 0.005
	nb := int((faceSearch+faceOutward)/bin) + 1
	hist := make([]int, nb)
	var offs []float64
	var along []float64
	wp.inBox(x0, y0, x1, y1, func(p Pt) {
		t := p.Sub(a).Dot(u)
		if t < endTrim || t > length-endTrim {
			return
		}
		s := l.N.Dot(p) - l.C // signed offset, positive = outward
		if s < -faceSearch || s >= faceOutward {
			return
		}
		hist[int((s+faceSearch)/bin)]++
		offs = append(offs, s)
		along = append(along, t)
	})
	if len(offs) == 0 {
		fit.Inferred = true
		return fit
	}
	best, bestN := 0, -1
	for i := range hist { // 3-bin smoothing
		n := hist[i]
		if i > 0 {
			n += hist[i-1]
		}
		if i+1 < nb {
			n += hist[i+1]
		}
		if n > bestN {
			best, bestN = i, n
		}
	}
	peak := -faceSearch + (float64(best)+0.5)*bin
	var sum, ss float64
	var n int
	covered := make([]bool, int(length/coverageBin)+1)
	for i, s := range offs {
		if math.Abs(s-peak) <= faceBand {
			sum += s
			ss += s * s
			n++
			covered[int(along[i]/coverageBin)] = true
		}
	}
	mean := sum / float64(n)
	fit.Support = n
	fit.Spread = math.Sqrt(math.Max(ss/float64(n)-mean*mean, 0))
	nc := 0
	for _, c := range covered {
		if c {
			nc++
		}
	}
	fit.Coverage = float64(nc) / ((length - 2*endTrim) / coverageBin)
	fit.Inferred = fit.Coverage < minCoverage
	fit.Layers = layers(hist, offs, along, length, bin, peak)
	if !fit.Inferred {
		fit.Line.C += mean
	}
	return fit
}

// Layer is one density peak of wall points near a face (diagnostics).
type Layer struct {
	Offset   float64 `json:"offset_m"` // relative to the chosen face, positive = outward
	Density  float64 `json:"density"`  // peak height relative to the densest layer
	Coverage float64 `json:"coverage"` // fraction of the wall length it spans
}

// layers lists the local maxima of the smoothed offset histogram holding
// at least 15 % of the densest one.
func layers(hist []int, offs, along []float64, length, bin, chosen float64) []Layer {
	nb := len(hist)
	sm := make([]int, nb)
	maxN := 0
	for i := range hist {
		sm[i] = hist[i]
		if i > 0 {
			sm[i] += hist[i-1]
		}
		if i+1 < nb {
			sm[i] += hist[i+1]
		}
		maxN = max(maxN, sm[i])
	}
	var out []Layer
	for i := range sm {
		if sm[i]*100 < maxN*15 || (i > 0 && sm[i-1] > sm[i]) || (i+1 < nb && sm[i+1] >= sm[i]) {
			continue
		}
		c := -faceSearch + (float64(i)+0.5)*bin
		covered := make([]bool, int(length/coverageBin)+1)
		for k, s := range offs {
			if math.Abs(s-c) <= faceBand {
				covered[int(along[k]/coverageBin)] = true
			}
		}
		nc := 0
		for _, v := range covered {
			if v {
				nc++
			}
		}
		out = append(out, Layer{
			Offset:   math.Round((c-chosen)*1000) / 1000,
			Density:  math.Round(float64(sm[i])/float64(maxN)*100) / 100,
			Coverage: math.Round(float64(nc)/((length-2*endTrim)/coverageBin)*100) / 100,
		})
	}
	return out
}

// minFeature is the smallest outline feature kept. Notches, bumps and
// leak strips narrower or shallower than this are not seen consistently
// from one capture to the next (they depend on which rectangles of the
// wall-line arrangement pass the ownership threshold), and they decide
// where the neighbouring walls end.
const minFeature = 0.30

// removeSmallFeatures flattens every U-shaped feature of the outline (two
// antiparallel edges joined by a third) whose depth (the shorter leg) or
// width (the joining edge) is below min: the joining edge is moved back
// by the shorter leg. Repeats until no such feature remains.
func removeSmallFeatures(p []Pt, min float64) []Pt {
	for iter := 0; iter < 4*len(p) && len(p) > 4; iter++ {
		n := len(p)
		found := false
		for i := 0; i < n; i++ {
			a, b, c, d := p[i], p[(i+1)%n], p[(i+2)%n], p[(i+3)%n]
			e0, e1, e2 := b.Sub(a), c.Sub(b), d.Sub(c)
			la, w, lb := e0.Norm(), e1.Norm(), e2.Norm()
			if la == 0 || lb == 0 || e0.Dot(e2) >= 0 || math.Abs(e0.Cross(e2)) > 1e-9*la*lb {
				continue // not a U turn
			}
			if math.Min(la, lb) >= min && w >= min {
				continue
			}
			back := e0.Scale(math.Min(la, lb) / la)
			p[(i+1)%n], p[(i+2)%n] = b.Sub(back), c.Sub(back)
			p = dropCollinear(dedupe(p))
			found = true
			break
		}
		if !found {
			break
		}
	}
	return p
}

// dedupe removes consecutive coincident vertices.
func dedupe(p []Pt) []Pt {
	var out []Pt
	for i := range p {
		if p[i].Sub(p[(i+1)%len(p)]).Norm() > 1e-9 {
			out = append(out, p[i])
		}
	}
	return out
}

// dropCollinear removes vertices whose two edges have the same direction.
func dropCollinear(p []Pt) []Pt {
	var out []Pt
	for i := range p {
		a, b, c := p[(i-1+len(p))%len(p)], p[i], p[(i+1)%len(p)]
		if math.Abs(b.Sub(a).Cross(c.Sub(b))) > 1e-12 {
			out = append(out, b)
		}
	}
	return out
}
