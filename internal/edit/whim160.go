package edit

import (
	"fmt"
	"io"
	"regexp"
)

func init() { register("whim160", Whim160) }

var (
	w160Param = regexp.MustCompile(`,\s*void\s+\*cookie\b`)
	w160Call  = regexp.MustCompile(`(do_cmdline\([^;]*?, (?:nullptr|getexline|getcmdkeycmd)), nullptr, `)
)

// Whim160 takes the cookie out of the line getters.
//
// do_cmdline() takes a function that gets the next line and a void * it hands
// that function, which vim's script and user-function readers used.  Here
// every call passes nullptr, do_one_cmd() and :append's reader only pass it
// on, and neither getter left -- getexline(), getcmdkeycmd() -- reads it: a
// parameter that is always nullptr and read by nothing.  It goes from the
// getter's type, from the functions that pass it and from exarg_T.  With it
// goes find_func_t, a typedef nothing names.  What is left of void * in the
// core is the functions of bytes, the allocators and a growarray's storage
// (internal/ccx's VoidPtrs).
func Whim160(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "cookie", w: w}
	var err error
	if n := regexp.MustCompile(`\bcookie\b`).FindAll(text, -1); len(n) != 19 {
		return nil, p.die("cookie is named %d times; this phase was written against 19", len(n))
	}
	if text, err = p.literal(text, "typedef long (*find_func_t)(const char *line, long line_len, char *buffer, long buffer_size, void *priv);\n", "", "find_func_t, a typedef nothing names, goes", 1); err != nil {
		return nil, err
	}
	if text, err = p.literal(text, "(int, void *, int, getline_opt_T)", "(int, int, getline_opt_T)", "a line getter takes no cookie: its type", 9); err != nil {
		return nil, err
	}
	n := len(w160Param.FindAll(text, -1))
	if n != 9 {
		return nil, p.die("a cookie parameter is declared %d times; this phase was written against 9", n)
	}
	text = w160Param.ReplaceAll(text, nil)
	p.say(fmt.Sprintf("%d declarations of do_cmdline(), getline_equal(), do_one_cmd(), getexline() and getcmdkeycmd() take no cookie", n))
	n = len(w160Call.FindAll(text, -1))
	if n != 6 {
		return nil, p.die("do_cmdline() is called %d times with a nullptr cookie; this phase was written against 6", n)
	}
	text = w160Call.ReplaceAll(text, []byte("${1}, "))
	p.say(fmt.Sprintf("%d calls of do_cmdline() pass none", n))
	steps := []struct {
		old, new, what string
		n              int
	}{
		{"    void        *cookie;\n", "", "exarg_T holds none", 1},
		{"eap->ea_getline(NUL, eap->cookie, indent, ", "eap->ea_getline(NUL, indent, ", ":append's reader passes none", 1},
		{"getline_equal(fgetline, cookie, getexline)", "getline_equal(fgetline, getexline)", "getline_equal() is asked without one", 4},
		{"fgetline(':', cookie, 0, ", "fgetline(':', 0, ", "do_cmdline() gets its next line without one", 1},
		{"do_one_cmd(&cmdline_copy, flags,  fgetline ,  cookie );", "do_one_cmd(&cmdline_copy, flags,  fgetline );", "and runs a command without one", 1},
		{"    ea.cookie = cookie;\n", "", "do_one_cmd() keeps none", 1},
	}
	for _, st := range steps {
		if text, err = p.literal(text, st.old, st.new, st.what, st.n); err != nil {
			return nil, err
		}
	}
	if m := regexp.MustCompile(`\bcookie\b`).Find(text); m != nil {
		return nil, p.die("cookie is still named")
	}
	return text, nil
}
