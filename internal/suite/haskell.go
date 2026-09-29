package suite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/arbace/go-whim/caprice"
)

// THE HASKELL EDITOR (caprice/, doc/HASKELL.md), on demand: `whim test
// --haskell`.  The candidate's core is written as the module Caprice.Editor
// by the Haskell backend and compiled by GHC with caprice's runtime, host and
// launcher, and held to what the Java and the Clojure editors are held to:
// every case answered as the C candidate answers it, and a control of its
// own -- " INSERT" changed in the generated Editor.hs -- that must move
// caprice's own answers.
//
// The core is one module, three minutes of GHC's time, so the two builds are
// kept in .cache/caprice-suite/, where GHC's recompilation check skips a
// module whose source has not moved: a run on an unchanged core pays for
// neither.

// hsControlOld is the control string as the module writes it: a primitive
// string literal, its NUL written in.
const hsControlOld, hsControlNew = `" INSERT\0"#`, `" INSERX\0"#`

// hsCache is where the suite's two Haskell builds are kept between runs.
var hsCache = filepath.Join(".cache", "caprice-suite")

// buildHaskell builds the Haskell editor from candSrc, and its control:
// the module generated once, the two compiled side by side.
func buildHaskell(gen caprice.Gen, candSrc string) (*jvmEditor, error) {
	e := &jvmEditor{name: "Haskell", where: "caprice/", file: "Editor.hs", launcher: "caprice",
		frame: regexp.MustCompile(`$^`)}
	cdir, ctlDir := filepath.Join(hsCache, "cand"), filepath.Join(hsCache, "control")
	hsOut, err := caprice.Generate(gen, candSrc, cdir)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(hsOut)
	if err != nil {
		return nil, err
	}
	if n := bytes.Count(src, []byte(hsControlOld)); n != 1 {
		return nil, fmt.Errorf("suite: the control string %s is in Editor.hs %d times, not once", hsControlOld, n)
	}
	// the control: the same sources, the one literal changed
	ctlSrc := filepath.Join(ctlDir, "src", "Caprice")
	if err := os.MkdirAll(ctlSrc, 0o755); err != nil {
		return nil, err
	}
	boot, err := os.ReadFile(filepath.Join(filepath.Dir(hsOut), "Editor.hs-boot"))
	if err != nil {
		return nil, err
	}
	for _, f := range []struct {
		name string
		b    []byte
	}{{"Editor.hs", bytes.Replace(src, []byte(hsControlOld), []byte(hsControlNew), 1)}, {"Editor.hs-boot", boot}} {
		p := filepath.Join(ctlSrc, f.name)
		if have, err := os.ReadFile(p); err != nil || !bytes.Equal(have, f.b) {
			if err := os.WriteFile(p, f.b, 0o644); err != nil {
				return nil, err
			}
		}
	}
	var wg sync.WaitGroup
	var errBin, errCtl error
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.bin, errBin = caprice.Compile(cdir, filepath.Join(cdir, "caprice"))
	}()
	go func() {
		defer wg.Done()
		e.ctl, errCtl = caprice.Compile(ctlDir, filepath.Join(ctlDir, "caprice-control"))
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
