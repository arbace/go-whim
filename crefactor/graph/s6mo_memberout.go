package graph

import (
	"fmt"
	"sort"
	"strings"
)

// MEMBEROUT (doc/GRAPH-MIGRATION.md, Step6): the address of a struct's
// scalar or pointer member taken out of the calls that pass it, asked of
// the graph's edges and types -- crefactor/xform's text step of the name,
// which type-checked the core with cc, on the nodes.  `f(a, &s->m)` becomes
// `f__m(a, s)`, and a function written once per callee and member does what
// the call did through a local:
//
//	static R f__m(A a, S *s0__) { typeof(s0__->m) m0__ = s0__->m; R r__ = f(a, &m0__); s0__->m = m0__; return r__; }
//
// A site is taken only where the copy is provably the call's:
//
//   - the callee is a function the core defines;
//   - no function it can reach names that member (of that struct) -- a call
//     through a pointer reaches every function whose name the core writes
//     but does not call, a call of what the core does not define every core
//     function the host names -- unless the member is of a local struct
//     whose address goes nowhere but into such a call;
//   - the callee's parameter is only dereferenced, indexed, selected from,
//     compared, tested, or handed on to a parameter that is so;
//   - the expression the member is reached through calls and stores
//     nothing;
//   - the callee is not variadic and names every parameter;
//   - every address of that member in the core is such a site.
//
// It works on the core (the forms above the first include form); the
// wrappers go before the first function holding a call of each, and the
// calls and the wrappers are made by FRAG, so that every node is typed as
// the import types it.

// MemberOutResult is what MemberOut did: the arguments that pass a member's
// address no more, the functions written, and the sites held, by reason.
type MemberOutResult struct {
	Taken    int
	Wrappers []string
	Held     map[string]int
}

// Lines are the text step's report lines, in its order.
func (r MemberOutResult) Lines() []string {
	out := []string{fmt.Sprintf("%d arguments pass a member's address no more, through %d functions", r.Taken, len(r.Wrappers))}
	var hs []string
	for why, n := range r.Held {
		hs = append(hs, fmt.Sprintf("held: %d %s", n, why))
	}
	sort.Strings(hs)
	return append(out, hs...)
}

type moFunc struct {
	defn     *Node
	members  map[*Node]bool // the members it names, by declaration
	calls    map[string]bool
	indirect bool
	host     bool
}

type moArgG struct {
	k      int
	arg    *Node // the argument, &E.m or &E->m
	sel    *Node // the selection E.m, E->m (a chain's whole form)
	member *Node // m's declaration
	name   string
	arrow  bool
	st     *Node // the struct's type node
}

type moSiteG struct {
	call   *Node
	callee string
	fn     *Node
	args   []moArgG
}

// moUnparen is n without the parentheses the source wrote around it.
func moUnparen(n *Node) *Node {
	for n != nil && n.Is("paren") && len(n.Kids) == 2 {
		n = n.Kids[1]
	}
	return n
}

// moCallee is the function a call names, when its callee is an identifier
// of a function: its name.
func moCallee(call *Node) string {
	c := moUnparen(call.Kids[1])
	if c.list {
		return ""
	}
	d := c.Ref()
	if d == nil || !d.Type.Is("function") {
		return ""
	}
	return ordinaryName(d)
}

// moSelected is the declaration of the last member a selection form names.
func moSelected(sel *Node) *Node {
	if len(sel.Kids) < 3 {
		return nil
	}
	return sel.Kids[len(sel.Kids)-1].Ref()
}

// moAggregate says t is an array, a struct or a union: an object already.
func moAggregate(t *Node) bool {
	return t.Is("array") || t.Is("struct") || t.Is("union") || t.Is("extern-struct") || t.Is("extern-union")
}

// MemberOut is the transform, on the core.
func (e *Editor) MemberOut() (MemberOutResult, error) {
	res := MemberOutResult{Held: map[string]int{}}
	core := e.Core()
	fns := map[string]*moFunc{}
	var order []*Node
	for _, f := range core {
		if f.Is("defn") {
			fns[topName(f)] = &moFunc{defn: f, members: map[*Node]bool{}, calls: map[string]bool{}}
			order = append(order, f)
		}
	}
	bodyWalk := func(fn *Node, f func(*Node) bool) {
		for _, it := range Body(fn) {
			Walk(it, f)
		}
	}
	called := map[*Node]bool{}
	for _, fd := range order {
		g := fns[topName(fd)]
		bodyWalk(fd, func(n *Node) bool {
			switch n.Head() {
			case ".", "->":
				for _, m := range n.Kids[2:] {
					if d := m.Ref(); d != nil {
						g.members[d] = true
					}
				}
			case "call":
				if c := moUnparen(n.Kids[1]); !c.list {
					called[c] = true
				}
				name := moCallee(n)
				switch {
				case name == "":
					g.indirect = true
				case fns[name] == nil:
					g.host = true
				default:
					g.calls[name] = true
				}
			}
			return true
		})
	}
	// the functions whose name the core writes but does not call
	valued := map[string]bool{}
	for _, f := range core {
		Walk(f, func(n *Node) bool {
			if n.list || called[n] {
				return true
			}
			if d := n.Ref(); d != nil && d.Type.Is("function") && fns[ordinaryName(d)] != nil {
				valued[ordinaryName(d)] = true
			}
			return true
		})
	}
	host, err := FormsC(e.Host())
	if err != nil {
		return res, err
	}
	hostCalls := map[string]bool{}
	for name := range fns {
		if wordIn(string(host), name) {
			hostCalls[name] = true
		}
	}
	reached := map[string]map[*Node]bool{}
	reachOf := func(name string) map[*Node]bool {
		if r := reached[name]; r != nil {
			return r
		}
		r := map[*Node]bool{}
		seen := map[string]bool{name: true}
		stack := []string{name}
		push := func(c string) {
			if !seen[c] {
				seen[c] = true
				stack = append(stack, c)
			}
		}
		for len(stack) > 0 {
			g := fns[stack[len(stack)-1]]
			stack = stack[:len(stack)-1]
			for m := range g.members {
				r[m] = true
			}
			for c := range g.calls {
				push(c)
			}
			if g.indirect {
				for c := range valued {
					push(c)
				}
			}
			if g.host {
				for c := range hostCalls {
					push(c)
				}
			}
		}
		reached[name] = r
		return r
	}
	// whether a function's k-th parameter is kept anywhere
	memo := map[string]bool{}
	var paramKept func(name string, k int) bool
	paramKept = func(name string, k int) bool {
		key := fmt.Sprintf("%s/%d", name, k)
		if v, ok := memo[key]; ok {
			return v
		}
		memo[key] = false
		ps := paramElems(defType(fns[name].defn))
		if k >= len(ps) || paramName(ps[k]) == "" {
			memo[key] = true
			return true
		}
		d := ps[k]
		isD := func(x *Node) bool {
			x = moUnparen(x)
			return !x.list && x.Ref() == d
		}
		total, fine := 0, 0
		bodyWalk(fns[name].defn, func(n *Node) bool {
			if !n.list {
				if n.Ref() == d {
					total++
				}
				return true
			}
			args := n.Args()
			switch n.Head() {
			case "deref", "!", "index", "->":
				if isD(args[0]) {
					fine++
				}
			case "call":
				callee := moCallee(n)
				for i, a := range args[1:] {
					if isD(a) && callee != "" && fns[callee] != nil && !paramKept(callee, i) {
						fine++
					}
				}
			case "==", "!=", "&&", "||":
				for _, a := range args {
					if isD(a) {
						fine++
					}
				}
			case "if", "switch":
				if isD(args[0]) {
					fine++
				}
			}
			return true
		})
		kept := total != fine
		memo[key] = kept
		return kept
	}
	// the local structs whose address goes nowhere but into a call's
	// argument &s.m
	localStruct := map[*Node]bool{}
	for _, fd := range order {
		local := map[*Node]bool{}
		bodyWalk(fd, func(n *Node) bool {
			if n.Is("def") && defNameAt(n) > 0 && !hasPrefix(n, "static") && !hasPrefix(n, "extern") &&
				(n.Type.Is("struct") || n.Type.Is("union") || n.Type.Is("extern-struct") || n.Type.Is("extern-union")) {
				local[n] = true
			}
			return true
		})
		atSite := map[*Node]bool{}
		bodyWalk(fd, func(n *Node) bool {
			if !n.Is("call") {
				return true
			}
			for _, a := range n.Kids[2:] {
				if u := moUnparen(a); u.Is("addr") {
					if sel := moUnparen(u.Kids[1]); sel.Is(".") && sel.Type != nil && !moAggregate(sel.Type) {
						atSite[u] = true
					}
				}
			}
			return true
		})
		escapes := map[*Node]bool{}
		bodyWalk(fd, func(n *Node) bool {
			if !n.Is("addr") || atSite[n] {
				return true
			}
			x := moUnparen(n.Kids[1])
			for x.Is(".") || x.Is("index") {
				x = moUnparen(x.Kids[1])
			}
			if !x.list && local[x.Ref()] {
				escapes[x.Ref()] = true
			}
			return true
		})
		// an array member of the local decays to a pointer into it
		bodyWalk(fd, func(n *Node) bool {
			if !n.Is(".") || len(n.Kids) < 3 {
				return true
			}
			if m := n.Kids[2].Ref(); m != nil && m.Type.Is("array") {
				if b := moUnparen(n.Kids[1]); !b.list && local[b.Ref()] {
					escapes[b.Ref()] = true
				}
			}
			return true
		})
		for d := range local {
			if !escapes[d] {
				localStruct[d] = true
			}
		}
	}
	effects := func(x *Node) bool {
		found := false
		Walk(x, func(n *Node) bool {
			switch h := n.Head(); {
			case h == "call", h == "post++", h == "post--", h == "pre++", h == "pre--", assignOps[h]:
				found = true
			}
			return !found
		})
		return found
	}
	variadicOrUnnamed := func(fd *Node) bool {
		for _, p := range paramElems(defType(fd)) {
			if !p.list && p.Atom == "..." || paramName(p) == "" {
				return true
			}
		}
		return false
	}
	// the sites
	var sites []*moSiteG
	for _, fd := range order {
		bodyWalk(fd, func(n *Node) bool {
			if !n.Is("call") {
				return true
			}
			callee := moCallee(n)
			var margs []moArgG
			for k, a := range n.Kids[2:] {
				u := moUnparen(a)
				if !u.Is("addr") {
					continue
				}
				sel := moUnparen(u.Kids[1])
				if !sel.Is(".") && !sel.Is("->") {
					continue
				}
				if sel.Type == nil || moAggregate(sel.Type) {
					continue
				}
				m := moSelected(sel)
				var base *Node // the local E.m names, when E is one
				if sel.Is(".") && len(sel.Kids) == 3 {
					if b := moUnparen(sel.Kids[1]); !b.list {
						base = b.Ref()
					}
				}
				why := ""
				switch {
				case callee == "" || fns[callee] == nil:
					why = "a callee the core does not define"
				case !localStruct[base] && reachOf(callee)[m]:
					why = "a callee that can name the member"
				case paramKept(callee, k):
					why = "a callee that keeps the pointer"
				case effects(sel.Kids[1]):
					why = "a member reached through what does something"
				case variadicOrUnnamed(fns[callee].defn):
					why = "a variadic callee, or an unnamed parameter"
				}
				if why != "" {
					res.Held[why]++
					continue
				}
				margs = append(margs, moArgG{k: k, arg: a, sel: sel, member: m, name: sel.Kids[len(sel.Kids)-1].Atom,
					arrow: sel.Is("->"), st: e.Parent(m)})
			}
			if len(margs) > 0 {
				sites = append(sites, &moSiteG{call: n, callee: callee, fn: fd, args: margs})
			}
			return true
		})
	}
	// a member is taken only when every address of it in the core is
	all := map[*Node]int{}
	for _, fd := range order {
		bodyWalk(fd, func(n *Node) bool {
			if !n.Is("addr") {
				return true
			}
			if sel := moUnparen(n.Kids[1]); sel.Is(".") || sel.Is("->") {
				all[moSelected(sel)]++
			}
			return true
		})
	}
	taken := map[*Node]int{}
	for _, s := range sites {
		for _, a := range s.args {
			taken[a.member]++
		}
	}
	var kept []*moSiteG
	for _, s := range sites {
		var args []moArgG
		for _, a := range s.args {
			if taken[a.member] == all[a.member] {
				args = append(args, a)
			} else {
				res.Held["a member whose address is kept elsewhere too"]++
			}
		}
		if len(args) > 0 {
			s.args = args
			kept = append(kept, s)
		}
	}
	sites = kept
	// the functions, one per callee and members, before the first function
	// that calls it; the calls
	names := map[string]string{}
	wrappers := map[string]bool{}
	defs := map[*Node][]string{}
	var fnOrder []*Node
	var frags []Frag
	for _, s := range sites {
		var parts, ms []string
		for _, a := range s.args {
			parts = append(parts, fmt.Sprintf("%d:%s:%p", a.k, a.name, a.st))
			ms = append(ms, a.name)
		}
		key := s.callee + "|" + strings.Join(parts, ",")
		name, ok := names[key]
		if !ok {
			name = s.callee + "__" + strings.Join(ms, "_")
			for i := 2; wrappers[name] || fns[name] != nil; i++ {
				name = fmt.Sprintf("%s__%s_%d", s.callee, strings.Join(ms, "_"), i)
			}
			names[key] = name
			wrappers[name] = true
			res.Wrappers = append(res.Wrappers, name)
			def, err := e.moWrapper(fns[s.callee].defn, name, s.args)
			if err != nil {
				return res, fmt.Errorf("%s: %v", s.callee, err)
			}
			if defs[s.fn] == nil {
				fnOrder = append(fnOrder, s.fn)
			}
			defs[s.fn] = append(defs[s.fn], def)
		}
		// the call: its callee the wrapper, each argument taken its base
		holes := Bindings{}
		byK := map[int]moArgG{}
		for _, a := range s.args {
			byK[a.k] = a
		}
		var args []string
		for k, a := range s.call.Kids[2:] {
			h := fmt.Sprintf("a%d", k)
			ma, ok := byK[k]
			if !ok {
				holes[h] = a
				args = append(args, "$"+h)
				continue
			}
			res.Taken++
			var b string
			if len(ma.sel.Kids) == 3 {
				holes[h] = ma.sel.Kids[1]
				b = "$" + h
			} else {
				c, err := ExprText(ma.sel)
				if err != nil {
					return res, err
				}
				op := "."
				if ma.arrow {
					op = "->"
				}
				b = strings.TrimSuffix(c, op+ma.name)
			}
			if !ma.arrow {
				b = "&" + b
			}
			args = append(args, b)
		}
		src := name + "(" + strings.Join(args, ", ") + ")"
		if p, i := e.index(s.call); e.place(p, i) == placeItem {
			src += ";" // a call that stands as a statement
		}
		frags = append(frags, Frag{At: e.SpotOf(s.call), Src: src, Holes: holes})
	}
	for _, fd := range fnOrder {
		frags = append(frags, Frag{At: e.SpotBefore(fd), Src: strings.Join(defs[fd], "")})
	}
	if len(frags) > 0 {
		if _, err := e.SpliceC(frags...); err != nil {
			return res, err
		}
	}
	return res, nil
}

// moWrapper is the C of the function named name that calls fd with the
// members of args passed through locals: its parameters fd's, each of
// args' the struct's pointer instead.  Its spellings are fd's C view's.
func (e *Editor) moWrapper(fd *Node, name string, args []moArgG) (string, error) {
	c, err := FormsC([]*Node{fd})
	if err != nil {
		return "", err
	}
	src := string(c)
	fname := topName(fd)
	i := strings.Index(src, "\n"+fname+"(")
	if i < 0 {
		return "", fmt.Errorf("the definition's head")
	}
	head := strings.TrimSpace(src[:i])
	pdecls, ok := moSplitParams(src[i+len(fname)+2:])
	if !ok {
		return "", fmt.Errorf("parameters")
	}
	ps := paramElems(defType(fd))
	if len(ps) != len(pdecls) {
		return "", fmt.Errorf("parameters")
	}
	byK := map[int]moArgG{}
	for _, a := range args {
		byK[a.k] = a
	}
	var decl, callArgs, pre, post []string
	for k := range pdecls {
		if a, ok := byK[k]; ok {
			s := fmt.Sprintf("s%d__", k)
			decl = append(decl, e.moStructName(a.st)+" *"+s)
			l := fmt.Sprintf("%s%d__", a.name, k)
			pre = append(pre, fmt.Sprintf("    typeof(%s->%s) %s = %s->%s;\n", s, a.name, l, s, a.name))
			post = append(post, fmt.Sprintf("    %s->%s = %s;\n", s, a.name, l))
			callArgs = append(callArgs, "&"+l)
			continue
		}
		decl = append(decl, pdecls[k])
		callArgs = append(callArgs, paramName(ps[k]))
	}
	call := fmt.Sprintf("%s(%s)", fname, strings.Join(callArgs, ", "))
	var b strings.Builder
	fmt.Fprintf(&b, "    %s\n%s(%s)\n{\n", head, name, strings.Join(decl, ", "))
	b.WriteString(strings.Join(pre, ""))
	if r := resultType(fd.Type); r.Is("basic") && len(r.Kids) == 2 && r.Kids[1].Atom == "void" {
		fmt.Fprintf(&b, "    %s;\n", call)
		b.WriteString(strings.Join(post, ""))
	} else {
		res := head
		for _, w := range []string{"static ", "inline "} {
			res = strings.TrimSpace(strings.ReplaceAll(res, w, ""))
		}
		fmt.Fprintf(&b, "    %s r__ = %s;\n", res, call)
		b.WriteString(strings.Join(post, ""))
		b.WriteString("    return r__;\n")
	}
	b.WriteString("}\n\n")
	return b.String(), nil
}

// moSplitParams splits a parameter list's C, from after its `(` to its
// matching `)`, at its top-level commas.
func moSplitParams(s string) ([]string, bool) {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[':
			depth++
		case ']':
			depth--
		case ')':
			if depth == 0 {
				p := strings.TrimSpace(s[start:i])
				if p != "" && p != "void" || len(out) > 0 {
					out = append(out, p)
				}
				return out, true
			}
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return nil, false
}

// moStructName is how a struct type is spelled: the first typedef of the
// file that names it, or `struct TAG`.
func (e *Editor) moStructName(st *Node) string {
	for _, f := range e.g.Forms {
		if f.Is("typedef") && f.Type == st {
			if n := topName(f); n != "" {
				return n
			}
		}
	}
	kw := strings.TrimPrefix(st.Head(), "extern-")
	return kw + " " + tagOf(st)
}
