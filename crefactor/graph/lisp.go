package graph

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// THE GRAPH AS LISP.  The containment view of the file, C-lisp's forms
// (crefactor/clisp/SPEC.md) with C's tokens as atoms, then the type nodes
// and the external nodes, each a top-level list of its own:
//
//	#12(call #13:f@7 #14:x@9)@:3     a list with id 12, typed by node 3;
//	                                  atoms 13 and 14, uses of nodes 7 and 9
//	(types #3(basic int) #4(pointer @:3) ...)
//	(externs #20(extern errno)@:3 ...)
//
// `#ID` before a node is its id -- before a list's `(`, and before an
// atom with a `:` between -- and every edge follows its node: `@ID` a
// refers edge, `@:ID` a typed edge.  A type's operand is an edge alone,
// `@:ID`.  A node with no id is a token of its form.  The four marks are
// here and nowhere else, so that the notation can change -- an id that is
// a content address, an EDN reader's tagged literals -- in one place.
const (
	markID    = '#' // #ID: the node's id
	markAtom  = ':' // #ID:atom
	markEdge  = '@' // @ID: a refers edge
	markTyped = ':' // @:ID: a typed edge
)

// Section heads: the type nodes and the external nodes, after the forms;
// and the last id the graph's IDs has given, `(ids N)`, last of all, so
// that an id an edit superseded is not given again by a graph read back:
// the greatest id the graph holds may be less than one it gave and removed.
const (
	typesHead   = "types"
	externsHead = "externs"
	idsHead     = "ids"
)

// A Lasting IDs says the last id it gave or saw (Sequential does): the Lisp
// writes it, and Read hands it to the IDs it makes.
type Lasting interface{ Last() ID }

// Width is the column a form is broken at.
const Width = 100

// Lisp writes the graph.
func (g *Graph) Lisp() []byte { return bytes.Join(g.LispUnits(), nil) }

// LispUnits is the graph's Lisp cut into its units, which concatenated are
// Lisp(): the opening comment, then a unit for each top-level node -- a
// form, a type node, an external node -- holding the space and the section
// head before it, then the sections' ends and `(ids N)`.  A unit is what a
// store of graphs keys by its bytes (Store): ids carry from phase to phase,
// so a form a phase did not touch is the same bytes before and after it.
func (g *Graph) LispUnits() [][]byte {
	var units [][]byte
	var b bytes.Buffer
	cut := func() {
		units = append(units, bytes.Clone(b.Bytes()))
		b.Reset()
	}
	b.WriteString(";; a C translation unit as a graph: crefactor/graph\n")
	for i, f := range g.Forms {
		cut()
		if i > 0 && !(f.Is("include") && g.Forms[i-1].Is("include")) {
			b.WriteByte('\n')
		}
		layout(&b, f, 0)
		b.WriteByte('\n')
	}
	for _, s := range []struct {
		head  string
		nodes []*Node
	}{{typesHead, g.Types}, {externsHead, g.Externs}} {
		cut()
		b.WriteString("\n(" + s.head)
		for _, n := range s.nodes {
			cut()
			b.WriteString("\n  ")
			layout(&b, n, 2)
		}
		b.WriteString(")\n")
	}
	if l, ok := g.ids.(Lasting); ok {
		b.WriteString("\n(" + idsHead + " " + strconv.FormatUint(uint64(l.Last()), 10) + ")\n")
	}
	cut()
	return units
}

// prefix and suffix are a node's marks.
func prefix(b *bytes.Buffer, n *Node) {
	if n.ID == 0 {
		return
	}
	b.WriteByte(markID)
	b.WriteString(strconv.FormatUint(uint64(n.ID), 10))
	if !n.list {
		b.WriteByte(markAtom)
	}
}

func suffix(b *bytes.Buffer, n *Node) {
	for _, r := range n.Refs {
		b.WriteByte(markEdge)
		b.WriteString(strconv.FormatUint(uint64(r.ID), 10))
	}
	if n.Type != nil {
		b.WriteByte(markEdge)
		b.WriteByte(markTyped)
		b.WriteString(strconv.FormatUint(uint64(n.Type.ID), 10))
	}
}

// flat writes n on one line.
func flat(b *bytes.Buffer, n *Node) {
	prefix(b, n)
	if n.list {
		b.WriteByte('(')
		for i, k := range n.Kids {
			if i > 0 {
				b.WriteByte(' ')
			}
			flat(b, k)
		}
		b.WriteByte(')')
	} else {
		b.WriteString(n.Atom)
	}
	suffix(b, n)
}

// fits says n is no wider than w on one line.
func fits(n *Node, w int) bool {
	return flatLen(n, w+1) <= w
}

// flatLen is n's width on one line, or more than limit.
func flatLen(n *Node, limit int) int {
	l := marksLen(n)
	if !n.list {
		return l + len(n.Atom)
	}
	l += 2 + len(n.Kids)
	for _, k := range n.Kids {
		if l > limit {
			return l
		}
		l += flatLen(k, limit-l)
	}
	return l
}

func marksLen(n *Node) int {
	l := 0
	if n.ID != 0 {
		l += 2 + digits(n.ID)
	}
	for _, r := range n.Refs {
		l += 1 + digits(r.ID)
	}
	if n.Type != nil {
		l += 2 + digits(n.Type.ID)
	}
	return l
}

func digits(id ID) int {
	d := 1
	for id >= 10 {
		id /= 10
		d++
	}
	return d
}

// broken says a form is always broken: a body is a statement a line.
func broken(n *Node) bool {
	switch n.Head() {
	case "defn":
		return true
	case "block", "stmt-expr":
		return len(n.Kids) > 2 || len(n.Kids) == 2 && n.Kids[1].list && broken(n.Kids[1])
	case "if", "while", "for", "do", "switch":
		for _, a := range n.Kids[1:] {
			if broken(a) {
				return true
			}
		}
	}
	return false
}

// keepArgs is how many of a broken form's arguments stay on its first line.
func keepArgs(n *Node) int {
	switch n.Head() {
	case "if", "while", "switch", "return", "case", "cast", "call", "=", "static_assert", "?":
		return 1
	case "for":
		return 3
	case "def", "typedef", "defn":
		if i := defNameAt(n); i > 0 {
			return i // the prefix and the name; the type below when it is long
		}
	}
	k := 0
	for _, a := range n.Args() {
		if a.list {
			break
		}
		k++
	}
	return k
}

func layout(b *bytes.Buffer, n *Node, col int) {
	if !n.list || !broken(n) && fits(n, Width-col) {
		flat(b, n)
		return
	}
	prefix(b, n)
	b.WriteByte('(')
	if len(n.Kids) == 0 {
		b.WriteByte(')')
		suffix(b, n)
		return
	}
	at := col + marksLen(n) + 1
	inner := col + 2
	first := n.Kids[0]
	layout(b, first, at)
	k := 0
	if !first.list {
		k = keepArgs(n)
		at += flatLen(first, Width)
	} else {
		inner = col + 1
	}
	for i, x := range n.Kids[1:] {
		if i < k && (fits(x, Width-at-1) || i == 0 && !x.list) {
			b.WriteByte(' ')
			layout(b, x, at+1)
			at += 1 + flatLen(x, Width)
			continue
		}
		k = 0
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", inner))
		layout(b, x, inner)
	}
	b.WriteByte(')')
	suffix(b, n)
}

// Read rebuilds a graph from its Lisp, parsing no C: the nodes with their
// ids, the containment, every edge.  The ids it holds are given; a fresh
// one is greater than all of them.
func Read(src []byte) (*Graph, error) {
	r := &reader{s: string(src), line: 1}
	r.slab = make([]Node, 0, len(src)/16)
	r.withID = make([]*Node, 0, bytes.Count(src, []byte{markID}))
	r.edges = make([]edge, 0, bytes.Count(src, []byte{markEdge}))
	r.refs = make([]*Node, 0, cap(r.edges))
	g := &Graph{}
	var last ID
	for {
		r.space()
		if r.i >= len(r.s) {
			break
		}
		if r.s[r.i] == '(' && r.word(idsHead) {
			r.i += 1 + len(idsHead)
			r.space()
			n, err := r.number()
			if err != nil {
				return nil, err
			}
			last = n
			r.space()
			if r.i >= len(r.s) || r.s[r.i] != ')' {
				return nil, r.errf("an unclosed (%s", idsHead)
			}
			r.i++
			continue
		}
		if r.s[r.i] == '(' && (r.word(typesHead) || r.word(externsHead)) {
			head := typesHead
			if r.word(externsHead) {
				head = externsHead
			}
			r.i += 1 + len(head)
			for {
				r.space()
				if r.i >= len(r.s) {
					return nil, r.errf("an unclosed (%s", head)
				}
				if r.s[r.i] == ')' {
					r.i++
					break
				}
				n, err := r.node()
				if err != nil {
					return nil, err
				}
				if head == typesHead {
					g.Types = append(g.Types, n)
				} else {
					g.Externs = append(g.Externs, n)
				}
			}
			continue
		}
		n, err := r.node()
		if err != nil {
			return nil, err
		}
		g.Forms = append(g.Forms, n)
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

type edge struct {
	slot **Node // where the target goes: the node's Type, or one of its Refs
	to   ID
	line int
}

type reader struct {
	s      string
	i      int
	line   int
	slab   []Node
	stack  []*Node
	refs   []*Node // the refers edges, allocated together
	tmp    []ID
	kids   []*Node // the lists' elements, allocated together
	withID []*Node
	edges  []edge
	maxID  ID
}

func (r *reader) errf(format string, a ...any) error {
	return fmt.Errorf("graph: line %d: %s", r.line, fmt.Sprintf(format, a...))
}

// word says the text at a `(` is that head and a space.
func (r *reader) word(w string) bool {
	j := r.i + 1
	return strings.HasPrefix(r.s[j:], w) && j+len(w) < len(r.s) && (isSpace(r.s[j+len(w)]) || r.s[j+len(w)] == ')')
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func (r *reader) space() {
	for r.i < len(r.s) {
		switch c := r.s[r.i]; c {
		case '\n':
			r.line++
			r.i++
		case ' ', '\t', '\r', '\f', '\v':
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

func (r *reader) alloc() *Node {
	if len(r.slab) == cap(r.slab) {
		r.slab = make([]Node, 0, 1<<14)
	}
	r.slab = r.slab[:len(r.slab)+1]
	return &r.slab[len(r.slab)-1]
}

func (r *reader) number() (ID, error) {
	start := r.i
	var v uint64
	for r.i < len(r.s) && r.s[r.i] >= '0' && r.s[r.i] <= '9' {
		v = v*10 + uint64(r.s[r.i]-'0')
		r.i++
	}
	if r.i == start || v == 0 || v > 1<<32-1 {
		return 0, r.errf("an id was expected")
	}
	return ID(v), nil
}

func (r *reader) node() (*Node, error) {
	n := r.alloc()
	if r.s[r.i] == markID {
		r.i++
		id, err := r.number()
		if err != nil {
			return nil, err
		}
		n.ID = id
		if id > r.maxID {
			r.maxID = id
		}
		r.withID = append(r.withID, n)
		if r.i < len(r.s) && r.s[r.i] == markAtom {
			r.i++
			if err := r.atom(n); err != nil {
				return nil, err
			}
			return n, r.suffix(n)
		}
		if r.i >= len(r.s) || r.s[r.i] != '(' {
			return nil, r.errf("#%d marks neither a list nor an atom", id)
		}
	}
	switch c := r.s[r.i]; {
	case c == '(':
		r.i++
		n.list = true
		base := len(r.stack)
		for {
			r.space()
			if r.i >= len(r.s) {
				return nil, r.errf("an unclosed list")
			}
			if r.s[r.i] == ')' {
				r.i++
				break
			}
			k, err := r.node()
			if err != nil {
				return nil, err
			}
			r.stack = append(r.stack, k)
		}
		if k := len(r.stack) - base; cap(r.kids)-len(r.kids) < k {
			r.kids = make([]*Node, 0, max(1<<16, k))
		}
		at := len(r.kids)
		r.kids = append(r.kids, r.stack[base:]...)
		n.Kids = r.kids[at:len(r.kids):len(r.kids)]
		r.stack = r.stack[:base]
	case c == ')':
		return nil, r.errf("an unexpected )")
	case c == markEdge:
		// an operand: an edge alone
	default:
		if err := r.atom(n); err != nil {
			return nil, err
		}
	}
	return n, r.suffix(n)
}

// suffix reads a node's edges.
func (r *reader) suffix(n *Node) error {
	refs := r.tmp[:0]
	for r.i < len(r.s) && r.s[r.i] == markEdge {
		r.i++
		typed := false
		if r.i < len(r.s) && r.s[r.i] == markTyped {
			typed = true
			r.i++
		}
		id, err := r.number()
		if err != nil {
			return err
		}
		if typed {
			r.edges = append(r.edges, edge{slot: &n.Type, to: id, line: r.line})
		} else {
			refs = append(refs, id)
		}
	}
	r.tmp = refs
	if len(refs) == 0 {
		return nil
	}
	if cap(r.refs)-len(r.refs) < len(refs) {
		r.refs = make([]*Node, 0, max(1<<12, len(refs)))
	}
	at := len(r.refs)
	r.refs = r.refs[:at+len(refs)]
	n.Refs = r.refs[at : at+len(refs) : at+len(refs)]
	for i, id := range refs {
		r.edges = append(r.edges, edge{slot: &n.Refs[i], to: id, line: r.line})
	}
	return nil
}

// atom reads C-lisp's atom -- up to a space, a parenthesis, `;` or an
// edge's mark, a quoted literal whole -- as a slice of the text.
func (r *reader) atom(n *Node) error {
	start := r.i
	for r.i < len(r.s) {
		c := r.s[r.i]
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v', '(', ')', ';', markEdge:
			n.Atom = r.s[start:r.i]
			return nil
		case '"', '\'':
			if c == '\'' && r.i > start && (r.s[start] >= '0' && r.s[start] <= '9' || r.s[start] == '.') {
				r.i++ // C23's digit separator, 1'000
				continue
			}
			r.i++
			for r.i < len(r.s) && r.s[r.i] != c {
				if r.s[r.i] == '\n' {
					return r.errf("a newline in a literal")
				}
				if r.s[r.i] == '\\' {
					r.i++
				}
				r.i++
			}
			if r.i >= len(r.s) {
				return r.errf("an unclosed literal")
			}
			r.i++
		default:
			r.i++
		}
	}
	n.Atom = r.s[start:r.i]
	return nil
}
