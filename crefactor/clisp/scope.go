package clisp

// THE RESOLVER.  Which declaration an identifier refers to, read off the
// forms alone: C's four name spaces -- ordinary identifiers (functions,
// objects, typedef names, enumerators), tags, members and labels -- and its
// scopes -- the file, a definition's parameters and body, every block, a
// for-statement's declaration, a statement expression -- each declaration in
// scope from its own declarator on, an inner one shadowing an outer.
//
// IT HAS NO TYPES.  An ordinary identifier, a tag and a label resolve
// exactly; a member does not, since which struct `p->next` names is the type
// of p: a member use resolves when one member of all the structs and unions
// has its name, and is ambiguous otherwise (Ref.Decl nil, Ambiguous set).
// What the forms carry as text -- a macro's invocation, `verbatim`, an
// attribute kept as text -- is not looked into; the forms holding it are
// listed in Opaque, for a caller that must know nothing hides a mention.

// A Space is one of C's name spaces.
type Space int

// The name spaces.
const (
	Ordinary Space = iota
	Tag
	Member
	Label
)

func (s Space) String() string {
	return [...]string{"ordinary", "tag", "member", "label"}[s]
}

// A Decl is one declared thing: every declaration of one file-scope name is
// one Decl -- a prototype and the definition, `extern` and the object.
type Decl struct {
	Name  string
	Space Space
	// Kind is function, object, typedef, enumerator, param, local, struct,
	// union, enum, member or label.
	Kind string
	// Nodes are the declaring forms: a def, typedef or defn; a parameter's
	// (NAME TYPE ...); a member's; an enumerator's (NAME [VALUE]); a tag's
	// struct, union or enum form; a label's (label L).
	Nodes []*Node
	File  bool  // declared at file scope
	Fn    *Node // the definition a parameter, local or label is in
	// Owner is a member's struct or union form.
	Owner *Node
}

// A Ref is one identifier's use.
type Ref struct {
	Atom      *Node // the atom; a designator's is `.name`
	Space     Space
	Decl      *Decl // nil: not declared in the forms, or an ambiguous member
	Ambiguous bool  // a member whose name more than one struct has
	Fn        *Node // the definition the use is in, nil at file scope
}

// Scopes is what Resolve found.
type Scopes struct {
	Refs    []Ref
	byAtom  map[*Node]int
	uses    map[*Decl][]int
	File    map[string]*Decl   // the file scope's ordinary names
	Tags    map[string]*Decl   // the file scope's tags
	Members map[string][]*Decl // every member, by name
	Decls   []*Decl            // every declaration, in order
	Opaque  []*Node            // forms whose text the tree does not hold
}

// DeclOf is the declaration an identifier's atom refers to, or nil.
func (s *Scopes) DeclOf(atom *Node) *Decl {
	if i, ok := s.byAtom[atom]; ok {
		return s.Refs[i].Decl
	}
	return nil
}

// RefOf is the use an atom is, and whether it is one.
func (s *Scopes) RefOf(atom *Node) (Ref, bool) {
	i, ok := s.byAtom[atom]
	if !ok {
		return Ref{}, false
	}
	return s.Refs[i], true
}

// Uses is every use of d, in order.
func (s *Scopes) Uses(d *Decl) []Ref {
	var out []Ref
	for _, i := range s.uses[d] {
		out = append(out, s.Refs[i])
	}
	return out
}

// Resolve resolves every identifier in forms.
func Resolve(forms []*Node) *Scopes {
	r := &resolver{s: &Scopes{
		byAtom: map[*Node]int{}, uses: map[*Decl][]int{},
		File: map[string]*Decl{}, Tags: map[string]*Decl{}, Members: map[string][]*Decl{},
	}}
	r.push()
	for _, f := range forms {
		r.top(f)
	}
	return r.s
}

type scope struct {
	ord, tag map[string]*Decl
}

type resolver struct {
	s      *Scopes
	stack  []scope
	fn     *Node
	labels map[string]*Decl
}

func (r *resolver) push() { r.stack = append(r.stack, scope{}) }
func (r *resolver) pop()  { r.stack = r.stack[:len(r.stack)-1] }

func (r *resolver) atFile() bool { return len(r.stack) == 1 }

func (r *resolver) lookup(sp Space, name string) *Decl {
	for i := len(r.stack) - 1; i >= 0; i-- {
		m := r.stack[i].ord
		if sp == Tag {
			m = r.stack[i].tag
		}
		if d, ok := m[name]; ok {
			return d
		}
	}
	return nil
}

// declare enters a declaration in the innermost scope; at file scope a name
// declared again is the same Decl.
func (r *resolver) declare(sp Space, name, kind string, n *Node) *Decl {
	top := &r.stack[len(r.stack)-1]
	m := &top.ord
	if sp == Tag {
		m = &top.tag
	}
	if *m == nil {
		*m = map[string]*Decl{}
	}
	if d, ok := (*m)[name]; ok && r.atFile() {
		d.Nodes = append(d.Nodes, n)
		if kind == "function" || (d.Kind == "object" && kind != "object") {
			d.Kind = kind
		}
		return d
	}
	d := &Decl{Name: name, Space: sp, Kind: kind, Nodes: []*Node{n}, File: r.atFile()}
	if !d.File {
		d.Fn = r.fn
	}
	(*m)[name] = d
	r.s.Decls = append(r.s.Decls, d)
	if d.File {
		if sp == Tag {
			r.s.Tags[name] = d
		} else {
			r.s.File[name] = d
		}
	}
	return d
}

func (r *resolver) ref(a *Node, sp Space, d *Decl) {
	i := len(r.s.Refs)
	r.s.Refs = append(r.s.Refs, Ref{Atom: a, Space: sp, Decl: d, Fn: r.fn})
	r.s.byAtom[a] = i
	if d != nil {
		r.s.uses[d] = append(r.s.uses[d], i)
	}
}

func (r *resolver) use(a *Node) {
	if !isIdent(a.Atom) || exprWords[a.Atom] {
		return
	}
	r.ref(a, Ordinary, r.lookup(Ordinary, a.Atom))
}

func (r *resolver) member(a *Node) {
	name := a.Atom
	if len(name) > 0 && name[0] == '.' {
		name = name[1:]
	}
	if !isIdent(name) {
		return
	}
	ms := r.s.Members[name]
	if len(ms) == 1 {
		r.ref(a, Member, ms[0])
		return
	}
	r.ref(a, Member, nil)
	r.s.Refs[len(r.s.Refs)-1].Ambiguous = len(ms) > 1
}

func (r *resolver) opaque(n *Node) { r.s.Opaque = append(r.s.Opaque, n) }

// ---- declarations

func (r *resolver) top(f *Node) {
	switch f.Head() {
	case "include":
	case "directive", "macro-decl", "verbatim":
		r.opaque(f)
	case "defn":
		r.defn(f)
	default:
		r.item(f)
	}
}

func (r *resolver) defn(f *Node) {
	i := defNameAt(f)
	if i < 0 {
		return
	}
	r.attrs(f.List[1:i])
	r.declare(Ordinary, f.List[i].Atom, "function", f)
	r.fn = f
	r.labels = map[string]*Decl{}
	Walk(f, func(c *Cursor) bool {
		if c.n.Is("label") && len(c.n.List) == 2 {
			name := c.n.List[1].Atom
			d := &Decl{Name: name, Space: Label, Kind: "label", Nodes: []*Node{c.n}, Fn: f}
			r.labels[name] = d
			r.s.Decls = append(r.s.Decls, d)
		}
		return true
	}, nil)
	r.push()
	// the parameters and the body are one scope
	if t := DefType(f); t != nil {
		r.typ(t, true)
	}
	rest := defRest(f)
	if len(rest) > 0 && rest[0].Is("kr-params") {
		for _, d := range rest[0].Args() {
			r.def(d, "param")
		}
	}
	for _, it := range Body(f) {
		r.item(it)
	}
	r.pop()
	r.fn, r.labels = nil, nil
}

// def is a def or typedef form; kind is what a non-function object of it is
// (local, param), at file scope object.
func (r *resolver) def(f *Node, kind string) {
	i := defNameAt(f)
	if i < 0 || i+1 >= len(f.List) {
		return
	}
	r.attrs(f.List[1:i])
	t := f.List[i+1]
	r.typ(t, false)
	k := kind
	switch {
	case f.Is("typedef") || HasPrefix(f, "typedef"):
		k = "typedef"
	case isFnType(t):
		k = "function"
	case r.atFile():
		k = "object"
	}
	name := f.List[i].Atom
	if fd := r.s.File[name]; fd != nil && !r.atFile() && (k == "function" || HasPrefix(f, "extern")) {
		// a block-scope extern, or a function's prototype, names the file's
		fd.Nodes = append(fd.Nodes, f)
		top := &r.stack[len(r.stack)-1]
		if top.ord == nil {
			top.ord = map[string]*Decl{}
		}
		top.ord[name] = fd
	} else {
		r.declare(Ordinary, name, k, f)
	}
	for _, x := range f.List[i+2:] {
		switch {
		case isAttr(x):
			r.attrs([]*Node{x})
		case x.Is("asm-label"):
		default:
			r.init(x)
		}
	}
}

func isFnType(t *Node) bool {
	for t.Is("paren") && len(t.List) == 2 {
		t = t.List[1]
	}
	return t.Is("fn") || t.Is("fn-ids")
}

func (r *resolver) attrs(as []*Node) {
	for _, a := range as {
		if a.Is("attr-text") {
			r.opaque(a)
		}
	}
}

// ---- types

// typeWords are the keywords a type's specifiers and qualifiers may be.
var typeWords = map[string]bool{
	"void": true, "char": true, "short": true, "int": true, "long": true, "float": true,
	"double": true, "signed": true, "unsigned": true, "_Bool": true, "bool": true,
	"_Complex": true, "__complex__": true, "_Imaginary": true, "const": true, "volatile": true,
	"restrict": true, "__restrict": true, "__restrict__": true, "__const": true,
	"__volatile": true, "__volatile__": true, "_Atomic": true, "_Nonnull": true,
	"__signed": true, "__signed__": true, "__int128": true, "_Float16": true, "_Float32": true,
	"_Float64": true, "_Float128": true, "_Float32x": true, "_Float64x": true,
	"_Decimal32": true, "_Decimal64": true, "_Decimal128": true, "__float128": true,
	"__auto_type": true, "auto": true, "static": true, "extern": true, "register": true,
	"typedef": true, "inline": true, "__inline": true, "__inline__": true, "_Noreturn": true,
	"_Thread_local": true, "thread_local": true, "__thread": true, "constexpr": true,
	"__extension__": true,
}

// exprWords are the identifiers an expression may be that name nothing.
var exprWords = map[string]bool{
	"nullptr": true, "true": true, "false": true,
	"__func__": true, "__FUNCTION__": true, "__PRETTY_FUNCTION__": true,
}

// typ walks a type; declParams says its outermost function's parameters are
// declared in the scope at hand (a definition's).
func (r *resolver) typ(t *Node, declParams bool) {
	if !t.list {
		if isIdent(t.Atom) && !typeWords[t.Atom] {
			r.use(t)
		}
		return
	}
	args := t.Args()
	switch t.Head() {
	case "ptr":
		if len(args) > 0 {
			r.typ(args[0], false)
			r.attrs(args[1:])
		}
	case "array":
		for len(args) > 0 && !args[0].list && arrayWords[args[0].Atom] {
			args = args[1:]
		}
		if len(args) == 2 {
			if args[0].list || args[0].Atom != "*" {
				r.expr(args[0])
			}
			args = args[1:]
		}
		if len(args) == 1 {
			r.typ(args[0], false)
		}
	case "fn":
		if len(args) == 2 {
			r.params(args[0], declParams)
			r.typ(args[1], false)
		}
	case "fn-ids":
		if len(args) == 2 {
			r.typ(args[1], false)
		}
	case "paren":
		if len(args) == 1 {
			r.typ(args[0], declParams)
		}
	case "struct", "union":
		r.structOrUnion(t)
	case "enum":
		r.enum(t)
	case "typeof", "__typeof__", "__typeof", "alignas":
		for _, a := range args {
			r.expr(a)
		}
	case "typeof-type", "__typeof__-type", "__typeof-type", "atomic", "alignas-type":
		for _, a := range args {
			r.typ(a, false)
		}
	case "attr", "attr-text":
		r.attrs([]*Node{t})
	case "spec":
		for _, a := range args {
			r.typ(a, false)
		}
	default:
		// a list of specifiers
		for _, a := range t.List {
			r.typ(a, false)
		}
	}
}

func (r *resolver) params(l *Node, declare bool) {
	if !l.list {
		return
	}
	for _, x := range l.List {
		if !x.list {
			if x.Atom != "..." {
				r.typ(x, false)
			}
			continue
		}
		if len(x.List) >= 2 && !isAttr(x.List[1]) {
			r.typ(x.List[1], false)
			r.attrs(x.List[2:])
			if declare {
				r.declare(Ordinary, x.List[0].Atom, "param", x)
			}
			continue
		}
		if len(x.List) > 0 {
			r.typ(x.List[0], false)
			r.attrs(x.List[1:])
		}
	}
}

func (r *resolver) structOrUnion(n *Node) {
	args := n.Args()
	var tag *Node
	if len(args) > 0 && !args[0].list && args[0].Atom != "{}" {
		tag = args[0]
		args = args[1:]
	}
	body := false
	for _, a := range args {
		if !a.Is("@") {
			body = true
		}
	}
	if tag != nil {
		r.tag(tag, n, body)
	}
	for _, m := range args {
		switch {
		case m.Is("@"):
			r.attrs(m.Args())
		case m.Is("static_assert"):
			r.staticAssert(m)
		case !m.list || len(m.List) == 0:
		case len(m.List) >= 2 && !m.List[1].Is("bits") && !isAttr(m.List[1]):
			r.typ(m.List[1], false)
			r.memberRest(m.List[2:])
			name := m.List[0].Atom
			d := &Decl{Name: name, Space: Member, Kind: "member", Nodes: []*Node{m}, Owner: n}
			r.s.Members[name] = append(r.s.Members[name], d)
			r.s.Decls = append(r.s.Decls, d)
		default:
			r.typ(m.List[0], false)
			r.memberRest(m.List[1:])
		}
	}
}

func (r *resolver) memberRest(xs []*Node) {
	for _, x := range xs {
		if x.Is("bits") {
			for _, w := range x.Args() {
				r.expr(w)
			}
		} else {
			r.attrs([]*Node{x})
		}
	}
}

// tag is a tag's use, or its declaration when body says the form defines it;
// a tag used before anything declares it is declared by the use, as C does.
func (r *resolver) tag(a, form *Node, body bool) {
	if body {
		top := &r.stack[len(r.stack)-1]
		if d, ok := top.tag[a.Atom]; ok {
			d.Nodes = append(d.Nodes, form)
			return
		}
		r.declare(Tag, a.Atom, form.Head(), form)
		return
	}
	d := r.lookup(Tag, a.Atom)
	if d == nil {
		d = r.declare(Tag, a.Atom, form.Head(), form)
	}
	r.ref(a, Tag, d)
}

func (r *resolver) enum(n *Node) {
	args := n.Args()
	var tag *Node
	if len(args) > 0 && !args[0].list {
		tag = args[0]
		args = args[1:]
	}
	if len(args) > 0 && args[0].Is(":") {
		for _, s := range args[0].Args() {
			r.typ(s, false)
		}
		args = args[1:]
	}
	if tag != nil {
		r.tag(tag, n, len(args) > 0)
	}
	for _, e := range args {
		if !e.list || len(e.List) == 0 || e.List[0].list {
			continue
		}
		if len(e.List) > 1 {
			r.expr(e.List[1])
		}
		r.declare(Ordinary, e.List[0].Atom, "enumerator", e)
	}
}

func (r *resolver) staticAssert(f *Node) {
	if a := f.Args(); len(a) > 0 {
		r.expr(a[0])
	}
}

// ---- statements

func (r *resolver) item(f *Node) {
	if !f.list {
		r.expr(f)
		return
	}
	args := f.Args()
	switch f.Head() {
	case "def":
		r.def(f, "local")
	case "typedef":
		r.def(f, "typedef")
	case "struct", "union", "enum":
		r.typ(f, false)
	case "declare":
		for _, a := range args {
			r.typ(a, false)
		}
	case "static_assert":
		r.staticAssert(f)
	case "macro-decl", "verbatim":
		r.opaque(f)
	case "block":
		r.push()
		for _, it := range args {
			r.item(it)
		}
		r.pop()
	case "label":
		// declared before the body was walked: a goto may come first
	case "case", "case-range", "goto*":
		for _, a := range args {
			r.expr(a)
		}
	case "default", "empty", "break", "continue":
	case "attributed":
		for _, a := range args {
			if isAttr(a) {
				r.attrs([]*Node{a})
			} else {
				r.expr(a)
			}
		}
	case "if", "switch", "while":
		if len(args) > 0 {
			r.expr(args[0])
		}
		for _, s := range args[1:] {
			r.body(s)
		}
	case "do":
		if len(args) == 2 {
			r.body(args[0])
			r.expr(args[1])
		}
	case "for":
		if len(args) != 4 {
			return
		}
		r.push()
		if declHeads[args[0].Head()] {
			r.item(args[0])
		} else {
			r.clause(args[0])
		}
		r.clause(args[1])
		r.clause(args[2])
		r.body(args[3])
		r.pop()
	case "return":
		for _, a := range args {
			r.expr(a)
		}
	case "goto":
		if len(args) == 1 {
			r.labelUse(args[0])
		}
	default:
		r.expr(f)
	}
}

// body is a statement's sub-statement, a scope of its own.
func (r *resolver) body(s *Node) {
	r.push()
	r.item(s)
	r.pop()
}

func (r *resolver) clause(n *Node) {
	if n.list && len(n.List) == 0 {
		return
	}
	r.expr(n)
}

func (r *resolver) labelUse(a *Node) {
	var d *Decl
	if r.labels != nil {
		d = r.labels[a.Atom]
	}
	r.ref(a, Label, d)
}

// ---- expressions

func (r *resolver) expr(n *Node) {
	if !n.list {
		r.use(n)
		return
	}
	if len(n.List) == 0 || n.List[0].list {
		for _, x := range n.List {
			r.expr(x)
		}
		return
	}
	args := n.Args()
	switch n.Head() {
	case ".", "->":
		if len(args) > 0 {
			r.expr(args[0])
			for _, m := range args[1:] {
				if m.list {
					r.expr(m)
				} else {
					r.member(m)
				}
			}
		}
	case "cast":
		if len(args) == 2 {
			r.typ(args[0], false)
			r.expr(args[1])
		}
	case "sizeof-type", "alignof-type":
		for _, a := range args {
			r.typ(a, false)
		}
	case "literal":
		if len(args) > 0 {
			r.typ(args[0], false)
			for _, x := range args[1:] {
				r.init(x)
			}
		}
	case "generic":
		if len(args) > 0 {
			r.expr(args[0])
			for _, a := range args[1:] {
				switch {
				case a.Is("default"):
					for _, v := range a.Args() {
						r.expr(v)
					}
				case a.list && len(a.List) == 2:
					r.typ(a.List[0], false)
					r.expr(a.List[1])
				}
			}
		}
	case "stmt-expr":
		r.push()
		for _, it := range args {
			r.item(it)
		}
		r.pop()
	case "label-addr":
		if len(args) == 1 {
			r.labelUse(args[0])
		}
	case "macro":
		r.opaque(n)
	case "init", "at":
		r.init(n)
	default:
		for _, a := range args {
			r.expr(a)
		}
	}
}

// init is a value: an expression, or an initializer's braces and
// designations.
func (r *resolver) init(n *Node) {
	switch n.Head() {
	case "init":
		for _, x := range n.Args() {
			r.init(x)
		}
	case "at":
		args := n.Args()
		if len(args) == 0 {
			return
		}
		for _, d := range args[:len(args)-1] {
			switch {
			case !d.list:
				r.member(d)
			case d.Is("idx"):
				for _, e := range d.Args() {
					r.expr(e)
				}
			}
		}
		r.init(args[len(args)-1])
	default:
		r.expr(n)
	}
}
