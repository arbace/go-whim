package p070

// Whim phase 70 (formerly 142) -- the version names no build date or time.  See GOAL.md.
//
// init_longVersion() put __DATE__ " " __TIME__ into the version line a
// command-line error is headed with, so the binary depended on when it was
// built (internal/gen/FINDINGS.md, 13).  The version is its name and release date.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim70", Edit) }

// Edit takes the build's date and time out of the version.
//
// init_longVersion() put __DATE__ " " __TIME__ into the version line that
// mainerr() prints above a command-line error: "VIM - Vi IMproved 9.2 (2026
// Feb 14, compiled <date> <time>)".  So the core's text was a function of when
// it was compiled, and two builds of the same source differed unless
// SOURCE_DATE_EPOCH pinned them; the Go transpilation had no such clock and
// wrote in the constant the pinned build produces (internal/gen/FINDINGS.md, 13).  The
// version is now its name and its release date, "(2026 Feb 14)", and the
// binary is a function of the source alone.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the format string respelled
// (RENAME's string rule), the length's term and the argument taken by form;
// the collection takes date_time; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("datetime", e, w)
	v.InFunction("init_longVersion", func(v *graph.Verbs) {
		v.RespellString(`"%s (%s, compiled %s)"`, `"%s (%s)"`, 1, "it is the name and the release date")
		v.Rewrite("(+ ?a (call musl_strlen date_time))", "?a", 1, "and its length counts no date")
		v.Rewrite("(call vim_snprintf longVersion len msg VIM_VERSION_LONG_ONLY VIM_VERSION_DATE_ONLY date_time)",
			"(call vim_snprintf longVersion len msg VIM_VERSION_LONG_ONLY VIM_VERSION_DATE_ONLY)", 1, "nor formats one")
	})
	return v.Done()
}
