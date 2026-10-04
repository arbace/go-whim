package p003d

// Whim phase 3d (formerly 65) -- no rot13, no operator function, no empty key handler.
// See GOAL.md.
//
// Three cuts, and only the first changes what the editor can do:
//
// ROT13  g?, the one operator here that encodes rather than edits.  It goes
// whole: nv_g_cmd()'s case, the OP_ROT13 dispatch label, nv_search()'s
// redirect -- which is how `g?` reaches the operator when a search is
// pending -- and swapchar()'s three arms, after which swapchar() is the
// case-changing function it always really was.
// THE OPERATOR FUNCTION  g@, which has had nothing to call since the eval
// feature went: op_function() is one emsg(), and 'operatorfunc' does not
// exist to name a function anyway.  The dispatch, the OP_FUNCTION term in
// the motion_force test, and op_function() itself all go.
// AN EMPTY CALL  ins_ctrl_x() has an empty body -- CTRL-X in Insert mode began
// a completion, and completion went in phase 4.  The key stays inert, but
// it no longer calls a function to do nothing.
//
// WHAT IS DELIBERATELY KEPT, because "does nothing" and "should be deleted" are
// different claims:
//
// CTRL-P and CTRL-N in Insert mode are `break;` -- they do nothing on purpose.
// Deleting the labels would drop them into `normalchar`, which INSERTS the
// control character, so removing dead-looking code would add behaviour.
// zy, zp and zP are live.  It looks as though `zy` must reach
// internal_error("get_op_type()"), since opchars[] has no {'z','y'} row --
// but get_op_type() special-cases 'z'+'y' to OP_YANK before it consults the
// table.  Measured on the binary of q64 of the old numbering: no error, no message.
// The opchars[] rows for g? and g@ stay.  The table is positional -- a row's
// index IS its OP_* value -- so removing one renumbers every operator after
// it.  Nothing reaches them once nv_g_cmd() has no case.
//
// THE DELTA: no Ex command, and no behaviour case -- the harness never rot13s.
// The probes check g?g? no longer encodes, that g?? and ?-with-operator-pending
// do not either, that g@g@ is refused, and that gu/gU/g~ still work, since they
// share swapchar() with the arms that go.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the labels and statements as
// items found by form, the operator function's case as a run, each scoped to
// its function (history keeps the text version).

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// cutRunAfter deletes the items of the run pats says but its first, the run
// found once in the scope.
func cutRunAfter(v *graph.Verbs, what string, pats ...string) {
	r := v.Run(what, pats...)
	if r == nil {
		return
	}
	for _, x := range r[1:] {
		if err := v.Editor().Delete(x); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
	}
	v.Say(what)
}

// Edit takes g? and g@ -- rot13 and the operator function -- and an empty
// call left behind by completion.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("norot13", e, w)

	// The case labels are scoped to nv_g_cmd: another switch entirely has '?'
	// and '@' next to each other, and an unscoped edit would have two places to
	// choose from.
	v.InFunction("nv_g_cmd", func(v *graph.Verbs) {
		cutRunAfter(v, "g? and g@ as operators", "(case 'U')", "(case '?')", "(case '@')")
	})
	v.InFunction("nv_search", func(v *graph.Verbs) {
		v.DropIf("(&& (== (-> cap cmdchar) '?') (== (-> cap oap op_type) OP_ROT13))", 1,
			"g? reaching the operator with a search pending")
	})
	v.InFunction("do_pending_operator", func(v *graph.Verbs) {
		v.Cut("(case OP_ROT13)", 1, "rot13 sharing the case-change dispatch")
	})
	v.InFunction("swapchar", func(v *graph.Verbs) {
		v.DropIf("(&& (>= c 0x80) (== op_type OP_ROT13))", 1, "rot13 refusing a multibyte character")
		v.FoldNever("(== op_type OP_ROT13)", 2, "rot13 rotating a letter")
	})

	// The operator function.
	v.InFunction("do_pending_operator", func(v *graph.Verbs) {
		v.CutRun("g@ reaching the operator function", "(case OP_FUNCTION)",
			"(block (def save_redo_VIsual redo_VIsual_T redo_VIsual) (call op_function oap) (= redo_VIsual save_redo_VIsual) (break))")
		v.DropOperand("(== (-> oap op_type) OP_FUNCTION)", 1,
			"the operator function deciding whether the motion is inclusive")
	})

	// An empty call.
	v.InFunction("edit", func(v *graph.Verbs) {
		cutRunAfter(v, "CTRL-X calling an empty function", "(case Ctrl_X)", "(call ins_ctrl_x)")
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim3d", Edit) }
