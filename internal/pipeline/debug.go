package pipeline

import (
	"image/color"

	"roomscan/internal/geometry"
)

var palette = []color.RGBA{{166, 206, 227, 255}, {178, 223, 138, 255}, {251, 154, 153, 255}, {253, 191, 111, 255},
	{202, 178, 214, 255}, {255, 255, 153, 255}, {141, 211, 199, 255}, {252, 205, 229, 255}, {204, 235, 197, 255}}

// writeDebug renders the plan rasters: observed floor/furniture, rooms,
// wall cells by orientation, gaps (magenta with header evidence, orange
// without) and the camera trajectory.
func writeDebug(path string, r *geometry.Rasters, wall []bool, orient []geometry.Orientation, gaps []geometry.Gap,
	labels []int32, traj [][2]float64, shapes []*geometry.RoomShape) error {
	n := r.W * r.H
	mask := func(f func(i int) bool) []bool {
		m := make([]bool, n)
		for i := range m {
			m[i] = f(i)
		}
		return m
	}
	occ := r.OccupiedMask()
	layers := []geometry.DebugLayer{
		{Mask: mask(func(i int) bool { return r.Floor[i] > 0 }), Color: color.RGBA{235, 235, 235, 255}},
		{Mask: occ, Color: color.RGBA{215, 215, 215, 255}},
	}
	var maxL int32
	for _, l := range labels {
		maxL = max(maxL, l)
	}
	for l := int32(1); l <= maxL; l++ {
		layers = append(layers, geometry.DebugLayer{Mask: mask(func(i int) bool { return labels[i] == l }), Color: palette[int(l-1)%len(palette)]})
	}
	layers = append(layers,
		geometry.DebugLayer{Mask: wall, Color: color.RGBA{60, 60, 60, 255}},
		geometry.DebugLayer{Mask: mask(func(i int) bool { return orient[i]&geometry.OrientH != 0 }), Color: color.RGBA{0, 120, 0, 255}},
		geometry.DebugLayer{Mask: mask(func(i int) bool { return orient[i]&geometry.OrientV != 0 }), Color: color.RGBA{0, 60, 200, 255}},
		geometry.DebugLayer{Mask: mask(func(i int) bool { return orient[i]&geometry.OrientOther != 0 }), Color: color.RGBA{220, 140, 0, 255}},
		geometry.DebugLayer{Mask: gapMask(r, gaps, true), Color: color.RGBA{220, 0, 220, 255}},
		geometry.DebugLayer{Mask: gapMask(r, gaps, false), Color: color.RGBA{255, 170, 0, 255}},
	)
	var pts []geometry.DebugPoint
	for _, p := range traj {
		pts = append(pts, geometry.DebugPoint{X: p[0], Y: p[1], Color: color.RGBA{230, 0, 0, 255}})
	}
	// Fitted outlines: observed walls black, inferred walls grey.
	for _, s := range shapes {
		if s == nil {
			continue
		}
		for i, w := range s.Walls {
			a, b := s.Corners[i], s.Corners[(i+1)%len(s.Corners)]
			c := color.RGBA{0, 0, 0, 255}
			if w.Inferred {
				c = color.RGBA{150, 150, 150, 255}
			}
			n := int(b.Sub(a).Norm()/(r.Res/2)) + 1
			for k := 0; k <= n; k++ {
				p := a.Add(b.Sub(a).Scale(float64(k) / float64(n)))
				pts = append(pts, geometry.DebugPoint{X: p.X, Y: p.Y, Color: c})
			}
		}
	}
	return geometry.WriteDebugPNG(path, r.Grid2, layers, pts, 2)
}

// gapMask marks the cells of opening-sized gaps (>= 0.5 m) with or without
// header evidence.
func gapMask(r *geometry.Rasters, gaps []geometry.Gap, header bool) []bool {
	m := make([]bool, r.W*r.H)
	for _, g := range gaps {
		if g.Width(r.Res) < 0.5 || (g.HeaderFill >= 0.5) != header {
			continue
		}
		geometry.ForGapCells(r, g, 0, func(i int) { m[i] = true })
	}
	return m
}
