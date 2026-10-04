package p057

// Whim phase 57 (formerly 129) -- p_emoji is an int.  See GOAL.md.
//
// The first phase that comes of transpiling editor.c to Go (internal/gen/FINDINGS.md, 1).
// 'emoji' is a boolean option, and the options table writes and reads every
// boolean option through an `int *` -- set_option_default() stores
// `*(int *)varp`, do_set_option_bool() and set_bool_option() likewise -- but
// p_emoji was declared `char_u *`.  The C got away with it: the static starts
// zeroed and the int lands in the pointer's low bytes, so utf_char2cells()'s
// `if (p_emoji && ...)` tests the right thing.  The Go transpilation cannot say
// that, and panicked on its first run.  This phase declares the variable what
// every writer and the reader already take it to be.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c: the check's probe runs both.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim57", Edit) }

// Edit declares p_emoji an int.
//
// 'emoji' is a P_BOOL option and the options table writes and reads every
// boolean option through an int *, but its variable was declared char_u *:
// set_option_default() stores `*(int *)varp` into the pointer's storage and
// utf_char2cells() tests the pointer for non-NULL.  It works on this target
// because the static starts zeroed and the int lands in the low bytes, and it
// is exactly the kind of thing a transpilation cannot say -- the Go of it
// panicked on the first run (internal/gen/FINDINGS.md, 1).  The table row, the reader
// and every writer are unchanged; only the declaration says what the storage
// holds.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the declaration retyped
// (RETYPE), its uses typed again; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("emoji", e, w)
	v.Retype("(def static p_emoji (ptr char_u))", "int",
		"p_emoji is declared int, the type every boolean option's variable has")
	return v.Done()
}
