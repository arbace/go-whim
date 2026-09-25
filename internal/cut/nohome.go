package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/cutil"
)

// homeReplaceCopy is what home_replace becomes: a bounded copy.
// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
const homeReplaceNote = `// A name is shown as what it is.  This was the shortening of a path under
// $HOME to ~/..., and its thirteen callers are every place that displays a
// file name to the user; they keep working, and see the name unchanged.
`

const homeReplaceCopy = `    size_t len;

    if (src == NULL)
    {
        *dst = NUL;
        return 0;
    }
    len =  strlen((char *)(src)) ;
    if (len >= (size_t)dstlen)
    {
        len = (size_t)dstlen - 1;
    }
     memmove((char *)(dst), (char *)(src), len) ;
    dst[len] = NUL;
    return len;`

var (
	initHomedir  = regexp.MustCompile(cutil.Line("init_homedir();"))
	expandUser   = regexp.MustCompile(`[ \t]*\{EXPAND_USER, get_users, TRUE, FALSE\},\n`)
	userNameHead = `^[ \t]*if \(\*xp->xp_pattern == '~'\)$`
)

// NoHome takes $HOME out of the editor: the value, the shortening, the
// expansion and the user database.
func NoHome(text []byte, w io.Writer) ([]byte, error) {
	if !initHomedir.Match(text) {
		return nil, fmt.Errorf("nohome: common_init_2 no longer calls init_homedir")
	}
	text, _ = replaceFirst(initHomedir, text, "")
	fmt.Fprintln(w, "  nohome       $HOME, read once at startup")

	ho, hc, found, _ := cutil.Body(text, "home_replace")
	if !found {
		return nil, fmt.Errorf("nohome: home_replace is not defined at file scope")
	}
	was := bytes.Count(text[ho:hc], []byte{'\n'})
	var hbuf []byte
	hbuf = append(hbuf, text[:ho]...)
	hbuf = append(hbuf, "{\n"...)
	hbuf = append(hbuf, homeReplaceCopy...)
	hbuf = append(hbuf, "\n}"...)
	text = append(hbuf, text[hc+1:]...)
	var err error
	fmt.Fprintf(w, "  nohome       home_replace was %d lines, and now shows a name as "+
		"it is\n", was)
	// expand_env_esc keeps its ~ arms here: nogetenv, the next step of this
	// phase, replaces its whole body with a copy that expands nothing.

	if n := len(expandUser.FindAll(text, -1)); n != 1 {
		return nil, fmt.Errorf("nohome: expected one EXPAND_USER completion row, got %d", n)
	}
	text = expandUser.ReplaceAll(text, nil)

	// And the context that reaches it.  Removing the completion row alone
	// leaves match_user() called from set_context_for_wildcard_arg(), and
	// match_user() is what walks the password database -- so the five pw
	// symbols stayed until this went too.
	if text, err = cutil.DropIf(text, "(?m)"+userNameHead, 1); err != nil {
		return nil, err
	}
	fmt.Fprintln(w, "  nohome       ~user completion, and the context that reaches it")

	// get_user_name() answers who this is, for a swap file's block zero.
	// There are no swap files; the block is still built in memory, and it can
	// be built without a name.  This is the last reader of getpwuid.
	go_, gc, found, _ := cutil.Body(text, "get_user_name")
	if !found {
		return nil, fmt.Errorf("nohome: get_user_name is not defined at file scope")
	}
	var gbuf []byte
	gbuf = append(gbuf, text[:go_]...)
	gbuf = append(gbuf, "{\n    return FAIL;\n}"...)
	text = append(gbuf, text[gc+1:]...)
	fmt.Fprintln(w, "  nohome       who this is, which only a swap file wanted to know")

	fmt.Fprintf(w, "  nohome       %d homedir mentions left for the sweep\n",
		bytes.Count(text, []byte("homedir")))
	return text, nil
}
