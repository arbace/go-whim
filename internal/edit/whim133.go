package edit

import (
	"io"

	"github.com/arbace/go-whim/internal/cutil"
)

func init() { register("whim133", Whim133) }

// W133FindnrBody is buflist_findnr()'s body after this phase, inside its braces
// (cutil.ReplaceBody writes those), exported so the check requires the
// identical text.
const W133FindnrBody = `    if (curbuf != nullptr && curbuf->b_fnum == nr)
    {
        return curbuf;
    }
    return nullptr;`

// Whim133 takes out the buffer hash table.
//
// Whim left one buffer: buflist_new() is called once, at startup, and curbuf is
// that buffer or NULL.  So buf_hashtab held one entry, and buflist_findnr() --
// its one reader -- recovered the buffer from the key inside it by subtracting
// the key's offset, the container_of the Go transpilation could not say
// (tx/FINDINGS.md, 3).  buflist_findnr() becomes "the current buffer, if its
// number is nr", and the table's init, add and remove go; the sweep takes the
// two helpers, the table, the b_key field and the message nothing says any
// more.
func Whim133(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "bufhash", w: w}
	out, _, err := cutil.ReplaceBody(text, "buflist_findnr", W133FindnrBody)
	if err != nil {
		return nil, p.die("buflist_findnr: %v", err)
	}
	p.say("buflist_findnr() is the current buffer, if its number is nr")
	steps := []struct{ old, new, what string }{
		{"    if (top_file_num == 1)\n    {\n        hash_init(&buf_hashtab);\n    }\n\n", "",
			"buflist_new() initialises no table"},
		{"        buf_hashtab_add(buf);\n", "", "and adds the buffer to none"},
		{"\n    buf_hashtab_remove(buf);\n", "\n", "free_buffer() removes it from none"},
	}
	for _, s := range steps {
		if out, err = p.literal(out, s.old, s.new, s.what, 1); err != nil {
			return nil, err
		}
	}
	return out, nil
}
