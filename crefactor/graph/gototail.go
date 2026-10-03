package graph

import (
	"fmt"
	"sort"
	"strings"
)

// GOTOTAIL (doc/GRAPH-MIGRATION.md, B3g): crefactor/xform's GotoTail on
// the nodes, with CLONE for the copies.
//
// A goto whose label marks a short tail -- at most n expression statements
// after the label, each an item of the label's block, and then `return x;`,
// or in a function returning void the end of its body -- is that tail: its
// statements copied over the goto (Clone: the same declarations, so the
// copy does at the goto what the jump led to), and the return.  Empty
// statements are not counted and not copied.  A goto is held, and so is its
// label, when the copy could mean something else or could not be written
// twice: a name the tail reads is declared in an inner block of the
// function, or at its top after the goto or after the label (a typedef
// name counts); or the tail holds a label, a case, a declaration or a
// statement expression.  A tail holding any other statement, or more than
// n, is no tail: its gotos stay, counted apart.
//
// A label no goto reaches any more goes, unless its address is taken.  When
// the statement before it always jumps, nothing reaches its tail either, and
// the tail goes with it; a label on an empty statement goes with the
// statement.

// TailReport is what one run of GotoTail did.
type TailReport struct {
	Tail   int            // the most statements a tail may hold
	Taken  int            // gotos rewritten
	Held   map[string]int // gotos held, by reason
	Other  int            // gotos to a label that marks no short tail
	Labels int            // labels that went
	Dead   int            // ... of them with a tail nothing else reached
	Kept   int            // labels no goto reaches, kept for their address
	Freed  []string       // functions left with no goto
	Before int            // functions with a goto before
}

// Lines is the report as the text step wrote it.
func (r TailReport) Lines() []string {
	var why []string
	for _, w := range []string{"a name", "a label", "a case", "a declaration", "a statement expression"} {
		if r.Held[w] > 0 {
			why = append(why, fmt.Sprintf("%d for %s", r.Held[w], w))
		}
	}
	held := 0
	for _, n := range r.Held {
		held += n
	}
	hs := fmt.Sprintf("%d held", held)
	if len(why) > 0 {
		hs += " (" + strings.Join(why, ", ") + ")"
	}
	return []string{
		fmt.Sprintf("%d gotos to a tail of at most %d statements and a return take the tail; %s; %d to a label that marks no such tail stay",
			r.Taken, r.Tail, hs, r.Other),
		fmt.Sprintf("%d labels go, %d with a tail nothing else reaches; %d that no goto reaches stay, for their address is taken", r.Labels, r.Dead, r.Kept),
		fmt.Sprintf("%d of the %d functions with a goto are left with none: %s", len(r.Freed), r.Before, strings.Join(r.Freed, " ")),
	}
}

// A gtTail is what a label marks: the statements from the label's own to
// the return, and why a goto to it is held when every goto to it is.
type gtTail struct {
	label *Node
	parts []*Node // the statements copied, empty ones left out; nil: the void function's `return;`
	first *Node   // the label's statement
	last  *Node   // the tail's last item
	items []*Node // the label's block's items
	at    int     // the label's index there
	why   string
	used  int
	stay  int
}

// GotoTail copies each short tail over the gotos to it, n the most
// statements a tail may hold before its return.
func (e *Editor) GotoTail(n int) (TailReport, error) {
	r := TailReport{Tail: n, Held: map[string]int{}}
	type copyOver struct {
		jump *Node
		with []*Node
	}
	var copies []copyOver
	var gone [][2]*Node // runs of items that go: a label, or a label and its tail
	for _, fd := range e.gfFunctions() {
		ord := gfOrderOf(fd)
		void := gtReturnsVoid(fd)
		// the function's names: at its top, where; in an inner block, at all
		top := map[string]int{}
		inner := map[string]bool{}
		for _, it := range Body(fd) {
			if isDeclItem(it) {
				for _, d := range gfDeclarators(it) {
					if !d.enum {
						top[d.name] = ord.pos[d.n]
					}
				}
			}
		}
		for _, it := range Body(fd) {
			for _, d := range gfDeclarators(it) {
				if at, ok := top[d.name]; d.enum || !ok || at != ord.pos[d.n] {
					inner[d.name] = true
				}
			}
		}
		// the labels that mark a tail, every goto, the labels whose address
		// is taken
		tails := map[string]*gtTail{}
		var jumps []*Node
		gotos := 0
		addr := map[string]bool{}
		walkBody(fd, func(x *Node) bool {
			if !x.list {
				return false
			}
			switch x.Head() {
			case "goto":
				jumps = append(jumps, x)
				gotos++
			case "goto*":
				gotos++
			case "label-addr":
				addr[labelName(x)] = true
			}
			return true
		})
		itemLists(fd, func(items []*Node) {
			items = append([]*Node(nil), items...)
			for i := range items {
				if t := gtTailAt(items, i, fd, void, n); t != nil {
					tails[labelName(t.label)] = t
				}
			}
		})
		if gotos > 0 {
			r.Before++
		}
		left := gotos
		for _, j := range jumps {
			t := tails[labelName(j)]
			if t == nil {
				r.Other++
				continue
			}
			t.used++
			why := t.why
			if why == "" && !e.gtNamesAgree(t, j, ord, top, inner) {
				why = "a name"
			}
			if why != "" {
				t.stay++
				r.Held[why]++
				continue
			}
			var with []*Node
			for _, p := range t.parts {
				if p == nil {
					with = append(with, Return(nil))
					continue
				}
				with = append(with, Clone(p))
			}
			if len(with) > 1 && e.gtLabeled(j) {
				// cc's `case 1: goto L;` is one labeled statement, whose
				// copy is a block
				with = []*Node{NewList(append([]*Node{NewAtom("block")}, with...)...)}
			}
			copies = append(copies, copyOver{j, with})
			r.Taken++
			left--
		}
		if gotos > 0 && left == 0 {
			r.Freed = append(r.Freed, declName(fd))
		}
		names := make([]string, 0, len(tails))
		for name := range tails {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			t := tails[name]
			if t.used == 0 || t.stay > 0 {
				continue
			}
			if addr[name] {
				r.Kept++
				continue
			}
			r.Labels++
			switch {
			case ItemTerminates(t.items, t.at-1):
				// nothing but the gotos reached it: it goes whole
				gone = append(gone, [2]*Node{t.label, t.last})
				r.Dead++
			case t.first.Is("empty"):
				gone = append(gone, [2]*Node{t.label, t.first})
			default:
				gone = append(gone, [2]*Node{t.label, t.label})
			}
		}
	}
	// the copies first, from the tails as they stand; then what goes
	for _, c := range copies {
		if err := e.Replace(c.jump, c.with...); err != nil {
			return r, err
		}
	}
	for _, g := range gone {
		if err := e.ReplaceRun(g[0], g[1]); err != nil {
			return r, err
		}
	}
	sort.Strings(r.Freed)
	return r, nil
}

// gtLabeled says the item j stands after a label: cc's labeled statement.
func (e *Editor) gtLabeled(j *Node) bool {
	p, i := e.index(j)
	return p != nil && labeledAt(e.kids(p), i)
}

// gtTailAt is the tail items[i] marks when it is a goto label with nothing
// labelling its group before it.
func gtTailAt(items []*Node, i int, fd *Node, void bool, n int) *gtTail {
	if !items[i].Is("label") || i > 0 && gfLabelItem(items[i-1]) {
		return nil
	}
	if i+1 >= len(items) {
		return nil // C23: a label at the end of a block
	}
	t := &gtTail{label: items[i], first: items[i+1], items: items, at: i}
	hold := func(why string) {
		if t.why == "" {
			t.why = why
		}
	}
	count := 0
	end := false
	j := i + 1
	for ; j < len(items); j++ {
		s := items[j]
		t.last = s
		switch {
		case s.Is("label"):
			hold("a label")
			continue
		case isCaseLabel(s):
			hold("a case")
			continue
		case isDeclItem(s):
			hold("a declaration")
			continue
		case s.Is("return"):
			t.parts = append(t.parts, s)
			gtInside(s, hold)
			end = true
		case s.Is("empty"), s.Is("attributed"):
			continue // cc's `;` with or without attributes: not counted, not copied
		case !IsStatement(s):
			count++
			t.parts = append(t.parts, s)
			gtInside(s, hold)
			if count > n {
				return nil
			}
			continue
		default:
			return nil
		}
		break
	}
	if !end {
		// the block's end: a void function's body ends in `return;`
		if !void || !gtIsBody(fd, items) {
			return nil
		}
		t.parts = append(t.parts, nil)
	}
	return t
}

// gtIsBody says items are the function's own body's.
func gtIsBody(fd *Node, items []*Node) bool {
	b := Body(fd)
	return len(b) == len(items) && len(b) > 0 && b[0] == items[0] && b[len(b)-1] == items[len(items)-1]
}

// gtInside holds a tail whose statement holds a statement expression.
func gtInside(n *Node, hold func(string)) {
	Walk(n, func(x *Node) bool {
		if x.Is("stmt-expr") {
			hold("a statement expression")
			return false
		}
		return true
	})
}

// gtReturnsVoid says the function returns void: its type's result is
// `void`, and its declarator is the name and its parameters.
func gtReturnsVoid(fd *Node) bool {
	t := defType(fd)
	return t != nil && t.Is("fn") && len(t.Kids) > 2 && !t.Kids[2].list && t.Kids[2].Atom == "void"
}

// gtNamesAgree says every name the tail reads is the same object at the
// goto as at the label: no inner block declares it, and a declaration at
// the function's top comes before both.  A typedef name is a name.
func (e *Editor) gtNamesAgree(t *gtTail, jump *Node, ord gfOrder, top map[string]int, inner map[string]bool) bool {
	first := min(ord.pos[jump], ord.pos[t.label])
	ok := true
	for _, part := range t.parts {
		if part == nil {
			continue
		}
		Walk(part, func(x *Node) bool {
			if !ok {
				return false
			}
			if x.list {
				return true
			}
			d := x.Ref()
			if d == nil || !gtOrdinary(e, d) {
				return true
			}
			name := x.Atom
			if inner[name] {
				ok = false
			}
			if at, is := top[name]; is && at >= first {
				ok = false
			}
			return ok
		})
	}
	return ok
}

// gtOrdinary says a use's declaration is an object, a function, a
// parameter, an enumerator or a typedef: what cc reads as an identifier
// or a typedef name, not a member, a tag or a label.
func gtOrdinary(e *Editor, d *Node) bool {
	switch d.Head() {
	case "struct", "union", "enum", "label", "member":
		return false
	}
	return !isMember(e, d)
}
