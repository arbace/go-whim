package edit

import (
	"io"

	"github.com/arbace/go-whim/internal/cutil"
)

func init() { register("whim148", Whim148) }

// W148LallocBody is lalloc()'s body after this phase, inside its braces.
const W148LallocBody = `    if (size == 0)
    {
        emsg_silent = 0;
        iemsg(e_internal_error_lalloc_zero);
    }

    return host_alloc(size);`

// Whim148 makes allocation unable to fail.
//
// lalloc() -- which alloc(), alloc_clear() and lalloc_clear() call -- returned
// NULL in two cases.  host_alloc() returning NULL, which it never does: the
// arena's allocator ends the process through host_exit() when it is full, and
// otherwise returns a pointer into the arena.  So the out-of-memory branches
// that released the scrollback and said E342 were dead.  And a request for
// zero bytes, which reported E341, an internal error, and returned NULL.  That
// one now reports the same error and returns host_alloc(0) -- a pointer to no
// bytes, where NULL was -- so that no allocation in the core can fail, and the
// failure branches after every allocation are dead (tx/FINDINGS.md, 9): the
// next phase folds them.  The Go transpilation had dropped them already.
func Whim148(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "nofail", w: w}
	out, _, err := cutil.ReplaceBody(text, "lalloc", W148LallocBody)
	if err != nil {
		return nil, p.die("lalloc: %v", err)
	}
	p.say("lalloc() returns what host_alloc() gives, which is never NULL, after reporting a request for zero bytes")
	return out, nil
}
