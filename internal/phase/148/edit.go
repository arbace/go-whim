package p148

// Whim phase 148 -- allocation cannot fail.  See GOAL.md.
//
// host_alloc() never returns NULL, and lalloc()'s one other NULL -- a request
// for zero bytes, an internal error -- now reports the error and returns
// host_alloc(0).  So no allocation in the core can fail (internal/gen/FINDINGS.md, 9).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim148", Edit) }

// W148LallocBody is lalloc()'s Body after this phase, inside its braces.
const W148LallocBody = `    if (size == 0)
    {
        emsg_silent = 0;
        iemsg(e_internal_error_lalloc_zero);
    }

    return host_alloc(size);
`

// Whim148 makes allocation unable to fail.
//
// lalloc() -- which alloc(), alloc_clear() and lalloc_clear() call -- returned
// NULL in two cases.  host_alloc() returning NULL, which it never does: the
// arena's allocator ends the process through host_exit() when it is full, and
// otherwise returns a pointer into the arena.  So the Out-of-memory branches
// that released the scrollback and said E342 were dead.  And a request for
// zero bytes, which reported E341, an internal error, and returned NULL.  That
// one now reports the same error and returns host_alloc(0) -- a pointer to no
// bytes, where NULL was -- so that no allocation in the core can fail, and the
// failure branches after every allocation are dead (internal/gen/FINDINGS.md, 9): the
// next phase folds them.  The Go transpilation had dropped them already.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nofail", text, w)
	e.Body("lalloc", W148LallocBody,
		"lalloc() returns what host_alloc() gives, which is never NULL, after reporting a request for zero bytes")
	return e.Done()
}
