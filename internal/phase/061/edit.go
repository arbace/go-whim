package p061

// Whim phase 61 -- no window title.  See GOAL.md.
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

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

var (
	// titleWanted is the flag a change sets to ask for a title update.
	titleWanted = edit.Head("if (need_maketitle)")
	// restoreTitle is the call that puts the terminal's title back.
	restoreTitle = edit.Line("mch_restore_title((SAVE_RESTORE_TITLE | SAVE_RESTORE_ICON));")
)

// Whim61 takes the window title: the flag, the eleven callers that set or
// restored it, and the terminal's title stack.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("notitle", text, w)

	for _, fn := range []string{"showruler", "redraw_cmd", "ui_focus_change", "main_loop"} {
		fn := fn
		e.InFunction(fn, func(e *edit.E) {
			e.DropIf(titleWanted, 1, fmt.Sprintf("%s updating the title", fn))
		})
	}
	for _, fn := range []string{"enter_buffer", "buf_name_changed", "do_ecmd", "set_termname", "set_shellsize_inner", "win_enter_ext"} {
		fn := fn
		e.InFunction(fn, func(e *edit.E) {
			e.Cut(edit.Line("maketitle();"), 1, fmt.Sprintf("%s updating the title", fn))
		})
	}

	// do_exedit went with :edit at phase 1 (filefront, the reform's D4)

	e.InFunction("ex_stop", func(e *edit.E) {
		e.Cut(restoreTitle, 1, ":stop restoring the title")
		e.Cut(edit.Line("maketitle();", "resettitle();"), 1, ":stop setting the title again")
	})
	e.InFunction("mch_exit", func(e *edit.E) {
		e.Cut(restoreTitle, 1, "exit restoring the title")
		e.Cut(edit.Line("term_pop_title((SAVE_RESTORE_TITLE | SAVE_RESTORE_ICON));"), 1,
			"exit popping the terminal's title stack")
	})
	e.InFunction("vim_main2", func(e *edit.E) {
		e.Cut(edit.Line("term_push_title((SAVE_RESTORE_TITLE | SAVE_RESTORE_ICON));"), 1,
			"startup pushing the terminal's title stack")
	})
	e.InFunction("clear_termoptions", func(e *edit.E) {
		e.Cut(restoreTitle, 1, "changing terminal restoring the title")
	})
	e.InFunction("value_changed", func(e *edit.E) {
		e.Cut(edit.Line("mch_restore_title(last == &lasttitle ? SAVE_RESTORE_TITLE : SAVE_RESTORE_ICON);"), 1,
			"a cleared value restoring the title")
	})
	e.InFunction("set_init_3", func(e *edit.E) {
		e.Cut(edit.Line("set_title_defaults();"), 1, "startup choosing 'title' and 'icon' defaults")
	})

	// Five: buf_write(), changed_internal(), unchanged(), redraw_titles(), and
	// maketitle()'s own early return, which the sweep takes anyway.
	// did_set_titlelen() had a sixth and went with 'titlelen''s row, dropped
	// at phase 1 (optfront, the reform's D3).
	e.Cut(edit.Line("need_maketitle = TRUE;"), 5, "changes asking for a title update")
	return e.Done()
}

func init() { phase.Register("whim61", Edit) }
