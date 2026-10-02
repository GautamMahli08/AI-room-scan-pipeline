package pipeline

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// inputStamp identifies an input (a video file, or a folder of photo
// folders) for the reconstruction cache: every file's relative path, size
// and modification time.
type inputStamp struct {
	Path  string   `json:"path"`
	Files []string `json:"files"`
}

func stamp(path string) (inputStamp, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return inputStamp{}, err
	}
	s := inputStamp{Path: filepath.ToSlash(abs)}
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(path, p)
		s.Files = append(s.Files, fmt.Sprintf("%s|%d|%d", filepath.ToSlash(rel), info.Size(), info.ModTime().Unix()))
		return nil
	})
	return s, err
}

func (a inputStamp) equal(b inputStamp) bool {
	if a.Path != b.Path || len(a.Files) != len(b.Files) {
		return false
	}
	for i := range a.Files {
		if a.Files[i] != b.Files[i] {
			return false
		}
	}
	return true
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
			if json.Unmarshal(b, &old) == nil && old.equal(st) {
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
