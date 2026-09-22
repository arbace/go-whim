package p100

// Whim phase 100 -- the deadly ladder that cannot run.  See GOAL.md.
//
// `deathtrap()` is the handler for the deadly signals, and it opens with a ladder that
// counts how many times it has been entered:
//
// if (entered >= 3)
// {
// reset_signals();
// if (entered >= 4)
// {
// _exit(8);
// }
// exit(7);
// }
//
// NOTHING IN ANY BUILD OF whim-vim CAN MAKE `entered` REACH 3, and that is the whole
// phase.  It is phase 96's kind of cut -- the POSSIBILITY has never existed -- rather
// than phase 92's, where an earlier Part II phase made a live path unreachable.  What makes
// it impossible is two facts about this file, and neither is the core's doing:
//
// * `catch_signals()` installs the deadly handler with `sa.sa_flags = 0` and
// `sigemptyset(&sa.sa_mask)`.  NO `SA_NODEFER`, so the signal being handled is
// blocked for the duration of its own handler.  That is the whole mechanism, and
// it is asserted below character for character.
// * `signal_info[]` has exactly TWO rows carrying `deadly = TRUE`, `SIGHUP` and
// `SIGTERM`.  SIGSEGV, SIGBUS, SIGILL and SIGFPE are at ZERO mentions in this file
// -- whim removed all four -- so there is no third deadly signal to arrive.
//
// Two deadly signals, each blocked inside its own handler, means `entered` can reach 2
// -- TERM nested inside HUP's handler, or the reverse, which is the
// `Vim: Double signal, exiting` arm, and that arm calls `getout(1)` and never returns.
// It cannot reach 3: by then both are blocked and nothing else is caught.
//
// THE REASON MATTERS AND THE WRONG REASON IS AVAILABLE.  A phase that removed the
// ladder because "reset_signals() makes it unreachable" would have the right answer for
// the wrong reason -- `reset_signals()` is INSIDE the ladder and is never reached -- and
// would be wrong on any tree with three deadly signals.  The argument is the signal
// mask and the two-row table, and phase/100/check.go measures exactly that: the same
// source with ONE FIELD CHANGED, `sa.sa_flags = SA_NODEFER`, reaches the ladder and
// exits 7, and with one more forced signal exits 8.
//
// THE COUNTING TRAP, stated here because the obvious assertion fails on a correct
// phase.  `\bexit\b` has SIX mentions in the input and only two of them are calls:
//
// 46925  "Type  :qa!  and press <Enter> to abandon all changes and exit Vim"
// 46945  "Type  :qa  and press <Enter> to exit Vim"          two string literals
// 55908          exit(7);                                    this phase's
// 56170      exit(r);                                        mch_exit's, and the
// only one that runs
// 58180                          goto exit;                  a LABEL, inside
// 58251  exit:                                               vim_regsub_both()
//
// So `assert exit at 0 mentions` fails on a correct phase, and `assert 'exit(' at 0`
// fails on `mch_exit(`, `preserve_exit(`, `prepare_to_exit(`, `read_error_exit(` and
// `getout(`.  What this edit asserts instead is every one of the six BY ITS OWN EXACT
// LINE, and what the check asserts is `nm -u`, which cannot be confused by a label.
//
// THE INPUT SOURCE AND ITS BINARY ARE KEPT, because every probe this phase has is a
// build of the source it was HANDED: the ladder is not in the output, so the only place
// it can be shown to be dead -- and shown to be live under `SA_NODEFER` -- is the input.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim100", Edit) }

var whim100ExitCall = regexp.MustCompile(`(?m)^\s*exit\(`)

// whim100Entered is the Python's `\+\+entered|entered\+\+|--entered|entered--|
// entered\s*=(?!=)` WITHOUT its negative lookahead, which RE2 has not.
//
// THE BYTE TEST IS WHAT REPLACES IT, and it is the shape tools/README.md
// records for `noconv` and `nobackup`: match the assignment without the
// lookahead, then require the byte after the `=` not to be another `=`.  A
// capturing rewrite is NOT equivalent in general -- a capture consumes its
// delimiters, so two matches sharing one would lose the second -- but here the
// excluded character is one byte past the end of the match and is consumed by
// nothing, so testing it is exact.
//
// What it is FOR: `entered == 2` is a comparison and `entered = 0` is a write,
// and the phase's whole argument is that `entered` is written by its
// initialiser and one `++entered` and by nothing else.
var whim100Entered = regexp.MustCompile(`\+\+entered|entered\+\+|--entered|entered--|entered\s*=`)

var whim100Space = regexp.MustCompile(`\s+`)

const whim100Table = "} signal_info[] =\n" +
	"{\n" +
	"    {SIGHUP,        \"HUP\",      TRUE},\n" +
	"    {SIGTERM,       \"TERM\",     TRUE},\n" +
	"    {SIGINT,        \"INT\",      FALSE},\n" +
	"    {SIGWINCH,      \"WINCH\",    FALSE},\n" +
	"    {SIGTSTP,       \"TSTP\",     FALSE},\n" +
	"    {-1,            \"Unknown!\", FALSE}\n" +
	"};\n"

const whim100Install = "        if (signal_info[i].deadly)\n" +
	"        {\n" +
	"            struct sigaction sa;\n" +
	"\n" +
	"            sa.sa_handler = func_deadly;\n" +
	"            sigemptyset(&sa.sa_mask);\n" +
	"            sa.sa_flags = 0;\n" +
	"            sigaction(signal_info[i].sig, &sa, NULL);\n" +
	"        }\n"

const whim100Head = "deathtrap  (int sigarg  __attribute__((unused)) ) \n" +
	"{\n" +
	"    static int  entered = 0;\n"

const whim100Ladder = "    if (entered >= 3)\n" +
	"    {\n" +
	"        reset_signals();\n" +
	"        if (entered >= 4)\n" +
	"        {\n" +
	"            _exit(8);\n" +
	"        }\n" +
	"        exit(7);\n" +
	"    }\n"

// Whim100 removes the deadly ladder that cannot run: nine lines of deathtrap()'s
// `entered >= 3` arm, with `_exit(8)` and `exit(7)`.
//
// The argument has two halves and a reader is most likely to assume the first:
// there are only TWO deadly signals, and each is blocked inside its own
// handler, so `entered` can reach 2 and never 3.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "deadly", W: w}

	// ---- 1. there is no third deadly signal, and there never was ---------
	// whim removed the four crash signals, so this is not a statement about
	// what zero did; it is the fact zero inherited.
	var live []string
	for _, s := range []string{"SIGSEGV", "SIGBUS", "SIGILL", "SIGFPE", "SIGABRT",
		"SIGQUIT", "SIGTRAP", "SIGSYS"} {
		if n := p.Mentions(text, s); n > 0 {
			live = append(live, fmt.Sprintf("%s (%d)", s, n))
		}
	}
	if len(live) > 0 {
		return nil, p.Die("this file names %s -- the ladder below is unreachable only because the ONLY "+
			"deadly signals are SIGHUP and SIGTERM, so a third one makes the whole "+
			"argument false", strings.Join(live, ", "))
	}
	if err := p.AssertOnce(text, whim100Table, "signal_info[] as this phase was written against",
		"the two-row deadly table IS the argument and it is asserted as text"); err != nil {
		return nil, err
	}
	p.Say("SIGSEGV, SIGBUS, SIGILL and SIGFPE at ZERO mentions -- whim removed all four -- " +
		"and signal_info[] is the five rows this phase was written against, of which " +
		"EXACTLY TWO are deadly: SIGHUP and SIGTERM")

	// ---- 2. each deadly signal is blocked inside its own handler ---------
	// One field.  `sa_flags = 0` with an empty `sa_mask` is the default: the
	// signal being delivered is added to the mask for the duration of the
	// handler.  SA_NODEFER is what would turn that off.
	if err := p.AssertOnce(text, whim100Install, "catch_signals()'s deadly arm",
		"the `sa_flags = 0` in it is the whole reason the ladder cannot be reached"); err != nil {
		return nil, err
	}
	for _, flag := range []string{"SA_NODEFER", "SA_RESETHAND", "SA_ONSTACK", "siginterrupt"} {
		if p.Mentions(text, flag) > 0 {
			return nil, p.Die("%s is named in this file, and it is exactly what would let a deadly "+
				"signal interrupt its own handler", flag)
		}
	}
	p.Say("catch_signals()'s deadly arm installs with `sigemptyset(&sa.sa_mask)` and " +
		"`sa.sa_flags = 0`, and SA_NODEFER, SA_RESETHAND and siginterrupt are named " +
		"NOWHERE in the file -- so each deadly signal is blocked for the duration of its " +
		"own handler, and with only two of them `entered` can reach 2 and no further")

	// ---- 3. nothing but the kernel can enter deathtrap -------------------
	if k := p.Mentions(text, "deathtrap"); k != 3 {
		return nil, p.Die("deathtrap has %d mentions, expected 3 -- its prototype, its definition and "+
			"the one `catch_signals(deathtrap, SIG_ERR)` that installs it.  A fourth "+
			"would be an ordinary call, which no signal mask protects against", k)
	}
	if err := p.AssertOnce(text, "    catch_signals(deathtrap, SIG_ERR);\n",
		"the one installation of deathtrap", "it is the only way the handler is reached"); err != nil {
		return nil, err
	}
	if err := p.AssertOnce(text, whim100Head, "deathtrap()'s head",
		"`entered` is a function-scope static starting at 0"); err != nil {
		return nil, err
	}
	i := bytes.Index(text, []byte(whim100Head))
	seg := text[i:]
	if j := bytes.Index(seg, []byte("\n}\n")); j >= 0 {
		seg = seg[:j+3]
	}
	var writes []string
	for _, loc := range whim100Entered.FindAllIndex(seg, -1) {
		m := seg[loc[0]:loc[1]]
		// The byte test: an assignment whose next byte is `=` is the
		// comparison `entered ==`, which the Python's (?!=) excluded.
		if m[len(m)-1] == '=' && loc[1] < len(seg) && seg[loc[1]] == '=' {
			continue
		}
		writes = append(writes, string(whim100Space.ReplaceAll(m, nil)))
	}
	sort.Strings(writes)
	if strings.Join(writes, "\x00") != "++entered\x00entered=" {
		return nil, p.Die("`entered` is written in deathtrap() as %s, expected only its initialiser and "+
			"one `++entered` -- any other write would make the ladder reachable by "+
			"arithmetic rather than by a signal", strings.Join(writes, ", "))
	}
	p.Say("deathtrap at THREE mentions -- a prototype, a definition and the one " +
		"`catch_signals(deathtrap, SIG_ERR)` -- and inside it `entered` is written by its " +
		"initialiser and by one `++entered` and by nothing else, so the only way to raise " +
		"it is to deliver a deadly signal")

	// ---- 4. the counting trap, and the six mentions one by one -----------
	for _, s := range []struct{ s, why string }{
		{"                char *ms = _(\"Type  :qa!  and press <Enter> to abandon all " +
			"changes and exit Vim\");\n", "a string literal"},
		{"                    msg(_(\"Type  :qa  and press <Enter> to exit Vim\"));\n",
			"a string literal"},
		{"        exit(7);\n", "the ladder's, which this phase removes"},
		{"    exit(r);\n", "mch_exit's -- the only exit() that has ever run"},
		{"                        goto exit;\n", "a GOTO, in vim_regsub_both()"},
		{"exit:\n", "a LABEL, in vim_regsub_both()"},
	} {
		if err := p.AssertOnce(text, s.s, "`"+strings.TrimSpace(s.s)+"`", s.why); err != nil {
			return nil, err
		}
	}
	if k := p.Mentions(text, "exit"); k != 6 {
		return nil, p.Die("`exit` as a word has %d mentions, expected the 6 named above", k)
	}
	if k := p.Mentions(text, "_exit"); k != 1 {
		return nil, p.Die("`_exit` has %d mentions, expected 1 -- the `_exit(8)` inside the ladder", k)
	}
	if err := p.AssertOnce(text, "            _exit(8);\n", "`_exit(8)`",
		"it is the ladder's inner arm"); err != nil {
		return nil, err
	}
	if k := len(whim100ExitCall.FindAll(text, -1)); k != 2 {
		return nil, p.Die("%d statements begin with `exit(`, expected 2 -- `exit(7)` and `exit(r)`", k)
	}
	p.Say("the counting trap: `exit` is SIX mentions and only TWO are calls -- two string " +
		"literals, a `goto exit;` and its `exit:` label in vim_regsub_both(), and " +
		"`exit(7)` and `exit(r)`.  `_exit` is one, and it is unambiguous")

	// ---- 5. what is around the ladder, for the sweep ---------------------
	for _, b := range []struct {
		Name string
		want int
	}{{"catch_signals", 4}, {"getout", 7}, {"mch_exit", 8}, {"preserve_exit", 3}, {"reset_signals", 4}} {
		if k := p.Mentions(text, b.Name); k != b.want {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors were counted against a "+
				"different file", b.Name, k, b.want)
		}
	}
	p.Say("reset_signals 4 and catch_signals 4 going in, with getout 7, preserve_exit 3 and " +
		"mch_exit 8 -- none of which this phase touches")

	// ---- 6. the cut: nine lines, and nothing else ------------------------
	runsBefore := p.BlankRuns(text)
	linesBefore := p.Lines(text)
	if err := p.AssertOnce(text, whim100Ladder, "the ladder",
		"it is deleted as exact text and there is one of it"); err != nil {
		return nil, err
	}
	// It sits between `full_screen = FALSE;` and the `entered == 2` arm with
	// no blank line either side, so nine lines go and the paragraphing does
	// not move.
	if err := p.AssertOnce(text, "    full_screen = FALSE;\n"+whim100Ladder+"    if (entered == 2)\n",
		"the ladder in its context",
		"deleting it must leave `full_screen = FALSE;` next to the `entered == 2` arm"); err != nil {
		return nil, err
	}
	text = bytes.Replace(text, []byte(whim100Ladder), nil, 1)
	p.Say("the nine lines go, and with them the only `_exit` in the file and one of its two " +
		"`exit()` calls")

	// ---- 7. what the file is now -----------------------------------------
	if p.Mentions(text, "_exit") != 0 {
		return nil, p.Die("_exit survives the cut")
	}
	if k := p.Mentions(text, "exit"); k != 5 {
		return nil, p.Die("`exit` as a word has %d mentions after the cut, expected 5", k)
	}
	if k := len(whim100ExitCall.FindAll(text, -1)); k != 1 {
		return nil, p.Die("%d statements begin with `exit(` after the cut, expected exactly one -- "+
			"mch_exit's `exit(r);`", k)
	}
	for _, a := range []struct {
		Name string
		want int
	}{{"catch_signals", 4}, {"reset_signals", 3}} {
		if k := p.Mentions(text, a.Name); k != a.want {
			return nil, p.Die("%s has %d mentions after the cut, expected %d", a.Name, k, a.want)
		}
	}
	if err := p.AssertOnce(text, "    reset_signals();\n", "the one remaining `reset_signals();` call",
		"it is mainerr's, and it is why the function is NOT orphaned by this cut"); err != nil {
		return nil, err
	}
	if r := p.BlankRuns(text); r != runsBefore {
		return nil, p.Die("the cut left %d runs of two blank lines where there were %d", r, runsBefore)
	}
	if n := p.Lines(text); n != linesBefore-9 {
		return nil, p.Die("the file lost %d lines, expected 9", linesBefore-n)
	}
	p.Sayf("_exit at 0, `exit` at 5 mentions with exactly ONE call left -- mch_exit's -- "+
		"reset_signals at 3, still called by mainerr() and so NOT orphaned, catch_signals "+
		"unchanged at 4, and %d runs of two blank lines, exactly as before", p.BlankRuns(text))
	return text, nil
}
