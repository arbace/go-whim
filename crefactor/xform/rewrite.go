package xform

// rewrite.go is text rewrites that may nest: a rewrite of a span whose new
// text is a function of the rendering of spans inside it -- a call's, say,
// of its arguments -- so that the rewrites inside those are applied too.
// And the parent of each node, which cc's tree does not keep.

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// nrw is a rewrite: src[a:z] becomes text(render), where render gives a
// span inside it with the rewrites inside that span applied.
type nrw struct {
	a, z int
	text func(render func(a, z int) string) string
}

// applyNested applies the rewrites to src: each span replaced, a span inside
// another rendered only through the outer one's text.  Spans that overlap
// without one holding the other are an error.
func applyNested(src []byte, rws []nrw) ([]byte, error) {
	sort.SliceStable(rws, func(i, j int) bool {
		if rws[i].a != rws[j].a {
			return rws[i].a < rws[j].a
		}
		return rws[i].z > rws[j].z
	})
	var render func(a, z int, rs []nrw) (string, error)
	render = func(a, z int, rs []nrw) (string, error) {
		var out []byte
		at := a
		for i := 0; i < len(rs); {
			r := rs[i]
			// the rewrites inside r
			j := i + 1
			for j < len(rs) && rs[j].a < r.z {
				if rs[j].z > r.z {
					return "", fmt.Errorf("rewrites overlap: [%d,%d) and [%d,%d)", r.a, r.z, rs[j].a, rs[j].z)
				}
				j++
			}
			inner := rs[i+1 : j]
			out = append(out, src[at:r.a]...)
			var ferr error
			txt := r.text(func(sa, sz int) string {
				var sub []nrw
				for _, x := range inner {
					if x.a >= sa && x.z <= sz {
						sub = append(sub, x)
					}
				}
				s, err := render(sa, sz, sub)
				if err != nil && ferr == nil {
					ferr = err
				}
				return s
			})
			if ferr != nil {
				return "", ferr
			}
			out = append(out, txt...)
			at = r.z
			i = j
		}
		out = append(out, src[at:z]...)
		return string(out), nil
	}
	s, err := render(0, len(src), rws)
	return []byte(s), err
}

// parents is the parent of each node under root.
func parents(root cc.Node) map[cc.Node]cc.Node {
	par := map[cc.Node]cc.Node{}
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		children(n, func(c cc.Node) {
			par[c] = n
			rec(c)
		})
	}
	rec(root)
	return par
}

// children calls f with each child node of n.
func children(n cc.Node, f func(cc.Node)) {
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if !t.Field(i).IsExported() {
			continue
		}
		fv := v.Field(i)
		if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface) && !fv.IsNil() {
			if c, ok := fv.Interface().(cc.Node); ok {
				f(c)
			}
		}
	}
}

// up is n's parent, past parentheses.
func up(par map[cc.Node]cc.Node, n cc.Node) cc.Node {
	for {
		p := par[n]
		switch x := p.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				n = p
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				n = p
				continue
			}
		}
		return p
	}
}

// identOf is the declarator e names, or nil.
func identOf(e cc.Node) *cc.Declarator {
	pe, ok := unparen(e).(*cc.PrimaryExpression)
	if !ok || pe.Case != cc.PrimaryExpressionIdent {
		return nil
	}
	d, _ := pe.ResolvedTo().(*cc.Declarator)
	return d
}

// isNullC says e is a null pointer constant: 0, or 0 cast to a pointer.
func isNullC(e cc.Node) bool {
	e = unparen(e)
	if c, ok := e.(*cc.CastExpression); ok && c.Case == cc.CastExpressionCast {
		return isNullC(c.CastExpression)
	}
	x, ok := e.(cc.ExpressionNode)
	if !ok {
		return false
	}
	switch v := x.Value().(type) {
	case cc.Int64Value:
		return v == 0
	case cc.UInt64Value:
		return v == 0
	}
	return false
}

// spanOf is n's span in src, to the end of the identifier it ends in: the
// front end measures a C23 nullptr three bytes short.
func spanOf(n cc.Node, src []byte) (int, int) {
	a, z := sweep.Span(n, file, src)
	for z > 0 && z < len(src) && idByte(src[z-1]) && idByte(src[z]) {
		z++
	}
	return a, z
}
