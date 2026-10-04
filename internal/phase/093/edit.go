package p093

// Whim phase 93 (formerly 174) -- no address of a position's line or column.  See GOAL.md.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim93", Edit) }

// oneAdjust is vim's one_adjust() macro, as its preprocessor expanded it into
// mark_adjust_internal() -- a line number reached through lp, moved with the
// lines -- with the value a deleted line's mark gets bound to ?v: 0
// (one_adjust) or line1 (one_adjust_nodel), and the lvalue to ?x.  An empty
// statement follows each.
const oneAdjust = `(block
  (= lp (addr (paren ?x)))
  (if (&& (>= (deref lp) line1) (<= (deref lp) line2))
    (block (if (== amount LONG_MAX) (block (= (deref lp) ?v)) (block (+= (deref lp) amount))))
    (if (&& amount_after (> (deref lp) line2)) (block (+= (deref lp) amount_after)))))`

// the macro as two functions of the value
const adjusters = `    static linenr_T
one_adjust(linenr_T lnum, linenr_T line1, linenr_T line2, long amount, long amount_after)
{
    if (lnum >= line1 && lnum <= line2)
    {
        if (amount == LONG_MAX)
        {
            return 0;
        }
        return lnum + amount;
    }
    if (amount_after && lnum > line2)
    {
        return lnum + amount_after;
    }
    return lnum;
}

    static linenr_T
one_adjust_nodel(linenr_T lnum, linenr_T line1, linenr_T line2, long amount, long amount_after)
{
    if (lnum >= line1 && lnum <= line2)
    {
        if (amount == LONG_MAX)
        {
            return line1;
        }
        return lnum + amount;
    }
    if (amount_after && lnum > line2)
    {
        return lnum + amount_after;
    }
    return lnum;
}

`

// Edit takes the address of a position's line (pos_T.lnum) and column
// (pos_T.col) out of the two functions that take it, so that neither member
// has its address taken anywhere: a Java or Clojure editor boxes such a
// member in a one-element array, and every cursor read in the file pays.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): each expansion, the block and
// the empty statement after it, found by its form and replaced by its call,
// the lvalue a hole written twice; the two functions before their caller and
// cursor_pos_info()'s locals, all by FRAG in one unit; history keeps the text
// version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("posaddr", e, w)
	mai := e.Defn("mark_adjust_internal")
	if mai == nil {
		v.Die("mark_adjust_internal is not defined")
		return v.Done()
	}
	pat, err := clisp.Pattern(oneAdjust)
	if err != nil {
		return err
	}
	var frags []graph.Frag
	n := map[string]int{}
	v.In(mai, func(v *graph.Verbs) {
		for _, b := range v.Find(oneAdjust) {
			m, _ := graph.Match(pat, b)
			ks := e.Parent(b).Kids
			var next *graph.Node
			for i, x := range ks {
				if x == b && i+1 < len(ks) {
					next = ks[i+1]
				}
			}
			fn := map[string]string{"0": "one_adjust", "line1": "one_adjust_nodel"}[m["v"].Atom]
			if next == nil || !next.Is("empty") || fn == "" || m["v"].IsList() {
				v.Die("an expansion of one_adjust() is not what this phase expects")
				return
			}
			n[fn]++
			frags = append(frags, graph.Frag{At: e.SpotRun(b, next), Src: "$x = " + fn + "($x, line1, line2, amount, amount_after);\n", Holes: graph.Bindings{"x": m["x"]}})
		}
	})
	v.Expect(n["one_adjust"] == 6 && n["one_adjust_nodel"] == 7, "%d one_adjust() and %d one_adjust_nodel() expansions, where this phase was written against 6 and 7", n["one_adjust"], n["one_adjust_nodel"])
	var gv []*graph.Node
	v.InFunction("cursor_pos_info", func(v *graph.Verbs) {
		gv = v.Find("(call getvcols curwin (addr min_pos) (addr max_pos) (addr (. min_pos col)) (addr (. max_pos col)) 0)")
	})
	if len(gv) != 1 || e.Item(gv[0]) != gv[0] {
		v.Die("cursor_pos_info's getvcols() is not where this phase expects it")
	}
	if v.Failed() {
		return v.Done()
	}
	frags = append(frags, graph.Frag{At: e.SpotBefore(mai), Src: adjusters},
		graph.Frag{At: e.SpotOf(gv[0]), Src: "{\n    colnr_T min_col;\n    colnr_T max_col;\n    getvcols(curwin, &min_pos, &max_pos, &min_col, &max_col, 0);\n    min_pos.col = min_col;\n    max_pos.col = max_col;\n}\n"})
	if _, err := e.SpliceC(frags...); err != nil {
		v.Die("mark_adjust_internal's expansions are calls -- %v", err)
		return v.Done()
	}
	v.Say("mark_adjust_internal's six one_adjust() expansions are calls")
	v.Say("and its seven one_adjust_nodel() expansions")
	v.In(mai, func(v *graph.Verbs) {
		v.CountIs("(= lp _)", 0, "no line number is reached through lp")
	})
	v.Say("one_adjust() and one_adjust_nodel(), before their caller")
	v.Say("cursor_pos_info's two columns come back through locals: getvcols() reads both positions before it writes either")
	return v.Done()
}
