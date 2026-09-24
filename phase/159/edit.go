package p159

// Whim phase 159 -- a struct's text is a pointer to an allocation of its own.  See GOAL.md.
//
// buffblock_T, msgchunk_T and regprog_T ended in a one-element array sized
// at allocation.  Each is now a char_u * to an allocation of its own.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim159", Edit) }

// W159Types are the three structs whose last member was a one-element array
// sized at allocation.
var W159Types = []string{"buffblock_T", "msgchunk_T", "regprog_T"}

// Whim159 makes each of the three struct hacks a pointer to an allocation of
// its own.
//
// buffblock_T's b_str, msgchunk_T's sb_text and regprog_T's program were
// declared char_u x[1] and allocated with the struct, offsetof(T, x) + n bytes
// at once: an array whose length is not its type's.  Each is now a char_u *,
// and the struct and its bytes are two allocations.  Nothing copies, sizes or
// declares a msgchunk_T or a regprog_T; a buffblock_T is declared once, as
// the head of each of the five buffer lists, whose one byte is written only
// with NUL (delete_buff_tail() of nothing) and read never.  Each head points
// at a one-byte array of its own, a compound literal with static storage.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "structhack", W: w}
	var err error
	steps := []struct {
		Old, New, What string
		n              int
	}{
		{"    usize b_strlen;\n    char_u b_str[1];\n", "    usize b_strlen;\n    char_u *b_str;\n", "a buffblock_T's text is a char_u *", 1},
		{"        p = alloc(__builtin_offsetof(buffblock_T, b_str) + len + 1);\n", "        p = alloc(sizeof(buffblock_T));\n        p->b_str = alloc(len + 1);\n", "allocated apart from the block", 1},
		{"    int sb_attr;\n    char_u sb_text[1];\n", "    int sb_attr;\n    char_u *sb_text;\n", "a msgchunk_T's text is a char_u *", 1},
		{"        mp = alloc(__builtin_offsetof(msgchunk_T, sb_text) + (s - *sb_str) + 1);\n", "        mp = alloc(sizeof(msgchunk_T));\n        mp->sb_text = alloc((s - *sb_str) + 1);\n", "allocated apart from the chunk", 1},
		{"    int regmlen;\n    char_u program[1];\n", "    int regmlen;\n    char_u *program;\n", "a regprog_T's program is a char_u *", 1},
		{"    r = alloc(__builtin_offsetof(regprog_T, program) + regsize);\n", "    r = alloc(sizeof(regprog_T));\n    r->program = alloc(regsize);\n", "allocated apart from the regprog_T", 1},
	}
	for _, st := range steps {
		if text, err = p.Literal(text, st.Old, st.New, st.What, st.n); err != nil {
			return nil, err
		}
	}
	for _, h := range []string{"redobuff", "old_redobuff", "recordbuff", "readbuf1", "readbuf2"} {
		// The canonical text writes an aggregate initialiser one element per
		// line, with the brace under the `=`, so the head is the first element's
		// line and not the first field of a one-line row.
		old := "static buffheader_T " + h + " =\n{\n    {nullptr, 0, {NUL}},\n"
		if text, err = p.Literal(text, old, "static buffheader_T "+h+" =\n{\n    {nullptr, 0, (char_u[1]){NUL}},\n", "the head of "+h+" points at a byte of its own", 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
