package suite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/arbace/go-whim/whimsy"
)

// THE RUST EDITOR (whimsy/, doc/RUST.md), on demand: `whim test --rust`.
// The candidate's core is written as the module `editor` by the Rust
// backend and compiled by cargo with whimsy's runtime, host and launcher,
// and held to what the Java, the Clojure and the Haskell editors are held
// to: every case answered as the C candidate answers it, and a control of
// its own -- " INSERT" changed in the generated editor.rs -- that must move
// whimsy's own answers.
//
// The two builds are kept in .cache/whimsy-suite/, where cargo's
// fingerprints skip a crate whose sources have not moved: a run on an
// unchanged core pays for neither.

// rsControlOld is the control string as the module writes it: a C-string
// literal, its NUL implied (doc/RUST-IDIOMS.md, item 9).
const rsControlOld, rsControlNew = `c" INSERT"`, `c" INSERX"`

// rsCache is where the suite's two Rust builds are kept between runs.
var rsCache = filepath.Join(".cache", "whimsy-suite")

// buildRust builds the Rust editor from candSrc, and its control: the
// module generated once, the two crates compiled side by side.
func buildRust(gen whimsy.Gen, candSrc string) (*jvmEditor, error) {
	e := &jvmEditor{name: "Rust", where: "whimsy/", file: "editor.rs", launcher: "whimsy",
		frame: regexp.MustCompile(`$^`)}
	cdir, ctlDir := filepath.Join(rsCache, "cand"), filepath.Join(rsCache, "control")
	rsOut, err := whimsy.Generate(gen, candSrc, cdir)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(rsOut)
	if err != nil {
		return nil, err
	}
	if n := bytes.Count(src, []byte(rsControlOld)); n != 1 {
		return nil, fmt.Errorf("suite: the control string %s is in the generated editor.rs %d times, not once", rsControlOld, n)
	}
	ctl := filepath.Join(ctlDir, "src", "editor.rs")
	if err := os.MkdirAll(filepath.Dir(ctl), 0o755); err != nil {
		return nil, err
	}
	b := bytes.Replace(src, []byte(rsControlOld), []byte(rsControlNew), 1)
	if have, err := os.ReadFile(ctl); err != nil || !bytes.Equal(have, b) {
		if err := os.WriteFile(ctl, b, 0o644); err != nil {
			return nil, err
		}
	}
	var wg sync.WaitGroup
	var errBin, errCtl error
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.bin, _, errBin = whimsy.Compile(cdir, filepath.Join(cdir, "whimsy"))
	}()
	go func() {
		defer wg.Done()
		e.ctl, _, errCtl = whimsy.Compile(ctlDir, filepath.Join(ctlDir, "whimsy-control"))
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
