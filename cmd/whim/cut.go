package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/arbace/go-whim/crefactor/edit"
)

// oneFile is the shape most cutters have: one file argument, read it whole,
// transform it, write it back, and print what was done.  A cutter that cannot
// do its work REFUSES rather than reporting a job it did not do.
func oneFile(args []string, name string, f func([]byte, *os.File) ([]byte, error)) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: whim %s <file>\n", name)
		return 1
	}
	text, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	out, err := f(text, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := writeFile(args[0], out); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	return 0
}

// runFold exercises crefactor/edit's fold primitives directly.
//
// cutil.py has no CLI -- phase programs import it -- so this exists so the
// three can be compared against the Python on the same input and the same
// pattern.  A primitive that 142 call sites in the phase programs depend on should be
// testable without running a phase.
func runFold(args []string) int {
	if len(args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: whim fold <always|never|dropif> <file> <pattern> <count>")
		return 2
	}
	kind, path, pattern := args[0], args[1], args[2]
	count, err := strconv.Atoi(args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 2
	}
	text, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	var out []byte
	switch kind {
	case "always":
		out, err = edit.FoldAlways(text, pattern, count)
	case "never":
		out, err = edit.FoldNever(text, pattern, count)
	case "dropif":
		out, err = edit.DropIf(text, pattern, count)
	default:
		fmt.Fprintln(os.Stderr, "usage: whim fold <always|never|dropif> <file> <pattern> <count>")
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := writeFile(path, out); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	return 0
}

// runRetire is tools/retire.py.

// runDroplocal is tools/droplocal.py.

// runDropoptions is tools/dropoptions.py.

// runDropopts is tools/dropopts.py -- NOT tools/dropoptions.py, which is a
// different tool with a confusingly similar name: that one removes rows from
// options[], this one removes options from command_line_scan.
