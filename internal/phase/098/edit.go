package p098

// Whim phase 98 (formerly 179) -- no mark is cleared when none was set.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim98", Edit) }

// Edit returns from ml_clearmarked() when no line is marked: lowest_marked
// is 0 then, and its loop, starting at line 0, read the slot before the
// first line of a block -- index -1: undefined in the C, a panic in the Go
// and the Java.  Line 0 is never marked, so nothing it cleared is left.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the condition found by its form
// and written anew by FRAG; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("clearmarked", e, w)
	v.InFunction("ml_clearmarked", func(v *graph.Verbs) {
		v.ReplaceC("(== (. (-> curbuf b_ml) ml_root) nullptr)", "curbuf->b_ml.ml_root == nullptr || lowest_marked == 0", 1,
			"ml_clearmarked returns when nothing is marked, before it reads line 0")
	})
	return v.Done()
}
