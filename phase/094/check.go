package p094

// Whim phase 94, the check -- `:q` quits, and nothing refuses any more.
// See phase/094/edit.go, and GOALS.md.
//
// Runs after phase/094/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from: the left-hand side of every
// before-and-after count, and of every probe.
//
// SIX THINGS ARE PROVED HERE, and the fifth is the only one that can say what this
// phase actually did to the editor.
//
// 1. THE CUT, WHICH IS THE SWEEP'S AND NOT THE EDIT'S.  SIXTEEN functions go and the
// edit names none of them, from ONE fold.  ELEVEN OF THE SIXTEEN ARE A SURPRISE
// and are the largest part of the phase: `check_changed_any()`'s tail is "go to
// the buffer that refused", and after whim removed the buffer list and the window
// commands that tail was the last caller of the whole switch-buffer/switch-window
// island.  The list below is a RECORDING of what the sweep did, which is the only
// place a list of removed names belongs (GOALS.md core rule 1), and the COUNT is
// asserted beside it -- 1,742 definitions to 1,726 -- because a check written
// from the plan's three names (GOALS.md II.3b) would pass while the island silently went.
//
// THE TRAPS, ALL MEASURED, that make a copied loop wrong here:
// * `bufIsChanged` GOES 10 -> 7 AND MUST NOT GO TO 0, and `curbufIsChanged`
// does not move at all.  The buffer still knows it is modified: CTRL-G still
// prints `[Modified]`, the status line still draws `[+]`, `:set modified?`
// still answers.  What went is the refusal, not the state.
// * `text_locked`, `curbuf_locked` and `before_quit_autocmds` DO NOT MOVE.  All
// three return early ABOVE the anchor, so `:q` can still decline -- just not
// for the reason this phase removed.
// * `open_buffer` goes 5 -> 4, `buf_spname` 5 -> 4 and `exiting` 17 -> 13.
// Phase 92's brief pinned `open_buffer` at 5 and phase 93's check at 5; both
// were right there and would fail here, `enter_buffer()` having been one of
// its four callers.
// * `p_wh` LOOKS WRITE-ONLY AND IS NOT.  It goes 4 -> 2 -- the two reads in
// `win_enter_ext`'s callee went with the island -- and the two that are left
// are its declaration, which carries the initialiser, and one real reader in
// the frame layer.  A "uses - writes - 1 <= 0" scan reports it and is wrong.
// * `SHM_FILEINFO` leaves, and it is the `'shortmess'` `f` letter.  The letter
// is accepted and inert afterwards; that is the options phase's and the flag
// strings are not touched here.
//
// 2. NOTHING IS LEFT WRITE-ONLY, AND IT IS COMPUTED ON BOTH TEXTS.  A file-scope
// static that is assigned and never read draws no warning, deadsweep.py acts on
// warnings, and nothing else here looks -- phase 93 had to take `readonlymode` by
// hand for exactly that shape.  So the scan runs on the source this phase was
// handed as well as on the one it made, and the two answers must be the SAME SET
// and must be exactly `vim_ignored`, upstream's sink for a return value that is
// deliberately ignored.  It finding something either side is what makes an empty
// answer a scan failure rather than a phase succeeding.  `p_wh` is what the
// OBVIOUS scan gets wrong and this one does not: a "uses - writes - 1 <= 0" count
// charges its initialiser to the writes.  The TWO STRUCT FIELDS that did become
// write-only are the edit's extra A and are already gone -- no warning and no tool
// sees a member that is only written.
//
// 3. THE LINE AGAINST THE PHASES AFTER THIS ONE, stated as counts so that reaching
// into one would fail here rather than widen quietly: `p_ro` 2 and `p_ur` 2 WITH
// their option rows (the options phase's), `read_cmd_fd` 12 (the terminal's),
// and `scriptin` 8, `redir_fd` 6 and `vim_fsync` 3 (the FILE* phase's), with
// `fclose`, `getc`, `putc` and `fsync` asserted STILL undefined.
//
// 4. THE ENUMERATORS, DUMPED EITHER SIDE.  Twelve go -- the four `CCGD_`, the two
// `DOBUF_`, `SHM_FILEINFO` and the five `WEE_` -- and NOTHING RENUMBERS, because
// typereach.py takes whole anonymous definitions and a whole definition leaving
// takes no survivor's value with it.  That is the opposite of phase 93, where 85
// moved, and it is worth the four seconds either side to say so rather than
// assume it.
//
// 5. THE PROBES, ON BOTH BINARIES, BECAUSE THE CORPUS CANNOT SEE THE EXIT STATUS.
// The one case it sees is `quit_modified`, and it ends with a trailing `:q!` --
// which quits the OLD binary too, so the status is 0 either side and what the
// recording holds is a message that changed.  `q_alone` is the probe that is not:
// `ihello<Esc> :set nopaste :q` AND NOTHING AFTER IT.  The old binary draws
// `E37: No write since last change (add ! to override)`, runs out of stdin,
// prints `Vim: Finished.` and exits 1; this one quits on the `:q` and exits 0.
// That difference -- 1 to 0 -- is the whole phase measured from outside, and no
// recording can see it.
//
// THE MUST-NOT-MOVE HALF IS REQUIRED TO BE DOING SOMETHING, or "it did not move"
// is two failures agreeing: `ctrl_g` must say `[Modified]`, `editing` must say
// `alpha`, `cquit` must exit 1 on both, `q_clean` must exit 0 on both with no E37
// anywhere -- it is `:q` on an UNMODIFIED buffer, which took the else arm before
// this phase and takes it now -- and `zz_key`/`zq_key` must leave the same screen.
//
// 6. A REAL TERMINAL, because every probe above went through a pipe.  On a pty the
// old binary answers E37 to `:q` and is still running, so the `:q!` after it
// reaches the command line; this one draws no E37 at all.  THE OPPOSITE PAIR --
// "the `:q!` did not reach the new binary" -- IS NOT ASSERTED, and it is a race:
// once the editor has quit the pty leaves raw mode and whether the trailing
// keystrokes are echoed back depends on how fast the process exits.  What the `:q`
// quit is `q_alone`'s to say, by its exit status through a pipe.  An ordinary
// editing session beside it must be identical either side.
//
// A record is built the way `zcases` builds one and scrubbed the same way
// (tools/zrec.py).  tools/zstream.py's session() is not called directly because this
// check needs the raw stream, the exit status and the snapshot count beside the
// screens.

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
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/dead"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim94", Check) }

var z11Gone = []string{"check_changed", "check_changed_any", "no_write_message",
	"no_write_message_nobang", "not_exiting",
	"add_bufnum", "set_curbuf", "enter_buffer", "win_enter", "win_enter_ext",
	"goto_tabpage_win", "goto_tabpage_tp", "get_winopts", "find_wininfo",
	"buflist_findfpos", "buflist_getfpos",
	"w_topline_was_set", "wi_changelistidx", "SHM_FILEINFO"}

var z11GoneStrings = []string{
	"E37: No write since last change (add ! to override)",
	"E37: No write since last change",
	"E162: No write since last change for buffer ",
}

var z11Kept = map[string]int{
	"bufIsChanged": 7, "curbufIsChanged": 7, "bufIsChangedNotTerm": 3,
	"text_locked": 6, "curbuf_locked": 7, "before_quit_autocmds": 2,
	"getout": 7, "mch_exit": 8, "exiting": 13, "buf_spname": 4, "open_buffer": 4,
	"fileinfo": 3, "check_fname": 3, "p_ur": 2, "p_ro": 2, "read_cmd_fd": 12,
	"vim_fsync": 3, "scriptin": 8, "redir_fd": 6, "msg_scrolled_ign": 2,
	"nv_error": 46, "p_wh": 2,
}

var z11EnumWant = []string{"CCGD_ALLBUF", "CCGD_EXCMD", "CCGD_FORCEIT", "CCGD_MULTWIN",
	"DOBUF_GOTO", "DOBUF_UNLOAD", "SHM_FILEINFO",
	"WEE_CURWIN_INVALID", "WEE_TRIGGER_ENTER_AUTOCMDS",
	"WEE_TRIGGER_LEAVE_AUTOCMDS", "WEE_TRIGGER_NEW_AUTOCMDS", "WEE_UNDO_SYNC"}

var (
	z11Decl   = regexp.MustCompile(`(?m)^static\s+[A-Za-z_][\w \t*]*?\b(\w+)\s*(=[^;]*)?;$`)
	z11Assign = regexp.MustCompile(`^\s*(\)\s*)?([-+|&^*/]|<<|>>)?=[^=]`)
)

// Whim94 is phase 94's check: the REFUSAL, `E37: No write since last change`,
// which has had no remedy to offer since phase 89 took every `:write`.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim94 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "noquit", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT, oldT := string(src), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim94")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// --- 1. what the sweep took ----------------------------------------------
	for _, g := range z11Gone {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	for _, g := range z11GoneStrings {
		if n := check.CountLinesWith(src, g); n != 0 {
			return stop("the string '%s' still has %d mentions", g, n)
		}
	}
	r.Say("sixteen functions, two struct fields, SHM_FILEINFO and the three 'No write since last change' literals at 0 mentions -- all of it the sweep's but the two fields, and the edit named not one function")

	// --- 2. what must NOT be at zero -----------------------------------------
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	// The claim is WHICH functions one fold takes, not how many the file
	// holds: the absolute counts, 1742 -> 1726, broke the day upstream added
	// two functions this phase never touches (vim 9.2.1122, 1744 -> 1728).
	// So the set that disappeared must be exactly the sixteen named above.
	defsOld := dead.FuncDefinitions([]byte(oldT), cutil.Blank([]byte(oldT)))
	defsNew := dead.FuncDefinitions(src, cutil.Blank(src))
	want := map[string]bool{}
	for _, g := range z11Gone[:16] {
		want[g] = true
	}
	var went, extra []string
	for name := range defsOld {
		if _, ok := defsNew[name]; !ok {
			went = append(went, name)
			if !want[name] {
				extra = append(extra, name)
			}
		}
	}
	for name := range defsNew {
		if _, ok := defsOld[name]; !ok {
			extra = append(extra, "+"+name)
		}
	}
	if len(went) != 16 || len(extra) != 0 {
		sort.Strings(extra)
		fail = append(fail, fmt.Sprintf("the functions that went are %d, not the sixteen named (%d -> %d; unexpected: %v): one fold takes SIXTEEN, and eleven of them are the switch-buffer island", len(went), len(defsOld), len(defsNew), extra))
	} else {
		r.Say("the functions that went, %d -> %d, are exactly the sixteen named -- eleven of them the switch-buffer island", len(defsOld), len(defsNew))
	}
	names := make([]string, 0, len(z11Kept))
	for n := range z11Kept {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		want := z11Kept[name]
		if k := count(newT, name); k != want {
			why := "something survived that should not have"
			if k < want {
				why = "this phase reached too far"
			}
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d -- %s", name, k, want, why))
		}
	}
	if !regexp.MustCompile(`(?m)^static long\s+p_wh = 1L;$`).MatchString(newT) {
		fail = append(fail, "p_wh lost its initialising declaration, which is the half of its two mentions that makes it look write-only")
	}
	if !regexp.MustCompile(`(?m)^\s*m = p_wh \+ `).MatchString(newT) {
		fail = append(fail, `p_wh lost its one real reader, and a "uses - writes - 1 <= 0" scan would then be right about it for the first time`)
	}
	woOld, woNew := z11WriteOnly(oldT), z11WriteOnly(newT)
	if strings.Join(woNew, " ") != strings.Join(woOld, " ") || strings.Join(woNew, " ") != "vim_ignored" {
		fail = append(fail, fmt.Sprintf("the write-only scan reports %s where the input reports %s; both must be exactly vim_ignored, which is upstream's sink for an ignored return value and was write-only before this phase",
			orNothing(woNew), orNothing(woOld)))
	}
	if strings.Contains(newT, "No write since last change") {
		fail = append(fail, "E37 or E162 survives, and this phase removes the refusal that said them -- phases 89 to 93 all assert the opposite, which is why none of them can share a stage with this one")
	}
	if !strings.Contains(oldT, "No write since last change") {
		fail = append(fail, "the input did not refuse, so this phase is being checked against a file it was not written for")
	}
	for _, lw := range []struct {
		lit  string
		want int
	}{{"[Modified]", 1}, {"[No Name]", 2}, {"E32: No file name", 1}} {
		if k := strings.Count(newT, lw.lit); k != lw.want {
			fail = append(fail, fmt.Sprintf("%s occurs %d times, expected %d -- the buffer still knows it is modified and still has no name; what went is the refusal",
				check.CutilRepr(lw.lit), k, lw.want))
		}
	}
	Body := ""
	if a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), "ex_quit"); ok {
		Body = newT[a:z]
	}
	if Body == "" {
		fail = append(fail, "ex_quit is gone, and this phase folds it rather than removing it")
	}
	if strings.Contains(Body, "save_exiting") || strings.Contains(Body, "exiting = TRUE") {
		fail = append(fail, "ex_quit still saves and sets `exiting`: getout() does both itself and never returns, which is why extra B is honest")
	}
	if strings.Count(Body, "getout(0);") != 1 {
		fail = append(fail, fmt.Sprintf("ex_quit does not end in exactly one getout(0);: %s", check.CutilRepr(tailN(Body, 120))))
	}
	for _, kept := range []string{"text_locked", "curbuf_locked", "before_quit_autocmds"} {
		if !strings.Contains(Body, kept) {
			fail = append(fail, fmt.Sprintf("ex_quit no longer calls %s, and all three early returns are above the anchor and not this phase's", kept))
		}
	}
	zet := ""
	if a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), "nv_Zet"); ok {
		zet = newT[a:z]
	}
	if strings.Count(zet, `do_cmdline_cmd((char_u *)"q!")`) != 2 {
		fail = append(fail, "nv_Zet does not run `q!` for both ZZ and ZQ: it has since phase 89 and rewriting either string would move a record phase 89 declared")
	}
	rows := check.Z6RowRe.FindAllString(newT, -1)
	got, errN := harness.CommandNamesIn(src, "whim-vim.c")
	if errN != nil {
		fail = append(fail, fmt.Sprintf("the checked parser refuses this table -- the row floor is no longer below 98: %s", errN))
	}
	if len(rows) != 98 || len(got) != 98 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 98 -- this phase removes no command", len(rows), len(got)))
	}
	for _, name := range []string{"quit", "cquit", "registers", "redo", "print"} {
		if !check.Contains(got, name) {
			fail = append(fail, fmt.Sprintf(":%s went, and this phase removes no row at all", name))
		}
	}
	if !strings.Contains(newT, "static_assert(sizeof(cmdnames) / sizeof(cmdnames[0]) == CMD_SIZE") {
		fail = append(fail, "the static_assert on the row count went, and it is what catches an enumerator removed without its row")
	}
	if !check.Z7QRow.MatchString(newT) {
		fail = append(fail, "the 'Q' row is no longer nv_error's, and phase 87 put it there")
	}
	for _, p := range []struct{ opt, v string }{{"'undoreload'", "p_ur"}, {"'readonly'", "p_ro"}} {
		if !strings.Contains(newT, "(char_u *)&"+p.v+",") {
			fail = append(fail, fmt.Sprintf("%s lost its option row, and that is the options phase's: a row removed here would change what :set answers", p.opt))
		}
	}
	// What the sixteen cost the old file, as a difference rather than a number.
	for _, p := range []struct {
		Name string
		want int
	}{{"check_changed", 4}, {"check_changed_any", 2}, {"not_exiting", 4},
		{"bufIsChanged", 10}, {"open_buffer", 5}, {"buf_spname", 5},
		{"exiting", 17}, {"p_wh", 4}, {"SHM_FILEINFO", 2}} {
		if count(oldT, p.Name) != p.want {
			fail = append(fail, fmt.Sprintf("the input is not the file this phase was written against: %s %d, expected %d",
				p.Name, count(oldT, p.Name), p.want))
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("the count is the assertion: 1,742 definitions -> 1,726, ELEVEN of the sixteen being the switch-buffer/switch-window island that hung off check_changed_any()'s tail and that no plan predicted")
	r.Cont("kept: bufIsChanged 7 and curbufIsChanged 7 -- the buffer still knows it is modified, so [Modified], [+] and `:set modified?` are untouched -- text_locked 6, curbuf_locked 7 and before_quit_autocmds 2, all three still able to decline above the anchor")
	r.Cont("p_wh 4 -> 2 and it is NOT write-only, which is what the obvious scan gets wrong: over every file-scope static, excluding the declaration by position, the only write-only one is vim_ignored -- upstream's sink for an ignored return value, and the same answer on the input")
	r.Cont("the later phases' line: p_ro 2 and p_ur 2 with their rows (the options phase's), read_cmd_fd 12 (the terminal's), scriptin 8, redir_fd 6 and vim_fsync 3 (the FILE* phase's)")
	r.Cont("table: 98 rows untouched, names() reads 98, static_assert in place, and nv_Zet still runs `q!` for ZZ and for ZQ")

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
		return harness.ErrReported
	}
	for _, keep := range []string{"fclose", "getc", "putc", "fsync", "read", "write", "close", "dup"} {
		if !check.Contains(after, keep) {
			return stop("%s went, and it is not this phase's: fclose, getc, putc and fsync are the FILE* phase's and read, write, close and dup are the terminal's", keep)
		}
	}
	for _, absent := range []string{"open", "access", "fcntl", "stat", "getcwd", "strerror",
		"chmod", "fchmod", "fstat", "lstat", "unlink"} {
		if check.Contains(after, absent) {
			return stop("%s is undefined again, and nothing here may add one", absent)
		}
	}
	r.Say("symbols %s -> %s, the same set as a cmp -- sixteen functions go and not one was libc's last caller; fclose, getc, putc and fsync are still there and are the FILE* phase's",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")),
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 4. the enumerators --------------------------------------------------
	evOld, evNew := filepath.Join(tmp, "ev.old"), filepath.Join(tmp, "ev.new")
	var ewg sync.WaitGroup
	ewg.Add(1)
	go func() {
		defer ewg.Done()
		exec.Command("sh", "tools/enumvals.sh", filepath.Join(state, "old.c"), evOld).Run()
	}()
	exec.Command("sh", "tools/enumvals.sh", f, evNew).Run()
	ewg.Wait()
	if err := z11Enums(r, check.ReadFile(evOld), check.ReadFile(evNew)); err != nil {
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

	if err := z11Probes(r, old, bin); err != nil {
		return err
	}
	return z11Pty(r, old, bin)
}

// z11WriteOnly is the scan that must report EXACTLY vim_ignored on both files:
// a static with writes and no reads.  It is here rather than in util.go because
// only this phase asks it, and what it is for is that the phase leaves none.
func z11WriteOnly(text string) []string {
	var Out []string
	for _, m := range z11Decl.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		reads, writes := 0, 0
		for _, x := range regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllStringIndex(text, -1) {
			if x[0] >= m[0] && x[0] < m[1] {
				continue
			}
			end := x[1]
			if z11Assign.MatchString(text[end:check.Min(end+4, len(text))]) {
				writes++
			} else {
				reads++
			}
		}
		if writes > 0 && reads == 0 {
			Out = append(Out, name)
		}
	}
	sort.Strings(Out)
	return Out
}

func orNothing(s []string) string {
	if len(s) == 0 {
		return "nothing"
	}
	return strings.Join(s, " ")
}

func tailN(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func z11Enums(r *check.Rep, oldTxt, newTxt string) error {
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
	var gone, came, moved []string
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
	if strings.Join(gone, " ") != strings.Join(z11EnumWant, " ") || len(came) > 0 || len(moved) > 0 {
		if strings.Join(gone, " ") != strings.Join(z11EnumWant, " ") {
			r.Say("the enumerators that went are %s, expected exactly %s", strings.Join(gone, " "), strings.Join(z11EnumWant, " "))
		}
		if len(came) > 0 {
			r.Say("enumerators arrived: %s", strings.Join(came, " "))
		}
		if len(moved) > 0 {
			r.Say("survivors renumbered, and no phase that takes whole anonymous definitions may: %s", strings.Join(moved, " "))
		}
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d: the four CCGD_, the two DOBUF_, SHM_FILEINFO and the five WEE_ go as whole anonymous definitions, NOT ONE SURVIVOR RENUMBERED and none arrived -- the opposite of phase 93, where 85 moved",
		len(o), len(n))
	return nil
}
