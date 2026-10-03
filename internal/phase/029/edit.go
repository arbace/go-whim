package p029

// Whim phase 29 (formerly 90) -- the editor loses every way to read a file.  See GOAL.md.
//
// The other half of taking the filesystem away.  Phase 28 removed the six commands
// that put bytes on a disk; this one removes the command that takes them off it,
// `:read`, and with it the `:r !cmd` arm -- the last caller of the filter and shell
// plumbing whim left as stubs.  What remains of reading a file is `readfile()`
// itself, which the startup path still uses and which is the "nothing reads a byte"
// phase's (GOALS.md II.3b P8); this phase asserts that it is untouched, by count.
//
// THREE ANCHORS, AND ONE FOLD THAT IS A JUDGEMENT.  Everything else is the sweep's
// (GOALS.md core rule 1: removal is computed, not listed), and six functions go
// without one of them being named here:
//
// 1. the `CMD_read` enumerator of `enum CMD_index`, one line;
// 2. the `cmdnames[]` row, one physical line, designated `[CMD_read] = {`;
// 3. `do_one_cmd`'s `if (ea.cmdidx == CMD_read) {...}` -- the parse that turns
// `:r!` and `:r !cmd` into a filter -- deleted as TEXT rather than folded,
// because its condition names the enumerator that is going.  It must go in the
// same edit as anchor 1 or nothing declares what it reads.
//
// THE JUDGEMENT: `exarg_T.usefilter`.  Phase 28 removed one of its two writers
// (`:w >>` and `:w !cmd`) and anchor 3 removes the other, so after this edit the
// field is WRITTEN NOWHERE -- and `do_one_cmd` memsets the struct, so every reader
// is constantly FALSE.  No tool here can see that: tools/deadfields.py removes a
// field nothing NAMES, and gcc has no warning for a struct member that is only
// read.  So the six readers are folded by hand, and the field, read then only in
// ex_read, goes in the sweep after it -- phase 27's argument for `exmode_active`
// in a smaller shape.  Measured on this
// input: folding costs 13 lines and gives a BYTE-IDENTICAL recording -- the fold
// changes no behaviour at all, it removes a test whose answer was already fixed.
//
// The alternative, leaving the field, was measured too and is a worse tree for the
// same 209 lines: `usefilter` would survive as a member nothing writes, seven tests
// of it would survive as dead branches, and the next phase to read this file would
// have to work out for itself that they can never be taken.
//
// THE TEXT THIS EDIT LEAVES DOES NOT COMPILE, and that is stated here because
// nothing else would say it.  `CMD_read` survives the cut in `ex_read` -- the
// function whose only reference was the row that just went -- and the enumerator
// it names is gone.  The invariant at the end is the honest form of that,
// computed rather than listed: every surviving mention is inside a function
// definition, and no surviving `cmdnames[]` row names that function, which is the
// whole argument that funcreach.py takes it in the sweep's first round.
//
// NO HANDLER IS DELETED BY NAME.  The row is the only reference a command handler
// has, so taking the row is what makes `ex_read` unreachable, and `do_bang`,
// `do_shell`, `do_filter`, `check_secure` and `prevcmd_is_set` follow it -- `:!` has
// not existed since whim, and phase 28 swept `ex_write`, which held `do_bang`'s other
// call (`:w !cmd`).  The check records the six as a measurement of what the sweep
// did.
//
// THE ROW FLOOR.  `cmdnames[]` goes 105 -> 104 rows, and create_cmdidxs's `names()`
// refuses a table of fewer than 100 -- a regex that stops matching otherwise yields
// a plausible all-zero index, so the floor is deliberate.  `zexcmds`
// enumerates the table through it, so crossing the floor would stop the core's command
// sweep rather than give a wrong answer.  After this phase the margin is FOUR rows,
// and GOALS.md II.3a gives it to the `:edit` phase, which must lower the floor.
//
// NO ENUMERATOR DUMP, for phase 28's reason.  Deleting one renumbers 46 survivors and
// every one is a `CMD_*`: `cmdnames[]` is DESIGNATED, so a row lands at its own
// enumerator whatever the numbering is, the `static_assert` on the row count catches
// a dropped pair, and all 104 surviving names are dispatched by `zexcmds`
// inside the declared delta.  There is no derived first-two-letters index in this
// file -- whim's Phase 26 took it with the 489 stub rows -- so nothing else depends
// on a position.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, exactly as internal/phase/004/e/edit.go, internal/phase/027/edit.go, internal/phase/archive/088/edit.go and
// `zcases`'s 102 cases types its own text and names no file, so `cmd_read`
// types `:read` with no file name and has only ever recorded `E32: No file name`.
// The only evidence that this phase removed reading rather than one error message is
// a probe that requires the OLD binary to pull a file off the disk, and that needs
// the old binary.  The source goes with it, as $state/old.c, for the before-and-
// after counts the check takes.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  The same acts on the
// program's graph, the report the text version's (history keeps it, with
// editlit.go's literal): the anchors and the writes of usefilter are the
// text's own counts on the C view (TEXTQ); the enumerator is RENUM's; the
// `:r!` parse is the one if asking for CMD_read, cut; each operand the text
// took out of a condition is DropOperand, or, where the parentheses went
// with it, the expression they held moved into the `&&`'s place.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim29", Edit) }

// w29Dying are the names whose survivors are the reason the text does not
// compile yet.  `usefilter` is not among them: its member declaration is still
// there when the edit returns, and the sweep takes it with ex_read's last read.
var w29Dying = []string{"CMD_read"}

var w29Assign = regexp.MustCompile(`\busefilter\s*=`)

// Whim29 takes the way to read a file: `:read`, its `:r !cmd` arm, and the
// exarg_T.usefilter field that nothing writes once both `:w !` and `:r !` are
// gone.
//
// STEP 4 IS THE JUDGEMENT OF THIS PHASE and the one thing here no tool could
// have found: a struct member that is READ and never written draws no warning,
// and deadfields.py removes only a member nothing names.  do_one_cmd memsets
// `ea`, so every test folded there is constantly FALSE.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noread", e, w)

	t := v.Text()
	for _, b := range []struct {
		Name string
		want int
	}{{"CMD_read", 2}, {"ex_read", 0}, {"open_buffer", 5}, // enter_buffer's went at phase 1 (quitfront)
		{"read_buffer", 0}, {"readfile", 0}, {"usefilter", 9}} {
		if k := edit.MentionCount(t, b.Name); k != b.want {
			v.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", b.Name, k, b.want)
			return v.Done()
		}
	}
	if writes := len(w29Assign.FindAll(t, -1)); writes != 2 {
		v.Die("usefilter is assigned %d times, expected the 2 that anchor 3 removes -- "+
			"phase 28 took the other two with `:w >>` and `:w !cmd`", writes)
		return v.Done()
	}
	v.Say("CMD_read 2 mentions, usefilter 9 -- the field, the two writes anchor 3 " +
		"removes and six reads")

	v.DeleteEnumerators(w29Dying, graph.Renumber, "the CMD_read enumerator of enum CMD_index")

	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.Cut("(if (== (. ea cmdidx) CMD_read) _*)", 1,
			"do_one_cmd no longer parses `:r!` or `:r !cmd`: usefilter loses its last two writes")
	})
	if !v.Failed() && w29Assign.Match(v.Text()) {
		v.Die("usefilter is still assigned after anchor 3, so the fold below would be wrong")
	}
	// `(X) && !ea.usefilter` is X, its parentheses gone with the operand
	unparen := func(m *graph.Node, _ graph.Bindings) ([]*graph.Node, error) {
		return []*graph.Node{m.Kids[1].Kids[1]}, nil
	}
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.RewriteFunc("(&& (paren (& (. ea argt) EX_CMDARG)) (! (. ea usefilter)))", 1, unparen,
			"EX_CMDARG takes its argument command whatever the (dead) filter flag said")
		v.RewriteFunc("(&& (paren (& (. ea argt) EX_TRLBAR)) (! (. ea usefilter)))", 1, unparen,
			"and EX_TRLBAR separates a trailing command")
		v.DropOperand("(. ea usefilter)", 1,
			"and only :global and :vglobal keep a backslash-newline in their argument")
	})
	v.InFunction("expand_filename", func(v *graph.Verbs) {
		v.RewriteFunc("(&& (! (-> eap usefilter)) (! escaped))", 1,
			func(m *graph.Node, _ graph.Bindings) ([]*graph.Node, error) { return []*graph.Node{m.Kids[2]}, nil },
			"expand_filename escapes a replacement unless it was escaped already")
		v.FoldNeverAt(graph.IfNotArm, "(&& (-> eap usefilter) _*)", 1,
			"and no longer escapes `!` for a shell, which only a filter needed")
		v.RewriteFunc("(&& (paren (& (-> eap argt) EX_NOSPC)) (! (-> eap usefilter)))", 1, unparen,
			"and EX_NOSPC refuses a second file name whatever it said")
	})
	if v.Failed() {
		return v.Done()
	}

	left, holders, _, err := vimtext.CoreResidue(edit.Ph{Tag: "noread", W: w}, v.Text(), w29Dying)
	if err != nil {
		return err
	}
	s := "s"
	if left == 1 {
		s = ""
	}
	v.Sayf("%d mention%s of %s left, inside %s, and no surviving row names it: the text "+
		"does not compile until the collection has run, and phasecheck is where "+
		"that is asserted", left, s, strings.Join(w29Dying, " and "), strings.Join(holders, ", "))
	return v.Done()
}
