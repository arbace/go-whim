package edit

import (
	"io"
)

func init() { register("whim157", Whim157) }

// Whim157 types the register get_register() hands to put_register().
//
// get_register() returns a copy of a yank register, a yankreg_T, as void *,
// and put_register() takes it back as void * and casts it to yankreg_T *: a
// register carried through void *, the one cast internal/ccx's Casts finds
// that is neither an allocation, a growarray nor a function of bytes.  Both
// functions and the two locals between them (nv_edit's reg1 and reg2) now
// say yankreg_T *.  No code changes: the pointer is the same pointer.
func Whim157(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "register", w: w}
	var err error
	steps := []struct{ old, new, what string }{
		{"static void *get_register(int name, int copy);\nstatic void put_register(int name, void *reg);\n",
			"static yankreg_T *get_register(int name, int copy);\nstatic void put_register(int name, yankreg_T *reg);\n", "get_register() returns a yankreg_T * and put_register() takes one: the prototypes"},
		{"    static void *\nget_register(", "    static yankreg_T *\nget_register(", "get_register()'s definition"},
		{"    return (void *)reg;\n", "    return reg;\n", "which returns the register without a cast"},
		{"put_register(int name, void *reg)\n", "put_register(int name, yankreg_T *reg)\n", "put_register()'s definition"},
		{"    *y_current = *(yankreg_T *)reg;\n", "    *y_current = *reg;\n", "which copies it without a cast"},
		{"    void *reg1 = nullptr;\n    void *reg2 = nullptr;\n", "    yankreg_T *reg1 = nullptr;\n    yankreg_T *reg2 = nullptr;\n", "and the two locals between them hold yankreg_T *"},
	}
	for _, st := range steps {
		if text, err = p.literal(text, st.old, st.new, st.what, 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
