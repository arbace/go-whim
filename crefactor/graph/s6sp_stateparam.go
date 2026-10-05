package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// STATEPARAM (doc/GRAPH-MIGRATION.md, Step6): a set of file-scope objects
// -- one machine's state -- made the members of one struct, of which one
// file-scope instance holds what they held; every function that names one
// of them, or calls a function that does, is handed a pointer to the
// struct as its first parameter, down from the roots, which keep their
// signatures and bind it to the instance.  An object `o` is `P->o` where it
// was named.  crefactor/xform's StateParam on the graph: the call graph and
// the uses by edge, the parameter by AddParam, the struct by FRAG, each
// use a selection built in its place and typed as the import types it.

// StateParamOptions are StateParam's knobs.
type StateParamOptions struct {
	// Core: only the forms before the first include are the program's;
	// what follows (the host) may not name a function that takes the state.
	Core bool
	// Objects are the file-scope objects: each becomes a member of Type,
	// of its own name and type, in this order.
	Objects []string
	// Type is the struct's typedef name, Instance the file-scope object of
	// it that holds what the objects held, and Param the parameter a
	// function is handed it by.
	Type, Instance, Param string
	// Roots keep their signatures: each binds Param to &Instance and hands
	// it down.  Every path from a function that names an object up to one
	// nothing calls must pass through one.
	Roots []string
}

// StateParamResult is what StateParam did.
type StateParamResult struct {
	Objects int
	Takers  []string // the functions that take the state, sorted
	Roots   []string // the roots that bind it, sorted
}

// Line is the text step's report line.
func (r StateParamResult) Line(typ string) string {
	return fmt.Sprintf("%d objects are %s's members; %d functions take it, from %d roots (%s)",
		r.Objects, typ, len(r.Takers), len(r.Roots), strings.Join(r.Roots, ", "))
}

// StateParam makes the objects a parameter.  It refuses -- before anything
// moves -- when a function that needs the state is not a root and is named
// other than by a call, or the host names it, or nothing but a root bounds
// the need, or it is variadic; when an object is not one file-scope
// definition, is named outside every function, or has an initialiser that
// is not zero (the instance is zero-initialised); when a name it adds is
// taken; and when a pointer object would be selected through `->`.
func (e *Editor) StateParam(o StateParamOptions) (StateParamResult, error) {
	var r StateParamResult
	forms := e.g.Forms
	if o.Core {
		forms = e.Core()
	}
	formAt := map[*Node]int{}
	for i, f := range forms {
		formAt[f] = i
	}
	isRoot := map[string]bool{}
	for _, n := range o.Roots {
		isRoot[n] = true
	}

	// the names it adds: declared or named nowhere in the program
	for _, name := range []string{o.Type, o.Instance, o.Param} {
		taken := false
		for _, f := range forms {
			Walk(f, func(n *Node) bool {
				taken = taken || !n.list && n.Atom == name
				return !taken
			})
		}
		if taken {
			return r, fmt.Errorf("the name %s is taken", name)
		}
	}

	// the objects: one file-scope definition each, zero-initialised
	objs := map[*Node]string{}
	var defs []*Node
	for _, name := range o.Objects {
		var d *Node
		for _, f := range forms {
			if f.Is("def") && topName(f) == name && !defType(f).Is("fn") {
				if d != nil {
					return r, fmt.Errorf("%s is declared twice", name)
				}
				d = f
			}
		}
		if d == nil {
			return r, fmt.Errorf("no file-scope object %s", name)
		}
		if hasPrefix(d, "extern") || hasPrefix(d, "typedef") {
			return r, fmt.Errorf("%s is not a file-scope object", name)
		}
		i := defNameAt(d)
		if len(d.Kids) > i+3 {
			return r, fmt.Errorf("%s is declared beside another object", name)
		}
		if len(d.Kids) == i+3 && !s6spZero(newXFold(e, FoldX{}), d.Kids[i+2]) {
			return r, fmt.Errorf("%s has an initialiser that is not zero", name)
		}
		objs[d] = name
		defs = append(defs, d)
	}

	// who names an object, who calls whom
	fns := map[string]*Node{}
	for _, f := range forms {
		if f.Is("defn") {
			fns[topName(f)] = f
		}
	}
	named := map[string]bool{}
	callers := map[string]map[string]bool{}
	for _, d := range defs {
		for _, u := range e.Uses(d) {
			f := e.Function(u)
			if f == nil {
				return r, fmt.Errorf("an object is named outside every function")
			}
			named[topName(f)] = true
		}
	}
	for name, f := range fns {
		Walk(f, func(n *Node) bool {
			if n.Is("call") {
				if c := s6spCallee(n); c != "" {
					if callers[c] == nil {
						callers[c] = map[string]bool{}
					}
					callers[c][name] = true
				}
			}
			return true
		})
	}
	need := map[string]bool{}
	var stack []string
	for n := range named {
		need[n] = true
		stack = append(stack, n)
	}
	for len(stack) > 0 {
		g := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if isRoot[g] {
			continue
		}
		for c := range callers[g] {
			if !need[c] {
				need[c] = true
				stack = append(stack, c)
			}
		}
	}
	var host []byte
	hostMentions := func(name string) bool {
		if !o.Core {
			return false
		}
		if host == nil {
			host, _ = FormsC(e.Host())
		}
		return edit.MentionCount(host, name) > 0
	}
	var takers []string
	for n := range need {
		if isRoot[n] {
			continue
		}
		valued := false
		for _, d := range e.funcDecls(n) {
			for _, u := range e.Uses(d) {
				p := e.Parent(u)
				for p != nil && p.Is("paren") {
					u, p = p, e.Parent(p)
				}
				if p == nil || !p.Is("call") || p.Kids[1] != u {
					valued = true
				}
			}
		}
		_, variadic, _, ok := funcParts(fns[n].Type)
		switch {
		case valued:
			return r, fmt.Errorf("%s needs the state and its address is taken", n)
		case hostMentions(n):
			return r, fmt.Errorf("%s needs the state and the host names it", n)
		case len(callers[n]) == 0:
			return r, fmt.Errorf("%s needs the state and nothing in the core calls it: no root bounds it", n)
		case !ok || variadic:
			return r, fmt.Errorf("%s needs the state and is variadic", n)
		}
		takers = append(takers, n)
	}
	sort.Strings(takers)
	var roots []string
	for n := range need {
		if isRoot[n] {
			roots = append(roots, n)
		}
	}
	sort.Strings(roots)
	for _, d := range defs {
		for _, u := range e.Uses(d) {
			if p := e.Parent(u); p.Is("->") && p.Kids[1] == u {
				return r, fmt.Errorf("%s is selected through `->`: the selection would be another chain", objs[d])
			}
		}
	}

	// where the struct goes: the first object's place, the typedefs of the
	// objects' types declared after it moved up to it; and, where a function
	// that takes it is declared before that, the type declared first there
	first := defs[0]
	for _, d := range defs {
		if formAt[d] < formAt[first] {
			first = d
		}
	}
	var moved []*Node
	for _, d := range defs {
		t := defType(d)
		for t.Is("array") {
			t = t.Kids[len(t.Kids)-1]
		}
		td := t.Ref()
		if t.list || td == nil || !td.Is("typedef") {
			continue
		}
		at, ok := formAt[td]
		if !ok || at <= formAt[first] || s6spHas(moved, td) {
			continue
		}
		moved = append(moved, td)
	}
	var early *Node
	for _, n := range takers {
		for _, d := range e.funcDecls(n) {
			if at, ok := formAt[d]; ok && at < formAt[first] && (early == nil || at < formAt[early]) {
				early = d
			}
		}
	}

	// the members, each the object's declaration without its storage class
	// and its initialiser
	var members strings.Builder
	for _, d := range defs {
		i := defNameAt(d)
		m := NewList(NewAtom("def"))
		for _, k := range d.Kids[1:i] {
			if k.list || k.Atom != "static" {
				e.g.save(m)
				m.Kids = append(m.Kids, Clone(k))
			}
		}
		e.g.save(m)
		m.Kids = append(m.Kids, Clone(d.Kids[i]), Clone(d.Kids[i+1]))
		c, err := FormsC([]*Node{m})
		if err != nil {
			return r, err
		}
		members.WriteString("    " + strings.TrimSpace(string(c)) + "\n")
	}

	// made: the typedefs moved, the struct, the instance and the roots'
	// locals in one unit
	for _, td := range moved {
		if err := e.MoveBefore(td, first); err != nil {
			return r, fmt.Errorf("moving %s up to the struct: %w", topName(td), err)
		}
	}
	ptr := o.Type + " *" + o.Param
	var fs []Frag
	if early != nil {
		tag := strings.TrimSuffix(o.Type, "_T") + "_S"
		fs = append(fs,
			Frag{At: e.SpotBefore(early), Src: "typedef struct " + tag + " " + o.Type + ";\n"},
			Frag{At: e.SpotBefore(first), Src: "struct " + tag + "\n{\n" + members.String() + "};\n\nstatic " + o.Type + " " + o.Instance + ";\n"})
	} else {
		fs = append(fs, Frag{At: e.SpotBefore(first), Src: "typedef struct\n{\n" + members.String() + "} " + o.Type + ";\n\nstatic " + o.Type + " " + o.Instance + ";\n"})
	}
	nfs := len(fs)
	for _, n := range roots {
		items := Body(fns[n])
		if len(items) == 0 {
			return r, fmt.Errorf("the root %s has an empty body", n)
		}
		fs = append(fs, Frag{At: e.SpotBefore(items[0]), Src: ptr + " = &" + o.Instance + ";\n"})
	}
	made, err := e.SpliceC(fs...)
	if err != nil {
		return r, err
	}
	local := map[string]*Node{}
	for k, n := range roots {
		local[n] = made[nfs+k][0]
	}

	// the parameter: every call is first handed a placeholder use of a
	// root's local of the same name and type, then pointed at the
	// parameter or local its function holds
	var holders []*Node
	if len(roots) == 0 {
		return r, fmt.Errorf("no root needs the state")
	}
	anyLocal := local[roots[0]]
	for _, n := range takers {
		_, err := e.AddParam(n, 0, "("+o.Param+" (ptr "+o.Type+"))", func(c *Node) (string, Bindings) {
			h := &Node{Atom: o.Param, Refs: []*Node{anyLocal}}
			holders = append(holders, h)
			return "?h", Bindings{"h": h}
		})
		if err != nil {
			return r, fmt.Errorf("%s: %w", n, err)
		}
	}
	for _, h := range holders {
		f := e.Function(h)
		if f == nil {
			return r, fmt.Errorf("a call handed the state is outside every function")
		}
		to := local[topName(f)]
		if to == nil {
			to = paramNamed(f, o.Param)
		}
		if to == nil {
			return r, fmt.Errorf("%s calls a function that takes the state and holds none", topName(f))
		}
		if err := e.Retarget(h, 0, to); err != nil {
			return r, err
		}
	}

	// each use `o` is `P->o`: the member's type, an array's decayed but
	// under `&`, as the import types it; nothing above it changes type
	tx := e.typeTx()
	for _, d := range defs {
		for _, u := range e.Uses(d) {
			ns, err := e.Build(u, "(-> "+o.Param+" "+objs[d]+")", nil)
			if err != nil {
				return r, err
			}
			sel := ns[0]
			if sel.Type.Is("array") && !e.Parent(u).Is("addr") {
				e.g.save(sel)
				sel.Type = tx.pointer(pointee(sel.Type))
			}
			e.g.save(u)
			u.Type = sel.Type // the value is the same: what holds it keeps its type
			if err := e.Replace(u, sel); err != nil {
				return r, err
			}
		}
	}
	tx.commit()
	for _, d := range defs {
		if err := e.Delete(d); err != nil {
			return r, err
		}
	}
	r.Objects, r.Takers, r.Roots = len(defs), takers, roots
	return r, nil
}

// s6spCallee is the name of the function a call calls by name, or "".
func s6spCallee(c *Node) string {
	f := c.Kids[1]
	for f.Is("paren") && len(f.Kids) == 2 {
		f = f.Kids[1]
	}
	if f.list {
		return ""
	}
	if d := f.Ref(); d != nil && (d.Is("defn") || d.Is("def") && defType(d).Is("fn")) {
		return topName(d)
	}
	return ""
}

// s6spZero says an initialiser is all zeros: each value a constant zero or
// a null pointer.
func s6spZero(x *xfold, in *Node) bool {
	if in.Is("init") {
		for _, k := range in.Args() {
			if k.Is("at") { // a designator and its value
				k = k.Kids[len(k.Kids)-1]
			}
			if !s6spZero(x, k) {
				return false
			}
		}
		return true
	}
	v, _, ok := x.xconst(in)
	return ok && v.zero()
}

func s6spHas(ns []*Node, n *Node) bool {
	for _, k := range ns {
		if k == n {
			return true
		}
	}
	return false
}
