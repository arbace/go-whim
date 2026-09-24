package p092

// Whim phase 92 -- nothing reads a byte.
// See GOAL.md.
//
// Phases 89, 90 and 91 took every way to ASK for a file: the six commands that put
// bytes on a disk, the one that takes them off it, and the five that point the
// editor at another one.  What was left of reading a file is the machinery under
// those commands -- `readfile()`, 787 lines, and the two arms of `open_buffer()`
// that call it.  This phase takes those arms, and the sweep takes the machinery.
//
// READFILE() WAS ALREADY UNREACHABLE WHEN THIS PHASE WAS HANDED THE TREE, and that
// is the whole of what makes the phase delicate rather than difficult.  Its three
// call sites are one in `read_buffer()` and two in `open_buffer()`, and
// `read_buffer`'s only callers are those same two arms; the outer arm needs
// `curbuf->b_ffname != NULL` and the inner one needs a `read_stdin` argument that,
// since phase 88 removed the file argument and the bare `-`, all four callers pass
// as FALSE.  So no input this editor can be given reaches it, and gcc keeps it only
// because it cannot prove `b_ffname != NULL` never holds.  The difference this
// phase makes is between code that cannot run and code that is not there -- and
// because it IS that, no behavioural probe can see it.  phase/092/check.go says
// what stands in for one: the input source built twice, instrumented.
//
// FOUR ANCHORS, ALL INSIDE open_buffer(), and everything else is the sweep's
// (GOALS.md core rule 1: removal is computed, not listed).  Sixteen functions go
// without one of them being named here.
//
// 1. the `if (curbuf->b_ffname != NULL) {...} else if (read_stdin) {...}` pair,
// as exact text with the blank line after it.  That is the entire cut: the two
// arms hold all three calls into the read path.
// 2. `int read_fifo = FALSE;` -- set nowhere once anchor 1 has gone, read twice.
// 3. `else if (retval == OK && !read_stdin && !read_fifo)` -> `else if (retval ==
// OK)`, which is where anchor 2's second reader was.
// 4. the signature: `open_buffer(int read_stdin, exarg_T *eap, int flags_arg)` ->
// `open_buffer(void)`, the `int flags = flags_arg;` local, and the four call
// sites, every one of which already passes `FALSE, NULL, 0`.
//
// ANCHOR 4 IS WHAT TAKES read_stdin TO ZERO, and it is measured rather than argued.
// Without it `open_buffer` keeps three parameters that nothing reads, and THE SWEEP
// CANNOT SEE THEM: tools/sweep.sh compiles with `-Wno-unused-parameter`, so an
// unused parameter is invisible where an unused local is not -- measured, anchors
// 1-3 alone leave the `int flags = flags_arg;` local deleted by the sweep's own
// unused-variable pass and `read_stdin` alive at exactly ONE mention, the parameter
// nothing reads.  Both swept files are 82,572 lines and differ in exactly five
// lines -- the signature and the four calls -- and THE TWO RECORDINGS ARE
// BYTE-IDENTICAL, as are the two binaries' sizes.  So the fold costs nothing, says
// what is true, and is taken.
//
// THERE IS NO PROTOTYPE FOR open_buffer.  It is defined above its first call, so
// the `static int open_buffer(...)` line the proto block would hold does not exist
// and a phase that edits one fails loudly.  Anchor 4 edits the definition alone.
//
// THE FURTHER FOLD IS DECLINED, DELIBERATELY.  After anchor 1, `retval` in
// `open_buffer` is `OK` from its initialiser to its return and nothing between can
// change it, so `if (retval != OK) return retval;` is dead, the function could be
// `void`, and the two `open_buffer() == FAIL` guards in the `ml_*` layer can never
// hold.  That is memline tidy and not the read path; this phase asserts `retval` at
// its 5 mentions and says it is constant, and leaves the fold to a later one.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, exactly as phase/085/edit.go and whim87- through phase/091/edit.go do it, and
// the source goes with it as $state/old.c.  The check needs BOTH: the binary is the
// left-hand side of every "this did not move" comparison, and the source is what it
// builds twice more, instrumented, for the only evidence this phase has.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim92", Edit) }

// w92Before is the file the four anchors were counted against.
var w92Before = map[string]int{
	"readfile": 5, "read_buffer": 17, "read_stdin": 23, "read_fifo": 9,
	"check_readonly": 4, "msg_scrolled_ign": 6, "filemess": 11, "read_cmd_fd": 12,
}

// w92After is what the sweep is handed, as a count rather than as trust.
var w92After = map[string]int{"readfile": 3, "read_buffer": 15, "read_stdin": 20, "read_fifo": 4}

var w92Assign = regexp.MustCompile(`\bretval\b\s*=[^=]`)

// Whim92 takes the machinery under every way to name a file: readfile(),
// read_buffer() and the message layer that reported what had been read.
//
// IT IS THE ONE PART II PHASE NO RECORDING CAN SEE, and its declared delta is
// nothing at all: readfile() was already unreachable when it ran, phases 88 to 91
// having taken every way to name a file.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "nobyte", W: w}
	var err error

	mentions := func(t []byte, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAll(t, -1))
	}
	within := func(t []byte, fn, old, new, what string, n int) ([]byte, error) {
		a, z, ok := cutil.FindDefinition(t, cutil.Blank(t), fn)
		if !ok {
			return nil, p.Die("%s is not defined", fn)
		}
		Body := string(t[a:z])
		k := strings.Count(Body, old)
		if k != n {
			return nil, p.Die("%s -- %s occurs %d times in %s, expected %d",
				what, cutil.PyRepr(edit.CoreHead(old, 60)), k, fn, n)
		}
		p.Say(what)
		return []byte(string(t[:a]) + strings.ReplaceAll(Body, old, new) + string(t[z:])), nil
	}

	// ---- 0. the shape the anchors below were counted on -----------------------
	// open_buffer AT EXACTLY 5 IS WHY THIS PHASE NEEDS SWEPT TEXT.
	if k := mentions(text, "open_buffer"); k != 5 {
		return nil, p.Die("open_buffer has %d mentions, expected 5 -- the definition and four callers, "+
			"every one of them `open_buffer(FALSE, NULL, 0)`.  On the text phase 91's "+
			"EDIT leaves there are six: do_ecmd is still there to make "+
			"`(void)open_buffer(FALSE, eap, readfile_flags);`, which anchor 4 would not "+
			"rewrite.  This phase needs swept text", k)
	}
	for _, name := range edit.SortedKeys(w92Before) {
		if k := mentions(text, name); k != w92Before[name] {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w92Before[name])
		}
	}
	p.Say("open_buffer 5, readfile 5, read_buffer 17, read_stdin 23 -- the file the four " +
		"anchors were counted against")

	// ---- 1. the two arms, which hold every call into the read path ------------
	if text, err = within(text, "open_buffer", w92Arms, "",
		"open_buffer's two read arms, 36 lines: both calls to readfile(), both "+
			"to read_buffer(), and the fifo test between them", 1); err != nil {
		return nil, err
	}
	// ---- 2. read_fifo, written nowhere now ------------------------------------
	if text, err = within(text, "open_buffer", w92lit1, "",
		"the read_fifo local: anchor 1 was its only writer", 1); err != nil {
		return nil, err
	}
	// ---- 3. the unchanged() test, where its second reader was -----------------
	if text, err = within(text, "open_buffer", w92lit2, w92lit3,
		"the unchanged() arm: !read_stdin and !read_fifo were both constantly true", 1); err != nil {
		return nil, err
	}
	// ---- 4. the signature, and the four callers -------------------------------
	if text, err = within(text, "open_buffer", w92lit4, w92lit5,
		"open_buffer(void): read_stdin, eap and flags_arg are read by nothing "+
			"now, and there is no prototype to follow", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "open_buffer", w92lit6, "",
		"the flags local, which only the deleted arms passed on", 1); err != nil {
		return nil, err
	}
	if k := strings.Count(string(text), "open_buffer(FALSE, NULL, 0)"); k != 4 {
		return nil, p.Die("open_buffer is called %d times as `open_buffer(FALSE, NULL, 0)`, expected "+
			"%d -- every caller already passes FALSE, NULL and 0, and a caller that "+
			"does not is one this fold would change", k, 4)
	}
	text = []byte(strings.ReplaceAll(string(text), "open_buffer(FALSE, NULL, 0)", "open_buffer()"))
	p.Say("the four call sites -- enter_buffer, ml_append_flags, ml_replace_len and " +
		"create_windows -- every one of which passed FALSE, NULL, 0")

	// ---- 5. what is left, and what is deliberately left -----------------------
	a, z, ok := cutil.FindDefinition(text, cutil.Blank(text), "open_buffer")
	if !ok {
		return nil, p.Die("open_buffer no longer parses as a definition")
	}
	Body := text[a:z]
	for _, gone := range []string{"read_stdin", "read_fifo", "eap", "flags", "readfile", "read_buffer"} {
		if mentions(Body, gone) > 0 {
			return nil, p.Die("%s is still named inside open_buffer", gone)
		}
	}
	assigns := len(w92Assign.FindAll(Body, -1))
	if assigns != 1 || mentions(Body, "retval") != 5 {
		return nil, p.Die("retval is assigned %d times in open_buffer and mentioned %d: this phase "+
			"leaves exactly one assignment, the initialiser, and 5 mentions",
			assigns, mentions(Body, "retval"))
	}
	p.Sayf("open_buffer is %d lines and calls nothing that reads: retval is OK from its "+
		"initialiser to its return, so `if (retval != OK) return retval;` is dead and "+
		"the two ml_ guards can never hold -- left for a later tidy, not folded here",
		strings.Count(string(Body), "\n"))

	for _, name := range edit.SortedKeys(w92After) {
		if k := mentions(text, name); k != w92After[name] {
			return nil, p.Die("%s has %d mentions after the cut, expected %d", name, k, w92After[name])
		}
	}
	p.Say("readfile 5 -> 3 and read_buffer 17 -> 15, and the survivors are not calls: a " +
		"prototype, two definitions and fourteen mentions of readfile's own local of " +
		"the same name.  read_buffer and fix_help_buffer are the two entry points the " +
		"sweep starts from, and they are exactly the two -Wunused-function warnings " +
		"this text produces")
	return text, nil
}
