package graph

import (
	"fmt"
	"io"
)

// Three verbs the conversions of phases 27-35 wanted (doc/GRAPH-MIGRATION.md,
// *B3b as built*), each a shape of the text verbs that B0's set did not say:
//
//   - a fold by where the if stands.  The text's `line("if (X)")` matched
//     a plain if's line and not `else if (X)`, whose spelling differs; on
//     the graph both are `(if X ...)`, the arm told apart by its place (an
//     if's else).  FoldNeverAt and FoldAlwaysAt count only the ifs of the
//     place asked for, so that the count is the text's;
//   - a cut of the matches a test takes (CutWhere): the text told two runs
//     of one statement apart by their indentation, which on the graph is
//     where the item stands;
//   - acts whose report is the caller's (Muted): the text often made
//     several acts and reported once, or not at all (a `within` with no
//     `what`); Muted runs them with their reports discarded, the first
//     refusal carried back.

// IfAt says which ifs a fold takes by where they stand.
type IfAt int

const (
	// IfAnywhere is any if whose condition matches: FoldNever's own rule.
	IfAnywhere IfAt = iota
	// IfNotArm is an if that is not an else's `if`: the text's `if (X)` line.
	IfNotArm
	// IfArm is an else's `if`: the text's `else if (X)` line.
	IfArm
)

// IsArm says the if s is an else's `if`: `else if (...)`.
func (e *Editor) IsArm(s *Node) bool {
	up := e.Parent(s)
	return s.Is("if") && up != nil && up.Is("if") && len(up.Kids) > 3 && up.Kids[3] == s
}

// placedIfs are the ifs of the scope whose condition matches pat and whose
// place is at's, refused unless there are n.
func (v *Verbs) placedIfs(at IfAt, pat string, n int, what string) ([]*Node, bool) {
	if v.Err != nil {
		return nil, false
	}
	p := v.pattern(pat, what)
	if p == nil {
		return nil, false
	}
	var ms []*Node
	for _, r := range v.roots() {
		Walk(r, func(x *Node) bool {
			if x.Is("if") && Matches(p, x.Kids[1]) {
				if arm := v.e.IsArm(x); at == IfAnywhere || arm == (at == IfArm) {
					ms = append(ms, x)
				}
			}
			return true
		})
	}
	if len(ms) != n {
		v.Die("%s -- matched %d times, expected %d -- a fold that is not counted is a guess", what, len(ms), n)
		return nil, false
	}
	return ms, true
}

// FoldNeverAt is FoldNever on the n ifs of the place at whose condition
// matches pat (a condition, never an `if` form): `if (F) A` goes, `if (F) A
// else C` is C, an arm with no else goes from its chain.
func (v *Verbs) FoldNeverAt(at IfAt, pat string, n int, what string) {
	ms, ok := v.placedIfs(at, pat, n, what)
	if !ok {
		return
	}
	if v.each(ms, what, func(s *Node) error {
		if len(s.Kids) < 4 {
			return v.e.Delete(s)
		}
		return v.e.Unwrap(s, s.Kids[3])
	}) {
		v.say(what)
	}
}

// FoldAlwaysAt is FoldAlways on the n plain ifs whose condition matches
// pat: the then arm's items in the if's place, refused for an if with an
// else.  noJump refuses, as the text's fold did, a kept branch that holds a
// break or a continue at any depth.
func (v *Verbs) FoldAlwaysAt(pat string, n int, noJump bool, what string) {
	ms, ok := v.placedIfs(IfNotArm, pat, n, what)
	if !ok {
		return
	}
	if noJump {
		for _, s := range ms {
			if holdsLoopJump(s.Kids[2]) {
				v.Die("%s -- the body kept by this fold carries a break or continue", what)
				return
			}
		}
	}
	if v.each(ms, what, func(s *Node) error {
		if len(s.Kids) > 3 {
			return fmt.Errorf("fold_always: the block has an else")
		}
		return v.e.Unwrap(s, s.Kids[2])
	}) {
		v.say(what)
	}
}

// holdsLoopJump says x holds a break or a continue, at any depth.
func holdsLoopJump(x *Node) bool {
	found := false
	Walk(x, func(y *Node) bool {
		found = found || y.Is("break") || y.Is("continue")
		return !found
	})
	return found
}

// CutWhere deletes the n matches of pat in the scope that ok accepts (all,
// when ok is nil).
func (v *Verbs) CutWhere(pat string, ok func(*Node) bool, n int, what string) {
	ms, p := v.counted(pat, n, what, ok)
	if p == nil {
		return
	}
	if v.each(ms, what, v.e.Delete) {
		v.say(what)
	}
}

// say is Say, and nothing for an act with no what.
func (v *Verbs) say(what string) {
	if what != "" {
		v.Say(what)
	}
}

// Muted runs acts in v's scope with their reports discarded; a refusal is
// v's, as any act's.
func (v *Verbs) Muted(acts func(*Verbs)) {
	if v.Err != nil {
		return
	}
	inner := &Verbs{Tag: v.Tag, W: io.Discard, e: v.e, scope: v.scope, batch: v.batch}
	acts(inner)
	if inner.Err != nil {
		v.Err = inner.Err
	}
}

// Run is the run of items the patterns name, in order: the first an item
// of the scope found once, each next one the item right after it, matching
// its pattern -- a text literal of several whole statements, as a run.
// It refuses on any other shape.
func (v *Verbs) Run(what string, pats ...string) []*Node {
	if v.Err != nil {
		return nil
	}
	if len(pats) == 0 {
		v.Die("%s -- an empty run", what)
		return nil
	}
	var run []*Node
	for k, src := range pats {
		p := v.pattern(src, what)
		if p == nil {
			return nil
		}
		if k == 0 {
			ms := v.find(p, func(x *Node) bool { return v.e.Item(x) == x })
			if len(ms) != 1 {
				v.Die("%s -- the run's first item matches %d items, expected 1", what, len(ms))
				return nil
			}
			run = append(run, ms[0])
			continue
		}
		next := v.e.Sibling(run[k-1], 1)
		if next == nil || !Matches(p, next) {
			v.Die("%s -- the run's item %d is not %s", what, k+1, src)
			return nil
		}
		run = append(run, next)
	}
	return run
}

// CutRun deletes the run of items the patterns name (Run).
func (v *Verbs) CutRun(what string, pats ...string) {
	run := v.Run(what, pats...)
	if run == nil {
		return
	}
	if err := v.e.ReplaceRun(run[0], run[len(run)-1]); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.say(what)
}

// DropArgPure is DropArg with the functions named taken as free of side
// effects: an argument that calls only them may be dropped.  A text program
// that took an argument with a call in it (`shortmess(SHM_RO) ? _("[RO]") :
// ...`) named those functions by deleting their calls; the cut names them
// here, and every other call still refuses.
func (e *Editor) DropArgPure(c *Node, i int, pure ...string) error {
	if !e.Live(c) || !c.Is("call") || 2+i >= len(c.Kids) {
		return e.DropArg(c, i) // its refusal
	}
	if a := c.Kids[2+i]; !(&closure{e: e, pureFn: set(pure)}).pure(a) {
		return fmt.Errorf("param: call #%d: argument %d has a side effect, which dropping it would lose", c.ID, i)
	}
	cl := &closure{e: e, pureFn: map[string]bool{}}
	if cl.pure(c.Kids[2+i]) {
		return e.DropArg(c, i)
	}
	// pure only by the names given: DropArg's own checks, then the deletion
	ft := typeOf(c.Kids[1])
	if ft.Is("pointer") {
		ft = pointee(ft)
	}
	params, variadic, _, ok := funcParts(ft)
	switch {
	case !ok:
		return fmt.Errorf("param: call #%d: its callee's type is not known", c.ID)
	case !variadic || i < len(params):
		return fmt.Errorf("param: call #%d: argument %d is not one its callee takes through `...`: drop the parameter", c.ID, i)
	}
	e.argLists = true
	defer func() { e.argLists = false }()
	return e.Delete(c.Kids[2+i])
}
