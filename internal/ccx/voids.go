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
// can give a type is:
//
//	the functions of bytes (musl_memmove, memcpy, memset, memcmp): bytes
//	the allocators and host_free: storage, typed where it is cast
//
// A prototype's parameters are its function's type, which is checked whole.
//
//	a growarray's ga_data: its element type (GrowArrays)
var voidOwners = map[string]string{
	"musl_memmove": "a function of bytes", "musl_memcpy": "a function of bytes",
	"musl_memset": "a function of bytes", "musl_memcmp": "a function of bytes",
	"lalloc": "an allocator", "lalloc_clear": "an allocator", "alloc": "an allocator", "alloc_clear": "an allocator",
	"host_alloc": "an allocator", "host_free": "an allocator",
	"ga_data": "a growarray's storage",
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
func VoidPtrs(ast *cc.AST) Result {
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
