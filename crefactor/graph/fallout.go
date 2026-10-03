package graph

import (
	"fmt"
	"strconv"
)

// THE FALL-OUT CLOSURE (doc/GRAPH.md, step 4).  A cut is a deletion: of a
// function, an object, a member, a statement.  What it leaves behind are
// DANGLING edges -- uses of what it deleted -- and FallOut closes over them
// until none is left, or refuses, naming the first it cannot discharge.  The
// rules are generic, written once, and name nothing in any program; what a
// program must say -- which of its functions only allocate, which act only
// on what they are handed -- comes through FallOutOptions, as the sweep's
// roots do, and a cut's own rules (a shape only it may remove) are Rules it
// passes, tried first.
//
// At a dangling use, the item (statement) holding it:
//
//   - CALL.  `f(args);` -- an expression statement, or one cast to void --
//     of a deleted function goes, its arguments free of side effects.  In
//     any other context -- the call's value used -- nothing is taken: the
//     cut must say what the value is (Replace), and the closure refuses.
//   - STORE.  An expression statement whose only effect is on deleted
//     locations goes: `x = E;`, `p->m += E;`, `++x;`, `a[i] = E;` (x, m, a
//     deleted), with E free of side effects or itself such a store
//     (`p->m = ++x;`).  A call of a function the options call Pure (one that
//     only allocates: its result unkept is a leak at most) is free of them.
//   - THROUGH.  A call of a function the options say acts only on what its
//     arguments point at (Through), every argument the address of a deleted
//     location, goes.
//   - VALUE.  A read of a deleted object that nothing writes and whose value
//     is a constant (its initialiser, or zero for a file-scope object without
//     one) is that value (FallOut's seed rule in crefactor/xform); a pointer's
//     is `((void *)V)`.
//
// Then what became constant, starting only from what an edit wrote (as
// xform.FallOut fires only on its own marks): an int expression whose
// operands are constants is its value; `c ? a : b` of a constant c is the
// branch; an `if` of a constant condition is the branch it takes, a block's
// items spliced into the block that held it (the block kept whole when it
// declares anything); `while (0)` goes.  And, unless KeepEmpty: a block an
// edit emptied goes when it is an item or an else, and an `if` whose only
// branch it was goes, its condition free of side effects.  Then the caller
// collects (Collect): what nothing names is the sweep's.
//
// Not generic, and so not here: `a && K` losing its K where only the
// truth is read, a function whose body is `return K;` made K at its calls,
// a parameter every call passes the same constant (xform.FallOut's later
// rules; no gate here exercises them).

// FallOutOptions are what the closure must be told about the program.
type FallOutOptions struct {
	// Pure are functions a call of which, its value unused, does nothing a
	// removed statement must keep: they only allocate.
	Pure []string
	// Through are functions that act only on what their arguments point
	// at: a call of one whose every argument is a deleted location's
	// address goes with the location.
	Through []string
	// Rules are the cut's own, tried first at every dangling edge.
	Rules []Rule
	// KeepEmpty leaves a block the closure empties, as the text cutters do.
	KeepEmpty bool
}

// A Rule is a cut's own fall-out: given a dangling edge, the items that go
// to discharge it, or none for the next rule to be asked.
type Rule struct {
	Name string
	Take func(e *Editor, d Dangling) ([]*Node, error)
}

// A Removal is one item the closure removed.
type Removal struct {
	Item   *Node  // the item, out of the graph, its form whole
	Fn     string // the function it was in, or ""
	Rule   string // call, store, through, empty, or a Rule's name
	Target *Node  // the deleted declaration whose use it held
}

// FallOutStats are what a closure did.
type FallOutStats struct {
	Removed  []Removal
	Values   int // reads of a deleted constant made its value
	Folded   int // expressions made their value
	Branches int // ifs, ?:s and while(0)s taken as the branch they take
	Rounds   int
}

// Unhandled is the closure's refusal: a dangling edge no rule discharges.
type Unhandled struct {
	D    Dangling
	Fn   string
	Left int
}

func (u *Unhandled) Error() string {
	kind := "refers to"
	if u.D.Typed {
		kind = "is typed by"
	}
	in := ""
	if u.Fn != "" {
		in = " in " + u.Fn
	}
	return fmt.Sprintf("fall-out: #%d %s%s %s #%d (%s), which an edit deleted, and no rule takes it (%d such edges)",
		u.D.Use.ID, label(u.D.Use), in, kind, u.D.Target.ID, label(u.D.Target), u.Left)
}

type closure struct {
	e       *Editor
	opt     FallOutOptions
	pureFn  map[string]bool
	through map[string]bool
	st      *FallOutStats
}

// FallOut closes over the dangling edges the edits so far left.
func (e *Editor) FallOut(opt FallOutOptions) (FallOutStats, error) {
	var st FallOutStats
	c := &closure{e: e, opt: opt, pureFn: set(opt.Pure), through: set(opt.Through), st: &st}
	for {
		st.Rounds++
		progress := false
		for _, d := range e.Dangling() {
			if !e.live(d) {
				continue
			}
			ok, err := c.discharge(d)
			if err != nil {
				return st, err
			}
			progress = progress || ok
		}
		if c.fold() {
			progress = true
		}
		if !opt.KeepEmpty && c.empties() {
			progress = true
		}
		if !progress {
			break
		}
	}
	if left := e.Dangling(); len(left) > 0 {
		return st, &Unhandled{D: left[0], Fn: declName(e.Function(left[0].Use)), Left: len(left)}
	}
	return st, nil
}

func set(names []string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

// remove takes item out, recorded.
func (c *closure) remove(item *Node, rule string, target *Node) error {
	fn := declName(c.e.Function(item))
	if err := c.e.Delete(item); err != nil {
		return err
	}
	c.st.Removed = append(c.st.Removed, Removal{Item: item, Fn: fn, Rule: rule, Target: target})
	return nil
}

// discharge applies the first rule that takes d.
func (c *closure) discharge(d Dangling) (bool, error) {
	if d.Typed {
		return false, nil
	}
	for _, r := range c.opt.Rules {
		items, err := r.Take(c.e, d)
		if err != nil {
			return false, err
		}
		if len(items) == 0 {
			continue
		}
		for _, it := range items {
			if c.e.Live(it) {
				if err := c.remove(it, r.Name, d.Target); err != nil {
					return false, fmt.Errorf("rule %s: %w", r.Name, err)
				}
			}
		}
		return true, nil
	}
	if it := c.e.Item(d.Use); it != nil && !IsStatement(it) {
		if kind := c.discardable(it); kind != "" {
			return true, c.remove(it, kind, d.Target)
		}
	}
	return c.value(d)
}

var assignOps = map[string]bool{
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true,
	"<<=": true, ">>=": true, "&=": true, "^=": true, "|=": true,
}

var incDec = map[string]bool{"pre++": true, "pre--": true, "post++": true, "post--": true}

// pure says x has no side effect: no store, no call but of a Pure
// function, nothing the forms keep as text.
func (c *closure) pure(x *Node) bool {
	if !x.list {
		return true
	}
	switch h := x.Head(); {
	case assignOps[h] || incDec[h] || h == "stmt-expr" || h == "macro" || h == "verbatim":
		return false
	case h == "call":
		if f := x.Kids[1]; f.list || !c.pureFn[f.Atom] {
			return false
		}
	}
	for _, k := range x.Kids[1:] {
		if !c.pure(k) {
			return false
		}
	}
	return true
}

// deletedLoc says x is (part of) a location an edit deleted: an object, a
// member selected (its base free of side effects), an element of one.
func (c *closure) deletedLoc(x *Node) bool {
	if !x.list {
		r := x.Ref()
		return r != nil && !c.e.Live(r)
	}
	switch x.Head() {
	case "paren":
		return c.deletedLoc(x.Kids[1])
	case "->", ".":
		last := x.Kids[len(x.Kids)-1]
		for _, k := range x.Kids[1 : len(x.Kids)-1] {
			if !c.pure(k) {
				return false
			}
		}
		if r := last.Ref(); r != nil && !c.e.Live(r) {
			return true
		}
		return x.Is(".") && c.deletedLoc(x.Kids[1])
	case "index":
		for _, k := range x.Kids[2:] {
			if !c.pure(k) {
				return false
			}
		}
		return c.deletedLoc(x.Kids[1])
	}
	return false
}

// discardable says what lets x, an expression whose value is unused, go:
// "call", "store", "through" or "pure"; "" when nothing does.
func (c *closure) discardable(x *Node) string {
	if !x.list {
		return "pure"
	}
	switch h := x.Head(); {
	case h == "paren" || h == "cast" && len(x.Kids) == 3 && !x.Kids[1].list && x.Kids[1].Atom == "void":
		return c.discardable(x.Kids[len(x.Kids)-1])
	case h == "comma":
		kind := "pure"
		for _, k := range x.Kids[1:] {
			switch k := c.discardable(k); k {
			case "":
				return ""
			case "pure":
			default:
				kind = k
			}
		}
		return kind
	case assignOps[h]:
		if c.deletedLoc(x.Kids[1]) && c.discardable(x.Kids[2]) != "" {
			return "store"
		}
		return ""
	case incDec[h]:
		if c.deletedLoc(x.Kids[1]) {
			return "store"
		}
		return ""
	case h == "call":
		args := x.Kids[2:]
		f := x.Kids[1]
		if !f.list && f.Ref() != nil && !c.e.Live(f.Ref()) {
			for _, a := range args {
				if !c.pure(a) {
					return ""
				}
			}
			return "call"
		}
		if !f.list && c.through[f.Atom] && len(args) > 0 {
			for _, a := range args {
				if !a.Is("addr") || !c.deletedLoc(a.Kids[1]) {
					return ""
				}
			}
			return "through"
		}
	}
	if c.pure(x) {
		return "pure"
	}
	return ""
}

// value makes a read of a deleted constant its value.
func (c *closure) value(d Dangling) (bool, error) {
	u, t := d.Use, d.Target
	if u.list || !t.Is("def") || hasPrefix(t, "typedef") || hasPrefix(t, "extern") || c.written(u) {
		return false, nil
	}
	for _, other := range c.e.Uses(t) {
		if c.written(other) {
			return false, nil
		}
	}
	var v int64
	if val := defValue(t); val != nil {
		var ok bool
		if v, ok = evalForm(val, nil); !ok {
			return false, nil
		}
	} else if !c.e.wasTop[t] {
		return false, nil // a local without a value has none
	}
	lit := Literal(v)
	if t.Type != nil && t.Type.Is("pointer") {
		lit = NewList(NewAtom("paren"), NewList(NewAtom("cast"), NewList(NewAtom("ptr"), NewAtom("void")), lit))
	}
	if err := c.e.Replace(u, lit); err != nil {
		return false, err
	}
	c.st.Values++
	return true, nil
}

// written says the use u is stored to, incremented or has its address
// taken.
func (c *closure) written(u *Node) bool {
	x := u
	for {
		p := c.e.Parent(x)
		if p == nil {
			return false
		}
		switch h := p.Head(); {
		case h == "paren", h == "index" && p.Kids[1] == x, h == "." && p.Kids[1] == x:
			x = p
			continue
		case (h == "->" || h == ".") && p.Kids[len(p.Kids)-1] == x:
			x = p
			continue
		case assignOps[h]:
			return p.Kids[1] == x
		case incDec[h], h == "addr":
			return true
		}
		return false
	}
}

// defValue is a def's value form, or nil.
func defValue(t *Node) *Node {
	i := defNameAt(t)
	if i == 0 || len(t.Kids) <= i+2 {
		return nil
	}
	v := t.Kids[len(t.Kids)-1]
	if isAttrForm(v) || v.Is("asm-label") {
		return nil
	}
	return v
}

// Literal is v written as C-lisp writes a constant: decimal, a negative one
// negated.
func Literal(v int64) *Node {
	if v < 0 {
		return NewList(NewAtom("-"), NewAtom(strconv.FormatInt(-v, 10)))
	}
	return NewAtom(strconv.FormatInt(v, 10))
}

// constant is x's value when x is a constant the closure may write: an
// atom or a negated atom literal, or an int expression of constants.
func constant(x *Node) (int64, bool) {
	if !x.list || x.Is("-") && len(x.Kids) == 2 && !x.Kids[1].list {
		return evalForm(x, nil)
	}
	if x.Type == nil || !x.Type.Is("basic") || len(x.Type.Kids) != 2 || x.Type.Kids[1].Atom != "int" {
		return 0, false
	}
	return evalForm(x, nil)
}

// fold takes what the edits wrote and what became constant above it.
func (c *closure) fold() bool {
	ws := c.e.Written
	c.e.Written = nil
	progress := false
	for _, w := range ws {
		x := w
		for c.e.Live(x) {
			p := c.e.Parent(x)
			if p == nil {
				break
			}
			if (p.Is("if") || p.Is("while")) && p.Kids[1] == x {
				if ok, err := c.branch(p); err == nil && ok {
					progress = true
				}
				break
			}
			if IsStatement(p) {
				break
			}
			if p.Is("?") && p.Kids[1] == x {
				if v, ok := constant(x); ok {
					b := p.Kids[3]
					if v != 0 {
						b = p.Kids[2]
					}
					if c.e.Replace(p, b) == nil {
						c.st.Branches++
						progress = true
						x = b
						continue
					}
				}
			}
			if p.list && !p.Is("?") {
				if v, ok := constant(p); ok && !SameForm(p, Literal(v)) {
					lit := Literal(v)
					if c.e.Replace(p, lit) == nil {
						c.st.Folded++
						progress = true
						x = lit
						continue
					}
				}
			}
			x = p
		}
	}
	return progress
}

// branch folds an if or a while whose condition is a constant.
func (c *closure) branch(s *Node) (bool, error) {
	v, ok := constant(s.Kids[1])
	if !ok {
		return false, nil
	}
	p, i := c.e.index(s)
	optional := c.e.place(p, i) == placeOptional
	var with []*Node
	switch {
	case s.Is("while"):
		if v != 0 {
			return false, nil
		}
	case v != 0:
		with = c.spliced(s.Kids[2], optional)
	case len(s.Kids) > 3:
		with = c.spliced(s.Kids[3], optional)
	}
	if err := c.e.Replace(s, with...); err != nil {
		return false, err
	}
	c.st.Branches++
	return true, nil
}

// spliced is what a branch taken puts in its if's place: its items, or the
// block whole where it declares something or must stay one node.
func (c *closure) spliced(b *Node, one bool) []*Node {
	if !b.Is("block") {
		return []*Node{b}
	}
	items := blockItems(b)
	if one || len(items) != len(b.Kids)-1 {
		return []*Node{b}
	}
	for _, it := range items {
		if IsStatement(it) && (it.Is("def") || it.Is("typedef") || it.Is("label")) {
			return []*Node{b}
		}
	}
	return items
}

// empties takes the blocks an edit emptied: an item or an else goes, and
// an if whose only branch it was.
func (c *closure) empties() bool {
	bs := c.e.emptied
	c.e.emptied = nil
	progress := false
	for _, b := range bs {
		if !c.e.Live(b) || len(blockItems(b)) > 0 {
			continue
		}
		p, i := c.e.index(b)
		var item *Node
		switch {
		case p == nil:
		case c.e.place(p, i) == placeItem, p.Is("if") && i == 3:
			item = b
		case p.Is("if") && i == 2 && len(p.Kids) == 3 && c.pure(p.Kids[1]):
			if q, j := c.e.index(p); q != nil && (c.e.place(q, j) == placeItem || q.Is("if") && j == 3) {
				item = p
			}
		}
		if item != nil && c.remove(item, "empty", nil) == nil {
			progress = true
		}
	}
	return progress
}
