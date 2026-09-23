package p106

// Whim phase 106, the check -- `nullptr` and `usize`.
// See phase/106/edit.go, and GOALS.md II.4c.
//
// Runs after phase/106/edit.go and the sweep tools/phaserun.sh runs between them, and
// reads nothing from the edit's shell -- only the work tree and the state directory.
// What the edit left there is `old.c`, the source this phase was HANDED, and `old`, that
// source built with SOURCE_DATE_EPOCH=0 and the boundary's own flags.
//
// THIS PHASE CHANGES NO STATEMENT, so there is no behavioural probe to offer and none is
// offered.  What it has instead is stronger than any recording: THE BINARY IS THE SAME
// BYTES.  That is tier 1 of CLAUDE.md's verification table, and it subsumes every screen
// case, every Ex-command row, every command line and every pty scenario at once, because
// the program that would be run is literally the same program.  tools/coredelta.sh
// --phase 106 still runs, from tools/phaserun.sh after this check, and corroborates; it
// is not the evidence.  It is phase 99's shape exactly, on three thousand edits instead
// of seven.
//
// WHAT IS CLAIMED, in five parts:
//
// ARITHMETIC  the counts are computed FROM THE INPUT and not written here: every
// `NULL` outside a literal became a `nullptr`, every `size_t` became a
// `usize`, the three literals are UNCHANGED, and `usize` gained exactly
// one for its own typedef.  So the check is about this phase and not about
// whatever it was handed.
// LANGUAGE    the typedef is taken OUT OF THE OUTPUT and compiled four ways: gcc's
// default and `-std=c23` must ACCEPT it, `-std=c11` and `-std=c99` must
// REFUSE it.  C23 is a real dependency of this file and is stated rather
// than assumed.  In the same translation unit, with the REAL <stddef.h>
// arriving after it, `_Generic((usize)0, size_t: 1, default: 0)` proves
// `usize` IS `size_t` -- the same type, not merely the same width -- and
// `sizeof(nullptr) == sizeof(void *)` proves the other half of why the
// thirty casts could go.
// SYMBOLS     `nm -u` is THE SAME SET, as a `comm` empty in BOTH directions, and `main`
// is still the only external symbol.  A rename inside one translation unit
// cannot move either, and a symbol ARRIVING must fail as loudly as one
// leaving.
// THE BINARY  `cmp` of the input's binary and the output's, both built with
// SOURCE_DATE_EPOCH=0 and the boundary's own flags.  This is the whole
// evidence.
// THE CONTROL and it is the point.  The same output with the literal exclusion
// REMOVED -- a plain `\bNULL\b` -> `nullptr` over the whole text, which
// rewrites the three strings -- MUST give a different binary.  Measured:
// 1,598 bytes differ, 1,354 of them in `.rodata`, and `strings` reports
// `[nullptr]`, `nullptr` and an E1507 message that names a C keyword at
// the user.  Without that control the `cmp` above is a pair of numbers
// agreeing, and CLAUDE.md is explicit that a test that cannot fail is not
// evidence.
//
// AND ONE CONTROL THAT MOVES NOTHING, REPORTED RATHER THAN DROPPED.  Reverting one
// `usize` to `size_t` compiles cleanly and gives a byte-identical binary, because the
// `#include`s are still at the TOP of the file and `size_t` is therefore still declared
// above every line of it.  That is the honest statement of what this phase's evidence
// cannot reach: the rename is not yet load-bearing, and it becomes so at phase 109, where
// the same control is three hard errors.  It is phase 105's b3/b4 in this phase's shape.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim106", Check) }

// z23Spans is the heredoc's literal_spans(): every string and character
// literal, escapes skipped, refusing one a newline or the end of the text cuts.
func z23Spans(t string) ([][2]int, bool) {
	var Out [][2]int
	i, n := 0, len(t)
	for i < n {
		c := t[i]
		if c == '"' || c == '\'' {
			j := i + 1
			for j < n {
				if t[j] == '\\' {
					j += 2
					continue
				}
				if t[j] == c || t[j] == '\n' {
					break
				}
				j++
			}
			if j >= n || t[j] != c {
				return nil, false
			}
			Out = append(Out, [2]int{i, j + 1})
			i = j + 1
		} else {
			i++
		}
	}
	return Out, true
}

func z23Outside(t string, S [][2]int, name string) int {
	starts := make([]int, len(S))
	for i, s := range S {
		starts[i] = s[0]
	}
	k := 0
	for _, m := range regexp.MustCompile(`\b`+name+`\b`).FindAllStringIndex(t, -1) {
		j := sort.SearchInts(starts, m[0]+1) - 1
		if !(j >= 0 && S[j][0] <= m[0] && m[0] < S[j][1]) {
			k++
		}
	}
	return k
}

// Whim106 is phase 106's check: `nullptr` and `usize`.
func Check(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: check whim106 <work-dir> <state-dir>")
	}
	work, state := args[0], args[1]
	r := &check.Rep{Tag: "language", W: w}
	f := filepath.Join(work, "whim-vim.c")
	beforeLines := strings.TrimSpace(check.ReadFile(filepath.Join(state, "input-lines")))
	tmp, err := os.MkdirTemp("", "whim106")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	mk := check.ReadFile(filepath.Join(work, "Makefile"))
	cflags, ldflags := strings.Fields(check.Z9Flag(mk, "CFLAGS")), strings.Fields(check.Z9Flag(mk, "LDFLAGS"))
	newC, oldC := check.ReadFile(f), check.ReadFile(filepath.Join(state, "old.c"))
	stop := func(format string, a ...any) error { r.Say(format, a...); return harness.ErrReported }
	sde := append(os.Environ(), "SOURCE_DATE_EPOCH=0")
	build := func(Out, src, log string) chan error {
		ch := make(chan error, 1)
		go func() {
			a := append(append(append([]string{}, cflags...), ldflags...), "-o", Out, src)
			c := exec.Command("gcc", a...)
			c.Env = sde
			if log != "" {
				lf, _ := os.Create(log)
				defer lf.Close()
				c.Stderr = lf
			}
			ch <- c.Run()
		}()
		return ch
	}
	newB := build(filepath.Join(tmp, "new"), f, "")

	c1, n1 := newC, 0
	c1 = regexp.MustCompile(`\bNULL\b`).ReplaceAllStringFunc(newC, func(string) string { n1++; return "nullptr" })
	if n1 != 3 {
		return stop("c1 rewrote %d `NULL`, expected the three inside string literals -- there is nothing else left for a literal-unaware sed to find, so this control would not be the control", n1)
	}
	const sig = "musl_memcpy(void *dest, const void *src, usize n)"
	if strings.Count(newC, sig) != 1 {
		return stop("the vendored signature `%s` is not in the output exactly once", sig)
	}
	c2 := strings.Replace(newC, sig, strings.ReplaceAll(sig, "usize", "size_t"), 1)
	for _, p := range [][2]string{{"c1", c1}, {"c2", c2}} {
		if p[1] == newC {
			return stop("%s changed nothing", p[0])
		}
		os.WriteFile(filepath.Join(tmp, p[0]+".c"), []byte(p[1]), 0o644)
	}
	r.Say("two controls written: c1 the literal exclusion removed -- the plain sed, which rewrites the three strings -- and c2 one vendored `usize` reverted to `size_t`")
	ctl := map[string]chan error{}
	for _, c := range []string{"c1", "c2"} {
		ctl[c] = build(filepath.Join(tmp, c), filepath.Join(tmp, c+".c"), filepath.Join(tmp, c+".log"))
	}
	defer func() {
		<-newB
		for _, ch := range ctl {
			select {
			case <-ch:
			default:
			}
		}
	}()

	// --- 1. the source, as arithmetic on the input ---------------------------
	var before int
	fmt.Sscan(beforeLines, &before)
	var fail []string
	mentions := func(t, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAllString(t, -1))
	}
	So, ok1 := z23Spans(oldC)
	if !ok1 {
		return stop("an unterminated literal in %s", f)
	}
	Sn, ok2 := z23Spans(newC)
	if !ok2 {
		return stop("an unterminated literal in %s", f)
	}
	inNull, inSize := mentions(oldC, "NULL"), mentions(oldC, "size_t")
	litNull := inNull - z23Outside(oldC, So, "NULL")
	litSize := inSize - z23Outside(oldC, So, "size_t")
	if litNull != 3 || litSize != 0 {
		fail = append(fail, fmt.Sprintf("the input has %d `NULL` and %d `size_t` inside literals, expected 3 and 0 -- the exclusion rule is about a set this phase has looked at", litNull, litSize))
	}
	for _, p := range []struct {
		Name string
		want int
		why  string
	}{
		{"NULL", litNull, fmt.Sprintf("exactly the literals: the E1507 message, \"[NULL]\" and \"NULL\", and nothing else in %d", inNull)},
		{"size_t", 0, fmt.Sprintf("every one of the %d, there being no literal to spare", inSize)},
		{"nullptr", inNull - litNull, "one at each `NULL` outside a literal"},
		{"usize", inSize + 1, "one at each `size_t`, plus its own typedef"},
		{"typeof", 1, "the typedef and nowhere else"},
	} {
		if m := mentions(newC, p.Name); m != p.want {
			fail = append(fail, fmt.Sprintf("`%s` has %d mentions, expected %d -- %s", p.Name, m, p.want, p.why))
		}
	}
	want := []string{`"E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"`, `"[NULL]"`, `"NULL"`}
	for _, lit := range want {
		if strings.Count(oldC, lit) != 1 || strings.Count(newC, lit) != 1 {
			fail = append(fail, fmt.Sprintf("the literal %s occurs %d times in the input and %d in the output, and must occur once in each -- a rename that reached inside a string changes DATA", lit, strings.Count(oldC, lit), strings.Count(newC, lit)))
		}
	}
	litsWith := func(t string, S [][2]int, re *regexp.Regexp) []string {
		var o []string
		for _, s := range S {
			if re.MatchString(t[s[0]:s[1]]) {
				o = append(o, t[s[0]:s[1]])
			}
		}
		return o
	}
	if strings.Join(litsWith(newC, Sn, regexp.MustCompile(`\b(NULL|nullptr)\b`)), "\x00") != strings.Join(litsWith(oldC, So, regexp.MustCompile(`\bNULL\b`)), "\x00") {
		fail = append(fail, "the literals mentioning `NULL` are not the same literals in the same order as in the input")
	}
	const typedef = "typedef typeof(sizeof(0)) usize;"
	L := strings.Split(newC, "\n")
	if strings.Count(newC, typedef+"\n") != 1 || len(L) < 13 || L[12] != typedef {
		fail = append(fail, fmt.Sprintf("`%s` is not on line 13 of the output exactly once -- it belongs directly below the includes, so that when phase 109 moves them to the bottom it is the first line of the core", typedef))
	}
	intro := regexp.MustCompile(`(?m)^[^\n]*\btypedef\b[^\n]*\busize\b[^\n]*$`).FindAllString(newC, -1)
	if len(intro) != 1 || intro[0] != typedef {
		s := strings.Join(intro, " / ")
		if s == "" {
			s = "nothing"
		}
		fail = append(fail, fmt.Sprintf("`usize` is introduced by %s and not by one typedef -- it is a TYPE NAME and not a static object", s))
	}
	if len(L)-1 != before+2 {
		fail = append(fail, fmt.Sprintf("the file is %d lines and the input was %d -- expected exactly two more, the typedef and its blank line", len(L)-1, before))
	}
	if regexp.MustCompile(`\(\s*void\s*\*\s*\)\s*nullptr\b`).MatchString(newC) {
		fail = append(fail, "a `(void *)nullptr` survives: the cast existed for the variadic hazard of an UNTYPED null constant, and `nullptr` is typed")
	}
	nCast := len(regexp.MustCompile(`\(void \*\)NULL\b`).FindAllString(oldC, -1))
	if nCast != 30 {
		fail = append(fail, fmt.Sprintf("the input has %d `(void *)NULL`, and this phase was measured on 30", nCast))
	}
	rows := check.Z6RowRe.FindAllString(newC, -1)
	got, _ := harness.CommandNamesIn([]byte(newC), "whim-vim.c")
	if len(rows) != 98 || len(got) != 98 {
		fail = append(fail, "cmdnames[] is not the 98 rows phase 93 left -- this phase touches no Ex command")
	}
	if i := strings.Index(newC, "static struct vimoption options[]"); i >= 0 {
		j := strings.Index(newC[i:], "\n};")
		if m := len(check.Z12RowRe.FindAllString(newC[i:i+j], -1)); m != 107 {
			fail = append(fail, fmt.Sprintf("options[] has %d rows, expected the 107 phase 103 left -- this phase removes no option", m))
		}
	}
	var d []string
	for _, l := range L {
		if strings.HasPrefix(l, "#") {
			d = append(d, l)
		}
	}
	dirOK := len(d) == 11 && len(L) >= 11 && strings.Join(L[:11], "\n") == strings.Join(d, "\n")
	for _, l := range d {
		if !strings.HasPrefix(l, "#include <") {
			dirOK = false
		}
	}
	if !dirOK {
		fail = append(fail, "the output does not have exactly the eleven `#include` directives phase 104 left, on its first eleven lines.  MOVING THEM IS PHASE 110")
	}
	for k := 1; k < len(L); k++ {
		if L[k] == "" && L[k-1] == "" {
			fail = append(fail, "there is a run of two blank lines, which canon.sh should have taken")
			break
		}
	}
	if len(fail) > 0 {
		for _, l := range fail {
			r.Say("%s", l)
		}
		return harness.ErrReported
	}
	r.Say("`NULL` %d -> %d and `nullptr` 0 -> %d; `size_t` %d -> 0 and `usize` 0 -> %d, the extra one being its own typedef.  Every count is computed FROM THE INPUT, so this is a check on the phase and not on whatever it was handed", inNull, litNull, inNull-litNull, inSize, inSize+1)
	r.Cont("THE THREE LITERALS ARE UNCHANGED, and they are the whole of what a line-wise sed would have got wrong: %s", strings.Join(want, ", "))
	r.Cont("%d `(void *)NULL` became plain `nullptr` -- the cast existed for the variadic hazard of an untyped null constant, and there is not one `(void *)nullptr` left; eleven vendored signatures took `usize` with everything else, and they have been the core's own since phases 97 and 98, so no contract with anybody moved", nCast)
	r.Cont("eleven #includes still on the first eleven lines -- MOVING THEM IS PHASE 110 -- cmdnames[] 98, options[] 107, %d -> %d lines and no run of two blank lines", before, len(L)-1)

	// --- 2. C23, and that `usize` IS `size_t` --------------------------------
	line13 := ""
	if len(L) >= 13 {
		line13 = L[12]
	}
	if line13 != typedef {
		return stop("line 13 of the output is not the typedef: %s", line13)
	}
	g := filepath.Join(tmp, "g.c")
	os.WriteFile(g, []byte(line13+"\n"+
		"static usize probe(void) { return sizeof(usize); }\n"+
		"#include <stddef.h>\n"+
		"static_assert(_Generic((usize)0, size_t: 1, default: 0), \"usize IS size_t\");\n"+
		"static_assert(sizeof(nullptr) == sizeof(void *), \"nullptr is pointer-sized\");\n"+
		"int main(void) { return (int)probe() - (int)sizeof(size_t); }\n"), 0o644)
	for _, std := range []string{"DEFAULT", "-std=c23"} {
		a := []string{"-Wall", "-Wextra", "-Wpedantic", "-o", filepath.Join(tmp, "g"), g}
		if std != "DEFAULT" {
			a = append([]string{std}, a...)
		}
		c := exec.Command("gcc", a...)
		var eb bytes.Buffer
		c.Stderr = &eb
		if c.Run() != nil {
			r.Say("the typedef is REFUSED under %s, and this phase requires it:", std)
			ls := strings.Split(strings.TrimRight(eb.String(), "\n"), "\n")
			for _, l := range check.Head(ls, 4) {
				fmt.Fprintln(w, "               "+l)
			}
			return harness.ErrReported
		}
	}
	if exec.Command(filepath.Join(tmp, "g")).Run() != nil {
		return stop("the probe ran and disagreed: sizeof(usize) is not sizeof(size_t)")
	}
	for _, std := range []string{"-std=c11", "-std=c99"} {
		if exec.Command("gcc", std, "-o", "/dev/null", g).Run() == nil {
			r.Say("the typedef is ACCEPTED under %s, and it must not be:", std)
			r.Cont("\"typeof\" is C23, and a check that passes under C11 is not")
			r.Cont("stating the dependency this file has.")
			return harness.ErrReported
		}
	}
	r.Say("C23 IS A REAL DEPENDENCY AND IT IS STATED: the typedef taken out of line 13 of the output compiles under gcc's default and -std=c23 and is REFUSED under -std=c11 and -std=c99.  It is not a NEW dependency -- this file already needs C23 for `enum : long`, `static_assert` and lowercase `bool`/`true`/`false`")
	r.Say("AND THE DERIVATION HOLDS: with the real <stddef.h> arriving after it in the same translation unit, _Generic((usize)0, size_t: 1, default: 0) is 1 -- `usize` IS `size_t`, the same TYPE and not merely the same width, on any target rather than on this one -- and sizeof(nullptr) == sizeof(void *), which is why the thirty casts could go")

	// --- 3. the compile, the linkage and the libc surface --------------------
	beforeU := check.ReadFile(filepath.Join(state, "symbols", "undefined"))
	if err := check.Run(w, "sh", "tools/phasecheck.sh", work, f, filepath.Join(state, "symbols")); err != nil {
		return harness.ErrReported
	}
	afterU := check.ReadFile(".cache/symbols/last/undefined")
	if beforeU != afterU {
		b, a := strings.Fields(beforeU), strings.Fields(afterU)
		r.Say("the libc surface moved, and RENAMING A TYPE CANNOT MOVE IT:")
		r.Cont("gone: %s", check.TrSpace(check.Comm23(b, a)))
		r.Cont("came: %s", check.TrSpace(check.Comm23(a, b)))
		return harness.ErrReported
	}
	r.Say("symbols %s -> %s, and the set is IDENTICAL as a cmp -- nothing left and nothing arrived; main is still the only external symbol",
		strings.TrimSpace(check.ReadFile(".cache/symbols/last/before")), strings.TrimSpace(check.ReadFile(".cache/symbols/last/after")))

	// --- 4. the binary -------------------------------------------------------
	bl := &check.Rep{Tag: "build", W: w}
	_ = exec.Command("make", "-C", work, "clean").Run()
	if _, err := os.Stat(filepath.Join(work, "whim-vim")); err == nil {
		bl.Say("the clean did not remove whim-vim, so a 'rebuild' below could be no rebuild at all")
		return harness.ErrReported
	}
	if err := exec.Command("make", "-C", work).Run(); err != nil {
		bl.Say("FAILED -- rerun by hand: make -C %s", work)
		return harness.ErrReported
	}
	bin := filepath.Join(work, "whim-vim")
	bl.Say("ok, %s -> %d lines, %d bytes", beforeLines, check.CountLines([]byte(check.ReadFile(f))), check.SizeOf(bin))
	if err := <-newB; err != nil {
		newB <- err
		return stop("the reproducible build of the output failed")
	}
	newB <- nil
	oldBin, newBin := filepath.Join(state, "old"), filepath.Join(tmp, "new")
	oldSize, newSize := check.SizeOf(oldBin), check.SizeOf(newBin)
	if newSize < 500000 || oldSize < 500000 {
		return stop("one of the two binaries is %d / %d bytes, which is not an editor -- a cmp of two files nothing wrote passes", oldSize, newSize)
	}
	if newSize != check.SizeOf(bin) {
		return stop("the reproducible build is %d bytes and make produced %d: the two differ by more than a timestamp, so the comparison below would not be about this boundary", newSize, check.SizeOf(bin))
	}
	ob, nb := []byte(check.ReadFile(oldBin)), []byte(check.ReadFile(newBin))
	if !bytes.Equal(ob, nb) {
		r.Say("THE BINARY MOVED.  This phase renames two names and deletes")
		for _, l := range []string{
			"thirty casts whose reason has evaporated, and must change no",
			"code at all, so the two binaries -- the input's and the",
			"output's, both built with SOURCE_DATE_EPOCH=0 and the",
			"boundary's own flags -- must be the same bytes.",
			fmt.Sprintf("%d in, %d out.  The GNU build-id note is a hash of", oldSize, newSize),
			"the whole image and sits near the front, so the first difference",
			"below is always that note and never the change itself:"} {
			r.Cont("%s", l)
		}
		o, _ := exec.Command("cmp", oldBin, newBin).CombinedOutput()
		for _, l := range strings.Split(strings.TrimRight(string(o), "\n"), "\n") {
			fmt.Fprintln(w, "               "+l)
		}
		return harness.ErrReported
	}
	r.Say("THE BINARY IS BYTE-IDENTICAL, %d bytes either side -- tier 1 of CLAUDE.md's verification table, and the whole of this phase's evidence.  A byte-identical binary subsumes every screen case, every Ex-command row, every command line and every pty scenario at once, because the program that would be run is the same program; the core delta at phase 106 runs next and corroborates rather than proves", newSize)

	// --- 5. the controls -----------------------------------------------------
	for _, c := range []string{"c1", "c2"} {
		<-ctl[c]
		ctl[c] <- nil
	}
	for _, c := range []string{"c1", "c2"} {
		if fi, e := os.Stat(filepath.Join(tmp, c)); e != nil || fi.Mode()&0o111 == 0 {
			r.Say("the control %s did not build:", c)
			for _, l := range check.Head(strings.Split(strings.TrimRight(check.ReadFile(filepath.Join(tmp, c+".log")), "\n"), "\n"), 5) {
				fmt.Fprintln(w, "               "+l)
			}
			return harness.ErrReported
		}
	}
	c1b := []byte(check.ReadFile(filepath.Join(tmp, "c1")))
	if bytes.Equal(ob, c1b) {
		r.Say("THE CONTROL c1 DID NOT SHOW.  This phase's own output with the")
		r.Cont("literal exclusion removed -- the plain sed, which rewrites")
		r.Cont("\"[NULL]\", \"NULL\" and the E1507 message -- gives a binary")
		r.Cont("IDENTICAL to the input's, so the cmp above is two numbers")
		r.Cont("agreeing and proves nothing.  A test that cannot fail is not")
		r.Cont("evidence (CLAUDE.md).")
		return harness.ErrReported
	}
	diffBytes := check.Z23CmpL(ob, c1b)
	orod, crod := filepath.Join(tmp, "old.rodata"), filepath.Join(tmp, "c1.rodata")
	exec.Command("objcopy", "-O", "binary", "--only-section=.rodata", oldBin, orod).Run()
	exec.Command("objcopy", "-O", "binary", "--only-section=.rodata", filepath.Join(tmp, "c1"), crod).Run()
	if check.SizeOf(orod) <= 0 || check.SizeOf(crod) <= 0 {
		return stop("objcopy wrote an empty .rodata, and comparing two empty streams reports every pair of binaries identical (CLAUDE.md)")
	}
	rodataBytes := check.Z23CmpL([]byte(check.ReadFile(orod)), []byte(check.ReadFile(crod)))
	if rodataBytes < 1000 {
		return stop("c1 differs in %d bytes of .rodata, and the three strings it rewrites are 84 characters between them -- expected over a thousand", rodataBytes)
	}
	so, _ := exec.Command("strings", "-a", filepath.Join(tmp, "c1")).Output()
	if !check.HasLine(so, "[nullptr]") {
		return stop("c1's .rodata does not contain '[nullptr]', so the control did not do the thing it is a control for")
	}
	if !bytes.Equal(ob, []byte(check.ReadFile(filepath.Join(tmp, "c2")))) {
		r.Say("THE CONTROL c2 MOVED, AND IT IS DECLARED TO MOVE NOTHING.")
		r.Cont("One vendored parameter reverted from `usize` to `size_t`")
		r.Cont("must give the same bytes while the #includes are still at the")
		r.Cont("top of the file: the two are THE SAME TYPE.  If it now moves")
		r.Cont("something, this phase's account of its own evidence has to be")
		r.Cont("rewritten rather than the number quietly updated.")
		return harness.ErrReported
	}
	r.Say("AND IT CAN FAIL: this phase's own output with the literal exclusion removed -- the plain `sed 's/\\bNULL\\b/nullptr/g'`, which is the one mistake this phase can make -- differs from the input's binary in %d bytes, %d of them in .rodata, and `strings` finds '[nullptr]' where the editor's data said '[NULL]'.  That is CLAUDE.md's rule that the check for DATA is the strings, arriving on a phase nobody expected it on", diffBytes, rodataBytes)
	r.Say("AND ONE CONTROL MOVES NOTHING, WHICH IS REPORTED RATHER THAN HIDDEN: one vendored `usize` reverted to `size_t` is byte-identical, because the #includes are still at the TOP and `size_t` is still declared above every line of the file.  The rename is not load-bearing YET; at phase 109, which moves them, the same control is three hard errors")
	return nil
}
