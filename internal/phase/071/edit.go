package p071

// Whim phase 71 -- one buffer, structurally.  See GOAL.md.
//
// THE INVARIANT IS ALREADY TRUE; this phase removes the machinery that pretended
// otherwise.  Phase 69 allowed at most one file argument, phase 70 made :e reuse the
// one buffer, and every buffer Ex command was retired long before that: all 24 rows
// -- :buffer :buffers :ls :files :bnext :bprevious :bNext :bfirst :blast :brewind
// :bmodified :bdelete :bunload :bwipeout :bufdo :ball :badd :balt -- already read
// ex_ni, and do_buffer, do_bufdel, ex_buffer, ex_bufdo and ex_listdo do not exist.
// So NOTHING can create a second buffer:
//
// * win_alloc_first() at startup makes the one buffer, BEFORE command_line_scan;
// * buflist_add() then names it through BLN_CURBUF, reusing that same buffer;
// * do_ecmd() no longer calls buflist_new() at all (phase 70).
//
// THREE THINGS NAMED b_next ARE NOT THE BUFFER LIST, and a regex over the name would
// gut the editor:
//
// * buffblock_T.b_next       -- the typeahead/redo chain: bh_first, redobuff,
// old_redobuff, readbuf1, readbuf2.  ~30 sites.
// * free_buffer()            -- buf->b_next = au_pending_free_buf, a free list.
// * buf_T.b_next / b_prev    -- THIS is the buffer list, and only this.
//
// Every edit below is scoped with in_function() for exactly that reason.
//
// TWO SITES ARE NOT SIMPLE FOLDS:
//
// * check_map_keycodes()'s walk is `for (bp = firstbuf; ; bp = bp->b_next)` with NO
// termination test -- it runs once per buffer and then ONCE MORE with bp == NULL,
// which is how the global (non buffer-local) maps get scanned, and breaks on that
// pass.  It becomes `for (bp = curbuf; ; bp = NULL)`: still exactly two
// iterations.  Folding it to a single pass would stop scanning half the maps.
//
// This walk feeds add_termcap_entry(), NOT mapping lookup: a mapping is found
// through curbuf->b_maphash[] directly, which never touches the buffer list and
// was never at risk here.  Worth stating because the first version of this file
// claimed otherwise.
// * close_buffer()'s wipe branch is guarded by (b_prev != NULL || b_next != NULL),
// which with a single buffer is ALREADY false.  The splice it guards is dead
// today, not merely dead afterwards, so the guard folds to its else.
//
// WHAT IS LOST: nothing reachable.  buf_valid() becomes `buf == curbuf`, which makes
// set_curbuf()'s `enter_buffer(lastbuf)` fallback unreachable and takes the rest of
// set_curbuf's other-buffer handling with it.
//
// WHAT STAYS: buf_hashtab and buflist_findnr(), because five live callers still look
// a buffer up by number -- eval_vars, setmark_pos, check_changed_any, buflist_nr2name
// and buflist_getfile.  Collapsing that to a curbuf test is a separate step.
//
// THE DELTA: none expected.  The buffer commands are already ex_ni, so no exsweep row
// can move; declared empty and left for the delta check to correct.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

// Whim71 makes the buffer list one buffer: buf_valid() is `buf == curbuf`,
// every walk over firstbuf folds to curbuf, and the list pointers go.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("onebuf", text, w)

	// 1. the keystone: a buffer is valid exactly when it is THE buffer
	e.Body("buf_valid", w71lit3, "buf_valid, which walked the list to find the buffer it was given")
	e.Body("anyBufIsChanged", w71lit4, "anyBufIsChanged, which asked every buffer")
	e.Body("buflist_findname_stat", w71lit5, "buflist_findname_stat, which searched the list by name")

	e.FoldWalk("ml_close_all", "buf", edit.FwdWalk, 1, "ml_close_all closing every buffer")
	e.FoldWalk("ml_close_notmod", "buf", edit.FwdWalk, 1, "ml_close_notmod closing every buffer")
	e.FoldWalk("shorten_fnames", "buf", edit.FwdWalk, 1, "shorten_fnames shortening every name")
	e.FoldWalk("did_set_paste", "buf", edit.FwdWalk, 3, "'paste' saving and restoring every buffer")
	e.FoldWalk("set_termname", "buf", edit.FwdWalk, 1, "a new terminal notifying every buffer")
	e.DropWalk("getout", edit.FwdWalk, w71lit6, 1, "quitting unloading every buffer, whose break bound to the walk")
	e.Body("buflist_findpat", w71lit7, "buflist_findpat matching against every buffer")
	e.DropWalk("check_changed_any", edit.FwdWalk, w71lit8,
		2, "counting the buffers to check, and re-adding the one already seeded")
	e.DropWalk("open_buffer", strings.ReplaceAll(edit.FwdWalk, "(buf)", "(curbuf)"), "", 1, "open_buffer looking for another loaded buffer")

	e.InFunction("open_buffer", func(e *edit.E) {
		e.FoldAlways(`(?m)^[ \t]*if \(curbuf == NULL\)$`, 1, "open_buffer testing whether it found one")
		e.Literal(w71lit15, "", 1, "open_buffer carrying on in another buffer instead")
	})
	e.Body("compute_buffer_local_count", w71lit9, "computing a buffer address by walking to an offset")

	e.InFunction("parse_cmd_address", func(e *edit.E) {
		e.Literal(w71AddrPair1, w71NewPair1, 1, "the default buffer range")
	})
	e.InFunction("address_default_all", func(e *edit.E) {
		e.Literal(w71AddrPair2, w71NewPair2, 1, "the :% buffer range")
	})
	e.InFunction("get_address", func(e *edit.E) {
		e.Literal(w71AddrPair3, w71NewPair3, 1, "the $ of a buffer range")
	})
	e.InFunction("invalid_range", func(e *edit.E) {
		e.Literal(w71AddrPair4, w71NewPair4, 1, "validating a buffer range")
	})

	e.InFunction("free_buffer", func(e *edit.E) {
		e.Literal(w71lit16, w71lit17, 1, "free_buffer deferring onto a chain nothing ever drained")
	})
	e.Literal(w71lit10, w71lit11, 1, "the mapping scan walking the list, still twice: curbuf then the globals")
	e.InFunction("set_curbuf", func(e *edit.E) {
		e.Literal(w71lit18, w71lit19, 1, "set_curbuf entering a different buffer")
	})
	e.InFunction("close_buffer", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(wipe_buf && buf->b_nwindows <= 0 && \(buf->b_prev != NULL \|\| buf->b_next != NULL\)\)$`, 1,
			"close_buffer unlinking a buffer that was never linked to another")
	})
	e.InFunction("buflist_new", func(e *edit.E) {
		e.Literal(w71lit20, "", 1, "buflist_new appending to the list")
	})

	// The wiped-fnum branch is cut from its head to the plain else after it.
	e.Splice(w71OldReuseStart, w71OldReuseEnd, w71lit13, "buflist_new reusing a wiped fnum and re-sorting the list for it")
	// au_pending_free_buf, set_curbuf's valid flag, firstbuf, lastbuf and the
	// buf_reuse pool are named by nothing now; the sweep takes them.
	e.Literal(w71lit12, "", 1, "the buffer list pointers in buf_T")
	return e.Done()
}

func init() { edit.Register("whim71", Edit) }
