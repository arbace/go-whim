package graph

import (
	"fmt"
	"sort"
	"strings"
)

// GOTOLOOP (doc/GRAPH-MIGRATION.md, B3g): crefactor/xform's GotoLoop on
// the nodes, with MOVE: the region's items go into the loop keeping their
// ids.
//
// A backward goto is the loop it is.  The label L marks item k of a block,
// and every goto to it is under an item i >= k of the same block; the items
// k..i -- the region -- become
//
//	for (;;) { <items k..i>; break; }     each `goto L;` -> `continue;`
//
// (no `break;` where item i always jumps), or, where the one goto is item i
// itself as `if (c) goto L;`,
//
//	do { <items k..i-1> } while (c);
//
// and the label goes.  A label is held, with every goto to it, when the
// loop would mean something else: a goto to it comes from before it; a loop
// or switch lies between a goto and the block; a break or continue in the
// region leaves it; a goto from outside the region reaches a label inside
// it, or a case label in it belongs to a switch around it; a declaration
// among items k..i is named after the region; another label marks item k
// with L, or the label is not a block item; its region overlaps one taken
// earlier in the same function.  The do form is written only where c names
// nothing the region declares.

// LoopReport is what one run of GotoLoop did.
type LoopReport struct {
	Gotos, Dos, Fors, Odd int
	Held                  map[string]int
}

// Line is the report as the text step wrote it.
func (r LoopReport) Line() string {
	n := 0
	var whys []string
	for why, k := range r.Held {
		n += k
		whys = append(whys, fmt.Sprintf("%d %s", k, why))
	}
	sort.Strings(whys)
	return fmt.Sprintf("%d backward gotos are loops: %d labels as do-while, %d as for (;;), each label gone; %d held (%s); %d functions left, doing what the walk does not model",
		r.Gotos, r.Dos, r.Fors, n, strings.Join(whys, ", "), r.Odd)
}

// A glLoop is one label's rewrite.
type glLoop struct {
	label *Node
	gotos []*Node
	run   []*Node // the region, the label first
	cond  *Node   // the do form's condition: the guard of its last item
	tail  bool    // the for form ends in a break
	a, z  int     // the region's extent
}

// GotoLoop writes each backward goto as its loop.
func (e *Editor) GotoLoop() (LoopReport, error) {
	r := LoopReport{Held: map[string]int{}}
	var loops []*glLoop
	for _, fd := range e.gfFunctions() {
		fl := gfFlowOf(fd)
		if fl.odd {
			r.Odd++
			continue
		}
		names := make([]string, 0, len(fl.labels))
		for name := range fl.labels {
			names = append(names, name)
		}
		sort.Strings(names)
		var taken [][2]int
		for _, name := range names {
			l, why := glLoopOf(fl, name)
			if l == nil {
				if why != "" {
					r.Held[why] += fl.gotos[name]
				}
				continue
			}
			over := false
			for _, t := range taken {
				over = over || l.a < t[1] && t[0] < l.z
			}
			if over {
				r.Held["its region overlaps another's"] += fl.gotos[name]
				continue
			}
			taken = append(taken, [2]int{l.a, l.z})
			loops = append(loops, l)
			r.Gotos += fl.gotos[name]
			if l.cond != nil {
				r.Dos++
			} else {
				r.Fors++
			}
		}
	}
	for _, l := range loops {
		if err := e.glRewrite(l); err != nil {
			return r, err
		}
	}
	return r, nil
}

// glRewrite makes one loop: the gotos continue, the region's items moved
// into the loop's block, the label gone.
func (e *Editor) glRewrite(l *glLoop) error {
	first, last := l.run[0], l.run[len(l.run)-1]
	if l.cond != nil {
		body := append([]*Node{NewAtom("block")}, l.run[1:len(l.run)-1]...)
		do := NewList(NewAtom("do"), NewList(body...), l.cond)
		return e.ReplaceRun(first, last, do)
	}
	for _, g := range l.gotos {
		c := NewList(NewAtom("continue"))
		if g == last {
			last = c
		}
		if err := e.Replace(g, c); err != nil {
			return err
		}
	}
	p, lo := e.index(first)
	_, hi := e.index(last)
	if p == nil || hi < lo {
		return fmt.Errorf("gotoloop: the region of %s is not where it was", labelName(first))
	}
	body := append([]*Node{NewAtom("block")}, e.kids(p)[lo+1:hi+1]...)
	if l.tail {
		body = append(body, Break())
	}
	empty := func() *Node { return NewList() }
	loop := NewList(NewAtom("for"), empty(), empty(), empty(), NewList(body...))
	return e.ReplaceRun(first, last, loop)
}

// glInRegion says the site's statement is under items k..i of block b.
func glInRegion(st []gfFrame, depth int, b *Node, k, i int) bool {
	return len(st) > depth && st[depth].kind == gfBlock && st[depth].node == b && k <= st[depth].idx && st[depth].idx <= i
}

// glLoopOf is the loop label name's backward gotos make, or nil and why
// not ("" for a label that is not the head of a backward jump at all).
func glLoopOf(fl *gfFlow, name string) (*glLoop, string) {
	lab := fl.labels[name]
	la := fl.ord.pos[lab.n]
	var gs []gfSite
	back := false
	for _, j := range fl.jumps {
		if j.n.Is("goto") && labelName(j.n) == name {
			gs = append(gs, j)
			back = back || fl.ord.pos[j.n] > la
		}
	}
	if !back {
		return nil, ""
	}
	for _, g := range gs {
		if fl.ord.pos[g.n] < la {
			return nil, "a goto comes from before its label"
		}
	}
	d := len(lab.stack) - 1
	fr := lab.stack[d]
	if labeledAt(fr.items, fr.idx) || fr.idx > 0 && fr.items[fr.idx-1].Is("stmt-attr") {
		return nil, "the label is not a block item"
	}
	if len(gfLabels(fr.items, fr.idx)) != 1 {
		return nil, "another label marks the statement"
	}
	b, items, k := fr.node, fr.items, fr.idx
	s := gfGroupStmt(items, k) // the label's statement: cc's item k ends there
	i := k
	for _, g := range gs {
		if !glInRegion(g.stack, d, b, k, len(items)-1) {
			return nil, "a goto is not in the label's block"
		}
		if gfBreakable(g.stack) > d {
			return nil, "a loop or switch between goto and label"
		}
		i = max(i, g.stack[d].idx)
	}
	// nothing in the region leaves it but by return or goto, and nothing
	// enters it but at the top
	for _, j := range fl.jumps {
		if j.n.Is("goto") || !glInRegion(j.stack, d, b, k, i) {
			continue
		}
		inner := false
		for _, f := range j.stack[d+1:] {
			inner = inner || f.kind == gfLoop || f.kind == gfSwitch && j.n.Is("break")
		}
		if !inner {
			return nil, "a break or continue leaves the region"
		}
	}
	for other, l := range fl.labels {
		if other == name || !glInRegion(l.stack, d, b, k, i) {
			continue
		}
		for _, j := range fl.jumps {
			if j.n.Is("goto") && labelName(j.n) == other && !glInRegion(j.stack, d, b, k, i) {
				return nil, "a goto enters the region"
			}
		}
	}
	for _, c := range fl.cases {
		if glInRegion(c.stack, d, b, k, i) && gfBreakable(c.stack) <= d {
			return nil, "a case label of a switch around the region is in it"
		}
	}
	// a name the region declares at its level, named after it
	var declared []string
	for _, it := range items[k : i+1] {
		if isDeclItem(it) {
			declared = append(declared, gfDeclared(it)...)
		}
	}
	if gfNames(items[i+1:], declared) {
		return nil, "a declaration in the region is named after it"
	}
	l := &glLoop{label: lab.n, run: items[k : i+1], a: la, z: fl.ord.end[items[i]]}
	for _, g := range gs {
		l.gotos = append(l.gotos, g.n)
	}
	// the do form: the one goto is item i, `if (c) goto L;`
	if len(gs) == 1 && i > s && !labeledAt(items, i) {
		if c := glGuard(items[i], gs[0].n); c != nil && !gfNames([]*Node{c}, declared) {
			l.cond = c
			return l, ""
		}
	}
	if i == s {
		// the label's own statement: the label goes, so it terminates as
		// it stands, unless a case labels it too
		l.tail = s != k+1 || !StmtTerminates(items[i])
	} else {
		l.tail = !ItemTerminates(items, i)
	}
	return l, ""
}

// glGuard is c when the item is `if (c) goto L;` with no else, the goto
// being g; else nil.
func glGuard(it, g *Node) *Node {
	if !it.Is("if") || len(it.Kids) != 3 {
		return nil
	}
	b := blockItems(it.Kids[2])
	if len(b) != 1 || b[0] != g {
		return nil
	}
	return it.Kids[1]
}
