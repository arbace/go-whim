package p033

// Whim phase 33 (formerly 94) -- `:q` quits, and `ZZ` is `ZQ`.  See GOAL.md.
//
// Phases 28 to 32 took every way to reach a file.  What is left of the filesystem in
// this editor is a REFUSAL: `:q` on a modified buffer answers `E37: No write since
// last change (add ! to override)` and stays.  The protection has no remedy once
// nothing can be written -- there is no `:w` to answer it with and no file the text
// could have come from -- so it is a door that opens onto nothing, and this phase
// takes it.  `:q`, `:q!`, `ZZ` and `ZQ` become one thing.
//
// THE FOLD IS PHASE 1'S.  `ex_quit()`'s test -- `check_changed(...)` or
// `check_changed_any(...)`, the refusal -- is folded NEVER at the front since
// the reform (quitfront, `internal/cut/extable.go`), so `:q` takes the else arm
// and quits, and the fall-out closure and the sweep take check_changed() and
// the switch-buffer/switch-window island its tail was the last caller of
// (`add_bufnum`, `set_curbuf`, `enter_buffer`, `win_enter_ext` and seven more)
// before this phase runs.  What is left here is what the fold made dead and no
// tool sees: the two extras below.
//
// `:q` CAN STILL DECLINE, and that is not this phase's: `text_locked()`,
// `curbuf_locked()` and `before_quit_autocmds()` all return early ABOVE the anchor
// and are untouched.  What goes is the refusal that asked whether the text had been
// saved.
//
// THE BUFFER STILL KNOWS IT IS MODIFIED.  `bufIsChanged` and `curbufIsChanged` keep
// their readers -- CTRL-G still prints `[Modified]`, the status line still draws
// `[+]`, `:set modified?` still answers.  What goes is the refusal, not the state.
//
// THERE ARE NO `'confirm'`-STYLE PROMPTS TO WORRY ABOUT: `grep -cw confirm` on the
// input is 0, whim having removed the dialog layer.  Say it, so that the next reader
// does not go looking for one.
//
// TWO EXTRAS ARE THE PHASE NOW, each measured byte-identical in the recording.
//
// A  TWO STRUCT FIELDS THAT BECOME WRITE-ONLY, WHICH NO TOOL CAN SEE.  This is
// phase 29's `usefilter` judgement in a smaller shape: tools/deadfields.py
// removes a field nothing NAMES, and gcc has no warning for a member that is
// only written.  `win_T.w_topline_was_set`'s only reader was in
// `enter_buffer()` and `wininfo_S.wi_changelistidx`'s only reader was in
// `get_winopts()`, and phase 1's sweep took both functions.
// B  THE TAIL THAT CANNOT RUN.  After the fold `ex_quit()` ends `int save_exiting
// = exiting; exiting = TRUE; getout(0); not_exiting(save_exiting);`.
// `getout()` sets `exiting = TRUE` ITSELF and ends in `mch_exit()`, which never
// returns, so the first, second and fourth statements are dead and gcc cannot
// prove it.  Replacing the four with `getout(0);` orphans `not_exiting()`, and
// `not_exiting()` IS the refusal machinery -- `exiting = save_exiting;
// settmode(TMODE_RAW);`, the "we changed our mind, put the terminal back" -- so
// it is this phase's and not tidy.  Check `getout()` before folding this on any
// other tree: the fold is right only because it sets `exiting` for itself.
//
// `ZZ` IS ALREADY `ZQ` AND STAYS SO.  `nv_Zet` runs `do_cmdline_cmd("q!")` for
// `case 'Z'` AND for `case 'Q'`, identical since phase 28.  After this phase `:q` and
// `:q!` are also identical, so all four spellings are one thing.  THE STRINGS ARE
// NOT REWRITTEN TO `"q"`: it would move `zz_key` and `zq_key` for no gain, and
// `case:zz_key` is phase 28's declaration and must not be re-declared here.
//
// WHAT LEAVES FOR A LATER PHASE TO NOTICE.  `SHM_FILEINFO` is the `'shortmess'` `f`
// letter and its only reader was inside `enter_buffer()`; the sweep takes it, and
// the letter is inert afterwards.  That is the options phase's and the flag strings
// are not touched here.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, as every Part II edit since phase 4e does, and the source goes with it as
// $state/old.c.  The check needs both: the exit status of a session that quits with
// unsaved changes is what moves, and no recording can see it.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B1b).  The cut
// is five deletions on the program's graph, through the editor,
// and its report is the text version's, which the plan ran until then
// (history keeps it, with editlit.go's literals):
//
//   - the anchors' file -- the mentions of w33Before and w33After -- is
//     asked of the C view, `\bname\b` as the text counted it (TEXTQ's
//     Mentions): the numbers include prototypes and declarations, which is
//     what they were written against;
//   - the tail is the four items `int save_exiting = exiting;`, `exiting =
//     TRUE;`, `getout(0);` and `not_exiting(save_exiting);` in a row, found
//     once in the file by pattern, and three of them deleted, the call to
//     getout keeping its node;
//   - not_exiting's "2 mentions, its prototype and its definition" is the
//     graph's question: two declarations and no use left;
//   - the two fields' declaration and writes are deleted by pattern, each
//     found once in the file.
//
// What nothing names afterwards -- not_exiting, w_topline_was_set -- is
// the collection's, as it was the sweep's.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim33", Edit) }

// w33Before is the file the two extras were counted against.  The fold, the
// refusal folded NEVER, is phase 1's since the reform (quitfront, the phase's
// move): check_changed and the whole switch-buffer island are gone before this
// phase runs, and what is left here is what the fold made dead.
var w33Before = map[string]int{
	"check_changed": 0, "not_exiting": 3, "check_changed_any": 0,
	"w_topline_was_set": 2, "wi_changelistidx": 2, "SHM_FILEINFO": 0,
	"set_curbuf": 0, "enter_buffer": 0, "get_winopts": 0, "find_wininfo": 0,
	"bufIsChanged": 7, "curbufIsChanged": 7, "bufIsChangedNotTerm": 3,
	"exiting": 16, "buf_spname": 4, "open_buffer": 4,
	"curbuf_locked": 7, "text_locked": 6, "before_quit_autocmds": 2,
	"p_ro": 0, "p_ur": 0, "read_cmd_fd": 12,
	"vim_fsync": 3, "scriptin": 8, "redir_fd": 0,
}

var w33After = map[string]int{
	"not_exiting": 2, "w_topline_was_set": 1, "wi_changelistidx": 0,
	"bufIsChanged": 7, "curbufIsChanged": 7, "bufIsChangedNotTerm": 3,
	"curbuf_locked": 7, "text_locked": 6, "before_quit_autocmds": 2,
	"p_ro": 0, "p_ur": 0, "read_cmd_fd": 12,
	"vim_fsync": 3, "scriptin": 8, "redir_fd": 0,
}

// w33Tail is ex_quit's tail, an item each, in order: the save, the set,
// the call that never returns, the restore.
var w33Tail = []string{
	"(def save_exiting int exiting)",
	"(= exiting TRUE)",
	"(call getout 0)",
	"(call not_exiting save_exiting)",
}

// Edit takes what `:q`'s refusal leaves behind once it folds: the tail
// that cannot run and two fields nothing reads.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noquit", e, w)

	// mentions holds the C view's `\bname\b` counts to want, one view for all.
	mentions := func(want map[string]int, format string) {
		if v.Failed() {
			return
		}
		t := v.Text()
		for _, name := range edit.SortedKeys(want) {
			if k := edit.MentionCount(t, name); k != want[name] {
				v.Die(format, name, k, want[name])
				return
			}
		}
	}

	// ---- 0. the shape every anchor below was counted against ------------------
	mentions(w33Before, "%s has %d mentions, expected %d -- the anchors below were counted "+
		"against a different file")
	v.Say("check_changed 0 and the switch-buffer island gone at phase 1 (quitfront), " +
		"not_exiting 3 -- the file the two extras were counted against")

	// ---- 2. extra B: the tail that cannot run ---------------------------------
	what := "ex_quit's tail: getout() sets `exiting = TRUE` itself and ends in " +
		"mch_exit(), which never returns, so the save, the set and the " +
		"restore are dead -- and not_exiting(), which was the whole of \"we " +
		"changed our mind, put the terminal back\", has no caller left"
	if tail := run(v, w33Tail, what); tail != nil {
		// the restore first, then the set, then the save it read
		for _, x := range []*graph.Node{tail[3], tail[1], tail[0]} {
			if err := e.Delete(x); err != nil {
				v.Die("%s -- %v", what, err)
				break
			}
		}
		v.Say(what)
	}
	if !v.Failed() {
		if u, d := v.UsesOf("not_exiting"), e.Decls("not_exiting"); len(u) != 0 || len(d) != 2 {
			v.Die("not_exiting has %d declarations and %d uses, expected 2 and none -- its "+
				"prototype and its definition, for the collection", len(d), len(u))
		}
	}

	// ---- 3. extra A: two fields that become write-only ------------------------
	// NOTHING SEES EITHER OF THESE.  deadfields.py removes a field nothing NAMES,
	// and a field that is only written is still named; gcc has no warning for one.
	// Their readers, in enter_buffer() and get_winopts(), went with the island
	// at phase 1 (w33Before counts both functions 0), so each field is only
	// written now.
	v.Cut("(= (-> wp w_topline_was_set) true)", 1,
		"win_T.w_topline_was_set's one surviving write, in set_topline(): its "+
			"only reader was inside enter_buffer(), and the field, named by nothing "+
			"after this, goes in the collection")
	v.Cut("(wi_changelistidx int)", 1, "wininfo_S.wi_changelistidx: its only reader was inside get_winopts()")
	v.Cut("(= (-> wip wi_changelistidx) (-> win w_changelistidx))", 1, "and its one surviving write")

	// ---- what the collection is handed, as a count rather than as trust -------
	mentions(w33After, "%s has %d mentions after the cut, expected %d")
	v.Say("the cut is done: not_exiting 2 -- its prototype and its definition, for " +
		"the collection -- and read_cmd_fd 12, vim_fsync 3 and scriptin 8 untouched, " +
		"each of them a later phase's")
	return v.Done()
}

// run is the items matching pats in a row, the first found once in the
// file and each after it its next sibling: what the text's one literal of
// several lines, counted once, asserted.
func run(v *graph.Verbs, pats []string, what string) []*graph.Node {
	first := v.One(pats[0], what)
	if first == nil {
		return nil
	}
	out := []*graph.Node{first}
	for k, src := range pats[1:] {
		x := v.Editor().Sibling(first, k+1)
		if x == nil || !graph.Matches(clisp.MustPattern(src), x) {
			v.Die("%s -- the item after %s is not %s", what, pats[k], src)
			return nil
		}
		out = append(out, x)
	}
	return out
}
