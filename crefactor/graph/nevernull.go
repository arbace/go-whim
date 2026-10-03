package graph

import "fmt"

// NEVERNULL (doc/GRAPH-MIGRATION.md, B3g): crefactor/xform's NeverNull on
// the nodes.  Every NULL test of a never-NULL function's result directly
// after the call folds: `== nullptr` is never true, `!= nullptr` always is.
// The never-NULL functions are found to a fixed point from the roots: a
// function returning a pointer whose every return is a never-NULL call,
// or a local assigned only never-NULL calls.  Folding one test can make
// another function never-NULL, so the set is found again until nothing
// changes; a label no goto reaches any more goes.

// NeverNullOptions are what NeverNull is told.
type NeverNullOptions struct {
	// In says which top-level forms it folds in (the code base's core).
	In func(form *Node) bool
	// Roots are the functions that never return NULL by the code base's
	// own argument: an allocator that ends the process rather than fail.
	Roots []string
}

// NeverNullReport is what NeverNull did.
type NeverNullReport struct {
	Folded int      // the tests that folded
	Never  int      // the functions that never return NULL, at the end
	Left   []string // the tests it had to leave, and why
}

// Lines is the report as the text step wrote it.
func (r NeverNullReport) Lines() []string {
	out := []string{fmt.Sprintf("%d NULL tests of a never-NULL function's result fold; %d functions never return NULL", r.Folded, r.Never)}
	for _, l := range r.Left {
		out = append(out, "left: "+l)
	}
	return out
}

// NeverNull folds the tests, to the fixed point.
func (e *Editor) NeverNull(o NeverNullOptions) (NeverNullReport, error) {
	var r NeverNullReport
	seen := map[string]bool{}
	for {
		nn := e.nnSet(o)
		changed := false
		// `if ((v = f(...)) == nullptr)` is `v = f(...);` and the test
		for {
			hit := false
			for _, s := range e.nnItems(o, func(s *Node) bool {
				if !s.Is("if") || !s.Kids[1].Is("==") || len(s.Kids[1].Kids) != 3 || !nnNull(s.Kids[1].Kids[2]) {
					return false
				}
				a := s.Kids[1].Kids[1]
				return a.Is("=") && len(a.Kids) == 3 && nnPath(a.Kids[1]) && nnCall(a.Kids[2], nn)
			}) {
				if p, i := e.index(s); p == nil || e.place(p, i) != placeItem {
					continue
				}
				a := s.Kids[1].Kids[1]
				test := NewList(NewAtom("=="), Clone(a.Kids[1]), s.Kids[1].Kids[2])
				arms := append([]*Node{NewAtom("if"), test}, s.Kids[2:]...)
				if err := e.Replace(s, a, NewList(arms...)); err != nil {
					return r, err
				}
				hit = true
				break
			}
			if !hit {
				break
			}
			changed = true
		}
		// `v = f(...);` (or `T *v = f(...);`), perhaps `w = v;`, then
		// `if (v == nullptr)` or `if (v != nullptr)`
		for _, box := range e.nnBoxes(o) {
			if !e.Live(box) {
				continue // a block a fold spliced away: its items are its parent's now
			}
			for i := 0; i < len(nnItemsOf(box)); i++ {
				items := nnItemsOf(box)
				folded, err := e.nnFold(items, i, nn, &r, seen)
				if err != nil {
					return r, err
				}
				if folded {
					// look at the store again, with what follows it now
					changed = true
					i--
				}
			}
		}
		if !changed {
			break
		}
	}
	if err := e.nnLabels(o); err != nil {
		return r, err
	}
	r.Never = len(e.nnSet(o))
	return r, nil
}

// nnForms are the forms o.In says yes to.
func (e *Editor) nnForms(o NeverNullOptions) []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		if f.Is("defn") && (o.In == nil || o.In(f)) {
			out = append(out, f)
		}
	}
	return out
}

// nnBoxes are the item lists' holders in the functions in scope -- a
// body, a block, a statement expression -- outer before inner, in order.
func (e *Editor) nnBoxes(o NeverNullOptions) []*Node {
	var out []*Node
	for _, f := range e.nnForms(o) {
		Walk(f, func(x *Node) bool {
			if x.Is("defn") || x.Is("block") || x.Is("stmt-expr") {
				out = append(out, x)
			}
			return x.list
		})
	}
	return out
}

// nnItemsOf are the items a box holds now.
func nnItemsOf(b *Node) []*Node {
	switch {
	case b.Is("defn"):
		return Body(b)
	case b.Is("block"):
		return blockItems(b)
	}
	return b.Kids[1:]
}

// nnFold folds the test after the store items[i], if there is one: a
// fold made says true, a fold refused is a line of the report's Left.
func (e *Editor) nnFold(items []*Node, i int, nn map[string]bool, r *NeverNullReport, seen map[string]bool) (bool, error) {
	v, call := nnStore(items[i], nn)
	if v == nil {
		return false, nil
	}
	j := i + 1
	if j < len(items) && items[j].Is("=") && len(items[j].Kids) == 3 && nnPath(items[j].Kids[1]) && nnPath(items[j].Kids[2]) {
		if !gfSame(items[j].Kids[2], v) {
			return false, nil // the line between is not a store of v
		}
		j++
	}
	if j >= len(items) {
		return false, nil
	}
	s := items[j]
	if !s.Is("if") || len(s.Kids) < 3 {
		return false, nil
	}
	c := s.Kids[1]
	if !(c.Is("==") || c.Is("!=")) || len(c.Kids) != 3 || !nnNull(c.Kids[2]) || !nnPath(c.Kids[1]) || !gfSame(c.Kids[1], v) {
		return false, nil
	}
	var err error
	switch {
	case c.Is("==") && len(s.Kids) < 4:
		err = e.Delete(s)
	case c.Is("=="):
		err = e.Unwrap(s, s.Kids[3])
	case len(s.Kids) > 3:
		err = fmt.Errorf("fold_always: the block has an else")
	default:
		err = e.Unwrap(s, s.Kids[2])
	}
	if err != nil {
		vt, _ := ExprText(v)
		ct, _ := ExprText(call)
		key := fmt.Sprintf("%s %s nullptr after %s", vt, c.Head(), ct)
		if !seen[key] {
			seen[key] = true
			r.Left = append(r.Left, key+": "+err.Error())
		}
		return false, nil
	}
	r.Folded++
	return true, nil
}

// nnItems are the items of the functions in scope that ok says yes to.
func (e *Editor) nnItems(o NeverNullOptions, ok func(*Node) bool) []*Node {
	var out []*Node
	for _, b := range e.nnBoxes(o) {
		for _, it := range nnItemsOf(b) {
			if it.list && ok(it) {
				out = append(out, it)
			}
		}
	}
	return out
}

// nnNull says x is the null pointer constant as the text spells it.
func nnNull(x *Node) bool { return !x.list && x.Atom == "nullptr" }

// nnPath says x is what the text's `[\w.>\[\]-]+` spells: a name, a
// member of one, an element of one by a name or a number.
func nnPath(x *Node) bool {
	if !x.list {
		return isIdent(x.Atom)
	}
	switch x.Head() {
	case ".", "->":
		for _, k := range x.Kids[2:] {
			if k.list {
				return false
			}
		}
		return nnPath(x.Kids[1])
	case "index":
		for _, k := range x.Kids[2:] {
			if k.list || !nnWord(k.Atom) {
				return false
			}
		}
		return nnPath(x.Kids[1])
	}
	return false
}

func nnWord(s string) bool {
	for i := 0; i < len(s); i++ {
		if !identByte(s[i]) {
			return false
		}
	}
	return s != ""
}

// nnCall says x is a call of a never-NULL function, the whole expression,
// under a cast to a pointer of a plain type at most.
func nnCall(x *Node, nn map[string]bool) bool {
	if x.Is("cast") && len(x.Kids) == 3 && nnPtrType(x.Kids[1]) {
		x = x.Kids[2]
	}
	return x.Is("call") && !x.Kids[1].list && nn[x.Kids[1].Atom]
}

// nnPtrType says t is a pointer, at any depth, to a type of one word
// (struct, const and unsigned allowed before it): the text's cast.
func nnPtrType(t *Node) bool {
	if !t.Is("ptr") || len(t.Kids) != 2 {
		return false
	}
	for t.Is("ptr") && len(t.Kids) == 2 {
		t = t.Kids[1]
	}
	if t.Is("const") && len(t.Kids) == 2 {
		t = t.Kids[1]
	}
	switch {
	case !t.list:
		return nnWord(t.Atom)
	case t.Is("struct") && len(t.Kids) == 2 && !t.Kids[1].list:
		return true
	case t.Is("unsigned") && len(t.Kids) == 2 && !t.Kids[1].list:
		return true
	}
	return false
}

// nnStore is v and the call when the item stores a never-NULL call in v:
// `v = f(...);`, or a pointer's declaration `T *v = f(...);`.
func nnStore(it *Node, nn map[string]bool) (*Node, *Node) {
	switch {
	case it.Is("=") && len(it.Kids) == 3 && nnPath(it.Kids[1]) && nnCall(it.Kids[2], nn):
		return it.Kids[1], it.Kids[2]
	case it.Is("def"):
		i := defNameAt(it)
		if i <= 0 || i+2 >= len(it.Kids) || !nnPtrType(it.Kids[i+1]) {
			return nil, nil
		}
		if v := it.Kids[len(it.Kids)-1]; nnCall(v, nn) && i+3 == len(it.Kids) {
			return it.Kids[i], v
		}
	}
	return nil, nil
}

// nnSet is the set of functions that never return NULL, to its fixed
// point: the roots, and every function in scope returning a pointer whose
// every return is a never-NULL call or a local assigned only never-NULL
// calls.
func (e *Editor) nnSet(o NeverNullOptions) map[string]bool {
	nn := map[string]bool{}
	for _, r := range o.Roots {
		nn[r] = true
	}
	fs := e.nnForms(o)
	for {
		changed := false
		for _, f := range fs {
			name := declName(f)
			if nn[name] {
				continue
			}
			t := defType(f)
			if t == nil || !t.Is("fn") || len(t.Kids) < 3 || !t.Kids[2].Is("ptr") {
				continue
			}
			all, any := true, false
			walkBody(f, func(x *Node) bool {
				if !x.Is("return") {
					return all
				}
				any = true
				if len(x.Kids) < 2 || !nnCall(x.Kids[1], nn) && !e.nnLocal(f, x.Kids[1], nn) {
					all = false
				}
				return false
			})
			if all && any {
				nn[name] = true
				changed = true
			}
		}
		if !changed {
			return nn
		}
	}
}

// nnLocal says ex is (a cast of) a pointer local of f declared without a
// value, never stepped, its address never taken, whose every store is a
// never-NULL call.  Its name is asked as the text asked it, by spelling:
// a store is any `=` whose left side ends in the name as printed (`*v =`
// and `p->v =` too, and a declaration of the name with a value); a step
// any `++`, `--`, `+=`, `-=` printed against it; an address any `&`
// printed before it (`x && v` too).
func (e *Editor) nnLocal(f, ex *Node, nn map[string]bool) bool {
	if ex.Is("cast") && len(ex.Kids) == 3 && nnPtrType(ex.Kids[1]) {
		ex = ex.Kids[2]
	}
	if ex.list || !isIdent(ex.Atom) {
		return false
	}
	v := ex.Atom
	declared := false
	walkBody(f, func(x *Node) bool {
		if x.Is("def") {
			if i := defNameAt(x); i > 0 && x.Kids[i].Atom == v && i+2 == len(x.Kids) && nnPtrType(x.Kids[i+1]) {
				declared = true
			}
		}
		return !declared
	})
	if !declared {
		return false
	}
	ok, stores := true, 0
	walkBody(f, func(x *Node) bool {
		if !ok || !x.list {
			return ok
		}
		switch h := x.Head(); {
		case h == "addr", h == "pre++", h == "pre--":
			ok = nnFirst(x.Kids[1]) != v
		case h == "&" || h == "&&":
			for _, k := range x.Kids[2:] {
				ok = ok && nnFirst(k) != v
			}
		case h == "post++", h == "post--", h == "+=", h == "-=":
			ok = nnLast(x.Kids[1]) != v
		case h == "=" && nnLast(x.Kids[1]) == v:
			stores++
			ok = nnCall(x.Kids[2], nn) && e.nnPlainStore(x)
		case h == "def":
			if i := defNameAt(x); i > 0 && x.Kids[i].Atom == v && i+2 < len(x.Kids) {
				stores++
				ok = nnCall(x.Kids[len(x.Kids)-1], nn)
			}
		}
		return ok
	})
	return ok && stores > 0
}

// nnPlainStore says the store stands where the text read its value whole,
// up to the `;` after it: a statement, a for's first clause, or the left
// of a comparison.
func (e *Editor) nnPlainStore(x *Node) bool {
	p, i := e.index(x)
	switch {
	case p == nil:
		return false
	case e.place(p, i) == placeItem:
		return true
	case p.Is("for") && i == 1:
		return true
	case p.Is("==") && i == 1:
		return true
	}
	return false
}

// nnPrefix are the forms printed with their operator first.
var nnPrefix = map[string]bool{"!": true, "~": true, "addr": true, "deref": true, "pre++": true, "pre--": true,
	"cast": true, "sizeof": true, "sizeof-bare": true, "sizeof-type": true, "alignof": true, "alignof-bare": true,
	"alignof-type": true, "paren": true, "literal": true, "generic": true, "stmt-expr": true, "macro": true, "label-addr": true}

// nnFirst is the first token x prints as when it is a name, else "".
func nnFirst(x *Node) string {
	for x.list {
		h := x.Head()
		if nnPrefix[h] || len(x.Kids) < 2 || (h == "-" || h == "+") && len(x.Kids) == 2 {
			return ""
		}
		x = x.Kids[1]
	}
	return x.Atom
}

// nnLast is the last token x prints as when it is a name, else "".
func nnLast(x *Node) string {
	for x.list {
		switch x.Head() {
		case ".", "->":
			return x.Kids[len(x.Kids)-1].Atom
		case "deref", "addr", "!", "~", "pre++", "pre--", "-", "+", "*", "/", "%", "<<", ">>", "<", ">", "<=", ">=",
			"==", "!=", "&", "^", "|", "&&", "||", "=", "+=", "-=", "*=", "/=", "%=", "<<=", ">>=", "&=", "^=", "|=", "comma", "?":
			x = x.Kids[len(x.Kids)-1]
		case "cast":
			x = x.Kids[2]
		default:
			return ""
		}
	}
	return x.Atom
}

// nnLabels deletes the labels of the functions in scope that no goto
// reaches.
func (e *Editor) nnLabels(o NeverNullOptions) error {
	var gone []*Node
	for _, f := range e.nnForms(o) {
		walkBody(f, func(x *Node) bool {
			if x.Is("label") && len(e.Uses(x)) == 0 {
				gone = append(gone, x)
			}
			return true
		})
	}
	for _, l := range gone {
		if err := e.Delete(l); err != nil {
			return err
		}
	}
	return nil
}
