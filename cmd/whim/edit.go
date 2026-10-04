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
	if _, ok := phase.LookupGraph(args[0]); !ok {
		fmt.Fprintf(os.Stderr, "whim edit: no edit for phase %q\n  phases: %s\n",
			args[0], strings.Join(phase.Names(), " "))
		return 1
	}
	// the phase's program runs on the graph of the file imported, and writes
	// its C view (steps.OnText)
	f, _ := steps.OnText("edit")
	rest := append([]string{args[0]}, args[2:]...)
	return oneFile(args[1:2], "edit "+args[0], func(t []byte, w *os.File) ([]byte, error) {
		return f(t, rest, w)
	})
}
