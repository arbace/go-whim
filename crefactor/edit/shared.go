package edit

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// What more than one phase uses, and knows nothing of the program it edits.
//
// EVERY DECLARATION HERE WAS ONE PHASE'S, and is here because another phase
// reached it: when each phase became a package of its own (internal/phase/NNN), a helper
// two phases share stopped being either one's.  The phase it was written for is
// named above each.  What the phases share that DOES know vim -- the buffer
// walks, the command table's shapes, the swap file's fields -- is in
// internal/whim/vimtext.

// From phase 4d.
// bindsToWalk names a `break` or `continue` in the Body that is not inside a
// loop or switch of the Body's own, or "" if there is none.
func BindsToWalk(raw []byte) string {
	rb := Blank(raw)
	type span struct{ a, z int }
	var spans []span
	for _, mm := range EnclosingLoop.FindAllIndex(rb, -1) {
		j := IndexFrom(rb, []byte("{"), mm[1])
		if j >= 0 {
			if k := Match(rb, j); k > 0 {
				spans = append(spans, span{j, k})
			}
		}
	}
	for _, kw := range []string{"break", "continue"} {
		re := regexp.MustCompile(`\b` + kw + `\b[ \t]*;`)
		for _, mm := range re.FindAllIndex(rb, -1) {
			inside := false
			for _, s := range spans {
				if s.a < mm[0] && mm[0] < s.z {
					inside = true
					break
				}
			}
			if !inside {
				return kw
			}
		}
	}
	return ""
}

// From phase 5b.
func CountNewlines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}

// From phase 5b.
func JoinInts(v []int) string {
	// sorted ascending, as the Python prints them
	for i := 0; i < len(v); i++ {
		for j := i + 1; j < len(v); j++ {
			if v[j] < v[i] {
				v[i], v[j] = v[j], v[i]
			}
		}
	}
	Out := ""
	for i, n := range v {
		if i > 0 {
			Out += " "
		}
		Out += fmt.Sprint(n)
	}
	return Out
}

// From phase 25.
// sortedKeys is GENERIC in the map's value, because three of us wrote one each:
// whim25 over map[string]string, record 88's whim88 over map[string]int and whim43 over
// map[string]bool.  Ranging a Go map yields a different order every run, and
// every one of those three was written because a REPORT line is built from the
// keys -- which is the difference a boundary cannot see and editcmp can.
func SortedKeys[V any](m map[string]V) []string {
	Out := make([]string, 0, len(m))
	for k := range m {
		Out = append(Out, k)
	}
	sort.Strings(Out)
	return Out
}

// From phase 26.
// contains is GENERIC because two sessions wrote one each and they collided:
// whim26's over []string and whim41's over []int.  One generic definition is
// the merge, and neither call site changed.
func Contains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// From phase 26.
// first is GENERIC for the same reason contains is: whim26 slices []string and
// whim53 slices []int, and one definition is cheaper than two names.
func First[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// From phase 26.
func IndexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// From phase 29.
// zHead is the heredocs' `old[:n]` in a refusal.
func CoreHead(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// From phase 37.
// zCalls counts occurrences of `name` as a CALL, which is what a header
// provides.  NOT `\bname\b`: "isprint" is also the name of an option and lives
// in a string literal, so a word count says 1 on a file that calls it nowhere.
//
// The Python spells it `(?<![\w])name\s*\(`, and RE2 has no lookbehind, so the
// preceding byte is tested instead -- which is CLAUDE.md's own answer for
// `nobackup` and `noconv`, and is exact rather than an approximation of one.
func CoreCalls(t []byte, name string) int {
	n := 0
	for _, m := range CoreCallRe(name).FindAllIndex(t, -1) {
		if m[0] > 0 && IsWordByte(t[m[0]-1]) {
			continue
		}
		n++
	}
	return n
}

// From phase 37.
func CoreCallRe(name string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(name) + `\s*\(`)
}

// From record 99.
func Uniq(in []string) []string {
	seen := map[string]bool{}
	var Out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			Out = append(Out, s)
		}
	}
	return Out
}

// From phase 51a.
func ContainsStr(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// PureCond reports whether a condition only reads -- no call, no assignment,
// no ++ or --.  Casts and sizeof read nothing a call could change; they are
// allowed.  (From phase 62.)
func PureCond(cond string) bool {
	c := dropCasts(cond)
	c = regexp.MustCompile(`\bsizeof\s*\(`).ReplaceAllString(c, "(")
	return !callToken.MatchString(c) && !writeToken.MatchString(c)
}

// castRe is what a cast looks like: a parenthesised type name.  So does the
// argument list of a call with one bare-name argument, `f(a)`.
var castRe = regexp.MustCompile(`\((?:const\s+)?(?:unsigned\s+)?[A-Za-z_]\w*\s*\**\s*\)`)

// dropCasts removes every cast from a condition.  A `(name)` is an argument
// list, not a cast, when what precedes it (spaces aside) ends a name or an
// expression -- a word character, `)` or `]` -- and it is kept, so that
// PureCond sees the call.
func dropCasts(cond string) string {
	var b strings.Builder
	last := 0
	for _, m := range castRe.FindAllStringIndex(cond, -1) {
		k := m[0]
		for k > 0 && (cond[k-1] == ' ' || cond[k-1] == '\t') {
			k--
		}
		if k > 0 && (IsWordByte(cond[k-1]) || cond[k-1] == ')' || cond[k-1] == ']') {
			continue
		}
		b.WriteString(cond[last:m[0]])
		last = m[1]
	}
	b.WriteString(cond[last:])
	return b.String()
}

// From phase 4d.
var EnclosingLoop = regexp.MustCompile(`\b(for|while|switch|do)\b`)

// From phase 62: EmptyGuardedBlock is an `if`, `else if` or `else` whose
// block is empty (its indentation, head and the block's indentation as
// groups); ElseLine is a line that begins with `else`; callToken and
// writeToken are what makes a condition not PureCond.
var (
	EmptyGuardedBlock = regexp.MustCompile(`(?m)^([ \t]*)(if \(.*\)|else if \(.*\)|else)\n([ \t]*)\{\n[ \t]*\}\n`)
	callToken         = regexp.MustCompile(`[A-Za-z_]\w*\s*\(`)
	writeToken        = regexp.MustCompile(`(^|[^=!<>])=($|[^=])|\+\+|--`)
	ElseLine          = regexp.MustCompile(`^[ \t]*else\b`)
)

// From phase 5d.
var (
	BareDeclOnly = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*[ \t]+\*?[A-Za-z_][A-Za-z0-9_]*;$`)
)

// From phase 25.
var returnStub = regexp.MustCompile(`(?s)\Areturn\s+(.+);\z`)

// From phase 25.
func pyRepr(s string) string { return "'" + s + "'" }

// IncludeCount is how many `#include` lines text has: the system headers a
// phase is handed.  Phases 36-54 once asserted a number -- eighteen, twelve,
// eleven -- because records 82 and 99 and phase 40 dropped the unused ones as
// they went; one last phase (88) drops them all now, so a phase asserts the count
// it was HANDED, and that it adds and removes none.
func IncludeCount(text []byte) int {
	n := 0
	for _, l := range bytes.Split(text, []byte{'\n'}) {
		if bytes.HasPrefix(l, []byte("#include ")) {
			n++
		}
	}
	return n
}

// MentionCount counts whole-word occurrences of name in text, outside
// `#include` lines: a directive names a HEADER, not an identifier.  Every
// phase's mention counter goes through it, so a count means the same thing
// whichever headers phase 88 has yet to drop.  (Until the headers were
// dropped last, a count before 166 included the lines of the headers still
// there -- `ioctl` was "the #include and the host's one call" -- and changed
// with every header a phase took.)
func MentionCount(text []byte, name string) int {
	if !isIdentWord(name) {
		return len(wordRe(name).FindAll(WithoutIncludes(text), -1))
	}
	// A name of word characters, counted as the regexp `\bname\b` counts
	// it, without the regexp: an occurrence with no word character on either
	// side, on a line that does not open with `#include `.  Two such
	// occurrences cannot overlap -- the second would start after a word
	// character of the first -- so these are FindAll's matches.
	nm := []byte(name)
	n := 0
	for pos := 0; ; {
		rel := bytes.Index(text[pos:], nm)
		if rel < 0 {
			return n
		}
		i := pos + rel
		j := i + len(nm)
		pos = i + 1
		if (i > 0 && identChar(text[i-1])) || (j < len(text) && identChar(text[j])) {
			continue
		}
		ls := bytes.LastIndexByte(text[:i], '\n') + 1
		if bytes.HasPrefix(text[ls:], []byte("#include ")) {
			continue
		}
		n++
	}
}

// isIdentWord says s is non-empty and all word characters, [0-9A-Za-z_].
func isIdentWord(s string) bool {
	for i := 0; i < len(s); i++ {
		if !identChar(s[i]) {
			return false
		}
	}
	return s != ""
}

// WithoutIncludes is text with every `#include` line blanked to an empty
// line, so offsets move but line numbers do not.
func WithoutIncludes(text []byte) []byte {
	if !bytes.Contains(text, []byte("#include ")) {
		return text
	}
	lines := bytes.Split(text, []byte{'\n'})
	out := make([][]byte, len(lines))
	for i, l := range lines {
		if bytes.HasPrefix(l, []byte("#include ")) {
			out[i] = nil
		} else {
			out[i] = l
		}
	}
	return bytes.Join(out, []byte{'\n'})
}

var wordRes sync.Map // name -> *regexp.Regexp

func wordRe(name string) *regexp.Regexp {
	if r, ok := wordRes.Load(name); ok {
		return r.(*regexp.Regexp)
	}
	r := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	wordRes.Store(name, r)
	return r
}
