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

// runWhimsical builds the editor in Scheme (whimsical/) from a whim-vim.c:
// the core cut from it and written as the library (whimsical editor),
// compiled by Chez Scheme with the runtime, the host and the launcher into
// a boot file, and linked with Chez's kernel into a program.
//
//	whim whimsical [--debug] [--out DIR] [FILE]
//
// FILE is src/whim-vim.c by default and DIR lib/whimsical, where the
// libraries and the boot files go; the program is bin/whimsical -- or
// DIR/whimsical when DIR is given. Chez's time and peak memory on the core
// are printed beside it.  --debug makes the debugging build instead
// (whimsical.Debug: optimize-level 2, safe, the inspector's information
// kept) in lib/whimsical-debug, the program bin/whimsical-debug (or
// DIR/whimsical-debug), and leaves the release build as it is.
func runWhimsical(args []string) int {
	file, out, name, mode := "src/whim-vim.c", "", "whimsical", whimsical.Release
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
		case args[i] == "--debug":
			mode, name = whimsical.Debug, "whimsical-debug"
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim whimsical [--debug] [--out DIR] [FILE]")
			return 2
		}
	}
	prog := filepath.Join("bin", name)
	if out == "" {
		out = filepath.Join("lib", name)
	} else {
		prog = filepath.Join(out, name)
	}
	_, st, err := whimsical.Build(scmGen, file, out, prog, mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim whimsical: %v\n", err)
		return 1
	}
	build := "the core in Scheme (whimsical/)"
	if mode == whimsical.Debug {
		build += ", the debugging build,"
	}
	fmt.Printf("  %-12s %s built in %s; %s\n", prog, build, out, st)
	return 0
}
