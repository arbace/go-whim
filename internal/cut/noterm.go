package cut

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoTerm makes the terminal what the build says it is.
//
// The environment stops choosing: no $TERM, no $LINES/$COLUMNS, no $COLORS, and
// the fallback becomes the only path -- so it has to name the 256-colour table
// rather than plain xterm, or the colours go with the lookup.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's regexps are forms --
// a DropIf, a Cut of the $TERM if and of ttest's bare $COLORS block, the
// fallback's literal RespellString, the probe's if FoldAlways -- and the
// leftover check the text's on the C view (history keeps the text version).
func NoTerm(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noterm", e, w)
	v.DropIf("(|| (== columns 0) (== rows 0) (!= (call vim_strchr p_cpo CPO_TSIZE) nullptr))", 1,
		"$LINES and $COLUMNS overriding the kernel")
	v.InFunction("termcapinit", func(v *graph.Verbs) {
		v.Cut(`(if (== term nullptr) (block (= term (cast _ (call getenv (cast _ (paren (cast _ "TERM"))))))))`, 1,
			"$TERM choosing the capability table")
		v.RespellString(`"xterm"`, `"xterm-256color"`, 1,
			"the fallback, which becomes the only path and must keep the colours")
	})
	v.InFunction("ttest", func(v *graph.Verbs) {
		v.Cut(`(block (= env_colors (cast _ (call getenv (cast _ (paren (cast _ "COLORS")))))) _)`, 1,
			"$COLORS overriding the table's colour count")
	})
	v.FoldAlways(`(== (cast _ (call getenv (cast _ (paren (cast _ "COLORS"))))) nullptr)`, 1,
		"$COLORS suppressing the 256-colour probe response")
	if v.Failed() {
		return v.Done()
	}
	// Only a getenv counts.  "TERM" is also a highlight-group key -- `:hi
	// term=bold` -- and the name of SIGTERM in the signal table, and a bare
	// substring test flags both.
	text := v.Text()
	for _, bad := range []string{"TERM", "LINES", "COLUMNS", "COLORS"} {
		if regexp.MustCompile(`getenv\([^)]*"` + bad + `"`).Match(text) {
			return fmt.Errorf("noterm: something still calls getenv(%q)", bad)
		}
	}
	v.Say("the terminal is xterm-256color by construction")
	return v.Done()
}
