package graph

import (
	"fmt"
	"strconv"
	"strings"
)

// THE TYPE RE-CHECK (doc/GRAPH.md, step 6; doc/GRAPH-MIGRATION.md, *Step6
// as built*).  An edit keeps the typed edges right where it can tell, and
// otherwise clears them and lists the expressions in Untyped: unknown,
// never wrong (edit.go, TYPES).  Recheck gives them back, from the graph's
// own type nodes and C's rules as crefactor/cc's check applies them -- so
// that a graph an edit leaves is the import of its C view, typed edges and
// all, and the next phase is handed what an import would give it:
//
//   - an expression's type from its operands': the integer promotions and
//     the usual arithmetic conversions (an enum by its underlying type, a
//     bit-field narrower than int an int), pointer arithmetic (a pointer and
//     an integer the pointer, two pointers ptrdiff_t), a comparison or a
//     logical operator int, an assignment its left side's, `?:` by C's six
//     cases, a call its callee's result, a selection its member's, `&x`,
//     `*p`, `p[i]`, a cast its type form's, sizeof and alignof size_t, the
//     comma its last operand's; an operand's type DECAYED as cc's check
//     decays it -- an array a pointer to its element, a function a pointer
//     to it -- everywhere but under `&` (and the parentheses or commas
//     between), which is where cc's check stores a decayed type too;
//   - a literal's: an integer constant's by its suffix and size, a
//     character constant int, a string an array of char, `nullptr`, `true`
//     and `false` the graph's types for them;
//   - a declaration's from its type form (B2c's formType), an array's size
//     from its initialiser, the other declarations of an array an
//     initialiser completes given the definition's type, and the
//     expressions above the uses of a declaration whose type moved typed
//     again;
//   - a typed edge to a struct, union or enum definition that is not in the
//     graph (FRAG writing a struct anew: B3f's 63 and 72, 96) retargeted to
//     the definition that now stands for it -- its tag's, its typedef's, the
//     anonymous one of its members -- with the uses of it and its members;
//     one with none (a typedef's uses retargeted to another's: 63's
//     bt_regprog_T) types again what it typed, through any type node;
//   - a `(paren ...)` an edit kept where the C view writes the parentheses
//     anyway taken out: the import has no node there.
//
// It looks where the edits wrote (Editor.touched: every node Written was
// given, which the closures consume and this does not) and at the
// Untyped list, not at the whole file.  What it cannot derive -- a
// macro's invocation, a `_Generic`, an operand it cannot type, what a
// header's macro spells -- keeps the type it had or stays listed.  The new
// type nodes are interned (the graph's of that structure, else made) and
// committed in one `type` act; a parenthesis taken out is a replace act,
// a use retargeted a retarget act; no other id moves.

// RecheckStats is what Recheck did.
type RecheckStats struct {
	Stale   int // typed edges to a removed definition, and its tag's uses, retargeted
	Unparen int // parentheses kept as a node where the C view writes them anyway, taken out
	Typed   int // expressions and declarations typed again
	Left    int // listed still: what it could not derive
}

func (s RecheckStats) String() string {
	return fmt.Sprintf("%d stale edges retargeted, %d parentheses taken out, %d typed again, %d left untyped",
		s.Stale, s.Unparen, s.Typed, s.Left)
}

// Recheck types again what the edits so far left untyped, and retargets
// the typed edges to definitions they removed.
func (e *Editor) Recheck() RecheckStats {
	var st RecheckStats
	e.s6walked = nil
	defer func() { e.s6walked = nil }()
	tx := e.typeTx()
	var redo []*Node
	st.Stale, redo = e.s6RetargetStale(tx)
	st.Unparen = e.s6Unparen()
	rc := &s6checker{e: e, tx: tx, todo: map[*Node]bool{}, done: map[*Node]bool{}, old: map[*Node]*Node{}}
	var list []*Node
	add := func(n *Node) {
		if n.list && n.Type == nil && !rc.todo[n] && e.Live(n) {
			rc.todo[n] = true
			list = append(list, n)
		}
	}
	for _, n := range e.Untyped {
		add(n)
	}
	var decls []*Node
	seen := map[*Node]bool{}
	for _, n := range redo {
		if rc.declForm(n) != nil {
			seen[n] = true
			decls = append(decls, n)
		} else if isExprForm(n) {
			add(n)
		}
	}
	e.s6pass++
	for _, w := range e.s6Touched() {
		e.s6Walk(w, func(x *Node) bool {
			if x.list && x.Type == nil && rc.typedInImport(x) {
				add(x)
			}
			return true
		})
	}
	e.s6pass++
	for _, w := range e.s6Touched() {
		e.s6Walk(w, func(x *Node) bool {
			if x.list && (x.Type == nil || x.Is("def") || x.Is("defn")) && !seen[x] && rc.declForm(x) != nil {
				seen[x] = true
				decls = append(decls, x)
			}
			return true
		})
		// a declaration above what was written: a table's rows changed,
		// and with them, maybe, its size
		for q := e.Parent(w); q != nil; q = e.Parent(q) {
			if !seen[q] && (q.Type == nil || q.Is("def") || q.Is("defn")) && rc.declForm(q) != nil {
				seen[q] = true
				decls = append(decls, q)
			}
		}
	}
	retype := func(d, t *Node) {
		if d.Type != t {
			d.Type = t
			e.typed(d)
			st.Typed++
		}
		// the expressions above its uses were typed by its old type
		for _, u := range e.Uses(d) {
			for q := e.Parent(u); q != nil && isExprForm(q) && !rc.todo[q]; q = e.Parent(q) {
				if q.Type != nil {
					rc.old[q] = q.Type
					q.Type = nil
				}
				add(q)
			}
		}
	}
	for _, d := range decls {
		t := rc.declType(d)
		if t == nil {
			continue
		}
		if t != d.Type || t.Is("array") && d.Is("def") {
			retype(d, t) // an array's rows may have changed its size
		}
	}
	// the file's declarations of an array an initialiser completes have the
	// definition's type, as cc's check gives them -- after any edit of the
	// rows (INITROW types the definition alone)
	defined := map[string]*Node{}
	for _, f := range e.g.Forms {
		if f.Is("def") && f.Type.Is("array") && len(f.Type.Kids) == 3 && len(f.Kids) > defNameAt(f)+2 {
			defined[topName(f)] = f
		}
	}
	for _, f := range e.g.Forms {
		if f.Is("def") && f.Type.Is("array") {
			if d := defined[topName(f)]; d != nil && d != f && f.Type != d.Type {
				if ft := rc.declType(f); ft != nil && ft.Is("array") && len(ft.Kids) == 2 {
					retype(f, d.Type)
				}
			}
		}
	}
	for _, n := range list {
		if t := rc.ensure(n); t != nil {
			st.Typed++
		}
	}
	tx.commit()
	out := e.Untyped[:0]
	for _, u := range e.Untyped {
		if e.Live(u) && u.Type == nil {
			out = append(out, u)
		}
	}
	e.Untyped = out
	st.Left = len(out)
	return st
}

// s6RetargetStale retargets every typed edge into a struct, union or enum
// definition that is not in the graph -- one an edit removed, or one of a
// fragment's own unit that FRAG carried a type over to -- and every use of
// it and of its members, to the file's definition that stands for it now:
// the one of its tag, or, for an anonymous one, the one its typedef's name
// now defines.  A typed edge to a pointer, array or function type node the
// types section does not hold is made one to the graph's node of that
// structure (interned in tx).
func (e *Editor) s6RetargetStale(tx *typeTx) (int, []*Node) {
	n := 0
	held := map[*Node]bool{}
	for _, t := range e.g.Types {
		held[t] = true
	}
	isStale := func(t *Node) bool { return t != nil && isDefForm(t) && !e.Live(t) }
	stale := map[*Node]*Node{} // a definition not in the graph -> its stand-in, or nil
	var order []*Node
	var deep func(t *Node, seen map[*Node]bool)
	deep = func(t *Node, seen map[*Node]bool) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		if isStale(t) {
			if _, ok := stale[t]; !ok {
				stale[t] = nil
				order = append(order, t)
			}
			return
		}
		if held[t] || isDefForm(t) || !t.list {
			return
		}
		Walk(t, func(x *Node) bool {
			if x != t {
				deep(x.Type, seen)
			}
			return true
		})
	}
	named := map[*Node]string{} // an anonymous definition a removed typedef held: its name
	for _, r := range e.removed {
		Walk(r, func(x *Node) bool {
			if x.Is("typedef") {
				if t := defType(x); t != nil && isDefForm(t) {
					named[t] = topName(x)
				}
			}
			if isStale(x) {
				deep(x, map[*Node]bool{})
			}
			return true
		})
	}
	var users []*Node // typed edges the scan saw going out of the graph
	scan := func(x *Node) bool {
		if t := x.Type; t != nil && (isStale(t) || t.list && !held[t] && !e.Live(t)) {
			users = append(users, x)
			deep(t, map[*Node]bool{})
		}
		return true
	}
	for _, t := range e.g.Types {
		Walk(t, func(x *Node) bool {
			if x != t {
				scan(x)
			}
			return true
		})
	}
	e.s6pass++
	for _, w := range e.s6Touched() {
		e.s6Walk(w, scan)
	}
	if len(order) == 0 && len(users) == 0 {
		return 0, nil
	}
	defs := map[string]*Node{}
	for _, f := range e.g.Forms {
		if f.Is("typedef") {
			if t := defType(f); t != nil && isDefForm(t) {
				if k := "typedef " + topName(f); defs[k] == nil {
					defs[k] = t
				}
			}
		}
		Walk(f, func(x *Node) bool {
			if isDefForm(x) && tagOf(x) != "" {
				k := x.Head() + " " + tagOf(x)
				if defs[k] == nil {
					defs[k] = x
				}
			}
			if isDefForm(x) && tagOf(x) == "" {
				k := s6Shape(x)
				if _, ok := defs[k]; ok {
					defs[k] = nil // not one: no stand-in by shape
				} else {
					defs[k] = x
				}
			}
			return !x.Is("defn")
		})
	}
	for _, d := range order {
		var to *Node
		if tag := tagOf(d); tag != "" {
			to = defs[d.Head()+" "+tag]
		} else if name := named[d]; name != "" {
			to = defs["typedef "+name]
		} else {
			for x := d.up; x != nil && to == nil; x = x.up {
				if x.Is("typedef") {
					to = defs["typedef "+topName(x)] // a fragment's unit's
				}
			}
			// replaced inside its typedef, which stays: the typedef's own
			// typed edge names it
			for _, u := range e.typedBy[d] {
				if to == nil && u.Type == d && u.Is("typedef") {
					if e.Live(u) && isDefForm(defType(u)) {
						to = defType(u)
					} else {
						to = defs["typedef "+topName(u)] // the typedef that held it, gone
					}
				}
			}
		}
		if to == nil && tagOf(d) == "" {
			to = defs[s6Shape(d)] // the one anonymous definition of its members
		}
		if to != nil && to.Head() == d.Head() {
			stale[d] = to
		}
	}
	// canon is t as the graph holds it
	memo := map[*Node]*Node{}
	var canon func(t *Node) *Node
	canon = func(t *Node) *Node {
		if t == nil {
			return nil
		}
		if c, ok := memo[t]; ok {
			return c
		}
		memo[t] = t
		var c *Node
		switch {
		case isStale(t):
			c = stale[t] // nil: no stand-in
		case isDefForm(t) || !t.list || isExtern(t) || t.Is("basic"):
			c = t
		default:
			k := NewList()
			lost := false
			for _, x := range t.Kids {
				switch {
				case x.list && x.Type == nil && x.Head() == "": // a function's parameters
					ps := NewList()
					for _, p := range x.Kids {
						if p.Type != nil {
							pt := canon(p.Type)
							lost = lost || pt == nil
							ps.Kids = append(ps.Kids, &Node{Type: pt})
						} else {
							ps.Kids = append(ps.Kids, NewAtom(p.Atom))
						}
					}
					k.Kids = append(k.Kids, ps)
				case x.Type != nil:
					xt := canon(x.Type)
					lost = lost || xt == nil
					k.Kids = append(k.Kids, &Node{Type: xt})
				default:
					k.Kids = append(k.Kids, NewAtom(x.Atom))
				}
			}
			same := len(k.Kids) == len(t.Kids)
			for i := 0; same && i < len(k.Kids); i++ {
				a, b := k.Kids[i], t.Kids[i]
				if a.list && a.Head() == "" && b.list {
					same = len(a.Kids) == len(b.Kids)
					for j := 0; same && j < len(a.Kids); j++ {
						same = a.Kids[j].Type == b.Kids[j].Type
					}
				} else {
					same = a.Type == b.Type
				}
			}
			switch {
			case lost:
			case same && held[t]:
				c = t
			default:
				c = tx.intern(k)
			}
		}
		memo[t] = c
		return c
	}
	for d, to := range stale {
		if to == nil {
			continue
		}
		users = append(users, e.typedBy[d]...)
		e.typedBy[d] = nil
		retarget := func(from, to *Node, uses []*Node) {
			for _, u := range uses {
				if !e.Live(u) {
					continue
				}
				for i, x := range u.Refs {
					if x == from && e.Retarget(u, i, to) == nil {
						n++
					}
				}
			}
		}
		retarget(d, to, e.usesOf(d))
		for _, m := range members(d) {
			if name := s6MemberName(m); name != "" {
				if mt := memberNamed(to, name); mt != nil {
					retarget(m, mt, e.usesOf(m))
				}
			}
		}
	}
	// a definition with no stand-in (its typedef's uses retargeted to
	// another's): what is typed with it, through any type node, is typed
	// again from its form or its operands, so every typed edge is asked
	lostAny := false
	for _, to := range stale {
		lostAny = lostAny || to == nil
	}
	if lostAny {
		for _, f := range e.g.Forms {
			Walk(f, func(x *Node) bool {
				if x.Type != nil && canon(x.Type) != x.Type {
					users = append(users, x)
				}
				return true
			})
		}
	}
	var redo []*Node
	done := map[*Node]bool{}
	for _, u := range users {
		if done[u] {
			continue
		}
		done[u] = true
		c := canon(u.Type)
		switch {
		case c == nil && e.Live(u) && !e.s6InTypes(u):
			u.Type = nil
			redo = append(redo, u)
			n++
		case c == nil:
			// a type node's operand: what it types goes with it
		case c != nil && c != u.Type:
			u.Type = c
			if isDefForm(c) {
				e.typedBy[c] = append(e.typedBy[c], u)
			}
			n++
		}
	}
	return n, redo
}

// s6Unparen takes out, in what the edits wrote, the parentheses an edit
// kept as a `(paren ...)` where the C view writes them anyway (precedence
// asks for them there): the import of that C has no such node.
func (e *Editor) s6Unparen() int {
	var ps []*Node
	e.s6pass++
	for _, w := range e.s6Touched() {
		e.s6Walk(w, func(x *Node) bool {
			if x.Is("paren") && len(x.Kids) == 2 && x.Kids[1].list && isExpr(x.Kids[1]) && !x.Kids[1].Is("paren") {
				ps = append(ps, x)
			}
			return true
		})
	}
	n := 0
	seen := map[*Node]bool{}
	for _, x := range ps {
		if seen[x] || !e.Live(x) {
			continue
		}
		seen[x] = true
		p, i := e.index(x)
		if p == nil || e.isTop(p) || !isExpr(p) && !p.Is("def") && !p.Is("init") && !p.Is("at") && !p.Is("return") {
			continue
		}
		want := placeLevel(p, i)
		if q := e.Parent(p); q != nil && q.Is("enum") {
			want = xlvCond
		}
		in := x.Kids[1]
		if formLevel(in) >= want || p.Is("sizeof") || p.Is("alignof") {
			continue
		}
		t := x.Type
		if e.Replace(x, in) == nil {
			if in.Type == nil {
				in.Type = t
			}
			n++
		}
	}
	return n
}

type s6checker struct {
	e          *Editor
	tx         *typeTx
	todo, done map[*Node]bool
	old        map[*Node]*Node // a typed edge cleared to be derived again
	xf         *xfold
	under      map[*Node]string
}

// typedInImport says the import gives the list n a typed edge: an
// expression's form (not a type form's parentheses), or a declaration.
func (rc *s6checker) typedInImport(n *Node) bool {
	if isExprForm(n) {
		return !n.Is("paren") || !rc.inTypeForm(n)
	}
	return false
}

// inTypeForm says n stands where a type form is read.
func (rc *s6checker) inTypeForm(n *Node) bool {
	p := rc.e.Parent(n)
	if p == nil {
		return false
	}
	switch p.Head() {
	case "ptr", "paren", "name-attr", "atomic":
		return p.Kids[1] == n && (p.Head() != "paren" || rc.inTypeForm(p))
	case "array":
		return p.Kids[len(p.Kids)-1] == n
	case "fn":
		return true
	case "cast", "sizeof-type", "alignof-type", "literal", "typeof-type":
		return p.Kids[1] == n
	case "def", "typedef", "defn":
		return defType(p) == n
	}
	return rc.e.declAbove(n) != nil
}

// decays says the expression n is checked as cc checks an operand that
// decays: everywhere but under `&`, through parentheses and commas.
func (rc *s6checker) decays(n *Node) bool {
	for {
		p := rc.e.Parent(n)
		if p == nil {
			return true
		}
		switch p.Head() {
		case "addr":
			return false
		case "paren":
			n = p
			continue
		case "comma":
			if p.Kids[len(p.Kids)-1] == n {
				n = p
				continue
			}
		}
		return true
	}
}

// decay is t as an operand that decays sees it.
func (rc *s6checker) decay(t *Node) *Node {
	switch {
	case t.Is("array"):
		return rc.tx.pointer(pointee(t))
	case t.Is("function"):
		return rc.tx.pointer(t)
	}
	return t
}

// ensure types n when it is to be typed, its operands first.
func (rc *s6checker) ensure(n *Node) *Node {
	if !rc.todo[n] || rc.done[n] {
		return n.Type
	}
	rc.done[n] = true
	for _, k := range n.Kids {
		if k.list && rc.todo[k] {
			rc.ensure(k)
		}
	}
	t := rc.derive(n)
	if t == nil {
		t = rc.old[n] // what it cannot derive keeps the type it had
	}
	if t != nil {
		n.Type = t
		rc.e.typed(n)
	}
	return t
}

// operand is the type cc's check gives the operand k, decayed or not.
func (rc *s6checker) operand(k *Node, decay bool) *Node {
	if k.list {
		if rc.todo[k] {
			return rc.ensure(k)
		}
		return k.Type
	}
	t := rc.atom(k)
	if t != nil && decay {
		t = rc.decay(t)
	}
	return t
}

// atom is an identifier's or a literal's type, undecayed.
func (rc *s6checker) atom(k *Node) *Node {
	if d := k.Ref(); d != nil {
		if len(k.Refs) > 1 || s6NameOf(d) != k.Atom {
			return nil // a macro's token: its edges are its expansion's names
		}
		if p := rc.e.Parent(d); p != nil && p.Is("enum") && s6Within(rc.e, k, p) {
			// an enumerator named in its own enum's definition: cc's check
			// types it as it stands then -- its value's type, or long long
			if v := enumValue(d); v != nil {
				return rc.operand(v, true)
			}
			return rc.basic("long", "long")
		}
		return d.Type
	}
	s := k.Atom
	switch {
	case s == "":
		return nil
	case s[0] == '"' || strings.HasPrefix(s, "u8\""):
		return rc.tx.array(strconv.Itoa(s6StringLen(s)), rc.basic("char"))
	case s[0] == '\'':
		return rc.basic("int")
	case s == "true" || s == "false":
		return rc.basic("_Bool")
	case s == "nullptr":
		return rc.tx.pointer(rc.basic("void"))
	case s[0] >= '0' && s[0] <= '9' || s[0] == '.':
		if _, t, ok := litType(s); ok {
			if strings.Contains(strings.ToLower(s), "ll") {
				if t.signed {
					return rc.basic("long", "long")
				}
				return rc.basic("unsigned", "long", "long")
			}
			return rc.xbasic(t)
		}
		return rc.floatLit(s)
	}
	return nil
}

// basic is the graph's (basic ...) of cc's words, made where it has none.
func (rc *s6checker) basic(words ...string) *Node {
	if t := basicType(rc.e.g, words); t != nil {
		return t
	}
	n := NewList(NewAtom("basic"))
	for _, w := range words {
		n.Kids = append(n.Kids, NewAtom(w))
	}
	return rc.tx.intern(n)
}

func (rc *s6checker) xbasic(t xtype) *Node {
	switch {
	case t == xInt:
		return rc.basic("int")
	case t == xUInt:
		return rc.basic("unsigned")
	case t == xLong:
		return rc.basic("long")
	case t == xULong:
		return rc.basic("unsigned", "long")
	}
	return nil
}

func (rc *s6checker) floatLit(s string) *Node {
	l := strings.ToLower(s)
	switch {
	case strings.HasPrefix(l, "0x") && !strings.ContainsAny(l, "p"):
		return nil
	case strings.HasSuffix(l, "f"):
		return rc.basic("float")
	case strings.HasSuffix(l, "l"):
		return rc.basic("long", "double")
	}
	return rc.basic("double")
}

// s6StringLen is a string literal's array length: its bytes, escapes
// decoded, and the NUL.
func s6StringLen(s string) int {
	s = strings.TrimPrefix(s, "u8")
	if v, err := strconv.Unquote(s); err == nil {
		return len(v) + 1
	}
	n := 0
	for i := 1; i < len(s)-1; i++ {
		if s[i] != '\\' {
			n++
			continue
		}
		i++
		switch {
		case s[i] >= '0' && s[i] <= '7':
			for j := 0; j < 2 && i+1 < len(s)-1 && s[i+1] >= '0' && s[i+1] <= '7'; j++ {
				i++
			}
		case s[i] == 'x':
			for i+1 < len(s)-1 && strings.IndexByte("0123456789abcdefABCDEF", s[i+1]) >= 0 {
				i++
			}
		}
		n++
	}
	return n + 1
}

// kind is a type node's arithmetic kind in cc's words: an enum's its
// underlying type's; "" for what is not arithmetic.
func (rc *s6checker) kind(t *Node) string {
	switch {
	case t == nil:
		return ""
	case t.Is("basic"):
		var ws []string
		for _, k := range t.Kids[1:] {
			ws = append(ws, k.Atom)
		}
		return strings.Join(ws, " ")
	case t.Is("enum") || t.Is("extern-enum"):
		return rc.underlying(t)
	}
	return ""
}

// underlying is an enum's underlying type, as cc's check chooses it: by
// its enumerators' least and greatest values.
func (rc *s6checker) underlying(t *Node) string {
	if !t.Is("enum") {
		return "unsigned"
	}
	if k, ok := rc.under[t]; ok {
		return k
	}
	k := rc.underlying1(t)
	if rc.under == nil {
		rc.under = map[*Node]string{}
	}
	rc.under[t] = k
	return k
}

func (rc *s6checker) underlying1(t *Node) string {
	if rc.xf == nil {
		rc.xf = &xfold{e: rc.e, enumVals: map[*Node]int64{}, val: map[*Node]int64{}}
	}
	x := rc.xf
	var lo int64
	var hi uint64
	for _, m := range body(t) {
		if !m.list || len(m.Kids) == 0 || m.Is("@") {
			continue
		}
		v, ok := x.enumerator(m)
		if !ok {
			return "int"
		}
		if v < lo {
			lo = v
		}
		if v > 0 && uint64(v) > hi {
			hi = uint64(v)
		}
	}
	switch {
	case lo >= -1<<31 && hi <= 1<<31-1:
		return "int"
	case lo >= 0 && hi <= 1<<32-1:
		return "unsigned"
	case hi < 1<<63-1:
		return "long"
	}
	return "unsigned long"
}

var s6ranks = map[string]int{
	"_Bool": 1, "char": 2, "signed char": 2, "unsigned char": 2, "short": 3, "unsigned short": 3,
	"int": 4, "unsigned": 4, "long": 5, "unsigned long": 5, "long long": 6, "unsigned long long": 6,
}

var s6sizes = map[string]int{
	"_Bool": 1, "char": 1, "signed char": 1, "unsigned char": 1, "short": 2, "unsigned short": 2,
	"int": 4, "unsigned": 4, "long": 8, "unsigned long": 8, "long long": 8, "unsigned long long": 8,
}

func s6signed(k string) bool {
	return k == "char" || k == "signed char" || k == "short" || k == "int" || k == "long" || k == "long long"
}

func s6isFloat(k string) bool {
	return k == "float" || k == "double" || k == "long double" || strings.HasPrefix(k, "_Float")
}

// promote is cc's IntegerPromotion: an enum unchanged.
func (rc *s6checker) promote(t *Node) *Node {
	if t.Is("basic") {
		switch rc.kind(t) {
		case "_Bool", "char", "signed char", "unsigned char", "short", "unsigned short":
			return rc.basic("int")
		}
	}
	return t
}

// usual is cc's UsualArithmeticConversions.
func (rc *s6checker) usual(a, b *Node) *Node {
	ak, bk := rc.kind(a), rc.kind(b)
	if ak == "" || bk == "" {
		return nil
	}
	if a.Is("enum") || a.Is("extern-enum") {
		a = rc.basic(strings.Fields(ak)...)
	}
	if b.Is("enum") || b.Is("extern-enum") {
		b = rc.basic(strings.Fields(bk)...)
	}
	for _, f := range []string{"long double", "double", "float"} {
		if ak == f {
			return a
		}
		if bk == f {
			return b
		}
	}
	if s6isFloat(ak) || s6isFloat(bk) {
		return nil
	}
	pa, pb := rc.promote(a), rc.promote(b)
	ak, bk = rc.kind(pa), rc.kind(pb)
	switch {
	case ak == bk:
		return pa
	case s6signed(ak) == s6signed(bk):
		if s6ranks[ak] > s6ranks[bk] {
			return pa
		}
		return pb
	}
	u, s, uk, sk := pb, pa, bk, ak
	if !s6signed(ak) {
		u, s, uk, sk = pa, pb, ak, bk
	}
	if s6ranks[uk] > s6ranks[sk] {
		return u
	}
	if s6sizes[sk] > s6sizes[uk] {
		return s
	}
	switch sk {
	case "int":
		return rc.basic("unsigned")
	case "long":
		return rc.basic("unsigned", "long")
	case "long long":
		return rc.basic("unsigned", "long", "long")
	}
	return nil
}

func s6isPointer(t *Node) bool { return t.Is("pointer") }

func (rc *s6checker) arithmetic(t *Node) bool {
	k := rc.kind(t)
	return s6ranks[k] > 0 || s6isFloat(k)
}

func (rc *s6checker) integer(t *Node) bool { return s6ranks[rc.kind(t)] > 0 }

// member is a selection's type: its member's declared type, a bit-field
// narrower than int an int, as cc's fieldType gives it.
func (rc *s6checker) member(m *Node) *Node {
	d := m.Ref()
	if d == nil || len(m.Refs) > 1 || m.list || s6MemberName(d) != m.Atom {
		return nil // a member macro's: its edges are its expansion's names
	}
	t := d.Type
	if w := s6BitWidth(d); w >= 0 && w < 32 && t.Is("basic") {
		return rc.basic("int")
	}
	return t
}

// s6BitWidth is a member's bit-field width, -1 when it is none.
func s6BitWidth(m *Node) int {
	for _, k := range m.Kids[1:] {
		if k.Is("bits") && len(k.Kids) == 2 && !k.Kids[1].list {
			if v, err := strconv.Atoi(k.Kids[1].Atom); err == nil {
				return v
			}
		}
	}
	return -1
}

// derive is the type cc's check gives the expression n, from its operands.
func (rc *s6checker) derive(n *Node) *Node {
	t := rc.derive1(n)
	if t != nil && rc.decays(n) {
		t = rc.decay(t)
	}
	return t
}

func (rc *s6checker) derive1(n *Node) *Node {
	args := n.Args()
	op := func(i int) *Node {
		if i >= len(args) {
			return nil
		}
		return rc.operand(args[i], true)
	}
	h := n.Head()
	switch h {
	case "==", "!=", "<", ">", "<=", ">=", "&&", "||", "!":
		return rc.basic("int")
	case "paren":
		return rc.operand(args[0], rc.decays(n))
	case "comma":
		return rc.operand(args[len(args)-1], rc.decays(n))
	case "call":
		return resultType(op(0))
	case "addr":
		t := rc.operand(args[0], false)
		if t == nil {
			return nil
		}
		return rc.tx.pointer(t)
	case "deref":
		return pointee(op(0))
	case "index":
		t := op(0)
		for i := range args[1:] {
			if !s6isPointer(t) {
				return nil
			}
			t = pointee(t)
			if i < len(args)-2 || true {
				t = rc.decay(t)
			}
		}
		return t
	case "->", ".":
		return rc.member(args[len(args)-1])
	case "post++", "post--", "pre++", "pre--":
		return op(0)
	case "cast":
		if len(args) == 2 {
			return rc.tx.formType(args[0], false)
		}
		return nil
	case "literal":
		return rc.tx.formType(args[0], false)
	case "sizeof", "sizeof-bare", "sizeof-type", "alignof", "alignof-bare", "alignof-type":
		return rc.basic("unsigned", "long")
	case "label-addr":
		return rc.tx.pointer(rc.basic("void"))
	case "-", "+":
		if len(args) == 1 {
			t := op(0)
			if t == nil || !rc.arithmetic(t) {
				return nil
			}
			return rc.promote(t)
		}
		t := op(0)
		for i := 1; i < len(args) && t != nil; i++ {
			t = rc.additive(h, t, op(i))
		}
		return t
	case "~":
		t := op(0)
		if t == nil || !rc.integer(t) {
			return nil
		}
		return rc.promote(t)
	case "*", "/", "%", "&", "|", "^":
		t := op(0)
		for i := 1; i < len(args) && t != nil; i++ {
			t = rc.usual(t, op(i))
		}
		return t
	case "<<", ">>":
		t := op(0)
		if t == nil || !rc.integer(t) {
			return nil
		}
		return rc.promote(t)
	case "?":
		var t2, t3 *Node
		if len(args) == 2 {
			t2, t3 = op(0), op(1)
		} else {
			t2, t3 = op(1), op(2)
		}
		return rc.conditional(t2, t3)
	case "stmt-expr":
		if len(args) == 0 {
			return rc.basic("void")
		}
		last := args[len(args)-1]
		if last.list && isExprForm(last) {
			return rc.operand(last, true)
		}
		if !last.list {
			return rc.operand(last, true)
		}
		return rc.basic("void")
	}
	if assignOps[h] && len(args) == 2 {
		return op(0)
	}
	return nil
}

func (rc *s6checker) additive(h string, a, b *Node) *Node {
	if a == nil || b == nil {
		return nil
	}
	switch {
	case rc.arithmetic(a) && rc.arithmetic(b):
		return rc.usual(a, b)
	case h == "-" && s6isPointer(a) && s6isPointer(b):
		return rc.basic("long")
	case s6isPointer(a) && rc.integer(b):
		return a
	case h == "+" && rc.integer(a) && s6isPointer(b):
		return b
	}
	return nil
}

// conditional is `?:`'s type, cc's six cases.
func (rc *s6checker) conditional(t2, t3 *Node) *Node {
	if t2 == nil || t3 == nil {
		return nil
	}
	switch {
	case rc.arithmetic(t2) && rc.arithmetic(t3):
		return rc.usual(t2, t3)
	case (t2.Is("struct") || t2.Is("union")) && t2.Head() == t3.Head():
		return t2
	case rc.kind(t2) == "void" && rc.kind(t3) == "void":
		return t2
	case s6isPointer(t2) && s6isPointer(t3):
		return t2
	case s6isPointer(t2) && rc.integer(t3):
		return t2
	case rc.integer(t2) && s6isPointer(t3):
		return t3
	case rc.kind(t2) == "void":
		return t2
	case rc.kind(t3) == "void":
		return t3
	}
	return nil
}

// s6NameOf is the name a declaration's form declares: a def's, a
// parameter's, a member's, an enumerator's, an external's.
func s6NameOf(d *Node) string {
	if isExtern(d) {
		if len(d.Kids) > 1 && !d.Kids[1].list {
			return d.Kids[1].Atom
		}
		return ""
	}
	return declName(d)
}

// s6MemberName is a member form's name: the file's `(NAME TYPE ...)`, an
// external struct's `(member NAME)`.
func s6MemberName(m *Node) string {
	if m.Is("member") && len(m.Kids) > 1 {
		return m.Kids[1].Atom
	}
	if m.list && len(m.Kids) > 0 && !m.Kids[0].list {
		return m.Kids[0].Atom
	}
	return ""
}

// s6Within says n is inside p.
func s6Within(e *Editor, n, p *Node) bool {
	for x := e.Parent(n); x != nil; x = e.Parent(x) {
		if x == p {
			return true
		}
	}
	return false
}

// declForm is the type form of the declaration d -- a def, a typedef, a
// function's definition, a member, a parameter -- or nil when d is none.
func (rc *s6checker) declForm(d *Node) *Node {
	switch d.Head() {
	case "def", "typedef", "defn":
		return defType(d)
	}
	p := rc.e.Parent(d)
	switch {
	case p == nil:
		return nil
	case isParamList(p):
		if !d.list || paramName(d) == "" {
			return nil // an unnamed parameter: no declarator, no typed edge
		}
		return paramTypeForm(d)
	case isMemberForm(rc.e, d) && len(d.Kids) > 1 && !d.Kids[0].list:
		return d.Kids[1]
	}
	return nil
}

// declType is the type a declaration's form names; an array of no size
// completed by its initialiser's elements.
func (rc *s6checker) declType(d *Node) *Node {
	form := rc.declForm(d)
	p := rc.e.Parent(d)
	t := rc.tx.formType(form, p != nil && isParamList(p))
	if t == nil || !t.Is("array") || len(t.Kids) != 2 || !d.Is("def") {
		return t
	}
	init := d.Kids[len(d.Kids)-1]
	if init == form {
		return t
	}
	switch {
	case init.Is("init"):
		n, k := 0, 0
		for _, x := range init.Args() {
			if x.Is("at") && len(x.Kids) > 2 && x.Kids[1].Is("idx") && len(x.Kids[1].Kids) >= 2 {
				v, err := strconv.Atoi(x.Kids[1].Kids[len(x.Kids[1].Kids)-1].Atom)
				if err != nil {
					return nil
				}
				k = v
			}
			k++
			n = max(n, k)
		}
		return rc.tx.array(strconv.Itoa(n), pointee(t))
	case !init.list && strings.HasPrefix(init.Atom, "\""):
		return rc.tx.array(strconv.Itoa(s6StringLen(init.Atom)), pointee(t))
	}
	return t
}

// s6Shape is an anonymous definition's members by name, in order.
func s6Shape(d *Node) string {
	var b strings.Builder
	b.WriteString("shape " + d.Head())
	for _, m := range body(d) {
		b.WriteString(" " + s6MemberName(m))
	}
	return b.String()
}

// s6InTypes says n is in the types section: a type node's operand.
func (e *Editor) s6InTypes(n *Node) bool {
	for x := n; x != nil; x = x.up {
		if x == e.top[1] {
			return true
		}
		if e.isTop(x) {
			return false
		}
	}
	return false
}

// s6Touched are the live nodes the edits wrote, each once.
func (e *Editor) s6Touched() []*Node {
	seen := map[*Node]bool{}
	var out []*Node
	for _, w := range e.touched {
		if !seen[w] && e.Live(w) {
			seen[w] = true
			out = append(out, w)
		}
	}
	e.touched = out
	return out
}

// s6Walk walks n as Walk does, but not into what an earlier call of the
// same pass walked (the roots s6Touched gives nest).
func (e *Editor) s6Walk(n *Node, f func(*Node) bool) {
	if e.s6walked == nil {
		e.s6walked = map[*Node]int{}
	}
	pass := e.s6pass
	Walk(n, func(x *Node) bool {
		if e.s6walked[x] == pass {
			return false
		}
		e.s6walked[x] = pass
		return f(x)
	})
}
