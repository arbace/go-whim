package togo

// hsshape.go is the shape a lowered function is printed in (doc/
// HASKELL-IDIOMS.md, item 2).  Every basic block could be a local function
// and every jump a tail call -- which is what caprice printed first -- but a
// block that one jump reaches is only the rest of the code that jumps: it is
// written there, straight on after a goto, or as the arm of the if or the
// case that goes to it.  A block that does nothing but jump is where it
// jumps.  What is left as a local function is what a Haskell programmer
// writes as one: a join, which several places go on to, and a loop's head,
// which a jump back reaches (loop'N) -- each a function of the variables live
// at its start.  So no block is copied and there is no state machine: a
// local function may be called from anywhere in its scope, which is what
// the Clojure's recur cannot say (clj_shape.go).  The order of every read,
// write and call is the lowered form's; only where its lines are printed
// moves.

import "strconv"

// shape decides, for the function being printed, where each block goes.
func (f *hfn) shape() {
	lf := f.lf
	f.fixed = f.unassigned()
	f.fwd = map[*lblock]*lblock{}
	for _, b := range lf.blocks {
		if len(b.steps) == 0 && b.term.kind == tGoto && b.term.to[0] != b {
			f.fwd[b] = b.term.to[0]
		}
	}
	f.start = f.resolve(lf.blocks[0])
	// the jumps into each block, from the blocks that are printed, and the
	// function's start
	n := map[*lblock]int{f.start: 1}
	f.loop = map[*lblock]bool{}
	for _, b := range lf.blocks {
		if f.resolve(b) != b {
			continue // it only jumps: it is not printed
		}
		for _, s := range b.term.to {
			t := f.resolve(s)
			n[t]++
			if t.id <= b.id {
				f.loop[t] = true // a jump back, in reverse postorder
			}
		}
	}
	f.inline = map[*lblock]bool{}
	for _, b := range lf.blocks {
		if f.resolve(b) == b && (n[b] == 1 && !f.loop[b] || f.onlyResult(b)) {
			f.inline[b] = true
		}
	}
}

// onlyResult says b does nothing but return a value that reads nothing -- no
// value, a binding variable, a constant: `pure x`, which is written where
// each jump to it is rather than as a join of its own.
func (f *hfn) onlyResult(b *lblock) bool {
	if len(b.steps) > 0 || f.sret {
		return false
	}
	switch t := b.term; t.kind {
	case tFall:
		return true
	case tRet:
		switch {
		case t.ret.isZero():
			return true
		case t.ret.v != nil:
			return f.reg(t.ret.v)
		case t.ret.n != nil:
			_, known := intValue(t.ret.n.Value())
			return known && !hasEffect(t.ret.n) && !f.hasSub(t.ret.n)
		}
	}
	return false
}

// resolve is where a jump to b goes: b, or, when b only jumps, where it
// jumps -- a chain of them followed, a cycle of them left as it is.
func (f *hfn) resolve(b *lblock) *lblock {
	seen := map[*lblock]bool{}
	for t := b; ; {
		next, ok := f.fwd[t]
		if !ok {
			return t
		}
		if seen[t] {
			return b
		}
		seen[t] = true
		t = next
	}
}

// local says b is printed as a local function: it is printed, and not in
// place.
func (f *hfn) local(b *lblock) bool {
	return f.resolve(b) == b && !f.inline[b]
}

// bname is a local function's name: a loop's head is a loop.
func (f *hfn) bname(b *lblock) string {
	if f.loop[b] {
		return "loop'" + strconv.Itoa(b.id)
	}
	return "j'" + strconv.Itoa(b.id)
}

// unassigned are the parameters no step assigns: each is its one name
// everywhere in the function, read from its scope, and no local function
// takes it.
func (f *hfn) unassigned() map[*lvar]bool {
	set := map[*lvar]bool{}
	for _, b := range f.lf.blocks {
		for _, s := range b.steps {
			switch s.op {
			case opSet, opInit:
				set[s.dst] = true
			case opAssign, opAssignOp, opIncDec:
				if v := f.lf.lhsVar(s.lhs); v != nil {
					set[v] = true
				}
			}
		}
	}
	fixed := map[*lvar]bool{}
	for _, v := range f.lf.params {
		if !set[v] && f.reg(v) {
			fixed[v] = true
		}
	}
	return fixed
}

// hsUnparen is s without the parentheses around the whole of it, if they
// are.
func hsUnparen(s string) string {
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return s
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '\'':
			if i > 0 && hsIdentChar(s[i-1]) {
				continue // a name's prime
			}
			// a character literal
			i++
			if i < len(s) && s[i] == '\\' {
				i++
			}
			i++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i < len(s)-1 {
				return s // closed before the end: (a) + (b)
			}
		}
	}
	return s[1 : len(s)-1]
}
