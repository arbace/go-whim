package p130

// Whim phase 130 -- the (pos_T *)-1 tests go.  See GOAL.md.
//
// get_address(), nv_gomark() and nv_pcmark() each compared a mark lookup with
// (pos_T *)-1, vim's old "mark in another file" -- and nothing in this tree
// returns it.  Each test is an if never taken, and cutil.FoldNever folds the
// three away keeping the branch that runs (internal/gen/FINDINGS.md, 10).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim130", Edit) }

// Whim130 removes the three tests for (pos_T *)-1.
//
// get_address(), nv_gomark() and nv_pcmark() each compare a mark lookup's
// result with (pos_T *)-1, the value vim once returned for "a mark in another
// file" -- and nothing in this tree returns it: getmark() is
// getmark_buf_fnum(), which returns a pointer into the buffer or NULL, and
// movechangelist() returns NULL or an element of b_changelist.  So each test
// is an `if` that is never taken, and folds away with its Body: the else
// branch stays, dedented, and an `else if` becomes the `if`.  The Go
// transpilation had to write each as `if false` (internal/gen/FINDINGS.md, 10).
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("sentinel", text, w)
	e.FoldNever(`if \([a-z]+ == \(pos_T \*\)-1\)`, 3,
		"the three tests for (pos_T *)-1 fold away, each keeping the branch that runs")
	return e.Done()
}
