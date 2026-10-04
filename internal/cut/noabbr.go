package cut

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

var abbrFolds = []struct{ what, cond string }{
	{"insert mode: ESC expanding an abbreviation first", "(call echeck_abbr (+ ESC ABBR_OFF))"},
	{"insert mode: CTRL-O expanding an abbreviation first", "(call echeck_abbr (+ Ctrl_O ABBR_OFF))"},
	{"insert mode: CTRL-L under 'insertmode'", "(call echeck_abbr (+ Ctrl_L ABBR_OFF))"},
	{"insert mode: Tab expanding an abbreviation first", "(call echeck_abbr (+ TAB ABBR_OFF))"},
	{"insert mode: Enter expanding an abbreviation first", "(call echeck_abbr (+ c ABBR_OFF))"},
	{"the command line: a special key expanding an abbreviation", "(call ccheck_abbr (+ c ABBR_OFF))"},
}

// has_mbyte is folded already: utf8only runs at phase 2 (the reform's D7).
var abbrRewrites = []struct{ what, pat, tmpl string }{
	{"insert mode: a non-word character inserting unless an abbreviation took it",
		"(paren (&& (! (call echeck_abbr (? (paren (>= c 0x100)) (paren (+ c ABBR_OFF)) c))) ?x))", "?x"},
	{"the command line: a non-word character expanding an abbreviation",
		"(|| (call ccheck_abbr (? (paren (>= c 0x100)) (paren (+ c ABBR_OFF)) c)) ?x)", "?x"},
}

// NoAbbr takes away every place an abbreviation could still be expanded.
//
// The last act is the one that matters: nothing may still ASK.  The two
// wrappers must have no caller at all and check_abbr() none outside them --
// which the collection takes, so a call left INSIDE one of them is not a
// caller.  On the graph (doc/GRAPH-MIGRATION.md, B4) that is asked of the
// edges: every use of the three outside their own definitions (history
// keeps the text version, which scanned the text outside their spans).
func NoAbbr(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noabbr", e, w)
	for _, f := range abbrFolds {
		v.FoldNever(f.cond, 1, f.what)
	}
	for _, r := range abbrRewrites {
		v.Rewrite(r.pat, r.tmpl, 1, r.what)
	}
	if v.Failed() {
		return v.Done()
	}
	names := map[string]bool{"echeck_abbr": true, "ccheck_abbr": true, "check_abbr": true}
	var still []string
	for _, n := range []string{"echeck_abbr", "ccheck_abbr", "check_abbr"} {
		for _, u := range v.UsesOf(n) {
			if f := e.Function(u); f == nil || !names[graph.DeclName(f)] {
				where := "file scope"
				if f != nil {
					where = graph.DeclName(f)
				}
				still = append(still, n+" in "+where)
			}
		}
	}
	if len(still) > 0 {
		return fmt.Errorf("noabbr: still asked at:\n    %s", strings.Join(still, "\n    "))
	}
	v.Say("nothing asks whether an abbreviation applies")
	return v.Done()
}
