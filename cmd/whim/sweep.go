package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/sweep"
	"github.com/arbace/go-whim/internal/whim"
)

// runSweep is the dead-code sweep on one file (internal/sweep).  By default it
// is told what the pipeline tells it, vim's profile; --root and --freeze tell
// it about another program instead: its entry points, and the functions whose
// presence freezes struct layouts.
func runSweep(args []string) int {
	opt := whim.Profile.Sweep
	var roots, freeze []string
	var file string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--root" && i+1 < len(args):
			i++
			roots = append(roots, args[i])
		case args[i] == "--freeze" && i+1 < len(args):
			i++
			freeze = append(freeze, args[i])
		case file == "" && len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			file = ""
			i = len(args)
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "usage: whim sweep [--root NAME]... [--freeze NAME]... <file.c>")
		return 1
	}
	if roots != nil || freeze != nil {
		opt = sweep.Options{Roots: roots, FreezeLayoutIf: freeze}
	}
	if _, err := sweep.Sweep(file, os.Stdout, opt); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	return 0
}
