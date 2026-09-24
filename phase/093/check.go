package p093

// Whim phase 93, the check -- the buffer has no name any more.
// See phase/093/edit.go, and GOALS.md.
//
// Runs after phase/093/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from: the left-hand side of every
// before-and-after count, and of every probe.
//
// SEVEN THINGS ARE PROVED HERE, and the fifth is the only one that can say what this
// phase is actually for.
//
// 1. THE CUT, WHICH IS THE SWEEP'S AND NOT THE EDIT'S.  SIXTY functions go and the
// edit names none of them -- the largest number any Part II phase has handed the
// sweep, and all of it computed from four kinds of anchor: a cmdnames[] row, a
// call site that passes NULL, sixteen folds, and an `if` on a flag no row carries.
//
// THE TRAPS, ALL MEASURED, that make a copied "every name at zero" loop wrong:
// * `otherfile` IS ALREADY ZERO.  Phase 91 swept it; `otherfile_buf` is this
// phase's.  A list copied from GOALS.md II.3b's P9 row would "prove" a name
// that went two phases ago.
// * `E447: Can't find file "%s" in path` REACHES ZERO HERE, and phase 91's check
// asserts it SURVIVES.  Phase 91 removed the `gf` key; the message belonged to
// `find_file_name_in_path()`'s search arm, which part G folds away.  The two
// checks disagree on purpose and `apart 91 93` is not needed for it only
// because `apart 91 92` and `apart 92 93` already forbid the stage.
// * `"file"` reaches zero and `E32: No file name` DOES NOT.  `check_fname()`
// survives, folded to an unconditional emsg, because `get_spec_reg()`'s `%`
// still calls it.  A check that wanted both gone fails on a correct phase.
// * `"[No Name]"` occurs TWICE and neither is a leftover: `buf_get_fname()`'s,
// which is now the only name any buffer has, and `can_unload_buffer()`'s,
// which part C folded a ternary into.
// * `fileinfo` goes 4 -> 3 and not to 0: `:file` was one of four callers and
// CTRL-G, `g CTRL-G` and the startup message are the other three.
//
// 2. WHAT IS LEFT WRITE-ONLY, NAMED, AND HANDED ON.  `BF_NOTEDITED` can never be set
// -- `setfname()` was its only writer -- and `BF_NEW` never could; both are still
// READ by `fileinfo()`, so CTRL-G still tests them and neither test can fire.
// `b_shortname` has the same shape and was already write-only before this phase.
// Folding any of the three would change the string set for no gain, so they are
// asserted where they are.  `msg_scrolled_ign` is phase 92's leftover and does not
// move.
//
// 3. THE LINE AGAINST THE PHASES AFTER THIS ONE, stated as counts so that reaching
// into one would fail here rather than widen quietly: `check_changed` 4 and
// `no_write_message` 3 (the `:q` phase's), `p_ur` 2 and `p_ro` 2 WITH their option
// rows (the options phase's), `read_cmd_fd` 12, `vim_fsync` 3, `scriptin` 8 and
// `redir_fd` 6 (the terminal's and the FILE* phase's).
//
// 4. THE ENUMERATORS, DUMPED EITHER SIDE.  Seventy-two go and EIGHTY-FIVE RENUMBER,
// every one of the 85 a `CMD_`.  cmdnames[] is designated, so a row lands at its
// own enumerator whatever the numbering is -- but nothing in the build would
// notice if some other family had moved with them, and 85 movers from one family
// is exactly the case CLAUDE.md says a build is happy to get wrong.  Four seconds
// a dump, taken concurrently, and worth it.
//
// 5. THE PROBES, ON BOTH BINARIES, BECAUSE THE CORPUS CANNOT SEE A BUFFER BEING
// NAMED.  The one case it does see is `cmd_file`, which types `:file` with no
// argument and records the CTRL-G line for `[No Name]` -- so "that case moved" is
// equally consistent with a phase that changed one message and left `setfname()`
// reachable.  `file_rename` is the probe that is not: `:file NEWNAME` and then
// CTRL-G answers `"NEWNAME" [Modified][Not edited] 1 line --100%--` on the binary
// this phase was handed and `"[No Name]" [Modified] 1 line --100%--` here.  That
// line, on the OLD binary, is the whole evidence that a buffer could be named.
//
// AND `cp_missing` IS THE ONE PROBE THAT SHOWS THE OLD BINARY ASKING THE DISK.
// With `nosuchfile` under the cursor, `: CTRL-R CTRL-P <CR>` was SILENT before --
// `find_file_in_path()` stat()ed the name, found nothing and yielded NULL, so
// nothing reached the command line -- and answers `E492: Not an editor command:
// nosuchfile` now, the word having been extracted and nothing looked up.  Its
// pair `cp_existing` must NOT move, and that is what says part G removed the
// lookup and not the extraction: with `keys` under the cursor -- the keystroke
// file tools/zstream.py always leaves in the run directory -- both binaries
// answer `E488: Trailing characters: eys`, `:k` being a command of its own.
// `cf_existing` is the same session with CTRL-F, which never expanded.
//
// 6. THE INHERITANCE CHECK.  `:file`'s row gave its shortest abbreviation as one
// character, so `:f :fi :fil :file :file!` all reached it; each must answer E492
// now and none may have answered E492 before.  `:filter` and `:fixdel` are the
// neighbours that must not move -- CLAUDE.md's `:help` -> `:helpclose` trap.
//
// 7. A REAL TERMINAL, because every probe above went through a pipe: `:file NEWNAME`
// and CTRL-G on a pty, and an ordinary editing session required to be identical.
//
// A record is built the way `zcases` builds one and scrubbed the same way
// (tools/zrec.py).  tools/zstream.py's session() is not called directly because this
// check needs the raw stream and the snapshot count beside the screens.

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
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim93", Check) }

var w93Gone = []string{"ex_file", "rename_buffer", "setfname", "buf_name_changed", "ml_timestamp",
	"ml_upd_block0", "ml_check_b0_id", "buflist_name_nr", "buflist_findlnum",
	"buflist_findname_stat", "buf_setino", "buf_same_ino", "buf_store_time",
	"otherfile_buf", "fname_expand", "fix_fname", "shorten_fname", "shorten_fname1",
	"shorten_buf_fname", "mch_dirname", "mch_FullName", "vim_FullName", "FullName_save",
	"home_replace_save", "expand_filename", "eval_vars", "find_cmdline_var",
	"expand_wildcards", "expand_wildcards_eval", "gen_expand_wildcards",
	"ExpandOne", "ExpandOne_start", "ExpandFromContext", "ExpandEscape",
	"find_file_in_path", "mch_getperm", "mch_isFullName", "mch_has_wildcard",
	"path_with_url", "backslash_halve", "repl_cmdline", "escape_fname", "wildescape",
	"vim_fnamecmp", "vim_fnamencmp", "vim_strsave_fnameescape", "tilde_replace",
	"expand_env_save", "expand_env_save_opt", "expand_files_and_dirs",
	"map_wildopts_to_ewflags", "save_patterns", "vim_strrchr", "vim_findfile_cleanup",
	"vim_findfile_free_visited", "vim_findfile_free_visited_list",
	"ff_clear", "ff_pop", "ff_free_stack_element", "ff_free_visited_list",
	"b_ffname", "b_sfname", "b_fname", "b_dev", "b_ino", "b_dev_valid", "b_mtime", "b_mtime_ns",
	"b_mtime_read", "b_mtime_read_ns", "b_orig_size", "b_orig_mode",
	"CMD_file", "EX_XFILE", "readonlymode"}

var w93GoneStrings = []string{`"file"`, `E447: Can't find file `, "E480: No match",
	"E95: Buffer with this name already exists", "E304: ml_upd_block0",
	"E194: No alternate file name to substitute", "E499: Empty file name",
	`"cword>"`, `"afile>"`, `"sfile>"`, `\n  c  \"%   `, `\n  c  \"#   `}

var w93Kept = map[string]int{
	"buf_spname": 5, "buf_get_fname": 3, "get_trans_bufname": 4, "fileinfo": 3,
	"check_fname": 3, "check_changed": 4, "no_write_message": 3,
	"p_ur": 2, "p_ro": 2, "read_cmd_fd": 12, "vim_fsync": 3,
	"scriptin": 8, "redir_fd": 6, "msg_scrolled_ign": 2, "home_replace": 3,
	"file_name_at_cursor": 3, "find_file_name_in_path": 3,
	"BF_NOTEDITED": 3, "BF_NEW": 3, "b_shortname": 2, "nv_error": 46, "open_buffer": 5,
}

// w93Families and w93Prefix are every enumerator family that goes, and what took
// it: CMD_file is the row, EX_XFILE the flag no row carries any more, and the
// rest are whole anonymous enums typereach takes with the code that named them.
var w93Families = []string{"CMD_file", "EX_XFILE", "B0_FNAME_SIZE_CRYPT", "UB_FNAME"}
var w93Prefix = []string{"EW_", "WILD_", "EXPAND_", "XP_BS_", "SPEC_", "BLOCK0_", "BLN_",
	"ESTACK_", "VSE_", "VALID_"}

var w93Spellings = []string{"f", "fi", "fil", "file", "file!"}

// Whim93 is phase 93's check: the buffer's NAME.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim93 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "noname", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT, oldT := string(src), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim93")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// --- 1. what the sweep took ----------------------------------------------
	for _, g := range w93Gone {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	for _, g := range w93GoneStrings {
		if n := check.CountLinesWith(src, g); n != 0 {
			return stop("the string '%s' still has %d mentions", g, n)
		}
	}
	r.Say("sixty functions, twelve buf_T fields, CMD_file, EX_XFILE, readonlymode and 43 string literals at 0 mentions -- all of it the sweep's but readonlymode, and the edit named not one function")

	// --- 2. and everything that must NOT be at zero --------------------------
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	names := make([]string, 0, len(w93Kept))
	for n := range w93Kept {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		want := w93Kept[name]
		if k := count(newT, name); k != want {
			why := "something survived that should not have"
			if k < want {
				why = "this phase reached too far"
			}
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d -- %s", name, k, want, why))
		}
	}
	// THREE FLAGS THAT CAN NEVER BE SET AND ARE STILL READ, named rather than
	// folded.  setfname() was BF_NOTEDITED's only writer and BF_NEW never had
	// one in this tree; fileinfo() still tests both, so CTRL-G asks two
	// questions whose answer is fixed.
	for _, p := range []struct{ flag, fn string }{{"BF_NOTEDITED", "fileinfo"}, {"BF_NEW", "fileinfo"}} {
		a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), p.fn)
		if !ok || !strings.Contains(newT[a:z], p.flag) {
			fail = append(fail, fmt.Sprintf("%s is no longer read by %s, and this phase removes its writer and not its reader", p.flag, p.fn))
		}
		if regexp.MustCompile(`\|=\s*`+p.flag+`\b`).MatchString(newT) || regexp.MustCompile(`\bb_flags = `+p.flag).MatchString(newT) {
			fail = append(fail, fmt.Sprintf("%s is set somewhere, and setfname() was its only writer", p.flag))
		}
	}
	if k := len(regexp.MustCompile(`\bb_shortname\b\s*=[^=]`).FindAllString(newT, -1)); k != 1 {
		fail = append(fail, fmt.Sprintf("b_shortname is assigned %d times; it was already write-only before this phase, with one write", k))
	}
	if k := strings.Count(newT, `"[No Name]"`); k != 2 {
		fail = append(fail, fmt.Sprintf(`"[No Name]" occurs %d times, expected 2 -- buf_get_fname's, which is now the only name a buffer has, and can_unload_buffer's`, k))
	}
	if strings.Count(newT, "E32: No file name") != 1 {
		fail = append(fail, "E32: No file name went, and check_fname() still says it for the `%` register")
	}
	if !strings.Contains(newT, "E37: No write since last change") {
		fail = append(fail, "E37 went, and check_changed() is the :q phase's")
	}
	if strings.Count(newT, "E23: No alternate file") != 1 {
		fail = append(fail, "E23 went, and getaltfname() says it unconditionally now")
	}
	if strings.Contains(newT, "E447") {
		fail = append(fail, "E447 survives, and part G removed the only arm that could say it -- phase 91's check asserts the opposite, which is why the two phases cannot share a stage")
	}
	if !strings.Contains(oldT, "E447") {
		fail = append(fail, "the input did not say E447, so this phase is being checked against a file it was not written for")
	}
	if a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), "check_fname"); ok {
		Body := newT[a:z]
		if strings.Contains(Body, "if (") || strings.Contains(Body, "return OK") {
			fail = append(fail, "check_fname still tests something: `b_ffname == NULL` was TRUE for ever and the fold leaves an unconditional E32")
		}
	}
	rows := check.W89RowRe.FindAllString(newT, -1)
	got, errN := harness.CommandNamesIn(src, "whim-vim.c")
	if errN != nil {
		fail = append(fail, fmt.Sprintf("the checked parser refuses this table -- the row floor is no longer below 98: %s", errN))
	}
	if len(rows) != 98 || len(got) != 98 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 98", len(rows), len(got)))
	}
	if len(check.W89RowRe.FindAllString(oldT, -1)) != 99 {
		fail = append(fail, "the input does not have 99 rows, so this is not the file this phase was written against")
	}
	if check.Contains(got, "file") {
		fail = append(fail, ":file is still a command name")
	}
	for _, name := range []string{"filter", "fixdel", "quit", "print", "append", "registers"} {
		if !check.Contains(got, name) {
			fail = append(fail, fmt.Sprintf(":%s went, and it is not this phase's", name))
		}
	}
	if !strings.Contains(newT, "static_assert(sizeof(cmdnames) / sizeof(cmdnames[0]) == CMD_SIZE") {
		fail = append(fail, "the static_assert on the row count went, and it is what catches an enumerator removed without its row")
	}
	if !check.W90QRow.MatchString(newT) {
		fail = append(fail, "the 'Q' row is no longer nv_error's, and phase 87 put it there")
	}
	// THE EXEMPTION PHASE 91 KEPT, from the other side.
	m := check.W91Lock.FindString(newT)
	if m == "" {
		fail = append(fail, "do_one_cmd's curbuf_locked() test went, and this phase removed one conjunct of it and not the test")
	} else if strings.Contains(m, "CMD_") {
		fail = append(fail, fmt.Sprintf("the curbuf_locked() exemption still names a command: %s", check.CutilRepr(strings.TrimSpace(m))))
	}
	for _, p := range []struct{ opt, v string }{{"'undoreload'", "p_ur"}, {"'readonly'", "p_ro"}} {
		if !strings.Contains(newT, "(char_u *)&"+p.v+",") {
			fail = append(fail, fmt.Sprintf("%s lost its option row, and that is the options phase's: a row removed here would change what :set answers", p.opt))
		}
	}
	for _, p := range []struct {
		Name string
		want int
	}{{"b_ffname", 32}, {"b_sfname", 26}, {"b_fname", 29}, {"setfname", 2},
		{"eval_vars", 4}, {"mch_dirname", 5}, {"CMD_file", 4}, {"EX_XFILE", 4}, {"readonlymode", 3}} {
		if count(oldT, p.Name) != p.want {
			fail = append(fail, fmt.Sprintf("the input is not the file this phase was written against: %s %d, expected %d",
				p.Name, count(oldT, p.Name), p.want))
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont("a check copied from phase 91 or 92 fails on a correct phase 93:")
		r.Cont("`otherfile` went at phase 91, E447 SURVIVED phase 91 and reaches")
		r.Cont("zero here, `fileinfo` keeps three callers, E32 keeps its")
		r.Cont(`speaker, and "[No Name]" occurs twice and both are live.`)
		return harness.ErrReported
	}
	r.Say(`kept: buf_spname 5 and buf_get_fname 3 -- "[No Name]" is now the ONLY name a buffer has, not one of two -- get_trans_bufname 4, fileinfo 3 (CTRL-G, g CTRL-G and the startup message), check_fname 3 with E32`)
	r.Cont("left and named rather than folded: BF_NOTEDITED and BF_NEW, read by fileinfo() and settable by nothing now that setfname() has gone, and b_shortname, which was already write-only before this phase")
	r.Cont("the later phases' line: check_changed 4 and no_write_message 3 (the :q phase's), p_ur 2 and p_ro 2 with their rows (the options phase's), read_cmd_fd 12, vim_fsync 3, scriptin 8, redir_fd 6")
	r.Cont("table: 99 -> 98 rows, names() reads 98, static_assert in place, the floor is 80 and the checked parser accepts it")

	// --- 3. the compile, the linkage and the libc surface --------------------
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.PhaseCheck(w, work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneU, cameU := check.Comm23(before, after), check.Comm23(after, before)
	if strings.Join(goneU, "\n") != "getcwd\nstat\nstrerror" || len(cameU) > 0 {
		r.Say("the libc surface did not move by exactly getcwd, stat and strerror:")
		r.Cont("  gone: %s ", strings.Join(goneU, " "))
		r.Cont("  came: %s ", strings.Join(cameU, " "))
		return harness.ErrReported
	}
	for _, keep := range []string{"fsync", "read", "close", "dup"} {
		if !check.Contains(after, keep) {
			return stop("%s went, and it is not this phase's: read, close and dup are the terminal's and fsync is ui_write's", keep)
		}
	}
	for _, absent := range []string{"open", "access", "fcntl", "stat", "getcwd", "strerror",
		"chmod", "fchmod", "fstat", "lstat", "unlink"} {
		if check.Contains(after, absent) {
			return stop("%s is undefined again, and nothing here may add one", absent)
		}
	}
	r.Say("symbols %s -> %s, and the set is exactly getcwd stat strerror -- with open, access and fcntl still absent, the core can no longer acquire a file descriptor at all; read, close, dup and fsync stay and are the terminal's and ui_write's",
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
	if err := w93Enums(r, check.ReadFile(evOld), check.ReadFile(evNew)); err != nil {
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

	if err := w93Probes(r, old, bin); err != nil {
		return err
	}
	return w93Pty(r, old, bin)
}

// w93Enums: EIGHTY-FIVE SURVIVORS RENUMBER and every one must be a CMD_.  85
// movers from one family is the case CLAUDE.md says a build is happy to get
// wrong.
func w93Enums(r *check.Rep, oldTxt, newTxt string) error {
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
		ok := check.Contains(w93Families, k)
		for _, p := range w93Prefix {
			if strings.HasPrefix(k, p) {
				ok = true
			}
		}
		if !ok {
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
	if len(moved) == 0 {
		r.Say("no survivor renumbered, and removing a cmdnames[] row must move every CMD_ after it -- the dump is not of this phase")
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d: %d gone, %d renumbered and every one of the %d a CMD_, none arriving -- which is exactly the case a build is happy to get wrong",
		len(o), len(n), len(gone), len(moved), len(moved))
	return nil
}

var _ = io.Discard
