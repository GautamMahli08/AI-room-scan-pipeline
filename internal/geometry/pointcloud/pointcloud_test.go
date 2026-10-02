package pointcloud

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"roomscan/internal/frame"
	"roomscan/internal/geom"
	"roomscan/internal/ingest/strayscanner"
)

func TestKeyRoundTrip(t *testing.T) {
	for _, c := range [][3]int{{0, 0, 0}, {-1, 2, -3}, {500, -500, 1000}, {-keyOff, keyOff - 1, 0}} {
		x, y, z := packKey(c[0], c[1], c[2]).Unpack()
		if x != c[0] || y != c[1] || z != c[2] {
			t.Errorf("round trip %v -> %d,%d,%d", c, x, y, z)
		}
	}
}

func TestFloorDiv(t *testing.T) {
	for _, c := range []struct {
		x    float64
		want int
	}{{0, 0}, {0.019, 0}, {0.02, 1}, {-0.001, -1}, {-0.02, -1}, {-0.021, -2}} {
		if got := floorDiv(c.x, 0.02); got != c.want {
			t.Errorf("floorDiv(%v) = %d, want %d", c.x, got, c.want)
		}
	}
}

// A camera at height 1.5 m looking straight down (OpenCV axes: z forward =
// world -Y) sees a flat floor 1.5 m away. Every back-projected point must
// land on y = 0 at the right horizontal offset.
func TestBackProjectFlatFloor(t *testing.T) {
	const w, h = 8, 6
	k := frame.Intrinsics{Fx: 5, Fy: 5, Cx: 3.5, Cy: 2.5, Width: w, Height: h}
	d := &frame.DepthMap{Width: w, Height: h, Z: make([]float32, w*h)}
	for i := range d.Z {
		d.Z[i] = 1.5
	}
	// camera x -> world x, camera y (down in image) -> world z, camera z -> world -y
	pose := frame.Pose{R: geom.Mat3{{1, 0, 0}, {0, 0, -1}, {0, 1, 0}}, T: geom.Vec3{0, 1.5, 0}}
	pts := BackProject(nil, d, nil, k, pose, 0, 0.1, 5, OpenCV)
	if len(pts) != w*h {
		t.Fatalf("got %d points, want %d", len(pts), w*h)
	}
	for i, p := range pts {
		u, v := float64(i%w), float64(i/w)
		want := geom.Vec3{(u - k.Cx) * 1.5 / k.Fx, 0, (v - k.Cy) * 1.5 / k.Fy}
		if p.Sub(want).Norm() > 1e-9 {
			t.Fatalf("pixel (%v,%v): got %v want %v", u, v, p, want)
		}
	}
}

func TestBackProjectFilters(t *testing.T) {
	k := frame.Intrinsics{Fx: 1, Fy: 1, Width: 3, Height: 1}
	d := &frame.DepthMap{Width: 3, Height: 1, Z: []float32{1, 0, 9}}
	c := &frame.ConfMap{Width: 3, Height: 1, C: []uint8{2, 2, 2}}
	pose := frame.Pose{R: geom.Identity3()}
	if n := len(BackProject(nil, d, c, k, pose, 2, 0.1, 5, OpenCV)); n != 1 {
		t.Errorf("depth filter: got %d points, want 1", n)
	}
	c.C[0] = 1
	if n := len(BackProject(nil, d, c, k, pose, 2, 0.1, 5, OpenCV)); n != 0 {
		t.Errorf("confidence filter: got %d points, want 0", n)
	}
}

func TestFuseCountsDistinctFrames(t *testing.T) {
	src := &memSource{}
	for i := 0; i < 3; i++ {
		d := &frame.DepthMap{Width: 2, Height: 2, Z: []float32{1, 1, 1, 1}}
		src.frames = append(src.frames, frame.Frame{Index: i, Pose: &frame.Pose{R: geom.Identity3()},
			K: frame.Intrinsics{Fx: 1000, Fy: 1000, Cx: -0.5, Cy: -0.5, Width: 2, Height: 2}})
		src.depth = append(src.depth, d)
	}
	opt := DefaultFuseOptions()
	opt.MinConf = 0
	g, err := Fuse(src, opt)
	if err != nil {
		t.Fatal(err)
	}
	// All 12 points fall in a single voxel (pixels are 1 mm apart at 1 m).
	if g.Len() != 1 || g.Cells[0].N != 12 || g.Cells[0].Frames != 3 {
		t.Fatalf("got %d voxels, first %+v; want 1 voxel with N=12 Frames=3", g.Len(), g.Cells[0])
	}
}

type memSource struct {
	frames []frame.Frame
	depth  []*frame.DepthMap
}

func (m *memSource) NumFrames() int            { return len(m.frames) }
func (m *memSource) FrameAt(i int) frame.Frame { return m.frames[i] }
func (m *memSource) LoadDepth(i int) (*frame.DepthMap, *frame.ConfMap, error) {
	return m.depth[i], nil, nil
}

// floorSharpness fuses a capture and returns the fraction of voxels in the
// densest 4 cm horizontal slab in the lower half of the scene: a correctly
// back-projected room has a single dominant floor plane.
func floorSharpness(t *testing.T, c *strayscanner.Capture, conv Convention) float64 {
	opt := DefaultFuseOptions()
	opt.Stride = 30
	opt.Convention = conv
	g, err := Fuse(c, opt)
	if err != nil {
		t.Fatal(err)
	}
	pts := g.Points(1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	const bin = 0.04
	hist := make([]int, int((maxY-minY)/bin)+1)
	for _, p := range pts {
		hist[int((p[1]-minY)/bin)]++
	}
	best := 0
	for _, n := range hist[:len(hist)/2] {
		best = max(best, n)
	}
	return float64(best) / float64(len(pts))
}

// TestConventionOnSample checks SYSTEM_DESIGN.md §4.2's convention finding on
// real data: OpenCV axes give a sharp floor below the camera, ARKit axes do
// not. Skipped when the sample capture is not present.
func TestConventionOnSample(t *testing.T) {
	dirs, _ := filepath.Glob("../../../single_room/*")
	if len(dirs) == 0 {
		t.Skip("sample capture single_room not present")
	}
	if _, err := os.Stat(filepath.Join(dirs[0], "odometry.csv")); err != nil {
		t.Skip("sample capture incomplete")
	}
	c, err := strayscanner.Load(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	cv, ar := floorSharpness(t, c, OpenCV), floorSharpness(t, c, ARKit)
	t.Logf("floor slab fraction: OpenCV %.3f, ARKit %.3f", cv, ar)
	if cv < 2*ar {
		t.Errorf("OpenCV convention should give a much sharper floor: %.3f vs %.3f", cv, ar)
	}
}

// The same capture must fuse to exactly the same cloud whatever order the
// workers finish in.
func TestFuseDeterministic(t *testing.T) {
	dirs, _ := filepath.Glob("../../../single_room/*")
	if len(dirs) == 0 {
		t.Skip("sample capture single_room not present")
	}
	c, err := strayscanner.Load(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	opt := DefaultFuseOptions()
	opt.Stride = 20
	var ref []geom.Vec3
	for _, workers := range []int{1, 3, 8} {
		opt.Workers = workers
		g, err := Fuse(c, opt)
		if err != nil {
			t.Fatal(err)
		}
		pts := g.Points(1)
		if ref == nil {
			ref = pts
			continue
		}
		if len(pts) != len(ref) {
			t.Fatalf("workers=%d: %d points, want %d", workers, len(pts), len(ref))
		}
		for i := range pts {
			if pts[i] != ref[i] {
				t.Fatalf("workers=%d: point %d differs: %v vs %v", workers, i, pts[i], ref[i])
			}
		}
	}
}
