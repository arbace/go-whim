package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/score"
)

// runScore is `make score`: bytes to store and symbols to provide, the input
// beside the product (internal/score).
func runScore(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: whimtools score")
		return 2
	}
	score.Score(os.Stdout, os.Getenv("WHIMCFLAGS"), os.Getenv("WHIMLDFLAGS"))
	return 0
}
