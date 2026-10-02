package pipeline

import (
	"testing"

	"roomscan/internal/damage"
	"roomscan/internal/geometry"
	"roomscan/internal/output"
)

// Damage, flags and scope written into a plan must satisfy the published
// schema (the samples have no damage, so real runs never exercise this).
func TestDamageOutputValidates(t *testing.T) {
	shapes := []*geometry.RoomShape{{Corners: []geometry.Pt{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 3}, {X: 0, Y: 3}}}}
	regions := []damage.Region{
		{Class: "mould", Surface: damage.Surface{Room: 0, Wall: 1}, U0: 1, U1: 1.4, V0: 0.1, V1: 0.5, Score: 0.6, Views: 3},
		{Class: "crack", Surface: damage.Surface{Room: 0, Wall: 2}, U0: 1, U1: 1.05, V0: 1.5, V1: 2.2, Score: 0.5, Views: 2},
		{Class: "hole", Surface: damage.Surface{Room: 0, Wall: damage.SurfCeiling}, U0: 1, U1: 1.1, V0: 1, V1: 1.1, Score: 0.4, Views: 2},
	}
	flags := damage.Rules(regions, nil)
	p := output.NewPlan("t", "lidar")
	p.Provenance = output.Provenance{PipelineVersion: "test", Models: []output.Model{}}
	addDamage(p, shapes, regions, flags)
	if len(p.Damage) != 3 || len(p.Scope) != 3 || len(p.ConcealedFlags) != 1 {
		t.Fatalf("damage %d scope %d flags %d, want 3/3/1", len(p.Damage), len(p.Scope), len(p.ConcealedFlags))
	}
	if p.Scope[1].Unit != "m" || p.Scope[1].Quantity.Value < 0.6 {
		t.Errorf("crack scope %+v, want a length in metres", p.Scope[1])
	}
	if err := output.ValidateValue(p); err != nil {
		t.Fatal(err)
	}
}
