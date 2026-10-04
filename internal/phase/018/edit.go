package p018

// Whim phase 18 (formerly 56) -- no shell, runtime or keyword-program options.  See GOAL.md.
//
// Six options whose readers survive only in machinery with nothing to serve:
//
// 'shell', 'shellquote', 'shellredir'  no shell is ever run -- call_shell() and
// mch_call_shell() went long ago.  'shell' only chose the default of
// 'shellredir' in set_init_3() and whether filename escaping doubled a `!`
// for csh; 'shellquote' only wrapped do_bang()'s command line.
// 'runtimepath', 'packpath'  there is no runtime to find.  Their readers are the
// completion of :colorscheme, :compiler, :ownsyntax, :setfiletype, :packadd
// and :runtime -- every one of them ex_ni -- and of :set ft=, which listed
// runtime syntax/indent/ftplugin names.
// 'keywordprg'  K is gone; only :set kp= defaulting to :help read it.
//
// THE DELTA: none the harnesses record.  The probes check the six are unknown.
// No sweep here.  One stood here, and the lines after it were written for swept text,
// but this phase and every stage it has run in reproduce their boundaries without
// it (GOALS.md, *The inner sweeps*; internal/phase/STAGES.md) -- the stage's one sweep does its work.
// get_varp()'s "local if set" case for 'keywordprg' is written &curbuf->b_p_kp,
// without the parentheses droplocal.py's pattern expects, so its two mentions
// would read as readers.  It is plumbing, and goes by hand first.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// exNiCompletions are commands whose handler is ex_ni -- present in the table,
// answering "not implemented" -- so completing their arguments is work for an
// answer nobody gets.
var exNiCompletions = []string{"colorscheme", "compiler", "ownsyntax", "setfiletype", "packadd"}

// runtimeContexts are the EXPAND_ contexts that named a file under
// 'runtimepath'.  There is no runtime directory in this build.
var runtimeContexts = []string{"COLORS", "COMPILER", "OWNSYNTAX", "FILETYPE", "PACKADD", "RUNTIME"}

// Edit takes 'shellredir' choosing itself by the shell's name, the shell
// quoting, and every completion that read the runtime directory.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's line cuts are Cuts
// and CutRuns by form, its head folds FoldNever and DropIf on the
// conditions (history keeps the text version).
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noshellrtp", e, w)

	// 'shellredir''s default was chosen by the name of 'shell'.
	v.InFunction("set_init_3", func(v *graph.Verbs) {
		v.Cut(`(= idx_srr (call findoption (cast (ptr char_u) "srr")))`, 1,
			"set_init_3 looking up 'shellredir'")
		v.FoldNever("(< idx_srr 0)", 1, "set_init_3 without a 'shellredir' row")
		v.Cut("(= do_srr (! (& (. (index options idx_srr) flags) P_WAS_SET)))", 1,
			"set_init_3 asking whether 'shellredir' was set")
		v.Cut("(= p (call get_isolated_shell_name))", 1, "set_init_3 naming the shell")
		v.DropIf("(!= p nullptr)", 1, "set_init_3 choosing 'shellredir' by shell")
	})
	// do_bang's 'shellquote' went with :! and :read and :write's filters (D2, D4)
	v.InFunction("vim_strsave_fnameescape", func(v *graph.Verbs) {
		v.DropIf("(&& (== what VSE_SHELL) (call csh_like_shell) (!= p nullptr))", 1,
			"filename escaping doubling ! for csh")
	})

	// Completion for commands that are ex_ni, and for :set ft=.
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		for _, c := range exNiCompletions {
			v.CutRun(fmt.Sprintf("completing :%s, which is ex_ni", c),
				fmt.Sprintf("(case CMD_%s)", c), "(= (-> xp xp_context) _)",
				"(= (-> xp xp_pattern) arg)", "(break)")
		}
		v.CutRun("completing :runtime, which is ex_ni", "(case CMD_runtime)",
			"(call set_context_in_runtime_cmd xp arg)", "(break)")
	})
	v.InFunction("ExpandFromContext", func(v *graph.Verbs) {
		for _, c := range runtimeContexts {
			v.FoldNever(fmt.Sprintf("(== (-> xp xp_context) EXPAND_%s)", c), 1,
				fmt.Sprintf("expanding runtime names for EXPAND_%s", c))
		}
	})
	v.InFunction("set_context_in_set_cmd", func(v *graph.Verbs) {
		v.DropIf("(== (. (index options opt_idx) var) (cast (ptr char_u) (addr p_ft)))", 1,
			":set ft= completing runtime file types")
		// at phase 2 (the reform's D6) every option the test names is dropped,
		// and only nomemfile has taken its term yet: the whole test folds
		v.FoldNever("(|| (== p (cast (ptr char_u) (addr p_bdir))) (== p (cast (ptr char_u) (addr p_path))) "+
			"(== p (cast (ptr char_u) (addr p_pp))) (== p (cast (ptr char_u) (addr p_rtp))) "+
			"(== p (cast (ptr char_u) (addr p_cdpath))))", 1,
			"'backupdir', 'path', 'packpath', 'runtimepath' and 'cdpath' completing as directories")
	})
	v.InFunction("stropt_get_newval", func(v *graph.Verbs) {
		v.FoldNever("(&& (== varp (cast (ptr char_u) (addr p_kp))) (|| (== (deref arg) NUL) (== (deref arg) ' ')))", 1,
			":set kp= defaulting to :help")
	})
	return v.Done()
}

// whim18kp, get_varp()'s per-buffer resolution of 'keywordprg', a second
// entry that ran in phase 18 before its droplocal, is gone: since B1a
// (doc/GRAPH-MIGRATION.md) droplocal's own get_varp rule takes the case,
// its address spelled without the parentheses (internal/cut/droplocal.go).

func init() {
	phase.RegisterGraph("whim18", Edit)
}
