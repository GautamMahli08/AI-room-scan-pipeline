// Package damage places 2D damage detections onto the plan's surfaces,
// merges them across views, and applies the concealed-damage rules
// (SYSTEM_DESIGN.md §7).
package damage

import (
	"math"
	"sort"

	"roomscan/internal/frame"
	"roomscan/internal/geom"
	"roomscan/internal/geometry"
)

// Detection is one detector box on one frame (original pixel coordinates
// of the RGB video).
type Detection struct {
	Frame int        `json:"frame"`
	Class string     `json:"class"`
	Score float64    `json:"score"`
	Box   [4]float64 `json:"box"`
}

// Surface identifies where a region lies: a wall of a room, or its floor
// or ceiling (Wall = -1 / -2).
type Surface struct {
	Room, Wall int
}

const (
	SurfFloor   = -1
	SurfCeiling = -2
)

// Region is a merged damage region on one surface. On a wall, U runs
// along the wall from its start and V is height above the floor; on the
// floor or ceiling, U and V are plan x and y.
type Region struct {
	Class          string
	Surface        Surface
	U0, U1, V0, V1 float64
	Score          float64
	Views          int
	frames         map[int]bool
}

func (r Region) Width() float64  { return r.U1 - r.U0 }
func (r Region) Height() float64 { return r.V1 - r.V0 }
func (r Region) Area() float64   { return r.Width() * r.Height() }

// Thresholds: precision over recall, so that an undamaged room reports
// nothing rather than confident false damage.
const (
	MinScore     = 0.35 // detector score
	MinViews     = 2    // distinct frames seeing the same region
	wallDist     = 0.25 // projected points within this of a wall face
	mergeSlack   = 0.10 // regions closer than this on a surface merge
	maxArea      = 4.0  // m²; larger "damage" is a whole surface misfire
	minPixelHits = 20   // depth pixels inside a box needed to place it
)

// Source provides frames and depth for projection.
type Source interface {
	FrameAt(i int) frame.Frame
	LoadDepth(i int) (*frame.DepthMap, *frame.ConfMap, error)
}

// Place projects detections onto surfaces and merges them. frameIndex
// maps a video frame number to the capture's frame position; rgbW, rgbH
// are the video size (boxes are in those pixels).
func Place(dets []Detection, src Source, frameIndex map[int]int, rgbW, rgbH int, pf geometry.PlanFrame,
	shapes []*geometry.RoomShape, ceilings []geometry.Ceiling) []Region {
	var regions []Region
	for _, d := range dets {
		if d.Score < MinScore {
			continue
		}
		pos, ok := frameIndex[d.Frame]
		if !ok {
			continue
		}
		r, ok := project(d, src, pos, rgbW, rgbH, pf, shapes, ceilings)
		if !ok || r.Area() > maxArea {
			continue
		}
		regions = mergeInto(regions, r)
	}
	var out []Region
	for _, r := range regions {
		r.Views = len(r.frames)
		if r.Views >= MinViews {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// project back-projects the depth pixels inside the central 80% of the
// box, finds the room containing them and the surface they lie on, and
// returns the 10-90 % extent on that surface.
func project(d Detection, src Source, pos, rgbW, rgbH int, pf geometry.PlanFrame,
	shapes []*geometry.RoomShape, ceilings []geometry.Ceiling) (Region, bool) {
	f := src.FrameAt(pos)
	dm, conf, err := src.LoadDepth(pos)
	if err != nil || f.Pose == nil {
		return Region{}, false
	}
	sx, sy := float64(dm.Width)/float64(rgbW), float64(dm.Height)/float64(rgbH)
	bw, bh := d.Box[2]-d.Box[0], d.Box[3]-d.Box[1]
	u0, u1 := int((d.Box[0]+0.1*bw)*sx), int((d.Box[2]-0.1*bw)*sx)
	v0, v1 := int((d.Box[1]+0.1*bh)*sy), int((d.Box[3]-0.1*bh)*sy)
	type pt struct{ x, y, h float64 }
	var pts []pt
	for v := max(v0, 0); v <= min(v1, dm.Height-1); v++ {
		for u := max(u0, 0); u <= min(u1, dm.Width-1); u++ {
			z := float64(dm.At(u, v))
			if z < 0.1 || z > 5 || (conf != nil && conf.At(u, v) < 1) {
				continue
			}
			p := f.Pose.Apply(geom.Vec3{(float64(u) - f.K.Cx) * z / f.K.Fx, (float64(v) - f.K.Cy) * z / f.K.Fy, z})
			x, y, h := pf.ToPlan(p)
			pts = append(pts, pt{x, y, h})
		}
	}
	if len(pts) < minPixelHits {
		return Region{}, false
	}
	xs, ys, hs := make([]float64, len(pts)), make([]float64, len(pts)), make([]float64, len(pts))
	for i, p := range pts {
		xs[i], ys[i], hs[i] = p.x, p.y, p.h
	}
	cx, cy, ch := median(xs), median(ys), median(hs)
	// Damage on a wall lies on the wall face, i.e. exactly on the room
	// boundary; look the room up from a point stepped 15 cm back towards
	// the camera, which is on the room's side.
	camX, camY, _ := pf.ToPlan(f.Pose.T)
	look := geometry.Pt{X: cx, Y: cy}
	if back := (geometry.Pt{X: camX - cx, Y: camY - cy}); back.Norm() > 1e-6 {
		look = look.Add(back.Scale(math.Min(0.15, back.Norm()) / back.Norm()))
	}
	room := -1
	for ri, s := range shapes {
		if s != nil && geometry.PointInPolygon(look, s.Corners) {
			room = ri
			break
		}
	}
	if room < 0 {
		return Region{}, false
	}
	r := Region{Class: d.Class, Score: d.Score, frames: map[int]bool{d.Frame: true}}
	ceil := 2.2
	if c := ceilings[room]; c.Observed {
		ceil = c.Height
	}
	switch {
	case ch < 0.05:
		r.Surface = Surface{room, SurfFloor}
		r.U0, r.U1, r.V0, r.V1 = q(xs, 0.1), q(xs, 0.9), q(ys, 0.1), q(ys, 0.9)
		return r, true
	case ch > ceil-0.15:
		r.Surface = Surface{room, SurfCeiling}
		r.U0, r.U1, r.V0, r.V1 = q(xs, 0.1), q(xs, 0.9), q(ys, 0.1), q(ys, 0.9)
		return r, true
	}
	s := shapes[room]
	best, bestD := -1, wallDist
	for wi, w := range s.Walls {
		if w.Inferred {
			continue
		}
		dd := math.Abs(w.Line.N.Dot(geometry.Pt{X: cx, Y: cy}) - w.Line.C)
		if dd < bestD {
			best, bestD = wi, dd
		}
	}
	if best < 0 {
		return Region{}, false // on furniture, not a surface
	}
	a, b := s.Corners[best], s.Corners[(best+1)%len(s.Corners)]
	u := b.Sub(a).Scale(1 / b.Sub(a).Norm())
	ts := make([]float64, len(pts))
	for i, p := range pts {
		ts[i] = geometry.Pt{X: p.x, Y: p.y}.Sub(a).Dot(u)
	}
	r.Surface = Surface{room, best}
	r.U0, r.U1, r.V0, r.V1 = q(ts, 0.1), q(ts, 0.9), q(hs, 0.1), q(hs, 0.9)
	return r, true
}

// mergeInto adds r to the region list, merging it with every region of
// the same class on the same surface that it overlaps (with slack).
func mergeInto(rs []Region, r Region) []Region {
	for i := range rs {
		o := &rs[i]
		if o.Class != r.Class || o.Surface != r.Surface {
			continue
		}
		if r.U0 > o.U1+mergeSlack || r.U1 < o.U0-mergeSlack || r.V0 > o.V1+mergeSlack || r.V1 < o.V0-mergeSlack {
			continue
		}
		o.U0, o.U1 = math.Min(o.U0, r.U0), math.Max(o.U1, r.U1)
		o.V0, o.V1 = math.Min(o.V0, r.V0), math.Max(o.V1, r.V1)
		o.Score = math.Max(o.Score, r.Score)
		for k := range r.frames {
			o.frames[k] = true
		}
		return rs
	}
	return append(rs, r)
}

func median(v []float64) float64 { return q(v, 0.5) }

func q(v []float64, p float64) float64 {
	s := append([]float64{}, v...)
	sort.Float64s(s)
	return s[int(p*float64(len(s)-1))]
}
