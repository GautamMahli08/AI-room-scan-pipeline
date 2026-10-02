package damage

import (
	"math"
	"testing"

	"roomscan/internal/frame"
	"roomscan/internal/geom"
	"roomscan/internal/geometry"
)

// A 4 x 3 m room (plan coordinates) with the camera 1.5 m from the y = 3
// wall, looking straight at it from 1.5 m height. Depth is the wall.
type wallSource struct{ f frame.Frame }

func (s wallSource) FrameAt(int) frame.Frame { return s.f }
func (s wallSource) LoadDepth(int) (*frame.DepthMap, *frame.ConfMap, error) {
	d := &frame.DepthMap{Width: 256, Height: 192, Z: make([]float32, 256*192)}
	c := &frame.ConfMap{Width: 256, Height: 192, C: make([]uint8, 256*192)}
	for i := range d.Z {
		d.Z[i], c.C[i] = 1.5, 2
	}
	return d, c, nil
}

func room() *geometry.RoomShape {
	return &geometry.RoomShape{
		Corners: []geometry.Pt{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 3}, {X: 0, Y: 3}},
		Walls: []geometry.WallFit{
			{Line: geometry.Line{N: geometry.Pt{X: 0, Y: -1}, C: 0}},
			{Line: geometry.Line{N: geometry.Pt{X: 1, Y: 0}, C: 4}},
			{Line: geometry.Line{N: geometry.Pt{X: 0, Y: 1}, C: 3}},
			{Line: geometry.Line{N: geometry.Pt{X: -1, Y: 0}, C: 0}},
		},
	}
}

func TestPlaceOnWall(t *testing.T) {
	// Plan (x, y) = world (x, -z); camera at plan (2, 1.5), 1.5 m up,
	// looking along plan +y (world -z). OpenCV axes: x right, y down, z forward.
	pose := frame.Pose{R: geom.Mat3{{1, 0, 0}, {0, -1, 0}, {0, 0, -1}}, T: geom.Vec3{2, 1.5, -1.5}}
	src := wallSource{frame.Frame{Pose: &pose, K: frame.Intrinsics{Fx: 200, Fy: 200, Cx: 128, Cy: 96, Width: 256, Height: 192}}}
	pf := geometry.PlanFrame{}
	box := [4]float64{108, 76, 148, 116} // 40 px square at the image centre
	dets := []Detection{
		{Frame: 10, Class: "water_stain", Score: 0.6, Box: box},
		{Frame: 20, Class: "water_stain", Score: 0.5, Box: box},
		{Frame: 30, Class: "crack", Score: 0.2, Box: box}, // below the score threshold
	}
	idx := map[int]int{10: 0, 20: 0, 30: 0}
	got := Place(dets, src, idx, 256, 192, pf, []*geometry.RoomShape{room()}, []geometry.Ceiling{{}})
	if len(got) != 1 {
		t.Fatalf("got %d regions, want 1: %+v", len(got), got)
	}
	r := got[0]
	if r.Surface != (Surface{Room: 0, Wall: 2}) || r.Views != 2 || r.Class != "water_stain" {
		t.Fatalf("region %+v, want water_stain on wall 2 seen in 2 views", r)
	}
	// The central 80 % of a 40 px box at 1.5 m and f = 200 spans 0.24 m;
	// the 10-90 % extent of the pixels inside is ~0.19 m, centred on x = 2
	// (along the wall from (4,3) towards (0,3): t = 2) and h = 1.5.
	if math.Abs((r.U0+r.U1)/2-2) > 0.02 || math.Abs((r.V0+r.V1)/2-1.5) > 0.02 {
		t.Errorf("centre (%.3f, %.3f), want (2, 1.5)", (r.U0+r.U1)/2, (r.V0+r.V1)/2)
	}
	if r.Width() < 0.17 || r.Width() > 0.24 || r.Height() < 0.17 || r.Height() > 0.24 {
		t.Errorf("extent %.3f x %.3f, want ~0.19 x 0.19", r.Width(), r.Height())
	}

	// One view only: not reported.
	if got := Place(dets[:1], src, idx, 256, 192, pf, []*geometry.RoomShape{room()}, []geometry.Ceiling{{}}); len(got) != 0 {
		t.Errorf("single-view detection reported: %+v", got)
	}
}

func TestRules(t *testing.T) {
	rs := []Region{
		{Class: "water_stain", Surface: Surface{0, SurfCeiling}},
		{Class: "mould", Surface: Surface{0, 1}, V0: 0.1, V1: 0.4},
		{Class: "crack", Surface: Surface{0, 2}, U0: 1.85, U1: 2.3, V0: 2.1, V1: 2.6},
		{Class: "water_stain", Surface: Surface{0, 1}, V0: 1.2, V1: 1.5}, // mid-wall: no rule
	}
	ops := []Opening{{Surface: Surface{0, 2}, U0: 1.0, U1: 1.9, HeadV: 2.05}}
	fl := Rules(rs, ops)
	want := map[int]string{0: "R-WET-CEIL-01", 1: "R-WALL-BASE-01", 2: "R-CRACK-DIAG-01"}
	if len(fl) != len(want) {
		t.Fatalf("got flags %+v, want %v", fl, want)
	}
	for _, f := range fl {
		if want[f.Region] != f.RuleID {
			t.Errorf("region %d: rule %s, want %s", f.Region, f.RuleID, want[f.Region])
		}
	}
}
