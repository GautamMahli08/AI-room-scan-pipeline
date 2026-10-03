# Fix result and post-mortem: repeatability gate

Declaration: [DECLARATION.md](DECLARATION.md) (commit `ee71729`, before any fix code).
Fix: tag `fixloop-after` (commit `376c5b6`); readable diff in [fix.diff](fix.diff).
Regenerate everything: `scripts/fixloop.sh` (each tag is built in its own worktree and run on all samples; both states are scored by the same `bench` binary).

## Results

**The old pipeline was not deterministic** (that was the root cause declared), so its gate score is a distribution, not a single number. It was sampled 6 times on identical input: [before-samples/summary.txt](before-samples/summary.txt) plus the bundle run in [fixloop-before/](fixloop-before/repeatability.md). The new pipeline returns the same number every run.

| Measure | Before (6 runs, identical input) | Predicted | After | Verdict |
|---|---|---|---|---|
| Same capture processed twice: pairs within gate | 8/14 (57%); footprint differs by up to 2.3% | 100%, identical plans | **20/20 (100%), identical plans** | prediction met |
| Cross-capture: pairs within gate | 7–36%, mean ≈ 19% | ~25% | **21%** (3/14) | **no meaningful movement** |
| Cross-capture: median \|Δ length\| | 3.8–13.6 cm, typically 13.6 | ~5 cm | **12.7 cm** | **prediction badly wrong** |
| Cross-capture: pairs ending at different walls | 2–5, median \|Δ\| 30–54 cm | ≤ 3 | 4, median \|Δ\| 25 cm | missed |
| Gate (≤1 cm or 0.5% per wall) | fail | fail (stated) | **fail** | as stated |

The gate still fails, as the declaration said it would. The fix moved the *same-input* part of repeatability from fail to pass. It did **not** measurably move *cross-capture* repeatability, which is what the gate scores.

## Post-mortem: why the cross-capture prediction was wrong

**What the hypothesis got right.** The pipeline was brittle and non-deterministic, and fusion merge order was the source. Removing it made plans exactly reproducible; previously the same capture gave footprints up to 2.3% apart. Drift-prior selection is no longer pinned to the edge of its grid (it now picks ×0.125 and ×0.06).

**What it got wrong.** I assumed that the error *between captures* was mostly the same instability as the error *between runs*, only larger, so that removing the brittleness would remove most of it. Measured after the fix, it is not:

1. **Faces that agree within a room are not the norm in every room.** The prediction used the median within-room face agreement (1.1 cm). After the fix, 3 of the 10 same-topology pairs come from one bedroom whose wall faces differ by 12.7–15.3 cm between captures. Its layer diagnostics show why. In `single_scan_floor_only` the wall shows as a smeared surface: points spread over 8.5 cm at near-equal density, covering under half the wall. In `single_scan_with_ceiling` the same wall is one crisp layer covering the whole wall, 13.8 cm further out. That is a curtain in front of the window wall in one capture: a **difference in the scene, not in the pipeline**. No amount of stability fixes it.
2. **Topology mismatches are partly real too.** The 4 remaining different-end-wall pairs come from the two scans splitting open-plan space differently. Each capture's segmentation is now stable, but the two captures still see different free space.
3. **Removing small features moved some corners the wrong way.** Flattening 0.30 m features removed notches that differed between captures, but it also moved a few corners that had matched. Net topology mismatches: 5 → 4, not ≤ 3.

**What the evidence supports next** (not shipped; recorded for the next iteration):

- **Soft-surface rejection.** A face layer whose points smear over more than about 5 cm with under 50% coverage is not a wall. Search further outward (up to about 25 cm) for a crisp, well-covered layer, and if none is found, mark the face uncertain and widen its interval. Expected effect: the bedroom's three pairs drop from 13–15 cm to a few cm, putting the median near 5 cm.
- **Intervals that admit it.** Until then, smeared faces should carry wide intervals. At present they get the same ±1 cm as crisp faces, which is the "confident garbage" the brief penalises.

## What changed (summary of fix.diff)

| File | Change |
|---|---|
| `internal/geometry/pointcloud/pointcloud.go` | Voxel sums in integer micrometres; points returned in key order |
| `internal/stitch/drift.go` | Observation sums in integer micrometres; prior search ×0.03–×8 |
| `internal/geometry/walls.go` | `removeSmallFeatures`: flatten U-shaped outline features under 0.30 m |
| tests | `TestFuseDeterministic` (1/3/8 workers give identical clouds), `TestRemoveSmallFeatures` |
