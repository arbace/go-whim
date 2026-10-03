package p019

// Whim phase 19 (formerly 60) -- no suffix, case, delay, verbose-file, debug or filter-program options.  See GOAL.md.
//
// Seven options whose default is the only value anything could still act on:
//
// 'suffixes'           ordered wildcard matches, and wildcards have not expanded
// since Phase 5 removed globbing; match_suffix() goes
// 'fileignorecase'     off: its five tests fold as false
// 'autocompletedelay'  0: inchar_loop()'s delay is never pending
// 'verbosefile'        empty: the file is never opened, so redir_write(),
// redirecting() and the verbose_enter/leave family fold
// 'debug'              empty: emsg_not_now(), emsg_core() and vim_beep() fold
// 'formatprg'          gq through an external program: it only built a
// 'equalprg'           :{range}!prg line, and :! has been ex_ni since Phase 15a.
// gq and = always take the internal path now.
//
// THE DELTA: none the harnesses record.  The probes check the seven are unknown
// and that gq still formats.
// get_varp()'s "local if set" case for 'equalprg' is written &curbuf->b_p_ep, without
// the parentheses droplocal.py matches -- the same gap as 'keywordprg' in Phase 18.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// Whim19 takes six options that share nothing but being unreachable: 'suffixes',
// 'fileignorecase', 'autocompletedelay', 'verbosefile', 'debug', and the pair
// 'formatprg'/'equalprg'.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the same acts in the same
// order, each found by its C-lisp form and counted, its report the text
// version's (history keeps it): a substring dropped from a condition is
// DropOperand, a whole expression replaced is Rewrite.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nosixopts", e, w)

	// 'suffixes'
	v.InFunction("ExpandOne_start", func(v *graph.Verbs) {
		v.Cut("(for (= i 0) (< i 2) (pre++ i) (block (if (call match_suffix _) _)))", 1,
			"a single match chosen by 'suffixes'")
	})
	v.InFunction("expand_wildcards", func(v *graph.Verbs) {
		v.DropIf("(&& (> (deref num_files) 1) (! got_int))", 1, "matches reordered by 'suffixes'")
	})

	// 'fileignorecase': match_file_pat()'s went with the autocommands at
	// phase 5 (whim5d, phase 5d's program, which runs before this phase now);
	// the other died with the command retired at phase 1 that reached it
	// (exfront, the reform's D2)
	v.InFunction("fname_match", func(v *graph.Verbs) {
		v.DropOperand("p_fic", 1, "buffer names ignoring case by 'fileignorecase'")
	})
	v.InFunction("vim_fnamecmp", func(v *graph.Verbs) {
		v.DropIf("p_fic", 1, "vim_fnamecmp ignoring case")
	})
	v.InFunction("vim_fnamencmp", func(v *graph.Verbs) {
		v.DropIf("p_fic", 1, "vim_fnamencmp ignoring case")
	})

	// 'autocompletedelay'
	v.InFunction("inchar_loop", func(v *graph.Verbs) {
		v.DropOperand("(! delay_pending)", 1, "blocking without waiting on the delay")
		v.FoldNever("delay_pending", 1, "waiting out the autocomplete delay")
		v.DropIf("(&& delay_pending (>= acl_elapsed p_acl) (>= maxlen 3) (! (call typebuf_changed tb_change_cnt)))", 1,
			"the autocomplete delay expiring")
	})

	// 'verbosefile'
	v.InFunction("redir_write", func(v *graph.Verbs) {
		v.DropIf("(&& (!= (deref p_vfile) NUL) (== verbose_fd nullptr))", 1,
			"opening 'verbosefile' on first write")
		v.FoldNever("(!= verbose_fd nullptr)", 2, "writing to 'verbosefile'")
	})
	v.InFunction("redirecting", func(v *graph.Verbs) {
		// redir_fd's test is gone already: only :redir wrote it, and the
		// fall-out closure folded it at phase 1 (exfront, the reform's D2)
		v.Rewrite("(return (!= (deref p_vfile) NUL))", "(return FALSE)", 1,
			"redirecting to 'verbosefile'")
	})
	// verbose_enter() and verbose_leave(), and their four calls, went with
	// the autocommands' verbose messages at phase 5 (whim5d, phase 5d's
	// program, which runs before this phase now)
	v.InFunction("verbose_enter_scroll", func(v *graph.Verbs) {
		v.FoldNever("(!= (deref p_vfile) NUL)", 1, "verbose_enter_scroll silencing for 'verbosefile'")
	})
	v.InFunction("verbose_leave_scroll", func(v *graph.Verbs) {
		v.FoldNever("(!= (deref p_vfile) NUL)", 1, "verbose_leave_scroll silencing for 'verbosefile'")
	})

	// 'debug'
	v.InFunction("emsg_not_now", func(v *graph.Verbs) {
		v.Rewrite("(&& ?off (== (call vim_strchr p_debug 'm') nullptr) (== (call vim_strchr p_debug 't') nullptr))",
			"?off", 1, "'debug' m and t showing suppressed errors")
	})
	v.InFunction("emsg_core", func(v *graph.Verbs) {
		v.DropOperand("(!= (call vim_strchr p_debug 't') nullptr)", 1,
			"'debug' t handling errors under emsg_off")
	})
	v.InFunction("vim_beep", func(v *graph.Verbs) {
		v.DropIf("(!= (call vim_strchr p_debug 'e') nullptr)", 1, "'debug' e showing Beep!")
	})

	// 'formatprg' and 'equalprg': gq through 'formatprg' and = through
	// 'equalprg' went with the format operator's case and the filter and
	// indent dispatch at phase 5 (whim5a, phase 5a's program, which runs
	// before this phase now)
	v.InFunction("op_colon", func(v *graph.Verbs) {
		v.FoldNever("(== (-> oap op_type) OP_INDENT)", 1, "op_colon building an 'equalprg' filter")
		v.FoldNever("(== (-> oap op_type) OP_FORMAT)", 1, "op_colon building a 'formatprg' filter")
	})
	return v.Done()
}

// whim19ep, get_varp()'s per-buffer resolution of 'equalprg', a second
// entry that ran between the phase's sweep and its droplocal, is gone:
// since B1a (doc/GRAPH-MIGRATION.md) droplocal's own get_varp rule takes
// the case, its address spelled without the parentheses
// (internal/cut/droplocal.go).

func init() {
	phase.RegisterGraph("whim19", Edit)
}
