package edit

// countBytes and replaceBytes are how every literal anchor in the pipeline is
// matched: E.Literal, Ph.Literal and Splice all come through here.  THE MATCH IS
// EXACT, and the alternative was measured rather than argued about.
//
// Normalize (norm.go) matches modulo whitespace, which is what an anchor
// written against macro-expansion residue -- `( strcmp(...)  == 0)` -- needs if
// the text is ever printed canonically.  Routed through here it did two things:
// it moved the canonical build's refusals from 148 of 163 phases to 147, and it
// changed the committed product in at least two places, because a widened match
// picks a DIFFERENT site where two differ only in spacing.  `&p_rtp )` was one;
// phase 71's wiped-fnum branch was another.
//
// So the match is EXACT FIRST.  Only when the exact literal occurs nowhere --
// an anchor copied from text an earlier phase wrote with aligned columns, now
// that every boundary is printed canonically -- does it fall back to matching
// modulo whitespace (matchCount, below, and CountAnchor for
// the phases' own closures).  Where the exact literal matches, nothing moved.
func countBytes(text []byte, s string) int {
	n, b := 0, []byte(s)
	for i := 0; i+len(b) <= len(text); {
		j := IndexFrom(text, b, i)
		if j < 0 {
			break
		}
		n++
		i = j + len(b)
	}
	return n
}

func replaceBytes(text []byte, old, new string) []byte {
	o, nw := []byte(old), []byte(new)
	i := IndexFrom(text, o, 0)
	if i < 0 {
		return text
	}
	Out := make([]byte, 0, len(text)-len(o)+len(nw))
	Out = append(Out, text[:i]...)
	Out = append(Out, nw...)
	return append(Out, text[i+len(o):]...)
}

func IndexFrom(text, needle []byte, from int) int {
	for i := from; i+len(needle) <= len(text); i++ {
		k := 0
		for k < len(needle) && text[i+k] == needle[k] {
			k++
		}
		if k == len(needle) {
			return i
		}
	}
	return -1
}

// THE CANONICAL FALLBACK.  Every boundary is printed canonically now, so an
// anchor copied from a text an earlier phase wrote with alignment --
// `{SIGWINCH,      "WINCH",    FALSE},` -- occurs nowhere, although the text it
// means is there.  matchCount keeps the exact match wherever the exact literal
// occurs at all, so nothing that matched before can move (the wider match picked
// a different site twice when it was tried first, above); ONLY when the exact
// literal occurs nowhere does it count modulo whitespace (norm.go).
// The count is still the assertion either way.
func matchCount(text []byte, s string) (n int, norm bool) {
	if n = countBytes(text, s); n > 0 {
		return n, false
	}
	if k := Count(text, []byte(s)); k > 0 {
		return k, true
	}
	return 0, false
}

// replaceMatched rewrites the first n occurrences of old, the way matchCount
// found them.
func replaceMatched(text []byte, old, new string, n int, norm bool) []byte {
	for i := 0; i < n; i++ {
		if norm {
			text = ReplaceFirst(text, []byte(old), []byte(new))
		} else {
			text = replaceBytes(text, old, new)
		}
	}
	return text
}
