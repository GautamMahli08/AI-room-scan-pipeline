#!/usr/bin/env bash
# Regenerate the fix-loop before/after runs (case study Part 4).
#
#   scripts/fixloop.sh            # sample captures expected in the repo root
#   DATA=/path/to/samples scripts/fixloop.sh
#
# For each tag (fixloop-before, fixloop-after) this checks the tag out in a
# temporary git worktree, builds that version's `scan`, runs it on every
# sample capture (with_ceiling twice, for the same-input check), and
# evaluates both states with the *same* bench binary and scripts from the
# current checkout, so the measuring tool does not change between them.
# Outputs go to fixloop/<tag>/.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
DATA=${DATA:-$ROOT}
OUT=$ROOT/fixloop
EXE=$(go env GOEXE)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

PY=${PYTHON:-python3}
"$PY" -c "import sys" >/dev/null 2>&1 || PY=python

(cd "$ROOT" && go build -o "$TMP/bench$EXE" ./cmd/bench)

for tag in fixloop-before fixloop-after; do
  echo "== $tag"
  wt="$TMP/wt-$tag"
  git -C "$ROOT" worktree add --detach --quiet "$wt" "$tag"
  (cd "$wt" && go build -o "$TMP/scan-$tag$EXE" ./cmd/scan)
  git -C "$ROOT" worktree remove --force "$wt"

  dst="$OUT/$tag"
  rm -rf "$dst"
  mkdir -p "$dst"
  for cap in single_room single_scan_floor_only single_scan_with_ceiling; do
    "$TMP/scan-$tag$EXE" run "$DATA/$cap" -out "$dst/run1" 2>&1 | tee -a "$dst/log.txt"
  done
  "$TMP/scan-$tag$EXE" run "$DATA/single_scan_with_ceiling" -out "$dst/run2" 2>&1 | tee -a "$dst/log.txt"

  cd "$dst"
  {
    echo "# Repeatability, $tag"
    echo
    echo "## Same property, two captures (gate)"
    "$TMP/bench$EXE" repeat run1/single_scan_floor_only/plan.json run1/single_scan_with_ceiling/plan.json
    echo "## Same property, two captures, drift correction off (ablation)"
    "$TMP/bench$EXE" repeat run1/single_scan_floor_only/plan_drift_off.json run1/single_scan_with_ceiling/plan_drift_off.json
    echo "## single_room against the same rooms in single_scan_with_ceiling"
    "$TMP/bench$EXE" repeat run1/single_room/plan.json run1/single_scan_with_ceiling/plan.json
    echo "## Same capture processed twice (single_scan_with_ceiling)"
    "$TMP/bench$EXE" repeat run1/single_scan_with_ceiling/plan.json run2/single_scan_with_ceiling/plan.json
  } > repeatability.md
  "$TMP/bench$EXE" repeat -json run1/single_scan_floor_only/plan.json run1/single_scan_with_ceiling/plan.json > bench.json
  "$PY" "$ROOT/scripts/repeat_decompose.py" bench.json > decomposition.txt
  "$TMP/bench$EXE" overlay run1/single_scan_floor_only/plan.json run1/single_scan_with_ceiling/plan.json overlay.svg
  find . -name '*.ply' -delete
  cd "$ROOT"
done

git -C "$ROOT" diff fixloop-before fixloop-after --stat -- . ':!fixloop' > "$OUT/fix.stat"
git -C "$ROOT" show fixloop-after -- internal > "$OUT/fix.diff"
echo "done: $OUT/{fixloop-before,fixloop-after}/repeatability.md, fix.diff"
