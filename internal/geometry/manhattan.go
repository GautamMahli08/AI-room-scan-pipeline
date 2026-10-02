package geometry

import (
	"math"
)

// Orientation flags of a wall cell. A corner cell can be both H and V.
type Orientation uint8

const (
	OrientNone  Orientation = 0
	OrientH     Orientation = 1 << iota // part of a wall running along plan x
	OrientV                             // part of a wall running along plan y
	OrientOther                         // part of a line at a non-Manhattan angle
)

// localLines estimates, for every wall cell, the angle of the local line
// through it from the 2x2 covariance of wall cells within radius cells.
// ok is false where the neighbourhood is not clearly linear.
func localLines(g Grid2, wall []bool, radius int) (angle []float64, ok []bool) {
	angle = make([]float64, len(wall))
	ok = make([]bool, len(wall))
	r2 := radius * radius
	for cy := 0; cy < g.H; cy++ {
		for cx := 0; cx < g.W; cx++ {
			if !wall[g.Idx(cx, cy)] {
				continue
			}
			var n, sx, sy, sxx, syy, sxy float64
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					x, y := cx+dx, cy+dy
					if dx*dx+dy*dy > r2 || x < 0 || y < 0 || x >= g.W || y >= g.H || !wall[g.Idx(x, y)] {
						continue
					}
					fx, fy := float64(dx), float64(dy)
					n++
					sx, sy = sx+fx, sy+fy
					sxx, syy, sxy = sxx+fx*fx, syy+fy*fy, sxy+fx*fy
				}
			}
			if n < float64(radius) {
				continue
			}
			cxx, cyy, cxy := sxx/n-sx*sx/n/n, syy/n-sy*sy/n/n, sxy/n-sx*sy/n/n
			tr, det := cxx+cyy, cxx*cyy-cxy*cxy
			disc := math.Sqrt(math.Max(tr*tr/4-det, 0))
			l1, l2 := tr/2+disc, tr/2-disc
			if l1 <= 0 || l2 > l1/6 { // need a clearly elongated neighbourhood
				continue
			}
			i := g.Idx(cx, cy)
			angle[i] = 0.5 * math.Atan2(2*cxy, cxx-cyy)
			ok[i] = true
		}
	}
	return angle, ok
}

// EstimateManhattan returns the dominant wall direction θ ∈ [0, π/2) of a
// raster built with Theta = 0, from a histogram of local line angles modulo
// 90°, and the fraction of linear wall cells within 3° of the Manhattan axes.
func EstimateManhattan(r *Rasters) (theta, support float64) {
	wall := r.WallMask()
	angle, ok := localLines(r.Grid2, wall, 4)

	const bins = 180 // 0.5° bins over 90°
	var hist [bins]float64
	var total float64
	for i := range angle {
		if !ok[i] {
			continue
		}
		a := math.Mod(angle[i], math.Pi/2)
		if a < 0 {
			a += math.Pi / 2
		}
		hist[int(a/(math.Pi/2)*bins)%bins]++
		total++
	}
	if total == 0 {
		return 0, 0
	}
	// Circular smoothing over ±2 bins.
	best, bestV := 0, -1.0
	for i := 0; i < bins; i++ {
		var v float64
		for d := -2; d <= 2; d++ {
			v += hist[(i+d+bins)%bins]
		}
		if v > bestV {
			best, bestV = i, v
		}
	}
	// Refine with a circular mean of 4·angle near the peak.
	peak := (float64(best) + 0.5) / bins * math.Pi / 2
	var sc, ss, near float64
	for i := range angle {
		if !ok[i] {
			continue
		}
		d := math.Remainder(angle[i]-peak, math.Pi/2)
		if math.Abs(d) < 3*math.Pi/180 {
			sc += math.Cos(4 * angle[i])
			ss += math.Sin(4 * angle[i])
			near++
		}
	}
	theta = math.Atan2(ss, sc) / 4
	if theta < 0 {
		theta += math.Pi / 2
	}
	return theta, near / total
}

// Orientations labels every wall cell of a Manhattan-aligned raster by the
// length of the straight runs of wall cells through it, which unlike a local
// covariance does not break down on thick walls or at corners. Cells on no
// long axis-aligned run but on a clearly linear neighbourhood are
// OrientOther.
func Orientations(r *Rasters, wall []bool) []Orientation {
	const minRun = 0.30 // metres
	n := int(minRun / r.Res)
	hRun := runLengths(r.W, r.H, wall, func(a, b int) int { return r.Idx(a, b) })
	vRun := runLengths(r.H, r.W, wall, func(a, b int) int { return r.Idx(b, a) })
	angle, ok := localLines(r.Grid2, wall, 10)
	out := make([]Orientation, len(wall))
	for i := range out {
		if !wall[i] {
			continue
		}
		if hRun[i] >= n {
			out[i] |= OrientH
		}
		if vRun[i] >= n {
			out[i] |= OrientV
		}
		if out[i] == OrientNone && ok[i] {
			a := math.Abs(math.Remainder(angle[i], math.Pi/2))
			if a > 10*math.Pi/180 {
				out[i] = OrientOther
			}
		}
	}
	return out
}

// runLengths returns, for every cell, the length of the run of set cells
// along the first index containing it.
func runLengths(alongN, acrossN int, m []bool, idx func(a, b int) int) []int {
	out := make([]int, len(m))
	for b := 0; b < acrossN; b++ {
		for a := 0; a < alongN; {
			if !m[idx(a, b)] {
				a++
				continue
			}
			e := a
			for e < alongN && m[idx(e, b)] {
				e++
			}
			for k := a; k < e; k++ {
				out[idx(k, b)] = e - a
			}
			a = e
		}
	}
	return out
}
