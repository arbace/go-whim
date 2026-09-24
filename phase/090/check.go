package p090

// Whim phase 90, the check -- nothing can take bytes off a disk on request any more.
// See phase/090/edit.go, and GOALS.md.
//
// Runs after phase/090/edit.go and the sweep tools/phaserun.sh runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from, which is the left-hand side of every
// before-and-after count below.
//
// FIVE THINGS ARE PROVED HERE, and the third is the only one that can say what this
// phase is actually for.
//
// 1. THE CUT, which is the sweep's work and not the edit's.  Six functions go, none
// of them named by the edit -- ex_read, do_bang, do_shell, do_filter,
// check_secure and prevcmd_is_set -- with the static `prevcmd`, the struct field
// `usefilter` the edit folded away, and five strings (`"read"`, E12, E34, E319
// and E484).  Listing them here is a RECORDING of what the sweep did, which is
// the only place a list of removed names belongs.
//
// THE TRAPS, ALL MEASURED, that make a copied "every name at zero mentions" loop
// the wrong check:
// * `secure` IS NOT `check_secure`.  The function goes; the variable keeps
// ELEVEN mentions, because `secure` is the vimrc/tag-search flag and half the
// editor tests it.  Only the two inside check_secure() went.
// * THE BARE WORD `read` SURVIVES three times -- two `read(fd, ...)` calls and
// an E222 string -- and `readfile`, `read_buffer`, `read_edit`, `readonly`
// and `filter` all survive at their own counts.  This phase removes
// `:read`, not reading.  (`shell` was in that list until the canonical text
// let a Part I phase finish emptying p_bo_values[]; see z7Kept.)
// * `E32: No file name` SURVIVES, still reachable through check_fname() from
// do_ecmd(); `E484: Can't open file` does NOT -- ex_read was its last
// speaker, and after this phase nothing in the file says it.  Both are
// asserted, in opposite directions.
//
// 2. THE LINE AGAINST THE PHASE THAT STOPS READING A BYTE (GOALS.md II.3b P8), stated
// as counts so that taking any of it here would fail rather than widen quietly:
// `readfile` is required to keep EXACTLY 5 mentions -- its prototype, its
// definition and the three calls in read_buffer() and open_buffer() -- with
// read_buffer at 17 and open_buffer at 6.  What this phase takes is the two calls
// that were ex_read's.  The table is checked the same way: 104 rows, names()
// reading exactly those, the static_assert in place, and the FOUR rows of margin
// above create_cmdidxs's floor of 100 said out loud -- the `:edit` phase is the
// one that must lower it (GOALS.md II.3a).
//
// 3. THE PROBES, on BOTH binaries, because THE CORPUS CANNOT SEE A FILE BEING READ.
// Every one of `zcases`'s 102 cases types its own text and names no file:
// `cmd_read` types `:read` with no file name, so the baseline it is compared
// against records `E32: No file name` -- an editor that FAILED to read.  A
// declared delta of "cmd_read and read_cmd_gone moved" is therefore consistent
// with a phase that changed two error messages and left readfile() reachable from
// a command.  So the probes run the binary this phase was handed beside the one
// it made, and the ones that matter require the OLD binary to pull a file off the
// disk and the new one to refuse.
//
// THE FILE IS `keys` ITSELF.  tools/zstream.py writes a session's keystrokes into
// a file called `keys` in the run directory and feeds it on stdin, so there is
// always one file there to read and no runner has to plant one: `:r keys` reads
// it back, and the old binary answers `"keys" [noeol] 1L, 33B` with the
// keystrokes in the buffer.
//
// 4. THE INHERITANCE CHECK.  whim's Phase 80 gave every row its shortest
// abbreviation and made a match require at least that many characters, so a
// removed name cannot be inherited by the next row -- but that is an argument,
// and `:r` silently becoming `:redo` is exactly the shape of bug CLAUDE.md
// records for `:help` -> `:helpclose`.  Six spellings are typed and each must
// answer E492 now and something else before; `:redo`, `:redraw`, `:registers`
// and `:reg` are required not to move at all.
//
// 5. A REAL TERMINAL.  Every probe above went through a pipe.  The session types
// `:r <file>` on a pty, where the file is one the RUNNER wrote -- the editor has
// had no way to write one since phase 89 -- and the old binary puts its line in
// the buffer while this one answers E492.  An ordinary editing session beside it
// is required to be identical.
//
// WHAT IS NOT ASSERTED, and why.  `E319: Sorry, the command is not available in this
// version` is what whim's do_shell()/do_filter() stubs answered, so it never reaches
// a SNAPSHOT: the message is drawn, a `Press ENTER` prompt follows and the next
// redraw wipes the line before the cursor comes back, which is where
// tools/zscreen.py takes its picture.  It is in the STREAM, so that is where the
// probe looks -- and its presence on the old binary is also the proof that no shell
// ever ran, the stub having refused before one could.
//
// A record is built the way `zcases` builds one and scrubbed the same way
// (tools/zrec.py).  tools/zstream.py's session() is not called directly because this
// check needs the raw stream beside the screens.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim90", Check) }

var z7GoneWords = []string{"ex_read", "do_bang", "do_shell", "do_filter", "check_secure",
	"prevcmd_is_set", "prevcmd", "CMD_read", "usefilter"}

// z7GoneStrings lose their last speaker.  `"read"` is the command name; the four
// errors were said by the five functions above and by nothing else.
var z7GoneStrings = []string{`"read"`, "E484: Can't open file", "E319: Sorry",
	"E34: No previous command", "E12: Command not allowed"}

// z7Kept is the BARE WORDS, each with the reason it is not this phase's.  A loop
// that wanted zero for any of these would fail on a correct phase.
//
// `shell` was one of them and is not any more, and the reason is a measurement
// rather than a relaxation: its single mention here was `"shell"` in
// p_bo_values[], the list of things 'belloff' may name.  In the
// residue text that list was packed several values to a line, so the Part I
// phases that drop the values whose feature is gone could only reach the ones
// that stood alone; the canonical text writes one value per line and the same
// edits now reach the rest -- `"complete"` between q12 and q41, `"shell"` and
// `"wildmode"` between q41 and q63.  Measured on this branch: `grep -cw shell`
// is 0 in q82, q86, q88 and q89 -- the word is gone from the file before this
// phase is handed anything, so there is no site left here to pin.
var z7Kept = map[string]int{
	"secure": 11, "read": 3, "readfile": 5, "read_buffer": 17, "open_buffer": 6,
	"read_edit": 2, "readonly": 4, "filter": 2, "check_fname": 4,
}

var z7Later = []string{"check_changed", "do_ecmd", "setfname", "otherfile", "fix_fname",
	"b_ffname", "b_fname", "getexline", "exe_commands", "nv_error"}

// Whim90 is phase 90's check: the way to read a file.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim90 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "noread", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT, oldT := string(src), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }

	// --- 1. what the sweep took ----------------------------------------------
	for _, g := range z7GoneWords {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	for _, g := range z7GoneStrings {
		if n := check.CountLinesWith(src, g); n != 0 {
			return stop("the string '%s' still has %d mentions", g, n)
		}
	}
	r.Say("six functions, the prevcmd static, the usefilter field and five strings at 0 mentions -- all but the field taken by the sweep")

	// --- 2. and everything that must NOT be at zero --------------------------
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	names := make([]string, 0, len(z7Kept))
	for n := range z7Kept {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		want := z7Kept[name]
		if k := count(newT, name); k != want {
			why := "something survived that should not have"
			if k < want {
				why = "this phase reached too far"
			}
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d -- %s", name, k, want, why))
		}
	}
	// E32 stays and E484 goes: the two are the whole difference between "no
	// file name was given" and "the file could not be opened", and only the
	// second was ex_read's.
	if strings.Count(newT, "E32: No file name") != 1 {
		fail = append(fail, "E32: No file name went, and it is reachable through check_fname() from do_ecmd(): it is not this phase's")
	}
	if strings.Contains(newT, "E484: Can't open file") {
		fail = append(fail, "E484: Can't open file survives, and ex_read was its last speaker")
	}
	for _, kept := range z7Later {
		if count(newT, kept) == 0 {
			fail = append(fail, fmt.Sprintf("%s went, and it is a later phase's", kept))
		}
	}
	if !check.Z7QRow.MatchString(newT) {
		fail = append(fail, "the 'Q' row is no longer nv_error's, and phase 87 put it there")
	}
	rows := check.Z6RowRe.FindAllString(newT, -1)
	got, _ := harness.CommandNamesIn(src, "whim-vim.c")
	if len(rows) != 104 || len(got) != 104 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 104", len(rows), len(got)))
	}
	if check.Contains(got, "read") {
		fail = append(fail, ":read is still a command name")
	}
	for _, name := range []string{"redo", "redraw", "registers", "edit", "print", "append"} {
		if !check.Contains(got, name) {
			fail = append(fail, fmt.Sprintf(":%s went, and it is not this phase's", name))
		}
	}
	if !strings.Contains(newT, "static_assert(sizeof(cmdnames) / sizeof(cmdnames[0]) == CMD_SIZE") {
		fail = append(fail, "the static_assert on the row count went, and it is what catches an enumerator removed without its row")
	}
	// The fold, from the other side: nothing assigns or tests a filter flag,
	// and the two functions whose conditions carried it are still there.
	for _, fn := range []string{"do_one_cmd", "expand_filename"} {
		if _, _, ok := cutil.FindDefinition(src, cutil.Blank(src), fn); !ok {
			fail = append(fail, fmt.Sprintf("%s went, and this phase only folded six tests inside it", fn))
		}
	}
	if count(oldT, "usefilter") != 10 || count(oldT, "check_secure") != 3 {
		fail = append(fail, fmt.Sprintf("the input is not the file this phase was written against: usefilter %d (10), check_secure %d (3)",
			count(oldT, "usefilter"), count(oldT, "check_secure")))
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont(`a "no mention anywhere" check fails on a correct phase here:`)
		r.Cont("`secure` is not check_secure, `read` is a libc call and an")
		r.Cont("E222 string, and readfile() belongs to a later phase.")
		return harness.ErrReported
	}
	r.Say("kept: secure 11, read 3, readfile 5, read_buffer 17, open_buffer 6, check_fname 4 with E32 -- and E484 has no speaker left")
	r.Say("table: 104 rows, names() reads 104, static_assert in place, 4 rows above create_cmdidxs's floor of 100")

	// --- 3. the compile, the linkage and the libc surface --------------------
	// NOTHING IS FREED, stated as an EQUALITY: `:read` reached readfile(),
	// which the startup path still uses, and the shell stubs never called a
	// shell.  A symbol going would mean the cut reached into P8's phase; a
	// symbol arriving would mean the sweep left something that now links.
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		r.Say("the libc surface moved, and this phase frees nothing:")
		r.Cont("  gone: %s ", strings.Join(check.Comm23(before, after), " "))
		r.Cont("  came: %s ", strings.Join(check.Comm23(after, before), " "))
		r.Cont("  open, access and read are the byte-reader phase's (GOALS.md II.3b P8)")
		return harness.ErrReported
	}
	for _, keep := range []string{"open", "read", "close", "stat"} {
		if !check.Contains(after, keep) {
			return stop("%s is gone, and it is the byte-reader phase's", keep)
		}
	}
	r.Say("symbols %s -> %s, the same set: this phase removes two commands, not the read path",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")),
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 4. the binary -------------------------------------------------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	old, _ := filepath.Abs(filepath.Join(state, "old"))
	now, _ := os.ReadFile(f)
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes", beforeLines, check.CountLines(now), check.SizeOf(bin))

	// --- 5. the probes, on both binaries -------------------------------------
	if err := z7Probes(r, old, bin); err != nil {
		return err
	}
	// --- 6. a real terminal, and a file the runner wrote ---------------------
	return z7Pty(r, old, bin)
}
