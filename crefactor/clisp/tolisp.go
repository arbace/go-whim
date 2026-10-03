package clisp

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
)

// ToLisp converts a C translation unit to its s-expressions.  It reads the
// tree crefactor/cemit prints, case for case, so that ToC of what it returns
// is cemit.Canonical of the same text -- and, for a text already canonical,
// the text itself, byte for byte.
func ToLisp(path string, src []byte) ([]byte, error) {
	forms, err := Forms(path, src)
	if err != nil {
		return nil, err
	}
	return Format(forms), nil
}

// Forms is ToLisp before the layout: one form per top-level line group.
func Forms(path string, src []byte) ([]*Node, error) {
	ast, src, err := cemit.Parse(path, src)
	if err != nil {
		return nil, err
	}
	return Options{}.FormsOf(ast, path, src)
}

// Options are a conversion's choices.  The zero value is Forms'.
type Options struct {
	// Origin, when set, is told the node of crefactor/cc's tree each form
	// and each identifier atom came from (crefactor/graph resolves and types
	// them by it): an expression's form its expression (the one that says
	// something, below the grammar's one-child levels; a run flattened into
	// one form, `(+ a b c)`, the outermost); a member's atom its selection;
	// a typedef name its TypeSpecifier; a declaration's form its Declarator,
	// Enumerator, StructOrUnionSpecifier or EnumSpecifier; a designator's
	// atom its Designator and its `(at ...)` the Initializer; a label's form
	// its LabeledStatement, a goto's its JumpStatement; a macro's form the
	// node it is the invocation of.  A node may be told more than once; the
	// last is the outermost.
	Origin func(n *Node, from cc.Node)
}

// FormsOf is Forms of a tree cemit.Parse returned: ast, the parse of path,
// and src, the blanked source it returned.
func (o Options) FormsOf(ast *cc.AST, path string, src []byte) ([]*Node, error) {
	c := &conv{m: cemit.NewMacros(src), opt: o}
	incl, inclAt := cemit.Includes(src)
	var out []*Node
	written := false
	flush := func() {
		if written || len(incl) == 0 {
			return
		}
		written = true
		for _, s := range incl {
			out = append(out, include(s))
		}
	}
	for l := ast.TranslationUnit; l != nil; l = l.TranslationUnit {
		d := l.ExternalDeclaration
		if d == nil || d.Case == cc.ExternalDeclarationEmpty {
			continue
		}
		pos := d.Position()
		if pos.Filename != path {
			continue
		}
		if pos.Line > inclAt {
			flush()
		}
		c.m.Enter(d)
		// CONVERTED ONCE, as cemit's File prints it once: the recovery
		// records the member macros it has handed out, so a second
		// conversion of the same declaration would lose them.  A declaration
		// that converts to nothing is left out.
		out = append(out, c.external(d)...)
		if c.err != nil {
			return nil, c.err
		}
	}
	flush()
	return out, nil
}

// include is an include line's form: `(include "<stdio.h>")`, or the line
// verbatim when it is not spelled `#include ` and its operand.
func include(line string) *Node {
	if rest, ok := strings.CutPrefix(line, "#include "); ok && rest != "" && !strings.ContainsAny(rest, " \t") {
		return L(A("include"), A(quote(rest)))
	}
	return L(A("directive"), A(quote(line)))
}

// quote writes s as a C string literal's atom: what the reader keeps whole,
// and unquote reverses.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

type conv struct {
	m *cemit.Macros
	// noMacros is set inside a statement expression, whose block cemit prints
	// with an emitter of its own that recovers no macro.
	noMacros bool
	err      error
	opt      Options
}

// at tells the Origin that n came from the tree's node from, and is n.
func (c *conv) at(n *Node, from cc.Node) *Node {
	if c.opt.Origin != nil && n != nil {
		c.opt.Origin(n, from)
	}
	return n
}

func (c *conv) fail(n cc.Node, format string, a ...any) {
	if c.err != nil {
		return
	}
	where := ""
	if n != nil {
		where = n.Position().String() + ": "
	}
	c.err = fmt.Errorf("clisp: %s%s", where, fmt.Sprintf(format, a...))
}

func tok(t cc.Token) string { return t.SrcStr() }

// macro is the recovered text of a whole node, if there is one.
func (c *conv) macro(n cc.Node) (string, bool) {
	if c.noMacros {
		return "", false
	}
	return c.m.Node(n)
}

// macroForm is a recovered invocation's form: an identifier as itself
// (`nullptr`, `errno`, `INT_MAX`), anything else `(macro "text")`.
func macroForm(s string) *Node {
	if isIdent(s) {
		return A(s)
	}
	return L(A("macro"), A(quote(s)))
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

func (c *conv) external(n *cc.ExternalDeclaration) []*Node {
	switch n.Case {
	case cc.ExternalDeclarationDecl:
		return c.decl(n.Declaration)
	case cc.ExternalDeclarationFuncDef:
		return []*Node{c.funcDef(n.FunctionDefinition)}
	case cc.ExternalDeclarationAsmStmt:
		return []*Node{verbatim(cc.NodeSource(n.AsmStatement))}
	}
	c.fail(n, "external declaration %v", n.Case)
	return nil
}

func verbatim(s string) *Node { return L(A("verbatim"), A(quote(strings.TrimSpace(s)))) }

// ---- specifiers

// A spec is one declaration specifier's form, and whether it may stand
// before the declared name: a storage class, a function specifier or an
// attribute that comes before every type specifier.
type spec struct {
	n      *Node
	prefix bool
	tdname bool // a typedef name
}

// prefixWords is every specifier keyword that may stand before a name.  ToC
// reads the same set.
var prefixWords = map[string]bool{
	"static": true, "extern": true, "typedef": true, "register": true, "auto": true,
	"inline": true, "__inline": true, "__inline__": true, "_Noreturn": true,
	"_Thread_local": true, "thread_local": true, "__thread": true, "constexpr": true,
	"__auto_type": true,
}

func (c *conv) declSpecs(n *cc.DeclarationSpecifiers) []spec {
	var out []spec
	for l := n; l != nil; l = l.DeclarationSpecifiers {
		switch l.Case {
		case cc.DeclarationSpecifiersStorage:
			s := tok(l.StorageClassSpecifier.Token)
			out = append(out, spec{n: A(s), prefix: prefixWords[s]})
		case cc.DeclarationSpecifiersTypeSpec:
			out = append(out, c.typeSpec(l.TypeSpecifier))
		case cc.DeclarationSpecifiersTypeQual:
			out = append(out, spec{n: A(tok(l.TypeQualifier.Token))})
		case cc.DeclarationSpecifiersFunc:
			s := tok(l.FunctionSpecifier.Token)
			out = append(out, spec{n: A(s), prefix: prefixWords[s]})
		case cc.DeclarationSpecifiersAlignSpec:
			out = append(out, spec{n: c.alignSpec(l.AlignmentSpecifier)})
		case cc.DeclarationSpecifiersAttr:
			for _, a := range c.attrs(l.AttributeSpecifierList) {
				out = append(out, spec{n: a, prefix: true})
			}
		default:
			c.fail(l, "declaration specifier %v", l.Case)
		}
	}
	return out
}

func (c *conv) specQuals(n *cc.SpecifierQualifierList) []spec {
	var out []spec
	for l := n; l != nil; l = l.SpecifierQualifierList {
		switch l.Case {
		case cc.SpecifierQualifierListTypeSpec:
			out = append(out, c.typeSpec(l.TypeSpecifier))
		case cc.SpecifierQualifierListTypeQual:
			if q := l.TypeQualifier; q.Case == cc.TypeQualifierAttr {
				// A member's leading attribute, as cemit prints it.
				for _, a := range c.attrs(q.AttributeSpecifierList) {
					out = append(out, spec{n: a})
				}
				break
			}
			out = append(out, spec{n: A(tok(l.TypeQualifier.Token))})
		case cc.SpecifierQualifierListAlignSpec:
			out = append(out, spec{n: c.alignSpec(l.AlignmentSpecifier)})
		default:
			c.fail(l, "specifier-qualifier %v", l.Case)
		}
	}
	return out
}

func (c *conv) alignSpec(n *cc.AlignmentSpecifier) *Node {
	switch n.Case {
	case cc.AlignmentSpecifierType:
		return L(A("alignas-type"), c.typeName(n.TypeName))
	case cc.AlignmentSpecifierExpr:
		return L(A("alignas"), c.expr(n.ConstantExpression, lvCond))
	}
	c.fail(n, "alignment specifier %v", n.Case)
	return A("?")
}

func (c *conv) typeSpec(n *cc.TypeSpecifier) spec {
	switch n.Case {
	case cc.TypeSpecifierStructOrUnion:
		return spec{n: c.structOrUnion(n.StructOrUnionSpecifier)}
	case cc.TypeSpecifierEnum:
		return spec{n: c.enum(n.EnumSpecifier)}
	case cc.TypeSpecifierTypeofExpr:
		// The head is the keyword as spelled: `typeof`, `__typeof__`, `__typeof`.
		return spec{n: L(A(tok(n.Token)), c.expr(n.ExpressionList, lvComma))}
	case cc.TypeSpecifierTypeofType:
		return spec{n: L(A(tok(n.Token)+"-type"), c.typeName(n.TypeName))}
	case cc.TypeSpecifierAtomic:
		return spec{n: L(A("atomic"), c.typeName(n.AtomicTypeSpecifier.TypeName))}
	case cc.TypeSpecifierBitInt:
		return spec{n: L(A("_BitInt"), c.expr(n.ExpressionList, lvCond))}
	case cc.TypeSpecifierTypeName:
		s := c.m.SpecToken(n.Token)
		if c.noMacros {
			s = tok(n.Token)
		}
		return spec{n: c.at(A(s), n), tdname: true}
	}
	s := tok(n.Token)
	if !c.noMacros {
		s = c.m.SpecToken(n.Token)
	}
	return spec{n: A(s)}
}

// base is the form of a specifier list: one specifier as itself, more as a
// list of them -- `(const char)`, `(unsigned long)` -- headed by `spec` when
// the first is a typedef name, which could otherwise be read as a head.
func base(specs []spec) *Node {
	if len(specs) == 1 {
		return specs[0].n
	}
	l := L()
	if len(specs) > 0 && specs[0].tdname {
		l.add(A("spec"))
	}
	for _, s := range specs {
		l.add(s.n)
	}
	return l
}

// splitPrefix separates a declaration's leading storage classes, function
// specifiers and attributes from the type they come before.
func splitPrefix(specs []spec) (prefix []*Node, rest []spec) {
	i := 0
	for i < len(specs) && specs[i].prefix {
		prefix = append(prefix, specs[i].n)
		i++
	}
	return prefix, specs[i:]
}

// ---- struct, union, enum

func (c *conv) structOrUnion(n *cc.StructOrUnionSpecifier) *Node {
	kw := tok(n.StructOrUnion.Token)
	f := c.at(L(A(kw)), n)
	if t := tok(n.Token); t != "" {
		f.add(A(t))
	}
	// ITS ATTRIBUTES ARE `(@ ATTR...)`, before the members when the source
	// has them before the body, after them when after: `struct
	// __attribute__((packed)) s { ... }` and `struct s { ... }
	// __attribute__((packed))`.
	if a := c.attrs(n.AttributeSpecifierList); len(a) > 0 {
		f.add(L(A("@")).add(a...))
	}
	switch n.Case {
	case cc.StructOrUnionSpecifierTag:
		return f
	case cc.StructOrUnionSpecifierDef:
		if n.StructDeclarationList == nil {
			f.add(A("{}"))
		}
		for l := n.StructDeclarationList; l != nil; l = l.StructDeclarationList {
			f.add(c.structDecl(l.StructDeclaration)...)
		}
		if a := c.attrs(n.AttributeSpecifierList2); len(a) > 0 {
			f.add(L(A("@")).add(a...))
		}
		return f
	}
	c.fail(n, "struct-or-union specifier %v", n.Case)
	return f
}

func (c *conv) structDecl(n *cc.StructDeclaration) []*Node {
	switch n.Case {
	case cc.StructDeclarationDecl:
		sq := c.specQuals(n.SpecifierQualifierList)
		if len(sq) == 0 {
			c.fail(n, "a member with no type")
			return nil
		}
		// The member's attributes follow its last declarator, as cemit
		// prints them.
		attrs := c.attrs(n.AttributeSpecifierList)
		if n.StructDeclaratorList == nil {
			return []*Node{c.at(L(base(sq)).add(attrs...), n)}
		}
		var out []*Node
		for l := n.StructDeclaratorList; l != nil; l = l.StructDeclaratorList {
			d := l.StructDeclarator
			var m *Node
			if d.Declarator == nil {
				m = c.at(L(base(sq)), d)
			} else {
				name, t := c.declarator(base(sq), d.Declarator)
				m = c.at(L(A(name), t), d.Declarator)
			}
			switch d.Case {
			case cc.StructDeclaratorDecl:
			case cc.StructDeclaratorBitField:
				m.add(L(A("bits"), c.expr(d.ConstantExpression, lvCond)))
			default:
				c.fail(d, "struct declarator %v", d.Case)
			}
			if l.StructDeclaratorList == nil {
				m.add(attrs...)
			}
			out = append(out, m)
		}
		return out
	case cc.StructDeclarationAssert:
		return []*Node{c.staticAssert(n.StaticAssertDeclaration)}
	}
	c.fail(n, "struct declaration %v", n.Case)
	return nil
}

func (c *conv) enum(n *cc.EnumSpecifier) *Node {
	f := c.at(L(A("enum")), n)
	if t := tok(n.Token2); t != "" {
		f.add(A(t))
	}
	// C23's attributes between `enum` and the tag, `(@ ATTR...)` after the
	// tag, as a struct's.
	if a := c.attrs(n.AttributeSpecifierList); len(a) > 0 {
		f.add(L(A("@")).add(a...))
	}
	if n.EnumTypeSpecifier != nil {
		u := L(A(":"))
		for _, s := range c.specQuals(n.EnumTypeSpecifier.SpecifierQualifierList) {
			u.add(s.n)
		}
		f.add(u)
	}
	switch n.Case {
	case cc.EnumSpecifierTag:
		return f
	case cc.EnumSpecifierDef:
		for l := n.EnumeratorList; l != nil; l = l.EnumeratorList {
			e := l.Enumerator
			switch e.Case {
			case cc.EnumeratorIdent:
				f.add(c.at(L(A(tok(e.Token))).add(c.attrs(e.AttributeSpecifierList)...), e))
			case cc.EnumeratorExpr:
				f.add(c.at(L(A(tok(e.Token))).add(c.attrs(e.AttributeSpecifierList)...).add(c.expr(e.ConstantExpression, lvCond)), e))
			default:
				c.fail(e, "enumerator %v", e.Case)
			}
		}
		return f
	}
	c.fail(n, "enum specifier %v", n.Case)
	return f
}

// ---- declarators: C's inside-out, read in order

// declarator is the name a declarator declares and its type over t, the
// type its specifiers give: `char *(*f[4])(int)` is f, `(array 4 (ptr (fn
// ((int)) (ptr char))))`.
func (c *conv) declarator(t *Node, n *cc.Declarator) (string, *Node) {
	return c.directDeclarator(c.pointer(t, n.Pointer), n.DirectDeclarator)
}

// pointer wraps t in a declarator's pointers, the leftmost innermost.
func (c *conv) pointer(t *Node, p *cc.Pointer) *Node {
	for ; p != nil; p = p.Pointer {
		switch p.Case {
		case cc.PointerTypeQual, cc.PointerPtr:
		default:
			c.fail(p, "pointer %v", p.Case)
			return t
		}
		t = L(A("ptr"), t).add(c.quals(p.TypeQualifiers)...)
		if p.Case == cc.PointerTypeQual {
			break
		}
	}
	return t
}

// quals is a pointer's or an array declarator's qualifiers, in the source's
// order (the front end's list holds them backwards; cemit's typeQuals), an
// `__attribute__` among them its attribute forms.
func (c *conv) quals(n *cc.TypeQualifiers) []*Node {
	var out []*Node
	for l := n; l != nil; l = l.TypeQualifiers {
		q := l.TypeQualifier
		switch {
		case q == nil:
		case q.Case == cc.TypeQualifierAttr:
			a := c.attrs(q.AttributeSpecifierList)
			for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
				a[i], a[j] = a[j], a[i]
			}
			out = append(out, a...)
		default:
			out = append(out, A(tok(q.Token)))
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// derived reports whether t is an array or a function, whose declarator
// takes the parentheses a pointer inside it needs.
func derived(t *Node) bool { return t.Is("array") || t.Is("fn") }

// group is the type a parenthesised declarator wraps: t itself when the
// parentheses are the ones ToC will write -- a pointer inside, an array or
// function outside -- and `(paren t)` when they are the source's own.
func group(t *Node, innerPtr bool) *Node {
	if innerPtr && derived(t) {
		return t
	}
	return L(A("paren"), t)
}

func (c *conv) directDeclarator(t *Node, n *cc.DirectDeclarator) (string, *Node) {
	for n != nil {
		switch n.Case {
		case cc.DirectDeclaratorIdent:
			// C23's attributes after the identifier: `(name-attr T ATTR...)`,
			// the outermost form, as they are the nearest the name.
			if a := c.attrs(n.AttributeSpecifierList); len(a) > 0 {
				t = L(A("name-attr"), t).add(a...)
			}
			return tok(n.Token), t
		case cc.DirectDeclaratorDecl:
			return c.declarator(group(t, n.Declarator.Pointer != nil), n.Declarator)
		case cc.DirectDeclaratorArr:
			t = c.array(t, c.quals(n.TypeQualifiers), n.AssignmentExpression)
		case cc.DirectDeclaratorStaticArr:
			t = c.array(t, append([]*Node{A("static")}, c.quals(n.TypeQualifiers)...), n.AssignmentExpression)
		case cc.DirectDeclaratorArrStatic:
			t = c.array(t, append(c.quals(n.TypeQualifiers), A("static")), n.AssignmentExpression)
		case cc.DirectDeclaratorStar:
			t = L(A("array")).add(c.quals(n.TypeQualifiers)...).add(A("*"), t)
		case cc.DirectDeclaratorFuncParam:
			t = L(A("fn"), c.params(n.ParameterTypeList), t)
		case cc.DirectDeclaratorFuncIdent:
			// An identifier list, K&R's: `(fn-ids (a b) int)`.
			ids := L()
			for l := n.IdentifierList; l != nil; l = l.IdentifierList {
				ids.add(A(tok(l.Token2)))
			}
			t = L(A("fn-ids"), ids, t)
		default:
			c.fail(n, "direct declarator %v", n.Case)
			return "", t
		}
		n = n.DirectDeclarator
	}
	c.fail(nil, "a declarator with no name")
	return "", t
}

// array is `(array WORD... N T)`: the words a parameter's array declarator
// says -- `static` and its qualifiers, in their order -- then its size, if it
// has one, then the element type.  `[*]` is the size `*`.
func (c *conv) array(t *Node, words []*Node, size cc.ExpressionNode) *Node {
	f := L(A("array")).add(words...)
	if size != nil {
		f.add(c.expr(size, lvAssign))
	}
	return f.add(t)
}

func (c *conv) abstract(t *Node, n *cc.AbstractDeclarator) *Node {
	if n == nil {
		return t
	}
	return c.directAbstract(c.pointer(t, n.Pointer), n.DirectAbstractDeclarator)
}

func (c *conv) directAbstract(t *Node, n *cc.DirectAbstractDeclarator) *Node {
	for n != nil {
		switch n.Case {
		case cc.DirectAbstractDeclaratorDecl:
			a := n.AbstractDeclarator
			return c.abstract(group(t, a != nil && a.Pointer != nil), a)
		case cc.DirectAbstractDeclaratorArr:
			// `[const static 5]` arrives as this case, its `static` in Token2.
			words := c.quals(n.TypeQualifiers)
			if tok(n.Token2) == "static" {
				words = append(words, A("static"))
			}
			t = c.array(t, words, n.AssignmentExpression)
		case cc.DirectAbstractDeclaratorStaticArr:
			t = c.array(t, append([]*Node{A("static")}, c.quals(n.TypeQualifiers)...), n.AssignmentExpression)
		case cc.DirectAbstractDeclaratorArrStar:
			t = L(A("array"), A("*"), t)
		case cc.DirectAbstractDeclaratorFunc:
			t = L(A("fn"), c.params(n.ParameterTypeList), t)
		default:
			c.fail(n, "direct abstract declarator %v", n.Case)
			return t
		}
		n = n.DirectAbstractDeclarator
	}
	return t
}

// params is a parameter list: `(x int)` a named parameter, `((ptr char))` an
// unnamed one, an atom type bare (`void`, `int`), and `...` last.
func (c *conv) params(n *cc.ParameterTypeList) *Node {
	out := L()
	if n == nil {
		return out
	}
	for l := n.ParameterList; l != nil; l = l.ParameterList {
		p := l.ParameterDeclaration
		b := base(c.declSpecs(p.DeclarationSpecifiers))
		attrs := c.attrs(p.AttributeSpecifierList)
		switch p.Case {
		case cc.ParameterDeclarationDecl:
			name, t := c.declarator(b, p.Declarator)
			out.add(c.at(L(A(name), t).add(attrs...), p.Declarator))
		case cc.ParameterDeclarationAbstract:
			t := c.abstract(b, p.AbstractDeclarator)
			if !t.list && len(attrs) == 0 {
				out.add(t)
			} else {
				out.add(c.at(L(t).add(attrs...), p))
			}
		default:
			c.fail(p, "parameter declaration %v", p.Case)
		}
	}
	if n.Case == cc.ParameterTypeListVar {
		out.add(A("..."))
	}
	return out
}

func (c *conv) typeName(n *cc.TypeName) *Node {
	sq := c.specQuals(n.SpecifierQualifierList)
	if len(sq) == 0 {
		c.fail(n, "a type name with no type")
		return A("?")
	}
	return c.abstract(base(sq), n.AbstractDeclarator)
}

// ---- attributes

// attrs is an attribute list's forms: `__attribute__((unused))` is `(attr
// unused)`, `__attribute__((cold, format(printf, 1, 2)))` is `(attr cold
// (format printf 1 2))`, C23's `[[nodiscard("why")]]` is `(std-attr
// (nodiscard "why"))`, and a specifier spelled any other way is `(attr-text
// "...")`, its source as cemit prints it.
func (c *conv) attrs(n *cc.AttributeSpecifierList) []*Node {
	var out []*Node
	for l := n; l != nil; l = l.AttributeSpecifierList {
		src := cc.NodeSource(l.AttributeSpecifier)
		f := attrForm(l.AttributeSpecifier)
		if f == nil || attrText(f) != src {
			f = L(A("attr-text"), A(quote(src)))
		}
		out = append(out, f)
	}
	return out
}

func attrForm(a *cc.AttributeSpecifier) *Node {
	f := L(A("attr"))
	switch {
	case a.IsStd():
		// C23's `[[...]]` is `(std-attr ...)`, a prefixed name one atom:
		// `[[gnu::aligned(8), nodiscard]]` is `(std-attr (gnu::aligned 8)
		// nodiscard)`.
		f = L(A("std-attr"))
	case tok(a.Token) != "__attribute__":
		return nil
	}
	for l := a.AttributeValueList; l != nil; l = l.AttributeValueList {
		v := l.AttributeValue
		if v == nil || v.BalancedTokenSequence != nil {
			return nil // an attribute left out, or another vendor's tokens: its text
		}
		name := tok(v.Prefix) + tok(v.Colon) + tok(v.Colon2) + tok(v.Token)
		switch v.Case {
		case cc.AttributeValueIdent:
			f.add(A(name))
		case cc.AttributeValueExpr:
			g := L(A(name))
			for al := v.ArgumentExpressionList; al != nil; al = al.ArgumentExpressionList {
				s := strings.TrimSpace(cc.NodeSource(al.AssignmentExpression))
				if !atomic(s) {
					return nil
				}
				g.add(A(s))
			}
			f.add(g)
		default:
			return nil
		}
	}
	return f
}

// atomic reports whether s can be one atom: an identifier, a number, a
// literal with no space in it.  Anything else makes the attribute text, and
// the comparison with its source decides the rest.
func atomic(s string) bool { return s != "" && !strings.ContainsAny(s, " \t\n()") }

// ---- declarations

func (c *conv) decl(n *cc.Declaration) []*Node {
	if n == nil {
		return nil
	}
	if s, ok := c.macro(n); ok {
		return []*Node{c.at(L(A("macro-decl"), A(quote(s))), n)}
	}
	switch n.Case {
	case cc.DeclarationDecl:
		specs := c.declSpecs(n.DeclarationSpecifiers)
		if n.InitDeclaratorList == nil {
			if len(specs) == 0 {
				return nil
			}
			if len(specs) == 1 && (specs[0].n.Is("struct") || specs[0].n.Is("union") || specs[0].n.Is("enum")) {
				return []*Node{specs[0].n}
			}
			f := L(A("declare"))
			for _, s := range specs {
				f.add(s.n)
			}
			return []*Node{f}
		}
		// A DECLARATION WITH NO TYPE SPECIFIER -- C23's `auto x = 1;`,
		// whose type is inferred, or an old `static x;` -- has the empty
		// specifier list for its type: `(def auto x () 1)`.
		prefix, rest := splitPrefix(specs)
		var out []*Node
		for l := n.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator
			name, t := c.declarator(base(rest), d.Declarator)
			f := defHead(prefix).add(A(name), t)
			f.add(c.attrs(d.AttributeSpecifierList)...)
			if d.Asm != nil {
				f.add(L(A("asm-label"), A(quote(strings.TrimSpace(cc.NodeSource(d.Asm))))))
			}
			switch d.Case {
			case cc.InitDeclaratorDecl:
			case cc.InitDeclaratorInit:
				f.add(c.initializer(d.Initializer))
			default:
				c.fail(d, "init declarator %v", d.Case)
			}
			out = append(out, c.at(f, d.Declarator))
		}
		return out
	case cc.DeclarationAssert:
		return []*Node{c.staticAssert(n.StaticAssertDeclaration)}
	case cc.DeclarationAuto:
		// `__auto_type x = 1;` is `(def __auto_type x () 1)`.
		name, t := c.declarator(L(), n.Declarator)
		return []*Node{c.at(L(A("def"), A(tok(n.Token)), A(name), t, c.initializer(n.Initializer)), n.Declarator)}
	}
	c.fail(n, "declaration %v", n.Case)
	return nil
}

// defHead is `(def PREFIX...`, or `(typedef` when the prefix is that alone.
func defHead(prefix []*Node) *Node {
	if len(prefix) == 1 && !prefix[0].list && prefix[0].Atom == "typedef" {
		return L(A("typedef"))
	}
	return L(A("def")).add(prefix...)
}

func (c *conv) staticAssert(n *cc.StaticAssertDeclaration) *Node {
	f := L(A("static_assert"), c.expr(n.ConstantExpression, lvCond))
	if m := n.Token4.SrcStr(); m != "" {
		f.add(A(m))
	}
	return f
}

func (c *conv) funcDef(f *cc.FunctionDefinition) *Node {
	prefix, rest := splitPrefix(c.declSpecs(f.DeclarationSpecifiers))
	name, t := c.declarator(base(rest), f.Declarator)
	d := L(A("defn")).add(prefix...).add(A(name), t)
	// C23's attributes after the declarator follow the type, as a def's.
	d.add(c.attrs(f.AttributeSpecifierList)...)
	// AN OLD-STYLE DEFINITION'S PARAMETER DECLARATIONS are `(kr-params
	// DECL...)`, before the body's items.
	if f.DeclarationList != nil {
		kr := L(A("kr-params"))
		for l := f.DeclarationList; l != nil; l = l.DeclarationList {
			kr.add(c.decl(l.Declaration)...)
		}
		d.add(kr)
	}
	return c.at(d.add(c.blockItems(f.CompoundStatement.BlockItemList)...), f.Declarator)
}

// ---- initializers

func (c *conv) initializer(n *cc.Initializer) *Node {
	switch n.Case {
	case cc.InitializerExpr:
		return c.expr(n.AssignmentExpression, lvAssign)
	case cc.InitializerInitList:
		return L(A("init")).add(c.initItems(n.InitializerList)...)
	}
	c.fail(n, "initializer %v", n.Case)
	return A("?")
}

func (c *conv) initItems(n *cc.InitializerList) []*Node {
	var out []*Node
	for l := n; l != nil; l = l.InitializerList {
		v := c.initializer(l.Initializer)
		if l.Designation != nil {
			at := L(A("at"))
			for dl := l.Designation.DesignatorList; dl != nil; dl = dl.DesignatorList {
				d := dl.Designator
				switch d.Case {
				case cc.DesignatorIndex:
					at.add(L(A("idx"), c.expr(d.ConstantExpression, lvCond)))
				case cc.DesignatorIndex2:
					at.add(L(A("idx"), c.expr(d.ConstantExpression, lvCond), c.expr(d.ConstantExpression2, lvCond)))
				case cc.DesignatorField:
					at.add(c.at(A("."+tok(d.Token2)), d))
				case cc.DesignatorField2:
					at.add(c.at(A(tok(d.Token)+":"), d))
				default:
					c.fail(d, "designator %v", d.Case)
				}
			}
			v = c.at(at.add(v), l.Initializer)
		}
		out = append(out, v)
	}
	return out
}

// ---- statements

func (c *conv) blockItems(n *cc.BlockItemList) []*Node {
	var out []*Node
	for l := n; l != nil; l = l.BlockItemList {
		b := l.BlockItem
		switch b.Case {
		case cc.BlockItemDecl:
			if cemit.FuncName(b.Declaration) {
				continue
			}
			out = append(out, c.decl(b.Declaration)...)
		case cc.BlockItemStmt:
			out = append(out, c.stmt(b.Statement)...)
		case cc.BlockItemLabel:
			out = append(out, verbatim(cc.NodeSource(b.LabelDeclaration)))
		case cc.BlockItemFuncDef:
			c.fail(b, "a nested function definition")
		default:
			c.fail(b, "block item %v", b.Case)
		}
	}
	return out
}

func (c *conv) compound(n *cc.CompoundStatement) *Node {
	return L(A("block")).add(c.blockItems(n.BlockItemList)...)
}

// body is a statement where cemit writes a block: a compound as itself, any
// other statement braced -- which is the same text.
func (c *conv) body(n *cc.Statement) *Node {
	if n == nil {
		return L(A("block"))
	}
	if n.Case == cc.StatementCompound {
		return c.attributedBlock(n)
	}
	return L(A("block")).add(c.stmt(n)...)
}

// attributedBlock is a compound statement's block, C23's attributes before
// it its `(@ ATTR...)`, first: `(block (@ (std-attr likely)) ...)`.
func (c *conv) attributedBlock(n *cc.Statement) *Node {
	b := c.compound(n.CompoundStatement)
	if a := c.attrs(n.AttributeSpecifierList); len(a) > 0 {
		b.List = append([]*Node{b.List[0], L(A("@")).add(a...)}, b.List[1:]...)
	}
	return b
}

// stmt is one statement's forms.  A LABEL IS AN ITEM OF ITS OWN, `(label L)`,
// `(case 1)`, `(default)`, before the statement it labels, as C writes it:
// the text is the same, and a switch reads as one.
//
// C23'S ATTRIBUTES BEFORE A STATEMENT -- one that does not hold them itself,
// as an expression statement does (`attributed`) -- are an item of their own
// before it, `(stmt-attr ATTR...)`, as cemit prints them a line of their own;
// a block's are its `(@ ATTR...)`.
func (c *conv) stmt(n *cc.Statement) []*Node {
	if n == nil {
		return nil
	}
	if n.AttributeSpecifierList != nil {
		if n.Case == cc.StatementCompound {
			return []*Node{c.attributedBlock(n)}
		}
		return append([]*Node{L(A("stmt-attr")).add(c.attrs(n.AttributeSpecifierList)...)}, c.bareStmt(n)...)
	}
	return c.bareStmt(n)
}

// bareStmt is stmt without the attributes before the statement.
func (c *conv) bareStmt(n *cc.Statement) []*Node {
	if !c.noMacros {
		if s, ok := c.m.Stmt(n); ok {
			return []*Node{c.at(macroForm(strings.TrimSuffix(s, ";")), n)}
		}
	}
	if s, ok := c.macro(n); ok {
		return []*Node{c.at(macroForm(s), n)}
	}
	switch n.Case {
	case cc.StatementLabeled:
		ls := n.LabeledStatement
		var lab *Node
		switch ls.Case {
		case cc.LabeledStatementLabel:
			lab = c.at(L(A("label"), A(tok(ls.Token))), ls)
		case cc.LabeledStatementCaseLabel:
			lab = L(A("case"), c.expr(ls.ConstantExpression, lvCond))
		case cc.LabeledStatementRange:
			lab = L(A("case-range"), c.expr(ls.ConstantExpression, lvCond), c.expr(ls.ConstantExpression2, lvCond))
		case cc.LabeledStatementDefault:
			lab = L(A("default"))
		default:
			c.fail(ls, "labeled statement %v", ls.Case)
			return nil
		}
		return append([]*Node{lab}, c.stmt(ls.Statement)...)
	case cc.StatementCompound:
		return []*Node{c.compound(n.CompoundStatement)}
	case cc.StatementExpr:
		x := n.ExpressionStatement
		var a []*Node
		for _, s := range c.declSpecs(x.Specifiers()) {
			if !isAttr(s.n) {
				c.fail(x, "a specifier on an expression statement")
			}
			a = append(a, s.n)
		}
		a = append(a, c.attrs(x.AttributeSpecifierList)...)
		var e *Node
		if x.ExpressionList != nil {
			e = c.expr(x.ExpressionList, lvComma)
		}
		switch {
		case len(a) > 0:
			f := L(A("attributed")).add(a...)
			if e != nil {
				f.add(e)
			}
			return []*Node{f}
		case e == nil:
			return []*Node{L(A("empty"))}
		}
		return []*Node{e}
	case cc.StatementSelection:
		s := n.SelectionStatement
		switch s.Case {
		case cc.SelectionStatementIf, cc.SelectionStatementIfElse:
			return []*Node{c.ifForm(s)}
		case cc.SelectionStatementSwitch:
			return []*Node{L(A("switch"), c.expr(s.ExpressionList, lvComma), c.body(s.Statement))}
		}
		c.fail(s, "selection statement %v", s.Case)
	case cc.StatementIteration:
		return []*Node{c.iteration(n.IterationStatement)}
	case cc.StatementJump:
		return []*Node{c.jump(n.JumpStatement)}
	case cc.StatementAsm:
		return []*Node{verbatim(cc.NodeSource(n.AsmStatement))}
	default:
		c.fail(n, "statement %v", n.Case)
	}
	return nil
}

// ifForm is `(if C THEN)` or `(if C THEN ELSE)`.  AN ELSE THAT IS AN IF is
// that if, `(if C THEN (if ...))`, which ToC writes `else if` as cemit does;
// an else that is a block holding an if is a block.
func (c *conv) ifForm(s *cc.SelectionStatement) *Node {
	f := L(A("if"), c.expr(s.ExpressionList, lvComma), c.body(s.Statement))
	if s.Case == cc.SelectionStatementIfElse {
		e := s.Statement2
		if e != nil && e.Case == cc.StatementSelection && e.SelectionStatement != nil && e.AttributeSpecifierList == nil &&
			(e.SelectionStatement.Case == cc.SelectionStatementIf || e.SelectionStatement.Case == cc.SelectionStatementIfElse) {
			f.add(c.ifForm(e.SelectionStatement))
		} else {
			f.add(c.body(e))
		}
	}
	return f
}

// clause is a for-statement's clause: an expression, or `()` when it is not
// there.
func (c *conv) clause(n cc.ExpressionNode) *Node {
	if n == nil {
		return L()
	}
	return c.expr(n, lvComma)
}

func (c *conv) iteration(n *cc.IterationStatement) *Node {
	switch n.Case {
	case cc.IterationStatementWhile:
		return L(A("while"), c.expr(n.ExpressionList, lvComma), c.body(n.Statement))
	case cc.IterationStatementDo:
		return L(A("do"), c.body(n.Statement), c.expr(n.ExpressionList, lvComma))
	case cc.IterationStatementFor:
		return L(A("for"), c.clause(n.ExpressionList), c.clause(n.ExpressionList2), c.clause(n.ExpressionList3), c.body(n.Statement))
	case cc.IterationStatementForDecl:
		d := c.decl(n.Declaration)
		var init *Node
		switch len(d) {
		case 0:
			c.fail(n, "a for-declaration that declares nothing")
			return nil
		case 1:
			init = d[0]
		default:
			// cemit prints a for-declaration of more than one declarator as
			// the source has it.
			init = L(A("verbatim"), A(quote(strings.TrimSpace(cc.NodeSource(n.Declaration)))))
		}
		return L(A("for"), init, c.clause(n.ExpressionList), c.clause(n.ExpressionList2), c.body(n.Statement))
	}
	c.fail(n, "iteration statement %v", n.Case)
	return nil
}

func (c *conv) jump(n *cc.JumpStatement) *Node {
	switch n.Case {
	case cc.JumpStatementGoto:
		return L(A("goto"), c.at(A(tok(n.Token2)), n))
	case cc.JumpStatementGotoExpr:
		return L(A("goto*"), c.expr(n.ExpressionList, lvComma))
	case cc.JumpStatementContinue:
		return L(A("continue"))
	case cc.JumpStatementBreak:
		return L(A("break"))
	case cc.JumpStatementReturn:
		if n.ExpressionList == nil {
			return L(A("return"))
		}
		return L(A("return"), c.expr(n.ExpressionList, lvComma))
	}
	c.fail(n, "jump statement %v", n.Case)
	return nil
}
