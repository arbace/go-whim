package xform

import (
	"sort"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// The statement structure of one function, as GotoBreak and GotoLoop read
// it: every jump and every label with the chain of statements that holds it.

// A frame says how a statement is held by the one around it.
type frame struct {
	kind  frameKind
	node  cc.Node         // the compound, if, loop, switch or labeled statement
	items []*cc.BlockItem // inBlock: the compound's items
	idx   int             // inBlock: the statement's item
}

type frameKind int

const (
	inBlock  frameKind = iota // an item of a compound statement
	inThen                    // the then-branch of an if
	inElse                    // the else-branch of an if
	inLoop                    // the body of a while, do or for
	inSwitch                  // the body of a switch
	inLabel                   // the statement of an ordinary label
	inCase                    // the statement of a case or default label
)

// A site is a statement found by the walk: a jump or a labeled statement,
// and the frames that hold it, outermost first.
type site struct {
	js    *cc.JumpStatement
	ls    *cc.LabeledStatement
	stack []frame
}

// A flow is one function body as the walk found it.
type flow struct {
	body   *cc.CompoundStatement
	jumps  []site          // every goto, break and continue
	labels map[string]site // every ordinary label
	cases  []site          // every case and default label
	gotos  map[string]int  // gotos to each label
	// odd is set when the function does what the walk does not model: a
	// computed goto, a label's address, a local label, a nested function, or a
	// jump or label inside an expression.  Such a function is left alone.
	odd bool
}

func flowOf(body *cc.CompoundStatement) *flow {
	f := &flow{body: body, labels: map[string]site{}, gotos: map[string]int{}}
	seen := map[cc.Node]bool{}
	push := func(st []frame, fr frame) []frame {
		n := make([]frame, len(st), len(st)+1)
		copy(n, st)
		return append(n, fr)
	}
	var stmt func(s *cc.Statement, st []frame)
	block := func(c *cc.CompoundStatement, st []frame) {
		var items []*cc.BlockItem
		for l := c.BlockItemList; l != nil; l = l.BlockItemList {
			items = append(items, l.BlockItem)
		}
		for i, it := range items {
			if it.Case == cc.BlockItemStmt {
				stmt(it.Statement, push(st, frame{kind: inBlock, node: c, items: items, idx: i}))
			}
		}
	}
	stmt = func(s *cc.Statement, st []frame) {
		if s == nil {
			return
		}
		switch s.Case {
		case cc.StatementCompound:
			block(s.CompoundStatement, st)
		case cc.StatementLabeled:
			ls := s.LabeledStatement
			seen[ls] = true
			if ls.Case == cc.LabeledStatementLabel {
				f.labels[ls.Token.SrcStr()] = site{ls: ls, stack: st}
				stmt(ls.Statement, push(st, frame{kind: inLabel, node: ls}))
			} else {
				f.cases = append(f.cases, site{ls: ls, stack: st})
				stmt(ls.Statement, push(st, frame{kind: inCase, node: ls}))
			}
		case cc.StatementSelection:
			ss := s.SelectionStatement
			if ss.Case == cc.SelectionStatementSwitch {
				stmt(ss.Statement, push(st, frame{kind: inSwitch, node: ss}))
				break
			}
			stmt(ss.Statement, push(st, frame{kind: inThen, node: ss}))
			if ss.Case == cc.SelectionStatementIfElse {
				stmt(ss.Statement2, push(st, frame{kind: inElse, node: ss}))
			}
		case cc.StatementIteration:
			is := s.IterationStatement
			stmt(is.Statement, push(st, frame{kind: inLoop, node: is}))
		case cc.StatementJump:
			js := s.JumpStatement
			seen[js] = true
			switch js.Case {
			case cc.JumpStatementGoto, cc.JumpStatementBreak, cc.JumpStatementContinue:
				f.jumps = append(f.jumps, site{js: js, stack: st})
			}
		}
	}
	block(body, nil)
	sweep.Walk(body, func(n cc.Node) bool {
		switch x := n.(type) {
		case *cc.JumpStatement:
			if x.Case == cc.JumpStatementGotoExpr || !seen[x] {
				f.odd = true
			}
			if x.Case == cc.JumpStatementGoto {
				f.gotos[x.Token2.SrcStr()]++
			}
		case *cc.LabeledStatement:
			f.odd = f.odd || !seen[x]
		case *cc.UnaryExpression:
			f.odd = f.odd || x.Case == cc.UnaryExpressionLabelAddr
		case *cc.BlockItem:
			f.odd = f.odd || x.Case == cc.BlockItemLabel || x.Case == cc.BlockItemFuncDef
		}
		return true
	})
	return f
}

// next is where control goes when the statement held by st runs off its
// end: the item after it in its block, or after the block, the if or the
// label around it, or after a switch whose body ends there.  It is not found
// when that is a loop's test, or the function's end.
func next(st []frame) (frame, bool) {
	for j := len(st) - 1; j >= 0; j-- {
		f := st[j]
		switch f.kind {
		case inBlock:
			if f.idx+1 < len(f.items) {
				f.idx++
				return f, true
			}
		case inLoop:
			return frame{}, false
		}
	}
	return frame{}, false
}

// labelsOf is the ordinary labels that mark the block item, outermost first.
func labelsOf(it *cc.BlockItem) []string {
	var out []string
	if it.Case != cc.BlockItemStmt {
		return nil
	}
	for s := it.Statement; s.Case == cc.StatementLabeled && s.LabeledStatement.Case == cc.LabeledStatementLabel; s = s.LabeledStatement.Statement {
		out = append(out, s.LabeledStatement.Token.SrcStr())
	}
	return out
}

// marks says the label named marks the item at f.
func marks(f frame, name string) bool {
	for _, l := range labelsOf(f.items[f.idx]) {
		if l == name {
			return true
		}
	}
	return false
}

// breakable is the index in st of the innermost loop or switch, or -1.
func breakable(st []frame) int {
	for j := len(st) - 1; j >= 0; j-- {
		if st[j].kind == inLoop || st[j].kind == inSwitch {
			return j
		}
	}
	return -1
}

// dropLabel is the span of `L:` before the labeled statement's own statement.
func dropLabel(ls *cc.LabeledStatement, text []byte) (int, int) {
	a, _ := sweep.Span(ls, file, text)
	b, _ := sweep.Span(ls.Statement, file, text)
	return a, b
}

// A rewrite replaces text[a:z] with with.
type rewrite struct {
	a, z int
	with string
}

// apply makes the rewrites, which must not overlap.
func apply(text []byte, rw []rewrite) ([]byte, bool) {
	sort.Slice(rw, func(i, j int) bool { return rw[i].a > rw[j].a })
	for i := 1; i < len(rw); i++ {
		if rw[i].z > rw[i-1].a {
			return nil, false
		}
	}
	for _, r := range rw {
		text = append(append(append([]byte{}, text[:r.a]...), r.with...), text[r.z:]...)
	}
	return text, true
}

// functions calls f on the body of every function the text defines.
func functions(ast *cc.AST, f func(body *cc.CompoundStatement)) {
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef || ed.Position().Filename != file {
			continue
		}
		f(ed.FunctionDefinition.CompoundStatement)
	}
}
