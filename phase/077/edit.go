package p077

// Whim phase 77 -- no buffer-name argument matching.  See GOAL.md.
//
// do_one_cmd() computes, at 20635,
//
// ni = (!(cmdidx < 0) && (cmd_func == ex_ni || cmd_func == ex_script_ni))
//
// -- "this command is not implemented" -- and SEVEN later checks consult it before
// doing work.  One does not: the EX_BUFNAME pre-dispatch block, guarded only by
// `!(cmdidx < 0)`, which compiles a regexp and matches it against the buffer to turn
// `:buffer foo` into a line number.
//
// Every command carrying EX_BUFNAME is ex_ni: :buffer, :bdelete, :bunload, :bwipeout,
// :checktime, :sbuffer.  So that block does real pattern-matching work for commands
// that cannot succeed, and its result is discarded when the handler errors.
//
// THE EDIT IS A FOLD, NOT A GUARD.  Adding `&& !ni` would leave a block that can
// still never run -- dead weight wearing a condition.  The condition is false for
// every command that reaches it, so fold_never removes it outright, and
// buflist_findpat loses its only caller.
//
// WHAT GOES BY CASCADE: buflist_findpat (71 lines), file_pat_to_reg_pat (167) and
// buflist_match (13) -- 251 lines whose whole purpose was naming a buffer by pattern.
// Nothing here deletes them by name; removing the one call site orphans them and the
// sweep takes them.
//
// THE BLOCK CONTAINS `goto doend;` AND THAT IS SAFE.  doend is do_one_cmd's shared
// exit label, targeted from many other places, so this removes a goto STATEMENT, not
// a label -- the distinction that mattered for readfile's `theend` in phase 75 and
// for close_buffer's `aucmd_abort`, where the label itself was inside the fold.
//
// THE DELTA: none expected.  `:buffer foo` already exits 1 with nothing on stderr --
// ex_ni sets eap->errmsg rather than printing, and an exsweep row is
// `exit= left= err=`.  Measured on q76: exit=1, stderr empty, file written.  So the
// gain here is code, not behaviour.  Declared empty, left for whimdelta.sh.

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

var (
	bufnameRows = regexp.MustCompile(`\[CMD_[a-zA-Z]+\] = \{\(char_u \*\)"([a-zA-Z]+)", [^,]+, *([a-z_]+)[^}]*EX_BUFNAME`)
	niComputed  = regexp.MustCompile(`(?m)^[ \t]*ni = \(!\(\(int\)\(ea\.cmdidx\) < 0\) && \(cmdnames\[ea\.cmdidx\]\.cmd_func == ex_ni`)
)

// Whim77 stops a command naming a buffer by pattern, having first proved that
// every command that could is already ex_ni.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nobufpat", text, w)

	rows := bufnameRows.FindAllSubmatch(text, -1)
	if len(rows) == 0 {
		e.Refuse("no command carries EX_BUFNAME -- the block this phase removes is already gone")
		return e.Done()
	}
	var live []string
	for _, r := range rows {
		fn := string(r[2])
		if fn != "ex_ni" && fn != "ex_script_ni" {
			live = append(live, string(r[1]))
		}
	}
	if len(live) > 0 {
		e.Refuse("these EX_BUFNAME commands have a LIVE handler and still need the pattern matching: %s",
			strings.Join(live, " "))
		return e.Done()
	}
	e.Say(fmt.Sprintf("confirmed: all %d EX_BUFNAME commands are ex_ni", len(rows)))

	if !niComputed.Match(text) {
		e.Refuse("`ni` is no longer computed as \"the handler is ex_ni\"")
		return e.Done()
	}
	e.FoldNeverIn2("do_one_cmd",
		`(?m)^[ \t]*if \(\(ea\.argt & EX_BUFNAME\) && \*ea\.arg != NUL && ea\.addr_count == 0 && !\(\(int\)\(ea\.cmdidx\) < 0\)\)$`,
		"naming a buffer by pattern for commands that cannot run", 1)
	return e.Done()
}

func init() { edit.Register("whim77", Edit) }
