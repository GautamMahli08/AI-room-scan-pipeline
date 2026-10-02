# Device matrix

Which tier runs on which phone, what it reads, and the accuracy it honestly delivers. Accuracy figures are measured on the three sample captures (no laser ground truth exists for them; see [BENCHMARK.md](BENCHMARK.md) for the method). Where a number is not yet measured, the table says so.

| Tier | Phones | Capture app | What the pipeline uses | Accuracy delivered (samples) | Status |
|---|---|---|---|---|---|
| **LiDAR** | iPhone 12 Pro / Pro Max or newer Pro models, iPad Pro with LiDAR | Stray Scanner (App Store, free) | LiDAR depth 256×192 at 60 Hz, ARKit poses and intrinsics, RGB video | Wall faces repeat to **1.1 cm** (median, within a room) across two captures; identical input gives an identical plan; walls with the same corners in two captures agree to 0.1–1.7 cm. Cross-capture wall *lengths* fail the 1 cm gate (21% pass, median 12.7 cm) because outlines differ between captures ([fixloop/RESULT.md](../fixloop/RESULT.md)). Ceiling measured in one sample (2.27–3.08 m by room). | Complete |
| **Video** | Any iPhone 15 or newer (also any phone video) | Built-in Camera, Video, 1× | RGB frames only (3 fps, sharpest of each pair); no depth, poses or intrinsics | Metric scale within about 18% on `single_room`; trajectory error 70 cm median over a 14 m walk; walls are not fitted, so rooms are outlined from free space (all walls inferred); footprint −23% vs LiDAR. **Not adequate for the ±3% gate.** | Runs end to end; accuracy not adequate |
| **Photo** | Any iPhone 15 or newer (any camera) | Built-in Camera, Photo, 1× | 2–8 photos per room folder; EXIF orientation and focal length when present | One room: cameras within 16 cm and scale within 8% of ARKit, but walls are not fitted (all inferred); footprint +42% vs LiDAR. **Not adequate for the ±8% gate.** | Runs end to end, incl. doorway linking; accuracy not adequate |

## Compute

| | LiDAR tier | Video / photo tiers |
|---|---|---|
| Runs on | Any machine with Go (CPU only) | NVIDIA GPU with ≥ 4 GB (tested: RTX 3050 Ti Laptop, 4 GB); CPU works but is much slower |
| Time on the samples | geometry 7–56 s per capture (incl. drift correction); damage detection +20–100 s (GPU) | single_room video: 9 min; 6-room photo set: 19 min (GPU memory bound) |
| Models | OWLv2 for damage detection (optional; skipped with a note if the ML environment is absent) | VGGT-1B (aggregator in fp16), Apple Depth Pro, Depth Anything V2 Metric-Indoor (orientation only) |

## Why the tiers differ

LiDAR measures depth directly, and ARKit fuses the IMU with vision for metric poses, so scale is known. Video and photos have neither. Poses and depth come from VGGT, and metric scale from a single-image metric depth model, whose bias depends on the focal length it assumes (see [TECHNICAL_REPORT.md](TECHNICAL_REPORT.md) §4). That scale uncertainty is added to every video/photo interval, which is why intervals widen as sensor data thins.
