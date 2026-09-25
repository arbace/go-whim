package whim

import (
	"bytes"

	"github.com/arbace/go-whim/crefactor/xform"
)

// The knobs crefactor/xform's transformations are built with, for
// vim: what those phases hard-coded before they became library code.  A
// count a phase refuses under is not here; it is an argument in the plan.

// Core is where whim-vim.c's core ends: the line break before the first
// `#include`, which is the line between the editor core and its host
// (phases 110 on).  Phases 132, 134, 149 and 166 hard-coded it.
var Core xform.Core = func(text []byte) int { return bytes.Index(text, []byte("\n#include ")) }

// DropCalls is phase 132's: since phase 124 host_free() has an empty body, so
// vim_free(), a NULL test around it, does nothing either; the host's
// formatter called the core's vim_free(), and calls its own host_free().
var DropCalls = xform.DropCallsKnobs{
	Core:     Core,
	Funcs:    []string{"vim_free", "host_free"},
	Redirect: [][2]string{{"vim_free", "host_free"}},
}

// NeverNull is phase 149's: host_alloc() returns a pointer into the arena or
// ends the process (phase 148).
var NeverNull = xform.NeverNullKnobs{
	Core:  Core,
	Roots: []string{"host_alloc"},
}

// BoolRet is phase 166's: vim's truth constants, TRUE and OK beside true,
// FALSE and FAIL beside false; main, whose int is the process's; and the
// sweep's layout guard, which says which members a positional initialiser
// fills.
var BoolRet = xform.BoolRetKnobs{
	Core:   Core,
	True:   []string{"TRUE", "OK"},
	False:  []string{"FALSE", "FAIL"},
	Keep:   []string{"main"},
	Layout: Profile.Sweep,
}

// Nullptr is phase 106's: the three string literals that hold `NULL` and
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

// Includes is phase 169's compiler question, phase 82's before it: a header
// is unnecessary when the file still compiles with NOTHING printed, under the
// sweep's warnings.
var Includes = xform.Silent{
	Cmd:   "gcc",
	Flags: []string{"-fsyntax-only", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter"},
}

// Own114 is phase 114's: abs and labs, the two libc functions the core called
// without a body of its own, become musl's, written above musl_bsearch with
// the other <stdlib.h> functions the core owns.
var Own114 = xform.OwnKnobs{
	Prefix: "musl_",
	Funcs: []xform.OwnFunc{
		{Name: "abs", Proto: "int abs(int n);", Def: "    static int\nmusl_abs(int a)\n{\n    return a > 0 ? a : -a;\n}\n\n"},
		{Name: "labs", Proto: "long labs(long n);", Def: "    static long\nmusl_labs(long a)\n{\n    return a > 0 ? a : -a;\n}\n\n"},
	},
	Before: "    static void *\nmusl_bsearch(",
}
