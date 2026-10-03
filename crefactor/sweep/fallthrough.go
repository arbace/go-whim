package sweep

import "github.com/arbace/go-whim/crefactor/cc"

// ---- orphaned fallthroughs -------------------------------------------------

// A FALLTHROUGH ATTRIBUTE THAT PRECEDES NO CASE LABEL GOES.  `[[fallthrough]];`
// (or GNU's `__attribute__((fallthrough));`) says that control falls from one
// case into the next on purpose; it attaches to nothing but the label after it.
// A cut that deletes the cases after one -- the last of a switch's cases, or
// the run a feature's drop takes out -- leaves it before a `}` or before a
// statement, which gcc reports ("attribute 'fallthrough' not preceding a case
// label or default label") and which is a statement nothing needs: like a local
// nothing reads, it goes in the closure, whatever cut orphaned it.
//
// WHERE CONTROL GOES NEXT is gcc's test, and this one's: the next item of the
// block, or, at the block's end, wherever control goes after the statement the
// block is the body of -- an if's branch, a do-while's body, a label's
// statement, a nested block -- and a case or default label there, or an
// ordinary label on one, keeps it.  The end of a switch's body, of a while's or
// a for's, and of the function reach no case label.  Only live functions are
// read: a dead one goes whole.
func (a *analysis) orphanedFallthroughs(fd *cc.FunctionDefinition) {
	a.ftCompound(fd.CompoundStatement, false)
}

// ftCompound walks a block whose end leads to a case label when next.
func (a *analysis) ftCompound(n *cc.CompoundStatement, next bool) {
	if n == nil {
		return
	}
	for l := n.BlockItemList; l != nil; l = l.BlockItemList {
		after := next
		if l.BlockItemList != nil {
			after = ftReachesCase(l.BlockItemList.BlockItem)
		}
		bi := l.BlockItem
		if bi == nil {
			continue
		}
		switch {
		case bi.Statement != nil:
			a.ftStatement(bi.Statement, after)
		case bi.CompoundStatement != nil:
			a.ftCompound(bi.CompoundStatement, false)
		}
	}
}

// ftStatement walks a statement whose end leads to a case label when next.
func (a *analysis) ftStatement(n *cc.Statement, next bool) {
	if n == nil {
		return
	}
	switch n.Case {
	case cc.StatementLabeled:
		a.ftStatement(n.LabeledStatement.Statement, next)
	case cc.StatementCompound:
		a.ftCompound(n.CompoundStatement, next)
	case cc.StatementExpr:
		if isFallthrough(n.ExpressionStatement) && !next {
			lo, hi := a.span(n)
			// GNU's are the specifiers, which the walk does not visit.
			if ds := n.ExpressionStatement.Specifiers(); ds != nil {
				if dlo, dhi := a.span(ds); dhi > dlo && dlo < lo {
					lo = dlo
				}
			}
			if hi > lo {
				a.orphans = append(a.orphans, [2]int{lo, hi})
			}
		}
	case cc.StatementSelection:
		x := n.SelectionStatement
		if x.Case == cc.SelectionStatementSwitch {
			a.ftStatement(x.Statement, false)
			return
		}
		a.ftStatement(x.Statement, next)
		a.ftStatement(x.Statement2, next)
	case cc.StatementIteration:
		x := n.IterationStatement
		a.ftStatement(x.Statement, x.Case == cc.IterationStatementDo && next)
	}
}

// ftReachesCase says the block item is a case or default label, or an
// ordinary label on one.
func ftReachesCase(bi *cc.BlockItem) bool {
	if bi == nil || bi.Statement == nil || bi.Statement.Case != cc.StatementLabeled {
		return false
	}
	switch x := bi.Statement.LabeledStatement; x.Case {
	case cc.LabeledStatementCaseLabel, cc.LabeledStatementRange, cc.LabeledStatementDefault:
		return true
	default:
		s := x.Statement
		for s != nil && s.Case == cc.StatementLabeled {
			if s.LabeledStatement.Case != cc.LabeledStatementLabel {
				return true
			}
			s = s.LabeledStatement.Statement
		}
		return false
	}
}

// isFallthrough says the statement is a null statement whose attributes are
// all fallthrough: C23's, held in the statement, or GNU's, which the parser
// holds as the specifiers of a declaration that declares nothing.
func isFallthrough(x *cc.ExpressionStatement) bool {
	if x == nil || x.ExpressionList != nil {
		return false
	}
	var lists []*cc.AttributeSpecifierList
	if x.AttributeSpecifierList != nil {
		lists = append(lists, x.AttributeSpecifierList)
	}
	for ds := x.Specifiers(); ds != nil; ds = ds.DeclarationSpecifiers {
		if ds.Case != cc.DeclarationSpecifiersAttr {
			return false
		}
		lists = append(lists, ds.AttributeSpecifierList)
	}
	seen := false
	for _, list := range lists {
		for l := list; l != nil; l = l.AttributeSpecifierList {
			if l.AttributeSpecifier == nil {
				return false
			}
			for v := l.AttributeSpecifier.AttributeValueList; v != nil; v = v.AttributeValueList {
				if v.AttributeValue == nil {
					continue
				}
				if nm := v.AttributeValue.Token.SrcStr(); nm != "fallthrough" && nm != "__fallthrough__" {
					return false
				}
				seen = true
			}
		}
	}
	return seen
}
