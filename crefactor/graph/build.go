package graph

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// BUILD (doc/GRAPH-MIGRATION.md, B0): small new nodes made with their edges,
// so that a cut can say what it puts in -- `return FALSE;`, `x = curbuf;`,
// a call of an existing function -- as the text verbs said it in C.  A
// TEMPLATE is C-lisp's forms (crefactor/clisp/SPEC.md), read and made nodes
// in the context of a PLACE in the graph:
//
//   - every ordinary identifier is a use of the declaration visible there --
//     a local or a parameter of the enclosing function declared before the
//     place, else the file's first declaration of the name above it (as
//     the importer resolves a use: cc's check takes the first declarator),
//     an enumerator, an external -- or of a `def` the template itself makes
//     before it; a member is resolved BY THE TYPE of what it selects from,
//     a tag to its definition, a goto's label to the function's label;
//   - `?name` is a HOLE: the node a pattern bound to name, moved in (a node
//     is contained once, so a hole is used once; a copy is CLONE's, B2a);
//   - each new expression form gets its typed edge where the type follows
//     from what it is made of -- a call its callee's result, a comparison
//     int, an assignment its left side's, a selection its member's, a cast
//     its type's when the graph holds that type node -- and otherwise none,
//     and is then listed in Untyped (step 4's rule: unknown, never wrong);
//   - the ids are the editor's to give: a built node has none until the
//     edit that puts it in the graph gives it a fresh one.
//
// What it refuses is FRAG's (B2a): a name declared nowhere visible, a
// macro's invocation, a compound literal, an initialiser, a function type.

// Build reads src, one or more C-lisp forms, as the nodes they make at the
// place of at -- where at stands, which the nodes will take or stand beside:
// a name declared by at itself, or after it, is not visible.  holes are the
// nodes `?name` stands for.
func (e *Editor) Build(at *Node, src string, holes Bindings) ([]*Node, error) {
	p, i := e.index(at)
	if i < 0 {
		return nil, fmt.Errorf("build at #%d: not in the graph", at.ID)
	}
	return e.buildAt(p, i, src, holes)
}

// BuildIn is Build at the end of the items of p: a block, a function's body.
func (e *Editor) BuildIn(p *Node, src string, holes Bindings) ([]*Node, error) {
	if !e.Live(p) {
		return nil, fmt.Errorf("build in #%d: not in the graph", p.ID)
	}
	return e.buildAt(p, len(p.Kids), src, holes)
}

func (e *Editor) buildAt(p *Node, i int, src string, holes Bindings) ([]*Node, error) {
	forms, err := clisp.Read([]byte(src))
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	b := &builder{e: e, p: p, i: i, holes: holes, used: map[string]bool{}, local: map[string]*Node{}}
	var out []*Node
	for _, f := range forms {
		n := b.item(f)
		if b.err != nil {
			return nil, fmt.Errorf("build %s: %w", f, b.err)
		}
		out = append(out, n)
	}
	for _, n := range b.untyped {
		if n.Type == nil {
			e.untype(n)
		}
	}
	return out, nil
}

// RefTo is a use of the declaration d: an atom spelling its name, with its
// refers edge.
func (e *Editor) RefTo(d *Node) *Node {
	return &Node{Atom: ordinaryName(d), Refs: []*Node{d}}
}

// Return is `return x;`, or `return;` for x nil.
func Return(x *Node) *Node {
	if x == nil {
		return NewList(NewAtom("return"))
	}
	return NewList(NewAtom("return"), x)
}

// Break is `break;`.
func Break() *Node { return NewList(NewAtom("break")) }

// Void is `(void)x`.
func Void(x *Node) *Node { return NewList(NewAtom("cast"), NewAtom("void"), x) }

// Call is `f(args...)`, f an expression (a use of a function, or of a
// pointer to one), typed with the function's result when f's type says it.
func (e *Editor) Call(f *Node, args ...*Node) *Node {
	n := NewList(append([]*Node{NewAtom("call"), f}, args...)...)
	n.Type = resultType(typeOf(f))
	return n
}

// Resolve is the declaration the ordinary identifier name, written where at
// stands, would refer to: nil when none is visible.
func (e *Editor) Resolve(at *Node, name string) *Node {
	p, i := e.index(at)
	if i < 0 {
		return nil
	}
	return e.resolveAt(p, i, name)
}

// resolveAt is the declaration name refers to at element i of p: the
// enclosing scopes' declarations before it, innermost first, then the
// file's.
func (e *Editor) resolveAt(p *Node, i int, name string) *Node {
	for p != nil && !e.isTop(p) {
		switch {
		case p.Is("block") || p.Is("stmt-expr") || p.Is("defn") && i >= defnItemsAt(p):
			for j := min(i, len(p.Kids)) - 1; j >= 1; j-- {
				if k := p.Kids[j]; (k.Is("def") || k.Is("typedef")) && topName(k) == name {
					return k
				}
			}
		case p.Is("for") && i > 1 && p.Kids[1].Is("def") && topName(p.Kids[1]) == name:
			return p.Kids[1]
		}
		if p.Is("defn") {
			if prm := paramNamed(p, name); prm != nil {
				return prm
			}
		}
		p, i = e.index(p)
	}
	if i < 0 {
		return nil
	}
	// the file: the first declaration of the name, at or above the form
	// the place is in (a function's own name is visible in its body)
	forms := e.g.Forms
	for j := 0; j < len(forms) && j <= i; j++ {
		f := forms[j]
		if topName(f) == name && (j < i || f.Is("defn")) {
			return f
		}
		if j < i {
			if en := enumeratorIn(f, name); en != nil {
				return en
			}
		}
	}
	for _, x := range e.g.Externs {
		if (x.Is("extern") || x.Is("extern-typedef") || x.Is("extern-enumerator") || x.Is("undeclared")) &&
			len(x.Kids) > 1 && x.Kids[1].Atom == name {
			return x
		}
	}
	return nil
}

// paramNamed is the parameter of the function definition f named name.
func paramNamed(f *Node, name string) *Node {
	t := defType(f)
	if t == nil || !t.Is("fn") || len(t.Kids) < 2 {
		return nil
	}
	for _, prm := range t.Kids[1].Kids {
		if prm.list && len(prm.Kids) >= 2 && !prm.Kids[0].list && prm.Kids[0].Atom == name {
			return prm
		}
	}
	return nil
}

// enumeratorIn is the enumerator name of an enum f declares, outside a
// function's body.
func enumeratorIn(f *Node, name string) *Node {
	if f.Is("defn") {
		return nil
	}
	var out *Node
	Walk(f, func(n *Node) bool {
		if out != nil {
			return false
		}
		if n.Is("enum") {
			for _, en := range body(n) {
				if en.list && len(en.Kids) > 0 && !en.Kids[0].list && en.Kids[0].Atom == name {
					out = en
				}
			}
		}
		return true
	})
	return out
}

// ordinaryName is the name a declaration declares in C's ordinary name
// space -- a def's, typedef's or defn's, a parameter's, an enumerator's, an
// external's -- or "" for a member, a tag, a label.
func ordinaryName(d *Node) string {
	switch h := d.Head(); {
	case h == "def" || h == "typedef" || h == "defn":
		return topName(d)
	case h == "extern" || h == "extern-typedef" || h == "extern-enumerator" || h == "undeclared":
		if len(d.Kids) > 1 {
			return d.Kids[1].Atom
		}
		return ""
	case h == "label" || h == "member" || strings.HasPrefix(h, "extern-") || h == "unresolved-member" ||
		h == "struct" || h == "union" || h == "enum":
		return ""
	}
	if d.list && len(d.Kids) > 0 && !d.Kids[0].list {
		// a parameter, an enumerator -- or a member: the caller's to tell apart
		return d.Kids[0].Atom
	}
	return ""
}

// builder makes a template's nodes at one place.
type builder struct {
	e       *Editor
	p       *Node
	i       int
	holes   Bindings
	used    map[string]bool
	local   map[string]*Node // the template's own defs, by name
	untyped []*Node
	err     error
}

func (b *builder) fail(format string, a ...any) *Node {
	if b.err == nil {
		b.err = fmt.Errorf(format, a...)
	}
	return NewAtom("?")
}

// keywords are C's, and the C-lisp atoms that are tokens in a form's
// argument places: never a name to resolve.
var keywords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`auto break case char const continue default do double else enum
		extern float for goto if inline int long register restrict return short signed sizeof static
		struct switch typedef union unsigned void volatile while _Bool _Complex _Imaginary _Noreturn
		_Thread_local _Atomic _Alignas _Alignof _Static_assert _Generic bool true false nullptr
		alignas alignof constexpr static_assert thread_local typeof typeof_unqual __inline __inline__
		__restrict __restrict__ __volatile__ __const __signed__ __attribute__ __extension__ __int128
		usize`) {
		keywords[w] = true
	}
}

func isIdent(s string) bool {
	if s == "" || keywords[s] {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

// hole is the node ?name stands for, used once.
func (b *builder) hole(a string) *Node {
	name := a[1:]
	n, ok := b.holes[name]
	if !ok {
		return b.fail("the hole %s is bound to nothing", a)
	}
	if b.used[name] {
		return b.fail("the hole %s is used twice: a node is contained once (a copy is CLONE's)", a)
	}
	b.used[name] = true
	return n
}

// name is a use of the ordinary identifier a.
func (b *builder) name(a string) *Node {
	d := b.local[a]
	if d == nil {
		d = b.e.resolveAt(b.p, b.i, a)
	}
	if d == nil {
		return b.fail("`%s` is declared nowhere visible from the place", a)
	}
	return &Node{Atom: a, Refs: []*Node{d}}
}

// atom is an atom in an expression's place.
func (b *builder) atom(a string) *Node {
	switch {
	case len(a) > 1 && a[0] == '?':
		return b.hole(a)
	case a == "_" || a == "_*":
		return b.fail("a pattern's %s in a template", a)
	case isIdent(a):
		return b.name(a)
	}
	return NewAtom(a)
}

// stmtKids are the statement forms whose elements are items or expressions,
// all of them; exprHeads the expression forms whose operands are.
var (
	stmtKids  = map[string]bool{"block": true, "if": true, "while": true, "do": true, "switch": true, "return": true, "case": true, "default": true, "break": true, "continue": true, "empty": true, "for": true}
	exprHeads = map[string]bool{}
)

func init() {
	for _, h := range strings.Fields(`+ - * / % << >> < > <= >= == != & ^ | && || ! ~ = += -= *= /= %= <<= >>= &= ^= |=
		? comma paren addr deref index call post++ post-- pre++ pre-- sizeof sizeof-bare alignof alignof-bare`) {
		exprHeads[h] = true
	}
}

// item is a statement, a declaration or an expression.
func (b *builder) item(f *clisp.Node) *Node {
	if !f.IsList() {
		return b.atom(f.Atom)
	}
	h := f.Head()
	switch {
	case h == "def":
		return b.def(f)
	case h == "goto" && len(f.List) == 2 && !f.List[1].IsList():
		return NewList(NewAtom("goto"), b.label(f.List[1].Atom))
	case h == "for":
		if len(f.List) != 5 {
			return b.fail("a for is (for INIT COND STEP BODY)")
		}
		kids := []*Node{NewAtom("for")}
		for _, k := range f.List[1:] {
			if k.IsList() && len(k.List) == 0 {
				kids = append(kids, NewList())
				continue
			}
			kids = append(kids, b.item(k))
		}
		return NewList(kids...)
	case stmtKids[h]:
		kids := []*Node{NewAtom(h)}
		for _, k := range f.List[1:] {
			kids = append(kids, b.item(k))
		}
		return NewList(kids...)
	}
	return b.expr(f)
}

// expr is an expression.
func (b *builder) expr(f *clisp.Node) *Node {
	if !f.IsList() {
		return b.atom(f.Atom)
	}
	h := f.Head()
	var n *Node
	switch {
	case h == "->" || h == ".":
		n = b.selection(f)
	case h == "cast" && len(f.List) == 3:
		t := b.typ(f.List[1])
		n = NewList(NewAtom("cast"), t, b.expr(f.List[2]))
		n.Type = b.typeOfForm(t)
	case (h == "sizeof-type" || h == "alignof-type") && len(f.List) == 2:
		n = NewList(NewAtom(h), b.typ(f.List[1]))
	case exprHeads[h]:
		kids := []*Node{NewAtom(h)}
		for _, k := range f.List[1:] {
			kids = append(kids, b.expr(k))
		}
		n = NewList(kids...)
		n.Type = exprType(b.e, n)
	default:
		return b.fail("BUILD does not make a (%s ...): a fragment of C is FRAG's (B2a)", h)
	}
	if n.Type == nil {
		b.untyped = append(b.untyped, n)
	}
	return n
}

// selection is `(-> base m...)` or `(. base m...)`, each member resolved
// by the type of what it selects from.
func (b *builder) selection(f *clisp.Node) *Node {
	if len(f.List) < 3 {
		return b.fail("a selection is (%s BASE MEMBER...)", f.Head())
	}
	base := b.expr(f.List[1])
	n := NewList(NewAtom(f.Head()), base)
	t := typeOf(base)
	for _, m := range f.List[2:] {
		if m.IsList() {
			return b.fail("a member is a name, not %s", m)
		}
		s := t // p->a->b selects through a pointer at each step, s.a.b never
		if f.Head() == "->" {
			s = pointee(t)
		}
		mem := memberNamed(s, m.Atom)
		if mem == nil {
			return b.fail("no member `%s` in the type selected from", m.Atom)
		}
		n.Kids = append(n.Kids, &Node{Atom: m.Atom, Refs: []*Node{mem}})
		t = mem.Type
	}
	n.Type = t
	return n
}

// label is a use of the label name in the place's function.
func (b *builder) label(name string) *Node {
	p := b.p
	for p != nil && !p.Is("defn") {
		p = b.e.Parent(p)
	}
	if p == nil {
		return b.fail("the label %s: the place is in no function", name)
	}
	for _, l := range Find(p, func(n *Node) bool { return n.Is("label") && len(n.Kids) == 2 && n.Kids[1].Atom == name }) {
		return &Node{Atom: name, Refs: []*Node{l}}
	}
	return b.fail("no label %s in %s", name, topName(p))
}

// def is a declaration the template makes: its name declared, its type and
// value made in the place's scope.
func (b *builder) def(f *clisp.Node) *Node {
	n := NewList(NewAtom("def"))
	at := 0
	for j := 1; j < len(f.List); j++ {
		k := f.List[j]
		if !k.IsList() && prefixWords[k.Atom] {
			n.Kids = append(n.Kids, NewAtom(k.Atom))
			continue
		}
		at = j
		break
	}
	if at == 0 || f.List[at].IsList() || at+1 >= len(f.List) {
		return b.fail("a def is (def PREFIX... NAME TYPE [VALUE])")
	}
	name := f.List[at].Atom
	n.Kids = append(n.Kids, NewAtom(name))
	t := b.typ(f.List[at+1])
	n.Kids = append(n.Kids, t)
	for _, v := range f.List[at+2:] {
		n.Kids = append(n.Kids, b.expr(v))
	}
	n.Type = b.typeOfForm(t)
	b.local[name] = n
	return n
}

// typ is a type form: its typedef names resolved, its tags.
func (b *builder) typ(f *clisp.Node) *Node {
	if !f.IsList() {
		if isIdent(f.Atom) {
			return b.name(f.Atom)
		}
		return NewAtom(f.Atom)
	}
	switch h := f.Head(); {
	case (h == "struct" || h == "union" || h == "enum") && len(f.List) == 2 && !f.List[1].IsList():
		d := b.e.tagDef(h, f.List[1].Atom)
		if d == nil {
			return b.fail("no %s %s is defined", h, f.List[1].Atom)
		}
		return NewList(NewAtom(h), &Node{Atom: f.List[1].Atom, Refs: []*Node{d}})
	case h == "fn" || h == "fn-ids" || h == "struct" || h == "union" || h == "enum":
		return b.fail("BUILD does not make a (%s ...) type: FRAG's (B2a)", h)
	case h == "array":
		n := NewList(NewAtom("array"))
		for j, k := range f.List[1:] {
			if j == len(f.List)-2 {
				n.Kids = append(n.Kids, b.typ(k))
			} else if k.IsList() || isIdent(k.Atom) {
				n.Kids = append(n.Kids, b.expr(k))
			} else {
				n.Kids = append(n.Kids, NewAtom(k.Atom))
			}
		}
		return n
	}
	n := NewList()
	for j, k := range f.List {
		if j == 0 && !k.IsList() && (typeHeads[k.Atom] || !isIdent(k.Atom)) {
			n.Kids = append(n.Kids, NewAtom(k.Atom))
			continue
		}
		n.Kids = append(n.Kids, b.typ(k))
	}
	return n
}

// typeHeads are the type forms' heads that are words of C-lisp's, not names.
var typeHeads = map[string]bool{}

func init() {
	for _, h := range strings.Fields(`ptr spec paren name-attr typeof typeof-type typeof_unqual typeof_unqual-type
		__typeof__ __typeof__-type atomic _BitInt alignas alignas-type`) {
		typeHeads[h] = true
	}
}

// tagDef is the definition of the struct, union or enum tag: the file's
// form, or the external one.
func (e *Editor) tagDef(kw, tag string) *Node {
	var out *Node
	for _, f := range e.g.Forms {
		if f.Is("defn") {
			continue
		}
		Walk(f, func(n *Node) bool {
			if out == nil && n.Is(kw) && tagOf(n) == tag && isDefForm(n) {
				out = n
			}
			return out == nil
		})
		if out != nil {
			return out
		}
	}
	for _, x := range e.g.Externs {
		if x.Is("extern-"+kw) && len(x.Kids) > 1 && x.Kids[1].Atom == tag {
			return x
		}
	}
	return nil
}

// typeOfForm is the type node a type form names, when the graph holds it:
// a builtin's, a typedef's, a tag's, a pointer to one of those; else nil.
func (b *builder) typeOfForm(t *Node) *Node {
	g := b.e.g
	if !t.list {
		if d := t.Ref(); d != nil {
			return d.Type // a typedef is its type
		}
		return basicType(g, []string{t.Atom})
	}
	switch h := t.Head(); h {
	case "struct", "union", "enum":
		if len(t.Kids) == 2 {
			return t.Kids[1].Ref()
		}
	case "ptr":
		if len(t.Kids) >= 2 {
			return pointerTo(g, b.typeOfForm(t.Kids[1]))
		}
	case "paren":
		return b.typeOfForm(t.Kids[1])
	case "array", "fn", "fn-ids":
		return nil
	}
	// specifiers: qualifiers aside, a builtin's words or one typedef name
	var words []string
	var named *Node
	for _, k := range t.Kids {
		switch {
		case k.list:
			return nil
		case k.Atom == "const" || k.Atom == "volatile" || k.Atom == "restrict" || k.Atom == "spec":
		case k.Ref() != nil:
			named = k
		default:
			words = append(words, k.Atom)
		}
	}
	if named != nil {
		if len(words) > 0 {
			return nil
		}
		return named.Ref().Type
	}
	return basicType(g, words)
}

// basicType is the graph's `(basic ...)` node for the specifiers spelled,
// in cc's words ("unsigned" for unsigned int, "long" for long int).
func basicType(g *Graph, spelled []string) *Node {
	count := map[string]int{}
	for _, w := range spelled {
		count[w]++
	}
	var words []string
	switch {
	case count["void"] == 1:
		words = []string{"void"}
	case count["_Bool"] == 1 || count["bool"] == 1:
		words = []string{"_Bool"}
	case count["char"] == 1:
		switch {
		case count["unsigned"] == 1:
			words = []string{"unsigned", "char"}
		case count["signed"] == 1:
			words = []string{"signed", "char"}
		default:
			words = []string{"char"}
		}
	default:
		var w []string
		if count["unsigned"] == 1 {
			w = append(w, "unsigned")
		}
		switch {
		case count["short"] == 1:
			w = append(w, "short")
		case count["long"] == 1:
			w = append(w, "long")
		case count["long"] == 2:
			w = append(w, "long", "long")
		case count["int"] == 1 || count["signed"] == 1 && len(w) == 0:
			if len(w) == 0 {
				w = append(w, "int")
			}
		}
		words = w
	}
	if len(words) == 0 {
		return nil
	}
	for _, t := range g.Types {
		if t.Is("basic") && len(t.Kids) == len(words)+1 {
			same := true
			for j, w := range words {
				same = same && t.Kids[j+1].Atom == w
			}
			if same {
				return t
			}
		}
	}
	return nil
}

// pointerTo is the graph's `(pointer @:t)`, when it holds one.
func pointerTo(g *Graph, t *Node) *Node {
	if t == nil {
		return nil
	}
	for _, p := range g.Types {
		if p.Is("pointer") && len(p.Kids) == 2 && p.Kids[1].Type == t {
			return p
		}
	}
	return nil
}

// typeOf is an expression's type node: a list's typed edge, an identifier's
// declaration's.
func typeOf(x *Node) *Node {
	if x == nil {
		return nil
	}
	if x.list {
		return x.Type
	}
	if d := x.Ref(); d != nil {
		return d.Type
	}
	return nil
}

// pointee is what a pointer or array type points at.
func pointee(t *Node) *Node {
	if t != nil && (t.Is("pointer") || t.Is("array")) && len(t.Kids) > 0 {
		return t.Kids[len(t.Kids)-1].Type
	}
	return nil
}

// resultType is a function's, or a pointer to function's, result.
func resultType(t *Node) *Node {
	if t.Is("pointer") {
		t = pointee(t)
	}
	if t.Is("function") && len(t.Kids) == 3 {
		return t.Kids[2].Type
	}
	return nil
}

// memberNamed is the member name of the struct or union type s: a member
// of an anonymous one inside it too; or an external struct's.
func memberNamed(s *Node, name string) *Node {
	if s == nil {
		return nil
	}
	if strings.HasPrefix(s.Head(), "extern-") {
		for _, m := range s.Kids[2:] {
			if m.Is("member") && len(m.Kids) > 1 && m.Kids[1].Atom == name {
				return m
			}
		}
		return nil
	}
	if !s.Is("struct") && !s.Is("union") {
		return nil
	}
	for _, m := range members(s) {
		if !m.list || len(m.Kids) == 0 {
			continue
		}
		if k := m.Kids[0]; !k.list && k.Atom == name {
			return m
		} else if k.list && len(m.Kids) == 1 {
			if in := memberNamed(k, name); in != nil {
				return in
			}
		}
	}
	return nil
}

// exprType is what an operator form's type is, from its operands', where
// that is plain: a comparison or a logical operator int, an assignment its
// left side's, parentheses their inside's, a call its callee's result,
// `&x` the pointer to x's type, `*p` and `p[i]` what p points at.
func exprType(e *Editor, n *Node) *Node {
	args := n.Args()
	switch h := n.Head(); h {
	case "==", "!=", "<", ">", "<=", ">=", "&&", "||", "!":
		return basicType(e.g, []string{"int"})
	case "paren":
		return typeOf(args[0])
	case "call":
		return resultType(typeOf(args[0]))
	case "addr":
		return pointerTo(e.g, typeOf(args[0]))
	case "deref":
		return pointee(typeOf(args[0]))
	case "index": // a[i][j] is one form: what a points at, once a subscript
		t := typeOf(args[0])
		for range args[1:] {
			t = pointee(t)
		}
		return t
	case "comma":
		return typeOf(args[len(args)-1])
	case "post++", "post--", "pre++", "pre--":
		return typeOf(args[0])
	}
	if assignOps[n.Head()] && len(args) == 2 {
		return typeOf(args[0])
	}
	return nil
}
