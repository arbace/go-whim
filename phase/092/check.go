package p092

// Whim phase 92, the check -- nothing in the editor reads a byte any more.
// See phase/092/edit.go, and GOALS.md.
//
// Runs after phase/092/edit.go and the sweep tools/phaserun.sh runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from: the left-hand side of every
// before-and-after count, and the thing this check builds twice more.
//
// THE HONEST PROBLEM, AND WHAT IS DONE ABOUT IT.  There is no behavioural probe for
// this phase, and no dishonest one is offered instead.  `readfile()` was ALREADY
// unreachable when the phase was handed the tree -- phases 88 through 91 took the
// file argument, the bare `-`, and every command that could name a file -- so
// nothing this editor can be given reached it before the cut either, and every
// recording is byte-identical across the phase BY CONSTRUCTION.  A probe that
// "moved" would mean the phase was wrong.
//
// So the evidence is an INSTRUMENTED PAIR, built from the source the phase was
// handed, and it is the whole of what this phase can prove:
//
// probe   old.c with `(void)write(2, "READFILE-ENTERED\n", 17);` as readfile()'s
// first statement.  Recorded with tools/zrecord.sh: ZERO of the 106
// records a recording held WHEN THIS PHASE WAS WRITTEN -- it is 122 since
// phase 123 added the memline corpus, and the assertions below are
// written against the count the run measures, not against that number --
// records may carry the marker.  That is the claim -- on the binary this
// phase was handed, nothing the instrument can do enters readfile().
// ctl     old.c with the IDENTICAL instrument in open_buffer(), which IS reached.
// 104 of the same 106 records carry it.  That is the proof the probe can
// fail: a marker written from a function the editor calls does arrive, in
// the same recording, through the same grep.
//
// The two that do not carry it under `ctl` are ref-pty.txt and ref-term.txt, and
// the reason is the instrument and not the editor: both drive a real pty and keep
// what was DRAWN, where the other three keep stderr separately.  They are named
// here so that a third one going quiet would be a failure rather than a shrug.
//
// EIGHT ADVERSARIAL SESSIONS run on both instrumented binaries, and they are the
// part that asks whether anything could still get in: `:file /etc/hostname` and
// then `G`, an insert and an undo, `:bdelete`, `:new`, `:ball`, `:buffer 1`, and
// the `%` and `#` registers.  Naming a buffer after a real file that exists and
// then making the editor want its contents is the shape of every way back into
// `readfile()` there was.  Each must mark under `ctl` and must not under `probe`:
// a session that reaches neither proves nothing, and that is checked.
//
// AND THE RECORDINGS ARE COMPARED DIRECTLY, old binary against new, rather than
// only through .reference/core-baselines: `diff -rq` over two full tools/zrecord.sh
// recordings, which is what "this phase declares nothing at all" means measured
// between the two binaries themselves.  tools/coredelta.sh runs afterwards and says
// the same thing against whim-vim's frozen behaviour.
//
// SIX THINGS THE SOURCE MUST SAY, and the traps that make the obvious check wrong,
// are in sections 1 and 2.

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

func init() { check.Register("whim92", Check) }

var z9Gone = []string{"readfile", "read_buffer", "read_eintr", "readfile_linenr", "filemess",
	"msg_add_fname", "msg_add_lines", "msg_add_eol", "after_pathsep",
	"dir_of_file_exists", "fix_help_buffer", "gettail_sep", "mch_isdir",
	"set_rw_fname", "u_find_first_changed", "utf_ptr2len_len",
	"read_stdin", "read_fifo", "check_readonly",
	"READ_NEW", "READ_STDIN", "READ_BUFFER", "READ_FIFO", "READ_FILTER", "READ_NOFILE",
	"READ_KEEP_UNDO", "READ_DUMMY"}

// z9GoneStrings are the twenty-four literals the message layer was the last to
// say: filemess(), msg_add_fname(), msg_add_lines() and msg_add_eol() are the
// whole of what printed `"keys" [noeol] 1L, 30B` after a read.
var z9GoneStrings = []string{
	`"%s%ldL, %lldB"`, `"%s%ld line, "`, `"%s%ld lines, "`, `"%lld byte"`,
	`"%lld bytes"`, `"[noeol]"`, `"[READ ERRORS]"`, `"[New DIRECTORY]"`,
	`"[Incomplete last line]"`, `"[long lines split]"`, `"[ILLEGAL BYTE in line %ld]"`,
	`"[File too big]"`, `"[Permission Denied]"`, `"[fifo]"`, `"[socket]"`,
	`"is a directory"`, `"is not a file"`, `"Illegal file name"`, `"-stdin-"`,
	`"Vim: Reading from stdin...\n"`, `"\" "`,
	`"E200: *ReadPre autocommands made the file unreadable"`,
	`"E201: *ReadPre autocommands must not change current buffer"`,
	`"E812: Autocommands changed buffer or buffer name"`,
}

var z9Kept = map[string]int{
	"open_buffer": 5, "read_cmd_fd": 12, "b_ffname": 32, "b_fname": 29, "b_sfname": 26,
	"setfname": 2, "check_fname": 3, "readonlymode": 3, "msg_scrolled_ign": 2,
	"b_mtime_read": 3, "b_mtime_read_ns": 3, "b_orig_size": 3, "b_orig_mode": 3,
	"buf_store_time": 3, "set_b0_fname": 4, "ml_open": 3, "p_ur": 2,
	"check_changed": 4, "nv_error": 46, "secure": 11,
	"eval_vars": 4, "expand_filename": 3, "fix_fname": 3,
	"vim_FullName": 3, "mch_FullName": 3, "mch_dirname": 5,
}

var z9EnumWant = []string{"BF_NEW_W", "CONV_RESTLEN", "CPO_FNAMER", "NOTDONE", "O_EXTRA",
	"READ_BUFFER", "READ_DUMMY", "READ_FIFO", "READ_FILTER", "READ_KEEP_UNDO",
	"READ_NEW", "READ_NOFILE", "READ_STDIN", "SHM_LAST", "SHM_LINES", "SHM_OVER", "SHM_OVERALL"}

const z9Mark = "READFILE-ENTERED"

// Whim92 is phase 92's check: the machinery under every way of naming a file.
//
// THIS IS THE ONE PART II PHASE NO RECORDING CAN SEE, and it says so.  readfile()
// was already unreachable when the phase ran -- phases 88 to 91 took every way to
// name a file -- so the declared delta is nothing at all and two full
// recordings are byte-identical.  The evidence is an INSTRUMENTED PAIR: the
// input source built twice, with a write(2, ...) first in readfile() and then
// in open_buffer(), the identical instrument.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim92 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "nobyte", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim92")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	inst := filepath.Join(tmp, "i")
	os.MkdirAll(inst, 0o755)

	// The two instrumented builds start FIRST and are waited for in section 6:
	// three and a half seconds each of wall time the source checks below can be
	// spending instead.  The flags are the boundary's, as everywhere.
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflags := strings.Fields(check.Z9Flag(mk, "CFLAGS"))
	ldflags := strings.Fields(check.Z9Flag(mk, "LDFLAGS"))
	oldC := check.ReadFile(filepath.Join(state, "old.c"))
	for _, p := range []struct{ fn, Name string }{{"readfile", "probe.c"}, {"open_buffer", "ctl.c"}} {
		var heads []string
		for _, l := range strings.Split(oldC, "\n") {
			if strings.HasPrefix(l, p.fn+"(") {
				heads = append(heads, l)
			}
		}
		if len(heads) != 1 {
			return stop("%s is not one definition head in the input source", p.fn)
		}
		head := heads[0] + "\n{\n"
		if strings.Count(oldC, head) != 1 {
			return stop("%s does not open exactly once in the input source", p.fn)
		}
		os.WriteFile(filepath.Join(inst, p.Name),
			[]byte(strings.Replace(oldC, head, head+"    (void)write(2, \""+z9Mark+"\\n\", 17);\n", 1)), 0o644)
	}
	var bwg sync.WaitGroup
	buildErr := map[string]error{}
	var bmu sync.Mutex
	for _, n := range []string{"probe", "ctl"} {
		bwg.Add(1)
		go func(n string) {
			defer bwg.Done()
			a := append(append([]string{}, cflags...), ldflags...)
			a = append(a, "-o", n, n+".c")
			c := exec.Command("gcc", a...)
			c.Dir = inst
			e := c.Run()
			bmu.Lock()
			buildErr[n] = e
			bmu.Unlock()
		}(n)
	}

	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT := string(src)

	// --- 1. what the sweep took ----------------------------------------------
	for _, g := range z9Gone {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	r.Say("sixteen functions, read_stdin, read_fifo, check_readonly and the eight READ_ enumerators at 0 mentions -- all of it the sweep's, the edit named none of them")

	// --- 2. the source, in both directions -----------------------------------
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	var still, missing []string
	for _, s := range z9GoneStrings {
		if strings.Contains(newT, s) {
			still = append(still, s)
		}
		if !strings.Contains(oldC, s) {
			missing = append(missing, s)
		}
	}
	if len(still) > 0 {
		fail = append(fail, fmt.Sprintf("%d of the 24 strings the message layer was the last to say are still here: %s",
			len(still), strings.Join(head4(still), " ")))
	}
	if len(missing) > 0 {
		fail = append(fail, fmt.Sprintf("the input did not say %s, so this phase is being checked against a file it was not written for",
			strings.Join(head4(missing), " ")))
	}
	// THE OTHER DIRECTION, where a copied "no mention anywhere" loop fails.
	for _, lw := range []struct {
		lit  string
		want int
	}{{`"[RO]"`, 2}, {`"[readonly]"`, 1}, {`"%ld line --%d%%--"`, 1}} {
		if k := strings.Count(newT, lw.lit); k != lw.want {
			fail = append(fail, fmt.Sprintf("%s occurs %d times, expected %d -- readfile was one speaker of the first two and none of the third",
				lw.lit, k, lw.want))
		}
	}
	names := make([]string, 0, len(z9Kept))
	for n := range z9Kept {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		want := z9Kept[name]
		if k := count(newT, name); k != want {
			why := "something survived that should not have"
			if k < want {
				why = "this phase reached too far"
			}
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d -- %s", name, k, want, why))
		}
	}
	// msg_scrolled_ign IS A LEFTOVER AND IS NAMED AS ONE: four writers, all
	// inside filemess() and readfile(), and one reader.  After this phase it is
	// FALSE for ever with the reader still testing it, and NOTHING SEES THAT.
	writes := regexp.MustCompile(`\bmsg_scrolled_ign\b\s*=`).FindAllString(newT, -1)
	if len(writes) != 1 || !strings.Contains(newT, "static int msg_scrolled_ign = FALSE;") {
		fail = append(fail, fmt.Sprintf("msg_scrolled_ign is assigned %d times: after this phase the only one left must be its initialiser, FALSE", len(writes)))
	}
	if a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), "msg_puts_attr_len"); !ok || !strings.Contains(newT[a:z], "msg_scrolled_ign") {
		fail = append(fail, "msg_puts_attr_len no longer reads msg_scrolled_ign, and this phase removed its writers and not its reader")
	}
	// THE FOUR FIELDS THAT BECOME WRITE-ONLY, and why no tool here can take
	// them: deadfields.py removes a field nothing NAMES, and these are named.
	for _, fld := range []string{"b_mtime_read", "b_mtime_read_ns", "b_orig_size", "b_orig_mode"} {
		var reads int
		for _, m := range regexp.MustCompile(`\b`+regexp.QuoteMeta(fld)+`\b`).FindAllStringIndex(newT, -1) {
			tail := newT[m[1]:check.Min(m[1]+3, len(newT))]
			if !regexp.MustCompile(`^\s*=[^=]`).MatchString(tail) {
				reads++
			}
		}
		if !regexp.MustCompile(`(?m)^    \w+[ \t]+` + regexp.QuoteMeta(fld) + `;$`).MatchString(newT) {
			fail = append(fail, fmt.Sprintf("%s is no longer a field of buf_T, and this phase leaves it write-only rather than removing it", fld))
		}
		if reads != 1 {
			fail = append(fail, fmt.Sprintf("%s is read %d times outside an assignment; after this phase every mention but the declaration must be a write", fld, reads-1))
		}
	}
	if strings.Count(newT, "E32: No file name") != 1 {
		fail = append(fail, "E32: No file name went, and it is reachable through check_fname()")
	}
	if !strings.Contains(newT, "E37: No write since last change") {
		fail = append(fail, "E37 went, and check_changed() is the :q phase's")
	}
	for _, kept := range []string{"open_buffer", "ml_open", "buflist_new", "do_one_cmd", "nv_g_cmd",
		"msg_puts_attr_len", "fileinfo", "get_spec_reg"} {
		if count(newT, kept) == 0 {
			fail = append(fail, fmt.Sprintf("%s went, and it is a later phase's or this phase only cut inside it", kept))
		}
	}
	rows := check.Z6RowRe.FindAllString(newT, -1)
	got, err9 := harness.CommandNamesIn(src, "whim-vim.c")
	// THE FLOOR, ASSERTED BY USING IT rather than by grepping for the number.
	if err9 != nil {
		fail = append(fail, fmt.Sprintf("the checked parser refuses this table -- the row floor is no longer below 99: %s", err9))
	}
	if len(rows) != 99 || len(got) != 99 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 99, unchanged: this phase removes no command", len(rows), len(got)))
	}
	if len(check.Z6RowRe.FindAllString(oldC, -1)) != 99 {
		fail = append(fail, "the input does not have 99 rows, so this is not the file this phase was written against")
	}
	if !strings.Contains(newT, "static_assert(sizeof(cmdnames) / sizeof(cmdnames[0]) == CMD_SIZE") {
		fail = append(fail, "the static_assert on the row count went, and it is what catches an enumerator removed without its row")
	}
	for _, name := range []string{"file", "quit", "print", "append", "registers"} {
		if !check.Contains(got, name) {
			fail = append(fail, fmt.Sprintf(":%s went, and no command is this phase's", name))
		}
	}
	for _, p := range []struct {
		Name string
		want int
	}{{"readfile", 5}, {"read_buffer", 17}, {"open_buffer", 5}, {"read_stdin", 23},
		{"check_readonly", 4}, {"readonlymode", 5}} {
		if count(oldC, p.Name) != p.want {
			fail = append(fail, fmt.Sprintf("the input is not the file this phase was written against: %s %d, expected %d",
				p.Name, count(oldC, p.Name), p.want))
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont(`a "no mention anywhere" check fails on a correct phase here:`)
		r.Cont("check_readonly was readfile's LOCAL and reaches 0 only now,")
		r.Cont(`readonlymode goes 5 -> 3 where phase 91 asserted 5, "[RO]" and`)
		r.Cont(`"[readonly]" each keep a speaker, and read_cmd_fd is the`)
		r.Cont("terminal's and does not move at all.")
		return harness.ErrReported
	}
	r.Say(`the 24 strings gone and "[RO]" 3 -> 2, "[readonly]" 2 -> 1 and CTRL-G's counter kept: readfile was one speaker of the first two and none of the third`)
	r.Cont("kept: open_buffer 5, read_cmd_fd 12 on 11 lines (the terminal's), b_ffname 32 and b_fname 29 (phase 93's), setfname 2 (set_rw_fname was its second caller), E32 and E37 with their speakers")
	r.Cont("left and named rather than folded: msg_scrolled_ign at 2 mentions, FALSE for ever with one reader in msg_puts_attr_len, and b_mtime_read, b_mtime_read_ns, b_orig_size and b_orig_mode write-only -- phase 93's")
	r.Cont("table: 99 rows and names() reads 99, unchanged -- this phase removes no command, and the floor keeps its 19 rows of margin")

	// --- 3. the compile, the linkage and the libc surface --------------------
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneU, cameU := check.Comm23(before, after), check.Comm23(after, before)
	if strings.Join(goneU, "\n") != "access\nfcntl\nopen" || len(cameU) > 0 {
		r.Say("the libc surface did not move by exactly access, fcntl and open:")
		r.Cont("  gone: %s ", strings.Join(goneU, " "))
		r.Cont("  came: %s ", strings.Join(cameU, " "))
		return harness.ErrReported
	}
	for _, keep := range []string{"stat", "getcwd", "strerror", "fsync", "read", "close", "dup"} {
		if !check.Contains(after, keep) {
			return stop("%s went, and it is not this phase's: read, close and dup are the terminal's, stat, getcwd and strerror are phase 93's, fsync the FILE* phase's", keep)
		}
	}
	r.Say("symbols %s -> %s, and the set is exactly access fcntl open; read close dup stat getcwd strerror fsync all still undefined",
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
	if err := z9Enums(r, check.ReadFile(evOld), check.ReadFile(evNew)); err != nil {
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
	bwg.Wait()
	if buildErr["probe"] != nil {
		return stop("the readfile() probe did not build")
	}
	if buildErr["ctl"] != nil {
		return stop("the open_buffer() control did not build")
	}
	return z9Evidence(r, tmp, inst, state, f, old, bin)
}

func head4(s []string) []string {
	if len(s) > 4 {
		return s[:4]
	}
	return s
}

// z9Enums: SEVENTEEN GO AND NOTHING RENUMBERS, which is the opposite of phase 91
// and worth the four seconds either side to say.  A whole anonymous definition
// leaving takes no survivor's value with it.
func z9Enums(r *check.Rep, oldTxt, newTxt string) error {
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
	if strings.Join(gone, " ") != strings.Join(z9EnumWant, " ") || len(came) > 0 || len(moved) > 0 {
		if strings.Join(gone, " ") != strings.Join(z9EnumWant, " ") {
			r.Say("enumerators gone: %s", strings.Join(gone, " "))
			r.Cont("expected exactly: %s", strings.Join(z9EnumWant, " "))
		}
		if len(came) > 0 {
			r.Say("enumerators arrived: %s", strings.Join(came, " "))
		}
		if len(moved) > 0 {
			r.Say("enumerators moved value, and none may: %s", strings.Join(moved, " "))
		}
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d: 17 whole anonymous definitions gone, NOT ONE survivor renumbered and none arriving -- no parallel table can have shifted", len(o), len(n))
	return nil
}

var _ = io.Discard
