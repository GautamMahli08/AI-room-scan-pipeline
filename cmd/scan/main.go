// Command scan runs the room-scanning pipeline on one capture.
//
//	scan run <capture> [-tier lidar|video|photo] [-out results] [-live] [flags]
//
// Photo tier input: a folder with one sub-folder of 2-8 photos per room.
//
// <capture> is a Stray Scanner export (or a folder holding one), a video
// file, or a folder of photo folders. With -tier video on a Stray Scanner
// capture only its rgb.mp4 is used: depth, poses, intrinsics and IMU are
// withheld.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"roomscan/internal/pipeline"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: scan run <capture> [-tier lidar|video|photo] [flags]")
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
	tier := fs.String("tier", "", "input tier: lidar, video or photo (default: from the input)")
	live := fs.Bool("live", false, "re-run model inference even if a cached export matches the input")
	fs.IntVar(&opt.Stride, "stride", opt.Stride, "use every n-th frame")
	fs.Float64Var(&opt.Voxel, "voxel", opt.Voxel, "voxel size in metres")
	minFrames := fs.Int("min-frames", int(opt.MinFrames), "drop voxels seen by fewer distinct frames")
	fs.BoolVar(&opt.WritePLY, "ply", false, "write the fused point cloud as PLY")
	fs.BoolVar(&opt.Debug, "debug", false, "write debug rasters")
	fs.BoolVar(&opt.Drift, "drift", opt.Drift, "plane-anchored drift correction; the uncorrected plan is also written as the ablation")
	fs.BoolVar(&opt.Calibrate, "calib", opt.Calibrate, "add empirical interval terms (false: model sigmas only, for bench calibrate)")
	cpuprofile := fs.String("cpuprofile", "", "write a CPU profile to this file")

	// Accept the capture before or after flags.
	args := os.Args[2:]
	var input string
	if len(args) > 0 && args[0][0] != '-' {
		input, args = args[0], args[1:]
	}
	fs.Parse(args)
	if input == "" && fs.NArg() > 0 {
		input = fs.Arg(0)
	}
	if input == "" {
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
	if err := run(input, *tier, *out, *live, opt); err != nil {
		log.Print(err)
		pprof.StopCPUProfile()
		os.Exit(1)
	}
}

func run(input, tier, outRoot string, live bool, opt pipeline.Options) error {
	if tier == "" {
		tier = detectTier(input)
	}
	base := strings.TrimSuffix(filepath.Base(filepath.Clean(input)), filepath.Ext(input))
	switch tier {
	case "lidar":
		export, err := captureDir(input)
		if err != nil {
			return err
		}
		outDir := filepath.Join(outRoot, base)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		_, err = pipeline.RunLiDAR(export, outDir, base, opt)
		return err
	case "video":
		video, err := videoFile(input)
		if err != nil {
			return err
		}
		id := base + "_video"
		outDir := filepath.Join(outRoot, id)
		export := filepath.Join(outDir, "export")
		if err := pipeline.Reconstruct("video", video, export, live); err != nil {
			return err
		}
		_, err = pipeline.RunLiDAR(export, outDir, id, opt)
		return err
	case "photo":
		id := base + "_photo"
		outDir := filepath.Join(outRoot, id)
		export := filepath.Join(outDir, "export")
		if err := pipeline.Reconstruct("photo", input, export, live); err != nil {
			return err
		}
		_, err := pipeline.RunLiDAR(export, outDir, id, opt)
		return err
	}
	return fmt.Errorf("unknown tier %q", tier)
}

func detectTier(input string) string {
	if isVideo(input) {
		return "video"
	}
	if _, err := captureDir(input); err != nil && hasPhotoFolders(input) {
		return "photo"
	}
	return "lidar"
}

// hasPhotoFolders reports whether dir holds sub-folders of images (the
// photo tier's one-folder-per-room layout).
func hasPhotoFolders(dir string) bool {
	subs, _ := os.ReadDir(dir)
	for _, s := range subs {
		if !s.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(dir, s.Name()))
		for _, f := range files {
			switch strings.ToLower(filepath.Ext(f.Name())) {
			case ".jpg", ".jpeg", ".png", ".heic", ".heif":
				return true
			}
		}
	}
	return false
}

func isVideo(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".mp4", ".mov", ".m4v":
		return true
	}
	return false
}

// videoFile resolves a video file, or the rgb.mp4 of a Stray Scanner
// capture (the video tier withholds everything else in the capture).
func videoFile(input string) (string, error) {
	if isVideo(input) {
		return input, nil
	}
	if export, err := captureDir(input); err == nil {
		v := filepath.Join(export, "rgb.mp4")
		if _, err := os.Stat(v); err == nil {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s: no video found", input)
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
