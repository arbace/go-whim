package cut

import (
	"bytes"
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
)

// runtimePaths are the path strings the runtime layer assembles or defaults
// to.  Each becomes empty; NONE is deleted, so the options still exist and
// still report.
var runtimePaths = []string{
	`"$VIMRUNTIME/doc/help.txt"`,
	`"~/.vim,$VIM/vimfiles,$VIMRUNTIME,$VIM/vimfiles/after,~/.vim/after"`,
	`"$VIM/vimfiles,$VIMRUNTIME,$VIM/vimfiles/after"`,
	`"$XDG_CONFIG_HOME/vim,$VIM/vimfiles,$VIMRUNTIME,$VIM/vimfiles/after,` +
		`$XDG_CONFIG_HOME/vim/after"`,
	`"~/.config/vim,$VIM/vimfiles,$VIMRUNTIME,$VIM/vimfiles/after,` +
		`~/.config/vim/after"`,
}

const getenvTest = `vimruntime = (strcmp((char *)(name), (char *)("VIMRUNTIME")) == 0);`

// NoRuntime takes away the runtime directory: the paths that name it, and the
// derivation that invented one.
//
// vim_getenv() derives a runtime directory from argv[0]'s directory when
// $VIMRUNTIME is unset.  Making the test never fire is enough -- the code
// behind it becomes unreachable and the sweep takes it.
func NoRuntime(text []byte, w io.Writer) ([]byte, error) {
	// :help, :runtime and their kin point at ex_ni from phase 1's first
	// steps (exfront, the reform's D2)
	nPath := 0
	for _, s := range runtimePaths {
		c := edit.CountAnchorB(text, s)
		if c == 0 {
			return nil, fmt.Errorf("noruntime: this runtime path is not here any more, so "+
				"the file has moved under this phase: %s", s)
		}
		nPath += c
		text = edit.ReplaceAnchorB(text, s, []byte(`""`), -1)
	}

	if !bytes.Contains(text, []byte(getenvTest)) {
		return nil, fmt.Errorf("noruntime: vim_getenv no longer tests for VIMRUNTIME")
	}
	text = bytes.ReplaceAll(text, []byte(getenvTest), []byte("vimruntime = FALSE;"))

	fmt.Fprintf(w, "  noruntime    %d runtime paths emptied, "+
		"vim_getenv no longer derives one\n", nPath)
	return text, nil
}
