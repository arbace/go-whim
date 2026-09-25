package togo

// clj_fn.go writes one C function, lowered (lower.go), as a Clojure
// function of the editor and the C's parameters.
//
// Each block's steps are one let: an assignment to a local rebinds it, and
// a step done for what it does is bound to _.  A block's terminator ends
// the let: an if, a case, a value.  The blocks are then nested:
//
//   - STRUCTURED, when they nest: a block reached by one edge is written
//     where that edge is; a join whose region between the branch and it
//     changes at most one live variable is (let [x (if ...)] join); a loop
//     is a loop whose bindings are the variables it changes that are live
//     at its head, recur its back edges, the code after it in its exit;
//   - a STATE MACHINE where they do not (a goto, a switch's fall-through,
//     a join with more than one variable changed, a break from a loop that
//     other code follows): (loop [st 0, x x, ...] (case st 0 ... 1 ...)),
//     each block that is not written where its one edge is a state, each
//     jump to one a recur with its number and the live variables.
//
// An address-taken local is a one-element Java array, and a struct or
// array local an object made at the function's start: neither is ever
// rebound, so neither is carried by a recur.

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// cbind is one binding of a let: name form, name _ for a step done for what
// it does.
type cbind struct{ name, form string }

// bindText is binds as a let's binding vector's contents, joined by sep.
func bindText(bs []cbind, sep string) string {
	var parts []string
	for _, b := range bs {
		parts = append(parts, b.name+" "+b.form)
	}
	return strings.Join(parts, sep)
}

// cvar is how a variable is held.
type cvar struct {
	jt    string // its Java type; "" while a temporary's is not yet known
	boxed bool   // a one-element Java array: its address is taken
	fixed bool   // a struct or array object made at the start, never rebound
}

// cfn is one function being written.
type cfn struct {
	c            *cgen
	lf           *lfn
	name         string
	vars         map[*lvar]*cvar
	ret          string // the Java result type
	pre          []cbind
	ntmp         int
	nvarsCarried int
	// what the blocks became
	steps map[*lblock][]cbind
	terms map[*lblock]cterm
}

// cterm is a block's terminator, printed.
type cterm struct {
	test  string // tIf: the test; tSwitch: the value; tRet: the value
	swap  bool   // tIf: the arms go the other way
	cases [][]int64
}

func (f *cfn) no(n cc.Node, format string, args ...any) {
	where := ""
	if n != nil {
		where = fmt.Sprintf(" at %d", n.Position().Line)
	}
	panic(unsupported{fmt.Sprintf(format, args...) + where})
}

// jt is the Java backend's type of t, or the function refused.
func (f *cfn) jt(t cc.Type, key string, where ...string) string {
	s, why := f.c.j.jt(t, key)
	if why != "" {
		if len(where) > 0 {
			f.no(nil, "%s: %s", why, strings.Join(where, " "))
		}
		f.no(nil, "%s", why)
	}
	return s
}

// capture runs fn and returns the steps it wrote, which it does not keep.
func (f *cfn) capture(fn func()) []cbind {
	old := f.pre
	f.pre = nil
	fn()
	r := f.pre
	f.pre = old
	return r
}

// effectOf writes s, evaluated for what it does.
func (f *cfn) effectOf(s string) {
	if s != "" {
		f.pre = append(f.pre, cbind{"_", s})
	}
}

// tmpName is a name the function's C has not: for what the printer binds.
func (f *cfn) tmpName(base string) string {
	f.ntmp++
	return fmt.Sprintf("%s__%d", base, f.ntmp)
}

// function writes one function definition.
func (c *cgen) function(fd *cc.FunctionDefinition) (src string, why string, mode string) {
	d := fd.Declarator
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(unsupported)
			if !ok {
				why = fmt.Sprint("panic: ", r)
				return
			}
			src, why = "", u.why
		}
	}()
	lf := lowerFunction(fd, c.g.a, c.g.p, c.localName)
	f := &cfn{c: c, lf: lf, name: d.Name(), vars: map[*lvar]*cvar{}}
	ft := lf.ft
	f.ret = f.jt(ft.Result(), "ret:"+d.Name(), "the result")
	for _, v := range lf.vars {
		cvr := &cvar{}
		switch {
		case v.temp:
			if v.boolean {
				cvr.jt = "boolean"
			} else if k, ok := scalarKind(v.c); ok {
				cvr.jt = k.java()
			} else if isAggr(v.c) || v.c.Kind() == cc.Array {
				cvr.jt = f.jt(v.c, "")
			}
		case v.param:
			cvr.jt = f.jt(v.c, v.key, "a parameter", v.name)
			cvr.boxed = v.decl != nil && c.j.isBoxed(v.decl)
		default:
			cvr.jt = f.jt(v.c, v.key, "a local", v.name)
			cvr.boxed = c.j.isBoxed(v.decl)
			cvr.fixed = !cvr.boxed && (isAggr(v.c) || v.c.Kind() == cc.Array)
		}
		f.vars[v] = cvr
	}
	if rb := c.runtimeBody(d.Name()); rb != nil {
		return f.header(rb(f.ret)), "", "runtime"
	}
	// twice: the first learns the temporaries' types, in reverse postorder
	f.printBlocks()
	f.ntmp = 0
	f.printBlocks()
	body, mode, pre := f.structure()
	if os.Getenv("WHIM_CLJ_STATS") != "" {
		fmt.Fprintf(os.Stderr, "stat %s %d %s %d %d\n", d.Name(), len(body), mode, strings.Count(body, "(recur "), strings.Count(body, "(recur ")*0+f.nvarsCarried)
	}
	return pre + f.header(body), "", mode
}

// printBlocks prints every block's steps and terminator.
func (f *cfn) printBlocks() {
	f.steps = map[*lblock][]cbind{}
	f.terms = map[*lblock]cterm{}
	for _, b := range f.lf.blocks {
		f.pre = nil
		for _, s := range b.steps {
			f.step(s)
		}
		f.steps[b] = f.pre
		f.pre = nil
		f.terms[b] = f.term(b.term)
		f.pre = nil
	}
}

// step writes one step into f.pre.
func (f *cfn) step(s lstep) {
	switch s.op {
	case opSet:
		v := f.vars[s.dst]
		r := f.lexprTo(s.e, v.jt)
		if r.t == "void" {
			f.no(s.at, "a value of no type")
		}
		if s.dst.boolean {
			f.pre = append(f.pre, cbind{s.dst.name, f.truth(r)})
			return
		}
		if v.jt == "" && !r.null {
			t := r.t
			v.jt = t
		}
		if v.jt == "" {
			f.pre = append(f.pre, cbind{s.dst.name, "nil"})
			return
		}
		f.pre = append(f.pre, f.assign(f.varLval(s.dst), r, s.at))
	case opAssign:
		lv := f.lval(s.lhs)
		r := f.lexprTo(s.e, lv.t)
		f.pre = append(f.pre, f.assign(lv, r, s.at))
	case opAssignOp:
		lv := f.lval(s.lhs)
		r := f.lexprTo(s.e, "")
		f.pre = append(f.pre, f.assignOp(lv, s.aop, r, s.at))
	case opIncDec:
		f.pre = append(f.pre, f.incdec(f.lval(s.lhs), s.inc, s.at))
	case opEval:
		r := f.lexprTo(s.e, "")
		f.effectOf(r.s)
	case opInit:
		f.initStep(s)
	}
}

// initStep gives a declared object its initializer's value where C declares
// it: an object local is zeroed in place and its list stored; a compound
// literal's temporary is a new object.
func (f *cfn) initStep(s lstep) {
	v := s.dst
	cvr := f.vars[v]
	t := v.c
	if !isAggr(t) && t.Kind() != cc.Array {
		// a scalar with braces: its one element
		n := 0
		for l := s.in.InitializerList; l != nil; l = l.InitializerList {
			n++
			if n > 1 || l.Designation != nil || l.Initializer.Case != cc.InitializerExpr {
				f.no(s.in, "a braced initializer for a %s", t)
			}
			lv := f.varLval(v)
			f.pre = append(f.pre, f.assign(lv, f.exprTo(l.Initializer.AssignmentExpression, lv.t), s.at))
		}
		if n == 0 {
			f.pre = append(f.pre, f.assign(f.varLval(v), cv{s: "0", t: "int", konst: true, kind: &jInt}, s.at))
		}
		return
	}
	if cvr.jt == "" {
		cvr.jt = f.jt(t, v.key)
	}
	obj := cv{s: v.name, t: cvr.jt, c: t, typed: true}
	if cvr.fixed {
		for _, z := range f.zeroInPlace(obj, t) {
			f.effectOf(z)
		}
	} else {
		f.pre = append(f.pre, cbind{v.name, f.newOf(t, cvr.jt)})
	}
	f.initInto(f.objPlace(obj, t, v.key), s.in)
}

// cplace is somewhere an initializer's value goes: an object that is there
// (a struct or an array), or a slot a scalar or pointer is stored in.
type cplace struct {
	obj cv                    // a struct or array object
	set func(v string) string // stores a scalar or pointer value
	t   cc.Type
	jt  string
	key string
}

// objPlace is a struct or array object as a place.
func (f *cfn) objPlace(obj cv, t cc.Type, key string) cplace {
	return cplace{obj: obj, t: t, jt: obj.t, key: key}
}

// memberPlace is member fl of struct object obj.
func (f *cfn) memberPlace(obj cv, fl *cc.Field) cplace {
	name := f.c.memberName(fl)
	key := fieldKey(fl)
	jt := f.jt(fl.Type(), key, "a member", fl.Name())
	o := tagged(obj)
	switch {
	case f.c.j.boxedField[key]:
		k, sc := scalarKind(fl.Type())
		return cplace{t: fl.Type(), jt: jt, key: key, set: func(v string) string {
			if sc {
				v = storeK(v, k)
			}
			return "(aset ^" + cljHint(jt+"[]") + " (.-" + name + " " + o + ") 0 " + v + ")"
		}}
	case isAggr(fl.Type()) || fl.Type().Kind() == cc.Array:
		return cplace{obj: cv{s: "(.-" + name + " " + o + ")", t: jt, c: fl.Type()}, t: fl.Type(), jt: jt, key: key}
	}
	k, sc := scalarKind(fl.Type())
	return cplace{t: fl.Type(), jt: jt, key: key, set: func(v string) string {
		if sc && k.boolean {
			v = "(boolean " + v + ")"
		}
		return "(.set_" + name + " " + o + " " + v + ")"
	}}
}

// elemPlace is element i of array object arr.
func (f *cfn) elemPlace(arr cv, i int64, et cc.Type, key string) cplace {
	jt := elemJ(arr.t)
	a := tagged(arr)
	idx := fmt.Sprint(i)
	if isAggr(et) || et.Kind() == cc.Array {
		return cplace{obj: cv{s: "(aget " + a + " " + idx + ")", t: jt, c: et}, t: et, jt: jt, key: key}
	}
	k, sc := scalarKind(et)
	return cplace{t: et, jt: jt, key: key, set: func(v string) string {
		if sc {
			v = storeK(v, k)
		}
		return "(aset " + a + " " + idx + " " + v + ")"
	}}
}

// initInto writes the steps that give place p, which is zero, the value of
// initializer in.
func (f *cfn) initInto(p cplace, in *cc.Initializer) {
	t := p.t
	if in.Case == cc.InitializerExpr {
		e := in.AssignmentExpression
		switch t.Kind() {
		case cc.Array:
			sv, ok := unparenE(e).Value().(cc.StringValue)
			if !ok || !isCharType(t.(*cc.ArrayType).Elem()) {
				f.no(in, "an array initialized from an expression")
			}
			f.effectOf("(Rt/init " + tagged(p.obj) + " " + cljQuote(strings.TrimSuffix(string(sv), "\x00")) + ")")
		case cc.Struct, cc.Union:
			v := f.exprTo(e, p.jt)
			f.effectOf("(.set " + tagged(p.obj) + " " + v.s + ")")
		default:
			v := f.exprTo(e, p.jt)
			f.effectOf(p.set(f.conv(v, p.jt, t)))
		}
		return
	}
	elided := func(item *cc.Initializer, it cc.Type) {
		if item.Case != cc.InitializerExpr || (!isAggr(it) && it.Kind() != cc.Array) {
			return
		}
		e := item.AssignmentExpression
		if isAggr(it) && e.Type() != nil && isAggr(e.Type()) {
			return
		}
		if _, ok := unparenE(e).Value().(cc.StringValue); ok && it.Kind() == cc.Array {
			return
		}
		f.no(in, "an initializer that elides its braces")
	}
	switch x := t.(type) {
	case *cc.StructType, *cc.UnionType:
		fs := members(x)
		i := 0
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			if d := f.designator(l); d != nil {
				if d.Case != cc.DesignatorField && d.Case != cc.DesignatorField2 {
					f.no(in, "an index designator in a struct")
				}
				name := d.Token2.SrcStr()
				if d.Case == cc.DesignatorField2 {
					name = d.Token.SrcStr()
				}
				i = -1
				for k, fl := range fs {
					if fl != nil && fl.Name() == name {
						i = k
					}
				}
				if i < 0 {
					f.no(in, "no field %s", name)
				}
			}
			if i >= len(fs) || fs[i] == nil {
				f.no(in, "more initializers than fields")
			}
			fl := fs[i]
			i++
			if zeroInit(l.Initializer) {
				continue
			}
			if fl.IsBitfield() {
				f.no(in, "a bitfield")
			}
			elided(l.Initializer, fl.Type())
			f.initInto(f.memberPlace(p.obj, fl), l.Initializer)
		}
	case *cc.ArrayType:
		i := int64(0)
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			if d := f.designator(l); d != nil {
				if d.Case != cc.DesignatorIndex {
					f.no(in, "a field designator in an array")
				}
				v, ok := d.ConstantExpression.Value().(cc.Int64Value)
				if !ok {
					f.no(in, "an index designator of no value")
				}
				i = int64(v)
			}
			k := i
			i++
			if zeroInit(l.Initializer) {
				continue
			}
			elided(l.Initializer, x.Elem())
			f.initInto(f.elemPlace(p.obj, k, x.Elem(), "elem:"+p.key), l.Initializer)
		}
	default:
		n := 0
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			n++
			if n > 1 || l.Designation != nil {
				f.no(in, "a braced initializer for a %s", t)
			}
			f.initInto(p, l.Initializer)
		}
	}
}

// designator is an element's one designator, or nil.
func (f *cfn) designator(l *cc.InitializerList) *cc.Designator {
	if l.Designation == nil {
		return nil
	}
	dl := l.Designation.DesignatorList
	if dl.DesignatorList != nil {
		f.no(l, "a designator of more than one step")
	}
	return dl.Designator
}

// freshInit is a new object of C type t with initializer in, as one
// expression: a compound literal.
func (f *cfn) freshInit(t cc.Type, jt string, in *cc.Initializer, key string) cv {
	name := f.tmpName("lit")
	obj := cv{s: name, t: jt, c: t, typed: true}
	pre := f.capture(func() { f.initInto(f.objPlace(obj, t, key), in) })
	h := cljHint(jt)
	binds := append([]cbind{{"^" + h + " " + name, f.newOf(t, jt)}}, pre...)
	return cv{s: "(let [" + bindText(binds, " ") + "] " + name + ")", t: jt, c: t}
}

// newOf is a new zeroed object of C type t, Java type jt: a struct, an
// array, or a scalar's or pointer's zero.
func (f *cfn) newOf(t cc.Type, jt string) string {
	switch t.Kind() {
	case cc.Struct, cc.Union:
		return "(new-" + jt + ")"
	case cc.Array:
		at := t.(*cc.ArrayType)
		n := at.Len()
		et := at.Elem()
		switch {
		case et.Kind() == cc.Array:
			ej := elemJ(jt)
			k := f.tmpName("k")
			a := f.tmpName("a")
			return fmt.Sprintf("(let [%s (object-array %d)] (dotimes [%s %d] (aset %s %s %s)) %s)", a, n, k, n, a, k, f.newOf(et, ej), a)
		case isAggr(et):
			return fmt.Sprintf("(array-%s %d)", elemJ(jt), n)
		}
		switch elemJ(jt) {
		case "byte":
			return fmt.Sprintf("(byte-array %d)", n)
		case "short":
			return fmt.Sprintf("(short-array %d)", n)
		case "int":
			return fmt.Sprintf("(int-array %d)", n)
		case "long":
			return fmt.Sprintf("(long-array %d)", n)
		case "boolean":
			return fmt.Sprintf("(boolean-array %d)", n)
		}
		return fmt.Sprintf("(object-array %d)", n)
	case cc.Bool:
		return "false"
	}
	if _, ok := scalarKind(t); ok {
		return "0"
	}
	return "nil"
}

// zeroInPlace are the forms that zero object obj of C type t where it is.
func (f *cfn) zeroInPlace(obj cv, t cc.Type) []string {
	switch t.Kind() {
	case cc.Struct, cc.Union:
		return []string{"(.zero " + tagged(obj) + ")"}
	case cc.Array:
		at := t.(*cc.ArrayType)
		et := at.Elem()
		a := tagged(obj)
		switch {
		case isAggr(et) || et.Kind() == cc.Array:
			k := f.tmpName("k")
			inner := f.zeroInPlace(cv{s: "(aget " + a + " " + k + ")", t: elemJ(obj.t), c: et}, et)
			return []string{fmt.Sprintf("(dotimes [%s %d] %s)", k, at.Len(), strings.Join(inner, " "))}
		case et.Kind() == cc.Bool:
			return []string{"(java.util.Arrays/fill " + a + " false)"}
		}
		if k, ok := scalarKind(et); ok {
			return []string{"(java.util.Arrays/fill " + a + " " + storeK("0", k) + ")"}
		}
		return []string{"(java.util.Arrays/fill " + a + " nil)"}
	}
	return nil
}

// term prints a terminator.
func (f *cfn) term(t lterm) cterm {
	switch t.kind {
	case tIf:
		v := f.lexprTo(t.cond, "")
		s, swap := f.cond(v)
		return cterm{test: wrapPre(f.takePre(), s), swap: swap}
	case tSwitch:
		v := f.lexprTo(t.cond, "")
		k, ok := scalarKind(t.cond.typeOf())
		if !ok {
			f.no(t.at, "a switch on no integer")
		}
		return cterm{test: wrapPre(f.takePre(), f.convK(v, promote(k))), cases: t.cases}
	case tRet:
		if t.ret.isZero() {
			return cterm{test: "nil"}
		}
		rt := f.lf.ft.Result()
		v := f.lexprTo(t.ret, f.ret)
		s := f.conv(v, f.ret, rt)
		if isAggr(rt) {
			s = "(.copy " + tagged(cv{s: s, t: f.ret}) + ")" // a struct is returned by value
		}
		return cterm{test: wrapPre(f.takePre(), s)}
	case tFall:
		return cterm{test: "(throw (IllegalStateException. " + cljQuote(f.name+": the end of a function with a result") + "))"}
	}
	return cterm{}
}

// takePre is the steps written so far for a terminator's own expression,
// which it keeps.
func (f *cfn) takePre() []cbind {
	r := f.pre
	f.pre = nil
	return r
}

// typeOf is a lowered value's C type.
func (e lexpr) typeOf() cc.Type {
	if e.v != nil {
		return e.v.c
	}
	return e.n.Type()
}

// header is the defn: the parameters, the boxes and objects made at the
// start, and body.
func (f *cfn) header(body string) string {
	lf := f.lf
	c := f.c
	prim := len(lf.params)+1 <= 4
	var ps []string
	var starts []cbind
	for _, v := range lf.params {
		cvr := f.vars[v]
		name := v.name
		if cvr.boxed {
			name = f.tmpName(v.name)
			k, sc := scalarKind(v.c)
			val := name
			if sc {
				if !prim && !k.boolean {
					val = "(long " + name + ")"
				}
				val = storeK(val, k)
			}
			starts = append(starts, cbind{"^" + cljHint(cvr.jt+"[]") + " " + v.name, "(doto " + f.newBoxOf(cvr.jt) + " (aset 0 " + val + "))"})
		}
		switch h := cljHint(cvr.jt); {
		case isIntJ(cvr.jt) && prim:
			ps = append(ps, "^long "+name)
		case isIntJ(cvr.jt):
			ps = append(ps, name)
			if !cvr.boxed {
				starts = append(starts, cbind{name, "(long " + name + ")"})
			}
		case h != "":
			ps = append(ps, "^"+h+" "+name)
		default:
			ps = append(ps, name)
		}
	}
	for _, v := range lf.vars {
		cvr := f.vars[v]
		if v.param {
			continue
		}
		switch {
		case cvr.boxed:
			starts = append(starts, cbind{"^" + cljHint(cvr.jt+"[]") + " " + v.name, f.newBoxOf(cvr.jt)})
		case cvr.fixed:
			starts = append(starts, cbind{"^" + cljHint(cvr.jt) + " " + v.name, f.newOf(v.c, cvr.jt)})
		}
	}
	ret := ""
	if isIntJ(f.ret) && prim {
		ret = "^long "
	} else if h := cljHint(f.ret); h != "" {
		ret = "^" + h + " "
	}
	var b strings.Builder
	fmt.Fprintf(&b, "(defn %s %s[^Editor ed%s]\n", c.fnName(f.name), ret, strings.Join(append([]string{""}, ps...), " "))
	if len(starts) > 0 {
		fmt.Fprintf(&b, "  (let [%s]\n", bindText(starts, "\n        "))
		b.WriteString(indent(body, 4))
		b.WriteString("))\n")
	} else {
		b.WriteString(indent(body, 2))
		b.WriteString(")\n")
	}
	return b.String()
}

// newBoxOf is a one-element Java array of Java type jt.
func (f *cfn) newBoxOf(jt string) string {
	switch jt {
	case "byte":
		return "(byte-array 1)"
	case "short":
		return "(short-array 1)"
	case "int":
		return "(int-array 1)"
	case "long":
		return "(long-array 1)"
	case "boolean":
		return "(boolean-array 1)"
	}
	return "(object-array 1)"
}

// indent indents every line of s by n spaces.
func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

// --- the structure ------------------------------------------------------

// rebindable says a variable is a Clojure local that a step rebinds: not a
// box, not an object made at the start.
func (f *cfn) rebindable(v *lvar) bool {
	cvr := f.vars[v]
	return !cvr.boxed && !cvr.fixed
}

// zeroVal is the value a rebindable variable starts with where C reads it
// before it is assigned: Java's zero, as the Java backend declares it.
func (f *cfn) zeroVal(v *lvar) string {
	cvr := f.vars[v]
	switch {
	case cvr.jt == "boolean":
		return "false"
	case isIntJ(cvr.jt):
		return "0"
	}
	return "nil"
}

// bindName is v's name in a binding vector: hinted with its class.
func (f *cfn) bindName(v *lvar) string {
	if h := cljHint(f.vars[v].jt); h != "" {
		return "^" + h + " " + v.name
	}
	return v.name
}

// structure nests the blocks: the body, and how it was written.
func (f *cfn) structure() (string, string, string) {
	lf := f.lf
	s := &shaper{f: f, live: lf.live()}
	s.analyze()
	body, ok := s.structured()
	if ok {
		return s.entryLets(body), "structured", ""
	}
	body = s.machine()
	if s.cost(body) > f.c.splitAt() {
		b := s.split()
		return b, "split", s.pre
	}
	return s.entryLets(body), "machine", ""
}

// cost is a guess at the bytecode a state machine's text becomes: its
// length, and each recur's stores of every variable it carries.
func (s *shaper) cost(body string) int {
	return len(body) + 10*(strings.Count(body, "(recur ")+strings.Count(body, "(do (aset fl__"))*len(s.vars)
}

// splitAt is the cost past which a function is split: the JVM's 64 KB of
// bytecode a method, measured on Clojure's (Profile.CljSplit).
func (c *cgen) splitAt() int {
	if c.g.p.CljSplit > 0 {
		return c.g.p.CljSplit
	}
	return 110000
}

// split is a state machine too large for one method: its states in groups,
// each group a function of the editor, a frame of the carried variables
// (fl__ their longs, fo__ the rest, fo__[0] the result) and the state to
// start at; it runs its own states and returns the next state of another
// group, or -1 when the function returns.  The function runs the groups.
func (s *shaper) split() string {
	f := s.f
	// every block a state: a group's size is then the sum of small ones
	s.state = map[*lblock]int{}
	s.states = nil
	carried := map[*lvar]bool{}
	for _, b := range f.lf.blocks {
		s.state[b] = len(s.states)
		s.states = append(s.states, b)
		for v := range s.live[b] {
			if f.rebindable(v) {
				carried[v] = true
			}
		}
	}
	s.vars = f.lf.sortedVars(carried)
	s.all = true
	// the frame
	s.slot = map[*lvar]int{}
	nl, no := 0, 1
	for _, v := range s.vars {
		if isIntJ(f.vars[v].jt) {
			s.slot[v] = nl
			nl++
		} else {
			s.slot[v] = no
			no++
		}
	}
	var fixed []*lvar
	for _, v := range f.lf.vars {
		if !f.rebindable(v) {
			fixed = append(fixed, v)
			s.slot[v] = no
			no++
		}
	}
	// the groups: states in order, until a group's text is large enough
	s.group = make([]int, len(s.states))
	size, g := 0, 0
	for i, st := range s.states {
		n := s.cost(s.emit(st, nil))
		if size > 0 && size+n > f.c.splitAt()/4 {
			g++
			size = 0
		}
		s.group[i] = g
		size += n
	}
	ngroups := g + 1
	s.splitting = true
	var defs strings.Builder
	name := f.c.fnName(f.name)
	var loads []string
	for _, v := range fixed {
		h := cljHint(f.vars[v].jt)
		if f.vars[v].boxed {
			h = cljHint(f.vars[v].jt + "[]")
		}
		loads = append(loads, fmt.Sprintf("^%s %s (aget fo__ %d)", h, v.name, s.slot[v]))
	}
	for g := 0; g < ngroups; g++ {
		s.cur = g
		binds := []string{"st st0__"}
		for _, v := range s.vars {
			if isIntJ(f.vars[v].jt) {
				binds = append(binds, fmt.Sprintf("%s (aget fl__ %d)", v.name, s.slot[v]))
			} else {
				binds = append(binds, fmt.Sprintf("%s (aget fo__ %d)", f.bindName(v), s.slot[v]))
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "(loop [%s]\n  (case st\n", strings.Join(binds, "\n       "))
		for i, st := range s.states {
			if s.group[i] == g {
				fmt.Fprintf(&b, "    %d\n%s\n", i, indent(s.emit(st, nil), 4))
			}
		}
		// a state of another group: the frame stored, and it is the next
		b.WriteString(indent(s.leave("st"), 4) + "))")
		body := b.String()
		if len(loads) > 0 {
			body = "(let [" + strings.Join(loads, "\n      ") + "]\n" + indent(body, 2) + ")"
		}
		fmt.Fprintf(&defs, "(defn- %s__%d ^long [^Editor ed ^longs fl__ ^objects fo__ ^long st0__]\n%s)\n\n", name, g, indent(body, 2))
	}
	s.pre = defs.String()
	// the function: the frame filled, the groups run
	var b strings.Builder
	fmt.Fprintf(&b, "(let [fl__ (long-array %d)\n      fo__ (object-array %d)]\n", nl, no)
	for _, v := range s.vars {
		init := s.initial(v)
		if s.live[f.lf.blocks[0]][v] && !v.param {
			init = f.zeroVal(v)
		}
		if isIntJ(f.vars[v].jt) {
			fmt.Fprintf(&b, "  (aset fl__ %d %s)\n", s.slot[v], init)
		} else if init != "nil" {
			fmt.Fprintf(&b, "  (aset fo__ %d %s)\n", s.slot[v], init)
		}
	}
	for _, v := range fixed {
		fmt.Fprintf(&b, "  (aset fo__ %d %s)\n", s.slot[v], v.name)
	}
	b.WriteString("  (loop [st 0]\n    (let [r__ (long (cond")
	for g := 0; g < ngroups; g++ {
		last := -1
		for i := range s.states {
			if s.group[i] == g {
				last = i
			}
		}
		if g == ngroups-1 {
			fmt.Fprintf(&b, "\n                :else (%s__%d ed fl__ fo__ st)", name, g)
		} else {
			fmt.Fprintf(&b, "\n                (<= st %d) (%s__%d ed fl__ fo__ st)", last, name, g)
		}
	}
	b.WriteString("))]\n      (if (neg? r__)\n        ")
	switch {
	case f.ret == "void":
		b.WriteString("nil")
	case isIntJ(f.ret):
		b.WriteString("(long (aget fo__ 0))")
	case cljHint(f.ret) != "":
		b.WriteString("(let [^" + cljHint(f.ret) + " r__ (aget fo__ 0)] r__)")
	default:
		b.WriteString("(aget fo__ 0)")
	}
	b.WriteString("\n        (recur r__)))))")
	return b.String()
}

// shaper nests one function's blocks.
type shaper struct {
	f      *cfn
	live   map[*lblock]map[*lvar]bool
	back   map[[2]int]bool // the back edges, by block ids
	header map[*lblock]bool
	edges  map[*lblock]int // incoming edges
	dup    map[*lblock]bool
	states []*lblock
	state  map[*lblock]int
	vars   []*lvar // what a recur carries
	// a split machine's
	splitting bool
	slot      map[*lvar]int
	group     []int
	cur       int
	pre       string // the groups' functions
	all       bool   // every block is a state
}

func (s *shaper) analyze() {
	f := s.f
	s.back = map[[2]int]bool{}
	s.header = map[*lblock]bool{}
	s.edges = map[*lblock]int{}
	s.dup = map[*lblock]bool{}
	for _, b := range f.lf.blocks {
		for _, t := range b.term.to {
			s.edges[t]++
			if t.id <= b.id {
				s.back[[2]int{b.id, t.id}] = true
				s.header[t] = true
			}
		}
	}
	for _, b := range f.lf.blocks {
		if s.header[b] || b.id == 0 || s.edges[b] < 2 {
			continue
		}
		// a small block that returns is written at each edge
		if len(b.steps) <= 2 && (b.term.kind == tRet || b.term.kind == tFall) && !s.hasLiteral(b) {
			s.dup[b] = true
		}
	}
}

// hasLiteral says a block's text holds a string literal: the suite's
// control needs " INSERT" once, and a copy would make it twice.
func (s *shaper) hasLiteral(b *lblock) bool {
	for _, st := range s.f.steps[b] {
		if strings.Contains(st.form, "(BytePtr/lit ") {
			return true
		}
	}
	return strings.Contains(s.f.terms[b].test, "(BytePtr/lit ")
}

// inline says a block is written where the edge to it is.
func (s *shaper) inline(b *lblock) bool {
	if b.id == 0 || s.header[b] || s.all {
		return false
	}
	return s.edges[b] == 1 || s.dup[b]
}

// structured is the body when the blocks nest with no state machine: a
// tree from the entry, no joins, no loops.
func (s *shaper) structured() (string, bool) {
	for _, b := range s.f.lf.blocks {
		if b.id != 0 && !s.inline(b) {
			return "", false
		}
		if s.header[b] {
			return "", false
		}
	}
	return s.emit(s.f.lf.blocks[0], nil), true
}

// machine is the body as a state machine.
func (s *shaper) machine() string {
	f := s.f
	s.state = map[*lblock]int{}
	for _, b := range f.lf.blocks {
		if b.id == 0 || !s.inline(b) {
			s.state[b] = len(s.states)
			s.states = append(s.states, b)
		}
	}
	carried := map[*lvar]bool{}
	for _, b := range s.states {
		for v := range s.live[b] {
			if f.rebindable(v) {
				carried[v] = true
			}
		}
	}
	s.vars = f.lf.sortedVars(carried)
	f.nvarsCarried = len(s.vars)
	var binds []string
	binds = append(binds, "st 0")
	for _, v := range s.vars {
		binds = append(binds, f.bindName(v)+" "+s.initial(v))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "(loop [%s]\n  (case st\n", strings.Join(binds, "\n       "))
	for i, st := range s.states {
		fmt.Fprintf(&b, "    %d\n%s\n", i, indent(s.emit(st, nil), 4))
	}
	fmt.Fprintf(&b, "    (throw (IllegalStateException. \"no state\"))))")
	return b.String()
}

// initial is a carried variable's value as the machine starts: a
// parameter's, or its zero.
func (s *shaper) initial(v *lvar) string {
	if v.param {
		return v.name
	}
	if s.live[s.f.lf.blocks[0]][v] {
		return v.name // bound at the start: C may read it before it is set
	}
	return s.f.zeroVal(v)
}

// entryLets binds, before body, the variables C may read before it sets
// them: Java's zero.
func (s *shaper) entryLets(body string) string {
	f := s.f
	var bs []cbind
	for _, v := range f.lf.sortedVars(s.live[f.lf.blocks[0]]) {
		if v.param || !f.rebindable(v) {
			continue
		}
		bs = append(bs, cbind{f.bindName(v), f.zeroVal(v)})
	}
	if len(bs) == 0 {
		return body
	}
	return "(let [" + bindText(bs, "\n      ") + "]\n" + indent(body, 2) + ")"
}

// emit is block b and what it inlines, as one expression.
func (s *shaper) emit(b *lblock, _ any) string {
	f := s.f
	term := s.emitTerm(b)
	binds := f.steps[b]
	if len(binds) == 0 {
		return term
	}
	var bs []string
	for _, bd := range binds {
		name := bd.name
		if name != "_" {
			if v := s.varNamed(name); v != nil {
				name = f.bindName(v)
			}
		}
		bs = append(bs, name+" "+bd.form)
	}
	return "(let [" + strings.Join(bs, "\n      ") + "]\n" + indent(term, 2) + ")"
}

// varNamed is the variable a binding's name is.
func (s *shaper) varNamed(name string) *lvar {
	for _, v := range s.f.lf.vars {
		if v.name == name {
			return v
		}
	}
	return nil
}

// boxed is a value of Java type jt as an Object: a primitive boxed.
func boxed(s, jt string) string {
	switch {
	case isIntJ(jt):
		return "(Long/valueOf " + s + ")"
	case jt == "boolean":
		return "(Boolean/valueOf (boolean " + s + "))"
	}
	return s
}

// leave is a split machine's group's value r, the carried variables
// stored in the frame first.
func (s *shaper) leave(r string) string {
	f := s.f
	var st []string
	for _, v := range s.vars {
		if isIntJ(f.vars[v].jt) {
			st = append(st, fmt.Sprintf("(aset fl__ %d %s)", s.slot[v], v.name))
		} else {
			st = append(st, fmt.Sprintf("(aset fo__ %d %s)", s.slot[v], boxed(v.name, f.vars[v].jt)))
		}
	}
	return "(do " + strings.Join(append(st, r), "\n    ") + ")"
}

func (s *shaper) emitTerm(b *lblock) string {
	f := s.f
	t := f.terms[b]
	switch b.term.kind {
	case tRet:
		if s.splitting {
			if b.term.ret.isZero() {
				return "-1"
			}
			return "(do (aset fo__ 0 " + boxed(t.test, f.ret) + ")\n    -1)"
		}
		return t.test
	case tFall:
		return t.test
	case tGoto:
		return s.edge(b.term.to[0])
	case tIf:
		a, e := s.edge(b.term.to[0]), s.edge(b.term.to[1])
		if t.swap {
			a, e = e, a
		}
		return "(if " + t.test + "\n  " + indent(a, 2)[2:] + "\n  " + indent(e, 2)[2:] + ")"
	case tSwitch:
		var sb strings.Builder
		fmt.Fprintf(&sb, "(case %s", t.test)
		for i, vs := range t.cases {
			var ks []string
			for _, v := range vs {
				ks = append(ks, fmt.Sprint(v))
			}
			sort.Slice(ks, func(a, c int) bool { return ks[a] < ks[c] })
			key := ks[0]
			if len(ks) > 1 {
				key = "(" + strings.Join(ks, " ") + ")"
			}
			fmt.Fprintf(&sb, "\n  %s\n%s", key, indent(s.edge(b.term.to[i]), 4))
		}
		fmt.Fprintf(&sb, "\n%s)", indent(s.edge(b.term.to[len(b.term.to)-1]), 2))
		return sb.String()
	}
	return "nil"
}

// edge is a jump to b: b written here, or a recur to its state.
func (s *shaper) edge(b *lblock) string {
	if s.inline(b) {
		return s.emit(b, nil)
	}
	k, ok := s.state[b]
	if !ok {
		s.f.no(nil, "a jump to block %d that is no state", b.id)
	}
	parts := []string{"recur", fmt.Sprint(k)}
	for _, v := range s.vars {
		parts = append(parts, v.name)
	}
	return "(" + strings.Join(parts, " ") + ")"
}
