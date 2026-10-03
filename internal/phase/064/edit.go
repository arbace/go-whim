package p064

// Whim phase 64 (formerly 136) -- the engine is called directly.  See GOAL.md.
//
// bt_regengine is the only regengine_T and every program's engine points at it,
// so the four calls through the table call known functions.  They name them,
// bt_regcomp() stops recording an engine, and the collection takes the table,
// the field and regengine_T (internal/gen/FINDINGS.md, 4).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1c): each of the text program's
// literals is the same statement found by its form and rebuilt from a
// template -- the callee named -- or the one assignment cut; history keeps
// the text version.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim64", Edit) }

// Edit calls the one regexp engine directly.
//
// With one engine, bt_regengine's four function pointers always hold
// bt_regcomp, bt_regfree, bt_regexec_nl and bt_regexec_multi, and every
// program's engine field always points at it -- bt_regcomp(), the only
// function that makes a program, sets it.  So each call through the table is
// a call to a function known here.  The four calls name their function, and
// bt_regcomp() stops recording an engine; the collection takes the table,
// the field and regengine_T (internal/gen/FINDINGS.md, 4).
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("engine", e, w)
	v.Rewrite("(= prog (call (. bt_regengine regcomp) expr re_flags))", "(= prog (call bt_regcomp expr re_flags))", 1,
		"vim_regcomp() calls bt_regcomp()")
	v.Rewrite("(call (-> prog engine regfree) prog)", "(call bt_regfree prog)", 1,
		"vim_regfree() calls bt_regfree()")
	v.Rewrite("(= result (call (-> rmp regprog engine regexec_nl) rmp line col nl))",
		"(= result (call bt_regexec_nl rmp line col nl))", 1,
		"vim_regexec_string() calls bt_regexec_nl()")
	v.Rewrite("(= result (call (-> rmp regprog engine regexec_multi) rmp win buf lnum col timed_out))",
		"(= result (call bt_regexec_multi rmp win buf lnum col timed_out))", 1,
		"vim_regexec_multi() calls bt_regexec_multi()")
	v.Cut("(= (-> r engine) (addr bt_regengine))", 1, "and a program records no engine")
	return v.Done()
}
