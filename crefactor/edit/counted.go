package edit

import (
	"fmt"
	"regexp"
)

// The counted acts, once.  Every verb that rewrites a literal, a pattern or
// one definition's text comes through here -- E's (driver.go), Ph's
// (phdriver.go) and the cutters' own driver in internal/cut -- and differs from
// the others only in how it reports: which tag, which column, which words
// around the reason.  The reason is the error these return, worded once, so a
// refusal says the same thing whichever driver it came through.
//
// A COUNT IS THE ASSERTION.  Each refuses unless the thing occurs exactly n
// times, because these run on a tree every earlier edit has touched, and an
// anchor that has moved means the edit is about to rewrite something other
// than what it was written against.

// ReplaceLiteral replaces the n occurrences of old with new, refusing on any
// other count.  The match is exact first, and modulo whitespace only when the
// exact literal occurs nowhere (anchor.go).
func ReplaceLiteral(t []byte, old, new string, n int) ([]byte, error) {
	k, norm := matchCount(t, old)
	if k != n {
		return nil, fmt.Errorf("occurs %d times, expected %d", k, n)
	}
	return replaceMatched(t, old, new, n, norm), nil
}

// ReplacePattern rewrites the pattern's n matches with repl, which may expand
// $1, refusing on any other count.
func ReplacePattern(t []byte, pattern, repl string, n int) ([]byte, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	if k := len(re.FindAll(t, -1)); k != n {
		return nil, fmt.Errorf("matched %d times, expected %d", k, n)
	}
	return re.ReplaceAll(t, []byte(repl)), nil
}

// InDefinition applies f to ONE file-scope definition's text and splices the
// result back, so a pattern that would match elsewhere in the file cannot.  It
// reports whether the definition is there at all; what the caller says when
// it is not is the caller's.
func InDefinition(t []byte, name string, f func([]byte) ([]byte, error)) (out []byte, found bool, err error) {
	a, z, ok := FindDefinition(t, Blank(t), name)
	if !ok {
		return nil, false, nil
	}
	seg, err := f(t[a:z])
	if err != nil {
		return nil, true, err
	}
	out = make([]byte, 0, len(t)-(z-a)+len(seg))
	out = append(out, t[:a]...)
	out = append(out, seg...)
	return append(out, t[z:]...), true, nil
}
