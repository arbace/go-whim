package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// The parser's half -- -t, -t's argument, -i -- is the command line's own,
// cut with it (argvfront, the reform's D1).

// NoCmdOpts removes -t, -i, -y and -Z, the fields they set, and restricted
// mode.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): each act by form, scoped where
// a name means more than one thing -- `tagname` is also a member of
// taggy_T, and the text scoped its field by the struct that closes as
// `} mparm_T;`; here the member is cut inside mparm_T's typedef (history
// keeps the text version).
func NoCmdOpts(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nocmdopts", e, w)
	v.Cut("(if (!= (. params tagname) nullptr) _)", 1,
		"the startup tag jump, which nothing can now ask for")
	v.DropOperand("(== (-> parmp tagname) nullptr)", 1, "exe_pre_commands testing for one")
	if td := v.One("(typedef mparm_T _)", "mparm_T"); td != nil {
		v.In(td, func(v *graph.Verbs) { v.Cut("(tagname (ptr char_u))", 1, "mparm_T's tagname field") })
	}
	// The flag itself was on twenty-four rows, every one deleted at phase 1
	// (extable, the reform's D2b): with the gate gone it is named by nothing.
	// The other way in, and a small find of its own: set_init_restricted_mode()
	// reads $SHELL at startup and turns the mode on when it is nologin or
	// false.  An environment read, deciding a mode that now restricts nothing.
	v.Cut("(call set_init_restricted_mode)", 1, "$SHELL deciding restricted mode at startup")
	// do_bang's restricted check went with :! and the commands that named a
	// file, at phase 1 (exfront and filefront, the reform's D2 and D4)
	v.InFunction("ex_stop", func(v *graph.Verbs) {
		v.Cut("(if (call check_restricted) (block (return)))", 1, "ex_stop's restricted check")
	})
	v.Cut("(if (&& (!= restricted 0) (paren (& (. ea argt) EX_RESTRICT))) _)", 1,
		"the EX_RESTRICT gate, which no live command reaches")
	if s := v.One("(= ignore_sigtstp (|| restricted _))", "ignore_sigtstp's store"); s != nil {
		v.In(s, func(v *graph.Verbs) {
			v.DropOperand("restricted", 1, "restricted deciding whether SIGTSTP is ignored")
		})
	}
	return v.Done()
}
