package graph

import (
	"sort"
	"strings"
)

// THE UNWRITTEN SEED (doc/GRAPH-MIGRATION.md, FOLDX; xform.FallOut's seeds
// on the graph).  A cut deletes the code that wrote something -- an option's
// arm, a command's handler -- and leaves its readers.  What nothing writes
// any more is found by the write edges: a use whose place stores to it (an
// assignment's left side, `++`, `--`), counted in the code the program's
// roots reach and in the file's initialisers, by the name it spells, as
// xform does.  Two kinds:
//
//   - an OBJECT: a file-scope scalar, defined, its address never taken, at
//     most one initialiser and that one a constant, nothing live writing
//     its name: each read is that value (zero without an initialiser);
//   - a MEMBER: one no live code writes by its name (no store, `&`, `++`,
//     designator names it), of a struct every instance of which is a
//     static object without an initialiser, reached only through pointers
//     to it that come from such an object, a null constant or a memset to
//     zero: each read is zero.
//
// Unwritten is the set; FoldX's seeds are what one leaves that was not so
// before it.  The analysis asks the typed edges (a conversion's two types,
// a call's parameters) where they are, and the forms' own types where an
// edit cleared them: what step 6's checker will say for every node.

// Unwritten is what nothing writes: objects by name, with their value, and
// members by their key, `S.m` (S the struct's typedef name, or `struct
// TAG`).
type Unwritten struct {
	Objects map[string]int64
	Members map[string]bool
}

// Names is every object's name and member's key, sorted.
func (u *Unwritten) Names() []string {
	var out []string
	for k := range u.Objects {
		out = append(out, k)
	}
	for k := range u.Members {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Has says name is an unwritten object or member key.
func (u *Unwritten) Has(name string) bool {
	if u == nil {
		return false
	}
	if _, ok := u.Objects[name]; ok {
		return true
	}
	return u.Members[name]
}

// Unwritten is what nothing the roots reach writes (main when roots is
// empty; every function when the file has none of them).
func (e *Editor) Unwritten(roots []string) *Unwritten {
	x := newXFold(e, FoldX{Roots: roots})
	x.index()
	return x.unwritten()
}

// ---- what is live, and who names whom

// isOrdinary says r, a refers edge's target, is what an identifier in an
// expression names: an object, a function, a parameter, an enumerator, an
// external -- not a member, a label, a typedef or a tag.
func (x *xfold) isOrdinary(r *Node) bool {
	switch {
	case r.Is("defn"):
		return true
	case r.Is("def"):
		return !hasPrefix(r, "typedef")
	case r.Is("extern"), r.Is("extern-enumerator"), r.Is("undeclared"):
		return true
	}
	p := x.e.Parent(r)
	if p == nil {
		return false
	}
	switch {
	case p.Is("enum"):
		return r.list && len(r.Kids) > 0 && !r.Kids[0].list
	case !p.list || p.Head() != "":
		return false
	}
	// a parameter: an element of a function type's parameter list
	q := x.e.Parent(p)
	return q != nil && (q.Is("fn") || q.Is("fn-ids")) && q.Kids[1] == p && r.list && len(r.Kids) >= 1 && !r.Kids[0].list
}

// identUse is the name an atom spells when it is an identifier in an
// expression, or "".
func (x *xfold) identUse(n *Node) string {
	if n.list || len(n.Refs) != 1 || !x.isOrdinary(n.Refs[0]) {
		return ""
	}
	return n.Atom
}

// isParam says d is a function's parameter.
func (x *xfold) isParam(d *Node) bool {
	if d.Is("def") || d.Is("defn") || !d.list {
		return false
	}
	p := x.e.Parent(d)
	if p == nil || p.Head() != "" {
		return false
	}
	q := x.e.Parent(p)
	return q != nil && (q.Is("fn") || q.Is("fn-ids"))
}

// index finds the functions, who names and calls them, and what the roots
// reach.
func (x *xfold) index() {
	x.fns = map[string]*Node{}
	x.fnList = nil
	for _, f := range x.e.g.Forms {
		if f.Is("defn") {
			nm := declName(f)
			if x.fns[nm] == nil {
				x.fns[nm] = f
				x.fnList = append(x.fnList, f)
			}
		}
	}
	x.mentions = map[string]int{}
	x.calls = map[string][]*Node{}
	x.caller = map[*Node]*Node{}
	refs := map[*Node]map[string]bool{} // nil: the file scope
	for _, f := range x.e.g.Forms {
		var in *Node
		root := f
		if f.Is("defn") {
			in = f
		}
		Walk(root, func(n *Node) bool {
			if n.list {
				if n.Is("call") && in != nil {
					if c := n.Kids[1]; !c.list && x.fns[x.identUse(c)] != nil {
						nm := c.Atom
						x.calls[nm] = append(x.calls[nm], n)
						x.caller[n] = in
					}
				}
				return true
			}
			nm := x.identUse(n)
			if nm == "" || x.fns[nm] == nil {
				return true
			}
			x.mentions[nm]++
			if refs[in] == nil {
				refs[in] = map[string]bool{}
			}
			refs[in][nm] = true
			return true
		})
	}
	// what the roots reach, and what a file-scope initialiser names
	x.live = map[*Node]bool{}
	roots := x.x.Roots
	if len(roots) == 0 {
		roots = []string{"main"}
	}
	var work []string
	for _, r := range roots {
		if x.fns[r] != nil {
			work = append(work, r)
		}
	}
	if len(work) == 0 {
		for _, f := range x.fnList {
			x.live[f] = true
		}
		return
	}
	for nm := range refs[nil] {
		work = append(work, nm)
	}
	for len(work) > 0 {
		nm := work[len(work)-1]
		work = work[:len(work)-1]
		f := x.fns[nm]
		if f == nil || x.live[f] {
			continue
		}
		x.live[f] = true
		for r := range refs[f] {
			work = append(work, r)
		}
	}
}

// liveWalk walks the file-scope forms and the live functions: where a
// write is one that can happen.
func (x *xfold) liveWalk(fn func(*Node) bool) {
	for _, f := range x.e.g.Forms {
		if f.Is("defn") && !x.live[f] {
			continue
		}
		Walk(f, fn)
	}
}

// lvalueName is the name an assignment's left side writes through
// parentheses, or "".
func (x *xfold) lvalueName(n *Node) string {
	for n.Is("paren") && len(n.Kids) == 2 {
		n = n.Kids[1]
	}
	if n.list || len(n.Refs) != 1 {
		return ""
	}
	r := n.Refs[0]
	if r.Is("def") || r.Is("defn") || x.isParam(r) {
		return n.Atom
	}
	return ""
}

// addrTaken says the use u has its address taken: `&u`, `&u.m`, `&u[i]` of
// an array, through parentheses and casts (cc's takeAddr).
func (x *xfold) addrTaken(u *Node) bool {
	n := u
	for {
		p := x.e.Parent(n)
		if p == nil {
			return false
		}
		switch {
		case p.Is("paren"):
		case p.Is(".") && p.Kids[1] == n:
		case p.Is("index") && p.Kids[1] == n && !x.typeNode(n).Is("pointer"):
		case p.Is("cast") && p.Kids[len(p.Kids)-1] == n:
		case p.Is("addr"):
			return true
		default:
			return false
		}
		n = p
	}
}

// isScalarType says a type node is an integer, a floating, an enum or a
// pointer type.
func isScalarType(t *Node) bool {
	switch {
	case t == nil:
		return false
	case t.Is("pointer"), t.Is("enum"), t.Is("extern-enum"):
		return true
	case t.Is("basic"):
		return !(len(t.Kids) == 2 && (t.Kids[1].Atom == "void" || t.Kids[1].Atom == "invalid"))
	}
	return false
}

// volatileForm says a type form says volatile.
func volatileForm(t *Node) bool {
	found := false
	if t != nil {
		Walk(t, func(n *Node) bool {
			found = found || !n.list && n.Atom == "volatile"
			return !found
		})
	}
	return found
}

// unwritten is the analysis on the graph as it stands.
func (x *xfold) unwritten() *Unwritten {
	return &Unwritten{Objects: x.unwrittenObjects(), Members: x.unwrittenMembers()}
}

// unwrittenObjects is every file-scope scalar object nothing writes, with
// its value.
func (x *xfold) unwrittenObjects() map[string]int64 {
	writes := map[string]int{}
	x.liveWalk(func(n *Node) bool {
		if !n.list {
			return true
		}
		h := n.Head()
		switch {
		case assignOps[h] && len(n.Kids) == 3:
			if nm := x.lvalueName(n.Kids[1]); nm != "" {
				writes[nm]++
			}
		case incDec[h] && len(n.Kids) == 2:
			if nm := x.lvalueName(n.Kids[1]); nm != "" {
				writes[nm]++
			}
		}
		return true
	})
	type agg struct {
		decls        []*Node
		inits        int
		value        int64
		bad, defined bool
	}
	byName := map[string]*agg{}
	var order []string
	for _, f := range x.e.g.Forms {
		if !f.Is("def") || hasPrefix(f, "typedef") {
			continue
		}
		t := f.Type
		if t.Is("function") || defType(f).Is("fn") {
			continue
		}
		nm := declName(f)
		a := byName[nm]
		if a == nil {
			a = &agg{}
			byName[nm] = a
			order = append(order, nm)
		}
		a.decls = append(a.decls, f)
		if !isScalarType(t) || volatileForm(defType(f)) {
			a.bad = true
		}
		v := defValue(f)
		if !hasPrefix(f, "extern") || v != nil {
			a.defined = true
		}
		if v != nil {
			a.inits++
			if v.Is("init") {
				a.bad = true
				continue
			}
			c, _, ok := x.xconst(v)
			if !ok {
				a.bad = true
			}
			a.value = c.v
		}
	}
	out := map[string]int64{}
	for _, nm := range order {
		a := byName[nm]
		if a.bad || !a.defined || a.inits > 1 || writes[nm] != 0 {
			continue
		}
		taken := false
		for _, d := range a.decls {
			for _, u := range x.e.Uses(d) {
				taken = taken || x.addrTaken(u)
			}
		}
		if !taken {
			out[nm] = a.value
		}
	}
	return out
}

// ---- members

// structKey names a struct form for a member's key: its typedef's name,
// or `struct TAG`.
func (x *xfold) structKey(s *Node) string {
	if k, ok := x.skeys[s]; ok {
		return k
	}
	k := ""
	if p := x.e.Parent(s); p != nil && (p.Is("typedef") || p.Is("def") && hasPrefix(p, "typedef")) && defType(p) == s {
		k = declName(p)
	} else if tag := tagOf(s); tag != "" {
		k = "struct " + tag
	} else {
		// cc's spelling of an anonymous struct's type: its members
		var ms []string
		for _, m := range members(s) {
			ms = append(ms, declName(m)+" "+typeString(m.Type))
		}
		k = "struct {" + strings.Join(ms, "; ") + "}"
	}
	x.skeys[s] = k
	return k
}

// typeString is a type node as cc spells a member's type in a struct's,
// as far as the keys need: the basic types' words.
func typeString(t *Node) string {
	if t.Is("basic") {
		var ws []string
		for _, k := range t.Kids[1:] {
			ws = append(ws, k.Atom)
		}
		return strings.Join(ws, " ")
	}
	if t.Is("pointer") {
		return "*" + typeString(pointee(t))
	}
	return "?"
}

// memberOwner is the struct form a member node belongs to, nil for a
// union's or an external's.
func (x *xfold) memberOwner(m *Node) *Node {
	if m == nil {
		return nil
	}
	p := x.e.Parent(m)
	if p == nil || !p.Is("struct") {
		return nil
	}
	return p
}

// memberKey is a member node's key, or "".
func (x *xfold) memberKey(m *Node) string {
	s := x.memberOwner(m)
	if s == nil {
		return ""
	}
	k := x.structKey(s)
	if k == "" {
		return ""
	}
	return k + "." + declName(m)
}

// selected are the member atoms a selection chain names, in order: `(-> p
// a b)` names a and b.
func selected(n *Node) []*Node {
	if !(n.Is("->") || n.Is(".")) || len(n.Kids) < 3 {
		return nil
	}
	return n.Kids[2:]
}

// lhsMember is the member an lvalue writes: the last one a selection names,
// through parentheses and indexing.
func lhsMember(n *Node) string {
	for {
		switch {
		case n.Is("paren") && len(n.Kids) == 2:
			n = n.Kids[1]
		case n.Is("index") && len(n.Kids) >= 2:
			n = n.Kids[1]
		case n.Is("->") || n.Is("."):
			ms := selected(n)
			if len(ms) == 0 {
				return ""
			}
			return memberAtomName(ms[len(ms)-1])
		default:
			return ""
		}
	}
}

// memberAtomName is the name a member atom spells.
func memberAtomName(a *Node) string {
	if a.list {
		return ""
	}
	return a.Atom
}

// unwrittenMembers is every member nothing writes, of a struct whose
// instances are all zero to begin with and reached only as themselves.
func (x *xfold) unwrittenMembers() map[string]bool {
	written := map[string]bool{}
	reads := map[string]bool{}
	x.liveWalk(func(n *Node) bool {
		if !n.list {
			return true
		}
		h := n.Head()
		switch {
		case assignOps[h] && len(n.Kids) == 3:
			if m := lhsMember(n.Kids[1]); m != "" {
				written[m] = true
			}
		case (incDec[h] || h == "addr") && len(n.Kids) == 2:
			if m := lhsMember(n.Kids[1]); m != "" {
				written[m] = true
			}
		case h == "at":
			for _, d := range n.Kids[1 : len(n.Kids)-1] {
				if !d.list && strings.HasPrefix(d.Atom, ".") {
					written[d.Atom[1:]] = true
				}
			}
		case h == "->" || h == ".":
			for _, a := range selected(n) {
				if k := x.memberKey(a.Ref()); k != "" {
					reads[k] = true
				}
			}
		}
		return true
	})
	cands := map[string]bool{}
	owners := map[*Node]bool{}
	byKey := map[string]*Node{}
	for _, s := range x.e.g.Forms {
		Walk(s, func(n *Node) bool {
			if n.Is("struct") && isDefForm(n) {
				if k := x.structKey(n); k != "" {
					byKey[k] = n
				}
			}
			return true
		})
	}
	for k := range reads {
		i := strings.LastIndexByte(k, '.')
		if !written[k[i+1:]] {
			cands[k] = true
			if s := byKey[k[:i]]; s != nil {
				owners[s] = true
			}
		}
	}
	if len(cands) == 0 {
		return cands
	}
	bad := x.unsafeOwners(owners)
	for k := range cands {
		if s := byKey[k[:strings.LastIndexByte(k, '.')]]; s == nil || bad[s] {
			delete(cands, k)
		}
	}
	return cands
}

// typeNode is an expression's type node: its typed edge, or, where an edit
// cleared it, what its form says.
func (x *xfold) typeNode(n *Node) *Node {
	if n == nil {
		return nil
	}
	if !n.list {
		if d := n.Ref(); d != nil {
			return d.Type
		}
		return nil
	}
	if n.Type != nil {
		return n.Type
	}
	args := n.Args()
	switch h := n.Head(); {
	case h == "paren" && len(args) == 1:
		return x.typeNode(args[0])
	case h == "?" && len(args) == 3:
		if t := x.typeNode(args[1]); t != nil {
			return t
		}
		return x.typeNode(args[2])
	case h == "comma" && len(args) > 0:
		return x.typeNode(args[len(args)-1])
	case (assignOps[h] || incDec[h]) && len(args) > 0:
		return x.typeNode(args[0])
	case h == "call" && len(args) > 0:
		return resultType(x.typeNode(args[0]))
	case (h == "deref" || h == "index") && len(args) > 0:
		return pointee(x.typeNode(args[0]))
	case (h == "->" || h == ".") && len(args) > 1:
		if d := args[len(args)-1].Ref(); d != nil {
			return d.Type
		}
	}
	return nil
}

// ptrTo says the expression n (or the type t, when n is nil) is a pointer
// to the struct s, an array of them decaying to one.
func (x *xfold) ptrTo(n, t, s *Node) bool {
	if n != nil {
		if n.Is("addr") && len(n.Kids) == 2 {
			return x.typeNode(n.Kids[1]) == s
		}
		t = x.typeNode(n)
	}
	if t == nil || !(t.Is("pointer") || n != nil && t.Is("array")) {
		return false
	}
	return pointee(t) == s
}

// ownerOf is the owner the type t is, holds, or is an array of.
func (x *xfold) ownerOf(t *Node, owners map[*Node]bool) *Node {
	strip := func(t *Node) *Node {
		for t != nil && t.Is("array") {
			t = pointee(t)
		}
		return t
	}
	t = strip(t)
	if t == nil || !(t.Is("struct") || t.Is("union")) {
		return nil
	}
	if owners[t] {
		return t
	}
	if t.Is("struct") {
		for _, m := range members(t) {
			if ft := strip(m.Type); ft != nil && ft.Is("struct") && owners[ft] {
				return ft
			}
		}
	}
	return nil
}

// unsafeOwners is each owner an instance of which may start other than
// zero, or be written other than member by member.
func (x *xfold) unsafeOwners(owners map[*Node]bool) map[*Node]bool {
	bad := map[*Node]bool{}
	zero := func(n *Node) bool {
		if n == nil {
			return false
		}
		v, _, ok := x.xconst(n)
		return ok && v.v == 0
	}
	conv := func(to *Node, from *Node, src *Node, memsetZero bool) {
		for s := range owners {
			pt := x.ptrTo(nil, to, s)
			pf := x.ptrTo(from, nil, s)
			switch {
			case pt == pf:
			case pf && memsetZero:
			case pt && zero(src):
			default:
				bad[s] = true
			}
		}
	}
	decl := func(d *Node, static, init, param bool) {
		if s := x.ownerOf(d.Type, owners); s != nil && (!static || init || param) {
			bad[s] = true
		}
	}
	var fn *Node
	visit := func(n *Node, top bool) bool {
		if !n.list {
			return true
		}
		args := n.Args()
		switch h := n.Head(); {
		case h == "def" && !hasPrefix(n, "typedef"):
			static := top || hasPrefix(n, "static") || hasPrefix(n, "extern")
			v := defValue(n)
			decl(n, static, v != nil, false)
			if v != nil && !v.Is("init") {
				conv(n.Type, v, v, false)
			}
		case h == "fn" && len(args) == 2 && args[0].list:
			for _, p := range args[0].Kids {
				if p.list && len(p.Kids) >= 2 && !p.Kids[0].list && p.Head() != "" && !isAttrForm(p) {
					decl(p, false, false, true)
				}
			}
		case h == "struct" || h == "union":
			for _, m := range members(n) {
				if s := x.ownerOf(m.Type, owners); s != nil {
					bad[s] = true // held inside another
				}
			}
		case assignOps[h] && len(args) == 2:
			if s := x.ownerOf(x.typeNode(n), owners); s != nil {
				bad[s] = true // a whole struct assigned
			}
			if h == "=" {
				conv(x.typeNode(args[0]), args[1], args[1], false)
			}
		case h == "cast" && len(args) == 2:
			conv(n.Type, args[1], args[1], false)
		case h == "literal":
			if s := x.ownerOf(n.Type, owners); s != nil {
				bad[s] = true
			}
		case h == "call" && len(args) > 0:
			for s := range owners {
				if x.ptrTo(n, nil, s) {
					bad[s] = true // an instance from somewhere else
				}
			}
			ct := x.typeNode(args[0])
			if ct.Is("pointer") {
				ct = pointee(ct)
			}
			var ps []*Node
			if ct.Is("function") && len(ct.Kids) == 3 {
				ps = ct.Kids[1].Kids
			}
			as := args[1:]
			memset := !args[0].list && args[0].Atom == "memset" && len(as) == 3 && zero(as[1])
			for i, a := range as {
				var to *Node
				if i < len(ps) && !(!ps[i].list && ps[i].Atom == "...") {
					to = ps[i].Type
				}
				conv(to, a, a, memset && i == 0)
			}
		case h == "return" && len(args) == 1 && fn != nil:
			conv(resultType(fn.Type), args[0], args[0], false)
		}
		return true
	}
	for _, f := range x.e.g.Forms {
		fn = nil
		if f.Is("defn") {
			fn = f
		}
		Walk(f, func(n *Node) bool { return visit(n, n == f) })
	}
	return bad
}
