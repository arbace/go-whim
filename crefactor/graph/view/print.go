package view

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// A Printer writes a view as Lisp.  With IDs, a list is `#ID(...)`, an
// atom with an id `#ID:atom`, a refers edge `@ID` after its node, and a
// link `@ID NAME`; without, names alone and a link `@NAME`.  Typed edges
// are not written: `whim graph` has them.
type Printer struct {
	IDs   bool
	Width int // the column a form is broken at; 0 is 100
}

// Elided is the atom a run of forms a context leaves out is written as.
const Elided = "..."

func (p Printer) width() int {
	if p.Width > 0 {
		return p.Width
	}
	return 100
}

// Tree writes t, and a newline.
func (p Printer) Tree(ix *Index, t *Tree) string {
	var b strings.Builder
	p.tree(&b, ix, t, 0)
	b.WriteByte('\n')
	return b.String()
}

func (p Printer) tree(b *strings.Builder, ix *Index, t *Tree, col int) {
	pad := strings.Repeat(" ", col)
	for _, n := range t.Notes {
		b.WriteString(pad + ";; " + n + "\n")
	}
	b.WriteString(pad)
	line := "(" + t.Head
	if t.Node != nil {
		line += " " + p.name(ix, t, t.Link)
	}
	at := col + len(line)
	var lb strings.Builder
	lb.WriteString(line)
	for _, f := range t.Inline {
		d := p.doc(f, nil)
		if d.flatLen() <= p.width()-at-1 {
			lb.WriteByte(' ')
			d.layout(&lb, at+1, p.width())
			at += 1 + d.flatLen()
		} else {
			lb.WriteString("\n" + pad + "  ")
			d.layout(&lb, col+2, p.width())
			at = p.width() // the rest goes below
		}
	}
	if t.Attrs != "" {
		lb.WriteString(" " + t.Attrs)
	}
	b.WriteString(lb.String())
	inner := strings.Repeat(" ", col+2)
	for _, c := range t.Contexts {
		b.WriteString("\n" + inner)
		d := p.context(ix, c)
		if c.Label != "" {
			d = &pdoc{list: true, kids: []*pdoc{{text: ":" + c.Label}, d}, head: ":" + c.Label}
		}
		d.layout(b, col+2, p.width())
	}
	for _, k := range t.Kids {
		b.WriteString("\n")
		p.tree(b, ix, k, col+2)
	}
	b.WriteString(")")
}

// name is how an entry names its node: `#ID NAME`, a link `@ID NAME`; or
// `NAME`, `@NAME`.  A name of more than one word -- `struct buf` -- is a
// list of them.
func (p Printer) name(ix *Index, t *Tree, link bool) string {
	n := t.Node
	s := t.Name
	if s == "" {
		s = ix.Name(n)
	}
	if strings.Contains(s, " ") {
		s = "(" + s + ")"
	}
	switch {
	case p.IDs && link:
		return "@" + idText(n.ID) + " " + s
	case p.IDs:
		return "#" + idText(n.ID) + " " + s
	case link:
		return "@" + s
	}
	return s
}

// Form writes one form, whole, laid out from column 0, and a newline.
func (p Printer) Form(n *graph.Node) string {
	var b strings.Builder
	p.doc(n, nil).layout(&b, 0, p.width())
	b.WriteByte('\n')
	return b.String()
}

// context is a context's form, what holds no use elided unless it is shown
// whole.
func (p Printer) context(ix *Index, c Context) *pdoc {
	if c.Whole {
		return p.doc(c.Form, nil)
	}
	path := map[*graph.Node]bool{}
	for _, u := range c.Uses {
		for n := u; n != nil; n = ix.Parent(n) {
			if path[n] {
				break
			}
			path[n] = true
			if n == c.Form {
				break
			}
		}
	}
	return p.doc(c.Form, path)
}

// collapsible are the forms whose elements a context shows only on the way
// to a use: a block's statements, an initialiser's elements, a type's
// members.
var collapsible = map[string]bool{"block": true, "init": true, "struct": true, "union": true, "enum": true, "defn": true, "stmt-expr": true}

// doc is n as a printable form: with path, the elements of a collapsible
// form off the path, and every block off it, a run of them one `...`.
func (p Printer) doc(n *graph.Node, path map[*graph.Node]bool) *pdoc {
	if !n.IsList() {
		return &pdoc{text: p.atom(n)}
	}
	d := &pdoc{list: true, head: n.Head()}
	if p.IDs && n.ID != 0 {
		d.pre = "#" + idText(n.ID)
	}
	if p.IDs {
		d.post = refsText(n)
	}
	elided := false
	for i, k := range n.Kids {
		off := path != nil && i > 0 && k.IsList() && !path[k] && (collapsible[n.Head()] || k.Is("block"))
		if off {
			if !elided {
				d.kids = append(d.kids, &pdoc{text: Elided})
			}
			elided = true
			continue
		}
		elided = false
		d.kids = append(d.kids, p.doc(k, path))
	}
	return d
}

func (p Printer) atom(n *graph.Node) string {
	if !p.IDs || n.ID == 0 {
		return n.Atom
	}
	return "#" + idText(n.ID) + ":" + n.Atom + refsText(n)
}

func refsText(n *graph.Node) string {
	s := ""
	for _, r := range n.Refs {
		s += "@" + idText(r.ID)
	}
	return s
}

// A pdoc is a form ready to lay out.
type pdoc struct {
	text      string // an atom's
	pre, post string // a list's marks
	head      string
	list      bool
	kids      []*pdoc
}

func (d *pdoc) flatLen() int {
	if !d.list {
		return len(d.text)
	}
	l := len(d.pre) + len(d.post) + 2
	for i, k := range d.kids {
		if i > 0 {
			l++
		}
		l += k.flatLen()
	}
	return l
}

func (d *pdoc) flat(b *strings.Builder) {
	if !d.list {
		b.WriteString(d.text)
		return
	}
	b.WriteString(d.pre + "(")
	for i, k := range d.kids {
		if i > 0 {
			b.WriteByte(' ')
		}
		k.flat(b)
	}
	b.WriteString(")" + d.post)
}

// broken says a form is always broken: a body is a statement a line, as
// C-lisp lays it out.
func (d *pdoc) broken() bool {
	if !d.list {
		return false
	}
	switch d.head {
	case "defn":
		return true
	case "block", "stmt-expr":
		return len(d.kids) > 2 || len(d.kids) == 2 && d.kids[1].broken()
	case "if", "while", "for", "do", "switch":
		for _, k := range d.kids[1:] {
			if k.broken() {
				return true
			}
		}
	}
	return false
}

// keep is how many arguments stay on a broken form's first line (C-lisp's
// rule).
func (d *pdoc) keep() int {
	atoms := 0
	for _, k := range d.kids[1:] {
		if k.list {
			break
		}
		atoms++
	}
	switch d.head {
	case "if", "while", "switch", "return", "case", "sizeof", "cast", "call", "index",
		".", "->", "=", "literal", "static_assert", "?", "fn":
		return 1
	case "for":
		return 3
	case "def", "typedef", "defn":
		return min(atoms+1, len(d.kids)-1)
	}
	return atoms
}

func (d *pdoc) layout(b *strings.Builder, col, width int) {
	if !d.list || !d.broken() && d.flatLen() <= width-col {
		d.flat(b)
		return
	}
	b.WriteString(d.pre + "(")
	if len(d.kids) == 0 {
		b.WriteString(")" + d.post)
		return
	}
	at := col + len(d.pre) + 1
	inner := col + 2
	first := d.kids[0]
	first.layout(b, at, width)
	k := 0
	if !first.list {
		k = d.keep()
		at += first.flatLen()
	} else {
		inner = col + 1
	}
	for i, x := range d.kids[1:] {
		if i < k && (x.flatLen() <= width-at-1 || i == 0 && !x.list) {
			b.WriteByte(' ')
			x.layout(b, at+1, width)
			at += 1 + x.flatLen()
			continue
		}
		k = 0
		b.WriteString("\n" + strings.Repeat(" ", inner))
		x.layout(b, inner, width)
	}
	b.WriteString(")" + d.post)
}
