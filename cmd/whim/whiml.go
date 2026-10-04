package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/whiml"
)

// mlGen is the OCaml backend as `whim skel <editor.c> <dir> -ml <out>`
// runs it, with the profile `whim gen` uses: what whiml.Build and the
// suite's --ocaml are handed.
func mlGen(editorC, dir, mlOut string) error {
	prof, err := genProfile()
	if err != nil {
		return err
	}
	var log bytes.Buffer
	if rc := togo.Run([]string{editorC, dir, "-ml", mlOut}, &log, prof); rc != 0 {
		return fmt.Errorf("the OCaml backend on %s: status %d\n%s", editorC, rc, log.Bytes())
	}
	return nil
}

// runWhiml builds the editor in OCaml (whiml/) from a whim-vim.c: the core
// cut from it and written as the module Editor, compiled by ocamlopt with
// the runtime, the printf, the host and the launcher into a program.
//
//	whim whiml [--out DIR] [FILE]
//
// FILE is src/whim-vim.c by default and DIR lib/whiml, where the modules
// go; the program is bin/whiml -- or DIR/whiml when DIR is given.
// ocamlopt's time and peak memory on the core are printed beside it.
func runWhiml(args []string) int {
	file, out := "src/whim-vim.c", ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim whiml [--out DIR] [FILE]")
			return 2
		}
	}
	prog := filepath.Join("bin", "whiml")
	if out == "" {
		out = filepath.Join("lib", "whiml")
	} else {
		prog = filepath.Join(out, "whiml")
	}
	_, st, err := whiml.Build(mlGen, file, out, prog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim whiml: %v\n", err)
		return 1
	}
	fmt.Printf("  %-12s the core in OCaml (whiml/) built in %s; %s\n", prog, out, st)
	return 0
}
