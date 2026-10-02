package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/whimsical"
)

// scmGen is the Scheme backend as `whim skel <editor.c> <dir> -scm <out>`
// runs it, with the profile `whim gen` uses: what whimsical.Build and the
// suite's --scheme are handed.
func scmGen(editorC, dir, scmOut string) error {
	prof, err := genProfile()
	if err != nil {
		return err
	}
	var log bytes.Buffer
	if rc := togo.Run([]string{editorC, dir, "-scm", scmOut}, &log, prof); rc != 0 {
		return fmt.Errorf("the Scheme backend on %s: status %d\n%s", editorC, rc, log.Bytes())
	}
	return nil
}

// runWhimsical builds the core in Scheme (whimsical/) from a whim-vim.c:
// the core cut from it and written as the library (whimsical editor),
// compiled by Chez Scheme with the runtime.
//
//	whim whimsical [--out DIR] [FILE]
//
// FILE is src/whim-vim.c by default and DIR lib/whimsical, where the
// libraries go. Chez's time and peak memory on the core are printed.
func runWhimsical(args []string) int {
	file, out := "src/whim-vim.c", filepath.Join("lib", "whimsical")
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim whimsical [--out DIR] [FILE]")
			return 2
		}
	}
	lib, st, err := whimsical.Build(scmGen, file, out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim whimsical: %v\n", err)
		return 1
	}
	fmt.Printf("  %-12s the core in Scheme (whimsical/), compiled; %s\n", lib, st)
	return 0
}
