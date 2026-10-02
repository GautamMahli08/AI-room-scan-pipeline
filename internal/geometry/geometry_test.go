package geometry

import (
	"math"
	"math/rand"
	"testing"

	"roomscan/internal/geom"
)

func TestEstimateFloorTilted(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var pts []geom.Vec3
	const a, b, c = 0.004, -0.002, -1.45
	for i := 0; i < 20000; i++ {
		x, z := rng.Float64()*6, rng.Float64()*5
		pts = append(pts, geom.Vec3{x, a*x + b*z + c + rng.NormFloat64()*0.003, z})
	}
	for i := 0; i < 8000; i++ { // furniture and walls above the floor
		pts = append(pts, geom.Vec3{rng.Float64() * 6, c + 0.3 + rng.Float64()*2, rng.Float64() * 5})
	}
	f, err := EstimateFloor(pts)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(f.A-a) > 2e-4 || math.Abs(f.B-b) > 2e-4 || math.Abs(f.C-c) > 2e-3 {
		t.Errorf("floor %+v, want A=%v B=%v C=%v", f, a, b, c)
	}
	if f.Residual > 0.005 {
		t.Errorf("residual %v", f.Residual)
	}
}

func TestDistanceTransformMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	g := Grid2{W: 23, H: 17, Res: 0.1}
	mask := make([]bool, g.W*g.H)
	for i := range mask {
		mask[i] = rng.Float64() < 0.8
	}
	d := distanceTransform(g, mask)
	for i, m := range mask {
		want := 0.0
		if m {
			want = math.Inf(1)
			for j, n := range mask {
				if !n {
					dx, dy := float64(i%g.W-j%g.W), float64(i/g.W-j/g.W)
					want = math.Min(want, math.Hypot(dx, dy)*g.Res)
				}
			}
		}
		if !math.IsInf(want, 1) && math.Abs(d[i]-want) > 1e-9 {
			t.Fatalf("cell %d: got %v want %v", i, d[i], want)
		}
	}
}

func TestTraceBoundaryLShape(t *testing.T) {
	// L shape: 4x2 cells bottom row plus a 2x2 block on the left above it.
	g := Grid2{W: 6, H: 6, Res: 1}
	mask := make([]bool, g.W*g.H)
	for y := 1; y < 5; y++ {
		for x := 1; x < 5; x++ {
			mask[g.Idx(x, y)] = y < 3 || x < 3
		}
	}
	p := traceGrid(g, mask, func(i int) float64 { return float64(i) }, func(j int) float64 { return float64(j) })
	want := []Pt{{1, 1}, {5, 1}, {5, 3}, {3, 3}, {3, 5}, {1, 5}}
	if len(p) != len(want) {
		t.Fatalf("got %v, want %v", p, want)
	}
	for i := range want {
		if p[i] != want[i] {
			t.Fatalf("got %v, want %v", p, want)
		}
	}
	if a := PolygonArea(p); a != 12 {
		t.Errorf("area %v, want 12 (counter-clockwise)", a)
	}
}

// synthRoom samples a 4.0 x 3.0 m room rotated by rot about the vertical,
// floor at y=-1.4, walls to 2.5 m, with a 0.90 m door (to 2.05 m) in the
// first long wall starting 1.0 m from the corner, as a voxelised point set.
func synthRoom(rot float64) (pts []geom.Vec3, traj [][2]float64, toWorld func(x, y float64) (float64, float64)) {
	const (
		w, d    = 4.0, 3.0
		floorY  = -1.4
		step    = 0.02
		doorX0  = 1.0
		doorW   = 0.90
		doorTop = 2.05
	)
	c, s := math.Cos(rot), math.Sin(rot)
	// Room-local (u, v) on the floor -> world x, z (plan y = -z).
	toWorld = func(u, v float64) (float64, float64) { return c*u - s*v, -(s*u + c*v) }
	add := func(u, v, h float64) {
		x, z := toWorld(u, v)
		pts = append(pts, geom.Vec3{x, floorY + h, z})
	}
	for u := step / 2; u < w; u += step {
		for v := step / 2; v < d; v += step {
			add(u, v, 0)
		}
	}
	// Integer steps so the jambs at exactly 1.00 m and 1.90 m are sampled.
	n := func(x float64) int { return int(math.Round(x / step)) }
	for hi := 0; hi <= n(2.5); hi++ {
		h := float64(hi) * step
		for ui := 0; ui <= n(w); ui++ {
			u := float64(ui) * step
			if !(ui > n(doorX0) && ui < n(doorX0+doorW) && h < doorTop) {
				add(u, 0, h)
			}
			add(u, d, h)
		}
		for vi := 0; vi <= n(d); vi++ {
			v := float64(vi) * step
			add(0, v, h)
			add(w, v, h)
		}
	}
	// Floor seen through the door, and a walk that goes out through it.
	for u := doorX0; u < doorX0+doorW; u += step {
		for v := -1.0; v < 0; v += step {
			add(u, v, 0)
		}
	}
	for k := 0; k < 200; k++ {
		u, v := 1.0+2.0*float64(k)/200, 1.5
		x, z := toWorld(u, v)
		traj = append(traj, [2]float64{x, -z})
	}
	for k := 0; k < 50; k++ { // through the door
		x, z := toWorld(doorX0+doorW/2, 1.0-1.5*float64(k)/50)
		traj = append(traj, [2]float64{x, -z})
	}
	return pts, traj, toWorld
}

func TestSyntheticRoomEndToEnd(t *testing.T) {
	const rot = 20 * math.Pi / 180
	pts, traj, _ := synthRoom(rot)
	floor, err := EstimateFloor(pts)
	if err != nil {
		t.Fatal(err)
	}
	pf := PlanFrame{Floor: floor}
	theta, _ := EstimateManhattan(BuildRasters(pts, pf, 0.02))
	if d := math.Abs(math.Remainder(theta-rot, math.Pi/2)); d > 0.5*math.Pi/180 {
		t.Fatalf("manhattan θ=%.2f°, want %.2f° (mod 90)", theta*180/math.Pi, rot*180/math.Pi)
	}
	pf.Theta = theta
	pf.Theta = RefineManhattan(pts, pf)
	theta = pf.Theta
	if d := math.Abs(math.Remainder(theta-rot, math.Pi/2)); d > 0.05*math.Pi/180 {
		t.Errorf("refined θ=%.3f°, want %.3f° ±0.05°", theta*180/math.Pi, rot*180/math.Pi)
	}
	// Trajectory into the plan frame.
	c, s := math.Cos(-theta), math.Sin(-theta)
	for i, p := range traj {
		traj[i] = [2]float64{c*p[0] - s*p[1], s*p[0] + c*p[1]}
	}

	r := BuildRasters(pts, pf, 0.02)
	wall := r.WallMask()
	orient := Orientations(r, wall)
	gaps := FindGaps(r, orient, 0.02, 1.6)
	free := make([]uint16, r.W*r.H)
	labels, regions := SegmentRooms(r, wall, gaps, free, traj, 0.25)
	if len(regions) != 1 {
		t.Fatalf("got %d rooms, want 1", len(regions))
	}
	wp := IndexWallPoints(pts, pf, r.Grid2)
	xs, ys := WallLines(r, orient)
	arr := BuildArrangement(r, CleanLabels(r, regions), xs, ys)
	shape := FitRoom(arr, regions[0].Label, wp)
	if shape == nil || len(shape.Corners) != 4 {
		t.Fatalf("want a 4-corner outline, got %+v", shape)
	}
	var lengths []float64
	for i := range shape.Corners {
		lengths = append(lengths, shape.Corners[(i+1)%4].Sub(shape.Corners[i]).Norm())
		if shape.Walls[i].Inferred {
			t.Errorf("wall %d inferred", i)
		}
	}
	long, short := 0, 0
	for _, l := range lengths {
		switch {
		case math.Abs(l-4.0) < 0.01:
			long++
		case math.Abs(l-3.0) < 0.01:
			short++
		}
	}
	t.Logf("wall lengths %.4f, area %.4f", lengths, PolygonArea(shape.Corners))
	if long != 2 || short != 2 {
		t.Errorf("wall lengths %v, want 2x4.00 and 2x3.00 within 1 cm", lengths)
	}
	if a := PolygonArea(shape.Corners); math.Abs(a-12) > 0.08 {
		t.Errorf("area %.3f, want 12 ±0.08", a)
	}

	ops := AssignOpenings(r, gaps, []*RoomShape{shape}, labels, wp, traj)
	var doors []Opening
	for _, o := range ops {
		if o.Kind == KindDoor || o.Kind == KindOpening {
			doors = append(doors, o)
		}
	}
	if len(doors) != 1 {
		t.Fatalf("got openings %+v, want exactly one door", ops)
	}
	if doors[0].Kind != KindDoor || !doors[0].Crossed {
		t.Errorf("door kind %v crossed %v, want door crossed", doors[0].Kind, doors[0].Crossed)
	}
	t.Logf("door width %.4f (jamb points %d)", doors[0].Width, doors[0].JambPoints)
	if math.Abs(doors[0].Width-0.90) > 0.01 {
		t.Errorf("door width %.3f, want 0.90 ±0.01", doors[0].Width)
	}
}

func TestRemoveSmallFeatures(t *testing.T) {
	// 4 x 3 room with a 0.8 m wide, 0.15 m deep bump on the top wall, a
	// 0.2 m wide, 2 m long leak strip on the right wall, and a 0.5 m deep
	// alcove on the bottom wall that must stay.
	p := []Pt{
		{0, 0}, {1, 0}, {1, -0.5}, {2, -0.5}, {2, 0}, // alcove (kept)
		{4, 0}, {4, 1}, {6, 1}, {6, 1.2}, {4, 1.2}, // leak strip (removed)
		{4, 3}, {2.5, 3}, {2.5, 3.15}, {1.7, 3.15}, {1.7, 3}, // bump (removed)
		{0, 3},
	}
	got := removeSmallFeatures(p, 0.30)
	want := []Pt{{0, 0}, {1, 0}, {1, -0.5}, {2, -0.5}, {2, 0}, {4, 0}, {4, 3}, {0, 3}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Sub(want[i]).Norm() > 1e-9 {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if a := PolygonArea(got); math.Abs(a-12.5) > 1e-9 {
		t.Errorf("area %v, want 12.5", a)
	}
}
