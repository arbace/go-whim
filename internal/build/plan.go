// Package build is whim's pipeline: slim-vim.c in, whim-vim.c out, in one
// process and in memory.  The driver is generic (crefactor/pipeline);
// what is whim's is here -- this plan, the file names, and how the plan's
// non-literal arguments are resolved (build.go's config).
//
// THE PLAN IS THE PIPELINE.  Each phase is a sequence of named steps
// (internal/steps), then the sweep, then the canonical print.  A step may be
// a sweep too, where an edit needs the text swept before its next step reads
// it.  There are no stages: every phase is swept, and what it hands on is
// C, swept and canonical.  That is everything a phase does to the source; the rest of what a phase
// program did -- building the binary its check measures, snapshotting symbols,
// writing a state directory -- is the check's, and a build does none of it.
//
// It was derived from the 163 phase programs and the stage schedule they ran
// under (internal/phase/STAGES.md, a record now), and it is held to them by
// the only gate that matters: the product.  A build from the committed slim-vim.c must be the committed whim-vim.c, byte
// for byte.  A step in the wrong order or a dropped argument
// moves those bytes, so the table is checked by `whim build --check` and
// not by reading it.
//
// THREE ARGUMENTS ARE NOT LITERAL, because three phases need something the
// source does not carry:
//
//	@state     a scratch directory.  Eight phases' edits write a file there
//	           for their check to read; a build gives them one and throws it
//	           away.  No edit reads anything from it -- measured.
//	@minmax    the host's MIN and MAX, asked of the preprocessor
//	           (steps.MinMax) exactly as phase 109's program asked.
//	Declared   phase 80 alone: its edit wants the rows it must find as stubs,
//	           which is what the phase declares in internal/phase/080/delta.md.
package build

import "github.com/arbace/go-whim/crefactor/pipeline"

// Step and Phase are the generic driver's (crefactor/pipeline): a
// Step marked Declared is handed REMOVED, which config sets from the phase's
// delta.md.
type (
	Step  = pipeline.Step
	Phase = pipeline.Phase
)

// Plan is the pipeline, phase by phase.
var Plan = []Phase{
	{N: 0, Block: "s00-seed", Name: "seed, in the one spelling every later phase reads", Seed: true, NoSource: true},
	{N: 1, Block: "d01-front", Name: "no `$VIMRUNTIME`",
		Steps: []Step{
			{Op: "front", Declared: true},
			{Op: "noruntime"},
		}},
	{N: 2, Block: "d02-outside", Name: "the options for features that are not here",
		Steps: []Step{
			{Op: "query-empty", Args: []string{"whim2"}},
		}},
	{N: 3, Name: "no introduction, and the command line says only what the editor still decides",
		Steps: []Step{
			{Op: "nointro"},
			{Op: "optreaders"},
		}},
	// 4, the binary's name stops choosing what it does: a record (internal/phase/archive/004/GOAL.md); its cut went to argvfront, phase 1.
	// 5, one regexp engine, not two: a record (internal/phase/archive/005/GOAL.md); its cut went to phase 1 (nonfa, the reform's D11).
	{N: 6, Name: "the editor stops writing shell scripts, and stops drawing a menu",
		Steps: []Step{
			{Op: "nowild"},
			{Op: "nowildmenu"},
		}},
	{N: 7, Name: "the editor stops looking for files it was not given",
		Steps: []Step{
			{Op: "noglob"},
		}},
	// 8, `:!` keeps its name and loses its process: a record (internal/phase/archive/008/GOAL.md); its cut went to phase 1 (noshellout, the reform's D12).
	// 9, the editor stops asking the environment what language it is in: a record (internal/phase/archive/009/GOAL.md); its cut went to phase 1 (nolocale, the reform's D6).
	// 10, no tag stack: a record (internal/phase/archive/010/GOAL.md); its cut went to phase 1 (notags, the reform's D10).
	// 11, nothing is written that was not asked for: a record (internal/phase/archive/011/GOAL.md); its cut went to phase 1 (noswap, the reform's D5).
	// 12, UTF-8, and no other encoding, ever: a record (internal/phase/archive/012/GOAL.md); its cut went to phase 1 (noenc, the reform's D7).
	{N: 13, Name: "the editor stops re-reading a file it has already read",
		Steps: []Step{
			{Op: "nostat"},
		}},
	{N: 14, Name: "a file name means the file of that name",
		Steps: []Step{
			{Op: "nofind"},
		}},
	// 15, the last two encoding options: a record (internal/phase/archive/015/GOAL.md); its cut went to phase 1 (nofencs, the reform's D7).
	{N: 16, Name: "six options that no longer decide anything",
		Steps: []Step{
			{Op: "noinertopts"},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_path", "b_p_sua", "b_p_tags", "b_p_tc", "b_p_ar", "b_p_swf"}},
		}},
	{N: 17, Name: "the last two per-buffer encoding options",
		Steps: []Step{
			// nofenc runs at phase 1 (the reform's D7); the fields still have
			// readers there
			{Op: "droplocal", Args: []string{"b_p_fenc", "b_p_bomb"}},
		}},
	// 18, nothing is read at startup, and nothing on the command line decides anything: a record (internal/phase/archive/018/GOAL.md); its cut went to phase 1 (nostartup and nocmdopts, the reform's D6).
	// 19, the terminal is what the build says: a record (internal/phase/archive/019/GOAL.md); its cut went to phase 1 (noterm, the reform's D8).
	{N: 20, Name: "nothing outside the process is consulted",
		Steps: []Step{
			{Op: "nohome"},
			{Op: "nogetenv"},
		}},
	// 21, there is nothing to recover, and the memfile is memory: a record (internal/phase/archive/021/GOAL.md); its cuts went to phase 1 (norecover and nomemfile, the reform's D5).
	{N: 22, Name: "the working directory is where it started",
		Steps: []Step{
			{Op: "nochdir"},
		}},
	{N: 23, Name: "no floating-point library",
		Steps: []Step{
			{Op: "nofloat"},
		}},
	// 24, there is no mouse: a record (internal/phase/archive/024/GOAL.md); its cut went to phase 1 (nomouse, the reform's D8).
	{N: 25, Name: "a write is a write, and nobody owns it",
		Steps: []Step{
			{Op: "nobackup"},
			{Op: "noowner"},
			{Op: "droplocal", Args: []string{"b_p_bkc"}},
		}},
	// 26, five signals, not twenty-one: a record (internal/phase/archive/026/GOAL.md); its cut went to phase 1 (nosignals, the reform's D12).
	// 27, `[[=a=]]` stops meaning \"a with any accent\": a record (internal/phase/archive/027/GOAL.md); its cut went to phase 1 (noequiclass, the reform's D11).
	{N: 28, Block: "d03-editing", Name: "C indenting",
		Steps: []Step{
			// nocindent runs at phase 1 (the reform's D10)
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_cin", "b_p_cink", "b_p_cino", "b_p_cinsd", "b_p_cinw"}},
		}},
	// 29, `:command`, user-defined commands: a record (internal/phase/archive/029/GOAL.md); its cut went to phase 1 (noucmd, the reform's D10).
	// 30, `K` and the tag jumps, keeping `*` and `#`: a record (internal/phase/archive/030/GOAL.md); its cut went to phase 1 (noident, the reform's D10).
	// 31, file-name modifiers: a record (internal/phase/archive/031/GOAL.md); its cut went to phase 1 (nofnamemod, the reform's D10).
	{N: 32, Name: "insert completion, the popup menu, and the keys that reached them",
		Steps: []Step{
			// nocompl and nocomplkeys run at phase 1 (the reform's D10)
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_cpt", "b_p_cot", "b_p_dict", "b_p_tsr", "b_p_inf", "b_p_ac"}},
		}},
	// 33, commands whose machinery has already gone: a record (internal/phase/archive/033/GOAL.md); what it cut went with the Ex commands retired at phase 1 (exfront, the reform's D2).
	// 34, no abbreviations: a record (internal/phase/archive/034/GOAL.md); its cut went to phase 1 (noabbr, the reform's D10).
	// 35, no scripts, no session, no autocommands: a record (internal/phase/archive/035/GOAL.md); its cut went to phase 1 (nosession, the reform's D6).
	// 36, one tab page, always: a record (internal/phase/archive/036/GOAL.md); its cut went to phase 1 (notabs, the reform's D9).
	// 37, no command that does nothing: a record (internal/phase/archive/037/GOAL.md); its cut went to phase 1 (noinert, the reform's D9).
	// 38, the argument list is walked by `:next` and `:previous` alone: a record (internal/phase/archive/038/GOAL.md); its cut went to phase 1 (noarglist, the reform's D9).
	// Phases 39-40, merged: one window, then no window sizes.
	// 40, one window, and no window sizes: a record (internal/phase/archive/040/GOAL.md); its cut went to phase 1 (nowindows and nowinsizes, the reform's D9).
	// 41, the buffer list is walked by `:bnext` and `:bprevious` alone: a record (internal/phase/archive/041/GOAL.md); its cut went to phase 1 (nobuflist, the reform's D9).
	{N: 42, Block: "d04-one-buffer", Name: "one buffer, always",
		Steps: []Step{
			{Op: "onebuffer"},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_bh"}},
		}},
	// 43, no -c, --cmd, -R, -m, -M or -w: a record (internal/phase/archive/043/GOAL.md); its cut went to argvfront and the fall-out closure, phase 1.
	// Phases 44-48, merged: Ex commands retired one by one, one idea split for history's sake.
	{N: 48, Block: "d05-commands-and-options", Name: "no filters, sorting, alignment, `:drop`, `:wall` and the `:…all` commands, `:startinsert` and its kin, or `:noswapfile`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim44"}},
			{Op: "edit", Args: []string{"whim48"}},
		}},
	{N: 49, Name: "one set of options",
		Steps: []Step{
			{Op: "oneoptset"},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_ml"}},
		}},
	{N: 50, Block: "d06-encoding", Name: "only LF text files",
		Steps: []Step{
			{Op: "lfonly"},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_bin", "b_p_ff", "b_p_fixeol", "b_p_tx"}},
		}},
	// Phases 51-53, merged: the encoding, reduced to UTF-8 in three steps.
	{N: 53, Name: "UTF-8: the bytes kept, UTF-8 only, no conversion",
		Steps: []Step{
			// utf8only runs at phase 1 (the reform's D7); keepbytes and noconv
			// stay, after phase 50's lfonly, which takes the ++ff arm their
			// getargopt chain starts after
			{Op: "keepbytes"},
			{Op: "noconv"},
			{Op: "droplocal", Args: []string{"b_p_menc"}},
		}},
	// 54, no option without a variable: a record (internal/phase/archive/054/GOAL.md); every row it dropped is dropped at phase 1 with every option the product has not (optfront, the reform's D3).
	{N: 55, Block: "d07-options", Name: "no option nothing reads",
		Steps: []Step{
			// its edit, 'cdpath''s completion term, went with the whole test at
			// phase 1 (whim56, the reform's D6)
			{Op: "droplocal", Args: []string{"b_p_sn", "b_p_cms", "b_p_lop"}},
		}},
	{N: 56, Name: "no shell, runtime or keyword-program options",
		Steps: []Step{
			// whim56 runs at phase 1 (the reform's D6); 'keywordprg''s field
			// still has readers there, so its cut stays here
			{Op: "edit", Args: []string{"whim56kp"}},
			{Op: "droplocal", Args: []string{"b_p_kp"}},
		}},
	{N: 57, Name: "no lisp",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim57"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_lisp", "b_p_lw"}},
		}},
	{N: 58, Name: "no language mappings",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim58"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_iminsert", "b_p_imsearch"}},
		}},
	{N: 59, Name: "no command-line completion",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim59"}},
		}},
	{N: 60, Name: "no suffix, case, delay, verbose-file, debug or filter-program options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim60"}},
			{Op: "sweep"},
			{Op: "edit", Args: []string{"whim60ep"}},
			{Op: "droplocal", Args: []string{"b_p_fp", "b_p_ep"}},
		}},
	// 61, no window title: a record (internal/phase/061/GOAL.md); its program runs at phase 1 (whim61, the reform's D8).
	{N: 62, Name: "no buffer-type, file-type, listing, jump, update-time or autowrite options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim62"}},
			{Op: "sweep"},
			{Op: "edit", Args: []string{"whim62bl"}},
			{Op: "droplocal", Args: []string{"b_p_bt", "b_p_ft"}},
		}},
	{N: 63, Block: "d08-editing", Name: "no jump list",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim63"}},
		}},
	{N: 64, Name: "no formatting, comment or nroff-macro options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim64"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_fo", "b_p_flp", "b_p_com"}},
		}},
	{N: 65, Name: "no rot13, no operator function, no empty key handler",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim65"}},
		}},
	{N: 66, Name: "no sentences, paragraphs, sections, methods, #if blocks or comment blocks",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim66"}},
		}},
	{N: 67, Block: "d09-one-of-each", Name: "no mouse, no spell plumbing, no write-only flags",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim67"}},
		}},
	{N: 68, Name: "one window, structurally",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim68"}},
		}},
	{N: 69, Name: "one file argument, and no argument list",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim69"}},
		}},
	{N: 70, Name: ":e reloads in place, and there is no swap file",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim70"}},
		}},
	{N: 71, Name: "one buffer, structurally",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim71"}},
		}},
	// Phases 72-73, merged: one window/tab page structurally, then one frame.
	{N: 73, Name: "one window and tab page structurally, and one frame",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim72"}},
			{Op: "edit", Args: []string{"whim73"}},
		}},
	{N: 74, Block: "d10-editing", Name: "no file marks",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim74"}},
		}},
	{N: 75, Name: "no autocommands",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim75"}},
		}},
	{N: 76, Name: "one regexp engine, so no retry",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim76"}},
		}},
	{N: 77, Block: "d11-commands", Name: "no buffer-name argument matching",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim77"}},
		}},
	{N: 78, Name: "empty functions, write-only counters, and the window id",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim78"}},
		}},
	{N: 79, Name: "the constant-return predicates",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim79"}},
		}},
	{N: 80, Name: "the Ex command table, cut to the commands that exist",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim80"}},
		}},
	// 81, one line, one command: a record (internal/phase/archive/081/GOAL.md); its edits went to onecmdfront, phase 1.
	// 82, every comment: a record (internal/phase/archive/082/GOAL.md); it edits nothing now.
	// 83, the core's compile line, and the baselines it is measured against: a record (internal/phase/archive/083/GOAL.md); it edits nothing now.
	// 84, the stack protector goes: a record (internal/phase/archive/084/GOAL.md); it edits nothing now.
	{N: 85, Block: "d12-terminal-and-ex", Name: "the core stops diagnosing its own terminal",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim85"}},
		}},
	// 86, the instrument becomes the screen: a record (internal/phase/archive/086/GOAL.md); it edits nothing now.
	{N: 87, Name: "no streaming Ex",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim87"}},
		}},
	// 88, argv is `+{command}` and `-T {term}`: a record (internal/phase/archive/088/GOAL.md); its cut went to argvfront and the fall-out closure, phase 1.
	{N: 89, Block: "d13-files", Name: "no write",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim89"}},
		}},
	{N: 90, Name: "no read",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim90"}},
		}},
	{N: 91, Name: "no `:edit`, and no `gf`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim91"}},
		}},
	{N: 92, Name: "nothing reads a byte",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim92"}},
		}},
	{N: 93, Name: "the buffer has no name",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim93"}},
		}},
	{N: 94, Name: "`:q` quits, and `ZZ` is `ZQ`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim94"}},
		}},
	{N: 95, Name: "the options nothing reads",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim95"}},
			{Op: "droplocal", Args: []string{"b_p_fs"}},
			{Op: "droplocal", Args: []string{"b_p_ro"}},
			{Op: "edit", Args: []string{"whim95rows"}},
		}},
	{N: 96, Name: "no `FILE *` that is never opened",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim96"}},
		}},
	{N: 97, Block: "r01-libc", Name: "the strings are the editor's own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim97"}},
		}},
	{N: 98, Name: "the character classes, the numbers and the sort",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim98"}},
		}},
	// 99, the includes nothing names: a record (internal/phase/archive/099/GOAL.md); it edits nothing now.
	// Phases 100-102, merged: the core's way out: the deadly ladder, vim_main, no stopping the process.
	{N: 102, Block: "r02-host-chain", Name: "the deadly ladder, `vim_main`, and a core that cannot stop the process",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim100"}},
			{Op: "edit", Args: []string{"whim101"}},
			{Op: "edit", Args: []string{"whim102"}},
		}},
	{N: 103, Name: "the signals and the terminal are the host's",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim103"}},
		}},
	{N: 104, Name: "the messages are the editor's, the writing is the host's",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim104"}},
		}},
	// Phases 105-107, merged: C23 spelling, in three steps.
	{N: 107, Block: "g01-c23-spelling", Name: "C23 spelling: the variadic collapse, `nullptr` and `usize`, the attributes",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim105"}},
			{Op: "edit", Args: []string{"whim106", "--casts", "1"}},
			{Op: "edit", Args: []string{"whim107"}},
		}},
	{N: 108, Block: "r03-boundary", Name: "the plain host calls",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim108"}},
		}},
	{N: 109, Name: "the header types and macros the core can own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim109", "@minmax"}},
		}},
	{N: 110, Name: "the move: the first `#include` becomes the boundary",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim110", "@state"}},
		}},
	{N: 111, Name: "the scalar clock",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim111"}},
		}},
	{N: 112, Name: "the case tables become one, and it is the union",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim112"}},
		}},
	{N: 113, Name: "the message fold: `msg_puts_printf()` and the branch that reaches it",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim113"}},
		}},
	{N: 114, Name: "`abs` and `labs`, the two the core took on trust",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim114", "abs=1", "labs=2"}},
		}},
	{N: 115, Name: "the clock crosses the boundary",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim115"}},
		}},
	// 116, the terminal table is asked with `+set term=`, not `$TERM`: a record (internal/phase/archive/116/GOAL.md); it edits nothing now.
	// Phases 117-119, merged: the core calls nothing but the host.
	{N: 119, Name: "the core calls nothing but the host",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim117"}},
			{Op: "edit", Args: []string{"whim118", "@state"}},
			{Op: "edit", Args: []string{"whim119", "@state"}},
		}},
	{N: 120, Block: "g02-unions", Name: "the degenerate unions go",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim120", "--degenerate", "1", "--genuine", "1"}},
		}},
	// Phases 121-122, merged: the terminal names, then -T.
	{N: 122, Block: "r04-terminal", Name: "the terminal names, and `-T`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim121"}},
			{Op: "edit", Args: []string{"whim122"}},
		}},
	// 123, the instrument could not see the text layer: a record (internal/phase/archive/123/GOAL.md); it edits nothing now.
	{N: 124, Block: "r05-memory", Name: "freeing is free, and the arena is measured",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim124", "@state"}},
		}},
	{N: 125, Block: "r06-memline", Name: "the swap file's residue, and what no sweep could find",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim125", "@state"}},
		}},
	{N: 126, Name: "a block number becomes a reference",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim126", "@state"}},
		}},
	{N: 127, Name: "de-page the leaf",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim127", "@state"}},
		}},
	{N: 128, Name: "fold the node types",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim128", "@state"}},
		}},
	{N: 129, Block: "r07-types", Name: "`p_emoji` is an `int`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim129"}},
		}},
	{N: 130, Name: "the `(pos_T *)-1` tests go",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim130"}},
		}},
	{N: 131, Name: "the saved input buffer is a `garray_T *`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim131"}},
		}},
	{N: 132, Block: "r08-memory", Name: "nothing frees",
		Steps: []Step{
			// 272: exe_commands' free of what cmds_tofree marked went at phase 1,
			// nothing writing it once the command line was cut (argvfront, D1)
			{Op: "edit", Args: []string{"whim132", "--calls", "272", "--redirected", "3"}},
		}},
	{N: 133, Name: "one buffer needs no hash table",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim133"}},
		}},
	{N: 134, Block: "g03-empty-blocks", Name: "the empty blocks fold",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim134", "--at-least", "20"}},
		}},
	{N: 135, Block: "r09-regex", Name: "one regexp program type",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim135"}},
		}},
	{N: 136, Name: "the engine is called directly",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim136"}},
		}},
	{N: 137, Block: "r10-types", Name: "the changedtick is a number",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim137"}},
		}},
	{N: 138, Name: "no parameter carries an eval value",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim138"}},
		}},
	{N: 139, Name: "the core sorts and searches typed arrays",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim139"}},
		}},
	{N: 140, Name: "highlight groups are found in their array",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim140"}},
		}},
	{N: 141, Block: "r11-gotos", Name: "`regrepeat()` does not jump into a case",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim141"}},
		}},
	{N: 142, Name: "the version names no build date or time",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim142"}},
		}},
	// Phases 143-145, merged: three functions lose their goto.
	{N: 145, Name: "`regatom()`, `edit()` and `check_termcode()` have no goto",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim143"}},
			{Op: "edit", Args: []string{"whim144"}},
			{Op: "edit", Args: []string{"whim145"}},
		}},
	{N: 146, Block: "r12-memline-and-host", Name: "a memline node names its block",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim146"}},
		}},
	{N: 147, Name: "`deathtrap()` runs at the host's next wait",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim147"}},
		}},
	// Phases 148-149, merged: allocation cannot fail, then its branches fold.
	{N: 149, Block: "g04-never-null", Name: "allocation cannot fail, and its branches fold",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim148"}},
			{Op: "edit", Args: []string{"whim149", "--at-least", "80"}},
		}},
	{N: 150, Block: "r13-translation", Name: "the regexp stack is three typed stacks",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim150"}},
		}},
	// Phases 151-152, merged: the option table typed: its defaults, then its variables.
	{N: 152, Name: "the option table's defaults and its variables, typed",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim151"}},
			{Op: "edit", Args: []string{"whim152"}},
		}},
	// Phases 153-154, merged: one function, its cast and then its NULL write.
	{N: 154, Name: "`free_one_termoption()` compares without a cast, and its NULL write is gone",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim153"}},
			{Op: "edit", Args: []string{"whim154"}},
		}},
	{N: 155, Name: "call arguments with effects are evaluated in gcc's order",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim155"}},
		}},
	{N: 156, Name: "the regex size pass's node is a static byte, not (char_u *) -1",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim156"}},
		}},
	{N: 157, Name: "get_register() and put_register() carry a yankreg_T *, not a void *",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim157"}},
		}},
	{N: 158, Name: "a highlight's terminal font is read only from a colour entry",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim158"}},
		}},
	{N: 159, Name: "a struct's text is a pointer to an allocation of its own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim159"}},
		}},
	{N: 160, Name: "no line getter takes a cookie",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim160"}},
		}},
	{N: 161, Name: "no goto jumps into a block",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim161"}},
		}},
	{N: 162, Name: "no two function pointers are compared",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim162"}},
		}},
	// 163, the product is in the one canonical spelling: a record (internal/phase/archive/163/GOAL.md); it edits nothing now.
	// Phases 164-165, merged: what the Go's linters found dead, in two steps.
	{N: 165, Block: "g05-dead", Name: "what the Go's linters found dead",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim164"}},
			{Op: "edit", Args: []string{"whim165"}},
		}},
	// Phases 166-168, merged: for the Go: bool, key names, goto as return.
	// 168, `goto` as `return`: a record (internal/phase/archive/168/GOAL.md); GotoTail
	// (170) takes every goto it took, a `return x;` being a tail of none (the
	// reform's G, measured byte for byte).
	{N: 167, Block: "g06-bool-and-keys", Name: "`bool` and key names",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim166"}},
			{Op: "edit", Args: []string{"whim167"}},
		}},
	{N: 169, Block: "g07-includes", Name: "the system headers nothing needs",
		Steps: []Step{
			{Op: "includes"},
		}},
	{N: 170, Block: "g08-gotos", Name: "a goto whose label marks a short tail is that tail",
		Steps: []Step{
			{Op: "gototail", Args: []string{"--at-least", "52"}},
		}},
	{N: 171, Name: "a goto that is a break is break",
		Steps: []Step{
			{Op: "gotobreak", Args: []string{"--at-least", "5"}},
		}},
	{N: 172, Name: "a goto back is a loop",
		Steps: []Step{
			{Op: "gotoloop", Args: []string{"--at-least", "2"}},
		}},
	{N: 173, Name: "a goto out of its block is a break",
		Steps: []Step{
			{Op: "gotoblock", Args: []string{"--at-least", "50"}},
		}},
	{N: 174, Block: "r14-parallel-substitute", Name: "no address of a position's line or column",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim174"}},
		}},
	{N: 175, Name: "a member's address a call hands back is a local's",
		Steps: []Step{
			{Op: "memberout"},
		}},
	{N: 176, Name: "the regex engine's state is a parameter",
		Steps: []Step{
			{Op: "stateparam", Args: []string{"--at-least", "43"}},
		}},
	{N: 177, Name: "a line's match on its own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim177"}},
		}},
	{N: 178, Name: ":g marks the lines match_lines finds",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim178"}},
		}},
	{N: 179, Name: "no mark is cleared when none was set",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim179"}},
		}},
	{N: 180, Name: "the host's clock can be held still",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim180"}},
		}},
	{N: 181, Block: "g09-values", Name: "an out-parameter a value, a struct local its members",
		Steps: []Step{
			{Op: "localout"},
			{Op: "structscalar"},
		}},
	{N: 182, Block: "g10-plain-c", Name: "gettext's identity not called, the ASCII tests named, constant ifs their branch",
		Steps: []Step{
			{Op: "identity"},
			{Op: "asciiclass"},
			{Op: "constbranch"},
		}},
	{N: 183, Block: "g11-bool", Name: "a file-scope flag is bool",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim183"}},
		}}, {N: 184, Name: "more flags are bool",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim184"}},
		}},
}
