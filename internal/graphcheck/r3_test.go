package graphcheck

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/build"
)

// TestSeedOnGraph (R3, doc/GRAPH-MIGRATION.md): phase 0 on the graph from
// the input (GRAPH_INPUT, slim-vim.c). The import's C view is the seed's
// canonical print byte for byte -- so a run imports the input once, and the
// seed is that import; the phase's parts 0a-0c on it, collected, give
// q000.c byte for byte; and the graph the phase hands phase 1 is the import
// of q000.c, ids aside (SameGraph), as every later phase's is
// (TestGraphPhasesAsImported).
func TestSeedOnGraph(t *testing.T) {
	dir, _, src, _ := setup(t)
	if src == nil {
		t.Skip("GRAPH_INPUT is not set")
	}
	g, text, _, err := pipeline.SeedGraph(src, filepath.Join(t.TempDir(), "whim-vim.c"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	canon, err := build.Seed(src, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(text, canon) {
		t.Fatalf("the import's C view is not the canonical print: %s", firstDiff(text, canon))
	}
	var log bytes.Buffer
	out, conv, err := build.AdvanceFrom(build.Plan[0], text, g, &log)
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	if want := snapOf(t, dir, 0); !bytes.Equal(out, want) {
		t.Fatalf("phase 0 on the graph is not q000.c: %s", firstDiff(out, want))
	}
	t.Logf("%s", conv)
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	i, _, err := graph.Import(filepath.Join(t.TempDir(), "whim-vim.c"), out)
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.SameGraph(h, i); err != nil {
		t.Fatalf("phase 0's graph is not the import of q000.c: %v", err)
	}
}
