package p059

// Whim phase 59 (formerly 131) -- the saved input buffer is a garray_T *.  See GOAL.md.
//
// get_input_buf() makes a garray_T, casts it to char_u * for
// tasave_T.save_inputbuf, and set_input_buf() casts it back; nothing reads it as
// characters.  The field, the prototypes, the definition and the return say
// what it is (internal/gen/FINDINGS.md, 6).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim59", Edit) }

// Edit gives the saved input buffer its type.
//
// save_typeahead() keeps what inbuf[] held in tasave_T.save_inputbuf, and
// get_input_buf() makes it: a garray_T, allocated and filled -- then cast to
// char_u * to be stored, and cast back by set_input_buf().  Nothing reads it
// as characters.  The Go transpilation could not carry a growarray in a
// string pointer and had to register it under a one-byte key
// (internal/gen/FINDINGS.md, 6).  The field, both prototypes, the definition and the
// return say garray_T *, and set_input_buf() takes the garray_T it always
// cast its argument to.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the member, the result and the
// parameter retyped (RETYPE); set_input_buf()'s local gap gives its uses to
// the parameter, which takes its name in the definition (RENAME) and in the
// prototype; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("inputbuf", e, w)
	v.Retype("(save_inputbuf (ptr char_u))", "(ptr garray_T)", "tasave_T.save_inputbuf is a garray_T *")
	v.RetypeResult("get_input_buf", "(ptr garray_T)", "get_input_buf() is declared and defined to return one")
	v.InFunction("get_input_buf", func(v *graph.Verbs) {
		v.Rewrite("(return (cast (ptr char_u) gap))", "(return gap)", 1, "and returns it without the cast")
	})
	v.InFunction("set_input_buf", func(v *graph.Verbs) {
		p := v.One("(p (ptr char_u))", "set_input_buf()'s parameter")
		gap := v.One("(def gap (ptr garray_T) (cast (ptr garray_T) p))", "set_input_buf()'s cast of it")
		if v.Failed() {
			return
		}
		if _, err := e.Retype(p, "(ptr garray_T)"); err != nil {
			v.Die("set_input_buf() is declared to take one -- %v", err)
			return
		}
		v.Say("set_input_buf() is declared to take one")
		if _, err := e.RetargetUses(gap, p); err != nil {
			v.Die("set_input_buf()'s gap -- %v", err)
			return
		}
		v.Cut("(def gap (ptr garray_T) (cast (ptr garray_T) p))", 1, "set_input_buf() casts nothing")
		v.Rename("p", "gap", "set_input_buf() takes the garray_T it used to cast its argument to")
	})
	if !v.Failed() {
		if _, err := e.RenamePrototypeParams("set_input_buf", 0, "gap"); err != nil {
			v.Die("set_input_buf()'s prototype -- %v", err)
		}
	}
	return v.Done()
}
