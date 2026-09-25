package p157

// Whim phase 157 -- get_register() and put_register() carry a yankreg_T *, not a void *.  See GOAL.md.
//
// get_register() returns a register as void * and put_register() casts it
// back: the register is typed yankreg_T * throughout, and the core's last
// pointer cast outside the classes internal/ccx names goes.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim157", Edit) }

// Whim157 types the register get_register() hands to put_register().
//
// get_register() returns a copy of a yank register, a yankreg_T, as void *,
// and put_register() takes it back as void * and casts it to yankreg_T *: a
// register carried through void *, the one cast internal/ccx's Casts finds
// that is neither an allocation, a growarray nor a function of bytes.  Both
// functions and the two locals between them (nv_edit's reg1 and reg2) now
// say yankreg_T *.  No code changes: the pointer is the same pointer.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("register", text, w)
	e.Literal("static void *get_register(int name, int copy);\n\nstatic void put_register(int name, void *reg);\n", "static yankreg_T *get_register(int name, int copy);\n\nstatic void put_register(int name, yankreg_T *reg);\n", 1,
		"get_register() returns a yankreg_T * and put_register() takes one: the prototypes")
	e.Literal("    static void *\nget_register(", "    static yankreg_T *\nget_register(", 1,
		"get_register()'s definition")
	e.Literal("    return (void *)reg;\n", "    return reg;\n", 1,
		"which returns the register without a cast")
	e.Literal("put_register(int name, void *reg)\n", "put_register(int name, yankreg_T *reg)\n", 1,
		"put_register()'s definition")
	e.Literal("    *y_current = *(yankreg_T *)reg;\n", "    *y_current = *reg;\n", 1,
		"which copies it without a cast")
	e.Literal("    void *reg1 = nullptr;\n    void *reg2 = nullptr;\n", "    yankreg_T *reg1 = nullptr;\n    yankreg_T *reg2 = nullptr;\n", 1,
		"and the two locals between them hold yankreg_T *")
	return e.Done()
}
