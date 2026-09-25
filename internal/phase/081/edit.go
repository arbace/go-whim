package p081

// Whim phase 81 -- one line, one command.  See GOAL.md.
//
// An Ex line could hold several commands separated by `|`, and end in a `"`
// comment.  Both existed for scripts -- a vimrc, a sourced file, a function body --
// and this editor reads none: every command it runs was typed, came from `+cmd`, or
// came from a mapping's right-hand side.  So a line is one command now, and `|` and
// `"` are ordinary characters in its argument.
//
// THE MACHINERY IS SMALL AND IN ONE PLACE.  separate_nextcmd() split a bar-splitting
// (EX_TRLBAR) command's argument at the first unescaped `|`, `"` or newline;
// check_nextcmd(), find_nextcmd(), ends_excmd() and ends_excmd2() each knew the same
// three characters; and a handful of callers knew them again -- :substitute's tail,
// the trailing-characters check in do_one_cmd, :append's `:a|text`, `:|` printing the
// line, and the whole-line `:" comment`.
//
// A NEWLINE STILL ENDS A COMMAND.  "One line, one command" is exactly that rule, and
// the newline branch of separate_nextcmd is kept as it was, backslash and all.
//
// DECIDED BEFORE THIS WAS WRITTEN, and each is probed below:
// `a|b`      the bar is argument text.  A command without EX_EXTRA reports E488
// trailing characters; one with it takes the bar as part of its argument.
// `a " x`    the quote is argument text too, so a trailing comment is an error or
// an argument, and `:" x` is E492.
// `\|`       means nothing special: the backslash stays, so `:map Q A\|b` maps to
// `A\|b` where it used to map to `A|b`.  `\"` likewise.  CTRL-V handling
// is unchanged.
//
// KEPT, deliberately: EX_NOTRLCOM.  Its comment meaning is gone, but it still decides
// whether separate_nextcmd strips trailing spaces, and that is what lets a mapping's
// right-hand side end in a space.  Behaviour, not comment syntax, so it stays.
//
// THE DELTA.  No behaviour case, no terminal row and no swept command uses a bar or a
// comment, so the cumulative list is phase 80's unchanged.  What moves is probed
// directly: a corpus of lines through the q80 binary and this one, with the lines
// expected to differ written out, and everything else required identical.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

var (
	nextcmdCalls = regexp.MustCompile(`\bseparate_nextcmd\(([^;]*)\);`)
	cmdArgtRow   = func(c string) *regexp.Regexp {
		return regexp.MustCompile(`(?m)^    \[CMD_` + c + `\] = \{.*\(long_u\)\(([^)]*)\)`)
	}
)

// Whim81 makes a command line one command: no bar separator and no trailing
// comment, so `|` and `"` stop being syntax.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("onecommand", text, w)

	var calls []string
	for _, m := range nextcmdCalls.FindAllSubmatch(text, -1) {
		c := string(m[1])
		if !bytes.HasPrefix([]byte(c), []byte("exarg_T")) {
			calls = append(calls, c)
		}
	}
	if len(calls) != 1 || calls[0] != "&ea, FALSE" {
		e.Refuse("separate_nextcmd is called as %s, expected once with FALSE", cutil.PyRepr(fmt.Sprint(calls)))
		return e.Done()
	}
	for _, c := range []string{"append", "insert", "change"} {
		m := cmdArgtRow(c).FindSubmatch(text)
		if m == nil || bytes.Contains(m[1], []byte("EX_EXTRA")) {
			e.Refuse(":%s is not a command without EX_EXTRA", c)
			return e.Done()
		}
	}
	e.Say("confirmed: one caller of separate_nextcmd, and :append, :insert, :change take no argument")

	e.Term("        else if ((*p == '\"' && !(eap->argt & EX_NOTRLCOM) && ((eap->cmdidx != CMD_at && eap->cmdidx != CMD_star) || p != eap->arg)) || (*p == '|' && eap->cmdidx != CMD_append && eap->cmdidx != CMD_change && eap->cmdidx != CMD_insert) || *p == '\\n')",
		"        else if (*p == '\\n')", 1, "separate_nextcmd splits at a newline and nothing else")
	e.Term("if ((eap->argt & (EX_CTRLV | EX_XFILE)) || keep_backslash)",
		"if (eap->argt & (EX_CTRLV | EX_XFILE))", 1, "its one caller never keeps a backslash")
	e.FoldAlways(`(?m)^[ \t]*if \(!keep_backslash\)$`, "so a backslash before a newline always goes")
	e.Term(w81lit1, "    return (c == NUL || c == '\\n');", 1, "ends_excmd: the end of the line")
	e.Term(w81lit2, "    return (c == NUL || c == '\\n');", 1, "ends_excmd2: the same")
	e.Term(w81lit3, w81lit4, 1, "find_nextcmd: the next line")
	e.Term(w81lit5, w81lit6, 1, "check_nextcmd: the same")
	e.Term("if (!(ea.argt & EX_EXTRA) && *ea.arg != NUL && *ea.arg != '\"' && (*ea.arg != '|' || (ea.argt & EX_TRLBAR) == 0))",
		"if (!(ea.argt & EX_EXTRA) && *ea.arg != NUL)", 1, "a bar or a quote after a command is trailing characters")
	e.Term("if (*ea.cmd == NUL || comment_start(ea.cmd, starts_with_colon) || (ea.nextcmd = check_nextcmd(ea.cmd)) != NULL)",
		"if (*ea.cmd == NUL || (ea.nextcmd = check_nextcmd(ea.cmd)) != NULL)", 1, "a line that is a comment is not empty")
	e.DropIf(`(?m)^[ \t]*if \(comment_start\(eap->cmd, starts_with_colon\)\)$`, "the modifier parser skips no comment")
	e.DropIf(`(?m)^[ \t]*if \(\*eap->cmd == ':'\)$`, "and records no colon")
	e.Term("if ((*eap->cmd == '|' || (exmode_active && eap->cmd != (char_u *)exmode_plus + 1)))",
		"if (exmode_active && eap->cmd != (char_u *)exmode_plus + 1)", 1, "`:|` no longer prints the line")
	e.Term(w81lit7, w81lit8, 1, ":substitute takes no trailing comment")
	e.FoldNever(`(?m)^[ \t]*if \(\*eap->arg == '\|'\)$`, ":append takes no text after a bar")

	// The closing assertion: the six parsers must no longer mention either
	// character at all, and comment_start must have lost its last caller.
	for _, p := range []struct{ Pat, What string }{{`'\|'`, "a bar"}, {`'"'`, "a quote"}} {
		for _, fn := range []string{"separate_nextcmd", "ends_excmd", "ends_excmd2", "find_nextcmd", "check_nextcmd", "do_one_cmd"} {
			if e.Failed() {
				return e.Done()
			}
			Body, ok := e.BodyOf(fn)
			if !ok {
				e.Refuse("%s is not defined", fn)
				return e.Done()
			}
			if regexp.MustCompile(p.Pat).Match(Body) {
				e.Refuse("%s still tests for %s", fn, p.What)
				return e.Done()
			}
		}
	}
	if !e.Failed() {
		stripped := bytes.ReplaceAll(e.Text(), []byte("comment_start(char_u"), nil)
		if bytes.Contains(stripped, []byte("comment_start(")) {
			e.Refuse("comment_start still has a caller")
			return e.Done()
		}
		e.Say("no command parser tests for a bar or a quote")
	}
	return e.Done()
}

func init() { edit.Register("whim81", Edit) }
