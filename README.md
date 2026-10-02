# Room scanning pipeline

Handheld phone capture in, dimensioned floor plan out. One command per capture, at three input tiers (LiDAR, video, photos). Every tier writes the same JSON contract ([schema/plan.schema.json](schema/plan.schema.json)) and an SVG plan. Every measurement carries a 90% interval.

| Start here | |
|---|---|
| What is built, where, and its status | [docs/COMPLIANCE.md](docs/COMPLIANCE.md) |
| How to capture (one page, for non-engineers) | [docs/CAPTURE_PROTOCOL.md](docs/CAPTURE_PROTOCOL.md) |
| Which tier runs on which phone, and how accurately | [docs/DEVICE_MATRIX.md](docs/DEVICE_MATRIX.md) |
| Results on the sample captures | [docs/BENCHMARK.md](docs/BENCHMARK.md) |
| Technical report | [docs/TECHNICAL_REPORT.md](docs/TECHNICAL_REPORT.md) |
| Fix loop (declaration, fix, before/after, post-mortem) | [fixloop/README.md](fixloop/README.md) |
| Architecture | [SYSTEM_DESIGN.md](SYSTEM_DESIGN.md) |

## Quick start

Requirements: Go ≥ 1.25, ffmpeg on `PATH`, git. The video and photo tiers also need [uv](https://docs.astral.sh/uv/), which fetches Python 3.12 itself; an NVIDIA GPU is used if present (4 GB is enough).

```bash
git clone <repo> && cd <repo>
scripts/setup.sh --lidar        # LiDAR tier only: builds and tests, about 1 minute
scripts/setup.sh                # all tiers: adds the Python env and ~7.5 GB of model weights
```

On Windows, run the scripts from Git Bash. Binaries are `bin/scan.exe` and `bin/bench.exe`.

## One command per capture

```bash
bin/scan run <stray_scanner_capture>                 # LiDAR tier (Stray Scanner export)
bin/scan run <video.mov|.mp4>                        # video tier (any phone video)
bin/scan run <capture> -tier video                   # video tier from a Stray Scanner capture's rgb.mp4 only
bin/scan run <folder_of_room_folders> -tier photo    # photo tier: one sub-folder of 2-8 photos per room
```

The tier is detected from the input when `-tier` is omitted. Output goes to `results/<name>/` (non-LiDAR tiers: `results/<name>_<tier>/`):

| File | What it is |
|---|---|
| `plan.json` | The output contract, schema-validated on every run |
| `plan.svg` | Dimensioned floor plan; open in any browser |
| `plan_drift_off.json/.svg` | The same plan with drift correction off (ablation) |
| `drift.json` | Drift correction: per-chunk corrections, held-out scores, footprint on/off |
| `export/` | Video/photo tiers: the model outputs in LiDAR layout (cached; `-live` recomputes) |
| `damage/detections.json` | Raw damage detections (cached; `-live` recomputes) |

Useful flags: `-debug` (top-down raster with rooms, walls and openings), `-ply` (fused point cloud), `-drift=false`, `-damage=false`, `-live` (re-run model inference, ignoring the cache).

## Regenerate every reported number

```bash
scripts/run_samples.sh      # LiDAR plans for the three samples + bench/repeatability.md
scripts/fixloop.sh          # fix-loop before/after, each built from its git tag
```

The samples (`single_room/`, `single_scan_floor_only/`, `single_scan_with_ceiling/`) are expected in the repo root, or set `DATA=/path`. They are not committed because of their size. Model weights are fetched by `scripts/fetch_weights.py` and are never committed. Nothing calls any service of ours; Hugging Face is used only to download public weights.

## Status by tier (honest)

| Tier | State | On the samples |
|---|---|---|
| **LiDAR** | Complete: rooms, walls, openings, ceilings, adjacency, drift correction with ablation, intervals | Runs on all three samples: geometry 7–56 s on CPU, plus 20–100 s for damage detection on GPU; see [BENCHMARK](docs/BENCHMARK.md) |
| **Video** | Runs end to end (VGGT + Depth Pro → LiDAR pipeline) | Not yet usable: metric scale within ~18%, trajectory bends over a long walk, walls do not survive fusion |
| **Photo** | Runs end to end, including doorway linking and stitching | Not yet usable: single-room geometry is distorted |
| **Damage** | OWLv2 detection on video frames → projected onto walls/floor/ceiling through LiDAR depth → merged across views → concealed-damage rules → scope items (LiDAR tier) | The samples have no damage: 226 raw detections across the three captures, **0 reported** (precision filters); the positive path is covered by synthetic tests |

The video and photo tiers fail honestly. They report wide intervals, or no rooms, rather than confident wrong numbers.

## Repository layout

```
cmd/scan, cmd/bench        one command per capture; benchmark tables
internal/ingest            Stray Scanner loader (also reads video/photo exports)
internal/geometry          fusion, floor, Manhattan frame, walls, openings, rooms, ceilings
internal/stitch            plane-anchored drift correction
internal/calib             measurement-model intervals
internal/output            JSON (schema-validated) and SVG
internal/bench             cross-capture registration and repeatability
internal/damage            damage placement on surfaces, multi-view merge, rules, scope
ml/                        Python: VGGT, Depth Pro (video and photo tiers), OWLv2 (damage)
schema/                    published output schema
scripts/                   setup, weights, sample runs, fix loop, diagnostics
fixloop/                   Part 4 bundle
```
