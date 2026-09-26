package suite

import (
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/arbace/go-whim/vijure"
)

// THE CLOJURE EDITOR (vijure/, doc/CLOJURE.md), on demand: `whim test
// --clojure`.  The candidate's core is written as the namespace whim.editor
// by the Clojure backend, AOT-compiled with vijure's glue and launcher on
// the Java editor's runtime and host, and held to what the Java editor is
// (java.go): every case answered as the C candidate answers it, and a
// control of its own -- " INSERT" changed in the generated editor.clj --
// that must move the Clojure editor's own answers.

// buildClojure builds the Clojure editor from candSrc, and its control,
// under dir: the namespace generated once, the two compiled side by side.
func buildClojure(gen vijure.Gen, candSrc, dir string) (*jvmEditor, error) {
	e := &jvmEditor{name: "Clojure", where: "vijure/", file: "editor.clj", launcher: "vijure",
		frame: regexp.MustCompile(`^\tat whim\.editor\$([A-Za-z0-9_]+)`)}
	cdir, ctlDir := filepath.Join(dir, "clj"), filepath.Join(dir, "clj-control")
	cljOut, err := vijure.Generate(gen, candSrc, cdir)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(cljOut)
	if err != nil {
		return nil, err
	}
	ctlSrc, err := writeControl(src, "editor.clj", ctlDir)
	if err != nil {
		return nil, err
	}
	return compileClojure(e, cljOut, ctlSrc, dir)
}

// compileClojure compiles the namespace in cljOut and its control in ctlSrc,
// side by side, into e's two launchers under dir.
func compileClojure(e *jvmEditor, cljOut, ctlSrc, dir string) (*jvmEditor, error) {
	var wg sync.WaitGroup
	var errBin, errCtl error
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.bin, errBin = vijure.Compile(cljOut, filepath.Dir(cljOut), filepath.Join(dir, "vijure"))
	}()
	go func() {
		defer wg.Done()
		e.ctl, errCtl = vijure.Compile(ctlSrc, filepath.Dir(ctlSrc), filepath.Join(dir, "vijure-control"))
	}()
	wg.Wait()
	if errBin != nil {
		return nil, errBin
	}
	if errCtl != nil {
		return nil, errCtl
	}
	return e, nil
}
