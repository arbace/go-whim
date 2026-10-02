# PHASES.md — the phases' numbers, new and old

**The pipeline's phases are numbered 0 to 103, in the order the plan runs
them, without a gap.** Until 2026-10-02 they were numbered with gaps from 0
to 184: the number a phase was given when it was written. The pipeline reform
(`PIPELINE-REFORM.md`) left 104 of them running and the gaps saying nothing --
45 records that edit nothing, compaction groups run under their last number,
programs the front calls from phase 2, 3, 6 or 7 -- so they were numbered
afresh (its §7, step 8, and §8's first question: the archive keeps the old
numbers).

## What a number names

- **Phase N** is the plan's Nth entry (`internal/build`'s `Plan`, held to it by
  `TestPlanNumbers`): its directory `internal/phase/NNN/`, its Go package
  `pNNN`, its program `whimN`, its snapshot `qNNN.c`, and the N that
  `go tool whim build --from N --to N` and the build's log name. A block
  (`d02-outside`) labels a run of them; `GOALS.md`, *The pipeline as it runs*,
  indexes the blocks by the new numbers.
- **Phase Nx is a part**: a program phase N runs among its steps that had a
  number of its own -- a member of a group of `PIPELINE-COMPACTION.md` §3d,
  run as one phase under the group's last number, which is now the phase; or a
  program the front calls. Its directory is `internal/phase/NNN/x/`, its
  package `pNNNx`, its program `whimNx`, and the letters follow the order the
  phase runs them. A part has no snapshot of its own.
- **A second entry** of a phase's package keeps its suffix: `whim18kp`,
  `whim19ep`, `whim20bl`, `whim34rows`; `whim2` is phase 2's query. One
  program runs outside its phase: `whim18`, which phase 2's front calls (the
  reform's D6), while its package's second entry runs in phase 18 -- a package
  is not split across two phases.
- **A record keeps its old number**: `internal/phase/archive/NNN/`, its
  `GOAL.md` alone, cited in live text as *record N*. Three group members whose
  rows the front retires now (45, 46 and 47, with no program of their own)
  joined the archive when the numbers moved, so it holds 48.

## Which text uses which numbers

The text that describes the pipeline as it runs uses the new numbers: the plan
and every comment and message of the code (the phases' programs included:
every citation by number was moved to the identity it names now), `CLAUDE.md`,
the READMEs, `AGENDA.md`, `GOALS.md`'s *The pipeline as it runs*, and the
surveys of the editors' idioms. Where such text needs an old number it says
so: *record 13*, *q71 of the old numbering*.

The text that records how the pipeline was written keeps the numbers it was
written in, and says so where it opens: every phase's `GOAL.md` (its title
carries the new number, and the line under it the old), the archive,
`GOALS.md`'s Parts I and II, `PIPELINE-REFORM.md`, `PIPELINE-COMPACTION.md`,
`internal/phase/STAGES.md`, `internal/gen/FINDINGS.md`, and the commit
messages. This file translates.

**How it was done.** `internal/phase/numbers.md` is the table, old number to
new, of every live directory; `internal/phase/renumber` read it once to move
the directories (`git mv`), rename the packages and the programs, title the
`GOAL.md`s and rewrite the citations, and reported what it could not decide,
which was written by hand. It is spent: run again, it would read the new
numbers as old ones, and it refuses. `go run ./internal/phase/renumber table`
still prints the first table below from the plan.

## The phases

| new | block | phase | old | parts: new (old) |
|---|---|---|---|---|
| 0 | `s00-seed` | seed, in the one spelling every later phase reads | 0 | 0a (106), 0b (105), 0c (107) |
| 1 | `d01-front` | no `$VIMRUNTIME` | 1 |  |
| 2 | `d01-front` | the front, continued; and the options for features that are not here | 2 | 2a (61) |
| 3 | `d01-front` | the front, continued; and no introduction, and the command line says only what the editor still decides | 3 | 3a (57), 3b (58), 3c (63), 3d (65), 3e (66), 3f (74) |
| 4 | `d01-front` | the front, on swept text; and the editor stops writing shell scripts, and stops drawing a menu | 6 | 4a (76), 4b (67), 4c (68), 4d (71), 4e (85), 4f (59) |
| 5 | `d01-front` | the front, ended; and the editor stops looking for files it was not given | 7 | 5a (64), 5b (72), 5c (73), 5d (75) |
| 6 | `d02-outside` | a file name means the file of that name | 14 |  |
| 7 | `d02-outside` | six options that no longer decide anything | 16 |  |
| 8 | `d02-outside` | the last two per-buffer encoding options | 17 |  |
| 9 | `d02-outside` | nothing outside the process is consulted | 20 |  |
| 10 | `d02-outside` | the working directory is where it started | 22 |  |
| 11 | `d02-outside` | no floating-point library | 23 |  |
| 12 | `d02-outside` | a write is a write, and nobody owns it | 25 |  |
| 13 | `d03-editing` | C indenting | 28 |  |
| 14 | `d03-editing` | insert completion, the popup menu, and the keys that reached them | 32 |  |
| 15 | `d05-commands-and-options` | no filters, sorting, alignment, `:drop`, `:wall` and the `:…all` commands, `:startinsert` and its kin, or `:noswapfile` | 48 | 15a (44) |
| 16 | `d05-commands-and-options` | one set of options | 49 |  |
| 17 | `d07-options` | no option nothing reads | 55 |  |
| 18 | `d07-options` | no shell, runtime or keyword-program options | 56 |  |
| 19 | `d07-options` | no suffix, case, delay, verbose-file, debug or filter-program options | 60 |  |
| 20 | `d07-options` | no buffer-type, file-type, listing, jump, update-time or autowrite options | 62 |  |
| 21 | `d09-one-of-each` | one file argument, and no argument list | 69 |  |
| 22 | `d09-one-of-each` | :e reloads in place, and there is no swap file | 70 |  |
| 23 | `d11-commands` | no buffer-name argument matching | 77 |  |
| 24 | `d11-commands` | empty functions, write-only counters, and the window id | 78 |  |
| 25 | `d11-commands` | the constant-return predicates | 79 |  |
| 26 | `d11-commands` | the Ex command table, cut to the commands that exist | 80 |  |
| 27 | `d12-terminal-and-ex` | no streaming Ex | 87 |  |
| 28 | `d13-files` | no write | 89 |  |
| 29 | `d13-files` | no read | 90 |  |
| 30 | `d13-files` | no `:edit`, and no `gf` | 91 |  |
| 31 | `d13-files` | nothing reads a byte | 92 |  |
| 32 | `d13-files` | the buffer has no name | 93 |  |
| 33 | `d13-files` | `:q` quits, and `ZZ` is `ZQ` | 94 |  |
| 34 | `d13-files` | the options nothing reads | 95 |  |
| 35 | `d13-files` | no `FILE *` that is never opened | 96 |  |
| 36 | `r01-libc` | the strings are the editor's own | 97 |  |
| 37 | `r01-libc` | the character classes, the numbers and the sort | 98 |  |
| 38 | `r02-host-chain` | the deadly ladder, `vim_main`, and a core that cannot stop the process | 102 | 38a (100), 38b (101) |
| 39 | `r02-host-chain` | the signals and the terminal are the host's | 103 |  |
| 40 | `r02-host-chain` | the messages are the editor's, the writing is the host's | 104 |  |
| 41 | `r03-boundary` | the plain host calls | 108 |  |
| 42 | `r03-boundary` | the header types and macros the core can own | 109 |  |
| 43 | `r03-boundary` | the move: the first `#include` becomes the boundary | 110 |  |
| 44 | `r03-boundary` | the scalar clock | 111 |  |
| 45 | `r03-boundary` | the case tables become one, and it is the union | 112 |  |
| 46 | `r03-boundary` | the message fold: `msg_puts_printf()` and the branch that reaches it | 113 |  |
| 47 | `r03-boundary` | `abs` and `labs`, the two the core took on trust | 114 |  |
| 48 | `r03-boundary` | the clock crosses the boundary | 115 |  |
| 49 | `r03-boundary` | the core calls nothing but the host | 119 | 49a (117), 49b (118) |
| 50 | `g02-unions` | the degenerate unions go | 120 |  |
| 51 | `r04-terminal` | the terminal names, and `-T` | 122 | 51a (121) |
| 52 | `r05-memory` | freeing is free, and the arena is measured | 124 |  |
| 53 | `r06-memline` | the swap file's residue, and what no sweep could find | 125 |  |
| 54 | `r06-memline` | a block number becomes a reference | 126 |  |
| 55 | `r06-memline` | de-page the leaf | 127 |  |
| 56 | `r06-memline` | fold the node types | 128 |  |
| 57 | `r07-types` | `p_emoji` is an `int` | 129 |  |
| 58 | `r07-types` | the `(pos_T *)-1` tests go | 130 |  |
| 59 | `r07-types` | the saved input buffer is a `garray_T *` | 131 |  |
| 60 | `r08-memory` | nothing frees | 132 |  |
| 61 | `r08-memory` | one buffer needs no hash table | 133 |  |
| 62 | `g03-empty-blocks` | the empty blocks fold | 134 |  |
| 63 | `r09-regex` | one regexp program type | 135 |  |
| 64 | `r09-regex` | the engine is called directly | 136 |  |
| 65 | `r10-types` | the changedtick is a number | 137 |  |
| 66 | `r10-types` | no parameter carries an eval value | 138 |  |
| 67 | `r10-types` | the core sorts and searches typed arrays | 139 |  |
| 68 | `r10-types` | highlight groups are found in their array | 140 |  |
| 69 | `r11-gotos` | `regrepeat()` does not jump into a case | 141 |  |
| 70 | `r11-gotos` | the version names no build date or time | 142 |  |
| 71 | `r11-gotos` | `regatom()`, `edit()` and `check_termcode()` have no goto | 145 | 71a (143), 71b (144) |
| 72 | `r12-memline-and-host` | a memline node names its block | 146 |  |
| 73 | `r12-memline-and-host` | `deathtrap()` runs at the host's next wait | 147 |  |
| 74 | `g04-never-null` | allocation cannot fail, and its branches fold | 149 | 74a (148) |
| 75 | `r13-translation` | the regexp stack is three typed stacks | 150 |  |
| 76 | `r13-translation` | the option table's defaults and its variables, typed | 152 | 76a (151) |
| 77 | `r13-translation` | `free_one_termoption()` compares without a cast, and its NULL write is gone | 154 | 77a (153) |
| 78 | `r13-translation` | call arguments with effects are evaluated in gcc's order | 155 |  |
| 79 | `r13-translation` | the regex size pass's node is a static byte, not (char_u *) -1 | 156 |  |
| 80 | `r13-translation` | get_register() and put_register() carry a yankreg_T *, not a void * | 157 |  |
| 81 | `r13-translation` | a highlight's terminal font is read only from a colour entry | 158 |  |
| 82 | `r13-translation` | a struct's text is a pointer to an allocation of its own | 159 |  |
| 83 | `r13-translation` | no line getter takes a cookie | 160 |  |
| 84 | `r13-translation` | no goto jumps into a block | 161 |  |
| 85 | `r13-translation` | no two function pointers are compared | 162 |  |
| 86 | `g05-dead` | what the Go's linters found dead | 165 | 86a (164) |
| 87 | `g06-bool-and-keys` | `bool` and key names | 167 | 87a (166) |
| 88 | `g07-includes` | the system headers nothing needs | 169 |  |
| 89 | `g08-gotos` | a goto whose label marks a short tail is that tail | 170 |  |
| 90 | `g08-gotos` | a goto that is a break is break | 171 |  |
| 91 | `g08-gotos` | a goto back is a loop | 172 |  |
| 92 | `g08-gotos` | a goto out of its block is a break | 173 |  |
| 93 | `r14-parallel-substitute` | no address of a position's line or column | 174 |  |
| 94 | `r14-parallel-substitute` | a member's address a call hands back is a local's | 175 |  |
| 95 | `r14-parallel-substitute` | the regex engine's state is a parameter | 176 |  |
| 96 | `r14-parallel-substitute` | a line's match on its own | 177 |  |
| 97 | `r14-parallel-substitute` | :g marks the lines match_lines finds | 178 |  |
| 98 | `r14-parallel-substitute` | no mark is cleared when none was set | 179 |  |
| 99 | `r14-parallel-substitute` | the host's clock can be held still | 180 |  |
| 100 | `g09-values` | an out-parameter a value, a struct local its members | 181 |  |
| 101 | `g10-plain-c` | gettext's identity not called, the ASCII tests named, constant ifs their branch | 182 |  |
| 102 | `g11-bool` | a file-scope flag is bool | 183 |  |
| 103 | `g11-bool` | more flags are bool | 184 |  |

## The records

What each record did is its `GOAL.md`; where its work went is the phase that
does it now.

| record | what it was | where its work went |
|---|---|---|
| 4 | the binary's name stops choosing what it does | `argvfront`, phase 1 (the reform's D1) |
| 5 | one regexp engine, not two | `nonfa`, phase 3 (D11) |
| 8 | `:!` keeps its name and loses its process | `noshellout`, phase 3 (D12) |
| 9 | the editor stops asking the environment what language it is in | `nolocale`, phase 2 (D6) |
| 10 | no tag stack | `notags`, phase 3 (D10) |
| 11 | nothing is written that was not asked for | `noswap`, phase 1 (D5) |
| 12 | UTF-8, and no other encoding, ever | `noenc`, phase 2 (D7) |
| 13 | the editor stops re-reading a file it has already read | `nostat`, phase 4, on the text the front swept |
| 15 | the last two encoding options | `nofencs`, phase 2 (D7) |
| 18 | nothing is read at startup, and nothing on the command line decides anything | `nostartup` and `nocmdopts`, phase 2 (D6) |
| 19 | the terminal is what the build says | `noterm`, phase 2 (D8) |
| 21 | there is nothing to recover, and the memfile is memory | `norecover` and `nomemfile`, phase 1 (D5) |
| 24 | there is no mouse | `nomouse`, phase 2 (D8) |
| 26 | five signals, not twenty-one | `nosignals`, phase 3 (D12) |
| 27 | `[[=a=]]` stops meaning "a with any accent" | `noequiclass`, phase 3 (D11) |
| 29 | `:command`, user-defined commands | `noucmd`, phase 3 (D10) |
| 30 | `K` and the tag jumps, keeping `*` and `#` | `noident`, phase 3 (D10) |
| 31 | file-name modifiers | `nofnamemod`, phase 3 (D10) |
| 33 | commands whose machinery has already gone | the Ex commands retired by `exfront`, phase 1 (D2) |
| 34 | no abbreviations | `noabbr`, phase 3 (D10) |
| 35 | no scripts, no session, no autocommands | `nosession`, phase 2 (D6) |
| 36 | one tab page, always | `notabs`, phase 3 (D9) |
| 37 | no command that does nothing | `noinert`, phase 3 (D9) |
| 38 | the argument list is walked by `:next` and `:previous` alone | `noarglist`, phase 3 (D9) |
| 39 | one window, always | `nowindows`, phase 3 (D9) |
| 40 | no window sizes to set | `nowinsizes`, phase 3 (D9) |
| 41 | the buffer list is walked by `:bnext` and `:bprevious` alone | `nobuflist`, phase 3 (D9) |
| 42 | one buffer, always | `onebuffer` and its `droplocal`, phase 4 |
| 43 | no -c, --cmd, -R, -m, -M or -w | `argvfront` and the fall-out closure, phase 1 |
| 45 | no `:drop` | its row, retired by `exfront`, phase 1 (D2) |
| 46 | no `:wall`, `:qall`, `:quitall`, `:wqall` or `:xall` | their rows, retired by `exfront`, phase 1 (D2) |
| 47 | no `:startinsert`, `:startreplace`, `:startgreplace` or `:stopinsert` | their rows, retired by `exfront`, phase 1 (D2) |
| 50 | only LF text files | `lfonly` and its `droplocal`, phase 5 |
| 51 | a byte that is not UTF-8 is kept as it is | `keepbytes`, phase 5 |
| 52 | UTF-8 is not a question | `utf8only`, phase 2 (D7) |
| 53 | no conversion layer, no 'encoding' | `noconv` and its `droplocal`, phase 5 |
| 54 | no option without a variable | `optfront`, phase 1 (D3), with every option the product has not |
| 81 | one line, one command | `onecmdfront`, phase 1 |
| 82 | every comment | nothing: the canonical print takes them |
| 83 | the core's compile line, and the baselines it is measured against | nothing: its line is every boundary's |
| 84 | the stack protector goes | nothing: its line is every boundary's |
| 86 | the instrument becomes the screen | nothing: the suite it built is gone |
| 88 | argv is `+{command}` and `-T {term}` | `argvfront` and the fall-out closure, phase 1 |
| 99 | the includes nothing names | phase 88, which drops every header nothing needs |
| 116 | the terminal table is asked with `+set term=`, not `$TERM` | nothing |
| 123 | the instrument could not see the text layer | nothing: the suite it built is gone |
| 163 | the product is in the one canonical spelling | nothing: every boundary is printed canonically |
| 168 | a goto whose label returns is that return | `GotoTail`, phase 89, a `return x;` being a tail of none |

## Old to new

Every old number, and what it is now: a phase, a part, or a record.

| old | now | old | now | old | now | old | now | old | now |
|---|---|---|---|---|---|---|---|---|---|
| 0 | 0 | 1 | 1 | 2 | 2 | 3 | 3 | 4 | record |
| 5 | record | 6 | 4 | 7 | 5 | 8 | record | 9 | record |
| 10 | record | 11 | record | 12 | record | 13 | record | 14 | 6 |
| 15 | record | 16 | 7 | 17 | 8 | 18 | record | 19 | record |
| 20 | 9 | 21 | record | 22 | 10 | 23 | 11 | 24 | record |
| 25 | 12 | 26 | record | 27 | record | 28 | 13 | 29 | record |
| 30 | record | 31 | record | 32 | 14 | 33 | record | 34 | record |
| 35 | record | 36 | record | 37 | record | 38 | record | 39 | record |
| 40 | record | 41 | record | 42 | record | 43 | record | 44 | 15a |
| 45 | record | 46 | record | 47 | record | 48 | 15 | 49 | 16 |
| 50 | record | 51 | record | 52 | record | 53 | record | 54 | record |
| 55 | 17 | 56 | 18 | 57 | 3a | 58 | 3b | 59 | 4f |
| 60 | 19 | 61 | 2a | 62 | 20 | 63 | 3c | 64 | 5a |
| 65 | 3d | 66 | 3e | 67 | 4b | 68 | 4c | 69 | 21 |
| 70 | 22 | 71 | 4d | 72 | 5b | 73 | 5c | 74 | 3f |
| 75 | 5d | 76 | 4a | 77 | 23 | 78 | 24 | 79 | 25 |
| 80 | 26 | 81 | record | 82 | record | 83 | record | 84 | record |
| 85 | 4e | 86 | record | 87 | 27 | 88 | record | 89 | 28 |
| 90 | 29 | 91 | 30 | 92 | 31 | 93 | 32 | 94 | 33 |
| 95 | 34 | 96 | 35 | 97 | 36 | 98 | 37 | 99 | record |
| 100 | 38a | 101 | 38b | 102 | 38 | 103 | 39 | 104 | 40 |
| 105 | 0b | 106 | 0a | 107 | 0c | 108 | 41 | 109 | 42 |
| 110 | 43 | 111 | 44 | 112 | 45 | 113 | 46 | 114 | 47 |
| 115 | 48 | 116 | record | 117 | 49a | 118 | 49b | 119 | 49 |
| 120 | 50 | 121 | 51a | 122 | 51 | 123 | record | 124 | 52 |
| 125 | 53 | 126 | 54 | 127 | 55 | 128 | 56 | 129 | 57 |
| 130 | 58 | 131 | 59 | 132 | 60 | 133 | 61 | 134 | 62 |
| 135 | 63 | 136 | 64 | 137 | 65 | 138 | 66 | 139 | 67 |
| 140 | 68 | 141 | 69 | 142 | 70 | 143 | 71a | 144 | 71b |
| 145 | 71 | 146 | 72 | 147 | 73 | 148 | 74a | 149 | 74 |
| 150 | 75 | 151 | 76a | 152 | 76 | 153 | 77a | 154 | 77 |
| 155 | 78 | 156 | 79 | 157 | 80 | 158 | 81 | 159 | 82 |
| 160 | 83 | 161 | 84 | 162 | 85 | 163 | record | 164 | 86a |
| 165 | 86 | 166 | 87a | 167 | 87 | 168 | record | 169 | 88 |
| 170 | 89 | 171 | 90 | 172 | 91 | 173 | 92 | 174 | 93 |
| 175 | 94 | 176 | 95 | 177 | 96 | 178 | 97 | 179 | 98 |
| 180 | 99 | 181 | 100 | 182 | 101 | 183 | 102 | 184 | 103 |
