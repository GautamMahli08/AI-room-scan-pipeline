package geometry

import "math"

// Morphological room splitting (Bormann et al., "Room segmentation: survey,
// implementation, and analysis", ICRA 2016): erode the free space step by
// step; a region that breaks into parts which each survive further erosion
// is several rooms joined by a narrow passage. The surviving cores are then
// grown back over the region by geodesic distance.

const (
	splitStep        = 0.04 // erosion increment, metres
	splitPersistence = 0.30 // a core must survive this much more erosion to count as a room
	splitMinCore     = 0.20 // m², minimum core area at the level it separates
)

// distanceTransform returns, for every cell in mask, the Euclidean distance
// in metres to the nearest cell outside mask (Felzenszwalb & Huttenlocher).
func distanceTransform(g Grid2, mask []bool) []float64 {
	const inf = 1e18
	f := make([]float64, len(mask))
	for i, m := range mask {
		if m {
			f[i] = inf
		}
	}
	buf := make([]float64, max(g.W, g.H))
	out := make([]float64, max(g.W, g.H))
	for x := 0; x < g.W; x++ {
		for y := 0; y < g.H; y++ {
			buf[y] = f[g.Idx(x, y)]
		}
		edt1d(buf[:g.H], out[:g.H])
		for y := 0; y < g.H; y++ {
			f[g.Idx(x, y)] = out[y]
		}
	}
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			buf[x] = f[g.Idx(x, y)]
		}
		edt1d(buf[:g.W], out[:g.W])
		for x := 0; x < g.W; x++ {
			f[g.Idx(x, y)] = math.Sqrt(out[x]) * g.Res
		}
	}
	return f
}

// edt1d is the 1D squared distance transform of sampled function f.
func edt1d(f, d []float64) {
	n := len(f)
	v := make([]int, n)
	z := make([]float64, n+1)
	k := 0
	z[0], z[1] = math.Inf(-1), math.Inf(1)
	for q := 1; q < n; q++ {
		for {
			p := v[k]
			s := ((f[q] + float64(q*q)) - (f[p] + float64(p*p))) / float64(2*q-2*p)
			if s <= z[k] {
				k--
				if k >= 0 {
					continue
				}
				k = 0
			} else {
				k++
			}
			v[k] = q
			z[k] = s
			z[k+1] = math.Inf(1)
			break
		}
	}
	k = 0
	for q := 0; q < n; q++ {
		for z[k+1] < float64(q) {
			k++
		}
		d[q] = float64((q-v[k])*(q-v[k])) + f[v[k]]
	}
}

// splitRegion returns the room cores inside cells (raster indices) using
// distances dist. A single core means the region is one room.
func splitRegion(r *Rasters, cells []int, dist []float64) [][]int {
	in := make([]bool, r.W*r.H)
	for _, i := range cells {
		in[i] = true
	}
	return splitAt(r, cells, in, dist, 0)
}

func splitAt(r *Rasters, comp []int, in []bool, dist []float64, level float64) [][]int {
	next := level + splitStep
	var children [][]int
	seen := map[int]bool{}
	for _, i := range comp {
		if seen[i] || dist[i] <= next {
			continue
		}
		c := flood(r.Grid2, i, func(j int) bool { return in[j] && dist[j] > next })
		for _, j := range c {
			seen[j] = true
		}
		children = append(children, c)
	}
	var sig [][]int
	for _, c := range children {
		peak := 0.0
		for _, j := range c {
			peak = math.Max(peak, dist[j])
		}
		if peak >= next+splitPersistence && float64(len(c))*r.Res*r.Res >= splitMinCore {
			sig = append(sig, c)
		}
	}
	switch len(sig) {
	case 0:
		return [][]int{comp}
	case 1:
		if sub := splitAt(r, sig[0], in, dist, next); len(sub) > 1 {
			return sub
		}
		return [][]int{comp}
	}
	var out [][]int
	for _, c := range sig {
		out = append(out, splitAt(r, c, in, dist, next)...)
	}
	return out
}

// growSeeds assigns every cell of the region to the seed it is closest to
// by geodesic (4-connected) distance within the region.
func growSeeds(r *Rasters, cells []int, seeds [][]int) map[int]int {
	in := make(map[int]bool, len(cells))
	for _, i := range cells {
		in[i] = true
	}
	owner := make(map[int]int, len(cells))
	var queue []int
	for k, s := range seeds {
		for _, i := range s {
			owner[i] = k
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		x, y := i%r.W, i/r.W
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= r.W || ny >= r.H {
				continue
			}
			j := r.Idx(nx, ny)
			if _, done := owner[j]; !done && in[j] {
				owner[j] = owner[i]
				queue = append(queue, j)
			}
		}
	}
	return owner
}
