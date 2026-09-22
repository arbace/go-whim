package edit

import (
	"io"
)

func init() { register("whim129", Whim129) }

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
func Whim129(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "emoji", w: w}
	return p.literal(text, "static char_u   *p_emoji;\n", "static int      p_emoji;\n",
		"p_emoji is declared int, the type every boolean option's variable has", 1)
}
