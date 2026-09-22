package verify

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/harness"
)

// THE BASELINES ARE THE ONE PRODUCED THING WORTH KEEPING, and this is what
// records them -- phase 0's program and phase 83's, which recorded them when a
// pass still ran shell.
//
// Two sets, because an editor with no file to write cannot be measured by the
// instrument that measures one:
//
//	.reference/baselines        from slim-vim.c, the pipeline's immutable
//	                            input: behaviour cases, terminals, Ex commands
//	.reference/core-baselines   from q82, the tree phase 83 is handed: screens,
//	                            memline, Ex commands, argv, pty, terminals
//
// EACH IS RECORDED THREE TIMES AND REQUIRED IDENTICAL.  A recording that is not
// deterministic is not a baseline, and a delta measured against it would be
// noise. An existing set is COMPARED, never overwritten: the same input
// recorded differently means a harness changed or the input did, and which one
// must be named before the recording is thrown away.

// Record writes both baseline sets, or compares them with what is there.
func Record(w io.Writer) error {
	if err := recordSlim(w); err != nil {
		return err
	}
	return recordCore(w)
}

// recordSlim is phase 0's half: the baselines of slim-vim.c's own binary.
func recordSlim(w io.Writer) error {
	tmp, err := os.MkdirTemp("", "record-slim")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	mk := filepath.Join(tmp, "Makefile")
	if err := build.ApplyMakefile("whim", mk); err != nil {
		return err
	}
	bin := filepath.Join(tmp, "whim-vim")
	if err := buildWith(mk, "slim-vim.c", bin); err != nil {
		return fmt.Errorf("the input did not build: %w", err)
	}
	fmt.Fprintf(w, "  seed         slim-vim.c, %d lines, built to %d bytes\n",
		countLines(mustRead("slim-vim.c")), sizeOf(bin))

	one := func(dir string) error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		errs := make(chan error, 3)
		go func() { errs <- harness.Behaviour(bin, filepath.Join(dir, "behaviour"), io.Discard) }()
		go func() { errs <- harness.TermCheck(bin, filepath.Join(dir, "ref-term.txt"), io.Discard) }()
		go func() {
			errs <- harness.ExSweep(bin, "slim-vim.c", filepath.Join(dir, "ref-exsweep.txt"), io.Discard)
		}()
		for i := 0; i < 3; i++ {
			if err := <-errs; err != nil {
				return err
			}
		}
		return nil
	}
	return recordThrice(w, tmp, ".reference/baselines", "slim-vim.c", one)
}

// recordCore is phase 83's half: the baselines of q82's binary, built with the
// makefile q82 carries.
func recordCore(w io.Writer) error {
	tmp, err := os.MkdirTemp("", "record-core")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	work := filepath.Join(tmp, "q82")
	fmt.Fprintf(w, "  q82          building the tree phase 83 is handed\n")
	if _, err := build.Run(build.Options{Src: "slim-vim.c", To: 82, Work: work, W: io.Discard}); err != nil {
		return err
	}
	src := filepath.Join(work, "whim-vim.c")
	mk := filepath.Join(work, "Makefile")
	bin := filepath.Join(work, "whim-vim")
	if err := buildWith(mk, src, bin); err != nil {
		return fmt.Errorf("q82 did not build with the makefile it carries: %w", err)
	}
	fmt.Fprintf(w, "  q82          %d lines, %d bytes\n", countLines(mustRead(src)), sizeOf(bin))

	one := func(dir string) error {
		return harness.ZRecord(bin, src, dir, io.Discard)
	}
	return recordThrice(w, tmp, ".reference/core-baselines", "q82", one)
}

// recordThrice records into three directories, requires them identical, and
// then compares with the recorded set or writes it.
func recordThrice(w io.Writer, tmp, base, from string, one func(dir string) error) error {
	runs := make([]string, 3)
	for i := range runs {
		runs[i] = filepath.Join(tmp, fmt.Sprintf("run%d", i+1))
		if err := one(runs[i]); err != nil {
			return fmt.Errorf("a harness failed on run %d: %w", i+1, err)
		}
		if i > 0 {
			if d, err := treeDiff(runs[0], runs[i]); err != nil {
				return err
			} else if d != "" {
				return fmt.Errorf("run %d differs from run 1 -- not deterministic, not a baseline:\n%s", i+1, d)
			}
		}
	}
	if _, err := os.Stat(base); err == nil {
		d, err := treeDiff(base, runs[0])
		if err != nil {
			return err
		}
		if d != "" {
			return fmt.Errorf("the recording DIFFERS from %s:\n%s\n"+
				"               %s is this pipeline's input, so the same input recorded\n"+
				"               differently: a harness changed, or the input did.  Name\n"+
				"               which before removing %s.", base, d, from, base)
		}
		fmt.Fprintf(w, "  baselines    match %s, 3 identical runs\n", base)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return err
	}
	if err := copyTree(runs[0], base); err != nil {
		return err
	}
	fmt.Fprintf(w, "  baselines    recorded %s from %s, 3 identical runs\n", base, from)
	return nil
}

// treeDiff names every file two directories disagree on, or that only one has.
func treeDiff(a, b string) (string, error) {
	fa, err := filesUnder(a)
	if err != nil {
		return "", err
	}
	fb, err := filesUnder(b)
	if err != nil {
		return "", err
	}
	var out []string
	for name, ba := range fa {
		bb, ok := fb[name]
		switch {
		case !ok:
			out = append(out, "only in "+a+": "+name)
		case !bytes.Equal(ba, bb):
			out = append(out, "differ: "+name)
		}
	}
	for name := range fb {
		if _, ok := fa[name]; !ok {
			out = append(out, "only in "+b+": "+name)
		}
	}
	sort.Strings(out)
	if len(out) > 10 {
		out = append(out[:10], fmt.Sprintf("... and %d more", len(out)-10))
	}
	return strings.Join(out, "\n                 "), nil
}

func filesUnder(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		r, _ := filepath.Rel(dir, p)
		out[r] = b
		return nil
	})
	return out, err
}

func copyTree(from, to string) error {
	fs, err := filesUnder(from)
	if err != nil {
		return err
	}
	part := to + ".part"
	os.RemoveAll(part)
	for name, b := range fs {
		p := filepath.Join(part, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return err
		}
	}
	return os.Rename(part, to)
}

// buildWith compiles a source with the flags a makefile states.
func buildWith(mk, src, out string) error {
	c, l, err := build.Flags(mk)
	if err != nil {
		return err
	}
	cmd := exec.Command("gcc", append(append(append([]string{}, c...), l...), "-o", out, src)...)
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, o)
	}
	return nil
}

func sizeOf(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return fi.Size()
}
