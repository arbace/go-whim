package edit

import (
	"bytes"
	"regexp"
	"regexp/syntax"
	"sync"
	"unicode/utf8"
)

// The counted acts' regexps, on the lines that can hold a match.
//
// A pattern of the edits is, almost always, a line or two of C around a word:
// `(?m)^[ \t]*if \(p_xyz\) \{\n`.  Go's regexp skips ahead by a LITERAL PREFIX
// only, and these begin with an anchor and a class, so every one of them ran
// its machine over the whole multi-megabyte text, once to count and again to
// rewrite.  That was most of the in-order build's 128 s of regexp.
//
// What every match must hold is read off the pattern's syntax tree instead: a
// literal no match lacks (lit), and the most newlines a match can span (k).
// A match then lies within k lines of an occurrence of lit, so the text is
// scanned only on WINDOWS: each occurrence's line, k+1 lines above and below
// it, merged where they meet.  The extra line on each side is the context: a
// match's `^`, `$` and `\b` at its ends see the same bytes in the window as in
// the text, so the window finds a match where the text does, the same one,
// and nothing between the windows can match.  So the matches are FindAll's,
// in order, offset back -- which TestScoped holds them to on the input and
// the product.
//
// Where the tree promises nothing -- no literal of two bytes or more, a
// newline under a star, a `\A` or `\z` (non-(?m) `^` and `$`), a case-folded
// literal -- the pattern runs on the whole text, as it did.

type scope struct {
	lit      []byte // a literal every match holds; nil: the whole text
	k        int    // the most newlines a match can span
	nonEmpty bool   // no match is empty
}

var scopes sync.Map // the pattern's source -> scope

func scopeOf(re *regexp.Regexp) scope {
	src := re.String()
	if s, ok := scopes.Load(src); ok {
		return s.(scope)
	}
	s := scope{}
	if t, err := syntax.Parse(src, syntax.Perl); err == nil {
		if k, ok := maxNewlines(t); ok {
			if lit := requiredLit(t); len(lit) >= 2 {
				s = scope{lit: []byte(lit), k: k}
			}
		}
		s.nonEmpty = minLen(t) > 0
	}
	scopes.Store(src, s)
	return s
}

const unbounded = 1 << 20

// maxNewlines is the most newlines a match of t can hold, and false where it
// is unbounded or the pattern asserts the start or the end of the text.
func maxNewlines(t *syntax.Regexp) (int, bool) {
	switch t.Op {
	case syntax.OpBeginText, syntax.OpEndText, syntax.OpAnyChar:
		return 0, false
	case syntax.OpLiteral:
		n := 0
		for _, r := range t.Rune {
			if r == '\n' {
				n++
			}
		}
		return n, true
	case syntax.OpCharClass:
		for i := 0; i+1 < len(t.Rune); i += 2 {
			if t.Rune[i] <= '\n' && '\n' <= t.Rune[i+1] {
				return 1, true
			}
		}
		return 0, true
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		n, ok := maxNewlines(t.Sub[0])
		if !ok {
			return 0, false
		}
		switch {
		case n == 0:
			return 0, true
		case t.Op == syntax.OpQuest:
			return n, true
		case t.Op == syntax.OpRepeat && t.Max >= 0 && t.Max*n < unbounded:
			return t.Max * n, true
		}
		return 0, false
	case syntax.OpConcat, syntax.OpAlternate:
		total := 0
		for _, s := range t.Sub {
			n, ok := maxNewlines(s)
			if !ok {
				return 0, false
			}
			if t.Op == syntax.OpConcat {
				total += n
			} else {
				total = max(total, n)
			}
		}
		return total, total < unbounded
	case syntax.OpCapture:
		return maxNewlines(t.Sub[0])
	}
	// The empty-width assertions that are a line's or a word's, and the
	// classes that hold no newline.
	return 0, true
}

// minLen is the fewest bytes a match of t can hold.
func minLen(t *syntax.Regexp) int {
	switch t.Op {
	case syntax.OpLiteral:
		n := 0
		for _, r := range t.Rune {
			n += utf8.RuneLen(r)
		}
		return n
	case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return 1
	case syntax.OpCapture, syntax.OpPlus:
		return minLen(t.Sub[0])
	case syntax.OpRepeat:
		return t.Min * minLen(t.Sub[0])
	case syntax.OpConcat:
		n := 0
		for _, s := range t.Sub {
			n += minLen(s)
		}
		return n
	case syntax.OpAlternate:
		n := -1
		for _, s := range t.Sub {
			if m := minLen(s); n < 0 || m < n {
				n = m
			}
		}
		return max(n, 0)
	}
	return 0 // the empty-width assertions, a star, a question mark
}

// requiredLit is the longest literal every match of t contains: a run of
// literal parts of a concatenation, or one inside a part every match passes
// through once at least.
func requiredLit(t *syntax.Regexp) string {
	switch t.Op {
	case syntax.OpLiteral:
		if t.Flags&syntax.FoldCase != 0 {
			return ""
		}
		return string(t.Rune)
	case syntax.OpCapture, syntax.OpPlus:
		return requiredLit(t.Sub[0])
	case syntax.OpRepeat:
		if t.Min >= 1 {
			return requiredLit(t.Sub[0])
		}
	case syntax.OpConcat:
		best, run := "", ""
		for _, s := range t.Sub {
			if s.Op == syntax.OpLiteral && s.Flags&syntax.FoldCase == 0 {
				run += string(s.Rune)
				if len(run) > len(best) {
					best = run
				}
				continue
			}
			run = ""
			if l := requiredLit(s); len(l) > len(best) {
				best = l
			}
		}
		return best
	}
	return ""
}

// windows is the merged spans of text, each starting at a line's start, that
// hold every match of a pattern of scope s.
func windows(text []byte, s scope) [][2]int {
	var at [][2]int
	for pos := 0; pos < len(text); {
		rel := bytes.Index(text[pos:], s.lit)
		if rel < 0 {
			break
		}
		i := pos + rel
		at = append(at, [2]int{i, i + len(s.lit)})
		pos = i + 1
	}
	return windowsAround(text, at, s.k)
}

// windowsAround is the merged spans of text around the spans at, in order:
// each span's lines, and k+1 lines above and below them.
func windowsAround(text []byte, at [][2]int, k int) [][2]int {
	var out [][2]int
	up := func(a, n int) int { // the start of the line n lines above a's
		a = bytes.LastIndexByte(text[:a], '\n') + 1
		for ; n > 0 && a > 0; n-- {
			a = bytes.LastIndexByte(text[:a-1], '\n') + 1
		}
		return a
	}
	down := func(z, n int) int { // the end of the line n lines below z's, its newline included
		for ; n >= 0; n-- {
			e := bytes.IndexByte(text[z:], '\n')
			if e < 0 {
				return len(text)
			}
			z += e + 1
		}
		return z
	}
	for _, sp := range at {
		a := up(sp[0], k+1)
		z := down(max(sp[0], sp[1]-1), k+1)
		if n := len(out); n > 0 && a <= out[n-1][1] {
			out[n-1][1] = max(out[n-1][1], z)
		} else {
			out = append(out, [2]int{a, z})
		}
	}
	return out
}

// AllSubmatchIndexAround is re.FindAllSubmatchIndex(text, -1) for a re
// every match of which holds one of the spans at (in order) and at most k
// newlines: a caller that knows a cheaper mark of a match than a literal --
// an empty block, `{` and `}` on the lines after a head -- scans the windows
// around its marks alone.  Its caller's test holds it to the regexp.
func AllSubmatchIndexAround(re *regexp.Regexp, text []byte, at [][2]int, k int) [][]int {
	var out [][]int
	for _, w := range windowsAround(text, at, k) {
		for _, m := range re.FindAllSubmatchIndex(text[w[0]:w[1]], -1) {
			for j := range m {
				if m[j] >= 0 {
					m[j] += w[0]
				}
			}
			out = append(out, m)
		}
	}
	return out
}

// AllSubmatchIndex is re.FindAllSubmatchIndex(text, -1), on the windows that
// can hold a match.
func AllSubmatchIndex(re *regexp.Regexp, text []byte) [][]int {
	s := scopeOf(re)
	if s.lit == nil {
		return re.FindAllSubmatchIndex(text, -1)
	}
	var out [][]int
	for _, w := range windows(text, s) {
		for _, m := range re.FindAllSubmatchIndex(text[w[0]:w[1]], -1) {
			for j := range m {
				if m[j] >= 0 {
					m[j] += w[0]
				}
			}
			out = append(out, m)
		}
	}
	return out
}

// AllIndex is re.FindAllIndex(text, -1), on the windows.
func AllIndex(re *regexp.Regexp, text []byte) [][]int { return AllIndexN(re, text, -1) }

// AllIndexN is re.FindAllIndex(text, n), on the windows.
func AllIndexN(re *regexp.Regexp, text []byte, n int) [][]int {
	s := scopeOf(re)
	if s.lit == nil {
		return re.FindAllIndex(text, n)
	}
	var out [][]int
	for _, w := range windows(text, s) {
		for _, m := range re.FindAllIndex(text[w[0]:w[1]], -1) {
			if n >= 0 && len(out) >= n {
				return out
			}
			out = append(out, []int{m[0] + w[0], m[1] + w[0]})
		}
	}
	return out
}

// CountMatches is len(re.FindAll(text, -1)), on the windows.
func CountMatches(re *regexp.Regexp, text []byte) int {
	if scopeOf(re).lit == nil {
		return len(re.FindAll(text, -1))
	}
	return len(AllIndexN(re, text, -1))
}

// FirstIndex is re.FindIndex(text), on the windows.
func FirstIndex(re *regexp.Regexp, text []byte) []int {
	s := scopeOf(re)
	if s.lit == nil {
		return re.FindIndex(text)
	}
	for _, w := range windows(text, s) {
		if m := re.FindIndex(text[w[0]:w[1]]); m != nil {
			return []int{m[0] + w[0], m[1] + w[0]}
		}
	}
	return nil
}

// FirstSubmatchIndex is re.FindSubmatchIndex(text), on the windows.
func FirstSubmatchIndex(re *regexp.Regexp, text []byte) []int {
	s := scopeOf(re)
	if s.lit == nil {
		return re.FindSubmatchIndex(text)
	}
	for _, w := range windows(text, s) {
		if m := re.FindSubmatchIndex(text[w[0]:w[1]]); m != nil {
			for j := range m {
				if m[j] >= 0 {
					m[j] += w[0]
				}
			}
			return m
		}
	}
	return nil
}

// ReplaceAllCounted is re.ReplaceAll(text, repl) and the number of matches
// it replaced, the matches found once, on the windows.
func ReplaceAllCounted(re *regexp.Regexp, text, repl []byte) ([]byte, int) {
	return replaceAllCounted(re, text, repl, false)
}

// ReplaceAllLiteralCounted is re.ReplaceAllLiteral(text, repl) and the
// number of matches it replaced, on the windows.
func ReplaceAllLiteralCounted(re *regexp.Regexp, text, repl []byte) ([]byte, int) {
	return replaceAllCounted(re, text, repl, true)
}

func replaceAllCounted(re *regexp.Regexp, text, repl []byte, literal bool) ([]byte, int) {
	s := scopeOf(re)
	if s.lit == nil && !s.nonEmpty {
		n := len(re.FindAll(text, -1))
		if literal {
			return re.ReplaceAllLiteral(text, repl), n
		}
		return re.ReplaceAll(text, repl), n
	}
	// Where no match is empty, ReplaceAll replaces exactly the matches
	// FindAll gives (they differ only on an empty match beside another), so
	// they are found once and expanded here.
	ms := AllSubmatchIndex(re, text)
	if len(ms) == 0 {
		return append([]byte(nil), text...), 0
	}
	out := make([]byte, 0, len(text))
	last := 0
	for _, m := range ms {
		out = append(out, text[last:m[0]]...)
		if literal {
			out = append(out, repl...)
		} else {
			out = re.Expand(out, repl, text, m)
		}
		last = m[1]
	}
	return append(out, text[last:]...), len(ms)
}
