# Repeatability, fixloop-before

## Same property, two captures (gate)
### single_scan_floor_only/plan vs single_scan_with_ceiling/plan

Registration: rotation 1×90°, translation (-6.202, 0.111) m, footprint IoU 0.853.  
Measured wall pairs: **11**, within gate (≤1 cm or ≤0.5%): **4** (36%), median |Δ| 3.8 cm.

Same corners in both plans: 2 pairs, 2 pass, median |Δ| 0.6 cm. Different corners: 9 pairs, 2 pass, median |Δ| 10.4 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R1 | R1 | 0.91 | 13.21 | 13.06 | unobserved | 2.397 |
| R5 | R3 | 0.87 | 10.96 | 12.25 | unobserved | 2.944 |
| R2 | R2 | 0.84 | 14.23 | 12.25 | unobserved | 3.084 |
| R3 | R4 | 0.83 | 10.13 | 10.64 | unobserved | 3.073 |
| R4 | R5 | 0.67 | 4.98 | 7.26 | unobserved | 2.335 |
| R6 | R6 | 0.52 | 5.78 | 3.46 | unobserved | 2.257 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R1/R1 | W1 | W12 | 1.055 | 1.159 | +10.4 | +9.86 | true | fail |
| R1/R1 | W2 | W1 | 3.824 | 3.608 | -21.6 | -5.65 | false | — |
| R1/R1 | W3 | W4 | 3.440 | 1.315 | -212.5 | -61.77 | false | — |
| R1/R1 | W4 | W7 | 0.453 | 0.490 | +3.7 | +8.17 | false | — |
| R1/R1 | W6 | W9 | 2.899 | 2.611 | -28.8 | -9.93 | false | — |
| R1/R1 | W7 | W10 | 2.792 | 2.802 | +1.0 | +0.36 | true | pass |
| R1/R1 | W8 | W11 | 0.473 | 0.511 | +3.8 | +8.03 | true | fail |
| R5/R3 | W2 | W7 | 1.061 | 2.321 | +126.0 | +118.76 | false | — |
| R5/R3 | W4 | W1 | 3.678 | 3.898 | +22.0 | +5.98 | false | — |
| R5/R3 | W5 | W2 | 3.002 | 3.008 | +0.6 | +0.20 | true | pass |
| R5/R3 | W6 | W3 | 3.110 | 2.977 | -13.3 | -4.28 | true | fail |
| R5/R3 | W7 | W4 | 1.909 | 1.884 | -2.5 | -1.31 | true | fail |
| R5/R3 | W8 | W5 | 1.629 | 3.243 | +161.4 | +99.08 | false | — |
| R2/R2 | W1 | W9 | 0.489 | 0.486 | -0.3 | -0.61 | false | — |
| R2/R2 | W12 | W8 | 2.148 | 0.917 | -123.1 | -57.31 | false | — |
| R2/R2 | W2 | W10 | 2.148 | 2.149 | +0.1 | +0.05 | true | pass |
| R2/R2 | W3 | W1 | 4.068 | 4.053 | -1.5 | -0.37 | false | — |
| R2/R2 | W4 | W2 | 2.064 | 2.113 | +4.9 | +2.37 | false | — |
| R2/R2 | W5 | W3 | 0.312 | 0.244 | -6.8 | -21.79 | false | — |
| R2/R2 | W6 | W4 | 0.810 | 0.746 | -6.4 | -7.90 | false | — |
| R2/R2 | W7 | W5 | 3.370 | 4.142 | +77.2 | +22.91 | false | — |
| R3/R4 | W10 | W7 | 0.183 | 1.714 | +153.1 | +836.61 | false | — |
| R3/R4 | W2 | W9 | 2.131 | 1.708 | -42.3 | -19.85 | true | fail |
| R3/R4 | W4 | W1 | 2.555 | 3.001 | +44.6 | +17.46 | false | — |
| R3/R4 | W8 | W5 | 1.928 | 2.189 | +26.1 | +13.54 | false | — |
| R3/R4 | W9 | W6 | 0.287 | 0.295 | +0.8 | +2.79 | false | — |
| R4/R5 | W2 | W1 | 2.054 | 2.953 | +89.9 | +43.77 | false | — |
| R4/R5 | W4 | W3 | 2.054 | 1.926 | -12.8 | -6.23 | false | — |
| R6/R6 | W10 | W3 | 0.890 | 1.488 | +59.8 | +67.19 | false | — |
| R6/R6 | W11 | W4 | 0.436 | 0.433 | -0.3 | -0.69 | true | pass |
| R6/R6 | W12 | W5 | 0.586 | 0.446 | -14.0 | -23.89 | true | fail |
| R6/R6 | W2 | W1 | 2.231 | 1.935 | -29.6 | -13.27 | true | fail |
| R6/R6 | W3 | W2 | 1.282 | 1.890 | +60.8 | +47.43 | false | — |

## Same property, two captures, drift correction off (ablation)
### single_scan_floor_only/plan_drift_off vs single_scan_with_ceiling/plan_drift_off

Registration: rotation 1×90°, translation (-5.979, -0.053) m, footprint IoU 0.852.  
Measured wall pairs: **10**, within gate (≤1 cm or ≤0.5%): **0** (0%), median |Δ| 23.1 cm.

Same corners in both plans: 0 pairs, 0 pass, median |Δ| 0.0 cm. Different corners: 10 pairs, 0 pass, median |Δ| 23.1 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R3 | R1 | 0.89 | 10.15 | 10.11 | unobserved | 2.956 |
| R2 | R3 | 0.86 | 14.37 | 12.29 | unobserved | 3.078 |
| R1 | R4 | 0.82 | 13.54 | 14.10 | unobserved | 2.418 |
| R4 | R2 | 0.79 | 10.08 | 11.06 | unobserved | 3.077 |
| R6 | R6 | 0.78 | 3.51 | 3.55 | unobserved | 2.269 |
| R5 | R5 | 0.69 | 5.68 | 7.27 | unobserved | 2.345 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R3/R1 | W1 | W8 | 1.144 | 0.170 | -97.4 | -85.14 | false | — |
| R3/R1 | W2 | W1 | 3.980 | 3.836 | -14.4 | -3.62 | false | — |
| R3/R1 | W3 | W2 | 3.047 | 2.993 | -5.4 | -1.77 | true | fail |
| R3/R1 | W4 | W3 | 1.916 | 2.970 | +105.4 | +55.01 | false | — |
| R3/R1 | W7 | W4 | 1.619 | 1.850 | +23.1 | +14.27 | true | fail |
| R3/R1 | W8 | W5 | 0.861 | 1.100 | +23.9 | +27.76 | false | — |
| R2/R3 | W3 | W6 | 2.073 | 2.097 | +2.4 | +1.16 | false | — |
| R2/R3 | W4 | W1 | 4.608 | 4.336 | -27.2 | -5.90 | true | fail |
| R2/R3 | W6 | W3 | 2.655 | 4.199 | +154.4 | +58.15 | false | — |
| R1/R4 | W1 | W10 | 1.058 | 1.135 | +7.7 | +7.28 | true | fail |
| R1/R4 | W2 | W1 | 3.777 | 4.495 | +71.8 | +19.01 | false | — |
| R1/R4 | W4 | W7 | 3.375 | 2.196 | -117.9 | -34.93 | true | fail |
| R1/R4 | W5 | W8 | 2.829 | 2.787 | -4.2 | -1.48 | true | fail |
| R1/R4 | W6 | W9 | 0.402 | 0.521 | +11.9 | +29.60 | true | fail |
| R4/R2 | W1 | W6 | 1.160 | 1.017 | -14.3 | -12.33 | true | fail |
| R4/R2 | W10 | W5 | 0.160 | 3.856 | +369.6 | +2310.00 | false | — |
| R4/R2 | W2 | W7 | 2.225 | 1.729 | -49.6 | -22.29 | true | fail |
| R4/R2 | W4 | W1 | 2.484 | 2.935 | +45.1 | +18.16 | false | — |
| R4/R2 | W5 | W2 | 2.811 | 2.473 | -33.8 | -12.02 | false | — |
| R4/R2 | W7 | W4 | 0.389 | 0.963 | +57.4 | +147.56 | false | — |
| R6/R6 | W1 | W8 | 1.235 | 1.467 | +23.2 | +18.79 | true | fail |
| R6/R6 | W3 | W4 | 1.686 | 0.424 | -126.2 | -74.85 | false | — |
| R6/R6 | W4 | W5 | 1.802 | 1.651 | -15.1 | -8.38 | false | — |
| R6/R6 | W5 | W6 | 0.451 | 0.429 | -2.2 | -4.88 | false | — |
| R6/R6 | W6 | W7 | 0.382 | 0.450 | +6.8 | +17.80 | false | — |
| R5/R5 | W2 | W1 | 2.397 | 2.953 | +55.6 | +23.20 | false | — |
| R5/R5 | W4 | W3 | 2.397 | 1.950 | -44.7 | -18.65 | false | — |

## single_room against the same rooms in single_scan_with_ceiling
### single_room/plan vs single_scan_with_ceiling/plan

Registration: rotation 0×90°, translation (1.069, 6.586) m, footprint IoU 0.383.  
Measured wall pairs: **1**, within gate (≤1 cm or ≤0.5%): **0** (0%), median |Δ| 4.7 cm.

Same corners in both plans: 1 pairs, 0 pass, median |Δ| 4.7 cm. Different corners: 0 pairs, 0 pass, median |Δ| 0.0 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R3 | R5 | 0.94 | 7.80 | 7.26 | unobserved | 2.335 |
| R1 | R4 | 0.60 | 9.14 | 10.64 | unobserved | 3.073 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R3/R5 | W1 | W1 | 3.045 | 2.953 | -9.2 | -3.02 | false | — |
| R3/R5 | W2 | W2 | 2.561 | 2.430 | -13.1 | -5.12 | false | — |
| R3/R5 | W3 | W5 | 3.045 | 1.027 | -201.8 | -66.27 | false | — |
| R3/R5 | W4 | W6 | 2.561 | 2.514 | -4.7 | -1.84 | true | fail |
| R1/R4 | W1 | W1 | 3.597 | 3.001 | -59.6 | -16.57 | false | — |
| R1/R4 | W3 | W3 | 3.597 | 0.806 | -279.1 | -77.59 | false | — |
| R1/R4 | W4 | W10 | 2.542 | 2.432 | -11.0 | -4.33 | false | — |

Unmatched rooms: A [R2], B [R1 R2 R3 R6]

## Same capture processed twice (single_scan_with_ceiling)
### single_scan_with_ceiling/plan vs single_scan_with_ceiling/plan

Registration: rotation 0×90°, translation (0.005, 0.013) m, footprint IoU 0.941.  
Measured wall pairs: **14**, within gate (≤1 cm or ≤0.5%): **8** (57%), median |Δ| 0.9 cm.

Same corners in both plans: 12 pairs, 8 pass, median |Δ| 0.6 cm. Different corners: 2 pairs, 0 pass, median |Δ| 31.2 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R4 | R1 | 0.98 | 10.64 | 10.64 | 3.073 | 3.073 |
| R5 | R5 | 0.94 | 7.26 | 7.53 | 2.335 | 2.336 |
| R2 | R2 | 0.93 | 12.25 | 11.70 | 3.084 | 3.084 |
| R1 | R3 | 0.88 | 13.06 | 14.58 | 2.397 | 2.414 |
| R6 | R6 | 0.85 | 3.46 | 3.02 | 2.257 | 2.257 |
| R3 | R4 | 0.82 | 12.25 | 10.03 | 2.944 | 2.945 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R4/R1 | W1 | W1 | 3.001 | 3.003 | +0.2 | +0.07 | true | pass |
| R4/R1 | W10 | W10 | 2.432 | 2.407 | -2.5 | -1.03 | true | fail |
| R4/R1 | W2 | W2 | 2.489 | 2.414 | -7.5 | -3.01 | false | — |
| R4/R1 | W3 | W3 | 0.806 | 0.826 | +2.0 | +2.48 | false | — |
| R4/R1 | W4 | W4 | 0.657 | 0.738 | +8.1 | +12.33 | false | — |
| R4/R1 | W5 | W5 | 2.189 | 2.174 | -1.5 | -0.69 | false | — |
| R4/R1 | W6 | W6 | 0.295 | 0.296 | +0.1 | +0.34 | false | — |
| R4/R1 | W7 | W7 | 1.714 | 1.710 | -0.4 | -0.23 | false | — |
| R4/R1 | W8 | W8 | 1.011 | 1.042 | +3.1 | +3.07 | true | fail |
| R4/R1 | W9 | W9 | 1.708 | 1.707 | -0.1 | -0.06 | true | pass |
| R5/R5 | W1 | W1 | 2.953 | 3.101 | +14.8 | +5.01 | false | — |
| R5/R5 | W2 | W2 | 2.430 | 2.428 | -0.2 | -0.08 | false | — |
| R5/R5 | W3 | W3 | 1.926 | 3.101 | +117.5 | +61.01 | false | — |
| R5/R5 | W6 | W4 | 2.514 | 2.428 | -8.6 | -3.42 | true | fail |
| R2/R2 | W1 | W1 | 4.053 | 4.044 | -0.9 | -0.22 | false | — |
| R2/R2 | W10 | W8 | 2.149 | 2.161 | +1.2 | +0.56 | false | — |
| R2/R2 | W2 | W2 | 2.113 | 2.109 | -0.4 | -0.19 | false | — |
| R2/R2 | W3 | W3 | 0.244 | 0.330 | +8.6 | +35.25 | false | — |
| R2/R2 | W4 | W4 | 0.746 | 0.750 | +0.4 | +0.54 | false | — |
| R2/R2 | W5 | W5 | 4.142 | 4.217 | +7.5 | +1.81 | false | — |
| R1/R3 | W1 | W1 | 3.608 | 4.524 | +91.6 | +25.39 | false | — |
| R1/R3 | W10 | W8 | 2.802 | 2.840 | +3.8 | +1.36 | true | fail |
| R1/R3 | W11 | W9 | 0.511 | 0.513 | +0.2 | +0.39 | true | pass |
| R1/R3 | W12 | W10 | 1.159 | 1.125 | -3.4 | -2.93 | true | fail |
| R1/R3 | W5 | W3 | 0.304 | 0.897 | +59.3 | +195.07 | false | — |
| R1/R3 | W6 | W4 | 1.476 | 1.456 | -2.0 | -1.36 | false | — |
| R1/R3 | W7 | W5 | 0.490 | 0.502 | +1.2 | +2.45 | false | — |
| R1/R3 | W8 | W6 | 0.435 | 0.428 | -0.7 | -1.61 | false | — |
| R1/R3 | W9 | W7 | 2.611 | 2.612 | +0.1 | +0.04 | false | — |
| R6/R6 | W1 | W1 | 1.935 | 1.623 | -31.2 | -16.12 | true | fail |
| R6/R6 | W2 | W2 | 1.890 | 1.895 | +0.5 | +0.26 | true | pass |
| R6/R6 | W3 | W3 | 1.488 | 1.494 | +0.6 | +0.40 | true | pass |
| R6/R6 | W4 | W4 | 0.433 | 0.429 | -0.4 | -0.92 | false | — |
| R6/R6 | W5 | W5 | 0.446 | 0.129 | -31.7 | -71.08 | false | — |
| R3/R4 | W1 | W1 | 3.898 | 3.909 | +1.1 | +0.28 | false | — |
| R3/R4 | W2 | W2 | 3.008 | 3.017 | +0.9 | +0.30 | true | pass |
| R3/R4 | W3 | W3 | 2.977 | 2.978 | +0.1 | +0.03 | true | pass |
| R3/R4 | W4 | W4 | 1.884 | 1.890 | +0.6 | +0.32 | true | pass |
| R3/R4 | W5 | W5 | 3.243 | 0.931 | -231.2 | -71.29 | false | — |
| R3/R4 | W8 | W6 | 0.150 | 1.127 | +97.7 | +651.33 | false | — |

