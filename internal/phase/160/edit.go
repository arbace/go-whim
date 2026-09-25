package p160

// Whim phase 160 -- no line getter takes a cookie.  See GOAL.md.
//
// do_cmdline() handed its line getter a void * cookie that every call passed
// as nullptr and no getter read.  It goes, with find_func_t, a typedef nothing
// names: what is left of void * in the core is memory.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim160", Edit) }

// Whim160 takes the cookie Out of the line getters.
//
// do_cmdline() takes a function that gets the next line and a void * it hands
// that function, which vim's script and user-function readers used.  Here
// every call passes nullptr, do_one_cmd() and :append's reader only pass it
// on, and neither getter left -- getexline(), getcmdkeycmd() -- reads it: a
// parameter that is always nullptr and read by nothing.  It goes from the
// getter's type, from the functions that pass it and from exarg_T (whose
// member, and find_func_t, a typedef nothing names, the sweep takes).  What is left of void * in the
// core is the functions of bytes, the allocators and a growarray's storage
// (internal/ccx's VoidPtrs).
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("cookie", text, w)
	n := e.Mentions("cookie")
	e.Expect(n == 19, "cookie is named %d times; this phase was written against 19", n)
	e.Literal("(int, void *, int, getline_opt_T)", "(int, int, getline_opt_T)", 9, "a line getter takes no cookie: its type")
	e.Cut(`,\s*void\s+\*cookie\b`, 9,
		"9 declarations of do_cmdline(), getline_equal(), do_one_cmd(), getexline() and getcmdkeycmd() take no cookie")
	e.Sub(`(do_cmdline\([^;]*?, (?:nullptr|getexline|getcmdkeycmd)), nullptr, `, "${1}, ", 6, "6 calls of do_cmdline() pass none")
	e.Literal("eap->ea_getline(NUL, eap->cookie, indent, ", "eap->ea_getline(NUL, indent, ", 1,
		":append's reader passes none")
	e.Literal("getline_equal(fgetline, cookie, getexline)", "getline_equal(fgetline, getexline)", 4,
		"getline_equal() is asked without one")
	e.Literal("fgetline(':', cookie, 0, ", "fgetline(':', 0, ", 1,
		"do_cmdline() gets its next line without one")
	e.Literal("do_one_cmd(&cmdline_copy, flags, fgetline, cookie);", "do_one_cmd(&cmdline_copy, flags, fgetline);", 1,
		"and runs a command without one")
	e.Literal("    ea.cookie = cookie;\n", "", 1,
		"do_one_cmd() keeps none")
	// exarg_T's own member is left, named by nothing now: the sweep takes it.
	n = e.Mentions("cookie")
	e.Expect(n == 1, "cookie is still named %d times, beside exarg_T's member", n-1)
	return e.Done()
}
