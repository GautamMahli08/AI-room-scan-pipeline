package pipeline

import (
	"encoding/json"
	"math"
	"os"

	"roomscan/internal/output"
	"roomscan/internal/stitch"
)

// DriftReport is written to drift.json: what the correction did and the
// ablation (stitched footprint with correction on and off).
type DriftReport struct {
	Iterations   []IterSummary      `json:"iterations"`
	Final        IterSummary        `json:"final"`
	FootprintOff output.Measurement `json:"footprint_off_m2"`
	FootprintOn  output.Measurement `json:"footprint_on_m2"`
	RoomsOff     int                `json:"rooms_off"`
	RoomsOn      int                `json:"rooms_on"`
	Chunks       []stitch.Chunk     `json:"chunks_first_iteration"`
}

// IterSummary describes one drift estimate.
type IterSummary struct {
	Observations    int                 `json:"observations"`
	Landmarks       int                 `json:"landmarks"`
	ScatterBeforeMM float64             `json:"scatter_before_mm"`
	ScatterAfterMM  float64             `json:"scatter_predicted_after_mm"`
	MaxShiftCM      float64             `json:"max_shift_cm"`
	MaxYawDeg       float64             `json:"max_yaw_deg"`
	HeldOutNoneMM   float64             `json:"held_out_no_correction_mm"`
	HeldOutBestMM   float64             `json:"held_out_selected_mm"`
	PriorScale      float64             `json:"prior_scale"`
	Selection       []stitch.ScaleScore `json:"prior_selection"`
}

func iterSummary(c *stitch.Correction) IterSummary {
	return IterSummary{
		Observations: c.Observations, Landmarks: c.Landmarks,
		ScatterBeforeMM: round(c.ScatterBefore*1000, 2), ScatterAfterMM: round(c.ScatterAfter*1000, 2),
		MaxShiftCM: round(c.MaxShift()*100, 2), MaxYawDeg: round(c.MaxYaw()*180/math.Pi, 3),
		HeldOutNoneMM: round(c.HeldOutNone*1000, 2), HeldOutBestMM: round(c.HeldOutBest*1000, 2),
		PriorScale: c.PriorScale, Selection: c.Selection,
	}
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
