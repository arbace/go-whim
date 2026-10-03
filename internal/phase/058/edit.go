package p058

// Whim phase 58 (formerly 130) -- the (pos_T *)-1 tests go.  See GOAL.md.
//
// get_address(), nv_gomark() and nv_pcmark() each compared a mark lookup with
// (pos_T *)-1, vim's old "mark in another file" -- and nothing in this tree
// returns it.  Each test is an if never taken, and FoldNever folds the three
// away keeping the branch that runs (internal/gen/FINDINGS.md, 10).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1c): the text program's one
// FoldNever, the ifs found by their condition's form instead of a regular
// expression on their lines; history keeps the text version.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim58", Edit) }

// Edit removes the three tests for (pos_T *)-1.
//
// get_address(), nv_gomark() and nv_pcmark() each compare a mark lookup's
// result with (pos_T *)-1, the value vim once returned for "a mark in another
// file" -- and nothing in this tree returns it: getmark() is
// getmark_buf_fnum(), which returns a pointer into the buffer or NULL, and
// movechangelist() returns NULL or an element of b_changelist.  So each test
// is an `if` that is never taken, and folds away: the else branch stays,
// its items spliced where the if was, and an `else if` becomes the `if`.
// The Go transpilation had to write each as `if false`
// (internal/gen/FINDINGS.md, 10).
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("sentinel", e, w)
	v.FoldNever("(== ?p (cast (ptr pos_T) (- 1)))", 3,
		"the three tests for (pos_T *)-1 fold away, each keeping the branch that runs")
	return v.Done()
}
