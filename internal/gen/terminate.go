package gen

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// goTerminates says the Go function body src (the statements between its
// braces) ends in a terminating statement as the Go spec defines one, so that
// a function with a result needs nothing after it.  The generator used to ask
// whether the last LINE began with return, goto or panic, and so appended
// panic("not reached") after every if/else whose branches both return --
// code vet then reports as unreachable.
func goTerminates(src string) bool {
	f, err := parser.ParseFile(token.NewFileSet(), "", "package p\nfunc _() {\n"+src+"\n}\n", 0)
	if err != nil {
		return false // not ours to judge: keep the old answer's safety
	}
	body := f.Decls[0].(*ast.FuncDecl).Body
	return terminatingList(body.List)
}

func terminatingList(l []ast.Stmt) bool {
	for i := len(l) - 1; i >= 0; i-- {
		if _, empty := l[i].(*ast.EmptyStmt); empty {
			continue
		}
		return terminating(l[i])
	}
	return false
}

// terminating is the Go spec's "Terminating statements", less select.
func terminating(s ast.Stmt) bool {
	switch x := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return x.Tok == token.GOTO || x.Tok == token.FALLTHROUGH
	case *ast.ExprStmt:
		c, ok := x.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := c.Fun.(*ast.Ident)
		return ok && id.Name == "panic"
	case *ast.BlockStmt:
		return terminatingList(x.List)
	case *ast.IfStmt:
		return x.Else != nil && terminatingList(x.Body.List) && terminating(x.Else)
	case *ast.LabeledStmt:
		return terminating(x.Stmt)
	case *ast.ForStmt:
		return x.Cond == nil && !breaks(x.Body, "")
	case *ast.SwitchStmt:
		return switchTerminates(x.Body, "")
	}
	return false
}

func switchTerminates(body *ast.BlockStmt, label string) bool {
	def := false
	for _, c := range body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil {
			def = true
		}
		if !terminatingList(cc.Body) || breaks(&ast.BlockStmt{List: cc.Body}, label) {
			return false
		}
	}
	return def
}

// breaks says a break refers to the statement whose body n is: an unlabeled
// one not inside a nested for, switch or select, or one naming label.
func breaks(n ast.Node, label string) bool {
	found := false
	var visit func(n ast.Node, nested bool)
	visit = func(n ast.Node, nested bool) {
		ast.Inspect(n, func(m ast.Node) bool {
			if found || m == nil {
				return false
			}
			switch x := m.(type) {
			case *ast.BranchStmt:
				if x.Tok == token.BREAK && ((x.Label == nil && !nested) || (x.Label != nil && x.Label.Name == label)) {
					found = true
				}
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				if m != n {
					visit(m, true)
					return false
				}
			case *ast.FuncLit:
				return false
			}
			return true
		})
	}
	visit(n, false)
	return found
}
