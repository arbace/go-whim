package p061

// Whim phase 61 (formerly 133) -- one buffer needs no hash table.  See GOAL.md.
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

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim61", Edit) }

// W61FindnrBody is buflist_findnr()'s Body after this phase, inside its braces
// (Body writes those), exported so the check requires the
// identical text.
const W61FindnrBody = `    if (curbuf != nullptr && curbuf->b_fnum == nr)
    {
        return curbuf;
    }
    return nullptr;
`

// Edit takes out the buffer hash table.
//
// Whim left one buffer: buflist_new() is called once, at startup, and curbuf is
// that buffer or NULL.  So buf_hashtab held one entry, and buflist_findnr() --
// its one reader -- recovered the buffer from the key inside it by subtracting
// the key's offset, the container_of the Go transpilation could not say
// (internal/gen/FINDINGS.md, 3).  buflist_findnr() becomes "the current buffer, if its
// number is nr", and the table's init, add and remove go; the collection takes the
// two helpers, the table, the b_key field and the message nothing says any
// more.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the body by FRAG, the three
// statements cut by their forms; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("bufhash", e, w)
	v.BodyC("buflist_findnr", W61FindnrBody, "buflist_findnr() is the current buffer, if its number is nr")
	v.Cut("(if (== top_file_num 1) (block (call hash_init (addr buf_hashtab))))", 1, "buflist_new() initialises no table")
	v.Cut("(call buf_hashtab_add buf)", 1, "and adds the buffer to none")
	v.Cut("(call buf_hashtab_remove buf)", 1, "free_buffer() removes it from none")
	return v.Done()
}
