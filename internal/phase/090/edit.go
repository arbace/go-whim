package p090

// Whim phase 90 -- the editor loses every way to read a file.  See GOAL.md.
//
// The other half of taking the filesystem away.  Phase 89 removed the six commands
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
// THE JUDGEMENT: `exarg_T.usefilter`.  Phase 89 removed one of its two writers
// (`:w >>` and `:w !cmd`) and anchor 3 removes the other, so after this edit the
// field is WRITTEN NOWHERE -- and `do_one_cmd` memsets the struct, so every reader
// is constantly FALSE.  No tool here can see that: tools/deadfields.py removes a
// field nothing NAMES, and gcc has no warning for a struct member that is only
// read.  So the six readers are folded by hand, and the field, read then only in
// ex_read, goes in the sweep after it -- phase 87's argument for `exmode_active`
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
// not existed since whim, and phase 89 swept `ex_write`, which held `do_bang`'s other
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
// NO ENUMERATOR DUMP, for phase 89's reason.  Deleting one renumbers 46 survivors and
// every one is a `CMD_*`: `cmdnames[]` is DESIGNATED, so a row lands at its own
// enumerator whatever the numbering is, the `static_assert` on the row count catches
// a dropped pair, and all 104 surviving names are dispatched by `zexcmds`
// inside the declared delta.  There is no derived first-two-letters index in this
// file -- whim's Phase 80 took it with the 489 stub rows -- so nothing else depends
// on a position.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, exactly as internal/phase/085/edit.go, internal/phase/087/edit.go, internal/phase/088/edit.go and
// `zcases`'s 102 cases types its own text and names no file, so `cmd_read`
// types `:read` with no file name and has only ever recorded `E32: No file name`.
// The only evidence that this phase removed reading rather than one error message is
// a probe that requires the OLD binary to pull a file off the disk, and that needs
// the old binary.  The source goes with it, as $state/old.c, for the before-and-
// after counts the check takes.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/cmdtab"
	"github.com/arbace/go-whim/internal/phase"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

func init() { phase.Register("whim90", Edit) }

const (
	w90RowsBefore = 105
	w90RowsAfter  = 104
	w90Floor      = 100
)

// w90Dying are the names whose survivors are the reason the text does not
// compile yet.  `usefilter` is not among them: its member declaration is still
// there when the edit returns, and the sweep takes it with ex_read's last read.
var w90Dying = []string{"CMD_read"}

var w90Assign = regexp.MustCompile(`\busefilter\s*=`)

// Whim90 takes the way to read a file: `:read`, its `:r !cmd` arm, and the
// exarg_T.usefilter field that nothing writes once both `:w !` and `:r !` are
// gone.
//
// STEP 4 IS THE JUDGEMENT OF THIS PHASE and the one thing here no tool could
// have found: a struct member that is READ and never written draws no warning,
// and deadfields.py removes only a member nothing names.  do_one_cmd memsets
// `ea`, so every test folded there is constantly FALSE.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "noread", W: w}
	var err error

	mentions := edit.MentionCount
	inFunction := func(t []byte, name string, fn func([]byte) ([]byte, error)) ([]byte, error) {
		a, z, ok := edit.FindDefinition(t, edit.Blank(t), name)
		if !ok {
			return nil, p.Die("%s is not defined", name)
		}
		Body, err := fn(t[a:z])
		if err != nil {
			return nil, err
		}
		return []byte(string(t[:a]) + string(Body) + string(t[z:])), nil
	}
	// within replaces exact text inside one function, counted THERE and not
	// file-wide, and reports -- whim89's namesake does not report, which is the
	// phase's own spelling and not a shared helper's.
	within := func(t []byte, fn, old, new, what string, n int) ([]byte, error) {
		Out, err := inFunction(t, fn, func(s []byte) ([]byte, error) {
			k := strings.Count(string(s), old)
			if k != n {
				return nil, p.Die("%s -- %s occurs %d times in %s, expected %d",
					what, edit.PyRepr(edit.CoreHead(old, 60)), k, fn, n)
			}
			return []byte(strings.ReplaceAll(string(s), old, new)), nil
		})
		if err != nil {
			return nil, err
		}
		p.Say(what)
		return Out, nil
	}

	// ---- 0. the table and the field, at the shape the anchors were counted on
	if n := len(vimtext.CoreRows(text)); n != w90RowsBefore {
		return nil, p.Die("cmdnames[] has %d rows, expected %d -- the anchors below were counted "+
			"against a different table", n, w90RowsBefore)
	}
	names, err := cmdtab.CommandNamesIn(text, "whim-vim.c")
	if err != nil || len(names) != w90RowsBefore {
		return nil, p.Die("create_cmdidxs names() does not read %d rows out of this table", w90RowsBefore)
	}
	for _, b := range []struct {
		Name string
		want int
	}{{"CMD_read", 3}, {"ex_read", 2}, {"open_buffer", 6}, {"read_buffer", 17},
		{"readfile", 7}, {"usefilter", 10}} {
		if k := mentions(text, b.Name); k != b.want {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", b.Name, k, b.want)
		}
	}
	if writes := len(w90Assign.FindAll(text, -1)); writes != 2 {
		return nil, p.Die("usefilter is assigned %d times, expected the 2 that anchor 3 removes -- "+
			"phase 89 took the other two with `:w >>` and `:w !cmd`", writes)
	}
	p.Say("cmdnames[] 105 rows, CMD_read 3 mentions, usefilter 10 -- the field, the two " +
		"writes anchor 3 removes and seven reads")

	// ---- 1. the CMD_read enumerator -------------------------------------------
	if strings.Count(string(text), "    CMD_read,\n") != 1 {
		return nil, p.Die("the CMD_read enumerator is not one line of its own")
	}
	text = []byte(strings.ReplaceAll(string(text), "    CMD_read,\n", ""))
	p.Say("the CMD_read enumerator of enum CMD_index")

	// ---- 2. the cmdnames[] row -------------------------------------------------
	m := regexp.MustCompile(`(?m)^    \[CMD_read\] = \{.*\n`).FindIndex(text)
	if m == nil {
		return nil, p.Die("cmdnames[] has no [CMD_read] row")
	}
	text = append(append([]byte{}, text[:m[0]]...), text[m[1]:]...)
	if n := len(vimtext.CoreRows(text)); n != w90RowsAfter {
		return nil, p.Die("cmdnames[] has %d rows after the cut, expected %d", n, w90RowsAfter)
	}
	p.Sayf("the cmdnames[] row; %d -> %d, and create_cmdidxs names() refuses under %d, so "+
		"the margin is %d rows -- the :edit phase spends it (GOALS.md II.3a)",
		w90RowsBefore, w90RowsAfter, w90Floor, w90RowsAfter-w90Floor)

	// ---- 3. do_one_cmd's `:r!` and `:r !cmd` parse -----------------------------
	if text, err = within(text, "do_one_cmd", w90lit2, "",
		"do_one_cmd no longer parses `:r!` or `:r !cmd`: usefilter loses its last "+
			"two writes", 1); err != nil {
		return nil, err
	}

	// ---- 4. the field nothing writes any more, and its seven readers -----------
	if w90Assign.Match(text) {
		return nil, p.Die("usefilter is still assigned after anchor 3, so the fold below would be wrong")
	}
	for _, a := range []struct{ Old, New, What string }{
		{"(ea.argt & EX_CMDARG) && !ea.usefilter", "ea.argt & EX_CMDARG",
			"EX_CMDARG takes its argument command whatever the (dead) filter flag said"},
		{"(ea.argt & EX_TRLBAR) && !ea.usefilter", "ea.argt & EX_TRLBAR",
			"and EX_TRLBAR separates a trailing command"},
		{" || ea.usefilter)", ")",
			"and only :global and :vglobal keep a backslash-newline in their argument"},
	} {
		if text, err = within(text, "do_one_cmd", a.Old, a.New, a.What, 1); err != nil {
			return nil, err
		}
	}
	if text, err = within(text, "expand_filename", "if (!eap->usefilter && !escaped)", "if (!escaped)",
		"expand_filename escapes a replacement unless it was escaped already", 1); err != nil {
		return nil, err
	}
	if text, err = inFunction(text, "expand_filename", func(s []byte) ([]byte, error) {
		return edit.FoldNever(s, `(?m)^[ \t]*if \(eap->usefilter &&.*\)$`, 1)
	}); err != nil {
		return nil, err
	}
	p.Say("and no longer escapes `!` for a shell, which only a filter needed")
	if text, err = within(text, "expand_filename", "(eap->argt & EX_NOSPC) && !eap->usefilter",
		"eap->argt & EX_NOSPC", "and EX_NOSPC refuses a second file name whatever it said", 1); err != nil {
		return nil, err
	}

	// ---- 5. what is left, and why it does not compile yet ----------------------
	left, holders, _, err := vimtext.CoreResidue(p, text, w90Dying)
	if err != nil {
		return nil, err
	}
	s := "s"
	if left == 1 {
		s = ""
	}
	p.Sayf("%d mention%s of %s left, inside %s, and no surviving row names it: the text "+
		"does not compile until the sweep has run, and phasecheck is where "+
		"that is asserted", left, s, strings.Join(w90Dying, " and "), strings.Join(holders, ", "))
	return text, nil
}
