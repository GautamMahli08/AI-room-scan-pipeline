// Command bench produces the benchmark tables from plan outputs.
//
//	bench repeat [-json] <planA.json> <planB.json> [more pairs...]   repeatability tables (markdown, or JSON)
//	bench overlay <planA.json> <planB.json> <out.svg>       both plans registered in one drawing
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"roomscan/internal/bench"
	"roomscan/internal/output"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) == 5 && os.Args[1] == "overlay" {
		a, b := load(os.Args[2]), load(os.Args[3])
		t, _ := bench.Register(a, b)
		if err := bench.WriteOverlay(os.Args[4], a, b, t); err != nil {
			log.Fatal(err)
		}
		return
	}
	if pairs := os.Args[2:]; len(os.Args) < 4 || os.Args[1] != "repeat" || (len(pairs)-boolInt(pairs[0] == "-json"))%2 != 0 {
		fmt.Fprintln(os.Stderr, "usage: bench repeat <planA.json> <planB.json> [<planC.json> <planD.json> ...]")
		os.Exit(2)
	}
	args := os.Args[2:]
	asJSON := false
	if args[0] == "-json" {
		asJSON, args = true, args[1:]
	}
	for i := 0; i < len(args); i += 2 {
		a, b := load(args[i]), load(args[i+1])
		r := bench.Compare(a, b)
		if asJSON {
			out, _ := json.MarshalIndent(map[string]any{"plan_a": args[i], "plan_b": args[i+1], "result": r}, "", "  ")
			fmt.Println(string(out))
			continue
		}
		fmt.Println(r.Markdown(name(args[i]), name(args[i+1])))
	}
}

func load(path string) *output.Plan {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var p output.Plan
	if err := json.Unmarshal(b, &p); err != nil {
		log.Fatalf("%s: %v", path, err)
	}
	return &p
}

// name is "<capture>/<file>" without the extension.
func name(path string) string {
	return filepath.ToSlash(filepath.Join(filepath.Base(filepath.Dir(path)), strings.TrimSuffix(filepath.Base(path), ".json")))
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
