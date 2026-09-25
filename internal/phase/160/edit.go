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
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim160", Edit) }

var (
	w160Param = regexp.MustCompile(`,\s*void\s+\*cookie\b`)
	w160Call  = regexp.MustCompile(`(do_cmdline\([^;]*?, (?:nullptr|getexline|getcmdkeycmd)), nullptr, `)
)

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
	p := edit.Ph{Tag: "cookie", W: w}
	var err error
	if n := regexp.MustCompile(`\bcookie\b`).FindAll(text, -1); len(n) != 19 {
		return nil, p.Die("cookie is named %d times; this phase was written against 19", len(n))
	}
	if text, err = p.Literal(text, "(int, void *, int, getline_opt_T)", "(int, int, getline_opt_T)", "a line getter takes no cookie: its type", 9); err != nil {
		return nil, err
	}
	n := len(w160Param.FindAll(text, -1))
	if n != 9 {
		return nil, p.Die("a cookie parameter is declared %d times; this phase was written against 9", n)
	}
	text = w160Param.ReplaceAll(text, nil)
	p.Say(fmt.Sprintf("%d declarations of do_cmdline(), getline_equal(), do_one_cmd(), getexline() and getcmdkeycmd() take no cookie", n))
	n = len(w160Call.FindAll(text, -1))
	if n != 6 {
		return nil, p.Die("do_cmdline() is called %d times with a nullptr cookie; this phase was written against 6", n)
	}
	text = w160Call.ReplaceAll(text, []byte("${1}, "))
	p.Say(fmt.Sprintf("%d calls of do_cmdline() pass none", n))
	steps := []struct {
		Old, New, What string
		n              int
	}{
		{"eap->ea_getline(NUL, eap->cookie, indent, ", "eap->ea_getline(NUL, indent, ", ":append's reader passes none", 1},
		{"getline_equal(fgetline, cookie, getexline)", "getline_equal(fgetline, getexline)", "getline_equal() is asked without one", 4},
		{"fgetline(':', cookie, 0, ", "fgetline(':', 0, ", "do_cmdline() gets its next line without one", 1},
		{"do_one_cmd(&cmdline_copy, flags, fgetline, cookie);", "do_one_cmd(&cmdline_copy, flags,  fgetline );", "and runs a command without one", 1},
		{"    ea.cookie = cookie;\n", "", "do_one_cmd() keeps none", 1},
	}
	for _, st := range steps {
		if text, err = p.Literal(text, st.Old, st.New, st.What, st.n); err != nil {
			return nil, err
		}
	}
	// exarg_T's own member is left, named by nothing now: the sweep takes it.
	if n := len(regexp.MustCompile(`\bcookie\b`).FindAll(text, -1)); n != 1 {
		return nil, p.Die("cookie is still named %d times, beside exarg_T's member", n-1)
	}
	return text, nil
}
