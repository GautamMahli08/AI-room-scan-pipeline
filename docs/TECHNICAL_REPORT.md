# Technical report

*Room scanning from handheld phone capture: LiDAR, video and photo tiers. Maximum 6 pages.*

## 1. Summary

One command turns a phone capture into a dimensioned floor plan that follows a published JSON schema. It also renders an SVG plan, and every measurement carries a 90% interval. The **LiDAR tier is complete**:

- It finds rooms, walls, doors, windows and ceilings, and stitches the multi-room plan with adjacency.
- It corrects drift and writes the on/off ablation.
- Its intervals are empirically calibrated.

It runs on all three sample captures in 7–80 s on a laptop CPU, and the same input always gives the same plan.

**The video and photo tiers run end to end but are not yet accurate** on the samples. The reasons are measured and stated in §4.

The fix loop targeted the worst gate, cross-capture repeatability. It made the pipeline deterministic, but it did not move the cross-capture number. The post-mortem in §6 says why.

All evaluation uses the **three sample captures from the hiring team**. Two of them turned out to be whole-property scans of the same flat, and the third covers part of it. That gives real multi-room, repeat-capture and drift test data, but no laser ground truth.

## 2. Architecture

```
capture ──► ingest ──► fusion ──► plan geometry ──► drift correction ──► assemble ──► plan.json + plan.svg
 (tier)      Frame[]    voxels     floor, Manhattan,    plane-anchored,      intervals    (schema-validated)
                                   walls, gaps, rooms,  re-extract           (model +
                                   outlines, ceilings,                       empirical)
                                   openings, adjacency
```

**Go owns everything geometric:** about 5k lines, a single binary, CPU only. **Python is used only for model inference** in the video and photo tiers. Those tiers write a Stray Scanner–style export (depth PNGs, per-frame poses and intrinsics, a `meta.json` with the scale uncertainty), so all three tiers share one geometry pipeline and one output contract. Model outputs are cached keyed on the input files and replay deterministically; `-live` recomputes.

**Plan extraction (LiDAR):**

1. **Fusion.** Back-project confidence-2 depth with the OpenCV camera convention; ARKit's convention inverts the room, and a test on the sample guards this. Fuse into 2 cm voxels counting the distinct frames that see each one, and drop voxels seen once. Sums are integer micrometres, so the result doesn't depend on merge order.
2. **Floor.** Take the strongest low horizontal slab and refine it with a least-squares plane, so heights are measured from the fitted plane. This exposed a 0.45° pose tilt in `single_room` that the same flat's longer scan does not have.
3. **Manhattan frame.** A histogram of local line angles in the wall raster, refined on the wall points to 0.01° by maximising histogram sharpness. The raster estimate alone was 0.5° off, which smears a 4 m wall over 3 cm.
4. **Walls and gaps.** Wall cells fill at least half of the 0.3–2.0 m band. They are labelled H/V by run length, which works on 10 cm thick walls where local covariance fails. Gaps between collinear wall runs are scored for a header (2.05–2.35 m), a sill, floor seen through them, and whether the camera passed through.
5. **Rooms.** Barriers are walls plus every gap of 1.6 m or less, so rooms do not leak through doors or windows. Interior is observed floor or furniture plus space carved free by depth rays, which recovers the floor under the user. Rooms are the components the camera walked through. Open-plan regions are split at narrow passages by morphological erosion with a persistence test (Bormann et al. 2016).
6. **Outlines.** Dominant H/V wall lines cut the plan into rectangles; each rectangle belongs to the room covering at least half of it, and a room is the union of its rectangles. Edges therefore follow real walls, and open-plan boundaries follow wall extensions. Each edge then snaps to the densest wall-point layer on the room's side, refined to millimetres. Weakly supported edges are marked `inferred`. Coplanar jogs are merged, and features under 0.30 m are flattened (the fix-loop change).
7. **Ceilings.** Per room, the horizontal layer above 1.9 m that covers most of the room, with other levels noted. If none covers at least 20%, the ceiling is reported as **unobserved**, with no number.
8. **Openings.** Gaps attached to walls get widths from the jamb points and are classified door, opening or window; windows must face outside. Open-plan boundaries become `opening`s, so adjacency always cites an opening.

**Testing.** A synthetic 4.00 × 3.00 m room rotated 20° with a 0.90 m door runs through the whole chain and comes back exactly. Other tests cover the distance transform against brute force, the boundary tracer, gap scoring, deterministic fusion with 1/3/8 workers, the drift transform against its linear model, schema rejection cases, and the ingest of the real sample.

## 3. Drift handling

The walk is cut into 4 s chunks. Each chunk gets a rigid correction (plan translation, yaw, vertical shift), interpolated in time. The landmarks are the measured wall faces (in 0.5 m segments) and the floor. A chunk observes a room's walls only from frames whose camera is inside that room, so the far face of a thick wall is never associated.

One linear least-squares problem solves all chunk corrections and landmark positions together, with a random-walk prior between neighbouring chunks; chunk 0 is the gauge. **Loop closure** comes from rooms re-entered late in the walk re-observing the same faces. This replaced the planned ICP, because landmark re-observation needs no raw-point overlap. The solve runs twice, re-fusing and re-fitting between iterations.

Because the corrections have many degrees of freedom, the prior strength is **chosen on held-out data**. Odd wall segments are never fitted, and a correction is applied only if it beats no correction there by 5%; otherwise the poses are kept and the report says so. Results:

| Capture | Wall scatter, uncorrected → corrected | Held-out (iteration 1) |
|---|---|---|
| `single_room` | 16.5 → 11.2 mm | 15.1 → 8.4 mm |
| `single_scan_floor_only` | 20.7 → 14.1 mm | 24.6 → 19.2 mm |
| `single_scan_with_ceiling` | 20.7 → 16.1 mm | 21.0 → 15.9 mm |

**Ablation:** the two property scans' footprints differ by 4.8% with correction off and **1.2% with it on**. Most chunks share a 0.5–0.9° heading offset relative to the first 4 s of the walk, which is consistent with ARKit's heading still settling at the start of a session.

## 4. Tier design and device matrix

| Tier | Device | Poses | Depth | Metric scale | State |
|---|---|---|---|---|---|
| LiDAR | iPhone/iPad Pro (Stray Scanner) | ARKit VIO + drift correction | LiDAR | measured | complete |
| Video | any phone video | VGGT-1B on 8-frame chunks, chained by similarity | VGGT | Depth Pro, rescaled by VGGT's focal | runs; not accurate |
| Photo | any camera, 2–8 photos per room | VGGT per room; rooms linked by doorway photos (SIFT, then joint VGGT) and laid out by a maximum spanning tree | VGGT | Depth Pro (EXIF focal when present) | runs; not accurate |

Full matrix: [DEVICE_MATRIX.md](DEVICE_MATRIX.md). Design decisions came from measurements against the samples, using the capture's ARKit poses and LiDAR depth **only as diagnostics** (`scripts/traj_eval.py`):

- **Incremental SfM fails on room scans.** COLMAP registered 49 of 223 frames, because people pan in place and pure rotation has no baseline. VGGT predicts depth and poses together and needs none.
- **VGGT fits a 4 GB GPU** with its aggregator in fp16 and its heads in fp32. Portrait frames must be padded to 518×518; otherwise VGGT assumes equal horizontal and vertical fields of view, giving fx 357 vs fy 470 for a true 432.
- **Single-image metric depth is focal-dependent.** Depth Anything V2 Metric-Indoor read 1.5–2.0× deep, and Depth Pro with its own focal 1.23–1.39×. Depth Pro rescaled to the true focal reads **0.97–1.12×**.
- **Within an 8-frame chunk VGGT is accurate:** 2–5 cm over 40 cm of motion. But it normalises scale per chunk (0.9–2.6×), so chunks are chained by similarity. Over a 14.6 m walk, chaining accumulates to **70 cm median error with metric scale off by 18%**, and walls do not survive fusion.
- **Orientation:** the sample videos are stored sideways. Straight lines decide the vertical axis and depth ordering the sign; this is correct on all three samples.

**What would fix the video tier** (not done): global alignment of the chunks, via loop closure between revisited places and a pose graph over chunk-to-chunk similarity transforms. That is the same machinery the photo stitch needs.

**Photo tier:** per-room geometry from 5–8 wide photos is still distorted on the sample-derived sets, which include a mirror and night windows. The photo sets were built from the video by `scripts/make_photo_sets.py`, which uses the LiDAR plan only to decide which frames a person would have taken.

## 5. Error budget and calibration

**Error budget, LiDAR wall length** (1σ):

| Source | Size | Evidence |
|---|---|---|
| Face fit noise (points per face, spread) | < 1 mm | thousands of points per face |
| LiDAR systematic per surface | ~5 mm | model floor (`calib.LiDARFaceSys`) |
| Pose drift within a room | ~10–15 mm | drift scatter after correction |
| Face repeatability across captures (within a room) | 11 mm median | fix-loop evidence 2 |
| **Corner placement / outline topology** | **10–40 cm, tail to 2 m** | fix-loop decomposition: 4 of 14 pairs end at different walls |
| Scene differences (curtains, clutter) | 13–15 cm in one room | layer diagnostics |

The measurement model alone covers the first two rows. That made its intervals **overconfident: only 29% of independent captures fell inside the combined 90% interval.** The empirical calibration ([internal/calib/empirical.go](../internal/calib/empirical.go)) fits the missing per-measurement σ (τ) from cross-capture pairs, taken from uncalibrated plans so nothing is counted twice:

- **Lengths:** τ = 16.7 cm.
- **Areas:** τ = 15.4% relative.

Both are added in quadrature. Coverage is now **93%** in sample and **88% leave-one-out** for lengths (89% for areas). With 17 length pairs, the finite-sample conformal rank would be the single worst pair (τ ≈ 1 m), so the plain empirical quantile is used and its leave-one-out coverage is reported instead.

Video and photo intervals add their measured scale σ (9–16%) to the LiDAR terms; they are **not separately calibrated**, because no repeat captures exist at those tiers. Ceiling intervals (±1.2 cm) are model-only, because only one capture saw the ceiling.

## 6. The fix loop

**Declared** ([fixloop/DECLARATION.md](../fixloop/DECLARATION.md), committed before any fix code):

- **Worst gate:** cross-capture repeatability, 2/14 wall pairs within the gate, median 13.6 cm.
- **Hypothesis:** outline decisions are brittle. The evidence was that identical input gave different plans: footprint 2.3% apart, one wall moving 17 cm.
- **Fix:** determinism, small-feature removal, and a wider drift-prior search.
- **Predicted:** identical reruns would give identical plans; median 13.6 → about 5 cm; within-gate 14 → about 25%.

**Result** ([fixloop/RESULT.md](../fixloop/RESULT.md)):

- **Same-input repeatability:** 8/14 → **20/20**, with identical plans. Prediction met.
- **Cross-capture:** **no meaningful movement** (21% after, against a before distribution of 7–36% over 6 runs), and the median prediction was **badly wrong** (12.7 cm).

**Post-mortem.** The hypothesis was right about brittleness but wrong that brittleness dominated the error between captures. After the fix, three of the ten remaining same-topology errors come from one bedroom whose window wall is a smeared, half-covered layer in one capture (a curtain) and a crisp full wall in the other. That is a **scene difference**, not instability.

Two earlier hypotheses were tested and rejected before declaring:
- "Topology alone": 0 of 7 pass even when both ends sit on full walls.
- "Wrong layer picked": taking the outermost layer made face agreement worse (3.9 → 4.5 cm).

## 7. Known failure modes

| Condition | Effect | Handling | Status |
|---|---|---|---|
| Curtains, soft furnishings in front of walls | Face fit picks the curtain: 13–15 cm, differs between captures | Diagnosed (smeared layer, under 50% coverage); planned fix: search outward for a crisp layer, widen the interval | open |
| Mirrors | LiDAR: points behind the wall plane fall outside rooms (barrier). Photo/video: a phantom room, distorted geometry | Capture protocol says pass mirrors at an angle | open for photo/video |
| Glass doors and windows | Sparse returns, gap in the wall | Windows need wall below; gaps with nothing seen are `unobserved_gap` notes, not openings | handled |
| Unscanned ceiling | – | `observed: false` with a reason, never a guess | handled |
| Open-plan spaces | Room boundaries are choices; they differ between captures | Split at narrow passages; boundaries along wall extensions, reported as inferred | partial (main repeatability cost) |
| Low light | LiDAR unaffected; video/photo models degrade | Protocol: lights on | stated |
| Rays through windows into outdoor space | Leak strips | Gaps of 1.6 m or less closed as barriers; 15 cm morphological opening | handled |
| Pose drift, heading settling at session start | 1–2 cm wall scatter; ~0.7° start offset | Plane-anchored correction (§3) | handled |
| Sideways raw video, missing EXIF | Upside-down or sideways input to the models | Line + depth orientation cue | handled |
| 4 GB GPU | VGGT batches over 8 frames spill to shared memory | Chunks of 8; half-precision aggregator | handled (slow) |

**Damage** (LiDAR tier):

- **Detect:** OWLv2 open-vocabulary detection on one frame per ~2 s, run on upright frames. The rotation comes from the ARKit poses.
- **Filter boxes:** boxes covering more than 20% of the frame are dropped, as are boxes overlapping a distractor (plant, curtain, shadow, …) or scoring below 0.35.
- **Place:** each surviving box is projected through LiDAR depth onto the wall (within 25 cm of its face), floor or ceiling, then merged across views. Only regions seen in at least 2 frames are reported.
- **Rules:** R-WET-CEIL-01, R-WALL-BASE-01 and R-CRACK-DIAG-01 raise concealed-damage flags. Scope items are generated per class (m², m or each) and linked to their damage and flags.

The samples contain one real defect, a hairline crack on the bathroom wall. A human reviewer spotted it after the first version had missed it, and the miss drove four fixes: a hairline-specific prompt (0.20 → 0.50), denser sampling, score hysteresis, and a unit-tested fix to the box coordinate mapping.

Now the crack is found in `single_room` on the right wall with the right shape (0.11 × 0.74 m). In `floor_only` it is reported as a water stain, contaminated by marble veining. In `with_ceiling` it is missed. Rules and scope are also tested synthetically.

**Not implemented:** damage at the video and photo tiers, and the head-to-head against a consumer app (needs a LiDAR device in the same rooms).
