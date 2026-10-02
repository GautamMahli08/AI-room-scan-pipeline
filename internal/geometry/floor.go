// Package geometry turns a fused point cloud into architectural elements:
// floor, Manhattan frame, walls, openings, rooms and ceilings.
package geometry

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/mat"

	"roomscan/internal/geom"
)

// Floor is the fitted floor plane y = A·x + B·z + C in ARKit world
// coordinates (Y is gravity-aligned, up).
type Floor struct {
	A, B, C  float64
	Residual float64 // RMS distance of inliers to the plane, metres
	Inliers  int
	TiltDeg  float64 // angle between plane normal and gravity
}

// Height returns the height of p above the floor plane.
func (f Floor) Height(p geom.Vec3) float64 { return p[1] - (f.A*p[0] + f.B*p[2] + f.C) }

// EstimateFloor finds the floor as the strongest horizontal slab in the
// lower half of the scene (1 cm histogram) and refines it with a least
// squares plane fit on the points within ±3 cm of that slab.
func EstimateFloor(pts []geom.Vec3) (Floor, error) {
	if len(pts) < 100 {
		return Floor{}, errors.New("floor: too few points")
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		lo, hi = math.Min(lo, p[1]), math.Max(hi, p[1])
	}
	const bin = 0.01
	hist := make([]int, int((hi-lo)/bin)+1)
	for _, p := range pts {
		hist[int((p[1]-lo)/bin)]++
	}
	// Smooth over 3 bins so a floor straddling a bin edge is not split.
	best, bestN := 0, -1
	for i := 1; i < len(hist)/2; i++ {
		n := hist[i-1] + hist[i] + hist[i+1]
		if n > bestN {
			best, bestN = i, n
		}
	}
	y0 := lo + (float64(best)+0.5)*bin

	f := Floor{C: y0}
	// Two passes: fit on a ±3 cm slab, then on ±2 cm around the fitted plane.
	for _, tol := range []float64{0.03, 0.02} {
		var in []geom.Vec3
		for _, p := range pts {
			if math.Abs(f.Height(p)) <= tol {
				in = append(in, p)
			}
		}
		if len(in) < 50 {
			return Floor{}, errors.New("floor: too few inliers")
		}
		a := mat.NewDense(len(in), 3, nil)
		b := mat.NewVecDense(len(in), nil)
		for i, p := range in {
			a.Set(i, 0, p[0])
			a.Set(i, 1, p[2])
			a.Set(i, 2, 1)
			b.SetVec(i, p[1])
		}
		var x mat.VecDense
		if err := x.SolveVec(a, b); err != nil {
			return Floor{}, err
		}
		f = Floor{A: x.AtVec(0), B: x.AtVec(1), C: x.AtVec(2), Inliers: len(in)}
		var ss float64
		for _, p := range in {
			ss += f.Height(p) * f.Height(p)
		}
		f.Residual = math.Sqrt(ss / float64(len(in)))
	}
	f.TiltDeg = math.Atan(math.Hypot(f.A, f.B)) * 180 / math.Pi
	return f, nil
}
