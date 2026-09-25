package xform

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// GotoBlock is the step that takes the forward exits: every `goto L;` whose
// label L is a statement of a block B that encloses the goto, after it, with
// nothing but if/else and blocks between the goto and B.  The region of B from
// the first item holding such a goto up to L's item is wrapped in
// `do { region } while (0);` and each goto is written `break;`, which leaves
// the do-while to the statement after it -- L's.  The label goes, since
// nothing jumps to it any more.  It is a labeled block, `L: { ... break L; }`,
// in the one form C has for it.
//
// A label is taken whole or not at all: every goto to it must qualify, and the
// region must mean the same in the new block.  So it is held when
//
//   - a goto to it comes after it, or from outside its block (backward);
//   - a loop or switch lies between a goto and B: the break would bind to it;
//   - the region holds a break or continue of its own that leaves it, or a
//     case or default label whose switch is outside it: those would bind to
//     the do-while, or be in a block their switch no longer sees directly;
//   - the region declares a name that is spelled at or after the label, in the
//     rest of B: the do-while is a new scope;
//   - a goto from outside the region jumps to a label inside it: into a block;
//   - the label is inside a statement (`if (c) L: ...`), not one of B's own;
//   - the function takes a label's address or jumps to a computed one.
//
// Two labels whose rewrites would touch overlapping text (one region inside
// or across another) are not rewritten in one round: the later one waits, and
// the text is parsed again, so that a goto now inside the first one's
// do-while is judged against it -- held, being a loop between.  Rounds run
// until one rewrites nothing.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N gotos
// are rewritten.
func GotoBlock() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "gotoblock", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		out, n, err := gotoBlock(p, text)
		if err != nil {
			return nil, err
		}
		if n < f["--at-least"] {
			return nil, p.Die("%d gotos become a break, fewer than the %d this step was told to expect", n, f["--at-least"])
		}
		return out, nil
	}
}

// The reasons a label is held, in the order they are tested and reported.
const (
	holdBackward = "a goto after it or outside its block"
	holdLoop     = "a loop or switch between a goto and the label's block"
	holdStray    = "a break, continue or case label of the region's own"
	holdDecl     = "a declaration in the region spelled after the label"
	holdInto     = "a jump into the region"
	holdComputed = "a computed goto in the function"
	holdNested   = "a label inside a statement, not one of its block's"
)

var holdOrder = []string{holdComputed, holdNested, holdBackward, holdLoop, holdStray, holdDecl, holdInto}

// A gbFrame is one statement on the way down from a function's body: a
// block's item, a loop, or a switch.
type gbFrame struct {
	kind byte // 'b' a block's item, 'l' a loop, 's' a switch
	cs   *cc.CompoundStatement
	item int
}

type gbJump struct {
	j     *cc.JumpStatement
	stack []gbFrame
}

type gbLabel struct {
	ls     *cc.LabeledStatement
	stack  []gbFrame
	direct bool // a statement of its block, through labels only
}

type gbEdit struct {
	a, z int
	with string
}

func gotoBlock(p edit.Ph, text []byte) ([]byte, int, error) {
	gotos, labels, rounds := 0, 0, 0
	var held map[string][2]int // reason -> labels, gotos
	for {
		ast, err := parse(text)
		if err != nil {
			return nil, 0, p.Die("the input does not parse (round %d): %v", rounds+1, err)
		}
		held = map[string][2]int{}
		var edits []gbEdit
		var taken [][2]int // the extents rewritten this round
		ng, nl := 0, 0
		for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
			ed := tu.ExternalDeclaration
			if ed.Case != cc.ExternalDeclarationFuncDef || ed.Position().Filename != file {
				continue
			}
			for _, r := range gbFunction(ed.FunctionDefinition.CompoundStatement, text, held) {
				if overlaps(taken, r.extent) {
					continue
				}
				taken = append(taken, r.extent)
				edits = append(edits, r.edits...)
				ng += r.gotos
				nl++
			}
		}
		if len(edits) == 0 {
			break
		}
		sort.Slice(edits, func(i, j int) bool {
			if edits[i].a != edits[j].a {
				return edits[i].a > edits[j].a
			}
			return edits[i].z > edits[j].z
		})
		for i := 1; i < len(edits); i++ {
			if edits[i].z > edits[i-1].a {
				return nil, 0, p.Die("two rewrites overlap at %d", edits[i].a)
			}
		}
		for _, e := range edits {
			text = append(append(append([]byte{}, text[:e.a]...), e.with...), text[e.z:]...)
		}
		gotos += ng
		labels += nl
		rounds++
	}
	var why []string
	for _, r := range holdOrder {
		if h, ok := held[r]; ok {
			why = append(why, fmt.Sprintf("%d (%d gotos) for %s", h[0], h[1], r))
		}
	}
	if len(why) == 0 {
		why = []string{"none"}
	}
	p.Say(fmt.Sprintf("%d gotos become a break out of a do-while(0), and their %d labels go, in %d rounds that rewrote; labels held: %s",
		gotos, labels, rounds, strings.Join(why, "; ")))
	return text, gotos, nil
}

func overlaps(taken [][2]int, x [2]int) bool {
	for _, t := range taken {
		if x[0] < t[1] && t[0] < x[1] {
			return true
		}
	}
	return false
}

// A gbRewrite is one label's: its edits, and the extent of text they touch.
type gbRewrite struct {
	extent [2]int
	edits  []gbEdit
	gotos  int
}

// gbFunction finds the labels of one function body that can be taken, and
// counts the ones held into held.
func gbFunction(body *cc.CompoundStatement, text []byte, held map[string][2]int) []gbRewrite {
	const path = file
	var jumps []gbJump
	lbls := map[string]gbLabel{}
	computed := false
	var walk func(s *cc.Statement, stack []gbFrame, direct bool)
	block := func(cs *cc.CompoundStatement, stack []gbFrame) {
		i := 0
		for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
			it := l.BlockItem
			st := append(append([]gbFrame{}, stack...), gbFrame{kind: 'b', cs: cs, item: i})
			switch it.Case {
			case cc.BlockItemStmt:
				walk(it.Statement, st, true)
			case cc.BlockItemDecl:
				// a statement expression in an initializer: its jumps count too
				sweep.Walk(it, func(n cc.Node) bool {
					if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionLabelAddr {
						computed = true
					}
					return true
				})
			default:
				computed = true // a local label declaration or a nested function: not ours
			}
			i++
		}
	}
	walk = func(s *cc.Statement, stack []gbFrame, direct bool) {
		if s == nil {
			return
		}
		switch s.Case {
		case cc.StatementCompound:
			block(s.CompoundStatement, stack)
		case cc.StatementLabeled:
			ls := s.LabeledStatement
			if ls.Case == cc.LabeledStatementLabel {
				lbls[ls.Token.SrcStr()] = gbLabel{ls: ls, stack: stack, direct: direct}
			}
			walk(ls.Statement, stack, direct)
		case cc.StatementSelection:
			sel := s.SelectionStatement
			if sel.Case == cc.SelectionStatementSwitch {
				stack = append(append([]gbFrame{}, stack...), gbFrame{kind: 's'})
			}
			gbExpr(sel.ExpressionList, &computed)
			walk(sel.Statement, stack, false)
			walk(sel.Statement2, stack, false)
		case cc.StatementIteration:
			stack = append(append([]gbFrame{}, stack...), gbFrame{kind: 'l'})
			walk(s.IterationStatement.Statement, stack, false)
			gbExpr(s.IterationStatement, &computed)
		case cc.StatementJump:
			j := s.JumpStatement
			switch j.Case {
			case cc.JumpStatementGoto:
				jumps = append(jumps, gbJump{j: j, stack: stack})
			case cc.JumpStatementGotoExpr:
				computed = true
			}
			gbExpr(j.ExpressionList, &computed)
		case cc.StatementExpr:
			gbExpr(s.ExpressionStatement, &computed)
		}
	}
	block(body, nil)

	byLabel := map[string][]gbJump{}
	for _, j := range jumps {
		byLabel[j.j.Token2.SrcStr()] = append(byLabel[j.j.Token2.SrcStr()], j)
	}
	names := make([]string, 0, len(byLabel))
	for n := range byLabel {
		if _, ok := lbls[n]; ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	hold := func(why string, n int) {
		h := held[why]
		held[why] = [2]int{h[0] + 1, h[1] + n}
	}
	var out []gbRewrite
	for _, name := range names {
		L := lbls[name]
		js := byLabel[name]
		if computed {
			hold(holdComputed, len(js))
			continue
		}
		if !L.direct {
			hold(holdNested, len(js))
			continue
		}
		// B, and L's item k in it
		var lb gbFrame
		for _, f := range L.stack {
			if f.kind == 'b' {
				lb = f
			}
		}
		B, k := lb.cs, lb.item
		items := blockItems(B)
		i0 := k
		why := ""
		for _, j := range js {
			at := -1
			for d, f := range j.stack {
				if f.kind == 'b' && f.cs == B {
					at = d
				}
			}
			if at < 0 || j.stack[at].item >= k {
				why = holdBackward
				break
			}
			for _, f := range j.stack[at+1:] {
				if f.kind != 'b' {
					why = holdLoop
				}
			}
			i0 = min(i0, j.stack[at].item)
		}
		if why == "" && gbStray(items[i0:k]) {
			why = holdStray
		}
		if why == "" && gbDeclUsed(items[i0:k], items[k:], text) {
			why = holdDecl
		}
		if why == "" {
			// a label inside the region reached by a goto outside it
			in := func(stack []gbFrame) bool {
				for _, f := range stack {
					if f.kind == 'b' && f.cs == B && f.item >= i0 && f.item < k {
						return true
					}
				}
				return false
			}
			for m, ml := range lbls {
				if m == name || !in(ml.stack) {
					continue
				}
				for _, j := range byLabel[m] {
					if !in(j.stack) {
						why = holdInto
					}
				}
			}
		}
		if why != "" {
			hold(why, len(js))
			continue
		}

		ra, _ := sweep.Span(items[i0], path, text)
		ka, kz := sweep.Span(items[k], path, text)
		la, _ := sweep.Span(L.ls, path, text)
		sa, sz := sweep.Span(L.ls.Statement, path, text)
		r := gbRewrite{extent: [2]int{ra, kz}, gotos: len(js)}
		r.edits = append(r.edits, gbEdit{ra, ra, "do {\n"}, gbEdit{ka, ka, "} while (0);\n"})
		for _, j := range js {
			a, z := sweep.Span(j.j, path, text)
			r.edits = append(r.edits, gbEdit{a, z, "break;"})
		}
		s := L.ls.Statement
		if la == ka && s.Case == cc.StatementExpr && s.ExpressionStatement.ExpressionList == nil && sz > sa {
			// `L: ;` alone: the empty statement was there for the label
			r.edits = append(r.edits, gbEdit{la, sz, ""})
		} else {
			r.edits = append(r.edits, gbEdit{la, sa, ""})
		}
		out = append(out, r)
	}
	return out
}

// gbExpr notes a label's address taken anywhere under n.
func gbExpr(n cc.Node, computed *bool) {
	if n == nil {
		return
	}
	sweep.Walk(n, func(n cc.Node) bool {
		if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionLabelAddr {
			*computed = true
		}
		return true
	})
}

func blockItems(cs *cc.CompoundStatement) []*cc.BlockItem {
	var out []*cc.BlockItem
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		out = append(out, l.BlockItem)
	}
	return out
}

// gbStray says the items hold a break or continue that leaves them, or a
// case or default label whose switch is not among them.
func gbStray(items []*cc.BlockItem) bool {
	stray := false
	var walk func(s *cc.Statement, loops, switches int)
	walk = func(s *cc.Statement, loops, switches int) {
		if s == nil || stray {
			return
		}
		switch s.Case {
		case cc.StatementCompound:
			for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
				if l.BlockItem.Case == cc.BlockItemStmt {
					walk(l.BlockItem.Statement, loops, switches)
				}
			}
		case cc.StatementLabeled:
			if s.LabeledStatement.Case != cc.LabeledStatementLabel && switches == 0 {
				stray = true
			}
			walk(s.LabeledStatement.Statement, loops, switches)
		case cc.StatementSelection:
			sel := s.SelectionStatement
			if sel.Case == cc.SelectionStatementSwitch {
				switches++
			}
			walk(sel.Statement, loops, switches)
			walk(sel.Statement2, loops, switches)
		case cc.StatementIteration:
			// a loop takes break and continue; a case inside it is still
			// its switch's, so the count of switches carries through
			walk(s.IterationStatement.Statement, loops+1, switches)
		case cc.StatementJump:
			switch s.JumpStatement.Case {
			case cc.JumpStatementBreak:
				stray = stray || loops+switches == 0
			case cc.JumpStatementContinue:
				stray = stray || loops == 0
			}
		}
	}
	for _, it := range items {
		if it.Case == cc.BlockItemStmt {
			walk(it.Statement, 0, 0)
		}
	}
	return stray
}

var ident = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*$`)

// gbDeclUsed says a declaration among region, at its level, names something
// -- an object, a type, a tag, an enumerator -- whose spelling appears in
// rest: in the new scope it would no longer be seen there.  By spelling, so
// a name that is only shadowed after is held too.
func gbDeclUsed(region, rest []*cc.BlockItem, text []byte) bool {
	var names []string
	for _, it := range region {
		if it.Case != cc.BlockItemDecl {
			continue
		}
		sweep.Walk(it, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.Declarator:
				names = append(names, x.Name())
			case *cc.Enumerator:
				names = append(names, x.Token.SrcStr())
			case *cc.StructOrUnionSpecifier:
				if s := x.Token.SrcStr(); ident.MatchString(s) {
					names = append(names, s)
				}
			case *cc.EnumSpecifier:
				if s := x.Token.SrcStr(); ident.MatchString(s) {
					names = append(names, s)
				}
			}
			return true
		})
	}
	if len(names) == 0 || len(rest) == 0 {
		return false
	}
	a, _ := sweep.Span(rest[0], file, text)
	_, z := sweep.Span(rest[len(rest)-1], file, text)
	after := text[a:z]
	for _, n := range names {
		if n == "" {
			continue
		}
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(n) + `\b`).Match(after) {
			return true
		}
	}
	return false
}
