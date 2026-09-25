package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/cljeditor"
	"github.com/arbace/go-whim/crefactor/togo"
)

// cljGen is the Clojure backend as `whim skel <editor.c> <dir> -clj <out>`
// runs it, with the profile `whim gen` uses: what cljeditor.Build and the
// suite's --clojure are handed.  A toolset without the backend writes no
// file, which cljeditor.Build reports as cljeditor.ErrNoBackend.
func cljGen(editorC, dir, cljOut string) error {
	prof, err := genProfile()
	if err != nil {
		return err
	}
	var log bytes.Buffer
	if rc := togo.Run([]string{editorC, dir, "-clj", cljOut}, &log, prof); rc != 0 {
		return fmt.Errorf("the Clojure backend on %s: status %d\n%s", editorC, rc, log.Bytes())
	}
	return nil
}

// copyGen is a Gen that generates nothing: it copies the namespace in file,
// written already -- `whim clj --editor` and `whim test --clojure-editor`.
func copyGen(file string) cljeditor.Gen {
	return func(_, _, cljOut string) error {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return os.WriteFile(cljOut, b, 0o644)
	}
}

// runClj builds the editor in Clojure (cljeditor/) from a whim-vim.c: the
// core cut from it and written as the namespace whim.editor, AOT-compiled
// with the glue, the launcher and the Java editor's runtime and host, and a
// launcher that runs it as a binary.
//
//	whim clj [--out DIR] [--editor editor.clj] [--jar FILE] [FILE]
//
// FILE is src/whim-vim.c by default and DIR lib/clj, where the sources, the
// classes and Clojure's jars go; the launcher is bin/whim-clj -- or
// DIR/whim-clj when DIR is given.  --editor compiles the namespace in that
// file instead of generating one (FILE is then not read); --jar also writes
// the whole as one executable jar.
func runClj(args []string) int {
	file, out, link := "src/whim-vim.c", filepath.Join("lib", "clj"), filepath.Join("bin", "whim-clj")
	editor, jar := "", ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
			link = filepath.Join(out, "whim-clj")
		case args[i] == "--editor" && i+1 < len(args):
			i++
			editor = args[i]
		case args[i] == "--jar" && i+1 < len(args):
			i++
			jar = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim clj [--out DIR] [--editor editor.clj] [--jar FILE] [FILE]")
			return 2
		}
	}
	fail := func(err error) int {
		fmt.Fprintf(os.Stderr, "whim clj: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return fail(err)
	}
	var err error
	if editor != "" {
		_, err = cljeditor.Compile(editor, out, link)
	} else {
		_, err = cljeditor.Build(cljGen, file, out, link, nil)
	}
	if err != nil {
		return fail(err)
	}
	cache := "no AOT cache: this JDK wrote none (JDK 25 and later do)"
	if _, err := os.Stat(filepath.Join(out, cljeditor.CacheName)); err == nil {
		cache = "its AOT cache " + filepath.Join(out, cljeditor.CacheName)
	}
	fmt.Printf("  %-12s the core in Clojure (cljeditor/): %s, %s\n", link, filepath.Join(out, cljeditor.JarName), cache)
	if jar != "" {
		if err := cljeditor.Jar(out, jar); err != nil {
			return fail(err)
		}
		st, err := os.Stat(jar)
		if err != nil {
			return fail(err)
		}
		fmt.Printf("  %-12s %d bytes, the core in Clojure (cljeditor/): java -jar %s\n", jar, st.Size(), jar)
	}
	return 0
}
