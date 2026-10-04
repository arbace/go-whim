// Package whiml builds the editor in OCaml (doc/OCAML.md): the core, the
// module Editor (editor.ml), written by crefactor/togo's OCaml backend from
// the core half of a whim-vim.c, and beside it the OCaml kept here by hand
// -- the runtime (rt.ml), vim's printf (snprintf.ml), the host (host.ml:
// the C host's functions as glue to a host record; term.ml, the terminal,
// with the C host's own calls OCaml's Unix has not in term_stubs.c) and the
// launcher (main.ml) -- compiled by ocamlopt into a native program:
// `whiml [args]`.
//
// The hand-written sources are embedded, as whimsical's Scheme and whimsy's
// Rust are, and written into the build directory only when they differ, so
// that a module whose source has not moved is not compiled again.
package whiml

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/whim"
)

//go:embed rt.ml snprintf.ml host.ml term.ml main.ml c/term_stubs.c
var sources embed.FS

// Gen writes the module Editor of the C core editorC to mlOut (with its
// .refused and .layout beside it), as `whim skel <editorC> <dir> -ml
// <mlOut>` does.
type Gen func(editorC, dir, mlOut string) error

// ErrNoBackend is what Build says when the generator wrote nothing.
var ErrNoBackend = errors.New("whiml: the generator wrote no editor.ml -- this toolset has no OCaml backend (`whim skel <editor.c> <dir> -ml <out.ml>`)")

// Stats is what compiling the core cost ocamlopt: the wall time and the
// peak resident memory, as wait4 reports them; zero when the core had not
// moved.
type Stats struct {
	Wall time.Duration
	Peak int64 // bytes
}

func (s Stats) String() string {
	if s.Wall == 0 {
		return "the core's module current"
	}
	return fmt.Sprintf("ocamlopt %.1f s, %.2f GB peak", s.Wall.Seconds(), float64(s.Peak)/(1<<30))
}

// Flags are ocamlopt's on every module: its defaults (-inline 200 doubled
// the core's compile, 22 s and 1.2 GB, and moved the heavy case by
// nothing measurable; the runtime's memory accesses are unchecked by
// their own primitives).
var Flags = []string{"-I", "+unix"}

// command is exec.Command whose process dies with ours.
func command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return cmd
}

// Build is the editor in OCaml from the C file src, in dir: dir/editor.c
// the core, dir/src/ the sources and what ocamlopt makes of them -- the
// generated editor.ml among them -- and the program at the path out,
// which it returns.
func Build(gen Gen, src, dir, out string) (string, Stats, error) {
	if _, err := Generate(gen, src, dir); err != nil {
		return "", Stats{}, err
	}
	return Compile(dir, out)
}

// Generate cuts the core from src into dir/editor.c and writes its module
// at dir/src/editor.ml (only when it differs), whose path it returns.  A
// core the backend refused any part of is refused.
func Generate(gen Gen, src, dir string) (string, error) {
	c, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	core, err := whim.Cut(c)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		return "", err
	}
	editorC := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(editorC, core, 0o644); err != nil {
		return "", err
	}
	scratch, err := os.MkdirTemp("", "mlgen.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	made := filepath.Join(scratch, "editor.ml")
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
		return "", fmt.Errorf("whiml: the OCaml backend refused part of the core:\n%s", r)
	}
	mlOut := filepath.Join(dir, "src", "editor.ml")
	if err := writeIfDiffers(mlOut, b); err != nil {
		return "", err
	}
	return mlOut, nil
}

// modules are the program's modules, in the order they are linked.
var modules = []string{"rt", "editor", "snprintf", "host", "term", "main"}

// Compile writes the embedded sources into dir/src and compiles them, with
// the generated module, into the program at out: the core's module when
// it or the runtime moved (its time and peak are the Stats), the rest, the
// stubs, and the link.
func Compile(dir, out string) (string, Stats, error) {
	var st Stats
	dir, err := filepath.Abs(filepath.Join(dir, "src"))
	if err != nil {
		return "", st, err
	}
	if err := WriteSources(dir); err != nil {
		return "", st, err
	}
	editor := filepath.Join(dir, "editor.ml")
	if _, err := os.Stat(editor); err != nil {
		return "", st, fmt.Errorf("whiml: no generated editor.ml in %s", dir)
	}
	run := func(what string, args ...string) error {
		cmd := command("ocamlopt", append(append([]string{}, Flags...), args...)...)
		cmd.Dir = dir
		o, err := cmd.CombinedOutput()
		if err != nil || len(bytes.TrimSpace(o)) > 0 {
			return fmt.Errorf("ocamlopt, %s: %v\n%s", what, err, o)
		}
		return nil
	}
	for _, m := range modules {
		src, cmx := filepath.Join(dir, m+".ml"), filepath.Join(dir, m+".cmx")
		ins := []string{src}
		if m != "rt" {
			ins = append(ins, filepath.Join(dir, "rt.cmx"))
		}
		if m != "rt" && m != "editor" {
			ins = append(ins, filepath.Join(dir, "editor.cmx"))
		}
		if !stale(cmx, ins...) {
			continue
		}
		start := time.Now()
		cmd := command("ocamlopt", append(append([]string{}, Flags...), "-c", m+".ml")...)
		cmd.Dir = dir
		o, err := cmd.CombinedOutput()
		if m == "editor" {
			st.Wall = time.Since(start)
			if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
				st.Peak = ru.Maxrss * 1024
			}
		}
		if err != nil || len(bytes.TrimSpace(o)) > 0 {
			return "", st, fmt.Errorf("ocamlopt, %s.ml: %v\n%s", m, err, o)
		}
	}
	if stale(filepath.Join(dir, "term_stubs.o"), filepath.Join(dir, "c", "term_stubs.c")) {
		if err := run("the stubs", "-ccopt", "-O2", "-c", "c/term_stubs.c"); err != nil {
			return "", st, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", st, err
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return "", st, err
	}
	link := []string{"unix.cmxa"}
	for _, m := range modules {
		link = append(link, m+".cmx")
	}
	link = append(link, "term_stubs.o", "-o", abs)
	if err := run("the link", link...); err != nil {
		return "", st, err
	}
	return out, st, nil
}

// stale says out is older than one of ins, or missing.
func stale(out string, ins ...string) bool {
	o, err := os.Stat(out)
	if err != nil {
		return true
	}
	for _, in := range ins {
		i, err := os.Stat(in)
		if err != nil || i.ModTime().After(o.ModTime()) {
			return true
		}
	}
	return false
}

// WriteSources writes the embedded hand-written sources into dir, each only
// when it differs.
func WriteSources(dir string) error {
	return fs.WalkDir(sources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := sources.ReadFile(path)
		if err != nil {
			return err
		}
		to := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		return writeIfDiffers(to, b)
	})
}

// writeIfDiffers writes b to path unless path holds it already, keeping the
// file's time for the compiles' staleness.
func writeIfDiffers(path string, b []byte) error {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}
