package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// OneCmdFront makes a command line one command (the reform's front cut for
// record 81's change): no `|` separator and no `"` comment, so both are
// ordinary characters in a command's argument.  A newline still ends a
// command.  The edits are record 81's, on the seed's spelling: there the
// parsers also know vim9script's `#`, which goes with the rest.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): each text literal is the
// operand it kept, moved out of the condition (Rewrite), or the operands it
// left out dropped (DropOperand); the two bodies are FRAG (history keeps
// the text version).
func OneCmdFront(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("onecmdfront", e, w)
	v.InFunction("separate_nextcmd", func(v *graph.Verbs) {
		v.Rewrite(`(|| (paren (&& (== (deref p) '"') _*)) (paren (&& (== (deref p) '#') _*)) (paren (&& (== (deref p) '|') _*)) ?nl)`,
			"?nl", 1, "separate_nextcmd splits at a newline and nothing else")
		v.Rewrite("(|| (paren ?x) keep_backslash)", "?x", 1, "its one caller never keeps a backslash")
		v.FoldAlways("(! keep_backslash)", 1, "so a backslash before a newline always goes")
	})
	v.BodyC("ends_excmd", "return (c == NUL || c == '\\n');", "ends_excmd: the end of the line")
	v.BodyC("ends_excmd2", "int c = *cmd;\nreturn (c == NUL || c == '\\n');", "ends_excmd2: the same")
	v.InFunction("find_nextcmd", func(v *graph.Verbs) {
		v.DropOperand("(!= (deref p) '|')", 1, "find_nextcmd: the next line")
	})
	v.InFunction("check_nextcmd", func(v *graph.Verbs) {
		v.DropOperand("(== (deref s) '|')", 1, "check_nextcmd: the same")
	})
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		q := graph.NewVerbs("onecmdfront", e, io.Discard)
		q.In(v.Scope(), func(q *graph.Verbs) {
			q.DropOperand(`(!= (deref (. ea arg)) '"')`, 1, "a quote after a command")
			q.DropOperand("(|| (!= (deref (. ea arg)) '|') (== (& (. ea argt) EX_TRLBAR) 0))", 1, "a bar after a command")
		})
		if q.Err != nil {
			v.Die("%v", q.Err)
			return
		}
		v.Say("a bar or a quote after a command is trailing characters")
		v.DropOperand("(call comment_start (. ea cmd) starts_with_colon)", 1, "a line that is a comment is not empty")
	})
	v.InFunction("parse_command_modifiers", func(v *graph.Verbs) {
		v.DropIf("(call comment_start (-> eap cmd) starts_with_colon)", 1, "the modifier parser skips no comment")
		v.DropIf("(== (deref (-> eap cmd)) ':')", 1, "and records no colon")
	})
	v.InFunction("ex_range_without_command", func(v *graph.Verbs) {
		v.Rewrite("(paren (|| (== (deref (-> eap cmd)) '|') (paren ?x)))", "?x", 1, "`:|` no longer prints the line")
	})
	v.InFunction("ex_substitute", func(v *graph.Verbs) {
		v.DropOperand(`(!= (deref cmd) '"')`, 1, ":substitute takes no trailing comment")
	})
	v.InFunction("ex_append", func(v *graph.Verbs) {
		v.FoldNever("(== (deref (-> eap arg)) '|')", 1, ":append takes no text after a bar")
	})
	return v.Done()
}
