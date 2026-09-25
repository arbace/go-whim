package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// runCemit prints a translation unit in the canonical form, in place.
//
//	whim cemit <file.c>            rewrite it canonically
//	whim cemit <file.c> --check    refuse if it is not already canonical
func runCemit(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: whim cemit <file.c> [--check]")
		return 2
	}
	path := args[0]
	check := len(args) > 1 && args[1] == "--check"
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	out, err := cemit.Canonical(path, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  cemit        %v\n", err)
		return 1
	}
	if check {
		if string(out) == string(src) {
			fmt.Printf("  cemit        %s is canonical\n", path)
			return 0
		}
		fmt.Fprintf(os.Stderr, "  cemit        %s is NOT canonical\n", path)
		return 1
	}
	if err := writeFile(path, out); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	fmt.Printf("  cemit        %s, %d lines from %d\n", path, lines(out), lines(src))
	return 0
}

func lines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}
