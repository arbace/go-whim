package p103

// Whim phase 103 -- the signals and the terminal are the host's.  See GOAL.md.
//
// GOALS.md II.4c's third step, and the largest of the three: `mch_get_shellsize`,
// `mch_settmode` and the signal handlers move across, "which takes ioctl, tcgetattr,
// tcsetattr, select, nanosleep and the nine signal symbols with them".  It takes none
// of them, and the phase says so rather than implying a reduction it does not make.
//
// WITHIN ONE TRANSLATION UNIT, MOVING CODE FROM THE CORE TO THE HOST FREES NOTHING.
// A symbol leaves `nm -u` when its last CALLER leaves the file, and that is the split.
// So `sigaction`, `sigemptyset`, `kill`, `ioctl`, `tcgetattr`, `tcsetattr`,
// `nanosleep` and `select` all survive this phase -- every one of them now called
// from the 240-line host block at the bottom of the file and from nowhere else.
// WHAT LEAVES IS WHAT THE PHASE DELETES: `raise sigaddset sigismember sigprocmask`,
// which were `mch_signal()`'s fifty lines of sigset() emulation and `sig_tstp`'s
// re-raise, and `close dup isatty`, which were the two dead descriptor sites and the
// three questions about terminals that nothing asks any more.  31 -> 24.
//
// THE PHASE'S VALUE IS STRUCTURAL, AND ``zhostonly`` IS THAT CLAIM AS AN
// ASSERTION: every mention of the host's vocabulary -- 43 words, from `sigaction` to
// `VMIN` -- is inside the host block, with eight named exceptions in the core that are
// the deadly-signal message and the clock.  Without that tool the strongest available
// form of "the core names none of this" is `sigaction` at 2 mentions, which says
// nothing about where.
//
// ------------------------------------------------------------------------------------
// WHAT THE HOST TELLS THE EDITOR, AND WHY IT IS IN BAND
//
// The editor had five signal handlers.  Four of them existed to tell it something:
// SIGWINCH "you were resized", SIGTSTP "you were asked to stop", SIGCONT "you were
// continued", SIGINT "the user interrupted you".  A host cannot deliver a signal to a
// core it is linked into -- there is no such thing -- so the four become BYTES the
// core reads from its input, which is the one channel that already exists.  The fifth,
// the deadly pair SIGHUP/SIGTERM, is not a message: it is the process ending, and it
// stays (below).
//
// THE WIRE FORMAT FOR THE RESIZE IS NOT OURS, AND IT WAS ALREADY IN THE FILE.
// `handle_csi()` has always parsed DEC private mode 2048's notification
// (`CSI 48 ; rows ; cols ; hpx ; wpx t`, Tim Culverhouse, 2024) -- and the arm could
// never run, because `\033[?2048$p`, the DECRQM query a terminal answers to set
// `win_resize_setting`, appears NOWHERE in slim-vim.c, whim-vim.c or whim-vim.c.  So
// the whole negotiation was dead code guarding a working parser.  This phase deletes
// the negotiation, the 'termresize' option that drove it and the two state variables,
// and makes the notification unconditional -- and the HOST writes it, from its own
// SIGWINCH handler and its own ioctl.
//
// WHY THE HOST KEEPS THE ioctl RATHER THAN RELYING ON THE TERMINAL.  Mode 2048 is
// implemented by ghostty, kitty, foot, iTerm2, contour and Bobcat, and NOT by xterm,
// tmux, GNU screen, WezTerm, Alacritty, VTE, Konsole, st, Windows Terminal, urxvt or
// Zellij -- and tmux and screen TERMINATE it, answering DECRQM with the correct "not
// recognised" and forwarding nothing.  A core that relied on the terminal would be an
// editor that never learns it was resized under tmux.  Deleting the host's
// `TIOCGWINSZ` instead would free `ioctl` and an eleventh #include and cost that,
// which is a number bought with the terminal.
//
// `\033[?1z` IS OURS AND IS IN NO SPEC, and that is said here rather than discovered.
// `first == '?' && argc == 1 && arg[0] == 1 && trail == 'z'` reaches no existing arm
// (the `?`-prefixed arms end in `c`, `y` and `u`), and `z` is a letter so the trail
// scan stops on it.  Doing real work from inside the termcode parser has precedent in
// the file: the resize arm calls `set_shellsize()`, which redraws the whole screen.
//
// THE INTERRUPT TRAVELS AS THE BYTE IT ALREADY IS.  `catch_sigint` set `got_int`
// directly; the host instead hands the core a `0x03`, which `fill_input_buf`'s own
// CTRL-C scan turns into `got_int` -- the only way `got_int` is ever set in raw mode,
// because raw mode clears ISIG.  IT IS NOT OPTIONAL: with no handler at all, SIG_DFL
// KILLS the editor, and the `10gs` probe found it -- CTRL-C during a `gs` sleep raises
// SIGINT, because the sleep mode deliberately leaves ISIG on (below).  Measured on the
// binary this phase was handed and on a build with no SIGINT handler: `kill -INT`
// mid-session leaves the first editing and kills the second with SIG2.
//
// CTRL-Z IS THE ONE THING THAT CANNOT GO IN BAND, and `musl_suspend()` is why there is
// one call out and not zero.  In raw mode ISIG is clear, so the keyboard CTRL-Z
// arrives as the byte 0x1a, reaches `nv_cmds[]`'s real `{Ctrl_Z, nv_suspend, 0, 0}`
// row, and THE CORE DECIDES to suspend.  A one-way host->core channel has no way to
// carry that decision back.  The external `kill -TSTP` is the other direction and fits
// the in-band path exactly.
//
// ------------------------------------------------------------------------------------
// THE TERMINAL: TWO OPERATIONS, NOT A MODE SETTER
//
// `settmode(tmode_T)` is nine call sites and a three-valued mode, and every site is
// attached to an OPERATION: the editor takes the terminal, gives it back, suspends,
// resumes, sleeps, or re-asserts that it still holds it.  Exposing `musl_set_raw(int)`
// would move the syscall without moving the responsibility, so `settmode()` splits at
// the one line that was ever the host's -- `mch_settmode(tmode)` -- into `term_enter()`
// and `term_leave()`, which keep the escape sequences (`t_BE`/`t_TI`, `t_CBD`/`t_TE`)
// because those are screen work.  `cur_tmode` becomes a boolean, `mch_cur_tmode` goes
// (the two were measured equal at all eleven sites), and `tmode_T` with `TMODE_SLEEP`
// goes to the sweep.
//
// `TMODE_SLEEP` IS NOT "DISCARD INPUT" AND `musl_delay`'s PARAMETER SAYS SO.  Measured
// from `mch_settmode`: the sleep mode is the SAVED termios with ICANON and ECHO
// cleared, applied TCSANOW and not TCSAFLUSH -- nothing is flushed and nothing is
// discarded.  What is different from raw mode is that ISIG is left ON, so the INTR
// character raises SIGINT and cuts the `nanosleep` short.  `10gs` then CTRL-C recovers
// in about 1.8 s on the binary this phase was handed and on this one, and NEVER
// recovers (measured to 26 s) on a build of this phase's own output with the two
// `host_tty_set` calls deleted.  The parameter is `interruptible`.
//
// NOTHING ASKS WHETHER THIS IS A TERMINAL, which is GOALS.md II.1's decision 7 --
// "the check goes entirely" -- finally kept: phases 85 and 87 left three `isatty()`
// calls alive and this takes all three.  `mch_check_win` and `stdout_isatty` go, and
// `nv_esc`'s `out_redir` folds to the terminal arm.  `musl_is_terminal()` is NOT
// written: the two questions had different subjects (fd 1 for the size, fd 0 for the
// input) and neither survives its caller, so a host call to answer a question nobody
// asks afterwards is boundary surface for nothing.
//
// THE CORE CANNOT ACQUIRE A DESCRIPTOR AT ALL ANY MORE.  `fill_input_buf`'s
// `close(0); vim_ignored = dup(2);` arm -- the core reopening its own stdin from its
// stderr when stdin hit EOF and was not a terminal -- is the core second-guessing the
// host about where input comes from, and once the host owns the terminal that is the
// host's business.  GOALS.md II.4b's invariant becomes absolute: the core is handed
// fds 0, 1 and 2 and that is the whole of it.  The behaviour that goes is real and
// probe-only: with stdin at EOF and a TERMINAL on fd 2 the old binary reopens fd 0 and
// carries on editing (2,016 bytes drawn), where this one prints `Vim: Finished.` and
// exits (168 bytes).  Zero of 253 recorded rows reach it.
//
// ------------------------------------------------------------------------------------
// WHAT STAYS IN THE CORE, AND EACH IS A DECISION
//
// `deathtrap` STAYS AND THE HOST INSTALLS IT, which is the whole reason the signal
// phase and the terminal phase were merged into one.  A host-side deadly handler that
// merely died would leave the terminal RAW -- measured on a prototype that deleted
// `deathtrap`: the process is killed by the signal, no message, tty still raw.  Keeping
// the body reachable costs nothing and keeps all of it: `deathtrap -> preserve_exit ->
// prepare_to_exit -> term_leave()` restores the tty, writes `Vim: Caught deadly signal
// TERM`, and ends through phase 102's `vim_host_exit` -> `__builtin_longjmp` -> `return
// 1`.  MEASURED identical on both binaries, to the byte: 2,241 on SIGTERM and 2,240 on
// SIGHUP, tty back to ICANON=1 ECHO=1 ISIG=1 ONLCR=1 ICRNL=1, exit 1.
//
// `vim_handle_signal` STAYS, and it is the one core mention of any host word that is
// not a message: `kill(getpid(), got_signal)`, re-raising a deadly signal that arrived
// while the editor was not reading.  Deleting it would make a deadly signal act in the
// middle of a screen update, which no recording can see.  ``zhostonly`` names
// it as an exception with that reason rather than loosening its pattern.
//
// `ui_get_shellsize()` STAYS A QUERY, and this is the design the survey got wrong.
// Making it "do I know my size?" -- a `shell_size_known` flag set by the host and by
// every notification -- BREAKS RESIZING, and the measurement is exact: at a
// `Press ENTER` prompt `set_shellsize()` does `State = MODE_SETWSIZE; return;` and
// DISCARDS the width and height it was given; `wait_return()` then calls
// `shell_resized()`, which is `set_shellsize(0, 0, FALSE)`, which learns the new size
// ONLY from `ui_get_shellsize()`'s side effect.  With the flag, a pty resized while
// the editor sits at a Press-ENTER prompt stays 24x80 for ever (measured, twice).  So
// `mch_get_shellsize()`'s body moves to the host as `musl_get_winsize(int *, int *)` --
// which is the name GOALS.md II.4c gave it -- and `ui_get_shellsize()` keeps its
// shape.  That also disposes of the `set_termname()` trap the survey spent an hour on:
// on a pipe the host's ioctl fails, `ui_get_shellsize()` returns FAIL exactly as
// before, `t_CWS` is still emitted and the recording does not move by one byte.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"fmt"
	"github.com/arbace/go-whim/internal/cutil"
	"io"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim103", Edit) }

// w103Before is counted on the INPUT, so a later phase that moved one of these
// fails here and not in the middle of a cut.
var w103Before = []struct {
	Name string
	want int
}{
	{"mch_signal", 18}, {"signal_info", 9}, {"settmode", 13},
	{"mch_settmode", 2}, {"isatty", 4}, {"deathtrap", 3},
	{"win_resize_enabled", 6}, {"got_tstp", 4}, {"do_resize", 6},
}

// Whim103 gives the signals and the terminal to the host: the five signal
// handlers, mch_settmode's three-valued mode, mch_delay's sleep,
// RealWaitForChar's select, mch_get_shellsize and all three isatty() calls move
// into a 229-line host block at the bottom of the same file.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 168 drops the unused)
	p := edit.Ph{Tag: "host", W: w}
	t := string(text)

	mentions := func(name string) int { return edit.MentionCount([]byte(t), name) }
	blankRuns := func(s string) int {
		L := strings.Split(s, "\n")
		n := 0
		for i := 1; i < len(L); i++ {
			if L[i] == "" && L[i-1] == "" {
				n++
			}
		}
		return n
	}
	sub := func(old, new string, n int, tag string) error {
		c := cutil.CountAnchor(t, old)
		if c != n {
			return p.Die("%s: `%s` occurs %d times, expected %d",
				tag, edit.CoreHead(strings.Split(strings.TrimSpace(old), "\n")[0], 70), c, n)
		}
		t = cutil.ReplaceAnchor(t, old, new, -1)
		return nil
	}
	// delfunc deletes a whole definition by brace matching from its name line.
	delfunc := func(sigline, tag string) error {
		if c := strings.Count(t, sigline); c != 1 {
			return p.Die("%s: the definition line `%s` occurs %d times, expected 1",
				tag, edit.CoreHead(strings.TrimSpace(sigline), 60), c)
		}
		i := strings.Index(t, sigline)
		k := strings.Index(t[i:], "{") + i
		d := 0
		for {
			if t[k] == '{' {
				d++
			} else if t[k] == '}' {
				d--
				if d == 0 {
					break
				}
			}
			k++
		}
		a := strings.LastIndex(t[:i], "\n")
		st := strings.LastIndex(t[:a], "\n") + 1
		t = t[:st] + t[strings.Index(t[k:], "\n")+k+1:]
		return nil
	}

	linesBefore := len(strings.Split(t, "\n"))

	// ---- 0. this is the file the phase was written against -------------------
	for _, b := range w103Before {
		if k := mentions(b.Name); k != b.want {
			return nil, p.Die("the input has %d mentions of `%s`, expected %d -- this is not the tree "+
				"this phase was written against", k, b.Name, b.want)
		}
	}
	if strings.Contains(t, `\033[?2048$p`) || strings.Contains(t, "2048$p") {
		return nil, p.Die("the DECRQM query for mode 2048 is in the file, so the negotiation this phase " +
			"deletes as dead code is not dead")
	}
	// The report line for this section is the FIRST ROW OF THE TABLE and is not
	// written here: every say() the phase makes is lifted with its acts, and a
	// second copy printed by hand is a duplicated line the tree cannot see.
	// Measured -- it printed twice.

	// ---- the acts, in the order the phase performs them ----------------------
	// TWO ACTS ARE NOT IN THE TABLE, and missing the second one is what the
	// first run of this port did.  Both are index splices rather than sub()
	// calls -- one deletes from get_tty_fd()'s head to get_stty()'s, the other
	// REPLACES RealWaitForChar()'s whole definition with a three-line one -- so
	// neither has text to count and neither is a call the generator can lift.
	// Each is performed by the TAG of the act that follows it and never by a
	// line number, because a table keyed on line numbers is a dependency on
	// every line above it (CLAUDE.md, `apart 107 108`).
	//
	// The way the omission showed was one mention short at the very end:
	// `musl_wait_for_input has 2 mentions, expected 3 -- its prototype, its
	// definition and RealWaitForChar()'s one call`.  The prototype and the
	// definition had landed; the call had not, because the function that makes
	// it had never been rewritten.
	for _, op := range w103Ops {
		if len(op.Args) > 0 {
			switch op.Args[len(op.Args)-1] {
			case "K2":
				i := strings.Index(t, "    static int\nget_tty_fd(int fd)")
				j := strings.Index(t, "    static void\nget_stty(void)")
				if i < 0 || j < 0 || i >= j {
					return nil, p.Die("K1: get_tty_fd() and get_stty() are not the adjacent pair this phase " +
						"cuts between")
				}
				t = t[:i] + t[j:]
			case "T1":
				i := strings.Index(t, "RealWaitForChar(int fd, long msec, int *check_for_gpm")
				j := strings.Index(t, "    static int\nno_Magic(int x)")
				if i < 0 || j < 0 || i >= j {
					return nil, p.Die("K5: RealWaitForChar() and no_Magic() are not the adjacent pair " +
						"this phase replaces between")
				}
				t = t[:i] + w103RealWait + t[j:]
			}
		}
		switch op.kind {
		case "say":
			// The LAST say is computed by the phase and carries no literal, so
			// it is written after the loop with the line counts it reports.
			if op.Args != nil {
				p.Say(op.Args[0])
			}
		case "cut", "sub":
			// cut(old, n, tag) is sub(old, "", n, tag); the defaults are n=1 and
			// an empty tag, and the table carries only the arguments the phase
			// actually passed.
			a := op.Args
			if a == nil {
				// The computed splice: the host block goes INSIDE the launcher
				// region phase 101 created and phase 102 filled, immediately above
				// `host_jump`, because that region is what becomes the second
				// file at the split.
				old := "\nstatic void *host_jump[5];\nstatic int host_code;\n"
				if err := sub(old, "\n"+w103Host+"static void *host_jump[5];\nstatic int host_code;\n",
					1, "H3"); err != nil {
					return nil, err
				}
				continue
			}
			old, new, rest := a[0], "", a[1:]
			if op.kind == "sub" {
				new, rest = a[1], a[2:]
			}
			n, tag := 1, ""
			if len(rest) > 0 {
				n, _ = strconv.Atoi(rest[0])
			}
			if len(rest) > 1 {
				tag = rest[1]
			}
			if err := sub(old, new, n, tag); err != nil {
				return nil, err
			}
		case "delfunc":
			tag := ""
			if len(op.Args) > 1 {
				tag = op.Args[1]
			}
			if err := delfunc(op.Args[0], tag); err != nil {
				return nil, err
			}
		}
	}

	// ---- what the file is now ------------------------------------------------
	var left []string
	for _, n := range w103Gone {
		if mentions(n) > 0 {
			left = append(left, n)
		}
	}
	if len(left) > 0 {
		return nil, p.Die("these names should be at 0 mentions and are not: %s", strings.Join(left, " "))
	}
	for _, r := range w103Want {
		if k := mentions(r.Name); k != r.want {
			return nil, p.Die("`%s` has %d mentions, expected %d -- %s", r.Name, k, r.want, r.why)
		}
	}
	n := 0
	for _, l := range strings.Split(t, "\n") {
		if strings.HasPrefix(l, "#") {
			n++
		}
	}
	if n != nInc {
		return nil, p.Die("the twelve #include directives moved, and this phase adds and removes none")
	}
	if blankRuns(t) == 0 {
		return nil, p.Die("the edit left no run of two blank lines at all, which means the deletions did " +
			"not happen where they were expected -- the sweep's canon.sh is what takes " +
			"them out, and the check asserts 0 AFTER the sweep")
	}
	p.Say(fmt.Sprintf("%d -> %d lines.  The core installs no handler, sets no terminal mode, runs no "+
		"select and asks the kernel nothing about a window; the host block is %d lines at "+
		"the bottom, inside the launcher region",
		linesBefore, len(strings.Split(t, "\n")), strings.Count(w103Host, "\n")))
	return []byte(t), nil
}
