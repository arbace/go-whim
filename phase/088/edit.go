package p088

// Whim phase 88 -- the command line is `+{command}` and `-T {term}`.  See GOAL.md.
//
// A core is handed its buffer by a host, not by a shell.  What is left of
// `command_line_scan()` after phases 85 and 87 is five things -- `+cmd`, `-T`, a bare
// `-`, `--` and a file argument -- and the last three are the three that name a
// FILE or a STREAM to edit.  They go, and argv ends as exactly two options: the
// commands to run and the terminal to assume.  Everything else is what every other
// unknown word already was, `mainerr(ME_UNKNOWN_OPTION)`.
//
// WHAT GOES, in the parser:
// * the file-argument branch -- the `else` arm: the ME_TOO_MANY_ARGS guard,
// `parmp->edit_type = EDIT_FILE`, the `vim_strsave()` and the `buflist_add()`
// that put the name in the buffer list.  The arm is REPLACED by
// `mainerr(ME_UNKNOWN_OPTION)` rather than deleted: with no arm at all a bare
// word matches neither `+` nor `-`, `argv[0][argv_idx]` is not NUL, and the
// `while` never advances -- an infinite loop, not an error.
// * `case NUL`, the bare `-`: EDIT_STDIN, `read_cmd_fd = 2` and its own
// ME_TOO_MANY_ARGS guard.  It falls to `default:`, so `-` is an unknown option.
// * `case '-'`, which is where `--` ended the options, with `had_minmin` and the
// two `&& !had_minmin` tests that read it.  `--foo` already went to
// ME_UNKNOWN_OPTION from inside that case and goes there from `default:` now;
// what changes is `--` itself, which used to mean "every word after this is a
// file name" and now means nothing.
// * `ME_TOO_MANY_ARGS`, whose two call sites were exactly those two branches, and
// its row in `main_errors[]`.
//
// THE ENUMERATOR IS THE ROW INDEX, so the two go together and the survivors
// renumber: `main_errors[n]` is what `mainerr(n)` prints, ME_ARG_MISSING moves 2->1,
// ME_GARBAGE 3->2 and ME_EXTRA_CMD 4->3.  That is the renumbering CLAUDE.md warns
// about, done deliberately -- the table and the enum are edited from one parse of
// both, and phase/088/check.go compares the DWARF enumerator values of the binary
// this phase was handed with the ones it made and requires exactly those three to
// have moved.  The dump of the input is taken HERE, in the background, because after
// the edit there is nothing left to dump it from.
//
// `main_errors[]` has SIX rows and had five enumerators; the sixth, "Invalid
// argument for", is unreachable already and was before this phase -- nothing names
// index 5.  It is whim's leftover, not this phase's, and it stays: this phase
// removes the row an enumerator it removes points at, and nothing else.
//
// WHAT `params.edit_type` THEN IS.  Nothing assigns it, so it is EDIT_NONE for
// ever, and its two readers in vim_main2() fold: `== EDIT_STDIN` never, which takes
// `read_stdin()`'s only call with it, and `!= EDIT_STDIN` always, which keeps
// `newline_on_exit` under the two conditions that were already there.  The field,
// the three EDIT_* enumerators, `read_stdin()` and `buflist_add()` are then what the
// sweep takes -- tools/deadfields.py for the field (there is no ml_recover() in this
// file, so a struct is no longer a disk format), deadenums.py for the three
// single-constant enums, each with an explicit value so nothing renumbers, and
// deadsweep.py for the two functions and whatever they orphan.
//
// WHERE THE LINE IS AGAINST THE LATER PHASES, and why it is there:
// * `readfile()`'s stdin half -- its `read_stdin` PARAMETER and the arms that read
// it, in readfile(), read_buffer() and open_buffer() -- is the "nothing reads a
// byte" phase's (GOALS.md II.3b P8).  This phase removes the FUNCTION
// `read_stdin()`, which is argv's entry point into that code; the parameter's 23
// mentions are asserted UNCHANGED, so a phase that took them here would fail.
// * `read_cmd_fd` keeps its definition and its twelve remaining mentions.  Nothing assigns it
// now, so it is 0 for ever and folding it is the stdin phase's; a file-scope
// static that is read and never written draws no warning, so the sweep will not
// touch it either way.  Only the assignment was argv's.
// * the buffer's NAME is the "buffer has no name" phase's (P9).  Nothing here
// touches `b_ffname`, `b_sfname` or `b_fname`: what goes is the one call that
// ever gave the startup buffer a name from argv.  `create_windows()` already
// opens an unnamed buffer when argv named none -- that is the `(none)` row of
// `zargv` -- so the startup path is the one that was always there.
//
// WHAT IS KEPT, and asserted by name in the check: `+{command}` with MAX_ARG_CMDS
// and ME_EXTRA_CMD; `-T {term}` with want_argument, ME_GARBAGE and
// mainerr_arg_missing; ME_UNKNOWN_OPTION as the answer to everything else;
// `exe_commands()` and the `+cmd` execution path; and `'paste'`, which every case of
// the corpus seeds itself with (GOALS.md II.2d).
//
// THERE IS NO usage() TO LEAVE ALONE.  The brief warns that the help text may still
// advertise options that no longer exist; in this file it does not exist either --
// `grep -i usage whim-vim.c` finds nothing, whim having removed it, and `--help` is
// already `Unknown option argument: "--help"` in .reference/core-baselines/
// ref-argv.txt.  Nothing here prints a list of options to keep true.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own makefile
// flags, exactly as phase/085/edit.go and phase/087/edit.go do it: the check
// requires the OLD binary to open the file and the new one to refuse, which is the
// difference between a probe and a formality.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).
// The enumerator values of the text this phase is HANDED.  It has to be taken
// before the edit, and it is the left-hand side of the check's renumbering proof.
// NOT create_cmdidxs --check, for phase/085/edit.go's reason: the derived
// first-two-letters index went with the command table whim reduced, and the tool
// raises rather than reporting nothing.  Nothing here touches the command table.
//
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim88", Edit) }

var (
	z5EnumRun  = regexp.MustCompile(`(?m)(?:^enum \{ ME_\w+ = \d+ \};\n)+`)
	z5EnumLine = regexp.MustCompile(`(?m)^enum \{ (ME_\w+) = (\d+) \};$`)
	z5Table    = regexp.MustCompile(`(?ms)^static char \*\(main_errors\[\]\) =\n\{\n(.*?)^\};\n`)
)

// z5Before is every identifier this phase removes at the mentions it has before
// it, plus the ones it must NOT move -- `read_stdin` as a parameter (23 of its
// 26 mentions belong to the phase that stops reading bytes) and the five kept
// ME_* / MAX_ARG_CMDS.
var z5Before = map[string]int{
	"had_minmin": 4, "edit_type": 7, "EDIT_NONE": 3, "EDIT_FILE": 2,
	"EDIT_STDIN": 4, "ME_TOO_MANY_ARGS": 3, "buflist_add": 3,
	"read_stdin": 26, "read_cmd_fd": 13, "ME_UNKNOWN_OPTION": 3,
	"ME_ARG_MISSING": 2, "ME_GARBAGE": 2, "ME_EXTRA_CMD": 2,
	"MAX_ARG_CMDS": 4, "want_argument": 4, "mainerr_arg_missing": 3,
	"exe_commands": 3,
}

// z5After is the same names once the cut has run: each removed name is down to
// its definition, and each definition is a kind tools/sweep.sh deletes.  Stated
// as a number per name, so a use that survived shows up HERE and not as a
// warning five minutes later.
var z5After = map[string]int{
	"had_minmin": 0, "edit_type": 1, "EDIT_NONE": 1, "EDIT_FILE": 1,
	"EDIT_STDIN": 1, "ME_TOO_MANY_ARGS": 0, "buflist_add": 2,
	"read_stdin": 25, "read_cmd_fd": 12, "ME_UNKNOWN_OPTION": 3,
	"ME_ARG_MISSING": 2, "ME_GARBAGE": 2, "ME_EXTRA_CMD": 2,
	"MAX_ARG_CMDS": 4, "want_argument": 4, "mainerr_arg_missing": 3,
	"exe_commands": 3,
}

// Whim88 leaves the command line as `+{command}` and `-T {term}`: the file
// argument, the bare `-` and `--` all become mainerr(ME_UNKNOWN_OPTION).
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "noargv", W: w}
	var err error

	mentions := func(t []byte, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAll(t, -1))
	}
	inFunction := func(t []byte, name string, edit func([]byte) ([]byte, error)) ([]byte, error) {
		a, z, ok := cutil.FindDefinition(t, cutil.Blank(t), name)
		if !ok {
			return nil, p.Die("%s is not defined", name)
		}
		Body, err := edit(t[a:z])
		if err != nil {
			return nil, err
		}
		return []byte(string(t[:a]) + string(Body) + string(t[z:])), nil
	}
	// within reports only when the caller gives it a `what`: three of the calls
	// below are the second half of an act the line before has already named.
	within := func(t []byte, fn, old, new, what string, n int) ([]byte, error) {
		Out, err := inFunction(t, fn, func(s []byte) ([]byte, error) {
			k := strings.Count(string(s), old)
			if k != n {
				w := what
				if w == "" {
					w = "in " + fn
				}
				return nil, p.Die("%s -- %s occurs %d times in %s, expected %d",
					w, cutil.PyRepr(old), k, fn, n)
			}
			return []byte(strings.ReplaceAll(string(s), old, new)), nil
		})
		if err != nil {
			return nil, err
		}
		if what != "" {
			p.Say(what)
		}
		return Out, nil
	}
	// line is a whole line by its trimmed text: the anchor is exact and countable.
	line := func(Body string) string { return `(?m)^[ \t]*` + regexp.QuoteMeta(Body) + `$` }

	// ---- 0. the invariants the cut rests on -----------------------------------
	for _, name := range edit.SortedKeys(z5Before) {
		if k := mentions(text, name); k != z5Before[name] {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, z5Before[name])
		}
	}
	p.Say("17 identifiers at their counted mentions: had_minmin 4, edit_type 7, " +
		"read_stdin 26 (23 of them a parameter)")

	// ---- 1. the parser: three ways to name a file or a stream -----------------
	if text, err = within(text, "command_line_scan", z5lit1, z5lit2,
		"a file argument is an unknown option: buflist_add loses its only caller", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "command_line_scan", z5lit3, "\n",
		"and `p`, which only that arm used", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "command_line_scan", z5lit5, "",
		"a bare `-` is an unknown option: EDIT_STDIN and read_cmd_fd = 2 go", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "command_line_scan", z5lit6, "",
		"`--` no longer ends the options", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "command_line_scan", z5lit7, "\n",
		"and had_minmin, the flag it set", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "command_line_scan", `if (argv[0][0] == '+' && !had_minmin)`,
		`if (argv[0][0] == '+')`, "so +cmd is +cmd wherever it appears", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "command_line_scan", `else if (argv[0][0] == '-' && !had_minmin)`,
		`else if (argv[0][0] == '-')`, "and an option is an option", 1); err != nil {
		return nil, err
	}

	// ---- 2. ME_TOO_MANY_ARGS, and the row it indexes --------------------------
	// ONE PARSE OF BOTH LISTS, and the edit is computed from it: the enumerators
	// are main_errors[]'s indices, so the two cannot be edited separately without
	// the numbering being a guess.
	const gone = "ME_TOO_MANY_ARGS"
	enums := z5EnumRun.FindString(string(text))
	if enums == "" {
		return nil, p.Die("the ME_* enumerators are not a run of `enum { NAME = N };` lines")
	}
	pairs := z5EnumLine.FindAllStringSubmatch(enums, -1)
	var repr []string
	for i, pr := range pairs {
		if v, _ := strconv.Atoi(pr[2]); v != i {
			for _, q := range pairs {
				repr = append(repr, "("+cutil.PyRepr(q[1])+", "+cutil.PyRepr(q[2])+")")
			}
			return nil, p.Die("the ME_* enumerators are not 0..%d in order: [%s]",
				len(pairs)-1, strings.Join(repr, ", "))
		}
	}
	tm := z5Table.FindStringSubmatchIndex(string(text))
	if tm == nil {
		return nil, p.Die("main_errors[] is not where it was")
	}
	whole := string(text[tm[0]:tm[1]])
	rows := edit.Z5Lines(string(text[tm[2]:tm[3]]))
	if len(rows) != len(pairs)+1 {
		return nil, p.Die("main_errors[] has %d rows for %d enumerators; this phase only knows the "+
			"shape where the one extra row is the unreachable one whim left", len(rows), len(pairs))
	}
	names := make([]string, len(pairs))
	for i, pr := range pairs {
		names[i] = pr[1]
	}
	i := edit.IndexOf(names, gone)
	if i < 0 {
		return nil, p.Die("%s is not among the ME_* enumerators", gone)
	}
	if !strings.Contains(rows[i], "Too many edit arguments") {
		return nil, p.Die("main_errors[%d] is %s, which is not %s's row",
			i, cutil.PyRepr(strings.TrimSpace(rows[i])), gone)
	}
	var kept []string
	for _, n := range names {
		if n != gone {
			kept = append(kept, n)
		}
	}
	var newEnums, newRows strings.Builder
	for k, n := range kept {
		fmt.Fprintf(&newEnums, "enum { %s = %d };\n", n, k)
	}
	for k, r := range rows {
		if k != i {
			newRows.WriteString(r)
		}
	}
	text = []byte(strings.ReplaceAll(
		strings.ReplaceAll(string(text), enums, newEnums.String()),
		whole, "static char *(main_errors[]) =\n{\n"+newRows.String()+"};\n"))
	var moved []string
	for k, n := range kept {
		if o := edit.IndexOf(names, n); o != k {
			moved = append(moved, fmt.Sprintf("%s %d->%d", n, o, k))
		}
	}
	p.Sayf("%s and its row go; %s", gone, strings.Join(moved, ", "))
	p.Sayf("main_errors[] keeps its sixth row, %s, which no enumerator named before this "+
		"phase either", strings.TrimSpace(strings.TrimRight(strings.TrimSpace(rows[len(rows)-1]), ",")))

	// ---- 3. what params.edit_type is once nothing assigns it ------------------
	if text, err = inFunction(text, "vim_main2", func(s []byte) ([]byte, error) {
		return cutil.FoldNever(s, line("if (params.edit_type == EDIT_STDIN)"), 1)
	}); err != nil {
		return nil, err
	}
	p.Say("vim_main2 no longer reads a buffer from stdin: read_stdin loses its call")
	if text, err = within(text, "vim_main2", " && params.edit_type != EDIT_STDIN)", ")",
		"and sets newline_on_exit on the two conditions that are left", 1); err != nil {
		return nil, err
	}

	// ---- 4. what is left is exactly what the sweep can take -------------------
	for _, name := range edit.SortedKeys(z5After) {
		k := mentions(text, name)
		if k != z5After[name] {
			why := "more went than was meant to"
			if k > z5After[name] {
				why = "a use survived"
			}
			return nil, p.Die("%s has %d mentions after the cut, expected %d -- %s",
				name, k, z5After[name], why)
		}
	}
	if strings.Contains(string(text), "Too many edit arguments") {
		return nil, p.Die("'Too many edit arguments' survives the edit")
	}
	p.Say("every use of the six is gone; the field, three enumerators, read_stdin and " +
		"buflist_add are what the sweep takes")
	return text, nil
}
