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
	Type string // the Go type a pointer to one is written as a pointer to
	Data string // the member holding the storage
}

// RuntimeBody is a function whose Go body is given: Body is it, for the Go
// type the C gives its result ("" when the function is not in the C).
type RuntimeBody struct {
	Name string
	Body func(result string) string
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
