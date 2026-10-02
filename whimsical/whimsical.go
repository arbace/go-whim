// Package whimsical builds the editor in Scheme (doc/SCHEME.md): the core,
// the R6RS library (whimsical editor) (whimsical/editor.ss), written by
// crefactor/togo's Scheme backend from the core half of a whim-vim.c, and
// beside it the Scheme kept here by hand -- the runtime (whimsical/rt.ss),
// the host (whimsical/host.ss: the C host's functions as glue to a host
// record; whimsical/term.ss, the terminal; whimsical/printf.ss, vim's
// printf) and the launcher (main.ss) -- compiled by Chez Scheme at
// optimize-level 3 into one boot file, converted to vfasl with Chez's own
// petite.boot, and linked with Chez's kernel and a main of ours (main.c,
// boot.s) into a native program: `whimsical [args]`.
//
// The hand-written sources are embedded, as caprice's Haskell and whimsy's
// Rust are, and written into the build directory only when they differ, so
// that a library whose source has not moved is not compiled again.
package whimsical

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
	"strings"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/whim"
)

//go:embed c/main.c c/boot.s main.ss whimsical/rt.ss whimsical/host.ss whimsical/term.ss whimsical/printf.ss
var sources embed.FS

// Gen writes the library (whimsical editor) of the C core editorC to scmOut
// (with its .refused and .host beside it), as `whim skel <editorC> <dir>
// -scm <scmOut>` does.
type Gen func(editorC, dir, scmOut string) error

// ErrNoBackend is what Build says when the generator wrote nothing.
var ErrNoBackend = errors.New("whimsical: the generator wrote no editor.ss -- this toolset has no Scheme backend (`whim skel <editor.c> <dir> -scm <out.ss>`)")

// Stats is what compiling the core cost Chez: the wall time and the peak
// resident memory, as wait4 reports them; zero when the core had not moved.
type Stats struct {
	Wall time.Duration
	Peak int64 // bytes
}

func (s Stats) String() string {
	if s.Wall == 0 {
		return "the core's library current"
	}
	return fmt.Sprintf("chez %.1f s, %.2f GB peak", s.Wall.Seconds(), float64(s.Peak)/(1<<30))
}

// command is exec.Command whose process dies with ours.
func command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return cmd
}

// Build is the editor in Scheme from the C file src, in dir: dir/editor.c
// the core, dir/src/ the sources and what Chez makes of them -- the
// generated whimsical/editor.ss among them -- and the program at the path
// out, which it returns.
func Build(gen Gen, src, dir, out string) (string, Stats, error) {
	if _, err := Generate(gen, src, dir); err != nil {
		return "", Stats{}, err
	}
	return Compile(dir, out)
}

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

// The compiler's settings: optimize-level 3 (unsafe, as C is), and no
// inspector information, which halves the start-up and changes nothing
// the program does (doc/SCHEME.md, §8.2).
const settings = `(optimize-level 3) (generate-inspector-information #f) (generate-procedure-source-information #f)
(compile-imported-libraries #t) (library-directories '(("." . ".")))
`

// coreScript compiles the runtime and the core's library.
const coreScript = settings + `(compile-library "whimsical/editor.ss" "whimsical/editor.so")
`

// restScript compiles the host's libraries and the launcher, as main.ss
// imports them, makes the boot file, and converts it and Chez's petite.boot
// to vfasl.
const restScript = settings + `(compile-file "main.ss" "main.so")
(make-boot-file "whimsical.boot" '("petite")
  "whimsical/rt.so" "whimsical/editor.so" "whimsical/printf.so" "whimsical/host.so" "whimsical/term.so" "main.so")
(vfasl-convert-file "whimsical.boot" "whimsical-v.boot" '("petite"))
(vfasl-convert-file (string-append (car (command-line-arguments)) "/petite.boot") "petite-v.boot" '())
`

// Compile writes the embedded sources into dir/src and compiles them, with
// the generated library, into the program at out: the core's library when
// it or the runtime moved (its time and peak are the Stats), the rest and
// the boot file, and the link.
func Compile(dir, out string) (string, Stats, error) {
	var st Stats
	dir, err := filepath.Abs(filepath.Join(dir, "src"))
	if err != nil {
		return "", st, err
	}
	if err := WriteSources(dir); err != nil {
		return "", st, err
	}
	editor := filepath.Join(dir, "whimsical", "editor.ss")
	if _, err := os.Stat(editor); err != nil {
		return "", st, fmt.Errorf("whimsical: no generated whimsical/editor.ss in %s", dir)
	}
	kernel, err := Kernel()
	if err != nil {
		return "", st, err
	}
	if err := os.WriteFile(filepath.Join(dir, "core.ss"), []byte(coreScript), 0o644); err != nil {
		return "", st, err
	}
	if err := os.WriteFile(filepath.Join(dir, "rest.ss"), []byte(restScript), 0o644); err != nil {
		return "", st, err
	}
	if stale(filepath.Join(dir, "whimsical", "editor.so"), editor, filepath.Join(dir, "whimsical", "rt.ss")) {
		cmd := command("chez", "-q", "--script", "core.ss")
		cmd.Dir = dir
		start := time.Now()
		o, err := cmd.CombinedOutput()
		st.Wall = time.Since(start)
		if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			st.Peak = ru.Maxrss * 1024
		}
		if err := quiet("chez, the core", o, err); err != nil {
			return "", st, err
		}
	}
	cmd := command("chez", "-q", "--script", "rest.ss", kernel)
	cmd.Dir = dir
	o, err := cmd.CombinedOutput()
	if err := quiet("chez, the host and the boot file", o, err); err != nil {
		return "", st, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", st, err
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return "", st, err
	}
	cmd = command("gcc", "-O2", "-o", abs, "-I"+kernel, "c/main.c", "c/boot.s", filepath.Join(kernel, "libkernel.a"),
		lz4(), "-lz", "-lncursesw", "-lpthread", "-ldl", "-lm")
	cmd.Dir = dir
	if o, err := cmd.CombinedOutput(); err != nil || len(bytes.TrimSpace(o)) > 0 {
		return "", st, fmt.Errorf("gcc: %v\n%s", err, o)
	}
	return out, st, nil
}

// quiet is err, or what Chez said that was not a library being compiled: a
// warning is an error.
func quiet(what string, o []byte, err error) error {
	var said []string
	for _, l := range strings.Split(string(o), "\n") {
		if l != "" && !strings.HasPrefix(l, "compiling ") {
			said = append(said, l)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %v\n%s", what, err, o)
	}
	if len(said) > 0 {
		return fmt.Errorf("%s: it said something, and the libraries are to say nothing:\n%s", what, strings.Join(said, "\n"))
	}
	return nil
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

// Kernel is the directory of Chez's kernel, its boot files and scheme.h:
// /usr/lib/csvVERSION/MACHINE, as the chez on the PATH has them.
func Kernel() (string, error) {
	cmd := command("chez", "-q")
	cmd.Stdin = strings.NewReader(`(call-with-values scheme-version-number (lambda (a b c) (printf "~a.~a.~a ~a" a b c (machine-type))))`)
	o, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("whimsical: chez: %v", err)
	}
	f := strings.Fields(string(o))
	if len(f) != 2 {
		return "", fmt.Errorf("whimsical: chez says %q", o)
	}
	path, err := exec.LookPath("chez")
	if err != nil {
		return "", err
	}
	for _, d := range []string{filepath.Join(filepath.Dir(path), "..", "lib", "csv"+f[0], f[1]), filepath.Join("/usr/lib", "csv"+f[0], f[1])} {
		if _, err := os.Stat(filepath.Join(d, "libkernel.a")); err == nil {
			return filepath.Clean(d), nil
		}
	}
	return "", fmt.Errorf("whimsical: no libkernel.a for Chez %s %s", f[0], f[1])
}

// lz4 is the lz4 library the kernel needs: the shared one, as the package
// has no static one.
func lz4() string {
	for _, p := range []string{"/usr/lib/liblz4.so", "/usr/lib/liblz4.so.1"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "-llz4"
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
