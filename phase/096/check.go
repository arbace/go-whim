package p096

// Whim phase 96, the check -- no `FILE *` that is never opened.
// See phase/096/edit.go, and GOALS.md.
//
// Runs after phase/096/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from.
//
// NOTHING THIS PHASE REMOVES IS REACHABLE, so there is no behavioural must-differ
// probe and no dishonest one is offered instead.  It is phase 92's situation and phase
// 92's answer: the source the phase was handed, built TWICE.
//
// probe  `(void)write(2, "FILESTAR-ENTERED\n", 17);` at FIVE places -- the top of
// `closescript()`, inside `inchar()`'s `getc(scriptin[curscript])` loop,
// inside `redir_write()`'s `redirecting()` block, inside `undo_cmdmod`'s,
// and the top of `vim_fsync()`.  ZERO of the records may carry it -- 106 of
// them when this phase was written, 122 since phase 123 added the
// memline corpus, and the check counts rather than pins.
// ctl    the IDENTICAL instrument at the top of `ui_write()`, which is reached by
// every byte the editor draws.  It must mark almost all of them, and the
// zero above is worth nothing without it.
//
// FIVE THINGS ARE PROVED.
//
// 1. THE COUNTS, WHICH ARE THE REST OF THE ARGUMENT.  `file` at **0 mentions** is the
// cleanest single assertion this phase has: the type is not named in `whim-vim.c`
// at all afterwards.  With it go `scriptin`, `curscript`, `NSCRIPT`,
// `saved_typebuf`, `closescript`, `using_script`, `redir_fd`, `redir_off`,
// `redir_write`, `redirecting`, `vim_fsync` and the two hand-folded locals
// `script_char` and `retesc`.
//
// THE TRAPS, ALL MEASURED:
// * `may_sync_undo()` AND `is_safe_now()` MUST NOT BE DELETED.  Both survive one
// conjunct shorter and still do their real work, and a check that expected
// them at 0 would fail on a correct phase.
// * `fputs` DOES NOT LEAVE, and GOALS.md II row 12 says it does.  After this
// phase the source names it nowhere and `nm -u` still lists it: gcc lowers
// `fprintf(stderr, "...")` to it, exactly as it lowers `printf` to `fputc`,
// `fwrite` and `putchar`.  The freed set is asserted as exactly
// `fclose fsync getc putc`.
// * `fsync` IS THIS PHASE'S, not the buffer-name phase's: its only caller was
// `vim_fsync()`, whose only caller was `ui_write()`'s `console` branch.
// * `retesc` is a LOCAL THAT IS READ AND NEVER WRITTEN after the loop goes. No
// warning covers it, `deadsweep.py` does not act on it, and leaving it would
// mean `inchar()` returns an uninitialised value on a path the compiler thinks
// exists.  `did_return` is the same shape.  Both were folded by hand.
// * `NSCRIPT` is the one enumerator that leaves, and NOTHING RENUMBERS.
//
// 2. THE LIBC SURFACE, NAMED AS A SET AND NOT AS A COUNT -- `fclose getc putc fsync`
// and nothing else -- and GOALS.md II.4b's invariant asserted in its strongest
// form: `open creat openat stat access fcntl getcwd strerror fopen fdopen opendir`
// absent from BOTH the source and the undefined set.  After this phase the core
// has no `open`, no `stat`, no stdio stream and no fourth descriptor: it can only
// read, write, close and dup fds 0, 1 and 2.
//
// 3. THE INSTRUMENTED PAIR, above.
//
// 4. EIGHTEEN ADVERSARIAL SESSIONS on both instrumented binaries, because "nothing
// reaches it" is a claim about every input and not about the corpus: `:messages`,
// `:verbose set ai?`, `:silent echo`, `:history`, `:registers`, `:display`, `ga`,
// an unknown command, `:set all`, `:marks`, `:undolist`, `:changes`, `:map`,
// `:highlight`, `:normal ihi`, `:g/a/p`, a recorded-and-replayed register and
// `:set verbose=9` -- every one of them a way of making the editor PRINT, which is
// where `redir_write()` sat.  Each must reach `ui_write()` and none may reach any
// of the five.
//
// 5. AND THE ORDINARY SESSIONS, byte-identical either side, each required to be doing
// something.  The corpus itself is `tools/st.sh delta --phase 96`, which

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

func init() { check.Register("whim96", Check) }

const w96Mark = "FILESTAR-ENTERED"

var w96Gone = []string{"scriptin", "curscript", "NSCRIPT", "saved_typebuf", "closescript", "using_script",
	"redir_fd", "redir_off", "redir_write", "redirecting", "vim_fsync", "script_char",
	"retesc", "did_return", "FILE"}

// w96Sites are the five places the probe marks, each asserted to occur exactly
// once in the input so the instrument lands where the argument says it does.
var w96Sites = []struct{ Text, What string }{
	{"closescript(void)\n{\n", "the top of closescript()"},
	{"    while (scriptin[curscript] != NULL && script_char < 0)\n    {\n", "inchar()'s script loop, which is getc()'s only caller"},
	{"    if (redirecting())\n    {\n", "redir_write()'s redirecting() block"},
	{"        if (redirecting())\n        {\n", "undo_cmdmod's redirecting() block"},
	{"vim_fsync(int fd)\n{\n", "the top of vim_fsync()"},
}

var w96Kept = map[string]int{
	"may_sync_undo": 3, "is_safe_now": 3, "free_typebuf": 4, "ui_write": 3, "mch_write": 2,
	"read_cmd_fd": 12, "p_paste": 12, "u_sync": 8,
}

var w96Absent = []string{"open", "creat", "openat", "fopen", "fdopen", "opendir", "stat",
	"access", "fcntl", "getcwd", "strerror", "fclose", "getc", "putc",
	"fsync", "mkdir", "rename", "unlink", "readlink"}

// Whim96 is phase 96's check: no FILE * that is never opened.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim96 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "nofile", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim96")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	inst := filepath.Join(tmp, "i")
	os.MkdirAll(inst, 0o755)

	// The two instrumented builds start first, as in whim92.
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflags, ldflags := strings.Fields(check.W92Flag(mk, "CFLAGS")), strings.Fields(check.W92Flag(mk, "LDFLAGS"))
	oldC := check.ReadFile(filepath.Join(state, "old.c"))
	mark := "    (void)write(2, \"" + w96Mark + "\\n\", 17);\n"
	probe := oldC
	for _, s := range w96Sites {
		if n := strings.Count(probe, s.Text); n != 1 {
			return stop("%s occurs %d times in the input source, expected 1", s.What, n)
		}
		probe = strings.Replace(probe, s.Text, s.Text+mark, 1)
	}
	os.WriteFile(filepath.Join(inst, "probe.c"), []byte(probe), 0o644)
	const ctlHead = "ui_write(char_u *s, int len, int console __attribute__((unused)))\n{\n"
	if strings.Count(oldC, ctlHead) != 1 {
		return stop("ui_write does not open exactly once in the input source")
	}
	os.WriteFile(filepath.Join(inst, "ctl.c"), []byte(strings.Replace(oldC, ctlHead, ctlHead+mark, 1)), 0o644)
	var bwg sync.WaitGroup
	buildErr := map[string]error{}
	var bmu sync.Mutex
	for _, n := range []string{"probe", "ctl"} {
		bwg.Add(1)
		go func(n string) {
			defer bwg.Done()
			a := append(append(append([]string{}, cflags...), ldflags...), "-o", n, n+".c")
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

	// --- 1. what went --------------------------------------------------------
	for _, g := range w96Gone {
		if n := check.CountWord(src, g); n != 0 {
			return stop("'%s' still has %d mentions", g, n)
		}
	}
	r.Say("FILE at 0 mentions -- the type is not named in whim-vim.c at all now -- with scriptin, curscript, NSCRIPT, saved_typebuf, closescript, using_script, redir_fd, redir_off, redir_write, redirecting, vim_fsync and the two hand-folded locals")

	// --- 2. and everything that must NOT be at zero --------------------------
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	names := make([]string, 0, len(w96Kept))
	for n := range w96Kept {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		want := w96Kept[name]
		if k := count(newT, name); k != want {
			why := "something survived that should not have"
			if k < want {
				why = "this phase reached too far"
			}
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d -- %s", name, k, want, why))
		}
	}
	if !regexp.MustCompile(`(?m)^ui_write\(char_u \*s, int len\)$`).MatchString(newT) {
		fail = append(fail, "ui_write does not have a two-parameter signature: dropping the parameter is what makes the cut honest, because sweep.sh compiles with -Wno-unused-parameter and would never see it")
	}
	if !strings.Contains(newT, "ui_write(out_buf, len);") {
		fail = append(fail, "ui_write's one call site still passes a third argument")
	}
	bodyOf := func(fn string) string {
		if a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), fn); ok {
			return newT[a:z]
		}
		return ""
	}
	if b := bodyOf("ui_write"); strings.Count(b, ";") != 1 || !strings.Contains(b, "mch_write") {
		fail = append(fail, fmt.Sprintf("ui_write is not mch_write() and nothing else now: %s", check.CutilRepr(b)))
	}
	if b := bodyOf("may_sync_undo"); strings.Contains(b, "scriptin") || !strings.Contains(b, "u_sync") || !strings.Contains(b, "arrow_used") {
		fail = append(fail, fmt.Sprintf("may_sync_undo is not one conjunct shorter with u_sync() still in it: %s", check.CutilRepr(b)))
	}
	safe := bodyOf("is_safe_now")
	for _, keep := range []string{"stuff_empty", "typebuf.tb_len", "global_busy"} {
		if !strings.Contains(safe, keep) {
			fail = append(fail, fmt.Sprintf("is_safe_now lost %s, and it keeps everything but the scriptin conjunct", keep))
		}
	}
	for _, g := range []string{"fputs", "fputc", "fwrite", "putchar"} {
		if count(newT, g) > 0 {
			fail = append(fail, fmt.Sprintf("%s is named in the source, and it should not be -- gcc lowers printf and fprintf to it", g))
		}
	}
	// `(?<![\w.>])name\s*\(` -- RE2 has no lookbehind, so the preceding byte is
	// tested directly: a call, not a member access `x.open(`, a `->open(` or a
	// longer identifier ending in the name.
	for _, absent := range w96Absent {
		if calledBare(newT, absent) {
			fail = append(fail, fmt.Sprintf("%s( is called in the source, and after this phase the core has no way to name or open anything", absent))
		}
	}
	rows := check.W89RowRe.FindAllString(newT, -1)
	got, _ := harness.CommandNamesIn(src, "whim-vim.c")
	if len(rows) != 98 || len(got) != 98 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 98", len(rows), len(got)))
	}
	if i := strings.Index(newT, "static struct vimoption options[]"); i >= 0 {
		j := strings.Index(newT[i:], "\n};")
		if len(check.W95RowRe.FindAllString(newT[i:i+j], -1)) != 108 {
			fail = append(fail, "options[] is not the 108 rows phase 95 left")
		}
	}
	for _, p := range []struct {
		Name string
		want int
	}{{"scriptin", 8}, {"redir_fd", 6}, {"redirecting", 4}, {"redir_write", 7}, {"vim_fsync", 3}, {"FILE", 2}, {"free_typebuf", 5}} {
		if count(oldC, p.Name) != p.want {
			fail = append(fail, fmt.Sprintf("the input is not the file this phase was written against: %s %d, expected %d", p.Name, count(oldC, p.Name), p.want))
		}
	}
	if strings.Contains(oldC, "FILESTAR") || strings.Contains(newT, "FILESTAR") {
		fail = append(fail, "the instrument marker is in a source file, and it belongs only to the two builds this check makes in a temp directory")
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont("may_sync_undo and is_safe_now SURVIVE folded, and a check that")
		r.Cont("expected them at 0 fails on a correct phase; fputs STAYS and is")
		r.Cont("gcc's own, named nowhere in the source.")
		return harness.ErrReported
	}
	r.Say("kept: may_sync_undo 3 and is_safe_now 3, both SURVIVING one conjunct shorter and still doing their work, free_typebuf 4 (closescript was its fifth mention), ui_write 3 with a TWO-parameter signature and mch_write() as its whole body")
	r.Cont("fputs, fputc, fwrite and putchar are named nowhere in the source and are gcc's own -- GOALS.md II row 12 gives fputs to this phase and it does not go")
	r.Cont("and nothing that could open or name anything is called: open, creat, openat, fopen, fdopen, opendir, stat, access, fcntl, getcwd, strerror, fclose, getc, putc and fsync are absent from the source")

	// --- 3. the compile, the linkage and the libc surface --------------------
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.PhaseCheck(w, work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneU, cameU := check.Comm23(before, after), check.Comm23(after, before)
	if strings.Join(goneU, "\n") != "fclose\nfsync\ngetc\nputc" || len(cameU) > 0 {
		r.Say("the libc surface did not move by exactly fclose, fsync, getc and putc:")
		r.Cont("  gone: %s ", strings.Join(goneU, " "))
		r.Cont("  came: %s ", strings.Join(cameU, " "))
		return harness.ErrReported
	}
	for _, keep := range []string{"read", "write", "close", "dup", "ioctl", "select", "tcgetattr", "tcsetattr",
		"nanosleep", "isatty", "printf", "fflush", "stderr", "fputs", "fputc", "fwrite", "putchar", "__errno_location"} {
		if !check.Contains(after, keep) {
			return stop("%s went, and it is not this phase's: read, write, close, dup, ioctl, select, tcgetattr, tcsetattr, nanosleep and isatty are the terminal's, printf, fflush and stderr are the message layer's, and fputs, fputc, fwrite, putchar and __errno_location are gcc's own", keep)
		}
	}
	for _, absent := range []string{"open", "creat", "openat", "stat", "access", "fcntl", "getcwd", "strerror",
		"fopen", "fdopen", "opendir", "chmod", "fchmod", "fstat", "lstat", "unlink", "ftruncate", "fclose", "getc", "putc", "fsync"} {
		if check.Contains(after, absent) {
			return stop("%s is undefined, and after this phase the core can neither open a file nor hold a stdio stream", absent)
		}
	}
	r.Say("symbols %s -> %s, and the set is exactly fclose fsync getc putc -- with open, creat, openat, stat, access, fcntl, getcwd, strerror, fopen, fdopen and opendir absent, the core has no open, no stat, no stdio stream and no fourth descriptor: it can read, write, close and dup fds 0, 1 and 2 and nothing else",
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
	if err := w96Enums(r, check.ReadFile(evOld), check.ReadFile(evNew)); err != nil {
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

	bwg.Wait()
	if buildErr["probe"] != nil {
		return stop("the five-site probe did not build")
	}
	if buildErr["ctl"] != nil {
		return stop("the ui_write() control did not build")
	}
	return w96Evidence(r, tmp, inst, old, bin)
}

// calledBare is `(?<![\w.>])name\s*\(`: a call of name that is not a member
// access and not the tail of a longer identifier.
func calledBare(text, name string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\(`)
	for _, m := range re.FindAllStringIndex(text, -1) {
		if m[0] > 0 {
			c := text[m[0]-1]
			if c == '.' || c == '>' || c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				continue
			}
		}
		return true
	}
	return false
}

func w96Enums(r *check.Rep, oldTxt, newTxt string) error {
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
	if strings.Join(gone, " ") != "NSCRIPT" || len(came) > 0 || len(moved) > 0 {
		if strings.Join(gone, " ") != "NSCRIPT" {
			g := strings.Join(gone, " ")
			if g == "" {
				g = "none"
			}
			r.Say("the enumerators that went are %s, expected exactly NSCRIPT", g)
		}
		if len(came) > 0 {
			r.Say("enumerators arrived: %s", strings.Join(came, " "))
		}
		if len(moved) > 0 {
			r.Say("survivors renumbered: %s", strings.Join(moved, " "))
		}
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d: NSCRIPT alone, as a whole anonymous definition, and not one survivor renumbered", len(o), len(n))
	return nil
}
