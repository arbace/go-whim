package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/sweep"
)

// runSweep is the dead-code sweep on one file, to a fixpoint (internal/sweep).
// It needs no particular directory: the sweep's files are temporary ones.
func runSweep(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: whim sweep <file.c>")
		return 1
	}
	if _, err := sweep.Sweep(args[0], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	return 0
}
