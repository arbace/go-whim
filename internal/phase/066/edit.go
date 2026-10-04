package p066

// Whim phase 66 (formerly 138) -- no parameter carries an eval value.  See GOAL.md.
//
// vim_regsub_both's expr, match_add's pos_list, cursor_pos_info's dict, the
// formatter's tvs and find_ex_command's Vim9 lookup and context are passed
// nullptr by every call.  They go, each test of them folds, and the sweep takes
// typval_T, lists, dicts, type_T, class_T and the rest of the eval values.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim66", Edit) }

// Edit takes out the parameters that carry an eval value.
//
// Four functions still take one, and every call passes nullptr: the
// substitute string's expression (vim_regsub_both's typval_T *expr, for
// substitute() with a funcref), matchaddpos()'s list of positions
// (match_add's list_T *pos_list), wordcount()'s dictionary
// (cursor_pos_info's dict_T *dict), and printf()'s argument list (the
// formatter's typval_T *tvs, in the host).  Each parameter goes with its
// nullptr, and each test of it becomes what it always was.  So do
// find_ex_command()'s Vim9 lookup and compile context, which it never read,
// and with them the builtin function types cfunc_T and cfunc_free_T, which
// the collection takes.  They are the last
// things naming typval_T, list_T and dict_T outside their own definitions, so
// the collection takes the eval layer's value types with them.
// layer's value types with them.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the tests folded first, then
// each parameter dropped with its argument (PARAM) -- the formatter's and
// the parse's it hands tvs to as one edit; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("evalparm", e, w)
	v.InFunction("vim_regsub_both", func(v *graph.Verbs) {
		v.Rewrite("(|| (paren (&& ?s (== expr nullptr))) ?d)", "(|| ?s ?d)", 1,
			"a NULL source is refused whatever the expression was")
		v.Rewrite("(|| (!= expr nullptr) (paren ?c))", "?c", 1, "and only a \\= source is an expression")
	})
	v.InFunction("cursor_pos_info", func(v *graph.Verbs) {
		v.FoldAlways("(== dict nullptr)", 3, "cursor_pos_info() always gives its message")
	})
	v.InFunction("vim_vsnprintf_typval", func(v *graph.Verbs) {
		v.FoldNever("(!= tvs nullptr)", 2, "and the formatter always clamps an overlong width or precision")
		v.FoldNever("(&& (!= tvs nullptr) _)", 1, "and never counts arguments left over in a list")
	})
	// no number in a format is read from a list: 10 arguments
	var tests []*graph.Node
	for _, fn := range []string{"parse_fmt_types", "vim_vsnprintf_typval"} {
		v.InFunction(fn, func(v *graph.Verbs) {
			for _, n := range v.Find("(!= tvs nullptr)") {
				if p := e.Parent(n); p != nil && p.Is("call") {
					tests = append(tests, n)
				}
			}
		})
	}
	if v.Failed() {
		return v.Done()
	}
	if len(tests) != 10 {
		v.Die("no number in a format is read from a list -- %d tests of tvs as arguments, expected 10", len(tests))
		return v.Done()
	}
	for _, n := range tests {
		p := e.Parent(n)
		f, err := e.Build(n, "FALSE", nil)
		if err == nil {
			err = e.Replace(n, f...)
		}
		if err != nil {
			v.Die("no number in a format is read from a list -- %v", err)
			return v.Done()
		}
		e.Rederive(p) // FALSE is an int's enumerator: the call's type again
	}
	v.Say("and no number in a format is read from a list")
	v.DropParam("vim_regsub_both", "expr", "vim_regsub_both() takes no expression: its prototype, its definition and its one call")
	v.DropParam("match_add", "pos_list", "match_add() takes no list of positions, which it never read, nor its one call")
	v.DropParam("cursor_pos_info", "dict", "cursor_pos_info() fills no dictionary: its prototype, its definition and its one call")
	// the formatter and the parse it hands tvs to: one edit
	if !v.Failed() {
		v.DropParams([]graph.ParamDrop{
			{Decl: e.FileDecls("vim_vsnprintf_typval")[0], I: e.ParamIndex("vim_vsnprintf_typval", "tvs")},
			{Decl: e.FileDecls("parse_fmt_types")[0], I: e.ParamIndex("parse_fmt_types", "tvs")},
		}, graph.ParamOptions{}, "the formatter takes no argument list, and parse_fmt_types() takes none either")
	}
	if !v.Failed() {
		v.DropParams([]graph.ParamDrop{
			{Decl: e.FileDecls("find_ex_command")[0], I: e.ParamIndex("find_ex_command", "lookup")},
			{Decl: e.FileDecls("find_ex_command")[0], I: e.ParamIndex("find_ex_command", "cctx")},
		}, graph.ParamOptions{}, "find_ex_command() takes no Vim9 lookup or context, which it never read")
	}
	if !v.Failed() {
		n := v.Mentions("tvs")
		v.Expect(n == 0, "tvs has %d mentions left", n)
	}
	return v.Done()
}
