package togo

// ml_record.go is the OCaml backend's structs as records
// (doc/OCAML-IDIOMS.md, item 8): a struct the profile names (MlRecords)
// is an OCaml record of mutable fields, `type lineoff = { mutable lnum :
// int; ... }`, where whiml keeps every other struct in the editor's
// memory.  A local of the type is a record made where the function
// starts (Lineoff_T.make ()), where the C gave it room in the call's
// frame; a pointer to one is the record, its members fields
// (loff.lnum, loff.lnum <- 1), a copy of one a copy of its fields
// (copy_into), a clearing a clearing of them.
//
// A member whose address is taken, and a member that is a struct or an
// array, stay memory: the record holds the address of a block of its
// own, the struct's size, in the call's frame where the C's local was
// (`exarg_mem`), and those members' accessors read and write there, as
// every struct's do; copy_into and clear copy and clear the block too.
// A table of function pointers whose functions take a record is a table
// of its own (recordParams: fn_table_1v_0exarg), so the ex commands take
// the command's record.
//
// What makes that the C's meaning is that no pointer to the struct is
// ever memory: the backend refuses a struct the profile names that a
// file-scope or static object, an array, a member of another struct or a
// union holds, that a cast converts, that sizeof measures (an
// allocation) or the functions of bytes are handed but to clear it, that
// is passed or returned by value, that has a bit field, or whose pointers
// are compared (a record's equality would be its fields', not its
// address; and a pointer compared with NULL would be an option).  OCaml's
// type checker proves the rest: a pointer to one that meets an integer
// -- a null pointer, a store to memory -- fails the build.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// recordKey is a struct type's identity: its first member, which a
// typedef's clone of the type shares.
func recordKey(t cc.Type) string {
	if st, ok := t.(*cc.StructType); ok && st.NumFields() > 0 {
		return fmt.Sprintf("%p", st.FieldByIndex(0))
	}
	return ""
}

// record is the name of the struct t is when it is a record, "" else.
func (s *sgen) record(t cc.Type) string {
	if !s.ml || len(s.records) == 0 || t == nil {
		return ""
	}
	return s.records[recordKey(t)]
}

// recordClear is the record a call clears -- the functions of bytes'
// Set, memset(p, 0, n), p a pointer to one and n its size -- "" for any
// other call.
func (s *sgen) recordClear(x *cc.PostfixExpression) string {
	if !s.ml || len(s.records) == 0 || x.Case != cc.PostfixExpressionCall {
		return ""
	}
	d := fnDesignator(x.PostfixExpression)
	if d == nil || d.Name() != s.g.p.Bytes.Set {
		return ""
	}
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	if len(args) != 3 || args[0].Type() == nil || args[0].Type().Kind() != cc.Ptr {
		return ""
	}
	t := args[0].Type().(*cc.PointerType).Elem()
	r := s.record(t)
	c, ok1 := intValue(args[1].Value())
	n, ok2 := intValue(args[2].Value())
	if r == "" || !ok1 || !ok2 || c != 0 || n != t.Size() {
		return ""
	}
	return r
}

// records finds the structs the profile names and checks what OCaml's
// type checker cannot.  It fills s.records.
func (m *mlgen) records() (err error) {
	s := m.s
	want := map[string]bool{}
	for _, n := range s.g.p.MlRecords {
		want[n] = true
	}
	if len(want) == 0 {
		return nil
	}
	s.records = map[string]string{}
	s.recMem = map[string]map[string]bool{}
	s.recSize = map[string]int64{}
	found := map[string]cc.Type{}
	walkDecls(s.g.ast.TranslationUnit, func(d *cc.Declarator) {
		if d.IsTypename() && want[d.Name()] {
			if _, ok := d.Type().(*cc.StructType); ok {
				found[d.Name()] = d.Type()
			}
		}
	}, func(*cc.EnumType) {})
	var whys []string
	for _, n := range s.g.p.MlRecords {
		t := found[n]
		if t == nil {
			whys = append(whys, n+": not a struct the C declares by that name")
			continue
		}
		if s.structName(t) != n {
			whys = append(whys, n+": its members are named after "+s.structName(t))
			continue
		}
		s.recMem[n] = map[string]bool{}
		for _, fl := range scmFields(t) {
			if fl.IsBitfield() {
				whys = append(whys, n+": a bit field, "+fl.Name())
			}
			if isAggr(fl.Type()) || fl.Type().Kind() == cc.Array {
				s.recMem[n][fl.Name()] = true // in the record's memory
			}
		}
		s.records[recordKey(t)] = n
		s.recSize[recordKey(t)] = t.Size()
	}
	rec := func(t cc.Type) string { return s.record(t) }
	ptrTo := func(t cc.Type) string {
		if t != nil && t.Kind() == cc.Ptr {
			return rec(t.(*cc.PointerType).Elem())
		}
		return ""
	}
	// held: by a member, an array, a static object
	var holds func(t cc.Type, depth int) string
	holds = func(t cc.Type, depth int) string {
		for t != nil && depth < 8 {
			switch x := t.(type) {
			case *cc.ArrayType:
				if r := rec(x.Elem()); r != "" {
					return r
				}
				t = x.Elem()
			case *cc.PointerType:
				t = x.Elem()
			default:
				return ""
			}
			depth++
		}
		return ""
	}
	seenType := map[string]bool{}
	var walkType func(t cc.Type)
	walkType = func(t cc.Type) {
		for t != nil && (t.Kind() == cc.Ptr || t.Kind() == cc.Array) {
			if t.Kind() == cc.Ptr {
				t = t.(*cc.PointerType).Elem()
			} else {
				t = t.(*cc.ArrayType).Elem()
			}
		}
		k := fmt.Sprintf("%p", t)
		if t == nil || seenType[k] {
			return
		}
		seenType[k] = true
		fs := scmFields(t)
		_, union := t.(*cc.UnionType)
		for _, fl := range fs {
			ft := fl.Type()
			if r := rec(ft); r != "" {
				whys = append(whys, fmt.Sprintf("%s: a member of %s", r, s.structName(t)))
			}
			if r := ptrTo(ft); r != "" && (union || rec(t) == "") {
				whys = append(whys, fmt.Sprintf("%s: a pointer to one a member of %s", r, s.structName(t)))
			}
			if r := holds(ft, 0); r != "" {
				whys = append(whys, fmt.Sprintf("%s: an array of them a member of %s", r, s.structName(t)))
			}
			walkType(ft)
		}
	}
	walkDecls(s.g.ast.TranslationUnit, func(d *cc.Declarator) {
		t := d.Type()
		walkType(t)
		if d.IsTypename() {
			return
		}
		if r := holds(t, 0); r != "" {
			whys = append(whys, fmt.Sprintf("%s: an array of them, %s", r, d.Name()))
		}
		if r := rec(t); r != "" && d.IsStatic() {
			whys = append(whys, fmt.Sprintf("%s: a static object, %s", r, d.Name()))
		}
		if ft, ok := t.(*cc.FunctionType); ok {
			if r := rec(ft.Result()); r != "" {
				whys = append(whys, fmt.Sprintf("%s: returned by value from %s", r, d.Name()))
			}
			for _, p := range ft.Parameters() {
				if r := rec(p.Type()); r != "" {
					whys = append(whys, fmt.Sprintf("%s: passed by value to %s", r, d.Name()))
				}
			}
		}
	}, func(*cc.EnumType) {})
	bytesFn := map[string]bool{s.g.p.Bytes.Set: true, s.g.p.Bytes.Cmp: true}
	for _, n := range s.g.p.Bytes.Move {
		bytesFn[n] = true
	}
	var walk func(n cc.Node)
	walk = func(n cc.Node) {
		switch x := n.(type) {
		case *cc.UnaryExpression:
			switch x.Case {
			case cc.UnaryExpressionAddrof:
				if p, ok := unparenE(x.CastExpression).(*cc.PostfixExpression); ok && (p.Case == cc.PostfixExpressionSelect || p.Case == cc.PostfixExpressionPSelect) {
					if t := p.PostfixExpression.Type(); t != nil {
						if r := rec(t) + ptrTo(t); r != "" {
							// the member lives in the record's memory
							s.recMem[r][p.Field().Name()] = true
						}
					}
				}
			case cc.UnaryExpressionSizeofExpr:
				if t := x.UnaryExpression.Type(); t != nil {
					if r := rec(t); r != "" {
						whys = append(whys, fmt.Sprintf("%s: measured at %v", r, x.Position()))
					}
				}
			case cc.UnaryExpressionSizeofType:
				if t := x.TypeName.Type(); t != nil {
					if r := rec(t); r != "" {
						whys = append(whys, fmt.Sprintf("%s: measured at %v", r, x.Position()))
					}
				}
			}
		case *cc.CastExpression:
			if x.Case == cc.CastExpressionCast {
				if r := ptrTo(x.Type()) + ptrTo(x.CastExpression.Type()); r != "" {
					whys = append(whys, fmt.Sprintf("%s: a cast at %v", r, x.Position()))
				}
			}
		case *cc.EqualityExpression:
			if r := ptrTo(x.EqualityExpression.Type()) + ptrTo(x.RelationalExpression.Type()); r != "" {
				whys = append(whys, fmt.Sprintf("%s: pointers compared at %v", r, x.Position()))
			}
		case *cc.RelationalExpression:
			if r := ptrTo(x.RelationalExpression.Type()) + ptrTo(x.ShiftExpression.Type()); r != "" {
				whys = append(whys, fmt.Sprintf("%s: pointers ordered at %v", r, x.Position()))
			}
		case *cc.PostfixExpression:
			if s.recordClear(x) != "" {
				return // a record cleared, which the printer writes so
			}
			if x.Case == cc.PostfixExpressionCall {
				if p, ok := unparenE(x.PostfixExpression).(*cc.PrimaryExpression); ok && bytesFn[p.Token.SrcStr()] {
					for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
						if r := ptrTo(l.AssignmentExpression.Type()); r != "" {
							whys = append(whys, fmt.Sprintf("%s: its bytes handed to %s at %v", r, p.Token.SrcStr(), x.Position()))
						}
					}
				}
			}
		}
		walkChildrenFn(n, walk)
	}
	walk(s.g.ast.TranslationUnit)
	if len(whys) > 0 {
		sort.Strings(whys)
		return fmt.Errorf("ml: a struct the profile names a record is memory: %s", strings.Join(whys, "; "))
	}
	return nil
}

// mlRecord is a record's OCaml: its type and its fields, in the C's order.
type mlRecord struct {
	c, typ, module string
	fields         []*mlMember // the members that are fields
	size           int64       // the C's size: a record's memory's
}

// recordTypes are the records' declarations, the fields the functions
// name: the type at the top (the interface's too), and in the struct's
// module the making, the clearing and the copy.
func (m *mlgen) recordTypes() string {
	recs := m.recordList()
	if len(recs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("(* The C's structs no byte of which is ever memory: records. *)\n")
	for _, r := range recs {
		var fs []string
		for _, f := range r.fields {
			fs = append(fs, fmt.Sprintf("mutable %s : %s", f.read, m.memberType(f)))
		}
		if m.s.recordMem(r.c) {
			fs = append(fs, r.typ+"_mem : int")
		}
		fmt.Fprintf(&b, "type %s = { %s }\n", r.typ, strings.Join(fs, "; "))
	}
	return b.String() + "\n"
}

// memberType is the OCaml type of a record's field.
func (m *mlgen) memberType(f *mlMember) string {
	if f.variant != nil {
		return f.variant.typ
	}
	return mlKindType(scmKindOf(f.kind))
}

// memberZero is a field's value in a record made: C's zero.
func (m *mlgen) memberZero(f *mlMember) string {
	if f.variant != nil {
		if c, ok := f.variant.byVal[0]; ok {
			return c
		}
		panic(unsupported{"a record's field of a variant with no zero: " + f.read})
	}
	if f.kind == "bool" {
		return "false"
	}
	return "0"
}

// recordModules are the records' functions, in each struct's module.
func (m *mlgen) recordModule(r *mlRecord) []string {
	var sets, inits, copies []string
	for _, f := range r.fields {
		inits = append(inits, fmt.Sprintf("%s = %s", f.read, m.memberZero(f)))
		sets = append(sets, fmt.Sprintf("r.%s <- %s", f.read, m.memberZero(f)))
		copies = append(copies, fmt.Sprintf("d.%s <- s.%s", f.read, f.read))
	}
	var out []string
	if mem := r.typ + "_mem"; m.s.recordMem(r.c) {
		// the members in memory: the record's own block, in the call's
		// frame where the C's local is
		inits = append(inits, mem+" = mem")
		sets = append(sets, fmt.Sprintf("mem_zero ed r.%s %d", mem, r.size))
		copies = append(copies, fmt.Sprintf("mem_copy ed d.%s s.%s %d", mem, mem, r.size))
		if m.used[r.module+".make"] {
			out = append(out, fmt.Sprintf("  let make mem = { %s }\n", strings.Join(inits, "; ")))
		}
		if m.used[r.module+".clear"] {
			out = append(out, fmt.Sprintf("  let clear ed r = %s\n", strings.Join(sets, "; ")))
		}
		if m.used[r.module+".copy_into"] {
			out = append(out, fmt.Sprintf("  let copy_into ed d s = %s\n", strings.Join(copies, "; ")))
		}
		return out
	}
	if m.used[r.module+".make"] {
		out = append(out, fmt.Sprintf("  let make () = { %s }\n", strings.Join(inits, "; ")))
	}
	if m.used[r.module+".clear"] {
		out = append(out, fmt.Sprintf("  let clear r = %s\n", strings.Join(sets, "; ")))
	}
	if m.used[r.module+".copy_into"] {
		out = append(out, fmt.Sprintf("  let copy_into d s = %s\n", strings.Join(copies, "; ")))
	}
	return out
}

// recordList is the records, each with the members the functions name.
func (m *mlgen) recordList() []*mlRecord {
	if m.recs != nil {
		return m.recs
	}
	byName := map[string]*mlRecord{}
	var names []string
	for _, n := range m.s.records {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		byName[n] = &mlRecord{c: n, typ: mlVariantType(n), module: m.modules[n]}
	}
	for n, mm := range m.members {
		st, _, _ := strings.Cut(n, ".")
		if r := byName[st]; r != nil && mm.record {
			r.fields = append(r.fields, mm)
		}
	}
	for k, n := range m.s.records {
		byName[n].size = m.s.recSize[k]
	}
	for _, n := range names {
		r := byName[n]
		sort.Slice(r.fields, func(i, j int) bool { return r.fields[i].off < r.fields[j].off })
		if len(r.fields) > 0 || m.s.recordMem(n) {
			m.recs = append(m.recs, r)
		}
	}
	return m.recs
}

// recordMem says the record named r keeps some of its members in memory
// of its own: an aggregate, an array, a member whose address is taken.
func (s *sgen) recordMem(r string) bool { return len(s.recMem[r]) > 0 }

// recordParams is what a function type adds to the kind of the table of
// function pointers its functions are in (ml.go): _N and the record's type
// for each parameter N that is a pointer to a record -- so that the ex
// commands, which take the command's record, are a table of their own.
func (s *sgen) recordParams(ft *cc.FunctionType) string {
	if !s.ml || len(s.records) == 0 {
		return ""
	}
	var b strings.Builder
	i := 0
	for _, p := range ft.Parameters() {
		t := p.Type()
		if t == nil || t.Kind() == cc.Void {
			continue
		}
		if t.Kind() == cc.Ptr {
			if r := s.record(t.(*cc.PointerType).Elem()); r != "" {
				fmt.Fprintf(&b, "_%d%s", i, mlVariantType(r))
			}
		}
		i++
	}
	return b.String()
}
