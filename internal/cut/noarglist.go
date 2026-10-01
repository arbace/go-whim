package cut

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	// The Python writes this as one pattern with a NEGATIVE LOOKAHEAD,
	// `^(?![ \t]*\[?CMD_)...`, which RE2 cannot spell.  What it means is "a
	// line naming one of these and not itself a table row", which is two
	// tests over the lines.
	arglistNamed = regexp.MustCompile(`\bCMD_(argdo|snext|argdelete)\b`)
	arglistRow   = regexp.MustCompile(`^[ \t]*\[?CMD_`)
)

// NoArgList takes the argument-list commands out of completion: the commands
// themselves are retired at phase 1 (exfront, the reform's D2).
func NoArgList(text []byte, w io.Writer) ([]byte, error) {
	e := ed{"noarglist", w}
	var err error

	// :next asking whether it was :snext, do_argfile sparing :argdo, and
	// :argdo's walk in ex_listdo died with those commands, retired at phase 1
	// (exfront, the reform's D2): what is left is completion.

	text, err = e.inFunction(text, "set_context_by_cmdname", func(seg []byte) ([]byte, error) {
		seg, err := e.subOnce(seg, `^[ \t]*case CMD_argdo:\n`, "completion for :argdo")
		if err != nil {
			return nil, err
		}
		return e.subOnce(seg, `^[ \t]*case CMD_argdelete:\n`+
			`[ \t]*while \(\(xp->xp_pattern = vim_strchr\(arg, ' '\)\) != NULL\)\n`+
			`[ \t]*\{\n`+
			`[ \t]*arg = xp->xp_pattern \+ 1;\n`+
			`[ \t]*\}\n`+
			`[ \t]*xp->xp_context = EXPAND_ARGLIST;\n[ \t]*xp->xp_pattern = arg;\n[ \t]*break;\n`, "completion for :argdelete")
	})
	if err != nil {
		return nil, err
	}

	text, err = e.literal(text, "        {EXPAND_ARGLIST, get_arglist_name, TRUE, FALSE},\n", "",
		"the argument-list expansion", 1)
	if err != nil {
		return nil, err
	}

	if left := linesMatchingUnless(text, arglistNamed, arglistRow); len(left) > 0 {
		for i, l := range left {
			left[i] = strings.TrimSpace(l)
		}
		return nil, fmt.Errorf("noarglist: still named outside the table: %s",
			strings.Join(left, "; "))
	}

	e.say("no argument-list command is completed")
	return text, nil
}
