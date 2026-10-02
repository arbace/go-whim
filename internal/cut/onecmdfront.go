package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
)

// OneCmdFront makes a command line one command (the reform's front cut for
// record 81's change): no `|` separator and no `"` comment, so both are
// ordinary characters in a command's argument.  A newline still ends a
// command.  The edits are record 81's, on the seed's spelling: there the
// parsers also know vim9script's `#`, which goes with the rest.
func OneCmdFront(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("onecmdfront", text, w)
	e.InFunction("separate_nextcmd", func(e *edit.E) {
		e.Literal(`        else if ((*p == '"' && !vim9script && !(eap->argt & EX_NOTRLCOM) && ((eap->cmdidx != CMD_at && eap->cmdidx != CMD_star) || p != eap->arg) && (eap->cmdidx != CMD_redir || p != eap->arg + 1 || p[-1] != '@')) || (*p == '#' && vim9script && !(eap->argt & EX_NOTRLCOM) && p > eap->cmd && ((p[-1]) == ' ' || (p[-1]) == '\t')) || (*p == '|' && eap->cmdidx != CMD_append && eap->cmdidx != CMD_change && eap->cmdidx != CMD_insert) || *p == '\n')`,
			`        else if (*p == '\n')`, 1, "separate_nextcmd splits at a newline and nothing else")
		e.Literal("if ((eap->argt & (EX_CTRLV | EX_XFILE)) || keep_backslash)",
			"if (eap->argt & (EX_CTRLV | EX_XFILE))", 1, "its one caller never keeps a backslash")
		e.FoldAlways(edit.Head("if (!keep_backslash)"), 1, "so a backslash before a newline always goes")
	})
	e.Body("ends_excmd", "    return (c == NUL || c == '\\n');\n", "ends_excmd: the end of the line")
	e.Body("ends_excmd2", "    int c = *cmd;\n    return (c == NUL || c == '\\n');\n", "ends_excmd2: the same")
	e.InFunction("find_nextcmd", func(e *edit.E) {
		e.Literal("    while (*p != '|' && *p != '\\n')\n", "    while (*p != '\\n')\n", 1, "find_nextcmd: the next line")
	})
	e.InFunction("check_nextcmd", func(e *edit.E) {
		e.Literal("    if (*s == '|' || *s == '\\n')\n", "    if (*s == '\\n')\n", 1, "check_nextcmd: the same")
	})
	e.InFunction("do_one_cmd", func(e *edit.E) {
		e.Literal(` && *ea.arg != NUL && *ea.arg != '"' && (*ea.arg != '|' || (ea.argt & EX_TRLBAR) == 0))`,
			` && *ea.arg != NUL)`, 1, "a bar or a quote after a command is trailing characters")
		e.Literal("if (*ea.cmd == NUL || comment_start(ea.cmd, starts_with_colon) || (ea.nextcmd = check_nextcmd(ea.cmd)) != nullptr)",
			"if (*ea.cmd == NUL || (ea.nextcmd = check_nextcmd(ea.cmd)) != nullptr)", 1, "a line that is a comment is not empty")
	})
	e.InFunction("parse_command_modifiers", func(e *edit.E) {
		e.DropIf(edit.Head("if (comment_start(eap->cmd, starts_with_colon))"), 1, "the modifier parser skips no comment")
		e.DropIf(edit.Head("if (*eap->cmd == ':')"), 1, "and records no colon")
	})
	e.InFunction("ex_range_without_command", func(e *edit.E) {
		e.Literal("if ((*eap->cmd == '|' || (exmode_active && eap->cmd != (char_u *)exmode_plus + 1)))",
			"if (exmode_active && eap->cmd != (char_u *)exmode_plus + 1)", 1, "`:|` no longer prints the line")
	})
	e.InFunction("ex_substitute", func(e *edit.E) {
		e.Literal("    if (*cmd && *cmd != '\"')\n    {\n        set_nextcmd(eap, cmd);",
			"    if (*cmd)\n    {\n        set_nextcmd(eap, cmd);", 1, ":substitute takes no trailing comment")
	})
	e.InFunction("ex_append", func(e *edit.E) {
		e.FoldNever(edit.Head("if (*eap->arg == '|')"), 1, ":append takes no text after a bar")
	})
	return e.Done()
}
