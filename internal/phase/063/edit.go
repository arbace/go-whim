package p063

// Whim phase 63 -- no jump list.  See GOAL.md.
//
// The per-window jump list goes: w_jumplist, w_jumplistlen and w_jumplistidx,
// setpcmark() appending to it, CTRL-O and CTRL-I walking it through movemark(),
// :jumps and :clearjumps, cleanup_jumplist(), copying it to a new window and
// freeing it with one, and the loops that kept its marks right when lines moved
// or a file was forgotten.
//
// What stays, because it is not the jump list: the previous-context mark behind
// '' and `` (w_pcmark, still set by setpcmark()), the change list and g; g,
// (nv_pcmark() keeps that half), :keepjumps (it guards the pcmark and the change
// list too), and JUMPLISTSIZE, which sizes the change list.  CTRL-O in Select mode
// still runs one Visual command; anywhere else CTRL-O and CTRL-I beep.
//
// THE DELTA: :jumps and :clearjumps, now ex_ni.  The probes check CTRL-O no longer
// jumps back, '' still does, and :jumps is refused.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim63 takes the jump list: :jumps and :clearjumps, CTRL-I and CTRL-O, and
// every place a line or column change moved its marks.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nojumplist", text, w)

	for _, c := range []struct{ Name, handler string }{{"jumps", "ex_jumps"}, {"clearjumps", "ex_clearjumps"}} {
		e.Sub(fmt.Sprintf(`(?m)^([ \t]*\[CMD_%s\] = \{\(char_u \*\)"%s", sizeof\("%s"\) - 1, )%s,`,
			c.Name, c.Name, c.Name, c.handler), "${1}ex_ni,", 1,
			fmt.Sprintf(":%s points at ex_ni", c.Name))
	}
	e.Sub(`(?m)^([ \t]*\{Ctrl_I, )nv_pcmark(, 0, 0\},)`, "${1}nv_error${2}", 1,
		"CTRL-I in Normal mode points at nv_error")

	// The append is cut from its test to the last line of its Body.  It is
	// matched by its two ends because the Body is the whole of building a
	// jump-list entry, which shares no shape with the test above it.
	e.InFunction("setpcmark", func(e *edit.E) {
		e.Splice("    if (++curwin->w_jumplistlen > JUMPLISTSIZE)\n", "fm->fname = NULL;\n", "",
			"setpcmark appending to the jump list")
	})

	e.InFunction("nv_ctrlo", func(e *edit.E) {
		e.Sub(`(?m)^([ \t]*)cap->count1 = -cap->count1;\n[ \t]*nv_pcmark\(cap\);\n`,
			"${1}clearopbeep(cap->oap);\n", 1, "CTRL-O walking back through the jump list")
	})
	e.InFunction("nv_pcmark", func(e *edit.E) {
		e.DropIf(edit.Head("if (cap->cmdchar == TAB && mod_mask == MOD_MASK_CTRL)"), 1,
			"CTRL-Tab refused by the jump-list command")
		e.Sub(`(?m)^([ \t]*)if \(cap->cmdchar == 'g'\)\n[ \t]*\{\n[ \t]*pos = movechangelist\(\(int\)cap->count1\);\n[ \t]*\}\n[ \t]*else\n[ \t]*\{\n[ \t]*pos = movemark\(\(int\)cap->count1\);\n[ \t]*\}\n`,
			"${1}pos = movechangelist((int)cap->count1);\n", 1,
			"the jump list as the other half of nv_pcmark")
		e.Sub(`(?m)^([ \t]*)else if \(cap->cmdchar == 'g'\)$`, "${1}else", 1,
			"the change-list messages no longer choosing by key")
		e.Cut(edit.Line("else", "{", "clearopbeep(cap->oap);", "}"), 1,
			"a jump-list miss beeping")
	})
	e.InFunction("mark_adjust_internal", func(e *edit.E) {
		e.DropIf(edit.Head("for (i = 0; i < win->w_jumplistlen; ++i)"), 1, "line changes moving jump-list marks")
		e.Cut(edit.Line("if ((cmdmod.cmod_flags & CMOD_LOCKMARKS) == 0)", "{", "}"), 1,
			"the now-empty 'lockmarks' test around them")
	})
	e.InFunction("mark_col_adjust", func(e *edit.E) {
		e.DropIf(edit.Head("for (i = 0; i < win->w_jumplistlen; ++i)"), 1, "column changes moving jump-list marks")
	})
	e.InFunction("mark_forget_file", func(e *edit.E) {
		e.DropIf(edit.Head("for (i = wp->w_jumplistlen - 1; i >= 0; --i)"), 1, "a forgotten file leaving the jump list")
	})
	e.InFunction("fmarks_check_names", func(e *edit.E) {
		e.Cut(`(?m)^[ \t]*for \(\(wp\) = firstwin; \(wp\) != NULL; \(wp\) = \(wp\)->w_next\)\s*\n[ \t]*\{\n[ \t]*for \(i = 0; i < wp->w_jumplistlen; \+\+i\)\n[ \t]*\{\n[ \t]*fmarks_check_one\(&wp->w_jumplist\[i\], name, buf\);\n[ \t]*\}\n[ \t]*\}\n`,
			1, "a named buffer resolving jump-list file names")
	})
	e.InFunction("win_init", func(e *edit.E) {
		e.Cut(edit.Line("copy_jumplist(oldp, newp);"), 1, "a new window copying the jump list")
	})
	e.InFunction("win_free", func(e *edit.E) {
		e.Cut(edit.Line("free_jumplist(wp);"), 1, "a closed window freeing the jump list")
	})
	return e.Done()
}

func init() { phase.Register("whim63", Edit) }
