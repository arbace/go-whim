package togo

// cpp_decl.go prints the C's declarations as C++ declares them: the
// canonical printer's shape (crefactor/cemit), the declarator as the
// grammar has it, with what C++ says otherwise -- no storage class (the
// class's scope is the file's), an enumerated type its integer type, a
// pointer to a function a pointer to the editor's member, typeof C++'s
// decltype, a name C++ reserves renamed.

import (
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// declSpecs prints a declaration's specifiers without its storage class
// and function specifiers; a struct or enumeration defined there is named
// (its definition is written before, at namespace scope) unless keepDef
// and it is untagged.
func (c *cppgen) declSpecs(n *cc.DeclarationSpecifiers, keepDef bool) string {
	var parts []string
	for l := n; l != nil; l = l.DeclarationSpecifiers {
		switch l.Case {
		case cc.DeclarationSpecifiersStorage, cc.DeclarationSpecifiersFunc:
		case cc.DeclarationSpecifiersTypeSpec:
			parts = append(parts, c.typeSpec(l.TypeSpecifier, keepDef))
		case cc.DeclarationSpecifiersTypeQual:
			if q := l.TypeQualifier; q.Case == cc.TypeQualifierAttr {
				parts = append(parts, c.attrs(q.AttributeSpecifierList))
				break
			}
			parts = append(parts, c.qual(l.TypeQualifier.Token.SrcStr()))
		case cc.DeclarationSpecifiersAlignSpec:
			parts = append(parts, c.alignSpec(l.AlignmentSpecifier))
		case cc.DeclarationSpecifiersAttr:
			parts = append(parts, c.attrs(l.AttributeSpecifierList))
		default:
			c.fail(l, "declaration specifier %v", l.Case)
		}
	}
	return strings.Join(nonEmptyS(parts), " ")
}

// qual is a type qualifier as C++ spells it.
func (c *cppgen) qual(q string) string {
	switch q {
	case "restrict", "__restrict", "__restrict__":
		return "__restrict"
	case "_Atomic":
		c.fail(nil, "_Atomic")
	}
	return q
}

func (c *cppgen) specQuals(n *cc.SpecifierQualifierList, keepDef bool) string {
	var parts []string
	for l := n; l != nil; l = l.SpecifierQualifierList {
		switch l.Case {
		case cc.SpecifierQualifierListTypeSpec:
			parts = append(parts, c.typeSpec(l.TypeSpecifier, keepDef))
		case cc.SpecifierQualifierListTypeQual:
			if q := l.TypeQualifier; q.Case == cc.TypeQualifierAttr {
				parts = append(parts, c.attrs(q.AttributeSpecifierList))
				break
			}
			parts = append(parts, c.qual(l.TypeQualifier.Token.SrcStr()))
		case cc.SpecifierQualifierListAlignSpec:
			parts = append(parts, c.alignSpec(l.AlignmentSpecifier))
		default:
			c.fail(l, "specifier-qualifier %v", l.Case)
		}
	}
	return strings.Join(nonEmptyS(parts), " ")
}

func (c *cppgen) alignSpec(n *cc.AlignmentSpecifier) string {
	if n == nil {
		return ""
	}
	switch n.Case {
	case cc.AlignmentSpecifierType:
		return "alignas(" + c.typeName(n.TypeName) + ")"
	case cc.AlignmentSpecifierExpr:
		return "alignas(" + c.expr(n.ConstantExpression) + ")"
	}
	c.fail(n, "alignment specifier %v", n.Case)
	return ""
}

// typeSpec prints one type specifier.  An enumeration is its integer type;
// a struct defined here is named by its tag, or, untagged and keepDef,
// defined in place.
func (c *cppgen) typeSpec(n *cc.TypeSpecifier, keepDef bool) string {
	switch n.Case {
	case cc.TypeSpecifierBool:
		return "bool"
	case cc.TypeSpecifierVoid, cc.TypeSpecifierChar, cc.TypeSpecifierShort, cc.TypeSpecifierInt,
		cc.TypeSpecifierLong, cc.TypeSpecifierSigned, cc.TypeSpecifierUnsigned:
		return n.Token.SrcStr()
	case cc.TypeSpecifierTypeName:
		return cppName(n.Token.SrcStr())
	case cc.TypeSpecifierStructOrUnion:
		return c.structRef(n.StructOrUnionSpecifier, keepDef)
	case cc.TypeSpecifierEnum:
		if e := c.scoped(n.EnumSpecifier.Type()); e != nil {
			return cppName(e.name)
		}
		if e, ok := n.EnumSpecifier.Type().(*cc.EnumType); ok {
			return c.scalar(e.UnderlyingType())
		}
		return c.scalar(n.EnumSpecifier.Type())
	case cc.TypeSpecifierTypeofExpr:
		return "decltype(" + c.expr(n.ExpressionList) + ")"
	case cc.TypeSpecifierTypeofType:
		return c.typeName(n.TypeName)
	}
	c.fail(n, "type specifier %v", n.Case)
	return ""
}

// structRef names a struct, or defines an untagged one in place.
func (c *cppgen) structRef(n *cc.StructOrUnionSpecifier, keepDef bool) string {
	kw := n.StructOrUnion.Token.SrcStr()
	tag := n.Token.SrcStr()
	if n.Case == cc.StructOrUnionSpecifierDef && tag == "" {
		if !keepDef {
			c.fail(n, "an untagged struct where its definition cannot be")
		}
		return c.structOrUnion(n)
	}
	return c.tagRef(kw, tag)
}

// tagRef names a struct by its tag alone, as C++ does (item 2) -- but
// where an ordinary name is the tag's too, which would hide it.
func (c *cppgen) tagRef(kw, tag string) string {
	if c.ordinary[tag] {
		return kw + " " + tag
	}
	return tag
}

// structOrUnion prints a struct's or a union's definition, a member a line.
func (c *cppgen) structOrUnion(n *cc.StructOrUnionSpecifier) string {
	return c.structNamed(n, n.Token.SrcStr())
}

// structNamed prints a struct's or a union's definition under name.
func (c *cppgen) structNamed(n *cc.StructOrUnionSpecifier, name string) string {
	kw := n.StructOrUnion.Token.SrcStr()
	var b strings.Builder
	b.WriteString(cppJoin(kw, name) + "\n" + c.pad() + "{\n")
	c.indent++
	for l := n.StructDeclarationList; l != nil; l = l.StructDeclarationList {
		for _, s := range c.structDecl(l.StructDeclaration) {
			b.WriteString(c.pad() + s + "\n")
		}
	}
	c.indent--
	b.WriteString(c.pad() + "}")
	return b.String()
}

// structDecl prints one member declaration, a declarator a line.
func (c *cppgen) structDecl(n *cc.StructDeclaration) []string {
	switch n.Case {
	case cc.StructDeclarationDecl:
		sq := c.specQuals(n.SpecifierQualifierList, true)
		if n.StructDeclaratorList == nil {
			return []string{sq + ";"}
		}
		var out []string
		for l := n.StructDeclaratorList; l != nil; l = l.StructDeclaratorList {
			d := l.StructDeclarator
			var s string
			switch d.Case {
			case cc.StructDeclaratorDecl:
				s = cppJoin(sq, c.declarator(d.Declarator, "", -1))
			case cc.StructDeclaratorBitField:
				s = cppJoin(sq, c.declarator(d.Declarator, "", -1)) + " : " + c.expr(d.ConstantExpression)
			default:
				c.fail(d, "struct declarator %v", d.Case)
			}
			out = append(out, s+";")
		}
		return out
	case cc.StructDeclarationAssert:
		return []string{c.staticAssert(n.StaticAssertDeclaration)}
	}
	c.fail(n, "struct declaration %v", n.Case)
	return nil
}

// declarator prints a declarator; name, when set, replaces its identifier,
// and arrLen, when not negative, fills its outermost array's empty size.
func (c *cppgen) declarator(n *cc.Declarator, name string, arrLen int64) string {
	if n == nil {
		return ""
	}
	return c.pointer(n.Pointer, false) + c.directDeclarator(n.DirectDeclarator, name, arrLen)
}

// pointer prints a declarator's pointers; fn says the leftmost points at a
// function, which makes it a pointer to the editor's member.
func (c *cppgen) pointer(n *cc.Pointer, fn bool) string {
	if n == nil {
		return ""
	}
	star := "*"
	if fn {
		star = "Editor::*"
		c.nFnPtr++
	}
	switch n.Case {
	case cc.PointerTypeQual:
		return star + qualSp(c.typeQuals(n.TypeQualifiers))
	case cc.PointerPtr:
		return star + qualSp(c.typeQuals(n.TypeQualifiers)) + c.pointer(n.Pointer, false)
	}
	c.fail(n, "pointer %v", n.Case)
	return ""
}

func qualSp(q string) string {
	if q == "" {
		return ""
	}
	return q + " "
}

func (c *cppgen) typeQuals(n *cc.TypeQualifiers) string {
	var parts []string
	for l := n; l != nil; l = l.TypeQualifiers {
		q := l.TypeQualifier
		switch {
		case q == nil:
		case q.Case == cc.TypeQualifierAttr:
		default:
			parts = append(parts, c.qual(q.Token.SrcStr()))
		}
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(nonEmptyS(parts), " ")
}

func (c *cppgen) directDeclarator(n *cc.DirectDeclarator, name string, arrLen int64) string {
	if n == nil {
		return ""
	}
	switch n.Case {
	case cc.DirectDeclaratorIdent:
		if name != "" {
			return name
		}
		return cppName(n.Token.SrcStr())
	case cc.DirectDeclaratorDecl:
		if n.Declarator.Pointer == nil {
			// (a[3]): parentheses that say nothing
			return c.declarator(n.Declarator, name, arrLen)
		}
		return "(" + c.declarator(n.Declarator, name, arrLen) + ")"
	case cc.DirectDeclaratorArr:
		size := c.expr(n.AssignmentExpression)
		if size == "" && arrLen >= 0 {
			size = strconv.FormatInt(arrLen, 10)
		}
		return c.directDeclarator(n.DirectDeclarator, name, -1) + "[" + cppJoin(c.typeQuals(n.TypeQualifiers), size) + "]"
	case cc.DirectDeclaratorFuncParam:
		inner := n.DirectDeclarator
		if inner.Case == cc.DirectDeclaratorDecl && inner.Declarator.Pointer != nil {
			// (*f)(...): a pointer to the editor's member function
			d := inner.Declarator
			return "(" + c.pointer(d.Pointer, true) + c.directDeclarator(d.DirectDeclarator, name, arrLen) + ")(" + c.params(n.ParameterTypeList) + ")"
		}
		return c.directDeclarator(inner, name, arrLen) + "(" + c.params(n.ParameterTypeList) + ")"
	}
	c.fail(n, "direct declarator %v", n.Case)
	return ""
}

func (c *cppgen) params(n *cc.ParameterTypeList) string {
	if n == nil {
		return ""
	}
	var parts []string
	for l := n.ParameterList; l != nil; l = l.ParameterList {
		p := l.ParameterDeclaration
		switch p.Case {
		case cc.ParameterDeclarationDecl:
			unused := ""
			if d := p.Declarator; d != nil && d.ReadCount() == 0 && !d.AddressTaken() {
				// a parameter the function never reads, as C's
				// __attribute__((unused)) said before the sweep took them
				unused = "[[maybe_unused]]"
			}
			decl := c.declarator(p.Declarator, "", -1)
			if c.refDecl[p.Declarator] {
				// T *p, never null: T &p
				decl = "&" + strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(decl, "*"), "const "), "__restrict ")
			}
			parts = append(parts, cppJoin(unused, c.declSpecs(p.DeclarationSpecifiers, false), decl))
		case cc.ParameterDeclarationAbstract:
			parts = append(parts, cppJoin(c.declSpecs(p.DeclarationSpecifiers, false), c.abstract(p.AbstractDeclarator)))
		default:
			c.fail(p, "parameter declaration %v", p.Case)
		}
	}
	if n.Case == cc.ParameterTypeListVar {
		parts = append(parts, "...")
	}
	return strings.Join(nonEmptyS(parts), ", ")
}

func (c *cppgen) abstract(n *cc.AbstractDeclarator) string {
	if n == nil {
		return ""
	}
	return c.pointer(n.Pointer, false) + c.directAbstract(n.DirectAbstractDeclarator)
}

func (c *cppgen) directAbstract(n *cc.DirectAbstractDeclarator) string {
	if n == nil {
		return ""
	}
	switch n.Case {
	case cc.DirectAbstractDeclaratorDecl:
		return "(" + c.abstract(n.AbstractDeclarator) + ")"
	case cc.DirectAbstractDeclaratorArr:
		return c.directAbstract(n.DirectAbstractDeclarator) + "[" + cppJoin(c.typeQuals(n.TypeQualifiers), c.expr(n.AssignmentExpression)) + "]"
	case cc.DirectAbstractDeclaratorFunc:
		inner := n.DirectAbstractDeclarator
		if inner != nil && inner.Case == cc.DirectAbstractDeclaratorDecl && inner.AbstractDeclarator != nil && inner.AbstractDeclarator.Pointer != nil {
			a := inner.AbstractDeclarator
			return "(" + c.pointer(a.Pointer, true) + c.directAbstract(a.DirectAbstractDeclarator) + ")(" + c.params(n.ParameterTypeList) + ")"
		}
		return c.directAbstract(inner) + "(" + c.params(n.ParameterTypeList) + ")"
	}
	c.fail(n, "direct abstract declarator %v", n.Case)
	return ""
}

func (c *cppgen) typeName(n *cc.TypeName) string {
	if n == nil {
		return ""
	}
	return cppJoin(c.specQuals(n.SpecifierQualifierList, false), c.abstract(n.AbstractDeclarator))
}

func (c *cppgen) staticAssert(n *cc.StaticAssertDeclaration) string {
	if n.Token4.SrcStr() == "" {
		return "static_assert(" + c.expr(n.ConstantExpression) + ");"
	}
	return "static_assert(" + c.expr(n.ConstantExpression) + ", " + n.Token4.SrcStr() + ");"
}

// attrs prints C23's attributes, which are C++'s; GNU's are dropped.
func (c *cppgen) attrs(n *cc.AttributeSpecifierList) string {
	var parts []string
	for l := n; l != nil; l = l.AttributeSpecifierList {
		s := cc.NodeSource(l.AttributeSpecifier)
		if strings.Contains(s, "__attribute__") {
			continue
		}
		parts = append(parts, strings.TrimSpace(s))
	}
	return strings.Join(nonEmptyS(parts), " ")
}

func cppJoin(parts ...string) string { return strings.Join(nonEmptyS(parts), " ") }

func nonEmptyS(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
