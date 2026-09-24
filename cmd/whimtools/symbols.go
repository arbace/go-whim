package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/dead"
	"github.com/arbace/go-whim/internal/verify"
)

// The gate at a prompt (internal/check's gate.go): the same functions a check
// calls in its own process.  Each exits 1 on a refusal, which it has already
// reported, and 2 on a usage error.

// enough is the shell's ${N:?}: a missing argument and an empty one are both
// missing.
func enough(args []string, n int, usage string) bool {
	for i := 0; i < n; i++ {
		if i >= len(args) || args[i] == "" {
			fmt.Fprintln(os.Stderr, "usage: whimtools "+usage)
			return false
		}
	}
	return true
}

// runPhasecheck was tools/phasecheck.sh.
func runPhasecheck(args []string) int {
	if !enough(args, 3, "phasecheck <work-dir> <source> <before-dir>") {
		return 2
	}
	if check.PhaseCheckTo(os.Stdout, os.Stderr, args[0], args[1], args[2]) != nil {
		return 1
	}
	return 0
}

// runPhasebuild was tools/phasebuild.sh.
func runPhasebuild(args []string) int {
	if !enough(args, 2, "phasebuild <work-dir> <lines-before>") {
		return 2
	}
	if check.PhaseBuildTo(os.Stdout, os.Stderr, args[0], args[1]) != nil {
		return 1
	}
	return 0
}

// runSymbols was tools/symbols.sh.
func runSymbols(args []string) int {
	if !enough(args, 2, "symbols <file.c> <outdir>") {
		return 2
	}
	if err := check.Symbols(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
		return 1
	}
	return 0
}

// runScore was tools/score.sh: WHIMCFLAGS and WHIMLDFLAGS come from the
// environment, as `make score` passes them.
func runScore(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: whimtools score")
		return 2
	}
	verify.Score(os.Stdout, os.Getenv("WHIMCFLAGS"), os.Getenv("WHIMLDFLAGS"))
	return 0
}

// runNvidx is tools/nvidxcheck.py.
func runNvidx(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: whimtools nvidx <file>")
		return 1
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
		return 1
	}
	line, ok := dead.NvIdxCheck(data)
	fmt.Println(line)
	if !ok {
		return 1
	}
	return 0
}
