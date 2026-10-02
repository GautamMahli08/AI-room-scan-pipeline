// Command bench produces the benchmark tables from plan outputs.
//
//	bench repeat <planA.json> <planB.json> [more pairs...]   repeatability tables (markdown on stdout)
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
	if len(os.Args) < 4 || os.Args[1] != "repeat" || len(os.Args[2:])%2 != 0 {
		fmt.Fprintln(os.Stderr, "usage: bench repeat <planA.json> <planB.json> [<planC.json> <planD.json> ...]")
		os.Exit(2)
	}
	args := os.Args[2:]
	for i := 0; i < len(args); i += 2 {
		a, b := load(args[i]), load(args[i+1])
		r := bench.Compare(a, b)
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
