package togo

// structvalues.go is struct locals as values, as the backends that keep C's
// memory raw ask it (cfacts.go; doc/HASKELL-IDIOMS.md, item 9): a local
// struct whose members are all scalars, used only by member, copied whole
// to or from memory or another struct, initialized or returned -- never its
// address taken, never passed -- is one binding per member, not bytes in
// the call's frame: `pos_T a = *pp; a.col += dc; *pp = a;` reads the members
// into a_lnum and a_col, adds to a_col and writes them back.  C's copy of a
// struct is its members' copies, so this is exact; what it cannot say -- a
// struct passed by value, its address taken -- keeps the frame.  And a
// function returning a struct of scalars returns its members as values
// (tupleRet).

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// structLocals decides which struct locals are values: c.sval.
func (c *cfacts) structLocals() {
	c.sval = map[*cc.Declarator]bool{}
	for _, fd := range c.defined {
		cands := map[*cc.Declarator]bool{}
		bad := map[*cc.Declarator]bool{}
		var rec func(n, parent, grand cc.Node)
		rec = func(n, parent, grand cc.Node) {
			if n == nil {
				return
			}
			switch x := n.(type) {
			case *cc.Declarator:
				if x.Name() != "__func__" && !x.IsParam() && x.StorageDuration() != cc.Static && scalarStruct(x.Type()) {
					cands[x] = true
				}
			case *cc.PrimaryExpression:
				if x.Case == cc.PrimaryExpressionIdent {
					if d, ok := x.ResolvedTo().(*cc.Declarator); ok && scalarStruct(d.Type()) && !structUse(n, parent, grand) {
						bad[d] = true
					}
				}
			}
			p := outParent(n, parent)
			g := grand
			if p != parent {
				g = parent
			}
			walkChildrenFn(n, func(c cc.Node) { rec(c, p, g) })
		}
		rec(fd.CompoundStatement, nil, nil)
		for d := range cands {
			if !bad[d] {
				c.sval[d] = true
			}
		}
	}
}

// scalarStruct says t is a struct whose members are all named scalars.
func scalarStruct(t cc.Type) bool {
	st, ok := t.(*cc.StructType)
	if !ok || st.NumFields() == 0 {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		fl := st.FieldByIndex(i)
		ft := fl.Type()
		if fl.Name() == "" || fl.IsBitfield() || ft == nil || isAggr(ft) || ft.Kind() == cc.Array || ft.Kind() == cc.Function {
			return false
		}
		if _, ok := scalarKind(ft); !ok && ft.Kind() != cc.Ptr {
			return false
		}
	}
	return true
}

// structUse says the struct n names is used under parent (and grand) as a
// value struct may be: by member, not the member's address; copied whole by
// an assignment or an initializer; returned.
func structUse(n, parent, grand cc.Node) bool {
	switch x := parent.(type) {
	case *cc.PostfixExpression:
		if x.Case != cc.PostfixExpressionSelect {
			return false
		}
		if u, ok := grand.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
			return false
		}
		return true
	case *cc.AssignmentExpression:
		return x.Case == cc.AssignmentExpressionAssign
	case *cc.Initializer:
		return x.Case == cc.InitializerExpr
	case *cc.JumpStatement:
		return x.Case == cc.JumpStatementReturn
	}
	return false
}

// tupleRet says function name returns a struct of scalars as a tuple of its
// members, not through an address its caller gives: a C struct result is
// how C returns several values (phase 181's out-parameters among them).
func (c *cfacts) tupleRet(name string) bool {
	fd := c.defined[name]
	if fd == nil || c.keep[name] {
		return false
	}
	ft, _ := fd.Declarator.Type().(*cc.FunctionType)
	return ft != nil && scalarStruct(ft.Result())
}
