"""Check how many frames VGGT can process at once on this GPU."""
import glob
import sys
import time

import cv2
import torch

from vggt_runner import VGGTRunner

frames = sorted(glob.glob(sys.argv[1] + "/*.jpg"))
runner = VGGTRunner("cuda")
print("weights loaded, allocated GB", round(torch.cuda.memory_allocated() / 1e9, 2), flush=True)
for n in [4, 8, 12, 16, 24, 32]:
    sel = frames[:: max(1, len(frames) // n)][:n]
    imgs = [cv2.cvtColor(cv2.resize(cv2.imread(f), (518, 392)), cv2.COLOR_BGR2RGB) for f in sel]
    torch.cuda.reset_peak_memory_stats()
    t = time.time()
    try:
        ext, K, D, C = runner(imgs)
        torch.cuda.synchronize()
        print(f"n={n}: ok, peak {torch.cuda.max_memory_allocated() / 1e9:.2f} GB, {time.time() - t:.1f}s, "
              f"depth {D.shape}, median depth {float(D.mean()):.2f}", flush=True)
    except torch.cuda.OutOfMemoryError:
        print(f"n={n}: out of memory", flush=True)
        break
    torch.cuda.empty_cache()
