package stitch

import (
	"math"
	"testing"

	"roomscan/internal/frame"
	"roomscan/internal/geom"
	"roomscan/internal/geometry"
)

// Apply must move every observed point in the plan exactly as the model in
// Estimate assumes: p' = R(dθ)(p − pivot) + pivot + d, in plan coordinates.
func TestApplyMatchesPlanModel(t *testing.T) {
	pf := geometry.PlanFrame{Theta: 0.7}
	pose := frame.Pose{R: geom.Quat{X: 0.3, Y: -0.5, Z: 0.1, W: 0.8}.Mat(), T: geom.Vec3{1.2, 0.3, -2.5}}
	frames := []frame.Frame{{Timestamp: 10, Pose: &pose}}
	ch := Chunk{T0: 0, T1: 20, Pivot: geometry.Pt{X: 0.4, Y: -1.1}, Dx: 0.05, Dy: -0.03, Dtheta: 0.02, Dh: 0.01}
	c := &Correction{Chunks: []Chunk{ch}}

	camPts := []geom.Vec3{{0, 0, 2}, {0.5, -0.3, 1.5}, {-1, 0.4, 3}}
	var before [][3]float64
	for _, q := range camPts {
		x, y, h := pf.ToPlan(pose.Apply(q))
		before = append(before, [3]float64{x, y, h})
	}
	Apply(frames, pf, c)
	for i, q := range camPts {
		x, y, h := pf.ToPlan(frames[0].Pose.Apply(q))
		bx, by := before[i][0]-ch.Pivot.X, before[i][1]-ch.Pivot.Y
		cs, sn := math.Cos(ch.Dtheta), math.Sin(ch.Dtheta)
		wx := cs*bx - sn*by + ch.Pivot.X + ch.Dx
		wy := sn*bx + cs*by + ch.Pivot.Y + ch.Dy
		wh := before[i][2] + ch.Dh
		if math.Abs(x-wx) > 1e-9 || math.Abs(y-wy) > 1e-9 || math.Abs(h-wh) > 1e-9 {
			t.Errorf("point %d: got (%.6f, %.6f, %.6f), want (%.6f, %.6f, %.6f)", i, x, y, h, wx, wy, wh)
		}
	}
}
