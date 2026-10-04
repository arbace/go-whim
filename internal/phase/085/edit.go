package p085

// Whim phase 85 (formerly 162) -- no two function pointers are compared.  See GOAL.md.
//
// do_cmdline() compared its line getter with getexline, the one comparison of
// two function pointers in the core.  Its callers say so with DOCMD_GETEXLINE.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim85", Edit) }

// Edit has do_cmdline() read a flag where it compared function pointers.
//
// do_cmdline() asks getline_equal(fgetline, getexline) four times: whether
// its lines come from the command line typed at ':'.  That is the one
// comparison of two function pointers in the core, and Go's func values
// compare only with nil.  Its two callers that pass getexline, nv_colon() and
// ex_at(), say so with a new flag, DOCMD_GETEXLINE, and do_cmdline() tests
// the flag.  do_one_cmd(), which is handed the flags, reads only
// DOCMD_VERBOSE of them; getline_equal() is left with no caller and the sweep
// takes it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the flag's enum, the two
// callers' flags and the four tests written by FRAG in one unit (Together),
// each place found by its form; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("getexline", e, w)
	keep := v.One("(enum (DOCMD_KEEPLINE 0x20))", "DOCMD_KEEPLINE's enum")
	if v.Failed() {
		return v.Done()
	}
	v.Together(func(v *graph.Verbs) {
		v.FragAt(e.SpotAfter(keep), "enum { DOCMD_GETEXLINE = 0x40 };\n", "a flag says the lines come from getexline()")
		v.LiteralExprC("(call do_cmdline nullptr getexline _)", "do_cmdline(nullptr, getexline, DOCMD_NOWAIT | DOCMD_VERBOSE)",
			"do_cmdline(nullptr, getexline, DOCMD_NOWAIT | DOCMD_VERBOSE | DOCMD_GETEXLINE)", 1, "ex_at() sets it")
		v.LiteralExprC("(call do_cmdline nullptr getexline _)", "do_cmdline(nullptr, getexline, flags)",
			"do_cmdline(nullptr, getexline, flags | DOCMD_GETEXLINE)", 1, "nv_colon() sets it")
		v.ReplaceC("(call getline_equal fgetline getexline)", "(flags & DOCMD_GETEXLINE)", 4,
			"do_cmdline() tests it where it compared its getter with getexline")
	})
	return v.Done()
}
