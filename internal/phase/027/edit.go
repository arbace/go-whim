package p027

// Whim phase 27 (formerly 87) -- no streaming Ex.  See GOAL.md.
//
// An embeddable core is driven by a host through a screen and a keyboard.  Ex mode
// is the opposite arrangement: the editor takes stdin over, prints its own prompt,
// reads a line at a time and writes the result back on stdout, and it brings a
// second mode with it -- silent mode, which redirects the whole message layer into
// `printf` and buffers stdout through setvbuf().  Both are entered from the command
// line (`-e`, `-E`, `-s`, `-v`) or from the keyboard (`Q`, `gQ`), and neither has
// any meaning for a core that is handed its input and its screen.
//
// WHAT GOES.  `do_exmode()` and `getexmodeline()`, the loop and the line reader;
// `nv_exmode()` and `nv_g_cmd`'s `case 'Q'`, the two keys that call them; the four
// command-line options; and then the two globals they were the only writers of --
// `exmode_active` (49 mentions) and `silent_mode` (23) -- which become constantly
// FALSE and fold away at every one of their readers.
//
// THE TWO COUNTS ARE ASSERTED BEFORE THE CUT AND AFTER IT, and so are the other
// nineteen identifiers that go with them.  A fold is only sound while the variable
// really is constant, so "every mention is accounted for" is the whole argument:
// 49 and 23 before, 0 and 0 after, counted by `\b` because `pending_exmode_active`
// contains `exmode_active` and a plain substring count says 53.
//
// EVERY FOLD IS COUNTED AND SCOPED TO ONE FUNCTION (tools/cutil.py), because the
// polarity is not the same at every site and a fold applied to "the first one" of
// several is a guess:
//
// * `if (exmode_active)`            folds NEVER -- the body is Ex mode's
// * `if (!exmode_active)`           folds ALWAYS -- the body is everyone else's
// * `if (exmode_active != EXMODE_NORMAL)` in msg_start() folds **ALWAYS**, because
// 0 != 1 is TRUE.  It reads like its neighbours and is their opposite, and
// getting it backwards would quietly give every message Ex mode's newline.
//
// WHAT STAYS, and the reader that forces each:
// * `getexline()` -- `:append`, `:insert` and `:change` read their lines through
// it, not through the Ex-mode reader.  It keeps `ex_at` and `nv_colon`.
// * `exe_commands()` -- it runs the `+{command}` list, which is how every harness
// here drives the editor.  Only its last statement, an `if (!exmode_active)`,
// folds.
// * everything the argv phase owns: `case NUL`'s else arm (`EDIT_STDIN`,
// `read_cmd_fd = 2`), `case '-'` and `had_minmin`, the file argument,
// `ME_TOO_MANY_ARGS` and its `main_errors[]` row, `case 'T'`, the `+cmd` arm.
// * `cmdwin` and `want_full_screen`, which lose a reader each and keep one.
//
// `-s` IS NOT IN THE DECLARED DELTA and the reason is worth stating: it was never
// an option on its own.  `case 's'` set silent mode only `if (exmode_active)` and
// called `mainerr(ME_UNKNOWN_OPTION)` otherwise, so `vim -s` already failed before
// this phase and fails in the same way after it.  `-e`, `-E`, `-e -s` and `-v` do
// move, and are declared in internal/phase/027/delta.md.
//
// THE KEYS AND THE CALLS GO BY HAND; THE FUNCTIONS THEY REACHED GO TO THE SWEEP.
// `nv_exmode`'s `nv_cmds[]` row is REPOINTED at `nv_error` and never deleted -- a
// deleted row shifts `nv_cmd_idx[]` and every key past the hole resolves to another
// key's handler (CLAUDE.md; `nvidx` is what would catch it).  Once the row, the
// `getline_equal()` test and `check_tty()`'s call are gone, `nv_exmode`,
// `do_exmode`, `getexmodeline` and `check_tty` are unreachable, and the sweep
// deletes them.
//
// SIX WRITE-ONLY LEFTOVERS lose their writes by hand, because the sweep deletes
// declarations, never statements: `ex_pressedreturn`, `ex_no_reprint` (seven
// writes), `ex_exitval`, `previous_got_int`, `use_plus_cmd` and `exmode_was`.
// Their declarations, unused once the writes are gone, the sweep takes.
//
// THE SWEEP TAKES the rest: `exmode_active`, `silent_mode`, `pending_exmode_active`,
// `s_vbuf`, `exmode_plus`, `e_at_end_of_file`, the `getexmodeline` and `nv_exmode`
// prototypes, the three single-constant enums (`EXMODE_NORMAL`, `EXMODE_VIM`,
// `BO_EX` -- each with an explicit value, so nothing renumbers), and
// `mch_input_isatty()`, with the fifth of the five `isatty()` calls.
//
// `check_tty()` IS PHASE 4E'S AS MUCH AS THIS ONE'S.  Phase 4e took its warning branch
// and kept the `if (exmode_active)` one deliberately, saying Ex mode was a later
// phase's.  This is that phase, and nothing is left -- which is why `isatty` goes
// from five calls to four here and not there.  It is deleted by name rather than
// folded, for the reason given at the site.
//
// THAT INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, exactly as internal/phase/004/e/edit.go does it: internal/phase/027/check.go requires the
// OLD binary to enter Ex mode and the new one to refuse, which is the difference
// between a probe and a formality.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
// NOT create_cmdidxs --check: the derived first-two-letters index went with
// the command table whim reduced, and there are no `ex_cmdidxs.h` banners left for it
// to find -- it raises rather than reporting nothing (internal/phase/004/e/edit.go says the
// same).  Nothing here touches the command table.
//
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  The cut is the text program's
// acts, one for one and in its order, on the program's graph through the
// verbs; its report is the text version's, line for line (history keeps
// the text program and its editlit.go):
//
//   - the anchors -- the mentions of w27Before and w27After, and the five
//     `isatty(` calls -- are asked of the C view, as the text counted them;
//   - a `line("if (X)")` fold is a fold of the ifs whose condition is X
//     that are not an else's `if`, and an `else if (X)` one of those that
//     are (FoldNeverAt's places; FoldAlwaysAt refusing a kept
//     break or continue, as the text's fold did); a `head(...)` fold names the condition's first
//     operands and `_*`;
//   - a literal that took an operand out of a condition is DropOperand, or
//     a Rewrite where the text's literal took the parentheses with it;
//   - the literals that deleted statements are Cuts of their forms, the
//     text's indentation (a function's own items, or one level in) said
//     by where the item stands;
//   - main_loop's `noexmode` is PARAM's: its prototype, its definition and
//     its one call, one edit, reported as the text's three literals.

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim27", Edit) }

var w27Before = map[string]int{
	// do_exedit's exmode handling went with :edit, :ex and :visual at phase 1
	// (filefront, the reform's D4): exmode_was was its local; readfile's
	// test and its write of ex_no_reprint at phase 1 (readfront, phase 31's move);
	// one exmode_active and one EXMODE_NORMAL with the windows at phase 1
	// (nowindows, the reform's D9), and pending_exmode_active, whose only
	// writes were do_exedit's, falls out there since D9
	"exmode_active": 38, "silent_mode": 21, "pending_exmode_active": 0,
	"exmode_plus": 3, "exmode_was": 0, "do_exmode": 4, "getexmodeline": 6,
	"nv_exmode": 3, "EXMODE_NORMAL": 4, "EXMODE_VIM": 3, "BO_EX": 2,
	"ex_pressedreturn": 6, "ex_no_reprint": 8, "ex_exitval": 3,
	"previous_got_int": 4, "use_plus_cmd": 5, "s_vbuf": 4,
	"e_at_end_of_file": 2, "noexmode": 4, "check_tty": 2,
	"mch_input_isatty": 2,
}

// w27After: every use is gone but the definitions and what the four unreachable
// functions (nv_exmode, do_exmode, getexmodeline, check_tty) still say, each of
// them a kind the sweep deletes.  Stated as a number per name, so a use that
// survived shows up HERE and not after the sweep.
// (exmode_active 6 and exmode_was 0: do_exedit went with :edit at phase 1, D4)
var w27After = map[string]int{
	"exmode_active": 6, "silent_mode": 2, "pending_exmode_active": 0,
	"exmode_plus": 1, "exmode_was": 0, "do_exmode": 2, "getexmodeline": 3,
	"nv_exmode": 2, "EXMODE_NORMAL": 2, "EXMODE_VIM": 2, "BO_EX": 2,
	"ex_pressedreturn": 4, "ex_no_reprint": 4, "ex_exitval": 1,
	"previous_got_int": 1, "use_plus_cmd": 1, "s_vbuf": 1,
	"e_at_end_of_file": 2, "noexmode": 0, "check_tty": 1,
	"mch_input_isatty": 2,
}

var w27Isatty = regexp.MustCompile(`\bisatty\(`)

func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noexmode", e, w)

	t := v.Text()
	for _, name := range edit.SortedKeys(w27Before) {
		if k := edit.MentionCount(t, name); k != w27Before[name] {
			v.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w27Before[name])
			return v.Done()
		}
	}
	if k := len(w27Isatty.FindAll(t, -1)); k != 5 {
		v.Die("isatty is called %d times, expected 5", k)
		return v.Done()
	}
	v.Say("21 identifiers at their counted mentions: exmode_active 49, silent_mode 23, isatty 5 calls")

	v.InTable("nv_cmds", func(v *graph.Verbs) {
		v.RewriteAt("(init 'Q' ?f NV_NCW 0)", "f", "nv_error", 1,
			"the 'Q' row points at nv_error, so Q beeps like any unused key")
	})
	v.InFunction("nv_g_cmd", func(v *graph.Verbs) {
		v.DropCase("(case 'Q')", 1, "gQ's arm of nv_g_cmd, which falls to default: clearopbeep")
	})
	v.Rewrite("(|| (call getline_equal fgetline cookie getexmodeline) ?b)", "?b", 1,
		"do_cmdline stops asking whether it is reading Ex-mode lines")

	in := func(fn string, acts func(v *graph.Verbs)) { v.InFunction(fn, acts) }
	in("do_ecmd", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "exmode_active", 1, "do_ecmd no longer puts the cursor on the last line")
	})
	in("ex_substitute", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "exmode_active", 1, "ex_substitute keeps its 85-line interactive else")
	})
	in("do_one_cmd", func(v *graph.Verbs) {
		v.DropOperand("exmode_active", 1, "do_one_cmd asks only whether it is sourcing")
	})
	in("ex_range_without_command", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "(&& exmode_active (!= (-> eap cmd) (+ (cast (ptr char_u) exmode_plus) 1)))", 1,
			"a bare range is no longer an implicit :print")
	})
	in("parse_command_modifiers", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "(&& (== (deref (-> eap cmd)) NUL) exmode_active _*)", 1,
			"an empty Ex-mode line is no longer the + command")
	})
	in("cmdline_erase_chars", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "exmode_active", 1, "backspacing off the start of a command line leaves Normal mode again")
	})
	in("getcmdline_int", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "(&& exmode_active (!= c ESC) _*)", 1, "a trailing backslash no longer continues a command line")
		v.FoldNeverAt(graph.IfNotArm, "(&& exmode_active (|| (== ex_normal_busy 0) (> (. typebuf tb_len) 0)))", 1,
			"and ESC on a command line is an abort again")
	})
	in("compute_cmdrow", func(v *graph.Verbs) {
		v.Rewrite("(|| exmode_active (paren ?r))", "?r", 1, "compute_cmdrow asks only about the scroll")
	})
	in("vgetorpeek", func(v *graph.Verbs) {
		v.DropOperand("(! exmode_active)", 1, "and the partial command is shown whenever there is one")
	})
	in("msg_strtrunc", func(v *graph.Verbs) {
		v.DropOperand("(! exmode_active)", 1, "msg_strtrunc truncates as it does on a screen")
	})
	in("msg_may_trunc", func(v *graph.Verbs) {
		v.Rewrite("(paren (&& (call shortmess SHM_TRUNC) (! exmode_active)))", "(call shortmess SHM_TRUNC)", 1,
			"and so does msg_may_trunc")
	})
	in("wait_return", func(v *graph.Verbs) {
		v.FoldAlwaysAt("(! exmode_active)", 2, true, "wait_return moves the command row, both times")
		v.FoldNeverAt(graph.IfArm, "exmode_active", 1, "and no longer answers its own prompt with a space")
	})
	in("msg_start", func(v *graph.Verbs) {
		v.FoldAlwaysAt("(!= exmode_active EXMODE_NORMAL)", 1, true,
			"msg_start keeps the cmdline_row it set after a newline (0 != 1 is TRUE)")
	})
	in("msg_puts_display", func(v *graph.Verbs) {
		v.Rewrite("(&& (> cmdline_row 0) (! exmode_active))", "(> cmdline_row 0)", 1, "msg_puts_display scrolls the command row")
		v.DropOperand("(! exmode_active)", 1, "and the more-prompt is offered whenever the screen is full")
	})
	in("screen_puts_len", func(v *graph.Verbs) {
		v.DropOperand("exmode_active", 1, "a character is redrawn when it changed, and not otherwise")
	})
	in("set_shellsize_inner", func(v *graph.Verbs) {
		v.DropOperand("exmode_active", 1, "a resize repeats the message only at a prompt")
	})
	in("vim_main2", func(v *graph.Verbs) {
		v.FoldAlwaysAt("(! exmode_active)", 1, true, "vim_main2 stops scrolling messages at startup")
		v.FoldNeverAt(graph.IfNotArm, "exmode_active", 2, "and clears the screen, and leaves the cursor where the file put it")
	})
	in("main_loop", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "(&& noexmode global_busy (! exmode_active) previous_got_int)", 1,
			"CTRL-C in a :global no longer drops into Ex mode")
		v.FoldAlwaysAt("(|| (! global_busy) (! exmode_active))", 1, true,
			"and swallows the interrupt as it always did outside :global")
		v.FoldAlwaysAt("(! exmode_active)", 1, true, "the main loop stops scrolling messages")
		v.DropOperand("exmode_active", 1, "and redraws unless the last command asked it not to")
		v.FoldNeverAt(graph.IfNotArm, "exmode_active", 1, "and runs normal_cmd, which is now the only thing it can run")
	})
	in("getout", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "exmode_active", 1, "the exit status is the one getout was given")
	})
	in("main", func(v *graph.Verbs) { v.Cut("(call check_tty)", 1, "and its call in main") })
	in("exe_commands", func(v *graph.Verbs) {
		v.FoldAlwaysAt("(! exmode_active)", 1, true, "exe_commands stops scrolling messages, and keeps running +cmd")
	})

	in("change_warning", func(v *graph.Verbs) {
		v.DropOperand("(! silent_mode)", 1, "the 'readonly' warning pauses whenever it is shown")
	})
	in("print_line", func(v *graph.Verbs) {
		v.Cut("(def save_silent int silent_mode)", 1, "print_line prints on the screen, once")
		v.CutWhere("(= silent_mode FALSE)", nil, 1, "")
		v.FoldNeverAt(graph.IfNotArm, "save_silent", 1, "and no longer flushes a line it never buffered")
	})
	quiet := "(! (&& silent_mode (== p_verbose 0)))"
	in("msg_puts_printf", func(v *graph.Verbs) {
		v.FoldAlwaysAt(quiet, 1, true, "msg_puts_printf writes what it is given")
		v.DropOperand(quiet, 1, "including the last piece")
	})
	in("do_set", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "(&& silent_mode did_show)", 1, ":set prints its listing on the screen")
	})
	in("showoneopt", func(v *graph.Verbs) {
		v.Cut("(def save_silent int silent_mode)", 1, "and so does one option")
		v.CutWhere("(= silent_mode FALSE)", nil, 1, "")
		v.CutWhere("(= silent_mode save_silent)", nil, 1, "")
	})
	in("exit_scroll", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "silent_mode", 1, "exiting scrolls the screen as it does from Normal mode")
	})
	in("typed_ahead", func(v *graph.Verbs) {
		v.Rewrite("(paren (&& (! silent_mode) ?c))", "?c", 1,
			"typed_ahead is char_avail() again -- slim's 'lazyredraw' fix had only one caller")
	})
	in("set_termname", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "silent_mode", 1, "a terminal is set up whenever one is named")
	})
	in("ui_write", func(v *graph.Verbs) { v.FoldAlwaysAt(quiet, 1, true, "ui_write writes") })
	in("read_error_exit", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "silent_mode", 1, "a read error is reported before the exit, not instead of it")
	})
	in("main", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "silent_mode", 1, "main stops buffering stdout: setvbuf and stdout leave the binary")
		v.DropOperand("(! silent_mode)", 1, "and sets up the terminal when it is asked to")
	})

	in("parse_command_modifiers", func(v *graph.Verbs) {
		v.Cut("(= ex_pressedreturn TRUE)", 1, "and its one remaining write")
	})
	// the text's two literals told the writes by their indentation: a
	// function's own item, or one level in
	v.CutWhere("(= ex_no_reprint TRUE)", func(x *graph.Node) bool { return e.Parent(x).Is("defn") }, 3, "and its five writes")
	v.CutWhere("(= ex_no_reprint TRUE)", nil, 1, "and the fifth")
	in("emsg_core", func(v *graph.Verbs) { v.Cut("(= ex_exitval 1)", 1, "and its one write") })
	in("main_loop", func(v *graph.Verbs) {
		v.Cut("(= previous_got_int TRUE)", 1, "previous_got_int's writes: only the Ex-mode arm read it")
		v.CutWhere("(block (= previous_got_int FALSE))", nil, 1, "")
	})
	in("parse_command_modifiers", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "use_plus_cmd", 2, "use_plus_cmd: the two arms of the :visual range rewrite")
		v.FoldNeverAt(graph.IfArm, "use_plus_cmd", 1, "and the + command it stood for")
	})

	if !v.Failed() {
		if _, err := e.DropParam("main_loop", "noexmode", graph.ParamOptions{}); err != nil {
			v.Die("main_loop's parameter -- %v", err)
		}
	}
	v.Say("main_loop's prototype")
	v.Say("its definition")
	v.Say("and its one caller")
	in("main_loop", func(v *graph.Verbs) {
		v.Cut("(label theend)", 1, "the theend: label, which nothing jumps to now")
	})
	if v.Failed() {
		return v.Done()
	}

	t = v.Text()
	for _, name := range edit.SortedKeys(w27After) {
		k := edit.MentionCount(t, name)
		if k != w27After[name] {
			why := "more went than was meant to"
			if k > w27After[name] {
				why = "a use survived"
			}
			v.Die("%s has %d mentions after the cut, expected %d -- %s", name, k, w27After[name], why)
			return v.Done()
		}
	}
	v.Say("every use of all 21 is gone; the lone definitions, the four unreachable " +
		"functions and mch_input_isatty are what the collection takes")
	return v.Done()
}
