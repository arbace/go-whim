package p030

// Whim phase 30 (formerly 91) -- the editor loses every way to name something else to edit.
// See GOAL.md.
//
// Phases 28 and 29 took the commands that put bytes on a disk and the one that takes
// them off it.  This one takes the commands that point the editor AT a file --
// `:edit :enew :ex :visual :view` -- and the four Normal-mode keys that do the same
// thing from the buffer's own text, `gf gF [f ]f`.  What is left of opening
// anything is `readfile()` and `open_buffer()`, which the startup path still uses
// and which are the "nothing reads a byte" phase's (GOALS.md II.3b P8); this phase
// asserts by count that both are untouched.
//
// WHAT THE FIVE COMMANDS ACTUALLY WERE, measured on the input binary in a directory
// holding a file called `keys`: `do_exedit` is thirty lines -- a lock guard, a
// `readonlymode` save/set/restore testing CMD_view and CMD_enew, `setpcmark()` and
// one `do_ecmd()` call.  So `:ex` and `:visual` are `:edit` spelled differently
// (their Ex-mode escape has had nothing to escape from since phase 27), `:view` is
// `:edit` with `'readonly'` set, and `:enew` is `:edit` with a NULL file name.  One
// handler, `ex_edit`, is all five rows, which is why they go together.
//
// SIX ANCHORS, and everything else is the sweep's (GOALS.md core rule 1: removal is
// computed, not listed).  Sixteen functions go without one of them being named
// here, seventeen with anchor 6:
//
// 1. five enumerators of `enum CMD_index`, one line each;
// 2. five `cmdnames[]` rows, one physical line each, designated `[CMD_x] = {`;
// 3. `do_one_cmd`'s `curbuf_locked()` exemption, which names `CMD_edit` in a
// conjunct -- `CMD_file` STAYS, being the `:file` phase's.  It must go in the
// same edit as anchor 1 or nothing declares what it reads, and it is the one
// anchor outside the table and the keys: an edit shaped like the table forgets
// it, and the build is what catches that.
// 4. `nv_g_cmd`'s `case 'f': case 'F': nv_gotofile(cap); break;` arm.  `gf` and
// `gf` fall to `default: clearopbeep` with the rest of the unused `g` keys.
// 5. `nv_brackets`'s `if (cap->nchar == 'f') { nv_gotofile(cap); } else { ... }`,
// where the else is the whole rest of the function.  `[f` and `]f` fall into
// the chain that ends in `clearopbeep`.
// 6. `do_one_cmd`'s `if (ea.argt & EX_ARGOPT) { while (... getargopt(&ea) ...) }`.
//
// NO `nv_cmds[]` ROW IS TOUCHED, and that is the hazard this phase does not have.
// There is no row for `gf`, `gf`, `[f` or `]f`: they are arms inside two handlers
// whose `g`, `[` and `]` rows dispatch dozens of other keys, so nothing is deleted
// from the table and nothing renumbers.  The check presses fifty of those keys on
// both binaries and requires exactly four to move.
//
// ANCHOR 5 IS NOT A `cutil.fold_never`, AND THE REASON IS INDENTATION.  fold_never
// keeps an `else` body by dedenting it four columns, which is right when the body
// was written one level in.  This one was not: upstream's `else` here has no braces
// at all (the `if` is inside `#ifdef FEAT_SEARCHPATH`), so slim's bracing pass put
// a `{`/`}` round the rest of the function and left every line at the function's
// own four columns.  Dedenting would put forty lines at column zero and no tool
// here would ever say so -- CLAUDE.md's "one thing no tier can see".  So the head
// and the matching closer are deleted as counted text, by brace matching, and the
// body keeps the indentation it already had.
//
// ANCHOR 6 IS WORTH ITS LINES, and it is measured rather than argued.  `EX_ARGOPT`
// -- `++ff=`, `++enc=`, `++bin`, `++edit` -- was on five rows: `:read`, which phase
// 29 took, and these four.  After anchor 2 it is on NONE, so the block can never be
// entered, `getargopt()` can never run, and `exarg_T.read_edit` is written by
// nothing and read by nothing.  Deleting the block hands all three to the sweep.
// Measured: 30 lines, one more function, and a recording BYTE-IDENTICAL to the one
// the five anchors alone produce -- `++edit` was only ever accepted by the commands
// this phase removes, so there is nothing to declare.
//
// `EX_CMDARG` REACHES ZERO ROWS TOO AND IS LEFT ALONE, deliberately.  Its two
// fields are `do_ecmd_cmd`, which after this phase has four mentions and no writer,
// and `do_ecmd_lnum`, which has two -- and `do_ecmd_lnum` is written through
// `eval_vars()`, which is the buffer-name phase's.  Folding round that is that
// phase's to do; this one names the counts so that a later widening has to move
// them.
//
// `'undoreload'` STAYS, AND THE ROW IS NOT THIS PHASE'S.  `p_ur` has three mentions
// -- the declaration, one reader inside `do_ecmd` and the option row -- and after
// this phase two and no reader.  Removing the row would change what `:set ur?`
// answers, which nothing here sweeps, so the delta could not be checked; and
// ``orphanopts`` refuses the opposite direction, a global whose row has
// gone.  The check asserts `p_ur` at exactly 2 with its row intact, and the
// manifest carries `uses options:94 files:91 mechanical` for the phase that takes it.
//
// THE ROW FLOOR IS CROSSED HERE, AND THE FLOOR MOVES IN THIS COMMIT.  `cmdnames[]`
// goes 104 -> 99 and create_cmdidxs's `names()` refused a table of fewer than 100 --
// not with "too few rows" but with `no command table found in either shape`, because
// names() tries both parsers with check=False and neither answer clears the bar.
// `zexcmds` enumerates the core's whole Ex sweep through names(), so the old
// floor would have stopped the sweep, tools/st.sh delta, the recording and every
// later phase's check rather than giving a wrong answer.  GOALS.md II decision 8:
// lowered deliberately, to 80, in the phase that crosses it and in the same commit,
// with the reason in the tool's own docstring.  The margin is 19 rows and the next
// row the plan removes is `:file`'s.
//
// THE TEXT THIS EDIT LEAVES DOES NOT COMPILE, as phases 28 and 29 leave theirs, and
// the invariant at the end is the honest form of that, computed rather than listed:
// every surviving mention of a deleted enumerator is inside a function definition,
// and no surviving `cmdnames[]` row names that function -- which is the whole
// argument that funcreach.py takes it in the sweep's first round.
//
// NO HANDLER IS DELETED BY NAME.  The row is the only reference a command handler
// has, so taking the five rows is what makes `ex_edit` unreachable, and `do_exedit`,
// `do_ecmd` and thirteen more follow it.  The check records the sixteen as a
// measurement of what the sweep did.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, exactly as internal/phase/004/e/edit.go, internal/phase/027/edit.go, internal/phase/archive/088/edit.go, internal/phase/028/edit.go
// and internal/phase/029/edit.go do it.  THE CORPUS SEES TWO CASES OF THIS PHASE and neither of
// them opens a file: `cmd_edit` types `:edit` with no file name and `key_gf` presses
// `gf` on a word that names nothing.  The only evidence that this phase removed
// opening a file rather than two error messages is a probe that requires the OLD
// binary to pull one off the disk, and that needs the old binary.  The source goes
// with it, as $state/old.c, for the before-and-after counts the check takes.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  The same acts on the
// program's graph, the report the text version's (history keeps it, with
// editlit.go's literals): the anchors, the rows and the residue are the
// text's own questions on the C view (TEXTQ); the enumerators are RENUM's;
// `:edit`'s exemption is an operand dropped; `gf`/`gF` are their two case
// labels and the run they head; `[f`/`]f` is the if whose then is the call
// of nv_gotofile, folded never, its else the rest of the function; `++opt`
// is the if asking for EX_ARGOPT, cut.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim30", Edit) }

// w30Going are the five Ex commands that name another file to edit.  They are
// ONE handler; `gf gF [f ]f` are arms inside two surviving handlers and not
// nv_cmds[] rows, which is why anchors 4 and 5 are text and not table edits.
var w30Going = []string{"CMD_edit", "CMD_enew", "CMD_ex", "CMD_visual", "CMD_view"}

var w30Before = map[string]int{
	// the five rows, ex_edit and do_exedit went at phase 1 (filefront, D4)
	"CMD_edit": 2, "CMD_enew": 0, "CMD_ex": 0, "CMD_view": 0, "CMD_visual": 0,
	"ex_edit": 0, "do_exedit": 0, "nv_gotofile": 3,
	"EX_ARGOPT": 2, "getargopt": 3, "read_edit": 2,
	// readfile went at phase 1 (readfront, phase 31's move)
	"readfile": 0, "open_buffer": 5, "p_ur": 0, // its row went at phase 1, and its reads fell out (D3)
}

// Whim30 takes every way to name another file to edit.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noedit", e, w)

	t := v.Text()
	for _, name := range edit.SortedKeys(w30Before) {
		if k := edit.MentionCount(t, name); k != w30Before[name] {
			v.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w30Before[name])
			return v.Done()
		}
	}
	for _, r := range vimtext.CoreRows(t) {
		if strings.Contains(string(r), "EX_ARGOPT") {
			v.Die("a cmdnames[] row still carries EX_ARGOPT")
			return v.Done()
		}
	}
	v.Say("no cmdnames[] row carries EX_ARGOPT; readfile 5 and open_buffer 6 -- the " +
		"line against the byte-reader phase")

	var named []string
	for _, n := range w30Going {
		if edit.MentionCount(t, n) > 0 {
			named = append(named, n)
		}
	}
	v.DeleteEnumerators(named, graph.Renumber,
		"the enumerators of "+strings.Join(w30Going, " ")+" that are still named")

	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.DropOperand("(!= (. ea cmdidx) CMD_edit)", 1,
			"do_one_cmd no longer exempts :edit from the curbuf_locked() refusal, and :file still is")
	})
	v.InFunction("nv_g_cmd", func(v *graph.Verbs) {
		v.Muted(func(v *graph.Verbs) {
			v.DropCase("(case 'f')", 1, "nv_g_cmd's `gf` and `gF` arm")
			v.DropCase("(case 'F')", 1, "nv_g_cmd's `gf` and `gF` arm")
		})
		v.Say("nv_g_cmd's `gf` and `gF` arm")
	})
	v.InFunction("nv_brackets", func(v *graph.Verbs) {
		v.One("(if (== (-> cap nchar) 'f') (block (call nv_gotofile cap)) (block _*))", "nv_brackets: the `[f` head")
		v.FoldNeverAt(graph.IfNotArm, "(== (-> cap nchar) 'f')", 1,
			"nv_brackets's `[f` and `]f` arm, keeping the else that is the rest of the "+
				"function at the indentation it already had")
	})
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.Cut("(if (& (. ea argt) EX_ARGOPT) _*)", 1,
			"do_one_cmd no longer parses `++opt`: EX_ARGOPT is on no row, so "+
				"getargopt() is unreachable and read_edit is written by nothing")
	})
	if v.Failed() {
		return v.Done()
	}

	left, holders, found, err := vimtext.CoreResidue(edit.Ph{Tag: "noedit", W: w}, v.Text(), w30Going)
	if err != nil {
		return err
	}
	s := "s"
	if left == 1 {
		s = ""
	}
	v.Sayf("%d mention%s of %s left, inside %s, and no surviving row names it: the text "+
		"does not compile until the collection has run, and phasecheck is where "+
		"that is asserted", left, s, strings.Join(found, " and "), strings.Join(holders, ", "))
	return v.Done()
}
