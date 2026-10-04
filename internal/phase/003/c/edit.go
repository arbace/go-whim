package p003c

// Whim phase 3c (formerly 63) -- no jump list.  See GOAL.md.
//
// The per-window jump list goes: w_jumplist, w_jumplistlen and w_jumplistidx,
// setpcmark() appending to it, CTRL-O and CTRL-I walking it through movemark(),
// :jumps and :clearjumps, cleanup_jumplist(), copying it to a new window and
// freeing it with one, and the loops that kept its marks right when lines moved
// or a file was forgotten.
//
// What stays, because it is not the jump list: the previous-context mark behind
// '' and `` (w_pcmark, still set by setpcmark()), the change list and g; g,
// (nv_pcmark() keeps that half), :keepjumps (it guards the pcmark and the change
// list too), and JUMPLISTSIZE, which sizes the change list.  CTRL-O in Select mode
// still runs one Visual command; anywhere else CTRL-O and CTRL-I beep.
//
// THE DELTA: :jumps and :clearjumps, now ex_ni.  The probes check CTRL-O no longer
// jumps back, '' still does, and :jumps is refused.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the row by RewriteAt, the
// loops and runs by form, each scoped to its function (history keeps the
// text version).

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// Edit takes the jump list: :jumps and :clearjumps, CTRL-I and CTRL-O, and
// every place a line or column change moved its marks.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nojumplist", e, w)

	// :jumps and :clearjumps point at ex_ni from phase 1 (exfront, D2)
	v.InTable("nv_cmds", func(v *graph.Verbs) {
		v.RewriteAt("(init Ctrl_I ?h 0 0)", "h", "nv_error", 1, "CTRL-I in Normal mode points at nv_error")
	})

	// The append is cut from its test to the last statement of its body: the
	// body is the whole of building a jump-list entry.
	v.InFunction("setpcmark", func(v *graph.Verbs) {
		v.CutRun("setpcmark appending to the jump list",
			"(if (> (pre++ (-> curwin w_jumplistlen)) JUMPLISTSIZE) _)",
			"(= (-> curwin w_jumplistidx) (-> curwin w_jumplistlen))",
			"(= fm _)", "(= (. (-> fm fmark) mark) _)", "(= (. (-> fm fmark) fnum) _)", "(= (-> fm fname) nullptr)")
	})

	v.InFunction("nv_ctrlo", func(v *graph.Verbs) {
		v.SpliceFirst("(= (-> cap count1) (- (-> cap count1)))", "(call nv_pcmark cap)",
			"(call clearopbeep (-> cap oap))", "CTRL-O walking back through the jump list")
	})
	v.InFunction("nv_pcmark", func(v *graph.Verbs) {
		v.DropIf("(&& (== (-> cap cmdchar) TAB) (== mod_mask MOD_MASK_CTRL))", 1,
			"CTRL-Tab refused by the jump-list command")
		v.FoldAlwaysElse("(if (== (-> cap cmdchar) 'g') (block (= pos (call movechangelist _))) (block (= pos (call movemark _))))", 1,
			"the jump list as the other half of nv_pcmark")
		// the arm that chose the change-list messages by key is its block,
		// the else that beeped on a jump-list miss with it
		if arm := v.One("(if (== (-> cap cmdchar) 'g') (block (if (== (-> curbuf b_changelistlen) 0) _ _)) (block (call clearopbeep (-> cap oap))))",
			"the change-list messages no longer choosing by key"); arm != nil {
			if err := e.Replace(arm, arm.Kids[2]); err != nil {
				v.Die("the change-list messages no longer choosing by key -- %v", err)
				return
			}
			v.Say("the change-list messages no longer choosing by key")
			v.Say("a jump-list miss beeping")
		}
	})
	v.InFunction("mark_adjust_internal", func(v *graph.Verbs) {
		v.Cut("(for (= i 0) (< i (-> win w_jumplistlen)) (pre++ i) _)", 1, "line changes moving jump-list marks")
		v.Cut("(if (== (& (. cmdmod cmod_flags) CMOD_LOCKMARKS) 0) (block))", 1,
			"the now-empty 'lockmarks' test around them")
	})
	v.InFunction("mark_col_adjust", func(v *graph.Verbs) {
		v.Cut("(for (= i 0) (< i (-> win w_jumplistlen)) (pre++ i) _)", 1, "column changes moving jump-list marks")
	})
	v.InFunction("mark_forget_file", func(v *graph.Verbs) {
		v.Cut("(for (= i (- (-> wp w_jumplistlen) 1)) _ _ _)", 1, "a forgotten file leaving the jump list")
	})
	v.InFunction("fmarks_check_names", func(v *graph.Verbs) {
		v.Cut("(for (= (paren wp) firstwin) _ _ (block (for (= i 0) (< i (-> wp w_jumplistlen)) _ _)))",
			1, "a named buffer resolving jump-list file names")
	})
	v.InFunction("win_init", func(v *graph.Verbs) {
		v.Cut("(call copy_jumplist oldp newp)", 1, "a new window copying the jump list")
	})
	v.InFunction("win_free", func(v *graph.Verbs) {
		v.Cut("(call free_jumplist wp)", 1, "a closed window freeing the jump list")
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim3c", Edit) }
