package p059

// Whim phase 59 -- no command-line completion.  See GOAL.md.
//
// The command line no longer completes anything.  In getcmdline_int() the
// 'wildchar' and 'wildcharm' keys, S-Tab, CTRL-D (list), CTRL-A (insert all),
// CTRL-L (longest match) and CTRL-N/CTRL-P over matches go; each of those keys is
// now an ordinary character, and CTRL-N/CTRL-P browse history as they did with no
// matches.  CTRL-L still adds a character to an incremental search.
//
// What completion shared with filename expansion stays: expand_filename() ->
// ExpandOne() with EXPAND_FILES, and the argument list through expand_wildcards().  So ExpandFromContext() keeps its file branch and loses the
// rest -- options, mappings, buffers, highlight groups, ++opt, every command's
// argument completion -- and ExpandOne() keeps the one mode its last caller asks
// for.  The sweep takes set_one_cmd_context() and everything under it.
//
// The six wild* options go: 'wildchar', 'wildcharm', 'wildmode', 'wildoptions',
// 'wildignore' and 'wildignorecase'.  The last two were read by globbing too, and
// fold as empty and off.
//
// THE DELTA: none the harnesses record.  The probes check the options are unknown
// and that `:e` still edits a named file.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func kex(k string) string {
	return fmt.Sprintf(`\(-\(\(KS_EXTRA\) \+ \(\(int\)\(%s\) << 8\)\)\)`, k)
}

const valueCompletion = `\bexpand_set_\w+(\s*,)`

// Whim59 takes command-line completion: the wildcard machinery in
// getcmdline_int, every options[] row's value-completion callback, and every
// context ExpandFromContext knew but files.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nocompletion", text, w)

	e.InFunction("getcmdline_int", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(ccline\.cmdbuff_replaced && xpc\.xp_numfiles > 0\)$`, 1,
			"a replaced command line freeing its matches")
		e.DropIf(fmt.Sprintf(`(?m)^[ \t]*if \(c == [ \t]*%s[ \t]*&& did_hist_navigate\)$`, kex("KE_WILD")), 1,
			"a wildcard trigger after history navigation")
		e.Cut(`(?m)^[ \t]*did_hist_navigate = TRUE;\n`, 1,
			"history navigation remembered for the wildcard trigger")
		e.DropIf(fmt.Sprintf(`(?m)^[ \t]*if \(c != p_wc && c == [ \t]*%s[ \t]*&& xpc\.xp_numfiles > 0\)$`, edit.Key("k", "B")), 1,
			"S-Tab stepping back through matches")
		e.DropIf(`(?m)^[ \t]*if \(\(did_wild_list\) && !key_is_wc && xpc\.xp_numfiles > 0\)$`, 1,
			"CTRL-E and CTRL-Y over a match list")
		e.DropIf(`(?m)^[ \t]*if \(\(c == ESC \|\| c == Ctrl_C\) && \(wim_flags\[0\] & WIM_LIST\)\)$`, 1,
			"leaving a 'wildmode' list clearing 'hlsearch'")
		e.Cut(`(?m)^[ \t]*end_wildmenu = \([^\n]*\);\n`, 1, "deciding the match list ends")
		e.DropIf(`(?m)^[ \t]*if \(end_wildmenu\)$`, 1, "ending the match list")
		e.DropIf(fmt.Sprintf(`(?m)^[ \t]*if \(\(c == p_wc && !gotesc && KeyTyped\) \|\| c == p_wcm \|\| c == [ \t]*%s[ \t]*\)$`, kex("KE_WILD")), 1,
			"completing on 'wildchar', 'wildcharm' or the wildcard trigger")
		e.DropIf(fmt.Sprintf(`(?m)^[ \t]*if \(c == [ \t]*%s[ \t]*&& KeyTyped\)$`, edit.Key("k", "B")), 1,
			"S-Tab completing backwards")
		e.Cut(`(?m)^[ \t]*case Ctrl_D:\n[ \t]*if \(showmatches\(&xpc, TRUE\) == EXPAND_NOTHING\)\n[ \t]*\{\n[ \t]*break;\n[ \t]*\}\n[ \t]*redrawcmd\(\);\n[ \t]*continue;\n`,
			1, "CTRL-D listing matches")
		e.Cut(`(?m)^[ \t]*case Ctrl_A:\n[ \t]*if \(nextwild\(&xpc, WILD_ALL, 0, firstc != '@'\) == FAIL\)\n[ \t]*\{\n[ \t]*break;\n[ \t]*\}\n[ \t]*xpc\.xp_context = EXPAND_NOTHING;\n[ \t]*did_wild_list = FALSE;\n[ \t]*goto cmdline_changed;\n`,
			1, "CTRL-A inserting every match")
		e.Sub(`(?m)^([ \t]*)if \(nextwild\(&xpc, WILD_LONGEST, 0, firstc != '@'\) == FAIL\)\n[ \t]*\{\n[ \t]*break;\n[ \t]*\}\n[ \t]*goto cmdline_changed;\n`,
			"${1}break;\n", 1, "CTRL-L completing the longest match")
		e.DropIf(`(?m)^[ \t]*if \(xpc\.xp_numfiles > 0\)$`, 1, "CTRL-N and CTRL-P stepping through matches")
		e.Cut(`(?m)^[ \t]*did_wild_list = FALSE;\n[ \t]*wim_index = 0;\n`, 1,
			"leaving the command line resetting the match list")
		e.Literal("may_trigger_safestate(xpc.xp_numfiles <= 0);", "may_trigger_safestate(TRUE);", 1,
			"SafeState not waiting on a match list")
		e.Literal("if (xpc.xp_context == EXPAND_NOTHING && (KeyTyped || vpeekc() == NUL))",
			"if (KeyTyped || vpeekc() == NUL)", 1,
			"incremental search not waiting on a completion context")
	})

	// Each options[] row names a callback that completes its value, called only
	// by :set completion, which is gone -- but the table keeps them reachable,
	// and with them ExpandGeneric() and the fuzzy matcher.  The row's callback
	// becomes NULL.  The count is a FLOOR rather than a number: how many rows
	// carry one is a fact about a table other phases are also cutting, and a
	// floor refuses the case this guards against -- a pattern that has stopped
	// matching.
	e.InTable("static struct vimoption options[] =\n", func(e *edit.E) {
		k := len(e.Query(valueCompletion, 0))
		e.Expect(k >= 20, "options[] names %d value-completion callbacks, expected many", k)
		e.Sub(valueCompletion, "NULL$1", k,
			fmt.Sprintf("options[] no longer names a value-completion callback (%d rows)", k))
	})

	e.InFunction("ExpandFromContext", func(e *edit.E) {
		// The non-file contexts are one run, from the empty-match assignment to
		// the function's last `return ret;`.  It is cut by its ends rather than
		// by a pattern over the whole span: the run is hundreds of lines of
		// unrelated cases, and a regular expression that matched all of them
		// would match anything.
		e.Splice("    *matches = (char_u **)\"\";\n", "    return ret;\n", "",
			"every completion context but files")
		e.FoldAlways(`(?m)^[ \t]*if \(xp->xp_context == EXPAND_FILES \|\| xp->xp_context == EXPAND_DIRECTORIES \|\| xp->xp_context == EXPAND_FILES_IN_PATH \|\| xp->xp_context == EXPAND_FINDFUNC \|\| xp->xp_context == EXPAND_DIRS_IN_CDPATH\)$`, 1,
			"file expansion is the only context")
	})

	e.InFunction("ExpandOne", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(mode == WILD_NEXT \|\| mode == WILD_PREV \|\| mode == WILD_PAGEUP \|\| mode == WILD_PAGEDOWN\)$`, 1,
			"ExpandOne stepping through matches")
		e.FoldNever(`(?m)^[ \t]*if \(mode == WILD_CANCEL\)$`, 1, "ExpandOne cancelling or applying a match")
		e.FoldNever(`(?m)^[ \t]*if \(mode == WILD_FREE\)$`, 1, "ExpandOne only freeing")
		e.FoldNever(`(?m)^[ \t]*if \(mode == WILD_LONGEST && xp->xp_numfiles > 0\)$`, 1, "ExpandOne finding the longest match")
		e.FoldNever(`(?m)^[ \t]*if \(mode == WILD_ALL && xp->xp_numfiles > 0 && !got_int\)$`, 1, "ExpandOne joining every match")
		e.FoldAlways(`(?m)^[ \t]*if \(mode == WILD_EXPAND_FREE \|\| mode == WILD_ALL\)$`, 1, "ExpandOne always cleaning up after expanding")
	})

	e.InFunction("didset_options2", func(e *edit.E) {
		e.Cut(`(?m)^[ \t]*check_opt_wim\(\);\n`, 1, "startup parsing 'wildmode' into flags nothing reads")
	})
	e.InFunction("expand_filename", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(p_wic\)$`, 1, "'wildignorecase' in filename globbing")
	})
	e.InFunction("expand_wildcards", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(\*p_wig\)$`, 1, "'wildignore' in filename globbing")
	})
	e.InFunction("do_set_option_numeric", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*else if \(\(\(long \*\)varp == &p_wc \|\| \(long \*\)varp == &p_wcm\)`, 1, ":set wc= accepting a key name")
	})
	e.InFunction("wc_use_keyname", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(\(\(long \*\)varp == &p_wc\) \|\| \(\(long \*\)varp == &p_wcm\)\)$`, 1, ":set wc? showing a key name")
	})
	return e.Done()
}

func init() { edit.Register("whim59", Edit) }
