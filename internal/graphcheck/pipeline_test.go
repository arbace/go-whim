package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/build"
)

// The pipeline on the graph (doc/GRAPH.md, step 5): the phases with a graph
// step, held one by one to the snapshots -- what `whim build --check` holds
// them to with the rest, here alone and both ways in, so that a change to a
// graph step can be tried in seconds.

// graphPhases are the plan's phases with a graph step.
func graphPhases() []build.Phase {
	var out []build.Phase
	for _, p := range build.Plan {
		if slices.ContainsFunc(p.Steps, func(s build.Step) bool { return s.Graph }) {
			out = append(out, p)
		}
	}
	return out
}

func snapOf(t *testing.T, dir string, n int) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("q%03d.c", n)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestPhasesOnGraph: each phase with a graph step gives qN from q(N-1), the
// graph imported where its graph steps begin; and one that begins on the
// graph gives it too handed q(N-1)'s graph written as Lisp and read back --
// the graph snapshot's own path, whether or not the snapshots hold one.
func TestPhasesOnGraph(t *testing.T) {
	dir, _, _, _ := setup(t)
	for _, p := range graphPhases() {
		t.Run(fmt.Sprint(p.N), func(t *testing.T) {
			in, want := snapOf(t, dir, p.N-1), snapOf(t, dir, p.N)
			out, conv, err := build.AdvanceFrom(p, in, nil, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, want) {
				t.Fatalf("imported: %d bytes, q%03d.c %d", len(out), p.N, len(want))
			}
			t.Logf("imported: %s", conv)
			if !pipeline.BeginsOnGraph(p) {
				return
			}
			g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
			if err != nil {
				t.Fatal(err)
			}
			h, err := graph.Read(g.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			out, conv, err = build.AdvanceFrom(p, in, h, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, want) {
				t.Fatalf("handed the graph: %d bytes, q%03d.c %d", len(out), p.N, len(want))
			}
			t.Logf("handed: %s", conv)
		})
	}
}

// TestGraphSnapshots: every graph snapshot a run kept (qNNN.g) is the graph
// of qNNN.c -- its header names that text's digest, and read back, its C
// view is the text byte for byte, and written again it reads back the same
// graph, ids and edges (step 2's gate on the pipeline's own graphs, which
// hold edits: superseded ids, fresh ones, the last id given).
func TestGraphSnapshots(t *testing.T) {
	dir, _, _, _ := setup(t)
	gs, _ := filepath.Glob(filepath.Join(dir, "q*.g"))
	if len(gs) == 0 {
		t.Skip("no graph snapshots in GRAPH_SNAPS")
	}
	for _, f := range gs {
		var n int
		if _, err := fmt.Sscanf(filepath.Base(f), "q%03d.g", &n); err != nil {
			t.Fatal(err)
		}
		text := snapOf(t, dir, n)
		g := build.GraphSnapshot(n, text)
		if g == nil {
			// build.GraphSnapshot reads .cache/boundaries: read this one
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if g, err = graph.Read(b); err != nil {
				t.Fatal(err)
			}
		}
		c, err := g.C()
		if err != nil || !bytes.Equal(c, text) {
			t.Fatalf("q%03d.g: its C view is not q%03d.c (%v)", n, n, err)
		}
		h, err := graph.Read(g.Lisp())
		if err != nil {
			t.Fatal(err)
		}
		if err := graph.Equal(g, h); err != nil {
			t.Fatalf("q%03d.g written again: %v", n, err)
		}
		t.Logf("q%03d.g: the graph of q%03d.c, %s", n, n, g.Count())
	}
}

// TestMeasureGraphPhases times each phase with a graph step one at a time,
// GRAPH_MEASURE runs each (or skipped), medians: as a run in order or the
// check without a graph snapshot runs it (the text in, the graph imported
// where its graph steps begin), and, for one that begins on the graph, as
// the check runs it from its graph snapshot (the Lisp read, timed apart).
func TestMeasureGraphPhases(t *testing.T) {
	dir, _, _, _ := setup(t)
	runs := 0
	fmt.Sscan(os.Getenv("GRAPH_MEASURE"), &runs)
	if runs == 0 {
		t.Skip("GRAPH_MEASURE is not set")
	}
	type cost struct{ wall, cpu time.Duration }
	median := func(cs []cost) cost {
		sort.Slice(cs, func(i, j int) bool { return cs[i].wall < cs[j].wall })
		return cs[len(cs)/2]
	}
	for _, p := range graphPhases() {
		in := snapOf(t, dir, p.N-1)
		var lisp []byte
		if pipeline.BeginsOnGraph(p) {
			g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
			if err != nil {
				t.Fatal(err)
			}
			lisp = g.Lisp()
		}
		var imp, handed, reads []cost
		var conv, hconv pipeline.Conv
		for range runs {
			w, c := time.Now(), cpuNow()
			_, cv, err := build.AdvanceFrom(p, in, nil, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			imp = append(imp, cost{time.Since(w), cpuNow() - c})
			conv = cv
			if lisp == nil {
				continue
			}
			w, c = time.Now(), cpuNow()
			g, err := graph.Read(lisp)
			if err != nil {
				t.Fatal(err)
			}
			reads = append(reads, cost{time.Since(w), cpuNow() - c})
			w, c = time.Now(), cpuNow()
			if _, hconv, err = build.AdvanceFrom(p, in, g, io.Discard); err != nil {
				t.Fatal(err)
			}
			handed = append(handed, cost{time.Since(w), cpuNow() - c})
		}
		m := median(imp)
		line := fmt.Sprintf("phase %3d  imported: wall %5d ms, cpu %5d ms (%s)", p.N, m.wall.Milliseconds(), m.cpu.Milliseconds(), conv)
		if lisp != nil {
			h, r := median(handed), median(reads)
			line += fmt.Sprintf("\n           handed:   wall %5d ms, cpu %5d ms, and the read %d ms (%s)", h.wall.Milliseconds(), h.cpu.Milliseconds(), r.wall.Milliseconds(), hconv)
		}
		fmt.Println(line)
	}
}

// TestMeasureCollect times the two ways a graph phase can go on editing
// after a collection (doc/GRAPH.md, step 5, item 3), GRAPH_MEASURE runs each,
// medians, on the text a few phases leave before their sweep: the
// collection through the editor (Editor.Collect: the index fixed up where
// the collection cut, the act logged), and the collection alone with the
// editor made anew (Collect, NewEditor).  Both leave the same C.
func TestMeasureCollect(t *testing.T) {
	dir, _, _, _ := setup(t)
	runs := 0
	fmt.Sscan(os.Getenv("GRAPH_MEASURE"), &runs)
	if runs == 0 {
		t.Skip("GRAPH_MEASURE is not set")
	}
	for _, n := range []int{1, 3, 8, 24, 30, 53, 60, 74, 89} {
		p := build.Plan[n]
		text, err := build.RunPhase(p, snapOf(t, dir, n-1), t.TempDir(), io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "whim-vim.c")
		g0, _, err := graph.Import(path, text)
		if err != nil {
			t.Fatal(err)
		}
		lisp := g0.Lisp()
		var through, anew []time.Duration
		var ca, cb []byte
		gone := 0
		for range runs {
			a, _ := graph.Read(lisp)
			e := graph.NewEditor(a)
			w := time.Now()
			if _, err := e.Collect(Options()); err != nil {
				t.Fatal(err)
			}
			through = append(through, time.Since(w))
			gone = len(e.Log[len(e.Log)-1].Gone)
			b, _ := graph.Read(lisp)
			graph.NewEditor(b)
			w = time.Now()
			if _, err := graph.Collect(b, Options()); err != nil {
				t.Fatal(err)
			}
			graph.NewEditor(b)
			anew = append(anew, time.Since(w))
			ca, _ = a.C()
			cb, _ = b.C()
		}
		if !bytes.Equal(ca, cb) || !bytes.Equal(ca, snapOf(t, dir, n)) {
			t.Fatalf("phase %d: the two collections differ, or are not q%03d.c", n, n)
		}
		sort.Slice(through, func(i, j int) bool { return through[i] < through[j] })
		sort.Slice(anew, func(i, j int) bool { return anew[i] < anew[j] })
		fmt.Printf("phase %3d  %7d ids collected: through the editor %4d ms, collected and indexed anew %4d ms\n",
			n, gone, through[runs/2].Milliseconds(), anew[runs/2].Milliseconds())
	}
}
