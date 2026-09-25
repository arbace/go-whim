package vimtext

import (
	"fmt"
	"regexp"
	"strings"
)

// What more than one phase uses that knows vim; the rest is in
// crefactor/edit/shared.go.
//
// EVERY DECLARATION HERE WAS ONE PHASE'S, and is here because another phase
// reached it: when each phase became a package of its own (internal/phase/NNN), a helper
// two phases share stopped being either one's.  The phase it was written for is
// named above each.

// From phase 59.
// key and kex spell the two ways this file writes a special key as an integer.
// They are built rather than written Out because the C is a nest of escaped
// parentheses and the same shape recurs: KEY('k','B') is S-Tab, KEX(KE_WILD) is
// the wildcard trigger.
func Key(a, b string) string {
	return fmt.Sprintf(`\(-\(\('%s'\) \+ \(\(int\)\('%s'\) << 8\)\)\)`, a, b)
}

// From phase 71.
var _ = BwdWalk

// From phase 119.
// IsPrototypeLine is the phase's `DECL`, whose Python spells the exclusion as a
// NEGATIVE LOOKAHEAD -- `^(?!static |typedef |static_assert)...`.  RE2 has none,
// and the three prefixes are tested instead, which is exact here for the reason
// the byte tests elsewhere are: what is excluded is at the START of the line and
// is consumed by nothing.
func IsPrototypeLine(l string) bool {
	if strings.HasPrefix(l, "static ") || strings.HasPrefix(l, "typedef ") ||
		strings.HasPrefix(l, "static_assert") {
		return false
	}
	return declRe.MatchString(l)
}

// From phase 119.
// JoinOrNone is Python's `' / '.join(xs) or 'none'`.
func JoinOrNone(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, " / ")
}

// From phase 119.
var (
	DirectiveRe     = regexp.MustCompile(`^ *# *`)
	SystemIncludeRe = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)
	declRe          = regexp.MustCompile(`^[A-Za-z_][\w *]*\**\w+\([^;]*\);$`)
	DeclNameRe      = regexp.MustCompile(`^.*?\**(\w+)\(.*$`)
)

// From phase 126.
// PyList is Python's str() of a list of strings, which the refusals quote.
func PyList(s []string) string {
	Out := make([]string, len(s))
	for i, v := range s {
		Out[i] = "'" + v + "'"
	}
	return "[" + strings.Join(Out, ", ") + "]"
}

// From phase 126.
// PyReprMultiline is Python's %r of a string that may hold a newline.
func PyReprMultiline(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\n", "\\n") + "'"
}

// From phase 127.
// SpliceLines is Python's `lines[a:b] = rows`.
func SpliceLines(lines []string, a, b int, rows []string) []string {
	Out := make([]string, 0, len(lines)-(b-a)+len(rows))
	Out = append(Out, lines[:a]...)
	Out = append(Out, rows...)
	return append(Out, lines[b:]...)
}

// From phase 127.
func LineIndex(lines []string, s string) int {
	for i, l := range lines {
		if l == s {
			return i
		}
	}
	// Exact first; only when no line is exactly s, a line that differs from it
	// in spacing alone -- every boundary is printed canonically, so a line an
	// earlier phase wrote with aligned columns is spelled with single spaces.
	want := strings.Join(strings.Fields(s), " ")
	for i, l := range lines {
		if strings.Join(strings.Fields(l), " ") == want {
			return i
		}
	}
	return -1
}

// From phase 127.
var (
	FnHeadRe = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\(`)
)

// From phase 071.
const (
	FwdWalk = `for ((buf) = firstbuf; (buf) != NULL; (buf) = (buf)->b_next)`
	BwdWalk = `for ((buf) = lastbuf; (buf) != NULL; (buf) = (buf)->b_prev)`
)
