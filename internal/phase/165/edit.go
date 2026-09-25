package p165

// Whim phase 165 -- no store nothing reads.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim165", Edit) }

// Edit removes six stores to locals that nothing reads before they are
// written again or go out of scope.  The Go transpilation showed them
// (staticcheck SA4006, SA4009); they are the C's as much as the Go's.  Each is
// named and found exactly once in its own function, because finding such a
// store in general is dataflow through gotos, and there are six.  A store
// whose right side calls something keeps the call: only the store goes.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("deadstore", text, w)

	// the line's text, fetched after an indent computed from the same line,
	// and never looked at before ptr is fetched again
	e.InFunction("open_line", func(e *edit.E) {
		e.Sub(`(?m)^([ \t]*)newindent = get_indent\(\);\n[ \t]*ptr = ml_get_curline\(\);\n`,
			"${1}newindent = get_indent();\n", 1, "open_line's ptr, fetched and never read")
	})
	// showmode() draws the mode; what it returns was kept and never read
	e.InFunction("edit", func(e *edit.E) {
		e.Sub(`(?m)^([ \t]*)i = showmode\(\);\n`, "${1}showmode();\n", 1,
			"edit()'s i = showmode(): the call stays, the store goes")
	})
	// the parameter is the key read here, never the caller's: a local
	e.Literal("cmdline_handle_ctrl_bsl(int c, int *gotesc)\n{\n",
		"cmdline_handle_ctrl_bsl(int *gotesc)\n{\n    int c;\n", 1,
		"cmdline_handle_ctrl_bsl's c, a parameter overwritten before any read, is a local")
	e.Literal("cmdline_handle_ctrl_bsl(c, &gotesc)", "cmdline_handle_ctrl_bsl(&gotesc)", 1,
		"and its one caller stops passing it")
	// one past a match at the end of the line, just before the loop is left
	e.InFunction("next_search_hl", func(e *edit.E) {
		e.Sub(`(?m)^[ \t]*\+\+matchcol;\n([ \t]*shl->lnum = 0;\n[ \t]*break;\n)`, "${1}", 1,
			"next_search_hl's ++matchcol before the loop is left")
	})
	// the column within the row, which nothing after asks for
	e.InFunction("adjust_skipcol", func(e *edit.E) {
		e.Lines(`col = col % width2;`, 1, "adjust_skipcol's col % width2")
	})
	// the line after the put, stepped back once the loop is done with it
	e.InFunction("do_put", func(e *edit.E) {
		e.Sub(`(?m)^([ \t]*while \(VIsual_active && lnum <= end_lnum\);\n)[ \t]*if \(VIsual_active\)\n[ \t]*\{\n[ \t]*lnum--;\n[ \t]*\}\n`,
			"${1}", 1, "do_put's lnum-- after the last use of lnum")
	})
	return e.Done()
}
