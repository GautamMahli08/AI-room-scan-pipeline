#!/usr/bin/env bash
# Regenerate every LiDAR-tier result and benchmark table from the raw
# sample captures.
#
#   scripts/run_samples.sh                 # samples in the repo root
#   DATA=/path/to/samples scripts/run_samples.sh
#
# Writes results/<capture>/ (plan.json, plan.svg, plan_drift_off.*,
# drift.json) and bench/repeatability.md.
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"
DATA=${DATA:-$ROOT}
EXE=$(go env GOEXE)
go build -o "bin/scan$EXE" ./cmd/scan
go build -o "bin/bench$EXE" ./cmd/bench

for cap in single_room single_scan_floor_only single_scan_with_ceiling; do
  "bin/scan$EXE" run "$DATA/$cap" -out results
done

R=results
{
  echo "# Repeatability (LiDAR tier)"
  echo
  echo "## The property scanned twice: floor_only vs with_ceiling (drift correction on)"
  "bin/bench$EXE" repeat $R/single_scan_floor_only/plan.json $R/single_scan_with_ceiling/plan.json
  echo "## Same, drift correction off (ablation)"
  "bin/bench$EXE" repeat $R/single_scan_floor_only/plan_drift_off.json $R/single_scan_with_ceiling/plan_drift_off.json
  echo "## single_room against the same rooms in with_ceiling"
  "bin/bench$EXE" repeat $R/single_room/plan.json $R/single_scan_with_ceiling/plan.json
} > bench/repeatability.md
echo "wrote results/ and bench/repeatability.md"
