package xform

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	ctext "github.com/arbace/go-whim/internal/crefactor/text"
)

// NullptrKnobs is what NullptrUsize is told.
type NullptrKnobs struct {
	// NullLiterals are the string literals that hold `NULL`, which stay as
	// they are; when not nil, any other set refuses.
	NullLiterals []string
}

var (
	nuInc     = regexp.MustCompile(`^#include <([A-Za-z0-9_/.]+)>$`)
	nuNull    = regexp.MustCompile(`\bNULL\b`)
	nuSizeT   = regexp.MustCompile(`\bsize_t\b`)
	nuBoth    = regexp.MustCompile(`\b(NULL|size_t)\b`)
	nuDecl    = regexp.MustCompile(`^(\*\s*)*[A-Za-z_,)]`)
	nuVoidPtr = regexp.MustCompile(`\(void \*\)nullptr\b`)
	nuIntro   = regexp.MustCompile(`(?m)^[^\n]*\btypedef\b[^\n]*\busize\b[^\n]*$`)
)

const nuTypedef = "typedef typeof(sizeof(0)) usize;"

// NullptrUsize is the step that gives a file two names the language supplies
// in place of two a header does: NULL becomes nullptr, a C23 keyword, and
// size_t becomes usize, declared by ONE new line directly below the file's
// leading `#include`s, `typedef typeof(sizeof(0)) usize;` -- which is size_t
// by definition, on any target.  Both are renamed in one pass over the text
// and only outside string and character literals, since a name in a literal
// is data; and every `(void *)nullptr` is then plain `nullptr`, which is
// typed, so the cast says nothing.
//
// It refuses a `size_t` in a position a typedef does not serve (a partition:
// every one is a cast or a declaration), a file whose directives are not all
// `#include <...>` on its first lines, and a file that already names usize
// or nullptr.  Its one argument is a floor: `--casts N` refuses when fewer
// than N `(void *)NULL` are found.
func NullptrUsize(k NullptrKnobs) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		f, err := flags("language", args, "--casts")
		if err != nil {
			return nil, err
		}
		return nullptrUsize(k, f["--casts"], text, w)
	}
}

func nullptrUsize(k NullptrKnobs, casts int, text []byte, w io.Writer) ([]byte, error) {
	nInc := ctext.IncludeCount(text) // the headers it was handed
	p := ctext.Ph{Tag: "language", W: w}

	// ---- 0. the file this step is written for ---------------------------
	// Every directive an `#include` on the file's first lines: the step adds
	// a line directly below them, so it must know exactly where they end.
	if err := nuDirectives(p, text, nInc, fmt.Sprintf("the file does not have exactly its %d preprocessor directives on its first "+
		"%d lines", nInc, nInc)); err != nil {
		return nil, err
	}
	for _, name := range []string{"usize", "nullptr"} {
		if n := p.Mentions(text, name); n != 0 {
			return nil, p.Die("`%s` already occurs %d times -- this step introduces it, so an existing "+
				"mention means the step has already run or the name is taken", name, n)
		}
	}
	p.Sayf("%d directives, every one an `#include <...>` on the first %d lines, and "+
		"`usize` and `nullptr` at zero mentions", nInc, nInc)

	// ---- 1. the literals, which are the one thing here that can go wrong -
	spans, err := ctext.LiteralSpans(p, text)
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

	// The literals holding `NULL` are data, and stay.  When the code base
	// names them (k.NullLiterals), a set other than those refuses: it would be
	// a message this step has never been told of.
	var holding []string
	inLits := 0
	for _, s := range spans {
		if n := len(nuNull.FindAll(text[s[0]:s[1]], -1)); n > 0 {
			holding = append(holding, string(text[s[0]:s[1]]))
			inLits += n
		}
	}
	if k.NullLiterals != nil {
		sortedHolding := append([]string(nil), holding...)
		sort.Strings(sortedHolding)
		sortedWant := append([]string(nil), k.NullLiterals...)
		sort.Strings(sortedWant)
		if strings.Join(sortedHolding, "\x00") != strings.Join(sortedWant, "\x00") {
			j := strings.Join(sortedHolding, " / ")
			if j == "" {
				j = "none"
			}
			return nil, p.Die("the literals containing `NULL` are not the %d this step was told of: %s", len(k.NullLiterals), j)
		}
	}
	for _, s := range spans {
		if nuSizeT.Match(text[s[0]:s[1]]) {
			return nil, p.Die("a literal contains `size_t`, and a rename may not reach into data")
		}
	}
	p.Sayf("%d string and character literals, %d of which contain `NULL` and none of which "+
		"contains `size_t`; a rename outside them leaves them as they are", len(spans), len(holding))

	// ---- 2. every `size_t` is in a position a typedef serves -------------
	// A PARTITION AND NOT A COUNT: casts and declarations must cover every
	// occurrence with nothing left over.  A leftover is a use that is not a
	// type name -- a case label, an array bound, a `sizeof(size_t)` -- which
	// a typedef could not serve.
	cast, decl := 0, 0
	for _, loc := range nuSizeT.FindAllIndex(text, -1) {
		after := bytes.TrimLeft(text[loc[1]:], " \t\n\r\v\f")
		before := bytes.TrimRight(text[:loc[0]], " \t\n\r\v\f")
		switch {
		case bytes.HasSuffix(before, []byte("(")) && bytes.HasPrefix(after, []byte(")")):
			cast++
		case nuDecl.Match(after):
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
				bytes.Count(text[:loc[0]], []byte{'\n'})+1, ctext.PyRepr(string(text[lo:hi])))
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
	for _, loc := range nuBoth.FindAllIndex(text, -1) {
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
	nCast := len(nuVoidPtr.FindAll(text, -1))
	if nCast < casts {
		return nil, p.Die("%d `(void *)NULL` sites, and this step was told there are at least %d", nCast, casts)
	}
	text = nuVoidPtr.ReplaceAll(text, []byte("nullptr"))
	p.Sayf("%d `(void *)nullptr` -> `nullptr`: the cast existed for the variadic hazard, and "+
		"`nullptr` is typed, so it says nothing a reader needs", nCast)

	// ---- 5. the one new line ---------------------------------------------
	// DIRECTLY BELOW THE INCLUDES, below the last of them, so that when they
	// move the typedef is the first line of what is left.  A TYPEDEF and not a
	// `static` anything: `usize` is a type name, and every one of its uses is a
	// type-name position.
	anchor := ""
	for _, l := range bytes.Split(text, []byte{'\n'}) {
		if bytes.HasPrefix(l, []byte("#include ")) {
			anchor = string(l) + "\n\n"
		}
	}
	if n := bytes.Count(text, []byte(anchor)); anchor == "" || n != 1 {
		return nil, p.Die("the last `#include` is not followed by exactly one blank line, so there is no " +
			"unambiguous place for the typedef")
	}
	text = bytes.Replace(text, []byte(anchor),
		[]byte(anchor+nuTypedef+"\n\n"), 1)

	// ---- 6. what the file is now -----------------------------------------
	if err := nuDirectives(p, text, nInc, "the directives are no longer the first lines"); err != nil {
		return nil, p.Die("the %d directives are no longer the first %d lines", nInc, nInc)
	}
	lines := bytes.Split(text, []byte{'\n'})
	at := nInc + 1 // below the includes and their blank
	if len(lines) <= at || string(lines[at]) != nuTypedef {
		got := ""
		if len(lines) > at {
			got = string(lines[at])
		}
		return nil, p.Die("the typedef did not land on line "+strconv.Itoa(at+1)+", below the includes and their blank: %s",
			ctext.PyRepr(got))
	}
	if bytes.Count(text, []byte(nuTypedef+"\n")) != 1 {
		return nil, p.Die("the typedef is not in the file exactly once")
	}
	for _, c := range []struct {
		Name string
		want int
	}{{"NULL", inLits}, {"size_t", 0}} {
		if n := p.Mentions(text, c.Name); n != c.want {
			return nil, p.Die("`%s` has %d mentions after the cut, expected %d", c.Name, n, c.want)
		}
	}
	after, err := ctext.LiteralSpans(p, text)
	if err != nil {
		return nil, err
	}
	var stillHolding []string
	for _, s := range after {
		if nuNull.Match(text[s[0]:s[1]]) {
			stillHolding = append(stillHolding, string(text[s[0]:s[1]]))
		}
	}
	if strings.Join(stillHolding, "\x00") != strings.Join(holding, "\x00") {
		return nil, p.Die("the literals holding `NULL` are not the ones they were")
	}
	intro := nuIntro.FindAllString(string(text), -1)
	if len(intro) != 1 || intro[0] != nuTypedef {
		j := strings.Join(intro, " / ")
		if j == "" {
			j = "nothing"
		}
		return nil, p.Die("`usize` is introduced by something other than exactly one typedef: %s -- it is "+
			"a TYPE NAME and not a static object, and every one of its uses is a type-name "+
			"position", j)
	}
	p.Sayf("the %d `NULL` in literals are the only `NULL` left, `size_t` is at zero, `usize` "+
		"is a typedef on line %d and %d runs of two blank lines, exactly as before",
		inLits, at+1, p.BlankRuns(text))
	return text, nil
}

// nuDirectives requires exactly n directives on the first n lines, each an
// `#include <...>` of a system header.
func nuDirectives(p ctext.Ph, text []byte, n int, msg string) error {
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
		if !nuInc.Match(lines[i]) {
			return p.Die("a directive is not an `#include <...>` of a system header, and no step may " +
				"add one")
		}
	}
	return nil
}
