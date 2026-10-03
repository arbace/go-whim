// Package build is whim's pipeline: slim-vim.c in, whim-vim.c out, in one
// process and in memory.  The driver is generic (crefactor/pipeline);
// what is whim's is here -- this plan, the file names, and how the plan's
// non-literal arguments are resolved (build.go's config).
//
// THE PLAN IS THE PIPELINE.  Each phase is a sequence of named steps
// (internal/steps), then the sweep, then the canonical print.  A step may be
// a sweep too, where an edit needs the text swept before its next step reads
// it.  A step is a TEXT step, the default, or a GRAPH step (Graph set: its op
// is in internal/steps' graph table and edits crefactor/graph's graph); the
// driver converts where the kind changes, and a phase that ends on the graph
// is collected and printed by the C view, the same text the sweep and the
// canonical print would give (crefactor/pipeline's hybrid.go, doc/GRAPH.md
// step 5).  There are no stages: every phase is swept, and what it hands on is
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
//	           (steps.MinMax) exactly as phase 42's program asked.
//	Declared   phase 1 alone: its front retires the command rows the phase
//	           declares in internal/phase/001/delta.md.
package build

import "github.com/arbace/go-whim/crefactor/pipeline"

// Step and Phase are the generic driver's (crefactor/pipeline): a
// Step marked Declared is handed REMOVED, which config sets from the phase's
// delta.md.
type (
	Step  = pipeline.Step
	Phase = pipeline.Phase
)

// Plan is the pipeline, phase by phase.  A phase's N is its place in the
// plan, 0 to 103 without a gap (TestPlanNumbers), and names its snapshot
// (qNNN.c), its directory (internal/phase/NNN/) and its program (whimN).  A
// program a phase runs among its steps that is not its own is a PART,
// internal/phase/NNN/x/ and whimNx, lettered in the order the phase runs them:
// a member of a group that runs as one phase, or a program the front calls.
// doc/PHASES.md maps every number to the old numbering, under which the
// phases that edit nothing now are records in internal/phase/archive/.
//
// THE GRAPH STEPS (doc/GRAPH.md step 5, doc/GRAPH-MIGRATION.md): every
// `droplocal` (internal/cut's DropLocal, a deletion and its fall-out) and
// phase 24's program.  Phases 8, 13, 14, 17 and 24 begin on the graph -- the
// first step, sweeps aside, is a graph step -- so the run hands them the
// graph the phase before left, and the check reads it from qNNN.g; the rest
// import the text where their graph steps begin.  Phases 7, 8, 12-14,
// 16-20 and 24 end on the graph: collected, not swept.
var Plan = []Phase{
	// Phase 0 seeds the input canonically and spells it in C23 there:
	// whim0a's `nullptr` and `usize` (internal/phase/000/a), whim0b's
	// variadic collapse (internal/phase/000/b: one function walks a va_list)
	// and whim0c's attributes (internal/phase/000/c: `unused` gone,
	// `fallthrough` C23's `[[fallthrough]];`), so that every phase
	// after it is written in the spelling the product has.
	{N: 0, Block: "s00-seed", Name: "seed, in the one spelling every later phase reads", Seed: true,
		Steps: []Step{
			{Op: "edit", Args: []string{"whim0a", "--casts", "1"}},
			{Op: "edit", Args: []string{"whim0b"}},
			{Op: "edit", Args: []string{"whim0c"}},
		}},
	{N: 1, Block: "d01-front", Name: "no `$VIMRUNTIME`",
		Steps: []Step{
			{Op: "front", Declared: true},
			{Op: "noruntime"},
		}},
	{N: 2, Name: "the front, continued; and the options for features that are not here",
		Steps: []Step{
			{Op: "front2"},
			{Op: "query-empty", Args: []string{"whim2"}},
		}},
	{N: 3, Name: "the front, continued; and no introduction, and the command line says only what the editor still decides",
		Steps: []Step{
			{Op: "front3"},
			{Op: "nointro"},
			{Op: "optreaders"},
		}},
	{N: 4, Name: "the front, on swept text; and the editor stops writing shell scripts, and stops drawing a menu",
		Steps: []Step{
			// part 4a's program, which asserts the NFA engine is gone: on the
			// text the front swept (nonfa, phase 3)
			{Op: "nostat"},
			{Op: "edit", Args: []string{"whim4a"}},
			// parts 3a's and 3b's fields, whose readers phase 3 took (whim3a
			// and whim3b, on the front)
			{Op: "droplocal", Graph: true, Args: []string{"b_p_lisp", "b_p_lw", "b_p_iminsert", "b_p_imsearch"}},
			// onebuffer (record 42's cut), its sweep and droplocal, then the
			// programs of parts 4b-4e: they count and anchor on swept text
			{Op: "onebuffer"},
			{Op: "sweep"},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_bh"}},
			{Op: "edit", Args: []string{"whim4b"}},
			{Op: "edit", Args: []string{"whim4c"}},
			{Op: "edit", Args: []string{"whim4d"}},
			{Op: "edit", Args: []string{"whim4e"}},
			{Op: "nowild"},
			{Op: "nowildmenu"},
			// part 4f's program, after nowildmenu: it takes what of
			// getcmdline_int()'s completion the menu's cut leaves
			{Op: "edit", Args: []string{"whim4f"}},
		}},
	{N: 5, Name: "the front, ended; and the editor stops looking for files it was not given",
		Steps: []Step{
			// nobackup (phase 12's cut), lfonly (record 50's) and keepbytes
			// and noconv (records 51-53's), on the text phase 4 swept, with
			// readfile() and mch_call_shell_fork() gone (nostat and nowild,
			// phase 4); part 5a's program, in noconv's spelling; then parts
			// 5b's and 5c's, which count ONE_WINDOW and the frame tree's
			// writers; then the fields of records 50 and 53 and of part 5a
			{Op: "nobackup"},
			{Op: "lfonly"},
			{Op: "keepbytes"},
			{Op: "noconv"},
			{Op: "edit", Args: []string{"whim5a"}},
			{Op: "edit", Args: []string{"whim5b"}},
			{Op: "edit", Args: []string{"whim5c"}},
			{Op: "sweep"},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_bin", "b_p_ff", "b_p_fixeol", "b_p_tx", "b_p_menc"}},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_fo", "b_p_flp", "b_p_com"}},
			// part 5d's program, which counts the dispatches on swept text
			{Op: "edit", Args: []string{"whim5d"}},
			{Op: "noglob"},
		}},
	{N: 6, Block: "d02-outside", Name: "a file name means the file of that name",
		Steps: []Step{
			{Op: "nofind"},
		}},
	{N: 7, Name: "six options that no longer decide anything",
		Steps: []Step{
			{Op: "noinertopts"},
			{Op: "sweep"},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_path", "b_p_sua", "b_p_tags", "b_p_tc", "b_p_ar", "b_p_swf"}},
		}},
	{N: 8, Name: "the last two per-buffer encoding options",
		Steps: []Step{
			// nofenc runs at phase 2 (the reform's D7); the fields still have
			// readers there
			{Op: "droplocal", Graph: true, Args: []string{"b_p_fenc", "b_p_bomb"}},
		}},
	{N: 9, Name: "nothing outside the process is consulted",
		Steps: []Step{
			{Op: "nohome"},
			{Op: "nogetenv"},
		}},
	{N: 10, Name: "the working directory is where it started",
		Steps: []Step{
			{Op: "nochdir"},
		}},
	{N: 11, Name: "no floating-point library",
		Steps: []Step{
			{Op: "nofloat"},
		}},
	{N: 12, Name: "a write is a write, and nobody owns it",
		Steps: []Step{
			// nobackup runs at phase 5, before noconv, which is written for
			// buf_write() without the backup
			{Op: "noowner"},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_bkc"}},
		}},
	{N: 13, Block: "d03-editing", Name: "C indenting",
		Steps: []Step{
			// nocindent runs at phase 3 (the reform's D10)
			{Op: "sweep"},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_cin", "b_p_cink", "b_p_cino", "b_p_cinsd", "b_p_cinw"}},
		}},
	{N: 14, Name: "insert completion, the popup menu, and the keys that reached them",
		Steps: []Step{
			// nocompl and nocomplkeys run at phase 3 (the reform's D10)
			{Op: "sweep"},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_cpt", "b_p_cot", "b_p_dict", "b_p_tsr", "b_p_inf", "b_p_ac"}},
		}},
	{N: 15, Block: "d05-commands-and-options", Name: "no filters, sorting, alignment, `:drop`, `:wall` and the `:…all` commands, `:startinsert` and its kin, or `:noswapfile`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim15a"}},
			{Op: "edit", Args: []string{"whim15"}},
		}},
	{N: 16, Name: "one set of options",
		Steps: []Step{
			{Op: "oneoptset"},
			{Op: "sweep"},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_ml"}},
		}},
	{N: 17, Block: "d07-options", Name: "no option nothing reads",
		Steps: []Step{
			// its edit, 'cdpath''s completion term, went with the whole test at
			// phase 2 (whim18, the reform's D6)
			{Op: "droplocal", Graph: true, Args: []string{"b_p_sn", "b_p_cms", "b_p_lop"}},
		}},
	{N: 18, Name: "no shell, runtime or keyword-program options",
		Steps: []Step{
			// whim18 runs at phase 2 (the reform's D6); 'keywordprg''s field
			// still has readers there, so its cut stays here
			{Op: "edit", Args: []string{"whim18kp"}},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_kp"}},
		}},
	{N: 19, Name: "no suffix, case, delay, verbose-file, debug or filter-program options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim19"}},
			{Op: "sweep"},
			{Op: "edit", Args: []string{"whim19ep"}},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_fp", "b_p_ep"}},
		}},
	{N: 20, Name: "no buffer-type, file-type, listing, jump, update-time or autowrite options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim20"}},
			{Op: "sweep"},
			{Op: "edit", Args: []string{"whim20bl"}},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_bt", "b_p_ft"}},
		}},
	{N: 21, Block: "d09-one-of-each", Name: "one file argument, and no argument list",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim21"}},
		}},
	{N: 22, Name: ":e reloads in place, and there is no swap file",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim22"}},
		}},
	{N: 23, Block: "d11-commands", Name: "no buffer-name argument matching",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim23"}},
		}},
	{N: 24, Name: "empty functions, write-only counters, and the window id",
		Steps: []Step{
			{Op: "edit", Graph: true, Args: []string{"whim24"}},
		}},
	{N: 25, Name: "the constant-return predicates",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim25"}},
		}},
	{N: 26, Name: "the Ex command table, cut to the commands that exist",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim26"}},
		}},
	{N: 27, Block: "d12-terminal-and-ex", Name: "no streaming Ex",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim27"}},
		}},
	{N: 28, Block: "d13-files", Name: "no write",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim28"}},
		}},
	{N: 29, Name: "no read",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim29"}},
		}},
	{N: 30, Name: "no `:edit`, and no `gf`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim30"}},
		}},
	{N: 31, Name: "nothing reads a byte",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim31"}},
		}},
	{N: 32, Name: "the buffer has no name",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim32"}},
		}},
	{N: 33, Name: "`:q` quits, and `ZZ` is `ZQ`",
		Steps: []Step{
			{Op: "edit", Graph: true, Args: []string{"whim33"}},
		}},
	{N: 34, Name: "the options nothing reads",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim34"}},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_fs"}},
			{Op: "droplocal", Graph: true, Args: []string{"b_p_ro"}},
			{Op: "edit", Args: []string{"whim34rows"}},
		}},
	{N: 35, Name: "no `FILE *` that is never opened",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim35"}},
		}},
	{N: 36, Block: "r01-libc", Name: "the strings are the editor's own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim36"}},
		}},
	{N: 37, Name: "the character classes, the numbers and the sort",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim37"}},
		}},
	{N: 38, Block: "r02-host-chain", Name: "the deadly ladder, `vim_main`, and a core that cannot stop the process",
		Steps: []Step{
			{Op: "edit", Graph: true, Args: []string{"whim38a"}},
			{Op: "edit", Args: []string{"whim38b"}},
			{Op: "edit", Args: []string{"whim38"}},
		}},
	{N: 39, Name: "the signals and the terminal are the host's",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim39"}},
		}},
	{N: 40, Name: "the messages are the editor's, the writing is the host's",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim40"}},
		}},
	{N: 41, Block: "r03-boundary", Name: "the plain host calls",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim41"}},
		}},
	{N: 42, Name: "the header types and macros the core can own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim42", "@minmax"}},
		}},
	{N: 43, Name: "the move: the first `#include` becomes the boundary",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim43", "@state"}},
		}},
	{N: 44, Name: "the scalar clock",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim44"}},
		}},
	{N: 45, Name: "the case tables become one, and it is the union",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim45"}},
		}},
	{N: 46, Name: "the message fold: `msg_puts_printf()` and the branch that reaches it",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim46"}},
		}},
	{N: 47, Name: "`abs` and `labs`, the two the core took on trust",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim47", "abs=1", "labs=2"}},
		}},
	{N: 48, Name: "the clock crosses the boundary",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim48"}},
		}},
	{N: 49, Name: "the core calls nothing but the host",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim49a"}},
			{Op: "edit", Args: []string{"whim49b", "@state"}},
			{Op: "edit", Args: []string{"whim49", "@state"}},
		}},
	{N: 50, Block: "g02-unions", Name: "the degenerate unions go",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim50", "--degenerate", "1", "--genuine", "1"}},
		}},
	{N: 51, Block: "r04-terminal", Name: "the terminal names, and `-T`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim51a"}},
			{Op: "edit", Args: []string{"whim51"}},
		}},
	{N: 52, Block: "r05-memory", Name: "freeing is free, and the arena is measured",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim52", "@state"}},
		}},
	{N: 53, Block: "r06-memline", Name: "the swap file's residue, and what no sweep could find",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim53", "@state"}},
		}},
	{N: 54, Name: "a block number becomes a reference",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim54", "@state"}},
		}},
	{N: 55, Name: "de-page the leaf",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim55", "@state"}},
		}},
	{N: 56, Name: "fold the node types",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim56", "@state"}},
		}},
	{N: 57, Block: "r07-types", Name: "`p_emoji` is an `int`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim57"}},
		}},
	{N: 58, Name: "the `(pos_T *)-1` tests go",
		Steps: []Step{
			{Op: "edit", Graph: true, Args: []string{"whim58"}},
		}},
	{N: 59, Name: "the saved input buffer is a `garray_T *`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim59"}},
		}},
	{N: 60, Block: "r08-memory", Name: "nothing frees",
		Steps: []Step{
			// 272: exe_commands' free of what cmds_tofree marked went at phase 1,
			// nothing writing it once the command line was cut (argvfront, D1)
			{Op: "edit", Args: []string{"whim60", "--calls", "272", "--redirected", "3"}},
		}},
	{N: 61, Name: "one buffer needs no hash table",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim61"}},
		}},
	{N: 62, Block: "g03-empty-blocks", Name: "the empty blocks fold",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim62", "--at-least", "20"}},
		}},
	{N: 63, Block: "r09-regex", Name: "one regexp program type",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim63"}},
		}},
	{N: 64, Name: "the engine is called directly",
		Steps: []Step{
			{Op: "edit", Graph: true, Args: []string{"whim64"}},
		}},
	{N: 65, Block: "r10-types", Name: "the changedtick is a number",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim65"}},
		}},
	{N: 66, Name: "no parameter carries an eval value",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim66"}},
		}},
	{N: 67, Name: "the core sorts and searches typed arrays",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim67"}},
		}},
	{N: 68, Name: "highlight groups are found in their array",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim68"}},
		}},
	{N: 69, Block: "r11-gotos", Name: "`regrepeat()` does not jump into a case",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim69"}},
		}},
	{N: 70, Name: "the version names no build date or time",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim70"}},
		}},
	{N: 71, Name: "`regatom()`, `edit()` and `check_termcode()` have no goto",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim71a"}},
			{Op: "edit", Args: []string{"whim71b"}},
			{Op: "edit", Args: []string{"whim71"}},
		}},
	{N: 72, Block: "r12-memline-and-host", Name: "a memline node names its block",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim72"}},
		}},
	{N: 73, Name: "`deathtrap()` runs at the host's next wait",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim73"}},
		}},
	{N: 74, Block: "g04-never-null", Name: "allocation cannot fail, and its branches fold",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim74a"}},
			{Op: "edit", Args: []string{"whim74", "--at-least", "80"}},
		}},
	{N: 75, Block: "r13-translation", Name: "the regexp stack is three typed stacks",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim75"}},
		}},
	{N: 76, Name: "the option table's defaults and its variables, typed",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim76a"}},
			{Op: "edit", Args: []string{"whim76"}},
		}},
	{N: 77, Name: "`free_one_termoption()` compares without a cast, and its NULL write is gone",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim77a"}},
			{Op: "edit", Graph: true, Args: []string{"whim77"}},
		}},
	{N: 78, Name: "call arguments with effects are evaluated in gcc's order",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim78"}},
		}},
	{N: 79, Name: "the regex size pass's node is a static byte, not (char_u *) -1",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim79"}},
		}},
	{N: 80, Name: "get_register() and put_register() carry a yankreg_T *, not a void *",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim80"}},
		}},
	{N: 81, Name: "a highlight's terminal font is read only from a colour entry",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim81"}},
		}},
	{N: 82, Name: "a struct's text is a pointer to an allocation of its own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim82"}},
		}},
	{N: 83, Name: "no line getter takes a cookie",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim83"}},
		}},
	{N: 84, Name: "no goto jumps into a block",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim84"}},
		}},
	{N: 85, Name: "no two function pointers are compared",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim85"}},
		}},
	{N: 86, Block: "g05-dead", Name: "what the Go's linters found dead",
		Steps: []Step{
			{Op: "edit", Graph: true, Args: []string{"whim86a"}},
			{Op: "edit", Args: []string{"whim86"}},
		}},
	{N: 87, Block: "g06-bool-and-keys", Name: "`bool` and key names",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim87a"}},
			{Op: "edit", Args: []string{"whim87"}},
		}},
	{N: 88, Block: "g07-includes", Name: "the system headers nothing needs",
		Steps: []Step{
			{Op: "includes"},
		}},
	{N: 89, Block: "g08-gotos", Name: "a goto whose label marks a short tail is that tail",
		Steps: []Step{
			{Op: "gototail", Args: []string{"--at-least", "52"}},
		}},
	{N: 90, Name: "a goto that is a break is break",
		Steps: []Step{
			{Op: "gotobreak", Args: []string{"--at-least", "5"}},
		}},
	{N: 91, Name: "a goto back is a loop",
		Steps: []Step{
			{Op: "gotoloop", Args: []string{"--at-least", "2"}},
		}},
	{N: 92, Name: "a goto out of its block is a break",
		Steps: []Step{
			{Op: "gotoblock", Args: []string{"--at-least", "50"}},
		}},
	{N: 93, Block: "r14-parallel-substitute", Name: "no address of a position's line or column",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim93"}},
		}},
	{N: 94, Name: "a member's address a call hands back is a local's",
		Steps: []Step{
			{Op: "memberout"},
		}},
	{N: 95, Name: "the regex engine's state is a parameter",
		Steps: []Step{
			{Op: "stateparam", Args: []string{"--at-least", "43"}},
		}},
	{N: 96, Name: "a line's match on its own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim96"}},
		}},
	{N: 97, Name: ":g marks the lines match_lines finds",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim97"}},
		}},
	{N: 98, Name: "no mark is cleared when none was set",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim98"}},
		}},
	{N: 99, Name: "the host's clock can be held still",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim99"}},
		}},
	{N: 100, Block: "g09-values", Name: "an out-parameter a value, a struct local its members",
		Steps: []Step{
			{Op: "localout"},
			{Op: "structscalar"},
		}},
	{N: 101, Block: "g10-plain-c", Name: "gettext's identity not called, the ASCII tests named, constant ifs their branch",
		Steps: []Step{
			{Op: "identity"},
			{Op: "asciiclass"},
			{Op: "constbranch"},
		}},
	{N: 102, Block: "g11-bool", Name: "a file-scope flag is bool",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim102"}},
		}}, {N: 103, Name: "more flags are bool",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim103"}},
		}},
}
