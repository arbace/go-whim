package togo

// java_tables.go writes a C table as its rows (doc/JAVA-IDIOMS.md, item 5):
// where the member-at-a-time initialiser said
//
//	foldCase[0].rangeStart = 65;
//	foldCase[0].rangeEnd = 90;
//	...
//
// the table is one statement, a row a line, as the C writes it:
//
//	Rt.rows(foldCase,
//	    new T_convertStruct(0x41, 0x5a, 1, 32),
//	    ...);
//
// and a table of numbers `Rt.rows(utf8len_tab, 1, 1, ...)`.  Rt.rows fills
// the array the field already holds, in the method the statements were in,
// so the array is the one every pointer to it holds, and what the rows name
// -- a function's reference, another object -- exists as it did.  A row
// replaces the element, so a table another initialiser names (an element's
// address taken before its row is written) keeps the statements.  A row is
// of a struct whose members are all numbers and references, each item one
// expression; anything else keeps the statements too.

import (
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// A member's form in a row: how its constructor takes it.
const (
	formPlain  = iota // a number, a boolean or a reference: as it is
	formBoxed         // held in a one-element array: its value
	formStruct        // a struct of a row class: a row of its own, set into it
	formArray         // an array of plain elements: an array, copied into it
)

// rowForm is fl's form in a row, and false when a row cannot hold it: a
// union, a bitfield, an array of arrays or of structs, a struct that is
// not a row class.
func (j *jgen) rowForm(fl *cc.Field, depth int) (int, bool) {
	if fl == nil || fl.IsBitfield() {
		return 0, false
	}
	if _, why := j.jt(fl.Type(), fieldKey(fl)); why != "" {
		return 0, false
	}
	if j.boxedField[fieldKey(fl)] {
		return formBoxed, true
	}
	switch t := fl.Type(); t.Kind() {
	case cc.Union:
		return 0, false
	case cc.Struct:
		_, ok := j.rowClassAt(t, depth+1)
		return formStruct, ok
	case cc.Array:
		switch t.(*cc.ArrayType).Elem().Kind() {
		case cc.Struct, cc.Union, cc.Array:
			return 0, false
		}
		return formArray, true
	}
	return formPlain, true
}

// rowClass says a struct is one a table row can be made of, every member
// of a form a row holds, and names its class.
func (j *jgen) rowClass(t cc.Type) (string, bool) { return j.rowClassAt(t, 0) }

func (j *jgen) rowClassAt(t cc.Type, depth int) (string, bool) {
	if t.Kind() != cc.Struct || depth > 4 {
		return "", false
	}
	name, why := j.structName(t)
	if why != "" {
		return "", false
	}
	for _, fl := range members(t) {
		if _, ok := j.rowForm(fl, depth); !ok {
			return "", false
		}
	}
	return name, true
}

// ctorParam is the Java type a row's constructor takes a member of Java
// type jt as: a byte or short as an int, which the constructor narrows,
// since Java narrows a constant in an assignment and not in a call.
func ctorParam(jt string) string {
	if jt == "byte" || jt == "short" {
		return "int"
	}
	return jt
}

// rowsInit writes target's initial value as Rt.rows when it is a table of
// rows (see above), and says whether it did.
func (f *jfn) rowsInit(target, name string, t cc.Type, key string, in *cc.Initializer) (ok bool) {
	at, isArr := t.(*cc.ArrayType)
	if !isArr || in.Case != cc.InitializerInitList || f.j.initNamed[name] {
		return false
	}
	var items []*cc.Initializer
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		// a designator that names the row's own place, [CMD_append] as
		// the C's command table has, says nothing a row does not
		if dn := l.Designation; dn != nil {
			if dn.DesignatorList.DesignatorList != nil || dn.DesignatorList.Designator.Case != cc.DesignatorIndex {
				return false
			}
			v, ok := dn.DesignatorList.Designator.ConstantExpression.Value().(cc.Int64Value)
			if !ok || int64(v) != int64(len(items)) {
				return false
			}
		}
		items = append(items, l.Initializer)
	}
	if len(items) < 2 || int64(len(items)) > at.Len() {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			if _, is := r.(unsupported); !is {
				panic(r)
			}
			ok = false
		}
	}()
	var rows []string
	elem := at.Elem()
	if k, scalar := scalarKind(elem); scalar {
		// a table of numbers: each a constant, as an int or a long
		pk := jInt
		if k.size == 8 {
			pk = jk{size: 8, signed: true}
		}
		if k.boolean {
			pk = k
		}
		for _, it := range items {
			if it.Case != cc.InitializerExpr {
				return false
			}
			v := f.exprTo(it.AssignmentExpression, pk.java())
			if !v.konst && !k.boolean {
				return false
			}
			rows = append(rows, f.rowArg(v, k, pk))
		}
		f.rowsText(target, rows, false)
		return true
	}
	if _, rowOK := f.j.rowClass(elem); !rowOK {
		return false
	}
	used := map[string]bool{}
	for _, it := range items {
		r, ok := f.rowValue(elem, it, used)
		if !ok {
			return false
		}
		rows = append(rows, r)
	}
	for c := range used {
		f.j.ctorClass[c] = true
	}
	f.rowsText(target, rows, true)
	return true
}

// rowValue is a struct t's initial value in as a row, `new T(...)`, and
// false when it is not one; used gathers the classes whose constructors
// it calls.
func (f *jfn) rowValue(t cc.Type, in *cc.Initializer, used map[string]bool) (string, bool) {
	cls, _ := f.j.rowClass(t)
	if zeroInit(in) {
		return "new " + cls + "()", true
	}
	if in.Case != cc.InitializerInitList {
		return "", false
	}
	used[cls] = true
	fs := members(t)
	var args []string
	i := 0
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		if l.Designation != nil || i >= len(fs) {
			return "", false
		}
		fl := fs[i]
		i++
		form, _ := f.j.rowForm(fl, 0)
		switch form {
		case formStruct:
			v, ok := f.rowValue(fl.Type(), l.Initializer, used)
			if !ok {
				return "", false
			}
			args = append(args, v)
			continue
		case formArray:
			v, ok := f.rowArray(fl, l.Initializer)
			if !ok {
				return "", false
			}
			args = append(args, v)
			continue
		}
		if l.Initializer.Case != cc.InitializerExpr {
			return "", false
		}
		v, ok := f.rowScalar(l.Initializer.AssignmentExpression, fl.Type(), f.jt(fl.Type(), fieldKey(fl)), true)
		if !ok {
			return "", false
		}
		args = append(args, v)
	}
	for ; i < len(fs); i++ {
		args = append(args, f.rowZero(fs[i]))
	}
	return "new " + cls + "(" + strings.Join(args, ", ") + ")", true
}

// rowScalar is a number's or reference's value e, of C type t and Java type
// jt; arg says it is a constructor's argument, where a byte or short is
// taken as an int, and not an array initialiser's element, where Java
// narrows a constant itself.
func (f *jfn) rowScalar(e cc.ExpressionNode, t cc.Type, jt string, arg bool) (s string, ok bool) {
	pre := f.capture(func() {
		v := f.exprTo(e, jt)
		k, isNum := scalarKind(t)
		switch {
		case isNum && arg && (k.size == 1 || k.size == 2) && !k.boolean:
			if !v.konst {
				f.no(nil, "a byte member of no constant value")
			}
			s = f.rowArg(v, k, jInt)
		case isNum:
			s = narrowConst(f.conv(v, jt, t), v, k)
		default:
			s = f.conv(v, jt, t)
		}
	})
	return s, pre == "" // a value that needed statements of its own is not a row's
}

// rowArray is an array member's initial value, `new E[] {a, b}`: as long
// as the initialiser, which the constructor copies from the start.
func (f *jfn) rowArray(fl *cc.Field, in *cc.Initializer) (string, bool) {
	at := fl.Type().(*cc.ArrayType)
	ajt := f.jt(fl.Type(), fieldKey(fl))
	ejt := elemJ(ajt)
	if in.Case != cc.InitializerInitList {
		return "", false
	}
	var els []string
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		if l.Designation != nil || l.Initializer.Case != cc.InitializerExpr {
			return "", false
		}
		v, ok := f.rowScalar(l.Initializer.AssignmentExpression, at.Elem(), ejt, false)
		if !ok {
			return "", false
		}
		els = append(els, v)
	}
	return "new " + raw(ejt) + "[] {" + strings.Join(els, ", ") + "}", true
}

// rowZero is a member's argument for a row that says nothing of it.
func (f *jfn) rowZero(fl *cc.Field) string {
	jt := f.jt(fl.Type(), fieldKey(fl))
	form, _ := f.j.rowForm(fl, 0)
	switch form {
	case formStruct:
		cls, _ := f.j.rowClass(fl.Type())
		return "new " + cls + "()"
	case formArray:
		return "new " + raw(elemJ(jt)) + "[0]"
	}
	return zeroOf(ctorParam(jt))
}

// rowArg is a constant v of kind k as the argument of kind pk that the
// row's parameter narrows back to k's bits: its name when that gives the
// same bits, else its value.
func (f *jfn) rowArg(v jval, k, pk jk) string {
	if k.boolean {
		return f.convK(v, k)
	}
	if k.size >= pk.size {
		return f.convK(v, k) // an int or a long: as the member takes it
	}
	if v.named && v.jn.size != 0 && v.jn.size <= pk.size && jwant(v.jn.v, k) == jwant(v.cv, k) {
		return v.s
	}
	return jlit(jwant(v.cv, k), pk)
}

// rowsText writes Rt.rows(target, ...): a row a line, or numbers as many
// to a line as fit.
func (f *jfn) rowsText(target string, rows []string, oneALine bool) {
	ind := strings.Repeat("    ", f.indent+1)
	f.line("Rt.rows(%s,", target)
	if oneALine {
		for i, r := range rows {
			sep := ","
			if i == len(rows)-1 {
				sep = ");"
			}
			f.out.WriteString(ind + r + sep + "\n")
		}
		return
	}
	line := ind
	for i, r := range rows {
		sep := ", "
		if i == len(rows)-1 {
			sep = ");"
		}
		if len(line)+len(r)+len(sep) > jwidth && line != ind {
			f.out.WriteString(strings.TrimRight(line, " ") + "\n")
			line = ind
		}
		line += r + sep
	}
	f.out.WriteString(line + "\n")
}

// ctorText is the constructors a row class gets: none, and its members in
// order -- a boxed member's value, a struct member's row set into it, an
// array member's elements copied into it from the start.
func (j *jgen) ctorText(name string, t cc.Type) string {
	var ps, body []string
	for i, fl := range members(t) {
		fname := j.memberName(fl, i)
		jt, _ := j.jt(fl.Type(), fieldKey(fl))
		form, _ := j.rowForm(fl, 0)
		switch form {
		case formStruct:
			ps = append(ps, jt+" "+fname)
			body = append(body, "            this."+fname+".set("+fname+");\n")
		case formArray:
			ps = append(ps, jt+" "+fname)
			body = append(body, "            System.arraycopy("+fname+", 0, this."+fname+", 0, "+fname+".length);\n")
		default:
			dst := "this." + fname
			if form == formBoxed {
				dst += "[0]"
			}
			p := ctorParam(jt)
			ps = append(ps, p+" "+fname)
			if p != jt {
				body = append(body, "            "+dst+" = ("+jt+") "+fname+";\n")
			} else {
				body = append(body, "            "+dst+" = "+fname+";\n")
			}
		}
	}
	return "\n        " + name + "() {\n        }\n\n        " + name + "(" + strings.Join(ps, ", ") + ") {\n" + strings.Join(body, "") + "        }\n"
}

// identDecl is the declarator an identifier names, or nil.
func identDecl(n cc.Node) *cc.Declarator {
	p, ok := n.(*cc.PrimaryExpression)
	if !ok || p.Case != cc.PrimaryExpressionIdent {
		return nil
	}
	d, _ := p.ResolvedTo().(*cc.Declarator)
	return d
}

// walkNodes calls fn with n and every node under it.
func walkNodes(n cc.Node, fn func(cc.Node)) {
	if n == nil {
		return
	}
	fn(n)
	walkChildrenFn(n, func(c cc.Node) { walkNodes(c, fn) })
}
