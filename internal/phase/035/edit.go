package p035

// Whim phase 35 (formerly 96) -- no `FILE *` that is never opened.  See GOAL.md.
//
// Two `static FILE *` survive in this editor and NOTHING HAS EVER OPENED EITHER OF
// THEM IN ANY BUILD OF whim-vim: `scriptin[NSCRIPT]`, which `-s {scriptfile}` filled
// and which whim removed the option for, and `redir_fd`, which `:redir > file` filled
// and which whim removed the command for.  So this phase removes the POSSIBILITY
// rather than a behaviour -- the same situation as phase 31, and the same answer:
// nothing it takes is reachable, the declared delta is nothing at all, and the
// evidence is an instrumented pair and a set of counts.
//
// THE COUNTS ARE THE ARGUMENT, and each is asserted before anything is folded:
//
// * `scriptin[]` is assigned in exactly ONE place in the whole file, and that place
// is `scriptin[curscript] = NULL;` inside `closescript()`.  So it is NULL for
// ever, `== NULL` is TRUE and `!= NULL` is FALSE at every site.
// * `redir_fd`'s only assignment is its own declaration, `= NULL`.  Same.
// * `ui_write()` has three mentions -- a prototype, a definition and ONE call --
// and that call passes `FALSE` for `console`.
//
// SIX ANCHORS, in three groups.
//
// A  scriptin[] is NULL for ever.
// 1  `may_sync_undo()` loses one conjunct and SURVIVES: `u_sync()` still runs on
// the same condition.
// 2  `is_safe_now()` loses one conjunct and SURVIVES.
// 3  `using_script()` is FALSE at both call sites -- a `&& !using_script()`
// conjunct and a `|| using_script()` disjunct -- and the sweep then takes it.
// 4  `inchar()`'s script reader, deleted as TEXT with its local, after which
// `if (script_char < 0)` is always true and folds.  THAT FOLD IS WHAT TAKES
// `closescript()`'s only caller, and `fclose` and `getc` with it.
// B  redir_fd is NULL for ever, so `redirecting()` is FALSE always and folds at
// BOTH call sites.  Their indentation differs, which is what makes two separate
// one-count patterns honest rather than a count of two over one pattern.
// C  `ui_write()`'s `console` is FALSE at its one call site, so the `vim_fsync(1)`
// it guards can never be entered.  THE PARAMETER GOES TOO, and that is what
// makes the cut honest: leaving it would leave `__attribute__((unused))` on
// something that will never be read again, which is phase 4e's argument for
// `check_tty(void)` -- and tools/sweep.sh compiles with -Wno-unused-parameter,
// so an unused parameter is invisible where an unused local is not.
//
// TWO LOCALS ARE FOLDED BY HAND AND NO TOOL COVERS EITHER.
//
// `retesc` is `FALSE` at its declaration, is written only inside the loop anchor A4
// deletes, and is read once.  Afterwards it is a local that is READ AND NEVER
// WRITTEN: gcc has no warning for that, tools/deadsweep.py acts on warnings, and
// leaving it would mean `inchar()` returns an uninitialised value on a path the
// compiler thinks exists.  `return retesc;` becomes `return FALSE;`, and the
// declaration, unused then, is the sweep's.  This is phase 29's `usefilter` judgement in this phase's shape.
//
// `did_return` is the same shape one level down: the `if (!did_return)` block the
// redir_write extra removes is its only reader, and an `if` with an empty body is
// not something any tool here removes either, so the block goes whole with
// cutil.drop_if, its one write goes, and the sweep takes the declaration.
//
// THE RECOMMENDED EXTRA IS TAKEN: `redir_write()` IS A NO-OP AFTERWARDS.  After B it
// is `{ char_u *s = str; static int cur_col = 0; if (redir_off) return; }` -- the
// sweep takes the two variables and leaves a function with five callers that cannot
// do anything.  Leaving it is the "concept the table has and the code does not" that
// record 18 argued against, so its five call sites go and the sweep takes it, and
// `redir_off` -- then written five times and read never, a file-scope static no
// warning covers -- loses its writes, and the sweep its declaration.
//
// A SECOND EXTRA IS DECLINED AND IS A QUESTION FOR THE USER, not an oversight.  After
// this phase `typedef struct stat stat_T;` has no user and `#include <sys/stat.h>`
// and `#include <fcntl.h>` are needed by nothing.  Removing all three is free -- it
// was measured: same binary, byte-identical recording -- but it would be the FIRST
// TIME ANY PART II PHASE CHANGES THE DIRECTIVE COUNT, and GOALS.md's charter says
// `whim-vim.c` "inherits 18 directives from `whim-vim.c`".  That sentence is a
// statement about the pipeline, so the change belongs to whoever decides it, either
// here or as an includes phase of its own.  The count stays 18.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, and the source goes with it as $state/old.c.  The check needs both: there is
// no behavioural probe this phase can offer, so it instruments the source it was
// HANDED at five places and requires zero markers, with a control that must fire.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  The same acts on the
// program's graph, the report the text version's (history keeps it, with
// editlit.go's literals): the anchors and the invariant -- the one write of
// scriptin[], redirecting()'s body, the one ui_write() call -- are the
// text's own questions on the C view (TEXTQ); the conjuncts and disjuncts
// are DropOperand; the script reader is the run of its two items; the
// folds are by condition; the writes and calls are Cuts of their forms;
// and ui_write's console parameter is PARAM's, one edit for the prototype,
// the definition (its guarded vim_fsync() cut first) and the call.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim35", Edit) }

var w35Before = map[string]int{
	"scriptin": 8, "curscript": 11, "NSCRIPT": 3, "saved_typebuf": 2,
	"closescript": 3, "using_script": 3, "script_char": 6, "retesc": 3,
	"redir_fd": 0, "redir_off": 7, "redir_write": 7, "redirecting": 4,
	"did_return": 3,
	"vim_fsync":  3, "ui_write": 3, "mch_write": 2, "FILE": 1,
	"may_sync_undo": 3, "is_safe_now": 3, "free_typebuf": 5,
	"read_cmd_fd": 12,
}

var w35After = map[string]int{
	"redirecting": 3, "closescript": 2, "vim_fsync": 2, "using_script": 1,
	"redir_write": 2, "redir_off": 2, "did_return": 1, "retesc": 1, "script_char": 1,
	"scriptin": 4, "curscript": 7,
	"may_sync_undo": 3, "is_safe_now": 3,
	"ui_write": 3, "mch_write": 2, "read_cmd_fd": 12,
}

var (
	w35ScriptWrite = regexp.MustCompile(`\bscriptin\s*\[[^\]]*\]\s*=[^=][^;\n]*;`)
	w35UiCall      = regexp.MustCompile(`(?m)^[ \t]*ui_write\([^;\n]*\);$`)
)

func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nofile", e, w)

	joined := func(ms []string, n int) string {
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = edit.CoreHead(strings.TrimSpace(m), n)
		}
		return strings.Join(out, " | ")
	}

	t := v.Text()
	for _, name := range edit.SortedKeys(w35Before) {
		if k := edit.WordPatternCount(t, name); k != w35Before[name] {
			v.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w35Before[name])
			return v.Done()
		}
	}
	v.Say("scriptin 8, redirecting 4, ui_write 3, FILE 1 -- the file the six " +
		"anchors were counted against")

	sw := w35ScriptWrite.FindAllString(string(t), -1)
	if len(sw) != 1 || sw[0] != "scriptin[curscript] = nullptr;" {
		out := make([]string, len(sw))
		for i, m := range sw {
			out[i] = edit.CoreHead(m, 60)
		}
		v.Die("scriptin[] is assigned %d times and not once to nullptr alone: %s",
			len(sw), strings.Join(out, " | "))
		return v.Done()
	}
	if strings.Contains(string(t), "redir_fd") {
		v.Die("redir_fd survives, and part B's folds assume it went at phase 1")
		return v.Done()
	}
	if !regexp.MustCompile(`(?m)^redirecting\(void\)\n\{\n    return FALSE;\n\}`).Match(t) {
		v.Die("redirecting() does not answer FALSE, and part B folds it so")
		return v.Done()
	}
	calls := w35UiCall.FindAllString(string(t), -1)
	if len(calls) != 1 || calls[0] != "    ui_write(out_buf, len, FALSE);" {
		v.Die("ui_write has %d call sites and not the one that passes FALSE: %s",
			len(calls), joined(calls, 60))
		return v.Done()
	}
	v.Say("scriptin[] is assigned ONCE in the whole file, to nullptr, inside closescript(); " +
		"redirecting() answers FALSE; ui_write() has ONE call and it passes " +
		"FALSE.  That, and nothing weaker, is why every fold below may take a constant -- " +
		"and it is also the whole claim of the phase: neither FILE* has ever been opened " +
		"in any build of whim-vim")

	null := "(== (index scriptin curscript) nullptr)"
	v.InFunction("may_sync_undo", func(v *graph.Verbs) {
		v.DropOperand(null, 1, "may_sync_undo: `scriptin[curscript] == nullptr` is TRUE, so the conjunct "+
			"goes -- the function SURVIVES and u_sync() still runs on the rest")
	})
	v.InFunction("is_safe_now", func(v *graph.Verbs) {
		v.DropOperand(null, 1, "is_safe_now: the same conjunct, and the same survival -- "+
			"stuff_empty() && typebuf.tb_len == 0 && !global_busy is what is left")
	})
	// (nv_visual, the text's report says; the conjunct is clear_showcmd's)
	v.InFunction("clear_showcmd", func(v *graph.Verbs) {
		v.DropOperand("(! (call using_script))", 1, "nv_visual: `!using_script()` is TRUE, so the conjunct goes")
	})
	v.InFunction("skip_showmode", func(v *graph.Verbs) {
		v.DropOperand("(call using_script)", 1, "skip_showmode: `using_script()` is FALSE, so the disjunct goes -- and "+
			"that was its last caller")
	})
	v.InFunction("inchar", func(v *graph.Verbs) {
		v.CutRun("inchar()'s script reader: the loop needs `scriptin[curscript] != nullptr`, "+
			"which is FALSE, so it never ran -- and it was closescript()'s only "+
			"caller and getc()'s",
			"(= script_char (- 1))",
			"(while (&& (!= (index scriptin curscript) nullptr) (< script_char 0)) _*)")
	})
	if v.Failed() {
		return v.Done()
	}
	if k := edit.WordPatternCount(v.Text(), "retesc"); k != 2 {
		v.Die("retesc has %d mentions after the loop went, expected 2 -- its declaration "+
			"and its one read", k)
		return v.Done()
	}
	v.InFunction("inchar", func(v *graph.Verbs) {
		v.Rewrite("(return retesc)", "(return FALSE)", 1,
			"inchar: `retesc` is read and never written now -- no warning covers "+
				"that, and the value it would return is uninitialised, so the read "+
				"becomes the FALSE it was initialised to")
		v.FoldAlwaysAt("(< script_char 0)", 1, false,
			"inchar: `script_char < 0` is TRUE for ever now, so the whole rest of the "+
				"function is what runs -- the body is dedented one level")
	})
	v.InFunction("undo_cmdmod", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "(call redirecting)", 1,
			"undo_cmdmod: `redirecting()` is FALSE, so unwinding :silent never resets "+
				"msg_col for a redirection that is not happening")
	})
	v.DropIf("(! did_return)", 1,
		"msg_end's `if (!did_return)` block, which held one of the five calls: dropping "+
			"the call alone would leave an `if` with an empty body, which is not something "+
			"any tool here removes")
	v.Cut("(call redir_write p (- 1))", 2, "emsg_core's two calls that echoed the error's source line")
	v.Cut("(call redir_write (cast (ptr char_u) s) (- 1))", 1, "emsg_core's call that echoed the message itself")
	v.Cut("(call redir_write (cast (ptr char_u) str) maxlen)", 1,
		"msg_puts_attr_len's call, which was every message the editor prints")
	if v.Failed() {
		return v.Done()
	}
	if k := edit.WordPatternCount(v.Text(), "redir_write"); k != 2 {
		v.Die("redir_write has %d mentions, expected 2 -- its prototype and its "+
			"definition, uncalled now and the collection's", k)
		return v.Done()
	}

	v.Cut("(= did_return TRUE)", 1, "msg_end's `did_return`'s one write: it is read never now")
	v.CutWhere("(= redir_off _)", func(x *graph.Node) bool {
		val := x.Kids[2]
		return e.Item(x) == x && !val.IsList() && (val.Atom == "TRUE" || val.Atom == "FALSE")
	}, 5, "and redir_off's FIVE writes, not four: it has no reader at all")
	if v.Failed() {
		return v.Done()
	}
	t = v.Text()
	if edit.WordPatternCount(t, "redir_off") != 2 || edit.WordPatternCount(t, "did_return") != 1 {
		v.Die("redir_off or did_return is named other than by its declaration " +
			"(and, for redir_off, uncalled redir_write's read)")
		return v.Done()
	}

	// ui_write's console: the guarded vim_fsync() and the parameter, one
	// edit for the prototype, the definition and the one call
	v.Say("ui_write's prototype loses the console parameter")
	v.InFunction("ui_write", func(v *graph.Verbs) {
		v.CutWhere("(if (&& console (== (index s (- len 1)) '\\n')) (block (call vim_fsync 1)))", nil, 1, "")
	})
	v.Say("and the definition: `console` is FALSE at the one call site, so the " +
		"vim_fsync(1) it guarded can never be entered, and ui_write is " +
		"mch_write now -- which is what takes vim_fsync() and fsync()")
	v.CountIs("(call ui_write out_buf len FALSE)", 1, "and its one call site, which already passed FALSE")
	if !v.Failed() {
		if _, err := e.DropParam("ui_write", "console", graph.ParamOptions{}); err != nil {
			v.Die("and its one call site, which already passed FALSE -- %v", err)
		}
	}
	v.Say("and its one call site, which already passed FALSE")
	if v.Failed() {
		return v.Done()
	}

	t = v.Text()
	for _, name := range edit.SortedKeys(w35After) {
		if k := edit.WordPatternCount(t, name); k != w35After[name] {
			v.Die("%s has %d mentions after the cut, expected %d", name, k, w35After[name])
			return v.Done()
		}
	}
	v.Say("the cut is done: closescript and vim_fsync at two mentions each " +
		"-- a prototype and a definition -- redirecting at three, uncalled redir_write's " +
		"call the third, and using_script at ONE, its definition " +
		"alone, having never had a prototype; scriptin 4 and curscript 7, every one of " +
		"them inside a function the collection now reads as unreachable; may_sync_undo and " +
		"is_safe_now SURVIVING at three; and read_cmd_fd 12, still the terminal's")
	return v.Done()
}
