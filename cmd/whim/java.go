package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/jeditor"
)

// javaGen is the Java backend as `whim skel <editor.c> <dir> -java <out>`
// runs it, with the profile `whim gen` uses: what jeditor.Build and the
// suite's --java are handed.
func javaGen(editorC, dir, javaOut string) error {
	prof, err := genProfile()
	if err != nil {
		return err
	}
	var log bytes.Buffer
	if rc := togo.Run([]string{editorC, dir, "-java", javaOut}, &log, prof); rc != 0 {
		return fmt.Errorf("the Java backend on %s: status %d\n%s", editorC, rc, log.Bytes())
	}
	return nil
}

// runJava builds the editor in Java (jeditor/) from a whim-vim.c: the core
// cut from it and written as Editor.java, compiled with the runtime, the host
// and the glue, and a launcher that runs it as a binary.
//
//	whim java [--out DIR] [FILE]
//
// FILE is src/whim-vim.c by default and DIR bin/java; the launcher is
// bin/whim-java, beside DIR.
func runJava(args []string) int {
	file, out := "src/whim-vim.c", filepath.Join("bin", "java")
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			file = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim java [--out DIR] [FILE]")
			return 2
		}
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "whim java: %v\n", err)
		return 1
	}
	link := filepath.Join(filepath.Dir(out), "whim-java")
	if _, err := jeditor.Build(javaGen, file, out, link, nil); err != nil {
		fmt.Fprintf(os.Stderr, "whim java: %v\n", err)
		return 1
	}
	fmt.Printf("  %-12s the core in Java (jeditor/), classes in %s\n", link, filepath.Join(out, "classes"))
	return 0
}
