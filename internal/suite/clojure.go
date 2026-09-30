package suite

import (
	"bytes"
	"fmt"
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
	ctlSrc, err := cljControl(cljOut, ctlDir)
	if err != nil {
		return nil, err
	}
	return compileClojure(e, cljOut, ctlSrc, dir)
}

// cljControl writes the namespace in cljOut, and the parts it loads, into
// dir with the control applied to the one file that holds the control's
// string, and returns the namespace's file there.
func cljControl(cljOut, dir string) (string, error) {
	parts, err := vijure.Parts(cljOut)
	if err != nil {
		return "", err
	}
	files := append([]string{"editor.clj"}, parts...)
	holder := ""
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(cljOut), f))
		if err != nil {
			return "", err
		}
		if bytes.Contains(b, []byte(javaControlOld)) {
			if holder != "" {
				return "", fmt.Errorf("suite: the control string %s is in %s and %s", javaControlOld, holder, f)
			}
			holder = f
		}
	}
	if holder == "" {
		return "", fmt.Errorf("suite: the control string %s is in no file of the namespace", javaControlOld)
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(cljOut), f))
		if err != nil {
			return "", err
		}
		dst := filepath.Join(dir, f)
		if f == holder {
			if _, err := writeControl(b, filepath.Base(f), filepath.Dir(dst)); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "editor.clj"), nil
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
