package p174

// Whim phase 174 -- no address of a position's line or column.  See GOAL.md.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim174", Edit) }

// oneAdjust is vim's one_adjust() macro, as its preprocessor expanded it into
// mark_adjust_internal() -- a line number reached through lp, moved with the
// lines -- with the value a deleted line's mark gets as its first group: 0
// (one_adjust) or line1 (one_adjust_nodel).  The second group is the lvalue.
const oneAdjust = `(?m)^([ \t]*)\{\n` +
	`[ \t]*lp = &\((.+)\);\n` +
	`[ \t]*if \(\*lp >= line1 && \*lp <= line2\)\n` +
	`[ \t]*\{\n[ \t]*if \(amount == LONG_MAX\)\n[ \t]*\{\n` +
	`[ \t]*\*lp = %s;\n` +
	`[ \t]*\}\n[ \t]*else\n[ \t]*\{\n[ \t]*\*lp \+= amount;\n[ \t]*\}\n[ \t]*\}\n` +
	`[ \t]*else if \(amount_after && \*lp > line2\)\n[ \t]*\{\n[ \t]*\*lp \+= amount_after;\n[ \t]*\}\n` +
	`[ \t]*\}\n[ \t]*;\n`

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
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("posaddr", text, w)
	e.InFunction("mark_adjust_internal", func(e *edit.E) {
		e.Sub(fmt.Sprintf(oneAdjust, "0"), "${1}$2 = one_adjust($2, line1, line2, amount, amount_after);\n", 6,
			"mark_adjust_internal's six one_adjust() expansions are calls")
		e.Sub(fmt.Sprintf(oneAdjust, "line1"), "${1}$2 = one_adjust_nodel($2, line1, line2, amount, amount_after);\n", 7,
			"and its seven one_adjust_nodel() expansions")
		e.CountIs(`lp = &`, 0, "no line number is reached through lp")
	})
	e.Literal("    static void\nmark_adjust_internal(", adjusters+"    static void\nmark_adjust_internal(", 1,
		"one_adjust() and one_adjust_nodel(), before their caller")
	e.InFunction("cursor_pos_info", func(e *edit.E) {
		e.Sub(`(?m)^([ \t]*)getvcols\(curwin, &min_pos, &max_pos, &min_pos\.col, &max_pos\.col, 0\);\n`,
			"${1}{\n${1}    colnr_T min_col;\n${1}    colnr_T max_col;\n"+
				"${1}    getvcols(curwin, &min_pos, &max_pos, &min_col, &max_col, 0);\n"+
				"${1}    min_pos.col = min_col;\n${1}    max_pos.col = max_col;\n${1}}\n", 1,
			"cursor_pos_info's two columns come back through locals: getvcols() reads both positions before it writes either")
	})
	return e.Done()
}
