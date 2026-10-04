package p038

// Whim phase 38 (formerly 102) -- the core can no longer stop the process.  See GOAL.md.
//
// `mch_exit()` ends the editor, and its last statement was `exit(r);`.  Phase 38a left
// that as the ONLY `exit()` call in the file and this phase replaces it with a call
// through a function pointer the host installs:
//
// static void (*vim_host_exit)(int);           beside mch_exit's definition
// vim_host_exit(r);                            mch_exit's last statement
// vim_main(int argc, char **argv, void (*exit_fn)(int))
// vim_host_exit = exit_fn;                 vim_main's first statement
//
// and phase 38b's six-line launcher becomes twenty: a jump buffer, a status, a
// `host_exit()` that records the status and jumps, and a `main()` that lands there and
// RETURNS the status.  The editor no longer ends the process; it hands the process
// back, with a number.
//
// WHY A FUNCTION POINTER AND NOT A NON-LOCAL JUMP IN THE CORE.  Three routes end
// `mch_exit` without calling `exit`, and only this one is a thing the core can SAY:
//
// * thread a return value up through every caller.  NOT AVAILABLE, and the reason is
// a type: `cmdnames[].cmd_func` is `void (*)(exarg_T *)` for all 98 rows and
// `nv_cmds[].cmd_func` is `void (*)(cmdarg_T *)` for all 194, both dispatched
// through ONE indirect call, and `deathtrap` is `void (*)(int)` by the kernel's
// contract.  It is not expensive; it cannot be written.
// * a `setjmp` in the core.  It puts the mechanism in the file that is meant to stop
// naming mechanisms, and it costs symbols -- see below.
// * the core calls out and does not come back.  `vim_host_exit(r);` is four words of
// C that say exactly that, the host decides HOW, and an indirect call names no
// symbol.  GOALS.md II.4c settles it, and it is the one route whose C text
// already says what a JVM host would have to do: an interface call whose
// implementation throws.
//
// THE INDIRECTION IS TEMPORARY AND GOALS.md II.4c SAYS SO.  It exists because
// everything is still one translation unit and "nothing is global but main()" is still
// the invariant: a pointer the launcher installs through a parameter adds no external
// symbol, where a `musl_exit(int)` the host defines would.  Once the file is split
// there IS a declared boundary, `vim_host_exit` becomes a plain `musl_exit(int)`
// prototype at the top of the editor file, and the parameter and the pointer both go.
//
// THE MECHANISM IN THE LAUNCHER IS `__builtin_setjmp`/`__builtin_longjmp`, AND THAT IS
// A MEASUREMENT RATHER THAN A PREFERENCE.  Returning from `main()` is what ends the
// process without naming `exit`, and getting back to `main()` from inside `deathtrap`
// needs a non-local jump.  Measured on this tree, all three spellings:
//
// launcher jumps with          nm -u        what it costs
// __builtin_setjmp             32 -> 31     nothing arrives; no header
// sigsetjmp/siglongjmp         32 -> 33     +sigsetjmp +siglongjmp, +<setjmp.h>
// setjmp/longjmp               32 -> 33     +setjmp +longjmp, +<setjmp.h>
// the launcher calls exit(r)   32 -> 32     nothing moves; the phase achieves
// nothing at all
//
// So the two library spellings are NET WORSE than not doing the phase: `exit` leaves
// and two symbols arrive in its place, plus a thirteenth `#include` in a file whose
// last phase but two removed six.  internal/phase/038/check.go builds the `sigsetjmp` variant
// and requires `nm -u` to show exactly that, so the road not taken is a number in the
// record and not a memory.
//
// THE ONE THING `sigsetjmp` BUYS, AND THE MEASUREMENT THAT SAYS IT IS NOT NEEDED HERE.
// `siglongjmp` restores the signal mask and `__builtin_longjmp` does not, so after a
// jump out of `deathtrap` on SIGTERM the landing site still has SIGTERM blocked
// (measured: `sigismember` says 1; on SIGHUP it says 0, because `prepare_to_exit()`
// calls `mch_signal(SIGHUP, SIG_IGN)` and that unblocks it on its way past).  THAT IS
// EXACTLY THE STATE THE PROCESS ALREADY DIED IN.  Measured on the source this phase
// was handed, with the same probe immediately before `exit(r);`: SIGTERM blocked on a
// SIGTERM death, clear on a SIGHUP one -- the identical pair.  `exit()` was being
// called from inside the handler, with the handled signal blocked, and it always has
// been.  So `__builtin_longjmp` PRESERVES the mask the process ends with and
// `siglongjmp` would CHANGE it; the check asserts the pair on both binaries.
//
// A host that keeps running rather than returning is the case where the mask matters,
// and that host does not exist yet: `main()` here lands and returns, four lines later.
// When the split happens the host writes `musl_exit(int)` for itself and owns that
// question along with `sigprocmask`, which is a symbol the HOST is allowed to name.
//
// `longjmp` OUT OF A SIGNAL HANDLER IS UNDEFINED BY THE LETTER OF C11 when the signal
// interrupted a function that is not async-signal-safe, which here it always does --
// `deathtrap` already calls `out_str`, `sprintf`, `ml_close_all` and `free`, and
// upstream has always done that and got away with it because the process was about to
// die.  It is deliberate, it is measured on musl/x86-64 at -O0, and it is said here
// rather than discovered later.  The design that removes it is the signal handlers
// becoming the host's -- `sig_winch`'s `do_resize = TRUE; return;` applied to the
// deadly two -- which is a later phase with a real declared delta.
//
// THE COUNTING TRAP, AGAIN, AND IT IS WHY THE SYMBOL IS THE ASSERTION.  `\bexit\b` is
// FIVE mentions in the input and only ONE is a call: two string literals, a
// `goto exit;` and its `exit:` label in vim_regsub_both(), and `exit(r);`.  After this
// phase it is FOUR and NONE is a call -- so `assert exit at 0 mentions` fails on a
// correct phase, and `assert 'exit(' at 0` fails on `mch_exit(`, `preserve_exit(`,
// `prepare_to_exit(`, `read_error_exit(`, `getout(` and the new `vim_host_exit(` and
// `host_exit(`.  The assertion that works is `nm -u`.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3c).  The pointer is a fragment
// before mch_exit()'s definition; the one line is the call `exit(r)`
// rebuilt as `vim_host_exit(r)`, found as mch_exit()'s last statement after
// `ml_close_all(TRUE)`; the callback is vim_main()'s third parameter
// (PARAM's AddParam: the launcher's call passes `host_exit`) and its
// installation a statement before the first of vim_main()'s body; the
// launcher's other half -- host_jump, host_code and host_exit() -- a
// fragment before main(), and main()'s body the setjmp landing.  The
// assertions are the text's on the C view, but the line count, which
// counted the text's own output before its canonical print.

import (
	"bytes"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim38", Edit) }

var whim38ExitCall = regexp.MustCompile(`(?m)^\s*exit\(`)

// whim38Old is phase 38b's five-line launcher; whim38New is the twenty-line one
// that lands on __builtin_setjmp and RETURNS the status.
const whim38Old = "\n    int\nmain(int argc, char **argv)\n{\n" +
	"    return vim_main(argc, argv);\n}\n"

const whim38New = "\nstatic void *host_jump[5];\n" +
	"\n" +
	"static int host_code;\n" +
	"\n" +
	"    static void\n" +
	"host_exit(int r)\n" +
	"{\n" +
	"    host_code = r;\n" +
	"    __builtin_longjmp(host_jump, 1);\n" +
	"}\n" +
	"\n" +
	"    int\n" +
	"main(int argc, char **argv)\n" +
	"{\n" +
	"    if (__builtin_setjmp(host_jump) != 0)\n" +
	"    {\n" +
	"        return host_code;\n" +
	"    }\n" +
	"    return vim_main(argc, argv, host_exit);\n" +
	"}\n"

// whim38Host is the launcher's half that goes before main(); whim38Main is
// main()'s new body.
const whim38Host = `static void *host_jump[5];
static int host_code;

    static void
host_exit(int r)
{
    host_code = r;
    __builtin_longjmp(host_jump, 1);
}
`

const whim38Main = `    if (__builtin_setjmp(host_jump) != 0)
    {
        return host_code;
    }
    return vim_main(argc, argv, host_exit);
`

// Edit takes the core's last way of stopping the process: mch_exit()'s
// `exit(r);` becomes a call through a pointer the launcher installs.
//
// __builtin_setjmp rather than <setjmp.h> because this phase adds no header,
// and the directive count is asserted at the end to say so.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	p := edit.Ph{Tag: "hostexit", W: w}
	text, err := e.Graph().C()
	if err != nil {
		return err
	}
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 88 drops the unused)

	// ---- 1. there is exactly one way Out, and phase 38a is why ------------
	// The counting trap first: `exit` is five words and one call.
	for _, f := range []struct{ s, why string }{
		{"                char *ms = _(\"Type  :qa!  and press <Enter> to abandon all " +
			"changes and exit Vim\");\n", "a string literal"},
		{"                    msg(_(\"Type  :qa  and press <Enter> to exit Vim\"));\n",
			"a string literal"},
		{"    exit(r);\n", "mch_exit's, and the ONLY exit() call in the file"},
		{"                        goto exit;\n", "a GOTO, in vim_regsub_both()"},
		{"exit:\n", "a LABEL, in vim_regsub_both()"},
	} {
		if err = p.AssertOnce(text, f.s, "`"+trimSpace(f.s)+"`", f.why); err != nil {
			return err
		}
	}
	if k := p.Mentions(text, "exit"); k != 5 {
		return p.Die("`exit` as a word has %d mentions, expected the 5 named above", k)
	}
	if p.Mentions(text, "_exit") != 0 {
		return p.Die("`_exit` is back, and phase 38a took it to zero")
	}
	if k := len(whim38ExitCall.FindAll(text, -1)); k != 1 {
		return p.Die("%d statements begin with `exit(`, expected exactly 1 -- phase 38a left "+
			"mch_exit's `exit(r);` as the file's only one, which is what makes this phase "+
			"ONE line in ONE function", k)
	}
	for _, name := range []string{"vim_host_exit", "host_exit", "host_jump", "host_code"} {
		if k := p.Mentions(text, name); k != 0 {
			return p.Die("`%s` already has %d mentions -- a name this phase introduces is taken",
				name, k)
		}
	}
	p.Say("`exit` is FIVE mentions and exactly ONE call -- two string literals, a `goto " +
		"exit;` and its `exit:` label in vim_regsub_both(), and mch_exit's `exit(r);`.  " +
		"Phase 38a is what left one call site, and it is why this phase is one line in one " +
		"function")

	// ---- 2. the pointer, beside the function that is its only reader -----
	// A file-scope OBJECT, not a prototype: objects do not inherit linkage
	// from a declaration, so the keyword is written here and is the whole
	// reason `nm --extern-only` still prints one name.
	mch := e.Defn("mch_exit")
	launcher := e.Defn("main")
	if mch == nil {
		return p.Die("mch_exit()'s definition is not in the file")
	}
	if !bytes.HasSuffix(text, []byte(whim38Old)) || launcher == nil {
		return p.Die("whim-vim.c does not end with phase 38b's five-line launcher, so this is not " +
			"the file this phase was written against")
	}
	// One import for the pointer and for the launcher's half that the call
	// to vim_main() will pass (step 4).
	if _, err := e.SpliceC(graph.Frag{At: e.SpotBefore(mch), Src: "static void (*vim_host_exit)(int);\n"},
		graph.Frag{At: e.SpotBefore(launcher), Src: whim38Host}); err != nil {
		return p.Die("mch_exit()'s definition and the launcher: %v", err)
	}

	// ---- 3. the one line -------------------------------------------------
	// everything mch_exit does before it is unchanged -- the terminal is
	// restored, the screen scrolled, the memfile closed -- and only the last
	// statement moves
	body := graph.Body(mch)
	if n := len(body); n < 2 || !graph.Matches(clisp.MustPattern("(call ml_close_all TRUE)"), body[n-2]) ||
		!graph.Matches(clisp.MustPattern("(call exit r)"), body[n-1]) {
		return p.Die("mch_exit()'s tail is not `ml_close_all(TRUE); exit(r);`")
	}
	v := graph.NewVerbs("hostexit", e, io.Discard)
	v.InFunction("mch_exit", func(v *graph.Verbs) {
		v.Rewrite("(call exit ?r)", "(call vim_host_exit ?r)", 1, "mch_exit()'s tail")
	})
	if v.Err != nil {
		return v.Err
	}

	// ---- 4. the host installs it, through a parameter and not a global ---
	// the pointer is installed by the caller and is not a global the host
	// assigns, because "nothing is global but main()" is still the invariant
	vm := e.Defn("vim_main")
	if vm == nil || !graph.Matches(clisp.MustPattern("(defn static vim_main (fn ((argc int) (argv (ptr (ptr char)))) int) _*)"), vm) {
		return p.Die("vim_main()'s head, which phase 38b made, is not `static int vim_main(int argc, char **argv)`")
	}
	if _, err := e.AddParamC("vim_main", 2, "void (*exit_fn)(int)",
		func(*graph.Node) (string, graph.Bindings) { return "host_exit", nil }); err != nil {
		return p.Die("vim_main()'s third parameter: %v", err)
	}

	// ---- 5. the launcher, and the installation ---------------------------
	if _, err := e.SpliceC(graph.Frag{At: e.SpotBefore(graph.Body(vm)[0]), Src: "vim_host_exit = exit_fn;"},
		graph.Frag{At: e.SpotBody(launcher), Src: whim38Main}); err != nil {
		return p.Die("vim_main()'s installation and the launcher: %v", err)
	}

	// ---- 6. what the file is now -----------------------------------------
	if text, err = e.Graph().C(); err != nil {
		return err
	}
	if k := p.Mentions(text, "exit"); k != 4 {
		return p.Die("`exit` as a word has %d mentions after the swap, expected 4 -- the two string "+
			"literals and the goto with its label", k)
	}
	if len(whim38ExitCall.FindAll(text, -1)) != 0 {
		return p.Die("a statement still begins with `exit(`")
	}
	for _, inv := range []struct {
		Name string
		want int
		why  string
	}{
		{"vim_host_exit", 3, "its declaration, the one call in mch_exit and the one " +
			"assignment in vim_main"},
		{"host_exit", 2, "the launcher's definition and the argument main() passes.  " +
			"`vim_host_exit` is a DIFFERENT word and \\b does not match " +
			"inside it, which is why these two counts are separate"},
		{"host_jump", 3, "its declaration, the longjmp and the setjmp"},
		{"host_code", 3, "its declaration, the write in host_exit and the return in " +
			"main"},
		{"vim_main", 2, "its definition and the one call from the launcher"},
		{"main", 1, "the launcher's head, still the only bare `main` in the file"},
	} {
		if k := p.Mentions(text, inv.Name); k != inv.want {
			return p.Die("`%s` has %d mentions, expected %d -- %s", inv.Name, k, inv.want, inv.why)
		}
	}
	if err = p.AssertOnce(text, "    vim_host_exit(r);\n", "the one call through the pointer",
		"mch_exit is the only function in the file that ends the editor"); err != nil {
		return err
	}
	if err = p.AssertOnce(text, "    vim_host_exit = exit_fn;\n", "the one installation",
		"vim_main is the only function that is handed the host callback"); err != nil {
		return err
	}
	if err = p.AssertOnce(text, "    static int\nvim_main(int argc, char **argv, void (*exit_fn)(int))\n{\n    vim_host_exit = exit_fn;\n",
		"vim_main()'s head", "the callback is its third parameter, installed first"); err != nil {
		return err
	}
	if !bytes.HasSuffix(text, []byte(whim38New)) {
		return p.Die("the launcher is not the last thing in the file")
	}
	n := 0
	for _, l := range bytes.Split(text, []byte{'\n'}) {
		if bytes.HasPrefix(l, []byte("#")) {
			n++
		}
	}
	if n != nInc {
		return p.Die("the directive count moved, and the whole reason for __builtin_setjmp rather " +
			"than <setjmp.h> is that this phase adds no header")
	}
	p.Say("mch_exit ends `vim_host_exit(r);`, vim_main takes the callback as its third " +
		"parameter and installs it, and the launcher is twenty lines that land on " +
		"__builtin_setjmp and RETURN the status.  `exit` is FOUR mentions and NONE is a " +
		"call; the twelve #includes are untouched")
	return nil
}

// trimSpace is Python's str.strip() for the one place these blocks use it: the
// `what` of an anchor is the anchor's own text with its indentation and
// newline taken off.
func trimSpace(s string) string { return string(bytes.TrimSpace([]byte(s))) }
