package p131

// Whim phase 131 -- the saved input buffer is a garray_T *.  See GOAL.md.
//
// get_input_buf() makes a garray_T, casts it to char_u * for
// tasave_T.save_inputbuf, and set_input_buf() casts it back; nothing reads it as
// characters.  The field, the prototypes, the definition and the return say
// what it is (internal/gen/FINDINGS.md, 6).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim131", Edit) }

// Whim131 gives the saved input buffer its type.
//
// save_typeahead() keeps what inbuf[] held in tasave_T.save_inputbuf, and
// get_input_buf() makes it: a garray_T, allocated and filled -- then cast to
// char_u * to be stored, and cast back by set_input_buf().  Nothing reads it
// as characters.  The Go transpilation could not carry a growarray in a
// string pointer and had to register it under a one-byte key
// (internal/gen/FINDINGS.md, 6).  The field, both prototypes, the definition and the
// return say garray_T *, and set_input_buf() takes the garray_T it always
// cast its argument to.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("inputbuf", text, w)
	e.Literal("    char_u *save_inputbuf;\n", "    garray_T *save_inputbuf;\n", 1,
		"tasave_T.save_inputbuf is a garray_T *")
	e.Literal("static char_u *get_input_buf(void);\n", "static garray_T *get_input_buf(void);\n", 1,
		"get_input_buf() is declared to return one")
	e.Literal("static void set_input_buf(char_u *p, int overwrite);\n", "static void set_input_buf(garray_T *gap, int overwrite);\n", 1,
		"set_input_buf() is declared to take one")
	e.Literal("    static char_u *\nget_input_buf(void)\n", "    static garray_T *\nget_input_buf(void)\n", 1,
		"get_input_buf() is defined to return one")
	e.Literal("    trash_input_buf();\n    return (char_u *)gap;\n}\n", "    trash_input_buf();\n    return gap;\n}\n", 1,
		"and returns it without the cast")
	e.Literal("set_input_buf(char_u *p, int overwrite)\n{\n    garray_T *gap = (garray_T *)p;\n", "set_input_buf(garray_T *gap, int overwrite)\n{\n", 1,
		"set_input_buf() takes the garray_T it used to cast its argument to")
	return e.Done()
}
