package graph

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/ccwalk"
	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/clisp"
)

// A Report is what an import resolved, and what it could not.
type Report struct {
	Counts Counts
	// Check is the type check's complaint, when it had one: the graph is
	// built whether or not, and what did not type stays unresolved or
	// untyped, counted below.  A text the pipeline holds between an edit
	// and its sweep may name what the edit removed.
	Check error

	Idents      Resolved // ordinary identifiers: objects, functions, parameters, locals, enumerators
	Late        int      // of Idents, uses resolved by name to a file-scope declaration after them
	Members     Resolved // a `.` or `->`'s member, by the type of what it selects from
	Ambiguous   int      // member uses whose name more than one struct or union of the file has
	AmbiguousOK int      // of those, resolved by type
	Designators Resolved // `.x` in an initializer
	Typedefs    Resolved
	Tags        Resolved
	Labels      Resolved
	Macros      int // macro invocations in the forms
	MacroRefs   int // their refers edges, from the names their expansions use
	Untyped     int // expression forms cc gave no type
	Unresolved  map[string]int
}

// Resolved counts the uses of one kind of name.
type Resolved struct {
	Uses, File, Local, Extern, Unresolved int
}

func (r Resolved) String() string {
	return fmt.Sprintf("%d uses: %d to the file's declarations, %d to locals, %d to the headers', %d unresolved",
		r.Uses, r.File, r.Local, r.Extern, r.Unresolved)
}

func (r *Report) unresolved(kind, name string) {
	r.Unresolved[kind+" "+name]++
}

// String is the report, a line a kind.
func (r *Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "graph: %s\n", r.Counts)
	if r.Check != nil {
		s := r.Check.Error()
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[:i] + " ..."
		}
		fmt.Fprintf(&b, "type check: %s\n", s)
	}
	fmt.Fprintf(&b, "identifiers: %s (%d resolved by name, after their use)\n", r.Idents, r.Late)
	fmt.Fprintf(&b, "members: %s; %d uses of a name more than one struct has, %d of them resolved by type\n",
		r.Members, r.Ambiguous, r.AmbiguousOK)
	fmt.Fprintf(&b, "designators: %s\n", r.Designators)
	fmt.Fprintf(&b, "typedef names: %s\n", r.Typedefs)
	fmt.Fprintf(&b, "tags: %s\n", r.Tags)
	fmt.Fprintf(&b, "labels: %s\n", r.Labels)
	fmt.Fprintf(&b, "macro invocations: %d, %d refers edges from their expansions; expression forms untyped: %d\n",
		r.Macros, r.MacroRefs, r.Untyped)
	keys := make([]string, 0, len(r.Unresolved))
	for k := range r.Unresolved {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  unresolved %s (%d)\n", k, r.Unresolved[k])
	}
	return b.String()
}

// Import parses src, the C translation unit at path, type checks it, and
// makes its graph: the forms C-lisp converts it to, each list and each use
// a node, resolved and typed by crefactor/cc.  The ids are sequential, in
// the containment's order, then the types', then the externs'.
func Import(path string, src []byte) (*Graph, *Report, error) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs // one name for a file however it was reached; a relative or `..` path imports the same (measured)
	}
	ast, bsrc, err := cemit.Parse(path, src)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := ccConfig()
	if err != nil {
		return nil, nil, err
	}
	rep := &Report{Unresolved: map[string]int{}}
	rep.Check = ast.Check(cfg)
	return ImportParsed(ast, path, bsrc, rep)
}

// ImportParsed is Import of a tree cemit.Parse returned and Check has
// typed (or tried to), its report begun as rep.
func ImportParsed(ast *cc.AST, path string, bsrc []byte, rep *Report) (*Graph, *Report, error) {
	im, err := importAST(ast, path, bsrc, rep)
	if err != nil {
		return nil, nil, err
	}
	return im.g, rep, nil
}

// importAST is ImportParsed, the importer kept: what each node came from
// (FRAG, frag.go, asks it where a fragment's nodes are).
func importAST(ast *cc.AST, path string, bsrc []byte, rep *Report) (*importer, error) {
	origin := map[*clisp.Node]cc.Node{}
	forms, err := clisp.Options{Origin: func(n *clisp.Node, from cc.Node) {
		origin[n] = from
	}}.FormsOf(ast, path, bsrc)
	if err != nil {
		return nil, err
	}
	im := &importer{
		g: &Graph{ids: &Sequential{}}, path: path, rep: rep,
		decl: map[cc.Node]*Node{}, owner: map[*Node]*Node{}, tagDefs: map[string][]*Node{},
		fileDecl: map[string]*Node{}, typeOf: map[cc.Type]*Node{}, typeKey: map[string]*Node{},
		keyOf: map[*Node]string{}, externs: map[string]*Node{}, from: map[*Node]cc.Node{},
	}
	for _, f := range forms {
		im.g.Forms = append(im.g.Forms, im.convert(f, origin))
	}
	im.top = map[*Node]bool{}
	for _, f := range im.g.Forms {
		im.top[f] = true
	}
	im.index()
	for _, f := range im.g.Forms {
		im.resolve(f, nil, 0)
	}
	for _, f := range im.g.Forms {
		if f.Is("defn") {
			im.labels(f)
		}
	}
	im.ambiguity()
	im.g.Number()
	rep.Counts = im.g.Count()
	return im, nil
}

type importer struct {
	g    *Graph
	path string
	rep  *Report

	decl     map[cc.Node]*Node  // a declaration of the file -> its form
	owner    map[*Node]*Node    // a member or enumerator -> its struct, union or enum
	tagDefs  map[string][]*Node // "struct foo" -> the file's definitions of that tag
	fileDecl map[string]*Node   // an ordinary name -> its first file-scope declaration

	typeOf  map[cc.Type]*Node
	typeKey map[string]*Node
	keyOf   map[*Node]string
	externs map[string]*Node
	nstruct int
	from    map[*Node]cc.Node // the node of cc's tree each came from
	top     map[*Node]bool
}

// convert makes the graph's node of a form, and of everything in it.
func (im *importer) convert(c *clisp.Node, origin map[*clisp.Node]cc.Node) *Node {
	n := &Node{Atom: c.Atom, list: c.IsList()}
	if o := origin[c]; o != nil {
		im.from[n] = o
	}
	if n.list {
		n.Kids = make([]*Node, len(c.List))
		for i, k := range c.List {
			n.Kids[i] = im.convert(k, origin)
		}
	}
	return n
}

// index records the file's declarations by the tree's nodes they came from,
// each member and enumerator's owner, the tags' definitions and the first
// file-scope declaration of each ordinary name.
func (im *importer) index() {
	for _, f := range im.g.Forms {
		if name := topName(f); name != "" && im.fileDecl[name] == nil {
			im.fileDecl[name] = f
		}
		Walk(f, func(n *Node) bool {
			switch x := im.from[n].(type) {
			case *cc.Declarator, *cc.Enumerator, *cc.LabeledStatement:
				if n.list {
					im.decl[x] = n
				}
			case *cc.StructOrUnionSpecifier:
				if n.list && x.Case == cc.StructOrUnionSpecifierDef {
					im.decl[x] = n
					if tag := x.Token.SrcStr(); tag != "" {
						im.tagDefs[n.Head()+" "+tag] = append(im.tagDefs[n.Head()+" "+tag], n)
					}
					for _, m := range n.Args() {
						im.owner[m] = n
					}
				}
			case *cc.EnumSpecifier:
				if n.list && x.Case == cc.EnumSpecifierDef {
					im.decl[x] = n
					if tag := x.Token2.SrcStr(); tag != "" {
						im.tagDefs["enum "+tag] = append(im.tagDefs["enum "+tag], n)
					}
					for _, m := range n.Args() {
						im.owner[m] = n
					}
				}
			}
			return true
		})
	}
	// An enumerator of a file-scope enum is a file-scope name.
	for _, f := range im.g.Forms {
		if f.Is("defn") {
			continue
		}
		Walk(f, func(n *Node) bool {
			if _, ok := im.from[n].(*cc.Enumerator); ok && n.list && len(n.Kids) > 0 && im.fileDecl[n.Kids[0].Atom] == nil {
				im.fileDecl[n.Kids[0].Atom] = n
			}
			return true
		})
	}
}

// topName is the ordinary name a top-level form declares, if any.
func topName(f *Node) string {
	switch f.Head() {
	case "def", "typedef", "defn":
		if i := defNameAt(f); i > 0 {
			return f.Kids[i].Atom
		}
	}
	return ""
}

// resolve gives n and everything in it their edges.  parent is the list n
// is element i of.
func (im *importer) resolve(n, parent *Node, i int) {
	if parent != nil && (parent.Is("->") || parent.Is(".")) && i >= 2 {
		// a member's place: its atom, or the `(macro ...)` a member macro is
		if x, ok := im.from[n].(*cc.PostfixExpression); ok {
			im.member(n, x)
		}
		return
	}
	switch x := im.from[n].(type) {
	case *cc.PrimaryExpression:
		if !n.list && x.Case == cc.PrimaryExpressionIdent && x.Token.SrcStr() == n.Atom {
			im.ident(n, x)
		} else {
			im.typed(n, x)
			im.macro(n, x)
		}
	case *cc.TypeSpecifier:
		im.typedefName(n, x)
	case *cc.StructOrUnionSpecifier:
		if x.Case != cc.StructOrUnionSpecifierDef {
			im.tag(n, x.LexicalScope(), x.Token.SrcStr(), n.Head())
		}
	case *cc.EnumSpecifier:
		if x.Case != cc.EnumSpecifierDef {
			im.tag(n, x.LexicalScope(), x.Token2.SrcStr(), "enum")
		}
	case *cc.Declarator:
		if n.list {
			n.Type = im.typeNode(x.Type())
		}
	case *cc.Enumerator:
		n.Type = im.typeNode(x.Type())
	case *cc.Initializer:
		im.designators(n, x)
	case *cc.Designator, *cc.JumpStatement, *cc.LabeledStatement, *cc.ParameterDeclaration,
		*cc.StructDeclarator, *cc.StructDeclaration:
		// a designator with its `(at ...)`; a label with its function
	case nil:
	case *cc.UnaryExpression:
		if n.list {
			im.typed(n, x)
		} // an atom is `&&L`'s label, which labels resolves
	case cc.ExpressionNode:
		im.typed(n, x)
		if n.Is("macro") || !n.list {
			im.macro(n, x)
		}
	default:
		// a declaration or statement that is wholly a macro's
		im.macro(n, x)
	}
	for j, k := range n.Kids {
		im.resolve(k, n, j)
	}
}

// typed gives an expression's form its type.
func (im *importer) typed(n *Node, x cc.Node) {
	if !n.list {
		return
	}
	t, ok := x.(interface{ Type() cc.Type })
	if !ok {
		return
	}
	if n.Type = im.typeNode(t.Type()); n.Type == nil {
		im.rep.Untyped++
	}
}

// ident resolves an ordinary identifier: the declaration cc's check found,
// or the one the parser's scopes say is visible; failing both, the
// file-scope declaration of the name (a use before it), or an undeclared
// external.
func (im *importer) ident(n *Node, x *cc.PrimaryExpression) {
	im.rep.Idents.Uses++
	name := n.Atom
	// What no scope declares, cc's check resolves to a declarator of its own
	// (an implicit function, an undefined name): that is no declaration.
	if rt := x.ResolvedTo(); synthetic(rt) && strings.HasPrefix(name, "__builtin_") {
		// a compiler builtin cc knows by its name alone
		n.Refs = []*Node{im.extern(rt, name)}
		im.rep.Idents.Extern++
		return
	}
	s := x.LexicalScope().Declares(x.Token)
	var d cc.Node
	if s != nil && !synthetic(x.ResolvedTo()) {
		d = x.ResolvedTo()
	}
	if d == nil {
		if s != nil {
			for _, v := range s.Nodes[name] {
				switch v.(type) {
				case *cc.Declarator, *cc.Enumerator, *cc.Parameter:
					if !synthetic(v) && (d == nil || im.decl[v] != nil) {
						d = v
					}
				}
			}
		}
	}
	if p, ok := d.(*cc.Parameter); ok {
		d = p.Declarator
	}
	if t := im.decl[d]; t != nil {
		n.Refs = []*Node{t}
		if im.fileScope(t) {
			im.rep.Idents.File++
		} else {
			im.rep.Idents.Local++
		}
		return
	}
	if t := im.fileDecl[name]; t != nil {
		// declared in a header too, or used before the file declares it
		n.Refs = []*Node{t}
		im.rep.Idents.File++
		if d == nil {
			im.rep.Late++
		}
		return
	}
	if d != nil {
		n.Refs = []*Node{im.extern(d, name)}
		im.rep.Idents.Extern++
		return
	}
	n.Refs = []*Node{im.undeclared("undeclared", name)}
	im.rep.Idents.Unresolved++
	im.rep.unresolved("identifier", name)
}

// fileScope says a declaration's form is the file's, not a function's.
func (im *importer) fileScope(t *Node) bool {
	return im.top[t] || im.fileDecl[declName(t)] == t
}

// declName is the name a declaration's form declares.
func declName(t *Node) string {
	switch t.Head() {
	case "def", "typedef", "defn":
		if i := defNameAt(t); i > 0 {
			return t.Kids[i].Atom
		}
	}
	if t.list && len(t.Kids) > 0 && !t.Kids[0].list {
		return t.Kids[0].Atom
	}
	return ""
}

// member resolves a member's atom (or the `(macro ...)` a member macro is)
// by the field cc's check found for its selection.
func (im *importer) member(n *Node, x *cc.PostfixExpression) {
	im.rep.Members.Uses++
	f := x.Field()
	if f == nil {
		n.Refs = []*Node{im.undeclared("unresolved-member", memberName(n))}
		im.rep.Members.Unresolved++
		im.rep.unresolved("member", memberName(n))
		return
	}
	im.field(n, f, &im.rep.Members)
}

// field makes n refer to the member f is.
func (im *importer) field(n *Node, f *cc.Field, r *Resolved) {
	if d := f.Declarator(); d != nil {
		if t := im.decl[d]; t != nil {
			n.Refs = []*Node{t}
			r.File++
			return
		}
	}
	s := im.typeNode(f.ParentType())
	if s == nil || !strings.HasPrefix(s.Head(), "extern-") {
		n.Refs = []*Node{im.undeclared("unresolved-member", f.Name())}
		r.Unresolved++
		im.rep.unresolved("member", f.Name())
		return
	}
	n.Refs = []*Node{im.externMember(s, f.Name(), f.Type())}
	r.Extern++
}

func memberName(n *Node) string {
	if n.list && len(n.Kids) == 2 {
		return n.Kids[1].Atom
	}
	return n.Atom
}

// typedefName resolves a typedef name in a type: the parser's scopes, which
// decided it is a type, say which typedef.
func (im *importer) typedefName(n *Node, x *cc.TypeSpecifier) {
	if n.list || x.Case != cc.TypeSpecifierTypeName {
		return
	}
	im.rep.Typedefs.Uses++
	name := x.Token.SrcStr()
	for s := x.LexicalScope(); s != nil; s = s.Parent {
		for _, v := range s.Nodes[name] {
			d, ok := v.(*cc.Declarator)
			if !ok || !d.IsTypename() {
				continue
			}
			if t := im.decl[d]; t != nil {
				n.Refs = []*Node{t}
				if im.fileScope(t) {
					im.rep.Typedefs.File++
				} else {
					im.rep.Typedefs.Local++
				}
				return
			}
			n.Refs = []*Node{im.extern(d, name)}
			im.rep.Typedefs.Extern++
			return
		}
	}
	n.Refs = []*Node{im.undeclared("undeclared", name)}
	im.rep.Typedefs.Unresolved++
	im.rep.unresolved("typedef", name)
}

// tag resolves a struct, union or enum named without its body: the
// definition the parser's scopes hold, from the innermost out, or an
// external.
func (im *importer) tag(n *Node, sc *cc.Scope, name, kw string) {
	if name == "" {
		return
	}
	im.rep.Tags.Uses++
	for s := sc; s != nil; s = s.Parent {
		var found, def cc.Node
		for _, v := range s.Nodes[name] {
			switch x := v.(type) {
			case *cc.StructOrUnionSpecifier:
				if kw == "enum" {
					continue
				}
				found = x
				if x.Case == cc.StructOrUnionSpecifierDef {
					def = x
				}
			case *cc.EnumSpecifier:
				if kw != "enum" {
					continue
				}
				found = x
				if x.Case == cc.EnumSpecifierDef {
					def = x
				}
			}
		}
		if found == nil {
			continue
		}
		if t := im.decl[def]; t != nil {
			n.Refs = []*Node{t}
			if im.fileScopeTag(t) {
				im.rep.Tags.File++
			} else {
				im.rep.Tags.Local++
			}
			return
		}
		break
	}
	n.Refs = []*Node{im.externTag(kw, name)}
	im.rep.Tags.Extern++
}

func (im *importer) fileScopeTag(t *Node) bool {
	for _, d := range im.tagDefs[t.Head()+" "+t.Kids[1].Atom] {
		if d == t {
			return true
		}
	}
	return false
}

// designators resolves an `(at ...)`'s field designators by the type of
// the object its brace list initialises, one designator after another.
func (im *importer) designators(at *Node, x *cc.Initializer) {
	var cur cc.Type
	if p := x.Parent(); p != nil {
		cur = p.Type()
	}
	var fields []*Node
	for _, k := range at.Kids {
		if _, ok := im.from[k].(*cc.Designator); ok && !k.list {
			fields = append(fields, k)
		}
	}
	for _, k := range at.Args() {
		d, _ := im.from[k].(*cc.Designator)
		switch {
		case d != nil && (d.Case == cc.DesignatorField || d.Case == cc.DesignatorField2):
			im.rep.Designators.Uses++
			name := strings.TrimSuffix(strings.TrimPrefix(k.Atom, "."), ":")
			var f *cc.Field
			if s, ok := cur.(interface{ FieldByName(string) *cc.Field }); ok && cur != nil {
				f = s.FieldByName(name)
			}
			if f == nil && len(fields) == 1 && x.Field() != nil && x.Field().Name() == name {
				f = x.Field()
			}
			if f == nil {
				k.Refs = []*Node{im.undeclared("unresolved-member", name)}
				im.rep.Designators.Unresolved++
				im.rep.unresolved("designator", name)
				cur = nil
				continue
			}
			im.field(k, f, &im.rep.Designators)
			cur = f.Type()
		case k.Is("idx"):
			if a, ok := cur.(*cc.ArrayType); ok {
				cur = a.Elem()
			} else {
				cur = nil
			}
		}
	}
}

// labels resolves the labels a function's gotos and `&&L`s name.
func (im *importer) labels(fn *Node) {
	defs := map[string]*Node{}
	Walk(fn, func(n *Node) bool {
		if n.Is("label") && len(n.Kids) == 2 {
			defs[n.Kids[1].Atom] = n
		}
		return true
	})
	Walk(fn, func(n *Node) bool {
		if (n.Is("goto") || n.Is("label-addr")) && len(n.Kids) == 2 && !n.Kids[1].list {
			a := n.Kids[1]
			im.rep.Labels.Uses++
			if t := defs[a.Atom]; t != nil {
				a.Refs = []*Node{t}
				im.rep.Labels.Local++
			} else {
				a.Refs = []*Node{im.undeclared("undeclared-label", a.Atom)}
				im.rep.Labels.Unresolved++
				im.rep.unresolved("label", a.Atom)
			}
		}
		return true
	})
}

// macro gives a form that is a macro's invocation -- `(macro "...")`, an
// identifier-like macro's atom, `(macro-decl ...)` -- a refers edge to
// every name its expansion uses: the identifiers, the members, the typedef
// names.  The forms do not structure the invocation's text; the tree has
// the expansion, and the edges are its.
func (im *importer) macro(n *Node, x cc.Node) {
	if x == nil {
		return
	}
	if !n.list && !isIdentText(n.Atom) {
		return // a literal, not an invocation
	}
	if n.list && !n.Is("macro") && !n.Is("macro-decl") {
		return
	}
	im.rep.Macros++
	seen := map[*Node]bool{}
	add := func(t *Node) {
		if t != nil && !seen[t] {
			seen[t] = true
			n.Refs = append(n.Refs, t)
		}
	}
	ccwalk.Walk(x, func(m cc.Node) bool {
		switch y := m.(type) {
		case *cc.PrimaryExpression:
			if y.Case == cc.PrimaryExpressionIdent {
				tmp := &Node{Atom: y.Token.SrcStr()}
				save := im.rep.Idents
				im.ident(tmp, y)
				im.rep.Idents = save
				add(tmp.Ref())
			}
		case *cc.PostfixExpression:
			if f := y.Field(); f != nil {
				tmp := &Node{}
				var r Resolved
				im.field(tmp, f, &r)
				add(tmp.Ref())
			}
		}
		return true
	})
	im.rep.MacroRefs += len(n.Refs)
}

func isIdentText(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// ambiguity counts the member uses the pilot's untyped resolver could not
// place: a name more than one of the file's structs and unions has.
func (im *importer) ambiguity() {
	owners := map[string]int{}
	for _, defs := range im.tagDefs {
		for _, s := range defs {
			if s.Is("enum") {
				continue
			}
			countMembers(s, owners)
		}
	}
	// anonymous structs and unions count too
	im.g.Walk(func(n *Node) bool {
		if (n.Is("struct") || n.Is("union")) && isDefForm(n) && tagOf(n) == "" {
			countMembers(n, owners)
		}
		return true
	})
	for _, f := range im.g.Forms {
		Walk(f, func(n *Node) bool {
			if !(n.Is("->") || n.Is(".")) {
				return true
			}
			for _, m := range n.Kids[2:] {
				name := memberName(m)
				if owners[name] > 1 {
					im.rep.Ambiguous++
					if t := m.Ref(); t != nil && !isUndeclared(t) {
						im.rep.AmbiguousOK++
					}
				}
			}
			return true
		})
	}
}

func countMembers(s *Node, owners map[string]int) {
	for _, m := range s.Args() {
		if name := memberDeclName(m); name != "" {
			owners[name]++
		}
	}
}

// memberDeclName is the name a struct's member form declares: `(NAME TYPE
// ...)`, not `(TYPE)` or `(TYPE (bits W))`.
func memberDeclName(m *Node) string {
	if !m.list || len(m.Kids) < 2 || m.Kids[0].list || m.Is("@") || m.Is("static_assert") {
		return ""
	}
	if a := m.Kids[1]; a.Is("bits") || isAttrForm(a) {
		return ""
	}
	return m.Kids[0].Atom
}

// synthetic says a declaration is one cc's check made for a name nothing
// declares, and put in the scope where it was used.
func synthetic(d cc.Node) bool {
	x, ok := d.(*cc.Declarator)
	return ok && x.IsSynthetic()
}
