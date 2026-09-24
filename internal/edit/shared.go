package edit

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/arbace/go-whim/internal/cutil"
)

// What more than one phase uses.
//
// EVERY DECLARATION HERE WAS ONE PHASE'S, and is here because another phase
// reached it: when each phase became a package of its own (internal/phase/NNN), a helper
// two phases share stopped being either one's.  The phase it was written for is
// named above each.

// From phase 48.
// inFunction applies edit to one file-scope definition's Body and splices it
// back, which is what the heredoc's in_function() did.
func InFunction(text []byte, name string, edit func([]byte) ([]byte, error)) ([]byte, error) {
	a, z, ok := cutil.FindDefinition(text, cutil.Blank(text), name)
	if !ok {
		return nil, fmt.Errorf("%s is not defined at file scope", name)
	}
	Body, err := edit(text[a:z])
	if err != nil {
		return nil, err
	}
	Out := append([]byte{}, text[:a]...)
	Out = append(Out, Body...)
	return append(Out, text[z:]...), nil
}

// From phase 59.
// key and kex spell the two ways this file writes a special key as an integer.
// They are built rather than written Out because the C is a nest of escaped
// parentheses and the same shape recurs: KEY('k','B') is S-Tab, KEX(KE_WILD) is
// the wildcard trigger.
func Key(a, b string) string {
	return fmt.Sprintf(`\(-\(\('%s'\) \+ \(\(int\)\('%s'\) << 8\)\)\)`, a, b)
}

// From phase 67.
// names returns the first capture of every match, which is how this phase builds
// the report line that lists what it removed.
func Names_(re *regexp.Regexp, text []byte) []string {
	var Out []string
	for _, m := range re.FindAllSubmatch(text, -1) {
		Out = append(Out, string(m[1]))
	}
	return Out
}

// From phase 71.
// bindsToWalk names a `break` or `continue` in the Body that is not inside a
// loop or switch of the Body's own, or "" if there is none.
func BindsToWalk(raw []byte) string {
	rb := cutil.Blank(raw)
	type span struct{ a, z int }
	var spans []span
	for _, mm := range EnclosingLoop.FindAllIndex(rb, -1) {
		j := IndexFrom(rb, []byte("{"), mm[1])
		if j >= 0 {
			if k := cutil.Match(rb, j); k > 0 {
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

// From phase 71.
var _ = BwdWalk

// From phase 72.
func CountNewlines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}

// From phase 72.
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

// From phase 79.
// sortedKeys is GENERIC in the map's value, because three of us wrote one each:
// whim79 over map[string]string, whim88 over map[string]int and whim110 over
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

// From phase 80.
// contains is GENERIC because two sessions wrote one each and they collided:
// whim80's over []string and whim108's over []int.  One generic definition is
// the merge, and neither call site changed.
func Contains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// From phase 80.
// first is GENERIC for the same reason contains is: whim80 slices []string and
// whim125 slices []int, and one definition is cheaper than two names.
func First[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// From phase 80.
func IndexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

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

// From phase 90.
// zHead is the heredocs' `old[:n]` in a refusal.
func CoreHead(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// From phase 98.
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

// From phase 98.
func CoreCallRe(name string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(name) + `\s*\(`)
}

// From phase 99.
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

// From phase 121.
func ContainsStr(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

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

// From phase 134.
// W134Pure: a condition that only reads -- no call, no assignment, no ++ or --.
// Casts and sizeof read nothing a call could change; they are allowed.
func W134Pure(cond string) bool {
	c := regexp.MustCompile(`\((?:const\s+)?(?:unsigned\s+)?[A-Za-z_]\w*\s*\**\s*\)`).ReplaceAllString(cond, "")
	c = regexp.MustCompile(`\bsizeof\s*\(`).ReplaceAllString(c, "(")
	return !W134Fn.MatchString(c) && !W134Write.MatchString(c)
}

// From phase 155.
// Whim155 evaluates call arguments with effects in the order gcc does.
//
// C leaves the order of a call's arguments unspecified, Go evaluates them left
// to right, and gcc -- measured on this file's code, line by line in its
// disassembly -- evaluates them right to left.  Where two arguments both call
// a function with an effect outside its frame (internal/ccx's Order), the
// order is part of what the program does, and a translation that evaluated
// them left to right would not be this program.  Eleven calls are such: nine
// vim_strnsave(ml_get...(), ml_get..._len()), which gcc evaluates length
// first; col_print(..., ml_get_curline_len(), linetabsize_str(p)), which it
// evaluates linetabsize_str first; and fileinfo()'s message, whose
// new_file_message() it calls before the shortmess() of an earlier argument.
// Each first-evaluated argument becomes a local computed before the call, so
// the order is written, not implied (internal/gen/FINDINGS.md).
func Whim155(text []byte, w io.Writer) ([]byte, error) {
	p := Ph{Tag: "argorder", W: w}
	s := string(text)
	n := 0
	var bad string
	s = W155Save.ReplaceAllStringFunc(s, func(m string) string {
		sm := W155Save.FindStringSubmatch(m)
		ind, lhs, get, args, getLen, args2 := sm[1], sm[2], sm[3], sm[4], sm[5], sm[6]
		if args != args2 || getLen != get+"_len" {
			bad = m
			return m
		}
		n++
		return fmt.Sprintf("%s{\n%s    colnr_T     len = %s(%s);\n\n%s    %s = vim_strnsave(%s(%s), len);\n%s}\n",
			ind, ind, getLen, args, ind, lhs, get, args, ind)
	})
	if bad != "" {
		return nil, p.Die("a copy of a line whose length is not that line's: %q", strings.TrimSpace(bad))
	}
	if n != W155Saves {
		return nil, p.Die("%d copies of a line with its length, and this phase was written against %d", n, W155Saves)
	}
	p.Say(fmt.Sprintf("the %d copies of a line compute its length first, as gcc does", n))
	var err error
	if o, e := p.Literal([]byte(s), "            col_print(buf2, sizeof(buf2), ml_get_curline_len(), linetabsize_str(p));\n",
		"            {\n                int         vcol = linetabsize_str(p);\n\n                col_print(buf2, sizeof(buf2), ml_get_curline_len(), vcol);\n            }\n",
		"cursor_pos_info() computes the line's width before its length, as gcc does", 1); e != nil {
		return nil, e
	} else {
		s = string(o)
	}
	old := `(curbuf->b_flags & BF_NEW) ? new_file_message() : "", `
	i := strings.Index(s, old)
	if i < 0 || strings.Count(s, old) != 1 {
		return nil, p.Die("fileinfo()'s new-file argument is not where this phase expects it")
	}
	ls := strings.LastIndex(s[:i], "\n") + 1
	le := i + strings.Index(s[i:], "\n") + 1
	line := s[ls:le]
	ind := line[:len(line)-len(strings.TrimLeft(line, " "))]
	nl := strings.Replace(line, old, "new_msg, ", 1)
	s = s[:ls] + ind + "{\n" + ind + `    char        *new_msg = (curbuf->b_flags & BF_NEW) ? new_file_message() : "";` + "\n\n" + ind + "    " + strings.TrimLeft(nl, " ") + ind + "}\n" + s[le:]
	p.Say("fileinfo() asks whether the file is new before whether to shorten the modified flag, as gcc does")
	return []byte(s), err
}

// From phase 155.
func init() { Register("whim155", Whim155) }

// From phase 071.
const (
	FwdWalk = `for ((buf) = firstbuf; (buf) != NULL; (buf) = (buf)->b_next)`
	BwdWalk = `for ((buf) = lastbuf; (buf) != NULL; (buf) = (buf)->b_prev)`
)

// From phase 071.
var EnclosingLoop = regexp.MustCompile(`\b(for|while|switch|do)\b`)

// From phase 134.
var (
	W134Empty = regexp.MustCompile(`(?m)^([ \t]*)(if \(.*\)|else if \(.*\)|else)\n([ \t]*)\{\n[ \t]*\}\n`)
	W134Fn    = regexp.MustCompile(`[A-Za-z_]\w*\s*\(`)
	W134Write = regexp.MustCompile(`(^|[^=!<>])=($|[^=])|\+\+|--`)
	W134Else  = regexp.MustCompile(`^[ \t]*else\b`)
)

// From phase 155.
var W155Save = regexp.MustCompile(`(?m)^( *)([\w_]+) = vim_strnsave\((ml_get\w*)\(([^()\n]*)\), (ml_get\w*_len)\(([^()\n]*)\)\);\n`)

// From phase 155.
// W155Saves is how many vim_strnsave(line, its length) statements this phase
// sequences.
const W155Saves = 9

// From phase 065.
// deleteDefinition removes a file-scope definition and says so, refusing if it
// is not there.  The refusal names the function, because "not defined" about a
// function the phase is removing on purpose is the case where an earlier phase
// has already taken it and this one is about to claim work it did not do.
func (e *E) DeleteDefinition(name, what string) {
	if e.Failed() {
		return
	}
	Out, gone := cutil.DeleteDefinition(e.Text(), name)
	if !gone {
		e.Refuse("%s is not defined", name)
		return
	}
	e.Set(Out)
	e.Say(what)
}

// From phase 070.
// linesT deletes n whole lines matching the pattern, TOLERATING TRAILING
// WHITESPACE -- `^[ \t]*<pattern>[ \t]*\n`, which is what whim70's own lines()
// spells.  E.Lines does not allow the trailing run, and the difference decides
// whether a line with a stray space at its end is found or silently left.
func (e *E) LinesT(pattern string, n int, what string) {
	e.Cut(`(?m)^[ \t]*`+pattern+`[ \t]*\n`, n, what)
}

// From phase 071.
// foldWalk turns `for ((v) = firstbuf; ...)` and its block into `v = curbuf;`
// followed by the block's Body, dedented.
//
// IT REFUSES A BODY WITH A `break` OR `continue` THAT BINDS TO THE WALK.
// Deleting the `for` header rebinds such a statement to whatever encloses it,
// or to nothing at all -- getout()'s walk did exactly that and produced "break
// statement not within loop or switch".  It is the same hazard CLAUDE.md
// records for unwrapping `do { } while (0)`, and it must be a refusal rather
// than a silent miscompile.
//
// The test is "is there an enclosing loop or switch INSIDE the body", not "is
// it at brace depth 0": C binds break to the nearest enclosing loop or switch
// and brace depth has nothing to do with it -- getout()'s break sits two ifs
// deep and still bound to the `for`.  An earlier version tested depth and would
// have passed it.
func (e *E) FoldWalk(fn, v, head, what string, n int) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		pat := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(head) + `[ \t]*\n([ \t]*)\{\n`)
		if k := len(pat.FindAll(e.buf, -1)); k != n {
			e.Die("%s -- the walk matches %d times, expected %d", what, k, n)
			return
		}
		for i := 0; i < n; i++ {
			m := pat.FindSubmatchIndex(e.buf)
			b := cutil.Blank(e.buf)
			o := IndexFrom(e.buf, []byte("{"), m[0])
			c := cutil.Match(b, o)
			raw := e.buf[IndexFrom(e.buf, []byte("\n"), o)+1 : LastNewlineBefore(e.buf, c)+1]
			if bad := BindsToWalk(raw); bad != "" {
				e.Die("%s -- the body has a `%s;` that binds to the walk being removed, not to anything inside it", what, bad)
				return
			}
			Body := cutil.Dedent4(raw)
			end := IndexFrom(e.buf, []byte("\n"), c) + 1
			Out := append([]byte{}, e.buf[:m[0]]...)
			Out = append(Out, e.buf[m[2]:m[3]]...)
			Out = append(Out, (v + " = curbuf;\n")...)
			Out = append(Out, Body...)
			e.buf = append(Out, e.buf[end:]...)
		}
		e.Say(what)
	})
}

// From phase 071.
// dropWalk replaces `for (<head>)` and the block it runs with repl.
//
// The head is matched as a REGEX built from the literal, never compared as one:
// these walk lines are macro-expanded and carry a trailing space after the
// closing paren, and a literal written Out in a shell heredoc is a bad place to
// depend on invisible whitespace -- one written that way already failed here.
func (e *E) DropWalk(fn, head, repl, what string, n int) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		pat := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(head) + `[ \t]*\n[ \t]*\{\n`)
		if k := len(pat.FindAll(e.buf, -1)); k != n {
			e.Die("%s -- the walk matches %d times, expected %d", what, k, n)
			return
		}
		for i := 0; i < n; i++ {
			m := pat.FindIndex(e.buf)
			b := cutil.Blank(e.buf)
			o := IndexFrom(e.buf, []byte("{"), m[0])
			c := cutil.Match(b, o)
			end := IndexFrom(e.buf, []byte("\n"), c) + 1
			Out := append([]byte{}, e.buf[:m[0]]...)
			Out = append(Out, repl...)
			e.buf = append(Out, e.buf[end:]...)
		}
		e.Say(what)
	})
}

// From phase 072.
// foldWalks folds every walk of one shape, back to front so the offsets still to
// be processed stay valid, refusing if any Body's break or continue would rebind
// to the loop being removed.
func (e *E) FoldWalks(headRe string, ok func([]string) bool, subst func([]string) string, what string) {
	if e.Err != nil {
		return
	}
	pat := regexp.MustCompile(`(?m)^([ \t]*)` + headRe + `[ \t]*\n[ \t]*\{\n`)
	var ms [][]int
	for _, m := range pat.FindAllSubmatchIndex(e.buf, -1) {
		g := make([]string, len(m)/2)
		for i := range g {
			if m[2*i] >= 0 {
				g[i] = string(e.buf[m[2*i]:m[2*i+1]])
			}
		}
		if ok(g) {
			ms = append(ms, m)
		}
	}
	if len(ms) == 0 {
		e.Die("%s -- no walk of this shape is left to fold", what)
		return
	}
	var unsafe []int
	total := len(ms)
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		b := cutil.Blank(e.buf)
		o := IndexFrom(e.buf, []byte("{"), m[0])
		c := cutil.Match(b, o)
		if c < 0 {
			e.Die("%s -- unbalanced block", what)
			return
		}
		raw := e.buf[IndexFrom(e.buf, []byte("\n"), o)+1 : LastNewlineBefore(e.buf, c)+1]
		if BindsToWalk(raw) != "" {
			unsafe = append(unsafe, 1+CountNewlines(e.buf[:m[0]]))
			continue
		}
		g := make([]string, len(m)/2)
		for k := range g {
			if m[2*k] >= 0 {
				g[k] = string(e.buf[m[2*k]:m[2*k+1]])
			}
		}
		Body := cutil.Dedent4(raw)
		end := IndexFrom(e.buf, []byte("\n"), c) + 1
		Out := append([]byte{}, e.buf[:m[0]]...)
		Out = append(Out, e.buf[m[2]:m[3]]...)
		Out = append(Out, (subst(g) + "\n")...)
		Out = append(Out, Body...)
		e.buf = append(Out, e.buf[end:]...)
	}
	if len(unsafe) > 0 {
		e.Die("%s -- %d walk(s) still carry an escaping break/continue and need an explicit rewrite: lines %s",
			what, len(unsafe), JoinInts(unsafe))
		return
	}
	e.Say(fmt.Sprintf("%s (%d)", what, total))
}

// From phase 072.
// replaceBlock replaces a brace-matched block, anchored on the line that opens it.
func (e *E) ReplaceBlock(fn, anchorRe, repl, what string) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		m := regexp.MustCompile(anchorRe).FindIndex(e.buf)
		if m == nil {
			e.Die("%s -- no line matches %s", what, cutil.PyRepr(anchorRe))
			return
		}
		k := LastNewlineBefore(e.buf, m[0]) + 1
		b := cutil.Blank(e.buf)
		o := IndexFrom(e.buf, []byte("{"), m[0])
		c := cutil.Match(b, o)
		if c < 0 {
			e.Die("%s -- unbalanced block", what)
			return
		}
		Out := append([]byte{}, e.buf[:k]...)
		Out = append(Out, repl...)
		e.buf = append(Out, e.buf[IndexFrom(e.buf, []byte("\n"), c)+1:]...)
	})
	if !e.Failed() {
		e.Say(what)
	}
}

// From phase 072.
// foldNeverIn folds inside one function and reports after, which is this
// phase's own shape -- the say() is outside in_function.
func (e *E) FoldNeverIn(fn, pattern, what string, n int) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		Out, err := cutil.FoldNever(e.buf, pattern, n)
		if err != nil {
			e.Die("%s -- %v", what, err)
			return
		}
		e.buf = Out
	})
	if !e.Failed() {
		e.Say(what)
	}
}

// From phase 072.
// dropWalkIn is whim71's dropWalk with the report after in_function.
func (e *E) DropWalkIn(fn, headRe, repl, what string, n int) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		pat := regexp.MustCompile(`(?m)^[ \t]*` + headRe + `[ \t]*\n[ \t]*\{\n`)
		if k := len(pat.FindAll(e.buf, -1)); k != n {
			e.Die("%s -- the walk matches %d times, expected %d", what, k, n)
			return
		}
		for i := 0; i < n; i++ {
			m := pat.FindIndex(e.buf)
			b := cutil.Blank(e.buf)
			o := IndexFrom(e.buf, []byte("{"), m[0])
			c := cutil.Match(b, o)
			end := IndexFrom(e.buf, []byte("\n"), c) + 1
			Out := append([]byte{}, e.buf[:m[0]]...)
			Out = append(Out, repl...)
			e.buf = append(Out, e.buf[end:]...)
		}
	})
	if !e.Failed() {
		e.Say(what)
	}
}

// From phase 075.
// dropBareBlock deletes the innermost block enclosing a statement, REFUSING if
// that block still does real work -- anything but the statement itself and bare
// declarations.  It is for the husk a removed call leaves behind.
func (e *E) DropBareBlock(fn, stmt, what string) {
	e.InFunction(fn, func(e *E) {
		if e.Failed() {
			return
		}
		b := cutil.Blank(e.buf)
		i := IndexFrom(e.buf, []byte(stmt), 0)
		if i < 0 {
			e.Die("%s -- %s is not in %s", what, cutil.PyRepr(stmt), fn)
			return
		}
		depth, j := 0, i
		for ; j >= 0; j-- {
			if b[j] == '}' {
				depth++
			} else if b[j] == '{' {
				if depth == 0 {
					break
				}
				depth--
			}
		}
		if j < 0 {
			e.Die("%s -- no enclosing block", what)
			return
		}
		c := cutil.Match(b, j)
		if c < 0 {
			e.Die("%s -- unbalanced block", what)
			return
		}
		Body := e.buf[IndexFrom(e.buf, []byte("\n"), j)+1 : LastNewlineBefore(e.buf, c)+1]
		for _, line := range strings.Split(string(Body), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == stmt || BareDeclOnly.MatchString(line) {
				continue
			}
			if len(line) > 60 {
				line = line[:60]
			}
			e.Die("%s -- the block still does real work: %s", what, cutil.PyRepr(line))
			return
		}
		k := LastNewlineBefore(e.buf, j) + 1
		e.Say(what)
		Out := append([]byte{}, e.buf[:k]...)
		e.buf = append(Out, e.buf[IndexFrom(e.buf, []byte("\n"), c)+1:]...)
	})
}

// From phase 079.
// innerBody is a definition's Body between the brace on its own line and the
// closing brace -- NOT the definition, which carries the parameter list.
func (e *E) InnerBody(name string) (string, bool) {
	def, ok := e.BodyOf(name)
	if !ok {
		return "", false
	}
	s := string(def)
	i := strings.Index(s, "{\n")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return "", false
	}
	return s[i+2 : j], true
}

// From phase 079.
// constOf requires name's whole Body to be `return <expect>;` and nothing else.
func (e *E) ConstOf(name, expect string) {
	if e.Failed() {
		return
	}
	Inner, ok := e.InnerBody(name)
	if !ok {
		e.Refuse("%s is not defined", name)
		return
	}
	Inner = strings.TrimSpace(Inner)
	m := returnStub.FindStringSubmatch(Inner)
	if m == nil {
		if len(Inner) > 70 {
			Inner = Inner[:70]
		}
		e.Refuse("%s is no longer a one-line stub: %s", name, pyRepr(Inner))
		return
	}
	if got := strings.TrimSpace(m[1]); got != expect {
		e.Refuse("%s returns %s, not %s -- folding it would change behaviour", name, pyRepr(got), pyRepr(expect))
	}
}

// From phase 080.
// foldAlwaysElse turns `if (TRUE) { A } else { B }` into A.  cutil.FoldAlways
// refuses an else arm, and this phase has exactly one of that shape.
func (e *E) FoldAlwaysElse(pattern, what string) {
	if e.Err != nil {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.Die("%s -- %v", what, err)
		return
	}
	ms := re.FindAllIndex(e.buf, -1)
	if len(ms) != 1 {
		e.Die("%s -- %d matches, expected 1", what, len(ms))
		return
	}
	t := string(e.buf)
	b := cutil.Blank(e.buf)
	k, o, c, head, err := cutil.Guarded(e.buf, b, ms[0])
	if err != nil {
		e.Die("%s -- %v", what, err)
		return
	}
	if head != "if" {
		e.Die("%s -- not a plain if: %s", what, cutil.PyRepr(head))
		return
	}
	end := strings.Index(t[c:], "\n") + c + 1
	m := W80Else.FindStringIndex(t[end:])
	if m == nil {
		e.Die("%s -- expected an else", what)
		return
	}
	o2 := strings.Index(string(b[end+m[1]:]), "{")
	if o2 < 0 {
		e.Die("%s -- expected an else", what)
		return
	}
	o2 += end + m[1]
	c2 := cutil.Match(b, o2)
	if W80Else2.MatchString(t[strings.Index(t[c2:], "\n")+c2+1:]) {
		e.Die("%s -- the else is followed by another else", what)
		return
	}
	bodyStart := strings.Index(t[o:], "\n") + o + 1
	bodyEnd := strings.LastIndex(t[:c], "\n") + 1
	Body := cutil.Dedent4([]byte(t[bodyStart:bodyEnd]))
	e.Say(what)
	e.buf = []byte(t[:k] + string(Body) + t[strings.Index(t[c2:], "\n")+c2+1:])
}

// From phase 080.
// refused records the message and returns it, so a raw check reads as one line.
func (e *E) Refused(format string, a ...interface{}) error {
	e.Die(format, a...)
	_, err := e.Done()
	return err
}

// From phase 081.
// term replaces a fragment n times, refusing on any other count.  It is
// Literal under whim81's own name.
func (e *E) Term(frag, repl string, n int, what string) { e.LiteralN(frag, repl, n, what) }

// From phase 075.
var (
	AutopatDecl = regexp.MustCompile(
		`(?m)^static AutoPat \*first_autopat\[NUM_EVENTS\] =\n\{\n[ \t]*NULL,\n\};$`)
	BareDispatch = regexp.MustCompile(`(?m)^[ \t]*(?:\(void\))?apply_autocmds\w*\([^\n]*\);[ \t]*\n`)
	CmdTrigger   = regexp.MustCompile(`(?m)^[ \t]*trigger_cmd_autocmd\([^\n]*\);[ \t]*\n`)
	BareDeclOnly = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*[ \t]+\*?[A-Za-z_][A-Za-z0-9_]*;$`)
)

// From phase 079.
var returnStub = regexp.MustCompile(`(?s)\Areturn\s+(.+);\z`)

// From phase 079.
func pyRepr(s string) string { return "'" + s + "'" }

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
	W80Blanks = regexp.MustCompile(`\n\n+`)
	W80Label  = regexp.MustCompile(`^([ \t]*)(case \w+:|default:)$`)
	W80Case   = regexp.MustCompile(`^[ \t]*case (\w+):$`)
	W80Fall   = regexp.MustCompile(`^[ \t]*(break;|goto \w+;|return\b.*;|\{)$`)
	W80Word   = regexp.MustCompile(`^[A-Za-z]+`)
	W80Skip   = regexp.MustCompile(`\b(ea\.|eap->)skip\b`)
	W80Vim9   = regexp.MustCompile(`\bvim9\b`)
	W80Else   = regexp.MustCompile(`^[ \t]*else[ \t]*\n`)
	W80Else2  = regexp.MustCompile(`^[ \t]*else\b`)
)

// IncludeCount is how many `#include` lines text has: the system headers a
// phase is handed.  Phases 97-126 once asserted a number -- eighteen, twelve,
// eleven -- because phases 82, 99 and 104 dropped the unused ones as they
// went; one last phase (167) drops them all now, so a phase asserts the count
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
// whichever headers phase 167 has yet to drop.  (Until the headers were
// dropped last, a count before 166 included the lines of the headers still
// there -- `ioctl` was "the #include and the host's one call" -- and changed
// with every header a phase took.)
func MentionCount(text []byte, name string) int {
	return len(wordRe(name).FindAll(WithoutIncludes(text), -1))
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
