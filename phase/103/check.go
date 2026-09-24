package p103

// Whim phase 103, the check -- the signals and the terminal are the host's.
// See phase/103/edit.go, and GOALS.md.
//
// Runs after phase/103/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old.c`, the source this phase was HANDED,
// and `old`, that source built with the boundary's own flags.  EVERY PROBE BELOW IS A
// PAIR, because a number from one binary is not evidence.
//
// WHAT IS CLAIMED, in three parts, and each has its own kind of check:
//
// STRUCTURE   the core names none of the host's vocabulary.  This is the phase's
// real content and ``zhostonly`` is the assertion: 43 words,
// every mention inside the host block, seven named exceptions in the
// core that are the deadly-signal message and the clock.  A count of
// `sigaction` would say nothing about WHERE.
// SYMBOLS     `nm -u` loses exactly `close dup isatty raise sigaddset sigismember
// sigprocmask` and gains nothing -- ONE comm, not two, because the two
// halves of this phase are one phase.  It does NOT lose `sigaction
// sigemptyset kill ioctl tcgetattr tcsetattr nanosleep select`, and the
// check requires those PRESENT: moving code from the core to the host
// inside one translation unit frees nothing, and a check that asserted
// them gone would be asserting the file split had happened.
// BEHAVIOUR   the declared delta is NOTHING, so tools/st.sh delta proves the
// recording did not move -- and the recording cannot see any of this
// (no case sends a signal, resizes a window, types `gs` or reaches EOF
// with a terminal on fd 2), so the phase owes probes.  Fifteen of them,
// below, seven that MUST differ and eight that MUST NOT.
//
// THE PROBES THAT MATTER MOST ARE THE ONES THAT MUST NOT DIFFER, and two of them are
// the whole reason the signal phase and the terminal phase were merged:
// `sigterm_restores` and `sighup_restores` require the editor killed mid-session to
// restore the terminal to ICANON=1 ECHO=1 ISIG=1 ONLCR=1 ICRNL=1, print `Vim: Caught
// deadly signal TERM` and `Vim: Finished.`, and exit 1 -- on BOTH binaries.  A
// host-side deadly handler that simply died would leave the tty raw, and the phase
// would have shipped a regression with a promise to fix it later.
//
// `gs_interrupt` IS THE PROBE WITH ITS OWN CONTROL.  `10gs` then CTRL-C recovers in
// about 1.8 s on both binaries; the check also builds THIS PHASE'S OWN OUTPUT with the
// two `host_tty_set` calls inside `musl_delay` deleted and requires that one NOT to
// recover at all.  Without that control, "both took 1.8 s" is two numbers agreeing and
// proves nothing about the sleep mode being real.
//
// `sigint_external` IS HERE BECAUSE IT CAUGHT A REAL BUG.  With no SIGINT handler at
// all -- which is what deleting `catch_sigint` leaves -- SIG_DFL kills the editor, and
// the `gs` probe found it: the sleep mode leaves ISIG on, so CTRL-C during a `gs`
// raises SIGINT.  The host catches it and hands the core a `0x03` byte, which is what
// `fill_input_buf` already turns into `got_int`.
//
// THE COUNTING TRAPS, AND WHY THE ASSERTIONS ARE SHAPED AS THEY ARE:
//
// * `resize_func` IS NOT AT 0.  It is `inchar_loop`'s parameter, which stays -- the
// core passes NULL and inchar_loop tests for it.  `assert resize_func at 0` fails
// on a correct phase.
// * `deathtrap` GOES UP, 3 -> 4, because the host installs it twice.  A phase about
// removing signal handling that leaves one MORE mention of a handler is exactly
// what this phase is, and the number says so.
// * `errno` DOES NOT MOVE, 3 -> 3.  Both its uses are host-side now and the third is
// `#include <errno.h>`, so `<errno.h>` leaves the CORE and `__errno_location`
// stays in `nm -u` until the split.  Said out loud rather than implied.
// * A PTY BYTE COUNT IS NOT AN ASSERTION.  The pty probes drive a real terminal with
// real waits, so what they assert is structural -- exit status, terminal mode,
// what text was drawn -- and the byte counts are REPORTED.  The two places a count
// IS the evidence are `tstp_external`, where it must differ, and `stopcont`, where
// both must draw a whole screen.  The deterministic pipe probes
// (tools/zstream.py) do assert exactly.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim103", Check) }

const (
	z20SleepA = "    if (relax)\n    {\n        host_tty_set(FALSE, TRUE);\n    }\n"
	z20SleepB = "    if (relax)\n    {\n        host_tty_set(TRUE, FALSE);\n    }\n"
)

// Whim103 is phase 103's check: the signals and the terminal are the host's.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim103 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "host", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	tmp, err := os.MkdirTemp("", "whim103")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflags, ldflags := strings.Fields(check.Z9Flag(mk, "CFLAGS")), strings.Fields(check.Z9Flag(mk, "LDFLAGS"))
	newC, oldC := check.ReadFile(f), check.ReadFile(filepath.Join(state, "old.c"))

	// --- 1. the control ------------------------------------------------------
	for _, t := range []string{z20SleepA, z20SleepB} {
		if strings.Count(newC, t) != 1 {
			r.Say("the musl_delay sleep-mode pair is not in the output exactly once, so the control below would not be the control")
			return harness.ErrReported
		}
	}
	nosleep := filepath.Join(tmp, "nosleep")
	os.WriteFile(nosleep+".c", []byte(strings.ReplaceAll(strings.ReplaceAll(newC, z20SleepA, ""), z20SleepB, "")), 0o644)
	r.Say("the control: this phase's own output with musl_delay()'s two host_tty_set() calls deleted and nothing else -- the sleep mode gone and the nanosleep left")
	ctl := make(chan error, 1)
	go func() {
		a := append(append(append([]string{}, cflags...), ldflags...), "-o", nosleep, nosleep+".c")
		ctl <- exec.Command("gcc", a...).Run()
	}()

	// --- 2. the source -------------------------------------------------------
	var fail []string
	mentions := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(t, -1))
	}
	for _, p := range []struct {
		Name string
		want int
	}{{"mch_signal", 18}, {"signal_info", 9}, {"settmode", 13}, {"isatty", 4}, {"deathtrap", 3}, {"win_resize_enabled", 6}} {
		if n := mentions(oldC, p.Name); n != p.want {
			fail = append(fail, fmt.Sprintf("the input has %d mentions of `%s`, expected %d", n, p.Name, p.want))
		}
	}
	gone := strings.Fields("mch_signal set_signals reset_signals catch_signals catch_int_signal " +
		"catch_sigint sig_tstp sigcont_handler sigcont_received after_sigcont " +
		"in_mch_suspend ignore_sigtstp got_tstp sig_winch set_sigwinch_handler " +
		"handle_resize do_resize mch_get_shellsize win_resize_setting " +
		"win_resize_enabled term_set_win_resize did_set_termresize p_trz " +
		"settmode mch_settmode mch_tcgetattr get_tty_fd mch_cur_tmode cur_tmode " +
		"mch_check_win stdout_isatty did_read_something isatty out_redir " +
		"tmode_T TMODE_COOK TMODE_RAW TMODE_SLEEP sighandler_T " +
		"MCH_DELAY_SETTMODE")
	var left []string
	for _, n := range gone {
		if mentions(newC, n) > 0 {
			left = append(left, n)
		}
	}
	if len(left) > 0 {
		fail = append(fail, "these names should be at 0 mentions and are not: "+strings.Join(left, " "))
	}
	for _, n := range gone {
		if mentions(oldC, n) == 0 {
			fail = append(fail, fmt.Sprintf("`%s` was already at 0 in the input, so its absence proves nothing", n))
		}
	}
	for _, p := range []struct {
		Name string
		want int
		why  string
	}{
		{"resize_func", 6, "inchar_loop's PARAMETER, which STAYS: the prototype, the definition and three uses inside it.  The core passes NULL and inchar_loop tests for it, so `assert resize_func at 0` fails on a correct phase"},
		{"deathtrap", 4, "a prototype, the definition and the TWO installations in musl_host_init().  It GOES UP: the host installs the core's deadly handler rather than replacing it, which is what keeps the terminal restored and the message printed"},
		{"vim_handle_signal", 5, "untouched -- a prototype, the definition, deathtrap's call and the two in ui_inchar"},
		{"signal_info", 4, "the struct tag, the array and deathtrap's two reads.  Its five rows are two and its `deadly` field is gone, because catch_signals() was its only reader"},
		{"term_enter", 6, "a prototype, the definition and four call sites"},
		{"term_leave", 5, "a prototype, the definition and three call sites"},
		{"term_entered", 6, "the flag that replaced the three-valued cur_tmode"},
		{"musl_host_init", 3, "prototype, definition, and the one call from mch_init()"},
		{"musl_get_winsize", 4, "prototype, definition, ui_get_shellsize()'s call and musl_read_input()'s"},
		{"musl_term_start", 3, "prototype, definition, term_enter()'s call"},
		{"musl_term_stop", 3, "prototype, definition, term_leave()'s call"},
		{"musl_tty_keys", 3, "prototype, definition, get_tty_info()'s call"},
		{"musl_delay", 3, "prototype, definition, mch_delay()'s call"},
		{"musl_wait_for_input", 3, "prototype, definition, RealWaitForChar()'s call -- ONE call site, because the pending check that was going to be a second function lives inside it"},
		{"musl_read_input", 3, "prototype, definition, fill_input_buf()'s call"},
		{"musl_suspend", 3, "prototype, definition, mch_suspend()'s call"},
		{"host_catch", 11, "its definition, eight installations in musl_host_init() and two in musl_suspend()"},
		{"errno", 3, "UNCHANGED, and said out loud: the #include and two uses, both of them host-side now.  So <errno.h> leaves the CORE and __errno_location stays in nm -u until the file splits"},
		{"sigaction", 2, "both in host_catch(), 0 in the core"},
		{"sigemptyset", 1, "host_catch(), 0 in the core"},
		{"kill", 2, "musl_suspend()'s kill(0, SIGTSTP) and vim_handle_signal()'s re-raise, which is the one core mention and a named exception in zhostonly"},
		{"ioctl", 2, "the #include and the host's one TIOCGWINSZ"},
		{"tcgetattr", 2, "host_tty_set() and musl_tty_keys()"},
		{"tcsetattr", 1, "host_tty_set()"},
		{"nanosleep", 1, "musl_delay()"},
		{"select", 1, "musl_wait_for_input().  There is no #include for it: it arrives transitively through <sys/param.h> (GOALS.md II.4c)"},
		{"getpid", 2, "mch_get_pid()'s and vim_handle_signal()'s -- neither is this phase's, and getpid stays"},
	} {
		if n := mentions(newC, p.Name); n != p.want {
			fail = append(fail, fmt.Sprintf("`%s` has %d mentions, expected %d -- %s", p.Name, n, p.want, p.why))
		}
	}
	rows := check.Z6RowRe.FindAllString(newC, -1)
	got, _ := harness.CommandNamesIn([]byte(newC), "whim-vim.c")
	if len(rows) != 98 || len(got) != 98 {
		fail = append(fail, "cmdnames[] is not the 98 rows phase 93 left -- this phase touches no Ex command")
	}
	if i := strings.Index(newC, "static struct vimoption options[]"); i >= 0 {
		j := strings.Index(newC[i:], "\n};")
		if n := len(check.Z12RowRe.FindAllString(newC[i:i+j], -1)); n != 107 {
			fail = append(fail, fmt.Sprintf("options[] has %d rows, expected 107 -- 'termresize' is the one row this phase removes, from the 108 phase 95 left", n))
		}
	}
	nd, allInc := 0, true
	L := strings.Split(newC, "\n")
	for _, l := range L {
		if strings.HasPrefix(l, "#") {
			nd++
			if !strings.HasPrefix(l, "#include <") {
				allInc = false
			}
		}
	}
	if nd != 12 || !allInc {
		fail = append(fail, "the output does not have exactly the twelve `#include` directives phase 99 left.  This phase adds none and removes none: <errno.h> and <termios.h> are still needed, by the HOST")
	}
	for k := 1; k < len(L); k++ {
		if L[k] == "" && L[k-1] == "" {
			fail = append(fail, "there is a run of two blank lines, which canon.sh should have taken")
			break
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("40 names at 0: every signal handler, the installer, the three-valued terminal mode and every question about whether this is a terminal.  `resize_func` is NOT among them (it is inchar_loop's parameter) and `deathtrap` goes UP, 3 -> 4, because the host installs the core's deadly handler rather than replacing it")
	r.Cont("the host is 11 host_catch() installations, 2 sigaction, 1 sigemptyset, 1 kill, 1 ioctl, 2 tcgetattr, 1 tcsetattr, 1 nanosleep and 1 select; `errno` does not move at all (3 -> 3), both its uses being host-side, so <errno.h> leaves the CORE and __errno_location stays until the split")
	r.Cont("cmdnames[] 98 unchanged, options[] 108 -> 107 (`termresize`), twelve #includes unchanged, no run of two blank lines")

	// --- 3. the structural claim ---------------------------------------------
	if err := check.Run(w, "tools/st.sh", "zhostonly", f); err != nil {
		return harness.ErrReported
	}

	// --- 4. the compile, the linkage and the libc surface --------------------
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneU, cameU := check.Comm23(before, after), check.Comm23(after, before)
	if strings.Join(goneU, " ") != "close dup isatty raise sigaddset sigismember sigprocmask" || len(cameU) > 0 {
		r.Say("the libc surface did not move by exactly those seven:")
		r.Cont("gone: %s", check.TrSpace(goneU))
		r.Cont("came: %s", check.TrSpace(cameU))
		r.Cont("NOTHING MAY ARRIVE, and nothing else may leave.")
		return harness.ErrReported
	}
	for _, want := range strings.Fields("sigaction sigemptyset kill ioctl tcgetattr tcsetattr nanosleep select read write __errno_location") {
		if !check.Contains(after, want) {
			r.Say("%s is NOT undefined any more, and this phase does not", want)
			r.Cont("claim to free it -- the host block calls it from the same")
			r.Cont("translation unit.  If this is really gone, the claim in")
			r.Cont("phase/103/edit.go's header is wrong and needs rewriting")
			return harness.ErrReported
		}
	}
	for _, absent := range strings.Fields("open creat openat stat access fcntl getcwd strerror fopen fdopen opendir fclose getc putc fsync exit _exit") {
		if check.Contains(after, absent) {
			r.Say("%s is undefined, and no phase since 96 has put it back", absent)
			return harness.ErrReported
		}
	}
	r.Say("symbols %s -> %s, the gone set is EXACTLY close dup isatty raise sigaddset sigismember sigprocmask and NOTHING arrives -- and sigaction sigemptyset kill ioctl tcgetattr tcsetattr nanosleep select are REQUIRED still present, because moving a call inside one translation unit frees nothing",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")), strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))
	r.Say("GOALS.md II.4b in its strongest form: with close and dup gone the core cannot open, close or duplicate ANY descriptor -- it is handed fds 0, 1 and 2 and that is the whole of it")

	// --- 5. the binary -------------------------------------------------------
	b := &check.Rep{Tag: "build", W: w}
	_ = exec.Command("make", "-C", work, "clean").Run()
	if _, err := os.Stat(filepath.Join(work, "whim-vim")); err == nil {
		b.Say("the clean did not remove whim-vim")
		return harness.ErrReported
	}
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		b.Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	old, _ := filepath.Abs(filepath.Join(state, "old"))
	b.Say("ok, %s -> %d lines, %d bytes", beforeLines, check.CountLines([]byte(check.ReadFile(f))), check.SizeOf(bin))
	if err := <-ctl; err != nil {
		r.Say("the control did not build")
		return harness.ErrReported
	}

	// --- 6. the probes -------------------------------------------------------
	return z20Probes(r, old, bin, nosleep)
}
