package treepilot

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/sweep"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/whim"
)

// A Cost is one segment's wall and CPU time (user and system, every thread).
type Cost struct{ Wall, CPU time.Duration }

func (c *Cost) add(o Cost) { c.Wall += o.Wall; c.CPU += o.CPU }

func cpuNow() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

// measure runs f and says what it cost.
func measure(f func()) Cost {
	w, c := time.Now(), cpuNow()
	f()
	return Cost{time.Since(w), cpuNow() - c}
}

// Times is one phase's run, by segment: the text steps (the edits as the
// plan runs them, and a sweep a phase's steps hold), the tree's three
// (ToLisp, the rewrite, Print), and what follows every phase (the sweep and
// the canonical print).
type Times struct {
	Steps, ToLisp, Rewrite, Print, Sweep, Canon Cost
}

// Total is the phase, every segment.
func (t Times) Total() Cost {
	var c Cost
	for _, x := range []Cost{t.Steps, t.ToLisp, t.Rewrite, t.Print, t.Sweep, t.Canon} {
		c.add(x)
	}
	return c
}

// PhaseOf is the plan's phase n.
func PhaseOf(n int) (build.Phase, bool) {
	for _, p := range build.Plan {
		if p.N == n {
			return p, true
		}
	}
	return build.Phase{}, false
}

// finish is what follows every phase, as crefactor/pipeline's finish runs
// it: the sweep, in memory, and the canonical print of its last parse.
func finish(text []byte, scratch string, t *Times) ([]byte, error) {
	path := filepath.Join(scratch, "whim-vim.c")
	var swept []byte
	var parsed *cc.AST
	var err error
	t.Sweep = measure(func() { swept, _, parsed, err = sweep.PruneParsed(text, path, whim.Profile.Sweep) })
	if err != nil {
		return nil, fmt.Errorf("sweep: %w", err)
	}
	var out []byte
	t.Canon = measure(func() { out, err = cemit.CanonicalParsed(path, swept, parsed) })
	return out, err
}

// RunText is phase p as the plan runs it (build.Advance), timed by segment.
func RunText(p build.Phase, in []byte) ([]byte, Times, error) {
	var t Times
	scratch, err := os.MkdirTemp("", fmt.Sprintf("treepilot%03d.", p.N))
	if err != nil {
		return nil, t, err
	}
	defer os.RemoveAll(scratch)
	var out []byte
	t.Steps = measure(func() { out, err = build.RunPhase(p, in, scratch, io.Discard) })
	if err != nil {
		return nil, t, err
	}
	out, err = finish(out, scratch, &t)
	return out, t, err
}

// TreeSteps are the pilot's: `droplocal` (DropLocal, a field at a time) and
// `edit whim24` (P24).
func TreeSteps(s build.Step) (func(*Tree, io.Writer) error, bool) {
	switch {
	case s.Op == "droplocal":
		return func(t *Tree, w io.Writer) error {
			for _, f := range s.Args {
				n, err := DropLocal(t, f)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "  droplocal    %-10s %d plumbing sites\n", f, n)
			}
			return nil
		}, true
	case s.Op == "edit" && len(s.Args) == 1 && s.Args[0] == "whim24":
		return P24, true
	}
	return nil, false
}

// RunTree is phase p with the steps the pilot has on the tree run there:
// the text converted once before a run of them (ToLisp), rewritten, and
// printed once after (Print); the other steps as the plan runs them; then
// the same sweep and canonical print.
func RunTree(p build.Phase, in []byte, w io.Writer) ([]byte, Times, error) {
	var t Times
	scratch, err := os.MkdirTemp("", fmt.Sprintf("treepilot%03d.", p.N))
	if err != nil {
		return nil, t, err
	}
	defer os.RemoveAll(scratch)
	text := in
	var tree *Tree
	flush := func() error {
		if tree == nil {
			return nil
		}
		var err error
		t.Print.add(measure(func() { text, err = clisp.Print(tree.Root.List) }))
		tree = nil
		return err
	}
	for _, s := range p.Steps {
		f, ok := TreeSteps(s)
		if !ok {
			if err := flush(); err != nil {
				return nil, t, err
			}
			one := p
			one.Steps = []build.Step{s}
			var err error
			t.Steps.add(measure(func() { text, err = build.RunPhase(one, text, scratch, io.Discard) }))
			if err != nil {
				return nil, t, err
			}
			continue
		}
		if tree == nil {
			var fs []*clisp.Node
			var err error
			t.ToLisp.add(measure(func() { fs, err = clisp.Forms(filepath.Join(scratch, "whim-vim.c"), text) }))
			if err != nil {
				return nil, t, err
			}
			tree = &Tree{Root: clisp.Root(fs)}
		}
		var err error
		t.Rewrite.add(measure(func() { err = f(tree, w) }))
		if err != nil {
			return nil, t, err
		}
	}
	if err := flush(); err != nil {
		return nil, t, err
	}
	out, err := finish(text, scratch, &t)
	return out, t, err
}
