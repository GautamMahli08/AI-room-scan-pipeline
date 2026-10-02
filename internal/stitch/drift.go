// Package stitch places rooms into one property plan and corrects
// accumulated pose drift.
//
// Drift correction is plane-anchored (SYSTEM_DESIGN.md §4.3): the walk is
// split into time chunks, each with a small rigid correction (plan
// translation, yaw, vertical shift). The measured wall faces and the floor
// are landmarks. Every chunk observes them through its own depth points,
// and one linear least-squares problem finds the chunk corrections and
// landmark positions that make all observations agree, with a random-walk
// smoothness prior between neighbouring chunks. A room revisited late in
// the walk re-observes the same faces, which is what closes the loop.
package stitch

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"gonum.org/v1/gonum/mat"

	"roomscan/internal/frame"
	"roomscan/internal/geom"
	"roomscan/internal/geometry"
	"roomscan/internal/geometry/pointcloud"
)

// Options control drift estimation.
type Options struct {
	ChunkSeconds float64 // length of a correction chunk
	FrameStride  int     // frames used for observations
	PixelStep    int     // pixel subsampling within a frame
	Window       float64 // points within ± this of a face observe it
	SegLen       float64 // wall observations are binned along the wall at this length
	// Random-walk prior between neighbouring chunks (standard deviations).
	SigTrans, SigYaw, SigVert float64
}

func DefaultOptions() Options {
	return Options{
		ChunkSeconds: 4, FrameStride: 1, PixelStep: 2, Window: 0.06, SegLen: 0.5,
		SigTrans: 0.01, SigYaw: 0.1 * math.Pi / 180, SigVert: 0.005,
	}
}

// Chunk is the correction of one time chunk, in plan coordinates.
type Chunk struct {
	T0, T1   float64 // timestamps covered
	Pivot    geometry.Pt
	Dx, Dy   float64 // plan translation, metres
	Dtheta   float64 // yaw, radians (counter-clockwise in the plan)
	Dh       float64 // vertical, metres
	WallObs  int     // wall observations in this chunk
	FloorObs bool
}

// Correction is a drift estimate for a capture.
type Correction struct {
	Chunks []Chunk
	// Scatter is the weighted RMS of wall-face observations about their
	// landmark, with no correction and as fitted with the chosen one.
	ScatterBefore, ScatterAfter float64
	// Held-out wall scatter (odd segments, never fitted) with no
	// correction and with the selected prior, and every prior tried.
	HeldOutNone, HeldOutBest float64
	PriorScale               float64 // 0 = no correction beat doing nothing
	Selection                []ScaleScore
	Observations             int
	Landmarks                int
}

// MaxShift is the largest translation applied to any chunk.
func (c *Correction) MaxShift() float64 {
	var m float64
	for _, ch := range c.Chunks {
		m = math.Max(m, math.Hypot(ch.Dx, ch.Dy))
	}
	return m
}

// MaxYaw is the largest yaw correction, radians.
func (c *Correction) MaxYaw() float64 {
	var m float64
	for _, ch := range c.Chunks {
		m = math.Max(m, math.Abs(ch.Dtheta))
	}
	return m
}

type wallRef struct {
	room, wall int
	n          geometry.Pt
	c          float64
	a, u       geometry.Pt
	length     float64
}

type obsKey struct{ lm, chunk, seg int }

type obsAcc struct {
	n      int
	s      float64
	px, py float64
}

// Estimate measures how each chunk sees the room walls and floor and
// solves for per-chunk corrections. shapes and pf are from a plan built
// with the capture's current poses.
func Estimate(src pointcloud.DepthSource, frames []frame.Frame, pf geometry.PlanFrame, shapes []*geometry.RoomShape, opt Options) (*Correction, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("drift: no frames")
	}
	t0 := frames[0].Timestamp
	chunkOf := func(t float64) int { return int((t - t0) / opt.ChunkSeconds) }
	nChunks := chunkOf(frames[len(frames)-1].Timestamp) + 1

	// Landmarks: measured walls of every room.
	var walls []wallRef
	roomWalls := make([][]int, len(shapes))
	for ri, s := range shapes {
		if s == nil {
			continue
		}
		for wi, w := range s.Walls {
			if w.Inferred {
				continue
			}
			a, b := s.Corners[wi], s.Corners[(wi+1)%len(s.Corners)]
			l := b.Sub(a).Norm()
			if l < 0.5 {
				continue
			}
			roomWalls[ri] = append(roomWalls[ri], len(walls))
			walls = append(walls, wallRef{room: ri, wall: wi, n: w.Line.N, c: w.Line.C, a: a, u: b.Sub(a).Scale(1 / l), length: l})
		}
	}

	pivotSum := make([]geometry.Pt, nChunks)
	pivotN := make([]int, nChunks)
	for _, f := range frames {
		if f.Pose == nil {
			continue
		}
		x, y, _ := pf.ToPlan(f.Pose.T)
		j := chunkOf(f.Timestamp)
		pivotSum[j] = pivotSum[j].Add(geometry.Pt{X: x, Y: y})
		pivotN[j]++
	}

	// Accumulate observations in parallel, one map per worker.
	const trim = 0.15
	workers := runtime.GOMAXPROCS(0)
	type acc struct {
		wall  map[obsKey]*obsAcc
		floor map[int]*obsAcc
	}
	accs := make([]acc, workers)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		accs[w] = acc{wall: map[obsKey]*obsAcc{}, floor: map[int]*obsAcc{}}
		wg.Add(1)
		go func(a acc) {
			defer wg.Done()
			for i := range jobs {
				f := frames[i]
				d, conf, err := src.LoadDepth(i)
				if err != nil || f.Pose == nil {
					continue
				}
				j := chunkOf(f.Timestamp)
				cx, cy, _ := pf.ToPlan(f.Pose.T)
				room := -1
				for ri, s := range shapes {
					if s != nil && geometry.PointInPolygon(geometry.Pt{X: cx, Y: cy}, s.Corners) {
						room = ri
						break
					}
				}
				for v := 0; v < d.Height; v += opt.PixelStep {
					for u := 0; u < d.Width; u += opt.PixelStep {
						z := float64(d.At(u, v))
						if (conf != nil && conf.At(u, v) < 2) || z < 0.1 || z > 4 {
							continue
						}
						p := f.Pose.Apply(geom.Vec3{(float64(u) - f.K.Cx) * z / f.K.Fx, (float64(v) - f.K.Cy) * z / f.K.Fy, z})
						x, y, h := pf.ToPlan(p)
						if math.Abs(h) < 0.05 {
							o := a.floor[j]
							if o == nil {
								o = &obsAcc{}
								a.floor[j] = o
							}
							o.n++
							o.s += h
							continue
						}
						if room < 0 || h < 0.3 || h > 2.0 {
							continue
						}
						pt := geometry.Pt{X: x, Y: y}
						for _, li := range roomWalls[room] {
							wr := &walls[li]
							s := wr.n.Dot(pt) - wr.c
							if math.Abs(s) > opt.Window {
								continue
							}
							t := pt.Sub(wr.a).Dot(wr.u)
							if t < trim || t > wr.length-trim {
								continue
							}
							k := obsKey{li, j, int(t / opt.SegLen)}
							o := a.wall[k]
							if o == nil {
								o = &obsAcc{}
								a.wall[k] = o
							}
							o.n++
							o.s += s
							o.px += x
							o.py += y
						}
					}
				}
			}
		}(accs[w])
	}
	for i := 0; i < len(frames); i += opt.FrameStride {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	wallObs := map[obsKey]*obsAcc{}
	floorObs := map[int]*obsAcc{}
	for _, a := range accs {
		for k, o := range a.wall {
			m := wallObs[k]
			if m == nil {
				m = &obsAcc{}
				wallObs[k] = m
			}
			m.n += o.n
			m.s += o.s
			m.px += o.px
			m.py += o.py
		}
		for k, o := range a.floor {
			m := floorObs[k]
			if m == nil {
				m = &obsAcc{}
				floorObs[k] = m
			}
			m.n += o.n
			m.s += o.s
		}
	}

	// Keep observations with enough points, and landmarks seen by >= 2 chunks.
	const minPts = 30
	keys := make([]obsKey, 0, len(wallObs))
	chunksOf := map[int]map[int]bool{}
	for k, o := range wallObs {
		if o.n < minPts {
			continue
		}
		keys = append(keys, k)
		if chunksOf[k.lm] == nil {
			chunksOf[k.lm] = map[int]bool{}
		}
		chunksOf[k.lm][k.chunk] = true
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.lm != b.lm {
			return a.lm < b.lm
		}
		if a.chunk != b.chunk {
			return a.chunk < b.chunk
		}
		return a.seg < b.seg
	})
	lmIndex := map[int]int{}
	var used []obsKey
	for _, k := range keys {
		if len(chunksOf[k.lm]) < 2 {
			continue
		}
		if _, ok := lmIndex[k.lm]; !ok {
			lmIndex[k.lm] = len(lmIndex)
		}
		used = append(used, k)
	}

	// Linear observation rows. Wall: n·d_j + dθ_j·n·perp(p̄ − c_j) − l_w = −s̄.
	// Floor: dh_j − f = −h̄.
	pivot := func(j int) geometry.Pt {
		if pivotN[j] == 0 {
			return geometry.Pt{}
		}
		return pivotSum[j].Scale(1 / float64(pivotN[j]))
	}
	obsW := func(n int) float64 {
		ne := math.Min(float64(n), 200)
		return 1 / math.Sqrt(0.01*0.01/ne+0.002*0.002)
	}
	var rows []obsRow
	for _, k := range used {
		o := wallObs[k]
		wr := walls[k.lm]
		pm := geometry.Pt{X: o.px / float64(o.n), Y: o.py / float64(o.n)}
		g := pm.Sub(pivot(k.chunk))
		rows = append(rows, obsRow{
			chunk: k.chunk, lm: lmIndex[k.lm], wall: true, test: k.seg%2 == 1,
			nx: wr.n.X, ny: wr.n.Y, ntheta: wr.n.Dot(geometry.Pt{X: -g.Y, Y: g.X}),
			value: o.s / float64(o.n), w: obsW(o.n),
		})
	}
	floorChunks := make([]int, 0, len(floorObs))
	for j := range floorObs {
		floorChunks = append(floorChunks, j)
	}
	sort.Ints(floorChunks)
	for _, j := range floorChunks {
		if o := floorObs[j]; o.n >= minPts {
			rows = append(rows, obsRow{chunk: j, value: o.s / float64(o.n), w: obsW(o.n)})
		}
	}

	sys := &system{nChunks: nChunks, nLm: len(lmIndex), rows: rows,
		sig: [4]float64{opt.SigTrans, opt.SigTrans, opt.SigYaw, opt.SigVert}}

	// Choose the prior strength on held-out wall segments (odd segments are
	// never fitted). Scale 0 means no correction: landmarks only.
	corr := &Correction{Observations: len(used), Landmarks: len(lmIndex)}
	baseline, err := sys.solve(true, 0)
	if err != nil {
		return nil, err
	}
	corr.HeldOutNone = sys.scatter(baseline, true)
	bestScale, bestScore := 0.0, corr.HeldOutNone
	for _, s := range []float64{0.25, 0.5, 1, 2, 4, 8} {
		x, err := sys.solve(true, s)
		if err != nil {
			return nil, err
		}
		sc := sys.scatter(x, true)
		corr.Selection = append(corr.Selection, ScaleScore{Scale: s, HeldOutMM: sc * 1000})
		if sc < bestScore*0.95 { // must beat the best so far by 5 %
			bestScale, bestScore = s, sc
		}
	}
	corr.PriorScale, corr.HeldOutBest = bestScale, bestScore

	none, err := sys.solve(false, 0)
	if err != nil {
		return nil, err
	}
	corr.ScatterBefore = sys.scatter(none, false)
	x := none
	if bestScale > 0 {
		if x, err = sys.solve(false, bestScale); err != nil {
			return nil, err
		}
	}
	corr.ScatterAfter = sys.scatter(x, false)

	for j := 0; j < nChunks; j++ {
		ch := Chunk{T0: t0 + float64(j)*opt.ChunkSeconds, T1: t0 + float64(j+1)*opt.ChunkSeconds, Pivot: pivot(j)}
		if j > 0 {
			ch.Dx, ch.Dy, ch.Dtheta, ch.Dh = x[sys.col(j, 0)], x[sys.col(j, 1)], x[sys.col(j, 2)], x[sys.col(j, 3)]
		}
		corr.Chunks = append(corr.Chunks, ch)
	}
	for _, r := range rows {
		if r.wall {
			corr.Chunks[r.chunk].WallObs++
		} else {
			corr.Chunks[r.chunk].FloorObs = true
		}
	}
	return corr, nil
}

// ScaleScore is the held-out wall scatter for one prior scale.
type ScaleScore struct {
	Scale     float64 `json:"scale"`
	HeldOutMM float64 `json:"held_out_mm"`
}

// obsRow is one linearised observation.
type obsRow struct {
	chunk, lm      int
	wall, test     bool
	nx, ny, ntheta float64 // coefficients of dx, dy, dθ (wall rows)
	value, w       float64 // observed offset (wall) or height (floor), weight
}

// system is the drift least-squares problem. Unknowns: chunks 1..J-1 ×
// (dx, dy, dθ, dh), then wall landmarks, then the floor landmark. Chunk 0
// is the gauge.
type system struct {
	nChunks, nLm int
	rows         []obsRow
	sig          [4]float64
}

func (s *system) col(chunk, k int) int { return (chunk-1)*4 + k }
func (s *system) lmCol(l int) int      { return (s.nChunks-1)*4 + l }
func (s *system) floorCol() int        { return (s.nChunks-1)*4 + s.nLm }
func (s *system) nUnk() int            { return s.floorCol() + 1 }

// residual is row r corrected by solution x, minus its landmark.
func (s *system) residual(r obsRow, x []float64) float64 {
	v := r.value
	if r.wall {
		if r.chunk > 0 {
			v += r.nx*x[s.col(r.chunk, 0)] + r.ny*x[s.col(r.chunk, 1)] + r.ntheta*x[s.col(r.chunk, 2)]
		}
		return v - x[s.lmCol(r.lm)]
	}
	if r.chunk > 0 {
		v += x[s.col(r.chunk, 3)]
	}
	return v - x[s.floorCol()]
}

// solve fits landmarks and, when scale > 0, chunk corrections with the
// random-walk prior widened by scale. trainOnly drops held-out wall rows.
func (s *system) solve(trainOnly bool, scale float64) ([]float64, error) {
	n := s.nUnk()
	var A [][]float64
	var b []float64
	add := func(coef map[int]float64, rhs, w float64) {
		row := make([]float64, n)
		for c, v := range coef {
			row[c] = v * w
		}
		A = append(A, row)
		b = append(b, rhs*w)
	}
	for _, r := range s.rows {
		if trainOnly && r.test {
			continue
		}
		var coef map[int]float64
		if r.wall {
			coef = map[int]float64{s.lmCol(r.lm): -1}
			if r.chunk > 0 && scale > 0 {
				coef[s.col(r.chunk, 0)] = r.nx
				coef[s.col(r.chunk, 1)] = r.ny
				coef[s.col(r.chunk, 2)] = r.ntheta
			}
		} else {
			coef = map[int]float64{s.floorCol(): -1}
			if r.chunk > 0 && scale > 0 {
				coef[s.col(r.chunk, 3)] = 1
			}
		}
		add(coef, -r.value, r.w)
	}
	for j := 1; j < s.nChunks; j++ {
		for k := 0; k < 4; k++ {
			if scale == 0 { // corrections pinned to zero
				add(map[int]float64{s.col(j, k): 1}, 0, 1e6)
				continue
			}
			coef := map[int]float64{s.col(j, k): 1}
			if j > 1 {
				coef[s.col(j-1, k)] = -1
			}
			add(coef, 0, 1/(s.sig[k]*scale))
			add(map[int]float64{s.col(j, k): 1}, 0, 1/(s.sig[k]*scale*50))
		}
	}
	// Weak priors keep landmarks without rows (train-only) determined.
	for l := 0; l < s.nLm; l++ {
		add(map[int]float64{s.lmCol(l): 1}, 0, 1e-3)
	}
	add(map[int]float64{s.floorCol(): 1}, 0, 1e-3)

	M := mat.NewDense(len(A), n, nil)
	for i, row := range A {
		M.SetRow(i, row)
	}
	var x mat.VecDense
	if err := x.SolveVec(M, mat.NewVecDense(len(b), b)); err != nil {
		return nil, fmt.Errorf("drift solve: %w", err)
	}
	return x.RawVector().Data, nil
}

// scatter is the weighted RMS wall residual over held-out rows (heldOut)
// or all wall rows.
func (s *system) scatter(x []float64, heldOut bool) float64 {
	var ss, ws float64
	for _, r := range s.rows {
		if !r.wall || (heldOut && !r.test) {
			continue
		}
		d := s.residual(r, x)
		ss += r.w * d * d
		ws += r.w
	}
	if ws == 0 {
		return 0
	}
	return math.Sqrt(ss / ws)
}

// at returns the correction at time t, linear between chunk centres.
func (c *Correction) at(t float64) Chunk {
	n := len(c.Chunks)
	if n == 0 {
		return Chunk{}
	}
	mid := func(i int) float64 { return (c.Chunks[i].T0 + c.Chunks[i].T1) / 2 }
	if t <= mid(0) {
		return c.Chunks[0]
	}
	if t >= mid(n-1) {
		return c.Chunks[n-1]
	}
	i := int((t - c.Chunks[0].T0) / (c.Chunks[0].T1 - c.Chunks[0].T0))
	if t < mid(i) {
		i--
	}
	a, b := c.Chunks[i], c.Chunks[i+1]
	w := (t - mid(i)) / (mid(i+1) - mid(i))
	lerp := func(x, y float64) float64 { return x + w*(y-x) }
	return Chunk{
		Pivot: geometry.Pt{X: lerp(a.Pivot.X, b.Pivot.X), Y: lerp(a.Pivot.Y, b.Pivot.Y)},
		Dx:    lerp(a.Dx, b.Dx), Dy: lerp(a.Dy, b.Dy), Dtheta: lerp(a.Dtheta, b.Dtheta), Dh: lerp(a.Dh, b.Dh),
	}
}

// Apply corrects every frame pose in place: p' = Ry(dθ)(p − pivot) + pivot + d.
// A plan-frame yaw is the same angle about world +Y (plan is world (x, −z)
// rotated about the vertical), and plan vectors map to world through the
// plan frame's linear part.
func Apply(frames []frame.Frame, pf geometry.PlanFrame, c *Correction) {
	for i := range frames {
		f := &frames[i]
		if f.Pose == nil {
			continue
		}
		ch := c.at(f.Timestamp)
		R := geom.RotY(ch.Dtheta)
		px, pz := pf.ToWorldXZ(ch.Pivot.X, ch.Pivot.Y)
		dx, dz := pf.ToWorldXZ(ch.Dx, ch.Dy)
		pivot := geom.Vec3{px, 0, pz}
		T := R.MulVec(f.Pose.T.Sub(pivot)).Add(pivot).Add(geom.Vec3{dx, ch.Dh, dz})
		f.Pose = &frame.Pose{R: R.Mul(f.Pose.R), T: T}
	}
}
