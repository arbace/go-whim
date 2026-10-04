package graph

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// THE GRAPH AS EDN.  A second serialisation, beside the Lisp (lisp.go), that
// Clojure's EDN reader (clojure.edn/read, with a reader function or
// tagged-literal for each tag) reads as it is: one map,
//
//	{:forms   [FORM ...]      the containment view of the file
//	 :types   [NODE ...]      the type nodes
//	 :externs [NODE ...]      the external nodes
//	 :ids     N}              the last id the graph's IDs gave
//
// A node with an id, or with edges, is a vector under the tag #g/n: its id,
// its form -- a list, or an atom -- and its edges, a refers edge #g/r ID and
// a typed edge #g/t ID; a type's operand, an edge alone, is #g/t ID; a node
// without either is its form bare:
//
//	#g/n [12 (call #g/n [13 f #g/r 7] #g/n [14 x #g/r 9]) #g/t 3]
//	#g/n [4 (pointer #g/t 3)]
//
// An atom -- C's token, or a C-lisp head -- is, by its spelling:
//
//	an EDN symbol      what EDN reads as one: identifiers, heads, most of C's
//	                   operators (`->`, `<<=`, `!=`, `.`, `...`)
//	true, false        C23's keywords, as EDN's booleans
//	an integer         a decimal constant EDN reads back as written: `26`
//	a string           a quoted atom, C's string literal or C-lisp's text
//	                   (`(include "<limits.h>")`), its content escaped as
//	                   EDN escapes a string
//	#c/num "0x7fUL"    any other number: hexadecimal, suffixed, floating
//	#c/char "'A'"      a character constant, its prefix included
//	#c/tok "||"        anything else, its text: `||`, `^=`, `L"wide"`
//
// Six tags, namespaced as EDN asks of tags that are not its own.  The marks
// are here and nowhere else.
const (
	ednNode = "#g/n"
	ednRef  = "#g/r"
	ednType = "#g/t"
	ednNum  = "#c/num"
	ednChar = "#c/char"
	ednTok  = "#c/tok"
)

// EDN writes the graph as EDN.
func (g *Graph) EDN() ([]byte, error) {
	var b bytes.Buffer
	w := &ednWriter{b: &b}
	b.WriteString(";; a C translation unit as a graph, in EDN: crefactor/graph\n")
	b.WriteString("{:forms\n [")
	for i, f := range g.Forms {
		if i > 0 {
			if !(f.Is("include") && g.Forms[i-1].Is("include")) {
				b.WriteByte('\n')
			}
			b.WriteString("\n  ")
		}
		w.layout(f, 2)
	}
	for _, s := range []struct {
		key   string
		nodes []*Node
	}{{typesHead, g.Types}, {externsHead, g.Externs}} {
		b.WriteString("]\n :" + s.key + "\n [")
		for i, n := range s.nodes {
			if i > 0 {
				b.WriteString("\n  ")
			}
			w.layout(n, 2)
		}
	}
	b.WriteString("]")
	if l, ok := g.ids.(Lasting); ok {
		b.WriteString("\n :" + idsHead + " " + strconv.FormatUint(uint64(l.Last()), 10))
	}
	b.WriteString("}\n")
	return b.Bytes(), w.err
}

type ednWriter struct {
	b   *bytes.Buffer
	err error
}

// operand says n is an edge alone: a type's operand.
func operand(n *Node) bool {
	return n.ID == 0 && !n.list && n.Atom == "" && len(n.Refs) == 0 && n.Type != nil
}

// wrapped says n is written as #g/n [ID FORM EDGE*].
func wrapped(n *Node) bool { return n.ID != 0 || len(n.Refs) > 0 || n.Type != nil }

func (w *ednWriter) open(n *Node) {
	w.b.WriteString(ednNode + " [")
	w.b.WriteString(strconv.FormatUint(uint64(n.ID), 10))
	w.b.WriteByte(' ')
}

func (w *ednWriter) close(n *Node) {
	for _, r := range n.Refs {
		w.b.WriteString(" " + ednRef + " ")
		w.b.WriteString(strconv.FormatUint(uint64(r.ID), 10))
	}
	if n.Type != nil {
		w.b.WriteString(" " + ednType + " ")
		w.b.WriteString(strconv.FormatUint(uint64(n.Type.ID), 10))
	}
	w.b.WriteByte(']')
}

func (w *ednWriter) flat(n *Node) {
	if operand(n) {
		w.b.WriteString(ednType + " ")
		w.b.WriteString(strconv.FormatUint(uint64(n.Type.ID), 10))
		return
	}
	if wrapped(n) {
		w.open(n)
	}
	if n.list {
		w.b.WriteByte('(')
		for i, k := range n.Kids {
			if i > 0 {
				w.b.WriteByte(' ')
			}
			w.flat(k)
		}
		w.b.WriteByte(')')
	} else if err := ednAtom(w.b, n.Atom); err != nil && w.err == nil {
		w.err = err
	}
	if wrapped(n) {
		w.close(n)
	}
}

// ednLen is n's width on one line, or more than limit.
func ednLen(n *Node, limit int) int {
	if operand(n) {
		return len(ednType) + 1 + digits(n.Type.ID)
	}
	l := 0
	if wrapped(n) {
		l = len(ednNode) + 3 + digits(n.ID)
		for _, r := range n.Refs {
			l += len(ednRef) + 2 + digits(r.ID)
		}
		if n.Type != nil {
			l += len(ednType) + 2 + digits(n.Type.ID)
		}
	}
	if !n.list {
		return l + ednAtomLen(n.Atom)
	}
	l += 2 + len(n.Kids)
	for _, k := range n.Kids {
		if l > limit {
			return l
		}
		l += ednLen(k, limit-l)
	}
	return l
}

// layout is the Lisp's (lisp.go): a form on one line when it fits and is
// not a body, else its head and the arguments keepArgs keeps, and the rest
// a line each, indented by two.
func (w *ednWriter) layout(n *Node, col int) {
	if !n.list || !broken(n) && ednLen(n, Width-col+1) <= Width-col {
		w.flat(n)
		return
	}
	at := col
	if wrapped(n) {
		w.open(n)
		at += len(ednNode) + 3 + digits(n.ID)
	}
	w.b.WriteByte('(')
	at++
	inner := col + 2
	if len(n.Kids) > 0 {
		first := n.Kids[0]
		w.layout(first, at)
		k := 0
		if !first.list {
			k = keepArgs(n)
			at += ednLen(first, Width)
		} else {
			inner = col + 1
		}
		for i, x := range n.Kids[1:] {
			if i < k && (ednLen(x, Width-at) <= Width-at-1 || i == 0 && !x.list) {
				w.b.WriteByte(' ')
				w.layout(x, at+1)
				at += 1 + ednLen(x, Width)
				continue
			}
			k = 0
			w.b.WriteByte('\n')
			w.b.WriteString(strings.Repeat(" ", inner))
			w.layout(x, inner)
		}
	}
	w.b.WriteByte(')')
	if wrapped(n) {
		w.close(n)
	}
}

// The atom's kinds, by its spelling.
const (
	atomSymbol = iota
	atomBool
	atomInt
	atomString
	atomNum
	atomChar
	atomTok
)

func atomKind(s string) int {
	switch {
	case s == "true" || s == "false":
		return atomBool
	case s == "":
		return atomTok
	case len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' && quotedOnce(s):
		return atomString
	case isDecimal(s):
		return atomInt
	case s[0] >= '0' && s[0] <= '9' || s[0] == '.' && len(s) > 1 && s[1] >= '0' && s[1] <= '9':
		return atomNum
	case isCharConst(s):
		return atomChar
	case ednSymbol(s):
		return atomSymbol
	}
	return atomTok
}

// quotedOnce says the quoted atom s is one literal: no unescaped quote
// inside it.
func quotedOnce(s string) bool {
	for i := 1; i < len(s)-1; i++ {
		switch s[i] {
		case '\\':
			i++
			if i == len(s)-1 {
				return false // the closing quote escaped
			}
		case '"':
			return false
		}
	}
	return true
}

// isDecimal says s is a decimal constant EDN reads back as written.
func isDecimal(s string) bool {
	if s == "0" {
		return true
	}
	if s[0] < '1' || s[0] > '9' || len(s) > 18 {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isCharConst(s string) bool {
	for _, p := range []string{"u8", "u", "U", "L"} {
		if strings.HasPrefix(s, p+"'") {
			s = s[len(p):]
			break
		}
	}
	return len(s) >= 3 && s[0] == '\'' && s[len(s)-1] == '\''
}

// ednSymbol says EDN, and Clojure's reader of it, read s as the symbol s:
// its characters EDN's, no namespace, not a number's start, not one of
// EDN's own words.
func ednSymbol(s string) bool {
	if s == "/" {
		return true
	}
	if s == "nil" || s == "true" || s == "false" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_':
		case c >= '0' && c <= '9':
			if i == 0 || i == 1 && (s[0] == '-' || s[0] == '+' || s[0] == '.') {
				return false
			}
		case strings.IndexByte("*+!-?$%&=<>.", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// ednAtom writes an atom.
func ednAtom(b *bytes.Buffer, s string) error {
	switch atomKind(s) {
	case atomSymbol, atomBool, atomInt:
		b.WriteString(s)
		return nil
	case atomString:
		return ednString(b, s[1:len(s)-1])
	case atomNum:
		b.WriteString(ednNum + " ")
	case atomChar:
		b.WriteString(ednChar + " ")
	default:
		b.WriteString(ednTok + " ")
	}
	return ednString(b, s)
}

func ednAtomLen(s string) int {
	switch atomKind(s) {
	case atomSymbol, atomBool, atomInt:
		return len(s)
	case atomString:
		return ednStringLen(s[1 : len(s)-1])
	case atomNum:
		return len(ednNum) + 1 + ednStringLen(s)
	case atomChar:
		return len(ednChar) + 1 + ednStringLen(s)
	}
	return len(ednTok) + 1 + ednStringLen(s)
}

// ednString writes s as an EDN string: `"` and `\` escaped, a control
// character refused, as an atom never holds one.
func ednString(b *bytes.Buffer, s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("graph: EDN: %q is not UTF-8", s)
	}
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c < ' ' || c == 0x7f:
			return fmt.Errorf("graph: EDN: %q holds a control character", s)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return nil
}

func ednStringLen(s string) int {
	return 2 + len(s) + strings.Count(s, `"`) + strings.Count(s, `\`)
}

// ReadEDN rebuilds a graph from its EDN, as Read does from its Lisp: the
// nodes with their ids, the containment, every edge.  It reads what EDN
// writes, not every EDN: one map of the four keys, the six tags.
func ReadEDN(src []byte) (*Graph, error) {
	r := &ednReader{s: string(src), line: 1}
	r.slab = make([]Node, 0, len(src)/24)
	r.edges = make([]edge, 0, bytes.Count(src, []byte(ednRef))+bytes.Count(src, []byte(ednType)))
	g := &Graph{}
	var last ID
	r.space()
	if !r.peek('{') {
		return nil, r.errf("a map was expected")
	}
	r.i++
	for {
		r.space()
		if r.peek('}') {
			r.i++
			break
		}
		if !r.peek(':') {
			return nil, r.errf("a key was expected")
		}
		key := r.token()
		r.space()
		switch key {
		case ":forms", ":" + typesHead, ":" + externsHead:
			ns, err := r.vector()
			if err != nil {
				return nil, err
			}
			switch key {
			case ":forms":
				g.Forms = ns
			case ":" + typesHead:
				g.Types = ns
			default:
				g.Externs = ns
			}
		case ":" + idsHead:
			n, err := r.number()
			if err != nil {
				return nil, err
			}
			last = n
		default:
			return nil, r.errf("an unknown key %s", key)
		}
	}
	r.space()
	if r.i < len(r.s) {
		return nil, r.errf("text after the map")
	}
	byID := make([]*Node, r.maxID+1)
	for _, n := range r.withID {
		if byID[n.ID] != nil {
			return nil, fmt.Errorf("graph: id %d is two nodes", n.ID)
		}
		byID[n.ID] = n
	}
	for _, e := range r.edges {
		var t *Node
		if int(e.to) < len(byID) {
			t = byID[e.to]
		}
		if t == nil {
			return nil, fmt.Errorf("graph: line %d: an edge to %d, which no node is", e.line, e.to)
		}
		*e.slot = t
	}
	seq := &Sequential{}
	seq.Saw(r.maxID)
	seq.Saw(last)
	g.ids = seq
	return g, nil
}

type ednReader struct {
	s      string
	i      int
	line   int
	slab   []Node
	stack  []*Node
	kids   []*Node
	refs   []*Node
	tmp    []ID
	withID []*Node
	edges  []edge
	maxID  ID
}

func (r *ednReader) errf(format string, a ...any) error {
	return fmt.Errorf("graph: EDN: line %d: %s", r.line, fmt.Sprintf(format, a...))
}

func (r *ednReader) peek(c byte) bool { return r.i < len(r.s) && r.s[r.i] == c }

func (r *ednReader) space() {
	for r.i < len(r.s) {
		switch r.s[r.i] {
		case '\n':
			r.line++
			r.i++
		case ' ', '\t', '\r', ',':
			r.i++
		case ';':
			for r.i < len(r.s) && r.s[r.i] != '\n' {
				r.i++
			}
		default:
			return
		}
	}
}

// token is the text up to a delimiter.
func (r *ednReader) token() string {
	start := r.i
	for r.i < len(r.s) {
		switch r.s[r.i] {
		case ' ', '\t', '\n', '\r', ',', '(', ')', '[', ']', '{', '}', '"', ';':
			return r.s[start:r.i]
		}
		r.i++
	}
	return r.s[start:r.i]
}

func (r *ednReader) number() (ID, error) {
	t := r.token()
	v, err := strconv.ParseUint(t, 10, 32)
	if err != nil {
		return 0, r.errf("an id was expected, not %q", t)
	}
	return ID(v), nil
}

func (r *ednReader) alloc() *Node {
	if len(r.slab) == cap(r.slab) {
		r.slab = make([]Node, 0, 1<<14)
	}
	r.slab = r.slab[:len(r.slab)+1]
	return &r.slab[len(r.slab)-1]
}

// seq reads elements up to the closing c into the stack, from base.
func (r *ednReader) seq(c byte) error {
	for {
		r.space()
		if r.i >= len(r.s) {
			return r.errf("an unclosed %c", c)
		}
		if r.s[r.i] == c {
			r.i++
			return nil
		}
		n, err := r.element()
		if err != nil {
			return err
		}
		r.stack = append(r.stack, n)
	}
}

func (r *ednReader) vector() ([]*Node, error) {
	if !r.peek('[') {
		return nil, r.errf("a vector was expected")
	}
	r.i++
	base := len(r.stack)
	if err := r.seq(']'); err != nil {
		return nil, err
	}
	ns := append([]*Node(nil), r.stack[base:]...)
	r.stack = r.stack[:base]
	return ns, nil
}

// kidsOf moves the stack's elements from base into the node's slab.
func (r *ednReader) kidsOf(n *Node, base int) {
	k := len(r.stack) - base
	if cap(r.kids)-len(r.kids) < k {
		r.kids = make([]*Node, 0, max(1<<16, k))
	}
	at := len(r.kids)
	r.kids = append(r.kids, r.stack[base:]...)
	n.Kids = r.kids[at:len(r.kids):len(r.kids)]
	r.stack = r.stack[:base]
}

// element reads a form: a list, a #g/n node, a #g/t operand, or an atom.
func (r *ednReader) element() (*Node, error) {
	switch c := r.s[r.i]; c {
	case '(':
		r.i++
		n := r.alloc()
		n.list = true
		base := len(r.stack)
		if err := r.seq(')'); err != nil {
			return nil, err
		}
		r.kidsOf(n, base)
		return n, nil
	case ')', ']', '}', '[', '{':
		return nil, r.errf("an unexpected %c", c)
	case '"':
		s, err := r.str()
		if err != nil {
			return nil, err
		}
		n := r.alloc()
		n.Atom = s
		return n, nil
	case '#':
		tag := r.token()
		r.space()
		switch tag {
		case ednNode:
			return r.node()
		case ednType:
			n := r.alloc()
			id, err := r.number()
			if err != nil {
				return nil, err
			}
			r.edges = append(r.edges, edge{slot: &n.Type, to: id, line: r.line})
			return n, nil
		case ednNum, ednChar, ednTok:
			if !r.peek('"') {
				return nil, r.errf("%s takes a string", tag)
			}
			s, err := r.str()
			if err != nil {
				return nil, err
			}
			n := r.alloc()
			n.Atom = s[1 : len(s)-1]
			return n, nil
		}
		return nil, r.errf("an unknown tag %s", tag)
	}
	n := r.alloc()
	n.Atom = r.token()
	if n.Atom == "" {
		return nil, r.errf("an empty token")
	}
	return n, nil
}

// node reads #g/n's vector: [ID FORM EDGE*].
func (r *ednReader) node() (*Node, error) {
	if !r.peek('[') {
		return nil, r.errf("%s takes a vector", ednNode)
	}
	r.i++
	r.space()
	id64, err := strconv.ParseUint(r.token(), 10, 32)
	if err != nil {
		return nil, r.errf("%s: an id was expected", ednNode)
	}
	r.space()
	if r.i >= len(r.s) {
		return nil, r.errf("an unclosed %s", ednNode)
	}
	n, err := r.element()
	if err != nil {
		return nil, err
	}
	if id := ID(id64); id != 0 {
		n.ID = id
		r.maxID = max(r.maxID, id)
		r.withID = append(r.withID, n)
	}
	refs := r.tmp[:0]
	for {
		r.space()
		if r.peek(']') {
			r.i++
			break
		}
		tag := r.token()
		r.space()
		to, err := r.number()
		if err != nil {
			return nil, err
		}
		switch tag {
		case ednRef:
			refs = append(refs, to)
		case ednType:
			r.edges = append(r.edges, edge{slot: &n.Type, to: to, line: r.line})
		default:
			return nil, r.errf("%s: an edge was expected, not %q", ednNode, tag)
		}
	}
	r.tmp = refs
	if len(refs) > 0 {
		if cap(r.refs)-len(r.refs) < len(refs) {
			r.refs = make([]*Node, 0, max(1<<12, len(refs)))
		}
		at := len(r.refs)
		r.refs = r.refs[:at+len(refs)]
		n.Refs = r.refs[at : at+len(refs) : at+len(refs)]
		for i, id := range refs {
			r.edges = append(r.edges, edge{slot: &n.Refs[i], to: id, line: r.line})
		}
	}
	return n, nil
}

// str reads an EDN string and returns it quoted, as the atom of a quoted
// literal is: a slice of the text when it holds no escape.
func (r *ednReader) str() (string, error) {
	start := r.i
	r.i++
	esc := false
	for r.i < len(r.s) && r.s[r.i] != '"' {
		switch r.s[r.i] {
		case '\\':
			esc = true
			r.i++
		case '\n':
			return "", r.errf("a newline in a string")
		}
		r.i++
	}
	if r.i >= len(r.s) {
		return "", r.errf("an unclosed string")
	}
	r.i++
	if !esc {
		return r.s[start:r.i], nil
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := start + 1; i < r.i-1; i++ {
		c := r.s[i]
		if c == '\\' {
			i++
			switch c = r.s[i]; c {
			case '"', '\\':
			case 'n':
				c = '\n'
			case 't':
				c = '\t'
			case 'r':
				c = '\r'
			default:
				return "", r.errf("an escape EDN writes not: \\%c", c)
			}
		}
		b.WriteByte(c)
	}
	b.WriteByte('"')
	return b.String(), nil
}
