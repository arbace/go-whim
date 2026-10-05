package graph

import (
	"fmt"
	"strings"
)

// RENAME (doc/GRAPH-MIGRATION.md, B2b): a declaration respelled with every
// use of it, found by the refers edges -- never by its spelling, so that a
// member `term` of one struct is not another's, and a local is not the
// file's object of the same name.
//
//   - Rename(d, to): the declaration and the entity it is one declaration
//     of -- a function's prototypes and definition, an object's
//     declarations; an enumerator; a typedef; a local or a parameter; a
//     struct's or union's member (its selections and its designators
//     `.m`); a struct's, union's or enum's tag (every `(struct TAG)` that
//     refers to it, and its forward declarations); a label (its gotos).  It
//     refuses what would change what a name means: a name already declared
//     where the new one would be (the file's, a member of the same struct,
//     a tag, a label of the same function, a local of the same scope), a
//     use the new name would resolve elsewhere (a local of that name
//     visible there), an outer declaration's use the renamed local would
//     capture, an external (the headers' -- retarget its uses instead), and
//     a use inside a macro's text (`(macro "...")`: respelling it would be a
//     text replacement).
//   - RetargetAs(use, i, to): a use pointed at a declaration of another
//     spelling, and respelled -- what Retarget refuses -- checked to
//     resolve there to what it now names; RetargetUses every use of an
//     entity at once (phase 36's `strlen` to `musl_strlen`).
//   - RespellString(s, spelling): a string literal written anew.  THE RULE
//     FOR STRINGS: a string is never renamed because it contains a name.
//     A cut respells exactly the literals it names, each whole: the verb
//     finds the literals spelled exactly as given (quotes and escapes
//     included), counts them against the number the cut expects, and
//     writes each anew -- an explicit opt-in list, never a text
//     replacement across literals.  Its array type changes with its length:
//     a typed edge above it that is an array is cleared and listed in
//     Untyped -- none on the importer's graphs, which type a literal's
//     parenthesis as the pointer it decays to, as cc does, even under
//     sizeof.
//
// A respelling keeps every node's id: the declaration and its uses are the
// same nodes, with the same edges, spelled anew.  It is logged as one act
// (`rename`, `respell`) whose Moved are the ids changed in place.

// Renamed is what a rename respelled.
type Renamed struct {
	Decls []*Node // the declarations respelled
	Uses  []*Node // the uses respelled
}

// declAtom is the atom of d that spells the name it declares, with the
// name's place: a def's, defn's or typedef's name; an enumerator's,
// member's or parameter's first element; a tag's; a label's.
func declAtom(d *Node) *Node {
	switch {
	case !d.list:
		return nil
	case d.Is("def") || d.Is("defn") || d.Is("typedef"):
		if i := defNameAt(d); i > 0 {
			return d.Kids[i]
		}
		return nil
	case d.Is("struct") || d.Is("union") || d.Is("enum"):
		if tagOf(d) != "" {
			return d.Kids[1]
		}
		return nil
	case d.Is("label"):
		if len(d.Kids) == 2 && !d.Kids[1].list {
			return d.Kids[1]
		}
		return nil
	}
	if len(d.Kids) > 0 && !d.Kids[0].list {
		return d.Kids[0]
	}
	return nil
}

// isExtern says d is a node of the externals section.
func (e *Editor) isExtern(d *Node) bool {
	for x := d; x != nil; x = x.up {
		if x == e.top[2] {
			return true
		}
	}
	return false
}

// declKind says what d declares, for the rules: "file" (a file-scope
// ordinary name), "local", "param", "enumerator", "member", "tag",
// "label", or "" for what Rename does not take.
func (e *Editor) declKind(d *Node) string {
	p := e.Parent(d)
	switch {
	case !e.Live(d) || !e.inFile(d):
		return ""
	case d.Is("label"):
		return "label"
	case d.Is("struct") || d.Is("union") || d.Is("enum"):
		if tagOf(d) == "" || !isDefForm(d) {
			return ""
		}
		return "tag"
	case e.IsEnumerator(d):
		return "enumerator"
	case p != nil && (p.Is("struct") || p.Is("union")):
		return "member"
	case d.Is("def") || d.Is("defn") || d.Is("typedef"):
		if p == nil {
			return "file"
		}
		return "local"
	case p != nil && e.Parent(p) != nil && e.Parent(p).Is("fn") && e.Function(d) != nil:
		if f := e.Function(d); f != nil && defType(f) == e.Parent(p) {
			return "param"
		}
	}
	return ""
}

// Rename respells the declaration d, the entity it is one declaration of,
// and every use of them, to.
func (e *Editor) Rename(d *Node, to string) (*Renamed, error) {
	kind := e.declKind(d)
	at := declAtom(d)
	if kind == "" || at == nil {
		return nil, fmt.Errorf("rename #%d (%s): not a declaration Rename takes (an external is the headers': retarget its uses)", d.ID, label(d))
	}
	old := at.Atom
	if !isIdent(to) {
		return nil, fmt.Errorf("rename %s: `%s` is not an identifier", old, to)
	}
	if to == old {
		return nil, fmt.Errorf("rename %s: it is spelled so already", old)
	}
	// the entity: every declaration of it
	decls := []*Node{d}
	switch kind {
	case "file":
		decls = nil
		typedef := d.Is("typedef") || hasPrefix(d, "typedef")
		for _, f := range e.g.Forms {
			if topName(f) == old && (f.Is("typedef") || hasPrefix(f, "typedef")) == typedef {
				decls = append(decls, f)
			}
		}
	case "tag":
		for _, f := range e.g.Forms {
			if f.Is(d.Head()) && f != d && tagOf(f) == old && !isDefForm(f) && f.Ref() == nil {
				decls = append(decls, f) // a forward declaration that refers to nothing
			}
		}
	}
	// the uses: by edge
	var uses []*Node
	seen := map[*Node]bool{}
	for _, x := range decls {
		for _, u := range e.Uses(x) {
			if !seen[u] {
				seen[u] = true
				uses = append(uses, u)
			}
		}
	}
	for _, u := range uses {
		if u.list && !(kind == "tag" && u.Is(d.Head()) && len(u.Kids) == 2 && !u.Kids[1].list && u.Kids[1].Atom == old) {
			return nil, fmt.Errorf("rename %s: #%d (%s) names it in text a respelling cannot reach", old, u.ID, label(u))
		}
		if !u.list && u.Atom != old && u.Atom != "."+old {
			return nil, fmt.Errorf("rename %s: the use #%d is spelled `%s`", old, u.ID, u.Atom)
		}
	}
	if err := e.renameClash(kind, d, decls, uses, old, to); err != nil {
		return nil, fmt.Errorf("rename %s to %s: %w", old, to, err)
	}
	// apply
	r := &Renamed{}
	act := Act{Op: "rename"}
	for _, x := range decls {
		e.g.save(declAtom(x))
		declAtom(x).Atom = to
		r.Decls = append(r.Decls, x)
		act.Moved = append(act.Moved, x.ID)
	}
	for _, u := range uses {
		switch {
		case u.list:
			e.g.save(u.Kids[1])
			u.Kids[1].Atom = to
		case strings.HasPrefix(u.Atom, "."):
			e.g.save(u)
			u.Atom = "." + to
		default:
			e.g.save(u)
			u.Atom = to
		}
		r.Uses = append(r.Uses, u)
		act.Moved = append(act.Moved, u.ID)
	}
	e.Log = append(e.Log, act)
	return r, nil
}

// renameClash refuses a rename that would change what a name means.
func (e *Editor) renameClash(kind string, d *Node, decls, uses []*Node, old, to string) error {
	mine := map[*Node]bool{}
	for _, x := range decls {
		mine[x] = true
	}
	switch kind {
	case "file", "enumerator":
		if ds := e.Decls(to); len(ds) > 0 {
			return fmt.Errorf("%s is declared at file scope already (#%d)", to, ds[0].ID)
		}
		// a block-scope declaration of the same entity would keep the old name
		for _, f := range e.g.Forms {
			if !f.Is("defn") {
				continue
			}
			for _, x := range Find(f, func(n *Node) bool { return n.Is("def") && hasPrefix(n, "extern") && topName(n) == old }) {
				return fmt.Errorf("#%d declares %s again inside %s", x.ID, old, topName(f))
			}
		}
		for _, u := range uses {
			if r := e.Resolve(u, to); r != nil && !mine[r] {
				return fmt.Errorf("the use #%d in %s would name #%d (%s), declared there", u.ID, label(e.Function(u)), r.ID, label(r))
			}
		}
	case "local", "param":
		f := e.Function(d)
		scope := e.Parent(d) // a local's block (or function body), a parameter's function
		if kind == "param" {
			scope = f
			if paramNamed(f, to) != nil {
				return fmt.Errorf("%s has a parameter %s already", topName(f), to)
			}
		}
		// the same scope
		var items []*Node
		switch {
		case scope.Is("defn"):
			items = Body(scope)
		case scope.Is("block") || scope.Is("stmt-expr"):
			items = scope.Kids[1:]
		}
		for _, x := range items {
			if (x.Is("def") || x.Is("typedef")) && topName(x) == to {
				return fmt.Errorf("#%d declares %s in the same scope", x.ID, to)
			}
		}
		if kind == "local" && e.Parent(d) == f {
			if paramNamed(f, to) != nil {
				return fmt.Errorf("%s has a parameter %s, in the same scope", topName(f), to)
			}
		}
		// an inner declaration of the new name would take a use
		for _, u := range uses {
			if r := e.Resolve(u, to); r != nil && !mine[r] && e.within(r, map[*Node]bool{scope: true}) {
				return fmt.Errorf("the use #%d would name #%d (%s), declared inside the scope", u.ID, r.ID, label(r))
			}
		}
		// an outer declaration's use inside the scope would be taken
		var err error
		Walk(scope, func(n *Node) bool {
			if err != nil || n.list || n.Ref() == nil || mine[n.Ref()] || n.Atom != to {
				return err == nil
			}
			if t := n.Ref(); ordinaryName(t) == to && !e.within(t, map[*Node]bool{scope: true}) {
				err = fmt.Errorf("the use #%d of #%d (%s) is in the scope, and would name the renamed one", n.ID, t.ID, label(t))
			}
			return true
		})
		if err != nil {
			return err
		}
	case "member":
		// an anonymous struct's or union's members are its container's
		top := e.Parent(d)
		for tagOf(top) == "" {
			m := e.Parent(top)
			if m == nil || len(m.Kids) != 1 {
				break
			}
			s := e.Parent(m)
			if s == nil || !s.Is("struct") && !s.Is("union") {
				break
			}
			top = s
		}
		if m := memberNamed(top, to); m != nil {
			return fmt.Errorf("#%d is a member %s of the same %s", m.ID, to, top.Head())
		}
	case "tag":
		for _, kw := range []string{"struct", "union", "enum"} {
			if t := e.tagDef(kw, to); t != nil {
				return fmt.Errorf("#%d is the %s %s, and tags share one name space", t.ID, kw, to)
			}
		}
	case "label":
		f := e.Function(d)
		if ls := Find(f, func(n *Node) bool { return n.Is("label") && len(n.Kids) == 2 && n.Kids[1].Atom == to }); len(ls) > 0 {
			return fmt.Errorf("%s has a label %s already", topName(f), to)
		}
	}
	return nil
}

// RetargetAs points use's i-th refers edge at to, and respells use with
// to's name: refused where that name, written there, would name something
// else.
func (e *Editor) RetargetAs(use *Node, i int, to *Node) error {
	if use.list {
		return fmt.Errorf("retarget #%d: a list is not respelled", use.ID)
	}
	at := declAtom(to)
	name := ordinaryName(to)
	member := false
	if p := e.Parent(to); p != nil && (p.Is("struct") || p.Is("union")) && at != nil {
		name, member = at.Atom, true
	}
	if name == "" {
		return fmt.Errorf("retarget #%d to #%d (%s): it declares no name a use spells", use.ID, to.ID, label(to))
	}
	if !member {
		r := e.Resolve(use, name)
		if r != nil && r != to && sameEntity(r, to) {
			to = r // the import's edge: a use refers to the first declaration
		}
		if r != to {
			what := "nothing"
			if r != nil {
				what = fmt.Sprintf("#%d (%s)", r.ID, label(r))
			}
			return fmt.Errorf("retarget #%d to #%d: `%s` written there names %s", use.ID, to.ID, name, what)
		}
	}
	spelled := use.Atom
	if strings.HasPrefix(spelled, ".") {
		e.g.save(use)
		use.Atom = "." + name
	} else {
		e.g.save(use)
		use.Atom = name
	}
	if err := e.Retarget(use, i, to); err != nil {
		e.g.save(use)
		use.Atom = spelled
		return err
	}
	e.Log[len(e.Log)-1].Op = "retarget-as"
	return nil
}

// sameEntity says a and b are top-level declarations of one name: a
// function's prototype and its definition.
func sameEntity(a, b *Node) bool {
	return a.list && b.list && topName(a) != "" && topName(a) == topName(b) &&
		(a.Is("def") || a.Is("defn")) && (b.Is("def") || b.Is("defn"))
}

// RetargetUses points every use of from (and of the declarations it is one
// of, at file scope) at to, respelled: checked first, all or none.
func (e *Editor) RetargetUses(from, to *Node) ([]*Node, error) {
	decls := []*Node{from}
	if e.Parent(from) == nil && topName(from) != "" && !e.isExtern(from) {
		decls = e.FileDecls(topName(from))
	}
	var uses []*Node
	for _, d := range decls {
		uses = append(uses, e.Uses(d)...)
	}
	name := ordinaryName(to)
	for _, u := range uses {
		if u.list {
			return nil, fmt.Errorf("retarget the uses of #%d: #%d (%s) names it in text a respelling cannot reach", from.ID, u.ID, label(u))
		}
		if r := e.Resolve(u, name); r != to && !(r != nil && sameEntity(r, to)) {
			return nil, fmt.Errorf("retarget the uses of #%d: `%s` written at #%d names another declaration", from.ID, name, u.ID)
		}
	}
	for _, u := range uses {
		for i, r := range u.Refs {
			for _, d := range decls {
				if r == d {
					if err := e.RetargetAs(u, i, to); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return uses, nil
}

// IsStringLiteral says the atom n is a string literal of the file's C, not
// the text of an include, a macro or an attribute.
func (e *Editor) IsStringLiteral(n *Node) bool {
	if n.list || !isStringAtom(n.Atom) || !e.Live(n) {
		return false
	}
	for p := e.Parent(n); p != nil; p = e.Parent(p) {
		if verbatimHeads[p.Head()] {
			return false
		}
	}
	return true
}

// stringToken says s is one string literal token: an encoding prefix, a
// quote, characters and escapes, the closing quote.
func stringToken(s string) bool {
	i := 0
	for i < len(s) && s[i] != '"' {
		switch s[i] {
		case 'L', 'u', 'U', '8':
		default:
			return false
		}
		i++
	}
	if i >= len(s) || i > 2 {
		return false
	}
	for i++; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i == len(s)-1
		case '\n':
			return false
		}
	}
	return false
}

// RespellString writes the string literal s as spelling, a whole literal,
// quotes included.
func (e *Editor) RespellString(s *Node, spelling string) error {
	switch {
	case !e.IsStringLiteral(s):
		return fmt.Errorf("respell #%d (%s): not a string literal in the graph", s.ID, label(s))
	case !stringToken(spelling):
		return fmt.Errorf("respell %s: %s is not one string literal", s.Atom, spelling)
	}
	e.g.save(s)
	s.Atom = spelling
	// the arrays it was, above it
	for q := e.Parent(s); q != nil && q.Type != nil && q.Type.Is("array"); q = e.Parent(q) {
		e.g.save(q)
		q.Type = nil
		e.untype(q)
	}
	id := s.ID
	if id == 0 {
		if p := e.Parent(s); p != nil {
			id = p.ID
		}
	}
	e.Log = append(e.Log, Act{Op: "respell", Moved: []ID{id}})
	return nil
}

// ---- the verbs

// Rename respells the declaration of name the scope holds -- the
// function's local or parameter in InFunction, else the file's (one
// entity) -- and every use (RENAME).
func (v *Verbs) Rename(name, to, what string) *Renamed {
	if v.Err != nil {
		return nil
	}
	var d *Node
	if v.scope != nil && v.scope.Is("defn") {
		ds := Find(v.scope, func(n *Node) bool { return (n.Is("def") || n.Is("typedef")) && topName(n) == name })
		if p := paramNamed(v.scope, name); p != nil {
			ds = append(ds, p)
		}
		if len(ds) != 1 {
			v.Die("%s -- %s declares %s %d times, expected 1", what, topName(v.scope), name, len(ds))
			return nil
		}
		d = ds[0]
	} else {
		ds := v.e.Decls(name)
		if len(ds) == 0 {
			v.Die("%s -- %s is not declared at file scope", what, name)
			return nil
		}
		d = ds[0]
	}
	r, err := v.e.Rename(d, to)
	if err != nil {
		v.Die("%s -- %v", what, err)
		return nil
	}
	v.Say(what)
	return r
}

// RespellString writes each of the n string literals of the scope spelled
// exactly old (a whole literal, quotes included) as new: the explicit list
// RENAME's rule for strings asks for.
func (v *Verbs) RespellString(old, new string, n int, what string) {
	if v.Err != nil {
		return
	}
	var ms []*Node
	for _, s := range v.Strings() {
		if s.Atom == old {
			ms = append(ms, s)
		}
	}
	if len(ms) != n {
		v.Die("%s -- the literal %s occurs %d times, expected %d", what, old, len(ms), n)
		return
	}
	if v.each(ms, what, func(s *Node) error { return v.e.RespellString(s, new) }) {
		v.Say(what)
	}
}
