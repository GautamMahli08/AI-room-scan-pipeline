package geometry

import (
	"math"

	"roomscan/internal/geom"
)

// Ceiling is the ceiling estimate of one room.
type Ceiling struct {
	Observed bool
	Height   float64 // metres above the floor plane
	Spread   float64 // std of the ceiling points about Height
	Support  int     // points in the ceiling layer
	Coverage float64 // fraction of the room's plan cells with ceiling points
	Reason   string  // why it is unobserved
	// OtherLevels are further ceiling heights covering a significant part
	// of the room (bulkheads, a corridor merged into an open-plan room).
	OtherLevels []float64
}

const (
	ceilMinH        = 1.9  // lowest plausible ceiling
	ceilMaxH        = 4.5  // highest considered
	ceilBand        = 0.02 // points within ± this of the peak define the layer
	ceilMinCoverage = 0.20 // a ceiling must be seen over this much of the room
	ceilCell        = 0.10 // plan cell size for coverage
)

// EstimateCeilings finds, per room label, the horizontal layer above
// ceilMinH that covers most of the room. A layer covering less than ceilMinCoverage of the room
// (e.g. the top of a wardrobe) is not a ceiling; the room is then reported
// as unobserved.
func EstimateCeilings(pts []geom.Vec3, f PlanFrame, r *Rasters, labels []int32, nRooms int) []Ceiling {
	const bin = 0.01
	nb := int((ceilMaxH-ceilMinH)/bin) + 1
	hist := make([][]int, nRooms)
	for i := range hist {
		hist[i] = make([]int, nb)
	}
	type hp struct {
		h    float64
		cell int
	}
	pointsOf := make([][]hp, nRooms)
	area := make([]int, nRooms)
	for _, l := range labels {
		if l > 0 {
			area[l-1]++
		}
	}
	for _, p := range pts {
		x, y, h := f.ToPlan(p)
		if h < ceilMinH || h >= ceilMaxH {
			continue
		}
		cx, cy, ok := r.Cell(x, y)
		if !ok {
			continue
		}
		l := labels[r.Idx(cx, cy)]
		if l == 0 {
			continue
		}
		hist[l-1][int((h-ceilMinH)/bin)]++
		coarse := int(math.Floor(x/ceilCell))*100003 + int(math.Floor(y/ceilCell))
		pointsOf[l-1] = append(pointsOf[l-1], hp{h, coarse})
	}

	out := make([]Ceiling, nRooms)
	for k := range out {
		smooth := make([]int, nb)
		maxN := 0
		for i := range hist[k] {
			n := hist[k][i]
			if i > 0 {
				n += hist[k][i-1]
			}
			if i+1 < nb {
				n += hist[k][i+1]
			}
			smooth[i] = n
			maxN = max(maxN, n)
		}
		if maxN == 0 {
			out[k] = Ceiling{Reason: "no points above 1.9 m were captured in this room"}
			continue
		}
		roomCells := float64(area[k]) * r.Res * r.Res / (ceilCell * ceilCell)
		// Candidate layers are local maxima holding at least 10% of the
		// strongest; the ceiling is the one covering most of the room (a
		// dense wardrobe top or bulkhead covers little).
		var layers []Ceiling
		for i := range smooth {
			if smooth[i] < maxN/10 || (i > 0 && smooth[i-1] > smooth[i]) || (i+1 < nb && smooth[i+1] >= smooth[i]) {
				continue
			}
			peak := ceilMinH + (float64(i)+0.5)*bin
			var sum, ss float64
			var n int
			cells := map[int]bool{}
			for _, q := range pointsOf[k] {
				if math.Abs(q.h-peak) <= ceilBand {
					sum += q.h
					ss += q.h * q.h
					n++
					cells[q.cell] = true
				}
			}
			mean := sum / float64(n)
			layers = append(layers, Ceiling{
				Height:   mean,
				Spread:   math.Sqrt(math.Max(ss/float64(n)-mean*mean, 0)),
				Support:  n,
				Coverage: math.Min(1, float64(len(cells))/roomCells),
			})
		}
		best := 0
		for i, l := range layers {
			if l.Coverage > layers[best].Coverage {
				best = i
			}
		}
		c := layers[best]
		for i, l := range layers {
			if i == best || l.Coverage < ceilMinCoverage || math.Abs(l.Height-c.Height) <= 0.1 {
				continue
			}
			dup := false
			for _, h := range c.OtherLevels {
				dup = dup || math.Abs(h-l.Height) <= 0.05
			}
			if !dup {
				c.OtherLevels = append(c.OtherLevels, l.Height)
			}
		}
		c.Observed = c.Coverage >= ceilMinCoverage
		if !c.Observed {
			c.Reason = "highest horizontal layer covers too little of the room to be the ceiling (ceiling not scanned)"
		}
		out[k] = c
	}
	return out
}
