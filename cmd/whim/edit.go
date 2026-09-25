package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arbace/go-whim/internal/phase"
)

func runEdit(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: whim edit <phase> <file> [args...]\n  phases: %s\n",
			strings.Join(phase.Names(), " "))
		return 1
	}
	f, ok := phase.Lookup(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "whim edit: no edit for phase %q\n  phases: %s\n",
			args[0], strings.Join(phase.Names(), " "))
		return 1
	}
	rest := args[2:]
	return oneFile(args[1:2], "edit "+args[0], func(t []byte, w *os.File) ([]byte, error) {
		return f(t, w, rest)
	})
}
