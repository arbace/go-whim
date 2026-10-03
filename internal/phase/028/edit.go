package p028

// Whim phase 28 (formerly 89) -- the editor loses every way to write a file.  See GOAL.md.
//
// A core does not own a disk.  Reading and writing files is the host's business
// (GOALS.md, the charter), and this is the first half of taking the filesystem
// away: the six Ex commands that put bytes on a disk -- `:write :wq :xit :exit
// :update :saveas` -- and, with them, everything only they reached.
//
// FOUR ANCHORS, AND NOT ONE FOLD.  Everything else is the sweep's (GOALS.md
// rule 1: removal is computed, not listed).  The alternative was measured: an edit
// that also deletes `ex_write`, `ex_update`, `ex_exit`, `do_write`, `check_writable`,
// `check_overwrite`, `not_writing` and `check_readonly` by name produces a
// BYTE-IDENTICAL swept file, in 3 rounds against 4 and 17 seconds against 22.  So
// the eight names are not written here: the table row is the only reference a
// command handler has, and taking the row is what makes the handler unreachable.
//
// 1. the six enumerators of `enum CMD_index`, one line each;
// 2. the six `cmdnames[]` rows, one physical line each, designated `[CMD_x] = {`;
// 3. `nv_Zet`'s `ZZ`, which runs the string "x": `do_cmdline_cmd("x")` -> "q!";
// 4. `do_one_cmd`'s `:w>>` / `:w!` parse, an `if (ea.cmdidx == CMD_write ||
// ea.cmdidx == CMD_update) {...}` with no else -- deleted as TEXT rather than
// folded, because its condition names two enumerators that are going.  It must
// go in the same edit as anchor 1 or nothing declares what it reads.
//
// `ZZ` BECOMES `q!`, WHICH IS A DECISION AND NOT A CONSEQUENCE.  `nv_Zet` runs a
// command STRING, so nothing here breaks at compile time: left alone, `ZZ` would
// type `:x` at a command that no longer exists and answer E492.  The user's settled
// decision is that ZZ is ZQ, and `case:zz_key` moves either way -- E32 today, E492
// if the string is left, nothing at all with `q!` -- so this phase owns it and says
// so rather than leaving a dead command named in the source.  the plan (GOALS.md II.3b) gives it
// to the `:q` phase; that row is annotated as built.
//
// THE TEXT THIS EDIT LEAVES DOES NOT COMPILE, and that is stated here because
// nothing else would say it.  Six mentions of the six enumerators survive the cut --
// `CMD_saveas` five times and `CMD_wq` once -- every one of them inside `ex_write`,
// `do_write` or `ex_exit`, which are exactly the functions whose only reference was
// the row that just went.  `funcreach.py` deletes them in the sweep's first round,
// and `phasecheck` in internal/phase/028/check.go is where "it compiles" is
// asserted.  The invariant below is the honest form of that: every surviving mention
// is inside a function definition, and no surviving `cmdnames[]` row names that
// function.
//
// THE ROW FLOOR.  `cmdnames[]` goes 111 -> 105 rows, and
// `create_cmdidxs`'s `names()` REFUSES a table of fewer than 100 -- a regex
// that stops matching otherwise yields a plausible all-zero index, so the floor is
// deliberate.  ``zexcmds`` enumerates the table through it, so crossing the
// floor would stop the core's command sweep rather than give a wrong answer.  After this
// phase the margin is FIVE ROWS.  GOALS.md II.3a: the `:edit` phase is the one that
// spends it, and it is the phase that must lower the floor.
//
// NO ENUMERATOR DUMP HERE, and record 88 had one for a reason that does not apply.
// Deleting the six renumbers 89 survivors, all of them `CMD_*` -- measured with
// tools/enumvals.sh: 1,323 enumerator values in, 1,303 out, 20 gone (the six plus
// fourteen single-constant explicit-value enums the sweep takes with their types),
// 89 moved and every one a command index.  Record 88's `main_errors[]` was a table
// indexed by the enumerators it removed, with the rows written in order, so a wrong
// index was invisible to the build and DWARF was the only witness.  `cmdnames[]` is
// DESIGNATED: a row lands at its own enumerator whatever the numbering is, the
// `static_assert` on the row count catches a dropped pair, and every one of the 105
// names is dispatched by `zexcmds` in the declared delta.  Three checks the
// build cannot dodge, and none of them needs the values.
//
// NOT create_cmdidxs --check, for internal/phase/004/e/edit.go's reason: the derived
// first-two-letters index went with the table whim reduced, and the tool raises
// rather than reporting nothing.  Its `names()` is called, which is the part that
// still means something here.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, exactly as internal/phase/004/e/edit.go, internal/phase/027/edit.go and internal/phase/archive/088/edit.go do it.  It
// is not decoration: THE CORPUS CANNOT SEE WRITING.  ``zcases``'s `cmd_write`
// types `:write` with no file name and has only ever recorded `E32: No file name`,
// so every screen the baselines hold is of an editor that failed to write.  The only
// evidence that this phase removed writing rather than one error message is a probe
// that requires the OLD binary to leave a file on the disk, and that needs the old
// binary.  The source goes with it, as $state/old.c, for the before-and-after counts
// the check takes.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  The same four acts on the
// program's graph, the report the text version's (history keeps it, with
// editlit.go's literal):
//
//   - the enumerators still named are deleted by RENUM, the values after
//     them moving as the text's deleted lines moved them (Renumber; on
//     today's input the two left are explicit and last, and nothing moves);
//   - ZZ's string is the one literal "x" in nv_Zet's call, respelled;
//   - do_one_cmd's `:w>>`/`:w!` parse is the one if whose condition asks
//     for CMD_write or CMD_update, cut;
//   - the residue is the text's own computation on the C view (TEXTQ).

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.RegisterGraph("whim28", Edit) }

// w28Six are the six commands that put bytes on a disk, and nothing else.
var w28Six = []string{"CMD_exit", "CMD_saveas", "CMD_update", "CMD_write", "CMD_wq", "CMD_xit"}

// Whim28 takes every way to write a file: the six Ex commands, ZZ and the
// `:w >>` / `:w !` parse.
//
// FOUR ANCHORS AND NOT ONE FOLD.  The row is the only reference a command
// handler has, so taking the row is what makes the handler unreachable and the
// sweep is what removes it.  The text this edit leaves DOES NOT COMPILE --
// step 5 is the honest form of that claim, computed rather than listed.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nowrite", e, w)

	// ---- 0. the six commands' rows went at phase 1 (filefront, the reform's
	// D4): their enumerators stay, after CMD_SIZE, until this phase takes the
	// last uses, and the table is the product's 98 rows.

	// ---- 1. the six enumerators of enum CMD_index -----------------------------
	t := v.Text()
	var named []string
	for _, n := range w28Six {
		// one nothing names any more went with its row (phase 26)
		if edit.MentionCount(t, n) > 0 {
			named = append(named, n)
		}
	}
	short := make([]string, len(w28Six))
	for i, n := range w28Six {
		short[i] = n[4:]
	}
	v.DeleteEnumerators(named, graph.Renumber, "six enumerators of enum CMD_index: "+strings.Join(short, " "))

	// ---- 3. ZZ: the one "x" nv_Zet runs is "q!" -------------------------------
	v.InFunction("nv_Zet", func(v *graph.Verbs) {
		x := v.One(`(call do_cmdline_cmd (cast (ptr char_u) "x"))`, `ZZ is ZQ: nv_Zet runs "q!" where it ran "x"`)
		if x != nil {
			if err := e.RespellString(x.Kids[2].Kids[2], `"q!"`); err != nil {
				v.Die(`ZZ is ZQ -- %v`, err)
			}
		}
	})
	v.Say(`ZZ is ZQ: nv_Zet runs the string "q!" where it ran "x", which is this ` +
		`phase's decision and moves case:zz_key`)

	// ---- 4. do_one_cmd's :w>> and :w! parse -----------------------------------
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.CutWhere("(if (|| (== (. ea cmdidx) CMD_write) (== (. ea cmdidx) CMD_update)) _*)", nil, 1, "")
	})
	v.Say("do_one_cmd's `:w>>` and `:w!` parse, deleted as text: its condition named " +
		"two of the enumerators above")

	// ---- 5. what is left, and why it does not compile yet ---------------------
	if v.Failed() {
		return v.Done()
	}
	left, holders, _, err := vimtext.CoreResidue(edit.Ph{Tag: "nowrite", W: w}, v.Text(), w28Six)
	if err != nil {
		return err
	}
	v.Sayf("%d mentions of the six are left, all inside %s, and no surviving row names "+
		"any of them: the text does not compile until the collection has run, and "+
		"phasecheck is where that is asserted", left, strings.Join(holders, ", "))
	return v.Done()
}
