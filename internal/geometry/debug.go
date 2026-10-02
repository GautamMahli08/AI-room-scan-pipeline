package geometry

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// DebugLayer paints cells where Mask is true. Later layers draw on top.
type DebugLayer struct {
	Mask  []bool
	Color color.RGBA
}

// DebugPoint is a plan-space marker (e.g. the camera trajectory).
type DebugPoint struct {
	X, Y  float64
	Color color.RGBA
}

// WriteDebugPNG renders a raster with y up, one pixel per cell scaled by
// scale.
func WriteDebugPNG(path string, g Grid2, layers []DebugLayer, pts []DebugPoint, scale int) error {
	img := image.NewRGBA(image.Rect(0, 0, g.W*scale, g.H*scale))
	set := func(cx, cy int, c color.RGBA) {
		for dy := 0; dy < scale; dy++ {
			for dx := 0; dx < scale; dx++ {
				img.SetRGBA(cx*scale+dx, (g.H-1-cy)*scale+dy, c)
			}
		}
	}
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for _, l := range layers {
		for cy := 0; cy < g.H; cy++ {
			for cx := 0; cx < g.W; cx++ {
				if l.Mask[g.Idx(cx, cy)] {
					set(cx, cy, l.Color)
				}
			}
		}
	}
	for _, p := range pts {
		if cx, cy, ok := g.Cell(p.X, p.Y); ok {
			set(cx, cy, p.Color)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
