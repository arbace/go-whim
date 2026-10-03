package graph

import (
	"slices"
	"strings"
)

// THE GOTOS ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3g): crefactor/xform's
// goto transforms -- GotoTail, GotoBreak, GotoLoop, GotoBlock -- asked of
// the nodes.  They read a function as its statements: every jump and every
// label with the chain of statements that holds it.  This file is that
// walk, the one the four share.
//
// cc's block item `L: x;` is ONE labeled statement; in C-lisp it is two
// items, `(label L) x`.  So where the text's rules speak of an item, these
// speak of a GROUP: the label items in a row (a goto's, a switch's, the
// attributes before one) and the statement after them.  A frame's index is
// C-lisp's; a rule that compares items compares groups.

// gfKind says how a statement is held by the one around it.
type gfKind int

const (
	gfBlock  gfKind = iota // an item of a block or of a function's body
	gfThen                 // an if's then-branch
	gfElse                 // an if's else-branch
	gfLoop                 // a while's, do's or for's body
	gfSwitch               // a switch's body
)

// A gfFrame is one statement on the way down from a function's body.
type gfFrame struct {
	kind  gfKind
	node  *Node   // the block (or the function), if, loop or switch
	items []*Node // gfBlock: the items, as they were when walked
	idx   int     // gfBlock: the item's index
}

// A gfSite is a jump, a label or a case found by the walk, and the frames
// that hold it, outermost first; a label's or a case's own item's frame is
// its last.
type gfSite struct {
	n     *Node
	stack []gfFrame
}

// A gfFlow is one function as the walk found it.
type gfFlow struct {
	fn     *Node
	jumps  []gfSite          // every goto, break and continue the statements reach
	labels map[string]gfSite // every label
	cases  []gfSite          // every case and default label
	gotos  map[string]int    // the gotos to each label, wherever they stand
	// odd is set when the function does what the walk does not model: a
	// computed goto, a label's address, a local label, a nested function,
	// or a jump or label inside an expression.  Such a function is left
	// alone.
	odd bool
	ord gfOrder
}

// gfOrder numbers a function's nodes in the file's order: pos where a node
// begins, end the last number inside it.  Where the text compared offsets,
// these compare positions.
type gfOrder struct{ pos, end map[*Node]int }

func gfOrderOf(n *Node) gfOrder {
	o := gfOrder{pos: map[*Node]int{}, end: map[*Node]int{}}
	k := 0
	var walk func(x *Node)
	walk = func(x *Node) {
		o.pos[x] = k
		k++
		for _, c := range x.Kids {
			walk(c)
		}
		o.end[x] = k - 1
	}
	walk(n)
	return o
}

// gfLabelItem says x is a label item: a goto's label, a case, a default, or
// the attributes before one.
func gfLabelItem(x *Node) bool { return isLabelItem(x) || x.Is("stmt-attr") }

// gfGroupStart is the first item of the group items[i] is in: the label
// items in a row before it.
func gfGroupStart(items []*Node, i int) int {
	for i > 0 && gfLabelItem(items[i-1]) {
		i--
	}
	return i
}

// gfGroupStmt is the statement of the group whose first item is items[i]:
// the first item from i on that is no label.
func gfGroupStmt(items []*Node, i int) int {
	for i < len(items) && gfLabelItem(items[i]) {
		i++
	}
	return i
}

// gfLabels are the goto labels that mark the group items[i] begins, in a
// row from it: cc's chain of ordinary labels, which a case ends.
func gfLabels(items []*Node, i int) []string {
	var out []string
	for ; i < len(items) && items[i].Is("label"); i++ {
		out = append(out, labelName(items[i]))
	}
	return out
}

func gfPush(st []gfFrame, f gfFrame) []gfFrame {
	n := make([]gfFrame, len(st), len(st)+1)
	copy(n, st)
	return append(n, f)
}

// gfFlowOf walks the function fn.
func gfFlowOf(fn *Node) *gfFlow {
	f := &gfFlow{fn: fn, labels: map[string]gfSite{}, gotos: map[string]int{}, ord: gfOrderOf(fn)}
	seen := map[*Node]bool{}
	var stmt func(s *Node, st []gfFrame)
	block := func(owner *Node, items []*Node, st []gfFrame) {
		items = slices.Clone(items)
		for i, it := range items {
			fr := gfPush(st, gfFrame{kind: gfBlock, node: owner, items: items, idx: i})
			switch {
			case it.Is("label"):
				seen[it] = true
				f.labels[labelName(it)] = gfSite{n: it, stack: fr}
			case isCaseLabel(it):
				seen[it] = true
				f.cases = append(f.cases, gfSite{n: it, stack: fr})
			case isDeclItem(it):
			default:
				stmt(it, fr)
			}
		}
	}
	stmt = func(s *Node, st []gfFrame) {
		if s == nil || !s.list {
			return
		}
		switch s.Head() {
		case "block":
			block(s, blockItems(s), st)
		case "if":
			stmt(s.Kids[2], gfPush(st, gfFrame{kind: gfThen, node: s}))
			if len(s.Kids) > 3 {
				stmt(s.Kids[3], gfPush(st, gfFrame{kind: gfElse, node: s}))
			}
		case "switch":
			stmt(s.Kids[2], gfPush(st, gfFrame{kind: gfSwitch, node: s}))
		case "while":
			stmt(s.Kids[2], gfPush(st, gfFrame{kind: gfLoop, node: s}))
		case "do":
			stmt(s.Kids[1], gfPush(st, gfFrame{kind: gfLoop, node: s}))
		case "for":
			stmt(s.Kids[4], gfPush(st, gfFrame{kind: gfLoop, node: s}))
		case "goto", "break", "continue":
			seen[s] = true
			f.jumps = append(f.jumps, gfSite{n: s, stack: st})
		case "return", "goto*":
			seen[s] = true
		}
	}
	block(fn, Body(fn), nil)
	walkBody(fn, func(x *Node) bool {
		if !x.list {
			return false
		}
		switch h := x.Head(); {
		case h == "goto":
			f.gotos[labelName(x)]++
			f.odd = f.odd || !seen[x]
		case h == "goto*" || h == "label-addr" || h == "defn":
			f.odd = true
		case jumps(x) || isLabelItem(x):
			f.odd = f.odd || !seen[x]
		case h == "verbatim":
			for _, k := range x.Kids[1:] {
				f.odd = f.odd || strings.Contains(k.Atom, "__label__")
			}
		}
		return true
	})
	return f
}

// gfNext is where control goes when the statement held by st runs off its
// end: the item after it in its block, or after the block, the if around
// it, or after a switch whose body ends there.  It is not found when that
// is a loop's test, or the function's end.
func gfNext(st []gfFrame) (gfFrame, bool) {
	for j := len(st) - 1; j >= 0; j-- {
		f := st[j]
		switch f.kind {
		case gfBlock:
			if f.idx+1 < len(f.items) {
				f.idx++
				return f, true
			}
		case gfLoop:
			return gfFrame{}, false
		}
	}
	return gfFrame{}, false
}

// gfMarks says the label named marks the group the frame's item begins.
func gfMarks(f gfFrame, name string) bool {
	return slices.Contains(gfLabels(f.items, f.idx), name)
}

// gfBreakable is the index in st of the innermost loop or switch, or -1.
func gfBreakable(st []gfFrame) int {
	for j := len(st) - 1; j >= 0; j-- {
		if st[j].kind == gfLoop || st[j].kind == gfSwitch {
			return j
		}
	}
	return -1
}

// gfFunctions are the file's function definitions, in order.
func (e *Editor) gfFunctions() []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		if f.Is("defn") {
			out = append(out, f)
		}
	}
	return out
}

// gfNames says a name of names is spelled in one of the nodes as a whole
// word: an atom of that spelling, or a literal or a macro's text that holds
// it -- the text's `\bname\b`, which a member, a string or a macro of the
// spelling satisfies too.
func gfNames(ns []*Node, names []string) bool {
	if len(names) == 0 {
		return false
	}
	found := false
	for _, n := range ns {
		Walk(n, func(x *Node) bool {
			if found {
				return false
			}
			if !x.list {
				for _, name := range names {
					if x.Atom == name || strings.ContainsAny(x.Atom, "\"' ") && wordIn(x.Atom, name) {
						found = true
					}
				}
			}
			return !found
		})
	}
	return found
}

// A gfDecl is a name a declaration declares, and the node that declares
// it: a def or a typedef, a named parameter of a function type, a named
// member of a struct or union, an enumerator.
type gfDecl struct {
	name string
	n    *Node
	enum bool
}

// gfDeclarators are the names declared under n -- every declarator cc's
// walk finds there, and the enumerators and tags it defines.
func gfDeclarators(n *Node) []gfDecl {
	var out []gfDecl
	Walk(n, func(x *Node) bool {
		if !x.list {
			return false
		}
		switch x.Head() {
		case "def", "typedef":
			if name := declName(x); name != "" {
				out = append(out, gfDecl{name: name, n: x})
			}
		case "fn":
			if len(x.Kids) > 1 && x.Kids[1].list {
				for _, p := range x.Kids[1].Kids {
					if p.list && len(p.Kids) > 1 && !p.Kids[0].list && isIdent(p.Kids[0].Atom) {
						out = append(out, gfDecl{name: p.Kids[0].Atom, n: p})
					}
				}
			}
		case "struct", "union", "enum":
			if t := tagOf(x); t != "" && isIdent(t) {
				out = append(out, gfDecl{name: t, n: x})
			}
			for _, m := range body(x) {
				if m.list && len(m.Kids) > 0 && !m.Kids[0].list && isIdent(m.Kids[0].Atom) && (x.Is("enum") || len(m.Kids) > 1) {
					out = append(out, gfDecl{name: m.Kids[0].Atom, n: m, enum: x.Is("enum")})
				}
			}
		}
		return true
	})
	return out
}

// gfDeclared are the names a declaration item declares.
func gfDeclared(it *Node) []string {
	var out []string
	for _, d := range gfDeclarators(it) {
		out = append(out, d.name)
	}
	return out
}

// gfSame says two expressions are the same C: the same forms and atoms.
func gfSame(a, b *Node) bool {
	if a.list != b.list || a.Atom != b.Atom || len(a.Kids) != len(b.Kids) {
		return false
	}
	for i := range a.Kids {
		if !gfSame(a.Kids[i], b.Kids[i]) {
			return false
		}
	}
	return true
}
