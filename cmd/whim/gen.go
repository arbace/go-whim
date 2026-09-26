package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/internal/gen/pre"
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
	prof, err := genProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if rc := togo.Run([]string{"editor.c", out, "-editor", filepath.Join(out, "editor.go")}, io.Discard, prof); rc != 0 {
		fmt.Fprintln(os.Stderr, "whim gen: the generator refused editor.c")
		return rc
	}
	// The same core in Java (doc/JAVA.md), tracked beside the Go and held to
	// the same check: braaam/Editor.java is what the Java backend writes, and
	// the backend refuses nothing -- a refusal would be a method that throws.
	jdir, err := os.MkdirTemp("", "jgen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(jdir)
	javaOut := filepath.Join(out, "Editor.java")
	if err := javaGen("editor.c", jdir, javaOut); err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if r, err := os.ReadFile(javaOut + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		fmt.Fprintf(os.Stderr, "whim gen: the Java backend refused part of the core:\n%s", r)
		return 1
	}
	// And in Clojure (doc/CLOJURE.md), the same way: whim.editor is what the
	// Clojure backend writes, refusing nothing.
	cdir, err := os.MkdirTemp("", "cljgen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(cdir)
	cljOut := filepath.Join(out, "editor.clj")
	if err := cljGen("editor.c", cdir, cljOut); err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if r, err := os.ReadFile(cljOut + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		fmt.Fprintf(os.Stderr, "whim gen: the Clojure backend refused part of the core:\n%s", r)
		return 1
	}
	files := []struct{ made, tracked string }{
		{filepath.Join(out, "editor.go"), "editor/editor.go"},
		{filepath.Join(out, "sigs.md"), "internal/gen/sigs.md"},
		{javaOut, "braaam/Editor.java"},
		{cljOut, "cljeditor/src/whim/editor.clj"},
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
			fmt.Printf("  %-12s is NOT what the generator writes from whim-vim.c.  Run: make editor/editor.go\n", filepath.Base(f.tracked))
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
		fmt.Printf("  %-12s is what the Java backend writes from whim-vim.c\n", "Editor.java")
		fmt.Printf("  %-12s is what the Clojure backend writes from whim-vim.c\n", "editor.clj")
	case changed:
		b, _ := os.ReadFile("editor/editor.go")
		fmt.Printf("  %-12s %d lines, generated from whim-vim.c\n", "editor.go", bytes.Count(b, []byte("\n")))
		j, _ := os.ReadFile("braaam/Editor.java")
		fmt.Printf("  %-12s %d lines, generated from whim-vim.c\n", "Editor.java", bytes.Count(j, []byte("\n")))
		c, _ := os.ReadFile("cljeditor/src/whim/editor.clj")
		fmt.Printf("  %-12s %d lines, generated from whim-vim.c\n", "editor.clj", bytes.Count(c, []byte("\n")))
	default:
		fmt.Printf("  %-12s current -- what internal/gen writes from whim-vim.c\n", "editor.go")
	}
	return 0
}

// runSkel is the generator itself, for a run by hand: `whim skel <editor.c>
// <outdir> [-bodies | -editor <editor.go> | -java <Class.java> | -clj
// <editor.clj> | -lowerc <lowered.c>]` writes the skeleton, the facts and,
// asked, the bodies, the whole editor.go, the Java class (doc/JAVA.md) or
// the Clojure namespace (doc/CLOJURE.md) and beside it what it refuses, or
// every function lowered and printed back as C, into outdir.
func runSkel(args []string) int {
	prof, err := genProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim skel: %v\n", err)
		return 1
	}
	return togo.Run(args, os.Stderr, prof)
}

// genProfile is whim.Gen with what editor/'s hand-written files declare --
// their methods on Editor and the fields of editorHost -- read from them now,
// so the instance pass is told what the package has and not a list that
// could fall behind it.
func genProfile() (togo.Profile, error) {
	p := whim.Gen
	if p.Instance == nil {
		return p, nil
	}
	paths, err := filepath.Glob("editor/*.go")
	if err != nil {
		return p, err
	}
	hand := map[string][]byte{}
	for _, path := range paths {
		n := filepath.Base(path)
		if n == "editor.go" || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return p, err
		}
		hand[n] = b
	}
	inst := *p.Instance
	if inst.Methods, inst.Fields, err = togo.HandNames(hand, inst.Type, inst.Embed); err != nil {
		return p, err
	}
	p.Instance = &inst
	return p, nil
}

// runPre runs one of internal/ccx's partitions on an editor.c:
// `whim pre casts|order|unions|garrays|voids|gotos|funcs <editor.c>`.
func runPre(args []string) int { return pre.Run(args, os.Stderr) }
