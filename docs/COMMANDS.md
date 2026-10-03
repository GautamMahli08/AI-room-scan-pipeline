# Commands: what to run, and what each one is for

All commands are run from the repository folder. On Windows, use **Git Bash**. The programs are then `bin/scan.exe` and `bin/bench.exe`, and the Python interpreter is `ml/.venv/Scripts/python.exe`. On Linux/macOS they are `bin/scan`, `bin/bench` and `ml/.venv/bin/python`.

## Quick recipes

| I want to… | Command |
|---|---|
| Set up a new machine, LiDAR only (about 1 minute) | `scripts/setup.sh --lidar` |
| Set up everything, incl. video/photo (downloads ~7.5 GB of models) | `scripts/setup.sh` |
| Make a floor plan from a LiDAR scan | `bin/scan.exe run single_room` |
| Make a floor plan from a phone video | `bin/scan.exe run captures/rooms.MOV` |
| Make a floor plan from photo folders | `bin/scan.exe run my_photos/ -tier photo` |
| Also get the debug picture | add `-debug` to any `scan run` |
| Rebuild all sample results and the repeatability tables | `scripts/run_samples.sh` |
| Rebuild the fix-loop before/after | `scripts/fixloop.sh` |
| Check that the code still works | `go test ./...` |

Results always go to `results/<capture name>/` (video and photo runs to `results/<name>_video/` or `results/<name>_photo/`).

## 1. Setup

| Command | What it does |
|---|---|
| `scripts/setup.sh --lidar` | Builds `bin/scan` and `bin/bench` and runs the Go tests. Enough for the LiDAR tier. Needs Go ≥ 1.25 and ffmpeg. |
| `scripts/setup.sh` | The above, plus creates the Python environment `ml/.venv` (with [uv](https://docs.astral.sh/uv/)) and downloads every model, so later runs work offline. |
| `ml/.venv/Scripts/python.exe scripts/fetch_weights.py` | Downloads the models only (VGGT-1B, Depth Pro, Depth Anything V2, OWLv2). `setup.sh` already runs it. |
| `go build -o bin/scan.exe ./cmd/scan` | Rebuilds the scan program after a code change (`./cmd/bench` for bench). |

## 2. `scan run`: make a floor plan

```
bin/scan.exe run <capture> [flags]
```

`<capture>` is one of:

- a **LiDAR scan folder** (Stray Scanner export, e.g. `single_room`); with `-tier video` only its video is used, as if no LiDAR existed
- a **video file** (`.mp4`, `.mov`, `.m4v`)
- a **folder of photo folders**, one sub-folder per room

The tier is detected from the input. `-tier` forces it.

| Flag | Default | What it does |
|---|---|---|
| `-tier lidar\|video\|photo` | detected | Which input type to treat the capture as |
| `-out <dir>` | `results` | Where result folders are written |
| `-debug` | off | Also write `debug_raster.png`, the picture of what the pipeline saw |
| `-drift` | on | Drift correction. Both plans are written: `plan.*` (corrected) and `plan_drift_off.*`. Turn off with `-drift=false` |
| `-damage` | on | Damage detection on the capture's video (LiDAR tier; needs the Python environment). Turn off with `-damage=false` |
| `-calib` | on | Adds the calibrated interval term. `-calib=false` gives model-only intervals, needed as input for `bench calibrate` |
| `-live` | off | Video/photo/damage results are cached; this forces the models to run again |
| `-ply` | off | Also write the 3D point cloud as `cloud.ply` (open in MeshLab or CloudCompare) |
| `-stride <n>` | 1 | Use every n-th frame (faster, less detail) |
| `-voxel <m>` | 0.02 | Point-cloud cell size in metres (video/photo use 0.04 automatically) |
| `-min-frames <n>` | 2 | Drop points seen by fewer frames (video/photo use 1 automatically) |
| `-cpuprofile <file>` | – | Write a CPU profile (performance work only) |

**What a run writes** (`results/<capture>/`):

| File | What it is |
|---|---|
| `plan.json` | Rooms, walls, openings, ceiling heights and areas, each with a 90% interval. Checked against `schema/plan.schema.json` |
| `plan.svg` | The dimensioned floor plan drawing, with damage markers |
| `plan_drift_off.json`, `.svg` | The same plan without drift correction (ablation) |
| `drift.json` | How much drift was corrected, and where |
| `debug_raster.png` | Only with `-debug` |
| `damage/detections.json` | Damage found by the model (cached) |
| `export/` | Video/photo only: the model's output, in LiDAR-scan format (cached) |

Examples:

```bash
bin/scan.exe run single_scan_with_ceiling -debug          # LiDAR, with debug picture
bin/scan.exe run single_room -damage=false -drift=false    # fastest LiDAR run
bin/scan.exe run single_room -tier video                 # only the scan's video (rgb.mp4) is used -> results/single_room_video
bin/scan.exe run captures/ceiling.MOV -debug -live         # your own video, models re-run
```

## 3. `bench`: measure quality

| Command | What it does |
|---|---|
| `bin/bench.exe repeat A/plan.json B/plan.json` | Compares two plans of the same place wall by wall: the repeatability table (markdown). Add more pairs to compare several at once |
| `bin/bench.exe repeat -json A/plan.json B/plan.json` | Same, as JSON (input for the `repeat_*.py` scripts) |
| `bin/bench.exe overlay A/plan.json B/plan.json out.svg` | Draws both plans on top of each other, aligned, to see where they differ |
| `bin/bench.exe calibrate A/plan.json B/plan.json …` | Fits the interval terms from repeat captures and prints JSON (saved as `internal/calib/empirical_lidar.json`). Inputs must be made with `scan run … -calib=false` |

## 4. Scripts that regenerate the evidence

| Command | What it does |
|---|---|
| `scripts/run_samples.sh` | Runs `scan` on the three sample captures and writes all LiDAR results plus `bench/repeatability.md`. Set `DATA=/path` if the samples are elsewhere |
| `scripts/fixloop.sh` | Builds the `fixloop-before` and `fixloop-after` versions in temporary worktrees, runs both on every sample and writes the before/after bundle in `fixloop/` |
| `scripts/make_photo_sets.py <scan_dir> <lidar_plan.json> <out_dir>` | Builds the photo-tier test sets: picks a few frames per room from a LiDAR capture, as a person would take photos |

Run the Python scripts with `ml/.venv/Scripts/python.exe scripts/<name>.py …`.

## 5. Pictures for the docs

| Command | What it makes |
|---|---|
| `scripts/explain_raster.py <debug_raster.png> <out.png> [title]` | The labelled version of a debug picture: every colour explained |
| `scripts/show_detection.py <capture_dir> <results_dir> <class> <out.jpg> [n]` | The strongest damage detections of one class (e.g. `crack`) drawn on their frames, for checking by eye |
| `scripts/draw_hld.py`, `scripts/draw_architecture.py`, `scripts/draw_as_built.py` | Optional diagrams (design, code architecture, what is built with status), written to `docs/img/`. The README uses the hand-drawn `Architecture-Full Picure.png` instead |

## 6. Diagnostics (used to find causes; not needed to get results)

| Command | What it answers |
|---|---|
| `scripts/repeat_decompose.py <bench.json>` | Is a repeatability difference from the wall faces, or from the outline ending at a different wall? |
| `scripts/repeat_diagnose.py <bench.json>` | Root-cause evidence for the fix loop: pass rate per wall-end type |
| `scripts/tau_view.py <uncal_results_dir>` | How large an interval each wall pair needs to be covered |
| `scripts/traj_eval.py <export_dir> <scan_dir>` | How far the video/photo camera path is from the true (ARKit) path |
| `scripts/photo_eval.py <export_dir> <photo_root> <manifest.json> <scan_dir>` | Same for the photo tier, per photo |
| `ml/vggt_chunk_test.py`, `ml/vggt_keyframe_test.py` | Is the video error from VGGT itself or from chaining chunks? |
| `ml/vggt_memtest.py <frames_dir>` | How many frames fit on this GPU at once |
| `ml/crack_probe.py <video> <frame> <rot>` | Crack scores on one frame, whole image vs tiles |

## 7. Called by `scan`, not by hand

`ml/video_recon.py`, `ml/photo_recon.py` and `ml/damage_detect.py` are started by `scan run` as subprocesses (`ml/vggt_runner.py` is their shared VGGT loader). They can be run directly for debugging; each prints its usage with `--help`.
