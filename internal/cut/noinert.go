package cut

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

var noinertLeft = regexp.MustCompile(`"(browse|confirm)", \d, TRUE|modec == 't'|CMD_behave:`)

// NoInert removes the modifiers, mapping modes and argument lists that do
// nothing in this build: :browse, :confirm, the terminal-job mapping mode and
// :behave's argument list.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's line cuts are a
// DropIf, a case's run dropped, an else-if arm folded never, an if cut, case
// labels and a case's run cut, and a row deleted from ExpandOther's table;
// the leftover check is the text's own regexp on the C view (history keeps
// the text version).
func NoInert(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noinert", e, w)
	v.InFunction("parse_command_modifiers", func(v *graph.Verbs) {
		v.DropIf(`(call checkforcmd_opt (addr (-> eap cmd)) "browse" 3 TRUE)`, 1, "the :browse modifier")
		v.CutRun("the :confirm modifier", "(case 'c')",
			`(if (! (call checkforcmd_opt (addr (-> eap cmd)) "confirm" 4 TRUE)) (block (break)))`,
			"(continue)")
	})
	v.InFunction("get_map_mode", func(v *graph.Verbs) {
		v.FoldNever("(== modec 't')", 1, "get_map_mode's 't'")
	})
	v.InFunction("map_mode_to_chars", func(v *graph.Verbs) {
		v.Cut("(if (& mode MODE_TERMINAL) (block (call ga_append (addr mapmode) 't')))", 1,
			"map_mode_to_chars printing 't'")
	})
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		for _, name := range []string{"browse", "confirm", "tmap", "tnoremap", "tunmap", "tmapclear"} {
			v.Cut("(case CMD_"+name+")", 1, "completion for :"+name)
		}
		v.CutRun("completion for :behave", "(case CMD_behave)",
			"(= (-> xp xp_context) EXPAND_BEHAVE)", "(= (-> xp xp_pattern) arg)", "(break)")
	})
	expandRow(v, "(init EXPAND_BEHAVE get_behave_arg TRUE TRUE)", ":behave's argument list")
	if v.Failed() {
		return v.Done()
	}
	if left := noinertLeft.FindAllString(string(v.Text()), -1); len(left) > 0 {
		return fmt.Errorf("noinert: still present after the cut: %s",
			strings.Join(left, ", "))
	}
	v.Say("no modifier, mapping mode or argument list that does nothing")
	return v.Done()
}

// expandRow deletes one row of ExpandOther's static table of expansions.
func expandRow(v *graph.Verbs, row, what string) {
	v.InFunction("ExpandOther", func(v *graph.Verbs) {
		if t := v.One("(def static tab _ _)", "ExpandOther's table"); t != nil {
			v.In(t, func(v *graph.Verbs) { deleteRowTyped(v, row, what) })
		}
	})
}

// deleteRowTyped deletes the one row of the scope's table pat matches, the
// table typed again for its new length as an import types it (INITROW's
// ArrangeRowsTyped), reported.
func deleteRowTyped(v *graph.Verbs, pat, what string) { deleteRowsTyped(v, []string{pat}, what) }

// deleteRowsTyped is deleteRowTyped for each pattern, one row each, in one
// arrangement.
func deleteRowsTyped(v *graph.Verbs, pats []string, what string) {
	if v.Failed() {
		return
	}
	def := v.Scope()
	init := graph.TableInit(def)
	if init == nil {
		v.Die("%s -- the scope is not a table", what)
		return
	}
	match := map[*graph.Node]bool{}
	for _, pat := range pats {
		ms := v.Find(pat)
		if len(ms) != 1 {
			v.Die("%s -- %d rows match %s, expected 1", what, len(ms), pat)
			return
		}
		match[ms[0]] = true
	}
	var order []*graph.Node
	n := 0
	for _, r := range init.Args() {
		if match[r] {
			n++
			continue
		}
		order = append(order, r)
	}
	if n != len(pats) {
		v.Die("%s -- %d rows match, expected %d", what, n, len(pats))
		return
	}
	if _, err := v.Editor().ArrangeRowsTyped(def, order, graph.RowIndex{}); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what)
}
