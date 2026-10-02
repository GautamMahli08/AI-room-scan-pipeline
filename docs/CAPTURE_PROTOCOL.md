# Capture protocol (one page)

Route 2: stock apps only. Pick the tier, follow its box, then hand over the files as described at the bottom. Rule for every tier: **lights on, doors open, walk slowly.**

---

### LiDAR tier: iPhone Pro / Pro Max (12 Pro or newer)

**Install:** *Stray Scanner* from the App Store (free). Open it once and allow camera access.

1. Stand in the first room. Hold the phone **upright (portrait) at chest height**. Tap the red record button.
2. **Walk along every wall** at a slow walking pace, about 1 m from it. Keep the phone pointed at the wall, not the floor.
3. In each room, **tilt the phone up once to the ceiling** and sweep across it (3–4 seconds). Without this the ceiling height is reported as "not observed".
4. **Walk through every doorway** into the next room. Do not just point the phone through it.
5. **End where you started**: walk back into the first room. This lets the pipeline correct drift.
6. Tap stop. About **30–40 seconds per room** (a 5-room flat takes about 3 minutes).

---

### Video tier: any iPhone 15 or newer (no Pro needed)

**App:** the built-in *Camera*, **Video** mode, **1×** lens (not 0.5×), 1080p or 4K at 30 fps.

Walk exactly as in the LiDAR box (steps 1–6). Choose portrait **or** landscape and **do not rotate the phone during the recording**.

---

### Photo tier: any iPhone 15 or newer

**App:** the built-in *Camera*, **Photo** mode, **1×** lens.

For each room:

1. Take **4–6 photos from the corners**: stand in a corner and aim at the opposite corner, phone held level. Each photo should show **floor, walls and ceiling line**.
2. For **every doorway**, take **one photo through it from each side**, standing about 2 m back, so the next room is visible.
3. 2–8 photos per room in total. Keep the same room's photos together.

---

### Avoid (every tier)

- **Mirrors and glass up close.** They show a fake room behind the wall. If a mirror is unavoidable, pass it at an angle and do not stop in front of it.
- **Long looks at a blank wall from under 1 m away**: nothing to track.
- Covering the camera or LiDAR with fingers, fast spins, and running.
- Dark rooms: turn the lights on. LiDAR still works in the dark, but the video and photo tiers do not.

---

### Hand the files to the pipeline

| Tier | On the phone | On the computer |
|---|---|---|
| LiDAR | Files app → *On My iPhone → Stray Scanner* → long-press the recording → **Share → AirDrop** (or copy over USB with Finder / iTunes File Sharing) | Unzip into a folder, e.g. `captures/flat1/` (it contains `rgb.mp4`, `depth/`, `odometry.csv`) and run `bin/scan run captures/flat1` |
| Video | Photos → the video → **Share → AirDrop**, choosing **Options → "All Photos Data"** if offered | `bin/scan run captures/walk.mov` |
| Photo | Select one room's photos → **Share → AirDrop** | Put them in **one folder per room**, all inside one folder: `captures/flat1_photos/kitchen/`, `…/bedroom/`, … and run `bin/scan run captures/flat1_photos -tier photo` |

The plan appears in `results/<name>/plan.svg` (open in a browser) and `results/<name>/plan.json`.
