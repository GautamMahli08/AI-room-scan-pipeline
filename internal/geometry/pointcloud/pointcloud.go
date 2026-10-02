// Package pointcloud back-projects depth frames into world space and fuses
// them into a voxel grid that tracks how many distinct frames observed each
// voxel.
package pointcloud

import (
	"cmp"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sync"

	"roomscan/internal/frame"
	"roomscan/internal/geom"
)

// Convention is the camera axis convention assumed when back-projecting.
type Convention int

const (
	// OpenCV: x right, y down, z forward. Verified correct for Stray Scanner
	// depth with its odometry poses (see TestConventionOnSample).
	OpenCV Convention = iota
	// ARKit: x right, y up, z backward. Kept for the convention test only.
	ARKit
)

// BackProject appends the world-space points of one depth frame to dst.
// Pixels with confidence below minConf or depth outside [minZ, maxZ] are
// skipped. conf may be nil.
func BackProject(dst []geom.Vec3, d *frame.DepthMap, conf *frame.ConfMap, k frame.Intrinsics, pose frame.Pose,
	minConf uint8, minZ, maxZ float64, conv Convention) []geom.Vec3 {
	for v := 0; v < d.Height; v++ {
		for u := 0; u < d.Width; u++ {
			if conf != nil && conf.At(u, v) < minConf {
				continue
			}
			z := float64(d.At(u, v))
			if z < minZ || z > maxZ {
				continue
			}
			p := geom.Vec3{(float64(u) - k.Cx) * z / k.Fx, (float64(v) - k.Cy) * z / k.Fy, z}
			if conv == ARKit {
				p[1], p[2] = -p[1], -p[2]
			}
			dst = append(dst, pose.Apply(p))
		}
	}
	return dst
}

// Key identifies a voxel by integer grid coordinates packed into 63 bits
// (21 bits per axis, offset so negative coordinates are representable).
type Key int64

const (
	keyBits = 21
	keyOff  = 1 << (keyBits - 1)
	keyMask = 1<<keyBits - 1
)

func packKey(ix, iy, iz int) Key {
	return Key(int64(ix+keyOff)&keyMask<<(2*keyBits) | int64(iy+keyOff)&keyMask<<keyBits | int64(iz+keyOff)&keyMask)
}

// Unpack returns the integer grid coordinates of k.
func (k Key) Unpack() (int, int, int) {
	return int(int64(k)>>(2*keyBits)&keyMask) - keyOff,
		int(int64(k)>>keyBits&keyMask) - keyOff,
		int(int64(k)&keyMask) - keyOff
}

// Voxel accumulates the points that fell into one grid cell. Positions
// are summed as integer micrometres so the result does not depend on the
// order frames are merged in (float addition is not associative, and
// workers finish in arbitrary order): the same capture always fuses to the
// same cloud.
type Voxel struct {
	SumUM  [3]int64 // sum of point positions, micrometres
	N      int32    // number of points
	Frames int32    // number of distinct frames that observed this voxel
}

// Centroid is the mean position of the voxel's points.
func (v Voxel) Centroid() geom.Vec3 {
	n := float64(v.N) * 1e6
	return geom.Vec3{float64(v.SumUM[0]) / n, float64(v.SumUM[1]) / n, float64(v.SumUM[2]) / n}
}

func toUM(x float64) int64 { return int64(math.Round(x * 1e6)) }

// Grid is a sparse voxel grid.
type Grid struct {
	Size  float64
	index map[Key]int
	Keys  []Key
	Cells []Voxel
}

func NewGrid(size float64) *Grid { return &Grid{Size: size, index: map[Key]int{}} }

// KeyOf returns the voxel containing p.
func (g *Grid) KeyOf(p geom.Vec3) Key {
	return packKey(floorDiv(p[0], g.Size), floorDiv(p[1], g.Size), floorDiv(p[2], g.Size))
}

func floorDiv(x, s float64) int {
	q := x / s
	i := int(q)
	if q < 0 && float64(i) != q {
		i--
	}
	return i
}

// frameCell is one voxel's contribution from a single frame.
type frameCell struct {
	key Key
	sum [3]int64 // micrometres
	n   int32
}

// aggregate groups one frame's points by voxel. Each returned cell counts as
// one frame observation when merged.
func (g *Grid) aggregate(pts []geom.Vec3) []frameCell {
	type kp struct {
		k Key
		i int32
	}
	ks := make([]kp, len(pts))
	for i, p := range pts {
		ks[i] = kp{g.KeyOf(p), int32(i)}
	}
	slices.SortFunc(ks, func(a, b kp) int {
		switch {
		case a.k < b.k:
			return -1
		case a.k > b.k:
			return 1
		}
		return 0
	})
	var out []frameCell
	for i := 0; i < len(ks); {
		c := frameCell{key: ks[i].k}
		for ; i < len(ks) && ks[i].k == c.key; i++ {
			p := pts[ks[i].i]
			c.sum[0] += toUM(p[0])
			c.sum[1] += toUM(p[1])
			c.sum[2] += toUM(p[2])
			c.n++
		}
		out = append(out, c)
	}
	return out
}

func (g *Grid) merge(cells []frameCell) {
	for _, c := range cells {
		i, ok := g.index[c.key]
		if !ok {
			i = len(g.Cells)
			g.index[c.key] = i
			g.Keys = append(g.Keys, c.key)
			g.Cells = append(g.Cells, Voxel{})
		}
		v := &g.Cells[i]
		v.SumUM[0] += c.sum[0]
		v.SumUM[1] += c.sum[1]
		v.SumUM[2] += c.sum[2]
		v.N += c.n
		v.Frames++
	}
}

// Len is the number of occupied voxels.
func (g *Grid) Len() int { return len(g.Cells) }

// Points returns the centroids of voxels observed by at least minFrames
// distinct frames, in voxel key order (independent of merge order).
func (g *Grid) Points(minFrames int32) []geom.Vec3 {
	idx := make([]int, len(g.Cells))
	for i := range idx {
		idx[i] = i
	}
	slices.SortFunc(idx, func(a, b int) int { return cmp.Compare(g.Keys[a], g.Keys[b]) })
	var out []geom.Vec3
	for _, i := range idx {
		if v := g.Cells[i]; v.Frames >= minFrames {
			out = append(out, v.Centroid())
		}
	}
	return out
}

// DepthSource yields frames with their depth for fusion.
type DepthSource interface {
	NumFrames() int
	FrameAt(i int) frame.Frame
	LoadDepth(i int) (*frame.DepthMap, *frame.ConfMap, error)
}

// FuseOptions controls depth fusion.
type FuseOptions struct {
	VoxelSize  float64    // metres
	MinConf    uint8      // minimum sensor confidence (ARKit 0..2)
	MinDepth   float64    // metres
	MaxDepth   float64    // metres
	Stride     int        // use every Stride-th frame
	Convention Convention // camera axis convention
	Workers    int        // 0 = GOMAXPROCS
}

// DefaultFuseOptions are the settings from SYSTEM_DESIGN.md §4.2.
func DefaultFuseOptions() FuseOptions {
	return FuseOptions{VoxelSize: 0.02, MinConf: 2, MinDepth: 0.1, MaxDepth: 5.0, Stride: 1, Convention: OpenCV}
}

// Fuse back-projects every Stride-th frame of src into a voxel grid.
func Fuse(src DepthSource, opt FuseOptions) (*Grid, error) {
	if opt.Stride < 1 {
		opt.Stride = 1
	}
	if opt.Workers < 1 {
		opt.Workers = runtime.GOMAXPROCS(0)
	}
	g := NewGrid(opt.VoxelSize)

	jobs := make(chan int)
	results := make(chan []frameCell, opt.Workers)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	for w := 0; w < opt.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var buf []geom.Vec3
			for i := range jobs {
				f := src.FrameAt(i)
				if f.Pose == nil {
					continue
				}
				d, c, err := src.LoadDepth(i)
				if err != nil {
					select {
					case errs <- fmt.Errorf("frame %d: %w", f.Index, err):
					default:
					}
					continue
				}
				buf = BackProject(buf[:0], d, c, f.K, *f.Pose, opt.MinConf, opt.MinDepth, opt.MaxDepth, opt.Convention)
				results <- g.aggregate(buf)
			}
		}()
	}
	go func() {
		for i := 0; i < src.NumFrames(); i += opt.Stride {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	for cells := range results {
		g.merge(cells)
	}
	select {
	case err := <-errs:
		return nil, err
	default:
	}
	return g, nil
}
