package p031

// Whim phase 31 (formerly 92) -- nothing reads a byte.
// See GOAL.md.
//
// Phases 28, 29 and 30 took every way to ASK for a file: the six commands that put
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
// since record 88 removed the file argument and the bare `-`, all four callers pass
// as FALSE.  So no input this editor can be given reaches it, and gcc keeps it only
// because it cannot prove `b_ffname != NULL` never holds.  The difference this
// phase makes is between code that cannot run and code that is not there -- and
// because it IS that, no behavioural probe can see it.  internal/phase/031/check.go says
// what stands in for one: the input source built twice, instrumented.
//
// ANCHOR 1 IS PHASE 1'S NOW: the two arms fold never at the front (readfront,
// internal/cut/extable.go, the pipeline reform's move of this phase), so
// readfile(), read_buffer() and everything only they reached are gone before
// this phase runs, and every edit phases 9-30 made inside them with them.
// What is left here is anchors 2-4, on the counts restated on q030.
//
// FOUR ANCHORS, ALL INSIDE open_buffer(), and everything else is the sweep's
// (GOALS.md core rule 1: removal is computed, not listed).  Sixteen functions go
// without one of them being named here.
//
// 1. the `if (curbuf->b_ffname != NULL) {...} else if (read_stdin) {...}` pair,
// as exact text with the blank line after it.  That is the entire cut: the two
// arms hold all three calls into the read path.
// 2. `read_fifo` -- set nowhere once anchor 1 has gone, read twice; its local
// `int read_fifo = FALSE;` is left to the sweep.
// 3. `else if (retval == OK && !read_stdin && !read_fifo)` -> `else if (retval ==
// OK)`, which is where anchor 2's second reader was.
// 4. the signature: `open_buffer(int read_stdin, exarg_T *eap, int flags_arg)` ->
// `open_buffer(void)`, and the four call sites, every one of which already
// passes `FALSE, NULL, 0`.  The `int flags = flags_arg;` local, unused, names a
// parameter that is gone until the sweep takes it.
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
// flags, exactly as internal/phase/004/e/edit.go and whim27- through internal/phase/030/edit.go do it, and
// the source goes with it as $state/old.c.  The check needs BOTH: the binary is the
// left-hand side of every "this did not move" comparison, and the source is what it
// builds twice more, instrumented, for the only evidence this phase has.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  The same acts on the
// program's graph, the report the text version's (history keeps it, with
// editlit.go's literals): the counts are the text's own regular expressions
// on the C view (TEXTQ), the file's or open_buffer's; the unchanged() arm
// is the `&&` rewritten to its first operand; and the three parameters are
// PARAM's, one edit -- the definition and the three calls, every one of
// which must pass FALSE, nullptr and 0 as the text required -- reported as
// the text's two literals.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim31", Edit) }

// w31Before is the file the three anchors were counted against.  The two
// read arms went at phase 1 (readfront, this phase's move), and readfile(),
// read_buffer() and what only they reached with them: what is left here is
// what that leaves constant.
var w31Before = map[string]int{
	"readfile": 0, "read_buffer": 0, "read_stdin": 2, "read_fifo": 2,
	"check_readonly": 0, "msg_scrolled_ign": 2, "filemess": 0, "read_cmd_fd": 12,
}

// w31After is what the sweep is handed, as a count rather than as trust.
var w31After = map[string]int{"readfile": 0, "read_buffer": 0, "read_stdin": 0, "read_fifo": 1}

var w31Assign = regexp.MustCompile(`\bretval\b\s*=[^=]`)

// Whim31 takes the machinery under every way to name a file: readfile(),
// read_buffer() and the message layer that reported what had been read.
//
// IT IS THE ONE PART II PHASE NO RECORDING CAN SEE, and its declared delta is
// nothing at all: readfile() was already unreachable when it ran, record 88 and phases 28 to 30
// having taken every way to name a file.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nobyte", e, w)

	mentions := func(t []byte, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAll(t, -1))
	}

	t := v.Text()
	if k := mentions(t, "open_buffer"); k != 4 {
		v.Die("open_buffer has %d mentions, expected 4 -- the definition and three callers, "+
			"every one of them `open_buffer(FALSE, nullptr, 0)`.  On the text phase 30's "+
			"EDIT leaves there are six: do_ecmd is still there to make "+
			"`(void)open_buffer(FALSE, eap, readfile_flags);`, which anchor 4 would not "+
			"rewrite.  This phase needs swept text", k)
		return v.Done()
	}
	for _, name := range edit.SortedKeys(w31Before) {
		if k := mentions(t, name); k != w31Before[name] {
			v.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w31Before[name])
			return v.Done()
		}
	}
	v.Say("open_buffer 4, read_stdin 2, read_fifo 2, and readfile and read_buffer gone " +
		"at phase 1 -- the file the three anchors were counted against")

	v.InFunction("open_buffer", func(v *graph.Verbs) {
		v.Rewrite("(&& ?ok (! read_stdin) (! read_fifo))", "?ok", 1,
			"the unchanged() arm: !read_stdin and !read_fifo were both constantly true")
	})
	if v.Failed() {
		return v.Done()
	}
	calls := v.Find("(call open_buffer _*)")
	if k := v.Count("(call open_buffer FALSE nullptr 0)"); k != 3 || len(calls) != 3 {
		v.Die("open_buffer is called %d times as `open_buffer(FALSE, nullptr, 0)`, expected "+
			"%d -- every caller already passes FALSE, nullptr and 0, and a caller that "+
			"does not is one this fold would change", k, 3)
		return v.Done()
	}
	var drops []graph.ParamDrop
	for _, p := range []string{"read_stdin", "eap", "flags_arg"} {
		drops = append(drops, graph.ParamDrop{Decl: e.Defn("open_buffer"), I: e.ParamIndex("open_buffer", p)})
	}
	if _, err := e.DropParams(drops, graph.ParamOptions{}); err != nil {
		v.Die("open_buffer(void) -- %v", err)
		return v.Done()
	}
	v.Say("open_buffer(void): read_stdin, eap and flags_arg are read by nothing " +
		"now, and there is no prototype to follow")
	v.Say("the three call sites -- ml_append_flags, ml_replace_len and create_windows " +
		"-- every one of which passed FALSE, nullptr, 0")

	v.InFunction("open_buffer", func(v *graph.Verbs) {
		body := v.Text()
		for _, gone := range []string{"read_stdin", "eap", "readfile", "read_buffer"} {
			if mentions(body, gone) > 0 {
				v.Die("%s is still named inside open_buffer", gone)
				return
			}
		}
		assigns := len(w31Assign.FindAll(body, -1))
		if assigns != 1 || mentions(body, "retval") != 5 {
			v.Die("retval is assigned %d times in open_buffer and mentioned %d: this phase "+
				"leaves exactly one assignment, the initialiser, and 5 mentions",
				assigns, mentions(body, "retval"))
			return
		}
		v.Sayf("open_buffer is %d lines and calls nothing that reads: retval is OK from its "+
			"initialiser to its return, so `if (retval != OK) return retval;` is dead and "+
			"the two ml_ guards can never hold -- left for a later tidy, not folded here",
			strings.Count(string(body), "\n"))
	})
	if v.Failed() {
		return v.Done()
	}

	t = v.Text()
	for _, name := range edit.SortedKeys(w31After) {
		if k := mentions(t, name); k != w31After[name] {
			v.Die("%s has %d mentions after the cut, expected %d", name, k, w31After[name])
			return v.Done()
		}
	}
	v.Say("read_stdin 2 -> 0, and read_fifo 2 -> 1: its declaration, which the collection takes")
	return v.Done()
}
