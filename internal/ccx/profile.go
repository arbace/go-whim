package ccx

import "regexp"

// Profile is what the checks are told about the program rather than know:
// its allocators, its functions of bytes, its growable-array type and the
// discriminants of its unions.  Nothing in this package names anything in
// the program; whim tells it vim's (internal/whim, CCX).
type Profile struct {
	// Allocators are the calls whose void * result is fresh memory, typed
	// where it is cast (Casts, GrowArrays, VoidPtrs).
	Allocators []string
	// Frees release what an allocator returned: their void * parameter is
	// storage too (VoidPtrs).
	Frees []string
	// ByteFuncs take their pointer arguments as bytes: memmove, memcpy,
	// memset and memcmp, by whatever names the program calls them.
	ByteFuncs []string
	// PureCalls are functions, defined here or not, that change no state a
	// sibling operand can see: they read their arguments and return a
	// value (Order).
	PureCalls []string
	// GrowArray is the program's growable array (GrowArrays); zero when it
	// has none.
	GrowArray GrowArray
	// Unions is what says which member of each union holds (Unions).
	Unions UnionProfile
}

// GrowArray is a growable array: a struct holding its storage as a void *,
// with its element size in a member of its own.
type GrowArray struct {
	Type     string // its typedef name
	Tag      string // its struct tag
	Data     string // the member holding the storage, a void *
	ItemSize string // the member holding the element size
	Init     string // the function that sets the element size from an argument
	InitSize int    // that argument's index
	Grow     string // the function that grows it, copying the storage as bytes
}

// UnionProfile is, for each union the program has, what says which member
// holds where: the facts Unions proves every access against.
type UnionProfile struct {
	// Rules: by the field holding a union, a regexp per member over the
	// guards that say it holds.  `\{base\}` in a regexp is the access's base
	// with that field stripped.
	Rules []UnionRule
	// Kinded is a family of unions whose member is the kind of a row in a
	// table, or nil.
	Kinded *KindedUnion
	// Guarded are calls that read one member and must be made where a
	// discriminant names it.
	Guarded GuardedCalls
	// State is a struct field that says which member of a union holds, and
	// must only move within a member's class; nil when there is none.
	State *StateField
	// Discriminants are the variables the rules' guards test: none may
	// change between the test and the read.
	Discriminants []Discriminant
	// Saves: a function that copies Var to Copy on entry and back before it
	// returns leaves every discriminant under Var as it found it.
	Saves *SavedVar
}

// A UnionRule says which guards show each member of one union holds.
type UnionRule struct {
	Union  string
	Member map[string]*regexp.Regexp
}

// KindedUnion is a family of unions -- Unions, the fields holding them --
// whose member is the kind of a row of Table: a row names its kind by one of
// Kinds' flags and its callbacks by Callback; Setters write the member of
// their kind; a guard `+Flags&F` or `-Flags&F` tests a row's kind.
type KindedUnion struct {
	Unions   []string
	Table    string
	Kinds    map[string]string // a row's flag -> the member of its kind
	Callback *regexp.Regexp    // a callback in a row, as the first group
	Setters  map[string]string // a function -> the member it writes
	Flags    string            // the expression a guard tests the flag in
	Written  string            // the class of an access by a setter of its kind
	Read     string            // the class of an access by a callback only rows of its kind name
}

// GuardedCalls: each of Funcs must be called where its guard holds, the
// guard testing Var; Class is how such a call is counted.
type GuardedCalls struct {
	Funcs []GuardedCall
	Var   string
	Class string
}

// GuardedCall is one function and the guard its calls must be under.
type GuardedCall struct {
	Func, Guard string
}

// StateField is Member, a struct field whose value says which member of a
// union holds: a store into it must keep the value in the class Class says
// it is in, or be Push's store of its own parameter Param.
type StateField struct {
	Member, Push, Param string
	Class               *regexp.Regexp // the values of one class
	Pushed, Moved       string         // the classes the two kinds of store are counted in
}

// Discriminant is a variable guards test, and the guards that test it.
type Discriminant struct {
	Var   string
	Guard *regexp.Regexp
}

// SavedVar is a variable saved to Copy and restored; For is the one
// discriminant whose report names the functions that do it.
type SavedVar struct {
	Var, Copy, For string
}

// set is a list of names as a set.
func set(names ...[]string) map[string]bool {
	m := map[string]bool{}
	for _, ns := range names {
		for _, n := range ns {
			m[n] = true
		}
	}
	return m
}
