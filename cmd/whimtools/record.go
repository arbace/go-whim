package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/verify"
)

// runRecord records the two baseline sets every delta is measured against, or
// compares them with what is already there.  It is phase 0's recording and
// phase 83's, which is why it builds q82 to make the second.
func runRecord(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: whimtools record")
		return 2
	}
	if err := verify.Record(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "  baselines    %v\n", err)
		return 1
	}
	return 0
}
