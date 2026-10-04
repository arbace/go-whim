package p003b

// Whim phase 3b (formerly 58) -- no language mappings.  See GOAL.md.
//
// 'iminsert' and 'imsearch' are 0 from here on, so language mappings are never
// active and nothing can make them so:
//
// :lmap :lnoremap :lunmap :lmapclear   point at ex_ni, and their completion goes
// CTRL-^ in Insert and on the command line   still consumed, and does nothing;
// it toggled MODE_LANGMAP and the two options
// MODE_LANGMAP   never set, so every test of it folds: in edit(), ex_append(),
// ins_insert(), normal_cmd_get_more_chars()'s r/f/t lookup, getcmdline_int()
// for / ? @, handle_mapping(), vgetorpeek(), get_map_mode() and
// map_mode_to_chars()
// the status line's <lang>   get_keymap_str() only ever printed it
//
// THE DELTA: :lmap, :lnoremap and :lmapclear, now ex_ni.  :lunmap is ex_ni too, and
// its row does not move: bare, it already failed for want of an argument.  The probes check the options
// are unknown, :lmap is refused, and CTRL-^ in Insert mode inserts nothing.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the case labels, folds and
// dropped operands by form, each scoped to its function (history keeps the
// text version).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// langmapCmds are the four commands that make a language mapping.  A row is
// pointed at ex_ni rather than deleted, which is this table's own rule.
var langmapCmds = []string{"lmap", "lnoremap", "lunmap", "lmapclear"}

// cutAfter deletes the statement that follows the case label, the two found
// as a run once in the scope.
func cutAfter(v *graph.Verbs, label, stmt, what string) {
	if r := v.Run(what, label, stmt); r != nil {
		if err := v.Editor().Delete(r[1]); err != nil {
			v.Die("%s -- %v", what, err)
			return
		}
		v.Say(what)
	}
}

// Edit takes language mappings: the four commands, the two CTRL-^ toggles,
// and every place a mode flag said a key was being read through one.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nolangmap", e, w)

	// the four rows point at ex_ni from phase 1 (exfront, the reform's D2)
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		for _, c := range langmapCmds {
			v.Cut("(case CMD_"+c+")", 1, fmt.Sprintf("no completion for :%s", c))
		}
	})

	// CTRL-^: consumed, and nothing to toggle.
	v.InFunction("edit", func(v *graph.Verbs) {
		v.DropIf("(== (-> curbuf b_p_iminsert) B_IMODE_LMAP)", 1,
			"Insert mode starting with language mappings")
		cutAfter(v, "(case Ctrl_HAT)", "(call ins_ctrl_hat)", "CTRL-^ in Insert mode toggling nothing")
	})
	v.InFunction("getcmdline_int", func(v *graph.Verbs) {
		v.DropIf("(|| (== firstc '/') (== firstc '?') (== firstc '@'))", 1,
			"a search line starting with language mappings")
		cutAfter(v, "(case Ctrl_HAT)", "(call cmdline_toggle_langmap _)", "CTRL-^ on the command line toggling nothing")
	})
	v.InFunction("ex_append", func(v *graph.Verbs) {
		v.DropIf("(== (-> curbuf b_p_iminsert) B_IMODE_LMAP)", 1,
			":append starting with language mappings")
	})
	v.InFunction("ins_insert", func(v *graph.Verbs) {
		v.DropOperand("(paren (& State MODE_LANGMAP))", 2,
			"<Insert> keeping the language-mapping flag")
		// the stores lost their operand: typed again from their left side
		for _, u := range append([]*graph.Node(nil), e.Untyped...) {
			e.Rederive(u)
		}
	})
	v.InFunction("normal_cmd_get_more_chars", func(v *graph.Verbs) {
		v.DropIf("(&& lang (== (-> curbuf b_p_iminsert) B_IMODE_LMAP))", 1,
			"r, f and t reading through language mappings")
		v.DropIf("langmap_active", 1, "r, f and t restoring after language mappings")
	})
	v.InFunction("handle_mapping", func(v *graph.Verbs) {
		v.DropOperand("(|| (== (& (-> mp m_mode) MODE_LANGMAP) 0) (== (. typebuf tb_maplen) 0))", 1,
			"a mapping refused only for a language mapping")
	})
	v.InFunction("vgetorpeek", func(v *graph.Verbs) {
		v.Rewrite("(|| ?a (== State MODE_LANGMAP))", "?a", 1,
			"the cursor placed while waiting in language-mapping state")
	})
	v.InFunction("get_map_mode", func(v *graph.Verbs) {
		v.FoldNever("(== modec 'l')", 1, "the 'l' map mode")
	})
	v.InFunction("map_mode_to_chars", func(v *graph.Verbs) {
		v.FoldNever("(& mode MODE_LANGMAP)", 1, "listing a mapping as 'l'")
	})
	v.InFunction("win_redr_status", func(v *graph.Verbs) {
		v.DropIf("(&& (> (= NameBufflen (call get_keymap_str _ _ _ _)) 0) _)", 1,
			"the status line's <lang>")
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim3b", Edit) }
