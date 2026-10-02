# Benchmark report

Every number here is regenerated from the raw sample captures by `scripts/run_samples.sh` (LiDAR tier, repeatability, drift) and `scripts/fixloop.sh` (fix loop). Video/photo diagnostics come from `scripts/traj_eval.py` and `ml/vggt_*_test.py`.

## The benchmark set, and what it cannot contain

The case study asks for a self-built benchmark: a multi-room capture, a room with staged damage, all rooms at all three tiers, one room captured twice, and laser ground truth. **As agreed with the hiring team, this submission uses only the three sample captures they provided**, and no LiDAR iPhone was available to add captures. What that allows:

| Required | Available here |
|---|---|
| Multi-room capture (≥ 3 rooms + connector) | ✅ `single_scan_floor_only` and `single_scan_with_ceiling` are whole-property scans (6 rooms + corridor) |
| Same room captured twice at the same tier | ✅ The property is scanned twice; `single_room` overlaps it a third time |
| Same rooms at all three tiers | ✅ Video and photo inputs are derived from the same captures (frames from `rgb.mp4`; photo folders built by `scripts/make_photo_sets.py`) |
| Laser / tape ground truth | ❌ None exists. Reference = the LiDAR result, independent repeat captures, and synthetic scenes with exact ground truth |
| Staged damage (two classes) | ❌ No damage in the samples; damage detection is not implemented |
| Head-to-head vs a consumer app (Part 3) | ❌ Needs the same rooms scanned with Polycam/magicplan; the rooms and a LiDAR device were not available. Not attempted, rather than faked. |

## Gates: LiDAR tier

| Gate | Requirement | Result | Verdict |
|---|---|---|---|
| Repeatability | Two captures agree within 1 cm or 0.5% per wall | Same capture processed twice: **20/20** walls identical. Two independent captures: **3/14 (21%)** measured wall pairs within the gate, median \|Δ\| 12.7 cm. Walls whose corners match in both plans agree to 0.1–1.7 cm | **Fail** (cross-capture). Diagnosed in the [fix loop](../fixloop/RESULT.md): corner topology plus a curtain-vs-wall scene difference |
| Ceiling height | ≤ 1.5 cm per room; spread ≤ 1 cm across captures | Measured in all 6 rooms of `single_scan_with_ceiling`: 2.95, 3.08, 3.08, 2.42, 2.34, 2.26 m, model intervals ±1.2 cm. The other two captures never scanned the ceiling and report it as **unobserved** (no number) | **Not measurable**: no ground truth, and only one capture saw the ceiling, so repeatable-vs-biased cannot be decided |
| Opening widths | ≤ 2 cm on ≥ 85%, misses and phantoms counted | Synthetic room: the 0.900 m door is measured as **0.9000 m**. Samples: 13 openings (`floor_only`) and 16 (`with_ceiling`) detected and classified door/opening/window | **Not measurable** on samples (no ground truth) |
| Drift accountability | Stated method, footprint with correction on and off | Plane-anchored chunk correction with held-out prior selection ([SYSTEM_DESIGN §4.3](../SYSTEM_DESIGN.md)); table below | **Done** |

### Drift ablation (correction on vs off)

| Capture | Path | Wall scatter uncorrected → corrected | Held-out scatter, uncorrected → corrected (iter 1) | Largest correction | Footprint off → on |
|---|---|---|---|---|---|
| `single_room` | 14.6 m | 16.5 → 11.2 mm | 15.1 → 8.4 mm | 6.3 cm, 0.38° | 25.02 → 25.02 m² |
| `single_scan_floor_only` | 54.3 m | 20.7 → 14.1 mm | 24.6 → 19.2 mm | 23.8 cm, 4.57° | 54.78 → 58.44 m² |
| `single_scan_with_ceiling` | 99.9 m | 20.7 → 16.1 mm | 21.0 → 15.9 mm | 6.2 cm, 0.57° | 57.47 → 57.72 m² |

Repeatability of the stitched footprint: the two property scans differ by 4.8% with correction off (54.78 vs 57.47 m²) and by **1.2% with it on** (58.44 vs 57.72 m²). Wall pairs within the gate rise from 1/13 (off) to 3/14 (on), and footprint overlap (IoU) from 0.829 to 0.857. The `floor_only` correction is large (24 cm, 4.6°) and is the least trusted. It is selected on held-out data, but by a smaller margin.

### Calibration (LiDAR tier)

Without ground truth, intervals are calibrated against independent repeat captures (split-conformal style, [internal/calib](../internal/calib/empirical.go)). For two independent estimates of the same wall, A − B should fall inside the combined 90% interval 90% of the time.

| | Stated coverage | Achieved (cross-capture wall pairs) |
|---|---|---|
| Measurement model only (face-fit noise) | 90% | **29%** (4/14): overconfident |
| + empirical term (lengths τ = 16.7 cm, areas τ = 15.4%) | 90% | **93%** in sample (13/14); **88%** leave-one-out (lengths, 17 pairs), 89% (areas, 9 pairs) |

The resulting wall intervals are about ±27 cm (90%), which is honest about what two captures actually agree on. Ceiling heights have no repeat pairs and keep model intervals; they are stated as uncalibrated. The footprint interval reuses the per-room relative term and is conservative (the two property scans agree within 1.2%).

### Synthetic ground truth (unit tests, exact)

`internal/geometry/geometry_test.go`: a 4.00 × 3.00 m room rotated 20° with a 0.90 m door runs through the whole chain. Walls come out as 4.0000 / 3.0000 m, the door as 0.9000 m and the Manhattan angle as 20.000°; the tilted floor plane is recovered to 2e-4. This checks the geometry is exact when the data is.

## Gates: video and photo tiers

| Gate | Requirement | Result | Verdict |
|---|---|---|---|
| Video wall lengths | ±3% | No walls survive fusion on `single_room`. The camera trajectory scored against the capture's ARKit poses (diagnostic only) gives metric scale within 18% and 70 cm median position error over 14.6 m. Single 8-frame VGGT chunks are accurate to 2–5 cm, so the error is accumulated chaining. | **Fail** |
| Photo wall lengths | ±8% with calibrated intervals | Single-room photo geometry is distorted (fan-shaped surfaces) on sample-derived photo sets | **Fail** |
| Photo whole-property stitch | One plan, correct adjacency, no overlaps, footprint ±8% | Doorway linking and spanning-tree layout run; 4 of 6 rooms linked on the first photo set, but placement is wrong because per-room geometry is wrong | **Fail** |

Scale diagnostics that drove the design (metric depth vs LiDAR depth on the same `single_room` frames):

| Metric depth source | Ratio to LiDAR depth |
|---|---|
| Depth Anything V2 Metric-Indoor | 1.50–2.04× (upright), varies by frame |
| Apple Depth Pro, its own focal | 1.23–1.39× (one outlier 2.74×) |
| Depth Pro rescaled to the true focal | **0.97–1.12×** |

## Timing

| | `single_room` | `single_scan_floor_only` | `single_scan_with_ceiling` |
|---|---|---|---|
| Frames / path | 1715 / 14.6 m | 5251 / 54.3 m | 9745 / 99.9 m |
| LiDAR tier, full run incl. 2 drift iterations | 7 s | 16 s | 40–80 s |
| Video tier (RTX 3050 Ti 4 GB) | 9 min | – | – |
| Photo tier, 6-room set (RTX 3050 Ti 4 GB) | – | – | 19 min |

CPU: laptop, Go pipeline parallel across cores. The GPU stages are bound by its 4 GB of memory: VGGT batches over 8 frames spill to shared memory and slow down 3–4×.
