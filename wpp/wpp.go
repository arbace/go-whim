// Package wpp builds whim++, the editor in C++23 (doc/CPP.md): the core cut
// from a whim-vim.c and written as C++ by crefactor/togo's C++ backend --
// the header editor.hpp (the types, the constants, class Editor) and
// editor.cpp (its member functions) -- compiled by g++ with the
// hand-written runtime (rt.hpp, rt.cpp), host (host.hpp, host.cpp), printf
// (printf.cpp), terminal host (term.hpp, term.cpp) and launcher
// (main.cpp) into one program.
//
// The C++ is in src/ (Go takes a .cpp beside a .go for cgo's).  The
// hand-written sources are embedded, as whiml's and whimsy's are, so
// that a build in any directory -- the suite's, a test's -- compiles the
// same files; editor.hpp and editor.cpp, tracked here too, are what `whim
// gen` writes and `make whim-editor-check` holds.
package wpp

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
	"sync"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/procattr"
	"github.com/arbace/go-whim/internal/whim"
)

//go:embed src/rt.hpp src/rt.cpp src/host.hpp src/host.cpp src/printf.cpp src/term.hpp src/term.cpp src/main.cpp
var sources embed.FS

// Gen writes the C++ of a core, editorC, as cppOut (editor.cpp) and the
// header beside it, using dir as its scratch: `whim skel <editorC> <dir>
// -cpp <cppOut>`, which cmd/whim hands in (this module cannot import the
// toolset).
type Gen func(editorC, dir, cppOut string) error

// ErrNoBackend is Generate's answer when the generator wrote nothing.
var ErrNoBackend = errors.New("wpp: the generator wrote no editor.cpp -- this toolset has no C++ backend (`whim skel <editor.c> <dir> -cpp <out.cpp>`)")

// Stats are g++'s time and peak memory on the core, when it compiled it.
type Stats struct {
	Wall time.Duration
	Peak int64 // bytes
}

func (s Stats) String() string {
	if s.Wall == 0 {
		return "the core's object current"
	}
	return fmt.Sprintf("g++ %.1f s, %.2f GB peak on the core", s.Wall.Seconds(), float64(s.Peak)/(1<<30))
}

// Flags are g++'s for every file: C++23, optimized, every warning of -Wall
// and -Wextra -- which the generated core is compiled with too, a warning
// a failed build.
var Flags = []string{"-std=c++23", "-O2", "-Wall", "-Wextra"}

// Program is the launcher's name: bin/whim++.
const Program = "whim++"

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.SysProcAttr = procattr.Child()
	return cmd
}

// Build generates the core of src into dir and compiles the program out.
func Build(gen Gen, src, dir, out string) (string, Stats, error) {
	if _, err := Generate(gen, src, dir); err != nil {
		return "", Stats{}, err
	}
	return Compile(dir, out)
}

// Generate cuts the core of the whim-vim.c src and writes it as
// dir/src/editor.hpp and editor.cpp -- each only when it differs, so that
// an object whose source has not moved is not compiled again -- and
// returns editor.cpp's path.
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
	scratch, err := os.MkdirTemp("", "cppgen.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	made := filepath.Join(scratch, "editor.cpp")
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
		return "", fmt.Errorf("wpp: the C++ backend refused part of the core:\n%s", r)
	}
	h, err := os.ReadFile(filepath.Join(scratch, "editor.hpp"))
	if err != nil {
		return "", err
	}
	if err := writeIfDiffers(filepath.Join(dir, "src", "editor.hpp"), h); err != nil {
		return "", err
	}
	out := filepath.Join(dir, "src", "editor.cpp")
	if err := writeIfDiffers(out, b); err != nil {
		return "", err
	}
	return out, nil
}

// units are the translation units, the generated core's first: it takes
// the longest, and starts first.
var units = []string{"editor", "rt", "host", "printf", "term", "main"}

// headers each unit includes, for its staleness.
var headers = map[string][]string{
	"editor": {"editor.hpp", "rt.hpp"},
	"rt":     {"rt.hpp"},
	"host":   {"editor.hpp", "host.hpp"},
	"printf": {"editor.hpp"},
	"term":   {"term.hpp", "host.hpp"},
	"main":   {"term.hpp", "host.hpp"},
}

// Compile compiles dir/src -- the generated core and the hand-written
// sources, written beside it -- into the program out, each unit side by
// side, and only the units whose sources moved.
func Compile(dir, out string) (string, Stats, error) {
	var st Stats
	dir, err := filepath.Abs(filepath.Join(dir, "src"))
	if err != nil {
		return "", st, err
	}
	if err := WriteSources(dir); err != nil {
		return "", st, err
	}
	if _, err := os.Stat(filepath.Join(dir, "editor.cpp")); err != nil {
		return "", st, fmt.Errorf("wpp: no generated editor.cpp in %s", dir)
	}
	var wg sync.WaitGroup
	errs := make([]error, len(units))
	for i, u := range units {
		obj := filepath.Join(dir, u+".o")
		ins := []string{filepath.Join(dir, u+".cpp")}
		for _, h := range headers[u] {
			ins = append(ins, filepath.Join(dir, h))
		}
		if !stale(obj, ins...) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			cmd := command("g++", append(append([]string{}, Flags...), "-c", u+".cpp", "-o", u+".o")...)
			cmd.Dir = dir
			o, err := cmd.CombinedOutput()
			if u == "editor" {
				st.Wall = time.Since(start)
				if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
					st.Peak = ru.Maxrss * 1024
				}
			}
			if err != nil || len(bytes.TrimSpace(o)) > 0 {
				os.Remove(obj)
				errs[i] = fmt.Errorf("g++, %s.cpp: %v\n%s", u, err, o)
			}
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return "", st, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", st, err
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return "", st, err
	}
	link := []string{"-o", abs}
	for _, u := range units {
		link = append(link, u+".o")
	}
	cmd := command("g++", link...)
	cmd.Dir = dir
	if o, err := cmd.CombinedOutput(); err != nil || len(bytes.TrimSpace(o)) > 0 {
		return "", st, fmt.Errorf("g++, the link: %v\n%s", err, o)
	}
	return out, st, nil
}

// stale reports whether out is missing or older than any of ins.
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

// WriteSources writes the hand-written sources into dir.
func WriteSources(dir string) error {
	return fs.WalkDir(sources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := sources.ReadFile(path)
		if err != nil {
			return err
		}
		return writeIfDiffers(filepath.Join(dir, filepath.Base(path)), b)
	})
}

func writeIfDiffers(path string, b []byte) error {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}
