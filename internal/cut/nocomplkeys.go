package cut

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// complkeysStubs are predicates the rest of the editor asks on its own
// account.  Each already had only one honest answer; saying it in the body is
// what lets the collection reach everything behind it.
var complkeysStubs = []struct{ name, ret string }{
	{"ins_compl_win_active", "(return FALSE)"},
	{"ins_compl_lnum_in_range", "(return FALSE)"},
	{"ins_compl_col_range_attr", "return -1;"}, // C: BUILD leaves a unary minus untyped, FRAG types it
	{"ins_compl_preinsert_effect", "(return FALSE)"},
	{"ins_compl_autocomplete_pending", "(return FALSE)"},
	{"ins_compl_autocomplete_elapsed", "(return 0)"},
	{"pum_under_menu", "(return FALSE)"},
	{"pum_get_height", "(return 0)"},
	{"vim_is_ctrl_x_key", "(return FALSE)"},
	// With ins_ctrl_x() empty, `ctrl_x_mode` is never assigned and stays
	// CTRL_X_NORMAL, so these two are decided.  ins_ctrl_ey() then takes its
	// else branch, which is CTRL-Y and CTRL-E's ordinary meaning.
	{"ctrl_x_mode_scroll", "(return FALSE)"},
	{"at_ins_compl_key", "(return FALSE)"},
}

// the autocomplete arm, written four times in edit(): after
// `ins_just_started = FALSE;` and in three `if (did_backspace)` blobs
const complkeysAutoArm = "(if (&& (call ins_compl_has_autocomplete) (! (call char_avail)) (> (. (-> curwin w_cursor) col) 0)) _)"

// NoComplKeys takes the completion keys away.
//
// A CUT THAT IS NOT UNIQUE IS NOT A CUT, IT IS A GUESS: the text's first
// version dropped `if (c != KE_CURSORHOLD && c != KE_COMPLETE_DELAY)` by
// pattern, and took `{ lastc = c; }`.  So every condition is counted, in the
// file where the text counted it there, and in edit() or do_put() where a
// bare search would take a copy in the completion code itself.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the stubs by Body, the guards
// by DropIf and Rewrite by form, the runs of items by Run; `docomplete`'s
// label goes last, after every goto to it (history keeps the text version).
func NoComplKeys(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("complkeys", e, w)
	q := graph.NewVerbs("nocomplkeys", e, io.Discard)
	q.Body("ins_ctrl_x", "", "ins_ctrl_x")
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("CTRL-X opens no submode; ctrl_x_mode stays normal")

	q.Rewrite("(|| ?t (call ins_compl_active) compl_busy (call pum_visible))", "?t", 1, "edit()'s entry guard")
	q.InFunction("edit", func(q *graph.Verbs) {
		q.Cut("(call ins_compl_clear)", 1, "edit()'s ins_compl_clear")
		q.Rewrite("(&& stop_insert_mode (! (call ins_compl_active)))", "stop_insert_mode", 1, "the stop_insert_mode guard")
		if r := q.Run("the ins_just_started autocomplete arm", "(= ins_just_started FALSE)", complkeysAutoArm); r != nil {
			if err := e.Delete(r[1]); err != nil {
				q.Die("the ins_just_started autocomplete arm -- %v", err)
			}
		}
		q.Cut("(call ins_compl_prep ESC)", 1, "the ESC hook")
		q.Cut("(if (&& (!= c _) (!= c _)) (block (call ins_compl_clear_autocomplete_delay) (call ins_compl_disarm_autostart) _))",
			1, "the per-key autocomplete disarm")
	})
	q.DropIf("(&& (call ins_compl_active) (>= (. (-> curwin w_cursor) col) (call ins_compl_col)) (call ins_compl_has_shown_match) (call pum_wanted))",
		1, "a completion arm")
	q.InFunction("edit", func(q *graph.Verbs) {
		q.Cut("(call ins_compl_init_get_longest)", 1, "edit()'s init_get_longest")
	})
	q.DropIf("(call ins_compl_prep c)", 1, "the per-key prep hook")
	q.DropIf("(&& (|| (== c Ctrl_V) (== c Ctrl_Q)) (call ctrl_x_mode_cmdline))", 1, "CTRL-V in cmdline completion")
	q.InFunction("edit", func(q *graph.Verbs) {
		if r := q.Run("the doESCkey disarm", "(label doESCkey)", "(call ins_compl_clear_autocomplete_delay)"); r != nil {
			if err := e.Delete(r[1]); err != nil {
				q.Die("the doESCkey disarm -- %v", err)
			}
		}
	})
	q.DropIf("(&& (call ctrl_x_mode_register) (! (call ins_compl_active)))", 1, "a completion arm")
	if err := q.Done(); err != nil {
		return err
	}

	// The three autocomplete arms inside an `if (did_backspace)` each, the
	// if going with its only statement.
	blob := "(if did_backspace (block " + complkeysAutoArm + "))"
	var n int
	q.InFunction("edit", func(q *graph.Verbs) {
		if n = q.Count(blob); n != 3 {
			q.Die("the backspace autocomplete blobs -- expected 3, matched %d", n)
			return
		}
		q.Cut(blob, 3, "the backspace autocomplete blobs")
	})
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("autocomplete disarmed at %d sites", n+3)

	q.InFunction("edit", func(q *graph.Verbs) {
		if r := q.Run("the KE_COMPLETE_DELAY arm",
			"(call ins_compl_clear_autocomplete_delay)",
			"(if (|| (! (call ins_compl_has_autocomplete)) (call char_avail) _) (block (break)))",
			"(= c (call char_before_cursor))",
			"(if (! (call vim_isprintc c)) (block (break)))",
			"(call ins_compl_enable_autocomplete)",
			"(call ins_compl_arm_autostart)",
			"(goto docomplete)"); r != nil {
			if err := e.ReplaceRun(r[0], r[len(r)-1], graph.Break()); err != nil {
				q.Die("the KE_COMPLETE_DELAY arm -- %v", err)
			}
		}
		const pum = "(if (call pum_visible) (block (goto docomplete)))"
		if n := q.Count(pum); n != 4 {
			q.Die("the arrow-key pum arms -- expected 4, matched %d", n)
			return
		}
		q.Cut(pum, 4, "the arrow-key pum arms")
	})
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("Up, Down, PageUp and PageDown no longer move a selection")

	q.InFunction("edit", func(q *graph.Verbs) {
		for _, k := range []struct{ key, mode string }{
			{"Ctrl_RSB", "ctrl_x_mode_tags"},
			{"Ctrl_F", "ctrl_x_mode_files"},
			{"Ctrl_S", "ctrl_x_mode_spell"},
		} {
			q.SpliceFirst("(if (! (call "+k.mode+")) (block (goto normalchar)))", "(goto docomplete)",
				"(goto normalchar)", "the "+k.key+" arm")
		}
		// the last gotos to docomplete, before its label goes
		q.DropIf("(&& (call ins_compl_has_autocomplete) (! (call char_avail)) (call vim_isprintc c))", 1,
			"the printable-character autocomplete arm")
		// CTRL-L completes no whole line; CTRL-N and CTRL-P do nothing
		q.FoldAlways("(! (call ctrl_x_mode_whole_line))", 1, "the docomplete label")
		if r := q.Run("the docomplete label",
			"(attributed (std-attr fallthrough))", "(case Ctrl_P)", "(case Ctrl_N)", "(label docomplete)",
			"(call ins_compl_clear_autocomplete_delay)", "(= compl_busy TRUE)", "(if (== (call ins_complete c TRUE) FAIL) _)",
			"(= compl_busy FALSE)", "(= can_si (call may_do_si))"); r != nil {
			for _, x := range append([]*graph.Node{r[0]}, r[3:]...) {
				if err := e.Delete(x); err != nil {
					q.Die("the docomplete label -- %v", err)
					return
				}
			}
		}
	})
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("docomplete is gone; CTRL-N and CTRL-P do nothing")

	q.DropIf("(&& (call ins_compl_active) (! (call ins_compl_win_active curwin)))", 1, "the end-of-loop cancel")
	if e.Defn("do_put") == nil {
		q.Die("do_put is not defined")
	}
	q.InFunction("do_put", func(q *graph.Verbs) {
		q.DropIf("(call ins_compl_preinsert_effect)", 1, "do_put's preinsert cleanup")
	})
	for _, s := range complkeysStubs {
		if strings.HasSuffix(s.ret, ";") {
			q.BodyC(s.name, s.ret, s.name)
		} else {
			q.Body(s.name, s.ret, s.name)
		}
	}
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("%d predicates answer without the machinery", len(complkeysStubs))
	return v.Done()
}
