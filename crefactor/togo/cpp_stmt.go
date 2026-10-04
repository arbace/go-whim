package togo

// cpp_stmt.go prints the C's statements and initial values as C++: the
// canonical printer's shape, a declaration a jump crosses split into a
// declaration and an assignment (C++ forbids the jump), a block-scope
// static the editor's field, and the values of a braced list converted as
// C converts them (C++ refuses a narrowing there).

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/cemit"
)

// crossed is the set of declarations in body that a jump crosses into their scope: a case of a switch around their block, or
// a label after them in it that a goto from outside what follows them
// reaches.
func (c *cppgen) crossed(body *cc.CompoundStatement) map[*cc.Declaration]bool {
	out := map[*cc.Declaration]bool{}
	// the gotos of each label, the function's
	gotos := map[string][]cc.Node{}
	var findGotos func(n cc.Node)
	findGotos = func(n cc.Node) {
		if n == nil {
			return
		}
		if j, ok := n.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementGoto {
			gotos[j.Token2.SrcStr()] = append(gotos[j.Token2.SrcStr()], j)
		}
		walkChildrenFn(n, findGotos)
	}
	findGotos(body)
	// crosses reports whether the items after a declaration hold a case of
	// an enclosing switch, or a label a goto outside them reaches
	crosses := func(later []*cc.BlockItem) bool {
		inside := map[cc.Node]bool{}
		var labels []string
		cases := false
		var r func(n cc.Node, nested bool)
		r = func(n cc.Node, nested bool) {
			if n == nil {
				return
			}
			switch x := n.(type) {
			case *cc.JumpStatement:
				inside[x] = true
			case *cc.LabeledStatement:
				if x.Case == cc.LabeledStatementLabel {
					labels = append(labels, x.Token.SrcStr())
				} else if !nested {
					cases = true
				}
			case *cc.SelectionStatement:
				if x.Case == cc.SelectionStatementSwitch {
					walkChildrenFn(n, func(ch cc.Node) { r(ch, true) })
					return
				}
			}
			walkChildrenFn(n, func(ch cc.Node) { r(ch, nested) })
		}
		for _, it := range later {
			r(it, false)
		}
		if cases {
			return true
		}
		for _, l := range labels {
			for _, g := range gotos[l] {
				if !inside[g] {
					return true
				}
			}
		}
		return false
	}
	var block func(l *cc.BlockItemList)
	var rec func(n cc.Node)
	block = func(l *cc.BlockItemList) {
		var items []*cc.BlockItem
		for ; l != nil; l = l.BlockItemList {
			items = append(items, l.BlockItem)
		}
		for i, it := range items {
			if it.Case == cc.BlockItemDecl && crosses(items[i+1:]) {
				out[it.Declaration] = true
			}
			rec(it)
		}
	}
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if cs, ok := n.(*cc.CompoundStatement); ok {
			block(cs.BlockItemList)
			return
		}
		walkChildrenFn(n, rec)
	}
	block(body.BlockItemList)
	return out
}

// flushPre writes the declarations an expression just printed needs
// before its statement.
func (c *cppgen) flushPre() {
	for _, p := range c.pre {
		c.line(p)
	}
	c.pre = nil
}

func (c *cppgen) stmt(n *cc.Statement) {
	if n == nil {
		return
	}
	defer func() {
		if len(c.pre) > 0 {
			c.fail(n, "a compound literal where no declaration can go before it")
			c.pre = nil
		}
	}()
	if a := c.attrs(n.AttributeSpecifierList); a != "" {
		c.line(a)
	}
	switch n.Case {
	case cc.StatementLabeled:
		c.labeled(n.LabeledStatement)
	case cc.StatementCompound:
		c.compound(n.CompoundStatement)
	case cc.StatementExpr:
		x := n.ExpressionStatement
		a := c.attrs(x.AttributeSpecifierList)
		if x.ExpressionList == nil {
			c.line(a + ";")
			return
		}
		if as, ok := pass(x.ExpressionList).(*cc.AssignmentExpression); ok && a == "" && as.Case != cc.AssignmentExpressionCond &&
			cppOrderMatters(as.UnaryExpression, as.AssignmentExpression) {
			// C (gcc's) evaluates the left of an assignment, its side effect
			// with it, before the right, where C++17 sequences the right
			// first: the left's address is taken first
			c.nOrdered++
			c.line("{")
			c.indent++
			c.line(c.typeDecl1(as.UnaryExpression.Type(), "*lhs__") + " = &" + c.paren(as.UnaryExpression) + ";")
			r := c.expr(as.AssignmentExpression)
			if as.Token.SrcStr() == "=" {
				r = c.conv(as.AssignmentExpression, as.UnaryExpression.Type())
			}
			c.line("*lhs__ " + as.Token.SrcStr() + " " + r + ";")
			c.indent--
			c.line("}")
			return
		}
		e := c.expr(x.ExpressionList)
		c.flushPre()
		c.line(cppJoin(a, e) + ";")
	case cc.StatementSelection:
		c.selection(n.SelectionStatement)
	case cc.StatementIteration:
		c.iteration(n.IterationStatement)
	case cc.StatementJump:
		c.jump(n.JumpStatement)
	default:
		c.fail(n, "statement %v", n.Case)
	}
}

func (c *cppgen) block(n *cc.Statement) {
	if n == nil {
		c.line("{")
		c.line("}")
		return
	}
	if n.Case == cc.StatementCompound {
		if a := c.attrs(n.AttributeSpecifierList); a != "" {
			c.line(a)
		}
		c.compound(n.CompoundStatement)
		return
	}
	c.line("{")
	c.indent++
	c.stmt(n)
	c.indent--
	c.line("}")
}

func (c *cppgen) compound(n *cc.CompoundStatement) {
	c.line("{")
	c.indent++
	c.blockItems(n.BlockItemList)
	c.indent--
	c.line("}")
}

func (c *cppgen) blockItems(n *cc.BlockItemList) {
	for l := n; l != nil; l = l.BlockItemList {
		b := l.BlockItem
		switch b.Case {
		case cc.BlockItemDecl:
			if cemit.FuncName(b.Declaration) {
				continue
			}
			lines := c.declLines(b.Declaration)
			c.flushPre()
			for _, s := range lines {
				c.line(s)
			}
		case cc.BlockItemStmt:
			c.stmt(b.Statement)
		default:
			c.fail(b, "block item %v", b.Case)
		}
	}
}

// declLines prints a block's declaration: a declarator a line, a static
// the editor's field (nothing here), one a jump crosses split in two.
func (c *cppgen) declLines(n *cc.Declaration) []string {
	switch n.Case {
	case cc.DeclarationAssert:
		return []string{c.staticAssert(n.StaticAssertDeclaration)}
	case cc.DeclarationDecl:
	default:
		c.fail(n, "declaration %v", n.Case)
		return nil
	}
	var out []string
	if n.InitDeclaratorList == nil {
		// a struct or an enumeration defined in a block
		out = append(out, c.localTypeDefs(n.DeclarationSpecifiers)...)
		return out
	}
	typedef, static := false, false
	for l := n.DeclarationSpecifiers; l != nil; l = l.DeclarationSpecifiers {
		if l.Case == cc.DeclarationSpecifiersStorage {
			switch l.StorageClassSpecifier.Token.SrcStr() {
			case "typedef":
				typedef = true
			case "static":
				static = true
			}
		}
	}
	out = append(out, c.localTypeDefs(n.DeclarationSpecifiers)...)
	specs := c.declSpecs(n.DeclarationSpecifiers, true)
	split := c.split[n]
	for l := n.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		d := l.InitDeclarator
		if _, ok := c.field[d.Declarator]; ok {
			continue // the editor's field
		}
		if _, ok := c.hoist[d.Declarator]; ok {
			continue // declared at the function's top
		}
		ln := int64(-1)
		if a, ok := d.Declarator.Type().(*cc.ArrayType); ok {
			ln = a.Len()
		}
		s := cppJoin(specs, c.declarator(d.Declarator, "", ln))
		switch {
		case typedef:
			out = append(out, "typedef "+s+";")
		case d.Initializer == nil && c.maybeRead(d.Declarator):
			// a scalar a path may read before C stores to it: zero, where
			// the C's is whatever the stack held (and g++ warns); stored
			// after the declaration where a jump crosses it
			c.nZeroed++
			if split {
				out = append(out, s+";", cppName(d.Declarator.Name())+" = {};")
			} else {
				out = append(out, s+"{};")
			}
		case d.Initializer == nil:
			out = append(out, s+";")
		case static:
			out = append(out, "static "+s+" "+c.assign(d.Initializer, d.Declarator.Type())+";")
		case split && d.Initializer.Case == cc.InitializerExpr:
			c.nSplit++
			out = append(out, dropConst(s)+";")
			out = append(out, cppName(d.Declarator.Name())+" = "+c.conv(d.Initializer.AssignmentExpression, d.Declarator.Type())+";")
		case split:
			c.fail(d, "a braced initial value a jump crosses")
			out = append(out, s+" "+c.assign(d.Initializer, d.Declarator.Type())+";")
		default:
			out = append(out, s+" "+c.assign(d.Initializer, d.Declarator.Type())+";")
		}
	}
	return out
}

// dropConst is a declaration without its const, for one split from its
// value.
func dropConst(s string) string {
	f := strings.Fields(s)
	var out []string
	for _, w := range f {
		if w != "const" {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// localTypeDefs writes the constants of an enumeration a block defines;
// a struct defined in a block is a local class, written in place.
func (c *cppgen) localTypeDefs(n *cc.DeclarationSpecifiers) []string {
	var out []string
	for l := n; l != nil; l = l.DeclarationSpecifiers {
		if l.Case != cc.DeclarationSpecifiersTypeSpec {
			continue
		}
		t := l.TypeSpecifier
		switch t.Case {
		case cc.TypeSpecifierEnum:
			e := t.EnumSpecifier
			if e.Case != cc.EnumSpecifierDef {
				continue
			}
			for el := e.EnumeratorList; el != nil; el = el.EnumeratorList {
				en := el.Enumerator
				v := ""
				if en.Case == cc.EnumeratorExpr {
					v = c.expr(en.ConstantExpression)
				} else {
					v = cppIntLit(en.Value(), en.Type())
				}
				out = append(out, "constexpr "+c.scalar(en.Type())+" "+cppName(en.Token.SrcStr())+" = "+v+";")
			}
		case cc.TypeSpecifierStructOrUnion:
			s := t.StructOrUnionSpecifier
			if s.Case == cc.StructOrUnionSpecifierDef && s.Token.SrcStr() != "" && !c.hoisted[s] {
				out = append(out, c.structOrUnion(s)+";")
			}
		}
	}
	return out
}

func (c *cppgen) labeled(n *cc.LabeledStatement) {
	switch n.Case {
	case cc.LabeledStatementLabel:
		if c.gotos[n.Token.SrcStr()] {
			// a label no goto names is no label: C++ warns of it
			c.outdented(n.Token.SrcStr() + ":")
		}
	case cc.LabeledStatementCaseLabel:
		c.outdented("case " + c.expr(n.ConstantExpression) + ":")
	case cc.LabeledStatementRange:
		c.outdented("case " + c.expr(n.ConstantExpression) + " ... " + c.expr(n.ConstantExpression2) + ":")
	case cc.LabeledStatementDefault:
		c.outdented("default:")
	default:
		c.fail(n, "labeled statement %v", n.Case)
		return
	}
	c.stmt(n.Statement)
}

func (c *cppgen) outdented(s string) {
	if c.indent > 0 {
		c.indent--
		c.line(s)
		c.indent++
		return
	}
	c.line(s)
}

func (c *cppgen) selection(n *cc.SelectionStatement) {
	switch n.Case {
	case cc.SelectionStatementIf:
		c.line("if (" + c.expr(n.ExpressionList) + ")")
		c.block(n.Statement)
	case cc.SelectionStatementIfElse:
		c.line("if (" + c.expr(n.ExpressionList) + ")")
		c.block(n.Statement)
		c.otherwise(n.Statement2)
	case cc.SelectionStatementSwitch:
		c.line("switch (" + c.expr(n.ExpressionList) + ")")
		c.block(n.Statement)
	default:
		c.fail(n, "selection statement %v", n.Case)
	}
}

func (c *cppgen) otherwise(n *cc.Statement) {
	if s := chainedIf(n); s != nil {
		c.line("else if (" + c.expr(s.ExpressionList) + ")")
		c.block(s.Statement)
		if s.Case == cc.SelectionStatementIfElse {
			c.otherwise(s.Statement2)
		}
		return
	}
	c.line("else")
	c.block(n)
}

func chainedIf(n *cc.Statement) *cc.SelectionStatement {
	if n == nil || n.Case != cc.StatementSelection || n.SelectionStatement == nil || n.AttributeSpecifierList != nil {
		return nil
	}
	s := n.SelectionStatement
	if s.Case == cc.SelectionStatementIf || s.Case == cc.SelectionStatementIfElse {
		return s
	}
	return nil
}

func clauseS(s string) string {
	if s == "" {
		return ""
	}
	return " " + s
}

func (c *cppgen) iteration(n *cc.IterationStatement) {
	switch n.Case {
	case cc.IterationStatementWhile:
		c.line("while (" + c.expr(n.ExpressionList) + ")")
		c.block(n.Statement)
	case cc.IterationStatementDo:
		c.line("do")
		c.block(n.Statement)
		c.line("while (" + c.expr(n.ExpressionList) + ");")
	case cc.IterationStatementFor:
		c.line("for (" + c.expr(n.ExpressionList) + ";" + clauseS(c.expr(n.ExpressionList2)) +
			";" + clauseS(c.expr(n.ExpressionList3)) + ")")
		c.block(n.Statement)
	case cc.IterationStatementForDecl:
		d := c.declLines(n.Declaration)
		decl := strings.Join(d, " ")
		if len(d) > 1 {
			// int i = a, j = b: one declaration, its specifiers once
			decl = c.forDecl(n.Declaration)
		}
		c.line("for (" + decl + clauseS(c.expr(n.ExpressionList)) + ";" + clauseS(c.expr(n.ExpressionList2)) + ")")
		c.block(n.Statement)
	default:
		c.fail(n, "iteration statement %v", n.Case)
	}
}

// forDecl is a for-declaration of more than one declarator, on one line.
func (c *cppgen) forDecl(n *cc.Declaration) string {
	specs := c.declSpecs(n.DeclarationSpecifiers, false)
	var ds []string
	for l := n.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		d := l.InitDeclarator
		s := c.declarator(d.Declarator, "", -1)
		if d.Initializer != nil {
			s += " " + c.assign(d.Initializer, d.Declarator.Type())
		}
		ds = append(ds, s)
	}
	return specs + " " + strings.Join(ds, ", ") + ";"
}

func (c *cppgen) jump(n *cc.JumpStatement) {
	switch n.Case {
	case cc.JumpStatementGoto:
		c.line("goto " + n.Token2.SrcStr() + ";")
	case cc.JumpStatementContinue:
		c.line("continue;")
	case cc.JumpStatementBreak:
		c.line("break;")
	case cc.JumpStatementReturn:
		if n.ExpressionList == nil {
			c.line("return;")
			return
		}
		c.line("return " + c.conv(n.ExpressionList, c.result) + ";")
	default:
		c.fail(n, "jump statement %v", n.Case)
	}
}

// assign is `= value` for an object of type t.
func (c *cppgen) assign(n *cc.Initializer, t cc.Type) string {
	s := c.initializer(n, t)
	if strings.HasPrefix(s, "\n") {
		return "=" + s
	}
	return "= " + s
}

// initializer prints an initial value: the outermost braced list an
// element a line.
func (c *cppgen) initializer(n *cc.Initializer, t cc.Type) string {
	switch n.Case {
	case cc.InitializerExpr:
		return c.conv(n.AssignmentExpression, t)
	case cc.InitializerInitList:
		if l := n.InitializerList; l != nil && l.InitializerList == nil && l.Designation == nil && t != nil &&
			(t.Kind() == cc.Struct || t.Kind() == cc.Union || t.Kind() == cc.Array) && l.Initializer.Case == cc.InitializerExpr && c.isNull(l.Initializer.AssignmentExpression) {
			// {0}: C's every member zero, C++'s {}
			return "{}"
		}
		var b strings.Builder
		b.WriteString("\n" + c.pad() + "{\n")
		c.indent++
		idx := int64(0)
		for l := n.InitializerList; l != nil; l = l.InitializerList {
			s := c.element(l.Initializer)
			if l.Designation != nil {
				c.nextIndex = idx
				if d := c.designation(l.Designation); d != "" {
					s = d + " = " + s
				}
			}
			idx++
			b.WriteString(c.pad() + s + ",\n")
		}
		c.indent--
		b.WriteString(c.pad() + "}")
		return b.String()
	}
	c.fail(n, "initializer %v", n.Case)
	return ""
}

// element is one element of a braced list, on one line, converted to the
// type of what it initializes.
func (c *cppgen) element(n *cc.Initializer) string {
	switch n.Case {
	case cc.InitializerExpr:
		return c.convElem(n.AssignmentExpression, n.Type())
	case cc.InitializerInitList:
		return "{" + c.initList(n.InitializerList) + "}"
	}
	c.fail(n, "initializer %v", n.Case)
	return ""
}

func (c *cppgen) initList(n *cc.InitializerList) string {
	var parts []string
	idx := int64(0)
	for l := n; l != nil; l = l.InitializerList {
		s := c.element(l.Initializer)
		if l.Designation != nil {
			c.nextIndex = idx
			if d := c.designation(l.Designation); d != "" {
				s = d + " = " + s
			}
		}
		idx++
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func (c *cppgen) designation(n *cc.Designation) string {
	var b strings.Builder
	for l := n.DesignatorList; l != nil; l = l.DesignatorList {
		d := l.Designator
		switch d.Case {
		case cc.DesignatorField:
			b.WriteString("." + cppName(d.Token2.SrcStr()))
		case cc.DesignatorIndex:
			// [k] = v where k is the next index anyway: C++ says it without
			if v, ok := intValue(d.ConstantExpression.Value()); !ok || v != c.nextIndex {
				c.fail(d, "an array designator out of order")
			}
		default:
			c.fail(d, "designator %v", d.Case)
		}
	}
	return b.String()
}

// convElem is conv in a braced list, where C++ refuses a narrowing
// conversion C makes: an integer of another type whose value is not a
// constant that fits is cast.
func (c *cppgen) convElem(n cc.ExpressionNode, t cc.Type) string {
	if t == nil {
		return c.expr(n)
	}
	switch t.Kind() {
	case cc.Ptr:
		return c.conv(n, t)
	case cc.Struct, cc.Union, cc.Array:
		return c.expr(n)
	}
	tk, ok1 := scalarKind(t)
	fk, ok2 := scalarKind(n.Type())
	if !ok1 || !ok2 || tk == fk {
		return c.expr(n)
	}
	if v, ok := intValue(n.Value()); ok && cppFits(v, n.Value(), tk) {
		return c.expr(n)
	}
	if !tk.boolean && !fk.boolean && tk.size > fk.size && (tk.signed || !fk.signed) {
		return c.expr(n) // widening
	}
	c.nCasts++
	return "(" + c.typeStr(t) + ")" + c.paren(n)
}

// fits reports whether the constant v is a value of kind k.
func cppFits(v int64, raw cc.Value, k jk) bool {
	if _, u := raw.(cc.UInt64Value); u && v < 0 {
		return false // past int64
	}
	if k.boolean {
		return v == 0 || v == 1
	}
	if k.size >= 8 {
		return k.signed || v >= 0
	}
	bits := uint(k.size * 8)
	if k.signed {
		lo, hi := -(int64(1) << (bits - 1)), int64(1)<<(bits-1)-1
		return v >= lo && v <= hi
	}
	return v >= 0 && v < int64(1)<<bits
}

// maybeRead reports whether a scalar local declared with no value may be
// read before a store to it, by Java's definite assignment (java_da.go):
// no bolder than JLS 16's, so at least every local g++'s flow warns of.
func (c *cppgen) maybeRead(d *cc.Declarator) bool {
	if c.inLoop(d) {
		return false // hoisted (hoistLoopLocals)
	}
	return c.mayReadFirst(d)
}

// inLoop reports whether d is declared in a loop's body.
func (c *cppgen) inLoop(d *cc.Declarator) bool {
	c.parents()
	for p := c.par[d]; p != nil; p = c.par[p] {
		if _, ok := p.(*cc.IterationStatement); ok {
			return true
		}
	}
	return false
}

func (c *cppgen) parents() {
	if c.par == nil {
		c.par = map[cc.Node]cc.Node{}
		var rec func(cc.Node)
		rec = func(n cc.Node) {
			walkChildrenFn(n, func(ch cc.Node) {
				c.par[ch] = n
				rec(ch)
			})
		}
		rec(c.fn.CompoundStatement)
	}
}

// hoistLoopLocals finds the scalars declared with no value in a loop's
// body that a path may read before a store: the C's keeps, at -O0, what
// the last iteration left in its stack slot, so that zeroing it each time
// round would not be the C's -- and g++ warns of it unset.  Each is
// declared once, zeroed, at the top of the function: what the last
// iteration left, and zero the first time.
func (c *cppgen) hoistLoopLocals() []string {
	c.hoist = map[*cc.Declarator]string{}
	c.parents()
	names := map[string]int{}
	var all []*cc.Declarator
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if d, ok := n.(*cc.Declarator); ok && d.Name() != "" {
			names[d.Name()]++
			all = append(all, d)
		}
		walkChildrenFn(n, rec)
	}
	rec(c.fn.CompoundStatement)
	var out []string
	for _, d := range all {
		id, ok := c.par[d].(*cc.InitDeclarator)
		if !ok || id.Initializer != nil || d.IsStatic() || d.IsTypename() || !c.inLoop(d) || !c.mayReadFirst(d) {
			continue
		}
		n := cppName(d.Name())
		for names[n] > 1 || c.isMember(n) {
			n += "_"
			names[n]++
		}
		c.hoist[d] = n
		c.nZeroed++
		out = append(out, c.typeDecl1(d.Type(), n)+"{};")
	}
	return out
}

// mayReadFirst reports whether a path from d's declaration reads it before
// a store, by Java's definite assignment.
func (c *cppgen) mayReadFirst(d *cc.Declarator) bool {
	t := d.Type()
	if t == nil || (t.Kind() != cc.Ptr && t.Kind() != cc.Bool && t.Kind() != cc.Enum && !cc.IsIntegerType(t)) {
		return false
	}
	c.parents()
	var n cc.Node = d
	for n != nil {
		if _, ok := n.(*cc.BlockItem); ok {
			break
		}
		n = c.par[n]
	}
	list, ok := c.par[n].(*cc.BlockItemList)
	if !ok {
		return true
	}
	w := &jda{d: d}
	u := true
	for l := list.BlockItemList; l != nil && !w.fail; l = l.BlockItemList {
		u = w.item(l.BlockItem, u)
	}
	return w.fail
}
