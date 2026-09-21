// Command skel reads editor.c with modernc.org/cc/v4 and writes the shared
// skeleton of its Go transpilation: which C pointers must become cursors
// (Ptr[T], a buffer and an offset) and which may stay plain Go pointers, and
// from that the Go types, globals and function signatures every transpiling
// agent writes against.
//
// A pointer is a CURSOR when anything it is ever connected to is indexed,
// incremented, added to, subtracted, ordered, or handed an array or a pointer
// into one.  "Connected" is a union-find over objects -- variables,
// parameters, struct fields, function results -- joined by assignment,
// initialisation, argument passing, return, the arms of ?:, casts and ==.  A
// class with no such use is a plain *T.  char and unsigned char pointers are
// always cursors: they are C strings.
package main

import (
	"fmt"
	"reflect"
	"sort"

	"modernc.org/cc/v4"
)

type uf struct {
	parent map[string]string
	flag   map[string]string // root -> first reason it is a cursor
	pun    map[string]string // root -> first reason it holds a non-char address
}

func newUF() *uf { return &uf{parent: map[string]string{}, flag: map[string]string{}, pun: map[string]string{}} }

func (u *uf) find(k string) string {
	if _, ok := u.parent[k]; !ok {
		u.parent[k] = k
	}
	for u.parent[k] != k {
		u.parent[k] = u.parent[u.parent[k]]
		k = u.parent[k]
	}
	return k
}

func (u *uf) union(a, b string) {
	if a == "" || b == "" {
		return
	}
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	u.parent[ra] = rb
	for _, m := range []map[string]string{u.flag} {
		if r, ok := m[ra]; ok {
			if _, ok2 := m[rb]; !ok2 {
				m[rb] = r
			}
			delete(m, ra)
		}
	}
}

// Punning is recorded on the object itself and never spread through a class:
// every string in the file is one class, and a pun is a property of the few
// objects that hold other things' addresses.
func (u *uf) markPun(k, why string) {
	if k == "" {
		return
	}
	if _, ok := u.pun[k]; !ok {
		u.pun[k] = why
	}
}

func (u *uf) punned(k string) bool {
	_, ok := u.pun[k]
	return ok
}

func (u *uf) mark(k, why string) {
	if k == "" {
		return
	}
	r := u.find(k)
	if _, ok := u.flag[r]; !ok {
		u.flag[r] = why
	}
}

func (u *uf) cursor(k string) (bool, string) {
	r := u.find(k)
	why, ok := u.flag[r]
	return ok, why
}

// analysis state
type an struct {
	edges   [][2]string // char-pointer flows, dst <- src, for spreading puns
	u       *uf
	curFn   *cc.Declarator
	fnDecls map[string]*cc.Declarator // name -> definition or declaration
	decls   map[*cc.Declarator]string // key per declarator
	statics []*staticLocal            // block-scope statics, hoisted to globals
	addr    map[string]bool           // functions used as values
}

type staticLocal struct {
	fn   string
	d    *cc.Declarator
	init *cc.Initializer
}

func isPtr(t cc.Type) bool { return t != nil && t.Kind() == cc.Ptr }

func elemOf(t cc.Type) cc.Type {
	if p, ok := t.(*cc.PointerType); ok {
		return p.Elem()
	}
	if a, ok := t.(*cc.ArrayType); ok {
		return a.Elem()
	}
	return nil
}

func typeOf(n cc.Node) cc.Type {
	if e, ok := n.(cc.ExpressionNode); ok && e != nil && !reflect.ValueOf(e).IsNil() {
		return e.Type()
	}
	return nil
}

// declKey names a declarator's object.
func (a *an) declKey(d *cc.Declarator) string {
	if k, ok := a.decls[d]; ok {
		return k
	}
	var k string
	switch {
	case d.IsParam():
		// parameters are keyed by their function and position, filled by
		// the function walk; one seen from elsewhere is its own object
		k = fmt.Sprintf("param:?:%s@%v", d.Name(), d.Position())
	case d.StorageDuration() == cc.Static && d.Linkage() == cc.None && a.curFn != nil:
		k = fmt.Sprintf("static:%s.%s", a.curFn.Name(), d.Name())
	case d.Linkage() != cc.None || a.curFn == nil:
		k = "global:" + d.Name()
	default:
		k = fmt.Sprintf("local:%s.%s@%d", a.curFn.Name(), d.Name(), d.Position().Line)
	}
	a.decls[d] = k
	return k
}

func fieldKey(f *cc.Field) string {
	if f == nil {
		return ""
	}
	pt := f.ParentType()
	name := "?"
	if pt != nil {
		name = typeName(pt)
	}
	return "field:" + name + "." + f.Name()
}

// typeName is a stable name for a struct or union type: its typedef, else its
// tag, else its string.
func typeName(t cc.Type) string {
	// the tag first: a struct reached through its typedef and through
	// `struct tag` is two cc.Type values and must be one object
	switch x := t.(type) {
	case *cc.StructType:
		if s := tagStr(x.Tag()); s != "" {
			return "struct " + s
		}
	case *cc.UnionType:
		if s := tagStr(x.Tag()); s != "" {
			return "union " + s
		}
	}
	if td := t.Typedef(); td != nil {
		return td.Name()
	}
	return t.String()
}

// obj is the object an expression denotes, for pointer flow: an identifier's
// declarator, a member's field, a call's function result -- seen through
// parentheses, casts, and the comma operator's last element.
func (a *an) obj(n cc.ExpressionNode) string {
	if n == nil || reflect.ValueOf(n).IsNil() {
		return ""
	}
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionIdent:
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
				if d.Type() != nil && d.Type().Kind() == cc.Function {
					return ""
				}
				return a.declKey(d)
			}
		case cc.PrimaryExpressionExpr:
			return a.obj(x.ExpressionList)
		}
	case *cc.ExpressionList:
		for x.ExpressionList != nil {
			x = x.ExpressionList
		}
		return a.obj(x.AssignmentExpression)
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return a.obj(x.CastExpression)
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			return fieldKey(x.Field())
		case cc.PostfixExpressionCall:
			if d := calleeDecl(x.PostfixExpression); d != nil {
				return "ret:" + d.Name()
			}
		case cc.PostfixExpressionIndex:
			// an element of an array of pointers: the elements share a class
			return "elem:" + a.obj(x.PostfixExpression)
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			if o := a.obj(x.CastExpression); o != "" {
				return "elem:" + o
			}
		}
	}
	return ""
}

// objTyped is obj without seeing through a cast that changes what a pointer
// points at.
func (a *an) objTyped(n cc.ExpressionNode) string {
	for {
		c, ok := n.(*cc.CastExpression)
		if !ok || c.Case != cc.CastExpressionCast {
			break
		}
		if !sameTarget(c.Type(), c.CastExpression.Type()) {
			return ""
		}
		n = c.CastExpression
	}
	return a.obj(n)
}

func calleeDecl(n cc.ExpressionNode) *cc.Declarator {
	for {
		switch x := n.(type) {
		case *cc.PrimaryExpression:
			if x.Case == cc.PrimaryExpressionIdent {
				if d, ok := x.ResolvedTo().(*cc.Declarator); ok {
					return d
				}
			}
			if x.Case == cc.PrimaryExpressionExpr {
				n = x.ExpressionList
				continue
			}
			return nil
		case *cc.ExpressionList:
			n = x.AssignmentExpression
			continue
		}
		return nil
	}
}

// intoArray: the expression's value points into an array -- an array that
// decays, pointer arithmetic, the address of an element.
func (a *an) intoArray(n cc.ExpressionNode) (bool, string) {
	if n == nil || reflect.ValueOf(n).IsNil() {
		return false, ""
	}
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent || x.Case == cc.PrimaryExpressionString {
			if t := x.Type(); t != nil && t.Undecay().Kind() == cc.Array {
				return true, "array decay"
			}
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Array {
				return true, "array decay"
			}
		}
		if x.Case == cc.PrimaryExpressionExpr {
			return a.intoArray(x.ExpressionList)
		}
	case *cc.ExpressionList:
		for x.ExpressionList != nil {
			x = x.ExpressionList
		}
		return a.intoArray(x.AssignmentExpression)
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return a.intoArray(x.CastExpression)
		}
	case *cc.AdditiveExpression:
		if isPtr(x.Type()) {
			return true, "pointer arithmetic"
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionAddrof {
			if p, ok := x.CastExpression.(*cc.PostfixExpression); ok && p.Case == cc.PostfixExpressionIndex {
				return true, "address of an element"
			}
		}
	case *cc.PostfixExpression:
		if x.Case == cc.PostfixExpressionSelect || x.Case == cc.PostfixExpressionPSelect {
			if f := x.Field(); f != nil && f.Type().Kind() == cc.Array {
				return true, "array member decay"
			}
		}
	}
	return false, ""
}

// sameTarget: two pointer types point at the same kind of thing, so a
// pointer may flow from one to the other and they must share a Go type.  void
// pointers join nothing: every pointer passes through vim_free.
func sameTarget(a, b cc.Type) bool {
	if a == nil || b == nil {
		return false
	}
	a, b = a.Decay(), b.Decay()
	if a.Kind() != cc.Ptr || b.Kind() != cc.Ptr {
		return false
	}
	ea, eb := a.(*cc.PointerType).Elem(), b.(*cc.PointerType).Elem()
	if ea.Kind() == cc.Void || eb.Kind() == cc.Void {
		return false
	}
	return sameType(ea, eb)
}

func sameType(a, b cc.Type) bool {
	isChar := func(k cc.Kind) bool { return k == cc.Char || k == cc.UChar || k == cc.SChar }
	if isChar(a.Kind()) && isChar(b.Kind()) {
		return true
	}
	if a.Kind() != b.Kind() {
		return false
	}
	switch a.Kind() {
	case cc.Struct, cc.Union:
		return typeName(a) == typeName(b)
	case cc.Ptr:
		return sameType(a.(*cc.PointerType).Elem(), b.(*cc.PointerType).Elem())
	case cc.Function:
		return true
	}
	return true
}

// flow: a value of expression src is stored into object dst, of type dt.
func (a *an) flow(dst string, src cc.ExpressionNode) { a.flowT(dst, nil, src) }

func isCharPtr(t cc.Type) bool {
	if t == nil || t.Decay().Kind() != cc.Ptr {
		return false
	}
	k := t.Decay().(*cc.PointerType).Elem().Kind()
	return k == cc.Char || k == cc.UChar || k == cc.SChar
}

// punSource: through casts, src is a pointer to something that is not a
// character -- the address of an int, a long, a pointer.
func punSource(src cc.ExpressionNode) bool {
	for {
		c, ok := src.(*cc.CastExpression)
		if !ok || c.Case != cc.CastExpressionCast {
			break
		}
		src = c.CastExpression
	}
	if p, ok := src.(*cc.PrimaryExpression); ok && p.Case == cc.PrimaryExpressionExpr {
		if el, ok := p.ExpressionList.(*cc.ExpressionList); ok && el.ExpressionList == nil {
			return punSource(el.AssignmentExpression)
		}
	}
	t := src.Type()
	if t == nil {
		return false
	}
	switch t.Decay().Kind() {
	case cc.Ptr:
		k := t.Decay().(*cc.PointerType).Elem().Kind()
		return k != cc.Char && k != cc.UChar && k != cc.SChar && k != cc.Void
	case cc.Int, cc.Long, cc.LongLong, cc.UInt, cc.ULong, cc.ULongLong:
		// an integer stored as a pointer, (char_u *)8L -- but not the constant 0
		if v, ok := src.Value().(cc.Int64Value); ok && v == 0 {
			return false
		}
		return true
	}
	return false
}

func (a *an) flowT(dst string, dt cc.Type, src cc.ExpressionNode) {
	if dst == "" || src == nil || reflect.ValueOf(src).IsNil() {
		return
	}
	if dt != nil && isCharPtr(dt) && punSource(src) {
		a.u.markPun(dst, fmt.Sprintf("holds a non-char address at %d", src.Position().Line))
	}
	if dt != nil && isCharPtr(dt) && isCharPtr(src.Type()) {
		// char pointer to char pointer, through any casts: one class
		if o := a.obj(src); o != "" {
			a.u.union(dst, o)
			a.edges = append(a.edges, [2]string{dst, o})
		}
	}
	if dt != nil && !sameTarget(dt, src.Type()) {
		// a void pointer, or a cast that changes what is pointed at: the
		// two sides need not share a representation
		if st := src.Type(); st == nil || st.Decay().Kind() != cc.Ptr || dt.Decay().Kind() != cc.Ptr ||
			dt.Decay().(*cc.PointerType).Elem().Kind() == cc.Void || st.Decay().(*cc.PointerType).Elem().Kind() == cc.Void {
			return
		}
	}
	if ok, why := a.intoArray(src); ok {
		a.u.mark(dst, fmt.Sprintf("%s at %d", why, src.Position().Line))
	}
	// through ?: both arms
	if c, ok := src.(*cc.ConditionalExpression); ok && c.Case == cc.ConditionalExpressionCond {
		a.flowT(dst, dt, c.ExpressionList)
		a.flowT(dst, dt, c.ConditionalExpression)
		return
	}
	if o := a.objTyped(src); o != "" {
		a.u.union(dst, o)
	}
	// a function designator stored as a value
	if d := calleeDecl(src); d != nil && d.Type() != nil && d.Type().Kind() == cc.Function {
		a.addr[d.Name()] = true
	}
}

func (a *an) paramKey(fn string, i int) string { return fmt.Sprintf("param:%s:%d", fn, i) }

// walk visits every node; the cases below record pointer uses and flows.
func (a *an) walk(n cc.Node) {
	if n == nil {
		return
	}
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return
	}
	switch x := n.(type) {
	case *cc.FunctionDefinition:
		d := x.Declarator
		prev := a.curFn
		a.curFn = d
		if ft, ok := d.Type().(*cc.FunctionType); ok {
			for i, p := range ft.Parameters() {
				if pd := p.Declarator; pd != nil {
					a.decls[pd] = a.paramKey(d.Name(), i)
				}
			}
		}
		a.walkChildren(n)
		a.curFn = prev
		return
	case *cc.InitDeclarator:
		if x.Case == cc.InitDeclaratorInit && x.Initializer != nil {
			d := x.Declarator
			k := a.declKey(d)
			if d.StorageDuration() == cc.Static && d.Linkage() == cc.None && a.curFn != nil && !d.IsSynthetic() && d.Name() != "__func__" {
				a.statics = append(a.statics, &staticLocal{a.curFn.Name(), d, x.Initializer})
			}
			if x.Initializer.Case == cc.InitializerExpr {
				a.flowT(k, d.Type(), x.Initializer.AssignmentExpression)
			}
		} else if x.Declarator != nil {
			d := x.Declarator
			a.declKey(d)
			if d.StorageDuration() == cc.Static && d.Linkage() == cc.None && a.curFn != nil && !d.IsSynthetic() && d.Name() != "__func__" {
				a.statics = append(a.statics, &staticLocal{a.curFn.Name(), d, nil})
			}
		}
	case *cc.Initializer:
		// an element of a braced initializer that sets a field
		if x.Case == cc.InitializerExpr {
			if f := x.Field(); f != nil && isPtr(f.Type()) {
				a.flowT(fieldKey(f), f.Type(), x.AssignmentExpression)
			}
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionIndex:
			if t := typeOf(x.PostfixExpression); t != nil && t.Kind() == cc.Ptr && t.Undecay().Kind() != cc.Array {
				a.u.mark(a.objTyped(x.PostfixExpression), fmt.Sprintf("indexed at %d", x.Position().Line))
			}
		case cc.PostfixExpressionInc, cc.PostfixExpressionDec:
			if isPtr(typeOf(x.PostfixExpression)) {
				a.u.mark(a.objTyped(x.PostfixExpression), fmt.Sprintf("incremented at %d", x.Position().Line))
			}
		case cc.PostfixExpressionCall:
			if d := calleeDecl(x.PostfixExpression); d != nil {
				var ps []*cc.Parameter
				if ft, ok := d.Type().(*cc.FunctionType); ok {
					ps = ft.Parameters()
				}
				i := 0
				for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
					var pt cc.Type
					if i < len(ps) {
						pt = ps[i].Type()
					}
					if pt != nil {
						a.flowT(a.paramKey(d.Name(), i), pt, l.AssignmentExpression)
					}
					i++
				}
			}
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionInc || x.Case == cc.UnaryExpressionDec {
			if isPtr(typeOf(x.UnaryExpression)) {
				a.u.mark(a.objTyped(x.UnaryExpression), fmt.Sprintf("incremented at %d", x.Position().Line))
			}
		}
	case *cc.AdditiveExpression:
		if x.Case == cc.AdditiveExpressionAdd || x.Case == cc.AdditiveExpressionSub {
			if isPtr(typeOf(x.AdditiveExpression)) {
				a.u.mark(a.objTyped(x.AdditiveExpression), fmt.Sprintf("arithmetic at %d", x.Position().Line))
			}
			if isPtr(typeOf(x.MultiplicativeExpression)) {
				a.u.mark(a.objTyped(x.MultiplicativeExpression), fmt.Sprintf("arithmetic at %d", x.Position().Line))
			}
		}
	case *cc.RelationalExpression:
		if x.Case != cc.RelationalExpressionShift {
			if isPtr(typeOf(x.RelationalExpression)) && isPtr(typeOf(x.ShiftExpression)) {
				a.u.mark(a.objTyped(x.RelationalExpression), fmt.Sprintf("ordered at %d", x.Position().Line))
				a.u.mark(a.objTyped(x.ShiftExpression), fmt.Sprintf("ordered at %d", x.Position().Line))
			}
		}
	case *cc.EqualityExpression:
		if x.Case != cc.EqualityExpressionRel {
			if sameTarget(typeOf(x.EqualityExpression), typeOf(x.RelationalExpression)) {
				a.u.union(a.objTyped(x.EqualityExpression), a.objTyped(x.RelationalExpression))
			}
		}
	case *cc.AssignmentExpression:
		switch x.Case {
		case cc.AssignmentExpressionAssign:
			if isPtr(typeOf(x.UnaryExpression)) {
				a.flowT(a.obj(x.UnaryExpression), typeOf(x.UnaryExpression), x.AssignmentExpression)
			}
		case cc.AssignmentExpressionAdd, cc.AssignmentExpressionSub:
			if isPtr(typeOf(x.UnaryExpression)) {
				a.u.mark(a.objTyped(x.UnaryExpression), fmt.Sprintf("compound arithmetic at %d", x.Position().Line))
			}
		}
	case *cc.JumpStatement:
		if x.Case == cc.JumpStatementReturn && a.curFn != nil && x.ExpressionList != nil {
			if ft, ok := a.curFn.Type().(*cc.FunctionType); ok {
				a.flowT("ret:"+a.curFn.Name(), ft.Result(), x.ExpressionList)
			}
		}
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			if d, ok := x.ResolvedTo().(*cc.Declarator); ok && d.Type() != nil && d.Type().Kind() == cc.Function {
				a.fnDecls[d.Name()] = d
			}
		}
	}
	a.walkChildren(n)
}

var nodeType = reflect.TypeOf((*cc.Node)(nil)).Elem()

func (a *an) walkChildren(n cc.Node) {
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Ptr, reflect.Interface:
			if fv.IsNil() {
				continue
			}
			if c, ok := fv.Interface().(cc.Node); ok {
				a.walk(c)
			}
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

// walkChildrenFn calls f on every exported child node of n.
func walkChildrenFn(n cc.Node, f func(cc.Node)) {
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if !t.Field(i).IsExported() {
			continue
		}
		fv := v.Field(i)
		if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface) && !fv.IsNil() {
			if c, ok := fv.Interface().(cc.Node); ok {
				f(c)
			}
		}
	}
}

func tagStr(t cc.Token) string { return t.SrcStr() }

// spreadPuns carries a pun from a source to what it is stored into, one way:
// varp = get_varp(p) makes varp a pun; strcpy(buf, varp) does not make buf one.
func (a *an) spreadPuns() {
	for changed := true; changed; {
		changed = false
		for _, e := range a.edges {
			if a.u.punned(e[1]) && !a.u.punned(e[0]) {
				a.u.markPun(e[0], "from "+e[1])
				changed = true
			}
		}
	}
}
