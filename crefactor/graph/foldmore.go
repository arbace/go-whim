package graph

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// FOLDX'S GENERIC RULES BESIDE THE CLOSURE'S (doc/GRAPH-MIGRATION.md,
// FOLDX).  Each is a fall-out a cut on the graph may want, opted into
// through FallOutOptions where it is told something, and generic:
//
//   - EMPTIES EVERYWHERE (EmptyBlocks; xform.EmptyBlocks).  Not only the
//     blocks an edit emptied: in the forms a caller names, an if whose
//     block is empty, with no else after it and a condition free of side
//     effects, goes; an empty else goes; an empty else-if ending its chain,
//     its condition free of side effects, goes.  With them, alternating to
//     a fixed point, DEAD LOCALS (edit.DeadStores): a local only ever given
//     a value -- every use the left side of `x = E;` alone as a statement,
//     E and its initialiser free of side effects -- goes with its stores.
//   - LABELS.  A label an edit took the last goto of goes.
//   - KEEP CONDITION.  An if whose only branch an edit emptied, its
//     condition doing something, is that condition alone, cast to void, a
//     `!` in front taken off: `if (!f()) {X}` with X cut is `(void)f();`.
//   - STORE IFS.  An if whose only branch held only stores the store rule
//     took, its condition free of side effects, goes, though KeepEmpty
//     leaves the blocks an edit empties.
//   - VALUES.  A read of an object a cut deleted is the value the cut says
//     it is worth, not its initialiser's.

// EmptyStats is what EmptyBlocks took.
type EmptyStats struct {
	Blocks int      // the empty blocks folded away
	Locals []string // the locals only given values, gone with their stores, in order
}

func (s EmptyStats) String() string {
	return fmt.Sprintf("%d empty blocks fold away; %d locals only given values go with their stores: %s",
		s.Blocks, len(s.Locals), strings.Join(s.Locals, " "))
}

// EmptyOptions are what EmptyBlocks is told.
type EmptyOptions struct {
	// In says which top-level forms it folds in: all of them when nil.
	In func(form *Node) bool
	// Cond, when set, is a caller's own test of a condition's (or a
	// store's value's) C text, beside its being free of side effects:
	// where a text step's bytes must come back, its heuristic --
	// crefactor/edit's PureCond, which refuses `c == '='` for its `=`.
	Cond func(text string) bool
}

// EmptyBlocks folds the empty blocks of the forms o.In says yes to, and
// the locals then only given values, each to its fixed point, in turn
// until neither takes anything.
func (e *Editor) EmptyBlocks(o EmptyOptions) (EmptyStats, error) {
	var st EmptyStats
	c := &closure{e: e, pureFn: map[string]bool{}, through: map[string]bool{}, st: &FallOutStats{}, cond: o.Cond}
	in := o.In
	for {
		n, err := c.emptyFold(in)
		if err != nil {
			return st, err
		}
		took, err := c.deadLocals(in)
		if err != nil {
			return st, err
		}
		st.Blocks += n
		st.Locals = append(st.Locals, took...)
		if n == 0 && len(took) == 0 {
			return st, nil
		}
	}
}

// scope is the forms in says yes to, in order.
func (c *closure) scope(in func(*Node) bool) []*Node {
	var out []*Node
	for _, f := range c.e.g.Forms {
		if in == nil || in(f) {
			out = append(out, f)
		}
	}
	return out
}

func isEmptyBlock(b *Node) bool {
	return b.Is("block") && len(b.Kids) == 1
}

// emptyFold is one fold to its fixed point: how many blocks went.
func (c *closure) emptyFold(in func(*Node) bool) (int, error) {
	n := 0
	for {
		var ifs []*Node
		for _, f := range c.scope(in) {
			Walk(f, func(x *Node) bool {
				if x.Is("if") {
					ifs = append(ifs, x)
				}
				return true
			})
		}
		changed := false
		for _, s := range ifs {
			if !c.e.Live(s) {
				continue
			}
			var gone *Node
			switch {
			case len(s.Kids) == 4 && isEmptyBlock(s.Kids[3]):
				gone = s.Kids[3] // an empty else
			case len(s.Kids) == 3 && isEmptyBlock(s.Kids[2]) && c.condPure(s.Kids[1]):
				// an if, or an else-if ending its chain
				p, i := c.e.index(s)
				if p != nil && (c.e.place(p, i) == placeItem || p.Is("if") && i == 3) {
					gone = s
				}
			}
			if gone == nil {
				continue
			}
			if err := c.e.Delete(gone); err != nil {
				return n, err
			}
			n++
			changed = true
		}
		if !changed {
			return n, nil
		}
	}
}

// deadLocals takes each local only ever given a value, with its stores, to
// the fixed point, in the order the text names them.
func (c *closure) deadLocals(in func(*Node) bool) ([]string, error) {
	var took []string
	for {
		d, stores := c.deadLocal(in)
		if d == nil {
			return took, nil
		}
		for _, s := range stores {
			if err := c.e.Delete(s); err != nil {
				return took, err
			}
		}
		if err := c.e.Delete(d); err != nil {
			return took, err
		}
		took = append(took, declName(d))
	}
}

// deadLocal is the first local only given values, and its stores.
func (c *closure) deadLocal(in func(*Node) bool) (*Node, []*Node) {
	for _, f := range c.scope(in) {
		if !f.Is("defn") {
			continue
		}
		var defs []*Node
		walkBody(f, func(x *Node) bool {
			if x.Is("def") {
				defs = append(defs, x)
			}
			return true
		})
		for _, d := range defs {
			if stores, ok := c.onlyStored(f, d); ok {
				return d, stores
			}
		}
	}
	return nil, nil
}

// onlyStored says the local d of the function f is only ever given a value,
// and returns the statements that give it one.
func (c *closure) onlyStored(f, d *Node) ([]*Node, bool) {
	p, i := c.e.index(d)
	if p == nil || c.e.place(p, i) != placeItem || hasPrefix(d, "typedef") || hasPrefix(d, "extern") || hasPrefix(d, "register") {
		return nil, false
	}
	t := defType(d)
	if t == nil || volatileForm(t) || t.Is("array") || t.Is("fn") || definesType(t) {
		return nil, false
	}
	if v := defValue(d); v != nil && (v.Is("init") || !c.condPure(v)) {
		return nil, false
	}
	name := declName(d)
	var stores []*Node
	ok := true
	uses := map[*Node]bool{}
	for _, u := range c.e.Uses(d) {
		uses[u] = true
		s := c.e.Parent(u)
		if s == nil || !s.Is("=") || s.Kids[1] != u || !c.condPure(s.Kids[2]) {
			return nil, false
		}
		if q, j := c.e.index(s); q == nil || c.e.place(q, j) != placeItem {
			return nil, false
		}
		stores = append(stores, s)
	}
	// the text's own caution: any other mention of the name after the
	// declaration, in the function -- another local's, a member's, a
	// string's -- keeps it
	after := false
	Walk(f, func(x *Node) bool {
		if !ok {
			return false
		}
		if x == d {
			after = true
			return false
		}
		if !after || x.list {
			return true
		}
		switch {
		case uses[x]:
		case x.Atom == name:
			ok = false
		case strings.HasPrefix(x.Atom, `"`) && wordIn(x.Atom, name):
			ok = false
		}
		return ok
	})
	return stores, ok
}

// condPure says an expression has no side effect, and passes the caller's
// own test of its text.
func (c *closure) condPure(x *Node) bool {
	if !c.pure(x) {
		return false
	}
	if c.cond == nil {
		return true
	}
	t, err := ExprText(x)
	return err == nil && c.cond(t)
}

// ExprText is an expression's C, as the C view prints it where a whole
// expression stands (an if's condition, a statement).
func ExprText(x *Node) (string, error) {
	f := clisp.L(clisp.A("defn"), clisp.A("f"), clisp.L(clisp.A("fn"), clisp.L(clisp.A("void")), clisp.A("void")), Lisp(x))
	out, err := clisp.Print([]*clisp.Node{f})
	if err != nil {
		return "", err
	}
	s := string(out)
	a := strings.Index(s, "{\n"+clisp.Indent)
	z := strings.LastIndex(s, ";\n}")
	if a < 0 || z < a {
		return "", fmt.Errorf("the C view of %s", label(x))
	}
	return s[a+2+len(clisp.Indent) : z], nil
}

// wordIn says name stands in s as a whole word.
func wordIn(s, name string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], name)
		if j < 0 {
			return false
		}
		a, z := i+j, i+j+len(name)
		if (a == 0 || !identByte(s[a-1])) && (z == len(s) || !identByte(s[z])) {
			return true
		}
		i = a + 1
	}
}

// labels takes each label an edit took the last goto of.
func (c *closure) labels() (bool, error) {
	progress := false
	for _, f := range c.e.g.Forms {
		if !f.Is("defn") {
			continue
		}
		var gone []*Node
		walkBody(f, func(x *Node) bool {
			if x.Is("label") && len(c.e.Uses(x)) == 0 && len(c.e.usesOf(x)) > 0 {
				gone = append(gone, x)
			}
			return true
		})
		for _, l := range gone {
			if err := c.remove(l, "label", nil); err != nil {
				return progress, err
			}
			progress = true
		}
	}
	return progress, nil
}

// keepConditions makes each if whose only branch an edit emptied, its
// condition doing something, the condition alone.
func (c *closure) keepConditions() (bool, error) {
	progress := false
	for _, b := range c.e.emptied {
		if !c.e.Live(b) || len(blockItems(b)) > 0 {
			continue
		}
		s := c.e.Parent(b)
		if s == nil || !s.Is("if") || len(s.Kids) != 3 || s.Kids[2] != b || c.pure(s.Kids[1]) {
			continue
		}
		if p, i := c.e.index(s); p == nil || c.e.place(p, i) != placeItem {
			continue
		}
		cond := s.Kids[1]
		if cond.Is("!") && len(cond.Kids) == 2 {
			cond = cond.Kids[1]
		}
		if err := c.e.Replace(s, Void(cond)); err != nil {
			return progress, err
		}
		c.st.Kept++
		progress = true
	}
	return progress, nil
}

// storeIf takes the if whose only branch the store rule just emptied.
func (c *closure) storeIf(block *Node) error {
	if block == nil || !c.e.Live(block) || len(blockItems(block)) > 0 {
		return nil
	}
	s := c.e.Parent(block)
	if s == nil || !s.Is("if") || len(s.Kids) != 3 || s.Kids[2] != block || !c.pure(s.Kids[1]) {
		return nil
	}
	if p, i := c.e.index(s); p == nil || !(c.e.place(p, i) == placeItem || p.Is("if") && i == 3) {
		return nil
	}
	return c.remove(s, "store-if", nil)
}
