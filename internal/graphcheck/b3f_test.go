package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// TestB3fDiff runs GRAPH_PHASES' phases from their snapshot, as a run in
// order would (the graph imported where its graph steps begin), and where
// one does not give qN.c writes what it gave beside the snapshot's and
// shows the diff: the converter's view of what TestPhasesOnGraph only
// counts.
func TestB3fDiff(t *testing.T) {
	if os.Getenv("GRAPH_PHASES") == "" {
		t.Skip("GRAPH_PHASES is not set")
	}
	dir, only, _, _ := setup(t)
	out := filepath.Join(os.TempDir(), "b3f-diff")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range only {
		p := build.Plan[n]
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			in, want := snapOf(t, dir, n-1), snapOf(t, dir, n)
			var log bytes.Buffer
			got, conv, err := build.AdvanceFrom(p, in, nil, &log)
			t.Logf("%s\n%s", log.String(), conv)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(got, want) {
				return
			}
			g := filepath.Join(out, fmt.Sprintf("got%03d.c", n))
			os.WriteFile(g, got, 0o644)
			d, _ := exec.Command("diff", "-u", filepath.Join(dir, fmt.Sprintf("q%03d.c", n)), g).CombinedOutput()
			if len(d) > 6000 {
				d = d[:6000]
			}
			t.Fatalf("phase %d differs:\n%s", n, d)
		})
	}
}

// TestB3fSame runs GRAPH_PHASES' phases whose every step is a registered
// graph edit on q(N-1)'s graph, collects, and holds the graph -- not only
// its C view -- to the import of qN.c (graph.SameGraph): what a phase hands
// the next one is what an import of its text would be.
func TestB3fSame(t *testing.T) {
	if os.Getenv("GRAPH_PHASES") == "" {
		t.Skip("GRAPH_PHASES is not set")
	}
	dir, only, _, _ := setup(t)
	for _, n := range only {
		p := build.Plan[n]
		var fs []func(*graph.Editor) error
		for _, s := range p.Steps {
			if s.Op != "edit" || !s.Graph {
				fs = nil
				break
			}
			f, ok := phase.LookupGraph(s.Args[0])
			if !ok {
				t.Fatalf("%s is not registered", s.Args[0])
			}
			args := s.Args[1:]
			fs = append(fs, func(e *graph.Editor) error { return f(e, io.Discard, args) })
		}
		if fs == nil {
			continue
		}
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			in, want := snapOf(t, dir, n-1), snapOf(t, dir, n)
			path := filepath.Join(t.TempDir(), "whim-vim.c")
			g, _, err := graph.Import(path, in)
			if err != nil {
				t.Fatal(err)
			}
			e := graph.NewEditor(g)
			for _, f := range fs {
				if err := f(e); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := e.Collect(whim.GraphCollect()); err != nil {
				t.Fatal(err)
			}
			got, err := g.C()
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("its C view is not q%03d.c (%v)", n, err)
			}
			h, _, err := graph.Import(path, want)
			if err != nil {
				t.Fatal(err)
			}
			if err := graph.SameGraph(g, h); err != nil {
				t.Fatalf("the graph is not the import of q%03d.c: %v", n, err)
			}
			t.Logf("%d untyped", len(e.Untyped))
		})
	}
}
