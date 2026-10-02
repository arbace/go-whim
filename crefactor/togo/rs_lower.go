package togo

// rs_lower.go is a function Rust's labeled blocks cannot say -- a goto back,
// or into a statement, a case label inside a statement of its switch --
// printed from the lowered form (lower.go: basic blocks), as the Clojure and
// the Haskell backends print every function: a loop over a match on the
// block's number, each arm a block's steps and the number of the next.
// The core has none such; a C program that has is still translated.

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// structuredOnly are the refusals of the structured printer the lowered
// form takes instead.
var structuredOnly = []string{"a goto that is no forward jump", "a case label inside a statement of its switch"}

// lowerable says a refusal is one the lowered form does not share.
func lowerable(why string) bool {
	for _, s := range structuredOnly {
		if strings.Contains(why, s) {
			return true
		}
	}
	return false
}

// lowered prints f's function from its lowered form.
func (f *rfn) lowered() {
	lf := lowerFunction(f.fd, f.r.g.a, f.r.g.p, rsName)
	f.lowered_ = true
	f.sub = lf.sub
	f.vars = map[*lvar]*rlocal{}
	for _, v := range lf.vars {
		if v.decl != nil {
			if l := f.local[v.decl]; l != nil {
				f.vars[v] = l
				continue
			}
		}
		n := f.temp(v.c)
		for _, l := range f.order {
			if l.name == n {
				if v.boolean {
					l.rty = "bool"
				}
				f.vars[v] = l
			}
		}
	}
	num := map[*lblock]int{}
	for i, b := range lf.blocks {
		num[b] = i
	}
	f.line("let mut blk: u32 = %d;", num[lf.blocks[0]])
	f.line("loop {")
	f.ind++
	f.line("match blk {")
	f.ind++
	for _, b := range lf.blocks {
		f.line("%d => {", num[b])
		f.ind++
		for _, s := range b.steps {
			f.lstep(s)
		}
		f.lterm(b.term, num)
		f.ind--
		f.line("}")
	}
	f.line("_ => unreachable!(),")
	f.ind--
	f.line("}")
	f.ind--
	f.line("}")
}

// lexpr is a value of the lowered form: a variable, or a C node read
// through the substitutions (raw: the node itself, its operands through
// them).
func (f *rfn) lexpr(r lexpr) rv {
	if r.v != nil {
		l := f.vars[r.v]
		ty := l.rty
		if ty == "" {
			ty = f.vty(r.v.c)
			if r.v.c.Kind() == cc.Array {
				return f.decay(l.name, r.v.c)
			}
		}
		l.read = true
		return rv{s: l.name, ty: ty}
	}
	if r.raw {
		old := f.skip
		f.skip = r.n
		defer func() { f.skip = old }()
	}
	return f.expr(r.n)
}

// lstep prints a step of the lowered form.
func (f *rfn) lstep(s lstep) {
	switch s.op {
	case opSet:
		l := f.vars[s.dst]
		v := f.lexpr(s.e)
		switch {
		case l.rty == "bool":
			f.line("%s = %s;", l.name, unparenRs(f.truth(v)))
		default:
			f.line("%s = %s;", l.name, unparenRs(f.conv(v, f.r.ty(s.dst.c)).s))
		}
		l.mutated = true
	case opAssign:
		v := f.lexpr(s.e)
		lv := f.lval(s.lhs)
		f.markWrite(s.lhs)
		f.line("%s = %s;", lv.place, unparenRs(f.conv(v, lv.ty).s))
	case opAssignOp:
		lv := f.lval(s.lhs)
		f.markWrite(s.lhs)
		r := f.lexpr(s.e)
		nv := f.compound(lv, s.aop, r, s.e.typeOf(), s.at)
		f.line("%s = %s;", lv.place, unparenRs(f.conv(nv, lv.ty).s))
	case opIncDec:
		f.incDecStmt(s.lhs, s.inc)
	case opEval:
		v := f.lexpr(s.e)
		if strings.HasSuffix(v.s, ")") && v.prec == pPrim {
			f.line("%s;", v.s)
		} else {
			f.line("let _ = %s;", unparenRs(v.s))
		}
	case opInit:
		l := f.vars[s.dst]
		l.mutated = true
		f.initLocal(l, s.in)
	default:
		f.no(s.at, "a step %d", s.op)
	}
}

// lterm prints a block's terminator: the next block's number, or the
// return.
func (f *rfn) lterm(t lterm, num map[*lblock]int) {
	switch t.kind {
	case tGoto:
		f.line("blk = %d;", num[t.to[0]])
	case tIf:
		c := unparenRs(f.truth(f.lexpr(t.cond)))
		f.line("blk = if %s { %d } else { %d };", c, num[t.to[0]], num[t.to[1]])
	case tSwitch:
		x := f.lexpr(t.cond)
		ty := x.ty
		if ty == "bool" {
			ty = "i32"
		}
		if isIntTy(ty) {
			ty = promoteTy(ty)
		}
		f.line("blk = match %s {", unparenRs(f.conv(x, ty).s))
		for i, vs := range t.cases {
			f.line("    %s => %d,", rsPattern(vs, ty), num[t.to[i]])
		}
		f.line("    _ => %d,", num[t.to[len(t.to)-1]])
		f.line("};")
	case tRet:
		switch {
		case t.ret.isZero() && f.ret != nil:
			f.line("return %s;", f.zero(f.ret))
		case t.ret.isZero():
			f.line("return;")
		default:
			v := f.conv(f.lexpr(t.ret), f.r.ty(f.ret))
			f.line("return %s;", unparenRs(v.s))
		}
	case tFall:
		if f.ret != nil {
			f.line("return %s;", f.zero(f.ret))
		} else {
			f.line("return;")
		}
	default:
		f.no(t.at, "a terminator %d", t.kind)
	}
}

var _ = fmt.Sprint
