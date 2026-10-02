// Package whimsical builds the editor in Scheme (doc/SCHEME.md): the core,
// the R6RS library (whimsical editor), written by crefactor/togo's Scheme
// backend from the core half of a whim-vim.c, against the runtime kept here
// by hand (whimsical/rt.ss: C's memory as one bytevector, a pointer an
// offset into it).
package whimsical

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/internal/whim"
)

// Gen writes the library (whimsical editor) of the C core editorC to scmOut
// (with its .refused and .host beside it), as `whim skel <editorC> <dir>
// -scm <scmOut>` does.
type Gen func(editorC, dir, scmOut string) error

// ErrNoBackend is what Build says when the generator wrote nothing.
var ErrNoBackend = errors.New("whimsical: the generator wrote no editor.ss -- this toolset has no Scheme backend (`whim skel <editor.c> <dir> -scm <out.ss>`)")

// Generate cuts the core from src into dir/editor.c and writes its library
// at dir/src/whimsical/editor.ss (only when it differs), whose path it
// returns.
// A core the backend refused any part of is refused.
func Generate(gen Gen, src, dir string) (string, error) {
	c, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	core, err := whim.Cut(c)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "whimsical"), 0o755); err != nil {
		return "", err
	}
	editorC := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(editorC, core, 0o644); err != nil {
		return "", err
	}
	scratch, err := os.MkdirTemp("", "scmgen.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	made := filepath.Join(scratch, "editor.ss")
	if err := gen(editorC, scratch, made); err != nil {
		return "", err
	}
	b, err := os.ReadFile(made)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrNoBackend
	} else if err != nil {
		return "", err
	}
	if r, err := os.ReadFile(made + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		return "", fmt.Errorf("whimsical: the Scheme backend refused part of the core:\n%s", r)
	}
	scmOut := filepath.Join(dir, "src", "whimsical", "editor.ss")
	if err := writeIfDiffers(scmOut, b); err != nil {
		return "", err
	}
	return scmOut, nil
}

// writeIfDiffers writes b to path unless path holds it already, keeping the
// file's time.
func writeIfDiffers(path string, b []byte) error {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}
