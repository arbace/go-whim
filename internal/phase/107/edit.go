package p107

// Whim phase 107 -- the attributes: 113 that say nothing, 20 that change spelling, 6 that
// stay.  See GOALS.md II.4c and GOALS.md.
//
// `__attribute__` IS A GNU EXTENSION, and this file has 139 of them in a core that is
// headed for another runtime.  The phase looks at all 139, in three groups, and takes a
// different decision on each -- which is the whole of it, and the reason it stops at 133
// rather than 139 is the third group.
//
// 113  __attribute__((unused))          DELETED.  They say nothing here.
// 20  __attribute__((fallthrough));    RESPELLED `[[fallthrough]];`, the C23 form.
// 6  format / format_arg              KEPT, and they are the only ones doing work.
//
// THE 113 SAY NOTHING BECAUSE OF THE FLAGS THE SWEEP ALREADY USES.  Every dead-code
// compile in this pipeline is `-Wall -Wextra -Wno-unused-parameter`
// (tools/deadsweep.py:71, tools/phasecheck.sh:48), so an unused PARAMETER is not
// diagnosed whatever is written on it.  MEASURED: with all 113 gone the sweep's own
// command line prints nothing at all, exactly as it does today.  Upstream needs them
// because upstream compiles this file in configurations where the parameter really is
// unused and others where it is not; there are no configurations here.
//
// AND ALL 113 ARE ON PARAMETERS, WHICH IS COMPUTED AND NOT ASSUMED.  Every one sits
// inside a parenthesised group whose innermost enclosing `(` is preceded by exactly a
// function name -- the 97 lines that hold them are all function DEFINITION headers, each
// followed by a line that is `{` -- so not one is on a variable, an object, a type or a
// field.  This file has no parenthesised group spanning a line break (CLAUDE.md), so
// that is a computation on one line and not a parse of C.  The check states it a second
// way, from the compiler: with `-Wunused-parameter` turned back ON, removing the 113
// produces 92 new warnings and EVERY ONE of them is `-Wunused-parameter` at a line that
// carried an attribute -- not one `-Wunused-variable`, which is what a misplaced
// deletion would have produced.
//
// THE OTHER 21 ARE THE INTERESTING NUMBER: 113 sites, 92 warn when the attribute goes,
// so TWENTY-ONE OF THEM MARK A PARAMETER THIS BUILD USES.  `ex_cquit(exarg_T *eap)`
// reads `eap->addr_count` on its first line; `check_winopt(winopt_T *wop)` dereferences
// `wop` five times; `deathtrap(int sigarg)` compares `sigarg` against SIGHUP.  The
// attribute is not merely redundant there, it is false, and it has been false since some
// whim or Part II phase made the parameter live again.  Deleting all 113 deletes 21 wrong
// statements along with 92 unnecessary ones.
//
// THE 20 ARE A ONE-FOR-ONE TEXTUAL SWAP, and `[[fallthrough]]` IS NOT A DIRECTIVE.  It
// is C23 attribute syntax -- a statement, in the grammar, spelled with brackets -- and
// the charter's rule is about PREPROCESSOR syntax: nothing here begins with `#`,
// nothing is expanded, and `gcc -E` on the output produces the same eleven headers
// pasted in and not one line more.  All 20 sites are standalone statements on lines of
// their own, so the swap cannot reach anything else; `[[` occurs ZERO times in the input
// and 20 times in the output, which is the assertion that says so.
//
// C23 IS NOT A NEW DEPENDENCY AND THE CHECK STATES WHAT IT IS RATHER THAN ASSUMING IT.
// Phase 106 measured `-std=c11` REFUSING its typedef; `[[fallthrough]]` is weaker than
// that and the difference is written down rather than glossed: gcc accepts it under
// every `-std` it has, and below C23 `-Wpedantic` says `ISO C does not support '[[]]'
// attributes before C23` and `-pedantic-errors` REFUSES it.  The GNU spelling it
// replaces is pedantically clean everywhere, being a reserved identifier.  So taken
// alone this swap narrows the dialects the file compiles under, and it costs nothing
// because the file is already C23 by four other routes -- `enum : long`,
// `static_assert`, lowercase `bool`, and phase 106's `typeof` and `nullptr`.  MEASURED,
// and computed rather than written here: `-std=c11` on the WHOLE file gives the same
// number of errors before this phase and after it.  The dialect floor does not move.
//
// THE SIX STAY, AND THE BINARY CANNOT TELL YOU WHY.  MEASURED: with all six removed the
// binary is `cmp`-IDENTICAL and `-Wformat=2` goes from 115 `-Wformat-nonliteral`
// warnings to ZERO.  They emit no code and decide what gcc will catch:
//
// format(printf, 3, 4)   on vim_snprintf, and format(printf, 3, 0) on the two
// v-forms.  Phase 105 expanded seven wrappers into 129 direct
// calls, so this ONE attribute is now what type-checks 201
// `vim_snprintf` mentions' arguments.  The check takes the
// prototype line verbatim out of the output and hands
// `vim_snprintf(b, 10, "%d", s)` a `const char *`: with the
// attribute gcc says `format '%d' expects argument of type
// 'int'`, and without it gcc is SILENT.
// format_arg(1)          on `_()` and format_arg(1)/(2) on `NGETTEXT`.  CLAUDE.md
// names this as the reason those two are `static inline`
// functions rather than macros: it is what lets `-Wformat`
// see THROUGH the translation wrapper.  MEASURED from the
// other end -- remove `_()`'s and the count goes 115 -> 135,
// twenty formats gcc stops being able to follow.
//
// So the phase removes every attribute whose job another flag already does, and keeps
// every attribute that is itself the flag.  That is a rule and not a list, and it is
// what a later phase should apply to anything new.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and it is this phase's whole
// evidence, exactly as at phase 106.  Neither edit generates code: `unused` suppresses a
// diagnostic and `[[fallthrough]]` is a hint to the same diagnostic machinery.  The
// check rebuilds the output the same way and requires THE SAME BYTES -- tier 1 of
// CLAUDE.md's verification table, which subsumes every screen case, every Ex-command
// row, every command line and every pty scenario at once, because the program that would
// run is the same program.  Nothing is staged and no editor is run, for that reason.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim107", Edit) }

var (
	w107Inc     = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
	w107Attr    = regexp.MustCompile(`__attribute__\(\((\w+)`)
	w107Words   = regexp.MustCompile(`attribute|fallthrough|unused`)
	w107Fall    = regexp.MustCompile(`(?m)^[ ]*__attribute__\(\(fallthrough\)\);$`)
	w107C23     = regexp.MustCompile(`(?m)^[ ]*\[\[fallthrough\]\];$`)
	w107Head    = regexp.MustCompile(`^(?:static\s+[\w \*]+?\s*\**)?(\w+)\s*$`)
	w107Fmt     = regexp.MustCompile(`format(_arg)?\(`)
	w107Pad     = regexp.MustCompile(`  [,)]`)
	w107Unused  = regexp.MustCompile(`__attribute__\(\(unused\)\)`)
	w107NeedleS = " __attribute__((unused))"
)

// w107Kinds are the four kinds of attribute this phase has a decision for.  A
// FIFTH APPEARING IS A DECISION THIS PHASE HAS NEVER TAKEN, and it must refuse
// rather than leave it or guess -- a partition and not a count.
var w107Kinds = []string{"unused", "fallthrough", "format", "format_arg"}

// Whim107 takes the attributes: 139 GNU `__attribute__` to six.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 169 drops the unused)
	p := edit.Ph{Tag: "attrs", W: w}

	// ---- 0. the file this edit was written against ----------------------------
	// `[[fallthrough]]` is a STATEMENT and not a directive, so this phase must
	// leave the eleven exactly where it found them.
	lines := strings.Split(string(text), "\n")
	var dIdx []int
	var dLines []string
	for i, l := range lines {
		if strings.HasPrefix(l, "#") {
			dIdx = append(dIdx, i)
			dLines = append(dLines, l)
		}
	}
	ok := len(dIdx) == nInc
	for i := range dIdx {
		if dIdx[i] != i {
			ok = false
		}
	}
	if !ok {
		at := make([]string, len(dIdx))
		for i, v := range dIdx {
			at[i] = strconv.Itoa(v)
		}
		return nil, p.Die("the file does not have exactly eleven preprocessor directives on its first "+
			"eleven lines: %d directives at lines %s", len(dIdx), strings.Join(at, " "))
	}
	for _, l := range dLines {
		if !w107Inc.MatchString(l) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	if strings.Contains(string(text), "[[") {
		return nil, p.Die("`[[` already occurs %d times -- this phase introduces C23 attribute syntax, "+
			"so an existing occurrence means the phase has already run or the spelling is "+
			"taken", strings.Count(string(text), "[["))
	}
	p.Say("eleven directives, every one an `#include <...>` on the first eleven lines, and " +
		"`[[` at zero occurrences")

	// ---- 1. the literals ------------------------------------------------------
	spans, err := edit.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	var bad, words []string
	for _, s := range spans {
		lit := string(text[s[0]:s[1]])
		if strings.Contains(lit, "__attribute__") || strings.Contains(lit, "[[") {
			bad = append(bad, lit)
		}
		if w107Words.MatchString(lit) {
			words = append(words, lit)
		}
	}
	if len(bad) > 0 {
		return nil, p.Die("a literal holds `__attribute__` or `[[`, and no substitution below may reach "+
			"inside a string: %s", strings.Join(bad, " / "))
	}
	p.Sayf("%d string and character literals, NONE holding `__attribute__` or `[[`.  Two hold "+
		"the English words and neither is reachable by either substitution: %s",
		len(spans), strings.Join(words, " / "))

	// ---- 2. the partition -----------------------------------------------------
	allAttrs := w107Attr.FindAllStringSubmatch(string(text), -1)
	kinds := map[string]int{}
	for _, m := range allAttrs {
		kinds[m[1]]++
	}
	var extra []string
	for k := range kinds {
		if !edit.Contains(w107Kinds, k) {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return nil, p.Die("the file holds an attribute this phase has never looked at: %s -- the three "+
			"decisions below are about %s and nothing else",
			strings.Join(extra, " "), strings.Join(w107Kinds, " "))
	}
	parts := make([]string, len(w107Kinds))
	for i, k := range w107Kinds {
		parts[i] = k + " " + strconv.Itoa(kinds[k])
	}
	p.Sayf("%d `__attribute__` in the file, and every one is one of four kinds: %s",
		len(allAttrs), strings.Join(parts, ", "))

	// ---- 3. the 113: on a parameter, every one, computed ----------------------
	// THE SHAPE IS EXACT AND IT IS THE TRAP.  Each is written `<declarator>
	// __attribute__((unused))` -- ONE space before and none after -- and is
	// followed by the `,` or `)` of the parameter list.  Deleting the attribute
	// without its space would leave a space before a `,` or a `)`, and canon.sh
	// takes neither.  RE2 has no lookaround, so the Python's `(?<=\S)` and
	// `(?=[,)])` are the byte either side, tested.
	var unusedSpans [][2]int
	s := string(text)
	for i := 0; ; {
		j := strings.Index(s[i:], w107NeedleS)
		if j < 0 {
			break
		}
		j += i
		e := j + len(w107NeedleS)
		if j > 0 && !w107IsSpace(s[j-1]) && e < len(s) && (s[e] == ',' || s[e] == ')') {
			unusedSpans = append(unusedSpans, [2]int{j, e})
		}
		i = e
	}
	nUnused := len(unusedSpans)
	if nUnused != kinds["unused"] {
		return nil, p.Die("%d of the %d `unused` attributes are written the way this edit reads them -- "+
			"two spaces before, one after, and a `,` or `)` next.  Deleting the rest by a "+
			"different rule would leave a doubled space or a space before a paren, and "+
			"canon.sh takes neither", nUnused, kinds["unused"])
	}

	// EVERY ONE IS IN A FUNCTION DEFINITION'S PARAMETER LIST, computed on the
	// line.  No parenthesised group in this file spans a line break (CLAUDE.md),
	// so the innermost enclosing `(` is on the same line and walking back to it
	// is exact.
	seen := map[int]bool{}
	var unusedLines []int
	for _, m := range w107Unused.FindAllStringIndex(s, -1) {
		ln := strings.Count(s[:m[0]], "\n")
		if !seen[ln] {
			seen[ln] = true
			unusedLines = append(unusedLines, ln)
		}
	}
	sort.Ints(unusedLines)
	for _, i := range unusedLines {
		l := lines[i]
		k := strings.Index(l, "__attribute__((unused))")
		d, j := 0, -1
		for j = k - 1; j >= 0; j-- {
			if l[j] == ')' {
				d++
			} else if l[j] == '(' {
				if d == 0 {
					break
				}
				d--
			}
		}
		if j < 0 || !w107Head.MatchString(l[:j]) {
			return nil, p.Die("the `unused` at line %d is not inside a function's parameter list -- what "+
				"precedes its innermost `(` is %s, which is not a function name, so this "+
				"may be an attribute on a variable, an object or a field and the phase has "+
				"no decision for those", i+1, cutil.PyRepr(w107Slice(l, j)))
		}
		if lines[i+1] != "{" {
			return nil, p.Die("line %d holds an `unused` but is not a function DEFINITION header: the "+
				"line below it is %s and not `{`", i+1, cutil.PyRepr(lines[i+1]))
		}
	}
	p.Sayf("%d `__attribute__((unused))`, ALL of them in the parameter list of a function "+
		"DEFINITION -- %d header lines, every one followed by `{` -- so not one is on a "+
		"variable, an object, a type or a field.  The sweep's own flags are "+
		"`-Wall -Wextra -Wno-unused-parameter`, which is why they say nothing",
		nUnused, len(unusedLines))

	// ---- 4. the 20: a standalone statement, every one -------------------------
	nFall := len(w107Fall.FindAllString(s, -1))
	if nFall != kinds["fallthrough"] {
		return nil, p.Die("%d of the %d `fallthrough` attributes are a whole line of their own -- the "+
			"swap below is one-for-one and textual, and an attribute sharing a line with "+
			"anything else is not a case it has looked at", nFall, kinds["fallthrough"])
	}
	p.Sayf("%d `__attribute__((fallthrough));`, every one a standalone statement on a line of "+
		"its own, so the swap to the C23 spelling is one-for-one and reaches nothing else", nFall)

	// ---- 5. the six that stay -------------------------------------------------
	// Recorded as the exact LINES they sit on, so the check can require them back
	// byte for byte.
	var keep []int
	var keepText []string
	for i, l := range lines {
		if w107Fmt.MatchString(l) && strings.Contains(l, "__attribute__") {
			keep = append(keep, i)
			keepText = append(keepText, l)
		}
	}
	nKeep := 0
	for _, l := range keepText {
		nKeep += strings.Count(l, "__attribute__")
	}
	if nKeep != kinds["format"]+kinds["format_arg"] {
		return nil, p.Die("the `format` and `format_arg` attributes are on %d lines carrying %d of them, "+
			"and there are %d in the file -- the phase must be able to name every one it "+
			"keeps", len(keep), nKeep, kinds["format"]+kinds["format_arg"])
	}
	p.Sayf("%d `format`/`format_arg` on %d lines KEPT, and they are the only attributes doing "+
		"work nothing else does: with all six removed the binary is cmp-IDENTICAL and "+
		"`-Wformat=2` goes from 115 warnings to ZERO", nKeep, len(keep))

	// ---- 6. the two substitutions ---------------------------------------------
	padBefore := len(w107Pad.FindAllString(s, -1))
	var Out strings.Builder
	prev := 0
	for _, sp := range unusedSpans {
		Out.WriteString(s[prev:sp[0]])
		prev = sp[1]
	}
	Out.WriteString(s[prev:])
	s = Out.String()
	a := len(unusedSpans)
	b := len(w107Fall.FindAllString(s, -1))
	s = w107Fall.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Replace(m, "__attribute__((fallthrough));", "[[fallthrough]];", 1)
	})
	if a != nUnused || b != nFall {
		return nil, p.Die("the substitutions took %d and %d where %d and %d were counted",
			a, b, nUnused, nFall)
	}
	text = []byte(s)
	p.Sayf("%d `__attribute__((unused))` deleted with the space before them, and %d "+
		"`__attribute__((fallthrough));` respelled `[[fallthrough]];`", a, b)

	// ---- 7. what the file is now ----------------------------------------------
	L := strings.Split(s, "\n")
	if len(L) != len(lines) {
		return nil, p.Die("the file is %d lines and the input was %d -- both edits are WITHIN lines and "+
			"neither may add or remove one", len(L)-1, len(lines)-1)
	}
	var leftK []string
	for _, m := range w107Attr.FindAllStringSubmatch(s, -1) {
		leftK = append(leftK, m[1])
	}
	wantK := make([]string, 0, kinds["format"]+kinds["format_arg"])
	for i := 0; i < kinds["format"]; i++ {
		wantK = append(wantK, "format")
	}
	for i := 0; i < kinds["format_arg"]; i++ {
		wantK = append(wantK, "format_arg")
	}
	gotK := append([]string{}, leftK...)
	sort.Strings(gotK)
	sort.Strings(wantK)
	if strings.Join(gotK, " ") != strings.Join(wantK, " ") {
		shown := strings.Join(gotK, " ")
		if shown == "" {
			shown = "none"
		}
		return nil, p.Die("the attributes left are %s, and they must be exactly the %d format and %d "+
			"format_arg", shown, kinds["format"], kinds["format_arg"])
	}
	for i, k := range keep {
		if L[k] != keepText[i] {
			return nil, p.Die("a line carrying a kept attribute is not the line it was, byte for byte")
		}
	}
	if len(w107Fall.FindAllString(s, -1)) > 0 || strings.Contains(s, "__attribute__((unused))") {
		return nil, p.Die("an `unused` or a GNU `fallthrough` survives the substitution")
	}
	if len(w107C23.FindAllString(s, -1)) != nFall {
		return nil, p.Die("the %d C23 statements are not %d standalone lines", nFall, nFall)
	}
	if k := len(w107Pad.FindAllString(s, -1)); k != padBefore {
		return nil, p.Die("the edit left %d doubled spaces before a `,` or `)` where there were %d -- "+
			"deleting the attribute without its own two spaces is exactly the mistake this "+
			"phase can make, and canon.sh does not take it", k, padBefore)
	}
	changed := 0
	for i := range L {
		if L[i] != lines[i] {
			changed++
		}
	}
	if changed != len(unusedLines)+nFall {
		return nil, p.Die("%d lines changed, expected %d -- the %d headers and the %d fallthrough "+
			"statements, and nothing else", changed, len(unusedLines)+nFall, len(unusedLines), nFall)
	}
	p.Sayf("%d attributes -> %d, the same %d lines, %d changed -- the %d function headers and "+
		"the %d fallthrough statements -- and the doubled-space count unmoved at %d",
		len(allAttrs), len(leftK), len(L)-1, changed, len(unusedLines), nFall, padBefore)
	return text, nil
}

func w107IsSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// w107Slice is Python's `l[:j]`, negative j included -- the refusal quotes it and
// j is -1 exactly when the attribute is the first thing on the line.
func w107Slice(l string, j int) string {
	if j < 0 {
		j += len(l)
		if j < 0 {
			j = 0
		}
	}
	if j > len(l) {
		j = len(l)
	}
	return l[:j]
}
