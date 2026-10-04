package cut

import (
	"bytes"
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// nofindBody answers "is there a file of this name?" and nothing else.
const nofindBody = `    char_u      *name;

    if (!first)
    {
        return nullptr;
    }

    name = vim_strnsave(ptr, len);
    if (name == nullptr)
    {
        return nullptr;
    }

    if (mch_getperm(name) < 0)
    {
        vim_free(name);
        return nullptr;
    }

    return name;`

// NoFind stops find_file_in_path searching 'path'.
//
// The old body must still delegate to find_file_in_path_option -- which is
// how the tool tells a file it has not seen from one it has already cut,
// rather than replacing its own output with itself.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the delegation is asked of the
// edges (a use of find_file_in_path_option inside the definition), the new
// body is the text's literal made nodes (FRAG: BodyC), and the mentions
// left are the text's count on the C view (history keeps the text version).
func NoFind(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nofind", e, w)
	d := e.Defn("find_file_in_path")
	if d == nil {
		return fmt.Errorf("nofind: find_file_in_path is not defined at file scope")
	}
	delegates := false
	for _, u := range v.UsesOf("find_file_in_path_option") {
		if e.Function(u) == d {
			delegates = true
		}
	}
	if !delegates {
		return fmt.Errorf("nofind: find_file_in_path no longer delegates to the " +
			"'path' search, so this has run already")
	}
	v.BodyC("find_file_in_path", nofindBody,
		`find_file_in_path answers "is there a file of this name?" and nothing else`)
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d find_file_in_path_option mentions left for the sweep",
		bytes.Count(v.Text(), []byte("find_file_in_path_option")))
	return v.Done()
}
