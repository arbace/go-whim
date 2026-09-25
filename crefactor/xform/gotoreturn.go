package xform

import (
	"fmt"
	"io"
	"sort"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// GotoReturn is the step that writes `return x;` for every `goto L;` whose
// label L marks `return x;`: at the goto, the return does what the jump would
// lead to, in the same state.  The return's own text is copied to the goto.
// It takes no knobs and no arguments.
//
// One thing can make the copy mean something else: a name in x that is not
// the same object at the goto as at the label.  So a goto is held when a name
// in x is declared in any inner block of the function (it might shadow), or
// at the function's top after the goto or after the label.  A label no goto
// reaches any more goes; and the return it marked goes with it where the
// statement before it always jumps, since nothing reaches it then (DeadStmt's
// rule).
func GotoReturn() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "gotoreturn", W: w}
		if _, err := flags(p.Tag, args); err != nil {
			return nil, err
		}
		return gotoReturn(p, text)
	}
}

func gotoReturn(p edit.Ph, text []byte) ([]byte, error) {
	const path = file
	ast, err := parse(text)
	if err != nil {
		return nil, p.Die("the input does not parse: %v", err)
	}
	type rewrite struct {
		a, z int
		with string
	}
	var rw []rewrite
	gotos, held, labels, dead := 0, 0, 0, 0
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef || ed.Position().Filename != path {
			continue
		}
		body := ed.FunctionDefinition.CompoundStatement

		// the function's names: at its top, where; in an inner block, at all
		top := map[string]int{}
		inner := map[string]bool{}
		for l := body.BlockItemList; l != nil; l = l.BlockItemList {
			if it := l.BlockItem; it.Case == cc.BlockItemDecl {
				sweep.Walk(it, func(n cc.Node) bool {
					if d, ok := n.(*cc.Declarator); ok {
						top[d.Name()] = d.Position().Offset
					}
					return true
				})
			}
		}
		sweep.Walk(body, func(n cc.Node) bool {
			if d, ok := n.(*cc.Declarator); ok && d.Position().Filename == path {
				if at, ok := top[d.Name()]; !ok || at != d.Position().Offset {
					inner[d.Name()] = true
				}
			}
			return true
		})

		// the labels that mark a return, and every goto
		type target struct {
			ls   *cc.LabeledStatement
			ret  *cc.JumpStatement
			used int // gotos to it, all
			kept int // gotos to it left as they are
		}
		targets := map[string]*target{}
		var jumps []*cc.JumpStatement
		prev := map[*cc.Statement]*cc.BlockItem{} // a block item's statement -> the item before it
		sweep.Walk(body, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.LabeledStatement:
				if x.Case != cc.LabeledStatementLabel {
					break
				}
				s := x.Statement
				for s.Case == cc.StatementLabeled && s.LabeledStatement.Case == cc.LabeledStatementLabel {
					s = s.LabeledStatement.Statement
				}
				if s.Case == cc.StatementJump && s.JumpStatement.Case == cc.JumpStatementReturn {
					targets[x.Token.SrcStr()] = &target{ls: x, ret: s.JumpStatement}
				}
			case *cc.JumpStatement:
				if x.Case == cc.JumpStatementGoto {
					jumps = append(jumps, x)
				}
			case *cc.CompoundStatement:
				var last *cc.BlockItem
				for l := x.BlockItemList; l != nil; l = l.BlockItemList {
					if l.BlockItem.Case == cc.BlockItemStmt && last != nil {
						prev[l.BlockItem.Statement] = last
					}
					last = l.BlockItem
				}
			}
			return true
		})
		for _, j := range jumps {
			t := targets[j.Token2.SrcStr()]
			if t == nil {
				continue
			}
			t.used++
			if !sameNames(t.ret, j, t.ls, top, inner) {
				t.kept++
				held++
				continue
			}
			ra, rz := sweep.Span(t.ret, path, text)
			ja, jz := sweep.Span(j, path, text)
			rw = append(rw, rewrite{ja, jz, string(text[ra:rz])})
			gotos++
		}
		for _, t := range targets {
			if t.used == 0 || t.kept > 0 {
				continue
			}
			labels++
			la, _ := sweep.Span(t.ls, path, text)
			sa, _ := sweep.Span(t.ls.Statement, path, text)
			// the labeled statement, as a block item: the Statement that holds it
			var stmt *cc.Statement
			for s := range prev {
				if s.Case == cc.StatementLabeled && s.LabeledStatement == t.ls {
					stmt = s
				}
			}
			if stmt != nil && t.ls.Statement.Case == cc.StatementJump && Terminates(prev[stmt]) {
				// nothing but the gotos reached it: it goes whole
				_, z := sweep.Span(t.ls, path, text)
				rw = append(rw, rewrite{la, z, ""})
				dead++
				continue
			}
			rw = append(rw, rewrite{la, sa, ""})
		}
	}
	sort.Slice(rw, func(i, j int) bool { return rw[i].a > rw[j].a })
	for i := 1; i < len(rw); i++ {
		if rw[i].z > rw[i-1].a {
			return nil, p.Die("two rewrites overlap at %d", rw[i].a)
		}
	}
	for _, r := range rw {
		text = append(append(append([]byte{}, text[:r.a]...), r.with...), text[r.z:]...)
	}
	p.Say(fmt.Sprintf("%d gotos to a return are that return; %d held for a name; %d labels go, %d with a return nothing else reaches",
		gotos, held, labels, dead))
	return text, nil
}

// sameNames says every name the return reads is the same object at the goto
// as at the label: no inner block declares it, and a declaration at the
// function's top comes before both.
func sameNames(ret, jump *cc.JumpStatement, label *cc.LabeledStatement, top map[string]int, inner map[string]bool) bool {
	ok := true
	first := min(jump.Position().Offset, label.Position().Offset)
	sweep.Walk(ret, func(n cc.Node) bool {
		if x, is := n.(*cc.PrimaryExpression); is && x.Case == cc.PrimaryExpressionIdent {
			name := x.Token.SrcStr()
			if inner[name] {
				ok = false
			}
			if at, is := top[name]; is && at >= first {
				ok = false
			}
		}
		return ok
	})
	return ok
}
