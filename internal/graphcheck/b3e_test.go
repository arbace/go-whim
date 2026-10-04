package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim"
)

// TestB3eDrafts holds phases 53-56, whose acts are text acts on the C view
// committed as FRAG (graph.Draft; doc/GRAPH-MIGRATION.md, *B3e as built*),
// to more than their bytes: on q(N-1)'s graph read back from its Lisp, the
// phase's program leaves an index that checks, a graph that reads back the
// same, and, collected, qN.c byte for byte and the graph an import of that
// text gives -- every refers edge and typed edge, the carried uses of the
// structs written anew among them, ids aside.
func TestB3eDrafts(t *testing.T) {
	dir, _, _, _ := setup(t)
	for n := 53; n <= 56; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			in, want := snapOf(t, dir, n-1), snapOf(t, dir, n)
			path := filepath.Join(t.TempDir(), "whim-vim.c")
			g, _, err := graph.Import(path, in)
			if err != nil {
				t.Fatal(err)
			}
			h, err := graph.Read(g.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			e := graph.NewEditor(h)
			f, ok := phase.LookupGraph(fmt.Sprintf("whim%d", n))
			if !ok {
				t.Fatalf("whim%d is not a graph program", n)
			}
			t0 := time.Now()
			if err := f(e, io.Discard, []string{t.TempDir()}); err != nil {
				t.Fatal(err)
			}
			took := time.Since(t0)
			if err := e.Check(); err != nil {
				t.Fatal(err)
			}
			r, err := graph.Read(h.Lisp())
			if err != nil {
				t.Fatal(err)
			}
			if err := graph.Equal(h, r); err != nil {
				t.Fatalf("read back: %v", err)
			}
			if _, err := e.Collect(whim.GraphCollect()); err != nil {
				t.Fatal(err)
			}
			got, err := h.C()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("collected: %d bytes, q%03d.c %d\n%s", len(got), n, len(want), diffAtB2c(got, want))
			}
			back, _, err := graph.Import(path, got)
			if err != nil {
				t.Fatal(err)
			}
			if err := graph.SameGraph(h, back); err != nil {
				t.Fatalf("collected, the graph is not the import of its C view: %v", err)
			}
			given, gone := 0, 0
			for _, a := range e.Log {
				given += len(a.New)
				gone += len(a.Gone)
			}
			t.Logf("phase %d on the graph %v: %d acts, %d ids given, %d superseded, %d untyped",
				n, took.Round(time.Millisecond), len(e.Log), given, gone, len(e.Untyped))
		})
	}
}
