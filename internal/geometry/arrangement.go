package geometry

import (
	"sort"
)

// Wall-line arrangement: the dominant H and V wall lines cut the plan into
// rectangles; each rectangle is given to the room covering most of it, and
// a room's outline is the boundary of its rectangles. Outlines therefore
// run along real wall lines, and open-plan boundaries fall on extensions
// of walls rather than wherever a region happened to stop.

const (
	lineMinLen   = 0.40 // metres of wall cells on a row/column to make a line
	lineNMS      = 3    // cells; peaks closer than this are one line
	rectMinShare = 0.5  // a rectangle belongs to a room when the room covers this share of it
)

// WallLines returns plan x positions of vertical wall lines and plan y
// positions of horizontal ones, from row/column counts of oriented wall
// cells. The raster bounds are always included.
func WallLines(r *Rasters, orient []Orientation) (xs, ys []float64) {
	colV := make([]float64, r.W)
	rowH := make([]float64, r.H)
	for cy := 0; cy < r.H; cy++ {
		for cx := 0; cx < r.W; cx++ {
			o := orient[r.Idx(cx, cy)]
			if o&OrientV != 0 {
				colV[cx]++
			}
			if o&OrientH != 0 {
				rowH[cy]++
			}
		}
	}
	minCells := lineMinLen / r.Res
	xs = []float64{r.X0, r.X0 + float64(r.W)*r.Res}
	for _, c := range peaks(colV, minCells) {
		xs = append(xs, r.X0+(c+0.5)*r.Res)
	}
	ys = []float64{r.Y0, r.Y0 + float64(r.H)*r.Res}
	for _, c := range peaks(rowH, minCells) {
		ys = append(ys, r.Y0+(c+0.5)*r.Res)
	}
	sort.Float64s(xs)
	sort.Float64s(ys)
	return xs, ys
}

// peaks returns sub-cell positions of local maxima of h (smoothed 1-2-1)
// at least minV high, with non-maximum suppression over lineNMS cells.
func peaks(h []float64, minV float64) []float64 {
	n := len(h)
	s := make([]float64, n)
	for i := range h {
		s[i] = 2 * h[i]
		if i > 0 {
			s[i] += h[i-1]
		}
		if i+1 < n {
			s[i] += h[i+1]
		}
		s[i] /= 4
	}
	var out []float64
	for i := range s {
		if s[i] < minV {
			continue
		}
		isMax := true
		for d := -lineNMS; d <= lineNMS && isMax; d++ {
			j := i + d
			if d != 0 && j >= 0 && j < n && (s[j] > s[i] || (s[j] == s[i] && j < i)) {
				isMax = false
			}
		}
		if !isMax {
			continue
		}
		// Centroid of the raw counts over the peak neighbourhood.
		var w, m float64
		for j := max(0, i-1); j <= min(n-1, i+1); j++ {
			w += h[j]
			m += h[j] * float64(j)
		}
		out = append(out, m/w)
	}
	return out
}

// Arrangement is the grid of rectangles cut by the wall lines.
type Arrangement struct {
	Xs, Ys []float64 // sorted line positions
	Owner  []int32   // per rectangle (i + j*(len(Xs)-1)), room label or 0
}

// BuildArrangement assigns every rectangle to the room label covering at
// least rectMinShare of its raster cells.
func BuildArrangement(r *Rasters, labels []int32, xs, ys []float64) *Arrangement {
	nx, ny := len(xs)-1, len(ys)-1
	a := &Arrangement{Xs: xs, Ys: ys, Owner: make([]int32, nx*ny)}
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			counts := map[int32]int{}
			total := 0
			for cy := cellIndex(ys[j], r.Y0, r.Res); cy < cellIndex(ys[j+1], r.Y0, r.Res); cy++ {
				for cx := cellIndex(xs[i], r.X0, r.Res); cx < cellIndex(xs[i+1], r.X0, r.Res); cx++ {
					if cx < 0 || cy < 0 || cx >= r.W || cy >= r.H {
						continue
					}
					total++
					if l := labels[r.Idx(cx, cy)]; l > 0 {
						counts[l]++
					}
				}
			}
			var best int32
			bestN := 0
			for l, n := range counts {
				if n > bestN || (n == bestN && l < best) {
					best, bestN = l, n
				}
			}
			if total > 0 && float64(bestN) >= rectMinShare*float64(total) {
				a.Owner[i+j*nx] = best
			}
		}
	}
	return a
}

// cellIndex is the first raster cell whose centre is at or after v.
func cellIndex(v, origin, res float64) int {
	c := (v-origin)/res - 0.5
	i := int(c)
	if float64(i) < c {
		i++
	}
	return i
}

// Outline traces the outer boundary of the largest 4-connected group of
// rectangles owned by label, counter-clockwise, in plan coordinates.
func (a *Arrangement) Outline(label int32) []Pt {
	nx, ny := len(a.Xs)-1, len(a.Ys)-1
	g := Grid2{W: nx, H: ny}
	mask := make([]bool, nx*ny)
	for i, o := range a.Owner {
		mask[i] = o == label
	}
	mask = largestComponent(g, mask)
	return traceGrid(g, mask, func(i int) float64 { return a.Xs[i] }, func(j int) float64 { return a.Ys[j] })
}

func largestComponent(g Grid2, mask []bool) []bool {
	seen := make([]bool, len(mask))
	var best []int
	for i := range mask {
		if !mask[i] || seen[i] {
			continue
		}
		c := flood(g, i, func(j int) bool { return mask[j] })
		for _, j := range c {
			seen[j] = true
		}
		if len(c) > len(best) {
			best = c
		}
	}
	out := make([]bool, len(mask))
	for _, i := range best {
		out[i] = true
	}
	return out
}
