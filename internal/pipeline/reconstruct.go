package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// inputStamp identifies an input file for the reconstruction cache.
type inputStamp struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time_unix"`
}

func stamp(path string) (inputStamp, error) {
	st, err := os.Stat(path)
	if err != nil {
		return inputStamp{}, err
	}
	abs, _ := filepath.Abs(path)
	return inputStamp{Path: filepath.ToSlash(abs), Size: st.Size(), ModTime: st.ModTime().Unix()}, nil
}

// Reconstruct runs the Python reconstruction for a video (or photo) input
// into a LiDAR-style export directory. Model inference is the only Python
// step; its output is cached and replayed when the input is unchanged,
// unless live is set.
func Reconstruct(tier, input, export string, live bool) error {
	st, err := stamp(input)
	if err != nil {
		return err
	}
	stampFile := filepath.Join(export, "input.json")
	if !live {
		if b, err := os.ReadFile(stampFile); err == nil {
			var old inputStamp
			if json.Unmarshal(b, &old) == nil && old == st {
				if _, err := os.Stat(filepath.Join(export, "meta.json")); err == nil {
					log.Printf("[%s] reusing cached %s reconstruction in %s (use -live to recompute)", tier, tier, export)
					return nil
				}
			}
		}
	}
	if err := os.MkdirAll(export, 0o755); err != nil {
		return err
	}
	script := map[string]string{"video": "video_recon.py", "photo": "photo_recon.py"}[tier]
	if script == "" {
		return fmt.Errorf("no reconstruction for tier %q", tier)
	}
	py, root, err := python()
	if err != nil {
		return err
	}
	cmd := exec.Command(py, filepath.Join(root, "ml", script), "--"+tier, input, "--out", export)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s reconstruction failed: %w", tier, err)
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	return os.WriteFile(stampFile, b, 0o644)
}

// python finds the ML environment's interpreter: $ROOMSCAN_PYTHON, else
// ml/.venv under the repository root (found from the working directory
// upwards).
func python() (exe, root string, err error) {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "ml", "video_recon.py")); err == nil {
			root = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("cannot find the repository root (ml/video_recon.py) from the working directory")
		}
		dir = parent
	}
	if p := os.Getenv("ROOMSCAN_PYTHON"); p != "" {
		return p, root, nil
	}
	exe = filepath.Join(root, "ml", ".venv", "bin", "python")
	if runtime.GOOS == "windows" {
		exe = filepath.Join(root, "ml", ".venv", "Scripts", "python.exe")
	}
	if _, err := os.Stat(exe); err != nil {
		return "", "", fmt.Errorf("ML environment not found at %s: see ml/requirements.txt (or set ROOMSCAN_PYTHON)", exe)
	}
	return exe, root, nil
}
