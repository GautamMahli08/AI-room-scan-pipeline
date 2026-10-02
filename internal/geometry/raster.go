package geometry

import (
	"math"
	"math/bits"

	"roomscan/internal/geom"
)

// PlanFrame maps ARKit world points to the 2D plan: heights are measured
// from the floor plane, and plan (x, y) is world (x, -z) rotated by -Theta
// so dominant walls are axis-aligned. Viewed from above, +y is up on the
// rendered plan and the frame is right-handed.
type PlanFrame struct {
	Floor Floor
	Theta float64 // Manhattan angle, radians, in [0, π/2)
}

// ToPlan returns plan coordinates (x, y) and height above floor h.
func (f PlanFrame) ToPlan(p geom.Vec3) (x, y, h float64) {
	c, s := math.Cos(-f.Theta), math.Sin(-f.Theta)
	wx, wy := p[0], -p[2]
	return c*wx - s*wy, s*wx + c*wy, f.Floor.Height(p)
}

// ToWorldXZ inverts the planar part of ToPlan (height is not recovered).
func (f PlanFrame) ToWorldXZ(x, y float64) (wx, wz float64) {
	c, s := math.Cos(f.Theta), math.Sin(f.Theta)
	px, py := c*x-s*y, s*x+c*y
	return px, -py
}

// Grid2 is the geometry of a regular 2D raster over the plan.
type Grid2 struct {
	X0, Y0 float64 // plan coordinates of the lower-left corner of cell (0,0)
	Res    float64
	W, H   int
}

// Cell returns the cell containing plan point (x, y).
func (g Grid2) Cell(x, y float64) (cx, cy int, ok bool) {
	cx = int(math.Floor((x - g.X0) / g.Res))
	cy = int(math.Floor((y - g.Y0) / g.Res))
	return cx, cy, cx >= 0 && cy >= 0 && cx < g.W && cy < g.H
}

// Center returns the plan coordinates of the centre of cell (cx, cy).
func (g Grid2) Center(cx, cy int) (float64, float64) {
	return g.X0 + (float64(cx)+0.5)*g.Res, g.Y0 + (float64(cy)+0.5)*g.Res
}

func (g Grid2) Idx(cx, cy int) int { return cy*g.W + cx }

// Height bands used throughout plan extraction.
const (
	heightBin   = 0.05 // vertical resolution of the occupancy mask
	floorTol    = 0.04 // |h| below this counts as floor
	wallBandLo  = 0.30 // walls are least occluded by furniture in this band
	wallBandHi  = 2.00
	wallMinFill = 0.5 // fraction of wall-band height bins that must be occupied
)

// Rasters are the top-down summaries of the fused cloud that plan
// extraction works on.
type Rasters struct {
	Grid2
	Frame PlanFrame
	// Heights has bit i set when a point with height in
	// [i·heightBin, (i+1)·heightBin) falls in the cell (up to 3.2 m).
	Heights []uint64
	Floor   []int32 // number of floor points per cell
}

// BuildRasters projects pts into a plan raster of resolution res.
func BuildRasters(pts []geom.Vec3, f PlanFrame, res float64) *Rasters {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		x, y, _ := f.ToPlan(p)
		minX, minY = math.Min(minX, x), math.Min(minY, y)
		maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
	}
	const pad = 0.2
	g := Grid2{X0: minX - pad, Y0: minY - pad, Res: res}
	g.W = int((maxX-minX+2*pad)/res) + 1
	g.H = int((maxY-minY+2*pad)/res) + 1
	r := &Rasters{Grid2: g, Frame: f, Heights: make([]uint64, g.W*g.H), Floor: make([]int32, g.W*g.H)}
	for _, p := range pts {
		x, y, h := f.ToPlan(p)
		cx, cy, ok := g.Cell(x, y)
		if !ok {
			continue
		}
		i := g.Idx(cx, cy)
		if math.Abs(h) < floorTol {
			r.Floor[i]++
			continue
		}
		if h > 0 && h < 64*heightBin {
			r.Heights[i] |= 1 << uint(h/heightBin)
		}
	}
	return r
}

// bandMask returns the bits of the height mask covering [lo, hi).
func bandMask(lo, hi float64) uint64 {
	var m uint64
	for i := int(lo / heightBin); i < int(math.Ceil(hi/heightBin)) && i < 64; i++ {
		m |= 1 << uint(i)
	}
	return m
}

// WallFill is the fraction of wall-band height bins occupied in cell i.
func (r *Rasters) WallFill(i int) float64 {
	m := bandMask(wallBandLo, wallBandHi)
	return float64(bits.OnesCount64(r.Heights[i]&m)) / float64(bits.OnesCount64(m))
}

// WallMask marks cells whose vertical extent in the wall band is mostly
// occupied: walls and tall furniture.
func (r *Rasters) WallMask() []bool {
	out := make([]bool, len(r.Heights))
	for i := range out {
		out[i] = r.WallFill(i) >= wallMinFill
	}
	return out
}

// OccupiedMask marks cells with any point between floor and wall band top
// (furniture, walls, clutter).
func (r *Rasters) OccupiedMask() []bool {
	m := bandMask(floorTol, wallBandHi)
	out := make([]bool, len(r.Heights))
	for i := range out {
		out[i] = r.Heights[i]&m != 0
	}
	return out
}
