# Repeatability, fixloop-after

## Same property, two captures (gate)
### single_scan_floor_only/plan vs single_scan_with_ceiling/plan

Registration: rotation 1×90°, translation (-6.197, 0.070) m, footprint IoU 0.857.  
Measured wall pairs: **14**, within gate (≤1 cm or ≤0.5%): **3** (21%), median |Δ| 12.7 cm.

Same corners in both plans: 1 pairs, 0 pass, median |Δ| 1.0 cm. Different corners: 13 pairs, 3 pass, median |Δ| 12.7 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R1 | R4 | 0.92 | 15.12 | 14.80 | unobserved | 2.415 |
| R2 | R3 | 0.91 | 12.82 | 12.05 | unobserved | 3.078 |
| R6 | R6 | 0.90 | 3.52 | 3.18 | unobserved | 2.259 |
| R4 | R1 | 0.81 | 10.66 | 9.54 | unobserved | 2.947 |
| R3 | R2 | 0.78 | 10.55 | 10.88 | unobserved | 3.075 |
| R5 | R5 | 0.77 | 5.76 | 7.25 | unobserved | 2.338 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R1/R4 | W1 | W10 | 0.984 | 1.110 | +12.6 | +12.80 | true | fail |
| R1/R4 | W2 | W1 | 4.731 | 4.527 | -20.4 | -4.31 | false | — |
| R1/R4 | W4 | W3 | 0.896 | 0.631 | -26.5 | -29.58 | false | — |
| R1/R4 | W5 | W4 | 1.719 | 1.476 | -24.3 | -14.14 | false | — |
| R1/R4 | W6 | W7 | 3.355 | 2.611 | -74.4 | -22.18 | false | — |
| R1/R4 | W7 | W8 | 2.825 | 2.822 | -0.3 | -0.11 | true | pass |
| R1/R4 | W8 | W9 | 0.479 | 0.525 | +4.6 | +9.60 | true | fail |
| R2/R3 | W2 | W1 | 4.606 | 4.208 | -39.8 | -8.64 | false | — |
| R2/R3 | W3 | W2 | 2.867 | 2.864 | -0.3 | -0.10 | true | pass |
| R2/R3 | W4 | W3 | 4.117 | 4.208 | +9.1 | +2.21 | false | — |
| R6/R6 | W2 | W1 | 2.224 | 1.968 | -25.6 | -11.51 | true | fail |
| R6/R6 | W3 | W2 | 1.716 | 1.720 | +0.4 | +0.23 | true | pass |
| R6/R6 | W4 | W3 | 1.534 | 1.491 | -4.3 | -2.80 | true | fail |
| R6/R6 | W5 | W4 | 0.432 | 0.422 | -1.0 | -2.31 | true | fail |
| R6/R6 | W6 | W5 | 0.690 | 0.477 | -21.3 | -30.87 | true | fail |
| R4/R1 | W2 | W6 | 5.461 | 4.045 | -141.6 | -25.93 | false | — |
| R4/R1 | W3 | W1 | 2.698 | 2.851 | +15.3 | +5.67 | true | fail |
| R4/R1 | W4 | W2 | 3.109 | 2.982 | -12.7 | -4.08 | true | fail |
| R4/R1 | W5 | W3 | 1.733 | 1.874 | +14.1 | +8.14 | true | fail |
| R4/R1 | W6 | W4 | 2.352 | 1.063 | -128.9 | -54.80 | false | — |
| R3/R2 | W1 | W4 | 1.024 | 0.776 | -24.8 | -24.22 | true | fail |
| R3/R2 | W2 | W5 | 2.117 | 1.729 | -38.8 | -18.33 | true | fail |
| R3/R2 | W4 | W1 | 2.590 | 3.027 | +43.7 | +16.87 | false | — |
| R5/R5 | W2 | W1 | 2.381 | 3.000 | +61.9 | +26.00 | false | — |
| R5/R5 | W3 | W2 | 2.421 | 2.418 | -0.3 | -0.12 | false | — |
| R5/R5 | W4 | W3 | 2.381 | 3.000 | +61.9 | +26.00 | false | — |

## Same property, two captures, drift correction off (ablation)
### single_scan_floor_only/plan_drift_off vs single_scan_with_ceiling/plan_drift_off

Registration: rotation 1×90°, translation (-5.961, -0.080) m, footprint IoU 0.829.  
Measured wall pairs: **13**, within gate (≤1 cm or ≤0.5%): **1** (8%), median |Δ| 18.7 cm.

Same corners in both plans: 1 pairs, 1 pass, median |Δ| 0.6 cm. Different corners: 12 pairs, 0 pass, median |Δ| 22.1 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R1 | R5 | 0.94 | 13.54 | 13.31 | unobserved | 2.416 |
| R3 | R1 | 0.88 | 9.71 | 9.93 | unobserved | 2.956 |
| R2 | R4 | 0.88 | 13.14 | 11.73 | unobserved | 3.078 |
| R6 | R7 | 0.76 | 3.51 | 3.48 | unobserved | 2.269 |
| R4 | R3 | 0.74 | 9.20 | 11.27 | unobserved | 3.077 |
| R5 | R6 | 0.67 | 5.68 | 7.15 | unobserved | 2.345 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R1/R5 | W1 | W8 | 1.058 | 1.134 | +7.6 | +7.18 | true | fail |
| R1/R5 | W2 | W1 | 3.777 | 3.888 | +11.1 | +2.94 | true | fail |
| R1/R5 | W3 | W2 | 3.887 | 3.504 | -38.3 | -9.85 | false | — |
| R1/R5 | W4 | W5 | 3.375 | 2.190 | -118.5 | -35.11 | true | fail |
| R1/R5 | W5 | W6 | 2.829 | 2.787 | -4.2 | -1.48 | true | fail |
| R1/R5 | W6 | W7 | 0.402 | 0.520 | +11.8 | +29.35 | true | fail |
| R3/R1 | W1 | W6 | 1.144 | 1.110 | -3.4 | -2.97 | false | — |
| R3/R1 | W2 | W1 | 3.974 | 3.917 | -5.7 | -1.43 | false | — |
| R3/R1 | W3 | W2 | 2.804 | 2.991 | +18.7 | +6.67 | true | fail |
| R3/R1 | W4 | W3 | 3.112 | 2.969 | -14.3 | -4.60 | true | fail |
| R3/R1 | W5 | W4 | 1.660 | 1.881 | +22.1 | +13.31 | true | fail |
| R3/R1 | W6 | W5 | 0.861 | 0.948 | +8.7 | +10.10 | false | — |
| R2/R4 | W1 | W1 | 4.604 | 3.999 | -60.5 | -13.14 | false | — |
| R2/R4 | W3 | W5 | 4.604 | 4.348 | -25.6 | -5.56 | true | fail |
| R2/R4 | W4 | W6 | 2.855 | 2.861 | +0.6 | +0.21 | true | pass |
| R6/R7 | W1 | W6 | 1.235 | 1.464 | +22.9 | +18.54 | true | fail |
| R6/R7 | W4 | W3 | 1.802 | 1.489 | -31.3 | -17.37 | false | — |
| R6/R7 | W5 | W4 | 0.451 | 0.429 | -2.2 | -4.88 | false | — |
| R6/R7 | W6 | W5 | 0.382 | 0.451 | +6.9 | +18.06 | false | — |
| R4/R3 | W1 | W6 | 0.738 | 1.020 | +28.2 | +38.21 | true | fail |
| R4/R3 | W2 | W7 | 2.222 | 1.728 | -49.4 | -22.23 | true | fail |
| R4/R3 | W4 | W1 | 2.484 | 3.014 | +53.0 | +21.34 | false | — |
| R4/R3 | W5 | W2 | 2.811 | 2.475 | -33.6 | -11.95 | false | — |
| R4/R3 | W7 | W4 | 0.354 | 0.966 | +61.2 | +172.88 | false | — |
| R5/R6 | W2 | W1 | 2.397 | 2.954 | +55.7 | +23.24 | false | — |
| R5/R6 | W4 | W3 | 2.397 | 2.954 | +55.7 | +23.24 | false | — |

Unmatched rooms: A [], B [R2]

## single_room against the same rooms in single_scan_with_ceiling
### single_room/plan vs single_scan_with_ceiling/plan

Registration: rotation 0×90°, translation (1.026, 6.632) m, footprint IoU 0.339.  
Measured wall pairs: **2**, within gate (≤1 cm or ≤0.5%): **0** (0%), median |Δ| 11.0 cm.

Same corners in both plans: 0 pairs, 0 pass, median |Δ| 0.0 cm. Different corners: 2 pairs, 0 pass, median |Δ| 11.0 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R1 | R5 | 0.91 | 7.74 | 7.25 | unobserved | 2.338 |
| R2 | R2 | 0.65 | 7.54 | 10.88 | unobserved | 3.075 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R1/R5 | W1 | W1 | 3.061 | 3.000 | -6.1 | -1.99 | false | — |
| R1/R5 | W2 | W2 | 2.528 | 2.418 | -11.0 | -4.35 | false | — |
| R1/R5 | W3 | W3 | 3.061 | 3.000 | -6.1 | -1.99 | false | — |
| R1/R5 | W4 | W4 | 2.528 | 2.418 | -11.0 | -4.35 | true | fail |
| R2/R2 | W1 | W1 | 3.008 | 3.027 | +1.9 | +0.63 | true | fail |
| R2/R2 | W2 | W2 | 2.506 | 3.152 | +64.6 | +25.78 | false | — |
| R2/R2 | W4 | W6 | 2.506 | 2.376 | -13.0 | -5.19 | false | — |

Unmatched rooms: A [R3], B [R1 R3 R4 R6]

## Same capture processed twice (single_scan_with_ceiling)
### single_scan_with_ceiling/plan vs single_scan_with_ceiling/plan

Registration: rotation 0×90°, translation (0.000, 0.000) m, footprint IoU 1.000.  
Measured wall pairs: **20**, within gate (≤1 cm or ≤0.5%): **20** (100%), median |Δ| 0.0 cm.

Same corners in both plans: 20 pairs, 20 pass, median |Δ| 0.0 cm. Different corners: 0 pairs, 0 pass, median |Δ| 0.0 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R1 | R1 | 1.00 | 9.54 | 9.54 | 2.947 | 2.947 |
| R2 | R2 | 1.00 | 10.88 | 10.88 | 3.075 | 3.075 |
| R3 | R3 | 1.00 | 12.05 | 12.05 | 3.078 | 3.078 |
| R4 | R4 | 1.00 | 14.80 | 14.80 | 2.415 | 2.415 |
| R5 | R5 | 1.00 | 7.25 | 7.25 | 2.338 | 2.338 |
| R6 | R6 | 1.00 | 3.18 | 3.18 | 2.259 | 2.259 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R1/R1 | W1 | W1 | 2.851 | 2.851 | +0.0 | +0.00 | true | pass |
| R1/R1 | W2 | W2 | 2.982 | 2.982 | +0.0 | +0.00 | true | pass |
| R1/R1 | W3 | W3 | 1.874 | 1.874 | +0.0 | +0.00 | true | pass |
| R1/R1 | W4 | W4 | 1.063 | 1.063 | +0.0 | +0.00 | false | — |
| R1/R1 | W5 | W5 | 0.977 | 0.977 | +0.0 | +0.00 | false | — |
| R1/R1 | W6 | W6 | 4.045 | 4.045 | +0.0 | +0.00 | false | — |
| R2/R2 | W1 | W1 | 3.027 | 3.027 | +0.0 | +0.00 | true | pass |
| R2/R2 | W2 | W2 | 3.152 | 3.152 | +0.0 | +0.00 | true | pass |
| R2/R2 | W3 | W3 | 4.756 | 4.756 | +0.0 | +0.00 | true | pass |
| R2/R2 | W4 | W4 | 0.776 | 0.776 | +0.0 | +0.00 | true | pass |
| R2/R2 | W5 | W5 | 1.729 | 1.729 | +0.0 | +0.00 | true | pass |
| R2/R2 | W6 | W6 | 2.376 | 2.376 | +0.0 | +0.00 | true | pass |
| R3/R3 | W1 | W1 | 4.208 | 4.208 | +0.0 | +0.00 | false | — |
| R3/R3 | W2 | W2 | 2.864 | 2.864 | +0.0 | +0.00 | true | pass |
| R3/R3 | W3 | W3 | 4.208 | 4.208 | +0.0 | +0.00 | false | — |
| R3/R3 | W4 | W4 | 2.864 | 2.864 | +0.0 | +0.00 | false | — |
| R4/R4 | W1 | W1 | 4.527 | 4.527 | +0.0 | +0.00 | false | — |
| R4/R4 | W10 | W10 | 1.110 | 1.110 | +0.0 | +0.00 | true | pass |
| R4/R4 | W2 | W2 | 2.038 | 2.038 | +0.0 | +0.00 | false | — |
| R4/R4 | W3 | W3 | 0.631 | 0.631 | +0.0 | +0.00 | false | — |
| R4/R4 | W4 | W4 | 1.476 | 1.476 | +0.0 | +0.00 | false | — |
| R4/R4 | W5 | W5 | 0.760 | 0.760 | +0.0 | +0.00 | false | — |
| R4/R4 | W6 | W6 | 0.418 | 0.418 | +0.0 | +0.00 | false | — |
| R4/R4 | W7 | W7 | 2.611 | 2.611 | +0.0 | +0.00 | false | — |
| R4/R4 | W8 | W8 | 2.822 | 2.822 | +0.0 | +0.00 | true | pass |
| R4/R4 | W9 | W9 | 0.525 | 0.525 | +0.0 | +0.00 | true | pass |
| R5/R5 | W1 | W1 | 3.000 | 3.000 | +0.0 | +0.00 | false | — |
| R5/R5 | W2 | W2 | 2.418 | 2.418 | +0.0 | +0.00 | false | — |
| R5/R5 | W3 | W3 | 3.000 | 3.000 | +0.0 | +0.00 | false | — |
| R5/R5 | W4 | W4 | 2.418 | 2.418 | +0.0 | +0.00 | true | pass |
| R6/R6 | W1 | W1 | 1.968 | 1.968 | +0.0 | +0.00 | true | pass |
| R6/R6 | W2 | W2 | 1.720 | 1.720 | +0.0 | +0.00 | true | pass |
| R6/R6 | W3 | W3 | 1.491 | 1.491 | +0.0 | +0.00 | true | pass |
| R6/R6 | W4 | W4 | 0.422 | 0.422 | +0.0 | +0.00 | true | pass |
| R6/R6 | W5 | W5 | 0.477 | 0.477 | +0.0 | +0.00 | true | pass |
| R6/R6 | W6 | W6 | 1.298 | 1.298 | +0.0 | +0.00 | true | pass |

