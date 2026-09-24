package p106

// Whim phase 106 -- `nullptr` and `usize`: the two the language supplies.
// See GOALS.md II.4c and GOALS.md.
//
// GOALS.md II.4c settled the design on 2026-09-18: THERE IS NO SPLIT INTO TWO FILES,
// there is one file with two parts, and THE FIRST `#include` IS THE BOUNDARY.  The core
// is the prefix above it and must name nothing a header supplies.  Four phases get
// there; this is the first, and it is deliberately the smallest, BECAUSE IT IS THE ONE
// THAT CAN BE CHECKED BY `cmp`.
//
// WHAT IT DOES.  Two names the core takes from a header are replaced by two the
// LANGUAGE supplies, so that the core owes the header nothing for either:
//
// NULL    -> nullptr, a C23 KEYWORD.  Nothing is declared, no enumerator, no line.
// size_t  -> usize, with ONE new line, `typedef typeof(sizeof(0)) usize;`.
//
// NEITHER IS A NEW DEPENDENCY.  gcc here defaults to C23 -- measured, `__STDC_VERSION__`
// is `202311L` -- and this file already depends on it for `enum : long`, `static_assert`
// and the lowercase `bool`/`true`/`false` it uses throughout.  The check states that
// dependency as a measurement rather than leaving it implicit: it takes the typedef line
// OUT OF THE OUTPUT and compiles it under gcc's default, `-std=c23`, `-std=c11` and
// `-std=c99`, where the first two must accept it and the last two must refuse.
//
// WHY `typeof(sizeof(0))` AND NOT `unsigned long`.  `sizeof(0)` HAS type `size_t` by
// definition, so the typedef IS `size_t` on any conforming implementation -- the check
// proves it with `_Generic((usize)0, size_t: 1, default: 0)` against the real
// `<stddef.h>`, which is the same TYPE and not merely the same width.  The alternative
// that was on the table, `typedef unsigned long size_t;`, is correct on this target and
// SILENTLY WRONG on one where `size_t` is not `unsigned long`; and it is silent when it
// is right, so nothing here could tell the two apart.  A derivation cannot be wrong on
// a target this repository has never seen.
//
// THE ONE THING IN THIS PHASE THAT CAN GO WRONG IS THE LITERALS, and it is measured
// rather than reasoned about.  THREE STRING LITERALS IN THIS FILE CONTAIN `NULL`:
//
// "E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"
// "[NULL]"                      the printf layer's stand-in for a null %s argument
// "NULL"                        what `ga_print` writes for an empty growarray
//
// and NO literal contains `size_t`.  A line-wise `sed` rewrites all three: measured, the
// binary then differs by 1,598 bytes -- 50 in `.text`, 174 in `.data` and 1,354 in
// `.rodata` -- and `strings` shows `[nullptr]`, `nullptr` and an E1507 message that
// names a C keyword at the user.  With the three excluded the binary is `cmp`-IDENTICAL.
// That is CLAUDE.md's rule that the check for data is the STRINGS, arriving on a phase
// nobody expected it on, and internal/phase/106/check.go builds the literal-unaware form as a
// control and requires it to differ.
//
// So the substitution below is not a `sed`.  It scans the file for string and character
// literals first -- which is cheap and exact here, this file having no preprocessor and
// no comments -- and rewrites `\bNULL\b` and `\bsize_t\b` ONLY OUTSIDE them.  BOTH NAMES
// ARE REWRITTEN IN ONE PASS over the original text, and that is not tidiness: a second
// pass would index literal spans computed on the first pass's OUTPUT, and every span
// after the first replacement is shifted.  Measured, the two-pass form leaves five of
// the 437 `size_t` behind -- and leaves a file that still COMPILES and is still
// byte-identical, because `<stddef.h>` is still above it.  The mistake is invisible to
// everything in this phase but the count.
//
// THE THIRTY `(void *)NULL` BECOME PLAIN `nullptr`, which is a decision and not a
// mechanical consequence.  The cast exists for exactly one hazard: an untyped null
// constant in a VARIADIC argument position passes a four-byte `int` where the callee
// reads an eight-byte pointer, and gcc does not warn.  `nullptr` is TYPED --
// `sizeof(nullptr) == sizeof(void *)` -- so the hazard is gone and the cast says nothing
// a reader needs.  Twenty-eight of the thirty are the regexp parser's comma expressions,
// `return (emsg(...), rc_did_emsg = TRUE, (void *)NULL);`, where the cast was carrying
// the comma expression's type; nullptr_t converts to any pointer type on return, so
// they are the same program, which the `cmp` says.  Doing it here rather than later is
// what keeps those thirty sites from being touched twice.
//
// WHAT THE EDIT DOES NOT ASSERT, and it is deliberate: the NUMBER of `NULL` or `size_t`
// in its input.  This is one rule applied to every occurrence, and it is correct for any
// count; pinning the count would make the phase refuse on a tree that is merely bigger
// without making a wrong substitution any more visible.  What it does assert is
// STRUCTURAL and cannot shrink quietly -- the eleven directives, `usize` and `nullptr`
// at zero, and the classification below, which is a partition and not a count.
//
// EVERY `size_t` IS IN A POSITION A TYPEDEF SERVES, and that is what makes the rename
// safe rather than merely mechanical.  The edit classifies all 437 into CASTS (202,
// `(size_t)` and `((size_t)`) and DECLARATIONS (235: parameter, local, struct field and
// return type), and requires the two classes to cover every one with nothing left over.
// A leftover would be a use that is not a type name -- a case label, an array bound, a
// `sizeof(size_t)` -- and there are none.  The partition is computed from the text, so
// it stays true of a file this phase has never seen.
//
// THE ELEVEN VENDORED SIGNATURES CHANGE WITH EVERYTHING ELSE, AND THAT IS NOT AN
// INTERFACE CHANGE.  `musl_memcpy musl_memmove musl_memset musl_memcmp musl_memchr
// musl_strncpy musl_strncmp musl_strncasecmp musl_bsearch musl_qsort` take `usize`
// parameters and `musl_strlen` returns one.  They have been the core's OWN `static`
// definitions since phases 97 and 98 -- nothing outside this file calls them and nothing
// forces libc's spelling on them -- so renaming their parameter type changes no
// contract with anybody.
//
// THE `#include`s STAY WHERE THEY ARE.  Moving them to the bottom is phase 109, and it is
// what makes this phase's own rename load-bearing rather than cosmetic.  Until then
// `size_t` is still DECLARED above every line of this file, which has one consequence
// the check reports rather than hides: reverting a `usize` to `size_t` still compiles
// and still gives a byte-identical binary.  That control moves nothing here on purpose.
//
// THE INPUT BINARY IS BUILT by the plan (internal/build's OldBinary) with SOURCE_DATE_EPOCH=0, and it is this phase's whole
// evidence.  Nothing below changes a statement, so the check rebuilds the output the
// same way and requires THE SAME BYTES -- tier 1 of CLAUDE.md's verification table,
// which subsumes every screen case, every Ex-command row, every command line and every
// pty scenario at once, because the program that would run is the same program.
// SOURCE_DATE_EPOCH is required because version.c's `__DATE__ " " __TIME__` otherwise
// moves between any two builds; the file's NAME is not, whim-vim.c naming no `__FILE__`
// and no `__LINE__` and gcc not being given `-g`.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"bytes"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim106", Edit) }

var (
	whim106Inc     = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
	whim106Null    = regexp.MustCompile(`\bNULL\b`)
	whim106SizeT   = regexp.MustCompile(`\bsize_t\b`)
	whim106Both    = regexp.MustCompile(`\b(NULL|size_t)\b`)
	whim106Decl    = regexp.MustCompile(`^(\*\s*)*[A-Za-z_,)]`)
	whim106VoidPtr = regexp.MustCompile(`\(void \*\)nullptr\b`)
	whim106Intro   = regexp.MustCompile(`(?m)^[^\n]*\btypedef\b[^\n]*\busize\b[^\n]*$`)
)

const whim106Typedef = "typedef typeof(sizeof(0)) usize;"
const whim106Anchor = "#include <termios.h>\n\n"

// Whim106 gives the core two names the language supplies instead of a header:
// NULL becomes nullptr and size_t becomes usize.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	nInc := edit.IncludeCount(text) // the headers it was handed (phase 167 drops the unused)
	p := edit.Ph{Tag: "language", W: w}

	// ---- 0. the file this edit was written against -----------------------
	// ELEVEN DIRECTIVES on the first eleven lines: phase 104 left that, and
	// this phase adds a line directly below them, so it must know exactly
	// where they end.
	if err := whim106Directives(p, text, nInc, "the file does not have exactly eleven preprocessor directives on its first "+
		"eleven lines"); err != nil {
		return nil, err
	}
	for _, name := range []string{"usize", "nullptr"} {
		if k := p.Mentions(text, name); k != 0 {
			return nil, p.Die("`%s` already occurs %d times -- this phase introduces it, so an existing "+
				"mention means the phase has already run or the name is taken", name, k)
		}
	}
	p.Say("eleven directives, every one an `#include <...>` on the first eleven lines, and " +
		"`usize` and `nullptr` at zero mentions")

	// ---- 1. the literals, which are the one thing here that can go wrong -
	spans, err := edit.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	starts := make([]int, len(spans))
	for i, s := range spans {
		starts[i] = s[0]
	}
	inLiteral := func(pos int) bool {
		k := sort.SearchInts(starts, pos+1) - 1
		return k >= 0 && spans[k][0] <= pos && pos < spans[k][1]
	}

	// THE THREE, NAMED, because the phase must be able to say afterwards
	// that they are UNCHANGED -- and because a fourth appearing is a fact
	// worth refusing on: it would be a message this phase has never seen.
	wantLits := []string{
		`"E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"`,
		`"[NULL]"`,
		`"NULL"`,
	}
	var holding []string
	for _, s := range spans {
		if whim106Null.Match(text[s[0]:s[1]]) {
			holding = append(holding, string(text[s[0]:s[1]]))
		}
	}
	sortedHolding := append([]string(nil), holding...)
	sort.Strings(sortedHolding)
	sortedWant := append([]string(nil), wantLits...)
	sort.Strings(sortedWant)
	if strings.Join(sortedHolding, "\x00") != strings.Join(sortedWant, "\x00") {
		j := strings.Join(sortedHolding, " / ")
		if j == "" {
			j = "none"
		}
		return nil, p.Die("the literals containing `NULL` are not the three this phase knows about: %s", j)
	}
	for _, s := range spans {
		if whim106SizeT.Match(text[s[0]:s[1]]) {
			return nil, p.Die("a literal contains `size_t`, which no literal in this file ever has")
		}
	}
	p.Sayf("%d string and character literals, THREE of which contain `NULL` -- the E1507 "+
		"message, \"[NULL]\" and \"NULL\" -- and none of which contains `size_t`.  A line-wise "+
		"sed rewrites all three and moves 1,598 bytes of the binary", len(spans))

	// ---- 2. every `size_t` is in a position a typedef serves -------------
	// A PARTITION AND NOT A COUNT: casts and declarations must cover every
	// occurrence with nothing left over.  A leftover is a use that is not a
	// type name -- a case label, an array bound, a `sizeof(size_t)` -- which
	// a typedef could not serve.
	cast, decl := 0, 0
	for _, loc := range whim106SizeT.FindAllIndex(text, -1) {
		after := bytes.TrimLeft(text[loc[1]:], " \t\n\r\v\f")
		before := bytes.TrimRight(text[:loc[0]], " \t\n\r\v\f")
		switch {
		case bytes.HasSuffix(before, []byte("(")) && bytes.HasPrefix(after, []byte(")")):
			cast++
		case whim106Decl.Match(after):
			decl++
		default:
			lo := loc[0] - 30
			if lo < 0 {
				lo = 0
			}
			hi := loc[1] + 30
			if hi > len(text) {
				hi = len(text)
			}
			return nil, p.Die("`size_t` at line %d is neither a cast nor a declaration, so it is a "+
				"position a typedef may not serve: %s",
				bytes.Count(text[:loc[0]], []byte{'\n'})+1, cutil.PyRepr(string(text[lo:hi])))
		}
	}
	p.Sayf("%d mentions of `size_t`, ALL of them type-name positions: %d casts and %d "+
		"declarations, and nothing left over", cast+decl, cast, decl)

	// ---- 3. the substitution: one pass, outside literals -----------------
	// ONE PASS OVER THE ORIGINAL TEXT, for both names.  Two passes would
	// index spans computed on the first pass's OUTPUT, and measured, that
	// leaves five of the 437 `size_t` behind -- in a file that still compiles
	// and whose binary is still identical.
	repl := map[string]string{"NULL": "nullptr", "size_t": "usize"}
	count := map[string]int{}
	var Out []byte
	last := 0
	for _, loc := range whim106Both.FindAllIndex(text, -1) {
		if inLiteral(loc[0]) {
			continue
		}
		name := string(text[loc[0]:loc[1]])
		Out = append(Out, text[last:loc[0]]...)
		Out = append(Out, repl[name]...)
		last = loc[1]
		count[name]++
	}
	Out = append(Out, text[last:]...)
	text = Out
	p.Sayf("`NULL` -> `nullptr` at %d sites and `size_t` -> `usize` at %d, in one pass and "+
		"outside every literal", count["NULL"], count["size_t"])

	// ---- 4. the cast the hazard used to need -----------------------------
	nCast := len(whim106VoidPtr.FindAll(text, -1))
	if nCast == 0 {
		return nil, p.Die("no `(void *)NULL` site was found, and the phase states there are thirty -- the " +
			"convention that made the untyped spelling survivable is not written the way " +
			"this edit reads it")
	}
	text = whim106VoidPtr.ReplaceAll(text, []byte("nullptr"))
	p.Sayf("%d `(void *)nullptr` -> `nullptr`: the cast existed for the variadic hazard, and "+
		"`nullptr` is typed, so it says nothing a reader needs", nCast)

	// ---- 5. the one new line ---------------------------------------------
	// DIRECTLY BELOW THE ELEVEN INCLUDES, so that when phase 110 moves them to
	// the bottom the typedef is the first line of the core.  A TYPEDEF and
	// not a `static` anything: `usize` is a type name, and every one of its
	// uses is a type-name position.
	// the LAST #include, whichever it is: phase 167 drops the unused headers
	// later, and <termios.h> is last only once it has
	anchor := whim106Anchor
	for _, l := range bytes.Split(text, []byte{'\n'}) {
		if bytes.HasPrefix(l, []byte("#include ")) {
			anchor = string(l) + "\n\n"
		}
	}
	if k := bytes.Count(text, []byte(anchor)); k != 1 {
		return nil, p.Die("the last `#include` is not followed by exactly one blank line, so there is no " +
			"unambiguous place for the typedef")
	}
	text = bytes.Replace(text, []byte(anchor),
		[]byte(anchor+whim106Typedef+"\n\n"), 1)

	// ---- 6. what the file is now -----------------------------------------
	if err := whim106Directives(p, text, nInc, "the eleven directives are no longer the first eleven lines"); err != nil {
		return nil, p.Die("the eleven directives are no longer the first eleven lines")
	}
	lines := bytes.Split(text, []byte{'\n'})
	at := nInc + 1 // below the includes and their blank
	if len(lines) <= at || string(lines[at]) != whim106Typedef {
		got := ""
		if len(lines) > at {
			got = string(lines[at])
		}
		return nil, p.Die("the typedef did not land on line "+strconv.Itoa(at+1)+", below the includes and their blank: %s",
			cutil.PyRepr(got))
	}
	if bytes.Count(text, []byte(whim106Typedef+"\n")) != 1 {
		return nil, p.Die("the typedef is not in the file exactly once")
	}
	for _, c := range []struct {
		Name string
		want int
	}{{"NULL", 3}, {"size_t", 0}} {
		if k := p.Mentions(text, c.Name); k != c.want {
			return nil, p.Die("`%s` has %d mentions after the cut, expected %d", c.Name, k, c.want)
		}
	}
	after, err := edit.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	var stillHolding []string
	for _, s := range after {
		if whim106Null.Match(text[s[0]:s[1]]) {
			stillHolding = append(stillHolding, string(text[s[0]:s[1]]))
		}
	}
	if strings.Join(stillHolding, "\x00") != strings.Join(holding, "\x00") {
		return nil, p.Die("the three literals holding `NULL` are not the three they were")
	}
	intro := whim106Intro.FindAllString(string(text), -1)
	if len(intro) != 1 || intro[0] != whim106Typedef {
		j := strings.Join(intro, " / ")
		if j == "" {
			j = "nothing"
		}
		return nil, p.Die("`usize` is introduced by something other than exactly one typedef: %s -- it is "+
			"a TYPE NAME and not a static object, and every one of its uses is a type-name "+
			"position", j)
	}
	p.Sayf("the three `NULL` literals are the only `NULL` left, `size_t` is at zero, `usize` "+
		"is a typedef on line 13 and %d runs of two blank lines, exactly as before",
		p.BlankRuns(text))
	return text, nil
}

// whim106Directives requires exactly n directives on the first n lines, each an
// `#include <...>` of a system header.
func whim106Directives(p edit.Ph, text []byte, n int, msg string) error {
	lines := bytes.Split(text, []byte{'\n'})
	var idx []int
	var at []string
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte("#")) {
			idx = append(idx, i)
			at = append(at, strconv.Itoa(i))
		}
	}
	bad := len(idx) != n
	for k, i := range idx {
		if i != k {
			bad = true
		}
	}
	if bad {
		return p.Die("%s: %d directives at lines %s", msg, len(idx), strings.Join(at, " "))
	}
	for _, i := range idx {
		if !whim106Inc.Match(lines[i]) {
			return p.Die("a directive is not an `#include <...>` of a system header, and no phase may " +
				"add one")
		}
	}
	return nil
}
