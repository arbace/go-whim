package togo

// clj_ed.go is which functions do without the editor (doc/CLOJURE-IDIOMS.md,
// item 7): a function that names no file-scope object and no hoisted
// static -- the editor's slots -- calls no host function and nothing through
// a pointer, takes no function's address, and calls only functions that do
// without it, is a Clojure function of its arguments, `(defn getdigits ^long
// [^BytePtr pp] ...)`, called without `ed`.  Closed over the calls, and
// held to what the printer writes: a function whose printed text names `ed`
// all the same (the groups of a state machine, an outlined region take it)
// keeps it, and so does every function that calls one that keeps it.
//
// What keeps the editor whatever it does: a function used as a value, whose
// signature is its table's; one the glue calls by name (Profile.CljGlue);
// and one whose runtime body names `ed`.

import (
	"regexp"

	"github.com/arbace/go-whim/crefactor/cc"
)

// edFree is the functions that do without the editor, before the printer
// has had its say (edHeld).
func (c *cgen) edFree(fds []*cc.FunctionDefinition) map[string]bool {
	p := c.g.p
	needs := map[string]bool{}
	calls := map[string][]string{}
	defined := map[string]bool{}
	for _, fd := range fds {
		defined[fd.Declarator.Name()] = true
	}
	for _, n := range p.CljGlue {
		needs[n] = true
	}
	for _, rb := range p.RuntimeBodies {
		if rb.Clj != nil && edWord.MatchString(rb.Clj("")) {
			needs[rb.Name] = true
		}
	}
	for _, fd := range fds {
		name := fd.Declarator.Name()
		if c.g.a.addr[name] {
			needs[name] = true // a value: its table's signature
		}
		walkNodes(fd.CompoundStatement, func(n cc.Node) {
			if needs[name] {
				return
			}
			switch x := n.(type) {
			case *cc.PrimaryExpression:
				d := identDecl(x)
				if d == nil || d.Type() == nil {
					return
				}
				if d.Type().Kind() == cc.Function {
					return // a call's designator, or a value (addr, above)
				}
				if d.StorageDuration() == cc.Static {
					needs[name] = true // a slot of the editor
				}
			case *cc.PostfixExpression:
				if x.Case != cc.PostfixExpressionCall {
					return
				}
				d := fnDesignator(x.PostfixExpression)
				switch {
				case d == nil:
					needs[name] = true // through a pointer: a value's signature
				case defined[d.Name()]:
					calls[name] = append(calls[name], d.Name())
				case d.Name() == "__builtin_expect", p.allocators[d.Name()], p.frees[d.Name()],
					p.byteMove(d.Name()), d.Name() == p.Bytes.Set, d.Name() == p.Bytes.Cmp:
					// the runtime's, which needs no editor
				default:
					needs[name] = true // the host
				}
			}
		})
	}
	closeNeeds(needs, calls)
	free := map[string]bool{}
	for _, fd := range fds {
		if n := fd.Declarator.Name(); !needs[n] {
			free[n] = true
		}
	}
	return free
}

// closeNeeds marks every function that calls one that needs the editor.
func closeNeeds(needs map[string]bool, calls map[string][]string) {
	for changed := true; changed; {
		changed = false
		for f, cs := range calls {
			if needs[f] {
				continue
			}
			for _, c := range cs {
				if needs[c] {
					needs[f], changed = true, true
					break
				}
			}
		}
	}
}

var edWord = regexp.MustCompile(`(^|[\s(\[])ed([\s)\]]|$)`)

// edHeld is free without the functions whose printed text names `ed`
// all the same, and the functions that call them; and whether it changed.
func (c *cgen) edHeld(free map[string]bool, texts map[string]string, fds []*cc.FunctionDefinition) bool {
	needs := map[string]bool{}
	calls := map[string][]string{}
	changed := false
	for _, fd := range fds {
		name := fd.Declarator.Name()
		if !free[name] {
			needs[name] = true
			continue
		}
		if edWord.MatchString(texts[name]) {
			needs[name] = true
			changed = true
		}
		walkNodes(fd.CompoundStatement, func(n cc.Node) {
			if x, ok := n.(*cc.PostfixExpression); ok && x.Case == cc.PostfixExpressionCall {
				if d := fnDesignator(x.PostfixExpression); d != nil {
					calls[name] = append(calls[name], d.Name())
				}
			}
		})
	}
	if !changed {
		return false
	}
	closeNeeds(needs, calls)
	for n := range free {
		if needs[n] {
			delete(free, n)
		}
	}
	return true
}
