package togo

// cpp_refs.go is pointer parameters as C++ references where the C proves
// them never null (doc/CPP-IDIOMS.md, item 6): a parameter `T *p` whose
// function only ever dereferences it -- `*p`, `p->m`, or passes it on to
// another such parameter -- and every call of which passes `&x` (an
// object's address, never null) or such a parameter of its own.  It is `T
// &p`, used as `p` and `p.m`, and a call passes `x`.  A function used as a
// value, one of the host's and one the hand-written C++ calls keep their
// pointers.

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// cppCand is a pointer parameter that may be a reference.
type cppCand struct {
	fn string
	i  int
	d  *cc.Declarator
}

// refParams decides c.refs: by function, the indices of its reference
// parameters; and c.refDecl, the parameters' declarators.
func (c *cppgen) refParams() {
	keep := map[string]bool{}
	for _, n := range c.g.p.CppExports {
		keep[n] = true
	}
	// the candidates: pointer parameters to complete objects
	cands := map[*cc.Declarator]*cppCand{}
	argOK := map[string]map[int]bool{} // every call passes &x or a reference
	for name, fd := range c.defined {
		if keep[name] || c.g.a.addr[name] || c.runtimeBodied(name) {
			continue
		}
		for i, d := range paramDecls(fd) {
			if d == nil || d.AddressTaken() {
				continue
			}
			t := d.Type()
			if t == nil || t.Kind() != cc.Ptr {
				continue
			}
			e := elemOf(t)
			if e == nil || e.Kind() == cc.Void || e.Kind() == cc.Function || e.Kind() == cc.Ptr || e.IsIncomplete() {
				continue
			}
			cands[d] = &cppCand{name, i, d}
			if argOK[name] == nil {
				argOK[name] = map[int]bool{}
			}
			argOK[name][i] = true
		}
	}
	type passOn struct {
		from *cc.Declarator
		to   string // "" for a call through a pointer
		i    int
	}
	var passes []passOn
	out := map[*cc.Declarator]bool{} // used as a pointer
	parent := map[cc.Node]cc.Node{}
	var walk func(n, p cc.Node)
	walk = func(n, p cc.Node) {
		if n == nil {
			return
		}
		parent[n] = p
		switch x := n.(type) {
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionCall {
				to := ""
				var ft *cc.FunctionType
				if d := fnDesig(x.PostfixExpression); d != nil {
					to = d.Name()
					ft, _ = d.Type().(*cc.FunctionType)
				}
				i := 0
				for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
					a := strip(l.AssignmentExpression)
					if d := candName(a, cands); d != nil {
						passes = append(passes, passOn{d, to, i})
					} else if argOK[to] != nil && argOK[to][i] {
						// &x of the parameter's very type
						if !isAddrOf(a) || ft == nil || i >= len(ft.Parameters()) || c.needsCast(l.AssignmentExpression, ft.Parameters()[i].Type()) {
							argOK[to][i] = false
						}
					}
					i++
				}
			}
		case *cc.PrimaryExpression:
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok && cands[d] != nil && x.Case == cc.PrimaryExpressionIdent {
				if !derefUse(x, parent) {
					out[d] = true
				}
			}
		}
		walkChildrenFn(n, func(ch cc.Node) { walk(ch, n) })
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if c.mine(tu.ExternalDeclaration) {
			walk(tu.ExternalDeclaration, nil)
		}
	}
	ref := func(k *cppCand) bool {
		return !out[k.d] && argOK[k.fn] != nil && argOK[k.fn][k.i]
	}
	// the fixed point: a reference is passed on only to references, and a
	// parameter is passed only references and addresses
	for changed := true; changed; {
		changed = false
		for _, p := range passes {
			from := cands[p.from]
			ok := argOK[p.to] != nil && argOK[p.to][p.i]
			if ok {
				if to := paramDecls(c.defined[p.to])[p.i]; to == nil || out[to] {
					ok = false // the callee's is a pointer: so is this
				}
			}
			if !ok && !out[p.from] {
				out[p.from] = true // passed on as a pointer
				changed = true
			}
			if ok && !ref(from) && argOK[p.to][p.i] {
				argOK[p.to][p.i] = false // passed a pointer that may be null
				changed = true
			}
			if ok && argOK[p.to][p.i] && c.canon(elemOf(p.from.Type()), false) != c.canon(elemOf(paramDecls(c.defined[p.to])[p.i].Type()), false) {
				argOK[p.to][p.i] = false
				changed = true
			}
		}
	}
	c.refs = map[string]map[int]bool{}
	c.refDecl = map[*cc.Declarator]bool{}
	for _, k := range cands {
		if ref(k) {
			if c.refs[k.fn] == nil {
				c.refs[k.fn] = map[int]bool{}
			}
			c.refs[k.fn][k.i] = true
			c.refDecl[k.d] = true
			c.nRefs++
		}
	}
}

// candName is the candidate parameter n names, or nil.
func candName(n cc.ExpressionNode, cands map[*cc.Declarator]*cppCand) *cc.Declarator {
	if p, ok := strip(n).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if d, ok := p.ResolvedTo().(*cc.Declarator); ok && cands[d] != nil {
			return d
		}
	}
	return nil
}

// refName is the reference parameter n names, or nil.
func (c *cppgen) refName(n cc.ExpressionNode) *cc.Declarator {
	if p, ok := strip(n).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
		if d, ok := p.ResolvedTo().(*cc.Declarator); ok && c.refDecl[d] {
			return d
		}
	}
	return nil
}

// runtimeBodied reports whether a function's body is the runtime's.
func (c *cppgen) runtimeBodied(name string) bool {
	for _, rb := range c.g.p.RuntimeBodies {
		if rb.Name == name && rb.Cpp != nil {
			return true
		}
	}
	return false
}

// paramDecls are a function definition's parameters' declarators, by index.
func paramDecls(fd *cc.FunctionDefinition) []*cc.Declarator {
	var out []*cc.Declarator
	dd := fd.Declarator.DirectDeclarator
	for dd != nil && dd.Case != cc.DirectDeclaratorFuncParam {
		if dd.Case == cc.DirectDeclaratorDecl {
			dd = dd.Declarator.DirectDeclarator
			continue
		}
		dd = dd.DirectDeclarator
	}
	if dd == nil || dd.ParameterTypeList == nil {
		return nil
	}
	for l := dd.ParameterTypeList.ParameterList; l != nil; l = l.ParameterList {
		p := l.ParameterDeclaration
		if p.Case != cc.ParameterDeclarationDecl || p.Declarator == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, p.Declarator)
	}
	return out
}

func isAddrOf(n cc.ExpressionNode) bool {
	u, ok := n.(*cc.UnaryExpression)
	return ok && u.Case == cc.UnaryExpressionAddrof
}

// derefUse reports whether a parameter's name x is used as *x or x->m, or
// as a whole argument (which the calls judge).
func derefUse(x *cc.PrimaryExpression, parent map[cc.Node]cc.Node) bool {
	var n cc.Node = x
	for {
		p := parent[n]
		if e, ok := p.(cc.ExpressionNode); ok && pass(e) == pass(x) {
			n = p
			continue
		}
		break
	}
	switch p := parent[n].(type) {
	case *cc.UnaryExpression:
		return p.Case == cc.UnaryExpressionDeref
	case *cc.PostfixExpression:
		return p.Case == cc.PostfixExpressionPSelect && p.PostfixExpression == n
	case *cc.ArgumentExpressionList:
		return true
	}
	return false
}
