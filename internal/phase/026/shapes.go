package p026

import (
	"regexp"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// This phase's own shapes: they were in internal/whim/vimtext, which holds
// only what more than one phase uses, and only this phase uses these.

var (
	skipRe = regexp.MustCompile(`\b(ea\.|eap->)skip\b`)
	// a cmdnames[] row: its designated index and its name
	clispRow = clisp.MustPattern(`(at (idx _) (init (cast (ptr char_u) ?name) _*))`)
	// the argument before eap->skip in do_addr_type's call
	clispAddrType = clisp.MustPattern(`(-> eap addr_type)`)
)
