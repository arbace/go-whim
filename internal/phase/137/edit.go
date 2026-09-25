package p137

// Whim phase 137 -- the changedtick is a number.  See GOAL.md.
//
// b:changedtick was a dictionary item inside buf_T, read as
// ((buf)->b_ct_di.di_tv.vval); with no buffer variables left it is a number
// in a typval in a struct, and the one field keeping typval_T alive in buf_T.
// The field becomes a varnumber_T and its 18 uses name it.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim137", Edit) }

// Whim137 makes the changedtick a number.
//
// vim kept b:changedtick as a dictionary item inside buf_T, so the buffer's
// own variables could hold it without a copy: CHANGEDTICK(buf) read the number
// Out of a dictitem16_T's typval.  Whim has no buffer variables since the eval
// layer went, so the dictionary item is a number in a typval in a struct, and
// it is the one field that keeps typval_T -- and through it the list, dict,
// type and class structures -- alive in buf_T.  The field becomes the number;
// init_changedtick() stops setting a type, a lock and flags nothing reads.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("tick", text, w)
	e.Literal("    dictitem16_T b_ct_di;\n", "    varnumber_T b_changedtick;\n", 1, "buf_T's changedtick is a varnumber_T")
	e.Body("init_changedtick", "    buf->b_changedtick = 0;\n", "init_changedtick() sets it to 0, and no type, lock or flags")
	// `((X)->b_ct_di.di_tv.vval)`, the expansion of CHANGEDTICK(X), becomes
	// `X->b_changedtick`.
	e.Sub(`\(\((\w+)\)->b_ct_di\.di_tv\.vval\)`, "${1}->b_changedtick", 18, "its 18 reads and writes name the field")
	return e.Done()
}
