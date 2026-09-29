package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/caprice"
	"github.com/arbace/go-whim/crefactor/togo"
)

// hsGen is the Haskell backend as `whim skel <editor.c> <dir> -hs <out>`
// runs it, with the profile `whim gen` uses: what caprice.Build and the
// suite's --haskell are handed.
func hsGen(editorC, dir, hsOut string) error {
	prof, err := genProfile()
	if err != nil {
		return err
	}
	var log bytes.Buffer
	if rc := togo.Run([]string{editorC, dir, "-hs", hsOut}, &log, prof); rc != 0 {
		return fmt.Errorf("the Haskell backend on %s: status %d\n%s", editorC, rc, log.Bytes())
	}
	return nil
}

// runCaprice builds the editor in Haskell (caprice/) from a whim-vim.c: the
// core cut from it and written as the module Caprice.Editor, compiled by GHC
// with the runtime, the host and the launcher into a program.
//
//	whim caprice [--out DIR] [--lint] [FILE]
//
// FILE is src/whim-vim.c by default and DIR lib/caprice, where the sources
// and GHC's objects go; the program is bin/caprice -- or DIR/caprice when
// DIR is given. GHC's time and peak memory are printed beside it; --lint
// then counts `ghc -Wall`'s warnings on the generated module, by flag.
func runCaprice(args []string) int {
	file, out, prog := "src/whim-vim.c", filepath.Join("lib", "caprice"), filepath.Join("bin", "caprice")
	lint := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
			prog = filepath.Join(out, "caprice")
		case args[i] == "--lint":
			lint = true
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim caprice [--out DIR] [--lint] [FILE]")
			return 2
		}
	}
	_, st, err := caprice.Build(hsGen, file, out, prog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim caprice: %v\n", err)
		return 1
	}
	fmt.Printf("  %-12s the core in Haskell (caprice/), built in %s; %s\n", prog, out, st)
	if lint {
		counts, err := caprice.Lint(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "whim caprice --lint: %v\n", err)
			return 1
		}
		fmt.Print(caprice.LintReport(counts))
	}
	return 0
}
