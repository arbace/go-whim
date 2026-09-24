package p162

// Whim phase 162 -- no two function pointers are compared.  See GOAL.md.
//
// do_cmdline() compared its line getter with getexline, the one comparison of
// two function pointers in the core.  Its callers say so with DOCMD_GETEXLINE.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim162", Edit) }

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
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "getexline", W: w}
	var err error
	steps := []struct {
		Old, New, What string
		n              int
	}{
		{"enum { DOCMD_KEEPLINE = 0x20 };\n", "enum { DOCMD_KEEPLINE = 0x20 };\n\nenum { DOCMD_GETEXLINE = 0x40 };\n", "a flag says the lines come from getexline()", 1},
		{"do_cmdline(nullptr, getexline, DOCMD_NOWAIT | DOCMD_VERBOSE);", "do_cmdline(nullptr, getexline, DOCMD_NOWAIT | DOCMD_VERBOSE | DOCMD_GETEXLINE);", "ex_at() sets it", 1},
		{"do_cmdline(nullptr, getexline, flags);", "do_cmdline(nullptr, getexline, flags | DOCMD_GETEXLINE);", "nv_colon() sets it", 1},
		{"getline_equal(fgetline, getexline)", "(flags & DOCMD_GETEXLINE)", "do_cmdline() tests it where it compared its getter with getexline", 4},
	}
	for _, st := range steps {
		if text, err = p.Literal(text, st.Old, st.New, st.What, st.n); err != nil {
			return nil, err
		}
	}
	return text, nil
}
