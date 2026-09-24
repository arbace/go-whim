package p073

// Whim phase 73 -- one frame.  See GOAL.md.
//
// THE STRONGEST INVARIANT OF THIS RUN, and it is proved by absence rather than by
// argument: GREPPING THE WHOLE FILE FOR A WRITE TO fr_child, fr_next, fr_prev OR
// fr_parent RETURNS NOTHING AT ALL.  The frame tree is never linked.
//
// * alloc_clear(sizeof(frame_T)) appears exactly once, in new_frame(), whose only
// caller is win_alloc_firstwin() -- itself called once, from win_alloc_first();
// * new_frame() writes fr_layout = FR_LEAF and fr_win = wp, and nothing else ever
// writes fr_layout;
// * win_alloc_firstwin() sets topframe = curwin->w_frame;
// * there is no frame_insert, frame_append, frame_remove, win_split or
// win_split_ins anywhere -- they went with the window layout in phase 68/72.
//
// So topframe == curwin->w_frame, fr_layout is FR_LEAF forever, and fr_child,
// fr_next, fr_prev and fr_parent are permanently NULL.  Every FR_ROW and FR_COL
// branch is dead, every fr_child walk iterates zero times, and every fr_parent walk
// terminates on its first test.  This phase is therefore a set of BODY REPLACEMENTS,
// not a fold campaign: each function keeps the arm that runs and loses the arms that
// cannot.
//
// THE BREAK HAZARD IS HANDLED BY CONSTRUCTION.  The audit named four loops whose
// break binds to the loop being removed -- stl_connected, frame_new_height,
// frame_new_width (twice) and command_height -- and all four are in the
// replace-whole-body set, so nothing is folded out from under a break.  That is the
// phase 71 lesson applied ahead of time rather than after three dry runs.
//
// WHAT IS NOT A CONSTANT, and must keep its arithmetic:
// * frame_minheight() reads p_wh, p_wmh and w_status_height.  min_rows() and
// did_set_cmdheight()'s clamp depend on the number it returns, so the leaf arm
// stays exactly as it is; only the recursion goes.  Replacing it with a literal
// would silently change what :set cmdheight= accepts.
// * fr_width and fr_height on the one frame are live layout state, read by
// win_do_lines, screen_ins_lines, screen_del_lines, redraw_block, screen_line,
// win_line and did_set_cmdheight.  The FIELDS stay; only the tree goes.
//
// WHAT GOES BY CASCADE: frame_fixed_height and frame_fixed_width reach `return FALSE`
// and their callers' `wfh`/`wfw` loops vanish, so the sweep removes them.  The FR_ROW
// and FR_COL enumerators lose every reader.  Nothing here deletes those by name.
//
// THE DELTA: none expected.  Every window-splitting and resizing Ex command is
// already ex_ni, and :set cmdheight= keeps the same accepted range because
// frame_minheight keeps its arithmetic.  Declared empty, left for the delta check.

import (
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/edit"
)

// frameLinkWrite is any write to the frame tree's four pointers.  THE PHASE
// OPENS BY PROVING THERE ARE NONE, because every replacement below assumes a
// frame is a leaf -- if the tree really were linked somewhere, all fifteen
// bodies would be wrong and the boundary would be the first thing to say so.
var frameLinkWrite = regexp.MustCompile(`fr_(?:child|next|prev|parent)[ \t]*(?:=[^=]|\+\+|--)`)

// Whim73 makes a frame a leaf: fifteen functions that recursed into children or
// climbed to parents become constants, and the four tree pointers go.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("oneframe", text, w)

	if k := len(frameLinkWrite.FindAll(text, -1)); k > 0 {
		e.Refuse("the frame tree IS linked somewhere (%d writes) -- the invariant this phase rests on is false, and every replacement below would be wrong", k)
		return e.Done()
	}
	e.Say("confirmed: nothing writes fr_child, fr_next, fr_prev or fr_parent")

	for _, f := range []struct{ Name, Body, What string }{
		{"frame_fixed_height", w73lit1, "frame_fixed_height, asked of a leaf"},
		{"frame_fixed_width", w73lit1, "frame_fixed_width, asked of a leaf"},
		{"frame_minheight", w73lit2, "frame_minheight recursing into a row or column"},
		{"frame_minwidth", w73lit3, "frame_minwidth recursing into a row or column"},
		{"frame_check_height", w73lit4, "frame_check_height comparing against children"},
		{"frame_check_width", w73lit5, "frame_check_width comparing against children"},
		{"frame_comp_pos", w73lit6, "frame_comp_pos descending into children"},
		{"frame_new_height", w73lit7, "frame_new_height distributing height over children"},
		{"frame_new_width", w73lit8, "frame_new_width distributing width over children"},
		{"frame_setheight", w73lit9, "frame_setheight taking room from siblings"},
		{"frame_setwidth", w73lit10, "frame_setwidth taking room from siblings"},
		{"frame_add_height", w73lit11, "frame_add_height propagating to parents"},
		{"last_status_rec", w73lit12, "last_status_rec descending a row or column of frames"},
		{"command_height", w73lit13, "command_height walking to the widest ancestor"},
		{"stl_connected", w73lit1, "stl_connected, which climbed the tree for a neighbour"},
	} {
		e.Body(f.Name, f.Body, f.What)
	}
	e.Lines(`frame_T[ \t]+\*fr_parent;`, 1, "the parent pointer")
	e.Lines(`frame_T[ \t]+\*fr_next;`, 1, "the next pointer")
	e.Lines(`frame_T[ \t]+\*fr_prev;`, 1, "the previous pointer")
	e.Lines(`frame_T[ \t]+\*fr_child;`, 1, "the child pointer")
	return e.Done()
}

func init() { edit.Register("whim73", Edit) }
