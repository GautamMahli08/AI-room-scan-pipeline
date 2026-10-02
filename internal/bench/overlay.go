package bench

import (
	"fmt"
	"math"
	"os"
	"strings"

	"roomscan/internal/geometry"
	"roomscan/internal/output"
)

// WriteOverlay draws plan A (blue) and plan B registered onto A (red) in
// one SVG, with room and wall ids, for inspecting repeatability failures.
func WriteOverlay(path string, a, b *output.Plan, t Transform) error {
	const scale, margin = 120.0, 40.0
	pa, pb := polys(a, Transform{}), polys(b, t)
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, ps := range [][][]geometry.Pt{pa, pb} {
		for _, p := range ps {
			for _, v := range p {
				minX, minY = math.Min(minX, v.X), math.Min(minY, v.Y)
				maxX, maxY = math.Max(maxX, v.X), math.Max(maxY, v.Y)
			}
		}
	}
	W, H := (maxX-minX)*scale+2*margin, (maxY-minY)*scale+2*margin
	tx := func(x float64) float64 { return (x-minX)*scale + margin }
	ty := func(y float64) float64 { return (maxY-y)*scale + margin }

	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="Helvetica, Arial, sans-serif">`+"\n", W, H)
	s.WriteString(`<rect width="100%" height="100%" fill="#fff"/>` + "\n")
	draw := func(p *output.Plan, ps [][]geometry.Pt, ws []wallLine, color string, dy float64) {
		for i, poly := range ps {
			var pts []string
			for _, v := range poly {
				pts = append(pts, fmt.Sprintf("%.1f,%.1f", tx(v.X), ty(v.Y)))
			}
			fmt.Fprintf(&s, `<polygon points="%s" fill="%s" fill-opacity="0.06" stroke="%s" stroke-width="2"/>`+"\n", strings.Join(pts, " "), color, color)
			c := centroid(poly)
			fmt.Fprintf(&s, `<text x="%.1f" y="%.1f" font-size="16" fill="%s" text-anchor="middle">%s</text>`+"\n", tx(c.X), ty(c.Y)+dy, color, p.Rooms[i].ID)
		}
		for _, w := range ws {
			dash := ""
			if !w.measured {
				dash = ` stroke-dasharray="6 4"`
			}
			fmt.Fprintf(&s, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="3"%s/>`+"\n", tx(w.a.X), ty(w.a.Y), tx(w.b.X), ty(w.b.Y), color, dash)
			m := w.mid.Sub(w.n.Scale(0.12 + dy/100))
			fmt.Fprintf(&s, `<text x="%.1f" y="%.1f" font-size="10" fill="%s" text-anchor="middle">%s %.2f</text>`+"\n", tx(m.X), ty(m.Y), color, w.id, w.length)
		}
	}
	draw(a, pa, walls(a, Transform{}), "#1f5fbf", 0)
	draw(b, pb, walls(b, t), "#c0392b", 18)
	s.WriteString("</svg>\n")
	return os.WriteFile(path, []byte(s.String()), 0o644)
}
