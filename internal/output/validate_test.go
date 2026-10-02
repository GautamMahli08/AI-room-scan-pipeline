package output

import (
	"strings"
	"testing"
)

const validPlan = `{
  "schema_version": "1.0",
  "capture_id": "single_room",
  "tier": "lidar",
  "rooms": [{
    "id": "R1",
    "polygon": [[0,0],[3.42,0],[3.42,3.5],[0,3.5]],
    "walls": [{"id": "W1", "start": [0,0], "end": [3.42,0], "length_m": {"value": 3.42, "ci90": [3.40, 3.44]}, "inferred": false}],
    "openings": [{"id": "O1", "wall": "W1", "type": "door", "width_m": {"value": 0.82, "ci90": [0.80, 0.84]}}],
    "ceiling_height_m": {"observed": false, "value": null, "reason": "no ceiling points captured"},
    "floor_area_m2": {"value": 12.1, "ci90": [11.8, 12.4]}
  }],
  "adjacency": [],
  "footprint_area_m2": {"value": 12.1, "ci90": [11.8, 12.4]},
  "damage": [{"id": "D1", "class": "water_stain", "location": {"room": "R1", "surface": "ceiling"},
    "area_m2": {"value": 0.3, "ci90": [0.2, 0.4]}, "width_m": {"value": 0.6, "ci90": [0.5, 0.7]},
    "height_m": {"value": 0.5, "ci90": [0.4, 0.6]}, "detection_confidence": 0.8}],
  "concealed_flags": [{"id": "F1", "rule_id": "R-WET-CEIL-01", "location": {"room": "R1", "surface": "ceiling"},
    "damage_ids": ["D1"], "message": "Possible leak above ceiling"}],
  "scope": [{"id": "S1", "location": {"room": "R1", "surface": "ceiling"}, "action": "stain block and repaint",
    "quantity": {"value": 0.3, "ci90": [0.2, 0.4]}, "unit": "m2", "damage_ids": ["D1"]}],
  "provenance": {"pipeline_version": "dev", "models": [], "runtime_s": 2.3}
}`

func TestValidPlan(t *testing.T) {
	if err := Validate([]byte(validPlan)); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidPlans(t *testing.T) {
	cases := map[string][2]string{
		"missing interval":          {`"length_m": {"value": 3.42, "ci90": [3.40, 3.44]}`, `"length_m": {"value": 3.42}`},
		"unobserved with value":     {`{"observed": false, "value": null,`, `{"observed": false, "value": 2.4,`},
		"observed without interval": {`{"observed": false, "value": null, "reason": "no ceiling points captured"}`, `{"observed": true, "value": 2.4}`},
		"unknown tier":              {`"tier": "lidar"`, `"tier": "radar"`},
		"unknown damage class":      {`"class": "water_stain"`, `"class": "rust"`},
		"bad rule id":               {`"rule_id": "R-WET-CEIL-01"`, `"rule_id": "wet ceiling"`},
	}
	for name, c := range cases {
		bad := strings.Replace(validPlan, c[0], c[1], 1)
		if bad == validPlan {
			t.Fatalf("%s: replacement did not apply", name)
		}
		if err := Validate([]byte(bad)); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}
