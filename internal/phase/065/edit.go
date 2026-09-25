package p065

// Whim phase 65 -- no rot13, no operator function, no empty key handler.
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
// a completion, and completion went in phase 6.  The key stays inert, but
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
// table.  Measured on the q64 binary: no error, no message.
// The opchars[] rows for g? and g@ stay.  The table is positional -- a row's
// index IS its OP_* value -- so removing one renumbers every operator after
// it.  Nothing reaches them once nv_g_cmd() has no case.
//
// THE DELTA: no Ex command, and no behaviour case -- the harness never rot13s.
// The probes check g?g? no longer encodes, that g?? and ?-with-operator-pending
// do not either, that g@g@ is refused, and that gu/gU/g~ still work, since they
// share swapchar() with the arms that go.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// opFunctionCase is g@'s arm in do_pending_operator.  It is matched as a
// literal because it is a whole block with its own braces and a saved
// redo_VIsual, and a pattern for it would be longer than the block.
const opFunctionCase = "        case OP_FUNCTION:\n" +
	"            {\n" +
	"                redo_VIsual_T save_redo_VIsual = redo_VIsual;\n" +
	"                op_function(oap);\n" +
	"                redo_VIsual = save_redo_VIsual;\n" +
	"                break;\n" +
	"            }\n"

// Whim65 takes g? and g@ -- rot13 and the operator function -- and an empty
// call left behind by completion.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("norot13", text, w)

	// The case labels are scoped to nv_g_cmd: another switch entirely has '?'
	// and '@' next to each other, and an unscoped edit would have two places to
	// choose from.
	e.InFunction("nv_g_cmd", func(e *edit.E) {
		e.Sub(`(?m)^([ \t]*case 'U':\n)[ \t]*case '\?':\n[ \t]*case '@':\n`, "${1}", 1, "g? and g@ as operators")
	})
	e.InFunction("nv_search", func(e *edit.E) {
		e.DropIf(edit.Head("if (cap->cmdchar == '?' && cap->oap->op_type == OP_ROT13)"), 1,
			"g? reaching the operator with a search pending")
	})
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.Cut(edit.Line("case OP_ROT13:"), 1, "rot13 sharing the case-change dispatch")
	})
	e.InFunction("swapchar", func(e *edit.E) {
		e.DropIf(edit.Head("if (c >= 0x80 && op_type == OP_ROT13)"), 1, "rot13 refusing a multibyte character")
		e.FoldNever(edit.Head("if (op_type == OP_ROT13)"), 2, "rot13 rotating a letter")
	})

	// The operator function.
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.Literal(opFunctionCase, "", 1, "g@ reaching the operator function")
	})
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.Literal(" || oap->op_type == OP_FUNCTION", "", 1, "the operator function deciding whether the motion is inclusive")
	})

	// An empty call.
	e.InFunction("edit", func(e *edit.E) {
		e.Sub(`(?m)^([ \t]*case Ctrl_X:\n)[ \t]*ins_ctrl_x\(\);\n`, "${1}", 1, "CTRL-X calling an empty function")
	})
	return e.Done()
}

func init() { edit.Register("whim65", Edit) }
