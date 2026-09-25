package togo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// the names every function body sees besides its own: the runtime's and Go's,
// and the size type (Profile.SizeType)
var runtimeNames = strings.Fields(`Ptr Mk View Addr S Alloc Realloc Memmove Memset Zero Memcmp B2i GaData GaGrowTo GoString
	int int8 int16 int32 int64 uint16 uint32 uint64 byte bool any nil new len cap true false`)

// condition is e as a Go bool, what it does written before it.
func (f *fnEmit) condition(e cc.ExpressionNode) string {
	return f.truth(f.expr(e))
}

func (f *fnEmit) stmt(s *cc.Statement) {
	if s == nil {
		return
	}
	switch s.Case {
	case cc.StatementCompound:
		f.block(s.CompoundStatement)
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
			f.no(s, "a case label outside its switch's body")
		}
		name := l.Token.SrcStr()
		if f.gotos[name] {
			f.indent--
			f.line("%s:", name)
			f.indent++
		}
		f.stmt(l.Statement)
	default:
		f.no(s, "a statement %v", s.Case)
	}
}

func (f *fnEmit) block(cs *cc.CompoundStatement) {
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		it := l.BlockItem
		switch it.Case {
		case cc.BlockItemDecl:
			f.declaration(it.Declaration)
		case cc.BlockItemStmt:
			f.stmt(it.Statement)
		default:
			f.no(it, "a block item %v", it.Case)
		}
	}
}

// declaration hoists a block's locals to the function's top (Go's goto may
// not jump over a declaration) and writes their initializers where they were.
func (f *fnEmit) declaration(d *cc.Declaration) {
	if d.Case != cc.DeclarationDecl {
		return
	}
	for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		id := l.InitDeclarator
		dd := id.Declarator
		t := dd.Type()
		if dd.IsTypename() || t.Kind() == cc.Function || dd.IsExtern() {
			continue
		}
		if dd.StorageDuration() == cc.Static {
			continue // a hoisted global, initialized there
		}
		key := f.g.a.declKey(dd)
		typ := f.objType(t, key)
		lc := f.declare(dd, typ)
		// WHERE C DECLARES IT, in a function with no goto, unless it is a case's
		// own local (C's case is no scope and Go's is) or an uninitialised
		// local in a loop: C at -O0 keeps its value from one iteration to the
		// next, and a Go declaration in the body would zero it each time.
		if !f.hoistAll && !f.inCase && (id.Initializer != nil || !f.inLoop()) {
			lc.scoped = true
			if id.Initializer != nil && id.Initializer.Case == cc.InitializerExpr && t.Kind() != cc.Array {
				// `x := e` when e is of x's Go type and not an untyped
				// constant; `var x T = e` when it is not.
				v := f.exprTo(id.Initializer.AssignmentExpression, typ)
				s := f.conv(v, typ)
				if !v.konst && !v.null && v.t == typ && s == v.s {
					f.line("%s := %s\x01%s", lc.name, s, lc.name)
				} else {
					f.line("var %s %s = %s\x01%s", lc.name, typ, s, lc.name)
				}
				continue
			}
			f.line("var %s %s\x01%s", lc.name, typ, lc.name)
		}
		if at, ok := t.(*cc.ArrayType); ok && strings.HasPrefix(typ, "Ptr[") {
			f.line("%s = Mk[%s](%d)", lc.name, elemOfGo(typ), at.Len())
		}
		if id.Initializer == nil {
			continue
		}
		switch id.Initializer.Case {
		case cc.InitializerExpr:
			if t.Kind() == cc.Array {
				f.no(dd, "an array initialized from an expression")
			}
			v := f.exprTo(id.Initializer.AssignmentExpression, typ)
			f.line("%s = %s", lc.name, f.conv(v, typ))
		case cc.InitializerInitList:
			switch {
			case zeroInit(id.Initializer):
				if !strings.HasPrefix(typ, "Ptr[") && !lc.scoped { // a scoped var is zero already
					f.line("%s = %s{}", lc.name, typ)
				}
			case strings.HasPrefix(typ, "Ptr["):
				at, ok := t.(*cc.ArrayType)
				if !ok {
					f.no(dd, "a braced initializer for a pointer")
				}
				et := elemOfGo(typ)
				for i, in := range listItems(f, id.Initializer) {
					if zeroInit(in) {
						continue
					}
					f.line("%s.Set(%d, %s)", lc.name, i, f.initValue(et, at.Elem(), "elem:"+key, in))
				}
			default:
				f.line("%s = %s", lc.name, f.initValue(typ, t, key, id.Initializer))
			}
		}
	}
}

// zeroInit says an initializer is all zeros and nulls: {0}, {}, {0, nullptr}.
func zeroInit(in *cc.Initializer) bool {
	if in.Case == cc.InitializerExpr {
		return isZeroConst(in.AssignmentExpression) || isNullConst(in.AssignmentExpression)
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		if l.Designation != nil || !zeroInit(l.Initializer) {
			return false
		}
	}
	return true
}

// listItems are a braced initializer's elements, in order; a designator
// stops the function.
func listItems(f *fnEmit, in *cc.Initializer) []*cc.Initializer {
	var r []*cc.Initializer
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		if l.Designation != nil {
			f.no(in, "a designated initializer")
		}
		r = append(r, l.Initializer)
	}
	return r
}

// designator is an element's one designator, [index] or .field, or nil.
func designator(f *fnEmit, l *cc.InitializerList) *cc.Designator {
	if l.Designation == nil {
		return nil
	}
	dl := l.Designation.DesignatorList
	if dl.DesignatorList != nil {
		f.no(l, "a designator of more than one step")
	}
	return dl.Designator
}

// initValue is an initializer as a Go value of type typ.
func (f *fnEmit) initValue(typ string, t cc.Type, key string, in *cc.Initializer) string {
	if in.Case == cc.InitializerExpr {
		v := f.exprTo(in.AssignmentExpression, typ)
		return f.conv(v, typ)
	}
	if zeroInit(in) {
		return typ + "{}"
	}
	var parts []string
	composite := false // an array of composite elements: one to a line
	switch x := t.(type) {
	case *cc.StructType:
		i := 0
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			if d := designator(f, l); d != nil {
				if d.Case != cc.DesignatorField && d.Case != cc.DesignatorField2 {
					f.no(in, "an index designator in a struct")
				}
				name := d.Token2.SrcStr()
				if d.Case == cc.DesignatorField2 {
					name = d.Token.SrcStr()
				}
				i = -1
				for k := 0; k < x.NumFields(); k++ {
					if x.FieldByIndex(k).Name() == name {
						i = k
					}
				}
				if i < 0 {
					f.no(in, "no field %s", name)
				}
			}
			fl := x.FieldByIndex(i)
			if fl == nil {
				f.no(in, "more initializers than fields")
			}
			ft := f.g.goType(fl.Type(), fieldKey(fl))
			parts = append(parts, f.g.goName(fl.Name())+": "+f.initValue(ft, fl.Type(), fieldKey(fl), l.Initializer))
			i++
		}
	case *cc.ArrayType:
		if !strings.HasPrefix(typ, "[") {
			f.no(in, "an array initializer for a %s", typ)
		}
		et := elemOfGo(typ)
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			v := f.initValue(et, x.Elem(), "elem:"+key, l.Initializer)
			// An element's type is the array's: `{...}`, not `T{...}` (gofmt -s).
			if strings.HasPrefix(v, et+"{") {
				v = v[len(et):]
				composite = true
			}
			if d := designator(f, l); d != nil {
				if d.Case != cc.DesignatorIndex {
					f.no(in, "a field designator in an array")
				}
				v = f.expr(d.ConstantExpression).s + ": " + v
			}
			parts = append(parts, v)
		}
	default:
		items := listItems(f, in)
		if len(items) == 1 {
			return f.initValue(typ, t, key, items[0])
		}
		f.no(in, "a braced initializer for a %s", t)
	}
	if composite && len(parts) > 1 {
		return typ + "{\n" + strings.Join(parts, ",\n") + ",\n}"
	}
	return typ + "{" + strings.Join(parts, ", ") + "}"
}

func (f *fnEmit) selection(s *cc.SelectionStatement) {
	switch s.Case {
	case cc.SelectionStatementIf, cc.SelectionStatementIfElse:
		c := f.condition(s.ExpressionList)
		f.line("if %s {", c)
		f.body(s.Statement)
		for s.Case == cc.SelectionStatementIfElse {
			el := s.Statement2
			if el.Case == cc.StatementSelection && el.SelectionStatement.Case != cc.SelectionStatementSwitch {
				var c2 string
				nl, nt := len(f.locals), f.tmp
				pre := f.capture(func() { c2 = f.condition(el.SelectionStatement.ExpressionList) })
				if pre != "" {
					// not an else-if: what the condition made is made again below
					for _, l := range f.locals[nl:] {
						delete(f.taken, l.name)
					}
					f.locals, f.tmp = f.locals[:nl], nt
				}
				if pre == "" {
					f.line("} else if %s {", c2)
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

func (f *fnEmit) body(s *cc.Statement) {
	f.indent++
	f.stmt(s)
	f.indent--
}

// switchStmt: Go's cases do not fall through; a C case that runs on into the
// next ends with fallthrough.  A trailing break is Go's end of a case.
func (f *fnEmit) switchStmt(s *cc.SelectionStatement) {
	v := f.expr(s.ExpressionList)
	sv := v.s
	if v.boolean {
		sv = "B2i(" + v.s + ")"
	}
	f.line("switch %s {", sv)
	if s.Statement.Case != cc.StatementCompound {
		f.no(s, "a switch whose body is not a block")
	}
	type group struct {
		labels []string
		stmts  []*cc.BlockItem
	}
	var groups []*group
	for l := s.Statement.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
		it := l.BlockItem
		if it.Case == cc.BlockItemStmt && it.Statement.Case == cc.StatementLabeled {
			st := it.Statement
			var labels []string
			for st.Case == cc.StatementLabeled && st.LabeledStatement.Case != cc.LabeledStatementLabel {
				ls := st.LabeledStatement
				switch ls.Case {
				case cc.LabeledStatementCaseLabel:
					cv := f.expr(ls.ConstantExpression)
					labels = append(labels, cv.s)
				case cc.LabeledStatementDefault:
					labels = append(labels, "")
				default:
					f.no(ls, "a case range")
				}
				st = ls.Statement
			}
			if len(labels) > 0 {
				if len(groups) > 0 && len(groups[len(groups)-1].stmts) == 0 {
					groups[len(groups)-1].labels = append(groups[len(groups)-1].labels, labels...)
				} else {
					groups = append(groups, &group{labels: labels})
				}
				groups[len(groups)-1].stmts = append(groups[len(groups)-1].stmts, &cc.BlockItem{Case: cc.BlockItemStmt, Statement: st})
				continue
			}
		}
		if len(groups) == 0 {
			if it.Case == cc.BlockItemDecl {
				f.declaration(it.Declaration)
				continue
			}
			f.no(it, "a statement before a switch's first case")
		}
		groups[len(groups)-1].stmts = append(groups[len(groups)-1].stmts, it)
	}
	for gi, g := range groups {
		var cases []string
		def := false
		for _, l := range g.labels {
			if l == "" {
				def = true
			} else {
				cases = append(cases, l)
			}
		}
		switch {
		case def && len(cases) > 0:
			// Go has no case A, default: the values fall through into default
			f.line("case %s:", strings.Join(cases, ", "))
			f.line("\tfallthrough")
			f.line("default:")
		case def:
			f.line("default:")
		default:
			f.line("case %s:", strings.Join(cases, ", "))
		}
		f.indent++
		stmts := g.stmts
		trailingBreak := false
		if n := len(stmts); n > 0 {
			last := stmts[n-1]
			if last.Case == cc.BlockItemStmt && last.Statement.Case == cc.StatementJump && last.Statement.JumpStatement.Case == cc.JumpStatementBreak {
				stmts = stmts[:n-1]
				trailingBreak = true
			}
		}
		if trailingBreak {
			// `if (c) { ...; break; } break;`: the inner break is the case's end
			// either way, and in Go, which does not fall through, it says nothing.
			f.tailBreaks(stmts)
		} else if n := len(stmts); n > 0 && stmts[n-1].Case == cc.BlockItemStmt && stmts[n-1].Statement.Case == cc.StatementCompound {
			// `case X: { ...; break; }`: the break is the block's end and the
			// case's, and endsInJump already keeps the fallthrough out.
			f.blockBreak(stmts[n-1].Statement)
		}
		f.pushBreakable(false)
		for _, it := range stmts {
			switch it.Case {
			case cc.BlockItemDecl:
				f.inCase = true
				f.declaration(it.Declaration)
				f.inCase = false
			case cc.BlockItemStmt:
				f.stmt(it.Statement)
			default:
				f.no(it, "a block item %v", it.Case)
			}
		}
		f.popBreakable()
		if !trailingBreak && gi < len(groups)-1 && !endsInJump(stmts) {
			f.line("fallthrough")
		}
		f.indent--
	}
	f.line("}")
}

func (f *fnEmit) pushBreakable(loop bool) { f.brk = append(f.brk, loop) }
func (f *fnEmit) popBreakable()           { f.brk = f.brk[:len(f.brk)-1] }

func endsInJump(items []*cc.BlockItem) bool {
	if len(items) == 0 {
		return false
	}
	last := items[len(items)-1]
	return last.Case == cc.BlockItemStmt && jumpsStmt(last.Statement)
}

func jumpsStmt(s *cc.Statement) bool {
	switch s.Case {
	case cc.StatementJump:
		return true
	case cc.StatementCompound:
		var items []*cc.BlockItem
		for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
			items = append(items, l.BlockItem)
		}
		return endsInJump(items)
	}
	return false
}

// hasContinue says a loop body has a continue for this loop.
func hasContinue(n cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		switch x := n.(type) {
		case *cc.IterationStatement:
			return // its continues are its own
		case *cc.JumpStatement:
			if x.Case == cc.JumpStatementContinue {
				found = true
			}
		}
		walkChildrenFn(n, rec)
	}
	walkChildrenFn(n, rec)
	return found
}

func (f *fnEmit) newLabel(base string) string {
	for {
		f.tmp++
		n := fmt.Sprintf("%s%d", base, f.tmp)
		if !f.taken[n] {
			f.taken[n] = true
			return n
		}
	}
}

func (f *fnEmit) loopBody(s *cc.Statement, cont string) {
	f.cont = append(f.cont, cont)
	f.pushBreakable(true)
	f.body(s)
	f.popBreakable()
	f.cont = f.cont[:len(f.cont)-1]
}

func isConstTrue(e cc.ExpressionNode) bool {
	if e == nil {
		return true
	}
	if hasEffect(e) {
		return false
	}
	switch v := e.Value().(type) {
	case cc.Int64Value:
		return v != 0
	case cc.UInt64Value:
		return v != 0
	}
	return false
}

func (f *fnEmit) iteration(s *cc.IterationStatement) {
	switch s.Case {
	case cc.IterationStatementWhile:
		if isConstTrue(s.ExpressionList) {
			f.line("for {")
			f.loopBody(s.Statement, "")
			f.line("}")
			return
		}
		var c string
		f.indent++
		pre := f.capture(func() { c = f.condition(s.ExpressionList) })
		f.indent--
		if pre == "" {
			f.line("for %s {", c)
		} else {
			f.line("for {")
			f.out.WriteString(pre)
			f.line("\tif %s {", not(c))
			f.line("\t\tbreak")
			f.line("\t}")
		}
		f.loopBody(s.Statement, "")
		f.line("}")
	case cc.IterationStatementDo:
		cont := ""
		if hasContinue(s.Statement) {
			cont = f.newLabel("cont")
		}
		f.line("for {")
		f.loopBody(s.Statement, cont)
		if cont != "" {
			f.line("%s:", cont)
		}
		f.indent++
		if !isConstTrue(s.ExpressionList) {
			c := f.condition(s.ExpressionList)
			f.line("if %s {", not(c))
			f.line("\tbreak")
			f.line("}")
		}
		f.indent--
		f.line("}")
	case cc.IterationStatementFor, cc.IterationStatementForDecl:
		if s.Case == cc.IterationStatementForDecl {
			f.declaration(s.Declaration)
		} else if s.ExpressionList != nil {
			f.exprStmt(s.ExpressionList)
		}
		cond, post := s.ExpressionList2, s.ExpressionList3
		if s.Case == cc.IterationStatementForDecl {
			cond, post = s.ExpressionList, s.ExpressionList2
		}
		var c string
		f.indent++
		pre := ""
		if !isConstTrue(cond) {
			pre = f.capture(func() { c = f.condition(cond) })
		}
		postText := ""
		if post != nil {
			postText = f.capture(func() { f.exprStmt(post) })
		}
		f.indent--
		postLine := strings.TrimSpace(postText)
		simplePost := !strings.Contains(postLine, "\n") && !strings.HasPrefix(postLine, "_ =") && !strings.HasPrefix(postLine, "if ")
		if pre == "" && simplePost {
			switch {
			case c == "" && postLine == "":
				f.line("for {")
			case postLine == "":
				f.line("for %s {", c)
			default:
				f.line("for ; %s; %s {", c, postLine)
			}
			f.loopBody(s.Statement, "")
			f.line("}")
			return
		}
		cont := ""
		if postText != "" && hasContinue(s.Statement) {
			cont = f.newLabel("cont")
		}
		f.line("for {")
		if c != "" {
			f.out.WriteString(pre)
			f.line("\tif %s {", not(c))
			f.line("\t\tbreak")
			f.line("\t}")
		}
		f.loopBody(s.Statement, cont)
		if cont != "" {
			f.line("%s:", cont)
		}
		f.out.WriteString(postText)
		f.line("}")
	default:
		f.no(s, "a loop %v", s.Case)
	}
}

func (f *fnEmit) jump(j *cc.JumpStatement) {
	switch j.Case {
	case cc.JumpStatementGoto:
		f.line("goto %s", j.Token2.SrcStr())
	case cc.JumpStatementBreak:
		if f.deadBrk[j] {
			return
		}
		f.line("break")
	case cc.JumpStatementContinue:
		if len(f.cont) == 0 {
			f.no(j, "a continue outside a loop")
		}
		if c := f.cont[len(f.cont)-1]; c != "" {
			f.line("goto %s", c)
		} else {
			f.line("continue")
		}
	case cc.JumpStatementReturn:
		rt := f.g.goType(f.ft.Result(), "ret:"+f.name)
		if j.ExpressionList == nil || rt == "" {
			if j.ExpressionList != nil {
				f.exprStmt(j.ExpressionList)
			}
			f.line("return")
			return
		}
		v := f.exprTo(j.ExpressionList, rt)
		f.line("return %s", f.conv(v, rt))
	default:
		f.no(j, "a jump %v", j.Case)
	}
}

// terminates says Go sees the statements end the function.
func terminates(lines []string) bool {
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}
		return strings.HasPrefix(l, "return") || strings.HasPrefix(l, "goto ") || strings.HasPrefix(l, "panic(")
	}
	return false
}

// emitFunction writes one function, or says why it cannot.
func (g *gen) emitFunction(fd *cc.FunctionDefinition) (src string, why string) {
	d := fd.Declarator
	ft, _ := d.Type().(*cc.FunctionType)
	f := &fnEmit{g: g, name: d.Name(), ft: ft, byDecl: map[*cc.Declarator]*local{}, taken: map[string]bool{}, gotos: map[string]bool{}}
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
	for _, n := range runtimeNames {
		f.taken[n] = true
	}
	f.taken[g.sizeType()] = true
	// every name the body refers to that is not its own
	walkChildrenFn(fd.CompoundStatement, func(n cc.Node) {})
	var names func(cc.Node)
	names = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				switch d := x.ResolvedTo().(type) {
				case *cc.Declarator:
					if !d.IsParam() && d.StorageDuration() != cc.Automatic {
						f.taken[g.goName(x.Token.SrcStr())] = true
					}
				case *cc.Enumerator:
					f.taken[g.goName(x.Token.SrcStr())] = true
				}
			}
		case *cc.JumpStatement:
			if x.Case == cc.JumpStatementGoto {
				f.gotos[x.Token2.SrcStr()] = true
			}
		}
		walkChildrenFn(n, names)
	}
	names(fd.CompoundStatement)
	// parameters
	var ps []string
	for i, p := range ft.Parameters() {
		if p.Type() != nil && p.Type().Kind() == cc.Void {
			continue
		}
		pn := p.Name()
		if pn == "" {
			pn = fmt.Sprintf("p%d", i)
		}
		pk := fmt.Sprintf("param:%s:%d", d.Name(), i)
		ps = append(ps, g.goName(pn)+" "+g.goType(p.Type(), pk))
		f.taken[g.goName(pn)] = true
	}
	if ft.IsVariadic() {
		f.no(d, "a variadic function")
	}
	for l := range f.gotos {
		f.taken[l] = true
	}
	f.hoistAll = len(f.gotos) > 0
	var body strings.Builder
	f.out = &body
	f.indent = 1
	f.block(fd.CompoundStatement)
	rt := g.goType(ft.Result(), "ret:"+d.Name())
	var b strings.Builder
	fmt.Fprintf(&b, "func %s(%s)", g.goName(d.Name()), strings.Join(ps, ", "))
	if rt != "" {
		b.WriteString(" " + rt)
	}
	b.WriteString(" {\n")
	hoisted := 0
	for _, l := range f.locals {
		if !l.scoped {
			fmt.Fprintf(&b, "\tvar %s %s\n", l.name, l.typ)
			hoisted++
		}
	}
	for _, l := range f.locals {
		if !l.read && !l.scoped {
			fmt.Fprintf(&b, "\t_ = %s\n", l.name)
		}
	}
	if hoisted > 0 {
		b.WriteString("\n")
	}
	bodyText := scopedDecls(body.String(), f.locals)
	b.WriteString(bodyText)
	if rt != "" && !terminates(strings.Split(bodyText, "\n")) && !goTerminates(bodyText) {
		b.WriteString("\tpanic(\"not reached\")\n")
	}
	b.WriteString("}\n")
	return b.String(), ""
}

// writeBodies writes bodies.go, every function the emitter writes whole, and
// bodies.txt, every one it does not, with why.
func (g *gen) writeBodies(dir string, skip map[string]bool) error {
	body, report := g.bodies(skip)
	if err := os.WriteFile(filepath.Join(dir, "bodies.go"), []byte("// Code generated by `go tool whim gen` from editor.c: the functions' bodies.\n\n"+g.p.pkg()+body), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "bodies.txt"), []byte(report), 0o644)
}

// bodies is every function the emitter writes whole, and a report of every
// one it does not, with why.
func (g *gen) bodies(skip map[string]bool) (string, string) {
	var out, report strings.Builder
	n, total := 0, 0
	whys := map[string]int{}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef {
			continue
		}
		fd := ed.FunctionDefinition
		name := fd.Declarator.Name()
		if skip[name] {
			continue
		}
		total++
		src, why := g.emitFunction(fd)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", name, why)
			k := why
			if i := strings.Index(k, " at "); i > 0 {
				k = k[:i]
			}
			if len(k) > 60 {
				k = k[:60]
			}
			whys[k]++
			continue
		}
		n++
		out.WriteString(src + "\n")
	}
	var ks []string
	for k := range whys {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return whys[ks[i]] > whys[ks[j]] })
	fmt.Fprintf(logw, "skel: %d of %d functions written\n", n, total)
	for i, k := range ks {
		if i == 25 {
			break
		}
		fmt.Fprintf(logw, "  %5d  %s\n", whys[k], k)
	}
	return out.String(), report.String()
}

// tailBreaks marks the breaks that end the branches of the if statements a
// case's statements end with, when the case itself ends in a break.
func (f *fnEmit) tailBreaks(items []*cc.BlockItem) {
	if len(items) == 0 {
		return
	}
	last := items[len(items)-1]
	if last.Case != cc.BlockItemStmt {
		return
	}
	var branch func(s *cc.Statement)
	branch = func(s *cc.Statement) {
		if s == nil {
			return
		}
		switch s.Case {
		case cc.StatementJump:
			if s.JumpStatement.Case == cc.JumpStatementBreak {
				if f.deadBrk == nil {
					f.deadBrk = map[*cc.JumpStatement]bool{}
				}
				f.deadBrk[s.JumpStatement] = true
			}
		case cc.StatementCompound:
			var its []*cc.BlockItem
			for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
				its = append(its, l.BlockItem)
			}
			if len(its) > 0 && its[len(its)-1].Case == cc.BlockItemStmt {
				branch(its[len(its)-1].Statement)
			}
		case cc.StatementSelection:
			sel := s.SelectionStatement
			switch sel.Case {
			case cc.SelectionStatementIf:
				branch(sel.Statement)
			case cc.SelectionStatementIfElse:
				branch(sel.Statement)
				branch(sel.Statement2)
			}
		}
	}
	if s := last.Statement; s.Case == cc.StatementSelection {
		branch(s)
	}
}

// blockBreak marks the break a compound statement ends with, through nested
// compounds.
func (f *fnEmit) blockBreak(s *cc.Statement) {
	for s != nil && s.Case == cc.StatementCompound {
		var last *cc.BlockItem
		for l := s.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
			last = l.BlockItem
		}
		if last == nil || last.Case != cc.BlockItemStmt {
			return
		}
		s = last.Statement
	}
	if s != nil && s.Case == cc.StatementJump && s.JumpStatement.Case == cc.JumpStatementBreak {
		if f.deadBrk == nil {
			f.deadBrk = map[*cc.JumpStatement]bool{}
		}
		f.deadBrk[s.JumpStatement] = true
	}
}

// inLoop says emission is inside a loop body.
func (f *fnEmit) inLoop() bool {
	for _, loop := range f.brk {
		if loop {
			return true
		}
	}
	return false
}

// forMerge is a scoped `var i int` followed by the loop that counts with it.
var forMerge = regexp.MustCompile(`(?m)^(\s*)var (\w+) int\n\s*for (\w+) = `)

// declMerge is a scoped `var x T` followed by x's first assignment.
var declMerge = regexp.MustCompile(`(?m)^(\s*)var (\w+) ([^\n=]+)\n\s*(\w+) = `)

// scopedDecls finishes the declarations emitted where C declares them: a
// local nothing reads gets `_ = x` after its declaration (Go refuses an
// unused variable), and a declaration followed by the local's first
// assignment becomes one statement, `var x T = e`.
func scopedDecls(body string, locals []*local) string {
	for _, l := range locals {
		if !l.scoped {
			continue
		}
		mark := "\x01" + l.name
		if l.read {
			body = strings.Replace(body, mark, "", 1)
		} else {
			i := strings.Index(body, mark)
			if i < 0 {
				continue
			}
			ls := strings.LastIndex(body[:i], "\n") + 1
			ind := body[ls : ls+len(body[ls:])-len(strings.TrimLeft(body[ls:], "\t"))]
			body = body[:i] + "\n" + ind + "_ = " + l.name + body[i+len(mark):]
		}
	}
	body = forMerge.ReplaceAllStringFunc(body, func(m string) string {
		s := forMerge.FindStringSubmatch(m)
		if s[2] != s[3] {
			return m
		}
		return s[1] + "for " + s[2] + " := "
	})
	return declMerge.ReplaceAllStringFunc(body, func(m string) string {
		s := declMerge.FindStringSubmatch(m)
		if s[2] != s[4] {
			return m
		}
		return s[1] + "var " + s[2] + " " + s[3] + " = "
	})
}
