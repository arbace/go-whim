package cc // import "github.com/arbace/go-whim/crefactor/cc"

import "modernc.org/token"

// go-whim: the C23 productions upstream's parser lacks, the new functions in
// one file so that the delta in upstream's own files stays a set of hooks.
// README.md lists them; crefactor/c23conf is their conformance test.

// stdAttrAt reports whether the token at lookahead i begins a C23 attribute
// specifier, `[[` (6.7.13.2).  Nowhere else in C do two '[' meet: an array
// size or a subscript cannot begin with one.
func (p *parser) stdAttrAt(i int) bool {
	return p.peek(i, false).Ch == '[' && p.peek(i+1, false).Ch == '['
}

// gnuAttributeSpecifierListOpt is upstream's attributeSpecifierListOpt, GNU
// attributes only.  It is called where upstream parses an attribute list and
// discards it: a C23 [[...]] there stays a syntax error, never a silent drop.
func (p *parser) gnuAttributeSpecifierListOpt() (r *AttributeSpecifierList) {
	var prev *AttributeSpecifierList
	for p.rune(false) == rune(ATTRIBUTE) {
		asl := &AttributeSpecifierList{AttributeSpecifier: p.attributeSpecifier()}
		if prev == nil {
			r = asl
		} else {
			prev.AttributeSpecifierList = asl
		}
		prev = asl
	}
	return r
}

// stdAttributeSpecifier parses one C23 attribute specifier, 6.7.13.2:
//
//	attribute-specifier:
//		[ [ attribute-list ] ]
//
// held in the node GNU's __attribute__((...)) is, '[' '[' in Token and
// Token2, ']' ']' in Token4 and Token5, Token3 zero.
func (p *parser) stdAttributeSpecifier() *AttributeSpecifier {
	r := &AttributeSpecifier{Token: p.shift(false), Token2: p.shift(false)}
	r.AttributeValueList = p.stdAttributeListOpt()
	r.Token4 = p.must(']')
	r.Token5 = p.must(']')
	return r
}

// IsStd reports whether n is a C23 attribute specifier, `[[...]]`, and not
// GNU's `__attribute__((...))`.
func (n *AttributeSpecifier) IsStd() bool { return n != nil && n.Token.Ch == '[' }

// stdAttributeListOpt parses
//
//	attribute-list:
//		attribute_opt
//		attribute-list , attribute_opt
//
// An attribute left out is an AttributeValueList with a nil AttributeValue:
// `[[]]` is no list, `[[a,]]` two elements, the second empty.
func (p *parser) stdAttributeListOpt() (r *AttributeValueList) {
	var prev *AttributeValueList
	add := func(l *AttributeValueList) {
		if prev == nil {
			r = l
		} else {
			prev.AttributeValueList = l
		}
		prev = l
	}
	present := func() bool { ch := p.rune(false); return ch != ',' && ch != ']' && ch != eof }
	if present() {
		add(&AttributeValueList{AttributeValue: p.stdAttribute()})
	}
	for p.rune(false) == ',' {
		l := &AttributeValueList{Token: p.shift(false)}
		if present() {
			l.AttributeValue = p.stdAttribute()
		}
		add(l)
	}
	return r
}

// stdAttribute parses
//
//	attribute:
//		attribute-token attribute-argument-clause_opt
//	attribute-token:
//		identifier
//		attribute-prefix :: identifier
//	attribute-argument-clause:
//		( balanced-token-sequence_opt )
//
// The argument clause of a standard attribute or a gnu:: one is parsed as
// GNU's is, an argument expression list, which is what each of them takes;
// another vendor's is kept as its tokens.  A keyword is an identifier here
// (`[[gnu::const]]`).
func (p *parser) stdAttribute() *AttributeValue {
	r := &AttributeValue{Case: AttributeValueIdent}
	if p.peek(1, false).Ch == ':' && p.peek(2, false).Ch == ':' {
		r.Prefix = p.attributeName()
		r.Colon = p.shift(false)
		r.Colon2 = p.shift(false)
	}
	r.Token = p.attributeName()
	if p.rune(false) == '(' {
		r.Case = AttributeValueExpr
		r.Token2 = p.shift(false)
		switch ns := r.Prefix.SrcStr(); ns {
		case "", "gnu", "__gnu__":
			r.ArgumentExpressionList = p.argumentExpressionListOpt(false)
		default:
			r.BalancedTokenSequence = p.balancedTokenSequenceOpt()
		}
		r.Token3 = p.must(')')
	}
	return r
}

func (p *parser) attributeName() Token {
	switch p.rune(false) {
	case rune(IDENTIFIER):
		return p.shift(false)
	default:
		if _, ok := p.isKeyword(p.toks[0].Src()); ok {
			t := p.shift(false)
			t.Ch = rune(IDENTIFIER)
			return t
		}
		t := p.shift(false)
		p.cpp.eh("%v: unexpected %v, expected attribute name", t.Position(), runeName(t.Ch))
		return t
	}
}

// enumSpecifierAttributes parses `enum attribute-specifier-sequence ...`,
// 6.7.3.3: the attributes between "enum" and the rest, which enumSpecifier
// then parses with "enum" put back before it.
func (p *parser) enumSpecifierAttributes() (r *EnumSpecifier) {
	kw := p.shift(false)
	attrs := p.attributeSpecifierListOpt()
	p.toks = append([]Token{kw}, p.toks...)
	if r = p.enumSpecifier(); r != nil {
		r.AttributeSpecifierList = attrs
	}
	return r
}

// compoundLiteralStorage parses a compound literal whose parenthesis holds
// storage-class specifiers before its type name, C23 6.5.3.6:
//
//	( storage-class-specifiers type-name ) braced-initializer
//
// `(static int[]){1, 2}`.  The specifiers go to the PostfixExpression
// postfixExpression makes of it, through p.complitStorage.
func (p *parser) compoundLiteralStorage(checkTypeName bool) ExpressionNode {
	lparen := p.shift(false)
	var scs []*StorageClassSpecifier
	for p.isStorageClassSpecifier(p.rune(false)) {
		scs = append(scs, p.storageClassSpecifier())
	}
	tn := p.typeName()
	rparen := p.must(')')
	if p.rune(false) != '{' {
		t := p.shift(false)
		p.cpp.eh("%v: unexpected %v, expected '{'", t.Position(), runeName(t.Ch))
		return nil
	}
	p.complitStorage = scs
	return p.unaryExpression(lparen, tn, rparen, checkTypeName)
}

// stripDigitSeparators is a constant's text without C23's digit separators
// (6.4.4.1, 6.4.4.2), `1'000` read as `1000`.
func stripDigitSeparators(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' {
			b = append(b, s[i])
		}
	}
	return string(b)
}

// unqualified is t without its const and volatile, typeof_unqual's type
// (6.7.3.6).  The attributes are replaced, not merged: mergeAttr ORs the
// qualifiers in.
func unqualified(t Type) Type {
	if t == nil {
		return t
	}
	if attr := t.Attributes(); attr != nil && (attr.isConst || attr.isVolatile) {
		a2 := *attr
		a2.setIsConst(false)
		a2.setIsVolatile(false)
		return t.setAttr(&a2)
	}
	return t
}

// BalancedTokenSequence is a C23 attribute's argument clause kept as its
// tokens, one a node (6.7.13.2: balanced-token-sequence): the arguments of an
// attribute in a namespace other than gnu's, which need not be expressions,
// `[[vendor::thing(1, [2])]]`.
type BalancedTokenSequence struct {
	Token                 Token
	BalancedTokenSequence *BalancedTokenSequence
}

// String implements fmt.Stringer.
func (n *BalancedTokenSequence) String() string { return PrettyString(n) }

// Position reports the position of the first component of n, if available.
func (n *BalancedTokenSequence) Position() (r token.Position) {
	if n == nil {
		return r
	}
	return n.Token.Position()
}

// balancedTokenSequenceOpt takes the tokens up to the ')' that closes the
// argument clause, the brackets of every kind balanced.
func (p *parser) balancedTokenSequenceOpt() (r *BalancedTokenSequence) {
	var prev *BalancedTokenSequence
	var open []rune
	for {
		ch := p.rune(false)
		switch ch {
		case eof:
			return r
		case '(', '[', '{':
			open = append(open, map[rune]rune{'(': ')', '[': ']', '{': '}'}[ch])
		case ')', ']', '}':
			if len(open) == 0 {
				return r
			}
			if open[len(open)-1] != ch {
				t := p.shift(false)
				p.cpp.eh("%v: unexpected %v in an attribute's arguments", t.Position(), runeName(t.Ch))
				return r
			}
			open = open[:len(open)-1]
		}
		l := &BalancedTokenSequence{Token: p.shift(false)}
		if prev == nil {
			r = l
		} else {
			prev.BalancedTokenSequence = l
		}
		prev = l
	}
}

// stdAttrsBeginDeclaration reports whether the C23 attribute-specifier-sequence
// at the lookahead is followed by a declaration specifier, and so begins a
// declaration and not a statement (6.7, 6.8).
func (p *parser) stdAttrsBeginDeclaration() bool {
	return p.stdAttrAt(0) && p.isDeclarationSpecifier(p.peek(p.stdAttrsEnd(), true).Ch, true)
}

// stdAttrsEnd is the lookahead index of the token after the C23
// attribute-specifier-sequence at the lookahead, 0 when there is none.
func (p *parser) stdAttrsEnd() int {
	i, depth := 0, 0
	for depth > 0 || p.stdAttrAt(i) {
		switch p.peek(i, false).Ch {
		case eof:
			return i
		case '[':
			depth++
		case ']':
			depth--
		}
		i++
	}
	return i
}

// attributedStatement parses a statement after a C23
// attribute-specifier-sequence (6.8): an expression or null statement holds
// the attributes itself, as GNU's `__attribute__((fallthrough));` does; any
// other statement -- a jump, a block, a selection, a labeled one -- holds them
// in its Statement.
func (p *parser) attributedStatement(labelDeclOK bool) *Statement {
	attrs := p.attributeSpecifierListOpt()
	switch ch := p.rune(false); {
	case ch == ';', p.isExpression(ch) && !(ch == rune(IDENTIFIER) && p.peek(1, false).Ch == ':'):
		return &Statement{Case: StatementExpr, ExpressionStatement: &ExpressionStatement{AttributeSpecifierList: attrs, ExpressionList: p.expression(true), Token: p.must(';')}}
	}
	p.labelDeclOK = labelDeclOK
	r := p.statement(false)
	if r != nil {
		r.AttributeSpecifierList = attrs
	}
	return r
}
