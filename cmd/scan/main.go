// Command scan runs the room-scanning pipeline on one capture.
//
//	scan run <capture_dir> [-out results] [-stride 1] [-voxel 0.02] [-ply] [-debug]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	"roomscan/internal/pipeline"
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
	opt := pipeline.DefaultOptions()
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	out := fs.String("out", "results", "output root directory")
	fs.IntVar(&opt.Stride, "stride", opt.Stride, "use every n-th frame")
	fs.Float64Var(&opt.Voxel, "voxel", opt.Voxel, "voxel size in metres")
	minFrames := fs.Int("min-frames", int(opt.MinFrames), "drop voxels seen by fewer distinct frames")
	fs.BoolVar(&opt.WritePLY, "ply", false, "write the fused point cloud as PLY")
	fs.BoolVar(&opt.Debug, "debug", false, "write debug rasters")
	cpuprofile := fs.String("cpuprofile", "", "write a CPU profile to this file")

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
	opt.MinFrames = int32(*minFrames)
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	export, err := captureDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	id := filepath.Base(filepath.Clean(dir))
	outDir := filepath.Join(*out, id)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	if _, err := pipeline.RunLiDAR(export, outDir, id, opt); err != nil {
		log.Print(err)
		pprof.StopCPUProfile()
		os.Exit(1)
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
