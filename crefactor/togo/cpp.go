package togo

// cpp.go is the C++ backend's frame (doc/CPP.md): the C++23 a translation
// unit becomes -- whim++'s, the eighth translation.  C++ is nearly C's
// superset, so the translation keeps the C as it is written -- its
// statements, its expressions with their own parentheses, its types and
// memory -- and changes only what C++ says otherwise:
//
//   - the editor is an instance: the file-scope objects and the block-scope
//     statics are the fields of one `class Editor`, their initial values its
//     default member initializers, and every function a member function
//     (declared in the class, defined after it as `R Editor::f(...)`), so
//     that the C's names resolve in the class's scope as they did in the
//     file's and several editors run in one process;
//   - a function pointer is a pointer to member, `R (Editor::*)(...)`, a
//     function used as a value `&Editor::f`, a call through one
//     `(this->*p)(...)`;
//   - an enumerator is a `constexpr` of the type C gives it, and an
//     enumerated type its integer type, so that C's arithmetic on them is
//     C's (no C++ enum's conversions);
//   - what C converts implicitly and C++ does not -- a `void *` to another
//     pointer, a string literal to `char *`, pointers to differently signed
//     chars, a value narrowed in a braced list -- is a cast, written where
//     the conversion was;
//   - a declaration with an initial value that a jump crosses is a
//     declaration and an assignment;
//   - names C++ reserves take a trailing underscore.
//
// The output is two files: the header (the types, the constants and the
// class) and the source (the member functions).

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// cppReserved are C++'s keywords that C's are not, and the names the
// generated code uses: a C name among them takes a trailing underscore.
var cppReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`alignas alignof and and_eq asm bitand bitor catch char8_t char16_t char32_t class
	compl concept consteval constexpr constinit const_cast co_await co_return co_yield decltype delete dynamic_cast
	explicit export friend inline mutable namespace new noexcept not not_eq nullptr operator or or_eq private
	protected public reflexpr reinterpret_cast requires static_assert static_cast template this thread_local throw
	try typeid typename using virtual wchar_t xor xor_eq final override import module
	Editor Glue glue_ std whimpp chunks`) {
		cppReserved[w] = true
	}
}

// cppName is a C name as C++ spells it.
func cppName(s string) string {
	for cppReserved[s] {
		s += "_"
	}
	return s
}

// cppgen is the C++ backend's state for one translation unit.
type cppgen struct {
	g       *gen
	ns      string
	defined map[string]*cc.FunctionDefinition
	order   []*cc.FunctionDefinition
	hostFns map[string]*cc.Declarator
	// the file-scope objects, by key, in the order the C first declares
	// them, each with its defining declarator
	objKeys []string
	objs    map[string]*cppObj
	// a block-scope static that is a field: its name
	field map[*cc.Declarator]string
	// the declarators that are the editor's fields (file-scope objects and
	// hoisted statics): what makes an initial value the instance's
	inst map[*cc.Declarator]bool
	// file-scope compound literals hoisted to fields
	complits []string
	nlit     int

	// the function being written
	fn        *cc.FunctionDefinition
	result    cc.Type
	split     map[*cc.Declaration]bool
	specs     map[*cc.Declarator]*cc.DeclarationSpecifiers
	inClass   bool // printing an initial value of the class's
	nextIndex int64
	// declarations the statement being printed needs before it
	pre     []string
	asserts []string
	// the tagged structs functions define, hoisted to namespace scope
	localStructs []*cc.StructOrUnionSpecifier
	hoisted      map[*cc.StructOrUnionSpecifier]bool

	src    []byte
	lineAt []int

	indent int
	b      strings.Builder
	err    []string

	nCasts, nSplit, nFnPtr, nZeroed, nOrdered int
	// the function's nodes' parents, for maybeRead
	par map[cc.Node]cc.Node
	// the labels the function's gotos name
	gotos map[string]bool
	// the loop's locals declared at the function's top, and their names
	hoist map[*cc.Declarator]string
}

// cppObj is a file-scope object.
type cppObj struct {
	key      string
	d        *cc.Declarator // the declarator printed: the defining one
	decl     *cc.Declaration
	in       *cc.Initializer
	isStatic bool // a static member: const, its value no instance's
}

// writeCpp writes the translation unit as the C++ header path (with .hpp)
// and source (path's .cpp), and path.refused beside them.
func (g *gen) writeCpp(editorC, path string) error {
	src, err := os.ReadFile(editorC)
	if err != nil {
		return err
	}
	c := &cppgen{g: g, ns: g.p.CppNamespace, defined: map[string]*cc.FunctionDefinition{},
		hostFns: map[string]*cc.Declarator{}, objs: map[string]*cppObj{}, field: map[*cc.Declarator]string{},
		inst: map[*cc.Declarator]bool{}}
	c.src = src
	c.lineAt = []int{0, 0}
	for i, b := range src {
		if b == '\n' {
			c.lineAt = append(c.lineAt, i+1)
		}
	}
	if c.ns == "" {
		c.ns = "whimpp"
	}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			fd := ed.FunctionDefinition
			c.defined[fd.Declarator.Name()] = fd
			c.order = append(c.order, fd)
		}
	}
	for name, d := range g.a.fnDecls {
		if c.defined[name] == nil && !strings.HasPrefix(name, "__") {
			c.hostFns[name] = d
		}
	}
	c.indexSpecs()
	c.objects()
	hdr := c.header()
	csrc := c.source()
	base := strings.TrimSuffix(path, filepath.Ext(path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(base+".hpp", []byte(hdr), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(base+".cpp", []byte(csrc), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(logw, "cpp: %d functions, %d host functions, %d fields (%d static), %d casts written, %d declarations split, %d function pointers, %d locals zeroed, %d assignments ordered\n",
		len(c.order), len(c.hostFns), len(c.objKeys)+len(c.field), c.nStatic(), c.nCasts, c.nSplit, c.nFnPtr, c.nZeroed, c.nOrdered)
	if err := os.WriteFile(path+".host", []byte(c.hostSigs()), 0o644); err != nil {
		return err
	}
	// the layout the C front end gives the structs, which a test holds
	// g++'s to (the Scheme's listing: it names the structs as C does)
	if err := os.WriteFile(path+".layout", []byte((&sgen{g: g}).layout()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(path+".fnptrs", []byte(c.fnPtrStructs()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(path+".refused", []byte(strings.Join(c.err, "\n")), 0o644)
}

func (c *cppgen) nStatic() int {
	n := 0
	for _, o := range c.objs {
		if o.isStatic {
			n++
		}
	}
	return n
}

func (c *cppgen) fail(n cc.Node, format string, a ...any) {
	where := ""
	if n != nil {
		where = n.Position().String() + ": "
	}
	c.err = append(c.err, where+fmt.Sprintf(format, a...))
}

// objects finds the file-scope objects and the block-scope statics.
func (c *cppgen) objects() {
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || d.IsTypename() || d.Type() == nil || d.Type().Kind() == cc.Function {
				continue
			}
			k := c.g.a.declKey(d)
			o := c.objs[k]
			if o == nil {
				o = &cppObj{key: k}
				c.objs[k] = o
				c.objKeys = append(c.objKeys, k)
			}
			c.inst[d] = true
			// the definition: the one with a value, else the last (an
			// array's size is complete there)
			if o.in == nil {
				o.d, o.decl, o.in = d, ed.Declaration, l.InitDeclarator.Initializer
			}
		}
	}
	taken := map[string]bool{}
	for _, k := range c.objKeys {
		taken[c.objs[k].d.Name()] = true
	}
	for name := range c.defined {
		taken[name] = true
	}
	for _, s := range c.g.a.statics {
		if c.localStatic(s.d, s.init) {
			continue
		}
		n := s.fn + "__" + s.d.Name()
		for taken[n] {
			n += "_"
		}
		taken[n] = true
		c.field[s.d] = n
		c.inst[s.d] = true
	}
	// a const object whose value names no instance's object is the class's
	for _, k := range c.objKeys {
		o := c.objs[k]
		o.isStatic = c.constant(o.d, o.in)
	}
}

// constant reports whether d is const and its initial value holds nothing
// of an instance's: then one copy serves every editor.
func (c *cppgen) constant(d *cc.Declarator, in *cc.Initializer) bool {
	if in == nil || !topConst(d.Type()) {
		return false
	}
	return !c.namesInstance(in)
}

// localStatic reports whether a block-scope static stays in its function:
// a constant, the same for every editor.
func (c *cppgen) localStatic(d *cc.Declarator, in *cc.Initializer) bool {
	return in != nil && topConst(d.Type()) && !c.namesInstanceLoose(in)
}

// topConst reports whether t is const, or an array of const elements.
func topConst(t cc.Type) bool {
	for t != nil && t.Kind() == cc.Array {
		t = t.(*cc.ArrayType).Elem()
	}
	return t != nil && t.Attributes().IsConst()
}

// namesInstance reports whether n names a file-scope object or a hoisted
// static of the editor's (whose address an initial value takes).
func (c *cppgen) namesInstance(n cc.Node) bool {
	found := false
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if found || n == nil {
			return
		}
		if p, ok := n.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
			if d, ok := p.ResolvedTo().(*cc.Declarator); ok && c.inst[d] {
				found = true
				return
			}
		}
		if p, ok := n.(*cc.PostfixExpression); ok && p.Case == cc.PostfixExpressionComplit {
			found = true
			return
		}
		walkChildrenFn(n, rec)
	}
	rec(n)
	return found
}

// namesInstanceLoose is namesInstance before the statics are known: any
// object of static storage named.
func (c *cppgen) namesInstanceLoose(n cc.Node) bool {
	found := false
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if found || n == nil {
			return
		}
		if p, ok := n.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
			if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() != cc.Function {
				found = true
				return
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(n)
	return found
}

// pad is the current indent.
func (c *cppgen) pad() string { return strings.Repeat("    ", c.indent) }

func (c *cppgen) line(s string) {
	if s == "" {
		c.b.WriteString("\n")
		return
	}
	c.b.WriteString(c.pad() + s + "\n")
}

// header is the header: the constants and the types in the C's order, then
// the class.
func (c *cppgen) header() string {
	var h strings.Builder
	h.WriteString("// Code generated by `go tool whim skel -cpp` from a C translation unit; DO NOT EDIT.\n\n")
	h.WriteString("#pragma once\n\n")
	fmt.Fprintf(&h, "namespace %s {\n\n", c.ns)
	h.WriteString("class Editor;\nstruct Glue;\n\n")
	c.b.Reset()
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		c.typeDecl(ed.Declaration)
	}
	// the structs a function defines, at namespace scope: a static of the
	// function's that is the editor's field has that type
	for _, sp := range c.localStructs {
		c.line(c.structOrUnion(sp) + ";")
		c.line("")
	}
	h.WriteString(c.b.String())
	c.b.Reset()
	// the class: the functions first, so that a static member's initial
	// value may take any one's address
	c.line("")
	c.line("// One editor: the core's file-scope objects and block-scope statics, and its functions.")
	c.line("class Editor")
	c.line("{")
	c.line("public:")
	c.indent++
	c.line("// the host the editor runs on: the hand-written glue's, not the C's")
	c.line("Glue *glue_ = nullptr;")
	c.line("")
	var host []string
	for n := range c.hostFns {
		host = append(host, n)
	}
	sort.Strings(host)
	c.line("// the host's: hand-written")
	for _, n := range host {
		c.line(c.prototype(c.hostFns[n], "") + ";")
	}
	c.line("")
	c.line("// the core's")
	for _, fd := range c.order {
		c.line(c.prototype(fd.Declarator, "") + ";")
	}
	c.line("")
	c.line("// the objects")
	c.inClass = true
	saved := c.b.String()
	c.b.Reset()
	for _, k := range c.objKeys {
		c.fieldDecl(c.objs[k])
	}
	for _, s := range c.g.a.statics {
		if n, ok := c.field[s.d]; ok {
			c.staticField(s, n)
		}
	}
	fields := c.b.String()
	c.b.Reset()
	c.b.WriteString(saved)
	for _, l := range c.complits {
		c.line(l)
	}
	c.b.WriteString(fields)
	c.inClass = false
	if len(c.asserts) > 0 {
		c.line("")
		for _, a := range c.asserts {
			c.line(a)
		}
	}
	c.indent--
	c.line("};")
	c.line("")
	fmt.Fprintf(&c.b, "} // namespace %s\n", c.ns)
	h.WriteString(c.b.String())
	return h.String()
}

// source is the member functions' definitions.
func (c *cppgen) source() string {
	c.b.Reset()
	c.b.WriteString("// Code generated by `go tool whim skel -cpp` from a C translation unit; DO NOT EDIT.\n\n")
	c.b.WriteString("#include \"editor.hpp\"\n#include \"rt.hpp\"\n\n")
	fmt.Fprintf(&c.b, "namespace %s {\n", c.ns)
	for _, fd := range c.order {
		c.b.WriteString("\n")
		c.function(fd)
	}
	fmt.Fprintf(&c.b, "\n} // namespace %s\n", c.ns)
	return c.b.String()
}

// typeDecl writes a file-scope declaration's types: an enumeration's
// constants, a typedef, a struct's definition, a static_assert.  Objects
// and functions are the class's.
func (c *cppgen) typeDecl(n *cc.Declaration) {
	switch n.Case {
	case cc.DeclarationAssert:
		if c.namesInstanceLoose(n.StaticAssertDeclaration) {
			// of an object's size: the class's
			c.asserts = append(c.asserts, c.staticAssert(n.StaticAssertDeclaration))
			return
		}
		c.line(c.staticAssert(n.StaticAssertDeclaration))
		c.line("")
		return
	case cc.DeclarationDecl:
	default:
		c.fail(n, "declaration %v", n.Case)
		return
	}
	typedef := false
	for l := n.DeclarationSpecifiers; l != nil; l = l.DeclarationSpecifiers {
		if l.Case == cc.DeclarationSpecifiersStorage && l.StorageClassSpecifier.Token.SrcStr() == "typedef" {
			typedef = true
		}
	}
	// the definitions the specifiers hold come first, on their own
	c.definitions(n.DeclarationSpecifiers)
	if !typedef {
		return
	}
	specs := c.declSpecs(n.DeclarationSpecifiers, true)
	for l := n.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		c.line("typedef " + cppJoin(specs, c.declarator(l.InitDeclarator.Declarator, "", -1)) + ";")
	}
	c.line("")
}

// definitions writes the enumerations and structs a declaration's
// specifiers define, before the declaration itself.
func (c *cppgen) definitions(n *cc.DeclarationSpecifiers) {
	for l := n; l != nil; l = l.DeclarationSpecifiers {
		if l.Case != cc.DeclarationSpecifiersTypeSpec {
			continue
		}
		c.defsOf(l.TypeSpecifier)
	}
}

func (c *cppgen) defsOf(t *cc.TypeSpecifier) {
	switch t.Case {
	case cc.TypeSpecifierEnum:
		if e := t.EnumSpecifier; e.Case == cc.EnumSpecifierDef {
			c.enumConsts(e)
		}
	case cc.TypeSpecifierStructOrUnion:
		s := t.StructOrUnionSpecifier
		if s.Case == cc.StructOrUnionSpecifierDef && s.Token.SrcStr() != "" {
			// a tagged struct is defined at namespace scope, as C's tags are
			// the file's; an untagged one is defined where it is used
			for l := s.StructDeclarationList; l != nil; l = l.StructDeclarationList {
				if d := l.StructDeclaration; d.Case == cc.StructDeclarationDecl {
					c.nestedDefs(d.SpecifierQualifierList)
				}
			}
			c.line(c.structOrUnion(s) + ";")
			c.line("")
		} else if s.Case == cc.StructOrUnionSpecifierDef {
			for l := s.StructDeclarationList; l != nil; l = l.StructDeclarationList {
				if d := l.StructDeclaration; d.Case == cc.StructDeclarationDecl {
					c.nestedDefs(d.SpecifierQualifierList)
				}
			}
		}
	}
}

// nestedDefs writes the tagged structs and the enumerations a member's
// type defines, before the struct that holds it.
func (c *cppgen) nestedDefs(n *cc.SpecifierQualifierList) {
	for l := n; l != nil; l = l.SpecifierQualifierList {
		if l.Case == cc.SpecifierQualifierListTypeSpec {
			c.defsOf(l.TypeSpecifier)
		}
	}
}

// enumConsts writes an enumeration's constants as constexprs of the type C
// gives them.
func (c *cppgen) enumConsts(n *cc.EnumSpecifier) {
	for l := n.EnumeratorList; l != nil; l = l.EnumeratorList {
		e := l.Enumerator
		t := c.scalar(e.Type())
		v := ""
		if e.Case == cc.EnumeratorExpr {
			v = c.expr(e.ConstantExpression)
		} else {
			v = cppIntLit(e.Value(), e.Type())
		}
		c.line(fmt.Sprintf("constexpr %s %s = %s;", t, cppName(e.Token.SrcStr()), v))
	}
	c.line("")
}

// cppIntLit is an integer constant's value as a C++ literal of its type.
func cppIntLit(v cc.Value, t cc.Type) string {
	switch x := v.(type) {
	case cc.Int64Value:
		if x < 0 {
			return fmt.Sprintf("(%d)", int64(x))
		}
		return fmt.Sprintf("%d", int64(x))
	case cc.UInt64Value:
		return fmt.Sprintf("%dull", uint64(x))
	}
	return "0"
}

// scalar is a scalar C type's C++ spelling: its typedef's name when it has
// one, an enumeration its integer type.
func (c *cppgen) scalar(t cc.Type) string {
	if t == nil {
		return "int"
	}
	if e, ok := t.(*cc.EnumType); ok {
		t = e.UnderlyingType()
	}
	switch t.Kind() {
	case cc.Bool:
		return "bool"
	case cc.Char:
		return "char"
	case cc.SChar:
		return "signed char"
	case cc.UChar:
		return "unsigned char"
	case cc.Short:
		return "short"
	case cc.UShort:
		return "unsigned short"
	case cc.Int:
		return "int"
	case cc.UInt:
		return "unsigned int"
	case cc.Long:
		return "long"
	case cc.ULong:
		return "unsigned long"
	case cc.LongLong:
		return "long long"
	case cc.ULongLong:
		return "unsigned long long"
	case cc.Void:
		return "void"
	}
	return "int"
}

// typeStr is a type's C++ spelling as a cast writes it.
func (c *cppgen) typeStr(t cc.Type) string { return c.typeDecl1(t, "") }

// typeDecl1 is a declaration of name of type t: C's declarator syntax
// inside out.
func (c *cppgen) typeDecl1(t cc.Type, name string) string {
	if t == nil {
		return cppJoin("int", name)
	}
	quals := ""
	if t.Attributes().IsConst() {
		quals = "const "
	}
	if td := t.Typedef(); td != nil && td.Name() != "" {
		return cppJoin(quals+cppName(td.Name()), name)
	}
	switch t.Kind() {
	case cc.Ptr:
		e := t.(*cc.PointerType).Elem()
		pq := ""
		if t.Attributes().IsConst() {
			pq = " const"
		}
		if e.Kind() == cc.Function {
			return c.typeDecl1(e, "(Editor::*"+pq+name+")")
		}
		inner := "*" + strings.TrimSpace(pq+" "+name)
		if pq == "" {
			inner = "*" + name
		}
		if e.Kind() == cc.Array {
			inner = "(" + inner + ")"
		}
		return c.typeDecl1(e, inner)
	case cc.Array:
		a := t.(*cc.ArrayType)
		n := ""
		if a.Len() >= 0 {
			n = fmt.Sprint(a.Len())
		}
		return c.typeDecl1(a.Elem(), name+"["+n+"]")
	case cc.Function:
		f := t.(*cc.FunctionType)
		var ps []string
		for _, p := range f.Parameters() {
			if p.Type() == nil || p.Type().Kind() == cc.Void {
				continue
			}
			ps = append(ps, c.typeStr(p.Type()))
		}
		if f.IsVariadic() {
			ps = append(ps, "...")
		}
		return c.typeDecl1(f.Result(), name+"("+strings.Join(ps, ", ")+")")
	case cc.Struct, cc.Union:
		kw := "struct"
		var tag string
		if t.Kind() == cc.Union {
			kw = "union"
			tag = tagStr(t.(*cc.UnionType).Tag())
		} else {
			tag = tagStr(t.(*cc.StructType).Tag())
		}
		if tag == "" {
			c.fail(nil, "an untagged struct named by a cast: %s", t)
		}
		return cppJoin(quals+kw+" "+tag, name)
	}
	return cppJoin(quals+c.scalar(t), name)
}

// fieldDecl writes a file-scope object as a field: an instance's, or the
// class's own when it is a constant.
func (c *cppgen) fieldDecl(o *cppObj) {
	specs := c.declSpecs(o.decl.DeclarationSpecifiers, false)
	n := o.d.Type()
	ln := int64(-1)
	if a, ok := n.(*cc.ArrayType); ok {
		ln = a.Len()
	}
	s := cppJoin(specs, c.declarator(o.d, "", ln))
	if o.isStatic {
		s = "static inline " + s
	}
	if o.in != nil {
		s += " " + c.assign(o.in, o.d.Type())
	}
	c.line(s + ";")
}

// staticField writes a block-scope static as a field of the editor's.
func (c *cppgen) staticField(s *staticLocal, name string) {
	specs := c.specs[s.d]
	ln := int64(-1)
	if a, ok := s.d.Type().(*cc.ArrayType); ok {
		ln = a.Len()
	}
	var line string
	if specs != nil {
		line = cppJoin(c.declSpecs(specs, false), c.declarator(s.d, name, ln))
	} else {
		line = c.typeDecl1(s.d.Type(), name)
	}
	if s.init != nil {
		line += " " + c.assign(s.init, s.d.Type())
	}
	c.line(line + ";")
}

// prototype is a function's declaration: its name qualified by qual.
func (c *cppgen) prototype(d *cc.Declarator, qual string) string {
	specs := c.specs[d]
	name := qual + cppName(d.Name())
	return cppJoin(c.declSpecs(specs, false), c.declarator(d, name, -1))
}

// function writes a function's definition, out of the class.
func (c *cppgen) function(fd *cc.FunctionDefinition) {
	c.fn = fd
	c.result = nil
	if ft, ok := fd.Declarator.Type().(*cc.FunctionType); ok {
		c.result = ft.Result()
	}
	c.split = c.crossed(fd.CompoundStatement)
	c.par = nil
	c.gotos = map[string]bool{}
	var gotos func(n cc.Node)
	gotos = func(n cc.Node) {
		if j, ok := n.(*cc.JumpStatement); ok && j.Case == cc.JumpStatementGoto {
			c.gotos[j.Token2.SrcStr()] = true
		}
		walkChildrenFn(n, gotos)
	}
	gotos(fd.CompoundStatement)
	c.line(c.prototype(fd.Declarator, "Editor::"))
	for _, rb := range c.g.p.RuntimeBodies {
		if rb.Name == fd.Declarator.Name() && rb.Cpp != nil {
			// a rule of the runtime's, not a translation (Profile.RuntimeBodies)
			c.line("{")
			c.indent++
			for _, l := range strings.Split(strings.TrimRight(rb.Cpp(c.typeStr(c.result)), "\n"), "\n") {
				c.line(l)
			}
			c.indent--
			c.line("}")
			c.fn = nil
			return
		}
	}
	hoisted := c.hoistLoopLocals()
	c.line("{")
	c.indent++
	for _, h := range hoisted {
		c.line(h)
	}
	c.blockItems(fd.CompoundStatement.BlockItemList)
	c.indent--
	c.line("}")
	c.fn = nil
}

// specsOf is each declarator's declaration specifiers: a declaration's, a
// function definition's, a parameter's.
func (c *cppgen) indexSpecs() {
	c.specs = map[*cc.Declarator]*cc.DeclarationSpecifiers{}
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *cc.Declaration:
			for l := x.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
				if d := l.InitDeclarator.Declarator; d != nil {
					c.specs[d] = x.DeclarationSpecifiers
				}
			}
		case *cc.FunctionDefinition:
			c.specs[x.Declarator] = x.DeclarationSpecifiers
		}
		walkChildrenFn(n, rec)
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		rec(tu.ExternalDeclaration)
	}
	// the tagged structs defined in functions, unless a tag is defined twice
	c.hoisted = map[*cc.StructOrUnionSpecifier]bool{}
	tags := map[string]int{}
	var local []*cc.StructOrUnionSpecifier
	var find func(n cc.Node, inFn bool)
	find = func(n cc.Node, inFn bool) {
		if n == nil {
			return
		}
		if sp, ok := n.(*cc.StructOrUnionSpecifier); ok && sp.Case == cc.StructOrUnionSpecifierDef && sp.Token.SrcStr() != "" {
			tags[sp.Token.SrcStr()]++
			if inFn {
				local = append(local, sp)
			}
		}
		if _, ok := n.(*cc.FunctionDefinition); ok {
			inFn = true
		}
		walkChildrenFn(n, func(ch cc.Node) { find(ch, inFn) })
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		find(tu.ExternalDeclaration, false)
	}
	for _, sp := range local {
		if tags[sp.Token.SrcStr()] == 1 {
			c.hoisted[sp] = true
			c.localStructs = append(c.localStructs, sp)
		}
	}
}

// hostSigs lists the host's functions, a line each, for a test's glue:
// the result, the name and the parameters' types, `|` between, and `...`
// last for a variadic one.
func (c *cppgen) hostSigs() string {
	var names []string
	for n := range c.hostFns {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		ft, ok := c.hostFns[n].Type().(*cc.FunctionType)
		if !ok {
			continue
		}
		var ps []string
		for _, p := range ft.Parameters() {
			if p.Type() == nil || p.Type().Kind() == cc.Void {
				continue
			}
			ps = append(ps, c.typeStr(p.Type().Decay()))
		}
		if ft.IsVariadic() {
			ps = append(ps, "...")
		}
		fmt.Fprintf(&b, "%s|%s|%s\n", c.typeStr(ft.Result()), n, strings.Join(ps, "|"))
	}
	return b.String()
}

// fnPtrStructs lists the structs and unions, by their C names, that hold a
// pointer to a function -- in themselves or in a member they hold by value
// -- a line each: C++'s pointer to a member function is two words where
// C's pointer is one, so their layout is not C's (doc/CPP.md).
func (c *cppgen) fnPtrStructs() string {
	memo := map[cc.Type]bool{}
	var holds func(t cc.Type, depth int) bool
	holds = func(t cc.Type, depth int) bool {
		if t == nil || depth > 16 {
			return false
		}
		switch t.Kind() {
		case cc.Ptr:
			return cppFnPtr(t)
		case cc.Array:
			return holds(elemOf(t), depth+1)
		case cc.Struct, cc.Union:
		default:
			return false
		}
		if v, ok := memo[t]; ok {
			return v
		}
		memo[t] = false
		r := false
		for _, f := range scmFields(t) {
			if holds(f.Type(), depth+1) {
				r = true
			}
		}
		memo[t] = r
		return r
	}
	seen := map[string]bool{}
	var names []string
	var visit func(t cc.Type, depth int)
	visit = func(t cc.Type, depth int) {
		if t == nil || depth > 8 {
			return
		}
		switch t.Kind() {
		case cc.Ptr, cc.Array:
			visit(elemOf(t), depth+1)
			return
		case cc.Struct, cc.Union:
		default:
			return
		}
		n := scmCSpelling(t)
		if n != "" && seen[n] {
			return
		}
		if n != "" {
			seen[n] = true
			if holds(t, 0) {
				names = append(names, n)
			}
		}
		for _, f := range scmFields(t) {
			visit(f.Type(), depth+1)
		}
	}
	var rec func(n cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if d, ok := n.(*cc.Declarator); ok {
			visit(d.Type(), 0)
		}
		walkChildrenFn(n, rec)
	}
	for tu := c.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		rec(tu.ExternalDeclaration)
	}
	sort.Strings(names)
	return strings.Join(names, "\n") + "\n"
}
