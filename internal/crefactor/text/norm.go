package text

import (
	"bytes"
	"regexp"
	"strings"
)

// WHITESPACE-INSENSITIVE LITERAL MATCHING.
//
// A phase's anchor is a fragment of the tree it was written against, and the
// tree it was written against is macro-expansion residue: phase 1 looks for
//
//	vimruntime = ( strcmp((char *)(name), (char *)("VIMRUNTIME"))  == 0);
//
// with a space after the `(` and two before the `==`, because that is what
// slim-vim.c says.  Canonical C says the same thing with one space, and the
// literal stops occurring -- which is 148 of the 163 phases, measured.
//
// So a literal is matched MODULO WHITESPACE.  Both sides are normalised the
// same way: every run of whitespace is dropped, except between two word
// characters, where one space stands for it.  `sizeof ("help")` and
// `sizeof("help")` are then the same anchor; `int x` and `intx` are not.
//
// WHAT IS NOT NORMALISED: the inside of a string or character literal, where a
// space is content and not layout.  `"Vim: Caught deadly signal %s\r\n"` keeps
// every one of its spaces, and an anchor that names it still has to say it
// exactly.
//
// THE MATCH IS STILL COUNTED.  Normalising widens what a literal can match, so
// two sites that differ only in spacing now look alike -- and a phase that
// declared "exactly once" then REFUSES with a count of two rather than
// rewriting the wrong one.  Widening the match does not widen the assertion.

// A Norm is a whitespace-normalised view of some source, and the offsets that
// map a match in it back to the source.
type Norm struct {
	Text []byte // the normalised bytes
	src  []byte // the source it came from
	beg  []int  // for each normalised byte, where it starts in the source
	end  []int  // and where it ends
}

// Normalize builds the view.
func Normalize(src []byte) *Norm {
	n := &Norm{
		src:  src,
		Text: make([]byte, 0, len(src)),
		beg:  make([]int, 0, len(src)),
		end:  make([]int, 0, len(src)),
	}
	put := func(b byte, from, to int) {
		n.Text = append(n.Text, b)
		n.beg = append(n.beg, from)
		n.end = append(n.end, to)
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			j := i
			for j < len(src) && isSpace(src[j]) {
				j++
			}
			// The run stands for a space only where it separates two words.
			if len(n.Text) > 0 && isWord(n.Text[len(n.Text)-1]) && j < len(src) && isWord(src[j]) {
				put(' ', i, j)
			}
			i = j
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(src) {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == c {
					j++
					break
				}
				j++
			}
			for k := i; k < j && k < len(src); k++ {
				put(src[k], k, k+1)
			}
			i = j
		default:
			put(c, i, i+1)
			i++
		}
	}
	return n
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func isWord(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// Spans is every place a literal occurs, as source offsets, in order and not
// overlapping.
//
// WHITESPACE AT THE NEEDLE'S EDGE IS PART OF THE MATCH.  A phase that deletes
// ` || p == (char_u *)&p_cdpath` means the space before it too, and a span that
// stopped at the first `|` would leave the line with two spaces where it had
// one.  Measured, it left exactly that: `&p_rtp )`.  So a needle that begins or
// ends with whitespace requires whitespace there, and the span covers it.
func (n *Norm) Spans(needle []byte) [][2]int {
	want := Normalize(needle).Text
	if len(want) == 0 {
		return nil
	}
	leading := len(needle) > 0 && isSpace(needle[0])
	trailing := len(needle) > 0 && isSpace(needle[len(needle)-1])
	var out [][2]int
	for i := 0; i+len(want) <= len(n.Text); {
		j := bytes.Index(n.Text[i:], want)
		if j < 0 {
			break
		}
		a := i + j
		b := a + len(want)
		lo, hi := n.beg[a], n.end[b-1]
		ok := true
		if leading {
			k := lo
			for k > 0 && isSpace(n.src[k-1]) {
				k--
			}
			if k == lo {
				ok = false // the needle wants a space here and there is none
			}
			lo = k
		}
		if trailing {
			k := hi
			for k < len(n.src) && isSpace(n.src[k]) {
				k++
			}
			if k == hi {
				ok = false
			}
			hi = k
		}
		if ok {
			out = append(out, [2]int{lo, hi})
		}
		i = b
	}
	return out
}

// Count is how many times a literal occurs, modulo whitespace.
func Count(src, needle []byte) int { return len(Normalize(src).Spans(needle)) }

// ReplaceFirst rewrites the first occurrence of old with new, modulo
// whitespace, and returns the source unchanged when there is none.
func ReplaceFirst(src, old, new []byte) []byte {
	spans := Normalize(src).Spans(old)
	if len(spans) == 0 {
		return src
	}
	a, b := spans[0][0], spans[0][1]
	// A span is the matched TOKENS: the whitespace around them is the text's,
	// and replacing it too would take the blank line after a block with it.
	for a < b && isSpace(src[a]) {
		a++
	}
	for b > a && isSpace(src[b-1]) {
		b--
	}
	out := make([]byte, 0, len(src)-(b-a)+len(new))
	out = append(out, src[:a]...)
	out = append(out, new...)
	return append(out, src[b:]...)
}

// Contains reports whether a literal occurs, modulo whitespace.
func Contains(src, needle []byte) bool { return len(Normalize(src).Spans(needle)) > 0 }

// Index is where a literal first occurs, as a source offset, or -1.
func Index(src, needle []byte) int {
	spans := Normalize(src).Spans(needle)
	if len(spans) == 0 {
		return -1
	}
	return spans[0][0]
}

// IndexFrom is Index, starting the search at a source offset.
func IndexFrom(src, needle []byte, from int) int {
	if from < 0 || from > len(src) {
		return -1
	}
	i := Index(src[from:], needle)
	if i < 0 {
		return -1
	}
	return from + i
}

// ReplaceN rewrites the first n occurrences, modulo whitespace, or every one of
// them when n is negative.
func ReplaceN(src, old, new []byte, n int) []byte {
	spans := Normalize(src).Spans(old)
	if len(spans) == 0 {
		return src
	}
	if n >= 0 && n < len(spans) {
		spans = spans[:n]
	}
	out := make([]byte, 0, len(src))
	last := 0
	for _, s := range spans {
		out = append(out, src[last:s[0]]...)
		out = append(out, new...)
		last = s[1]
	}
	return append(out, src[last:]...)
}

// ReplaceAll rewrites every occurrence, modulo whitespace.
func ReplaceAll(src, old, new []byte) []byte { return ReplaceN(src, old, new, -1) }

// ANCHORS, EXACT FIRST.  Every boundary is printed canonically, so an anchor
// copied from text an earlier phase wrote in another layout -- aligned columns,
// two declarations with no blank line between -- occurs nowhere, although the
// text it means is there.  These count and replace EXACTLY wherever the exact
// literal occurs at all, so nothing that matched before can move, and only when
// it occurs nowhere do they fall back to matching modulo whitespace.  The
// caller's count is still the assertion.

// CountAnchor is how many times old occurs in text: exactly, or else modulo
// whitespace.
func CountAnchor(text, old string) int { return CountAnchorB([]byte(text), old) }

// CountAnchorB is CountAnchor on bytes.
func CountAnchorB(text []byte, old string) int {
	if n := bytes.Count(text, []byte(old)); n > 0 {
		return n
	}
	return Count(text, []byte(old))
}

// ReplaceAnchor replaces the first n occurrences of old (all of them when n is
// negative), found the way CountAnchor finds them.
func ReplaceAnchor(text, old, new string, n int) string {
	return string(ReplaceAnchorB([]byte(text), old, []byte(new), n))
}

// ReplaceAnchorB is ReplaceAnchor on bytes.
func ReplaceAnchorB(text []byte, old string, new []byte, n int) []byte {
	if bytes.Contains(text, []byte(old)) {
		return bytes.Replace(text, []byte(old), new, n)
	}
	k := Count(text, []byte(old))
	if n < 0 || n > k {
		n = k
	}
	for i := 0; i < n; i++ {
		text = ReplaceFirst(text, []byte(old), new)
	}
	return text
}

// Line is the anchor most patterns are: one or more whole C lines, each after
// its indentation, spelled as the C is.  Line("maketitle();") is the regular
// expression `(?m)^[ \t]*maketitle\(\);\n` -- the one phases wrote by hand --
// and Line("if (x)", "{") matches the two lines in a row.  On canonical text
// the indentation is the only thing a line's spelling does not fix.
func Line(lines ...string) string {
	var b strings.Builder
	b.WriteString(`(?m)`)
	for i, l := range lines {
		if i == 0 {
			b.WriteString(`^`)
		}
		b.WriteString(`[ \t]*`)
		b.WriteString(regexp.QuoteMeta(l))
		b.WriteString(`\n`)
	}
	return b.String()
}

// Head is one whole C line after its indentation, up to the end of the line
// and not past it: the head of a statement a fold takes, `if (x)` of
// `if (x)\n{...}`.  Head("if (ind)") is `(?m)^[ \t]*if \(ind\)$`.
func Head(line string) string {
	return `(?m)^[ \t]*` + regexp.QuoteMeta(line) + `$`
}
