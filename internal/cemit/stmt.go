package cemit

import (
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// BRACES ALWAYS, ONE STATEMENT PER LINE.  `if (x) y();` comes back as an `if`
// with a block, and a block's `{` is on a line of its own under the line that
// controls it -- which is the shape the input already uses where a human wrote
// it, and the shape a text tool can find the end of.

// stmt writes one statement at the current indent.
func (e *emitter) stmt(n *cc.Statement) {
	if n == nil {
		return
	}
	if s, ok := e.stmtMacro(n); ok {
		e.line(s)
		return
	}
	if s, ok := e.fromMacro(n); ok {
		e.line(s + ";")
		return
	}
	switch n.Case {
	case cc.StatementLabeled:
		e.labeled(n.LabeledStatement)
	case cc.StatementCompound:
		e.compound(n.CompoundStatement)
	case cc.StatementExpr:
		// AN ATTRIBUTE STATEMENT IS A STATEMENT.  `__attribute__((fallthrough));`
		// is an expression statement with no expression and an attribute, and
		// dropping the attribute -- 37 of them -- would turn the file's
		// deliberate fallthroughs into ones the compiler warns about.
		x := n.ExpressionStatement
		a := join(e.declSpecs(x.Specifiers()), e.attrs(x.AttributeSpecifierList))
		if x.ExpressionList == nil {
			e.line(a + ";")
			return
		}
		e.line(join(a, e.expr(x.ExpressionList)) + ";")
	case cc.StatementSelection:
		e.selection(n.SelectionStatement)
	case cc.StatementIteration:
		e.iteration(n.IterationStatement)
	case cc.StatementJump:
		e.jump(n.JumpStatement)
	case cc.StatementAsm:
		e.line(strings.TrimSpace(cc.NodeSource(n.AsmStatement)))
	default:
		e.fail(n, "statement %v", n.Case)
	}
}

// block writes a statement as a block, adding the braces when the source did
// not have them.
func (e *emitter) block(n *cc.Statement) {
	if n == nil {
		e.line("{")
		e.line("}")
		return
	}
	if n.Case == cc.StatementCompound {
		e.compound(n.CompoundStatement)
		return
	}
	e.line("{")
	e.indent++
	e.stmt(n)
	e.indent--
	e.line("}")
}

func (e *emitter) compound(n *cc.CompoundStatement) {
	e.line("{")
	e.indent++
	e.blockItems(n.BlockItemList)
	e.indent--
	e.line("}")
}

// compoundInline is a statement expression's block, ({ ... }), printed where an
// expression is expected.
func (e *emitter) compoundInline(n *cc.CompoundStatement) string {
	sub := &emitter{indent: e.indent}
	sub.compound(n)
	if sub.err != nil && e.err == nil {
		e.err = sub.err
	}
	return strings.TrimSpace(sub.b.String())
}

func (e *emitter) blockItems(n *cc.BlockItemList) {
	for l := n; l != nil; l = l.BlockItemList {
		b := l.BlockItem
		switch b.Case {
		case cc.BlockItemDecl:
			// THE FRONT END SYNTHESISES `__func__`.  C23 predeclares it inside
			// every function body and cc/v4 puts the declaration in the tree,
			// so a printer that walked the tree faithfully would write a
			// declaration the source never had.  The name is reserved, and the
			// input has none of its own: measured, 0 in slim-vim.c and 0 in
			// whim-vim.c.
			if funcName(b.Declaration) {
				continue
			}
			for _, s := range e.declLines(b.Declaration) {
				e.line(s)
			}
		case cc.BlockItemStmt:
			e.stmt(b.Statement)
		case cc.BlockItemLabel:
			e.line(strings.TrimSpace(cc.NodeSource(b.LabelDeclaration)))
		case cc.BlockItemFuncDef:
			e.fail(b, "a nested function definition")
		default:
			e.fail(b, "block item %v", b.Case)
		}
	}
}

func (e *emitter) labeled(n *cc.LabeledStatement) {
	switch n.Case {
	case cc.LabeledStatementLabel:
		// A LABEL SITS AT THE BLOCK'S OWN INDENT, one column out, which is
		// where every goto target in the input already is.
		e.outdented(tok(n.Token) + ":")
	case cc.LabeledStatementCaseLabel:
		e.outdented("case " + e.expr(n.ConstantExpression) + ":")
	case cc.LabeledStatementRange:
		e.outdented("case " + e.expr(n.ConstantExpression) + " ... " + e.expr(n.ConstantExpression2) + ":")
	case cc.LabeledStatementDefault:
		e.outdented("default:")
	default:
		e.fail(n, "labeled statement %v", n.Case)
		return
	}
	e.stmt(n.Statement)
}

// outdented writes a line one level out, for labels.
func (e *emitter) outdented(s string) {
	if e.indent > 0 {
		e.indent--
		e.line(s)
		e.indent++
		return
	}
	e.line(s)
}

func (e *emitter) selection(n *cc.SelectionStatement) {
	switch n.Case {
	case cc.SelectionStatementIf:
		e.line("if (" + e.expr(n.ExpressionList) + ")")
		e.block(n.Statement)
	case cc.SelectionStatementIfElse:
		e.line("if (" + e.expr(n.ExpressionList) + ")")
		e.block(n.Statement)
		e.otherwise(n.Statement2)
	case cc.SelectionStatementSwitch:
		e.line("switch (" + e.expr(n.ExpressionList) + ")")
		e.block(n.Statement)
	default:
		e.fail(n, "selection statement %v", n.Case)
	}
}

// clause is one of a for-statement's clauses after the first: a space and the
// clause, or nothing at all when there is no clause.
func clause(s string) string {
	if s == "" {
		return ""
	}
	return " " + s
}

// otherwise writes an `else`.  A CHAIN STAYS A CHAIN: when the else's own
// statement IS an `if` -- not a block holding one -- it is written `else if`
// on the `else`'s line, which is how C spells a ladder and what every reader
// of one looks for (`^[ \t]*else if \(...\)$`).  Bracing it instead would
// nest the twenty links of main()'s long-option ladder twenty levels deep and
// leave no `else if` in the file at all.
func (e *emitter) otherwise(n *cc.Statement) {
	if s := chained(n); s != nil {
		switch s.Case {
		case cc.SelectionStatementIf:
			e.line("else if (" + e.expr(s.ExpressionList) + ")")
			e.block(s.Statement)
			return
		case cc.SelectionStatementIfElse:
			e.line("else if (" + e.expr(s.ExpressionList) + ")")
			e.block(s.Statement)
			e.otherwise(s.Statement2)
			return
		}
	}
	e.line("else")
	e.block(n)
}

// chained returns the `if` an else branch is, or nil when the branch is
// anything else -- a block, a statement, or a `switch`.
func chained(n *cc.Statement) *cc.SelectionStatement {
	if n == nil || n.Case != cc.StatementSelection || n.SelectionStatement == nil {
		return nil
	}
	s := n.SelectionStatement
	if s.Case == cc.SelectionStatementIf || s.Case == cc.SelectionStatementIfElse {
		return s
	}
	return nil
}

func (e *emitter) iteration(n *cc.IterationStatement) {
	switch n.Case {
	case cc.IterationStatementWhile:
		e.line("while (" + e.expr(n.ExpressionList) + ")")
		e.block(n.Statement)
	case cc.IterationStatementDo:
		e.line("do")
		e.block(n.Statement)
		e.line("while (" + e.expr(n.ExpressionList) + ");")
	case cc.IterationStatementFor:
		// A CLAUSE THAT IS NOT THERE TAKES NO SPACE: `for (;;)`, which the
		// input says 135 times, and not `for (; ; )`.
		e.line("for (" + e.expr(n.ExpressionList) + ";" + clause(e.expr(n.ExpressionList2)) +
			";" + clause(e.expr(n.ExpressionList3)) + ")")
		e.block(n.Statement)
	case cc.IterationStatementForDecl:
		// A for-declaration of more than one declarator is the one place the
		// one-declarator-per-line rule cannot apply: `for (int i = a, j = b;`
		// declares both in the loop's own scope, and splitting them would move
		// j out of it.  It is printed as the source had it -- measured, the
		// input has exactly one such loop.
		d := e.declLines(n.Declaration)
		decl := strings.Join(d, " ")
		if len(d) > 1 {
			decl = strings.TrimSpace(cc.NodeSource(n.Declaration))
		}
		e.line("for (" + decl + clause(e.expr(n.ExpressionList)) + ";" +
			clause(e.expr(n.ExpressionList2)) + ")")
		e.block(n.Statement)
	default:
		e.fail(n, "iteration statement %v", n.Case)
	}
}

func (e *emitter) jump(n *cc.JumpStatement) {
	switch n.Case {
	case cc.JumpStatementGoto:
		e.line("goto " + tok(n.Token2) + ";")
	case cc.JumpStatementGotoExpr:
		e.line("goto *" + e.expr(n.ExpressionList) + ";")
	case cc.JumpStatementContinue:
		e.line("continue;")
	case cc.JumpStatementBreak:
		e.line("break;")
	case cc.JumpStatementReturn:
		if n.ExpressionList == nil {
			e.line("return;")
			return
		}
		e.line("return " + e.expr(n.ExpressionList) + ";")
	default:
		e.fail(n, "jump statement %v", n.Case)
	}
}

// initializer prints a declaration's initializer: its outermost braced list one
// element per line, everything inside it on one.
func (e *emitter) initializer(n *cc.Initializer) string {
	switch n.Case {
	case cc.InitializerExpr:
		return e.expr(n.AssignmentExpression)
	case cc.InitializerInitList:
		return e.braced(n.InitializerList)
	}
	e.fail(n, "initializer %v", n.Case)
	return ""
}

// braced prints a declaration's outermost braced initializer, ONE ELEMENT PER
// LINE.  That is what makes a table a table: the input writes one row of
// cmdnames[], one row of options[], one name of main_errors[] and one number of
// included_patches[] per line, every tool that reads those tables reads them BY
// THE LINE -- `^    \[CMD_\w+\] = \{`, `^static char \*\(main_errors\[\]\) =\n\{`
// -- and a table on one line is a table no text tool can see into.
//
// AND THE BREAK IS AT THE TOP ONLY.  A row of options[] ends in a braced pair of
// defaults, so exploding all the way down would put the row's own fields one per
// line too and there would be no such thing as the line a row is on -- which is
// the line `{"spell",` that every reader of that table looks for.  A row is one
// line however deep it goes.
// The brace is on its own line, under the `=`, which is the shape the input has
// and the shape the checks read (`^static char \*\(main_errors\[\]\) =\n\{\n`).
func (e *emitter) braced(n *cc.InitializerList) string {
	var b strings.Builder
	b.WriteString("\n" + e.pad() + "{\n")
	e.indent++
	for l := n; l != nil; l = l.InitializerList {
		s := e.inlineInitializer(l.Initializer)
		if l.Designation != nil {
			s = e.designation(l.Designation) + " = " + s
		}
		b.WriteString(e.pad() + s + ",\n")
	}
	e.indent--
	b.WriteString(e.pad() + "}")
	return b.String()
}

// inlineInitializer prints an initializer without ever breaking a line: it is
// what a row of a table is printed with.
func (e *emitter) inlineInitializer(n *cc.Initializer) string {
	switch n.Case {
	case cc.InitializerExpr:
		return e.expr(n.AssignmentExpression)
	case cc.InitializerInitList:
		return "{" + e.initializerList(n.InitializerList) + "}"
	}
	e.fail(n, "initializer %v", n.Case)
	return ""
}

func (e *emitter) initializerList(n *cc.InitializerList) string {
	var parts []string
	for l := n; l != nil; l = l.InitializerList {
		s := e.inlineInitializer(l.Initializer)
		if l.Designation != nil {
			s = e.designation(l.Designation) + " = " + s
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func (e *emitter) designation(n *cc.Designation) string {
	var b strings.Builder
	for l := n.DesignatorList; l != nil; l = l.DesignatorList {
		d := l.Designator
		switch d.Case {
		case cc.DesignatorIndex:
			b.WriteString("[" + e.expr(d.ConstantExpression) + "]")
		case cc.DesignatorIndex2:
			b.WriteString("[" + e.expr(d.ConstantExpression) + " ... " + e.expr(d.ConstantExpression2) + "]")
		case cc.DesignatorField:
			b.WriteString("." + tok(d.Token2))
		case cc.DesignatorField2:
			b.WriteString(tok(d.Token) + ":")
		default:
			e.fail(d, "designator %v", d.Case)
		}
	}
	return b.String()
}

// funcName reports whether a declaration is the predeclared `__func__`.
func funcName(n *cc.Declaration) bool {
	if n == nil || n.Case != cc.DeclarationDecl || n.InitDeclaratorList == nil {
		return false
	}
	l := n.InitDeclaratorList
	if l.InitDeclaratorList != nil {
		return false
	}
	d := l.InitDeclarator.Declarator
	if d == nil || d.DirectDeclarator == nil {
		return false
	}
	dd := d.DirectDeclarator
	for dd.Case != cc.DirectDeclaratorIdent && dd.DirectDeclarator != nil {
		dd = dd.DirectDeclarator
	}
	return dd.Case == cc.DirectDeclaratorIdent && dd.Token.SrcStr() == "__func__"
}
