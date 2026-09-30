package togo

// Profile is what the generator is told about the C it translates rather
// than knows: the functions the hand-written runtime replaces, the
// allocators and the functions of bytes it translates to the runtime's, the
// program's growable array, and the parameters that pun.  Nothing in the
// generator names anything in the program; whim tells it vim's core
// (internal/whim, Gen).
type Profile struct {
	// Header opens the file -editor writes, the package clause included.
	Header string
	// Package is the Go package the files are in; "" is main.
	Package string
	// Rename maps a C identifier Go cannot spell as it is to its Go name --
	// `_`, Go's blank -- before the reserved words' trailing underscore.
	Rename map[string]string

	// Runtime are the C functions the runtime replaces: their calls are
	// translated, their bodies are not, and a pointer that reaches one
	// stays a Ptr (forward).
	Runtime []string
	// RuntimeBodies are the functions whose body is a rule of the runtime's
	// rather than a translation, written before the translated bodies, in
	// this order.
	RuntimeBodies []RuntimeBody

	// Allocators return fresh storage, sized in bytes by their first
	// argument: new(T), Mk[T](n), Alloc(n) or make([]byte, n).
	Allocators []string
	// Frees release what an allocator returned: an argument handed to one
	// flows nowhere, so it does not make a class of pointers walk.
	Frees []string
	// Bytes are the functions of bytes, translated to the runtime's.
	Bytes ByteFuncs
	// SizeType is the Go type of a sizeof: the program's own name for
	// size_t, declared by its typedef; "" is uint64.
	SizeType string

	// GrowArray is the program's growable array; zero when it has none.
	GrowArray GrowArray
	// Puns are parameter names: a char * parameter so named holds an
	// object of whatever type, and its class is a pun.
	Puns []string

	// Instance, when set, is the instance pass applied to the file -editor
	// writes: the state a struct's fields, the functions its methods
	// (instance.go).  Own is that file; Fields and Methods are what the
	// package's hand-written files declare (HandNames).
	Instance *Instance

	// JavaPackage is the package of the Java class -java writes; "" is the
	// unnamed package.  The class is named after the file.
	JavaPackage string
	// JavaFiles writes the class in its package as Java writes one: in the
	// -java file's directory, the class's file holds its fields and
	// methods, Constants.java the constants (imported statically), and each
	// struct class and function interface a file of its own.  It needs a
	// JavaPackage, since the unnamed package cannot be imported from.
	JavaFiles bool

	// CljNamespace is the namespace -clj writes; "" is whim.editor.
	CljNamespace string
	// CljHost is the namespace a function declared and not defined is
	// called in, the editor first; "" is whim.cljhost.
	CljHost string
	// CljExports are file-scope objects the host reads: each a function of
	// the editor of the same name.
	CljExports []string
	// CljSplit is the size, in the backend's guess at bytecode, past which a
	// function is split into functions of its states (0: its default).
	CljSplit int
	// CljOutline is the size, in the same guess, past which a region's code
	// or a loop of a function too large for one method is written as a
	// function of its own (0: the backend's default).
	CljOutline int
	// HsModule and HsHost are the Haskell module's name and its host's (the
	// module the functions declared and not defined are called in).
	HsModule, HsHost string
	// HsExports are the functions and file-scope objects the Haskell host
	// calls back: declared in the module's hs-boot interface, which the
	// host imports {-# SOURCE #-}, an object as its address, addr'NAME.
	HsExports []string
	// CljGlue are the functions the hand-written glue calls by name, the
	// editor first: they keep it, whatever they do (clj_ed.go).
	CljGlue []string

	// CljParts, when more than 1, is how many files the Clojure namespace's
	// functions are written in: the namespace's own file holds the rest and
	// loads them, `(load "editor/part1")`, each beginning `(in-ns ...)` --
	// clojure.core's own split.  A file ahead of time is a class whose one
	// method, load(), runs its top-level forms and is held to 64 KB, 24
	// bytes a function; the functions keep their order, so what a part
	// calls is defined or declared before it.
	CljParts int

	// HsParts is how many modules the Haskell functions are split into, by
	// the call graph (hssplit.go): 0 or 1, one module.
	HsParts int
}

// ByteFuncs are the program's memmove and memcpy (Memmove), memset
// (Memset, Zero or a fill) and memcmp (Memcmp, or == for two structs).
type ByteFuncs struct {
	Move     []string
	Set, Cmp string
}

// GrowArray is a growable array whose storage member, a void *, the runtime
// types at its first use: GaData[T](gap).
type GrowArray struct {
	Type   string // the Go type a pointer to one is written as a pointer to
	Data   string // the member holding the storage
	MaxLen string // the member holding how many elements it has asked for (the Java's Ga)
}

// RuntimeBody is a function whose Go body is given: Body is it, for the Go
// type the C gives its result ("" when the function is not in the C).
// Java, when set, is the Java method's body -- its statements, the C's
// parameter names in scope -- for the Java type the C gives its result.
type RuntimeBody struct {
	Name string
	Body func(result string) string
	Java func(result string) string
	// Clj, when set, is the Clojure function's body -- one expression, the
	// C's parameter names in scope -- for the Java type the C gives its
	// result (the Clojure backend takes the Java's types).
	Clj func(result string) string
	// Hs, when set, is the Haskell function's body -- the lines of its do
	// block, the C's parameter names in scope, the editor ed' -- for the
	// Haskell type the C gives its result.
	Hs func(result string) string
}

// profile is a Profile's lists as sets, for the lookups.
type profile struct {
	Profile
	runtime, allocators, frees, puns map[string]bool
}

func (p Profile) sets() *profile {
	set := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	return &profile{Profile: p, runtime: set(p.Runtime), allocators: set(p.Allocators), frees: set(p.Frees), puns: set(p.Puns)}
}

// sizeType is Profile.SizeType, uint64 when the profile names none.
func (g *gen) sizeType() string {
	if g.p.SizeType == "" {
		return "uint64"
	}
	return g.p.SizeType
}

func (p *profile) byteMove(name string) bool {
	for _, m := range p.Bytes.Move {
		if m == name {
			return true
		}
	}
	return false
}

// pkg is the package clause of every file the generator writes.
func (p Profile) pkg() string {
	if p.Package == "" {
		return "package main\n\n"
	}
	return "package " + p.Package + "\n\n"
}
