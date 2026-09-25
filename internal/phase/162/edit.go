package p162

// Whim phase 162 -- no two function pointers are compared.  See GOAL.md.
//
// do_cmdline() compared its line getter with getexline, the one comparison of
// two function pointers in the core.  Its callers say so with DOCMD_GETEXLINE.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim162", Edit) }

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
	e := edit.New("getexline", text, w)
	e.Literal("enum { DOCMD_KEEPLINE = 0x20 };\n", "enum { DOCMD_KEEPLINE = 0x20 };\n\nenum { DOCMD_GETEXLINE = 0x40 };\n", 1,
		"a flag says the lines come from getexline()")
	e.Literal("do_cmdline(nullptr, getexline, DOCMD_NOWAIT | DOCMD_VERBOSE);", "do_cmdline(nullptr, getexline, DOCMD_NOWAIT | DOCMD_VERBOSE | DOCMD_GETEXLINE);", 1,
		"ex_at() sets it")
	e.Literal("do_cmdline(nullptr, getexline, flags);", "do_cmdline(nullptr, getexline, flags | DOCMD_GETEXLINE);", 1,
		"nv_colon() sets it")
	e.Literal("getline_equal(fgetline, getexline)", "(flags & DOCMD_GETEXLINE)", 4,
		"do_cmdline() tests it where it compared its getter with getexline")
	return e.Done()
}
