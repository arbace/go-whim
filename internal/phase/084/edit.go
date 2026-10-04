package p084

// Whim phase 84 (formerly 161) -- no goto jumps into a block.  See GOAL.md.
//
// ml_get_buf()'s goto errorret jumped back into an earlier if block, which
// Go's goto may not.  The block's tail becomes ml_get_invalid().
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim84", Edit) }

const (
	W84Tail = `    errorret:
        musl_strcpy((char *)(questions), (char *)("???"));
        buf->b_ml.ml_line_len = 4;
        buf->b_ml.ml_line_textlen = buf->b_ml.ml_line_len;
        buf->b_ml.ml_line_lnum = lnum;
        return questions;
`
	W84Func = `    static char_u *
ml_get_invalid(buf_T *buf, linenr_T lnum)
{
    static char_u questions[4];

    musl_strcpy((char *)(questions), (char *)("???"));
    buf->b_ml.ml_line_len = 4;
    buf->b_ml.ml_line_textlen = buf->b_ml.ml_line_len;
    buf->b_ml.ml_line_lnum = lnum;
    return questions;
}

`
)

// Edit takes ml_get_buf()'s goto Out of a block.
//
// ml_get_buf() answers a line it cannot give with "???": the tail of its
// first if block, labelled errorret, which a goto further down jumps back
// into when the line is not found.  Go's goto may not jump into a block, and
// it is the one goto in the core that does (internal/ccx's Gotos).  The tail
// becomes ml_get_invalid(), with the static buffer it returns, and both
// paths return what it returns.  ml_get_buf()'s own buffer is then read by
// nothing, and the sweep takes it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the labelled tail and the jump
// replaced, and ml_get_invalid() put before ml_get_buf()'s definition, by
// FRAG in one unit (Together); history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("errorret", e, w)
	f := e.Defn("ml_get_buf")
	if f == nil {
		v.Die("ml_get_buf is not defined")
		return v.Done()
	}
	v.Together(func(v *graph.Verbs) {
		v.InFunction("ml_get_buf", func(v *graph.Verbs) {
			v.LiteralC("        ml_flush_line(buf);\n"+W84Tail, "        ml_flush_line(buf);\n        return ml_get_invalid(buf, lnum);\n", 1,
				"ml_get_buf() returns ml_get_invalid() for a line past the end")
			v.ReplaceC("(goto errorret)", "return ml_get_invalid(buf, lnum);\n", 1, "and for a line it cannot find, with no goto")
		})
		v.FragAt(e.SpotBefore(f), W84Func, "ml_get_invalid() is the tail the goto jumped into")
	})
	return v.Done()
}
