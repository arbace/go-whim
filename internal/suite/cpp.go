package suite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/arbace/go-whim/wpp"
)

// THE C++ EDITOR (wpp/, doc/CPP.md), on demand: `whim test --cpp`.  The
// candidate's core is written as editor.hpp and editor.cpp by the C++
// backend and compiled by g++ with whim++'s runtime, host and launcher,
// and held to what the other editors are held to: every case answered as
// the C candidate answers it, and a control of its own -- " INSERT"
// changed in the generated editor.cpp -- that must move whim++'s own
// answers.
//
// The two builds are kept in .cache/wpp-suite/, where a unit whose source
// has not moved is not compiled again: a run on an unchanged core pays for
// neither.

// cppControl is the control string as the source writes it.
var cppControl = []byte(`" INSERT"`)

// cppCache is where the suite's two C++ builds are kept between runs.
var cppCache = filepath.Join(".cache", "wpp-suite")

// buildCpp builds the C++ editor from candSrc, and its control: the source
// generated once, the two compiled side by side.
func buildCpp(gen wpp.Gen, candSrc string) (*jvmEditor, error) {
	e := &jvmEditor{name: "C++", where: "wpp/", file: "editor.cpp", launcher: wpp.Program,
		frame: regexp.MustCompile(`$^`)}
	cdir, ctlDir := filepath.Join(cppCache, "cand"), filepath.Join(cppCache, "control")
	out, err := wpp.Generate(gen, candSrc, cdir)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	if n := bytes.Count(src, cppControl); n != 1 {
		return nil, fmt.Errorf("suite: the control string %s is in the generated editor.cpp %d times, not once", cppControl, n)
	}
	hdr, err := os.ReadFile(filepath.Join(filepath.Dir(out), "editor.hpp"))
	if err != nil {
		return nil, err
	}
	ctl := filepath.Join(ctlDir, "src", "editor.cpp")
	if err := os.MkdirAll(filepath.Dir(ctl), 0o755); err != nil {
		return nil, err
	}
	for path, b := range map[string][]byte{
		filepath.Join(ctlDir, "src", "editor.hpp"): hdr,
		ctl: bytes.Replace(src, cppControl, []byte(`" INSERX"`), 1),
	} {
		if have, err := os.ReadFile(path); err != nil || !bytes.Equal(have, b) {
			if err := os.WriteFile(path, b, 0o644); err != nil {
				return nil, err
			}
		}
	}
	var wg sync.WaitGroup
	var errBin, errCtl error
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.bin, _, errBin = wpp.Compile(cdir, filepath.Join(cdir, wpp.Program))
	}()
	go func() {
		defer wg.Done()
		e.ctl, _, errCtl = wpp.Compile(ctlDir, filepath.Join(ctlDir, wpp.Program+"-control"))
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
