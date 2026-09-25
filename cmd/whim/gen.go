package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/internal/gen/pre"
	"github.com/arbace/go-whim/internal/gen/splice"
	"github.com/arbace/go-whim/internal/whim"
)

// runGen writes editor/editor.go from editor.c, the core of whim-vim.c, and
// internal/gen/sigs.md beside it -- or, with --check, refuses when either is
// not what internal/gen writes.  make cuts editor.c first (`make
// editor/editor.go`, `make whim-editor-check`); this never runs make, so a
// check cannot start a build.  crt.go and host.go are not generated: they are
// the runtime and the host.  Each file is written only when it differs, so a
// current one keeps its mtime.
func runGen(args []string) int {
	check := false
	switch {
	case len(args) == 1 && args[0] == "--check":
		check = true
	case len(args) != 0:
		fmt.Fprintln(os.Stderr, "usage: whim gen [--check]")
		return 2
	}
	if _, err := os.Stat("editor.c"); err != nil {
		fmt.Println("whim gen: no editor.c; make cuts it from whim-vim.c: make editor/editor.go")
		return 1
	}
	out, err := os.MkdirTemp("", "gen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(out)
	if rc := togo.Run([]string{"editor.c", out, "-editor", filepath.Join(out, "editor.go")}, io.Discard, whim.Gen); rc != 0 {
		fmt.Fprintln(os.Stderr, "whim gen: the generator refused editor.c")
		return rc
	}
	files := []struct{ made, tracked string }{
		{filepath.Join(out, "editor.go"), "editor/editor.go"},
		{filepath.Join(out, "sigs.md"), "internal/gen/sigs.md"},
	}
	fail, changed := false, false
	for _, f := range files {
		made, err := os.ReadFile(f.made)
		if err != nil {
			fmt.Fprintf(os.Stderr, "whim: %v\n", err)
			return 1
		}
		have, _ := os.ReadFile(f.tracked)
		if bytes.Equal(made, have) {
			continue
		}
		if check {
			fmt.Printf("  %-12s is NOT what internal/gen writes from whim-vim.c.  Run: make editor/editor.go\n", filepath.Base(f.tracked))
			fail = true
			continue
		}
		if err := os.WriteFile(f.tracked, made, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "whim: %v\n", err)
			return 1
		}
		changed = true
	}
	switch {
	case fail:
		return 1
	case check:
		fmt.Printf("  %-12s is what internal/gen writes from whim-vim.c\n", "editor.go")
	case changed:
		b, _ := os.ReadFile("editor/editor.go")
		fmt.Printf("  %-12s %d lines, generated from whim-vim.c\n", "editor.go", bytes.Count(b, []byte("\n")))
	default:
		fmt.Printf("  %-12s current -- what internal/gen writes from whim-vim.c\n", "editor.go")
	}
	return 0
}

// runSkel is the generator itself, for a run by hand: `whim skel <editor.c>
// <outdir> [-bodies | -editor <editor.go>]` writes the skeleton, the facts and,
// asked, the bodies or the whole editor.go into outdir.
func runSkel(args []string) int { return togo.Run(args, os.Stderr, whim.Gen) }

// runSplice measures the emitted bodies against the hand-written editor:
// `whim splice <editor-dir> <bodies.go> <out-dir>`.
func runSplice(args []string) int { return splice.Run(args, os.Stderr) }

// runPre runs one of internal/ccx's partitions on an editor.c:
// `whim pre casts|order|unions|garrays|voids|gotos|funcs <editor.c>`.
func runPre(args []string) int { return pre.Run(args, os.Stderr) }
