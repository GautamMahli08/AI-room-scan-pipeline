package strayscanner

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSynthetic(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"depth", "confidence"} {
		os.Mkdir(filepath.Join(dir, sub), 0o755)
	}
	os.WriteFile(filepath.Join(dir, "camera_matrix.csv"), []byte("1600.0, 0.0, 959.5\n0.0, 1600.0, 719.5\n0.0, 0.0, 1.0"), 0o644)
	os.WriteFile(filepath.Join(dir, "odometry.csv"), []byte(
		"timestamp, frame, x, y, z, qx, qy, qz, qw, fx, fy, cx, cy, distortion_center_x, distortion_center_y\n"+
			"1.0, 000000, 0.1, 0.2, 0.3, 0, 0, 0, 1, 1500, 1500, 959.5, 719.5, , \n"+
			"1.5, 000001, 0.1, 0.2, 0.3, 0, 0.7071068, 0, 0.7071068, , , , , , \n"), 0o644)

	for i := 0; i < 2; i++ {
		d := image.NewGray16(image.Rect(0, 0, 256, 192))
		d.Pix[0], d.Pix[1] = 0x05, 0xDC // 1500 mm at pixel (0,0)
		c := image.NewGray(image.Rect(0, 0, 256, 192))
		c.Pix[0] = 2
		name := []string{"000000.png", "000001.png"}[i]
		writePNG(t, filepath.Join(dir, "depth", name), d)
		writePNG(t, filepath.Join(dir, "confidence", name), c)
	}

	cap, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cap.Frames) != 2 || cap.DepthWidth != 256 || cap.DepthHeight != 192 {
		t.Fatalf("frames=%d depth=%dx%d", len(cap.Frames), cap.DepthWidth, cap.DepthHeight)
	}

	// Frame 0 uses its own intrinsics; frame 1 has none and falls back to camera_matrix.csv.
	s := 256.0 / 1920
	k0, k1 := cap.Frames[0].K, cap.Frames[1].K
	if math.Abs(k0.Fx-1500*s) > 1e-9 || math.Abs(k1.Fx-1600*s) > 1e-9 {
		t.Errorf("fx: frame0 %v (want %v), frame1 %v (want %v)", k0.Fx, 1500*s, k1.Fx, 1600*s)
	}
	// Optical centre exactly at image centre must stay at image centre.
	if math.Abs(k0.Cx-127.5) > 1e-9 || math.Abs(k0.Cy-95.5) > 1e-9 {
		t.Errorf("principal point %v,%v, want 127.5,95.5", k0.Cx, k0.Cy)
	}
	// 90° about Y maps +X to -Z.
	p := cap.Frames[1].Pose.R.MulVec([3]float64{1, 0, 0})
	if math.Abs(p[2]+1) > 1e-6 {
		t.Errorf("rotation: R·x = %v, want (0,0,-1)", p)
	}

	d, c, err := cap.LoadDepth(0)
	if err != nil {
		t.Fatal(err)
	}
	if d.At(0, 0) != 1.5 || d.At(1, 0) != 0 || c.At(0, 0) != 2 {
		t.Errorf("depth(0,0)=%v depth(1,0)=%v conf(0,0)=%v", d.At(0, 0), d.At(1, 0), c.At(0, 0))
	}
}

func TestLoadMissingDepth(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "depth"), 0o755)
	os.WriteFile(filepath.Join(dir, "camera_matrix.csv"), []byte("1,0,1\n0,1,1\n0,0,1"), 0o644)
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for capture without depth frames")
	}
}
