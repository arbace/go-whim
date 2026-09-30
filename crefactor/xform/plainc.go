package xform

// plainc.go is C written plainly where a translation spelled it as the
// preprocessor left it (doc/JAVA-IDIOMS.md, item 8): three steps, each
// knowing no code base.
//
//   - Identity: a function whose body returns its one parameter, cast to
//     its result or not -- gettext's `_()` once there was no gettext --
//     is not called: each call is its argument, cast where the types
//     differ.  The function stays for whoever names it without calling.
//   - AsciiClass: `(unsigned)c - 'A' < 26`, C's ASCII class test with one
//     comparison, which a language with no unsigned int says as
//     Integer.compareUnsigned(c - 'A', 26) < 0, is a call of a function
//     named for it again (ascii_isupper, _islower, _isdigit), defined at
//     the top.
//   - ConstBranch: an if whose condition is a constant is the branch it
//     takes, when neither branch holds a label.

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// coreAST is the core's text, parsed and typed, and where the core ends.
func coreAST(p edit.Ph, core Core, text []byte) ([]byte, int, *cc.AST, error) {
	end := len(text)
	if core != nil {
		if e := core(text); e >= 0 {
			end = e
		}
	}
	src := append([]byte(nil), text[:end]...)
	ast, err := translate(append([]byte(nil), src...))
	if err != nil {
		return nil, 0, nil, p.Die("the core does not type-check: %v", err)
	}
	return src, end, ast, nil
}

// funcs calls fn with each function the core defines.
func funcs(ast *cc.AST, fn func(*cc.FunctionDefinition)) {
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case == cc.ExternalDeclarationFuncDef && ed.Position().Filename == file {
			fn(ed.FunctionDefinition)
		}
	}
}

// primaryText says e prints as one operand: it needs no parentheses where
// an argument stood.
func primaryText(e cc.Node) bool {
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		return true
	case *cc.PostfixExpression:
		return x.Case != cc.PostfixExpressionInc && x.Case != cc.PostfixExpressionDec
	}
	return false
}

// --- Identity ------------------------------------------------------------------

// Identity is the step that writes a call of an identity function as its
// argument.
func Identity(core Core) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "identity", W: w}
		src, end, ast, err := coreAST(p, core, text)
		if err != nil {
			return nil, err
		}
		type ident struct {
			result cc.Type
			cast   string // the body's cast of its parameter, as the C spells it: "" for none
		}
		ids := map[string]ident{}
		funcs(ast, func(fd *cc.FunctionDefinition) {
			ft, ok := fd.Declarator.Type().(*cc.FunctionType)
			if !ok || len(ft.Parameters()) != 1 || ft.IsVariadic() {
				return
			}
			pd := ft.Parameters()[0].Declarator
			// the body's one statement; the front end declares __func__
			// in every body
			var stmts []*cc.Statement
			for l := fd.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
				if it := l.BlockItem; it.Case == cc.BlockItemStmt {
					stmts = append(stmts, it.Statement)
				} else if !funcName(it) {
					return
				}
			}
			if pd == nil || len(stmts) != 1 {
				return
			}
			st := stmts[0]
			if st.Case != cc.StatementJump || st.JumpStatement.Case != cc.JumpStatementReturn || st.JumpStatement.ExpressionList == nil {
				return
			}
			e := cc.Node(st.JumpStatement.ExpressionList)
			cast := ""
			for {
				if c, ok := e.(*cc.CastExpression); ok && c.Case == cc.CastExpressionCast {
					if cast == "" {
						a, z := spanOf(c.TypeName, src)
						cast = "(" + strings.TrimSpace(string(src[a:z])) + ")"
					}
					e = c.CastExpression
					continue
				}
				if l, ok := e.(*cc.ExpressionList); ok && l.ExpressionList == nil {
					e = l.AssignmentExpression
					continue
				}
				if q, ok := e.(*cc.PrimaryExpression); ok && q.Case == cc.PrimaryExpressionExpr {
					e = q.ExpressionList
					continue
				}
				break
			}
			if d := identOf(e); d != nil && d.IsParam() && d.Name() == pd.Name() {
				ids[fd.Declarator.Name()] = ident{ft.Result(), cast}
			}
		})
		var rws []nrw
		n := 0
		funcs(ast, func(fd *cc.FunctionDefinition) {
			if _, self := ids[fd.Declarator.Name()]; self {
				return
			}
			sweep.Walk(fd.CompoundStatement, func(nd cc.Node) bool {
				x, ok := nd.(*cc.PostfixExpression)
				if !ok || x.Case != cc.PostfixExpressionCall {
					return true
				}
				d := fnDesignator(x.PostfixExpression)
				if d == nil {
					return true
				}
				id, ok := ids[d.Name()]
				if !ok || x.ArgumentExpressionList == nil || x.ArgumentExpressionList.ArgumentExpressionList != nil {
					return true
				}
				arg := x.ArgumentExpressionList.AssignmentExpression
				a, z := spanOf(x, src)
				aa, az := spanOf(arg, src)
				// the body's cast, where the argument is not of the result's
				// type already
				cast := id.cast
				if arg.Type() != nil && arg.Type().Decay().String() == id.result.String() {
					cast = ""
				}
				bare := primaryText(arg)
				rws = append(rws, nrw{a: a, z: z, text: func(render func(a, z int) string) string {
					inner := render(aa, az)
					if !bare {
						inner = "(" + inner + ")"
					}
					return cast + inner
				}})
				n++
				return true
			})
		})
		fmt.Fprintf(w, "  %s: %d calls of %d identity functions are their arguments\n", p.Tag, n, len(ids))
		if n == 0 {
			return text, nil
		}
		out, err := applyNested(src, rws)
		if err != nil {
			return nil, p.Die("%v", err)
		}
		return append(out, text[end:]...), nil
	}
}

// funcName says the block item is the front end's __func__.
func funcName(it *cc.BlockItem) bool {
	if it.Case != cc.BlockItemDecl || it.Declaration == nil {
		return false
	}
	l := it.Declaration.InitDeclaratorList
	return l != nil && l.InitDeclaratorList == nil && l.InitDeclarator.Declarator != nil && l.InitDeclarator.Declarator.Name() == "__func__"
}

// fnDesignator is the function a call's designator names, or nil.
func fnDesignator(e cc.ExpressionNode) *cc.Declarator {
	for {
		if q, ok := e.(*cc.PrimaryExpression); ok && q.Case == cc.PrimaryExpressionExpr {
			if l, ok := q.ExpressionList.(*cc.ExpressionList); ok && l.ExpressionList == nil {
				e = l.AssignmentExpression
				continue
			}
			e = q.ExpressionList
			continue
		}
		break
	}
	d := identOf(e)
	if d == nil || d.Type() == nil || d.Type().Kind() != cc.Function {
		return nil
	}
	return d
}

// --- AsciiClass ----------------------------------------------------------------

// asciiClasses are the tests, by the character and the bound: the name.
var asciiClasses = map[string]string{"'A' 26": "ascii_isupper", "'a' 26": "ascii_islower", "'0' 10": "ascii_isdigit"}

// AsciiClass is the step that names C's one-comparison ASCII class tests.
func AsciiClass(core Core) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "asciiclass", W: w}
		src, end, ast, err := coreAST(p, core, text)
		if err != nil {
			return nil, err
		}
		for _, name := range asciiClasses {
			if len(ast.Scope.Nodes[name]) > 0 {
				return nil, p.Die("%s is taken", name)
			}
		}
		var rws []nrw
		used := map[string]int{}
		funcs(ast, func(fd *cc.FunctionDefinition) {
			sweep.Walk(fd.CompoundStatement, func(nd cc.Node) bool {
				r, ok := nd.(*cc.RelationalExpression)
				if !ok || r.Case != cc.RelationalExpressionLt {
					return true
				}
				bound := strings.TrimSpace(string(src[spanA(r.ShiftExpression, src):spanZ(r.ShiftExpression, src)]))
				sub, ok := r.RelationalExpression.(*cc.AdditiveExpression)
				if !ok || sub.Case != cc.AdditiveExpressionSub {
					return true
				}
				ch := strings.TrimSpace(string(src[spanA(sub.MultiplicativeExpression, src):spanZ(sub.MultiplicativeExpression, src)]))
				name, ok := asciiClasses[ch+" "+bound]
				if !ok {
					return true
				}
				c, ok := sub.AdditiveExpression.(*cc.CastExpression)
				if !ok || c.Case != cc.CastExpressionCast || c.Type() == nil || c.Type().Kind() != cc.UInt {
					return true
				}
				a, z := spanOf(r, src)
				xa, xz := spanOf(c.CastExpression, src)
				rws = append(rws, nrw{a: a, z: z, text: func(render func(a, z int) string) string {
					return name + "(" + render(xa, xz) + ")"
				}})
				used[name]++
				return false
			})
		})
		n := 0
		for _, k := range used {
			n += k
		}
		fmt.Fprintf(w, "  %s: %d tests named (%v)\n", p.Tag, n, used)
		if n == 0 {
			return text, nil
		}
		out, err := applyNested(src, rws)
		if err != nil {
			return nil, p.Die("%v", err)
		}
		// the functions, at the top, as the tests were written
		var defs strings.Builder
		for _, k := range []string{"'A' 26", "'a' 26", "'0' 10"} {
			name := asciiClasses[k]
			if used[name] == 0 {
				continue
			}
			f := strings.Fields(k)
			fmt.Fprintf(&defs, "static inline bool\n%s(int c)\n{\n    return (unsigned)c - %s < %s;\n}\n\n", name, f[0], f[1])
		}
		return append([]byte(defs.String()), append(out, text[end:]...)...), nil
	}
}

func spanA(n cc.Node, src []byte) int { a, _ := spanOf(n, src); return a }
func spanZ(n cc.Node, src []byte) int { _, z := spanOf(n, src); return z }

// --- ConstBranch ---------------------------------------------------------------

// ConstBranch is the step that writes an if of a constant condition as the
// branch it takes.
func ConstBranch(core Core) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "constbranch", W: w}
		src, end, ast, err := coreAST(p, core, text)
		if err != nil {
			return nil, err
		}
		var rws []nrw
		n := 0
		funcs(ast, func(fd *cc.FunctionDefinition) {
			sweep.Walk(fd.CompoundStatement, func(nd cc.Node) bool {
				s, ok := nd.(*cc.SelectionStatement)
				if !ok || (s.Case != cc.SelectionStatementIf && s.Case != cc.SelectionStatementIfElse) {
					return true
				}
				var v int64
				switch x := s.ExpressionList.Value().(type) {
				case cc.Int64Value:
					v = int64(x)
				case cc.UInt64Value:
					v = int64(x)
				default:
					return true
				}
				if hasLabel(s.Statement) || s.Case == cc.SelectionStatementIfElse && hasLabel(s.Statement2) {
					return true
				}
				a, z := spanOf(s, src)
				var take cc.Node
				switch {
				case v != 0:
					take = s.Statement
				case s.Case == cc.SelectionStatementIfElse:
					take = s.Statement2
				}
				if take == nil {
					rws = append(rws, nrw{a: a, z: z, text: func(func(a, z int) string) string { return "{\n}" }})
				} else {
					ta, tz := spanOf(take, src)
					rws = append(rws, nrw{a: a, z: z, text: func(render func(a, z int) string) string { return render(ta, tz) }})
				}
				n++
				return true
			})
		})
		fmt.Fprintf(w, "  %s: %d ifs of a constant condition are the branch they take\n", p.Tag, n)
		if n == 0 {
			return text, nil
		}
		out, err := applyNested(src, rws)
		if err != nil {
			return nil, p.Die("%v", err)
		}
		return append(out, text[end:]...), nil
	}
}

// hasLabel says s holds a label or a case: a way in from outside it.
func hasLabel(s cc.Node) bool {
	found := false
	sweep.Walk(s, func(n cc.Node) bool {
		if _, ok := n.(*cc.LabeledStatement); ok {
			found = true
		}
		return !found
	})
	return found
}
