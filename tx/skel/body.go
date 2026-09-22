package main

// body.go writes the functions' bodies: editor.c's statements and expressions
// in Go, to tx/CONVENTIONS.md, against the types, globals and signatures this
// program already generates.  Every construct it meets has a rule or stops
// the function: a function it cannot write whole is reported and not written,
// so what it writes is only ever a complete translation.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"modernc.org/cc/v4"
)

// unsupported stops the function being written.
type unsupported struct{ why string }

func (f *fnEmit) no(n cc.Node, format string, args ...any) {
	where := ""
	if n != nil {
		where = fmt.Sprintf(" at %d", n.Position().Line)
	}
	panic(unsupported{fmt.Sprintf(format, args...) + where})
}

// val is an emitted expression: its Go source and its Go type.  konst is an
// untyped Go constant (a literal, an enum constant, sizeof); boolean is a Go
// bool where C had an int (a comparison or a logical operator).
type val struct {
	s       string
	t       string
	c       cc.Type
	konst   bool
	boolean bool
	null    bool // C's null pointer constant
}

type local struct {
	name, typ string
	read      bool
}

type fnEmit struct {
	g      *gen
	name   string
	ft     *cc.FunctionType
	out    *strings.Builder
	indent int
	locals []*local
	byDecl map[*cc.Declarator]*local
	taken  map[string]bool // Go names in use in this function
	tmp    int
	gotos  map[string]bool // labels some goto names
	cont   []string        // the label a continue goes to, per loop; "" is Go's continue
	brk    []bool          // whether the innermost breakable is a loop
}

func (f *fnEmit) line(format string, args ...any) {
	f.out.WriteString(strings.Repeat("\t", f.indent))
	fmt.Fprintf(f.out, format, args...)
	f.out.WriteString("\n")
}

// --- types -----------------------------------------------------------------

// canon is a Go type with its aliases resolved, for comparing.
func (g *gen) canon(t string) string {
	if g.aliasOf == nil {
		g.aliasOf = map[string]string{}
		for _, a := range g.aliases {
			var n, v string
			if _, err := fmt.Sscanf(a, "type %s = %s", &n, &v); err == nil {
				g.aliasOf[n] = strings.TrimPrefix(a, "type "+n+" = ")
			}
		}
	}
	for i := 0; i < 8; i++ {
		v, ok := g.aliasOf[t]
		if !ok {
			break
		}
		t = v
	}
	return t
}

func isIntGo(t string) bool {
	switch t {
	case "byte", "int8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "usize":
		return true
	}
	return false
}

// objType is the Go type of an object: a pointer takes its kind from the
// object's class; a local array a cursor reaches is a Ptr, as a global is.
func (f *fnEmit) objType(t cc.Type, key string) string {
	if at, ok := t.(*cc.ArrayType); ok && at.Len() > 0 && f.g.cursor(key) {
		elem := f.g.goType(at.Elem(), "elem:"+key)
		if isByteType(at.Elem()) {
			elem = "byte"
		}
		return "Ptr[" + elem + "]"
	}
	return f.g.goType(t, key)
}

func isByteType(t cc.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case cc.Char, cc.UChar, cc.SChar:
		return t.Kind() != cc.SChar
	}
	return false
}

// arith is the Go type of a C arithmetic type.
func (f *fnEmit) arith(t cc.Type) string {
	if s := scalar(t); s != "" {
		return s
	}
	f.no(nil, "not arithmetic: %s", t)
	return ""
}

// promote is C's integer promotion and usual arithmetic conversion, as Go
// scalar names.
type ik struct {
	signed bool
	size   int
}

var ikOf = map[string]ik{"byte": {false, 1}, "int8": {true, 1}, "int16": {true, 2}, "uint16": {false, 2},
	"int32": {true, 4}, "uint32": {false, 4}, "int64": {true, 8}, "uint64": {false, 8}, "bool": {false, 1}}

func goOfIk(k ik) string {
	switch {
	case k.size <= 4 && k.signed:
		return "int32"
	case k.size <= 4:
		return "uint32"
	case k.signed:
		return "int64"
	}
	return "uint64"
}

func promoted(t string) string {
	k, ok := ikOf[t]
	if !ok {
		return t
	}
	if k.size < 4 {
		return "int32"
	}
	return t
}

func usual(a, b string) string {
	a, b = promoted(a), promoted(b)
	if a == b {
		return a
	}
	ka, oka := ikOf[a]
	kb, okb := ikOf[b]
	if !oka || !okb {
		return a
	}
	if ka.signed == kb.signed {
		if ka.size >= kb.size {
			return a
		}
		return b
	}
	u, s := ka, kb
	if ka.signed {
		u, s = kb, ka
	}
	if u.size >= s.size {
		return goOfIk(u)
	}
	return goOfIk(s)
}

// --- conversions -----------------------------------------------------------

func elemOfGo(t string) string {
	switch {
	case strings.HasPrefix(t, "Ptr[") && strings.HasSuffix(t, "]"):
		return t[4 : len(t)-1]
	case strings.HasPrefix(t, "*"):
		return t[1:]
	case strings.HasPrefix(t, "["):
		return t[strings.Index(t, "]")+1:]
	}
	return ""
}

// conv is v as Go type to.
func (f *fnEmit) conv(v val, to string) string {
	if to == "" {
		return v.s
	}
	from, want := f.g.canon(v.t), f.g.canon(to)
	if v.null {
		switch {
		case strings.HasPrefix(want, "Ptr["):
			return to + "{}"
		case strings.HasPrefix(want, "*"), strings.HasPrefix(want, "func("), want == "any":
			return "nil"
		case isIntGo(want):
			return "0"
		}
	}
	if v.boolean && isIntGo(want) {
		if want == "int32" {
			return "B2i(" + v.s + ")"
		}
		return to + "(B2i(" + v.s + "))"
	}
	if want == "bool" && !v.boolean {
		return "(" + f.truth(v) + ")"
	}
	if from == want {
		return v.s
	}
	if v.konst && (isIntGo(want) || want == "bool") {
		if c := constValue(v.c, v); c != "" && isIntGo(want) {
			return c
		}
		return v.s
	}
	switch {
	case isIntGo(want) && (isIntGo(from) || v.konst):
		return to + "(" + v.s + ")"
	case want == "any":
		return v.s
	case from == "any":
		return v.s + ".(" + to + ")"
	case strings.HasPrefix(want, "Ptr[") && strings.HasPrefix(from, "*") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return "Addr(" + v.s + ")"
	case strings.HasPrefix(want, "*") && strings.HasPrefix(from, "Ptr[") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return v.s + ".P()"
	case strings.HasPrefix(want, "Ptr[") && strings.HasPrefix(from, "[") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return "View(" + v.s + "[:])"
	case strings.HasPrefix(want, "func(") && strings.HasPrefix(from, "func("):
		return v.s
	}
	f.no(nil, "no conversion from %s to %s for %s", v.t, to, v.s)
	return ""
}

// constValue is a constant as a Go literal of its C value when it would not
// fit the Go type as written (a negative constant made unsigned).
func constValue(t cc.Type, v val) string {
	return ""
}

// truth is v as a Go condition.
func (f *fnEmit) truth(v val) string {
	if v.boolean {
		return v.s
	}
	t := f.g.canon(v.t)
	switch {
	case strings.HasPrefix(t, "Ptr["):
		return "!" + v.s + ".Nil()"
	case strings.HasPrefix(t, "*"), strings.HasPrefix(t, "func("), t == "any":
		return v.s + " != nil"
	case t == "bool":
		return v.s
	}
	return v.s + " != 0"
}

func (f *fnEmit) falsity(v val) string {
	if v.boolean {
		return "!(" + v.s + ")"
	}
	t := f.g.canon(v.t)
	switch {
	case strings.HasPrefix(t, "Ptr["):
		return v.s + ".Nil()"
	case strings.HasPrefix(t, "*"), strings.HasPrefix(t, "func("), t == "any":
		return v.s + " == nil"
	case t == "bool":
		return "!" + v.s
	}
	return v.s + " == 0"
}

// paren wraps a composite expression.
func paren(s string) string {
	simple := true
	depth := 0
	for _, r := range s {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ' ':
			if depth == 0 {
				simple = false
			}
		}
	}
	if simple || (strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && balanced(s[1:len(s)-1])) {
		return s
	}
	return "(" + s + ")"
}

func balanced(s string) bool {
	d := 0
	for _, r := range s {
		switch r {
		case '(':
			d++
		case ')':
			d--
			if d < 0 {
				return false
			}
		}
	}
	return d == 0
}

// --- names -----------------------------------------------------------------

func (f *fnEmit) newTemp(typ string) string {
	for {
		f.tmp++
		n := fmt.Sprintf("t%d", f.tmp)
		if !f.taken[n] {
			f.taken[n] = true
			f.locals = append(f.locals, &local{name: n, typ: typ, read: true})
			return n
		}
	}
}

func (f *fnEmit) declare(d *cc.Declarator, typ string) *local {
	name := GoName(d.Name())
	base := name
	for i := 2; f.taken[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	f.taken[name] = true
	l := &local{name: name, typ: typ}
	f.locals = append(f.locals, l)
	f.byDecl[d] = l
	return l
}

// ident is a name as a value.
func (f *fnEmit) ident(x *cc.PrimaryExpression) val {
	switch d := x.ResolvedTo().(type) {
	case *cc.Declarator:
		t := d.Type()
		if l, ok := f.byDecl[d]; ok {
			l.read = true
			return val{s: l.name, t: l.typ, c: t}
		}
		if t.Kind() == cc.Function {
			return val{s: GoName(d.Name()), t: f.g.goType(t, d.Name()), c: t}
		}
		key := f.g.a.declKey(d)
		if strings.HasPrefix(key, "static:") {
			n := f.g.globalName[key]
			if n == "" {
				f.no(x, "a static with no global: %s", key)
			}
			return val{s: n, t: f.objType(t, key), c: t}
		}
		if d.IsParam() {
			for i, p := range f.ft.Parameters() {
				if p.Declarator == d {
					key = fmt.Sprintf("param:%s:%d", f.name, i)
				}
			}
			return val{s: GoName(d.Name()), t: f.g.goType(t, key), c: t}
		}
		if strings.HasPrefix(key, "global:") {
			return val{s: GoName(d.Name()), t: f.objType(t, key), c: t}
		}
		f.no(x, "a name with no declaration here: %s (%s)", d.Name(), key)
	case *cc.Enumerator:
		return val{s: GoName(x.Token.SrcStr()), t: "int32", c: x.Type(), konst: true}
	}
	f.no(x, "an identifier resolved to %T", x.ResolvedTo())
	return val{}
}

var _ = sort.Strings
var _ = strconv.Itoa
