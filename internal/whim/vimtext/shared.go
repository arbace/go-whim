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

// From phase 88.
// w88Lines is Python's splitlines(keepends=True): each line with its newline.
func W88Lines(s string) []string {
	var Out []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			Out = append(Out, s)
			break
		}
		Out = append(Out, s[:i+1])
		s = s[i+1:]
	}
	return Out
}

// From phase 119.
// w119IsDecl is the phase's `DECL`, whose Python spells the exclusion as a
// NEGATIVE LOOKAHEAD -- `^(?!static |typedef |static_assert)...`.  RE2 has none,
// and the three prefixes are tested instead, which is exact here for the reason
// the byte tests elsewhere are: what is excluded is at the START of the line and
// is consumed by nothing.
func W119IsDecl(l string) bool {
	if strings.HasPrefix(l, "static ") || strings.HasPrefix(l, "typedef ") ||
		strings.HasPrefix(l, "static_assert") {
		return false
	}
	return W119DeclRe.MatchString(l)
}

// From phase 119.
// w119Or is Python's `' / '.join(xs) or 'none'`.
func W119Or(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, " / ")
}

// From phase 119.
var (
	W119Dir     = regexp.MustCompile(`^ *# *`)
	W119Inc     = regexp.MustCompile(`^#include <[A-Za-z0-9_/.]+>$`)
	W119DeclRe  = regexp.MustCompile(`^[A-Za-z_][\w *]*\**\w+\([^;]*\);$`)
	W119Reraise = regexp.MustCompile(`^(\s*)kill\(getpid\(\), (\w+)\);$`)
	W119ProtoGP = regexp.MustCompile(`^static [\w *]+mch_get_pid\(.*\);$`)
	W119Write   = regexp.MustCompile(`^\s*long_to_char\(mch_get_pid\(\), (\w+)->b0_pid\);$`)
	W119Field   = regexp.MustCompile(`^\s*char_u\s+b0_pid\[\d+\];$`)
	W119Name    = regexp.MustCompile(`^.*?\**(\w+)\(.*$`)
	W119RetType = regexp.MustCompile(`^    [\w *]+$`)
)

// From phase 126.
// w126PyList is Python's str() of a list of strings, which the refusals quote.
func W126PyList(s []string) string {
	Out := make([]string, len(s))
	for i, v := range s {
		Out[i] = "'" + v + "'"
	}
	return "[" + strings.Join(Out, ", ") + "]"
}

// From phase 126.
// w126PyRepr is Python's %r of a string that may hold a newline.
func W126PyRepr(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\n", "\\n") + "'"
}

// From phase 127.
// w127NotDecl: `return OK;` has the shape of a declaration and is not one.  The
// first word of a declaration is a type, never one of these.
var W127NotDecl = []string{"return", "goto", "break", "continue", "case", "else", "do"}

// From phase 127.
// w127Splice is Python's `lines[a:b] = rows`.
func W127Splice(lines []string, a, b int, rows []string) []string {
	Out := make([]string, 0, len(lines)-(b-a)+len(rows))
	Out = append(Out, lines[:a]...)
	Out = append(Out, rows...)
	return append(Out, lines[b:]...)
}

// From phase 127.
func W127Index(lines []string, s string) int {
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
	W127Decl     = regexp.MustCompile(`^\s+(?:static\s+)?[A-Za-z_][A-Za-z0-9_]*(?:\s+\*?[A-Za-z_][A-Za-z0-9_]*)*\s+\*?([A-Za-z_][A-Za-z0-9_]*)\s*(?:=[^;]*)?;$`)
	W127FnHead   = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\(`)
	W127MlFlags  = regexp.MustCompile(`ml_flags \|=`)
	W127DlText   = regexp.MustCompile(`\.dl_text\s*=[^=]`)
	W127Interior = regexp.MustCompile(`\(char_u? \*\)dp[a-z_]* *\+`)
)

// From phase 071.
const (
	FwdWalk = `for ((buf) = firstbuf; (buf) != NULL; (buf) = (buf)->b_next)`
	BwdWalk = `for ((buf) = lastbuf; (buf) != NULL; (buf) = (buf)->b_prev)`
)

// From phase 080.
var (
	W80Table  = regexp.MustCompile(`(?ms)^static struct cmdname cmdnames\[\] =\n\{\n(.*?)^\};\n`)
	W80RowRe  = regexp.MustCompile(`^    \[CMD_(\w+)\] = \{\(char_u \*\)"([^"]*)", sizeof\("([^"]*)"\) - 1, *(\w+) *, \(long_u\)\(.*\), ADDR_\w+\},$`)
	W80EnumRe = regexp.MustCompile(`(?ms)^enum CMD_index\n\{\n(.*?)^    CMD_SIZE,\n\};\n`)
	W80IdRe   = regexp.MustCompile(`(?m)^    CMD_(\w+),$`)
	W80Idx1   = regexp.MustCompile(`(?s)static const unsigned short cmdidxs1\[26\] =\n\{\n(.*?)\};`)
	W80Idx2   = regexp.MustCompile(`(?s)static const unsigned char cmdidxs2\[26\]\[26\] =\n\{\n(.*?)\n\};`)
	W80Count  = regexp.MustCompile(`static const int command_count = (\d+);`)
	W80Chars  = regexp.MustCompile(`vim_strchr\(\(char_u \*\)"([^"]*)", \*p\) != NULL\)\n`)
	W80Banner = regexp.MustCompile(`(?ms)^static const unsigned short cmdidxs1\[26\] =\n.*?^static const int command_count = \d+;\n`)
	W80Num    = regexp.MustCompile(`\d+`)
	W80Label  = regexp.MustCompile(`^([ \t]*)(case \w+:|default:)$`)
	W80Case   = regexp.MustCompile(`^[ \t]*case (\w+):$`)
	W80Fall   = regexp.MustCompile(`^[ \t]*(break;|goto \w+;|return\b.*;|\{)$`)
	W80Word   = regexp.MustCompile(`^[A-Za-z]+`)
	W80Skip   = regexp.MustCompile(`\b(ea\.|eap->)skip\b`)
	W80Vim9   = regexp.MustCompile(`\bvim9\b`)
)
