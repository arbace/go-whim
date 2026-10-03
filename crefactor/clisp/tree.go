package clisp

import "strings"

// THE TREE API.  The forms are a tree a program can search and edit in place,
// and print again with Print: what a phase does to canonical C text with
// regular expressions and counted line cuts, said on the forms instead.  It
// names nothing in any one program.
//
// A Cursor is a node and where it stands: the list that holds it, its index
// there, and the cursor of that list.  Walk hands one to its callbacks, Find
// returns them, and their edits -- Delete, Replace, InsertBefore,
// InsertAfter -- change the list in place.  A cursor finds its node again by
// identity when an edit elsewhere has moved it, so cursors collected by Find
// stay good while their siblings are cut, in any order.

// Root is a file's forms as one list, the parent of every top-level form: the
// root a Walk or a Find starts from.  Its List is the forms, as edited.
func Root(forms []*Node) *Node { return L(forms...) }

// A Cursor is a node in its place.
type Cursor struct {
	n    *Node
	in   *Node // the list holding n; nil for the root
	i    int
	up   *Cursor
	gone bool // deleted, or replaced: its node is no longer in the tree
	// inserted counts, for Walk, the nodes inserted after n and the nodes n
	// was replaced by, which the walk steps over rather than visits.
	after int
}

// Node is the cursor's node.
func (c *Cursor) Node() *Node { return c.n }

// Parent is the list that holds the node, nil at the root.
func (c *Cursor) Parent() *Node { return c.in }

// Up is the parent's cursor, nil at the root.
func (c *Cursor) Up() *Cursor { return c.up }

// Index is the node's position in its parent's list, found again if an edit
// moved it; -1 when it is no longer there.
func (c *Cursor) Index() int {
	if c.in == nil || c.gone {
		return -1
	}
	if c.i < len(c.in.List) && c.in.List[c.i] == c.n {
		return c.i
	}
	for j, x := range c.in.List {
		if x == c.n {
			c.i = j
			return j
		}
	}
	c.gone = true
	return -1
}

// Sibling is the node k places after this one in the same list (before it,
// for a negative k), or nil.
func (c *Cursor) Sibling(k int) *Node {
	i := c.Index()
	if i < 0 || i+k < 0 || i+k >= len(c.in.List) {
		return nil
	}
	return c.in.List[i+k]
}

// SiblingCursor is Sibling's cursor.
func (c *Cursor) SiblingCursor(k int) *Cursor {
	n := c.Sibling(k)
	if n == nil {
		return nil
	}
	return &Cursor{n: n, in: c.in, i: c.i + k, up: c.up}
}

// Enclosing is the nearest cursor above this one whose node is a list headed
// by head -- `defn`, `switch`, `block` -- or nil.
func (c *Cursor) Enclosing(head string) *Cursor {
	for u := c.up; u != nil; u = u.up {
		if u.n.Is(head) {
			return u
		}
	}
	return nil
}

// Function is the name of the definition the node is in, or "".
func (c *Cursor) Function() string {
	if d := c.Enclosing("defn"); d != nil {
		return DefName(d.n)
	}
	if c.n.Is("defn") {
		return DefName(c.n)
	}
	return ""
}

// IsItem says whether the node stands where a statement or a block-scope
// declaration does: in a block, a definition's body, a statement expression.
// An expression there is an expression statement.
func (c *Cursor) IsItem() bool {
	if c.in == nil {
		return false
	}
	switch c.in.Head() {
	case "block", "stmt-expr":
		return c.Index() > 0
	case "defn":
		i := c.Index()
		return i > 0 && i >= len(c.in.List)-len(Body(c.in))
	}
	return false
}

// Live says whether the node is still in the tree its walk started from:
// it, and every node above it, still in its place.
func (c *Cursor) Live() bool {
	for u := c; u != nil && u.in != nil; u = u.up {
		if u.Index() < 0 {
			return false
		}
	}
	return true
}

// Item is the cursor of the item -- the statement or block-scope
// declaration -- the node is, or is in; nil when it is in none.
func (c *Cursor) Item() *Cursor {
	for u := c; u != nil; u = u.up {
		if u.IsItem() {
			return u
		}
	}
	return nil
}

// Delete removes the node from its list.
func (c *Cursor) Delete() {
	i := c.Index()
	if i < 0 {
		return
	}
	c.in.List = append(c.in.List[:i], c.in.List[i+1:]...)
	c.gone = true
}

// Replace puts ns where the node is: none deletes it, several splice in.
func (c *Cursor) Replace(ns ...*Node) {
	i := c.Index()
	if i < 0 {
		return
	}
	l := make([]*Node, 0, len(c.in.List)-1+len(ns))
	l = append(l, c.in.List[:i]...)
	l = append(l, ns...)
	c.in.List = append(l, c.in.List[i+1:]...)
	c.gone = true
	c.after += len(ns)
}

// InsertBefore puts ns before the node.
func (c *Cursor) InsertBefore(ns ...*Node) {
	i := c.Index()
	if i < 0 {
		return
	}
	c.splice(i, ns)
	c.i += len(ns)
}

// InsertAfter puts ns after the node.
func (c *Cursor) InsertAfter(ns ...*Node) {
	i := c.Index()
	if i < 0 {
		return
	}
	c.splice(i+1, ns)
	c.after += len(ns)
}

func (c *Cursor) splice(at int, ns []*Node) {
	l := make([]*Node, 0, len(c.in.List)+len(ns))
	l = append(l, c.in.List[:at]...)
	l = append(l, ns...)
	c.in.List = append(l, c.in.List[at:]...)
}

// Walk visits root and every node under it, depth first and in order.  pre
// is called on the way down and says whether to go into the node; post, if
// not nil, on the way back up.  Either may edit the node at hand through its
// cursor -- delete it, replace it, insert beside it -- and the walk goes on
// with the node after it; the nodes it inserted or put in its place are not
// visited.
func Walk(root *Node, pre func(*Cursor) bool, post func(*Cursor)) {
	walk(&Cursor{n: root}, pre, post)
}

func walk(c *Cursor, pre func(*Cursor) bool, post func(*Cursor)) {
	if pre != nil && !pre(c) {
		return
	}
	if !c.gone && c.n.list {
		l := c.n
		for i := 0; i < len(l.List); {
			k := &Cursor{n: l.List[i], in: l, i: i, up: c}
			walk(k, pre, post)
			if k.gone {
				i = k.i + k.after
			} else {
				i = k.Index() + 1 + k.after
			}
		}
	}
	if post != nil && !c.gone {
		post(c)
	}
}

// Find is a cursor on every node under root (root included) that ok accepts,
// in order.  It goes into the nodes it accepts too.
func Find(root *Node, ok func(*Node) bool) []*Cursor {
	var out []*Cursor
	Walk(root, func(c *Cursor) bool {
		if ok(c.n) {
			out = append(out, c)
		}
		return true
	}, nil)
	return out
}

// FindIn is Find under the definition of the function name, at file scope in
// root: nil and false when there is none.
func FindIn(root *Node, name string, ok func(*Node) bool) ([]*Cursor, bool) {
	d := Definition(root, name)
	if d == nil {
		return nil, false
	}
	var out []*Cursor
	walk(d, func(c *Cursor) bool {
		if ok(c.n) {
			out = append(out, c)
		}
		return true
	}, nil)
	return out, true
}

// An Index is every atom under a root, by its text, as cursors: what a cut
// that starts from a name looks up instead of walking the whole tree, as the
// text verbs find a name's lines with bytes.Index before any regexp runs.
// Deleting keeps it good -- a cursor whose node was cut, or is under a cut
// node, is not Live and is not returned -- but a node an edit inserts or
// moves is not in it: a caller that inserts or moves indexes again.
type Index struct {
	atoms  map[string][]*Cursor
	quoted []*Cursor
}

// NewIndex indexes root, in one walk.
func NewIndex(root *Node) *Index {
	ix := &Index{atoms: map[string][]*Cursor{}}
	Walk(root, func(c *Cursor) bool {
		if !c.n.list {
			a := c.n.Atom
			if strings.ContainsAny(a, "\"'") {
				ix.quoted = append(ix.quoted, c)
			} else {
				ix.atoms[a] = append(ix.atoms[a], c)
			}
		}
		return true
	}, nil)
	return ix
}

// Atoms is the live cursors of the atoms spelled s, in order.
func (ix *Index) Atoms(s string) []*Cursor {
	var out []*Cursor
	for _, c := range ix.atoms[s] {
		if c.Live() {
			out = append(out, c)
		}
	}
	return out
}

// Mentions is Mentions(root, name, inLiterals), from the index.
func (ix *Index) Mentions(name string, inLiterals bool) int {
	n := len(ix.Atoms(name)) + len(ix.Atoms("."+name))
	if inLiterals {
		for _, c := range ix.quoted {
			if strings.Contains(c.n.Atom, name) && c.Live() {
				n += wordsIn(c.n.Atom, name)
			}
		}
	}
	return n
}

// Heads is Find of the lists headed by h.
func Heads(root *Node, h string) []*Cursor {
	return Find(root, func(n *Node) bool { return n.Is(h) })
}

// Atoms is Find of the atoms spelled s.
func Atoms(root *Node, s string) []*Cursor {
	return Find(root, func(n *Node) bool { return !n.list && n.Atom == s })
}

// Definition is a cursor on the function definition of that name among the
// top-level forms of root, or nil.
func Definition(root *Node, name string) *Cursor {
	rc := &Cursor{n: root}
	for i, f := range root.List {
		if f.Is("defn") && DefName(f) == name {
			return &Cursor{n: f, in: root, i: i, up: rc}
		}
	}
	return nil
}

// DefName is the declared name of a def, typedef or defn form: the first
// atom after its prefix.
func DefName(f *Node) string {
	if i := defNameAt(f); i >= 0 {
		return f.List[i].Atom
	}
	return ""
}

// defNameAt is the index in f.List of a def's name, or -1.
func defNameAt(f *Node) int {
	switch f.Head() {
	case "def", "typedef", "defn":
	default:
		return -1
	}
	for i := 1; i < len(f.List); i++ {
		a := f.List[i]
		if a.list {
			if isAttr(a) {
				continue
			}
			return -1
		}
		if !prefixWords[a.Atom] {
			return i
		}
	}
	return -1
}

// Prefix is a def's prefix: its storage class and the other words before its
// name, and attributes there.
func Prefix(f *Node) []*Node {
	if i := defNameAt(f); i > 0 {
		return f.List[1:i]
	}
	return nil
}

// HasPrefix says whether a def's prefix holds the word w.
func HasPrefix(f *Node, w string) bool {
	for _, a := range Prefix(f) {
		if !a.list && a.Atom == w {
			return true
		}
	}
	return false
}

// DefType is a def's declared type, or nil.
func DefType(f *Node) *Node {
	if i := defNameAt(f); i >= 0 && i+1 < len(f.List) {
		return f.List[i+1]
	}
	return nil
}

// DefValue is a def's value -- an expression or an (init ...) -- or nil;
// a defn has none.
func DefValue(f *Node) *Node {
	if f.Is("defn") {
		return nil
	}
	if r := defRest(f); len(r) == 1 {
		return r[0]
	}
	return nil
}

// defRest is what follows a def's name, type, attributes and asm label.
func defRest(f *Node) []*Node {
	i := defNameAt(f)
	if i < 0 || i+1 >= len(f.List) {
		return nil
	}
	i += 2
	for i < len(f.List) && isAttr(f.List[i]) {
		i++
	}
	if i < len(f.List) && f.List[i].Is("asm-label") {
		i++
	}
	return f.List[i:]
}

// Body is a definition's items, after its old-style parameter declarations
// if it has them.
func Body(f *Node) []*Node {
	if !f.Is("defn") {
		return nil
	}
	r := defRest(f)
	if len(r) > 0 && r[0].Is("kr-params") {
		r = r[1:]
	}
	return r
}

// Equal says whether two trees are the same, atom for atom.
func Equal(a, b *Node) bool {
	if a.list != b.list {
		return false
	}
	if !a.list {
		return a.Atom == b.Atom
	}
	if len(a.List) != len(b.List) {
		return false
	}
	for i := range a.List {
		if !Equal(a.List[i], b.List[i]) {
			return false
		}
	}
	return true
}

// Contains says whether t holds a subtree equal to x.
func Contains(t, x *Node) bool {
	found := false
	Walk(t, func(c *Cursor) bool {
		if found {
			return false
		}
		if Equal(c.n, x) {
			found = true
			return false
		}
		return true
	}, nil)
	return found
}

// Clone is a deep copy.
func Clone(n *Node) *Node {
	if !n.list {
		return A(n.Atom)
	}
	c := &Node{List: make([]*Node, len(n.List)), list: true}
	for i, x := range n.List {
		c.List[i] = Clone(x)
	}
	return c
}

// Mentions counts the places name is mentioned under root as `\bname\b`
// counts them in the C: an atom that is the name, a designator `.name`, and,
// when inLiterals, the name as a whole word inside a quoted atom (a string,
// a macro's text).
func Mentions(root *Node, name string, inLiterals bool) int {
	n := 0
	Walk(root, func(c *Cursor) bool {
		if c.n.list {
			return true
		}
		a := c.n.Atom
		switch {
		case a == name, len(a) == len(name)+1 && a[0] == '.' && a[1:] == name:
			n++
		case inLiterals && strings.ContainsAny(a, "\"'") && strings.Contains(a, name):
			n += wordsIn(a, name)
		}
		return true
	}, nil)
	return n
}

func wordsIn(s, w string) int {
	n := 0
	for pos := 0; ; {
		k := strings.Index(s[pos:], w)
		if k < 0 {
			return n
		}
		i := pos + k
		j := i + len(w)
		pos = i + 1
		if (i > 0 && identByte(s[i-1])) || (j < len(s) && identByte(s[j])) {
			continue
		}
		n++
	}
}

func identByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// FoldNever takes an `if` whose condition can no longer be true: without an
// else it goes, with one the else stands in its place -- a block's items
// spliced into the list around it, an `else if` the if.
func (c *Cursor) FoldNever() bool {
	if !c.n.Is("if") {
		return false
	}
	args := c.n.Args()
	switch {
	case len(args) == 2:
		c.Delete()
	case args[2].Is("block"):
		c.Replace(args[2].Args()...)
	default:
		c.Replace(args[2])
	}
	return true
}
