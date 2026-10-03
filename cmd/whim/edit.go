package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/steps"
)

func runEdit(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: whim edit <phase> <file> [args...]\n  phases: %s\n",
			strings.Join(phase.Names(), " "))
		return 1
	}
	_, onText := phase.Lookup(args[0])
	_, onGraph := phase.LookupGraph(args[0])
	if !onText && !onGraph {
		fmt.Fprintf(os.Stderr, "whim edit: no edit for phase %q\n  phases: %s\n",
			args[0], strings.Join(phase.Names(), " "))
		return 1
	}
	// a phase whose program is on the graph runs on the file imported, and
	// writes its C view (steps.OnText)
	f, _ := steps.OnText("edit")
	rest := append([]string{args[0]}, args[2:]...)
	return oneFile(args[1:2], "edit "+args[0], func(t []byte, w *os.File) ([]byte, error) {
		return f(t, rest, w)
	})
}
