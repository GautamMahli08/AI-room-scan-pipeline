package output

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// WriteSVG renders the plan as a dimensioned floor plan. Plan +y is drawn
// up. Observed walls are solid, inferred walls dashed; doors and openings
// are breaks in the wall, windows a thin double line.
func WriteSVG(path string, p *Plan) error {
	const scale = 100.0 // px per metre
	const margin = 60.0
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, r := range p.Rooms {
		for _, v := range r.Polygon {
			minX, minY = math.Min(minX, v[0]), math.Min(minY, v[1])
			maxX, maxY = math.Max(maxX, v[0]), math.Max(maxY, v[1])
		}
	}
	if math.IsInf(minX, 0) {
		minX, minY, maxX, maxY = 0, 0, 1, 1
	}
	W := (maxX-minX)*scale + 2*margin
	H := (maxY-minY)*scale + 2*margin + 40
	tx := func(x float64) float64 { return (x-minX)*scale + margin }
	ty := func(y float64) float64 { return (maxY-y)*scale + margin + 40 }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="Helvetica, Arial, sans-serif">`+"\n", W, H, W, H)
	b.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/>` + "\n")
	fmt.Fprintf(&b, `<text x="%.0f" y="28" font-size="18" fill="#222">%s — %s tier — footprint %.1f m² [%.1f, %.1f]</text>`+"\n",
		margin, esc(p.CaptureID), p.Tier, p.FootprintAreaM2.Value, p.FootprintAreaM2.CI90[0], p.FootprintAreaM2.CI90[1])

	fills := []string{"#eaf2fb", "#eef7e8", "#fdf0e6", "#f3edf8", "#fdf8e1", "#e8f6f4", "#fbecef", "#f0f0f0"}
	for k, r := range p.Rooms {
		var pts []string
		for _, v := range r.Polygon {
			pts = append(pts, fmt.Sprintf("%.1f,%.1f", tx(v[0]), ty(v[1])))
		}
		fmt.Fprintf(&b, `<polygon points="%s" fill="%s" stroke="none"/>`+"\n", strings.Join(pts, " "), fills[k%len(fills)])
	}

	for _, r := range p.Rooms {
		cuts := map[string][][2]float64{} // wall id -> opening [start,end] along wall
		for _, o := range r.Openings {
			if o.OffsetM != nil {
				cuts[o.Wall] = append(cuts[o.Wall], [2]float64{o.OffsetM.Value, o.OffsetM.Value + o.WidthM.Value})
			}
		}
		for _, w := range r.Walls {
			dx, dy := w.End[0]-w.Start[0], w.End[1]-w.Start[1]
			l := math.Hypot(dx, dy)
			if l == 0 {
				continue
			}
			ux, uy := dx/l, dy/l
			at := func(t float64) (float64, float64) { return tx(w.Start[0] + ux*t), ty(w.Start[1] + uy*t) }
			style := `stroke="#222" stroke-width="5" stroke-linecap="square"`
			if w.Inferred {
				style = `stroke="#999" stroke-width="2" stroke-dasharray="8 6"`
			}
			// Draw the wall minus its openings.
			t := 0.0
			for _, c := range sortedCuts(cuts[w.ID]) {
				if c[0] > t {
					x0, y0 := at(t)
					x1, y1 := at(c[0])
					fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" %s/>`+"\n", x0, y0, x1, y1, style)
				}
				t = math.Max(t, c[1])
			}
			if t < l {
				x0, y0 := at(t)
				x1, y1 := at(l)
				fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" %s/>`+"\n", x0, y0, x1, y1, style)
			}
			// Dimension label, inside the room (left of travel for a CCW polygon).
			if l >= 0.4 {
				mx, my := w.Start[0]+dx/2-uy*0.22, w.Start[1]+dy/2+ux*0.22
				rot := -math.Atan2(dy, dx) * 180 / math.Pi
				if rot > 90 || rot < -90 {
					rot += 180
				}
				fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="11" fill="%s" text-anchor="middle" dominant-baseline="middle" transform="rotate(%.1f %.1f %.1f)">%.2f m ±%.0f cm</text>`+"\n",
					tx(mx), ty(my), map[bool]string{true: "#999", false: "#333"}[w.Inferred], rot, tx(mx), ty(my),
					w.LengthM.Value, (w.LengthM.CI90[1]-w.LengthM.CI90[0])/2*100)
			}
		}
		for _, o := range r.Openings {
			if o.OffsetM == nil {
				continue
			}
			w := wallByID(r.Walls, o.Wall)
			if w == nil {
				continue
			}
			dx, dy := w.End[0]-w.Start[0], w.End[1]-w.Start[1]
			l := math.Hypot(dx, dy)
			ux, uy := dx/l, dy/l
			x0, y0 := tx(w.Start[0]+ux*o.OffsetM.Value), ty(w.Start[1]+uy*o.OffsetM.Value)
			x1, y1 := tx(w.Start[0]+ux*(o.OffsetM.Value+o.WidthM.Value)), ty(w.Start[1]+uy*(o.OffsetM.Value+o.WidthM.Value))
			switch o.Type {
			case "window":
				fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#3a7bd5" stroke-width="5"/>`+"\n", x0, y0, x1, y1)
			case "door":
				// Quarter-circle swing into the room.
				nx, ny := -uy, ux
				r := o.WidthM.Value * scale
				hx, hy := x0+nx*r, y0-ny*r
				fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f A %.1f %.1f 0 0 %d %.1f %.1f" fill="none" stroke="#c0392b" stroke-width="1.2"/>`+"\n",
					x0, y0, hx, hy, r, r, 1, x1, y1)
			}
			mx, my := (x0+x1)/2, (y0+y1)/2
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="10" fill="#c0392b" text-anchor="middle">%s %.2f</text>`+"\n", mx, my-8, o.Type, o.WidthM.Value)
		}
	}

	for _, r := range p.Rooms {
		cx, cy := centroid(r.Polygon)
		ceil := "ceiling not observed"
		if r.CeilingHeightM.Observed && r.CeilingHeightM.Value != nil {
			ceil = fmt.Sprintf("ceiling %.2f m", *r.CeilingHeightM.Value)
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="15" font-weight="bold" fill="#222" text-anchor="middle">%s</text>`+"\n", tx(cx), ty(cy)-10, esc(r.ID))
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="12" fill="#444" text-anchor="middle">%.1f m² [%.1f, %.1f]</text>`+"\n", tx(cx), ty(cy)+8, r.FloorAreaM2.Value, r.FloorAreaM2.CI90[0], r.FloorAreaM2.CI90[1])
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="11" fill="#666" text-anchor="middle">%s</text>`+"\n", tx(cx), ty(cy)+24, ceil)
	}

	// Damage: a red ring at the region centre (on a wall: the point along
	// the wall; on floor/ceiling: the plan point), sized by its width,
	// labelled with class, area and any concealed-damage rule.
	flagged := map[string][]string{}
	for _, f := range p.ConcealedFlags {
		for _, id := range f.DamageIDs {
			flagged[id] = append(flagged[id], f.RuleID)
		}
	}
	for _, d := range p.Damage {
		x, y, ok := damagePoint(p, d)
		if !ok {
			continue
		}
		r := math.Max(6, d.WidthM.Value*scale/2)
		label := fmt.Sprintf("%s %.2f m²", strings.ReplaceAll(d.Class, "_", " "), d.AreaM2.Value)
		if d.Location.Surface == "ceiling" || d.Location.Surface == "floor" {
			label += " (" + d.Location.Surface + ")"
		}
		if rules := flagged[d.ID]; len(rules) > 0 {
			label += " ⚠ " + strings.Join(rules, ", ")
		}
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#e74c3c" fill-opacity="0.25" stroke="#c0392b" stroke-width="2"/>`+"\n", tx(x), ty(y), r)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="10" fill="#c0392b" font-weight="bold">%s %s</text>`+"\n", tx(x)+r+3, ty(y)+3, esc(d.ID), esc(label))
	}

	// 1 m scale bar.
	fmt.Fprintf(&b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#222" stroke-width="3"/><text x="%.0f" y="%.0f" font-size="11">1 m</text>`+"\n",
		margin, H-20, margin+scale, H-20, margin+scale+6, H-16)
	b.WriteString("</svg>\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func sortedCuts(c [][2]float64) [][2]float64 {
	out := append([][2]float64{}, c...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j][0] < out[j-1][0]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func wallByID(ws []Wall, id string) *Wall {
	for i := range ws {
		if ws[i].ID == id {
			return &ws[i]
		}
	}
	return nil
}

// centroid is the area centroid of a simple polygon.
func centroid(p [][2]float64) (float64, float64) {
	var a, cx, cy float64
	for i := range p {
		j := (i + 1) % len(p)
		c := p[i][0]*p[j][1] - p[j][0]*p[i][1]
		a += c
		cx += (p[i][0] + p[j][0]) * c
		cy += (p[i][1] + p[j][1]) * c
	}
	if a == 0 {
		return p[0][0], p[0][1]
	}
	return cx / (3 * a), cy / (3 * a)
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// damagePoint returns the plan position of a damage region's centre.
func damagePoint(p *Plan, d Damage) (float64, float64, bool) {
	if d.CentreM == nil {
		return 0, 0, false
	}
	if d.Location.Surface == "floor" || d.Location.Surface == "ceiling" {
		return d.CentreM[0], d.CentreM[1], true
	}
	for _, r := range p.Rooms {
		if r.ID != d.Location.Room {
			continue
		}
		if w := wallByID(r.Walls, d.Location.Surface); w != nil {
			dx, dy := w.End[0]-w.Start[0], w.End[1]-w.Start[1]
			l := math.Hypot(dx, dy)
			if l == 0 {
				return 0, 0, false
			}
			t := d.CentreM[0] / l
			return w.Start[0] + dx*t, w.Start[1] + dy*t, true
		}
	}
	return 0, 0, false
}
