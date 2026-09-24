package p054

// Whim phase 54 -- no option without a variable.  See GOAL.md.
//
// A row whose variable is (char_u *)NULL is an option :set accepts, reports and
// ignores: its feature was never compiled in, or went in an earlier phase.  The
// set is COMPUTED from options[] rather than listed, so a row a later upstream
// adds without a variable goes too.
//
// 174 OF THEM, AND IT WAS 172 BEFORE THE TEXT BECAME CANONICAL.  'statusline'
// and 'statuslineopt' are no-variable rows like the rest, and the anchor missed
// them because vim writes those two rows with spaces before the comma --
// {"statusline"  ,"stl", -- which is upstream's own text.  Phase 0 prints one
// shape per construct, so the computation sees all of them.  Nothing here
// excludes a name and nothing lists one: that is the point of computing the set.
//
// THE DELTA: none the harnesses record -- no case sets an option without a
// variable, and the Ex sweep runs command names, not option names, so the two
// that arrived are a change the corpus cannot see.  The probes check four of
// them are now unknown.
// The variable field, not the row: a string default is often (char_u *)NULL too.
// The other two traps were spelling and the canonical text retired them: the
// spacing varied -- 'termguicolors' was (char_u*)NULL -- and a row's flags could
// wrap onto a second line ('diffopt', 'foldmarker', 'guifont', 'guifontwide',
// 'breakindentopt', 'undodir').  The pattern still tolerates both.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

// noVarRow matches an options[] row whose VARIABLE FIELD is a null pointer --
// not the row's default, since a string default is often (char_u *)NULL too.
//
// Two things the pattern has to tolerate, both recorded by the phase: the
// spacing varies, 'termguicolors' being (char_u*)NULL; and a row's flags can
// wrap onto a second line -- 'diffopt', 'foldmarker', 'guifont', 'guifontwide',
// 'breakindentopt' and 'undodir' -- so the flag list allows whitespace.  A first
// version without it left those six behind.
var noVarRow = regexp.MustCompile(`(?m)^[ \t]*\{"(\w+)",\s*(?:"\w*"|NULL),\s*P_[\w|\s]+,\s*\(char_u ?\*\)NULL,\s*PV_NONE,`)

// Whim54NoVar prints every options[] row that has no variable.
func Whim54NoVar(text []byte, w io.Writer) error {
	var names []string
	for _, m := range noVarRow.FindAllSubmatch(text, -1) {
		names = append(names, string(m[1]))
	}
	_, err := io.WriteString(w, strings.Join(names, " ")+"\n")
	return err
}

func init() { edit.RegisterQuery("whim54", Whim54NoVar) }
