package graph

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// STRUCT LOCALS AS THEIR MEMBERS' LOCALS (doc/GRAPH-MIGRATION.md, Step6;
// crefactor/xform's StructScalar, phase 100's second step, on the graph):
// `pos_T a = *pp; a.col += dc; *pp = a;` becomes
//
//	typeof(((pos_T *)0)->lnum) a_lnum = (*pp).lnum; typeof(((pos_T *)0)->col) a_col = (*pp).col;
//	a_col += dc;
//	{ (*pp).lnum = a_lnum; (*pp).col = a_col; }
//
// A target with no struct values -- Java, Clojure -- makes a local struct an
// object, allocated each time its declaration runs and copied field by
// field; the members as locals are neither.  C's copy of a struct is its
// members' copies, so this is exact where the struct is used only so: a
// local -- not static, not a parameter, an item of a block -- of a struct
// (named by a typedef or a tag) whose members are all named scalars or
// pointers, and every use of it one of
//
//   - s.m, not its address;
//   - s = E or E = s as a statement, E another such struct, or an
//     expression that does nothing (no call, no store), read once per
//     member where C read it once;
//   - its declaration's value: such an E, or braces of values by
//     position (the rest zero);
//   - the value of another such struct's declaration.
//
// Everything is asked of the graph: the declarations' and the members'
// types by their typed edges, the uses by their refers edges.  The new C --
// the members' declarations, the names in the selections' places, the
// copies member by member -- is made by FRAG in one synthesized import, so
// that it is typed as an import types it; what a rewrite holds that another
// rewrites (a selection inside a copied expression) is printed with it.

// ssUse2 is one use of a candidate struct local.
type ssUse2 struct {
	kind  string // "member", "set" (s = E), "store" (E = s), "init" (S t = s)
	node  *Node  // the selection, or the assignment
	other *Node  // E
	peer  *Node  // t, when E is another struct: s = t, t = s, S t = s
	stmt  *Node  // the statement an assignment is
	wonly bool   // a member use that only stores (s.m = E;): not a read
	field int    // a member use's member
}

// StructScalars rewrites, in each function definition in says yes to (all
// when nil), its struct locals of scalars as their members' locals, and
// says how many it took.
func (e *Editor) StructScalars(in func(*Node) bool) (int, error) {
	var fns []*Node
	for _, f := range e.g.Forms {
		if f.Is("defn") && (in == nil || in(f)) {
			fns = append(fns, f)
		}
	}
	var src []byte
	mentioned := func(name string) bool {
		if src == nil {
			var forms []*Node
			for _, f := range e.g.Forms {
				if in == nil || in(f) {
					forms = append(forms, f)
				}
			}
			b, err := FormsC(forms)
			if err != nil {
				b = []byte{}
			}
			src = b
		}
		return wordIn(string(src), name)
	}
	rw := &ssRewrites{text: map[*Node]func() string{}}
	n := 0
	for _, f := range fns {
		n += e.ssFunction(f, rw, mentioned)
	}
	if n == 0 {
		return 0, nil
	}
	// the outermost rewrites, made; one inside another is printed with it
	var fs []Frag
	var gone []*Node
	for _, x := range rw.order {
		nested := false
		for q := e.Parent(x); q != nil; q = e.Parent(q) {
			if rw.text[q] != nil {
				nested = true
				break
			}
		}
		if nested {
			continue
		}
		s := rw.text[x]()
		if s == "" {
			gone = append(gone, x)
			continue
		}
		fs = append(fs, Frag{At: e.SpotOf(x), Src: s})
	}
	for _, f := range fs {
		if f.At.err != nil {
			return 0, f.At.err
		}
	}
	if _, err := e.SpliceC(fs...); err != nil {
		return 0, err
	}
	for _, x := range gone {
		if err := e.Delete(x); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// ssRewrites are the rewrites by the node each replaces, in the order
// found: the first for a node is the one kept.
type ssRewrites struct {
	text  map[*Node]func() string
	order []*Node
}

func (r *ssRewrites) add(x *Node, f func() string) {
	if r.text[x] != nil {
		return
	}
	r.text[x] = f
	r.order = append(r.order, x)
}

// render is x's C with the rewrites at or under it applied.
func (r *ssRewrites) render(x *Node) string {
	var sub func(n *Node) *clisp.Node
	sub = func(n *Node) *clisp.Node {
		if f := r.text[n]; f != nil {
			return clisp.A(f())
		}
		if !n.list {
			return clisp.A(n.Atom)
		}
		kids := make([]*clisp.Node, len(n.Kids))
		for i, k := range n.Kids {
			kids[i] = sub(k)
		}
		return clisp.L(kids...)
	}
	f := clisp.L(clisp.A("defn"), clisp.A("f"), clisp.L(clisp.A("fn"), clisp.L(clisp.A("void")), clisp.A("void")), sub(x))
	out, err := clisp.Print([]*clisp.Node{f})
	if err != nil {
		return "/* " + err.Error() + " */"
	}
	s := string(out)
	a := strings.Index(s, "{\n"+clisp.Indent)
	z := strings.LastIndex(s, ";\n}")
	if a < 0 || z < a {
		return "/* the C view of " + label(x) + " */"
	}
	return s[a+2+len(clisp.Indent) : z]
}

// ssUnparen is x without its parentheses.
func ssUnparen(x *Node) *Node {
	for x != nil && x.Is("paren") && len(x.Kids) == 2 {
		x = x.Kids[1]
	}
	return x
}

// ssUp is n's container past parentheses, and the node it holds there.
func (e *Editor) ssUp(n *Node) (p, at *Node) {
	at = n
	for p = e.Parent(n); p != nil && p.Is("paren"); p = e.Parent(p) {
		at = p
	}
	return p, at
}

// ssIdentOf is the declaration x names, past parentheses: an object's,
// a local's, a parameter's or a function's; nil otherwise.
func ssIdentOf(x *Node) *Node {
	x = ssUnparen(x)
	if x == nil || x.list {
		return nil
	}
	d := x.Ref()
	if d == nil || !d.list || d.Is("enumerator") || isExtern(d) {
		return nil
	}
	if d.Is("def") || d.Is("defn") || isParamForm(d) {
		return d
	}
	return nil
}

// isParamForm says d is a parameter's form `(NAME TYPE ...)`.
func isParamForm(d *Node) bool {
	return d.up != nil && isParamList(d.up)
}

// ssEffects says x stores or calls: a call, `++`, `--`, an assignment.
func ssEffects(x *Node) bool {
	found := false
	Walk(x, func(n *Node) bool {
		h := n.Head()
		if h == "call" || incDec[h] || assignOps[h] {
			found = true
		}
		return !found
	})
	return found
}

// ssStmtPlace says x stands where a statement does: an item, or a branch
// or body of an if, a loop or a switch.
func (e *Editor) ssStmtPlace(x *Node) bool {
	p, i := e.index(x)
	if i < 0 {
		return false
	}
	switch {
	case p.Is("block") || p.Is("stmt-expr"):
		return true
	case p.Is("defn"):
		return i >= defnItemsAt(p)
	case p.Is("if"):
		return i >= 2
	case p.Is("while"), p.Is("switch"):
		return i == 2
	case p.Is("do"):
		return i == 1
	case p.Is("for"):
		return i == 4
	}
	return false
}

// ssStructOf is a candidate's struct: its type's definition, when every
// member is a named scalar or pointer and none a bit-field; and the name C
// spells it by, its typedef's or `struct TAG`.
func ssStructOf(d *Node) (st *Node, fields []string, spelled string) {
	t := d.Type
	if t == nil || !t.Is("struct") {
		return nil, nil, ""
	}
	ms := members(t)
	if len(ms) == 0 {
		return nil, nil, ""
	}
	for _, m := range ms {
		if m.Is("static_assert") {
			continue
		}
		if !m.list || len(m.Kids) < 2 || m.Kids[0].list {
			return nil, nil, ""
		}
		for _, k := range m.Kids[2:] {
			if k.Is("bits") {
				return nil, nil, ""
			}
		}
		mt := m.Type
		if mt == nil {
			return nil, nil, ""
		}
		switch mt.Head() {
		case "struct", "union", "array", "function", "extern-struct", "extern-union":
			return nil, nil, ""
		case "basic":
			if len(mt.Kids) == 2 && mt.Kids[1].Atom == "void" {
				return nil, nil, ""
			}
		}
		fields = append(fields, m.Kids[0].Atom)
	}
	// the spelling: a typedef name, or the tag
	tf := defType(d)
	var named *Node
	Walk(tf, func(x *Node) bool {
		if named != nil {
			return false
		}
		if !x.list && x.Ref() != nil && x.Ref().Is("typedef") {
			named = x
			return false
		}
		return true
	})
	switch {
	case named != nil:
		spelled = named.Atom
	default:
		tag := ""
		if s := ssStructForm(tf); s != nil {
			tag = tagOf(s)
		}
		spelled = "struct " + tag
	}
	return t, fields, spelled
}

// ssStructForm is the `(struct ...)` form a type form spells, if any.
func ssStructForm(t *Node) *Node {
	var s *Node
	Walk(t, func(x *Node) bool {
		if s != nil {
			return false
		}
		if x.Is("struct") {
			s = x
			return false
		}
		return true
	})
	return s
}

// ssInitOK says a candidate's value is one the rewrite writes: an
// expression that does nothing, or braces of values by position.
func ssInitOK(v *Node) bool {
	if !v.Is("init") {
		return !ssEffects(v)
	}
	for _, k := range v.Args() {
		if k.Is("at") || k.Is("init") {
			return false
		}
	}
	return true
}

// ssFunction finds one function's struct locals and their rewrites, and
// says how many it takes.
func (e *Editor) ssFunction(fn *Node, rw *ssRewrites, mentioned func(string) bool) int {
	// the candidates: declared as an item, automatic, of a struct of scalars
	type cand struct {
		fields  []string
		spelled string
	}
	decl := map[*Node]*cand{}
	var order []*Node
	Walk(fn, func(x *Node) bool {
		if x == fn || !x.Is("def") {
			return true
		}
		if hasPrefix(x, "static") || hasPrefix(x, "extern") || hasPrefix(x, "typedef") {
			return true
		}
		if p := e.Parent(x); p == nil || !(p.Is("block") || p.Is("defn") || p.Is("stmt-expr")) || !e.ssStmtPlace(x) {
			return true
		}
		st, fields, spelled := ssStructOf(x)
		if st == nil || spelled == "struct " {
			return true
		}
		if v := defValue(x); v != nil && !ssInitOK(v) {
			return true
		}
		decl[x] = &cand{fields, spelled}
		order = append(order, x)
		return true
	})
	if len(decl) == 0 {
		return 0
	}
	// the uses
	uses := map[*Node][]ssUse2{}
	bad := map[*Node]bool{}
	Walk(fn, func(x *Node) bool {
		if x.list || len(x.Refs) == 0 {
			return true
		}
		d := x.Ref()
		if decl[d] == nil || x == d {
			return true
		}
		u, ok := e.ssUseOf(x, decl[d].fields)
		if !ok {
			bad[d] = true
			return true
		}
		uses[d] = append(uses[d], u)
		return true
	})
	for changed := true; changed; {
		changed = false
		for _, d := range order {
			if bad[d] {
				continue
			}
			for _, u := range uses[d] {
				if u.kind == "init" && u.peer != nil && (decl[u.peer] == nil || bad[u.peer]) {
					bad[d] = true
					changed = true
					break
				}
			}
			if v := defValue(d); !bad[d] && v != nil && !v.Is("init") {
				if t := ssIdentOf(v); t != nil && (decl[t] == nil || bad[t]) {
					bad[d] = true
					changed = true
				}
			}
		}
	}
	taken := map[*Node]bool{}
	var ds []*Node
	for _, d := range order {
		if !bad[d] {
			taken[d] = true
			ds = append(ds, d)
		}
	}
	if len(ds) == 0 {
		return 0
	}
	// the members' names
	names := map[*Node][]string{}
	for _, d := range ds {
		for _, f := range decl[d].fields {
			nm := declName(d) + "_" + f
			for mentioned(nm) {
				nm += "_"
			}
			names[d] = append(names[d], nm)
		}
	}
	// what is read of each
	read := map[*Node][]bool{}
	for _, d := range ds {
		read[d] = make([]bool, len(names[d]))
	}
	for changed := true; changed; {
		changed = false
		mark := func(d *Node, i int) {
			if i >= 0 && !read[d][i] {
				read[d][i] = true
				changed = true
			}
		}
		for _, d := range ds {
			for _, u := range uses[d] {
				switch u.kind {
				case "member":
					if !u.wonly {
						mark(d, u.field)
					}
				case "store":
					for i := range read[d] {
						if u.peer != nil && taken[u.peer] {
							if read[u.peer][i] {
								mark(d, i)
							}
							continue
						}
						mark(d, i)
					}
				case "init":
					for i := range read[d] {
						if taken[u.peer] && read[u.peer][i] {
							mark(d, i)
						}
					}
				}
			}
			if v := defValue(d); v != nil && v.Is("init") {
				for i, k := range v.Args() {
					if i >= len(read[d]) {
						break
					}
					if ssEffects(k) {
						mark(d, i)
					}
				}
			}
		}
	}
	// member i of the expression x: another taken struct's local, or (x).m
	memberOf := func(x, d *Node, i int) string {
		if t := ssIdentOf(x); t != nil && taken[t] {
			return names[t][i]
		}
		return "(" + rw.render(x) + ")." + decl[d].fields[i]
	}
	for _, d := range ds {
		d := d
		c := decl[d]
		rw.add(d, func() string {
			vals := make([]string, len(names[d]))
			switch v := defValue(d); {
			case v == nil:
			case !v.Is("init"):
				for i := range vals {
					vals[i] = memberOf(v, d, i)
				}
			default:
				i := 0
				for _, k := range v.Args() {
					if i >= len(vals) {
						break
					}
					vals[i] = rw.render(k)
					i++
				}
				for ; i < len(vals); i++ {
					vals[i] = "0"
				}
			}
			var b strings.Builder
			for i, nm := range names[d] {
				if !read[d][i] {
					continue // nothing reads it
				}
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				fmt.Fprintf(&b, "typeof(((%s *)0)->%s) %s", c.spelled, c.fields[i], nm)
				if vals[i] != "" {
					b.WriteString(" = " + vals[i])
				}
				b.WriteString(";")
			}
			return b.String()
		})
		for _, u := range uses[d] {
			u := u
			switch u.kind {
			case "member":
				i := u.field
				if u.wonly && !read[d][i] {
					// a store nothing reads: what the value does, if anything
					rhs := u.other
					rw.add(u.stmt, func() string {
						if !ssEffects(rhs) {
							return ";"
						}
						return rw.render(rhs) + ";"
					})
					continue
				}
				rw.add(u.node, func() string { return names[d][i] })
			case "set", "store":
				if u.kind == "set" && u.peer != nil && taken[u.peer] && u.peer == d {
					continue
				}
				rw.add(u.stmt, func() string {
					var parts []string
					for i, nm := range names[d] {
						if u.kind == "set" && !read[d][i] || u.kind == "store" && u.peer != nil && taken[u.peer] && !read[u.peer][i] {
							continue // nothing reads it
						}
						if u.kind == "set" {
							parts = append(parts, nm+" = "+memberOf(u.other, d, i))
							continue
						}
						if t := ssIdentOf(u.other); t != nil && taken[t] {
							parts = append(parts, names[t][i]+" = "+nm)
							continue
						}
						parts = append(parts, "("+rw.render(u.other)+")."+c.fields[i]+" = "+nm)
					}
					if len(parts) == 0 {
						return ";"
					}
					return "{ " + strings.Join(parts, "; ") + "; }"
				})
			}
		}
	}
	return len(ds)
}

// ssUseOf is the use of a candidate at the atom x, or false when it is one
// the rewrite cannot write.
func (e *Editor) ssUseOf(x *Node, fields []string) (ssUse2, bool) {
	p, at := e.ssUp(x)
	if p == nil {
		return ssUse2{}, false
	}
	switch {
	case p.Is(".") && len(p.Kids) == 3 && p.Kids[1] == at:
		if q, _ := e.ssUp(p); q != nil && q.Is("addr") {
			return ssUse2{}, false
		}
		field := -1
		for i, f := range fields {
			if f == p.Kids[2].Atom {
				field = i
			}
		}
		if field < 0 {
			return ssUse2{}, false
		}
		use := ssUse2{kind: "member", node: p, field: field}
		if q, sel := e.ssUp(p); q != nil && q.Is("=") && len(q.Kids) == 3 && q.Kids[1] == sel && e.ssStmtPlace(q) {
			use.wonly, use.stmt, use.other = true, q, q.Kids[2]
		}
		return use, true
	case p.Is("=") && len(p.Kids) == 3:
		if !e.ssStmtPlace(p) {
			return ssUse2{}, false
		}
		if p.Kids[1] == at {
			other := p.Kids[2]
			if ssEffects(other) {
				return ssUse2{}, false
			}
			return ssUse2{kind: "set", node: p, other: other, peer: ssIdentOf(other), stmt: p}, true
		}
		other := p.Kids[1]
		if ssEffects(other) {
			return ssUse2{}, false
		}
		return ssUse2{kind: "store", node: p, other: other, peer: ssIdentOf(other), stmt: p}, true
	case p.Is("def") && defValue(p) == at:
		return ssUse2{kind: "init", node: at, peer: p}, true
	}
	return ssUse2{}, false
}
