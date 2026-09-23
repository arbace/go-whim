package p085

// Whim phase 85 -- the core stops diagnosing its own terminal.  See GOAL.md.
//
// An embeddable core is handed its input and output by a host.  Whether either of
// them is a terminal is the host's business, and vim's answer to it is to print two
// warnings on stderr and then stop for two seconds so a user can read them:
//
// Vim: Warning: Output is not to a terminal
// Vim: Warning: Input is not from a terminal
//
// WHAT GOES.  check_tty()'s second branch entirely -- both warnings, the out_flush()
// that pushes them, the exit(1) that --ttyfail asked for, and the ui_delay(2005L)
// under `scriptin[0] == NULL` that exists only so the warnings can be read -- and
// with it the --ttyfail flag: its `case '-'` test, the `parmp->tty_fail = TRUE` it
// set, and the tty_fail field of mparm_T.  `--ttyfail` becomes what any other
// unknown word after `--` is, ME_UNKNOWN_OPTION through mainerr().
//
// THE PAUSE WAS LOOKED FOR RATHER THAN ASSUMED.  There are nine ui_delay() call
// sites in the file and exactly one is the warnings': 2005L, inside the branch that
// printed them.  The other eight are a different pause each -- 3001L for the "W14:
// List of file names overflow" message, 1002L for the 'readonly' warning, the
// wait_now loop inside ui_delay's own caller, 1003L and 3003L in wait_return(),
// 1006L in check_for_delay(), and p_mat's two in showmatch() -- and none of them is
// reached from check_tty().  The `scriptin[0] == NULL` condition it sat under is
// upstream's "do not pause while a script is being read"; with no pause left there
// is nothing for it to condition.
//
// WHAT STAYS, and the reader that forces each:
// * the exmode_active branch -- `if (!input_isatty) silent_mode = TRUE` -- because
// Ex mode is removed in a later phase and not this one.  It is also the reason
// mch_input_isatty() keeps a caller.
// * stdout_isatty, which is read outside main by `out_redir = !stdout_isatty` in
// the message layer, and so keeps mch_check_win() -- its one assignment -- and
// mch_check_win()'s isatty(1) alive.  Folding those is not this phase's job.
// * want_full_screen, whose second reader (`params.want_full_screen &&
// !silent_mode` in main) survives the branch that went.
// So all five isatty() calls remain, and the undefined symbol count does not move.
// This phase is the warnings, the pause and the flag -- not every isatty caller.
//
// check_tty() then reads nothing from its argument, so it takes void and its one
// caller drops the `&params`.  The alternative is __attribute__((unused)) on a
// parameter nothing will ever read again, which is how slim MARKS an unused
// parameter and how whim 67 got rid of one.
//
// THE DELTA IS NONE, AND THE HARNESSES ARE BLIND TO THIS PHASE -- which is why the
// check runs probes of its own.  behaviour.py and exsweep.py run the editor `-e -s`,
// where exmode_active is set before check_tty() and the first branch takes it, so
// the warnings were never on any recorded stderr (grepped: no baseline mentions a
// terminal outside two "SKIPPED (hands over the terminal)" rows).  termcheck.py
// drives a real pty, where stdin and stdout are both terminals and the branch is
// not entered at all.  A delta of "none" from a harness that cannot see the code is
// not evidence, so phase/085/check.go measures the removed behaviour directly, in
// both directions, against the binary this phase was handed.
//
// THAT BINARY IS BUILT HERE, before the edit, from the boundary's own makefile
// flags -- the same shape as whim phases 80 and 81, which keep an `old` binary in
// the state directory for their checks to compare against.  It is what makes the
// probes able to fail: the check requires the OLD binary to warn and to pause and
// the new one to do neither.
// The input binary, for the check's before-and-after.  The flags are read out of the
// boundary's makefile rather than written here a second time: the core's compile line is
// the boundary's (GOALS.md core rule 8), and a copy of it in this file would be a
// second statement of it that could drift.
// NOT create_cmdidxs --check, which every whim edit of the command table
// runs: the derived first-two-letters index went with the table whim reduced, and
// there are no `ex_cmdidxs.h` banners left in whim-vim.c for it to find -- it raises
// rather than reporting nothing.  Nothing here touches the command table anyway.
//
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim85", Edit) }

// Whim85 stops the core diagnosing its own terminal.
//
// An embeddable core is handed its input and output by a host.  Whether either
// is a terminal is the host's business, and vim's answer to it is to print two
// warnings on stderr and then stop for two seconds so a user can read them.
//
// WHAT GOES: check_tty()'s second branch entirely -- both warnings, the
// out_flush() that pushes them, the exit(1) that --ttyfail asked for, and the
// ui_delay(2005L) under `scriptin[0] == NULL` that exists only so the warnings
// can be read -- and with it the --ttyfail flag itself.
//
// WHAT STAYS, each with the reader that forces it: the exmode_active branch,
// because Ex mode goes in a later phase and not this one; stdout_isatty, read
// outside main by `out_redir = !stdout_isatty`; and want_full_screen, whose
// second reader survives the branch that went.  So all five isatty() calls
// remain and the undefined symbol count does not move.  This phase is the
// warnings, the pause and the flag -- not every isatty caller.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "nottywarn", W: w}
	var err error

	// ---- 0. the invariants the cut rests on ------------------------------
	// Asserted rather than trusted from the survey: if an upstream gives
	// tty_fail a second reader, or moves the pause Out of the branch, this
	// fails loudly instead of taking a decision that is no longer the one
	// written down.
	for _, inv := range []struct {
		Name string
		want int
	}{{"tty_fail", 3}, {"ui_delay(2005L", 1}} {
		if err = p.Count(text, regexp.QuoteMeta(inv.Name), inv.want, inv.Name); err != nil {
			return nil, err
		}
	}
	if err = p.Count(text, `\bisatty\(`, 5, "isatty"); err != nil {
		return nil, err
	}
	p.Say("tty_fail has its field, its one write and its one read; the 2005 ms pause is the only one")

	// ---- 1. the warnings, the pause and the exit -------------------------
	// One `else if` in a chain, so foldNever takes the whole branch and
	// leaves the exmode_active one before it.
	text, err = p.FoldNever(text, "check_tty",
		`^[ \t]*else if \(parmp->want_full_screen && \(!stdout_isatty \|\| !input_isatty\)\)$`,
		"both warnings, the flush, the --ttyfail exit and the two-second pause", 1)
	if err != nil {
		return nil, err
	}

	// ---- 2. the flag that asked for the exit -----------------------------
	// The test can no longer be true -- there is no --ttyfail -- so foldNever
	// keeps what it chose between: the else branch, where an unrecognised
	// word after `--` is ME_UNKNOWN_OPTION and a bare `--` sets had_minmin.
	text, err = p.FoldNever(text, "command_line_scan",
		`^[ \t]*if \(strncasecmp\(\(char \*\)\(argv\[0\] \+ argv_idx\), \(char \*\)\("ttyfail"\), \(7\)\) == 0\)$`,
		"--ttyfail, which is now an unknown option like any other", 1)
	if err != nil {
		return nil, err
	}
	text, err = p.Literal(text, "    int tty_fail;\n", "", "the mparm_T field it set", 1)
	if err != nil {
		return nil, err
	}

	// ---- 3. the argument check_tty no longer reads -----------------------
	// The alternative is __attribute__((unused)) on a parameter nothing will
	// ever read again, which is how slim MARKS an unused parameter and how
	// whim 67 got rid of one.
	text, err = p.Literal(text, "check_tty(mparm_T *parmp)\n", "check_tty(void)\n",
		"check_tty takes nothing: its only use of parmp went with the branch", 1)
	if err != nil {
		return nil, err
	}
	text, err = p.Literal(text, "    check_tty(&params);\n", "    check_tty();\n",
		"and its one caller", 1)
	if err != nil {
		return nil, err
	}

	if err = p.Gone(text, "tty_fail", "ttyfail", "not to a terminal",
		"not from a terminal", "ui_delay(2005L"); err != nil {
		return nil, err
	}
	return text, nil
}
