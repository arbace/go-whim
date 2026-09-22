package edit

import (
	"io"
)

func init() { register("whim136", Whim136) }

// Whim136 calls the one regexp engine directly.
//
// With one engine, bt_regengine's four function pointers always hold
// bt_regcomp, bt_regfree, bt_regexec_nl and bt_regexec_multi, and every
// program's engine field always points at it -- bt_regcomp(), the only
// function that makes a program, sets it.  So each call through the table is
// a call to a function known here.  The four calls name their function, and
// bt_regcomp() stops recording an engine; the sweep takes the table, the field
// and regengine_T (tx/FINDINGS.md, 4).
func Whim136(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "engine", w: w}
	var err error
	steps := []struct{ old, new, what string }{
		{"prog = bt_regengine.regcomp(expr, re_flags);", "prog = bt_regcomp(expr, re_flags);", "vim_regcomp() calls bt_regcomp()"},
		{"prog->engine->regfree(prog);", "bt_regfree(prog);", "vim_regfree() calls bt_regfree()"},
		{"rmp->regprog->engine->regexec_nl(rmp, line, col, nl);", "bt_regexec_nl(rmp, line, col, nl);", "vim_regexec_string() calls bt_regexec_nl()"},
		{"rmp->regprog->engine->regexec_multi(rmp, win, buf, lnum, col, timed_out);", "bt_regexec_multi(rmp, win, buf, lnum, col, timed_out);", "vim_regexec_multi() calls bt_regexec_multi()"},
		{"    r->engine = &bt_regengine;\n", "", "and a program records no engine"},
	}
	for _, s := range steps {
		if text, err = p.literal(text, s.old, s.new, s.what, 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
