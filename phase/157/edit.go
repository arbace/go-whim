package p157

// Whim phase 157 -- get_register() and put_register() carry a yankreg_T *, not a void *.  See GOAL.md.
//
// get_register() returns a register as void * and put_register() casts it
// back: the register is typed yankreg_T * throughout, and the core's last
// pointer cast outside the classes internal/ccx names goes.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

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
	p := edit.Ph{Tag: "register", W: w}
	var err error
	steps := []struct{ Old, New, What string }{
		{"static void *get_register(int name, int copy);\n\nstatic void put_register(int name, void *reg);\n",
			"static yankreg_T *get_register(int name, int copy);\nstatic void put_register(int name, yankreg_T *reg);\n", "get_register() returns a yankreg_T * and put_register() takes one: the prototypes"},
		{"    static void *\nget_register(", "    static yankreg_T *\nget_register(", "get_register()'s definition"},
		{"    return (void *)reg;\n", "    return reg;\n", "which returns the register without a cast"},
		{"put_register(int name, void *reg)\n", "put_register(int name, yankreg_T *reg)\n", "put_register()'s definition"},
		{"    *y_current = *(yankreg_T *)reg;\n", "    *y_current = *reg;\n", "which copies it without a cast"},
		{"    void *reg1 = nullptr;\n    void *reg2 = nullptr;\n", "    yankreg_T *reg1 = nullptr;\n    yankreg_T *reg2 = nullptr;\n", "and the two locals between them hold yankreg_T *"},
	}
	for _, st := range steps {
		if text, err = p.Literal(text, st.Old, st.New, st.What, 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
