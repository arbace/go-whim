package reach

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// THE CAST GUARD.  A pointer to one struct or union cast to a pointer to
// ANOTHER is a pun: the code reads the object through the other type, and
// what it reads is the common initial sequence -- `(regprog_T *)bt_prog` reads
// re_engine, re_flags and re_in_use of a bt_regprog_T by their offsets in
// regprog_T.  A member nothing names through its own type is still read
// there, and deleting it moves every member after it: measured on q82, the
// closure deleted bt_regprog_T's re_engine and re_flags, re_in_use moved, and
// every pattern said E956.
//
// So every member of BOTH sides of such a cast is held.  Not only the common
// initial sequence: which prefix the two share is a fact about their current
// layouts, and a member after it is the cheapest thing to over-keep.  A held
// member that the closure did not otherwise reach is put in ClassCast, and
// what it names is reached from it.
//
// Through void *.  An object stored as void * and read back as a different
// struct is the same hazard in two casts, or in none: C converts to and from
// void * implicitly, so most of those conversions are no cast at all and no
// walk of the casts can pair a writer with its reader.  That is why they are
// NOT held: holding every struct that meets a void * would be a guess at the
// pairing, not a measurement of it.  What IS measured, and reported, is how
// many struct types meet an explicit void * cast in each direction, and how
// many do both (PunStats) -- the size of what this guard cannot see.  A
// round trip through void * as the SAME type is no hazard: the writer and the
// reader agree on the layout whatever is deleted from it.

// PunStats is what the cast guard found.
type PunStats struct {
	Pairs []string // "A <-> B", every distinct pair of struct types a cast puns
	Held  []string // the member IDs held that the closure did not otherwise reach

	// Explicit casts through void *, by distinct struct type.
	FromVoid      []string // cast from void * to a pointer to it, an allocator's result excluded
	FromAllocator []string // cast from an allocator's void * result
	ToVoid        []string // cast from a pointer to it to void *
	RoundTrip     []string // both ToVoid and FromVoid
}

var allocators = map[string]bool{"alloc": true, "alloc_clear": true, "lalloc": true, "lalloc_clear": true, "host_alloc": true}

type fielded interface {
	NumFields() int
	FieldByIndex(int) *cc.Field
}

// structAt is the definition offset of the struct or union t is, when the
// closure has its members; -1 otherwise.
func (c *Closure) structAt(t cc.Type, fieldKey map[int]string) int {
	f, ok := t.(fielded)
	if !ok || t.Kind() != cc.Struct && t.Kind() != cc.Union {
		return -1
	}
	for i := 0; i < f.NumFields(); i++ {
		fl := f.FieldByIndex(i)
		if fl == nil || fl.Declarator() == nil {
			continue
		}
		if k, ok := fieldKey[fl.Declarator().Position().Offset]; ok {
			return c.structOf[k]
		}
	}
	return -1
}

// structName names a struct by its members' owner, as their IDs do.
func (c *Closure) structName(at int) string {
	ms := c.members[at]
	if len(ms) == 0 {
		return fmt.Sprintf("<struct at %d>", at)
	}
	id := c.byKey[ms[0]].ID
	return strings.TrimPrefix(id[:strings.LastIndexByte(id, '.')], "M:")
}

func pointee(t cc.Type) cc.Type {
	switch x := t.(type) {
	case *cc.PointerType:
		return x.Elem()
	case *cc.ArrayType:
		return x.Elem()
	}
	return nil
}

func calleeOf(e cc.ExpressionNode) string {
	for {
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case != cc.PrimaryExpressionExpr {
				return ""
			}
			e = x.ExpressionList
		case *cc.ExpressionList:
			if x.ExpressionList != nil {
				return ""
			}
			e = x.AssignmentExpression
		case *cc.PostfixExpression:
			if x.Case != cc.PostfixExpressionCall {
				return ""
			}
			if p, ok := x.PostfixExpression.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
				return p.Token.SrcStr()
			}
			return ""
		default:
			return ""
		}
	}
}

// punned walks every cast, records the puns and the void * traffic, and
// roots every member of a punned struct the closure has not reached.  It
// returns the keys to propagate from.
func (c *Closure) punned(ast *cc.AST, path string, fieldKey map[int]string) []string {
	pun := map[int]bool{}
	pairs := map[string]bool{}
	fromVoid, fromAlloc, toVoid := map[int]bool{}, map[int]bool{}, map[int]bool{}
	walk(ast.TranslationUnit, func(n cc.Node) {
		x, ok := n.(*cc.CastExpression)
		if !ok || x.Case != cc.CastExpressionCast || x.Position().Filename != path {
			return
		}
		to := x.Type()
		if to == nil || to.Kind() != cc.Ptr || x.CastExpression == nil {
			return
		}
		from := x.CastExpression.Type()
		if from == nil || (from.Kind() != cc.Ptr && from.Kind() != cc.Array) {
			return
		}
		te, fe := pointee(to), pointee(from)
		if te == nil || fe == nil {
			return
		}
		ts, fs := c.structAt(te, fieldKey), c.structAt(fe, fieldKey)
		switch {
		case ts >= 0 && fs >= 0 && ts != fs:
			pun[ts], pun[fs] = true, true
			a, b := c.structName(ts), c.structName(fs)
			if a > b {
				a, b = b, a
			}
			pairs[a+" <-> "+b] = true
		case ts >= 0 && fe.Kind() == cc.Void:
			if allocators[calleeOf(x.CastExpression)] {
				fromAlloc[ts] = true
			} else {
				fromVoid[ts] = true
			}
		case fs >= 0 && te.Kind() == cc.Void:
			toVoid[fs] = true
		}
	})
	names := func(m map[int]bool) []string {
		var out []string
		for at := range m {
			out = append(out, c.structName(at))
		}
		sort.Strings(out)
		return out
	}
	rt := map[int]bool{}
	for at := range toVoid {
		if fromVoid[at] {
			rt[at] = true
		}
	}
	c.Pun = PunStats{FromVoid: names(fromVoid), FromAllocator: names(fromAlloc), ToVoid: names(toVoid), RoundTrip: names(rt)}
	for p := range pairs {
		c.Pun.Pairs = append(c.Pun.Pairs, p)
	}
	sort.Strings(c.Pun.Pairs)

	var stack []string
	for at := range pun {
		for _, k := range c.members[at] {
			e := c.byKey[k]
			if e.reached {
				continue
			}
			c.root(ClassCast, e)
			c.Pun.Held = append(c.Pun.Held, e.ID)
			stack = append(stack, k)
		}
	}
	sort.Strings(c.Pun.Held)
	return stack
}
