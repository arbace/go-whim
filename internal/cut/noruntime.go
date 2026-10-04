package cut

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// runtimePaths are the path strings the runtime layer assembles or defaults
// to.  Each becomes empty; NONE is deleted, so the options still exist and
// still report.
//
// 'helpfile', 'runtimepath' and 'packpath' are dropped at phase 1 with every
// option the product has not (optfront, the reform's D3), and with them the
// two defaults only their rows held.
var runtimePaths = []string{
	`"$VIM/vimfiles,$VIMRUNTIME,$VIM/vimfiles/after"`,
	`"$XDG_CONFIG_HOME/vim,$VIM/vimfiles,$VIMRUNTIME,$VIM/vimfiles/after,` +
		`$XDG_CONFIG_HOME/vim/after"`,
	`"~/.config/vim,$VIM/vimfiles,$VIMRUNTIME,$VIM/vimfiles/after,` +
		`~/.config/vim/after"`,
}

// getenvTest is vim_getenv's `vimruntime = (strcmp(name, "VIMRUNTIME") == 0);`.
const getenvTest = `(= vimruntime (paren (== (call strcmp (cast (ptr char) (paren name)) ` +
	`(cast (ptr char) (paren "VIMRUNTIME"))) 0)))`

// NoRuntime takes away the runtime directory: the paths that name it, and the
// derivation that invented one.
//
// vim_getenv() derives a runtime directory from argv[0]'s directory when
// $VIMRUNTIME is unset.  Making the test never fire is enough -- the code
// behind it becomes unreachable and the collection takes it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): each path literal respelled
// "" (RespellString, every occurrence, at least one), the test a Rewrite
// (history keeps the text version).
func NoRuntime(e *graph.Editor, w io.Writer) error {
	// :help, :runtime and their kin point at ex_ni from phase 1's first
	// steps (exfront, the reform's D2)
	v := graph.NewVerbs("noruntime", e, w)
	q := graph.NewVerbs("noruntime", e, io.Discard)
	nPath := 0
	for _, s := range runtimePaths {
		c := 0
		for _, n := range q.Strings() {
			if n.Atom == s {
				c++
			}
		}
		if c == 0 {
			return fmt.Errorf("noruntime: this runtime path is not here any more, so "+
				"the file has moved under this phase: %s", s)
		}
		nPath += c
		q.RespellString(s, `""`, c, "a runtime path")
	}
	n := q.Count(getenvTest)
	if n == 0 {
		return fmt.Errorf("noruntime: vim_getenv no longer tests for VIMRUNTIME")
	}
	q.Rewrite(getenvTest, "(= vimruntime FALSE)", n, "vim_getenv's test")
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("%d runtime paths emptied, vim_getenv no longer derives one", nPath)
	return v.Done()
}
