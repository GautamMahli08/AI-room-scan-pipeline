// Package frame defines the common internal representation every input tier
// is normalised to. Downstream stages never branch on tier; they read which
// fields are present.
package frame

import "roomscan/internal/geom"

// Tier is the input tier a frame came from.
type Tier int

const (
	Photo Tier = iota
	Video
	LiDAR
)

func (t Tier) String() string {
	switch t {
	case Photo:
		return "photo"
	case Video:
		return "video"
	case LiDAR:
		return "lidar"
	}
	return "unknown"
}

// Intrinsics is a pinhole camera model at a given image resolution.
type Intrinsics struct {
	Fx, Fy, Cx, Cy float64
	Width, Height  int
}

// Rescale returns the intrinsics for the same camera at a different
// resolution, treating pixel centres at +0.5 so the optical centre maps
// exactly between resolutions.
func (k Intrinsics) Rescale(width, height int) Intrinsics {
	sx := float64(width) / float64(k.Width)
	sy := float64(height) / float64(k.Height)
	return Intrinsics{
		Fx: k.Fx * sx, Fy: k.Fy * sy,
		Cx: (k.Cx+0.5)*sx - 0.5, Cy: (k.Cy+0.5)*sy - 0.5,
		Width: width, Height: height,
	}
}

// Pose is a camera-to-world rigid transform: p_world = R·p_cam + T.
type Pose struct {
	R geom.Mat3
	T geom.Vec3
}

func (p Pose) Apply(v geom.Vec3) geom.Vec3 { return p.R.MulVec(v).Add(p.T) }

// DepthMap is metric depth along the camera z axis, row-major, in metres.
// Zero means no measurement.
type DepthMap struct {
	Width, Height int
	Z             []float32
}

func (d *DepthMap) At(u, v int) float32 { return d.Z[v*d.Width+u] }

// ConfMap is per-pixel sensor confidence (ARKit: 0 low, 1 medium, 2 high).
type ConfMap struct {
	Width, Height int
	C             []uint8
}

func (c *ConfMap) At(u, v int) uint8 { return c.C[v*c.Width+u] }

// Frame is one view of the scene. Depth and Confidence are filled lazily by
// the ingest package (a 10k-frame capture does not fit in memory at once);
// they stay nil for photo/video until depth is estimated.
type Frame struct {
	Index      int
	Timestamp  float64
	RGBPath    string    // video path for video-derived frames, image path for photos
	Depth      *DepthMap // nil until loaded or estimated
	Confidence *ConfMap  // nil if the sensor doesn't provide it
	Pose       *Pose     // camera-to-world; nil for unposed photos
	K          Intrinsics
	Source     Tier
}
