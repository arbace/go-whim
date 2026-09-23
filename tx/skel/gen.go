package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// gen writes the skeleton: types.go, globals.go and sigs.txt.
type gen struct {
	ast *cc.AST
	a   *an

	structName map[cc.Type]string   // struct/union type -> Go type name
	emitted    map[string]bool      // Go type names written
	types      []string             // type declarations, in order
	consts     []string             // const lines
	aliases    []string             // typedef aliases
	globals    []string             // var lines
	sigs       []string             // signature lines for agents
	notes      []string             // things an agent must know
	seenEnum   map[*cc.EnumType]bool
	globalName map[string]string // C global or static-local key -> Go name
	defined    map[string]bool
	aliasOf    map[string]string // a type alias's name -> what it is
}

func newGen(ast *cc.AST, a *an) *gen {
	return &gen{ast: ast, a: a, structName: map[cc.Type]string{}, emitted: map[string]bool{},
		seenEnum: map[*cc.EnumType]bool{}, globalName: map[string]string{}}
}

var goReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`break case chan const continue default defer else fallthrough for func go goto if
	import interface map package range return select struct switch type var any append bool byte cap clear close
	complex complex64 complex128 copy delete error false float32 float64 imag int int8 int16 int32 int64 iota len
	make max min new nil panic print println real recover rune string true uint uint8 uint16 uint32 uint64 uintptr
	comparable main init`) {
		goReserved[w] = true
	}
}

// GoName is a C identifier as a Go one: reserved words get a trailing underscore.
func GoName(s string) string {
	if s == "_" {
		return "gettext_" // vim's _(), the identity once gettext went; _ is Go's blank
	}
	if goReserved[s] {
		return s + "_"
	}
	return s
}

func (g *gen) cursor(key string) bool {
	ok, _ := g.a.u.cursor(key)
	return ok
}

// scalar maps a C arithmetic type.
func scalar(t cc.Type) string {
	switch t.Kind() {
	case cc.Bool:
		return "bool"
	case cc.Char, cc.UChar:
		return "byte"
	case cc.SChar:
		return "int8"
	case cc.Short:
		return "int16"
	case cc.UShort:
		return "uint16"
	case cc.Int:
		return "int32"
	case cc.UInt:
		return "uint32"
	case cc.Long, cc.LongLong:
		return "int64"
	case cc.ULong, cc.ULongLong:
		return "uint64"
	case cc.Float:
		return "float32"
	case cc.Double, cc.LongDouble:
		return "float64"
	case cc.Enum:
		if e, ok := t.(*cc.EnumType); ok {
			return scalar(e.UnderlyingType())
		}
		return "int32"
	}
	return ""
}

// goType is t in Go; key is the object whose pointer kind the top level takes.
func (g *gen) goType(t cc.Type, key string) string {
	if t == nil {
		return "any"
	}
	// scalar and struct typedefs keep their names (defined as aliases)
	if td := t.Typedef(); td != nil {
		switch t.Kind() {
		case cc.Struct, cc.Union:
			return g.structType(t)
		case cc.Ptr, cc.Array, cc.Function:
			// expanded below: the pointer kind is the object's
		default:
			if s := scalar(t); s != "" {
				return GoName(td.Name())
			}
		}
	}
	switch t.Kind() {
	case cc.Void:
		return ""
	case cc.Ptr:
		e := t.(*cc.PointerType).Elem()
		switch e.Kind() {
		case cc.Function:
			return g.funcType(e.(*cc.FunctionType), "fp:"+key)
		case cc.Void:
			return "any"
		case cc.Char, cc.SChar, cc.UChar:
			if g.a.u.punned(key) {
				// the C code keeps other things' addresses in it, cast
				return "any"
			}
			return "Ptr[byte]"
		}
		inner := g.goType(e, "elem:"+key)
		if g.cursor(key) {
			return "Ptr[" + inner + "]"
		}
		return "*" + inner
	case cc.Array:
		at := t.(*cc.ArrayType)
		inner := g.goType(at.Elem(), "elem:"+key)
		if at.Elem().Kind() == cc.Char || at.Elem().Kind() == cc.UChar || at.Elem().Kind() == cc.SChar {
			inner = "byte"
		}
		if at.IsIncomplete() || at.Len() <= 0 {
			return "Ptr[" + inner + "]"
		}
		return fmt.Sprintf("[%d]%s", at.Len(), inner)
	case cc.Struct, cc.Union:
		return g.structType(t)
	case cc.Function:
		return g.funcType(t.(*cc.FunctionType), key)
	}
	if s := scalar(t); s != "" {
		return s
	}
	return "any /* " + t.String() + " */"
}

func (g *gen) funcType(ft *cc.FunctionType, key string) string {
	var ps []string
	for i, p := range ft.Parameters() {
		if p.Type() != nil && p.Type().Kind() == cc.Void {
			continue
		}
		ps = append(ps, g.goType(p.Type(), fmt.Sprintf("%s:%d", key, i)))
	}
	if ft.IsVariadic() {
		ps = append(ps, "...any")
	}
	r := g.goType(ft.Result(), "ret:"+key)
	s := "func(" + strings.Join(ps, ", ") + ")"
	if r != "" {
		s += " " + r
	}
	return s
}

// structType names a struct or union, emitting its declaration once.
func (g *gen) structType(t cc.Type) string {
	if n, ok := g.structName[t]; ok {
		return n
	}
	var name, tag string
	switch x := t.(type) {
	case *cc.StructType:
		tag = tagStr(x.Tag())
	case *cc.UnionType:
		tag = tagStr(x.Tag())
	}
	td := t.Typedef()
	switch {
	case tag != "":
		// named by its tag; a typedef of it is an alias
		name = "S_" + tag
		if td != nil {
			g.aliases = append(g.aliases, fmt.Sprintf("type %s = %s", GoName(td.Name()), name))
		}
	case td != nil:
		name = GoName(td.Name())
	default:
		// anonymous and not typedef'd: an inline literal type
		return g.structBody(t)
	}
	g.structName[t] = name
	if !g.emitted[name] {
		g.emitted[name] = true
		body := g.structBody(t)
		g.types = append(g.types, fmt.Sprintf("type %s %s\n", name, body))
	}
	return name
}

func (g *gen) structBody(t cc.Type) string {
	var b strings.Builder
	var n int
	var field func(int) *cc.Field
	isUnion := false
	switch x := t.(type) {
	case *cc.StructType:
		n, field = x.NumFields(), x.FieldByIndex
	case *cc.UnionType:
		n, field = x.NumFields(), x.FieldByIndex
		isUnion = true
	}
	b.WriteString("struct {")
	if isUnion {
		b.WriteString(" // C union: every member is its own field here")
	}
	b.WriteString("\n")
	for i := 0; i < n; i++ {
		f := field(i)
		if f == nil {
			continue
		}
		fname := f.Name()
		if fname == "" {
			fname = fmt.Sprintf("_anon%d", i)
		}
		key := fieldKey(f)
		ft := f.Type()
		gt := g.goType(ft, key)
		note := ""
		if at, ok := ft.(*cc.ArrayType); ok && i == n-1 && (at.Len() <= 1) && t.Kind() == cc.Struct {
			// the struct hack: a trailing one-element array sized at allocation
			elem := g.goType(at.Elem(), "elem:"+key)
			if at.Elem().Kind() == cc.Char || at.Elem().Kind() == cc.UChar {
				elem = "byte"
			}
			gt = "Ptr[" + elem + "]"
			note = " // C struct hack: sized at allocation"
		}
		if f.IsBitfield() {
			note += fmt.Sprintf(" // C bitfield :%d", f.ValueBits())
		}
		fmt.Fprintf(&b, "\t%s %s%s\n", GoName(fname), gt, note)
	}
	b.WriteString("}")
	return b.String()
}

func enumValue(e *cc.Enumerator) string {
	switch v := e.Value().(type) {
	case cc.Int64Value:
		return fmt.Sprint(int64(v))
	case cc.UInt64Value:
		return fmt.Sprint(uint64(v))
	}
	return "0 /* ? */"
}

func (g *gen) enum(e *cc.EnumType) {
	if g.seenEnum[e] {
		return
	}
	g.seenEnum[e] = true
	for _, en := range e.Enumerators() {
		g.consts = append(g.consts, fmt.Sprintf("\t%s = %s", GoName(en.Token.SrcStr()), enumValue(en)))
	}
}

// collect walks every file-scope declaration.
func (g *gen) collect() {
	fnSeen := map[string]bool{}
	g.defined = map[string]bool{}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			g.defined[ed.FunctionDefinition.Declarator.Name()] = true
		}
	}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		switch ed.Case {
		case cc.ExternalDeclarationFuncDef:
			fd := ed.FunctionDefinition
			g.signature(fd.Declarator, "")
			fnSeen[fd.Declarator.Name()] = true
			g.collectBlockTypes(fd)
		case cc.ExternalDeclarationDecl:
			g.declaration(ed.Declaration, fnSeen)
		}
	}
	// only the types something uses: a declaration, a cast or a sizeof inside
	// a function body reaches them here; signatures and globals already did
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			walkTypes(ed.FunctionDefinition.CompoundStatement, func(t cc.Type) { g.goType(t, "") })
		}
	}
	// hoisted block-scope statics
	for _, s := range g.a.statics {
		name := GoName(s.fn + "_" + s.d.Name())
		key := fmt.Sprintf("static:%s.%s", s.fn, s.d.Name())
		g.globalName[key] = name
		g.globals = append(g.globals, g.varLine(name, s.d.Type(), key, "static in "+s.fn+"()"))
	}
}

// collectBlockTypes emits enums declared inside a function (their constants
// are file-wide in Go).
func (g *gen) collectBlockTypes(fd *cc.FunctionDefinition) {
	var visit func(n cc.Node)
	visit = func(n cc.Node) {}
	_ = visit
	// enums inside functions are found through the declarator types of the
	// function's locals by the walk below
	walkDecls(fd.CompoundStatement, func(d *cc.Declarator) {
		if e, ok := d.Type().(*cc.EnumType); ok {
			g.enum(e)
		}
	}, func(e *cc.EnumType) { g.enum(e) })
}

func (g *gen) declaration(d *cc.Declaration, fnSeen map[string]bool) {
	if d.Case != cc.DeclarationDecl {
		return
	}
	// enum constants declared by the specifiers
	for ds := d.DeclarationSpecifiers; ds != nil; ds = ds.DeclarationSpecifiers {
		if ds.TypeSpecifier != nil && ds.TypeSpecifier.EnumSpecifier != nil {
			if e, ok := ds.TypeSpecifier.EnumSpecifier.Type().(*cc.EnumType); ok {
				g.enum(e)
			}
		}
	}
	for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		dd := l.InitDeclarator.Declarator
		t := dd.Type()
		switch {
		case dd.IsTypename():
			g.typedef(dd)
		case t.Kind() == cc.Function:
			// a prototype of a function defined here is covered by its
			// definition; one defined nowhere in the core is the host's
			if !g.defined[dd.Name()] && !strings.HasPrefix(dd.Name(), "__") {
				g.signature(dd, "host")
			}
		default:
			key := "global:" + dd.Name()
			name := GoName(dd.Name())
			g.globalName[key] = name
			g.globals = append(g.globals, g.varLine(name, t, key, ""))
		}
	}
}

func (g *gen) typedef(d *cc.Declarator) {
	t := d.Type()
	name := GoName(d.Name())
	switch t.Kind() {
	case cc.Struct, cc.Union:
		g.structType(t)
	case cc.Enum:
		if e, ok := t.(*cc.EnumType); ok {
			g.enum(e)
		}
		g.aliases = append(g.aliases, fmt.Sprintf("type %s = %s", name, scalar(t)))
	case cc.Ptr, cc.Function, cc.Array:
		g.aliases = append(g.aliases, fmt.Sprintf("type %s = %s", name, g.goType(t, "typedef:"+d.Name())))
	default:
		if s := scalar(t); s != "" && !g.emitted[name] {
			g.emitted[name] = true
			g.aliases = append(g.aliases, fmt.Sprintf("type %s = %s", name, s))
		}
	}
}

// varLine declares a global.  An array a cursor reaches is a Ptr with its
// storage allocated here, so its identity is stable.
func (g *gen) varLine(name string, t cc.Type, key, note string) string {
	c := ""
	if note != "" {
		c = " // " + note
	}
	if at, ok := t.(*cc.ArrayType); ok && at.Len() > 0 && g.cursor(key) {
		elem := g.goType(at.Elem(), "elem:"+key)
		if at.Elem().Kind() == cc.Char || at.Elem().Kind() == cc.UChar {
			elem = "byte"
		}
		return fmt.Sprintf("var %s = Mk[%s](%d)%s // C: %s", name, elem, at.Len(), c, t.String())
	}
	return fmt.Sprintf("var %s %s%s", name, g.goType(t, key), c)
}

// signature records the Go signature of a function.
func (g *gen) signature(d *cc.Declarator, who string) {
	ft, ok := d.Type().(*cc.FunctionType)
	if !ok {
		return
	}
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
		if (pn == "varp" || pn == "varp_arg") && isCharPtr(p.Type()) {
			// an option's variable, of whatever type: see the puns in facts.json
			g.a.u.markPun(pk, "named varp")
		}
		ps = append(ps, GoName(pn)+" "+g.goType(p.Type(), pk))
	}
	if ft.IsVariadic() {
		ps = append(ps, "args ...any")
	}
	r := g.goType(ft.Result(), "ret:"+d.Name())
	s := fmt.Sprintf("func %s(%s)", GoName(d.Name()), strings.Join(ps, ", "))
	if r != "" {
		s += " " + r
	}
	if who != "" {
		s += " // " + who
	}
	g.sigs = append(g.sigs, s)
}

// typesText is types.go: the aliases, the struct types and the constants.
func (g *gen) typesText() string {
	var b strings.Builder
	b.WriteString("package main\n\n")
	sort.Strings(g.aliases)
	for _, l := range dedupe(g.aliases) {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n")
	for _, t := range g.types {
		b.WriteString(t + "\n")
	}
	b.WriteString("const (\n")
	for _, c := range dedupe(g.consts) {
		b.WriteString(c + "\n")
	}
	b.WriteString(")\n")
	return b.String()
}

// globalsText is globals.go: every file-scope object and every block-scope
// static, hoisted as <function>_<name>, with its type.
func (g *gen) globalsText() string {
	var b strings.Builder
	b.WriteString("package main\n\n")
	for _, l := range dedupe(g.globals) {
		b.WriteString(l + "\n")
	}
	return b.String()
}

func (g *gen) write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte("// Code generated by tx/skel from editor.c: the types of the transpilation.\n\n"+g.typesText()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "globals.go"), []byte("// Code generated by tx/skel from editor.c: every file-scope object and every\n// block-scope static, hoisted as <function>_<name>, with its type and no initializer.\n\n"+g.globalsText()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "sigs.txt"), []byte(strings.Join(dedupe(g.sigs), "\n")+"\n"), 0o644)
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var r []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			r = append(r, x)
		}
	}
	return r
}

// walkDecls visits the declarators and enum types inside a block.
func walkDecls(n cc.Node, fd func(*cc.Declarator), fe func(*cc.EnumType)) {
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.EnumSpecifier:
			if e, ok := x.Type().(*cc.EnumType); ok {
				fe(e)
			}
		case *cc.Declarator:
			if x.Type() != nil {
				fd(x)
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(n)
}

// walkTypes visits the type of every declarator and type name in a block.
func walkTypes(n cc.Node, f func(cc.Type)) {
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.Declarator:
			if x.Type() != nil {
				f(x.Type())
			}
		case *cc.TypeName:
			if x.Type() != nil {
				f(x.Type())
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(n)
}
