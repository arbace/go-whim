package ccx

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// A growarray's storage (Profile.GrowArray.Data; vim's ga_data) is a void *:
// C casts it to its element type where it is used, and a translation into Go
// has to give the storage one type.
// editor/crt.go's GaData[T] makes the storage at the first typed access and
// panics if a growarray is later used as another type.  GrowArrays proves it
// never is: every growarray object has at most one element type.
//
// An object is a global or a local growarray, a struct's growarray field (all
// such fields of one struct type are one object: nothing tells them apart), or
// an array of them, or one allocated.  A pointer to a growarray -- a
// parameter, a local, a field, a function's result -- points at every object
// it is given, by a call's argument, an assignment, an initializer or a
// return.  The element type comes from every use of ga_data: the
// pointer it is cast to, or converted to by an argument, an assignment, an
// initializer or a return; and from every itemsize set as sizeof(T).  A test
// against nullptr, a store into ga_data and a function of bytes (ga_grow's
// copy) are neutral.  Char, unsigned char and signed char are one type,
// bytes.

type gaUse struct {
	key, typ, fn, where string
}

type gaWalk struct {
	uses   []gaUse
	points map[string]map[string]bool // pointer key -> keys it can point at
	left   []Finding
	fnType map[string]*cc.FunctionType
	fn     string

	ga                    GrowArray
	allocators, byteFuncs map[string]bool
}

func (w *gaWalk) isGarray(t cc.Type) bool {
	if t == nil || t.Kind() != cc.Struct {
		return false
	}
	s := t.String()
	return w.ga.Type != "" && s == w.ga.Type || w.ga.Tag != "" && strings.Contains(s, "struct "+w.ga.Tag)
}

// elemName names an element type; the character types are one, bytes.
func elemName(t cc.Type) string {
	if t == nil {
		return ""
	}
	if isByte(t) {
		return "bytes"
	}
	if t.Kind() == cc.Void {
		return ""
	}
	if p, ok := t.(*cc.PointerType); ok {
		if e := elemName(p.Elem()); e != "" {
			return "pointer to " + e
		}
	}
	return t.String()
}

// varKey names a variable: a local by its function, a global by its name.
func (w *gaWalk) varKey(p *cc.PrimaryExpression) string {
	if d, ok := p.ResolvedTo().(*cc.Declarator); ok && (d.IsParam() || d.StorageDuration() == cc.Automatic || (d.IsStatic() && d.Linkage() == cc.None)) {
		return w.fn + "." + p.Token.SrcStr()
	}
	return p.Token.SrcStr()
}

// objKey names the growarray object an lvalue of type garray_T is.
func (w *gaWalk) objKey(e cc.ExpressionNode) string {
	e = unparen(e)
	switch x := e.(type) {
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			return w.varKey(x)
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect:
			return "field " + typeOf(x.PostfixExpression).String() + "." + x.Token2.SrcStr()
		case cc.PostfixExpressionPSelect:
			if p, ok := typeOf(x.PostfixExpression).(*cc.PointerType); ok {
				return "field " + p.Elem().String() + "." + x.Token2.SrcStr()
			}
		case cc.PostfixExpressionIndex:
			return w.objKey(x.PostfixExpression) + "[]"
		}
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionDeref {
			return w.ptrKey(x.CastExpression)
		}
	}
	return ""
}

// ptrKey names what an expression of type garray_T * is: &object is the
// object; a pointer variable or field is itself, "*" and its name.
func (w *gaWalk) ptrKey(e cc.ExpressionNode) string {
	e = unparen(e)
	switch x := e.(type) {
	case *cc.UnaryExpression:
		if x.Case == cc.UnaryExpressionAddrof {
			return w.objKey(x.CastExpression)
		}
	case *cc.PrimaryExpression:
		if x.Case == cc.PrimaryExpressionIdent {
			return "*" + w.varKey(x)
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionCall:
			if name := callee(x); w.allocators[name] {
				return "new growarray in " + w.fn // a fresh object
			} else if name != "" {
				return "*" + name + "()" // what the function returns
			}
		case cc.PostfixExpressionSelect:
			return "*field " + typeOf(x.PostfixExpression).String() + "." + x.Token2.SrcStr()
		case cc.PostfixExpressionPSelect:
			if p, ok := typeOf(x.PostfixExpression).(*cc.PointerType); ok {
				return "*field " + p.Elem().String() + "." + x.Token2.SrcStr()
			}
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionCast {
			return w.ptrKey(x.CastExpression)
		}
	}
	return ""
}

// flow records that pointer key to may hold what the expression e is.
func (w *gaWalk) flow(to string, e cc.ExpressionNode, where cc.Node) {
	if v := unparen(e); v != nil {
		if c := v.Value(); c != nil && fmt.Sprint(c) == "0" {
			return
		}
		if c, ok := v.(*cc.CastExpression); ok && c.Case == cc.CastExpressionCast {
			if k := c.CastExpression.Value(); k != nil && fmt.Sprint(k) == "0" {
				return
			}
		}
	}
	from := w.ptrKey(e)
	if from == "" {
		w.left = append(w.left, Finding{w.fn, where.Position().String(), "a growarray pointer from " + srcOrdered(e)})
		return
	}
	if w.points[to] == nil {
		w.points[to] = map[string]bool{}
	}
	w.points[to][from] = true
}

func (w *gaWalk) ptrToGarray(t cc.Type) bool {
	p, ok := t.(*cc.PointerType)
	return ok && w.isGarray(p.Elem())
}

// visit walks n with its ancestors, innermost last.
func (w *gaWalk) visit(n cc.Node, up []cc.Node) {
	if n == nil {
		return
	}
	switch x := n.(type) {
	case *cc.FunctionDefinition:
		w.fn = x.Declarator.Name()
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionSelect, cc.PostfixExpressionPSelect:
			if x.Token2.SrcStr() == w.ga.Data {
				w.use(x, up)
			}
		case cc.PostfixExpressionCall:
			w.call(x)
		}
	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionAssign {
			lt := typeOf(x.UnaryExpression)
			if w.ptrToGarray(lt) {
				w.flow(w.ptrKey(x.UnaryExpression), x.AssignmentExpression, x)
			}
			if w.isGarray(lt) {
				w.left = append(w.left, Finding{w.fn, x.Token.Position().String(), "a growarray copied: " + srcOrdered(x)})
			}
			if memberName(x.UnaryExpression) == w.ga.ItemSize {
				w.itemsize(x.UnaryExpression, x.AssignmentExpression, x)
			}
		}
	case *cc.JumpStatement:
		if x.Case == cc.JumpStatementReturn && x.ExpressionList != nil {
			if ft := w.fnType[w.fn]; ft != nil && w.ptrToGarray(ft.Result()) {
				w.flow("*"+w.fn+"()", x.ExpressionList, x)
			}
		}
	case *cc.InitDeclarator:
		if x.Initializer != nil && x.Initializer.AssignmentExpression != nil && w.fn != "" {
			t := x.Declarator.Type()
			if w.ptrToGarray(t) {
				w.flow("*"+w.fn+"."+x.Declarator.Name(), x.Initializer.AssignmentExpression, x)
			}
			if w.isGarray(t) {
				w.left = append(w.left, Finding{w.fn, x.Declarator.Position().String(), "a growarray initialized from " + srcOrdered(x.Initializer)})
			}
		}
	}
	up = append(up, n)
	for _, c := range children(n) {
		w.visit(c, up)
	}
}

// sizeofType is T in sizeof(T), or the type of E in sizeof(E).
func sizeofType(e cc.ExpressionNode) cc.Type {
	u, ok := unparen(e).(*cc.UnaryExpression)
	if !ok {
		return nil
	}
	switch u.Case {
	case cc.UnaryExpressionSizeofType:
		return u.TypeName.Type()
	case cc.UnaryExpressionSizeofExpr:
		return typeOf(u.UnaryExpression)
	}
	return nil
}

func (w *gaWalk) itemsize(member, size cc.ExpressionNode, where cc.Node) {
	t := sizeofType(size)
	if t == nil && w.fn == w.ga.Init && w.initSize() != "" && strings.Contains(srcOrdered(size), w.initSize()) {
		return // its calls give the sizeof
	}
	if t == nil {
		if v := size.Value(); v != nil && fmt.Sprint(v) == "1" {
			w.add(member, "bytes", where)
			return
		}
		w.left = append(w.left, Finding{w.fn, where.Position().String(), "an itemsize not a sizeof: " + srcOrdered(size)})
		return
	}
	w.add(member, elemName(t), where)
}

// initSize names the parameter of GrowArray.Init that is the element size.
func (w *gaWalk) initSize() string {
	ft := w.fnType[w.ga.Init]
	if ft == nil || w.ga.InitSize >= len(ft.Parameters()) {
		return ""
	}
	return ft.Parameters()[w.ga.InitSize].Name()
}

// add records typ for the growarray whose member expression m is.
func (w *gaWalk) add(m cc.ExpressionNode, typ string, where cc.Node) {
	x, ok := unparen(m).(*cc.PostfixExpression)
	if !ok {
		return
	}
	key := ""
	if x.Case == cc.PostfixExpressionSelect {
		key = w.objKey(x.PostfixExpression)
	} else {
		key = w.ptrKey(x.PostfixExpression)
	}
	if key == "" {
		w.left = append(w.left, Finding{w.fn, where.Position().String(), "a growarray no key names: " + srcOrdered(m)})
		return
	}
	w.uses = append(w.uses, gaUse{key, typ, w.fn, where.Position().String()})
}

// call records the growarray pointers a call hands to its parameters, and
// the element size GrowArray.Init is given.
func (w *gaWalk) call(x *cc.PostfixExpression) {
	name := callee(x)
	ft := w.fnType[name]
	if ft == nil {
		return
	}
	var args []cc.ExpressionNode
	for l := x.ArgumentExpressionList; l != nil; l = l.ArgumentExpressionList {
		args = append(args, l.AssignmentExpression)
	}
	ps := ft.Parameters()
	for i, a := range args {
		if i >= len(ps) || !w.ptrToGarray(ps[i].Type()) {
			continue
		}
		pname := ""
		if ps[i].Declarator != nil {
			pname = ps[i].Declarator.Name()
		}
		w.flow("*"+name+"."+pname, a, x)
	}
	if name == w.ga.Init && len(args) > w.ga.InitSize {
		if t := sizeofType(args[w.ga.InitSize]); t != nil {
			k := w.ptrKey(args[0])
			w.uses = append(w.uses, gaUse{k, elemName(t), w.fn, x.Position().String()})
		}
	}
}

// use classifies one use of the storage by what its value becomes.
func (w *gaWalk) use(m *cc.PostfixExpression, up []cc.Node) {
	var child cc.Node = m
	for i := len(up) - 1; i >= 0; i-- {
		switch p := up[i].(type) {
		case *cc.PrimaryExpression, *cc.ExpressionList:
			child = p
			continue
		case *cc.CastExpression:
			if p.Case == cc.CastExpressionCast {
				t := p.Type()
				if pt, ok := t.(*cc.PointerType); ok {
					if e := elemName(pt.Elem()); e != "" {
						w.add(m, e, m)
						return
					}
				}
				w.add(m, "", m) // to void *
				return
			}
			child = p
			continue
		case *cc.EqualityExpression:
			w.add(m, "", m) // a test
			return
		case *cc.AssignmentExpression:
			if p.UnaryExpression == child || unparen(p.UnaryExpression) == child {
				w.add(m, "", m) // a store of storage
				return
			}
			w.target(m, typeOf(p.UnaryExpression))
			return
		case *cc.Initializer:
			for j := i - 1; j >= 0; j-- {
				if d, ok := up[j].(*cc.InitDeclarator); ok {
					w.target(m, d.Declarator.Type())
					return
				}
			}
		case *cc.JumpStatement:
			if ft := w.fnType[w.fn]; ft != nil {
				w.target(m, ft.Result())
				return
			}
		case *cc.ArgumentExpressionList:
			call, ok := up[i-countArgLists(up[:i])].(*cc.PostfixExpression)
			if !ok {
				break
			}
			name := callee(call)
			if w.byteFuncs[name] {
				w.add(m, "", m) // bytes, the grow's copy
				return
			}
			ft := w.fnType[name]
			idx := 0
			for l := call.ArgumentExpressionList; l != nil && l != p; l = l.ArgumentExpressionList {
				idx++
			}
			if ft != nil && idx < len(ft.Parameters()) {
				w.target(m, ft.Parameters()[idx].Type())
				return
			}
			if ft != nil && ft.IsVariadic() {
				// a variadic argument: vim_snprintf's %s
				w.add(m, "bytes", m)
				return
			}
		}
		break
	}
	w.left = append(w.left, Finding{w.fn, m.Position().String(), w.ga.Data + " used as nothing this knows: " + srcOrdered(up[len(up)-1])})
}

// countArgLists is how many ArgumentExpressionList nodes end the chain above,
// so the call is the node before them.
func countArgLists(up []cc.Node) int {
	n := 1
	for i := len(up) - 1; i >= 0; i-- {
		if _, ok := up[i].(*cc.ArgumentExpressionList); !ok {
			break
		}
		n++
	}
	return n
}

func (w *gaWalk) target(m *cc.PostfixExpression, t cc.Type) {
	if pt, ok := t.(*cc.PointerType); ok {
		w.add(m, elemName(pt.Elem()), m)
		return
	}
	w.left = append(w.left, Finding{w.fn, m.Position().String(), w.ga.Data + " converted to a " + fmt.Sprint(t)})
}

// GrowArrays partitions every growarray object by its element type.
func GrowArrays(ast *cc.AST, p Profile) Result {
	w := &gaWalk{points: map[string]map[string]bool{}, fnType: map[string]*cc.FunctionType{},
		ga: p.GrowArray, allocators: set(p.Allocators), byteFuncs: set(p.ByteFuncs)}
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		if d, ok := n.(*cc.Declarator); ok {
			if ft, ok := d.Type().(*cc.FunctionType); ok {
				w.fnType[d.Name()] = ft
			}
		}
	})
	for l := ast.TranslationUnit; l != nil; l = l.TranslationUnit {
		w.fn = ""
		w.visit(l.ExternalDeclaration, nil)
	}
	// every key a pointer can reach, through other pointers
	var targets func(k string, seen map[string]bool) []string
	targets = func(k string, seen map[string]bool) []string {
		if !strings.HasPrefix(k, "*") {
			return []string{k}
		}
		if seen[k] {
			return nil
		}
		seen[k] = true
		var r []string
		for f := range w.points[k] {
			r = append(r, targets(f, seen)...)
		}
		return r
	}
	types := map[string]map[string][]string{} // object -> type -> where
	for _, u := range w.uses {
		if u.typ == "" {
			continue
		}
		objs := targets(u.key, map[string]bool{})
		if strings.HasPrefix(u.key, "*") {
			// a generic function's own uses must agree too
			objs = append(objs, u.key)
		}
		for _, o := range objs {
			if types[o] == nil {
				types[o] = map[string][]string{}
			}
			types[o][u.typ] = append(types[o][u.typ], u.fn)
		}
	}
	res := Result{Title: "growarrays", Classes: map[string]int{}, Left: w.left}
	var keys []string
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	nobj, nptr := 0, 0
	for _, k := range keys {
		if len(types[k]) > 1 {
			var ts []string
			for t, fns := range types[k] {
				ts = append(ts, fmt.Sprintf("%s (in %s)", t, strings.Join(uniq(fns), ", ")))
			}
			sort.Strings(ts)
			res.Left = append(res.Left, Finding{"", "", k + " is used as " + strings.Join(ts, " and ")})
			continue
		}
		if strings.HasPrefix(k, "*") {
			nptr++
		} else {
			nobj++
		}
	}
	typed, neutral := 0, 0
	for _, u := range w.uses {
		if u.typ == "" {
			neutral++
		} else {
			typed++
		}
	}
	if os.Getenv("CCX_GARRAYS") != "" {
		for _, k := range keys {
			for ty := range types[k] {
				fmt.Fprintf(os.Stderr, "%-60s %s\n", k, ty)
			}
		}
	}
	res.Classes[fmt.Sprintf("growarray objects, each of one element type (%d; and %d pointers to them)", nobj, nptr)] = nobj
	res.Classes["uses that give an element type: casts, conversions, itemsizes"] = typed
	res.Classes["uses that give none: tests, stores of storage, "+w.ga.Grow+"'s copy"] = neutral
	return res
}

func uniq(s []string) []string {
	m := map[string]bool{}
	var r []string
	for _, x := range s {
		if !m[x] {
			m[x] = true
			r = append(r, x)
		}
	}
	sort.Strings(r)
	return r
}
