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

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// exNiCompletions are commands whose handler is ex_ni -- present in the table,
// answering "not implemented" -- so completing their arguments is work for an
// answer nobody gets.
var exNiCompletions = []string{"colorscheme", "compiler", "ownsyntax", "setfiletype", "packadd"}

// runtimeContexts are the EXPAND_ contexts that named a file under
// 'runtimepath'.  There is no runtime directory in this build.
var runtimeContexts = []string{"COLORS", "COMPILER", "OWNSYNTAX", "FILETYPE", "PACKADD", "RUNTIME"}

// Whim18 takes 'shellredir' choosing itself by the shell's name, the shell
// quoting, and every completion that read the runtime directory.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("noshellrtp", text, w)

	// 'shellredir''s default was chosen by the name of 'shell'.
	e.InFunction("set_init_3", func(e *edit.E) {
		e.Cut(edit.Line(`idx_srr = findoption((char_u *)"srr");`), 1,
			"set_init_3 looking up 'shellredir'")
		e.FoldNever(edit.Head("if (idx_srr < 0)"), 1,
			"set_init_3 without a 'shellredir' row")
		e.Cut(edit.Line("do_srr = !(options[idx_srr].flags & P_WAS_SET);"), 1,
			"set_init_3 asking whether 'shellredir' was set")
		e.Cut(edit.Line("p = get_isolated_shell_name();"), 1,
			"set_init_3 naming the shell")
		e.DropIf(edit.Head("if (p != nullptr)"), 1,
			"set_init_3 choosing 'shellredir' by shell")
	})
	// do_bang's 'shellquote' went with :! and :read and :write's filters (D2, D4)
	e.InFunction("vim_strsave_fnameescape", func(e *edit.E) {
		e.DropIf(edit.Head("if (what == VSE_SHELL && csh_like_shell() && p != nullptr)"), 1,
			"filename escaping doubling ! for csh")
	})

	// Completion for commands that are ex_ni, and for :set ft=.
	e.InFunction("set_context_by_cmdname", func(e *edit.E) {
		for _, c := range exNiCompletions {
			e.Cut(fmt.Sprintf(`(?m)^[ \t]*case CMD_%s:\n[ \t]*xp->xp_context = EXPAND_\w+;\n[ \t]*xp->xp_pattern = arg;\n[ \t]*break;\n`, c),
				1, fmt.Sprintf("completing :%s, which is ex_ni", c))
		}
		e.Cut(edit.Line("case CMD_runtime:", "set_context_in_runtime_cmd(xp, arg);", "break;"),
			1, "completing :runtime, which is ex_ni")
	})
	e.InFunction("ExpandFromContext", func(e *edit.E) {
		for _, c := range runtimeContexts {
			e.FoldNever(fmt.Sprintf(edit.Head("if (xp->xp_context == EXPAND_%s)"), c), 1,
				fmt.Sprintf("expanding runtime names for EXPAND_%s", c))
		}
	})
	e.InFunction("set_context_in_set_cmd", func(e *edit.E) {
		e.DropIf(edit.Head("if (options[opt_idx].var == (char_u *)&p_ft)"), 1,
			":set ft= completing runtime file types")
		// at phase 2 (the reform's D6) every option the test names is dropped,
		// and only nomemfile has taken its term yet: the whole test folds
		e.FoldNever(edit.Head("if (p == (char_u *)&p_bdir || p == (char_u *)&p_path || p == (char_u *)&p_pp || p == (char_u *)&p_rtp || p == (char_u *)&p_cdpath)"), 1,
			"'backupdir', 'path', 'packpath', 'runtimepath' and 'cdpath' completing as directories")
	})
	e.InFunction("stropt_get_newval", func(e *edit.E) {
		e.FoldNever(edit.Head("if (varp == (char_u *)&p_kp && (*arg == NUL || *arg == ' '))"), 1,
			":set kp= defaulting to :help")
	})

	return e.Done()
}

// whim18kp, get_varp()'s per-buffer resolution of 'keywordprg', a second
// entry that ran in phase 18 before its droplocal, is gone: since B1a
// (doc/GRAPH-MIGRATION.md) droplocal's own get_varp rule takes the case,
// its address spelled without the parentheses (internal/cut/droplocal.go).

func init() {
	phase.Register("whim18", Edit)
}
