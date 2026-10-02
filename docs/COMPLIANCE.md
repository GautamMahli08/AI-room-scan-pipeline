# Compliance matrix

Requirement → where it lives → what to look at → status. Status key: ✅ done · ⚠️ partial (what is missing is stated) · ❌ not done (the reason is stated).

## Part 1: capture route and input tiers

| Requirement | File path | Artifact | Status |
|---|---|---|---|
| Capture route (Route 2: stock apps, one-page protocol) | [docs/CAPTURE_PROTOCOL.md](CAPTURE_PROTOCOL.md) | One page: install, walk, duration, what to avoid, file hand-over | ✅ |
| Device matrix: tier × hardware × honest accuracy | [docs/DEVICE_MATRIX.md](DEVICE_MATRIX.md) | Table with measured accuracy per tier | ✅ |
| LiDAR tier: depth, poses, intrinsics | [internal/ingest/strayscanner](../internal/ingest/strayscanner/strayscanner.go), [internal/pipeline/lidar.go](../internal/pipeline/lidar.go) | `bin/scan run <capture>` | ✅ |
| Video tier: handheld clip, any iPhone 15+ | [ml/video_recon.py](../ml/video_recon.py), [cmd/scan](../cmd/scan/main.go) | `bin/scan run <video>` | ⚠️ runs end to end; accuracy not adequate on the samples ([BENCHMARK](BENCHMARK.md)) |
| Photo tier: 2–8 stills per room, folder per room, stitched plan | [ml/photo_recon.py](../ml/photo_recon.py) | `bin/scan run <rooms_dir> -tier photo` | ⚠️ runs incl. doorway linking and layout; per-room geometry distorted on sample-derived photos |
| Same output contract from every tier, intervals widen as data thins | [internal/pipeline/lidar.go](../internal/pipeline/lidar.go) (`assemble`, `measure`) | Same schema; video/photo add their scale σ to every interval | ✅ |
| Native iPhone inputs (HEIC/JPEG, MOV/MP4, EXIF orientation) | [ml/photo_recon.py](../ml/photo_recon.py) `load_photo`, [ml/video_recon.py](../ml/video_recon.py) | pillow-heif, EXIF transpose, 35 mm focal used for scale; sideways raw video auto-rotated | ✅ (not tested on a native iPhone file: no device) |

## Part 2: output contract

| Requirement | File path | Artifact | Status |
|---|---|---|---|
| Per-room plan: walls, ceiling height, floor area, openings | [internal/geometry](../internal/geometry), [internal/output/plan.go](../internal/output/plan.go) | `results/<capture>/plan.json` → `rooms[]` | ✅ |
| Stitched multi-room plan with correct adjacency | [internal/geometry/rooms.go](../internal/geometry/rooms.go), [openings.go](../internal/geometry/openings.go) | `rooms[]` in one frame + `adjacency[]` via openings / open boundaries | ✅ LiDAR · ⚠️ photo/video |
| Per-surface damage regions with class and metric extent | [schema/plan.schema.json](../schema/plan.schema.json) `damage` | Schema and output field exist (always empty) | ❌ detection not implemented |
| Concealed-damage flags with the rule that fired | schema `concealed_flags`, rule IDs in [SYSTEM_DESIGN §7](../SYSTEM_DESIGN.md) | Field exists (empty) | ❌ depends on damage detection |
| Scope line items keyed to surfaces | schema `scope` | Field exists (empty) | ❌ depends on damage detection |
| A confidence interval on every measurement | [internal/calib](../internal/calib) | Every `*_m` / `*_m2` value has `ci90`; empirically calibrated for lengths and areas (88–93% coverage) | ✅ |
| One command per capture | [cmd/scan](../cmd/scan/main.go) | `bin/scan run <capture>` | ✅ |
| JSON to the published schema | [schema/plan.schema.json](../schema/plan.schema.json), [internal/output/validate.go](../internal/output/validate.go) | Every run is validated; invalid output is an error | ✅ |
| Rendered plan | [internal/output/svg.go](../internal/output/svg.go) | `plan.svg` (dimensions, doors, windows, areas, ceilings) | ✅ |

## Part 2: gates

| Gate | Where measured | Status |
|---|---|---|
| Opening widths ≤ 2 cm on ≥ 85% | [BENCHMARK §LiDAR](BENCHMARK.md); synthetic test exact (0.9000 m) | ⚠️ not measurable on samples (no ground truth) |
| Ceiling ≤ 1.5 cm; spread ≤ 1 cm | [BENCHMARK](BENCHMARK.md) | ⚠️ measured in 1 capture; spread not measurable |
| Repeatability: 1 cm or 0.5% per wall | [bench/repeatability.md](../bench/repeatability.md), [cmd/bench](../cmd/bench/main.go) | ❌ fails cross-capture (21%); same-input 100%. Fix loop target |
| Drift accountability + on/off ablation | [internal/stitch/drift.go](../internal/stitch/drift.go), `plan_drift_off.*`, `drift.json` | ✅ |
| Photo whole-property stitch | [ml/photo_recon.py](../ml/photo_recon.py) | ❌ implemented, not accurate |
| Photo ±8%, video ±3%, calibrated at every tier | [BENCHMARK](BENCHMARK.md) | ❌ not met; video/photo use LiDAR calibration plus scale σ |

## Part 3: head-to-head

| Requirement | Status |
|---|---|
| Our LiDAR output vs a consumer app on 2 benchmark rooms | ❌ Needs the same rooms scanned with Polycam/magicplan on a LiDAR device; the sample rooms were not accessible and no LiDAR iPhone was available. Not faked. |

## Part 4: fix loop

| Requirement | File path | Status |
|---|---|---|
| Declaration: worst gate + number, root cause + evidence, fix + prediction | [fixloop/DECLARATION.md](../fixloop/DECLARATION.md) (committed before the fix, `1af94b6`) | ✅ |
| Shipped fix with readable diff | tag `fixloop-after`, [fixloop/fix.diff](../fixloop/fix.diff) | ✅ |
| Before and after runs, regenerable | [scripts/fixloop.sh](../scripts/fixloop.sh), `fixloop/fixloop-{before,after}/` | ✅ |
| Post-mortem | [fixloop/RESULT.md](../fixloop/RESULT.md) | ✅ (same-input gate fixed; cross-capture prediction missed, explained) |

## Part 5: process evidence

| Requirement | Status |
|---|---|
| Commit as you work | ✅ `git log`: ~30 commits, each a working step, from design doc to calibration |

## Deliverables

| # | Deliverable | Where | Status |
|---|---|---|---|
| 1 | Compliance matrix | this file | ✅ |
| 2 | Capture route + device matrix | [CAPTURE_PROTOCOL](CAPTURE_PROTOCOL.md), [DEVICE_MATRIX](DEVICE_MATRIX.md) | ✅ |
| 3 | Repo, README, one command per capture, clean machine < 15 min | [README](../README.md), [scripts/setup.sh](../scripts/setup.sh) | ✅ LiDAR (~1 min) · ⚠️ all tiers: weights download (~7.5 GB) depends on bandwidth |
| 4 | Reproduction bundle | [scripts/run_samples.sh](../scripts/run_samples.sh), [scripts/fixloop.sh](../scripts/fixloop.sh), [scripts/fetch_weights.py](../scripts/fetch_weights.py); model outputs cached under `results/*/export/` and replayed when the input is unchanged, `-live` recomputes | ✅ |
| 5 | Benchmark report: gates at all tiers, repeatability, head-to-head, timing | [docs/BENCHMARK.md](BENCHMARK.md) | ⚠️ head-to-head missing (Part 3) |
| 6 | Fix loop bundle | [fixloop/](../fixloop/README.md) | ✅ |
| 7 | Technical report ≤ 6 pages | [docs/TECHNICAL_REPORT.md](TECHNICAL_REPORT.md) | ✅ |
| 8 | Raw benchmark data | The three sample captures (provided by the hiring team; not committed for size) and the derived photo sets (regenerated by `scripts/make_photo_sets.py`) | ⚠️ no ground-truth measurements or app exports exist |

## Constraints

| Constraint | Status |
|---|---|
| Handheld consumer capture only | ✅ |
| Pretrained models disclosed | ✅ VGGT-1B (Meta), Depth Pro (Apple), Depth Anything V2 Metric-Indoor; listed in every plan's `provenance.models` |
| Runs without calling our infrastructure | ✅ only public weight downloads |
| Weights fetched by script | ✅ [scripts/fetch_weights.py](../scripts/fetch_weights.py) |
| Mirrors, glass, wet-look surfaces, low light covered | ⚠️ Handled: unobserved gaps are not reported as openings; ceilings not seen are "unobserved"; windows need wall below; soft surfaces (curtains) found in the fix loop. Not yet handled: mirror phantom rooms in photo/video ([TECHNICAL_REPORT §7](TECHNICAL_REPORT.md)) |
