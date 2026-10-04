package p002a

// Whim phase 2a (formerly 61) -- no window title.  See GOAL.md.
//
// 'title', 'titlelen', 'titleold', 'titlestring', 'icon' and 'iconstring' go, and
// with them everything that set or restored the terminal's title: maketitle() and
// its thirteen callers, need_maketitle, resettitle(), mch_settitle(),
// mch_restore_title(), set_title_defaults(), term_settitle(), the X11 title and
// icon probes, and the title-stack push at startup and pop at exit.  The editor
// no longer writes to the terminal's title at all.  The t_ts, t_fs, t_ST and t_RT
// terminal codes stay: the terminal codes were kept as a whole.
//
// THE DELTA: none the harnesses record.  The probes check the six are unknown.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's line cuts are Cuts
// of the statements by form, its head fold a DropIf, its two-line cut a
// CutRun (history keeps the text version).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

const (
	// restoreTitle is the call that puts the terminal's title back.
	restoreTitle = "(call mch_restore_title (paren (| SAVE_RESTORE_TITLE SAVE_RESTORE_ICON)))"
	maketitle    = "(call maketitle)"
)

// Edit takes the window title: the flag, the eleven callers that set or
// restored it, and the terminal's title stack.
func Edit(g *graph.Editor, w io.Writer, _ []string) error {
	e := graph.NewVerbs("notitle", g, w)

	for _, fn := range []string{"showruler", "redraw_cmd", "ui_focus_change", "main_loop"} {
		e.InFunction(fn, func(e *graph.Verbs) {
			e.DropIf("need_maketitle", 1, fmt.Sprintf("%s updating the title", fn))
		})
	}
	for _, fn := range []string{"enter_buffer", "buf_name_changed", "do_ecmd", "set_termname", "set_shellsize_inner", "win_enter_ext"} {
		e.InFunction(fn, func(e *graph.Verbs) {
			e.Cut(maketitle, 1, fmt.Sprintf("%s updating the title", fn))
		})
	}

	// do_exedit went with :edit at phase 1 (filefront, the reform's D4)

	e.InFunction("ex_stop", func(e *graph.Verbs) {
		e.Cut(restoreTitle, 1, ":stop restoring the title")
		e.CutRun(":stop setting the title again", maketitle, "(call resettitle)")
	})
	e.InFunction("mch_exit", func(e *graph.Verbs) {
		e.Cut(restoreTitle, 1, "exit restoring the title")
		e.Cut("(call term_pop_title (paren (| SAVE_RESTORE_TITLE SAVE_RESTORE_ICON)))", 1,
			"exit popping the terminal's title stack")
	})
	e.InFunction("vim_main2", func(e *graph.Verbs) {
		e.Cut("(call term_push_title (paren (| SAVE_RESTORE_TITLE SAVE_RESTORE_ICON)))", 1,
			"startup pushing the terminal's title stack")
	})
	e.InFunction("clear_termoptions", func(e *graph.Verbs) {
		e.Cut(restoreTitle, 1, "changing terminal restoring the title")
	})
	e.InFunction("value_changed", func(e *graph.Verbs) {
		e.Cut("(call mch_restore_title (? (== last (addr lasttitle)) SAVE_RESTORE_TITLE SAVE_RESTORE_ICON))", 1,
			"a cleared value restoring the title")
	})
	e.InFunction("set_init_3", func(e *graph.Verbs) {
		e.Cut("(call set_title_defaults)", 1, "startup choosing 'title' and 'icon' defaults")
	})

	// Five: buf_write(), changed_internal(), unchanged(), redraw_titles(), and
	// maketitle()'s own early return, which the sweep takes anyway.
	// did_set_titlelen() had a sixth and went with 'titlelen''s row, dropped
	// at phase 1 (optfront, the reform's D3).  Six, since this runs in phase
	// 2's front (the reform's D8), where what phases 3-19 took is still there.
	e.Cut("(= need_maketitle TRUE)", 6, "changes asking for a title update")
	return e.Done()
}

func init() { phase.RegisterGraph("whim2a", Edit) }
