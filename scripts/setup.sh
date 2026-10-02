#!/usr/bin/env bash
# One-time setup on a clean machine.
#
#   scripts/setup.sh          # LiDAR tier + video/photo tiers (Python, GPU optional)
#   scripts/setup.sh --lidar  # LiDAR tier only (Go only, ~1 minute)
#
# Needs: Go >= 1.25, ffmpeg on PATH; for video/photo also uv
# (https://docs.astral.sh/uv/) which fetches Python 3.12 itself.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
EXE=$(go env GOEXE)

echo "== building Go tools"
go build -o "bin/scan$EXE" ./cmd/scan
go build -o "bin/bench$EXE" ./cmd/bench
go test ./... >/dev/null && echo "   tests pass"

if [[ "${1:-}" == "--lidar" ]]; then
  echo "done (LiDAR tier only). Try: bin/scan$EXE run <stray_scanner_capture>"
  exit 0
fi

echo "== Python environment for the video and photo tiers (ml/.venv)"
uv venv ml/.venv --python 3.12
if command -v nvidia-smi >/dev/null 2>&1; then
  INDEX=https://download.pytorch.org/whl/cu124   # NVIDIA GPU
else
  INDEX=https://download.pytorch.org/whl/cpu     # CPU only: works, much slower
fi
uv pip install --python ml/.venv -r ml/requirements.txt \
  --index-url "$INDEX" --extra-index-url https://pypi.org/simple --index-strategy unsafe-best-match

PY=ml/.venv/bin/python
[[ -x $PY ]] || PY=ml/.venv/Scripts/python.exe
echo "== model weights (~7.5 GB, cached under ~/.cache/huggingface)"
"$PY" scripts/fetch_weights.py
echo "done. Try: bin/scan$EXE run <capture> -tier lidar|video|photo"
