package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/dead"
	"github.com/arbace/go-whim/internal/verify"
)

// runSymbols is tools/symbols.sh.
func runSymbols(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: whimtools symbols <file.c> <outdir>")
		return 1
	}
	if err := verify.Symbols(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
		return 1
	}
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
