package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/steps"
)

// fileStep is a subcommand that is one step from internal/steps, run on a file
// in place: `whimtools <step> <file> [args...]`.
//
// THE STEP IS DEFINED ONCE.  A phase says what it wants done as a name and its
// arguments, and internal/steps says what that name means; this is the same
// table reached from a command line, so a cutter cannot mean one thing to a
// build and another to a person running it by hand.
func fileStep(name string) func([]string) int {
	return func(args []string) int {
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "usage: whimtools %s <file> [args...]\n", name)
			return 1
		}
		step, ok := steps.Lookup(name)
		if !ok {
			fmt.Fprintf(os.Stderr, "whimtools: no step named %q\n", name)
			return 1
		}
		path := args[0]
		text, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
			return 1
		}
		out, err := step(text, args[1:], os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := writeFile(path, out); err != nil {
			fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
			return 1
		}
		return 0
	}
}
