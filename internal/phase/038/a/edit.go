package p038a

// Whim phase 38a (formerly 100) -- the deadly ladder that cannot run.  See GOAL.md.
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
// phase.  It is phase 35's kind of cut -- the POSSIBILITY has never existed -- rather
// than phase 31's, where an earlier Part II phase made a live path unreachable.  What makes
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
// mask and the two-row table, and internal/phase/038/a/check.go measures exactly that: the same
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

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B1b).  The
// assertions are the phase, and they are asked of the program's graph: the
// text version (history keeps it) matched exact lines and regular
// expressions, and each becomes the graph's question it stood for --
//
//   - `\bname\b` counts stay the text's question, on the C view (TEXTQ's
//     Mentions), number for number: a mention there is a prototype, a
//     string or a label as well as a use, which is the counting trap's
//     whole point;
//   - an exact line or block becomes a pattern: signal_info[]'s six rows,
//     catch_signals()'s deadly arm, deathtrap()'s head and its ladder
//     between `full_screen = FALSE;` and the `entered == 2` arm;
//   - "the one `catch_signals(deathtrap, SIG_ERR)`" is deathtrap's one use;
//   - the regexp over deathtrap's lines for the writes of `entered` is the
//     uses of the static's declaration, each asked whether it is written:
//     one, `++entered`, beside the initialiser;
//   - "statements beginning with `exit(`" are the uses of the external
//     exit(), each a call standing as a statement; and the six mentions of
//     `exit` are told apart -- two string literals, a goto and its label,
//     two calls -- where the text listed six lines.
//
// The ladder is deleted as a node.  The text's line arithmetic -- nine
// lines gone, the runs of two blank lines unchanged -- has no counterpart
// on the graph and is not asserted; the bytes are the build check's.

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim38a", Edit) }

// whim38aTable is signal_info[] as this phase was written against, a
// pattern for each row.
var whim38aTable = []string{
	`(init SIGHUP "HUP" TRUE)`,
	`(init SIGTERM "TERM" TRUE)`,
	`(init SIGINT "INT" FALSE)`,
	`(init SIGWINCH "WINCH" FALSE)`,
	`(init SIGTSTP "TSTP" FALSE)`,
	`(init (- 1) "Unknown!" FALSE)`,
}

// whim38aInstall is catch_signals()'s deadly arm: its then-block, the
// else-if after it whatever it is.
const whim38aInstall = `(if (. (index signal_info i) deadly)
	(block
		(def sa (struct sigaction))
		(= (. sa sa_handler) func_deadly)
		(call sigemptyset (addr (. sa sa_mask)))
		(= (. sa sa_flags) 0)
		(call sigaction (. (index signal_info i) sig) (addr sa) nullptr))
	_*)`

const whim38aLadder = `(if (>= entered 3)
	(block
		(call reset_signals)
		(if (>= entered 4) (block (call _exit 8)))
		(call exit 7)))`

// whim38aExitWord is `exit` as a word, in a string literal's text.
var whim38aExitWord = regexp.MustCompile(`\bexit\b`)

// Whim38a removes the deadly ladder that cannot run: deathtrap()'s
// `entered >= 3` arm, with `_exit(8)` and `exit(7)`.
//
// The argument has two halves and a reader is most likely to assume the first:
// there are only TWO deadly signals, and each is blocked inside its own
// handler, so `entered` can reach 2 and never 3.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("deadly", e, w)
	text := v.Text() // the file's C view, for the `\bname\b` counts
	mentions := func(name string) int { return edit.MentionCount(text, name) }

	// ---- 1. there is no third deadly signal, and there never was ---------
	// whim removed the four crash signals, so this is not a statement about
	// what zero did; it is the fact zero inherited.
	var live []string
	for _, s := range []string{"SIGSEGV", "SIGBUS", "SIGILL", "SIGFPE", "SIGABRT",
		"SIGQUIT", "SIGTRAP", "SIGSYS"} {
		if n := mentions(s); n > 0 {
			live = append(live, fmt.Sprintf("%s (%d)", s, n))
		}
	}
	if len(live) > 0 {
		v.Die("this file names %s -- the ladder below is unreachable only because the ONLY "+
			"deadly signals are SIGHUP and SIGTERM, so a third one makes the whole "+
			"argument false", strings.Join(live, ", "))
	}
	v.InTable("signal_info", func(v *graph.Verbs) {
		rows := v.Rows()
		ok := len(rows) == len(whim38aTable)
		for i := 0; ok && i < len(rows); i++ {
			ok = graph.Matches(clisp.MustPattern(whim38aTable[i]), rows[i])
		}
		v.Expect(ok, "signal_info[] is not the six rows this phase was written against -- "+
			"the two-row deadly table IS the argument")
	})
	v.Say("SIGSEGV, SIGBUS, SIGILL and SIGFPE at ZERO mentions -- whim removed all four -- " +
		"and signal_info[] is the five rows this phase was written against, of which " +
		"EXACTLY TWO are deadly: SIGHUP and SIGTERM")

	// ---- 2. each deadly signal is blocked inside its own handler ---------
	// One field.  `sa_flags = 0` with an empty `sa_mask` is the default: the
	// signal being delivered is added to the mask for the duration of the
	// handler.  SA_NODEFER is what would turn that off.
	v.InFunction("catch_signals", func(v *graph.Verbs) {
		v.One(whim38aInstall, "catch_signals()'s deadly arm -- the `sa_flags = 0` in it is "+
			"the whole reason the ladder cannot be reached")
	})
	for _, flag := range []string{"SA_NODEFER", "SA_RESETHAND", "SA_ONSTACK", "siginterrupt"} {
		if !v.Failed() && mentions(flag) > 0 {
			v.Die("%s is named in this file, and it is exactly what would let a deadly "+
				"signal interrupt its own handler", flag)
		}
	}
	v.Say("catch_signals()'s deadly arm installs with `sigemptyset(&sa.sa_mask)` and " +
		"`sa.sa_flags = 0`, and SA_NODEFER, SA_RESETHAND and siginterrupt are named " +
		"NOWHERE in the file -- so each deadly signal is blocked for the duration of its " +
		"own handler, and with only two of them `entered` can reach 2 and no further")

	// ---- 3. nothing but the kernel can enter deathtrap -------------------
	if k := mentions("deathtrap"); !v.Failed() && k != 3 {
		v.Die("deathtrap has %d mentions, expected 3 -- its prototype, its definition and "+
			"the one `catch_signals(deathtrap, SIG_ERR)` that installs it.  A fourth "+
			"would be an ordinary call, which no signal mask protects against", k)
	}
	if u := v.UsesOf("deathtrap"); !v.Failed() &&
		(len(u) != 1 || !graph.Matches(clisp.MustPattern("(call catch_signals deathtrap SIG_ERR)"), e.Parent(u[0])) ||
			e.Item(u[0]) != e.Parent(u[0])) {
		v.Die("deathtrap has %d uses, expected the one statement `catch_signals(deathtrap, "+
			"SIG_ERR);` -- it is the only way the handler is reached", len(u))
	}
	v.InFunction("deathtrap", func(v *graph.Verbs) {
		d := v.Scope()
		v.Expect(graph.Matches(clisp.MustPattern("(defn static deathtrap (fn ((sigarg int)) void) (def static entered int 0) _*)"), d),
			"deathtrap()'s head is not `deathtrap(int sigarg) { static int entered = 0;` -- "+
				"`entered` is a function-scope static starting at 0")
		if v.Failed() {
			return
		}
		entered := graph.Body(d)[0]
		var writes []string
		for _, u := range e.Uses(entered) {
			if s := written(e, u); s != "" {
				writes = append(writes, s)
			}
		}
		v.Expect(len(writes) == 1 && writes[0] == "pre++",
			"`entered` is written in deathtrap() as %s besides its initialiser, expected only "+
				"one `++entered` -- any other write would make the ladder reachable by "+
				"arithmetic rather than by a signal", strings.Join(writes, ", "))
	})
	v.Say("deathtrap at THREE mentions -- a prototype, a definition and the one " +
		"`catch_signals(deathtrap, SIG_ERR)` -- and inside it `entered` is written by its " +
		"initialiser and by one `++entered` and by nothing else, so the only way to raise " +
		"it is to deliver a deadly signal")

	// ---- 4. the counting trap, and the six mentions one by one -----------
	if !v.Failed() {
		var strs []string
		for _, s := range v.Strings() {
			if whim38aExitWord.MatchString(s.Atom) {
				strs = append(strs, s.Atom)
			}
		}
		want := []string{`"Type  :qa!  and press <Enter> to abandon all changes and exit Vim"`,
			`"Type  :qa  and press <Enter> to exit Vim"`}
		v.Expect(strings.Join(strs, "\x00") == strings.Join(want, "\x00"),
			"the string literals saying `exit` are %s, expected nv_esc's two", strings.Join(strs, ", "))
	}
	v.InFunction("vim_regsub_both", func(v *graph.Verbs) {
		v.One("(goto exit)", "`goto exit;` -- a GOTO, in vim_regsub_both()")
		v.One("(label exit)", "`exit:` -- a LABEL, in vim_regsub_both()")
	})
	calls := exitCalls(v, "exit")
	v.Expect(len(calls) == 2 && calls[0] == "deathtrap (call exit 7)" && calls[1] == "mch_exit (call exit r)",
		"the calls of exit() are %s, expected `exit(7)`, the ladder's, which this phase removes, "+
			"and `exit(r)`, mch_exit's -- the only exit() that has ever run", strings.Join(calls, "; "))
	if k := mentions("exit"); !v.Failed() && k != 6 {
		v.Die("`exit` as a word has %d mentions, expected the 6 named above", k)
	}
	if k := mentions("_exit"); !v.Failed() && k != 1 {
		v.Die("`_exit` has %d mentions, expected 1 -- the `_exit(8)` inside the ladder", k)
	}
	calls = exitCalls(v, "_exit")
	v.Expect(len(calls) == 1 && calls[0] == "deathtrap (call _exit 8)",
		"the calls of _exit() are %s, expected `_exit(8)` -- it is the ladder's inner arm", strings.Join(calls, "; "))
	v.Say("the counting trap: `exit` is SIX mentions and only TWO are calls -- two string " +
		"literals, a `goto exit;` and its `exit:` label in vim_regsub_both(), and " +
		"`exit(7)` and `exit(r)`.  `_exit` is one, and it is unambiguous")

	// ---- 5. what is around the ladder, for the collection ----------------
	for _, b := range []struct {
		Name string
		want int
	}{{"catch_signals", 4}, {"getout", 7}, {"mch_exit", 8}, {"preserve_exit", 3}, {"reset_signals", 4}} {
		if k := mentions(b.Name); !v.Failed() && k != b.want {
			v.Die("%s has %d mentions, expected %d -- the anchors were counted against a "+
				"different file", b.Name, k, b.want)
		}
	}
	v.Say("reset_signals 4 and catch_signals 4 going in, with getout 7, preserve_exit 3 and " +
		"mch_exit 8 -- none of which this phase touches")

	// ---- 6. the cut: the ladder, and nothing else ------------------------
	// It is found once in the file, and it sits between `full_screen =
	// FALSE;` and the `entered == 2` arm.
	if ladder := v.One(whim38aLadder, "the ladder -- it is deleted as one node and there is one of it"); ladder != nil {
		f := e.Function(ladder)
		v.Expect(f != nil && f == e.Defn("deathtrap") &&
			graph.Matches(clisp.MustPattern("(= full_screen FALSE)"), e.Sibling(ladder, -1)) &&
			graph.Matches(clisp.MustPattern("(if (== entered 2) _*)"), e.Sibling(ladder, 1)),
			"the ladder in its context -- deleting it must leave `full_screen = FALSE;` next to "+
				"the `entered == 2` arm")
		if !v.Failed() {
			if err := e.Delete(ladder); err != nil {
				v.Die("the ladder -- %v", err)
			}
		}
	}
	v.Say("the ladder goes, and with it the only `_exit` in the file and one of its two " +
		"`exit()` calls")

	// ---- 7. what the file is now -----------------------------------------
	if v.Failed() {
		return v.Done()
	}
	text = v.Text()
	if mentions("_exit") != 0 {
		v.Die("_exit survives the cut")
	}
	if k := mentions("exit"); !v.Failed() && k != 5 {
		v.Die("`exit` as a word has %d mentions after the cut, expected 5", k)
	}
	if calls := exitCalls(v, "exit"); !v.Failed() && (len(calls) != 1 || calls[0] != "mch_exit (call exit r)") {
		v.Die("the calls of exit() after the cut are %s, expected exactly one -- "+
			"mch_exit's `exit(r);`", strings.Join(calls, "; "))
	}
	for _, a := range []struct {
		Name string
		want int
	}{{"catch_signals", 4}, {"reset_signals", 3}} {
		if k := mentions(a.Name); !v.Failed() && k != a.want {
			v.Die("%s has %d mentions after the cut, expected %d", a.Name, k, a.want)
		}
	}
	if calls := exitCalls(v, "reset_signals"); !v.Failed() && (len(calls) != 1 || calls[0] != "mainerr (call reset_signals)") {
		v.Die("the calls of reset_signals() are %s, expected the one statement in mainerr -- "+
			"it is why the function is NOT orphaned by this cut", strings.Join(calls, "; "))
	}
	v.Say("_exit at 0, `exit` at 5 mentions with exactly ONE call left -- mch_exit's -- " +
		"reset_signals at 3, still called by mainerr() and so NOT orphaned, catch_signals " +
		"unchanged at 4")
	return v.Done()
}

// written is how the use u writes what it names -- `pre++`, `=`, `&` (its
// address taken: a write nothing here can see) -- or "" for a read.
func written(e *graph.Editor, u *graph.Node) string {
	p := e.Parent(u)
	if p == nil || !p.IsList() || len(p.Kids) < 2 || p.Kids[1] != u {
		return ""
	}
	switch h := p.Head(); h {
	case "pre++", "pre--", "post++", "post--", "addr":
		return h
	case "==", "!=", "<=", ">=":
		return ""
	default:
		if strings.HasSuffix(h, "=") {
			return h
		}
	}
	return ""
}

// exitCalls are the uses of name's declarations in the file, each told as
// `function form` when it is the callee of a call standing as a statement,
// and as what it is otherwise.
func exitCalls(v *graph.Verbs, name string) []string {
	if v.Failed() {
		return nil
	}
	e := v.Editor()
	var out []string
	for _, u := range v.UsesOf(name) {
		c := e.Parent(u)
		f := e.Function(u)
		fn := "file scope"
		if f != nil {
			fn = graph.DeclName(f)
		}
		if c == nil || !c.Is("call") || c.Kids[1] != u || e.Item(c) != c {
			out = append(out, fmt.Sprintf("%s: %s, not a call standing as a statement", fn, u.Atom))
			continue
		}
		out = append(out, fmt.Sprintf("%s %s", fn, formOf(c)))
	}
	return out
}

// formOf is n as a form, its atoms as spelled and no ids: `(call exit 7)`.
func formOf(n *graph.Node) string {
	if !n.IsList() {
		return n.Atom
	}
	s := make([]string, len(n.Kids))
	for i, k := range n.Kids {
		s[i] = formOf(k)
	}
	return "(" + strings.Join(s, " ") + ")"
}
