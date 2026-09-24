package p097

// Whim phase 97, the check -- the strings are the editor's own.
// See phase/097/edit.go, and GOALS.md.
//
// Runs after phase/097/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from.
//
// SIX THINGS ARE PROVED.
//
// 1. THE SOURCE, as counts.  The seventeen bare names at 0, the sixteen musl_* at
// their source counts plus their own definitions, `sprintf` and `f_l` at 0,
// `vim_snprintf` 55 -> 68, and `tolower` STILL AT 2.
//
// THE `tolower` COUNT IS NOT DECORATION.  musl's strcasecmp() and strncasecmp()
// call tolower(); these do not, and inline `(unsigned)c - 'A' < 26 ? c | 32 : c`
// instead, which IS musl's tolower() in the C locale -- `tolower.c` is
// `if (isupper(c)) return c | 32; return c;` and `isupper.c` is
// `(unsigned)c-'A' < 26`.  Inlining costs nothing and keeps this phase and the
// character-class phase independent: that phase counts `tolower` mentions, and
// four new ones here would trip it.
//
// THE SINGLE CALLER IS ASSERTED AGAIN HERE, and that is the point of asserting it
// at all.  `highlight_arg_to_string(..., char_u *buf)` takes a POINTER, so
// sizeof(buf) is 8; its size argument is MAX_ATTR_LEN, which is only its buffer's
// size while `highlight_list_arg` is its ONLY caller and declares `char_u
// buf[MAX_ATTR_LEN]`.  A second caller appearing later, with a smaller buffer,
// would silently invalidate the bound and nothing else in this tree would notice.
// So the count is pinned at 2 -- the definition and the one call -- for ever.
//
// AND THE CHARTER: eighteen lines starting with `#`, every one an `#include <...>`,
// and not one comment or tab added.  This phase writes 301 lines of C into
// whim-vim.c and none of it is a directive and none of it is a comment.
//
// 2. THE LIBC SURFACE, NAMED AS A SET AND NOT AS A COUNT -- `memchr memcmp memcpy
// memmove memset sprintf strcasecmp strcat strchr strcmp strcpy strlen strncasecmp
// strncmp strncpy strpbrk strstr`, seventeen, and NOTHING arriving.  61 -> 44.
//
// THIS IS THE MEASUREMENT THE PHASE TURNS ON.  gcc emits `memcpy` and `memset`
// FOR ITSELF, for aggregate assignments and large zero initialisers, whatever the
// source calls -- so a rename might have left both behind and forced a definition
// under the real name, which is external linkage.  It does not happen here:
// `gcc -S` on the swept text contains not one call to any of the seventeen.  The
// check asserts the `nm -u` absence, so a later phase that adds an aggregate over
// gcc's threshold (measured between 8 KiB and 16 KiB) and assigns it whole fails
// loudly rather than quietly reacquiring a libc symbol.
//
// GOALS.md II.4b's invariant is asserted again beside it, unchanged.
//
// 3. THE ENUMERATORS, which must not move at all: this phase deletes no type, no
// enum and no table row, so all 1,181 must come back with the same values.
//
// 4. THE ONE MUST-DIFFER PROBE, and it is a BUG FIX.  `t_CF` is a user-settable
// option (options[] row "t_CF") that `term_font()` uses as a FORMAT STRING into
// `char buf[20]`.  `sprintf` has no bound.  Measured: `:set t_CF=` + 40 X + `%d`,
// then `:highlight Search ctermfont=3` and a search, kills the binary this phase
// was handed -- exit -11, SIGSEGV -- and exits 0 here with the output truncated to
// nineteen characters.  It is the ONLY reachable input on which this phase changes
// what the editor does, and the check requires BOTH halves: the old one must die
// and the new one must not.
//
// 5. THE FORMATS THAT RENDER DIFFERENTLY, recorded rather than declared.  `t_CF` is
// the one place a USER-SUPPLIED format reaches the formatter, and vim's own printf
// is not musl's: `%f` goes from `[0.000000]` to `[f]`, `%b` from nothing to
// `[1101]`, `%*d` from a garbage int to the argument, `%z` from nothing to `[z]`.
// `%d` and `%1$d` are identical.  `%s` SEGFAULTS ON BOTH BINARIES and is not this
// phase's: t_CF `%s` reads a pointer out of an int argument, and it did that
// before.  Nothing in the instrument sets t_CF, t_CF is empty under every built-in
// terminal but `debug` (where it is "[CF%d]"), and `%d` is the only directive that
// entry uses -- so this is a finding the check records and NOT a declared delta.
//
// 6. THE PROBES THAT MUST NOT DIFFER: thirty-two sessions on both binaries, covering
// every one of the thirteen external sprintf sites and the number formatting the
// nine internal ones did, plus the places where strcasecmp, strncasecmp, memcmp,
// memcpy and strstr are the only reason the screen says what it says.
//
// WHAT NO PROBE COVERS, AND THE PHASE SAYS SO RATHER THAN PRETENDING.  Four of the
// vendored functions are there for code that cannot run, measured by breaking each
// and finding that nothing moves: `musl_strpbrk` (its one site needs P_NFNAME or
// P_NDNAME, and each of those has exactly two mentions in the file -- its own enum
// and that one test -- so no options[] row carries either), `musl_memchr` (its one
// site is vim_vsnprintf_typval's `%.*s`, and the only `%.*s` in the file is the
// OSC-timeout message), `musl_strchr`'s NUL arm (both call sites pass '%') and
// `musl_fmtptr` (nothing formats a pointer).  Their correctness rests on musl's
// source, not on the recording.
//
// The corpus itself is tools/st.sh delta --phase 97, which internal/verify runs
// after this check, and its declaration is NOTHING AT ALL.

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

func init() { check.Register("whim97", Check) }

var w97Seventeen = []string{"memmove", "strlen", "memset", "strncmp", "strcmp", "strcpy", "sprintf",
	"memcpy", "strncasecmp", "strcat", "strcasecmp", "strncpy", "strstr",
	"strchr", "memcmp", "memchr", "strpbrk"}

var w97Input = map[string]int{"memmove": 159, "strlen": 127, "memset": 79, "strncmp": 82, "strcmp": 62,
	"strcpy": 51, "sprintf": 22, "memcpy": 7, "strncasecmp": 13, "strcat": 6,
	"strcasecmp": 6, "strncpy": 4, "strstr": 3, "strchr": 2, "memcmp": 2,
	"memchr": 1, "strpbrk": 2, "vim_snprintf": 55, "tolower": 2}

var w97AfterC = map[string]int{"musl_memmove": 160, "musl_strlen": 133, "musl_memset": 80,
	"musl_strncmp": 83, "musl_strcmp": 63, "musl_strcpy": 53, "musl_memcpy": 8,
	"musl_strncasecmp": 14, "musl_strcat": 7, "musl_strcasecmp": 7,
	"musl_strncpy": 5, "musl_strstr": 4, "musl_strchr": 3, "musl_memcmp": 3,
	"musl_memchr": 2, "musl_strpbrk": 3,
	"musl_fmtnum": 9, "musl_fmtptr": 2, "musl_fmtbase": 5,
	"vim_snprintf": 68, "vim_vsnprintf_typval": 4, "f_l": 0, "tolower": 2,
	"highlight_arg_to_string": 2, "highlight_list_arg": 11, "MAX_ATTR_LEN": 3}

// w97Sized are the twelve sites whose vim_snprintf must carry the size its
// destination really has -- a sprintf becoming a snprintf with the WRONG bound
// compiles, runs and truncates somewhere nobody looks.
var w97Sized = []struct{ What, needle string }{
	{"update_wincolor", `vim_snprintf((char *)str, sizeof("!(:") +  musl_strlen((char *)(opt)) ,`},
	{"show_one_mark", `vim_snprintf((char *)IObuff,  (1024+1) , " %c %6ld %4d ",`},
	{"ex_changes", `vim_snprintf((char *)IObuff,  (1024+1) , "%c %3d %5ld %4d ",`},
	{"do_ascii", `vim_snprintf((char *)IObuff + rlen, (size_t)( (1024+1)  - rlen), "%02x ",`},
	{"get_emsg_source", `vim_snprintf((char *)Buf,  musl_strlen((char *)(sname))  +  musl_strlen((char *)(p)) ,`},
	{"get_emsg_lnum", `vim_snprintf((char *)Buf,  musl_strlen((char *)(p))  + 20,`},
	{"option_value2string", `vim_snprintf((char *)NameBuff, PATH_MAX, "%ld",`},
	{"deadly_signal", `vim_snprintf((char *)IObuff,  (1024+1) , "Vim: Caught deadly signal`},
	{"recording_mode", `vim_snprintf(s, sizeof(s), " @%c", reg_recording);`},
	{"set_color_count", `vim_snprintf((char *)nr_colors, sizeof(nr_colors), "%d", t_colors);`},
	{"term_font", `vim_snprintf(buf, sizeof(buf), (char *) ( term_strings[(int)(KS_CF)] ) , 9 + n);`},
	{"term_color", `vim_snprintf(buf, sizeof(buf), format, lead, tail);`},
}

var w97Keep = []string{"read", "write", "close", "dup", "ioctl", "select", "tcgetattr", "tcsetattr",
	"nanosleep", "isatty", "printf", "fflush", "stderr", "fputs", "fputc", "fwrite", "putchar",
	"__errno_location", "malloc", "free", "realloc", "tolower", "toupper", "towlower", "towupper",
	"qsort", "bsearch"}

var w97Absent = []string{"open", "creat", "openat", "stat", "access", "fcntl", "getcwd", "strerror",
	"fopen", "fdopen", "opendir", "chmod", "fchmod", "fstat", "lstat", "unlink", "ftruncate",
	"fclose", "getc", "putc", "fsync"}

var w97CallRe = regexp.MustCompile(`call[[:space:]]+(memcpy|memset|memmove|strlen|sprintf|strcpy|strcat|strcmp|strncmp|strchr|strstr|memchr|memcmp|strncpy|strcasecmp|strncasecmp|strpbrk)\b`)

// Whim97 is phase 97's check: the libc that is pure computation, defined in
// the file as `static musl_*`.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim97 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "strings", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT, oldT := string(src), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim97")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// --- 1. the source, as counts --------------------------------------------
	// OCCURRENCES: `grep -c` counts LINES and disagrees with eight of the
	// seventeen -- strncasecmp is 13 occurrences on 7 lines.
	var fail []string
	count := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}
	sortedKeys := func(m map[string]int) []string {
		k := make([]string, 0, len(m))
		for x := range m {
			k = append(k, x)
		}
		sort.Strings(k)
		return k
	}
	// The input's counts are READ, not remembered: w97Input is the input these
	// were first written against, and only sprintf's 22 is a fact the phase
	// depends on -- thirteen external sites and nine inside the formatter.
	if k := count(oldT, "sprintf"); k != 22 {
		fail = append(fail, fmt.Sprintf("sprintf has %d mentions in the input, and this phase rewrites exactly 22", k))
	}
	// `(?<!_)\b` in the Python is `\b` alone: `_` is a word character, so a
	// word boundary already refuses a match inside musl_memmove.
	for _, name := range w97Seventeen {
		if k := count(newT, name); k > 0 {
			fail = append(fail, fmt.Sprintf("%s survives as a bare libc name %d times", name, k))
		}
	}
	// What the phase ADDS per name is fixed (w97After less w97Input); the base
	// is the input's own count.
	for _, name := range sortedKeys(w97AfterC) {
		want := w97AfterC[name]
		base := strings.TrimPrefix(name, "musl_")
		if b, ok := w97Input[base]; ok {
			want = count(oldT, base) + (w97AfterC[name] - b)
		}
		if k := count(newT, name); k != want {
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d", name, k, want))
		}
	}
	if !regexp.MustCompile(`(?m)^highlight_arg_to_string\(int .*char_u \*buf\)$`).MatchString(newT) {
		fail = append(fail, "highlight_arg_to_string is not defined with a char_u *buf parameter")
	}
	if strings.Count(newT, "    ts = highlight_arg_to_string(type, iarg, sarg, buf);\n") != 1 {
		fail = append(fail, "highlight_arg_to_string is not called exactly once from highlight_list_arg, and MAX_ATTR_LEN is only its size while that is true -- a second caller with a smaller buffer would make the bound wrong and nothing else here would see it")
	}
	if strings.Count(newT, "    char_u buf[MAX_ATTR_LEN];\n") != 1 {
		fail = append(fail, "highlight_list_arg's `char_u buf[MAX_ATTR_LEN];` is gone, and it is where site 25443's bound comes from")
	}
	if !strings.Contains(newT, `vim_snprintf((char *)buf, MAX_ATTR_LEN, "%d", iarg - 1);`) {
		fail = append(fail, "site 25443 does not use MAX_ATTR_LEN as its bound")
	}
	for _, s := range w97Sized {
		if strings.Count(newT, s.needle) != 1 {
			fail = append(fail, fmt.Sprintf("%s does not call vim_snprintf with the size its destination really has", s.What))
		}
	}
	for _, fn := range []string{"musl_strcasecmp", "musl_strncasecmp"} {
		Body := ""
		if a, z, ok := cutil.FindDefinition(src, cutil.Blank(src), fn); ok {
			Body = newT[a:z]
		}
		if strings.Count(Body, "(unsigned)*l - 'A' < 26 ? *l | 32 : *l") != 2 || strings.Count(Body, "(unsigned)*r - 'A' < 26 ? *r | 32 : *r") != 2 {
			fail = append(fail, fmt.Sprintf("%s does not inline the C-locale tolower -- musl tolower.c is `if (isupper(c)) return c | 32; return c;` and isupper.c is `(unsigned)c-'A' < 26`, and the cast is what keeps a byte over 127 out of the range test", fn))
		}
	}
	var directives []string
	for _, l := range strings.Split(newT, "\n") {
		if strings.HasPrefix(l, "#") {
			directives = append(directives, l)
		}
	}
	allInclude := true
	for _, l := range directives {
		if !strings.HasPrefix(l, "#include <") {
			allInclude = false
		}
	}
	if len(directives) != 18 || !allInclude {
		fail = append(fail, fmt.Sprintf("the file has %d lines starting with #, and GOALS.md says eighteen #includes and nothing else", len(directives)))
	}
	for _, p := range []struct{ tok, Name string }{{"/*", "a block comment"}, {"\t", "a tab"}} {
		if strings.Count(newT, p.tok) != strings.Count(oldT, p.tok) {
			fail = append(fail, fmt.Sprintf("%s count moved %d -> %d, and this phase writes 301 lines of C with neither", p.Name, strings.Count(oldT, p.tok), strings.Count(newT, p.tok)))
		}
	}
	if strings.Count(newT, "//") != strings.Count(oldT, "//") {
		fail = append(fail, fmt.Sprintf("`//` count moved %d -> %d", strings.Count(oldT, "//"), strings.Count(newT, "//")))
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
	for _, flag := range []string{"P_NFNAME", "P_NDNAME"} {
		if count(newT, flag) != 2 {
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected 2 -- its own enum and the one test in musl_strpbrk's only caller.  If an options[] row ever carries it, musl_strpbrk stops being unreachable and this phase's claim about it has to be re-measured", flag, count(newT, flag)))
		}
	}
	if len(regexp.MustCompile(`%\.\*s`).FindAllString(newT, -1)) != 1 {
		fail = append(fail, "there is no longer exactly one `%.*s` in the file, and it was the only thing that could reach musl_memchr")
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("the seventeen bare names at 0 and the sixteen musl_* at their source counts; sprintf and f_l at 0 and vim_snprintf 55 -> 68; tolower STILL AT 2, the case fold being musl's C-locale tolower inlined, so the character-class phase counts what it counted before")
	r.Cont("highlight_arg_to_string has ONE caller and MAX_ATTR_LEN is its bound only while that holds -- pinned at two mentions, the definition and the call")
	r.Cont("eighteen #include lines and no other directive, no comment and no tab added; cmdnames[] 98 rows, options[] 108, both untouched")

	// --- 2. the compile, the linkage and the libc surface --------------------
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.PhaseCheck(w, work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneU, cameU := check.Comm23(before, after), check.Comm23(after, before)
	want := append([]string{}, w97Seventeen...)
	sort.Strings(want)
	if strings.Join(goneU, "\n") != strings.Join(want, "\n") || len(cameU) > 0 {
		r.Say("the libc surface did not move by exactly the seventeen string and memory symbols:")
		r.Cont("  gone: %s ", strings.Join(goneU, " "))
		r.Cont("  came: %s ", strings.Join(cameU, " "))
		return harness.ErrReported
	}
	// gcc may emit a call to one of the seventeen that no source line writes: an
	// aggregate assignment or a large zero initialiser over -O0's threshold.
	asm := filepath.Join(tmp, "z.s")
	if err := exec.Command("gcc", "-S", "-O0", "-fno-stack-protector", "-o", asm, f).Run(); err != nil {
		return fmt.Errorf("gcc -S refused")
	}
	if calls := w97CallRe.FindAllStringSubmatch(check.ReadFile(asm), -1); len(calls) > 0 {
		r.Say("gcc emitted a call to one of the seventeen that no source line writes:")
		counts := map[string]int{}
		var order []string
		for _, c := range calls {
			key := c[0]
			if counts[key] == 0 {
				order = append(order, key)
			}
			counts[key]++
		}
		sort.Strings(order)
		for _, k := range order {
			r.Cont("  %7d %s", counts[k], k)
		}
		r.Cont("That is an aggregate assignment or a large zero initialiser")
		r.Cont("over gcc's -O0 threshold, which is between 8 KiB and 16 KiB.")
		return harness.ErrReported
	}
	for _, keep := range w97Keep {
		if !check.Contains(after, keep) {
			return stop("%s went, and it is not this phase's: this phase is string and memory work and takes nothing else", keep)
		}
	}
	for _, absent := range w97Absent {
		if check.Contains(after, absent) {
			return stop("%s is undefined, and the core has neither a way to open a file nor a stdio stream since phase 96", absent)
		}
	}
	r.Say("symbols %s -> %s, and the set is exactly the seventeen -- with NOT ONE call to any of them left in the assembly, so gcc emits none of them for itself here and nothing had to be defined under a real name",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")),
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 3. the enumerators, which must not move at all ----------------------
	evOld, evNew := filepath.Join(tmp, "ev.old"), filepath.Join(tmp, "ev.new")
	var ewg sync.WaitGroup
	ewg.Add(1)
	go func() {
		defer ewg.Done()
		exec.Command("sh", "tools/enumvals.sh", filepath.Join(state, "old.c"), evOld).Run()
	}()
	exec.Command("sh", "tools/enumvals.sh", f, evNew).Run()
	ewg.Wait()
	if err := w97Enums(r, check.ReadFile(evOld), check.ReadFile(evNew)); err != nil {
		return err
	}

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

	return w97Probes(r, old, bin)
}

func w97Enums(r *check.Rep, oldTxt, newTxt string) error {
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
	if len(gone)+len(came)+len(moved) > 0 {
		for _, p := range []struct {
			What  string
			names []string
		}{{"went", gone}, {"arrived", came}, {"renumbered", moved}} {
			if len(p.names) > 0 {
				r.Say("enumerators %s: %s -- this phase deletes no type, no enum and no table row, so not one may move", p.What, strings.Join(p.names, " "))
			}
		}
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d, not one value moved: this phase adds functions and renames call sites and touches no type, no enum and no table", len(o), len(n))
	return nil
}
