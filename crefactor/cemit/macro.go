package cemit

import (
	"reflect"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// MACRO-INVOCATION RECOVERY.
//
// The front end expands the preprocessor, so a printer that walked the tree
// alone would write what the macro expanded TO and not what the source said:
// `errno` comes back as `(*__errno_location())`, `va_copy(*ap, ap_start)` as an
// assignment to an array, and the text stops being this machine's source and
// starts being this machine's glibc.
//
// WHAT MAKES IT RECOVERABLE is a property of cc/v4 worth stating, because
// everything here rests on it: EVERY token of an expansion carries the
// INVOCATION's position -- the arguments of a function-like macro too, not just
// its replacement list.  Measured: `MAXX(n + 1, 3)` gives twenty-one tokens,
// all at the column `MAXX` starts on, and `errno` gives six.
//
// So an expansion is a run of tokens sharing one position, and the text it came
// from is the source from that position to the next position any token has.
// That span is exactly what the preprocessor consumed -- the macro name for an
// object-like macro, the name and its balanced arguments for a function-like
// one -- without this file having to know which kind it was, or having a macro
// table at all.
//
// A node is printed as its invocation when the WHOLE node came from one
// expansion.  When only part of it did, the part is recovered where it sits and
// the rest is printed from the tree, so `if (MACRO(x))` keeps its `if`.

// expansions is what one external declaration's tokens say about the source:
// which positions are expansions, and where the text at each ends.
type expansions struct {
	at    map[int]int    // offset of an expansion -> offset just past its source text
	spans map[any][2]int // node -> the offsets of its first and last token
	count map[int]int    // how many tokens sit at an offset
	says  map[int]string // what a token at an offset says, when there is one
	src   []byte
	line  []int // offset of the start of each 1-based line
}

// tokens and spans are found by ONE reflective walk that stays in
// reflect.Value: calling Interface() on every field of every node -- which is
// what a walk over `any` does, and what cc.NodeTokens does -- boxes millions of
// values and spends its time in the collector.  Measured on the core: 37
// seconds that way, under three this way.
var tokenType = reflect.TypeOf(cc.Token{})

// walk records every token's offset, and -- when spans is not nil -- the first
// and last token offset of every node.
func (x *expansions) walk(v reflect.Value, spans map[any][2]int) (lo, hi int) {
	lo, hi = 1<<62, -1
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return lo, hi
		}
		return x.walk(v.Elem(), spans)
	case reflect.Struct:
	default:
		return lo, hi
	}
	if v.Type() == tokenType {
		tk, ok := v.Interface().(cc.Token)
		if !ok || tk.SrcStr() == "" {
			return 1 << 62, -1
		}
		p := tk.Position()
		off := x.offset(p.Line, p.Column)
		if off < 0 {
			return 1 << 62, -1
		}
		x.count[off]++
		if x.count[off] == 1 {
			x.says[off] = tk.SrcStr()
		}
		return off, off
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).PkgPath != "" && t.Field(i).Type != tokenType {
			continue // unexported, and not a token
		}
		f := v.Field(i)
		switch f.Kind() {
		case reflect.Pointer, reflect.Interface, reflect.Struct:
			a, b := x.walk(f, spans)
			if a < lo {
				lo = a
			}
			if b > hi {
				hi = b
			}
		}
	}
	if spans != nil && hi >= 0 && v.CanAddr() {
		spans[v.Addr().Interface()] = [2]int{lo, hi}
	}
	return lo, hi
}

// scan finds every expansion in a node and the span each one consumed, in ONE
// reflective walk: cc.NodeTokens would sort every token of every declaration,
// and asking it per node sorted them again at each level.
func (e *emitter) scan(n cc.Node) *expansions {
	x := &expansions{
		at:    map[int]int{},
		spans: map[any][2]int{},
		count: map[int]int{},
		says:  map[int]string{},
		src:   e.src,
		line:  e.lineAt,
	}
	x.walk(reflect.ValueOf(n), nil)
	offs := make([]int, 0, len(x.count))
	for off := range x.count {
		offs = append(offs, off)
	}
	sort.Ints(offs)
	for i, off := range offs {
		end := len(x.src)
		if i+1 < len(offs) {
			end = offs[i+1]
		}
		// One token at a position that says what the source says is ordinary
		// text, not an expansion.
		// COMPARE THE TOKEN'S OWN LENGTH, never `string(src[off:])`: converting
		// the rest of the file to a string for each of a million tokens is
		// quadratic, and it was -- 37 seconds on the core where the walk itself
		// takes a quarter of one.
		if x.count[off] == 1 && says(x.src, off, x.says[off]) {
			continue
		}
		// AN INVOCATION STARTS WITH AN IDENTIFIER, and the front end's own
		// synthesised tokens do not.  C23 predeclares `__func__` at the `{` of
		// every body, and the patched parser gives a trailing label an empty
		// statement whose `;` carries the COLON's position -- both look like
		// expansions by the test above, and recovering them wrote `{` and `:`
		// where a statement belonged.
		if !identStart(x.src, off) {
			continue
		}
		x.at[off] = end
	}
	// THE SECOND PASS IS ONLY PAID WHERE THERE IS SOMETHING TO RECOVER.  A
	// declaration with no expansion in it -- every declaration of the core,
	// which is header-free -- needs no node spans at all.
	if len(x.at) > 0 {
		x.walk(reflect.ValueOf(n), x.spans)
	}
	return x
}

// text is the source a macro invocation consumed, on one line.
func (x *expansions) text(off int) (string, bool) {
	end, ok := x.at[off]
	if !ok {
		return "", false
	}
	if end > len(x.src) {
		end = len(x.src)
	}
	return collapse(string(x.src[off:end])), true
}

// offset is the byte offset of a 1-based line and column.
func (x *expansions) offset(line, col int) int {
	if line < 1 || line >= len(x.line) {
		return -1
	}
	off := x.line[line] + col - 1
	if off > len(x.src) {
		return -1
	}
	return off
}

// collapse puts an invocation on one line: runs of whitespace become one space,
// which is what keeps the printed form a fixpoint when it is parsed again.
func collapse(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// fromMacro returns the invocation a node came from, if the whole node did.
func (e *emitter) fromMacro(n cc.Node) (string, bool) {
	if e.exp == nil || n == nil {
		return "", false
	}
	p := n.Position()
	off := e.exp.offset(p.Line, p.Column)
	if off < 0 {
		return "", false
	}
	if _, ok := e.exp.at[off]; !ok {
		return "", false
	}
	// The whole node came from it when its last token is at the same position.
	span, ok := e.exp.spans[n]
	if !ok || span[1] != off {
		return "", false
	}
	return e.exp.text(off)
}

// lineIndex is the offset of the start of each 1-based line of src.
func lineIndex(src []byte) []int {
	out := []int{0, 0} // line 0 unused, line 1 starts at 0
	for i, c := range src {
		if c == '\n' {
			out = append(out, i+1)
		}
	}
	return out
}

// says reports whether the source at off begins with the token's own text.
func says(src []byte, off int, tok string) bool {
	if tok == "" || off+len(tok) > len(src) {
		return false
	}
	return string(src[off:off+len(tok)]) == tok
}

// atExpansion is the expansion a token belongs to, if it belongs to one.
func (e *emitter) atExpansion(t cc.Token) (int, bool) {
	if e.exp == nil {
		return 0, false
	}
	p := t.Position()
	off := e.exp.offset(p.Line, p.Column)
	if off < 0 {
		return 0, false
	}
	_, ok := e.exp.at[off]
	return off, ok
}

// stmtMacro is fromMacro for a statement, where the source's own `;` is not
// part of the invocation: `FD_ZERO(&rfds);` expands to a `do { ... } while (0)`
// whose last token is that semicolon.  Without this the recovery cannot fire at
// the statement at all, and fires instead at every expression inside the
// expansion -- printing the invocation once per node of the replacement list.
func (e *emitter) stmtMacro(n cc.Node) (string, bool) {
	if e.exp == nil || n == nil {
		return "", false
	}
	p := n.Position()
	off := e.exp.offset(p.Line, p.Column)
	if off < 0 {
		return "", false
	}
	end, ok := e.exp.at[off]
	if !ok {
		return "", false
	}
	span, ok := e.exp.spans[n]
	if !ok {
		return "", false
	}
	switch {
	case span[1] == off:
		// the whole statement came from the expansion
	case span[1] >= end && span[1] < len(e.src) && e.src[span[1]] == ';' &&
		strings.TrimSpace(string(e.src[end:span[1]])) == "":
		// the statement is the expansion and the source's own semicolon
	default:
		return "", false
	}
	text, ok := e.exp.text(off)
	if !ok {
		return "", false
	}
	return text + ";", true
}

// identStart reports whether the source at off begins an identifier.
func identStart(src []byte, off int) bool {
	if off < 0 || off >= len(src) {
		return false
	}
	c := src[off]
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// specToken prints a type-specifier keyword AS THE SOURCE SPELLED IT.  With
// <stdbool.h> the input writes `bool`, the preprocessor makes it `_Bool`, and
// the token the parser hands back says `_Bool` at the column `bool` starts on:
// 78 declarations came out with a spelling the input does not use.  The
// recovery is the file's own, narrowed to the case where it cannot mean
// anything else -- an expansion whose whole source text is one
// identifier, which is what an object-like macro of a type name looks like.
func (e *emitter) specToken(t cc.Token) string {
	off, ok := e.atExpansion(t)
	if !ok {
		return tok(t)
	}
	s, ok := e.exp.text(off)
	if !ok || !isIdent(s) {
		return tok(t)
	}
	return s
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
