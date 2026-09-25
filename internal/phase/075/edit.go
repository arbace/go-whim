package p075

// Whim phase 75 -- no autocommands.  See GOAL.md.
//
// PROVED BY ABSENCE, not inferred from the command table.  `first_autopat[NUM_EVENTS]
// = { NULL }` is the ONLY write to that array in the whole file -- every other mention
// reads it.  No autocommand pattern can ever be registered, so:
//
// * apply_autocmds_group() is already `return FALSE;`
// * apply_autocmds(), apply_autocmds_exarg() and apply_autocmds_retval() are
// one-line wrappers onto it, so ALL ~74 dispatch sites are no-ops;
// * has_cursormovedI/has_textchangedI/has_textchangedP each return
// `first_autopat[...] != NULL`, i.e. always FALSE;
// * au_cleanup, au_remove_pat, au_del_cmd and aubuflocal_remove walk a permanently
// empty list.
//
// :autocmd, :augroup, :doautocmd, :doautoall and :noautocmd were already ex_ni, but
// that is the weaker argument; the array being write-once-to-NULL is the strong one.
//
// TWO SITES ARE REWRITTEN, NOT FOLDED, and both would have been silent damage:
//
// * close_buffer() -- the label `aucmd_abort:` sits INSIDE the block guarded by
// apply_autocmds(EVENT_BUFWINLEAVE, ...), and THREE gotos target it, two of them
// from outside that block under `if (abort_if_last)`.  fold_never would delete
// the label and orphan them.  This is the phase-71 break-rebinding hazard wearing
// a label instead of a loop.  The abort arm is hoisted out and kept reachable.
// * open_buffer() -- the aco block mixes the autocommand call with REAL work:
// `curbuf->b_flags &= ~(BF_CHECK_RO | BF_NEVERLOADED)`.  Deleting it wholesale
// would change behaviour.
//
// THE CLASSIFICATION WAS DONE BY HAND because a scan got two sites BACKWARDS.
// 7712 and 7741 read `if (!(did_cmd = apply_autocmds_exarg(...)))` -- negated with an
// embedded assignment -- so they are ALWAYS TRUE (fold_always), not always false.
// A `startswith("if (!apply_autocmds")` test misses the `!(var = ...)` shape, and
// folding them the other way would have deleted the branch that actually runs.
//
// fold_never (condition always FALSE)   6327 6344 6416 6423 6768 27758 27768
// fold_always (condition always TRUE)   6531 7712 7741
// dies with its guard                   15497 15512 (has_textchanged*), 21638
// (has_cmdundefined)
// value consumed                        7725 (did_cmd), 18098 (ins_apply_autocmds
// -> return FALSE), 86559 (drop the |= term)
// bare statements                       60 lines, deleted
//
// WHAT GOES BY CASCADE: the EVENT_ enum (123 enumerators, 127 lines), event_tab
// (127 rows), event_nr2name, auto_next_pat, AutoPat, AutoCmd, AutoPatCmd_T,
// active_apc_list, first_autopat, last_autopat, au_need_clean, autocmd_blocked,
// aucmd_prepbuf, aucmd_restbuf and aco_save_T.  Nothing here deletes those by name.
//
// SCOPE NOTE: trigger_cmd_autocmd() comes out HERE rather than with the other empty
// functions, because its call sites pass EVENT_* constants that this phase removes.
// may_trigger_modechanged() takes no argument and waits for the combined phase.
//
// THE DELTA: none expected.  Nothing could fire an autocommand, so removing the
// dispatch cannot change what the editor does.  Declared empty, left for the delta check.

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

// writesTo returns the 1-based line of every assignment TO name, or to an
// element of it.
//
// It steps over a BALANCED SUBSCRIPT and only then looks at the operator.  A
// first version matched `name[^\n;]*=` and reported three writes that were the
// `!=` of the has_* predicates -- it spanned the subscript and landed on the
// comparison.  AN ASSERTION THAT CRIES WOLF IS WORSE THAN NONE, because the
// temptation is to loosen it until it passes.
func writesTo(text []byte, name string) []int {
	var Out []int
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	for _, m := range re.FindAllIndex(text, -1) {
		end := m[1] + 80
		if end > len(text) {
			end = len(text)
		}
		s := strings.TrimLeft(string(text[m[1]:end]), " \t\n")
		if strings.HasPrefix(s, "[") {
			depth := 0
			for i, ch := range s {
				if ch == '[' {
					depth++
				} else if ch == ']' {
					depth--
					if depth == 0 {
						s = s[i+1:]
						break
					}
				}
			}
			s = strings.TrimLeft(s, " \t\n")
		}
		if strings.HasPrefix(s, "=") && !strings.HasPrefix(s, "==") {
			Out = append(Out, 1+edit.CountNewlines(text[:m[0]]))
		}
	}
	return Out
}

// Whim75 removes the autocommand dispatch, having first PROVED that nothing can
// register one.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("noautocmd", text, w)

	if len(edit.AutopatDecl.FindAll(text, -1)) != 1 {
		e.Refuse("the first_autopat declaration is not where this phase expects it")
		return e.Done()
	}
	if ws := writesTo(text, "first_autopat"); len(ws) != 1 {
		e.Refuse("first_autopat is assigned in %d place(s), not just its all-NULL initialiser (lines %s) -- an autocommand CAN be registered and this whole phase is wrong",
			len(ws), edit.JoinInts(ws))
		return e.Done()
	}
	e.Say("confirmed: first_autopat is only ever the all-NULL initialiser")

	e.InFunction("close_buffer", func(e *edit.E) {
		e.Literal(w75OldCb, w75NewCb, 1, "close_buffer, whose abort label three gotos still target")
	})
	e.InFunction("buf_freeall", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(apply_autocmds\(EVENT_BUFUNLOAD,`, 1, "unloading a buffer asking the autocommands first")
	})
	e.InFunction("buf_freeall", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(apply_autocmds\(EVENT_BUFWIPEOUT,`, 1, "wiping a buffer asking them")
	})
	e.InFunction("buflist_new", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(apply_autocmds\(EVENT_BUFNEW,`, 1, "a new buffer announcing itself")
	})
	e.InFunction("readfile", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(apply_autocmds_exarg\(EVENT_BUFREADCMD,`, 1, "a read being handled by an autocommand instead")
	})
	e.InFunction("readfile", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*else if \(apply_autocmds_exarg\(EVENT_FILEREADCMD,`, 1, "and the file-read variant of the same")
	})
	e.InFunction("set_curbuf", func(e *edit.E) {
		e.FoldAlways(`(?m)^[ \t]*if \(!apply_autocmds\(EVENT_BUFLEAVE,`, 1, "leaving a buffer asking permission")
	})
	e.InFunction("buf_write", func(e *edit.E) {
		e.FoldAlways(`(?m)^[ \t]*if \(!\(did_cmd = apply_autocmds_exarg\(EVENT_FILEAPPENDCMD,`, 1, "an autocommand taking over an append")
	})
	e.InFunction("buf_write", func(e *edit.E) {
		e.FoldAlways(`(?m)^[ \t]*if \(!\(did_cmd = apply_autocmds_exarg\(EVENT_FILEWRITECMD,`, 1, "an autocommand taking over a write")
	})
	e.InFunction("ins_redraw", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(ready && has_textchangedI\(\)`, 1, "insert mode reporting a change")
	})
	e.InFunction("ins_redraw", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(ready && has_textchangedP\(\)`, 1, "and the popup-menu variant")
	})
	e.InFunction("do_one_cmd", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(p != NULL && ea\.cmdidx == CMD_SIZE && !ea\.skip && [^\n]*has_cmdundefined\(\)\)$`, 1, "an unknown command being defined by an autocommand")
	})
	e.InFunction("buf_write", func(e *edit.E) {
		e.Literal(w75lit4, "", 1, "an autocommand taking over the whole write")
	})
	e.Body("ins_apply_autocmds", w75lit2, "ins_apply_autocmds, which dispatched and watched the tick")
	e.InFunction("ui_focus_change", func(e *edit.E) {
		e.Literal(w75lit5, "", 1, "a focus change telling the autocommands")
	})
	e.InFunction("open_buffer", func(e *edit.E) {
		e.Literal(w75lit6, w75lit7, 1, "open_buffer, keeping the flag clearing the autocmd call was wrapped around")
	})
	e.DropBlocks("buf_write", `(?m)^[ \t]*if \(!got_int\)$`, 1, "the post-write announcements")
	e.DropBlocks("set_termname", `(?m)^[ \t]*if \(curbuf->b_ml\.ml_mfp != NULL\)$`, 1, "a new terminal telling every buffer")
	e.Lines(`ins_apply_autocmds\(EVENT_[A-Z]+\);`, 6, "the insert-mode dispatches")
	e.InFunction("ins_redraw", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(ready && \(has_cursormovedI\(\)\)`, 1, "insert mode reporting the cursor moved")
	})
	e.InFunction("free_buffer", func(e *edit.E) {
		e.Lines(`aubuflocal_remove\(buf\);`, 1, "a freed buffer detaching its buffer-local patterns")
	})
	for _, ev := range []string{"VIMLEAVEPRE", "VIMLEAVE"} {
		ev := ev
		e.InFunction("getout", func(e *edit.E) {
			e.Literal(fmt.Sprintf(w75lit14, ev), "", 1, fmt.Sprintf("quitting unblocking autocommands to announce EVENT_%s", ev))
		})
	}
	e.InFunction("do_one_cmd", func(e *edit.E) {
		e.Literal(w75lit8, w75lit9, 1, "asking whether the command came from an autocommand")
	})
	// the command-line type: its one write goes, and the sweep takes the
	// declaration
	e.InFunction("getcmdline_int", func(e *edit.E) {
		e.Lines(`cmdline_type = firstc == NUL \? '-' : firstc;`, 1, "the line that set the command-line type")
	})
	if !e.Failed() {
		n := len(edit.BareDispatch.FindAll(e.Text(), -1))
		e.Set(edit.BareDispatch.ReplaceAll(e.Text(), nil))
		e.Say(fmt.Sprintf("every remaining bare dispatch (%d)", n))
		n = len(edit.CmdTrigger.FindAll(e.Text(), -1))
		e.Set(edit.CmdTrigger.ReplaceAll(e.Text(), nil))
		e.Say(fmt.Sprintf("the command-line triggers (%d)", n))
	}
	e.InFunction("buf_write", func(e *edit.E) {
		e.Literal(w75lit10, "", 1, "buf_write bracketing the write with an autocommand buffer swap")
	})
	e.InFunction("buf_write", func(e *edit.E) {
		e.Literal(w75lit11, w75lit12, 1, "the write asking whether an autocommand had taken over")
	})
	e.InFunction("buf_write", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(did_cmd\)$`, 1, "and the arm only an autocommand-driven write could reach")
	})
	// buf_write's aco, bufref and did_cmd are named by nothing now; the sweep
	// takes them.
	e.DropBareBlock("set_termname", "buf = curbuf;", "the husk the terminal notification left behind")
	return e.Done()
}

func init() { edit.Register("whim75", Edit) }
