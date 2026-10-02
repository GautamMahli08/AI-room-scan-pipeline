# Fix declaration: repeatability gate

Committed before any fix code. The before state is tag `fixloop-before` (commit `ce9f9ae`); diagnostics added afterwards (`bench overlay`, `bench repeat -json`, `scripts/repeat_*.py`, the `wall_layers` diagnostic) do not change any plan output.

## 1. Worst gate and the failing number

**Repeatability**, LiDAR tier: two captures of the same room must agree within 1 cm or 0.5% per wall. Pair: `single_scan_floor_only` vs `single_scan_with_ceiling` (the same property scanned twice), drift correction on.

| Measure | Before |
|---|---|
| Measured wall pairs within gate | **2 / 14 (14%)** |
| Median \|Δ length\| | **13.6 cm** |
| Same capture processed twice: pairs within gate | 13 / 14; footprint 59.10 vs 60.46 m² (2.3%) |

Every other gate that can be measured on the samples is either passing or bounded by missing ground truth. This one fails by an order of magnitude.

## 2. Root-cause hypothesis and evidence

**Hypothesis: the room outline comes out of brittle discrete decisions.** These are: the Manhattan refinement subsample, the 50% rectangle-ownership rule over a global wall-line arrangement, small notches and leak strips, and an over-flexible drift solve. Small perturbations flip which wall a given wall ends at. A second capture is a larger perturbation of the same kind. Wall *faces* are measured repeatably; wall *ends* are not.

Evidence (`scripts/repeat_decompose.py`, `scripts/repeat_diagnose.py`, `bench overlay`):

1. **Lengths split into face error and topology error, and the split is exact.** For pairs that both plans end at the same two end walls, the end walls' face shifts predict the length difference to a median of 0.1 cm. Those 9 pairs have a median \|Δ\| of 4.9 cm. The other 5 pairs end at *different* walls and have a median \|Δ\| of **54 cm**. Topology carries most of the error.
2. **Faces are repeatable within a room.** Once each matched room is registered on its own, wall faces agree to a median of **1.1 cm** (drift off) and 1.4 cm (drift on). Globally they disagree by 4–5 cm, which is warping across the property, not error within rooms.
3. **The same input does not give the same plan.** Running `single_scan_with_ceiling` twice changes the footprint by 2.3% and moves one measured wall by 17 cm. The source is non-deterministic: fusion merges worker results in completion order, and `RefineManhattan` subsamples points by index. So the Manhattan angle jitters, and the threshold decisions downstream flip.
4. **Drift correction amplifies the instability.** Identical reruns differ by 0.3% in footprint with drift off and 2.3% with it on. The held-out prior selection picked the strongest prior on its grid (×0.25) in 3 of 4 iterations, so the optimum lies outside the grid and the correction is more flexible than the data support. Within-room face disagreement rises from 1.1 cm (off) to 1.4 cm (on).
5. **Two hypotheses tested and rejected.** "Topology alone": 0 of 7 pairs pass even when both ends sit on full walls. "Wrong point layer chosen as the face": taking the outermost covered layer instead of the densest *worsens* face agreement (median 3.9 → 4.5 cm).

## 3. The fix and the predicted number

1. **Determinism.** Merge fusion results in frame order and subsample by a fixed stride over key-sorted voxels, so the same input always gives the same plan.
2. **Outline stabilisation.** Remove outline features smaller than 0.30 m (notches, slivers, leak strips) at the rectangle-arrangement level before tracing, so corners only come from structures large enough to be seen again in another capture.
3. **Drift prior.** Extend the prior search to stronger values (down to ×0.03), so the selection is no longer pinned to the edge of the grid.

**Predicted after the fix:**

| Measure | Before | Predicted |
|---|---|---|
| Same capture twice | 13/14, footprint Δ 2.3% | **14/14, identical plans** |
| Topology-mismatch pairs (cross-capture) | 5 / 14 | **≤ 3** |
| Median \|Δ length\| (cross-capture) | 13.6 cm | **~5 cm** |
| Measured pairs within gate | 2 / 14 (14%) | **~25%** (3–4 pairs) |

**The gate itself will still fail, and the reason is stated now.** Faces agree to about 1.1 cm within a room, so a length (two faces) carries about √2 × 1.1 ≈ 1.6 cm of noise. That exceeds the 1 cm / 0.5% tolerance for walls under about 3 m. This fix targets the dominant error (topology and instability), not that sensor-level floor. Passing the gate would need sub-centimetre faces, i.e. per-wall depth-bias calibration, which is a separate fix.
