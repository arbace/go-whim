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

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim133", Edit) }

// W133FindnrBody is buflist_findnr()'s Body after this phase, inside its braces
// (Body writes those), exported so the check requires the
// identical text.
const W133FindnrBody = `    if (curbuf != nullptr && curbuf->b_fnum == nr)
    {
        return curbuf;
    }
    return nullptr;
`

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
	e := edit.New("bufhash", text, w)
	e.Body("buflist_findnr", W133FindnrBody,
		"buflist_findnr() is the current buffer, if its number is nr")
	e.Literal("    if (top_file_num == 1)\n    {\n        hash_init(&buf_hashtab);\n    }\n", "", 1,
		"buflist_new() initialises no table")
	e.Literal("        buf_hashtab_add(buf);\n", "", 1, "and adds the buffer to none")
	e.Literal("\n    buf_hashtab_remove(buf);\n", "\n", 1, "free_buffer() removes it from none")
	return e.Done()
}
