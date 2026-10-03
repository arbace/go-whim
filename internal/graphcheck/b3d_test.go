package graphcheck

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// B3d's phases (43-52) hand the graph on to each other with no import
// between them, so what a phase leaves must be what an import of its text
// gives -- not only the same C view, which TestPhasesOnGraph holds, but the
// same graph: every refers edge where the importer puts it (a token made a
// use of the core's enumerator, an edge retargeted to the declaration cc
// now resolves it to), every typed edge to a type of the same structure.
// Each phase whose steps are all graph edits runs on q(N-1)'s graph read
// back, its steps' programs called directly, the collection after them;
// the result is held to qN.c's import by SameGraph, ids aside.
func TestB3dHandsOnTheImport(t *testing.T) {
	dir, only, _, _ := setup(t)
	for _, p := range build.Plan {
		if p.N < 43 || p.N > 52 || os.Getenv("GRAPH_PHASES") != "" && !slices.Contains(only, p.N) {
			continue
		}
		all := true
		for _, s := range p.Steps {
			all = all && s.Graph && s.Op == "edit"
		}
		if !all {
			continue
		}
		t.Run(fmt.Sprint(p.N), func(t *testing.T) {
			in, want := snapOf(t, dir, p.N-1), snapOf(t, dir, p.N)
			g, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), in)
			if err != nil {
				t.Fatal(err)
			}
			h, err := graph.Read(g.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			e := graph.NewEditor(h)
			for _, s := range p.Steps {
				f, ok := phase.LookupGraph(s.Args[0])
				if !ok {
					t.Fatalf("no graph edit %s", s.Args[0])
				}
				if err := f(e, io.Discard, s.Args[1:]); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := e.Collect(whim.GraphCollect()); err != nil {
				t.Fatal(err)
			}
			got, err := e.Graph().C()
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("its C view is not q%03d.c", p.N)
			}
			imp, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), want)
			if err != nil {
				t.Fatal(err)
			}
			if err := graph.SameGraph(e.Graph(), imp); err != nil {
				t.Fatalf("the graph phase %d hands on is not q%03d.c's import: %v", p.N, p.N, err)
			}
		})
	}
}
