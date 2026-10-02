# Fix loop bundle (case study Part 4)

| File | What it is |
|---|---|
| [DECLARATION.md](DECLARATION.md) | Worst gate, root cause with evidence, fix and prediction. Committed before the fix (`1af94b6`). |
| [RESULT.md](RESULT.md) | Before/after numbers against the prediction, and the post-mortem. |
| [fix.diff](fix.diff), [fix.stat](fix.stat) | The fix commit (tag `fixloop-after`). |
| `fixloop-before/`, `fixloop-after/` | Regenerated runs: plans, `repeatability.md`, `decomposition.txt`, `overlay.svg`, logs. |
| `before-samples/` | 5 further runs of the before build on identical input (it is non-deterministic). |

Regenerate: `scripts/fixloop.sh` (needs Go, Python 3, and the sample captures in the repo root or `DATA=...`).
