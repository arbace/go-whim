package cemit

import (
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// THE DECLARATOR IS WHERE C IS HARD.  A declaration is a specifier list and a
// declarator, and the declarator is read outward from the name: `char *(*f[4])(int)`
// is an array of four pointers to functions returning a pointer to char.  This
// prints it the way the grammar has it -- a DirectDeclarator wrapping a
// Declarator wrapping a Pointer -- so the shape comes back as it went in,
// without the printer having to understand what it means.

// declSpecs prints a declaration's specifiers, in source order, one space apart.
func (e *emitter) declSpecs(n *cc.DeclarationSpecifiers) string {
	var parts []string
	for l := n; l != nil; l = l.DeclarationSpecifiers {
		switch l.Case {
		case cc.DeclarationSpecifiersStorage:
			parts = append(parts, tok(l.StorageClassSpecifier.Token))
		case cc.DeclarationSpecifiersTypeSpec:
			parts = append(parts, e.typeSpec(l.TypeSpecifier))
		case cc.DeclarationSpecifiersTypeQual:
			parts = append(parts, tok(l.TypeQualifier.Token))
		case cc.DeclarationSpecifiersFunc:
			parts = append(parts, tok(l.FunctionSpecifier.Token))
		case cc.DeclarationSpecifiersAlignSpec:
			parts = append(parts, e.alignSpec(l.AlignmentSpecifier))
		case cc.DeclarationSpecifiersAttr:
			parts = append(parts, e.attrs(l.AttributeSpecifierList))
		default:
			e.fail(l, "declaration specifier %v", l.Case)
		}
	}
	return strings.Join(nonEmpty(parts), " ")
}

func (e *emitter) specQuals(n *cc.SpecifierQualifierList) string {
	var parts []string
	for l := n; l != nil; l = l.SpecifierQualifierList {
		switch l.Case {
		case cc.SpecifierQualifierListTypeSpec:
			parts = append(parts, e.typeSpec(l.TypeSpecifier))
		case cc.SpecifierQualifierListTypeQual:
			parts = append(parts, tok(l.TypeQualifier.Token))
		case cc.SpecifierQualifierListAlignSpec:
			parts = append(parts, e.alignSpec(l.AlignmentSpecifier))
		default:
			e.fail(l, "specifier-qualifier %v", l.Case)
		}
	}
	return strings.Join(nonEmpty(parts), " ")
}

func (e *emitter) alignSpec(n *cc.AlignmentSpecifier) string {
	if n == nil {
		return ""
	}
	switch n.Case {
	case cc.AlignmentSpecifierType:
		return "alignas(" + e.typeName(n.TypeName) + ")"
	case cc.AlignmentSpecifierExpr:
		return "alignas(" + e.expr(n.ConstantExpression) + ")"
	}
	e.fail(n, "alignment specifier %v", n.Case)
	return ""
}

// typeSpec prints one type specifier.  A struct, union or enum DEFINITION is
// printed by its own writer, which needs lines; here it is asked for as a block
// and spliced in, which is why a definition inside an expression (a compound
// literal's type, say) still comes out with its members one per line.
func (e *emitter) typeSpec(n *cc.TypeSpecifier) string {
	switch n.Case {
	case cc.TypeSpecifierVoid, cc.TypeSpecifierChar, cc.TypeSpecifierShort, cc.TypeSpecifierInt,
		cc.TypeSpecifierInt128, cc.TypeSpecifierUint128, cc.TypeSpecifierLong, cc.TypeSpecifierFloat,
		cc.TypeSpecifierFloat16, cc.TypeSpecifierBFloat16, cc.TypeSpecifierDecimal32,
		cc.TypeSpecifierDecimal64, cc.TypeSpecifierDecimal128, cc.TypeSpecifierFloat128,
		cc.TypeSpecifierFloat128x, cc.TypeSpecifierDouble, cc.TypeSpecifierSigned,
		cc.TypeSpecifierUnsigned, cc.TypeSpecifierBool, cc.TypeSpecifierComplex,
		cc.TypeSpecifierImaginary, cc.TypeSpecifierTypeName, cc.TypeSpecifierFloat32,
		cc.TypeSpecifierFloat64, cc.TypeSpecifierFloat32x, cc.TypeSpecifierFloat64x:
		return e.specToken(n.Token)
	case cc.TypeSpecifierStructOrUnion:
		return e.structOrUnion(n.StructOrUnionSpecifier)
	case cc.TypeSpecifierEnum:
		return e.enum(n.EnumSpecifier)
	case cc.TypeSpecifierTypeofExpr:
		return tok(n.Token) + "(" + e.expr(n.ExpressionList) + ")"
	case cc.TypeSpecifierTypeofType:
		return tok(n.Token) + "(" + e.typeName(n.TypeName) + ")"
	case cc.TypeSpecifierAtomic:
		return e.atomic(n.AtomicTypeSpecifier)
	}
	e.fail(n, "type specifier %v", n.Case)
	return ""
}

func (e *emitter) atomic(n *cc.AtomicTypeSpecifier) string {
	return "_Atomic(" + e.typeName(n.TypeName) + ")"
}

// structOrUnion prints a struct or union, with its members one per line when it
// is a definition.
func (e *emitter) structOrUnion(n *cc.StructOrUnionSpecifier) string {
	kw := tok(n.StructOrUnion.Token)
	switch n.Case {
	case cc.StructOrUnionSpecifierTag:
		return join(kw, e.attrs(n.AttributeSpecifierList), tok(n.Token))
	case cc.StructOrUnionSpecifierDef:
		head := join(kw, e.attrs(n.AttributeSpecifierList), tok(n.Token))
		var b strings.Builder
		b.WriteString(head + "\n" + e.pad() + "{\n")
		e.indent++
		for l := n.StructDeclarationList; l != nil; l = l.StructDeclarationList {
			for _, s := range e.structDecl(l.StructDeclaration) {
				b.WriteString(e.pad() + s + "\n")
			}
		}
		e.indent--
		b.WriteString(e.pad() + "}")
		if a := e.attrs(n.AttributeSpecifierList2); a != "" {
			b.WriteString(" " + a)
		}
		return b.String()
	}
	e.fail(n, "struct-or-union specifier %v", n.Case)
	return ""
}

// structDecl prints one member declaration -- ONE DECLARATOR PER LINE, which is
// one of the forms this printer reduces: `int x, y;` becomes two lines.
func (e *emitter) structDecl(n *cc.StructDeclaration) []string {
	switch n.Case {
	case cc.StructDeclarationDecl:
		sq := e.specQuals(n.SpecifierQualifierList)
		if n.StructDeclaratorList == nil {
			return []string{sq + ";"}
		}
		var out []string
		for l := n.StructDeclaratorList; l != nil; l = l.StructDeclaratorList {
			d := l.StructDeclarator
			switch d.Case {
			case cc.StructDeclaratorDecl:
				out = append(out, join(sq, e.declarator(d.Declarator))+";")
			case cc.StructDeclaratorBitField:
				out = append(out, join(sq, e.declarator(d.Declarator))+" : "+e.expr(d.ConstantExpression)+";")
			default:
				e.fail(d, "struct declarator %v", d.Case)
			}
		}
		return out
	case cc.StructDeclarationAssert:
		return []string{e.staticAssert(n.StaticAssertDeclaration)}
	}
	e.fail(n, "struct declaration %v", n.Case)
	return nil
}

func (e *emitter) enum(n *cc.EnumSpecifier) string {
	// C23'S FIXED UNDERLYING TYPE IS PART OF THE TYPE.  `enum : long { ... }` is
	// what phase 109 wrote the header limits as, and dropping the `: long`
	// leaves the constants `int`: measured, four comparisons in the core then
	// warn -Wsign-compare that did not.
	under := ""
	if n.EnumTypeSpecifier != nil {
		under = " : " + e.specQuals(n.EnumTypeSpecifier.SpecifierQualifierList)
	}
	switch n.Case {
	case cc.EnumSpecifierTag:
		return join("enum"+under, tok(n.Token2))
	case cc.EnumSpecifierDef:
		head := join("enum"+under, tok(n.Token2))
		// ONE ENUMERATOR IS ONE LINE.  `enum { EXTRA_MARKS = 10 };` is how this
		// tree spells a constant -- arbace/slim-vim writes every `#define` of a
		// number that way, 1,448 of them -- and the pipeline reads, rewrites and
		// WRITES that line: `enum { %s = %d };` is emitted by two phases and
		// matched by six checks.  A constant is a line here, as a table's row is.
		if l := n.EnumeratorList; l != nil && l.EnumeratorList == nil {
			return head + " { " + e.enumerator(l.Enumerator) + " }"
		}
		var b strings.Builder
		b.WriteString(head + "\n" + e.pad() + "{\n")
		e.indent++
		for l := n.EnumeratorList; l != nil; l = l.EnumeratorList {
			b.WriteString(e.pad() + e.enumerator(l.Enumerator) + ",\n")
		}
		e.indent--
		b.WriteString(e.pad() + "}")
		return b.String()
	}
	e.fail(n, "enum specifier %v", n.Case)
	return ""
}

func (e *emitter) enumerator(n *cc.Enumerator) string {
	switch n.Case {
	case cc.EnumeratorIdent:
		return tok(n.Token)
	case cc.EnumeratorExpr:
		return tok(n.Token) + " = " + e.expr(n.ConstantExpression)
	}
	e.fail(n, "enumerator %v", n.Case)
	return ""
}

// declarator prints a declarator: its pointers, then its direct declarator.
func (e *emitter) declarator(n *cc.Declarator) string {
	if n == nil {
		return ""
	}
	return e.pointer(n.Pointer) + e.directDeclarator(n.DirectDeclarator)
}

func (e *emitter) pointer(n *cc.Pointer) string {
	if n == nil {
		return ""
	}
	switch n.Case {
	case cc.PointerTypeQual:
		return "*" + qualsSpace(e.typeQuals(n.TypeQualifiers))
	case cc.PointerPtr:
		return "*" + qualsSpace(e.typeQuals(n.TypeQualifiers)) + e.pointer(n.Pointer)
	}
	e.fail(n, "pointer %v", n.Case)
	return ""
}

func (e *emitter) typeQuals(n *cc.TypeQualifiers) string {
	var parts []string
	for l := n; l != nil; l = l.TypeQualifiers {
		if l.TypeQualifier != nil {
			parts = append(parts, tok(l.TypeQualifier.Token))
		}
	}
	return strings.Join(nonEmpty(parts), " ")
}

func (e *emitter) directDeclarator(n *cc.DirectDeclarator) string {
	if n == nil {
		return ""
	}
	switch n.Case {
	case cc.DirectDeclaratorIdent:
		return tok(n.Token)
	case cc.DirectDeclaratorDecl:
		return "(" + e.declarator(n.Declarator) + ")"
	case cc.DirectDeclaratorArr:
		return e.directDeclarator(n.DirectDeclarator) + "[" +
			joinSp(e.typeQuals(n.TypeQualifiers), e.expr(n.AssignmentExpression)) + "]"
	case cc.DirectDeclaratorStar:
		return e.directDeclarator(n.DirectDeclarator) + "[" +
			joinSp(e.typeQuals(n.TypeQualifiers), "*") + "]"
	case cc.DirectDeclaratorFuncParam:
		return e.directDeclarator(n.DirectDeclarator) + "(" + e.params(n.ParameterTypeList) + ")"
	case cc.DirectDeclaratorFuncIdent:
		return e.directDeclarator(n.DirectDeclarator) + "(" + e.identList(n.IdentifierList) + ")"
	}
	e.fail(n, "direct declarator %v", n.Case)
	return ""
}

func (e *emitter) identList(n *cc.IdentifierList) string {
	var parts []string
	for l := n; l != nil; l = l.IdentifierList {
		parts = append(parts, tok(l.Token))
	}
	return strings.Join(nonEmpty(parts), ", ")
}

func (e *emitter) params(n *cc.ParameterTypeList) string {
	if n == nil {
		return ""
	}
	var parts []string
	for l := n.ParameterList; l != nil; l = l.ParameterList {
		p := l.ParameterDeclaration
		switch p.Case {
		// A PARAMETER'S ATTRIBUTE IS PART OF IT.  The input says
		// `spellvars_T *spv __attribute__((unused))` 303 times, which is how it
		// keeps -Wunused-parameter quiet where a parameter is deliberately
		// ignored; dropping them printed 316 attributes as 13.
		case cc.ParameterDeclarationDecl:
			parts = append(parts, join(e.declSpecs(p.DeclarationSpecifiers),
				e.declarator(p.Declarator), e.attrs(p.AttributeSpecifierList)))
		case cc.ParameterDeclarationAbstract:
			parts = append(parts, join(e.declSpecs(p.DeclarationSpecifiers),
				e.abstract(p.AbstractDeclarator), e.attrs(p.AttributeSpecifierList)))
		default:
			e.fail(p, "parameter declaration %v", p.Case)
		}
	}
	if n.Case == cc.ParameterTypeListVar {
		parts = append(parts, "...")
	}
	return strings.Join(nonEmpty(parts), ", ")
}

func (e *emitter) abstract(n *cc.AbstractDeclarator) string {
	if n == nil {
		return ""
	}
	// BOTH CASES PRINT BOTH PARTS.  `AbstractDeclaratorPtr` is the grammar's
	// pointer-only production, but the parser also builds it for the
	// direct-abstract-declarator alone -- `(*)[N]` in a cast arrives with a nil
	// Pointer and a DirectAbstractDeclarator.  Printing only the pointer turned
	// `(score_t(*)[N])block` into `(score_t)block`, which gcc rejected.
	return e.pointer(n.Pointer) + e.directAbstract(n.DirectAbstractDeclarator)
}

func (e *emitter) directAbstract(n *cc.DirectAbstractDeclarator) string {
	if n == nil {
		return ""
	}
	switch n.Case {
	case cc.DirectAbstractDeclaratorDecl:
		return "(" + e.abstract(n.AbstractDeclarator) + ")"
	case cc.DirectAbstractDeclaratorArr:
		return e.directAbstract(n.DirectAbstractDeclarator) + "[" +
			joinSp(e.typeQuals(n.TypeQualifiers), e.expr(n.AssignmentExpression)) + "]"
	case cc.DirectAbstractDeclaratorArrStar:
		return e.directAbstract(n.DirectAbstractDeclarator) + "[*]"
	case cc.DirectAbstractDeclaratorFunc:
		return e.directAbstract(n.DirectAbstractDeclarator) + "(" + e.params(n.ParameterTypeList) + ")"
	}
	e.fail(n, "direct abstract declarator %v", n.Case)
	return ""
}

func (e *emitter) typeName(n *cc.TypeName) string {
	if n == nil {
		return ""
	}
	return join(e.specQuals(n.SpecifierQualifierList), e.abstract(n.AbstractDeclarator))
}

// staticAssert prints `static_assert(expr, "message");`.  The message is
// Token4 -- Token3 is the comma -- and cc/v4's parser requires it, so the C23
// one-argument form would not have parsed in the first place.
func (e *emitter) staticAssert(n *cc.StaticAssertDeclaration) string {
	if n.Token4.SrcStr() == "" {
		return "static_assert(" + e.expr(n.ConstantExpression) + ");"
	}
	return "static_assert(" + e.expr(n.ConstantExpression) + ", " + tok(n.Token4) + ");"
}

// attrs prints a C23 attribute list, `[[...]]`, as the source had it.
func (e *emitter) attrs(n *cc.AttributeSpecifierList) string {
	if n == nil {
		return ""
	}
	var parts []string
	for l := n; l != nil; l = l.AttributeSpecifierList {
		parts = append(parts, cc.NodeSource(l.AttributeSpecifier))
	}
	return strings.Join(nonEmpty(parts), " ")
}

func (e *emitter) pad() string { return strings.Repeat(Indent, e.indent) }

func join(parts ...string) string   { return strings.Join(nonEmpty(parts), " ") }
func joinSp(parts ...string) string { return strings.Join(nonEmpty(parts), " ") }

func qualsSpace(q string) string {
	if q == "" {
		return ""
	}
	return q + " "
}

func nonEmpty(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// declLines prints one declaration as one line per declarator -- the reduction
// that costs the most and buys the most: `int a, b = 1;` is two lines, so a
// tool looking for a declaration of `b` finds a whole line and not a fragment
// between commas.
func (e *emitter) declLines(n *cc.Declaration) []string {
	if n == nil {
		return nil
	}
	if s, ok := e.fromMacro(n); ok {
		return []string{s}
	}
	switch n.Case {
	case cc.DeclarationDecl:
		specs := e.declSpecs(n.DeclarationSpecifiers)
		if n.InitDeclaratorList == nil {
			// A DECLARATION THAT DECLARES NOTHING IS NOT PRINTED.  A `;` after
			// a static_assert or a struct definition parses as an empty
			// declaration of its own, and printing it would add a line every
			// time the printer ran -- the fixpoint would not exist.
			if specs == "" {
				return nil
			}
			return []string{specs + ";"}
		}
		var out []string
		for l := n.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator
			s := join(specs, e.declarator(d.Declarator))
			if a := e.attrs(d.AttributeSpecifierList); a != "" {
				s = join(s, a)
			}
			if d.Asm != nil {
				s = join(s, strings.TrimSpace(cc.NodeSource(d.Asm)))
			}
			switch d.Case {
			case cc.InitDeclaratorDecl:
			case cc.InitDeclaratorInit:
				s += " " + e.assign(d.Initializer)
			default:
				e.fail(d, "init declarator %v", d.Case)
			}
			out = append(out, s+";")
		}
		return out
	case cc.DeclarationAssert:
		return []string{e.staticAssert(n.StaticAssertDeclaration)}
	case cc.DeclarationAuto:
		return []string{join(e.declSpecs(n.DeclarationSpecifiers), e.declarator(n.Declarator)) +
			" " + e.assign(n.Initializer) + ";"}
	}
	e.fail(n, "declaration %v", n.Case)
	return nil
}

// assign writes `= <initializer>`, with no space before a braced list that
// begins on the next line.
func (e *emitter) assign(n *cc.Initializer) string {
	s := e.initializer(n)
	if strings.HasPrefix(s, "\n") {
		return "=" + s
	}
	return "= " + s
}
