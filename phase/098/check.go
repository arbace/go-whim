package p098

// Whim phase 98, the check -- the character classes, the numbers and the sort.
// See phase/098/edit.go, and GOALS.md.
//
// Runs after phase/098/edit.go and the sweep internal/verify runs between them,
// and reads nothing from the edit's shell -- only the work tree and the state
// directory.  What the edit left there is `old`, the binary this phase was HANDED,
// and `old.c`, the source it was built from.
//
// THE DECLARED DELTA IS NOTHING AT ALL, AND IT IS A THIRD KIND OF EMPTY DECLARATION.
// Phase 92's was code that could not run; phase 96's was a possibility that had never
// existed; this one is an EQUALITY.  The code this phase replaces runs constantly --
// `towupper` alone is entered 892 times in a trivial session, before 'casemap' has
// even been applied -- and what is claimed is that the replacement computes the same
// answer.  So there is no must-differ probe, every behavioural probe is a
// MUST-NOT-DIFFER, and the weight of the evidence sits on equivalence instead:
//
// muslcase --verify   re-derives all 1,114,112 codepoints from THIS
// MACHINE'S libc through ctypes and compares them
// with the 187 + 171 rows the phase shipped.  The
// table is checked against the only authority there
// is, not remembered.
// muslctype --verify  slices the seventeen functions OUT OF THE SOURCE
// THIS PHASE PRODUCED, compiles them with -Wall
// -Wextra and runs them beside libc's.
//
// Both are proven able to fail: perturbing one `convertStruct` offset, one `& 0x5f`,
// one comparator direction and one `return` in the binary search each makes the
// matching tool refuse.
//
// THE HEADER CONTRACT IS ASSERTED HERE AND NOT LEFT TO PHASE 99.  A copy of the
// produced source with `#include <ctype.h>` and `#include <wctype.h>` deleted must
// compile SILENTLY, and the same deletion on the source this phase was HANDED must
// fail.  That is a check that can fail in both directions, and it is what phase 99
// needs to be true before it can move.  Neither copy is left in the tree: this phase
// changes no directive, and the count stays 18.
//
// FIVE THINGS ARE PROVED.
//
// 1. THE SOURCE, as counts.  Nothing <ctype.h> or <wctype.h> provides is CALLED
// anywhere, `iswupper` is gone, and the seventeen musl_ functions are defined
// once each.  The count is call-shaped and not `\b`-shaped: "isprint" is also an
// option name in a string literal, and a word count says 1 on a file that calls
// it nowhere.
//
// THE TRAPS, ALL MEASURED:
// * `nm -u` UNDER-REPORTS <ctype.h> BY FIVE NAMES.  isalpha, isdigit, isgraph,
// islower and isupper are function-like macros in musl, so a source that
// calls them has no undefined symbol to show for it.  A check written from
// the symbol list alone passes on a phase that left all seventeen sites.
// * `latin1flags`, `latin1upper` and `latin1lower` MUST SURVIVE.  They are read
// only from the unreachable arms this phase deliberately does not touch, and
// a check that expected them to go would fail on a correct phase.
// * `utf_convert` GAINS two callers and keeps its own.
// * THE BINARY GROWS.  805,544 against 803,912 on this phase's input: the 358
// rows are data the image did not carry before, and the musl objects they
// replace were smaller, because musl packs the same mapping into 16,998 bytes
// of two-level base-6 table.  A phase check that assumed removal means
// shrinkage would fail here.
//
// 2. THE LIBC SURFACE, NAMED AS A SET AND NOT AS A COUNT -- `atoi atol bsearch
// isalnum iscntrl ispunct qsort tolower toupper towlower towupper` and nothing
// else -- with the terminal, the memory and the message layer asserted still
// there, and GOALS.md II.4b's invariant asserted again.
//
// 3. THE TWO EQUIVALENCE TOOLS, above.
//
// 4. FOURTEEN PROBE SESSIONS ON BOTH BINARIES, byte-identical, each required to be
// doing something.  The corpus is ASCII-only and seeds itself by typing, so it
// cannot reach Unicode case folding, `'casemap'`, the four bsearch tables or the
// sort at all -- which is exactly why `tools/st.sh delta` saying "nothing moved"
// is not enough on its own.
//
// 5. AND THE SORT, WHICH NO RECORD CAN SEE.  `:undolist` is wiped by the Press-ENTER
// redraw before the `\x1b[?25h` that ends a step, so its row order is read out of
// the raw stdout stream.  It is compared rather than sha'd because the rows carry
// "0 seconds ago", which is the one nondeterminism the corpus scrubs.
// tools/musl-case.txt
// tools/musl-ctype.txt
// phasecheck
// tools/st.sh
// tools/st.sh zrecord

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

func init() { check.Register("whim98", Check) }

var z15ProvidedC = strings.Fields(`isalnum isalpha isblank iscntrl isdigit isgraph islower isprint
ispunct isspace isupper isxdigit isascii toascii tolower toupper
iswalnum iswalpha iswblank iswcntrl iswdigit iswgraph iswlower
iswprint iswpunct iswspace iswupper iswxdigit towlower towupper
towctrans wctrans wctype iswctype`)

var z15Had = strings.Fields("isalnum isalpha isdigit isgraph islower ispunct iscntrl isupper iswupper tolower toupper towlower towupper")

var z15Defs = strings.Fields(`musl_isdigit musl_isalpha musl_isupper musl_islower musl_isgraph
musl_isspace musl_isalnum musl_iscntrl musl_ispunct musl_tolower musl_toupper musl_atoi
musl_atol musl_strtol musl_bsearch musl_qsort musl_towupper musl_towlower`)

var z15Rewritten = strings.Fields("tolower toupper towlower towupper isalnum iscntrl ispunct isalpha isdigit isgraph islower isupper isspace atoi atol strtol qsort bsearch")

// z15Calls is `(?<![\w])name\s*\(`: a call whose name is not the tail of a
// longer identifier.  RE2 has no lookbehind, so the byte before is tested.
func z15Calls(text, name string) int {
	re := regexp.MustCompile(regexp.QuoteMeta(name) + `\s*\(`)
	n := 0
	for _, m := range re.FindAllStringIndex(text, -1) {
		if m[0] > 0 {
			c := text[m[0]-1]
			if c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				continue
			}
		}
		n++
	}
	return n
}

// Whim98 is phase 98's check: the character classes, the two ato*, qsort and
// bsearch, vendored as static musl_* functions.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim98 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "vendor", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	src, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	newT, oldT := string(src), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	tmp, err := os.MkdirTemp("", "whim98")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	words := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).FindAllString(t, -1))
	}

	// --- 1. the source, as counts --------------------------------------------
	var fail []string
	var left []string
	for _, n := range z15ProvidedC {
		if z15Calls(newT, n) > 0 {
			left = append(left, n)
		}
	}
	if len(left) > 0 {
		fail = append(fail, fmt.Sprintf("<ctype.h>/<wctype.h> still has a user: %s", strings.Join(left, " ")))
	}
	if words(newT, "iswupper") > 0 {
		fail = append(fail, "iswupper survives, and it was the dead statement")
	}
	for _, t := range []string{"wint_t", "wctype_t", "wctrans_t"} {
		if words(newT, t) > 0 {
			fail = append(fail, fmt.Sprintf("%s is still named, and it is <wctype.h>'s", t))
		}
	}
	var had []string
	for _, n := range z15ProvidedC {
		if z15Calls(oldT, n) > 0 {
			had = append(had, n)
		}
	}
	wantHad := append([]string{}, z15Had...)
	sort.Strings(had)
	sort.Strings(wantHad)
	if strings.Join(had, " ") != strings.Join(wantHad, " ") {
		fail = append(fail, fmt.Sprintf("the input is not the file this phase was written against: it calls %s", strings.Join(had, " ")))
	}
	if words(oldT, "iswupper") == 0 {
		fail = append(fail, "the input did not name iswupper, so deleting it proves nothing")
	}
	for _, name := range z15Defs {
		if len(regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(name)+`\(`).FindAllString(newT, -1)) != 1 {
			fail = append(fail, fmt.Sprintf("%s is not defined exactly once", name))
		}
	}
	block := check.ReadFile("tools/musl-ctype.txt") + check.ReadFile("tools/musl-case.txt")
	for _, name := range []string{"tolower", "toupper", "towlower", "towupper", "isalnum", "iscntrl",
		"ispunct", "isalpha", "isdigit", "isgraph", "islower", "isupper", "isspace", "atoi", "atol", "strtol", "qsort", "bsearch"} {
		want := z15Calls(oldT, name) + z15Calls(block, "musl_"+name)
		if got := z15Calls(newT, "musl_"+name); got != want {
			fail = append(fail, fmt.Sprintf("musl_%s is called %d times, expected %d -- the %d sites the input called %s at, plus the %d times the vendored text names it",
				name, got, want, z15Calls(oldT, name), name, z15Calls(block, "musl_"+name)))
		}
	}
	moved := 0
	for _, n := range z15Rewritten {
		moved += z15Calls(oldT, n)
	}
	// How many sites there are is the input's (44 in both 9.2.1037's and
	// 9.2.1122's, one atol having become a strtol); the per-name counts above
	// are the partition.
	if moved == 0 {
		fail = append(fail, "the input has no call site to rewrite")
	}
	for _, name := range []string{"toUpper", "toLower"} {
		want := words(oldT, name)
		if words(newT, name) != want || words(newT, "musl_"+name) != want {
			fail = append(fail, fmt.Sprintf("%s is named %d times and musl_%s %d, and both must be %d -- vim KEEPS its own case tables and musl's sit BESIDE them",
				name, words(newT, name), name, words(newT, "musl_"+name), want))
		}
	}
	if z15Calls(newT, "utf_convert") != z15Calls(oldT, "utf_convert")+2 {
		fail = append(fail, fmt.Sprintf("utf_convert is called %d times, expected %d -- its own callers plus the two wrappers this phase adds",
			z15Calls(newT, "utf_convert"), z15Calls(oldT, "utf_convert")+2))
	}
	for _, p := range []struct {
		Name string
		want int
	}{{"latin1flags", 3}, {"latin1upper", 2}, {"latin1lower", 2}, {"sort_strings", 3}, {"sort_compare", 2}} {
		if k := words(newT, p.Name); k != p.want {
			fail = append(fail, fmt.Sprintf("%s has %d mentions, expected %d -- the folded Latin-1 arms and the sort are not this phase's", p.Name, k, p.want))
		}
	}
	var directives []string
	allInc := true
	for _, l := range strings.Split(newT, "\n") {
		if strings.HasPrefix(l, "#") {
			directives = append(directives, l)
			if !strings.HasPrefix(l, "#include <") {
				allInc = false
			}
		}
	}
	if len(directives) != 18 || !allInc {
		fail = append(fail, "the directives are not the 18 #includes they were")
	}
	if !strings.Contains(newT, "#include <ctype.h>") || !strings.Contains(newT, "#include <wctype.h>") {
		fail = append(fail, "a header was removed, and removing them is phase 99's")
	}
	rows := check.Z6RowRe.FindAllString(newT, -1)
	got, _ := harness.CommandNamesIn(src, "whim-vim.c")
	if len(rows) != 98 || len(got) != 98 {
		fail = append(fail, fmt.Sprintf("cmdnames[] has %d rows and names() reads %d; both must be 98", len(rows), len(got)))
	}
	if i := strings.Index(newT, "static struct vimoption options[]"); i >= 0 {
		j := strings.Index(newT[i:], "\n};")
		if len(check.Z12RowRe.FindAllString(newT[i:i+j], -1)) != 108 {
			fail = append(fail, "options[] is not the 108 rows phase 95 left")
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		r.Cont("nm -u under-reports <ctype.h> by FIVE macro names; latin1flags,")
		r.Cont("latin1upper and latin1lower SURVIVE, being read only from the")
		r.Cont("unreachable arms this phase does not touch.")
		return harness.ErrReported
	}
	r.Say("nothing <ctype.h> or <wctype.h> provides is called anywhere and iswupper is gone -- while the input called twelve of them and named iswupper, so the assertion is one that can fail; seventeen musl_ definitions, 44 rewritten call sites; vim keeps toUpper[] and toLower[] and musl's sit BESIDE them, utf_convert at six callers; latin1flags 3, latin1upper 2 and latin1lower 2 SURVIVING, because the seventeen other statements that cannot run are a phase of their own; 18 directives, both headers still there because removing them is phase 99's")

	// --- 2. the two equivalence tools ----------------------------------------
	for _, t := range []string{"muslcase", "muslctype"} {
		if err := check.Run(w, "sh", "tools/st.sh", t, "--verify", f); err != nil {
			return harness.ErrReported
		}
	}

	// --- 3. the header contract, in both directions --------------------------
	strip := func(in, Out string) {
		var keep []string
		for _, l := range strings.Split(check.ReadFile(in), "\n") {
			if l == "#include <ctype.h>" || l == "#include <wctype.h>" {
				continue
			}
			keep = append(keep, l)
		}
		os.WriteFile(Out, []byte(strings.Join(keep, "\n")), 0o644)
	}
	noinc, noincOld := filepath.Join(tmp, "noinc.c"), filepath.Join(tmp, "noinc-old.c")
	strip(f, noinc)
	strip(filepath.Join(state, "old.c"), noincOld)
	gccArgs := []string{"-c", "-O0", "-Wall", "-Wextra", "-Wno-unused-parameter", "-o", "/dev/null"}
	if Out, err := exec.Command("gcc", append(gccArgs, noinc)...).CombinedOutput(); err != nil || len(Out) > 0 {
		r.Say("the produced source does not compile without <ctype.h> and <wctype.h>, so phase 99 could not remove them:")
		lines := strings.Split(strings.TrimRight(string(Out), "\n"), "\n")
		for i, l := range lines {
			if i >= 5 {
				break
			}
			r.Cont("  %s", l)
		}
		return harness.ErrReported
	}
	if exec.Command("gcc", append(gccArgs, noincOld)...).Run() == nil {
		return stop("the INPUT also compiles without the two headers, so this check cannot fail and proves nothing")
	}
	oldErr, _ := exec.Command("gcc", "-c", "-O0", "-o", "/dev/null", noincOld).CombinedOutput()
	r.Say("the produced source compiles SILENTLY with #include <ctype.h> and <wctype.h> deleted, and the source this phase was handed gives %d errors under the same deletion -- that pair, and not a grep, is what says phase 99 can move",
		check.CountLinesWith(oldErr, "error:"))

	// --- 4. the compile, the linkage and the libc surface --------------------
	before := strings.Fields(check.ReadFile(filepath.Join(state, "symbols", "undefined")))
	if err := check.PhaseCheck(w, work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	goneU, cameU := check.Comm23(before, after), check.Comm23(after, before)
	// The eleven, and strtol wherever the input calls it: vim 9.2.1122's
	// getdigits() moved from atol to strtol, which this phase vendors too.
	wantGone := strings.Fields("atoi atol bsearch isalnum iscntrl ispunct qsort tolower toupper towlower towupper")
	if z15Calls(oldT, "strtol") > 0 {
		wantGone = append(wantGone, "strtol")
	}
	sort.Strings(wantGone)
	if strings.Join(goneU, " ") != strings.Join(wantGone, " ") || len(cameU) > 0 {
		r.Say("the libc surface did not move by exactly %s:", strings.Join(wantGone, " "))
		r.Cont("  gone: %s ", strings.Join(goneU, " "))
		r.Cont("  came: %s ", strings.Join(cameU, " "))
		return harness.ErrReported
	}
	for _, keep := range strings.Fields("read write close dup ioctl select tcgetattr tcsetattr nanosleep isatty printf fflush stderr malloc free realloc time gettimeofday sigaction kill raise getpid exit _exit __errno_location") {
		if !check.Contains(after, keep) {
			return stop("%s went, and it is not this phase's: this phase takes only what is a function of its arguments", keep)
		}
	}
	for _, absent := range strings.Fields("open creat openat stat access fcntl getcwd strerror fopen fdopen opendir fclose getc putc fsync") {
		if check.Contains(after, absent) {
			return stop("%s is undefined, and the core has had no way to open a file since phase 96", absent)
		}
	}
	r.Say("symbols %s -> %s, the set is exactly %s -- and FIVE MORE identifiers left the source with no symbol to show for it, isalpha isdigit isgraph islower isupper being macros in musl, which is why phase 99 needs this phase's own count and not nm -u",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")),
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")), strings.Join(wantGone, " "))

	// --- 5. the enumerators --------------------------------------------------
	evOld, evNew := filepath.Join(tmp, "ev.old"), filepath.Join(tmp, "ev.new")
	var ewg sync.WaitGroup
	ewg.Add(1)
	go func() {
		defer ewg.Done()
		exec.Command("sh", "tools/enumvals.sh", filepath.Join(state, "old.c"), evOld).Run()
	}()
	exec.Command("sh", "tools/enumvals.sh", f, evNew).Run()
	ewg.Wait()
	if err := z15Enums(r, check.ReadFile(evOld), check.ReadFile(evNew)); err != nil {
		return err
	}

	// --- 6. the structural tools, none of whose tables this phase touches ----
	for _, t := range []string{"nvidx", "orphanopts"} {
		if err := check.Run(w, "sh", "tools/st.sh", t, f); err != nil {
			return harness.ErrReported
		}
	}

	// --- 7. the binary -------------------------------------------------------
	_ = exec.Command("make", "-C", work, "clean").Run()
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		(&check.Rep{Tag: "build", W: w}).Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin, _ := filepath.Abs(filepath.Join(work, "whim-vim"))
	old, _ := filepath.Abs(filepath.Join(state, "old"))
	hdr, _ := exec.Command("readelf", "-h", bin).Output()
	if !regexp.MustCompile(`Type:.*EXEC`).Match(hdr) {
		return stop("the binary is no longer EXEC")
	}
	if pl, _ := exec.Command("readelf", "-l", bin).Output(); strings.Contains(string(pl), "INTERP") {
		return stop("the binary grew an INTERP")
	}
	if Dyn, _ := exec.Command("readelf", "-d", bin).Output(); strings.Contains(string(Dyn), "Dynamic section") {
		return stop("the binary grew a dynamic section")
	}
	now, _ := os.ReadFile(f)
	(&check.Rep{Tag: "build", W: w}).Say("ok, %s -> %d lines, %d bytes -- and it GROWS, because the 358 convertStruct rows are data the image did not carry and musl packed the same mapping into 16,998 bytes",
		beforeLines, check.CountLines(now), check.SizeOf(bin))

	return z15Probes(r, old, bin)
}

func z15Enums(r *check.Rep, oldTxt, newTxt string) error {
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
			What string
			s    []string
		}{{"went", gone}, {"arrived", came}, {"renumbered", moved}} {
			if len(p.s) > 0 {
				r.Say("enumerators %s: %s", p.What, strings.Join(p.s, " "))
			}
		}
		return harness.ErrReported
	}
	r.Say("enumerators %d -> %d: not one went, arrived or renumbered -- this phase adds functions and two convertStruct tables and touches no enum", len(o), len(n))
	return nil
}
