package p065

// Whim phase 65 (formerly 137) -- the changedtick is a number.  See GOAL.md.
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

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim65", Edit) }

// Edit makes the changedtick a number.
//
// vim kept b:changedtick as a dictionary item inside buf_T, so the buffer's
// own variables could hold it without a copy: CHANGEDTICK(buf) read the number
// out of a dictitem16_T's typval.  Whim has no buffer variables since the eval
// layer went, so the dictionary item is a number in a typval in a struct, and
// it is the one field that keeps typval_T -- and through it the list, dict,
// type and class structures -- alive in buf_T.  The field becomes the number;
// init_changedtick() stops setting a type, a lock and flags nothing reads.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the member retyped and renamed
// (RETYPE, RENAME), the body by FRAG, and CHANGEDTICK(X)'s 18 expansions
// rewritten by form; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("tick", e, w)
	ct := v.One("(b_ct_di dictitem16_T)", "buf_T's changedtick")
	if v.Failed() {
		return v.Done()
	}
	if _, err := e.Retype(ct, "varnumber_T"); err != nil {
		v.Die("buf_T's changedtick is a varnumber_T -- %v", err)
		return v.Done()
	}
	if _, err := e.Rename(ct, "b_changedtick"); err != nil {
		v.Die("buf_T's changedtick is a varnumber_T -- %v", err)
		return v.Done()
	}
	v.Say("buf_T's changedtick is a varnumber_T")
	v.BodyC("init_changedtick", "    buf->b_changedtick = 0;\n", "init_changedtick() sets it to 0, and no type, lock or flags")
	// `((X)->b_ct_di.di_tv.vval)`, the expansion of CHANGEDTICK(X), becomes
	// `X->b_changedtick`.
	v.Rewrite("(paren (. (-> (paren ?x) b_changedtick) di_tv vval))", "(-> ?x b_changedtick)", 18,
		"its 18 reads and writes name the field")
	return v.Done()
}
