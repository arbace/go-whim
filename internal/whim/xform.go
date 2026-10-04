package whim

import (
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

// The knobs crefactor/graph's transformations are built with, for
// vim: what those phases hard-coded before they became library code.  A
// count a phase refuses under is not here; it is an argument in the plan.

// DropCalls is phase 60's (crefactor/graph's Editor.DropCalls, its In the
// core, GraphCore): since phase 52 host_free() has an empty body, so
// vim_free(), a NULL test around it, does nothing either; the host's
// formatter called the core's vim_free(), and calls its own host_free().
// A local left only given values goes as crefactor/edit's DeadStores took
// it: a value whose text PureCond passes.
var DropCalls = graph.DropCallsOptions{
	Funcs:    []string{"vim_free", "host_free"},
	Redirect: [][2]string{{"vim_free", "host_free"}},
	Cond:     edit.PureCond,
}

// NeverNull is phase 74's (crefactor/graph's Editor.NeverNull, its In the
// core): host_alloc() returns a pointer into the arena or ends the process
// (phase 74a).
var NeverNull = graph.NeverNullOptions{
	Roots: []string{"host_alloc"},
}

// Nullptr is phase 0a's, run by the seed: the three string literals that hold `NULL` and
// stay as they are -- a message, the printf layer's stand-in for a null %s,
// and what an empty growarray prints.  A fourth would be a message the phase
// has never seen, and refuses.
var Nullptr = graph.NullptrKnobs{
	NullLiterals: []string{
		`"E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"`,
		`"[NULL]"`,
		`"NULL"`,
	},
}

// GotoTail is phase 89's bound (crefactor/graph's Editor.GotoTail): a
// label's tail is copied over a goto when it is at most three statements
// before its return.  Three is the longest straight tail a goto of the core
// reaches (a history browser's three stores), and each copy costs its
// length.
const GotoTail = 3

// Own47 is phase 47's: abs and labs, the two libc functions the core called
// without a body of its own, become musl's, written above musl_bsearch with
// the other <stdlib.h> functions the core owns (crefactor/graph's Own, on the
// graph since B3d).
var Own47 = graph.OwnKnobs{
	Prefix: "musl_",
	Funcs: []graph.OwnFunc{
		{Name: "abs", Proto: "int abs(int n);", Def: "    static int\nmusl_abs(int a)\n{\n    return a > 0 ? a : -a;\n}\n\n"},
		{Name: "labs", Proto: "long labs(long n);", Def: "    static long\nmusl_labs(long a)\n{\n    return a > 0 ? a : -a;\n}\n\n"},
	},
	Before: "musl_bsearch",
}
