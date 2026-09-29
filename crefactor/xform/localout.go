package xform

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/sweep"
)

// LocalOut is the step that makes an out-parameter a value in and a value
// out.  A function's parameter `T *p` whose callee only reads and writes *p
// and tests p against null, and whose every caller passes `&x` of a local of
// its own that nothing else takes the address of, takes x's value and
// returns its new one:
//
//	void f(A a, T *p)     ->  T f(A a, T p)                    x = f(a, x)
//	R g(A a, T *p, U *q)  ->  g__out_T g(A a, T p, U q)        (g__o = g(a, x, y), x = g__o.p, y = g__o.q, g__o.r__)
//
// where g__out_T is a struct of the result and the parameters' values, so
// that C can return them all.  A target with no address of a variable --
// Java, Clojure -- boxes x in a one-element array wherever its address is
// taken; after this, only where the address is kept does.  Haskell and Go
// read the struct's members at once.
//
// It is exact because x is reachable only through the parameter: the
// callee cannot see it by another way, so when the value comes back is when
// C's stores through p would be seen -- phase 175's proof (MemberOut), for
// locals: a site is taken only when
//
//   - the callee is a function the core defines, not variadic, its
//     parameters named, its address never taken and the host naming it not;
//   - the parameter points at a scalar or a pointer, and the callee uses it
//     only as *p (not &*p) and in a test of whether it is null;
//   - every call passes &x there, x a local or a parameter of the caller, of
//     the pointee's type, once, and no other argument names x;
//   - every address of x taken anywhere is such an argument.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N
// parameters are taken.
func LocalOut(core Core) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "localout", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		out, n, fns, loDeadIn, err := localOut(p, core, text)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "  %s: %d out-parameters are values in and out, of %d functions; %d take no value in\n", p.Tag, n, fns, loDeadIn)
		if min, ok := f["--at-least"]; ok && n < min {
			return nil, p.Die("%d parameters taken, fewer than the %d asked for", n, min)
		}
		return out, nil
	}
}

type loSite struct {
	call *cc.PostfixExpression
	g    string
	k    int
	u    *cc.UnaryExpression // &x
	x    *cc.Declarator
	fn   *cc.FunctionDefinition
}

func localOut(p edit.Ph, core Core, text []byte) ([]byte, int, int, int, error) {
	end := len(text)
	if core != nil {
		if e := core(text); e >= 0 {
			end = e
		}
	}
	src := append([]byte(nil), text[:end]...)
	ast, err := translate(append([]byte(nil), src...))
	if err != nil {
		return nil, 0, 0, 0, p.Die("the core does not type-check: %v", err)
	}
	fns := map[string]*cc.FunctionDefinition{}
	var order []*cc.FunctionDefinition
	protos := map[string][]*cc.Declaration{}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Position().Filename != file {
			continue
		}
		switch ed.Case {
		case cc.ExternalDeclarationFuncDef:
			fd := ed.FunctionDefinition
			fns[fd.Declarator.Name()] = fd
			order = append(order, fd)
		case cc.ExternalDeclarationDecl:
			if d := ed.Declaration; d != nil && d.InitDeclaratorList != nil && d.InitDeclaratorList.InitDeclaratorList == nil {
				if dd := d.InitDeclaratorList.InitDeclarator.Declarator; dd != nil && dd.Type() != nil && dd.Type().Kind() == cc.Function {
					protos[dd.Name()] = append(protos[dd.Name()], d)
				}
			}
		}
	}
	valued := valuedFns(ast, fns)
	pars := map[*cc.FunctionDefinition]map[cc.Node]cc.Node{}
	for _, fd := range order {
		pars[fd] = parents(fd.CompoundStatement)
	}
	// the candidates: what each callee does with the parameter
	cand := map[string]map[int]bool{}
	for _, fd := range order {
		g := fd.Declarator.Name()
		ft, _ := fd.Declarator.Type().(*cc.FunctionType)
		if ft == nil || variadicOrUnnamed(fd) || valued[g] || mentions(text[end:], g) || len(paramDecls(fd.Declarator)) != len(params(fd)) {
			continue
		}
		for k, d := range params(fd) {
			if loParam(fd, d, pars[fd]) {
				if cand[g] == nil {
					cand[g] = map[int]bool{}
				}
				cand[g][k] = true
			}
		}
	}
	// the calls
	var sites []loSite
	for _, fd := range order {
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			call, ok := n.(*cc.PostfixExpression)
			if !ok || call.Case != cc.PostfixExpressionCall {
				return true
			}
			g := calleeName(call)
			if cand[g] == nil {
				return true
			}
			as := callArgs(call)
			seen := map[*cc.Declarator]bool{}
			for k := range cand[g] {
				if k >= len(as) {
					delete(cand[g], k)
					continue
				}
				u, x := loAddr(as[k])
				e := elemOfPtr(params(fns[g])[k].Type())
				if x == nil || seen[x] || !sameScalar(x.Type(), e) || namedElsewhere(as, k, x) || unsequenced(pars[fd], call, x) {
					delete(cand[g], k)
					continue
				}
				seen[x] = true
				sites = append(sites, loSite{call, g, k, u, x, fd})
			}
			return true
		})
	}
	// every address of a local taken must be such an argument; a call
	// nested in another's arguments is left to the next run
	for changed := true; changed; {
		changed = false
		good := map[*cc.UnaryExpression]bool{}
		for _, s := range sites {
			if cand[s.g][s.k] {
				good[s.u] = true
			}
		}
		escapes := map[*cc.Declarator]bool{}
		for _, fd := range order {
			sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
				if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
					if d := identOf(u.CastExpression); d != nil && !good[u] {
						escapes[d] = true
					}
				}
				return true
			})
		}
		for _, s := range sites {
			if cand[s.g][s.k] && escapes[s.x] {
				delete(cand[s.g], s.k)
				changed = true
			}
		}
	}
	outs := map[string][]int{}
	n := 0
	for g, ks := range cand {
		for k := range ks {
			outs[g] = append(outs[g], k)
			n++
		}
		if len(outs[g]) == 0 {
			delete(outs, g)
		}
		sort.Ints(outs[g])
	}
	if n == 0 {
		return text, 0, 0, 0, nil
	}
	// the parameters whose value in is dead: written before it is read on
	// every path, returned nowhere before
	deadIn := map[string]map[int]bool{}
	for g, ks := range outs {
		for _, k := range ks {
			if writesFirst(fns[g].CompoundStatement, params(fns[g])[k]) {
				if deadIn[g] == nil {
					deadIn[g] = map[int]bool{}
				}
				deadIn[g][k] = true
			}
		}
	}
	dead := 0
	for _, ks := range deadIn {
		dead += len(ks)
	}
	rws, err := loRewrites(src, fns, protos, pars, outs, sites, deadIn)
	if err != nil {
		return nil, 0, 0, 0, p.Die("%v", err)
	}
	out, err := applyNested(src, rws)
	if err != nil {
		return nil, 0, 0, 0, p.Die("%v", err)
	}
	return append(out, text[end:]...), n, len(outs), dead, nil
}

// valuedFns are the functions the core names but does not call: what a
// call through a pointer may run.
func valuedFns(ast *cc.AST, fns map[string]*cc.FunctionDefinition) map[string]bool {
	called := map[*cc.PrimaryExpression]bool{}
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		if x, ok := n.(*cc.PostfixExpression); ok && x.Case == cc.PostfixExpressionCall {
			if pe, ok := unparen(x.PostfixExpression).(*cc.PrimaryExpression); ok {
				called[pe] = true
			}
		}
		return true
	})
	valued := map[string]bool{}
	sweep.Walk(ast.TranslationUnit, func(n cc.Node) bool {
		pe, ok := n.(*cc.PrimaryExpression)
		if !ok || pe.Case != cc.PrimaryExpressionIdent || called[pe] {
			return true
		}
		if d, ok := pe.ResolvedTo().(*cc.Declarator); ok && fns[d.Name()] != nil && d.Type() != nil && d.Type().Kind() == cc.Function {
			valued[d.Name()] = true
		}
		return true
	})
	return valued
}

// elemOfPtr is what a pointer type points at.
func elemOfPtr(t cc.Type) cc.Type {
	if pt, ok := t.(*cc.PointerType); ok {
		return pt.Elem()
	}
	return nil
}

// loParam says fd uses its parameter d only as *d and in a test of whether
// it is null, and d points at a scalar or a pointer.
func loParam(fd *cc.FunctionDefinition, d *cc.Declarator, par map[cc.Node]cc.Node) bool {
	if d == nil || d.Type() == nil || d.Type().Kind() != cc.Ptr {
		return false
	}
	switch e := elemOfPtr(d.Type()); {
	case e == nil:
		return false
	case e.Kind() == cc.Struct || e.Kind() == cc.Union || e.Kind() == cc.Array || e.Kind() == cc.Function || e.Kind() == cc.Void:
		return false
	}
	ok, used := true, false
	sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
		pe, isPE := n.(*cc.PrimaryExpression)
		if !isPE || pe.Case != cc.PrimaryExpressionIdent || pe.ResolvedTo() != cc.Node(d) {
			return ok
		}
		used = true
		if loUse(par, pe) == "" {
			ok = false
		}
		return ok
	})
	return ok && used
}

// loUse is how the parameter at pe is used: "deref" (*p, not &*p), "ne"
// or "eq" (a test against null: p != NULL, p == NULL), "not" (!p), "true"
// (p where a truth value is asked), or "" for any other use.
func loUse(par map[cc.Node]cc.Node, pe *cc.PrimaryExpression) string {
	u := up(par, pe)
	switch x := u.(type) {
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionDeref:
			if a, ok := up(par, x).(*cc.UnaryExpression); ok && a.Case == cc.UnaryExpressionAddrof {
				return ""
			}
			return "deref"
		case cc.UnaryExpressionNot:
			return "not"
		}
	case *cc.EqualityExpression:
		if x.Case == cc.EqualityExpressionRel {
			return ""
		}
		other := cc.Node(x.RelationalExpression)
		if unparen(x.RelationalExpression) == cc.Node(pe) {
			other = x.EqualityExpression
		}
		if !isNullC(other) {
			return ""
		}
		if x.Case == cc.EqualityExpressionEq {
			return "eq"
		}
		return "ne"
	case *cc.LogicalAndExpression:
		if x.Case == cc.LogicalAndExpressionLAnd {
			return "true"
		}
	case *cc.LogicalOrExpression:
		if x.Case == cc.LogicalOrExpressionLOr {
			return "true"
		}
	case *cc.SelectionStatement:
		return "true"
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond && unparen(x.LogicalOrExpression) == cc.Node(pe) {
			return "true"
		}
	}
	return ""
}

// loAddr is e as &x of a local or a parameter x, when it is.
func loAddr(e cc.ExpressionNode) (*cc.UnaryExpression, *cc.Declarator) {
	u, ok := unparen(e).(*cc.UnaryExpression)
	if !ok || u.Case != cc.UnaryExpressionAddrof {
		return nil, nil
	}
	d := identOf(u.CastExpression)
	if d == nil || d.Type() == nil || d.StorageDuration() != cc.Automatic {
		return nil, nil
	}
	return u, d
}

// sameScalar says a and b are one scalar type, or both pointers.
func sameScalar(a, b cc.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Kind() == cc.Ptr && b.Kind() == cc.Ptr {
		return true
	}
	return a.Kind() == b.Kind() && a.Size() == b.Size() && a.Kind() != cc.Struct && a.Kind() != cc.Union && a.Kind() != cc.Array
}

// namedElsewhere says an argument of as other than the k-th names x.
func namedElsewhere(as []cc.ExpressionNode, k int, x *cc.Declarator) bool {
	for i, a := range as {
		if i == k {
			continue
		}
		found := false
		sweep.Walk(a, func(n cc.Node) bool {
			if pe, ok := n.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && pe.ResolvedTo() == cc.Node(x) {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// unsequenced says the full expression holding call names x where C does
// not order the read after the call: the value would move from before the
// call to after it.  A name on the right of && or || whose left holds the
// call, in an arm of ?: whose condition does, or in a later operand of a
// comma is after it.
func unsequenced(par map[cc.Node]cc.Node, call *cc.PostfixExpression, x *cc.Declarator) bool {
	// the full expression: up to a statement or a declaration
	full := cc.Node(call)
	for {
		p := par[full]
		if p == nil {
			break
		}
		if _, ok := p.(cc.ExpressionNode); !ok {
			if _, ok := p.(*cc.ArgumentExpressionList); !ok {
				break
			}
		}
		full = p
	}
	inside := func(n, root cc.Node) bool {
		for ; n != nil; n = par[n] {
			if n == root {
				return true
			}
		}
		return false
	}
	bad := false
	sweep.Walk(full, func(n cc.Node) bool {
		pe, ok := n.(*cc.PrimaryExpression)
		if !ok || pe.Case != cc.PrimaryExpressionIdent || pe.ResolvedTo() != cc.Node(x) || inside(pe, call) {
			return !bad
		}
		// the lowest node holding both
		lca := cc.Node(pe)
		for lca != nil && !inside(call, lca) {
			lca = par[lca]
		}
		after := false
		switch l := lca.(type) {
		case *cc.LogicalAndExpression:
			after = l.Case == cc.LogicalAndExpressionLAnd && inside(call, l.LogicalAndExpression) && inside(pe, l.InclusiveOrExpression)
		case *cc.LogicalOrExpression:
			after = l.Case == cc.LogicalOrExpressionLOr && inside(call, l.LogicalOrExpression) && inside(pe, l.LogicalAndExpression)
		case *cc.ConditionalExpression:
			after = l.Case == cc.ConditionalExpressionCond && inside(call, l.LogicalOrExpression) && !inside(pe, l.LogicalOrExpression)
		case *cc.ExpressionList:
			after = l.ExpressionList != nil && inside(call, l.AssignmentExpression) && inside(pe, l.ExpressionList)
		}
		if !after {
			bad = true
		}
		return !bad
	})
	return bad
}

// loRewrites are the rewrites: each callee's head, parameters, uses and
// returns, and each call.
func loRewrites(src []byte, fns map[string]*cc.FunctionDefinition, protos map[string][]*cc.Declaration,
	pars map[*cc.FunctionDefinition]map[cc.Node]cc.Node, outs map[string][]int, sites []loSite, deadIn map[string]map[int]bool) ([]nrw, error) {
	var rws []nrw
	lit := func(a, z int, s string) nrw {
		return nrw{a, z, func(func(int, int) string) string { return s }}
	}
	span := func(n cc.Node) (int, int) { return spanOf(n, src) }
	type shape struct {
		void   bool
		simple bool   // void, one: returns the value
		stype  string // the struct's name
		res    string // the result's type
		vals   []string
		tspell []string
	}
	shapes := map[string]*shape{}
	var gs []string
	for g := range outs {
		gs = append(gs, g)
	}
	sort.Strings(gs)
	for _, g := range gs {
		fd := fns[g]
		ft := fd.Declarator.Type().(*cc.FunctionType)
		sh := &shape{void: ft.Result().Kind() == cc.Void}
		sh.simple = sh.void && len(outs[g]) == 1
		ps := params(fd)
		pds := paramDecls(fd.Declarator)
		for _, k := range outs[g] {
			t, err := derefSpelling(src, pds[k])
			if err != nil {
				return nil, fmt.Errorf("%s: %v", g, err)
			}
			sh.tspell = append(sh.tspell, t)
			sh.vals = append(sh.vals, ps[k].Name())
		}
		if !sh.void {
			r, err := resultSpelling(src, fd)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", g, err)
			}
			sh.res = r
		}
		sh.stype = g + "__out_T"
		for mentions(src, sh.stype) {
			sh.stype += "_"
		}
		shapes[g] = sh
		// the struct, before the first declaration of g
		var def string
		if !sh.simple {
			var b strings.Builder
			b.WriteString("typedef struct {\n")
			if !sh.void {
				fmt.Fprintf(&b, "    %s;\n", withName(sh.res, "r__"))
			}
			for i, v := range sh.vals {
				fmt.Fprintf(&b, "    %s;\n", withName(sh.tspell[i], v))
			}
			fmt.Fprintf(&b, "} %s;\n\n", sh.stype)
			def = b.String()
		}
		newRes := sh.stype
		if sh.simple {
			newRes = sh.tspell[0]
		}
		// the heads: the definition's and the prototypes'
		type head struct {
			a, z  int
			specs string
		}
		var heads []head
		{
			sa, _ := span(fd.DeclarationSpecifiers)
			na := declName(fd.Declarator)
			heads = append(heads, head{sa, na, string(src[sa:na])})
		}
		for _, pd := range protos[g] {
			sa, _ := span(pd.DeclarationSpecifiers)
			na := declName(pd.InitDeclaratorList.InitDeclarator.Declarator)
			heads = append(heads, head{sa, na, string(src[sa:na])})
		}
		sort.Slice(heads, func(i, j int) bool { return heads[i].a < heads[j].a })
		for i, hd := range heads {
			storage := storageWords(hd.specs)
			txt := strings.TrimSpace(storage+" "+newRes) + "\n"
			if i == 0 && def != "" {
				// the struct before the first declaration: in its head's
				// text, which starts its line
				txt = def + txt
			}
			rws = append(rws, lit(hd.a, hd.z, txt))
		}
		// the parameters: `T *p` is `T p`, in the definition and the prototypes
		starOff := func(pd *cc.ParameterDeclaration) (nrw, error) {
			a, z := span(pd)
			sa, sz := span(pd.DeclarationSpecifiers)
			_ = sa
			rest := string(src[sz:z])
			i := strings.Index(rest, "*")
			if i < 0 {
				return nrw{}, fmt.Errorf("a parameter with no *: %s", src[a:z])
			}
			return lit(sz+i, sz+i+1, ""), nil
		}
		// a parameter whose value in is dead leaves the list: a local; the
		// list rewritten whole, what stays rendered with its own rewrites
		list := func(pds []*cc.ParameterDeclaration) nrw {
			a, _ := span(pds[0])
			_, z := span(pds[len(pds)-1])
			return nrw{a, z, func(render func(int, int) string) string {
				var kept []string
				for k, pd := range pds {
					if deadIn[g][k] {
						continue
					}
					pa, pz := span(pd)
					kept = append(kept, render(pa, pz))
				}
				if len(kept) == 0 {
					return "void"
				}
				return strings.Join(kept, ", ")
			}}
		}
		var locals []string
		for _, k := range outs[g] {
			if !deadIn[g][k] {
				continue
			}
			t, err := derefSpelling(src, pds[k])
			if err != nil {
				return nil, fmt.Errorf("%s: %v", g, err)
			}
			locals = append(locals, withName(t, ps[k].Name())+";")
		}
		dropped := len(locals) > 0
		if !sh.simple {
			// one struct for every return, declared once
			locals = append(locals, sh.stype+" out__;")
		}
		if dropped {
			rws = append(rws, list(pds))
			for _, pd := range protos[g] {
				rws = append(rws, list(paramDecls(pd.InitDeclaratorList.InitDeclarator.Declarator)))
			}
		}
		if len(locals) > 0 {
			ba, _ := span(fd.CompoundStatement)
			rws = append(rws, lit(ba+1, ba+1, "\n    "+strings.Join(locals, "\n    ")))
		}
		for _, k := range outs[g] {
			if deadIn[g][k] {
				continue
			}
			r, err := starOff(pds[k])
			if err != nil {
				return nil, fmt.Errorf("%s: %v", g, err)
			}
			rws = append(rws, r)
			for _, pd := range protos[g] {
				ppds := paramDecls(pd.InitDeclaratorList.InitDeclarator.Declarator)
				if k >= len(ppds) {
					return nil, fmt.Errorf("%s: a prototype of other parameters", g)
				}
				r, err := starOff(ppds[k])
				if err != nil {
					return nil, fmt.Errorf("%s: %v", g, err)
				}
				rws = append(rws, r)
			}
		}
		// the uses
		par := pars[fd]
		isOut := map[cc.Node]int{}
		for i, k := range outs[g] {
			isOut[ps[k]] = i
		}
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			pe, ok := n.(*cc.PrimaryExpression)
			if !ok || pe.Case != cc.PrimaryExpressionIdent {
				return true
			}
			d, _ := pe.ResolvedTo().(*cc.Declarator)
			if _, ok := isOut[d]; !ok {
				return true
			}
			u := up(par, pe)
			switch loUse(par, pe) {
			case "deref":
				a, z := span(u)
				rws = append(rws, lit(a, z, d.Name()))
			case "ne":
				a, z := span(u)
				rws = append(rws, lit(a, z, "1"))
			case "eq", "not":
				a, z := span(u)
				rws = append(rws, lit(a, z, "0"))
			case "true":
				a, z := span(pe)
				rws = append(rws, lit(a, z, "1"))
			}
			return true
		})
		// the returns
		outVal := func(e string) string {
			if sh.simple {
				return "return " + sh.vals[0] + ";"
			}
			var b strings.Builder
			b.WriteString("{")
			if e != "" {
				fmt.Fprintf(&b, " out__.r__ = %s;", e)
			}
			for _, v := range sh.vals {
				fmt.Fprintf(&b, " out__.%s = %s;", v, v)
			}
			b.WriteString(" return out__; }")
			return b.String()
		}
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			js, ok := n.(*cc.JumpStatement)
			if !ok || js.Case != cc.JumpStatementReturn {
				return true
			}
			a, z := span(js)
			if js.ExpressionList == nil {
				rws = append(rws, lit(a, z, outVal("")))
				return true
			}
			ea, ez := span(js.ExpressionList)
			rws = append(rws, nrw{a, z, func(render func(int, int) string) string {
				return outVal("(" + render(ea, ez) + ")")
			}})
			return true
		})
		if sh.void && !endsInReturn(fd) {
			_, z := span(fd.CompoundStatement)
			rws = append(rws, lit(z-1, z-1, "    "+outVal("")+"\n"))
		}
	}
	// the calls, and each caller's locals for the structs
	decls := map[*cc.FunctionDefinition]map[string]bool{}
	byCall := map[*cc.PostfixExpression][]loSite{}
	var calls []*cc.PostfixExpression
	for _, s := range sites {
		if !isOutK(outs[s.g], s.k) {
			continue
		}
		if byCall[s.call] == nil {
			calls = append(calls, s.call)
		}
		byCall[s.call] = append(byCall[s.call], s)
	}
	// a call's out-value is written back only to a local something reads
	// after the call (laterRead)
	good := map[*cc.UnaryExpression]loSite{}
	for _, s := range sites {
		if isOutK(outs[s.g], s.k) {
			good[s.u] = s
		}
	}
	readAfter := func(s loSite) bool {
		return laterRead(pars[s.fn], s.fn, s.x, s.call, func(u *cc.UnaryExpression) bool {
			gs, ok := good[u]
			return ok && deadIn[gs.g][gs.k]
		}) && !killedAfter(pars[s.fn], s.x, s.call, func(u *cc.UnaryExpression) bool {
			gs, ok := good[u]
			return ok && deadIn[gs.g][gs.k]
		})
	}
	for _, call := range calls {
		ss := byCall[call]
		g := ss[0].g
		readBack := map[*cc.Declarator]bool{}
		for _, s := range ss {
			readBack[s.x] = readAfter(s)
		}
		sh := shapes[g]
		fd := ss[0].fn
		addr := map[int]*cc.Declarator{}
		for _, s := range ss {
			addr[s.k] = s.x
		}
		as := callArgs(call)
		ca, cz := span(call)
		fa, fz := span(call.PostfixExpression)
		var spans [][2]int
		for _, a := range as {
			a0, a1 := span(a)
			spans = append(spans, [2]int{a0, a1})
		}
		o := g + "__o"
		anyBack := false
		for _, s := range ss {
			anyBack = anyBack || readBack[s.x]
		}
		// the struct is a local of the caller where the call reads it: a
		// value written back, or the result
		if !sh.simple && (anyBack || !sh.void) {
			if decls[fd] == nil {
				decls[fd] = map[string]bool{}
			}
			decls[fd][o] = true
		}
		rws = append(rws, nrw{ca, cz, func(render func(int, int) string) string {
			var args []string
			for k := range as {
				if x, ok := addr[k]; ok {
					if !deadIn[g][k] {
						args = append(args, x.Name())
					}
					continue
				}
				args = append(args, render(spans[k][0], spans[k][1]))
			}
			callTxt := render(fa, fz) + "(" + strings.Join(args, ", ") + ")"
			if sh.simple {
				if !readBack[addr[outs[g][0]]] {
					return callTxt // nothing reads the value
				}
				return "(" + addr[outs[g][0]].Name() + " = " + callTxt + ")"
			}
			if !anyBack && sh.void {
				return callTxt // nothing reads what it returns
			}
			parts := []string{o + " = " + callTxt}
			for i, k := range outs[g] {
				if !readBack[addr[k]] {
					continue // nothing reads the value
				}
				parts = append(parts, fmt.Sprintf("%s = %s.%s", addr[k].Name(), o, sh.vals[i]))
			}
			if sh.void {
				parts = append(parts, "(void)0")
			} else {
				parts = append(parts, o+".r__")
			}
			return "(" + strings.Join(parts, ", ") + ")"
		}})
	}
	for fd, os := range decls {
		a, _ := span(fd.CompoundStatement)
		var names []string
		for o := range os {
			names = append(names, o)
		}
		sort.Strings(names)
		var b strings.Builder
		for _, o := range names {
			fmt.Fprintf(&b, "\n    %s %s;", shapes[strings.TrimSuffix(o, "__o")].stype, o)
		}
		rws = append(rws, lit(a+1, a+1, b.String()))
	}
	return rws, nil
}

func isOutK(ks []int, k int) bool {
	for _, j := range ks {
		if j == k {
			return true
		}
	}
	return false
}

// storageWords are the storage class and inline words of a specifiers text.
func storageWords(specs string) string {
	var out []string
	for _, w := range strings.Fields(specs) {
		switch w {
		case "static", "extern", "inline":
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// withName is a declaration of name of type t, a spelling that may end in
// *s: `char_u *` and `p` is `char_u *p`.
func withName(t, name string) string {
	if strings.HasSuffix(t, "*") {
		return t + name
	}
	return t + " " + name
}

// derefSpelling is the spelling of what parameter pd points at: its text
// with its name and one * taken off (`char_u **arg` -> `char_u *`).
func derefSpelling(src []byte, pd *cc.ParameterDeclaration) (string, error) {
	a, z := spanOf(pd, src)
	_, sz := spanOf(pd.DeclarationSpecifiers, src)
	specs := strings.TrimSpace(string(src[a:sz]))
	rest := strings.TrimSpace(string(src[sz:z]))
	if pd.Declarator != nil {
		rest = strings.TrimSpace(strings.TrimSuffix(rest, pd.Declarator.Name()))
	}
	if !strings.HasPrefix(rest, "*") || strings.ContainsAny(rest, "()[]") {
		return "", fmt.Errorf("a parameter spelled %q", src[a:z])
	}
	rest = strings.TrimSpace(rest[1:])
	if strings.Contains(rest, "const") && !strings.HasPrefix(rest, "*") {
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "const"))
	}
	if rest == "" {
		return specs, nil
	}
	return specs + " " + rest, nil
}

// resultSpelling is fd's result type: its specifiers without the storage
// class, and the *s before its name.
func resultSpelling(src []byte, fd *cc.FunctionDefinition) (string, error) {
	sa, sz := spanOf(fd.DeclarationSpecifiers, src)
	var ws []string
	for _, w := range strings.Fields(string(src[sa:sz])) {
		switch w {
		case "static", "extern", "inline":
		default:
			ws = append(ws, w)
		}
	}
	na := declName(fd.Declarator)
	stars := strings.TrimSpace(string(src[sz:na]))
	if strings.Trim(stars, "* ") != "" {
		return "", fmt.Errorf("a result spelled %q", src[sa:na])
	}
	res := strings.Join(ws, " ")
	if s := strings.ReplaceAll(stars, " ", ""); s != "" {
		res += " " + s
	}
	return res, nil
}

// endsInReturn says fd's body's last statement is a return.
func endsInReturn(fd *cc.FunctionDefinition) bool {
	var last cc.Node
	for l := fd.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
		last = l.BlockItem
	}
	if bi, ok := last.(*cc.BlockItem); ok && bi.Statement != nil {
		if js, ok := bi.Statement.JumpStatement, true; ok && js != nil && js.Case == cc.JumpStatementReturn {
			return true
		}
	}
	return false
}

// writesFirst says no path from body's start reads the value p points at
// before a store to *p -- a return, which sends the value back, counting as
// a read, and so does the end of the body -- so the value going in is dead.
func writesFirst(body *cc.CompoundStatement, p *cc.Declarator) bool {
	w := &wf{p: p}
	u := w.block(body, true)
	return !w.fail && !u
}

// wf follows whether the value may still be the one that came in (u): a
// read then fails, a store clears it; a statement after a jump is reached
// with nothing (false), a case label with what the switch had.
type wf struct {
	p       *cc.Declarator
	fail    bool
	switch_ []bool
}

func (w *wf) block(cs *cc.CompoundStatement, u bool) bool {
	for l := cs.BlockItemList; l != nil && !w.fail; l = l.BlockItemList {
		bi := l.BlockItem
		if bi.Statement == nil {
			if u && namesDecl(bi, w.p) {
				w.fail = true
			}
			continue
		}
		u = w.stmt(bi.Statement, u)
	}
	return u
}

func (w *wf) expr(e cc.Node, u bool) bool {
	if e == nil {
		return u
	}
	if st := w.storeIn(e); st != nil {
		// its one name of p is a store C always evaluates: the value is
		// stored, from what does not read it
		if u && namesDecl(st.AssignmentExpression, w.p) {
			w.fail = true
		}
		return false
	}
	if u && namesDecl(e, w.p) {
		w.fail = true
	}
	return u
}

// storeIn is the assignment *p = R in e, when it is e's one name of p and
// no operator C evaluates lazily (&&, ||, ?:) holds it.
func (w *wf) storeIn(e cc.Node) *cc.AssignmentExpression {
	var names []*cc.PrimaryExpression
	sweep.Walk(e, func(n cc.Node) bool {
		if pe, ok := n.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && pe.ResolvedTo() == cc.Node(w.p) {
			names = append(names, pe)
		}
		return true
	})
	if len(names) != 1 {
		return nil
	}
	par := parents(e)
	d, ok := up(par, names[0]).(*cc.UnaryExpression)
	if !ok || d.Case != cc.UnaryExpressionDeref {
		return nil
	}
	as, ok := up(par, d).(*cc.AssignmentExpression)
	if !ok || as.Case != cc.AssignmentExpressionAssign || unparen(as.UnaryExpression) != cc.Node(d) {
		return nil
	}
	for n := par[as]; n != nil; n = par[n] {
		switch x := n.(type) {
		case *cc.LogicalAndExpression:
			if x.Case == cc.LogicalAndExpressionLAnd {
				return nil
			}
		case *cc.LogicalOrExpression:
			if x.Case == cc.LogicalOrExpressionLOr {
				return nil
			}
		case *cc.ConditionalExpression:
			if x.Case == cc.ConditionalExpressionCond {
				return nil
			}
		}
	}
	return as
}

func (w *wf) stmt(st *cc.Statement, u bool) bool {
	if w.fail {
		return u
	}
	switch st.Case {
	case cc.StatementCompound:
		return w.block(st.CompoundStatement, u)
	case cc.StatementExpr:
		if st.ExpressionStatement.ExpressionList == nil {
			return u
		}
		return w.expr(st.ExpressionStatement.ExpressionList, u)
	case cc.StatementSelection:
		sel := st.SelectionStatement
		u = w.expr(sel.ExpressionList, u)
		switch sel.Case {
		case cc.SelectionStatementIf:
			return w.stmt(sel.Statement, u) || u
		case cc.SelectionStatementIfElse:
			a := w.stmt(sel.Statement, u)
			b := w.stmt(sel.Statement2, u)
			return a || b
		default: // a switch: every case is entered with what it had
			w.switch_ = append(w.switch_, u)
			end := w.stmt(sel.Statement, u)
			w.switch_ = w.switch_[:len(w.switch_)-1]
			return end || u
		}
	case cc.StatementIteration:
		it := st.IterationStatement
		// the body runs from the state it is entered with, any number of
		// times: what it stores does not hold after it
		switch it.Case {
		case cc.IterationStatementFor:
			u = w.expr(it.ExpressionList, u)
			u = w.expr(it.ExpressionList2, u)
			w.stmt(it.Statement, u)
			w.expr(it.ExpressionList3, u)
		case cc.IterationStatementForDecl:
			if u && namesDecl(it.Declaration, w.p) {
				w.fail = true
			}
			u = w.expr(it.ExpressionList, u)
			w.stmt(it.Statement, u)
			w.expr(it.ExpressionList2, u)
		default:
			u = w.expr(it.ExpressionList, u)
			w.stmt(it.Statement, u)
		}
		return u
	case cc.StatementJump:
		js := st.JumpStatement
		switch js.Case {
		case cc.JumpStatementReturn:
			// the value goes back with the result
			if u {
				w.fail = true
			}
			return false
		case cc.JumpStatementGoto, cc.JumpStatementGotoExpr:
			w.fail = true
		}
		return false // a break or a continue: what follows is not reached this way
	case cc.StatementLabeled:
		ls := st.LabeledStatement
		if ls.Case == cc.LabeledStatementLabel || len(w.switch_) == 0 {
			w.fail = true
			return u
		}
		return w.stmt(ls.Statement, u || w.switch_[len(w.switch_)-1])
	}
	if u && namesDecl(st, w.p) {
		w.fail = true
	}
	return u
}

// namesDecl says n names d.
func namesDecl(n cc.Node, d *cc.Declarator) bool {
	found := false
	sweep.Walk(n, func(m cc.Node) bool {
		if pe, ok := m.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && pe.ResolvedTo() == cc.Node(d) {
			found = true
		}
		return !found
	})
	return found
}

// laterRead says fd reads x after call: a read -- a name of x not only
// stored to, and not an out-argument whose value in is dead (skip) --
// later in the text, or anywhere in a loop holding the call; with a goto's
// label in fd, any read at all.
func laterRead(par map[cc.Node]cc.Node, fd *cc.FunctionDefinition, x *cc.Declarator, call *cc.PostfixExpression, skip func(*cc.UnaryExpression) bool) bool {
	callEnd := call.Position().Offset // a read inside the call is excluded below
	var loops []cc.Node
	for n := par[call]; n != nil; n = par[n] {
		if _, ok := n.(*cc.IterationStatement); ok {
			loops = append(loops, n)
		}
	}
	labels := false
	sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
		if ls, ok := n.(*cc.LabeledStatement); ok && ls.Case == cc.LabeledStatementLabel {
			labels = true
		}
		return !labels
	})
	inside := func(n, root cc.Node) bool {
		for ; n != nil; n = par[n] {
			if n == root {
				return true
			}
		}
		return false
	}
	found := false
	sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
		pe, ok := n.(*cc.PrimaryExpression)
		if !ok || pe.Case != cc.PrimaryExpressionIdent || pe.ResolvedTo() != cc.Node(x) || inside(pe, call) {
			return !found
		}
		switch u := up(par, pe).(type) {
		case *cc.UnaryExpression:
			if u.Case == cc.UnaryExpressionAddrof && skip(u) {
				return !found
			}
		case *cc.AssignmentExpression:
			if u.Case == cc.AssignmentExpressionAssign && unparen(u.UnaryExpression) == cc.Node(pe) {
				return !found // only stored to
			}
		}
		if labels || pe.Position().Offset > callEnd {
			found = true
		}
		for _, l := range loops {
			if inside(pe, l) {
				found = true
			}
		}
		return !found
	})
	return found
}

// killedAfter says what runs after call stores to x before anything reads
// it: the value the call would write back is dead.  From the statement
// holding the call, the statements after it in its block, then after the
// block's if, and so on out; a statement that names x otherwise, a break, a
// continue, a goto, a label, a loop or a switch around the way out ends the
// search with no.  The function's end is a yes: x dies there.
func killedAfter(par map[cc.Node]cc.Node, x *cc.Declarator, call *cc.PostfixExpression, deadArg func(*cc.UnaryExpression) bool) bool {
	// the statement holding the call, and x named nowhere else in it
	var st cc.Node = call
	for {
		p := par[st]
		if p == nil {
			return false
		}
		st = p
		if _, ok := st.(*cc.Statement); ok {
			break
		}
	}
	names := 0
	sweep.Walk(st, func(n cc.Node) bool {
		if pe, ok := n.(*cc.PrimaryExpression); ok && pe.Case == cc.PrimaryExpressionIdent && pe.ResolvedTo() == cc.Node(x) {
			names++
		}
		return true
	})
	if names != 1 {
		return false
	}
	jumps := func(n cc.Node) bool {
		found := false
		sweep.Walk(n, func(m cc.Node) bool {
			switch y := m.(type) {
			case *cc.JumpStatement:
				if y.Case != cc.JumpStatementReturn {
					found = true
				}
			case *cc.LabeledStatement:
				found = true
			case *cc.IterationStatement, *cc.SelectionStatement:
				// a break inside a loop or a switch of its own is that one's
				_ = y
			}
			return !found
		})
		return found
	}
	cur := st
	for {
		switch p := par[cur].(type) {
		case *cc.BlockItem:
			list := par[p].(*cc.BlockItemList)
			for l := list.BlockItemList; l != nil; l = l.BlockItemList {
				bi := l.BlockItem
				if bi.Statement == nil {
					if namesDecl(bi, x) {
						return false
					}
					continue
				}
				if stored(bi.Statement, x) || storedByCall(par, bi.Statement, x, deadArg) {
					return true
				}
				if namesDecl(bi.Statement, x) || jumps(bi.Statement) {
					return false
				}
			}
			// the block's end: on out of it
			var cs cc.Node = list
			for {
				cs = par[cs]
				if _, ok := cs.(*cc.CompoundStatement); ok || cs == nil {
					break
				}
			}
			if cs == nil {
				return false
			}
			switch q := par[cs].(type) {
			case *cc.FunctionDefinition:
				return true
			case *cc.Statement:
				cur = q
			default:
				return false
			}
		case *cc.SelectionStatement:
			if p.Case == cc.SelectionStatementSwitch {
				return false
			}
			cur = par[p] // the if's statement: on after it
		case *cc.LabeledStatement:
			if p.Case == cc.LabeledStatementLabel {
				return false // a goto's: reached from elsewhere too
			}
			cur = par[p] // a case's statement: on after the labeled one
		default:
			return false
		}
	}
}

// stored says statement st is x = R;, R not naming x.
func stored(st *cc.Statement, x *cc.Declarator) bool {
	if st.Case != cc.StatementExpr || st.ExpressionStatement.ExpressionList == nil {
		return false
	}
	as, ok := unparen(st.ExpressionStatement.ExpressionList).(*cc.AssignmentExpression)
	return ok && as.Case == cc.AssignmentExpressionAssign && identOf(as.UnaryExpression) == x && !namesDecl(as.AssignmentExpression, x)
}

// storedByCall says every name of x in st is an out-argument whose value in
// is dead (deadArg) that C always evaluates: the call stores x, reading
// nothing of it.
func storedByCall(par map[cc.Node]cc.Node, st *cc.Statement, x *cc.Declarator, deadArg func(*cc.UnaryExpression) bool) bool {
	n, ok := 0, true
	sweep.Walk(st, func(m cc.Node) bool {
		pe, isPE := m.(*cc.PrimaryExpression)
		if !isPE || pe.Case != cc.PrimaryExpressionIdent || pe.ResolvedTo() != cc.Node(x) {
			return ok
		}
		n++
		u, isU := up(par, pe).(*cc.UnaryExpression)
		if !isU || u.Case != cc.UnaryExpressionAddrof || !deadArg(u) || lazyWithin(par, u, st) {
			ok = false
		}
		return ok
	})
	return ok && n > 0 && st.Case != cc.StatementIteration && st.Case != cc.StatementLabeled
}

// lazyWithin says C may not evaluate n when it evaluates root: n is in the
// right operand of && or ||, or an arm of ?:, between them -- or in a
// statement root runs on a condition (an if's branch).
func lazyWithin(par map[cc.Node]cc.Node, n, root cc.Node) bool {
	inside := func(a, b cc.Node) bool {
		for ; a != nil; a = par[a] {
			if a == b {
				return true
			}
		}
		return false
	}
	for c := n; c != nil && c != root; c = par[c] {
		switch x := par[c].(type) {
		case *cc.LogicalAndExpression:
			if x.Case == cc.LogicalAndExpressionLAnd && inside(n, x.InclusiveOrExpression) {
				return true
			}
		case *cc.LogicalOrExpression:
			if x.Case == cc.LogicalOrExpressionLOr && inside(n, x.LogicalAndExpression) {
				return true
			}
		case *cc.ConditionalExpression:
			if x.Case == cc.ConditionalExpressionCond && !inside(n, x.LogicalOrExpression) {
				return true
			}
		case *cc.SelectionStatement:
			if !inside(n, x.ExpressionList) {
				return true
			}
		case *cc.IterationStatement:
			return true
		}
	}
	return false
}
