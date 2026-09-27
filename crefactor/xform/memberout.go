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

// MemberOut is the step that takes the address of a struct's scalar or
// pointer member out of the calls that pass it: `f(a, &s->m)` becomes
// `f__m(a, s)`, and a function written once per callee and member does what
// the call did through a local:
//
//	static R f__m(A a, S *s__) { typeof(s__->m) m__ = s__->m; R r__ = f(a, &m__); s__->m = m__; return r__; }
//
// A target with no address of a variable -- Java, Clojure -- holds a member
// whose address is taken anywhere in a one-element array, and every read of
// it pays; after this, only where the address is kept does.
//
// The copy is the call's exactly when nothing the call runs can reach the
// member but through the pointer, and the pointer does not outlive the call.
// So a site is taken only when
//
//   - the callee is a function the core defines;
//   - no function it can reach names a member of that name, of any struct
//     (`.m`, `->m`) -- where a call through a pointer reaches every function
//     whose address the core takes, and a call to the host every core
//     function the host's code names;
//   - every address of that member in the core is such a site: one kept
//     elsewhere boxes the member whatever is done here;
//   - the callee's parameter is only dereferenced, indexed, compared or
//     tested, or handed on as an argument to a function whose own parameter
//     is so (the pointer is kept nowhere);
//   - the expression the member is reached through does nothing (no call,
//     no store), so reading it once in the call's place is reading it where
//     the call did;
//   - the callee is not variadic, and each of its parameters has a name.
//
// It works on the core (core says where it ends): the only part a target
// translates.  Its one argument is a floor: `--at-least N` refuses when
// fewer than N sites are taken.
func MemberOut(core Core) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := edit.Ph{Tag: "memberout", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		out, taken, wrappers, held, err := memberOut(p, core, text)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "  %s: %d arguments pass a member's address no more, through %d functions\n", p.Tag, taken, len(wrappers))
		var hs []string
		for why, n := range held {
			hs = append(hs, fmt.Sprintf("%d %s", n, why))
		}
		sort.Strings(hs)
		for _, h := range hs {
			fmt.Fprintf(w, "  %s: held: %s\n", p.Tag, h)
		}
		if min, ok := f["--at-least"]; ok && taken < min {
			return nil, p.Die("%d sites taken, fewer than the %d asked for", taken, min)
		}
		return out, nil
	}
}

// translate type-checks text as one translation unit.
func translate(text []byte) (*cc.AST, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	return cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: file, Value: text},
	})
}

// a function of the core, as MemberOut sees it
type moFn struct {
	fd       *cc.FunctionDefinition
	members  map[string]bool // the member names it names
	calls    map[string]bool // what it calls by name
	indirect bool            // it calls through a pointer: any function whose address is taken
	host     bool            // it calls what the core does not define: the host, and what the host calls
}

// a site: a call passing &E.m or &E->m as its k-th argument
type moSite struct {
	call   *cc.PostfixExpression
	callee string
	fn     *cc.FunctionDefinition // the function holding it
	args   []moArg
}

type moArg struct {
	k      int
	arg    cc.ExpressionNode // &E.m
	base   cc.ExpressionNode // E
	member string
	arrow  bool // E->m: E is the struct's pointer
	st     cc.Type
}

func memberOut(p edit.Ph, core Core, text []byte) ([]byte, int, map[string]bool, map[string]int, error) {
	held := map[string]int{}
	end := len(text)
	if core != nil {
		if e := core(text); e >= 0 {
			end = e
		}
	}
	// copies: the parser appends to the source it is handed, which would
	// write over what follows the core in text
	src := append([]byte(nil), text[:end]...)
	ast, err := translate(append([]byte(nil), src...))
	if err != nil {
		return nil, 0, nil, nil, p.Die("the core does not type-check: %v", err)
	}
	fns := map[string]*moFn{}
	var order []*cc.FunctionDefinition
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef || ed.Position().Filename != file {
			continue
		}
		fd := ed.FunctionDefinition
		fns[fd.Declarator.Name()] = &moFn{fd: fd, members: map[string]bool{}, calls: map[string]bool{}}
		order = append(order, fd)
	}
	// the functions whose address is taken -- named but not called -- are
	// what a call through a pointer may run
	valued := map[string]bool{}
	for _, f := range fns {
		sweep.Walk(f.fd.CompoundStatement, func(n cc.Node) bool {
			x, ok := n.(*cc.PostfixExpression)
			if !ok {
				return true
			}
			switch x.Case {
			case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
				f.members[memberKey(x)] = true
			case cc.PostfixExpressionCall:
				name := calleeName(x)
				switch {
				case name == "":
					f.indirect = true
				case fns[name] == nil:
					f.host = true
				default:
					f.calls[name] = true
				}
			}
			return true
		})
	}
	called := map[*cc.PrimaryExpression]bool{}
	for _, f := range fns {
		sweep.Walk(f.fd.CompoundStatement, func(n cc.Node) bool {
			if x, ok := n.(*cc.PostfixExpression); ok && x.Case == cc.PostfixExpressionCall {
				if pe, ok := unparen(x.PostfixExpression).(*cc.PrimaryExpression); ok {
					called[pe] = true
				}
			}
			return true
		})
	}
	// and in the file-scope objects' initialisers: every name of a function
	// the core writes but does not call
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
	// the host's code, after the core: every core function it names
	hostCalls := map[string]bool{}
	for name := range fns {
		if mentions(text[end:], name) {
			hostCalls[name] = true
		}
	}
	// what each function can reach: its members named and whether any of it
	// is opaque
	type reach struct {
		members map[string]bool
	}
	reached := map[string]*reach{}
	reachOf := func(name string) *reach {
		if r := reached[name]; r != nil {
			return r
		}
		r := &reach{members: map[string]bool{}}
		seen := map[string]bool{name: true}
		stack := []string{name}
		push := func(c string) {
			if !seen[c] {
				seen[c] = true
				stack = append(stack, c)
			}
		}
		for len(stack) > 0 {
			g := fns[stack[len(stack)-1]]
			stack = stack[:len(stack)-1]
			for m := range g.members {
				r.members[m] = true
			}
			for c := range g.calls {
				push(c)
			}
			if g.indirect {
				for c := range valued {
					push(c)
				}
			}
			if g.host {
				for c := range hostCalls {
					push(c)
				}
			}
		}
		reached[name] = r
		return r
	}
	// whether a function's k-th parameter is kept nowhere
	memo := map[string]bool{}
	var paramKept func(name string, k int) bool
	paramKept = func(name string, k int) bool {
		key := fmt.Sprintf("%s/%d", name, k)
		if v, ok := memo[key]; ok {
			return v
		}
		memo[key] = false // recursion: assumed not kept until shown
		f := fns[name]
		ps := params(f.fd)
		if k >= len(ps) || ps[k] == nil {
			memo[key] = true
			return true
		}
		d := ps[k]
		total, fine := 0, 0
		isD := func(e cc.Node) bool {
			pe, ok := unparen(e).(*cc.PrimaryExpression)
			return ok && pe.Case == cc.PrimaryExpressionIdent && pe.ResolvedTo() == cc.Node(d)
		}
		sweep.Walk(f.fd.CompoundStatement, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.PrimaryExpression:
				if x.Case == cc.PrimaryExpressionIdent && x.ResolvedTo() == cc.Node(d) {
					total++
				}
			case *cc.UnaryExpression:
				if (x.Case == cc.UnaryExpressionDeref || x.Case == cc.UnaryExpressionNot) && isD(x.CastExpression) {
					fine++
				}
			case *cc.PostfixExpression:
				switch x.Case {
				case cc.PostfixExpressionIndex, cc.PostfixExpressionPSelect:
					if isD(x.PostfixExpression) {
						fine++
					}
				case cc.PostfixExpressionCall:
					callee := calleeName(x)
					for i, a := range callArgs(x) {
						if isD(a) && callee != "" && fns[callee] != nil && !paramKept(callee, i) {
							fine++
						}
					}
				}
			case *cc.EqualityExpression:
				if x.Case != cc.EqualityExpressionRel {
					if isD(x.EqualityExpression) {
						fine++
					}
					if isD(x.RelationalExpression) {
						fine++
					}
				}
			case *cc.SelectionStatement:
				if x.ExpressionList != nil && isD(x.ExpressionList) {
					fine++
				}
			case *cc.LogicalAndExpression:
				if x.Case == cc.LogicalAndExpressionLAnd {
					if isD(x.LogicalAndExpression) {
						fine++
					}
					if isD(x.InclusiveOrExpression) {
						fine++
					}
				}
			case *cc.LogicalOrExpression:
				if x.Case == cc.LogicalOrExpressionLOr {
					if isD(x.LogicalOrExpression) {
						fine++
					}
					if isD(x.LogicalAndExpression) {
						fine++
					}
				}
			}
			return true
		})
		kept := total != fine
		memo[key] = kept
		return kept
	}

	// the local structs whose address goes nowhere but into a call's
	// argument &s.m: no other frame -- this function's own, recursing,
	// included -- can reach one, so what a callee names does not matter
	localStruct := map[*cc.Declarator]bool{}
	for _, fd := range order {
		local := map[*cc.Declarator]bool{}
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			if d, ok := n.(*cc.Declarator); ok && d.StorageDuration() == cc.Automatic && d.Type() != nil && (d.Type().Kind() == cc.Struct || d.Type().Kind() == cc.Union) {
				local[d] = true
			}
			return true
		})
		escapes := map[*cc.Declarator]bool{}
		atSite := map[*cc.UnaryExpression]bool{}
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			x, ok := n.(*cc.PostfixExpression)
			if !ok || x.Case != cc.PostfixExpressionCall {
				return true
			}
			for _, a := range callArgs(x) {
				if u, ok := unparen(a).(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
					if sel, ok := unparen(u.CastExpression).(*cc.PostfixExpression); ok && sel.Case == cc.PostfixExpressionSelect {
						if t := sel.Type(); t != nil && t.Kind() != cc.Array && t.Kind() != cc.Struct && t.Kind() != cc.Union {
							atSite[u] = true
						}
					}
				}
			}
			return true
		})
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			u, ok := n.(*cc.UnaryExpression)
			if !ok || u.Case != cc.UnaryExpressionAddrof || atSite[u] {
				return true
			}
			// any other address of the local, or of a part of it
			var e cc.Node = unparen(u.CastExpression)
			for {
				switch x := e.(type) {
				case *cc.PostfixExpression:
					if x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionIndex {
						e = unparen(x.PostfixExpression)
						continue
					}
				case *cc.PrimaryExpression:
					if d, ok := x.ResolvedTo().(*cc.Declarator); ok && local[d] {
						escapes[d] = true
					}
				}
				break
			}
			return true
		})
		// an array member of the local decays to a pointer into it without
		// an & -- held too
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			sel, ok := n.(*cc.PostfixExpression)
			if !ok || sel.Case != cc.PostfixExpressionSelect || sel.Type() == nil || sel.Type().Kind() != cc.Array {
				return true
			}
			if pe, ok := unparen(sel.PostfixExpression).(*cc.PrimaryExpression); ok {
				if d, ok := pe.ResolvedTo().(*cc.Declarator); ok && local[d] {
					escapes[d] = true
				}
			}
			return true
		})
		for d := range local {
			if !escapes[d] {
				localStruct[d] = true
			}
		}
	}

	// the sites
	var sites []*moSite
	for _, fd := range order {
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			x, ok := n.(*cc.PostfixExpression)
			if !ok || x.Case != cc.PostfixExpressionCall {
				return true
			}
			callee := calleeName(x)
			var margs []moArg
			for k, a := range callArgs(x) {
				u, ok := unparen(a).(*cc.UnaryExpression)
				if !ok || u.Case != cc.UnaryExpressionAddrof {
					continue
				}
				sel, ok := unparen(u.CastExpression).(*cc.PostfixExpression)
				if !ok || sel.Case != cc.PostfixExpressionSelect && sel.Case != cc.PostfixExpressionPSelect {
					continue
				}
				t := sel.Type()
				if t == nil || t.Kind() == cc.Array || t.Kind() == cc.Struct || t.Kind() == cc.Union {
					continue // a struct or an array member is an object already
				}
				why := ""
				switch {
				case callee == "" || fns[callee] == nil:
					why = "a callee the core does not define"
				case !localStruct[baseIdent(sel)] && reachOf(callee).members[memberKey(sel)]:
					why = "a callee that can name the member"
				case paramKept(callee, k):
					why = "a callee that keeps the pointer"
				case effects(sel.PostfixExpression):
					why = "a member reached through what does something"
				case variadicOrUnnamed(fns[callee].fd):
					why = "a variadic callee, or an unnamed parameter"
				}
				if why != "" {
					held[why]++
					continue
				}
				st := sel.PostfixExpression.Type()
				if sel.Case == cc.PostfixExpressionPSelect {
					st = st.(*cc.PointerType).Elem()
				}
				margs = append(margs, moArg{k: k, arg: a, base: sel.PostfixExpression, member: sel.Token2.SrcStr(),
					arrow: sel.Case == cc.PostfixExpressionPSelect, st: st})
			}
			if len(margs) > 0 {
				sites = append(sites, &moSite{call: x, callee: callee, fn: fd, args: margs})
			}
			return true
		})
	}

	// a member is taken only when every address of it in the core is: a
	// member whose address is kept elsewhere stays boxed whatever is done here
	all := map[string]int{}
	for _, fd := range order {
		sweep.Walk(fd.CompoundStatement, func(n cc.Node) bool {
			u, ok := n.(*cc.UnaryExpression)
			if !ok || u.Case != cc.UnaryExpressionAddrof {
				return true
			}
			sel, ok := unparen(u.CastExpression).(*cc.PostfixExpression)
			if !ok || sel.Case != cc.PostfixExpressionSelect && sel.Case != cc.PostfixExpressionPSelect {
				return true
			}
			st := sel.PostfixExpression.Type()
			if sel.Case == cc.PostfixExpressionPSelect {
				pt, ok := st.(*cc.PointerType)
				if !ok {
					return true
				}
				st = pt.Elem()
			}
			all[structName(st)+"."+sel.Token2.SrcStr()]++
			return true
		})
	}
	taken := map[string]int{}
	for _, s := range sites {
		for _, a := range s.args {
			taken[structName(a.st)+"."+a.member]++
		}
	}
	var kept []*moSite
	for _, s := range sites {
		var args []moArg
		for _, a := range s.args {
			if k := structName(a.st) + "." + a.member; taken[k] == all[k] {
				args = append(args, a)
			} else {
				held["a member whose address is kept elsewhere too"]++
			}
		}
		if len(args) > 0 {
			s.args = args
			kept = append(kept, s)
		}
	}
	sites = kept
	ncalls := 0
	for _, s := range sites {
		ncalls += len(s.args)
	}

	// the functions: one per callee and members, before the first function
	// that calls it
	type rewrite struct {
		a, z int
		with string
	}
	var rw []rewrite
	wrappers := map[string]bool{}
	defs := map[*cc.FunctionDefinition][]string{}
	names := map[string]string{} // key -> name
	for _, s := range sites {
		var parts []string
		for _, a := range s.args {
			parts = append(parts, fmt.Sprintf("%d:%s:%s", a.k, a.member, structName(a.st)))
		}
		key := s.callee + "|" + strings.Join(parts, ",")
		name, ok := names[key]
		if !ok {
			var ms []string
			for _, a := range s.args {
				ms = append(ms, a.member)
			}
			name = s.callee + "__" + strings.Join(ms, "_")
			for i := 2; wrappers[name] || fns[name] != nil; i++ {
				name = fmt.Sprintf("%s__%s_%d", s.callee, strings.Join(ms, "_"), i)
			}
			names[key] = name
			wrappers[name] = true
			def, err := wrapper(src, fns[s.callee].fd, name, s.args)
			if err != nil {
				return nil, 0, nil, nil, p.Die("%s: %v", s.callee, err)
			}
			defs[s.fn] = append(defs[s.fn], def)
		}
		fa, fz := sweep.Span(s.call.PostfixExpression, file, src)
		rw = append(rw, rewrite{fa, fz, name})
		for _, a := range s.args {
			aa, az := sweep.Span(a.arg, file, src)
			ba, bz := sweep.Span(a.base, file, src)
			with := string(src[ba:bz])
			if !a.arrow {
				with = "&" + with // E.m's E is a postfix expression: & binds looser
			}
			rw = append(rw, rewrite{aa, az, with})
		}
	}
	for fd, ds := range defs {
		a := defStart(fd, src)
		// the definition starts at its line's start
		for a > 0 && src[a-1] != '\n' {
			a--
		}
		rw = append(rw, rewrite{a, a, strings.Join(ds, "")})
	}
	sort.SliceStable(rw, func(i, j int) bool { return rw[i].a > rw[j].a || rw[i].a == rw[j].a && rw[i].z > rw[j].z })
	out := append([]byte{}, src...)
	for _, x := range rw {
		out = append(append(append([]byte{}, out[:x.a]...), x.with...), out[x.z:]...)
	}
	out = append(out, text[end:]...)
	return out, ncalls, wrappers, held, nil
}

// wrapper is the function named name that calls fd with the members of
// args passed through locals: its parameters fd's, each of args' the
// struct's pointer instead.
func wrapper(src []byte, fd *cc.FunctionDefinition, name string, args []moArg) (string, error) {
	ps := params(fd)
	pds := paramDecls(fd.Declarator)
	if len(ps) != len(pds) {
		return "", fmt.Errorf("parameters")
	}
	byK := map[int]moArg{}
	for _, a := range args {
		byK[a.k] = a
	}
	var decl, callArgs, pre, post []string
	for k, pd := range pds {
		if a, ok := byK[k]; ok {
			s := fmt.Sprintf("s%d__", k)
			decl = append(decl, structName(a.st)+" *"+s)
			l := fmt.Sprintf("%s%d__", a.member, k)
			pre = append(pre, fmt.Sprintf("    typeof(%s->%s) %s = %s->%s;\n", s, a.member, l, s, a.member))
			post = append(post, fmt.Sprintf("    %s->%s = %s;\n", s, a.member, l))
			callArgs = append(callArgs, "&"+l)
			continue
		}
		a, z := sweep.Span(pd, file, src)
		decl = append(decl, string(src[a:z]))
		callArgs = append(callArgs, ps[k].Name())
	}
	// the result's spelling: the definition's text before its name
	fa := defStart(fd, src)
	na := fd.Declarator.Position().Offset
	if nt := declName(fd.Declarator); nt >= 0 {
		na = nt
	}
	head := strings.TrimSpace(string(src[fa:na]))
	ft := fd.Declarator.Type().(*cc.FunctionType)
	call := fmt.Sprintf("%s(%s)", fd.Declarator.Name(), strings.Join(callArgs, ", "))
	var b strings.Builder
	fmt.Fprintf(&b, "    %s\n%s(%s)\n{\n", head, name, strings.Join(decl, ", "))
	b.WriteString(strings.Join(pre, ""))
	if ft.Result().Kind() == cc.Void {
		fmt.Fprintf(&b, "    %s;\n", call)
		b.WriteString(strings.Join(post, ""))
	} else {
		res := head
		for _, w := range []string{"static ", "inline "} {
			res = strings.TrimSpace(strings.ReplaceAll(res, w, ""))
		}
		fmt.Fprintf(&b, "    %s r__ = %s;\n", res, call)
		b.WriteString(strings.Join(post, ""))
		b.WriteString("    return r__;\n")
	}
	b.WriteString("}\n\n")
	return b.String(), nil
}

// defStart is where a function definition's text starts: the line before
// its name's, where the canonical print writes its specifiers (`    static
// char_u *`).  A span of the definition does not reach them, and one of its
// specifiers reaches what the parser makes up (__func__'s declaration).
func defStart(fd *cc.FunctionDefinition, src []byte) int {
	a := declName(fd.Declarator)
	if a < 0 {
		a, _ = sweep.Span(fd.Declarator, file, src)
	}
	for a > 0 && src[a-1] != '\n' {
		a--
	}
	prev := a - 1
	for prev > 0 && src[prev-1] != '\n' {
		prev--
	}
	line := strings.TrimSpace(string(src[prev:a]))
	if prev >= 0 && line != "" && !strings.HasSuffix(line, ";") && !strings.HasSuffix(line, "}") && !strings.HasSuffix(line, "{") {
		return prev
	}
	return a
}

// memberKey is a member named by x, E.m or E->m: its struct's type and its
// name, so that two structs' members of one name are two.
func memberKey(x *cc.PostfixExpression) string {
	t := x.PostfixExpression.Type()
	if x.Case == cc.PostfixExpressionPSelect {
		pt, ok := t.(*cc.PointerType)
		if !ok {
			return "?." + x.Token2.SrcStr()
		}
		t = pt.Elem()
	}
	if t == nil {
		return "?." + x.Token2.SrcStr()
	}
	return structName(t) + "." + x.Token2.SrcStr()
}

// baseIdent is the local declarator E.m's E names, when E is one.
func baseIdent(sel *cc.PostfixExpression) *cc.Declarator {
	if sel.Case != cc.PostfixExpressionSelect {
		return nil
	}
	pe, ok := unparen(sel.PostfixExpression).(*cc.PrimaryExpression)
	if !ok || pe.Case != cc.PrimaryExpressionIdent {
		return nil
	}
	d, _ := pe.ResolvedTo().(*cc.Declarator)
	return d
}

// declName is the offset of a declarator's name token.
func declName(d *cc.Declarator) int {
	if d.DirectDeclarator == nil {
		return -1
	}
	dd := d.DirectDeclarator
	for dd.DirectDeclarator != nil {
		dd = dd.DirectDeclarator
	}
	if dd.Case == cc.DirectDeclaratorIdent {
		return dd.Token.Position().Offset
	}
	return -1
}

// structName is how a struct type is spelled: its typedef's name, or
// `struct tag`.
func structName(t cc.Type) string {
	if td := t.Typedef(); td != nil {
		return td.Name()
	}
	switch x := t.(type) {
	case *cc.StructType:
		tag := x.Tag()
		return "struct " + tag.SrcStr()
	case *cc.UnionType:
		tag := x.Tag()
		return "union " + tag.SrcStr()
	}
	return t.String()
}

func calleeName(x *cc.PostfixExpression) string {
	pe, ok := unparen(x.PostfixExpression).(*cc.PrimaryExpression)
	if !ok || pe.Case != cc.PrimaryExpressionIdent {
		return ""
	}
	if d, ok := pe.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
		return d.Name()
	}
	return ""
}

func callArgs(x *cc.PostfixExpression) []cc.ExpressionNode {
	var out []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		out = append(out, l.AssignmentExpression)
	}
	return out
}

// params are a function definition's parameter declarators, nil for an
// unnamed one.
func params(fd *cc.FunctionDefinition) []*cc.Declarator {
	var out []*cc.Declarator
	for _, pd := range paramDecls(fd.Declarator) {
		out = append(out, pd.Declarator)
	}
	return out
}


// mentions says text names name as an identifier.
func mentions(text []byte, name string) bool {
	for i := 0; ; {
		j := strings.Index(string(text[i:]), name)
		if j < 0 {
			return false
		}
		a, z := i+j, i+j+len(name)
		if (a == 0 || !idByte(text[a-1])) && (z >= len(text) || !idByte(text[z])) {
			return true
		}
		i = z
	}
}

func idByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func variadicOrUnnamed(fd *cc.FunctionDefinition) bool {
	if ft, ok := fd.Declarator.Type().(*cc.FunctionType); !ok || ft.IsVariadic() {
		return true
	}
	for _, pd := range paramDecls(fd.Declarator) {
		if pd.Declarator == nil || pd.Declarator.Name() == "" {
			return true
		}
	}
	return false
}

// effects says an expression calls or stores.
func effects(e cc.Node) bool {
	found := false
	sweep.Walk(e, func(n cc.Node) bool {
		switch x := n.(type) {
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionCall || x.Case == cc.PostfixExpressionInc || x.Case == cc.PostfixExpressionDec {
				found = true
			}
		case *cc.UnaryExpression:
			if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
				found = true
			}
		case *cc.AssignmentExpression:
			if x.Case != cc.AssignmentExpressionCond {
				found = true
			}
		}
		return !found
	})
	return found
}

// unparen is e without its parentheses.
func unparen(e cc.Node) cc.Node {
	for {
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionExpr {
				e = x.ExpressionList
				continue
			}
		case *cc.ExpressionList:
			if x.ExpressionList == nil {
				e = x.AssignmentExpression
				continue
			}
		}
		return e
	}
}
