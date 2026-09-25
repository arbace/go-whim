package p094

// Whim phase 94 -- `:q` quits, and `ZZ` is `ZQ`.  See GOAL.md.
//
// Phases 89 to 93 took every way to reach a file.  What is left of the filesystem in
// this editor is a REFUSAL: `:q` on a modified buffer answers `E37: No write since
// last change (add ! to override)` and stays.  The protection has no remedy once
// nothing can be written -- there is no `:w` to answer it with and no file the text
// could have come from -- so it is a door that opens onto nothing, and this phase
// takes it.  `:q`, `:q!`, `ZZ` and `ZQ` become one thing.
//
// ONE ANCHOR, AND THE PHASE IS THAT FOLD.  `ex_quit()` is
//
// if ((check_changed(...)) || (check_changed_any(...))) { not_exiting(...); }
// else                                                  { getout(0); ... }
//
// and the test is the refusal.  Folding it NEVER keeps the `else` -- "quit" -- and
// is the last reference `check_changed()` has.  FIFTEEN FUNCTIONS THEN GO AND THIS
// FILE NAMES NOT ONE OF THEM (GOALS.md core rule 1), which is the largest surprise
// the phase has: eleven of the fifteen are not the refusal at all.
// `check_changed_any()`'s tail is "go to the buffer that refused" -- it calls
// `set_curbuf()`, which calls `enter_buffer()` and `win_enter_ext()` -- and after
// whim removed the buffer list and the window commands, THAT TAIL WAS THE LAST
// CALLER OF THE WHOLE SWITCH-BUFFER/SWITCH-WINDOW ISLAND.  THE ISLAND IS A GRAPH AND
// NOT A FAN: only `add_bufnum`, `set_curbuf` and `goto_tabpage_win` are called by
// `check_changed_any` itself and the other eight hang off those, so what the edit
// computes before it folds anything is that every call to any of the eleven is inside
// `check_changed_any` or inside another of the eleven.  After this phase the editor
// has no code for entering a different buffer or a different window at all.
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
// TWO EXTRAS GO WITH THE FOLD, each measured byte-identical in the recording.
//
// A  TWO STRUCT FIELDS THAT BECOME WRITE-ONLY, WHICH NO TOOL CAN SEE.  This is
// phase 90's `usefilter` judgement in a smaller shape: tools/deadfields.py
// removes a field nothing NAMES, and gcc has no warning for a member that is
// only written.  `win_T.w_topline_was_set`'s only reader was in
// `enter_buffer()` and `wininfo_S.wi_changelistidx`'s only reader was in
// `get_winopts()`, and the sweep takes both functions.  THE TEXT THIS LEAVES
// DOES NOT COMPILE -- two mentions survive inside functions the sweep is about
// to take -- exactly as internal/phase/090/edit.go says of its own, and that is stated
// here rather than discovered by whoever runs the edit alone.
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
// `case 'Z'` AND for `case 'Q'`, identical since phase 89.  After this phase `:q` and
// `:q!` are also identical, so all four spellings are one thing.  THE STRINGS ARE
// NOT REWRITTEN TO `"q"`: it would move `zz_key` and `zq_key` for no gain, and
// `case:zz_key` is phase 89's declaration and must not be re-declared here.
//
// WHAT LEAVES FOR A LATER PHASE TO NOTICE.  `SHM_FILEINFO` is the `'shortmess'` `f`
// letter and its only reader was inside `enter_buffer()`; the sweep takes it, and
// the letter is inert afterwards.  That is the options phase's and the flag strings
// are not touched here.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, as every Part II edit since phase 85 does, and the source goes with it as
// $state/old.c.  The check needs both: the exit status of a session that quits with
// unsaved changes is what moves, and no recording can see it.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim94", Edit) }

// w94Swept are the twelve the sweep reads Out of check_changed_any's tail, and
// they are mostly three mentions each -- a prototype, a definition and one call.
// THREE OF THEM ARE NOT, which is what a counted anchor is for, and those three
// are named in w94Before instead.
var w94Swept = []string{"no_write_message", "add_bufnum", "set_curbuf", "enter_buffer",
	"win_enter", "win_enter_ext", "goto_tabpage_win", "goto_tabpage_tp", "get_winopts",
	"find_wininfo", "buflist_findfpos", "buflist_getfpos"}

// w94Island is the switch-buffer/switch-window island that hangs off that tail.
// IT IS A GRAPH AND NOT A FAN: only add_bufnum, set_curbuf and goto_tabpage_win
// are called by check_changed_any itself and the other eight hang off those, so
// a check requiring all eleven to be its callees would fail on a correct phase.
var w94Island = []string{"add_bufnum", "set_curbuf", "enter_buffer", "win_enter",
	"win_enter_ext", "goto_tabpage_win", "goto_tabpage_tp", "get_winopts",
	"find_wininfo", "buflist_findfpos", "buflist_getfpos"}

var w94Before = map[string]int{
	"check_changed": 4, "not_exiting": 4,
	"check_changed_any": 2, "no_write_message_nobang": 2,
	"w_topline_was_set": 4, "wi_changelistidx": 3, "SHM_FILEINFO": 2,
	"bufIsChanged": 10, "curbufIsChanged": 7, "bufIsChangedNotTerm": 3,
	"exiting": 17, "buf_spname": 5, "open_buffer": 5,
	"curbuf_locked": 7, "text_locked": 6, "before_quit_autocmds": 2,
	"p_ro": 2, "p_ur": 2, "read_cmd_fd": 12,
	"vim_fsync": 3, "scriptin": 8, "redir_fd": 6,
}

var w94After = map[string]int{
	"check_changed": 3, "not_exiting": 2,
	"w_topline_was_set": 2, "wi_changelistidx": 1,
	"bufIsChanged": 10, "curbufIsChanged": 7, "bufIsChangedNotTerm": 3,
	"curbuf_locked": 7, "text_locked": 6, "before_quit_autocmds": 2,
	"p_ro": 2, "p_ur": 2, "read_cmd_fd": 12,
	"vim_fsync": 3, "scriptin": 8, "redir_fd": 6,
}

func init() {
	for _, n := range w94Swept {
		w94Before[n] = 3
	}
}

// Whim94 takes the last thing the filesystem left behind: the refusal,
// `E37: No write since last change`, which has had no remedy to offer since
// phase 89 took every `:write`.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "noquit", W: w}
	var err error

	mentions := func(t []byte, name string) int {
		return len(regexp.MustCompile(`\b`+name+`\b`).FindAll(t, -1))
	}
	textEdit := func(t []byte, old, new, what string, n int) ([]byte, error) {
		// exact first, then modulo whitespace: a fold earlier in this phase keeps a
		// body at its old indentation, and the canonical print re-indents it
		k := cutil.CountAnchorB(t, old)
		if k != n {
			return nil, p.Die("%s -- the text occurs %d times, expected %d: %s",
				what, k, n, cutil.PyRepr(edit.CoreHead(old, 70)))
		}
		p.Say(what)
		return cutil.ReplaceAnchorB(t, old, []byte(new), n), nil
	}

	// ---- 0. the shape every anchor below was counted against ------------------
	for _, name := range edit.SortedKeys(w94Before) {
		if k := mentions(text, name); k != w94Before[name] {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w94Before[name])
		}
	}
	p.Say("check_changed 4, not_exiting 4, check_changed_any 2 and the twelve the " +
		"sweep reads from it -- the file the one fold was counted against")

	// THE INVARIANT, COMPUTED BEFORE ANYTHING IS FOLDED: every call to any of the
	// eleven is inside check_changed_any or inside another of the eleven, so the
	// whole island is reachable from that one tail and from nowhere else.
	blanked := cutil.Blank(text)
	spans := map[string][2]int{}
	for _, name := range append(append([]string{}, w94Island...), "check_changed_any") {
		a, z, ok := cutil.FindDefinition(text, blanked, name)
		if !ok {
			return nil, p.Die("%s is not defined", name)
		}
		spans[name] = [2]int{a, z}
	}
	for _, name := range w94Island {
		own := spans[name]
		proto := regexp.MustCompile(`^static\b.*\b` + name + `\(.*\);$`)
		calls, stray := 0, []int{}
		for _, m := range regexp.MustCompile(`\b`+name+`\b`).FindAllIndex(text, -1) {
			if own[0] <= m[0] && m[0] < own[1] {
				continue // inside its own definition
			}
			s := strings.LastIndex(string(text[:m[0]]), "\n") + 1
			e := strings.Index(string(text[m[0]:]), "\n") + m[0]
			if proto.MatchString(strings.TrimSpace(string(text[s:e]))) {
				continue // its prototype
			}
			calls++
			in := false
			for _, sp := range spans {
				if sp[0] <= m[0] && m[0] < sp[1] {
					in = true
					break
				}
			}
			if !in {
				stray = append(stray, strings.Count(string(text[:m[0]]), "\n")+1)
			}
		}
		if calls == 0 || len(stray) > 0 {
			first := "-"
			if len(stray) > 0 {
				first = strconv.Itoa(stray[0])
			}
			return nil, p.Die("%s has %d call site(s) and %d of them are outside check_changed_any "+
				"and the island (first at line %s); the switch-buffer island does not "+
				"hang off that one tail after all", name, calls, len(stray), first)
		}
	}
	p.Say("every call to add_bufnum, set_curbuf, enter_buffer, win_enter, win_enter_ext, " +
		"goto_tabpage_win, goto_tabpage_tp, get_winopts, find_wininfo, buflist_findfpos " +
		"and buflist_getfpos is inside check_changed_any or inside another of the " +
		"eleven -- its tail, \"go to the buffer that refused\", is the last caller of the " +
		"whole switch-buffer/switch-window island, and that, and nothing weaker, is why " +
		"the one fold below takes eleven functions nobody would predict")

	// ---- 1. the anchor: the refusal folds never -------------------------------
	a, z, ok := cutil.FindDefinition(text, cutil.Blank(text), "ex_quit")
	if !ok {
		return nil, p.Die("ex_quit is not defined")
	}
	what := "ex_quit: the refusal folds NEVER, so `:q` takes the else arm and quits -- " +
		"this one fold is the phase, and it is check_changed()'s last reference"
	Body, err := cutil.FoldNever(text[a:z],
		`(?m)^    if \(\(check_changed\(wp->w_buffer, \(eap->forceit \? CCGD_FORCEIT : 0\) \| CCGD_EXCMD\)\) \|\| \(check_changed_any\(eap->forceit, TRUE\)\)\)$`, 1)
	if err != nil {
		return nil, p.Die("%s -- %v", what, err)
	}
	text = []byte(string(text[:a]) + string(Body) + string(text[z:]))
	p.Say(what)

	// ---- 2. extra B: the tail that cannot run ---------------------------------
	if text, err = textEdit(text, w94lit1, w94lit2,
		"ex_quit's tail: getout() sets `exiting = TRUE` itself and ends in "+
			"mch_exit(), which never returns, so the save, the set and the "+
			"restore are dead -- and not_exiting(), which was the whole of \"we "+
			"changed our mind, put the terminal back\", has no caller left", 1); err != nil {
		return nil, err
	}
	if k := mentions(text, "not_exiting"); k != 2 {
		return nil, p.Die("not_exiting has %d mentions, expected 2 -- its prototype and its "+
			"definition, for the sweep", k)
	}

	// ---- 3. extra A: two fields that become write-only ------------------------
	// NOTHING SEES EITHER OF THESE.  deadfields.py removes a field nothing NAMES,
	// and a field that is only written is still named; gcc has no warning for one.
	for _, fr := range []struct{ fld, reader string }{
		{"w_topline_was_set", "enter_buffer"}, {"wi_changelistidx", "get_winopts"},
	} {
		a, z, ok := cutil.FindDefinition(text, cutil.Blank(text), fr.reader)
		if !ok || !strings.Contains(string(text[a:z]), fr.fld) {
			return nil, p.Die("%s is not named inside %s, and the sweep taking that function is the "+
				"whole reason this field becomes write-only", fr.fld, fr.reader)
		}
	}
	for _, e := range []struct{ Old, What string }{
		{w94lit3, "win_T.w_topline_was_set: its only reader is inside enter_buffer(), " +
			"which the sweep takes -- so the field is write-only and nothing sees it"},
		{w94lit4, "and its one surviving write, in set_topline()"},
		{w94lit5, "wininfo_S.wi_changelistidx: its only reader is inside get_winopts(), " +
			"swept with the rest of the island"},
		{w94lit6, "and its one surviving write, in find_wininfo() -- which the sweep " +
			"takes too, so this line is removed for what it says and not for what " +
			"it costs"},
	} {
		if text, err = textEdit(text, e.Old, "", e.What, 1); err != nil {
			return nil, err
		}
	}

	// ---- what the sweep is handed, as a count rather than as trust ------------
	for _, name := range edit.SortedKeys(w94After) {
		if k := mentions(text, name); k != w94After[name] {
			return nil, p.Die("%s has %d mentions after the cut, expected %d", name, k, w94After[name])
		}
	}
	p.Say("the cut is done: check_changed 3 -- its prototype, its definition and the " +
		"one call inside check_changed_any, which is where the sweep starts -- " +
		"not_exiting 2, and read_cmd_fd 12, vim_fsync 3, scriptin 8 and redir_fd 6 " +
		"untouched, each of them a later phase's")
	return text, nil
}
