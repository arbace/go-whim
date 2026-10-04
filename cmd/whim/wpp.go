package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/wpp"
)

// cppGen is the C++ backend as `whim skel <editor.c> <dir> -cpp <out>` runs
// it, with the profile `whim gen` uses: what wpp.Build and the suite's
// --cpp are handed.
func cppGen(editorC, dir, cppOut string) error {
	prof, err := genProfile()
	if err != nil {
		return err
	}
	var log bytes.Buffer
	if rc := togo.Run([]string{editorC, dir, "-cpp", cppOut}, &log, prof); rc != 0 {
		return fmt.Errorf("the C++ backend on %s: status %d\n%s", editorC, rc, log.Bytes())
	}
	return nil
}

// runWpp builds whim++, the editor in C++ (wpp/), from a whim-vim.c: the
// core cut from it and written as editor.hpp and editor.cpp, compiled by
// g++ with the runtime, the host, the printf, the terminal host and the
// launcher into a program.
//
//	whim wpp [--out DIR] [FILE]
//
// FILE is src/whim-vim.c by default and DIR lib/wpp, where the objects go;
// the program is bin/whim++ -- or DIR/whim++ when DIR is given.  g++'s time
// and peak memory on the core are printed beside it.
func runWpp(args []string) int {
	file, out := "src/whim-vim.c", ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim wpp [--out DIR] [FILE]")
			return 2
		}
	}
	prog := filepath.Join("bin", wpp.Program)
	if out == "" {
		out = filepath.Join("lib", "wpp")
	} else {
		prog = filepath.Join(out, wpp.Program)
	}
	_, st, err := wpp.Build(cppGen, file, out, prog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim wpp: %v\n", err)
		return 1
	}
	fmt.Printf("  %-12s the core in C++ (wpp/) built in %s; %s\n", prog, out, st)
	return 0
}
