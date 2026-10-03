package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arbace/go-whim/caprice"
	"github.com/arbace/go-whim/crefactor/togo"
	"github.com/arbace/go-whim/internal/suite"
)

// hscatLimit is how long one run of a case may take on the program hscat
// builds, against the suite's 10 s: at -O0 the par_* cases run past that.
const hscatLimit = 2 * time.Minute

// runHscat is the Haskell editor's core as ONE module, as gocat is the Go
// editor as one file: the Haskell backend with the profile `whim gen` uses
// but HsParts 1, so Caprice.Editor is not split into Defs and parts.
//
//	whim hscat [--ghc GHC] [--hsl FILE] [--out DIR] [--no-test] [SRC]
//
// With no --ghc it prints the module. With --ghc, GHC is ghc-lisp's ghc
// (github.com/arbace/ghc-lisp, doc/GHC-LISP.md): the module is converted to
// ghc-lisp's s-expressions (--hs2lisp), the round trip is checked
// (--lisp-check: Haskell, Lisp, Haskell give the same tree), and the .hsl,
// in the module's place, is compiled by that ghc with caprice's runtime, host
// and launcher into DIR/caprice, and that program is run on the quick suite
// (whim test --haskell-bin DIR/caprice --limit 2m, against HEAD) -- the proof it
// is the editor; --no-test skips the suite. The .hsl is written to FILE
// (stdout when no --hsl) only when all of it passed. SRC is src/whim-vim.c
// by default; DIR a temporary directory, removed after, unless --out names
// one.
func runHscat(args []string) int {
	src, ghc, hsl, out, test := "src/whim-vim.c", "", "", "", true
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
		case args[i] == "--no-test":
			test = false
		case len(args[i]) > 0 && args[i][0] != '-':
			src = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim hscat [--ghc GHC] [--hsl FILE] [--out DIR] [--no-test] [SRC]")
			return 2
		}
	}
	if err := hscat(src, ghc, hsl, out, test); err != nil {
		fmt.Fprintf(os.Stderr, "whim hscat: %v\n", err)
		return 1
	}
	return 0
}

func hscat(src, ghc, hsl, out string, test bool) error {
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
	// looks for a .hsl module's: in Lisp, converted and checked as the module
	// is, where ghc-lisp reads an .hsl-boot as Lisp (arbace/ghc-lisp#1), and
	// in Haskell before, when its parser took a file for Lisp only when the
	// name ended in .hsl (GHC.Parser.Lisp's isLispFile).
	if boot := hs + "-boot"; exists(boot) {
		to := hs[:len(hs)-len(".hs")] + ".hsl-boot"
		lispBoot, err := lispBoots(ghc)
		if err != nil {
			return err
		}
		if lispBoot {
			_, err = toLisp(ghc, boot, to)
		} else {
			err = os.Rename(boot, to)
		}
		if err != nil {
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
	// And it answers as the editor does: the quick suite, the program held to
	// the C on every case with its control its own bytes, one literal changed.
	// At -O0 the par_* cases that build 3,000 lines by keys run past the
	// suite's 10 s, so a run may take hscatLimit; the heavy case times it but,
	// for a program built already, holds it to no bound.
	if test {
		if err := suite.Check(os.Stderr, "HEAD", src, suite.JVM{HaskellBin: prog, Limit: hscatLimit}); err != nil {
			return fmt.Errorf("the program %s ghc-lisp built is not the editor: %v", prog, err)
		}
	}
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

// lispBoots reports whether ghc reads an .hsl-boot as Lisp: it type-checks
// two modules whose import cycle a boot file in Lisp breaks.
func lispBoots(ghc string) (bool, error) {
	dir, err := os.MkdirTemp("", "hslboot")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	for name, text := range map[string]string{
		"A.hsl":      "(module A [a])\n\n(import :source B [b])\n\n(:: a Int)\n(= a b)\n",
		"B.hsl":      "(module B [b])\n\n(import A [a])\n\n(:: b Int)\n(= b 1)\n",
		"B.hsl-boot": "(module B [b])\n\n(:: b Int)\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			return false, err
		}
	}
	cmd := exec.Command(ghc, "--make", "-fno-code", "B.hsl")
	cmd.Dir = dir
	return cmd.Run() == nil, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
