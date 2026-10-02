package togo

// rs_fn.go is a C function printed as Rust (rs.go says the frame): an
// `unsafe fn` of the editor and the C's parameters, its statements C's own.
//
//   - every local is declared at the function's top, zeroed -- as C at -O0
//     keeps a local's value from one iteration of a loop to the next, and as
//     a goto's labeled block must not end a local's scope -- and given its
//     initializer where C declares it;
//   - if, while, do and for are Rust's if, while and loop; a for's step and a
//     do's condition run after the body, which a continue reaches by leaving
//     a labeled block around the body, `'cN: { ... break 'cN; ... }`;
//   - a break or a continue says its loop's label where a labeled block
//     stands between it and its loop, as Rust requires;
//   - a goto leaves a labeled block that ends at its label (the Java's rule,
//     java_stmt.go): every goto left in the core is a forward jump to a
//     label of a block holding it;
//   - a switch is a match when no case falls into the next, its arms each
//     case's statements; one that falls through is a ladder of labeled
//     blocks, the match at its heart breaking to the block its case's
//     statements follow, so that falling through is going on.
//
// What Rust's own analysis requires is kept: a statement after one that
// cannot complete (in Rust's terms) is not written, being dead, and a
// function whose end Rust can reach returns its type's zero there -- C's
// undefined value.

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// rfn is one function being printed.
type rfn struct {
	r     *rgen
	fd    *cc.FunctionDefinition
	name  string
	ft    *cc.FunctionType
	ret   cc.Type // the result's type, nil for void
	local map[*cc.Declarator]*rlocal
	order []*rlocal
	taken map[string]bool
	out   *strings.Builder
	ind   int
	ctl   []*rctl
	nlbl  int
	ntmp  int
	gotos map[*cc.JumpStatement]string // a goto -> the label of the block it leaves

	// the lowered form, where the function is printed from it (rs_lower.go)
	lowered_ bool
	sub      map[cc.ExpressionNode]lexpr
	vars     map[*lvar]*rlocal
	skip     cc.ExpressionNode // a node read itself, not its substitution

	global bool // init_globals: a compound literal is the editor's
}

// rlocal is a C local or parameter as a Rust variable.
type rlocal struct {
	name    string
	t       cc.Type
	d       *cc.Declarator
	param   bool
	mutated bool // assigned, or its address taken: `mut`
	read    bool
	temp    bool
	rty     string // a temporary's Rust type, when no C type says it
	unused  bool   // the body never names it
	sunk    bool   // declared where it is first given a value (sinkDecls)
	addr    bool   // its address is taken
}

// rctl is a construct a break or a continue may leave: a loop, a switch, or
// a labeled block.
type rctl struct {
	kind   int    // ctlLoop, ctlSwitch, ctlBlock
	label  string // its label, without the quote
	used   bool   // something said its label
	broken bool   // a break leaves it: it can complete
	cont   string // a loop's continue: the label of the block around its body ("" when it has none)
	match  bool   // a switch that is a match: its break at an arm's end is the arm's end
}

const (
	ctlLoop = iota
	ctlSwitch
	ctlBlock
)

// function prints one function definition, or says why it cannot.
func (r *rgen) function(fd *cc.FunctionDefinition) (src string, why string) {
	d := fd.Declarator
	defer func() {
		if e := recover(); e != nil {
			u, ok := e.(unsupported)
			if !ok {
				if os.Getenv("RS_DEBUG") != "" {
					panic(e)
				}
				why = fmt.Sprint("panic: ", e)
				return
			}
			src, why = "", u.why
		}
	}()
	ft, _ := d.Type().(*cc.FunctionType)
	if ft == nil {
		panic(unsupported{"a function of no function type"})
	}
	if ft.IsVariadic() {
		panic(unsupported{"a variadic function"})
	}
	f := &rfn{r: r, fd: fd, name: d.Name(), ft: ft, local: map[*cc.Declarator]*rlocal{}, taken: map[string]bool{},
		gotos: map[*cc.JumpStatement]string{}, out: &strings.Builder{}}
	if rt := ft.Result(); rt != nil && rt.Kind() != cc.Void {
		f.ret = rt
	}
	f.declareAll()
	for _, rb := range r.g.p.RuntimeBodies {
		if rb.Name == d.Name() && rb.Rs != nil {
			// a rule of the runtime's, not a translation (Profile.RuntimeBodies)
			body := r.layoutMarks(rb.Rs(r.ty(f.ret)))
			for _, l := range f.order {
				if l.param {
					l.read = true
				}
			}
			sig := r.signature(d, f, nil)
			return sig + " {\n" + indent(strings.TrimRight(body, "\n"), 4) + "\n}\n", ""
		}
	}
	f.ind = 1
	if why := f.structured(); why != "" {
		if !lowerable(why) {
			panic(unsupported{why})
		}
		// a goto back, or a case inside a statement: the lowered form
		for _, l := range f.order {
			l.mutated, l.read = l.param && l.mutated, l.param && l.read
		}
		f.out = &strings.Builder{}
		f.ntmp, f.nlbl, f.ctl = 0, 0, nil
		f.lowered()
		r.nLowered++
	}
	body := f.out.String()
	var b strings.Builder
	usesEd := rsUsesEd(body)
	code := rsStrRe.ReplaceAllString(body, `b""`)
	for _, l := range f.order {
		// what the body never names: a parameter `_`, a local not declared
		used := regexp.MustCompile(`\b` + regexp.QuoteMeta(l.name) + `\b`).MatchString(code)
		if !used {
			l.read, l.mutated, l.unused = false, false, true
		}
	}
	b.WriteString(r.signature(d, f, &usesEd) + " {\n")
	body = f.sinkDecls(body)
	for _, l := range f.order {
		if l.param || l.unused || l.sunk {
			continue
		}
		mut := ""
		if l.mutated {
			mut = "mut "
		}
		fmt.Fprintf(&b, "    let %s%s: %s = %s;\n", mut, l.name, l.declTy(r), l.zeroOf(r))
	}
	b.WriteString(body)
	b.WriteString("}\n")
	return b.String(), ""
}

// sinkDecls declares a local where the body first gives it a value, when
// that is a statement of the function's own block -- `let x: T = v;` for
// the top's `let mut x: T = 0;` and the body's `x = v;` -- `mut` only when
// the rest of the body writes it again.  A statement at the function's own
// level is inside no loop and no labeled block, so every path to a later
// use passes it, and the zero it replaces was never read.
func (f *rfn) sinkDecls(body string) string {
	lines := strings.Split(body, "\n")
	code := make([]string, len(lines))
	for i, l := range lines {
		code[i] = rsStrRe.ReplaceAllString(l, `b""`)
	}
	for _, l := range f.order {
		if l.param || l.unused {
			continue
		}
		// the local, not a member of the same name: `(*p).lnum` is no use of lnum
		word := regexp.MustCompile(`(^|[^.\w])` + regexp.QuoteMeta(l.name) + `\b`)
		first := -1
		for i, c := range code {
			if word.MatchString(c) {
				first = i
				break
			}
		}
		if first < 0 {
			continue
		}
		line := lines[first]
		prefix := "    " + l.name + " = "
		if !strings.HasPrefix(line, prefix) || strings.HasPrefix(line, "     ") || !strings.HasSuffix(line, ";") {
			continue
		}
		rhs := strings.TrimSuffix(strings.TrimPrefix(code[first], prefix), ";")
		if word.MatchString(rhs) || !rsBalanced(rhs) {
			continue
		}
		write := regexp.MustCompile(`(^|[^.\w*])` + regexp.QuoteMeta(l.name) + `(\.\w+|\[[^\]]*\])*\s(\+|-|\*|/|%|&|\||\^|<<|>>)?=[^=]|&raw mut ` + regexp.QuoteMeta(l.name) + `\b`)
		mut := ""
		for _, c := range code[first+1:] {
			if write.MatchString(c) {
				mut = "mut "
				break
			}
		}
		lines[first] = "    let " + mut + l.name + ": " + l.declTy(f.r) + " = " + strings.TrimPrefix(line, prefix)
		l.sunk = true
	}
	return strings.Join(lines, "\n")
}

// structured prints the function's body as C's statements, or says why it
// cannot.
func (f *rfn) structured() (why string) {
	defer func() {
		if e := recover(); e != nil {
			u, ok := e.(unsupported)
			if !ok {
				panic(e)
			}
			why = u.why
		}
	}()
	div := f.items(f.fd.CompoundStatement)
	if !div && f.ret != nil {
		f.line("return %s;", f.zero(f.ret))
	}
	return ""
}

var rsEdRe = regexp.MustCompile(`\bed\b`)
var rsStrRe = regexp.MustCompile(`b"(?:[^"\\]|\\.)*"`)

// rsUsesEd says a body names the editor (outside its strings).
func rsUsesEd(body string) bool {
	return rsEdRe.MatchString(rsStrRe.ReplaceAllString(body, `b""`))
}

// signature is a function's head: `pub unsafe fn name(ed: *mut Editor, p: T) -> R`.
// With f, the parameters are its variables, `mut` where assigned and `_`
// where never read; usesEd, when known, says whether the editor is named.
func (r *rgen) signature(d *cc.Declarator, f *rfn, usesEd *bool) string {
	ft, _ := d.Type().(*cc.FunctionType)
	ed := "ed"
	if usesEd != nil && !*usesEd {
		ed = "_ed"
	}
	ps := []string{ed + ": *mut Editor"}
	i := 0
	for _, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		t := p.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		name := fmt.Sprintf("p%d", i)
		if p.Name() != "" {
			name = rsName(p.Name())
		}
		if f != nil && p.Declarator != nil {
			if l := f.local[p.Declarator]; l != nil {
				name = l.name
				if l.unused {
					name = "_" + name
				}
				if l.mutated {
					name = "mut " + name
				}
			}
		}
		ps = append(ps, name+": "+r.declType(t))
		i++
	}
	res := ""
	if rt := ft.Result(); rt != nil && rt.Kind() != cc.Void {
		res = " -> " + r.declType(rt)
	}
	return fmt.Sprintf("pub unsafe fn %s(%s)%s", r.fnName[d.Name()], strings.Join(ps, ", "), res)
}

// declareAll names the parameters and every local, each a name of its own
// in the function: C's nested scopes are one Rust scope here.
func (f *rfn) declareAll() {
	for i, p := range f.ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void || p.Declarator == nil {
			continue
		}
		t := p.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		n := p.Name()
		if n == "" {
			n = fmt.Sprintf("p%d", i)
		}
		l := &rlocal{name: f.unique(rsName(n)), t: t, d: p.Declarator, param: true}
		f.local[p.Declarator] = l
		f.order = append(f.order, l)
	}
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if id, ok := n.(*cc.InitDeclarator); ok {
			d := id.Declarator
			if d != nil && !d.IsTypename() && d.Type() != nil && d.Type().Kind() != cc.Function && !d.IsExtern() &&
				d.StorageDuration() == cc.Automatic {
				if _, ok := f.local[d]; !ok {
					l := &rlocal{name: f.unique(rsName(d.Name())), t: d.Type(), d: d}
					f.local[d] = l
					f.order = append(f.order, l)
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(f.fd.CompoundStatement)
	// what is read and what is written, as the C says it
	var use func(n cc.Node)
	use = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
					if l := f.local[d]; l != nil {
						l.read = true
					}
				}
			}
		case *cc.AssignmentExpression:
			if x.Case == cc.AssignmentExpressionAssign {
				if l := f.identLocal(x.UnaryExpression); l != nil {
					l.mutated = true
					use(x.AssignmentExpression)
					return
				}
			} else if x.Case != cc.AssignmentExpressionCond {
				if l := f.identLocal(x.UnaryExpression); l != nil {
					l.mutated = true
				}
			}
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionInc, cc.UnaryExpressionDec:
				if l := f.identLocal(x.UnaryExpression); l != nil {
					l.mutated = true
				}
			case cc.UnaryExpressionAddrof:
				if l := f.identLocal(x.CastExpression); l != nil {
					l.mutated = true
					l.addr = true
				}
			}
		case *cc.PostfixExpression:
			switch x.Case {
			case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
				if l := f.identLocal(x.PostfixExpression); l != nil {
					l.mutated = true
				}
			}
		case *cc.InitDeclarator:
			if x.Initializer != nil {
				if l := f.local[x.Declarator]; l != nil {
					l.mutated = true // given its value where declared
				}
			}
		}
		walkChildrenFn(n, use)
	}
	use(f.fd.CompoundStatement)
	for _, l := range f.order {
		if !l.param && (isAggr(l.t) || l.t.Kind() == cc.Array) {
			l.mutated = true // its members and elements are written through it
		}
	}
}

// identLocal is the local an lvalue names, when it is one: through
// parentheses, a member of a struct local, or an element of an array local.
func (f *rfn) identLocal(e cc.ExpressionNode) *rlocal {
	for {
		e = unparenE(e)
		switch x := e.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
					return f.local[d]
				}
			}
			return nil
		case *cc.PostfixExpression:
			if x.Case == cc.PostfixExpressionSelect {
				e = x.PostfixExpression
				continue
			}
			if x.Case == cc.PostfixExpressionIndex && x.PostfixExpression.Type() != nil && x.PostfixExpression.Type().Kind() == cc.Array {
				e = x.PostfixExpression
				continue
			}
			return nil
		}
		return nil
	}
}

func (f *rfn) unique(base string) string {
	name := base
	for i := 2; f.taken[name] || f.r.valueTaken[name] || rsReserved[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	f.taken[name] = true
	return name
}

// temp is a new variable of the function, of Rust type ty, zeroed at its top.
func (f *rfn) temp(t cc.Type) string {
	for {
		f.ntmp++
		n := fmt.Sprintf("t%d", f.ntmp)
		if !f.taken[n] && !f.r.valueTaken[n] {
			f.taken[n] = true
			f.order = append(f.order, &rlocal{name: n, t: t, temp: true, mutated: true, read: true})
			return n
		}
	}
}

func (f *rfn) no(n cc.Node, format string, args ...any) {
	where := ""
	if n != nil {
		where = fmt.Sprintf(" at %d", n.Position().Line)
	}
	panic(unsupported{fmt.Sprintf(format, args...) + where})
}

// line writes a line at the current indentation.
func (f *rfn) line(format string, args ...any) {
	f.out.WriteString(strings.Repeat("    ", f.ind))
	fmt.Fprintf(f.out, format, args...)
	f.out.WriteString("\n")
}

// capture is what fn writes, one level in.
func (f *rfn) capture(fn func()) string {
	old := f.out
	b := &strings.Builder{}
	f.out = b
	f.ind++
	fn()
	f.ind--
	f.out = old
	return b.String()
}

// zero is a type's zero, as a Rust expression of it.
func (f *rfn) zero(t cc.Type) string { return f.r.zero(t) }

func (r *rgen) zero(t cc.Type) string {
	if t == nil {
		return "()"
	}
	switch t.Kind() {
	case cc.Ptr:
		if e := elemOf(t); e != nil && e.Kind() == cc.Function {
			return "None"
		}
		return "null_mut()"
	case cc.Function:
		return "None"
	case cc.Array, cc.Struct, cc.Union:
		return "core::mem::zeroed()"
	case cc.Bool:
		return "false"
	}
	return "0"
}

// --- statements -------------------------------------------------------------

// items writes a block's items, and none after one that cannot complete --
// until a label a goto reaches.  It reports whether the block cannot
// complete, as Rust sees it.
//
// A goto is a labeled block: the items from the first that holds a goto to
// L up to L's are wrapped in `'L: { ... }`, the goto is `break 'L;`, and
// the label's statement follows the block.  Two such blocks that would
// cross are made to nest by starting the later one where the earlier one
// starts.
func (f *rfn) items(cs *cc.CompoundStatement) bool {
	var its []*cc.BlockItem
	for l := cs.BlockItemList; l != nil; l = l.BlockItemList {
		its = append(its, l.BlockItem)
	}
	return f.itemList(its)
}

type rgblock struct {
	labels     []string
	start, end int // the block wraps its[start:end]; its[end] is the label's
	ctl        *rctl
}

func (f *rfn) itemList(its []*cc.BlockItem) bool {
	var blocks []*rgblock
	for k, it := range its {
		labels := itemLabels(it)
		if len(labels) == 0 {
			continue
		}
		start := -1
		var gs []*cc.JumpStatement
		for _, lb := range labels {
			for i := 0; i < k; i++ {
				g := gotosTo(its[i], lb)
				if len(g) > 0 && (start < 0 || i < start) {
					start = i
				}
				gs = append(gs, g...)
			}
		}
		if start >= 0 {
			b := &rgblock{labels: labels, start: start, end: k}
			f.nlbl++
			b.ctl = &rctl{kind: ctlBlock, label: "g_" + labels[0]}
			if f.labelTaken(b.ctl.label) {
				b.ctl.label = fmt.Sprintf("g%d_%s", f.nlbl, labels[0])
			}
			for _, g := range gs {
				f.gotos[g] = b.ctl.label
			}
			blocks = append(blocks, b)
		}
	}
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
	return f.itemRange(its, 0, len(its), blocks)
}

// labelTaken says a label of that name is open.
func (f *rfn) labelTaken(l string) bool {
	for _, c := range f.ctl {
		if c.label == l {
			return true
		}
	}
	return false
}

// itemRange writes its[from:to], opening the goto blocks that start there.
func (f *rfn) itemRange(its []*cc.BlockItem, from, to int, blocks []*rgblock) bool {
	div := false
	for i := from; i < to; i++ {
		// the outermost block starting here: it holds the rest that start here
		var open *rgblock
		for _, b := range blocks {
			if b.start == i && b.end <= to && (open == nil || b.end > open.end) {
				open = b
			}
		}
		if open != nil {
			var inner []*rgblock
			for _, b := range blocks {
				if b != open {
					inner = append(inner, b)
				}
			}
			f.ctl = append(f.ctl, open.ctl)
			var bdiv bool
			body := f.capture(func() { bdiv = f.itemRange(its, i, open.end, inner) })
			f.ctl = f.ctl[:len(f.ctl)-1]
			if open.ctl.used {
				f.r.nGoto++
				f.line("'%s: {", open.ctl.label)
				f.out.WriteString(body)
				f.line("}")
			} else {
				f.out.WriteString(indentBy(body, -1))
			}
			div = bdiv && !open.ctl.broken
			i = open.end - 1
			blocks = inner
			continue
		}
		if div {
			// dead, unless a goto's block ends here: then i is its label's,
			// and was reached above
			if it := its[i]; it.Case == cc.BlockItemDecl {
				continue
			}
			continue
		}
		it := its[i]
		switch it.Case {
		case cc.BlockItemDecl:
			f.declaration(it.Declaration)
		case cc.BlockItemStmt:
			if f.stmt(it.Statement) {
				div = true
			}
		default:
			f.no(it, "a block item %v", it.Case)
		}
	}
	return div
}

// indentBy moves every line of s by n levels.
func indentBy(s string, n int) string {
	if n >= 0 {
		return indent(s, 4*n)
	}
	pad := strings.Repeat("    ", -n)
	ls := strings.Split(s, "\n")
	for i, l := range ls {
		ls[i] = strings.TrimPrefix(l, pad)
	}
	return strings.Join(ls, "\n")
}

// declaration gives its locals their initial values where C declares them.
func (f *rfn) declaration(d *cc.Declaration) {
	if d.Case != cc.DeclarationDecl {
		return
	}
	for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		id := l.InitDeclarator
		dd := id.Declarator
		if dd == nil || id.Initializer == nil {
			continue
		}
		loc := f.local[dd]
		if loc == nil {
			continue // a static: the editor's, initialized with the file's objects
		}
		f.initLocal(loc, id.Initializer)
	}
}

// stmt writes a statement and reports whether it cannot complete.
func (f *rfn) stmt(s *cc.Statement) bool {
	if s == nil {
		return false
	}
	switch s.Case {
	case cc.StatementCompound:
		return f.items(s.CompoundStatement)
	case cc.StatementExpr:
		if s.ExpressionStatement.ExpressionList != nil {
			f.exprStmt(s.ExpressionStatement.ExpressionList)
		}
		return false
	case cc.StatementSelection:
		return f.selection(s.SelectionStatement)
	case cc.StatementIteration:
		return f.iteration(s.IterationStatement)
	case cc.StatementJump:
		return f.jump(s.JumpStatement)
	case cc.StatementLabeled:
		l := s.LabeledStatement
		if l.Case != cc.LabeledStatementLabel {
			f.no(s, "a case label inside a statement of its switch")
		}
		return f.stmt(l.Statement)
	}
	f.no(s, "a statement %v", s.Case)
	return false
}

// body writes a statement as the body of an if, a loop or an arm.
func (f *rfn) body(s *cc.Statement) (string, bool) {
	var div bool
	b := f.capture(func() { div = f.stmt(s) })
	return b, div
}

func (f *rfn) selection(s *cc.SelectionStatement) bool {
	switch s.Case {
	case cc.SelectionStatementIf, cc.SelectionStatementIfElse:
		c := f.cond(s.ExpressionList)
		then, tdiv := f.body(s.Statement)
		if s.Case == cc.SelectionStatementIf {
			f.line("if %s {", c)
			f.out.WriteString(then)
			f.line("}")
			return false
		}
		f.line("if %s {", c)
		f.out.WriteString(then)
		div := tdiv
		els := s.Statement2
		for {
			if els.Case == cc.StatementSelection && (els.SelectionStatement.Case == cc.SelectionStatementIf || els.SelectionStatement.Case == cc.SelectionStatementIfElse) {
				// else if: written on, and its own pieces
				in := els.SelectionStatement
				c := f.cond(in.ExpressionList)
				b, d := f.body(in.Statement)
				f.line("} else if %s {", c)
				f.out.WriteString(b)
				if in.Case == cc.SelectionStatementIf {
					f.line("}")
					return false
				}
				div = div && d
				els = in.Statement2
				continue
			}
			b, d := f.body(els)
			f.line("} else {")
			f.out.WriteString(b)
			f.line("}")
			return div && d
		}
	case cc.SelectionStatementSwitch:
		return f.switchStmt(s)
	}
	f.no(s, "a selection %v", s.Case)
	return false
}

// cond is a condition as a Rust bool, its outer parentheses dropped.
func (f *rfn) cond(e cc.ExpressionNode) string {
	return unparenRs(f.truth(f.expr(e)))
}

// loop pushes a loop's construct and writes its body, returning the body,
// whether it cannot complete, and the construct.
func (f *rfn) loopBody(s *cc.Statement, contBlock bool) (string, bool, *rctl) {
	f.nlbl++
	c := &rctl{kind: ctlLoop, label: fmt.Sprintf("l%d", f.nlbl)}
	f.ctl = append(f.ctl, c)
	var body string
	var div bool
	if contBlock && rsHasContinue(s) {
		// the body in a block a continue leaves: what follows it runs
		cb := &rctl{kind: ctlBlock, label: fmt.Sprintf("c%d", f.nlbl)}
		c.cont = cb.label
		f.ctl = append(f.ctl, cb)
		inner, idiv := f.body(s)
		f.ctl = f.ctl[:len(f.ctl)-1]
		if cb.used {
			body = f.capture(func() {
				f.line("'%s: {", cb.label)
				f.out.WriteString(indentBy(inner, 1))
				f.line("}")
			})
			div = idiv && !cb.broken
		} else {
			body = indentBy(inner, 1)
			div = idiv
		}
		body = indentBy(body, -1)
	} else {
		body, div = f.body(s)
	}
	f.ctl = f.ctl[:len(f.ctl)-1]
	return body, div, c
}

// rsHasContinue says s has a continue of its own loop: not one inside a
// loop of its own.
func rsHasContinue(s cc.Node) bool {
	found := false
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil || found {
			return
		}
		switch x := n.(type) {
		case *cc.IterationStatement:
			return
		case *cc.JumpStatement:
			if x.Case == cc.JumpStatementContinue {
				found = true
			}
			return
		}
		walkChildrenFn(n, rec)
	}
	walkChildrenFn(s, rec)
	if st, ok := s.(*cc.Statement); ok && st.Case == cc.StatementJump && st.JumpStatement.Case == cc.JumpStatementContinue {
		return true
	}
	return found
}

func (f *rfn) loopHead(c *rctl, head string) {
	if c.used {
		f.line("'%s: %s {", c.label, head)
	} else {
		f.line("%s {", head)
	}
}

func (f *rfn) iteration(s *cc.IterationStatement) bool {
	switch s.Case {
	case cc.IterationStatementWhile:
		if isConstTrue(s.ExpressionList) && !isConstFalse(s.ExpressionList) {
			body, _, c := f.loopBody(s.Statement, false)
			f.loopHead(c, "loop")
			f.out.WriteString(body)
			f.line("}")
			return !c.broken
		}
		cond := f.cond(s.ExpressionList)
		body, _, c := f.loopBody(s.Statement, false)
		f.loopHead(c, "while "+cond)
		f.out.WriteString(body)
		f.line("}")
		return false
	case cc.IterationStatementDo:
		if isConstFalse(s.ExpressionList) {
			// do { ... } while (0): a block a break or a continue leaves
			f.nlbl++
			c := &rctl{kind: ctlLoop, label: fmt.Sprintf("l%d", f.nlbl), cont: "self"}
			f.ctl = append(f.ctl, c)
			body, div := f.body(s.Statement)
			f.ctl = f.ctl[:len(f.ctl)-1]
			if c.used {
				f.line("'%s: {", c.label)
				f.out.WriteString(body)
				f.line("}")
			} else {
				f.out.WriteString(indentBy(body, -1))
			}
			return div && !c.broken
		}
		body, bdiv, c := f.loopBody(s.Statement, true)
		f.loopHead(c, "loop")
		f.out.WriteString(body)
		if !bdiv {
			f.ind++
			if !isConstTrue(s.ExpressionList) {
				f.line("if !(%s) {", f.cond(s.ExpressionList))
				f.line("    break;")
				f.line("}")
				c.broken = true
			}
			f.ind--
		}
		f.line("}")
		return !c.broken
	case cc.IterationStatementFor, cc.IterationStatementForDecl:
		var cond, step cc.ExpressionNode
		if s.Case == cc.IterationStatementFor {
			if s.ExpressionList != nil {
				f.exprStmt(s.ExpressionList)
			}
			cond, step = s.ExpressionList2, s.ExpressionList3
		} else {
			f.declaration(s.Declaration)
			cond, step = s.ExpressionList, s.ExpressionList2
		}
		forever := cond == nil || isConstTrue(cond) && !isConstFalse(cond)
		head := "loop"
		if !forever {
			head = "while " + f.cond(cond)
		}
		body, bdiv, c := f.loopBody(s.Statement, step != nil)
		f.loopHead(c, head)
		f.out.WriteString(body)
		if step != nil && !bdiv {
			f.ind++
			f.exprStmt(step)
			f.ind--
		}
		f.line("}")
		return forever && !c.broken
	}
	f.no(s, "an iteration %v", s.Case)
	return false
}

// target finds the construct a break (brk) or a continue leaves, and says
// whether a labeled block stands between: then the jump says its label.
func (f *rfn) target(brk bool) (*rctl, bool) {
	between := false
	for i := len(f.ctl) - 1; i >= 0; i-- {
		c := f.ctl[i]
		switch c.kind {
		case ctlLoop:
			return c, between
		case ctlSwitch:
			if brk {
				return c, between
			}
			between = true // a continue leaves the switch's own block
		case ctlBlock:
			between = true
		}
	}
	return nil, false
}

func (f *rfn) jump(j *cc.JumpStatement) bool {
	switch j.Case {
	case cc.JumpStatementReturn:
		if j.ExpressionList == nil {
			if f.ret != nil {
				f.line("return %s;", f.zero(f.ret))
				return true
			}
			f.line("return;")
			return true
		}
		if f.ret == nil {
			// a void function's return of a void expression
			f.exprStmt(j.ExpressionList)
			f.line("return;")
			return true
		}
		v := f.conv(f.expr(j.ExpressionList), f.r.ty(f.ret))
		f.line("return %s;", unparenRs(v.s))
		return true
	case cc.JumpStatementBreak:
		c, between := f.target(true)
		if c == nil {
			f.no(j, "a break out of nothing")
		}
		c.broken = true
		if c.kind == ctlSwitch || between || c.cont == "self" {
			c.used = true
			f.line("break '%s;", c.label)
			return true
		}
		f.line("break;")
		return true
	case cc.JumpStatementContinue:
		c, between := f.target(false)
		if c == nil {
			f.no(j, "a continue out of nothing")
		}
		switch {
		case c.cont == "self":
			// do { } while (0): its condition is false, so out
			c.broken = true
			c.used = true
			f.line("break '%s;", c.label)
		case c.cont != "":
			for _, x := range f.ctl {
				if x.label == c.cont {
					x.used = true
					x.broken = true
				}
			}
			f.line("break '%s;", c.cont)
		case between:
			c.used = true
			f.line("continue '%s;", c.label)
		default:
			f.line("continue;")
		}
		return true
	case cc.JumpStatementGoto:
		l, ok := f.gotos[j]
		if !ok {
			f.no(j, "a goto that is no forward jump to a label of a block holding it: %s", j.Token2.SrcStr())
		}
		for _, x := range f.ctl {
			if x.label == l {
				x.used = true
				x.broken = true
			}
		}
		f.line("break '%s;", l)
		return true
	}
	f.no(j, "a jump %v", j.Case)
	return false
}

// --- switch -------------------------------------------------------------

// rcase is a group of a switch's items: its case labels and its statements.
type rcase struct {
	vals  []int64  // the values that come here
	pats  []string // each value as a pattern: its constant's name where C names one
	def   bool     // default comes here
	items []*cc.BlockItem
}

func (f *rfn) switchStmt(s *cc.SelectionStatement) bool {
	x := f.expr(s.ExpressionList)
	k, ok := scalarKind(s.ExpressionList.Type())
	if !ok {
		f.no(s, "a switch on %v", s.ExpressionList.Type())
	}
	k = promote(k)
	ty := rsKind(k)
	xs := unparenRs(f.conv(x, ty).s)
	if s.Statement.Case != cc.StatementCompound {
		f.no(s, "a switch whose body is no block")
	}
	var groups []*rcase
	var cur *rcase
	for l := s.Statement.CompoundStatement.BlockItemList; l != nil; l = l.BlockItemList {
		it := l.BlockItem
		labels, st := caseChain(it)
		if len(labels) > 0 {
			cur = &rcase{}
			groups = append(groups, cur)
			for _, lb := range labels {
				switch lb.Case {
				case cc.LabeledStatementCaseLabel:
					v, ok := intValue(lb.ConstantExpression.Value())
					if !ok {
						f.no(lb, "a case of no constant value")
					}
					v = truncK(v, k)
					cur.vals = append(cur.vals, v)
					cur.pats = append(cur.pats, f.casePattern(lb.ConstantExpression, v, ty))
				case cc.LabeledStatementDefault:
					cur.def = true
				default:
					f.no(lb, "a case range")
				}
			}
			cur.items = append(cur.items, &cc.BlockItem{Case: cc.BlockItemStmt, Statement: st})
			continue
		}
		if cur == nil {
			continue // before the first case: never reached, a declaration's initializer included
		}
		cur.items = append(cur.items, it)
	}
	// the runs: a case's statements and those of the cases it falls into
	var runs [][]*rcase
	for i, g := range groups {
		if i > 0 && rsItemsComplete(groups[i-1].items) {
			runs[len(runs)-1] = append(runs[len(runs)-1], g)
			continue
		}
		runs = append(runs, []*rcase{g})
	}
	f.nlbl++
	c := &rctl{kind: ctlSwitch, label: fmt.Sprintf("s%d", f.nlbl), match: true}
	if len(runs) == len(groups) {
		f.r.nMatch++
	} else {
		f.r.nLadder++
	}
	return f.matchSwitch(xs, ty, runs, c)
}

// casePattern is a case's value as a pattern of ty: the enumerator's name
// where C names one of that type, else the number -- a character's with
// the character beside it.
func (f *rfn) casePattern(e cc.ExpressionNode, v int64, ty string) string {
	if p, ok := unparenE(e).(*cc.PrimaryExpression); ok {
		switch p.Case {
		case cc.PrimaryExpressionIdent:
			if en, ok := p.ResolvedTo().(*cc.Enumerator); ok {
				if c := f.r.consts[en.Token.SrcStr()]; c != nil && c.ty == ty && c.v == v {
					return c.name
				}
			}
		case cc.PrimaryExpressionChar:
			if v >= 32 && v < 127 && v != '*' && v != '/' {
				return fmt.Sprintf("%s /* '%c' */", rsLit(v, ty), rune(v))
			}
		}
	}
	return rsLit(v, ty)
}

// rsItemsComplete says a case's items may go on into the next: the last
// that is reached completes.
func rsItemsComplete(its []*cc.BlockItem) bool {
	cs := &cc.CompoundStatement{}
	var list *cc.BlockItemList
	for i := len(its) - 1; i >= 0; i-- {
		list = &cc.BlockItemList{BlockItem: its[i], BlockItemList: list}
	}
	cs.BlockItemList = list
	return jcompleteItems(cs)
}

// rsPattern is a case group's values as an or-pattern.
func rsPattern(vals []int64, ty string) string {
	var ps []string
	for _, v := range vals {
		ps = append(ps, rsLit(v, ty))
	}
	return strings.Join(ps, " | ")
}

// matchSwitch is a switch as a match, an arm a run of cases: a case's
// statements, and those of the cases it falls into.  A run of one case is
// its statements, the break that ends them dropped; a longer one is a
// ladder of labeled blocks, a match on the run's own value at its heart
// breaking to the block whose end its case's statements follow, so that
// falling through is going on:
//
//	v @ (1 | 2) => 'r1: { 'r0: { match v { 1 => break 'r0, _ => break 'r1 } }
//	                      case 1's statements }
//	               case 2's statements
func (f *rfn) matchSwitch(x, ty string, runs [][]*rcase, c *rctl) bool {
	f.ctl = append(f.ctl, c)
	var arms []string
	div := true
	hasDef := false
	for ri, run := range runs {
		var pats []string
		def := false
		for _, g := range run {
			pats = append(pats, g.pats...)
			def = def || g.def
		}
		last := run[len(run)-1]
		its := last.items
		// the break that ends the run is the arm's end
		if n := len(its); n > 0 && rsIsBreak(its[n-1]) {
			its = its[:n-1]
			c.broken = true
		}
		var adiv bool
		var body string
		bind := ""
		if len(run) == 1 {
			body = f.capture(func() {
				f.ind++
				adiv = f.itemList(its)
				f.ind--
			})
		} else {
			bind = fmt.Sprintf("v%d_%d", f.nlbl, ri)
			body = f.capture(func() {
				f.ind++
				adiv = f.ladderRun(bind, ty, run, its)
				f.ind--
			})
		}
		if len(its) < len(last.items) {
			adiv = false
		}
		div = div && adiv
		pat := strings.Join(pats, " | ")
		switch {
		case def:
			pat = "_"
			hasDef = true
			if bind != "" {
				pat = bind
			}
		case bind != "" && len(pats) > 1:
			pat = bind + " @ (" + pat + ")"
		case bind != "":
			pat = bind + " @ " + pat
		}
		if strings.TrimSpace(body) == "" {
			arms = append(arms, fmt.Sprintf("%s => {}", pat))
		} else {
			arms = append(arms, fmt.Sprintf("%s => {\n%s%s}", pat, body, strings.Repeat("    ", f.ind+1)))
		}
	}
	f.ctl = f.ctl[:len(f.ctl)-1]
	if !hasDef {
		arms = append(arms, "_ => {}")
		div = false
	}
	// the default's arm last, where Rust wants a catch-all: C's default
	// takes what no case does, wherever it stands
	for i, a := range arms {
		if i < len(arms)-1 && rsCatchAll(a) {
			arms = append(append(arms[:i:i], arms[i+1:]...), a)
			break
		}
	}
	if c.used {
		f.line("'%s: {", c.label)
		f.ind++
	}
	f.line("match %s {", x)
	for _, a := range arms {
		f.line("    %s", a)
	}
	f.line("}")
	if c.used {
		f.ind--
		f.line("}")
	}
	return div && !c.broken
}

// rsCatchAll says an arm's pattern takes every value: `_`, or a run's
// binding alone.
func rsCatchAll(arm string) bool {
	p := strings.SplitN(arm, " => ", 2)[0]
	return p == "_" || rsBindRe.MatchString(p)
}

var rsBindRe = regexp.MustCompile(`^v\d+_\d+$`)

// ladderRun is a run of cases one of which falls into the next, its value
// bound to v: the cases' statements one after the other, each after the
// labeled block the match breaks to for its values, the last's items its.
func (f *rfn) ladderRun(v, ty string, run []*rcase, lastItems []*cc.BlockItem) bool {
	labels := make([]string, len(run))
	for i := range run {
		labels[i] = fmt.Sprintf("%s_%d", v, i)
	}
	var arms []string
	def := ""
	for i, g := range run {
		if len(g.pats) > 0 {
			arms = append(arms, fmt.Sprintf("%s => break '%s,", strings.Join(g.pats, " | "), labels[i]))
		}
		if g.def {
			def = labels[i]
		}
	}
	if def == "" {
		// every value the arm takes is a case's: the last's catches the rest
		arms[len(arms)-1] = "_" + arms[len(arms)-1][strings.Index(arms[len(arms)-1], " => "):]
	} else {
		arms = append(arms, fmt.Sprintf("_ => break '%s,", def))
	}
	for i := len(run) - 1; i >= 0; i-- {
		f.line("'%s: {", labels[i])
	}
	f.ind++
	f.line("match %s {", v)
	for _, a := range arms {
		f.line("    %s", a)
	}
	f.line("}")
	f.ind--
	div := false
	for i, g := range run {
		f.line("}")
		its := g.items
		if i == len(run)-1 {
			its = lastItems
		}
		div = f.itemList(its)
	}
	return div
}

// rsIsBreak says an item is a break.
func rsIsBreak(it *cc.BlockItem) bool {
	return it.Case == cc.BlockItemStmt && it.Statement.Case == cc.StatementJump && it.Statement.JumpStatement.Case == cc.JumpStatementBreak
}
