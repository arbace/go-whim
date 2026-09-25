package togo

// lower_c.go prints the lowered form back as C: each function's body
// replaced by its blocks -- every variable declared at the top, each block a
// label, each step a statement, each terminator a goto, an if of two gotos,
// a switch of gotos or a return.  A C compiler then says whether the
// lowering kept what the function does: the test compiles the printed
// program and requires it to print what the original prints.

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// writeLoweredC writes the translation unit at src with every function
// definition's body replaced by its lowered form, printed as C.
func (g *gen) writeLoweredC(src, path string) error {
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	type splice struct {
		from, to int
		body     string
	}
	var sps []splice
	failed := 0
	for tu := g.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed.Case != cc.ExternalDeclarationFuncDef {
			continue
		}
		fd := ed.FunctionDefinition
		cs := fd.CompoundStatement
		if cs.Token.Position().Filename != src {
			continue
		}
		body, why := g.loweredC(fd)
		if why != "" {
			fmt.Fprintf(logw, "lower: %s: %s\n", fd.Declarator.Name(), why)
			failed++
			continue
		}
		sps = append(sps, splice{cs.Token.Position().Offset, cs.Token2.Position().Offset + 1, body})
	}
	sort.Slice(sps, func(a, b int) bool { return sps[a].from < sps[b].from })
	var b strings.Builder
	at := 0
	for _, s := range sps {
		b.Write(text[at:s.from])
		b.WriteString(s.body)
		at = s.to
	}
	b.Write(text[at:])
	if failed > 0 {
		fmt.Fprintf(logw, "lower: %d functions not lowered\n", failed)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// loweredC is one function's body, lowered and printed as C.
func (g *gen) loweredC(fd *cc.FunctionDefinition) (body string, why string) {
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(unsupported)
			if !ok {
				panic(r)
			}
			body, why = "", u.why
		}
	}()
	f := lowerFunction(fd, g.a, g.p, nil)
	return f.printC(), ""
}

// printC is the function's body as C.
func (f *lfn) printC() string {
	var b strings.Builder
	b.WriteString("{\n")
	// the types the body declares: its struct, union and enum definitions
	// and its typedefs, first
	var types func(cc.Node)
	types = func(n cc.Node) {
		if n == nil {
			return
		}
		if d, ok := n.(*cc.Declaration); ok && d.Case == cc.DeclarationDecl {
			allTypedef := d.InitDeclaratorList != nil
			for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
				if !l.InitDeclarator.Declarator.IsTypename() {
					allTypedef = false
				}
			}
			if allTypedef {
				fmt.Fprintf(&b, "    %s\n", cc.NodeSource(d))
				return
			}
		}
		switch x := n.(type) {
		case *cc.StructOrUnionSpecifier:
			if x.Case == cc.StructOrUnionSpecifierDef {
				fmt.Fprintf(&b, "    %s;\n", cc.NodeSource(x))
				return
			}
		case *cc.EnumSpecifier:
			if x.EnumeratorList != nil {
				fmt.Fprintf(&b, "    %s;\n", cc.NodeSource(x))
				return
			}
		}
		walkChildrenFn(n, types)
	}
	types(f.fd.CompoundStatement)
	for _, d := range f.statics {
		init := ""
		if in := f.staticInit(d); in != nil {
			init = " = " + f.cInit(in)
		}
		fmt.Fprintf(&b, "    static %s%s;\n", cDecl(d.Type(), d.Name()), init)
	}
	for _, v := range f.vars {
		if v.param {
			continue
		}
		fmt.Fprintf(&b, "    %s;\n", cDecl(v.c, v.name))
	}
	if rt := f.ft.Result(); rt != nil && rt.Kind() != cc.Void {
		fmt.Fprintf(&b, "    %s;\n", cDecl(rt, "fall_"))
	}
	for _, bl := range f.blocks {
		fmt.Fprintf(&b, "b%d: ; /* %s */\n", bl.id, bl.label)
		for _, s := range bl.steps {
			b.WriteString("    " + f.cStep(s) + "\n")
		}
		b.WriteString("    " + f.cTerm(bl.term) + "\n")
	}
	b.WriteString("}")
	return b.String()
}

// staticInit is a block-scope static's initializer.
func (f *lfn) staticInit(d *cc.Declarator) *cc.Initializer {
	for _, s := range f.a.statics {
		if s.d == d {
			return s.init
		}
	}
	return nil
}

func (f *lfn) cStep(s lstep) string {
	switch s.op {
	case opSet:
		if s.dst.boolean {
			return fmt.Sprintf("%s = (%s) != 0;", s.dst.name, f.cLexpr(s.e))
		}
		return fmt.Sprintf("%s = %s;", s.dst.name, f.cLexpr(s.e))
	case opAssign:
		return fmt.Sprintf("%s = %s;", f.cExpr(s.lhs), f.cLexpr(s.e))
	case opAssignOp:
		return fmt.Sprintf("%s %s= %s;", f.cExpr(s.lhs), s.aop, f.cLexpr(s.e))
	case opIncDec:
		if s.inc {
			return f.cExpr(s.lhs) + "++;"
		}
		return f.cExpr(s.lhs) + "--;"
	case opEval:
		return fmt.Sprintf("(void)(%s);", f.cLexpr(s.e))
	case opInit:
		return fmt.Sprintf("{ %s = %s; __builtin_memcpy(&%s, &init_, sizeof(%s)); }", cDecl(s.dst.c, "init_"), f.cInit(s.in), s.dst.name, s.dst.name)
	}
	return "/* ? */"
}

func (f *lfn) cTerm(t lterm) string {
	switch t.kind {
	case tGoto:
		return fmt.Sprintf("goto b%d;", t.to[0].id)
	case tIf:
		return fmt.Sprintf("if (%s) goto b%d; else goto b%d;", f.cLexpr(t.cond), t.to[0].id, t.to[1].id)
	case tSwitch:
		var b strings.Builder
		fmt.Fprintf(&b, "switch (%s) {", f.cLexpr(t.cond))
		for i, vs := range t.cases {
			for _, v := range vs {
				fmt.Fprintf(&b, " case %dLL:", v)
			}
			fmt.Fprintf(&b, " goto b%d;", t.to[i].id)
		}
		fmt.Fprintf(&b, " default: goto b%d; }", t.to[len(t.to)-1].id)
		return b.String()
	case tRet:
		if t.ret.isZero() {
			return "return;"
		}
		return "return " + f.cLexpr(t.ret) + ";"
	case tFall:
		return "return fall_;"
	}
	return "/* ? */"
}

func (f *lfn) cLexpr(e lexpr) string {
	if e.v != nil {
		return e.v.name
	}
	if e.raw {
		return f.cExpr1(e.n)
	}
	return f.cExpr(e.n)
}

// cInit is an initializer, its expressions through the substitutions.
func (f *lfn) cInit(in *cc.Initializer) string {
	if in.Case == cc.InitializerExpr {
		return f.cExpr(in.AssignmentExpression)
	}
	var parts []string
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		d := ""
		if l.Designation != nil {
			d = cc.NodeSource(l.Designation) + " "
		}
		parts = append(parts, d+f.cInit(l.Initializer))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// cExpr is a C expression, read through the substitutions, fully
// parenthesized.
func (f *lfn) cExpr(e cc.ExpressionNode) string {
	if r, ok := f.sub[e]; ok {
		return "(" + f.cLexpr(r) + ")"
	}
	return f.cExpr1(e)
}

// cExpr1 is cExpr of the node itself, its operands through the
// substitutions.
func (f *lfn) cExpr1(e cc.ExpressionNode) string {
	bin := func(l cc.ExpressionNode, op string, r cc.ExpressionNode) string {
		return "(" + f.cExpr(l) + " " + op + " " + f.cExpr(r) + ")"
	}
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
				if v, ok := f.byDecl[d]; ok {
					return v.name
				}
			}
			return x.Token.SrcStr()
		case cc.PrimaryExpressionExpr:
			return "(" + f.cExpr(x.ExpressionList) + ")"
		}
		return cc.NodeSource(x)
	case *cc.ConstantExpression:
		return f.cExpr(x.ConditionalExpression)
	case *cc.ExpressionList:
		var parts []string
		for ; x != nil; x = x.ExpressionList {
			parts = append(parts, f.cExpr(x.AssignmentExpression))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			return f.cExpr(x.PostfixExpression) + "[" + f.cExpr(x.ExpressionList) + "]"
		case cc.PostfixExpressionCall:
			var args []string
			for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
				args = append(args, f.cExpr(l.AssignmentExpression))
			}
			return f.cExpr(x.PostfixExpression) + "(" + strings.Join(args, ", ") + ")"
		case cc.PostfixExpressionSelect:
			return f.cExpr(x.PostfixExpression) + "." + x.Token2.SrcStr()
		case cc.PostfixExpressionPSelect:
			return f.cExpr(x.PostfixExpression) + "->" + x.Token2.SrcStr()
		case cc.PostfixExpressionInc:
			return "(" + f.cExpr(x.PostfixExpression) + "++)"
		case cc.PostfixExpressionDec:
			return "(" + f.cExpr(x.PostfixExpression) + "--)"
		}
		f.no(x, "a postfix expression %v in C", x.Case)
	case *cc.UnaryExpression:
		op := map[cc.UnaryExpressionCase]string{cc.UnaryExpressionAddrof: "&", cc.UnaryExpressionDeref: "*",
			cc.UnaryExpressionPlus: "+", cc.UnaryExpressionMinus: "-", cc.UnaryExpressionCpl: "~", cc.UnaryExpressionNot: "!"}
		if o, ok := op[x.Case]; ok {
			return "(" + o + f.cExpr(x.CastExpression) + ")"
		}
		switch x.Case {
		case cc.UnaryExpressionSizeofExpr, cc.UnaryExpressionSizeofType, cc.UnaryExpressionAlignofExpr, cc.UnaryExpressionAlignofType:
			if v, ok := intValue(x.Value()); ok {
				return fmt.Sprintf("((unsigned long)%dUL)", v)
			}
		case cc.UnaryExpressionInc:
			return "(++" + f.cExpr(x.UnaryExpression) + ")"
		case cc.UnaryExpressionDec:
			return "(--" + f.cExpr(x.UnaryExpression) + ")"
		}
		f.no(x, "a unary expression %v in C", x.Case)
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return "((" + cc.NodeSource(x.TypeName) + ")" + f.cExpr(x.CastExpression) + ")"
		}
	case *cc.MultiplicativeExpression:
		op := map[cc.MultiplicativeExpressionCase]string{cc.MultiplicativeExpressionMul: "*", cc.MultiplicativeExpressionDiv: "/", cc.MultiplicativeExpressionMod: "%"}[x.Case]
		return bin(x.MultiplicativeExpression, op, x.CastExpression)
	case *cc.AdditiveExpression:
		op := "+"
		if x.Case == cc.AdditiveExpressionSub {
			op = "-"
		}
		return bin(x.AdditiveExpression, op, x.MultiplicativeExpression)
	case *cc.ShiftExpression:
		op := "<<"
		if x.Case == cc.ShiftExpressionRsh {
			op = ">>"
		}
		return bin(x.ShiftExpression, op, x.AdditiveExpression)
	case *cc.RelationalExpression:
		op := map[cc.RelationalExpressionCase]string{cc.RelationalExpressionLt: "<", cc.RelationalExpressionGt: ">", cc.RelationalExpressionLeq: "<=", cc.RelationalExpressionGeq: ">="}[x.Case]
		return bin(x.RelationalExpression, op, x.ShiftExpression)
	case *cc.EqualityExpression:
		op := "=="
		if x.Case == cc.EqualityExpressionNeq {
			op = "!="
		}
		return bin(x.EqualityExpression, op, x.RelationalExpression)
	case *cc.AndExpression:
		return bin(x.AndExpression, "&", x.EqualityExpression)
	case *cc.ExclusiveOrExpression:
		return bin(x.ExclusiveOrExpression, "^", x.AndExpression)
	case *cc.InclusiveOrExpression:
		return bin(x.InclusiveOrExpression, "|", x.ExclusiveOrExpression)
	case *cc.LogicalAndExpression:
		return bin(x.LogicalAndExpression, "&&", x.InclusiveOrExpression)
	case *cc.LogicalOrExpression:
		return bin(x.LogicalOrExpression, "||", x.LogicalAndExpression)
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionCond {
			return "(" + f.cExpr(x.LogicalOrExpression) + " ? " + f.cExpr(x.ExpressionList) + " : " + f.cExpr(x.ConditionalExpression) + ")"
		}
	case *cc.AssignmentExpression:
		if x.Case != cc.AssignmentExpressionCond {
			return "(" + cc.NodeSource(x) + ")"
		}
	}
	f.no(e, "an expression %T in C", e)
	return ""
}

// cDecl declares name as C type t (qualifiers dropped: the lowered body
// assigns what C initialized).
func cDecl(t cc.Type, name string) string { return cDeclQ(t, name, false) }

// cDeclQ is cDecl, with the qualifier const kept when pointee says t is
// what a pointer points at.
func cDeclQ(t cc.Type, name string, pointee bool) string {
	switch t.Kind() {
	case cc.Ptr:
		e := t.(*cc.PointerType).Elem()
		q := ""
		if pointee && t.Attributes().IsConst() {
			q = " const "
		}
		if e.Kind() == cc.Array || e.Kind() == cc.Function {
			return cDeclQ(e, "(*"+q+name+")", true)
		}
		return cDeclQ(e, "*"+q+name, true)
	case cc.Array:
		at := t.(*cc.ArrayType)
		n := ""
		if !at.IsIncomplete() && at.Len() >= 0 {
			n = fmt.Sprint(at.Len())
		}
		return cDeclQ(at.Elem(), name+"["+n+"]", pointee)
	case cc.Function:
		ft := t.(*cc.FunctionType)
		var ps []string
		for _, p := range ft.Parameters() {
			if p.Type() == nil || p.Type().Kind() == cc.Void {
				continue
			}
			ps = append(ps, cDecl(p.Type(), ""))
		}
		if ft.IsVariadic() {
			ps = append(ps, "...")
		}
		if len(ps) == 0 {
			ps = []string{"void"}
		}
		return cDecl(ft.Result(), name+"("+strings.Join(ps, ", ")+")")
	}
	q := ""
	if pointee && t.Attributes().IsConst() {
		q = "const "
	}
	return strings.TrimSpace(q + cBase(t) + " " + name)
}

// cBase names a type that is not derived: a struct, union or enum by its
// tag or typedef, a scalar by C's name.
func cBase(t cc.Type) string {
	switch x := t.(type) {
	case *cc.StructType:
		if s := tagStr(x.Tag()); s != "" {
			return "struct " + s
		}
	case *cc.UnionType:
		if s := tagStr(x.Tag()); s != "" {
			return "union " + s
		}
	case *cc.EnumType:
		if s := tagStr(x.Tag()); s != "" {
			return "enum " + s
		}
		if td := t.Typedef(); td != nil {
			return td.Name()
		}
		return cBase(x.UnderlyingType())
	}
	if t.Kind() == cc.Struct || t.Kind() == cc.Union {
		if td := t.Typedef(); td != nil {
			return td.Name()
		}
	}
	switch t.Kind() {
	case cc.Void:
		return "void"
	case cc.Bool:
		return "_Bool"
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
	case cc.Float:
		return "float"
	case cc.Double:
		return "double"
	case cc.LongDouble:
		return "long double"
	}
	return t.String()
}
