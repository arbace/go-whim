package p080

// Whim phase 80 (formerly 157) -- get_register() and put_register() carry a yankreg_T *, not a void *.  See GOAL.md.
//
// get_register() returns a register as void * and put_register() casts it
// back: the register is typed yankreg_T * throughout, and the core's last
// pointer cast outside the classes internal/ccx names goes.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim80", Edit) }

// Edit types the register get_register() hands to put_register().
//
// get_register() returns a copy of a yank register, a yankreg_T, as void *,
// and put_register() takes it back as void * and casts it to yankreg_T *: a
// register carried through void *, the one cast internal/ccx's Casts finds
// that is neither an allocation, a growarray nor a function of bytes.  Both
// functions and the two locals between them (nv_edit's reg1 and reg2) now
// say yankreg_T *.  No code changes: the pointer is the same pointer.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the result, the parameter and
// the two locals retyped (RETYPE), the two casts rewritten by form; history
// keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("register", e, w)
	v.RetypeResult("get_register", "(ptr yankreg_T)", "get_register() returns a yankreg_T *: its prototype and its definition")
	v.InFunction("get_register", func(v *graph.Verbs) {
		v.Rewrite("(return (cast (ptr void) ?r))", "(return ?r)", 1, "which returns the register without a cast")
	})
	v.InFunction("put_register", func(v *graph.Verbs) {
		v.Retype("(reg (ptr void))", "(ptr yankreg_T)", "put_register() takes one: its prototype and its definition")
		v.Rewrite("(deref (cast (ptr yankreg_T) ?r))", "(deref ?r)", 1, "which copies it without a cast")
	})
	v.Retype("(def reg1 (ptr void) nullptr)", "(ptr yankreg_T)", "and the two locals between them hold yankreg_T *")
	v.Retype("(def reg2 (ptr void) nullptr)", "(ptr yankreg_T)", "both")
	return v.Done()
}
