package p161

// Whim phase 161 -- no goto jumps into a block.  See GOAL.md.
//
// ml_get_buf()'s goto errorret jumped back into an earlier if block, which
// Go's goto may not.  The block's tail becomes ml_get_invalid().
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim161", Edit) }

// W161Tail is the error path ml_get_buf()'s goto jumped back into, and
// W161Func the function it becomes.
const (
	W161Tail = `errorret:
         musl_strcpy((char *)(questions), (char *)("???")) ;
        buf->b_ml.ml_line_len = 4;
        buf->b_ml.ml_line_textlen = buf->b_ml.ml_line_len;
        buf->b_ml.ml_line_lnum = lnum;
        return questions;
`
	W161Func = `    static char_u *
ml_get_invalid(buf_T *buf, linenr_T lnum)
{
    static char_u questions[4];

     musl_strcpy((char *)(questions), (char *)("???")) ;
    buf->b_ml.ml_line_len = 4;
    buf->b_ml.ml_line_textlen = buf->b_ml.ml_line_len;
    buf->b_ml.ml_line_lnum = lnum;
    return questions;
}

`
)

// Whim161 takes ml_get_buf()'s goto Out of a block.
//
// ml_get_buf() answers a line it cannot give with "???": the tail of its
// first if block, labelled errorret, which a goto further down jumps back
// into when the line is not found.  Go's goto may not jump into a block, and
// it is the one goto in the core that does (internal/ccx's Gotos).  The tail
// becomes ml_get_invalid(), with the static buffer it returns, and both
// paths return what it returns.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "errorret", W: w}
	var err error
	steps := []struct{ Old, New, What string }{
		{"        ml_flush_line(buf);\n" + W161Tail, "        ml_flush_line(buf);\n        return ml_get_invalid(buf, lnum);\n", "ml_get_buf() returns ml_get_invalid() for a line past the end"},
		{"            goto errorret;\n", "            return ml_get_invalid(buf, lnum);\n", "and for a line it cannot find, with no goto"},
		{"    static int  recursive = 0;\n    static char_u questions[4];\n\n    if (lnum > buf->b_ml.ml_line_count)\n", "    static int  recursive = 0;\n\n    if (lnum > buf->b_ml.ml_line_count)\n", "the \"???\" it returns is ml_get_invalid()'s"},
		{"    static char_u  *\nml_get_buf(buf_T       *buf,", W161Func + "    static char_u  *\nml_get_buf(buf_T       *buf,", "ml_get_invalid() is the tail the goto jumped into"},
	}
	for _, st := range steps {
		if text, err = p.Literal(text, st.Old, st.New, st.What, 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
