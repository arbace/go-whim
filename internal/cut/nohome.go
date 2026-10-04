package cut

import (
	"bytes"
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// homeReplaceCopy is what home_replace becomes: a bounded copy.
// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const homeReplaceNote = `// A name is shown as what it is.  This was the shortening of a path under
// $HOME to ~/..., and its thirteen callers are every place that displays a
// file name to the user; they keep working, and see the name unchanged.
`

const homeReplaceCopy = `    usize len;

    if (src == nullptr)
    {
        *dst = NUL;
        return 0;
    }
    len =  strlen((char *)(src)) ;
    if (len >= (usize)dstlen)
    {
        len = (usize)dstlen - 1;
    }
     memmove((char *)(dst), (char *)(src), len) ;
    dst[len] = NUL;
    return len;`

// NoHome takes $HOME out of the editor: the value, the shortening, the
// expansion and the user database.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the call cut as an item, the
// shortening's body the text's literal made nodes (FRAG: BodyC), the user's
// name a one-statement body (BUILD); the lengths and the mentions left are
// the text's numbers, on the C view (history keeps the text version).
func NoHome(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nohome", e, w)
	if v.Count("(call init_homedir)") == 0 {
		return fmt.Errorf("nohome: common_init_2 no longer calls init_homedir")
	}
	v.Cut("(call init_homedir)", 1, "$HOME, read once at startup")

	if e.Defn("home_replace") == nil {
		return fmt.Errorf("nohome: home_replace is not defined at file scope")
	}
	was := 0
	v.InFunction("home_replace", func(v *graph.Verbs) { was = bodyLines(v.Text()) })
	v.BodyC("home_replace", homeReplaceCopy, fmt.Sprintf("home_replace was %d lines, and now shows a name as it is", was))
	// expand_env_esc keeps its ~ arms here: nogetenv, the next step of this
	// phase, replaces its whole body with a copy that expands nothing.

	// ~user completion -- the EXPAND_USER row and the context in
	// set_context_for_wildcard_arg() that reached match_user(), the walk of
	// the password database -- went with every completion context but files
	// at phase 4 (whim4f, phase 4f's program, which runs before this phase
	// now).

	// get_user_name() answers who this is, for a swap file's block zero.
	// There are no swap files; the block is still built in memory, and it can
	// be built without a name.  This is the last reader of getpwuid.
	if !v.Failed() && e.Defn("get_user_name") == nil {
		return fmt.Errorf("nohome: get_user_name is not defined at file scope")
	}
	v.Body("get_user_name", "(return FAIL)", "who this is, which only a swap file wanted to know")
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d homedir mentions left for the sweep", bytes.Count(v.Text(), []byte("homedir")))
	return v.Done()
}
