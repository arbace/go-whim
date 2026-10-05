// Package box is the store in Joker, the namespace box: blobs named by
// their SHA-256, in hex, and refs naming blobs (doc/LISP-SANDBOX.md, *The
// store*).  The guest hands it the store's hypercalls (../main.go), the
// host runner a directory of the host's (vmm.DirStore, ../cmd/jokerhost):
// the same functions either way.
//
//	(box/put s)            the blob of s's bytes: its hash
//	(box/get h)            blob h as a string, nil when there is none
//	(box/size h)           its size, nil when there is none
//	(box/ref name)         the hash ref name points at, nil when none
//	(box/ref! name h)      point name at h: true
//	(box/ref! name h old)  only if it points at old (nil: if there is no
//	                       such ref) -- true, or false when it does not
//	(box/load name)        (some-> (box/ref name) box/get)
package box

import (
	"encoding/hex"

	. "github.com/candid82/joker/core"
)

// A Store keeps blobs and refs: vmm.Store's methods.
type Store interface {
	Put(data []byte) ([32]byte, error)
	Size(h [32]byte) (int64, bool)
	Get(h [32]byte, off int64, p []byte) (int, bool)
	Ref(name string) ([32]byte, bool)
	SetRef(name string, h [32]byte, old *[32]byte) error
}

// conflict is how a Store says SetRef's comparison failed: its error has
// Conflict() true (vmm.ErrConflict).
type conflict interface{ Conflict() bool }

// hash is args[i] as a hash; nil, which box/ref answers for no ref, is
// none, so that (box/get (box/ref name)) is nil and not an error.
func hash(args []Object, i int) (h [32]byte, ok bool) {
	if _, none := args[i].(Nil); none {
		return h, false
	}
	s := EnsureArgIsString(args, i).S
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		panic(RT.NewError("not a hash: " + s))
	}
	copy(h[:], b)
	return h, true
}

func str(h [32]byte) Object { return MakeString(hex.EncodeToString(h[:])) }

// get is blob h whole, read in pieces as a guest reads it.
func get(s Store, h [32]byte) ([]byte, bool) {
	n, ok := s.Size(h)
	if !ok {
		return nil, false
	}
	b := make([]byte, n)
	for off := 0; off < len(b); {
		k, ok := s.Get(h, int64(off), b[off:min(len(b), off+1<<20)])
		if !ok || k <= 0 {
			return nil, false
		}
		off += k
	}
	return b, true
}

// Install interns the namespace box's functions on s.  Joker's global
// environment must be initialised and its lock held.
func Install(s Store) {
	ns := GLOBAL_ENV.EnsureSymbolIsNamespace(MakeSymbol("box"))
	def := func(name string, fn func(args []Object) Object) {
		ns.Intern(MakeSymbol(name)).Value = &Proc{Fn: fn, Name: "box/" + name}
	}
	def("put", func(args []Object) Object {
		CheckArity(args, 1, 1)
		h, err := s.Put([]byte(EnsureArgIsString(args, 0).S))
		if err != nil {
			panic(RT.NewError("box/put: " + err.Error()))
		}
		return str(h)
	})
	def("get", func(args []Object) Object {
		CheckArity(args, 1, 1)
		if h, ok := hash(args, 0); ok {
			if b, ok := get(s, h); ok {
				return MakeString(string(b))
			}
		}
		return NIL
	})
	def("size", func(args []Object) Object {
		CheckArity(args, 1, 1)
		if h, ok := hash(args, 0); ok {
			if n, ok := s.Size(h); ok {
				return MakeInt(int(n))
			}
		}
		return NIL
	})
	def("ref", func(args []Object) Object {
		CheckArity(args, 1, 1)
		if h, ok := s.Ref(EnsureArgIsString(args, 0).S); ok {
			return str(h)
		}
		return NIL
	})
	def("ref!", func(args []Object) Object {
		CheckArity(args, 2, 3)
		var old *[32]byte
		if len(args) == 3 {
			old = new([32]byte)
			*old, _ = hash(args, 2)
		}
		h, ok := hash(args, 1)
		if !ok {
			panic(RT.NewError("box/ref!: no hash"))
		}
		err := s.SetRef(EnsureArgIsString(args, 0).S, h, old)
		if c, ok := err.(conflict); ok && c.Conflict() {
			return MakeBoolean(false)
		}
		if err != nil {
			panic(RT.NewError("box/ref!: " + err.Error()))
		}
		return MakeBoolean(true)
	})
	def("load", func(args []Object) Object {
		CheckArity(args, 1, 1)
		h, ok := s.Ref(EnsureArgIsString(args, 0).S)
		if !ok {
			return NIL
		}
		if b, ok := get(s, h); ok {
			return MakeString(string(b))
		}
		return NIL
	})
}
