package p060

// Whim phase 60 -- no suffix, case, delay, verbose-file, debug or filter-program options.  See GOAL.md.
//
// Seven options whose default is the only value anything could still act on:
//
// 'suffixes'           ordered wildcard matches, and wildcards have not expanded
// since Phase 7 removed globbing; match_suffix() goes
// 'fileignorecase'     off: its five tests fold as false
// 'autocompletedelay'  0: inchar_loop()'s delay is never pending
// 'verbosefile'        empty: the file is never opened, so redir_write(),
// redirecting() and the verbose_enter/leave family fold
// 'debug'              empty: emsg_not_now(), emsg_core() and vim_beep() fold
// 'formatprg'          gq through an external program: it only built a
// 'equalprg'           :{range}!prg line, and :! has been ex_ni since Phase 44.
// gq and = always take the internal path now.
//
// THE DELTA: none the harnesses record.  The probes check the seven are unknown
// and that gq still formats.
// get_varp()'s "local if set" case for 'equalprg' is written &curbuf->b_p_ep, without
// the parentheses droplocal.py matches -- the same gap as 'keywordprg' in Phase 56.

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// Whim60 takes six options that share nothing but being unreachable: 'suffixes',
// 'fileignorecase', 'autocompletedelay', 'verbosefile', 'debug', and the pair
// 'formatprg'/'equalprg'.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nosixopts", text, w)

	// 'suffixes'
	e.InFunction("ExpandOne_start", func(e *edit.E) {
		e.Cut(edit.Line("for (i = 0; i < 2; ++i)", "{", "if (match_suffix(xp->xp_files[i]))", "{", "++non_suf_match;", "}", "}"),
			1, "a single match chosen by 'suffixes'")
	})
	e.InFunction("expand_wildcards", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(\*num_files > 1 && !got_int\)$`, 1, "matches reordered by 'suffixes'")
	})

	// 'fileignorecase'
	e.Literal("regmatch.rm_ic = p_fic;", "regmatch.rm_ic = FALSE;", 2,
		"file patterns ignoring case by 'fileignorecase'")
	e.InFunction("fname_match", func(e *edit.E) {
		e.Literal("rmp->rm_ic = p_fic || ignore_case;", "rmp->rm_ic = ignore_case;", 1,
			"buffer names ignoring case by 'fileignorecase'")
	})
	e.InFunction("vim_fnamecmp", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(p_fic\)$`, 1, "vim_fnamecmp ignoring case")
	})
	e.InFunction("vim_fnamencmp", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(p_fic\)$`, 1, "vim_fnamencmp ignoring case")
	})

	// 'autocompletedelay'
	e.InFunction("inchar_loop", func(e *edit.E) {
		e.Literal(" && !delay_pending", "", 1, "blocking without waiting on the delay")
		e.FoldNever(`(?m)^[ \t]*else if \(delay_pending\)$`, 1, "waiting out the autocomplete delay")
		e.DropIf(`(?m)^[ \t]*if \(delay_pending && acl_elapsed >= p_acl && maxlen >= 3 && !typebuf_changed\(tb_change_cnt\)\)$`, 1,
			"the autocomplete delay expiring")
	})

	// 'verbosefile'
	e.InFunction("redir_write", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(\*p_vfile != NUL && verbose_fd == NULL\)$`, 1,
			"opening 'verbosefile' on first write")
		e.FoldNever(`(?m)^[ \t]*if \(verbose_fd != NULL\)$`, 2, "writing to 'verbosefile'")
	})
	e.InFunction("redirecting", func(e *edit.E) {
		e.Sub(`return redir_fd != NULL \|\| \*p_vfile != NUL\s*;`, "return redir_fd != NULL;", 1,
			"redirecting to 'verbosefile'")
	})
	e.InFunction("verbose_enter", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(\*p_vfile != NUL\)$`, 1, "verbose_enter silencing for 'verbosefile'")
	})
	e.InFunction("verbose_leave", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(\*p_vfile != NUL\)$`, 1, "verbose_leave silencing for 'verbosefile'")
	})
	e.Sub(`(?m)^[ \t]*verbose_(?:enter|leave)\(\);\n`, "", 4,
		"calls to the emptied verbose_enter and verbose_leave")
	e.InFunction("verbose_enter_scroll", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(\*p_vfile != NUL\)$`, 1, "verbose_enter_scroll silencing for 'verbosefile'")
	})
	e.InFunction("verbose_leave_scroll", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(\*p_vfile != NUL\)$`, 1, "verbose_leave_scroll silencing for 'verbosefile'")
	})

	// 'debug'
	e.InFunction("emsg_not_now", func(e *edit.E) {
		e.Literal("(emsg_off > 0 && vim_strchr(p_debug, 'm') == NULL && vim_strchr(p_debug, 't') == NULL)",
			"(emsg_off > 0)", 1, "'debug' m and t showing suppressed errors")
	})
	e.InFunction("emsg_core", func(e *edit.E) {
		e.Literal("if (!emsg_off || vim_strchr(p_debug, 't') != NULL)", "if (!emsg_off)", 1,
			"'debug' t handling errors under emsg_off")
	})
	e.InFunction("vim_beep", func(e *edit.E) {
		e.DropIf(`(?m)^[ \t]*if \(vim_strchr\(p_debug, 'e'\) != NULL\)$`, 1, "'debug' e showing Beep!")
	})

	// 'formatprg' and 'equalprg'
	e.InFunction("do_pending_operator", func(e *edit.E) {
		e.Literal("if (oap->op_type == OP_INDENT && *get_equalprg() == NUL)", "if (oap->op_type == OP_INDENT)", 1,
			"= through 'equalprg'")
		e.FoldNever(`(?m)^[ \t]*if \(\*p_fp != NUL \|\| \*curbuf->b_p_fp != NUL\)$`, 1, "gq through 'formatprg'")
	})
	e.InFunction("op_colon", func(e *edit.E) {
		e.FoldNever(`(?m)^[ \t]*if \(oap->op_type == OP_INDENT\)$`, 1, "op_colon building an 'equalprg' filter")
		e.FoldNever(`(?m)^[ \t]*if \(oap->op_type == OP_FORMAT\)$`, 1, "op_colon building a 'formatprg' filter")
	})
	return e.Done()
}

// Whim60EP takes get_varp()'s per-buffer resolution of 'equalprg'.  A second
// entry for whim56kp's reason: it stands after a dropoptions call in the phase
// program, and folding it in would move the cut.
//
// The report line is the heredoc's last statement and it prints AFTER the
// write, which is why a filtered read of the file missed it and the port was
// briefly silent.  editcmp caught it in both directions -- first that the
// message existed, then that removing it was wrong -- which is the whole reason
// the report is compared and not only the tree.
func Whim60EP(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("nosixopts", text, w)
	e.Cut(`(?m)^[ \t]*case[^\n]*\bBV_EP\b[^\n]*\n[ \t]*return \*curbuf->b_p_ep != NUL \? \(char_u \*\)&curbuf->b_p_ep : p->var;\n`,
		1, "get_varp no longer resolves 'equalprg' per buffer")
	return e.Done()
}

func init() {
	edit.Register("whim60", Edit)
	edit.Register("whim60ep", Whim60EP)
}
