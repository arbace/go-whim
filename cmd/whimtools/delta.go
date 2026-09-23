package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/arbace/go-whim/internal/harness"
	"github.com/arbace/go-whim/internal/verify"
)

// runDelta is the declared delta, in the three shapes a person or a program
// asks for it (internal/verify).
//
//	whimtools delta <binary> <source> --phase N   the delta at N, as a check
//	whimtools delta --declared N                  what phase N itself declares
//	whimtools delta --list FROM TO                a run of phases' declarations
//
// The first is what a stage runs and what measures a build of the Go editor.
// The second is the list a phase program asserts its own table against.  The
// third is the grammar underneath both, which is worth reading by hand when a
// declaration does not fold the way its author meant.
//
// From build.CoreFrom (83) on, every one of them is the core's: other
// baselines, another instrument, and zero's tokens rather than whim's.
func runDelta(args []string) int {
	usage := "usage: whimtools delta <binary> <source> --phase N | --declared N | --list FROM TO"
	num := func(s string) (int, bool) {
		n, err := strconv.Atoi(s)
		if err != nil {
			fmt.Fprintln(os.Stderr, usage)
			return 0, false
		}
		return n, true
	}
	switch {
	case len(args) == 2 && args[0] == "--declared":
		n, ok := num(args[1])
		if !ok {
			return 2
		}
		own, err := verify.PhaseDeclared(n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
			return 1
		}
		for _, t := range own {
			fmt.Println(t)
		}
		return 0

	case len(args) == 3 && args[0] == "--list":
		from, ok := num(args[1])
		to, ok2 := num(args[2])
		if !ok || !ok2 {
			return 2
		}
		text, err := verify.Declarations(from, to)
		if err != nil {
			fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
			return 1
		}
		fmt.Print(text)
		return 0

	case len(args) == 4 && args[2] == "--phase":
		n, ok := num(args[3])
		if !ok {
			return 2
		}
		if err := verify.Delta(args[0], args[1], n, os.Stdout); err != nil {
			if err != harness.ErrReported {
				fmt.Fprintf(os.Stderr, "  delta        %v\n", err)
			}
			return 1
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, usage)
	return 2
}
