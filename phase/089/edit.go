package p089

// Whim phase 89 -- the editor loses every way to write a file.  See GOAL.md.
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
// and `phasecheck` in phase/089/check.go is where "it compiles" is
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
// NO ENUMERATOR DUMP HERE, and phase 88 had one for a reason that does not apply.
// Deleting the six renumbers 89 survivors, all of them `CMD_*` -- measured with
// tools/enumvals.sh: 1,323 enumerator values in, 1,303 out, 20 gone (the six plus
// fourteen single-constant explicit-value enums the sweep takes with their types),
// 89 moved and every one a command index.  Phase 88's `main_errors[]` was a table
// indexed by the enumerators it removed, with the rows written in order, so a wrong
// index was invisible to the build and DWARF was the only witness.  `cmdnames[]` is
// DESIGNATED: a row lands at its own enumerator whatever the numbering is, the
// `static_assert` on the row count catches a dropped pair, and every one of the 105
// names is dispatched by `zexcmds` in the declared delta.  Three checks the
// build cannot dodge, and none of them needs the values.
//
// NOT create_cmdidxs --check, for phase/085/edit.go's reason: the derived
// first-two-letters index went with the table whim reduced, and the tool raises
// rather than reporting nothing.  Its `names()` is called, which is the part that
// still means something here.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, exactly as phase/085/edit.go, phase/087/edit.go and phase/088/edit.go do it.  It
// is not decoration: THE CORPUS CANNOT SEE WRITING.  ``zcases``'s `cmd_write`
// types `:write` with no file name and has only ever recorded `E32: No file name`,
// so every screen the baselines hold is of an editor that failed to write.  The only
// evidence that this phase removed writing rather than one error message is a probe
// that requires the OLD binary to leave a file on the disk, and that needs the old
// binary.  The source goes with it, as $state/old.c, for the before-and-after counts
// the check takes.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { edit.Register("whim89", Edit) }

// z6Six are the six commands that put bytes on a disk, and nothing else.
var z6Six = []string{"CMD_exit", "CMD_saveas", "CMD_update", "CMD_write", "CMD_wq", "CMD_xit"}

const (
	z6RowsBefore = 111
	z6RowsAfter  = 105
	z6Floor      = 100
)

// Whim89 takes every way to write a file: the six Ex commands, ZZ and the
// `:w >>` / `:w !` parse.
//
// FOUR ANCHORS AND NOT ONE FOLD.  The row is the only reference a command
// handler has, so taking the row is what makes the handler unreachable and the
// sweep is what removes it.  The text this edit leaves DOES NOT COMPILE --
// step 5 is the honest form of that claim, computed rather than listed.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "nowrite", W: w}

	// literal is the heredoc's, and it does NOT report: step 1 and step 2 each
	// call it six times and then say one line.
	literal := func(t []byte, old, new, what string, n int) ([]byte, error) {
		k := strings.Count(string(t), old)
		if k != n {
			return nil, p.Die("%s -- %s occurs %d times, expected %d", what, cutil.PyRepr(edit.ZHead(old, 50)), k, n)
		}
		return []byte(strings.ReplaceAll(string(t), old, new)), nil
	}
	// within replaces exact text inside ONE function, counted there and not
	// file-wide -- `q!` is already in nv_Zet's neighbour as ZQ's.
	within := func(t []byte, fn, old, new, what string, n int) ([]byte, error) {
		a, z, ok := cutil.FindDefinition(t, cutil.Blank(t), fn)
		if !ok {
			return nil, p.Die("%s is not defined", fn)
		}
		Body := string(t[a:z])
		k := strings.Count(Body, old)
		if k != n {
			return nil, p.Die("%s -- %s occurs %d times in %s, expected %d",
				what, cutil.PyRepr(edit.ZHead(old, 50)), k, fn, n)
		}
		return []byte(string(t[:a]) + strings.ReplaceAll(Body, old, new) + string(t[z:])), nil
	}

	// ---- 0. the table this phase edits, at the shape the anchors were counted on
	if n := len(edit.ZRows(text)); n != z6RowsBefore {
		return nil, p.Die("cmdnames[] has %d rows, expected %d -- the anchors below were counted "+
			"against a different table", n, z6RowsBefore)
	}
	names, err := harness.CommandNamesIn(text, "whim-vim.c")
	if err != nil || len(names) != z6RowsBefore {
		return nil, p.Die("create_cmdidxs.names() does not read %d rows out of this table", z6RowsBefore)
	}

	// ---- 1. the six enumerators of enum CMD_index -----------------------------
	for _, e := range z6Six {
		if text, err = literal(text, fmt.Sprintf("    %s,\n", e), "", "the "+e+" enumerator", 1); err != nil {
			return nil, err
		}
	}
	short := make([]string, len(z6Six))
	for i, e := range z6Six {
		short[i] = e[4:]
	}
	p.Sayf("six enumerators of enum CMD_index: %s", strings.Join(short, " "))

	// ---- 2. the six cmdnames[] rows -------------------------------------------
	for _, e := range z6Six {
		re := regexp.MustCompile(`(?m)^    \[` + e + `\] = \{.*\n`)
		m := re.FindIndex(text)
		if m == nil {
			return nil, p.Die("cmdnames[] has no [%s] row", e)
		}
		text = append(append([]byte{}, text[:m[0]]...), text[m[1]:]...)
	}
	if n := len(edit.ZRows(text)); n != z6RowsAfter {
		return nil, p.Die("cmdnames[] has %d rows after the cut, expected %d", n, z6RowsAfter)
	}
	p.Sayf("six cmdnames[] rows; %d -> %d, and create_cmdidxs.names() refuses under %d, "+
		"so the margin is %d rows -- the :edit phase spends it (GOALS.md II.3a)",
		z6RowsBefore, z6RowsAfter, z6Floor, z6RowsAfter-z6Floor)

	// ---- 3. ZZ ----------------------------------------------------------------
	if text, err = within(text, "nv_Zet", `do_cmdline_cmd((char_u *)"x");`,
		`do_cmdline_cmd((char_u *)"q!");`, `ZZ is ZQ: nv_Zet runs "q!" where it ran "x"`, 1); err != nil {
		return nil, err
	}
	p.Say(`ZZ is ZQ: nv_Zet runs the string "q!" where it ran "x", which is this ` +
		`phase's decision and moves case:zz_key`)

	// ---- 4. do_one_cmd's :w>> and :w! parse -----------------------------------
	if text, err = within(text, "do_one_cmd", z6lit1, "",
		"do_one_cmd no longer parses `:w >>file` or `:w !cmd`", 1); err != nil {
		return nil, err
	}
	p.Say("do_one_cmd's `:w>>` and `:w!` parse, deleted as text: its condition named " +
		"two of the enumerators above")

	// ---- 5. what is left, and why it does not compile yet ---------------------
	left, holders, _, err := edit.ZResidue(p, text, z6Six)
	if err != nil {
		return nil, err
	}
	p.Sayf("%d mentions of the six are left, all inside %s, and no surviving row names "+
		"any of them: the text does not compile until the sweep has run, and "+
		"phasecheck is where that is asserted", left, strings.Join(holders, ", "))
	return text, nil
}
