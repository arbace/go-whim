package togo

// ml_enum.go is the OCaml backend's enumerations as variants
// (doc/OCAML-IDIOMS.md, item 7): a C enumeration the profile names
// (MlVariants) is an OCaml type of constant constructors, `type
// paste_mode = Paste_insert | Paste_cmdline | ...`, its enumerators the
// constructors wherever the C names them -- a value, a comparison, a case
// label, a field's initial value.  OCaml's type checker is the proof that
// no value of the type meets an integer: a store to memory, an
// arithmetic, a conversion or a test of a number fails the build.  What
// it does not see is an order, which OCaml's polymorphic compare would
// give a variant: so the backend refuses an enumeration that a
// relational operator in the C compares, and one whose object lives in
// memory -- a file-scope object that is not a field of the state.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// mlEnum is a C enumeration printed as a variant.
type mlEnum struct {
	c, typ string           // the C's name; the OCaml type's
	ctors  []string         // the constructors, in the C's order
	byVal  map[int64]string // a value -> its constructor
	first  *cc.Enumerator   // the enumeration's identity

	needOf, needTo bool // a read of memory converts to it; a store from it
}

// variants finds the enumerations the profile names and checks what OCaml
// cannot: the relational operators.  It fills m.variant (a Scheme
// enumerator -> its enumeration) and m.venums.
func (m *mlgen) variants() (err error) {
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(unsupported)
			if !ok {
				panic(r)
			}
			err = fmt.Errorf("ml: %s", u.why)
		}
	}()
	want := map[string]bool{}
	for _, n := range m.s.g.p.MlVariants {
		want[n] = true
	}
	if len(want) == 0 {
		return nil
	}
	found := map[string]*cc.EnumType{}
	walkDecls(m.s.g.ast.TranslationUnit, func(d *cc.Declarator) {
		if e, ok := d.Type().(*cc.EnumType); ok && d.IsTypename() && want[d.Name()] {
			found[d.Name()] = e
		}
	}, func(e *cc.EnumType) {
		if tk := e.Tag(); tk.SrcStr() != "" && want[tk.SrcStr()] {
			t := tk.SrcStr()
			found[t] = e
		}
	})
	owner := map[*cc.Enumerator]*mlEnum{}
	var names []string
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		e := found[n]
		ens := e.Enumerators()
		if len(ens) < 2 {
			panic(unsupported{"a variant of fewer than two enumerators: " + n})
		}
		me := &mlEnum{c: n, typ: mlVariantType(n), byVal: map[int64]string{}, first: ens[0]}
		seen := map[string]bool{}
		for _, en := range ens {
			c := mlCtor(en.Token.SrcStr())
			for seen[c] {
				c += "'"
			}
			seen[c] = true
			v, ok := enumInt(en)
			if !ok {
				panic(unsupported{"an enumerator whose value is not an integer: " + en.Token.SrcStr()})
			}
			if _, dup := me.byVal[v]; dup {
				panic(unsupported{"a variant two of whose enumerators have one value: " + n})
			}
			me.byVal[v] = c
			me.ctors = append(me.ctors, c)
			owner[en] = me
			m.variant[scmName(en.Token.SrcStr())] = me
			m.ctorOf[scmName(en.Token.SrcStr())] = c
		}
		m.venums = append(m.venums, me)
	}
	for _, n := range m.s.g.p.MlVariants {
		if found[n] == nil {
			panic(unsupported{"a variant the C does not declare: " + n})
		}
	}
	// the orders: an operand of a relational operator that is an
	// enumerator of one, or of its type
	byType := func(t cc.Type) *mlEnum {
		if e, ok := t.(*cc.EnumType); ok && len(e.Enumerators()) > 0 {
			return owner[e.Enumerators()[0]]
		}
		return nil
	}
	of := func(x cc.ExpressionNode) *mlEnum {
		for {
			p, ok := x.(*cc.PrimaryExpression)
			if !ok {
				break
			}
			if p.Case == cc.PrimaryExpressionExpr {
				x = p.ExpressionList
				continue
			}
			if en, ok := p.ResolvedTo().(*cc.Enumerator); ok {
				return owner[en]
			}
			break
		}
		if x == nil || x.Type() == nil {
			return nil
		}
		return byType(x.Type())
	}
	var whys []string
	var rec func(cc.Node)
	rec = func(n cc.Node) {
		if r, ok := n.(*cc.RelationalExpression); ok {
			for _, x := range []cc.ExpressionNode{r.RelationalExpression, r.ShiftExpression} {
				if me := of(x); me != nil {
					whys = append(whys, fmt.Sprintf("%s ordered at %v", me.c, r.Position()))
				}
			}
		}
		walkChildrenFn(n, rec)
	}
	rec(m.s.g.ast.TranslationUnit)
	if len(whys) > 0 {
		panic(unsupported{"a variant OCaml would order by its constructors: " + strings.Join(whys, "; ")})
	}
	return nil
}

// enumInt is an enumerator's value.
func enumInt(en *cc.Enumerator) (int64, bool) {
	switch v := en.Value().(type) {
	case cc.Int64Value:
		return int64(v), true
	case cc.UInt64Value:
		return int64(v), true
	}
	return 0, false
}

// mlVariantType is a C enumeration's name as an OCaml type's: lowered,
// its _T dropped.
func mlVariantType(c string) string {
	n := strings.ToLower(strings.TrimSuffix(c, "_T"))
	if mlReserved[n] {
		n += "_"
	}
	return n
}

// mlCtor is a C enumerator's name as a constructor's: its first letter a
// capital, the rest as the C spells them but lowered where the C's are all
// capitals (MAGIC_NONE Magic_none, CMD_Next Cmd_Next).
func mlCtor(c string) string {
	r := c
	if strings.ToUpper(c) == c {
		r = strings.ToLower(c)
	}
	return strings.ToUpper(r[:1]) + r[1:]
}

// variantOfObject is the variant a file-scope object of Scheme key's type
// is, nil for none.
func (m *mlgen) variantOfObject(key string) *mlEnum { return m.variantOfType(m.s.facts.segType[key]) }

// variantOfType is the variant t is, nil for none.
func (m *mlgen) variantOfType(t cc.Type) *mlEnum {
	if e, ok := t.(*cc.EnumType); ok && len(e.Enumerators()) > 0 {
		for _, me := range m.venums {
			if me.first == e.Enumerators()[0] {
				return me
			}
		}
	}
	return nil
}

// variantTypes is the variants' declarations.
func (m *mlgen) variantTypes() string {
	if len(m.venums) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("(* The C's enumerations whose values are only named, compared and matched:\n   variants. *)\n")
	for _, me := range m.venums {
		fmt.Fprintf(&b, "type %s = %s\n", me.typ, strings.Join(me.ctors, " | "))
	}
	return b.String() + "\n"
}

// variantNeeds marks the conversions the accessors of memory use: a
// member or an object in memory of a variant's type is its number there,
// converted where it is read and written.
func (m *mlgen) variantNeeds() {
	for _, mm := range m.members {
		if mm.variant != nil && mm.kind != "agg" {
			mm.variant.needOf = mm.variant.needOf || m.used[mm.module+"."+mm.read]
			mm.variant.needTo = mm.variant.needTo || m.used[mm.module+"."+mm.set]
		}
	}
	for _, ob := range m.objOf {
		if ob.variant != nil && !ob.field {
			ob.variant.needOf = ob.variant.needOf || m.used[ob.read]
			ob.variant.needTo = ob.variant.needTo || m.used[ob.set]
		}
	}
}

// variantConvs are the conversions between a variant and the number the
// C stores, where memory holds one: a number no enumerator has fails.
func (m *mlgen) variantConvs() string {
	var b strings.Builder
	for _, me := range m.venums {
		var vs []int64
		for v := range me.byVal {
			vs = append(vs, v)
		}
		sort.Slice(vs, func(i, j int) bool { return vs[i] < vs[j] })
		if me.needOf {
			fmt.Fprintf(&b, "let %s_of_int = function\n", me.typ)
			for _, v := range vs {
				fmt.Fprintf(&b, "  | %s -> %s\n", mlIntText(v), me.byVal[v])
			}
			fmt.Fprintf(&b, "  | _ -> failwith \"not a %s\"\n", me.c)
		}
		if me.needTo {
			fmt.Fprintf(&b, "let int_of_%s = function\n", me.typ)
			for _, v := range vs {
				fmt.Fprintf(&b, "  | %s -> %s\n", me.byVal[v], mlIntText(v))
			}
		}
		if me.needOf || me.needTo {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// rest are the constructors a match's arms do not name, in the C's order:
// its other arm's pattern, which a wildcard would make fragile.
func (me *mlEnum) rest(covered map[string]bool) []string {
	var out []string
	for _, c := range me.ctors {
		if !covered[c] {
			out = append(out, c)
		}
	}
	return out
}
