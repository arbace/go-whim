package togo

// ml_expr.go prints the Scheme printer's forms (scm_fn.go, scm_expr.go,
// as scm_tidy.go reads them back) as OCaml: a define a let, a local define
// a let rec ... in, a named let a let rec and its call, let, let* and
// let-values lets, if, cond, case, when and unless OCaml's if and match,
// the runtime's operations the runtime's functions or OCaml's operators.
// What OCaml needs that Scheme does not say is the types: a form whose
// value is not looked at is () -- a call of a function with a value
// ignored -- which the printer knows from the C's types of the functions
// it calls and of the function it is in; the rest OCaml infers.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The precedences of OCaml's expressions, low to high.
const (
	precOpen   = 0 // let, match, fun: they extend as far as they can
	precSeq    = 5
	precIf     = 10
	precAssign = 12
	precTuple  = 15
	precOr     = 20
	precAnd    = 25
	precCmp    = 30
	precCons   = 45
	precAdd    = 50
	precMul    = 60
	precShift  = 70
	precNeg    = 80
	precApp    = 90
	precAtom   = 100
)

// mlx is an OCaml expression: its layout, its precedence, and what it
// ends with that would take what follows it -- "let" (a let, fun or seq's
// last), "match", "if" (an if with no else), "" nothing.
type mlx struct {
	d    *mdoc
	prec int
	tail string
}

func mlAtom(s string) mlx { return mlx{d: mtext(s), prec: precAtom} }

// mlParen is x where an expression of precedence at least p is wanted.
func mlParen(x mlx, p int) mlx {
	if x.prec >= p && (x.tail == "" || p <= precOpen) {
		return x
	}
	return mlx{d: mcat(mtext("("), mnest(1, x.d), mtext(")")), prec: precAtom}
}

// mlLetTail is the tail of a let whose body is x: what a let takes, and
// what its body's last match takes besides.
func mlLetTail(x mlx) string {
	if x.tail == "match" {
		return "match"
	}
	return "let"
}

// mlClosed is x where something follows it: parenthesized when it would
// take what follows -- a let a ; after it, a match a | or a ; (an if
// with no else takes an else, which mlArm sees to).
func mlClosed(x mlx, p int) mlx {
	if x.tail == "let" || x.tail == "match" {
		return mlx{d: mcat(mtext("("), mnest(1, x.d), mtext(")")), prec: precAtom}
	}
	return mlParen(x, p)
}

// What a form's value is wanted as.
const (
	wantValue = iota
	wantUnit
)

// mlBind is what a Scheme name means in a scope: a variable, a local of
// the C's in the call's frame, or a local function.
type mlBind struct {
	kind    int
	name    string // the OCaml name: the variable's, the frame local's address's
	memKind string // a frame local's: s32..., agg
	used    bool
	node    *mdoc            // a variable's name where it is bound: _name when it is not used
	calls   map[*mlBind]bool // a local function's: the local functions its body names
}

const (
	mbVar = iota
	mbFrame
	mbFn
)

type mlScope struct {
	up *mlScope
	m  map[string]*mlBind
}

func newMlScope(up *mlScope) *mlScope { return &mlScope{up: up, m: map[string]*mlBind{}} }

func (sc *mlScope) find(n string) *mlBind {
	for ; sc != nil; sc = sc.up {
		if b, ok := sc.m[n]; ok {
			return b
		}
	}
	return nil
}

// mlfn is one function being printed.
type mlfn struct {
	m       *mlgen
	unitRet bool            // the function's value is ()
	refs    map[string]bool // the functions at file scope it names
	vars    []*mlBind       // the variables it binds
	stack   []*mlBind       // the local functions whose bodies are being printed
}

// named marks the binding b named where it is printed: a variable used, a
// local function called from the bodies being printed.
func (f *mlfn) named(b *mlBind) {
	b.used = true
	if b.kind == mbFn {
		for _, s := range f.stack {
			s.calls[b] = true
		}
	}
}

// ed is the editor's name where the printer writes it itself -- the
// memory's accessors take it -- marked used.
func (f *mlfn) ed(sc *mlScope) string {
	if b := sc.find("ed"); b != nil {
		f.named(b)
	}
	return "ed"
}

// finish names the variables bound and never used _name, as OCaml asks.
func (f *mlfn) finish() {
	for _, b := range f.vars {
		if !b.used && b.node.s != "()" && !strings.HasPrefix(b.node.s, "_") {
			b.node.s = "_" + b.node.s
		}
	}
}

// mlWords is ds with a space between them, () for none.
func mlWords(ds []*mdoc) *mdoc {
	if len(ds) == 0 {
		return mtext("()")
	}
	return mjoin(mtext(" "), ds)
}

// function prints a define of the Scheme as a let's: its name, the
// parameters and the body, after `let` or `and`.
func (m *mlgen) function(x *sform) (d *mdoc, refs map[string]bool, why string) {
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(unsupported)
			if !ok {
				why = fmt.Sprint("panic: ", r)
				return
			}
			why = u.why
		}
	}()
	if x.head() != "define" || len(x.kids) < 3 || !x.kids[1].isList() || len(x.kids[1].kids) == 0 {
		panic(unsupported{"not a define of a function"})
	}
	name := x.kids[1].kids[0].atom
	f := &mlfn{m: m, unitRet: m.unitFn[name], refs: map[string]bool{}}
	sc := newMlScope(nil)
	var ps []*mdoc
	for _, p := range x.kids[1].kids[1:] {
		ps = append(ps, f.bindVar(sc, p.atom))
	}
	want := wantValue
	if f.unitRet {
		want = wantUnit
	}
	body := f.body(x.kids[2:], sc, want)
	on := m.glob[name]
	if on == "" {
		panic(unsupported{"a function with no name: " + name})
	}
	f.finish()
	return mcat(mtext(on+" "), mlWords(ps), mtext(" ="), mnest(2, mhard, body.d)), f.refs, ""
}

// localName is the OCaml name of a Scheme local: as mlMangle spells it,
// and not one of the names at file scope that are not their C's.
func (f *mlfn) localName(n string) string {
	switch n {
	case "ed", "fr", "sret", "mem":
		return n
	}
	o := mlMangle(n)
	for f.m.renamed[o] {
		o += "'"
	}
	return o
}

// bindVar binds the Scheme name n in sc as a variable, and is its name
// where it is bound.
func (f *mlfn) bindVar(sc *mlScope, n string) *mdoc {
	o := f.localName(n)
	b := &mlBind{kind: mbVar, name: o, node: mtext(o)}
	sc.m[n] = b
	f.vars = append(f.vars, b)
	return b.node
}

// body is a body's forms: its local defines -- functions and the C's
// locals in the frame -- then its expressions, the last its value.
func (f *mlfn) body(forms []*sform, up *mlScope, want int) mlx {
	sc := newMlScope(up)
	var fns []*sform
	var frames []*sform
	i := 0
	for ; i < len(forms); i++ {
		h := forms[i].head()
		if h == "define-c-local" {
			frames = append(frames, forms[i])
			continue
		}
		if h == "define" {
			fns = append(fns, forms[i])
			continue
		}
		break
	}
	if i == len(forms) {
		panic(unsupported{"a body of definitions alone"})
	}
	// the frame's locals: name its address, (fr + off), where named
	var frameBinds []*mlBind
	var frameDefs []string
	for _, x := range frames {
		// (define-c-local name &name kind off)
		n, kind, off := x.kids[1].atom, x.kids[3].atom, x.kids[4].atom
		o := f.localName(n)
		if kind != "agg" {
			o = f.localName(n + "_addr")
		}
		b := &mlBind{kind: mbFrame, name: o, memKind: kind}
		sc.m[n] = b
		sc.m["&"+n] = b
		frameBinds = append(frameBinds, b)
		frameDefs = append(frameDefs, "let "+o+" = fr + "+off+" in")
	}
	var binds []*mlBind
	for _, x := range fns {
		if !x.kids[1].isList() {
			panic(unsupported{"a define of a value"})
		}
		b := &mlBind{kind: mbFn, name: f.localName(x.kids[1].kids[0].atom), calls: map[*mlBind]bool{}}
		sc.m[x.kids[1].kids[0].atom] = b
		binds = append(binds, b)
	}
	defs := make([]*mdoc, len(fns))
	for k, x := range fns {
		psc := newMlScope(sc)
		var ps []*mdoc
		for _, p := range x.kids[1].kids[1:] {
			ps = append(ps, f.bindVar(psc, p.atom))
		}
		w := wantValue
		if f.unitRet {
			w = wantUnit
		}
		f.stack = append(f.stack, binds[k])
		b := f.body(x.kids[2:], psc, w)
		f.stack = f.stack[:len(f.stack)-1]
		defs[k] = mcat(mtext(binds[k].name+" "), mlWords(ps), mtext(" ="), mnest(2, mhard, b.d))
	}
	fnDocs := mlLocalGroups(binds, defs)
	x := f.seq(forms[i:], sc, want)
	for k := len(fnDocs) - 1; k >= 0; k-- {
		x = mlx{d: mcat(fnDocs[k], mhard, mtext("in"), mhard, x.d), prec: precOpen, tail: mlLetTail(x)}
	}
	for k := len(frameBinds) - 1; k >= 0; k-- {
		if frameBinds[k].used {
			x = mlx{d: mcat(mtext(frameDefs[k]), mhard, x.d), prec: precOpen, tail: mlLetTail(x)}
		}
	}
	return x
}

// mlLocalGroups are a body's local functions as lets, each a let of the
// functions that call each other, in an order that defines each before
// the functions that call it: let rec only where a function calls itself
// or another of its let, as OCaml asks.
func mlLocalGroups(binds []*mlBind, defs []*mdoc) []*mdoc {
	idx := map[*mlBind]int{}
	for i, b := range binds {
		idx[b] = i
	}
	edges := make([][]int, len(binds))
	for i, b := range binds {
		for c := range b.calls {
			if j, ok := idx[c]; ok {
				edges[i] = append(edges[i], j)
			}
		}
		sort.Ints(edges[i])
	}
	var out []*mdoc
	for _, comp := range mlSCC(len(binds), edges) {
		rec := len(comp) > 1 || binds[comp[0]].calls[binds[comp[0]]]
		var ds []*mdoc
		for k, i := range comp {
			kw := "and "
			if k == 0 {
				kw = "let "
				if rec {
					kw = "let rec "
				}
			}
			ds = append(ds, mcat(mtext(kw), defs[i]))
		}
		out = append(out, mjoin(mhard, ds))
	}
	return out
}

// seq is forms evaluated in turn, the last one's value wanted as want.
func (f *mlfn) seq(forms []*sform, sc *mlScope, want int) mlx {
	if len(forms) == 1 {
		return f.expr(forms[0], sc, want)
	}
	var ds []*mdoc
	for k, x := range forms {
		if k < len(forms)-1 {
			e := f.expr(x, sc, wantUnit)
			if mlIsUnit(e) {
				continue
			}
			ds = append(ds, mcat(mlClosed(e, precSeq+1).d, mtext(";")))
			continue
		}
		e := f.expr(x, sc, want)
		ds = append(ds, mlParen(e, precSeq).d)
		if len(ds) == 1 {
			return e
		}
		return mlx{d: mjoin(mhard, ds), prec: precSeq, tail: e.tail}
	}
	return mlAtom("()")
}

// mlIsUnit says x is ().
func mlIsUnit(x mlx) bool {
	return x.d.kind == mdText && x.d.s == "()"
}

// expr is the form x, its value wanted as want.
func (f *mlfn) expr(x *sform, sc *mlScope, want int) mlx {
	if !x.isList() {
		v := f.atom(x.atom, sc)
		if want == wantUnit {
			return mlAtom("()")
		}
		return v
	}
	if len(x.kids) == 0 {
		panic(unsupported{"an empty form"})
	}
	if x.kids[0].isList() {
		panic(unsupported{"a call of a form: " + x.flat()})
	}
	h := x.head()
	args := x.kids[1:]
	switch h {
	case "let", "let*":
		if len(args) > 0 && !args[0].isList() {
			return f.namedLet(x, sc, want)
		}
		return f.let(h == "let*", args[0], args[1:], sc, want)
	case "let-values", "let*-values":
		return f.letValues(args[0], args[1:], sc, want)
	case "if":
		c := f.cond(args[0], sc)
		a := f.expr(args[1], sc, want)
		if len(args) == 2 {
			return mlIf(c, a, nil)
		}
		b := f.expr(args[2], sc, want)
		return mlIf(c, a, &b)
	case "when", "unless":
		c := f.cond(args[0], sc)
		if h == "unless" {
			c = mlNot(c)
		}
		x := f.seq(args[1:], newMlScope(sc), wantUnit)
		r := mlIf(c, x, nil)
		if want == wantValue {
			panic(unsupported{"a when whose value is wanted"})
		}
		return r
	case "cond":
		return f.condForm(args, sc, want)
	case "case", "c-case":
		return f.caseForm(args, sc, want)
	case "begin":
		return f.seq(args, newMlScope(sc), want)
	case "and", "or":
		op, p := "&&", precAnd
		if h == "or" {
			op, p = "||", precOr
		}
		var xs []mlx
		for _, a := range args {
			xs = append(xs, f.expr(a, sc, wantValue))
		}
		return f.unit(mlInfixR(op, p, xs), want)
	case "not":
		return f.unit(mlNot(f.expr(args[0], sc, wantValue)), want)
	case "set!":
		return f.set(args[0].atom, f.expr(args[1], sc, wantValue), sc)
	case "lambda":
		psc := newMlScope(sc)
		var ps []*mdoc
		for _, p := range args[0].kids {
			ps = append(ps, f.bindVar(psc, p.atom))
		}
		inner := &mlfn{m: f.m, unitRet: false, refs: f.refs}
		b := inner.body(args[1:], psc, wantValue)
		inner.finish()
		return mlx{d: mgroup(mtext("fun "), mlWords(ps), mtext(" ->"), mnest(2, mline, b.d)), prec: precOpen, tail: mlLetTail(b)}
	case "values":
		var ds []*mdoc
		for _, a := range args {
			ds = append(ds, mlClosed(f.expr(a, sc, wantValue), precTuple+1).d)
		}
		if len(ds) == 1 {
			return mlx{d: ds[0], prec: precAtom}
		}
		return mlx{d: mgroup(mtext("("), mnest(1, mjoin(mcat(mtext(","), mline), ds)), mtext(")")), prec: precAtom}
	case "list":
		var ds []*mdoc
		for _, a := range args {
			ds = append(ds, mlClosed(f.expr(a, sc, wantValue), precSeq+1).d)
		}
		return mlx{d: mgroup(mtext("["), mnest(1, mjoin(mcat(mtext(";"), mline), ds)), mtext("]")), prec: precAtom}
	case "void":
		return mlAtom("()")
	case "error":
		return mlx{d: mtext("failwith " + strconv.Quote(x.flat())), prec: precApp}
	}
	if r, ok := f.runtime(h, args, sc, want); ok {
		return r
	}
	if r, ok := f.memberOp(h, args, sc, want); ok {
		return r
	}
	return f.call(h, args, sc, want)
}

// unit is x where want is said: () where its value is not looked at.
func (f *mlfn) unit(x mlx, want int) mlx {
	if want == wantUnit {
		f.m.st.ignores++
		return mlx{d: mcat(mtext("ignore "), mlParen(x, precAtom).d), prec: precApp}
	}
	return x
}

// atom is a name or a literal.
func (f *mlfn) atom(a string, sc *mlScope) mlx {
	switch a {
	case "#t":
		return mlAtom("true")
	case "#f":
		return mlAtom("false")
	case "'()":
		return mlAtom("[]")
	}
	if c := a[0]; c >= '0' && c <= '9' || c == '-' && len(a) > 1 && a[1] >= '0' && a[1] <= '9' {
		t := f.m.intLit(a)
		if strings.HasPrefix(t, "-") {
			return mlx{d: mtext(t), prec: precNeg}
		}
		return mlAtom(t)
	}
	if b := sc.find(a); b != nil {
		f.named(b)
		switch b.kind {
		case mbVar, mbFn:
			return mlAtom(b.name)
		case mbFrame:
			if b.memKind == "agg" || strings.HasPrefix(a, "&") {
				return mlAtom(b.name)
			}
			return mlx{d: mtext("ld_" + b.memKind + " " + f.ed(sc) + " " + b.name), prec: precApp}
		}
	}
	name := strings.TrimPrefix(a, "&")
	if ob := f.m.objOf[name]; ob != nil {
		if ob.agg {
			return mlAtom(ob.read)
		}
		if strings.HasPrefix(a, "&") {
			return mlAtom(ob.addr)
		}
		return mlx{d: mtext(ob.read + " " + f.ed(sc)), prec: precApp}
	}
	if g, ok := f.m.glob[a]; ok {
		if _, isFn := f.m.fns[a]; isFn {
			f.refs[a] = true
		}
		return mlAtom(g)
	}
	panic(unsupported{"a name the printer does not know: " + a})
}

// set is (set! name v): a store to an object or a local in the frame.
func (f *mlfn) set(n string, v mlx, sc *mlScope) mlx {
	val := mlParen(v, precAtom).d
	if b := sc.find(n); b != nil {
		if b.kind != mbFrame {
			panic(unsupported{"a set! of a variable: " + n})
		}
		f.named(b)
		return mlx{d: mgroup(mtext("st_"+b.memKind+" "+f.ed(sc)+" "+b.name), mnest(2, mline, val)), prec: precApp}
	}
	if ob := f.m.objOf[n]; ob != nil && !ob.agg {
		return mlx{d: mgroup(mtext(ob.set+" "+f.ed(sc)), mnest(2, mline, val)), prec: precApp}
	}
	panic(unsupported{"a set! of what is not an object: " + n})
}

// cond is a form wanted as a truth value.
func (f *mlfn) cond(x *sform, sc *mlScope) mlx { return f.expr(x, sc, wantValue) }

// mlNot is not x, its comparisons turned round.
func mlNot(x mlx) mlx {
	return mlx{d: mcat(mtext("not "), mlParen(x, precAtom).d), prec: precApp}
}

// mlIf is if c then a else b, b nil for no else.
func mlIf(c, a mlx, b *mlx) mlx {
	cd := mlClosed(c, precIf+1).d
	ad := mlArm(a)
	head := mcat(mtext("if "), mnest(3, cd), mtext(" then"))
	if b == nil {
		return mlx{d: mgroup(head, mnest(2, mline, ad)), prec: precIf, tail: "if"}
	}
	var els *mdoc
	if mlIsIf(*b) {
		els = mcat(mline, mtext("else "), b.d)
	} else {
		els = mcat(mline, mtext("else"), mnest(2, mline, mlParen(*b, precIf).d))
	}
	return mlx{d: mgroup(head, mnest(2, mline, ad), els), prec: precIf, tail: b.tail}
}

// mlArm is a then-branch: parenthesized where it is a sequence or would
// take the else.
func mlArm(a mlx) *mdoc {
	if a.prec <= precSeq || a.tail != "" {
		return mcat(mtext("("), mnest(1, a.d), mtext(")"))
	}
	return mlParen(a, precIf+1).d
}

// mlIsIf says x is an if with an else: an else-if may follow an else.
func mlIsIf(x mlx) bool {
	return x.prec == precIf && x.tail != "if" && x.d.kind == mdGroup && len(x.d.kids) > 0 &&
		x.d.kids[0].kind == mdCat && len(x.d.kids[0].kids) > 0 && x.d.kids[0].kids[0].kind == mdText && x.d.kids[0].kids[0].s == "if "
}

// condForm is a cond: an if for each clause.
func (f *mlfn) condForm(clauses []*sform, sc *mlScope, want int) mlx {
	if len(clauses) == 0 {
		return mlAtom("()")
	}
	c := clauses[0]
	if c.kids[0].atom == "else" {
		return f.seq(c.kids[1:], newMlScope(sc), want)
	}
	t := f.cond(c.kids[0], sc)
	var a mlx
	if len(c.kids) == 1 {
		panic(unsupported{"a cond clause of a test alone"})
	}
	a = f.seq(c.kids[1:], newMlScope(sc), want)
	if len(clauses) == 1 {
		if want == wantValue {
			panic(unsupported{"a cond with no else whose value is wanted"})
		}
		return mlIf(t, a, nil)
	}
	rest := f.condForm(clauses[1:], sc, want)
	return mlIf(t, a, &rest)
}

// caseForm is a case or a c-case: a match on the integer's values.
func (f *mlfn) caseForm(args []*sform, sc *mlScope, want int) mlx {
	e := f.expr(args[0], sc, wantValue)
	var arms []*mdoc
	hasElse := false
	for k, c := range args[1:] {
		var pat string
		if !c.kids[0].isList() && c.kids[0].atom == "else" {
			pat = "_"
			hasElse = true
		} else {
			var ps []string
			for _, l := range c.kids[0].kids {
				ps = append(ps, f.label(l.atom))
			}
			pat = strings.Join(ps, " | ")
		}
		b := f.seq(c.kids[1:], newMlScope(sc), want)
		last := k == len(args)-2
		bd := b.d
		if !last && b.tail == "match" {
			bd = mlParen(mlx{d: b.d, prec: precAtom - 1, tail: b.tail}, precAtom).d
		}
		arms = append(arms, mgroup(mtext("| "+pat+" ->"), mnest(4, mline, bd)))
	}
	if !hasElse {
		arms = append(arms, mtext("| _ -> ()"))
	}
	head := mgroup(mtext("match "), mnest(6, mlClosed(e, precIf+1).d), mtext(" with"))
	return mlx{d: mcat(head, mhard, mjoin(mhard, arms)), prec: precOpen, tail: "match"}
}

// label is a case label as a pattern: a number, a constant's value, a
// character's code.
func (f *mlfn) label(l string) string {
	if strings.HasPrefix(l, `#\`) {
		v, ok := scmCharValue(l)
		if !ok {
			panic(unsupported{"a character label " + l})
		}
		return fmt.Sprintf("%d (* %s *)", v, mlCharComment(v))
	}
	if v, ok := f.m.enumVal[l]; ok {
		return f.m.intLit(v) + " (* " + l + " *)"
	}
	return f.m.intLit(l)
}

// mlCharComment is how a comment shows the character of code v.
func mlCharComment(v int64) string {
	if v >= 0x20 && v < 0x7f && v != '\'' && v != '\\' && v != '"' {
		return "'" + string(rune(v)) + "'"
	}
	return fmt.Sprintf("'\\x%02x'", v)
}

// scmCharValue is the code of a Scheme character literal.
func scmCharValue(l string) (int64, bool) {
	s := l[2:]
	switch s {
	case "space":
		return ' ', true
	case "tab":
		return '\t', true
	case "newline":
		return '\n', true
	case "return":
		return '\r', true
	case "esc":
		return 27, true
	case "delete":
		return 127, true
	case "backspace":
		return 8, true
	case "alarm":
		return 7, true
	case "nul":
		return 0, true
	}
	if len(s) == 1 {
		return int64(s[0]), true
	}
	if s[0] == 'x' {
		v, err := strconv.ParseInt(s[1:], 16, 64)
		return v, err == nil
	}
	return 0, false
}

// let is a let's or a let*'s bindings and body.
func (f *mlfn) let(star bool, bs *sform, body []*sform, sc *mlScope, want int) mlx {
	inner := newMlScope(sc)
	var group [][]mlLet
	var cur []mlLet
	var names []string
	for _, b := range bs.kids {
		n := b.kids[0].atom
		if n == "mem" {
			continue // the memory is the editor's: ed reaches it
		}
		at := sc
		if star {
			at = inner
		}
		x := f.expr(b.kids[1], at, wantValue)
		if star {
			o := f.bindVar(inner, n)
			group = append(group, []mlLet{{o, x}})
			continue
		}
		cur = append(cur, mlLet{nil, x})
		names = append(names, n)
	}
	if !star && len(cur) > 0 {
		for k := range cur {
			cur[k].pat = f.bindVar(inner, names[k])
		}
		group = append(group, cur)
	}
	x := f.body(body, inner, want)
	return f.wrapLets(group, x)
}

// mlLet is a binding of a let: its pattern and its value.
type mlLet struct {
	pat *mdoc
	x   mlx
}

// wrapLets is x after the lets of groups, each group one let ... and ...
func (f *mlfn) wrapLets(groups [][]mlLet, x mlx) mlx {
	for k := len(groups) - 1; k >= 0; k-- {
		var ds []*mdoc
		for i, b := range groups[k] {
			kw := "let "
			if i > 0 {
				kw = "and "
			}
			ds = append(ds, mgroup(mtext(kw), b.pat, mtext(" ="), mnest(2, mline, b.x.d)))
		}
		x = mlx{d: mcat(mgroup(mjoin(mline, ds), mline, mtext("in")), mhard, x.d), prec: precOpen, tail: mlLetTail(x)}
	}
	return x
}

// letValues is a let-values: each clause's values bound as a tuple.
func (f *mlfn) letValues(bs *sform, body []*sform, sc *mlScope, want int) mlx {
	inner := newMlScope(sc)
	var groups [][]mlLet
	for _, b := range bs.kids {
		x := f.expr(b.kids[1], sc, wantValue)
		var ns []*mdoc
		for _, n := range b.kids[0].kids {
			ns = append(ns, f.bindVar(inner, n.atom))
		}
		pat := mjoin(mtext(", "), ns)
		if len(ns) > 1 {
			pat = mcat(mtext("("), pat, mtext(")"))
		}
		groups = append(groups, []mlLet{{pat, x}})
		sc = inner
	}
	x := f.body(body, inner, want)
	return f.wrapLets(groups, x)
}

// namedLet is (let loop ([p v] ...) body): let rec loop p ... = body in
// loop v ...
func (f *mlfn) namedLet(x *sform, sc *mlScope, want int) mlx {
	name := x.kids[1].atom
	bs := x.kids[2].kids
	var inits []mlx
	for _, b := range bs {
		inits = append(inits, f.expr(b.kids[1], sc, wantValue))
	}
	lsc := newMlScope(sc)
	self := &mlBind{kind: mbFn, name: f.localName(name), calls: map[*mlBind]bool{}}
	lsc.m[name] = self
	psc := newMlScope(lsc)
	var ps []*mdoc
	for _, b := range bs {
		ps = append(ps, f.bindVar(psc, b.kids[0].atom))
	}
	w := wantValue
	if f.unitRet {
		w = wantUnit
	}
	f.stack = append(f.stack, self)
	body := f.body(x.kids[3:], psc, w)
	f.stack = f.stack[:len(f.stack)-1]
	on := self.name
	call := f.apply(on, inits)
	kw := "let "
	if self.calls[self] {
		kw = "let rec "
	}
	return mlx{d: mcat(mtext(kw+on+" "), mlWords(ps), mtext(" ="), mnest(2, mhard, body.d), mhard, mtext("in"), mhard, call.d),
		prec: precOpen, tail: mlLetTail(call)}
}

// apply is fn applied to args, () for none.
func (f *mlfn) apply(fn string, args []mlx) mlx {
	if len(args) == 0 {
		return mlx{d: mtext(fn + " ()"), prec: precApp}
	}
	var ds []*mdoc
	for _, a := range args {
		// each argument on the line before it while it fits: a fill
		ds = append(ds, mgroup(mline, mlParen(a, precAtom).d))
	}
	return mlx{d: mgroup(mtext(fn), mnest(2, ds...)), prec: precApp}
}

// call is a call of a function: the function's own, a local one, the
// host's.
func (f *mlfn) call(h string, args []*sform, sc *mlScope, want int) mlx {
	var xs []mlx
	for _, a := range args {
		xs = append(xs, f.expr(a, sc, wantValue))
	}
	if b := sc.find(h); b != nil {
		if b.kind != mbFn {
			panic(unsupported{"a call of a variable: " + h})
		}
		f.named(b)
		x := f.apply(b.name, xs)
		if want == wantUnit && !f.unitRet {
			return f.unit(x, want)
		}
		return x
	}
	g, ok := f.m.glob[h]
	if !ok {
		panic(unsupported{"a call of what the printer does not know: " + h})
	}
	if _, isFn := f.m.fns[h]; !isFn {
		panic(unsupported{"a call of what is not a function: " + h})
	}
	f.refs[h] = true
	x := f.apply(g, xs)
	if want == wantUnit && !f.m.unitFn[h] {
		return f.unit(x, want)
	}
	return x
}

// memberOp is a member's read, write or address: (S.m p), (S.m-set! p
// v), (S.m& p).
func (f *mlfn) memberOp(h string, args []*sform, sc *mlScope, want int) (mlx, bool) {
	base, op := h, "read"
	switch {
	case strings.HasSuffix(h, "-set!"):
		base, op = strings.TrimSuffix(h, "-set!"), "set"
	case strings.HasSuffix(h, "&"):
		base, op = strings.TrimSuffix(h, "&"), "addr"
	}
	mm := f.m.members[base]
	if mm == nil {
		return mlx{}, false
	}
	var xs []mlx
	for _, a := range args {
		xs = append(xs, f.expr(a, sc, wantValue))
	}
	switch op {
	case "addr":
		return f.unit(f.apply(mm.module+"."+mm.addr, xs), want), true
	case "set":
		return f.apply(mm.module+"."+mm.set+" "+f.ed(sc), xs), true
	}
	return f.unit(f.apply(mm.module+"."+mm.read+" "+f.ed(sc), xs), want), true
}

// mlInfixL is xs joined by the left-associative operator op of
// precedence p.
func mlInfixL(op string, p int, xs []mlx) mlx {
	d := mlClosed(xs[0], p).d
	for _, x := range xs[1:] {
		d = mcat(d, mtext(" "+op), mline, mlClosed(x, p+1).d)
	}
	return mlx{d: mgroup(d), prec: p}
}

// mlInfixR is xs joined by the right-associative operator op.
func mlInfixR(op string, p int, xs []mlx) mlx {
	if len(xs) == 1 {
		return xs[0]
	}
	var ds []*mdoc
	for i, x := range xs {
		q := p + 1
		if i == len(xs)-1 {
			q = p
		}
		if i > 0 {
			ds = append(ds, mtext(" "+op), mline)
		}
		ds = append(ds, mlClosed(x, q).d)
	}
	return mlx{d: mgroup(ds...), prec: p}
}

// mlBinOps are the Scheme's operations OCaml has as operators: the
// operator, its precedence and whether it associates to the left.
var mlBinOps = map[string]struct {
	op   string
	prec int
}{
	"fx+": {"+", precAdd}, "+": {"+", precAdd}, "u64+": {"+", precAdd},
	"fx-": {"-", precAdd}, "-": {"-", precAdd}, "u64-": {"-", precAdd},
	"fx*": {"*", precMul}, "*": {"*", precMul}, "u64*": {"*", precMul},
	"fxquotient": {"/", precMul}, "quotient": {"/", precMul}, "i32/": {"/", precMul}, "i64/": {"/", precMul}, "u32/": {"/", precMul},
	"fxremainder": {"mod", precMul}, "remainder": {"mod", precMul}, "i32%": {"mod", precMul}, "i64%": {"mod", precMul}, "u32%": {"mod", precMul},
	"fxand": {"land", precMul}, "bitwise-and": {"land", precMul},
	"fxior": {"lor", precMul}, "bitwise-ior": {"lor", precMul},
	"fxxor": {"lxor", precMul}, "bitwise-xor": {"lxor", precMul},
	"fxsll": {"lsl", precShift}, "bitwise-arithmetic-shift-left": {"lsl", precShift}, "i64<<": {"lsl", precShift}, "u64<<": {"lsl", precShift},
	"fxsra": {"asr", precShift}, "bitwise-arithmetic-shift-right": {"asr", precShift}, "i32>>": {"asr", precShift}, "i64>>": {"asr", precShift},
	"fxsrl": {"lsr", precShift}, "u32>>": {"lsr", precShift},
}

// mlCmpOps are the comparisons.
var mlCmpOps = map[string]string{
	"fx=?": "=", "=": "=", "eqv?": "=", "eq?": "=", "fx<?": "<", "<": "<", "fx>?": ">", ">": ">",
	"fx<=?": "<=", "<=": "<=", "fx>=?": ">=", ">=": ">=",
}

// mlRtFns are the Scheme runtime's operations that are the OCaml
// runtime's functions (whiml/rt.ml): the Scheme's name -> OCaml's, and
// whether it takes the editor first.
var mlRtFns = map[string]struct {
	name string
	ed   bool
}{
	"->i8": {"to_i8", false}, "->u8": {"to_u8", false}, "->i16": {"to_i16", false}, "->u16": {"to_u16", false},
	"->i32": {"to_i32", false}, "->u32": {"to_u32", false},
	"b->i":  {"Bool.to_int", false},
	"i32<<": {"i32_shl", false}, "u32+": {"u32_add", false}, "u32-": {"u32_sub", false}, "u32*": {"u32_mul", false},
	"u32<<": {"u32_shl", false}, "u32~": {"u32_not", false},
	"u64/": {"u64_div", false}, "u64%": {"u64_rem", false}, "u64>>": {"u64_shr", false},
	"u64<?": {"u64_lt", false}, "u64<=?": {"u64_le", false}, "u64>?": {"u64_gt", false}, "u64>=?": {"u64_ge", false},
	"fxnot": {"lnot", false}, "bitwise-not": {"lnot", false}, "u64~": {"lnot", false},
	"fxmin": {"min", false}, "fxmax": {"max", false}, "min": {"min", false}, "max": {"max", false},
	"mem-copy!": {"mem_copy", true}, "mem-zero!": {"mem_zero", true}, "mem-fill!": {"mem_fill", true},
	"fn-ptr": {"fn_ptr", false}, "frame-push!": {"frame_push", false}, "frame-pop!": {"frame_pop", false},
	"chunks": {"chunks", false},
}

// mlUnitOps are the runtime's operations whose value is ().
var mlUnitOps = map[string]bool{"mem-copy!": true, "mem-zero!": true, "mem-fill!": true, "frame-pop!": true}

// runtime is one of the runtime's operations, or ok false.
func (f *mlfn) runtime(h string, args []*sform, sc *mlScope, want int) (mlx, bool) {
	vals := func() []mlx {
		var xs []mlx
		for _, a := range args {
			xs = append(xs, f.expr(a, sc, wantValue))
		}
		return xs
	}
	if op, ok := mlBinOps[h]; ok {
		xs := vals()
		if len(xs) == 1 && op.op == "-" {
			return f.unit(mlx{d: mcat(mtext("- "), mlClosed(xs[0], precNeg+1).d), prec: precNeg}, want), true
		}
		if len(xs) < 2 {
			panic(unsupported{"an operation of one operand: " + h})
		}
		if op.prec == precShift {
			// right-associative: a lsl b lsl c is a lsl (b lsl c)
			return f.unit(mlx{d: mgroup(mlClosed(xs[0], precShift+1).d, mtext(" "+op.op), mline, mlClosed(xs[1], precShift+1).d), prec: precShift}, want), true
		}
		return f.unit(mlInfixL(op.op, op.prec, xs), want), true
	}
	if op, ok := mlCmpOps[h]; ok {
		xs := vals()
		if len(xs) != 2 {
			panic(unsupported{"a comparison of other than two: " + h})
		}
		return f.unit(mlx{d: mgroup(mlClosed(xs[0], precCmp+1).d, mtext(" "+op), mline, mlClosed(xs[1], precCmp+1).d), prec: precCmp}, want), true
	}
	switch h {
	case "fxzero?", "zero?":
		xs := vals()
		return f.unit(mlx{d: mcat(mlClosed(xs[0], precCmp+1).d, mtext(" = 0")), prec: precCmp}, want), true
	case "->i64", "->u64":
		return f.expr(args[0], sc, want), true
	case "c-str":
		lit := scmUnstring(args[1].atom)
		return mlx{d: mtext(args[0].atom + " (* " + mlString(lit) + " *)"), prec: precAtom}, true
	case "ch":
		v, ok := scmCharValue(args[0].atom)
		if !ok {
			panic(unsupported{"a character " + args[0].atom})
		}
		if v >= 0x20 && v < 0x7f && v != '\'' && v != '\\' {
			return mlx{d: mtext("Char.code '" + string(rune(v)) + "'"), prec: precApp}, true
		}
		return mlAtom(strconv.FormatInt(v, 10)), true
	case "fn-index":
		return mlx{d: mcat(mtext("fn_index "), mlParen(f.expr(args[0], sc, wantValue), precAtom).d), prec: precApp}, true
	}
	if strings.HasPrefix(h, "call-ptr") {
		k := strings.TrimPrefix(h, "call-ptr")
		f.m.tables[k] = true
		_, void := mlTableKind(k)
		x := f.apply("call_ptr"+k, vals())
		if want == wantUnit && !void {
			return f.unit(x, want), true
		}
		return x, true
	}
	if strings.HasPrefix(h, "ld-") {
		x := f.apply("ld_"+strings.TrimPrefix(h, "ld-")+" "+f.ed(sc), vals())
		return f.unit(x, want), true
	}
	if strings.HasPrefix(h, "st-") && strings.HasSuffix(h, "!") {
		return f.apply("st_"+strings.TrimSuffix(strings.TrimPrefix(h, "st-"), "!")+" "+f.ed(sc), vals()), true
	}
	if fn, ok := mlRtFns[h]; ok {
		name := fn.name
		if fn.ed {
			name += " " + f.ed(sc)
		}
		x := f.apply(name, vals())
		if !mlUnitOps[h] {
			x = f.unit(x, want)
		}
		return x, true
	}
	return mlx{}, false
}
