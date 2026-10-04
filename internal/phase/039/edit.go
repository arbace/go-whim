package p039

// Whim phase 39 (formerly 103) -- the signals and the terminal are the host's.  See GOAL.md.
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
// "the check goes entirely" -- finally kept: phases 4e and 27 left three `isatty()`
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
// TERM`, and ends through phase 38's `vim_host_exit` -> `__builtin_longjmp` -> `return
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
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3c).  The table of acts is the
// text program's, and so is the order: each `sub` and `cut` is its literal
// substitution made at the smallest node it changes (crefactor/graph's
// Substitutor: consecutive independent ones in one synthesized import, the
// rest after what they depend on), each `delfunc` the function's
// definition deleted as a form, the two index splices top-level forms --
// get_tty_fd() through the form before get_stty() deleted, RealWaitForChar()
// replaced by its three lines -- and the host block a substitution in the
// launcher region.  Every count is the text's, on the C view.  What has no
// counterpart: the line count in the last report line is the C view's,
// before the collection, where the text counted its own output before the
// sweep's canonical print, and the check that the edit left a run of two
// blank lines -- the text's sign that its deletions landed -- is gone with
// the text.

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim39", Edit) }

// w39Before is counted on the INPUT, so a later phase that moved one of these
// fails here and not in the middle of a cut.
var w39Before = []struct {
	Name string
	want int
}{
	{"mch_signal", 18}, {"signal_info", 9}, {"settmode", 13},
	{"mch_settmode", 2}, {"isatty", 4}, {"deathtrap", 3},
	{"win_resize_enabled", 6}, {"got_tstp", 4}, {"do_resize", 6},
}

// Edit gives the signals and the terminal to the host: the five signal
// handlers, mch_settmode's three-valued mode, mch_delay's sleep,
// RealWaitForChar's select, mch_get_shellsize and all three isatty() calls move
// into a 229-line host block at the bottom of the same file.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("host", e, w)
	text, err := e.Graph().C()
	if err != nil {
		return err
	}
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 88 drops the unused)
	t := string(text)
	mentions := func(name string) int { return edit.MentionCount([]byte(t), name) }
	linesBefore := len(strings.Split(t, "\n"))
	input := t // the anchors of the acts that are not substitutions are read on it: no act before them reaches theirs

	// ---- 0. this is the file the phase was written against -------------------
	for _, b := range w39Before {
		if k := mentions(b.Name); k != b.want {
			v.Die("the input has %d mentions of `%s`, expected %d -- this is not the tree "+
				"this phase was written against", k, b.Name, b.want)
			return v.Done()
		}
	}
	if strings.Contains(t, `\033[?2048$p`) || strings.Contains(t, "2048$p") {
		v.Die("the DECRQM query for mode 2048 is in the file, so the negotiation this phase " +
			"deletes as dead code is not dead")
		return v.Done()
	}

	// ---- the acts, in the order the phase performs them ----------------------
	// The substitutions report nothing (the text's sub() did not); each
	// `say` is the table's.  TWO ACTS ARE NOT IN THE TABLE: one deletes from
	// get_tty_fd()'s head to get_stty()'s, the other REPLACES
	// RealWaitForChar()'s whole definition with a three-line one.  Each is
	// performed before the act whose TAG follows it, never by a line number.
	quiet := graph.NewVerbs("host", e, io.Discard)
	b := quiet.Substitutions()
	forms := func(first, before string, what string) (lo, hi int) {
		f, g := e.Defn(first), e.Defn(before)
		lo, hi = -1, -1
		for i, x := range e.Graph().Forms {
			switch x {
			case f:
				lo = i
			case g:
				hi = i
			}
		}
		if f == nil || g == nil || lo < 0 || lo >= hi {
			v.Die("%s", what)
			return -1, -1
		}
		return lo, hi
	}
	// M1 -- the prototypes of the functions this phase brings in -- is made
	// FIRST: a fragment's names resolve where it is written (FRAG), and the
	// text wrote the calls of musl_get_winsize() and term_enter() above
	// their prototypes before it wrote the prototypes.  It is a substitution
	// of one line no other act's literal reaches, so the order moves no byte.
	ops := make([]w39Op, 0, len(w39Ops))
	for _, op := range w39Ops {
		if len(op.Args) > 0 && op.Args[len(op.Args)-1] == "M1" {
			ops = append([]w39Op{op}, ops...)
		} else {
			ops = append(ops, op)
		}
	}
	for _, op := range ops {
		if len(op.Args) > 0 {
			switch op.Args[len(op.Args)-1] {
			case "K2":
				what := "K1: get_tty_fd() and get_stty() are not the adjacent pair this phase cuts between"
				if !strings.Contains(input, "    static int\nget_tty_fd(int fd)") ||
					!strings.Contains(input, "    static void\nget_stty(void)") {
					v.Die("%s", what)
					return v.Done()
				}
				lo, hi := forms("get_tty_fd", "get_stty", what)
				if lo < 0 {
					return v.Done()
				}
				for _, x := range append([]*graph.Node(nil), e.Graph().Forms[lo:hi]...) {
					if b.Pending(x) {
						b.Flush()
					}
					if err := e.Delete(x); err != nil {
						v.Die("K1: %v", err)
						return v.Done()
					}
				}
			case "T2":
				what := "K5: RealWaitForChar() and no_Magic() are not the adjacent pair this phase replaces between"
				if !strings.Contains(input, "    static int\nRealWaitForChar(int fd, long msec, int *check_for_gpm") ||
					!strings.Contains(input, "    static int\nno_Magic(int x)") {
					v.Die("%s", what)
					return v.Done()
				}
				lo, hi := forms("RealWaitForChar", "no_Magic", what)
				if lo < 0 {
					return v.Done()
				}
				fs := e.Graph().Forms
				for _, x := range fs[lo:hi] {
					if b.Pending(x) {
						b.Flush()
					}
				}
				fs = e.Graph().Forms
				if _, err := e.SpliceC(graph.Frag{At: e.SpotRun(fs[lo], fs[hi-1]), Src: "static int\n" + w39RealWait}); err != nil {
					v.Die("K5: %v", err)
					return v.Done()
				}
			}
		}
		switch op.kind {
		case "say":
			// The LAST say is computed by the phase and carries no literal, so
			// it is written after the loop with the line counts it reports.
			if op.Args != nil {
				v.Say(op.Args[0])
			}
		case "cut", "sub":
			// cut(old, n, tag) is sub(old, "", n, tag); the defaults are n=1 and
			// an empty tag, and the table carries only the arguments the phase
			// actually passed.
			a := op.Args
			if a == nil {
				// The computed splice: the host block goes INSIDE the launcher
				// region phase 38b created and phase 38 filled, immediately above
				// `host_jump`, because that region is what becomes the second
				// file at the split.
				b.Add(graph.Subst{Old: "\nstatic void *host_jump[5];\nstatic int host_code;\n",
					New: "\n" + w39Host + "static void *host_jump[5];\nstatic int host_code;\n", N: 1, What: "H3"})
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
			b.Add(graph.Subst{Old: old, New: new, N: n, What: tag})
		case "delfunc":
			tag := ""
			if len(op.Args) > 1 {
				tag = op.Args[1]
			}
			// the definition whose head line it is, deleted whole: what the
			// text's brace matching took
			sig := strings.TrimLeft(op.Args[0], "\n")
			name := sig[:strings.Index(sig, "(")]
			d := e.Defn(name)
			if k := strings.Count(input, op.Args[0]); k != 1 || d == nil {
				v.Die("%s: the definition line `%s` occurs %d times, expected 1",
					tag, edit.CoreHead(strings.TrimSpace(op.Args[0]), 60), k)
				return v.Done()
			}
			if b.Pending(d) {
				b.Flush()
			}
			if err := e.Delete(d); err != nil {
				v.Die("%s: %v", tag, err)
				return v.Done()
			}
		}
		if quiet.Err != nil {
			return quiet.Err
		}
	}
	b.Flush()
	if quiet.Err != nil {
		return quiet.Err
	}

	// ---- what the file is now ------------------------------------------------
	if text, err = e.Graph().C(); err != nil {
		return err
	}
	t = string(text)
	var left []string
	for _, n := range w39Gone {
		if mentions(n) > 0 {
			left = append(left, n)
		}
	}
	if len(left) > 0 {
		v.Die("these names should be at 0 mentions and are not: %s", strings.Join(left, " "))
		return v.Done()
	}
	for _, r := range w39Want {
		if k := mentions(r.Name); k != r.want {
			v.Die("`%s` has %d mentions, expected %d -- %s", r.Name, k, r.want, r.why)
			return v.Done()
		}
	}
	n := 0
	for _, l := range strings.Split(t, "\n") {
		if strings.HasPrefix(l, "#") {
			n++
		}
	}
	if n != nInc {
		v.Die("the twelve #include directives moved, and this phase adds and removes none")
		return v.Done()
	}
	v.Say(fmt.Sprintf("%d -> %d lines.  The core installs no handler, sets no terminal mode, runs no "+
		"select and asks the kernel nothing about a window; the host block is %d lines at "+
		"the bottom, inside the launcher region",
		linesBefore, len(strings.Split(t, "\n")), strings.Count(w39Host, "\n")))
	return v.Done()
}
