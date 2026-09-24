package p089

// Whim phase 89, the check -- nothing can put bytes on a disk any more.
// See phase/089/edit.go, and GOALS.md.
//
// Runs after phase/089/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from, which is the left-hand side of every
// before-and-after count below.
//
// FIVE THINGS ARE PROVED HERE, and the third is the only one that can say what this
// phase is actually for.
//
// 1. THE CUT, which is the sweep's work and not the edit's.  Nineteen functions go,
// none of them named by the edit: ex_write, ex_update, ex_exit, do_write,
// check_writable, check_overwrite, not_writing, check_readonly,
// check_file_readonly, buf_write, buf_write_bytes, check_mtime, time_differs,
// write_eintr, vim_fexists, mch_setperm, mch_fsetperm, mch_nodetype and
// u_update_save_nr, with one struct field (exarg_T.append) and fourteen
// enumerators.  Listing them here is a RECORDING of what the sweep did, which is
// the only place a list of removed names belongs.
//
// TWO TRAPS, BOTH MEASURED, that make a phase-4-style "every name at zero
// mentions" loop the wrong check:
// * `check_readonly` is ALSO A LOCAL, in readfile() -- `int check_readonly;`
// and three uses.  After this phase `grep -cw` is 4, not 0, and a loop that
// wanted 0 would fail on a correct phase.  What must be gone is the
// DEFINITION, `^check_readonly(`, and the four survivors must all be inside
// readfile(), which is asserted rather than assumed.
// * `"write"` SURVIVES, as the name of the 'write' option, and `E32: No file
// name` survives with it, still reachable through check_fname() from
// do_ecmd() and ex_bang().  A "no mention of write anywhere" check would fail
// on a correct phase just as surely.
//
// 2. THE TABLE.  105 rows, create_cmdidxs.names() reading exactly those, the
// static_assert still there, and the five rows of margin above the tool's floor
// of 100 said out loud -- the :edit phase is the one that must lower it
// (GOALS.md II.3a).
//
// 3. THE PROBES, on BOTH binaries, because THE CORPUS CANNOT SEE WRITING.  Every
// one of `zcases`'s 102 cases types its own text and never names a file:
// `cmd_write` types `:write` with no file name, so the baseline it is compared
// against records `E32: No file name` -- an editor that FAILED to write.  A
// declared delta of "cmd_write and zz_key moved" is therefore consistent with a
// phase that changed one error message and left buf_write() reachable.  So the
// probes run the binary this phase was handed beside the one it made, in a
// directory they KEEP, and the six that matter require the old binary to leave a
// file on the disk and the new one to leave none.
//
// 4. THE INHERITANCE CHECK.  whim's Phase 80 gave every row its shortest
// abbreviation and made a match require at least that many characters, so a
// removed name cannot be inherited by the next row (GOALS.md II.3a) -- but that
// is an argument, and `:w` silently becoming `:winsize` is exactly the shape of
// bug CLAUDE.md records for `:help` -> `:helpclose`.  Eight spellings are typed
// and each must answer E492 now and something else before.
//
// 5. A REAL TERMINAL.  `:wq` on a pty is how a person leaves this editor, and every
// probe above went through a pipe.  The session is run on both binaries: the old
// one writes the file and exits, the new one answers E492 and writes nothing.
// phase/STAGES.md says why that needs no `apart 85 89`: it opens the file as an
// ARGUMENT and phase 88 already made that an unknown option, so it never reaches
// the `:wq` -- measured identically on a phase 88 and a phase 89 tree.
//
// A record is built the way `zcases` builds one and scrubbed the same way
// (tools/zrec.py), with one section added: the files the run left behind.
// tools/zstream.py's session() throws its directory away, which is the one thing a
// phase about writing files cannot do, so the runner is here.

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

func init() { check.Register("whim89", Check) }

var w89GoneFuncs = []string{
	"ex_write", "ex_update", "ex_exit", "do_write", "check_writable", "check_overwrite",
	"not_writing", "check_file_readonly", "buf_write", "buf_write_bytes", "check_mtime",
	"time_differs", "write_eintr", "vim_fexists", "mch_setperm", "mch_fsetperm",
	"mch_nodetype", "u_update_save_nr",
}

var w89GoneEnums = []string{"CMD_write", "CMD_wq", "CMD_xit", "CMD_exit", "CMD_update", "CMD_saveas"}

// w89GoneNames are the command NAMES as strings.  `"write"` is NOT among them:
// it is the 'write' option's name and it stays.
var w89GoneNames = []string{`"wq"`, `"xit"`, `"exit"`, `"update"`, `"saveas"`}

var w89Later = []string{"check_changed", "no_write_message", "do_bang", "check_fname",
	"setfname", "otherfile", "fix_fname", "readfile", "b_ffname"}

// Whim89 is phase 89's check: every way to write a file.
//
// THE CORPUS CANNOT SEE WRITING, which is why the probes exist.  Every one of
// zcases's 102 cases types its own text and never names a file, so `cmd_write`
// types `:write` with no file name and the baseline records `E32: No file name`
// -- an editor that FAILED to write.  A declared delta of "cmd_write and zz_key
// moved" is therefore consistent with a phase that changed one error message
// and left buf_write() reachable.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim89 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "nowrite", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT := string(src)
	oldT := check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim89")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// --- 1. what the sweep took ----------------------------------------------
	for _, g := range append(append([]string{}, w89GoneFuncs...), w89GoneEnums...) {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	for _, g := range w89GoneNames {
		if n := check.CountLinesWith(src, g); n != 0 {
			return stop("the string %s still has %d mentions", g, n)
		}
	}
	if check.HasLinePrefix(src, "check_readonly(") {
		return stop("check_readonly() is still defined")
	}
	r.Say("18 functions and the six enumerators at 0 mentions, all taken by the sweep")

	// --- 2. and the two things that must NOT be at zero ----------------------
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	// check_readonly: the function is gone and the LOCAL in readfile() is not.
	if n := count(newT, "check_readonly"); n != 4 {
		fail = append(fail, fmt.Sprintf("check_readonly has %d mentions, expected the 4 that are readfile()'s local and its uses", n))
	} else {
		a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), "readfile")
		if !ok {
			fail = append(fail, "readfile() is not defined, and it is not this phase's to take")
		} else {
			outside := 0
			for _, m := range regexp.MustCompile(`\bcheck_readonly\b`).FindAllStringIndex(newT, -1) {
				if m[0] < a || m[0] >= z {
					outside++
				}
			}
			if outside > 0 {
				fail = append(fail, fmt.Sprintf("%d mentions of check_readonly are outside readfile()", outside))
			}
		}
	}
	for _, lw := range []struct {
		lit  string
		want int
	}{{`"write"`, 1}, {"E32: No file name", 1}} {
		if k := strings.Count(newT, lw.lit); k != lw.want {
			fail = append(fail, fmt.Sprintf("%s occurs %d times and must occur %d: it is not this phase's",
				cutil.PyRepr(lw.lit), k, lw.want))
		}
	}
	for _, opt := range []string{"p_fs", "p_write", "p_wa"} {
		if k := count(newT, opt); k != 2 {
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected 2 -- its definition and its option row, which are the options phase's to take", opt, k))
		}
	}
	if count(newT, "append") != 1 || !strings.Contains(newT, "[CMD_append]") {
		fail = append(fail, fmt.Sprintf("exarg_T.append is not the only `append` left (%d), or the :append row went", count(newT, "append")))
	}
	for _, kept := range w89Later {
		if count(newT, kept) == 0 {
			fail = append(fail, fmt.Sprintf("%s went, and it is a later phase's", kept))
		}
	}
	rows := check.W89RowRe.FindAllString(newT, -1)
	got, _ := harness.CommandNamesIn(src, "whim-vim.c")
	if len(rows) != 105 || len(got) != 105 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 105", len(rows), len(got)))
	}
	for _, name := range []string{"write", "wq", "xit", "exit", "update", "saveas"} {
		if check.Contains(got, name) {
			fail = append(fail, fmt.Sprintf(":%s is still a command name", name))
		}
	}
	if !strings.Contains(newT, "static_assert(sizeof(cmdnames) / sizeof(cmdnames[0]) == CMD_SIZE") {
		fail = append(fail, "the static_assert on the row count went, and it is what catches an enumerator removed without its row")
	}
	// The libc the write side reached, counted in OCCURRENCES: stat( appears
	// more than once on a line.
	nOld := len(regexp.MustCompile(`\bstat\(`).FindAllString(oldT, -1))
	nNew := len(regexp.MustCompile(`\bstat\(`).FindAllString(newT, -1))
	if nOld != 14 || nNew != 8 {
		fail = append(fail, fmt.Sprintf("stat( is called %d times and was %d; expected 14 -> 8", nNew, nOld))
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont(`a "no mention anywhere" check fails on a correct phase here:`)
		r.Cont(`check_readonly is a local in readfile(), and "write" is an`)
		r.Cont("option name.  Both are measured, not assumed.")
		return harness.ErrReported
	}
	r.Say(`kept: check_readonly as readfile()'s local (4 mentions), "write" as the option's name, E32 through check_fname, p_fs/p_write/p_wa at their rows`)
	r.Say("table: 105 rows, names() reads 105, static_assert in place, 5 rows above create_cmdidxs's floor of 100")
	r.Say("stat( 14 -> 8 calls")

	// --- 3. the compile, the linkage and the libc surface --------------------
	// SIX SYMBOLS GO, stated as a SET and not a count: this is the first zero
	// phase that frees any.  The five a later phase owns must still be
	// undefined, so a cut reaching past this boundary fails here.
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.PhaseCheck(w, work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneSet := check.Comm23(before, after)
	came := check.Comm23(after, before)
	want := []string{"chmod", "fchmod", "fstat", "ftruncate", "lstat", "unlink"}
	sort.Strings(want)
	if strings.Join(goneSet, "\n") != strings.Join(want, "\n") || len(came) > 0 {
		r.Say("the libc surface is not what this phase frees:")
		r.Cont("  gone: %s ", strings.Join(goneSet, " "))
		r.Cont("  came: %s ", strings.Join(came, " "))
		r.Cont("  expected exactly: chmod fchmod fstat ftruncate lstat unlink")
		return harness.ErrReported
	}
	for _, keep := range []string{"stat", "open", "access", "fsync", "getcwd"} {
		if !check.Contains(after, keep) {
			return stop("%s is gone, and it is a later phase's: stat and getcwd go with the buffer's name, open and access with readfile, fsync with the options", keep)
		}
	}
	r.Say("symbols %s -> %s: exactly chmod fchmod fstat ftruncate lstat unlink, and stat/open/access/fsync/getcwd still needed",
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

	// --- 5. the probes, in a directory they keep -----------------------------
	if err := w89Probes(r, old, bin); err != nil {
		return err
	}
	// --- 6. a real terminal --------------------------------------------------
	return w89Pty(r, old, bin)
}
