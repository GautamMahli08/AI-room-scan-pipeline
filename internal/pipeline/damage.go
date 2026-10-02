package pipeline

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"roomscan/internal/damage"
	"roomscan/internal/geometry"
	"roomscan/internal/ingest/strayscanner"
	"roomscan/internal/output"
)

// damageEvery is the frame spacing of damage detection (~2 s at 46 fps).
const damageEvery = 90

// detectDamage runs the detector on every damageEvery-th frame of the
// capture's video (cached), then places, merges and rules the detections.
// It returns nothing, with a logged reason, when the ML environment or the
// video is missing: damage is optional output, the plan is not.
func detectDamage(c *strayscanner.Capture, g *scene, outDir string, live bool, logf func(string, ...any)) ([]damage.Region, []damage.Flag) {
	video := filepath.Join(c.Dir, "rgb.mp4")
	if _, err := os.Stat(video); err != nil {
		logf("damage: no rgb.mp4 in the capture; skipped")
		return nil, nil
	}
	py, root, err := python()
	if err != nil {
		logf("damage: %v; skipped", err)
		return nil, nil
	}
	var idx []string
	frameIndex := map[int]int{}
	for i := 0; i < len(c.Frames); i += damageEvery {
		idx = append(idx, strconv.Itoa(c.Frames[i].Index))
		frameIndex[c.Frames[i].Index] = i
	}
	rot := uprightRotation(c)
	dir := filepath.Join(outDir, "damage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logf("damage: %v; skipped", err)
		return nil, nil
	}
	detPath := filepath.Join(dir, "detections.json")
	key := fmt.Sprintf("%s|rot=%d|%s", video, rot, strings.Join(idx, ","))
	st, _ := stamp(video)
	keyPath := filepath.Join(dir, "input.json")
	cached := false
	if !live {
		if b, err := os.ReadFile(keyPath); err == nil {
			var old struct {
				Key   string     `json:"key"`
				Input inputStamp `json:"input"`
			}
			if json.Unmarshal(b, &old) == nil && old.Key == key && old.Input.equal(st) {
				cached = true
			}
		}
	}
	if !cached {
		cmd := exec.Command(py, filepath.Join(root, "ml", "damage_detect.py"), "--video", video,
			"--frames", strings.Join(idx, ","), "--rot", strconv.Itoa(rot), "--out", detPath)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			logf("damage: detector failed (%v); skipped", err)
			return nil, nil
		}
		b, _ := json.Marshal(map[string]any{"key": key, "input": st})
		_ = os.WriteFile(keyPath, b, 0o644)
	} else {
		logf("damage: reusing cached detections (use -live to recompute)")
	}
	b, err := os.ReadFile(detPath)
	if err != nil {
		logf("damage: %v; skipped", err)
		return nil, nil
	}
	var res struct {
		Detections []damage.Detection `json:"detections"`
	}
	if err := json.Unmarshal(b, &res); err != nil {
		logf("damage: %v; skipped", err)
		return nil, nil
	}
	regions := damage.Place(res.Detections, c, frameIndex, c.RGBK.Width, c.RGBK.Height, g.pf, g.shapes, g.ceilings)
	var ops []damage.Opening
	for _, o := range g.openings {
		if o.Kind == geometry.KindDoor || o.Kind == geometry.KindOpening {
			ops = append(ops, damage.Opening{Surface: damage.Surface{Room: o.Room, Wall: o.Wall},
				U0: o.Offset, U1: o.Offset + o.Width, HeadV: 2.05})
		}
	}
	flags := damage.Rules(regions, ops)
	logf("damage: %d frames, %d raw detections, %d regions on surfaces (score >= %.2f, >= %d views), %d flags",
		len(idx), len(res.Detections), len(regions), damage.MinScore, damage.MinViews, len(flags))
	return regions, flags
}

// uprightRotation returns the clockwise quarter turns that make the
// capture's video frames upright, from the ARKit poses: world up expressed
// in camera axes (x right, y down), median over the walk.
func uprightRotation(c *strayscanner.Capture) int {
	var xs, ys []float64
	for _, f := range c.Frames {
		if f.Pose == nil {
			continue
		}
		// Camera-to-world R: world up (0,1,0) in camera axes is row 1 of R.
		xs = append(xs, f.Pose.R[1][0])
		ys = append(ys, f.Pose.R[1][1])
	}
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	sort.Float64s(ys)
	ux, uy := xs[len(xs)/2], ys[len(ys)/2]
	if math.Abs(uy) >= math.Abs(ux) {
		if uy < 0 {
			return 0
		}
		return 2
	}
	if ux < 0 {
		return 1
	}
	return 3
}

// addDamage writes regions, flags and scope items into the plan, using
// the room and wall ids assemble assigned.
func addDamage(p *output.Plan, shapes []*geometry.RoomShape, regions []damage.Region, flags []damage.Flag) {
	roomID := map[int]string{}
	n := 0
	for i, s := range shapes {
		if s != nil {
			n++
			roomID[i] = fmt.Sprintf("R%d", n)
		}
	}
	extent := func(v float64) output.Measurement { return measure(v, math.Hypot(0.03, 0.15*v), 0, 1, "measured") }
	surface := func(s damage.Surface) string {
		switch s.Wall {
		case damage.SurfFloor:
			return "floor"
		case damage.SurfCeiling:
			return "ceiling"
		}
		return fmt.Sprintf("W%d", s.Wall+1)
	}
	ids := make([]string, len(regions))
	for i, r := range regions {
		ids[i] = fmt.Sprintf("D%d", i+1)
		w, h := extent(r.Width()), extent(r.Height())
		relW := (w.CI90[1] - w.CI90[0]) / 2 / math.Max(r.Width(), 1e-6)
		relH := (h.CI90[1] - h.CI90[0]) / 2 / math.Max(r.Height(), 1e-6)
		area := measure(r.Area(), math.Hypot(relW, relH)/1.6449*r.Area(), 0, 1, "measured")
		centre := [2]float64{round((r.U0+r.U1)/2, 3), round((r.V0+r.V1)/2, 3)}
		p.Damage = append(p.Damage, output.Damage{
			ID: ids[i], Class: r.Class,
			Location: output.SurfaceRef{Room: roomID[r.Surface.Room], Surface: surface(r.Surface)},
			CentreM:  &centre, AreaM2: area, WidthM: w, HeightM: h,
			DetectionConfidence: round(r.Score, 3), Views: r.Views,
		})
	}
	flagIDs := map[int][]string{}
	for k, f := range flags {
		id := fmt.Sprintf("F%d", k+1)
		r := regions[f.Region]
		p.ConcealedFlags = append(p.ConcealedFlags, output.ConcealedFlag{
			ID: id, RuleID: f.RuleID, Location: output.SurfaceRef{Room: roomID[r.Surface.Room], Surface: surface(r.Surface)},
			DamageIDs: []string{ids[f.Region]}, Message: f.Message,
		})
		flagIDs[f.Region] = append(flagIDs[f.Region], id)
	}
	for i, r := range regions {
		action, unit := damage.ScopeAction(r.Class)
		qty := p.Damage[i].AreaM2
		switch unit {
		case "m":
			qty = p.Damage[i].WidthM
			if r.Height() > r.Width() {
				qty = p.Damage[i].HeightM
			}
		case "each":
			qty = output.Measurement{Value: 1, CI90: [2]float64{1, 1}, Source: "measured"}
		}
		p.Scope = append(p.Scope, output.ScopeItem{
			ID: fmt.Sprintf("S%d", i+1), Location: p.Damage[i].Location, Action: action,
			Quantity: qty, Unit: unit, DamageIDs: []string{ids[i]}, FlagIDs: flagIDs[i],
		})
	}
	if p.Diagnostics == nil {
		p.Diagnostics = map[string]any{}
	}
	p.Diagnostics["damage_thresholds"] = map[string]any{"min_score": damage.MinScore, "min_views": damage.MinViews,
		"frame_every": damageEvery}
}
