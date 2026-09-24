package p136

// Whim phase 136, the check -- the engine is called directly.
// See phase/136/edit.go, and GOALS.md.
//
// phase/136/check.go proves every program's engine was bt_regengine,
// requires the table, the type and the field gone, and probes a search and two
// substitutions against a control that matches nothing.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim136", Check) }

// Whim136 is phase 136's check: the engine is called directly.
//
//  1. THE PREMISE, on the input: bt_regengine is initialised with exactly
//     bt_regcomp, bt_regfree, bt_regexec_nl, bt_regexec_multi, in the order of
//     regengine_T's four fields; `->engine =` is assigned once, to
//     &bt_regengine, in bt_regcomp(); and only bt_regcomp() allocates a program.
//     So every call through a program's engine called these four.
//  2. THE CUT: no mention of bt_regengine, regengine_T or an engine field is
//     left -- the sweep took the table, the type and the field -- and the four
//     functions are each called directly.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBE: a search, a single-line substitution with a back-reference and
//     a multi-line one write the same bytes on both binaries; the CONTROL, the
//     same keys with a pattern that matches nothing, writes different ones.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim136", "engine")
	if err != nil {
		return err
	}
	r := c.R
	// The canonical text writes an initialiser one element per line with a
	// trailing comma after the LAST one, so the `,?` is what the printer put
	// there and not a loosening: with or without it this is one site.
	init := regexp.MustCompile(`static regengine_T bt_regengine =\s*\{\s*bt_regcomp,\s*bt_regfree,\s*bt_regexec_nl,\s*bt_regexec_multi,?\s*\};`)
	if !init.MatchString(c.Old) {
		r.Bad("bt_regengine is not initialised with the four backtracking functions in order")
	}
	if !strings.Contains(c.Old, "regprog_T *(*regcomp)(char_u *, int);\n    void (*regfree)(regprog_T *);\n    int (*regexec_nl)(") {
		r.Bad("regengine_T's fields are not regcomp, regfree, regexec_nl, regexec_multi in that order")
	}
	if as := regexp.MustCompile(`->engine\s*=[^=]`).FindAllStringIndex(c.Old, -1); len(as) != 1 || !strings.Contains(c.Old, "r->engine = &bt_regengine;") {
		r.Bad("->engine is assigned %d times on the input, where once, to &bt_regengine, is what this phase depends on", len(as))
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("on the input every program's engine is bt_regengine, whose four functions are the backtracking ones: each call through it called a known function")
	for _, n := range []string{"bt_regengine", "regengine_T", "engine"} {
		if k := check.Word(c.New, n); k != 0 {
			r.Bad("%s still has %d mentions", n, k)
		}
	}
	for _, call := range []string{"prog = bt_regcomp(expr, re_flags);", "bt_regfree(prog);", "bt_regexec_nl(rmp, line, col, nl);", "bt_regexec_multi(rmp, win, buf, lnum, col, timed_out);"} {
		if strings.Count(c.New, call) != 1 {
			r.Bad("%q is not called directly, once", call)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the four functions are called directly; the table, regengine_T and the engine field are gone (the file lost %d lines)",
		check.CountLines([]byte(c.Old))-check.CountLines([]byte(c.New)))
	if err := c.Gate(true); err != nil {
		return err
	}
	old, nw := c.Bins()
	seed := []byte("ione two\rthree four\rtwo again\x1bgg")
	// each probe, and its control: the same command on a pattern that matches
	// nothing in the text
	probes := [][2]string{
		{"/tw\r", "/Qw\r"},
		{":s/\\(o\\)\\(.\\)/\\2\\1/g\r", ":s/\\(Q\\)\\(.\\)/\\2\\1/g\r"},
		{":%s/t\\(w\\|h\\)/T\\1/g\r", ":%s/Q\\(w\\|h\\)/T\\1/g\r"},
	}
	for _, pr := range probes {
		u := pr[0]
		keys := [][]byte{seed, []byte(u), []byte(":q!\r")}
		so, _, e1 := check.Stream(old, keys, nil)
		sn, _, e2 := check.Stream(nw, keys, nil)
		ctl := [][]byte{seed, []byte(pr[1]), []byte(":q!\r")}
		sc, _, e3 := check.Stream(nw, ctl, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.Say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if so != sn {
			r.Bad("%q is written differently on the two binaries", u)
		}
		if sc == sn {
			r.Bad("the CONTROL for %q did not move", u)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: a search, a single-line substitution with back-references and a multi-line one write the same bytes on both binaries; each CONTROL, a pattern matching nothing, writes different ones")
	return nil
}
