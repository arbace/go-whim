package suite

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/arbace/go-whim/whimsical"
)

// THE SCHEME EDITOR (whimsical/, doc/SCHEME.md), on demand: `whim test
// --scheme`.  The candidate's core is written as the library (whimsical
// editor) by the Scheme backend and compiled by Chez Scheme with
// whimsical's runtime, host and launcher, and held to what the Java, the
// Clojure, the Haskell and the Rust editors are held to: every case
// answered as the C candidate answers it, and a control of its own --
// " INSERT" changed in the generated editor.ss -- that must move
// whimsical's own answers.
//
// The two builds are kept in .cache/whimsical-suite/, where a library whose
// source has not moved is not compiled again: a run on an unchanged core
// pays for neither.  `--scheme-debug` runs the debugging build instead
// (whimsical.Debug: optimize-level 2, safe, the inspector's information
// kept), its two builds kept in .cache/whimsical-suite-debug/.

// scmControl is the control string as the library writes it: the bytes of
// the literal in the memory's image, a NUL before and after it.
var scmControl = regexp.MustCompile(`(\\x0;|")( INSERT)(\\x0;)`)

// scmCache is where the suite's two Scheme builds are kept between runs,
// and scmDebugCache the debugging build's two.
var scmCache, scmDebugCache = filepath.Join(".cache", "whimsical-suite"), filepath.Join(".cache", "whimsical-suite-debug")

// buildScheme builds the Scheme editor from candSrc, and its control: the
// library generated once, the two compiled side by side in mode m.
func buildScheme(gen whimsical.Gen, candSrc string, m whimsical.Mode) (*jvmEditor, error) {
	e := &jvmEditor{name: "Scheme", where: "whimsical/", file: "editor.ss", launcher: "whimsical",
		frame: regexp.MustCompile(`$^`)}
	cache := scmCache
	if m == whimsical.Debug {
		e.name, e.launcher, cache = "Scheme (debug)", "whimsical-debug", scmDebugCache
	}
	cdir, ctlDir := filepath.Join(cache, "cand"), filepath.Join(cache, "control")
	scmOut, err := whimsical.Generate(gen, candSrc, cdir)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(scmOut)
	if err != nil {
		return nil, err
	}
	if n := len(scmControl.FindAllIndex(src, -1)); n != 1 {
		return nil, fmt.Errorf("suite: the control string %q is in the generated editor.ss %d times, not once", " INSERT", n)
	}
	ctl := filepath.Join(ctlDir, "src", "whimsical", "editor.ss")
	if err := os.MkdirAll(filepath.Dir(ctl), 0o755); err != nil {
		return nil, err
	}
	b := scmControl.ReplaceAll(src, []byte("${1} INSERX${3}"))
	if have, err := os.ReadFile(ctl); err != nil || string(have) != string(b) {
		if err := os.WriteFile(ctl, b, 0o644); err != nil {
			return nil, err
		}
	}
	var wg sync.WaitGroup
	var errBin, errCtl error
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.bin, _, errBin = whimsical.Compile(cdir, filepath.Join(cdir, "whimsical"), m)
	}()
	go func() {
		defer wg.Done()
		e.ctl, _, errCtl = whimsical.Compile(ctlDir, filepath.Join(ctlDir, "whimsical-control"), m)
	}()
	wg.Wait()
	if errBin != nil {
		return nil, errBin
	}
	if errCtl != nil {
		return nil, errCtl
	}
	for _, p := range []*string{&e.bin, &e.ctl} {
		abs, err := filepath.Abs(*p)
		if err != nil {
			return nil, err
		}
		*p = abs
	}
	return e, nil
}
