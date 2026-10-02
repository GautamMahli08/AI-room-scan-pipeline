"""VGGT inference that fits a 4 GB GPU.

The 1B-parameter aggregator runs in fp16 (2.5 GB of weights); the camera
and depth heads, which VGGT runs with autocast disabled, keep fp32 weights
and get fp32 tokens. The point and tracking heads are not used and are
dropped to save memory.

VGGT was trained with image width 518. Portrait frames (taller than wide)
are padded left and right to 518x518, as VGGT's own "pad" preprocessing
does; otherwise the predicted horizontal and vertical fields of view come
out equal and the focal lengths wrong (fx 357 vs fy 470 for a true 432 on
single_room). Outputs are cropped back and pixels are forced square.
"""
import numpy as np


class VGGTRunner:
    def __init__(self, device):
        import torch
        from vggt.models.vggt import VGGT
        from vggt.utils.pose_enc import pose_encoding_to_extri_intri
        self.torch, self.device, self.to_extri = torch, device, pose_encoding_to_extri_intri
        model = VGGT.from_pretrained("facebook/VGGT-1B")
        model.point_head = None
        model.track_head = None
        if device == "cuda":
            model.aggregator.half()
        self.model = model.to(device).eval()

    def forward(self, x):
        """x: (1, S, 3, H, W) float in [0, 1] on the device."""
        torch = self.torch
        m = self.model
        cuda = self.device == "cuda"
        with torch.no_grad():
            with torch.autocast("cuda", dtype=torch.float16, enabled=cuda):
                tokens, start = m.aggregator(x.half() if cuda else x)
            tokens = [t.float() if t is not None else None for t in tokens]
            pose = m.camera_head(tokens)[-1]
            depth, conf = m.depth_head(tokens, images=x.float(), patch_start_idx=start)
        return pose, depth, conf

    def __call__(self, imgs):
        """imgs: list of HxWx3 uint8 RGB of one size (multiples of 14, long
        side 518). Returns cam_from_world (S,3,4), K (S,3,3), depth (S,H,W),
        conf (S,H,W) in chunk units, at the input size."""
        torch = self.torch
        H, W = imgs[0].shape[:2]
        pad = (H - W) // 2 if H > W else 0
        arr = np.stack(imgs).astype(np.float32) / 255.0
        if pad:
            arr = np.pad(arr, ((0, 0), (0, 0), (pad, H - W - pad), (0, 0)), constant_values=1.0)
        x = torch.from_numpy(arr).permute(0, 3, 1, 2).to(self.device)[None]
        pose, depth, conf = self.forward(x)
        ext, K = self.to_extri(pose, x.shape[-2:])
        ext = ext[0].cpu().numpy().astype(np.float64)
        K = K[0].cpu().numpy().astype(np.float64)
        depth = depth[0, ..., 0].cpu().numpy()
        conf = conf[0].cpu().numpy()
        if pad:
            depth, conf = depth[:, :, pad:pad + W], conf[:, :, pad:pad + W]
            K[:, 0, 2] -= pad
        f = (K[:, 0, 0] + K[:, 1, 1]) / 2  # square pixels
        K[:, 0, 0] = K[:, 1, 1] = f
        return ext, K, depth, conf
