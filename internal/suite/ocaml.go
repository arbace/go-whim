package suite

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/arbace/go-whim/whiml"
)

// THE OCAML EDITOR (whiml/, doc/OCAML.md), on demand: `whim test --ocaml`.
// The candidate's core is written as the module Editor by the OCaml
// backend and compiled by ocamlopt with whiml's runtime, host and launcher,
// and held to what the other editors are held to: every case answered as
// the C candidate answers it, and a control of its own -- " INSERT"
// changed in the generated editor.ml -- that must move whiml's own
// answers.
//
// The two builds are kept in .cache/whiml-suite/, where a module whose
// source has not moved is not compiled again: a run on an unchanged core
// pays for neither.

// mlControl is the control string as the module writes it: the bytes of
// the literal in the memory's image, a NUL before and after it.
var mlControl = regexp.MustCompile(`(\\x00|")( INSERT)(\\x00)`)

// mlCache is where the suite's two OCaml builds are kept between runs.
var mlCache = filepath.Join(".cache", "whiml-suite")

// buildOCaml builds the OCaml editor from candSrc, and its control: the
// module generated once, the two compiled side by side.
func buildOCaml(gen whiml.Gen, candSrc string) (*jvmEditor, error) {
	e := &jvmEditor{name: "OCaml", where: "whiml/", file: "editor.ml", launcher: "whiml",
		frame: regexp.MustCompile(`$^`)}
	cdir, ctlDir := filepath.Join(mlCache, "cand"), filepath.Join(mlCache, "control")
	mlOut, err := whiml.Generate(gen, candSrc, cdir)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(mlOut)
	if err != nil {
		return nil, err
	}
	if n := len(mlControl.FindAllIndex(src, -1)); n != 1 {
		return nil, fmt.Errorf("suite: the control string %q is in the generated editor.ml %d times, not once", " INSERT", n)
	}
	ctl := filepath.Join(ctlDir, "src", "editor.ml")
	if err := os.MkdirAll(filepath.Dir(ctl), 0o755); err != nil {
		return nil, err
	}
	b := mlControl.ReplaceAll(src, []byte("${1} INSERX${3}"))
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
		e.bin, _, errBin = whiml.Compile(cdir, filepath.Join(cdir, "whiml"))
	}()
	go func() {
		defer wg.Done()
		e.ctl, _, errCtl = whiml.Compile(ctlDir, filepath.Join(ctlDir, "whiml-control"))
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
