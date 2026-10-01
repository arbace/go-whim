// Package cut holds the phase cutters: the tools a phase program calls to
// remove one named thing from the source.
//
// They share a rule that is GOALS.md's first: cut the entry point and let
// the compiler find the rest.  A cutter does not decide what becomes
// unreachable next -- that is the sweep's job -- and it REFUSES on a name it
// cannot find, because a silent miss leaves the thing in place and the report
// would say the work was done.
package cut

import (
	"bytes"

	"github.com/arbace/go-whim/crefactor/edit"
)

// DropRow removes options[]'s row for name, found by BRACE MATCHING rather
// than by counting lines: a row may run to several lines and ends at the brace
// that closes its initialiser.  It asks no guard: optfront drops every row the
// product has not on the seed, each global left at its static zero until the
// phase that removes its readers.
func DropRow(text []byte, name string) ([]byte, bool) {
	spans := indentedLit(text, `{"`+name+`",`)
	if len(spans) == 0 {
		return text, false
	}
	start := spans[0][0]
	b := edit.Blank(text)
	open := start + bytes.IndexByte(text[start:], '{')
	rowEnd := edit.Match(b, open)
	if rowEnd < 0 {
		return text, false
	}
	end := rowEnd + 1 // the same text, blanked above: its closing brace
	for end < len(text) && (text[end] == ' ' || text[end] == '\t' || text[end] == ',') {
		end++
	}
	out := make([]byte, 0, len(text))
	out = append(out, text[:start]...)
	out = append(out, text[end:]...)
	return out, true
}

// indentedLit is every place lit begins a line after its indentation --
// what `(?m)^[ \t]*` followed by the quoted literal matches -- as the span
// from the line's start to the literal's end, in order.  It searches for the
// literal and looks back to the line's start, where the regular expression,
// with no literal to start from, ran its machine over the whole file for each
// option: 38 s of phase 54's 44, measured.  Only the first occurrence on a
// line can have nothing but blanks before it, so the spans are the regular
// expression's matches, leftmost first and not overlapping.
func indentedLit(text []byte, lit string) [][2]int {
	var out [][2]int
	needle := []byte(lit)
	for from := 0; ; {
		i := bytes.Index(text[from:], needle)
		if i < 0 {
			return out
		}
		i += from
		ls := bytes.LastIndexByte(text[:i], '\n') + 1
		blank := true
		for _, c := range text[ls:i] {
			if c != ' ' && c != '\t' {
				blank = false
				break
			}
		}
		if blank {
			out = append(out, [2]int{ls, i + len(needle)})
		}
		from = i + len(needle)
	}
}
