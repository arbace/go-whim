package edit

import (
	"io"
)

func init() { register("whim131", Whim131) }

// Whim131 gives the saved input buffer its type.
//
// save_typeahead() keeps what inbuf[] held in tasave_T.save_inputbuf, and
// get_input_buf() makes it: a garray_T, allocated and filled -- then cast to
// char_u * to be stored, and cast back by set_input_buf().  Nothing reads it
// as characters.  The Go transpilation could not carry a growarray in a
// string pointer and had to register it under a one-byte key
// (tx/FINDINGS.md, 6).  The field, both prototypes, the definition and the
// return say garray_T *, and set_input_buf() takes the garray_T it always
// cast its argument to.
func Whim131(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "inputbuf", w: w}
	var err error
	steps := []struct{ old, new, what string }{
		{"    char_u              *save_inputbuf;\n", "    garray_T            *save_inputbuf;\n",
			"tasave_T.save_inputbuf is a garray_T *"},
		{"static char_u *get_input_buf(void);\n", "static garray_T *get_input_buf(void);\n",
			"get_input_buf() is declared to return one"},
		{"static void set_input_buf(char_u *p, int overwrite);\n", "static void set_input_buf(garray_T *gap, int overwrite);\n",
			"set_input_buf() is declared to take one"},
		{"    static char_u *\nget_input_buf(void)\n", "    static garray_T *\nget_input_buf(void)\n",
			"get_input_buf() is defined to return one"},
		{"    trash_input_buf();\n    return (char_u *)gap;\n}\n", "    trash_input_buf();\n    return gap;\n}\n",
			"and returns it without the cast"},
		{"set_input_buf(char_u *p, int overwrite)\n{\n    garray_T    *gap = (garray_T *)p;\n\n", "set_input_buf(garray_T *gap, int overwrite)\n{\n",
			"set_input_buf() takes the garray_T it used to cast its argument to"},
	}
	for _, s := range steps {
		if text, err = p.literal(text, s.old, s.new, s.what, 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
