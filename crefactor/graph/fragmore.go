package graph

import "github.com/arbace/go-whim/crefactor/clisp"

// Two FRAG verbs more (B3f), for the text programs' literals that name part
// of an item -- an expression, a condition, an argument -- and for C put at
// a spot no pattern names (before a function's definition, which follows
// its prototype).

// LiteralExprC replaces each of the n expressions of the scope that pat
// matches and whose C is old -- spaces aside, outside literals, as the
// printer writes the expression alone, without the parentheses its context
// would add -- by the C new.  It is LiteralC for a text literal that named
// part of an item; pat narrows where it looks.
func (v *Verbs) LiteralExprC(pat, old, new string, n int, what string) {
	key := normC(old)
	if key == "" && v.Err == nil {
		v.Die("%s -- the literal is empty", what)
	}
	ms, _ := v.counted(pat, n, what, func(m *Node) bool {
		c, err := clisp.PrintExpr(Lisp(m))
		return err == nil && normC(c) == key
	})
	if v.Err != nil {
		return
	}
	fs := make([]Frag, len(ms))
	for i, m := range ms {
		fs[i] = Frag{At: v.e.SpotOf(m), Src: new}
	}
	v.spliceC(what, fs...)
}

// FragAt puts the C src at the spot, an act of its own -- deferred with the
// rest in Together.
func (v *Verbs) FragAt(at Spot, src, what string) {
	v.spliceC(what, Frag{At: at, Src: src})
}

// ReplaceEachC replaces each of the n nodes ns, found by the caller, by the
// C src: one act, counted.
func (v *Verbs) ReplaceEachC(ns []*Node, src string, n int, what string) {
	if v.Err != nil {
		return
	}
	if len(ns) != n {
		v.Die("%s -- %d places, expected %d", what, len(ns), n)
		return
	}
	fs := make([]Frag, len(ns))
	for i, m := range ns {
		fs[i] = Frag{At: v.e.SpotOf(m), Src: src}
	}
	v.spliceC(what, fs...)
}

// AfterEachC puts the items src after each of the n items ns, found by the
// caller: one act, counted.  BeforeEachC puts them before each.
func (v *Verbs) AfterEachC(ns []*Node, src string, n int, what string) {
	v.besideEachC(ns, src, n, what, true)
}

// BeforeEachC puts the items src before each of the n items ns.
func (v *Verbs) BeforeEachC(ns []*Node, src string, n int, what string) {
	v.besideEachC(ns, src, n, what, false)
}

func (v *Verbs) besideEachC(ns []*Node, src string, n int, what string, after bool) {
	if v.Err != nil {
		return
	}
	if len(ns) != n {
		v.Die("%s -- %d places, expected %d", what, len(ns), n)
		return
	}
	fs := make([]Frag, len(ns))
	for i, m := range ns {
		at := v.e.SpotBefore(m)
		if after {
			at = v.e.SpotAfter(m)
		}
		fs[i] = Frag{At: at, Src: src}
	}
	v.spliceC(what, fs...)
}

// WrapEachC replaces each of the n nodes ns by the C src in which $hole is
// that node, moved in with its ids: a statement wrapped in an if, an
// expression in a call.  One act, counted.
func (v *Verbs) WrapEachC(ns []*Node, hole, src string, n int, what string) {
	if v.Err != nil {
		return
	}
	if len(ns) != n {
		v.Die("%s -- %d places, expected %d", what, len(ns), n)
		return
	}
	fs := make([]Frag, len(ns))
	for i, m := range ns {
		fs[i] = Frag{At: v.e.SpotOf(m), Src: src, Holes: Bindings{hole: m}}
	}
	v.spliceC(what, fs...)
}
