package graph

import (
	"fmt"
	"slices"
)

// AddParamC is AddParam (param.go) with the parameter written in C -- `void
// (*exit_fn)(int)` -- and made by FRAG in each declaration's place, so that
// it may be of any type the C can say: a pointer to a function, which BUILD
// does not make.  Its argument at every call is what arg says, as
// AddParam's.  The rules are AddParam's: every use of fn a call, the name
// neither declared at the top of the definition's body nor hiding a name
// the body uses.
func (e *Editor) AddParamC(fn string, i int, param string, arg func(c *Node) (string, Bindings)) (ParamStats, error) {
	var st ParamStats
	ds := e.funcDecls(fn)
	if len(ds) == 0 {
		return st, fmt.Errorf("param: no function %s is declared", fn)
	}
	var calls []*Node
	for _, d := range ds {
		for _, u := range e.Uses(d) {
			p := e.Parent(u)
			for p != nil && p.Is("paren") {
				u, p = p, e.Parent(p)
			}
			if p == nil || !p.Is("call") || p.Kids[1] != u {
				return st, fmt.Errorf("param: %s is used at #%d in %s other than as a callee: its type would no longer fit", fn, u.ID, inFn(e, u))
			}
			calls = append(calls, p)
		}
	}
	// the parameter, made in each declaration's place: the one parameter
	// of a prototype written before it
	type made struct{ d, p *Node }
	var ms []made
	var name string
	for _, d := range ds {
		top := e.topOf(d)
		out, err := e.MakeC(Frag{At: e.SpotBefore(top), Src: "void __whim_param_of_(" + param + ");"})
		if err != nil {
			return st, fmt.Errorf("param: %q: %w", param, err)
		}
		if len(out) != 1 || len(out[0]) != 1 || defType(out[0][0]) == nil || len(paramElems(defType(out[0][0]))) != 1 {
			return st, fmt.Errorf("param: %q is not one parameter", param)
		}
		p := paramElems(defType(out[0][0]))[0]
		if p.Type == nil || !p.list || p.Kids[0].list {
			return st, fmt.Errorf("param: %q is not one named, typed parameter", param)
		}
		name = p.Kids[0].Atom
		ms = append(ms, made{d, p})
	}
	if f := e.Defn(fn); f != nil {
		if paramNamed(f, name) != nil {
			return st, fmt.Errorf("param: %s already has a parameter %s", fn, name)
		}
		for _, it := range Body(f) {
			if (it.Is("def") || it.Is("typedef")) && topName(it) == name {
				return st, fmt.Errorf("param: %s declares %s at the top of its body", fn, name)
			}
		}
		var hidden *Node
		Walk(f, func(n *Node) bool {
			for _, r := range n.Refs {
				if hidden == nil && ordinaryName(r) == name && e.Function(r) == nil && !isMember(e, r) {
					hidden = n
				}
			}
			return hidden == nil
		})
		if hidden != nil {
			return st, fmt.Errorf("param: %s's body names the file's %s at #%d, which the parameter would hide", fn, name, hidden.ID)
		}
	}
	tx := e.typeTx()
	plain := e.typeTx()
	newT := map[*Node]*Node{}
	for _, m := range ms {
		fnf := defType(m.d)
		if plain.formType(fnf, false) != m.d.Type {
			return st, fmt.Errorf("param: the type form of %s does not say its type node", label(m.d))
		}
		if i < 0 || i > len(paramElems(fnf)) {
			return st, fmt.Errorf("param: %s has no place %d for a parameter", fn, i)
		}
		params, variadic, result, ok := funcParts(m.d.Type)
		if !ok {
			return st, fmt.Errorf("param: %s's type is not a function's", label(m.d))
		}
		newT[m.d] = tx.function(slices.Insert(slices.Clone(params), i, m.p.Type), variadic, result)
	}
	args := make([]*Node, len(calls))
	for k, c := range calls {
		src, holes := arg(c)
		ns, err := e.Build(c, src, holes)
		if err != nil {
			return st, fmt.Errorf("param: the argument at call #%d: %w", c.ID, err)
		}
		if len(ns) != 1 || IsStatement(ns[0]) {
			return st, fmt.Errorf("param: the argument at call #%d is not one expression", c.ID)
		}
		if 2+i > len(c.Kids) {
			return st, fmt.Errorf("param: call #%d has fewer than %d arguments", c.ID, i)
		}
		args[k] = ns[0]
	}
	tx.commit()
	e.argLists = true
	defer func() { e.argLists = false }()
	for _, m := range ms {
		pl := defType(m.d).Kids[1]
		var err error
		if len(paramElems(defType(m.d))) == 0 && len(pl.Kids) == 1 {
			err = e.Replace(pl.Kids[0], m.p)
		} else {
			err = e.splice("insert", pl, i, i, []*Node{m.p})
		}
		if err != nil {
			return st, err
		}
		e.typedAll(m.p)
		st.Params++
	}
	for k, c := range calls {
		if err := e.splice("insert", c, 2+i, 2+i, []*Node{args[k]}); err != nil {
			return st, err
		}
		st.Args++
		st.Calls++
	}
	e.argLists = false
	act := Act{Op: "retype"}
	for _, m := range ms {
		e.g.save(m.d)
		m.d.Type = newT[m.d]
		act.Moved = append(act.Moved, m.d.ID)
		st.Retyped++
	}
	e.Log = append(e.Log, act)
	return st, nil
}
