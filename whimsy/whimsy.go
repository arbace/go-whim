// Package whimsy builds the editor in Rust (doc/RUST.md): the core, the
// module `editor` (src/editor.rs), written by crefactor/togo's Rust backend
// from the core half of a whim-vim.c, and beside it the Rust kept here by
// hand -- the runtime (src/rt.rs), the host (src/host.rs: the C host's
// functions as glue to a Host; src/term.rs, the terminal; src/printf.rs,
// vim's printf) and the launcher (src/main.rs) -- compiled by cargo, offline
// and with std alone, into a native program: `whimsy [args]`.
//
// The hand-written sources are embedded, as caprice's Haskell are, and
// written into the build directory only when they differ, so that cargo's
// fingerprints skip what has not moved.
package whimsy

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

//go:embed Cargo.toml src/lib.rs src/rt.rs src/host.rs src/printf.rs src/term.rs src/main.rs
var sources embed.FS

// Gen writes the module `editor` of the C core editorC to rsOut (with its
// .refused, .host and .layout beside it), as `whim skel <editorC> <dir> -rs
// <rsOut>` does.
type Gen func(editorC, dir, rsOut string) error

// ErrNoBackend is what Build says when the generator wrote nothing.
var ErrNoBackend = errors.New("whimsy: the generator wrote no editor.rs -- this toolset has no Rust backend (`whim skel <editor.c> <dir> -rs <out.rs>`)")

// Stats is what a compile cost cargo and rustc: the wall time and the peak
// resident memory (cargo's and its children's, as wait4 reports them).
type Stats struct {
	Wall time.Duration
	Peak int64 // bytes
}

func (s Stats) String() string {
	return fmt.Sprintf("rustc %.1f s, %.2f GB peak", s.Wall.Seconds(), float64(s.Peak)/(1<<30))
}

// command is exec.Command whose process dies with ours.
func command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return cmd
}

// Build is the editor in Rust from the C file src, in dir: dir/editor.c the
// core, dir/ the crate -- the generated src/editor.rs among its sources --
// dir/target cargo's, and the program at the path out, which it returns.
func Build(gen Gen, src, dir, out string) (string, Stats, error) {
	if _, err := Generate(gen, src, dir); err != nil {
		return "", Stats{}, err
	}
	return Compile(dir, out)
}

// Generate cuts the core from src into dir/editor.c and writes its module
// `editor` at dir/src/editor.rs (only when it differs), whose path it
// returns.  A core the backend refused any part of is refused.
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
	scratch, err := os.MkdirTemp("", "rsgen.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	made := filepath.Join(scratch, "editor.rs")
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
		return "", fmt.Errorf("whimsy: the Rust backend refused part of the core:\n%s", r)
	}
	rsOut := filepath.Join(dir, "src", "editor.rs")
	if err := writeIfDiffers(rsOut, b); err != nil {
		return "", err
	}
	return rsOut, nil
}

// Compile writes the embedded sources into dir and compiles them, with the
// generated module, into the program at out: cargo's release profile,
// offline, its target dir/target.
func Compile(dir, out string) (string, Stats, error) {
	var st Stats
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", st, err
	}
	if err := WriteSources(dir); err != nil {
		return "", st, err
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "editor.rs")); err != nil {
		return "", st, fmt.Errorf("whimsy: no generated src/editor.rs in %s", dir)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", st, err
	}
	cmd := command("cargo", "build", "--release", "--offline", "--quiet",
		"--manifest-path", filepath.Join(dir, "Cargo.toml"), "--target-dir", filepath.Join(dir, "target"))
	start := time.Now()
	o, err := cmd.CombinedOutput()
	st.Wall = time.Since(start)
	if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		st.Peak = ru.Maxrss * 1024
	}
	if err != nil {
		return "", st, fmt.Errorf("cargo build: %v\n%s", err, o)
	}
	if len(bytes.TrimSpace(o)) > 0 {
		return "", st, fmt.Errorf("cargo build: it said something, and the crate is to say nothing:\n%s", o)
	}
	b, err := os.ReadFile(filepath.Join(dir, "target", "release", "whimsy"))
	if err != nil {
		return "", st, err
	}
	if err := writeIfDiffers(out, b); err != nil {
		return "", st, err
	}
	return out, st, os.Chmod(out, 0o755)
}

// WriteSources writes the embedded hand-written sources into the crate at
// dir, each only when it differs.
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
// file's time for cargo's fingerprints.
func writeIfDiffers(path string, b []byte) error {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}
