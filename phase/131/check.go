package p131

// Whim phase 131, the check -- the saved input buffer is a garray_T *.
// See phase/131/edit.go, and GOALS.md.
//
// phase/131/check.go: every line naming the saved input buffer is one of
// seven, each saying garray_T where the input said char_u; the two casts are
// gone and no other appeared; the compile is silent.

import (
	"io"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/check"
)

func init() { check.Register("whim131", Check) }

// Whim131 is phase 131's check: the saved input buffer is a garray_T *.
//
//  1. THE MENTIONS, as a partition: every line naming save_inputbuf,
//     get_input_buf or set_input_buf is one of the seven this phase expects --
//     the field, two prototypes, two definitions, the two uses in
//     save_typeahead()/restore_typeahead() -- and each says garray_T where it
//     says a type.  On the input the same seven said char_u.
//  2. NO CAST LEFT: `(char_u *)gap` and `(garray_T *)p` are gone, and no other
//     cast to or from garray_T * appeared.
//  3. THE GATE: a silent compile -- which is itself the proof that nothing
//     else passed a string where the growarray goes -- and the libc surface
//     unchanged.
//
// The recording is the behaviour check: typeahead is saved and restored
// around every Insert-mode CTRL-O and command-line window in the corpus, and
// phase/131/delta declares nothing.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim131", "inputbuf")
	if err != nil {
		return err
	}
	r := c.R
	collect := func(text string) []string {
		seen := map[string]bool{}
		var ls []string
		for _, n := range []string{"save_inputbuf", "get_input_buf", "set_input_buf"} {
			for _, l := range check.LinesWith(text, n) {
				if !seen[l] {
					seen[l] = true
					ls = append(ls, l)
				}
			}
		}
		sort.Strings(ls)
		return ls
	}
	want := []string{
		"garray_T            *save_inputbuf;",
		"static garray_T *get_input_buf(void);",
		"static void set_input_buf(garray_T *gap, int overwrite);",
		"get_input_buf(void)",
		"set_input_buf(garray_T *gap, int overwrite)",
		"tp->save_inputbuf = get_input_buf();",
		"set_input_buf(tp->save_inputbuf, overwrite);",
	}
	sort.Strings(want)
	got := collect(c.New)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		r.Bad("the lines naming the saved input buffer are not the seven expected:\n    %s", strings.Join(got, "\n    "))
	}
	in := collect(c.Old)
	if len(in) != 7 || strings.Count(strings.Join(in, "\n"), "char_u") != 4 {
		r.Bad("the input's seven lines are %d, and %d of them say char_u; this phase was written against 7 and 4", len(in), strings.Count(strings.Join(in, "\n"), "char_u"))
	}
	for _, cast := range []string{"(char_u *)gap", "(garray_T *)p;"} {
		if strings.Contains(c.New, cast) {
			r.Bad("%s is still there", cast)
		}
	}
	if a, b := strings.Count(c.Old, "(garray_T *)"), strings.Count(c.New, "(garray_T *)"); b != a-1 {
		r.Bad("casts to garray_T * went %d -> %d, expected one fewer (set_input_buf's)", a, b)
	}
	if n := check.CountLines([]byte(c.Old)) - check.CountLines([]byte(c.New)); n != 2 {
		r.Bad("the file lost %d lines; set_input_buf()'s local and its blank line are 2", n)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the field, both prototypes, both definitions and both uses say garray_T; the two casts are gone and set_input_buf() takes the growarray directly (2 lines fewer)")
	return c.Gate(true)
}
