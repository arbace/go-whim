package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/suite"
)

// runTest is the minimal behaviour check (internal/suite): every session in
// internal/suite/cases.md on the editor built from src/whim-vim.c at REV
// (default HEAD) and from FILE (default src/whim-vim.c), which must behave
// the same, with a control the corpus must see.
//
//	whim test [--ref REV] [FILE]
func runTest(args []string) int {
	rev, file, wide := "HEAD", "src/whim-vim.c", false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--ref" && i+1 < len(args):
			i++
			rev = args[i]
		case args[i] == "--wide":
			wide = true
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim test [--wide] [--ref REV] [FILE]")
			return 2
		}
	}
	if wide {
		// the wide suite, on demand: 200-odd cases the quick one does not reach
		if err := suite.Wide(os.Stdout, rev, file); err != nil {
			fmt.Fprintf(os.Stderr, "  wide         %v\n", err)
			return 1
		}
		return 0
	}
	if err := suite.Check(os.Stdout, rev, file); err != nil {
		fmt.Fprintf(os.Stderr, "  test         %v\n", err)
		return 1
	}
	return 0
}
