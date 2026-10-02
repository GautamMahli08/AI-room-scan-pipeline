package bench

import (
	"roomscan/internal/calib"
	"roomscan/internal/output"
)

// CalibrationPairs extracts, from a comparison of two uncalibrated plans,
// the length pairs (walls measured in both) and area pairs (rooms
// overlapping by IoU >= 0.5) with their model sigmas.
func CalibrationPairs(r *Result, a, b *output.Plan) (lengths, areas []calib.Pair) {
	wall := func(p *output.Plan, room, id string) *output.Wall {
		for i := range p.Rooms {
			if p.Rooms[i].ID != room {
				continue
			}
			for j := range p.Rooms[i].Walls {
				if p.Rooms[i].Walls[j].ID == id {
					return &p.Rooms[i].Walls[j]
				}
			}
		}
		return nil
	}
	room := func(p *output.Plan, id string) *output.Room {
		for i := range p.Rooms {
			if p.Rooms[i].ID == id {
				return &p.Rooms[i]
			}
		}
		return nil
	}
	sig := func(m output.Measurement) float64 { return (m.CI90[1] - m.CI90[0]) / 2 / 1.6449 }
	for _, m := range r.Rooms {
		for _, w := range m.Walls {
			if !w.Measured {
				continue
			}
			wa, wb := wall(a, m.A, w.A), wall(b, m.B, w.B)
			if wa == nil || wb == nil {
				continue
			}
			lengths = append(lengths, calib.Pair{Diff: w.Delta, SigA: sig(wa.LengthM), SigB: sig(wb.LengthM)})
		}
		if m.IoU >= 0.5 {
			ra, rb := room(a, m.A), room(b, m.B)
			mean := (ra.FloorAreaM2.Value + rb.FloorAreaM2.Value) / 2
			areas = append(areas, calib.Pair{Diff: (rb.FloorAreaM2.Value - ra.FloorAreaM2.Value) / mean,
				SigA: sig(ra.FloorAreaM2) / mean, SigB: sig(rb.FloorAreaM2) / mean})
		}
	}
	return lengths, areas
}
