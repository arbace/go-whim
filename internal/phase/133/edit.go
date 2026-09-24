package p133

// Whim phase 133 -- one buffer needs no hash table.  See GOAL.md.
//
// Whim left one buffer, so buf_hashtab held one entry and buflist_findnr(), its
// one reader, recovered the buffer from the key inside it by subtracting the
// key's offset (internal/gen/FINDINGS.md, 3).  buflist_findnr() becomes "the current
// buffer, if its number is nr"; the table's init, add and remove go, and the
// sweep takes the helpers, the table and b_key.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim133", Edit) }

// W133FindnrBody is buflist_findnr()'s Body after this phase, inside its braces
// (cutil.ReplaceBody writes those), exported so the check requires the
// identical text.
const W133FindnrBody = `    if (curbuf != nullptr && curbuf->b_fnum == nr)
    {
        return curbuf;
    }
    return nullptr;`

// Whim133 takes Out the buffer hash table.
//
// Whim left one buffer: buflist_new() is called once, at startup, and curbuf is
// that buffer or NULL.  So buf_hashtab held one entry, and buflist_findnr() --
// its one reader -- recovered the buffer from the key inside it by subtracting
// the key's offset, the container_of the Go transpilation could not say
// (internal/gen/FINDINGS.md, 3).  buflist_findnr() becomes "the current buffer, if its
// number is nr", and the table's init, add and remove go; the sweep takes the
// two helpers, the table, the b_key field and the message nothing says any
// more.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "bufhash", W: w}
	Out, _, err := cutil.ReplaceBody(text, "buflist_findnr", W133FindnrBody)
	if err != nil {
		return nil, p.Die("buflist_findnr: %v", err)
	}
	p.Say("buflist_findnr() is the current buffer, if its number is nr")
	steps := []struct{ Old, New, What string }{
		{"    if (top_file_num == 1)\n    {\n        hash_init(&buf_hashtab);\n    }\n", "",
			"buflist_new() initialises no table"},
		{"        buf_hashtab_add(buf);\n", "", "and adds the buffer to none"},
		{"\n    buf_hashtab_remove(buf);\n", "\n", "free_buffer() removes it from none"},
	}
	for _, s := range steps {
		if Out, err = p.Literal(Out, s.Old, s.New, s.What, 1); err != nil {
			return nil, err
		}
	}
	return Out, nil
}
