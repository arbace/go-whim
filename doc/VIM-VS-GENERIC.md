# Vim-specific or generic C: a survey of go-whim's phases and machinery

**Done since:** the separation of §4, all nine steps, 2026-09-25, each step
byte-identical under `whim-build-check` (and gen's under `whim-editor-check` and
`whim test`). The library is `crefactor/`, a Go module of its own
(`github.com/arbace/go-whim/crefactor`, required by this one through a `replace`
to `./crefactor`), so it cannot import whim's code. It holds `cc` (the forked C
front end), `cemit`, `sweep`, `pipeline` (the driver, told through a Config),
`edit` (cutil and the verb set, first called `text`), `xform` (the eleven generic transformations;
164 and 168 share `terminates.go`) and `togo` (the C-to-Go translator). What it
is told about vim lives in `internal/whim` (`profile.go`, `xform.go`,
`analysis.go`, `gen.go`) and `internal/whim/vimtext`. The forwarding layers the move
left (`internal/edit`, `internal/cutil`, `internal/gen`'s wrapper) are retired:
the phases import `crefactor/edit`, `internal/whim/vimtext` and the registry,
`internal/phase`, by their own names. reach, ccx and dead take their names as options and
have since moved into the module too, reach's core/host cut and funcreach's
floor lifted into `whim.Reach` and `whim.Dead`; the phase-numbered helpers
the library carried (`W134*`) have descriptive names.

**Since written:** two side findings were checked. Its claim that CLAUDE.md still said 164 phases, no test suite and a `Sweep` field is wrong: CLAUDE.md says none of those. Its finding that the sweep documented an `ml_recover` guard it did not implement was right, and the guard is implemented now: phases 1-18 had been deleting struct members while the editor could still read a swap file. The throwaway instruments it names under `.tmp/survey/` are not tracked.

2026-09-25. This survey reads the code and changes nothing. It was measured on commit
`a368ee2` ("edit, cutil: Head, a fold's head spelled as the C it matches"),
exported with `git archive` to `.tmp/survey/head/`. The main checkout was being
edited during the survey (15 modified files in `internal/cut`, `internal/edit` and
`internal/phase`), and those uncommitted edits are **not** measured here.

## 0. What was measured, and how

- **Vim's vocabulary as a set.**
  - Every identifier token in `.cache/boundaries/q000.c` (the seed: canonical,
    no comments), with string and character literals blanked, minus every
    identifier in the preprocessed system headers slim-vim.c includes
    (`gcc -E -P` plus `gcc -dM -E`), minus C keywords. That gives **13,496 vim
    identifiers**.
  - A "distinctive" subset excludes short, plain-English names: an identifier
    enters it if it contains `_`, a digit or an upper-case letter, or has 7 or
    more characters. That gives **12,200**.
  - The same process on `src/whim-vim.c` gives the 335 names the pipeline itself
    introduced (`host_*`, `musl_*`, `K_*`, `usize`, `vim_main`...).
- **A go/ast scanner** (`.tmp/survey/scan/`). For every Go file, it lists the
  vim identifiers named inside string literals. Import paths and the format
  strings of `fmt`/`errors`/`log` calls (which are messages) are excluded. It
  produced the per-phase counts (`.tmp/survey/phases.tsv`) and the file:line
  lists in section 2, and grep filled the gaps. Hard-coded knowledge that is
  *not* a vim identifier (`"main"`, `"\n#include "`, the `musl_*` names) was
  found by grepping for the whim-introduced names and for bare identifier
  literals.
- **Every phase was read.**
  - Phases 129-169: GOAL.md and edit.go, read directly.
  - Phases 1-53, 54-100 and 101-128: three parallel read-only passes, each
    against a fixed rubric. Their citations were spot-checked. Two line
    references were wrong (`cut/retire.go`, `cut/droplocal.go`) and are
    corrected below.
- **The generic machinery was run on C that is not vim.** A throwaway driver
  (`.tmp/survey/head/cmd/survey`, in the exported copy, never in the checkout)
  runs `build.Seed` (cemit), the sweep, the `includes` step and the phase
  edits on any file. Two inputs were used:
  1. **AT&T `testregex.c`** (`/root/go-lisp/src/regexp/testdata/`, 2,286
     lines). It was "slimmed" the way slim-vim is: its own macros expanded
     against empty stand-ins for the system headers, and its 10 system
     `#include`s put back. That gives 1,598 lines, and it builds and passes
     122/122 tests on `basic.dat`, `nullsubexpr.dat` and `repetition.dat`.
  2. **A 40-line probe** (`.tmp/survey/foreign/mini.c`) with a jump followed
     by dead code, a `goto` to a `return`, and three yes/no functions.

  Behaviour was compared by building each result with gcc and diffing the
  test output.
- **Nothing in `.cache/boundaries` was written or read for writing.** No
  `make whim-build`, `whim-build-check` or full `whim build` was run.

## 1. Every phase, classified

**The classes.**
- **G** (generic): the edit computes its targets from C semantics. Vim appears
  at most as a parameter: a root name, a set of constants, a count floor.
- **M** (mixed): the phase is an instance of a nameable generic kernel (fold
  a constant, stub a dispatch entry, drop an always-null parameter, retype to
  the one type used, outline a jumped-into block...), with vim choosing the
  targets. For each M phase, the table says whether the kernel exists as code
  or is applied by hand-written anchors.
- **Mt**: a vim cutter from `internal/cut` plus the table ops (`retire`,
  `dropoptions`, `droplocal`, `dropopts`). Those ops have a generic mechanism
  written against vim's table layouts.
- **V** (vim-specific): the transformation rewrites vim's own semantics or
  data structures (a behaviour choice, the memline, the terminal table), and
  no kernel is worth naming beyond the text verbs.
- **N**: a NoSource phase, which changes no text.

| class | phases | count | Go lines in `phase/NNN/*.go` |
|---|---|---:|---:|
| G | 0 106 107 114 120 132 134 149 164 166 168 169 | **12** | 3,281 |
| M | 2 33 44-47 52-54 57 58 67 68 71-73 75 76 78 79 87 89-91 93-98 100-102 104 105 108-110 115 117-119 124 129-131 135 136 138 141 143-146 155-157 159-161 165 167 | **62** | 13,270 |
| Mt | 3 5-7 9-12 14-18 21 24 25 28 29 32 34-42 48-50 | **31** | 45 (their code is `internal/cut`, 9,889 lines) |
| V | 1 4 8 13 19 20 22 23 26 27 30 31 43 51 55 56 59-66 69 70 74 77 80 81 85 88 92 103 111-113 121 122 125-128 133 137 139 140 142 147 148 150-154 158 162 | **57** | 11,070 |
| N | 82 83 84 86 99 116 123 163 | **8** | 13 (a stale `082/editlit.go`) |

That is **170 phases** (0-169; `CLAUDE.md` still says 164). Twelve are generic
and 93 are mixed. Of the M phases, **about 15 carry their kernel as code**: 52,
53, 71, 72, 89-91, 95, 97, 98, 105, 110, 118 and 119, plus the `retire` op in 2
and 45-47. The other M phases apply their kernel through hand-written anchors
on vim's text.

### Phases 0-53 (Part I: cutters and table ops)

| # | class | why, and the vim knowledge encoded |
|---|---|---|
| 0 | G | seed: the canonical print (`build.Seed`, cemit) |
| 1 | V | `noruntime`: `$VIMRUNTIME` and `vim_getenv`'s derivation from argv[0] (cut/noruntime.go:13-34) |
| 2 | M | `query-empty whim2` asserts that 24 menu and spell `cmdnames[]` rows are already `ex_ni`; `dropoptions` for spell*/menuitems. Kernel: "these dispatch entries are stubs" plus "delete table rows" |
| 3 | Mt | `nointro` (`:intro`/`:version` rows, `maybe_intro_message`, cut/small.go); `dropopts` on 27 flags; `optreaders` |
| 4 | V | `noargv0`: argv[0] selecting the mode (cut/small.go:47-62) |
| 5 | Mt | `nonfa`: the NFA engine and `\%#=`; `regexpengine` |
| 6 | Mt | `nowild`, `nowildmenu`: shell delegations in `gen_expand_wildcards` |
| 7 | Mt | `noglob`; `retire cd lcd tcd pwd ...` |
| 8 | V | `noshellout` |
| 9 | Mt | `nolocale`: setlocale, the `enc_dbcs` block in `mb_init`; `retire language`; lang* options |
| 10 | Mt | `notags`: 7 entry points; 15 commands retired; tag* options |
| 11 | Mt | `noswap`: `ml_open_file` stubbed; recover/mk* retired |
| 12 | Mt | `noenc`: the `mb_init` dispatch and iconv in readfile/buf_write |
| 13 | V | `nostat`: `check_timestamps` returns 0 |
| 14-18 | Mt | `nofind`, `nofencs`, `noinertopts`, `nofenc`, `nostartup`, `nocmdopts` (scoped to `command_line_scan`), each with its options and `b_p_*` fields |
| 19, 20, 22 | V | `noterm` (`$TERM`/`$LINES`), `nohome`/`nogetenv` (`expand_env_esc`), `nochdir` |
| 21 | Mt | `norecover`, `nomemfile` (four replacement bodies) |
| 23 | V | `nofloat`. Its precondition is generic (no printf float conversion in any literal, cut/nofloat.go:17-78), but the cut is vim's formatter, `TYPE_FLOAT` and fzy |
| 24, 25, 28, 29, 32 | Mt | mouse; backup/owner; cindent; `:command`; completion (with `nocomplkeys`) |
| 26, 27, 30, 31 | V | 5 signals (`signal_info`); `[[=a=]]`; `K` and tag keys; `%` modifiers |
| 33 | M | `retire` of 17 commands, then whim33 folds `eap->cmdidx == CMD_cdo...` in `ex_listdo`. Kernel: fold tests of retired enum values |
| 34-42 | Mt | abbreviations, scripts/sessions, tab pages, inert commands, arglist, windows, window sizes, buffer list, one buffer: `retire` lists plus a cutter each |
| 43, 51 | V | `-c/--cmd/-R/-m/-M/-w`; `keepbytes` (`bad_char_behavior`) |
| 44 | M | `retire` 7, and the `nv_cmds[]` `'!'` row pointed at `nv_error` (phase/044/edit.go:24-35). Kernel: point a dispatch entry at a stub, applied to a second table |
| 45-47 | M | `retire` only (`:drop`; `:wall` and the rest; `:startinsert` and the rest) |
| 48-50 | Mt | `:noswapfile` (with a flag folded by hand); one option set; LF only (`lfonly`) |
| 52 | M | `utf8only`. **Generic kernel as code**: constant propagation of named globals (replace, delete the declaration and assignments, simplify, fold if/while), cut/utf8only.go:98-458. Parameters: `{enc_utf8, has_mbyte, enc_latin1like: 1; enc_dbcs, enc_unicode: 0}`, init function `mb_init`. Vim leaks into the kernel: `DBCS_\w+` (:38-39) and flag names in its regexes (:42-45) |
| 53 | M | `noconv`. **Generic kernel as code**: `directCall` devirtualises `(*ptr)(`/`ptr(` to `fn(` given a ptr→fn map (cut/noconv.go:17-23, 78-116). The rest is vim (`vimconv`, `CONV_NONE`) |

### Phases 54-100 (Part I end, Part II start)

| # | class | why / vim knowledge | kernel, and whether it exists as code |
|---|---|---|---|
| 54 | M | delete every `options[]` row whose variable is `(char_u *)NULL` | "delete rows of table T where field k is NULL"; the query is in the phase, the deletion is the vim-layout `dropoptions` |
| 55, 56 | V | hand-listed options and `t_` codes; shell, runtimepath and keywordprg readers | - |
| 57, 58 | M | `b_p_lisp` folded FALSE at about 15 hand anchors; `MODE_LANGMAP` never set, `:lmap` rows to `ex_ni` | "fold every test of V as K" (hand); "retarget rows to a stub" (inline regex) |
| 59-66 | V | completion, suffix and delay options, title, buffer-type options, jump list, formatting, rot13/`g@`, sentence and paragraph motions | several contain the retarget-row kernel, inline |
| 67 | M | mouse rows (V), plus write-only statics (G, list at phase/067/edit.go:68) | `edit.DeadStores` exists, but this phase does not use it |
| 68 | M | `one_window`/`last_window` return TRUE, then their callers are folded | "make F return K, fold callers" (hand) |
| 69, 70 | V | one file argument; `:e` reloads in place | - |
| 71, 72 | M | one buffer / window / tabpage: a walk over a one-element list becomes one binding | **as code**: `edit.FoldWalk`/`DropWalk`/`FoldWalks` (edit/blocks.go:31-157). `FoldWalk` hard-codes `curbuf` (blocks.go:54) |
| 73 | M | the frame tree is never linked | proved by a count of writes (phase/073/edit.go:57); bodies by hand |
| 74, 77 | V | file marks; `EX_BUFNAME` block | - |
| 75, 76 | M | `first_autopat[]` is only its initialiser; `re_engine` is assigned once | "a value with one constant assignment folds its tests"; `writesTo` (075) and `assignsTo` (076) are copies of each other |
| 78, 79 | M | empty functions, write-only counters, window id; 28 constant-return predicates | "delete bare calls to empty F" (inline, hand list at 078/edit.go:85); "fold callers of `return K` functions" (`ConstOf` only asserts; about 40 folds by hand) |
| 80, 81 | V | `cmdnames` cut to the live commands, with abbreviations re-encoded; one line is one command | - |
| 82, 83, 84, 86, 99 | N | comments (went to cemit); baselines; `-fno-stack-protector`; the screen instrument; headers (went to 169) | - |
| 85, 88, 92 | V | tty diagnosis; argv is `+cmd`/`-T`; `open_buffer`'s read arms | - |
| 87 | M | no Ex mode: mostly "fold global V as FALSE" | re-implements `inFunction`/`within` inline (087/edit.go:141,197) |
| 89-91 | M | no `:write`, `:read`, `:edit` | "delete table rows and their enumerators, check the residue" (`edit.CoreResidue`, edit/residue.go, as code) |
| 93, 94, 96, 100 | M | the buffer has no name (fold `F == NULL` TRUE); `:q` quits (a call-graph check, inline); `scriptin`/`redir_fd` always NULL; `deathtrap`'s bounded counter | hand folds |
| 95 | M | options whose global has no reader | computed inline (`Whim95Rows`, 095/edit.go:346); the exemptions are vim's |
| 97, 98 | M | vendor 16 musl string functions, ctype, `atoi`, `qsort`/`bsearch` | "libc symbol S becomes an in-file `musl_S`", parameterised by a list (as code, per phase) |

### Phases 101-128 (the embeddable core)

| # | class | why / vim knowledge | kernel and its parameters |
|---|---|---|---|
| 101 | M | `main` becomes `vim_main` plus a launcher | rename main to X and append a launcher; literal text at the tail, and pins `vim_main2` |
| 102 | M | `exit` becomes a host callback that longjmps | parameters: the exiting function (`mch_exit`) and the entry point |
| 103, 111-113 | V | signals/terminal to the host (about 50 literal ops in 103/editlit.go); the scalar clock; case tables merged with musl's; `msg_puts_printf` | - |
| 104 | M | stdio output becomes `host_message` | 20 literal sites |
| 105 | M | 7 variadic wrappers collapse at 129 call sites | the call-site rewriter (`closeParen`, `splitArgs`, statement vs comma-expression) is generic; wrapper, buffer and tail tables are vim's (`w105Wrap`/`w105Room`/`w105Lead`) |
| 106 | G | `NULL` to `nullptr`, `size_t` to `usize` | computed; only a 3-literal pre-check (`wantLits`) is vim's |
| 107 | G | `__attribute__((unused))` dropped, `[[fallthrough]]` | computed, on cemit's layout |
| 108, 109, 115 | M | callbacks to static host calls; header types and `MIN`/`MAX` (from a preprocessor probe, `@minmax`) and `offsetof`; the clock to `host_time` | names and anchors hard-coded |
| 110 | M | **the host boundary**: includes move down, and a gcc `-c -Wall` fixpoint moves what the core cannot compile | the most generic algorithm in Part II; the seed (`w110Variadic`), keep set, `host_winch_pending` anchor, `w110Consts` and a `len(enums) < 400` floor are vim's |
| 114 | G | `abs`/`labs` become in-file `musl_abs`/`musl_labs` | name, body and anchor (`musl_bsearch`) as parameters |
| 116, 123 | N | harness only | - |
| 117 | M | `realloc` becomes malloc+copy+free at 2 sites | old size per site; not generic in practice |
| 118, 119 | M | libc F becomes `host_F`: prototypes removed, calls renamed by partition | as code; the triples (118/editlit.go) and anchors are vim's; 119's `getpid`/`b0_pid` half is V |
| 120 | G | a degenerate union (fewer than 2 members) becomes its member | fully computed |
| 121, 122, 125-128 | V | terminal table to two rows; `-T`; the swap file's residue; memline block numbers, leaf and node types | - |
| 124 | M | host arena: free is a no-op, malloc a bump pointer | parameter: arena size; the mention partition is hard-coded |

### Phases 129-169 (what the Go transpilation had to work around, then idiom)

| # | class | why / vim knowledge | kernel, and its evidence |
|---|---|---|---|
| 129, 131 | M | `p_emoji` retyped to `int`; `save_inputbuf` to `garray_T *` | "retype to what every reader and writer uses" (hand) |
| 130, 156 | M | `(pos_T *)-1` tests folded; `regcode == (char_u *)-1` becomes `&reg_calc_size_node` | "a sentinel never produced folds", "an integer sentinel pointer becomes a static's address" (2 verbs each, hand) |
| 132 | G | every call to `vim_free`/`host_free` goes, keeping argument side effects; then `edit.DeadStores` | parameters: the no-op functions (`w132Call`, 132/edit.go:28) and a floor of 273 calls |
| 133, 137, 139, 140, 142, 147, 148, 150-154, 158 | V | one-buffer hash; changedtick; typed bsearch; highlight lookup; `__DATE__`; deathtrap self-pipe; allocation cannot fail; regstack; option defaults and varp typing; `free_one_termoption`; union discriminant | the finding may come from a generic analysis (ccx), but the edit is vim's |
| 134 | G | an empty block under a condition that only reads goes, alternating with `DeadStores` to a fixpoint | its check computes the whole output with the same rule and the real sweep; its floor ("30 or so") is vim's |
| 135, 146 | M | struct-prefix inheritance with one subtype is merged; header-plus-tag becomes a typed pointer | hand |
| 136 | M | calls through `bt_regengine`'s function pointers become direct calls | devirtualisation (hand; cf. 53's `directCall`) |
| 138, 160 | M | parameters every call passes `nullptr` are dropped | hand |
| 141, 143-145, 161 | M | a goto into a case or block is removed by outlining or restructuring | the finder is generic (`ccx.Gotos`); the rewrites are hand-written per function |
| 149 | G | a NULL test of a never-NULL allocation folds; the never-NULL set is found to a fixpoint | parameter: the roots, `W149Base = {"host_alloc"}` (149/edit.go:29); its premise is phase 148's behaviour choice |
| 155 | M | effectful call arguments get gcc's (right-to-left) order written out | finder generic (`ccx.Order`); 11 sites by hand; the order is gcc's, a target fact |
| 157, 159 | M | `void *` to `yankreg_T *`; the struct hack becomes a second allocation | hand (157 is the last cast outside `ccx.Casts`' classes) |
| 162 | V | a function-pointer comparison becomes the flag `DOCMD_GETEXLINE` | finder generic (`ccx.FuncCompares`); the design is vim's |
| 163 | N | the canonical print of the product (`cemit`) | - |
| 164 | G | statements after a jump, up to the next label, go | `cc.Parse` locates, text cuts; holds a run containing a declaration |
| 165 | M | six stores nothing reads | **the kind is generic, the code is not**: six hand-named sites (`open_line`, `edit`, `cmdline_handle_ctrl_bsl`, `next_search_hl`, `adjust_skipcol`, `do_put`). They are partial dead stores, which `DeadStores` (whole variables) cannot see |
| 166 | G | a function, local, member or parameter that only holds a yes/no answer is `bool`, and `== OK`/`!= FAIL` fold | parameters: the answers `TRUE/FALSE/OK/FAIL` (166/edit.go:305, 469-471, 819), `main` (:82, :122), the core/host cut `"\n#include "` (:31) |
| 167 | M | every constant key code gets a name (an enumerator before first use) | generic: "name a repeated constant expression, from a table in the file, with a mechanical fallback, and insert an enum before first use"; vim: the `TERMCAP2KEY` shape, the `key_names_table` row, `KS_EXTRA`/`KE_` (167/edit.go:20-25, 63) |
| 168 | G | a `goto` whose label marks only a `return` becomes that return; the label goes | holds when a name could be shadowed; duplicates 164's `stmtTerminates` |
| 169 | G | every system header the file does not need goes | asks gcc: a silent `-fsyntax-only -Wall -Wextra` compile, each line alone, then together, then one by one from the bottom |

**Measured on foreign C.**

| input | tool | result |
|---|---|---|
| testregex.c | cemit | 1,598 → 2,042 lines, fixpoint `true`; the binary passes the same 122/122 tests with identical output |
| testregex.c | sweep | 1 member removed, behaviour identical |
| testregex.c | 164, 168 | ran, found nothing to do, refused nothing |
| testregex.c | 166 | found nothing (it does not use named answers) |
| testregex.c | 134 | **refused**: "0 empty blocks fold; this phase was written against the 30 or so phase 132 leaves". The floor is vim's |
| testregex.c | 149 | **refused** the same way: the floor is vim's |
| testregex.c | 169 (`includes`) | **refused**: "the input does not compile silently". testregex has 32 `-Wall -Wextra` warnings |
| mini.c | 164 | cut the dead `printf` after `continue` |
| mini.c | 168 | turned `goto done` into `return i;` and dropped the label |
| mini.c | 166 | retyped `is_small` and `is_big` to `bool` and rewrote `is_big(300) == TRUE` to `is_big(300)`. It left `one_or_zero` (which returns the literal `1`/`0`) as `int`. The compiled output is identical before and after |
| mini.c | 166, file starting with `#include` | **refused**: "no #include: the core/host line is not where this phase expects it". `"\n#include "` needs a newline before it |
| mini.c | 166, with the core put above the include | worked |

## 2. The shared machinery, file by file

The classes here:
- **G**: generic.
- **Gv**: generic, with vim knowledge embedded at the lines given.
- **V**: vim-specific.

Each piece of embedded knowledge is given with its file:line and the parameter
it could become.

### internal/steps (65 ops, plus the `sweep` pseudo-op in `build`)

The plan uses 62 of the 65 ops. The unused three are `cemit` (reached through
`build.Seed`), `funcreach` and `query`.

| op(s) | class | vim knowledge, and how it could become a parameter |
|---|---|---|
| 53 plain cutters (steps.go:39-93): keepbytes lfonly noabbr noarglist noargv0 nobackup nobuflist nochdir nocindent nocmdargs nocmdopts nocompl nocomplkeys noconv noenc noequiclass nofenc nofencs nofind nofloat nofnamemod nogetenv noglob nohome noident noinert noinertopts nointro nolocale nomemfile nomouse nonfa noowner norecover noruntime nosession noshellout nosignals nostartup nostat noswap notabs notags noterm noucmd nowild nowildmenu nowindows nowinsizes onebuffer oneoptset optreaders utf8only | V | each is a scripted edit of vim (see `internal/cut`) |
| `dropoptions` | Gv | row start `{"%s",` anywhere in the file (cut/dropoptions.go:21); lookup functions `findoption\|set_string_option_direct\|set_option_value\w*\|option_was_set` (:22-23); `(char_u *)&var` (:24); `PV_\w+` (:25); the modeline whitelist `"name",\n` (:172). Parameters: a row-key regex, a lookup-function regex, a variable-field regex, a locality marker and secondary lists. **Not worth it** (section 5) |
| `droplocal` | Gv | `buf->X`, `curbuf->X = -1`, `check_/clear_string_option`, `get_varp`'s `case ... return (char_u *)&(curbuf->X)` (cut/droplocal.go:23-31). Kernel: "a struct member whose only uses are writes, frees and one escaping accessor" |
| `retire` | Gv | the row `[CMD_x] = {(char_u *)"x", sizeof("x") - 1, handler` and the stub `ex_ni` (cut/retire.go:13, 21-49). Kernel: "point a dispatch table's function field at a stub". Parameters: a row regex with key and handler groups, and the stub |
| `dropopts` | Gv | `command_line_scan` with exactly two `switch (c)` (cut/dropopts.go:17, 25-43, 235). `DropShort`/`DropLong` are generic if handed a switch rather than calling `Switches` |
| `query-empty`, `query-dropoptions` | V | the report text "all 24 menu and spell commands" (steps.go:303); the phase queries `whim2`/`whim54` |
| `cmdidxs` | V | vim's first-two-letters index (`internal/cmdtab`) |
| `edit`, `query` | G | dispatch to a registered phase program |
| `cemit` | G | the canonical print |
| `includes` | G | steps/includes.go: a gcc silent-compile oracle; "phase 82's" order from the bottom is a reproduction detail |
| `funcreach` | Gv | root `main` (dead/funcreach.go:132), floor `MinDefinitions = 100` (dead/funcreach.go:189) |

### crefactor/sweep: Gv (4 files)

- **prune.go:232**: `a.roots = append(a.roots, "o:main")`. **The one piece
  of vim knowledge, and it matters.** Measured: with `main` renamed in
  testregex.c (a library-shaped TU), the sweep deleted 1,988 of 2,042 lines.
  Parameter: `Roots []string` (plus "every external-linkage definition", which
  is `reach`'s policy).
- **prune.go:43**: the comment promises a guard, "while ml_recover is defined a
  struct layout is a disk format", that the code **does not implement**. The
  guards are in `guards()` (:613-657) and `positional()` (:660-715). The
  comment is stale; `reach` still has the guard (below).
- **prune.go:54-60** (a policy, not a name): an unused local goes **with an
  initialiser that calls something** (`Stats.ActingLocals`). That was right
  for vim (phase 60's `cmdline_fuzzy_complete`) but is not behaviour-preserving
  in general. A generic library needs it as an option,
  `DeleteEffectfulInitialisers bool`.
- eval.go, sweep.go and prune_test.go: G.

### crefactor/cemit: G (5 files)

- **No vim names.** What it carries from vim is a **style**. emit.go:209-221
  puts the name at column 0 under indented specifiers ("vim's own style"),
  which every `^name(` anchor in the repo depends on. decl.go:177 puts one
  enumerator per line for a one-constant enum (slim-vim's `#define`
  residue).
- **A precondition it does not enforce.** `includes()` (emit.go:238-254)
  keeps only `#include` lines. **Measured**: a `#define N 3` is silently
  dropped, leaving `printf("%d\n", N)` undefined, and `#if 1 ... #endif` is
  silently resolved. That contradicts its own doc ("never a silent
  omission"). It is harmless for vim, whose inputs have only `#include`
  (slim 41, whim 12), and it must refuse in a generic tool.

### internal/cutil: G (9 files)

blank, body, definition, depths, fold, match, norm, phase and split are all
generic C-text utilities on the canonical form. The only vim text is in a
comment (norm.go:14, `VIMRUNTIME`). `fold.go`'s `PyRepr`/`pyPattern` are
Python-port residue, not vim.

### internal/edit (10 files)

| file | class | vim knowledge |
|---|---|---|
| driver.go, phdriver.go, literals.go, notword.go, query.go, edit.go | G | none (driver.go:335 and phdriver.go:94 are comments). The verb set: Sub, Cut, Lines, Literal, FoldNever/FoldAlways/FoldAlwaysElse/DropIf, InFunction, InTable, Splice, Body, DeleteDefinition, DropBlocks |
| blocks.go | Gv | `FoldWalk` writes `v = curbuf;` (blocks.go:16, 54). Parameter: the binding expression. `ReplaceBlock`, `DropBareBlock`, `InnerBody` and `ConstOf` are G |
| stores.go | G | `DeadStores`: set-but-not-used locals, to a fixpoint |
| residue.go | V | the `cmdnames[]` row shape `^    \[CMD_\w+\] = \{` (residue.go:12-14) |
| shared.go | mixed | **G**: Key, CountNewlines, JoinInts, SortedKeys, Contains, First, IndexOf, Uniq, ContainsStr, CoreHead, CoreCalls/CoreCallRe, W134Pure (:314), W134Empty/Fn/Write/Else (:330-335) (since renamed PureCond, EmptyGuardedBlock, callToken, writeToken, ElseLine), EnclosingLoop, BareDeclOnly, IncludeCount, MentionCount, WithoutIncludes, Line, Head. **V**: BindsToWalk/FwdWalk/BwdWalk (`firstbuf`/`b_next`, `lastbuf`/`b_prev`, :33, :321-324); W119* (`mch_get_pid`, `b0_pid`, `long_to_char`, `kill(getpid()`, :231-240); W127* (`ml_flags`, `dl_text`, `dp*`, :303-308); W80* (`cmdnames`, `CMD_index`, `CMD_SIZE`, `cmdidxs1/2`, `command_count`, `vim_strchr`, `ea.skip`, `vim9`, :349-365); W88Lines and W126Py* are port helpers |

### internal/cut (54 files): V, with generic helpers

- **Every exported cutter is vim.** The report column in cut/edits.go carries
  no vim names.
- **Generic helpers.**
  - edits.go (all generic): `literal`, `subOnce`, `subCount`, `subCountRepl`,
    `foldNever`, `foldAlways`, `dropIf`, `dropIfUncounted`, `inFunction`,
    `linesMatchingUnless`.
  - keepbytes.go:21 `foldAll` and :41 `keepThenChain`.
  - lfonly.go:19 `keepThen`.
  - dropopts.go `DropShort`/`DropLong` (generic given a switch).
  - utf8only.go:98-458 (constant propagation of flags).
  - noconv.go:78-116 (`directCall`).
- **A third copy of the verb set.** cut/edits.go duplicates `edit.E`'s verbs
  (a third copy, after `edit.E` and `edit.Ph`).

### internal/cmdtab (2 files): V

cmdidxs.go and cmdnames.go regenerate vim's `ex_cmdidxs` block from
`cmdnames[]`/`EXCMD` rows (cmdnames.go:24-25, cmdidxs.go:23-24). The idea
"a derived table must equal f(source table)" is generic; the code is not.

### internal/build (4 files)

| file | class | vim/whim knowledge |
|---|---|---|
| plan.go | V (data) | the pipeline: 170 phases, their op lists and vim names (81 distinct vim identifiers in its literals) |
| build.go | Gv | the work file `whim-vim.c` (:89); `REMOVED` env var and `internal/phase/%03d/delta.md` (:226-232, :268-290), phase 80's declared tokens; `@minmax` (:251), phase 109's host probe; `inner.c` (:199). Parameters: a work name, an argument-resolver hook (`map[string]func(scratch) string`), declared-input files per phase |
| phase.go | Gv | the work file `whim-vim.c` (:49); `SnapDir = ".cache/boundaries"` (:71). Advance/finish/Check/snapshots are the generic pipeline driver: finish = sweep + canonical print |
| compile.go | G | gcc's compile line (a build fact, not vim) |

### internal/gen (9 files, plus pre/ and splice/): Gv, a C→Go transpiler with a whim-core profile

**Its precondition** is a core with no libc and no preprocessor, calling only
`host_*`, `musl_*` and the allocators: `editor.c` after Part II. Its runtime
(`editor/crt.go`, `Ptr[T]`, `GaData`) is hand-written for that core.

| location | knowledge | parameter |
|---|---|---|
| analyze.go:565 | `a.freeing = d.Name() == "vim_free"` | `Frees []string` |
| gen.go:421-423 | parameters named `varp`/`varp_arg` are puns | `Puns []string` |
| skel.go:85-91 | `crtFuncs`: `alloc*`, `musl_mem*`, `musl_str*`, `ga_grow_inner` | `RuntimeFuncs` |
| editor.go:10-99 | a fixed Go body for `ga_grow_inner`, and its OK/FAIL→bool rewrite (:82-95) | a runtime-body table |
| body_expr.go:315, 865, 1252 | `ga_data` member is a growarray's storage | `GrowArray{Type, Data, ItemSize}` |
| body_expr.go:897, 1055 | `allocators = alloc, alloc_clear, lalloc, lalloc_clear` | `Allocators` |
| body_expr.go:923, 939 | `*garray_T` | `GrowArray.Type` |
| body_expr.go:1013-1042 | `musl_memmove/memcpy/memset/memcmp` map to runtime `Memmove`/`Memset`/`Memcmp` | `ByteFuncs` |

- **G**: body.go, body_init.go, body_stmt.go, terminate.go, pre/pre.go and
  splice/splice.go carry no names. `__builtin_expect` (body_expr.go:1053) is
  GCC, not vim.

### internal/reach (3 files, plus a test): Gv

- reach.go:197 sets `mlRecover` when `ml_recover` is defined, and :661 gives
  the reason `WhyRecover`. Parameter: `FrozenLayoutIf []string` (a function
  whose presence means structs are a disk format).
- cast.go:49: `allocators` (+`host_alloc`).
- control.go: G (gcc as control).
- **Roots**: reach's roots are *all external linkage + static_assert +
  across the core/host cut*. The sweep's are *main + static_assert*. The two
  root policies differ, and a generic library should offer both.

### internal/ccx (9 files; not in the brief, but gen and reach use it): Gv

- **G**: gotos.go, funcs.go, ccx.go and pure.go (`PureFuncs` is computed).
- **Gv**:
  - order.go:31-37: `pureCalls` = the `musl_*` names plus `gettext` and `_`.
  - casts.go:72 (allocators) and :130 (`byteFuncs`).
  - voids.go:22-28 (`voidOwners`).
  - garrays.go: vim's growarray model (`garray_T`, `ga_data`, `ga_itemsize`,
    `ga_init2`, `ga_grow`).
- **V**: unions.go:377-431, 487-551, a table of vim's unions and their
  discriminants (`ae_u` by `t_colors`, `rs_un` by `RS_*`, option callbacks by
  `P_BOOL/P_NUM/P_STRING`).

### internal/dead (2 files): Gv

funcreach.go:132 has root `main`, and `MinDefinitions = 100`. gcc.go is
generic. Only cut/ and edit/residue.go use it now.

## 3. Catalogue: the generic transformations already present

Written for someone using the generic tool on another code base.

**Common preconditions.**
- **The input is a slimmed TU**: one C23 file, parseable by the
  `crefactor/cc` fork (modernc cc/v4 plus two C23 productions), with **no
  preprocessor directives other than `#include <...>`** and no old-style
  parameter lists.
- **Anchors expect the canonical form.** Every text-level tool assumes
  cemit's printed form, so run the canonical print first.
- **Floors are vim's.** Many edits carry *count floors* calibrated on vim
  ("written against 273"), and those must become arguments.

| # | name | what it does | preconditions | evidence in the repo | where |
|---|---|---|---|---|---|
| 1 | **Canonical print** | prints the TU from the syntax tree in one spelling per construct (one statement per line, braces always, name at column 0, one declarator per declaration, comments dropped), recovering macro invocations from token positions | the common ones; **refuses nothing for `#define`/`#if` and drops them silently (measured), so add a refusal** | checked fixpoint `Emit(Emit(x)) == Emit(x)`; no `-g`, so a format change leaves the binary byte-identical; `whim-build-check` over 170 boundaries. Measured on testregex.c: fixpoint, 122/122 tests identical | crefactor/cemit |
| 2 | **Reachability sweep** | one closure from the roots over C's name spaces (ordinary, tag, member by name); deletes unreached functions, objects, prototypes, typedefs, tags, members, enumerators and unused locals, to a fixpoint | roots are `main` + static_assert names (hard-coded); members are live by name; **an unused local goes with an effectful initialiser (counted)**; guards: positional initialisers keep all members, enumerator runs are pinned to their values or kept, a struct or enum is never emptied | replaced six gcc-warning deleters; prune_test.go; `whim-build-check`; `internal/reach` reports the same closure with gcc as control. Measured: testregex.c kept its behaviour | crefactor/sweep |
| 3 | **Dead statements after a jump** | deletes the statements after a terminating statement (jump, `if/else` whose branches both terminate, a block ending in one) up to the next label/case | holds a run containing a declaration | `whim-test` 45/45 C and Go; staticcheck 17 → 0 on the Go. Measured on mini.c | phase/164 |
| 4 | **goto to a return** | writes the label's `return E;` at each `goto` whose label marks only a return; drops the label when unreferenced, and a now-unreachable return | holds when a name in E could be shadowed (inner-block or later declarations) | `whim-test` 45/45; the binary may differ at `-O0`. Measured on mini.c | phase/168 |
| 5 | **A question returns bool** | an `int` function whose every return is an answer (named truth constants, comparison, `!`/`&&`/`\|\|`, another answer, an answer-only local) becomes `bool`, with its locals, members and parameters; `f() == OK` becomes `f()` | truth constants are named (`TRUE/FALSE/OK/FAIL`, literal 0/1 are not answers); a function used as a value or named by the host stays `int`; the core ends at the first `"\n#include "` | "changes no value" argument; `whim-test` 45/45; vet/staticcheck clean. Measured: works on mini.c, refuses a file starting with `#include` | phase/166 |
| 6 | **Empty guarded blocks and dead stores** | an empty block under a condition that only reads (no call, assignment, ++/--) goes, as do empty `else`/final `else if`; locals only ever stored go (`DeadStores`); alternating to a fixpoint | loops are kept; the purity test is syntactic (`W134Pure`) | its old check recomputed the whole output with the real sweep, byte for byte | phase/134, edit/stores.go |
| 7 | **Never-NULL folding** | from allocator roots, finds to a fixpoint the functions whose every return is non-NULL, and folds `== nullptr`/`!= nullptr` tests directly after such a call | the roots must truly never return NULL (whim's phase 148 made that so) | old check: the whole output recomputed and compared | phase/149 |
| 8 | **Calls to a no-op function go** | every call to F becomes nothing, or its argument's side effect (`--n`) | F has an empty body | old check: the premise proved from the input (273 calls) | phase/132 |
| 9 | **Unneeded system headers** | each `#include <...>` removed alone if the file still compiles *silently*, then all together, else one by one from the bottom | the file compiles silently under `-fsyntax-only -O0 -Wall -Wextra -Wno-unused-parameter` (measured refusal on testregex.c: 32 warnings) | a silent compile is the oracle; the binary is byte-identical (GOAL 169) | steps/includes.go |
| 10 | **`nullptr` and `usize`** | `NULL` becomes `nullptr`, `size_t` becomes `typedef typeof(sizeof(0)) usize`, `(void *)nullptr` becomes `nullptr`, outside literals | C23 | byte-identical binary | phase/106 |
| 11 | **Attributes normalised** | drops `__attribute__((unused))` on parameters, writes `[[fallthrough]]`, keeps `format`/`format_arg` | cemit's layout | byte-identical binary, 788,488 bytes | phase/107 |
| 12 | **Degenerate unions** | a union with fewer than 2 members becomes its member, and `.member` accesses go | refuses a 0-member union | byte-identical binary, with a control that differs by 31,056 bytes | phase/120 |
| 13 | **libc function to own implementation** | a libc function becomes an in-file `musl_F`, calls renamed outside literals, prototype removed | a body per function (musl's) | byte-identical machine code for `abs`; per-phase partition | phase/097, 098, 114 |
| 14 | **libc F to host_F** | removes libc prototypes from the core's declaration block and renames calls by a partition of every mention | a host boundary | per-phase partition checks | phase/118, 119 |
| 15 | **Host boundary by gcc fixpoint** | moves every include to a boundary, compiles the prefix with `gcc -c -Wall`, and moves below what is "defined but not used" to a fixpoint; limits become `enum` + `static_assert` | seed, keep set, anchor and constants are vim's today | the core compiles with 0 preprocessor lines; `make editor.c` | phase/110 |
| 16 | **Variadic collapse** | a wrapper `W(lead..., fmt, ...)` becomes `(fmt_into(buf, room, fmt, ...), tail(buf))` at every call site, statement or value | a per-wrapper spec | per-phase check | phase/105 |
| 17 | **Constant propagation of flags** | named globals replaced by constants, declarations and assignments deleted, pure expressions simplified, `if`/`while` folded | flag names and init function; `DBCS_` leaks | old phase check | cut/utf8only.go:98-458 |
| 18 | **Devirtualise** | `(*ptr)(` / `ptr(` becomes `fn(` given a ptr→fn map | map proved by the phase | old phase check | cut/noconv.go:78-116 (also 136 by hand) |
| 19 | **Name constant expressions** | a repeated constant expression of one shape gets a name (from a table in the file, else mechanical), as an enumerator before first use | the expression shape and table row are vim's (`TERMCAP2KEY`, `key_names_table`) | byte-identical binary | phase/167 |
| 20 | **One-element walks** | `for (v = first; v; v = v->next) BODY` becomes `v = CUR; BODY` when the list provably has one node | the binding is hard-coded `curbuf` | phases 71, 72 | edit/blocks.go |
| 21 | **Text verbs** (the DSL) | counted, refusing edits on canonical text: `Sub`, `Cut`, `Lines`, `Literal`, `FoldNever` / `FoldAlways` / `FoldAlwaysElse` / `DropIf` (brace-matched `if` folding), `InFunction` / `InTable` scoping, `Splice`, `Body`, `DeleteDefinition`, `DropBlocks`; `cutil.Norm` (whitespace-insensitive match) | canonical form; the counts are per-codebase calibrations | every phase uses them; a miss refuses | edit/, cutil/ (and a third copy in cut/edits.go) |
| 22 | **Analyses (report only)** | `reach` (closure partition with gcc as control); `ccx`: `Gotos`, `FuncCompares`, `Order` (unsequenced effectful operands), `PureFuncs`, `Casts`, `VoidPtrs`, `GrowArrays`, `Unions`; `dead.FuncReach` | parameters as in section 2 | partitions: every entity or site in exactly one class, a leftover is a finding | reach/, ccx/, dead/ |
| 23 | **C → Go** | transpiles a libc-free core to Go on a `Ptr[T]` runtime | a whim-shaped core and a runtime profile (section 2) | `whim-editor-check`; `whim-test` runs the Go against the C | gen/, editor/crt.go |

## 4. A proposed separation

### Package layout

```
crefactor/          GENERIC: no vim identifier in any string literal (the scanner is the check)
  cc/          the fork, as is (moved or aliased)
  cprint/      = cemit; Style (column-0 names: default), and REFUSES non-#include directives
  sweep/       = sweep; Options{Roots, RootExternal, DeleteEffectfulInits, FreezeLayoutIf}
  text/        = cutil + edit's generic verbs (E, Ph, blocks minus curbuf, literals, notword,
                 stores) + the generic half of edit/shared.go; ONE verb set (retire cut/edits.go's copy)
  xform/       the generic transformations as Steps built from a Profile:
               deadstmt(164) gotoreturn(168) boolret(166) emptyblocks(134) nevernull(149)
               dropcalls(132) nullptr(106) attrs(107) unions(120) includes(169)
               devirt(53) constprop(52) namekeys(167) libcown(114) hostcalls(118/119) boundary(110)
  analysis/    = reach, ccx, dead, with their name tables from the Profile
  pipeline/    = build's driver: Phase, Step, Plan, Advance, Run, Check, snapshots, KeepGoing;
                 Config{WorkName, SnapDir, Finish, Resolve}
  togo/        = gen, with its runtime profile (Allocators, Frees, ByteFuncs, GrowArray, Puns)
internal/whim/               VIM: everything that knows vim
  profile.go   ONE value holding all the knobs below
  plan.go      = build/plan.go (data)
  steps.go     the op table: vim ops + generic ones, by name
  cut/ cmdtab/ phase/NNN/ (as they are), vimtext/ (W80*, W119*, W127*, walks, residue.go)
```

### The interface

It is one value, built by the vim side and passed in:

```go
type Profile struct {
    Roots         []string              // sweep/reach/funcreach: {"main"}
    Boundary      func([]byte) int      // core/host cut: index of "\n#include " (166, 167, 110...)
    Truth         struct{ True, False []string } // {"TRUE","OK"}, {"FALSE","FAIL"}   (166)
    NeverRetype   []string              // {"main"} + host-named                        (166)
    NoOpFuncs     []string              // {"vim_free","host_free"}                      (132)
    NeverNull     []string              // {"host_alloc"}                               (149)
    Allocators    []string              // alloc, alloc_clear, lalloc, lalloc_clear, host_alloc
    Frees         []string              // vim_free, host_free
    ByteFuncs     []string              // musl_memmove/memcpy/memset/memcmp
    PureCalls     []string              // musl_str*/is*, gettext, _
    GrowArray     struct{ Type, Data, ItemSize, Init, Grow string } // garray_T, ga_data...
    Puns          []string              // varp, varp_arg
    FreezeLayoutIf []string             // ml_recover
    Unions        []Discriminant        // ccx.Unions' table
}
```

- **Steps take the profile through their constructor.** The Step signature
  stays `func(text []byte, args []string, w io.Writer) ([]byte, error)`, and
  `xform.BoolRet(p)` returns one. Floors become arguments in the plan
  (`{Op: "emptyblocks", Args: []string{"--at-least", "30"}}`), so a foreign
  code base passes 0.
- **Phase-specific target lists** (retire's commands, a fold's variable,
  167's shape and table) stay as `Args`, as they are now.
- **The sweep's roots** come from `Profile.Roots`, set once on
  `pipeline.Config`, because the finish runs after every phase.

### Migration

Every step must keep `make whim-build-check` byte-identical: 77 s with the
sealed snapshots. Steps that touch gen must also keep `make whim-editor-check`.

1. **Sweep roots become a parameter.** `sweep.Prune(src, name, Options{Roots:
   []string{"main"}})`, with `Sweep` passing the vim value, and the stale
   `ml_recover` comment (prune.go:43) corrected. It is the smallest change, and
   the measured need is the library test that lost 1,988 of 2,042 lines. It is
   byte-identical by construction (same roots). **This is the first step.**
2. **cemit refuses any directive other than `#include`.** This is
   byte-identical for vim (41/12 directives, all `#include`), and it closes a
   measured silent loss. Add a foreign-C smoke check beside it: testregex.c
   through cemit+sweep+164/168, with 122/122 tests.
3. **Create `crefactor/text`** by moving cutil and edit's generic files, with
   type aliases and forwarding functions left in `edit` so that none of the 111
   phase packages changes. Byte-identical: code moves only.
4. **Split `edit/shared.go`**: the vim half to `whim/vimtext`, the generic half
   to `crefactor/text`. `FoldWalk` takes the binding as an argument
   (`"curbuf"` from phases 71 and 72).
5. **Move 164 and 168 into `crefactor/xform`**, sharing `stmtTerminates`;
   `phase/164` and `phase/168` become one-line registrations. Then 134, 132,
   149 and 166 with their knobs from the Profile and their floors as plan
   arguments. Then 106, 107, 120, 169 (and 114 with a body table).
6. **Split `build`** into `pipeline` (Advance, Run, Check, snapshots) and
   `whim` (plan, `whim-vim.c`, `delta.md`/`REMOVED`, `@minmax`) through
   `pipeline.Config`.
7. **Lift the name tables** out of reach, ccx and dead into the Profile.
8. **Lift gen's profile** (section 2's table) out of gen, and check with
   `whim-editor-check`.
9. **Optional: make `crefactor` a separate Go module**, with the foreign-C
   check as its own test. This needs `crefactor/cc` to move with it.

### Effort

| steps | work | estimate |
|---|---|---|
| 1-2 | parameter plus refusal plus smoke check | 0.5 day |
| 3-4 | mechanical moves, with aliases | 1.5-2 days |
| 5 | 10 phases become library transforms plus Profile plumbing | 2-3 days |
| 6-7 | build split, analysis tables | 1.5-2 days |
| 8 | gen profile | 1.5-2 days |
| 9 | own module | 1 day |

That is about **8-11 working days** to a generic library that the whim pipeline
calls byte-identically.

Extracting the kernels that M phases apply by hand (fold-by-value,
make-F-return-K-and-fold-callers, stub a dispatch entry, delete rows with their
enumerators, drop an always-null parameter, retype to the one type used,
outline a jumped-into block) as **new** library code is another **3-5 weeks**.
It should be written and tested on foreign C, **not** by rewriting the old
phases (section 5).

## 5. What is not worth separating

- **The 57 V phases and the 31 cutter phases** (internal/cut, 9,889 lines;
  11,070 lines of V phase code) are the record of what vim became. Their
  anchors are vim's canonical text, so "generic" versions would be rewrites
  that must reproduce vim's bytes exactly, with nothing reused.
- **Most M phases, as code.** Their kernels are real, but applied through
  hand anchors and count pins. Byte identity would force a generic
  fold-by-value to hit exactly the sites a hand wrote in 2026, quirks
  included. Build the kernels fresh in `crefactor/xform`, and leave the phases
  as they are.
- **The table ops** (`dropoptions`, `droplocal`, `retire`, `dropopts`,
  `query-*`, `cmdidxs`, `internal/cmdtab`, `edit/residue.go`). Their generic
  core is a brace match plus a regex. What makes them safe is vim knowledge:
  lookup-by-string functions, `PV_` locality, the modeline whitelist,
  `get_varp`'s accessor, `ex_ni`. Parameterising the regexes buys a tool no
  other code base's tables fit.
- **`ccx.Unions`' discriminant table and `ccx.GrowArrays`' growarray model.**
  These are knowledge of vim's data structures; the reusable shell around
  them is small.
- **gen's runtime** (`editor/crt.go`, `host.go`, `libc.go`). It is a
  contract with the whim core (`Ptr[T]`, the arena, the `musl_*` set). A
  profile is worth lifting (step 8); the runtime is not.
- **The canonical style** (the name at column 0). Every text tool relies on
  `^name(`. Keep it fixed; a `Style` option would only be a way to break
  anchors.
- **The count floors and per-verb counts.** They are calibration, and belong
  to each code base's plan, not the library. They move to plan arguments; the
  library only enforces "a miss refuses".
- **The 8 NoSource phases** are records, with nothing to separate. This
  includes phase 82's stale `editlit.go`, which could simply be deleted.
- **The `crefactor/cc` fork** is already generic and needs no split. It only
  needs to move with the library if the library becomes its own module.

## Side findings (not acted on)

- **`CLAUDE.md` is stale** in several sentences it says to re-measure: "164
  phases" (the plan has 170, 0-169), "the next phase is 164", "There is no
  test suite" (`internal/suite` and `make whim-test` exist, as `AGENDA.md`
  says), and "`plan.go`'s `Sweep` field" (there is none: every phase is swept
  in `finish`).
- **The sweep and reach disagree on roots**: `main` + static_assert against
  external linkage + static_assert + the host cut. **The sweep's `ml_recover`
  guard is documented but gone** (prune.go:43), while reach still reports it.
- **`stmtTerminates` exists twice** (phase/164, phase/168). **The verb set
  exists three times** (`edit.E`, `edit.Ph`, cut/edits.go). **Phases 87-98
  each redefine `inFunction`/`within`/`fold`**, and 075's `writesTo` and 076's
  `assignsTo` are copies of each other.

---

Scratch data: `.tmp/survey/` holds the vim identifier sets (`vim.ids`,
`vimd.ids`, `whimnew.ids`), `phases.tsv`, `classes.txt`, the scanner
(`scan/`), the exported snapshot (`head/`, with the survey driver
`cmd/survey`) and the foreign runs (`foreign/run1-7`).
