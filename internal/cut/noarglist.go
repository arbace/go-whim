package cut

import (
	"io"
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

	// Not asserted that nothing outside the table names an argument-list
	// command: this runs at phase 1 (the reform's D9), where the retired
	// commands' handlers and what phases 2-37 took still do.

	e.say("no argument-list command is completed")
	return text, nil
}
