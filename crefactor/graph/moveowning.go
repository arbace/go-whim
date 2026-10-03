package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// A MOVE THAT OWNS WHAT IT LEAVES UNPROVIDED (B3d, phase 43's shape).
//
// MoveFormsBefore and MoveFormsAfter refuse a move that would leave a name
// a header provided unprovided above its first use.  Moving the includes
// themselves below forms that use their macros is such a move, and is
// exactly how a file's first include becomes the line between a core that
// takes nothing from the headers and a host that does: the core must then
// declare those names itself, and its tokens that were the macros become
// uses of its own declarations -- what an import of the text after makes
// of them.  MoveWouldLose asks what a move would leave unprovided, as a
// query; MoveFormsOwning makes the move, lets the caller declare the lost
// macros (FRAG, BUILD), and rebinds the tokens, holding the result to the
// headers' rule against the graph before the move.

// MoveWouldLose is what moving the top-level forms ns to just before (or,
// after set, just after) the top-level form at would leave unprovided:
// the names the headers provide to some form now and would not after,
// each with the first form that uses it.  The graph is not changed.
func (e *Editor) MoveWouldLose(at *Node, after bool, ns []*Node) ([]HeaderUse, error) {
	nf, err := e.movedOrder(at, after, ns)
	if err != nil {
		return nil, err
	}
	before := e.g.Forms
	s0, err := e.headerState()
	if err != nil {
		return nil, err
	}
	e.g.Forms = nf
	s1, err := e.headerState()
	e.g.Forms = before
	if err != nil {
		return nil, err
	}
	return lostBetween(s0, s1), nil
}

// MoveFormsOwning moves the top-level forms ns to just before (after set:
// just after) at, keeping their ids, where the move may leave MACROS a
// header provided unprovided: own is called with those (the lost uses, as
// MoveWouldLose names them) once the forms have moved, and must declare
// each name as the file's own where its uses will resolve to it (a FRAG or
// BUILD of declarations); then every token that was such a macro -- an
// atom with no edge, not a declaring one, in a form no include above
// provides it to any more -- becomes a use of the declaration Resolve
// finds there (a new atom with its refers edge).  It returns those uses.
//
// Every edge the move crosses -- a use inside the moved forms of a
// declaration outside them, a use outside of one inside -- must resolve
// from where it now stands to the declaration it refers to or to another
// file-scope declaration of its name (one entity in C), as the importer
// resolves it (cc's rule: the first visible declaration, but the
// definition where that is a prototype naming no parameter and a
// definition naming them is visible), and is retargeted there.
//
// Refused before anything changes: what moveForms refuses (a form not
// top-level, named twice, the place itself), an edge that would resolve to
// nothing or elsewhere, and a lost DECLARATION (a header's function or type: no
// declaration of the file's own stands in for a header's declaration
// here).  Refused after the move, with the graph left as the move, own and
// the rebinding made it (a run of edits that stops): own's error; a token
// that resolves to nothing the file declares, or to an external; a lost
// macro said in a macro invocation's text (the text is not nodes); and,
// held to the rule against the graph before the move, a name still left
// unprovided or a new collision.
func (e *Editor) MoveFormsOwning(at *Node, after bool, ns []*Node, own func(lost []HeaderUse) error) ([]*Node, error) {
	what := fmt.Sprintf("move %d form(s) owning what they leave", len(ns))
	nf, err := e.movedOrder(at, after, ns)
	if err != nil {
		return nil, err
	}
	moving := map[*Node]bool{}
	for _, n := range ns {
		moving[n] = true
	}
	before := append([]*Node(nil), e.g.Forms...)
	s0, err := e.headerState()
	if err != nil {
		return nil, fmt.Errorf("%s: %v", what, err)
	}
	e.g.Forms = nf
	re, err := e.crossingRetargets(ns, moving)
	if err != nil {
		e.g.Forms = before
		return nil, fmt.Errorf("%s: %v", what, err)
	}
	s1, err := e.headerState()
	if err != nil {
		e.g.Forms = before
		return nil, fmt.Errorf("%s: %v", what, err)
	}
	lost := lostBetween(s0, s1)
	var hard []HeaderUse
	names := map[string]bool{}
	for _, u := range lost {
		if u.Macro {
			names[u.Name] = true
		} else {
			hard = append(hard, u)
		}
	}
	if len(hard) > 0 {
		e.g.Forms = before
		return nil, fmt.Errorf("%s: %s", what, unprovided(hard))
	}
	act := Act{Op: "move"}
	for _, n := range ns {
		act.Moved = append(act.Moved, n.ID)
	}
	e.Log = append(e.Log, act)
	for _, r := range re {
		r.use.Refs[r.i] = r.to
		e.extra[r.to] = append(e.extra[r.to], r.use)
		e.Log = append(e.Log, Act{Op: "retarget", Moved: []ID{r.use.ID}})
	}
	if own != nil {
		if err := own(lost); err != nil {
			return nil, fmt.Errorf("%s: %v", what, err)
		}
	}
	// the tokens that were the lost macros, each resolved where it is
	s2, err := e.headerState()
	if err != nil {
		return nil, fmt.Errorf("%s: %v", what, err)
	}
	type rebind struct{ atom, decl *Node }
	var rb []rebind
	declaring := map[*Node]bool{}
	for i, f := range s2.forms {
		if IsInclude(f) || f.Is("directive") {
			continue
		}
		declaringAtoms(f, declaring)
		Walk(f, func(n *Node) bool {
			if err != nil {
				return false
			}
			if n.list {
				if (n.Is("macro") || n.Is("macro-decl") || n.Is("verbatim")) && len(n.Kids) > 1 && !n.Kids[1].list {
					for _, id := range identsIn(unquote(n.Kids[1].Atom)) {
						if names[id] && s2.providedAbove(id, true, i, nil) == nil {
							err = fmt.Errorf("%s: `%s`, a lost macro, is said in %s's text, which cannot be made a use", what, id, label(n))
						}
					}
				}
				return true
			}
			if !names[n.Atom] || declaring[n] || n.Ref() != nil || s2.providedAbove(n.Atom, true, i, nil) != nil {
				return true
			}
			d := e.Resolve(n, n.Atom)
			if d == nil || isExtern(d) {
				err = fmt.Errorf("%s: `%s` in %s would name nothing the file declares there", what, n.Atom, label(f))
				return false
			}
			rb = append(rb, rebind{n, d})
			return true
		})
		if err != nil {
			return nil, err
		}
	}
	// a token's type is its macro's: where the declaration that takes it
	// has that type, the typed edges above it stay as they are
	alike := map[string]bool{}
	for _, u := range lost {
		if _, ok := alike[u.Name]; !ok {
			alike[u.Name] = false
			if d := e.Resolve(u.First, u.Name); d != nil && d.Type != nil && d.Type.Is("basic") {
				var words []string
				for _, k := range d.Type.Kids[1:] {
					words = append(words, k.Atom)
				}
				alike[u.Name] = macroBasicType(s0, u.Name) == strings.Join(words, " ")
			}
		}
	}
	var out []*Node
	for _, r := range rb {
		type held struct{ q, t *Node }
		var keep []held
		if alike[r.atom.Atom] {
			for q := e.Parent(r.atom); q != nil && q.Type != nil && !IsStatement(q) && q.up != e.top[0]; q = e.Parent(q) {
				keep = append(keep, held{q, q.Type})
			}
		}
		use := &Node{Atom: r.atom.Atom, Refs: []*Node{r.decl}}
		if err := e.Replace(r.atom, use); err != nil {
			return out, fmt.Errorf("%s: %v", what, err)
		}
		for _, h := range keep {
			h.q.Type = h.t
			e.typed(h.q)
		}
		out = append(out, use)
	}
	// the rule, against the graph before the move
	s3, err := e.headerState()
	if err != nil {
		return out, fmt.Errorf("%s: %v", what, err)
	}
	if still := lostBetween(s0, s3); len(still) > 0 {
		return out, fmt.Errorf("%s: %s", what, unprovided(still))
	}
	old := map[string]bool{}
	for _, c := range s0.collisions() {
		old[IncludeSpec(c.Include)+" "+c.Name] = true
	}
	var fresh []Collision
	seen := map[string]bool{}
	for _, c := range s3.collisions() {
		if !old[IncludeSpec(c.Include)+" "+c.Name] && !seen[c.Name] {
			seen[c.Name] = true
			fresh = append(fresh, c)
		}
	}
	if len(fresh) > 0 {
		return out, &CollisionError{What: what, Collisions: fresh}
	}
	return out, nil
}

// movedOrder is the file's forms with ns moved beside at, checked as
// moveForms checks them.
func (e *Editor) movedOrder(at *Node, after bool, ns []*Node) ([]*Node, error) {
	what := fmt.Sprintf("move %d form(s)", len(ns))
	if len(ns) == 0 {
		return nil, fmt.Errorf("%s: nothing to move", what)
	}
	if e.TopForm(at) != at {
		return nil, fmt.Errorf("%s: #%d (%s) is not a top-level form", what, at.ID, label(at))
	}
	moving := map[*Node]bool{}
	for _, n := range ns {
		switch {
		case e.TopForm(n) != n:
			return nil, fmt.Errorf("%s: #%d (%s) is not a top-level form", what, n.ID, label(n))
		case moving[n]:
			return nil, fmt.Errorf("%s: #%d (%s) is named twice", what, n.ID, label(n))
		case n == at:
			return nil, fmt.Errorf("%s: #%d (%s) is the place itself", what, n.ID, label(n))
		}
		moving[n] = true
	}
	var nf []*Node
	for _, f := range e.g.Forms {
		if moving[f] {
			continue
		}
		if f == at && !after {
			nf = append(nf, ns...)
		}
		nf = append(nf, f)
		if f == at && after {
			nf = append(nf, ns...)
		}
	}
	return nf, nil
}

// lostBetween are the uses the headers provide in s0 and not in s1, by
// name and kind, each with its first using form in s1, in s1's order.
func lostBetween(s0, s1 *headerState) []HeaderUse {
	had := map[string]bool{}
	for _, u := range s0.uses(nil) {
		if u.From != nil {
			had[useKey(u)] = true
		}
	}
	var out []HeaderUse
	for _, u := range s1.uses(nil) {
		if u.From == nil && had[useKey(u)] {
			out = append(out, u)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return s1.pos[out[i].First] < s1.pos[out[j].First] })
	return out
}

// crossingRetargets are the edges across the moved forms' edge, the move
// made, that now resolve to another file-scope declaration of their name;
// an edge that resolves to nothing, or to something else, refuses.
func (e *Editor) crossingRetargets(ns []*Node, moving map[*Node]bool) ([]retarget, error) {
	var re []retarget
	seen := map[retarget]bool{}
	in := func(n *Node) bool { return moving[e.TopForm(n)] }
	check := func(u *Node, k int, t *Node) error {
		if !e.inFile(t) || !e.fileScope(t) {
			return nil // a type, an external, a local: it moves with its use
		}
		if isMember(e, t) || isDefForm(t) {
			if e.topIndex(t) > e.topIndex(u) {
				return fmt.Errorf("#%d (%s) would be used at #%d before its definition", t.ID, label(t), u.ID)
			}
			return nil
		}
		name := ordinaryName(t)
		if name == "" {
			return fmt.Errorf("#%d (%s), which #%d refers to, is not a declaration a move can follow", t.ID, label(t), u.ID)
		}
		r := e.resolveAsImported(u, name)
		switch {
		case r == t:
			return nil
		case r != nil && e.fileScope(r):
			if x := (retarget{u, k, r}); !seen[x] {
				seen[x] = true
				re = append(re, x)
			}
			return nil
		case r == nil:
			return fmt.Errorf("`%s` at #%d in %s would resolve to nothing", name, u.ID, inFn(e, u))
		}
		return fmt.Errorf("`%s` at #%d in %s would resolve to #%d (%s), not #%d (%s)", name, u.ID, inFn(e, u), r.ID, label(r), t.ID, label(t))
	}
	var err error
	for _, n := range ns {
		Walk(n, func(x *Node) bool {
			for k, t := range x.Refs {
				if err == nil && !in(t) {
					err = check(x, k, t)
				}
			}
			if err == nil && x.list {
				for _, u := range e.Uses(x) {
					if in(u) {
						continue
					}
					for k, t := range u.Refs {
						if t == x && err == nil {
							err = check(u, k, t)
						}
					}
				}
			}
			return err == nil
		})
		if err != nil {
			return nil, err
		}
	}
	return re, nil
}

// resolveAsImported is Resolve with cc's rule for a function: where the
// file-scope declaration Resolve finds is a prototype naming no parameter
// and a definition of the name naming them is visible from at (above it,
// or the one at is in), the definition.
func (e *Editor) resolveAsImported(at *Node, name string) *Node {
	r := e.Resolve(at, name)
	if r == nil || !r.Is("def") || !e.fileScope(r) || namesParams(defType(r)) {
		return r
	}
	ui := e.topIndex(at)
	for _, d := range e.Decls(name) {
		if d.Is("defn") && e.topIndex(d) <= ui && namesParams(defType(d)) {
			return d
		}
	}
	return r
}

// namesParams says a fn form names at least one of its parameters.
func namesParams(fn *Node) bool {
	if fn == nil || !fn.Is("fn") || len(fn.Kids) < 2 || !fn.Kids[1].list {
		return false
	}
	for _, p := range fn.Kids[1].Kids {
		if p.list && len(p.Kids) >= 2 && !p.Kids[0].list && isIdentText(p.Kids[0].Atom) {
			return true
		}
	}
	return false
}

// macroBasicType is the basic type cc gives the macro name where the
// includes of s provide it -- `int`, `unsigned long` -- in a unit of those
// includes and a declaration of its type alone; "" where it is not a basic
// type or the unit does not check.
func macroBasicType(s *headerState, name string) string {
	var src strings.Builder
	for _, inc := range s.incs {
		if s.hdr[inc].Macros[name] {
			fmt.Fprintf(&src, "#include %s\n", IncludeSpec(inc))
			break
		}
	}
	if src.Len() == 0 {
		return ""
	}
	fmt.Fprintf(&src, "static __typeof__(%s) %s;\n", name, probeName)
	headers.Lock()
	err := initHeaders()
	cfg := headers.cfg
	headers.Unlock()
	if err != nil {
		return ""
	}
	ast, err := cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: "graph-macro-probe.c", Value: src.String()},
	})
	if err != nil {
		return ""
	}
	for _, n := range ast.Scope.Nodes[probeName] {
		if d, ok := n.(*cc.Declarator); ok && d.Type() != nil {
			switch k := d.Type().Kind(); k {
			case cc.Ptr, cc.Array, cc.Function, cc.Struct, cc.Union, cc.Enum, cc.InvalidKind:
				return ""
			default:
				return strings.Join(strings.Fields(k.String()), " ")
			}
		}
	}
	return ""
}
