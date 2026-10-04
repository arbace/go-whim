package p004d

// Whim phase 4d (formerly 71) -- one buffer, structurally.  See GOAL.md.
//
// THE INVARIANT IS ALREADY TRUE; this phase removes the machinery that pretended
// otherwise.  Phase 21 allowed at most one file argument, phase 22 made :e reuse the
// one buffer, and every buffer Ex command was retired long before that: all 24 rows
// -- :buffer :buffers :ls :files :bnext :bprevious :bNext :bfirst :blast :brewind
// :bmodified :bdelete :bunload :bwipeout :bufdo :ball :badd :balt -- already read
// ex_ni, and do_buffer, do_bufdel, ex_buffer, ex_bufdo and ex_listdo do not exist.
// So NOTHING can create a second buffer:
//
// * win_alloc_first() at startup makes the one buffer, BEFORE command_line_scan;
// * buflist_add() then names it through BLN_CURBUF, reusing that same buffer;
// * do_ecmd() no longer calls buflist_new() at all (phase 22).
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

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's bodies,
// walks and literals are acts on the nodes -- the one-statement bodies built
// from templates (BUILD), the long ones and the address cases spliced as C
// in two units (FRAG, Together), the walks folded (FoldWalk), runs of items
// cut, the list's members cut -- each counted, its report the text's
// (history keeps it).

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// walk is the forward walk over the buffer list, by the variable it walks
// with (vimtext.FwdWalk's text, as a form).
func walk(v, body string) string {
	return "(for (= (paren " + v + ") firstbuf) (!= (paren " + v + ") nullptr) (= (paren " + v + ") (-> (paren " + v + ") b_next)) " + body + ")"
}

// Whim4d makes the buffer list one buffer: buf_valid() is `buf == curbuf`,
// every walk over firstbuf folds to curbuf, and the list pointers go.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("onebuf", e, w)

	// 1. the keystone: a buffer is valid exactly when it is THE buffer
	v.Body("buf_valid", "(return (== buf curbuf))", "buf_valid, which walked the list to find the buffer it was given")
	v.Body("anyBufIsChanged", "(return (call bufIsChanged curbuf))", "anyBufIsChanged, which asked every buffer")
	v.BodyC("buflist_findname_stat", w4dlit5, "buflist_findname_stat, which searched the list by name")

	fold := func(fn string, n int, what string) {
		v.InFunction(fn, func(v *graph.Verbs) { v.FoldWalk(walk("buf", "_"), "(= buf curbuf)", n, what) })
	}
	fold("ml_close_all", 1, "ml_close_all closing every buffer")
	fold("ml_close_notmod", 1, "ml_close_notmod closing every buffer")
	fold("shorten_fnames", 1, "shorten_fnames shortening every name")
	fold("did_set_paste", 3, "'paste' saving and restoring every buffer")
	fold("set_termname", 1, "a new terminal notifying every buffer")
	v.Together(func(v *graph.Verbs) {
		v.InFunction("getout", func(v *graph.Verbs) {
			v.ReplaceC(walk("buf", "_"), w4dlit6, 1, "quitting unloading every buffer, whose break bound to the walk")
		})
		v.BodyC("buflist_findpat", w4dlit7, "buflist_findpat matching against every buffer")
		v.BodyC("autowrite_all", w4dAutowriteAll, "autowrite_all writing every changed buffer")
	})
	// check_changed_any's walks went with :q's refusal at phase 1 (quitfront,
	// phase 33's move)
	v.InFunction("open_buffer", func(v *graph.Verbs) {
		v.Cut(walk("curbuf", "_"), 1, "open_buffer looking for another loaded buffer")
		v.FoldAlways("(== curbuf nullptr)", 1, "open_buffer testing whether it found one")
		v.Splice("(call emsg (call _ e_cannot_allocate_buffer_using_other_one))", "(return FAIL)", "",
			"open_buffer carrying on in another buffer instead")
	})
	v.Body("compute_buffer_local_count", "(return (-> curbuf b_fnum))", "computing a buffer address by walking to an offset")

	v.Together(func(v *graph.Verbs) {
		v.InFunction("parse_cmd_address", func(v *graph.Verbs) {
			v.LiteralC(w4dAddrPair1, w4dNewPair1, 1, "the default buffer range")
		})
		v.InFunction("address_default_all", func(v *graph.Verbs) {
			v.LiteralC(w4dAddrPair2, w4dNewPair2, 1, "the :% buffer range")
		})
		v.InFunction("get_address", func(v *graph.Verbs) {
			v.LiteralC(w4dAddrPair3, w4dNewPair3, 1, "the $ of a buffer range")
		})
		v.InFunction("invalid_range", func(v *graph.Verbs) {
			v.LiteralC(w4dAddrPair4, w4dNewPair4, 1, "validating a buffer range")
		})
	})

	// free_buffer's deferral onto au_pending_free_buf folds at phase 1:
	// autocmd_busy falls out with the autocommands' last writers (the reform's
	// D6)
	v.Rewrite("(for (= bp firstbuf) () (= bp (-> bp b_next)) ?body)", "(for (= bp curbuf) () (= bp nullptr) ?body)", 1,
		"the mapping scan walking the list, still twice: curbuf then the globals")
	// set_curbuf went with ex_quit's refusal at phase 1 (quitfront, phase 33's move)
	v.InFunction("close_buffer", func(v *graph.Verbs) {
		v.FoldNever("(&& wipe_buf (<= (-> buf b_nwindows) 0) (|| (!= (-> buf b_prev) nullptr) (!= (-> buf b_next) nullptr)))", 1,
			"close_buffer unlinking a buffer that was never linked to another")
	})
	v.InFunction("buflist_new", func(v *graph.Verbs) {
		v.Splice("(= (-> buf b_next) nullptr)", "(= lastbuf buf)", "", "buflist_new appending to the list")
		// The wiped-fnum branch is cut from its head to the plain else after it.
		v.Rewrite("(if (&& (paren (& flags BLN_REUSE)) (> (. buf_reuse ga_len) 0)) _ (block (= (-> buf b_fnum) (post++ top_file_num))))",
			"(= (-> buf b_fnum) (post++ top_file_num))", 1,
			"buflist_new reusing a wiped fnum and re-sorting the list for it")
	})
	// au_pending_free_buf, set_curbuf's valid flag, firstbuf, lastbuf and the
	// buf_reuse pool are named by nothing now; the collection takes them.
	if v.Failed() {
		return v.Done()
	}
	q := graph.NewVerbs("onebuf", e, io.Discard)
	q.Cut("(b_next (ptr buf_T))", 1, "b_next")
	q.Cut("(b_prev (ptr buf_T))", 1, "b_prev")
	if q.Err != nil {
		return q.Err
	}
	v.Say("the buffer list pointers in buf_T")
	return v.Done()
}

func init() { phase.RegisterGraph("whim4d", Edit) }
