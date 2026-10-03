package clisp

import (
	"fmt"
	"strings"
)

// A Node is an s-expression: an atom, or a list of nodes.
//
// AN ATOM IS C'S OWN TOKEN TEXT.  An identifier, a keyword, a number, a
// character or string literal -- prefixes and escapes included -- is one atom
// spelled exactly as the C spells it, so nothing is decoded on the way in and
// nothing re-encoded on the way out: `"a\tb"`, `'\n'`, `L"x"` and `0x7fUL` are
// atoms, and so are the operator heads (`+`, `->`, `<<=`).
type Node struct {
	Atom string
	List []*Node
	list bool
}

// A makes an atom.
func A(s string) *Node { return &Node{Atom: s} }

// L makes a list.
func L(items ...*Node) *Node { return &Node{List: items, list: true} }

// IsList reports whether n is a list (the empty list included).
func (n *Node) IsList() bool { return n != nil && n.list }

// Head is a list's first element when it is an atom, else "".
func (n *Node) Head() string {
	if n == nil || !n.list || len(n.List) == 0 || n.List[0].list {
		return ""
	}
	return n.List[0].Atom
}

// Is reports whether n is a list headed by the atom h.
func (n *Node) Is(h string) bool { return n.Head() == h }

// Args is a list's elements after its head.
func (n *Node) Args() []*Node {
	if n == nil || !n.list || len(n.List) == 0 {
		return nil
	}
	return n.List[1:]
}

func (n *Node) add(items ...*Node) *Node {
	n.List = append(n.List, items...)
	return n
}

// String is n on one line.
func (n *Node) String() string {
	var b strings.Builder
	n.flat(&b)
	return b.String()
}

func (n *Node) flat(b *strings.Builder) {
	if !n.list {
		b.WriteString(n.Atom)
		return
	}
	b.WriteByte('(')
	for i, x := range n.List {
		if i > 0 {
			b.WriteByte(' ')
		}
		x.flat(b)
	}
	b.WriteByte(')')
}

// Read parses s-expression text into its top-level forms.  A `;` outside a
// literal starts a comment that runs to the end of the line.
func Read(src []byte) ([]*Node, error) {
	r := &reader{src: src, line: 1}
	var out []*Node
	for {
		r.space()
		if r.i >= len(r.src) {
			return out, nil
		}
		n, err := r.node()
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
}

type reader struct {
	src  []byte
	i    int
	line int
}

func (r *reader) errf(format string, a ...any) error {
	return fmt.Errorf("clisp: line %d: %s", r.line, fmt.Sprintf(format, a...))
}

// space skips whitespace and comments.
func (r *reader) space() {
	for r.i < len(r.src) {
		switch c := r.src[r.i]; c {
		case '\n':
			r.line++
			r.i++
		case ' ', '\t', '\r', '\f', '\v':
			r.i++
		case ';':
			for r.i < len(r.src) && r.src[r.i] != '\n' {
				r.i++
			}
		default:
			return
		}
	}
}

func (r *reader) node() (*Node, error) {
	switch r.src[r.i] {
	case '(':
		r.i++
		n := L()
		for {
			r.space()
			if r.i >= len(r.src) {
				return nil, r.errf("unclosed list")
			}
			if r.src[r.i] == ')' {
				r.i++
				return n, nil
			}
			x, err := r.node()
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, x)
		}
	case ')':
		return nil, r.errf("unexpected )")
	}
	return r.atom()
}

// atom reads up to whitespace or a parenthesis, a quoted literal inside it
// whole: `'('`, `"a b"` and `L"x"` are one atom each.
func (r *reader) atom() (*Node, error) {
	start := r.i
	for r.i < len(r.src) {
		c := r.src[r.i]
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v', '(', ')', ';':
			return A(string(r.src[start:r.i])), nil
		case '"', '\'':
			if c == '\'' && r.i > start && (r.src[start] >= '0' && r.src[start] <= '9' || r.src[start] == '.') {
				r.i++ // C23's digit separator inside a number, 1'000
				break
			}
			r.i++
			for r.i < len(r.src) && r.src[r.i] != c {
				if r.src[r.i] == '\n' {
					return nil, r.errf("a newline in a literal")
				}
				if r.src[r.i] == '\\' {
					r.i++
				}
				r.i++
			}
			if r.i >= len(r.src) {
				return nil, r.errf("unclosed literal")
			}
			r.i++
		default:
			r.i++
		}
	}
	return A(string(r.src[start:r.i])), nil
}

// Width is the column a form is broken at.
const Width = 100

// Format lays forms out one top-level form after another, a blank line
// between them but between two includes; a form fits on its line or is broken
// with its arguments one per line, two columns in.
func Format(forms []*Node) []byte {
	var b strings.Builder
	for i, f := range forms {
		if i > 0 && !(f.Is("include") && forms[i-1].Is("include")) {
			b.WriteByte('\n')
		}
		layout(&b, f, 0)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// keep is how many arguments stay on a broken form's first line: the ones
// that say what the form is -- a condition, a name, the declared type --
// before what it holds.
func keep(n *Node) int {
	args := n.Args()
	atoms := 0
	for atoms < len(args) && !args[atoms].list {
		atoms++
	}
	switch n.Head() {
	case "if", "while", "switch", "return", "case", "sizeof", "cast", "call", "index",
		".", "->", "=", "literal", "static_assert", "?", "fn":
		return 1
	case "for":
		return 3
	case "def", "typedef":
		// the prefix and the name, and the type: the value goes below.
		return min(atoms+1, len(args))
	case "defn":
		return min(atoms+1, len(args))
	case "struct", "union", "enum":
		return atoms
	}
	return atoms
}

// broken reports whether a form is always broken: a body of statements is a
// statement a line, however short.
func broken(n *Node) bool {
	switch n.Head() {
	case "defn":
		return true
	case "block":
		return len(n.Args()) > 1 || (len(n.Args()) == 1 && n.Args()[0].list && broken(n.Args()[0]))
	case "if", "while", "for", "do", "switch", "stmt-expr":
		for _, a := range n.Args() {
			if broken(a) {
				return true
			}
		}
	}
	return false
}

func layout(b *strings.Builder, n *Node, col int) {
	if !n.list {
		b.WriteString(n.Atom)
		return
	}
	flat := n.String()
	if col+len(flat) <= Width && !broken(n) && !strings.Contains(flat, "\n") {
		b.WriteString(flat)
		return
	}
	if len(n.List) == 0 {
		b.WriteString("()")
		return
	}
	b.WriteByte('(')
	at := col + 1
	first := n.List[0]
	layout(b, first, at)
	inner := col + 2
	rest := n.List[1:]
	k := 0
	if !first.list {
		k = keep(n)
		at += len(first.Atom)
	} else {
		// a list of lists -- parameters, a spec list -- aligns its elements
		// under the first.
		inner = col + 1
	}
	for i, x := range rest {
		// A KEPT ARGUMENT THAT DOES NOT FIT ENDS THE FIRST LINE: it is laid out
		// below, two columns in, rather than broken far to the right.
		if s := x.String(); i < k && (at+1+len(s) <= Width || i == 0 && !x.list) {
			b.WriteByte(' ')
			layout(b, x, at+1)
			at += 1 + len(s)
			continue
		}
		k = 0
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", inner))
		layout(b, x, inner)
	}
	b.WriteByte(')')
}
