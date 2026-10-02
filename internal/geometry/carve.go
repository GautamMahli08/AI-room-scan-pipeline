package geometry

import (
	"math"
	"runtime"
	"sync"

	"roomscan/internal/geom"
	"roomscan/internal/geometry/pointcloud"
)

// Carve marks plan cells that depth rays passed through: a ray from the
// camera to a measured point proves the space in between was empty. This
// recovers free floor the camera never looked at directly, such as the
// floor under the user's feet. It returns the number of rays per cell.
//
// Every stride-th frame is used and every pixStep-th pixel in each
// direction; the last carveStop metres before each endpoint are left alone
// so depth noise does not eat into walls.
func Carve(src pointcloud.DepthSource, r *Rasters, stride, pixStep int) []uint16 {
	const carveStop = 0.08
	workers := runtime.GOMAXPROCS(0)
	jobs := make(chan int)
	partial := make([][]uint16, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		partial[w] = make([]uint16, r.W*r.H)
		wg.Add(1)
		go func(acc []uint16) {
			defer wg.Done()
			for i := range jobs {
				f := src.FrameAt(i)
				if f.Pose == nil {
					continue
				}
				d, c, err := src.LoadDepth(i)
				if err != nil {
					continue
				}
				ox, oy, _ := r.Frame.ToPlan(f.Pose.T)
				for v := 0; v < d.Height; v += pixStep {
					for u := 0; u < d.Width; u += pixStep {
						z := float64(d.At(u, v))
						if (c != nil && c.At(u, v) < 2) || z < 0.1 || z > 5 {
							continue
						}
						p := f.Pose.Apply(geom.Vec3{(float64(u) - f.K.Cx) * z / f.K.Fx, (float64(v) - f.K.Cy) * z / f.K.Fy, z})
						px, py, h := r.Frame.ToPlan(p)
						if h < -0.1 || h > wallBandHi {
							continue
						}
						carveRay(r, acc, ox, oy, px, py, carveStop)
					}
				}
			}
		}(partial[w])
	}
	for i := 0; i < src.NumFrames(); i += stride {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	out := partial[0]
	for _, p := range partial[1:] {
		for i, n := range p {
			out[i] = uint16(min(int(out[i])+int(n), math.MaxUint16))
		}
	}
	return out
}

// carveRay increments every cell along the segment (ox,oy)→(px,py),
// stopping stop metres short of the endpoint. Cells are sampled at half
// cell spacing; each cell is counted once per ray.
func carveRay(r *Rasters, acc []uint16, ox, oy, px, py, stop float64) {
	dx, dy := px-ox, py-oy
	l := math.Hypot(dx, dy)
	if l <= stop {
		return
	}
	n := int((l - stop) / (r.Res / 2))
	last := -1
	for k := 0; k <= n; k++ {
		t := float64(k) * (r.Res / 2) / l
		cx, cy, ok := r.Cell(ox+t*dx, oy+t*dy)
		if !ok {
			continue
		}
		i := r.Idx(cx, cy)
		if i != last && acc[i] < math.MaxUint16 {
			acc[i]++
			last = i
		}
	}
}
