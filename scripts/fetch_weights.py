"""Download every model the video and photo tiers use, ahead of time, so a
run never downloads mid-way (and works offline afterwards).

usage: ml/.venv/bin/python scripts/fetch_weights.py   (Windows: ml\\.venv\\Scripts\\python.exe)
"""
from huggingface_hub import snapshot_download

MODELS = [
    ("facebook/VGGT-1B", "multi-view poses + depth (video and photo tiers)"),
    ("apple/DepthPro-hf", "metric depth with focal estimate (metric scale)"),
    ("depth-anything/Depth-Anything-V2-Metric-Indoor-Base-hf", "depth ordering for image orientation"),
    ("google/owlv2-base-patch16-ensemble", "open-vocabulary damage detection"),
]

for repo, use in MODELS:
    path = snapshot_download(repo)
    print(f"{repo:60s} {use}\n    -> {path}")
