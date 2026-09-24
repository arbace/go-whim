package p048

// Whim phase 48 -- no :noswapfile.  See GOAL.md.
//
// There has been no swap file since Phase 21: the memfile is memory.  The
// modifier set CMOD_NOSWAPFILE, and its two readers, in ml_open() and
// buf_copy_options(), were already empty blocks.  The modifier is matched by name
// before the table, so its branch goes as well as its row.
//
// THE DELTA: the row, which succeeded run bare.

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

var (
	noswapModifier = regexp.MustCompile(`(?m)^[ \t]*case 'n':\n[ \t]*if \(!checkforcmd_noparen\(&eap->cmd, "noswapfile", 3\)\)\n[ \t]*\{\n[ \t]*break;\n[ \t]*\}\n[ \t]*cmod->cmod_flags \|= CMOD_NOSWAPFILE;\n[ \t]*continue;\n`)
	noswapComplete = regexp.MustCompile(`(?m)^[ \t]*case CMD_noswapfile:\n`)
	noswapTest     = `(?m)^[ \t]*if \(cmdmod\.cmod_flags & CMOD_NOSWAPFILE\)$`
	cmodNoSwapFile = regexp.MustCompile(`\bCMOD_NOSWAPFILE\b`)
)

// cutOnce deletes the pattern's single match, refusing on any other count.
func cutOnce(re *regexp.Regexp, what string) func([]byte) ([]byte, error) {
	return func(s []byte) ([]byte, error) {
		if n := len(re.FindAll(s, -1)); n != 1 {
			return nil, fmt.Errorf("whim48: %s -- matched %d times", what, n)
		}
		return re.ReplaceAll(s, nil), nil
	}
}

// Whim48 takes the :noswapfile modifier and everything that reads it.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	steps := []struct {
		fn, What string
		re       *regexp.Regexp
	}{
		{"parse_command_modifiers", "the :noswapfile modifier", noswapModifier},
		{"set_context_by_cmdname", "completion for :noswapfile", noswapComplete},
	}
	var err error
	for _, s := range steps {
		if text, err = edit.InFunction(text, s.fn, cutOnce(s.re, s.What)); err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "  noswapfile   %s\n", s.What)
	}

	if text, err = edit.InFunction(text, "ml_open", func(s []byte) ([]byte, error) {
		return cutil.DropIf(s, noswapTest, 1)
	}); err != nil {
		return nil, err
	}
	fmt.Fprintln(w, "  noswapfile   ml_open asking for it")

	if text, err = edit.InFunction(text, "buf_copy_options", func(s []byte) ([]byte, error) {
		return cutil.FoldNever(s, noswapTest, 1)
	}); err != nil {
		return nil, err
	}
	fmt.Fprintln(w, "  noswapfile   buf_copy_options asking for it")

	// The enumerator is the only mention that may survive: anything else is a
	// reader this phase did not find, and shipping it would be the phase
	// half-done.
	if n := len(cmodNoSwapFile.FindAll(text, -1)); n != 1 {
		return nil, fmt.Errorf("whim48: CMOD_NOSWAPFILE outside its enumerator -- %d mentions, expected 1", n)
	}
	return text, nil
}

func init() { edit.Register("whim48", Edit) }
