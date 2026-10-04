package p022

// Whim phase 22 (formerly 70) -- :e reloads in place, and there is no swap file.  See GOAL.md.
//
// THIS INVARIANT IS IMPOSED, NOT PROVED, and that is the difference between it and
// phase 4c.  One window fell out of the two places a window could be created.  A
// second BUFFER is genuinely reachable: curbuf_reusable() wants an unnamed, empty
// buffer, so once the first file is named, `:e other` allocates a new buf_T and
// switches to it.  Measured on q021: `:e h2.txt` then `+wq h1.txt` writes h2.
//
// So do_ecmd() is made to reuse the one buffer:
//
// * the other_file branch renames curbuf instead of calling buflist_new(),
// sets oldbuf = FALSE, and falls through.  The rename is setfname()'s, WRITTEN
// OUT: phase 20's sweep took setfname() with set_rw_fname(), its last caller,
// so a call of it here was a call of nothing, and q022-q029 did not compile.
// What is written is what setfname() did with one buffer -- the other buffer
// of that name it could find is curbuf or none -- and buf_name_changed() as it
// stood by then, which was status_redraw_all();
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
// was the DETECTION prompt, and it was already unreachable: measured on q021, a .swp
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
// `exit= left= err=` -- the same reason :next did not move in phase 21.  Declared
// empty and left for the delta check to correct.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the two block literals are
// made nodes in one unit (FRAG: LiteralC, Together) -- stat(), stat_T and
// the header's st_dev/st_ino resolved as the importer resolves them -- and
// the rest are acts on the nodes, each counted, its report the text's
// (history keeps the text version).

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim22 makes one buffer the invariant: :edit stops opening a second, the
// swap-file dialog goes with everything that armed or answered it, and the
// swap file itself is never opened.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("onebuffer", e, w)

	// fname2fnum, which gave a file mark its own buffer, went with the file
	// marks at phase 3 (whim3f, on the front)

	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.Together(func(v *graph.Verbs) {
			v.LiteralC(w22OldOpen, w22NewOpen, 1, ":edit opening a second buffer")
			v.LiteralC(w22OldOldbuf, w22lit2, 1, ":edit deciding the buffer was already loaded")
		})
		// The body may be any length, which is the point.
		v.DropIf("(if (!= buf curbuf) (block (def save_au_new_curbuf bufref_T) _*))", 1,
			":edit leaving one buffer for another")
		v.Rewrite("(&& (! other_file) (! oldbuf))", "(! oldbuf)", 1, "the reload path asking whether the file changed")
		if r := v.Run(":edit arming the swap-file dialog", "(= swap_exists_action SEA_DIALOG)",
			"(|= (-> curbuf b_flags) BF_CHECK_RO)"); r != nil {
			if err := e.Delete(r[0]); err != nil {
				v.Die(":edit arming the swap-file dialog -- %v", err)
			} else {
				v.Say(":edit arming the swap-file dialog")
			}
		}
		v.CutRun(":edit answering it", "(if (== swap_exists_action SEA_QUIT) (block (= retval FAIL)))",
			"(call handle_swap_exists (addr old_curbuf))")
	})
	// readfile went at phase 1 (readfront, phase 31's move)
	// create_windows() armed and answered it at startup; its body is phase
	// 5b's from phase 5 (whim5b, which runs before this phase now)
	// read_stdin() armed and answered it too; it went at phase 1, nothing
	// writing the edit type that called it (argvfront, the reform's D1)
	v.InFunction("ml_open", func(v *graph.Verbs) {
		v.Cut("(= (-> buf b_may_swap) false)", 1, "ml_open clearing b_may_swap")
	})
	v.InFunction("changed", func(v *graph.Verbs) {
		v.FoldNever("(-> curbuf b_may_swap)", 1, "the first change to a buffer opening a swap file")
	})
	// the two calls of check_need_swap() went with readfile at phase 1
	// (readfront, phase 31's move)
	// handle_swap_exists(), check_swap_exists_action(), check_need_swap(),
	// ml_open_file(), swap_exists_action, the SEA_* actions and the
	// b_may_swap field are named by nothing live now; the collection takes
	// them.
	return v.Done()
}

func init() { phase.RegisterGraph("whim22", Edit) }
