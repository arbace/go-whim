package graphcut

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/graph"
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

func measure(f func()) Cost {
	w, c := time.Now(), cpuNow()
	f()
	return Cost{time.Since(w), cpuNow() - c}
}

// Times is one phase's run, by segment: the text steps, the graph's (the
// import, or nothing when the graph is handed in; the editor's index; the
// cut with its closure; the collection; the C view), and
// the text's finish (the sweep and the canonical print) when the phase
// ends on a text step.
type Times struct {
	Steps, Import, Index, Edit, Collect, Print, Sweep, Canon Cost
}

// Total is the phase, every segment.
func (t Times) Total() Cost {
	var c Cost
	for _, x := range []Cost{t.Steps, t.Import, t.Index, t.Edit, t.Collect, t.Print, t.Sweep, t.Canon} {
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

// A GraphStep is a plan step done on the graph.
type GraphStep func(e *graph.Editor, w io.Writer) error

// GraphSteps are the ones this package has: `droplocal` (DropLocal, a
// field at a time) and `edit whim24` (P24).
func GraphSteps(s build.Step) (GraphStep, bool) {
	switch {
	case s.Op == "droplocal":
		return func(e *graph.Editor, w io.Writer) error {
			for _, f := range s.Args {
				n, err := DropLocal(e, f, fallOut())
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "  droplocal    %-10s %d plumbing sites\n", f, n)
			}
			return nil
		}, true
	case s.Op == "edit" && len(s.Args) == 1 && s.Args[0] == "whim24":
		return func(e *graph.Editor, w io.Writer) error { return P24(e, fallOut(), w) }, true
	}
	return nil, false
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
func RunText(p build.Phase, in []byte, w io.Writer) ([]byte, Times, error) {
	var t Times
	scratch, err := os.MkdirTemp("", fmt.Sprintf("graphcut%03d.", p.N))
	if err != nil {
		return nil, t, err
	}
	defer os.RemoveAll(scratch)
	var out []byte
	t.Steps = measure(func() { out, err = build.RunPhase(p, in, scratch, w) })
	if err != nil {
		return nil, t, err
	}
	out, err = finish(out, scratch, &t)
	return out, t, err
}

// RunGraph is phase p with its graph steps on the graph.  The composition:
// the text steps run as the plan runs them; before the first of a run of
// graph steps the text is imported (or, when it is the phase's first step
// and from is given, from is the graph -- q(N-1)'s, read from its Lisp);
// the run edits that one graph; a text step after it gets the graph's C
// view (canonical, as every text a phase hands on is once printed).  A
// phase that ends on the graph is collected and printed: no sweep, no
// canonical print.  One that ends on text is finished as the plan finishes
// it.
func RunGraph(p build.Phase, in []byte, from *graph.Graph, w io.Writer) ([]byte, Times, error) {
	var t Times
	scratch, err := os.MkdirTemp("", fmt.Sprintf("graphcut%03d.", p.N))
	if err != nil {
		return nil, t, err
	}
	defer os.RemoveAll(scratch)
	path := filepath.Join(scratch, "whim-vim.c")
	text := in
	var e *graph.Editor
	flush := func() error {
		if e == nil {
			return nil
		}
		var err error
		t.Print.add(measure(func() { text, err = e.Graph().C() }))
		e = nil
		return err
	}
	for i, s := range p.Steps {
		f, ok := GraphSteps(s)
		if !ok {
			if err := flush(); err != nil {
				return nil, t, err
			}
			one := p
			one.Steps = []build.Step{s}
			var err error
			t.Steps.add(measure(func() { text, err = build.RunPhase(one, text, scratch, w) }))
			if err != nil {
				return nil, t, err
			}
			continue
		}
		if e == nil {
			g := from
			if i > 0 || g == nil {
				var err error
				t.Import.add(measure(func() { g, _, err = graph.Import(path, text) }))
				if err != nil {
					return nil, t, err
				}
			}
			t.Index.add(measure(func() { e = graph.NewEditor(g) }))
		}
		var err error
		t.Edit.add(measure(func() { err = f(e, w) }))
		if err != nil {
			return nil, t, err
		}
	}
	if e == nil {
		out, err := finish(text, scratch, &t)
		return out, t, err
	}
	g := e.Graph()
	t.Collect = measure(func() { _, err = graph.Collect(g, whim.GraphCollect()) })
	if err != nil {
		return nil, t, fmt.Errorf("collect: %w", err)
	}
	var out []byte
	t.Print.add(measure(func() { out, err = g.C() }))
	return out, t, err
}

// emptyGoes is a control: set, the closure takes the blocks it empties, its
// own rule, which the text cutters do not have.
var emptyGoes bool

// fallOut is what the closure is told: vim's (whim.GraphFallOut).
func fallOut() graph.FallOutOptions {
	o := whim.GraphFallOut
	if emptyGoes {
		o.KeepEmpty = false
	}
	return o
}
