// Package build runs the pipeline: slim-vim.c in, whim-vim.c out, in one
// process and in memory.
//
// THE PLAN IS THE PIPELINE.  Each phase is a sequence of named steps
// (internal/steps) and, after the last of them, a sweep where the schedule put
// one.  That is everything a phase does to the source; the rest of what a phase
// program did -- building the binary its check measures, snapshotting symbols,
// writing a state directory -- is the check's, and a build does none of it.
//
// It was derived from the 163 phase programs and the stage schedule they ran
// under, and it is held to them by the only gate that matters: the product.  A
// build from the committed slim-vim.c must be the committed whim-vim.c, byte
// for byte.  A step in the wrong order, a missing sweep or a dropped argument
// moves those bytes, so the table is checked by `whimtools build --check` and
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
//	           which is what the phase declares in phase/080/delta.
package build

// A Step is one call: an op in internal/steps and its arguments.
type Step struct {
	Op       string
	Args     []string
	Declared bool // the phase's declared tokens go in REMOVED (phase 80)
}

// A Phase is what one phase of the pipeline does to the source, and what its
// check was handed beside it.
//
// The four State fields are the whole of what a phase program built for its
// check: the source it was handed, that source compiled, the enumerator values
// of it, and -- once -- a directory holding the source.  A build writes none of
// them; internal/verify writes exactly these.
type Phase struct {
	N        int
	Name     string
	Seed     bool // phase 0: the tree is the input, copied
	NoSource bool // the phase changes no source at all (83, 86, 116, 123)
	Steps    []Step
	Sweep    bool // a sweep follows this phase's steps

	Makefile string // what this phase does to the work tree's makefile:
	//	"whim"  tools/templates/whim.mk, the line phase 0 starts from
	//	"core"  tools/templates/core.mk, the line phase 83 writes over it
	//	"+flag" appended to CFLAGS (phase 84, -fno-stack-protector)

	Stage string // the unit this phase was verified in, from phase/STAGES.md
	Each  bool   // its stage sweeps after every edit and checks each tree

	OldSource bool   // state/old.c: the source the phase was handed
	OldBinary string // state/old: that source built -- "epoch", "plain" or "fixed"
	EnumVals  bool   // state/enums-before: its enumerator values
	OldDir    bool   // state/old/whim-vim.c: the same source, in a directory
}

// CoreFrom is the line between the two arcs: from this phase on, a delta is
// measured against .reference/core-baselines and with the core's instrument.
const CoreFrom = 83

// Plan is the pipeline, phase by phase.
var Plan = []Phase{
	{N: 0, Name: "seed, and prove the copy is a copy", Seed: true, NoSource: true, Makefile: "whim", Stage: "0"},
	{N: 1, Name: "no `$VIMRUNTIME`",
		Steps: []Step{
			{Op: "noruntime"},
		}, Stage: "1-12"},
	{N: 2, Name: "the options for features that are not here",
		Steps: []Step{
			{Op: "query-empty", Args: []string{"whim2"}},
			{Op: "dropoptions", Args: []string{"spell", "spellcapcheck", "spellfile", "spelllang", "spelloptions", "spellsuggest", "menuitems"}},
		}, Stage: "1-12"},
	{N: 3, Name: "no introduction, and the command line says only what the editor still decides",
		Steps: []Step{
			{Op: "nointro"},
			{Op: "dropopts", Args: []string{"-h", "-?", "-A", "-F", "-H", "-g", "-f", "-X", "-Y", "-d", "-U", "-l", "-C", "-N", "-n", "-p", "-V", "--help", "--version", "--clean", "--literal", "--nofork", "--noplugin", "--not-a-term", "--gui-dialog-file", "--startuptime", "--log"}},
			{Op: "optreaders"},
		}, Stage: "1-12"},
	{N: 4, Name: "the binary's name stops choosing what it does",
		Steps: []Step{
			{Op: "noargv0"},
		}, Stage: "1-12"},
	{N: 5, Name: "one regexp engine, not two",
		Steps: []Step{
			{Op: "nonfa"},
			{Op: "dropoptions", Args: []string{"regexpengine"}},
			{Op: "funcreach", Args: []string{"--delete"}},
		}, Stage: "1-12"},
	{N: 6, Name: "the editor stops writing shell scripts, and stops drawing a menu",
		Steps: []Step{
			{Op: "nowild"},
			{Op: "nowildmenu"},
			{Op: "dropoptions", Args: []string{"wildmenu"}},
		}, Stage: "1-12"},
	{N: 7, Name: "the editor stops looking for files it was not given",
		Steps: []Step{
			{Op: "noglob"},
			{Op: "retire", Args: []string{"cd", "chdir", "lcd", "lchdir", "tcd", "tchdir", "pwd"}},
		}, Stage: "1-12"},
	{N: 8, Name: "`:!` keeps its name and loses its process",
		Steps: []Step{
			{Op: "noshellout"},
		}, Stage: "1-12"},
	{N: 9, Name: "the editor stops asking the environment what language it is in",
		Steps: []Step{
			{Op: "nolocale"},
			{Op: "retire", Args: []string{"language"}},
			{Op: "dropoptions", Args: []string{"langmap", "langmenu", "langnoremap", "langremap"}},
		}, Stage: "1-12"},
	{N: 10, Name: "no tag stack",
		Steps: []Step{
			{Op: "notags"},
			{Op: "retire", Args: []string{"tag", "tags", "tNext", "tfirst", "tjump", "tlast", "tnext", "tprevious", "trewind", "tselect", "stag", "stjump", "stselect", "ltag", "pop"}},
			{Op: "dropoptions", Args: []string{"tagbsearch", "taglength", "tagrelative", "tagstack", "tagsecure", "showfulltag"}},
		}, Stage: "1-12"},
	{N: 11, Name: "nothing is written that was not asked for",
		Steps: []Step{
			{Op: "noswap"},
			{Op: "retire", Args: []string{"recover", "preserve", "swapname", "mkvimrc", "mkexrc", "mksession", "mkview", "checktime"}},
			{Op: "dropoptions", Args: []string{"updatecount", "swapsync"}},
		}, Stage: "1-12"},
	{N: 12, Name: "UTF-8, and no other encoding, ever",
		Steps: []Step{
			{Op: "noenc"},
			{Op: "dropoptions", Args: []string{"--strict", "charconvert"}},
		}, Sweep: true, Stage: "1-12"},
	{N: 13, Name: "the editor stops re-reading a file it has already read",
		Steps: []Step{
			{Op: "nostat"},
		}, Stage: "13-41"},
	{N: 14, Name: "a file name means the file of that name",
		Steps: []Step{
			{Op: "nofind"},
			{Op: "retire", Args: []string{"find", "sfind", "tabfind"}},
		}, Stage: "13-41"},
	{N: 15, Name: "the last two encoding options",
		Steps: []Step{
			{Op: "nofencs"},
			{Op: "dropoptions", Args: []string{"--strict", "fileencodings", "termencoding"}},
		}, Stage: "13-41"},
	{N: 16, Name: "six options that no longer decide anything",
		Steps: []Step{
			{Op: "noinertopts"},
			{Op: "dropoptions", Args: []string{"--local", "path", "suffixesadd", "tags", "tagcase", "autoread", "swapfile"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_path", "b_p_sua", "b_p_tags", "b_p_tc", "b_p_ar", "b_p_swf"}},
		}, Stage: "13-41"},
	{N: 17, Name: "the last two per-buffer encoding options",
		Steps: []Step{
			{Op: "nofenc"},
			{Op: "dropoptions", Args: []string{"--local", "fileencoding", "bomb"}},
			{Op: "droplocal", Args: []string{"b_p_fenc", "b_p_bomb"}},
		}, Stage: "13-41"},
	{N: 18, Name: "nothing is read at startup, and nothing on the command line decides anything",
		Steps: []Step{
			{Op: "nostartup"},
			{Op: "dropoptions", Args: []string{"--strict", "exrc"}},
			{Op: "dropopts", Args: []string{"-y", "-Z", "-u"}},
			{Op: "nocmdopts"},
			{Op: "dropoptions", Args: []string{"--strict", "viminfo", "viminfofile"}},
		}, Stage: "13-41"},
	{N: 19, Name: "the terminal is what the build says",
		Steps: []Step{
			{Op: "noterm"},
		}, Stage: "13-41"},
	{N: 20, Name: "nothing outside the process is consulted",
		Steps: []Step{
			{Op: "nohome"},
			{Op: "nogetenv"},
		}, Stage: "13-41"},
	{N: 21, Name: "there is nothing to recover, and the memfile is memory",
		Steps: []Step{
			{Op: "norecover"},
			{Op: "nomemfile"},
			{Op: "sweep"},
			{Op: "dropoptions", Args: []string{"--strict", "directory", "maxmem", "maxmemtot"}},
		}, Stage: "13-41"},
	{N: 22, Name: "the working directory is where it started",
		Steps: []Step{
			{Op: "nochdir"},
		}, Stage: "13-41"},
	{N: 23, Name: "no floating-point library",
		Steps: []Step{
			{Op: "nofloat"},
		}, Stage: "13-41"},
	{N: 24, Name: "there is no mouse",
		Steps: []Step{
			{Op: "nomouse"},
			{Op: "sweep"},
			{Op: "dropoptions", Args: []string{"--strict", "mouse", "mousefocus", "mousehide", "mousemodel", "mousemoveevent", "mouseshape", "mousetime", "ttymouse"}},
		}, Stage: "13-41"},
	{N: 25, Name: "a write is a write, and nobody owns it",
		Steps: []Step{
			{Op: "nobackup"},
			{Op: "dropoptions", Args: []string{"--local", "--strict", "backup", "backupcopy", "backupdir", "backupext", "backupskip", "patchmode", "writebackup"}},
			{Op: "noowner"},
			{Op: "droplocal", Args: []string{"b_p_bkc"}},
		}, Stage: "13-41"},
	{N: 26, Name: "five signals, not twenty-one",
		Steps: []Step{
			{Op: "nosignals"},
		}, Stage: "13-41"},
	{N: 27, Name: "`[[=a=]]` stops meaning \"a with any accent\"",
		Steps: []Step{
			{Op: "noequiclass"},
		}, Stage: "13-41"},
	{N: 28, Name: "C indenting",
		Steps: []Step{
			{Op: "nocindent"},
			{Op: "dropoptions", Args: []string{"--local", "cindent", "cinkeys", "cinoptions", "cinscopedecls", "cinwords"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_cin", "b_p_cink", "b_p_cino", "b_p_cinsd", "b_p_cinw"}},
		}, Stage: "13-41"},
	{N: 29, Name: "`:command`, user-defined commands",
		Steps: []Step{
			{Op: "noucmd"},
			{Op: "retire", Args: []string{"command", "comclear", "delcommand"}},
		}, Stage: "13-41"},
	{N: 30, Name: "`K` and the tag jumps, keeping `*` and `#`",
		Steps: []Step{
			{Op: "noident"},
		}, Stage: "13-41"},
	{N: 31, Name: "file-name modifiers",
		Steps: []Step{
			{Op: "nofnamemod"},
		}, Stage: "13-41"},
	{N: 32, Name: "insert completion, the popup menu, and the keys that reached them",
		Steps: []Step{
			{Op: "nocompl"},
			{Op: "dropoptions", Args: []string{"--local", "autocomplete", "complete", "completefunc", "completeopt", "dictionary", "infercase", "pumborder", "pummaxwidth", "pumopt", "pumheight", "pumwidth", "thesaurus"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_cpt", "b_p_cot", "b_p_dict", "b_p_tsr", "b_p_inf", "b_p_ac"}},
			{Op: "nocomplkeys"},
		}, Stage: "13-41"},
	{N: 33, Name: "commands whose machinery has already gone",
		Steps: []Step{
			{Op: "retire", Args: []string{"shell", "gui", "gvim", "cdo", "cfdo", "ldo", "lfdo", "vim9cmd", "endclass", "endinterface", "endenum", "public", "static", "this", "digraphs", "redrawtabpanel", "colorscheme"}},
			{Op: "edit", Args: []string{"whim33"}},
		}, Stage: "13-41"},
	{N: 34, Name: "no abbreviations",
		Steps: []Step{
			{Op: "retire", Args: []string{"abbreviate", "noreabbrev", "unabbreviate", "abclear", "iabbrev", "inoreabbrev", "iunabbrev", "iabclear", "cabbrev", "cnoreabbrev", "cunabbrev", "cabclear"}},
			{Op: "noabbr"},
		}, Stage: "13-41"},
	{N: 35, Name: "no scripts, no session, no autocommands",
		Steps: []Step{
			{Op: "retire", Args: []string{"source", "redir", "sleep", "smile", "scriptencoding", "scriptversion", "vim9script", "legacy", "autocmd", "augroup", "doautocmd", "doautoall", "noautocmd", "sandbox", "filetype", "setfiletype"}},
			{Op: "nosession"},
			{Op: "dropoptions", Args: []string{"eventignore"}},
			{Op: "dropoptions", Args: []string{"--local", "eventignorewin"}},
			{Op: "dropoptions", Args: []string{"--strict", "sessionoptions", "viewoptions", "viewdir", "loadplugins"}},
		}, Stage: "13-41"},
	{N: 36, Name: "one tab page, always",
		Steps: []Step{
			{Op: "retire", Args: []string{"tab", "tabclose", "tabdo", "tabedit", "tabfirst", "tabmove", "tablast", "tabnext", "tabnew", "tabonly", "tabprevious", "tabNext", "tabrewind", "tabs", "redrawtabline"}},
			{Op: "notabs"},
			{Op: "dropoptions", Args: []string{"tabclose"}},
			{Op: "dropoptions", Args: []string{"--strict", "showtabline", "tabline", "tabpagemax"}},
		}, Stage: "13-41"},
	{N: 37, Name: "no command that does nothing",
		Steps: []Step{
			{Op: "retire", Args: []string{"browse", "confirm", "tmap", "tnoremap", "tunmap", "tmapclear", "winpos", "behave", "mode", "open"}},
			{Op: "noinert"},
		}, Stage: "13-41"},
	{N: 38, Name: "the argument list is walked by `:next` and `:previous` alone",
		Steps: []Step{
			{Op: "retire", Args: []string{"args", "argglobal", "arglocal", "argadd", "argdelete", "argdedupe", "argedit", "argument", "sargument", "first", "sfirst", "rewind", "srewind", "last", "slast", "snext", "wnext", "Next", "sNext", "sprevious", "wNext", "wprevious", "all", "sall", "argdo"}},
			{Op: "noarglist"},
		}, Stage: "13-41"},
	{N: 39, Name: "one window, always",
		Steps: []Step{
			{Op: "retire", Args: []string{"split", "vsplit", "new", "vnew", "sview", "close", "only", "resize", "wincmd", "windo", "syncbind", "hide", "sbuffer", "sbNext", "sball", "sbfirst", "sblast", "sbmodified", "sbnext", "sbprevious", "sbrewind", "ball", "unhide", "sunhide", "aboveleft", "leftabove", "belowright", "rightbelow", "topleft", "botright", "vertical", "horizontal"}},
			{Op: "nowindows"},
			{Op: "dropoptions", Args: []string{"switchbuf", "scrollopt", "cmdwinheight", "cedit"}},
			{Op: "dropoptions", Args: []string{"--local", "scrollbind", "cursorbind", "winfixbuf"}},
			{Op: "dropoptions", Args: []string{"--strict", "previewheight"}},
			{Op: "dropoptions", Args: []string{"--strict", "--local", "previewwindow"}},
		}, Stage: "13-41"},
	{N: 40, Name: "no window sizes to set",
		Steps: []Step{
			{Op: "nowinsizes"},
			{Op: "dropoptions", Args: []string{"splitbelow", "splitright", "splitkeep", "equalalways", "eadirection", "winheight", "winminheight", "winwidth", "winminwidth", "helpheight"}},
			{Op: "dropoptions", Args: []string{"--local", "winfixheight", "winfixwidth"}},
		}, Stage: "13-41"},
	{N: 41, Name: "the buffer list is walked by `:bnext` and `:bprevious` alone",
		Steps: []Step{
			{Op: "retire", Args: []string{"buffer", "buffers", "files", "ls", "badd", "balt", "bdelete", "bunload", "bwipeout", "bfirst", "brewind", "blast", "bmodified", "bNext", "bufdo"}},
			{Op: "nobuflist"},
		}, Sweep: true, Stage: "13-41"},
	{N: 42, Name: "one buffer, always",
		Steps: []Step{
			{Op: "retire", Args: []string{"bnext", "bprevious", "keepalt"}},
			{Op: "onebuffer"},
			{Op: "dropoptions", Args: []string{"hidden"}},
			{Op: "dropoptions", Args: []string{"--local", "bufhidden"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_bh"}},
		}, Stage: "42-63"},
	{N: 43, Name: "no -c, --cmd, -R, -m, -M or -w",
		Steps: []Step{
			{Op: "nocmdargs"},
		}, Stage: "42-63"},
	{N: 44, Name: "no filters, sorting or alignment",
		Steps: []Step{
			{Op: "retire", Args: []string{"!", "sort", "uniq", "retab", "left", "center", "right"}},
			{Op: "edit", Args: []string{"whim44"}},
		}, Stage: "42-63"},
	{N: 45, Name: "no `:drop`",
		Steps: []Step{
			{Op: "retire", Args: []string{"drop"}},
		}, Stage: "42-63"},
	{N: 46, Name: "no `:wall`, `:qall`, `:quitall`, `:wqall` or `:xall`",
		Steps: []Step{
			{Op: "retire", Args: []string{"wall", "qall", "quitall", "wqall", "xall"}},
		}, Stage: "42-63"},
	{N: 47, Name: "no `:startinsert`, `:startreplace`, `:startgreplace` or `:stopinsert`",
		Steps: []Step{
			{Op: "retire", Args: []string{"startinsert", "startreplace", "startgreplace", "stopinsert"}},
		}, Stage: "42-63"},
	{N: 48, Name: "no `:noswapfile`",
		Steps: []Step{
			{Op: "retire", Args: []string{"noswapfile"}},
			{Op: "edit", Args: []string{"whim48"}},
		}, Stage: "42-63"},
	{N: 49, Name: "one set of options",
		Steps: []Step{
			{Op: "retire", Args: []string{"setlocal", "setglobal"}},
			{Op: "oneoptset"},
			{Op: "dropoptions", Args: []string{"--local", "modeline"}},
			{Op: "dropoptions", Args: []string{"modelines", "modelineexpr", "modelinestrict"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_ml"}},
		}, Stage: "42-63"},
	{N: 50, Name: "only LF text files",
		Steps: []Step{
			{Op: "dropopts", Args: []string{"-b"}},
			{Op: "lfonly"},
			{Op: "dropoptions", Args: []string{"--local", "binary", "fileformat", "endofline", "fixendofline", "endoffile", "textmode"}},
			{Op: "dropoptions", Args: []string{"fileformats", "textauto"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_bin", "b_p_ff", "b_p_fixeol", "b_p_tx"}},
		}, Stage: "42-63"},
	{N: 51, Name: "a byte that is not UTF-8 is kept as it is",
		Steps: []Step{
			{Op: "keepbytes"},
		}, Stage: "42-63"},
	{N: 52, Name: "UTF-8 is not a question",
		Steps: []Step{
			{Op: "utf8only"},
		}, Stage: "42-63"},
	{N: 53, Name: "no conversion layer, no 'encoding'",
		Steps: []Step{
			{Op: "noconv"},
			{Op: "dropoptions", Args: []string{"encoding"}},
			{Op: "dropoptions", Args: []string{"--local", "makeencoding"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_menc"}},
		}, Stage: "42-63"},
	{N: 54, Name: "no option without a variable",
		Steps: []Step{
			{Op: "query-dropoptions", Args: []string{"whim54", "100"}},
		}, Stage: "42-63"},
	{N: 55, Name: "no option nothing reads",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim55"}},
			{Op: "dropoptions", Args: []string{"autocompletetimeout", "cdhome", "cdpath", "completetimeout", "imcmdline", "secure", "shellcmdflag", "shelltemp", "shellxescape", "shellxquote", "ttybuiltin", "warn", "xtermcodes", "completefuzzycollect", "completeitemalign", "helpfile", "operatorfunc", "t_8b", "t_8f", "t_EC", "t_EI", "t_GP", "t_RB", "t_RC", "t_RF", "t_RS", "t_SC", "t_SH", "t_SI", "t_SR", "t_WP", "t_XM", "t_u7"}},
			{Op: "dropoptions", Args: []string{"--local", "shortname", "commentstring", "lispoptions"}},
			{Op: "droplocal", Args: []string{"b_p_sn", "b_p_cms", "b_p_lop"}},
		}, Stage: "42-63"},
	{N: 56, Name: "no shell, runtime or keyword-program options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim56"}},
			{Op: "dropoptions", Args: []string{"shell", "shellquote", "shellredir", "runtimepath", "packpath"}},
			{Op: "dropoptions", Args: []string{"--local", "keywordprg"}},
			{Op: "edit", Args: []string{"whim56kp"}},
			{Op: "droplocal", Args: []string{"b_p_kp"}},
		}, Stage: "42-63"},
	{N: 57, Name: "no lisp",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim57"}},
			{Op: "dropoptions", Args: []string{"--local", "lisp", "lispwords"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_lisp", "b_p_lw"}},
		}, Stage: "42-63"},
	{N: 58, Name: "no language mappings",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim58"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
			{Op: "dropoptions", Args: []string{"--local", "iminsert", "imsearch"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_iminsert", "b_p_imsearch"}},
		}, Stage: "42-63"},
	{N: 59, Name: "no command-line completion",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim59"}},
			{Op: "dropoptions", Args: []string{"wildchar", "wildcharm", "wildmode", "wildoptions", "wildignore", "wildignorecase"}},
		}, Stage: "42-63"},
	{N: 60, Name: "no suffix, case, delay, verbose-file, debug or filter-program options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim60"}},
			{Op: "dropoptions", Args: []string{"suffixes", "fileignorecase", "autocompletedelay", "verbosefile", "debug"}},
			{Op: "dropoptions", Args: []string{"--local", "formatprg", "equalprg"}},
			{Op: "sweep"},
			{Op: "edit", Args: []string{"whim60ep"}},
			{Op: "droplocal", Args: []string{"b_p_fp", "b_p_ep"}},
		}, Stage: "42-63"},
	{N: 61, Name: "no window title",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim61"}},
			{Op: "dropoptions", Args: []string{"title", "titlelen", "titleold", "titlestring", "icon", "iconstring"}},
		}, Stage: "42-63"},
	{N: 62, Name: "no buffer-type, file-type, listing, jump, update-time or autowrite options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim62"}},
			{Op: "dropoptions", Args: []string{"jumpoptions", "updatetime", "autowrite", "autowriteall"}},
			{Op: "dropoptions", Args: []string{"--local", "buflisted", "buftype", "filetype"}},
			{Op: "sweep"},
			{Op: "edit", Args: []string{"whim62bl"}},
			{Op: "droplocal", Args: []string{"b_p_bt", "b_p_ft"}},
		}, Stage: "42-63"},
	{N: 63, Name: "no jump list",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim63"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Sweep: true, Stage: "42-63"},
	{N: 64, Name: "no formatting, comment or nroff-macro options",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim64"}},
			{Op: "dropoptions", Args: []string{"paragraphs", "sections"}},
			{Op: "dropoptions", Args: []string{"--local", "formatoptions", "formatlistpat", "comments"}},
			{Op: "sweep"},
			{Op: "droplocal", Args: []string{"b_p_fo"}},
			{Op: "droplocal", Args: []string{"b_p_flp"}},
			{Op: "droplocal", Args: []string{"b_p_com"}},
		}, Stage: "64-65"},
	{N: 65, Name: "no rot13, no operator function, no empty key handler",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim65"}},
		}, Sweep: true, Stage: "64-65"},
	{N: 66, Name: "no sentences, paragraphs, sections, methods, #if blocks or comment blocks",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim66"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "66-71"},
	{N: 67, Name: "no mouse, no spell plumbing, no write-only flags",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim67"}},
		}, Stage: "66-71"},
	{N: 68, Name: "one window, structurally",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim68"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "66-71"},
	{N: 69, Name: "one file argument, and no argument list",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim69"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "66-71"},
	{N: 70, Name: ":e reloads in place, and there is no swap file",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim70"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "66-71"},
	{N: 71, Name: "one buffer, structurally",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim71"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Sweep: true, Stage: "66-71"},
	{N: 72, Name: "one window, one tabpage, structurally",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim72"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Sweep: true, Stage: "72"},
	{N: 73, Name: "one frame",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim73"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "73-77"},
	{N: 74, Name: "no file marks",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim74"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "73-77"},
	{N: 75, Name: "no autocommands",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim75"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "73-77"},
	{N: 76, Name: "one regexp engine, so no retry",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim76"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Stage: "73-77"},
	{N: 77, Name: "no buffer-name argument matching",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim77"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Sweep: true, Stage: "73-77"},
	{N: 78, Name: "empty functions, write-only counters, and the window id",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim78"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Sweep: true, Stage: "78"},
	{N: 79, Name: "the constant-return predicates",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim79"}},
			{Op: "cmdidxs", Args: []string{"--check", ">/dev/null"}},
		}, Sweep: true, Stage: "79"},
	{N: 80, Name: "the Ex command table, cut to the commands that exist",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim80", "@state/words"}, Declared: true},
		}, Sweep: true, Stage: "80", OldSource: true, OldBinary: "fixed"},
	{N: 81, Name: "one line, one command",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim81"}},
		}, Sweep: true, Stage: "81", OldSource: true, OldBinary: "fixed"},
	{N: 82, Name: "the system headers nothing needs, and every comment",
		Steps: []Step{
			{Op: "includes", Args: []string{"@state"}},
			{Op: "edit", Args: []string{"whim82"}},
		}, Sweep: true, Stage: "82", OldDir: true},
	{N: 83, Name: "the core's compile line, and the baselines it is measured against", NoSource: true, Makefile: "core", Stage: "83"},
	{N: 84, Name: "the stack protector goes", NoSource: true, Makefile: "+-fno-stack-protector", Stage: "84"},
	{N: 85, Name: "the core stops diagnosing its own terminal",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim85"}},
		}, Sweep: true, Stage: "85", OldSource: true, OldBinary: "plain"},
	{N: 86, Name: "the instrument becomes the screen", NoSource: true, Stage: "86"},
	{N: 87, Name: "no streaming Ex",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim87"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 88, Name: "argv is `+{command}` and `-T {term}`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim88"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain", EnumVals: true},
	{N: 89, Name: "no write",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim89"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 90, Name: "no read",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim90"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 91, Name: "no `:edit`, and no `gf`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim91"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 92, Name: "nothing reads a byte",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim92"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 93, Name: "the buffer has no name",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim93"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 94, Name: "`:q` quits, and `ZZ` is `ZQ`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim94"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 95, Name: "the options nothing reads",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim95"}},
			{Op: "dropoptions", Args: []string{"--strict", "prompt", "undoreload", "write", "writeany"}},
			{Op: "droplocal", Args: []string{"b_p_fs"}},
			{Op: "dropoptions", Args: []string{"--strict", "--local", "fsync"}},
			{Op: "dropoptions", Args: []string{"--strict", "--local", "readonly"}},
			{Op: "droplocal", Args: []string{"b_p_ro"}},
			{Op: "edit", Args: []string{"whim95rows"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 96, Name: "no `FILE *` that is never opened",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim96"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 97, Name: "the strings are the editor's own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim97"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 98, Name: "the character classes, the numbers and the sort",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim98", "tools/musl-ctype.txt", "tools/musl-case.txt"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 99, Name: "the includes nothing names",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim99"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 100, Name: "the deadly ladder that cannot run",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim100"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 101, Name: "`main()` is demoted to `vim_main()`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim101"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 102, Name: "the core can no longer stop the process",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim102"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 103, Name: "the signals and the terminal are the host's",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim103"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 104, Name: "the messages are the editor's, the writing is the host's",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim104"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 105, Name: "the variadic collapse",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim105"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 106, Name: "`nullptr` and `usize`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim106"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 107, Name: "the attributes",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim107"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 108, Name: "the plain host calls",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim108"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 109, Name: "the header types and macros the core can own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim109", "@minmax"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 110, Name: "the move: the first `#include` becomes the boundary",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim110", "@state"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 111, Name: "the scalar clock",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim111"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 112, Name: "the case tables become one, and it is the union",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim112"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "plain"},
	{N: 113, Name: "the message fold: `msg_puts_printf()` and the branch that reaches it",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim113"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 114, Name: "`abs` and `labs`, the two the core took on trust",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim114"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 115, Name: "the clock crosses the boundary",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim115"}},
		}, Sweep: true, Stage: "87-115", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 116, Name: "the terminal table is asked with `+set term=`, not `$TERM`", NoSource: true, Stage: "116"},
	{N: 117, Name: "the core stops reallocating",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim117"}},
		}, Sweep: true, Stage: "117-122", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 118, Name: "the core calls nothing but the host",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim118", "@state"}},
		}, Sweep: true, Stage: "117-122", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 119, Name: "the core names no libc function at all",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim119", "@state"}},
		}, Sweep: true, Stage: "117-122", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 120, Name: "the degenerate unions go",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim120"}},
		}, Sweep: true, Stage: "117-122", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 121, Name: "the eight terminal names go, leaving two",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim121"}},
		}, Sweep: true, Stage: "117-122", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 122, Name: "`-T {term}` goes, and the command line is `+{command}`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim122"}},
		}, Sweep: true, Stage: "117-122", Each: true, OldSource: true, OldBinary: "epoch", EnumVals: true},
	{N: 123, Name: "the instrument could not see the text layer", NoSource: true, Stage: "123"},
	{N: 124, Name: "freeing is free, and the arena is measured",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim124", "@state"}},
		}, Sweep: true, Stage: "124-128", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 125, Name: "the swap file's residue, and what no sweep could find",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim125", "@state"}},
		}, Sweep: true, Stage: "124-128", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 126, Name: "a block number becomes a reference",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim126", "@state"}},
		}, Sweep: true, Stage: "124-128", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 127, Name: "de-page the leaf",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim127", "@state"}},
		}, Sweep: true, Stage: "124-128", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 128, Name: "fold the node types",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim128", "@state"}},
		}, Sweep: true, Stage: "124-128", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 129, Name: "`p_emoji` is an `int`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim129"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 130, Name: "the `(pos_T *)-1` tests go",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim130"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 131, Name: "the saved input buffer is a `garray_T *`",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim131"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 132, Name: "nothing frees",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim132"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 133, Name: "one buffer needs no hash table",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim133"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 134, Name: "the empty blocks fold",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim134"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 135, Name: "one regexp program type",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim135"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 136, Name: "the engine is called directly",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim136"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 137, Name: "the changedtick is a number",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim137"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 138, Name: "no parameter carries an eval value",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim138"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 139, Name: "the core sorts and searches typed arrays",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim139"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 140, Name: "highlight groups are found in their array",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim140"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 141, Name: "`regrepeat()` does not jump into a case",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim141"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 142, Name: "the version names no build date or time",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim142"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 143, Name: "`regatom()` has no goto",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim143"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 144, Name: "`edit()` has no goto",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim144"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 145, Name: "`check_termcode()` has no goto",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim145"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 146, Name: "a memline node names its block",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim146"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 147, Name: "`deathtrap()` runs at the host's next wait",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim147"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 148, Name: "allocation cannot fail",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim148"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 149, Name: "the allocation-failure branches fold",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim149"}},
		}, Sweep: true, Stage: "129-149", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 150, Name: "the regexp stack is three typed stacks",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim150"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 151, Name: "the option table's defaults are typed",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim151"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 152, Name: "the option variables are typed",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim152"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 153, Name: "`free_one_termoption()` compares without a cast",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim153"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 154, Name: "the NULL write in `free_one_termoption()` is gone",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim154"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 155, Name: "call arguments with effects are evaluated in gcc's order",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim155"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 156, Name: "the regex size pass's node is a static byte, not (char_u *) -1",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim156"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 157, Name: "get_register() and put_register() carry a yankreg_T *, not a void *",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim157"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 158, Name: "a highlight's terminal font is read only from a colour entry",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim158"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 159, Name: "a struct's text is a pointer to an allocation of its own",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim159"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 160, Name: "no line getter takes a cookie",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim160"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 161, Name: "no goto jumps into a block",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim161"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
	{N: 162, Name: "no two function pointers are compared",
		Steps: []Step{
			{Op: "edit", Args: []string{"whim162"}},
		}, Sweep: true, Stage: "150-162", Each: true, OldSource: true, OldBinary: "epoch"},
}
