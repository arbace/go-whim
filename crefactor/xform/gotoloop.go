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

// GotoLoop is the step that writes a backward goto as the loop it is.  The
// label L marks item k of a block, and every goto to it is under an item i >= k
// of the same block; the items k..i -- the region, i the last goto's -- become
//
//	for (;;) { <items k..i>; break; }     each `goto L;` -> `continue;`
//
// (no `break;` where item i always jumps), or, where the one goto is item i
// itself as `if (c) goto L;`,
//
//	do { <items k..i-1> } while (c);
//
// and the label goes.  A loop runs its body again where the goto jumped back,
// and leaves it where control ran off the region's end.
//
// A label is held, with every goto to it, when the loop would mean something
// else:
//
//   - a goto to it comes from before it (it would jump into the loop);
//   - a loop or switch lies between a goto and the block (the `continue`
//     would bind to it, or the goto is not a plain continue of the region);
//   - a break or continue in the region leaves it (the new loop would take it);
//   - a goto from outside the region reaches a label inside it, or a case
//     label in it belongs to a switch around it (control would enter the loop
//     from the side);
//   - a declaration among items k..i is named after the region (the loop's
//     braces would end its scope);
//   - another label marks item k with L, or the label is not a block item;
//   - its region overlaps one taken earlier in the same function.
//
// The do form is written only where c names nothing the region declares.  A
// function with a computed goto, a label's address, a local label, a nested
// function or a jump inside an expression is left as it is.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N gotos
// become loops.  Without it there is none.
func GotoLoop() Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "gotoloop", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		return gotoLoop(p, text, f["--at-least"])
	}
}

func gotoLoop(p edit.Ph, text []byte, floor int) ([]byte, error) {
	ast, err := parse(text)
	if err != nil {
		return nil, p.Die("the input does not parse: %v", err)
	}
	var rw []rewrite
	gotos, dos, fors, odd := 0, 0, 0, 0
	held := map[string]int{}
	functions(ast, func(body *cc.CompoundStatement) {
		fl := flowOf(body)
		if fl.odd {
			odd++
			return
		}
		var names []string
		for name := range fl.labels {
			names = append(names, name)
		}
		sort.Strings(names)
		var taken [][2]int // the regions rewritten, as spans
		for _, name := range names {
			r, why := loopOf(fl, name, text)
			if r == nil {
				if why != "" {
					held[why] += fl.gotos[name]
				}
				continue
			}
			over := false
			for _, t := range taken {
				over = over || r.a < t[1] && t[0] < r.z
			}
			if over {
				held["its region overlaps another's"] += fl.gotos[name]
				continue
			}
			taken = append(taken, [2]int{r.a, r.z})
			rw = append(rw, r.rewrite)
			gotos += fl.gotos[name]
			if r.do {
				dos++
			} else {
				fors++
			}
		}
	})
	out, ok := apply(text, rw)
	if !ok {
		return nil, p.Die("two rewrites overlap")
	}
	nheld := 0
	var whys []string
	for why, n := range held {
		nheld += n
		whys = append(whys, fmt.Sprintf("%d %s", n, why))
	}
	sort.Strings(whys)
	p.Say(fmt.Sprintf("%d backward gotos are loops: %d labels as do-while, %d as for (;;), each label gone; %d held (%s); %d functions left, doing what the walk does not model",
		gotos, dos, fors, nheld, strings.Join(whys, ", "), odd))
	if gotos < floor {
		return nil, p.Die("%d gotos become loops, fewer than the %d this step was told to expect", gotos, floor)
	}
	return out, nil
}

// A loop is one label's rewrite.
type loop struct {
	rewrite
	do bool
}

// inRegion says the site's statement is under items k..i of block b.
func inRegion(st []frame, depth int, b cc.Node, k, i int) bool {
	return len(st) > depth && st[depth].kind == inBlock && st[depth].node == b && k <= st[depth].idx && st[depth].idx <= i
}

// loopOf is the loop label name's backward gotos make, or nil and why not
// ("" for a label that is not the head of a backward jump at all).
func loopOf(fl *flow, name string, text []byte) (*loop, string) {
	lab := fl.labels[name]
	la, _ := sweep.Span(lab.ls, file, text)
	var gs []site
	back := false
	for _, j := range fl.jumps {
		if j.js.Case == cc.JumpStatementGoto && j.js.Token2.SrcStr() == name {
			gs = append(gs, j)
			a, _ := sweep.Span(j.js, file, text)
			back = back || a > la
		}
	}
	if !back {
		return nil, ""
	}
	for _, g := range gs {
		if a, _ := sweep.Span(g.js, file, text); a < la {
			return nil, "a goto comes from before its label"
		}
	}
	d := len(lab.stack) - 1
	if d < 0 || lab.stack[d].kind != inBlock {
		return nil, "the label is not a block item"
	}
	if len(labelsOf(lab.stack[d].items[lab.stack[d].idx])) != 1 {
		return nil, "another label marks the statement"
	}
	b, items, k := lab.stack[d].node, lab.stack[d].items, lab.stack[d].idx
	i := k
	for _, g := range gs {
		if !inRegion(g.stack, d, b, k, len(items)-1) {
			return nil, "a goto is not in the label's block"
		}
		if breakable(g.stack) > d {
			return nil, "a loop or switch between goto and label"
		}
		i = max(i, g.stack[d].idx)
	}
	// nothing in the region leaves it but by return or goto, and nothing
	// enters it but at the top
	for _, j := range fl.jumps {
		if j.js.Case == cc.JumpStatementGoto || !inRegion(j.stack, d, b, k, i) {
			continue
		}
		inner := false
		for _, f := range j.stack[d+1:] {
			inner = inner || f.kind == inLoop || f.kind == inSwitch && j.js.Case == cc.JumpStatementBreak
		}
		if !inner {
			return nil, "a break or continue leaves the region"
		}
	}
	for other, l := range fl.labels {
		if other == name || !inRegion(l.stack, d, b, k, i) {
			continue
		}
		for _, j := range fl.jumps {
			if j.js.Case == cc.JumpStatementGoto && j.js.Token2.SrcStr() == other && !inRegion(j.stack, d, b, k, i) {
				return nil, "a goto enters the region"
			}
		}
	}
	for _, c := range fl.cases {
		if inRegion(c.stack, d, b, k, i) && breakable(c.stack) <= d {
			return nil, "a case label of a switch around the region is in it"
		}
	}
	// a name the region declares at its level, named after it
	var declared []string
	for _, it := range items[k : i+1] {
		if it.Case == cc.BlockItemDecl {
			sweep.Walk(it, func(n cc.Node) bool {
				if x, ok := n.(*cc.Declarator); ok {
					declared = append(declared, x.Name())
				}
				return true
			})
		}
	}
	if i+1 < len(items) {
		a, _ := sweep.Span(items[i+1], file, text)
		_, z := sweep.Span(items[len(items)-1], file, text)
		if names(text[a:z], declared) {
			return nil, "a declaration in the region is named after it"
		}
	}

	head, _ := sweep.Span(lab.ls.Statement, file, text)
	_, end := sweep.Span(items[i], file, text)
	// the do form: the one goto is item i, `if (c) goto L;`
	if len(gs) == 1 && i > k {
		if c := guard(items[i], gs[0].js); c != nil {
			ca, cz := sweep.Span(c, file, text)
			if !names(text[ca:cz], declared) {
				_, z := sweep.Span(items[i-1], file, text)
				with := "do\n{\n" + string(text[head:z]) + "\n}\nwhile (" + string(text[ca:cz]) + ");"
				return &loop{rewrite{la, end, with}, true}, ""
			}
		}
	}
	var in []rewrite
	for _, g := range gs {
		a, z := sweep.Span(g.js, file, text)
		in = append(in, rewrite{a - head, z - head, "continue;"})
	}
	body, _ := apply(append([]byte{}, text[head:end]...), in)
	tail := "\nbreak;\n"
	last := items[i].Statement
	if i == k {
		last = lab.ls.Statement // the label goes
	}
	if items[i].Case == cc.BlockItemStmt && StmtTerminates(last) {
		tail = "\n"
	}
	return &loop{rewrite{la, end, "for (;;)\n{\n" + string(body) + tail + "}"}, false}, ""
}

// guard is c when the item is `if (c) goto L;` or `if (c) { goto L; }` with
// no else, the goto being g; else nil.
func guard(it *cc.BlockItem, g *cc.JumpStatement) cc.ExpressionNode {
	if it.Case != cc.BlockItemStmt || it.Statement.Case != cc.StatementSelection {
		return nil
	}
	sel := it.Statement.SelectionStatement
	if sel.Case != cc.SelectionStatementIf {
		return nil
	}
	s := sel.Statement
	if s.Case == cc.StatementCompound {
		l := s.CompoundStatement.BlockItemList
		if l == nil || l.BlockItemList != nil || l.BlockItem.Case != cc.BlockItemStmt {
			return nil
		}
		s = l.BlockItem.Statement
	}
	if s.Case != cc.StatementJump || s.JumpStatement != g {
		return nil
	}
	return sel.ExpressionList
}

// names says the text names one of the identifiers, as a whole word -- so a
// string or a member of the same spelling counts too, which only holds more.
func names(text []byte, ids []string) bool {
	for _, id := range ids {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(id) + `\b`).Match(text) {
			return true
		}
	}
	return false
}
