// Package graphcheck holds crefactor/graph's sweep, Collect, to the
// pipeline's (doc/GRAPH.md, step 3): on each phase's text before its sweep
// -- q(N-1) through the phase's steps -- the collected graph's C view must
// be qN, the snapshot the sweep and the canonical print made, byte for
// byte.  It is whim's: it runs whim's plan and tells the collector vim's
// roots and guard (internal/whim's Profile).
package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/sweep"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/whim"
)

// Options are the collector's, from vim's profile: the sweep's roots and
// its guard.
func Options() graph.CollectOptions {
	return graph.CollectOptions{Roots: whim.Profile.Sweep.Roots, FreezeLayoutIf: whim.Profile.Sweep.FreezeLayoutIf}
}

// A Result is one phase's.
type Result struct {
	N      int
	Same   bool   // the collected graph's C view is qN
	Diff   string // where it is not
	Stats  graph.CollectStats
	Sweep  sweep.Stats
	Counts graph.Counts  // the graph collected
	Report *graph.Report // the import's, of the text before the sweep

	// Prune is crefactor/sweep's, its parses included; Import is the
	// graph's (cc's parse and check, the conversion, the resolution);
	// Collect is the collection alone; Print the C view.  Wall and CPU.
	Prune, Import, Collect, Print Cost
}

// A Cost is a segment's wall and CPU time.
type Cost struct{ Wall, CPU time.Duration }

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

// Snap is snapshot n of dir.
func Snap(dir string, n int) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, fmt.Sprintf("q%03d.c", n)))
}

// PreSweep is phase n's text before its sweep: q(n-1) -- for phase 0 the
// seed of src, the input -- through the phase's steps as the plan runs
// them.  ok is false for a phase that edits nothing.
func PreSweep(dir string, n int, src []byte) (pre []byte, ok bool, err error) {
	p := build.Plan[n]
	if p.N != n {
		return nil, false, fmt.Errorf("the plan's phase %d is numbered %d", n, p.N)
	}
	if p.NoSource {
		return nil, false, nil
	}
	var in []byte
	if n == 0 {
		if src == nil {
			return nil, false, fmt.Errorf("phase 0 needs the input")
		}
		if in, err = build.Seed(src, io.Discard); err != nil {
			return nil, false, err
		}
	} else if in, err = Snap(dir, n-1); err != nil {
		return nil, false, err
	}
	scratch, err := os.MkdirTemp("", fmt.Sprintf("graphcheck%03d.", n))
	if err != nil {
		return nil, false, err
	}
	defer os.RemoveAll(scratch)
	pre, err = build.RunPhase(p, in, scratch, io.Discard)
	return pre, err == nil, err
}

// Check is step 3's gate on phase n's text before its sweep, pre: the
// sweep as the pipeline runs it (timed), and the graph imported from pre,
// collected and printed, held to want, qN.  control, when set, is applied
// to the collected graph before it is printed: a deliberate fault the gate
// must catch.
func Check(n int, pre, want []byte, opt graph.CollectOptions, control func(*graph.Graph)) (Result, error) {
	r := Result{N: n}
	path := filepath.Join(os.TempDir(), "whim-vim.c")
	var err error
	r.Prune = measure(func() { _, r.Sweep, _, err = sweep.PruneParsed(pre, path, whim.Profile.Sweep) })
	if err != nil {
		return r, fmt.Errorf("phase %d: sweep: %w", n, err)
	}
	var g *graph.Graph
	var rep *graph.Report
	r.Import = measure(func() { g, rep, err = graph.Import(path, pre) })
	if err != nil {
		return r, fmt.Errorf("phase %d: import: %w", n, err)
	}
	r.Report = rep
	r.Collect = measure(func() { r.Stats, err = graph.Collect(g, opt) })
	if err != nil {
		return r, fmt.Errorf("phase %d: collect: %w", n, err)
	}
	if control != nil {
		control(g)
	}
	var out []byte
	r.Print = measure(func() { out, err = g.C() })
	if err != nil {
		return r, fmt.Errorf("phase %d: C view: %w", n, err)
	}
	r.Counts = g.Count()
	r.Same = bytes.Equal(out, want)
	if !r.Same {
		r.Diff = firstDiff(out, want)
	}
	return r, nil
}

// firstDiff says where two texts part.
func firstDiff(a, b []byte) string {
	line := 1
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			s := bytes.LastIndexByte(a[:i], '\n') + 1
			ea := bytes.IndexByte(a[i:], '\n')
			eb := bytes.IndexByte(b[i:], '\n')
			if ea < 0 {
				ea = len(a) - i
			}
			if eb < 0 {
				eb = len(b) - i
			}
			return fmt.Sprintf("line %d: got `%s`, want `%s`", line, a[s:i+ea], b[s:i+eb])
		}
		if a[i] == '\n' {
			line++
		}
	}
	return fmt.Sprintf("lengths %d and %d", len(a), len(b))
}

// StatsDiffer says where the collection's counts are not the sweep's, by
// kind: what each took should be the same things.
func (r Result) StatsDiffer() string {
	a, b := r.Stats, r.Sweep
	var out []string
	for _, x := range []struct {
		name string
		g, s int
	}{
		{"functions", a.Funcs, b.Funcs}, {"objects", a.Objects, b.Objects}, {"prototypes", a.Protos, b.Protos},
		{"typedefs", a.Typedefs, b.Typedefs}, {"tags", a.Tags, b.Tags}, {"members", a.Members, b.Members},
		{"enumerators", a.Enumerators, b.Enumerators}, {"locals", a.Locals, b.Locals}, {"fallthroughs", a.Fallthroughs, b.Fallthroughs}, {"pinned", a.Pinned, b.Pinned},
		{"kept runs", a.KeptRuns, b.KeptRuns}, {"positional", a.Positional, b.Positional}, {"rounds", a.Rounds, b.Rounds},
	} {
		if x.g != x.s {
			out = append(out, fmt.Sprintf("%s %d against %d", x.name, x.g, x.s))
		}
	}
	return strings.Join(out, ", ")
}
