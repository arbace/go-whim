package p067

// Whim phase 67 -- no mouse, no spell plumbing, no write-only flags.
// See GOAL.md.
//
// Three cuts, none of which changes what the editor can do, because none of it
// could happen in the first place.
//
// THE MOUSE, WHICH CANNOT ARRIVE.  There is no 'mouse' option row, and
// setmouse(), mch_setmouse(), mouse_has() and p_mouse are all gone, so
// nothing ever asks a terminal to report mouse events.  What served them
// goes: is_mouse_key() and the term in the input loop that called it,
// reset_dragwin()/reset_held_button() with dragwin and held_button,
// mouse_row/mouse_col and old_mouse_row/old_mouse_col -- a save-and-restore
// pair that nothing else reads -- the 13 mouse rows of key_names_table, the
// [MOUSE] entry of the terminal string table, and check_termcode()'s mouse
// matching.  The 26 nv_cmds rows STAY at nv_error: that table's index is a
// permutation of its rows, so a removed row renumbers the keys after it.
//
// ONE REAL CHANGE OF BEHAVIOUR IS BURIED HERE, and it is why the pty check
// below matters.  `looks_like_mouse_start` is not mouse-specific despite
// its name: it is set for ANY two-byte `ESC [` termcode whose third byte is
// not a digit, and it defers the match so that a longer code -- a mouse one
// -- can win instead.  With no mouse code able to arrive, deferring can only
// lose, so the fold makes such a code match at once.
//
// THE SPELL PLUMBING.  spellvars_T is one field, win_line()'s spv parameter is
// already __attribute__((unused)), and win_update() declares one on the
// stack only to pass its address twice.
//
// FOURTEEN WRITE-ONLY STATICS.  gcc never warns about these -- a static that is
// assigned counts as used -- which is the blind spot that hid can_cindent
// until phase 64 and struct fields until deadfields.py.  Two of them are a
// whole function body each, so state_no_longer_safe() and its two calls go
// with was_safe.
//
// vim_ignored IS NOT ONE OF THEM, though it looks identical to the detector.
// Its five sites are `vim_ignored = ftruncate(...)`, `= dup(2)` and
// `= write(1, ...)`: it exists to swallow warn_unused_result, and removing
// it ADDS warnings.  A (void) cast does not silence that attribute in gcc.
//
// THE DELTA: none.  No key, command or option changes -- every cut is code that
// nothing could reach.  The probes check the editor still starts, edits and
// writes, and the pty check is what would catch the termcode fold going wrong.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// The mouse names in key_names_table are matched ON THE NAME and not on the
// row's first field: five of eighteen -- DecMouse, JsbMouse, NetMouse,
// PtermMouse, UrxvtMouse -- are written with the key code first and a trailing
// FALSE across THREE lines, so an anchor on `{TRUE,` found 13 and left the
// terminal-specific ones behind, and a single-line pattern cannot see them at
// all.
const (
	mouseNameOneLine   = `(?m)^[ \t]*\{TRUE,[^\n]*\(char_u \*\)\("(\w*(?:Mouse|Drag|Release|Wheel)\w*)"\)[^\n]*\n`
	mouseNameThreeLine = `(?m)^[ \t]*\{FALSE, [^\n]*\(char_u \*\)\("(\w*Mouse\w*)"\)[^\n]*\n`
)

// writeOnlyStatics are file-scope variables that are written and never read:
// their writes go here, and the declarations, named by nothing after that,
// go to the sweep.  Where the write is a whole `if` Body or a whole
// function, that goes too -- see the cases below the loop.
var writeOnlyStatics = []struct {
	writes     string
	n          int
	writesWhat string
}{
	{`did_check_timestamps = FALSE;`, 3, "the three writes to did_check_timestamps"},
	{`did_emsg_syntax = (?:TRUE|FALSE);`, 2, "did_emsg_syntax's two writes"},
	{`typebuf_was_empty = (?:TRUE|FALSE);`, 2, "typebuf_was_empty's two writes"},
	{`in_mch_delay = (?:TRUE|FALSE);`, 2, "in_mch_delay's two writes"},
}

// Whim67 takes the mouse -- every key name, the deferred-match machinery in
// check_termcode and the statics that tracked a pointer -- the spell plumbing
// win_line still carried, and eleven file-scope variables written and never
// read.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nomouse", text, w)

	e.Literal(" || (is_mouse_key(n) && n != (-((KS_EXTRA) + ((int)(KE_LEFTMOUSE) << 8))))", "", 1,
		"the input loop asking whether a key is a mouse key")
	e.Cut(edit.Line("reset_dragwin();"), 2, "the two calls that forgot the dragged window")
	e.Cut(edit.Line("reset_held_button();"), 1, "the call that forgot the held button")
	// mouse_row/col and old_mouse_row/col are a closed loop: saved here,
	// restored there, read by nothing else.
	e.Cut(edit.Line("mouse_row = old_mouse_row;"), 1, "restoring the mouse row")
	e.Cut(edit.Line("mouse_col = old_mouse_col;"), 1, "restoring the mouse column")
	e.Cut(edit.Line("old_mouse_row = mouse_row;"), 1, "saving the mouse row")
	e.Cut(edit.Line("old_mouse_col = mouse_col;"), 1, "saving the mouse column")

	one := e.Query(mouseNameOneLine, 1)
	e.Expect(len(one) == 13, "key_names_table -- %d single-line mouse names, expected 13: %s", len(one), strings.Join(one, " "))
	e.Cut(mouseNameOneLine, 13, "the mouse key names: "+strings.Join(one, " "))
	three := e.Query(mouseNameThreeLine, 1)
	e.Expect(len(three) == 5, "key_names_table -- %d three-line mouse names, expected 5: %s", len(three), strings.Join(three, " "))
	e.Cut(mouseNameThreeLine, 5, "the terminal-specific mouse names: "+strings.Join(three, " "))
	e.Cut(`(?m)^[ \t]*\{\(-\(\(KS_MOUSE\) \+ \(\(int\)\(\('X'\)\) << 8\)\)\),[^\n]*"\[MOUSE\]"\},\n`, 1,
		"the [MOUSE] entry of the terminal string table")

	e.InFunction("check_termcode", func(e *edit.E) {
		// The whole `slen == 2 && ESC [` block existed to set that flag, and its
		// only other arm counted the semicolons of a DEC mouse report.
		e.DropIf(edit.Head("if (slen == 2 && len > 2 && termcodes[idx].code[0] == ESC && termcodes[idx].code[1] == '[')"), 1,
			"deferring an ESC [ code in case a mouse code is longer")
		e.FoldNever(edit.Head("if (looks_like_mouse_start)"), 1, "a deferred match winning over a real one")
		e.Literal(" && mouse_index_found < 0", "", 1, "the modifier scan waiting for a deferred mouse match")
		e.FoldNever(edit.Head("else if (idx == tc_len && mouse_index_found >= 0)"), 1, "falling back to the deferred mouse match")
		e.Cut(edit.Line("if (key_name[0] == KS_MOUSE || key_name[0] == KS_SGR_MOUSE || key_name[0] == KS_SGR_MOUSE_RELEASE)", "{", "}"), 1,
			"a mouse report being handled by an empty block")
	})

	// the spell plumbing
	e.Literal(", spellvars_T *spv __attribute__((unused)))", ")", 1, "win_line's unused spell parameter")
	e.Literal("win_line(wp, lnum, srow, wp->w_height, 0, &spv)", "win_line(wp, lnum, srow, wp->w_height, 0)", 1, "the first win_line call")
	e.Literal("win_line(wp, lnum, srow, wp->w_height, wp->w_lines[idx].wl_size, &spv)",
		"win_line(wp, lnum, srow, wp->w_height, wp->w_lines[idx].wl_size)", 1, "the second win_line call")

	// the write-only statics
	for _, s := range writeOnlyStatics {
		e.Lines(s.writes, s.n, s.writesWhat)
	}
	e.Cut(edit.Line("frame_locked++;"), 1, "the lock it took")
	e.Cut(edit.Line("frame_locked--;"), 1, "the lock it released")
	e.Cut(edit.Line("swap_exists_did_quit = TRUE;"), 1, "its one write")
	e.Cut(edit.Line("did_swapwrite_msg = FALSE;"), 1, "its one write")
	e.Cut(edit.Line("autocmd_nested = ac->nested;"), 1, "its one write")
	e.Cut(edit.Line("oldtitle_outdated = TRUE;"), 1, "its one write")
	e.Cut(edit.Line("deadly_signal = sigarg;"), 1, "the signal number it recorded")
	// mr_patternlen's two writes are a whole if/else, so the test goes with them.
	e.Cut(edit.Line("if (mr_pattern == NULL)", "{", "mr_patternlen = 0;", "}", "else", "{", "mr_patternlen = patlen;", "}"), 1,
		"mr_patternlen's if/else")
	// was_safe is a whole function Body, and that function has two callers.
	e.Lines(`state_no_longer_safe\("(?:ins_typebuf\(\)|key typed)"\);`, 2, "the two calls that declared the state unsafe")
	e.DeleteDefinition("state_no_longer_safe", "state_no_longer_safe, whose body was one write")
	e.Lines(`was_safe = (?:is_safe|FALSE);`, 2, "its remaining writes")
	return e.Done()
}

func init() { phase.Register("whim67", Edit) }
