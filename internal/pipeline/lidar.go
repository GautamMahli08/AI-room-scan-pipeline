// Package pipeline runs one capture end to end and assembles the output
// plan.
package pipeline

import (
	"fmt"
	"log"
	"math"
	"path/filepath"
	"time"

	"roomscan/internal/calib"
	"roomscan/internal/geometry"
	"roomscan/internal/geometry/pointcloud"
	"roomscan/internal/ingest/strayscanner"
	"roomscan/internal/output"
	"roomscan/internal/stitch"
)

// Version is reported in provenance.
const Version = "0.1.0"

// Options control a LiDAR run.
type Options struct {
	Stride    int     // use every n-th frame for fusion
	Voxel     float64 // voxel size, metres
	MinFrames int32   // drop voxels seen by fewer frames
	PlanRes   float64 // plan raster resolution, metres
	WritePLY  bool
	Debug     bool // write debug rasters
	// Drift enables plane-anchored drift correction. The uncorrected plan
	// is also written (plan_drift_off.*) as the ablation.
	Drift      bool
	DriftIters int
}

func DefaultOptions() Options {
	return Options{Stride: 1, Voxel: 0.02, MinFrames: 2, PlanRes: 0.02, Drift: true, DriftIters: 2}
}

// Result is everything a LiDAR run produced.
type Result struct {
	Plan *output.Plan
}

// RunLiDAR processes a Stray Scanner export directory and writes plan.json
// and plan.svg (plus debug files) to outDir.
func RunLiDAR(exportDir, outDir, captureID string, opt Options) (*Result, error) {
	start := time.Now()
	logf := func(format string, a ...any) { log.Printf("[%s] "+format, append([]any{captureID}, a...)...) }

	c, err := strayscanner.Load(exportDir)
	if err != nil {
		return nil, err
	}
	logf("ingest: %d frames, depth %dx%d", len(c.Frames), c.DepthWidth, c.DepthHeight)

	off, err := extract(c, opt, outDir, logf)
	if err != nil {
		return nil, err
	}
	final := off
	var report *DriftReport
	if opt.Drift {
		report = &DriftReport{}
		t := time.Now()
		for it := 0; it < opt.DriftIters; it++ {
			cr, err := stitch.Estimate(c, c.Frames, final.pf, final.shapes, stitch.DefaultOptions())
			if err != nil {
				return nil, err
			}
			report.Iterations = append(report.Iterations, iterSummary(cr))
			if it == 0 {
				report.Chunks = cr.Chunks
			}
			logf("drift iter %d: %d obs on %d walls; held-out scatter %.1f mm uncorrected, %.1f mm with prior x%g; max shift %.1f cm, max yaw %.2f°",
				it+1, cr.Observations, cr.Landmarks, cr.HeldOutNone*1000, cr.HeldOutBest*1000, cr.PriorScale, cr.MaxShift()*100, cr.MaxYaw()*180/math.Pi)
			if cr.PriorScale == 0 {
				logf("drift: no correction beats none on held-out walls; poses kept")
				break
			}
			stitch.Apply(c.Frames, final.pf, cr)
			if final, err = extract(c, opt, outDir, logf); err != nil {
				return nil, err
			}
		}
		// Re-measure the scatter on the final, corrected plan.
		cr, err := stitch.Estimate(c, c.Frames, final.pf, final.shapes, stitch.DefaultOptions())
		if err != nil {
			return nil, err
		}
		report.Final = iterSummary(cr)
		logf("drift: wall scatter %.1f mm uncorrected -> %.1f mm corrected (%.1fs)",
			report.Iterations[0].ScatterBeforeMM, report.Final.ScatterBeforeMM, time.Since(t).Seconds())
	}

	write := func(g *scene, drift, suffix string) (*output.Plan, error) {
		p := assemble(captureID, g.floor, g.shapes, g.ceilings, g.openings, g.bounds)
		p.Provenance = output.Provenance{
			PipelineVersion: Version, Input: filepath.ToSlash(exportDir), Models: []output.Model{},
			DriftCorrection: drift, RuntimeS: math.Round(time.Since(start).Seconds()*10) / 10,
		}
		p.Diagnostics = g.diagnostics(c)
		if err := output.WriteJSON(filepath.Join(outDir, "plan"+suffix+".json"), p); err != nil {
			return nil, err
		}
		return p, output.WriteSVG(filepath.Join(outDir, "plan"+suffix+".svg"), p)
	}
	if !opt.Drift {
		p, err := write(off, "off", "")
		if err != nil {
			return nil, err
		}
		logf("wrote plan.json, plan.svg (%.1fs total)", time.Since(start).Seconds())
		return &Result{Plan: p}, nil
	}
	pOff, err := write(off, "off", "_drift_off")
	if err != nil {
		return nil, err
	}
	pOn, err := write(final, "on", "")
	if err != nil {
		return nil, err
	}
	report.FootprintOff, report.FootprintOn = pOff.FootprintAreaM2, pOn.FootprintAreaM2
	report.RoomsOff, report.RoomsOn = len(pOff.Rooms), len(pOn.Rooms)
	if err := writeJSONFile(filepath.Join(outDir, "drift.json"), report); err != nil {
		return nil, err
	}
	logf("footprint: drift off %.2f m², on %.2f m²; wrote plan.*, plan_drift_off.*, drift.json (%.1fs total)",
		pOff.FootprintAreaM2.Value, pOn.FootprintAreaM2.Value, time.Since(start).Seconds())
	return &Result{Plan: pOn}, nil
}

// scene is the geometry extracted from one set of poses.
type scene struct {
	pts      int
	floor    geometry.Floor
	pf       geometry.PlanFrame
	support  float64
	regions  []geometry.Region
	shapes   []*geometry.RoomShape
	ceilings []geometry.Ceiling
	openings []geometry.Opening
	bounds   []geometry.OpenBoundary
}

// extract fuses the capture with its current poses and builds the plan
// geometry.
func extract(c *strayscanner.Capture, opt Options, outDir string, logf func(string, ...any)) (*scene, error) {
	t := time.Now()
	fo := pointcloud.DefaultFuseOptions()
	fo.Stride, fo.VoxelSize = opt.Stride, opt.Voxel
	grid, err := pointcloud.Fuse(c, fo)
	if err != nil {
		return nil, err
	}
	pts := grid.Points(opt.MinFrames)
	logf("fuse: %d voxels, %d seen by >=%d frames (%.1fs)", grid.Len(), len(pts), opt.MinFrames, time.Since(t).Seconds())
	if opt.WritePLY {
		if err := pointcloud.WritePLY(filepath.Join(outDir, "cloud.ply"), pts); err != nil {
			return nil, err
		}
	}

	t = time.Now()
	floor, err := geometry.EstimateFloor(pts)
	if err != nil {
		return nil, err
	}
	pf := geometry.PlanFrame{Floor: floor}
	theta, support := geometry.EstimateManhattan(geometry.BuildRasters(pts, pf, opt.PlanRes))
	pf.Theta = theta
	pf.Theta = geometry.RefineManhattan(pts, pf)
	logf("floor: residual %.1f mm, tilt %.2f°; manhattan θ=%.2f° (%.0f%% support)", floor.Residual*1000, floor.TiltDeg, pf.Theta*180/math.Pi, support*100)

	r := geometry.BuildRasters(pts, pf, opt.PlanRes)
	wall := r.WallMask()
	orient := geometry.Orientations(r, wall)
	gaps := geometry.FindGaps(r, orient, opt.PlanRes, 1.6)
	var traj [][2]float64
	for _, f := range c.Frames {
		x, y, _ := pf.ToPlan(f.Pose.T)
		traj = append(traj, [2]float64{x, y})
	}
	free := geometry.Carve(c, r, 5, 4)
	labels, regions := geometry.SegmentRooms(r, wall, gaps, free, traj, 0.25)

	wp := geometry.IndexWallPoints(pts, pf, r.Grid2)
	clean := geometry.CleanLabels(r, regions)
	xs, ys := geometry.WallLines(r, orient)
	arr := geometry.BuildArrangement(r, clean, xs, ys)
	shapes := make([]*geometry.RoomShape, len(regions))
	for i, reg := range regions {
		shapes[i] = geometry.FitRoom(arr, reg.Label, wp)
	}
	g := &scene{
		pts: len(pts), floor: floor, pf: pf, support: support, regions: regions, shapes: shapes,
		ceilings: geometry.EstimateCeilings(pts, pf, r, labels, len(regions)),
		openings: geometry.AssignOpenings(r, gaps, shapes, labels, wp, traj),
		bounds:   geometry.OpenBoundaries(r, labels, shapes),
	}
	logf("plan: %d rooms, %d openings, %d open boundaries (%.1fs)", len(regions), len(g.openings), len(g.bounds), time.Since(t).Seconds())

	if opt.Debug {
		if err := writeDebug(filepath.Join(outDir, "debug_raster.png"), r, wall, orient, gaps, labels, traj, shapes); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (g *scene) diagnostics(c *strayscanner.Capture) map[string]any {
	return map[string]any{
		"frames":              len(c.Frames),
		"fused_points":        g.pts,
		"floor_residual_mm":   round(g.floor.Residual*1000, 1),
		"floor_tilt_deg":      round(g.floor.TiltDeg, 3),
		"manhattan_deg":       round(g.pf.Theta*180/math.Pi, 2),
		"manhattan_support":   round(g.support, 3),
		"room_regions_m2":     regionAreas(g.regions),
		"ceilings":            ceilingDiag(g.ceilings),
		"trajectory_length_m": round(pathLength(c), 1),
	}
}

// assemble converts geometry into the output contract with intervals.
func assemble(id string, floor geometry.Floor, shapes []*geometry.RoomShape, ceilings []geometry.Ceiling,
	openings []geometry.Opening, bounds []geometry.OpenBoundary) *output.Plan {
	p := output.NewPlan(id, "lidar")
	roomID := map[int]string{}
	openingID := 0
	floorSig := calib.FaceSigma(floor.Residual, floor.Inliers, calib.LiDARHeightSys)
	var footVar, footArea float64

	for ri, s := range shapes {
		if s == nil {
			continue
		}
		rid := fmt.Sprintf("R%d", len(p.Rooms)+1)
		roomID[ri] = rid
		room := output.Room{ID: rid, Walls: []output.Wall{}, Openings: []output.Opening{}}
		n := len(s.Corners)
		sig := make([]float64, n)
		for i, w := range s.Walls {
			sig[i] = calib.InferredFaceSig
			if !w.Inferred {
				sig[i] = calib.FaceSigma(w.Spread, w.Support, calib.LiDARFaceSys)
			}
		}
		var areaVar float64
		for i := range s.Walls {
			a, b := s.Corners[i], s.Corners[(i+1)%n]
			l := b.Sub(a).Norm()
			ls := calib.LengthSigma(sig[(i-1+n)%n], sig[(i+1)%n])
			src := "measured"
			if s.Walls[i].Inferred || s.Walls[(i-1+n)%n].Inferred || s.Walls[(i+1)%n].Inferred {
				src = "inferred"
			}
			room.Walls = append(room.Walls, output.Wall{
				ID: fmt.Sprintf("W%d", i+1), Start: [2]float64{round(a.X, 3), round(a.Y, 3)}, End: [2]float64{round(b.X, 3), round(b.Y, 3)},
				LengthM:  meas(l, ls, src),
				Inferred: s.Walls[i].Inferred,
			})
			room.Polygon = append(room.Polygon, [2]float64{round(a.X, 3), round(a.Y, 3)})
			areaVar += (l * sig[i]) * (l * sig[i])
		}
		area := geometry.PolygonArea(s.Corners)
		room.FloorAreaM2 = meas(area, math.Sqrt(areaVar), "measured")
		footArea += area
		footVar += areaVar

		cl := ceilings[ri]
		if cl.Observed {
			room.CeilingHeightM = output.ObservedValue(meas(cl.Height, calib.HeightSigma(calib.FaceSigma(cl.Spread, cl.Support, calib.LiDARHeightSys), floorSig), "measured"))
			for _, h := range cl.OtherLevels {
				room.Notes = append(room.Notes, output.Note{Code: "other",
					Message: fmt.Sprintf("part of this room has a different ceiling level at %.2f m (bulkhead or merged space)", h)})
			}
		} else {
			room.CeilingHeightM = output.Unobserved(cl.Reason)
		}
		p.Rooms = append(p.Rooms, room)
	}

	// Openings, keyed back to rooms.
	type adjKey [2]string
	adj := map[adjKey][]string{}
	var adjOrder []adjKey
	addAdj := func(a, b, via string) {
		if a == b || a == "" || b == "" {
			return
		}
		if a > b {
			a, b = b, a
		}
		k := adjKey{a, b}
		if _, ok := adj[k]; !ok {
			adjOrder = append(adjOrder, k)
		}
		adj[k] = append(adj[k], via)
	}
	roomIdx := func(rid string) int {
		for i := range p.Rooms {
			if p.Rooms[i].ID == rid {
				return i
			}
		}
		return -1
	}
	for _, o := range openings {
		rid, ok := roomID[o.Room]
		if !ok {
			continue
		}
		room := &p.Rooms[roomIdx(rid)]
		if o.Kind == geometry.KindUnobserved {
			room.Notes = append(room.Notes, output.Note{Code: "unobserved_gap", Wall: fmt.Sprintf("W%d", o.Wall+1),
				Message: fmt.Sprintf("%.2f m gap with nothing observed in or through it; not reported as an opening", o.Width)})
			continue
		}
		openingID++
		oid := fmt.Sprintf("O%d", openingID)
		ws := 0.025 // raster-only width: 2 cm cells at both jambs
		if o.JambPoints > 0 {
			ws = 0.012 // jambs from points on the wall face
		}
		off := meas(o.Offset, ws/math.Sqrt2, "measured")
		room.Openings = append(room.Openings, output.Opening{
			ID: oid, Wall: fmt.Sprintf("W%d", o.Wall+1), Type: o.Kind.String(),
			OffsetM: &off, WidthM: meas(o.Width, ws, "measured"),
		})
		if len(o.Rooms) == 2 {
			addAdj(roomID[int(o.Rooms[0])-1], roomID[int(o.Rooms[1])-1], oid)
		}
	}
	for _, b := range bounds {
		rid, ok := roomID[b.Room]
		if !ok {
			continue
		}
		openingID++
		oid := fmt.Sprintf("O%d", openingID)
		room := &p.Rooms[roomIdx(rid)]
		off := meas(b.Offset, calib.InferredFaceSig, "inferred")
		room.Openings = append(room.Openings, output.Opening{
			ID: oid, Wall: fmt.Sprintf("W%d", b.Wall+1), Type: "opening",
			OffsetM: &off, WidthM: meas(b.Width, calib.InferredFaceSig, "inferred"),
		})
		addAdj(rid, roomID[b.Other], oid)
	}
	for _, k := range adjOrder {
		p.Adjacency = append(p.Adjacency, output.Adjacency{Rooms: [2]string{k[0], k[1]}, Via: adj[k]})
	}
	p.FootprintAreaM2 = meas(footArea, math.Sqrt(footVar), "measured")
	return p
}

func meas(v, sigma float64, src string) output.Measurement {
	ci := calib.Interval(v, sigma)
	return output.Measurement{Value: round(v, 3), CI90: [2]float64{round(ci[0], 3), round(ci[1], 3)}, Source: src}
}

func round(v float64, d int) float64 {
	m := math.Pow(10, float64(d))
	return math.Round(v*m) / m
}

func regionAreas(rs []geometry.Region) []float64 {
	var out []float64
	for _, r := range rs {
		out = append(out, round(r.Area, 2))
	}
	return out
}

func pathLength(c *strayscanner.Capture) float64 {
	var l float64
	for i := 1; i < len(c.Frames); i++ {
		l += c.Frames[i].Pose.T.Sub(c.Frames[i-1].Pose.T).Norm()
	}
	return l
}

func ceilingDiag(cs []geometry.Ceiling) []map[string]any {
	var out []map[string]any
	for _, c := range cs {
		out = append(out, map[string]any{"observed": c.Observed, "height": round(c.Height, 3), "coverage": round(c.Coverage, 2), "support": c.Support, "spread_mm": round(c.Spread*1000, 1), "other_levels": c.OtherLevels})
	}
	return out
}
