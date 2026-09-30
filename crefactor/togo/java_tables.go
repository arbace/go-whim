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

// rowClass says a struct is one a table row can be made of: every member a
// number, a boolean or a reference held plainly -- no array, struct, union,
// bitfield, or member held in a one-element array.
func (j *jgen) rowClass(t cc.Type) (string, bool) {
	if t.Kind() != cc.Struct {
		return "", false
	}
	name, why := j.structName(t)
	if why != "" {
		return "", false
	}
	for _, fl := range members(t) {
		if fl == nil || fl.IsBitfield() || j.boxedField[fieldKey(fl)] {
			return "", false
		}
		switch fl.Type().Kind() {
		case cc.Struct, cc.Union, cc.Array:
			return "", false
		}
		if _, why := j.jt(fl.Type(), fieldKey(fl)); why != "" {
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
	cls, rowOK := f.j.rowClass(elem)
	if !rowOK {
		return false
	}
	fs := members(elem)
	for _, it := range items {
		if zeroInit(it) {
			rows = append(rows, "new "+cls+"()")
			continue
		}
		if it.Case != cc.InitializerInitList {
			return false
		}
		var args []string
		i := 0
		for l := it.InitializerList; l != nil; l = l.InitializerList {
			if l.Designation != nil || i >= len(fs) || l.Initializer.Case != cc.InitializerExpr {
				return false
			}
			fl := fs[i]
			i++
			mjt := f.jt(fl.Type(), fieldKey(fl))
			var s string
			pre := f.capture(func() {
				v := f.exprTo(l.Initializer.AssignmentExpression, mjt)
				if k, isNum := scalarKind(fl.Type()); isNum && (k.size == 1 || k.size == 2) && !k.boolean {
					if !v.konst {
						f.no(nil, "a byte member of no constant value")
					}
					s = f.rowArg(v, k, jInt)
				} else {
					s = f.conv(v, mjt, fl.Type())
				}
			})
			if pre != "" {
				return false // the value needed statements of its own
			}
			args = append(args, s)
		}
		for ; i < len(fs); i++ {
			args = append(args, zeroOf(ctorParam(f.jt(fs[i].Type(), fieldKey(fs[i])))))
		}
		rows = append(rows, "new "+cls+"("+strings.Join(args, ", ")+")")
	}
	f.j.ctorClass[cls] = true
	f.rowsText(target, rows, true)
	return true
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
// order.
func (j *jgen) ctorText(name string, t cc.Type) string {
	var ps, body []string
	for i, fl := range members(t) {
		fname := j.memberName(fl, i)
		jt, _ := j.jt(fl.Type(), fieldKey(fl))
		ps = append(ps, ctorParam(jt)+" "+fname)
		if ctorParam(jt) != jt {
			body = append(body, "            this."+fname+" = ("+jt+") "+fname+";\n")
		} else {
			body = append(body, "            this."+fname+" = "+fname+";\n")
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
