package p101

// Whim phase 101 -- main() is demoted to vim_main().  See GOAL.md.
//
// The editor's entry point stops being the program's entry point.  What was
//
// int
// main
// (int argc, char **argv)
// {
// ...
// return vim_main2();
// }
//
// becomes `static int vim_main(int argc, char **argv)` with the SAME BODY, and a new
// five-line `main()` at the bottom of the file whose whole content is
// `return vim_main(argc, argv);`.  Nothing else moves.  This is GOALS.md II.4c's
// first step, and it is deliberately the ONLY thing this phase does: the host
// boundary is a sequence of small demotions and this is the one that names them.
//
// BOTH STAY IN whim-vim.c, AND THAT IS THE POINT RATHER THAN A COMPROMISE.  Two
// tools hard-code today's invariant -- tools/phasecheck.sh's `grep -v '^main$'` and
// tools/funcreach.py's `{'main'}` root -- and splitting the launcher into a second
// translation unit is what breaks both.  Measured while `exit` was being reviewed:
// ONE APPENDED LINE to tools/phasecheck.sh moves 118 implementation keys (12 whim
// stages, 82 whim edits, 12 Part II units, 12 Part II edits, and no slim key).  So every
// demotion that CAN be done inside one file is done inside one file, and the split
// happens once, late, when there is nothing left to do before it.
//
// `vim_main` IS static, and that is what keeps the invariant exact.  The check asserts
// `nm --extern-only --defined-only` prints `main` and nothing else, which it has for
// every Part II phase; a non-static `vim_main` would be the first Part II phase ever to add
// an external symbol, and it would do it for no reason -- nothing outside this file
// calls it yet.
//
// THE NAME WAS CHECKED FOR A COLLISION BEFORE IT WAS CHOSEN.  `vim_main2()` already
// exists in this file -- it is upstream's, the second half of the old main() split at
// the point where the screen is up -- and `vim_main` is a DIFFERENT identifier, not a
// prefix collision: C has no such thing.  The assertions below pin `vim_main2` at its
// two mentions and require `vim_main` to have had NONE before this phase, both as
// whole words, so the two cannot be confused by a substring grep either.
//
// THE HEAD WAS A FOSSIL AND PHASE 0 RETIRED IT.  main()'s head used to be spelled over
// THREE lines --
//
// int
// main
// (int argc, char **argv)
//
// -- which is upstream's, where the name and the argument list were separated by an
// `#ifdef` that gave MS-Windows a different signature.  The conditional went with the
// preprocessor in slim's phase 5 and the line break stayed.  The canonical text phase 0
// prints has one shape per construct, so the break is gone before this phase is
// handed anything: the head arrives in the ordinary two-line shape every other
// function here has, `vim_main` keeps it and the new `main` gets it too.  What was a
// real consequence of the fossil is now simply true of the file -- tools/funcreach.py's
// definition finder matches every head, `main` among them, so this phase adds TWO
// definitions to what it counts and one of them is `main` itself.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).  The
// input binary is kept because the check probes the exit statuses of BOTH.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"bytes"
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim101", Edit) }

// head is upstream's #ifdef'ed signature with the conditional gone: the name
// sits on a line of its own because there was a directive between it and the
// argument list, and slim's phase 5 took the conditional and left the break.
const whim101Head = "\n    int\nmain(int argc, char **argv)\n{\n"

const whim101NewHead = "\n    static int\nvim_main(int argc, char **argv)\n{\n"

// The launcher.  Five lines, and every one of them is what GOALS.md II.4c
// says the host file will hold: it calls the editor and it does nothing else.
//
// No prototype is written for vim_main -- it is DEFINED above its only call,
// so a declaration would be one the sweep is entitled to delete.
const whim101Launch = "\n" +
	"    int\n" +
	"main(int argc, char **argv)\n" +
	"{\n" +
	"    return vim_main(argc, argv);\n" +
	"}\n"

// Whim101 demotes main() to a static vim_main() and appends a launcher, both
// still in the one file.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "demote", W: w}
	linesBefore := p.Lines(text)
	runsBefore := p.BlankRuns(text)

	// ---- 1. the name is free, and `vim_main2` is not it ------------------
	// C has no prefix collisions, but a reader greps, and so does this file's
	// own machinery.
	if k := p.Mentions(text, "vim_main"); k != 0 {
		return nil, p.Die("`vim_main` already has %d mentions as a whole word -- the name this phase "+
			"gives the editor is taken", k)
	}
	if k := p.Mentions(text, "vim_main2"); k != 2 {
		return nil, p.Die("`vim_main2` has %d mentions, expected 2 -- its definition and the one call at "+
			"the bottom of main().  It is upstream's second half of main(), it is NOT the "+
			"name this phase introduces, and pinning it is what keeps a substring grep "+
			"from confusing the two", k)
	}
	if err := p.AssertOnce(text, "    static int\nvim_main2(void)\n{\n", "vim_main2()'s definition",
		"it is the function the new vim_main() ends by calling, and it does not move"); err != nil {
		return nil, err
	}
	p.Say("`vim_main` is free -- ZERO mentions as a whole word -- and `vim_main2` is the two " +
		"it has always had: upstream's second half of main(), which this phase does not " +
		"touch.  They are different identifiers and C has no prefix collision")

	// ---- 2. main() is where and what this phase was written against ------
	if err := p.AssertOnce(text, whim101Head, "main()'s head",
		"two lines, the shape every other function here has: the fossil break between "+
			"the name and the argument list, which upstream put an #ifdef in, is one of "+
			"the things the canonical text no longer writes"); err != nil {
		return nil, err
	}
	if k := p.Mentions(text, "main"); k != 1 {
		return nil, p.Die("`main` as a whole word has %d mentions, expected 1 -- the definition below is "+
			"the ONLY place this file writes the bare word.  `main_loop`, `main_errors`, "+
			"`vim_main2` and `domain` are different words and \\b does not match inside "+
			"them", k)
	}
	if !bytes.HasSuffix(text, []byte("    return vim_main2();\n}\n")) {
		return nil, p.Die("whim-vim.c does not end with main()'s `return vim_main2();` and its closing " +
			"brace -- CLAUDE.md states that as the shape of this file, and this phase " +
			"appends after it")
	}
	p.Say("main() is the last function in the file, its head spelled over two lines, and " +
		"`main` as a whole word occurs ONCE in 80,000 lines -- that one head.  " +
		"`main_loop` and `main_errors` are different words")

	// ---- 3. the demotion: one head rewritten, one function appended ------
	text = bytes.Replace(text, []byte(whim101Head), []byte(whim101NewHead), 1)
	text = append(text, []byte(whim101Launch)...)

	// ---- 4. what the file is now -----------------------------------------
	if err := p.AssertOnce(text, "    static int\nvim_main(int argc, char **argv)\n{\n", "vim_main()'s head",
		"the editor entry point, static, in the two-line shape every other function here "+
			"uses"); err != nil {
		return nil, err
	}
	if err := p.AssertOnce(text, whim101Launch, "the launcher",
		"it is appended once and it is the last thing in the file"); err != nil {
		return nil, err
	}
	if !bytes.HasSuffix(text, []byte(whim101Launch)) {
		return nil, p.Die("the launcher is not the last thing in the file")
	}
	if k := p.Mentions(text, "vim_main"); k != 2 {
		return nil, p.Die("`vim_main` has %d mentions, expected 2 -- its definition and the one call from "+
			"main().  A third would be a prototype, and a prototype for a function defined "+
			"above its only call is redundant", k)
	}
	if p.Mentions(text, "vim_main2") != 2 {
		return nil, p.Die("`vim_main2` moved, and this phase does not touch it")
	}
	if k := p.Mentions(text, "main"); k != 1 {
		return nil, p.Die("`main` as a whole word has %d mentions after the demotion, expected 1 -- the "+
			"launcher's head and nothing else.  `vim_main` and `vim_main2` are different "+
			"words", k)
	}
	// The old head was rewritten, not copied: `main(int argc` is the launcher's
	// head and nothing else.  It replaces the assertion that the fossil head is
	// gone, which the canonical text makes true before this phase begins and so
	// could no longer fail.
	if k := bytes.Count(text, []byte("\nmain(int argc")); k != 1 {
		return nil, p.Die("a head beginning `main(int argc` occurs %d times, expected 1 -- the "+
			"launcher's.  The newline is what tells it from `vim_main(int argc`, which is "+
			"the definition this phase just renamed", k)
	}
	if r := p.BlankRuns(text); r != runsBefore {
		return nil, p.Die("the demotion left %d runs of two blank lines where there were %d", r, runsBefore)
	}
	if n := p.Lines(text); n != linesBefore+6 {
		return nil, p.Die("the file gained %d lines, expected 6 -- the head keeps its two lines "+
			"and the launcher is six", n-linesBefore)
	}
	p.Say("main() is now `static int vim_main(int argc, char **argv)` with the same body, and " +
		"the last six lines of the file are a launcher whose whole content is `return " +
		"vim_main(argc, argv);`.  +6 lines: the head is the shape it already had, the " +
		"launcher added six")
	return text, nil
}
