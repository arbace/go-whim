package xform

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// GotoTailKnobs are GotoTail's: Tail is the most statements a label's tail
// may hold before its return -- the bound on what one goto may copy.
type GotoTailKnobs struct {
	Tail int
}

// GotoTail is the step that copies a short tail over every `goto L;` whose
// label L marks one: at most k.Tail expression statements, each a block item
// after the label's own, and then `return x;` -- or, in a function that
// returns void, the end of its body, which is `return;`.  At the goto the copy
// does what the jump leads to, in the same state, and returns; so it does not
// matter which loops or switches lie between the goto and the label.  It is
// GotoReturn's rule widened from no statement to a few.  Empty statements
// are not counted and not copied.
//
// A goto is held, and so is its label, when the copy could mean something
// else or could not be written twice:
//
//   - a name the tail reads is declared in an inner block of the function
//     (it might shadow at the goto or at the label), or at the function's top
//     after the goto or after the label (GotoReturn's rule; a typedef name
//     counts as a name);
//   - the tail holds a label, a case or a default (another way in, or a
//     second copy of one), a declaration (the copy would declare it twice,
//     or where the goto's scope differs), or a statement expression (a block
//     inside an expression, which could declare, be labeled, or break and
//     continue where the copy binds them elsewhere).
//
// A tail holding any other statement -- an if, a loop, a jump that is not
// the return -- or more than k.Tail statements is not a tail: its gotos stay
// and are counted apart.
//
// A label no goto reaches any more goes, unless its address is taken (`&&L`,
// which a computed goto may jump to).  When the statement before it always
// jumps, nothing reaches its tail any more either, and the tail goes with it
// (DeadStmt's rule); a label on an empty statement goes with the statement.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N gotos
// are rewritten.  Without it there is none.
func GotoTail(k GotoTailKnobs) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "gototail", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		out, r, err := gotoTail(p, k, text)
		if err != nil {
			return nil, err
		}
		if r.taken < f["--at-least"] {
			return nil, p.Die("%d gotos take their tail, fewer than the %d this step was told to expect", r.taken, f["--at-least"])
		}
		for _, s := range r.lines(k) {
			p.Say(s)
		}
		return out, nil
	}
}

// tailReport is what one run of GotoTail did.
type tailReport struct {
	taken  int            // gotos rewritten
	held   map[string]int // gotos held, by reason
	other  int            // gotos to a label that marks no short tail
	labels int            // labels that went
	dead   int            // ... of them with a tail nothing else reached
	kept   int            // labels no goto reaches, kept for their address
	freed  []string       // functions left with no goto
	before int            // functions with a goto before
}

func (r tailReport) lines(k GotoTailKnobs) []string {
	var why []string
	for _, w := range []string{"a name", "a label", "a case", "a declaration", "a statement expression"} {
		if r.held[w] > 0 {
			why = append(why, fmt.Sprintf("%d for %s", r.held[w], w))
		}
	}
	held := 0
	for _, n := range r.held {
		held += n
	}
	hs := fmt.Sprintf("%d held", held)
	if len(why) > 0 {
		hs += " (" + strings.Join(why, ", ") + ")"
	}
	out := []string{
		fmt.Sprintf("%d gotos to a tail of at most %d statements and a return take the tail; %s; %d to a label that marks no such tail stay",
			r.taken, k.Tail, hs, r.other),
		fmt.Sprintf("%d labels go, %d with a tail nothing else reaches; %d that no goto reaches stay, for their address is taken", r.labels, r.dead, r.kept),
		fmt.Sprintf("%d of the %d functions with a goto are left with none: %s", len(r.freed), r.before, strings.Join(r.freed, " ")),
	}
	return out
}

// A tail is what a label marks: the statements from the label's own to the
// return, and why a goto to it is held when every goto to it is.
type tail struct {
	ls    *cc.LabeledStatement
	parts []cc.Node // the statements copied, empty ones left out
	last  cc.Node   // the last statement of the tail, for its span
	why   string    // non-empty: every goto to it is held for this
	prev  *cc.BlockItem
	used  int // gotos to it
	stay  int // gotos to it left as they are
}

func gotoTail(p edit.Ph, k GotoTailKnobs, text []byte) ([]byte, tailReport, error) {
	const path = file
	r := tailReport{held: map[string]int{}}
	ast, err := parse(text)
	if err != nil {
		return nil, r, p.Die("the input does not parse: %v", err)
	}
	type rewrite struct {
		a, z int
		with string
	}
	var rw []rewrite
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef || ed.Position().Filename != path {
			continue
		}
		fd := ed.FunctionDefinition
		body := fd.CompoundStatement
		void := returnsVoid(fd)

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
			switch d := n.(type) {
			case *cc.Declarator:
				if d.Position().Filename == path {
					if at, ok := top[d.Name()]; !ok || at != d.Position().Offset {
						inner[d.Name()] = true
					}
				}
			case *cc.Enumerator:
				inner[d.Token.SrcStr()] = true
			}
			return true
		})

		// the labels that mark a tail, every goto, and the labels whose
		// address is taken
		tails := map[string]*tail{}
		var jumps []*cc.JumpStatement
		gotos := 0
		addr := map[string]bool{}
		asItem := map[*cc.JumpStatement]bool{} // a goto that is itself a block item
		sweep.Walk(body, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.JumpStatement:
				if x.Case == cc.JumpStatementGoto {
					jumps = append(jumps, x)
				}
				if x.Case == cc.JumpStatementGoto || x.Case == cc.JumpStatementGotoExpr {
					gotos++
				}
			case *cc.UnaryExpression:
				if x.Case == cc.UnaryExpressionLabelAddr {
					addr[x.Token2.SrcStr()] = true
				}
			case *cc.CompoundStatement:
				var items []*cc.BlockItem
				for l := x.BlockItemList; l != nil; l = l.BlockItemList {
					items = append(items, l.BlockItem)
				}
				for i, it := range items {
					if it.Case != cc.BlockItemStmt {
						continue
					}
					if s := it.Statement; s.Case == cc.StatementJump {
						asItem[s.JumpStatement] = true
					}
					if t := tailAt(items, i, x == body && void, k.Tail); t != nil {
						if i > 0 {
							t.prev = items[i-1]
						}
						tails[t.ls.Token.SrcStr()] = t
					}
				}
			}
			return true
		})
		if gotos > 0 {
			r.before++
		}
		left := gotos
		for _, j := range jumps {
			t := tails[j.Token2.SrcStr()]
			if t == nil {
				r.other++
				continue
			}
			t.used++
			why := t.why
			if why == "" && !namesAgree(t.parts, j, t.ls, top, inner) {
				why = "a name"
			}
			if why != "" {
				t.stay++
				r.held[why]++
				continue
			}
			var copies []string
			for _, n := range t.parts {
				if _, v := n.(voidReturn); v {
					copies = append(copies, "return;")
					continue
				}
				a, z := sweep.Span(n, path, text)
				copies = append(copies, string(text[a:z]))
			}
			ja, jz := sweep.Span(j, path, text)
			with := strings.Join(copies, " ")
			if len(copies) > 1 && !asItem[j] {
				with = "{ " + with + " }"
			}
			rw = append(rw, rewrite{ja, jz, with})
			r.taken++
			left--
		}
		if gotos > 0 && left == 0 {
			r.freed = append(r.freed, fd.Declarator.Name())
		}
		for _, t := range tails {
			if t.used == 0 || t.stay > 0 {
				continue
			}
			if addr[t.ls.Token.SrcStr()] {
				r.kept++
				continue
			}
			r.labels++
			la, _ := sweep.Span(t.ls, path, text)
			sa, sz := sweep.Span(t.ls.Statement, path, text)
			switch {
			case Terminates(t.prev):
				// nothing but the gotos reached it: it goes whole
				_, z := sweep.Span(t.last, path, text)
				rw = append(rw, rewrite{la, z, ""})
				r.dead++
			case emptyStmt(t.ls.Statement):
				rw = append(rw, rewrite{la, sz, ""})
			default:
				rw = append(rw, rewrite{la, sa, ""})
			}
		}
	}
	sort.Slice(rw, func(i, j int) bool { return rw[i].a > rw[j].a })
	for i := 1; i < len(rw); i++ {
		if rw[i].z > rw[i-1].a {
			return nil, r, p.Die("two rewrites overlap at %d", rw[i].a)
		}
	}
	for _, x := range rw {
		text = append(append(append([]byte{}, text[:x.a]...), x.with...), text[x.z:]...)
	}
	sort.Strings(r.freed)
	return text, r, nil
}

// tailAt is the tail items[i] marks when it is a label: its statement and the
// items after it, at most n of them expression statements (empty ones not
// counted), then a return -- or, where end says the block is a void
// function's body, its end.  It is nil when items[i] is no label or marks no
// such tail.  A tail that has the shape but holds what a copy cannot carry
// says why.
func tailAt(items []*cc.BlockItem, i int, end bool, n int) *tail {
	it := items[i]
	if it.Case != cc.BlockItemStmt || it.Statement.Case != cc.StatementLabeled ||
		it.Statement.LabeledStatement.Case != cc.LabeledStatementLabel {
		return nil
	}
	if it.Statement.LabeledStatement.Statement == nil {
		return nil // C23: a label at the end of a block
	}
	t := &tail{ls: it.Statement.LabeledStatement}
	count := 0
	hold := func(why string) {
		if t.why == "" {
			t.why = why
		}
	}
	// step takes one statement of the tail; it says whether the tail ends
	// there (a return), and false with ok false when it is no tail.
	step := func(s *cc.Statement) (done, ok bool) {
		for s.Case == cc.StatementLabeled {
			switch s.LabeledStatement.Case {
			case cc.LabeledStatementLabel:
				hold("a label")
			default:
				hold("a case")
			}
			s = s.LabeledStatement.Statement
		}
		t.last = s
		switch s.Case {
		case cc.StatementJump:
			if s.JumpStatement.Case != cc.JumpStatementReturn {
				return false, false
			}
			t.parts = append(t.parts, s.JumpStatement)
			inside(s.JumpStatement, hold)
			return true, true
		case cc.StatementExpr:
			if emptyStmt(s) {
				return false, true
			}
			count++
			t.parts = append(t.parts, s.ExpressionStatement)
			inside(s.ExpressionStatement, hold)
			return false, count <= n
		}
		return false, false
	}
	done, ok := step(it.Statement.LabeledStatement.Statement)
	for j := i + 1; ok && !done && j < len(items); j++ {
		if items[j].Case != cc.BlockItemStmt {
			hold("a declaration")
			t.last = items[j]
			continue
		}
		done, ok = step(items[j].Statement)
	}
	if !ok {
		return nil
	}
	if !done {
		if !end {
			return nil
		}
		t.parts = append(t.parts, voidReturn{})
	}
	return t
}

// voidReturn stands for the `return;` a void function's end is.
type voidReturn struct{ cc.Node }

// inside holds a tail whose statement holds a statement expression: a
// block inside an expression, which can declare, jump or be labeled -- every
// one of them a thing a copy may not carry.
func inside(n cc.Node, hold func(string)) {
	sweep.Walk(n, func(x cc.Node) bool {
		if _, ok := x.(*cc.CompoundStatement); ok {
			hold("a statement expression")
			return false
		}
		return true
	})
}

// emptyStmt says s is `;`.
func emptyStmt(s *cc.Statement) bool {
	return s.Case == cc.StatementExpr && s.ExpressionStatement.ExpressionList == nil
}

// returnsVoid says the function returns void: `void` among its specifiers,
// and its declarator the name and its parameters, with no pointer.
func returnsVoid(fd *cc.FunctionDefinition) bool {
	d := fd.Declarator
	if d.Pointer != nil {
		return false
	}
	dd := d.DirectDeclarator
	if dd.Case != cc.DirectDeclaratorFuncParam && dd.Case != cc.DirectDeclaratorFuncIdent {
		return false
	}
	if dd.DirectDeclarator.Case != cc.DirectDeclaratorIdent {
		return false
	}
	for s := fd.DeclarationSpecifiers; s != nil; s = s.DeclarationSpecifiers {
		if s.TypeSpecifier != nil && s.TypeSpecifier.Case == cc.TypeSpecifierVoid {
			return true
		}
	}
	return false
}

// namesAgree says every name the tail reads is the same object at the goto
// as at the label: no inner block declares it, and a declaration at the
// function's top comes before both.  A typedef name is a name.
func namesAgree(parts []cc.Node, jump *cc.JumpStatement, label *cc.LabeledStatement, top map[string]int, inner map[string]bool) bool {
	ok := true
	first := min(jump.Position().Offset, label.Position().Offset)
	for _, part := range parts {
		if _, v := part.(voidReturn); v {
			continue
		}
		sweep.Walk(part, func(n cc.Node) bool {
			name := ""
			switch x := n.(type) {
			case *cc.PrimaryExpression:
				if x.Case == cc.PrimaryExpressionIdent {
					name = x.Token.SrcStr()
				}
			case *cc.TypeSpecifier:
				if x.Case == cc.TypeSpecifierTypeName {
					name = x.Token.SrcStr()
				}
			}
			if name != "" {
				if inner[name] {
					ok = false
				}
				if at, is := top[name]; is && at >= first {
					ok = false
				}
			}
			return ok
		})
	}
	return ok
}
