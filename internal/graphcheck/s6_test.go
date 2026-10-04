package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/build"
)

// TestGraphPhasesAsImported holds every phase that runs on the graph from
// end to end (each of its steps a graph step) to more than its bytes:
// handed q(N-1)'s graph read back from its Lisp, it gives qN.c byte for
// byte AND leaves a graph that is the import of its C view, ids aside
// (graph.SameGraph): every refers edge where cc's check puts it, every
// typed edge to a type of the same structure, nothing untyped that the
// import types.  What a phase hands the next one is what an import of its
// text would be.  It generalizes B3c's, B3d's and B3f's tests of their own
// ranges (TestB3cAsImported, TestB3dHandsOnTheImport, TestB3fSame) to every
// graph phase; GRAPH_PHASES narrows it, GRAPH_JOBS runs that many at once
// (default 4: each holds two imports).
func TestGraphPhasesAsImported(t *testing.T) {
	dir, only, _, _ := setup(t)
	jobs := 4
	if s := os.Getenv("GRAPH_JOBS"); s != "" {
		jobs, _ = strconv.Atoi(s)
	}
	sem := make(chan struct{}, max(jobs, 1))
	t.Run("phases", func(t *testing.T) {
		for _, p := range build.Plan {
			if !slices.Contains(only, p.N) || !s6AllGraph(p) {
				continue
			}
			t.Run(fmt.Sprint(p.N), func(t *testing.T) {
				t.Parallel()
				sem <- struct{}{}
				defer func() { <-sem }()
				in, want := snapOf(t, dir, p.N-1), snapOf(t, dir, p.N)
				g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
				if err != nil {
					t.Fatal(err)
				}
				h, err := graph.Read(g.Lisp())
				if err != nil {
					t.Fatal(err)
				}
				g = nil
				out, _, err := build.AdvanceFrom(p, in, h, io.Discard)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(out, want) {
					t.Fatalf("%d bytes, q%03d.c %d", len(out), p.N, len(want))
				}
				i, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), out)
				if err != nil {
					t.Fatal(err)
				}
				if err := graph.SameGraph(h, i); err != nil {
					t.Fatalf("phase %d's graph is not the import of its C view: %v", p.N, err)
				}
			})
		}
	})
}

// s6AllGraph says every step of p is a graph step.
func s6AllGraph(p build.Phase) bool {
	if len(p.Steps) == 0 {
		return false
	}
	for _, s := range p.Steps {
		if !s.Graph && s.Op != "sweep" {
			return false
		}
	}
	return true
}
