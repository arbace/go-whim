package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/whimsy"
)

// rsGen is the Rust backend as `whim skel <editor.c> <dir> -rs <out>` runs
// it, with the profile `whim gen` uses: what whimsy.Build and the suite's
// --rust are handed.
func rsGen(editorC, dir, rsOut string) error {
	prof, err := genProfile()
	if err != nil {
		return err
	}
	var log bytes.Buffer
	if rc := togo.Run([]string{editorC, dir, "-rs", rsOut}, &log, prof); rc != 0 {
		return fmt.Errorf("the Rust backend on %s: status %d\n%s", editorC, rc, log.Bytes())
	}
	return nil
}

// runWhimsy builds the editor in Rust (whimsy/) from a whim-vim.c: the core
// cut from it and written as the module `editor`, compiled by cargo with the
// runtime, the host and the launcher into a program.
//
//	whim whimsy [--out DIR] [--lint] [FILE]
//
// FILE is src/whim-vim.c by default and DIR lib/whimsy, where the crate and
// cargo's target go; the program is bin/whimsy -- or DIR/whimsy when DIR is
// given. rustc's time and peak memory are printed beside it; --lint then
// counts rustc's warnings on the generated module, its #![allow] taken out
// (whimsy.Lint).
func runWhimsy(args []string) int {
	file, out, prog := "src/whim-vim.c", filepath.Join("lib", "whimsy"), filepath.Join("bin", "whimsy")
	lint := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
			prog = filepath.Join(out, "whimsy")
		case args[i] == "--lint":
			lint = true
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim whimsy [--out DIR] [--lint] [FILE]")
			return 2
		}
	}
	_, st, err := whimsy.Build(rsGen, file, out, prog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim whimsy: %v\n", err)
		return 1
	}
	fmt.Printf("  %-12s the core in Rust (whimsy/), built in %s; %s\n", prog, out, st)
	if lint {
		counts, err := whimsy.Lint(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "whim whimsy --lint: %v\n", err)
			return 1
		}
		fmt.Print(whimsy.LintReport(counts))
	}
	return 0
}
