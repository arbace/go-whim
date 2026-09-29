// Package caprice builds the editor in Haskell (doc/HASKELL.md): the core,
// the module Caprice.Editor, written by crefactor/togo's Haskell backend
// from the core half of a whim-vim.c, and beside it the Haskell kept here by
// hand -- the runtime (rt/Caprice/Rt.hs: C's memory, raw), the host
// (host/Caprice/Host.hs and Printf.hs: the C host's functions) and the
// launcher (Main.hs) -- compiled by GHC into a native program: `caprice
// [args]`.
//
// The Haskell sources are embedded, as vijure's Clojure are, and written
// into the build directory only when they differ, so that GHC's
// recompilation check skips what has not moved: the core is one module of
// some 120,000 lines, three minutes of GHC's time at -O1.
package caprice

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

	"github.com/arbace/go-whim/internal/whim"
)

//go:embed rt/Caprice/Rt.hs host/Caprice/Host.hs host/Caprice/Term.hs host/Caprice/Run.hs host/Caprice/Printf.hs Main.hs
var sources embed.FS

// Gen writes the module Caprice.Editor of the C core editorC to hsOut (and
// its hs-boot beside it), as `whim skel <editorC> <dir> -hs <hsOut>` does.
type Gen func(editorC, dir, hsOut string) error

// ErrNoBackend is what Build says when the generator wrote nothing.
var ErrNoBackend = errors.New("caprice: the generator wrote no Caprice/Editor.hs -- this toolset has no Haskell backend (`whim skel <editor.c> <dir> -hs <out.hs>`)")

// GHCFlags are GHC's options for the editor, each for a reason:
//
//   - -O1: the core is some 120,000 lines of IO code on raw memory; -O1
//     compiles it in about the time -O0 does (183 s against 198, measured)
//     and makes the join points jumps;
//   - -threaded: the host waits for keys, a signal and a timeout at once in
//     STM (registerDelay, threadWaitReadSTM), which the threaded RTS has;
//   - -rtsopts: the RTS's options can be given at a run (+RTS ... -RTS);
//   - -with-rtsopts=-N -qg: a capability a core, for match_lines's chunks,
//     and one thread collecting, as without them: the parallel collector made
//     the sequential work slower (a 100,000-line :%s/the/THE/g 0.46 s ->
//     0.81), and with -qg it is as before, and \v(a|b)+c 3.6 s -> 0.36.
var GHCFlags = []string{"-O1", "-threaded", "-rtsopts", "-with-rtsopts=-N -qg"}

// command is exec.Command whose process dies with ours.
func command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return cmd
}

// Build is the editor in Haskell from the C file src, in dir: dir/editor.c
// the core, dir/src/ the sources -- the generated Caprice/Editor.hs among
// them -- dir/o/ GHC's objects, and the program at the path out, which it
// returns.
func Build(gen Gen, src, dir, out string) (string, error) {
	if _, err := Generate(gen, src, dir); err != nil {
		return "", err
	}
	return Compile(dir, out)
}

// Generate cuts the core from src into dir/editor.c and writes the module
// Caprice.Editor of it at dir/src/Caprice/Editor.hs (only when it differs),
// whose path it returns.
func Generate(gen Gen, src, dir string) (string, error) {
	c, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	core, err := whim.Cut(c)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "Caprice"), 0o755); err != nil {
		return "", err
	}
	editorC := filepath.Join(dir, "editor.c")
	if err := os.WriteFile(editorC, core, 0o644); err != nil {
		return "", err
	}
	scratch, err := os.MkdirTemp("", "hsgen.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	made := filepath.Join(scratch, "Editor.hs")
	if err := gen(editorC, scratch, made); err != nil {
		return "", err
	}
	if _, err := os.Stat(made); errors.Is(err, fs.ErrNotExist) {
		return "", ErrNoBackend
	} else if err != nil {
		return "", err
	}
	if r, err := os.ReadFile(made + ".refused"); err == nil && len(bytes.TrimSpace(r)) > 0 {
		return "", fmt.Errorf("caprice: the Haskell backend refused part of the core:\n%s", r)
	}
	hsOut := filepath.Join(dir, "src", "Caprice", "Editor.hs")
	for _, f := range []struct{ from, to string }{{made, hsOut}, {filepath.Join(scratch, "Editor.hs-boot"), filepath.Join(dir, "src", "Caprice", "Editor.hs-boot")}} {
		b, err := os.ReadFile(f.from)
		if err != nil {
			return "", err
		}
		if err := writeIfDiffers(f.to, b); err != nil {
			return "", err
		}
	}
	return hsOut, nil
}

// Compile writes the embedded sources into dir/src and compiles them with
// the generated module into the program at out.
func Compile(dir, out string) (string, error) {
	return CompileMain(dir, out, "")
}

// CompileMain is Compile with the program's module Main the file main
// instead of Main.hs, when main is not empty: a Main that imports only the
// editor's modules. It shares dir/o with the editor's build and GHC's flags
// -- the search path among them, which is why main's directory is not added
// to it -- so only Main is compiled.
func CompileMain(dir, out, main string) (string, error) {
	// GHC's recompilation check fingerprints the search path and the output
	// directory as written: one build's relative path and another's absolute
	// one would each recompile the core for the other.
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	srcDir := filepath.Join(dir, "src")
	err = fs.WalkDir(sources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := sources.ReadFile(path)
		if err != nil {
			return err
		}
		// rt/Caprice/Rt.hs and host/Caprice/Host.hs go under src/Caprice
		rel := path
		for _, top := range []string{"rt/", "host/"} {
			if len(rel) > len(top) && rel[:len(top)] == top {
				rel = rel[len(top):]
			}
		}
		to := filepath.Join(srcDir, rel)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		return writeIfDiffers(to, b)
	})
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	args := append(append([]string{}, GHCFlags...), "-v0", "-i"+srcDir, "-outputdir", filepath.Join(dir, "o"), "-o", out)
	if main == "" {
		args = append(args, filepath.Join(srcDir, "Main.hs"))
	} else {
		args = append(args, main)
	}
	if o, err := command("ghc", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("ghc: %v\n%s", err, o)
	}
	return out, nil
}

// writeIfDiffers writes b to path unless path holds it already, keeping the
// file's time for GHC's recompilation check.
func writeIfDiffers(path string, b []byte) error {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}
