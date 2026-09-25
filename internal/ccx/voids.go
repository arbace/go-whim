package ccx

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// A void * has no element type, and a translation into Go must give it one.
// VoidPtrs partitions every declaration whose type names a void *: a
// parameter, a variable, a field, a function's result or a typedef, anywhere
// in its type, a function pointer's parameters included.  What the emitter
// can give a type is, by the profile:
//
//	the functions of bytes (ByteFuncs): bytes
//	the allocators and the frees: storage, typed where it is cast
//
// A prototype's parameters are its function's type, which is checked whole.
//
//	a growarray's storage (GrowArray.Data): its element type (GrowArrays)
func voidOwners(p Profile) map[string]string {
	m := map[string]string{}
	for _, n := range p.ByteFuncs {
		m[n] = "a function of bytes"
	}
	for _, n := range append(append([]string{}, p.Allocators...), p.Frees...) {
		m[n] = "an allocator"
	}
	if p.GrowArray.Data != "" {
		m[p.GrowArray.Data] = "a growarray's storage"
	}
	return m
}

// namesVoidPtr says whether a type is, or points at, returns or takes a void
// *; a struct's own fields are declarations of their own.
func namesVoidPtr(t cc.Type) bool {
	switch x := t.(type) {
	case *cc.PointerType:
		return x.Elem().Kind() == cc.Void || namesVoidPtr(x.Elem())
	case *cc.ArrayType:
		return namesVoidPtr(x.Elem())
	case *cc.FunctionType:
		if namesVoidPtr(x.Result()) {
			return true
		}
		for _, p := range x.Parameters() {
			if namesVoidPtr(p.Type()) {
				return true
			}
		}
	}
	return false
}

// VoidPtrs partitions every declaration that names a void *.
func VoidPtrs(ast *cc.AST, p Profile) Result {
	voidOwners := voidOwners(p)
	res := Result{Title: "void pointers", Classes: map[string]int{}}
	walk(ast.TranslationUnit, "", func(n cc.Node, fn string) {
		d, ok := n.(*cc.Declarator)
		if !ok || d.Name() == "" || strings.HasPrefix(d.Position().Filename, "<") || (fn == "" && d.IsParam()) || !namesVoidPtr(d.Type()) {
			return
		}
		owner := d.Name()
		if fn != "" && fn != d.Name() {
			owner = fn // a parameter or a local: its function says what it is
		}
		if c, ok := voidOwners[owner]; ok {
			res.Classes[c]++
			return
		}
		res.Left = append(res.Left, Finding{fn, d.Position().String(), fmt.Sprintf("%s: %s", d.Name(), d.Type())})
	})
	return res
}
