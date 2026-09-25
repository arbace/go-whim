package togo

// java.go is the Java backend's frame: the types, the struct classes, the
// class the translation unit becomes and the report of what it refuses.  It
// reads the same analysis the Go is written from (an, the pointer classes)
// and writes one Java source file against the hand-written runtime in
// jeditor/rt (package whim.rt): BytePtr and its kin, Ptr<T>, Rt.
// doc/JAVA.md is the design.
//
// The mapping, in short:
//
//   - an integer type is the Java primitive of its size, holding its bits:
//     unsigned int is int, and C's unsigned division, comparison, right
//     shift and widening are written as Integer.divideUnsigned, compareUnsigned,
//     >>> and a mask; _Bool is boolean;
//   - a pointer to a scalar is a BytePtr, ShortPtr, IntPtr, LongPtr or
//     BoolPtr -- whether it walks or not: Java has no other address of a
//     scalar;
//   - a pointer to a struct is a plain reference to its class, or a Ptr<S>
//     over an S[] when its class walks (the analysis's cursor); a pointer to
//     a pointer is a Ptr<...> over the slots;
//   - an array is a Java array, which decays into the pointer classes above
//     over the same storage;
//   - a struct is a class whose set() is C's assignment, a copy; its struct
//     and array members are made with it; a union is the same class, every
//     member a field of its own, as the Go's;
//   - a scalar or pointer object whose address is taken -- a variable or a
//     struct's member -- is a one-element array from its declaration on;
//   - a void * is an Object; the growarray's storage is typed where it is
//     used by an accessor over the runtime's Ga, as the Go's GaData[T];
//   - a pointer to a function is a functional interface per Java signature,
//     a function used as a value one field holding its method reference;
//   - the file-scope objects are the class's fields, the functions its
//     methods, and the functions declared and not defined its abstract
//     methods: the host.
//
// A function it cannot write whole is REFUSED with its reason, as the Go
// emitter's -bodies does, and written as a stub that throws, so that the
// class compiles and its callers with it.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// jgen is the Java backend's state for one translation unit.
type jgen struct {
	g           *gen
	class       string
	structs     map[string]string // typeName -> Java class name
	classOrder  []string          // the struct classes, in the order met
	classType   map[string]cc.Type
	needEq      map[string]bool      // a class some code compares whole (memcmp)
	boxedField  map[string]bool      // a member whose address is taken, by fieldKey
	gaUsed      map[string]*gaHelper // the growarray's typed accessors, by the Java pointer type
	ifaces      map[string]*jiface   // the functional interfaces, by their signature
	ifaceByName map[string]*jiface
	fnRefs      map[string]string // a function as a value of an interface -> its field
	fnRefOrder  []string
	fnRefText   map[string]string
	anon        int
	boxed       map[*cc.Declarator]bool // a scalar or pointer object whose address is taken
	// ... and a file-scope one, by name: it may be declared more than once
	boxedGlobal map[string]bool
	enums       map[string]string // an enumerator used -> its declaration
	defined     map[string]bool
}

var javaReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`abstract assert boolean break byte case catch char class const continue
	default do double else enum extends final finally float for goto if implements import instanceof int
	interface long native new package private protected public return short static strictfp super switch
	synchronized this throw throws transient try void volatile while true false null var yield record
	sealed permits when module exports open opens requires uses provides to with transitive _
	Object String Integer Long Short Byte Boolean Math System Arrays BytePtr ShortPtr IntPtr LongPtr BoolPtr Ptr Rt Ga Struct`) {
		javaReserved[w] = true
	}
}

// jName is a C identifier as a Java one: the profile's renames, then a
// trailing underscore for Java's reserved words and the runtime's names.
func (j *jgen) jName(s string) string {
	if r, ok := j.g.p.Rename[s]; ok {
		s = r
	}
	if javaReserved[s] {
		return s + "_"
	}
	return s
}

// jk is a C scalar type as Java holds it: its size in bytes and signedness,
// or a boolean.
type jk struct {
	size    int
	signed  bool
	boolean bool
}

var jInt = jk{size: 4, signed: true}

func scalarKind(t cc.Type) (jk, bool) {
	if t == nil {
		return jk{}, false
	}
	switch t.Kind() {
	case cc.Bool:
		return jk{size: 1, boolean: true}, true
	case cc.Enum:
		if e, ok := t.(*cc.EnumType); ok {
			return scalarKind(e.UnderlyingType())
		}
		return jInt, true
	case cc.Char, cc.SChar, cc.UChar, cc.Short, cc.UShort, cc.Int, cc.UInt, cc.Long, cc.ULong, cc.LongLong, cc.ULongLong:
		return jk{size: int(t.Size()), signed: cc.IsSignedInteger(t)}, true
	}
	return jk{}, false
}

// java is the primitive holding the kind's bits.
func (k jk) java() string {
	if k.boolean {
		return "boolean"
	}
	switch k.size {
	case 1:
		return "byte"
	case 2:
		return "short"
	case 8:
		return "long"
	}
	return "int"
}

// promote is C's integer promotion.
func promote(k jk) jk {
	if k.boolean || k.size < 4 {
		return jInt
	}
	return k
}

// usualK is C's usual arithmetic conversion of two integer kinds.
func usualK(a, b jk) jk {
	a, b = promote(a), promote(b)
	if a == b {
		return a
	}
	if a.signed == b.signed {
		if a.size >= b.size {
			return a
		}
		return b
	}
	u, s := a, b
	if a.signed {
		u, s = b, a
	}
	if u.size >= s.size {
		return u
	}
	return s
}

// jlit is the C value v, converted to kind k, as a Java literal of k's type.
func jlit(v int64, k jk) string {
	switch {
	case k.boolean:
		if v != 0 {
			return "true"
		}
		return "false"
	case k.size == 1:
		return fmt.Sprintf("(byte) %d", int8(v))
	case k.size == 2:
		return fmt.Sprintf("(short) %d", int16(v))
	case k.size == 8:
		return fmt.Sprintf("%dL", v)
	}
	return fmt.Sprint(int32(v))
}

// ptrClass is the runtime's class of a pointer to a scalar of kind k.
func ptrClass(k jk) string {
	if k.boolean {
		return "BoolPtr"
	}
	switch k.size {
	case 1:
		return "BytePtr"
	case 2:
		return "ShortPtr"
	case 8:
		return "LongPtr"
	}
	return "IntPtr"
}

// scalarPtrElem is the primitive a runtime pointer class points at, or "".
func scalarPtrElem(t string) string {
	switch t {
	case "BytePtr":
		return "byte"
	case "ShortPtr":
		return "short"
	case "IntPtr":
		return "int"
	case "LongPtr":
		return "long"
	case "BoolPtr":
		return "boolean"
	}
	return ""
}

// elemJ is what a Java pointer or array type points at or holds.
func elemJ(t string) string {
	if e := scalarPtrElem(t); e != "" {
		return e
	}
	if strings.HasPrefix(t, "Ptr<") && strings.HasSuffix(t, ">") {
		return t[4 : len(t)-1]
	}
	if strings.HasSuffix(t, "[]") {
		return t[:len(t)-2]
	}
	return ""
}

// ptrOver is the pointer to element k of the Java array a of elements of
// Java type e.
func ptrOver(e, a, k string) string {
	switch e {
	case "byte":
		return "new BytePtr(" + a + ", " + k + ")"
	case "short":
		return "new ShortPtr(" + a + ", " + k + ")"
	case "int":
		return "new IntPtr(" + a + ", " + k + ")"
	case "long":
		return "new LongPtr(" + a + ", " + k + ")"
	case "boolean":
		return "new BoolPtr(" + a + ", " + k + ")"
	}
	return "new Ptr<" + e + ">(" + a + ", " + k + ")"
}

// ptrOfArray is the Java type an array of elements of Java type e decays to.
func ptrOfArray(e string) string {
	switch e {
	case "byte":
		return "BytePtr"
	case "short":
		return "ShortPtr"
	case "int":
		return "IntPtr"
	case "long":
		return "LongPtr"
	case "boolean":
		return "BoolPtr"
	}
	return "Ptr<" + e + ">"
}

// raw is a Java type with its type arguments dropped, for `new`: Java makes
// no array of a generic type.
func raw(t string) string {
	for {
		i := strings.Index(t, "<")
		if i < 0 {
			return t
		}
		d, k := 0, i
		for ; k < len(t); k++ {
			if t[k] == '<' {
				d++
			} else if t[k] == '>' {
				d--
				if d == 0 {
					break
				}
			}
		}
		t = t[:i] + t[k+1:]
	}
}

// jt is C type t as Java, key the object whose pointer class the top level
// takes; or why it has none.
func (j *jgen) jt(t cc.Type, key string) (string, string) {
	if t == nil {
		return "", "an expression of no type"
	}
	if k, ok := scalarKind(t); ok {
		return k.java(), ""
	}
	switch t.Kind() {
	case cc.Void:
		return "void", ""
	case cc.Float, cc.Double, cc.LongDouble, cc.ComplexFloat, cc.ComplexDouble, cc.ComplexLongDouble:
		return "", "floating point"
	case cc.Ptr:
		e := t.(*cc.PointerType).Elem()
		switch e.Kind() {
		case cc.Function:
			return j.fnIface(e.(*cc.FunctionType), "fp:"+key)
		case cc.Void:
			// a void *: whatever it holds, as an Object -- the growarray's
			// storage, typed where the C casts it (Ga), and the host's
			// allocation
			return "Object", ""
		case cc.Array:
			return "", "a pointer to an array"
		case cc.Struct, cc.Union:
			n, why := j.structName(e)
			if why != "" {
				return "", why
			}
			if j.g.cursor(key) {
				return "Ptr<" + n + ">", ""
			}
			return n, ""
		}
		if k, ok := scalarKind(e); ok {
			return ptrClass(k), ""
		}
		inner, why := j.jt(e, "elem:"+key)
		if why != "" {
			return "", why
		}
		return "Ptr<" + inner + ">", ""
	case cc.Array:
		at := t.(*cc.ArrayType)
		if at.IsIncomplete() || at.Len() <= 0 {
			return "", "an array of unknown size"
		}
		inner, why := j.jt(at.Elem(), "elem:"+key)
		if why != "" {
			return "", why
		}
		return inner + "[]", ""
	case cc.Struct, cc.Union:
		return j.structName(t)
	case cc.Function:
		return j.fnIface(t.(*cc.FunctionType), "fp:"+key)
	}
	return "", "a " + t.String()
}

// jnew is a new zeroed object of C type t, Java type jt: an array, a struct,
// or a scalar's or pointer's zero.
func (j *jgen) jnew(t cc.Type, jt string) (string, string) {
	switch t.Kind() {
	case cc.Struct, cc.Union:
		return "new " + jt + "()", ""
	case cc.Array:
		at := t.(*cc.ArrayType)
		base, dims := jt, ""
		et := t
		for strings.HasSuffix(base, "[]") {
			a := et.(*cc.ArrayType)
			dims += fmt.Sprintf("[%d]", a.Len())
			base = base[:len(base)-2]
			et = a.Elem()
		}
		if et.Kind() == cc.Struct || et.Kind() == cc.Union {
			if at.Elem().Kind() == cc.Array {
				return "", "an array of arrays of structs"
			}
			return fmt.Sprintf("%s.array(%d)", base, at.Len()), ""
		}
		return "new " + raw(base) + dims, ""
	case cc.Bool:
		return "false", ""
	}
	if _, ok := scalarKind(t); ok {
		return "0", ""
	}
	return "null", ""
}

// structName is the Java class of a struct or union type; its class is
// written when the class is (classText), since what it holds -- a member
// boxed, an eq() -- is known only when every method is.
func (j *jgen) structName(t cc.Type) (string, string) {
	if t.Kind() != cc.Struct && t.Kind() != cc.Union {
		return "", "a " + t.String()
	}
	key := typeName(t)
	if n, ok := j.structs[key]; ok {
		return n, ""
	}
	var name, tag string
	prefix := "S_"
	switch x := t.(type) {
	case *cc.StructType:
		tag = tagStr(x.Tag())
	case *cc.UnionType:
		tag = tagStr(x.Tag())
		prefix = "U_"
	}
	switch {
	case tag != "":
		name = prefix + tag
	case t.Typedef() != nil:
		name = "T_" + t.Typedef().Name()
	default:
		j.anon++
		name = fmt.Sprintf("A_%d", j.anon)
	}
	j.structs[key] = name
	j.classOrder = append(j.classOrder, name)
	j.classType[name] = t
	return name, ""
}

// members are a struct's or a union's fields.
func members(t cc.Type) []*cc.Field {
	var n int
	var field func(int) *cc.Field
	switch x := t.(type) {
	case *cc.StructType:
		n, field = x.NumFields(), x.FieldByIndex
	case *cc.UnionType:
		n, field = x.NumFields(), x.FieldByIndex
	}
	var fs []*cc.Field
	for i := 0; i < n; i++ {
		fs = append(fs, field(i))
	}
	return fs
}

// memberName is a member's Java name: its own, or _anonN for an unnamed one.
func (j *jgen) memberName(fl *cc.Field, i int) string {
	if fl.Name() == "" {
		return fmt.Sprintf("_anon%d", i)
	}
	return j.jName(fl.Name())
}

// structClass is a struct's Java class -- the other object o$, a name no C
// member has -- its members, set() -- C's
// assignment, a copy of every member -- copy(), array(n), n of them, and
// eq(), C's memcmp() == 0 of two, when some code compares them.  A union is
// the same class: every member is a field of its own, as the Go's union is
// (the C writes a member and reads that member back).  A scalar or pointer
// member whose address is taken somewhere is a one-element array, as an
// address-taken local is: &s->m is a pointer over that array.
func (j *jgen) structClass(name string, t cc.Type) string {
	var b, set, eq strings.Builder
	fmt.Fprintf(&b, "    static final class %s implements Struct<%s> {\n", name, name)
	if t.Kind() == cc.Union {
		b.WriteString("        // C union: every member is its own field here\n")
	}
	for i, fl := range members(t) {
		if fl == nil {
			continue
		}
		fname := j.memberName(fl, i)
		ft, why := j.jt(fl.Type(), fieldKey(fl))
		if why != "" {
			fmt.Fprintf(&b, "        Object %s; // C: %s -- %s\n", fname, fl.Type(), why)
			fmt.Fprintf(&set, "            %s = o$.%s;\n", fname, fname)
			fmt.Fprintf(&eq, "            if (%s != o$.%s) {\n                return false;\n            }\n", fname, fname)
			continue
		}
		if j.boxedField[fieldKey(fl)] {
			fmt.Fprintf(&b, "        final %s[] %s = new %s[1];\n", ft, fname, raw(ft))
			fmt.Fprintf(&set, "            %s[0] = o$.%s[0];\n", fname, fname)
			eq.WriteString(eqStmt(fname+"[0]", "o$."+fname+"[0]", fl.Type(), ft, 0, "            "))
			continue
		}
		switch fl.Type().Kind() {
		case cc.Struct, cc.Union, cc.Array:
			nw, why := j.jnew(fl.Type(), ft)
			if why != "" {
				fmt.Fprintf(&b, "        Object %s; // C: %s -- %s\n", fname, fl.Type(), why)
				fmt.Fprintf(&set, "            %s = o$.%s;\n", fname, fname)
				continue
			}
			fmt.Fprintf(&b, "        final %s %s = %s;\n", ft, fname, nw)
			set.WriteString(copyStmt(fname, "o$."+fname, fl.Type(), 0, "            "))
		default:
			fmt.Fprintf(&b, "        %s %s;\n", ft, fname)
			fmt.Fprintf(&set, "            %s = o$.%s;\n", fname, fname)
		}
		eq.WriteString(eqStmt(fname, "o$."+fname, fl.Type(), ft, 0, "            "))
	}
	fmt.Fprintf(&b, "\n        public %s set(%s o$) {\n%s            return this;\n        }\n", name, name, set.String())
	fmt.Fprintf(&b, "\n        %s copy() {\n            return new %s().set(this);\n        }\n", name, name)
	fmt.Fprintf(&b, "\n        public %s zero() {\n            return set(new %s());\n        }\n", name, name)
	fmt.Fprintf(&b, "\n        static %s[] array(int n) {\n            %s[] a = new %s[n];\n            for (int k = 0; k < n; k++) {\n                a[k] = new %s();\n            }\n            return a;\n        }\n", name, name, name, name)
	if j.needEq[name] {
		fmt.Fprintf(&b, "\n        boolean eq(%s o$) {\n%s            return true;\n        }\n", name, eq.String())
	}
	b.WriteString("    }\n")
	return b.String()
}

// eqStmt is the statements that return false when two values of C type t,
// Java type jt, differ: memcmp's answer on what C holds, member by member.
func eqStmt(a, b string, t cc.Type, jt string, depth int, ind string) string {
	neq := ""
	switch t.Kind() {
	case cc.Struct, cc.Union:
		neq = "!" + a + ".eq(" + b + ")"
	case cc.Array:
		at := t.(*cc.ArrayType)
		k := fmt.Sprintf("k$%d", depth)
		return fmt.Sprintf("%sfor (int %s = 0; %s < %d; %s++) {\n%s%s}\n", ind, k, k, at.Len(), k,
			eqStmt(a+"["+k+"]", b+"["+k+"]", at.Elem(), elemJ(jt), depth+1, ind+"    "), ind)
	default:
		neq = jnot(ptrEq(jt, a, b))
	}
	return fmt.Sprintf("%sif (%s) {\n%s    return false;\n%s}\n", ind, neq, ind, ind)
}

// eqClosure marks for an eq() every class a class that has one compares.
func (j *jgen) eqClosure() {
	for changed := true; changed; {
		changed = false
		for name := range j.needEq {
			for _, fl := range members(j.classType[name]) {
				if fl == nil {
					continue
				}
				t := fl.Type()
				for t.Kind() == cc.Array {
					t = t.(*cc.ArrayType).Elem()
				}
				if t.Kind() == cc.Struct || t.Kind() == cc.Union {
					if n, why := j.structName(t); why == "" && !j.needEq[n] {
						j.needEq[n] = true
						changed = true
					}
				}
			}
		}
	}
}

// copyStmt copies a value of C type t from src into dst, which exists:
// a scalar or pointer is assigned, a struct set, an array copied element by
// element.
func copyStmt(dst, src string, t cc.Type, depth int, ind string) string {
	switch t.Kind() {
	case cc.Struct, cc.Union:
		return ind + dst + ".set(" + src + ");\n"
	case cc.Array:
		at := t.(*cc.ArrayType)
		switch at.Elem().Kind() {
		case cc.Struct, cc.Union, cc.Array:
			k := fmt.Sprintf("k$%d", depth)
			return fmt.Sprintf("%sfor (int %s = 0; %s < %d; %s++) {\n%s%s}\n", ind, k, k, at.Len(), k,
				copyStmt(dst+"["+k+"]", src+"["+k+"]", at.Elem(), depth+1, ind+"    "), ind)
		}
		return fmt.Sprintf("%sSystem.arraycopy(%s, 0, %s, 0, %d);\n", ind, src, dst, at.Len())
	}
	return ind + dst + " = " + src + ";\n"
}

// boxes finds every scalar or pointer object whose address is taken -- a
// variable, or a struct's member -- Java has no address of either, so each
// is a one-element array.
func (j *jgen) boxes() {
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if n == nil {
			return
		}
		if u, ok := n.(*cc.UnaryExpression); ok && u.Case == cc.UnaryExpressionAddrof {
			if m, ok := unparenE(u.CastExpression).(*cc.PostfixExpression); ok && (m.Case == cc.PostfixExpressionSelect || m.Case == cc.PostfixExpressionPSelect) {
				if fl := m.Field(); fl != nil && fl.Type() != nil {
					if _, sc := scalarKind(fl.Type()); sc || fl.Type().Kind() == cc.Ptr {
						j.boxedField[fieldKey(fl)] = true
					}
				}
			}
			if p, ok := unparenE(u.CastExpression).(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionIdent {
				if d, ok := p.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil {
					if _, sc := scalarKind(d.Type()); sc || d.Type().Kind() == cc.Ptr {
						if d.Linkage() != cc.None {
							j.boxedGlobal[d.Name()] = true
						} else {
							j.boxed[d] = true
						}
					}
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	for tu := j.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		rec(tu.ExternalDeclaration)
	}
}

// isBoxed says an object is a one-element array.
func (j *jgen) isBoxed(d *cc.Declarator) bool {
	if d.Linkage() != cc.None {
		return j.boxedGlobal[d.Name()]
	}
	return j.boxed[d]
}

// writeJava writes the translation unit as one Java class to path, the class
// named after the file, and beside it path.refused: every function it
// refuses, with why.
func (g *gen) writeJava(path string) error {
	class := strings.TrimSuffix(filepath.Base(path), ".java")
	j := &jgen{g: g, class: class, structs: map[string]string{}, boxed: map[*cc.Declarator]bool{}, boxedGlobal: map[string]bool{},
		enums: map[string]string{}, defined: map[string]bool{},
		classType: map[string]cc.Type{}, needEq: map[string]bool{}, boxedField: map[string]bool{},
		gaUsed: map[string]*gaHelper{}, ifaces: map[string]*jiface{}, ifaceByName: map[string]*jiface{},
		fnRefs: map[string]string{}, fnRefText: map[string]string{}}
	j.boxes()
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		if ed := tu.ExternalDeclaration; ed.Case == cc.ExternalDeclarationFuncDef {
			j.defined[ed.FunctionDefinition.Declarator.Name()] = true
		}
	}

	// the functions, written or refused
	var methods, report strings.Builder
	written, replaced, total := 0, 0, 0
	rtWritten, rtTotal := 0, 0 // of the functions the Go's runtime replaces (Profile.Runtime)
	whys := map[string]int{}
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef {
			continue
		}
		fd := ed.FunctionDefinition
		total++
		name := fd.Declarator.Name()
		inRt := g.p.runtime[name]
		if inRt {
			rtTotal++
		}
		if j.replaced(name) {
			// the Java runtime's: every call is written as its (BytePtr.alloc,
			// Rt.memmove, ...), and no call reaches a method
			replaced++
			continue
		}
		src, why := j.method(fd)
		if why != "" {
			fmt.Fprintf(&report, "%s: %s\n", fd.Declarator.Name(), why)
			whys[reasonKey(why)]++
			methods.WriteString(j.stub(fd.Declarator, why))
			continue
		}
		written++
		if inRt {
			rtWritten++
		}
		methods.WriteString(src)
	}

	// the host: every function declared, used and not defined
	var host strings.Builder
	var hostNames []string
	for name := range g.a.fnDecls {
		p := g.p
		if p.allocators[name] || p.frees[name] || p.byteMove(name) || name == p.Bytes.Set || name == p.Bytes.Cmp {
			continue // written as the runtime's
		}
		if !j.defined[name] && !strings.HasPrefix(name, "__") {
			hostNames = append(hostNames, name)
		}
	}
	sort.Strings(hostNames)
	for _, name := range hostNames {
		host.WriteString(j.abstract(g.a.fnDecls[name]))
	}

	fields, inits, failed := j.fields()
	for _, f := range failed {
		fmt.Fprintf(&report, "initial value of %s\n", f)
	}
	j.eqClosure()
	var classes []string
	for i := 0; i < len(j.classOrder); i++ {
		name := j.classOrder[i]
		classes = append(classes, j.structClass(name, j.classType[name]))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by `go tool whim skel -java` from a C translation unit; DO NOT EDIT.\n")
	fmt.Fprintf(&b, "// %d of %d functions written, %d the runtime's; the rest are stubs that throw, with the reason.\n\n", written, total, replaced)
	if g.p.JavaPackage != "" {
		fmt.Fprintf(&b, "package %s;\n\n", g.p.JavaPackage)
	}
	b.WriteString("import whim.rt.*;\n\n")
	b.WriteString("@SuppressWarnings({\"unchecked\", \"rawtypes\"})\n")
	abstract := ""
	if len(hostNames) > 0 {
		abstract = "abstract "
	}
	fmt.Fprintf(&b, "public %sclass %s {\n", abstract, class)
	var enums []string
	for _, e := range j.enums {
		enums = append(enums, e)
	}
	sort.Strings(enums)
	for _, e := range enums {
		b.WriteString(e)
	}
	if len(enums) > 0 {
		b.WriteString("\n")
	}
	for _, c := range classes {
		b.WriteString(c + "\n")
	}
	b.WriteString(j.ifaceDecls())
	b.WriteString(fields)
	for _, k := range j.fnRefOrder {
		b.WriteString(j.fnRefText[k])
	}
	fmt.Fprintf(&b, "\n    public %s() {\n", class)
	for i := range inits {
		fmt.Fprintf(&b, "        initGlobals%d();\n", i)
	}
	b.WriteString("    }\n")
	for i, in := range inits {
		fmt.Fprintf(&b, "\n    private void initGlobals%d() {\n%s    }\n", i, in)
	}
	if host.Len() > 0 {
		b.WriteString("\n    // the host: declared, and not defined here\n")
		b.WriteString(host.String())
	}
	b.WriteString(j.gaHelpers())
	b.WriteString(methods.String())
	b.WriteString("}\n")

	var ks []string
	for k := range whys {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(a, b int) bool {
		if whys[ks[a]] != whys[ks[b]] {
			return whys[ks[a]] > whys[ks[b]]
		}
		return ks[a] < ks[b]
	})
	fmt.Fprintf(logw, "java: %d of %d functions written, %d the Java runtime's, %d refused", written, total, replaced, total-written-replaced)
	if rtTotal > 0 {
		fmt.Fprintf(logw, "; of the %d the Go's runtime replaces, %d written and %d the Java runtime's", rtTotal, rtWritten, replaced)
	}
	fmt.Fprintln(logw)
	for _, k := range ks {
		fmt.Fprintf(logw, "  %5d  %s\n", whys[k], k)
	}
	if len(failed) > 0 {
		fmt.Fprintf(logw, "java: %d initial values refused\n", len(failed))
	}
	if err := os.WriteFile(path+".refused", []byte(report.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// replaced says a function is the Java runtime's and not written: an
// allocator or a function of bytes (Profile), whose calls the backend
// writes as the runtime's, and which nothing takes the address of.
func (j *jgen) replaced(name string) bool {
	p := j.g.p
	return (p.allocators[name] || p.byteMove(name) || name == p.Bytes.Set || name == p.Bytes.Cmp) && !j.g.a.addr[name]
}

// reasonKey is a refusal's reason without where it was met or the detail
// after a colon: what the refusals are grouped by.
func reasonKey(why string) string {
	if i := strings.Index(why, " at "); i > 0 {
		why = why[:i]
	}
	if i := strings.Index(why, ": "); i > 0 {
		why = why[:i]
	}
	return why
}

// sigType is a parameter's or result's Java type in a signature that must be
// written though it may not be expressible: a stub's, the host's.
func (j *jgen) sigType(t cc.Type, key string) string {
	if t.Kind() == cc.Array {
		t = t.Decay()
	}
	s, why := j.jt(t, key)
	if why != "" {
		return "Object"
	}
	return s
}

// sigWhy is why a function's signature is not Java, or "".
func (j *jgen) sigWhy(name string, ft *cc.FunctionType) string {
	for i, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		t := p.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		if _, why := j.jt(t, fmt.Sprintf("param:%s:%d", name, i)); why != "" {
			return why
		}
	}
	if _, why := j.jt(ft.Result(), "ret:"+name); why != "" {
		return why
	}
	return ""
}

// params is a signature's parameter list, names p0... when names is false.
func (j *jgen) params(d *cc.Declarator, ft *cc.FunctionType) string {
	var ps []string
	for i, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		ps = append(ps, fmt.Sprintf("%s p%d", j.sigType(p.Type(), fmt.Sprintf("param:%s:%d", d.Name(), i)), i))
	}
	if ft.IsVariadic() {
		ps = append(ps, "Object... args")
	}
	return strings.Join(ps, ", ")
}

// stub is a refused function: its signature, and a body that throws with
// the reason.
func (j *jgen) stub(d *cc.Declarator, why string) string {
	ft, _ := d.Type().(*cc.FunctionType)
	rt := j.sigType(ft.Result(), "ret:"+d.Name())
	return fmt.Sprintf("\n    %s %s(%s) {\n        throw new UnsupportedOperationException(%s);\n    }\n",
		rt, j.jName(d.Name()), j.params(d, ft), javaQuote("refused: "+why))
}

// abstract is a host function: declared, not defined, so the host's.
func (j *jgen) abstract(d *cc.Declarator) string {
	ft, ok := d.Type().(*cc.FunctionType)
	if !ok {
		return ""
	}
	rt := j.sigType(ft.Result(), "ret:"+d.Name())
	return fmt.Sprintf("    abstract %s %s(%s);\n", rt, j.jName(d.Name()), j.params(d, ft))
}

// fields are the file-scope objects and the hoisted block-scope statics as
// fields, and their C initial values as the bodies of methods of at most a
// few hundred lines each: a Java method's code is at most 64 KB.
func (j *jgen) fields() (string, []string, []string) {
	var b strings.Builder
	var failed []string
	f := j.newFn("initGlobals", nil)
	f.indent = 2
	var inits []string
	var cur strings.Builder
	f.out = &cur
	flush := func() {
		if cur.Len() > 0 {
			inits = append(inits, f.hoisted()+cur.String())
			cur.Reset()
			f.locals = nil
		}
	}
	one := func(d *cc.Declarator, name, key string, in *cc.Initializer) {
		t := d.Type()
		jt, why := j.jt(t, key)
		if why != "" {
			fmt.Fprintf(&b, "    Object %s; // C: %s -- %s\n", name, t, why)
			return
		}
		switch {
		case j.isBoxed(d):
			fmt.Fprintf(&b, "    %s[] %s = new %s[1];\n", jt, name, raw(jt))
		case isAggr(t) || t.Kind() == cc.Array:
			nw, why := j.jnew(t, jt)
			if why != "" {
				fmt.Fprintf(&b, "    Object %s; // C: %s -- %s\n", name, t, why)
				return
			}
			fmt.Fprintf(&b, "    final %s %s = %s;\n", jt, name, nw)
		default:
			fmt.Fprintf(&b, "    %s %s;\n", jt, name)
		}
		if in == nil || zeroInit(in) {
			return
		}
		target := name
		if j.isBoxed(d) {
			target += "[0]"
		}
		text := f.capture(func() {
			defer func() {
				if r := recover(); r != nil {
					u, ok := r.(unsupported)
					if !ok {
						panic(r)
					}
					failed = append(failed, d.Name()+": "+u.why)
					f.line("// refused: the initial value of %s: %s", d.Name(), u.why)
				}
			}()
			f.initInto(target, t, key, in, true)
		})
		cur.WriteString(text)
		if strings.Count(cur.String(), "\n") > 400 {
			flush()
		}
	}
	// an object declared more than once -- extern, tentatively, then
	// defined -- is one field: of the declaration whose type is complete,
	// with the initializer one of them has
	type global struct {
		d  *cc.Declarator
		in *cc.Initializer
	}
	var order []string
	globals := map[string]*global{}
	for tu := j.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration.Case != cc.DeclarationDecl {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			id := l.InitDeclarator
			d := id.Declarator
			if d.IsTypename() || d.Type().Kind() == cc.Function {
				continue
			}
			gl, ok := globals[d.Name()]
			if !ok {
				gl = &global{d: d}
				globals[d.Name()] = gl
				order = append(order, d.Name())
			}
			if at, ok := gl.d.Type().(*cc.ArrayType); ok && (at.IsIncomplete() || at.Len() <= 0) || id.Initializer != nil {
				gl.d = d
			}
			if id.Initializer != nil {
				gl.in = id.Initializer
			}
		}
	}
	for _, name := range order {
		gl := globals[name]
		one(gl.d, j.jName(name), "global:"+name, gl.in)
	}
	for _, s := range j.g.a.statics {
		one(s.d, j.staticName(s.fn, s.d.Name()), fmt.Sprintf("static:%s.%s", s.fn, s.d.Name()), s.init)
	}
	flush()
	if len(inits) == 0 {
		inits = []string{""}
	}
	return b.String(), inits, failed
}

// staticName is a hoisted block-scope static's field: <function>_<name>.
func (j *jgen) staticName(fn, name string) string {
	return j.jName(fn + "_" + name)
}

// javaQuote is a Java string literal of bytes: printable ASCII as itself,
// every other byte an octal escape (never \u, which Java reads before it
// reads the literal), so that each char is one byte (BytePtr.lit).
func javaQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\t':
			b.WriteString(`\t`)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\%03o`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// isAggr says t is a struct or a union: a class, assigned by set().
func isAggr(t cc.Type) bool {
	return t != nil && (t.Kind() == cc.Struct || t.Kind() == cc.Union)
}

// jiface is a functional interface: C's pointer to a function of one
// signature, as Java writes it -- `interface FnN { R call(P0 a0, ...); }`.
// Two C types that Java writes alike are one interface: Java's types are
// nominal, C's function types are not.
type jiface struct {
	name   string
	params []string
	result string
	sig    string
}

// fnIface is the functional interface of C function type ft, key the object
// whose pointers' classes its parameters (key:i) and result (ret:key) take.
func (j *jgen) fnIface(ft *cc.FunctionType, key string) (string, string) {
	return j.fnIfaceK(ft, key, "ret:"+key)
}

// fnIfaceK is fnIface with the result's key given: a function's own
// interface has param:f:i and ret:f.
func (j *jgen) fnIfaceK(ft *cc.FunctionType, key, rkey string) (string, string) {
	if ft.IsVariadic() {
		return "", "a pointer to a variadic function"
	}
	var ps []string
	for i, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		t := p.Type()
		if t.Kind() == cc.Array {
			t = t.Decay()
		}
		s, why := j.jt(t, fmt.Sprintf("%s:%d", key, i))
		if why != "" {
			return "", why
		}
		ps = append(ps, s)
	}
	r, why := j.jt(ft.Result(), rkey)
	if why != "" {
		return "", why
	}
	sig := r + "(" + strings.Join(ps, ", ") + ")"
	if f, ok := j.ifaces[sig]; ok {
		return f.name, ""
	}
	f := &jiface{name: fmt.Sprintf("Fn%d", len(j.ifaces)+1), params: ps, result: r, sig: sig}
	j.ifaces[sig] = f
	j.ifaceByName[f.name] = f
	return f.name, ""
}

// ifaceDecls declares the functional interfaces, in the order they were met.
func (j *jgen) ifaceDecls() string {
	fs := make([]*jiface, 0, len(j.ifaces))
	for _, f := range j.ifaces {
		fs = append(fs, f)
	}
	sort.Slice(fs, func(a, b int) bool {
		return len(fs[a].name) < len(fs[b].name) || len(fs[a].name) == len(fs[b].name) && fs[a].name < fs[b].name
	})
	var b strings.Builder
	for _, f := range fs {
		var ps []string
		for i, p := range f.params {
			ps = append(ps, fmt.Sprintf("%s a%d", p, i))
		}
		fmt.Fprintf(&b, "    @FunctionalInterface\n    interface %s {\n        %s call(%s);\n    }\n\n", f.name, f.result, strings.Join(ps, ", "))
	}
	return b.String()
}

// runtimeBody is the Java body the profile gives a function, or nil.
func (j *jgen) runtimeBody(name string) func(string) string {
	for _, rb := range j.g.p.RuntimeBodies {
		if rb.Name == name && rb.Java != nil {
			return rb.Java
		}
	}
	return nil
}

// gaHelpers are the growarray's typed accessors the methods use: each makes
// or grows the storage as its type (Ga) and keeps it in the array.
func (j *jgen) gaHelpers() string {
	var ks []string
	for k := range j.gaUsed {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b strings.Builder
	for _, to := range ks {
		h := j.gaUsed[to]
		var make string
		switch to {
		case "BytePtr":
			make = "Ga.bytes(gap.%[1]s, gap.%[2]s)"
		case "ShortPtr":
			make = "Ga.shorts(gap.%[1]s, gap.%[2]s)"
		case "IntPtr":
			make = "Ga.ints(gap.%[1]s, gap.%[2]s)"
		case "LongPtr":
			make = "Ga.longs(gap.%[1]s, gap.%[2]s)"
		case "BoolPtr":
			make = "Ga.bools(gap.%[1]s, gap.%[2]s)"
		default:
			e := elemJ(to)
			mk := raw(e) + "[]::new"
			if _, ok := j.classType[e]; ok {
				mk = e + "::array"
			}
			make = "Ga.ptrs(gap.%[1]s, gap.%[2]s, " + mk + ")"
		}
		make = fmt.Sprintf(make, j.jName(j.g.p.GrowArray.Data), j.jName(j.g.p.GrowArray.MaxLen))
		fmt.Fprintf(&b, "\n    private static %s %s(%s gap) {\n        %s p = %s;\n        gap.%s = p;\n        return p;\n    }\n",
			to, h.name, h.class, to, make, j.jName(j.g.p.GrowArray.Data))
	}
	return b.String()
}

// gaHelper is one typed accessor of the growarray's storage.
type gaHelper struct{ name, class string }
