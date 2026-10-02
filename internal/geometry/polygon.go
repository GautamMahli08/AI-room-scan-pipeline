package geometry

import (
	"math"
)

// Pt is a plan-space point in metres.
type Pt struct{ X, Y float64 }

func (a Pt) Sub(b Pt) Pt        { return Pt{a.X - b.X, a.Y - b.Y} }
func (a Pt) Add(b Pt) Pt        { return Pt{a.X + b.X, a.Y + b.Y} }
func (a Pt) Scale(s float64) Pt { return Pt{a.X * s, a.Y * s} }
func (a Pt) Dot(b Pt) float64   { return a.X*b.X + a.Y*b.Y }
func (a Pt) Cross(b Pt) float64 { return a.X*b.Y - a.Y*b.X }
func (a Pt) Norm() float64      { return math.Hypot(a.X, a.Y) }

// PolygonArea is the signed shoelace area (positive when counter-clockwise).
func PolygonArea(p []Pt) float64 {
	var s float64
	for i := range p {
		s += p[i].Cross(p[(i+1)%len(p)])
	}
	return s / 2
}

// cleanRegion applies a morphological opening of radius rad metres to the
// cell set and returns the largest remaining 4-connected component as a
// mask. Thin protrusions (rays leaking through gaps) disappear; the room
// body keeps its shape.
func cleanRegion(r *Rasters, cells []int, rad float64) []bool {
	in := make([]bool, r.W*r.H)
	for _, i := range cells {
		in[i] = true
	}
	d := distanceTransform(r.Grid2, in)
	core := make([]bool, len(in))
	for i := range in {
		core[i] = in[i] && d[i] > rad
	}
	notCore := make([]bool, len(in))
	for i := range core {
		notCore[i] = !core[i]
	}
	dc := distanceTransform(r.Grid2, notCore) // distance to the nearest core cell
	opened := make([]bool, len(in))
	for i := range in {
		opened[i] = in[i] && dc[i] <= rad+r.Res
	}
	var best []int
	seen := make([]bool, len(in))
	for _, i := range cells {
		if !opened[i] || seen[i] {
			continue
		}
		c := flood(r.Grid2, i, func(j int) bool { return opened[j] })
		for _, j := range c {
			seen[j] = true
		}
		if len(c) > len(best) {
			best = c
		}
	}
	out := make([]bool, len(in))
	for _, i := range best {
		out[i] = true
	}
	return out
}

// CleanLabels applies cleanRegion to every room and returns the resulting
// label raster.
func CleanLabels(r *Rasters, regions []Region) []int32 {
	out := make([]int32, r.W*r.H)
	for _, reg := range regions {
		for i, m := range cleanRegion(r, reg.Cells, openRadius) {
			if m {
				out[i] = reg.Label
			}
		}
	}
	return out
}

// traceBoundary follows the outer boundary of a 4-connected cell mask
// counter-clockwise (region on the left) and returns the corner vertices in
// plan coordinates. Holes are ignored.
func traceBoundary(r *Rasters, mask []bool) []Pt {
	return traceGrid(r.Grid2, mask,
		func(i int) float64 { return r.X0 + float64(i)*r.Res },
		func(j int) float64 { return r.Y0 + float64(j)*r.Res })
}

// traceGrid traces the outer boundary of mask on grid g; vx and vy map
// vertex indices to plan coordinates, so the grid need not be uniform.
func traceGrid(r Grid2, mask []bool, vx, vy func(int) float64) []Pt {
	start := -1
	for i, m := range mask { // lowest row, then leftmost cell
		if m {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	in := func(cx, cy int) bool {
		return cx >= 0 && cy >= 0 && cx < r.W && cy < r.H && mask[r.Idx(cx, cy)]
	}
	// edgeOK reports whether walking from vertex (vx,vy) in direction d
	// keeps the region on the left and outside on the right.
	edgeOK := func(vx, vy int, d [2]int) bool {
		// Cells left/right of the edge, from the edge midpoint offset by ±½ normal.
		mx2, my2 := 2*vx+d[0], 2*vy+d[1] // doubled midpoint
		lx2, ly2 := mx2-d[1], my2+d[0]
		rx2, ry2 := mx2+d[1], my2-d[0]
		return in(floorHalf(lx2), floorHalf(ly2)) && !in(floorHalf(rx2), floorHalf(ry2))
	}
	sx, sy := start%r.W, start/r.W
	px, py := sx, sy
	d := [2]int{1, 0}
	var verts []Pt
	for steps := 0; steps < 4*len(mask); steps++ {
		left := [2]int{-d[1], d[0]}
		right := [2]int{d[1], -d[0]}
		back := [2]int{-d[0], -d[1]}
		var nd [2]int
		found := false
		for _, c := range [][2]int{left, d, right, back} {
			if edgeOK(px, py, c) {
				nd, found = c, true
				break
			}
		}
		if !found {
			break
		}
		if nd != d || len(verts) == 0 {
			verts = append(verts, Pt{vx(px), vy(py)})
		}
		d = nd
		px, py = px+d[0], py+d[1]
		// The start corner (bottom-left of the lowest, leftmost cell) is an
		// extreme point of the region, so the boundary passes it once.
		if px == sx && py == sy {
			break
		}
	}
	return verts
}

// PointInPolygon reports whether p is inside polygon poly (even-odd rule).
func PointInPolygon(p Pt, poly []Pt) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}

// floorHalf returns floor(v/2) for a doubled coordinate.
func floorHalf(v int) int {
	if v < 0 {
		return (v - 1) / 2
	}
	return v / 2
}

// douglasPeucker simplifies a closed polygon with tolerance tol (metres).
func douglasPeucker(p []Pt, tol float64) []Pt {
	if len(p) < 4 {
		return p
	}
	// Split the ring at the two mutually farthest-apart vertices (approx.).
	far := 0
	for i := range p {
		if p[i].Sub(p[0]).Norm() > p[far].Sub(p[0]).Norm() {
			far = i
		}
	}
	a := dpOpen(append([]Pt{}, p[:far+1]...), tol)
	b := dpOpen(append(append([]Pt{}, p[far:]...), p[0]), tol)
	return append(a[:len(a)-1], b[:len(b)-1]...)
}

func dpOpen(p []Pt, tol float64) []Pt {
	if len(p) < 3 {
		return p
	}
	a, b := p[0], p[len(p)-1]
	ab := b.Sub(a)
	l := ab.Norm()
	best, bi := -1.0, 0
	for i := 1; i < len(p)-1; i++ {
		var d float64
		if l == 0 {
			d = p[i].Sub(a).Norm()
		} else {
			d = math.Abs(ab.Cross(p[i].Sub(a))) / l
		}
		if d > best {
			best, bi = d, i
		}
	}
	if best <= tol {
		return []Pt{a, b}
	}
	l1 := dpOpen(p[:bi+1], tol)
	l2 := dpOpen(p[bi:], tol)
	return append(l1[:len(l1)-1], l2...)
}
