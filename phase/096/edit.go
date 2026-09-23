package p096

// Whim phase 96 -- no `FILE *` that is never opened.  See GOAL.md.
//
// Two `static FILE *` survive in this editor and NOTHING HAS EVER OPENED EITHER OF
// THEM IN ANY BUILD OF zero-vim: `scriptin[NSCRIPT]`, which `-s {scriptfile}` filled
// and which whim removed the option for, and `redir_fd`, which `:redir > file` filled
// and which whim removed the command for.  So this phase removes the POSSIBILITY
// rather than a behaviour -- the same situation as phase 92, and the same answer:
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
// something that will never be read again, which is phase 85's argument for
// `check_tty(void)` -- and tools/sweep.sh compiles with -Wno-unused-parameter,
// so an unused parameter is invisible where an unused local is not.
//
// TWO LOCALS ARE FOLDED BY HAND AND NO TOOL COVERS EITHER.
//
// `retesc` is `FALSE` at its declaration, is written only inside the loop anchor A4
// deletes, and is read once.  Afterwards it is a local that is READ AND NEVER
// WRITTEN: gcc has no warning for that, tools/deadsweep.py acts on warnings, and
// leaving it would mean `inchar()` returns an uninitialised value on a path the
// compiler thinks exists.  `return retesc;` becomes `return FALSE;` and the
// declaration goes.  This is phase 90's `usefilter` judgement in this phase's shape.
//
// `did_return` is the same shape one level down: the `if (!did_return)` block the
// redir_write extra removes is its only reader, and an `if` with an empty body is
// not something any tool here removes either, so the block goes whole with
// cutil.drop_if and the variable's two lines go with it.
//
// THE RECOMMENDED EXTRA IS TAKEN: `redir_write()` IS A NO-OP AFTERWARDS.  After B it
// is `{ char_u *s = str; static int cur_col = 0; if (redir_off) return; }` -- the
// sweep takes the two variables and leaves a function with five callers that cannot
// do anything.  Leaving it is the "concept the table has and the code does not" that
// phase 18 argued against, so it goes with its five call sites, and
// `redir_off` -- then written five times and read never, a file-scope static no
// warning covers -- goes with them.
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
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own makefile
// flags, and the source goes with it as $state/old.c.  The check needs both: there is
// no behavioural probe this phase can offer, so it instruments the source it was
// HANDED at five places and requires zero markers, with a control that must fire.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim96", Edit) }

var z13Before = map[string]int{
	"scriptin": 8, "curscript": 11, "NSCRIPT": 3, "saved_typebuf": 2,
	"closescript": 3, "using_script": 3, "script_char": 6, "retesc": 3,
	"redir_fd": 6, "redir_off": 7, "redir_write": 7, "redirecting": 4,
	"did_return": 3,
	"vim_fsync":  3, "ui_write": 3, "mch_write": 2, "FILE": 2,
	"may_sync_undo": 3, "is_safe_now": 3, "free_typebuf": 5,
	"read_cmd_fd": 12,
}

var z13After = map[string]int{
	"redirecting": 2, "closescript": 2, "vim_fsync": 2, "using_script": 1,
	"redir_write": 0, "redir_off": 0, "did_return": 0, "retesc": 0, "script_char": 0,
	"scriptin": 4, "curscript": 7,
	"may_sync_undo": 3, "is_safe_now": 3,
	"ui_write": 3, "mch_write": 2, "read_cmd_fd": 12,
}

var (
	z13ScriptWrite = regexp.MustCompile(`\bscriptin\s*\[[^\]]*\]\s*=[^=][^;\n]*;`)
	z13RedirWrite  = regexp.MustCompile(`(?m)^[ \t]*(?:static\s+FILE\s*\*\s*)?redir_fd\s*=[^=][^;]*;$`)
	z13UiCall      = regexp.MustCompile(`(?m)^[ \t]*ui_write\([^;\n]*\);$`)
	z13RedirOff    = regexp.MustCompile(`(?m)^[ \t]*redir_off = (?:TRUE|FALSE);\n`)
)

// Whim96 removes the two `static FILE *` that nothing has ever opened in any
// build of whim-vim, ui_write()'s console parameter, and the five functions the
// sweep finds under them.
//
// ITS ARGUMENT IS TEXTUAL AND THE INVARIANT IS COMPUTED: scriptin[] is assigned
// ONCE in the whole file, to NULL, inside the function this phase removes, and
// redir_fd only by its own declaration -- so the phase removes the POSSIBILITY
// and not a behaviour.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "nofile", W: w}
	var err error

	mentions := func(t []byte, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAll(t, -1))
	}
	textEdit := func(t []byte, old, new, what string, n int) ([]byte, error) {
		k := strings.Count(string(t), old)
		if k != n {
			return nil, p.Die("%s -- the text occurs %d times, expected %d: %s",
				what, k, n, cutil.PyRepr(edit.ZHead(old, 70)))
		}
		p.Say(what)
		return []byte(strings.ReplaceAll(string(t), old, new)), nil
	}
	fold := func(t []byte, fn, how, pattern, what string, n int) ([]byte, error) {
		a, z, ok := cutil.FindDefinition(t, cutil.Blank(t), fn)
		if !ok {
			return nil, p.Die("%s is not defined", fn)
		}
		f := cutil.FoldNever
		switch how {
		case "always":
			f = cutil.FoldAlways
		case "drop":
			f = cutil.DropIf
		}
		Body, err := f(t[a:z], pattern, n)
		if err != nil {
			return nil, p.Die("%s -- %v", what, err)
		}
		p.Say(what)
		return []byte(string(t[:a]) + string(Body) + string(t[z:])), nil
	}
	joined := func(ms []string, n int) string {
		Out := make([]string, len(ms))
		for i, m := range ms {
			Out[i] = edit.ZHead(strings.TrimSpace(m), n)
		}
		return strings.Join(Out, " | ")
	}

	// ---- 0. the shape every anchor below was counted against ------------------
	for _, name := range edit.SortedKeys(z13Before) {
		if k := mentions(text, name); k != z13Before[name] {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, z13Before[name])
		}
	}
	p.Say("scriptin 8, redir_fd 6, redirecting 4, ui_write 3, FILE 2 -- the file the six " +
		"anchors were counted against")

	// ---- THE INVARIANT, COMPUTED BEFORE ANYTHING IS FOLDED --------------------
	sw := z13ScriptWrite.FindAllString(string(text), -1)
	if len(sw) != 1 || sw[0] != "scriptin[curscript] = NULL;" {
		Out := make([]string, len(sw))
		for i, m := range sw {
			Out[i] = edit.ZHead(m, 60)
		}
		return nil, p.Die("scriptin[] is assigned %d times and not once to NULL alone: %s",
			len(sw), strings.Join(Out, " | "))
	}
	rw := z13RedirWrite.FindAllString(string(text), -1)
	if len(rw) != 1 || rw[0] != "static FILE *redir_fd  = NULL ;" {
		return nil, p.Die("redir_fd is assigned %d times and not once by its declaration alone: %s",
			len(rw), joined(rw, 60))
	}
	calls := z13UiCall.FindAllString(string(text), -1)
	if len(calls) != 1 || calls[0] != "    ui_write(out_buf, len, FALSE);" {
		return nil, p.Die("ui_write has %d call sites and not the one that passes FALSE: %s",
			len(calls), joined(calls, 60))
	}
	p.Say("scriptin[] is assigned ONCE in the whole file, to NULL, inside closescript(); " +
		"redir_fd only by its own declaration; ui_write() has ONE call and it passes " +
		"FALSE.  That, and nothing weaker, is why every fold below may take a constant -- " +
		"and it is also the whole claim of the phase: neither FILE* has ever been opened " +
		"in any build of whim-vim")

	// ---- A. scriptin[] is NULL for ever ---------------------------------------
	for _, e := range []struct {
		Old, New, What string
		n              int
	}{
		{z13lit1, z13lit2, "may_sync_undo: `scriptin[curscript] == NULL` is TRUE, so the conjunct " +
			"goes -- the function SURVIVES and u_sync() still runs on the rest", 1},
		{z13lit3, "", "is_safe_now: the same conjunct, and the same survival -- " +
			"stuff_empty() && typebuf.tb_len == 0 && !global_busy is what is left", 1},
		{" && !using_script()", "", "nv_visual: `!using_script()` is TRUE, so the conjunct goes", 1},
		{" || using_script()", "", "skip_showmode: `using_script()` is FALSE, so the disjunct goes -- and " +
			"that was its last caller", 1},
		{z13lit4, "", "inchar()'s script reader: the loop needs `scriptin[curscript] != NULL`, " +
			"which is FALSE, so it never ran -- and it was closescript()'s only " +
			"caller and getc()'s", 1},
	} {
		if text, err = textEdit(text, e.Old, e.New, e.What, e.n); err != nil {
			return nil, err
		}
	}
	// `retesc` IS READ AND NEVER WRITTEN AFTER A4, which draws no warning and
	// which deadsweep.py does not act on.
	if k := mentions(text, "retesc"); k != 2 {
		return nil, p.Die("retesc has %d mentions after the loop went, expected 2 -- its declaration "+
			"and its one read", k)
	}
	for _, e := range []struct{ Old, New, What string }{
		{z13lit5, z13lit6, "inchar: `retesc` is read and never written now -- no warning covers " +
			"that, and the value it would return is uninitialised, so the read " +
			"becomes the FALSE it was initialised to"},
		{z13lit7, "", "and its declaration goes with it"},
		{z13lit8, "", "and the script_char local itself, which nothing writes now"},
	} {
		if text, err = textEdit(text, e.Old, e.New, e.What, 1); err != nil {
			return nil, err
		}
	}
	// IT IS FOLDED LAST, after the two locals above: fold_always dedents the Body
	// it keeps, and `return retesc;` sits inside it -- a rewrite counted against
	// the original indentation would refuse afterwards.
	if text, err = fold(text, "inchar", "always", `(?m)^    if \(script_char < 0\)$`,
		"inchar: `script_char < 0` is TRUE for ever now, so the whole rest of the "+
			"function is what runs -- the body is dedented one level", 1); err != nil {
		return nil, err
	}

	// ---- B. redir_fd is NULL for ever -----------------------------------------
	// THE TWO PATTERNS DIFFER ONLY IN INDENTATION, and that is deliberate: a
	// single pattern with a count of two would fold two different shapes with one
	// rule.
	if text, err = fold(text, "undo_cmdmod", "never", `(?m)^        if \(redirecting\(\)\)$`,
		"undo_cmdmod: `redirecting()` is FALSE, so unwinding :silent never resets "+
			"msg_col for a redirection that is not happening", 1); err != nil {
		return nil, err
	}
	if text, err = fold(text, "redir_write", "never", `(?m)^    if \(redirecting\(\)\)$`,
		"redir_write: the same constant takes the whole body -- the fputs() and "+
			"putc() block, which is putc's only caller and fputs's only NAMED one", 1); err != nil {
		return nil, err
	}
	if k := mentions(text, "redirecting"); k != 2 {
		return nil, p.Die("redirecting has %d mentions after both callers were folded, expected 2 -- "+
			"its prototype and its definition, for the sweep", k)
	}

	// ---- the recommended extra: the no-op that is left ------------------------
	Out, err := cutil.DropIf(text, `(?m)^    if \(!did_return\)$`, 1)
	if err != nil {
		return nil, p.Die("msg_end's `if (!did_return)` block -- %v", err)
	}
	text = Out
	p.Say("msg_end's `if (!did_return)` block, which held one of the five calls: dropping " +
		"the call alone would leave an `if` with an empty body, which is not something " +
		"any tool here removes")
	for _, e := range []struct {
		Old, What string
		n         int
	}{
		{z13lit9, "emsg_core's two calls that echoed the error's source line", 2},
		{z13lit10, "emsg_core's call that echoed the message itself", 1},
		{z13lit11, "msg_puts_attr_len's call, which was every message the editor prints", 1},
		{z13lit12, "redir_write's prototype", 1},
	} {
		if text, err = textEdit(text, e.Old, "", e.What, e.n); err != nil {
			return nil, err
		}
	}
	var removed bool
	if text, removed = cutil.DeleteDefinition(text, "redir_write"); !removed {
		return nil, p.Die("redir_write has no definition to remove")
	}
	if k := mentions(text, "redir_write"); k != 0 {
		return nil, p.Die("redir_write still has %d mentions", k)
	}
	p.Say("redir_write itself, which could no longer do anything: five call sites and the " +
		"definition")

	// A local draws -Wunused-but-set-variable, which the sweep may act on; a
	// FILE-SCOPE static draws NOTHING AT ALL -- phase 93's `readonlymode` in this
	// phase's shape -- so both go here rather than being left to a tool.
	for _, e := range []struct{ Old, What string }{
		{z13lit13, "msg_end's `did_return`, written once and read never now"},
		{z13lit14, "and its one write"},
	} {
		if text, err = textEdit(text, e.Old, "", e.What, 1); err != nil {
			return nil, err
		}
	}
	if k := len(z13RedirOff.FindAll(text, -1)); k != 5 {
		return nil, p.Die("redir_off has %d writes, expected 5 -- a phase written from a description "+
			"of four would leave one behind", k)
	}
	text = z13RedirOff.ReplaceAll(text, nil)
	if text, err = textEdit(text, z13lit15, "",
		"and redir_off: FIVE writes, not four, and no reader at all -- a "+
			"file-scope static that is assigned and never read draws no warning, "+
			"and tools/deadsweep.py acts on warnings", 1); err != nil {
		return nil, err
	}
	if mentions(text, "redir_off") > 0 || mentions(text, "did_return") > 0 {
		return nil, p.Die("redir_off or did_return survives")
	}

	// ---- C. ui_write's console ------------------------------------------------
	for _, e := range []struct{ Old, New, What string }{
		{z13lit16, z13lit17, "ui_write's prototype loses the console parameter"},
		{z13lit18, z13lit19, "and the definition: `console` is FALSE at the one call site, so the " +
			"vim_fsync(1) it guarded can never be entered, and ui_write is " +
			"mch_write now -- which is what takes vim_fsync() and fsync()"},
		{z13lit20, z13lit21, "and its one call site, which already passed FALSE"},
	} {
		if text, err = textEdit(text, e.Old, e.New, e.What, 1); err != nil {
			return nil, err
		}
	}

	// ---- what the sweep is handed, as a count rather than as trust ------------
	for _, name := range edit.SortedKeys(z13After) {
		if k := mentions(text, name); k != z13After[name] {
			return nil, p.Die("%s has %d mentions after the cut, expected %d", name, k, z13After[name])
		}
	}
	p.Say("the cut is done: redirecting, closescript and vim_fsync at two mentions each " +
		"-- a prototype and a definition -- and using_script at ONE, its definition " +
		"alone, having never had a prototype; scriptin 4 and curscript 7, every one of " +
		"them inside a function the sweep now reads as unreachable; may_sync_undo and " +
		"is_safe_now SURVIVING at three; and read_cmd_fd 12, still the terminal's")
	return text, nil
}
