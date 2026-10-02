### single_scan_floor_only/plan_drift_off vs single_scan_with_ceiling/plan_drift_off

Registration: rotation 1×90°, translation (-5.976, -0.073) m, footprint IoU 0.843.  
Measured wall pairs: **10**, within gate (≤1 cm or ≤0.5%): **0** (0%), median |Δ| 22.9 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R1 | R5 | 0.91 | 13.54 | 13.66 | unobserved | 2.416 |
| R3 | R1 | 0.89 | 10.15 | 10.11 | unobserved | 2.956 |
| R2 | R4 | 0.81 | 14.37 | 11.73 | unobserved | 3.078 |
| R4 | R3 | 0.78 | 10.08 | 11.27 | unobserved | 3.077 |
| R6 | R7 | 0.78 | 3.51 | 3.55 | unobserved | 2.269 |
| R5 | R6 | 0.68 | 5.68 | 7.26 | unobserved | 2.345 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R1/R5 | W1 | W14 | 1.058 | 1.134 | +7.6 | +7.18 | true | fail |
| R1/R5 | W2 | W1 | 3.777 | 4.138 | +36.1 | +9.56 | false | — |
| R1/R5 | W3 | W8 | 3.887 | 2.426 | -146.1 | -37.59 | false | — |
| R1/R5 | W4 | W11 | 3.375 | 2.190 | -118.5 | -35.11 | true | fail |
| R1/R5 | W5 | W12 | 2.829 | 2.787 | -4.2 | -1.48 | true | fail |
| R1/R5 | W6 | W13 | 0.402 | 0.520 | +11.8 | +29.35 | true | fail |
| R3/R1 | W1 | W8 | 1.144 | 0.167 | -97.7 | -85.40 | false | — |
| R3/R1 | W2 | W1 | 3.980 | 3.917 | -6.3 | -1.58 | false | — |
| R3/R1 | W3 | W2 | 3.047 | 2.991 | -5.6 | -1.84 | true | fail |
| R3/R1 | W4 | W3 | 1.916 | 2.969 | +105.3 | +54.96 | false | — |
| R3/R1 | W7 | W4 | 1.619 | 1.851 | +23.2 | +14.33 | true | fail |
| R3/R1 | W8 | W5 | 0.861 | 1.100 | +23.9 | +27.76 | false | — |
| R2/R4 | W3 | W6 | 2.073 | 2.861 | +78.8 | +38.01 | true | fail |
| R2/R4 | W4 | W1 | 4.608 | 3.999 | -60.9 | -13.22 | false | — |
| R2/R4 | W6 | W5 | 2.655 | 4.348 | +169.3 | +63.77 | false | — |
| R4/R3 | W1 | W6 | 1.160 | 1.020 | -14.0 | -12.07 | true | fail |
| R4/R3 | W10 | W5 | 0.160 | 3.852 | +369.2 | +2307.50 | false | — |
| R4/R3 | W2 | W7 | 2.225 | 1.728 | -49.7 | -22.34 | true | fail |
| R4/R3 | W4 | W1 | 2.484 | 3.014 | +53.0 | +21.34 | false | — |
| R4/R3 | W5 | W2 | 2.811 | 2.475 | -33.6 | -11.95 | false | — |
| R4/R3 | W7 | W4 | 0.389 | 0.966 | +57.7 | +148.33 | false | — |
| R6/R7 | W1 | W8 | 1.235 | 1.464 | +22.9 | +18.54 | true | fail |
| R6/R7 | W3 | W4 | 1.686 | 0.425 | -126.1 | -74.79 | false | — |
| R6/R7 | W4 | W5 | 1.802 | 1.651 | -15.1 | -8.38 | false | — |
| R6/R7 | W5 | W6 | 0.451 | 0.429 | -2.2 | -4.88 | false | — |
| R6/R7 | W6 | W7 | 0.382 | 0.451 | +6.9 | +18.06 | false | — |
| R5/R6 | W2 | W1 | 2.397 | 2.953 | +55.6 | +23.20 | false | — |
| R5/R6 | W4 | W3 | 2.397 | 1.951 | -44.6 | -18.61 | false | — |

Unmatched rooms: A [], B [R2]

### single_scan_floor_only/plan vs single_scan_with_ceiling/plan

Registration: rotation 1×90°, translation (-6.195, 0.024) m, footprint IoU 0.818.  
Measured wall pairs: **14**, within gate (≤1 cm or ≤0.5%): **2** (14%), median |Δ| 13.6 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R5 | R3 | 0.88 | 10.96 | 10.03 | unobserved | 2.944 |
| R1 | R1 | 0.87 | 13.21 | 14.91 | unobserved | 2.414 |
| R3 | R4 | 0.81 | 10.13 | 10.84 | unobserved | 3.073 |
| R2 | R2 | 0.77 | 14.23 | 12.29 | unobserved | 3.084 |
| R4 | R5 | 0.64 | 4.98 | 7.64 | unobserved | 2.335 |
| R6 | R6 | 0.57 | 5.78 | 3.40 | unobserved | 2.257 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R5/R3 | W4 | W1 | 3.678 | 3.908 | +23.0 | +6.25 | false | — |
| R5/R3 | W5 | W2 | 3.002 | 3.019 | +1.7 | +0.57 | true | fail |
| R5/R3 | W6 | W3 | 3.110 | 2.973 | -13.7 | -4.41 | true | fail |
| R5/R3 | W7 | W4 | 1.909 | 1.895 | -1.4 | -0.73 | true | fail |
| R5/R3 | W8 | W5 | 1.629 | 0.934 | -69.5 | -42.66 | false | — |
| R1/R1 | W1 | W10 | 1.055 | 1.122 | +6.7 | +6.35 | true | fail |
| R1/R1 | W2 | W1 | 3.824 | 4.521 | +69.7 | +18.23 | false | — |
| R1/R1 | W3 | W4 | 3.440 | 1.457 | -198.3 | -57.65 | false | — |
| R1/R1 | W4 | W5 | 0.453 | 0.831 | +37.8 | +83.44 | false | — |
| R1/R1 | W6 | W7 | 2.899 | 2.544 | -35.5 | -12.25 | false | — |
| R1/R1 | W7 | W8 | 2.792 | 2.841 | +4.9 | +1.76 | true | fail |
| R1/R1 | W8 | W9 | 0.473 | 0.515 | +4.2 | +8.88 | true | fail |
| R3/R4 | W1 | W8 | 1.172 | 0.944 | -22.8 | -19.45 | false | — |
| R3/R4 | W10 | W7 | 0.183 | 2.570 | +238.7 | +1304.37 | false | — |
| R3/R4 | W2 | W9 | 2.131 | 1.709 | -42.2 | -19.80 | false | — |
| R3/R4 | W4 | W1 | 2.555 | 2.999 | +44.4 | +17.38 | false | — |
| R3/R4 | W5 | W2 | 2.754 | 2.497 | -25.7 | -9.33 | false | — |
| R3/R4 | W8 | W5 | 1.928 | 1.385 | -54.3 | -28.16 | true | fail |
| R2/R2 | W2 | W6 | 2.148 | 2.143 | -0.5 | -0.23 | false | — |
| R2/R2 | W3 | W1 | 4.068 | 4.336 | +26.8 | +6.59 | true | fail |
| R2/R2 | W6 | W2 | 0.810 | 2.860 | +205.0 | +253.09 | true | fail |
| R2/R2 | W7 | W3 | 3.370 | 4.184 | +81.4 | +24.15 | false | — |
| R4/R5 | W2 | W1 | 2.054 | 3.105 | +105.1 | +51.17 | true | fail |
| R4/R5 | W4 | W5 | 2.054 | 1.025 | -102.9 | -50.10 | false | — |
| R6/R6 | W1 | W6 | 1.284 | 1.292 | +0.8 | +0.62 | true | pass |
| R6/R6 | W11 | W4 | 0.436 | 0.435 | -0.1 | -0.23 | true | pass |
| R6/R6 | W12 | W5 | 0.586 | 0.450 | -13.6 | -23.21 | true | fail |
| R6/R6 | W2 | W1 | 2.231 | 2.081 | -15.0 | -6.72 | true | fail |
| R6/R6 | W3 | W2 | 1.282 | 1.727 | +44.5 | +34.71 | false | — |
| R6/R6 | W6 | W3 | 0.162 | 1.632 | +147.0 | +907.41 | false | — |

### single_room/plan vs single_scan_with_ceiling/plan

Registration: rotation 0×90°, translation (1.031, 6.552) m, footprint IoU 0.346.  
Measured wall pairs: **3**, within gate (≤1 cm or ≤0.5%): **1** (33%), median |Δ| 3.3 cm.

| Room A | Room B | IoU | Area A m² | Area B m² | Ceiling A | Ceiling B |
|---|---|---|---|---|---|---|
| R1 | R5 | 0.96 | 7.76 | 7.64 | unobserved | 2.335 |
| R2 | R4 | 0.60 | 8.97 | 10.84 | unobserved | 3.073 |

| Room | Wall A | Wall B | Len A m | Len B m | Δ cm | Δ % | Both measured | Gate |
|---|---|---|---|---|---|---|---|---|
| R1/R5 | W1 | W1 | 3.072 | 3.105 | +3.3 | +1.07 | true | fail |
| R1/R5 | W2 | W2 | 2.527 | 2.428 | -9.9 | -3.92 | true | fail |
| R1/R5 | W3 | W5 | 3.072 | 1.025 | -204.7 | -66.63 | false | — |
| R1/R5 | W4 | W6 | 2.527 | 2.523 | -0.4 | -0.16 | true | pass |
| R2/R4 | W1 | W1 | 3.599 | 2.999 | -60.0 | -16.67 | false | — |
| R2/R4 | W3 | W3 | 3.599 | 0.753 | -284.6 | -79.08 | false | — |
| R2/R4 | W4 | W10 | 2.493 | 2.497 | +0.4 | +0.16 | false | — |

Unmatched rooms: A [R3], B [R1 R2 R3 R6]

