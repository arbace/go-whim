package togo

import (
	"strings"
)

// THE PRINTER'S NOISE.  The Clojure printer composes its forms as text, a
// basic block at a time, and what it composes reads as machine output:
// `(let [_ (f)] nil)`, `(if c x nil)`, a let whose body is a let, `(do (do
// ...))`, `(and (and a b) c)` (doc/CLOJURE-IDIOMS.md, item 1).  cljTidy
// reads each defn of the finished namespace into a tree, rewrites it by
// rules each of which keeps the value it is used for -- or, where the value
// is thrown away (a statement), keeps what it does -- and prints it back in
// the printer's layout.  A top-level form that is not a defn, and a defn it
// cannot read or lay out, it leaves as it is.
//
// It runs after the printer has split its long functions (shaper.cost), so
// it cannot make a method longer than the one the split was sized for.

// cnode is a form: an atom, a list, a vector, a map, a set, a form with its
// metadata (`^T x`) or with a reader prefix (`'x`, `@x`).
type cnode struct {
	kind  byte // 'a' atom, '(' list, '[' vector, '{' map, '#' set, '^' meta, '\'' prefix
	text  string
	kids  []*cnode
	multi bool // it spanned lines as the printer wrote it
}

func catom(s string) *cnode { return &cnode{kind: 'a', text: s} }

func (n *cnode) head() string {
	if n.kind == '(' && len(n.kids) > 0 && n.kids[0].kind == 'a' {
		return n.kids[0].text
	}
	return ""
}

func (n *cnode) isAtom(s string) bool { return n.kind == 'a' && n.text == s }

// creader reads forms from src.
type creader struct {
	src string
	i   int
	bad bool
}

func (r *creader) space() {
	for r.i < len(r.src) {
		switch c := r.src[r.i]; {
		case c == ' ' || c == '\n' || c == '\t' || c == ',' || c == '\r':
			r.i++
		case c == ';':
			r.bad = true // a comment inside a form: not read
			return
		default:
			return
		}
	}
}

func (r *creader) form() *cnode {
	r.space()
	if r.bad || r.i >= len(r.src) {
		r.bad = true
		return nil
	}
	start := r.i
	c := r.src[r.i]
	var n *cnode
	switch {
	case c == '(' || c == '[' || c == '{' || c == '#' && r.i+1 < len(r.src) && r.src[r.i+1] == '{':
		open, close := c, map[byte]byte{'(': ')', '[': ']', '{': '}'}[c]
		kind := c
		if c == '#' {
			r.i++
			open, close, kind = '{', '}', '#'
		}
		_ = open
		r.i++
		n = &cnode{kind: kind}
		for {
			r.space()
			if r.bad || r.i >= len(r.src) {
				r.bad = true
				return nil
			}
			if r.src[r.i] == close {
				r.i++
				break
			}
			k := r.form()
			if k == nil {
				return nil
			}
			n.kids = append(n.kids, k)
		}
	case c == ')' || c == ']' || c == '}':
		r.bad = true
		return nil
	case c == '^':
		j := r.i + 1
		for j < len(r.src) && !strings.ContainsRune(" \n\t,()[]{}", rune(r.src[j])) {
			j++
		}
		meta := r.src[r.i:j]
		if j == r.i+1 {
			r.bad = true // ^{...} or ^[...]: not read
			return nil
		}
		r.i = j
		k := r.form()
		if k == nil {
			return nil
		}
		n = &cnode{kind: '^', text: meta, kids: []*cnode{k}}
	case c == '\'' || c == '@' || c == '`' || c == '~':
		p := string(c)
		r.i++
		if c == '~' && r.i < len(r.src) && r.src[r.i] == '@' {
			p += "@"
			r.i++
		}
		k := r.form()
		if k == nil {
			return nil
		}
		n = &cnode{kind: '\'', text: p, kids: []*cnode{k}}
	case c == '"' || c == '#' && r.i+1 < len(r.src) && r.src[r.i+1] == '"':
		j := r.i + 1
		if c == '#' {
			j++
		}
		for j < len(r.src) && r.src[j] != '"' {
			if r.src[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(r.src) {
			r.bad = true
			return nil
		}
		n = catom(r.src[r.i : j+1])
		r.i = j + 1
	case c == '\\':
		j := r.i + 2 // a character: \x, \newline, A, \(
		for j < len(r.src) && !strings.ContainsRune(" \n\t,()[]{}\"", rune(r.src[j])) {
			j++
		}
		n = catom(r.src[r.i:j])
		r.i = j
	case c == '#':
		r.bad = true // #'x, #(...), #_ and the rest: not read
		return nil
	default:
		j := r.i
		for j < len(r.src) && !strings.ContainsRune(" \n\t,()[]{}\"^;", rune(r.src[j])) {
			j++
		}
		n = catom(r.src[r.i:j])
		r.i = j
	}
	n.multi = strings.Contains(r.src[start:r.i], "\n")
	return n
}

// the forms the printer lays out over lines; any other form it writes on
// one line
var cblock = map[string]bool{"let": true, "loop": true, "if": true, "if-not": true, "when": true, "when-not": true, "do": true, "case": true, "defn": true, "defn-": true}

// cprinter prints forms in the printer's layout; ok goes false for a form
// it has no layout for.
type cprinter struct{ ok bool }

func pad(n int) string { return strings.Repeat(" ", n) }

// pr is n printed with its first character at column col.
func (p *cprinter) pr(n *cnode, col int) string {
	switch n.kind {
	case 'a':
		return n.text
	case '^':
		return n.text + " " + p.pr(n.kids[0], col+len(n.text)+1)
	case '\'':
		return n.text + p.pr(n.kids[0], col+len(n.text))
	case '[':
		return p.flat("[", "]", n.kids, col)
	case '{':
		return p.flat("{", "}", n.kids, col)
	case '#':
		return p.flat("#{", "}", n.kids, col)
	}
	h := n.head()
	if !cblock[h] || !n.multi {
		if n.multi {
			p.ok = false // a form over lines the printer has no layout for
		}
		return p.flat("(", ")", n.kids, col)
	}
	var b strings.Builder
	switch h {
	case "let", "loop":
		if len(n.kids) < 2 || n.kids[1].kind != '[' {
			p.ok = false
			return p.flat("(", ")", n.kids, col)
		}
		b.WriteString("(" + h + " [")
		bcol := col + len(h) + 3
		bs := n.kids[1].kids
		for i := 0; i+1 < len(bs); i += 2 {
			if i > 0 {
				b.WriteString("\n" + pad(bcol))
			}
			name := p.pr(bs[i], bcol)
			b.WriteString(name + " " + p.pr(bs[i+1], bcol+len(name)+1))
		}
		b.WriteString("]")
		for _, k := range n.kids[2:] {
			b.WriteString("\n" + pad(col+2) + p.pr(k, col+2))
		}
	case "if", "if-not", "when", "when-not":
		if len(n.kids) < 2 {
			p.ok = false
			return p.flat("(", ")", n.kids, col)
		}
		b.WriteString("(" + h + " " + p.pr(n.kids[1], col+len(h)+2))
		for _, k := range n.kids[2:] {
			b.WriteString("\n" + pad(col+2) + p.pr(k, col+2))
		}
	case "do":
		if len(n.kids) < 2 {
			return p.flat("(", ")", n.kids, col)
		}
		b.WriteString("(do " + p.pr(n.kids[1], col+4))
		for _, k := range n.kids[2:] {
			b.WriteString("\n" + pad(col+4) + p.pr(k, col+4))
		}
	case "case":
		if len(n.kids) < 2 {
			p.ok = false
			return p.flat("(", ")", n.kids, col)
		}
		b.WriteString("(case " + p.pr(n.kids[1], col+6))
		arms := n.kids[2:]
		i := 0
		for ; i+1 < len(arms); i += 2 {
			b.WriteString("\n" + pad(col+2) + p.pr(arms[i], col+2))
			b.WriteString("\n" + pad(col+4) + p.pr(arms[i+1], col+4))
		}
		if i < len(arms) {
			b.WriteString("\n" + pad(col+2) + p.pr(arms[i], col+2))
		}
	case "defn", "defn-":
		// the name, its result's hint and its parameters on the first line
		i := 1
		for i < len(n.kids) && !cparams(n.kids[i]) {
			i++
		}
		if i >= len(n.kids) {
			p.ok = false
			return p.flat("(", ")", n.kids, col)
		}
		b.WriteString(strings.TrimSuffix(p.flat("(", ")", n.kids[:i+1], col), ")"))
		for _, k := range n.kids[i+1:] {
			b.WriteString("\n" + pad(col+2) + p.pr(k, col+2))
		}
	}
	b.WriteString(")")
	return b.String()
}

// flat is kids between open and close, a space apart, each at its column.
func (p *cprinter) flat(open, close string, kids []*cnode, col int) string {
	var b strings.Builder
	b.WriteString(open)
	at := col + len(open)
	for i, k := range kids {
		if i > 0 {
			b.WriteString(" ")
			at++
		}
		s := p.pr(k, at)
		b.WriteString(s)
		if j := strings.LastIndexByte(s, '\n'); j >= 0 {
			at = len(s) - j - 1
		} else {
			at += len(s)
		}
	}
	b.WriteString(close)
	return b.String()
}

// cljTidyFile is cljTidy on every top-level defn and defn- of a
// namespace's text.
func cljTidyFile(src string) string {
	var b strings.Builder
	at := 0
	for i := 0; i < len(src); {
		// a top-level form starts at a line's start
		if src[i] == '(' && (i == 0 || src[i-1] == '\n') && (strings.HasPrefix(src[i:], "(defn ") || strings.HasPrefix(src[i:], "(defn- ")) {
			r := &creader{src: src, i: i}
			n := r.form()
			if n != nil && !r.bad {
				b.WriteString(src[at:i])
				b.WriteString(cljTidy(src[i:r.i], n))
				at, i = r.i, r.i
				continue
			}
		}
		j := strings.IndexByte(src[i:], '\n')
		if j < 0 {
			break
		}
		i += j + 1
	}
	b.WriteString(src[at:])
	return b.String()
}

// cljTidy is the defn n, read from text, rewritten and printed; text itself
// when it cannot be laid out.
func cljTidy(text string, n *cnode) string {
	n = ctx(n)
	p := &cprinter{ok: true}
	out := p.pr(n, 0)
	if !p.ok {
		return text
	}
	return out
}

// ctx rewrites a defn: its body is a body whose value is the function's.
func ctx(n *cnode) *cnode {
	i := 1
	for i < len(n.kids) && !cparams(n.kids[i]) {
		i++
	}
	if i >= len(n.kids) {
		return n
	}
	body := cbody(n.kids[i+1:], false)
	if len(body) == 0 {
		body = []*cnode{catom("nil")}
	}
	return &cnode{kind: '(', kids: append(append([]*cnode{}, n.kids[:i+1]...), body...), multi: n.multi}
}

// cparams says n is a defn's parameters: a vector, or one with the
// result's hint (^BytePtr [...]).
func cparams(n *cnode) bool {
	return n.kind == '[' || n.kind == '^' && n.kids[0].kind == '['
}

// cpure says a statement n does nothing: an atom.
func cpure(n *cnode) bool { return n.kind == 'a' }

// cbody rewrites forms, a body -- a do's, a let's, a when's: every form but
// the last is a statement, whose value is thrown away, and the last is one
// too when stmt says the body's value is.  A statement that does nothing
// goes, and a do among them is its forms.
func cbody(forms []*cnode, stmt bool) []*cnode {
	var out []*cnode
	for i, f := range forms {
		last := i == len(forms)-1
		k := crule(f, !last || stmt)
		if (!last || stmt) && cpure(k) {
			continue
		}
		if k.head() == "do" {
			out = append(out, k.kids[1:]...)
			continue
		}
		out = append(out, k)
	}
	return out
}

// cdo is forms as one form: itself when there is one, nil when none.
func cdo(forms []*cnode, multi bool) *cnode {
	switch len(forms) {
	case 0:
		return catom("nil")
	case 1:
		return forms[0]
	}
	return &cnode{kind: '(', kids: append([]*cnode{catom("do")}, forms...), multi: multi}
}

func clist(multi bool, kids ...*cnode) *cnode { return &cnode{kind: '(', kids: kids, multi: multi} }

// cname is a binding's name without its hint.
func cname(n *cnode) string {
	for n.kind == '^' {
		n = n.kids[0]
	}
	if n.kind == 'a' {
		return n.text
	}
	return ""
}

// the operators Clojure writes (op a b c) as (op (op a b) c) for
var cnary = map[string]bool{"+": true, "*": true, "bit-and": true, "bit-or": true, "bit-xor": true}

// the conversions: of no effect but their value
var cconv = map[string]bool{"long": true, "int": true, "boolean": true, "unchecked-byte": true, "unchecked-short": true,
	"unchecked-int": true, "unchecked-long": true, "i8": true, "u8": true, "i16": true, "u16": true, "i32": true, "u32": true}

// the conversions to a width, and what a macro of the namespace converts to
var cnarrow = map[string]int{"unchecked-byte": 8, "unchecked-short": 16, "unchecked-int": 32,
	"i8": 8, "u8": 8, "i16": 16, "u16": 16, "i32": 32, "u32": 32}

// crule is n rewritten; stmt says its value is thrown away.
func crule(n *cnode, stmt bool) *cnode {
	switch n.kind {
	case '^', '\'':
		return &cnode{kind: n.kind, text: n.text, kids: []*cnode{crule(n.kids[0], false)}, multi: n.multi}
	case '[', '{', '#':
		k := &cnode{kind: n.kind, multi: n.multi}
		for _, e := range n.kids {
			k.kids = append(k.kids, crule(e, false))
		}
		return k
	case 'a':
		return n
	}
	h := n.head()
	kids := n.kids
	switch h {
	case "fn", "fn*", "letfn", "reify", "quote", "defn", "defn-", "defmacro":
		return n // not looked into
	case "let":
		if len(kids) < 2 || kids[1].kind != '[' || len(kids[1].kids)%2 != 0 {
			return n
		}
		var bs []*cnode
		for i := 0; i+1 < len(kids[1].kids); i += 2 {
			name := kids[1].kids[i]
			bs = append(bs, name, crule(kids[1].kids[i+1], cname(name) == "_"))
		}
		return clet(bs, cbody(kids[2:], stmt), stmt, n.multi)
	case "loop":
		if len(kids) < 2 || kids[1].kind != '[' {
			return n
		}
		v := &cnode{kind: '[', multi: kids[1].multi}
		for i, e := range kids[1].kids {
			if i%2 == 1 {
				e = crule(e, false)
			}
			v.kids = append(v.kids, e)
		}
		body := cbody(kids[2:], false) // a recur is its tail
		if len(body) == 0 {
			body = []*cnode{catom("nil")}
		}
		return clist(n.multi, append([]*cnode{kids[0], v}, body...)...)
	case "do":
		return cdo(cbody(kids[1:], stmt), n.multi)
	case "if":
		if len(kids) < 3 || len(kids) > 4 {
			return n
		}
		test := crule(kids[1], false)
		th := crule(kids[2], stmt)
		el := catom("nil")
		if len(kids) == 4 {
			el = crule(kids[3], stmt)
		}
		switch {
		case el.isAtom("nil") && th.isAtom("nil") && stmt:
			return test
		case el.isAtom("nil"):
			return cwhen("when", test, th, stmt, n.multi)
		case th.isAtom("nil"):
			return cwhen("when-not", test, el, stmt, n.multi)
		}
		return clist(n.multi, kids[0], test, th, el)
	case "when", "when-not":
		if len(kids) < 2 {
			return n
		}
		return cwhen(h, crule(kids[1], false), cdo(cbody(kids[2:], stmt), n.multi), stmt, n.multi)
	case "case":
		if len(kids) < 3 {
			return n
		}
		k := clist(n.multi, kids[0], crule(kids[1], false))
		arms := kids[2:]
		for i := 0; i < len(arms); i++ {
			if i%2 == 0 && i == len(arms)-1 {
				// the default: a machine's, which no state reaches, goes --
				// a case with none throws already
				if cthrowsNoState(arms[i]) {
					break
				}
				k.kids = append(k.kids, crule(arms[i], stmt))
				break
			}
			if i%2 == 0 {
				k.kids = append(k.kids, arms[i])
			} else {
				k.kids = append(k.kids, crule(arms[i], stmt))
			}
		}
		return k
	case "and", "or":
		k := clist(n.multi, kids[0])
		for _, e := range kids[1:] {
			e = crule(e, false)
			if e.head() == h {
				k.kids = append(k.kids, e.kids[1:]...) // (and (and a b) c) is (and a b c)
				continue
			}
			k.kids = append(k.kids, e)
		}
		return k
	}
	if _, conv := cconv[h]; conv && stmt && len(kids) == 2 {
		return crule(kids[1], true) // a conversion of a value no one reads
	}
	k := clist(n.multi)
	for _, e := range kids {
		k.kids = append(k.kids, crule(e, false))
	}
	// (+ (+ a b) c) is (+ a b c): Clojure expands the second into the first
	// for these operators, pair by pair from the left
	if cnary[h] && len(k.kids) >= 3 && k.kids[1].head() == h && len(k.kids[1].kids) >= 3 {
		k.kids = append(append([]*cnode{k.kids[0]}, k.kids[1].kids[1:]...), k.kids[2:]...)
	}
	// a conversion of a conversion to as many bits or more is the outer one
	// of the inner's argument: (unchecked-int (i32 x)) is (unchecked-int x)
	if w, ok := cnarrow[h]; ok && len(k.kids) == 2 {
		if in := k.kids[1]; len(in.kids) == 2 {
			if w2, ok := cnarrow[in.head()]; ok && w2 >= w {
				k.kids[1] = in.kids[1]
			}
		}
	}
	return k
}

// clet is (let [bs] body...), with its noise taken out: a let whose body is
// one let is one let; a step bound to _ before any name is bound is done
// before the let, and one after the last name in its body; a let that binds
// nothing is its body; (let [x v] x) is v.
func clet(bs, body []*cnode, stmt, multi bool) *cnode {
	if len(body) == 1 && body[0].head() == "let" && len(body[0].kids) >= 2 && body[0].kids[1].kind == '[' {
		inner := body[0]
		bs = append(append([]*cnode{}, bs...), inner.kids[1].kids...)
		body = inner.kids[2:]
	}
	var before []*cnode
	for len(bs) >= 2 && cname(bs[0]) == "_" {
		before = append(before, bs[1])
		bs = bs[2:]
	}
	var after []*cnode
	for len(bs) >= 2 && cname(bs[len(bs)-2]) == "_" {
		after = append([]*cnode{bs[len(bs)-1]}, after...)
		bs = bs[:len(bs)-2]
	}
	body = cbody(append(after, body...), stmt)
	var main *cnode
	switch {
	case len(bs) == 0:
		main = cdo(body, multi)
	case len(bs) == 2 && len(body) == 1 && body[0].kind == 'a' && body[0].text == cname(bs[0]) && body[0].text != "" && !cprimHint(bs[0]) && (bs[0].kind != '^' || cmetable(bs[1]) || cliteral(bs[1])):
		main = bs[1] // (let [x v] x) is v; (let [^T x v] x) is ^T v, the hint kept
		if bs[0].kind == '^' && cmetable(bs[1]) {
			main = &cnode{kind: '^', text: bs[0].text, kids: []*cnode{bs[1]}, multi: bs[1].multi}
		}
	default:
		if len(body) == 0 {
			body = []*cnode{catom("nil")}
		}
		main = clist(multi, append([]*cnode{catom("let"), {kind: '[', kids: bs, multi: multi}}, body...)...)
	}
	if len(before) == 0 {
		return main
	}
	return cdo(cbody(append(before, main), stmt), multi)
}

// cmetable says n can carry a hint: a call or a symbol -- not a literal,
// which needs none, nor a special form, which drops it.
func cmetable(n *cnode) bool {
	switch n.kind {
	case '(':
		// a call: a special form, or a macro that expands to one, drops
		// its hint
		switch n.head() {
		case "", "if", "if-not", "let", "let*", "loop", "do", "case", "when", "when-not", "cond", "and", "or", "recur", "throw", "try", "fn", "fn*", "quote", "var", "set!":
			return false
		}
		return true
	case 'a':
		return !cliteral(n)
	}
	return false
}

// cliteral says n is a literal: a number, a string, a character, a keyword,
// nil, true or false.
func cliteral(n *cnode) bool {
	if n.kind != 'a' {
		return false
	}
	c := n.text[0]
	return c >= '0' && c <= '9' || c == '-' && len(n.text) > 1 && n.text[1] >= '0' && n.text[1] <= '9' ||
		c == '"' || c == '\\' || c == ':' || n.text == "nil" || n.text == "true" || n.text == "false"
}

// cprimHint says a binding's name carries a primitive's hint, which a form
// cannot: the let stays.
func cprimHint(n *cnode) bool {
	if n.kind != '^' {
		return false
	}
	switch n.text {
	case "^long", "^int", "^double", "^float", "^boolean", "^byte", "^short", "^char":
		return true
	}
	return false
}

// cwhen is (h test body), a do body spliced; as a statement, a when whose
// body does nothing is its test.
func cwhen(h string, test, body *cnode, stmt, multi bool) *cnode {
	forms := []*cnode{body}
	if body.head() == "do" {
		forms = body.kids[1:]
	}
	if stmt {
		forms = cbody(forms, true)
		if len(forms) == 0 {
			return test
		}
	}
	return clist(multi, append([]*cnode{catom(h), test}, forms...)...)
}

// cthrowsNoState says n is a state machine's default: (throw
// (IllegalStateException. "no state")).
func cthrowsNoState(n *cnode) bool {
	return n.head() == "throw" && len(n.kids) == 2 && n.kids[1].head() == "IllegalStateException." &&
		len(n.kids[1].kids) == 2 && n.kids[1].kids[1].isAtom(`"no state"`)
}
