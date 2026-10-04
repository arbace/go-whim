package cut

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// signalsKeep are the rows of signal_info the editor can still be sent, in
// the table's order, before the -1 that ends it.
var signalsKeep = []string{"SIGHUP", "SIGTERM", "SIGINT", "SIGWINCH", "SIGTSTP"}

var sigNames = regexp.MustCompile(`\bSIG[A-Z0-9]+\b`)

// cookTerminal is what prepare_to_exit's settmode statement becomes.
//
// THE CLAIM WAS FALSE WHEN THIS PHASE WAS WRITTEN.  prepare_to_exit() calls
// settmode(TMODE_COOK) to put the terminal back, and settmode opens with
// `if (!full_screen) return;` -- while deathtrap() sets full_screen = FALSE
// several lines before it gets there.  So the editor printed "Vim: Caught
// deadly signal TERM", emitted stoptermcap's escapes, exited, and left the
// terminal with ICANON and ECHO off.  Measured on the slave side of a pty,
// before and after this phase: identical, and wrong both times.  Upstream has
// the same hole.  The guard is there to avoid drawing on a screen that is
// not there, and putting the terminal back is not drawing, so it is lent
// full_screen for the length of the call.
//
// It uses settmode() rather than mch_settmode(), because mch_settmode() is
// defined 89,000 lines further down with no forward declaration left to
// reach it -- record 8 removed the ones nothing needed.
const cookTerminal = `{
    int was_full_screen = full_screen;
    full_screen = TRUE;
    settmode(TMODE_COOK);
    full_screen = was_full_screen;
}`

// NoSignals leaves the five signals this editor can still be sent.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): signal_info's other rows
// deleted (INITROW, nothing names a row's position), the calls cut before
// the definitions they name (DeleteDefinition refuses while one remains),
// the folds by form, and prepare_to_exit's statement by FRAG, scoped to it
// (`settmode(TMODE_COOK);` is in buf_write too).  The text version wrote
// the table anew whole; history keeps it.
func NoSignals(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nosignals", e, w)
	q := graph.NewVerbs("nosignals", e, io.Discard)
	keep := map[string]bool{"-1": true}
	for _, s := range signalsKeep {
		keep[s] = true
	}
	was := 0
	var gone []string
	q.InTable("signal_info", func(q *graph.Verbs) {
		for _, r := range q.Rows() {
			sig := r.Args()
			if len(sig) == 0 {
				continue
			}
			name := sig[0].Atom
			if sig[0].Is("-") {
				name = "-1"
			}
			if strings.HasPrefix(name, "SIG") {
				was++
			}
			if !keep[name] {
				gone = append(gone, "(init "+name+" _ _)")
			}
		}
		deleteRowsTyped(q, gone, "signal_info's rows")
	})
	if q.Failed() {
		return q.Done()
	}
	v.Sayf("signal_info: %d entries -> %d", was, len(signalsKeep))

	q.InFunction("set_signals", func(q *graph.Verbs) {
		q.Cut("(call mch_signal SIGUSR1 catch_sigusr1)", 1, "the SIGUSR1 install")
		q.Cut("(call mch_signal SIGPWR catch_sigpwr)", 1, "the SIGPWR install")
	})
	q.Cut("(call may_core_dump)", 2, "the may_core_dump calls")
	q.Cut("(= signal_stack (call alloc (call get_signal_stack_size)))", 1, "the signal stack")
	q.Cut("(call init_signal_stack)", 1, "its install")
	for _, name := range []string{"catch_sigusr1", "catch_sigpwr", "may_core_dump",
		"init_signal_stack", "get_signal_stack_size"} {
		q.DeleteDefinition(name, name)
	}
	q.Cut("(def static signal_stack (ptr char))", 1, "signal_stack")
	q.Cut("(def static sigstk stack_t)", 1, "sigstk")
	q.Cut("(def static got_sigusr1 _ FALSE)", 1, "got_sigusr1")
	if q.Failed() {
		return q.Done()
	}
	v.Say("SIGPWR, whose handler called an empty function; SIGUSR1, whose flag nothing reads")

	var onstack []*graph.Node
	for _, s := range q.Find("(= (. sa sa_flags) SA_ONSTACK)") {
		onstack = append(onstack, s.Args()[1])
	}
	q.ReplaceEachC(onstack, "0", 1, "the alternate signal stack")
	if q.Failed() {
		return q.Done()
	}
	v.Say("the alternate signal stack: sigaltstack, sysconf")

	q.FoldAlways("(if (!= sig SIGPWR) _)", 1, "the SIGPWR test")
	if q.Failed() {
		return q.Done()
	}
	v.Say("the test for a signal that can no longer arrive")

	// `in_mch_delay && sigarg == SIGQUIT` and the early return for
	// HUP/QUIT/TERM/PWR/USR1/USR2 were written when all six could arrive here.
	// Two can.  Left alone they would be a lie in the one function whose
	// remaining job is to be trustworthy.
	q.InFunction("deathtrap", func(q *graph.Verbs) {
		q.DropIf("(&& in_mch_delay (== sigarg SIGQUIT))", 1, "deathtrap's SIGQUIT test")
		q.CountIs(earlyTest, 1, "deathtrap's early-return test")
		q.Rewrite("(|| 0 ?hup (== sigarg SIGQUIT) ?term "+
			"(== sigarg SIGPWR) (== sigarg SIGUSR1) (== sigarg SIGUSR2))",
			"(|| ?hup ?term)", 1, "deathtrap's early-return test")
	})
	if q.Failed() {
		return q.Done()
	}
	v.Say("deathtrap stops testing for signals it cannot be sent")

	q.InFunction("prepare_to_exit", func(q *graph.Verbs) {
		q.ReplaceC("(call settmode TMODE_COOK)", cookTerminal, 1, "prepare_to_exit's settmode")
	})
	if q.Failed() {
		return q.Done()
	}
	v.Say("a killed editor puts the terminal back, which is what SIGHUP and SIGTERM are kept for")

	seen := map[string]bool{}
	for _, m := range sigNames.FindAll(v.Text(), -1) {
		seen[string(m)] = true
	}
	left := make([]string, 0, len(seen))
	for s := range seen {
		left = append(left, s)
	}
	sort.Strings(left)
	v.Sayf("signals named in the file: %s", strings.Join(left, " "))
	if err := v.Done(); err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

// earlyTest is deathtrap's early return as the seed wrote it, for six
// signals.
const earlyTest = "(|| 0 (== sigarg SIGHUP) (== sigarg SIGQUIT) (== sigarg SIGTERM) " +
	"(== sigarg SIGPWR) (== sigarg SIGUSR1) (== sigarg SIGUSR2))"
