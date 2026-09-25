package togo

// body.go writes the functions' bodies: editor.c's statements and expressions
// in Go, to internal/gen/CONVENTIONS.md, against the types, globals and signatures this
// program already generates.  Every construct it meets has a rule or stops
// the function: a function it cannot write whole is reported and not written,
// so what it writes is only ever a complete translation.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
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
	null    bool  // C's null pointer constant
	cv      int64 // a constant's C value
	hasCv   bool
}

type local struct {
	name, typ string
	read      bool
	scoped    bool // declared where C declares it, not at the function's top
}

type fnEmit struct {
	g        *gen
	name     string
	ft       *cc.FunctionType
	out      *strings.Builder
	indent   int
	locals   []*local
	byDecl   map[*cc.Declarator]*local
	taken    map[string]bool // Go names in use in this function
	tmp      int
	gotos    map[string]bool            // labels some goto names
	cont     []string                   // the label a continue goes to, per loop; "" is Go's continue
	brk      []bool                     // whether the innermost breakable is a loop
	brkTo    []string                   // the label a C break at that level leaves by, or "" for a bare break
	deadBrk  map[*cc.JumpStatement]bool // breaks that end a case already ending in one
	hoistAll bool                       // the function has a goto: every local at the top, where no jump can pass it
	inCase   bool                       // emitting a switch case's own items: C's case is no scope, Go's is
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
	switch {
	case strings.HasPrefix(t, "*"):
		return "*" + g.canon(t[1:])
	case strings.HasPrefix(t, "Ptr[") && strings.HasSuffix(t, "]"):
		return "Ptr[" + g.canon(t[4:len(t)-1]) + "]"
	case strings.HasPrefix(t, "[") && strings.Contains(t, "]"):
		i := strings.Index(t, "]")
		return t[:i+1] + g.canon(t[i+1:])
	}
	for i := 0; i < 8; i++ {
		v, ok := g.aliasOf[t]
		if !ok {
			break
		}
		t = v
	}
	if t != "" && t != "any" && (strings.HasPrefix(t, "*") || strings.HasPrefix(t, "Ptr[") || strings.HasPrefix(t, "[")) {
		return g.canon(t)
	}
	return t
}

func (f *fnEmit) isIntGo(t string) bool {
	switch t {
	case "byte", "int8", "int16", "uint16", "int32", "uint32", "int64", "uint64":
		return true
	}
	return t == f.g.sizeType()
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
		case strings.HasPrefix(want, "*"), strings.HasPrefix(want, "func("), want == "any", strings.HasPrefix(want, "[]"):
			return "nil"
		case f.isIntGo(want):
			return "0"
		}
	}
	if v.boolean && f.isIntGo(want) {
		if want == "int32" {
			return "B2i(" + v.s + ")"
		}
		return to + "(B2i(" + v.s + "))"
	}
	if from == "bool" && f.isIntGo(want) {
		if want == "int32" {
			return "B2i(" + v.s + ")"
		}
		return to + "(B2i(" + v.s + "))"
	}
	if want == "bool" && !v.boolean {
		if t := f.truth(v); t == "true" || t == "false" {
			return t
		}
		return "(" + f.truth(v) + ")"
	}
	if from == want {
		return v.s
	}
	if v.konst && v.hasCv && v.cv == 0 && (strings.HasPrefix(want, "Ptr[") || strings.HasPrefix(want, "*")) {
		return f.conv(val{s: "nil", t: to, null: true}, to) // a 0 is a null pointer constant
	}
	if v.konst && (f.isIntGo(want) || want == "bool") {
		if k, ok := ikOf[want]; ok && v.hasCv && !k.signed && v.cv < 0 {
			// a negative constant made unsigned: its value, as C converts it
			u := uint64(v.cv)
			if k.size < 8 {
				u &= 1<<(8*k.size) - 1
			}
			return strconv.FormatUint(u, 10)
		}
		if want == f.g.sizeType() && v.hasCv && v.cv < 0 {
			return strconv.FormatUint(uint64(v.cv), 10)
		}
		return v.s
	}
	switch {
	case f.isIntGo(want) && (f.isIntGo(from) || v.konst):
		return to + "(" + v.s + ")"
	case want == "any":
		return v.s
	case from == "any":
		return v.s + ".(" + to + ")"
	case strings.HasPrefix(want, "Ptr[") && strings.HasPrefix(from, "*") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return "Addr(" + v.s + ")"
	case strings.HasPrefix(want, "*") && strings.HasPrefix(from, "Ptr[") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return elemRef(v.s)
	case strings.HasPrefix(want, "*") && strings.HasPrefix(from, "[") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return "&" + v.s + "[0]" // an array decays to its first element's address
	case strings.HasPrefix(want, "[]") && strings.HasPrefix(from, "Ptr[") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		// a C pointer into a slice that only walks forward from it
		if strings.HasPrefix(v.s, "S(\"") && strings.HasSuffix(v.s, "\")") && want == "[]byte" {
			return "[]byte(" + strings.TrimSuffix(strings.TrimPrefix(v.s, "S("), "\")") + "\\x00\")"
		}
		return v.s + ".Tail()"
	case strings.HasPrefix(want, "[]") && strings.HasPrefix(from, "*") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return "One(" + v.s + ")"
	case strings.HasPrefix(want, "[]") && strings.HasPrefix(from, "[") && !strings.HasPrefix(from, "[]") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return v.s + "[:]"
	case strings.HasPrefix(want, "Ptr[") && strings.HasPrefix(from, "[]") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return "View(" + v.s + ")"
	case strings.HasPrefix(want, "Ptr[") && strings.HasPrefix(from, "[") && f.g.canon(elemOfGo(want)) == f.g.canon(elemOfGo(from)):
		return "View(" + v.s + "[:])"
	case strings.HasPrefix(want, "func(") && strings.HasPrefix(from, "func("):
		return v.s
	}
	f.no(nil, "no conversion from %s to %s for %s", v.t, to, v.s)
	return ""
}

// truth is v as a Go condition.
func (f *fnEmit) truth(v val) string {
	if v.boolean {
		return v.s
	}
	if v.konst && v.hasCv {
		return strconv.FormatBool(v.cv != 0) // C's `true`, `1`, `0`: Go's constant
	}
	t := f.g.canon(v.t)
	switch {
	case strings.HasPrefix(t, "Ptr["):
		return "!" + v.s + ".Nil()"
	case strings.HasPrefix(t, "*"), strings.HasPrefix(t, "func("), t == "any", strings.HasPrefix(t, "[]"):
		return v.s + " != nil"
	case t == "bool":
		return v.s
	}
	return v.s + " != 0"
}

func (f *fnEmit) falsity(v val) string {
	if v.boolean {
		return not(v.s)
	}
	if v.konst && v.hasCv {
		return strconv.FormatBool(v.cv == 0)
	}
	t := f.g.canon(v.t)
	switch {
	case strings.HasPrefix(t, "Ptr["):
		return v.s + ".Nil()"
	case strings.HasPrefix(t, "*"), strings.HasPrefix(t, "func("), t == "any", strings.HasPrefix(t, "[]"):
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
			l := &local{name: n, typ: typ, read: true}
			f.locals = append(f.locals, l)
			if !f.hoistAll {
				// declared where it is made: every temporary is assigned on the
				// next line, so it carries nothing from an earlier evaluation
				l.scoped = true
				f.line("var %s %s\x01%s", n, typ, n)
			}
			return n
		}
	}
}

func (f *fnEmit) declare(d *cc.Declarator, typ string) *local {
	name := f.g.goName(d.Name())
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
		if ft, ok := t.(*cc.FunctionType); ok {
			return val{s: f.g.goName(d.Name()), t: f.g.funcSig(d.Name(), ft), c: t}
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
			return val{s: f.g.goName(d.Name()), t: f.g.goType(t, key), c: t}
		}
		if strings.HasPrefix(key, "global:") {
			return val{s: f.g.goName(d.Name()), t: f.objType(t, key), c: t}
		}
		f.no(x, "a name with no declaration here: %s (%s)", d.Name(), key)
	case *cc.Enumerator:
		return val{s: f.g.goName(x.Token.SrcStr()), t: "int32", c: x.Type(), konst: true}
	}
	f.no(x, "an identifier resolved to %T", x.ResolvedTo())
	return val{}
}

var _ = sort.Strings
var _ = strconv.Itoa

// funcSig is a defined function's Go type, from its own parameters' classes.
func (g *gen) funcSig(name string, ft *cc.FunctionType) string {
	var ps []string
	for i, p := range ft.Parameters() {
		if p.Type() != nil && p.Type().Kind() == cc.Void {
			continue
		}
		ps = append(ps, g.goType(p.Type(), fmt.Sprintf("param:%s:%d", name, i)))
	}
	if ft.IsVariadic() {
		ps = append(ps, "...any")
	}
	s := "func(" + strings.Join(ps, ", ") + ")"
	if r := g.goType(ft.Result(), "ret:"+name); r != "" {
		s += " " + r
	}
	return s
}

// elemRef is the Ptr s as a *T: the element it points at.  An element address
// that went through a Ptr only to be read through is said as one --
// View(a[:]).Add(k) is &a[k], and p.Add(k) is p.Ref(k).
func elemRef(s string) string {
	base, k := s, "0"
	if strings.HasSuffix(s, ")") {
		if i := openParen(s); i > 5 && s[i-4:i] == ".Add" {
			base, k = s[:i-4], s[i+1:len(s)-1]
		}
	}
	if strings.HasPrefix(base, "View(") && strings.HasSuffix(base, "[:])") && openParen(base) == 4 {
		return "&" + base[5:len(base)-4] + "[" + k + "]"
	}
	if k == "0" {
		return s + ".P()"
	}
	return base + ".Ref(" + k + ")"
}

// openParen is the index of the '(' that the ')' ending s closes, or -1 --
// and -1 for text with a literal in it, whose brackets are not code.
func openParen(s string) int {
	if strings.ContainsAny(s, "\"'`") {
		return -1
	}
	depth := 0
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ')', ']', '}':
			depth++
		case '(', '[', '{':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// deref is *s, folding *&x to x.
func deref(s string) string {
	if strings.HasPrefix(s, "&") && paren(s[1:]) == s[1:] {
		return s[1:]
	}
	return "*" + paren(s)
}

// not is !s, folding !!x to x.
func not(s string) string {
	if strings.HasPrefix(s, "!") && paren(s[1:]) == s[1:] {
		return s[1:]
	}
	return "!" + paren(s)
}
