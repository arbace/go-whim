package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/build"
)

// TestB3cAsImported holds B3c's phases (36-42, GRAPH_PHASES' among them) to
// more than their bytes: each, handed q(N-1)'s graph read back, leaves a
// graph that is the import of its C view, ids aside (SameGraph) -- every
// refers edge where cc's check puts it, every typed edge of the same
// structure, nothing the substitutions, the fragments, the renames, the
// retypes and the dropped parameters made left pointing at what an import
// would not.
func TestB3cAsImported(t *testing.T) {
	dir, only, _, _ := setup(t)
	for _, p := range build.Plan {
		if p.N < 36 || p.N > 42 || !pipeline.BeginsOnGraph(p) || !slices.Contains(only, p.N) {
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
}
