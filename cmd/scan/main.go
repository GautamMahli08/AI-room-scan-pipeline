// Command scan runs the room-scanning pipeline on one capture.
//
//	scan run <capture_dir> [-out results] [-stride 1] [-voxel 0.02] [-ply]
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"roomscan/internal/geometry"
	"roomscan/internal/geometry/pointcloud"
	"roomscan/internal/ingest/strayscanner"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: scan run <capture_dir> [flags]")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 || os.Args[1] != "run" {
		usage()
	}
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	out := fs.String("out", "results", "output root directory")
	stride := fs.Int("stride", 1, "use every n-th frame")
	voxel := fs.Float64("voxel", 0.02, "voxel size in metres")
	minFrames := fs.Int("min-frames", 2, "drop voxels seen by fewer distinct frames")
	ply := fs.Bool("ply", false, "write the fused point cloud as PLY")

	// Accept the capture dir before or after flags.
	args := os.Args[2:]
	var dir string
	if len(args) > 0 && args[0][0] != '-' {
		dir, args = args[0], args[1:]
	}
	fs.Parse(args)
	if dir == "" && fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if dir == "" {
		usage()
	}

	if err := run(dir, *out, *stride, *voxel, int32(*minFrames), *ply); err != nil {
		log.Fatal(err)
	}
}

// captureDir resolves either an export directory or a sample folder that
// contains exactly one export (e.g. single_room/ -> single_room/c00a170fe1).
func captureDir(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "odometry.csv")); err == nil {
		return dir, nil
	}
	sub, _ := filepath.Glob(filepath.Join(dir, "*", "odometry.csv"))
	if len(sub) == 1 {
		return filepath.Dir(sub[0]), nil
	}
	return "", fmt.Errorf("%s: no Stray Scanner export found (want odometry.csv here or in one subfolder, found %d)", dir, len(sub))
}

func run(dir, outRoot string, stride int, voxel float64, minFrames int32, ply bool) error {
	start := time.Now()
	cdir, err := captureDir(dir)
	if err != nil {
		return err
	}
	id := filepath.Base(filepath.Clean(dir))
	outDir := filepath.Join(outRoot, id)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	c, err := strayscanner.Load(cdir)
	if err != nil {
		return err
	}
	log.Printf("[%s] ingest: %d frames, depth %dx%d (%.1fs)", id, len(c.Frames), c.DepthWidth, c.DepthHeight, time.Since(start).Seconds())

	t := time.Now()
	opt := pointcloud.DefaultFuseOptions()
	opt.Stride, opt.VoxelSize = stride, voxel
	g, err := pointcloud.Fuse(c, opt)
	if err != nil {
		return err
	}
	pts := g.Points(minFrames)
	log.Printf("[%s] fuse: %d voxels, %d seen by >=%d frames (%.1fs)", id, g.Len(), len(pts), minFrames, time.Since(t).Seconds())

	if ply {
		p := filepath.Join(outDir, "cloud.ply")
		if err := pointcloud.WritePLY(p, pts); err != nil {
			return err
		}
		log.Printf("[%s] wrote %s", id, p)
	}

	t = time.Now()
	floor, err := geometry.EstimateFloor(pts)
	if err != nil {
		return err
	}
	log.Printf("[%s] floor: y=%.3f residual=%.1fmm tilt=%.2f° inliers=%d", id, floor.C, floor.Residual*1000, floor.TiltDeg, floor.Inliers)

	pf := geometry.PlanFrame{Floor: floor}
	theta, support := geometry.EstimateManhattan(geometry.BuildRasters(pts, pf, 0.02))
	pf.Theta = theta
	log.Printf("[%s] manhattan: θ=%.2f° (%.0f%% of linear wall cells within 3°)", id, theta*180/math.Pi, support*100)

	r := geometry.BuildRasters(pts, pf, 0.02)
	wall := r.WallMask()
	orient := geometry.Orientations(r, wall)
	// Close every wall break up to 1.6 m for segmentation; breaks of 0.5 m
	// and more are opening candidates.
	gaps := geometry.FindGaps(r, orient, 0.02, 1.6)
	var trajXY [][2]float64
	for _, f := range c.Frames {
		x, y, _ := pf.ToPlan(f.Pose.T)
		trajXY = append(trajXY, [2]float64{x, y})
	}
	tc := time.Now()
	free := geometry.Carve(c, r, 5, 4)
	log.Printf("[%s] carve: %.1fs", id, time.Since(tc).Seconds())
	labels, regions := geometry.SegmentRooms(r, wall, gaps, free, trajXY, 0.25)
	for _, reg := range regions {
		log.Printf("  room %d: %.2f m², %d trajectory samples", reg.Label, reg.Area, reg.TrajCells)
	}
	log.Printf("[%s] plan: raster %dx%d, %d gaps, %d rooms (%.1fs)", id, r.W, r.H, len(gaps), len(regions), time.Since(t).Seconds())

	dbg := filepath.Join(outDir, "debug_raster.png")
	if err := writeDebug(dbg, r, wall, orient, gaps, labels, trajXY); err != nil {
		return err
	}
	log.Printf("[%s] done in %.1fs", id, time.Since(start).Seconds())
	return nil
}
