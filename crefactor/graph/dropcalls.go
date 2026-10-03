package graph

import (
	"fmt"
	"strings"
)

// DROPCALLS (doc/GRAPH-MIGRATION.md, B3g): crefactor/xform's DropCalls on
// the nodes.  Every call statement of a function that does nothing goes
// from the forms named: a call whose argument has no side effect goes, and
// one whose only effect is one ++ or -- becomes that step, as a statement
// of its own; any other argument refuses.  A local then only given values
// goes with its stores (DeadLocals), and calls elsewhere are pointed at
// another function, as told.

// DropCallsOptions are what DropCalls is told.
type DropCallsOptions struct {
	// In says which top-level forms the calls go from.
	In func(form *Node) bool
	// Funcs are the functions whose calls do nothing.
	Funcs []string
	// Redirect, pair by pair, points the calls of a function outside In
	// at another: {from, to}.
	Redirect [][2]string
	// Cond is DeadLocals' test of a value's text (EmptyOptions.Cond).
	Cond func(text string) bool
}

// DropCallsReport is what DropCalls did.
type DropCallsReport struct {
	Calls, Steps, Redirected int
	Locals                   []string
}

// Lines is the report as the text step wrote it.
func (r DropCallsReport) Lines(funcs []string) []string {
	named := strings.Join(funcs, "(), ") + "()"
	return []string{
		fmt.Sprintf("%d calls to %s in the core go -- %d of them keep the ++ or -- their argument did, as a statement of its own -- and %d of the host's calls are redirected", r.Calls, named, r.Steps, r.Redirected),
		fmt.Sprintf("%d locals were only passed to them and given values, and go with their stores: %s", len(r.Locals), strings.Join(r.Locals, " ")),
	}
}

// DropCalls drops the calls, then the locals left only given values, then
// redirects.
func (e *Editor) DropCalls(o DropCallsOptions) (DropCallsReport, error) {
	var r DropCallsReport
	if len(o.Funcs) == 0 {
		return r, fmt.Errorf("no function was named")
	}
	funcs := map[string]bool{}
	for _, f := range o.Funcs {
		funcs[f] = true
	}
	type drop struct{ call, step *Node }
	var drops []drop
	for _, f := range e.g.Forms {
		if !f.Is("defn") || o.In != nil && !o.In(f) {
			continue
		}
		var err error
		itemLists(f, func(items []*Node) {
			for _, it := range items {
				if err != nil || !it.Is("call") || it.Kids[1].list || !funcs[it.Kids[1].Atom] {
					continue
				}
				step, ok := dcStep(it.Kids[2:])
				if !ok {
					a, _ := ExprText(it)
					err = fmt.Errorf("%s does something this step cannot keep", a)
					continue
				}
				drops = append(drops, drop{it, step})
			}
		})
		if err != nil {
			return r, err
		}
	}
	for _, d := range drops {
		var err error
		if d.step == nil {
			err = e.Delete(d.call)
		} else {
			err = e.Replace(d.call, d.step)
			r.Steps++
		}
		if err != nil {
			return r, err
		}
		r.Calls++
	}
	took, err := e.DeadLocals(EmptyOptions{In: o.In, Cond: o.Cond})
	if err != nil {
		return r, err
	}
	r.Locals = took
	for _, rd := range o.Redirect {
		to := e.FileDecls(rd[1])
		if len(to) == 0 {
			return r, fmt.Errorf("redirect %s: no %s to point its calls at", rd[0], rd[1])
		}
		for _, d := range e.FileDecls(rd[0]) {
			for _, u := range e.Uses(d) {
				if o.In != nil && o.In(e.TopForm(u)) {
					continue
				}
				if p, i := e.index(u); p == nil || !p.Is("call") || i != 1 {
					continue
				}
				if err := e.RetargetAs(u, 0, to[0]); err != nil {
					return r, err
				}
				r.Redirected++
			}
		}
	}
	return r, nil
}

// dcStep is what a call with the arguments args leaves: nothing (nil, true)
// when they have no side effect, their one ++ or -- when that is their only
// effect, and false otherwise.
func dcStep(args []*Node) (*Node, bool) {
	var steps []*Node
	ok := true
	for _, a := range args {
		Walk(a, func(x *Node) bool {
			if !ok || !x.list {
				return false
			}
			switch h := x.Head(); {
			case incDec[h]:
				steps = append(steps, x)
				return false
			case assignOps[h], h == "call", h == "stmt-expr", h == "sizeof", h == "alignof":
				ok = false
			}
			return ok
		})
	}
	switch {
	case !ok || len(steps) > 1:
		return nil, false
	case len(steps) == 1:
		return steps[0], true
	}
	return nil, true
}

// DeadLocals takes each local only ever given a value -- every use the
// left side of `x = E;` alone as a statement, E and its initialiser free of
// side effects (and passing o.Cond) -- with its stores, in the forms o.In
// says yes to, to the fixed point: crefactor/edit's DeadStores, the half of
// EmptyBlocks that takes the locals.
func (e *Editor) DeadLocals(o EmptyOptions) ([]string, error) {
	c := &closure{e: e, pureFn: map[string]bool{}, through: map[string]bool{}, st: &FallOutStats{}, cond: o.Cond}
	return c.deadLocals(o.In)
}
