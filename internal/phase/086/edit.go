package p086

// Whim phase 86 (formerly 165) -- no store nothing reads.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim86", Edit) }

// Edit removes six stores to locals that nothing reads before they are
// written again or go out of scope.  The Go transpilation showed them
// (staticcheck SA4006, SA4009); they are the C's as much as the Go's.  Each is
// named and found exactly once in its own function, because finding such a
// store in general is dataflow through gotos, and there are six.  A store
// whose right side calls something keeps the call: only the store goes.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): each store found by its form
// and the statements around it that the text's pattern named; the parameter
// made a local by PARAM's ParamToLocal (the same node, its uses unchanged);
// history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("deadstore", e, w)
	is := func(n *graph.Node, pat string) bool {
		p, err := clisp.Pattern(pat)
		return err == nil && n != nil && graph.Matches(p, n)
	}
	sibling := func(n *graph.Node, k int) *graph.Node {
		ks := e.Parent(n).Kids
		for i, x := range ks {
			if x == n && i+k >= 0 && i+k < len(ks) {
				return ks[i+k]
			}
		}
		return nil
	}
	// cut deletes the one item pat matches for which ok holds
	cut := func(pat string, ok func(*graph.Node) bool, what string) {
		var ms []*graph.Node
		for _, m := range v.Find(pat) {
			if e.Item(m) == m && ok(m) {
				ms = append(ms, m)
			}
		}
		if len(ms) != 1 {
			v.Die("%s -- matched %d times, expected 1", what, len(ms))
			return
		}
		if err := e.Delete(ms[0]); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
		v.Say(what)
	}
	// the line's text, fetched after an indent computed from the same line,
	// and never looked at before ptr is fetched again
	v.InFunction("open_line", func(v *graph.Verbs) {
		cut("(= ptr (call ml_get_curline))", func(m *graph.Node) bool { return is(sibling(m, -1), "(= newindent (call get_indent))") },
			"open_line's ptr, fetched and never read")
	})
	// showmode() draws the mode; what it returns was kept and never read
	v.InFunction("edit", func(v *graph.Verbs) {
		v.Rewrite("(= i (call showmode))", "(call showmode)", 1, "edit()'s i = showmode(): the call stays, the store goes")
	})
	// the parameter is the key read here, never the caller's: a local
	v.ParamToLocal("cmdline_handle_ctrl_bsl", "c",
		"cmdline_handle_ctrl_bsl's c, a parameter overwritten before any read, is a local, and its one caller stops passing it")
	// one past a match at the end of the line, just before the loop is left
	v.InFunction("next_search_hl", func(v *graph.Verbs) {
		cut("(pre++ matchcol)", func(m *graph.Node) bool {
			return is(sibling(m, 1), "(= (-> shl lnum) 0)") && is(sibling(m, 2), "(break)")
		}, "next_search_hl's ++matchcol before the loop is left")
	})
	// the column within the row, which nothing after asks for
	v.InFunction("adjust_skipcol", func(v *graph.Verbs) {
		v.Cut("(= col (% col width2))", 1, "adjust_skipcol's col % width2")
	})
	// the line after the put, stepped back once the loop is done with it
	v.InFunction("do_put", func(v *graph.Verbs) {
		cut("(if VIsual_active (block (post-- lnum)))", func(m *graph.Node) bool {
			return is(sibling(m, -1), "(do _ (&& VIsual_active (<= lnum end_lnum)))")
		}, "do_put's lnum-- after the last use of lnum")
	})
	return v.Done()
}
