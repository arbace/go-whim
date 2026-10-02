package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/braaam"
	"github.com/arbace/go-whim/caprice"
	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/internal/gen/pre"
	"github.com/arbace/go-whim/internal/whim"
	"github.com/arbace/go-whim/vijure"
)

// runGen writes editor/editor.go, braaam/Editor.java,
// vijure/src/whim/editor.clj, caprice/Caprice/Editor.hs (with its hs-boot),
// whimsy/src/editor.rs and whimsical/whimsical/editor.ss from the core of
// src/whim-vim.c (or FILE), and
// internal/gen/sigs.md beside them -- or, with --check, refuses when any is
// not what the generator writes.  It cuts the core itself (whim.Cut), as `whim
// java` and `whim clj` do, into a directory of its own: nothing is written
// under src/.  It never runs make, so a check cannot start a build.  crt.go
// and host.go are not generated: they are the runtime and the host.  Each
// file is written only when it differs, so a current one keeps its mtime.
func runGen(args []string) int {
	check, file := false, "src/whim-vim.c"
	for _, a := range args {
		switch {
		case a == "--check":
			check = true
		case len(a) > 0 && a[0] != '-':
			file = a
		default:
			fmt.Fprintln(os.Stderr, "usage: whim gen [--check] [FILE]")
			return 2
		}
	}
	out, err := os.MkdirTemp("", "gen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(out)
	editorC, err := cutCore(file, out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	prof, err := genProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if rc := togo.Run([]string{editorC, out, "-editor", filepath.Join(out, "editor.go")}, io.Discard, prof); rc != 0 {
		fmt.Fprintln(os.Stderr, "whim gen: the generator refused the core")
		return rc
	}
	// The same core in Java (doc/JAVA.md), tracked beside the Go and held to
	// the same check: braaam/editor/, the package whim.editor, is what the
	// Java backend writes, and the backend refuses nothing -- a refusal
	// would be a method that throws.
	jdir, err := os.MkdirTemp("", "jgen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(jdir)
	javaDir := filepath.Join(out, "java")
	if err := os.MkdirAll(javaDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	javaOut := filepath.Join(javaDir, "Editor.java")
	if err := javaGen(editorC, jdir, javaOut); err != nil {
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
	cljDir := filepath.Join(out, "clj")
	if err := os.MkdirAll(cljDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	cljOut := filepath.Join(cljDir, "editor.clj")
	if err := cljGen(editorC, cdir, cljOut); err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if r, err := os.ReadFile(cljOut + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		fmt.Fprintf(os.Stderr, "whim gen: the Clojure backend refused part of the core:\n%s", r)
		return 1
	}
	// And in Haskell (doc/HASKELL.md): the module Caprice.Editor and its
	// hs-boot interface, which caprice's host imports, refusing nothing.
	hdir, err := os.MkdirTemp("", "hsgen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(hdir)
	hsOut := filepath.Join(out, "Editor.hs")
	if err := hsGen(editorC, hdir, hsOut); err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if r, err := os.ReadFile(hsOut + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		fmt.Fprintf(os.Stderr, "whim gen: the Haskell backend refused part of the core:\n%s", r)
		return 1
	}
	// And in Rust (doc/RUST.md): the module editor of the crate whimsy,
	// refusing nothing.
	rdir, err := os.MkdirTemp("", "rsgen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(rdir)
	rsOut := filepath.Join(out, "editor.rs")
	if err := rsGen(editorC, rdir, rsOut); err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if r, err := os.ReadFile(rsOut + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		fmt.Fprintf(os.Stderr, "whim gen: the Rust backend refused part of the core:\n%s", r)
		return 1
	}
	// And in Scheme (doc/SCHEME.md): the library (whimsical editor),
	// refusing nothing.
	sdir, err := os.MkdirTemp("", "scmgen")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	defer os.RemoveAll(sdir)
	scmOut := filepath.Join(out, "editor.ss")
	if err := scmGen(editorC, sdir, scmOut); err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	if r, err := os.ReadFile(scmOut + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		fmt.Fprintf(os.Stderr, "whim gen: the Scheme backend refused part of the core:\n%s", r)
		return 1
	}
	files := []struct{ made, tracked string }{
		{filepath.Join(out, "editor.go"), "editor/editor.go"},
		{filepath.Join(out, "sigs.md"), "internal/gen/sigs.md"},
		{cljOut, "vijure/src/whim/editor.clj"},
		{rsOut, "whimsy/src/editor.rs"},
		{scmOut, "whimsical/whimsical/editor.ss"},
	}
	// the Haskell: the module, its hs-boot, and its parts
	hsFiles, err := caprice.Generated(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	made := map[string]bool{}
	// the Clojure's parts, which its namespace loads
	cljParts, err := vijure.Parts(cljOut)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	for _, p := range cljParts {
		tracked := filepath.Join("vijure", "src", "whim", p)
		made[tracked] = true
		files = append(files, struct{ made, tracked string }{filepath.Join(cljDir, p), tracked})
	}
	// the Java: the package's files
	javaFiles, err := braaam.Generated(javaDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim gen: %v\n", err)
		return 1
	}
	for _, f := range javaFiles {
		tracked := filepath.Join("braaam", "editor", f)
		made[tracked] = true
		files = append(files, struct{ made, tracked string }{filepath.Join(javaDir, f), tracked})
	}
	for _, f := range hsFiles {
		tracked := filepath.Join("caprice", "Caprice", f)
		made[tracked] = true
		files = append(files, struct{ made, tracked string }{filepath.Join(out, f), tracked})
	}
	stale, _ := filepath.Glob(filepath.Join("caprice", "Caprice", "Editor", "*.hs"))
	javaStale, _ := filepath.Glob(filepath.Join("braaam", "editor", "*.java"))
	stale = append(stale, javaStale...)
	cljStale, _ := filepath.Glob(filepath.Join("vijure", "src", "whim", "editor", "*.clj"))
	stale = append(stale, cljStale...)
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
		if err := os.MkdirAll(filepath.Dir(f.tracked), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "whim: %v\n", err)
			return 1
		}
		if err := os.WriteFile(f.tracked, made, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "whim: %v\n", err)
			return 1
		}
		changed = true
	}
	for _, p := range stale {
		if made[p] {
			continue
		}
		if check {
			fmt.Printf("  %-12s is NOT what the generator writes from whim-vim.c (it writes no such part).  Run: make editor/editor.go\n", p)
			fail = true
			continue
		}
		if err := os.Remove(p); err != nil {
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
		fmt.Printf("  %-12s is what the Java backend writes from whim-vim.c\n", "whim.editor")
		fmt.Printf("  %-12s is what the Clojure backend writes from whim-vim.c\n", "editor.clj")
		fmt.Printf("  %-12s is what the Haskell backend writes from whim-vim.c\n", "Editor.hs")
		fmt.Printf("  %-12s is what the Rust backend writes from whim-vim.c\n", "editor.rs")
		fmt.Printf("  %-12s is what the Scheme backend writes from whim-vim.c\n", "editor.ss")
	case changed:
		b, _ := os.ReadFile("editor/editor.go")
		fmt.Printf("  %-12s %d lines, generated from whim-vim.c\n", "editor.go", bytes.Count(b, []byte("\n")))
		j, _ := os.ReadFile("braaam/editor/Editor.java")
		n := 0
		for _, f := range javaFiles {
			b, _ := os.ReadFile(filepath.Join("braaam", "editor", f))
			n += bytes.Count(b, []byte("\n"))
		}
		fmt.Printf("  %-12s %d lines, and %d in %d files beside it (whim.editor), generated from whim-vim.c\n", "Editor.java", bytes.Count(j, []byte("\n")), n-bytes.Count(j, []byte("\n")), len(javaFiles)-1)
		c, _ := os.ReadFile("vijure/src/whim/editor.clj")
		cn := 0
		for _, p := range cljParts {
			b, _ := os.ReadFile(filepath.Join("vijure", "src", "whim", p))
			cn += bytes.Count(b, []byte("\n"))
		}
		fmt.Printf("  %-12s %d lines, and %d in the %d parts it loads, generated from whim-vim.c\n", "editor.clj", bytes.Count(c, []byte("\n")), cn, len(cljParts))
		n = 0
		for _, f := range hsFiles {
			hs, _ := os.ReadFile(filepath.Join("caprice", "Caprice", f))
			n += bytes.Count(hs, []byte("\n"))
		}
		fmt.Printf("  %-12s %d lines in %d files, generated from whim-vim.c\n", "Editor.hs", n, len(hsFiles))
		rs, _ := os.ReadFile("whimsy/src/editor.rs")
		fmt.Printf("  %-12s %d lines, generated from whim-vim.c\n", "editor.rs", bytes.Count(rs, []byte("\n")))
		ss, _ := os.ReadFile("whimsical/whimsical/editor.ss")
		fmt.Printf("  %-12s %d lines, generated from whim-vim.c\n", "editor.ss", bytes.Count(ss, []byte("\n")))
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

// cutCore cuts the core of the whim-vim.c at file into dir/editor.c, says
// so, and returns the path.
func cutCore(file, dir string) (string, error) {
	c, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	core, err := whim.Cut(c)
	if err != nil {
		return "", fmt.Errorf("%s: %v", file, err)
	}
	p := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(p, core, 0o644); err != nil {
		return "", err
	}
	fmt.Printf("  %-12s %s lines, cut at the first #include of %s\n", "core", commas(bytes.Count(core, []byte("\n"))), commas(bytes.Count(c, []byte("\n"))))
	return p, nil
}

// commas is n with a comma every three digits.
func commas(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// runCut prints the core of a whim-vim.c -- src/whim-vim.c, or FILE -- on
// stdout: what every translation is written from, to read, or to hand
// `whim skel` and `whim pre`.
//
//	whim cut [FILE]
func runCut(args []string) int {
	file := "src/whim-vim.c"
	switch len(args) {
	case 0:
	case 1:
		file = args[0]
	default:
		fmt.Fprintln(os.Stderr, "usage: whim cut [FILE]")
		return 2
	}
	c, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim cut: %v\n", err)
		return 1
	}
	core, err := whim.Cut(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim cut: %s: %v\n", file, err)
		return 1
	}
	os.Stdout.Write(core)
	return 0
}
