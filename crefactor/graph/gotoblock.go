package graph

import (
	"fmt"
	"sort"
	"strings"
)

// GOTOBLOCK (doc/GRAPH-MIGRATION.md, B3g): crefactor/xform's GotoBlock on
// the nodes, with MOVE: the region's items go into the do-while keeping
// their ids.
//
// The forward exits: every `goto L;` whose label L is a statement of a block
// B that encloses the goto, after it, with nothing but if/else and blocks
// between the goto and B.  The region of B from the first item holding such
// a goto up to L's item is wrapped in `do { region } while (0);` and each
// goto is `break;`.  The label goes.  A label is taken whole or not at all,
// and held when a goto to it comes after it or from outside its block; a
// loop or switch lies between a goto and B; the region holds a break or
// continue of its own that leaves it, or a case label whose switch is
// outside it; the region declares a name spelled at or after the label; a
// goto from outside the region jumps to a label inside it; the function
// takes a label's address or jumps to a computed one.  Two labels whose
// rewrites would overlap are not rewritten in one round: the later one
// waits for the next, which judges it against the first's do-while.

// The reasons a label is held, in the order they are reported.
const (
	gbHoldBackward = "a goto after it or outside its block"
	gbHoldLoop     = "a loop or switch between a goto and the label's block"
	gbHoldStray    = "a break, continue or case label of the region's own"
	gbHoldDecl     = "a declaration in the region spelled after the label"
	gbHoldInto     = "a jump into the region"
	gbHoldComputed = "a computed goto in the function"
	gbHoldNested   = "a label inside a statement, not one of its block's"
)

var gbHoldOrder = []string{gbHoldComputed, gbHoldNested, gbHoldBackward, gbHoldLoop, gbHoldStray, gbHoldDecl, gbHoldInto}

// BlockReport is what one run of GotoBlock did.
type BlockReport struct {
	Gotos, Labels, Rounds int
	Held                  map[string][2]int // reason -> labels, gotos: the last round's
}

// Line is the report as the text step wrote it.
func (r BlockReport) Line() string {
	var why []string
	for _, h := range gbHoldOrder {
		if n, ok := r.Held[h]; ok {
			why = append(why, fmt.Sprintf("%d (%d gotos) for %s", n[0], n[1], h))
		}
	}
	if len(why) == 0 {
		why = []string{"none"}
	}
	return fmt.Sprintf("%d gotos become a break out of a do-while(0), and their %d labels go, in %d rounds that rewrote; labels held: %s",
		r.Gotos, r.Labels, r.Rounds, strings.Join(why, "; "))
}

// A gbRewrite is one label's: the region, the gotos, the label.
type gbRewrite struct {
	region   []*Node // items i0 .. k-1 of the block, k the label's group
	gotos    []*Node
	label    *Node
	empty    *Node // the label's `;`, which goes with it
	from, to int   // the extent it touches
}

// GotoBlock wraps each forward exit's region in a do-while(0), rounds
// until one rewrites nothing.
func (e *Editor) GotoBlock() (BlockReport, error) {
	var r BlockReport
	for {
		r.Held = map[string][2]int{}
		var todo []gbRewrite
		for _, fd := range e.gfFunctions() {
			var taken [][2]int
			for _, w := range e.gbFunction(fd, r.Held) {
				over := false
				for _, t := range taken {
					over = over || w.from < t[1] && t[0] < w.to
				}
				if over {
					continue
				}
				taken = append(taken, [2]int{w.from, w.to})
				todo = append(todo, w)
			}
		}
		if len(todo) == 0 {
			return r, nil
		}
		for _, w := range todo {
			if err := e.gbRewrite(w); err != nil {
				return r, err
			}
			r.Gotos += len(w.gotos)
			r.Labels++
		}
		r.Rounds++
	}
}

func (e *Editor) gbRewrite(w gbRewrite) error {
	first, last := w.region[0], w.region[len(w.region)-1]
	for _, g := range w.gotos {
		b := Break()
		switch g {
		case first:
			first = b
		}
		if g == last {
			last = b
		}
		if err := e.Replace(g, b); err != nil {
			return err
		}
	}
	if err := e.Delete(w.label); err != nil {
		return err
	}
	if w.empty != nil {
		if err := e.Delete(w.empty); err != nil {
			return err
		}
	}
	p, lo := e.index(first)
	_, hi := e.index(last)
	if p == nil || hi < lo {
		return fmt.Errorf("gotoblock: the region of %s is not where it was", labelName(w.label))
	}
	body := append([]*Node{NewAtom("block")}, e.kids(p)[lo:hi+1]...)
	return e.ReplaceRun(first, last, NewList(NewAtom("do"), NewList(body...), Literal(0)))
}

// A gbFrame is one statement on the way down from a function's body: a
// block's item (its group's first), a loop, or a switch.
type gbFrame struct {
	kind  byte // 'b' a block's item, 'l' a loop, 's' a switch
	block *Node
	item  int
}

type gbSite struct {
	n     *Node
	stack []gbFrame
}

// gbFunction finds the labels of one function that can be taken, and
// counts the ones held into held.
func (e *Editor) gbFunction(fd *Node, held map[string][2]int) []gbRewrite {
	ord := gfOrderOf(fd)
	var jumps []gbSite
	lbls := map[string]gbSite{}
	computed := false
	walkBody(fd, func(x *Node) bool {
		switch {
		case !x.list:
			return false
		case x.Is("label-addr"), x.Is("goto*"), x.Is("defn"):
			computed = true
		case x.Is("verbatim"):
			for _, k := range x.Kids[1:] {
				computed = computed || strings.Contains(k.Atom, "__label__")
			}
		}
		return true
	})
	items := map[*Node][]*Node{} // each block's items as walked
	var walk func(s *Node, stack []gbFrame)
	block := func(owner *Node, its []*Node, stack []gbFrame) {
		its = append([]*Node(nil), its...)
		items[owner] = its
		for i, it := range its {
			st := append(append([]gbFrame{}, stack...), gbFrame{kind: 'b', block: owner, item: gfGroupStart(its, i)})
			switch {
			case it.Is("label"):
				lbls[labelName(it)] = gbSite{it, st}
			case gfLabelItem(it), isDeclItem(it):
			default:
				walk(it, st)
			}
		}
	}
	walk = func(s *Node, stack []gbFrame) {
		if !s.list {
			return
		}
		switch s.Head() {
		case "block":
			block(s, blockItems(s), stack)
		case "if":
			walk(s.Kids[2], stack)
			if len(s.Kids) > 3 {
				walk(s.Kids[3], stack)
			}
		case "switch":
			walk(s.Kids[2], append(append([]gbFrame{}, stack...), gbFrame{kind: 's'}))
		case "while", "do", "for":
			b := s.Kids[2]
			switch s.Head() {
			case "do":
				b = s.Kids[1]
			case "for":
				b = s.Kids[4]
			}
			walk(b, append(append([]gbFrame{}, stack...), gbFrame{kind: 'l'}))
		case "goto":
			jumps = append(jumps, gbSite{s, stack})
		}
	}
	block(fd, Body(fd), nil)

	byLabel := map[string][]gbSite{}
	for _, j := range jumps {
		byLabel[labelName(j.n)] = append(byLabel[labelName(j.n)], j)
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
			hold(gbHoldComputed, len(js))
			continue
		}
		lb := L.stack[len(L.stack)-1]
		B, k := lb.block, lb.item
		its := items[B]
		i0 := k
		why := ""
		for _, j := range js {
			at := -1
			for d, f := range j.stack {
				if f.kind == 'b' && f.block == B {
					at = d
				}
			}
			if at < 0 || j.stack[at].item >= k {
				why = gbHoldBackward
				break
			}
			for _, f := range j.stack[at+1:] {
				if f.kind != 'b' {
					why = gbHoldLoop
				}
			}
			i0 = min(i0, j.stack[at].item)
		}
		if why == "" && gbStray(its[i0:k]) {
			why = gbHoldStray
		}
		if why == "" {
			var declared []string
			for _, it := range its[i0:k] {
				if isDeclItem(it) {
					declared = append(declared, gfDeclared(it)...)
				}
			}
			if gfNames(its[k:], declared) {
				why = gbHoldDecl
			}
		}
		if why == "" {
			// a label inside the region reached by a goto outside it
			in := func(stack []gbFrame) bool {
				for _, f := range stack {
					if f.kind == 'b' && f.block == B && f.item >= i0 && f.item < k {
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
						why = gbHoldInto
					}
				}
			}
		}
		if why != "" {
			hold(why, len(js))
			continue
		}
		w := gbRewrite{region: its[i0:k], label: L.n}
		for _, j := range js {
			w.gotos = append(w.gotos, j.n)
		}
		at := 0
		for at < len(its) && its[at] != L.n {
			at++
		}
		s := gfGroupStmt(its, k)
		if at == k && at+1 < len(its) && its[at+1].Is("empty") {
			w.empty = its[at+1] // `L: ;` alone: the empty statement was there for the label
		}
		w.from = ord.pos[its[i0]]
		w.to = ord.end[its[min(s, len(its)-1)]] + 1
		out = append(out, w)
	}
	return out
}

// gbStray says the items hold a break or continue that leaves them, or a
// case or default label whose switch is not among them.
func gbStray(items []*Node) bool {
	stray := false
	var walk func(s *Node, loops, switches int)
	walk = func(s *Node, loops, switches int) {
		if stray || !s.list {
			return
		}
		switch h := s.Head(); {
		case h == "block":
			for _, it := range blockItems(s) {
				walk(it, loops, switches)
			}
		case isCaseLabel(s):
			stray = stray || switches == 0
		case h == "if":
			walk(s.Kids[2], loops, switches)
			if len(s.Kids) > 3 {
				walk(s.Kids[3], loops, switches)
			}
		case h == "switch":
			walk(s.Kids[2], loops, switches+1)
		case h == "while":
			walk(s.Kids[2], loops+1, switches)
		case h == "do":
			walk(s.Kids[1], loops+1, switches)
		case h == "for":
			walk(s.Kids[4], loops+1, switches)
		case h == "break":
			stray = stray || loops+switches == 0
		case h == "continue":
			stray = stray || loops == 0
		}
	}
	for _, it := range items {
		walk(it, 0, 0)
	}
	return stray
}
