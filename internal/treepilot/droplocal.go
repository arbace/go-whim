// Package treepilot is the pilot of doc/C-LISP-TREE.md: two of the
// pipeline's text edits written again on C-lisp's tree (crefactor/clisp),
// to be held byte for byte to the text versions on their snapshots and
// measured beside them.  The plan does not run it; internal/cut and
// internal/phase/024 are the edits the pipeline runs.
package treepilot

import (
	"fmt"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// The plumbing of a buffer-local option field, as forms: the shapes
// internal/cut's DropLocal finds with six regular expressions, written as
// the statements they are.  ?f is the field.
var (
	// buf->F = ...;
	plAssign = clisp.MustPattern("(= (-> buf ?f) _)")
	// curbuf->F = -1;
	plUnset = clisp.MustPattern("(= (-> curbuf ?f) (- 1))")
	// check_string_option(&buf->F); clear_string_option(&buf->F);
	plCheck = clisp.MustPattern("(call check_string_option (addr (-> buf ?f)))")
	plClear = clisp.MustPattern("(call clear_string_option (addr (-> buf ?f)))")
	// get_varp's cases: return (char_u *)&(curbuf->F);
	plVarp = clisp.MustPattern("(return (cast (ptr char_u) (addr (paren (-> curbuf ?f)))))")
	// and return <test of curbuf->F> ? (char_u *)&(curbuf->F) : p->var;
	plVarpBoth = clisp.MustPattern("(return (? ?test (cast (ptr char_u) (addr (paren (-> curbuf ?f)))) (-> p var)))")
)

// controlKeepCase is the pilot's control: set, DropLocal leaves the case
// label of a get_varp case it cuts -- C still, a label falling through to the
// next case, and a change the byte comparison must catch.
var controlKeepCase bool

// plumbingTypes are the declared types DropLocal's first pattern accepts.
var plumbingTypes = []*clisp.Node{
	clisp.MustPattern("(ptr char_u)"), clisp.A("int"), clisp.A("long"),
}

// A Tree is the forms a run of tree steps edits, and its index, made when a
// step first asks for it and kept while the steps only delete.
type Tree struct {
	Root *clisp.Node
	ix   *clisp.Index
}

// Index is the tree's index of atoms.
func (t *Tree) Index() *clisp.Index {
	if t.ix == nil {
		t.ix = clisp.NewIndex(t.Root)
	}
	return t.ix
}

// DropLocal is internal/cut's DropLocal on the tree: it removes a
// buffer-local option field and its plumbing -- the declaration, the
// assignments, the frees and checks, and the get_varp case that hands its
// address out, the case label with it -- and refuses as the text version
// does: on no mention, on fewer than three sites, and on any mention left.
// It starts from the field's mentions in the index, as the text version
// starts from the lines that hold the field's name.
func DropLocal(t *Tree, bvar string) (int, error) {
	ix := t.Index()
	if ix.Mentions(bvar, true) == 0 {
		return 0, fmt.Errorf("droplocal: there is no %s here", bvar)
	}
	var cut []*clisp.Cursor
	seen := map[*clisp.Node]bool{}
	add := func(cs ...*clisp.Cursor) {
		for _, c := range cs {
			if !seen[c.Node()] {
				seen[c.Node()] = true
				cut = append(cut, c)
			}
		}
	}
	isField := func(b clisp.Bindings) bool { return b["f"].Atom == bvar }
	for _, a := range ix.Atoms(bvar) {
		// the declaration: a member, or a def with no prefix and no value
		if d := a.Up(); d != nil && declares(d, bvar) {
			add(d)
			continue
		}
		it := a.Item()
		if it == nil {
			continue
		}
		n := it.Node()
		for _, p := range []*clisp.Node{plAssign, plUnset, plCheck, plClear} {
			if b, ok := clisp.Match(p, n); ok && isField(b) {
				add(it)
			}
		}
		// a get_varp case: the label before the return goes with it
		if b, ok := clisp.Match(plVarp, n); ok && isField(b) || isVarpBoth(n, bvar) {
			if prev := it.SiblingCursor(-1); prev != nil && prev.Node().Is("case") {
				if !controlKeepCase {
					add(prev)
				}
				add(it)
			}
		}
	}
	n := 0
	for _, c := range cut {
		if !c.Node().Is("case") {
			n++
		}
		c.Delete()
	}
	if n < 3 {
		return 0, fmt.Errorf("droplocal: %s: only %d plumbing sites, expected at "+
			"least the field, an initialiser and a get_varp case -- the shape has moved",
			bvar, n)
	}
	if left := ix.Mentions(bvar, true); left > 0 {
		return 0, fmt.Errorf("droplocal: %s still has %d mentions after the plumbing "+
			"went -- those are readers, and the phase has to deal with them before the "+
			"field can go", bvar, left)
	}
	return n, nil
}

// declares says whether c is the field's declaration as DropLocal's first
// pattern finds it: `char_u *F;`, `int F;` or `long F;` on a line of its own
// -- a struct's member, or a declaration with no storage class and no value.
func declares(c *clisp.Cursor, bvar string) bool {
	n := c.Node()
	var typ *clisp.Node
	switch {
	case n.Is("def"):
		if clisp.DefName(n) != bvar || len(clisp.Prefix(n)) > 0 || len(n.List) != 3 {
			return false
		}
		typ = n.List[2]
	case c.Parent() != nil && (c.Parent().Is("struct") || c.Parent().Is("union")):
		if len(n.List) != 2 || n.List[0].IsList() || n.List[0].Atom != bvar {
			return false
		}
		typ = n.List[1]
	default:
		return false
	}
	for _, t := range plumbingTypes {
		if clisp.Equal(t, typ) {
			return true
		}
	}
	return false
}

// isVarpBoth is the second get_varp shape: its test must read the field too,
// as the text pattern's `curbuf->F` before the `?` asks.
func isVarpBoth(n *clisp.Node, bvar string) bool {
	b, ok := clisp.Match(plVarpBoth, n)
	return ok && b["f"].Atom == bvar && clisp.Contains(b["test"], clisp.L(clisp.A("->"), clisp.A("curbuf"), clisp.A(bvar)))
}
