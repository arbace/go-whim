package togo

// java_stmt.go writes C's statements and functions in Java.  C's control
// flow maps to Java's almost one to one -- Java's switch falls through, its
// do-while and for run the condition and the increment on a continue -- so a
// loop is a loop and a switch a switch.  What does not map:
//
//   - every local is declared at the method's top, with its zero: Java
//     refuses a variable read before it is surely assigned, and C's local in
//     a loop keeps its value from one turn to the next (at -O0), which one
//     declaration for the whole method does too;
//   - Java refuses a statement it can prove unreachable, and a method that
//     can end without a return: jcomplete is Java's rule, written on the C,
//     and a statement after one that cannot complete is dead in both and not
//     written;
//   - a condition or increment that does something first (an assignment, a
//     call inside &&) is written before its test, and a continue that must
//     reach it leaves a labeled block around the body: `c1: { ... break c1; ... }`;
//   - a goto, a forward jump to a label of a block holding it, leaves a
//     labeled block that ends at the label: `L_x: { ... break L_x; ... }`.

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

type jlocal struct {
	name, typ string
	boxed     bool   // a one-element array: its address is taken
	init      string // its declaration's initial value; "" for a parameter
}

// ref is the local as an lvalue.
func (l *jlocal) ref() string {
	if l.boxed {
		return l.name + "[0]"
	}
	return l.name
}

// jfn is one method being written.
type jfn struct {
	j      *jgen
	name   string
	ft     *cc.FunctionType
	out    *strings.Builder
	indent int
	locals []*jlocal // declared at the method's top, in order
	byDecl map[*cc.Declarator]*jlocal
	taken  map[string]bool
	tmp    int
	cont   []string                   // per loop: what a continue is -- "continue", or "break cN"
	gotoOK map[*cc.JumpStatement]bool // a goto whose labeled block is written
	ret    string                     // the Java result type
}

func (j *jgen) newFn(name string, ft *cc.FunctionType) *jfn {
	return &jfn{j: j, name: name, ft: ft, byDecl: map[*cc.Declarator]*jlocal{}, taken: map[string]bool{}, gotoOK: map[*cc.JumpStatement]bool{}}
}

func (f *jfn) no(n cc.Node, format string, args ...any) {
	where := ""
	if n != nil {
		where = fmt.Sprintf(" at %d", n.Position().Line)
	}
	panic(unsupported{fmt.Sprintf(format, args...) + where})
}

// jt is j.jt, or the function refused, saying where the type was met.
func (f *jfn) jt(t cc.Type, key string, where ...string) string {
	s, why := f.j.jt(t, key)
	if why != "" {
		if len(where) > 0 {
			f.no(nil, "%s: %s", why, strings.Join(where, " "))
		}
		f.no(nil, "%s", why)
	}
	return s
}

func (f *jfn) line(format string, args ...any) {
	f.out.WriteString(strings.Repeat("    ", f.indent))
	fmt.Fprintf(f.out, format, args...)
	f.out.WriteString("\n")
}

// stmt1 writes one simple statement.
func (f *jfn) stmt1(format string, args ...any) {
	f.line(format+";", args...)
}

func (f *jfn) capture(fn func()) string {
	old := f.out
	b := &strings.Builder{}
	f.out = b
	fn()
	f.out = old
	return b.String()
}

// zeroOf is the zero of a Java scalar or reference type.
func zeroOf(t string) string {
	switch t {
	case "boolean":
		return "false"
	case "byte", "short", "int", "long":
		return "0"
	}
	return "null"
}

func (f *jfn) unique(base string) string {
	name := base
	for i := 2; f.taken[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	f.taken[name] = true
	return name
}

func (f *jfn) newTemp(t string, _ cc.Type) string {
	for {
		f.tmp++
		n := fmt.Sprintf("t%d", f.tmp)
		if !f.taken[n] {
			f.taken[n] = true
			f.locals = append(f.locals, &jlocal{name: n, typ: t, init: zeroOf(t)})
			return n
		}
	}
}

func (f *jfn) newLabel() string {
	for {
		f.tmp++
		n := fmt.Sprintf("c%d", f.tmp)
		if !f.taken[n] {
			f.taken[n] = true
			return n
		}
	}
}

// hoisted is the declarations of the method's locals, at its top.
func (f *jfn) hoisted() string {
	var b strings.Builder
	ind := strings.Repeat("    ", f.indent)
	for _, l := range f.locals {
		if l.init == "" {
			continue
		}
		t := l.typ
		if l.boxed {
			t += "[]"
		}
		fmt.Fprintf(&b, "%s%s %s = %s;\n", ind, t, l.name, l.init)
	}
	return b.String()
}

// declareLocal declares a C local at the method's top.
func (f *jfn) declareLocal(d *cc.Declarator, t cc.Type, jt string) *jlocal {
	l := &jlocal{name: f.unique(f.j.jName(d.Name())), typ: jt}
	switch {
	case f.j.isBoxed(d):
		l.boxed = true
		l.init = "new " + raw(jt) + "[1]"
	case isAggr(t) || t.Kind() == cc.Array:
		nw, why := f.j.jnew(t, jt)
		if why != "" {
			f.no(d, "%s", why)
		}
		l.init = nw
	default:
		l.init = zeroOf(jt)
	}
	f.locals = append(f.locals, l)
	f.byDecl[d] = l
	return l
}

// method writes one function as a method, or says why it cannot.
func (j *jgen) method(fd *cc.FunctionDefinition) (src string, why string) {
	d := fd.Declarator
	ft, _ := d.Type().(*cc.FunctionType)
	f := j.newFn(d.Name(), ft)
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
	if ft.IsVariadic() {
		f.no(d, "a variadic function")
	}
	// every name the body refers to that is not its own, and the gotos
	var names func(cc.Node)
	names = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				switch dd := x.ResolvedTo().(type) {
				case *cc.Declarator:
					if !dd.IsParam() && dd.StorageDuration() != cc.Automatic {
						f.taken[j.jName(x.Token.SrcStr())] = true
					}
				case *cc.Enumerator:
					f.taken[j.jName(x.Token.SrcStr())] = true
				}
			}
		}
		walkChildrenFn(n, names)
	}
	names(fd.CompoundStatement)
	var ps []string
	var boxes []string
	for i, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		t := p.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		jt := f.jt(t, fmt.Sprintf("param:%s:%d", d.Name(), i), "a parameter", p.Name())
		pn := p.Name()
		if pn == "" {
			pn = fmt.Sprintf("p%d", i)
		}
		name := f.unique(j.jName(pn))
		if p.Declarator != nil && j.isBoxed(p.Declarator) {
			arg := f.unique(name + "_arg")
			ps = append(ps, jt+" "+arg)
			l := &jlocal{name: name, typ: jt, boxed: true}
			f.byDecl[p.Declarator] = l
			boxes = append(boxes, fmt.Sprintf("        %s[] %s = new %s[] {%s};\n", jt, name, raw(jt), arg))
			continue
		}
		ps = append(ps, jt+" "+name)
		if p.Declarator != nil {
			f.byDecl[p.Declarator] = &jlocal{name: name, typ: jt}
		}
	}
	f.ret = f.jt(ft.Result(), "ret:"+d.Name(), "the result")
	var body strings.Builder
	f.out = &body
	f.indent = 2
	if rb := j.runtimeBody(d.Name()); rb != nil {
		// a rule of the runtime's, not a translation (Profile.RuntimeBodies)
		return fmt.Sprintf("\n    %s %s(%s) {\n%s    }\n", f.ret, j.jName(d.Name()), strings.Join(ps, ", "), rb(f.ret)), ""
	}
	f.items(fd.CompoundStatement)
	if f.ret != "void" && jcompleteItems(fd.CompoundStatement) {
		f.stmt1("throw new IllegalStateException(%s)", javaQuote(d.Name()+": the end of a function with a result"))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n    %s %s(%s) {\n", f.ret, j.jName(d.Name()), strings.Join(ps, ", "))
	for _, s := range boxes {
		b.WriteString(s)
	}
	b.WriteString(f.hoisted())
	b.WriteString(body.String())
	b.WriteString("    }\n")
	return b.String(), ""
}

// items writes a block's items, and none after one that cannot complete:
// Java refuses them as unreachable, and they are dead in C too -- until a
// label a goto reaches, which is live again.
//
// A goto is a labeled block: every goto is a forward jump to a label that
// is an item of a block holding it, so the items from the first that holds
// a goto to L up to L's are wrapped in `L_x: { ... }`, the goto is
// `break L_x;`, and the label's statement follows the block.  Two such
// blocks that would cross are made to nest by starting the later one where
// the earlier one starts.
func (f *jfn) items(cs *cc.CompoundStatement) {
	var its []*cc.BlockItem
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		its = append(its, l.BlockItem)
	}
	type gblock struct {
		labels     []string
		start, end int // the block wraps its[start:end]; its[end] is the label's
	}
	var blocks []*gblock
	for k, it := range its {
		labels := itemLabels(it)
		if len(labels) == 0 {
			continue
		}
		start := -1
		for _, lb := range labels {
			for i := 0; i < k; i++ {
				gs := gotosTo(its[i], lb)
				if len(gs) > 0 && (start < 0 || i < start) {
					start = i
				}
				for _, g := range gs {
					f.gotoOK[g] = true
				}
			}
		}
		if start >= 0 {
			blocks = append(blocks, &gblock{labels: labels, start: start, end: k})
		}
	}
	// nest: a block starting inside an earlier one and ending after it
	// starts where that one does
	for changed := true; changed; {
		changed = false
		for _, a := range blocks {
			for _, b := range blocks {
				if a.start < b.start && b.start < a.end && a.end < b.end {
					b.start = a.start
					changed = true
				}
			}
		}
	}
	live := true
	for i, it := range its {
		// open the blocks starting here, the one ending last outermost
		var opening []*gblock
		for _, b := range blocks {
			if b.start == i {
				opening = append(opening, b)
			}
		}
		for x := 0; x < len(opening); x++ {
			for y := x + 1; y < len(opening); y++ {
				if opening[y].end > opening[x].end {
					opening[x], opening[y] = opening[y], opening[x]
				}
			}
		}
		for _, b := range opening {
			var ls []string
			for _, lb := range b.labels {
				ls = append(ls, gotoLabel(lb)+":")
			}
			f.line("%s {", strings.Join(ls, " "))
			f.indent++
		}
		for _, b := range blocks {
			if b.end == i {
				live = true // reached by the block's break
			}
		}
		switch it.Case {
		case cc.BlockItemDecl:
			f.declaration(it.Declaration, live)
		case cc.BlockItemStmt:
			if live {
				f.stmt(it.Statement)
				if !jcomplete(it.Statement) {
					live = false
				}
			}
		default:
			f.no(it, "a block item %v", it.Case)
		}
		// close the blocks whose label is next
		closing := 0
		for _, b := range blocks {
			if b.end == i+1 {
				closing++
			}
		}
		for ; closing > 0; closing-- {
			f.indent--
			f.line("}")
		}
	}
}

// gotoLabel is a C label as Java's.
func gotoLabel(name string) string { return "L_" + name }

// itemLabels are the labels a block item's statement carries.
func itemLabels(it *cc.BlockItem) []string {
	if it.Case != cc.BlockItemStmt {
		return nil
	}
	var ls []string
	for st := it.Statement; st.Case == cc.StatementLabeled && st.LabeledStatement.Case == cc.LabeledStatementLabel; st = st.LabeledStatement.Statement {
		ls = append(ls, st.LabeledStatement.Token.SrcStr())
	}
	return ls
}

// gotosTo are the gotos to label n holds.
func gotosTo(n cc.Node, label string) []*cc.JumpStatement {
	var gs []*cc.JumpStatement
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if j, ok := n.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementGoto && j.Token2.SrcStr() == label {
			gs = append(gs, j)
			return
		}
		walkChildrenFn(n, rec)
	}
	rec(n)
	return gs
}

// body writes a statement as the body of an if or a loop.
func (f *jfn) body(s *cc.Statement) {
	f.indent++
	f.stmt(s)
	f.indent--
}

func (f *jfn) stmt(s *cc.Statement) {
	if s == nil {
		return
	}
	switch s.Case {
	case cc.StatementCompound:
		f.items(s.CompoundStatement)
	case cc.StatementExpr:
		if s.ExpressionStatement.ExpressionList != nil {
			f.exprStmt(s.ExpressionStatement.ExpressionList)
		}
	case cc.StatementSelection:
		f.selection(s.SelectionStatement)
	case cc.StatementIteration:
		f.iteration(s.IterationStatement)
	case cc.StatementJump:
		f.jump(s.JumpStatement)
	case cc.StatementLabeled:
		l := s.LabeledStatement
		if l.Case != cc.LabeledStatementLabel {
			f.no(s, "a case label inside a statement of its switch")
		}
		f.stmt(l.Statement)
	default:
		f.no(s, "a statement %v", s.Case)
	}
}

// declaration declares a block's locals at the method's top and, when live,
// writes their initializers where C has them.
func (f *jfn) declaration(d *cc.Declaration, live bool) {
	if d.Case != cc.DeclarationDecl {
		return
	}
	for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		id := l.InitDeclarator
		dd := id.Declarator
		t := dd.Type()
		if dd.IsTypename() || t.Kind() == cc.Function || dd.IsExtern() || dd.StorageDuration() == cc.Static {
			continue // a static is a field, initialized with the others
		}
		key := f.j.g.a.declKey(dd)
		jt := f.jt(t, key, "a local", dd.Name())
		lc := f.declareLocal(dd, t, jt)
		if live && id.Initializer != nil {
			f.initInto(lc.ref(), t, key, id.Initializer, false)
		}
	}
}

// initInto writes the statements that give target, an object of C type t,
// its initial value.  fresh says its storage is new, and zero; a local's is
// made anew for a braced list, which C zeroes where the list says nothing.
func (f *jfn) initInto(target string, t cc.Type, key string, in *cc.Initializer, fresh bool) {
	jt := f.jt(t, key)
	if in.Case == cc.InitializerExpr {
		e := in.AssignmentExpression
		switch t.Kind() {
		case cc.Array:
			sv, ok := unparenE(e).Value().(cc.StringValue)
			if !ok || !isCharType(t.(*cc.ArrayType).Elem()) {
				f.no(in, "an array initialized from an expression")
			}
			f.stmt1("Rt.init(%s, %s)", target, javaQuote(strings.TrimSuffix(string(sv), "\x00")))
		case cc.Struct, cc.Union:
			v := f.exprTo(e, jt)
			f.stmt1("%s.set(%s)", target, v.s)
		default:
			v := f.exprTo(e, jt)
			f.stmt1("%s = %s", target, f.conv(v, jt, t))
		}
		return
	}
	if !fresh && (isAggr(t) || t.Kind() == cc.Array) {
		nw, why := f.j.jnew(t, jt)
		if why != "" {
			f.no(in, "%s", why)
		}
		f.stmt1("%s = %s", target, nw)
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
			name := f.j.memberName(fl, i-1)
			if f.j.boxedField[fieldKey(fl)] {
				name += "[0]"
			}
			f.initInto(target+"."+name, fl.Type(), fieldKey(fl), l.Initializer, true)
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
			f.initInto(fmt.Sprintf("%s[%d]", target, k), x.Elem(), "elem:"+key, l.Initializer, true)
		}
	default:
		n := 0
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			n++
			if n > 1 || l.Designation != nil {
				f.no(in, "a braced initializer for a %s", t)
			}
			f.initInto(target, t, key, l.Initializer, fresh)
		}
	}
}

// isCharType says t is char, signed char or unsigned char.
func isCharType(t cc.Type) bool {
	switch t.Kind() {
	case cc.Char, cc.SChar, cc.UChar:
		return true
	}
	return false
}

// designator is an element's one designator, or nil.
func (f *jfn) designator(l *cc.InitializerList) *cc.Designator {
	if l.Designation == nil {
		return nil
	}
	dl := l.Designation.DesignatorList
	if dl.DesignatorList != nil {
		f.no(l, "a designator of more than one step")
	}
	return dl.Designator
}

func (f *jfn) condition(e cc.ExpressionNode) string {
	return f.truth(f.expr(e))
}

func (f *jfn) selection(s *cc.SelectionStatement) {
	switch s.Case {
	case cc.SelectionStatementIf, cc.SelectionStatementIfElse:
		f.line("if (%s) {", f.condition(s.ExpressionList))
		f.body(s.Statement)
		for s.Case == cc.SelectionStatementIfElse {
			el := s.Statement2
			if el.Case == cc.StatementSelection && el.SelectionStatement.Case != cc.SelectionStatementSwitch {
				var c2 string
				pre := f.capture(func() { c2 = f.condition(el.SelectionStatement.ExpressionList) })
				if pre == "" {
					f.line("} else if (%s) {", c2)
					s = el.SelectionStatement
					f.body(s.Statement)
					continue
				}
			}
			f.line("} else {")
			f.body(el)
			break
		}
		f.line("}")
	case cc.SelectionStatementSwitch:
		f.switchStmt(s)
	}
}

// caseChain is a switch body's item as its labels -- "" for default -- and
// the statement they label; no labels for any other item.
func caseChain(it *cc.BlockItem) ([]*cc.LabeledStatement, *cc.Statement) {
	if it.Case != cc.BlockItemStmt {
		return nil, nil
	}
	st := it.Statement
	var ls []*cc.LabeledStatement
	for st.Case == cc.StatementLabeled && st.LabeledStatement.Case != cc.LabeledStatementLabel {
		ls = append(ls, st.LabeledStatement)
		st = st.LabeledStatement.Statement
	}
	return ls, st
}

// switchStmt is Java's switch, which falls through as C's does.  Its
// statements after one that cannot complete are dead until the next label.
func (f *jfn) switchStmt(s *cc.SelectionStatement) {
	ck, ok := scalarKind(s.ExpressionList.Type())
	if !ok {
		f.no(s, "a switch on a %s", s.ExpressionList.Type())
	}
	pk := promote(ck)
	if pk.size == 8 {
		f.no(s, "a switch on a long")
	}
	v := f.expr(s.ExpressionList)
	if s.Statement.Case != cc.StatementCompound {
		f.no(s, "a switch whose body is not a block")
	}
	f.line("switch (%s) {", f.convK(v, pk))
	live := false
	f.indent++
	for l := s.Statement.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
		it := l.BlockItem
		if it.Case == cc.BlockItemDecl {
			f.indent++
			f.declaration(it.Declaration, live)
			f.indent--
			continue
		}
		labels, st := caseChain(it)
		for _, ls := range labels {
			switch ls.Case {
			case cc.LabeledStatementCaseLabel:
				f.line("case %s:", f.convK(f.expr(ls.ConstantExpression), pk))
			case cc.LabeledStatementDefault:
				f.line("default:")
			default:
				f.no(ls, "a case range")
			}
			live = true
		}
		if !live {
			continue
		}
		f.indent++
		f.stmt(st)
		f.indent--
		live = jcomplete(st)
	}
	f.indent--
	f.line("}")
}

func (f *jfn) loopBody(s *cc.Statement, cont string) {
	f.cont = append(f.cont, cont)
	f.body(s)
	f.cont = f.cont[:len(f.cont)-1]
}

// contBody writes a loop's body whose continue must reach what is written
// after it: inside a labeled block a continue leaves.
func (f *jfn) contBody(s *cc.Statement) {
	if !hasContinue(s) {
		f.loopBody(s, "continue")
		return
	}
	lbl := f.newLabel()
	f.line("    %s: {", lbl)
	f.indent++
	f.loopBody(s, "break "+lbl)
	f.indent--
	f.line("    }")
}

func (f *jfn) iteration(s *cc.IterationStatement) {
	switch s.Case {
	case cc.IterationStatementWhile:
		switch {
		case isConstFalse(s.ExpressionList):
			return // never run: Java refuses the body as unreachable
		case isConstTrue(s.ExpressionList):
			f.line("while (true) {")
		default:
			var c string
			f.indent++
			pre := f.capture(func() { c = f.condition(s.ExpressionList) })
			f.indent--
			if pre == "" {
				f.line("while (%s) {", c)
			} else {
				f.line("while (true) {")
				f.out.WriteString(pre)
				f.line("    if (%s) {", jnot(c))
				f.line("        break;")
				f.line("    }")
			}
		}
		f.loopBody(s.Statement, "continue")
		f.line("}")
	case cc.IterationStatementDo:
		c, pre := "", ""
		switch {
		case isConstFalse(s.ExpressionList):
			c = "false"
		case isConstTrue(s.ExpressionList):
			c = "true"
		default:
			f.indent++
			pre = f.capture(func() { c = f.condition(s.ExpressionList) })
			f.indent--
		}
		f.line("do {")
		if pre == "" {
			f.loopBody(s.Statement, "continue")
			f.line("} while (%s);", c)
			return
		}
		f.contBody(s.Statement)
		if jcomplete(s.Statement) || hasContinue(s.Statement) {
			f.out.WriteString(pre)
			f.line("} while (%s);", c)
		} else {
			f.line("} while (false);") // the condition is never reached
		}
	case cc.IterationStatementFor, cc.IterationStatementForDecl:
		cond, post := s.ExpressionList2, s.ExpressionList3
		if s.Case == cc.IterationStatementForDecl {
			f.declaration(s.Declaration, true)
			cond, post = s.ExpressionList, s.ExpressionList2
		} else if s.ExpressionList != nil {
			f.exprStmt(s.ExpressionList)
		}
		if cond != nil && isConstFalse(cond) {
			return
		}
		c, pre := "", ""
		f.indent++
		if cond != nil && !isConstTrue(cond) {
			pre = f.capture(func() { c = f.condition(cond) })
		}
		postText := ""
		if post != nil {
			postText = f.capture(func() { f.exprStmt(post) })
		}
		f.indent--
		update, simple := forUpdate(postText)
		if pre == "" && simple {
			if c == "" && update == "" {
				f.line("for (;;) {")
			} else {
				f.line("for (; %s; %s) {", c, update)
			}
			f.loopBody(s.Statement, "continue")
			f.line("}")
			return
		}
		f.line("for (;;) {")
		if c != "" {
			f.out.WriteString(pre)
			f.line("    if (%s) {", jnot(c))
			f.line("        break;")
			f.line("    }")
		}
		f.contBody(s.Statement)
		if jcomplete(s.Statement) || hasContinue(s.Statement) {
			f.out.WriteString(postText)
		}
		f.line("}")
	default:
		f.no(s, "a loop %v", s.Case)
	}
}

// forUpdate is a for's increment as Java's update list, when each of its
// statements is one statement expression.
func forUpdate(text string) (string, bool) {
	var parts []string
	for _, l := range strings.Split(strings.TrimSpace(text), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.ContainsAny(l, "{}") || !strings.HasSuffix(l, ";") {
			return "", false
		}
		parts = append(parts, strings.TrimSuffix(l, ";"))
	}
	return strings.Join(parts, ", "), true
}

func (f *jfn) jump(j *cc.JumpStatement) {
	switch j.Case {
	case cc.JumpStatementGoto:
		if !f.gotoOK[j] {
			f.no(j, "a goto that is no forward jump to a label of a block holding it: %s", j.Token2.SrcStr())
		}
		f.stmt1("break %s", gotoLabel(j.Token2.SrcStr())) // out of the labeled block that ends at the label
	case cc.JumpStatementBreak:
		f.stmt1("break")
	case cc.JumpStatementContinue:
		if len(f.cont) == 0 {
			f.no(j, "a continue outside a loop")
		}
		f.stmt1("%s", f.cont[len(f.cont)-1])
	case cc.JumpStatementReturn:
		if j.ExpressionList == nil {
			f.stmt1("return")
			return
		}
		if f.ret == "void" {
			f.exprStmt(j.ExpressionList)
			f.stmt1("return")
			return
		}
		rt := f.ft.Result()
		v := f.exprTo(j.ExpressionList, f.ret)
		if isAggr(rt) {
			f.stmt1("return %s.copy()", v.s) // a struct is returned by value
			return
		}
		f.stmt1("return %s", f.conv(v, f.ret, rt))
	default:
		f.no(j, "a jump %v", j.Case)
	}
}

// jcomplete is Java's "can complete normally" for the statement as this
// backend writes it (JLS 14.22).
func jcomplete(s *cc.Statement) bool {
	switch s.Case {
	case cc.StatementCompound:
		return jcompleteItems(s.CompoundStatement)
	case cc.StatementJump:
		return false
	case cc.StatementSelection:
		sel := s.SelectionStatement
		switch sel.Case {
		case cc.SelectionStatementIfElse:
			return jcomplete(sel.Statement) || jcomplete(sel.Statement2)
		case cc.SelectionStatementSwitch:
			return switchCompletes(sel)
		}
		return true
	case cc.StatementIteration:
		it := s.IterationStatement
		switch it.Case {
		case cc.IterationStatementWhile:
			if isConstTrue(it.ExpressionList) && !isConstFalse(it.ExpressionList) {
				return liveBreak(it.Statement)
			}
		case cc.IterationStatementDo:
			if isConstTrue(it.ExpressionList) {
				return liveBreak(it.Statement)
			}
			return jcomplete(it.Statement) || hasContinue(it.Statement) || liveBreak(it.Statement)
		case cc.IterationStatementFor, cc.IterationStatementForDecl:
			cond := it.ExpressionList2
			if it.Case == cc.IterationStatementForDecl {
				cond = it.ExpressionList
			}
			if cond == nil || isConstTrue(cond) {
				return liveBreak(it.Statement)
			}
		}
		return true
	case cc.StatementLabeled:
		return jcomplete(s.LabeledStatement.Statement)
	}
	return true
}

// jcompleteItems: the last statement completes, or a label after the last
// that cannot -- which a goto's block's break reaches.  Only a label an
// earlier item of the block jumps to is reached: that goto is the block's
// break (items).  A label nothing jumps to leaves the code after it as dead
// as C has it, and items writes none of it -- so it must not count as live
// here, or what follows is written where Java proves it unreachable.
func jcompleteItems(cs *cc.CompoundStatement) bool {
	var its []*cc.BlockItem
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		its = append(its, l.BlockItem)
	}
	live := true
	for k, it := range its {
		for _, lb := range itemLabels(it) {
			for i := 0; i < k && !live; i++ {
				if len(gotosTo(its[i], lb)) > 0 {
					live = true
				}
			}
		}
		if live && it.Case == cc.BlockItemStmt && !jcomplete(it.Statement) {
			live = false
		}
	}
	return live
}

// switchCompletes: the last group completes, or a break leaves the switch,
// or it has no default.
func switchCompletes(sel *cc.SelectionStatement) bool {
	if sel.Statement.Case != cc.StatementCompound {
		return true
	}
	live, def := false, false
	for l := sel.Statement.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
		labels, st := caseChain(l.BlockItem)
		for _, ls := range labels {
			live = true
			if ls.Case == cc.LabeledStatementDefault {
				def = true
			}
		}
		if st != nil && live {
			live = jcomplete(st)
		}
	}
	return live || !def || switchBreak(sel.Statement.CompoundStatement)
}

// liveBreak says s holds a break of the loop or switch it is the body of,
// that Java sees reachable.
func liveBreak(s *cc.Statement) bool {
	switch s.Case {
	case cc.StatementJump:
		return s.JumpStatement.Case == cc.JumpStatementBreak
	case cc.StatementCompound:
		live := true
		for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
			if it := l.BlockItem; it.Case == cc.BlockItemStmt {
				if len(itemLabels(it)) > 0 {
					live = true // a goto's label
				}
				if !live {
					continue
				}
				if liveBreak(it.Statement) {
					return true
				}
				if !jcomplete(it.Statement) {
					live = false
				}
			}
		}
	case cc.StatementSelection:
		sel := s.SelectionStatement
		switch sel.Case {
		case cc.SelectionStatementIf:
			return liveBreak(sel.Statement)
		case cc.SelectionStatementIfElse:
			return liveBreak(sel.Statement) || liveBreak(sel.Statement2)
		}
	case cc.StatementLabeled:
		return liveBreak(s.LabeledStatement.Statement)
	}
	return false
}

// switchBreak is liveBreak for a switch's body, whose labels make what
// follows them live again.
func switchBreak(cs *cc.CompoundStatement) bool {
	live := false
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		labels, st := caseChain(l.BlockItem)
		if len(labels) > 0 {
			live = true
		}
		if st == nil || !live {
			continue
		}
		if liveBreak(st) {
			return true
		}
		live = jcomplete(st)
	}
	return false
}
