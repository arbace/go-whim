package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
)

// LOCALOUT (doc/GRAPH-MIGRATION.md, Step6): an out-parameter made a value in
// and a value out, on the graph -- crefactor/xform's LocalOut (phase 100's
// first step), its analysis asked of the forms, the refers edges and the
// typed edges, its rewrites made by RETYPE, PARAM and FRAG.  A function's
// parameter `T *p` whose callee only reads and writes *p and tests p
// against null, and whose every caller passes `&x` of a local of its own
// that nothing else takes the address of, takes x's value and returns its
// new one:
//
//	void f(A a, T *p)     ->  T f(A a, T p)                    x = f(a, x)
//	R g(A a, T *p, U *q)  ->  g__out_T g(A a, T p, U q)        (g__o = g(a, x, y), x = g__o.p, y = g__o.q, g__o.r__)
//
// g__out_T a struct of the result and the parameters' values, declared
// before g's first declaration.  A site is taken only when
//
//   - the callee is a function the core defines, not variadic, its
//     parameters named, its address never taken and the host naming it not;
//   - the parameter points at a scalar or a pointer, and the callee uses it
//     only as *p (not &*p) and in a test of whether it is null;
//   - every call passes &x there, x a local or a parameter of the caller, of
//     the pointee's type, once, no other argument names x, and nothing in
//     the full expression reads x where C does not order the read after
//     the call;
//   - every address of x taken anywhere is such an argument.
//
// A parameter whose value in is dead (written before it is read on every
// path) leaves the parameter list for a local of the callee; a call writes
// back only the values something reads after it.
//
// Every question the text asked cc's tree is asked of the core's forms
// here, in the same order; the bytes are the text's.  The edits: the
// structs by FRAG before each callee's first declaration; the result and
// the parameters by RetypeResult, Retype and ParamToLocal (ids kept, every
// declaration of the callee); the uses of p rewritten in place (`*p` is p,
// moved; a test its constant); the locals the text declared at a body's
// start by FRAG; and the returns and the calls by FRAG, a call's other
// arguments and a return's value moved in as holes -- innermost first, a
// round of one synthesized unit at a time.  The expressions the retypes
// and rewrites left untyped are what they were: nothing an expression
// above `*p`, a test or a call stands in changes type (the call's
// replacement is of the call's old type), and FRAG types what it makes as
// the import does.

// LocalOutStats is what LocalOut did.
type LocalOutStats struct {
	Params int // out-parameters made values in and out
	Funcs  int // of that many functions
	DeadIn int // of the parameters, those that take no value in
}

type s6loSite struct {
	call *Node // the call
	g    string
	k    int
	u    *Node // &x
	x    *Node // x's declaration
	fn   *Node // the caller's definition
}

type s6loShape struct {
	void   bool
	simple bool // void, one: returns the value
	stype  string
	res    *Node // the result's type form
	vals   []string
	tforms []*Node // what each parameter points at, a type form
}

// LocalOut makes the core's out-parameters values (above); the core is
// the forms above the first include form, and the forms below it (the
// host) only asked whether they name a callee.
func (e *Editor) LocalOut() (LocalOutStats, error) {
	var st LocalOutStats
	core := e.Core()
	coreText, err := s6loPrint(core)
	if err != nil {
		return st, err
	}
	hostText, err := s6loPrint(e.Host())
	if err != nil {
		return st, err
	}
	fns := map[string]*Node{}
	var order []*Node
	for _, f := range core {
		if f.Is("defn") {
			fns[topName(f)] = f
			order = append(order, f)
		}
	}
	valued := e.s6loValued(core, fns)
	// the candidates: what each callee does with the parameter
	cand := map[string]map[int]bool{}
	for _, fd := range order {
		g := topName(fd)
		fn := defType(fd)
		if fn == nil || !fn.Is("fn") || s6loVariadicOrUnnamed(fn) || valued[g] || s6loMentions(hostText, g) {
			continue
		}
		for k, p := range paramElems(fn) {
			if e.s6loParam(fd, p) {
				if cand[g] == nil {
					cand[g] = map[int]bool{}
				}
				cand[g][k] = true
			}
		}
	}
	// the calls
	var sites []s6loSite
	for _, fd := range order {
		for _, it := range fd.Kids[defnItemsAt(fd):] {
			Walk(it, func(c *Node) bool {
				if !c.Is("call") {
					return true
				}
				g := s6loCallee(c)
				if cand[g] == nil {
					return true
				}
				as := c.Kids[2:]
				seen := map[*Node]bool{}
				ps := paramElems(defType(fns[g]))
				for _, k := range s6loKeys(cand[g]) {
					if k >= len(as) {
						delete(cand[g], k)
						continue
					}
					u, x := e.s6loAddr(as[k])
					el := pointee(ps[k].Type)
					if x == nil || seen[x] || !s6loSameScalar(x.Type, el) || s6loNamedElsewhere(as, k, x) || e.s6loUnsequenced(c, x) {
						delete(cand[g], k)
						continue
					}
					seen[x] = true
					sites = append(sites, s6loSite{c, g, k, u, x, fd})
				}
				return true
			})
		}
	}
	// every address of a local taken must be such an argument
	for changed := true; changed; {
		changed = false
		good := map[*Node]bool{}
		for _, s := range sites {
			if cand[s.g][s.k] {
				good[s.u] = true
			}
		}
		escapes := map[*Node]bool{}
		for _, fd := range order {
			for _, it := range fd.Kids[defnItemsAt(fd):] {
				Walk(it, func(n *Node) bool {
					if n.Is("addr") && len(n.Kids) == 2 {
						if x := s6loUnparen(n.Kids[1]); !x.list && x.Ref() != nil && !good[n] {
							escapes[x.Ref()] = true
						}
					}
					return true
				})
			}
		}
		for _, s := range sites {
			if cand[s.g][s.k] && escapes[s.x] {
				delete(cand[s.g], s.k)
				changed = true
			}
		}
	}
	outs := map[string][]int{}
	for g, ks := range cand {
		for k := range ks {
			outs[g] = append(outs[g], k)
			st.Params++
		}
		if len(outs[g]) == 0 {
			delete(outs, g)
		}
		sort.Ints(outs[g])
	}
	if st.Params == 0 {
		return st, nil
	}
	st.Funcs = len(outs)
	// the parameters whose value in is dead
	deadIn := map[string]map[int]bool{}
	for g, ks := range outs {
		ps := paramElems(defType(fns[g]))
		for _, k := range ks {
			if s6loWritesFirst(fns[g], ps[k]) {
				if deadIn[g] == nil {
					deadIn[g] = map[int]bool{}
				}
				deadIn[g][k] = true
				st.DeadIn++
			}
		}
	}
	return st, e.s6loRewrite(coreText, fns, outs, sites, deadIn)
}

// s6loRewrite makes the edits.
func (e *Editor) s6loRewrite(coreText string, fns map[string]*Node, outs map[string][]int, sites []s6loSite, deadIn map[string]map[int]bool) error {
	var gs []string
	for g := range outs {
		gs = append(gs, g)
	}
	sort.Strings(gs)
	shapes := map[string]*s6loShape{}
	for _, g := range gs {
		fd := fns[g]
		fn := defType(fd)
		sh := &s6loShape{res: fn.Kids[2]}
		sh.void = s6loIsVoid(resultType(fd.Type))
		sh.simple = sh.void && len(outs[g]) == 1
		ps := paramElems(fn)
		for _, k := range outs[g] {
			t, err := s6loDerefForm(ps[k])
			if err != nil {
				return fmt.Errorf("%s: %v", g, err)
			}
			sh.tforms = append(sh.tforms, t)
			sh.vals = append(sh.vals, paramName(ps[k]))
		}
		sh.stype = g + "__out_T"
		for s6loMentions(coreText, sh.stype) {
			sh.stype += "_"
		}
		shapes[g] = sh
	}
	// the sites still taken, by call, and which values a caller reads back
	good := map[*Node]s6loSite{}
	byCall := map[*Node][]s6loSite{}
	var calls []*Node
	for _, s := range sites {
		if !s6loHas(outs[s.g], s.k) {
			continue
		}
		good[s.u] = s
		if byCall[s.call] == nil {
			calls = append(calls, s.call)
		}
		byCall[s.call] = append(byCall[s.call], s)
	}
	nargsOf := map[*Node]int{}
	for _, c := range calls {
		nargsOf[c] = len(c.Kids) - 2
	}
	deadArg := func(u *Node) bool {
		gs, ok := good[u]
		return ok && deadIn[gs.g][gs.k]
	}
	readBack := map[*Node]bool{} // by &x
	for _, s := range sites {
		if s6loHas(outs[s.g], s.k) {
			readBack[s.u] = e.s6loLaterRead(s.fn, s.x, s.call, deadArg) && !e.s6loKilledAfter(s.x, s.call, deadArg)
		}
	}
	// the uses of each parameter, and the returns, before anything moves
	type use struct {
		n    *Node
		kind string
	}
	uses := map[string][]use{}
	rets := map[string][]*Node{}
	for _, g := range gs {
		fd := fns[g]
		ps := paramElems(defType(fd))
		isOut := map[*Node]bool{}
		for _, k := range outs[g] {
			isOut[ps[k]] = true
		}
		for _, it := range fd.Kids[defnItemsAt(fd):] {
			Walk(it, func(n *Node) bool {
				if !n.list && n.Ref() != nil && isOut[n.Ref()] {
					uses[g] = append(uses[g], use{n, e.s6loUse(n)})
				}
				if n.Is("return") {
					rets[g] = append(rets[g], n)
				}
				return true
			})
		}
	}
	// the types every expression of the functions touched has now
	saved := map[*Node]*Node{}
	touched := map[*Node]bool{}
	for _, g := range gs {
		touched[fns[g]] = true
	}
	for _, s := range sites {
		touched[s.fn] = true
	}
	for f := range touched {
		Walk(f, func(n *Node) bool {
			if n.list && n.Type != nil {
				saved[n] = n.Type
			}
			return true
		})
	}

	// 1. the structs, before each callee's first declaration
	var fs []Frag
	for _, g := range gs {
		sh := shapes[g]
		if sh.simple {
			continue
		}
		var first *Node
		for _, f := range e.Core() {
			if isFuncDecl(f) && topName(f) == g {
				first = f
				break
			}
		}
		mem := []*clisp.Node{}
		if !sh.void {
			mem = append(mem, clisp.L(clisp.A("r__"), Lisp(sh.res)))
		}
		for i, v := range sh.vals {
			mem = append(mem, clisp.L(clisp.A(v), Lisp(sh.tforms[i])))
		}
		def := clisp.L(clisp.A("typedef"), clisp.A(sh.stype), clisp.L(append([]*clisp.Node{clisp.A("struct")}, mem...)...))
		src, err := clisp.Print([]*clisp.Node{def})
		if err != nil {
			return err
		}
		fs = append(fs, Frag{At: e.SpotBefore(first), Src: string(src)})
	}
	if len(fs) > 0 {
		if _, err := e.SpliceC(fs...); err != nil {
			return fmt.Errorf("the structs: %v", err)
		}
	}

	// 2. the results and the parameters, in every declaration
	for _, g := range gs {
		sh := shapes[g]
		res := sh.stype
		if sh.simple {
			res = s6loForm(sh.tforms[0])
		}
		if _, err := e.RetypeResult(g, res); err != nil {
			return fmt.Errorf("%s: %v", g, err)
		}
		ks := outs[g]
		for i := len(ks) - 1; i >= 0; i-- {
			k := ks[i]
			p := paramElems(defType(fns[g]))[k]
			if deadIn[g][k] {
				if _, err := e.ParamToLocal(g, paramName(p)); err != nil {
					return fmt.Errorf("%s: %v", g, err)
				}
			}
			if _, err := e.Retype(p, s6loForm(sh.tforms[i])); err != nil {
				return fmt.Errorf("%s: %v", g, err)
			}
		}
	}

	// 3. the uses: *p is p, a test its constant
	for _, g := range gs {
		for _, u := range uses[g] {
			q := e.s6loUp(u.n)
			var with *Node
			switch u.kind {
			case "deref":
				with = u.n
			case "ne":
				with = NewAtom("1")
			case "eq", "not":
				with = NewAtom("0")
			case "true":
				q, with = u.n, NewAtom("1")
			}
			// the parentheses the C view wrote around what goes stay, as
			// the text kept them: `(*p)[i]` is `(p)[i]`
			if e.parenthesised(q) {
				with = &Node{list: true, Kids: []*Node{NewAtom("paren"), with}, Type: saved[q]}
			}
			err := e.Replace(q, with)
			if err != nil {
				return fmt.Errorf("%s: %v", g, err)
			}
		}
	}

	// 4. the locals the text declared at each body's start: the callee's
	// struct, then the caller's, after the parameters made locals
	decls := map[*Node][]string{}
	var declFns []*Node
	addDecl := func(f *Node, s string) {
		if decls[f] == nil {
			declFns = append(declFns, f)
		}
		decls[f] = append(decls[f], s)
	}
	for _, g := range gs {
		if !shapes[g].simple {
			addDecl(fns[g], shapes[g].stype+" out__;")
		}
	}
	callerOs := map[*Node]map[string]bool{}
	for _, c := range calls {
		ss := byCall[c]
		g := ss[0].g
		sh := shapes[g]
		anyBack := false
		for _, s := range ss {
			anyBack = anyBack || readBack[s.u]
		}
		if !sh.simple && (anyBack || !sh.void) {
			if callerOs[ss[0].fn] == nil {
				callerOs[ss[0].fn] = map[string]bool{}
			}
			callerOs[ss[0].fn][g+"__o"] = true
		}
	}
	var callers []*Node
	for f := range callerOs {
		callers = append(callers, f)
	}
	sort.Slice(callers, func(a, b int) bool { return callers[a].ID < callers[b].ID })
	for _, f := range callers {
		os := callerOs[f]
		var names []string
		for o := range os {
			names = append(names, o)
		}
		sort.Strings(names)
		for _, o := range names {
			addDecl(f, shapes[strings.TrimSuffix(o, "__o")].stype+" "+o+";")
		}
	}
	fs = nil
	for _, f := range declFns {
		at := defnItemsAt(f) + len(deadIn[topName(f)])
		if at >= len(f.Kids) {
			return fmt.Errorf("%s: no item to declare its locals before", topName(f))
		}
		fs = append(fs, Frag{At: e.SpotBefore(f.Kids[at]), Src: strings.Join(decls[f], "\n")})
	}
	if len(fs) > 0 {
		if _, err := e.SpliceC(fs...); err != nil {
			return fmt.Errorf("the locals: %v", err)
		}
	}

	// 5. the returns and the calls, innermost first
	type job struct {
		n    *Node
		frag func() (Frag, error)
	}
	var jobs []job
	outVal := func(sh *s6loShape, withE bool) string {
		if sh.simple {
			return "return " + sh.vals[0] + ";"
		}
		var b strings.Builder
		b.WriteString("{")
		if withE {
			b.WriteString(" out__.r__ = ($e);")
		}
		for _, v := range sh.vals {
			fmt.Fprintf(&b, " out__.%s = %s;", v, v)
		}
		b.WriteString(" return out__; }")
		return b.String()
	}
	for _, g := range gs {
		sh := shapes[g]
		for _, r := range rets[g] {
			r := r
			jobs = append(jobs, job{r, func() (Frag, error) {
				if len(r.Kids) > 1 && !sh.simple {
					return Frag{At: e.SpotOf(r), Src: outVal(sh, true), Holes: Bindings{"e": r.Kids[1]}}, nil
				}
				return Frag{At: e.SpotOf(r), Src: outVal(sh, false)}, nil
			}})
		}
	}
	for _, c := range calls {
		c := c
		ss := byCall[c]
		g := ss[0].g
		sh := shapes[g]
		addr := map[int]*Node{}
		back := map[int]bool{}
		for _, s := range ss {
			addr[s.k] = s.x
			back[s.k] = readBack[s.u]
		}
		nargs := nargsOf[c]
		jobs = append(jobs, job{c, func() (Frag, error) {
			cur := c.Kids[2:]
			holes := Bindings{}
			var args []string
			j := 0
			for k := 0; k < nargs; k++ {
				if deadIn[g][k] {
					continue // gone with its parameter
				}
				if j >= len(cur) {
					return Frag{}, fmt.Errorf("a call of %s: its arguments moved", g)
				}
				a := cur[j]
				j++
				if x, ok := addr[k]; ok {
					args = append(args, declName(x))
					continue
				}
				h := fmt.Sprintf("a%d", k)
				holes[h] = a
				args = append(args, "$"+h)
			}
			callee, err := clisp.PrintExpr(Lisp(c.Kids[1]))
			if err != nil {
				return Frag{}, err
			}
			callTxt := callee + "(" + strings.Join(args, ", ") + ")"
			var src string
			anyBack := false
			for _, k := range outs[g] {
				anyBack = anyBack || back[k]
			}
			switch {
			case sh.simple:
				src = callTxt
				if back[outs[g][0]] {
					src = "(" + declName(addr[outs[g][0]]) + " = " + callTxt + ")"
				}
			case !anyBack && sh.void:
				src = callTxt
			default:
				o := g + "__o"
				parts := []string{o + " = " + callTxt}
				for i, k := range outs[g] {
					if back[k] {
						parts = append(parts, fmt.Sprintf("%s = %s.%s", declName(addr[k]), o, sh.vals[i]))
					}
				}
				if sh.void {
					parts = append(parts, "(void)0")
				} else {
					parts = append(parts, o+".r__")
				}
				src = "(" + strings.Join(parts, ", ") + ")"
			}
			at := e.SpotOf(c)
			if p, i := e.index(c); i >= 0 && e.place(p, i) == placeItem {
				src += ";"
			}
			return Frag{At: at, Src: src, Holes: holes}, nil
		}})
	}
	// the end of a void function that does not end in a return
	fs = nil
	for _, g := range gs {
		sh := shapes[g]
		if sh.void && !s6loEndsInReturn(fns[g]) {
			fs = append(fs, Frag{At: e.SpotEnd(fns[g]), Src: outVal(sh, false)})
		}
	}
	done := map[*Node]bool{}
	for len(done) < len(jobs) || len(fs) > 0 {
		pending := map[*Node]bool{}
		for _, j := range jobs {
			if !done[j.n] {
				pending[j.n] = true
			}
		}
		var round []job
		for _, j := range jobs {
			if done[j.n] {
				continue
			}
			inner := false
			for _, k := range j.n.Kids {
				Walk(k, func(x *Node) bool {
					if pending[x] {
						inner = true
					}
					return !inner
				})
			}
			if !inner {
				round = append(round, j)
			}
		}
		if len(round) == 0 && len(fs) == 0 {
			return fmt.Errorf("the returns and the calls: a round with nothing to do")
		}
		for _, j := range round {
			f, err := j.frag()
			if err != nil {
				return err
			}
			fs = append(fs, f)
			done[j.n] = true
		}
		if _, err := e.SpliceC(fs...); err != nil {
			return fmt.Errorf("the returns and the calls: %v", err)
		}
		fs = nil
	}

	// 6. the types the edits above left unknown are what they were
	e.s6loRestore(saved)
	return nil
}

// s6loRestore gives each expression an edit left untyped the type it had.
func (e *Editor) s6loRestore(saved map[*Node]*Node) {
	out := e.Untyped[:0]
	for _, n := range e.Untyped {
		if !e.Live(n) {
			continue // gone: nothing to type
		}
		if t := saved[n]; t != nil && e.Live(n) && n.Type == nil {
			e.g.save(n)
			n.Type = t
			if e.inFile(t) {
				e.typedBy[t] = append(e.typedBy[t], n)
			}
			continue
		}
		out = append(out, n)
	}
	e.Untyped = out
}

// s6loPrint is forms' C.
func s6loPrint(forms []*Node) (string, error) {
	cs := make([]*clisp.Node, len(forms))
	for i, f := range forms {
		cs[i] = Lisp(f)
	}
	b, err := clisp.Print(cs)
	return string(b), err
}

// s6loForm is a type form as C-lisp's text.
func s6loForm(t *Node) string {
	return strings.TrimSpace(string(clisp.Format([]*clisp.Node{Lisp(t)})))
}

// s6loMentions says text names name as an identifier.
func s6loMentions(text, name string) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], name)
		if j < 0 {
			return false
		}
		a, z := i+j, i+j+len(name)
		if (a == 0 || !s6loIDByte(text[a-1])) && (z >= len(text) || !s6loIDByte(text[z])) {
			return true
		}
		i = z
	}
}

func s6loIDByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func s6loKeys(m map[int]bool) []int {
	var out []int
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func s6loHas(ks []int, k int) bool {
	for _, j := range ks {
		if j == k {
			return true
		}
	}
	return false
}

// s6loUnparen is n without its parentheses.
func s6loUnparen(n *Node) *Node {
	for n.Is("paren") && len(n.Kids) == 2 {
		n = n.Kids[1]
	}
	return n
}

// s6loUp is n's container, past parentheses.
func (e *Editor) s6loUp(n *Node) *Node {
	p := e.Parent(n)
	for p != nil && p.Is("paren") {
		p = e.Parent(p)
	}
	return p
}

// s6loInside says n is root or inside it.
func s6loInside(n, root *Node) bool {
	for ; n != nil; n = n.up {
		if n == root {
			return true
		}
	}
	return false
}

// s6loNames says n names the declaration d.
func s6loNames(n, d *Node) bool {
	found := false
	Walk(n, func(m *Node) bool {
		if !m.list && refersTo(m, d) {
			found = true
		}
		return !found
	})
	return found
}

// s6loCallee is a call's callee's name, when it names a function.
func s6loCallee(c *Node) string {
	f := s6loUnparen(c.Kids[1])
	if f.list {
		return ""
	}
	if r := f.Ref(); r != nil && isFuncDecl(r) {
		return topName(r)
	}
	return ""
}

// s6loValued are the functions the core names but does not call: what a
// call through a pointer may run.
func (e *Editor) s6loValued(core []*Node, fns map[string]*Node) map[string]bool {
	valued := map[string]bool{}
	for _, f := range core {
		Walk(f, func(n *Node) bool {
			if n.list || n.Ref() == nil {
				return true
			}
			r := n.Ref()
			if !isFuncDecl(r) || fns[topName(r)] == nil {
				return true
			}
			q := n
			for p := e.Parent(q); p != nil && p.Is("paren"); p = e.Parent(q) {
				q = p
			}
			if p := e.Parent(q); p != nil && p.Is("call") && p.Kids[1] == q {
				return true
			}
			valued[topName(r)] = true
			return true
		})
	}
	return valued
}

// s6loVariadicOrUnnamed says a fn form takes `...`, or a parameter with no
// name -- `(void)` among them, as cc's declarators say.
func s6loVariadicOrUnnamed(fn *Node) bool {
	if len(fn.Kids) < 2 || !fn.Kids[1].list {
		return true
	}
	for _, p := range fn.Kids[1].Kids {
		if !p.list && p.Atom == "..." || paramName(p) == "" {
			return true
		}
	}
	return false
}

// s6loParam says fd uses its parameter p only as *p and in a test of
// whether it is null, and p points at a scalar or a pointer.
func (e *Editor) s6loParam(fd, p *Node) bool {
	if p.Type == nil || !p.Type.Is("pointer") {
		return false
	}
	el := pointee(p.Type)
	if el == nil {
		return false
	}
	switch el.Head() {
	case "struct", "union", "array", "function", "extern-struct", "extern-union":
		return false
	}
	if s6loIsVoid(el) {
		return false
	}
	ok, used := true, false
	for _, it := range fd.Kids[defnItemsAt(fd):] {
		Walk(it, func(n *Node) bool {
			if n.list || !refersTo(n, p) {
				return ok
			}
			used = true
			if e.s6loUse(n) == "" {
				ok = false
			}
			return ok
		})
	}
	return ok && used
}

func s6loIsVoid(t *Node) bool {
	return t.Is("basic") && len(t.Kids) == 2 && t.Kids[1].Atom == "void"
}

// s6loUse is how the parameter is used at its atom pe: "deref" (*p, not
// &*p), "ne" or "eq" (a test against null), "not" (!p), "true" (p where a
// truth value is asked), or "" for any other use.
func (e *Editor) s6loUse(pe *Node) string {
	u := e.s6loUp(pe)
	if u == nil {
		return ""
	}
	switch u.Head() {
	case "deref":
		if a := e.s6loUp(u); a != nil && a.Is("addr") {
			return ""
		}
		return "deref"
	case "!":
		return "not"
	case "==", "!=":
		if len(u.Kids) != 3 {
			return ""
		}
		other := u.Kids[2]
		if s6loUnparen(u.Kids[2]) == pe {
			other = u.Kids[1]
		}
		if !s6loIsNull(other) {
			return ""
		}
		if u.Is("==") {
			return "eq"
		}
		return "ne"
	case "&&", "||":
		return "true"
	case "if", "switch":
		if s6loUnparen(u.Kids[1]) == pe {
			return "true"
		}
	case "?":
		if s6loUnparen(u.Kids[1]) == pe {
			return "true"
		}
	}
	return ""
}

// s6loIsNull says n is a null pointer constant: 0, nullptr, or one cast to
// a pointer.
func s6loIsNull(n *Node) bool {
	n = s6loUnparen(n)
	if n.Is("cast") && len(n.Kids) == 3 {
		return s6loIsNull(n.Kids[2])
	}
	if n.list {
		return false
	}
	switch {
	case n.Atom == "nullptr":
		return true
	case n.Atom == `'\0'`:
		return true
	case n.Atom != "" && n.Atom[0] >= '0' && n.Atom[0] <= '9':
		v, ok := parseInt(n.Atom)
		return ok && v == 0
	}
	return false
}

// s6loAddr is a as &x of a local or a parameter x, when it is: the `addr`
// node and x's declaration.
func (e *Editor) s6loAddr(a *Node) (*Node, *Node) {
	u := s6loUnparen(a)
	if !u.Is("addr") || len(u.Kids) != 2 {
		return nil, nil
	}
	x := s6loUnparen(u.Kids[1])
	if x.list || x.Ref() == nil {
		return nil, nil
	}
	d := x.Ref()
	if d.Type == nil || !e.s6loAutomatic(d) {
		return nil, nil
	}
	return u, d
}

// s6loAutomatic says d is a local object of automatic storage, or a
// function definition's parameter.
func (e *Editor) s6loAutomatic(d *Node) bool {
	if isParam(d) {
		fn := d.up.up
		return fn.up != nil && fn.up.Is("defn") && defType(fn.up) == fn
	}
	if !d.Is("def") || isFuncDecl(d) || e.Function(d) == nil || e.Function(d) == d {
		return false
	}
	return !hasPrefix(d, "static") && !hasPrefix(d, "extern") && !hasPrefix(d, "thread_local") && !hasPrefix(d, "_Thread_local")
}

// s6loSameScalar says a and b are one scalar type, or both pointers.
func s6loSameScalar(a, b *Node) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Is("pointer") && b.Is("pointer") {
		return true
	}
	switch a.Head() {
	case "struct", "union", "array", "extern-struct", "extern-union":
		return false
	case "enum", "extern-enum":
		return b.Is("enum") || b.Is("extern-enum")
	}
	return a == b
}

// s6loNamedElsewhere says an argument of as other than the k-th names x.
func s6loNamedElsewhere(as []*Node, k int, x *Node) bool {
	for i, a := range as {
		if i != k && s6loNames(a, x) {
			return true
		}
	}
	return false
}

// s6loExprHead says a form is an expression's: an operator's.
var s6loExprHeads = map[string]bool{
	"call": true, "paren": true, "comma": true, "?": true, "->": true, ".": true, "index": true,
	"addr": true, "deref": true, "cast": true, "sizeof": true, "sizeof-bare": true, "alignof": true,
	"alignof-bare": true, "post++": true, "post--": true, "pre++": true, "pre--": true,
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "<<=": true, ">>=": true,
	"&=": true, "^=": true, "|=": true, "+": true, "-": true, "*": true, "/": true, "%": true,
	"<<": true, ">>": true, "<": true, ">": true, "<=": true, ">=": true, "==": true, "!=": true,
	"&": true, "^": true, "|": true, "&&": true, "||": true, "!": true, "~": true, "macro": true,
	"generic": true,
}

// s6loUnsequenced says the full expression holding call names x where C
// does not order the read after the call: on the right of && or || whose
// left holds the call, in an arm of ?: whose condition does, or in a later
// operand of a comma is after it; anywhere else is not.
func (e *Editor) s6loUnsequenced(call, x *Node) bool {
	full := call
	for {
		p := e.Parent(full)
		if p == nil || !s6loExprHeads[p.Head()] {
			break
		}
		full = p
	}
	bad := false
	Walk(full, func(pe *Node) bool {
		if bad {
			return false
		}
		if pe.list || !refersTo(pe, x) || s6loInside(pe, call) {
			return true
		}
		lca := pe
		for lca != nil && !s6loInside(call, lca) {
			lca = lca.up
		}
		after := false
		switch lca.Head() {
		case "&&", "||", "comma":
			after = s6loKidOf(lca, call) < s6loKidOf(lca, pe)
		case "?":
			after = s6loInside(call, lca.Kids[1]) && !s6loInside(pe, lca.Kids[1])
		}
		if !after {
			bad = true
		}
		return !bad
	})
	return bad
}

// s6loKidOf is the index of the element of p that holds n.
func s6loKidOf(p, n *Node) int {
	for i, k := range p.Kids {
		if s6loInside(n, k) {
			return i
		}
	}
	return -1
}

// s6loDerefForm is what the parameter p points at, a type form: its type
// form's target, the pointer's own qualifiers gone, as the text spelled it.
func s6loDerefForm(p *Node) (*Node, error) {
	t := paramTypeForm(p)
	if !t.Is("ptr") || len(t.Kids) < 2 {
		return nil, fmt.Errorf("a parameter spelled %s", Lisp(p))
	}
	x := t.Kids[1]
	bad := false
	Walk(x, func(n *Node) bool {
		if n.Is("fn") || n.Is("array") || n.Is("paren") {
			bad = true
		}
		return !bad
	})
	if bad {
		return nil, fmt.Errorf("a parameter spelled %s", Lisp(p))
	}
	return x, nil
}

// s6loEndsInReturn says fd's body's last statement is a return, not one a
// label holds.
func s6loEndsInReturn(fd *Node) bool {
	items := fd.Kids[defnItemsAt(fd):]
	if len(items) == 0 || !items[len(items)-1].Is("return") {
		return false
	}
	if len(items) > 1 {
		switch items[len(items)-2].Head() {
		case "label", "case", "case-range", "default":
			return false
		}
	}
	return true
}

// ---- whether the value going in is dead --------------------------------

// s6loWritesFirst says no path from fd's body's start reads the value p
// points at before a store to *p -- a return, which sends the value back,
// counting as a read, and so does the end of the body.
func s6loWritesFirst(fd, p *Node) bool {
	w := &s6loWF{p: p}
	u := w.items(fd.Kids[defnItemsAt(fd):], true)
	return !w.fail && !u
}

type s6loWF struct {
	p    *Node
	fail bool
	sw   []bool
}

var s6loDeclHeads = map[string]bool{
	"def": true, "typedef": true, "struct": true, "union": true, "enum": true, "declare": true,
	"static_assert": true, "macro-decl": true,
}

func (w *s6loWF) items(items []*Node, u bool) bool {
	for _, it := range items {
		if w.fail {
			break
		}
		switch {
		case s6loDeclHeads[it.Head()]:
			if u && s6loNames(it, w.p) {
				w.fail = true
			}
		case it.Is("label"):
			w.fail = true
		case it.Is("case") || it.Is("case-range") || it.Is("default"):
			if len(w.sw) == 0 {
				w.fail = true
				break
			}
			u = u || w.sw[len(w.sw)-1]
		default:
			u = w.stmt(it, u)
		}
	}
	return u
}

func s6loClause(n *Node) *Node {
	if n == nil || n.list && len(n.Kids) == 0 {
		return nil
	}
	return n
}

func (w *s6loWF) expr(e *Node, u bool) bool {
	e = s6loClause(e)
	if e == nil {
		return u
	}
	if as := w.storeIn(e); as != nil {
		if u && s6loNames(as.Kids[2], w.p) {
			w.fail = true
		}
		return false
	}
	if u && s6loNames(e, w.p) {
		w.fail = true
	}
	return u
}

// storeIn is the assignment *p = R in e, when it is e's one name of p and
// no operator C evaluates lazily holds it.
func (w *s6loWF) storeIn(e *Node) *Node {
	var names []*Node
	Walk(e, func(n *Node) bool {
		if !n.list && refersTo(n, w.p) {
			names = append(names, n)
		}
		return true
	})
	if len(names) != 1 {
		return nil
	}
	up := func(n *Node) *Node {
		if n == e {
			return nil
		}
		p := n.up
		for p != nil && p != e && p.Is("paren") {
			p = p.up
		}
		if p != nil && p.Is("paren") {
			return nil
		}
		return p
	}
	d := up(names[0])
	if d == nil || !d.Is("deref") {
		return nil
	}
	as := up(d)
	if as == nil || !as.Is("=") || len(as.Kids) != 3 || s6loUnparen(as.Kids[1]) != d {
		return nil
	}
	for n := as; n != e; {
		n = n.up
		switch n.Head() {
		case "&&", "||", "?":
			return nil
		}
	}
	return as
}

func (w *s6loWF) stmt(st *Node, u bool) bool {
	if w.fail {
		return u
	}
	switch st.Head() {
	case "block":
		return w.items(blockItems(st), u)
	case "empty":
		return u
	case "if":
		u = w.expr(st.Kids[1], u)
		if len(st.Kids) == 3 {
			return w.stmt(st.Kids[2], u) || u
		}
		a := w.stmt(st.Kids[2], u)
		b := w.stmt(st.Kids[3], u)
		return a || b
	case "switch":
		u = w.expr(st.Kids[1], u)
		w.sw = append(w.sw, u)
		end := w.stmt(st.Kids[2], u)
		w.sw = w.sw[:len(w.sw)-1]
		return end || u
	case "for":
		init, cond, step, body := st.Kids[1], st.Kids[2], st.Kids[3], st.Kids[4]
		if init.Is("def") {
			if u && s6loNames(init, w.p) {
				w.fail = true
			}
		} else {
			u = w.expr(init, u)
		}
		u = w.expr(cond, u)
		w.stmt(body, u)
		w.expr(step, u)
		return u
	case "while":
		u = w.expr(st.Kids[1], u)
		w.stmt(st.Kids[2], u)
		return u
	case "do":
		u = w.expr(st.Kids[2], u)
		w.stmt(st.Kids[1], u)
		return u
	case "return":
		if u {
			w.fail = true
		}
		return false
	case "goto", "goto*":
		w.fail = true
		return false
	case "break", "continue":
		return false
	case "label":
		w.fail = true
		return u
	}
	if IsStatement(st) {
		if u && s6loNames(st, w.p) {
			w.fail = true
		}
		return u
	}
	return w.expr(st, u)
}

// ---- whether a caller reads the value back ------------------------------

// s6loLaterRead says fd reads x after call: a read -- a name of x not only
// stored to, and not an out-argument whose value in is dead (skip) --
// later in the text, or anywhere in a loop holding the call; with a goto's
// label in fd, any read at all.
func (e *Editor) s6loLaterRead(fd, x, call *Node, skip func(*Node) bool) bool {
	var loops []*Node
	for n := call.up; n != nil && n != fd; n = n.up {
		switch n.Head() {
		case "while", "do", "for":
			loops = append(loops, n)
		}
	}
	pos := map[*Node]int{}
	labels := false
	i := 0
	Walk(fd, func(n *Node) bool {
		pos[n] = i
		i++
		if n.Is("label") {
			labels = true
		}
		return true
	})
	found := false
	for _, it := range fd.Kids[defnItemsAt(fd):] {
		Walk(it, func(pe *Node) bool {
			if found {
				return false
			}
			if pe.list || !refersTo(pe, x) || s6loInside(pe, call) {
				return true
			}
			switch u := e.s6loUp(pe); {
			case u.Is("addr") && skip(u):
				return true
			case u.Is("=") && s6loUnparen(u.Kids[1]) == pe:
				return true // only stored to
			}
			if labels || pos[pe] > pos[call] {
				found = true
			}
			for _, l := range loops {
				if s6loInside(pe, l) {
					found = true
				}
			}
			return !found
		})
	}
	return found
}

// s6loStmtOf is the statement holding n, as cc's tree has one: an item
// that is not a declaration, or a block that is not a function's body;
// nil where the body is reached first.
func s6loStmtOf(n *Node) *Node {
	for c := n; c.up != nil; c = c.up {
		p := c.up
		switch {
		case p.Is("defn"):
			if !s6loDeclHeads[c.Head()] && c != defType(p) {
				return c
			}
			return nil
		case p.Is("block"):
			if !s6loDeclHeads[c.Head()] {
				return c
			}
		case IsStatement(p) && !p.Is("def") && !s6loDeclHeads[p.Head()]:
			// an if's, a loop's, a switch's, a return's: the form is
			// the statement, unless c is its branch (a block)
			if c.Is("block") {
				return c
			}
		}
	}
	return nil
}

// s6loKilledAfter says what runs after call stores to x before anything
// reads it: the value the call would write back is dead.
func (e *Editor) s6loKilledAfter(x, call *Node, deadArg func(*Node) bool) bool {
	st := s6loStmtOf(call)
	if st == nil {
		return false
	}
	names := 0
	Walk(st, func(n *Node) bool {
		if !n.list && refersTo(n, x) {
			names++
		}
		return true
	})
	if names != 1 {
		return false
	}
	jumps := func(n *Node) bool {
		found := false
		Walk(n, func(m *Node) bool {
			switch m.Head() {
			case "break", "continue", "goto", "goto*", "label", "case", "case-range", "default":
				found = true
			}
			return !found
		})
		return found
	}
	cur := st
	for {
		q := cur.up
		if q == nil {
			return false
		}
		switch {
		case q.Is("block") || q.Is("defn"):
			var items []*Node
			if q.Is("block") {
				items = blockItems(q)
			} else {
				items = q.Kids[defnItemsAt(q):]
			}
			i := 0
			for i < len(items) && items[i] != cur {
				i++
			}
			if i == len(items) {
				return false
			}
			for j := i - 1; j >= 0; j-- {
				h := items[j].Head()
				if h == "label" {
					return false
				}
				if h != "case" && h != "case-range" && h != "default" {
					break
				}
			}
			for _, bi := range items[i+1:] {
				if s6loDeclHeads[bi.Head()] {
					if s6loNames(bi, x) {
						return false
					}
					continue
				}
				switch bi.Head() {
				case "label", "case", "case-range", "default":
					return false
				}
				if s6loStored(bi, x) || e.s6loStoredByCall(bi, x, deadArg) {
					return true
				}
				if s6loNames(bi, x) || jumps(bi) {
					return false
				}
			}
			if q.Is("defn") {
				return true
			}
			cur = q
		case q.Is("if") && cur != q.Kids[1]:
			cur = q
		default:
			return false
		}
	}
}

// s6loStored says statement st is x = R;, R not naming x.
func s6loStored(st, x *Node) bool {
	if IsStatement(st) {
		return false
	}
	as := s6loUnparen(st)
	if !as.Is("=") || len(as.Kids) != 3 {
		return false
	}
	l := s6loUnparen(as.Kids[1])
	return !l.list && refersTo(l, x) && !s6loNames(as.Kids[2], x)
}

// s6loStoredByCall says every name of x in st is an out-argument whose
// value in is dead that C always evaluates.
func (e *Editor) s6loStoredByCall(st, x *Node, deadArg func(*Node) bool) bool {
	n, ok := 0, true
	Walk(st, func(m *Node) bool {
		if !ok {
			return false
		}
		if m.list || !refersTo(m, x) {
			return true
		}
		n++
		u := e.s6loUp(m)
		if u == nil || !u.Is("addr") || !deadArg(u) || s6loLazyWithin(u, st) {
			ok = false
		}
		return ok
	})
	switch st.Head() {
	case "while", "do", "for":
		return false
	}
	return ok && n > 0
}

// s6loLazyWithin says C may not evaluate n when it evaluates root.
func s6loLazyWithin(n, root *Node) bool {
	for c := n; c != nil && c != root; c = c.up {
		p := c.up
		if p == nil {
			return false
		}
		switch p.Head() {
		case "&&", "||":
			if s6loKidOf(p, c) > 1 {
				return true
			}
		case "?":
			if s6loKidOf(p, c) != 1 {
				return true
			}
		case "if", "switch":
			if s6loKidOf(p, c) != 1 {
				return true
			}
		case "while", "do", "for":
			return true
		}
	}
	return false
}
