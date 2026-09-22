package p058

// Whim phase 58 -- no language mappings.  See GOAL.md.
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

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// langmapCmds are the four commands that make a language mapping.  A row is
// pointed at ex_ni rather than deleted, which is this table's own rule.
var langmapCmds = []struct{ Name, handler string }{
	{"lmap", "ex_map"},
	{"lnoremap", "ex_map"},
	{"lunmap", "ex_unmap"},
	{"lmapclear", "ex_mapclear"},
}

// Whim58 takes language mappings: the four commands, the two CTRL-^ toggles,
// and every place a mode flag said a key was being read through one.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nolangmap", text, w)

	for _, c := range langmapCmds {
		e.Sub(fmt.Sprintf(`(?m)^([ \t]*\[CMD_%s\] = \{\(char_u \*\)"%s", sizeof\("%s"\) - 1, )%s,`,
			c.Name, c.Name, c.Name, c.handler), "${1}ex_ni,", 1,
			fmt.Sprintf(":%s points at ex_ni", c.Name))
	}
	e.InFunction("set_context_by_cmdname", func(e *edit.E) {
		for _, c := range langmapCmds {
			e.Cut(fmt.Sprintf(`(?m)^[ \t]*case CMD_%s:\n`, c.Name), 1,
				fmt.Sprintf("no completion for :%s", c.Name))
		}
	})

	// CTRL-^: consumed, and nothing to toggle.
	e.InFunction("edit", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(curbuf->b_p_iminsert == B_IMODE_LMAP\)$`,
			"Insert mode starting with language mappings")
		e.Sub(`(?m)^([ \t]*case Ctrl_HAT:\n)[ \t]*ins_ctrl_hat\(\);\n`, "${1}", 1,
			"CTRL-^ in Insert mode toggling nothing")
	})
	e.InFunction("getcmdline_int", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(firstc == '/' \|\| firstc == '\?' \|\| firstc == '@'\)$`,
			"a search line starting with language mappings")
		e.Sub(`(?m)^([ \t]*case Ctrl_HAT:\n)[ \t]*cmdline_toggle_langmap\([^\n]*\);\n`, "${1}", 1,
			"CTRL-^ on the command line toggling nothing")
	})
	e.InFunction("ex_append", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(curbuf->b_p_iminsert == B_IMODE_LMAP\)$`,
			":append starting with language mappings")
	})
	e.InFunction("ins_insert", func(e *edit.E) {
		e.LiteralN(" | (State & MODE_LANGMAP)", "", 2,
			"<Insert> keeping the language-mapping flag")
	})
	e.InFunction("normal_cmd_get_more_chars", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(lang && curbuf->b_p_iminsert == B_IMODE_LMAP\)$`,
			"r, f and t reading through language mappings")
		e.DropIf(`(?m)^[ \t]*if \(langmap_active\)$`,
			"r, f and t restoring after language mappings")
	})
	e.InFunction("handle_mapping", func(e *edit.E) {
		e.Literal(" && ((mp->m_mode & MODE_LANGMAP) == 0 || typebuf.tb_maplen == 0)", "",
			"a mapping refused only for a language mapping")
	})
	e.InFunction("vgetorpeek", func(e *edit.E) {
		e.Literal("((State & (MODE_NORMAL | MODE_INSERT)) || State == MODE_LANGMAP)",
			"(State & (MODE_NORMAL | MODE_INSERT))",
			"the cursor placed while waiting in language-mapping state")
	})
	e.InFunction("get_map_mode", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*else if \(modec == 'l'\)$`, "the 'l' map mode")
	})
	e.InFunction("map_mode_to_chars", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*else if \(mode & MODE_LANGMAP\)$`, "listing a mapping as 'l'")
	})
	e.InFunction("win_redr_status", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(\(NameBufflen = get_keymap_str\(wp, \(char_u \*\)"<%s>", NameBuff,  PATH_MAX \)\) > 0`,
			"the status line's <lang>")
	})
	return e.Done()
}

func init() { edit.Register("whim58", Edit) }
