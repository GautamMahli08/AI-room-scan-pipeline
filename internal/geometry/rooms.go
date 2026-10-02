package geometry

// Region is one connected interior area of the plan raster.
type Region struct {
	Label     int32
	Cells     []int // raster indices
	Area      float64
	TrajCells int // number of trajectory samples inside
}

// SegmentRooms splits the plan into rooms. Barriers are wall cells plus
// every wall gap up to the closing width (doors and windows are closed so
// rooms do not leak into each other or outdoors). The interior is any
// observed cell (floor, furniture, or space carved free by depth rays) that
// is not a barrier; rooms are the
// connected components of the interior that the camera walked through.
// Unobserved holes smaller than maxHole m² inside a room are filled (e.g.
// the floor under a low table); larger ones stay out of the room.
//
// It returns a per-cell label (0 = not in a room) and the regions sorted by
// label.
func SegmentRooms(r *Rasters, wall []bool, gaps []Gap, free []uint16, traj [][2]float64, maxHole float64) ([]int32, []Region) {
	n := r.W * r.H
	barrier := make([]bool, n)
	copy(barrier, wall)
	for _, g := range gaps {
		ForGapCells(r, g, 0, func(i int) { barrier[i] = true })
	}
	occ := r.OccupiedMask()
	interior := make([]bool, n)
	for i := range interior {
		interior[i] = !barrier[i] && (r.Floor[i] > 0 || occ[i] || free[i] >= minFreeRays)
	}

	labels := make([]int32, n)
	var regions []Region
	var next int32 = 1
	for _, p := range traj {
		cx, cy, ok := r.Cell(p[0], p[1])
		if !ok {
			continue
		}
		seed := nearestInterior(r, interior, cx, cy, 10)
		if seed < 0 {
			continue
		}
		if l := labels[seed]; l != 0 {
			regions[l-1].TrajCells++
			continue
		}
		cells := flood(r, seed, func(i int) bool { return interior[i] && labels[i] == 0 })
		for _, i := range cells {
			labels[i] = next
		}
		regions = append(regions, Region{Label: next, Cells: cells, TrajCells: 1})
		next++
	}

	fillHoles(r, labels, regions, int(maxHole/(r.Res*r.Res)))

	// Split open-plan regions at narrow passages, drop slivers, relabel.
	inRoom := make([]bool, n)
	byLabel := make([][]int, len(regions))
	for i, l := range labels {
		if l > 0 {
			inRoom[i] = true
			byLabel[l-1] = append(byLabel[l-1], i)
		}
	}
	dist := distanceTransform(r.Grid2, inRoom)
	var parts [][]int
	for _, cells := range byLabel {
		if float64(len(cells))*r.Res*r.Res < minRoomArea {
			continue
		}
		seeds := splitRegion(r, cells, dist)
		if len(seeds) == 1 {
			parts = append(parts, cells)
			continue
		}
		owner := growSeeds(r, cells, seeds)
		sub := make([][]int, len(seeds))
		for _, i := range cells {
			if k, ok := owner[i]; ok {
				sub[k] = append(sub[k], i)
			}
		}
		parts = append(parts, sub...)
	}

	for i := range labels {
		labels[i] = 0
	}
	regions = regions[:0]
	for _, cells := range parts {
		if float64(len(cells))*r.Res*r.Res < minRoomArea {
			continue
		}
		l := int32(len(regions) + 1)
		for _, i := range cells {
			labels[i] = l
		}
		regions = append(regions, Region{Label: l, Cells: cells, Area: float64(len(cells)) * r.Res * r.Res})
	}
	for _, p := range traj {
		if cx, cy, ok := r.Cell(p[0], p[1]); ok {
			if l := labels[r.Idx(cx, cy)]; l > 0 {
				regions[l-1].TrajCells++
			}
		}
	}
	return labels, regions
}

// minFreeRays is how many carving rays must cross a cell before it counts
// as observed free space.
const minFreeRays = 3

// minRoomArea drops slivers (e.g. a few cells a trajectory sample landed
// on between barriers).
const minRoomArea = 1.0

// ForGapCells calls fn for every raster cell of g, widened across the wall
// by pad cells.
func ForGapCells(r *Rasters, g Gap, pad int, fn func(i int)) {
	for a := g.A0; a < g.A1; a++ {
		for b := g.B0 - pad; b < g.B1+pad; b++ {
			x, y := a, b
			if g.Orient == OrientV {
				x, y = b, a
			}
			if x >= 0 && y >= 0 && x < r.W && y < r.H {
				fn(r.Idx(x, y))
			}
		}
	}
}

// nearestInterior finds the interior cell closest to (cx, cy) within
// radius cells, or -1.
func nearestInterior(r *Rasters, interior []bool, cx, cy, radius int) int {
	best, bestD := -1, radius*radius+1
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			x, y := cx+dx, cy+dy
			if d := dx*dx + dy*dy; d < bestD && x >= 0 && y >= 0 && x < r.W && y < r.H && interior[r.Idx(x, y)] {
				best, bestD = r.Idx(x, y), d
			}
		}
	}
	return best
}

// flood returns the 4-connected component of cells reachable from seed
// through cells where ok is true.
func flood(r *Rasters, seed int, ok func(i int) bool) []int {
	seen := map[int]bool{seed: true}
	stack, out := []int{seed}, []int{}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, i)
		x, y := i%r.W, i/r.W
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= r.W || ny >= r.H {
				continue
			}
			j := r.Idx(nx, ny)
			if !seen[j] && ok(j) {
				seen[j] = true
				stack = append(stack, j)
			}
		}
	}
	return out
}

// fillHoles assigns to a room every unlabelled component of at most maxCells
// cells whose whole boundary is that room.
func fillHoles(r *Rasters, labels []int32, regions []Region, maxCells int) {
	visited := make([]bool, len(labels))
	for start := range labels {
		if labels[start] != 0 || visited[start] {
			continue
		}
		var touches int32 = -1 // -1 none yet, 0 border/several rooms
		comp := flood(r, start, func(i int) bool { return labels[i] == 0 && !visited[i] })
		for _, i := range comp {
			visited[i] = true
			x, y := i%r.W, i/r.W
			if x == 0 || y == 0 || x == r.W-1 || y == r.H-1 {
				touches = 0
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= r.W || ny >= r.H {
					continue
				}
				if l := labels[r.Idx(nx, ny)]; l != 0 {
					if touches == -1 {
						touches = l
					} else if touches != l {
						touches = 0
					}
				}
			}
		}
		if touches > 0 && len(comp) <= maxCells {
			for _, i := range comp {
				labels[i] = touches
			}
		}
	}
}
