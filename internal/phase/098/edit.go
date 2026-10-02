package p098

// Whim phase 98 (formerly 179) -- no mark is cleared when none was set.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim98", Edit) }

// Edit returns from ml_clearmarked() when no line is marked: lowest_marked
// is 0 then, and its loop, starting at line 0, read the slot before the
// first line of a block -- index -1: undefined in the C, a panic in the Go
// and the Java.  Line 0 is never marked, so nothing it cleared is left.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("clearmarked", text, w)
	e.InFunction("ml_clearmarked", func(e *edit.E) {
		e.Literal("    if (curbuf->b_ml.ml_root == nullptr)\n",
			"    if (curbuf->b_ml.ml_root == nullptr || lowest_marked == 0)\n", 1,
			"ml_clearmarked returns when nothing is marked, before it reads line 0")
	})
	return e.Done()
}
