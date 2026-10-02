package damage

import "fmt"

// Flag is a concealed-damage flag raised by a named rule.
type Flag struct {
	RuleID  string
	Region  int // index into the regions
	Message string
}

// Opening is the part of an opening the crack rule needs: which wall it
// is on and where its jambs are.
type Opening struct {
	Surface Surface
	U0, U1  float64 // along the wall
	HeadV   float64 // head height (2.05 m assumed for doors when unknown)
}

// Rules (auditable; each flag names the rule that fired):
//
//	R-WET-CEIL-01   water stain or mould on a ceiling        -> possible leak above the ceiling
//	R-WALL-BASE-01  stain, mould or peeling within 0.3 m of the floor on a wall
//	                                                          -> possible moisture in the wall cavity
//	R-CRACK-DIAG-01 crack within 0.3 m of an opening's head corner on the same wall
//	                                                          -> possible structural movement
func Rules(rs []Region, openings []Opening) []Flag {
	var out []Flag
	for i, r := range rs {
		switch {
		case r.Surface.Wall == SurfCeiling && (r.Class == "water_stain" || r.Class == "mould"):
			out = append(out, Flag{"R-WET-CEIL-01", i, "Water stain or mould on the ceiling: possible leak above the ceiling (roof, pipe or the room above)."})
		case r.Surface.Wall >= 0 && r.V0 <= 0.3 && (r.Class == "water_stain" || r.Class == "mould" || r.Class == "peeling_paint"):
			out = append(out, Flag{"R-WALL-BASE-01", i, fmt.Sprintf("%s at the base of the wall (from %.2f m): possible moisture in the wall cavity (rising or penetrating damp).", label(r.Class), r.V0)})
		case r.Surface.Wall >= 0 && r.Class == "crack":
			for _, o := range openings {
				if o.Surface != r.Surface {
					continue
				}
				nearCorner := (abs(r.U0-o.U0) < 0.3 || abs(r.U1-o.U1) < 0.3 || abs(r.U1-o.U0) < 0.3 || abs(r.U0-o.U1) < 0.3) &&
					abs(r.V0-o.HeadV) < 0.3
				if nearCorner {
					out = append(out, Flag{"R-CRACK-DIAG-01", i, "Crack starting at the corner of an opening: possible structural movement; inspect lintel and foundations."})
					break
				}
			}
		}
	}
	return out
}

// ScopeAction is the repair line for a damage class, and its unit.
func ScopeAction(class string) (action, unit string) {
	switch class {
	case "water_stain":
		return "Stain-block and repaint (after the source is fixed)", "m2"
	case "mould":
		return "Treat mould, stain-block and repaint", "m2"
	case "peeling_paint":
		return "Scrape, prime and repaint", "m2"
	case "crack":
		return "Rake out, fill and repaint crack", "m"
	case "hole":
		return "Patch and repaint", "each"
	}
	return "Inspect", "each"
}

func label(class string) string {
	return map[string]string{"water_stain": "Water stain", "mould": "Mould", "peeling_paint": "Peeling paint",
		"crack": "Crack", "hole": "Hole"}[class]
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
