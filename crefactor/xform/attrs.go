package xform

import (
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	ctext "github.com/arbace/go-whim/crefactor/text"
)

var (
	atInc     = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
	atAttr    = regexp.MustCompile(`__attribute__\(\((\w+)`)
	atWords   = regexp.MustCompile(`attribute|fallthrough|unused`)
	atFall    = regexp.MustCompile(`(?m)^[ ]*__attribute__\(\(fallthrough\)\);$`)
	atC23     = regexp.MustCompile(`(?m)^[ ]*\[\[fallthrough\]\];$`)
	atHead    = regexp.MustCompile(`^(?:static\s+[\w \*]+?\s*\**)?(\w+)\s*$`)
	atFmt     = regexp.MustCompile(`format(_arg)?\(`)
	atPad     = regexp.MustCompile(`  [,)]`)
	atUnused  = regexp.MustCompile(`__attribute__\(\(unused\)\)`)
	atNeedleS = " __attribute__((unused))"
)

var atKinds = []string{"unused", "fallthrough", "format", "format_arg"}

// Attrs is the step that normalises a file's GNU attributes, taking a
// different decision on each of the four kinds it knows and refusing any
// other: `__attribute__((unused))` on a function definition's parameter is
// deleted with the space before it (an unused parameter is not diagnosed
// under -Wno-unused-parameter, which is how the sweep compiles);
// `__attribute__((fallthrough));`, a statement on a line of its own, is
// respelled `[[fallthrough]];`, the C23 form; and `format` and `format_arg`
// stay, being the only ones that do work (format checking).  Both edits are
// within lines, and the file's directives, every one an `#include <...>` on
// its first lines, stay where they are.  It takes no knobs and no arguments.
func Attrs() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		if _, err := flags("attrs", args); err != nil {
			return nil, err
		}
		return attrs(text, w)
	}
}

func attrs(text []byte, w io.Writer) ([]byte, error) {
	nInc := ctext.IncludeCount(text) // the headers it was handed
	p := ctext.Ph{Tag: "attrs", W: w}

	// ---- 0. the file this edit was written against ----------------------------
	// `[[fallthrough]]` is a STATEMENT and not a directive, so this step must
	// leave the directives exactly where it found them.
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
		return nil, p.Die("the file does not have exactly its %d preprocessor directives on its first "+
			"lines: %d directives at lines %s", nInc, len(dIdx), strings.Join(at, " "))
	}
	for _, l := range dLines {
		if !atInc.MatchString(l) {
			return nil, p.Die("a directive is not an `#include <...>` of a system header, and no step may " +
				"add one")
		}
	}
	if strings.Contains(string(text), "[[") {
		return nil, p.Die("`[[` already occurs %d times -- this step introduces C23 attribute syntax, "+
			"so an existing occurrence means the step has already run or the spelling is "+
			"taken", strings.Count(string(text), "[["))
	}
	p.Sayf("%d directives, every one an `#include <...>` on the first %d lines, and "+
		"`[[` at zero occurrences", nInc, nInc)

	// ---- 1. the literals ------------------------------------------------------
	spans, err := ctext.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	var bad, words []string
	for _, s := range spans {
		lit := string(text[s[0]:s[1]])
		if strings.Contains(lit, "__attribute__") || strings.Contains(lit, "[[") {
			bad = append(bad, lit)
		}
		if atWords.MatchString(lit) {
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
	allAttrs := atAttr.FindAllStringSubmatch(string(text), -1)
	kinds := map[string]int{}
	for _, m := range allAttrs {
		kinds[m[1]]++
	}
	var extra []string
	for k := range kinds {
		if !ctext.Contains(atKinds, k) {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return nil, p.Die("the file holds an attribute this step has never looked at: %s -- its "+
			"decisions below are about %s and nothing else",
			strings.Join(extra, " "), strings.Join(atKinds, " "))
	}
	parts := make([]string, len(atKinds))
	for i, k := range atKinds {
		parts[i] = k + " " + strconv.Itoa(kinds[k])
	}
	p.Sayf("%d `__attribute__` in the file, and every one is one of four kinds: %s",
		len(allAttrs), strings.Join(parts, ", "))

	// ---- 3. unused: on a parameter, every one, computed ----------------------
	// THE SHAPE IS EXACT AND IT IS THE TRAP.  Each is written `<declarator>
	// __attribute__((unused))` -- ONE space before and none after -- and is
	// followed by the `,` or `)` of the parameter list.  Deleting the attribute
	// without its space would leave a space before a `,` or a `)`, and the canonical
	// print takes neither.  RE2 has no lookaround, so `(?<=\S)` and
	// `(?=[,)])` are the byte either side, tested.
	var unusedSpans [][2]int
	s := string(text)
	for i := 0; ; {
		j := strings.Index(s[i:], atNeedleS)
		if j < 0 {
			break
		}
		j += i
		e := j + len(atNeedleS)
		if j > 0 && !atIsSpace(s[j-1]) && e < len(s) && (s[e] == ',' || s[e] == ')') {
			unusedSpans = append(unusedSpans, [2]int{j, e})
		}
		i = e
	}
	nUnused := len(unusedSpans)
	if nUnused != kinds["unused"] {
		return nil, p.Die("%d of the %d `unused` attributes are written the way this edit reads them -- "+
			"two spaces before, one after, and a `,` or `)` next.  Deleting the rest by a "+
			"different rule would leave a doubled space or a space before a paren, and "+
			"the canonical print takes neither", nUnused, kinds["unused"])
	}

	// EVERY ONE IS IN A FUNCTION DEFINITION'S PARAMETER LIST, computed on the
	// line.  No parenthesised group in the canonical print spans a line break,
	// so the innermost enclosing `(` is on the same line and walking back to it
	// is exact.
	seen := map[int]bool{}
	var unusedLines []int
	for _, m := range atUnused.FindAllStringIndex(s, -1) {
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
		if j < 0 || !atHead.MatchString(l[:j]) {
			return nil, p.Die("the `unused` at line %d is not inside a function's parameter list -- what "+
				"precedes its innermost `(` is %s, which is not a function name, so this "+
				"may be an attribute on a variable, an object or a field and the step has "+
				"no decision for those", i+1, ctext.PyRepr(atSlice(l, j)))
		}
		if lines[i+1] != "{" {
			return nil, p.Die("line %d holds an `unused` but is not a function DEFINITION header: the "+
				"line below it is %s and not `{`", i+1, ctext.PyRepr(lines[i+1]))
		}
	}
	p.Sayf("%d `__attribute__((unused))`, ALL of them in the parameter list of a function "+
		"DEFINITION -- %d header lines, every one followed by `{` -- so not one is on a "+
		"variable, an object, a type or a field.  The sweep's own flags are "+
		"`-Wall -Wextra -Wno-unused-parameter`, which is why they say nothing",
		nUnused, len(unusedLines))

	// ---- 4. the 20: a standalone statement, every one -------------------------
	nFall := len(atFall.FindAllString(s, -1))
	if nFall != kinds["fallthrough"] {
		return nil, p.Die("%d of the %d `fallthrough` attributes are a whole line of their own -- the "+
			"swap below is one-for-one and textual, and an attribute sharing a line with "+
			"anything else is not a case it has looked at", nFall, kinds["fallthrough"])
	}
	p.Sayf("%d `__attribute__((fallthrough));`, every one a standalone statement on a line of "+
		"its own, so the swap to the C23 spelling is one-for-one and reaches nothing else", nFall)

	// ---- 5. the format ones stay -----------------------------------------------
	// Recorded as the exact LINES they sit on, so the check can require them back
	// byte for byte.
	var keep []int
	var keepText []string
	for i, l := range lines {
		if atFmt.MatchString(l) && strings.Contains(l, "__attribute__") {
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
			"and there are %d in the file -- the step must be able to name every one it "+
			"keeps", len(keep), nKeep, kinds["format"]+kinds["format_arg"])
	}
	p.Sayf("%d `format`/`format_arg` on %d lines KEPT, and they are the only attributes doing "+
		"work nothing else does: without them the compiler checks no format string "+
		"they name", nKeep, len(keep))

	// ---- 6. the two substitutions ---------------------------------------------
	padBefore := len(atPad.FindAllString(s, -1))
	var Out strings.Builder
	prev := 0
	for _, sp := range unusedSpans {
		Out.WriteString(s[prev:sp[0]])
		prev = sp[1]
	}
	Out.WriteString(s[prev:])
	s = Out.String()
	a := len(unusedSpans)
	b := len(atFall.FindAllString(s, -1))
	s = atFall.ReplaceAllStringFunc(s, func(m string) string {
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
	for _, m := range atAttr.FindAllStringSubmatch(s, -1) {
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
	if len(atFall.FindAllString(s, -1)) > 0 || strings.Contains(s, "__attribute__((unused))") {
		return nil, p.Die("an `unused` or a GNU `fallthrough` survives the substitution")
	}
	if len(atC23.FindAllString(s, -1)) != nFall {
		return nil, p.Die("the %d C23 statements are not %d standalone lines", nFall, nFall)
	}
	if k := len(atPad.FindAllString(s, -1)); k != padBefore {
		return nil, p.Die("the edit left %d doubled spaces before a `,` or `)` where there were %d -- "+
			"deleting the attribute without its own two spaces is exactly the mistake this "+
			"step can make, and the canonical print does not take it", k, padBefore)
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

func atIsSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// atSlice is `l[:j]` in Python's sense, negative j included -- the refusal quotes it and
// j is -1 exactly when the attribute is the first thing on the line.
func atSlice(l string, j int) string {
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
