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
		x := n.ExpressionStatement
		if x.ExpressionList == nil {
			e.line(";")
			return
		}
		e.line(e.expr(x.ExpressionList) + ";")
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
		e.line("else")
		e.block(n.Statement2)
	case cc.SelectionStatementSwitch:
		e.line("switch (" + e.expr(n.ExpressionList) + ")")
		e.block(n.Statement)
	default:
		e.fail(n, "selection statement %v", n.Case)
	}
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
		e.line("for (" + e.expr(n.ExpressionList) + "; " + e.expr(n.ExpressionList2) +
			"; " + e.expr(n.ExpressionList3) + ")")
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
		e.line("for (" + decl + " " + e.expr(n.ExpressionList) + "; " + e.expr(n.ExpressionList2) + ")")
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

// initializer prints an initializer.  A braced list is printed on one line when
// it is short and one element per line when it is not, which is the only place
// this printer looks at length rather than at structure.
func (e *emitter) initializer(n *cc.Initializer) string {
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
		s := e.initializer(l.Initializer)
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
