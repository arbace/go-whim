package cut

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// pyList renders a []string the way Python prints a list of str.
func pyList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = edit.PyRepr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

var preCommands = regexp.MustCompile(edit.Line("exe_pre_commands(&params);"))

// NoCmdArgs removes what -c, --cmd, -R, -m, -M and -w left: the options
// themselves went with the command line (argvfront, the reform's D1), and
// with --cmd went the only way to fill the commands startup ran before the
// vimrc.
func NoCmdArgs(text []byte, w io.Writer) ([]byte, error) {
	if n := len(preCommands.FindAll(text, -1)); n != 1 {
		return nil, fmt.Errorf("nocmdargs: startup running the --cmd commands -- matched "+
			"%d times, expected 1", n)
	}
	text = preCommands.ReplaceAll(text, nil)
	fmt.Fprintln(w, "  nocmdargs    startup running the --cmd commands")
	return text, nil
}
