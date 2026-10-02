package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// Plan mirrors schema/plan.schema.json.
type Plan struct {
	SchemaVersion   string          `json:"schema_version"`
	CaptureID       string          `json:"capture_id"`
	Tier            string          `json:"tier"`
	Rooms           []Room          `json:"rooms"`
	Adjacency       []Adjacency     `json:"adjacency"`
	FootprintAreaM2 Measurement     `json:"footprint_area_m2"`
	Damage          []Damage        `json:"damage"`
	ConcealedFlags  []ConcealedFlag `json:"concealed_flags"`
	Scope           []ScopeItem     `json:"scope"`
	Provenance      Provenance      `json:"provenance"`
	Diagnostics     map[string]any  `json:"diagnostics,omitempty"`
}

// Measurement is a value with a 90% interval.
type Measurement struct {
	Value  float64    `json:"value"`
	CI90   [2]float64 `json:"ci90"`
	Source string     `json:"source,omitempty"` // measured | inferred | prior
}

// Observable is a measurement that may not have been observed.
type Observable struct {
	Observed bool         `json:"observed"`
	Value    *float64     `json:"value"`
	CI90     *[2]float64  `json:"ci90,omitempty"`
	Source   string       `json:"source,omitempty"`
	Reason   string       `json:"reason,omitempty"`
	Prior    *Measurement `json:"prior,omitempty"`
}

// Observed returns an observed measurement.
func ObservedValue(m Measurement) Observable {
	v, ci := m.Value, m.CI90
	return Observable{Observed: true, Value: &v, CI90: &ci, Source: m.Source}
}

// Unobserved returns an unobserved measurement with a reason.
func Unobserved(reason string) Observable { return Observable{Reason: reason} }

type Room struct {
	ID             string       `json:"id"`
	Label          string       `json:"label,omitempty"`
	Polygon        [][2]float64 `json:"polygon"`
	Walls          []Wall       `json:"walls"`
	Openings       []Opening    `json:"openings"`
	CeilingHeightM Observable   `json:"ceiling_height_m"`
	FloorAreaM2    Measurement  `json:"floor_area_m2"`
	Notes          []Note       `json:"notes,omitempty"`
}

type Wall struct {
	ID       string      `json:"id"`
	Start    [2]float64  `json:"start"`
	End      [2]float64  `json:"end"`
	LengthM  Measurement `json:"length_m"`
	Inferred bool        `json:"inferred"`
}

type Opening struct {
	ID      string       `json:"id"`
	Wall    string       `json:"wall"`
	Type    string       `json:"type"`
	OffsetM *Measurement `json:"offset_m,omitempty"`
	WidthM  Measurement  `json:"width_m"`
	HeightM *Measurement `json:"height_m,omitempty"`
	SillM   *Measurement `json:"sill_m,omitempty"`
}

type Note struct {
	Code    string `json:"code"`
	Wall    string `json:"wall,omitempty"`
	Message string `json:"message"`
}

type Adjacency struct {
	Rooms [2]string `json:"rooms"`
	Via   []string  `json:"via"`
}

type SurfaceRef struct {
	Room    string `json:"room"`
	Surface string `json:"surface"`
}

type Damage struct {
	ID                  string      `json:"id"`
	Class               string      `json:"class"`
	Location            SurfaceRef  `json:"location"`
	CentreM             *[2]float64 `json:"centre_m,omitempty"`
	AreaM2              Measurement `json:"area_m2"`
	WidthM              Measurement `json:"width_m"`
	HeightM             Measurement `json:"height_m"`
	DetectionConfidence float64     `json:"detection_confidence"`
	Views               int         `json:"views,omitempty"`
}

type ConcealedFlag struct {
	ID        string     `json:"id"`
	RuleID    string     `json:"rule_id"`
	Location  SurfaceRef `json:"location"`
	DamageIDs []string   `json:"damage_ids"`
	Message   string     `json:"message"`
}

type ScopeItem struct {
	ID        string      `json:"id"`
	Location  SurfaceRef  `json:"location"`
	Action    string      `json:"action"`
	Quantity  Measurement `json:"quantity"`
	Unit      string      `json:"unit"`
	DamageIDs []string    `json:"damage_ids,omitempty"`
	FlagIDs   []string    `json:"flag_ids,omitempty"`
}

type Model struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	WeightsSHA256 string `json:"weights_sha256,omitempty"`
	Cached        bool   `json:"cached,omitempty"`
}

type Provenance struct {
	PipelineVersion string  `json:"pipeline_version"`
	Input           string  `json:"input,omitempty"`
	Models          []Model `json:"models"`
	DriftCorrection string  `json:"drift_correction,omitempty"`
	RuntimeS        float64 `json:"runtime_s"`
}

// NewPlan returns a plan with every array initialised (the schema requires
// arrays, not nulls).
func NewPlan(captureID, tier string) *Plan {
	return &Plan{
		SchemaVersion: "1.0", CaptureID: captureID, Tier: tier,
		Rooms: []Room{}, Adjacency: []Adjacency{}, Damage: []Damage{},
		ConcealedFlags: []ConcealedFlag{}, Scope: []ScopeItem{},
		Provenance: Provenance{Models: []Model{}},
	}
}

// WriteJSON validates p against the schema and writes it indented. An
// invalid plan is still written (for debugging) but an error is returned.
func WriteJSON(path string, p *Plan) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	if err := Validate(b); err != nil {
		return fmt.Errorf("%s does not match the schema: %w", path, err)
	}
	return nil
}
