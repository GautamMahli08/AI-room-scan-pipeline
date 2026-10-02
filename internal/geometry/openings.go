package geometry

import (
	"math"
	"sort"
)

// OpeningKind classifies a wall gap.
type OpeningKind int

const (
	KindDoor       OpeningKind = iota // floor-level passage with a header
	KindOpening                       // floor-level passage without a header (open to ceiling or header unseen)
	KindWindow                        // wall below the gap (sill)
	KindUnobserved                    // nothing seen in or through the gap: not reported as an opening
)

func (k OpeningKind) String() string {
	return [...]string{"door", "opening", "window", "unobserved"}[k]
}

// Opening is a gap assigned to a wall of a room.
type Opening struct {
	Room, Wall int // indices into rooms and RoomShape.Walls
	Kind       OpeningKind
	Offset     float64 // from wall start to the near jamb, metres
	Width      float64 // between jambs, metres
	JambPoints int     // wall points that bound the gap on both sides (0 = raster width only)
	Rooms      []int32 // room labels on the two sides (for adjacency)
	Crossed    bool    // the camera walked through it
}

const (
	minOpening   = 0.50 // narrower gaps are holes in the wall data, not openings
	jambBand     = 0.04 // wall points within ± this of the face bound the gap
	jambSearch   = 0.30 // look this far beyond the raster gap for jambs
	assignDist   = 0.20 // gap centre must be this close to the wall line
	sideReach    = 0.30 // look this far across the wall for the rooms on each side
	trajCrossTol = 0.10
)

// AssignOpenings attaches every opening-sized gap to the nearest matching
// wall of a room, classifies it and refines its width from the jambs.
func AssignOpenings(r *Rasters, gaps []Gap, shapes []*RoomShape, labels []int32, wp *WallPoints, traj [][2]float64) []Opening {
	var out []Opening
	for _, g := range gaps {
		if g.Width(r.Res) < minOpening {
			continue
		}
		// Gap centre line in plan coordinates.
		a0, a1 := float64(g.A0)*r.Res, float64(g.A1)*r.Res
		bc := (float64(g.B0+g.B1) / 2) * r.Res
		var c0, c1 Pt
		if g.Orient == OrientH {
			c0, c1 = Pt{r.X0 + a0, r.Y0 + bc}, Pt{r.X0 + a1, r.Y0 + bc}
		} else {
			c0, c1 = Pt{r.X0 + bc, r.Y0 + a0}, Pt{r.X0 + bc, r.Y0 + a1}
		}
		mid := c0.Add(c1).Scale(0.5)
		sides := gapSides(r, g, labels)

		bestRoom, bestWall, bestD := -1, -1, assignDist
		for ri, s := range shapes {
			if s == nil || !containsLabel(sides, int32(ri+1)) {
				continue
			}
			for wi, w := range s.Walls {
				if w.Axis != g.Orient {
					continue
				}
				a, b := s.Corners[wi], s.Corners[(wi+1)%len(s.Corners)]
				u := b.Sub(a)
				t := mid.Sub(a).Dot(u) / u.Dot(u)
				if t < 0 || t > 1 {
					continue
				}
				if d := math.Abs(w.Line.N.Dot(mid) - w.Line.C); d < bestD {
					bestRoom, bestWall, bestD = ri, wi, d
				}
			}
		}
		if bestRoom < 0 {
			continue
		}
		s := shapes[bestRoom]
		w := s.Walls[bestWall]
		a, b := s.Corners[bestWall], s.Corners[(bestWall+1)%len(s.Corners)]
		u := b.Sub(a).Scale(1 / b.Sub(a).Norm())
		t0, t1 := c0.Sub(a).Dot(u), c1.Sub(a).Dot(u)
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		o := Opening{Room: bestRoom, Wall: bestWall, Offset: t0, Width: t1 - t0, Rooms: sides}
		if lo, hi, n, ok := jambs(wp, w.Line, a, u, t0, t1); ok {
			o.Offset, o.Width, o.JambPoints = lo, hi-lo, n
		}
		o.Crossed = crosses(traj, c0, c1, trajCrossTol+float64(g.B1-g.B0)*r.Res)
		// A window has wall below it and faces outside: with rooms on both
		// sides, wall seen only low down is furniture in front of a wall
		// that was not seen higher up.
		passage := o.Crossed || g.FloorFill >= 0.3
		switch {
		case g.LowFill >= 0.5 && !o.Crossed && len(sides) < 2:
			o.Kind = KindWindow
		case passage || (len(sides) == 2 && g.LowFill < 0.5):
			if g.HeaderFill >= 0.5 {
				o.Kind = KindDoor
			} else {
				o.Kind = KindOpening
			}
		default:
			o.Kind = KindUnobserved
		}
		out = append(out, o)
	}
	return dedupeOpenings(out)
}

// dedupeOpenings merges openings on the same wall whose extents overlap
// (one physical gap found on several raster rows, or by both jamb and
// raster extents). The one with jamb evidence, else the wider, wins.
func dedupeOpenings(os []Opening) []Opening {
	sort.SliceStable(os, func(i, j int) bool {
		if os[i].Room != os[j].Room {
			return os[i].Room < os[j].Room
		}
		if os[i].Wall != os[j].Wall {
			return os[i].Wall < os[j].Wall
		}
		return os[i].Offset < os[j].Offset
	})
	var out []Opening
	for _, o := range os {
		if n := len(out); n > 0 {
			p := &out[n-1]
			if p.Room == o.Room && p.Wall == o.Wall && o.Offset < p.Offset+p.Width {
				better := (o.JambPoints > 0) != (p.JambPoints > 0) && o.JambPoints > 0 ||
					(o.JambPoints > 0) == (p.JambPoints > 0) && o.Width > p.Width
				if better {
					*p = o
				}
				p.Crossed = p.Crossed || o.Crossed
				continue
			}
		}
		out = append(out, o)
	}
	return out
}

// gapSides returns the distinct room labels found just across the gap on
// both sides of the wall.
func gapSides(r *Rasters, g Gap, labels []int32) []int32 {
	reach := int(sideReach / r.Res)
	seen := map[int32]bool{}
	for a := g.A0; a < g.A1; a++ {
		for _, b := range []int{g.B0 - reach, g.B1 - 1 + reach} {
			x, y := a, b
			if g.Orient == OrientV {
				x, y = b, a
			}
			if x >= 0 && y >= 0 && x < r.W && y < r.H {
				if l := labels[r.Idx(x, y)]; l > 0 {
					seen[l] = true
				}
			}
		}
	}
	var out []int32
	for l := range seen {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func containsLabel(ls []int32, l int32) bool {
	for _, x := range ls {
		if x == l {
			return true
		}
	}
	return false
}

// jambs finds the wall points nearest the gap on each side along the wall
// face; the opening runs between them.
func jambs(wp *WallPoints, l Line, a, u Pt, t0, t1 float64) (lo, hi float64, n int, ok bool) {
	ts0, ts1 := t0-jambSearch, t1+jambSearch
	p0, p1 := a.Add(u.Scale(ts0)), a.Add(u.Scale(ts1))
	lo, hi = math.Inf(-1), math.Inf(1)
	mid := (t0 + t1) / 2
	wp.inBox(math.Min(p0.X, p1.X)-jambBand, math.Min(p0.Y, p1.Y)-jambBand,
		math.Max(p0.X, p1.X)+jambBand, math.Max(p0.Y, p1.Y)+jambBand, func(p Pt) {
			if math.Abs(l.N.Dot(p)-l.C) > jambBand {
				return
			}
			t := p.Sub(a).Dot(u)
			if t < ts0 || t > ts1 {
				return
			}
			n++
			if t < mid && t > lo {
				lo = t
			}
			if t >= mid && t < hi {
				hi = t
			}
		})
	ok = !math.IsInf(lo, 0) && !math.IsInf(hi, 0) && hi-lo >= minOpening*0.8
	return lo, hi, n, ok
}

// crosses reports whether the trajectory passes through segment c0-c1
// (within tol of it, between its ends).
func crosses(traj [][2]float64, c0, c1 Pt, tol float64) bool {
	d := c1.Sub(c0)
	l := d.Norm()
	u := d.Scale(1 / l)
	n := Pt{-u.Y, u.X}
	prevSide := 0.0
	for _, q := range traj {
		p := Pt{q[0], q[1]}
		t := p.Sub(c0).Dot(u)
		s := p.Sub(c0).Dot(n)
		if t < 0 || t > l || math.Abs(s) > tol*4 {
			prevSide = 0
			continue
		}
		if prevSide != 0 && math.Signbit(s) != math.Signbit(prevSide) {
			return true
		}
		prevSide = s
	}
	return false
}

// OpenBoundary is where two rooms of an open-plan space meet without a
// wall. It is reported as an inferred "opening" on the nearest wall of Room.
type OpenBoundary struct {
	Room, Other int // room indices
	Wall        int // index into shapes[Room].Walls
	Offset      float64
	Width       float64
}

// OpenBoundaries finds room pairs whose regions touch directly over at
// least minOpening metres.
func OpenBoundaries(r *Rasters, labels []int32, shapes []*RoomShape) []OpenBoundary {
	type pair [2]int32
	cells := map[pair][]Pt{}
	var order []pair
	for cy := 0; cy < r.H; cy++ {
		for cx := 0; cx < r.W; cx++ {
			a := labels[r.Idx(cx, cy)]
			if a == 0 {
				continue
			}
			for _, d := range [2][2]int{{1, 0}, {0, 1}} {
				nx, ny := cx+d[0], cy+d[1]
				if nx >= r.W || ny >= r.H {
					continue
				}
				b := labels[r.Idx(nx, ny)]
				if b == 0 || b == a {
					continue
				}
				k := pair{min(a, b), max(a, b)}
				if _, ok := cells[k]; !ok {
					order = append(order, k)
				}
				x0, y0 := r.Center(cx, cy)
				x1, y1 := r.Center(nx, ny)
				cells[k] = append(cells[k], Pt{(x0 + x1) / 2, (y0 + y1) / 2})
			}
		}
	}
	var out []OpenBoundary
	for _, k := range order {
		pts := cells[k]
		if float64(len(pts))*r.Res < minOpening {
			continue
		}
		ri := int(k[0]) - 1
		s := shapes[ri]
		if s == nil {
			continue
		}
		var c Pt
		for _, p := range pts {
			c = c.Add(p)
		}
		c = c.Scale(1 / float64(len(pts)))
		best, bestD := -1, math.Inf(1)
		for wi, w := range s.Walls {
			a, b := s.Corners[wi], s.Corners[(wi+1)%len(s.Corners)]
			u := b.Sub(a)
			t := c.Sub(a).Dot(u) / u.Dot(u)
			if t < -0.1 || t > 1.1 {
				continue
			}
			if d := math.Abs(w.Line.N.Dot(c) - w.Line.C); d < bestD {
				best, bestD = wi, d
			}
		}
		if best < 0 {
			continue
		}
		a, b := s.Corners[best], s.Corners[(best+1)%len(s.Corners)]
		l := b.Sub(a).Norm()
		u := b.Sub(a).Scale(1 / l)
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, p := range pts {
			t := p.Sub(a).Dot(u)
			lo, hi = math.Min(lo, t), math.Max(hi, t)
		}
		lo, hi = math.Max(lo, 0), math.Min(hi, l)
		if hi-lo < minOpening {
			continue
		}
		out = append(out, OpenBoundary{Room: ri, Other: int(k[1]) - 1, Wall: best, Offset: lo, Width: hi - lo})
	}
	return out
}
