package cut

import (
	"fmt"
	"io"
	"regexp"
)

// noinertoptsEdits are the readers of six globals that are about to lose their
// option rows.
//
// THE THIRD ONE SEGFAULTED.  Every other reader of these six is an ADDRESS
// comparison -- `var == &p_path` -- which survives the global going away
// without noticing.  That one DEREFERENCES p_tc, at startup, in a function
// that runs before anything else, so the editor died before its first
// keystroke and the harness reported it as all 67 behaviour cases, the
// terminal table and every Ex command moving at once.
//
// The address comparisons are harmless at run time -- a test against a
// variable nobody can name is simply false -- but they keep the globals alive,
// and a global that is alive is one the phase's own check cannot prove unread.
var noinertoptsEdits = []struct{ what, pat, repl string }{
	// ex_drop's save and restore of 'autoread' died with :drop, retired at
	// phase 1 (exfront, the reform's D2)
	{`` + "`:setlocal autoread` meaning \"follow the global\"",
		`(?m)[ \t]*if \(\(int \*\)varp == &curbuf->b_p_ar && opt_flags == OPT_LOCAL\)\n` +
			`[ \t]*\{\n[ \t]*value = -1;\n[ \t]*\}\n[ \t]*else if`,
		"        if"},
	{"option_expand escaping for 'path' and 'tags'",
		`(?m)[ \t]*int esc = var == &p_tags \|\| var == &p_path;\n`,
		"    int esc = FALSE;\n"},
	// The directory-completion block, with its backslash rule for 'path',
	// folds at phase 1 (whim56, the reform's D6).  The file-completion
	// block's rule for 'tags' went with set_context_in_set_cmd(), :set's
	// completion, at phase 6 (whim59, phase 59's program, which runs before
	// this phase now).
	{"didset_string_options reading 'tagcase' at startup",
		`(?m)[ \t]*\(void\)opt_strings_flags\(p_tc, p_tc_values, &tc_flags, FALSE\);\n`, ""},
	// ml_open's swap-file test read p_uc, which nothing writes once
	// 'updatecount''s row is dropped: the fall-out closure took it at phase
	// 1 (optfront, the reform's D3)
}

// NoInertOpts removes the last readers of six globals whose option rows go
// with them.
func NoInertOpts(text []byte, w io.Writer) ([]byte, error) {
	for _, e := range noinertoptsEdits {
		re := regexp.MustCompile(e.pat)
		n := len(re.FindAll(text, -1))
		if n != 1 {
			return nil, fmt.Errorf("noinertopts: %s -- expected 1, matched %d", e.what, n)
		}
		text = re.ReplaceAllLiteral(text, []byte(e.repl))
		fmt.Fprintf(w, "  noinertopts  %s\n", e.what)
	}
	return text, nil
}
