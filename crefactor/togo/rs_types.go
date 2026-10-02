package togo

// rs_types.go is the Rust backend's types: a C scalar is the Rust integer
// of its width and signedness (a bool a bool), a pointer a raw pointer, a
// pointer to a function an Option of an unsafe fn of the editor and the C's
// parameters, an array a Rust array, and a struct or a union a #[repr(C)]
// type of the same members in the same order -- so the same layout, which
// the layout listing (path.layout) lets a test hold to gcc's.  A typedef of
// a scalar or a pointer is a type alias of its name, as the C spells it.

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// rsKind is the Rust type of a C scalar kind.
func rsKind(k jk) string {
	if k.boolean {
		return "bool"
	}
	if k.signed {
		return fmt.Sprintf("i%d", k.size*8)
	}
	return fmt.Sprintf("u%d", k.size*8)
}

// aggTag is a struct's or a union's tag, "" when it has none.
func aggTag(t cc.Type) string {
	switch x := t.(type) {
	case *cc.StructType:
		tk := x.Tag()
		return tk.SrcStr()
	case *cc.UnionType:
		tk := x.Tag()
		return tk.SrcStr()
	}
	return ""
}

// aggKey is a struct's or a union's identity: the typedef's clone of a type
// shares its fields, so its first field is the type; an incomplete one is
// its tag.
func aggKey(t cc.Type) string {
	var first *cc.Field
	switch x := t.(type) {
	case *cc.StructType:
		first = x.FieldByIndex(0)
	case *cc.UnionType:
		first = x.FieldByIndex(0)
	}
	if first == nil {
		return "tag:" + aggTag(t)
	}
	return fmt.Sprintf("%p", first)
}

// structName is the Rust name of a struct or a union: its typedef's, else
// its tag's, else hint (its member's name in the type that holds it).
func (r *rgen) structName(t cc.Type, hint string) string {
	key := aggKey(t)
	if n, ok := r.structNames[key]; ok {
		return n
	}
	tag := aggTag(t)
	n := r.tagTypedef[tag]
	if n == "" && t.Typedef() != nil {
		n = t.Typedef().Name()
	}
	if n == "" {
		n = tag
	}
	if n == "" {
		n = hint
	}
	if n == "" {
		n = "anon"
	}
	n = rsName(n)
	base := n
	for i := 2; r.structTaken[n]; i++ {
		n = fmt.Sprintf("%s_%d", base, i)
	}
	r.structTaken[n] = true
	r.structNames[key] = n
	r.structType[n] = t
	r.structOrder = append(r.structOrder, n)
	return n
}

// ty is t's Rust type, every typedef expanded: the canonical spelling, two
// types are one when theirs are.
func (r *rgen) ty(t cc.Type) string { return r.rtype(t, false, "") }

// declType is t's Rust type as a declaration says it: a typedef of a
// scalar or a pointer its alias.
func (r *rgen) declType(t cc.Type) string { return r.rtype(t, true, "") }

func (r *rgen) rtype(t cc.Type, pretty bool, hint string) string {
	if t == nil {
		return "()"
	}
	if pretty {
		if td := t.Typedef(); td != nil {
			if a, ok := r.aliasOf[td]; ok {
				return a
			}
		}
	}
	switch t.Kind() {
	case cc.Void:
		return "()"
	case cc.Ptr:
		e := elemOf(t)
		switch {
		case e == nil || e.Kind() == cc.Void:
			return "*mut c_void"
		case e.Kind() == cc.Function:
			return r.fnType(e, pretty)
		}
		return "*mut " + r.rtype(e, pretty, "")
	case cc.Array:
		at := t.(*cc.ArrayType)
		n := at.Len()
		if n < 0 {
			n = 0
		}
		return fmt.Sprintf("[%s; %d]", r.rtype(at.Elem(), pretty, hint), n)
	case cc.Function:
		return r.fnType(t, pretty)
	case cc.Struct, cc.Union:
		return r.structName(t, hint)
	}
	k, ok := scalarKind(t)
	if !ok {
		panic(unsupported{"a type " + t.String()})
	}
	return rsKind(k)
}

// fnType is a pointer to a function of type t: an Option of an unsafe fn of
// the editor and the C's parameters.
func (r *rgen) fnType(t cc.Type, pretty bool) string {
	ft, ok := t.(*cc.FunctionType)
	if !ok {
		panic(unsupported{"a function type " + t.String()})
	}
	if ft.IsVariadic() {
		panic(unsupported{"a pointer to a variadic function"})
	}
	ps := []string{"*mut Editor"}
	for _, p := range ft.Parameters() {
		if p.Type() == nil || p.Type().Kind() == cc.Void {
			continue
		}
		pt := p.Type()
		if pt.Kind() == cc.Array {
			pt = pt.Decay()
		}
		ps = append(ps, r.rtype(pt, pretty, ""))
	}
	res := ""
	if rt := ft.Result(); rt != nil && rt.Kind() != cc.Void {
		res = " -> " + r.rtype(rt, pretty, "")
	}
	return "Option<unsafe fn(" + strings.Join(ps, ", ") + ")" + res + ">"
}

// typedefAliases names the typedefs of scalars, pointers and enumerations
// as Rust aliases, and a second name of a struct.
func (r *rgen) typedefAliases() {
	for tu := r.g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationDecl || ed.Declaration == nil {
			continue
		}
		for l := ed.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			if d == nil || !d.IsTypename() || d.Type() == nil {
				continue
			}
			t := d.Type()
			if strings.HasPrefix(d.Name(), "__") || t.Kind() == cc.Function || t.Kind() == cc.Array || t.Kind() == cc.Void {
				continue
			}
			if isAggr(t) {
				if r.structName(t, "") == rsName(d.Name()) {
					continue
				}
			} else if _, ok := scalarKind(t); !ok && t.Kind() != cc.Ptr {
				continue
			}
			n := rsName(d.Name())
			if r.structTaken[n] || r.aliases[n] != "" {
				continue
			}
			body := r.aliasBody(t, d)
			if body == "" {
				continue
			}
			r.aliases[n] = body
			r.aliasOrder = append(r.aliasOrder, n)
			r.aliasOf[d] = n
		}
	}
}

// aliasBody is what typedef d of type t stands for: its type, through the
// typedefs before it.
func (r *rgen) aliasBody(t cc.Type, d *cc.Declarator) (s string) {
	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(unsupported); !ok {
				panic(e)
			}
			s = ""
		}
	}()
	if isAggr(t) {
		return r.structName(t, "")
	}
	if t.Kind() == cc.Ptr {
		return r.rtype(t, true, "")
	}
	k, _ := scalarKind(t)
	return rsKind(k)
}

// typeDefs are the aliases and the structs and unions, each with every
// member, laid out as C lays it out.
func (r *rgen) typeDefs() string {
	var b strings.Builder
	for _, n := range r.aliasOrder {
		fmt.Fprintf(&b, "pub type %s = %s;\n", n, r.aliases[n])
	}
	if len(r.aliasOrder) > 0 {
		b.WriteString("\n")
	}
	// a member names the types of its own: the list grows as it is written
	for i := 0; i < len(r.structOrder); i++ {
		n := r.structOrder[i]
		b.WriteString(r.structDef(n, r.structType[n]))
	}
	return b.String()
}

// fields are a struct's or a union's members.
func fields(t cc.Type) []*cc.Field {
	var fs []*cc.Field
	switch x := t.(type) {
	case *cc.StructType:
		for i := 0; i < x.NumFields(); i++ {
			fs = append(fs, x.FieldByIndex(i))
		}
	case *cc.UnionType:
		for i := 0; i < x.NumFields(); i++ {
			fs = append(fs, x.FieldByIndex(i))
		}
	}
	return fs
}

func (r *rgen) structDef(n string, t cc.Type) string {
	kw := "struct"
	if t.Kind() == cc.Union {
		kw = "union"
	}
	fs := fields(t)
	var b strings.Builder
	fmt.Fprintf(&b, "#[repr(C)]\n#[derive(Clone, Copy)]\npub %s %s {\n", kw, n)
	if len(fs) == 0 {
		b.WriteString("    _opaque: [u8; 0],\n")
	}
	for _, f := range fs {
		if f.IsBitfield() {
			panic(unsupported{"a bit field in " + n})
		}
		if f.Name() == "" {
			panic(unsupported{"a member with no name in " + n})
		}
		fmt.Fprintf(&b, "    pub %s: %s,\n", rsName(f.Name()), r.constTy(memberSlot(f), r.rtype(f.Type(), true, n+"_"+f.Name())))
	}
	b.WriteString("}\n\n")
	return b.String()
}

// cSpelling is how C names a struct or union type at file scope: its
// typedef, or its tag; "" when it has neither.
func (r *rgen) cSpelling(t cc.Type) string {
	tag := aggTag(t)
	if td := r.tagTypedef[tag]; td != "" && tag != "" {
		return td
	}
	if t.Typedef() != nil {
		return t.Typedef().Name()
	}
	if tag == "" {
		return ""
	}
	if t.Kind() == cc.Union {
		return "union " + tag
	}
	return "struct " + tag
}

// layout lists every struct and union C can name -- its size, and each
// member's offset, through the members of a type that has no name of its
// own -- as the front end lays them out, which is gcc's:
//
//	size RUST C SIZE
//	offset RUST C PATH OFFSET
//
// a test prints the same with gcc's sizeof and offsetof and Rust's size_of
// and offset_of!, and requires the three to agree.
func (r *rgen) layout() string {
	var b strings.Builder
	var walk func(root, c, rpath, cpath string, t cc.Type, base int64)
	walk = func(root, c, rpath, cpath string, t cc.Type, base int64) {
		for _, f := range fields(t) {
			rp, cp := rsName(f.Name()), f.Name()
			if rpath != "" {
				rp, cp = rpath+"."+rp, cpath+"."+cp
			}
			fmt.Fprintf(&b, "offset %s %s %s %s %d\n", root, strings.ReplaceAll(c, " ", "~"), rp, cp, base+f.Offset())
			if ft := f.Type(); isAggr(ft) && r.cSpelling(ft) == "" {
				walk(root, c, rp, cp, ft, base+f.Offset())
			}
		}
	}
	for _, n := range r.structOrder {
		t := r.structType[n]
		c := r.cSpelling(t)
		if c == "" || len(fields(t)) == 0 {
			continue
		}
		if blockScope(t) {
			// a type of a function's own: C names it there and nowhere else
			c = "-"
		}
		fmt.Fprintf(&b, "size %s %s %d\n", n, strings.ReplaceAll(c, " ", "~"), t.Size())
		walk(n, c, "", "", t, 0)
	}
	return b.String()
}

// blockScope says a struct or a union is declared in a function, not at
// file scope.
func blockScope(t cc.Type) bool {
	var s *cc.Scope
	switch x := t.(type) {
	case *cc.StructType:
		s = x.LexicalScope()
	case *cc.UnionType:
		s = x.LexicalScope()
	}
	return s != nil && s.Parent != nil
}
