package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arbace/go-whim/caprice"
	"github.com/arbace/go-whim/crefactor/togo"
)

// runHscat is the Haskell editor's core as ONE module, as gocat is the Go
// editor as one file: the Haskell backend with the profile `whim gen` uses
// but HsParts 1, so Caprice.Editor is not split into Defs and parts.
//
//	whim hscat [--ghc GHC] [--hsl FILE] [--out DIR] [SRC]
//
// With no --ghc it prints the module. With --ghc, GHC is ghc-lisp's ghc
// (github.com/arbace/ghc-lisp, doc/GHC-LISP.md): the module is converted to
// ghc-lisp's s-expressions (--hs2lisp), the round trip is checked
// (--lisp-check: Haskell, Lisp, Haskell give the same tree), and the .hsl,
// in the module's place, is compiled by that ghc with caprice's runtime, host
// and launcher into DIR/caprice -- the proof it is the editor. The .hsl is
// written to FILE (stdout when no --hsl). SRC is src/whim-vim.c by default;
// DIR a temporary directory, removed after, unless --out names one.
func runHscat(args []string) int {
	src, ghc, hsl, out := "src/whim-vim.c", "", "", ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--ghc" && i+1 < len(args):
			i++
			ghc = args[i]
		case args[i] == "--hsl" && i+1 < len(args):
			i++
			hsl = args[i]
		case args[i] == "--out" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			src = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim hscat [--ghc GHC] [--hsl FILE] [--out DIR] [SRC]")
			return 2
		}
	}
	if err := hscat(src, ghc, hsl, out); err != nil {
		fmt.Fprintf(os.Stderr, "whim hscat: %v\n", err)
		return 1
	}
	return 0
}

func hscat(src, ghc, hsl, out string) error {
	dir := out
	if dir == "" {
		d, err := os.MkdirTemp("", "hscat.")
		if err != nil {
			return err
		}
		defer os.RemoveAll(d)
		dir = d
	}
	one := func(editorC, scratch, hsOut string) error {
		prof, err := genProfile()
		if err != nil {
			return err
		}
		prof.HsParts = 1
		var log bytes.Buffer
		if rc := togo.Run([]string{editorC, scratch, "-hs", hsOut}, &log, prof); rc != 0 {
			return fmt.Errorf("the Haskell backend on %s: status %d\n%s", editorC, rc, log.Bytes())
		}
		return nil
	}
	hs, err := caprice.Generate(one, src, dir)
	if err != nil {
		return err
	}
	if ghc == "" {
		b, err := os.ReadFile(hs)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(b)
		return err
	}
	// The module converted, its round trip checked, and put in the Haskell's
	// place: ghc's finder takes Caprice/Editor.hsl where it looked for
	// Caprice/Editor.hs.
	lisp, err := toLisp(ghc, hs, hs[:len(hs)-len(".hs")]+".hsl")
	if err != nil {
		return err
	}
	// The boot file goes beside it as Editor.hsl-boot, where ghc-lisp's finder
	// looks for a .hsl module's, but in Haskell: ghc-lisp's parser takes a
	// file for Lisp only when its name ends in .hsl (GHC.Parser.Lisp's
	// isLispFile), so it reads an .hsl-boot as Haskell.
	if boot := hs + "-boot"; exists(boot) {
		if err := os.Rename(boot, hs[:len(hs)-len(".hs")]+".hsl-boot"); err != nil {
			return err
		}
	}
	// The proof that it is the editor is that it compiles and runs, not its
	// speed: at -O1 GHC takes nine minutes and 22.6 GB on the one module, at
	// -O0 a fraction of both.
	caprice.GHC = ghc
	caprice.GHCFlags = append([]string{"-O0"}, caprice.GHCFlags[1:]...)
	prog, st, err := caprice.CompileStats(dir, filepath.Join(dir, "caprice"), "")
	if err != nil {
		return fmt.Errorf("%s does not compile the .hsl with caprice's runtime: %v", ghc, err)
	}
	fmt.Fprintf(os.Stderr, "  caprice.hsl  %d lines; --lisp-check OK; compiled with caprice's runtime into %s (%s)\n",
		bytes.Count(lisp, []byte{'\n'}), prog, st)
	if hsl == "" {
		_, err = os.Stdout.Write(lisp)
		return err
	}
	return os.WriteFile(hsl, lisp, 0o644)
}

// toLisp converts the Haskell file hs with ghc-lisp's ghc, requires its round
// trip to give the same tree, writes the Lisp at to and removes hs.
func toLisp(ghc, hs, to string) ([]byte, error) {
	lisp, err := exec.Command(ghc, "--hs2lisp", hs).Output()
	if err != nil {
		return nil, fmt.Errorf("%s --hs2lisp %s: %v", ghc, hs, err)
	}
	check, err := exec.Command(ghc, "--lisp-check", hs).CombinedOutput()
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(check)), "OK") {
		return nil, fmt.Errorf("%s --lisp-check %s: the round trip does not give the same tree: %v\n%s", ghc, hs, err, check)
	}
	if err := os.WriteFile(to, lisp, 0o644); err != nil {
		return nil, err
	}
	return lisp, os.Remove(hs)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
