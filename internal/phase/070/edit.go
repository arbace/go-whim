package p070

// Whim phase 70 -- :e reloads in place, and there is no swap file.  See GOAL.md.
//
// THIS INVARIANT IS IMPOSED, NOT PROVED, and that is the difference between it and
// phase 68.  One window fell out of the two places a window could be created.  A
// second BUFFER is genuinely reachable: curbuf_reusable() wants an unnamed, empty
// buffer, so once the first file is named, `:e other` allocates a new buf_T and
// switches to it.  Measured on q69: `:e h2.txt` then `+wq h1.txt` writes h2.
//
// So do_ecmd() is made to reuse the one buffer:
//
// * the other_file branch renames curbuf with setfname() instead of calling
// buflist_new(), sets oldbuf = FALSE, and falls through;
// * the reload path below it -- u_sync(), u_savecommon(), buf_freeall(curbuf,
// BFA_KEEP_UNDO), then open_buffer(... READ_KEEP_UNDO) -- ALREADY IS "wipe and
// re-read in place".  Its gate widens from `!other_file && !oldbuf` to `!oldbuf`;
// * the whole `if (buf != curbuf)` block goes: BufLeave, buf_copy_options, u_sync,
// close_buffer(DOBUF_WIPE), the auto_buf dance, the curwin->w_buffer swap and
// get_winopts.  It is removed by brace matching, not by matching its body.
//
// ORDER: fname2fnum() FIRST.  It calls buflist_new(name, p, 1, 0) to give a file
// mark's file a buffer, and once reuse is unconditional that call would wipe the
// buffer being edited.  Folded to nothing; getmark_buf_fnum() already treats a zero
// fnum as "not in a buffer".
//
// NO SWAP FILE, EVER -- not even one left from another age.  Swap files are already
// never WRITTEN here: findswapname, p_swf, swapfile_info, swapfile_unchanged,
// ml_recover and ml_sync_all are gone, mf_open() is the in-memory memfile, and
// ml_open_file() had been reduced to a single `b_may_swap = FALSE`.  What survived
// was the DETECTION prompt, and it was already unreachable: measured on q69, a .swp
// sitting beside the file produces no prompt at all, and the edit and the write go
// through in silence.  So swap_exists_action, the three SEA_* actions,
// handle_swap_exists(), check_swap_exists_action(), check_need_swap(), ml_open_file()
// and the b_may_swap field all go together -- vestigial scaffolding, the can_cindent
// shape again: a flag written in three places and never true.
//
// The one test that was not obviously dead is in changed(), not buf_write -- the
// first change to a buffer used to open its swap file there.  It is folded away.
//
// WHAT IS LOST: the state of the file you leave -- its undo history and its marks.
// `:e` and `:wq` keep working, on one buffer.  What stays: the load path, the
// argument-free reload (`:e` with no name), and :e! discarding changes.
//
// THE DELTA: none expected.  :e prints nothing to stderr, and an exsweep row is
// `exit= left= err=` -- the same reason :next did not move in phase 69.  Declared
// empty and left for the delta check to correct.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// Whim70 makes one buffer the invariant: :edit stops opening a second, the
// swap-file dialog goes with everything that armed or answered it, and the
// swap file itself is never opened.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("onebuffer", text, w)

	// fname2fnum's Body goes entirely: a file mark gets its own buffer now.
	e.Body("fname2fnum", "", "fname2fnum giving a file mark its own buffer")

	e.InFunction("do_ecmd", func(e *edit.E) {
		e.Literal(w70OldOpen, w70NewOpen, 1, ":edit opening a second buffer")
		e.Literal(w70OldOldbuf, w70lit2, 1, ":edit deciding the buffer was already loaded")
		// Brace-matched: the Body may be any length, which is the point.
		e.DropIf(`(?m)^[ \t]*if \(buf != curbuf\)\n[ \t]*\{\n[ \t]*bufref_T[ \t]+save_au_new_curbuf;$`, 1,
			":edit leaving one buffer for another")
		e.Literal(w70lit3, w70lit4, 1, "the reload path asking whether the file changed")
		e.Literal(w70lit5, w70lit6, 1, ":edit arming the swap-file dialog")
		e.Literal(w70lit7, "", 1, ":edit answering it")
	})
	e.InFunction("readfile", func(e *edit.E) {
		e.Literal(w70lit8, "", 1, "reading a file abandoning it for a swap file")
	})
	e.InFunction("create_windows", func(e *edit.E) {
		e.Literal(w70lit9, w70lit10, 1, "the startup open arming and answering the dialog")
	})
	e.InFunction("read_stdin", func(e *edit.E) {
		e.Literal(w70lit11, "", 1, "reading stdin arming the dialog")
		e.Literal(w70lit12, "", 1, "reading stdin answering it")
	})
	e.InFunction("ml_open", func(e *edit.E) {
		e.Cut(edit.Line("buf->b_may_swap = false;"), 1, "ml_open clearing b_may_swap")
	})
	e.InFunction("changed", func(e *edit.E) {
		e.FoldNever(edit.Head("if (curbuf->b_may_swap)"), 1, "the first change to a buffer opening a swap file")
	})
	e.Cut(edit.Line("check_need_swap(newfile);"), 2, "the two calls that asked for a swap file")
	// handle_swap_exists(), check_swap_exists_action(), check_need_swap(),
	// ml_open_file(), swap_exists_action, the SEA_* actions and the
	// b_may_swap field are named by nothing live now; the sweep takes them.
	return e.Done()
}

func init() { edit.Register("whim70", Edit) }
