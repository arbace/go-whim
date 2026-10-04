package whim

import (
	"bytes"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/xform"
)

// The knobs crefactor/xform's transformations are built with, for
// vim: what those phases hard-coded before they became library code.  A
// count a phase refuses under is not here; it is an argument in the plan.

// Core is where whim-vim.c's core ends: the line break before the first
// `#include`, which is the line between the editor core and its host
// (phases 43 on).  Phases 60, 62, 74 and 87a hard-coded it; on the graph it
// is the first include form (GraphCore).
var Core xform.Core = func(text []byte) int { return bytes.Index(text, []byte("\n#include ")) }

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
var Nullptr = xform.NullptrKnobs{
	NullLiterals: []string{
		`"E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"`,
		`"[NULL]"`,
		`"NULL"`,
	},
}

// Includes is phase 88's compiler question, record 82's before it: a header
// is unnecessary when the file still compiles with NOTHING printed, under the
// sweep's warnings.
var Includes = xform.Silent{
	Cmd:   "gcc",
	Flags: []string{"-fsyntax-only", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter"},
	Same:  true, // phase 43's static_asserts compare the core's limits with the headers'
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
