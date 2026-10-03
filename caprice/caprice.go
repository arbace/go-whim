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
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

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
//     0.81), and with -qg it is as before, and \v(a|b)+c 3.6 s -> 0.36;
//   - -j4: the core's parts (Profile.HsParts) compiled four at a time
//     where the call graph lets them be.
var GHCFlags = []string{"-O1", "-threaded", "-rtsopts", "-with-rtsopts=-N -qg", "-j4"}

// GHC is the compiler CompileStats runs: ghc from PATH, or another (ghc-lisp's,
// for `whim hscat --ghc`).
var GHC = "ghc"

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
func Build(gen Gen, src, dir, out string) (string, GHCStats, error) {
	if _, err := Generate(gen, src, dir); err != nil {
		return "", GHCStats{}, err
	}
	return CompileStats(dir, out, "")
}

// GHCStats is what a compile cost GHC: its wall time and its peak resident
// memory (its own and its children's, as wait4 reports them). The core is
// one module of some 120,000 lines, so a change to how it is printed moves
// these before it moves the editor: they are printed beside every build.
type GHCStats struct {
	Wall time.Duration
	Peak int64 // bytes
}

func (s GHCStats) String() string {
	return fmt.Sprintf("ghc %.1f s, %.2f GB peak", s.Wall.Seconds(), float64(s.Peak)/(1<<30))
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
	if err := CopyGenerated(scratch, filepath.Dir(hsOut)); err != nil {
		return "", err
	}
	return hsOut, nil
}

// Compile writes the embedded sources into dir/src and compiles them with
// the generated module into the program at out.
func Compile(dir, out string) (string, error) {
	p, _, err := CompileStats(dir, out, "")
	return p, err
}

// CompileMain is Compile with the program's module Main the file main
// instead of Main.hs, when main is not empty: a Main that imports only the
// editor's modules. It shares dir/o with the editor's build and GHC's flags
// -- the search path among them, which is why main's directory is not added
// to it -- so only Main is compiled.
func CompileMain(dir, out, main string) (string, error) {
	p, _, err := CompileStats(dir, out, main)
	return p, err
}

// CompileStats is CompileMain, and what GHC cost.
func CompileStats(dir, out, main string) (string, GHCStats, error) {
	var st GHCStats
	// GHC's recompilation check fingerprints the search path and the output
	// directory as written: one build's relative path and another's absolute
	// one would each recompile the core for the other.
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", st, err
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
		return "", st, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", st, err
	}
	args := append(append([]string{}, GHCFlags...), "-v0", "-i"+srcDir, "-outputdir", filepath.Join(dir, "o"), "-o", out)
	if main == "" {
		args = append(args, filepath.Join(srcDir, "Main.hs"))
	} else {
		args = append(args, main)
	}
	cmd := command(GHC, args...)
	start := time.Now()
	o, err := cmd.CombinedOutput()
	st.Wall = time.Since(start)
	if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		st.Peak = ru.Maxrss * 1024
	}
	if err != nil {
		return "", st, fmt.Errorf("ghc: %v\n%s", err, o)
	}
	return out, st, nil
}

// Generated are the files the Haskell backend writes for the module
// Caprice.Editor in dir, relative to it: Editor.hs, its hs-boot, and, split
// (Profile.HsParts), Editor/Defs.hs and Editor/PartN.hs.
func Generated(dir string) ([]string, error) {
	out := []string{"Editor.hs", "Editor.hs-boot"}
	for _, f := range out {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return nil, err
		}
	}
	parts, err := filepath.Glob(filepath.Join(dir, "Editor", "*.hs"))
	if err != nil {
		return nil, err
	}
	sort.Strings(parts)
	for _, p := range parts {
		out = append(out, filepath.Join("Editor", filepath.Base(p)))
	}
	return out, nil
}

// CopyGenerated copies the generated files from dir `from` to dir `to`,
// each only when it differs, and removes a part in `to` the backend no
// longer writes.
func CopyGenerated(from, to string) error {
	names, err := Generated(from)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, f := range names {
		keep[f] = true
		b, err := os.ReadFile(filepath.Join(from, f))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(to, f)), 0o755); err != nil {
			return err
		}
		if err := writeIfDiffers(filepath.Join(to, f), b); err != nil {
			return err
		}
	}
	old, _ := filepath.Glob(filepath.Join(to, "Editor", "*.hs"))
	for _, p := range old {
		if !keep[filepath.Join("Editor", filepath.Base(p))] {
			if err := os.Remove(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeIfDiffers writes b to path unless path holds it already, keeping the
// file's time for GHC's recompilation check.
func writeIfDiffers(path string, b []byte) error {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}

// Lint is `ghc -fno-code -Wall` on the generated module Caprice.Editor of the
// build in dir -- a copy of its sources, the module's -w taken out -- and the
// count of its warnings by flag: the printer's lint measure (clj-kondo's for
// the Clojure). It is an instrument, not a step of the build: -Wall's own
// analyses make it some minutes on the core.
func Lint(dir string) (map[string]int, error) {
	scratch, err := os.MkdirTemp("", "hslint.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	src := filepath.Join(dir, "src")
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if rel == filepath.Join("Caprice", "Editor.hs") || strings.HasPrefix(rel, filepath.Join("Caprice", "Editor")+string(filepath.Separator)) {
			b = bytes.Replace(b, []byte("{-# OPTIONS_GHC -w #-}\n"), nil, 1)
		}
		to := filepath.Join(scratch, rel)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	})
	if err != nil {
		return nil, err
	}
	editor := filepath.Join(scratch, "Caprice", "Editor.hs")
	o, err := command("ghc", "-fno-code", "-Wall", "-i"+scratch, editor).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ghc -fno-code -Wall: %v\n%s", err, o)
	}
	counts := map[string]int{}
	for _, m := range lintRe.FindAllSubmatch(o, -1) {
		if f := string(m[1]); f == editor || strings.HasPrefix(f, strings.TrimSuffix(editor, ".hs")+string(filepath.Separator)) {
			counts[string(m[2])]++
		}
	}
	return counts, nil
}

// lintRe is a warning's file and flag in GHC's output.
var lintRe = regexp.MustCompile(`(?m)^(\S+?):\d+:\d+: warning:[^\n]*?\[(-W[a-z-]+)`)

// LintReport is Lint's counts, most first, and their total.
func LintReport(counts map[string]int) string {
	var flags []string
	total := 0
	for f, n := range counts {
		flags = append(flags, f)
		total += n
	}
	sort.Slice(flags, func(i, j int) bool {
		if counts[flags[i]] != counts[flags[j]] {
			return counts[flags[i]] > counts[flags[j]]
		}
		return flags[i] < flags[j]
	})
	var b strings.Builder
	fmt.Fprintf(&b, "  -Wall        %d warnings in Caprice.Editor\n", total)
	for _, f := range flags {
		fmt.Fprintf(&b, "  %7d %s\n", counts[f], f)
	}
	return b.String()
}
