// Package ccwalk walks crefactor/cc's syntax tree: Walk visits every node,
// WalkTok every token, in source order -- the exported fields of each node
// that hold a token, a node or an interface a node is stored in, in
// declaration order.
//
// THE WALKS ARE A GENERATED TYPE SWITCH (walk_gen.go, `go generate
// ./ccwalk`), with reflection behind it for a node type the switch was not
// generated for.  Reflection was every walk once: asking for each node's
// fields and boxing each through reflect.Value.Interface was 169 s of the
// 1,274 s of CPU an in-order build took.  TestWalkGen holds the two to each
// other.
package ccwalk

import (
	"reflect"
	"sync"

	"github.com/arbace/go-whim/crefactor/cc"
)

//go:generate go run ./walkgen

// fields is, per node type, which fields a walk visits: the exported ones
// holding a node or a token.  Asking reflect for them at every node was most of
// a sweep's time; they are a fact about the type.
var fields sync.Map // reflect.Type -> []fieldAt

type fieldAt struct {
	i     int
	token bool
}

var (
	nodeType  = reflect.TypeOf((*cc.Node)(nil)).Elem()
	tokenType = reflect.TypeOf(cc.Token{})
)

func fieldsOf(t reflect.Type) []fieldAt {
	if f, ok := fields.Load(t); ok {
		return f.([]fieldAt)
	}
	var out []fieldAt
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		switch {
		case sf.Type == tokenType:
			out = append(out, fieldAt{i, true})
		case sf.Type.Implements(nodeType), sf.Type.Kind() == reflect.Interface:
			// a node, or an interface one is stored in (ExpressionNode)
			out = append(out, fieldAt{i, false})
		}
	}
	fields.Store(t, out)
	return out
}

// walkTokReflect is WalkTok by reflection: what the generated WalkTok
// (walk_gen.go) does for a node type it was not generated for.
func walkTokReflect(n cc.Node, f func(cc.Token)) {
	v := reflect.ValueOf(n)
	if n == nil || v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for _, fa := range fieldsOf(e.Type()) {
		fv := e.Field(fa.i)
		if fa.token {
			f(fv.Interface().(cc.Token))
			continue
		}
		if x, ok := fv.Interface().(cc.Node); ok {
			WalkTok(x, f)
		}
	}
}

// walkReflect is Walk by reflection: what the generated Walk (walk_gen.go)
// does for a node type it was not generated for.  It was every walk, and
// most of a sweep's time.
func walkReflect(n cc.Node, f func(cc.Node) bool) {
	v := reflect.ValueOf(n)
	if n == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return
	}
	if !f(n) {
		return
	}
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for _, fa := range fieldsOf(e.Type()) {
		if fa.token {
			continue
		}
		if x, ok := e.Field(fa.i).Interface().(cc.Node); ok {
			Walk(x, f)
		}
	}
}
