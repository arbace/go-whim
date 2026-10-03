package clisp

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cemit"
)

// ToC prints s-expressions as C, in crefactor/cemit's canonical spelling: the
// same lines, the same indentation, the same spacing, so that ToC(ToLisp(x))
// is cemit.Canonical(x).  It is a printer of its own, written case for case
// after cemit's, and not a tree handed back to cemit: a form set that had to
// be turned into the front end's tree first would be a second parser.
func ToC(src []byte) ([]byte, error) {
	forms, err := Read(src)
	if err != nil {
		return nil, err
	}
	return Print(forms)
}

// Print is ToC of forms already read.
func Print(forms []*Node) ([]byte, error) {
	p := &printer{}
	for i, f := range forms {
		if i > 0 && !(f.Is("include") && forms[i-1].Is("include")) {
			p.w("\n")
		}
		p.top(f)
		if p.err != nil {
			return nil, p.err
		}
	}
	return []byte(p.b.String()), nil
}

// Indent is cemit's indent.
const Indent = "    "

type printer struct {
	b      strings.Builder
	indent int
	err    error
}

func (p *printer) fail(n *Node, format string, a ...any) {
	if p.err != nil {
		return
	}
	at := ""
	if n != nil {
		s := n.String()
		if len(s) > 120 {
			s = s[:120] + "..."
		}
		at = " in " + s
	}
	p.err = fmt.Errorf("clisp: %s%s", fmt.Sprintf(format, a...), at)
}

func (p *printer) w(s string) {
	if p.err == nil {
		p.b.WriteString(s)
	}
}

func (p *printer) pad() string { return strings.Repeat(Indent, p.indent) }

func (p *printer) line(s string) {
	if s == "" {
		p.w("\n")
		return
	}
	p.w(p.pad() + s + "\n")
}

func (p *printer) outdented(s string) {
	if p.indent > 0 {
		p.indent--
		p.line(s)
		p.indent++
		return
	}
	p.line(s)
}

// arg is a form's i-th argument, or a failure naming the form.
func (p *printer) arg(n *Node, i int) *Node {
	if a := n.Args(); i < len(a) {
		return a[i]
	}
	p.fail(n, "%s has no argument %d", n.Head(), i+1)
	return A("")
}

// unquote is the text a quoted atom holds (quote's inverse).
func (p *printer) unquote(n *Node) string {
	s := n.Atom
	if n.list || len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		p.fail(n, "a quoted text was expected")
		return ""
	}
	s = s[1 : len(s)-1]
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == 'n' {
				b.WriteByte('\n')
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func join(parts ...string) string {
	out := parts[:0:0]
	for _, s := range parts {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

// ---- top level and block items

func (p *printer) top(f *Node) {
	switch f.Head() {
	case "include":
		p.w("#include " + p.unquote(p.arg(f, 0)) + "\n")
	case "directive":
		p.w(p.unquote(p.arg(f, 0)) + "\n")
	case "defn":
		p.funcDef(f)
	default:
		if !declHeads[f.Head()] {
			p.fail(f, "not a top-level form")
			return
		}
		p.line(p.declLine(f))
	}
}

// declHeads are the forms that are a declaration's line.
var declHeads = map[string]bool{
	"def": true, "typedef": true, "declare": true, "struct": true, "union": true, "enum": true,
	"static_assert": true, "macro-decl": true, "verbatim": true,
}

func (p *printer) item(f *Node) {
	if !f.list {
		p.line(p.expr(f, lvComma) + ";")
		return
	}
	if declHeads[f.Head()] {
		p.line(p.declLine(f))
		return
	}
	switch f.Head() {
	case "block":
		p.compound(f.Args())
	case "label":
		p.outdented(p.arg(f, 0).Atom + ":")
	case "case":
		p.outdented("case " + p.expr(p.arg(f, 0), lvCond) + ":")
	case "case-range":
		p.outdented("case " + p.expr(p.arg(f, 0), lvCond) + " ... " + p.expr(p.arg(f, 1), lvCond) + ":")
	case "default":
		p.outdented("default:")
	case "empty":
		p.line(";")
	case "attributed":
		args := f.Args()
		var attrs []string
		var e *Node
		for i, a := range args {
			if i == len(args)-1 && !a.Is("attr") && !a.Is("attr-text") {
				e = a
				break
			}
			attrs = append(attrs, p.attr(a))
		}
		s := join(attrs...)
		if e == nil {
			p.line(s + ";")
			return
		}
		p.line(join(s, p.expr(e, lvComma)) + ";")
	case "if":
		p.line("if (" + p.expr(p.arg(f, 0), lvComma) + ")")
		p.block(p.arg(f, 1))
		if len(f.Args()) > 2 {
			p.otherwise(p.arg(f, 2))
		}
	case "switch":
		p.line("switch (" + p.expr(p.arg(f, 0), lvComma) + ")")
		p.block(p.arg(f, 1))
	case "while":
		p.line("while (" + p.expr(p.arg(f, 0), lvComma) + ")")
		p.block(p.arg(f, 1))
	case "do":
		p.line("do")
		p.block(p.arg(f, 0))
		p.line("while (" + p.expr(p.arg(f, 1), lvComma) + ");")
	case "for":
		init := p.arg(f, 0)
		var head string
		if declHeads[init.Head()] {
			head = p.declLine(init)
		} else {
			head = p.clause(init) + ";"
		}
		cond, step := p.clause(p.arg(f, 1)), p.clause(p.arg(f, 2))
		p.line("for (" + head + space(cond) + ";" + space(step) + ")")
		p.block(p.arg(f, 3))
	case "return":
		if len(f.Args()) == 0 {
			p.line("return;")
			return
		}
		p.line("return " + p.expr(p.arg(f, 0), lvComma) + ";")
	case "break":
		p.line("break;")
	case "continue":
		p.line("continue;")
	case "goto":
		p.line("goto " + p.arg(f, 0).Atom + ";")
	case "goto*":
		p.line("goto *" + p.expr(p.arg(f, 0), lvComma) + ";")
	default:
		p.line(p.expr(f, lvComma) + ";")
	}
}

// clause is a for-statement's clause: `()` is none.
func (p *printer) clause(n *Node) string {
	if n.list && len(n.List) == 0 {
		return ""
	}
	return p.expr(n, lvComma)
}

func space(s string) string {
	if s == "" {
		return ""
	}
	return " " + s
}

func (p *printer) compound(items []*Node) {
	p.line("{")
	p.indent++
	for _, it := range items {
		p.item(it)
	}
	p.indent--
	p.line("}")
}

// block is a statement in a place cemit braces: a block as itself, anything
// else braced.
func (p *printer) block(n *Node) {
	if n.Is("block") {
		p.compound(n.Args())
		return
	}
	p.line("{")
	p.indent++
	p.item(n)
	p.indent--
	p.line("}")
}

func (p *printer) otherwise(n *Node) {
	if n.Is("if") {
		p.line("else if (" + p.expr(p.arg(n, 0), lvComma) + ")")
		p.block(p.arg(n, 1))
		if len(n.Args()) > 2 {
			p.otherwise(p.arg(n, 2))
		}
		return
	}
	p.line("else")
	p.block(n)
}

// ---- declarations

// A decl is a def's parts: its prefix, name, type, the attributes and asm
// label after the declarator, and the rest -- a value, or a body.
type decl struct {
	prefix []*Node
	name   string
	typ    *Node
	attrs  []*Node
	asm    *Node
	rest   []*Node
}

func isAttr(n *Node) bool { return n.Is("attr") || n.Is("attr-text") }

func (p *printer) parseDef(f *Node) decl {
	var d decl
	args := f.Args()
	i := 0
	if f.Is("typedef") {
		d.prefix = []*Node{A("typedef")}
	}
	for i < len(args) && ((!args[i].list && prefixWords[args[i].Atom]) || isAttr(args[i])) {
		d.prefix = append(d.prefix, args[i])
		i++
	}
	if i+1 >= len(args) || args[i].list {
		p.fail(f, "a name and a type were expected")
		return d
	}
	d.name, d.typ = args[i].Atom, args[i+1]
	i += 2
	for i < len(args) && isAttr(args[i]) {
		d.attrs = append(d.attrs, args[i])
		i++
	}
	if i < len(args) && args[i].Is("asm-label") {
		d.asm = args[i]
		i++
	}
	d.rest = args[i:]
	return d
}

func (p *printer) declLine(f *Node) string {
	switch f.Head() {
	case "macro-decl", "verbatim":
		return p.unquote(p.arg(f, 0))
	case "static_assert":
		return p.staticAssert(f)
	case "struct", "union", "enum":
		return p.spec(f) + ";"
	case "declare":
		return p.specs(f.Args()) + ";"
	}
	d := p.parseDef(f)
	items, dc := p.build(d.typ, dcl{direct: d.name})
	s := join(p.specs(d.prefix), p.specs(items), dc.text())
	if len(d.attrs) > 0 {
		s = join(s, p.attrList(d.attrs))
	}
	if d.asm != nil {
		s = join(s, p.unquote(p.arg(d.asm, 0)))
	}
	switch len(d.rest) {
	case 0:
	case 1:
		s += " " + p.assign(d.rest[0])
	default:
		p.fail(f, "more than one value")
	}
	return s + ";"
}

func (p *printer) staticAssert(f *Node) string {
	s := "static_assert(" + p.expr(p.arg(f, 0), lvCond)
	if len(f.Args()) > 1 {
		s += ", " + p.arg(f, 1).Atom
	}
	return s + ");"
}

func (p *printer) funcDef(f *Node) {
	d := p.parseDef(f)
	items, dc := p.build(d.typ, dcl{direct: d.name})
	if len(d.attrs) > 0 || d.asm != nil {
		p.fail(f, "an attribute after a definition's declarator")
	}
	if head := join(p.specs(d.prefix), p.specs(items), dc.ptr); head != "" {
		p.w(Indent + head + "\n")
	}
	p.w(dc.direct + "\n")
	p.compound(d.rest)
}

// ---- specifiers

func (p *printer) specs(items []*Node) string {
	var parts []string
	for _, it := range items {
		parts = append(parts, p.spec(it))
	}
	return join(parts...)
}

func (p *printer) spec(n *Node) string {
	if !n.list {
		return n.Atom
	}
	switch n.Head() {
	case "struct", "union":
		return p.structOrUnion(n)
	case "enum":
		return p.enum(n)
	case "typeof":
		return "typeof(" + p.expr(p.arg(n, 0), lvComma) + ")"
	case "typeof-type":
		return "typeof(" + p.typeName(p.arg(n, 0)) + ")"
	case "atomic":
		return "_Atomic(" + p.typeName(p.arg(n, 0)) + ")"
	case "alignas":
		return "alignas(" + p.expr(p.arg(n, 0), lvCond) + ")"
	case "alignas-type":
		return "alignas(" + p.typeName(p.arg(n, 0)) + ")"
	case "attr", "attr-text":
		return p.attr(n)
	}
	p.fail(n, "not a specifier")
	return ""
}

// baseItems is the specifiers a type's base holds.
func baseItems(t *Node) []*Node {
	if !t.list {
		return []*Node{t}
	}
	switch t.Head() {
	case "spec":
		return t.Args()
	case "struct", "union", "enum", "typeof", "typeof-type", "atomic", "alignas", "alignas-type", "attr", "attr-text":
		return []*Node{t}
	}
	return t.List
}

func (p *printer) attrList(as []*Node) string {
	var parts []string
	for _, a := range as {
		parts = append(parts, p.attr(a))
	}
	return join(parts...)
}

// attr is an attribute specifier's text.
func (p *printer) attr(n *Node) string {
	if n.Is("attr-text") {
		return p.unquote(p.arg(n, 0))
	}
	if !n.Is("attr") {
		p.fail(n, "not an attribute")
		return ""
	}
	return attrText(n)
}

// attrText writes `(attr a (b x y))` as `__attribute__((a, b(x, y)))`.
func attrText(n *Node) string {
	var vs []string
	for _, v := range n.Args() {
		if !v.list {
			vs = append(vs, v.Atom)
			continue
		}
		var args []string
		for _, a := range v.Args() {
			args = append(args, a.Atom)
		}
		vs = append(vs, v.Head()+"("+strings.Join(args, ", ")+")")
	}
	return "__attribute__((" + strings.Join(vs, ", ") + "))"
}

func (p *printer) structOrUnion(n *Node) string {
	kw := n.Head()
	args := n.Args()
	tag := ""
	if len(args) > 0 && !args[0].list && args[0].Atom != "{}" {
		tag = args[0].Atom
		args = args[1:]
	}
	if len(args) == 0 {
		return join(kw, tag)
	}
	if len(args) == 1 && !args[0].list && args[0].Atom == "{}" {
		args = nil
	}
	var b strings.Builder
	b.WriteString(join(kw, tag) + "\n" + p.pad() + "{\n")
	p.indent++
	for _, m := range args {
		b.WriteString(p.pad() + p.member(m) + "\n")
	}
	p.indent--
	b.WriteString(p.pad() + "}")
	return b.String()
}

func (p *printer) member(m *Node) string {
	if m.Is("static_assert") {
		return p.staticAssert(m)
	}
	args := m.List
	if !m.list || len(args) == 0 {
		p.fail(m, "not a member")
		return ""
	}
	if len(args) == 1 {
		return p.specs(baseItems(args[0])) + ";"
	}
	if args[1].Is("bits") {
		return join(p.specs(baseItems(args[0]))) + " : " + p.expr(p.arg(args[1], 0), lvCond) + ";"
	}
	items, dc := p.build(args[1], dcl{direct: args[0].Atom})
	s := join(p.specs(items), dc.text())
	if len(args) > 2 && args[2].Is("bits") {
		s += " : " + p.expr(p.arg(args[2], 0), lvCond)
	}
	return s + ";"
}

func (p *printer) enum(n *Node) string {
	args := n.Args()
	tag, under := "", ""
	if len(args) > 0 && !args[0].list {
		tag = args[0].Atom
		args = args[1:]
	}
	if len(args) > 0 && args[0].Is(":") {
		under = " : " + p.specs(args[0].Args())
		args = args[1:]
	}
	head := join("enum", tag) + under
	if len(args) == 0 {
		return head
	}
	if len(args) == 1 {
		return head + " { " + p.enumerator(args[0]) + " }"
	}
	var b strings.Builder
	b.WriteString(head + "\n" + p.pad() + "{\n")
	p.indent++
	for _, e := range args {
		b.WriteString(p.pad() + p.enumerator(e) + ",\n")
	}
	p.indent--
	b.WriteString(p.pad() + "}")
	return b.String()
}

func (p *printer) enumerator(e *Node) string {
	if !e.list || len(e.List) == 0 || e.List[0].list {
		p.fail(e, "not an enumerator")
		return ""
	}
	if len(e.List) == 1 {
		return e.List[0].Atom
	}
	return e.List[0].Atom + " = " + p.expr(e.List[1], lvCond)
}

// ---- declarators

// A dcl is a declarator being built outward from its name: the pointers
// before it and the direct declarator, as C's grammar has them.
type dcl struct{ ptr, direct string }

func (d dcl) text() string { return d.ptr + d.direct }

// parenthesised is d made a direct declarator: a pointer in it is
// parenthesised before an array or a function can follow.
func (d dcl) parenthesised() dcl {
	if d.ptr == "" {
		return d
	}
	return dcl{direct: "(" + d.ptr + d.direct + ")"}
}

// build writes a type around a declarator, from the outside of the type in --
// which is from the name out -- and returns the specifiers left at its base.
func (p *printer) build(t *Node, d dcl) ([]*Node, dcl) {
	for p.err == nil {
		args := t.Args()
		switch t.Head() {
		case "ptr":
			if len(args) == 0 {
				p.fail(t, "ptr of nothing")
				return nil, d
			}
			var qs []string
			for _, q := range args[1:] {
				qs = append(qs, q.Atom)
			}
			q := strings.Join(qs, " ")
			if q != "" {
				q += " "
			}
			d.ptr = "*" + q + d.ptr
			t = args[0]
		case "array":
			d = d.parenthesised()
			switch len(args) {
			case 1:
				d.direct += "[]"
				t = args[0]
			case 2:
				d.direct += "[" + p.expr(args[0], lvAssign) + "]"
				t = args[1]
			default:
				p.fail(t, "an array is (array T) or (array N T)")
				return nil, d
			}
		case "fn":
			if len(args) != 2 {
				p.fail(t, "a function is (fn PARAMS RESULT)")
				return nil, d
			}
			d = d.parenthesised()
			d.direct += "(" + p.params(args[0]) + ")"
			t = args[1]
		case "paren":
			d = dcl{direct: "(" + d.ptr + d.direct + ")"}
			t = p.arg(t, 0)
		default:
			return baseItems(t), d
		}
	}
	return nil, d
}

func (p *printer) params(l *Node) string {
	if !l.list {
		p.fail(l, "a parameter list was expected")
		return ""
	}
	var parts []string
	for _, x := range l.List {
		if !x.list {
			parts = append(parts, x.Atom)
			continue
		}
		if len(x.List) >= 2 && !isAttr(x.List[1]) {
			items, dc := p.build(x.List[1], dcl{direct: x.List[0].Atom})
			parts = append(parts, join(p.specs(items), dc.text(), p.attrList(x.List[2:])))
			continue
		}
		if len(x.List) == 0 {
			p.fail(l, "an empty parameter")
			return ""
		}
		items, dc := p.build(x.List[0], dcl{})
		parts = append(parts, join(p.specs(items), dc.text(), p.attrList(x.List[1:])))
	}
	return strings.Join(parts, ", ")
}

func (p *printer) typeName(t *Node) string {
	items, dc := p.build(t, dcl{})
	return join(p.specs(items), dc.text())
}

// ---- initializers

func (p *printer) assign(n *Node) string {
	s := p.initializer(n)
	if strings.HasPrefix(s, "\n") {
		return "=" + s
	}
	return "= " + s
}

func (p *printer) initializer(n *Node) string {
	if n.Is("init") {
		return p.braced(n.Args())
	}
	return p.expr(n, lvAssign)
}

func (p *printer) braced(items []*Node) string {
	var b strings.Builder
	b.WriteString("\n" + p.pad() + "{\n")
	p.indent++
	for _, it := range items {
		b.WriteString(p.pad() + p.inlineInit(it) + ",\n")
	}
	p.indent--
	b.WriteString(p.pad() + "}")
	return b.String()
}

func (p *printer) inlineInit(n *Node) string {
	switch n.Head() {
	case "at":
		args := n.Args()
		if len(args) < 2 {
			p.fail(n, "a designation is (at D... VALUE)")
			return ""
		}
		v := p.inlineInit(args[len(args)-1])
		var b strings.Builder
		for _, d := range args[:len(args)-1] {
			switch {
			case !d.list:
				b.WriteString(d.Atom)
			case d.Is("idx") && len(d.Args()) == 1:
				b.WriteString("[" + p.expr(p.arg(d, 0), lvCond) + "]")
			case d.Is("idx") && len(d.Args()) == 2:
				b.WriteString("[" + p.expr(p.arg(d, 0), lvCond) + " ... " + p.expr(p.arg(d, 1), lvCond) + "]")
			default:
				p.fail(d, "not a designator")
			}
		}
		return b.String() + " = " + v
	case "init":
		return "{" + p.initList(n.Args()) + "}"
	}
	return p.expr(n, lvAssign)
}

func (p *printer) initList(items []*Node) string {
	var parts []string
	for _, it := range items {
		parts = append(parts, p.inlineInit(it))
	}
	return strings.Join(parts, ", ")
}

// ---- expressions

// expr writes an expression where its place asks for level want,
// parenthesised when its form is below it.
func (p *printer) expr(n *Node, want int) string {
	s := p.form(n)
	if level(n) < want {
		return "(" + s + ")"
	}
	return s
}

func (p *printer) form(n *Node) string {
	if !n.list {
		return n.Atom
	}
	h := n.Head()
	args := n.Args()
	if lv, ok := binaryLevel[h]; ok && len(args) >= 2 {
		if lv == lvAssign {
			return p.expr(args[0], lvUnary) + " " + h + " " + p.expr(p.arg(n, 1), lvAssign)
		}
		s := p.expr(args[0], lv)
		for _, a := range args[1:] {
			s += " " + h + " " + p.expr(a, lv+1)
		}
		return s
	}
	switch h {
	case "paren":
		return "(" + p.expr(p.arg(n, 0), lvComma) + ")"
	case "macro":
		return p.unquote(p.arg(n, 0))
	case "stmt-expr":
		sub := &printer{indent: p.indent}
		sub.compound(args)
		if sub.err != nil && p.err == nil {
			p.err = sub.err
		}
		return "(" + strings.TrimSpace(sub.b.String()) + ")"
	case "generic":
		var parts []string
		for _, a := range args[1:] {
			if a.Is("default") {
				parts = append(parts, "default: "+p.expr(p.arg(a, 0), lvAssign))
				continue
			}
			if !a.list || len(a.List) != 2 {
				p.fail(a, "a generic association is (TYPE VALUE)")
				return ""
			}
			parts = append(parts, p.typeName(a.List[0])+": "+p.expr(a.List[1], lvAssign))
		}
		return "_Generic(" + p.expr(p.arg(n, 0), lvAssign) + ", " + strings.Join(parts, ", ") + ")"
	case "label-addr":
		return "&&" + p.arg(n, 0).Atom
	case "call":
		var parts []string
		for _, a := range args[1:] {
			parts = append(parts, p.expr(a, lvAssign))
		}
		return p.expr(p.arg(n, 0), lvPostfix) + "(" + strings.Join(parts, ", ") + ")"
	case "index":
		s := p.expr(p.arg(n, 0), lvPostfix)
		for _, a := range args[1:] {
			s += "[" + p.expr(a, lvComma) + "]"
		}
		return s
	case ".", "->":
		s := p.expr(p.arg(n, 0), lvPostfix)
		for _, a := range args[1:] {
			s += h + p.form(a)
		}
		return s
	case "post++":
		return p.expr(p.arg(n, 0), lvPostfix) + "++"
	case "post--":
		return p.expr(p.arg(n, 0), lvPostfix) + "--"
	case "literal":
		return "(" + p.typeName(p.arg(n, 0)) + "){" + p.initList(args[1:]) + "}"
	case "pre++":
		return "++" + p.expr(p.arg(n, 0), lvUnary)
	case "pre--":
		return "--" + p.expr(p.arg(n, 0), lvUnary)
	case "addr":
		return cemit.Prefix("&", p.expr(p.arg(n, 0), lvCast))
	case "deref":
		return "*" + p.expr(p.arg(n, 0), lvCast)
	case "-", "+":
		return cemit.Prefix(h, p.expr(p.arg(n, 0), lvCast))
	case "!", "~":
		return h + p.expr(p.arg(n, 0), lvCast)
	case "sizeof", "alignof":
		return h + "(" + p.expr(p.arg(n, 0), lvComma) + ")"
	case "sizeof-bare", "alignof-bare":
		kw := strings.TrimSuffix(h, "-bare")
		in := p.expr(p.arg(n, 0), lvUnary)
		if strings.HasPrefix(in, "(") {
			return kw + in
		}
		return kw + " " + in
	case "sizeof-type", "alignof-type":
		return strings.TrimSuffix(h, "-type") + "(" + p.typeName(p.arg(n, 0)) + ")"
	case "cast":
		return "(" + p.typeName(p.arg(n, 0)) + ")" + p.expr(p.arg(n, 1), lvCast)
	case "?":
		return p.expr(p.arg(n, 0), lvLOr) + " ? " + p.expr(p.arg(n, 1), lvComma) + " : " + p.expr(p.arg(n, 2), lvCond)
	case "comma":
		var parts []string
		for _, a := range args {
			parts = append(parts, p.expr(a, lvAssign))
		}
		return strings.Join(parts, ", ")
	}
	p.fail(n, "not an expression")
	return ""
}
