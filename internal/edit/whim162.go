package edit

import (
	"io"
)

func init() { register("whim162", Whim162) }

// Whim162 has do_cmdline() read a flag where it compared function pointers.
//
// do_cmdline() asks getline_equal(fgetline, getexline) four times: whether
// its lines come from the command line typed at ':'.  That is the one
// comparison of two function pointers in the core, and Go's func values
// compare only with nil.  Its two callers that pass getexline, nv_colon() and
// ex_at(), say so with a new flag, DOCMD_GETEXLINE, and do_cmdline() tests
// the flag.  do_one_cmd(), which is handed the flags, reads only
// DOCMD_VERBOSE of them; getline_equal() is left with no caller and the sweep
// takes it.
func Whim162(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "getexline", w: w}
	var err error
	steps := []struct {
		old, new, what string
		n              int
	}{
		{"enum { DOCMD_KEEPLINE = 0x20 };\n", "enum { DOCMD_KEEPLINE = 0x20 };\nenum { DOCMD_GETEXLINE = 0x40 };\n", "a flag says the lines come from getexline()", 1},
		{"do_cmdline(nullptr, getexline, DOCMD_NOWAIT|DOCMD_VERBOSE);", "do_cmdline(nullptr, getexline, DOCMD_NOWAIT|DOCMD_VERBOSE|DOCMD_GETEXLINE);", "ex_at() sets it", 1},
		{"do_cmdline(nullptr, getexline, flags);", "do_cmdline(nullptr, getexline, flags | DOCMD_GETEXLINE);", "nv_colon() sets it", 1},
		{"getline_equal(fgetline, getexline)", "(flags & DOCMD_GETEXLINE)", "do_cmdline() tests it where it compared its getter with getexline", 4},
	}
	for _, st := range steps {
		if text, err = p.literal(text, st.old, st.new, st.what, st.n); err != nil {
			return nil, err
		}
	}
	return text, nil
}
