// Package strayscanner loads Stray Scanner exports (LiDAR tier):
//
//	camera_matrix.csv   3x3 intrinsics at RGB resolution
//	odometry.csv        timestamp, frame, x, y, z, qx, qy, qz, qw, fx, fy, cx, cy, ...
//	depth/NNNNNN.png    uint16 depth in millimetres (256x192)
//	confidence/NNNNNN.png  uint8 confidence 0/1/2
//	rgb.mp4             RGB video, one frame per odometry row
package strayscanner

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"roomscan/internal/frame"
	"roomscan/internal/geom"
)

// RGB resolution of Stray Scanner video. Used to interpret intrinsics, which
// are given at RGB resolution.
const (
	rgbWidth  = 1920
	rgbHeight = 1440
)

// Capture is a loaded Stray Scanner export. Frames carry poses and
// intrinsics already scaled to depth resolution; depth is loaded on demand.
type Capture struct {
	Meta        Meta
	Dir         string
	Frames      []frame.Frame
	DepthWidth  int
	DepthHeight int
	RGBK        frame.Intrinsics // camera_matrix.csv, at RGB resolution
}

// Load parses the CSVs and checks that depth/confidence exist for every
// frame. It does not decode any depth images.
func Load(dir string) (*Capture, error) {
	c := &Capture{Dir: dir}
	var err error
	if c.RGBK, err = readCameraMatrix(filepath.Join(dir, "camera_matrix.csv")); err != nil {
		return nil, err
	}
	if c.Meta, err = readMeta(dir); err != nil {
		return nil, err
	}
	if c.Meta.RGBWidth > 0 {
		c.RGBK.Width, c.RGBK.Height = c.Meta.RGBWidth, c.Meta.RGBHeight
	}

	w, h, err := pngSize(filepath.Join(dir, "depth", "000000.png"))
	if err != nil {
		return nil, fmt.Errorf("probe depth resolution: %w", err)
	}
	c.DepthWidth, c.DepthHeight = w, h

	if c.Frames, err = readOdometry(filepath.Join(dir, "odometry.csv"), c.RGBK, w, h); err != nil {
		return nil, err
	}
	rgb := filepath.Join(dir, "rgb.mp4")
	for i := range c.Frames {
		c.Frames[i].RGBPath = rgb
	}

	for _, f := range c.Frames {
		for _, p := range []string{c.depthPath(f.Index), c.confPath(f.Index)} {
			if _, err := os.Stat(p); err != nil {
				return nil, fmt.Errorf("frame %d: %w", f.Index, err)
			}
		}
	}
	return c, nil
}

// Meta is the optional meta.json written by the video and photo tiers,
// whose exports use the same layout with estimated depth and poses.
// Native Stray Scanner exports have none: tier lidar at 1920x1440.
type Meta struct {
	Tier          string   `json:"tier"`
	RGBWidth      int      `json:"rgb_width"`
	RGBHeight     int      `json:"rgb_height"`
	ScaleRelSigma float64  `json:"scale_rel_sigma"` // relative 1-sigma of metric scale
	Models        []Model  `json:"models"`
	RuntimeS      float64  `json:"runtime_s"`
	Notes         []string `json:"notes"`
}

// Model is a model used to produce an export.
type Model struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func readMeta(dir string) (Meta, error) {
	m := Meta{Tier: "lidar"}
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("meta.json: %w", err)
	}
	return m, nil
}

func (c *Capture) depthPath(i int) string {
	return filepath.Join(c.Dir, "depth", fmt.Sprintf("%06d.png", i))
}

func (c *Capture) confPath(i int) string {
	return filepath.Join(c.Dir, "confidence", fmt.Sprintf("%06d.png", i))
}

func (c *Capture) NumFrames() int            { return len(c.Frames) }
func (c *Capture) FrameAt(i int) frame.Frame { return c.Frames[i] }

// LoadDepth decodes the depth and confidence maps of frame i (by position in
// Frames) and returns them without attaching them to the frame, so callers
// processing frames concurrently do not retain every map.
func (c *Capture) LoadDepth(i int) (*frame.DepthMap, *frame.ConfMap, error) {
	idx := c.Frames[i].Index
	d, err := readDepth(c.depthPath(idx))
	if err != nil {
		return nil, nil, err
	}
	conf, err := readConf(c.confPath(idx))
	if err != nil {
		return nil, nil, err
	}
	if d.Width != conf.Width || d.Height != conf.Height {
		return nil, nil, fmt.Errorf("frame %d: depth %dx%d vs confidence %dx%d", idx, d.Width, d.Height, conf.Width, conf.Height)
	}
	return d, conf, nil
}

func readCameraMatrix(path string) (frame.Intrinsics, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return frame.Intrinsics{}, err
	}
	var m [3][3]float64
	rows := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(rows) != 3 {
		return frame.Intrinsics{}, fmt.Errorf("%s: want 3 rows, got %d", path, len(rows))
	}
	for i, r := range rows {
		cols := strings.Split(r, ",")
		if len(cols) != 3 {
			return frame.Intrinsics{}, fmt.Errorf("%s row %d: want 3 columns", path, i)
		}
		for j, s := range cols {
			if m[i][j], err = strconv.ParseFloat(strings.TrimSpace(s), 64); err != nil {
				return frame.Intrinsics{}, fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	return frame.Intrinsics{Fx: m[0][0], Fy: m[1][1], Cx: m[0][2], Cy: m[1][2], Width: rgbWidth, Height: rgbHeight}, nil
}

// readOdometry parses odometry.csv. Per-frame intrinsics take precedence over
// camera_matrix.csv; both are at RGB resolution and are rescaled to depth.
func readOdometry(path string, fallback frame.Intrinsics, dw, dh int) ([]frame.Frame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimSpace(h)] = i
	}
	for _, k := range []string{"timestamp", "frame", "x", "y", "z", "qx", "qy", "qz", "qw"} {
		if _, ok := col[k]; !ok {
			return nil, fmt.Errorf("%s: missing column %q", path, k)
		}
	}
	_, hasK := col["fx"]

	var frames []frame.Frame
	for line := 2; ; line++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		get := func(k string) (float64, error) {
			return strconv.ParseFloat(strings.TrimSpace(rec[col[k]]), 64)
		}
		var v [9]float64
		for i, k := range []string{"timestamp", "frame", "x", "y", "z", "qx", "qy", "qz", "qw"} {
			if v[i], err = get(k); err != nil {
				return nil, fmt.Errorf("%s:%d %s: %w", path, line, k, err)
			}
		}
		k := fallback
		if hasK {
			fx, e1 := get("fx")
			fy, e2 := get("fy")
			cx, e3 := get("cx")
			cy, e4 := get("cy")
			if e1 == nil && e2 == nil && e3 == nil && e4 == nil && fx > 0 {
				k = frame.Intrinsics{Fx: fx, Fy: fy, Cx: cx, Cy: cy, Width: fallback.Width, Height: fallback.Height}
			}
		}
		pose := &frame.Pose{
			R: geom.Quat{X: v[5], Y: v[6], Z: v[7], W: v[8]}.Mat(),
			T: geom.Vec3{v[2], v[3], v[4]},
		}
		frames = append(frames, frame.Frame{
			Index:     int(v[1]),
			Timestamp: v[0],
			Pose:      pose,
			K:         k.Rescale(dw, dh),
			Source:    frame.LiDAR,
		})
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("%s: no frames", path)
	}
	return frames, nil
}

func pngSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}

func readDepth(path string) (*frame.DepthMap, error) {
	img, err := decodePNG(path)
	if err != nil {
		return nil, err
	}
	g, ok := img.(*image.Gray16)
	if !ok {
		return nil, fmt.Errorf("%s: want 16-bit grayscale depth, got %T", path, img)
	}
	b := g.Bounds()
	d := &frame.DepthMap{Width: b.Dx(), Height: b.Dy(), Z: make([]float32, b.Dx()*b.Dy())}
	for y := 0; y < d.Height; y++ {
		row := g.Pix[y*g.Stride:]
		for x := 0; x < d.Width; x++ {
			mm := uint16(row[2*x])<<8 | uint16(row[2*x+1])
			d.Z[y*d.Width+x] = float32(mm) / 1000
		}
	}
	return d, nil
}

func readConf(path string) (*frame.ConfMap, error) {
	img, err := decodePNG(path)
	if err != nil {
		return nil, err
	}
	g, ok := img.(*image.Gray)
	if !ok {
		return nil, fmt.Errorf("%s: want 8-bit grayscale confidence, got %T", path, img)
	}
	b := g.Bounds()
	c := &frame.ConfMap{Width: b.Dx(), Height: b.Dy(), C: make([]uint8, b.Dx()*b.Dy())}
	for y := 0; y < c.Height; y++ {
		copy(c.C[y*c.Width:(y+1)*c.Width], g.Pix[y*g.Stride:y*g.Stride+c.Width])
	}
	return c, nil
}
