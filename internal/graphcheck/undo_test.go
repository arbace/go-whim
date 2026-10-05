package graphcheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/pipeline"
	"github.com/arbace/go-whim/internal/build"
)

// TestUndoPhases holds the journal (crefactor/graph's journal.go) to every
// phase: the phase run on its snapshot's graph under a journal, its output
// its snapshot's, then undone -- the graph's Lisp the snapshot's byte for
// byte, every id, edge and section as it was.  A field an edit changes
// without saving it first is found here, in the phase that changes it.
func TestUndoPhases(t *testing.T) {
	dir, only, _, _ := setup(t)
	for _, p := range graphPhases() {
		if os.Getenv("GRAPH_PHASES") != "" && !slices.Contains(only, p.N) {
			continue
		}
		if !pipeline.BeginsOnGraph(p) {
			continue
		}
		t.Run(fmt.Sprint(p.N), func(t *testing.T) {
			b, err := graph.ReadStoreLisp(dir, fmt.Sprintf("q%03d", p.N-1))
			if err != nil {
				t.Skipf("no graph q%03d in the store: %v", p.N-1, err)
			}
			g, err := graph.Read(b)
			if err != nil {
				t.Fatal(err)
			}
			before := g.Lisp()
			j := g.Begin()
			out, _, err := build.AdvanceFrom(p, snapOf(t, dir, p.N-1), g, io.Discard)
			j.End()
			if err != nil {
				t.Fatal(err)
			}
			if want := snapOf(t, dir, p.N); !bytes.Equal(out, want) {
				t.Fatalf("under a journal: %d bytes, q%03d.c %d", len(out), p.N, len(want))
			}
			j.Undo()
			after := g.Lisp()
			if !bytes.Equal(after, before) {
				t.Fatalf("undone (%d nodes saved), the graph differs from q%03d's: %s", j.Changed(), p.N-1, lineDiff(before, after))
			}
			t.Logf("%d nodes saved, undone", j.Changed())
		})
	}
}

// lineDiff is the first line where a and b differ, each side.
func lineDiff(a, b []byte) string {
	al, bl := bytes.Split(a, []byte("\n")), bytes.Split(b, []byte("\n"))
	for i := range min(len(al), len(bl)) {
		if !bytes.Equal(al[i], bl[i]) {
			return fmt.Sprintf("line %d:\n  was %.300s\n  now %.300s", i+1, al[i], bl[i])
		}
	}
	return fmt.Sprintf("%d lines, %d", len(al), len(bl))
}
