package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// nocomplStubs are the predicates the whole subsystem hangs from, plus the
// popup menu's hooks into the REDRAW loop -- which update_screen() reaches
// rather than completion, so answering pum_visible() is not enough to orphan
// pum_redraw().  pum_display() goes too, so the island behind it is orphaned
// whichever caller survives: chasing them one at a time found three and missed
// two.
var nocomplStubs = []struct{ name, rep string }{
	{"ins_complete", "(return FAIL)"},
	{"ins_compl_prep", "(return FALSE)"},
	{"ins_compl_active", "(return FALSE)"},
	{"pum_visible", "(return FALSE)"},
	{"ins_compl_has_autocomplete", "(return FALSE)"},
	{"pum_redraw_in_same_position", "(return FALSE)"},
	{"pum_may_redraw", ""},
	{"pum_undisplay", ""},
	{"pum_display", ""},
}

// NoCompl removes insert-mode completion and the popup menu.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the stubs by Body, the
// stubbed lines the text's count on each function's C view; the guards and
// arms by form, scoped to their functions (history keeps the text version).
func NoCompl(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nocompl", e, w)
	q := graph.NewVerbs("nocompl", e, io.Discard)
	total := 0
	for _, s := range nocomplStubs {
		if e.Defn(s.name) == nil {
			v.Die("%s is not defined at file scope", s.name)
			return v.Done()
		}
		q.InFunction(s.name, func(q *graph.Verbs) { total += bodyLines(q.Text()) })
		q.Body(s.name, s.rep, s.name)
	}
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("%d lines stubbed in the five predicates the whole subsystem hangs from", total)

	// One reader the sweep cannot reach, because it is inside edit(): the
	// guard that sends CTRL-N and CTRL-P to `normalchar` when 'complete' is
	// empty.  With the option gone there is nothing to test, and the two keys
	// should take that path unconditionally -- which is what `goto normalchar`
	// already said they should.
	q.InFunction("edit", func(q *graph.Verbs) {
		q.DropIf("(&& (== (deref (-> curbuf b_p_cpt)) NUL) (|| (call ctrl_x_mode_normal) (call ctrl_x_mode_whole_line)) _)",
			1, "edit()'s test for an empty 'complete'")
	})
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("edit()'s test for an empty 'complete'")

	// has_compl_option() complains that 'dictionary' or 'thesaurus' is empty.
	// Both options are going, and its two callers are the CTRL-X submode arms
	// for them -- guarded by ctrl_x_mode_thesaurus() and
	// ctrl_x_mode_dictionary(), which can no longer be true.
	n := 0
	q.InFunction("edit", func(q *graph.Verbs) {
		n = q.Count("(if (&& (== c Ctrl_T) (call ctrl_x_mode_thesaurus)) _)") +
			q.Count("(if (call ctrl_x_mode_dictionary) _)")
		if n != 2 {
			q.Die("the CTRL-X dictionary/thesaurus arms are not where this expects (%d)", n)
			return
		}
		q.Cut("(if (&& (== c Ctrl_T) (call ctrl_x_mode_thesaurus)) _)", 1, "the thesaurus arm")
		q.Cut("(if (call ctrl_x_mode_dictionary) _)", 1, "the dictionary arm")
	})
	// has_compl_option itself is the collection's once its two callers are gone.

	// 'infercase' asked smartcase to stand down while completing.
	q.InFunction("ignorecase_opt", func(q *graph.Verbs) {
		q.DropOperand("(! (&& (call ctrl_x_mode_not_default) (-> curbuf b_p_inf)))", 1, "'infercase'")
	})

	// `:set autocomplete<` resets a buffer-local boolean to "ask the global":
	// an arm of an else-chain, the next one promoted.
	q.FoldNever("(&& (== (cast (ptr int) varp) (addr (-> curbuf b_p_ac))) (== opt_flags OPT_LOCAL))", 1, "`:set autocomplete<`")
	q.Cut("(= (-> curbuf b_p_ac) (- 1))", 1, "curbuf's b_p_ac reset")
	q.Cut("(= (-> buf b_p_ac) (- 1))", 2, "buf's b_p_ac resets")

	// set_shellsize_inner() redraws the popup menu when the terminal resizes --
	// a live path into the pum that completion itself does not reach.
	// SCOPED TO THAT FUNCTION: there are six `if (pum_visible())` in the file.
	if e.Defn("set_shellsize_inner") == nil {
		q.Die("set_shellsize_inner is not defined at file scope")
	}
	q.InFunction("set_shellsize_inner", func(q *graph.Verbs) {
		q.DropIf("(call pum_visible)", 1, "set_shellsize_inner's pum block")
		if q.Count("(call ins_compl_show_pum)") != 0 {
			q.Die("set_shellsize_inner's pum block is not the one cut")
		}
	})
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("has_compl_option, 'infercase' in smartcase, " +
		"`:set autocomplete<`, and the pum's resize hook")

	// didset_string_options() dereferences every string option's global once
	// at startup.  This is the FOURTH phase to meet it -- 20, 29, 30 and now
	// this one -- and here it was a segfault before the first keystroke,
	// because 'completeopt''s row goes and nothing else reads p_cot.
	q.Cut("(cast void (call opt_strings_flags p_cot p_cot_values (addr cot_flags) TRUE))", 1,
		"didset_string_options' p_cot line")
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("didset_string_options stops reading 'completeopt'")
	return v.Done()
}
