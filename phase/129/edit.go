package p129

// Whim phase 129 -- p_emoji is an int.  See GOAL.md.
//
// The first phase that comes of transpiling editor.c to Go (tx/FINDINGS.md, 1).
// 'emoji' is a boolean option, and the options table writes and reads every
// boolean option through an `int *` -- set_option_default() stores
// `*(int *)varp`, do_set_option_bool() and set_bool_option() likewise -- but
// p_emoji was declared `char_u *`.  The C got away with it: the static starts
// zeroed and the int lands in the pointer's low bytes, so utf_char2cells()'s
// `if (p_emoji && ...)` tests the right thing.  The Go transpilation cannot say
// that, and panicked on its first run.  This phase declares the variable what
// every writer and the reader already take it to be.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c: the check's probe runs both.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim129", Edit) }

// Whim129 declares p_emoji an int.
//
// 'emoji' is a P_BOOL option and the options table writes and reads every
// boolean option through an int *, but its variable was declared char_u *:
// set_option_default() stores `*(int *)varp` into the pointer's storage and
// utf_char2cells() tests the pointer for non-NULL.  It works on this target
// because the static starts zeroed and the int lands in the low bytes, and it
// is exactly the kind of thing a transpilation cannot say -- the Go of it
// panicked on the first run (tx/FINDINGS.md, 1).  The table row, the reader
// and every writer are unchanged; only the declaration says what the storage
// holds.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "emoji", W: w}
	return p.Literal(text, "static char_u *p_emoji;\n", "static int      p_emoji;\n",
		"p_emoji is declared int, the type every boolean option's variable has", 1)
}
