package edit

import (
	"io"

	"github.com/arbace/go-whim/internal/cutil"
)

func init() { register("whim130", Whim130) }

// Whim130 removes the three tests for (pos_T *)-1.
//
// get_address(), nv_gomark() and nv_pcmark() each compare a mark lookup's
// result with (pos_T *)-1, the value vim once returned for "a mark in another
// file" -- and nothing in this tree returns it: getmark() is
// getmark_buf_fnum(), which returns a pointer into the buffer or NULL, and
// movechangelist() returns NULL or an element of b_changelist.  So each test
// is an `if` that is never taken, and folds away with its body: the else
// branch stays, dedented, and an `else if` becomes the `if`.  The Go
// transpilation had to write each as `if false` (tx/FINDINGS.md, 10).
func Whim130(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "sentinel", w: w}
	out, err := cutil.FoldNever(text, `if \([a-z]+ == \(pos_T \*\)-1\)`, 3)
	if err != nil {
		return nil, p.die("%v", err)
	}
	p.say("the three tests for (pos_T *)-1 fold away, each keeping the branch that runs")
	return out, nil
}
