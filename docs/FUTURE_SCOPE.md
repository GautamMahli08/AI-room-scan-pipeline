# Future scope

What is deliberately not built yet, why, and how it would be done. Items are ordered by expected impact on the case-study gates. The measured status of everything that *is* built is in [COMPLIANCE.md](COMPLIANCE.md) and [BENCHMARK.md](BENCHMARK.md).

## 1. Accuracy of the video and photo tiers (highest impact)

| Item | Current state | Approach | Expected effect |
|---|---|---|---|
| **Global alignment of the video walk** | VGGT chunks are chained one after another; error accumulates to ~70 cm and 18% scale over 14.6 m | Recognise revisited places (image retrieval across chunks), add chunk-to-chunk similarity constraints, and solve a pose graph over all chunks. This is the video counterpart of the LiDAR drift correction already built | Removes accumulated drift; walls stop smearing and can be fitted |
| **Wall fitting from model depth** | From a few views of model depth, walls are not detected, so rooms fall back to rectangles over the floor seen (−48% area) | Fit vertical planes directly in 3D (RANSAC on the fused cloud) instead of from the 2D wall raster, which needs dense, thin wall evidence | Real outlines instead of fallback rectangles |
| **Separate calibration per tier** | Video/photo intervals reuse the LiDAR terms plus a scale term; the fallback interval factors come from only 2 cases | Repeat captures at each tier, then the same split-conformal fit as for LiDAR (`bench calibrate`) | Validated intervals at every tier |
| **Photo-tier stitching** | 2 of 4 rooms linked on the sample-derived set | Photos taken to the protocol (wide shots, doorway photos from both sides) rather than frames cut from a scanning video, plus a layout solver that also uses wall alignment and no-overlap constraints | Whole-property plan from photos |

## 2. LiDAR tier

| Item | Current state | Approach |
|---|---|---|
| **Soft surfaces (curtains, blinds)** | The fix-loop post-mortem: a curtain in front of a window wall was measured as the wall in one capture, giving 13–15 cm errors | When the densest layer is smeared (spread > 5 cm, coverage < 50%), search further out for a crisp layer, and widen that face's interval |
| **Open-plan room boundaries** | Boundaries between open-plan spaces differ between captures; this is the main cost to repeatability | Use ceiling-height changes and floor-material changes as extra cues for where one space ends |
| **Non-Manhattan walls** | Angled walls are detected but approximated by axis-aligned steps in the outline | Add angled wall lines to the arrangement (they are already labelled `OrientOther`) |
| **Ceiling repeatability** | Measured in one capture only | Needs a second capture that scans the ceiling; the code already reports per-room heights and other ceiling levels |
| **Mirrors** | Points behind a wall plane are excluded from rooms; mirrors are not explicitly detected | Flag large planar regions whose reflection produces a mirrored copy of the room behind a wall |
| **Multi-storey properties** | One floor per capture | Split by floor height, then stitch floors by the stair core |

## 3. Damage

| Item | Current state | Approach |
|---|---|---|
| **Benchmark with real or staged damage** | The samples contain one real defect (a bathroom crack): found in 1 capture, mislabelled in 1, missed in 1 | Stage at least two damage classes in a room, as the brief specifies, and measure recall and false-alarm rate per class |
| **Masks instead of boxes** | Boxes from OWLv2; extent is the 10–90% range of the box's points | Segment with SAM 2 inside each box for exact area |
| **Class confusion** | The same crack was labelled a water stain in one capture | A second-stage classifier on the cropped region, and agreement across views on the class |
| **Video and photo tiers** | Damage runs at the LiDAR tier only (it needs depth to place regions) | Use the tier's model depth for placement, with wider extent intervals |
| **More rules** | 3 concealed-damage rules | Rules that need a room graph: a ceiling stain under a wet room, mould on an exterior wall |

## 4. Not in the brief, but useful

| Item | Note |
|---|---|
| **Furniture on the plan** | Furniture is already detected (everything between the floor and 2 m that is not wall: the grey areas in `debug_raster.png`) and used internally (floor under furniture counts as room; damage on furniture is rejected). It is not drawn on `plan.svg` because the brief does not ask for it. Next steps: draw the footprints with their height; name them (bed, sofa, wardrobe) with an object detector |
| **Room labels** | Rooms are R1, R2, …; the photo tier keeps folder names. Name rooms (kitchen, bathroom) from fixtures seen |
| **Own capture app (Route 1)** | A TestFlight app with ARKit/RoomPlan would remove the export and transfer step from the capture protocol |
| **Speed** | The LiDAR tier takes 7–56 s on CPU; the video tier takes ~9 min on a 4 GB GPU. Larger VGGT batches on a bigger GPU would cut the video time several-fold |

## 5. Evaluation gaps (needs access to rooms and devices)

| Item | What is needed |
|---|---|
| **Laser ground truth** | A laser measurer in the benchmark rooms: walls, openings, ceiling heights |
| **Head-to-head (Part 3)** | The same rooms scanned with Polycam or magicplan on a LiDAR iPhone, and the export compared dimension by dimension with `bench` |
| **Walk-in readiness on a native iPhone file** | The loaders handle HEIC/MOV and EXIF orientation, but have not been run on a file straight from an iPhone camera |
