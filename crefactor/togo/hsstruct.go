package togo

// hsstruct.go is how the Haskell prints struct locals as values
// (doc/HASKELL-IDIOMS.md, item 9): which locals are values, and which
// functions return a struct of scalars as its members, is decided in
// structvalues.go, with the Scheme backend; here each such local is one
// binding per member -- `pos_T a = *pp; a.col += dc; *pp = a;` reads the
// members into a_lnum and a_col, adds to a_col and writes them back -- and
// a struct result a tuple.

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// structVars gives each struct local that is a value its members' variables.
func (f *hfn) structVars() {
	f.sv = map[*lvar][]*lvar{}
	f.svOf = map[*lvar]*lvar{}
	for _, v := range f.lf.vars {
		if v.decl == nil || !f.h.sval[v.decl] {
			continue
		}
		st := v.c.(*cc.StructType)
		for i := 0; i < st.NumFields(); i++ {
			fl := st.FieldByIndex(i)
			m := &lvar{name: v.name + "_" + fl.Name(), c: fl.Type()}
			f.sv[v] = append(f.sv[v], m)
			f.svOf[m] = v
			f.extra = append(f.extra, m)
		}
	}
}

// memberVar is the member variable e is -- s.m of a struct that is a value
// -- or nil.
func (f *hfn) memberVar(e cc.ExpressionNode) *lvar {
	if len(f.sv) == 0 {
		return nil
	}
	x, ok := unparenE(e).(*cc.PostfixExpression)
	if !ok || x.Case != cc.PostfixExpressionSelect {
		return nil
	}
	s := f.structNamed(x.PostfixExpression)
	if s == nil {
		return nil
	}
	for i, m := range f.sv[s] {
		if f.fieldOf(s, i).Name() == x.Field().Name() {
			return m
		}
	}
	f.no(x, "a member %s of a struct that is a value", x.Field().Name())
	return nil
}

// structNamed is the struct that is a value e names, or nil.
func (f *hfn) structNamed(e cc.ExpressionNode) *lvar {
	p, ok := unparenE(e).(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return nil
	}
	d, ok := p.ResolvedTo().(*cc.Declarator)
	if !ok {
		return nil
	}
	if v := f.lf.byDecl[d]; v != nil && f.sv[v] != nil {
		return v
	}
	return nil
}

// fieldOf is member i of struct s's type.
func (f *hfn) fieldOf(s *lvar, i int) *cc.Field {
	return s.c.(*cc.StructType).FieldByIndex(i)
}

// copyStruct gives struct s the value x: another struct that is a value
// (x.rec), or the struct at the address x is, member by member.
func (f *hfn) copyStruct(s *lvar, x hv) {
	if x.tup != nil {
		f.lines = append(f.lines, x.binds...)
		for i, m := range f.sv[s] {
			f.setVar(m, hv{val: x.tup[i], ht: f.vtype(m)})
		}
		return
	}
	if x.rec != nil {
		for i, m := range f.sv[s] {
			f.setVar(m, f.varRead(f.sv[x.rec][i]))
		}
		return
	}
	src := f.flush(x)
	for i, m := range f.sv[s] {
		fl := f.fieldOf(s, i)
		a := f.member(haddr{base: hv{val: src, ht: "P"}}, s.c, fl)
		f.setVar(m, f.readAt(a, fl.Type()))
	}
}

// storeStruct writes struct s, a value, member by member at the address
// base plus off (a's).
func (f *hfn) storeStruct(a haddr, s *lvar) {
	// the address once, as C computes it once
	a.base = hv{val: f.flush(a.base), ht: "P"}
	for i, m := range f.sv[s] {
		fl := f.fieldOf(s, i)
		ma := f.member(a, s.c, fl)
		base := f.flush(ma.base)
		ht := f.h.hsType(fl.Type())
		val := f.flushPlain(f.conv(f.varRead(m), ht))
		f.emit("wr%s %s %s %s", hsAccess(ht), base, offStr(ma), val)
	}
}

// initStruct is struct s's initializer where C declares it: every member
// zero, then the initializer's.
func (f *hfn) initStruct(s *lvar, in *cc.Initializer) {
	if in != nil && in.Case == cc.InitializerExpr {
		f.copyStruct(s, f.expr(in.AssignmentExpression))
		return
	}
	vals := map[int64]hv{}
	var walk func(in *cc.Initializer)
	walk = func(in *cc.Initializer) {
		if in == nil {
			return
		}
		if in.Case == cc.InitializerExpr {
			vals[in.Offset()] = f.expr(in.AssignmentExpression)
			return
		}
		for l := in.InitializerList; l != nil; l = l.InitializerList {
			walk(l.Initializer)
		}
	}
	walk(in)
	for i, m := range f.sv[s] {
		fl := f.fieldOf(s, i)
		if x, ok := vals[fl.Offset()]; ok {
			f.setVar(m, x)
		} else {
			f.setVar(m, hv{val: hsZero(f.vtype(m)), ht: f.vtype(m), konst: true})
		}
	}
}

// tupleType is a struct of scalars' members' types as a tuple type.
func (h *hgen) tupleType(t cc.Type) string {
	st := t.(*cc.StructType)
	var ts []string
	for i := 0; i < st.NumFields(); i++ {
		ts = append(ts, h.sigType(st.FieldByIndex(i).Type()))
	}
	return "(" + strings.Join(ts, ", ") + ")"
}

// structVals are the members' values of x, a struct value of type t: a
// tuple's names, a struct that is a value's members, or the members read
// from the address x is.
func (f *hfn) structVals(x hv, t cc.Type) []string {
	st := t.(*cc.StructType)
	var out []string
	switch {
	case x.tup != nil:
		f.lines = append(f.lines, x.binds...) // the call that binds it
		return x.tup
	case x.rec != nil:
		for i, m := range f.sv[x.rec] {
			fl := st.FieldByIndex(i)
			out = append(out, f.flushPlain(f.conv(f.varRead(m), f.h.hsType(fl.Type()))))
		}
		return out
	}
	src := f.flush(x)
	for i := 0; i < st.NumFields(); i++ {
		fl := st.FieldByIndex(i)
		a := f.member(haddr{base: hv{val: src, ht: "P"}}, t, fl)
		out = append(out, f.flush(f.readAt(a, fl.Type())))
	}
	return out
}

// materialize is a struct value in memory: a tuple written into the frame,
// for what takes a struct's address.
func (f *hfn) materialize(x hv, t cc.Type) hv {
	if x.tup == nil {
		return x
	}
	binds := append([]string{}, x.binds...) // the call that binds the tuple, first
	off := f.alloc(nil, t)
	base := fmt.Sprintf("(pAdd fr' %d)", off)
	st := t.(*cc.StructType)
	for i, v := range x.tup {
		fl := st.FieldByIndex(i)
		a := f.member(haddr{base: hv{val: base, ht: "P"}}, t, fl)
		ht := f.h.hsType(fl.Type())
		binds = append(binds, fmt.Sprintf("wr%s %s %s %s", hsAccess(ht), base, offStr(a), v))
	}
	return hv{binds: binds, val: base, ht: "agg"}
}
