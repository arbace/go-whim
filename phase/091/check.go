package p091

// Whim phase 91, the check -- nothing can point the editor at another file any more.
// See phase/091/edit.go, and GOALS.md.
//
// Runs after phase/091/edit.go and the sweep tools/phaserun.sh runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from, which is the left-hand side of every
// before-and-after count below.
//
// SIX THINGS ARE PROVED HERE, and the fourth is the only one that can say what this
// phase is actually for.
//
// 1. THE CUT, which is the sweep's work and not the edit's.  Seventeen functions go,
// none of them named by the edit -- do_ecmd (328 lines), get_visual_text,
// check_lnums_both, do_exedit, nv_gotofile, grab_file_name, prepare_help_buffer,
// u_unch_branch, text_or_buf_locked, reset_VIsual, reset_VIsual_and_resel,
// delbuf_msg, ex_edit, u_unchanged, otherfile, check_lnums and getargopt -- with
// two struct fields (exarg_T.read_edit and one more the sweep found), the five
// CMD_ enumerators, EX_ARGOPT and seven string literals.  Listing them here is a
// RECORDING of what the sweep did, which is the only place a list of removed
// names belongs.
//
// THE TRAPS, ALL MEASURED, that make a copied "every name at zero mentions" loop
// the wrong check:
// * `check_lnums` and `check_lnums_both`, `reset_VIsual` and
// `reset_VIsual_and_resel`, `u_unchanged` and `u_unch_branch`, `do_ecmd` and
// `do_ecmd_cmd`, `otherfile` and `otherfile_buf` are five pairs where one
// name is a prefix of another and only one of each pair goes.  Every count
// here is `\b`-anchored for that reason.
// * `"edit"` REACHES ZERO AND `"ex"` DOES NOT.  `getargopt()`'s `++edit`
// strncmp was the last speaker of `"edit"` once the row went, and anchor 6
// takes it; `"ex"` survives as one word of `'belloff'`'s value list.  A
// "no mention of edit anywhere" check fails on a correct phase, and a
// "both survive" check fails on this one.
// * `E447: Can't find file "%s" in path` SURVIVES.  nv_gotofile() was not its
// only speaker, so the message the `gf` probe looks for on the OLD binary is
// still in the source afterwards -- it is the KEY that went, not the string.
// * `readonlymode`, `do_ecmd_cmd` and `do_ecmd_lnum` are left WRITE-ONLY rather
// than removed, and are asserted at their counts.  `readonlymode` is FALSE
// for ever, `do_ecmd_lnum` is written through eval_vars() which is the
// buffer-name phase's, and a struct member that is only written draws no
// warning from anything.
//
// 2. THE LINE AGAINST THE PHASE THAT STOPS READING A BYTE (GOALS.md II.3b P8), stated
// as counts so that taking any of it here would fail rather than widen quietly:
// `readfile` keeps EXACTLY 5 mentions -- its prototype, its definition and the
// three calls in read_buffer() and open_buffer() -- and `read_buffer` 17.
// `open_buffer` goes 6 -> 5, and that is the one number GOALS.md II.3c got
// backwards: do_ecmd was a caller of `open_buffer`, NOT of `readfile`, so what
// this phase costs the read path is one call site and nothing else.  The `uses`
// line the plan wrote for P8 is corrected there.
//
// 3. `'undoreload'` IS NOT THIS PHASE'S.  `p_ur`'s only reader was inside do_ecmd,
// so after this phase it is a global with an option row and nothing that reads
// it -- which is the options phase's to remove, not this one's: removing a row
// changes what `:set` answers and nothing here sweeps `:set`, so the delta could
// not be checked, and `orphanopts` refuses the opposite direction.  It is
// asserted at exactly 2 mentions WITH its row, and the manifest carries
// `uses options:94 files:91 mechanical` for the phase that takes it.
//
// 4. THE PROBES, on BOTH binaries, because THE CORPUS CANNOT SEE A FILE BEING
// OPENED.  The two cases it does see are `cmd_edit`, which types `:edit` with no
// file name and has only ever recorded `E37: No write since last change`, and
// `key_gf`, which presses `gf` on a word naming nothing and recorded E447.  A
// declared delta of "those two moved" is therefore consistent with a phase that
// changed two error messages and left do_ecmd() reachable.  So the probes run the
// binary this phase was handed beside the one it made, and the ones that matter
// require the OLD binary to pull a file off the disk and the new one to refuse.
//
// THE FILE IS `keys` ITSELF.  tools/zstream.py writes a session's keystrokes into
// a file called `keys` in the run directory and feeds it on stdin, so there is
// always one file there to open and no runner has to plant one: `:e! keys` loads
// it, and WHAT PROVES THE BYTES ARRIVED IS THE ESCAPE IN THEM -- the keystroke
// file holds `...\x1b:q!\r`, which tools/zscreen.py draws as `^[:q!^M`, and an
// Escape can only be in the buffer if the file was opened.
//
// AND THE FOUR COMMANDS ARE PROVED TO HAVE BEEN `:edit` IN DISGUISE: `:ex! keys`
// and `:visual! keys` load the file exactly as `:e! keys` does, `:view! keys`
// loads it AND makes `:set ro?` answer `readonly`, and `:enew!` empties the
// buffer.  That is do_exedit's thirty lines, measured from the outside.
//
// 5. THE KEYS, AND THE HAZARD THIS PHASE DOES NOT HAVE.  No `nv_cmds[]` row is
// touched: `gf`, `gf`, `[f` and `]f` are arms inside two handlers whose `g`, `[`
// and `]` rows dispatch dozens of other keys.  CLAUDE.md's twelve-phase arrow-key
// bug was a deleted row under a precomputed index, and the general guard is
// `nvidx`, which tools/phasecheck.sh runs.  The specific one is here:
// FIFTY `g*`, `[` and `]` keys are pressed on both binaries and exactly four must
// move, which is what proves the two large handlers survived the two cuts inside
// them.
//
// 6. A REAL TERMINAL.  Every probe above went through a pipe.  The session types
// `:e <file>` on a pty, where the file is one the RUNNER wrote -- the editor has
// had no way to write one since phase 89 -- and the old binary puts its line in
// the buffer while this one answers E492.  An ordinary editing session beside it
// is required to be identical.
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
	"sync"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim91", Check) }

var z8GoneWords = []string{"do_ecmd", "get_visual_text", "check_lnums_both", "do_exedit",
	"nv_gotofile", "grab_file_name", "prepare_help_buffer", "u_unch_branch",
	"text_or_buf_locked", "reset_VIsual", "reset_VIsual_and_resel", "delbuf_msg",
	"ex_edit", "u_unchanged", "otherfile", "check_lnums", "getargopt",
	"CMD_edit", "CMD_enew", "CMD_ex", "CMD_view", "CMD_visual", "EX_ARGOPT", "read_edit"}

var z8GoneStrings = []string{`"edit"`, `"enew"`, `"view"`, `"visual"`,
	"E143: Autocommands unexpectedly deleted new buffer",
	"E1546: Cannot switch to a closing buffer",
	`!-~,^*,^|,^\",192-255`}

var z8Kept = map[string]int{
	"readfile": 5, "read_buffer": 17, "open_buffer": 5, "otherfile_buf": 3,
	"do_ecmd_cmd": 6, "do_ecmd_lnum": 2, "readonlymode": 5, "p_ur": 2,
	"getargcmd": 3, "EX_CMDARG": 2, "check_changed": 4, "nv_error": 46,
	"check_fname": 3, "b_ffname": 43, "b_fname": 37,
}

var z8Later = []string{"setfname", "fix_fname", "expand_filename", "eval_vars", "do_one_cmd",
	"nv_g_cmd", "nv_brackets", "getexline", "exe_commands"}

// z8EnumOK are the ten single-constant enums the sweep took with their types,
// beside the five CMD_ and EX_ARGOPT.  Anything else leaving is unaccounted for.
var z8EnumOK = map[string]bool{
	"CPO_GOTO1": true, "DOCMD_RANGEOK": true, "ECMD_FORCEIT": true, "ECMD_HIDE": true,
	"ECMD_NOWINENTER": true, "ECMD_OLDBUF": true, "ECMD_SET_HELP": true,
	"EX_ARGOPT": true, "FNAME_REL": true, "FNAME_UNESC": true, "READ_NOWINENTER": true,
}

// Whim91 is phase 91's check: every way to name another file to edit.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim91 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "noedit", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT, oldT := string(src), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim91")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// --- 1. what the sweep took ----------------------------------------------
	for _, g := range z8GoneWords {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	for _, g := range z8GoneStrings {
		if n := check.CountLinesWith(src, g); n != 0 {
			return stop("the string '%s' still has %d mentions", g, n)
		}
	}
	r.Say("seventeen functions, five enumerators, EX_ARGOPT, the read_edit field and seven string literals at 0 mentions -- all of it the sweep's, the edit named none of them")

	// --- 2. and everything that must NOT be at zero --------------------------
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	names := make([]string, 0, len(z8Kept))
	for n := range z8Kept {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		want := z8Kept[name]
		if k := count(newT, name); k != want {
			why := "something survived that should not have"
			if k < want {
				why = "this phase reached too far"
			}
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d -- %s", name, k, want, why))
		}
	}
	// E447 STAYS AND `"ex"` STAYS, both the opposite direction from section 1.
	// E447 is what the `gf` probes look for on the old binary; if it had gone
	// with nv_gotofile() the probe would still pass and the assertion would be
	// a coincidence.
	if strings.Count(newT, `E447: Can't find file `) != 1 {
		fail = append(fail, "E447 went, and nv_gotofile() was not its only speaker -- the KEY goes here, not the message")
	}
	if strings.Count(newT, `"ex"`) != 1 {
		fail = append(fail, `"ex" is not the one word of 'belloff's value list that it should be now that the row has gone`)
	}
	if !strings.Contains(newT, "E37: No write since last change") {
		fail = append(fail, "E37 went, and check_changed() is the :q phase's")
	}
	if strings.Count(newT, "E32: No file name") != 1 {
		fail = append(fail, "E32: No file name went, and it is reachable through check_fname()")
	}
	for _, kept := range z8Later {
		if count(newT, kept) == 0 {
			fail = append(fail, fmt.Sprintf("%s went, and it is a later phase's or this phase only cut inside it", kept))
		}
	}
	if !check.Z7QRow.MatchString(newT) {
		fail = append(fail, "the 'Q' row is no longer nv_error's, and phase 87 put it there")
	}
	for _, key := range []string{`'g'`, `'\['`, `'\]'`} {
		if !regexp.MustCompile(`(?m)^ *\{` + key + `, nv_`).MatchString(newT) {
			fail = append(fail, fmt.Sprintf("the %s row of nv_cmds[] went, and this phase only cut arms inside the two handlers it names", key))
		}
	}
	if !strings.Contains(newT, "(char_u *)&p_ur, PV_NONE") {
		fail = append(fail, "'undoreload' lost its option row, and that is the options phase's: a row removed here would change what :set answers, which nothing this pipeline records sweeps")
	}
	rows := check.Z6RowRe.FindAllString(newT, -1)
	got, _ := harness.CommandNamesIn(src, "whim-vim.c")
	if len(rows) != 99 || len(got) != 99 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 99", len(rows), len(got)))
	}
	for _, name := range []string{"edit", "enew", "ex", "view", "visual"} {
		if check.Contains(got, name) {
			fail = append(fail, fmt.Sprintf(":%s is still a command name", name))
		}
	}
	for _, name := range []string{"earlier", "file", "vglobal", "vmap", "quit", "print", "append"} {
		if !check.Contains(got, name) {
			fail = append(fail, fmt.Sprintf(":%s went, and it is not this phase's", name))
		}
	}
	if !strings.Contains(newT, "static_assert(sizeof(cmdnames) / sizeof(cmdnames[0]) == CMD_SIZE") {
		fail = append(fail, "the static_assert on the row count went, and it is what catches an enumerator removed without its row")
	}
	// ANCHOR 3, FROM THE OTHER SIDE: the one anchor outside the table and the
	// keys.  An edit shaped like the table forgets it, and `:file` is what must
	// still be exempt.
	m := check.Z8Lock.FindString(newT)
	if m == "" {
		fail = append(fail, "do_one_cmd's curbuf_locked() test went, and this phase only removed one conjunct of it")
	} else if strings.Contains(m, "CMD_edit") || !strings.Contains(m, "CMD_file") {
		fail = append(fail, fmt.Sprintf("the curbuf_locked() exemption is %s -- CMD_edit was to go and CMD_file to stay, `:file` being the buffer-name phase's",
			check.CutilRepr(strings.TrimSpace(m))))
	}
	if count(oldT, "do_ecmd") != 4 || count(oldT, "ex_edit") != 6 || count(oldT, "EX_ARGOPT") != 6 {
		fail = append(fail, fmt.Sprintf("the input is not the file this phase was written against: do_ecmd %d (4), ex_edit %d (6), EX_ARGOPT %d (6)",
			count(oldT, "do_ecmd"), count(oldT, "ex_edit"), count(oldT, "EX_ARGOPT")))
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont(`a "no mention anywhere" check fails on a correct phase here:`)
		r.Cont("check_lnums_both, reset_VIsual_and_resel, u_unch_branch,")
		r.Cont("do_ecmd_cmd and otherfile_buf each contain a name that goes,")
		r.Cont(`E447 keeps another speaker, and "ex" is a belloff value.`)
		return harness.ErrReported
	}
	r.Say("kept: readfile 5, read_buffer 17, open_buffer 5 (do_ecmd was the fifth caller, NOT a caller of readfile -- GOALS.md II.3c is corrected), E447 and E32 with their other speakers")
	r.Cont("left write-only and named rather than removed: do_ecmd_cmd 6, do_ecmd_lnum 2, readonlymode 5 (and FALSE for ever), p_ur 2 with its row -- 'undoreload' is the options phase's")
	r.Cont("table: 99 rows, names() reads 99, static_assert in place, and the floor is 80: 19 rows of margin (GOALS.md II decision 8)")

	// --- 3. the compile, the linkage and the libc surface --------------------
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
	r.Say("symbols %s -> %s, the same set: this phase removes five commands and four keys, not the read path",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")),
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 4. the enumerators, before and after --------------------------------
	// Eighty-seven survivors renumber here, and every one must be a CMD_*:
	// cmdnames[] is designated, so a row lands at its own enumerator whatever
	// the numbering is, but nothing in the build would notice if some OTHER
	// family had moved with them.  Two dumps, taken concurrently.
	evOld, evNew := filepath.Join(tmp, "ev.old"), filepath.Join(tmp, "ev.new")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		exec.Command("sh", "tools/enumvals.sh", filepath.Join(state, "old.c"), evOld).Run()
	}()
	if err := exec.Command("sh", "tools/enumvals.sh", f, evNew).Run(); err != nil {
		return fmt.Errorf("tools/enumvals.sh refused")
	}
	wg.Wait()
	if err := z8Enums(r, check.ReadFile(evOld), check.ReadFile(evNew)); err != nil {
		return err
	}

	// --- 5. the binary -------------------------------------------------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	old, _ := filepath.Abs(filepath.Join(state, "old"))
	now, _ := os.ReadFile(f)
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes", beforeLines, check.CountLines(now), check.SizeOf(bin))

	// --- 6, 7, 8 -------------------------------------------------------------
	if err := z8Probes(r, old, bin); err != nil {
		return err
	}
	if err := z8Keys(r, old, bin); err != nil {
		return err
	}
	return z8Pty(r, old, bin)
}

func z8Enums(r *check.Rep, oldTxt, newTxt string) error {
	load := func(s string) map[string]string {
		m := map[string]string{}
		for _, l := range strings.Split(s, "\n") {
			if i := strings.LastIndexByte(l, '='); i > 0 {
				m[l[:i]] = l[i+1:]
			}
		}
		return m
	}
	o, n := load(oldTxt), load(newTxt)
	var gone, came, moved, stray, bad []string
	for k := range o {
		if _, ok := n[k]; !ok {
			gone = append(gone, k)
		} else if n[k] != o[k] {
			moved = append(moved, k)
		}
	}
	for k := range n {
		if _, ok := o[k]; !ok {
			came = append(came, k)
		}
	}
	sort.Strings(gone)
	sort.Strings(came)
	sort.Strings(moved)
	for _, k := range moved {
		if !strings.HasPrefix(k, "CMD_") {
			stray = append(stray, k)
		}
	}
	for _, k := range gone {
		if !strings.HasPrefix(k, "CMD_") && !z8EnumOK[k] {
			bad = append(bad, k)
		}
	}
	if len(came) > 0 || len(stray) > 0 || len(bad) > 0 {
		if len(came) > 0 {
			r.Say("enumerators arrived: %s", strings.Join(came, " "))
		}
		if len(stray) > 0 {
			r.Say("enumerators outside CMD_ moved value: %s", strings.Join(stray, " "))
		}
		if len(bad) > 0 {
			r.Say("enumerators went that this phase does not account for: %s", strings.Join(bad, " "))
		}
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d: %d gone (the five CMD_, EX_ARGOPT, and ten single-constant enums the sweep took with their types), %d renumbered and every one a CMD_, none arriving",
		len(o), len(n), len(gone), len(moved))
	return nil
}

var _ = io.Discard
