# WHIM-GOAL.md — reduce slim-vim to an embedded editor, and that to an embeddable core

`slim-vim.c` is vim as one translation unit, with every feature upstream's
`tiny` configuration has. **`whim-vim.c` is what is left when the editor stops
expecting a filesystem to have been installed for it** (Part I, phases 0 to 82),
**and then stops being a program at all and becomes a component a host program
runs** (Part II, phases 83 to 128).

```
slim-vim.c = F(upstream@sha)          SLIM-GOAL.md, twelve phases
whim-vim.c = G(slim-vim.c)            this document: phases 0-82, then 83-128
```

The two pipelines are the same construct — a phase is a function of the tree it
is handed, memoized in three tiers — and differ only in what they remove.
`SLIM-GOAL.md` removes *files and preprocessor* and changes nothing about what
the editor can do. **This one removes capability, on purpose**, and every phase
has to say which and prove it removed nothing else.

The two parts are one pipeline with one numbering and one product. What changes
at phase 83 is what a phase is measured against: Part I's deltas are
`pipes/whim.delta` against slim-vim's behaviour, Part II's are `pipes/zero.delta`
against q82's, with an instrument an editor with no file to write can still be
measured by. `ZERO_FROM` in `tools/pipeline.sh` is that line, stated once. Part
II was a pipeline of its own, zero, until it was folded into this one; zero
phase *N* is phase *N*+83, and every number in this document is the one
pipeline's.

# Part I — phases 0 to 82: an editor with no runtime

## The charter

Whim vim is an **embedded** editor: one static binary, no installation, nothing
read from disk that was not compiled in. That is a different product from
slim-vim rather than a better one, and both are kept.

Four kinds of work, in rough order of value:

1. **Pruning** — capability that presumes an installed runtime.
2. **Dropping dependencies**, at run time and at build time. An embedded target
   cares less about bytes than about what it needs from the world.
3. **Simplification** — what the removals leave behind, which is usually more
   than they took.
4. **Optimisation** — last, because measuring it before the shape has settled
   optimises the wrong thing.

## What is measured

**Binary size and external surface**, reported by `make score`:

| | what it says |
| --- | --- |
| stripped bytes | what the target has to store |
| libc symbols still referenced | what the target has to provide |
| source lines | how much is left to reason about |

The symbol set is the one that matters. An embedded target is defined by what
it must supply, not by what it costs, and a phase that shrinks the binary while
adding a syscall has gone backwards. **Both numbers go in the same direction or
the phase is wrong.**

## The rules

1. **Removal is computed, not listed.** Cut the entry points — a command row,
   an option default, a branch of the environment layer — and let the sweep
   find what becomes unreachable: all six kinds of dead thing, in every phase
   (see *The sweep*). A phase that names 900 functions to
   delete has written down what the compiler already knows, and will be wrong
   the first time upstream moves.
2. **Every phase states its delta, in advance, as a check.** This is the whole
   difference from `SLIM-GOAL.md`, where any behavioural change is a bug. Here
   a change is the *point*, so the phase must say which behaviour changes and
   the harness must show exactly that set and no more. "Six cases differ" is a
   check; "some cases differ" is not.
3. **A command is deleted from both lists, and no other command inherits its
   words.** Until Phase 80 a removed command was pointed at `ex_ni` and kept its
   row, because the lookup took the first row whose name began with what was
   typed, so every row decided the abbreviations of the rows below it — the trap
   `SLIM-GOAL.md` records, where deleting `:help` makes the name run
   `:helpclose`. Phase 80 gave each row its shortest abbreviation and deleted the
   489 stubs. A typed word now names a row only if it is at least that long, so
   a match is unique, and a deleted row's words resolve to nothing: E492.
4. **`whim-vim.c` is produced from the committed `slim-vim.c`**, not from a
   pass. The two pipelines are decoupled: `make whim-vim` needs no clone, no
   network and no agent, and the memoize key is `slim-vim.c`'s digest and the
   implementation's, exactly as the other pipeline keys on upstream's sha.
5. **`whim-vim.c` carries no comments.** Phase 82 removed every one. A phase's
   replacement text contains no `//` or `/*` outside a string literal, and a
   comment an edit would make wrong is deleted, not reworded.

## The sweep, and what unreachable covers

Every phase ends the same way: `tools/sweep.sh` deletes what the phase's cut
left unreachable, to a fixpoint, because each kind of dead thing orphans the
others — deleting a function orphans a type, deleting a type orphans a
prototype, deleting a field orphans an enumerator. **Six kinds, in every
phase:**

| | by what | islands? |
| --- | --- | --- |
| functions | `deadsweep.py` (gcc) and `funcreach.py` | yes — reachability |
| prototypes | `deadprotos.py` | n/a |
| types | `typereach.py` | yes — reachability |
| variables | `deadsweep.py`, `-Wunused-variable` | no — reference counting |
| struct fields | `deadfields.py` | no — a mention outside every type definition |
| enumerators | `deadenums.py` | no — a mention anywhere |

**The sweep is not written into a phase program; the driver runs it, once per
stage.** Phases 1–82 are each two files: `pipes/whim<N>-edit.sh` makes the cut (and
may sweep part way through, where a second cut needs the first one swept), and
`pipes/whim<N>-check.sh` asserts, builds and probes. A **stage** is a run of phases
whose edits share one sweep: `tools/phaserun.sh` runs every edit in order on text no
sweep has touched since the stage began, one `tools/sweep.sh`, every check in order
on the swept text and its binary, and then the declared delta once. The check shares
nothing with the edit but the work tree and a state directory — the line count of
the text its edit was handed, the stage's symbol snapshot, and whatever file the
edit names for it. Only a stage's end is a boundary.

`pipes/whim.stages` is the schedule — 0 | 1-12 | 13-41 | 42-63 | 64-65 | 66-71 | 72 |
73-77 | 78 | 79 | 80 | 81 | 82 — and two kinds of fact that decide it, both measured
and both checked by `tools/stages.sh`:

- **what an edit needs of its input** (`need P swept|silent|swept-inner:K`). **A
  phase whose cut is computed from the text must see it swept** — phase 54 after 53
  without its inner sweep cut one option row too few and did not refuse — so that
  requirement is declared, not discovered. A counted anchor refuses on unswept text;
  a computed set shrinks silently.
- **which checks must see a boundary before a later phase** (`apart P K`). Inside a
  stage every check runs on the stage's end, so a check that asserts something a
  later phase removes on purpose — 64's "startPS stays", which 66 takes — fails
  there, and the two phases go in different stages.

And a stage whose end does not reproduce its recording is a failure, which is the
check behind both: without it the silent under-cut would have shipped.

`pipes/whim.delta` is rule 2's list, **written once**: for each phase, the Ex
commands, behaviour cases (`case:`) and terminal table (`term-moved`) it changes,
and `drop:` for a command that stops differing. The lines up to a phase are the
whole difference from slim at that phase; `tools/whimdelta.sh --phase N` checks a
binary against exactly that, and a stage checks its last phase's.

### Adding a phase

A new phase is the next one after 128, and Part II's *Adding a phase* is the current
process for it; what follows is how the mechanics work, and holds for both parts.

1. Write `pipes/whim<N>-edit.sh <work> <state>` — the cut — and
   `pipes/whim<N>-check.sh <work> <state>` — the assertions, `tools/phasecheck.sh`,
   `tools/phasebuild.sh` and the probes. Neither sweeps at its end and neither checks
   the delta; anything the check needs from the edit goes in `$state` by name.
2. **Declare its delta** in `pipes/whim.delta`: a line `N  command… case:name…`, or
   none if the harness sees nothing new — before running it. (From phase 83 on the
   delta is `pipes/zero.delta`'s, against other baselines: `WHIM-GOAL.md`.)
3. **Place it in the schedule.** Add N to the `phases` line of `pipes/whim.stages`,
   then add `stage N` there as a stage of its own, or widen the last stage to end
   at N. It must start a stage if its edit counts anchors against,
   or computes its cut from, swept text (declare `need N swept`), or needs a silent
   compile (`need N silent`). It must not share a stage with an earlier phase whose
   check it breaks (declare `apart P N`) — run the earlier checks on its result to
   find out.
   Then put N in the `package` line of its concept (or a new one), declare a `uses`
   line for each phase of another package it relies on, and run
   `tools/packages.sh whim --check`, which refuses a phase in no package. Nothing
   runs the packages, so this moves no key.
4. `make whim-tip` runs the last stage and records its boundary; `make whim-verify`
   then proves every stage from the recorded one before it.

**The last two are covered by no warning at all**, and for a while they were
covered by no sweep either. A phase of its own asserted them, part way through
the pipeline and then again at the tip, because the first assertion had been
followed by nine phases that orphaned 40 more fields and 14 more enumerators and
nothing in their own sweeps noticed. **An invariant asserted in one place is a
cleanup.** Asserted in every sweep, it holds at every boundary, and no phase is
ever handed dead code by the one before it.

**A struct field is not a variable.** `deadfields.py` calls a field live if its
name appears outside every type definition, since a mention inside another struct
is a different field with the same name. It refuses what it cannot be sure of,
because being wrong here is silent:

- a bitfield or anonymous member, whose declaration does not say plainly what
  it declares;
- the last field of a struct, since an empty struct is not C and whole types
  are `typereach.py`'s;
- any field of a type that is ever initialised positionally.
  `static termrequest_T crv_status = {STATUS_GET, -1};` fills two fields and
  names neither, so the second looks dead, and removing it gives *"excess
  elements in struct initializer"* — a warning, not an error, which a sweep keyed
  on errors would have shipped;
- **every field, while `ml_recover()` exists.** Removing a field moves the ones
  after it, and until the editor cannot read a swap file, block zero and the
  memfile's pages are a disk format: a field nothing in the code reads is still
  a field another vim wrote. The question is asked of the file rather than of a
  phase number, so the field sweep starts by itself in the phase that removes
  recovery — and never in `slim-vim.c`, which keeps it. Measured: no phase before
  Phase 21 removes a field, and Phase 21 removes 80.

**An enumerator's value is its position**, so deleting one renumbers every
implicit one after it, and several enums index a parallel table. `deadenums.py`
reads the values from DWARF — `tools/enumvals.sh`, where the compiler has already
done the arithmetic for `1 << 3` and `0x80000000L` — pins the first survivor
after each deleted run, and dumps DWARF again after the sweep to require that no
survivor moved. The dump costs a debug build, so it is taken **on first need**:
most sweeps find no dead enumerator and never pay for it. A survivor DWARF has no
value for cannot be pinned, so the run before it stays — an unpinned survivor
renumbers silently, and a before-and-after comparison cannot see a name that is
in neither dump.

### The bug that asking about fields found first

`typereach.py` had a blind spot that no amount of new tooling would have
covered. `START` matched `struct X {` with the brace on the same line, and
**111 of this file's type definitions put the brace on the next line**. Those
were not definitions as far as the tool was concerned, so every field inside
them counted as a *root* — and a whole dead island lived on because of it:
`channel_T` is mentioned exactly twice outside its own definitions, and both are
fields, `jv_channel` in `jobvar_S` and `ch_next` in `channel_S`. `jobvar_S` was
invisible, so `jv_channel` was a root, so the `+channel` and `+job` types sat
there complete, long after every function that used them had gone.

Recognising the form took two goes, and both failures are the same shape as the
`deadsweep` bug in Phase 24:

1. `static struct modmasktable { … } mod_mask_table[] = { … };` is a type
   definition **and a variable** in one construct, and the declarator sits
   between the *struct's* closing brace and the `=` — not after the last `}`,
   which belongs to the initialiser. So the name was never collected, the tag
   was unreachable, and the whole construct went, leaving `mod_mask_table[i]`
   undeclared 40,000 lines away. A construct that declares a variable is not a
   type definition to delete; it is a variable, and `deadsweep.py` owns those.
2. `typedef struct { … } chanpart_T;` does **not** declare a variable — there
   the declarator names the type — so the rule above had to exclude typedefs.

Fixed, it removes **378 lines** on its own, and it made every sweep stronger.


## Concept index: the phases as packages

The phase sections below are in the order the work was done, and it shows: windows
are cut in six places, buffers in nine, options in eight. This index reads the same
83 phases **by concept** — eighteen *packages* — so that "everything this editor
lost about windows" is one list. It is placed here, after the rules and the sweep
that every phase shares and before the first phase section, because it is a table of
contents for those sections: it introduces nothing a phase depends on, and every
line of it points down into one of them.

**A package is a view, and nothing runs it.** No phase moved, no boundary moved, and
no cache key moved: the schedule is still `pipes/whim.stages`' `stage` lines, and a
package's phases are spread across stages. The data is two more kinds of line in the
same file — `package NAME P...`, and `uses A:P B:Q KIND why` for a phase that relies
on a phase of another package having run — which `tools/stages.sh` ignores and
`tools/packages.sh whim` prints. `tools/packages.sh whim --check` refuses a phase in
no package or in two, an unknown phase or package, a `uses` inside one package and a
`uses` whose dependency runs later. It is a tool of its own because
`tools/phaserun.sh` names `tools/stages.sh`, which puts every byte of that script in
every stage's cache key.

**Packages were assigned from what each phase's program does, not from its title**,
and a phase that does two things is in the package of the larger cut: phase 6 cuts
the shell's wildcard expander and the wildmenu and is in `files`; phase 44 retires
`:!` and six text commands and is in `text`; phase 70 makes `:e` reuse the one
buffer and removes swap-file detection and is in `buffers`.

**Each dependency is tagged with its kind.** *Mechanical*: without the earlier
phase the later one fails or cuts wrongly — an anchor or assertion refuses, a tool
refuses to drop an option something still reads, a computed set comes out
different, or what it removes as dead or constant would still be live. *Rationale*:
the later phase would still run and cut the same thing, and the earlier one is the
reason given that the cut costs nothing or is the right call. Of the 50, 37 are
mechanical and 13 are rationale. `tools/packages.sh whim --check` refuses any other
kind, and `make whim-verify` and `make whim-tip` run that check before they start.

**A `uses` line is only written where a phase program or a section here says so**,
and each was checked against the program that did the work — which is how four
phase numbers in the prose were found stale (26's `ml_sync_all()` was emptied by 11,
its `preserve_exit()` loop by 21; `K`'s `:!` lost its process in 8; `ins_ctrl_x()`
was emptied by 32). Dependencies inside a package are its order and are not listed.
Where the reason is a constant a later phase asserts or folds, the dependency is
real: phase 79's step 1 requires each of 28 bodies to be `return <constant>;`, and
refuses if the phase that made it so had not run.

| package | phases |
| --- | --- |
| [`seed`](#seed) | 0 |
| [`environment`](#environment) | 1 9 20 23 26 |
| [`options`](#options) | 2 16 49 54 55 56 60 62 |
| [`startup`](#startup) | 3 4 18 43 |
| [`regexp`](#regexp) | 5 27 76 |
| [`files`](#files) | 6 7 8 13 14 22 25 31 |
| [`tags`](#tags) | 10 30 63 74 |
| [`swap`](#swap) | 11 21 48 |
| [`encodings`](#encodings) | 12 15 17 50 51 52 53 |
| [`terminal`](#terminal) | 19 24 61 67 |
| [`text`](#text) | 28 44 57 64 65 66 |
| [`scripts`](#scripts) | 29 35 75 |
| [`completion`](#completion) | 32 59 |
| [`commands`](#commands) | 33 37 47 80 81 |
| [`mappings`](#mappings) | 34 58 |
| [`windows`](#windows) | 36 39 40 68 72 73 |
| [`buffers`](#buffers) | 38 41 42 45 46 69 70 71 77 |
| [`tidy`](#tidy) | 78 79 82 |

### seed

The copy that every later phase is measured against. Stage `0`.

| phase | title | stage |
| --- | --- | --- |
| 0 | seed, and prove the copy is a copy | `0` |

### environment

What the host would have to provide: an installed runtime, a locale, a home directory and environment variables, a maths library, signals. Stages `1-12`, `13-41`.

| phase | title | stage |
| --- | --- | --- |
| 1 | no `$VIMRUNTIME` | `1-12` |
| 9 | the editor stops asking the environment what language it is in | `1-12` |
| 20 | nothing outside the process is consulted | `13-41` |
| 23 | no floating-point library | `13-41` |
| 26 | five signals, not twenty-one | `13-41` |

Relies on:

- **20** after **18** (`startup`), *mechanical* — vimrc_found()'s callers are dead: every do_source() passes DOSO_NONE since 18
- **26** after **11** (`swap`), *rationale* — SIGPWR's handler called ml_sync_all(), empty since 11 (tools/noswap.py)
- **26** after **21** (`swap`), *rationale* — deathtrap() cannot preserve: 21 removed preserve_exit()'s loop (tools/nomemfile.py)

Relied on by: 12 (`encodings`), 21 (`swap`), 25 (`files`), 31 (`files`).

### options

Options as a table: the rows no feature reads any more, and every way to make two copies of one differ. Stages `1-12`, `13-41`, `42-63`.

| phase | title | stage |
| --- | --- | --- |
| 2 | the options for features that are not here | `1-12` |
| 16 | six options that no longer decide anything | `13-41` |
| 49 | one set of options | `42-63` |
| 54 | no option without a variable | `42-63` |
| 55 | no option nothing reads | `42-63` |
| 56 | no shell, runtime or keyword-program options | `42-63` |
| 60 | no suffix, case, delay, verbose-file, debug or filter-program options | `42-63` |
| 62 | no buffer-type, file-type, listing, jump, update-time or autowrite options | `42-63` |

Relies on:

- **16** after **10** (`tags`), *mechanical* — 'tags' and 'tagcase' have decided nothing since the tag stack went
- **16** after **11** (`swap`), *mechanical* — 'swapfile': ml_open() says no swap file whatever it is set to, since 11
- **16** after **13** (`files`), *mechanical* — 'autoread': ex_drop() saved and restored it around a check 13 removed
- **16** after **14** (`files`), *mechanical* — 'path' and 'suffixesadd' have decided nothing since the file finder went
- **54** after **53** (`encodings`), *mechanical* — its row set is COMPUTED, and holds 'arabic' only once 53's second cut is swept
- **56** after **8** (`files`), *mechanical* — 'shell', 'shellquote', 'shellredir': no shell is run since 8
- **56** after **30** (`tags`), *mechanical* — 'keywordprg': K went in 30
- **60** after **7** (`files`), *mechanical* — 'suffixes' ordered wildcard matches, and nothing has expanded since 7
- **60** after **44** (`text`), *mechanical* — 'formatprg', 'equalprg' only built a :{range}! line, ex_ni since 44
- **62** after **11** (`swap`), *mechanical* — 'updatetime': the idle sync reached ml_sync_all(), empty since 11
- **62** after **35** (`scripts`), *mechanical* — 'buflisted', 'filetype': their readers chose events, and 35 made dispatch FALSE

Relied on by: 64 (`text`), 65 (`text`).

### startup

What an invocation may say: the binary's name, the command-line flags, and the files read before the first command. Stages `1-12`, `13-41`, `42-63`.

| phase | title | stage |
| --- | --- | --- |
| 3 | no introduction, and the command line says only what the editor still decides | `1-12` |
| 4 | the binary's name stops choosing what it does | `1-12` |
| 18 | nothing is read at startup, and nothing on the command line decides anything | `13-41` |
| 43 | no -c, --cmd, -R, -m, -M or -w | `42-63` |

Relied on by: 20 (`environment`), 35 (`scripts`).

### regexp

One engine, and the parts of a pattern nothing here writes. Stages `1-12`, `13-41`, `73-77`.

| phase | title | stage |
| --- | --- | --- |
| 5 | one regexp engine, not two | `1-12` |
| 27 | `[[=a=]]` stops meaning "a with any accent" | `13-41` |
| 76 | one regexp engine, so no retry | `73-77` |

### files

The editor reaching the filesystem on its own account: globbing, directories, `path` search, timestamps, backups, the shell and file-name modifiers. Stages `1-12`, `13-41`.

| phase | title | stage |
| --- | --- | --- |
| 6 | the editor stops writing shell scripts, and stops drawing a menu | `1-12` |
| 7 | the editor stops looking for files it was not given | `1-12` |
| 8 | `:!` keeps its name and loses its process | `1-12` |
| 13 | the editor stops re-reading a file it has already read | `13-41` |
| 14 | a file name means the file of that name | `13-41` |
| 22 | the working directory is where it started | `13-41` |
| 25 | a write is a write, and nobody owns it | `13-41` |
| 31 | file-name modifiers | `13-41` |

Relies on:

- **25** after **20** (`environment`), *mechanical* — get_user_name() is `return FAIL;` since 20, so its caller's test folds
- **31** after **20** (`environment`), *rationale* — :~ shortened a name under $HOME, a notion 20 removed

Relied on by: 16 (`options`), 30 (`tags`), 32 (`completion`), 33 (`commands`), 44 (`text`), 56 (`options`), 60 (`options`), 79 (`tidy`).

### tags

Finding a place by name: the tag stack and tag keys, the jump list, file marks. Stages `1-12`, `13-41`, `42-63`, `73-77`.

| phase | title | stage |
| --- | --- | --- |
| 10 | no tag stack | `1-12` |
| 30 | `K` and the tag jumps, keeping `*` and `#` | `13-41` |
| 63 | no jump list | `42-63` |
| 74 | no file marks | `73-77` |

Relies on:

- **30** after **8** (`files`), *rationale* — K ran 'keywordprg' through :!, which has had no process since 8
- **74** after **70** (`buffers`), *rationale* — fname2fnum() is an empty body since 70, left for the file-mark cut

Relied on by: 16 (`options`), 32 (`completion`), 56 (`options`).

### swap

The swap file, recovery, and the memfile as a disk format. Stages `1-12`, `13-41`, `42-63`.

| phase | title | stage |
| --- | --- | --- |
| 11 | nothing is written that was not asked for | `1-12` |
| 21 | there is nothing to recover, and the memfile is memory | `13-41` |
| 48 | no `:noswapfile` | `42-63` |

Relies on:

- **21** after **20** (`environment`), *rationale* — :undolist's clock time goes because 20 took every way of being told the zone

Relied on by: 16 (`options`), 17 (`encodings`), 26 (`environment`), 62 (`options`), 70 (`buffers`), 77 (`buffers`).

### encodings

One encoding and one line ending: UTF-8, LF, and the conversion layer behind them. Stages `1-12`, `13-41`, `42-63`.

| phase | title | stage |
| --- | --- | --- |
| 12 | UTF-8, and no other encoding, ever | `1-12` |
| 15 | the last two encoding options | `13-41` |
| 17 | the last two per-buffer encoding options | `13-41` |
| 50 | only LF text files | `42-63` |
| 51 | a byte that is not UTF-8 is kept as it is | `42-63` |
| 52 | UTF-8 is not a question | `42-63` |
| 53 | no conversion layer, no 'encoding' | `42-63` |

Relies on:

- **12** after **9** (`environment`), *mechanical* — mb_init() refuses all but utf-8; the compiled default is utf-8 only since 9
- **17** after **11** (`swap`), *mechanical* — add_b0_fenc() wrote into a swap file's block zero, and 11 left none

Relied on by: 54 (`options`), 79 (`tidy`).

### terminal

What the terminal is told and asked beyond drawing: its type from the environment, the mouse protocol, the window title. Stages `13-41`, `42-63`, `66-71`.

| phase | title | stage |
| --- | --- | --- |
| 19 | the terminal is what the build says | `13-41` |
| 24 | there is no mouse | `13-41` |
| 61 | no window title | `42-63` |
| 67 | no mouse, no spell plumbing, no write-only flags | `66-71` |

### text

Operations on text that go: C and lisp indenting, filters and alignment, formatting, rot13 and the operator function, sentence and paragraph motions. Stages `13-41`, `42-63`, `64-65`, `66-71`.

| phase | title | stage |
| --- | --- | --- |
| 28 | C indenting | `13-41` |
| 44 | no filters, sorting or alignment | `42-63` |
| 57 | no lisp | `42-63` |
| 64 | no formatting, comment or nroff-macro options | `64-65` |
| 65 | no rot13, no operator function, no empty key handler | `64-65` |
| 66 | no sentences, paragraphs, sections, methods, #if blocks or comment blocks | `66-71` |

Relies on:

- **44** after **8** (`files`), *rationale* — :r !cmd and :w !cmd keep reaching do_bang() for 8's refusal
- **64** after **60** (`options`), *mechanical* — = only re-applied the existing indent once 'equalprg' went in 60
- **65** after **55** (`options`), *rationale* — g@ had no 'operatorfunc' to call after 55
- **65** after **32** (`completion`), *mechanical* — ins_ctrl_x() is empty since 32 (tools/nocomplkeys.py), so its call goes

Relied on by: 60 (`options`).

### scripts

Anything that runs later or from a file: user commands, scripts and sessions, autocommands. Stages `13-41`, `73-77`.

| phase | title | stage |
| --- | --- | --- |
| 29 | `:command`, user-defined commands | `13-41` |
| 35 | no scripts, no session, no autocommands | `13-41` |
| 75 | no autocommands | `73-77` |

Relies on:

- **35** after **18** (`startup`), *rationale* — a script has nowhere to come from: nothing is read at startup since 18

Relied on by: 62 (`options`), 68 (`windows`), 78 (`tidy`), 79 (`tidy`).

### completion

Insert-mode and command-line completion. Stages `13-41`, `42-63`.

| phase | title | stage |
| --- | --- | --- |
| 32 | insert completion, the popup menu, and the keys that reached them | `13-41` |
| 59 | no command-line completion | `42-63` |

Relies on:

- **32** after **10** (`tags`), *rationale* — the tag source of CTRL-X completion was already gone
- **32** after **7** (`files`), *rationale* — file-name completion went through the globbing 7 removed

Relied on by: 65 (`text`), 79 (`tidy`).

### commands

The Ex command layer itself: rows that only refuse, rows that duplicate a key, the table, and the bar. Stages `13-41`, `42-63`, `80`, `81`.

| phase | title | stage |
| --- | --- | --- |
| 33 | commands whose machinery has already gone | `13-41` |
| 37 | no command that does nothing | `13-41` |
| 47 | no `:startinsert`, `:startreplace`, `:startgreplace` or `:stopinsert` | `42-63` |
| 80 | the Ex command table, cut to the commands that exist | `80` |
| 81 | one line, one command | `81` |

Relies on:

- **33** after **8** (`files`), *rationale* — :shell's row goes because it has answered E319 since 8
- **80** after **79** (`tidy`), *mechanical* — the length field's one reader, the Vim9 whole-name check, is dead since 79

### mappings

Keys the editor rewrites as they are typed: abbreviations and language mappings. Stages `13-41`, `42-63`.

| phase | title | stage |
| --- | --- | --- |
| 34 | no abbreviations | `13-41` |
| 58 | no language mappings | `42-63` |

### windows

One tab page, one window, one frame — first the commands, then the structure. Stages `13-41`, `66-71`, `72`, `73-77`.

| phase | title | stage |
| --- | --- | --- |
| 36 | one tab page, always | `13-41` |
| 39 | one window, always | `13-41` |
| 40 | no window sizes to set | `13-41` |
| 68 | one window, structurally | `66-71` |
| 72 | one window, one tabpage, structurally | `72` |
| 73 | one frame | `73-77` |

Relies on:

- **68** after **35** (`scripts`), *mechanical* — the autocommand window ran autocommands, and dispatch is FALSE since 35

Relied on by: 45 (`buffers`), 46 (`buffers`), 77 (`buffers`), 78 (`tidy`), 79 (`tidy`).

### buffers

One buffer and no argument list — first the commands, then the structure. Stages `13-41`, `42-63`, `66-71`, `73-77`.

| phase | title | stage |
| --- | --- | --- |
| 38 | the argument list is walked by `:next` and `:previous` alone | `13-41` |
| 41 | the buffer list is walked by `:bnext` and `:bprevious` alone | `13-41` |
| 42 | one buffer, always | `42-63` |
| 45 | no `:drop` | `42-63` |
| 46 | no `:wall`, `:qall`, `:quitall`, `:wqall` or `:xall` | `42-63` |
| 69 | one file argument, and no argument list | `66-71` |
| 70 | :e reloads in place, and there is no swap file | `66-71` |
| 71 | one buffer, structurally | `66-71` |
| 77 | no buffer-name argument matching | `73-77` |

Relies on:

- **45** after **39** (`windows`), *rationale* — with one window :drop was :args plus :first
- **46** after **39** (`windows`), *mechanical* — with one window :qall is :q, which is what exsweep.py falls back to
- **70** after **11** (`swap`), *mechanical* — ml_open_file() is only `b_may_swap = FALSE` since 11
- **70** after **21** (`swap`), *mechanical* — findswapname, swapfile_info and ml_recover went with recovery in 21
- **77** after **11** (`swap`), *mechanical* — it asserts every EX_BUFNAME row is ex_ni; :checktime went in 11
- **77** after **39** (`windows`), *mechanical* — the same assertion; :sbuffer went in 39

Relied on by: 74 (`tags`), 79 (`tidy`).

### tidy

What no package owns: empty functions, write-only counters, constant predicates, headers and comments. Stages `78`, `79`, `82`.

| phase | title | stage |
| --- | --- | --- |
| 78 | empty functions, write-only counters, and the window id | `78` |
| 79 | the constant-return predicates | `79` |
| 82 | the system headers nothing needs, and every comment | `82` |

Relies on:

- **78** after **75** (`scripts`), *mechanical* — autocmd_blocked and prevwin lost their last readers in 75
- **78** after **68** (`windows`), *mechanical* — w_id is a constant only because 68 left one window
- **79** after **13** (`files`), *mechanical* — asserts check_timestamps() is `return 0;`, which 13 made it
- **79** after **17** (`encodings`), *mechanical* — asserts bomb_size() is `return 0;`, which 17 made it
- **79** after **32** (`completion`), *mechanical* — asserts pum_visible(), ins_compl_active() and five more are constant (32)
- **79** after **35** (`scripts`), *mechanical* — asserts in_vim9script() and the `has_*()` event tests are FALSE (35)
- **79** after **59** (`completion`), *mechanical* — asserts wc_use_keyname() is `return FALSE;`, which 59 made it
- **79** after **36** (`windows`), *mechanical* — asserts tabline_height() is `return 0;`, which 36 made it
- **79** after **39** (`windows`), *mechanical* — asserts check_can_set_curbuf_forceit/_disabled() are TRUE (39)
- **79** after **68** (`windows`), *mechanical* — asserts only_one_window() is `return TRUE;`, which 68 made it
- **79** after **72** (`windows`), *mechanical* — asserts current_win_nr() and current_tab_nr() are `return 1;` (72)
- **79** after **73** (`windows`), *mechanical* — asserts stl_connected() is `return FALSE;`, which 73 made it
- **79** after **69** (`buffers`), *mechanical* — asserts check_more() is OK and append_arg_number() is 0 (69)

Relied on by: 80 (`commands`).

## Phase 0 — seed, and prove the copy is a copy

`whim-vim.c` starts as a byte-for-byte copy of `slim-vim.c`, and the phase's
only job is to establish that. It matters because everything after it is
measured as a delta: if the seed is not identical, every later phase's report
is against the wrong thing.

The check is `cmp`, and the boundary digest is the same file's.

## Phase 1 — no `$VIMRUNTIME`

**The first ground truth: there is no runtime directory.** Nothing is installed
beside the binary, so every path that goes looking for one is dead weight and,
worse, a promise the editor cannot keep — `:help` that opens nothing is more
confusing than `:help` that says it is not implemented.

Four entry points are cut, and everything unreachable behind them is *found*
rather than listed:

- **Six command rows point at `ex_ni`**: `:help`, `:helpclose`, `:helptags`,
  `:runtime`, `:exusage`, `:viusage`. All six exist only to read or display
  files from the runtime directory.
- **`'helpfile'` and `'runtimepath'` default to `""`**, in both halves of the
  `{vi, vim}` pair. They named `$VIMRUNTIME/doc/help.txt` and a five-element
  path through `~/.vim` and `$VIM/vimfiles`.
- **The `VIMRUNTIME` branches of `vim_getenv()` and `vim_setenv()` go.** That
  is the layer that *derives* a runtime directory from the executable's own
  path when the variable is unset, which is precisely the behaviour an embedded
  binary must not have.
- **The `help.c` region** — 981 lines, 13 functions — is then unreachable and
  the sweep removes it, along with whatever else it was the only caller of.

**The delta this is allowed to cause**, and nothing else: the six commands
report `E319` instead of acting, and `:set helpfile? runtimepath?` report
empty. Every other behaviour case, every other Ex command, every pty scenario
and the whole terminal table are unchanged. The harness checks exactly that
against `slim-vim`'s recorded baselines, and then records `whim-vim`'s own.

### The trap

`:help` is not the only way in. `'helpfile'` is read by anything that opens
help, `$VIMRUNTIME` is consulted by the vimrc search, and `:runtime` is what
`:packadd` was built on. Cutting the commands without cutting the option
defaults leaves an editor that still tries to open a file it will never find —
which is why the option defaults are part of *this* phase and not a later one.

## Phase 2 — the options for features that are not here

**There are no commands to cut, and checking that first is the point.** All
fourteen `:menu` commands and all eight `:spell` ones are *already* `ex_ni`:
upstream's `tiny` configuration never compiled them, and the slim pipeline's
empty-object prune removed their sources. A phase that repointed them would be
busywork dressed as progress, and this one asserts the fact rather than assuming
it — if a handler ever comes back, it fails and says so.

What survived those features is their **settings**. Six spell options and one
menu option are still in the table, still settable, still listed by `:set all`,
and read by nothing whatsoever. That is the same lie `:help` told: a control the
editor offers and cannot honour. So `'spell'`, `'spellcapcheck'`,
`'spellfile'`, `'spelllang'`, `'spelloptions'`, `'spellsuggest'` and
`'menuitems'` go, along with their entries in `modeline_whitelist[]`, which
would otherwise outlive the options they name.

**`'mousemodel'` is deliberately kept**, and it is the interesting one. It looks
like a menu option and is not: `:behave` sets it, and it selects how a mouse
click behaves in a terminal — which this build still does.

**The delta is cumulative and does not grow here.** `:set spell` becomes E518
and `:set all` stops listing seven options, but the Ex sweep exercises commands
rather than settings, so it records nothing new. The evidence that this phase
did something is the score, not the delta — which is the honest way round, and
better than inventing a delta to point at.

## Phase 3 — no introduction, and the command line says only what the editor still decides

**An embedded editor starts in a buffer, not on a title card, and is started by
something that knows what it wants.** This was two phases with a third's worth
of work left undone between them. They were one question — *what may an
invocation say?* — and are answered once.

### The introduction

- **`:intro` and `:version` point at `ex_ni`.**
- **The splash screen's two call sites go.** `maybe_intro_message()` is called
  from the *redraw path* when the buffer is empty and no file was named. It is
  not a command, so an editor whose `:intro` was `ex_ni` would still greet you
  on startup.
- **`-h`, `-?`, `--help` and `--version` go**, and with them `usage()` and
  `list_version()`, which `--version` was the other door to.

That last is where the removal pays. With `list_version()` gone the sweep takes
the version tables, the feature lists, and `pathdef`'s `compiled_user` and
`compiled_sys`, which bake the *building machine's hostname* into the binary.
Measured: the name appears once in this phase's input and nowhere in its output.
That is worth removing on an embedded artifact's account and worth removing
twice on a reproducible one — a binary that names the machine that built it
cannot be byte-identical anywhere else.

### The command line

`tools/dropopts.py` deletes each option's `case` label or `else if` link, so the
option reaches `mainerr(ME_UNKNOWN_OPTION)` — the path anything unrecognised
already takes. `tools/optreaders.py` then removes what only a dropped option
could ever set, and every reader of it, because a field nothing sets is still
*read*: no warning names it and no sweep can take it.

| | options | |
| --- | --- | --- |
| **refusing** | `-A`, `-F`, `-H`, `-g`, `-nb` | print "not enabled at compile time" and exit — what an unknown option does anyway, one message less specifically |
| **inert** | `-f`, `-X`, `-Y`, `-d`, `-U`, `--nofork`, `--literal`, `--gui-dialog-file`, `--startuptime`, `--log` | accepted with an empty body, or an argument that goes nowhere |
| **said another way** | `-l`, `-C`, `-N`, `-V`, `--noplugin` | each is a `:set` — `lisp showmatch`, `compatible`, `nocompatible`, `verbose` and `verbosefile`, `noloadplugins` |
| | `-n` | `'updatecount'` to 0, so no swap file is written; `:set updatecount=0` says the same, and from Phase 11 there is no swap file on disk to avoid |
| | `-p` | the files as tab pages; `-o` and `-O` still lay them out as windows |
| | `--clean` | `-u DEFAULTS`, and empty defaults for `'runtimepath'` and `'packpath'` |
| **a capability** | `--not-a-term` | see below |

**`--not-a-term` goes on purpose, and it is the one that removes something.** It
told a full-screen run with no terminal not to warn, not to wait, and not to
restore a title. Without it that run warns and waits two seconds, as it did
before the option existed. An embedded editor is given a terminal or run with
`-e`, and every harness here runs `-e -s`.

What goes with them, found by `optreaders.py` rather than by the sweep:
`early_arg_scan()`, which existed to refuse `-nb` before anything else ran; the
pre-scan of `argv` at the top of `main()` that set `params.clean` before options
existed, `set_init_1()`'s parameter, and `set_init_clean_rtp()`; `is_not_a_term()`
and `is_not_a_term_or_gui()`, whose eight callers each keep the branch they took
without the option; the reader that turned `-n` into `'updatecount'`;
`WIN_TABS` at seven tests in `create_windows()` and `edit_buffers()`,
`p_shm_save`, and `make_tabpages()`; and `More info with: "vim -h"`, which ended
every usage error by naming a removed option and a binary this one is not.

**Not here:** `-y`, `-Z`, `-t` and `-i` go in Phase 18, and `-r` and `-L` in
Phase 21, each with the capability it selected — a flag is pointless only once
the thing it chose is gone.

### The trap, and the harness it needed

`dropopts.py` removed a long option's `else if` and then asked whether the text
*before* it ended in `else` — which it never did, because the match had already
consumed that `else`. So removing **any** link turned the next `else if` into a
bare `if`, and the chain came apart: `--clean`, `--noplugin` and `--not-a-term`
each matched their own branch, failed every test after it, and reached `mainerr`
anyway. **Three options broken for thirty phases, and nothing noticed, because no
harness passed a single option** — `behaviour.py`, `exsweep.py` and
`termcheck.py` all ran `-u NONE -e -s` and nothing else. The removed link's own
`else` decides now.

`tools/clicheck.py` is the harness that was missing. It runs every option the
parser has. A dropped one must exit 1 naming itself as unknown; a kept one must
not, and must do what it says wherever `:set`, a file or an exit status can show
it — `-c`, `+`, `--cmd`, `-S` and `-u` each set an option the run then reports,
`-b`, `-R`, `-m`, `-M` and `-w7` report theirs, `-W` and `-w` write their file,
`-v` leaves Ex mode and so warns that there is no terminal, and `--ttyfail`
exits 1. "It did not complain" is accepted only for `-s`, `-o`, `-O`, `-T`, `-`
and `--`, whose effects need a terminal to see. **Proven able to fail:** against
`slim-vim` 30 of its 52 cases are wrong, and against the `whim-vim` built before
this phase 12 are — every option this phase newly drops that still worked,
counting `-p2` and `-V9`.

`case 'X':` also appears in more than one switch in this file — the normal-mode
tables and `get_c_indent()` have their own — so everything `dropopts.py` does is
bounded by `command_line_scan()`'s own text, and a label it removes from one of
the parser's two switches it removes from the other.

### The delta

Cumulative against slim-vim's baselines: `:helpclose` from phase 1, and now
`:intro` and `:version`, which succeed in slim-vim and report E319 here. Nothing
else may move — and the pty scenarios are the ones to watch, since a startup
screen is exactly the kind of thing a terminal harness records. The command line
is `clicheck.py`'s to check, because nothing else ever passes an option.

Measured: **180,328 → 178,431 lines**, 1,368 of them taken by the sweep in three
rounds, and libc symbols 146 → 146 — the introduction and the command line were
never what the editor needed from the world.

## Phase 4 — the binary's name stops choosing what it does

`parse_command_name()` reads `argv[0]` and picks a mode from it: a leading `r`
is restricted mode, `e` selects evim, `g` the GUI, and `view`, `diff` and `ex`
prefixes each change it again. **That is a Unix *installation* convention** —
symlink `rvim`, `view` and `ex` at one binary and let the name decide — and an
embedded editor, which is one file that was never installed, has no use for it.

It is also the trap this repository has paid for more than once. A reference
binary saved as `ref` runs restricted, where every shell-out fails. Renaming the
product to `slim-vim` needed a side-by-side check before it could be trusted.
And every harness here stages the binary under test as `vim` for no reason
except this function. Removing it removes the whole class.

**Nothing is lost, and that is checked rather than asserted.** Every mode the
name could select has an option that selects it explicitly, and
`tools/noargv0.py` refuses to run unless all of them are still there:

| | | |
| --- | --- | --- |
| `-Z` restricted | `-R` readonly | `-y` evim |
| `-e` Ex mode | `-E` improved Ex | |

`diff` is not among them because it never selected a mode here: this build has
no diff feature, and the name only ever printed that and exited. `-d`, which
looks like its option, was an argument that went nowhere, and Phase 3 dropped
it.

**Corrected:** an earlier draft of this section claimed `view` set
`'undolevels'` to 10000 where `-R` did not. It is wrong. `p_uc = 10000` appears
at both sites in `slim-vim.c` — once in the `view` branch and once in the `-R`
case — so the two are exactly equivalent and the removal loses nothing at all.
The claim was written from the name-parsing code without checking the option
beside it, which is the mistake this document warns about everywhere else.

**The delta: none.** The harnesses stage the binary as `vim`, which selected
plain vim mode before and selects it now, so nothing they record can move. The
evidence is the score — and the fact that `whim-vim` can now be called anything
at all.

## Phase 5 — one regexp engine, not two

vim carries two regexp engines and an option to choose between them. **That is a
migration path** — the NFA engine was new once, and `'regexpengine'` existed so a
user could go back when it misbehaved — and an embedded fork inherits the
machinery without inheriting the reason.

This is the first removal here driven by *measurement* rather than by category.
`'regexpengine'` is compiled in as `1`, so nothing this editor does by default
enters the NFA code, and `tools/coverage.sh` never reached a line of it across
the behaviour cases, all 600 Ex commands and the pty scenarios. It was the
largest single entry on that list — `nfa_emit_equi_class` alone is 4,122 lines.

**It is not unused, so this is a decision.** `:set re=2` and `\%#=2` reach it,
and both go: the option is dropped and `vim_regcomp()` stops choosing.

**Checked before cutting**: the custom delimiter atoms this tree's upstream
branch exists for are implemented in *both* engines — `delimiter_atom` appears
once in the `regexp_bt.c` region and again in `regexp_nfa.c` — so the
backtracking engine keeps them and the feature survives intact. Removing the
engine that happened to carry a feature nothing else implements would have been
the one unrecoverable mistake available here.

Three entry points: `vim_regcomp()` compiles with the backtracking engine
unconditionally, `prog_magic_wrong()` stops asking whether a program came from
the NFA engine, and `'regexpengine'` goes through the same tool that dropped the
spell options.

**And the sweep could not finish it, which is this phase's real lesson.** With
the entry points cut, thirteen thousand lines were reachable from nothing — and
`-Wall` said not a word, because every function in the NFA engine is *mentioned*
by another function in it. A recursive-descent parser (`nfa_reg` →
`nfa_regbranch` → `nfa_regconcat` → `nfa_regpiece` → `nfa_regatom` → `nfa_reg`)
and a mutually recursive matcher (`nfa_regmatch` ↔ `addstate`) are immune to
reference counting by construction. `CLAUDE.md` records this trap for *types*;
it is the same shape for functions and nothing here computed it.

`tools/funcreach.py` is `typereach.py`'s argument applied to functions:
reachability from roots, not reference counts. Roots are `main` and every
function named outside all bodies — a handler in `cmdnames[]`, a callback in a
struct — **with prototypes stripped, because a declaration is not a use** and
this file has two thousand of them naming everything there is. It found 54
functions holding 13,376 lines, every one in the `regexp_nfa.c` region,
including seven that do not carry the prefix and that any name-based rule would
have missed.

**The delta: none the harness records.** It never sets `'regexpengine'` and
never writes `\%#=`, and every pattern it does use is compiled by the same
engine as before. That is what a default the product never changed means.

## Three phases moved to SLIM-GOAL.md

They were "the forward declarations nothing needs" and "every definition says
its own linkage", and this was the wrong home for them. **Neither removes a
capability**, which is the only thing this document is for; both are simply true
of a single translation unit whatever it contains, so they belong to whichever
pipeline first has one — which is the slim one, from its Phase 6 onward. They
are `SLIM-GOAL.md` Phases 10 and 11 now, and `slim-vim.c` carries their result.

Keeping them here had a cost beyond misfiling. `tools/allstatic.py` did in one
pass exactly what slim's Phase 8 was doing with one process per symbol — two
pipelines away from the phase that needed it — and that duplication is why
slim's Phase 8 took 369 seconds instead of 115.

The third followed them later. "The table moves below what it names" moved
`cmdnames[]` below its handlers so the declarations it forced could go with the
rest, and that is a fact about a translation unit too: it is part of slim's
Phase 10 now.

## Phase 6 — the editor stops writing shell scripts, and stops drawing a menu

Two cuts, both at the boundary between the editor and everything outside it.

### Wildcards go to the shell, or nowhere

`expand_wildcards()` has **two** expanders behind it and only one of them is the
editor's own. `gen_expand_wildcards()` walks directories itself — `opendir`,
`readdir`, `unix_expandpath()` — and handles `*`, `?`, `[...]`, `~` and `$VAR`
without leaving the process. Everything it cannot do it hands to
`mch_expand_wildcards()`, which is a different animal: it sniffs `'shell'` for
csh, zsh or bash, picks one of five quoting styles, writes a shell *function*
into a temporary file, runs the shell and parses back a NUL-separated list.

That second expander is the editor doing the shell's job in 250 lines, and it
goes. **Shell-out itself stays** — `:!`, `:%!`, `:r !` and the `` `= `` form are
untouched — but the editor no longer generates shell in order to expand a
pattern. What reaches that path now returns unexpanded, which is exactly what
`save_patterns()` already did for a pattern with no wildcard in it at all.

One thing the cut has to carry with it: `save_patterns()` is defined sixty
thousand lines *below* its new caller, so it needs the forward declaration the
old expander's used to hold. Reusing that slot keeps `static` on it, which is
the difference between a file-local function and a new external symbol — hence
the `nm` check at the end of this phase as well as SLIM-GOAL.md Phase 11's.

### The completion menu, in both of its forms

`'wildmenu'` draws the completion matches as a horizontal menu in the status
line and rebinds the arrow keys to walk it; `'wildoptions'=pum` draws the same
matches as a popup. Both are a *display* of what Tab completion already
computed, and both cost a control path reaching from the option table through
key translation into the redraw code.

**Dropping the option is not enough, and that is this phase's real lesson.**
`p_wmnu` is read at thirteen places, and the dead-code sweep counts references:
a variable that is never assigned TRUE makes every one of those branches
unreachable, and every one of them is still a reference. So `p_wmnu` is folded
to FALSE *at the source*, which turns thirteen reachability questions into the
one question the sweep can answer.

The popup form goes for the mirror image of that reason. With the option gone
`cmdline_pum_active()` can only ever answer FALSE — while still being *called*
ten times, which keeps two hundred lines alive that can no longer run. **The
popup menu itself stays**: `pum_display()` has a second caller in insert-mode
completion, so only the command line's use of it is cut. `'wildoptions'` keeps
its other three values and loses `pum`, because an option value that is still
accepted and now does nothing is what Phase 3 exists to prevent.

### The delta

`:e {a,b}.txt`, `:e 'quoted'` and a backtick in a file argument stop expanding
and name a file literally. `:e *.c`, `:e ~/x`, `:e $HOME/x` and file-name
completion are the native path and do not move. `'wildmenu'` and the `pum`
value of `'wildoptions'` stop existing, so Tab completion behaves as it does
under `set nowildmenu` — which is what this build now always is. **No Ex
command changes**, so the cumulative list is still `helpclose intro version`.

**The libc surface does not move at all, and that was expected.** Shell-out
keeps `fork`, `execvp`, `pipe` and `waitpid`. This phase buys complexity, not
dependencies — 1,109 lines of it — and it is worth doing on those terms alone.

**The directory syscalls are held by three things, and only one of them is the
wildcard layer**, which is worth writing down because it is the obvious next
guess and it is wrong. `unix_expandpath()` is the native expander. `readdir_core()`
is reached only from `delete_recursive()`, which removes the temp directory
tree. `vim_opentempdir()` holds an open `dirfd` on that directory as a lock.
The second and third are the **temp directory**, which exists for shell-out —
`:%!sort` writes a temp file — so they stay for as long as `:!` does.
`getcwd` is not in this layer at all: it is `mch_dirname()`, with eighteen
callers across `:pwd`, `:cd`, full-path resolution and the file finder.

Measured, rather than reasoned about: stubbing `gen_expand_wildcards()` to hand
every pattern back unexpanded — deleting the editor's own globbing outright —
removes 1,025 further lines and **not one libc symbol**. `opendir`, `readdir`,
`closedir`, `getcwd` and `lstat` all survive it. Lowering the surface is a
different question from this one, and the answer to it is not here.

## Phase 7 — the editor stops looking for files it was not given

Two removals that are the same thing seen from two sides: the editor asking the
filesystem what is around the file it was handed.

### Wildcards, the rest of the way

Phase 6 removed the expander that wrote shell scripts. This removes the
editor's own. `gen_expand_wildcards()` walked directories with `opendir` and
`readdir` to match `*`, `?`, `[...]`, `~` and `$VAR`, and now hands every
pattern back unchanged — which is not a stub written for the occasion but the
path vim already took for a pattern with no wildcard in it, `save_patterns()`,
`backslash_halve()` included.

**This costs something real and the cost was measured before it was chosen.**
`:e *.c` opens one buffer named `*.c`, and **file-name completion stops
working**: `:e ali<Tab>` used to produce `alias.c` by globbing `ali*` and now
produces `ali\*`. A shell expands `*.c` before vim ever sees it, which is the
argument for this living outside; inside the editor it is 1,025 lines.

### The current directory

`:cd`, `:chdir`, `:lcd`, `:lchdir`, `:tcd`, `:tchdir` and `:pwd` are retired to
`ex_ni`. A process with a notion of "where I am" that the user can move is a
process with a filesystem; an embedded editor handed a buffer has neither.

### Two things this does not do, both of which look as though it should

**`opendir` and `readdir` do not go with the globbing.** They are held by the
**temp directory** — `vim_opentempdir()`, and `delete_recursive()` via
`readdir_core()` — which exists so `:%!sort` has somewhere to put a file.
`vim_tempname()` has exactly two callers, `do_filter()` and `get_cmd_output()`,
both of them shell users, so the directory layer dies with shell-out in Phase
8 and not with globbing here. That was measured rather than reasoned about,
after reasoning about it gave the wrong answer twice.

**`getcwd` does not go either.** It is `mch_dirname()`, and `:cd`/`:pwd` are two
of its eleven callers; the rest are `buf_modname`, `mch_FullName`,
`shorten_fnames`, `modify_fname` and the file finder, all of them resolving a
path the user named. Retiring the commands does not touch it.

### The delta

`:e *.c` names a file literally, file-name completion stops completing, and
seven command names report "not implemented" instead of changing or printing a
working directory.

**And `:recover` moves, which this phase did not predict.** The check caught it,
not the author: `recover_names()` finds swap files by building the patterns
`*.sw?`, `.*.sw?` and `.sw?` and expanding them, so an editor that does not
expand patterns cannot find a swap file whose name it was not given. That is a
consequence of removing globbing rather than a bug in it, so it is declared —
the alternative, widening the list until it fits, is how a delta list stops
being a check. It also says something about Phase 10: the swap file is already
half unreachable.

Cumulatively: `helpclose intro version cd chdir lcd lchdir tcd tchdir pwd
recover`.

## Phase 8 — `:!` keeps its name and loses its process

`:!cmd`, `:[range]!cmd`, `:r !cmd`, `:w !cmd` and `:shell` keep their names,
their ranges and their parsing. What goes is everything under them — the fork,
the exec, the pipe, the wait — and **the temporary file with them**, because a
temp file is not interface. It exists only because a Unix shell needs a file to
read a range out of, and there is no longer a shell.

### The placement is the phase

`do_filter()` calls `vim_tempname()` *before* it reaches `mch_call_shell()`. So
stubbing the shell alone leaves the whole temporary-directory layer alive,
assembling a file for a command that will never run. Measured on a scratch
build before any of this was written down:

| cut at | libc symbols |
| --- | --- |
| `mch_call_shell` | 146 → 140 |
| `do_filter` / `do_shell` / `get_cmd_output` | 146 → **130** |

The second takes `closedir dirfd execvp flock fork fread fseek ftell mkdtemp
opendir pipe readdir rmdir setsid stdin waitpid`. **This is the first whim
phase whose point is the symbol count**, so the phase *checks* it: a run that
shrank the source and left the surface where it was would have cut in the wrong
place, which is the mistake the phase exists to avoid.

### Three entry points, and one call that outlived them

`do_filter()` and `do_shell()` report in the words `ex_ni` uses for a command
that is not in this build. `get_cmd_output()` returns NULL and says **nothing**
— it is an internal helper whose one caller, `find_locales()`, shells out to
`locale -a` to complete `:language` and already handles NULL; an `emsg` there
would fire on a Tab press rather than on a command.

And `ml_close_all()` calls `vim_deltempdir()` on the way out. Nothing creates a
temp directory any more, but the teardown was unconditional, and it was the last
thing holding `opendir` and `readdir`. Deleting nothing is not worth three
syscalls.

**`do_filter()` and `do_shell()` are left named and reporting rather than
retired to `ex_ni`, and that is deliberate.** An embedded editor with no process
of its own may still be handed a filter by its host, and those two functions are
where it would attach. That is the seam this phase is shaped around.

### The delta

Filtering and shelling out report `E319: Sorry, the command is not available in
this version` instead of running anything. `:language` completion stops listing
locales, silently.

## Phase 9 — the editor stops asking the environment what language it is in

`setlocale(LC_ALL, "")` reads `$LANG`, `$LC_ALL` and `$LC_CTYPE` at startup and
changes how this process compares strings, classifies characters and formats a
time. `:language` lets the user change it again. `enc_locale()` derives
`'encoding'` from `nl_langinfo(CODESET)`. All of it is the editor taking
instruction from whatever environment it happened to be started in.

### One edit here is not a removal, and the phase is wrong without it

**`'encoding'` compiles in as `latin1`.** It is only ever `utf-8` because
`set_init_default_encoding()` asks the locale at startup and overwrites the
default with the answer. Remove that call on its own and this silently becomes
a latin1 editor — every multibyte motion, every `:s` over non-ASCII, every file
read — and it would pass the build, the linkage check and the symbol check
without complaint. So `'encoding'` defaults to `utf-8` in the same edit that
removes the derivation, and the phase **checks the running binary's
`'encoding'`** rather than trusting that it did.

That is not a behaviour change on this target, and that was measured rather than
assumed: musl answers UTF-8 to `nl_langinfo(CODESET)` unconditionally, so the
derived value was already `utf-8` — with `$LANG` set, and with `$LANG` unset.
The change makes the encoding **a property of the build instead of a property of
the machine**, which is the whole point, and it is what Phase 10 builds on.

**And `set_init_default_encoding()` is replaced, not deleted**, which took three
tries to get right. It did three things: ask the locale, re-initialise the
multibyte layer for whatever it answered, and write that back as the option's
default. Only the first is locale. The second is load-bearing and invisible:
`p_enc` is set from the option table, and **nothing acts on it until `mb_init()`
runs**. Delete the call outright and `:set encoding?` says `utf-8` while
`enc_utf8` is still FALSE — the editor claims UTF-8 and behaves like latin1,
which is worse than either. The build is clean, the symbol check passes, and
`:set encoding?` gives the right answer, so nothing above the harness can see
it. Five multibyte behaviour cases could: `à é î` stopped upper-casing. The call
becomes `(void)mb_init();`.

Two smaller traps in the same phase, both of a kind this file already records.
`mb_init()`'s `if (enc_dbcs)` block needed **brace matching, not a regex** — a
lazy `(?:[^\n]*\n)*?\}` stops at the first line that is only a brace, which here
is an inner `if`'s, leaving `vim_free(p);` and a stray `}` at file scope, which
gcc reports four hundred lines away as *"data definition has no type or storage
class"*. And `vimconv` **stays**: `mb_init()` tests `vimconv.vc_type` again two
hundred lines below the block, and removing the declaration on the strength of
one visible use is a compile error a long way from the edit.

The phase itself had a third fault worth fixing rather than noting: **an error
is not a warning.** The warning sweep counted lines matching `warning:`, found
none in a run that had failed outright, and `set -e` on the next plain compile
ended the phase with no output at all. It now asks gcc whether it succeeded
before asking what it complained about.

### The four `lang*` options

`'langmap'`, `'langmenu'`, `'langnoremap'` and `'langremap'` are all wired to
`(char_u *)NULL` — they accept a value and store it nowhere. They are Phase 3's
rule arriving late rather than a new decision, and no behaviour can change.

### The delta

`:language` reports that it is not available. Nothing else: the process runs in
the C locale now, which is what it was already running in for every purpose this
build has. `setlocale`, `nl_langinfo` and `strcoll` leave the symbol table.

## Phase 10 — no tag stack

A tag jump is the editor discovering, on its own, that a file it was never told
about exists. `get_tagfname()` walks `'tags'` upward from the current file,
opens whatever it finds and binary-searches it — filesystem-layout knowledge of
exactly the kind Phase 7 removed from `'path'`, and the largest single item
left in the tree at 2,364 lines.

### Four entry points that are not commands

Retiring the fifteen rows is most of it, and would have removed almost nothing
on its own, because each of these keeps the whole subtree alive by itself:

- **`nv_help()` — the `<Help>` key — calls `ex_help()`, which calls `do_tag()`.**
  `:help` has been `ex_ni` since Phase 1, but the *key* was never cut, so the
  entire help-tag search survived a phase that believed it had removed it. This
  is the clearest case in this tree for the rule that entry points are cut, not
  commands.
- `nv_tagpop()` — CTRL-T — calls `do_tag()` straight out of `nv_cmds[]`.
- `ExpandFromContext()` dispatches `EXPAND_TAGS` to `expand_tags()` and
  `EXPAND_HELP` to `find_help_tags()`. Completion is a caller like any other.
- `get_next_completion_match()` dispatches CTRL-X CTRL-] to
  `get_next_tag_completion()`.

**CTRL-`]` is not on that list and does not need to be.** `nv_ident()` builds
the string `":ta "` and runs it as an Ex command, so retiring the row is enough
and the key reports what `:tag` reports — which is also the honest answer.

### What stays

`vim_findfile()`. `'tags'` searching and `'path'` searching share it, and
`find_file_in_path_option()` still serves `:find` and `gf`. Cutting that is a
separate decision from this one, because `gf` is a normal-mode command a user
would miss, and it deserves to be made on its own.

### Two options that cannot go, and the trap they exposed

Six of the eight tag options are dropped. **`'tags'` and `'tagcase'` are
`PV_BOTH` — buffer-local — and their rows are also what initialise their
globals**, because `set_init_1()` sets `p_tags` and `p_tc` by walking
`options[]`. Remove the row and the global stays NULL, and any reader the sweep
does not reach dereferences it at startup.

`'tagcase'` is the one that taught this. Dropping it **built cleanly, swept to
silence, passed the linkage and symbol checks, and segfaulted before the first
keystroke.** From outside, the harness reported it as *every* behaviour case,
the terminal table and *every* Ex command moving at once — which is what a crash
looks like through a delta check. Every option any phase had dropped until then
was `PV_NONE`, so nothing had ever exercised this path.

`tools/dropoptions.py` now refuses a row whose `indir` is not `PV_NONE`, and
says why. Refusing is the right answer rather than handling it: removing the
buffer-local field, its initialiser, its copy, its free and its readers is real
surgery, and it should be a phase that says so rather than a side effect of a
call that looks like the six beside it. The two options stay, inert, until then.

### The delta

**One row moves, not fifteen**, and the difference is worth keeping. Retiring a
command only shows up in the Ex sweep if the command used to *succeed*: `:tag`,
`:tjump` and the rest already failed for want of a tags file to read, and
`ex_ni` fails too, so their recorded exit is unchanged. `:tags` listed an empty
tag stack and exited 0, and now reports instead. CTRL-`]` and CTRL-T report what
`:tag` reports. The declared list is what moved, not what was cut.

## Phase 11 — nothing is written that was not asked for

A swap file is not a recovery add-on bolted to the side of the editor. It is
**memline's backing store**: created beside every file you open, written to as
you type, deleted on a clean exit. For an embedded editor it is the last thing
writing a file nobody asked for, and it is why `'directory'` is searched for a
free `.swp` name and why a 576-line recovery reader exists.

**What goes is the file, not the memline.** `mf_open()` already supports a
memfile with no name — that is what `:set noswapfile` has always produced — so
the buffer keeps its block structure and never acquires a fd. The cost is real
and was agreed before any of it was written: **no crash recovery**, and a buffer
larger than memory can no longer page out to disk.

Five entry points, because `ml_open_file()` has seven callers and no-oping them
one at a time would be seven chances to miss one. `ml_open_file()` returns
having set `b_may_swap = FALSE`, so the callers that retry stop retrying — a
body that merely returned would search `'directory'` again on the next
keystroke. `ml_preserve()`, `ml_sync_all()` and `ml_setname()` become no-ops:
flushing, syncing and renaming a file that does not exist. And the `SEA_RECOVER`
arm of the ATTENTION prompt goes, which is the only way into `ml_recover()` once
`:recover` is retired.

With it go the two other things that wrote without being asked: `:mkvimrc`,
`:mkexrc`, `:mksession` and `:mkview`, which drop a script into the current
directory, and `:checktime`.

### The check this phase exists for

No build can make it, so the phase runs the binary: **edit a file in an empty
directory and nothing may be left beside it.** `ls -A` must show exactly the
file that was edited.

### Not done here

**The automatic timestamp check remains**, for now. `check_timestamps()` is
still called from `main_loop()`, `edit()` and `wait_return()`, so the editor
still notices a file changing underneath it — retiring `:checktime` removed the
command, not the polling. That is a separate cut with a separate delta, and
**Phase 13 is where it happens**.

### The delta, and three things the harness knew better than the author

Eight command names report that they are not available; `'updatecount'` and
`'swapsync'` stop existing. `'swapfile'` cannot go — it is
`PV_BUF` and its row is what initialises the global, the trap Phase 10 records —
so it stays and is now always effectively off.

### `'directory'` was dropped here once, and that was a bug

It is in the list above no longer, and the correction is worth more than the
line it takes. A row is also what **initialises** its global, so a row can only
go once nothing reads the global — and `recover_names()` scans every directory
in `p_dir` looking for swap files, right up until Phase 21 deletes it. Dropping
the row here left `p_dir` NULL for ever, with a live dereference in
`check_overwrite()`, which asks whether *another* vim has a swap file beside the
file you are about to overwrite. So this shipped for twelve phases:

```
:w! <an existing other file>      ->      Segmentation fault
```

**Nothing saw it, and each reason is worth knowing.** The build is clean. The
dead-code sweep is silent, because an orphaned global is *used* — no
unused-variable warning names it. The linkage and symbol checks pass. The Ex
sweep runs every command from its own scratch directory, where the target does
not exist; the 67 behaviour cases write to the file they opened; and neither
writes over an existing file *under a different name* with `!`, which is the one
shape that reaches it.

`dropoptions.py --strict` refuses exactly this and did not exist when this phase
was written. The repair has three parts, and only the first is about this bug:

1. `'directory'` moves to Phase 21, where its last reader goes. Phase 11 keeps
   the row, so `p_dir` is initialised for every phase in between.
2. `tools/orphanopts.py` runs in **every** whim phase, out of `whimdelta.sh`. It
   parses `options[]`, collects every `&p_xx` it names, and compares that with
   every `p_xx` declared at file scope. It is type-aware, which is the whole
   trick: a `long` orphan reads as 0 and is reported, a `char_u *` orphan is
   fatal. `'updatecount'` is genuinely safe to drop here for that reason —
   `p_uc` reading 0 *is* "never create a swap file".
3. `--strict` learned that `varp == (char_u *)&p_x` takes an address rather than
   reading a value. Counting those made it refuse `'directory'` in Phase 21,
   where the row genuinely was inert — a guard that cries wolf gets turned off,
   which would have cost more than the bug did.

`mf_sync()`'s `MFS_FLUSH` tail goes here too, as the last reader of `p_sws`, and
takes `sync()` with it. It sat behind `if (mfp->mf_fd < 0) return FAIL;` and so
was never reached — latent rather than live, and removed for the same reason.

`:mksession` and `:mkview` **do not move**: they already failed. And `:recover`
**leaves** the cumulative list it joined in Phase 7 — removing globbing had
made it fail differently from the slim baseline, and `ex_ni` makes it fail the
same way again, so it stops being a difference. A cumulative delta can shrink,
which is not something a list maintained by hand would ever discover.

## Phase 12 — UTF-8, and no other encoding, ever

Phase 9 made `'encoding'` a property of the build rather than of the machine.
This makes it **not a setting at all**: `mb_init()` accepts `utf-8` and returns
"invalid argument" for anything else, so `:set enc=latin1` fails the way a
misspelt value fails, and the latin1 and DBCS character paths lose their only
caller and are swept.

### The conversion layer is cut at its entry points, not unpicked from its callers

This is the shape of the phase and the reason it is small. `readfile()` is 1,758
lines with conversion woven through a retry loop, partial-character carry-over
and a `goto retry`; `buf_write()` is much the same. Excising that by hand is the
kind of surgery that compiles, passes a symbol check, and corrupts a file on
some path nobody tested.

Instead **six functions answer differently**, and every one of those answers is
a case the callers already branch on:

| | now answers | which is what happens when |
| --- | --- | --- |
| `my_iconv_open()` | failure | the system has no iconv — a case upstream supports |
| `convert_setup()` | `CONV_NONE` | source and target encodings are the same |
| `string_convert()` | `NULL` | there is nothing to convert |
| `check_for_bom()` | no BOM found | the file has none |
| `make_bom()` | writes nothing | `'bomb'` is off |
| `convert_input_safe()` | the input unchanged | no input conversion is set up |

Nothing is restructured. The sweep then takes `convert_setup_ext`,
`string_convert_ext` and `iconv_string`, because nothing reaches them.

**Only then** are the seven branches that can no longer be taken deleted — and
only because the calls inside them are what keep `iconv`, `iconv_open` and
`iconv_close` in the symbol table. A dependency that is linked in and never
reached is exactly what this pipeline exists to remove. Each is a brace-matched
`if` with no `else`, and **every anchor names a line of the body, not just the
condition**: `if (fio_flags == 0)` occurs twice in `readfile()` and the first has
an `else` after it, so the obvious anchor deleted the wrong block and left an
orphaned `else` — which gcc reported as *"expected `}` before `else`"* and then
as two undefined labels six hundred lines away. The same shape as
`funcreach.py`'s two regex bugs: a span that ended in the wrong place.

### Two checks no build can make

An editor that silently stopped being a UTF-8 editor passes the build, the
linkage check and the symbol check. So the phase runs the binary: `'encoding'`
must report `utf-8`, and `gUU` over `à é` must produce `À É` — byte for byte,
`c3 80 c3 89 0a`. It also asserts that **no symbol beginning `iconv` is linked**,
asked of the object and after the sweep, because asking the source beforehand
gets the wrong answer: `iconv_string()` is still there at that point and it is
the sweep that removes it.

### What cannot go, and the rule that finally states why

**Of the six encoding options, only `'charconvert'` can actually be removed.**
The other five each fail for what turns out to be the same reason, arrived at
three times by three different routes:

| | why it stays |
| --- | --- |
| `'encoding'` | `PV_NONE`, but `p_enc` is read in twenty-nine places |
| `'fileencodings'` | `PV_NONE`, not reached by name — and `readfile()` dereferences `p_fencs` |
| `'termencoding'` | `PV_NONE` — and `did_set_encoding()` dereferences `p_tenc` |
| `'fileencoding'`, `'bomb'` | `PV_BUF`, the trap Phase 10 recorded |

**A row is what initialises its global.** Phase 10 found that for a
buffer-local option and guarded on `PV_`; this phase found it for a `PV_NONE`
option reached by *name* (`set_string_option_direct((char_u *)"fencs", …)`,
which answers `E685` and then segfaults) and then again for one reached only
through its variable. The `PV_` test and the name test are both special cases of
the real invariant: **an option is inert only when nothing reads its global any
more.** One whose feature has truly gone has an unread global and the sweep
deletes it a moment later; one that is still read is not inert, it is live code
with its initialiser removed.

**The `PV_` test is unconditional; the other two are `--strict`, and that
distinction is not tidiness.** `PV_BUF` is a property of the row, true whenever
you look. "Nothing reads this global" is only true *after the sweep*, and most
phases drop their options before it — so asking then names the readers the sweep
is about to delete. Phase 2 (`'spell'`) and Phase 5 (`'regexpengine'`) both
fail that question and are both correct. This phase drops after sweeping and so
asks in strict mode. It is the same mistake this phase made twice more — a check
placed one step too early — and it is worth naming because it looks like
rigour.

`'fileencodings'` keeps its row and loses its content — empty is the branch
`readfile()` already takes when a user empties it.

### The delta

A byte-order mark becomes three ordinary bytes at the top of the buffer, which
is what ignoring it means, and the `bomb_on` behaviour case moves because of it.
`'charconvert'` stops existing. **No Ex command moves.**

## Phase 13 — the editor stops re-reading a file it has already read

vim watches the files it holds. `check_timestamps()` walks every buffer and
stats its file — from the main loop, from insert mode, from the `Press ENTER`
prompt, and whenever the terminal regains focus — and `buf_check_timestamp()`
does the same for one buffer on entering it. If the file moved underneath it
prompts, and with `'autoread'` it reloads.

That is the editor initiating filesystem traffic on its own account, which is
the boundary this fork narrows. **Phase 11 retired `:checktime`, which removed
the command; this removes the polling, which is what actually reached the
disk.** What is left is an editor that reads a file when told to and writes it
when told to.

`check_timestamps()` returns 0 without looking at anything, and its four callers
are left calling it. Stubbing rather than unpicking them is deliberate: each
sits in a different control structure and each already handles that answer. The
three direct `buf_check_timestamp()` calls — in `do_ecmd()`, `enter_buffer()`
and `ex_drop()` — are deleted, because with the poll gone they are the only
thing keeping 339 lines of checking and reloading alive.

**`check_mtime()` stays.** `buf_write()` calls it before overwriting a file that
changed since it was read, and that is not polling: it happens only when the
user asks to write, and it is what stops a write silently clobbering someone
else's edit. `b_mtime_read` is still recorded on read, so it still works.

`'autoread'` cannot go — `PV_BOTH`, and its row is what initialises the global.
It stays, and now decides nothing.

### The delta

**None the harness records.** Nothing it does changes a file behind the editor's
back, so nothing it does reaches this code — which is worth stating rather than
glossing, because a phase with no delta is either well-chosen or untested, and
the only way to tell them apart is to say which you think it is.

## Phase 14 — a file name means the file of that name

`'path'` searching is the last of the three ways this editor knew where files
live, after globbing (Phase 7) and `'tags'` (Phase 10). `vim_findfile()` walks a
path list downward and upward, remembers directories it has visited so a symlink
loop cannot trap it, and can be asked for the second match and the third — 866
lines of filesystem-layout knowledge behind `:find`, `:sfind`, `:tabfind` and
`gf`.

`:find`, `:sfind` and `:tabfind` are retired: the whole of what they do is the
search.

**`gf` is kept, and resolves the name literally.** It is the one place a user
names a file from *inside the buffer* rather than on a command line, and taking
it away would be taking away the naming rather than the searching. So
`find_file_in_path()` stops consulting `'path'` and answers the only question
left — is there a file of this name? Fifteen lines against eight hundred and
sixty-six, and it reaches the filesystem no differently from `:e`.

Two details of the contract it has to keep, both visible in
`find_file_name_in_path()`: `first == FALSE` asks for the *next* match, which is
what `3gf` and `]f` use, and there is never a next one now — so it answers NULL
and the caller's loop ends, which is the same answer the search gave when the
path held one match. And the result is owned by the caller, so it is allocated
even though the name is already in hand.

`'path'` and `'suffixesadd'` cannot go — `PV_BOTH` and `PV_BUF`, and a row is
what initialises its global. They stay, and now decide nothing.

### The delta

`gf` opens the name under the cursor if there is a file of that name rather than
searching `'path'` for one. **No Ex command moves** — Phase 10's lesson again
rather than a surprise: retiring a command only shows in the sweep if it used to
*succeed*, and `:find`, `:sfind` and `:tabfind` already failed for want of an
argument.

## Phase 15 — the last two encoding options

**Phase 12 emptied `'fileencodings'` and said so, and it was true at startup and
not afterwards.** `set_option_default()` special-cases the option, so `:set
fencs&` restored `ucs-bom,utf-8,default,latin1` from `fencs_utf8_default` — a
third reference Phase 12 did not find, because it names the *string* rather than
the function the other two called. Measured on the shipped binary before this
was written:

```
  at startup           fileencodings=
  after :set fencs&    fileencodings=ucs-bom,utf-8,default,latin1
```

That is worth recording as a pattern and not just a fix. Phase 12 cut two
callers of `set_fencs_unicode()` and asked whether anything still called it;
nothing did. The question it did not ask was whether anything still used the
*value*, and a search for the function name cannot answer that.

Three readers go, and with them the two options can finally follow.
`set_option_default()` stops special-casing `'fileencodings'`, which is what
makes Phase 12's claim true at every moment rather than one. `readfile()` stops
choosing between an empty list and a list to walk, and takes the buffer's own
`'fileencoding'` — the branch the empty case already took. And
`did_set_encoding()` stops setting up a conversion between `'termencoding'` and
`'encoding'`, which `convert_setup()` has answered `CONV_NONE` to since Phase 12,
so the block could only ever have succeeded at doing nothing.

**`'encoding'` still cannot go, and here that stops being temporary.** `p_enc`
is the *name* of the one encoding, compared against in twenty-nine places.
Removing the option would mean removing the name, and the name is doing work.
Of the six encoding options this fork began with, one remains, and it reports
`utf-8` and refuses everything else.

### The delta

**None.** `:set fencs&` no longer restores a list of encodings this build cannot
convert between, which is a correction rather than a change.

## Phase 16 — six options that no longer decide anything

`'path'` and `'suffixesadd'` have been inert since the file finder went,
`'tags'` and `'tagcase'` since the tag stack, `'autoread'` since the timestamp
poll, and `'swapfile'` since the swap file. All six were still here, because a
row is what initialises its global and `tools/dropoptions.py` refuses to leave
one dangling — **Phase 10's trap, which this phase clears rather than works
around.**

### The order is the phase, and it is forced rather than chosen

1. the three readers that are not plumbing
2. the rows, with `--local`
3. **the sweep** — which is what removes `did_set_tagcase()` and
   `did_set_swapfile()`, the option callbacks, reachable only from the rows
4. the buffer fields and their plumbing
5. the sweep again

**Steps 3 and 4 cannot swap**, and the reason is a property of how this pipeline
sweeps rather than of the code. The callbacks read the buffer field, so removing
the field first stops the file compiling; the sweep works by reading gcc's
*warnings*, so a file that does not compile is a file the sweep cannot act on,
and the callbacks would stay for ever. Every other phase has been free to order
its cut however it liked; this one is not.

### The three that are not plumbing

`ex_drop()` set `'autoread'` on, checked the timestamp, and set it back —
and Phase 13 took the check out from between, so what was left was a variable
saved and restored across nothing at all. `do_set_option_bool()` special-cased
`:setlocal autoread` to mean "follow the global", the `-1` sentinel, and there
is no global to follow. `ml_open()` asked whether this buffer may have a swap
file; since Phase 11 the answer has been no whatever `'swapfile'` said, so it
now says no directly.

Everything else is the five fixed idioms every buffer-local option has — the
field in `buf_T`, the initialiser in `buf_copy_options()`, `check_buf_options()`,
`free_buf_options()`, and one or two `get_varp()` cases — which is what makes
`tools/droplocal.py` possible at all. It takes the *field* name rather than the
option's, because by the time it runs the row is already gone and there is
nothing left to look the field up from.

### The delta

**None.** All six report `E518: Unknown option` instead of a value that decided
nothing.

## Phase 17 — the last two per-buffer encoding options

`'fileencoding'` names the encoding a buffer was read in and will be written
back in, and `'bomb'` whether it had a byte-order mark. With one encoding and no
BOM, both have had one possible value since Phase 12 — but **unlike the six
Phase 16 took, these are not plumbing.** Eight functions read them, and each had
to be looked at:

| | what it wanted them for |
| --- | --- |
| `buf_write()`, `readfile()` | the conversion target, and whether to write a BOM |
| `bomb_size()` | how many bytes of the file are a BOM, for `g CTRL-G` |
| `save_file_ff()`, `file_ff_differs()` | remembering the pair, so `:w` can warn they changed |
| `utf_find_illegal()` (`g8`) | converting to the buffer's encoding to find a byte illegal in it |
| `add_b0_fenc()` | writing the name into a swap file's block zero |

None of those questions has more than one answer now, and the last has no swap
file to write into.

**What the options leave behind is a pair of remembered copies in `buf_T`** —
`b_start_fenc` and `b_start_bomb`, written on every read and looked at by
nobody. A struct field is not a variable, so no warning reports it and the
sweep cannot see it, which is why those are listed in the tool rather than left
to fall out. The same is true of `gvarp`, the local that asked *which* encoding
option was being set: there is one.

`'fileformat'`, `'endofline'` and `'endoffile'` can still change under a buffer,
so `file_ff_differs()` keeps those and loses only the two that cannot.

### The delta

**None.** Both report `E518` instead of a value with one possible setting. What
is left is `'encoding'`, alone, reporting `utf-8`.

## Phase 18 — nothing is read at startup, and nothing on the command line decides anything

### nothing is read at startup that was not named on the command line

An editor that goes looking for its own configuration has a filesystem layout in
its head. `source_startup_scripts()` tried, in order:

```
$VIMRUNTIME/evim.vim      $VIMRUNTIME/defaults.vim      $VIM/vimrc
$VIMINIT                  $HOME/.vimrc                  $HOME/.exrc
./.vimrc                  ./.exrc
```

— the last two only with `'exrc'` on, and each guarded by an ownership check,
because reading a config file out of the current directory is a way to be handed
someone else's commands.

All of it goes, **and so does `-u`**. `-u <file>` was the one branch left that
read a file, and `-u NONE` — how every harness kept a vimrc out of a recorded run
— is a no-op once nothing is searched for. With no path to search and no name to
be given, `source_startup_scripts()` has no body, its call goes, and the sweep
takes it; the `-u NONE` test in `main()` that switched `'loadplugins'` off goes
with the option it asked about. `:source` stays until Phase 35.

**It goes here, not later, so that no tool depends on it.** The harnesses are
shared with the slim pipeline, whose editor still searches, so they could not
simply stop passing `-u NONE`. They isolate through the environment instead — an
empty `$HOME`, `$VIM`, `$VIMRUNTIME` and `$XDG_CONFIG_HOME` — which is what `-u
NONE` was for and holds for both editors. Measured against `slim-vim`, with a
real `~/.vimrc` present on the machine: 0 of 67 behaviour cases, 0 of 600 Ex
commands and 0 of 19 terminal rows differ from the baselines recorded with `-u
NONE`, and `tools/verify.sh` is all clear. `tools/clicheck.py` still checks `-u
rc.vim` as an option at Phase 3, where it exists.

`set_init_xdg_rtp()` goes with them. It built a `'runtimepath'` out of
`$XDG_CONFIG_HOME`, and **Phase 1 emptied that option while this was still
filling it back in** — an option reported as empty and rebuilt at startup, which
is the kind of thing only a survey of every `getenv` finds. So does
`process_env()`, which ran `$VIMINIT` or `$EXINIT` as Ex commands, and `'exrc'`,
which selected between two searches that no longer happen.

#### The delta

**None, and that is the point rather than a surprise.** No harness passes `-u`,
and under an empty environment none of these paths is taken. What changes is
that the editor no longer needs to be told — and that it can no longer be told.
The phase checks that `-u NONE` is now an unknown option against a control that
still runs.

### command-line options that no longer decide anything

Four outlived what they controlled, each in a different way.

| | why it is inert |
| --- | --- |
| `-y` | evim mode. `parmp->evim_mode` is assigned and read nowhere — its one reader was the line Phase 18 removed |
| `-Z` | restricted mode, whose purpose is to refuse shell commands. `check_restricted()` has two callers left: `do_bang()`, stubbed in Phase 8, and `ex_stop()`. No *live* command carries `EX_RESTRICT` either — the ten that do are all `ex_script_ni` |
| `-t` | jump to a tag at startup, by running `:ta <tag>`. Phase 10 retired `:tag`, so its whole effect is to run a command that reports it is not implemented |
| `-i` | the viminfo file. `'viminfo'` and `'viminfofile'` are wired to `(char_u *)NULL` in **both** editors — the tiny configuration has no viminfo at all |

**`-u` went above**, with the search it used to suppress.

#### The harnesses change, and that is the check

All three stop passing `-i NONE`, and `slim-vim` — which still has the option —
must still match its recorded baselines afterwards. It does. That is what proves
the option was a no-op *there* too, rather than only here: `-i NONE` has been
doing nothing for as long as this fork has existed, which is exactly why it was
passed for years without anyone noticing.

`set_init_restricted_mode()` goes with `-Z`, and is a small find of its own: it
read `$SHELL` at startup and turned restricted mode on when the answer was
`nologin` or `false`. An environment read, deciding a mode that restricts
nothing. `EX_RESTRICT` comes out of the twenty-four rows that carry it, because
a flag nothing reads is a concept the table still has and the code does not.

#### Two cuts that landed in the wrong place first

Both are the same mistake and both were caught by the compiler rather than by
care. `case 't':` occurs in `get_c_indent()` as well, three thousand lines away
and about `'cinoptions'`, and a substitution with `count=1` takes whichever comes
first *in the file* — the first attempt cut a branch out of the C indenter.
`char_u *tagname;` is also a field of `taggy_T`, seventeen hundred lines
earlier. Everything that edits the option parser is now applied to
`command_line_scan()`'s body alone, and the struct field is anchored on `int
edit_type;`, which sits immediately above it and nowhere else.

#### The delta

**None.**

## Phase 19 — the terminal is what the build says

Five environment variables describe the terminal and the editor believed all of
them: `$TERM` picks a capability table, `$LINES` and `$COLUMNS` override the size
the kernel reports, `$COLORS` overrides the colour count, `$COLORFGBG` the
background.

### The compiled name is `xterm-256color`, and that is the whole care here

Measured on the shipped binary before choosing:

```
TERM=xterm-256color   -> term=xterm-256color  t_Co=256
TERM=xterm            -> term=xterm           t_Co=8
TERM= (unset)         -> term=xterm           t_Co=8
```

`set_termname()` keeps the *requested* name and tests
`strstr(requested, "256color")` to apply `builtin_256colors` on top of whichever
table it chose. So **the obvious fallback — the one the unset case already took
— would have cost eight of every nine colours the terminal can show**, silently,
for nothing. `xterm-256color` resolves to the same `builtin_xterm` table and
keeps the add-on.

### The size is still autodetected

`ioctl(TIOCGWINSZ)` stays; only the `$LINES`/`$COLUMNS` override goes. Verified
on a pty: with the window at 24×80 and `LINES=9 COLUMNS=9` in the environment,
the editor reports 24×80. An editor that believes `$LINES` over the kernel is
one that draws off the bottom of a resized window.

`-T <term>` stays. It is not the environment, and with one compiled default it
is the only way left to say "this is a dumb terminal"; the ten built-in entries
are still there and `-T` still reaches them.

### The delta

**The terminal table collapses.** Nineteen rows, one per `TERM` the harness
tries, each of which used to resolve to its own entry — now every one of them,
including unset and `no-such-term-9x`, answers `term=xterm-256color t_Co=256`.

That is declared with `--term-moved`, which `tools/whimdelta.sh` grew for this
phase. Until now no phase could move that table, so *"expected unchanged"* was
the whole check; a phase that makes every terminal resolve to one entry has to
be able to say so, and the flag asserts the table moved rather than merely
allowing it to.

### `cutil.drop_if`, extracted here

Four phase tools had written their own "delete an `if` and the block it guards",
and four had written the same bug: a lazy `(?:[^\n]*\n)*?\}` to find the end,
which stops at the first line that is only a brace — an inner block's, whenever
there is one. This phase made it five. It is one function in `cutil.py` now,
brace-matched, refusing a block that has an `else`.

## Phase 20 — nothing outside the process is consulted

### there is no home directory

`$HOME` is where an editor keeps the things it was told not to keep. This fork
stopped writing them in Phase 11 and stopped looking for them in Phase 18, and
what was left is the *notion* of a home directory — `~/x` meaning a path, `~bob`
meaning someone else's, and `/home/you/x` displayed back as `~/x`.

All three go, and the last is why this is not only a `getenv` removal:
`home_replace()` has **thirteen callers**, every one a place that shows the user
a file name. It becomes a bounded copy, so the thirteen keep working and a name
is shown as what it is.

The user database goes with them — `init_users()`, `add_user()`, `match_user()`
and `get_users()` exist so that `~bob` can complete, and `mch_get_uname()` so
that a swap file could say who wrote it. Two smaller things fall out and had to
be taken by hand, because `-Wunused-but-set-variable` is not a shape the sweep
deletes: `at_start`, which existed only to know whether a `~` began a path, and
`startstr_len`, measured for the one test that used it.

#### Where the symbol count moves

`getpwnam`, `getpwent`, `setpwent`, `endpwent` — 119 → 115. CLAUDE.md notes that
`getpwnam()` working under static musl is one of the two things that make this
binary honestly standalone. It no longer needs it.

#### Two corrections to what this phase was planned to do

**`getuid` and `getgid` do not go.** `buf_write()` uses them to check ownership
before overwriting a read-only file and to preserve owner and group. That is
file writing, which whim-vim keeps, and the plan was wrong to list them here.

**`getpwuid` does not go either.** `mch_get_uname()` is still reached from
`swapfile_info()`, under `-r`, which lists swap files that cannot exist — Phase
13 removed the swap file and left the option that reads them. That wants a phase
of its own rather than a corner of this one: `ml_recover()` alone is 559 lines,
`recover_names()` 216 and `swapfile_info()` 103.

#### The delta

**None the harness records.** `:e ~/notes` opens a file called `~/notes` in the
current directory, which no harness asks for.

`--term-moved` is **cumulative**, like the command list — the comparison is
always against the slim baseline, and Phase 19 collapsed that table for good, so
every phase after it declares the same thing. Discovered by this phase failing
when it did not.

### nothing is read from the environment

The third and last of the standalone phases. Phase 18 stopped reading
configuration files, Phase 20 stopped believing in a home directory, and this
one removes the environment itself — after it, no answer this editor gives
depends on how it was invoked.

**`vim_getenv()` had already been half dead, and that is what makes this
phase small.** Phase 1 folded its `vimruntime` flag to FALSE, so
`vim_getenv("VIMRUNTIME")` had been returning NULL unconditionally ever since,
and `"VIM"` was the only name left that could reach the `$VIM`/`'helpfile'`
fallback chain — which nothing asks for any more. So the function **can only
ever answer "not set"**, and every caller collapses to the branch it was
already taking:

| what it read | what took its place |
| --- | --- |
| `$VAR` in a file name (`expand_env_esc`) | the name, as written |
| `$PATH` (`expand_shellcmd`) | the pattern's own directory |
| `$VIMRUNTIME` (`fix_help_buffer`) | the `*local-additions*` scan, 111 lines, already a no-op |
| `$SHELL`, `$CDPATH`, `$VIM_POSIX` | the compiled-in defaults |
| `$TMPDIR`, `$TEMP`, `$TMP` | `/tmp`, which was always in the list |
| `$COLORFGBG` | what Phase 19 decided the terminal is |
| `$TZ` | `localtime_r`, which does the zone setup itself |
| `$VIM`, `$VIMRUNTIME`, `$MYVIMDIR`, written | nothing writes them |
| `environ`, walked for `$VAR` completion | the row and its `$`-prefix context go, as `~user`'s did |

`expand_env_esc` is the same answer Phase 20 gave `home_replace`: with the `$`
arm gone what remains is `skipwhite`, the backslash escape and the bound on
`dstlen`, and a name reaches its caller intact.

**`vimrc_found()` was already unreachable**, and finding that out is what kept
this phase from being an argument about whether `$VIM` should still be
published. Every `do_source()` call in the file passes `DOSO_NONE`, so the two
arms that called it have been dead since Phase 18. Deleting them takes
`vim_setenv`, `export_myvimdir` and `$MYVIMDIR` with them.

#### The check is the object, not the source

`getenv`, `setenv`, `unsetenv` and `environ` leave `nm -u`: **115 → 110**, the
fifth being `tzset`. Grepping the source is not sufficient and the phase does
both — the sweep is what removes `vim_getenv`, so asking before it runs gets
the wrong answer, which this pipeline has now learned four times.

#### What stays

`vim_localtime()` still calls `localtime_r()`, and musl reads `$TZ` inside it.
The rule this phase enforces is that *this source* asks the environment
nothing; making a file's timestamp display in UTC would be a different
decision, and not this one.

#### The delta

**None the harness records.** `:w $FOO.txt` writes a file called `$FOO.txt`,
`:e $HOME/notes.txt` needs a directory literally named `$HOME`, and
`:set shell?` says `sh` whatever `$SHELL` was — verified by hand, none of it
something a harness asks for.

## Phase 21 — there is nothing to recover, and the memfile is memory

### there is nothing to recover

Phase 11 made the swap file memory-only: the block structure is still built,
still paged, still where every line of the buffer lives, but it never reaches a
disk. What that left behind is the other half of the feature — the code that
reads *someone else's* swap file back, which is code for reading a file this
editor cannot have written.

`-r` and `-L` are the only two things that ever set `recoverymode`, so the
global folds to FALSE and its seven readers each collapse to the branch they
were already taking. Three of them are in `readfile()`, which had to know
whether it was filling a buffer from a swap file rather than from the file
itself; the other four are the two `-r`-with-no-file arms, the stdin arm, and
the recovery arm of `create_windows()`. `ml_recover()` (559 lines),
`recover_names()` (216) and `swapfile_info()` (103) go with them.

**This is where `getpwuid` goes** — the fifth of the five password-database
symbols, and the one Phase 20 said would need a phase of its own.
`swapfile_info()` called `mch_get_uname()` to say who owned a swap file.

`:recover` was pointed at `ex_ni` earlier and does not move. It already failed,
needing a swap file to read — which is Rule 3's other half: retiring a command
only shows in the Ex sweep if it used to *succeed*.

#### Time, which is the part that is a decision rather than a consequence

`swapfile_info()` was the only caller of `get_ctime()`, which left
`vim_localtime()` with exactly one user: `add_time()`, the timestamp in
`:undolist` and in `1 change; before #3`. It is dropped too, and **not because
it is unreachable**. `localtime_r()` asks libc what the local zone is, and
Phase 20 took away every way this editor could be told; a wall-clock time
without a zone is a wrong answer rather than a partial one. Undo history does
not outlive the process either — `:wundo` and `:rundo` have been `ex_ni` since
Phase 11 — so every time `add_time()` formats is within one session, and the
relative form it already used below 100 seconds is the true one. `strftime` and
both format strings go with it, and `:undolist` now reads `1 second ago` where
it used to read `14:23:07`.

#### Where the symbol count moves

**110 → 107**: `getpwuid`, `localtime_r`, `strftime`.

#### The delta

**None.** Verified by hand: `-r` is now `Unknown option argument: "-r"`,
`:undolist` prints `1 second ago`, and editing is untouched.

### the memfile is memory, and only memory

Phase 11 stopped the editor creating a swap file and Phase 21 stopped it reading
one back. What was left is a **file back-end with no file**: `memfile_T` still
carried a descriptor, still knew how to page a block out and read it in, and
still sized an LRU cache against how much memory the machine has — all of it
behind `if (mfp->mf_fd >= 0)`, and `mf_fd` could no longer be anything but −1.

The proof is short. `mf_open()` has two callers: `ml_open()` passes `(NULL, 0)`,
and `ml_recover()` passed a name — Phase 21 deleted it. Phase 11 stubbed
`ml_open_file()` to `b_may_swap = FALSE`. So nothing can hand the memfile a
name, `mf_do_open()` is unreachable, and `mf_write()` and `mf_read()` return
FAIL on their first lines.

Which makes **`'maxmem'` and `'maxmemtot'` options that decide nothing**:

```c
need_release = (mfp->mf_used_count >= mfp->mf_used_count_max
                || (total_mem_used >> 10) >= (long_u)p_mmt);
...
if (mfp->mf_fd < 0 || !need_release) { return NULL; }
```

`need_release` is the only place either is read, and the test in front of it is
always true — so the answer is computed and discarded. `mch_total_mem()` went to
real trouble to size that cache, through `sysinfo`, `sysconf` and `getrlimit`,
for a cache that never evicts.

Three more things fall out: `mch_get_host_name()`, which wrote the machine's
name into block zero so a recovering vim could say the swap file came from
elsewhere (**`uname`**); `lalloc()`'s retry loop, whose whole point was that
`mf_release_all()` might have freed memory by paging blocks to disk; and
`check_overwrite()`'s "swap file exists" warning.

**What does not change is the block structure.** Lines still live in blocks,
blocks still have numbers, `mf_trans` still maps them. This removes the ability
to *evict* a block, which was already impossible — not the ability to have one.

#### A bug this phase fixes, and where it came from

`check_overwrite()` is the last reader of `p_dir`, so **`'directory'` can
finally go**. Phase 11 dropped its row while this still read it, and a row is
what initialises its global — so `p_dir` was NULL for ever, and

```
:w! <an existing other file>      ->      Segmentation fault
```

shipped for twelve phases. Nothing saw it. The build is clean; an orphaned
global is *used*, so no unused-variable warning names it; the linkage and symbol
checks pass; and neither the Ex sweep nor the 67 behaviour cases write over an
existing file under a different name with `!`.

`dropoptions.py --strict` refuses exactly this and had not been written when
Phase 11 was. The repair is in three parts: Phase 11 keeps `'directory'` and
drops it here instead; `tools/orphanopts.py` checks the invariant in **every**
whim phase, and is type-aware — a `long` orphan reads as 0 and is reported, a
`char_u *` orphan is fatal; and `--strict` learned that `varp == (char_u *)&p_x`
takes an address rather than reading a value, which is what made it refuse a row
that was genuinely inert.

#### Where the symbol count moves

**107 → 104**: `sysinfo`, `getrlimit`, `uname`. `sysconf` stays — its other
caller is `_SC_SIGSTKSZ`, for `sigaltstack`.

#### The delta

**None.** `:w!` over an existing other file stops crashing and writes it, which
is what it should always have done, and the phase asserts that directly — no
harness does.

## Phase 22 — the working directory is where it started

`:cd`, `:lcd` and `:tcd` are `ex_ni`, `:!` no longer forks, and nothing else in
this editor moves the process. So **the directory it starts in is the one it
dies in**, and three pieces of machinery that exist because that was not true
stop being needed.

  * `mch_FullName()` chdir'd into the leading directory of a relative name,
    asked `getcwd()` where that had landed, and chdir'd back — via `fchdir()`
    on a descriptor it held open, falling back to `chdir()`. That dance is what
    resolved `..` and a symlinked directory on the way to a full name.
  * `win_fix_current_dir()` restores a window's or tab's local directory, and
    runs only when `w_localdir`, `tp_localdir` or `globaldir` is set. The first
    two come only from `:lcd` and `:tcd`; `globaldir` is assigned only inside
    this function. Unreachable.
  * `edit_buffers()` takes a `cwd` to return to between `-o` windows, and is
    passed `start_dir` — `static char_u *start_dir = NULL;`, which nothing
    assigns. The `-o` local-directory handling that set it is already gone.

### What it costs

A full name is now the working directory with the name appended, so `../x/y`
becomes `/cwd/../x/y` instead of `/real/x/y`. It opens the same file. What it
loses is that **two spellings of one path no longer compare equal**, so
`:e ../x/y` and `:e /real/x/y` are two buffers rather than one.

### The trap, and the harness that caught it

The first version of this dropped the dance and kept the rest of the function,
which reads `if ((force || !mch_isFullName(fname)) && ...)`. That condition is
true for an *absolute* name when `force` is set — harmless while the dance
existed, because the dance chdir'd to the name's own directory and `getcwd()`
came back with it. Without the dance, the working directory was prepended to a
name that already had one: `/tmp/x` became `/cwd//tmp/x`.

The delta check named it in one line — `:read`, `:write` and `:wq` moved, and
nothing else — which is the whole argument for declaring a delta in advance
rather than reading a diff afterwards. The fix is that `force` has nothing left
to re-resolve, so an absolute name is its own answer.

### `getcwd` stays, and is asked once

It has five callers through `mch_dirname()`: `shorten_fname1()` and
`shorten_fnames()` shorten every displayed name against it, `buf_modname()`
builds names from it, `modify_fname()` implements `%:p`, `fname2fnum()` resolves
a mark's file, and `mch_FullName()` is how a relative name becomes absolute at
all. Dropping it would mean `b_ffname` could not be a full path — a capability
cut rather than plumbing, and a different decision.

Since nothing can move the process, though, the answer cannot change. It is read
into a static on the first call and every later call is a copy: one syscall for
the life of the editor, where there used to be one per path operation.

### Where the symbol count moves

**103 → 101**: `chdir`, `fchdir`.

### The delta

**None the harness records** — and the phase adds a check of its own, because
none of them walks the path this changes: every harness edits a file in the
directory it is standing in. So it writes `sub/f.txt` from above and then
`../sub/f.txt` from inside `sub`, and requires the file to come back correct
both times.

## Phase 23 — no floating-point library

Three calls are the whole of libm in this editor, and they turn out to be two
different questions.

**`ceil()` and `floor()`** appear once, in the fuzzy matcher, as the two halves
of rounding half away from zero:

```c
(fzy_score < 0) ? (int)ceil(fzy_score * SCORE_SCALE - 0.5)
                : (int)floor(fzy_score * SCORE_SCALE + 0.5)
```

C's double-to-int conversion truncates **toward zero**, which is `ceil` for a
negative value and `floor` for a positive one — so biasing by half in the sign's
own direction and then converting gives the same answer for every input, and the
two arms collapse into one expression.

**`log10()` is not translated, because it cannot be**, and finding that out is
the useful part of this phase. It appears once, as
`max_prec -= (size_t)log10(abs_f)`, and the obvious integer equivalent —
dividing by ten until the value drops below ten — **is a different function**.
Just below a power of ten, `log10()` returns a double that rounds up to the
integer:

```
(size_t)log10(99.999999999999986)  ==  2        counting digits gives 1
```

`tools/nolibm_check.c` swept a million values through both forms and found 79
disagreements, all of that shape. A rounding rewrite that is merely believed is
how an off-by-one reaches a release — and here the check turned a translation
into a removal, which is the better phase.

### Nothing can reach the `%f` branch

So the whole floating-point branch of `vim_vsnprintf()` goes instead. The
premise is checkable and the phase checks it: **there is not one `%f`, `%F`,
`%e`, `%E`, `%g` or `%G` conversion in any format string in the file**, and the
single `vim_snprintf()` call whose format is not a literal takes a local
`char *fmt` that is one of two constants, `"%*ld "` and `"%-*ld "`. Without
`+eval` there is no `printf()` to supply one at run time either.

That takes the conversion case (139 lines), `TYPE_FLOAT` and its three arms,
`infinity_str()` and `typename_float` — and with them `log10`, `isinf` and
`isnan`. `TYPE_FLOAT` is the **last** enumerator, checked before removing it,
because several enums here index a parallel table.

`<math.h>` stays: `INFINITY` is the fuzzy matcher's score sentinel, in thirteen
places. Under musl libm is part of libc, so the link line does not change
either — what changes is that `nm -u` stops naming a floating-point function.

### Where the symbol count moves

**101 → 98**: `ceil`, `floor`, `log10`.

### The delta

**None.**

## Phase 24 — there is no mouse

A terminal mouse is a protocol, not a device: the terminal is asked to report
clicks, it sends escape sequences, the editor decodes them into key codes, and
the normal, insert and command-line loops dispatch those like any other key.
All four layers are here, and an editor driven from a keyboard needs none of
them.

**The island is bounded**, which is what makes this a cut rather than a rewrite.
Thirty-five functions mention the mouse and all but two are reached only from
each other, so `funcreach.py` deletes the interior once the roots are gone. The
tool removes only the roots:

| layer | what goes |
| --- | --- |
| the tables | 22 rows of `nv_cmds[]` point at `nv_error`, and the 14 `<LeftMouse>`/`<ScrollWheelUp>`/`<MouseMove>` rows of the key-name table go, so `:map <LeftMouse>` no longer names anything |
| the dispatch | `edit()`'s insert-mode case run, `getcmdline_int()`'s six case runs, `]<LeftMouse>` in `nv_brackets()` and `g<LeftMouse>` in `nv_g_cmd()`, and the click that dismissed a `Press ENTER` prompt |
| the decoder | `check_termcode_mouse()`'s call, and 41 lines in `set_termname()` that read the terminal's 1006 capability, set `'ttymouse'` from it and install the termcodes |
| the switch | `setmouse()`'s **31** calls, every one a bare statement, and `mch_setmouse()`'s |
| the questions | `mouse_has()` and `mouse_has_any()`, whose three callers outside the island each become the answer they now always get |

### The rows of nv_cmds[] are pointed away, never deleted

**This phase first deleted the 22 rows, and the arrow keys stopped working in
normal mode for twelve phases.** Normal mode finds a key's handler through
`nv_cmd_idx[]`, a sorted index into `nv_cmds[]` that upstream generates and this
tree writes into the C as a constant. Deleting rows left the index 22 entries
longer than the table, still compiling, and every key found past the first hole
resolved to another key's row. Nothing noticed because every harness that typed
an arrow typed it in insert mode, which decodes the arrows in a `switch`.

So the rows stay and answer `nv_error` — rule 3, applied to the normal-mode
table, which is what Phase 30 already did for `K` and CTRL-]. Two checks now
guard it: `tools/nvidxcheck.py`, run by `phasecheck.sh` in every phase, requires
the index to be a permutation of the table's rows, and `tools/arrowcheck.py` —
retired after Phase 82, see there — pressed all four arrows in normal mode, in the `ESC O` form a terminal sends once
vim has switched its keypad to application mode.

### Two names that are not about the mouse

Both checked rather than assumed. `get_mouse_class()`, `find_start_of_word()`
and `find_end_of_word()` classify characters for **double-click word
selection** and are reached only from `do_mouse()`, so they go with it — the
name says mouse and the work is text, which is exactly the shape that survives a
careless sweep.

`WaitForCharOrMouse()` has no mouse in it at all: the name is left over from the
GUI build, where it also polled for motion events. Here it is `input_available()`
and `RealWaitForChar()`. It is folded into `WaitForChar()`, its only caller,
rather than left telling a lie.

### The enumerators stay

`KE_LEFTMOUSE`, `KS_MOUSE` and the rest are constants that cost nothing, and
**deleting an enumerator renumbers every one after it** — several enums in this
file index a parallel table. That is a different kind of change and does not
belong in a phase about capability.

### Four circles, and a tool bug

This phase found more than it removed, and all of it is the same shape: **an
option row is a root for reachability**, so a row keeps its own readers alive
and `--strict` then refuses to drop the row because those readers exist.

1. `did_set_string_option()` asks `if (varp == &p_mouse)`, and
   `check_mouse_termcode()` survives because `did_set_ttymouse()` names it.
2. `'mouse'`, `'mousemodel'` and `'ttymouse'` each name a `did_set_` and an
   `expand_set_` handler in the row itself, and `did_set_mousemodel()` reads
   `p_mousem`. The rows are pointed at NULL first; they go a moment later.
3. `:behave mswin` sets `'mousemodel'` **by name**, and the terminal's
   mouse-protocol reply sets `'ttymouse'` by name — the lookup that returns −1
   for a row that is not there and is not checked. `:behave` is about selection
   and keeps working; it just stops setting an option that has gone.
4. `didset_string_options()` dereferences every string option's global once at
   startup, which is the trap Phase 18 records. A row can be inert to every
   other reader and still be read there.

**And one real tool bug, which cost the most and was worth the most.** The first
run of this phase deleted **654 functions** and left a file that would not
compile. The cause:

```c
static struct mousetable
{
    int     pseudo_code;
    ...
} mouse_table[] =
{
    ...
};
```

gcc reports the unused variable at `} mouse_table[] =`, and `deadsweep.py` ran
forward from there — taking the initialiser and leaving the struct body open, so
the next declaration landed inside it and gcc said *"expected
specifier-qualifier-list before `static`"* a hundred lines later. Everything
after that was garbage compiling on garbage.

It is the same class of mistake as keying on a warning's sentence instead of its
option: **the extent of a thing is not "the line it was reported on"**.
`declaration_extent()` now walks *backwards* too when the declarator starts with
`}`, over the type body and its head. There are five constructs of that shape in
the file — `modmasktable`, `key_name_entry`, `mousetable`, `signalinfo` and
`termcode` — and this is the first phase that ever made one unused.

`dropoptions.py` was bounded at the same time: its guards looked 400 characters
ahead from the row's start, and `'mousefocus'` and `'mousehide'` are
`(char_u *)NULL` — GUI options with no global at all — so the search ran past
the end of the row and found the *next* option's variable. Every guard is now
bounded by the row's own braces.

### The delta

**None the harness records.** No Ex command is a mouse command, no behaviour
case clicks, and the pty harness types keys. The phase adds a check of its own:
`:set mouse=a` must be refused.

It took three tries to write that check, and each failure is one CLAUDE.md
already warns about. Reading the error message finds nothing, because silent Ex
mode prints nothing. Testing whether a later `:w` wrote the file finds nothing
either — **a failing `-c` does not abandon the ones after it**, so
`set nosuchopt` followed by `w` still writes. The exit status is the answer: 0
for an option that exists, 1 for one that does not. It is paired with
`:set ignorecase` as a control, so the check fails if the binary starts exiting
1 whatever it is asked.

## Phase 25 — a write is a write, and nobody owns it

### a write is a write

Writing a file in vim is not one operation. Before the new contents go anywhere
the old file may be renamed or copied aside, its permissions, owner, group, ACL
and timestamps carried over, the write attempted, and the whole thing rolled
back if it fails — and afterwards the copy is kept, or deleted, or renamed again
for `'patchmode'`. That is **437 lines of `buf_write()`**, and what `'backup'`,
`'writebackup'`, `'backupcopy'`, `'backupdir'`, `'backupext'`, `'backupskip'`
and `'patchmode'` are between them.

An embedded editor writes the file it was asked to write.

**`dobackup` is the hinge.** It is `(p_wb || p_bk || *p_pm != NUL)`, so with the
options gone it is FALSE, `backup` stays NULL and `backup_copy` stays FALSE —
and the tests spread through the rest of the function each collapse to the
branch they were already taking under `:set nobackup nowritebackup`, a
configuration vim has always supported. One of them is an `if`/`else if` whose
*else* is the live arm, so the pair collapses to that rather than going;
`buf_setino()` still has to happen.

Three things fall out that are worth naming separately:

  * **`vim_rename()` has five callers and all five are in here** — make the
    backup, put it back when the write fails, put it back when it is abandoned,
    and move it aside for `'patchmode'`. So `vim_copyfile()` goes with it, and
    that is `readlink`, `symlink` and `rename`.
  * `set_file_time()` carried the old file's timestamps onto the backup. One
    caller, and that is `utime`.
  * `mch_get_acl()`, `mch_set_acl()` and `mch_free_acl()` are **already stubs** —
    this build has no ACL support, so one returns NULL and the others do nothing
    with it. They went unnoticed for thirty phases because a stub compiles. The
    `vim_acl_T` that threaded through `buf_write()` to reach them goes too, and
    its three forward declarations go *here* rather than in the sweep: the sweep
    has to compile the file first, and a prototype naming a type this removes is
    an error, not a warning.

`fchown` and `umask` were not on the list and went anyway — every call to both
was inside the backup block.

#### The same circle, twice more

`'backupcopy'` names `did_set_backupcopy` and `expand_set_backupcopy` in its own
row, and `'backupext'` and `'patchmode'` share
`did_set_backupext_or_patchmode`; a row is a root, so the handlers survive the
sweep, read `p_bkc` and `p_bex`, and `--strict` then refuses to drop the row
that is the only thing keeping them alive. Phase 24 met this three times. The
rows are pointed at NULL first.

`didset_string_options()` reads `p_bkc` at startup — the trap Phase 18 records,
met again — and `set_init_default_backupskip()` looks its row up **by name**,
the lookup that returns −1 and is not checked.

#### Where the symbol count moves

**98 → 92**: `fchown`, `readlink`, `rename`, `symlink`, `umask`, `utime`.

#### The delta

**None the harness records.** `:w` writes; it just stops leaving a `~` file
beside what it wrote, which no harness asked for. The phase checks that
directly — overwrite a file and the directory must hold exactly what it held
before, with the new contents in it.

### nobody owns a file

An embedded editor runs where there are no users to tell apart, so asking who
you are is asking a question with no answer. Four places were still asking.

  * `:w!` on a read-only file makes it writable first, but only **if you own
    it**: `st_old.st_uid == getuid()`. The ownership test goes and the `chmod`
    stays. Nothing widens in practice — where the test used to say no, the
    `chmod` now says no instead, and the same error comes back by a different
    route.
  * When a write fails and `!` makes it retry, the mode carried onto the new
    file is masked to `0777`, dropping setuid, setgid and sticky — but only if
    you are not the owner. The test goes and **the masking stays**, which is the
    safe direction: a file this editor writes never carries a setuid bit.
  * `'modeline'` is forced off when `getuid() == ROOT_UID`, a protection against
    a modeline running as root. There is no root here and no `+eval` for a
    modeline to reach.
  * `get_user_name()` was stubbed to `return FAIL;` in Phase 20, when the
    password database went, and its two callers were left writing the answer
    into the swap file's block zero. The second one's `else` — the arm that
    spliced a user name into the recorded file name — has therefore been dead
    since Phase 20 and goes now, along with the `b0_uname` field itself. **A
    struct field is not a variable**: no warning names one that nothing reads,
    and the sweep cannot see it, so it has to be named here.

#### Permissions are not ownership

`chmod` and `fchmod` stay, through `mch_setperm()` and `mch_fsetperm()`. A file
still has a mode, `:w!` still has to clear the read-only bit to write, and the
mode of the file that was there is still put back on the file that replaces it.
Removing those would take `:w!` on a read-only file with them, which is a
capability and not a concept — so the phase asserts both halves: `getuid` and
`getgid` gone from `nm -u`, `mch_setperm`/`mch_fsetperm`/`mch_getperm` still
called, and `:w!` over a `chmod 444` file still writes it. No harness writes to
a read-only file, which is why that check lives here.

#### Where the symbol count moves

**92 → 90**: `getuid`, `getgid`.

#### The delta

**None.**

## Phase 26 — five signals, not twenty-one

`signal_info[]` had twenty-one entries and five handlers. Reviewed one at a
time, four earn their keep.

| kept | why |
| --- | --- |
| `SIGWINCH` | `sig_winch()` sets `do_resize`, read in nine places. Without it the editor never learns the terminal changed size. |
| `SIGINT` | `catch_sigint()` sets `got_int` — **read in 222 places**. That number is the argument: `got_int` is how every long operation is interruptible. Without the handler, CTRL-C reverts to its default action, which kills the process and loses the buffer, turning "stop that" into "lose your work". |
| `SIGTSTP` | CTRL-Z and `:suspend`, through `sig_tstp()` and `got_tstp`. The only caller of `raise()`. |
| `SIGHUP`, `SIGTERM` | reaching `deathtrap()`, so that a killed editor **puts the terminal back**. |

Sixteen entries and three handlers go: `SIGPWR`, whose handler called
`ml_sync_all()` — **an empty function** since Phase 11; `SIGUSR1`, whose flag
**nothing reads** (assigned and never examined, so `-Wunused-variable` never
fires and the sweep would never have found it); and `SIGQUIT`, `SIGILL`,
`SIGTRAP`, `SIGABRT`, `SIGFPE`, `SIGBUS`, `SIGSEGV`, `SIGSYS`, `SIGALRM`,
`SIGVTALRM`, `SIGPROF`, `SIGXCPU`, `SIGXFSZ`, `SIGUSR2`, `SIGPIPE`. With them go
`sigaltstack` and its stack — which existed so a SIGSEGV from stack overflow
could still run a handler, and SEGV no longer reaches one — and
`may_core_dump()`, which re-raises to produce a core there is nobody to read.

Eight signal names are left in the file: the five kept, plus `SIGCONT`,
`SIGALRM` and `SIGPIPE`, which `mch_suspend()` sets around the stop. The phase
asserts exactly that list.

**The cost, decided deliberately: a crash no longer restores the terminal.**
`SIGSEGV` and `SIGBUS` take their default action. The alternative is keeping a
handler for conditions this editor should not have, to tidy up after a bug that
should not exist.

### The reason to keep `SIGHUP` and `SIGTERM` was not true until this phase

This is the part worth recording, because the phase was written on a claim that
turned out to be false and the check is what caught it.

`deathtrap()` reaches `preserve_exit()` → `prepare_to_exit()`, which calls
`settmode(TMODE_COOK)` to put the terminal back. And:

```c
settmode(tmode_T tmode)
{
    if (!full_screen)
        return;
```

— while `deathtrap()` sets `full_screen = FALSE` several lines before it gets
there. So the editor printed `Vim: Caught deadly signal TERM`, emitted
`stoptermcap()`'s escapes, exited, and **left the terminal with `ICANON` and
`ECHO` off**. The shell that got it back was unusable; the user had to type
`reset` blind. Measured on the slave side of a pty, before and after the cut:
identical, and wrong both times. Upstream has the same hole.

The fix is additive, so the ordinary exit path is untouched: `full_screen` is
lent for the length of the call. The guard exists to avoid drawing on a screen
that is not there, and putting the terminal back is not drawing.

`mch_settmode()` would have been the more direct call and is not available —
it is defined 89,000 lines further down and `SLIM-GOAL.md` Phase 10 removed the
forward declaration nothing needed.

### The check

`tools/termrestore.py` opens a pty, starts the editor on it, **verifies it
really entered raw mode** — otherwise the test would pass for the wrong reason,
on an editor that never changed anything — sends `SIGTERM`, and requires
`ICANON` and `ECHO` back on the slave side. No harness here kills an editor
halfway: the behaviour cases and the Ex sweep run it to completion and the pty
harness quits cleanly. This is the one thing the kept signals are for, so it is
checked in the phase.

### Where the symbol count moves

**90 → 88**: `sigaltstack`, `sysconf`. `raise` stays — `sig_tstp()` needs it —
and so does `kill`, whose three sites are `mch_suspend()`, the signal-blocking
helper, and `may_core_dump()`; only the last goes.

### The delta

**None the harness records.** The Ex sweep records `:suspend` and `:stop` as
*skipped* — they hand over the terminal — and `SIGTSTP` stays regardless.

## Phase 27 — `[[=a=]]` stops meaning "a with any accent"

A POSIX bracket expression has three bracketed forms inside it, and they are
three different features that happen to share a syntax:

| | | |
| --- | --- | --- |
| `[[:alpha:]]` | a character **class** | stays |
| `[[.x.]]` | a collating **element** | stays |
| `[[=a=]]` | an equivalence **class** | goes |

The third means "this character and every accented form of it", and expanding it
takes **`reg_equi_class()`, 1,397 lines** — a switch over every base letter
listing its variants across Latin-1, Latin Extended-A and Latin Extended-B. It
was the largest single function left in the file and the least used: reached
only when a pattern contains `[=`, and nothing in the editor writes one.

Two call sites, and the sweep did the rest: the bracket parser in `regatom()`,
where the `get_equi_class()` arm goes so `[=` falls through to being taken one
character at a time; and `skip_regexp()`'s scan, which asked the same question
only to know how far to skip.

`\w`, `\a` and `[[:alpha:]]` are a different mechanism and are untouched. The
phase checks both halves, because only the pair is a check: `[[=a=]]` must stop
matching an accented `a`, and `[[:alpha:]]` must still classify.

## Phase 28 — C indenting

`get_c_indent()` was **1,534 lines** and the largest function left: a model of C
syntax built to answer one question, how far to indent this line. It knows about
labels, scope declarations, `case` bodies, continuation lines, comment blocks
and function arguments, and about `'cinoptions'`, a miniature language for
adjusting all of it. With `in_cinkeys()` and the `cin_*` helpers, **3,007 lines**.

`'autoindent'` stays — it is on by default here — and copies the previous line's
indent. That is what an embedded editor needs; the rest is a C compiler's front
end used for whitespace. `'lisp'` and `'indentexpr'` are different indenters and
are not touched.

Five options go, all `PV_BUF`: `'cindent'`, `'cinkeys'`, `'cinoptions'`,
`'cinscopedecls'`, `'cinwords'`.

### It is spelled in more places than it is named

The first cut found five call sites. The sweep found seven more, and each is a
different way of not being a call to `get_c_indent`:

  * `op_reindent(oap, get_c_indent)` — the `=` operator passes the indenter as a
    **function pointer**, so a grep for `get_c_indent(` does not see it. `=` now
    passes `get_indent`, which sets each line's indent to the indent it already
    has: a no-op, the honest answer for a buffer whose language the editor
    cannot read.
  * `preprocs_left()` and `may_do_si()` — `'smartindent'` **defers to**
    `'cindent'` when both are set, so both read `b_p_cin` without touching the
    indenter.
  * `parse_cino()` has **four** callers and none of them indents anything:
    opening a buffer, resizing a window, setting `'shiftwidth'` (some
    `'cinoptions'` are expressed in shiftwidths), and `check_buf_options()`.
  * `cin_is_cinword()`, reached from `'smartindent'`, because `'cinwords'` told
    it which keywords begin a block.
  * insert completion re-indents on accept, through `want_cindent`.

`cindent_on()` is left, returning FALSE. It has seven callers and five only ask
in order to do something else instead; an editor that answers "no, this buffer
is not C-indented" is telling the truth.

### A tool bug this found

`droplocal.py` counted a field's remaining mentions with `text.count(name)` and
no word boundary, so `b_p_cin` appeared to have 23 readers when it had none —
they were `b_p_cink`, `b_p_cino`, `b_p_cinsd` and `b_p_cinw`. Ordering the
arguments around it would have hidden the bug rather than fixed it.

### The awkward part

Insert mode tests for a re-indent in two places hundreds of lines apart, and the
first **jumps into the second**:

```c
if (cindent_on() && ctrl_x_mode_none())        ... goto force_cindent;
...
if (can_cindent && cindent_on() && ...)  { force_cindent: ... }
```

So the two have to go together or not at all — removing the second alone orphans
the label, and removing the first alone leaves a label nothing reaches.

Measured: 142,170 → 138,178 lines, 3,992 removed against 3,007 predicted; the
option plumbing and the `b_ind_*` fields were the difference.

## Phase 29 — `:command`, user-defined commands

`:command` lets a user give a name to an Ex command line and have it dispatched
like a built-in. The machinery is **1,451 lines**: a parser for the `-nargs`,
`-range`, `-complete` and `-bang` attributes; a per-buffer and a global growable
array of definitions; `uc_check_code()`, 286 lines, expanding `<args>`,
`<q-args>`, `<line1>`, `<count>`, `<bang>`, `<reg>` and `<mods>`; and a listing
mode.

**Without `+eval` a user command can only invoke built-in commands**, which makes
it a way of writing an alias — and this editor reads no vimrc, so the only way to
define one is to type `:command` by hand in the session where it is used.

What goes beyond the three commands: `do_ucmd()`, which `do_one_cmd()` reaches
when `ea.cmdidx` is negative — the marker for "this name is not in `cmdnames[]`,
try the user table" — so an unknown name is now simply not a command;
`find_ucmd()`'s two callers; **six rows of the completion table**, which kept six
`get_user_cmd_*` functions alive and which no grep for `do_ucmd` would find,
because a table row is a reference the same as a call; the walk past the end of
`cmdnames[]` in `expand_user_command_name()`; and the `b_ucmds` field.

### The delta is two names, not the three retired

`:command` with no arguments lists what is defined, and `:comclear` clears it:
both succeed today, so both move in the Ex sweep. **`:delcommand` does not** — it
is `EX_NEEDARG`, so the sweep's bare call already failed. Declaring three and
being told two is the check working, and it is Rule 3's other half: retiring a
command only shows in the sweep if it used to succeed.

## Phase 30 — `K` and the tag jumps, keeping `*` and `#`

`nv_ident()` is not one command, it is five, and they have nothing in common but
the first step — read the identifier under the cursor:

| | | |
| --- | --- | --- |
| `*` `#` `g*` `g#` | search for that word | **stay** |
| `K` | run `'keywordprg'` on it | goes |
| `]` `CTRL-]` `g]` | jump to its tag | goes |

`*` and `#` are among the most used keys in vim and are pure search, so this
phase **rewrites** the function rather than deleting it. `K` runs `'keywordprg'`
through `:!` and Phase 8 took the process behind it; the tag jumps build `ta `, `tj `,
`ts ` or `he! ` and hand them to `do_cmdline_cmd()`, and Phase 10 made every one
of those `ex_ni`. Both arms have been building commands that fail.

### Rule 3 applies to normal-mode commands too

The first version **deleted** the `K` and `CTRL-]` rows from `nv_cmds[]`. It
built, it swept clean, it passed the linkage and symbol checks — and **39 of the
67 behaviour cases moved**: CTRL-A, joins, macros, marks, multibyte motions,
nothing to do with `K` or tags.

`nv_cmd_idx[]` is a `static const` array of **indices into `nv_cmds[]`**,
precomputed and sorted by command character, with `nv_max_linear` marking how
far a direct lookup works. Deleting two rows shifts every later index while the
precomputed table still points at the old positions, so every normal command
after them dispatches to the wrong function. It is the parallel-table trap the
enumerators have, one table over.

So **Rule 3 — a command is never deleted from the table, it is pointed at
`ex_ni`** — extends to `nv_cmds[]`, where the equivalent is `nv_error()`, the
handler already used for keys that do nothing.

### And the check had to be a pty

The first `*` check ran under `-e -s` and compared the file. The two binaries
disagreed — before the phase `:normal *dd` did nothing at all, after it the `*`
was ignored and the `dd` deleted line 1. **Neither is what `*` does.**
`normal_search()` wants a screen, so silent Ex mode measures something that is
not the feature. In a real pty both binaries give the same correct answer, and
`tools/starcheck.py` now asks it there: from `foo` on line 1, `*` must land on
the `foo` on line 5 and **skip `foobar`**, which `dd` then proves.

Measured: 136,190 → 135,941 lines; `nv_ident()` from 227 lines to the search
half. **The delta is none** — these are normal-mode keys, so no Ex command
moves.

## Phase 31 — file-name modifiers

`eval_vars()` expands `%` and `#` into the current and alternate file names, and
`<cword>`, `<afile>` and the rest. **That stays** — `:w %` and `:e #` are how a
file name is written without typing it.

What goes is the **suffix language** that may follow: `modify_fname()`, 426
lines implementing `:p` (full path), `:h` (head), `:t` (tail), `:r` (root),
`:e` (extension), `:s/from/to/`, `:gs`, `:~` and `:.`, applied left to right so
that `%:p:h:t` means something. It is a small programming language over path
strings, and **most of it asks questions this editor can no longer answer**:

| modifier | what it needed | which phase took it |
| --- | --- | --- |
| `:p` | where the working directory is | 24 — there is one answer now |
| `:~` | the notion of `$HOME` | 22 — nothing outside the process |
| `:s//` | a regexp over a file name | the only place a pattern is applied to something that is not buffer text |

**One caller**, which is why the cut is small: `eval_vars()` reaches it once, in
the arm that runs when the next character is not `<`. That arm goes, and with
it `tilde_file` and `skip_mod`, which existed only to be passed to it. The `<`
arm — which strips one extension and is not part of the modifier language —
stays. After this a modifier is left in the command line as the literal
characters it is written with, which is what an editor that does not know the
syntax does.

**The delta is none, so the phase checks both halves itself**, and only the
pair is a check: `:w %` must still write the file being edited, and `%:t` must
stop being a tail. One without the other passes on a `eval_vars()` that returns
NULL for everything.

Measured: 135,941 → 135,315 lines.

## Phase 32 — insert completion, the popup menu, and the keys that reached them

CTRL-N, CTRL-P and the whole CTRL-X family — `CTRL-X CTRL-F` for file names,
`CTRL-X CTRL-K` for a dictionary, `CTRL-X CTRL-L` for whole lines — plus the
popup menu that displays the matches. This is the largest single subsystem left
after the regexp engine, and it is the one whose sources are all gone already:
the tag stack went in Phase 10 and the `CTRL-]` key in Phase 30, the shell in
Phases 6 and 8, `'dictionary'` and `'thesaurus'` name files this editor has no
business reading, and `'completefunc'` needs the eval layer.

**Two cuts, with a sweep between them.** The first answers the questions
completion is entered through, so it produces nothing; the second removes the
code that kept asking. This was two phases, and the second existed only because
the first had stopped short.

### The predicates

**Nine predicates become constants**, and the sweep follows them:

```
ins_complete              FAIL        pum_visible                    FALSE
ins_compl_prep            FALSE       pum_redraw_in_same_position    FALSE
ins_compl_active          FALSE       pum_may_redraw   pum_undisplay   pum_display
ins_compl_has_autocomplete FALSE
```

Twelve option rows go with them — `autocomplete complete completefunc
completeopt dictionary infercase pumborder pummaxwidth pumopt pumheight pumwidth
thesaurus` — and six buffer-local fields, `b_p_cpt b_p_cot b_p_dict b_p_tsr
b_p_inf b_p_ac`.

### `didset_string_options()`, for the fourth time

This is the fourth phase to be caught by it, and this time it was a **segfault
before the first keystroke**. The function dereferences every string option's
global at startup, so dropping `'completeopt'`'s row while leaving

```c
opt_strings_flags(p_cot, p_cot_values, &cot_flags, TRUE);
```

hands a NULL to something that reads it. The editor did not mis-complete; it
did not start.

`orphanopts.py` existed precisely to catch this and did not, because it looked
for an explicit `*p_x` dereference and this is a bare argument. **It now counts
any mention at all.** A pointer nothing mentions is harmless — the sweep takes
it — and one that is mentioned while having no row to initialise it is a NULL
going somewhere, which is enough to fail on without judging the shape of the
somewhere. Re-run over every earlier boundary: no new complaints, so the
stricter rule costs nothing and closes the trap that had cost four phases.

### Checking that a key does nothing

Bare CTRL-N in insert mode is **already inert** in a build with nothing to
complete from, so a before/after comparison of it proves nothing either way.
`tools/complcheck.py` uses `CTRL-X CTRL-N` instead, which is unambiguous, and
checks the half that must survive in the same run: **insert mode still
inserts**. A completion check that only proves completion is gone also passes on
a binary that cannot type.

### The callers

**Seventy functions named `ins_compl_*`, `pum_*` or `compl_*` survive the
stubs.** They are *reachable*, so no sweep can touch them, and never *entered*,
because `ins_complete()` returns FAIL before any of them runs. `edit()` does not
reach completion through one door: it calls `ins_compl_addleader()`,
`ins_compl_bs()`, `ins_compl_accept_char()` and twenty-five more directly, and
`update_screen()`, `win_line()`, `showruler()` and `screen_puts_len()` each ask
`pum_visible()` on their own account. That is the shape worth naming: **a stub
answers a question; it does not remove the caller that asks it.** So the second
cut removes the callers, and the second sweep takes the callees.

What goes, all of it inside `edit()`:

| | |
| --- | --- |
| the CTRL-X submode | `ins_ctrl_x()` is empty, so `ctrl_x_mode` never leaves `CTRL_X_NORMAL` and every `ctrl_x_mode_*()` test is decided |
| the per-key completion arm | forty lines feeding each keystroke to the match list |
| `'autocomplete'` | six arming sites, three of them one-line blobs macro expansion left behind |
| the arrow keys | four `if (pum_visible()) goto docomplete;` arms on Up, Down, PageUp and PageDown |
| `docomplete:` | the label itself |

**What stays is the answer the stubs gave**: CTRL-N and CTRL-P are still
insert-mode keys, and they now do nothing, which is what an unbound key does.

**The sweep between the two cuts is kept, and it is not a formality.**
`tools/nocomplkeys.py` counts and matches text in `edit()` as the first sweep
leaves it, and a count taken over code about to be swept is a different count.

### A cut that is not unique is a guess

The first version dropped the autocomplete disarm by matching its condition,
`if (c != KE_CURSORHOLD && c != KE_COMPLETE_DELAY)`. That condition occurs
**three times inside `edit()`**, and the one the search took was

```c
        {
            lastc = c;
        }
```

— the last-character save, which has nothing to do with completion. It
compiled, it swept clean, the island still shrank by 2,400 lines, and **nothing
downstream objected.** `drop_unique()` now refuses any condition that is not
unique in the file; anything genuinely ambiguous is spelled out in full or
anchored to one function with `drop_if_in()`. The phase also asserts the `lastc`
line is still there, because that is the failure that got through.

### Two halves, and only the pair is a check

Completion must be absent — `tools/complcheck.py` — and the arrow keys, whose
`pum_visible()` arms this phase cuts, must still move the cursor. Cutting a
guard and the key's real body together is exactly what no completion check would
notice, so `tools/arrowcheck.py` (since retired) asked in a pty: from `one/two/three`, `A` then
Down then `X` must give `twoX`. **It was proved able to fail first** — with
`ins_down()` removed it reports `oneX`.

### The delta

Measured: **135,315 → 127,131 lines** and symbols 88 → 88, the subsystem being
pure computation over things already removed. The thirteen functions that remain
of the island are constant-answer stubs the redraw layer asks on its own account.
**Merged, the phase reproduces the boundary the two phases recorded byte for
byte**, in 173 seconds against the 203 they took in sequence. **The delta is
none** — no behaviour case types CTRL-N, these are insert-mode keys, and no Ex
command moves.

## Phase 33 — commands whose machinery has already gone

Every one of these still had a handler, and every one refused or did nothing
when run with a sensible argument. That was measured one at a time, in Ex mode,
reading the message each left behind:

| command | what it said |
| --- | --- |
| `:shell` | `E319`, no processes since Phase 8 |
| `:gui`, `:gvim` | `E25`, no GUI in this build |
| `:cdo` `:cfdo` `:ldo` `:lfdo` | `E319`, no quickfix lists |
| `:vim9cmd` | `E319`, no eval layer |
| `:endclass` `:endinterface` `:endenum` `:public` `:static` `:this` | Vim9 class keywords, invalid without the eval layer |
| `:digraphs` | `E196`, no digraphs in this build |
| `:redrawtabpanel` | `E1547`, no tab panel |
| `:colorscheme` | `E185`, no colour scheme to find — nothing is installed |

**A command that only says no is a row pointing at a handler that exists to say
no.** So the rows go to `ex_ni` — rule 3, the table keeps its shape — and the
sweep takes `ex_shell`, `ex_nogui`, `ex_digraphs`, `ex_redrawtabpanel`,
`ex_colorscheme` and `load_colors()`, which nothing else called. `ex_listdo`
stays, because `:argdo`, `:bufdo`, `:windo` and `:tabdo` use it, and its two tests
for the quickfix commands are folded rather than left asking a question that can
no longer be true. `ex_wrongmodifier` stays for the modifiers that still work.

**`:!` is the one refusal kept, and on purpose.** `:!cmd`, `:r !cmd` and `:w !cmd`
are how a user reaches for a process, and the answer Phase 8 gave them is the
sentence it prints. `:filetype` and `:vim9script` are not here either: they run
without an error, and nothing measured shows them refusing.

Left alone, as every earlier phase left them: the arms of
`set_context_by_cmdname()` that set up command-line completion for these names.
A retired row still parses, so its completion context still fires, and it
completes nothing.

### A tool bug this found

`tools/retire.py` matched a row with exactly one space before the handler, and
the `:gui` and `:gvim` rows are spelled `- 1,  ex_nogui ,` — macro expansion's
spacing. It refused, loudly, which is what it is for; the rule is the row, not
the spacing, so it takes any whitespace now. The six earlier phases that retire
rows with it were re-verified and all reproduce their boundaries.

### The delta

**`:colorscheme`**, which run bare reported the current scheme and succeeded in
`slim-vim`, and now reports that it is not implemented. Every other row already
failed, or is one the sweep skips because it hands over the terminal. Measured:
127,131 → **127,037 lines**, libc symbols 88 → 88.

## Phase 34 — no abbreviations

An abbreviation is a word the editor rewrites as you type it. Nothing reads a
vimrc here, so the only way to have one was to type `:abbreviate` in the session
that wanted it — and the twelve rows that did that, `:abbreviate`, `:noreabbrev`,
`:unabbreviate`, `:abclear` and their `i` and `c` forms, go to `ex_ni`.

**Retiring the rows removes the answer, not the question.** Insert mode asked
`echeck_abbr()` on ESC, CTRL-O, CTRL-L, Tab, Enter and every non-word character,
and the command line asked `ccheck_abbr()` twice, and each would go on asking
for ever and being told no. `tools/noabbr.py` removes the questions, each a fold
whose answer is now known:

- `if (echeck_abbr(...)) { ... }` and `if (ccheck_abbr(...)) { ... }` guard what
  happens when an abbreviation fired, so the blocks go;
- `!echeck_abbr(x) && c != Ctrl_RSB` is `c != Ctrl_RSB`;
- `(ccheck_abbr(x) || c == Ctrl_RSB)` is `c == Ctrl_RSB` — CTRL-] on the command
  line still triggers "an abbreviation", which is to say nothing, and still does
  not insert itself.

The sweep then takes `check_abbr()` — 195 lines — its two wrappers,
`ex_abbreviate` and `ex_abclear`. **What stays** is the mapping code's `abbr`
parameters and list, which mappings share; nothing can put an entry on that list
any more, and nothing here pretends that makes the shared code smaller.

The tool's own check failed twice before the phase ran, both times on itself:
it counted `check_abbr()` calls inside the two wrappers the sweep removes, and
then prototypes it matched with one space where the file has two. **A check that
cannot tell a caller from a definition is measuring the wrong thing**, and it
now asks only about calls outside the definitions going away.

### The delta

**The nine rows that succeeded run bare** — `:abbreviate`, `:noreabbrev` and
`:abclear` with their `i` and `c` forms, which listed or cleared nothing and
exited 0 — measured before the cut. The three `:unabbreviate` rows already failed
with no argument. Measured: 127,037 → **126,756 lines**, libc symbols 88 → 88.

## Phase 35 — no scripts, no session, no autocommands

Three things that are one question: can the editor be told to do something
later, or somewhere else, by a file? A script is commands read from a file, a
session is a script the editor wrote about itself, and an autocommand is a
command registered now to run when an event happens. None of them has anywhere
to come from: nothing is installed, no vimrc is searched for, and since Phase 18
nothing at all is read at startup.

| | what goes |
| --- | --- |
| scripts | `:source` `:scriptencoding` `:scriptversion` `:vim9script` `:legacy`, the `vim9cmd` modifier, `'loadplugins'` |
| the session | `:redir` `:sleep` `:smile` `:sandbox`, `-S`, `-s file`, `-w`/`-W file`, `'sessionoptions'` `'viewoptions'` `'viewdir'` |
| autocommands | `:autocmd` `:augroup` `:doautocmd` `:doautoall` `:noautocmd` `:filetype` `:setfiletype`, the engine, `'eventignore'` `'eventignorewin'` |

The sixteen rows go to `ex_ni`. `tools/nosession.py` removes what a row cannot:

- **The modifiers are parsed by name.** `parse_command_modifiers()` matches
  `legacy`, `noautocmd`, `sandbox` and `vim9cmd` before the table is consulted,
  so retiring a row changes nothing about `:noautocmd w`. The four blocks go,
  with the save and restore of `'eventignore'` that `:noautocmd` did.
- **The engine is answered at its doors.** `apply_autocmds_group()`,
  `has_autocmd()` and the per-event `has_*()` say no, and the `trigger_*()`
  helpers and `may_trigger_win_scrolled_resized()` do nothing — which is what
  each already did with no autocommand defined. The sweep takes the engine
  behind the doors. The calls that fire events stay: each is a call to a
  constant now, and removing them is a phase of its own.
- **Filetype detection after a rename** ran only when the `filetypedetect` group
  existed, which only `:augroup` or `:autocmd` could make; both tests fold, and
  `do_doautocmd()` goes with its last callers.
- **`in_vim9script()` is FALSE**: it was true only after `:vim9script` or under
  `vim9cmd`.
- **Suspending stays.** CTRL-Z, `:stop` and `:suspend` still hand the terminal
  back to the shell. The first version of this phase took them as part of the
  session and they were put back on request: suspending is job control, and
  nothing about it is read from or written to a file.
- **The command line loses its scripts.** `-S`, `-s file` outside Ex mode, and
  `-w file`/`-W file` are unknown options. `-s` keeps silent Ex mode after `-e`,
  `-wN` still sets `'window'`, and `-u file` stays.

### Two options that have to go before the sweep

`dropoptions.py --strict` refused `'eventignore'` after the first sweep: the
readers `event_ignored()` and `check_ei()` were still live. Two things held them.
`did_set_eventignore()` is the callback of **both** `'eventignore'` and
`'eventignorewin'`, and calls `check_ei()` — so while either row stands, the
reader is reachable from the option table and no sweep can take it, and
`--strict` cannot be satisfied in either order. And the WinScrolled/WinResized
scan read `'eventignorewin'` from every window before learning that neither
event had an autocommand.

So both rows go **before** the sweep and without `--strict`, and the
post-condition is the check: after the sweep nothing names `p_ei`, `wo_eiw`,
`check_ei`, `event_ignored` or `check_window_scroll_resize`. The enumerator
`WV_EIW` stays, named only by its own declaration: the `WV_` and `BV_` index
enums are anonymous, `enum { WV_LIST = 0, ... }`, and the definition finder
`deadenums.py` walks sees only tagged and typedef'd enums, so it never examines
them. That is a gap in a shared tool, measured here and not yet closed — closing
it changes every phase's implementation digest.
`'eventignorewin'` is window-local and `tools/droplocal.py` knows only buffer
fields, so `nosession.py` removes its field and the four places that maintain
it — `get_varp()`, `copy_winopt()`, `check_winopt()`, `clear_winopt()` — itself.

### The delta

**The ten rows that succeeded run bare** — `:sleep`, `:smile`, `:vim9script`,
`:autocmd`, `:augroup`, `:doautocmd`, `:doautoall`, `:noautocmd`, `:sandbox` and
`:filetype` — measured before the cut. `:source`, `:redir`, `:scriptencoding`,
`:scriptversion`, `:legacy` and `:setfiletype` already failed with no argument.
No harness sources, redirects,
suspends or defines an autocommand, and each passes `-s` only after `-e`.
Measured: 126,756 → **123,384 lines**, libc symbols 88 → 88.

## Phase 36 — one tab page, always

A tab page is a set of windows the editor can switch between whole. The
tab-page list is also the container every window lives in — `curtab` and
`first_tabpage` are read in hundreds of places — so **it stays, with exactly one
entry**, and every way to make or reach a second one goes.

The fifteen rows go to `ex_ni`: `:tab`, `:tabnew`, `:tabedit`, `:tabclose`,
`:tabonly`, `:tabnext`, `:tabNext`, `:tabprevious`, `:tabfirst`, `:tabrewind`,
`:tablast`, `:tabmove`, `:tabs`, `:tabdo` and `:redrawtabline`.
`tools/notabs.py` removes what a row cannot:

- **The `:tab` modifier** is matched by name in `parse_command_modifiers()`, and
  it was the only thing that set `cmdmod.cmod_tab`. With it gone every test of
  `cmod_tab` is decided: the tab branches of `:all`, `:ball`, `:drop`,
  `:argedit`, `:wincmd` and the command-line window fold.
- **The handlers the tab commands shared** keep their other users. `:tabnew` and
  `:tabedit` went through `ex_splitview()` with `:split` and `:new`, and `:tabdo`
  through `ex_listdo()` with `:windo`, so only their terms and branches go.
- **Each key keeps the answer it already gave with one tab page.**
  `goto_tabpage(n)` with a single tab page beeps when `n > 1` and otherwise does
  nothing, and there is never a last-used tab page. So `gt`, CTRL-PageDown and
  CTRL-W gt beep for a count above 1; `gT`, CTRL-PageUp and CTRL-W gT do
  nothing; `g<Tab>`, CTRL-Tab and CTRL-W g`<Tab>` beep; insert mode's
  CTRL-PageUp and CTRL-PageDown stay no-ops. The keys are not given a new
  meaning — they lose a function nothing could reach.
- **CTRL-W T and CTRL-W gf/gF open a tab page and nothing else**, so they beep
  now, as an unknown window command does. CTRL-W T with one window used to say
  "Already only one window"; that message goes with the command.
- **The tab line** is 0 lines and `draw_tabline()` draws nothing — what both
  answered for one tab page under the default `'showtabline'`. `win_split()`
  no longer asks `may_open_tabpage()` whether a `:tab`-modified split became a
  tab page — a stub would have answered, and left the caller and the function
  alive, which is how the first run of this phase failed. The sweep then takes `'showtabline'`, `'tabline'`
  and `'tabpagemax'`'s readers, and `dropoptions.py --strict` their rows.
  `'tabclose'` is the Phase 35 trap again: its own callback, `did_set_tabclose()`,
  reads `p_tcl`, so while the row stands the reader is live and no order of sweep
  and `--strict` works. Its row goes before the sweep, and the post-condition —
  nothing names `p_tcl` or `tcl_flags` afterwards — is the check.

Left alone, as every earlier phase left them: the completion arms of
`set_context_by_cmdname()` for these names.

### The delta

**The thirteen rows that succeeded run bare** — `:tab`, `:tabedit`, `:tabfirst`,
`:tabmove`, `:tablast`, `:tabnext`, `:tabnew`, `:tabonly`, `:tabprevious`,
`:tabNext`, `:tabrewind`, `:tabs` and `:redrawtabline` — measured before the cut.
`:tabclose` and `:tabdo` already failed with no argument. No harness opens a tab
page. Measured: 123,384 → **122,290 lines**, libc symbols 88 → 88.

## Phase 37 — no command that does nothing

What was left in the table after Phase 36, read handler by handler, had ten
rows that either did nothing or did something this editor does not want:

| rows | what they did |
| --- | --- |
| `:browse`, `:confirm` | modifiers whose flags went with the file browser and the dialogs; each skipped its own name and ran the rest |
| `:tmap`, `:tnoremap`, `:tunmap`, `:tmapclear` | stored mappings for terminal-job mode, which nothing enters — no assignment puts `MODE_TERMINAL` in `State` |
| `:winpos` | "not implemented" bare; with two numbers, checked them and did nothing |
| `:behave` | set `'selection'`, `'selectmode'` and `'keymodel'` to another editor's habits — `mswin`'s naming the mouse, which Phase 24 removed |
| `:mode` | a screen-mode switch no terminal here has; bare, a redraw |
| `:open` | vi's open mode, which here was a cursor move followed by `:visual` |

All ten go to `ex_ni`. `tools/noinert.py` removes what a row cannot:

- **`:browse` and `:confirm` are matched by name in
  `parse_command_modifiers()`**, before the table, so their branches go; the
  name then reaches the table and is not implemented. `:browse set ic` no longer
  sets anything.
- **Terminal-job mappings.** `get_map_mode()` loses its `'t'` and
  `map_mode_to_chars()` the letter it printed. The `MODE_TERMINAL` enumerator
  and the masks that test it stay — constants, costing nothing.
- **Completion.** Unlike the phases before it, this one takes the
  `set_context_by_cmdname()` arms for its names, because `:behave`'s is what
  kept `get_behave_arg()` alive.

**`:highlight` stays.** It sets the colours of highlight groups, and `Search` is
the one `'hlsearch'` draws with; the defaults are applied through
`do_highlight()` whether or not the command exists, but changing them needs it.
Syntax highlighting is not in this build — the `:syntax` row already points
at `ex_ni`.

### The delta

**The seven rows that succeeded run bare** — `:browse`, `:confirm`, `:mode`,
`:open`, `:tmap`, `:tmapclear` and `:tnoremap`, read from the slim baseline.
`:behave`, `:tunmap` and `:winpos` already failed with no argument. Measured:
122,290 → **122,145 lines**, libc symbols 88 → 88.

## Phase 38 — the argument list is walked by `:next` and `:previous` alone

**The list stays.** `vim a b c` fills it, `:next` and `:previous` move through
it, `:next x y` replaces it, `:drop` sets it, and quitting with files not yet
edited is still refused. Every other command on it goes — twenty-five rows:
`:args`, `:argglobal`, `:arglocal`, `:argadd`, `:argdelete`, `:argdedupe`,
`:argedit`, `:argument`, `:sargument`, `:first`, `:sfirst`, `:rewind`,
`:srewind`, `:last`, `:slast`, `:snext`, `:wnext`, `:Next`, `:sNext`,
`:sprevious`, `:wNext`, `:wprevious`, `:all`, `:sall` and `:argdo`.

**`:Next` is a row of its own**, spelled apart from `:previous` though it shares
the handler, so `:N` goes with it; `:prev` still reaches `:previous`.

`tools/noarglist.py` removes what a row cannot:

- **The shared handlers keep their other users.** `:snext` went through
  `ex_next()`, and `:argdo` through `ex_listdo()` with `:bufdo` and `:windo`, so
  only their terms go; `do_argfile()` no longer spares `:argdo` the `'` mark.
  **`ex_rewind()` stays**, because `:drop` ends in it — the `:first` row going
  does not make its handler dead, and the phase checks that it survives.
- **Completion** for `:argdo` and `:argdelete`, and the argument-list expansion
  only `:argdelete` asked for, so the sweep takes `get_arglist_name()`.

The sweep takes the handlers, `do_arg_all()` and its helpers, `alist_new()` —
only `:arglocal` gave a window a list of its own — and `list_in_columns()`,
which only `:args` printed with.

### The delta

**The sixteen rows that succeeded run bare** — `:all`, `:args`, `:argadd`,
`:argdelete`, `:argdedupe`, `:argglobal`, `:arglocal`, `:argument`, `:first`,
`:last`, `:rewind`, `:sargument`, `:sall`, `:sfirst`, `:slast` and `:srewind`,
read from the slim baseline. The other nine already failed with no argument.
Measured: 122,145 → **121,368 lines**, libc symbols 88 → 88.

## Phase 39 — one window, always

The window list is the container the editor draws into, and `aucmd_prepbuf()`
still slots its hidden autocommand window into the frame tree beside the user's
with `win_split_ins()`. So **the list stays, with one user window in it**, and
every way to make, reach, resize, close or bind a second one goes.

Thirty-two rows go to `ex_ni`: `:split`, `:vsplit`, `:new`, `:vnew`, `:sview`,
`:close`, `:only`, `:resize`, `:wincmd`, `:windo`, `:syncbind`, `:hide`, `:sbuffer`,
`:sbNext`, `:sball`, `:sbfirst`, `:sblast`, `:sbmodified`, `:sbnext`,
`:sbprevious`, `:sbrewind`, `:ball`, `:unhide`, `:sunhide`, and the eight split
modifiers `:aboveleft`, `:leftabove`, `:belowright`, `:rightbelow`, `:topleft`,
`:botright`, `:vertical` and `:horizontal`. `tools/nowindows.py` removes what a row
cannot:

- **The modifiers** are matched by name in `parse_command_modifiers()` and were
  all that set `cmdmod.cmod_split`. **`:hide {cmd}` is a modifier too, and stays**
  — only bare `:hide`, which closed the window, is a row.
- **CTRL-W** points at `nv_error`, and `do_window()` goes with every window command
  behind it.
- **The command-line window is a split**, so it goes: `q:`, `q/` and `q?` are the
  recordings they would be without it, CTRL-F on the command line is an ordinary
  key, and every test of `cmdwin_type`, `cmdwin_win`, `cmdwin_buf` and
  `cmdwin_result` folds. `vgetorpeek()`'s `tc` remembered the previous key for one
  of those tests alone, and goes with it — the warning check caught it.
- **`-o` and `-O`** are unknown options, and startup opens no window per file:
  `create_windows()` loses its count and `edit_buffers()` its call.
  `tools/clicheck.py` still lists them as accepted, because it runs at Phase 3
  where they are; the phase checks they are refused, in its terms.
- **The paths that still split.** `:drop` split when the buffer could not be
  abandoned, and now does what `:first` does — refuses. `do_argfile()`'s `s`
  commands, `goto_buffer()`'s `:sb` family and `buflist_getfile()`'s
  `'switchbuf'` block fold.
- **`'scrollbind'`, `'cursorbind'`** bind one window to another, and
  **`'winfixbuf'`** is answered by splitting: their tests fold, their assignments
  and `get_varp()`/`copy_winopt()` plumbing go, and the rows go before the sweep.
  `'switchbuf'`, `'scrollopt'`, `'cmdwinheight'` and `'cedit'` lose their last
  reader here too — `'cedit'` via `didset_options()`, which a first run missed —
  and `'previewheight'` and `'previewwindow'` had none.

Left alone: `check_can_set_curbuf_disabled()` and `_forceit()` now always answer
yes and keep their nine callers, and `z{height}<CR>` still resizes the one window.

Checked in a terminal against `slim-vim`: after CTRL-W s, CTRL-W v or `q:`, `:q`
leaves the editor, where slim-vim stays open with the second window.

### The delta

**The twenty-seven rows that succeeded run bare**, read from the slim baseline.
`:close`, `:hide`, `:sbmodified`, `:wincmd` and `:windo` already failed.
Measured: 121,368 → **118,516 lines**, libc symbols 88 → 88.

## Phase 40 — no window sizes to set

With one window, `'winheight'`, `'winminheight'`, `'winwidth'`, `'winminwidth'`,
`'helpheight'`, `'splitbelow'`, `'splitright'`, `'splitkeep'`, `'equalalways'`,
`'eadirection'`, `'winfixheight'` and `'winfixwidth'` have nothing to decide. The
rows go. **The values do not**, and that is the whole difficulty of the phase.

The frame arithmetic still runs — `aucmd_prepbuf()` inserts its hidden window
with `win_split_ins()` and `win_close()` takes it out — and it reads the size
globals as it goes. A row is what writes a default into its global (see
`tools/orphanopts.py`), so dropping one alone would leave `p_wmh` at 0 and
`p_spk` NULL. **`tools/nowinsizes.py` gives each global its default as an
initialiser of its own** before the rows go: `FALSE`, `FALSE`, `"cursor"`, `TRUE`,
`"both"`, `1`, `1`, `20`, `1`. The value is exactly what the defaults gave, and
nothing can change it; the arithmetic keeps its own temporary writes, which a
variable allows as well as an option. `orphanopts.py` accepts an initialised
global by construction.

The two window-local fields are never set now, so their tests fold instead:
`win_split_ins()` keeping a fixed size, `winframe_remove()` passing over one,
`frame_setheight()`/`frame_setwidth()` reserving room for one, `win_enter_ext()`
sparing one, and `command_height()`'s loop over fixed-height frames, which never
runs. `frame_fixed_height()` and `frame_fixed_width()` answer `FALSE` for a window
and keep their recursive callers. `'helpheight'` had no reader but the callback
it shared with `'winheight'`, and the sweep takes both.

### The delta

**None the Ex sweep records beyond Phase 39's** — no row is retired. Each of the
twelve names is refused by `:set` now, which the phase probes against a
`:set ignorecase` control. Measured: 118,516 → **118,130 lines**, libc symbols
88 → 88.

## Phase 41 — the buffer list is walked by `:bnext` and `:bprevious` alone

**The list stays.** Every file edited is a buffer on it, `:bnext` and
`:bprevious` move through it, `:e #` reaches the alternate one, and
quitting still refuses while a hidden buffer is changed. Every other command on
it goes — fifteen rows: `:buffer`, `:buffers`, `:files`, `:ls`, `:badd`, `:balt`,
`:bdelete`, `:bunload`, `:bwipeout`, `:bfirst`, `:brewind`, `:blast`,
`:bmodified`, `:bNext` and `:bufdo`.

**`:bNext` is a row of its own**, spelled apart from `:bprevious` though it shares
the handler, so `:bN` goes with it; `:bp` still reaches `:bprevious`.

`tools/nobuflist.py` removes what a row cannot:

- **`:badd` and `:balt`** went through `ex_edit()` and `do_exedit()` with `:edit`,
  so only their terms go — and `do_ecmd()`'s `ECMD_ADDBUF` and `ECMD_ALTBUF`
  paths, which nothing else passed.
- **`:bdelete`, `:bwipeout` and `:bunload`** were `do_bufdel()` and `do_buffer()`,
  which the sweep takes. That leaves `do_buffer_ext()` one caller, `goto_buffer()`,
  and one action, `DOBUF_GOTO`, so its `unload` is always false and every branch
  that unloaded, deleted or wiped folds. **That fold is true only after the
  sweep**, so the phase asks after it: exactly one call, and that one.
  `set_curbuf()` and `empty_curbuf()` keep their unload paths, because
  `check_changed_any()` still passes one. `do_one_cmd()` stops asking whether a
  buffer-name argument belongs to one of the three.
- **Completion** for the retired names. `ex_listdo()` had `:bufdo` as its last
  user and goes whole.

A first run counted the retired names before the sweep, and found four in
`ex_listdo()` and `ex_bunload()` — handlers with no row, which the sweep then
took. The count is asked after the sweep now.

### The delta

**The ten rows that succeeded run bare** — `:buffer`, `:bNext`, `:bdelete`,
`:bfirst`, `:blast`, `:brewind`, `:buffers`, `:bwipeout`, `:files` and `:ls`, read
from the slim baseline. `:badd`, `:balt`, `:bmodified`, `:bufdo` and `:bunload`
already failed with no argument. Measured: 118,130 → **117,506 lines**, libc
symbols 88 → 88.

## Phase 42 — one buffer, always

The buffer list is the container the editor edits in — `firstbuf`, `curbuf` and
the buffer hash table are read everywhere — so **it stays, with exactly one
buffer on it between commands**. Two decisions were the user's, not the
process's, and were asked: editing another file **reuses the one buffer**, and
**there is no alternate file**.

**The mechanism is `'bufhidden=wipe'`, made unconditional.** `do_ecmd()` still
makes the new buffer and then closes the old one; it closes it with `DOBUF_WIPE`
instead of `DOBUF_UNLOAD`, or not at all under `ECMD_HIDE`. So `:e`, `:enew`,
`:next`, `:previous`, `:drop` and `gf` work as before, and a file left behind
takes its undo history, marks and local options with it. Changes cannot be lost
by it: `do_ecmd()` has already refused a changed buffer unless it was written or
`!` was given, exactly as under `'nohidden'`.

Three rows go to `ex_ni` — `:bnext`, `:bprevious` and `:keepalt` — and
`tools/onebuffer.py` removes what a row cannot:

- **Nothing is hidden.** `buf_hide()` answered from `'hidden'`, the `:hide`
  modifier and `'bufhidden'`, and all three go, so each of its sixteen call sites
  folds as if it said no and the sweep takes it. `close_buffer()` stops reading
  `'bufhidden'`, and `tools/droplocal.py` takes the field.
- **No alternate file**, which is itself a second buffer. Nothing writes
  `w_alt_fnum`: `do_ecmd()`, `set_curbuf()`, `do_exedit()` and `win_init()` stop,
  `:file`, `:read` and `:write` stop making an alternate buffer for a name, and
  `buflist_findnr(0)` and a `#` pattern find nothing — which is what they did when
  there was no alternate. CTRL-^ points at `nv_error`; `:e #` fails.
- **`:saveas` renamed the buffer by swapping names with an alternate buffer**
  made for the new name. With no alternate it would have written the file and
  kept the old name, so it renames the one buffer with `setfname()`. It is the one
  addition in the phase, and the reason is that the mechanism, not the behaviour,
  needed a second buffer.
- **The argument list stops making buffers.** `alist_add()` put every file
  argument on the buffer list, unloaded, when it was named. An entry is a name now,
  buffer number 0 — `alist_name()` and `editing_arg_idx()` already fall back to the
  name — and the one buffer is named for the first file only while it is still the
  empty buffer startup made.

**Stays:** `:qall`, `:wall`, `:wqall` and `:xall`, which are `:q` and `:w` with one
buffer, and which every harness here quits with. `'buflisted'`, whose field is
internal state the buffer code reads. No command-line option opened more than one
buffer, so none goes.

The phase checks the behaviour it changes against what it replaces: under
`'nohidden'` an unloaded buffer keeps its marks, so marking a line, editing
another file and coming back finds the mark; with one buffer it is gone, and
deleting to it changes nothing. It checks that `:e #` is refused and that
`:saveas` renames.

Two first runs failed usefully: the leftover counts included `setaltfname()` and
`buf_hide()`, which have no caller and which the sweep takes, and
`rename_buffer()`'s `xfname` had held the old short name for the alternate alone.

### The delta

**`:bnext`, `:bprevious` and `:keepalt`**, which succeeded run bare. Measured:
117,506 → **117,013 lines**, libc symbols 88 → 88.

## Phase 43 — no -c, --cmd, -R, -m, -M or -w

Six command-line options become what any unknown option is: exit 1, naming
itself. `+{command}` stays, and fills the same list `-c` did.

`tools/dropopts.py` removes `-R`, `-w` and `--cmd`, and the argument switch's
`case 'c':`. It refuses the other two, rightly, and `tools/nocmdargs.py` cuts them
by hand: **`-c` has a body of its own that falls through** into `-T` and `-u` —
`-c{command}` takes the rest of its argument and breaks, `-c {command}` falls
through to ask for the next one — and **`-M` falls through into `-m`**, so the
pair goes together. `--cmd` was the only long option that took an argument, so
the argument switch's `case '-':` goes, the option switch's one
`if (!want_argument)` can no longer be false, and `exe_pre_commands()` loses its
call and goes with the fields it read.

**The harnesses drove the editor with `-c`.** `tools/behaviour.py` and
`tools/exsweep.py` pass `+{command}` now, and — since Phase 18 took `-u` — no
`-u NONE` either. It is the same list in the same order,
so the change moves nothing against any binary either pipeline has made —
measured against `.reference/slim-vim`: 0 of 67 behaviour cases and 0 of 600
Ex-sweep rows differ from the baselines recorded with `-c`. Both are in every
phase's implementation digest, through `whimdelta.sh` and `verify.sh`, so every
boundary in both pipelines was verified again after the change.
`tools/clicheck.py` still passes `-c`: it runs at Phase 3, where `-c` exists.

The phase checks each dropped spelling — `-c qa!`, `-cqa!`, `--cmd qa!`, `-R`,
`-m`, `-M`, `-w7` — against a `+qa!` control.

### The delta

**None the Ex sweep records.** Measured: 117,013 → **116,892 lines**, libc
symbols 88 → 88.

## Phase 44 — no filters, sorting or alignment

Seven rows go to `ex_ni`: `:!` (with `:{range}!`), `:sort`, `:uniq`, `:retab`,
`:left`, `:center` and `:right`. **`:!` was kept in Phase 8 on purpose**, as the
sentence it printed instead of starting a process; it is dropped here on
request. The `!` operator key built nothing but a `:{range}!` command line, so
its row in `nv_cmds[]` points at `nv_error` — pointed, not deleted, as every row
there is. Completion for `:retab` goes with its row.

**`:r !cmd` and `:w !cmd` stay as they were.** They reach `do_bang()` through
`:read` and `:write`, not through the `:!` row, and keep Phase 8's refusal:
without their `!` being special, `:w !cmd` would write a file of that name.

### The delta

**The six rows that succeeded run bare** — `:sort`, `:uniq`, `:retab`, `:left`,
`:center` and `:right` — and **three behaviour cases**, `retab`, `sort_u` and
`sort_n`, which used them. `:!` already differed from Phase 8. Measured: 116,892
→ **115,798 lines**.

## Phase 45 — no `:drop`

`:drop` edited a file by making it the argument list and going to its first
entry: with one window and one buffer it was `:args` plus `:first`, both gone.
`ex_drop()` was the last caller of `set_arglist()` and `ex_rewind()`, and the
sweep takes all three.

### The delta

**None.** `:drop` already failed with no argument. Measured: 115,798 → **115,744
lines**.

## Phase 46 — no `:wall`, `:qall`, `:quitall`, `:wqall` or `:xall`

With one window and one buffer these were `:w`, `:q`, `:wq` and `:x` under longer
names. `:quitall` is `:qall`'s long spelling, the same handler, and goes with it.
`do_wqall()` and `ex_quit_all()` go with their rows.

**`tools/exsweep.py` quit every run with `:qall!`**, which is what made `:new`,
`:split` and the other window commands deterministic in `slim-vim`. It now runs
the binary once with `+qall!` and quits with `:q!` wherever that fails — which is
the same thing from Phase 39 on, where there is one window. Against `slim-vim`
it still quits with `:qall!`, and `tools/verify.sh` is all clear. The phase
checks that `:q!` still quits and `:qa!` is not a command.

### The delta

**The five rows**, which succeeded run bare. Measured: 115,744 → **115,646
lines**.

## Phase 47 — no `:startinsert`, `:startreplace`, `:startgreplace` or `:stopinsert`

Commands that entered or left Insert mode from a command line. `i`, `R`, `gR`
and Esc are the keys for that, and stay.

### The delta

**The four rows**, which succeeded run bare. Measured: 115,646 → **115,588
lines**.

## Phase 48 — no `:noswapfile`

There has been no swap file since Phase 21: the memfile is memory. The modifier
set `CMOD_NOSWAPFILE`, whose two readers in `ml_open()` and `buf_copy_options()`
were already empty blocks. It is matched by name in `parse_command_modifiers()`
before the table, so its branch goes as well as its row, and so does its line in
the completion arm for modifiers.

### The delta

**The row**, which succeeded run bare. Measured: 115,588 → **115,568 lines**.

## Phase 49 — one set of options

Every buffer and window option has two copies inside the editor, a global and a
local one. **The storage stays**: collapsing it would touch every option's reader
for nothing a user can see. What goes is every way to make the two copies differ,
so that `:set` — which writes both — is the only way an option is given a value,
and there is one set of options as far as anything outside can tell.
`tools/oneoptset.py` removes the four things that made them differ:

- **`:setlocal` and `:setglobal`** wrote one copy each. Their rows go to `ex_ni`,
  `ex_set()` stops choosing a flag for them, and their completion arms go.
- **`:set opt<`** copied the global copy into the local one. `<` is no longer an
  accepted suffix, and its three branches — boolean, number, string — fold, so
  `:set ts<` is an error like any other malformed `:set`.
- **Modelines** set a file's local copy from a `vim: set ...:` line, and the user
  was asked and chose to drop them. The four calls of `do_modelines()` go and the
  sweep takes it and `chk_modeline()`; every test of `OPT_MODELINE`, a flag
  nothing passes after that, folds; and `'modeline'`'s save and restore around
  `'binary'` in `set_options_bin()` goes. Then the rows of `'modeline'`,
  `'modelines'`, `'modelineexpr'` and `'modelinestrict'` go, `droplocal.py` takes
  `b_p_ml`, and `b_p_ml_nobin` — not an option, so with no `get_varp()` case that
  tool knows — goes by hand. A first run found that.

**Left alone:** a value detected from the file being read. `'fileformat'`, and
`'binary'` from `-b`, are the current file's state, and with one buffer only ever
one file's.

The phase checks that `:set ts<` is refused against a `:set ts=3` control, and
that `>>` on a file whose modeline says `sw=2` indents by the compiled-in four.
**The first version of that check proved nothing.** It used `ff=dos`, and
`'modelinestrict'` let a modeline set only whitelisted options, which
`'fileformat'` is not, so it passed against binaries that still read modelines.
`'shiftwidth'` is on the whitelist: measured, the Phase 48 binary indents by two
and this one by four.

**Measuring it showed something else.** `slim-vim` indents by four too, and so do
whim Phases 0 to 24, with `:set modeline?` answering `nomodeline`; Phases 25 to 48
answer `modeline`. The harnesses run as root, and upstream forces `'modeline'` off
for root — the check Phase 25 removed, as its section says. So a modeline was
read, as root, from Phase 25 until this phase, and no harness case has a modeline
to notice. Diffing `:set all` between Phases 24 and 25 shows that it is the only
value that moved besides the backup options that phase removed on purpose.

### The delta

**`:setlocal` and `:setglobal`**, which succeeded run bare. Measured: 115,568 →
**115,246 lines**.

## Phase 50 — only LF text files

Every line ends with LF when it is read and when it is written, and a CR is a
character like any other. `-b` goes, and with it `'binary'`, `'fileformat'`,
`'fileformats'`, `'endofline'`, `'fixendofline'`, `'endoffile'`, and the old
spellings `'textmode'` and `'textauto'`; so do the `++bin`, `++nobin`, `++ff` and
`++fileformat` arguments. `tools/lfonly.py` removes what a row cannot:

- **`readfile()` stops choosing and detecting a format.** The choice from `++ff`,
  `'binary'` and `'fileformats'`, the DOS and Mac detection, the loop that split
  lines at CR, CR stripping and its retry as Unix, the CTRL-Z at the end of a DOS
  file and the "[CR missing]" message all fold. A last line with no LF is still
  read and still reported as "[noeol]" — that describes the file.
- **`buf_write()` writes LF after every line, the last included, and no CTRL-Z.**
  Its CR branch goes by a local helper that keeps an `if`'s body and drops its
  `else`, which `cutil.fold_always` rightly refuses to guess.
- **Everything that compared a buffer's format with the one it was read in** —
  `file_ff_differs()`, `save_file_ff()`, `set_file_options()` — has nothing to
  compare, so its callers fold, including `unchanged()`, `bufIsChangedNotTerm()`,
  `set_init_1()` and `did_set_modified()`, and the sweep takes it with
  `get_fileformat()`, `set_fileformat()`, `default_fileformat()`,
  `msg_add_fileformat()` and `set_options_bin()`. stdin and fifos stop being read
  as binary.
- **`'endofline'` and `'endoffile'` had no initialiser in `buf_copy_options()`** —
  only resets, which the cut removed — so `tools/droplocal.py` does not recognise
  their shape, and their fields go by hand. So does `b_no_eol_lnum`, the last-line
  marker a binary write used.

**Several first runs failed, each on a count.** Two `else if (curbuf->b_p_bin)` in
`readfile()` until the format chain folded first; the detection block's
`fileformat == -1` test repeated inside itself, now anchored on what follows it;
two `save_file_ff()` calls outside the functions that die, found once the final
check reported *where* each leftover call sits rather than comparing a total.

The phase checks that `-b` is unknown and `:set ff=dos` refused against a
`:set ts=3` control; that `:%s/$/X/` on a CR LF file writes `one\rX\n`, where a
DOS file gave `oneX\r\n`; and that a last line with no LF gains one.

### The delta

**No Ex command; the behaviour cases `ff_dos` and `binary_mode`**, whose
`:set ff=dos` and `:set binary` are refused. Measured: 115,246 → **114,399
lines**.

## Phase 51 — a byte that is not UTF-8 is kept as it is

**Phase 12 changed this without declaring it.** It made UTF-8 the only encoding by
cutting the conversion layer at its entry points, and its table lists what each
cut function now answers — but not what that does to a file that is not valid
UTF-8. `slim-vim` reads such a file by falling back to latin1 and writes its
bytes back unchanged. With no fallback, `readfile()` replaced every invalid byte
with `?` (`bad_char_behavior`'s default, `BAD_REPLACE`) and made the buffer
read-only, and a forced `:w` wrote the `?`s. Measured: `ok\n\xff bad\n` comes back
as `ok\n? bad\n` from every whim binary since Phase 12, and unchanged from
`slim-vim` and whim Phases 0 to 11. No harness case has an invalid byte, which is
how it went unnoticed; it was found planning the UTF-8 phase, and the user was
asked what an editor that only edits UTF-8 should do.

**The answer was what `++bad=keep` already did.** The byte stays in the buffer as a
byte, shows as `<ff>`, is written back as it was, and the buffer is not made
read-only; "[ILLEGAL BYTE in line N]" is still reported, because that describes
the file. So keeping is the only behaviour, and `tools/keepbytes.py` folds every
test of `bad_char_behavior` — in the UTF-8 check and in the conversion loops —
drops `++bad` from `getargopt()`, and lets the sweep take `get_bad_opt()` and the
buffer's `b_bad_char`.

The phase checks, against a UTF-8 edit as control, that a file with `\xff` is
written back byte for byte after an edit to another line, that reading it leaves
`noreadonly`, and that `++bad=keep` is refused. **Its first run failed on the
probe, not the editor**: `+s/ok/OK/` runs on the last line, where Ex mode starts,
and an `:s` that does not match there stops the `:wq` after it. The probe says
`+1s`.

### The delta

**None the harnesses record.** Measured: 114,399 → **114,275 lines**.

## Phase 52 — UTF-8 is not a question

Since Phase 12, `mb_init()` sets the same five globals to the same values every
time: `enc_utf8`, `has_mbyte` and `enc_latin1like` TRUE, `enc_dbcs` and
`enc_unicode` 0. **456 places still asked them**, in every shape C allows — a bare
`if`, a chain of `&&` and `||`, a ternary, a comparison with a DBCS code page, an
argument — and each one was a branch for an encoding this editor cannot have.

`tools/utf8only.py` folds them as constants, on the source, and **never drops a
side effect**:

1. The five lose their declarations and their assignments in `mb_init()`, and every
   other mention becomes a marker — `__T__` for the three that are TRUE, `__Z__`
   for the two that are 0. A marker is an identifier, so the text still parses,
   and a `TRUE` already in the source is never mistaken for one the tool made.
2. Every expression holding a marker is simplified, innermost first, to a
   fixpoint: a ternary on a constant condition becomes its branch; in an `||` list
   a false operand goes and a true one ends the list, in an `&&` list the reverse;
   `!` flips a constant; parentheses around one collapse; `__Z__ == DBCS_x` is
   false. **An operand is dropped only where C would not have evaluated it, or
   where it is pure** — no call, no assignment, no `++` or `--`. Otherwise it stays.
3. Every `if`, `else if` and `while` on a constant marker folds with its else chain,
   by brace matching — from the last occurrence in a function, because the same
   false condition can be nested inside its own block, and folding the outer one
   first makes the inner vanish. A first test run met exactly that.
4. What is left — a marker compared with something that is not a constant, or
   assigned — becomes `TRUE`, `FALSE` or `0` again.

Measured on the phase's input: 239 expression simplifications and 261 statement
folds in 178 functions, 7 constants left as values. The sweep then takes the DBCS
and latin1 paths nothing reaches, and with them two libc symbols, `iswupper` and
`mblen`.

**Tested before it became a phase, against the binary it replaces.** Applied to a
copy of the Phase 49 source, compiled with every warning the sweep does not own
silenced, built, and run through the behaviour and Ex-sweep harnesses beside the
unfolded binary: 0 of 67 cases and 0 of 600 rows differ. That test also caught the
tool's own mistakes twice before it counted — once by crashing, and once by
reporting "0 differ" for a file the crash had left unchanged, which is why the
test now refuses to compare unless the tool succeeded and the file moved.

The phase checks `gUU` over *à é* for `c3 80 c3 89 0a`, byte for byte, and `x` on a
three-byte character.

### The delta

**None.** Folding a constant changes no behaviour, and the harnesses — with their
multibyte motion, case and insertion cases — are the check. Measured: 114,275 →
**112,439 lines**, libc symbols 84 → 82.

## Phase 53 — no conversion layer, no 'encoding'

Phase 12 cut the conversion layer at its entry points and left its body. Two ways
in were still open: **`++enc`** on `:e`, `:r` and `:w`, and a buffer whose
`'buftype'` is `help`, which `readfile()` read as latin1-or-utf-8. `tools/noconv.py`
closes both, and then everything behind them has one answer: the encoding name is
always empty, `need_conversion("")` is false, and so `converted`, the conversion
flags, the iconv descriptor, the `'charconvert'` temporary file and the retry with
the next encoding never change. Every test of them folds, in `readfile()` and
`buf_write()`; `buf_write_bytes()` loses the UCS-2, UTF-16, UCS-4 and latin1
writers no flag reached; the byte-order-mark check goes, since `check_for_bom()`
has answered "none" since Phase 12; the rewind that retried another encoding goes,
with its `retry` and `failed` labels.

**`'encoding'` goes**, and `mb_init()` stops asking `p_enc` — **but its NULL
branch was taken, once.** `common_init_1()` calls `mb_init()` before any option
exists, and that call filled the byte-length table with 1s and returned;
`set_init_1()` made the real one. Folded as never-taken, the first call ran on into
`init_chartab()` with no `curbuf`, and the editor crashed before its first command.
So `common_init_1()` now does what its call did then. **`'makeencoding'` goes with it** —
it converted `:make` output, `:make` went long ago, and it shared
`did_set_encoding()`, which is why that function survived the first attempt.

**The terminal is not converted either, and this is not tidying.** `input_conv`
and `output_conv` were `CONV_NONE` whenever `'encoding'` was utf-8. The only
assignment of `input_conv.vc_factor` was in the `mb_init()` branch folded above,
and `fill_input_buf()` divides by it: a first version of this phase folded the one
and not the other, and would have built an editor that divided by zero on its
first read of input. The post-condition grep caught the survivor before the build
did. Their tests fold in `ui_write()`, `fill_input_buf()` and `utf_find_illegal()`.

**The ten `mb_*` function pointers are calls.** `mb_init()` pointed all ten at the
UTF-8 implementations every time; 390 calls through them become direct calls to
`utfc_ptr2len()`, `utf_ptr2char()` and the rest, and the latin1 implementations
they were initialised to are swept. `mb_tail_off()` kept two dead returns after
its last live one from Phase 52; they go, and `dbcs_head_off()` with them.

Completion for `++ff`, `++enc` and `++bad`, left behind by Phases 50, 51 and this
one, goes from `expand_argopt()` and `get_argopt_name()`.

**The sweep met a declaration shape it had never deleted.** `enc_canon_table[]`
and `enc_alias_table[]` are written `static struct`, then the whole body on one
line, then the declarator alone — and gcc reports the declarator's line.
`deadsweep.py` walked back over a type only when that line began with `}`, so it
took the table and left `static struct {...}` open at file scope, where the next
declaration became "duplicate 'static'". It now recognises the one-line body too.
The branch is new and the old one untouched, so no earlier boundary could move —
and `slim-verify` and `whim-specpass` were run to show it, since the tool is in
every phase's implementation digest.

Both this and the crash above were found the expensive way: the phase program
failed, the pass fell through to an agent, and the agent's account named the two
causes. Its boundary and its synthesised residue were discarded; the fixes are in
the programs.

The phase checks that `:set enc?`, `:set menc?` and `++enc` are refused, that `gUU`
over *à é* still gives `c3 80 c3 89 0a`, and that an invalid byte is still written
back unchanged.

### The delta

**None the harnesses record** — no case converts. Measured: 112,439 →
**110,672 lines**, libc symbols 82 → 81 (`lseek`, whose two callers were the
retry's rewind and the help buffer's look at a file's first line — the second
already unreachable, behind a `c = TRUE` its own test could never pass).

## Phase 54 — no option without a variable

A row of `options[]` whose variable is `(char_u *)NULL` is an option `:set` accepts,
reports and ignores: its feature was never compiled in — folding, syntax, the GUI,
printing, cscope, the interpreter DLLs — or went in an earlier phase. **172 of
them.** `pipes/whim54.sh` computes the set from the table rather than listing it,
so a row upstream adds later without a variable goes too, and hands it to
`dropoptions.py`. None was buffer- or window-local, and no code outside the table
names one by string.

**The pattern met two traps, both worth keeping.** The variable field has to be
matched, not the row: a string default is often `(char_u *)NULL` too, and a first
count by row put `'messagesopt'`, `'wincolor'` and `'winhighlight'` among them. And
the spacing varies — `'termguicolors'` is `(char_u*)NULL` — so a pattern with the
space found 165. **And a row's flags can wrap onto a second line** — `'diffopt'`,
`'foldmarker'`, `'guifont'`, `'guifontwide'`, `'breakindentopt'` and `'undodir'` — so
a flag list without whitespace in it left those six behind; they were found only
when the options that survived were read by eye. The post-condition had a trap of its own: a row's flags and its
variable are on two lines, so a `grep` for rows left counted 0 whatever was left.
It reads across lines now, and was checked to count 172 on the phase's input.

The phase checks `:set sw` still works and that `'foldmethod'`, `'cursorline'`,
`'undofile'` and `'clipboard'` are unknown.

### The delta

**None the harnesses record** — no case sets an option without a variable.
Measured: 110,672 → **110,025 lines**.

## Phase 55 — no option nothing reads

Phase 54 took the options with no variable. These have one, and nothing but the
option machinery reads it — the declaration and the row, `get_varp()` and the
buffer copy for a local one, `set_context_in_set_cmd()`'s completion, and a
`did_set_*` callback that only validates the value or fills a flag set nothing
reads. Setting any of them changed nothing:

    autocompletetimeout  cdhome  cdpath  completetimeout  imcmdline  secure
    shellcmdflag  shelltemp  shellxescape  shellxquote  shortname  ttybuiltin
    warn  xtermcodes  commentstring  completefuzzycollect  completeitemalign
    helpfile  lispoptions  operatorfunc

and sixteen terminal codes the built-in tables and `:set` store and the editor
never sends — `t_8b t_8f t_EC t_EI t_GP t_RB t_RC t_RF t_RS t_SC t_SH t_SI t_SR
t_WP t_XM t_u7`. Their `KS_` enumerators stay, since the built-in terminal tables
still name them.

**Found by reachability, not by name.** Each option's variable was mapped from its
row — `p_xx`, `b_p_xx` from `BV_XX`, `wo_xx` from `WV_XX` (not `w_p_xx`, which a first
count used and so found `'list'` and `'number'` unread), `KS_XX` for a terminal
code — and every mention attributed to its function. Mentions in the plumbing did
not count. **A callback counted only through what it touched:**
`'belloff'`, `'casemap'`, `'display'`, `'jumpoptions'` and `'keymodel'` looked
unused until their callbacks' flag sets were followed to `vim_beep()`, the case
mappers, screen drawing, the jump list and selection; `'modified'`, `'terse'` and
`'wincolor'` act in the callback itself. Those stay. `cfc_flags`, `cia_flags` and
`opfunc_cb` were set and never read, so their options go. No option of the 36 is
named by string anywhere outside the table.

One real reader had to go first: `'cdpath'` was completed as a directory list,
the only use of `p_cdpath`. The phase greps afterwards for every variable, flag set
and callback of the 36, so a reader that appears later fails it. It checks `:set
sw` still works and that `'shelltemp'`, `'commentstring'` and `t_EI` are unknown.

### The delta

**None the harnesses record** — no case sets one. Measured: 110,025 →
**109,655 lines**.

## Phase 56 — no shell, runtime or keyword-program options

Six options whose readers survived only in machinery with nothing left to serve.
**`'shell'`, `'shellquote'` and `'shellredir'`**: no shell is ever run — `call_shell()`
and `mch_call_shell()` went long before — so `'shell'` only chose the default of
`'shellredir'` in `set_init_3()` and whether filename escaping doubled a `!` for csh,
and `'shellquote'` only wrapped `do_bang()`'s command line. **`'runtimepath'` and
`'packpath'`**: there is no runtime to find. Their readers were the completion of
`:colorscheme`, `:compiler`, `:ownsyntax`, `:setfiletype`, `:packadd` and
`:runtime` — every one of them `ex_ni` — and of `:set ft=`, which listed runtime
syntax, indent and ftplugin names. **`'keywordprg'`**: `K` is gone; only `:set kp=`
defaulting to `:help` read it. Each reader is folded before the rows go, and the
phase greps afterwards for every variable and helper.

**One plumbing site had a shape `droplocal.py` did not know.** `get_varp()`'s "local
if set" case for `'keywordprg'` reads `&curbuf->b_p_kp`, without the parentheses
every other such case has, so its two mentions counted as readers and the tool
refused. The phase removes that case by hand first; the shared tool is unchanged,
so no other phase's key moved.

### The delta

**None the harnesses record.** Measured: 109,655 → **109,039 lines**.

## Phase 57 — no lisp

`'lisp'` and `'lispwords'` go, and with them everything they switched on:
`get_lisp_indent()` for autoindent, `=`, `gq` and new lines; `lisp_match()` over
`'lispwords'`; `-` as a keyword character; `;` line comments in
`check_linecomment()`; and `findmatchlimit()`'s lisp mode, which stopped `%` at a
`;` comment and skipped `#\(` character literals. `'lispoptions'` went in Phase 55.

`b_p_lisp` is folded as false at every reader rather than stubbed — nine places
in `findmatchlimit()` alone — so each branch it guarded is gone or taken
unconditionally. One test inverts: `op_reindent()` skipped the last line of a
range only when re-indenting with `get_lisp_indent()`, so its `how !=
get_lisp_indent` is always true and the branch is kept. The phase checks the two
options are unknown and that `%` on `(a ; b)` now matches across the `;`.

### The delta

**None the harnesses record** — no case sets `'lisp'`. Measured: 109,039 →
**108,651 lines**.

## Phase 58 — no language mappings

`'iminsert'` and `'imsearch'` are 0 from here on, so language mappings are never
active and nothing can make them so. `:lmap`, `:lnoremap`, `:lunmap` and
`:lmapclear` point at `ex_ni` and lose their completion. CTRL-^ in Insert mode and
on the command line is still consumed — its `case` stays, so it does not start
inserting itself — and toggles nothing. `MODE_LANGMAP` is never set, so every test
of it folds: in `edit()`, `ex_append()`, `ins_insert()`,
`normal_cmd_get_more_chars()`'s lookup for `r`, `f` and `t`, `getcmdline_int()`
for `/`, `?` and `@`, `handle_mapping()`, `vgetorpeek()`, `get_map_mode()` and
`map_mode_to_chars()`. The status line's `<lang>` goes with `get_keymap_str()`,
which only ever printed it.

**The declared delta was wrong once, and the harness said so.** It named all four
commands; `:lunmap`'s row did not move, because bare `:lunmap` already failed for
want of an argument and `ex_ni` fails too. The declaration was corrected rather
than the check widened. The phase checks the two options are unknown, `:lmap` is
refused, and CTRL-^ in Insert mode inserts nothing.

### The delta

`:lmap`, `:lnoremap` and `:lmapclear`, now `ex_ni`. Measured: 108,651 →
**108,374 lines**.

## Phase 59 — no command-line completion

The command line no longer completes anything. In `getcmdline_int()` the
`'wildchar'` and `'wildcharm'` keys, S-Tab, CTRL-D (list), CTRL-A (insert all),
CTRL-L (longest match) and CTRL-N/CTRL-P over matches go; each of those keys is now
an ordinary character, CTRL-N and CTRL-P browse history as they did when there were
no matches, and CTRL-L still adds a character to an incremental search. The six
wild* options go with them: `'wildchar'`, `'wildcharm'`, `'wildmode'`,
`'wildoptions'`, `'wildignore'` and `'wildignorecase'`.

What completion shared with filename expansion stays: `expand_filename()` →
`ExpandOne()` with `EXPAND_FILES`, and the argument list through
`expand_wildcards()`. So `ExpandFromContext()` keeps its file branch and loses the
rest — options, mappings, buffers, highlight groups, `++opt`, every command's
arguments — and `ExpandOne()` keeps the one mode its last caller asks for. The
phase checks after the sweep that `expand_filename()` is that last caller.

**Three things kept the machinery alive, and each was found by the post-condition
greps rather than by reading.** `didset_options2()` still parsed `'wildmode'` into
`wim_flags` at startup, which nothing read. Every `options[]` row still named the
callback that completes its value — 29 of them — so the table kept `ExpandGeneric()`
and the fuzzy matcher reachable; no code reads that field any more, and the rows
now hold `NULL`. And the check that `ExpandOne()` had one caller ran first before
the sweep, when its dead callers were all still there. The sweep then took
6,208 lines.

**`:e` does not expand a wildcard, and has not since Phase 7 — deliberately.** The
first probe here asked that `:e onlyo*` edit `onlyone.txt`. It failed, and failed
identically on the previous phase's binary, which wrote a file named `onlyo*`. That
is Phase 7's declared delta, not a regression: Phase 7 replaced
`gen_expand_wildcards()` with `save_patterns()`, so `:e *.c` names a file
literally, and said so. Bisecting the boundaries confirms it — q6 expands the
pattern, q7 does not. An earlier draft of this section called the loss silent and
placed it "at or before Phase 12"; that came from testing binaries before reading
Phase 7, and was wrong. This phase does not change it, and the probe checks `:e`
on a plain name instead.

### The delta

**None the harnesses record** beyond Phase 58's. Measured: 108,374 →
**102,166 lines**.

## Phase 60 — no suffix, case, delay, verbose-file, debug or filter-program options

Seven options whose default is the only value anything could still act on.
`'suffixes'` ordered wildcard matches, and wildcards have not expanded since
Phase 7 removed globbing, so `match_suffix()` and its two reordering blocks go.
`'fileignorecase'` is off, and its five tests fold as false. `'autocompletedelay'`
is 0, so `inchar_loop()`'s delay was never pending. `'verbosefile'` is empty, so
the file is never opened: `redir_write()`, `redirecting()` and the
`verbose_enter`/`verbose_leave` family fold, and `fopen` leaves the libc symbols.
`'debug'` is empty, and its tests in `emsg_not_now()`, `emsg_core()` and
`vim_beep()` fold.

**`'formatprg'` and `'equalprg'` were suspicious, and dead.** Their only effect was
to make `gq` and `=` build a `:{range}!prg` line, and `:!` has been `ex_ni` since
Phase 44 — so a non-empty value turned a working operator into an error. `gq` and
`=` take the internal path unconditionally now, and `op_colon()` loses its
indent and format branches. `get_varp()`'s `'equalprg'` case has the same
missing parentheses as `'keywordprg'`'s in Phase 56 and is removed by hand.
`'formatoptions'` and `'formatlistpat'`, suspected with them, are live —
auto-wrap, comment leaders, `gq`, `J` and numbered-list indent read them — and stay.

The phase checks the seven are unknown and that `gqq` with `tw=4` still breaks
`aaa bbb` into two lines.

### The delta

**None the harnesses record.** Measured: 102,166 → **101,826 lines**; libc symbols
81 → 80.

## Phase 61 — no window title

`'title'`, `'titlelen'`, `'titleold'`, `'titlestring'`, `'icon'` and `'iconstring'`
go, and with them everything that set or restored the terminal's title:
`maketitle()` and its thirteen callers, `need_maketitle` and the six places that
asked for an update, `resettitle()`, `mch_settitle()`, `mch_restore_title()` in
`:stop`, exit, a terminal change and `value_changed()`, `set_title_defaults()`,
`term_settitle()`, the X11 title and icon probes, and the title-stack push at
startup and pop at exit. The editor no longer writes to the terminal's title at
all. The `t_ts`, `t_fs`, `t_ST` and `t_RT` codes stay, with the other terminal codes.

**Two of the phase's own checks were wrong first, and both failed loudly.** One
guarded `do_exedit()`, where `n` held the argument index only to decide whether to
update the title, by requiring no other mention of `n` — but `n` also saves and
restores `readonlymode` around `:view`. It now requires exactly those three
mentions. The other counted five `need_maketitle = TRUE` assignments where there
are six: `maketitle()` sets it itself before an early return. Each was tried first on
a copy of the Phase 59 boundary, since this phase touches nothing Phase 60 does.

### The delta

**None the harnesses record.** Measured: 101,826 → **101,188 lines**.

## Phase 62 — no buffer-type, file-type, listing, jump, update-time or autowrite options

Seven options, each checked by what its readers still did:

- **`'buflisted'`** — every reader chose which autocommand event to fire, and
  `apply_autocmds_group()` has been `return FALSE` since autocommands went, or
  searched a buffer list of one. The `set_buflisted()` calls go with it.
- **`'filetype'`** — every reader fed the FileType event, which cannot fire, or
  `fix_help_buffer()`, and `:help` is `ex_ni`.
- **`'buftype'`** — the one that was live, but only through `:set bt=`: `nofile`,
  `nowrite`, `acwrite` and `prompt` refused `:w` and skipped reading, and `help` set
  `b_help`. Nothing inside the editor ever set it. `bt_dontwrite()`,
  `bt_nofilename()`, `bt_nofileread()` and `bt_prompt()` fold as false at every
  caller — including two inside one `snprintf` line in `fileinfo()`, the
  `[Not edited]` and `[New]` notes, and `buf_write()`'s `nofile_err`, set in three
  branches and read in two tests and one condition.
- **`'jumpoptions'`** — empty, so the "stack" behaviour of the jump list folds.
- **`'updatetime'`** — **dead, though it looked live.** After that long idle,
  `inchar_loop()` asked `trigger_cursorhold()`, which is `return FALSE`, and called
  `before_blocking()`. Its swap sync, `updatescript(0)`, reaches an `ml_sync_all()`
  whose body is empty; its terminal flush only writes while `sync_output_state` is
  above zero, which is inside `update_screen()` or `redraw_after_callback()` — both
  close it before returning, with no `return` or `goto` in between — so never at
  idle. The idle wait therefore goes: a wait with no timeout blocks at once, and
  `before_blocking()`, `updatescript()`, `ml_sync_all()` and the `scriptout`
  save and restore in `wait_return()` go with it. A first version of this phase
  kept the timeout at its 4000 ms default, on the strength of the call alone; it
  was corrected in place when the chain was read to the end.
- **`'autowrite'`** and **`'autowriteall'`** — off, so `autowrite()` always failed and
  `autowrite_all()` returned at once. Their callers fold, and so does the `CCGD_AW`
  flag — including the two places, `:next` and `do_argfile()`, that passed it
  unconditionally.

**The phase took six runs to get right, and every failure was the post-condition
grep or a tool refusing, never the build.** `droplocal.py` refused `b_p_bl` with
two plumbing sites where it requires three — the folds had already taken its
initialiser, so its field and `get_varp()` case go by hand — and then refused
`b_p_bt` because `fileinfo()` still called `bt_dontwrite()` a second time. The
grep then found `CCGD_AW` and `nofile_err` alive, and — once the idle wait was
removed — `did_start_blocking`, still read by the loop's exit test. That one needed
thought rather than deletion: blocking now starts on the first wait with no
timeout, so the flag was always TRUE where it was tested, and the term goes so that
an interrupted wait still returns instead of blocking again. Each was a reader the
first reading had missed, not a reader the check invented.

### The delta

**None the harnesses record.** Measured: 101,188 → **100,643 lines**.

## Phase 63 — no jump list

The per-window jump list goes: `w_jumplist`, `w_jumplistlen` and
`w_jumplistidx`; `setpcmark()` appending to it; CTRL-O and CTRL-I walking it
through `movemark()`; `:jumps` and `:clearjumps`, now `ex_ni`; `cleanup_jumplist()`;
copying it into a new window and freeing it with one; and the loops in
`mark_adjust_internal()`, `mark_col_adjust()`, `mark_forget_file()` and
`fmarks_check_names()` that kept its marks right when lines moved or a file was
forgotten.

**What stays, because it is not the jump list.** The previous-context mark behind
`''` and `` ` ` `` — `w_pcmark`, still set by `setpcmark()`. The change list and
`g;`/`g,`: `nv_pcmark()` served both, and keeps that half. `:keepjumps`, which
guards the pcmark and the change list too. And `JUMPLISTSIZE`, which sizes the
change list. CTRL-O in Select mode still runs one Visual command; elsewhere CTRL-O
and CTRL-I beep, and `<Tab>` is mapped to `%` in this build, so losing CTRL-I's
meaning costs nothing typed. The phase greps afterwards that `w_pcmark` and
`movechangelist` survived, since either going would mean it took more than the
jump list.

It was tried first on the Phase 61 boundary, before Phase 62 was recorded. That
trial failed only on the `'jumpoptions'` block Phase 62 removes — so it checked
this phase's own script and nothing else.

The phase checks `:jumps` is refused, CTRL-O after `3G` leaves the cursor on line
3, and `''` after `3G` still returns to line 1.

### The delta

`:jumps` and `:clearjumps`, now `ex_ni`. Measured: 100,643 → **100,354 lines**.

## Phase 64 — no formatting, comment or nroff-macro options

Five options, each dropped with the machinery that only it gave a meaning to.

- **`'comments'`** — no comment leader is recognised. `get_leader_len()` and
  `get_last_leader_offset()` would answer 0 and -1 everywhere, so every reader
  folds that way. The following all go:
  - `open_line()` copying, replacing, right-aligning and padding a leader;
  - `insertchar()` completing a `*/`, through `end_comment_pending`;
  - `J` removing leaders, through `skip_comment()`;
  - `same_leader()`, and the leader a formatted or wrapped line keeps;
  - `gd` skipping comment lines;
  - `%` skipping a `//` comment when `buf_has_cstyle_comments()` said the buffer
    looked like C.

  `check_linecomment()` stays, because `findmatchlimit()` still uses it.
- **`'formatoptions'`** — fixed at its default, `tcq`. With no leader, `c` and `q`
  have nothing to act on, so what is left is `t`:
  - Typing still wraps at `'textwidth'`, and `'paste'` still stops it, since
    `has_format_option()` answered FALSE under paste.
  - `gq` still formats.

  Every other flag was off, and its code goes:
  - `a`: `auto_format()`, `check_auto_format()`, `did_add_space` and all 18
    calls;
  - `w`, `n`, `2`, `b`, `l`, `v`, `m`, `M`, `B`, `1`, `p`, `]`, `j`, `r`, `o` and
    `/`.

  `format_lines()` is left as a plain paragraph loop with no second-line indent.
  `internal_format()` breaks only at blanks, which removes the multibyte branch,
  and `Insstart_textlen` and `Insstart_blank_vcol` go too.
- **`'formatlistpat'`** — only `n` read it, through `get_number_indent()`.
- **`'paragraphs'`** and **`'sections'`** — no nroff macro starts a paragraph or a
  section. `{`, `}`, `[[`, `]]`, `(`, `)` and the `ip`/`ap` objects stop at blank
  lines, form feeds and braces; `inmacro()` goes. These were not folded to their
  default, which would have kept a table of nroff macro names. Dropping the
  recognition is the same choice as for `'comments'`, whose default would have
  kept all of the leader machinery.

**And the two mechanisms that were left reading what those options described.**
An option and its only consumer are one cut, not two:

- **The format operator.** `gq` and `gw`, their doubled `gqq`/`gqgq`/`gww`/`gwgw`,
  `op_format()`, `format_lines()` and `fmt_check_par()`. A paragraph was only a
  paragraph in order to decide where a format stopped. What stays is the wrap
  while typing: `'textwidth'` and `'wrapmargin'` still break a line through
  `insertchar()` and `internal_format()`, and `'paste'` still stops it. With no
  `gq`, `INSCHAR_FORMAT` is never set, so `comp_textwidth()` loses the flag that
  chose the screen width for it, and `insertchar()` loses its `c == NUL` entry.
- **Go to local declaration.** `gd` and `gD`, `nv_gd()` and `find_decl()`, which
  searched from the start of the block the cursor was in. `gd` was the only
  caller. `gq`, `gw`, `gd` and `gD` now fall to `nv_g_cmd()`'s default and beep.
- **The `=` operator.** `==`, `=G` and the rest. `op_reindent()` re-applied
  `get_indent()` — the indent the line already has — because `'equalprg'` went in
  phase 60 and C-indenting is off, so `=` had nothing left to compute. Its
  `nv_cmds` row points at `nv_error`.
- **The `!` operator, which was already dead.** Its `nv_cmds` row has been
  `nv_error` for phases, and `get_op_type()` is reached only from `nv_operator()`,
  so `OP_FILTER` could no longer be set at all. What goes is the dispatch nothing
  reached: the `OP_FILTER` case, the `!` that `op_colon()` typed after a range,
  and `do_bang()`'s `bangredo` block — the only thing that set it. **`:w !cmd` and
  `:r !cmd` still reach `do_bang()`**, and `do_filter()` still says the command is
  not available in this version, so the filter commands are untouched.
- **What C-indenting left behind.** The engine went phases ago — no
  `get_c_indent()`, no `cin_*` anything, and none of `'cindent'`, `'cinoptions'`,
  `'cinkeys'`, `'cinwords'`, `'indentexpr'` or `'indentkeys'`. What stayed was a
  switch wired to `FALSE` and its plumbing: `cindent_on()`, which is
  `return FALSE`, and **`can_cindent`, written in ten places and read in none.**
  gcc does not warn about that — a static that is assigned counts as used — which
  is the same blind spot `deadfields.py` exists for, one level up. `cindent_on()`'s
  two callers fold: CTRL-U in `ins_bs()` keeps the indent for `'autoindent'` alone,
  and the multi-character insert in `insertchar()` stops asking. `set_can_cindent()`
  goes with the flag. Three of the ten writes are the whole body of an `if`, so the
  test goes too — and each is scoped to its function, because `if (inindent(0))`
  also guards `do_pending_operator()`'s `oap->motion_type = MLINE`, which stays.

**`'smartindent'` is kept, and checked rather than assumed.** `may_do_si()`,
`did_si`/`can_si`/`can_si_back`/`no_si` and `open_line()`'s `{`, `}`, `#` and `)`
rules are a different mechanism from `'cindent'`, and this build switches it on by
default. The phase greps that all four survive, and a probe indents `y;` by one
`'shiftwidth'` after a line ending in `{` and brings `}` back out.

The `opchars[]` rows for `g`+`q` and `g`+`w` stay. The table is positional — its
index *is* the `OP_*` value — so a removed row would renumber every operator
after it. Nothing reaches them: `nv_g_cmd()` reaches the default first.

**Deleted outright rather than left to the sweep**: `auto_format()`,
`check_auto_format()`, `paragraph_start()`, `op_format()`, `format_lines()` and
`fmt_check_par()`. The last three matter for order — `comp_textwidth()` loses its
argument in the same phase, and `format_lines()` would still be calling it with
one when the sweep compiles.

**The options half does not fold `format_lines()` or `fmt_check_par()` first.** An
earlier version did, twenty-odd edits deep, and then the operator half deleted
both. Folding a function that is about to go is work the phase throws away; the
output is identical either way, and that was checked rather than assumed.

**Seven dry runs, and not one failure was a broken build.**

1. **The script failed its own counts.** `open_line()`'s leader block holds
   `lead_len = 0` statements and an `if (lead_len > 0)` of its own, so it is
   dropped first, by a pattern anchored on its first declaration.
2. **`phasecheck.sh` found `extra_len` set and not read** — it sized the leader's
   allocation and nothing else.
3. **The post-condition grep found `oparg_T`'s `cursor_start`**, which was `gw`'s
   alone. `deadfields.py` will not touch it: `pagescroll()` has
   `oparg_T oa = { 0 };`, and a **positional** initialiser names no field, so the
   tool keeps every field of a type that has one. It goes by hand, and `{ 0 }`
   fills only the first field, so removing a later one is safe.
4. **`do_bang()`'s `theend:` label went unused.** The `bangredo` block held the
   only `goto` that reached it, and a label nothing jumps to is a warning. The
   free below it runs either way, so only the marker goes.
5. **The `=` probe asserted the wrong thing**, and the measurement is the useful
   part. A retired operator does not let the motion through: it abandons the rest
   of the sequence. Measured on the q63 boundary, `=jix` gave `xa|b|c|` — `=`
   took `j`, came back to line 1 and inserted — while `!jix`, already `nv_error`,
   left the file alone. So the check is that **both** keys now leave it alone,
   with `!` as the invariant that says what a retired operator looks like, and a
   bare `ix` as the control that proves the binary still inserts.

### The delta

Three behaviour cases: `format_gq` (`gqq` with `tw=20`, which now beeps and
changes nothing), and the two that set the options, `format_comment` and
`open_comment`. No Ex command moves — these are all Normal-mode keys, and
`:center`, `:left` and `:right` were `ex_ni` long before this phase.

The phase checks:
- the five options are unknown to `:set`;
- typing `aaa bbb ccc ddd` with `tw=10` still wraps to two lines;
- `gqq` and `gqj` leave a long line exactly as it was;
- `gd` on `x` leaves the cursor where it is;
- `=jix` and `!jix` leave the file alone, while `ix` still inserts;
- `'smartindent'` still indents `y;` after a line ending in `{`, and `}` comes back;
- `}` from line 1 passes `.PP` to the last line.

Two more failures came from the checks rather than the cuts:

6. **`if (inindent(0))` matched twice.** It guards a `can_cindent` write in `edit()`
   and `oap->motion_type = MLINE` in `do_pending_operator()`, which stays. Each of
   the three `if`-bodied writes is now scoped to its own function.
7. **The `'smartindent'` probe sent no carriage return**, so `GAy;` appended to the
   same line and the probe failed where the editor was right. Calibrated against
   the q63 binary, which gives `if (x) {` / `    y;` / `}` for the corrected keys.

Measured: 100,354 → **97,734 lines**.

## Phase 65 — no rot13, no operator function, no empty key handler

Three cuts, and only the first changes what the editor can do.

- **rot13.** `g?` is the one operator here that encodes rather than edits. It goes
  whole: `nv_g_cmd()`'s case, the `OP_ROT13` dispatch label, `nv_search()`'s
  redirect — which is how `g?` reaches the operator while a search is pending —
  and `swapchar()`'s three arms, after which `swapchar()` is the case-changing
  function it always really was.
- **The operator function.** `g@` has had nothing to call since the eval feature
  went: `op_function()` was one `emsg()`, and `'operatorfunc'` does not exist to
  name a function anyway. The dispatch, the `OP_FUNCTION` term in the
  `motion_force` test and `op_function()` itself go, and
  `e_eval_feature_not_available` falls to the sweep with its only reader.
- **An empty call.** `ins_ctrl_x()` had an empty body — CTRL-X in Insert mode
  began a completion, and completion went in phase 32. The key stays inert, but
  it no longer calls a function in order to do nothing.

**Three things are kept deliberately, because "does nothing" and "should be
deleted" are different claims.**

- **CTRL-P and CTRL-N in Insert mode are `break;`** — they do nothing *on purpose*.
  Deleting the labels would drop them into `normalchar`, which **inserts the
  control character**, so removing dead-looking code would add behaviour. The
  phase greps that `case Ctrl_P:` survives.
- **`zy`, `zp` and `zP` are live.** It looks as though `zy` must reach
  `internal_error("get_op_type()")`, since `opchars[]` has no `{'z','y'}` row —
  but `get_op_type()` special-cases `'z'`+`'y'` to `OP_YANK` before it consults
  the table. Measured on the q64 binary before cutting: no error, no message.
  This is why the "dead weight" list was checked key by key rather than read off
  the table.
- **The `opchars[]` rows for `g?` and `g@` stay**, for the reason phase 64
  records: the table is positional, so a removed row renumbers every operator
  after it. Nothing reaches them once `nv_g_cmd()` has no case.

The `'?'` and `'@'` case labels are edited **scoped to `nv_g_cmd()`**: another
switch entirely has `'?'` and `'@'` adjacent, and an unscoped edit would have had
two places to choose between — the same trap as `if (inindent(0))` in phase 64.

### The delta

**None.** No Ex command moves, and no behaviour case covers rot13 — the harness
never encodes anything. The probes check `g?g?` and `g??` no longer encode, that
`g@g@` is refused, that `gUU`, `guu` and `g~~` still change case (they share
`swapchar()` with the arms that went), and that `zyy` still yanks.

Measured: 97,734 → **97,684 lines**.

## Phase 66 — no sentences, paragraphs, sections, methods, #if blocks or comment blocks

One idea, cut at all three places it was reachable from. A sentence you cannot
move over is not one you can select, or address a line range with.

- **The motions.** `(` and `)` by sentence, `{` and `}` by paragraph — these four
  `nv_cmds` rows point at `nv_error` — and from `nv_brackets()`/`nv_bracket_block()`:
  `[[` `]]` `[]` `][` by section, `[m` `]m` `[M` `]M` to a method's braces, `[#` `]#`
  to the enclosing `#if`/`#endif`, and `[/` `]/` `[*` `]*` to the enclosing C comment.
  The dispatch sets shrink from `"{(*/#mM"`/`"})*/#mM"` to `"{("`/`"})"`.
- **The text objects.** `is`, `as`, `ip` and `ap` — `current_sent()` and
  `current_par()`.
- **The Ex addresses.** `'{`, `'}`, `'(` and `')` as line addresses, which
  `get_address()` answered by calling `findpar()` and `findsent()`.

After which `findsent()`, `findpar()` and `startPS()` have no callers at all, and
the concept is gone from the editor rather than merely unbound.

**What stays, and is checked rather than assumed**: `%` and the enclosing-bracket
motions `[{` `]}` `[(` `])`, which are `findmatchlimit()` and never had anything to
do with paragraphs; the `(` `)` `{` `}` `[` `]` `<` `>` **text objects** (`i{`, `a(`
…), which are `current_block()`; `iw`/`aw`; and the `'[` `']` `'<` `'>` marks, which
`get_address()` answers from stored positions.

**Four failures, all in the phase's own machinery rather than the tree.**

1. **The method test matched twice.** `if (cap->nchar == 'm' || cap->nchar == 'M')`
   is both the head that picks the character to match and the half that walks out
   to the method. The counted helpers cannot express "the second of two" — they
   die on any count but the one given — so the walk-out is cut from a slice that
   starts at it, and only then is the head the single match the counted fold wants.
2. **A dead assignment in the script itself**, left over from the first attempt at
   that ordering.
3. **`prev_pos` was set and not used** once the walk-out went. gcc reports
   `-Wunused-but-set-variable`, which `deadsweep.py` does not handle: it deletes
   *unused* variables, not written ones. The declaration and both writes go by
   hand. `c` is a plain unused variable after the same cut, and the sweep takes it.
4. **`lines()` was never defined in this script** — the three `prev_pos` removals
   were carried over from phase 64 without its helper.

### The delta

**None.** No behaviour case moves over a sentence or a paragraph, and no Ex
command changes. The probes check that each cut key leaves the file untouched —
a beep abandons the rest of a `:normal!` sequence, so the `ix` after it never
runs, with a bare `ix` as the control — that `[{` still walks out to the enclosing
`{`, `%` still matches, `di{` still deletes a block's contents without its braces,
and `'{,'}d` is refused.

Measured: 97,684 → **96,848 lines**.

## Phase 67 — no mouse, no spell plumbing, no write-only flags

Three cuts, none of which changes what the editor can do, because none of it
could happen in the first place. This is the first phase driven by
`tools/coverage.sh` and by a scan for **write-only statics**, rather than by a
capability to remove.

- **The mouse, which cannot arrive.** There is no `'mouse'` option row, and
  `setmouse()`, `mch_setmouse()`, `mouse_has()` and `p_mouse` are all gone, so
  nothing ever asks a terminal to report mouse events. What served them goes:
  `is_mouse_key()` and the term in the input loop that called it,
  `reset_dragwin()`/`reset_held_button()` with `dragwin` and `held_button`,
  `mouse_row`/`mouse_col` and `old_mouse_row`/`old_mouse_col` — a save-and-restore
  pair nothing else reads — the 18 mouse rows of `key_names_table`, the `[MOUSE]`
  entry of the terminal string table, and `check_termcode()`'s mouse matching.
  **The 26 `nv_cmds` rows stay at `nv_error`**: that table's index is a permutation
  of its rows, so a removed row renumbers the keys after it.
- **The spell plumbing.** `spellvars_T` was one field, `win_line()`'s `spv`
  parameter was already `__attribute__((unused))`, and `win_update()` declared one
  on the stack only to pass its address twice.
- **Fourteen write-only statics.** `did_check_timestamps`, `was_safe`,
  `did_emsg_syntax`, `typebuf_was_empty`, `in_mch_delay`, `mr_patternlen`,
  `frame_locked`, `swap_exists_did_quit`, `did_swapwrite_msg`, `autocmd_nested`,
  `dragwin`, `held_button`, `oldtitle_outdated`, `deadly_signal`. Two were a whole
  function body, so `state_no_longer_safe()` and its two calls go with `was_safe`.

**`vim_ignored` is not one of them, though it looks identical to the detector.**
Its five sites are `vim_ignored = ftruncate(...)`, `= dup(2)` and
`= write(1, ...)`: it exists to swallow `warn_unused_result`, and removing it
*adds* warnings — a `(void)` cast does not silence that attribute in gcc. The
phase greps that it survives.

**One real change of behaviour is buried in the mouse cut.**
`looks_like_mouse_start` is not mouse-specific despite its name: it is set for any
two-byte `ESC [` termcode whose third byte is not a digit, and it *defers* the
match so a longer code — a mouse one — can win instead. With no mouse code able to
arrive, deferring can only lose, so the fold makes such a code match at once.
`tools/arrowcheck.py`, which drove a real pty, was what would have caught that
going wrong, until it was retired after Phase 82.

**Two failures, both in the phase's own counting, and both caught by a guard
rather than by the build.**

1. **A probe that could not fail.** It asserted `:map <LeftMouse> x` is refused
   once the name is gone. Measured on both binaries: **an unrecognised `<...>` is
   taken as a literal string, not refused** — `<Foo>` and `<ZZnotakey>` are
   accepted too. The evidence that the names are gone is the grep; what the probe
   checks now is that a name which *does* exist still maps.
2. **Thirteen of eighteen rows.** Five mouse rows — `DecMouse`, `JsbMouse`,
   `NetMouse`, `PtermMouse`, `UrxvtMouse` — are written across **three** lines
   (`{`, `FALSE,`, then code and name), so a single-line pattern could not see
   them. This is phase 54's wrapped-option-row trap again. Both patterns are
   anchored on the *name*, which is what keeps them off the sixth three-line row,
   `SNR`.

### The delta

**None.** No key, command or option changes — every cut is code nothing could
reach. Measured: 96,848 → **96,636 lines**.

## Phase 68 — one window, structurally

**This phase establishes an invariant and then spends it**, which is why it is the
largest cut here since the early ones.

A window is created in exactly two places: `win_alloc_firstwin()`, once at startup,
and `win_split_ins()`, whose **only** caller is `aucmd_prepbuf()`. `win_split()`,
`make_windows()` and `win_new_tabpage()` have no mentions at all. A tabpage is
created once, by `alloc_tabpage()` in `win_alloc_first()`. So removing the
autocommand window means nothing can ever add a window or a tabpage again:

```
    firstwin == lastwin        first_tabpage->tp_next == NULL
```

`one_window()`, `last_window()` and `only_one_window()` are then constant TRUE —
not as an observation about the harness, but as a consequence of the two creation
sites — and every caller folds.

**Why the autocommand window can go.** `aucmd_prepbuf()` splits one open only when
no window shows the buffer, in order to run autocommands in it — and
`apply_autocmds_group()` has been `return FALSE` since autocommands went. The
window was built to run nothing.

**What was already a no-op**, which is why this removes capability from the source
and none from the editor:

- `win_close()` tests `last_window()` first and answers *cannot close last window*,
  so the calls in `ex_quit()`, `ex_exit()` and `do_exedit()` could never close
  anything — and the first two reach `getout(0)` before them regardless.
- `do_exedit()`'s call is guarded by `old_curwin != NULL`, and its one caller
  passes `NULL`.
- `close_windows()` loops `wp != NULL && !(firstwin == lastwin)`, false at once,
  then over tabpages other than `curtab`, of which there are none.

Gone with them: `win_split_ins`, `win_close`, `close_windows`, `win_close_othertab`,
`close_last_window_tabpage`, `close_tabpage`, `free_tabpage`, `winframe_remove`,
`win_equal`, `win_equal_rec`, `frame2win`, `win_altframe`, `is_aucmd_win`, the four
snapshot functions, `win_alloc_popup_win`, `win_init_popup_win` and the `aucmd_win[]`
table.

**What stays, and is checked rather than assumed.** `win_comp_pos()`,
`frame_comp_pos()`, `last_status()` and `last_status_rec()` are reached from
`shell_new_rows()` and `did_set_laststatus()`, so a terminal resize and
`:set laststatus` still compute the one window's geometry. The frame code does not
vanish wholesale.

**Four failures, and two of them were the kind that ship.**

1. **A splice that would have compiled.** The first version cut everything from the
   `aucmd_win[]` search through `curbuf = buf;` — which also swallowed
   `aco->save_curwin_id` and `aco->save_prevwin_id`, the two fields
   `aucmd_restbuf()`'s surviving branch reads back through `win_find_by_id()`. It
   would have built cleanly and restored from uninitialised stack. A count check on
   an unrelated line is what stopped it; the cut is now two narrow splices.
2. **`drop_if` refused an `else`, correctly.** `aucmd_restbuf()`'s
   `if (aco->use_aucmd_win_idx >= 0)` has one, and deleting the `if` alone would
   orphan it. `fold_never` is the helper for that shape.
3. **Three places managed the table without reading it** — `autocmd_init()`, whose
   whole body was a `memset` of it, and two loops in `screenalloc()` freeing and
   reallocating line sizes for windows that can no longer exist. The `can_cindent`
   shape from phase 64, found by the post-condition grep rather than by any warning.
4. **The must-go list contradicted the phase's own header.** It demanded
   `last_status_rec` reach zero mentions while the header said `last_status()`
   stays. The check was wrong, not the tree.

### The delta

**None.** No key, command or option changes. Measured: 96,636 → **94,122 lines**,
the largest single phase since the early cuts.

## Phase 69 — one file argument, and no argument list

**The order is the opposite of the obvious one, and the first attempt at this
phase proved why.** That attempt imposed buffer reuse inside `buflist_new()` and
deleted the argument-list call that reaches it — and that call is **the only thing
that names the first buffer**. `open_buffer()` reads through
`readfile(curbuf->b_ffname, …)`, so with no name it read nothing: the buffer came
up empty, every edit was a silent no-op, and `:wq` wrote the original bytes back.
It compiled cleanly and passed two of its three probes. It was dropped whole.

So this phase limits the command line **first** and leaves the naming path exactly
as it is:

- **One file argument.** A second non-option argument is
  `mainerr(ME_TOO_MANY_ARGS)`, which is what vim already answers for a second `-`.
  One file means one entry, which is what makes the list pointless rather than
  merely unused.
- **The name still goes through `buflist_add()`.** `curbuf` exists and is unnamed
  and empty when `command_line_scan()` runs — `main()` calls `common_init_2()`,
  which calls `win_alloc_first()`, before the scan — so `buflist_new()` reuses it
  and sets `b_ffname`, exactly as before. Only the *list* around that call goes.
- **The argument list.** `:next` and `:previous` point at `ex_ni`; the other 21
  argument commands already did. Gone with them: `ex_next`, `ex_previous`,
  `do_argfile`, `do_arglist`, `arglist_del_files`, `alist_set`, `alist_clear`,
  `alist_add`, `alist_add_list`, `alist_check_arg_idx`, `alist_name`,
  `check_arg_idx`, `editing_arg_idx`, `arg_all`, `check_arglist_locked`,
  `arg_had_last`, `global_alist`, `alist_T`, `aentry_T`, `w_alist`, `w_arg_idx`,
  `w_arg_idx_invalid` and `mparm_T.fname`.

**What folds because the count is always one**: `check_more()`, whose "N more
files to edit" refusal can never fire; `append_arg_number()`, the `(N of M)`
suffix; `##` in a file-name modifier, which had every argument to expand and now
has none; and the seven `ADDR_ARGUMENTS` arms of Ex range parsing.

**`ADDR_ARGUMENTS`'s labels stay, and its bodies go.** Deleting the labels earns
seven *"enumeration value not handled in switch"* warnings — those switches
enumerate `ADDR_*` exhaustively — which is what the sweep kept reporting as "left
alone 7" while never converging. Each arm gets a constant body instead.

### The delta

**None, and that was measured rather than assumed.** `:next` and `:previous` were
declared as moving and did not. An `exsweep` row is `exit= left= err=`, and with
one file argument `do_argfile()` already answered *"there is only one file to
edit"* — so pointing the rows at `ex_ni` changes the message text, which the sweep
does not record, while the exit status, the files touched and stderr all stay the
same. The declaration was **narrowed** to match the measurement; widening one to
fit is what `whimdelta.sh` exists to refuse.

The probes are **load-first**: `+$` then `+s/^/LAST /` proves the buffer holds the
file's lines, which is the check the abandoned attempt lacked and needed. Then
plain editing, a second file argument refused without writing either file, `:next`
refused, and `:e` still opening a second file.

Measured: 94,122 → **93,393 lines**.

## Phase 70 — :e reloads in place, and there is no swap file

**This invariant is imposed, not proved**, and that is the difference between it and
phase 68. One window fell out of the two places a window could be created. A second
*buffer* is genuinely reachable: `curbuf_reusable()` wants an unnamed, empty buffer,
so once the first file is named, `:e other` allocates a new `buf_T` and switches to
it. Measured on q69: `:e h2.txt` then `+wq h1.txt` writes **h2**.

So `do_ecmd()` is made to reuse the one buffer:

- the `other_file` branch renames `curbuf` with `setfname()` instead of calling
  `buflist_new()`, sets `oldbuf = FALSE`, and falls through;
- the reload path below it — `u_sync()`, `u_savecommon()`,
  `buf_freeall(curbuf, BFA_KEEP_UNDO)`, then `open_buffer(… READ_KEEP_UNDO)` —
  **already is** "wipe and re-read in place". Its gate widens from
  `!other_file && !oldbuf` to `!oldbuf`;
- the whole `if (buf != curbuf)` block goes: BufLeave, `buf_copy_options`, `u_sync`,
  `close_buffer(DOBUF_WIPE)`, the `auto_buf` dance, the `curwin->w_buffer` swap and
  `get_winopts`. It is removed by **brace matching**, not by matching its body —
  the body is long and macro-expanded, and three runs of an abandoned phase died on
  patterns transcribed from truncated views of exactly such lines.

**Order: `fname2fnum()` first.** It called `buflist_new(name, p, 1, 0)` to give a
file mark's file a buffer, and once reuse is unconditional that call would wipe the
buffer being edited. It is **folded to an empty body, not removed** —
`getmark_buf_fnum()` still calls it, and the file marks are a separate cut. An empty
shell with a live caller is the fold, not a leftover, so the check asserts that its
body can no longer reach `buflist_new()` rather than that the symbol is gone. A
first version demanded zero mentions and failed on its own terms.

**What is lost:** the state of the file you leave — its undo history and its marks.
`:e`, `:e!` and `:wq` keep working, on one buffer.

### No swap file, ever — not even one left from another age

Swap files are already never *written* here: `findswapname`, `p_swf`,
`swapfile_info`, `swapfile_unchanged`, `ml_recover` and `ml_sync_all` went with the
recovery phase, `mf_open()` is the in-memory memfile, and `ml_open_file()` had been
reduced to a single `b_may_swap = FALSE`.

What survived was the **detection** half — the prompt for a swap file somebody else
left behind — and it was already unreachable. Measured on q69 with a `.swp` sitting
beside the file: **no prompt at all**, no stderr, the edit and the write going
through in silence. This is the `can_cindent` shape again, a flag written in three
places and never once true, and the compiler cannot say so because assigning to a
static counts as using it.

So the whole surface goes together: `swap_exists_action`, the three `SEA_*` actions,
`handle_swap_exists()`, `check_swap_exists_action()`, `check_need_swap()`,
`ml_open_file()` and the `b_may_swap` field, with the `SEA_DIALOG` setters in
`do_ecmd`, `read_stdin` and `create_windows` and the three `SEA_QUIT` tests that
could never fire.

One of those tests is worth naming because it is not where it looks like it should
be: the *first change to a buffer* used to open its swap file, and that test lives in
`changed()`, not in `buf_write()`. Writing the host function from memory got it
wrong, and reading line 8756 got it right.

The three enums the sweep reports as "every constant is dead and the type is in use,
which cannot be expressed" are **inherited, not made here** — pristine q69 reports
the same three.

### The delta

**None.** `:e` prints nothing to stderr, and an `exsweep` row is `exit= left= err=`
— the same reason `:next` did not move in phase 69. Removing the swap surface moves
nothing either, for the same reason and one more: the prompt it removes was already
never reached. The probes are load-first and **quote-free**: `+$` then `+s/^/LAST /`
proves the buffer holds the file; then `:e h2.txt` leaving h1 untouched and writing
h2; plain editing; and `:e!` discarding an unwritten change. No probe key contains
`'`, which is what made an abandoned phase's mark probes measure nothing three times
over.

Measured: 93,393 → **93,127 lines**.

## Phase 71 — one buffer, structurally

**The invariant was already true; this phase removes the machinery that pretended
otherwise.** Phase 69 allowed at most one file argument, phase 70 made `:e` reuse the
one buffer, and every buffer Ex command had been retired long before that — all 24
rows (`:buffer`, `:buffers`/`:ls`/`:files`, `:bnext`, `:bprevious`, `:bNext`,
`:bfirst`, `:blast`, `:brewind`, `:bmodified`, `:bdelete`, `:bunload`, `:bwipeout`,
`:bufdo`, `:ball`, `:badd`, `:balt`) already read `ex_ni`, and `do_buffer`,
`do_bufdel`, `ex_buffer`, `ex_bufdo` and `ex_listdo` do not exist. So nothing can
make a second buffer: `win_alloc_first()` makes the one buffer at startup, *before*
`command_line_scan()`, and `buflist_add()` then names that same buffer through
`BLN_CURBUF`.

`buf_valid()` becoming `return buf == curbuf;` is the keystone — it makes
`set_curbuf()`'s `enter_buffer(lastbuf)` fallback unreachable, and the rest of that
function's other-buffer handling with it.

### Three things named b_next are not the buffer list

A regex over the name would gut the editor, so every edit is scoped by function:

- `buffblock_T.b_next` — the typeahead and redo chain: `bh_first`, `redobuff`,
  `old_redobuff`, `readbuf1`, `readbuf2`. About thirty sites.
- `free_buffer()` — `buf->b_next = au_pending_free_buf`, a free list.
- `buf_T.b_next`/`b_prev` — **this** is the buffer list, and only this.

`au_pending_free_buf` turned out to be written in two places and **read in none**:
nothing ever drained that chain, so the `autocmd_busy` branch leaked the buffer and
always had. It goes with the field it linked through, and `free_buffer()` now always
frees immediately.

### break binds to the loop, not to the braces

Folding `for ((buf) = firstbuf; …)` into `buf = curbuf;` rebinds any `break` or
`continue` in the body to whatever loop encloses it next — and brace depth has
nothing to do with which statements those are. `getout()`'s `break` sits two `if`s
deep and still bound to the walk; `buflist_findpat()`'s body has a `break` **and** a
`continue` that bind to the walk while a third `break` correctly belongs to an inner
window loop.

The first version folded both anyway. `getout()` failed to compile, which is the
cheap outcome. `buflist_findpat()` **compiled fine and changed behaviour** — its two
statements silently rebound to the enclosing `for (;;)` retry loop — and sat
undetected through three dry runs. So `fold_walk()` now refuses a body whose
`break`/`continue` is not inside a nested loop or switch of its own, and the two
functions are rewritten rather than folded. An earlier version of that guard tested
brace depth and would have passed `getout()`; depth is the wrong question.

With one buffer `buflist_findpat()` has nothing to retry — one candidate, so the
"more than one match" (`-2`) arm is unreachable by construction.

### What stays

`buf_hashtab` and `buflist_findnr()`, because five live callers still look a buffer
up by number: `eval_vars`, `setmark_pos`, `check_changed_any`, `buflist_nr2name` and
`buflist_getfile`. Collapsing that to a `curbuf` test is a separate step.
`DOBUF_WIPE_REUSE` keeps its enum and the two tests that name it — no caller ever
passes it, which is what made `close_buffer()`'s wipe splice unreachable; that splice
was guarded by `(b_prev != NULL || b_next != NULL)`, already false with one buffer.

### The delta

**None**, and `whimdelta.sh` confirms it. The buffer commands were already `ex_ni`,
so no `exsweep` row can move.

**A probe that cannot fail proves nothing, again.** The buffer-local mapping probe
was written `+normal! Q` — and `normal!` suppresses mappings *by definition*, so it
could never fire on any build. Calibrated against q70: `x` with the bang, `x!`
without it, and the phase-71 build gives `x!` too. The bang is gone and the comment
says not to put it back. It also corrected a belief: that walk is
`check_map_keycodes()`, which feeds `add_termcap_entry()`, **not** mapping lookup —
a mapping is found through `curbuf->b_maphash[]`, which never touches the list.

Measured: 93,127 → **92,749 lines**.

## Phase 72 — one window, one tabpage, structurally

**The invariant is provable, not imposed** — the phase 68 shape rather than the phase
70 one. Windows are created in exactly one place: `win_alloc(NULL, FALSE)` from
`win_alloc_firstwin()`, whose only caller is `win_alloc_first()` at startup.
`alloc_tabpage()` is called exactly once, from the same function, and `curtab` is set
to it there. `:tabnew`, `:tabedit`, `:split`, `:new` and the rest are already
`ex_ni`, and `aucmd_win` went in phase 68. So `firstwin == lastwin == curwin` and
`first_tabpage == curtab`, always.

### Two layers, folded as a pair

Nearly every tabpage walk immediately contains

```c
for ((wp) = ((tp) == curtab) ? firstwin : (tp)->tp_firstwin; (wp); (wp) = (wp)->w_next)
```

so folding the outer walk to `tp = curtab` makes that ternary constant-fold to
`firstwin`, and folding the inner one then gives `curwin`. Cutting one layer without
the other would leave half a traversal at 15 sites. 43 walks folded: 13 nested, 14
over the tabpage list, 16 over the window list.

**The whole tabpage-switching group hangs from one gate.** `goto_tabpage_tp()`'s body
is `if (tp != curtab && leave_tabpage(…) == OK)`, never true with one tabpage.
Folding that gate orphans `leave_tabpage`, `enter_tabpage`, `valid_tabpage`,
`use_tabpage`, `win_init`, `win_copy_options` and `win_init_some`, and the sweep
removes them — taking the last readers of `tp_firstwin`, `tp_lastwin` and
`tp_prevwin` with them, 13 dead fields in all. Nothing here deletes those by name.

### The break audit, run before writing the script

Phase 71 learned that folding a walk rebinds any `break`/`continue` in its body, and
that the failure can be silent. So this phase's walk audit ran **first**, and named
its seven exceptions in advance: `aucmd_prepbuf`, `can_unload_buffer`,
`borrow_stl_vsep_hl` (two walks), `current_win_nr`, `current_tab_nr`, `getout` and
`create_windows`. Each is rewritten rather than folded, and `fold_walks()` **dies**
rather than skipping if it ever meets an eighth. It did not.

`borrow_stl_vsep_hl` lends a status line's highlight to the separator beside it; with
one window there is no beside, so the function and its two calls go.

### What the walk audit could not see

`w_next` survived the fold with six readers, because `win_ins_lines`,
`win_del_lines` and `win_do_lines` ask `wp->w_next` as a **layout question** — "is
there anything below this window on the screen" — and never traverse. No `for` head
mentions them. The zero-mention assertion is what found them.

One of those is not cosmetic: `win_rest_invalid()` no longer walks, so it
dereferences its argument unconditionally, and the two surviving
`win_rest_invalid(wp->w_next)` calls would have passed NULL and crashed.

### A blind spot the sweep does not cover

Folding `for ((tp) = first_tabpage; …)` into `tp = curtab;` leaves a variable nothing
reads, which gcc reports as `-Wunused-but-set-variable` — and `deadsweep.py` handles
`unused-variable` and `unused-function` and **nothing else**. Eight functions were
left that way, which is what the sweep's persistent "left alone 8" meant.
`check_changed_any` and `min_rows_for_all_tabpages` still read their `tp`, so theirs
stay; `changed_common` had three assignments, not one.

The first count of five came from reading a gcc list I had truncated at twenty lines.
The full list is eight.

### What stays, deliberately

The **frame layer** — `topframe`, `frame_T`, `fr_next`, `fr_child`, `fr_parent`. One
window still has one frame and sizing needs it; cutting frames is its own phase.
**`b_nwindows`**: tracing every write, it is 1 at creation, balanced `++`/`--` in
`enter_buffer` and `aucmd_restbuf`, and `--` in `close_buffer` — genuinely 0 once the
window drops the buffer, so `<= 0` and `== 0` are live "not displayed" tests. An
earlier plan folded all 20 sites to a constant 1; that would have broken buffer
release silently. **`prevwin` and `w_id`**, which `aucmd_prepbuf`/`aucmd_restbuf` and
the incsearch state use, are not list state.

### The delta

**None**, and `whimdelta.sh` confirms it. Every window and tabpage Ex command was
already `ex_ni`.

**A third probe that could not fail.** The autocommand probe was written
`+autocmd BufWritePre * normal! A-au` — and `:autocmd`, `:augroup`, `:doautocmd` and
`:doautoall` are all `ex_ni` here, so it registered nothing and fired on no build.
Calibrated against q71: the same answer as this phase gives. After `<LeftMouse>` and
`normal!`, the rule that catches all three is now written into the script:
**calibrate a new probe against the previous boundary before trusting it**, which
costs one build. The replacements — a write that completes through `getout()`'s
rewritten BUFWINLEAVE block, and an `O` that exercises the folded scroll path — were
calibrated that way and pass on q71.

Measured: 92,749 → **92,110 lines**.

## Phase 73 — one frame

**The strongest invariant of this run, and it is proved by absence.** Grepping the
whole file for a write to `fr_child`, `fr_next`, `fr_prev` or `fr_parent` returns
**nothing at all**. The frame tree is never linked:

- `alloc_clear(sizeof(frame_T))` appears exactly once, in `new_frame()`, whose only
  caller is `win_alloc_firstwin()` — itself called once, from `win_alloc_first()`;
- `new_frame()` writes `fr_layout = FR_LEAF` and `fr_win = wp`, and nothing else ever
  writes `fr_layout`;
- `win_alloc_firstwin()` sets `topframe = curwin->w_frame`;
- there is no `frame_insert`, `frame_append`, `frame_remove`, `win_split` or
  `win_split_ins` anywhere — they went with the window layout in phases 68 and 72.

So `topframe == curwin->w_frame`, `fr_layout` is `FR_LEAF` forever, and the four tree
pointers are permanently NULL. Every `FR_ROW`/`FR_COL` branch is dead, every
`fr_child` walk iterates zero times, and every `fr_parent` walk stops on its first
test.

That makes this phase a set of **body replacements** rather than a fold campaign —
fifteen of them. Each function keeps the arm that runs and loses the arms that
cannot: `frame_fixed_height`/`frame_fixed_width` → `FALSE`; the minima keep their
leaf arm; `frame_check_height`/`frame_check_width` compare one frame;
`frame_comp_pos` keeps the `fr_win != NULL` arm; `frame_new_height` keeps the
cmdheight adjustment and `win_new_height`; `frame_new_width` clears `w_vsep_width`
and calls `win_new_width`; `frame_setheight` keeps the root arm and `frame_setwidth`
returns; `frame_add_height` loses the parent walk; `last_status_rec` keeps `FR_LEAF`;
`command_height` loses the walk to the widest ancestor; `stl_connected` → `FALSE`.

**The invariant is asserted in the phase, not just in this document.** The script
greps for a write to any tree pointer and dies if it finds one, so if an upstream
ever links a frame again this fails loudly instead of producing an editor that
silently mis-sizes its one window.

**The break hazard was handled by construction.** The audit named four loops whose
`break` binds to the loop being removed — `stl_connected`, `frame_new_height`,
`frame_new_width` (twice) and `command_height` — and all four were already in the
replace-whole-body set, so nothing was folded out from under a `break`. This is the
phase 71 lesson applied ahead of time, and it is why **this phase passed its first
dry run**, the only one in this run that did.

### What is not a constant

`frame_minheight()` reads `p_wh`, `p_wmh` and `w_status_height`, and `min_rows()` and
`did_set_cmdheight()`'s clamp both depend on the number it returns — so the leaf arm
keeps its arithmetic exactly and only the recursion goes. Replacing it with a literal
would silently change what `:set cmdheight=` accepts, which no probe here would have
caught. A post-condition asserts `p_wh` and `p_wmh` still appear in its body.

`fr_width` and `fr_height` stay: they are live layout state, read by `win_do_lines`,
`screen_ins_lines`, `screen_del_lines`, `redraw_block`, `screen_line`, `win_line` and
`did_set_cmdheight`. The **fields** stay; only the tree goes.

`frame_fixed_height`, `frame_fixed_width`, `frame_minwidth` and `frame_fix_height`
fall out by cascade once their callers' `wfh`/`wfw` loops vanish, and `FR_ROW` and
`FR_COL` lose every reader. Nothing deletes those by name.

### The delta

**None**, and `whimdelta.sh` confirms it. Every splitting and resizing Ex command is
already `ex_ni`, and `:set cmdheight=` keeps its accepted range because
`frame_minheight` kept its arithmetic.

Two new probes drive the rewritten bodies — `:set cmdheight=2` through
`did_set_cmdheight` → `command_height` → `frame_add_height`, and `:set laststatus=2`
through `last_status` → `last_status_rec` — and **both were calibrated against q72
before being trusted**, which is the rule three earlier probes in this run were
written in violation of.

Measured: 92,110 → **91,329 lines**.

## Phase 74 — no file marks

**This is the phase that was abandoned as 70**, and the difference between the two
attempts is the whole lesson.

The first attempt died three times on patterns transcribed from *truncated views* —
`ex_delmarks`' `|| to < from` tail, `clrallmarks`' `static int i = -1` guard over
`26 + 1` (not `26 + EXTRA_MARKS`, which is what I kept writing), and four adjust
sites I had never enumerated. Worse, its probes **measured nothing**: a mark name
like `'A` carries a quote, the shell quoting broke, and the key was never pressed —
so three runs produced no evidence either way. It was dropped on instruction.

This time every site was read with `cat -A` first, **every anchor was pre-flighted
against pristine q73 and came back ALL OK before the script ran**, and both probes
were calibrated on q73. It passed its first dry run.

### What goes

`namedfm[26 + EXTRA_MARKS]` — which is **both** the uppercase A–Z file marks and the
numbered 0–9 marks. One array, one set of code paths, so they cannot be separated;
and with viminfo long gone nothing could ever set the numbered ones anyway, so they
were a store no code could write.

`getmark_buf_fnum`'s A–Z/0–9 arm was the **only** caller of `buflist_getfile()` and of
`fname2fnum()` — the latter already an empty body from phase 70, folded there
precisely because *"the file marks are a separate cut"*. Removing the arm leaves
`posp` NULL, which `check_mark()` already reports as E20, exactly as an unset
lowercase mark does.

The sweep then took `buflist_nr2name`, `fmarks_check_one` and `getfile` by cascade.

### What stays

The lowercase marks `a`–`z` in `buf->b_namedm[]`, and every special mark — `'`,
`` ` ``, `"`, `^`, `.`, `[`, `]`, `<`, `>` — none of which touch `namedfm`. `:marks`
still lists what is left and `:delmarks` still clears it; uppercase and digits now
fall to "invalid argument".

**`fmark_T` stays**: `struct taggy` embeds it, so the tag stack depends on it. Only
`xfmark_T`, which exists to bolt a filename onto a mark, goes.

**`do_join()` has a parameter named `setmark`**, so every edit is scoped by function.
An unscoped pattern or a global rename would have corrupted it — the `b_next` lesson
from phase 71 wearing a different hat.

### The delta

**None**, and `whimdelta.sh` confirms it.

The probes are worth stating because this is where the first attempt failed. The
**discriminator** is a pair: on q73 a lowercase mark gives `K1|K2-kept|K3|` and an
uppercase mark gives `U1-up|U2|U3|`; after the cut lowercase is unchanged and
uppercase gives `U1|U2|U3|`. The uppercase half *changes*, which is what makes it
evidence rather than decoration. The mark name is kept out of shell-metacharacter
position by double-quoting the vim text.

`:marks` and `:delmarks` are **not** discriminators — both write the file either way
and neither reaches stderr — so they are no-crash checks only, and the script says so
rather than letting a later reader mistake them for proof.

Measured: 91,329 → **90,972 lines**.

## Phase 75 — no autocommands

**Proved by absence, not inferred from the command table.**
`first_autopat[NUM_EVENTS] = { NULL }` is the **only** write to that array in the
whole file; every other mention reads it. No autocommand pattern can ever be
registered. `:autocmd`, `:augroup`, `:doautocmd`, `:doautoall` and `:noautocmd` were
already `ex_ni`, but that is the weaker argument — the array being write-once-to-NULL
is the strong one.

It follows that `apply_autocmds_group()` was already `return FALSE;`, that its three
one-line wrappers made **all ~74 dispatch sites no-ops**, that
`has_cursormovedI`/`has_textchangedI`/`has_textchangedP` were always FALSE, and that
`au_cleanup`, `au_remove_pat`, `au_del_cmd` and `aubuflocal_remove` walked a
permanently empty list.

**The invariant is asserted in the phase**, and the assertion was *proved able to
fail*: injecting a synthetic `first_autopat[0] = NULL;` makes it fire. A first
version matched `first_autopat[^\n;]*=` and reported three writes that were the `!=`
of the `has_*` predicates — it spanned the subscript and landed on the comparison. An
assertion that cries wolf is worse than none, because the temptation is to loosen it
until it passes.

### Two sites rewritten, not folded

`close_buffer()` — the label `aucmd_abort:` sat **inside** the block guarded by
`apply_autocmds(EVENT_BUFWINLEAVE, …)`, with three `goto`s targeting it, two from
outside. Folding would have orphaned them: the phase-71 break-rebinding hazard
wearing a label instead of a loop. All three gotos fold away with their conditions,
so the label goes too and `if (abort_if_last)` carries the abort directly.

`buf_write()` — the 132-line block **looks** like pure scaffolding (`aco_save_T`,
`aucmd_prepbuf`/`restbuf`, `set_bufref`, `did_cmd`) but computes `buf_ffname`,
`buf_sfname`, `buf_fname_f` and `buf_fname_s`, which are **read a hundred lines
later** to restore `ffname`/`sfname`/`fname`. Deleting it wholesale would have broken
`:w` on a renamed buffer. The scaffolding goes; the flags stay, and a `:w dst.txt`
probe guards it.

### Classification had to be done by hand

A scan got two sites **backwards**. `7712` and `7741` read
`if (!(did_cmd = apply_autocmds_exarg(...)))` — negated *with an embedded assignment*
— so they are always **TRUE** (`fold_always`), not always false. A
`startswith("if (!apply_autocmds")` test cannot see the `!(var = …)` shape, and
folding them the other way deletes the branch that runs.

`did_cmd` then needed its **two readers folded before its declaration was removed**;
doing it the other way round leaves them undeclared, which is precisely the compile
error a first version produced.

### Ordering is part of the edit

Three edits depend on an *earlier* edit having created their target, and must run
after it: `buf_write`'s scaffold and `set_termname`'s husk only take their final
shape once the blanket dispatch removal has emptied them. Pre-flighting those against
an already-swept tree confirms the shape while saying nothing about when it becomes
valid — the fix is to **replay the script's own steps** and read the result.

`set_termname`'s husk is removed by brace matching keyed on `buf = curbuf;`, not by a
literal: the literal was transcribed twice from a post-sweep tree where `deadsweep`
had already dropped the now-unused `aco_save_T aco;`. The helper **refuses** a block
that does any real work, and that refusal was demonstrated before being relied on.

### What survives, deliberately

`block_autocmds`/`unblock_autocmds` keep four caller pairs that are not autocommand
code — `set_string_option_direct_in_win`, `u_undoredo`, `win_alloc` — so both stay,
and `autocmd_blocked` stays with them as a **write-only counter** belonging to the
combined phase. Asserting any of the three reached zero would fail the phase on its
own terms.

### The delta

**None**, and `whimdelta.sh` confirms it. Nothing could fire an autocommand, so
removing the dispatch cannot change what the editor does.

A `:%!sort` probe was written and **discarded**: `!` went in phase 64, so it fails on
the baseline too and would have measured nothing. The eight that remain — load,
write, `:w name`, `:e`, `:g`, `:s`, `:m`, undo, insert — were each calibrated against
q74 first.

Gone: the `EVENT_` enum (123 enumerators), `event_tab` (127 rows), `AutoPat`,
`AutoCmd`, `AutoPatCmd_T`, `active_apc_list`, `first_autopat`, `last_autopat`,
`aucmd_prepbuf`, `aucmd_restbuf`, `aco_save_T`, `getnextac`, `auto_next_pat`,
`event_nr2name` and all four dispatch wrappers.

Measured: 90,972 → **89,804 lines**.

## Phase 76 — one regexp engine, so no retry

**Proved by a single assignment.** `prog->re_engine = BACKTRACKING_ENGINE` is the only
place `re_engine` is ever written, so the field can hold no other value — and both

```c
if (rmp->regprog->re_engine == AUTOMATIC_ENGINE && result == -1)
```

blocks, one in `vim_regexec_string` and one in `vim_regexec_multi`, are unreachable.
They exist to recompile a pattern with the backtracking engine when the automatic
choice failed; with one engine there is nothing to fall back to. `nfa_regengine` and
`regexp_engine` were already at zero — the NFA engine went in an earlier phase, and
these two blocks were what remained pointing at its corpse.

**What went with them.** `p_re` entirely: it is an **orphan option** — no row in the
table sets it, so it reads as 0 for ever — and its only uses were a `< 0 || > 2`
validation that could never fire and the save/restore inside the two dead blocks.
`AUTOMATIC_ENGINE`, which had no other reader. And `nfa_regprog_T` with `nfa_state_T`
by cascade: their only non-type mentions were the two
`((nfa_regprog_T *)rmp->regprog)->pattern` casts **inside** the dead blocks — a husk
kept alive purely by unreachable code. The sweep deleted three type definitions.

`orphanopts` independently confirms the claim: its count fell from six orphans to
five, with `p_re` gone from the list.

### The guard was proved against both failure modes

The phase asserts in-flight that `re_engine` has exactly one assignment and that it
is to `BACKTRACKING_ENGINE`. Before relying on it, it was checked three ways: it
reports one on the real file, it **fires** when a second write is injected, and it
does **not** miscount a `!=` comparison as a write — which is exactly the cry-wolf
bug that cost an iteration in phase 75, where a guard matched `name[^\n;]*=`, spanned
the subscript and landed on the comparison.

### Audited before writing, not after

Both blocks are 25 lines, carry no `break` or `continue` that would rebind, contain
no label, and are followed by no `else` — so `fold_never` takes them without any of
the hazards phases 71, 72 and 75 each ran into. No edit's target is created by an
earlier edit either, so the specific-then-blanket ordering problem does not arise.
**It passed its first dry run.**

### The delta

**None**, and `whimdelta.sh` confirms it. The blocks never ran, so removing them
cannot change a match.

The probes exercise **matching**, not editing, because a load-and-edit probe would
pass whatever happened to the regexp layer: a quantified `%s/a\+/X/g`, `:g` over a
pattern driving `vim_regexec_multi`, capture groups with back-references, a counted
non-capturing group `\%(a\|b\)\{2}` — the shape the NFA engine used to be chosen for
— and a plain search. All five were calibrated against q75 first.

Measured: 89,804 → **89,713 lines**.

## Phase 77 — no buffer-name argument matching

`do_one_cmd()` computes

```c
ni = (!(cmdidx < 0) && (cmd_func == ex_ni || cmd_func == ex_script_ni))
```

— "this command is not implemented" — and **seven** later checks consult it before
doing work. One does not: the `EX_BUFNAME` pre-dispatch block, guarded only by
`!(cmdidx < 0)`, which compiles a regexp and matches it against the buffer to turn
`:buffer foo` into a line number.

**Every command carrying `EX_BUFNAME` is `ex_ni`** — `:buffer`, `:bdelete`,
`:bunload`, `:bwipeout`, `:checktime`, `:sbuffer` and `:pbuffer`. That last one is
worth noting: an earlier hand grep found only six because `:pbuffer`'s row spells the
handler with surrounding spaces (` ex_ni `). The phase counts the rows **dynamically**
and asserts every handler is `ex_ni`, so it is right regardless of how many there are.

**The edit is a fold, not a guard.** Adding `&& !ni` would leave a block that can
still never run — dead weight wearing a condition. The condition is false for every
command that reaches it, so `fold_never` removes it outright and `buflist_findpat`
loses its only caller.

### A goto statement, not a label

The block contains `goto doend;`, and that is safe: `doend` is `do_one_cmd`'s shared
exit label with 27 gotos targeting it, so this removes a goto **statement**. The
distinction is the one that mattered for `readfile`'s `theend` in phase 75, and for
`close_buffer`'s `aucmd_abort`, where the label itself sat inside the fold and three
gotos would have been orphaned.

### What went by cascade

`buflist_findpat` (71 lines), `file_pat_to_reg_pat` (167), `buflist_match` (13) and
`fname_match` — the last reachable only through the pattern matcher and not
predicted. 306 lines against an estimate of 251. Nothing is deleted by name here;
removing the one call site orphans them all and the sweep takes them.

### The delta

**None**, and `whimdelta.sh` confirms it. `:buffer foo` already exited 1 with nothing
on stderr — `ex_ni` sets `eap->errmsg` rather than printing, and an `exsweep` row is
`exit= left= err=`. Measured on q76: exit 1, empty stderr, file written either way.
So the gain is code, not behaviour, and the phase says so rather than claiming a
user-visible fix.

`:buffer nosuchname` is therefore **not used as a discriminator** — only as a
does-not-crash check. The probes that can actually fail exercise what survives: the
load, a write, `:e` naming a file (the argument path *next to* the one removed), and
`:g` taking a pattern. All were calibrated against q76 first.

It passed its first dry run.

Measured: 89,713 → **89,407 lines**.

## Phase 78 — empty functions, write-only counters, and the window id

Three unrelated kinds of leftover, all invisible to the compiler and so to every
sweep this pipeline runs. A fourth — the constant-return predicates — was **split
out into its own phase** once the survey showed it is not one shape but several:
only about twenty of the twenty-nine sit in a foldable `if`, the rest needing
term-level or expression edits, seven fold sites carry an `else`, one needs a rewrite
for an escaping `break`, and one is a function pointer in an option table row that
must not be touched. Bundling them here would have repeated the shape that cost
phase 75 eight iterations.

**Fifteen empty functions, 49 call sites.** Each was emptied by an earlier phase and
left with its callers in place. **Every one of the 49 is a bare statement** — checked,
not assumed: none appears in an `if`, an assignment or any larger expression, so a
line removal cannot corrupt a condition. That was the trap in phase 75, where
`ins_apply_autocmds` calls were invisible to a regex anchored on `apply_autocmds`.
The phase re-checks each body is still empty before removing its calls, and counts
mentions against bare-calls-plus-prototype-plus-definition so a hidden use fails the
phase rather than the compiler.

**`nv_nop` is not among them.** It is empty by design — the `nv_cmds` row for
`KE_NOP` — and `nvidxcheck.py` requires `nv_cmd_idx[]` to stay a permutation.

**Six write-only statics**: `autocmd_blocked` (its reader went in 75),
`autocmd_no_enter`, `autocmd_no_leave`, `redrawing_for_callback`, `prevwin`,
`last_win_id`. `block_autocmds()`/`unblock_autocmds()` become empty and **stay** —
eight live call sites, one deliberately unpaired in `deathtrap()` where the process
is dying and never unblocks.

**Two that the same scan flags and that must not be touched**: `breakcheck_count` is
*read* by `if (++breakcheck_count >= BREAKCHECK_SKIP)`, and `vim_ignored` is the
deliberate sink for discarded return values kept in phase 67. A scanner counting
`++x` as a write, unable to see the read in `x = call()`, reports both as write-only.

**The window id**, one chain: with one window `curwin->w_id` is constant, so both
`if (is_state.winid != curwin->w_id)` guards in `getcmdline_int` can never fire —
matched by regex, not a literal, because they sit at different indents. Folding them
makes `winid` write-only, then `w_id`, then `last_win_id` and `LOWEST_WIN_ID`.

### The field that was not dead

`termrequest_T.tr_start` was on the Tier-1 list with **one** identifier mention — its
declaration — and removing it produced three *"excess elements in struct
initializer"* errors. `termrequest_T` is positionally initialised three times as
`{STATUS_GET, -1}`, and **that `-1` is `tr_start`**. A positional initialiser names
nothing, so counting identifiers cannot see the use — which is exactly why
`deadfields.py` exempts every field of a type that has one. The exemption was
recorded during the audit and then ignored.

`cmdarg_T.prechar` is the contrasting case and was removed safely: its only
positional initialiser is `cmdarg_T ca = { 0 };`, which supplies one value and
zero-fills the rest, so it cannot overflow. The distinction is whether the initialiser
supplies enough values to reach the field being removed.

Two assertions in this phase also had to be repaired before it would run: one still
demanded `tr_start` reach zero while another demanded it survive, and an initialiser
count used `[a-z_]+_status`, which cannot match the digit in `u7_status`.

### The delta

**None**, and `whimdelta.sh` confirms it. An empty function called or not called does
the same nothing, a counter nobody reads has no effect, and the two `winid` guards
could never fire. The inventory was produced by two independent scanners that agree
exactly on all 16 empty functions and 32 stubs.

Measured: 89,407 → **89,233 lines**.

## Phase 79 — the constant-return predicates

Twenty-eight functions whose whole body is `return <constant>;`. Each was emptied by
an earlier phase and left with its callers in place, so the editor still asks "is the
popup menu visible", "are we in a Vim9 script", "is there more than one window" — and
still branches on an answer that cannot change. No sweep can see this: at `-O0` each
is a real call and a real branch, and the code is *reachable*. Unuseful, not unused.

**The invariant is asserted, not trusted.** Step 1 reads all 28 definitions and
requires each body to be exactly `return <expected>;`, with the expected token written
out per name. If an upstream ever gives one a real body the phase fails instead of
folding a live predicate. That is phase 77's pattern, and it is the only thing between
a fold and a wrong answer.

### Four that look identical and are not

A scan for `return <single token>;` reports 32. Four of those tokens are **variables**:
`get_hislen`→`hislen`, `is_maphash_valid`→`maphash_valid`, `get_search_pat`→`mr_pattern`,
`get_text_locked_msg`→ a static message. My first classifier said *thirteen* of the 32
returned a variable — it had matched the bare tokens `0`, `1` and `NULL` against
unrelated declarations elsewhere in the file. Reading the **definitions** gives four.
Supplying the expected constant per name is what makes that error impossible to repeat
silently, and an assertion requires all four to survive.

**`did_set_number_relativenumber` is a constant and still is not touched.** Its only
two mentions are option-table rows where it appears as a *function pointer* with no
call parentheses. Folding is meaningless and deleting it would leave two rows pointing
at nothing. An assertion requires both rows intact.

### `binds_out` vetoes fold_always, not fold_never

`parse_command_modifiers`' `if (vim9script)` block contains a `break` that binds to the
enclosing `for (;;)` — the shape that made `buflist_findpat` change behaviour silently
in phase 71. But that phase **folded a walk**, keeping the body while removing the loop
around it, so the `break` rebound. `fold_never` **deletes** the body, `break` and all,
and the condition was false, so it never fired. Every `fold_always` site here was
audited separately and none contains an escaping `break`.

### Three second-order cuts, each proved in the phase

**`skip_for_popup`** is not a stub on entry — it has three returns. Once `pum_under_menu`
and `pum_visible` fold, both its guards go and it becomes `return FALSE;`. The phase
re-runs the same `const_of()` check to *prove* the collapse before using it, then takes
nine more sites.

**`may_have_range`** is a local of `do_one_cmd` with two writes. One is inside the
`if (vim9script && …)` block this phase folds; the other is that block's `else` arm,
`may_have_range = TRUE;`. After the fold it has one write, is constantly true, and both
readers fold.

**`wc`** in `option_value2string` is `long wc = 0;` whose only "write" is `&wc` passed to
`wc_use_keyname` — which never dereferences `wcp`. So **both** arms of that chain are
dead, not just the first, and it collapses to the `sprintf`. Read from the body, not
assumed; the phase asserts `wcp` is absent from it.

`need_check_timestamps`, `need_redraw` and `bom_count` each become write-only once the
stub feeding them is gone, so their tests fold and the variables sweep.

### Ordering, and the ternary that spans a line break

Specific literals run before blanket regexes **except where a blanket edit creates the
specific one's target**. Three places turn on it, and the third is the sharp one: the
address ternary in `do_one_cmd` is the rare construct in this tree that spans a line
break, so it must be replaced *before* the blanket `current_win_nr` pass — which would
otherwise rewrite one half and leave `eap->line2 = eap->addr_type == ADDR_WINDOWS`
dangling. All 74 anchors were counted against the q78 tree before the program was
written, and that pre-flight caught the one edit I had never transcribed:
`&& !at_ins_compl_key()`, which I knew about only from a truncated survey line.

**No term edit ends in whitespace.** `only_one_window() && check_changed_any` becomes
`check_changed_any` rather than stripping `only_one_window() && `, because a literal
with a trailing space lost it passing through an editor in phase 71.

### Two bugs, both in the checks rather than the edits

**An assertion that a correct edit could not satisfy.** `vim9script` was in the
zero-mention loop, but four mentions survive and none is the identifier: three
former-file banner comments and the `[CMD_vim9script]` row, where it is the command
*name* — a string literal. A bare word count cannot tell an identifier from a comment
or a string. Phase 78 made this same mistake twice in one script.

**`set -e` killed the phase on the behaviour it was checking.** The quit probes were
written `( … ); rc=$?`, and a subshell whose status is not *tested* aborts under
`set -e`. The second probe runs `:q` on a **modified** file, which exits 1 by design —
so the script died after the build with every probe unrun, and the truncated log made
it look like a clean finish. The form is `rc=0; ( … ) || rc=$?`, which is a tested
context. Phases 77 and 78 avoided this with `|| true` and never needed the status.

### The delta

**None**, and `whimdelta.sh` confirms it. Every fold removes a branch whose condition
cannot hold and every term edit removes a constantly-true conjunct or a constantly-false
disjunct. The quit path is the one place where an error would be silent rather than
fatal — `check_more()` feeds the four `ex_quit`/`ex_exit` conditions that decide whether
`getout(0)` runs, so wrong folding makes the editor refuse to quit or quit without
saving. Five quit probes calibrated on q78 guard it: `:q` refuses a modified file,
`:q!` discards, `:wq` and `:x` write and exit.

Measured: 89,233 → **88,636 lines**.

## Phase 80 — the Ex command table, cut to the commands that exist

600 rows in `enum CMD_index` and `cmdnames[]`, and **489 were `ex_ni` or
`ex_script_ni`**. Every phase that removed a command had pointed its row at the stub
and left it, under rule 3 as it then read, because a row still did one job: its
*name* decided what every abbreviation of every other name meant. The rows, and the
two-level prefix index generated from them, were kept for that alone.

**The rows go and what they were for stays.** The old lookup took the first row, in
index order, whose name started with the typed word, so a command's shortest
abbreviation was implied by every row above it. Measured on q79: deleting the 489
rows in place hands **15 prefixes** that used to reach a stub to a live command —
`:n` to `nmap`, `:o` to `omap`, `:h` to `highlight`, `:sa` to `saveas`, `:la` to
`later`, `:en` to `enew`, `:ve` to `verbose`. No live command would lose an
abbreviation or gain another's; an error would just quietly become a mapping
listing.

So each surviving row **carries its shortest abbreviation**, computed from the
600-row table before a row is touched, in the field that held the name's length —
whose one reader was the Vim9 whole-name check, dead since Phase 79. A word names a
row when it is a prefix of the name and at least that long. That makes a match
**unique**, which makes row order irrelevant, which makes the index pointless:
`cmdidxs1`, `cmdidxs2`, `command_count` and E943 go, and the lookup is a scan of 111
rows. `tools/create_cmdidxs.py --check`, which fifteen phases between 58 and 79 ran, has no
block to check in `whim-vim.c` any more and is not called from here on. Its `names()`
still reads the table for `exsweep.py`, and refuses fewer than 100 rows; a phase that
takes the table below that has to lower the floor.

**Proved rather than argued, twice.** The program models the old lookup — the index
read out of the file, its start points and all — and the new one, over all 2,538
prefixes of the 600 names. They must agree wherever the old answer survives, find
nothing wherever it did not, and no word may match two rows. Then every one of those
words, plus 94 command lines covering every address form the surviving commands
take, goes through **both binaries** — the input's, built in the background while
the edits run, and the output — comparing exit status, stderr, the file afterwards
and anything left in the directory. Words the old table sent to `:stop` or
`:suspend` are left out, as the command sweep leaves those commands out.

### What went with the rows

- **26 `CMD_` tests** of commands that no longer exist: `:wincmd`'s address type, the
  filename-escaping exceptions for `:grep`, `:make` and `:terminal`, `:new`/`:split`/
  `:sview` in `do_exedit`, `:try`, the Vim9 `:final` and `:horizontal` quirks, and
  the index's two start points `CMD_Next` and `CMD_bang`.
- **The `ni` flag** in `do_one_cmd`, which exempted a stub from the range, bang,
  count and argument checks. No row can raise it.
- **The user-command test `(int)cmdidx < 0`**: nothing assigns a negative index.
- **The `py3` and `vim9` digit rules** in `find_ex_command`: no row left starts with
  `py` or `vim`.
- **Seven address types** only stub rows used — argument list, buffers, loaded
  buffers, two for tab pages, two for quickfix — 49 case labels, 35 whole arms, and
  the buffer-offset arithmetic behind them. The program refuses to delete an arm
  that the arm above it can fall into.
- **`:if`, and with it `ea.skip`.** `:if` was a stub row that `do_one_cmd`
  special-cased to raise `if_level`, which made later commands skipped. But `:if`
  takes the rest of its line, `if_level` is reset at the end of every `do_cmdline`,
  and nothing passes `DOCMD_REPEAT`, so no command could ever run with it raised.
  `ea.skip` was already constantly false, and its nineteen readers fold.

### The delta

A removed name gives **E492 "Not an editor command"** instead of E319, with the same
exit status, and the command sweep cannot see the text. Two things can see a
difference, and both were agreed before the program was written:

- **`:if`** was accepted silently (exit 0) and is an error now (exit 1).
- **`stub|cmd`** used to run `cmd` after the stub's error, because a stub row with
  `EX_TRLBAR` split its line at the bar; an unknown name takes the whole line.
  `:buffer|%s/a/X/|w` wrote the substitution before and writes nothing now.
  `:h|…` is the control: `:help`'s row never had `EX_TRLBAR`, and it comes out the
  same.

And every removed row leaves the command sweep, which dispatches the names in the
table — so the declared list is Phase 79's plus all 489, and the program requires
that list to be exactly the stub rows.

### What the dry runs caught

All five were in the checks or my arithmetic, not the edits: a `vim9` word check
that matched the string literal `"vim9"` (which is how the digit rules were found); a
lookup span counted as 31 lines that is 33; `cutil.delete_definition` returning a
pair, not the text; a "no `sizeof(\"` left" check that matched unrelated string
lengths elsewhere in the file; and `:h|…` expected to differ, because I assumed every
stub row split at the bar without reading `:help`'s flags.

Measured: 88,636 → **87,142 lines**, the binary 1,008,424 → 955,976 bytes.

## Phase 81 — one line, one command

An Ex line could hold several commands separated by `|` and end in a `"` comment.
Both exist for scripts — a vimrc, a sourced file, a function body — and this editor
reads none. Every command it runs was typed, came from `+cmd`, or came from a
mapping's right-hand side. So a line is one command now, and `|` and `"` are ordinary
argument characters. **A newline still ends a command**: that is the rule itself, and
the newline branch of `separate_nextcmd` is kept exactly as it was.

**The machinery was small and in one place.** `separate_nextcmd` split a bar-splitting
command's argument at `|`, `"` or a newline; `check_nextcmd`, `find_nextcmd`,
`ends_excmd` and `ends_excmd2` each knew the same three characters; and a handful of
callers knew them again — `:substitute`'s tail, the trailing-characters check in
`do_one_cmd`, `:a|text`, `:|` printing the line, and the whole-line `:" comment`
with the `starts_with_colon` flag that only existed to feed it.

**Decided before it was written:**

- `a|b` — the bar is argument text. A command without `EX_EXTRA` reports E488; one
  with it takes the bar. `:map Q A|b` now maps `Q` to `A|b`.
- `a " x` — the quote is argument text too, so `:set ts=3 " x` is an error and
  `:" x` is E492.
- `\|` means nothing special: the backslash stays, so `:map Q A\|b` maps to `A\|b`.
  CTRL-V handling is unchanged.

**`EX_NOTRLCOM` stays.** Its comment meaning is gone, but it still decides whether
trailing spaces are stripped, which is what lets a mapping end in a space.

### The delta

No behaviour case, terminal row or swept command uses a bar or a comment, so the
cumulative list is phase 80's, unchanged, and `whimdelta.sh` confirms it. What moves
is probed directly: 43 cases through q80's binary and this one, comparing exit
status, stderr and what was written. Fourteen differ, each declared with its reason;
29 controls must not — `:s/a\|b/…/` and `:g/a\|c/d` (a bar inside a pattern was
never a separator), `:normal! A|x`, `:map Q A"b` (a mapping never took a comment),
CTRL-V before a bar, and two commands separated by a real newline.

One expectation in the corpus was wrong on the first run, and not the edit: the
mapping cases assumed the cursor on line 1, and `-e -s` starts on the last line.

Measured: 87,142 → **87,107 lines**. A small cut by count — the point was the
rule, and the splitter was never large.

## Phase 82 — the system headers nothing needs, and every comment

`whim-vim.c` opened with the same 41 `#include`s as `slim-vim.c`, and eighty-one
phases had taken away most of what they were for — the directory walker, the locale,
the password file, `dlopen`, `setjmp`, the maths library, `utime`, `uname`. The
object leaves 80 symbols for musl to supply, and a header that provides none of them
is a dependency on the host that buys nothing.

**The set is computed, not listed.** A header's name says what it is for, not what
this file takes from it, and musl's headers include one another. So each `#include`
is deleted in turn and the compile must stay **silent** under the sweep's flags; gcc
15 compiles C23, where an undeclared function or an unknown type is an error, so
silence means nothing the header provided was used. 28 of 41 can go on their own, in
parallel. Together they do not build — some pairs each carry what the other declares
— so they are removed one at a time, keeping each removal only while the build stays
silent.

**From the bottom, and the first dry run is why.** Walked top down, it dropped
`<string.h>` and `<stdlib.h>`, whose declarations happen to arrive through headers
further down, and kept `<wchar.h>`. The general headers come first, so walking up
from the end drops the specific ones and keeps what everything else leans on.
`<iconv.h>` stays: no iconv function is called, but `iconv_t` is still named.

**The proof is the binary, byte for byte.** A header can define a function-like
macro that shadows a function — musl's `<ctype.h>` does — and losing one would change
code silently if the prototype still came from elsewhere. So the input and output are
both built with `SOURCE_DATE_EPOCH` pinned, from the same file name, and must be
identical. They are, 955,976 bytes, which makes "no delta" a measurement.

Removed, 23: `limits.h` `sys/types.h` `dirent.h` `sys/time.h` `pwd.h` `sys/file.h`
`strings.h` `setjmp.h` `locale.h` `float.h` `math.h` `inttypes.h` `stdbool.h`
`sys/select.h` `wchar.h` `utime.h` `langinfo.h` `sys/sysinfo.h` `sys/wait.h`
`stropts.h` `sys/utsname.h` `dlfcn.h` `sys/resource.h`. Left, 18: `stdio.h` `ctype.h`
`sys/stat.h` `stdlib.h` `unistd.h` `sys/param.h` `time.h` `signal.h` `string.h`
`errno.h` `stdint.h` `wctype.h` `stdarg.h` `stddef.h` `fcntl.h` `iconv.h`
`sys/ioctl.h` `termios.h`.

### Every comment

The same phase strips every comment: the former-file banners, the seven notes, and
the lines earlier whim phases wrote to explain themselves — 314 lines, and the blank
lines around the banners that would otherwise have doubled up. `whim-vim.c` carries
code and nothing else from here, and **no later phase writes a comment into it**
(rule 5); the reasoning lives in the phase programs, this file and the commit
messages. Comments are found by a scanner that knows string and character literals,
because `"pack/*/start/*"` and `"://"` are data. Comments never reach the binary —
nothing uses `__LINE__` — so the byte-for-byte check covers this cut too, and the
paragraphing is checked separately: no run of blank lines, and the counts of blank
lines after `{` and before `}` unchanged.

Measured: 87,107 → **86,614 lines**.

### `arrowcheck.py` retired

The pty check that the arrow keys still move the cursor ran in 46 phases, from 24
on, at about **20 seconds of wall time each** — for most phases more than the
phase's own work. It guarded against one accident: the mouse phase deleting
`nv_cmds[]` rows under a precomputed index, which `tools/nvidxcheck.py` now catches
structurally, in `phasecheck.sh`, in no measurable time. One accident does not buy
a pty session per phase for ever, so the call went from every phase program and the
tool was deleted. Phase 32's own check keeps its completion half.

## Unused, and unuseful

These are different questions and only one of them has a tool.

**Unused** is what the compiler can prove: nothing reaches it. Every phase here
ends with the sweep run to a joint fixpoint, so unused code never survives a
phase, and no judgement is involved.

**Unuseful** is code that is reachable, compiles, would run, and should not be
here. No warning will ever name it. The only way to make it tractable is to
measure: `tools/coverage.sh` builds with `--coverage`, runs every harness there
is — the behaviour cases, all 600 Ex commands, the pty scenarios — and ranks
what was never entered by size.

**That list is evidence, not a verdict**, and it has at least three kinds in it:

1. **genuinely unuseful** — a feature this product's own defaults never reach;
2. **useful but unexercised** — error paths, rare modes, `vim -` reading stdin.
   A hit here is a finding about the *harness*, and arguably the more valuable
   of the two;
3. **reachable only through something already removed** — the best candidates,
   and the reason to re-run this after every phase.

Deleting from the list without deciding which kind each entry is would remove
working features and call it progress.

### What it says today

**36% of `whim-vim`'s functions are never entered** — 750 of 2,065, holding
11,235 lines. Measured after Phase 67:

```
    258  win_split_ins              the window layout, reachable only via aucmd_prepbuf
    158  win_equal_rec
    136  eval_vars                  % and # expansion in an Ex line
    117  scroll_cursor_bot
    115  op_replace                 Visual-block r
    108  op_insert                  Visual-block I and A
    101  do_more_prompt
    100  handle_csi
     98  cursor_pos_info            g CTRL-G
     98  change_indent              Insert-mode CTRL-D and CTRL-T
     94  shift_block
     91  win_close
     91  file_pat_to_reg_pat
     89  nv_zet                     the z commands
     89  internal_format            wrapping at 'textwidth'
```

**Two entries were examined in this review and deliberately not cut**, which is
worth recording so the next pass does not re-litigate them.

**The `z` commands are kind 2.** `nv_zet` (177 lines), `nv_z_get_count` (54) and
`set_leftcol` (52, called from `nv_zet` alone) would free 283 lines — but not
`scroll_cursor_top`, `scroll_cursor_halfway` or `scroll_cursor_bot`, which keep
external callers in `update_topline()` and `scroll_redraw()` and so stay whatever
happens to `z`. What would go is view positioning (`zt`, `zz`, `zb`, `z<CR>`,
`z.`, `z-`), horizontal scrolling (`zh`, `zl`, `zH`, `zL`, `zs`, `ze`, which do
nothing unless `'wrap'` is off) and `zp`/`zP`/`zy`. **Kept**: `zz` centring the
current line has no replacement among CTRL-E/CTRL-Y, CTRL-D/CTRL-U, CTRL-F/CTRL-B
or H/M/L, and a never-entered `nv_zet` is a statement about the harness, which
presses no `z`.

**The window layout is not the kind-3 candidate it looks like.** 2,162 lines
across 21 functions — `win_split_ins` (538 by its own extent), `win_equal_rec`
(314), `win_close` (196), `last_status_rec` (149), the `frame_*` family — and
`win_split()`, `win_new_tabpage()` and `make_windows()` have **zero** mentions, so
nothing user-facing can split a window. But `win_split_ins()`'s caller is
`aucmd_prepbuf()`, which is called from `open_buffer()`, `buf_write()`,
`ins_redraw()` and `set_termname()` — all live. It is reachable and never taken,
not unreachable. Cutting it means proving the autocommand window can never be
created and folding those four callers: a phase, not a sweep.

**The list is doing its job, and the way to read it is against the last
reading.** After Phase 6 it said 1,520 of 3,255 over 27,865 lines, with
`reg_equi_class` (775), `get_c_indent` (713), `do_mouse` (284) and
`modify_fname` (160) at the top. Phases 24, 27, 28 and 31 removed **all four**,
and 10,059 lines of never-entered code with them. That is what a kind-3 entry
looks like when it is acted on.

What is left at the top has changed kind. `do_window`, `op_replace`,
`op_insert` and `scroll_cursor_bot` are **kind 2** — reachable, useful, and
simply not exercised, which is a finding about the harness rather than the code;
nothing here should press CTRL-W on its behalf. The kind-3 entries are now
`set_context_in_set_cmd` and `set_context_by_cmdname`, which is command-line
completion, and `ins_compl_build_pum` at 98 lines — the tail of a completion
whose key handling had survived the stubs, and which Phase 32's second cut then
removed.

**Also measured: the harness itself.** `tools/coverage.sh` was resolving the
source path relative to the wrong directory, so `exsweep.py` exited 1 and the
`&&` chain took the pty scenarios down with it — and what came back was a
figure computed from one harness out of three: 64% never entered instead of
47%. It was lower than the previous reading, it moved in a plausible direction,
and it was wrong. The three harnesses are now run and reported separately, so a
failure says so instead of quietly shrinking the denominator.

# Part II — phases 83 to 128: an embeddable core

`whim-vim.c` at q82 is an embedded editor: one static binary that expects nothing to
have been installed for it. **Phases 83 onwards take it to what is left when the editor
stops being a program at all, and becomes a component a host program runs.**

They were a second pipeline, zero — `zero-vim.c = H(whim-vim.c)`, handed the committed
result of phase 82 and numbered from 0 — until it was folded into this one. Zero phase
*N* is phase *N*+83, and every number in this part is the one pipeline's; `zero-vim.c`
is the name the product had then, and it is `whim-vim.c` from phase 83 on. What differs
from Part I is what the phases remove and what their behaviour is measured against
(core rules 2 and 3).

**This part is iterative, and so far it has forty-six phases.** Phase 83 is where
the core begins, phase 84 is a compiler flag, phase 85 is the first cut in the source — the first
piece of *a component, not a program* — phase 86 changes no source at all: it
replaces the instrument every later phase is measured with; phase 87 removes Ex mode,
phase 88 leaves the command line as `+{command}` and `-T {term}`, and phases 89 to 96
are *no filesystem* on request: the editor loses every way to write a file, then
every way to read one, then every way to name another one to edit, then the
machinery that read the bytes — which by then nothing could reach — then the
buffer's own name, with the last three questions the core asked the filesystem on
its own initiative, then the refusal that asked whether the text had been saved,
which by then had no remedy to offer, then the option rows that reported settings
nothing read, and finally the two `FILE *` that had never been opened. After
phase 96 the core has no `open`, no `stat`, no stdio stream and no fourth
descriptor: it can read, write, close and dup fds 0, 1 and 2 and nothing else.
Phases 97 and 98 are *no musl dependencies* on request, for the half of that charter
item which is pure computation: twenty-eight libc functions — the strings and memory
blocks, the character classes, the numbers, the sort — defined in the file as local
`static` ones instead of asked of a host, which takes `nm -u` from 61 to 33 and is the
first change in the pipeline that makes the file *longer*. Phase 99 then removes the
six `#include`s that nothing names any more, eighteen directives to twelve, and it is
the first phase in any of the pipelines to change a directive count. Phase 100 is
nine lines and one symbol: `deathtrap()`'s `entered >= 3` ladder, which no build of
zero-vim could ever have reached — two deadly signals, each blocked inside its own
handler — and with it `_exit`, leaving exactly one `exit()` call in the whole file.
Phases 101, 102 and 103 are *a component, not a program* on request, and they are
`WHIM-PLAN.md` §II.4c's three steps: `main()` becomes `static int vim_main(...)` with a
launcher of its own below it; `mch_exit()`'s last `exit(r);` becomes a call through a
pointer the launcher installs, so the editor hands the process back with a number
instead of ending it; and then the signal handlers, the window size, the terminal
mode, the delay and the wait all move into a host block at the bottom of the same
file, which takes `nm -u` to 24 and leaves the core making exactly two syscalls for
itself, a `read` of fd 0 and a `write` of fd 1. Phase 104 finishes §4c's second step:
every byte the editor put on a *stream* rather than a screen goes through one callback
the launcher installs, which takes `nm -u` to **17**, removes `<stdio.h>` and leaves
`write` the only syscall the core still makes for itself. Phases 105 and 106 are the
first of the reorganisation that draws the boundary itself: 105 expands the seven
wrappers that walk a `va_list` at their 129 call sites, so that `va_start` appears
**once** in the file and the formatter becomes movable, and 106 replaces `NULL` and
`size_t` with `nullptr` and a `usize` of the core's own — two names the **language**
supplies instead of a header — with a binary that is byte for byte the one it was
handed. Phase 107 is the one phase of the last five that is not about the boundary at
all: it asks which dialect of C the core is written in, takes 139 GNU attributes to
six — deleting 113 `unused`, **21 of which were false claims about code that uses its
parameter** — and keeps the six `format`/`format_arg` that are themselves the check
`-Wformat` performs. Phases 108, 109 and 110 finish the boundary: 108 makes the two host
calls plain, a forward declaration and a direct call where there was a function pointer
the launcher installed; 109 gives the core its own `time_T`, `volatile int`, `usize`,
tagless clock struct, `MIN`/`MAX` and `__builtin_offsetof`, and nine plain libc
prototypes, **while the real headers are still above them to be cross-checked against**;
and 110 moves the eleven `#include`s below the core, so that **above them there is not
one preprocessor directive** and the first `#include` is the line between the core and
the host. `make editor.c` writes the lines above it — **76,716** of `zero-vim.c`'s
78,681 today — and they are a complete translation unit whose warnings are the
interface. Phases 111 to 115 are five separate answers to *what the core may still name
and still call*: 111 gives the elapsed-milliseconds clock to the host as one scalar,
`long musl_now_ms(void)`, and deletes the tagless `struct timeval` mirror phase 109 had
to invent because a struct could not cross the line; 112 merges vim's Unicode case map
and the musl one phase 98 vendored into **one table, and it is the union** — a core
with no C library has nothing left for `'casemap'` to choose between; 113 folds the arm
of `msg_puts_attr_len()` that reached a terminal without a screen into two lines
addressed to the host, and is where phase 104's `exit_scroll` claim was measured and
corrected; 114 vendors `abs` and `labs`, which the core **called** and which never
appeared in `nm -u` because gcc lowers both to inline arithmetic — the first
application of `WHIM-PLAN.md` §II.4c's rule that the core is optimised for transpilation
and may not depend on latent compiler behaviour; and 115 sends the wall clock across
too, `host_time()` below the boundary where `vim_time()` was above it, replacing the
`long time(long *tp);` prototype with a `static_assert` that is strictly stronger than
what it removes. Phase 116 is the second in the pipeline to change no source at all: it
replaces the one part of the recording that still asked the **environment** about the
terminal, a question whim phase 19 had already stopped the editor reading, so that all
nineteen rows stopped saying the same thing. And 117 and 118 finish what 111, 114 and 115
were each a piece of: 117 rewrites `realloc` at the two core sites that used it, because
`realloc` is the one libc function that **cannot** be vendored — to move the old
contents it needs an old size its interface does not carry — and 118 sends `malloc`,
`free` and `write` to the host, so that the core's own block of ordinary libc
declarations is **two lines**, `getpid` and `kill`. Phase 119 takes both and the block
with them, by two different routes — `getpid` is avoidable outright and `kill` crosses as
`host_raise()` — so that **the core names no libc function at all**, a claim measured on
the cut compiled to an object and not on the block. Phase 120 is a tidy: six of the
thirteen `union` keywords unite nothing with anything, five single-member and one empty,
every one a leftover of a cut already made, and the binary is the same bytes either side.
And 121 and 122 are the terminal, which is the first thing zero has taken that the
recording can see since phase 94: 121 keeps **two** of the ten built-in terminal names,
`xterm-256color` and `debug`, and declares `term-moved` — the first use of a token that
had been in the grammar since phase 83 — and 122 removes `-T {term}`, so that
`+{command}` is the whole command line and nothing outside the process can say what
terminal this is. **And 123 to 128 are the memline, which is one arc and not six phases**,
the charter's *the text later held as a tree* begun: 123 changes no source and gives the
pipeline a corpus that can see the text layer at all, because a `zero-vim` with one line
deleted from `ml_find_line()`'s descent recorded all 102 screen cases byte for byte; 41
makes `host_alloc()` a bump allocator and `host_free()` a no-op, which is the charter's
*A GARBAGE COLLECTOR IS ASSUMED FROM HERE ON* and is what makes a record per line cheap;
125 clears out the swap file's residue, four groups of written-but-never-read bookkeeping
that no tool in `tools/` can see; 126 turns a **block number into a reference**, which is
the single thing in this pipeline that buys a JVM port the most; 127 lets the **leaf** stop
being a byte arena and become an array of line records, so a line's text is its own
allocation valid for the lifetime of the process; and 128 folds the node types, so that
there are no pages, no blocks and no memfile left — and turns the arc's standing hazard,
that shrinking `PTR_EN` would silently take the root split out of the corpus, into a
`static_assert` that fails to compile.
Phases are added one at a time, each on the user's own request, and each is written into
this part, into `pipes/` and into `pipes/zero.delta` and `pipes/whim.stages` when it is
added — never in advance.

## The core's charter

The core is an **embeddable editor core**: a library-shaped piece of C that a host
links in or translates, rather than a process that owns a terminal and a disk. What
the concept is, as the user has stated it, and in no particular order of phases:

- **A component, not a program.** `main()` is demoted. What remains of the C library
  calls — the part that talks to an operating system — is a host, and the core is what
  it drives. **The shape that was settled is one file with two parts, not two files**,
  and `editor.c` is the name of the *core*, not of the launcher this bullet first gave
  it to: from phase 110 `make editor.c` is the cut at the first `#include`, the 77,678
  lines above the boundary, and the host is everything below it in the same translation
  unit. `WHIM-PLAN.md` §II.4c is where that was decided and `.claude/briefs/zero-split.md`
  is the survey of the two-file design it replaced — abandoned, and worth reading only
  for what it measured the split would have cost.
- **No musl dependencies** in `zero-vim.c` itself. Whatever the core still needs
  from the world, the host provides.
- **The screen and all visual editing stay.** This is still vim to look at and to
  type into; what goes is the editor's reach outside itself.
- **No filesystem access.** Reading and writing files is the host's business, not
  the core's.
- **The text representation is PREPARED for a move it does not make.** The move from
  lines to a structure — a tree mirroring an abstract syntax tree, with the line view
  every motion, command and redraw expects simulated on top of it — **is not this
  repository's work and never was**: it happens after transpilation, in the repository
  that receives the core. What belongs here is everything that makes that move possible
  for somebody else: preparation, simplification and canonicalization, so the thing
  handed over is a representation a porter can read rather than one they must decode.
  Earlier drafts of this bullet had the move itself happening here, and that was the
  right plan for what was known then: the boundary had not been drawn, and where the
  core ended and its host began was exactly the question the pipeline was still
  answering. Once the core turned out to name no libc function at all, the handover
  point moved, and the move went with it. **The work argued for on the old premise is
  unaffected** — every reason to simplify the memline survives the move leaving, and
  the bullet below is why. A plan that shifts as the thing it plans comes into focus is
  the process working; this line records the shift so that a reader meeting both
  versions knows which is current and why they differ.
- **A GARBAGE COLLECTOR IS ASSUMED FROM HERE ON**, and it is assumed precisely so the
  core may stop earning its memory. The target is a JVM, where allocation is cheap and
  freeing is somebody else's problem, so the core is free to allocate finely — a record
  per line rather than a byte arena per page — and simply not free it. No real collector
  is built: `host_alloc` becomes a bump allocator in the host with enough arena for the
  test suite and `host_free` returns without doing anything, which is a change entirely
  below the boundary and touches no core line. **This is what makes the memline
  simplification cheap rather than clever.** The page arena exists for exactly one
  reason — to avoid a `malloc` per line — and once that reason is gone the arena, its
  fourteen interior pointers, its thirty-four `db_index` subscripts, its stolen flag bit
  and its three `offsetof` all go with it, by having nothing left to measure.
- **The memline page is the worst of what a JVM cannot express**, and it is the thing
  the two bullets above are aimed at: the page is a `struct data_block` whose last
  member is
  `unsigned db_index[1]` indexed to the block's line count, whose entries are byte
  offsets **into the same block** read back as fourteen interior pointers of the shape
  `(char_u *)dp + start`, whose top bit is stolen as the `DB_MARKED` flag, and whose
  two bytes of padding after `db_id` are load-bearing — `offsetof(DATA_BL, db_index)` is
  24 of a 32-byte struct and the page-count arithmetic uses it. `WHIM-PLAN.md` §II.4d
  states it and names the one other construct of the same shape. **And nothing
  constrains what replaces it**, which is worth knowing before the move is designed:
  the memfile is purely in memory — `mf_open()` takes no name, sets `mf_page_size` from
  a constant and hands out pages `alloc()` returns, `mf_sync()` clears the dirty flag
  and returns FAIL, and `mf_write`, `mf_read`, `mf_release`, `mf_fd` and `ml_recover`
  have **0 mentions** in `zero-vim.c` — so `DATA_BL` is an internal layout with no
  compatibility constraint on it and not a disk format any more.
- **`zero-vim.c` stays pure C without a preprocessor** — it inherited 18 directives
  from `whim-vim.c` and is down to **eleven**, every one an `#include` of a system
  header, and **since phase 110 not one of them is above the boundary**: the core, the
  77,678 lines `make editor.c` writes, has **no preprocessor syntax at all**, and
  **no phase adds a `#define`, a conditional or an `#include`** — because a
  following repository transpiles it to the JVM, and every construct in the file is one
  that translation has to understand. **A phase may remove one**, and phase 99 is the
  first that did, phase 104 the second, taking `<stdio.h>` with the seven symbols it
  freed. That permission is stated here because the old reading — a count the
  pipeline preserves — is exactly why phase 96 measured that removing three of them was
  free and declined, writing *"the count stays 18"* into its own program. Nothing is
  ever added back: the eleven reach `select`, `gettimeofday`, `fd_set` and `struct
  timeval` only through musl's own `sys/param.h` → `sys/resource.h` → `sys/time.h` →
  `sys/select.h`, which is recorded as a fragility rather than repaired with three more
  directives. **The `*_MAX` are no longer among them**: phase 110 made all twelve
  header-supplied constants the core used enumerators of its own, each one asserted
  against the header from **below** the boundary.

None of that is done by phase 83, and none of it is a commitment to an order. Each
item becomes a phase, or several, when one is asked for.

## What is measured from phase 83 on

The same two numbers as Part I, reported by `make score` beside slim-vim: **bytes to
store** and **libc symbols still referenced**. From phase 83 the second is the direct
measure of the charter — "no musl dependencies" is that count
reaching the set the launcher, and not the core, supplies.

## The core's rules

Cited as *core rule N*; Part I's rules still hold.

1. **Removal is computed, not listed.** Cut the entry points and let the sweep find
   what becomes unreachable (Part I, *The sweep*). The same six kinds, the
   same tools.
2. **Every phase states its delta, in advance, as a check.** `pipes/zero.delta` is
   the list — an Ex command by name, `case:` for a screen case, `argv:` for a
   command line, `term-moved`, `pty-moved`, and the two dimensions `screen-moved`
   and `stderr-moved` — in `pipes/whim.delta`'s grammar, and
   `tools/zerodelta.sh --phase N` shows exactly that set moved and no more. "Some
   cases differ" is not a check, and neither is a dimension declared that nothing
   touched: `tools/zcompare.py` refuses a `-moved` token whose dimension did not
   move.
3. **The delta is from q82, not from slim-vim, and it is cumulative.** From phase 83
   behaviour is compared with `.reference/zero-baselines`, which phase 83 records from
   the tree it is handed — q82's `whim-vim.c` built with the compile line q82 carries.
   Everything phases 0 to 82 removed is therefore already in them, `pipes/zero.delta`
   starts empty at 83, and the lines up to phase N are the whole difference from q82 at
   N, as `pipes/whim.delta`'s are from slim. A phase declares what *it* changes, and
   every phase after it is held to that line too. **This is the one place the pipeline
   records baselines from its own output**, and it is not the mistake `CLAUDE.md` warns
   about, a pipeline re-recording from its own current binary and agreeing by
   construction: nothing from phase 83 on can reach q82, and phase 83 also proves the
   recording is q82's (see *Phase 83*). The instrument is another one because an editor
   with no file to write cannot be measured by Part I's (`tools/zrecord.sh`, phase 86).
4. **The product is `whim-vim.c`, produced from the committed `slim-vim.c`** by one pass
   of every phase, as Part I's rule 4 says. There was a `zero-vim.c`, produced from a
   committed `whim-vim.c` and chained to it by `whim.sha`; both went with the second
   pipeline, and the `whim-vim.c` committed now is what `zero-vim.c` was.
5. **Phases are programs, not agents.** A phase is `pipes/whim<N>.sh`, or an edit and a
   check, `pipes/whim<N>-edit.sh` and `pipes/whim<N>-check.sh`, whose bodies are
   `internal/edit/whim<N>.go` and `internal/check/whim<N>.go`, run by
   `tools/phaserun.sh` in stages. From phase 87 the split phases run in **each** stages:
   every edit swept on its own, then every check at once, each on exactly its own tree
   (`pipes/whim.stages`).
6. **Stages and packages are declared and checked.** `pipes/whim.stages` holds the phase
   list (`phases`), the schedule (`stage`, `need`, `apart`, read by `tools/stages.sh`)
   and the concept view (`package`, `uses`, read by `tools/packages.sh`). `make
   whim-verify` and `make whim-tip` run `tools/packages.sh whim --check` first.
7. **The product carries no comments.** Phase 82 removed the last, and no phase from 83
   on writes one.
8. **The compile line from phase 84 on is `gcc -O0 -fno-stack-protector -static -no-pie
   -s`.** `-no-pie` is what phase 83 introduces: phases 0 to 82 keep `-O0 -static -s`, a
   static-PIE, and from 83 the binary is an ordinary static executable — `readelf -h`
   says `EXEC`, with no `INTERP`, no dynamic section and no relocations — because a core
   with nothing left to relocate is one a host can place without a loader. Every further
   flag is **a phase of its own**, and a phase changes the flags by editing the
   boundary's `whim/Makefile`, never `tools/templates/zero.mk`, which phase 83 writes and
   which is in its key. `-fno-stack-protector` is phase 84. `whim.mk` states the result
   once more as `WHIMCFLAGS` and `WHIMLDFLAGS`, and `whim-pass` refuses when they differ.
9. **`tools/` is shared, and gated.** A change to a tool a phase names must leave `make
   whim-verify` (every stage boundary reproduces) passing, and moves the key of every
   unit that names it: `tools/implhash.sh` for every unit and every edit is the account
   of what it moves. Prefer a tool only phases from 83 on name (`tools/zerodelta.sh`) or
   a value in `tools/pipeline.sh` to an edit of a tool every key reads, and **never write
   such a tool's path into `tools/phaserun.sh`**: every edit's key reads what that file
   names.

## Phases 83 to 128 as they stand

**This section is the whole of phases 83 to 128 read across, and it lives here
rather than in `CLAUDE.md` because it is 910 lines of one pipeline's history in a
file that is loaded into every session.** What `CLAUDE.md` keeps is the summary
and a pointer to this heading; what is here is the phase-by-phase account, the
symbol accounting, the declared-delta taxonomy, the `apart` and `need`
derivations and the instrument's own history. Every sentence is as it stood in
`CLAUDE.md`, and three cross-references that named a section of that file now say
so; nothing else was edited in the move.

**Phases 83 to 128 are, so far, a seed, a flag,
twenty cuts, three instruments, two demotions, a host block, a variadic collapse, a
dialect phase, two tidies, a case-table merge, a vendoring, two clock phases, two
allocation rewrites, two hand-overs to the host, the four that draw the boundary and the
three that make the text a tree — the
filesystem work is finished, the terminal is down to two names it can describe and no
way to be told which, **the memline has no pages, no blocks and no memfile left**, the
libc
that is pure computation is
inside the file, seven of the eighteen `#include`s are gone, the core has no `exit()`
call at all — it asks the host to end the process and the launcher at the bottom
of the same file returns the status out of `main()` — it installs no signal handler,
sets no terminal mode, runs no `select`, asks the kernel nothing about a window,
writes to no stream, **reads neither clock for itself** — the elapsed milliseconds
and the wall time both cross the boundary as `long` — and **allocates, frees and writes
nothing for itself either** — and since phase 124 **freeing is free**, `host_alloc()`
being a bump allocator into a 1 GiB arena and `host_free()` a function that returns —
`va_start` appears once in
the whole file, `NULL` and `size_t`
are gone in favour of two names the language supplies, and **the eleven `#include`s are
no longer at the top**: they sit at line 76,718 and the first of them is the line
between the core and the host. **And since phase 119 the core names no libc function at
all**: the block of ordinary, non-`static` declarations it kept above that line is empty
and gone, every outward call is a `musl_` or a `host_`, and the claim is checked by
compiling the cut to an object — 18 undefined names, every one of them defined below the
boundary in the same file.
`WHIM-GOAL.md` states what it is for — an embeddable editor core that keeps the
screen and all visual editing and loses the filesystem, with `main()` demoted to a
host launcher and the text later held as a tree — and has forty-six phases:
`pipes/whim83.sh`, the seed; `pipes/whim84.sh`, which adds `-fno-stack-protector`;
`pipes/whim85-edit.sh` with `pipes/whim85-check.sh`, the first source cut — the two
"not to a terminal" warnings, the two-second pause after them and `--ttyfail`;
`pipes/whim86.sh`, which changes no source at all and replaces the instrument
(below), so q86's tree and q85's have the same digest; `pipes/whim87-edit.sh` with
`pipes/whim87-check.sh`, which removes Ex mode, silent mode and the `-e -E -s -v`
options; `pipes/whim88-edit.sh` with `pipes/whim88-check.sh`, which leaves the
command line as `+{command}` and `-T {term}` — the file argument, the bare `-` and
`--` all become `mainerr(ME_UNKNOWN_OPTION)`; and `pipes/whim89-edit.sh` with
`pipes/whim89-check.sh`, which takes every way to write a file — the six Ex commands
`:write :wq :xit :exit :update :saveas`, four anchors and nineteen functions the
sweep finds, `ZZ` becoming `q!`; and `pipes/whim90-edit.sh` with
`pipes/whim90-check.sh`, which takes the way to read one — `:read` and its `:r !cmd`
arm, three anchors, six functions the sweep finds and one fold no tool could make,
the `exarg_T.usefilter` field that nothing writes once both `:w !` and `:r !` are
gone; and `pipes/whim91-edit.sh` with `pipes/whim91-check.sh`, which takes every way to
name another file to edit — the five Ex commands `:edit :enew :ex :visual :view`,
which are one handler, and the `gf gF [f ]f` keys, which are **arms** inside two
surviving handlers and not `nv_cmds[]` rows, six anchors and seventeen functions the
sweep finds; and `pipes/whim92-edit.sh` with `pipes/whim92-check.sh`, which takes the
machinery under all of those — `readfile()`, `read_buffer()` and the message layer
that reported what had been read, four anchors all inside `open_buffer()` and sixteen
functions the sweep finds; and `pipes/whim93-edit.sh` with `pipes/whim93-check.sh`,
which takes the buffer's **name** — `:file`, `buflist_new()`'s two name parameters,
sixteen folds of `b_ffname`/`b_sfname`/`b_fname`, and three further folds that free
the last three questions the core asked the filesystem — seven parts and **sixty**
functions the sweep finds, the most any zero phase has handed it; and
`pipes/whim94-edit.sh` with `pipes/whim94-check.sh`, which takes the last thing the
filesystem left behind — the **refusal**, `E37: No write since last change`, which has
had no remedy to offer since phase 89 took every `:write` — as ONE fold of `ex_quit`'s
test, and sixteen functions the sweep finds, eleven of them the whole
switch-buffer/switch-window island that hung off `check_changed_any()`'s tail and that
no plan foresaw; and `pipes/whim95-edit.sh` with `pipes/whim95-check.sh`, **the
options nothing reads** — six `options[]` rows of a *computed* seven whose global has
no reader left, `'fsync' 'prompt' 'readonly' 'undoreload' 'write' 'writeany'`, with
`'readonly'`'s `W10` warning, its one-second pause and its two `[RO]` indicators;
and `pipes/whim96-edit.sh` with `pipes/whim96-check.sh`, **no `FILE *` that is never
opened** — `scriptin[NSCRIPT]` and `redir_fd`, two `static FILE *` that nothing has
ever opened in any build of zero-vim, `ui_write()`'s `console` parameter, and the
five functions the sweep finds under them; `pipes/whim97-edit.sh` with
`pipes/whim97-check.sh` and `pipes/whim98-edit.sh` with `pipes/whim98-check.sh`, **the
libc that is pure computation**, defined in the file as local `static musl_*`
functions — the sixteen `mem*`/`str*` of `<string.h>`, with `sprintf` moved onto the
editor's own `vim_snprintf` instead, then the character classes, the two `ato*`,
`qsort` and `bsearch`; and `pipes/whim99-edit.sh` with
`pipes/whim99-check.sh`, **the includes nothing names** — six of eighteen,
`<sys/stat.h>` `<fcntl.h>` `<iconv.h>` dead since before the pipeline and `<string.h>`
`<ctype.h>` `<wctype.h>` dead since phase 98, with the `stat_T` typedef no sweep could
take; and `pipes/whim100-edit.sh` with `pipes/whim100-check.sh`, **the deadly ladder
that cannot run** — the nine lines of `deathtrap()`'s `entered >= 3` arm,
`reset_signals()`, `_exit(8)` and `exit(7)`, which no build of zero-vim could ever
reach; and `pipes/whim101-edit.sh` with `pipes/whim101-check.sh`, **`main()` demoted to
`vim_main()`** — `static`, with a six-line launcher appended below it, both still in the
one file; and `pipes/whim102-edit.sh` with `pipes/whim102-check.sh`, **the core can no
longer stop the process** — `mch_exit()`'s `exit(r);` becomes `vim_host_exit(r);`
through a pointer the launcher installs, and the launcher lands on `__builtin_setjmp`
and returns the status; and `pipes/whim103-edit.sh` with `pipes/whim103-check.sh`, **the
signals and the terminal are the host's** — the five signal handlers, `mch_settmode`'s
three-valued mode, `mch_delay`'s sleep, `RealWaitForChar`'s `select`, `mch_get_shellsize`
and all three `isatty()` calls move into a 229-line `host_*`/`musl_*` block at the
bottom of the same file, the resize and the external stop and the interrupt arrive as
**bytes** in the input stream, and `fill_input_buf`'s `close(0); dup(2)` arm goes; and
`pipes/whim104-edit.sh` with `pipes/whim104-check.sh`, **the messages are the editor's and
the writing is the host's** — twenty output statements in five functions become eight
calls through one `vim_host_message(msg, len, err)` the launcher installs, with
`<stdio.h>` and seven symbols going with them; and `pipes/whim105-edit.sh` with
`pipes/whim105-check.sh`, **the variadic collapse** — the seven wrappers that walk a
`va_list` expanded at their 129 call sites into `vim_snprintf` plus the tail each
already had, five helpers against seven deleted definitions, so `va_start` appears
**once**; and `pipes/whim106-edit.sh` with `pipes/whim106-check.sh`, **`nullptr` and
`usize`** — `NULL` 2,555 → 3 and `size_t` 437 → 0, two names the language supplies
instead of a header, on a **byte-identical binary**; `pipes/whim107-edit.sh` with
`pipes/whim107-check.sh`, **the attributes** — 139 GNU `__attribute__` to six, the 113
`unused` deleted (**21 of them false**, marking a parameter the code reads), the 20
`fallthrough` respelled as the C23 `[[fallthrough]]`, and the three `format` and three
`format_arg` kept because they *are* the check `-Wformat` performs; `pipes/whim108-edit.sh`
with `pipes/whim108-check.sh`, **the plain host calls** — the two function pointers the
launcher installed become a forward declaration and a direct call, so `vim_main(int argc,
char **argv)` is phase 101's signature again; `pipes/whim109-edit.sh` with
`pipes/whim109-check.sh`, **the header types and macros the core can own** — `time_t`,
`sig_atomic_t`, `uintptr_t`, `struct timeval`, `MIN`, `MAX` and `offsetof` become the
core's own and nine libc prototypes are written out, **while the headers are still above
them to be cross-checked against**; and `pipes/whim110-edit.sh` with
`pipes/whim110-check.sh`, **the move** — the eleven `#include`s go below the core, twelve
header-supplied constants become enumerators asserted from below, and the formatter's
private island follows the four `va_list` functions down; and `pipes/whim111-edit.sh`
with `pipes/whim111-check.sh`, **the scalar clock** — `long musl_now_ms(void)` replaces
`void musl_gettimeofday(long *, long *)` and takes `elapsed_T`, `elapsed()` and the
out-parameter pair with it, **so no host call's shape is decided any more by a type the
core cannot name**; and `pipes/whim112-edit.sh` with `pipes/whim112-check.sh`, **the case
tables become one, and it is the union** — vim's `toUpper[]`/`toLower[]` and the
`musl_to*[]` phase 98 vendored disagreed at 97 upper and 96 lower codepoints, vim's
newer by ninety-six and musl's knowing `ß → ẞ` alone, and a core with no C library has
nothing for `'casemap'` to choose between; and `pipes/whim113-edit.sh` with
`pipes/whim113-check.sh`, **the message fold** — `msg_puts_attr_len()`'s never-taken arm
becomes one `host_message()` call, with `msg_puts_printf()` and `vim_strlen_maxlen()`
going, and **two folds measured and declined**; and `pipes/whim114-edit.sh` with
`pipes/whim114-check.sh`, **`abs` and `labs`** — called by the core, never in `nm -u`
because gcc lowers both to inline arithmetic, and vendored so that the core stops
depending on behaviour nothing states; and `pipes/whim115-edit.sh` with
`pipes/whim115-check.sh`, **the wall clock crosses too** — `vim_time()` becomes
`host_time()` below the boundary, `long time(long *tp);` leaves the core's prototype
block and a `static_assert` stronger than it replaces it; and `pipes/whim116.sh`,
**the terminal table is asked a question it can answer** — the second phase that changes
no source at all, replacing `$TERM`, which whim phase 19 stopped the editor reading, with
`+set term={name}`, so nineteen rows that carried one answer between them carried ten
resolutions and nine refusals — two and seventeen since phase 121; and
`pipes/whim117-edit.sh` with
`pipes/whim117-check.sh`, **the core stops reallocating** — `realloc` rewritten at its two
core sites as a `host` allocation, a `musl_memcpy` of the **old** size and a free, because
`realloc` is the one libc function that cannot be vendored at all: to move the old
contents it needs a length its interface does not carry; and `pipes/whim118-edit.sh` with
`pipes/whim118-check.sh`, **the core calls nothing but the host** — `malloc`, `free` and
`write` become `host_alloc`, `host_free` and `host_write`, three prototypes above the
boundary and three definitions below it; and `pipes/whim119-edit.sh` with
`pipes/whim119-check.sh`, **the core names no libc function at all** — the last two
declarations go, and by different routes: `getpid` is **avoidable outright**, its one
caller `mch_get_pid()` feeding a `b0_pid` that whim's removal of recovery had already
left write-only, so the write, the function and the field all go and nothing calls a
host; `kill` is **moved**, `vim_handle_signal()`'s `kill(getpid(), got_signal)` becoming
`host_raise(got_signal)`, which takes no pid because a core that cannot ask for its own
process id must not be handed one; and `pipes/whim120-edit.sh` with
`pipes/whim120-check.sh`, **the degenerate unions** — six of the thirteen `union`
keywords unite nothing with anything, five single-member (`uh_next`, `uh_prev`,
`uh_alt_next`, `uh_alt_prev`, `vval`) and one **empty** (`es_info`), every one a leftover
of a cut already made and the last of them a GNU extension ISO C forbids, on a
**byte-identical binary**; and `pipes/whim121-edit.sh` with `pipes/whim121-check.sh`, **the
eight terminal names** — `builtin_terminals[]` goes from ten rows to **two**,
`xterm-256color`, which is already the compiled default, and `debug`, with three
capability tables, `find_builtin_term()`'s xterm-family clause and a repair to
`set_termname()`'s no-screen fallback that is not optional; and `pipes/whim122-edit.sh`
with `pipes/whim122-check.sh`, **`-T {term}` goes** — `command_line_scan()` becomes one
`if (argv[0][0] == '+')` and one `else` answering `mainerr(ME_UNKNOWN_OPTION)`, with two
`main_errors[]` rows and their enumerators, `mparm_T.term`, and the no-screen arm of
`set_termname()` that only `-T` could reach; and then the **memline arc**, which is one
arc and not six phases — `pipes/whim123.sh`, **the instrument could not see the text
layer**, a whole-phase program that changes no source and adds the sixth part of a
recording, because a `zero-vim` with `pp->pb_pointer[idx].pe_line_count--` deleted from
`ml_find_line()`'s descent recorded **all 102 screen cases byte for byte** and forty
phases had been verified by a corpus that allocates exactly one data block a case;
`pipes/whim124-edit.sh` with `pipes/whim124-check.sh`, **freeing is free** — `host_alloc()`
a bump allocator into a 1 GiB arena and `host_free()` a function that returns, the
charter's *a garbage collector is assumed from here on* built entirely below the
boundary, with `free malloc realloc` leaving `nm -u`; `pipes/whim125-edit.sh` with
`pipes/whim125-check.sh`, **the swap file's residue** — `struct block0` with eight fields
written and none read, the negative block numbers `ml_append()`'s `newfile` could never
make, a three-layer dirtiness nothing tests and `pe_old_lnum`, none of which any tool in
`tools/` can see because **every one of them is written**; `pipes/whim126-edit.sh` with
`pipes/whim126-check.sh`, **a block number becomes a reference** — `pe_bnum` and `ip_bnum`
become `bhdr_T *`, `memline_T` gains `ml_root`, and the hash table that turned an integer
into a page goes with the free list and `mf_blocknr_max`, eleven functions and three
types; `pipes/whim127-edit.sh` with `pipes/whim127-check.sh`, **de-page the leaf** — a data
block stops being an index of byte offsets over a text arena and becomes
`DATA_LN db_line[DB_LINE_MAX]`, so a line's text is its own allocation valid for the
lifetime of the process, taking `db_index`'s 34 mentions, the fourteen interior pointers
and both `offsetof(DATA_BL, db_index)` **by having nothing left to measure**; and
`pipes/whim128-edit.sh` with `pipes/whim128-check.sh`, **fold the node types** — `bhdr_T`
becomes `struct block_hdr { short_u bh_id; }`, `memfile_T` goes entirely, a node is one
allocation at its own size (1,040 bytes for a leaf and 4,088 for a branch against 4,128
for either before), and the file gains a `static_assert` that **fails to compile** if a
later phase narrows `PTR_EN`, so
`zero-vim.c` is
now 78,666 lines
against `whim-vim.c`'s 86,617, and `access`, `fcntl` and `open` join the six libc
symbols phase 89 freed, with `getcwd`, `stat` and `strerror` at phase 93, `fclose`,
`getc`, `putc` and `fsync` at phase 96, seventeen at phase 97, eleven at phase 98 and
`_exit` at phase 100, `exit` at phase 102, `close dup isatty raise sigaddset
sigismember sigprocmask` at phase 103, `fflush fputc fputs fwrite printf putchar
stderr` at phase 104 and `free malloc realloc` at phase 124:
phases 90, 91, 94, 95, 99, 101, 105 to 123 and 125 to 128 free none and say so as an equality,
and phases
92, 93, 96, 97, 98, 100, 102, 103, 104 and 124 name the set each frees rather than the count.
**Five of those equalities are about a symbol a reader expects the phase to take**:
phase 111 says it of `gettimeofday`, 32 of `time`, 34 of `realloc`, 35 of `malloc`,
`free` and `write`, and 36 of `getpid` and `kill`, because the host still calls each one
and a symbol leaves when its last *caller* leaves the **file**. **Phase 124 reads that
rule the other way**: the callers did not move, the *calls* went, because what changed is
what is behind `host_alloc` and `host_free` and not where they are — and it is the first
zero phase since 28 to free a symbol at all. **Phase 126's equality is the one worth
reading twice**: it deletes a hash table, a free list, three types and eleven functions
and frees nothing, because all of it was **pure computation inside the file**, reaching
the outside only through `alloc()` and `vim_free()`.

**After phase 96 the core cannot acquire a file descriptor and holds no stdio
stream, and that is an invariant rather than a count.** `open`, `access` and `fcntl` went at phase 92 and
`stat`, `getcwd` and `strerror` at phase 93, with `chmod fchmod fstat ftruncate
lstat unlink` already gone at phase 89 — so nothing in `zero-vim.c` can name anything
on a disk, and `read`, `write`, `close` and `dup` work on fds 0, 1 and 2 alone.
Phase 93's check asserts it in both directions: the undefined set must move by
exactly `getcwd stat strerror`, none of the eleven may be back, and `read close dup
fsync` must still be there — `fsync` being `ui_write()`'s and the `FILE *` phase's.
Phases 94 and 95 assert it again while freeing nothing themselves, and **phase 96
finishes it**: `FILE` is not named in `zero-vim.c` at all (2 → 0), `fclose getc putc
fsync` are gone, and the check requires `open creat openat stat access fcntl getcwd
strerror fopen fdopen opendir` absent from **both** the source and `nm -u`. The core
can read, write, close and dup fds 0, 1 and 2 and nothing else. `WHIM-PLAN.md` §II.4b
states the invariant and it is assertable in that strongest form from here on.

**After forty-six phases zero-vim is 78,681 lines and 14 libc symbols, and what is
left of the host boundary is a line in the file and nothing else.** From
`whim-vim.c`'s 86,583 lines, 869,512
bytes and 80 symbols that is **−7,902 lines (9.1 %), −109,088 bytes and −66 symbols**;
the binary is 760,424 bytes, still `EXEC` with no `INTERP`, no dynamic section and no
relocation. **The file grew for the first time at phases 97 and 98** — 79,603 →
80,440 lines — because those phases move code *in*, which is the trade the symbol
count is the measure of, and 104, 105, 106, 109, 110, 114, 117, 118 and 124 each grew it again for
the same reason; 41's 68 lines are **every one of them below the first `#include`**, which
is the whole of *this phase touches no core line* and is checked as a `cmp` of
`make editor.c` rather than argued. **Phases 106, 107, 108, 109, 110 and 111 all leave the binary at exactly 788,488
bytes**, and only the first two leave it the same *bytes*: 107 is a `cmp`, 25 differs in
347,279 bytes, and 109, 110 and 111 differ because a call and a move at `-O0` are
different code. **Only three of phases 112 to 122 move the size at all**: 29 —
788,488 → 782,760, 358 sixteen-byte case-map rows out and one in — and then the terminal
pair, 38 taking eight rows and three capability tables to **781,096** and 39 taking a
parser arm and an unreachable fallback to **781,064**. The rest leave it where it was,
113 to 120 at 782,760, where 31 differs in 604,650 bytes for putting two definitions
near the front of the file, 35 in 206,588 and 36 in 397,978 for the same reason — and
**37 is the only one of them that is the same bytes**, which is its whole evidence.
**The memline arc then moves it four times and is the largest run of shrinkage since the
case tables**: 41 to **772,872**, which is smaller although the file grew by 68 lines and
`.bss` by a gigabyte, because `.bss` is `NOBITS` and musl's allocator left the link; 42
to **768,744**; 43 to **760,456**; 127 to 760,456 again — the same size, different bytes,
absorbed by alignment padding; and 45 to **760,424**.
**Phases 116 and 123 change no
source at all, as phase 86 did**, so q116's `zero-vim.c` is q115's byte for byte and q123's
is q122's, and each boundary digest is its input's — `d2a14122ccf7` either side of 33 and
`68e450fd6912` either side of 40.
**Phases 100, 101 and 102 each leave the image at exactly the same 805,544
bytes** — different bytes, the same size, the difference absorbed by alignment padding
— although 17 removed nine lines, 18 added five and 19 added eighteen. Their measure is
the symbol, not the size, and 101's is neither: it frees nothing and says so as a `cmp`.

The 14 are the terminal (`read ioctl select tcgetattr tcsetattr nanosleep`), `write`,
the clock
(`time gettimeofday`, **both of them the host's since phase 115**), signals (`sigaction
sigemptyset kill getpid`), and one gcc emit
the source names nowhere (`__errno_location`). **Not one of the 14 is called from the
core any more**: `realloc` went to the host at phase 117, `write`, `malloc` and `free` at
35, and `getpid` and `kill` at 36 — which is why there is no longer a row for *the one
syscall the core does for itself*. **And the memory row is gone outright**: phase 124 made
`host_alloc` a bump allocator over one static arena and `host_free` a function that
returns, so `malloc`, `free` and `realloc` have no caller anywhere in the file. The three
were the only users of `<stdlib.h>`, which is **dead and stays**, measured — the output
built with the directive deleted is byte-identical, 772,872 either way — on phase 96's
precedent for declining, because a phase that changes two things cannot say which one a
difference came from. `tools/symbols.sh` counts 15 because it
compiles plain `-O0` and so adds `__stack_chk_fail`. **The messages row is gone and the
"gcc's own" row is down from five to one**: phase 104 deleted every `printf` and
`fprintf` call, which took `fflush` and `stderr` with them and took `fputc fputs fwrite
putchar` — four symbols the source has never named, gcc's own expansion of
`printf("%s", x)` — with the construct. **The signals row used to be "signals and
exit", then "signals", and is now four calls the HOST BLOCK makes** — `kill` and
`getpid` were the core's last two until phase 119 and `host_raise()` makes both now:
phase 100 took the
`_exit(8)`/`exit(7)` pair that could not run, phase 102 took `mch_exit`'s `exit(r);`, the
one that did, and phase 103 took `raise sigaddset sigismember sigprocmask` with the fifty
lines of `mch_signal()` that emulated `sigset()`. **There is no row for strings,
character classes, numbers or sorting any more**: those 28 were the pure computation,
and phases 97 and 98 put them inside the file as `static` definitions rather than asking
a host for them.

**All 14 are now called from the host and from nowhere else** —
`read ioctl select tcgetattr tcsetattr nanosleep sigaction sigemptyset`, `gettimeofday`
since phase 109 put `musl_gettimeofday` in the host block, `time` since phase 115 put
`host_time()` there, `write` since phase 118, and `getpid kill` since 36 — with
`__errno_location`, which no line of either
half names and which gcc emits for the host's three `errno` mentions — **measured by
splitting
the file at the first `#include`**, which since phase 110 is an exact line and not a
region anybody has to identify. Measured on the committed `zero-vim.c` that way: **the
core's whole vocabulary of the fourteen is three English words inside string
literals** — the two `NGETTEXT`
strings in `op_shift()` that say *time*, and `E222`'s *"already read from"*. That is the
thing `nm -u` cannot show: moving a
call from the core into the host inside ONE translation unit frees no symbol, because a
symbol leaves when its last *caller* leaves the file and that is the split.
`tools/zhostonly.py` is the assertion instead — **45 host words**, `getpid` having
joined them at phase 119, every one of their 64
mentions inside the 256-line
block, and a named exception with a reason for each thing the core still says. Run on
the committed `zero-vim.c` it reports **6 of
its 18 named exceptions live in the core, and all six are one thing**: `SIGHUP` and
`SIGTERM`, three times each, in `signal_info[]`, in `deathtrap()` and as the two
enumerators phase 110 wrote with the `static_assert` that checks them against the header
below. They are there because the editor **prints** those two names. **Until phase 119
there were eight, and the two that went are the whole of what the core still said that
was not a message**: `mch_get_pid()`'s `getpid()` and `vim_handle_signal()`'s
`kill(getpid(), got_signal)`, the deferred re-raise, which is now `host_raise()` inside
the block and calls both from there. `musl_suspend()` still calls `kill` too, which is
why `HOST_MUST` can go on requiring it.
`write` was shared from phase 104 to 117, `mch_write`'s `write(1, …)` in the core and
`host_message`'s in the launcher, and **since phase 118 both are the host's**, as are the
three allocations — `host_alloc`, `host_free` and `host_write` are three prototypes at
the end of the one declaration block and three five-line definitions below the boundary.
**The clock is not split any more** — `gettimeofday` went to the host at phase 109,
because `struct
timeval` is a layout the core must not name; phase 111 replaced the out-parameter pair
with `long musl_now_ms(void)`, a **scalar**, so that no host call's shape is decided by
a type the core cannot spell; and phase 115 sent `vim_time()` over as `host_time()`, so
`time` has **no core call site at all**.
**`getpid` was the last one that was not the host's, and phase 119 did not move it — it
removed the need for it.** `mch_get_pid()` was `return (long)getpid();` with exactly one
caller, writing a `b0_pid` that nothing reads: block zero's process id, whose readers
went with recovery in `whim-vim.c` itself, so the field has had two mentions — its
declaration and that write — since before this pipeline began. The write goes, the
function goes with it, the sweep takes the forward declaration and the field, and no
host call is added at all. **Phase 103 saw it coming and said so**: *“`b0_pid` is
written and never read, so one line frees it whenever block zero is somebody's phase”*.
`__errno_location` is the one symbol that was never anybody's call, and phase 104 says so
rather than implying
otherwise: `errno` has three mentions and did not move at all when stdio went — the
`#include`, the `tcsetattr` retry and the `select` test — so **`<errno.h>` leaves the
CORE and the symbol leaves the process at the split**.

**`WHIM-PLAN.md` §II.4c is built out, and phases 118 and 119 finished it.** Its three steps
were `main()`, the two stream calls, and the terminal with its signal set; 101 and 102 did
the first, 103 did the third, 104 did half of the second and **35 did the other half** — so
**the core makes no syscall for itself at all**, and since 119 names no libc function
either. All three `write` call sites and the one
`read` are below the boundary: `musl_read_input`'s `read(0, …)` where phase 103 put it,
`host_message`'s and `host_write`'s. Phases 97 to 99
reached §4c's closing sentence early, the two rows it expected to be left in the core
being gone before the launcher existed; it expected `exit` and `_exit` still to be
there after the move, and both are gone; and it expected the third step to take
`ioctl`, `tcgetattr`, `tcsetattr`, `select`, `nanosleep` and the nine signal symbols
out of `nm -u`, which it does not and cannot. **`isatty` was the terminal's and not the
filesystem's** — three call sites, `WHIM-PLAN.md` §II.4a had it going, and §1's decision 7
(*"do not ask whether stdin or stdout is a terminal"*) was only two thirds kept until
phase 103 took all three. **§4c's boundary is now drawn, and it is the `boundary`
package: phases 106, 108, 109, 110, 111 and 117** — one file with two parts rather than two
files,
the `#include`s moved to line 76,689 and **the first one the line between the core and
the host**, marked by nothing else. 106 was first and deliberately the smallest, because
it is the one a `cmp` can check; 108 made the two host calls plain; 26 gave the core its
own types and macros *while the headers were still above them to check against*; 27
performed the move; **28 is the one that finished what the line is for** — it
deleted the tagless `struct timeval` mirror 109 had to invent, so that no core → host
signature's shape is decided any more by a type the core cannot name, all eighteen of
them now taking scalars and byte buffers only; and **34 is in the package because its
product is one line fewer in the block 109 wrote**, not because anything is vendored —
`realloc` is the one libc function that *cannot* be vendored, needing an old size its
interface does not carry, so the core loses it by each call site supplying the length it
already knows. Phase 107, the attributes, sits between
them in a package of its own, `dialect`, and moves no line at all; phases 113, 115, 118, 119
and 124 are
`host`, because they move a thing the core did for itself **across** a line already
drawn — 41 being the one that moves nothing and changes what is *behind* a name the
core already asked through, which is why its evidence is that `make editor.c` is
**byte-identical in and out**, 77,681 lines and 2,064,232 bytes; and 120 and 125 are
`tidy` with phase 96, because five of 37's six unions and all four of 42's groups are
leftovers of cuts already made. See *The core and the host are one file with a line in it* in `CLAUDE.md`.

**What phases 121 and 122 take is not the boundary but the terminal, and that is the
`terminal` package: 85, 121 and 122.** Phase 85 removed the **question** the core asked about
a terminal it had not been told about — the two "not to a terminal" warnings, the pause
and `--ttyfail`; phase 121 removed the **vocabulary**, eight of the ten built-in terminal
names with three capability tables and `find_builtin_term()`'s xterm-family clause,
leaving `xterm-256color` and `debug`; and phase 122 removed the **telling**, `-T {term}`,
so nothing outside the process can say what terminal this is and `+set term=` inside the
editor is the only way. **38 is the first zero phase to declare anything since phase
94**, and the first ever to use `term-moved`, a token that had been in the grammar since
phase 83 with nothing to say.

**And what the last six take is the text layer, which is one arc and not six phases —
`harness:123`, `host:124`, `tidy:125` and the `memline` package, 126, 127 and 128.** The order
is the argument. **40 had to be first**, because until it ran nothing in the pipeline
could tell a working memline from a broken one: a `zero-vim` with
`pp->pb_pointer[idx].pe_line_count--` deleted from `ml_find_line()`'s descent records
**all 102 screen cases byte for byte**, every one of them allocating exactly one data
block, so `idx` is 0 every time and a pointer entry's line count never decides anything.
`tools/zmemline.py` is sixteen cases of 200 to 25,000 lines built **in the editor** —
there is no file argument, no `:edit` and no `:read` — and its depth is measured and not
intended: a data block splits in 16 of 16 and the **root** splits in 4, against 0 of 102
for all seven markers. **41 made the work cheap**, `host_alloc()` becoming a bump
allocator and `host_free()` a function that returns, which is the charter's *a garbage
collector is assumed from here on*: the core may allocate a record per line and simply
not free it. **42 cleared the way**, four groups of swap-file bookkeeping that is
**written and never read** and that no tool in `tools/` can see for exactly that reason.
Then **43 turned a block number into a reference** — `pe_bnum` and `ip_bnum` become
`bhdr_T *` and the hash table that resolved integers to pages has nothing left to look
up — **44 let the leaf stop being a byte arena**, so a line's text is its own allocation
valid for the lifetime of the process, and **45 folded the node types**, so a node is one
allocation at its own size and `memfile_T` is gone. **45 also ends the arc's standing
hazard the only way that survives a reader who has not read the documents**: the file
carries `static_assert(PB_COUNT_MAX == (4096 - 8) / sizeof(PTR_EN), …)`, which **fails to
compile** if a later phase narrows the entry — because an 8-byte `PTR_EN` would put the
fanout at 511, the corpus's deepest case builds 391 data blocks, and root-split coverage
would go to zero **without moving one record**. That last is demonstrated and not argued:
the `fanout` control moves 0 of 118 and takes three markers from 1 to 0.

**Phase 95 is the other zero phase that declares nothing, and for the opposite
reason.** Phase 92 removed code that could not run; phase 95 removes code that *can*
run and that the instrument cannot see — no recorded case or row asks `:set ro?` or
any of the other five, bare `:set` is wiped by the Press-ENTER redraw before a
snapshot is taken, `tools/zexcmds.py` keeps no stream digest for the `set` row, and
nothing types `:set ro`, so the `W10` warning and the two `[RO]` indicators are never
drawn. Two full recordings are byte-identical, and the evidence is 27 probes on both
binaries — `ro_w10` being the one that shows behaviour going: `:set ro` on an
unmodified buffer and then an insert draws `W10: Warning: Changing a readonly file`
and pauses **1,006 ms** on the binary the phase was handed and **2 ms** here.

**Phase 96 is the third that declares nothing and it is phase 92's kind, not phase
95's — and phase 100 is that kind too**, nine lines of `deathtrap()` that no build of
zero-vim could reach, because `catch_signals()` installs with `sa_flags = 0` and
`signal_info[]` has exactly two deadly rows, so `entered` can reach 2 and never 3. Its
evidence is the same source built five ways, of which two differ in **one `sigaction`
field**: with `sa_flags = 0` a forced double signal stops at depth 2 and exits 1, and
with `SA_NODEFER` it reaches depth 3, runs the ladder and exits **7** — which is
`exit(7)` executing — and one forced signal further exits **8**, which is `_exit(8)`.
Phase 96's own argument was textual: `scriptin[]` is assigned once in the whole file — to NULL, inside the
function the phase removes — and `redir_fd` only by its own declaration, so neither
`FILE *` has ever been opened in any build of zero-vim and the phase removes the
*possibility*. Its evidence is the source it was handed, built with
`write(2, "FILESTAR-ENTERED\n", 17)` at **five** places (**0 of 106 records**) and
then with the identical instrument on `ui_write()` (**105 of 106**), plus eighteen
adversarial sessions that are each a way of making the editor *print* — which is
where `redir_write()` sat.

**Phase 92 is the one zero phase no recording can see, and it says so.** `readfile()`
was already unreachable when it ran — phases 88 to 91 had taken every way to name a
file — so its declared delta is *nothing at all*, two full recordings are
byte-identical, and the evidence is an instrumented pair: the input source built
twice, with `write(2, "READFILE-ENTERED\n", 17)` first in `readfile()` (**0 of 106
records** carry it) and then in `open_buffer()` (**104 of 106**, the identical
instrument), plus eight adversarial sessions that name the buffer after a real file
and are required to reach `open_buffer()` and not `readfile()`. An empty declaration
is a statement here rather than an omission: the phase removes code that could not
run.

**Phase 99 is the fourth, and its empty declaration is the strongest of the four
because a byte comparison settles it.** Phases 92, 95 and 96 each removed *something* —
code that could not run, code the instrument cannot see, a possibility. Phase 99
removes six `#include` lines and one typedef and **changes no code at all**, so its
evidence is not that the recording did not move but that the binary is the same bytes:
805,544 either side, both built with `SOURCE_DATE_EPOCH=0` and the boundary's own
flags. That is verification tier 1 below, and it subsumes every screen case, every
Ex-command row, every command line and every pty scenario at once, because the program
that would be run is literally the same program. Its own argument is a *computation*
rather than a list: each of the twelve surviving `#include`s is dropped in turn and the
compile must fail, and the identical loop on the source it was handed must find exactly
the six it removes — the same loop proving it can fail, in the same run.
**Phase 106 is that kind too, on three thousand edits instead of seven**: `NULL` → `nullptr`
and `size_t` → `usize` change no statement, so the evidence is `cmp` — 788,488 bytes
either side — and the control is what keeps it from being two numbers agreeing, the same
edit with the string literals *not* excluded differing by 1,598 bytes, 1,354 of them in
`.rodata`.
**Phases 97 and 98 declare nothing for a fifth reason and it is the weakest**: their
code changes and their binary moves, and the claim is that twenty-eight replacement
implementations do what the ones they replace did. Their own checks argue that.
**Phase 112 is a ninth kind, and the only one where the behaviour really did move**: both
arms of `'casemap'` change — 96 upper and 96 lower codepoints gain a mapping on the
non-internal arm and `ß → ẞ` arrives on the default one — and the corpus cannot see any
of it, because all 102 screen cases seed themselves by typing ASCII and none touches the
option. So the declaration is empty and the phase owes twelve probes, six that must
differ and six that must not, plus a claim checked over **all 1,114,112 codepoints**
with the musl half re-derived from this machine's libc rather than from the bytes the
phase deleted. **Phase 114's is a tenth**: `abs` and `labs` were never in `nm -u` at all,
gcc lowering both to inline arithmetic, so the phase's whole value is that the core
stops depending on behaviour nothing states — and the evidence is neither a `cmp` nor a
recording but an **accounting**, the object's `.text` growing by exactly 42 bytes, which
is `musl_abs` (19) plus `musl_labs` (24) plus what the three callers gained or lost
(−2, +1, 0) and nothing else. **Phase 116's is an eleventh, and it is phase 86's**: the
phase changes no source at all, so nothing about the editor's behaviour *can* have
moved, and what it has to argue instead is that the **comparison** moved safely — which
it does by measurement rather than by citation, `./zero-vim` extracted from all 33
recorded boundary tars plus `whim-vim.c` built with whim's own line giving **one digest
across every one of them** under the new question, as the old question gave one across
every one of them.
**Phases 117 and 118 are the strongest instance of a kind already on this list and not a
new one**, and it is worth saying which: their evidence is two byte-identical recordings,
which is usually the weak answer, and here it is not, because what they touch is on the
path of everything. `ga_grow_inner()` is driven **4,289 times per recording** on both
sides, 2,739 of those with `ga_data == nullptr`, and the control that keeps 34's rewrite
and copies nothing moves **102 of 102** screen cases; `lalloc()`, `vim_free()` and
`mch_write()` are entered **53,848, 22,417 and 1,012 times** over the 102 cases, and
`host_write` writing half the bytes moves 102 of 102 while `host_write` reporting half
moves 0 — which is 35's claim about the return value measured rather than argued. 34 also
owes a harness rather than probes, because **its four traps are memory bugs and not
differences**: an AddressSanitizer driver built at run time from both sources, where six
of seven controls each produce their own named finding.

Phases are added one at a time, on request. Its input is the **committed**
`whim-vim.c`, immutable, and `whim.sha` records the one a committed `zero-vim.c` was
produced from, exactly as `slim.sha` does for whim. It is born staged:
`pipes/whim.stages` (a stage per phase and seventeen packages: `seed 0`, `build 1`,
`terminal 2 38 39`, `harness 3 33 40`, `streams 4 5`, `files 6 7 8 9 10`, `buffers 11`,
`options 12`, `tidy 13 37 42`, `vendor 14 15 31`, `includes 16`,
`host 17 18 19 20 21 30 32 35 36 41`,
`format 22`, `boundary 23 25 26 27 28 34`, `casemap 29`, `dialect 24` and
`memline 43 44 45`, with `apart 85 87` — phase 85's
check runs both its binaries with `-e -s`, which phase 87 removes — `apart 87 88`,
because phase 87's check names `EDIT_STDIN`, `had_minmin`, `buflist_add` and
`ME_TOO_MANY_ARGS` as things the argv phase is still to take, `apart 88 89`,
because phase 88's check states that *it* frees no libc symbol and phase 89 frees
six, `apart 89 90`, because phase 89's check names `do_bang` as a later phase's and
requires 105 rows, `apart 90 91`, because phase 90's check names `do_ecmd` and
`otherfile` as a later phase's and requires 104 rows with `:edit` among them,
`apart 91 92`, because phase 91's check requires `readfile` at 5 mentions and
`read_buffer` at 17 and phase 92 takes both to 0, `apart 92 93`, because phase 92's
check draws its line against the name phase as counts — `b_ffname` 32, `b_fname` 29
and sixteen more — and phase 93 takes every one to 0, and `apart 93 94`, because
phases 89 to 93 all assert `E37: No write since last change` survives and name
`check_changed` as the `:q` phase's — only the last of the five is written, a stage
holding 89 and 94 holding 10 already — and `apart 94 95`, because phase 94's check pins
`p_ro` and `p_ur` at 2 mentions with their option rows and phase 95 removes both, and
`apart 95 96`, because phase 95's check pins `scriptin` at 8, `redir_fd` at 6 and
`vim_fsync` at 3 and names all three as the `FILE *` phase's, and
five `need`s, `need 90 swept` — phase 90's `usefilter` anchor counts eleven mentions on
the text phase 89's edit leaves and ten on the swept one — `need 91 swept`, phase
91's `readfile` anchor counting seven where it wants five, `ex_read` still being there
to make two of them, and `need 92 swept`, phase 92's `open_buffer` anchor counting six
where it wants five, `do_ecmd` still being there to make the one call site its
signature fold would not rewrite, `need 93 swept`, phase 93's `b_ffname` anchor
counting forty where it wants 32, `readfile()` still being there to make eight of
them, and `need 94 swept`, phase 94's `buflist_findfpos` anchor counting four where it
wants three, `buflist_findlnum()` still being there to call it from outside the island
the whole phase rests on — **`need 95 swept` and `need 96 swept` are both measured NOT
to be required**, phase 95's computation giving the same seven rows on unswept text
and phase 96's anchors all holding there) and `pipes/zero.delta`
(`2 stderr-moved`, phase 87's six records, phase 88's six command lines, phase 89's
two cases and six command rows, phase 90's two cases and one, phase 91's two cases
and five, **nothing at all for phase 92**, phase 93's one case and one row, and phase
94's one case and one row — where the row `quit` is the first zero has declared that
**changes message rather than ceasing to exist** — and **nothing at all for phases
95 and 96**, and **nothing at all for 97 through 120** —
where 99's, 106's, 107's and 120's empty declarations are the fourth kind above, a byte-identical
binary; 101's, 102's, 104's, 108's, 115's, 117's and 118's are a sixth, the code runs and the
instrument sees it
do the same thing; 103's, 105's, 111's and 113's are phase 85's and phase 95's, the code runs
and the instrument cannot
see it, so each owes probes and they run fifteen, 263, four and 36 of them; 109's is a seventh,
six of its seven changes a `cmp` and only the clock a recording; 110's is an eighth,
**the source is the same lines rearranged** — not one of the input's 80,197 lines
missing and 35 added — because a phase that moves 1,568 lines of definitions can have no
`cmp` at all; 112's is a ninth and the one where behaviour really moved, on both arms of
`'casemap'`, invisible to a corpus that types ASCII; 114's is a tenth, an accounting
of 42 bytes of `.text` for a phase that frees no symbol because there was none to free;
116's is an eleventh, a phase that changes no source at all, where what has to be
argued is that the comparison moved safely and not that the editor did not; and **36's is
phase 85's and phase 95's again** — the code runs and the instrument cannot see it — where
the two halves are invisible for **opposite** reasons, the `b0_pid` write running in 102
of 102 screen cases and moving nothing because nothing reads the field, and the deferred
re-raise firing in **0** of 102 because no keystroke corpus sends the editor a deadly
signal, so the phase owes probes and builds six binaries of its own — **and then
`38 term-moved` and phase 122's four command lines, the first lines the file has gained
since phase 94 that are not a comment**: 121 moves eight of `tools/ztermcheck.py`'s
nineteen rows, each from `term=<itself>` to `E522 term=xterm-256color t_Co=256`, and 39
moves `-T xterm`, `-T no-such-term-9x`, `-T` and `-Txterm`, **two of which were already
errors and moved anyway** because the messages they gave were `ME_ARG_MISSING` and
`ME_GARBAGE`, enumerators whose last use was the argument block that went — **and then
nothing at all again for 123 to 128**, which between them are four of the kinds already on
this list: 123's is 116's and 86's, no source changed at all; 124's and 126's are the sixth,
the code runs and the instrument sees it do the same thing — 43's being the strongest
instance the pipeline has, because every keystroke this editor draws reaches its text
through the function it rewrites; 125's is phase 92's and phase 95's **at once**, code that
could not run and code that runs everywhere and the instrument cannot see, both carried
by one instrumented build over **252 records**; and 127's and 128's are the **weakest**,
phases 97 and 98's, the code changing and the binary moving with nothing but controls to
say a replacement does what the original did — eleven of them for 44 and twelve for 45,
each with the three that must *not* move named with the reason each cannot be seen),
checked
by `tools/stages.sh zero` and `tools/packages.sh zero` as whim's are. The last two
`apart` lines are the same shape as `apart 88 89`: `apart 97 98`, because each of those
two checks states that the undefined set moved by exactly *its* symbols and a stage
takes one snapshot at its start, and `apart 98 99`, in both directions — phase 98's
check requires `<ctype.h>` and `<wctype.h>` still present, and phase 99's states as a
`cmp` that it frees nothing while phase 98 frees eleven — and `apart 99 100`, in one
direction only: phase 99's check requires the file to have lost exactly eight lines and
phase 100 takes nine more, while phase 100's own check passes on a shared stage. The
last two are the same shape and both measured: `apart 100 101`, because phase 100's check
requires the file to have lost exactly nine lines and 101 adds five (`the file lost 4
lines, expected 9`), and `apart 101 102`, because phase 101's check builds its own control
by rewriting `mch_exit`'s `exit(r);` and phase 102 has replaced that line (`the control
edit changed nothing`) — each in one direction only, the later phase's check passing on
the shared stage either time. **`apart 102 103` is the first that is both directions**,
and its measured message is the one a reader would not predict: `tools/phaserun.sh zero
19-20` on q101 stops in phase 102's check with ``deathtrap` as a whole word has 4
mentions, expected 3` — a phase about *removing* signal handling leaves one MORE
mention of a handler, because the host installs the core's `deathtrap` rather than
replacing it. The other direction is reasoning rather than a second run, because 19's
check refuses first and there is nothing after it to observe: phase 103's check states
its gone set as one `comm` against the stage's symbol snapshot, and a stage takes one
snapshot at its start, so on a shared stage it would be handed q101's set and the gone
set would be its seven plus `exit`. That is `apart 97 98`'s shape.

**The last three are measured too, and each is one direction only.** `apart 103 104`:
phase 103's check states the twelve `#include` directives as a property it preserves and
phase 104 takes `<stdio.h>`, so a 20-21 stage stops with *"the output does not have
exactly the twelve `#include` directives phase 99 left"* — and the same run measures the
other half, phase 104's edit applying unchanged to phase 103's unswept output, which is
why there is no `need 104 swept`. `apart 104 105`, and its first complaint is not the
predicted one: ``printf` has 4 mentions, expected 10` — **six of phase 104's nine
`format(printf, …)` attributes sit on the wrapper prototypes phase 105 deletes**, which
is phase 104's own counting trap read from the other end. And `apart 105 106`, where a
phase that renamed nothing breaks on a phase that renamed two type names: phase 105
writes its four controls by matching the helpers' text **verbatim**, two of the three
hold `if (IObuff == NULL)`, and phase 106 spells that `nullptr` — so it stops at its
first act with ``iobuff_room` is not in the output exactly once, so the controls below
would not be controls`. **A check that quotes C is a dependency on spelling**, and that
is the general form of it.

**The last four continue the pattern, and one of them is the general form of a second
dependency.** There is **no `apart 106 107` and no `need 107`** — measured in one run,
where `tools/phaserun.sh whim 106-107` on q105 runs both edits, one sweep and both checks
and every part passes. `apart 107 108`'s first complaint is the one nobody would predict:
**phase 107 records the six `format`/`format_arg` lines it keeps by LINE NUMBER**, and
phase 108 deletes an object with its blank line at 495, so everything below moves up by
two and the check stops at *"a line carrying a kept attribute is not the line it was,
byte for byte"*. **A check that pins a line number is a dependency on every line above
it**, which is the spelling trap one level up. `apart 108 109` is phase 108's line-count
assertion meeting the twenty-four lines phase 109 adds, and `apart 109 110` is phase 109's
sixteen `static_assert`s and four wrong declarations, **every one of which needs the
headers above the core** and none of which can compile once they are below. Each is one
direction only, the later phase's check passing on the shared stage, and **`need 108`,
`need 109` and `need 110` are all measured not to be required** — phase 109's case by a
direct `cmp`, phase 108's edit on q107 giving a `zero-vim.c` byte-identical to q108's.

**Phases 111 to 115 add eight more `apart` lines and no `need` at all** — each of the five
was measured to apply unchanged to the unswept text before it. `apart 110 111` is the
neatest of all of them: phase 110's boundary argument rests
on moving **one core function below the cut** as a control, the function it picked is
`elapsed`, and that is the function phase 111 deletes — so the check stops at its first
act with *"`elapsed` is not defined exactly once above the boundary, so the control that
moves one core function below it would not be a control"*. `apart 111 112` is arithmetic:
phase 111 states its own as a line count **of the core**, and a 28-29 stage gives it
*"the core is 77978 lines and was 78359, a difference of −381 where −16 was expected"*,
its −16 less phase 112's −365. `apart 113 114` is `apart 100 101`'s shape — phase 113's line
count meeting the ten lines phase 114 adds to the same swept text. `apart 104 113` could
not be run at all, seven phases and their state directories sitting between the two, so
it was measured by **applying phase 104's pin table directly** to the tree phase 113
leaves: of its thirteen pinned names seven are already broken by phases 105–112, four are
untouched, and exactly two move here. And phase 115 carries **four**: 109 and 110 both hold
the nine-entry `PROTOS` list and now find three of the nine at 0, and 110 and 111 both
**write the boundary out as thirteen names** where phase 115 makes it fourteen — which is
the argument for computing a set at run time rather than quoting it, made from the
losing side.

**Phases 116 to 119 add ten more, and one pair is forbidden rather than declared.** 34
carries three — `apart 109 117` and `apart 110 117`, because both those checks assert
`void *realloc(void *p, usize n);` on a line of its own exactly once and it is not there
any more, and `apart 113 117` in **both** directions, phase 113's line count meeting the ten
lines 117 adds and 34's meeting the eighty-one 113 removes. 35 carries six: 109 and 110's
`PROTOS` list again, now with **seven** of its nine at 0 and only `getpid` and `kill` at
1; 110 and 111's written-out boundary, seventeen names where 115 made it fourteen;
`apart 113 118`, which is the sharpest instance this pipeline has of *a check that CALLS
libc from above the boundary is a dependency on the core still declaring it* — phase 113's
check writes `write(2, "T-cleos\n", 8);` **into the core** to build an instrumented
control, so run as a pair it does not compile at all; and `apart 117 118`, measured as a
real shared stage, where phase 117's check stops four ways and two of them are
`apart 105 106`'s lesson landing on the phase that had just taught it — 34's check matches
the code it wrote **verbatim**, `pp = malloc(new_len);` and `free(gap->ga_data);`, and 35
renames exactly those calls. **There cannot be an `apart 116 117` at all, and that is a
measurement and not an omission**: a stage of more than one phase is made of split
programs and `pipes/whim116.sh` is ONE file, so `stage 116-117` is refused before any check
runs — `phase 116 is in stage 116-117 but is not an edit and a check` from
`tools/stages.sh`, and `phase 116 has no edit and check to run` from
`tools/phaserun.sh`. **No `need 116`, `need 117`, `need 118` or `need 119`** — 118's measured
three times, on phases 113's, 115's and 117's unswept output, and 36's on 35's, with the
prototype it is about to orphan still there and `b0_pid` still a field.

**Phase 119 carries exactly one `apart` line and the reason it carries only one is worth
copying.** `apart 118 119` was measured as a real shared stage on q117, and phase 118's check
stops three ways, the first being the block the phase exists to empty: *the ordinary
declarations above the boundary are none and the input's block minus the three is
`int getpid(void); / int kill(int pid, int sig);`*. Six further lines — 109, 110, 111, 113, 115
and 117, every one of whose checks this output also breaks, 109's and 110's nine-entry
`PROTOS` list reaching **zero of nine** on it — are **deliberately not written**, because
every one of them is already broken by phase 118 and recorded against it, and any stage
holding 36 and one of the six would have to hold 35 too. **A redundant `apart` nobody
measured is worse than none.**

**The last three are one each, and two of them are arithmetic.** `apart 119 120` is the
only one of the three measured in **both** directions, both phases being split: 36's
check stops on its own line count — *the file is 79786 lines and the input was 79804
(79804 recorded) — expected 79799* — and, run the other way on exactly the tree the
driver would hand it next, **37's check stops too**, with *the six declarations are 13
lines shorter between them* and *the boundary moved by 15 lines and the file by 13*. The
two extra lines are **phase 119's residue**, `mch_get_pid`'s forward declaration and the
`b0_pid` member, which 119 leaves for the sweep on purpose, arriving inside 37's
arithmetic because a stage sweeps **once**, at the end: *a phase that states its line
count against its own edit cannot share a sweep with a phase that leaves work for it.*
`apart 120 121` is `apart 100 101`'s shape — 37's line count meeting the 118 lines 121 takes
out of the same swept text — and `apart 121 122` is the sharpest form of `apart 105 106`'s
lesson this pipeline has: phase 121's check builds its first control by finding **the
fallback it repaired**, and phase 122 deletes that fallback, so a 38-39 stage stops at 38's
**first act** with *set_termname() names 0 of the surviving rows and this check needs
one — the fallback*. **A check that depends on the code the phase repaired is a
dependency on the next phase not needing it.** No `need 120`, `need 121` or `need 122` —
120's and 122's measured on the unswept output before them, 38's reported as **vacuous**,
phase 120's sweep removing nothing at all.

**The memline arc adds three `apart` lines and two `need`s, and one `apart` is forbidden
rather than declared.** There is **no `apart 122 123` and no `need 123`**, for the reason
there is no `apart 116 117`: `pipes/whim123.sh` is one file, so `stage 122-123` is refused
before any check runs — *phase 123 is in stage 122-123 but is not an edit and a check* from
`tools/stages.sh` and *phase 123 has no edit and check to run* from
`tools/phaserun.sh`, both measured — and `need` is a statement about an **edit part**,
which a whole-phase program has none of. **`apart 124 125` is measured and is not the
mechanism the phase predicted**: it expected `apart 97 98`'s shape, an undefined-set
equality against a stage's one snapshot, and what actually fires is phase 124's **own
promise** — `tools/phaserun.sh whim 124-125` on q123 stops with *the text above the first
`#include` is not byte-identical in and out, and this phase is entirely below it*. **A
phase that promises to touch no core line cannot share a stage with one that deletes 366
of them.** **There is no `apart 125 126`, deliberately**, although phase 125's check quotes
verbatim two lines phase 126 rewrites and would stop: `need 126 swept` already forbids the
only stage that could hold both, `tools/stages.sh` answering *43 needs swept input and
does not start a stage (42-43)* and exiting 1 before any check runs, and **an `apart`
nobody can measure is phase 119's rule**. **`apart 126 127` is `apart 119 120` in its sharper
form**: phase 126 states the division between its edit and its sweep as a **partition over
names**, a stage sweeps once at the end, and the ten names phase 127's edit orphans land
in phase 126's sweep set, so the check stops naming all twelve — *36-37 was that lesson in
a line count; this is the same lesson in a set, and a set is what a later phase is more
likely to state.* And `apart 127 128` is phase 127's own scope statement read from the other
end, and needed no reasoning: that phase wrote that `offsetof(PTR_BL, pb_pointer)`
measures a **pointer** block and is not the leaf, and says it as a count of 1, so running
its check on the q128 tree gives *`ml_new_ptr`'s offsetof moved*.

**`need 126 swept` and `need 128 swept` are both required, and neither breaks where a reader
would guess.** 126's is not a counted anchor at all: phase 125 leaves `mf_hash_free_all`
standing for the sweep and its **forward declaration** names all three types 126 deletes,
so the edit runs to its last act and refuses with *names this phase removes are still
said: `blocknr_T` 1, `mf_hashitem_T` 1, `mf_hashtab_T` 1* — **the cut applied cleanly and
the partition refused, which is what a partition is for.** 128's is **the first in this
pipeline about blank lines**: the edit asserts it leaves no run of two blank lines
anywhere, which is a statement about *this* edit only if the text it was handed had none,
and phase 127's unswept output has one at line 33,815 that `canon.py` removes in phase 127's
sweep. **The order of those two tests is the whole of it** — asked the other way round the
refusal blamed this phase for the previous one's residue. There is **no `need 125`** for a
reason stronger than a stage measurement (phase 124's sweep is a **no-op**, so there is no
unswept text to be handed at all) and **no `need 127`**, measured as an *equality* rather
than as a run that did not refuse: handed phase 126's unswept output, the file the one
sweep leaves is byte-identical to the sequential run's, 78,859 lines either way.

**Phase 118's anchors are a partition and not a count, and phase 117 is why.** It first
asserted `malloc` at 2 mentions, `free` at 3 and `write` at 2 — the counts measured on
the boundary it was written against — and then phase 117's `realloc` rewrite took two of
them to 4 and 5 and the anchors **refused**, which is what a counted anchor is for. Both
programs now assert the *shape* instead: every mention of each name above the boundary is
its own declarator or a call of it, the declaration goes, every call is rewritten, and how
many there are is read off the text — with a mention that is neither, an address taken or
a variable of the name, refusing rather than surviving into a file whose declaration is
gone. That is *Rename a name across the whole file* in `CLAUDE.md`, and the general rule it states:
**assert a partition, not a count**, because a count is a fact about a tree that was
measured and a partition is a fact about the tree that arrives.

**A phase can break a harness rather than change behaviour, and the two must not be
confused.** `tools/termcheck.py` — whim's, and the one instrument the three
pipelines shared — asks `:set term? t_Co?` with a file argument on the command
line, so from phase 88 it records nineteen empty rows. Declaring `term-moved`
for that would have switched the terminal table off for every later phase; instead
`tools/ztermcheck.py` imports `termcheck.py` and replaces the one call that passes
a file, and it is proven to record the baseline's nineteen rows from the binary
phase 88 was handed and from `whim-vim.c` in phase 83. `termcheck.py` itself is
untouched, which is what keeps whim's and slim's keys where they were; only
`tools/zrecord.sh` names the new tool, and that re-keyed zero's five earlier phases
and nothing else.

**And a harness can be blind for a reason that has nothing to do with the phase it is
run for.** That is what phase 116 found and fixed. `ztermcheck.py` still asked
`$TERM`, and **whim phase 19 had removed the `getenv("TERM")` the editor read it with** —
*the terminal is what the build says* — so all nineteen rows of
`.reference/zero-baselines/ref-term.txt` said `term=xterm-256color t_Co=256`: nineteen
ways of recording that the environment does nothing. It was content-free and measurably
so — **a prototype that deleted eight of the ten built-in terminal names and three of the
nine capability tables, 118 lines of terminal description, passed `tools/zcompare.py`
against the real baselines declaring nothing at all**. The question is now
`+set term={name}` on the command line, which reaches `did_set_term()`, and a refused
name answers `E522 term=<the terminal the editor stayed on> t_Co=…` — the error *and*
the terminal, which is what makes a refusal distinguishable from the old vacuous row.
**The new instrument is proven able to
fail and the old one proven not to be**, in one measurement: with one row deleted from
`builtin_terminals[]` the new table moves exactly 1 of 19 and the old one moves 0 of 19.
**And the prototype then became phase 121**, which is why those nineteen rows now read
**two names resolving to themselves, sixteen refused as `E522 term=xterm-256color
t_Co=256` and one `E529`**, the empty string, which `'term'` refuses before any table is
consulted.
**The re-record was safe because the baseline and every phase's recording move
together**, and `pipes/whim116.sh` measures that rather than citing it: `./zero-vim` out
of every recorded boundary tar **up to its own number** plus `whim-vim.c` built with
whim's own line gives one digest across every one of them, so `term-moved` stays
undeclared at every phase before it. The incantation, and **both halves are needed** —
measured, with only
`.cache/r0` removed `pipes/whim83.sh` refuses and names `ref-term.txt`, which is right —
is `rm -rf .reference/zero-baselines .cache/r0 && make whim-phase-83`.

**That bound is a repair, and the rule behind it is general.** The loop globbed
`.build-whim/r*.tar` and required one terminal table across **all** boundaries, which was
true when it was written and reaches boundaries that did not exist then — so phase 121
moving the table on purpose made **phase 116's** check fail, measured with 38's tar
present: *q121 records a different table:*, exit 1. **A phase may assert anything it likes
about the past; it may not assert that the future will not change what it measured.** And
the place it would have struck is worth knowing: `make whim-verify` could never have
caught it, because a verify scratch root links `tools/`, `pipes/` and the baselines and
has **no `.build-whim`**, so section 4 takes its "no boundary binary" arm there. It would
have struck a sequential `make whim-repass` in the repository root — the run that
*produces* boundaries rather than checking them, and the more expensive one to lose.

**A declared delta of none can be a harness that cannot see the phase**, and zero
phase 85 was the case: `behaviour.py` and `exsweep.py` run the editor `-e -s`, where
Ex mode takes `check_tty()`'s other branch, and `termcheck.py` drives a real pty
where both streams *are* terminals — so nothing recorded ever held those warnings.
Its check therefore builds the binary the phase was **handed** and requires the old
one to warn and pause (85 bytes of stderr, 2,010 ms) and the new one not to (0 bytes,
5 ms), with the 2,108-byte escape stream byte-identical. **When a phase's delta is
none because the harnesses are blind rather than because nothing moved, the phase
owes probes of its own** — and when a later phase gives the pipeline an instrument
that *can* see it, the delta is declared where it happened: phase 86 made
`2 stderr-moved` true and checkable, at q85 and at every boundary after it.

**Zero's instrument is the screen, and it is zero's own** (phase 86, `WHIM-PLAN.md` part II).
A recording is `tools/zrecord.sh`: keystrokes in a file on stdin, escape sequences
out on stdout, and a 24x80 screen rebuilt from them by `tools/zscreen.py` — snapshot
at every `\x1b[?25h`, which is where a redraw ends, and which is the only reason the
message line is recordable at all. **Six parts and 122 records**, where phase 86 made five
and 106: 102 keystroke cases that type their
own text under `'paste'` (`zcases.py`), every Ex command typed at `:` and recorded by
the message it prints (`zexcmds.py`), every command line the parser may see
(`zargv.py`), five pty scenarios for the window size, raw mode and a modified key
(`zpty.py`),
the terminal table (`ztermcheck.py`, whim's `termcheck.py` asked without a file
argument and, since phase 116, with `+set term={name}` instead of `$TERM` — see above),
and, since **phase 123**, sixteen memline cases of 200 to 25,000 lines built in the editor
(`zmemline.py`), which are the only part that can see the text layer as a tree.

**A memline record carries `stream N redraws` and no digest, and that is a defect phase
123 found rather than a shortcut.** `tools/zrec.py` scrubs the undo message's age padded to
the width it replaces, so the *screen* is protected — but the leak is **arithmetic on that
text's width**: an undo reports its age and the editor then positions the cursor to clear
the line, so `0 seconds ago` emits `\033[24;40H\033[K` and `1 second ago` emits
`\033[24;39H`, a column derived from a scrubbed string's length and living in bytes the
scrub never touches. **Hashing the scrubbed stream would not close it.** It failed zero
phase 99, whose binary is byte-identical either side, which is the only reason it was
catchable. What replaces the digest is stronger than one: both clocks the core can read
replaced by runaway counters move **0 of 16 memline records against 9 of the 102 screen
cases**, which says the record does not depend on the clock **at all**. `zcases.py` still
digests the raw stream and that is **open**: the `--- stream` line is named in eighty
files, forty-seven times in phase 95's check alone, so it is expensive rather than
difficult and deserves a pass of its own.

**Adding a part to a recording breaks every check that pinned its size, and there is no
way to add one that does not.** A new part is a new **file** whatever shape it takes, so
`zpty.py`'s precedent — one record however many scenarios it holds — could not be
followed; measured, phase 92 stopped with *a recording is 122 files, not the 106 this
phase counted*. Four phases had tested the count as an **equality** (92, 96, 113, 117) and
four as a floor of 100 (108, 118, 119, 120), and only the four equalities broke. They are now
**computed** — phase 92's control must mark *total − 2*, quiet only in `ref-pty.txt` and
`ref-term.txt`, and the other three take the floor their siblings already use — which is
this file's own rule that **a number a phase cannot move is reported and not pinned**.
The same rule is why `zpty.py` and not `zcases.py` got the `keymodel` repair's scenario.
It is deterministic — three recordings
per phase 83 and phase 86 run, identical, *including the sha256 of every stdout
stream* — and it is proven able to fail: `do_addsub()` returning `FAIL` moves exactly
11 of the 102 cases. The file-based `behaviour.py` and `exsweep.py` are untouched:
they are whim's and slim's, and phase 86 keeps `whimdelta.sh` as the one bridge
between the two pipelines' recordings.

**Three things differ from whim, and each was decided rather than inherited:**

- **The compile line is `gcc -O0 -fno-stack-protector -static -no-pie -s`.** The
  seed adds `-no-pie` (`tools/templates/zero.mk`), which is why `readelf -h zero-vim`
  says `EXEC` where whim-vim and slim-vim say `DYN`: see *The binary is standalone* in `CLAUDE.md`.
  Phase 84 adds `-fno-stack-protector` by editing the boundary's `zero/Makefile` —
  never the template, which is the pipeline's input and in q83's digest. The product
  rule cannot read `zero/`, so `whim.mk` states the flags once more, as `ZEROCFLAGS`
  and `ZEROLDFLAGS`, and `whim-pass` refuses to copy `zero-vim.c` out when they
  differ from the last boundary makefile's `CFLAGS` and `LDFLAGS` (proven to refuse).
  `make score` passes both to `tools/score.sh`, so zero's symbol count is taken with
  zero's flags.
- **Zero's behaviour is measured against its own baselines**,
  `.reference/zero-baselines`, which phase 83 records from `whim-vim.c` built
  with whim's compile line. So `pipes/zero.delta` starts empty and each zero phase
  declares only what it changes relative to whim, checked by `tools/zerodelta.sh`
  — `whimdelta.sh`'s rule and grammar against those baselines. Phase 83 also runs
  the harnesses on the `-no-pie` binary and requires no difference at all, and
  runs `tools/whimdelta.sh --phase 82` on it against slim-vim's baselines, which
  still holds: 489 commands and 11 cases, exactly whim's declared delta.
- **Zero's phase list is the `phases` line of `pipes/whim.stages`**, not a
  `PHASE_LIST` written into `tools/pipeline.sh`, because `pipeline.sh` is in every
  whim split key (above) and a zero phase added there would re-key all of whim.


## Adding a phase

Only on request, and one at a time. A new phase is the next whim phase — 129 is the
first — and the rest of this section is how it joins.

1. Write the program: `pipes/whim<N>-edit.sh <work> <state>` and
   `pipes/whim<N>-check.sh <work> <state>`, as `WHIM-GOAL.md`'s *Adding a phase*
   describes, with the body in `internal/edit/whim<N>.go` and
   `internal/check/whim<N>.go`.
2. **Declare its delta** in `pipes/zero.delta`, before running it: every phase from
   83 on is measured against `.reference/zero-baselines` with `tools/zrecord.sh`, and
   `tools/whimdelta.sh` hands it to `tools/zerodelta.sh`.
3. **Place it**: add N to the `phases` line of `pipes/whim.stages` — the phase list
   lives there and not in `tools/pipeline.sh`, so adding a phase moves no other key —
   then widen the last `each` stage to end at N (a split phase), or give it a
   `stage N` of its own (a single-file program, or a phase that must not re-run the
   last stage's checks). Inside an each stage every edit is swept before the next
   and every check sees its own tree, so write its `need` and `apart` lines anyway,
   measured: they are what a shared stage would have to respect. Then put it in a
   `package` with its `uses` lines. `tools/stages.sh whim
   --check` and `tools/packages.sh whim --check` must be silent.
4. Write its `## Phase N — ...` section here.
5. `make whim-tip` runs the last stage and records it; `make whim-verify` then proves
   every stage from the recorded one before it.
6. **`make whim-pass`, which is the step that copies the product out.** `whim-tip`
   records a boundary and nothing else; the tracked `whim-vim.c` is an **output** of the
   memoize and an input to nothing, so it can be arbitrarily wrong while every boundary
   reproduces. Measured, in the old zero pipeline: running only the first left the
   tracked product **two phases stale** — q119's 79,799 lines while the pipeline was at
   q121's 79,668 — and it was pushed in that state, with the verify reporting every
   boundary reproducing, correctly, throughout. `whim.mk` **warns**
   (`whim-product-check`) when the tracked file is not the last boundary's; it is a
   warning and not a failure because between a phase landing and `whim-pass` running
   the product is *expected* to lag.

## Phase 83 (zero 0) — the core's compile line, and the baselines it is measured against

**Since the merge this phase is not a seed.** It is handed q82's tree — `whim-vim.c`
and the makefile whim's phases carry, `tools/templates/whim.mk` — and there is no
committed file for it to `cmp` with. So step 1 below is now *build the tree it was
handed with the compile line it carries*: that binary is the one the zero baselines
are recorded from (step 3), and the phase then writes `tools/templates/zero.mk` over
the makefile and builds again (step 2). The baselines are therefore recorded from the
pipeline's own output at q82 — the one place it does so, and legitimate for the
reason recording from an input is: nothing from 83 on can reach q82. What it costs is
that a change to what phases 0-82 produce moves the recording, and this phase refuses
rather than overwrite it. `pipes/whim83.sh` carries the argument. What follows is the
phase as it was written.

`zero-vim.c` starts as a byte-for-byte copy of the committed `whim-vim.c`, and the
phase is `pipes/whim83.sh`, one whole program. Four things, each depending on the one
before:

1. **The seed is the input.** `cmp` against `whim-vim.c`; the boundary digest is the
   same file's.
2. **It builds, absolutely static.** `make -C zero` with `tools/templates/zero.mk`,
   `gcc -O0 -static -no-pie -s`, and then `readelf`: the type is `EXEC`, there is no
   `INTERP`, no dynamic section and no relocation. Measured on this input: 894,088
   bytes, where whim's static-PIE of the same source is 955,976 bytes, `DYN`, with a
   dynamic section and 1,986 relative relocations.
3. **The zero baselines are whim-vim's.** `whim-vim.c` is built in a scratch
   directory with `tools/templates/whim.mk` — whim's compile line — and the three
   harnesses whim's delta reads, `behaviour.py`, `exsweep.py` and `termcheck.py`,
   record it three times; the runs must be identical. If `.reference/zero-baselines`
   exists the recording must equal it, and it is never overwritten: a difference
   means a harness or the input changed, and has to be named. If it does not exist,
   it is written.
4. **zero-vim does exactly what whim-vim does.** `tools/zerodelta.sh zero/zero-vim
   zero/zero-vim.c --phase 83`, with `pipes/zero.delta` empty, requires no behaviour
   case, no Ex command and not the terminal table to move — so `-no-pie` changed
   nothing a harness sees. And `tools/whimdelta.sh --phase 82` on the same binary,
   against slim-vim's baselines, shows whim's whole declared delta still holds: the
   frozen whim behaviour is intact under the new compile line.

A tier-3 hit on this phase records nothing, because the phase does not run. So
`whim.mk` checks afterwards: `whim-pass`, and `zero-phase-N` — and through them
`whim-repass`, `whim-specpass`, `whim-tip` and the `zero-vim.c` rule — run
`whim-baselines-check` once the chain has reached its boundary, and it refuses unless
`.reference/zero-baselines` holds a non-empty `behaviour/`, `ref-exsweep.txt` and
`ref-term.txt`, naming the fix: `rm -rf .cache/r0 && make whim-phase-83`. The check
lives in `whim.mk`, which no implementation digest reads, so it moves no key.

## Phase 84 (zero 1) — the stack protector goes

`pipes/whim84.sh`, one whole program: there is no source edit, so there is nothing
for a sweep to do and a split phase would pay for one. `zero-vim.c` comes out of it
byte for byte as it went in, and what changes is one line of `zero/Makefile`:

```make
CFLAGS  = -O0                       ->  CFLAGS  = -O0 -fno-stack-protector
```

**Why.** gcc 15 on this machine enables `-fstack-protector-strong` by default, so
every function with a local array or an address-taken local gets a canary and the
object calls `__stack_chk_fail`. That is a symbol the core would have to be given by
its host, for a check the editor does not ask for — and *what it must be given* is
the number `WHIM-GOAL.md` measures. Measured on this input: the undefined symbols of
`gcc -c` on `zero-vim.c` go from **80 to 79**, the one that goes is
`__stack_chk_fail` and nothing comes, and the binary goes from **894,088 to 869,512
bytes**.

**Where the flag lives.** In the boundary — the tree a phase transforms — and not in
`tools/templates/zero.mk`, which is the pipeline's *input*: the input rule copies it
into `zero/Makefile`, and editing it would move q83's input digest and invalidate
phase 83's recording. The product rule in `whim.mk` cannot read `zero/`, which does
not exist in a checkout that only builds the committed `zero-vim.c`, so it states the
same flags once as `ZEROCFLAGS` and `ZEROLDFLAGS` — and `whim-pass` refuses to copy
`zero-vim.c` out when they differ from the `CFLAGS` and `LDFLAGS` of the makefile the
last phase left. The two statements cannot drift without a pass saying so; proven by
running `make whim-pass ZEROCFLAGS=-O0`, which refuses and names both. `make score`
passes both variables to `tools/score.sh`, which applies them to the object it counts
symbols in as well as to the binary — the count is otherwise taken with the default
CFLAGS, and would still show `__stack_chk_fail`.

**What the phase proves, in order:** the makefile has exactly one `CFLAGS` line and
it does not already carry the flag; the **old** flags do reference
`__stack_chk_fail` and the new ones do not — both measured with `nm -u` on unstripped
objects of the same source, so the check is one that can fail, and a compiler whose
default changed is reported rather than silently passing; `zero-vim.c` is unchanged;
the binary is still absolutely static (`EXEC`, no `INTERP`, no dynamic section, no
relocation); and `tools/zerodelta.sh --phase 84` sees no behaviour case, no Ex command
and no terminal-table row move against whim-vim's baselines. `pipes/zero.delta`
declares nothing for it, because a canary is code around the locals and not
behaviour.

It is `stage 84` and `package build` in `pipes/whim.stages`, with one `uses`:
`build:84 seed:83 mechanical`, because `zerodelta.sh` refuses without the
`.reference/zero-baselines` phase 83 records. It runs in 9 seconds.

## Phase 85 (zero 2) — the core stops diagnosing its own terminal

`pipes/whim85-edit.sh` and `pipes/whim85-check.sh`, `stage 85`, `package terminal`. The
first phase that cuts source, and the first piece of *a component, not a program*: a
host hands the core its input and output, and whether either is a terminal is the
host's business. Upstream's answer is to complain and then wait —

```
Vim: Warning: Output is not to a terminal
Vim: Warning: Input is not from a terminal
```

— on stderr, `out_flush()`, `exit(1)` if `--ttyfail` was given, and then
`ui_delay(2005L, TRUE)` so that a person can read them. All of it is
`check_tty()`'s second branch, and all of it goes, with the `--ttyfail` flag: its
`case '-'` test, the `parmp->tty_fail = TRUE` it set, and the `tty_fail` field of
`mparm_T`. `--ttyfail` becomes what any other unknown word is, `ME_UNKNOWN_OPTION`
through `mainerr()`.

**The pause was looked for, not assumed.** There are nine `ui_delay()` call sites in
`zero-vim.c` and exactly one is the warnings': 2005 ms, inside the branch that
printed them, under upstream's `scriptin[0] == NULL` ("do not pause while a script
is being read", which has nothing left to condition). The other eight are each a
different pause — 3001 ms for `W14: List of file names overflow`, 1002 ms for the
`'readonly'` warning, the 1000 ms slices of `ui_delay`'s own wait loop, 1003 and
3003 in `wait_return()`, 1006 in `check_for_delay()`, and `p_mat`'s two in
`showmatch()` — and none is reached from `check_tty()`. **Measured: the pause is
2,005 ms once, for both warnings together, not one per warning.**

**What stays, and the reader that forces each.** The `exmode_active` branch —
`if (!input_isatty) silent_mode = TRUE` — because Ex mode is a later phase, and it is
also what keeps `mch_input_isatty()` called. `stdout_isatty`, read outside `main` by
`out_redir = !stdout_isatty` in the message layer, which keeps its one assignment and
so `mch_check_win()` and that function's `isatty(1)`. `want_full_screen`, whose
second reader, `params.want_full_screen && !silent_mode`, survives the branch that
went. **So all five `isatty()` calls remain and the libc surface does not move:
79 undefined symbols before and after, the same set.** Folding an `isatty` caller is
a phase of its own if it is ever one; this phase is the warnings, the pause and the
flag. `check_tty()` then reads nothing from its argument, so it takes `void` and its
one caller drops the `&params` — the alternative being
`__attribute__((unused))` on a parameter nothing will read again.

**The delta is none, and every harness here is blind to it — so the phase's own
probes are the check.** `behaviour.py` and `exsweep.py` run the editor `-e -s`:
`exmode_active` is set before `check_tty()`, the first branch takes it, and the
warnings were never on any recorded stderr (grepped: the only baseline line
mentioning a terminal is `exsweep`'s `SKIPPED (hands over the terminal)`).
`termcheck.py` drives a real pty, where both streams *are* terminals. A delta of
"none" from a harness that cannot see the code proves nothing, so
`pipes/whim85-check.sh` measures the removed behaviour directly, in both directions:
the edit part first builds the binary the phase was **handed**, from the boundary's
own makefile flags, and every probe requires the old binary to do the thing and the
new one not to.

What they measured, `TERM=xterm`, stdin a file of `ihello world<Esc>:q!`, stdout a
file:

| | input binary | after |
| --- | --- | --- |
| stderr | 85 bytes, both warnings | **0 bytes** |
| elapsed | 2,010 ms | **5 ms** |
| stdout, the escape stream | 2,108 bytes | 2,108 bytes, **byte-identical** |
| exit | 0 | 0 |

`--ttyfail` exits 1 under both — the old binary because the flag asked it to, the new
one because the flag is gone — so the status is not the check and what `mainerr()`
prints is: `Unknown option argument: "--ttyfail"`, present after and absent before.
A bare `--` still ends the options, `+cmd` and `-T dumb` still work, each with the
same result from both binaries. And a real terminal is untouched: one `ptyrun`
session that types text, asks `:set term?` and `:wq` gives the same status, the same
file and the same answer either side — as do the 19 pty sessions `termcheck.py` runs
inside the declared delta.

**Measured, and what did not move.** 86,614 → 86,586 lines. The sweep found nothing
at all — no function, prototype, type, variable, field or enumerator — so the cut
orphaned nothing, which is what the kept readers above predicted. In the plain
object `.text` goes 654,846 → 654,576 bytes and `.rodata` 17,785 → 17,689, the two
strings and the branch; the stripped static binary is **869,512 bytes either side**,
the shrinkage absorbed by alignment padding. The phase runs in 28 seconds, 3.6 of
them the extra compile of the input binary its probes need. Its boundary is
`74ca3e1ffeb8`, and `make whim-verify` recomputes all three.

One `uses` line: `terminal:85 seed:83 mechanical`, for phase 84's reason —
`tools/zerodelta.sh` refuses without the `.reference/zero-baselines` phase 83 records,
and "none" is checked against them.

It does not run `tools/create_cmdidxs.py --check`, which every whim edit of the
command table ends with: the derived first-two-letters index went with the table whim
reduced, there are no `ex_cmdidxs.h` banners left in `whim-vim.c`, and the tool
raises rather than reporting nothing.

## Phase 86 (zero 3) — the instrument becomes the screen

**No source change at all**: `q86`'s `zero-vim.c` is `q85`'s byte for byte, and the
phase asserts it — the two boundaries have the same digest, `74ca3e1ffeb8`. What
changes is how every later phase is measured, and it had to change before those
phases are written rather than after.

### Why the old instrument stops working

`tools/behaviour.py` ends every case with `+w! <file>` and reads the file back;
`tools/exsweep.py` runs a command on a file and records the exit status and the
files left in the directory. Zero's editor is on its way to having **no file to
write, no file to read and no stream to print on** (`WHIM-PLAN.md` part II), so both stop
being instruments the moment the phases they exist to measure land. Waiting until
then would mean removing the filesystem and the means of noticing it in one step.

### What replaces it

`tools/zrecord.sh`: **keystrokes in on stdin, escape sequences out on stdout, and
a screen rebuilt from them**. No pty, no settle time, no ANSI stripping and no
Press-ENTER hazard; the terminal is 80x24 by construction because the window-size
ioctl fails on a pipe. Five parts, and a recording is all five — **six since phase 123**,
and 122 records where this phase made 106:

| | what it is | how big |
| --- | --- | --- |
| `screen/` | `tools/zcases.py`: 102 keystroke cases, one record each | 141 KB |
| `ref-excmds.txt` | `tools/zexcmds.py`: every Ex command name typed at `:` | 111 rows |
| `ref-argv.txt` | `tools/zargv.py`: every command line the parser may see | 30 rows |
| `ref-pty.txt` | `tools/zpty.py`: what only a real terminal shows | 4 scenarios, 5 since the `keymodel` repair |
| `ref-term.txt` | `tools/ztermcheck.py`: whim's `termcheck.py` with no file argument (phase 88) | 19 terminals |
| `memline/` | `tools/zmemline.py`, **phase 123**: buffers big enough to make the text layer a tree | 16 cases |

**One screen per redraw, taken from the bytes.** The editor hides the cursor while
it draws and shows it when the screen is settled, so `\x1b[?25h` is a step boundary
visible in the stream. That is what makes the message line recordable: the keys
that quit the editor wipe it, and with only the final screen every row of the
command sweep read `~`. It is also why the sweep is now a *message-level* record
where the file-based one was an exit status — retiring `:write` will move
`E32: No file name` to `E492`, which the old sweep could not have seen, both being
exit 1.

**A case types its own text under `'paste'`.** Nothing can load a file, so the seed
is typed — and typing is subject to the compiled-in `ai si et sts=4` and the four
mappings. `+set paste` (on the command line, before the first screen) turns exactly
those off, and a typed `:set nopaste` puts them back before the case's real editing,
which happens under the real defaults. `'paste'` and `+{command}` therefore survive
every zero phase by decision, and are named as such wherever a later phase might
take them.

**Two things are scrubbed, padded to the width they replace**: undo's
"1 second ago", which comes from `time()`, and `mainerr()`'s version banner, which
carries `__DATE__`. The padding is not cosmetic — the screen is columns, and a
shorter replacement moved the ruler into a different one.

### The delta grammar grows two dimensions

`case:NAME`, a command name, `argv:NAME`, `term-moved` and `pty-moved` name one
record each. `screen-moved` and `stderr-moved` name a **dimension** of every
record: what the editor drew, and what it wrote to stderr. A dimension token
excludes that dimension from every comparison and is itself checked — a phase that
declares `screen-moved` and draws the same screens fails, which was proven by
declaring it here and watching `tools/zcompare.py` refuse. Everything outside the
declared dimension is still compared record by record.

### The baselines are the input's behaviour, and the delta is cumulative

`.reference/zero-baselines` is recorded by **phase 83** from `whim-vim.c` built with
whim's own compile line — three recordings that must be identical — and is compared,
never silently overwritten. So the difference a phase declares is the difference
from the **input**, and the lines up to phase N are the whole of it, exactly as
whim's are against slim's baselines.

That is why phase 86 makes phase 85's delta visible. Phase 85 removed the two "not to
a terminal" warnings and the two-second pause, and declared nothing, because every
old harness ran the editor `-e -s` or on a pty and could not see them. The new
instrument runs it on a pipe, which is precisely where they were printed:
**`2   stderr-moved`** is the line, and it is checked at q85 and at every boundary
after it. Measured: all 102 cases, 109 of the 111 command rows (`:stop` and
`:suspend` are skipped) and 13 of the 30 command lines differ in their stderr **and
in nothing else** — the screens, the stream digests, the exit statuses, the bells,
the pty scenarios and the terminal table are identical.

### What the phase proves

1. the tree is untouched — `zero-vim.c` in, `zero-vim.c` out, same sha;
2. it builds with the boundary's flags and is still `EXEC`, no `INTERP`, no
   dynamic section, no relocation;
3. **the instrument is deterministic**: three recordings of that binary, identical,
   *including the sha256 of every stdout stream* — stronger than "the screens
   agree", since a redraw that draws the same result differently moves the digest;
4. **the instrument can fail**: a scratch copy of the source with `do_addsub()`
   returning `FAIL` — `CLAUDE.md`'s canonical break — moves **exactly 11 of the 102
   cases**, the ten that increment or decrement plus `mb_incr`, and nothing else.
   A corpus that cannot fail is not evidence;
5. the declared delta holds (`tools/zerodelta.sh --phase 86`);
6. **the bridge still stands**: `tools/whimdelta.sh` on the same binary against
   slim-vim's baselines gives whim's whole declared delta, 489 commands and 11
   cases. The file-based harnesses are kept untouched — they are whim's and slim's,
   and they are the only recording the two pipelines share. Nothing zero does from
   here reads them.

### Measured

One recording is **5.1 s** (its parts run at once; the 102 cases alone are 0.5 s
against a binary with no startup pause and 2.4 s against whim-vim, which still has
one). The phase runs in **30 s**, phase 83 in **33 s** with its three recordings and
the baseline write, and the whole four-phase pass cold in **1 m 45 s**;
`make whim-verify` reproduces all four boundaries in **36 s** of wall time over
109 s of phases. The recording is 180 KB on disk. No whim or slim cache key moved:
all 107 — 13 whim stages, 82 whim edits, 12 slim phases — are identical to `main`'s.

It is `stage 86` and `package harness` in `pipes/whim.stages`, with two `uses`
lines: `harness:86 seed:83 mechanical`, because it is measured against the baselines
phase 83 records *in the shape phase 83 now records them*, and `harness:86 terminal:85
rationale`, because the delta it proves is phase 85's.

**The old recording had to be removed once, by hand.** The two shapes have no file
in common, so phase 83 names the old one rather than printing a diff of everything
against everything: `rm -rf .reference/zero-baselines && rm -rf .cache/r0 && make
zero-phase-0`. It refuses rather than overwriting, which is the property that makes
the baselines a reference at all.

**Removing it means removing it, and that was got wrong once.** The new recording
was written *into* the old directory rather than in place of it, so
`.reference/zero-baselines` kept `behaviour/` and `ref-exsweep.txt` beside
`screen/` — and phase 83's `diff -r` then reported two extras on every run and
refused, while the "old shape" branch above did not fire, `screen/` being present.
The two are the pre-phase-3 file-based recording and nothing records them now;
deleting them is what phase 83 asks for when it says *name which before removing*,
and it makes `make whim-verify` reproduce q83 again. Phase 88 is where that was
found, because it is the first phase whose gate ran every boundary from the
recorded one before it.

## Phase 87 (zero 4) — no streaming Ex

`pipes/whim87-edit.sh` and `pipes/whim87-check.sh`, `stage 87`, `package streams`. The
second cut, and the first that removes a *mode*. Ex mode is the arrangement a core
does not have: the editor takes stdin over, prints its own prompt, reads a line at a
time and writes the result back on stdout. Silent mode comes with it — the message
layer redirected into `printf()` and stdout buffered through `setvbuf()` — and both
are entered from the command line (`-e`, `-E`, `-s`, `-v`) or from the keyboard
(`Q`, `gQ`).

`do_exmode()` (96 lines), `getexmodeline()` (263) and `nv_exmode()` (12) go, the
four option cases go, and then `exmode_active` (49 mentions) and `silent_mode` (23)
are constantly FALSE and fold at every reader. **The two counts are the whole
argument and are asserted both ways**: 49 and 23 before, every use gone after, with
the nineteen identifiers that go with them — counted by `\b`, because
`pending_exmode_active` contains `exmode_active` and a plain substring count says
53.

**Every fold is counted and scoped to one function, because the polarity is not the
same at every site.** `if (exmode_active)` folds never; `if (!exmode_active)` folds
always; and `msg_start()`'s `if (exmode_active != EXMODE_NORMAL)` folds **always**,
because `0 != 1` is TRUE. That one sits among its opposites and reads exactly like
them, and getting it backwards would have given every message Ex mode's newline,
with nothing in the build to say so. Two other shapes needed care: `fold_never` on
an `if` rewrites the `else if` after it into an `if`, so `main_loop`'s next anchor
is written without the `else` it had a moment earlier; and `command_line_scan` has
two `if (exmode_active)`, so the four option cases go first or the counted fold
refuses — loudly, which is the point.

**Three functions are deleted by name rather than left to the sweep.** A function
whose address is taken is reachable as far as gcc is concerned: `getexmodeline` is
passed to `do_cmdline()` and compared with `getline_equal()`. Its last live
reference is one disjunct of `do_cmdline`'s 200-column `while` condition — miss it
and 263 lines survive silently. `nv_exmode` goes the same way, because an
`nv_cmds[]` row is a reference: **the `'Q'` row is repointed at `nv_error` and never
deleted**, a hole being what moves every key past it onto another key's handler.

**Six write-only leftovers go by hand**, because nothing sees them:
`ex_pressedreturn`, `ex_no_reprint` (seven writes), `ex_exitval`,
`previous_got_int`, `use_plus_cmd` and `exmode_was`. A file-scope static that is
assigned and never read draws no warning at all, and the locals draw
`-Wunused-but-set-variable`, which `tools/deadsweep.py` does not act on.
`main_loop`'s `noexmode` parameter and the `theend:` label it jumped to go with
them — an unused label *is* a warning, and `tools/phasecheck.sh` fails on it.

**`check_tty()` is deleted here, and that is a gap in the sweep worth naming.**
Folding its one remaining branch leaves `int input_isatty; input_isatty =
mch_input_isatty();` — set and never read, which is exactly the kind
`deadsweep.py` does not act on. Measured: the sweep reports `left alone 1` and
settles with the warning still there. So the function and its call in `main()` go by
name, and `mch_input_isatty()` — whose only caller it was — is what the sweep takes,
with the fifth `isatty()` call. **It is phase 85's cut as much as this one's**: phase
85 kept that branch deliberately, saying Ex mode was a later phase's, which is why
the manifest carries `uses streams:87 terminal:85 mechanical`. `WHIM-PLAN.md` part II's table
gives `isatty` to P2; it arrives here.

**What stays, and the reader that forces each.** `getexline()`, because `:append`,
`:insert` and `:change` read their lines through it and not through the Ex-mode
reader. `exe_commands()`, because it runs the `+{command}` list every harness here
drives the editor with — only its last statement folds. And everything the argv
phase owns: `case NUL`'s `EDIT_STDIN` arm, `read_cmd_fd = 2`, `had_minmin`, the file
argument, `ME_TOO_MANY_ARGS` and its `main_errors[]` row, `case 'T'`. The check
names each of them.

### The declared delta, and the one the plan over-declared

Six records move, and every one is a way *in* to Ex mode: `case:key_Q`,
`case:key_gQ`, `argv:-e`, `argv:-E`, `argv:-e_-s`, `argv:-v`. The two keys drew
`Entering Ex mode.  Type "visual" to go to Normal mode.` and now beep once, from
`nv_error` and from `nv_g_cmd`'s `default: clearopbeep`; the command lines are
`mainerr(ME_UNKNOWN_OPTION)` like any other unknown letter.

**`-s` alone is not declared.** `case 's'` set silent mode only `if (exmode_active)`
and called `mainerr()` otherwise, so a bare `-s` was an unknown option *before* this
phase; measured, its record is byte-identical. `WHIM-PLAN.md` part II's P3 row lists it.
Two of the four that are declared — `-e` and `-e -s` — differ only in stderr, which
`stderr-moved` already excuses everywhere; they are named anyway, because they are
ways into Ex mode and this is the phase that closes them.

### The probes, and why a delta is not enough here

The baselines are one recording of one binary, so `tools/zerodelta.sh` can say
"exactly these six moved" and cannot say "the old binary entered Ex mode". The check
says it, by running both: the binary the phase was **handed**, built by the edit
part from the boundary's own makefile flags, and the one it made. **29 probes, in
two halves** — six required to move and 23 required not to — each of the six also
required to show Ex mode, or an option the old parser accepted, on the *old* binary.
A probe that only looks at the new binary passes on a phase that did nothing.

The 23 are `-s` alone, six `+{command}` forms, `:append`/`:insert`/`:change`,
`:visual`/`:vi`/`:view`/`:ex` from Normal mode (whose Ex-mode escape this phase
folded away), bare `-`, `--`, `-- +q!`, one and two file arguments, the three `-T`
spellings and an ordinary edit. Each record is built the way `tools/zcases.py`
builds one and scrubbed the same way, because `mainerr()` prints the version banner
and two binaries built a minute apart disagree on `__DATE__` for a reason that is
not the editor's behaviour — which is precisely why `-s` reads as unchanged and
must.

Two pty sessions beside them: `Q`, `visual<CR>`, `<Esc>:q!` — Ex mode entered by the
old binary and by nothing now — and an editing session identical either side. **The
`<Esc>` is not decoration.** Without Ex mode those six letters are Normal-mode keys
that end in Insert mode, `:q!` is typed into the buffer, and the session runs to the
timeout and is killed: status 9, measured, and it looked like a broken harness.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 86,586 | **85,813** (−773) |
| functions | 1,867 | 1,862 |
| `nm -u` | 80 | **78** — `setvbuf`, `stdout`, nothing else |
| `isatty(` calls | 5 | **4**, and the symbol stays |
| binary | 869,512 | **861,288** |

Five functions: `do_exmode`, `getexmodeline`, `nv_exmode` and `check_tty` by name,
`mch_input_isatty` by the sweep — which is clean in two rounds and also takes the
three single-constant enums `EXMODE_NORMAL`, `EXMODE_VIM` and `BO_EX`, each with an
explicit value, so nothing renumbers. In the plain object `.text` goes 654,576 →
648,968 and `.rodata` 17,689 → 17,625. `stdout` was the file's only mention, and it
was `setvbuf`'s argument.

The phase is **81 s** cold and 51 s with its edit cached; its boundary is
`97a2ab4895e7`, and `make whim-verify` recomputes all five in 81 s of wall time over
191 s of phases. No whim or slim cache key moved: all 107 are identical to `main`'s.

Its placement carries one thing the schedule does not need yet and will:
`apart 85 87`. Phase 85's check runs both of its binaries with `-e -s` and requires
exit 0, which is how it proves `--`, `+cmd` and `-T` still work — and this phase
removes `-e`. Every zero phase is a stage of its own today, so the line is a
statement; it becomes a constraint the moment two of them share a sweep.

## Phase 88 (zero 5) — argv is `+{command}` and `-T {term}`

`pipes/whim88-edit.sh` and `pipes/whim88-check.sh`, `stage 88`, `package streams`. A
core is handed its buffer by a host, not by a shell. What phases 85 and 87 left of
`command_line_scan()` is five things — `+cmd`, `-T`, a bare `-`, `--` and a file
argument — and this phase takes the last three, which are exactly the three that
name a **file** or a **stream** to edit. Everything the parser does not recognise
is now what every other unknown word already was, `mainerr(ME_UNKNOWN_OPTION)`.

**The file-argument arm is replaced, not deleted**, and that is not tidiness: with
no `else` at all a bare word matches neither `+` nor `-`, `argv[0][argv_idx]` is
not NUL for any word of more than one character, and the `while` never advances.
Deleting it gives an infinite loop, not an error. What goes with it is
`parmp->edit_type = EDIT_FILE`, the `vim_strsave()` and the `buflist_add()` that
put the name in the buffer list. `case NUL` — the bare `-`, with `EDIT_STDIN` and
`read_cmd_fd = 2` — and `case '-'` — where `--` set `had_minmin` and made every
later word a file name — fall to `default:` instead. `--foo` reached
ME_UNKNOWN_OPTION from *inside* `case '-'` and reaches it from `default:` now, so
only `--` itself changes.

**`ME_TOO_MANY_ARGS` had exactly those two call sites**, so it goes with its row in
`main_errors[]` — and **the enumerator is the row index**, so the three after it
move down by one. That is the renumbering `CLAUDE.md` warns about, done on purpose:
the table and the enum are rewritten from **one parse of both**, which is what
makes the mapping a fact rather than two edits that agree, and the check compares
the DWARF enumerator values of the binary the phase was handed with the ones it
made. Measured: **1,327 in, 1,323 out** — `ME_TOO_MANY_ARGS` and the three `EDIT_*`
gone, `ME_ARG_MISSING` 2→1, `ME_GARBAGE` 3→2, `ME_EXTRA_CMD` 4→3, and **1,320
unmoved**. A wrong index here shows up nowhere else: the build is perfectly happy
with it, and `-Txterm` would simply start printing another message.

`main_errors[]` keeps a **sixth** row, `"Invalid argument for"`, which no
enumerator named before this phase either. It is whim's leftover, and this phase
removes the row an enumerator it removes points at and nothing else.

**What `params.edit_type` then is: EDIT_NONE, for ever**, because nothing assigns
it. Its two readers are in `vim_main2()` and their polarity is opposite — `==
EDIT_STDIN` folds never, taking `read_stdin()`'s only call with it, and `!=
EDIT_STDIN` folds always, leaving `newline_on_exit` under the two conditions that
were already there. The field, the three `EDIT_*` enumerators, `read_stdin()` and
`buflist_add()` are then what the **sweep** takes: `deadfields.py` for the field —
there is no `ml_recover()` in this file, so a struct is no longer a disk format —
`deadenums.py` for the enumerators, and `deadsweep.py` for the two functions. Two
rounds, 79 lines.

### Where the line is against the later phases

Three things this phase could have taken and did not, each asserted by a **count**
so that taking them would fail here rather than widen quietly:

* **`readfile()`'s stdin half** is the "nothing reads a byte" phase's
  (`WHIM-PLAN.md` part II P8). What goes here is the *function* `read_stdin()`, argv's
  entry point into that code; the 23 remaining mentions of the name are the
  **parameter** of `readfile()`, `read_buffer()` and `open_buffer()`, and the check
  requires exactly 23.
* **`read_cmd_fd`** keeps its definition and its twelve remaining mentions. Only
  the assignment was argv's; nothing writes it now, so it is 0 for ever and folding
  it belongs with stdin. A file-scope static that is read and never written draws
  no warning, so the sweep would not have touched it either way.
* **The buffer's name** is P9's. Nothing here touches `b_ffname`, `b_sfname` or
  `b_fname`: what goes is the one call that ever gave the startup buffer a name
  from argv. `create_windows()` already opens an unnamed buffer when argv named
  none — that is `tools/zargv.py`'s `(none)` row — so the startup path is the one
  that was always there.

**There is no `usage()` to leave alone.** A phase that removes options usually owes
the help text an apology; `grep -i usage zero-vim.c` finds nothing at all, whim
having removed it, and `--help` is already `Unknown option argument: "--help"` in
the baselines.

### The declared delta: six command lines, and nothing else

`argv:-`, `argv:--`, `argv:f.txt`, `argv:f.txt_g.txt`, `argv:+q!_f.txt` and
`argv:--_+q!` — six of `tools/zargv.py`'s 30 rows, every one a way of naming a file
or a stream:

* **`-` was the row that blocked.** The editor read the keystroke file itself as
  buffer text, closed fd 0, duped stderr and waited there for keys that never came;
  the baseline record is `blocked` and it cost the harness its timeout on every
  run. It is `Unknown option argument: "-"`, exit 1, now.
* **`f.txt` and `+q! f.txt`** opened a buffer and drew a screen; both are exit 1
  with an empty stream.
* **`--` and `-- +q!` disagreed with each other** in the baselines, because `+q!`
  after `--` was a *file name* rather than a command. They agree now, and that
  difference is the whole of what `--` did.
* **`f.txt g.txt` is declared although `stderr-moved` would absorb it**: it exited
  1 with an empty stream for `Too many edit arguments: "g.txt"` and exits 1 with an
  empty stream for `Unknown option argument: "f.txt"`. It is the two-file-argument
  row and this is the phase that removes file arguments, so the list says so rather
  than letting a dimension cover it — phase 87's reason for naming `-e`.

**Nothing else moves, and the corpus is the reason it cannot**: every one of the
102 screen cases seeds itself by *typing* under `'paste'`, so not one passes a file
argument. Measured: 102/102 cases, 111/111 Ex-command rows, the four pty scenarios
and all 24 other command lines identical — including every `+{command}` form and
all three `-T` spellings.

### The instrument this phase broke, and the one that replaced it

**`tools/termcheck.py` asks its question with a file argument.** It is whim's and
slim's, the one harness zero kept (phase 86), and it opens a three-line `f.txt` so
that the screen has something on it before `:set term? t_Co?`. From this boundary
that argument is an unknown option, the editor exits 1 before drawing, and **all
nineteen rows read `(none)`** — measured. That is the harness failing, not the
terminal table moving, and declaring `term-moved` for it would switch the terminal
table off for every phase after this one, which is the one thing a phase must not
buy its way out with.

So zero's recording now uses **`tools/ztermcheck.py`**: `termcheck.py` imported
with its `ask()` replaced and nothing else, so the terminal list, the environment
isolation, the settle ladder and the output format stay in one place and cannot
drift from whim's. Editing `termcheck.py` itself is what core rule 9 forbids — it is
named by `tools/whimdelta.sh` and `tools/verify.sh`, so its bytes are in every whim
stage's key. **The swap is proven, not asserted**, in three places: the new tool
records `.reference/zero-baselines/ref-term.txt` byte for byte from the binary this
phase was *handed* (which still accepts a file argument, so both forms work on it),
the old tool records nineteen `(none)` rows from the one it *made*, and phase 83 —
which records from `whim-vim.c` three times and compares with the baselines —
reproduces `q83` unchanged under the new instrument. Only `tools/zrecord.sh` changed
to name it, which re-keyed zero's five earlier phases and **no whim or slim key**:
all 107 are identical to `main`'s.

### The probes, and why the delta is not enough

The baselines are one recording of one binary, so `tools/zerodelta.sh` can say
"exactly these six moved" and cannot say "the old binary opened the file". The
check says it, by running both — the binary the phase was **handed**, built by the
edit part from the boundary's own makefile flags, and the one it made. **22 probes,
six required to move and 16 required not to**, and each of the six is also required
to show the old behaviour on the *old* binary: a buffer drawn and exit 0 for
`f.txt`, `Too many edit arguments` for two of them, `blocked` for the bare `-`, and
`--` reading differently from `-- +q!`. Proven able to fail in both directions: run
with the old binary on both sides all six report *was to move and did not*, and
with the new binary on both sides they add *the input binary already refused it, so
this proves nothing*.

The 16 are `+`, `+q!`, `+set nu`, two `+{command}`s at once, **`+set paste`** with
typing under it — `'paste'` and `+cmd` are what the whole corpus is seeded with and
this is the phase that could have lost both — the three `-T` spellings and an
unknown terminal name, the four options that were already unknown, and an ordinary
keystroke edit. Two pty sessions beside them: `vim f.txt` on a real terminal, which
the old binary edits and the new one refuses with wait status 256, and an editing
session with no arguments, identical either side.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 85,813 | **85,734** (−79) |
| functions | 1,862 | 1,860 |
| enumerators (DWARF) | 1,327 | **1,323** |
| `nm -u`, zero's flags | 77 | **77**, the same set |
| `nm -u`, as `phasecheck.sh` counts it | 78 | 78 (the extra is `__stack_chk_fail`) |
| `.text` / `.rodata` of the plain object | 648,968 / 17,625 | 648,496 / 17,609 |
| binary | 861,288 | **861,288** |

**Nothing is freed, and that is the measurement rather than a disappointment**:
`read_stdin()`'s `close()` and `dup()` have other callers and `buflist_add()` names
no libc directly, so the undefined set is equal — stated as an equality, so a
symbol *arriving* would fail. The binary does not move either: 472 bytes of `.text`
and 16 of `.rodata` go, and alignment padding absorbs them, exactly as in phase 85.

The phase is **52 s** cold and 57 s under `make whim-verify`, which recomputes all
six boundaries in 80 s of wall time over 247 s of phases. A `make whim-repass` from
an empty zero cache ran the first five in **152 s** — 30, 12, 28, 31, 51 — and took
phase 88 from tier 3, so the whole pipeline cold is 204 s; every boundary matched
its recording. Its own is `d466c3b9245b`.

Its placement carries one new constraint, and it is measured rather than predicted:
**`apart 87 88`**. Phase 87's check names `EDIT_STDIN`, `read_cmd_fd = 2`,
`had_minmin`, `buflist_add` and `ME_TOO_MANY_ARGS` one by one and requires each to
be *there* — "it is the argv phase's to take" — and this phase takes all five. Run
on a phase 88 tree it says `'EDIT_STDIN' went, and it is the argv phase's to take`
and exits 1. Phase 85's check would fail on a phase 88 tree too, but a stage holding
85 and 88 holds 87, and `apart 85 87` already forbids that.

Two `uses` lines: `streams:88 seed:83 mechanical`, because the six declared records
are compared with the baselines phase 83 records, and `streams:88 harness:86
mechanical`, because an argv record is something a zero recording only has from
phase 86. Phase 87 is in the same package, so the ordering between them is the
package's and not a `uses`.

## Phase 89 (zero 6) — no write

`pipes/whim89-edit.sh` and `pipes/whim89-check.sh`, `stage 89`, `package files`. A
core does not own a disk: reading and writing files is the host's business, and
this is the first half of taking the filesystem away. The six Ex commands that
put bytes on one go — `:write :wq :xit :exit :update :saveas` — and with them
everything only they reached.

### Four anchors, and not one fold

The phase is four edits, and every removal after them is the sweep's
(core rule 1). The `cmdnames[]` row is the only reference a command handler has, so
taking the row is what makes the handler unreachable:

1. the six enumerators of `enum CMD_index`, one line each;
2. the six `cmdnames[]` rows, one physical line each, designated `[CMD_x] = {`;
3. `nv_Zet`'s `ZZ`, which runs the command *string* `"x"` → `"q!"`;
4. `do_one_cmd`'s `:w>>` / `:w!` parse — an `if (ea.cmdidx == CMD_write ||
   ea.cmdidx == CMD_update) {…}` with no else — deleted as **text** rather than
   folded, because its condition names two of the enumerators that are going. It
   has to go in the same edit as anchor 1 or nothing declares what it reads.

**The alternative was measured.** An edit that also deletes `ex_write`,
`ex_update`, `ex_exit`, `do_write`, `check_writable`, `check_overwrite`,
`not_writing` and `check_readonly` by name produces a **byte-identical swept
file**, in 3 sweep rounds against 4 and 17 seconds against 22. Four seconds is
not a reason to write eight names into a phase program, so the minimal edit is
what runs.

**The text the edit leaves does not compile, and the program says so.** Six
mentions of the six enumerators survive it — `CMD_saveas` five times and
`CMD_wq` once — every one inside `ex_write`, `do_write` or `ex_exit`. The edit
asserts exactly that, as a computation rather than a list: each survivor is
inside a function definition, and no surviving `cmdnames[]` row names that
function, which is the whole argument that `funcreach.py` takes them in the
sweep's first round. `tools/phasecheck.sh` in the check is where *it compiles*
is asserted.

### `ZZ` is `ZQ`, and that is a decision

`nv_Zet` runs a command **string**, so nothing here breaks at compile time:
left alone, `ZZ` would type `:x` at a command that no longer exists and answer
`E492`. `case:zz_key` therefore moves whatever is done — `E32` today, `E492` if
the string is left, nothing at all with `"q!"` — so this phase owns it rather
than leaving a dead command named in the source. It is the user's settled
decision that ZZ is ZQ. `WHIM-PLAN.md` part II gives it to the `:q` phase; that row is
annotated as built.

### No DWARF dump, and phase 88's reason for one does not apply

Deleting the six renumbers **89** survivors, and every one is a `CMD_*`.
Measured with `tools/enumvals.sh`: **1,323 enumerator values in, 1,303 out** —
the six, plus fourteen single-constant explicit-value enums the sweep takes with
their types (`CPO_FNAMEAPP CPO_FNAMEW CPO_FWRITE CPO_KEEPRO CPO_OVERNEW
CPO_PLUS NODE_NORMAL NODE_OTHER NODE_WRITABLE SHM_WRI SHM_WRITE SMALLBUFSIZE
TRUNC_ON_OPEN WRITEBUFSIZE`) — 89 moved and nothing else touched, nothing
arriving.

Phase 88 compared DWARF either side because `main_errors[]` was a table written
in its enumerators' order, where a wrong index was invisible to the build.
`cmdnames[]` is **designated**: a row lands at its own enumerator whatever the
numbering is, the `static_assert` on the row count catches a dropped pair, and
all 105 surviving names are dispatched by `tools/zexcmds.py` inside the declared
delta. Three checks the build cannot dodge, and none of them needs the values.

**The row floor now has five rows of margin.** `cmdnames[]` goes 111 → 105 and
`tools/create_cmdidxs.py`'s `names()` refuses a table of fewer than 100 — a
regex that stops matching otherwise yields a plausible all-zero index, so the
floor is deliberate. `tools/zexcmds.py` enumerates the table through it, so
crossing it would stop zero's command sweep rather than give a wrong answer.
`WHIM-PLAN.md` II.3a: the `:edit` phase spends the margin, and it is the phase that
must lower the floor.

### Two traps, and both make the obvious check the wrong one

- **`check_readonly` is also a local**, in `readfile()`: `int check_readonly;`
  and three uses. After the phase `grep -cw` is **4, not 0**, so a phase-4-style
  "every name at zero mentions" loop fails on a correct phase. What must be gone
  is the definition, `^check_readonly(`, and the four survivors are required to
  be inside `readfile()`.
- **`"write"` survives**, as the `'write'` option's name, and `E32: No file
  name` with it — still reachable through `check_fname()` from `do_ecmd()` and
  `ex_bang()`. A "no mention of write anywhere" check fails on a correct phase
  just as surely.

### The declared delta, and what the corpus cannot see

`6   case:cmd_write case:zz_key` and the six rows `write wq xit exit update
saveas`. The two kinds of movement are different: `cmd_write` goes from `E32: No
file name` to `E492: Not an editor command: write`, `zz_key` loses its bell,
its `E32` and two snapshots — and the six command rows **cease to exist**,
because `tools/zexcmds.py` enumerates 105 names where it enumerated 111.
Measured with `tools/zcompare.py`: the other 100 screen cases, the other 105
command rows, all 30 command lines, the four pty scenarios and the terminal
table are identical.

**And that is the whole of what any recording here can see.** Every one of the
102 screen cases types its own text and names no file, so `cmd_write` types
`:write` with no file name and what the baselines hold is an editor that
**failed** to write. "cmd_write and zz_key moved" is equally consistent with a
phase that changed one error message and left `buf_write()` reachable.

### The probes, which are the only evidence writing went

**26, on both binaries** — the one the phase was handed, built by the edit part
from the boundary's own makefile flags, and the one it made — **in a directory
they keep**. `tools/zstream.py`'s `session()` throws its run directory away,
which is the one thing a phase about files cannot do, so the runner is in the
check and adds one section to the record: what the run left on the disk.

- **`write_roundtrip`** types `WROTEME`, writes it to `out.txt`, empties the
  buffer and reads the file back. The old binary answers `"out.txt" 1L, 8B`; the
  new one `E484: Can't open file out.txt` and an empty buffer. It leans on
  `:read`, which the next phase removes — harmless, because `make whim-verify`
  runs every check on its own boundary.
- **`:w :sav :update :wq :x` with a file name**: `{'out.txt': 6}` on the old
  binary and `{}` on the new, for all five, with `:wq` and `:x` going exit 0 → 1.
- **Eight spellings** — `:w :x :wq :up :sav a :w! :w >>f :w !cat` — each E492
  now and none before, which is the inheritance check `CLAUDE.md`'s `:help` →
  `:helpclose` trap asks for.
- **Ten that must not move and do not**: `:q` on a modified buffer (still E37 —
  `check_changed` stays and is the `:q` phase's), `:q!`, `:read` (still E32, the
  read phase's), `:edit`, `:file`, `:%!sort`, `:s/x/y/`, `u`, CTRL-G and an
  ordinary edit.
- **A pty session**, because every probe above went through a pipe: `:wq
  out.txt` writes the file and quits on the old binary and answers E492 here,
  and an editing session is identical either side.

**Proven able to fail in both directions**: with the old binary on both sides
all sixteen report *was to move and did not*; with the new binary on both sides
they add *the input binary left {}, not a 6-byte out.txt, so this proves nothing
about writing*.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 85,734 | **84,675** (−1,059) |
| functions | 1,860 | 1,841 (−19) |
| enumerators (DWARF) | 1,323 | 1,303 |
| `cmdnames[]` rows | 111 | **105** |
| `nm -u`, zero's flags | 77 | **71** |
| `nm -u`, as `phasecheck.sh` counts it | 78 | **72** |
| `.text` / `.rodata` of the plain object | 648,496 / 17,609 | 640,449 / 17,289 |
| binary | 861,288 | **847,656** |

**The six that go are `chmod fchmod fstat ftruncate lstat unlink`**, and they
are the first any zero phase has freed. The check states the set rather than a
count, and requires `stat`, `open`, `access`, `fsync` and `getcwd` to be
**still** undefined, so a cut reaching into a later phase fails here rather than
widening quietly: `stat` is down from 14 calls to 8 and belongs to the phase
that gives up the buffer's name, `open` and `access` to the one that stops
reading a byte, `fsync` to the options.

Nineteen functions go, none of them named by the edit — `ex_write ex_update
ex_exit do_write check_writable check_overwrite not_writing check_readonly
check_file_readonly buf_write buf_write_bytes check_mtime time_differs
write_eintr vim_fexists mch_setperm mch_fsetperm mch_nodetype u_update_save_nr`
— with one struct field, `exarg_T.append`, and 39 string literals. The sweep is
**4 rounds, 22 s**, and the phase **46–49 s**. Its boundary is `8ce685cf2592`,
and `make whim-verify` recomputes all seven in 80 s of wall time over 299 s of
phases.

**What no instrument here sees, said out loud.** `p_fs`, `p_write` and `p_wa`
lose their last readers and keep their rows and their `:set` answers — removing
a row is the options phase's, and `tools/orphanopts.py` refuses a global whose
row has gone, so they are asserted at exactly two mentions each. Six
`'cpoptions'` and two `'shortmess'` letters lose their readers with no
observable change, the validity lists being string literals. `'readonly'` keeps
its `W10` warning and its `[RO]` indicator. And the row floor now has five rows
of margin rather than eleven.

### Its placement

`stage 89`, `package files`, and two `uses` lines: `files:89 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records,
and `files:89 harness:86 mechanical`, because the old file-based sweep recorded an
exit status and `:write` went from E32 to E492 without changing it — phase 86's
message-level record is what can see this phase at all.

**`apart 88 89` is measured rather than predicted.** Phase 88's check states that
*it* frees no libc symbol, as a `cmp` against the stage's starting undefined
set, and inside a stage every check compares with the **stage's** start. Run as
one stage — `tools/phaserun.sh whim 88-89` — phase 88's check fails with *the libc
surface moved, and this phase frees nothing* and names all six.

**There is deliberately no `apart 85 89`.** Phase 85's check drives a pty with
`:wq` and reads the file back, which this phase would break — but measured, it
already fails identically on a phase **5** tree (status 256, the file
unchanged), because the session opens `f.txt` as a file *argument* and never
reaches the `:wq`. So the failure at 89 is phase 88's, a stage holding 85 and 89
holds 87, and `apart 85 87` forbids it already. Phase 87's check fails on a phase 89
tree for the reasons `apart 87 88` records.

## Phase 90 (zero 7) — no read

`pipes/whim90-edit.sh` and `pipes/whim90-check.sh`, `stage 90`, `package files`. The
other half of taking the filesystem away. Phase 89 removed the six commands that put
bytes on a disk; this one removes the command that takes them off it on request —
`:read` — and with it the `:r !cmd` arm, which was the last caller of the filter and
shell plumbing whim left as stubs. What is left of reading a file is `readfile()`
itself, which the startup path still uses, and this phase asserts by count that it
is untouched.

### Three anchors, and one fold that is a judgement

1. the `CMD_read` enumerator of `enum CMD_index`, one line;
2. the `cmdnames[]` row, one physical line, designated `[CMD_read] = {`;
3. `do_one_cmd`'s `if (ea.cmdidx == CMD_read) {…}` — the parse that turns `:r!` and
   `:r !cmd` into a filter — deleted as **text** rather than folded, because its
   condition names the enumerator that is going, and in the same edit as anchor 1.

**No handler is named.** The row is the only reference a command handler has, so
taking the row is what makes `ex_read` unreachable, and `do_bang`, `do_shell`,
`do_filter`, `check_secure` and `prevcmd_is_set` follow it — `:!` has not existed
since whim, and phase 89 swept `ex_write`, which held `do_bang`'s other call.

**The fold is `exarg_T.usefilter`, and it is the one thing here no tool could have
found.** Phase 89 removed one of its two writers (`:w >>`, `:w !cmd`) and anchor 3
removes the other, so after the edit the field is **written nowhere** — and
`do_one_cmd` memsets the struct, so all seven readers are constantly FALSE.
`tools/deadfields.py` removes a field nothing *names*, and gcc has no warning for a
member that is only read, so neither would ever have reported it. The six
surviving tests are folded with their polarity stated — two `&& !ea.usefilter`
conjuncts and one `|| ea.usefilter` disjunct in `do_one_cmd`, a
`!eap->usefilter &&` and the whole `if (eap->usefilter && strpbrk(repl, "!"))` arm
and one more conjunct in `expand_filename` — and the field goes with them.
**Measured both ways**: the fold costs 13 lines and gives a **byte-identical
recording**, because it removes tests whose answer was already fixed.

**The text the edit leaves does not compile**, and the edit says so as a
computation rather than a list: one mention of `usefilter` survives, in `ex_read`,
whose only reference was the row that just went, and no surviving `cmdnames[]` row
names that function — which is the whole argument that `funcreach.py` takes it in
the sweep's first round. `tools/phasecheck.sh` in the check is where *it compiles*
is asserted. It is phase 89's shape exactly, one name instead of six.

### The traps, which make the obvious check the wrong one

- **`secure` is not `check_secure`.** The function goes; the variable keeps
  **eleven** mentions, being the vimrc and tag-search flag half the editor tests.
  Only the two inside `check_secure()` went. A copied "every name at zero" loop
  fails on a correct phase.
- **The bare word `read` survives three times** — two `read(fd, …)` calls and an
  E222 string — and `readfile` 5, `read_buffer` 17, `open_buffer` 6, `read_edit` 2,
  `readonly` 4, `shell` 1 and `filter` 2. The eight names that genuinely reach zero
  are `ex_read do_bang do_shell do_filter check_secure prevcmd_is_set prevcmd
  CMD_read`.
- **`E32: No file name` survives and `E484: Can't open file` does not.** E32 is
  still reachable through `check_fname()` from `do_ecmd()`; `ex_read` was E484's
  last speaker, and after this phase nothing in the file says it. Both are asserted,
  in opposite directions, with `"read"`, E12, E34 and E319 — the four strings the
  five swept functions were the last to say.

**No DWARF dump, for phase 89's reason.** Deleting one enumerator renumbers 46
survivors and every one is a `CMD_*`; `cmdnames[]` is designated, the
`static_assert` on the row count catches a dropped pair, and all 104 names are
dispatched by `tools/zexcmds.py` inside the declared delta. Measured with
`tools/enumvals.sh` anyway, once, for this document: **1,303 values in, 1,302 out**,
`CMD_read` the only one gone, 46 moved, nothing arriving.

**The row floor now has four rows of margin.** `cmdnames[]` goes 105 → 104 and
create_cmdidxs's `names()` refuses a table of fewer than 100. `WHIM-PLAN.md` II.3a: the
`:edit` phase spends the rest, and it is the phase that must lower the floor.

### The declared delta, and what the corpus cannot see

`7   case:cmd_read case:read_cmd_gone` and the sweep row `read`. `cmd_read` types
`:read` **with no file name**, so what the baselines hold is an editor that *failed*
to read, `E32: No file name`; it is `E492: Not an editor command: read` now.
`read_cmd_gone` types `:r !echo piped`, which whim's stub answered with
`E319: Sorry, the command is not available in this version`; it is E492 now, one
snapshot fewer and the same single bell either side. The `read` row does not change
message — it **ceases to exist**, `tools/zexcmds.py` enumerating 104 names where it
enumerated 105.

**`filter_gone` is not declared, and `WHIM-PLAN.md` part II's P6 row over-declares it.**
`:!` has not existed since whim, so `:%!sort` already answered E492 on the input
binary and its record is byte-identical. What this phase removes is the code behind
a command that was already gone — which the check asserts, by requiring E492 on
*both* binaries.

Measured with `tools/zcompare.py`: the other 100 screen cases, the other 103 command
rows, all 30 command lines, the four pty scenarios and the terminal table are
identical.

### The probes, which are the only evidence reading went

**25, on both binaries** — the one the phase was handed, built by the edit part from
the boundary's own makefile flags, and the one it made — eleven required to move and
fourteen required not to.

**The file they read is `keys` itself.** `tools/zstream.py` writes a session's
keystrokes into a file called `keys` in the run directory and feeds it on stdin, so
there is always one file there and no runner has to plant one: `:r keys` reads it
back. **What proves the bytes arrived is the Escape in them** — the keystroke file
holds `…\x1b:q!\r`, which `tools/zscreen.py` draws as `^[:q!^M`, and an Escape can
only be in the buffer if the file was read. The *message* is not the check: `:1r
keys` reads the file and leaves the message line blank, measured, so only `:r keys`
is asked for `"keys" [noeol] 1L, 33B`.

- **`r_keys` and `r_range`** (`:r keys`, `:1r keys`): the keystrokes in the buffer
  on the old binary, E492 and nothing read on this one.
- **`r_missing`** (`:1r nosuch`): `E484: Can't open file nosuch` before, E492 now —
  the probe that pairs with E484 having no speaker left in the source.
- **`r_bang`** (`:r !echo piped`): **E319 in the stream** on the old binary and not
  here. It is in the stream and never in a snapshot, because the message is drawn, a
  `Press ENTER` prompt follows and the next redraw wipes the line before the cursor
  comes back, which is where `tools/zscreen.py` takes its picture. Its presence on
  the old binary is also the proof that no shell ever ran: the stub refused before
  one could.
- **Five spellings** — `:r :re :rea :r! :r !cat` — each E492 now and none before,
  which is the inheritance check `CLAUDE.md`'s `:help` → `:helpclose` trap asks for,
  with `:redo`, `:redraw`, `:registers` and `:reg` required not to move at all.
- **Fourteen that must not move and do not**: `:%!sort`, `:redo`, `:redraw`,
  `:registers`, `:reg`, `:undo`, `:edit`, `:print`, `:append`/`:insert`/`:change`,
  `:q` on a modified buffer (still E37), `:q!` and an ordinary editing session. The
  ones that must not move are required to be *doing* something — `:append` shows its
  added line, `:registers` prints its table **in the stream**, for E319's reason.
- **Two pty sessions**, because every probe above went through a pipe: `:r
  planted.txt`, where **the runner writes the file** — the editor has had no way to
  write one since phase 89 — which the old binary reads into the buffer and this one
  answers E492; and an editing session identical either side.

**Proven able to fail in both directions**: with the old binary on both sides all
eleven report *was to move and did not*, and with the new binary on both sides they
add *the keystroke file did not reach the buffer on the input binary, so this proves
nothing about reading*. Both pty sessions fail the same way round.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 84,675 | **84,453** (−222) |
| functions | 1,841 | 1,835 (−6) |
| enumerators (DWARF) | 1,303 | 1,302 |
| `cmdnames[]` rows | 105 | **104** |
| `nm -u`, zero's flags | 71 | **71, the same set** |
| `nm -u`, as `phasecheck.sh` counts it | 72 | 72 |
| `.text` / `.data` of the plain object | 640,449 / 38,923 | 638,960 / 38,699 |
| binary | 847,656 | **847,368** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set — so a symbol *arriving* would fail too. `:read` reached `readfile()`,
which the startup path still uses, and the shell stubs never called a shell: this
phase removes two commands and not the read path, and `open`, `read`, `close` and
`stat` are required to be **still** undefined, `WHIM-PLAN.md` part II P8 being the phase
that frees them. `.rodata` does not move at all and `.data` loses 224 bytes,
because the five strings are `static char e_…[]` arrays and not `const`.

Six functions go, none of them named by the edit — `ex_read do_bang do_shell
do_filter check_secure prevcmd_is_set` — with three prototypes and five file-scope
variables: `prevcmd` and the four error strings, `"read"` having gone with the row.
The `usefilter` field is the edit's, and the only removal this phase names. The sweep is
**3 rounds, 19 s**, and the phase **43–47 s**. Its boundary is `8f1a98bef913`, and
`make whim-verify` recomputes all eight in 80 s of wall time over 344 s of phases.

### Its placement

`stage 90`, `package files`, and two `uses` lines: `files:90 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records, and
`files:90 harness:86 mechanical`, because the old file-based sweep recorded an exit
status and `:read` goes from E32 to E492 without changing it — phase 86's
message-level record is what can see this phase at all. **There is no `files:90
files:89` line**: `tools/packages.sh --check` refuses a `uses` inside one package,
the ordering between two phases of the same package being the package's.

**`apart 89 90` is measured rather than predicted.** Phase 89's check names `do_bang`
among the things a later phase takes — "it is a later phase's" — and requires
`cmdnames[]` to hold 105 rows. Run on the tree this phase leaves it answers
`do_bang went, and it is a later phase's` and `cmdnames[] has 104 rows and names()
reads 104; both must be 105`, and exits 1. Its `write_roundtrip` probe reads the
file back with `:r out.txt` and would fail too; the source assertions come first.

**And `need 90 swept`, which is the first `need` the zero manifest has.** The edit's
anchor is `usefilter` at exactly 10 mentions — the field, the two writes anchor 3
removes and seven reads. On the text phase 89's *edit* leaves there are **eleven**,
`ex_write` still being there to read one, and the counted anchor refuses: measured
with `tools/phaserun.sh whim 89-90`, which says `usefilter has 11 mentions, expected
10`. The same run shows the edit's build of the input binary failing on phase 89's
non-compiling intermediate, which is true of every zero edit that builds one and is
not declared for that reason.

## Phase 91 (zero 8) — no `:edit`, and no `gf`

`pipes/whim91-edit.sh` and `pipes/whim91-check.sh`, `stage 91`, `package files`. Phases
89 and 90 took the commands that put bytes on a disk and the one that takes them off
it. This one takes the commands that point the editor **at** a file — `:edit :enew
:ex :visual :view` — and the four Normal-mode keys that do the same thing from the
buffer's own text, `gf gF [f ]f`. What is left of opening anything is `readfile()`
and `open_buffer()`, which the startup path still uses, and this phase asserts by
count that it has not reached them.

**What the five commands were, measured from outside rather than read.**
`do_exedit` is thirty lines after phase 87 — a lock guard, a `readonlymode`
save/set/restore testing `CMD_view` and `CMD_enew`, `setpcmark()` and one
`do_ecmd()` call — so on the input binary, in a directory holding a file called
`keys`: `:e! keys` loads it (`"keys" [noeol] 1L, 30B`), `:ex! keys` and `:visual!
keys` do exactly the same, `:view! keys` loads it **and makes `:set ro?` answer
`readonly`**, and `:enew!` empties the buffer. One handler, `ex_edit`, is all five
rows, which is why the five go together.

### Six anchors, and the sixth is measured

1. five enumerators of `enum CMD_index`, one line each;
2. five `cmdnames[]` rows, one physical line each, designated `[CMD_x] = {`;
3. `do_one_cmd`'s `curbuf_locked()` exemption: the `ea.cmdidx != CMD_edit` conjunct
   goes and **`CMD_file` stays**, `:file` being the buffer-name phase's. It must go
   in the same edit as anchor 1, and it is **the one anchor outside the table and
   the keys** — an edit shaped like the table forgets it, and the build is what
   catches that;
4. `nv_g_cmd`'s `case 'f': case 'F': nv_gotofile(cap); break;` arm. `case 'f':`
   alone occurs six times in that function and the four-line block once;
5. the same call in `nv_brackets`;
6. `do_one_cmd`'s `if (ea.argt & EX_ARGOPT) { while (… getargopt(&ea) …) }`.

**The sixth is worth its lines, and that is a measurement.** `EX_ARGOPT` — `++ff=`,
`++enc=`, `++bin`, `++edit` — was on five rows: `:read`, which phase 90 took, and
these four. After anchor 2 it is on **none**, so the block can never be entered,
`getargopt()` can never run, and `exarg_T.read_edit` is written by nothing and read
by nothing. Deleting it hands all three to the sweep: 30 lines, a seventeenth
function, and a recording **byte-identical** to the one the five anchors alone
produce — `++edit` was only ever accepted by the commands this phase removes, so
there is nothing to declare. `EX_CMDARG` reaches zero rows too and is deliberately
left: `do_ecmd_lnum` is written through `eval_vars()`, which is the buffer-name
phase's, so the fold round it belongs there.

**Anchor 5 is not a `cutil.fold_never`, and the reason is indentation.**
`fold_never` keeps an `else` body by dedenting it four columns, which is right when
the body was written one level in. This one was not: upstream's `else` here has no
braces at all — the `if` is inside `#ifdef FEAT_SEARCHPATH` — so slim's bracing pass
put a `{`/`}` round the rest of the function and left every line at the function's
own four columns. Dedenting would have put forty lines at **column zero**, and
`CLAUDE.md`'s *Verification tiers* says no tier can see indentation. So the head and
its matching closer are deleted as counted text, found by brace matching and
required to be a line of its own, and the body keeps what it had.

**No handler is named.** The row is the only reference a command handler has, so
taking the five rows is what makes `ex_edit` unreachable and sixteen more follow it.

**The text the edit leaves does not compile**, as phases 89 and 90 leave theirs: three
mentions of `CMD_enew` and `CMD_view` survive, all inside `do_exedit`, and the edit
asserts that as a computation — every survivor is inside a function definition and
no surviving `cmdnames[]` row names that function, which is the whole argument that
`funcreach.py` takes it in the sweep's first round.

### The hazard this phase does not have, asserted anyway

**No `nv_cmds[]` row is deleted or repointed.** There is no row for `gf`, `gF`, `[f`
or `]f`: they are arms inside two handlers whose `g`, `[` and `]` rows dispatch
dozens of other keys. That is an argument, and `CLAUDE.md`'s twelve-phase arrow-key
bug is what an argument costs when it is wrong, so the check measures it: **fifty
`g*`, `[` and `]` keys pressed on both binaries, and exactly four moved** — `gf gF
[f ]f` — the other 46 identical in exit, bells, snapshots and stream digest.
`tools/nvidxcheck.py` still reports 194 rows indexed once each.

### The row floor is crossed here, and the floor moves in the same commit

`cmdnames[]` goes **104 → 99**, and create_cmdidxs's `names()` refused a table of
fewer than 100. **The failure is not the one the name suggests**: measured, it is
`no command table found in either shape`, because `names()` tries both parsers with
`check=False` and neither answer clears the bar. `tools/zexcmds.py` enumerates
zero's whole Ex sweep through it, so the old floor would have stopped the sweep,
`tools/zerodelta.sh`, the recording and every later phase's check rather than giving
a wrong answer. `WHIM-PLAN.md` II decision 8 is settled: **lowered to 80, deliberately,
in the phase that crosses it, with the reason in the tool's own docstring.** The
margin is 19 rows and the next row the plan removes is `:file`'s.

**What the floor edit costs, measured over all 115 implementation keys**
(`tools/implhash.sh` for every whim stage, every whim edit, every slim phase and
every zero phase): **28 move** — 6 whim stages (42-63, 66-71, 72, 73-77, 78, 79), 15
whim edits (58, 63, 66, 68–79), 2 slim phases (6, 7) and 5 of the phases from 83 (85, 87, 88, 89,
90). The last five are there only because their programs name the tool's path in a
comment; `implhash.sh` greps for paths and does not know what a comment is. Every
one of those phases calls the tool with a 489- or 600-row table, so `--check` passes
identically and every boundary reproduces; the cost is CPU in a repass. **Gated on
both**: `make slim-verify` 12 of 12 (394 s of phases in 114 s of wall time) and
`make whim-verify` 13 of 13 (1,638 s in 595 s), green after the edit. This is core rule 9
being paid rather than avoided — a zero-only tool was not an option, because the
floor is inside the tool the sweep reads the table with.

### The traps, which make the obvious check the wrong one

- **Five pairs where one name is a prefix of another and only one goes**:
  `check_lnums`/`check_lnums_both`, `reset_VIsual`/`reset_VIsual_and_resel`,
  `u_unchanged`/`u_unch_branch`, `do_ecmd`/`do_ecmd_cmd`, `otherfile`/`otherfile_buf`.
  Every count in both programs is `\b`-anchored for that reason.
- **`"edit"` reaches zero and `"ex"` does not.** `getargopt()`'s `++edit` strncmp
  was the last speaker of `"edit"` once the row went, and anchor 6 takes it; `"ex"`
  survives as one word of `'belloff'`'s value list. A check that wanted both to
  survive fails on this phase, and one that wanted both gone fails on a correct one.
- **`E447: Can't find file "%s" in path` survives.** `nv_gotofile()` was not its only
  speaker, so the message the key probes look for on the old binary is still in the
  source afterwards. It is the **key** that went, not the string.
- **Three things are left write-only rather than removed**, and are asserted at
  their counts so that a later widening has to move them: `readonlymode` (5
  mentions, one write, and that write `FALSE`), `do_ecmd_cmd` (6) and `do_ecmd_lnum`
  (2).

**`'undoreload'` is not this phase's, and it earns the plan's `uses` line.** `p_ur`'s
only reader was inside `do_ecmd`, so it is now a global with an option row and
nothing that reads it. Removing the row would change what `:set ur?` answers, which
nothing this pipeline records sweeps, so the delta could not be checked — and
`tools/orphanopts.py` refuses the opposite direction, a global whose row has gone.
It is asserted at exactly 2 mentions **with** its row, and
`uses options:94 files:91 mechanical  'undoreload' is read by do_ecmd` is the line
the options phase carries.

### The line against the byte-reader phase, and one correction to the plan

`readfile` keeps exactly **5** mentions — its prototype, its definition and the
three calls in `read_buffer()` and `open_buffer()` — and `read_buffer` 17.
`open_buffer` goes **6 → 5**, and that is the number `WHIM-PLAN.md` II.3c got
backwards: it says `readfile`'s last caller is `do_ecmd`, and `do_ecmd` called
`open_buffer`. What this phase costs the read path is one call site. The plan's
`uses files:91 files:90` line is corrected there.

### The declared delta, and what the corpus cannot see

`8   case:cmd_edit case:key_gf` and the five rows `edit enew ex view visual`. The
two kinds of movement are different:

- **`cmd_edit`** types `:edit` with no file name, so the baselines hold an editor
  that got as far as `check_changed()` and refused — `E37: No write since last
  change (add ! to override)`. It is `E492: Not an editor command: edit` now.
- **`key_gf`** presses `gf` on a word naming nothing, so the baselines hold
  `E447: Can't find file "nosuchfile" in path` — an editor that **looked**. The key
  beeps from `nv_g_cmd`'s `default: clearopbeep` now, where every other unused `g`
  key does: the record loses one snapshot and keeps **one bell either side**, so the
  check is the screen and the snapshot count and not the bell.
- the five `ref-excmds.txt` rows do not change message, they **cease to exist**:
  `tools/zexcmds.py` enumerates 99 names where it enumerated 104.

Measured with `tools/zcompare.py`: the other 100 screen cases, the other 94 command
rows, all 30 command lines, the four pty scenarios and the terminal table are
identical.

### The probes, which are the only evidence

**34, on both binaries** — the one the phase was handed, built by the edit part from
the boundary's own makefile flags, and the one it made — twenty required to move and
fourteen not, plus the fifty-key sweep and two pty sessions.

**The file they open is `keys` itself.** `tools/zstream.py` writes a session's
keystrokes into a file called `keys` in the run directory and feeds it on stdin, so
there is always one file there and no runner has to plant one. **What proves the
bytes arrived is the Escape in them** — the keystroke file holds `…\x1b:q!\r`, which
`tools/zscreen.py` draws as `^[:q!^M`, and nothing typed at `:` can put an Escape in
the buffer.

- **`edit_keys`, `ex_keys`, `visual_keys`, `view_keys`**: the keystrokes in the
  buffer and the file named on the old binary, E492 and nothing opened on this one.
  `view_keys` then asks `:set ro?` and requires `readonly` before and `noreadonly`
  after, which is the whole of what `do_exedit` did with `CMD_view`.
- **`enew_bang`**: the old binary throws the text away and this one does not.
- **`key_gf gF [f ]f`**: E447 on the old binary — the proof that the key reached
  `nv_gotofile()` and looked — and **one snapshot fewer** afterwards, the bell
  unchanged.
- **Ten spellings** — `:e :ed :edit :enew :ex :vi :vis :vie :view :visual` — each
  answering E37 before and E492 now, which is the inheritance check `CLAUDE.md`'s
  `:help` → `:helpclose` trap asks for. **`:en` is the eleventh and is not one of
  them**: `enew`'s shortest abbreviation is three characters, so `:en` matched
  nothing before this phase either, and it is asserted as E492 on *both* sides —
  whim's Phase 80 rule, measured rather than argued.
- **Fourteen that must not move**: `:en`, `:earlier`, `:verbose set ro?`,
  `:vglobal/a/d`, `:vmap`, `:file` (still `[No Name]`), `:read keys` (E492 on both,
  phase 90 having taken it), `:print`, `:append`, `:registers`, CTRL-G, `:q` on a
  modified buffer (still E37), `:q!` and an ordinary editing session. The ones that
  must not move are required to be *doing* something.
- **Two pty sessions**: `:e! planted.txt`, where the runner writes the file — the
  editor has had no way to write one since phase 89 — which the old binary loads and
  this one answers E492; and an editing session identical either side.

**Proven able to fail in both directions**: with the new binary on both sides all
twenty report *was to move and did not* and add *the keystroke file did not reach
the buffer on the input binary, so this proves nothing about opening a file*; with
the old binary on both sides they add *the new binary opened the file anyway*.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 84,453 | **83,755** (−698) |
| functions | 1,835 | 1,818 (−17) |
| type definitions | 1,010 | 998 |
| enumerators (DWARF) | 1,302 | **1,286** |
| `cmdnames[]` rows | 104 | **99** |
| `nm -u`, as `phasecheck.sh` counts it | 72 | **72, the same set** |
| `.text` / `.data` of the plain object | 638,960 / 38,699 | 633,907 / 38,571 |
| binary | 847,368 | **838,856** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set, so a symbol *arriving* would fail too. `:edit` reached `do_ecmd()`,
which reached `open_buffer()` and `readfile()`, and both are the startup path's, so
`open`, `read`, `close` and `stat` are required to be **still** undefined,
`WHIM-PLAN.md` part II P8 being the phase that frees them.

Seventeen functions go, none of them named by the edit — `do_ecmd` (328 lines),
`get_visual_text`, `check_lnums_both`, `do_exedit`, `nv_gotofile`, `grab_file_name`,
`prepare_help_buffer`, `u_unch_branch`, `text_or_buf_locked`, `reset_VIsual`,
`reset_VIsual_and_resel`, `delbuf_msg`, `ex_edit`, `u_unchanged`, `otherfile`,
`check_lnums` and `getargopt` — with two struct fields, sixteen enumerators and
seven string literals (`"edit" "enew" "view" "visual"`, E143, E1546 and the help
buffer's `'iskeyword'`). **Sixteen enumerators go and 87 renumber, every one of the
87 a `CMD_`**, and the check dumps DWARF either side and requires it: the sixteen
are the five `CMD_`, `EX_ARGOPT`, and ten single-constant enums the sweep takes with
their types (`CPO_GOTO1 DOCMD_RANGEOK ECMD_FORCEIT ECMD_HIDE ECMD_NOWINENTER
ECMD_OLDBUF ECMD_SET_HELP FNAME_REL FNAME_UNESC READ_NOWINENTER`). The sweep is **3
rounds** and the phase **50 s**. Its boundary is `eeb4031a31b4`, and `make
whim-verify` recomputes all nine in 79 s of wall time over 394 s of phases.

### Its placement

`stage 91`, `package files`, and two `uses` lines: `files:91 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records, and
`files:91 harness:86 mechanical`, because the old file-based sweep recorded an exit
status and `:edit` goes from E37 to E492 without changing it. **There is no `files:91
files:90` line**, for phase 90's reason: `tools/packages.sh --check` refuses a `uses`
inside one package.

**`need 91 swept`, measured.** The edit's anchor is `readfile` at exactly 5 mentions.
On the text phase 90's *edit* leaves there are **seven**, `ex_read` still being there
to make two of them, and the counted anchor refuses: `tools/phaserun.sh whim 90-91`
says `readfile has 7 mentions, expected 5`. The same run shows the edit's build of
the input binary failing on phase 90's non-compiling intermediate, which is true of
every zero edit that builds one and is not declared for that reason.

**`apart 90 91`, measured.** Phase 90's check requires `check_fname` at 4 mentions,
`open_buffer` at 6 and `read_edit` at 2, names `do_ecmd` and `otherfile` as a later
phase's, and requires `cmdnames[]` to hold 104 rows with `:edit` among them. Run on
the tree this phase leaves it gives seven complaints — `:edit went, and it is not
this phase's` and `do_ecmd went, and it is a later phase's` among them — and exits 1.

**And deliberately no `apart 89 91`**, which is the shape of the missing `apart 85 89`.
Phase 89's check *does* fail on a phase 91 tree — measured: `do_bang went`, `otherfile
went`, `cmdnames[] has 99 rows and names() reads 99; both must be 105`, exit 1 — but
a stage holding 89 and 91 holds 90, and `apart 89 90` forbids that already.

## Phase 92 (zero 9) — nothing reads a byte

`pipes/whim92-edit.sh` and `pipes/whim92-check.sh`, `stage 92`, `package files`. Phases
89, 90 and 91 took every way to *ask* for a file. This one takes the machinery those
commands used: `readfile()`, 787 lines, `read_buffer()`, the four functions of the
message layer that reported what had been read, and eleven more the sweep finds
under them. The file loses 1,183 lines and the core loses `access`, `fcntl` and
`open` — the first libc symbols a zero phase has freed since phase 89.

### What makes this phase different from every one before it

**`readfile()` was already unreachable when the phase was handed the tree**, and
that is the whole of what makes it delicate rather than difficult. Its three call
sites are one in `read_buffer()` and two in `open_buffer()`, and `read_buffer`'s
only callers are those same two arms. The outer arm needs `curbuf->b_ffname !=
NULL` and the inner one a `read_stdin` argument that all four callers pass as
`FALSE`; phase 88 took the file argument and the bare `-`, and phases 89, 90 and 91 took
every command that could name a file. Nothing the editor can be given reaches it.
gcc keeps the code only because it cannot prove `b_ffname != NULL` never holds.

So **the difference this phase makes is between code that cannot run and code that
is not there**, and no behavioural probe can see it. A recording that *moved* would
mean the cut was wrong. That is why the declared delta is nothing at all, and why
the evidence is something else.

### Four anchors, all inside `open_buffer()`

1. the `if (curbuf->b_ffname != NULL) {…} else if (read_stdin) {…}` pair, as exact
   text with the blank line after it — 36 lines holding all three calls into the
   read path;
2. `int read_fifo = FALSE;`, whose only writer was anchor 1;
3. `else if (retval == OK && !read_stdin && !read_fifo)` → `else if (retval == OK)`,
   where anchor 2's second reader was;
4. the signature — `open_buffer(int read_stdin, exarg_T *eap, int flags_arg)` →
   `open_buffer(void)` — the `int flags = flags_arg;` local, and the four call sites
   in `enter_buffer`, `ml_append_flags`, `ml_replace_len` and `create_windows`,
   every one of which already passed `FALSE, NULL, 0`.

**There is no prototype for `open_buffer`.** It is defined above its first call, so
the `static int open_buffer(…);` line the proto block would hold does not exist, and
a phase that edits one fails loudly. Anchor 4 edits the definition alone.

**Anchor 4 is what takes `read_stdin` to zero, and it is measured rather than
argued.** Without it `open_buffer` keeps three parameters nothing reads, and **the
sweep cannot see them**: `tools/sweep.sh` compiles with `-Wno-unused-parameter`, so
an unused parameter is invisible where an unused local is not. Measured both ways
on this input: anchors 1–3 alone leave the sweep deleting the `int flags =
flags_arg;` local by its own unused-variable pass and `read_stdin` alive at exactly
**one** mention, the parameter. Both swept files are **82,572 lines** and differ in
exactly **five** — the signature and the four calls — the two binaries are the same
830,440 bytes, and **the two recordings are byte-identical**. The fold costs
nothing, says what is true, and is taken.

**One further fold is declined, deliberately.** After anchor 1, `retval` in
`open_buffer` is `OK` from its initialiser to its return and nothing between can
change it, so `if (retval != OK) return retval;` is dead, the function could be
`void`, and the two `open_buffer() == FAIL` guards in the `ml_*` layer can never
hold. That is memline tidy, not the read path. The edit asserts `retval` at its **5
mentions with one assignment** and says it is constant, and the 75-line function is
left for a later phase.

**The text this edit leaves compiles**, where phases 89, 90 and 91 each left theirs
broken until the sweep had run: nothing it removed was named from outside what it
removed, so there is no dangling enumerator and no handler without a row. `readfile` goes 5 → 3
and `read_buffer` 17 → 15, and the survivors are not calls — a prototype, two
definitions, and **fourteen mentions of `readfile`'s own local `int read_buffer =
(flags & READ_BUFFER);`**. The edit asserts exactly that, and the two entry points
the sweep starts from are exactly the two `-Wunused-function` warnings the text
produces: `read_buffer` and `fix_help_buffer`.

### The evidence, which is an instrumented pair and nothing else

There is no behavioural must-differ probe and no dishonest one is offered instead.
What the check does is build **the source the phase was handed, twice**:

- **probe** — `old.c` with `(void)write(2, "READFILE-ENTERED\n", 17);` as
  `readfile()`'s first statement. Recorded with `tools/zrecord.sh`: **0 of the 106
  records** carry the marker.
- **ctl** — the *identical* instrument in `open_buffer()`, which **is** reached.
  **104 of the same 106** carry it.

The zero is the claim; the 104 is what makes it a probe that can fail. The two
records that stay quiet under `ctl` are `ref-pty.txt` and `ref-term.txt`, and the
reason is the instrument and not the editor — both drive a real pty and keep what
was *drawn*, where the other three keep stderr separately. They are named in the
check so that a third going quiet is a failure rather than a shrug.

**Proven able to fail, by measurement**: with `readfile` replaced by `open_buffer`
in the probe build, the check reports *104 of 106 records ENTERED readfile() on the
binary this phase was handed* and exits 1.

**Eight adversarial sessions** run on both instrumented binaries, and they are the
part that asks whether anything could still get in. Naming a buffer after a real
file that exists and then making the editor want its contents is the shape of every
way back into `readfile()` there was: `:file /etc/hostname` and then `G`, an insert
and an undo, `:bdelete`, the `%` register, and then `:new`, `:ball`, `:buffer 1` and
the `#` register. **Each reached `open_buffer()` and not one reached `readfile()`** —
and the first half of that is checked too, because a session that gets nowhere is
not an adversary.

### The declared delta is nothing at all, and it is measured twice

`pipes/zero.delta` gets a comment for phase 92 and no line, as phases 83, 84 and 86 do.
**`diff -rq` over two full `tools/zrecord.sh` recordings — the binary the phase was
handed against the one it made — is empty**: all 102 screen cases, all 111
Ex-command rows, all 30 command lines, the four pty scenarios and the nineteen
terminal rows. `tools/zerodelta.sh --phase 92` then finds the same against whim-vim's
frozen baselines, with the eight lines phases 85 to 91 declared and nothing new.

The check also runs eight sessions directly between the two binaries and requires
each to be identical **and to be doing something**: an ordinary editing session,
`:file` and CTRL-G (still `[No Name]`), `:registers` with its table in the stream,
the `%` and `#` registers, `:q` on a modified buffer (still E37) and `:q!`.

### The traps, which make the obvious check the wrong one

- **`check_readonly` reaches zero here, not at phase 89.** It was `readfile()`'s
  *local*, four mentions since phase 89, and a check copied from that phase fails on
  a correct phase 92.
- **`readonlymode` goes 5 → 3**, where phase 91's check asserts 5: `readfile` held
  two of them. It is still write-only and `FALSE`, and still the options phase's.
- **`msg_scrolled_ign` becomes read-only, and nothing sees it.** Four writers, all
  inside `filemess()` and `readfile()`; after this phase it is `FALSE` for ever with
  one reader left, in `msg_puts_attr_len()`. gcc has no warning for a variable that
  is only read, `deadsweep.py` removes what is unused rather than what is constant,
  and `deadfields.py` is about struct members. It is asserted at **2 mentions,
  read-only**, and handed on rather than folded.
- **Four struct fields become write-only and `deadfields.py` cannot see them**,
  because they are still *named* — by the writes in `buf_store_time()` and
  `set_b0_fname()`: `b_mtime_read`, `b_mtime_read_ns`, `b_orig_size`, `b_orig_mode`,
  three mentions each, of which exactly one is not a write. They go with the
  buffer's name in phase 93. (`b_mtime` and `b_mtime_ns` are read, but only to feed
  `b_mtime_read`, so the whole six-field cluster is dead from outside.)
- **`"[RO]"` goes 3 → 2 and `"[readonly]"` 2 → 1**, each losing `readfile`'s copy
  and keeping the rest, and **CTRL-G's counter survives** — `"%ld line --%d%%--"` is
  `fileinfo()`'s and was never `readfile`'s. All three are asserted at their counts,
  which is the opposite direction from the 24 strings that go.
- **`read_cmd_fd` does not move at all**: 12 mentions on 11 lines, and every one of
  them the terminal's — `fill_input_buf()` and `mch_settmode()`.
- **`setfname` goes 3 → 2**, because `set_rw_fname` was its second caller. That is
  what makes phase 93 possible.

### Seventeen enumerators go and nothing renumbers

`typereach.py` deletes **seventeen whole anonymous enum definitions** — the eight
`READ_*` flags, and `BF_NEW_W`, `CONV_RESTLEN`, `CPO_FNAMER`, `NOTDONE`, `O_EXTRA`,
`SHM_LAST`, `SHM_LINES`, `SHM_OVER` and `SHM_OVERALL` — and a whole definition
leaving takes no survivor's value with it. The check dumps DWARF either side and
requires exactly that: **1,286 → 1,269, not one survivor renumbered and none
arriving**, so no parallel table can have shifted. That is the opposite of phase 91,
where 87 renumbered, and it is worth the four seconds either side to say.

**No `cmdnames[]` row and no `nv_cmds[]` row is touched**: this phase removes no
command. The table is the same 99 rows phase 91 left, `names()` reads 99, the
`static_assert` is in place, and the floor phase 91 lowered to 80 is asserted **by
using it** — the checked parser is called and must not refuse — rather than by
grepping for the number. `tools/nvidxcheck.py` still reports 194 rows indexed once
each. `E32: No file name` survives with `check_fname` at 3 mentions, and E37 with
`check_changed` at 4.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 83,755 | **82,572** (−1,183) |
| functions | 1,818 | 1,802 (−16) |
| type definitions | 998 | 981 (−17) |
| enumerators (DWARF) | 1,286 | **1,269** |
| `cmdnames[]` rows | 99 | 99 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 72 | **69** |
| `.text` / `.data` / `.rodata` of the object | 633,907 / 38,571 / 17,225 | 623,651 / 38,379 / 16,921 |
| binary | 838,856 | **830,440** |

**Three symbols go and the check names the set, not the count**: `access`, `fcntl`
and `open` were `readfile()`'s and nothing else's. `read`, `close` and `dup` **stay**
and are the terminal's alone — `fill_input_buf()` and `mch_settmode()` — so a check
that read "the file symbols went" would be wrong here; `stat`, `getcwd` and
`strerror` are phase 93's and `fsync` the `FILE *` phase's, and all seven are
required to be **still** undefined.

Sixteen functions go, none of them named by the edit: `readfile` (787 lines),
`read_buffer`, `read_eintr`, `readfile_linenr`, `filemess`, `msg_add_fname`,
`msg_add_lines`, `msg_add_eol`, `after_pathsep`, `dir_of_file_exists`,
`fix_help_buffer`, `gettail_sep`, `mch_isdir`, `set_rw_fname`,
`u_find_first_changed` and `utf_ptr2len_len` — with fifteen prototypes, three
file-scope error strings, seventeen enumerators and **24 string literals**, which
are the whole of the message layer: `"%s%ldL, %lldB"`, `"[noeol]"`,
`"[READ ERRORS]"`, `"[New DIRECTORY]"`, `"Vim: Reading from stdin...\n"`, E200, E201,
E812 and sixteen more. Nothing in the instrument loses a message: the last thing
that could print `"keys" 1L, 30B` was `:read`/`:edit`. The sweep is **3 rounds** and
the phase **42 s**. Its boundary is `6755bb567bea`, and `make whim-verify`
recomputes all ten in 80 s of wall time over 448 s of phases.

### Its placement

`stage 92`, `package files`, and two `uses` lines: `files:92 seed:83 mechanical`,
because `tools/zerodelta.sh` compares the recording with the baselines phase 83
records and this phase's whole declaration is that nothing in them moved, and
`files:92 streams:88 mechanical`, because the `read_stdin` *argument* anchor 4 removes
is `FALSE` at all four call sites only since phase 88 took the bare `-` and
`EDIT_STDIN` with it. **There is no `files:92 files:90` or `files:92 files:91` line**,
for phase 90's reason — `tools/packages.sh --check` refuses a `uses` inside one
package — and both dependencies are real and are stated here and in the program's
head instead: `:read` held two of `readfile`'s seven mentions before phase 90, and
`do_ecmd` passed `eap` and flags to `open_buffer` until phase 91, which is what lets
the signature fold happen at all.

**`need 92 swept`, measured.** The edit's anchor is `open_buffer` at exactly 5
mentions — the definition and four callers, every one of them `open_buffer(FALSE,
NULL, 0)`, which is what lets anchor 4 rewrite all four by text. On the text phase
91's *edit* leaves there are **six**: `do_ecmd` is still there to make
`(void)open_buffer(FALSE, eap, readfile_flags);`, the one call site the rewrite
would **not** match and one the sweep would then delete, hiding the mistake.
`tools/phaserun.sh whim 91-92` says `open_buffer has 6 mentions, expected 5`. The same
run shows the edit's build of the input binary failing on phase 91's non-compiling
intermediate, which is true of every zero edit that builds one and is not declared
for that reason.

**`apart 91 92`, measured.** Phase 91's check draws its line against this phase as
counts — `readfile` 5, `read_buffer` 17, `readonlymode` 5, `b_ffname` 43,
`b_fname` 37 — so that reaching into the read path would fail rather than widen
quietly. Run on the tree this phase leaves it gives five complaints, `readfile has 0
mentions, expected 5` among them, and exits 1. Its symbol check would fail too,
being a `cmp` of the whole undefined set against a phase that frees three, but the
source assertions come first.

## Phase 93 (zero 10) — the buffer has no name

`pipes/whim93-edit.sh` and `pipes/whim93-check.sh`, `stage 93`, `package files`.
Phases 89, 90 and 91 took every way to *ask* for a file and phase 92 took the machinery
that read one. What was left of the filesystem in this editor is a **name**: three
`char_u *` fields on every buffer — `b_ffname`, `b_sfname`, `b_fname` — and the one
command that could still set them, `:file`. This phase takes the command, stops
`buflist_new()` naming the buffer it makes, folds the sixteen places that ask what
the name is, and hands the sweep **sixty functions**, the largest number any zero
phase has. It is also where the core stops asking the filesystem questions of its
own initiative: `stat`, `getcwd` and `strerror` go, and with `access`, `fcntl` and
`open` already gone at phase 92 **the process has no way left to acquire a fourth
file descriptor**.

### Seven parts, and every removal that is not one of them is the sweep's

**A — `:file` goes.** The enumerator, the `cmdnames[]` row, and **both** of
`do_one_cmd`'s `CMD_file` tests: the `curbuf_locked()` conjunct phase 91 deliberately
kept, saying this phase would take it, and the second test below it. All in the same
edit as the enumerator or the text does not compile. The row is the only reference a
handler has, so taking it is what makes `ex_file` unreachable, and `rename_buffer`
and `setfname` follow — `set_rw_fname` having been `setfname`'s second caller until
phase 92 took it. **`fileinfo()` survives**, with three callers: CTRL-G, `g CTRL-G`
and the startup message.

**B — `buflist_new()` never names.** Its one call site is `create_windows`', and it
has passed `NULL, NULL` since phase 88 took the file argument. So both parameters go
with the `fname_expand`/`stat`/`buflist_findname_stat` prologue, the
`if (ffname != NULL)` assignment that *was* the naming, the failure arm's frees and
the `st.st_dev` block — nine counted replacements inside one definition, plus the
prototype and the call.

**C — sixteen folds, one per site, each with its constant written out in the
program.** The three fields are NULL for ever, so `== NULL` is TRUE and folds always
and `!= NULL` is FALSE and folds never. **The invariant is computed before anything
is folded**: every write to the three fields is enumerated and required to be inside
`buflist_new`, `setfname`, `rename_buffer` or `shorten_buf_fname` — the four this
phase accounts for — and a write anywhere else would make every fold a guess.

**D — `EX_XFILE` reaches zero rows, which is the largest part of the phase and is
computed.** `:read` was one of its six rows and phase 90 took it; four more went with
the `:edit` family at phase 91; `:file` was the last. So `do_one_cmd`'s
`if ((ea.argt & EX_XFILE) && expand_filename(…) == FAIL)` can never be entered, and
folding it never hands the sweep **32 of the sixty functions** — `expand_filename`,
`eval_vars`, `find_cmdline_var`, the whole `ExpandOne`/`ExpandFromContext`/
`gen_expand_wildcards` layer, `vim_FullName`, `mch_FullName`, `FullName_save`,
`shorten_fname`, `home_replace_save`, `backslash_halve` and the `ff_*` remnants. It
is phase 91's `EX_ARGOPT` in exactly the same shape. One more edit goes with it:
`separate_nextcmd`'s `eap->argt & (EX_CTRLV | EX_XFILE)` loses the second disjunct,
which is 0 for every row, and that is what takes the enumerator itself to zero.

**E — two write-only leftovers nothing can see.** `readonlymode` had one reader,
inside `open_buffer`, and part C folds it; a file-scope static that is assigned and
never read draws no warning and `deadsweep.py` acts on warnings, so it goes by hand
with the `if` around its write — an `if` with an empty body being something no tool
here removes either. `b_dev_valid`'s one surviving assignment is part B's fold of
the device block, and `deadfields.py` removes a field nothing *names*, not one that
is only written, so it goes by hand too and the three fields it guards sweep.

**F — `shorten_fnames()` stops asking where it is.** `shorten_buf_fname()` is empty
after part C, so the `mch_dirname()` cwd fetched for it is fetched for nothing. The
signature folds to `void` in the same edit, for phase 92's measured reason:
`tools/sweep.sh` compiles with `-Wno-unused-parameter`, so an unused parameter is
invisible where an unused local is not.

**G — nothing looks a name up on a disk.** `find_file_name_in_path()`'s
`if (options & FNAME_EXP)` arm searched `'path'` and `mch_getperm()`ed each
candidate; the other arm returns the word itself. Folding the arm never is the
charter reading of *no filesystem access*: the editor extracts text and asks
nothing. `find_file_in_path()` and `mch_getperm()` are then the sweep's, and `stat`
goes with them. **The cost is that CTRL-F and CTRL-P become indistinguishable**, and
that is the one piece of behaviour this phase gives up beyond `:file`.

### Three things about the folds that a tool would have got wrong

**`buflist_name_nr` is folded at its callers and never in place, and the agent that
surveyed this phase made the mistake first.** Its body is `buf =
buflist_findnr(fnum); if (buf == NULL || buf->b_fname == NULL) return FAIL; *fname =
buf->b_fname; … return OK;`. Folding the whole `if` away gives a function that
returns OK with `*fname` never written — a silent behaviour change in the direction
that crashes. What is true is that it returns **FAIL always**, so the fold belongs
at `getaltfname()`, which becomes `emsg(E23); return NULL;`, and at `ex_display()`,
whose `"#` block then does nothing and goes whole. Only then is it uncalled.

**Three sites have an `else` and `cutil.fold_always` refuses them, by design.**
Keeping a body and dropping an else is not what it does, so `fileinfo()`,
`set_b0_fname()` and `get_trans_bufname()` use a local `fold_always_else()` that
keeps the if body dedented four columns — right only because each of the three was
**read** first, phase 91's anchor 5 being what a wrong dedent costs. The helper
refuses a body that is not written one level in.

**Four more are a function whose whole body is the `if`.** `buf_spname()`,
`buf_get_fname()`, `check_fname()` and `getaltfname()` each end in a second
`return`, and `fold_always` there leaves it behind **unreachable and alive** —
measured: `return buf->b_fname;` would have kept `b_fname` referenced for ever and
no sweep tool removes it. Those four are exact-text rewrites of the body.

**And two folds the brief asked for are not made.** Both of `eval_vars()`'s
`if (b_fname == NULL)` arms are inside a function part D makes unreachable —
`expand_filename()` and `expand_wildcards_eval()` are its only callers and both go —
so folding inside text the sweep deletes changes no output and states nothing. Rule
1 applies, and the check requires `eval_vars` at 0 mentions instead.

### What the screen shows afterwards

`[No Name]` survives and is now **the only thing `buf_spname()` can return**, not
one of two answers. CTRL-G prints `"[No Name]" [Modified] 1 line --100%--`
byte-identically either side, and so do `g CTRL-G`, the status line and `:ls`.
`:registers` does not move either: its `"%` and `"#` lines were never printed,
`b_fname` having been NULL since phase 88.

**Three flags are left read-only and named rather than folded.** `BF_NOTEDITED` can
never be set — `setfname()` was its only writer — and `BF_NEW` never could; both are
still read by `fileinfo()`, so CTRL-G asks two questions whose answer is fixed.
`b_shortname` has the same shape and was **already** write-only before this phase.
Folding any of the three changes the string set for no gain, so all three are
asserted where they are. `msg_scrolled_ign` is phase 92's leftover and does not move.

**`"file"` reaches zero and `E32: No file name` does not.** `check_fname()` survives
folded to an unconditional `emsg`, because `get_spec_reg()`'s `%` still calls it.
**`E447: Can't find file "%s" in path` does reach zero here**, and phase 91's check
asserts it *survives* — part G takes its last speaker. The two checks disagree on
purpose, and `apart 91 93` is unnecessary only because `apart 91 92` and `apart 92 93`
already forbid the stage.

### The declared delta: one case and one row

```
10    case:cmd_file
      file
```

`cmd_file` types `:file` with no argument, so what the baselines hold is the CTRL-G
line for `[No Name]` — an editor that answered with the buffer's name. It is
`E492: Not an editor command: file` now, with the **same exit, the same bells and
the same snapshot count**, the stream going 2,310 → 2,327 bytes. The `ref-excmds.txt`
row `file` does not change message: it **ceases to exist**, `tools/zexcmds.py`
enumerating 98 names where it enumerated 99.

**Nothing else can move, and the reason is stronger than a measurement**: every fold
takes the branch the code already took at run time. Measured with
`tools/zcompare.py`: the other 101 screen cases, the other 97 command rows, all 30
command lines, the four pty scenarios and the terminal table are identical —
`:filter` and `:fixdel` among them, and `:q` on a modified buffer (still E37).

### The probes, which are the only evidence a buffer could be named

**21, on both binaries** — the one the phase was handed, built by the edit part from
the boundary's own makefile flags, and the one it made — eight required to move and
thirteen not.

- **`file_rename` is the probe.** `ihello<Esc>:file NEWNAME<CR>` and then CTRL-G:
  the old binary answers `"NEWNAME" [Modified][Not edited] 1 line --100%--` and this
  one `"[No Name]" [Modified] 1 line --100%--`. That line, on the *old* binary, is
  the whole evidence that a buffer could be named, and **the corpus cannot see it**:
  every case types its own text and names nothing. `[Not edited]` is `BF_NOTEDITED`,
  which `rename_buffer()` set and which nothing can set now. `file_bang` is the same
  with `:file!`.
- **`cp_missing` is the only probe that shows the old binary asking the disk.** With
  `nosuchfile` under the cursor, `: CTRL-R CTRL-P <CR>` was **silent** before —
  `find_file_in_path()` stat()ed the name, found nothing and yielded NULL, so nothing
  reached the command line — and answers `E492: Not an editor command: nosuchfile`
  now. Its pair **`cp_existing` must not move and does not**, which is what says part
  G removed the lookup and not the extraction: with `keys` under the cursor — the
  keystroke file `tools/zstream.py` always leaves in the run directory — both
  binaries answer `E488: Trailing characters: eys`, `:k` being a command of its own.
  `cf_existing` is the same session with CTRL-F, which never expanded.
- **Five spellings** — `:f :fi :fil :file :file!` — each E492 now and none before,
  `:file`'s row having given its shortest abbreviation as one character. `:filter`
  and `:fixdel` are the neighbours required not to move, which is the inheritance
  check `CLAUDE.md`'s `:help` → `:helpclose` trap asks for.
- **Thirteen that must not move and do not**: CTRL-G, `g CTRL-G`, `:registers` with
  its table **and without a `"%` or `"#` line**, the `%` and `#` registers,
  `cp_existing`, `cf_existing`, `:filter`, `:fixdel`, `:ls`, `:q` on a modified
  buffer (still E37), `:q!` and an ordinary editing session. Each is required to be
  *doing* something.
- **Two pty sessions**, because every probe above went through a pipe: `:file
  NEWNAME` then CTRL-G, which renames on the old binary and answers E492 and
  `[No Name]` here, and an editing session identical either side.

**Proven able to fail in both directions**: with the new binary on both sides all
eight report *was to move and did not* and add *the input binary did not name the
buffer NEWNAME, so this proves nothing*; with the old binary on both sides they add
*the new binary named the buffer anyway* and *a removed name has been inherited*.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 82,572 | **80,387** (−2,185) |
| functions | 1,803 | **1,743** (−60) |
| type definitions | 981 | 922 |
| enumerators (DWARF) | 1,269 | **1,197** |
| `buf_T` fields | | **−12** |
| `cmdnames[]` rows | 99 | **98** |
| `nm -u`, as `phasecheck.sh` counts it | 69 | **66** |
| binary | 830,440 | **812,744** |

**Three symbols go and the check names the set, not the count**: `stat` was
`mch_getperm()`'s and nothing else's, `getcwd` and `strerror` were `mch_dirname()`'s.
`read`, `close` and `dup` **stay** and are the terminal's alone, and `fsync` is
`ui_write`'s and the `FILE *` phase's; all four are required to be still undefined,
and `open access fcntl chmod fchmod fstat lstat unlink` to be still **absent**, which
is the file-descriptor invariant stated as a check.

**Seventy-two enumerators go and eighty-five renumber, every one of the 85 a
`CMD_`** — the `EXPAND_*`, `WILD_*`, `EW_*`, `XP_BS_*`, `SPEC_*`, `BLOCK0_*`, `BLN_*`,
`ESTACK_*`, `VSE_*` and `VALID_*` families leave as whole anonymous definitions, which
takes no survivor's value with them, and `CMD_file`'s row is what moves the rest.
`cmdnames[]` is designated, so a row lands at its own enumerator whatever the
numbering is — but 85 movers from one family is exactly the case `CLAUDE.md` says a
build is happy to get wrong, so the check dumps DWARF either side and requires it.

The sweep is **4 rounds** and the phase **67 s**. Its boundary is `2829849cb53a`, and
`make whim-verify` recomputes all eleven in 80 s of wall time over 523 s of phases.

### Its placement

`stage 93`, `package files`, and two `uses` lines: `files:93 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records, and
`files:93 harness:86 mechanical`, because `:file` went from the CTRL-G line to E492
with the **same exit status** — the old file-based sweep recorded an exit status, so
phase 86's message-level record is what can see this phase at all. **There is no
`files:93 files:90`, `files:93 files:91` or `files:93 files:92` line**, for phase 90's
reason — `tools/packages.sh --check` refuses a `uses` inside one package — and all
three dependencies are real and are stated in the program's head instead: `:read` was
one of the six `EX_XFILE` rows and phase 90 took it, four more went with the `:edit`
family at phase 91, which is what makes `:file` the last and part D possible, and
`set_rw_fname` was `setfname`'s second caller and went with `readfile` at phase 92.

**`need 93 swept`, measured.** The edit's anchor is `b_ffname` at exactly 32
mentions, with `b_sfname` at 26 and `b_fname` at 29. On the text phase 92's *edit*
leaves there are **forty**, `readfile()` still being there to make eight of them, and
the counted anchor refuses: `tools/phaserun.sh whim 92-93` says `b_ffname has 40
mentions, expected 32`. Unlike 90, 91 and 92, the text before it **compiles** — phase
92's edit left valid C — so the refusal is the counted anchor alone.

**`apart 92 93`, measured.** Phase 92's check draws its line against this phase as
counts — `b_ffname` 32, `b_sfname` 26, `b_fname` 29, `setfname` 2, `readonlymode` 3,
`eval_vars` 4, `mch_dirname` 5 and the four write-only fields at 3 each — and names
the four as things phase 93 takes. This phase takes every one to 0. Run on the tree
it leaves, phase 92's check gives eighteen count complaints, `b_ffname has 0 mentions,
expected 32` among them, plus `b_mtime_read is no longer a field of buf_T`, and exits
1. **There is deliberately no `apart 91 93`**, although phase 91's check does fail here
— it requires E447 to survive — because a stage holding 91 and 93 holds 92, and `apart
91 92` forbids that already. It is the shape of the missing `apart 85 89` and `apart 89 91`.

## Phase 94 (zero 11) — `:q` quits, and `ZZ` is `ZQ`

`pipes/whim94-edit.sh` and `pipes/whim94-check.sh`, `stage 94`, `package buffers`.
Phases 89 to 93 took every way to reach a file. What was left of the filesystem in
this editor was a **refusal**: `:q` on a modified buffer answered `E37: No write
since last change (add ! to override)` and stayed. The protection has no remedy once
nothing can be written — there is no `:w` to answer it with and no file the text
could have come from — so it is a door onto nothing, and this phase takes it. `:q`,
`:q!`, `ZZ` and `ZQ` are one thing afterwards.

### One anchor, and eleven of the sixteen functions are a surprise

`ex_quit()` is `if ((check_changed(…)) || (check_changed_any(…))) { not_exiting(…); }
else { getout(0); … }`, and the test *is* the refusal. Folding it **never** keeps
the `else` — quit — and is the last reference `check_changed()` has. That single
fold is the phase; the edit names not one function.

**Fifteen functions follow by reachability and eleven of them are not the refusal at
all.** `check_changed_any()`'s tail is *"go to the buffer that refused"* — it calls
`set_curbuf()`, which calls `enter_buffer()` and `win_enter_ext()` — and after whim
removed the buffer list and the window commands, **that tail was the last caller of
the whole switch-buffer/switch-window island**: `add_bufnum`, `set_curbuf`,
`enter_buffer`, `win_enter`, `win_enter_ext`, `goto_tabpage_win`, `goto_tabpage_tp`,
`get_winopts`, `find_wininfo`, `buflist_findfpos` and `buflist_getfpos`. After this
phase the editor has no code for entering a different buffer or a different window.

**The island is a graph and not a fan, and the edit computes that before it folds
anything.** Only `add_bufnum`, `set_curbuf` and `goto_tabpage_win` are called by
`check_changed_any` itself; the other eight hang off those. So what is required is
that *every* call to any of the eleven is inside `check_changed_any` or inside
another of the eleven — and a check that asked for the simpler shape would fail on a
correct phase. Three of the fifteen also have **no prototype**, being defined above
their first call (`check_changed_any` and `no_write_message_nobang` at two mentions,
`add_bufnum` at three for having two calls), so a loop that wanted three for all of
them refuses. Both facts were discovered by the counted anchors refusing.

### Two extras, each measured byte-identical in the recording

**A — two struct fields that become write-only, which no tool can see.** This is
phase 90's `usefilter` judgement in a smaller shape: `deadfields.py` removes a field
nothing *names*, and gcc has no warning for a member that is only written.
`win_T.w_topline_was_set`'s only reader was in `enter_buffer()` and
`wininfo_S.wi_changelistidx`'s only reader was in `get_winopts()`. The declaration
and the one surviving write of each go by hand, and **the text the edit leaves does
not compile** — both readers are still there, inside functions the sweep is about to
take — which is said in the program rather than discovered, as `pipes/whim90-edit.sh`
says of its own.

**B — the tail that cannot run.** After the fold `ex_quit()` ended `int save_exiting
= exiting; exiting = TRUE; getout(0); not_exiting(save_exiting);`. `getout()` sets
`exiting = TRUE` **itself** and ends in `mch_exit()`, which never returns, so the
first, second and fourth statements are dead and gcc cannot prove it. Replacing the
four with `getout(0);` orphans `not_exiting()`, and **`not_exiting()` is the refusal
machinery** — `exiting = save_exiting; settmode(TMODE_RAW);`, the "we changed our
mind, put the terminal back" — so it is this phase's and not tidy. The fold is right
only because `getout()` sets `exiting` for itself; check that before making it on
another tree.

### What survives, and why a check copied from phases 89 to 93 fails here

* **The buffer still knows it is modified.** `bufIsChanged` goes 10 → 7 and
  `curbufIsChanged` does not move at all: CTRL-G still prints `[Modified]`, the
  status line still draws `[+]`, `:set modified?` still answers. What went is the
  refusal, not the state.
* **`:q` can still decline.** `text_locked()`, `curbuf_locked()` and
  `before_quit_autocmds()` all return early **above** the anchor and are untouched.
* **Phases 89, 90, 91, 92 and 93 each assert `E37: No write since last change` survives**
  and name `check_changed` as the `:q` phase's. This is the `:q` phase, so the check
  asserts the opposite in both directions: E37 must be absent here and must have been
  present in the input. `apart 93 94` records it.
* **`open_buffer` goes 5 → 4** — `enter_buffer()` was one of its four callers — where
  phases 92 and 93 both pin it at 5. `buf_spname` goes 5 → 4 and `exiting` 17 → 13.
* **`p_wh` looks write-only and is not.** It goes 4 → 2, the two reads inside the
  island having gone, and what is left is its declaration, which carries the
  initialiser, and one real reader in the frame layer. A naive "uses − writes − 1 ≤ 0"
  scan reports it; the check's scan excludes the declaration **by position** and then
  reports nothing but `vim_ignored`, upstream's sink for an ignored return value,
  which is write-only in the input too. Running it on both texts and requiring the
  same set is what stops an empty answer being a broken scan rather than a clean phase.
* **`SHM_FILEINFO` leaves**, and it is the `'shortmess'` `F` letter: its only reader
  was inside `enter_buffer()`. The letter is accepted and inert afterwards. That is
  the options phase's and no flag string is touched here.
* **`ZZ` is already `ZQ` and stays so.** `nv_Zet` has run `do_cmdline_cmd("q!")` for
  `case 'Z'` and for `case 'Q'` since phase 89. The strings are **not** rewritten to
  `"q"`: it would move `zz_key` and `zq_key` for no gain, and `case:zz_key` is phase
  89's declaration.
* **There are no `'confirm'`-style prompts to worry about**: `grep -cw confirm` on the
  input is 0, whim having removed the dialog layer. Said out loud so that the next
  reader does not go looking.

### Twelve enumerators go and nothing renumbers

`typereach.py` takes twelve as whole anonymous definitions — the four `CCGD_`, the
two `DOBUF_`, `SHM_FILEINFO` and the five `WEE_` — and a whole definition leaving
takes no survivor's value with it. The check dumps DWARF either side and requires
exactly that: **1,197 → 1,185, not one survivor renumbered and none arriving.** That
is the opposite of phase 93, where 85 moved, and it is worth the four seconds either
side to say rather than assume. **No `cmdnames[]` row and no `nv_cmds[]` row moves**:
98 rows, `names()` reads 98, the `static_assert` is in place and `nvidxcheck` reports
194.

### The declared delta: one case and one row, and the row is a third kind

```
11    case:quit_modified
      quit
```

`quit_modified` types text and then `:q`, so what the baselines hold is an editor
that refused: the E37 line goes, the one bell with it, and the record loses a
snapshot — 2,342 → 2,213 bytes. **The exit status does not move there, and that is
the corpus's limit rather than the phase's**: every `zcases.py` case ends with a
trailing `:q!`, which quits the old binary too.

The `ref-excmds.txt` row `quit` **changes message and does not cease to exist**,
unlike every row phases 89 to 93 declared: `:quit` is still a command with its row, so
`tools/zexcmds.py` enumerates the same 98 names and compares the block, whose `msgs`
go from `:set nopaste / E37… / :q!` to `:set nopaste / :quit`. The `cquit` row does
not move. Measured with `tools/zcompare.py`: the other 101 screen cases, the other 97
command rows, all 30 command lines, the four pty scenarios and the terminal table are
identical.

### The probes, which are the only evidence the refusal went

Seventeen, on both binaries — the one the phase was handed, built by the edit part
from the boundary's own makefile flags, and the one it made — six required to move
and eleven not.

* **`q_alone` is the probe.** `ihello<Esc>`, `:set nopaste`, `:q` **and nothing after
  it**. The old binary draws E37, runs out of stdin, prints `Vim: Finished.` and exits
  **1**; this one quits on the `:q` and exits **0**. That difference is the whole
  phase measured from outside and no recording can see it.
* **Five more spellings of the same refusal** — `:q` with the trailing `:q!` (the
  declared case), `:1q`, `:qu`, `:quit`, and `x` then `:q` — each required to have
  refused **before** and not to now, and each to lose the E37 snapshot. `q_modified`'s
  bells must go 1 → 0.
* **Eleven that must not move and are required to be *doing* something**: `q_clean`
  (`:q` on an **unmodified** buffer, status 0 either side and no E37 anywhere — it
  took the else arm before this phase and takes it now, which makes it `q_alone`'s
  pair), `q_bang`, `zz_key` and `zq_key` — which must also be identical **to each
  other** — `cquit` (exit 1 either side), `ctrl_g` (must say `[Modified]`),
  `cmd_set_ro`, `cmd_set_mod` (`:set modified?` must answer), `reg_list`, `cmd_undo`
  and an ordinary editing session.
* **A real terminal**, because every probe above went through a pipe: `ityped<Esc>`,
  `:q`, `:q!`. On the old binary the `:q` draws E37 and leaves the editor running, so
  the `:q!` is what ends the session; here the `:q` quits and the `:q!` reaches
  nothing. An ordinary pty editing session beside it is identical either side.

**Proven able to fail in both directions**: with the new binary on both sides all six
report *was to move and did not* and add *the input binary did not refuse, so this
proves nothing about a refusal being removed* and *exited 0, expected 1*; with the old
binary on both sides they add *this binary still refuses* and *exited 1, expected 0*.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,387 | **79,866** (−521) |
| functions | 1,742 | **1,726** (−16) |
| type definitions | 922 | 910 |
| enumerators (DWARF) | 1,197 | **1,185** |
| struct fields | | **−2**, by hand |
| `cmdnames[]` rows | 98 | 98 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 66 | **66** |
| binary | 812,744 | **804,360** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set, so a symbol *arriving* fails too. Sixteen functions go and not one was
libc's last caller: the refusal printed through `emsg()` and the island moved windows,
neither of which reaches the C library on its own. `fclose`, `getc`, `putc` and
`fsync` are required to be **still** undefined and are the `FILE *` phase's; `open`,
`access`, `fcntl`, `stat`, `getcwd` and `strerror` to be still **absent**.

The sweep is **3 rounds** and the phase **97 s**. Its boundary is `b81ce6372fc4`, and
`make whim-verify` recomputes all twelve in 107 s of wall time over 629 s of phases.

### Its placement

`stage 94`, `package buffers` — the first phase of a package of its own — and two
`uses` lines: `buffers:94 seed:83 mechanical`, because the two declared records are
compared with the baselines phase 83 records, and `buffers:94 files:89 rationale`,
because the refusal has no remedy once nothing can be written: phase 89 took every
`:write`, so E37 asked for a save the editor no longer had any way to perform.

**`need 94 swept`, measured, and the brief that specified this phase said there was
none.** The invariant the whole phase rests on is that every call to any of the eleven
island functions is inside `check_changed_any` or inside another of the eleven. On the
text phase 93's *edit* leaves that is **false**: `buflist_findlnum()` is still there to
make `return buflist_findfpos(buf)->lnum;`, a call from outside the island, and phase
93's sweep is what takes it — so `buflist_findfpos` has four mentions where the anchor
wants three. `SHM_FILEINFO` refuses first, at 3 where it wants 2, `ex_file()` still
being there to read the `'shortmess'` `F` letter: `tools/phaserun.sh whim 93-94` says
`SHM_FILEINFO has 3 mentions, expected 2`. Unlike 90, 91 and 92 the text before it
**compiles** — phase 93's edit left valid C — so the refusal is the counted anchors
alone.

**`apart 93 94`, measured.** Phase 93's check pins `check_changed` at 4,
`no_write_message` at 3, `buf_spname` at 5 and `open_buffer` at 5, and requires `E37:
No write since last change` to survive. Run on the tree this phase leaves it gives
five complaints — `check_changed has 0 mentions, expected 4` among them, and `E37
went, and check_changed() is the :q phase's` — and exits 1. **Only `apart 93 94` is
written**, and the four before it are implied: a stage holding 89 and 94 holds 93, so
that line forbids it already. It is the shape of the missing `apart 85 89`, `apart 89 91`
and `apart 91 93`.

## Phase 95 (zero 12) — the options nothing reads

`pipes/whim95-edit.sh` and `pipes/whim95-check.sh`, `stage 95`, `package options`.
Phases 89 to 94 took every way to reach a file and then the refusal that guarded the
text. What they left behind is a set of **settings**: `options[]` rows whose global
nothing reads any more, so that `:set fsync?` answers a question about machinery that
is not there. An option that cannot do anything is a lie, and the same argument that
removed `:write` removes `'write'`.

### Which rows go is computed, not listed

The edit walks `options[]`, finds each row's `(char_u *)&p_xx` and counts readers of
that global outside the row, with `tools/dropoptions.py --strict`'s own exclusions —
another row, the row's `var` field, the variable's declaration (which is what the row
initialises), and taking the address, which asks which option a pointer refers to and
never touches the value. **Exactly seven of the 114 rows have no reader**, and the
program requires that set rather than naming six of them:

| row | var | indir | verdict |
| --- | --- | --- | --- |
| `fsync` | `p_fs` | `PV_BOTH` | goes, after `droplocal.py b_p_fs` |
| `modified` | `p_mod` | `PV_BUF` | **stays** |
| `prompt` | `p_prompt` | `PV_NONE` | goes |
| `readonly` | `p_ro` | `PV_BUF` | goes, by `WHIM-PLAN.md` II decision 5 |
| `undoreload` | `p_ur` | `PV_NONE` | goes |
| `write` | `p_write` | `PV_NONE` | goes |
| `writeany` | `p_wa` | `PV_NONE` | goes |

**`'modified'` has no reader of `p_mod` either and must not go.** Decision 5 keeps it:
the state it reports lives in `b_changed`, not in `p_mod`, so `:set modified?` answers
correctly and the row is not a lie. A computation that took "no reader" as the
criterion would delete it, which is why the seven are computed and the six are
*chosen*. `dropoptions.py` refuses it anyway, on the `PV_` guard. It is now the only
option row with no reader of its own global.

**`'prompt'` is the find, and `WHIM-PLAN.md`'s II row 11 computed four.** Its only reader
was `getexmodeline()`'s `if (p_prompt) msg_putchar(':');`, so it is **phase 87's
orphan**, collected here — which is a `uses` line the plan does not have.

**`'paste'` is exempt for ever**, and that is asserted rather than only written down:
`p_paste` has 12 mentions before and after, its five save slots `p_ai_nopaste`
`p_et_nopaste` `p_sts_nopaste` `p_tw_nopaste` `p_wm_nopaste` four each, and the edit
refuses outright if the computation ever offers `'paste'`. `+{command}` is likewise
untouched. That is the user's standing promise (`WHIM-PLAN.md` II.2d and decision 8), and
the next person to widen the computation meets the assertion and not just a comment.

### Four parts, and only one of them is live code

**A — four clean rows**, `dropoptions.py --strict prompt undoreload write writeany`.
The sweep then takes the four globals as `-Wunused-variable`.

**B — `'fsync'`, which `--strict` alone refuses**, and not on a reader: the row is
`PV_BOTH + PV_BUF + BV_FS`, so the tool stops on the `PV_` guard, because *the row is
what initialises the global* (`'tagcase'` taught that by segfaulting before the first
keystroke). `droplocal.py b_p_fs` is the other half and goes first — six plumbing
sites, including `get_varp()`'s two-line "local if set" form — then `--strict --local
fsync`.

**C — `'readonly'`, which is live code and not an inert row.** `p_ro` the global has
had no reader since whim; what survives is the buffer-local `b_p_ro`, at ten mentions,
and since phase 89 nothing but `:set ro` can set it, which is decision 5's premise.
Five edits, in this order and for this reason:

1. **the W10 warning.** `change_warning()` and its six call sites, each one statement
   on a line of its own. There is **no prototype** — it is defined above its first
   call — so a program that removes one fails loudly. This also takes the
   `ui_delay(1002L, TRUE)` that phase 85 named as one of the eight other pauses, and
   the `static char *w_readonly` inside the function.
2. **the `[RO]` in `fileinfo()`.** The format string and the argument move
   **together**, `%s%s%s%s%s%s` to `%s%s%s%s%s`, and nothing in the build checks a
   `vim_snprintf_safelen` count.
3. **the `[RO]` on the status line**, in `win_redr_status()`: the name-padding
   disjunct and the block that appends the indicator.
4. **`did_set_readonly()`, by name and with the reason.** It is the row's callback and
   the row is its only other reference, so the sweep would take it — but `droplocal.py`
   runs in the *same edit* and would find it still reading `b_p_ro`. Measured without
   it: `droplocal: b_p_ro still has 1 mentions after the plumbing went`, which is the
   tool working. The alternative is an inner sweep; this is cheaper and honest.
5. the row, then `droplocal.py b_p_ro` — three plumbing sites.

**D — what the sweep then finds**: `SHM_RO`, `BV_FS`, `BV_RO`, `w_readonly`, the six
globals, and the `b_did_warn` field — which becomes dead **only after both**
`change_warning` and `did_set_readonly` have gone. Remove one and it is a field with
one reader and one writer, which no tool reports.

### The flag letters are not touched, and that is a decision

`'cpoptions'` and `'shortmess'` each have a **validity list that is a separate string
literal from the value**, so removing a letter from a list cannot move `:set cpo?` or
`:set shm?`. But it *would* turn `:set shm=F`, accepted silently, into `E539: Illegal
character`, and no corpus case, Ex row, argv row or pty scenario types `:set shm=` —
which is exactly the kind of change core rule 2 exists to prevent. Accepting a letter that
does nothing is what upstream does for every feature a build lacks.

Measured: **23 of `'cpoptions'` 60 letters and 14 of `'shortmess'` 23 are inert** — in
a validity list with no enumerator of that value — and **this phase makes exactly one
more so, `'shortmess'`'s `r`**, whose `SHM_RO` goes with the `[RO]` indicator. Both
literals are asserted character for character, the inert sets are computed either side
and required to differ by exactly `{r}`, and four probes require `:set shm=F` and
`:set cpo=g` to be accepted silently on **both** binaries and `:set shm=y` and `:set
cpo=h` to answer E539 on both.

### The row floor, which this phase crosses

`tools/orphanopts.py` refused a table it parsed fewer than 100 distinct `&p_xx` out
of; this phase takes the count **102 → 96**, and its first call crosses it.
`tools/zerodelta.sh` runs that tool beside its harnesses, so crossing the floor does
not fail *this* phase — it fails the delta check of **every zero phase after it**, with
a message about a table that moved. It is the same failure shape as
`create_cmdidxs.py`'s 100-row floor at phase 91, arriving from a different table.

**The floor is 80 now, lowered in this phase's own commit**, with the reason in the
tool's docstring — the same number and the same argument as `create_cmdidxs.py`'s, so
that the two floors stay one idea. 80 leaves 16 globals of margin below 96 and the
plan removes no further rows. The check proves it **by using it**, not by grepping for
the number: the tool must not refuse, and its output on this source must be
byte-identical to its output on the input — five non-pointer orphans, which are
`'paste'`'s save slots, and every option pointer still with the row that sets it.

**It cost implementation keys, and that is stated rather than hidden.** `tools/whimdelta.sh`
names `orphanopts.py` and `tools/implhash.sh` hashes what a delta checker names, so
lowering the number re-keys whim and zero. Measured, before and after, over every
whim stage, every whim phase-as-unit, every whim edit, every slim phase and every zero
unit and edit: **12 whim stage keys, 4 whim edit keys, 82 whim phase-as-unit keys, 12
zero unit keys and 3 zero edit keys move, and not one slim key.** The tool's *verdict*
is unchanged everywhere — `slim-vim.c`, `whim-vim.c` and every zero boundary are far
above either floor, and the output is byte-identical — so no boundary can move; the
cost is CPU in a repass. `make whim-verify` and `make slim-verify` are the gate
`WHIM-GOAL.md` core rule 9 asks for, and both were run.

### The declared delta is nothing at all, and the reason is not phase 92's

Phase 92 removed code that **could not run**. This phase removes code that **can run
and that the instrument cannot see**. Measured record by record:

* `:set <name>?` goes from an answer to `E518: Unknown option`, and **no recorded case
  or row asks any of the six.**
* **bare `:set` does not move**, because none of the six differs from its default, and
  its listing is wiped by the Press-ENTER redraw before `zscreen.py` takes its picture.
* **`:set all` does move** — `readonly`, `fsync`, `prompt` and `undoreload` are in the
  old stream and absent from the new — and `:set all` is in no harness. It is a probe.
* **the W10 warning and the two indicators move**, and no recorded case sets
  `'readonly'`: they need `:set ro`, which nothing types.
* `tools/zexcmds.py` records `exit`, `bells`, `stderr`, `text` and `msgs` for the `set`
  row and **no stream digest**, so even a change to what `:set` prints in the stream
  would be invisible there.

So a phase that did nothing and a phase that did everything have the same recording.
`diff -rq` over two full recordings — the binary the phase was handed against the one
it made — is **empty**, and `tools/zerodelta.sh --phase 95` finds the nine lines phases
85 to 94 declared and nothing new. `pipes/zero.delta` gets a comment and no line.

### The probes, which are not a supplement but the check

**Twenty-seven, on both binaries**, thirteen required to move and fourteen not.

* **`ro_w10` is the one that shows behaviour going rather than a row.** `:set ro` on an
  **unmodified** buffer, then an insert: the old binary prints `W10: Warning: Changing
  a readonly file` and **pauses a second** — 1,006 ms measured against 2 ms here, the
  same shape as phase 85's 2,010 ms → 5 ms. The message is never in a snapshot: it is
  drawn, a Press-ENTER follows and the redraw wipes it, exactly as `whim90-check`'s
  E319, so the assertion is on the *stream* and on the elapsed time. The buffer must be
  unmodified when `:set ro` runs — `change_warning()` returned early on `b_did_warn ||
  curbufIsChanged()` — so a probe that types its seed first shows nothing on either
  binary.
* **`ro_ctrlg`** (`[readonly]`, not `[RO]`, because `'shortmess'`'s default has no `r`),
  **`ro_shm_r`** (`:set shm=r` first, the only probe that reaches `SHM_RO`) and
  **`ro_statusline`** (`+set laststatus=2`, which `win_redr_status` is reached by
  nothing else here).
* **`set_all`**, and the six `:set <name>?` spellings plus `:set readonly` and `:set
  ro`, each an answer before and `E518: Unknown option` after.
* **Fourteen that must not move and are required to be *doing* something**:
  `paste_roundtrip` (the exempt option, which the whole corpus depends on),
  `mod_query`, `shm_query`, `cpo_query`, `shm_F`, `shm_bad`, `cpo_g`, `cpo_bad`,
  `nu_query`, `bare_set`, `set_listing`, `ctrl_g` (still `[Modified]`), `undo_case`
  and an ordinary editing session.

**Proven able to fail in both directions**: with the new binary on both sides all
thirteen report *was to move and did not* and add *the input binary did not warn* and
*took 10 ms … under half a second means it never drew it*; with the old binary on both
sides they add *this binary still warns*, *took 1,006 ms, so something is still
pausing* and *`:set ro` does not answer E518 now, so something can still mark a buffer
read only*.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,866 | **79,757** (−109) |
| functions | 1,726 | 1,724 (−2) |
| type definitions | 910 | 909 |
| enumerators (DWARF) | 1,185 | **1,182** (−3, nothing renumbers) |
| struct fields | | **−3** (`b_p_fs`, `b_p_ro`, `b_did_warn`) |
| `options[]` rows | 114 | **108** |
| distinct `&p_xx` in `options[]` | 102 | **96** |
| `cmdnames[]` rows | 98 | 98 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 66 | **66** |
| binary | 804,360 | **803,912** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set. An option row is not a libc call, and `fsync` is still reached from
`ui_write()` and is the `FILE *` phase's.

**Three enumerators go and nothing renumbers, and `BV_RO` is why it needs saying**: it
is *unpinned*, so `BV_SI`, the next survivor, would follow it down. `tools/deadenums.py`
pins `BV_SI = 53` in the sweep and `enumvals.sh --verify` reports it there; the check's
independent dump either side requires `BV_SI` to hold its value and no other survivor
to move. `BV_FS` had an explicit value and so does its successor.

The sweep is **2 rounds** and the phase **37 s**. Its boundary is `fbaa6d80884b`.

### Its placement

`stage 95`, `package options`, and four `uses` lines: `options:95 seed:83 mechanical`,
because the declaration is "none" and "none" is checked against phase 83's baselines;
`options:95 files:89 mechanical` (`'write'`, `'writeany'` and `'fsync'` were
`do_write`'s, `not_writing`'s, `check_overwrite`'s and `buf_write`'s, and phase 89 left
`:set ro` as the only thing that could mark a buffer read only); `options:95 files:91
mechanical` (`'undoreload'` was read by `do_ecmd`); and **`options:95 streams:87
mechanical`, which the plan does not have** — `'prompt'`'s only reader was
`getexmodeline()`.

**`need 95 swept` is not required, and it was measured rather than assumed.**
`tools/phaserun.sh whim 94-95` runs phase 95's edit on the unswept text phase 94's edit
leaves, and every counted anchor matches: the same seven rows come back from the
computation and the cut ends at the same 108 rows and 96 globals. The run fails only
on the edit's build of its input binary, which is true of every zero edit that builds
one and is not declared for that reason.

**`apart 94 95`, measured.** Phase 94's check pins `p_ro` and `p_ur` at 2 mentions
**with their option rows** and says in as many words that removing one is the options
phase's. Run on the tree this phase leaves it gives six complaints — `p_ro has 0
mentions, expected 2`, `'readonly' lost its option row, and that is the options
phase's`, `curbufIsChanged has 6 mentions, expected 7` (`change_warning`'s early return
read it) and `the function count went 1742 -> 1724, expected 1742 -> 1726` among them —
and exits 1. Phase 93's check pins the same two rows and would fail too, but a stage
holding 93 and 95 holds 94 and `apart 93 94` forbids that already.

## Phase 96 (zero 13) — no `FILE *` that is never opened

`pipes/whim96-edit.sh` and `pipes/whim96-check.sh`, `stage 96`, `package tidy`. Two
`static FILE *` survive in this editor and **nothing has ever opened either of them in
any build of `zero-vim`**: `scriptin[NSCRIPT]`, which `-s {scriptfile}` filled and for
which whim removed the option, and `redir_fd`, which `:redir > file` filled and for
which whim removed the command. So this phase removes the **possibility** rather than
a behaviour — phase 92's situation and phase 92's answer.

### The counts are the argument, and each is computed before anything is folded

* **`scriptin[]` is assigned in exactly one place in the whole file**, and that place
  is `scriptin[curscript] = NULL;` inside `closescript()`. So it is NULL for ever.
* **`redir_fd`'s only assignment is its own declaration**, `= NULL`.
* **`ui_write()` has three mentions** — a prototype, a definition and one call — and
  that call passes `FALSE` for `console`.

The edit asserts all three as exact text before it folds anything, because every fold
rests on them; a fourth assignment anywhere would make every one a guess.

### Six anchors, in three groups

**A — `scriptin[]` is NULL for ever.** `may_sync_undo()` and `is_safe_now()` each lose
one conjunct and **survive**: `u_sync()` still runs on the same condition, and
`is_safe_now()` is still `stuff_empty() && typebuf.tb_len == 0 && !global_busy`.
`using_script()` is FALSE at both call sites — a `&& !using_script()` conjunct and a
`|| using_script()` disjunct — and the sweep then takes it. And `inchar()`'s script
reader goes as text with its local, after which `if (script_char < 0)` is always true
and folds; **that fold is what takes `closescript()`'s only caller**, and `fclose` and
`getc` with it.

**B — `redir_fd` is NULL for ever**, so `redirecting()` is FALSE always and folds at
both call sites, in `undo_cmdmod` and inside `redir_write()`. **Their indentation
differs**, which is what makes two separate one-count patterns honest rather than a
count of two over one pattern. The second fold takes the whole `fputs`/`putc` block.

**C — `ui_write()`'s `console`** is FALSE at its one call site, so the `vim_fsync(1)`
it guards can never be entered. **The parameter goes too**, and that is what makes the
cut honest: leaving it would leave `__attribute__((unused))` on something that will
never be read again — phase 85's argument for `check_tty(void)` — and `tools/sweep.sh`
compiles with `-Wno-unused-parameter`, so an unused parameter is invisible where an
unused local is not. `vim_fsync()` is then uncalled and `fsync` goes.

### Two locals are folded by hand and no tool covers either

**`retesc`** is written only inside the loop anchor A4 deletes and read once.
Afterwards it is a local that is **read and never written**: gcc has no warning for
that, `deadsweep.py` acts on warnings, and leaving it would mean `inchar()` returns an
uninitialised value on a path the compiler thinks exists. `return retesc;` becomes
`return FALSE;` and the declaration goes. It is folded **after** the two declarations
and **before** `fold_always`, because the fold dedents the body it keeps and a rewrite
counted against the original indentation refuses afterwards — which it did, the first
time this was run.

**`did_return`** is the same shape one level down: the `if (!did_return)` block the
`redir_write` extra removes is its only reader, and an `if` with an empty body is not
something any tool here removes either, so the block goes whole with `cutil.drop_if`
and the variable's two lines with it.

### The recommended extra is taken, and a second is declined

After B, `redir_write()` is `{ char_u *s = str; static int cur_col = 0; if (redir_off)
return; }` — the sweep takes the two variables and leaves a function with five callers
that cannot do anything. Leaving it is the "concept the table has and the code does
not" that whim's Phase 18 argued against, so it goes with its five call sites, and
`redir_off` — then written **five** times, not four, and read never, a file-scope
static that no warning covers — goes with them. `msg_puts_attr_len()`'s call was every
message the editor prints, and that is the one to notice: nothing is printed
differently, because `redir_write()` returned without doing anything at every one of
them.

**A second extra is declined and is a question for the user, not an oversight.** After
this phase `typedef struct stat stat_T;` has no user and `#include <sys/stat.h>` and
`#include <fcntl.h>` are needed by nothing. Removing all three is free and was
measured — same binary, byte-identical recording, four fewer lines — but it would be
**the first time any zero phase changes the directive count**, and the charter above
says `zero-vim.c` "inherits 18 directives from `whim-vim.c`". That sentence is a
statement about the pipeline, so the change belongs to whoever decides it, either here
or as an includes phase of its own. The count stays **18**.

**The user took it, as phase 99, and that phase found the paragraph above short by a
header and by a line.** There is a third that supplies nothing — `<iconv.h>`, whose
only occurrence in `zero-vim.c` is its own `#include` line, whim having removed the
conversion layer and left it — and the cut is five lines and not four, because the
typedef sits between two blank lines and one of them has to go with it. So the answer
here was **15 directives**, not 16, and phase 99 takes six because phases 97 and 98
emptied three more. The decision to decline was right for its reason: the charter now
says in as many words that a phase may remove a directive and may not add one.

### The honest problem, and the probe that answers it

Nothing this phase removes is reachable, so there is **no behavioural must-differ
probe** and no dishonest one is offered instead. The check builds the source the phase
was handed, twice:

- **probe** — `(void)write(2, "FILESTAR-ENTERED\n", 17);` at **five** places: the top
  of `closescript()`, inside `inchar()`'s `getc(scriptin[curscript])` loop, inside
  `redir_write()`'s `redirecting()` block, inside `undo_cmdmod`'s, and the top of
  `vim_fsync()`. **0 of the 106 records** carry the marker.
- **ctl** — the *identical* instrument at the top of `ui_write()`, which every byte
  the editor draws goes through. **105 of the same 106** carry it.

The zero is the claim; the 105 is what makes it a probe that can fail. **Proven able
to fail, by measurement**: with the instrument moved to `ui_write()` in the probe
build, the recording carries the marker in 105 of 106 records and the check reports
*105 of 106 records ENTERED one of the five sites on the binary this phase was handed*
and exits 1.

**Eighteen adversarial sessions** run on both instrumented binaries, and every one of
them is a way of making the editor **print**, which is where `redir_write()` sat —
`msg_puts_attr_len()` called it for every message. `:messages`, `:verbose set ai?`,
`:silent echo`, `:history`, `:registers`, `:display`, `ga`, an unknown command, `:set
all`, `:marks`, `:undolist`, `:changes`, `:map`, `:highlight`, `:normal ihi`,
`:g/a/p`, a recorded-and-replayed register and `:set verbose=9`. **Each reached
`ui_write()` and not one reached any of the five** — and the first half is checked
too, because a session that draws nothing is not an adversary.

### The declared delta is nothing at all, measured twice over

`diff -rq` over two full recordings — the binary the phase was handed against the one
it made — is **empty**: all 102 screen cases, all 111 Ex-command rows, all 30 command
lines, the four pty scenarios and the nineteen terminal rows. `tools/zerodelta.sh
--phase 96` then finds the same against whim-vim's frozen baselines, with the nine
lines phases 85 to 94 declared and nothing new. `pipes/zero.delta` gets a comment and
no line. Nine ordinary sessions run directly between the two binaries and each is
required to be identical **and** to be doing something.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,757 | **79,603** (−154) |
| functions | 1,724 | 1,719 (−5) |
| type definitions | 909 | 908 |
| enumerators (DWARF) | 1,182 | 1,181 (`NSCRIPT`, nothing renumbers) |
| `FILE` mentions | 2 | **0** |
| `#include` | 18 | 18 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 66 | **62** |
| `nm -u` with zero's own flags | 65 | **61** |
| binary | 803,912 | **799,816** |

**Four symbols go and the check names the set, not the count**: `fclose` and `getc`
were `closescript()`'s and `inchar()`'s script loop's, `putc` was `redir_write()`'s,
and `fsync` was `vim_fsync()`'s — whose only caller was `ui_write()`'s `console`
branch, which is why **`fsync` is this phase's and not the buffer-name phase's**.

**`fputs` does not go, and `WHIM-PLAN.md` II row 12 says it does.** After this phase the
source names it nowhere and `nm -u` still lists it: gcc lowers `fprintf(stderr, "…")`
to it, exactly as it lowers `printf` to `fputc`, `fwrite` and `putchar`. The check
asserts the freed set as exactly `fclose fsync getc putc`, with `fputs fputc fwrite
putchar __errno_location` named as gcc's own and required to be **still** undefined.

**`WHIM-PLAN.md` §II.4b's invariant is assertable in its strongest form now**, and the
check states it: `open creat openat stat access fcntl getcwd strerror fopen fdopen
opendir` are absent from **both** the source and the undefined set. The core has no
`open`, no `stat`, no stdio stream and no fourth descriptor — it can read, write,
close and dup fds 0, 1 and 2 and nothing else.

The sweep is **3 rounds** and the phase **37 s**. Its boundary is `f995296f2536`.

### Its placement

`stage 96`, `package tidy`, and two `uses` lines: `tidy:96 seed:83 mechanical`, because
the "none" is checked against phase 83's baselines, and `tidy:96 terminal:85 rationale`,
because `ui_write()`'s `console` argument is FALSE at its one call site either way and
phase 85 is where the terminal stopped being asked anything — so dropping the parameter
rather than leaving `__attribute__((unused))` on it is that phase's argument for
`check_tty(void)`.

**`need 96 swept` is not required, and it was measured**: phase 96's edit applies
unchanged to the *unswept* text phase 95's edit leaves, every counted anchor at the
same number. The run fails only on the edit's build of its input binary, which is true
of every zero edit that builds one.

**`apart 95 96`, measured.** Phase 95's check pins `scriptin` at 8 mentions, `redir_fd`
at 6 and `vim_fsync` at 3 and names all three as the `FILE *` phase's; it also requires
`fclose`, `getc`, `putc` and `fsync` to be **still** undefined. Run on the tree this
phase leaves it gives three complaints — `redir_fd has 0 mentions, expected 6`,
`scriptin has 0 mentions, expected 8`, `vim_fsync has 0 mentions, expected 3` — and
exits 1. Its symbol check would fail too, being a `cmp` of the whole undefined set
against a phase that frees four, but the source assertions come first. Phase 94's check
pins the same three and would fail as well, but a stage holding 94 and 96 holds 95 and
`apart 94 95` forbids that already.

## Phase 97 (zero 14) — the strings are the editor's own

`pipes/whim97-edit.sh` and `pipes/whim97-check.sh`, `stage 97`, `package vendor`.
Seventeen of the 61 libc symbols zero-vim still asked for are string and memory work,
and every one of them is **pure computation**: no descriptor, no clock, no signal,
nothing the host owns. So they are not a boundary to move, they are code the file can
simply contain. This phase brings sixteen of them in as `static musl_*` functions
written from `/root/musl/src/string/`, and moves the seventeenth — `sprintf` — onto
the printf this editor already carries. `nm -u` goes **61 → 44** and the recording
does not move at all.

### The measurement the phase turned on, and it was the open question

**gcc emits calls to `memcpy` and `memset` for itself**, for aggregate assignments and
large zero initialisers, whatever the source calls. So renaming every call site might
have left both symbols undefined and forced a definition under the **real** name — and
a definition of `memcpy` cannot be `static` without the question of whether gcc's own
emitted call still binds to it, which is the "nothing is global but `main()`"
invariant at stake. Measured on this file and it does not happen: after the rename
`gcc -S` contains **not one call to any of the seventeen**, and all seventeen leave
`nm -u`.

Two things make that a bound rather than luck, both measured. gcc's `-O0` inline-copy
threshold is **between 8 KiB and 16 KiB** — an 8,192-byte struct assignment is
inlined, a 16,384-byte one calls `memcpy`. And `-Wlarger-than=8192` on `zero-vim.c`
reports **exactly one** object above 8 KiB, `options[]` at 13,536 bytes, which is a
table nothing assigns whole, while `-Wframe-larger-than=8192` reports none. The check
asserts the absence from the **assembly** as well as from `nm -u`, so a later phase
that adds a big aggregate and assigns it whole fails loudly rather than quietly
reacquiring a libc symbol.

**Measured and not taken:** `-fno-builtin` and `-ffreestanding` each *add* `abs
fprintf labs` and remove `fputc fputs fwrite putchar` — a different set, not a smaller
problem, and none of it this phase's. `ZEROCFLAGS` is untouched, so this phase edits
no makefile and `whim.mk` needs no change. **And, for the record, because it was the
question asked:** a `static` definition *does* satisfy gcc's own emitted call — a
64 KiB struct assignment beside `static void *memcpy(…)` compiles to `call
memcpy@PLT` and the object has no undefined symbols at all. The escape hatch existed
and was not needed.

### `sprintf` is two populations, and the split is the whole story

Of its **22** occurrences, **thirteen** are ordinary call sites and **nine are inside
`vim_vsnprintf_typval` itself** — which is what `vim_snprintf` calls, so using the
in-house printf for those nine would be circular. The user's decision was to use the
in-house one; it applies to thirteen of the twenty-two and cannot apply to the rest.

The thirteen become `vim_snprintf(dest, size, …)`, each a statement whose return value
was already discarded. **Every size argument is knowable and none is invented**: eight
are `sizeof()` of a visible array or the constant the buffer was allocated with —
`IObuff` is `alloc((1024+1))` and `NameBuff` is `alloc(PATH_MAX)` — three repeat the
`alloc()` expression from three lines above, and one is a pointer **parameter** where
`sizeof(buf)` would be 8 and wrong. `highlight_arg_to_string`'s bound is
`MAX_ATTR_LEN`, and **that is sound only because the function has exactly one
caller**, `highlight_list_arg`, whose local is `char_u buf[MAX_ATTR_LEN]`. Both
programs assert that caller count and pin it at two mentions for ever: a second caller
with a smaller buffer would silently invalidate the bound and nothing else here would
see it.

The nine are narrower than they look. `f` is built twenty lines above the call and is
`%`, an optional `h`/`l`/`ll`, and one of `p d o u x X` — **no flags, no width, no
precision**, because vim does all of those itself in `tmp[]` before and after. So the
nine are "write this integer in this base", and they become `musl_fmtnum()` and
`musl_fmtptr()`, which have no format string and are not a printf. `musl_fmtptr()`
reproduces musl's `%p` exactly — musl's `vfprintf` does `p = MAX(p, 2*sizeof(void*));
t = 'x'; fl |= ALT_FORM`, so a null pointer is `0x0000000000000000` and not glibc's
`(nil)`. The `char f[6]` block goes with them: leaving it would draw
`-Wunused-but-set-variable`, which is in `-Wall`.

### One thing changes on one reachable input, and it is a bug fix

`t_CF` is a **user-settable option** that `term_font()` uses as a **format string**
into `char buf[20]`, and `sprintf` has no bound. Measured: `:set t_CF=` followed by
forty `X` and `%d`, then `:highlight Search ctermfont=3` and a search, exits **−11
(SIGSEGV)** on the binary this phase was handed, and exits **0** here with the output
truncated to nineteen characters. It is the only reachable input on which this phase
changes what the editor does, and the check requires **both halves** — the old one
must die and the new one must not.

It is **not** a declared delta, and the reason is the one phase 95 established:
nothing in the instrument sets `t_CF`, and the only built-in `t_CF` is the `debug`
terminal's `"[CF%d]"`. The same probe records the rest of the difference rather than
hiding it: a user-set `%f`, `%b`, `%*d` or `%z` now renders as vim's own printf spells
it rather than as musl's — `[0.000000]` → `[f]`, nothing → `[1101]`, a garbage int →
the argument, nothing → `[z]` — while `%d` and `%1$d` are identical. **`%s` segfaults
on both binaries** and is pre-existing, not this phase's, and the check says so.

### The case fold is inlined, so that the next phase stays independent

`musl_strcasecmp` and `musl_strncasecmp` **do not call `tolower`**, and musl's do.
musl's `tolower()` in the C locale is `(unsigned)c - 'A' < 26 ? c | 32 : c` and
nothing else — `tolower.c` is `if (isupper(c)) return c | 32; return c;` and
`isupper.c` is `(unsigned)c-'A' < 26` — so the arithmetic is written out. The cast is
what keeps a byte over 127 out of the range test, and the check reads both bodies to
confirm it is there. Inlining costs nothing and it is what keeps this phase and phase
98 orderable either way: phase 98 counts `tolower` mentions, and four new ones here
would have tripped it. **`tolower` is at 2 mentions before and after.**

### Four functions are vendored for code that cannot run, and the check says so

Breaking each moves nothing in the 106-record corpus or in the probes, and the phase
states that rather than offering a probe that cannot fail:

* **`musl_strpbrk`** — its one site needs `P_NFNAME` or `P_NDNAME`, and each of those
  has exactly **two** mentions in the file, its own enumerator and that one test. No
  `options[]` row carries either, so the condition is false always.
* **`musl_memchr`** — its one site is `vim_vsnprintf_typval`'s `%.*s`, and the only
  `%.*s` format string in the file is the OSC-timeout message.
* **`musl_strchr`'s NUL arm** — both call sites pass `'%'`.
* **`musl_fmtptr`** — nothing formats a pointer.

Their correctness rests on musl's source and on a standalone comparison against libc,
not on the recording. `musl_strstr` and `musl_strpbrk` are **naive loops** rather than
musl's two-way and bitset versions, which is the "performance is not a concern"
licence being used deliberately.

### The declared delta is nothing at all, for a third reason

Phase 92's "none" was code that could not run. Phase 95's was code the instrument
cannot see. **This phase removes no code and changes no behaviour**, and a recording
that moved would mean a vendored function was wrong. `diff -rq` over two full
recordings is empty and `tools/zerodelta.sh --phase 97` finds exactly the lines phases
85 to 94 declared and nothing new — screen 102/102, `ref-excmds.txt` 111/111,
`ref-argv.txt` 30/30.

**Thirty-three probes run on both binaries and are byte-identical**, and they exist
because the corpus reaches only part of this: every one of the thirteen external
`sprintf` sites (`:highlight`, `:marks`, `:changes`, `ga`, `:set sw?`/`all`, a
recording register, `:set term? t_Co?`, `t_CF` used properly), the numbers the nine
internal ones formatted (CTRL-G, the search count, a `:%s` count, the `Ndd`/`N>>`/undo
line reports, the ruler and its percentage), and the three things nothing else
reaches — `:history SEARCH` and `:history ALL` for the case fold, `:highlight Search
ctermfg=1` twice for `memcmp`, and `:set winhighlight=` for `memcpy`.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,603 | **79,884** (+281) |
| functions | 1,719 | **1,738** (+19) |
| type definitions | 908 | 908 |
| enumerators (DWARF) | 1,181 | 1,181 — **not one value moved** |
| `cmdnames[]` / `options[]` | 98 / 108 | 98 / 108 |
| `sprintf` mentions | 22 | **0** |
| `vim_snprintf` mentions | 55 | 68 |
| `#include` | 18 | 18 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 62 | **45** |
| `nm -u` with zero's own flags | 61 | **44** |
| binary | 799,816 | **803,912** |

**The freed set is named and not counted**: `memchr memcmp memcpy memmove memset
sprintf strcasecmp strcat strchr strcmp strcpy strlen strncasecmp strncmp strncpy
strpbrk strstr`, and **nothing arrives**. The binary grows by exactly **4,096 bytes**,
one page — the seventeen libc objects that stop being linked in roughly pay for the C
added. The sweep removes **nothing**, in one round, and `canon.sh` settles on the
first: the vendored text is already in the file's shape. The phase is **29 s** and its
boundary is `17a649164516`.

**The nineteen definitions are written the way the rest of the file writes one** —
the return type indented four spaces on its own line, the name at **column 0** — and
that is not cosmetic. `tools/funcreach.py` reads a definition as
`^([A-Za-z_]\w*)\([^;\n]*\)[ \t]*$`, so a one-line header is invisible to it. This
phase first emitted them on one line and the reachability sweep counted 1,719
definitions afterwards, exactly what it counted before the phase ran; phase 98's agent
found it. Nothing was broken by it — the block calls no vim helper, so nothing could
be orphaned, and `-Wunused-function` still covered a dead one — but it was a trap for
the first phase to make one of these call into the editor, and it was repaired while
it was cheap. The repair is pure formatting and its check is **tier 1**: both sources
built with `SOURCE_DATE_EPOCH=0` give a binary of 805,544 bytes and `cmp` says
byte-identical.

### Its placement

`stage 97`, `package vendor`, and two `uses` lines: `vendor:97 seed:83 mechanical`,
because the "none" is checked against phase 83's baselines, and `vendor:97 harness:86
mechanical`, because the one must-differ probe is a **screen** record of a binary that
segfaults — the old file-based sweep recorded an exit status and could not tell a
crash from a quit.

**`need 97 swept` is not required**: every anchor is exact text or a counted
identifier, and the phase is a stage of one, so its input is a boundary and is swept
by construction. **`apart 96 97` is real and is not written**: phase 96's check
asserts the libc surface moves by exactly `fclose fsync getc putc` and names `fputs
fputc fwrite putchar` as still undefined, and this phase moves seventeen more — but a
stage holding 96 and 97 holds them adjacent and every zero phase is its own stage, so
it is the shape of the missing `apart 85 89`. **`apart 97 98` is phase 98's**, and it is
the one that matters: this check pins `tolower` and `toupper` and requires both still
undefined, and phase 98 takes them.

## Phase 98 (zero 15) — the character classes, the numbers and the sort

`pipes/whim98-edit.sh` and `pipes/whim98-check.sh`, `stage 98`, `package vendor`. Phase
97 took the strings; this takes everything else in `zero-vim.c` that is **pure
computation** — a function of its arguments that asks the operating system nothing —
and defines it in the file, as plain C, with no preprocessor and no comment. Eleven
undefined symbols go: `tolower toupper towlower towupper isalnum iscntrl ispunct`, the
character classes; `atoi atol`, the numbers; and `qsort bsearch`.

### `nm -u` is not the scope, and that is why the phase is bigger than that list

musl spells six more classifiers as **function-like macros** in `include/ctype.h` —
`isalpha isdigit isgraph islower isupper`, and `isspace` through `__isspace` — so a
source that calls them produces **no undefined symbol at all**. Five of them are live
here, at **seventeen sites**: `isdigit` 7, `isupper` 5, `isalpha` 3, `islower` 1,
`isgraph` 1. Until they go, `<ctype.h>` cannot, and a phase scoped by the symbol list
would have looked finished and left phase 99 unable to move.

So this phase asserts **phase 99's contract itself**, in both directions: a copy of the
produced source with `#include <ctype.h>` and `#include <wctype.h>` deleted must
compile **silently**, and the same deletion on the source the phase was handed must
fail — measured, **13 errors**. That pair, and not a grep, is what says phase 99 can
move. Neither copy goes near the tree; this phase changes no directive and the count
stays 18.

**The sign-extension question is moot at every one of the seventeen sites, twice
over.** Fifteen already cast to `(unsigned char)`, and the two that do not are
`regatom()`'s, where `int cu` runs `1..127`. And the replacements are **total over
`int` and bit-for-bit equal to musl's for every argument**: EOF, `INT_MIN` and
everything outside 0..255 give false for every classifier and the identity for both
mappings, exactly as musl's do. Measured once, over **all 4,294,967,296 `int` values:
zero disagreements**, in 66 seconds.

### The obvious reading of `towupper` and `towlower` is wrong

Their one live arm each is `if (!(cmp_flags & CMP_INTERNAL)) return towupper(a);`
inside `utf_toupper()` and `utf_tolower()`, and `'casemap'` defaults to
`"internal,keepascii"` — so they look unreachable without `:set casemap=`. **An
instrumented build marks 104 of the 106 records of a full recording, 892 times in one
trivial session.** The caller is `utf_islower()`/`utf_isupper()` from
`buf_init_chartab()`, running **before `'casemap'` has been applied**, with `cmp_flags`
still its static zero and arguments in **exactly 128..255**. They classify the whole
Latin-1 range at startup, and nothing in the source says so.

The other two mentions, in `vim_toupper()` and `vim_tolower()`, sit on the line after
an unconditional `return`, and gcc drops the unreachable block even at `-O0` — which is
why `iswupper`, whose **only** mention has that shape, is a name in the source and not
a symbol in the object.

### Four answers were built, recorded and probed

| | lines added | binary | corpus | the case probes |
| --- | --- | --- | --- | --- |
| musl's `casemap.h` verbatim | +548 | unchanged | identical | identical |
| **range-compressed (taken)** | +557 | +1,632 | identical | **identical** |
| vim's own table instead | +166 | −4,096 | identical | 2 of 5 differ |
| ASCII only | +188 | −4,096 | identical | 2 of 5 differ |

musl packs the mapping into a two-level base-6 table, 297 lines and forty lines of
`(v*mt[y]>>11)%6` bit arithmetic: exact, and the opposite of obvious idiomatic C.
Routing the calls to vim's own table instead costs **97 upper and 96 lower codepoints,
96 and 96 of them above U+00FF** — the one below, U+00DF, is neutralised twice in the
source, by `utf_islower()`'s `|| a == 0xdf` and by `swapchar()`'s hard-coded ß→ẞ.

**The ASCII fallback is rejected with a number, not an opinion.** It misclassifies
**62 of 128** Latin-1 bytes at startup — every accented letter and µ — and *every
harness here says it is fine*, because `'isprint'` (`"@,161-255"`) and `'iskeyword'`
(`"@,48-57,_,192-255"`) re-cover the same bytes by range: `g_chartab` and `b_chartab`
come out byte-identical under all four variants, and stay identical under `:set isk=@`
and `:set isp=@`. A variant whose only defence is that two option defaults happen to
paper over it is not one to ship.

What runs is the fourth: **musl's mapping range-compressed into the shape this file
already has**, `187 + 171` `convertStruct` rows read by `utf_convert()` — the same size
as vim's own 198 + 183, and read by the same function. Exact, and in the file's own
idiom. **Dropping `'casemap'`'s non-internal arm is a phase of its own and deliberately
not this one**: it would delete those 358 rows and take the binary down 5,728 bytes,
and deciding what `'casemap'` means in a core is a different idea from vendoring.

### The two data files are not remembered constants

`tools/musl-case.txt` is generated by `tools/muslcase.py --generate`, which reads
**this machine's libc** through `ctypes`; `--verify` re-derives every one of the
**1,114,112** codepoints from the same authority and refuses on one disagreement, in
1.4 s. A musl upgrade that moved one codepoint fails the phase rather than passing it
quietly. `tools/musl-ctype.txt` is the seventeen functions, and `tools/muslctype.py
--verify` slices them **out of the source the phase produced** — not out of the data
file, which would only prove the copy was faithful — compiles them with `-Wall
-Wextra` and runs them beside libc's, in 0.35 s. Both are proven able to fail:
perturbing one `convertStruct` offset, one `& 0x5f`, one comparator direction and one
`return` in the binary search each makes the matching tool refuse.

**`muslctype.py`'s domain is bounded on purpose and its docstring says so.** The 2³²
sweep costs 66 seconds, which is more than the rest of the phase, and a check that
doubles a phase's time stops being run. What runs is every int in [−1024, 1024], every
threshold in the definitions and its two neighbours, EOF, `INT_MIN`, `INT_MAX` and a
**fixed** pseudo-random sample of 1,000,000 — 1,002,140 values. Every one of the eleven
is a closed form in `(unsigned)c`, so a disagreement anywhere is a disagreement at a
threshold, and every threshold is in the bounded set.

### Three anchors and sixteen counted rewrites

**A — the seventeen functions and two prototypes**, at the **end of the block phase 97
started**, between the last `#include` and the enum wall, so the file has one vendored
block and not two. The anchor is the junction itself, the end of `musl_fmtptr()` and
the first enum. The prototypes are needed only because the two **dead** `return
towupper(c);` mentions are rewritten too and sit below.

**They use the file's two-line definition style, and that is mechanical rather than
cosmetic.** `tools/funcreach.py` finds a definition with `^([A-Za-z_]\w*)\(…\)$` —
the *name* at column 0, which is what `    static int` on its own line gives. Phase
97 wrote one-line headers, and **measured, `funcreach.py` saw 1,719 definitions on
this phase's input, exactly what it saw before phase 97 ran**: not one of its
nineteen was in the reachability graph. Nothing was wrong — the block calls nothing
`funcreach.py` tracks, and `-Wunused-function` still covers a dead one — but a
function written that way is outside the sweep, so these seventeen are written the
way the other 1,719 are.

**B — the two tables**, after vim's own `toUpper[]`, so musl's sit beside vim's and
`utf_convert()` is already declared above them.

**C — `return iswupper(c);`, deleted** rather than vendored, and that is measured
rather than argued: same file name, `SOURCE_DATE_EPOCH=0`, the binary is
**byte-identical either side**.

**D — sixteen counted rewrites, 44 call sites**, applied only to the text *after* the
inserted block, so that no vendored body rewrites itself — `musl_ispunct()` calls
`musl_isalnum()`, and a file-wide regex would have made `musl_tolower()`'s body read
`musl_musl_tolower`.

**The check counts them as a rule and not as a table of constants**, and writing it
that way is what found that they are **44 and not 42**: for each name it requires
`calls(new, "musl_N") == calls(old, "N") + calls(vendored text, "musl_N")`, both halves
read at run time, so it says *every site moved and none was invented* on whatever input
it is handed rather than on the one it was written against.

### Nothing in `tools/sweep.sh` covers an unreachable statement

gcc's `-Wunreachable-code` has been a no-op since gcc 4.5 and is not in `-Wall
-Wextra`; `deadsweep.py` acts on warnings; and `funcreach.py` and `typereach.py` read
*definitions*, so a libc name with no definition here is invisible to them. An
unreachable statement is a **sixth kind of dead thing** beside the five the sweep
knows. Measured: `zero-vim.c` holds **eighteen** of them and exactly **one** names a
libc symbol. This phase takes that one. The other seventeen are a phase of their own —
three are the folded Latin-1 arms of these same four functions, and deleting those
would orphan `latin1flags`, `latin1upper` and `latin1lower`, which would turn a
vendoring phase into a cut. The check requires all three to **survive**, because a
check that expected them at zero would fail on a correct phase.

### `qsort` is stable on purpose, and it cannot matter

`sort_strings()` has one caller, `ex_undolist()`, and the keys begin `"%6ld"` of
`uh_seq`, which is `++curbuf->b_u_seq_last` — **one assignment in the whole file** —
so `strcmp` can never return 0 there. The replacement is an insertion sort, **stable**
where musl's smoothsort is not: where they could differ it returns the input order,
which is a function of the input alone, and that is what a memoized pipeline wants.

Measured three ways. The same `:undolist` row order as the input binary over five runs;
an **anti-stable** build (`>=` for `>`) giving the same order too, because there are no
ties; and a **reversed** build giving `5 4 3 2` where the others give `2 3 4 5`, which
is the control that proves the probe can see the sort at all. `tools/muslctype.py`
proves the converse in C, where the editor cannot: on three equal keys the two sorts
place **2 of 5** pointers differently while sorting to the same strings.

**`:undolist` is not visible in the screen snapshots** — the Press-ENTER redraw wipes
it before the `\x1b[?25h` that ends a step — so the probe reads the row order out of
the raw stdout stream, and compares the order rather than a digest, because the rows
carry `"0 seconds ago"`.

### `bsearch` is a binary search, and one of its four tables has a duplicate row

All four were checked by running their real comparators over their real data:
`highlight_tab` 13 rows, `color_name_tab` 28 and `char_class_tab` 19 are strictly
increasing; **`key_names_table` has two adjacent rows both spelled `"Tab"`**, one
carrying `TAB` and one `K_TAB`. It is still sorted, and it is not a hazard, because
`get_special_key_code()` ends `return key == K_TAB ? TAB : key`. Measured: both
binaries resolve `<Tab>` to row 100.

musl's algorithm is kept line for line rather than a linear scan, for two reasons that
are both about failure modes: a scan would silently **repair** a future unsorted table,
and the binary search resolves that tie identically. Against libc: **1,845 lookups**
over sizes 0..40 and every key, **the same pointer every time** — which is stronger
than "it found something".

### `atoi` and `atol`

musl routes both through `strtol`-shaped code; the replacement is the plain loop — skip
`isspace`, an optional sign, then accumulate **negatively** so that `INT_MIN` and
`LONG_MIN` do not overflow on the way in. **Overflow is undefined in the real ones
too**, and these are undefined in the same place and no other; all ten call sites can
already be handed arbitrary digits by a user or a terminal. `getdigits()` does not skip
the whitespace `atol` skips — upstream's, unchanged, and a tidier `atol` would have
changed it. Measured against libc: **137,560 strings**, every one of length 1..4 over an
alphabet holding whitespace, both signs, digits, letters and the two high bytes `\x80`
and `\xff` — **zero disagreements**.

### The declared delta is nothing at all, and it is a third kind

Phase 92's was code that **could not run**; phase 96's was a **possibility that had
never existed**; this one is an **equality**. The code it replaces runs constantly, so
the evidence is equivalence rather than unreachability and **every probe is a
must-not-differ**.

And the corpus reaches none of it: every one of the 102 screen cases seeds itself by
typing ASCII, so nothing in it touches Unicode case folding, `'casemap'`, the four
`bsearch` tables or the sort. **Fourteen probe sessions run on both binaries beside the
recording** — six over an ASCII, a Latin-1, a Greek, a Cyrillic, a circled Latin and a
Coptic letter under every `'casemap'` the option can hold, four for the four `bsearch`
tables, three for `atoi` and `atol`, one for the chartab those 892 startup calls build,
and `:undolist`. **The probe text holds two groups on purpose**: the first three
letters separate an ASCII fallback and the last two separate vim's own table, and a
text with only one group would pass one of the two wrong answers. Validated against
both: with the vim-table build `case_empty` and `case_keepascii` move; with the ASCII
build they move **and** stop showing the Greek capital.

Two full recordings, the binary the phase was handed against the one it made, are
byte-identical — all 102 screen cases, all 111 Ex-command rows, all 30 command lines,
the four pty scenarios and the nineteen terminal rows — and `tools/zerodelta.sh --phase
98` finds the same against whim-vim's frozen baselines. `pipes/zero.delta` gets a
comment and no line.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,884 | **80,440** (+556) |
| functions | 1,738 | **1,755** (+17) |
| type definitions | 908 | 908 |
| enumerators (DWARF) | 1,181 | 1,181 — none gone, none arrived, none renumbered |
| `#include` | 18 | 18 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 45 | **34** |
| `nm -u` with zero's own flags | 44 | **33** |
| binary | 803,912 | **805,544** |

**The binary grows, and that is the measurement rather than a disappointment.** The 358
rows are data the image did not carry, and the musl objects they replace were smaller
because musl packs the same mapping into 16,998 bytes. Phase 97 was the first phase
in this pipeline to make the file longer and the image bigger; this is the second.

**Eleven symbols go and the check names the set, not the count** — `atoi atol bsearch
isalnum iscntrl ispunct qsort tolower toupper towlower towupper`, with nothing
arriving — and **five more identifiers leave the source with no symbol to show for it**,
`isalpha isdigit isgraph islower isupper` being macros in musl. The check asserts the
terminal, the memory, the message layer, the clock and the signals still undefined
beside them, and `WHIM-PLAN.md` §II.4b's invariant again.

The functions row is `funcreach.py`'s, and the **+17 is this phase's seventeen**. Both
absolute numbers here are 19 higher than they first read, because phase 97 emitted its
nineteen definitions with the header on one line, where `funcreach.py` cannot see them;
this phase's agent found that, phase 97 was restyled into the file's own shape, and the
difference the row records did not move.

The sweep is **1 round** and removes **nothing** — every one of the seventeen new
functions is called — and `tools/canon.sh` is a **no-op** on the output, so the
inserted text is already in the file's canonical form. The phase is **41 s**: 7 s the
edit and its build of the input binary, 6 s the sweep, 24 s the check and 4 s
`tools/zerodelta.sh`. `make whim-tip` is 48 s. Its boundary is `b1b12e506ceb`.

### Its placement

`stage 98`, `package vendor` with phase 97, and two `uses` lines: `vendor:98 seed:83
mechanical`, because the "none" is checked against phase 83's baselines, and `vendor:98
harness:86 mechanical`, because "nothing moved" is a statement about a zero recording
and a zero recording is what phase 86 made one. Phase 97 is in the same package, so the
ordering between them is the package's and not a `uses`.

**`need 98 swept` is not required, and it was measured**: every anchor is exact text,
every count was taken on unswept input, and the sweep removes nothing here.

**`apart 97 98`, measured.** Both checks state that the undefined set moved by exactly
**their** symbols — phase 97's seventeen strings, phase 98's eleven — and a stage
holding both takes one symbol snapshot at the stage's start. Run with a snapshot from
before phase 97, phase 98's check reports **28 symbols gone instead of 11** and exits
1. Phase 97's check fails the other way for a reason of its own: it pins `tolower` at
**2 mentions** and names it as the character-class phase's, and on the tree phase 98
leaves it is **0**.

`make whim-verify` reproduces every boundary in **110 s** of wall time over 863 s of
phases -- sixteen, q83 to q98, when this phase was written, and seventeen since -- and
all 107 whim and slim implementation keys are unchanged.

## Phase 99 (zero 16) — the includes nothing names

`pipes/whim99-edit.sh` and `pipes/whim99-check.sh`, `stage 99`, `package includes`.
`zero-vim.c` inherited **eighteen** preprocessor directives from `whim-vim.c`, every
one an `#include` of a system header, and fifteen phases removed none of them. Six are
now needed by nothing, and this phase takes them — **the first zero phase to change
that count**, and the only one whose evidence is a byte comparison rather than a
recording.

### Six headers, and three of them have been dead all along

| | why it is unused | since |
| --- | --- | --- |
| `<sys/stat.h>` | supplies **nothing**: its one user is `typedef struct stat stat_T;`, and nothing uses `stat_T` | before the pipeline |
| `<fcntl.h>` | supplies **nothing at all** — `fcntl`, `creat`, `openat` and every `O_*` at zero mentions | phase 92 |
| `<iconv.h>` | supplies **nothing at all**, and nobody had noticed: `iconv` occurs exactly once in `zero-vim.c` and that once is its own `#include` line | whim |
| `<string.h>` | the sixteen `mem*`/`str*` functions | phase 97 |
| `<ctype.h>` | **ten** identifiers | phase 98 |
| `<wctype.h>` | `towlower`, `towupper` — and `iswupper` | phase 98 |

**`<iconv.h>` is the find, and it was missed twice.** `WHIM-PLAN.md` §II.4 measures this
cut as "79,599 lines and **16 directives**", and `pipes/whim96-edit.sh` names two
headers where there are three. Both were counting `<sys/stat.h>` and `<fcntl.h>` and
neither looked at the rest; the answer is **15 directives** at that point and twelve
here.

**`<ctype.h>` is ten identifiers and not five, and the five that are invisible are the
interesting half.** `isalnum`, `iscntrl`, `ispunct`, `tolower` and `toupper` are real
calls and appear in `nm -u`. `isalpha`, `isdigit`, `isgraph`, `islower` and `isupper`
are musl **macros** — `#define isalpha(a) (0 ? isalpha(a) : (((unsigned)(a)|32)-'a') <
26)`, where the `0 ?` arm keeps the prototype visible and is never emitted — so they
are in no undefined set at all and a survey driven by the symbol list cannot see them.
Seventeen occurrences of real source, and `<ctype.h>` does not go until all ten are
handled.

**`iswupper` is the same shape one step further on**: a name the header had to supply
that was never a symbol either. Its one occurrence sits directly after
`return utf_isupper(c);` inside `vim_isupper()`, so gcc never emitted the call. Phase 98
deleted the statement rather than vendoring a function nothing calls.

### The typedef, which is the one silent drop in this file

`typedef struct stat stat_T;` goes in the same edit as its header, and that is not
tidiness. **Removing `<sys/stat.h>` alone compiles cleanly** — the typedef simply
declares a new, *incomplete* `struct stat` at file scope — and what is left is a lie
that only `sizeof(stat_T)` would ever expose. Measured both ways: `-Wall -Wextra
-Wno-unused-parameter` is silent on that file, and a two-line probe using `stat_T` by
value gives *"invalid application of `sizeof` to incomplete type `stat_T` {aka `struct
stat`}"*. The check's loop could not have caught it, because the loop asks the compiler
and the compiler is content.

**And no sweep could ever have taken it, for two textual reasons, both measured.**
`tools/typereach.py` takes as roots every identifier mentioned outside a type
definition, and this definition's name set is `{stat, stat_T}`. It is kept alive by
`update_search_stat()`'s local variable `searchstat_T stat;` **and by the `#include
<sys/stat.h>` line itself**, whose text contains the token `stat`. Measured:
`typereach.py` reports `0 unreachable` on the committed file, `0 unreachable` with only
the include gone, `0 unreachable` with only the local renamed, and **`1 unreachable —
stat,stat_T`** only when both are gone. Fifteen phases of sweeps had left it.

**One blank line goes with it.** The typedef sits between two blank lines, so deleting
the line alone leaves a run of two, which `CLAUDE.md` states this tree does not have —
and which neither verification tier can see. `tools/canon.sh` would collapse it inside
the sweep; the edit does it, so the text the sweep is handed is already right. Six
includes, the typedef and one blank is **eight lines**.

### The argument is a computation and not a list

A phase that deleted six named headers would prove only that six named headers were
deletable. `pipes/whim99-check.sh` proves something else, and it is the whole phase:

* **on the output**, each of the twelve surviving `#include`s is removed in turn and
  the compile **must fail**. A dead include that survived this phase would be a compile
  that succeeded.
* **on the source the phase was handed**, the identical loop over eighteen must find
  **exactly the six this phase removes** droppable and the other twelve not. That is
  the same loop proving it can fail, in the same run and on the same code path — phase
  96's `ui_write()` control in this phase's shape.

Thirty compiles, run at once, about five seconds. **gcc 15 defaults to C23, where an
implicit function declaration is a hard error**, so a header that still supplies a
function, a type, a macro constant or an enum constant cannot be dropped quietly:
there is no `-Wimplicit-*` to look for because there is nothing left to warn about.
Every one of the twelve refusals is recorded in the phase's output, so the check is a
statement of *why* each survivor is held as well as a test that it is.

Proven able to fail three ways, each measured on a scratch tree: a live `offsetof`
rewritten to `__builtin_offsetof` leaves `<stddef.h>` dead and the loop names it; the
typedef put back with its header gone is caught by the assertion above and not by the
loop; and one character changed in one string literal is caught by the `cmp`.

### `<sys/param.h>` is not touched, and the reason is recorded rather than repaired

Its **own** contribution to this file is `MIN` and `MAX`. Everything else it supplies
arrives through three levels of musl-internal inclusion — measured with `gcc -E -H`:

```
sys/param.h -> sys/resource.h -> sys/time.h -> sys/select.h
```

and `select`, `gettimeofday`, `fd_set`, `FD_SET`, `FD_ZERO`, `FD_ISSET`, `struct
timeval` and every `*_MAX` are supplied by **no other header in this file** — measured,
one probe per identifier against each of the eighteen. `zero-vim.c` has no
`<limits.h>`, no `<sys/time.h>` and no `<sys/select.h>`. That is a real fragility, and
it is written down instead of being fixed: fixing it means **adding** three directives,
and the charter says no phase adds one. If musl ever reorganises those headers the
build breaks outright, which is the loud failure and the acceptable one.

**Two of the twelve are held by almost nothing**, and the edit counts those exactly,
because the count is the statement. `<stddef.h>` is held by `offsetof` **alone**, nine
mentions — `size_t` and `NULL` come from six of the twelve, so nothing else there is at
risk. `<stdint.h>` is held by exactly **two** identifiers at one mention each:
`SIZE_MAX`, which has held it all along, and `uintptr_t`, **which phase 97 brought** —
the first thing in this pipeline's history to make a header *more* held rather than
less, and the reason the number is two. A later phase that took them would find this
check's loop reporting a dead include.

### The declared delta is nothing at all, and it is a fourth kind

Three phases before this one declared nothing, each for a different reason, and the
reason is the statement: **9** removed code that could not run, **12** removed code that
can run and that the instrument cannot see, **13** removed the possibility. **This phase
changes no code at all**, and its evidence is not that the recording did not move but
that **the binary is the same bytes** — the input's and the output's, both built with
`SOURCE_DATE_EPOCH=0` and the boundary's own flags, compared with `cmp`.

That is tier 1 of `CLAUDE.md`'s verification table, and it subsumes every screen case,
every Ex-command row, every command line and every pty scenario at once, because the
program that would be run is literally the same program. `tools/zerodelta.sh --phase 99`
runs and finds nothing moved, as it must; here it corroborates rather than proves.

**`SOURCE_DATE_EPOCH` is required and the file name is not.** `version.c`'s
`__DATE__ " " __TIME__` is the only thing in `zero-vim.c` that a build can vary — there
is no `__FILE__` and no `__LINE__` anywhere — so two ordinary builds of the same bytes
differ, measured, while the same source built under two different names and from two
different directories is identical. The `cmp` is also as sensitive as a comparison can
be: gcc writes a GNU build-id note near the front of the image and it is a hash of the
whole output, so any difference anywhere moves it and the first difference `cmp` reports
is always that note.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,440 | **80,432** (−8) |
| `#include` | **18** | **12** |
| other directives | 0 | 0 |
| functions | 1,755 | 1,755 — untouched |
| type definitions | 908 | **907** (−1, `stat_T`) |
| DWARF enumerators | 1,181 | 1,181 |
| `nm -u`, as `phasecheck.sh` counts it | 34 | **34 — the same set, as a `cmp`** |
| `nm -u` with zero's own flags | 33 | **33** |
| external symbols | `main` | `main` |
| binary | 805,544 | **805,544, byte-identical** |
| sweep | | **1 round, a complete no-op** |

**Nothing is freed and nothing arrives**, and the check states it as a `cmp` of the
whole undefined set rather than as a count: a header is not code, so a symbol moving in
either direction would mean the phase had done something it does not claim to do. The
sweep finds nothing at all — 0 prototypes, 0 functions, 0 variables, 0 types, 0 fields,
0 enumerators, `canon settled` — and the file it hands back is the one the edit wrote.
`main` is still the only external symbol, which is also the cheapest re-assertion that
phases 97 and 98 did not forget a `static`.

### Its placement

`stage 99`, `package includes`, and four `uses` lines: `includes:99 seed:83 mechanical`,
because the "none" is checked against phase 83's baselines; `includes:99 tidy:96
rationale`, because `pipes/whim96-edit.sh` names `<sys/stat.h>` and `<fcntl.h>`,
measures that removing them is free and **declines** — *"the count stays 18"* — so this
phase is that decision reversed; and one line each to `vendor:97` and `vendor:98`, whose vendoring
is the only reason `<string.h>`, `<ctype.h>` and `<wctype.h>` are unused.

**`need 99 swept` is not required, and it was measured.** The anchors are six exact
`#include <...>` lines at one occurrence each and one exact typedef line — text no
sweep has ever touched — and the computed part is a *compile* rather than a count, so
it cannot shrink silently on unswept text the way a counted cut can. Measured: the edit
applies unchanged to unswept text, and the sweep that follows it removes nothing.

**`apart 98 99`, and it is measured in both directions.** Phase 98's check requires the
three headers it emptied to be still present — "it is the includes phase's to take" —
and this phase removes them. And phase 99's check states that it frees **nothing**, as
a `cmp` of the stage's starting undefined set against the one it made; inside a stage
every check compares with the *stage's* start, and phase 98 frees eleven symbols, so in
a shared stage phase 99 says *"the libc surface moved, and REMOVING AN `#include`
CANNOT MOVE IT"* and names them. It is `apart 88 89`'s shape exactly.

**The edit refuses loudly on a tree the vendoring has not reached**, which is what
makes the six a requirement rather than a wish: run on the phase-13 boundary it says
*"`<string.h>` is NOT unused: memchr (1), memcmp (2), memcpy (7), memmove (159) …"* and
exits 1 with the file untouched.

### What zero-vim is after sixteen phases

```
zero-vim.c        80,432 lines          from whim-vim.c's 86,614  (-6,182, 7.1%)
functions         1,755
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      33 with zero's flags, 34 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**Two of those numbers went the other way and that is the trade.** The file is 829
lines longer than it was after phase 96 and the binary 5,728 bytes larger, because
phases 97 and 98 move code *in*: twenty-eight libc functions are `static` definitions
here now instead of names a host has to answer. `WHIM-GOAL.md` measures bytes to store
**and** libc symbols to provide, and this is the first place the two disagree.

**The 33, attributed.** The whole host boundary that is left is a terminal, a message
line, memory, a clock and the process:

| why | symbols |
| --- | --- |
| **the terminal** | `read` `write` `close` `dup` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` `isatty` (10) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals and exit** | `sigaction` `sigaddset` `sigemptyset` `sigismember` `sigprocmask` `kill` `raise` `getpid` `exit` `_exit` (10) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

**Four rows are gone and they were the pure computation**: strings and memory blocks
(17, with `sprintf`), character classes (7), numbers (2), sorting and searching (2).
Phases 97 and 98 put all 28 inside `zero-vim.c` as `static` definitions, `sprintf`
going onto the editor's own `vim_snprintf` rather than being copied. Nothing in the
list above is a function of its arguments alone: every one of the 33 asks the
operating system something.

`tools/symbols.sh` counts 34 because it compiles plain `-O0` and so adds
`__stack_chk_fail`, which zero's `-fno-stack-protector` removes. **`isatty` survives
with three call sites** and belongs to the terminal, not the filesystem —
`mch_check_win`'s `isatty(1)`, `mch_get_shellsize`'s `!isatty(fd) &&
isatty(read_cmd_fd)` and `fill_input_buf`'s `!did_read_something &&
!isatty(read_cmd_fd)`.

**The filesystem work is finished.** Phases 89 to 96 took, in order: every way to write
a file, every way to read one, every way to name another one to edit, the machinery
that read the bytes, the buffer's own name with the last three questions the core
asked a disk on its own initiative, the refusal that asked whether the text had been
saved, the option rows that reported settings nothing read, and the two `FILE *` that
were never opened.

**And the pure computation is finished too.** Phases 97, 98 and 99 took the other half
of *no musl dependencies*: the twenty-eight functions a host should never have been
asked for, and then the six headers that had nothing left to supply. What remains of
`WHIM-PLAN.md` part II is the host boundary itself: `main()` demoted to a launcher, the
terminal and the signal set moved out of the core, and the text representation changed
from lines to a tree. **Its §4c expected strings, memory and arithmetic to be what was
left in the core after that move; they are already gone.** Phases 101 and 102 are the
demotion itself: the launcher exists, at the bottom of the same file, and the core
asks it to end the process rather than ending it.

## Phase 100 (zero 17) — the deadly ladder that cannot run

`pipes/whim100-edit.sh` and `pipes/whim100-check.sh`, `stage 100`, `package host`. Nine
lines, one libc symbol, and the smallest zero phase so far. `deathtrap()` — the handler
for the deadly signals — opens with a ladder that counts how often it has been entered:

```c
    if (entered >= 3)
    {
        reset_signals();
        if (entered >= 4)
        {
            _exit(8);
        }
        exit(7);
    }
```

**`entered` cannot reach 3 in any build of zero-vim**, so this removes a *possibility*
and not a behaviour. That is phase 96's kind of cut rather than phase 92's: phase 92 took
code that had *become* unreachable, when phases 88 to 91 removed every way to name a file,
and this — like the two `FILE *` — was never reachable in anything the pipeline has ever
produced.

### Why it cannot run, and why the obvious reason is the wrong one

Two facts, and **neither is zero's doing**:

* `catch_signals()` installs the deadly handler with `sigemptyset(&sa.sa_mask)` and
  **`sa.sa_flags = 0`**. No `SA_NODEFER`, so the signal being handled is blocked for the
  duration of its own handler.
* `signal_info[]` carries exactly **two** rows with `deadly = TRUE`, `SIGHUP` and
  `SIGTERM`. `SIGSEGV`, `SIGBUS`, `SIGILL` and `SIGFPE` are at **zero mentions** in
  `zero-vim.c` — whim removed all four — so there is no third deadly signal to arrive.

Two signals, each blocked inside its own handler, lets `entered` reach **2** — TERM
nested inside HUP's handler or the reverse, which is the `Vim: Double signal, exiting`
arm, and that arm calls `getout(1)` and never returns. It cannot reach 3: by then both
are blocked and nothing else is caught.

**The wrong reason is available and would be wrong elsewhere.** A phase that removed the
ladder because *"`reset_signals()` makes it unreachable"* would have the right answer for
the wrong reason — `reset_signals()` is **inside** the ladder and is never reached — and
would be wrong on any tree with three deadly signals. What the edit asserts, character for
character, is the two-row table and the `sa_flags = 0` arm, on the **output**, so that a
later phase cannot quietly falsify either without this check saying so.

### The argument is two binaries differing in one field

`pipes/whim100-check.sh` instruments the source the phase was **handed** —
`write(2, "DTn\n", 4)` immediately after `++entered;`, and `write(2, "DTLADDER\n", 9)` as
the first statement inside the ladder — and builds it five ways:

| | what it is | what it must report |
| --- | --- | --- |
| `in_mark` | the input, marked | **bombarded**: 8 concurrent sessions, 60 alternating SIGTERM/SIGHUP each at full speed. Every session reaches the handler, **the maximum `entered` ever observed is 2**, `DTLADDER` never appears |
| `in_forced` | plus `raise(SIGHUP)` in `preserve_exit()` and `raise(SIGTERM)` in the `entered == 2` arm | exactly `DT1 DT2`, no ladder, **exit 1** |
| `in_nodefer3` | the same source with **one field changed**, `sa.sa_flags = SA_NODEFER` | `DT1 DT2 DT3`, `DTLADDER`, **exit 7** — `exit(7)` running |
| `in_nodefer4` | plus one more forced signal at `entered == 3` | `DT1..DT4`, `DTLADDER`, **exit 8** — `_exit(8)` running |
| `out_forced` | **the output**, instrumented and forced identically | `DT1 DT2`, exit 1, and the same screen `in_forced` drew |

**`in_forced` and `in_nodefer3` are the whole phase in two binaries.** They differ in one
`sigaction` field and nothing else, and the ladder runs in one and not the other — so
both statements this phase deletes are live code that only the signal mask keeps out of
reach, and the bombardment's zero is a probe that is *proven* able to report otherwise.
That is phase 96's `ui_write()` control in this phase's shape, and it is stronger: the
control is not a different place in the program, it is the same place with the reason
removed.

The bombardment's assertion is deliberately **one-sided** — no session may report 3 or
more — so machine load can only ever weaken it, never make it fail spuriously. What load
*could* do is stop a session reaching the handler at all, which would make the zero
meaningless, so every session is required to have reported a depth. Measured over eight
runs: 8 of 8 sessions always reach it and between 2 and 8 of them reach depth 2.

### The stream is not comparable, and the screen is

The uninstrumented pair — the binary the phase was handed and the one it made — is sent a
single SIGTERM and a single SIGHUP, and the two must agree. **They cannot be compared as
byte streams.** The editor emits `\x1b[?4m`, a private mode that draws nothing, at a point
that depends on when its own flush happened: measured, `in_forced` and `out_forced` came
back 2,136 bytes each, byte-for-byte equal in three runs out of six and differing at
offset 2,003 in the other three, the whole difference being those five bytes sitting
before `\x1b[?25l` rather than after `\x1b[?25h`. The plain pair flaked the same way.

So the comparison is **what the editor drew** — `tools/zscreen.py`, zero's own instrument:
every snapshot, the final screen and the bell count, none of which a private mode touches
— beside the exit status and stderr, which are bytes and are compared as such. And the
screen is required to *carry* `Vim: Caught deadly signal TERM`/`HUP` and `Vim: Finished.`,
so the equality is not two blank screens agreeing. Six consecutive runs of the whole
phase were green.

**The harness waits on content, not on a clock**, for the same reason. A fixed sleep
signals the editor wherever its redraw had got to, and with sixteen sessions running at
once that is a recording of the machine's load; a quiet-for-300 ms drain was still wrong
once, on a startup that took longer than that to produce its first byte. The wait is for
the typed text to be on the screen *and then* for quiet, and a session that never gets
there is reported rather than compared.

### The counting trap

`exit` is a word this file uses for four things that are not a call. In the input:

```
    "Type  :qa!  and press <Enter> to abandon all changes and exit Vim"   a string
    "Type  :qa  and press <Enter> to exit Vim"                            a string
            exit(7);                                              this phase's
        exit(r);                                                  mch_exit's
                        goto exit;                                a goto, and
exit:                                                             its label, both
                                                                  in vim_regsub_both()
```

So **`assert exit at 0 mentions` fails on a correct phase**, and `assert 'exit(' at 0`
fails on `mch_exit(`, `preserve_exit(` and `getout(`. The assertion that works is
**`nm -u`** — the gone set is exactly `{_exit}`, nothing arrives, and `exit` is required
to be *still* undefined, being `mch_exit`'s and a later phase's. `_exit` as a word is
unambiguous, the label being `exit`, and it is asserted as well.

### Nothing is orphaned, and `reset_signals()` is the one that looks as though it should be

The ladder held one of `reset_signals()`'s four mentions; `mainerr()` holds another, so
the function stays and the check pins it at three. The sweep after the edit is a complete
no-op — 0 prototypes, 0 functions, 0 variables, 0 types, 0 fields, 0 enumerators, `canon
settled` — and `funcreach.py` reports 1,755 of 1,755 definitions reachable either side.

### The declared delta is nothing at all

`pipes/zero.delta` gets a comment and no line, and the reason is the statement. Five
phases now declare nothing and each for a different reason: **9** removed code that could
not run, **12** removed code that can run and that the instrument cannot see, **13**
removed the possibility, **16** changed no code at all, and **14** and **15** replaced
code with code that computes the same answers. **This one is 96's**: the ladder has never
been reachable in any build of zero-vim, so a recording that *moved* would mean the cut
was wrong. `tools/zerodelta.sh --phase 100` finds the corpus unmoved, as it must.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,432 | **80,423** (−9) |
| functions | 1,755 | 1,755 — untouched |
| type definitions | 907 | 907 |
| DWARF enumerators | 1,181 | 1,181 |
| `nm -u`, as `phasecheck.sh` counts it | 34 | **33**, gone set exactly `{_exit}` |
| `nm -u` with zero's own flags | 33 | **32** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| binary | 805,544 | **805,544 — the same size, different bytes** |
| sweep | | **1 round, a complete no-op** |
| phase | | **17 s** |

**Nine lines of code that never ran cost no bytes at all**, which is worth saying because
it is the opposite of what the line count suggests: the image is the same 805,544 and the
two binaries are not the same bytes, so what the ladder occupied was absorbed by alignment
padding. This phase's measure is the symbol, not the size.

### Its placement

`stage 100`, `package host` — new, and it is where `main()`'s demotion and `mch_exit`'s
one remaining `exit(r)` will go — with two `uses` lines, `exit:17 seed:83 mechanical` and
`exit:17 harness:86 mechanical`, which is what every phase declaring "nothing moved" owes.

**`need 100 swept` is not required**, and the anchors say why: every one is exact text at a
counted occurrence — the nine-line ladder, the six `exit` lines one by one, `signal_info[]`
and `catch_signals()`'s deadly arm — and none of it is text any sweep has ever touched.

**`apart 99 100`, measured.** Phase 99's check requires the file to have lost **exactly
eight** lines and this phase takes nine more: `tools/phaserun.sh whim 99-100` reports *"the
file lost 17 lines, expected 8"* and exits 1. Its libc check would fail as well — phase 99
states as a `cmp` that it frees **nothing**, inside a stage every check compares with the
*stage's* start, and this phase frees `_exit` — but the source assertions come first. It
is `apart 98 99`'s shape in **one** direction only: phase 100's own check passes on a 16-17
stage, phase 99 having freed nothing for it to be blamed for. And **no `apart 98 100`** is
written, although phase 98's check does require `_exit` to be still undefined: a stage
holding 98 and 100 holds 99, and `apart 98 99` forbids that already.

### What zero-vim is after seventeen phases

```
zero-vim.c        80,423 lines          from whim-vim.c's 86,614  (-6,191, 7.1%)
functions         1,755
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      32 with zero's flags, 33 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 32, attributed.** One row of the table moved and it is the last row of the host
boundary that is not a device:

| why | symbols |
| --- | --- |
| **the terminal** | `read` `write` `close` `dup` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` `isatty` (10) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals and exit** | `sigaction` `sigaddset` `sigemptyset` `sigismember` `sigprocmask` `kill` `raise` `getpid` `exit` (9) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

**`exit` is now a single call site**, `mch_exit`'s `exit(r);`, and that is what this phase
was for as much as the symbol: the next phase in this package has one line to replace in
one function rather than three in two.

## Phase 101 (zero 18) — `main()` is demoted to `vim_main()`

`pipes/whim101-edit.sh` and `pipes/whim101-check.sh`, `stage 101`, `package host`. Five
lines, no libc symbol, and `WHIM-PLAN.md` §II.4c's first step. What was

```c
    int
main
(int argc, char **argv)
{
    ...
    return vim_main2();
}
```

becomes `static int vim_main(int argc, char **argv)` with the **same body, byte for
byte**, and a six-line launcher is appended below it:

```c
    int
main(int argc, char **argv)
{
    return vim_main(argc, argv);
}
```

The editor runs one call frame deeper and does exactly what it did. Nothing else moves;
this is deliberately the only thing the phase does.

### Both stay in `zero-vim.c`, and that is the point rather than a compromise

Two tools hard-code today's invariant: `tools/phasecheck.sh`'s `grep -v '^main$'` and
`tools/funcreach.py`'s `{'main'}` root. Splitting the launcher into a second translation
unit is what breaks both, and the cost was measured while `exit` was being reviewed —
**one appended line to `tools/phasecheck.sh` moves 118 implementation keys**: 12 whim
stages, 82 whim edits, 12 zero units, 12 zero edits, and no slim key. So every demotion
that *can* be done inside one file is done inside one file, and the split happens once,
late, when there is nothing left to do before it.

**`vim_main` is `static` for the same reason.** Nothing outside this file calls it, and
a non-static one would be the first external symbol any zero phase has ever added.
`nm --extern-only --defined-only` on the object still prints exactly `main`.

### The name was checked for a collision rather than assumed

`vim_main2()` already exists — it is upstream's, the second half of the old `main()`
split at the point where the screen is up — and `vim_main` had **zero** mentions as a
whole word. C has no prefix collision, but a reader greps, so the edit and the check pin
all three words separately: `main` 1 (the launcher's head, and the only bare `main` in
80,000 lines), `vim_main` 2 (its definition and the one call — a third would be a
prototype, and a function defined above its only call needs none), `vim_main2` 2
(untouched). `\b` does not match inside `main_loop`, `main_errors` or `vim_main2`, and a
substring grep does.

### A fossil goes with it, and it turns out not to be cosmetic

`main()`'s head was spelled over **three** lines because upstream had an `#ifdef`
between the name and the argument list, giving MS-Windows a different signature; slim's
phase 5 took the conditional and left the line break. Every other function in this file
spells its head over two, so `vim_main` gets the ordinary shape and the new `main` gets
it too.

**Measured: `tools/funcreach.py`'s definition finder never matched the three-line head.**
`main` had never been one of the definitions it counts — its `{'main'}` root was a name
added by hand to a set that did not contain it. So 1,755 definitions become **1,757**
for one new function, and the second is `main` itself, seen for the first time. All
1,757 are reachable.

### The evidence is every way the editor can end

The binary is **not** byte-identical and is not asserted to be: at `-O0` an extra call
frame is real code. Measured, both built `SOURCE_DATE_EPOCH=0` with the boundary's own
flags: **805,544 bytes either side — the same size, different bytes**, the frame
absorbed by alignment padding. So phase 99's tier-1 argument is not available here and
something else has to stand in its place.

What stands in its place is the six routes `WHIM-PLAN.md` part II maps that a probe can reach
from outside, run on **both** binaries:

| | how it starts | path | status |
| --- | --- | --- | --- |
| `quit` | `+q!` | `ex_quit` → `getout(0)` | **0** |
| `cquit3` | `+cq 3` | `ex_cquit` → `getout(3)` | **3** |
| `eof` | stdin at `/dev/null` | `read_error_exit` → `preserve_exit` → `getout(1)` | **1** |
| `badopt` | `-Z` | `mainerr` → `mch_exit(1)` | **1** |
| `sigterm` | SIGTERM | `deathtrap` → `preserve_exit` → `getout(1)` | **1** |
| `sighup` | SIGHUP | the same | **1** |

`return vim_main(argc, argv);` puts a value in the program's path that was not there
before, and that table is what says it arrives. **The binary the phase was handed is
required to give the same six**, so an agreement cannot be two wrong answers agreeing.

**And the table is proven able to fail.** The output is built a second time with
`mch_exit`'s `exit(r)` changed to `exit(r + 1)` — one character — and all six move:
1, 4, 2, 2, 2, 2. Six statuses that agree prove nothing unless a wrong one would have
been caught, which is phase 100's `SA_NODEFER` control in this phase's shape.

**`/dev/null` and not a pipe, and that is a measurement.** With stdin a pipe the harness
closes, the EOF row came back as a twenty-second timeout on four binaries out of five
and as a clean 1 on the fifth — a race in the *harness*, not in the editor. A file that
is already at end of file has no race in it, and the four non-signal rows are then
deterministic over repeated runs.

### The declared delta is nothing at all

`pipes/zero.delta` gets a comment and no line, and this is a **sixth** kind of empty
declaration. The five before it each removed *something*: **9** code that could not run,
**12** code that can run and that the instrument cannot see, **13** a possibility, **16**
no code at all with the binary the same bytes, **14** and **15** code replaced by code
that computes the same answers. **This one adds a call frame and removes nothing**, so
there is nothing to declare and nothing for a recording to show. `tools/zerodelta.sh
--phase 101` finds the corpus unmoved, as it must: 102 of 102 screen cases, 111 of 111
Ex-command rows, 30 of 30 command lines.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,423 | **80,428** (+5) |
| functions `funcreach` counts | 1,755 | **1,757** — one new, and `main` seen at last |
| type definitions | 907 | 907 |
| `nm -u`, as `phasecheck.sh` counts it | 33 | **33, identical as a `cmp`** |
| `nm -u` with zero's own flags | 32 | **32** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| binary | 805,544 | **805,544 — the same size, different bytes** |
| sweep | | **1 round, a complete no-op** |
| phase | | **22 s** |

### Its placement

`stage 101`, `package host` beside phase 100, with `uses host:101 seed:83 mechanical` and
`uses host:101 harness:86 mechanical` — the two every phase declaring "nothing moved"
owes.

**`need 101 swept` is not required.** Both anchors are exact text at a counted
occurrence — the three-line head, and the file's last two lines — and neither is text a
sweep has ever touched.

**`apart 100 101`, measured.** Phase 100's check requires the file to have lost **exactly
nine** lines and this phase adds five, so on a shared stage the one swept text 17's
check is handed is four lines shorter than its input rather than nine:
`tools/phaserun.sh whim 100-101` on q99 reports *"the file lost 4 lines, expected 9"* and
exits 1. It is `apart 99 100`'s shape in **one** direction only — phase 101's own check
compares against the text *its* edit was handed, which is 17's output either way, so it
passes on a 17-18 stage.

### What zero-vim is after eighteen phases

```
zero-vim.c        80,428 lines          from whim-vim.c's 86,614  (-6,186, 7.1%)
functions         1,757  (1,755 + vim_main, + main itself, now a shape funcreach sees)
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      32 with zero's flags, 33 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**Nothing in the table moved but the line count and the function count**, which is what
a phase that renames one function and adds another is entitled to move. `exit` is still
`mch_exit`'s single call site; the next phase in this package is the one that takes it.

## Phase 102 (zero 19) — the core can no longer stop the process

`pipes/whim102-edit.sh` and `pipes/whim102-check.sh`, `stage 102`, `package host`.
Eighteen lines, one libc symbol, and `WHIM-PLAN.md` §II.4c's second step. `mch_exit()`'s
last statement stops being `exit(r);`:

```c
static void (*vim_host_exit)(int);

    static void
mch_exit(int r)
{
    ...
    ml_close_all(TRUE);

    vim_host_exit(r);
}
```

`vim_main()` takes the callback as a third parameter and installs it as its first
statement, and phase 101's six-line launcher becomes twenty:

```c
static void *host_jump[5];
static int host_code;

    static void
host_exit(int r)
{
    host_code = r;
    __builtin_longjmp(host_jump, 1);
}

    int
main(int argc, char **argv)
{
    if (__builtin_setjmp(host_jump) != 0)
    {
        return host_code;
    }
    return vim_main(argc, argv, host_exit);
}
```

The editor no longer ends the process. It hands the process back, with a number.
**`nm -u` 32 → 31, the gone set exactly `{exit}`, and nothing arrives** — an indirect
call through a pointer names no symbol, and *returning* from `main()` ends the process
without naming one either.

### Why a function pointer, and why the other two routes are not available

* **Thread a status up through every caller.** Not expensive — *not writable*.
  `cmdnames[].cmd_func` is `void (*)(exarg_T *)` for all 98 rows and
  `nv_cmds[].cmd_func` is `void (*)(cmdarg_T *)` for all 194, each dispatched through
  one indirect call, so every handler would have to change signature together; and
  `deathtrap` is `void (*)(int)` by the kernel's contract and cannot participate at
  all. Say it plainly, because "thread the value up" is the first thing a reader
  proposes.
* **A `setjmp` in the core.** It puts the mechanism in the file that is meant to stop
  naming mechanisms, and it costs symbols.
* **The core calls out and does not come back.** `vim_host_exit(r);` is four words of C
  that say exactly that; the host decides *how*. It is the one route whose C text
  already says what a JVM host would have to do — an interface call whose
  implementation throws.

**The indirection is temporary and `WHIM-PLAN.md` §II.4c says so.** It exists because
everything is still one translation unit and *nothing is global but `main()`* is still
the invariant: a pointer the launcher installs through a parameter adds no external
symbol, where a `musl_exit(int)` the host defines would. Once the file is split there
*is* a declared boundary, `vim_host_exit` becomes a plain `musl_exit(int)` prototype at
the top of the editor file, and the parameter and the pointer both go.

### The mechanism in the launcher is measured, and the review that proposed this phase got it wrong

Returning from `main()` is what ends the process without naming `exit`, and getting
back to `main()` from inside `deathtrap` needs a non-local jump. All four spellings,
measured on this tree:

| the launcher jumps with | `nm -u` | what it costs |
| --- | --- | --- |
| **`__builtin_setjmp`/`__builtin_longjmp`** | **32 → 31** | nothing arrives; no header |
| `sigsetjmp`/`siglongjmp` | 32 → **33** | `+sigsetjmp` `+siglongjmp`, `+<setjmp.h>` |
| `setjmp`/`longjmp` | 32 → **33** | `+setjmp` `+longjmp`, `+<setjmp.h>` |
| the launcher calls `exit(r)` | 32 → 32 | nothing moves; the phase achieves nothing |

**The two library spellings are net worse than not doing the phase**: `exit` leaves and
two symbols arrive in its place, plus a thirteenth `#include` in a file whose last
phase but two removed six. The review this phase comes from recommended `sigsetjmp`,
having counted the *core* at 42 with the launcher's cost attributed to a host file that
does not exist yet; in one translation unit there is no separate. So
`pipes/whim102-check.sh` **builds the `sigsetjmp` variant on every run** and requires
`nm -u` to show 33 against the output's 32, with `sigsetjmp` and `siglongjmp` present
and `exit` gone from both — the road not taken as a number rather than a memory. It is
compiled to an object; it is an answer, not a program.

### The one thing `sigsetjmp` would have bought, and the measurement that says it is not needed

`__builtin_longjmp` does not restore the process signal mask and `siglongjmp` does, so
after a jump out of `deathtrap` on SIGTERM the landing site still has SIGTERM blocked.
**That is exactly the state the process already died in.** Measured on the source this
phase was handed, with a `sigprocmask`/`sigismember` probe immediately before
`exit(r);`, and on the output with the identical probe immediately before the launcher
returns:

| | before `exit(r);` (input) | before `return host_code;` (output) |
| --- | --- | --- |
| SIGTERM | `TERM-MASKED HUP-CLEAR` | `TERM-MASKED HUP-CLEAR` |
| SIGHUP | `TERM-CLEAR HUP-CLEAR` | `TERM-CLEAR HUP-CLEAR` |

`exit()` was always being called from inside the handler with the handled signal
blocked. SIGHUP is clear only because `prepare_to_exit()` calls
`mch_signal(SIGHUP, SIG_IGN)`, which unblocks it on the way past. So the builtin
**preserves** the mask the process ends with and `siglongjmp` would have **changed**
it. The check asserts the pair every run, and requires the input's half to be non-empty
so the equality is not two silences agreeing.

A host that keeps running rather than returning is where the mask would matter, and
there is none: `main()` lands and returns four lines later. When the file is split that
host writes `musl_exit(int)` for itself and owns the question along with `sigprocmask`,
which is a symbol the *host* is allowed to name.

**One thing is deliberate and is said here rather than discovered later.** A non-local
jump out of a signal handler is undefined by the letter of C11 when the signal
interrupted a function that is not async-signal-safe, which here it always does —
`deathtrap` already calls `out_str`, `sprintf`, `ml_close_all` and `free`, and upstream
has always done that and got away with it because the process was about to die. The
design that removes it is the signal handlers becoming the host's — `sig_winch`'s
`do_resize = TRUE; return;` applied to the deadly two — which is a later phase and the
first of these with a real declared delta.

### The evidence

The six routes again, on both binaries, every one of which now leaves `mch_exit`
through the pointer, lands in `main()` and comes back as a **return value**:

```
quit=0   cquit3=3   eof=1   badopt=1   sigterm=1   sighup=1
```

**Proven able to fail**: the output built again with `host_exit`'s `host_code = r;`
made `host_code = r + 1;` — one character — moves all six, to 1, 4, 2, 2, 2, 2. That is
the value travelling from `mch_exit` through a function pointer, into a jump buffer and
out of `main()`, and the control is what says the table measures it.

**One finding the harness cost, and the program now records it.** The EOF row's status
is a statement about the harness's fd 2 as much as about the editor.
`fill_input_buf()` answers a read of nothing on a non-tty fd 0 with `close(0);
vim_ignored = dup(2);` and tries again — so the EOF route's *second* read is a read of
whatever stderr is. Measured on the binary this phase was handed: with stderr at
`/dev/null` the second read is another end of file, `read_error_exit` runs and the
status is 1; with stderr a **pipe the harness holds open**, the editor waits there for
keys that never come and the row times out on every binary. Both probes use
`/dev/null`. `tools/zstream.py`'s docstring has the same finding from the other end,
about `vim -`.

### The counting trap, one phase further on

`exit` as a word is **5** in the input and **4** in the output, and the number of
statements beginning with `exit(` is **1 → 0**. The four that stay are two string
literals (`"Type  :qa!  and press <Enter> to abandon all changes and exit Vim"` and its
shorter twin) and a `goto exit;` with its `exit:` label inside `vim_regsub_both()`. So
`assert exit at 0 mentions` fails on a correct phase, and `assert 'exit(' at 0` fails
on `mch_exit(`, `preserve_exit(`, `prepare_to_exit(`, `read_error_exit(`, `getout(` and
now `vim_host_exit(` and `host_exit(` as well. The assertion that works is `nm -u`, and
it is asserted in both directions: `exit` `_exit` `abort` `_Exit` `quick_exit` `atexit`
and every spelling of a jump must be **absent**.

### The declared delta is nothing at all

`pipes/zero.delta` gets a comment and no line, and it is phase 101's kind. Everything
`mch_exit` does before the changed line is untouched — the terminal restored, the
screen scrolled, the memfile closed — so what the editor *draws* on its way out cannot
move, and the recording is of what the editor draws. **18 and 102 are the first two
phases in this pipeline whose empty declaration means neither "nothing ran" nor "the
instrument cannot see it": the code runs, the instrument sees it, and it does the same
thing.** `tools/zerodelta.sh --phase 102` finds the corpus unmoved: 102 of 102 screen
cases, 111 of 111 Ex-command rows, 30 of 30 command lines.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,428 | **80,446** (+18) |
| functions | 1,757 | **1,758** (`host_exit`) |
| type definitions | 907 | 907 |
| `nm -u`, as `phasecheck.sh` counts it | 33 | **32**, gone set exactly `{exit}` |
| `nm -u` with zero's own flags | 32 | **31** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| binary | 805,544 | **805,544 — the same size, different bytes** |
| sweep | | **1 round, a complete no-op** |
| phase | | **24 s** |

### Its placement

`stage 102`, `package host` beside 100 and 101, with `uses host:102 seed:83 mechanical` and
`uses host:102 harness:86 mechanical`.

**`need 102 swept` is not required**: every anchor is exact text at a counted occurrence
— `mch_exit()`'s definition and its tail, `vim_main()`'s head, and the file's last six
lines — and none of it is text a sweep has ever touched.

**`apart 101 102`, measured, and the measurement found a better reason than the four that
were predicted.** Phase 101's check fails at its **first act**: it builds its own control
by rewriting `mch_exit`'s `exit(r);` to `exit(r + 1);` with `sed`, and refuses when
that changes nothing — and this phase has replaced that line. `tools/phaserun.sh zero
18-19` on q100 reports *"the control edit changed nothing — mch_exit's `exit(r);` is not
where this phase expects it"* and exits 1. Three more of its assertions would have
failed after it — the `cmp` on an undefined set that a stage snapshots once at its
start, the five-line gain where a 18-19 stage gains 23, and the six-line launcher it
requires the file to end with. **One direction only**: phase 102's own check compares
against the text *its* edit was handed and its gone set from q100 is still exactly
`exit`, 18 having freed nothing for it to be blamed for, so it passes on a 18-19 stage.

### What zero-vim is after nineteen phases

```
zero-vim.c        80,446 lines          from whim-vim.c's 86,614  (-6,168, 7.1%)
functions         1,758
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      31 with zero's flags, 32 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 31, attributed.** One row of the table lost its last member:

| why | symbols |
| --- | --- |
| **the terminal** | `read` `write` `close` `dup` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` `isatty` (10) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals** | `sigaction` `sigaddset` `sigemptyset` `sigismember` `sigprocmask` `kill` `raise` `getpid` (8) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

**The row used to be "signals and exit" and it is now "signals".** What is left of the
host boundary is a terminal, a clock, three allocations and eight signal calls — and
§4c's remaining step is the one that takes the first and the last of those out
together.

## Phase 103 (zero 20) — the signals and the terminal are the host's

`pipes/whim103-edit.sh` and `pipes/whim103-check.sh`, `stage 103`, `package host`.
`WHIM-PLAN.md` §II.4c's third step, and it is **one** phase where the plan and two
surveys had two. The signals half on its own leaves the terminal in **raw mode** after
`kill -TERM`, because the only way to delete the core's signal handlers is to delete
`deathtrap`, and `deathtrap` is what restores it. Keeping `deathtrap` and having the
host merely *install* it costs nothing and keeps all of it:

```c
    static void
musl_host_init(void)
{
    host_catch(SIGHUP, deathtrap);
    host_catch(SIGTERM, deathtrap);
    host_catch(SIGWINCH, host_on_winch);
    host_catch(SIGCONT, host_on_winch);
    host_catch(SIGTSTP, host_on_tstp);
    host_catch(SIGINT, host_on_int);
    host_catch(SIGPIPE, SIG_IGN);
    host_catch(SIGALRM, SIG_IGN);
}
```

`deathtrap → preserve_exit → prepare_to_exit → term_leave()` still runs, and phase
102's `vim_host_exit` → `__builtin_longjmp` turns `mch_exit(1)` into `return 1`, so the
handler needs nothing of its own. Measured, identical on both binaries: `kill -TERM`
exits 1, restores the terminal to `ICANON=1 ECHO=1 ISIG=1 ONLCR=1 ICRNL=1`, and draws
`Vim: Caught deadly signal TERM` and `Vim: Finished.` in **2,241 bytes** — SIGHUP the
same in **2,240**. There is no phase 104.

### Within one translation unit, moving code frees nothing — and the phase says so

A symbol leaves `nm -u` when its last **caller** leaves the file, and that is the
split. `sigaction`, `sigemptyset`, `kill`, `ioctl`, `tcgetattr`, `tcsetattr`,
`nanosleep`, `select` and `__errno_location` all survive this phase; every one of them
is now called from a 229-line host block at the bottom of the same file and from
nowhere else. **What leaves is what the phase DELETES**, stated as one `comm` because
the two halves are one phase:

```
nm -u  31 → 24, gone exactly
    close  dup  isatty  raise  sigaddset  sigismember  sigprocmask
arrived: nothing
```

`raise` was `sig_tstp`'s re-raise. `sigaddset sigismember sigprocmask` were
`mch_signal()`'s fifty lines emulating `sigset()`, where the host's `host_catch()` is
four lines of `sigaction`. `isatty` is all three surviving calls. `close` and `dup` are
`mch_tcgetattr`'s dead `close(tty_fd)` and `fill_input_buf`'s `close(0); dup(2)` arm.

**So the phase's real claim is structural, and it ships the assertion for it.**
`tools/zhostonly.py` is new, zero-only, and named by `pipes/whim103-check.sh` alone: 43
host words — `sigaction`, `kill`, `ioctl`, `tcsetattr`, `select`, `nanosleep`, every
`SIG*`, `struct termios`, `fd_set`, `ICANON`, `VMIN` — and **every mention of every one
of them must be inside the host block**. It reports 60 mentions of 43 words, all of
them there. Seven exceptions are named with their reason and their exact count, and
they are the whole of what the core still says: `signal_info[]` and `deathtrap` name
`SIGHUP` and `SIGTERM` because that is the **message** the editor prints,
`vim_handle_signal` re-raises a deferred deadly signal with `kill`, and `elapsed_T` is
`struct timeval` because it is the clock. It ignores string literals — the file
contains `hash_remove(&buf_hashtab, hi, "close buffer")`, and a tool that read that as
a `close()` would report the buffer layer as filesystem code — and `#` lines, because
`#include <errno.h>` and `#include <sys/ioctl.h>` name two of the words. It refuses to
pass vacuously: the host region must be found, must define all fourteen of its
functions, and must itself mention `sigaction ioctl tcsetattr nanosleep select kill`.
Without it the strongest available form of *the core names none of this* is
`sigaction` at 2 mentions, which says nothing about **where**.

### The in-band protocol was already in the file, and it could never run

This is the finding that reframed the phase. `handle_csi()` has always parsed DEC
private mode 2048's notification — `CSI 48 ; rows ; cols ; hpx ; wpx t`, specified by
Tim Culverhouse in 2024 — and carried the whole negotiation around it:
`win_resize_setting`, `win_resize_enabled`, `term_set_win_resize()`, the `'termresize'`
option with its `inband`/`sigwinch` values, and the `CSI ? 2048 h` / `l` it writes.

**And none of it could ever run.** `win_resize_setting` is written in exactly one place
in `zero-vim.c` — inside the parser for the DECRQM *response*, `CSI ? 2048 ; Ps $ y` —
and `\033[?2048$p`, the DECRQM **query** that is the only thing a terminal answers with
that response, appears **nowhere in `slim-vim.c`, `whim-vim.c` or `zero-vim.c`**. The
editor never asks, so it is never told, so `win_resize_setting` is 0 for ever,
so `term_set_win_resize(true)` always takes its first branch and sets
`win_resize_enabled = false`, so the notification arm is unreachable. Proven by running
it: typing `\x1b[48;30;100t` at the committed binary inserts the escape **as buffer
text**. So the phase deletes the negotiation, the option that drove it and the two
state variables, and switches the parser on — and the **host** writes the
notification, from its own SIGWINCH handler and its own `ioctl`.

That is the cheapest half of the phase, and it is half a deletion of code that could
not run rather than half a rewrite.

### Why the host keeps its `ioctl`, which is the decision the whole design turns on

Mode 2048 is a good protocol that almost nothing implements. Source-verified:

| implements 2048 | does **not** |
| --- | --- |
| ghostty, kitty, foot, iTerm2, contour, Bobcat | **xterm, tmux, GNU screen, WezTerm, Alacritty, VTE/gnome-terminal, Konsole, Windows Terminal, rxvt-unicode, st, Rio, zutty, xterm.js, Zellij, mintty, Terminator, mlterm, Warp, Hyper** |

**tmux and GNU screen terminate it**: their private-DECSET whitelists stop short of
2048, their DECRQM arms answer `CSI ? 2048 ; 0 $ y` — the correct *"not recognised"* —
and neither forwards `CSI ? 2048 h` outward. A core that relied on the terminal would
be an editor that never learns it was resized under tmux. The alternative query,
XTWINOPS `CSI 18 t`, is a **round trip** whose answer races real keystrokes in the same
stream, is unsupported by `st`, and can be switched off by xterm's `allowWindowOps` —
and it answers "what size am I", never "did it change".

So the host keeps `SIGWINCH` and `TIOCGWINSZ` and **injects** the notification the
spec defines. Deleting the host's `ioctl` instead would free `ioctl` and an eleventh
`#include` and cost exactly that list of terminals: a number bought with the terminal.

### The four messages become bytes, and the one that cannot

Four of the five handlers existed to *tell the editor something*, and a host cannot
deliver a signal to a core it is linked into — there is no such thing. So the four
become bytes on the channel that already exists:

| the host catches | the core reads |
| --- | --- |
| `SIGWINCH`, `SIGCONT` | `CSI 48 ; rows ; cols ; 0 ; 0 t` — the mode-2048 notification |
| `SIGTSTP` | `\033[?1z` |
| `SIGINT` | the byte `0x03` |
| `SIGHUP`, `SIGTERM` | nothing — it is not a message, it is the process ending |

**`\033[?1z` is ours and is in no spec**, and that is said here rather than discovered.
`first == '?' && argc == 1 && arg[0] == 1 && trail == 'z'` reaches no existing arm —
the `?`-prefixed arms end in `c`, `y` and `u` — and `z` is a letter, so the trail scan
stops on it. Doing real work from inside the termcode parser has precedent in the file:
the resize arm calls `set_shellsize()`, which redraws the whole screen.

**The interrupt travels as the byte it already is, and it is not optional.**
`catch_sigint` set `got_int` directly. The host instead hands the core a `0x03`, which
`fill_input_buf`'s own CTRL-C scan turns into `got_int` — the only way `got_int` is
ever set in raw mode anyway, because raw mode clears `ISIG`. **With no handler at all,
`SIG_DFL` KILLS the editor**: measured, `kill -INT` mid-session leaves the binary this
phase was handed editing and kills a no-handler build with SIG2. The `10gs` probe is
what found it, because the sleep mode deliberately leaves `ISIG` on (below).

**Keyboard CTRL-Z is the one thing that cannot go in band**, and `musl_suspend()` is
why there is one call out and not zero. In raw mode `ISIG` is clear, so CTRL-Z arrives
as the byte `0x1a`, reaches `nv_cmds[]`'s real `{Ctrl_Z, nv_suspend, 0, 0}` row, and
**the core decides** to suspend. A one-way host→core channel has no way to carry that
decision back. The external `kill -TSTP` is the other direction and fits the in-band
path exactly, which is the asymmetry stated once:

```c
    static void
musl_suspend(void)
{
    host_catch(SIGTSTP, SIG_DFL);
    kill(0, SIGTSTP);
    host_catch(SIGTSTP, host_on_tstp);
}
```

`mch_suspend()` goes from 27 lines to eight, and `in_mch_suspend`, `sigcont_received`
and the four-iteration `mch_delay` back-off loop go with it. The first existed only so
the core's own `sig_tstp` could tell *my* stop from *someone else's*, and the second
only to drive the loop: **the call returning is the handshake.**

### The terminal is two operations, not a mode setter

`settmode(tmode_T)` had nine call sites and a three-valued mode, and every site is
attached to an **operation** — the editor takes the terminal, gives it back, suspends,
resumes, sleeps, or re-asserts that it still holds it. Exposing `musl_set_raw(int)`
would move the syscall without moving the responsibility. So `settmode()` splits at the
one line that was ever the host's:

```c
    if (termcap_active && tmode != TMODE_SLEEP && cur_tmode != TMODE_SLEEP)
        ... out_str(t_CBD); out_str_t_TE();     /* leaving  -- screen work, stays */
        ... out_str_t_BE(); out_str_t_TI();     /* entering -- screen work, stays */
    out_flush();
    mch_settmode(tmode);                        /* <- the only host part */
```

into `term_enter()` and `term_leave()`, which keep the escape sequences because those
are screen work. `cur_tmode` becomes a boolean `term_entered`, `mch_cur_tmode` goes —
the two were equal at every one of the eleven call sites, over 521 recorded
observations — and `tmode_T` with `TMODE_COOK`, `TMODE_RAW` and `TMODE_SLEEP` goes to
the sweep, along with `MCH_DELAY_SETTMODE`, which had no caller.

**The two sites that look like exceptions are re-assertions, and a re-assertion is an
idempotent operation.** `getcmdline_int`'s `settmode(TMODE_RAW)` found the terminal
already raw on all 521 observations, and `ask_yesno`'s runs only `if (exiting)`, after
`prepare_to_exit` cooked it. Both are `term_enter()`.

### `TMODE_SLEEP` leaves `ISIG` on deliberately, and the `gs` probe is what proves it

The parameter is `interruptible`, not `discard_input`, and the code says so. Measured
from `mch_settmode`: the sleep mode is the **saved** termios with `ICANON` and `ECHO`
cleared and `VMIN=1 VTIME=0`, applied **`TCSANOW`, not `TCSAFLUSH`** — nothing is
flushed and nothing is discarded. What is different from raw mode is that **`ISIG` is
left on**, so the terminal's INTR character raises `SIGINT` and cuts the `nanosleep`
short instead of sitting in the input queue.

That is load-bearing, and a phase that "simplified" `musl_delay()` into a plain
`nanosleep` would break CTRL-C during `gs` with no recorded harness able to see it —
`gs` is in no case. So the check builds **this phase's own output** with `musl_delay()`'s
two `host_tty_set()` calls deleted and nothing else, and runs `10gs` followed by CTRL-C
1.5 s later on all three:

| binary | comes back |
| --- | --- |
| the one this phase was handed | **1.81 s** |
| this phase's output | **1.81 s** |
| this phase's output without the sleep mode | **never** — killed at 99 s |

Two numbers agreeing prove nothing if a wrong one is not caught.

### Nothing asks whether this is a terminal, which is decision 7 finally kept

`WHIM-PLAN.md` §II.1's decision 7 is *"do not ask whether stdin or stdout is a terminal.
The check goes entirely."* Phases 85 and 87 left three `isatty()` calls alive and this
takes all three: `mch_check_win`'s, `mch_get_shellsize`'s (with the whole function),
and `fill_input_buf`'s (with the arm below). `stdout_isatty` folds to TRUE and
`nv_esc`'s `out_redir` folds to the terminal arm — **both arms together**, because
folding one leaves an `if (out_redir)` with no definition. **`musl_is_terminal()` is
deliberately NOT written**: the two questions had different subjects — fd 1 for the
size, fd 0 for the input — and neither survives its caller, so a host call to answer a
question nobody asks afterwards is boundary surface for nothing.

### The core cannot acquire a descriptor at all any more

`fill_input_buf`'s `close(0); vim_ignored = dup(2);` arm reopened the editor's stdin
from its stderr when stdin hit end of file and was not a terminal. **That is the core
second-guessing the host about where input comes from**, and once the host owns the
terminal it is the host's business. `WHIM-PLAN.md` §II.4b becomes absolute: the core is
handed fds 0, 1 and 2 and that is the whole of it — it cannot open, close or duplicate
anything.

The behaviour that goes is real and probe-only. With stdin at EOF and a **terminal on
fd 2** the old binary reopens fd 0 and carries on editing (2,016 bytes of drawn
screen); this one prints `Vim: Finished.` and exits 1 (168 bytes). It fired **0 times
across all 253 recorded rows**, which is why the declaration is empty and the probe is
the evidence.

### `ui_get_shellsize()` stays a query, and this is a design the survey got wrong

The survey proposed making it *"do I know my size?"* — a `shell_size_known` flag set by
the host and by every notification. **That breaks resizing, and the measurement is
exact.** At a `Press ENTER` prompt `set_shellsize()` does

```c
    if (State == (0x2000 | MODE_NORMAL) || State == MODE_SETWSIZE)
    {
        State = MODE_SETWSIZE;
        return;
    }
```

and **discards the width and height it was given**. `wait_return()` then calls
`shell_resized()`, which is `set_shellsize(0, 0, FALSE)`, which reaches
`mustset || (ui_get_shellsize() == FAIL && height != 0)` — and learns the new size only
from `ui_get_shellsize()`'s **side effect**. With the flag, a pty resized while the
editor sits at that prompt stays 24x80 for ever; measured twice, and the in-band
notification is delivered and parsed and still lost.

So `mch_get_shellsize()`'s body moves to the host as `musl_get_winsize(int *, int *)` —
`WHIM-PLAN.md` §II.4c's own name for it — and `ui_get_shellsize()` keeps its shape. That
also disposes of the trap the survey spent an hour on: on a pipe the host's `ioctl`
fails, `ui_get_shellsize()` returns FAIL exactly as `mch_get_shellsize()` did,
`set_termname()` still emits `t_CWS`, and the recording does not move by one byte. The
flag version cost 10 bytes of stream in every one of the 102 cases.

### `vim_handle_signal` stays, and it is the one core mention that is not a message

It is the deferral machine: `ui_inchar` calls it with `-2` before a long wait and `-1`
after, and `deathtrap` consults it to *defer* a deadly signal arriving while the editor
is not reading, re-raising it later with `kill(getpid(), got_signal)`. Deleting it
would make a deadly signal act in the middle of a screen update — a behaviour change no
recording can see. So it stays, and `tools/zhostonly.py` names it as an exception with
that reason rather than loosening its pattern.

### The wait has two answers, not three

`musl_wait_for_input(long ms)` returns 1 (something to read) or 0 (the time elapsed);
`ms < 0` waits for ever. There is no "interrupted", and the reason is measured:
**nothing reads the one that exists today.** `RealWaitForChar` writes `*interrupted` in
two places, `WaitForChar` passes it through, and `inchar_loop` declares
`int interrupted = FALSE;`, passes `&interrupted` and **never reads it** — eleven
mentions, not one of them a read. The whole `efds` set exists only to produce it, and
it goes.

**The `EINTR` retry does not disappear; it moves inside the host**, and so does the
pending-flag check, which must happen on **both** sides of the `select`: only after an
`EINTR`, and a signal arriving while the editor is not inside `select` is lost until
the next keystroke; only before it, and one arriving during it is lost until it
returns. That is why the signals half's proposed `musl_input_pending()` and the
terminal half's `musl_wait_for_input()` are **one function** and the phase declares one:

```c
    static int
musl_wait_for_input(long ms)
{
    ...
    for (;;)
    {
        if (host_winch_pending || host_tstp_pending || host_int_pending) { return 1; }
        FD_ZERO(&rfds);
        FD_SET(0, &rfds);
        ret = select(1, &rfds, NULL, NULL, tvp);
        if (ret == -1 && errno == EINTR) { continue; }
        return ret > 0 && FD_ISSET(0, &rfds);
    }
}
```

`tvp` is computed once, before the loop, because Linux decrements it in place and the
`goto select_eintr` it replaces did the same.

### Two findings that are patterns, not incidents

* **A struct field whose only reader the phase deletes must go in the EDIT, not the
  sweep.** `signal_info[]`'s `deadly` was read only by `catch_signals()`.
  `tools/deadfields.py` duly removed the **member** — and left the three initialisers
  behind: `warning: excess elements in struct initializer`, three times, which
  `tools/phasecheck.sh` then fails on. A field the edit knows is dead is the edit's to
  take, **with its data**.
* **`-Wunused-but-set-variable` does not reach an address-taken or file-scope object**,
  and this phase met it twice in one edit: `did_read_something`, left set and never read
  by the reopen arm going, and `*interrupted`, write-only through three functions
  because `inchar_loop` takes its address. `CLAUDE.md` records that `deadsweep.py` does
  not act on that warning; both had to be removed by hand.

### The declared delta is nothing at all, and it is phase 85's kind

`pipes/zero.delta` gets a comment block and no line. Two full recordings either side
are **byte-identical** — 102 screen cases, `ref-excmds.txt`, `ref-argv.txt`,
`ref-pty.txt`, `ref-term.txt` — and `tools/zerodelta.sh --phase 103` finds the corpus
unmoved: 102 of 102, 111 of 111, 30 of 30.

But it is **phase 85's** kind and not phase 101's: the code runs and the instrument
*cannot see it*. On a pipe `tcgetattr`/`tcsetattr` fail and change nothing; no recorded
case sends a signal, resizes a window, types `gs`, or reaches EOF with a terminal on
fd 2. So the phase owes probes, and the check runs **fifteen** on both binaries.

**MUST DIFFER (7)**

| probe | the input | this |
| --- | --- | --- |
| `resize_inband` — `ESC[48;30;100t` then `:set columns?` | the escape lands as buffer text, 2,248 B | `columns=100`, 5,279 B |
| `trz_query` — `:set trz?` | `termresize=` | `E518: Unknown option: trz?` |
| `trz_set` — `:set trz=sigwinch` | accepted, no message | `E518` |
| `inband_stop` — `ESC[?1z` | not a command; the session ends at EOF, rc 1 | runs `:stop`, comes back, `:q!` quits, rc 0 |
| `eof_on_tty2` — stdin `/dev/null`, a pty on fd 2 | **still editing**, 2,016 B | `Vim: Finished.`, exit 1, 168 B |
| `ctrl_c_redir` — CTRL-C, stdout a pipe | `do_cmdline_cmd("qa")` → `E492` | `Type :qa and press <Enter> to exit Vim` |
| `tstp_external` — `kill -TSTP` | 4,379 B | 4,356 B — the in-band path |

**MUST NOT DIFFER (8)**

| probe | both binaries |
| --- | --- |
| `sigterm_restores` | exit 1, tty back to `ICANON=1 ECHO=1 ISIG=1 ONLCR=1 ICRNL=1`, both messages, **2,241 B** |
| `sighup_restores` | the same, **2,240 B** |
| `sigint_external` | survives and keeps editing, 2,327 B |
| `ctrl_z_key`, `stop_cmd` | 4,401 B and 4,379 B, editing afterwards |
| `raw_mode_live` | `ICANON=0 ECHO=0 ISIG=0 ONLCR=0 ICRNL=0` while editing |
| `pty_resize` | 24x80 asked, resized to 30x100, asked again: `lines=30 columns=100` |
| `stopcont` | a whole screen redrawn on CONT — 1,986 B and 2,008 B |
| `gs_interrupt` | 1.81 s, with its control at 99 s |

**A pty byte count is not an assertion, and the check says which are which.** The pty
probes drive a real terminal with real waits, so what they assert is structural — exit
status, terminal mode, what text was drawn — and the counts are reported. The two
places a count **is** the evidence are `tstp_external`, where it must differ, and
`stopcont`, where both must draw a whole screen. The pipe probes, which are
`tools/zstream.py` and deterministic, assert exactly.

**One correction to a number the survey recorded.** It reported the committed binary
drawing 2,008 bytes on `kill -STOP; kill -CONT` and a variant without the fix drawing
63. Re-measured: the committed binary draws **69 B** in one harness shape and
**1,986 B** in another, because `mch_signal()` installs with `SA_RESTART` and the
`select` simply restarts — `sigcont_handler`'s `redraw_later(UPD_CLEAR)` is deferred to
the next keystroke. Catching `SIGCONT` with the same handler as `SIGWINCH`, and
dropping the notification arm's `if (height != Rows || width != Columns)` guard so that
a notification is always a full redraw, is still right; the reason is *the screen is
correct at once instead of at the next keystroke*, not *a regression is avoided*.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,446 | **80,148 (−298)** |
| functions | 1,758 | **1,757** |
| type definitions | 907 | **904** (`tmode_T`, and the two the sweep found with it) |
| DWARF enumerators | 1,181 | **1,177** (`TMODE_COOK` `TMODE_RAW` `TMODE_SLEEP` `MCH_DELAY_SETTMODE`) |
| `nm -u` with zero's own flags | 31 | **24**, the gone set as one `comm` |
| `nm -u` as `tools/symbols.sh` counts it | 32 | **25** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| `options[]` rows | 108 | **107** (`'termresize'`) |
| `cmdnames[]` rows | 98 | 98 — **no Ex command is touched** |
| `nv_cmds[]` rows | 194 | 194 |
| binary | 805,544 | **797,192 (−8,352)** |
| sweep | | 2 rounds, 0 warnings |
| phase | | **56 s**, of which the probes are most |

### Its placement

`stage 103`, `package host` beside 100, 101 and 102, with `uses host:103 seed:83`,
`uses host:103 harness:86` and `uses host:103 terminal:85 rationale` — phase 85 removed the
two "not to a terminal" warnings, and this is where the core stops asking the kernel
about a terminal at all. The two dependencies a reader expects, on phases 101 and 102,
**cannot be written**: `tools/packages.sh --check` refuses a `uses` inside one package,
and 17–20 are one. Their reasons are in the phase program's header instead — the host
block goes inside the launcher region phase 101 created, and the deadly-signal restore
ends in phase 102's `vim_host_exit` → `__builtin_longjmp` → `return 1`.

**`need 103 swept` is not required, and it was measured.** The edit applied to the
unswept text phase 102's edit leaves gives the identical 80,447 → 80,181; every anchor
is exact text at a counted occurrence.

**`apart 102 103`, measured, and the first of its three messages is the one a reader
would not predict.** `tools/phaserun.sh whim 102-103` on q101 stops in phase 102's check
with `` `deathtrap` as a whole word has 4 mentions, expected 3 `` — a phase about
*removing* signal handling leaves one **more** mention of a handler, because the host
installs the core's rather than replacing it. The other two are ordinary: `the file
gained -280 lines, expected 18`, and `options[] is not the 108 rows phase 95 left`.
**This one is both directions**, unlike 99–100, 100–101 and 101–102: phase 103's own check
states its gone set as one `comm` against the stage's symbol snapshot, and a stage
takes one snapshot at its start — so on a 19-20 stage it is handed q101's set and the
gone set is its seven **plus `exit`**. That is `apart 97 98`'s shape, and it is
reasoning from the two programs rather than a second run, because 19's check refuses
first and there is nothing after it to observe.

### What zero-vim is after twenty phases

```
zero-vim.c        80,148 lines          from whim-vim.c's 86,614  (-6,466, 7.5%)
functions         1,757
type definitions  904
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      24 with zero's flags, 25 as tools/symbols.sh counts
binary            797,192 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 24, attributed — and two rows are now the host block's alone.**

| why | symbols |
| --- | --- |
| **the terminal**, every one of them in the host block | `read` `write` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` (7) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals**, all four in the host block | `sigaction` `sigemptyset` `kill` `getpid` (4) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

`getpid` is the odd one: it is `mch_get_pid()`'s, called once to write `b0_pid` into
block zero, and `vim_handle_signal()`'s re-raise — neither is this phase's, and `b0_pid`
is written and never read, so one line frees it whenever block zero is somebody's
phase. `__errno_location` stays too, and the phase says so rather than implying
otherwise: `errno` has exactly three mentions and does not move at all — the
`#include <errno.h>`, the `tcsetattr` retry and the `select` test — and the last two are
host-side now, so **`<errno.h>` leaves the CORE and the symbol leaves the process at the
split**.

**`WHIM-PLAN.md` §II.4c is built out.** Its three steps were `main()`, the two stream
calls, and the terminal with its signal set; 101 and 102 did the first and this does the
third, and the second is what is left — `mch_write`'s `write(1, …)` and
`musl_read_input`'s `read(0, …)`, the last two syscalls the core still makes for
itself. What remains after that is the file split, and `tools/zhostonly.py` is the
check that survives into it: when `editor.c` and `zero-vim.c` become two files, the
host block becomes the second file and the tool becomes `grep` over the first.

## Phase 104 (zero 21) — the messages are the editor's, the writing is the host's

`pipes/whim104-edit.sh` and `pipes/whim104-check.sh`, `stage 104`, `package host`.
`WHIM-PLAN.md` §II.4c's second step, and the half of it that is not the screen: *"`printf`
for the messages that appear before there is a screen, which is itself a question for
the host"*. Every byte this file has ever put on a **stream** instead of a screen now
goes through one callback the launcher installs, in exactly phase 102's shape:

```c
static void (*vim_host_message)(const char *msg, int len, int err);

    static void
host_message(const char *msg, int len, int err)
{
    ...
    int w = (int)write(err ? 2 : 1, msg + off, (size_t)(n - off));
    ...
}
```

`vim_main()` takes it as a fourth parameter and installs it beside `vim_host_exit`; the
two become `musl_` prototypes at the split, together. `<stdio.h>` goes with the symbols
— **twelve directives become eleven**, the second time a zero phase has removed one and
the same argument phase 99 made.

**Formatting stays in the core, and that is what turns twenty statements into eight call
sites.** `vim_snprintf` has been the only formatter in the file since phase 97, so the
two multi-part speakers assemble into a 1024-byte local and hand over one string, and
the other six sites are one call each with the text unchanged. Measured with a
`SOCK_SEQPACKET` socketpair as fd 2, which preserves write boundaries exactly: `-Q` was
**6 writes of 96 bytes and is 1 write of 96**; `-T no-such-term-9x` was **5 of 54 and is
1 of 54**. The same bytes, one syscall — and the latent hazard goes with them, stdout's
buffered `printf` arm arriving after everything the editor drew.

### Four of the seven symbols are named nowhere in the source

Unlike phase 103 this one really frees symbols, and the reason is the rule phase 103
stated: a symbol leaves when its last **caller** leaves the file. `printf` and `fprintf`
were being *called* here, not merely mentioned.

```
nm -u  24 → 17, gone exactly
    fflush  fputc  fputs  fwrite  printf  putchar  stderr
arrived: nothing
```

**`fputc`, `fputs`, `fwrite` and `putchar` have never appeared in `zero-vim.c` at all.**
They are what gcc emits for `printf("%s", x)` and `fprintf(stderr, "%s", x)`, and no
grep of the source could have found them. So they were **predicted** to leave with the
construct and then **verified by building** — which is the whole reason the check states
the claim as one `comm` with an *empty* `arrived` side rather than as a count. A
prediction about a symbol the source does not name has nothing but the linker to
confirm it.

**`__errno_location` is not this phase's and the check requires it PRESENT.** `errno` is
3 → 3, its two uses being `host_tty_set`'s and `musl_wait_for_input`'s `EINTR` tests,
both inside phase 103's host block. It leaves at the split, not here — said out loud,
because a reader who watches seven symbols go will look for the eighth.

**And `printf` is not at 0.** It is 13 → 10, and none of the ten is a call: nine are
`__attribute__((format(printf, …)))` and one is the string `"E767: Too many arguments
for printf()"`. `assert printf at 0` fails on a correct phase. The assertions that work
are `fprintf` 16 → 0, `stderr` 17 → 0, `fflush` 1 → 0, `printf` 13 → 10, and `nm -u`.

### The inventory is twenty statements in five functions, and the brief said twenty-one in six

The survey this phase was written from counted twenty-one output statements in six
functions, and the sixth was `nv_esc`'s `Type :qa! and press <Enter> to abandon all
changes`. **Phase 103 had already taken it**, with `stdout_isatty` and the `out_redir`
arm it sat in. So `fprintf` is sixteen here and not seventeen, `stderr` is seventeen and
not eighteen, and there are **eight call sites and not nine**. The edit counts its input
rather than trusting the survey, which is the only reason the arithmetic closed:

| function | statements | what it says |
| --- | --- | --- |
| `mainerr` | 6 `fprintf` | the version banner and the argv refusal |
| `report_term_error` | 7 `fprintf` | `'<term>' not known, defaulting to 'xterm'` |
| `set_termname` | 1 `fflush` | eleven lines *below* the call above, not inside it |
| `msg_puts_printf` | 2 `printf`, 2 `fprintf` | whatever message was being printed |
| `exit_scroll` | 1 `printf`, 1 `fprintf` | `"\n"` / `"\r\n"` on the way out |

The instrument goes on **nineteen** of the twenty, `fflush` taking no message.

### `msg_puts_printf()` and `exit_scroll`'s printf arm are deliberately KEPT

All 75 lines of the first and the else arm of the second stay, and that is a decision
rather than an oversight. **`msg_use_printf()` is not dead: it returns TRUE 23 times in
106 records** — once in each `mainerr` record, from `mch_exit` → `exit_scroll()`'s else
arm → `msg_clr_eos_force()`, where `full_screen` is FALSE and the body it guards
therefore does nothing. It is never true at `msg_puts_attr()`'s call site, so
`msg_puts_printf()` is entered **0 of 106 records** against a control that marks 100 of
102 screens. That is phase 95's kind of dead and not phase 92's: the branch *can* be
taken and never is.

Folding either would run `screen_fill()` on a screen the test has just called unusable —
`msg_clr_eos_force()` with no valid screen, or `msg_puts_display()` on a screen
`msg_use_printf()` has just said is not there. Removing them is a separate phase with a
separate question — *"the screen is always usable in this build"* — and phase 95's kind
of evidence to gather, and **it would free nothing, because the symbols are gone here**.

**PHASE 30 CORRECTED THIS, AND THE PART THAT IS WRONG IS THE PART ABOUT
`exit_scroll`.** This phase's check says the two speakers "fire in ZERO of 106
records", which is true of the **corpus** and true of the **editor** only for
`msg_puts_printf()`. `exit_scroll()`'s printf arm is **alive**, with no signal at all:
measured in phase 113's check, it moves **three of that phase's 32 stream probes**
(`t_ti_more`, `debug_more`, `term_ti_then_ti` — each `:set t_ti=X` or `-T debug`, a
paged `:set all`, exit) and **three of its four deadly-signal probes**. It is invisible
here because a recording drives one pty on which fd 1 and fd 2 are the same device and
the two bytes are the same two bytes either way — `out_char('\n')` emits `\r` first —
so the fold moves them from **fd 2 to fd 1** and nothing in this pipeline's instrument
can see that. It is therefore not merely undone but **undeclarable**, and it belongs to
whichever phase decides the core writes nothing to fd 2 at all. The other half of the
sentence survives intact: `msg_clr_eos_force()`'s test cannot be folded safely, and
phase 113 measured *why* — `screen_fill()` returns early on `ScreenLines == nullptr`, so
the fold leaves the whole 106-record recording byte-identical and two probes see the
eighteen extra bytes it emits after `Vim: Finished.`

### The bound that comes with the buffer, stated rather than declared

`mainerr`'s `str` and `report_term_error`'s `term` are both argv, so assembling into a
1024-byte buffer caps a message that used to be unbounded. That is a real behaviour
change and **no instrument in this pipeline can see it**, because nothing in the corpus
comes within 800 characters of the bound and `pipes/zero.delta` is a list of records
that moved. So it is not declared; it is pinned as probes, in both directions and in
both speakers, so it cannot drift:

| probe | the input | this |
| --- | --- | --- |
| an unknown option of **900** characters | 995 B | **995 B — byte-identical** |
| an unknown option of **930** characters | 1,025 B | **1,023 B — the turn** |
| an unknown option of **2,000** characters | 2,095 B | **exactly 1,023 B** |
| `-T` of **900** characters | 939 B | **939 B — byte-identical** |
| `-T` of **2,000** characters | 2,039 B | **exactly 1,023 B** |

1024 is `IOSIZE`, which is what every other message in this editor is built in. The
counts are **raw and not scrubbed** except at 900: the version banner's
`__DATE__`/`__TIME__` differ between two builds and their *length* does not, so only the
equality needs scrubbing and the caps do not.

### `zerodelta.sh` is the second opinion here and not the first

The declared delta is nothing at all, and two full recordings either side are
**byte-identical** — `diff -r` reports 0 lines across 102 screen cases, `ref-excmds.txt`,
`ref-argv.txt`, `ref-pty.txt` and `ref-term.txt`. The control proves that table can
fail: `write(err ? 2 : 1, …)` made `write(err ? 1 : 1, …)`, **one character**, moves 24
records and **217 lines** of `diff -r`.

**And it confirms a trap the survey named.** Run on the same control,
`tools/zerodelta.sh` names only **fourteen** of those twenty-four, because ten of them
are argv rows phases 87 and 88 already declared (`-`, `--`, `-e`, `-E`, `-e -s`, `-v`,
`f.txt`, `f.txt g.txt`, `+q! f.txt`, `-- +q!`) and `tools/zcompare.py` therefore accepts
any *further* movement in them silently. A declared row is not compared again. So this
check diffs the two recordings itself and keeps `zerodelta.sh` as the second opinion —
run on the control, where it must refuse.

**The instrumented pair is what makes the empty declaration mean something**, and it is
phase 92's shape: the input built with `write(2, "MESSAGE-OUT\n", 12)` at all nineteen
output statements and the output with the identical instrument inside `host_message()`
mark **exactly the same 24 of the 30 argv rows, by name**, and 0 of 102 screens, 0 of
`ref-excmds.txt`, 0 of `ref-pty.txt` and 0 of `ref-term.txt`. Same places, same times,
different primitive. The 24 are 23 `mainerr` and one `report_term_error` — which is also
what says the other four speakers fire in zero of 106 records.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,148 | **80,173 (+25)** |
| functions | 1,757 | **1,758** (`host_message`) |
| type definitions | 904 | 904 |
| DWARF enumerators | 1,177 | 1,177 |
| `nm -u` with zero's own flags | 24 | **17**, the gone set as one `comm` |
| `nm -u` as `tools/symbols.sh` counts it | 25 | **18** |
| external symbols | `main` | `main` |
| `#include` | 12 | **11** (`<stdio.h>`) |
| bare `write()` call sites | 1 | **2** — `mch_write`'s and `host_message`'s |
| `options[]` rows | 107 | 107 |
| `cmdnames[]` rows | 98 | 98 |
| `nv_cmds[]` rows | 194 | 194 |
| binary | 797,192 | **784,392 (−12,800)** |
| sweep | | 0 warnings |
| phase | | **28 s** |

### Its placement

`stage 104`, `package host` beside 100, 101, 102 and 103, with `uses host:104 seed:83` and
`uses host:104 harness:86` mechanical, `uses host:104 vendor:97 mechanical` — `mainerr` and
`report_term_error` assemble with `vim_snprintf`, which is the only formatter left in
the file because phase 97 put `sprintf` onto it rather than vendoring one — and
`uses host:104 includes:99 rationale`, because a phase may remove a directive at all only
since phase 99 replaced the charter's old reading of the directive count as a property
the pipeline preserves.

**`need 104 swept` is not required, and it was measured** in the same run that measured
`apart 103 104`: phase 104's edit applied to the **unswept** text phase 103's edit leaves
gives 80,181 → 80,206, every one of its anchors holding — the twelve input counts, the
twenty statements, the eleven output counts and the whole launcher tail. Its cuts are
exact text in functions no sweep touches and its computed parts are counts of words a
sweep cannot create, so there is nothing that could shrink silently.

**`apart 103 104`, measured, one direction only.** `tools/phaserun.sh whim 103-104` on q102
runs both edits and two sweeps and stops in phase 103's check on one message — *"the
output does not have exactly the twelve `#include` directives phase 99 left"*. Phase 103
states the twelve as a property it preserves and this phase takes `<stdio.h>`. There is
a second reason the run never reaches, and it is `apart 97 98`'s and `apart 102 103`'s
shape: phase 103 states its gone set as one `comm` against the **stage's** symbol
snapshot, a stage takes one snapshot at its start, so on a 20-21 stage its gone set
would be its seven plus these seven.

### What zero-vim is after twenty-one phases

```
zero-vim.c        80,173 lines          from whim-vim.c's 86,614  (-6,441, 7.4%)
functions         1,758
type definitions  904
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, every one a system header; no #define, no conditional
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            784,392 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 17, attributed — and a whole row of the table is gone.**

| why | symbols |
| --- | --- |
| **the terminal**, every one of them in the host block | `read` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` (6) |
| **the two writes** — `mch_write`'s in the core, `host_message`'s in the launcher | `write` (1) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals** — `sigaction` and `sigemptyset` in the host block, `kill` and `getpid` in both | `sigaction` `sigemptyset` `kill` `getpid` (4) |
| **gcc's own**, named nowhere in the source | `__errno_location` (1) |

**There is no row for "messages before there is a screen" any more**, and the five-strong
"gcc's own" row is down to one. `write` has a row of its own because it is the only one
here the **core** still does for itself as well as the host: `mch_write`'s
`write(1, …)`, which with `musl_read_input`'s `read(0, …)` is all of `WHIM-PLAN.md`
§II.4c's remaining step.

## Phase 105 (zero 22) — the variadic collapse

`pipes/whim105-edit.sh` and `pipes/whim105-check.sh`, `stage 105`, `package format`.
C cannot forward `...` — which is why `vsnprintf` exists beside `snprintf` — so a
function that takes `...`, opens a `va_list` and hands it to `vim_vsnprintf` cannot
survive a split unless the formatter goes with it. There are eight such functions.
**Seven are wrappers over the eighth**, and this phase expands every one of their 129
call sites into `vim_snprintf(…)` plus the tail the wrapper ran afterwards.

**`va_start` goes from eight functions to one**, and that is the whole product: no libc
symbol falls, no Ex command goes, no option goes, no message changes, and the binary
gets *bigger*. This is the half of the `va_list` decision that needs only one file, and
it exists separately for the reason `WHIM-PLAN.md` §II.4c gives — its declared delta must
be nothing at all, provable as a byte-identical recording, which is a far stronger
position from which to make 129 mechanical edits than making them while everything else
is moving.

| wrapper | mentions | protos | own def | **call sites** |
| --- | --- | --- | --- | --- |
| `smsg` | 12 | 1 | 1 | **10** |
| `smsg_attr` | 4 | 1 | 1 | **2** |
| `smsg_attr_keep` | 2 | **0** | 1 | **1** |
| `semsg` | 96 | 1 | 1 | **94** |
| `siemsg` | 12 | 1 | 1 | **10** |
| `vim_snprintf_add` | 3 | 1 | 1 | **1** |
| `vim_snprintf_safelen` | 13 | 1 | 1 | **11** |
| | | | | **129** |

`smsg_attr_keep` has no prototype and `vim_snprintf` has **two**, so a phase that
deletes "the prototype and the definition" for each of seven names fails on the first
and leaves one behind on the second.

### No message logic was written, because the tails already existed

Read each wrapper beside its non-variadic twin and the wrapper *is* the twin with a
format in front of it. `semsg`'s tail is `emsg()`, `siemsg`'s is `iemsg()`, `smsg`'s is
`msg()`, `smsg_attr`'s `msg_attr()`, `smsg_attr_keep`'s `msg_attr_keep(…, TRUE)`. All
five already existed and all five were already called from elsewhere. What is left over
is the two guards, and those become helpers: `iobuff_room()`, `emsg_iobuff_room()`,
`iobuff_or()`, `safelen_result()` and `append_room()` — **five helpers against seven
deleted definitions, which is the whole of 1,758 → 1,756.**

**The size-zero trick is what makes the expansion exactly faithful, and it was measured
rather than assumed.** `vim_vsnprintf_typval` guards every write with
`if (str_l < str_m)` and terminates with `if (str_m > 0)`, so `vim_snprintf(buf, 0, …)`
**writes nothing and does not fault** — measured with a build whose first act is
`vim_snprintf(canary, 0, …)` and `vim_snprintf(NULL, 0, …)`: all eight canary bytes
untouched, no fault on the null destination. So a helper returning 0 reproduces **both**
of the wrapper's guards — `emsg_off > 0` and `IObuff == NULL` — with **no conditional at
any site**. That is why there are five helpers and not an `if`/`else` written out 117
times.

### The sites come in three shapes, and an edit that emits two statements always gets two of them wrong

| shape | sites | what the expansion does |
| --- | --- | --- |
| a plain statement alone on its line | **92** | two lines at the same indentation |
| a **whole block on one line** inside `parse_fmt_types` | **7** | inline on the same line — two lines would put a statement in front of the closing brace |
| **value position** | **30** | a comma expression, the format call then the tail |

The 30 are the 18 `return (semsg(…), rc_did_emsg = TRUE, (void *)NULL);` comma
expressions in the regexp engine, all eleven `vim_snprintf_safelen`s — whose value is
consumed at every site, five of them `+=` — and `vim_snprintf_add`'s one. A statement is
told from an operand by the character after the closing paren. **The comma shape already
existed in the file**, as `return (iemsg(e_internal_error_in_regexp), rc_did_emsg =
TRUE, (void *)NULL);`, so the expansion invents no idiom.

The survey split them 93 / 29 / 7 and put `vim_snprintf_add`'s site in the plain column;
it is in value position, and the implementation's 92 / 30 / 7 is the count that makes
the edit correct.

### All 129 formats are non-literals, which is why the warning list is the check that matters

The thing a reader expects to be a problem is not one, and the measurement is the
opposite of the expected answer: **every one of `semsg`'s 94 formats is `_(e_name)` or
`(const char *)(_(e_name))`**, where `e_name` is a `static char e_name[] = "E123: …";`
array. Whim's constant fold turned upstream's string macros into arrays, so **there is
no string literal at a `semsg` site anywhere in the file.** It changes nothing about the
edit, which copies the format *expression* verbatim into `vim_snprintf`'s third
argument — but it means the build cannot catch a mis-expanded argument list.

What can is `-Wformat=2`. The wrappers carry `format(printf, 1, 2)` / `(2, 3)` / `(3,
4)` and `vim_snprintf` carries `format(printf, 3, 4)`, and every expansion puts the
format expression at `vim_snprintf`'s third parameter — so gcc checks exactly what it
checked before. **115 `-Wformat-nonliteral` warnings in 53 functions before, and the
identical 115 in the identical 53 after**, compared as an exact list equality and not as
two numbers. `_()` and `NGETTEXT()` are `static inline __attribute__((format_arg(1)))`,
so gcc sees through them either side.

**And the invariant fired for real on its first run**, which is the part worth keeping.
It reported `115 warnings in 0 distinct functions`: gcc quotes identifiers as `'x'`
under the phase's locale and as curly quotes under the author's, so the function-name
regex matched nothing and an empty list compared equal to an empty list. The regex
matches both quotings now, and **a zero-function list can no longer pass for an
equality** — a list comparison that can be satisfied by two empty lists is the same
mistake as a test that cannot fail.

The other thing that could have gone wrong was measured too. The expansion mentions the
format **twice** — once in `vim_snprintf`, once in the tail's `iobuff_or(F)` — and over
all 129 sites every format expression is side-effect-free: 118 are `_(e_name)`, a bare
`e_name` or a literal, 8 are `NGETTEXT(a, b, n)` (a pure inline `return`), 2 are a `? :`
over two `_()`s, and 1 is a parameter.

### `nm -u` cannot move for a restructure inside one translation unit, and the check asserts that as an equality

```
nm -u  17 → 17, THE SAME SET
gone: nothing        arrived: nothing
```

A reader meeting a 129-site phase expects a symbol to fall, and none can: a symbol
leaves when its last **caller** leaves the file, and nothing left. `vim_vsnprintf_typval`
still does every conversion in the same file, and `<stdarg.h>`'s three names are macros
and a compiler builtin type, which are no symbol at all. **The binary GROWS — 784,392 →
788,488 — and the check requires it to**, because at `-O0` 129 sites that carried one
call now carry a format call and a tail call. It is the same fact wearing its other
face, and the check reports the number rather than letting it look like a mistake.

What the phase moves is not code across a boundary but the **possibility of drawing
one**: eight functions calling `va_start` cannot be split, one can.

### Two controls move nothing, and they are kept

The recording is nearly blind to this phase, and that is measured rather than asserted.
The input source built again with `write(2, "ZW|<wrapper>|<format>\n", …)` at the entry
to each of the seven, run over the 102 screen cases: `vim_snprintf_safelen` is entered
**617** times, `smsg_attr_keep` **6**, `vim_snprintf_add` **2**, and `smsg`, `smsg_attr`,
`semsg` and `siemsg` **not once**. `semsg` is 94 of the 129 sites and the screen corpus
enters it zero times. So the phase owes probes, and the check runs **263** on both
binaries — the Ex-command errors, 132 regexp errors over both engines and three magic
settings (which are where the 18 comma-expression sites live), the report messages, the
substitute-confirm prompt, undo, CTRL-G and the ruler, and four incsearch probes with a
bad pattern, which are the only way to reach a `semsg` under `emsg_off > 0`. The same
instrument says they enter `semsg` **232 times over 34 distinct formats** and reach **67
of the 129 sites**.

**263 probes, 0 differ. Four deliberate breaks, and two of them move nothing on
purpose:**

| break | records that differ |
| --- | --- |
| both room helpers return 20 instead of `IOSIZE` | **129 of 263** |
| every `semsg` site given `msg()` for a tail instead of `emsg()` | **155 of 263** |
| `safelen_result`'s clamp reduced to `return str_l;` | **0 of 263** |
| all three guards removed | **0 of 263** |

The two zeroes are reported rather than dropped, in the program, the delta file and
here, because **they are the honest statement of what this evidence cannot reach**: the
clamp needs a message longer than 1,025 bytes out of `fileinfo`, and the guards need
`IObuff == NULL`, which is an out-of-memory failure of the first two allocations the
process makes. Reporting them as 0 is the difference between *"the probes prove the
guards are load-bearing"*, which would be false, and *"the guards are correct by
construction and the probes say so about the other two"*.

The other 62 sites are covered by the edit being **one rule applied uniformly** and by
the whole-file equalities above. `semsg`'s unreached sites are out-of-memory reports and
the twenty inside the formatter itself — which fire only on a format string the editor
would have to have got wrong, and every format in this file is one of its own — and
`siemsg`'s ten are the memfile detecting its own corruption.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,173 | **80,176 (+3)** |
| functions | 1,758 | **1,756** — seven wrappers out, five helpers in |
| type definitions | 904 | 904 |
| DWARF enumerators | 1,177 | **1,177**, and not one went, arrived or renumbered |
| `va_start` / `va_list` / `va_end` | 8 / 15 / 10 | **1 / 8 / 3** |
| `vim_snprintf` mentions | 73 | **201** — one per site less the redundant second prototype |
| `-Wformat-nonliteral` | 115 in 53 functions | **the identical 115 in the identical 53** |
| `nm -u` with zero's own flags | 17 | **17, the same set**, a `comm` empty both ways |
| `nm -u` as `tools/symbols.sh` counts it | 18 | **18** |
| external symbols | `main` | `main` |
| `#include` | 11 | 11 |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 784,392 | **788,488 (+4,096)** |
| sweep | | takes nothing, 0 warnings |
| phase | | **37 s** |

**Four functions still hold a `va_list`** — `vim_snprintf`, `vim_vsnprintf`,
`vim_vsnprintf_typval` and `skip_to_arg`, the positional-argument walker, which
`WHIM-PLAN.md` part II named as three. All four belong below the first `#include` when the
reorganisation comes.

### Its placement

`stage 105`, `package format`. **`format` is a new package and it is deliberately not
`vendor`**: nothing is brought in. A layer is *flattened* — seven wrappers over one
formatter become 129 call sites and five helpers, so that `va_start` appears once and
the formatter becomes movable. Its `uses` are `format:105 seed:83` and
`format:105 harness:86` mechanical, `format:105 vendor:97 rationale` — `musl_strlen` is
what `append_room()` measures the appended string with, and phase 97 is where the core
got its own string functions — and `format:105 host:104 mechanical`, because phase 104
routed `mainerr` and `report_term_error` through `vim_snprintf`, so the mention count
this phase's arithmetic starts from is phase 104's. **The check therefore asserts
`vim_snprintf`'s count only AFTER**, as the transformer's own arithmetic against
whatever it was handed.

**`need 105 swept` is not required, and it was measured** in the same run that measured
`apart 104 105`: this edit finds its sites by word boundary and balanced parens over the
whole file and asserts no counted anchor a sweep can move, and on phase 104's **unswept**
output it finds the same 129 sites in the same 92 / 7 / 30 shapes, 80,174 → 80,182
lines.

**`apart 104 105`, measured, and the first complaint is not the predicted one.**
`tools/phaserun.sh whim 104-105` on q103 stops in phase 104's check with **``printf` has 4
mentions, expected 10``** — phase 104's own documented counting trap read from the other
end. None of the ten is a call; nine are `format(printf, …)` attributes, and **six of
those nine sit on the wrapper prototypes this phase deletes**. The other three
complaints are ordinary: ``vim_snprintf` has 201 mentions, expected 73``, ``musl_strlen`
has 135 mentions, expected 134`` — `append_room()`'s — and *the file is 80176 lines and
the input was 80148, expected exactly 25 more*. **One direction only**, and it is not
observable in that run because 21's check refuses first.

### What zero-vim is after twenty-two phases

```
zero-vim.c        80,176 lines          from whim-vim.c's 86,614  (-6,438, 7.4%)
functions         1,756
type definitions  904
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, every one a system header; no #define, no conditional
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
va_start          1, in vim_snprintf
```

**The 17 are phase 104's 17, unchanged**, and that is this phase's claim rather than an
omission. What it produced is not a symbol, a line count or a row but a *shape*: one
`va_start` in the file, which is what the split needs and what nothing before it could
have asserted.

## Phase 106 (zero 23) — `nullptr` and `usize`

`pipes/whim106-edit.sh` and `pipes/whim106-check.sh`, `stage 106`, `package boundary`.
`WHIM-PLAN.md` §II.4c settled the design: **there is no split into two files, there is one
file with two parts, and the first `#include` is the boundary.** The core is the prefix
above it and must name nothing a header supplies. Four phases draw that line; this is
the first, and it is deliberately the smallest **because it is the one that can be
checked by `cmp`**.

Two names the core takes from a header are replaced by two the **language** supplies:

```c
    NULL    →  nullptr                            a C23 keyword; nothing is declared
    size_t  →  usize                              typedef typeof(sizeof(0)) usize;
```

Neither is a new dependency. gcc here defaults to C23 — `__STDC_VERSION__` is
`202311L` — and this file already depends on it for `enum : long`, `static_assert` and
the lowercase `bool`/`true`/`false` it uses throughout. The check states the dependency
as a measurement rather than leaving it implicit: it lifts the typedef line **out of the
output** and compiles it four ways, where gcc's default and `-std=c23` must accept it
and `-std=c11` and `-std=c99` must refuse.

**Both spellings came from the user and both beat what had been proposed.** An
enumerator with the value 0 is a null pointer constant everywhere except a variadic
argument, where it passes four bytes to a callee reading eight **with no warning from
gcc**; `nullptr` is typed, so the hazard does not exist and the rule the phase would
have had to assert for ever is not needed. And `typeof(sizeof(0))` **is** `size_t` on
any target, because `sizeof(0)` has that type by definition — proved in the same
translation unit as the real `<stddef.h>` with `_Generic((usize)0, size_t: 1, default:
0)`, which is the same *type* and not merely the same width. `typedef unsigned long
size_t;` is correct here and silently wrong elsewhere, and silent when it is right, so
nothing in this repository could have told the two apart.

The `#include`s stay at the top. Moving them is phase 110 — 109 when this was written,
before the attributes took the number 24. Eleven directives sit on the
first eleven lines and the typedef on line 13, which is the whole of **+2 lines**.

### The binary is byte-identical, and that is the whole of the evidence

`cmp` of the input's binary against the output's, both built with
`SOURCE_DATE_EPOCH=0` and the boundary's own flags: **788,488 bytes either side, no
difference at all**. That is tier 1 of `CLAUDE.md`'s verification table, and it subsumes
every screen case, every Ex-command row, every command line and every pty scenario at
once, **because the program that would be run is the same program**. `nm -u` holds still
as a `comm` empty both ways, `main` is still the only external symbol, the sweep took
nothing and canon settled in one round. `tools/zerodelta.sh --phase 106` still runs and
corroborates; it is not the evidence. It is phase 99's shape exactly, on three thousand
edits instead of seven.

### Every `size_t` was partitioned before any was renamed

`NULL` 2,555 → 3 and `size_t` 437 → 0, and the second number is a **partition and not a
count**. The edit classifies all 437 into

| class | sites |
| --- | --- |
| casts — `(size_t)` and `((size_t)` | **202** |
| declarations — parameter, local, struct field, return type | **235** |
| anything else | **0** |

and **refuses on a leftover**. A leftover would be a use a typedef does not serve — a
case label, an array bound, a `sizeof(size_t)` — and there are none. The classification
is computed from the text, so it stays true of a file this phase has never seen; the
edit asserts no *count* of its input at all, because one rule applied to every
occurrence is correct for any number of them, and pinning the count would make the phase
refuse on a tree that is merely bigger without making a wrong substitution any more
visible.

**The eleven vendored signatures change with everything else, and that is not an
interface change.** `musl_memcpy musl_memmove musl_memset musl_memcmp musl_memchr
musl_strncpy musl_strncmp musl_strncasecmp musl_bsearch musl_qsort` take `usize`
parameters and `musl_strlen` returns one. They have been the core's own `static`
definitions since phases 97 and 98 — nothing outside this file calls them — so renaming
their parameter type changes no contract with anybody.

### Three `NULL`s survive, and the control is what proves they matter

Three string literals in this file contain `NULL`:

```
"E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"
"[NULL]"          the printf layer's stand-in for a null %s argument
"NULL"            what ga_print writes for an empty growarray
```

and no literal contains `size_t`. So the substitution is not a `sed`: it scans the file
for string and character literals first — cheap and exact here, this file having no
preprocessor and no comments — and rewrites only outside them.

**The check builds the literal-unaware form as a control and requires it to differ.**
Measured: a plain line-wise `\bNULL\b` → `nullptr` gives a binary **1,598 bytes
different — 50 in `.text`, 174 in `.data` and 1,354 in `.rodata`** — and `strings` finds
`[nullptr]`, `nullptr` and an E1507 message that names a C keyword at the user. That is
`CLAUDE.md`'s rule that *what must not change is data, and the check for that is the
strings*, arriving on a phase nobody expected it on. Without the control the `cmp` above
is a pair of numbers agreeing, and a test that cannot fail is not evidence.

The survey measured the same control at **1,597 bytes and 49 in `.text`**; it ran it on
q104, and on q105 — the text this phase was actually handed — it is 1,598 and 50. The
`.rodata` and `.data` figures are the same either side, which is what says the extra
byte is code motion and not another string.

### The one-pass rule, which is worth more than the phase

**Both names must be rewritten in ONE pass over the original text**, and that is not
tidiness. A second pass indexes literal spans computed on the **first pass's output**,
and every span after the first replacement is shifted. Measured: the two-pass form
leaves **five of the 437 `size_t` behind** — and leaves a file that still **compiles**,
whose binary is still **byte-identical**, because `<stddef.h>` is still above every line
of it. Every check this phase has passes on that file except the count.

It would have surfaced at phase 110, as five unexplained errors in a move that had
nothing to do with them and nothing pointing back here. **A whole-file substitution is
literal-aware and single-pass**, and `CLAUDE.md` records it as a pattern now rather than
as this phase's incident.

### The thirty `(void *)NULL` become plain `nullptr`, and the survey said eighteen

That is a decision and not a mechanical consequence: a mechanical `\bNULL\b` → `nullptr`
leaves them as `(void *)nullptr`, which compiles and is byte-identical. The cast exists
for exactly one hazard — an untyped null constant in a variadic argument position
passing a four-byte `int` where the callee reads an eight-byte pointer — and `nullptr`
is typed, `sizeof(nullptr) == sizeof(void *)`, so the cast now says nothing a reader
needs. Doing it here rather than later is what keeps those sites from being touched
twice.

**The survey counted eighteen of them and it was simply wrong**, at q105 and at q104
alike. Re-measured, there are **thirty**: 28 comma expressions in the regexp parser,
`return (emsg(…), rc_did_emsg = TRUE, (void *)NULL);`, where the cast was carrying the
comma expression's type, and 2 returns in `get_register`. `nullptr_t` converts to any
pointer type on return, so they are the same program — which the `cmp` says. The
survey's occurrence counts were q104's as well, and phase 105 moved both.

### And one control that moves nothing, reported rather than dropped

Reverting one `usize` to `size_t` compiles cleanly and gives a byte-identical binary,
because the `#include`s are still at the **top** of the file and `size_t` is therefore
still declared above every line of it. That is the honest statement of what this phase's
evidence cannot reach: **the rename is not yet load-bearing**, and it becomes so at
phase 110, where the same control is three hard errors — measured there and exactly
three: reverting one `usize` in `musl_bsearch`'s signature gives two `unknown type name
'size_t'` and one implicit declaration at its call site. It is phase 105's b3/b4 in this
phase's shape.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,176 | **80,178 (+2)** — the typedef and its blank |
| functions | 1,756 | 1,756 |
| type definitions | 904 | **905** (`usize`) |
| DWARF enumerators | 1,177 | 1,177 |
| `NULL` | 2,555 | **3**, all three inside string literals |
| `nullptr` | 0 | **2,552** |
| `size_t` | 437 | **0** — 202 casts and 235 declarations, nothing left over |
| `usize` | 0 | **438** — the 437 and its own typedef |
| `(void *)NULL` | 30 | **0** |
| `nm -u` with zero's own flags | 17 | **17, the same set**, a `comm` empty both ways |
| `nm -u` as `tools/symbols.sh` counts it | 18 | **18** |
| external symbols | `main` | `main` |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 — `cmp`-identical** |
| sweep | | takes nothing, 0 warnings, canon settles in one round |
| phase | | **18 s** |

### Its placement

`stage 106`, `package boundary`. **The package is `boundary` and not `language`**,
although `language` is what this phase and the plain-host-call phase do: the four are one
idea — 23, the two names the language supplies instead of a header; then the two host
calls that become plain ones; then the header types and macros the core can own; then the
move itself — and `language` would have named the first and the second of those while
leaving the other two in a package that did not describe them. **They were numbered 23 to
26 when this was written and they are 106, 108, 109 and 110**: the attributes were asked for
in between and took the number 24, which is `package dialect` and not this idea at all. Its `uses` are `boundary:106 seed:83 mechanical`,
`boundary:106 vendor:97 rationale` and `boundary:106 vendor:98 rationale` for the eleven
vendored signatures above, and `boundary:106 host:104 mechanical` — this edit puts the
typedef directly below the **last** `#include` and asserts eleven directives on the
first eleven lines, and the eleventh and the count are phase 104's, which took
`<stdio.h>` with the seven symbols it freed.

**`need 106 swept` is not required, and it was measured** in the same run that measured
`apart 105 106`. This edit asserts **no count of its input**, so there is no counted anchor
that could shrink silently; what it asserts is structural — eleven directives on the
first eleven lines, `usize` and `nullptr` at zero, the three literals holding `NULL`, and
the partition — and every one of those held on the unswept text phase 105's edit leaves,
giving the same 2,552, 437 and 30. The one number that differs is the blank-line runs it
preserves, 5 on unswept text against 0 on swept, and it **preserves whatever it is
handed** rather than requiring a value.

**`apart 105 106`, measured, and the refusal is a phase that renamed nothing breaking on a
phase that renamed two type names.** `tools/phaserun.sh whim 105-106` on q104 runs both
edits and two sweeps and stops at phase 105's check's **first act** — ``iobuff_room` is
not in the output exactly once, so the controls below would not be controls`. Phase 105
writes its four controls by matching the helpers' text **verbatim**, and two of the three
hold `if (IObuff == NULL)`, which this phase spells `nullptr`. Behind that refusal sit
every other literal text it names: `safelen_result`'s clamp is `((size_t)str_l >= str_m)
? …` and all six declarations it requires are `static size_t …`. **One direction
observed**, because 22's check refuses first; what the run does show is phase 106's edit
applying unchanged to phase 105's unswept output, and its own input binary building to the
same 788,488 bytes.

### What zero-vim is after twenty-three phases

```
zero-vim.c        80,178 lines          from whim-vim.c's 86,614  (-6,436, 7.4%)
functions         1,756
type definitions  905
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
va_start          1, in vim_snprintf
NULL / size_t     3 (all in string literals) / 0
```

**Three phases in a row have declared nothing**, and each is a different kind of nothing:
21 the code runs and the instrument sees it do the same thing; 22 the code runs and the
instrument is nearly blind to it, so 263 probes stand in; 23 **the binary is the same
bytes**, which is the strongest kind this pipeline has — phase 99's, and the reason this
phase was made the smallest of the four rather than the first convenient one.

## Phase 107 (zero 24) — the attributes

`pipes/whim107-edit.sh` and `pipes/whim107-check.sh`, `stage 107`, `package dialect`.
`__attribute__` is a GNU extension, and a core on its way to another runtime was
carrying 139 of them. This phase looks at all 139, in three groups, and takes a
different decision on each:

```
    113   __attribute__((unused))         deleted
     20   __attribute__((fallthrough));   respelled  [[fallthrough]];
      6   format / format_arg             kept
```

139 → 6, **117 lines changed, not one line added or removed**, and not one statement
changed. None of the three kinds emits code — `unused` suppresses a diagnostic,
`fallthrough` gives one a hint, `format` decides what gcc will check — so the binary is
byte-identical, 788,488 bytes either side.

### Why the 113 said nothing, and why upstream needs them

Every dead-code compile in this pipeline is `-Wall -Wextra -Wno-unused-parameter`
(`tools/deadsweep.py`, `tools/phasecheck.sh`), so an unused **parameter** is not
diagnosed whatever is written on it. Upstream carries the marker because upstream
compiles this file in configurations where a parameter is used and others where it is
not; there are no configurations here, and have not been since slim's Phase 5. Measured:
with all 113 gone the sweep's own command line prints nothing at all.

### All 113 were on parameters, computed twice rather than assumed

Every one sits inside a parenthesised group whose innermost enclosing `(` is preceded by
exactly a function name, and the **97 lines** that carry them are all function
*definition* headers, each followed by a line that is `{` — so not one is on a variable,
an object, a type or a field. No parenthesised group in this file spans a line break
(`CLAUDE.md`), so that is a computation on one line and not a parse of C.

The compiler says it a second way. With `-Wunused-parameter` turned back **on**, the
output warns **135** times against the input's **43**, and every one of the 92 new
warnings is `-Wunused-parameter` at a line that carried an attribute — **not one
`-Wunused-variable`**, which is what a deletion that had reached an object would have
produced.

### Twenty-one of the 113 were false, and that is the find of the phase

113 sites, 92 warnings. **The difference is 21 attributes that marked a parameter this
build uses**:

```
    ex_cquit(exarg_T *eap)         reads eap->addr_count on its first line
    check_winopt(winopt_T *wop)    dereferences wop five times
    deathtrap(int sigarg)          compares sigarg against SIGHUP
```

There the attribute was not redundant, it was a **claim the code contradicts** — a
statement some earlier whim or zero phase made untrue, and nothing in either pipeline
checked. So the phase does not delete 113 redundant markers: it deletes 92 unnecessary
ones and **21 wrong statements**, and the check names them rather than letting the
arithmetic swallow them.

### `[[fallthrough]]` is not refused under `-std=c11`, which is an argument against the swap

The swap is one-for-one and textual: all 20 sites were standalone statements on lines of
their own, `[[` occurs **0** times in the input and **20** in the output, and
`-Wimplicit-fallthrough` is silent either side, so not one suppression was lost. It is
not a directive either — C23 attribute syntax is a *statement* in the grammar, spelled
with brackets, and the charter's rule is about preprocessor syntax.

What it costs is stated rather than glossed, and it is **weaker than phase 106's
`typeof`**, which `-std=c11` refuses outright. Measured: gcc accepts `[[fallthrough]]`
under every `-std` it has, as an extension; below C23 `-Wpedantic` says *ISO C does not
support '[[]]' attributes before C23* and **`-pedantic-errors` refuses it** — where the
GNU spelling it replaces is accepted even there, being a reserved identifier. So taken
alone **the swap narrows the dialects this file compiles under**. It costs nothing in
practice, because the file has been C23 by four other routes since before this phase,
and that too is computed rather than argued: `-std=c11` on the whole file gives the
**same 729 errors** before and after.

An honest argument against a change belongs in the phase that makes it, not in the
commit of the phase that has to undo it.

### Two of the twenty are already redundant, and are named rather than pruned

Lines 11480 and 20181, after `case ESC:` and `case Ctrl_P:`: each follows a case label
with **no statement at all**, where C falls through silently and gcc has nothing to
diagnose. They are computed from the structure and required to agree with the control's
count — two independent methods for one number — and then **kept**. What makes a
fallthrough deliberate is the author saying so, not the compiler currently asking, and a
phase that quietly drops what it noticed was unnecessary is how a real suppression goes
missing later.

### The evidence splits in two, and the phase says so

For the **133** that go or change spelling, the binary is the evidence: `cmp`-identical,
which is `CLAUDE.md`'s tier 1 and subsumes every screen case, every Ex-command row,
every command line and every pty scenario at once, because the program that would be run
is the same program.

For the **6** that stay, **the binary is blind** — measured, removing all six as well
leaves it *still* `cmp`-identical while `-Wformat=2` goes from **115**
`-Wformat-nonliteral` warnings to **zero**. So the warnings are their evidence, and the
form is phase 105's invariant reused verbatim: the identical 115 warnings in the
identical **53** functions before and after, compared as list equality, with two controls
that break it in opposite directions — `vim_snprintf`'s `format(printf, 3, 4)` removed
gives **0**, and `_()`'s `format_arg(1)` removed gives **135**, gcc having lost its way
through the translation wrapper.

That is what the six are for. Phase 105 expanded seven wrappers into 129 direct calls, so
**one attribute now type-checks 201 `vim_snprintf` mentions**, and `format_arg` on `_()`
and `NGETTEXT` is why `CLAUDE.md` records that those two were never macro-expanded. The
rule the phase applies is therefore not *remove GNU extensions* but **remove every
attribute whose job another flag already does, and keep every attribute that IS the
flag**.

### Four controls, and one of them is the one that makes `cmp` mean something

c1 and c2 are the two above. c3 blanks all 20 `[[fallthrough]];` and gets **18** warnings
where there were none — which is also how the two redundant sites are confirmed from the
other end. c4 replaces **one** of them by `break;` — the smallest change at those sites
that is a change to the *program* rather than to a diagnostic — and moves **512,594
bytes** of binary. Without c4 the byte-identical binary is two numbers agreeing.

### One fact about the tools, which is why the check is seconds and not minutes

`CLAUDE.md` records that `-fsyntax-only` does not report `-Wunused-function`. It does not
report **`-Wimplicit-fallthrough`** either, which needs the CFG — measured, the c3
control warns 18 times under `-c` and **zero** times under `-fsyntax-only`. A fallthrough
section written with it would have passed while checking nothing. It *does* report
`-Wunused-parameter` and `-Wformat-nonliteral`, which is what the other sections use.

### The trap was the whitespace, not the attribute

Each attribute was written with **two** spaces before it and **one** after, so deleting
the text alone leaves a doubled space or a space before a paren — and `tools/canon.sh`
does not fix either. The check measures it: the count of doubled spaces before a `,` or
a `)` is **639 either side**, and canon is a no-op on the output.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,178 | **80,178** — 117 lines changed, none added, none removed |
| bytes | 2,139,325 | **2,136,127** |
| `__attribute__` | 139 | **6** — three `format`, three `format_arg` |
| `__attribute__((unused))` | 113 | **0** — on 97 definition headers, 21 of them false |
| `__attribute__((fallthrough))` | 20 | **0** |
| `[[fallthrough]]` | 0 | **20** |
| `-Wunused-parameter` when asked for | 43 | **135** — 92 new, not one `-Wunused-variable` |
| `-Wformat-nonliteral` under `-Wformat=2` | 115 in 53 functions | **115 in the same 53** |
| `-std=c11` errors on the whole file | 729 | **729** |
| functions / type definitions / DWARF enumerators | 1,756 / 905 / 1,177 | 1,756 / 905 / 1,177 |
| `nm -u` with zero's own flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 — `cmp`-identical** |
| sweep | | takes nothing, canon a no-op |
| phase | | **22 s** |

### Its placement

`stage 107`, and **a package of its own, `dialect`**, which is the one placement decision
here. The boundary is where the line between core and host falls, and this phase moves
no line: it asks which dialect of C the core is written in and which of its extensions
are still being paid for. `boundary` would have made the package mean two things.
Its `uses` are `dialect:107 seed:83 mechanical`; `dialect:107 tidy:96 rationale`, because
phase 96 dropped `ui_write()`'s `console` parameter rather than leaving
`__attribute__((unused))` on it and said so in as many words — that was this phase's
judgement one phase at a time; `dialect:107 format:105 mechanical`, phase 105's 129 direct
calls being why one `format` attribute now carries 201 mentions; and
`dialect:107 boundary:106 mechanical`, the `%d` probe being built from the output's own
lines — `vim_snprintf`'s prototype and the `typedef typeof(sizeof(0)) usize;` it needs to
compile — both of which read `usize` because of phase 106.

**`apart 106 107` and `need 107` are both measured NOT to be required**, in one run:
`tools/phaserun.sh whim 106-107` on q105 runs both edits, one sweep and both checks, and
every part passes. This phase touches no `NULL`, no `size_t` and none of the helpers
phase 106's controls quote, and phase 106 neither creates nor destroys an attribute; the
edit asserts no count of its input that a sweep could move, what it asserts being a
partition and a shape.

**This phase renumbered the three that follow it, and this document had the old numbers
until now.** `WHIM-PLAN.md` §II.4c's reorganisation steps 24, 25 and 26 are phases 108, 109
and 110, which `pipes/whim.stages` states; *Phase 106 — `nullptr` and `usize`* above said
*"26, the move itself"* and has been corrected to say 27. The phase *programs* are
deliberately **not** corrected — `pipes/whim106-*.sh` still says "phase 109" where it means
the move — because every byte of a phase program is in its unit's implementation digest,
and a comment fixed there re-keys a boundary to change nothing.

### What zero-vim is after twenty-four phases

```
zero-vim.c        80,178 lines          from whim-vim.c's 86,614  (-6,436, 7.4%)
functions         1,756
type definitions  905
DWARF enumerators 1,177
GNU attributes    6, all format or format_arg
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
```

**Four phases in a row have now declared nothing**, and 106 and 107 are the same kind: the
binary is the same bytes. The difference between them is what that kind can and cannot
carry. Phase 106's whole content was inside the `cmp`; a sixth of this phase's — the six
attributes it keeps — is outside it, and the phase had to go and get a second instrument
for that part rather than let the strongest evidence it had cover a decision the evidence
cannot see.

## Phase 108 (zero 25) — the plain host calls

`pipes/whim108-edit.sh` and `pipes/whim108-check.sh`, `stage 108`, `package boundary`.
The core reached the host through two function pointers:

```c
    static void (*vim_host_exit)(int);                          phase 102's, one call site
    static void (*vim_host_message)(const char *, int, int);    phase 104's, eight
```

each a file-scope object installed through a parameter of `vim_main()` that `main()`
passed. This phase makes both of them ordinary calls to `static` functions declared
above and defined below. **Two objects, two parameters, two assignments and two
arguments go; two prototypes arrive**, and `vim_main(int argc, char **argv)` is exactly
the signature phase 101 wrote when it demoted `main()`. 80,178 → 80,173 lines.

### The indirection had one reason, and the design changed underneath it

`pipes/whim102-edit.sh` states it in as many words: *a pointer the launcher installs
through a parameter adds no external symbol, where a `musl_exit(int)` the host defines
would.* The invariant it was protecting is `nm --extern-only --defined-only` printing
exactly `main`, and under **two translation units** the sentence is true — the host's
definition of a function the core calls has external linkage by construction.

`WHIM-PLAN.md` §II.4c is now one file with two parts and the first `#include` as the
boundary, so the host's definitions sit below the core in the **same** translation unit.
A `static` forward declaration above and a `static` definition below is all a direct call
needs, and the global the indirection existed to avoid does not appear at all. The
machinery outlived its argument by six phases, which is the ordinary way a design change
leaves debris.

### The control that matters is the one that would have passed unnoticed

This is the phase that makes the core **name** the host's two functions, so
`nm --extern-only --defined-only` is the assertion it could have broken, and both halves
of the `static` trap are built on the phase's own output:

```
    static off the two PROTOTYPES                gcc refuses --
                                                 "static declaration of 'host_exit'
                                                  follows non-static declaration"
    static off the prototypes AND the            the build is SILENT and the object
    definitions                                  defines host_exit, host_message, main
```

The second is the mistake nothing else here would have caught: it compiles, it links, it
runs, every recording matches, and the core has quietly acquired two external symbols.
The check runs it every time. A third control deletes the two prototype lines and
requires the file **not** to compile, which is what makes *load-bearing* a measurement
rather than a description.

Each prototype is built out of its definition's own lines rather than retyped, so the two
cannot disagree. Measured on the output: `host_exit` declared at line 3533, called at
55872, defined at 80137; `host_message` declared at 3534, called at eight sites from
37649 to 79847, defined at 80144 — declared above every use and defined below every one,
which is the shape that keeps the declaration serving when a later phase moves the
definitions further down. It did: phase 110 moved them, and these two lines did not change.

### The evidence is the recording, because the binary moves

788,488 bytes either side and **347,279 of them differing**. An indirect call through a
pointer loads the pointer and calls a register where a direct call is relative to a known
address, and removing two file-scope objects moves what follows them; at `-O0` that is
simply different code. So this phase cannot use tier 1, does not pretend to, and falls
back on what every zero phase before 23 used: **two full recordings, `diff -r` empty
across all 106 records** — the 102 screen cases, every Ex command typed at `:`, every
command line the parser may see, the four pty scenarios and the terminal table.

And it can fail, **once for each name the phase makes direct**, which is what keeps an
empty `diff -r` from being a harness that recorded nothing. `host_exit`'s own
`host_code = r;` changed to `r + 1` moves **105 of the 106** records — every one but the
terminal table, which records no exit status. `host_message`'s `write(err ? 2 : 1, …)`
with the two streams swapped moves **`ref-argv.txt` and nothing else**, which is phase
104's own finding read back: everything reaching that function is a message printed before
there is a screen.

### The two prototypes join phase 103's nine, and that is the point of doing it here

The core → host boundary is now **one block of eleven declarations** rather than nine in
a block and two wherever an object happened to sit. `tools/zhostonly.py` — phase 103's
structural check — is run rather than assumed: its vocabulary is libc's terminal, signal
and descriptor names, `host_exit` and `host_message` are not in it, and it passes
unchanged at 60 mentions of 43 words, all inside the host block, with the same seven
named exceptions.

### It answers phase 105's open question rather than leaving it

Phase 105's survey flagged that 20 of its 129 new call sites sat inside the functions a
**split** would have moved, so host code would be calling back into the core's `emsg()`
and reading the core's `IObuff` — a genuine boundary question with three unattractive
answers. Under one file there is no question, and the measurement is here rather than in
a note somebody has to find: the four functions that still hold a `va_list` —
`vim_snprintf`, `vim_vsnprintf`, `vim_vsnprintf_typval` and `skip_to_arg` — call
**twenty distinct core functions at forty-one sites** and read `IObuff` once, and not one
of those costs a declaration. **Everything above the cut is visible below it**; only the
core → host direction ever needs a name declared, which is why this phase costs two
prototypes and not four.

### The declared delta is nothing at all, and it is phase 104's kind

The code runs, the instrument sees it, and it does the same thing. Not a `cmp` — that
kind belongs to 99, 106 and 107 — and not a blindness either: the recording is shown able
to see both names change.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,178 | **80,173 (−5)** |
| `vim_host_exit` / `vim_host_message` | 3 / 10 | **0 / 0** |
| `host_exit` / `host_message` | 2 / 2 | **3 / 10** |
| `exit_fn` / `message_fn` | 2 / 2 | **0 / 0** |
| `vim_main`'s signature | four parameters | `(int argc, char **argv)` — phase 101's, back again |
| core → host prototypes | 9 | **11**, one block |
| functions / type definitions / DWARF enumerators | 1,756 / 905 / 1,177 | 1,756 / 905 / 1,177 |
| `nm -u` with zero's own flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | **`main`**, with both halves of the trap built |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 bytes, 347,279 of them differing** |
| records that moved | | **0 of 106** |
| sweep | | takes nothing, one round, canon a no-op |
| phase | | **25 s** |

### Its placement

`stage 108`, `package boundary` with 106, 109 and 110 — it writes no C23 and removes no GNU
extension, so `dialect` would be wrong; what it changes is how the core names what is on
the other side of the line. Its `uses` are `boundary:108 seed:83 mechanical`;
`boundary:108 harness:86 mechanical`, its evidence being a recording and not a `cmp`;
`boundary:108 host:101 rationale`, the signature it gives back being the one phase 101
wrote; `boundary:108 host:102 mechanical`, for the pointer, the parameter, the assignment
and the reason they were a pointer — and for the launcher it must leave untouched, `exit`
staying out of `nm -u` because `host_exit()` still records a status and jumps; and
`boundary:108 host:103 mechanical` and `boundary:108 host:104 mechanical` for the block the
two prototypes join and the eight call sites and the control that reads back phase 104's
finding.

**`apart 107 108` is measured, and its first complaint is one nobody would predict.**
`tools/phaserun.sh whim 107-108` on q106 stops inside **phase 107's** check with *"a line
carrying a kept attribute is not the line it was, byte for byte: lines 847 852 3081
3092"* — phase 107 records the six `format`/`format_arg` lines it keeps **by line
number**, and this phase deletes the `vim_host_message` object with its blank line at
495, so everything below moves up by two. A check that pins a line number is a dependency
on every line above it, exactly as a check that quotes C is a dependency on spelling
(`apart 105 106`). One direction only, measured: phase 108's check passes on the tree that
stage leaves.

**`need 108 swept` is measured NOT to be required**: this edit applies unchanged to the
unswept text phase 107's edit leaves, giving the same 80,178 → 80,173 and the same line
numbers.

### What zero-vim is after twenty-five phases

```
zero-vim.c        80,173 lines          from whim-vim.c's 86,614  (-6,441, 7.4%)
functions         1,756
type definitions  905
DWARF enumerators 1,177
core -> host      11 prototypes in one block; no function pointer left between them
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
```

**The product was not in this phase's branch.** `zero-vim.c` landed in a commit of its
own after the merge, as phase 109's did, where phases 107 and 110 carried theirs in the
branch. Nothing was lost — the file is q108's boundary either way, and `make whim-verify`
reproduces it — but a merge whose diff holds the programs and not the thing they produce
is easy to read as a phase that changed no source, and it is worth knowing that two of
these four look like that in `git log`.

## Phase 109 (zero 26) — the header types and macros the core can own

`pipes/whim109-edit.sh` and `pipes/whim109-check.sh`, `stage 109`, `package boundary`.
Eight things the core took from a header stop coming from one:

```
    time_t          →  typedef long time_T;  plus  long time(long *tp);
    sig_atomic_t    →  volatile int                       2 in the core; the host's 3 stay
    uintptr_t       →  usize                              1 site
    struct timeval  →  a TAGLESS struct of two longs, and musl_gettimeofday(long *, long *)
    MIN / MAX       →  the text the preprocessor gives, at 19 lines
    offsetof        →  __builtin_offsetof                 9 sites
```

and the nine libc functions the core still calls — `malloc realloc free time getpid kill
write labs abs` — get plain prototypes, none of them `static`. 80,173 → 80,197 lines,
exactly the 24 the edit adds.

### It comes BEFORE the move, and the ordering is the whole of its evidence

Every declaration here replaces something a header **still supplies from above it**, so
the ordinary build cross-checks each one for free. After the move there is nothing left
to check against, and the check that can be written then is only *it compiles*.

**Sixteen `static_assert`s**, all silent, compare every core-owned spelling against the
header type it replaces:

```c
    sizeof(elapsed_T) == sizeof(struct timeval)              and both offsets, both widths
    _Generic((usize)0,  uintptr_t: 1, default: 0)            and again against size_t
    _Generic((time_T)0, time_t:    1, default: 0)
    _Generic((int)0,    sig_atomic_t: 1, default: 0)
    _Generic(time, long (*)(long *): 1, default: 0)
    __builtin_offsetof(T, m) == offsetof(T, m)               at all six types the 9 sites use
```

Every one of them **names a header type**, so not one can be written once phase 110 moves
the includes below. They are a control in the check and never in the product, and a
seventeenth with one comparison deliberately wrong must fail — so they are compiled and
not merely present.

**Four deliberately wrong declarations give four diagnostics**, which is the same
argument from the other side: `int time(int *tp);`, `void *malloc(int n);` and
`long getpid(void);` are each `conflicting types`, and `static void *malloc(usize n);` —
the trap the survey names — is `error: static declaration of 'malloc' follows non-static
declaration`. After the move the first three become **nothing at all** and the fourth
changes shape entirely. One more argument for doing this first, and phase 110 measured
where the fourth went.

### Six of the seven changes are tier 1, and the check says so as an equality

Six of them are a rename or an expansion the preprocessor was already performing, so
none can generate a different instruction. That is stated rather than claimed: **with the
clock ALONE reverted, the binary is `cmp`-identical** to the 788,488-byte one the phase
was handed. So `time_T`, `volatile int`, `usize`, the 23 `MIN`/`MAX` expansions,
`__builtin_offsetof` and the nine prototypes are `CLAUDE.md`'s tier 1 — literally the
same program — and only the clock has anything to answer for.

**The seventh is the clock, and it is the only libc *type* the core could not rename
away.** `struct timeval` is a **layout**, so `elapsed_T` becomes the core's own tagless
`struct { long tv_sec; long tv_usec; }` and the five `gettimeofday(&X, nullptr)` calls go
through a new `musl_gettimeofday(long *, long *)` in the host block. That is a call and
two stores where there was a syscall wrapper, so the binary moves, and **two full
recordings, `diff -r` empty across all 106 records**, are what answers for it.

The recording is not blind to it: `musl_gettimeofday` writing its two fields the wrong
way round moves **six of the 102 screen cases** — `key_Q`, `key_gQ`, `key_gf`, `macro_q`,
`reg_percent` and `ruler_move`.

### `MIN` and `MAX` are read from the header, not written into the program

The edit sends `MIN(ZZA,ZZB)` and `MAX(ZZA,ZZB)` through the preprocessor with
`<sys/param.h>` included and turns what comes back — `(((ZZA)<(ZZB))?(ZZA):(ZZB))` —
into its template. That is the only honest meaning of *the exact text the header gives*,
and the check re-derives the same two shapes and requires 7 more `MIN` expansions and 16
more `MAX` ones, with the repeated argument really repeated. **Two of the 19 lines pass a
call as an argument and so evaluate it twice** — exactly as the macro did, which is what
the `cmp` proves and what writing `<` by hand would have quietly fixed into a different
program.

### Two corrections to the brief, both measured, and both make the rule stronger

**A core-defined `struct timeval` tag is NOT a hard error.** The brief says it is
`error: redefinition`. Measured: **C23 permits a struct to be redeclared with the same
members**, so gcc 15's default dialect is *silent* both ways round, and only `-std=c11`
and `-std=c17` refuse it. The tagless struct is still mandatory, for a better reason than
a diagnostic — **the core must not define a libc tag at all**, and the layout equality
has to be **asserted**, which is what three of the sixteen `static_assert`s do. A rule
that rests on a diagnostic the standard has since removed is a rule with a shelf life.

**There is no `musl_time`.** The brief has the core calling its own
`long musl_time(long *)`. `time` keeps its name, so that `long time(long *tp);` sits
above `<time.h>`'s own declaration of the same function and gcc compares the two, where
a wrapper would have cast any mismatch away at its own boundary.

**AND WHAT THAT PROTOTYPE PINNED IS NOT WHAT THIS SECTION SAID IT WAS.** It read
*"precisely what pins `time_T`'s width"*, and so does this phase's commit; **phase 115
measured it and both are wrong**. The prototype pinned `long == time_t` — real, and
phase 115's `m1` compile re-ran this phase's own control to confirm it, `int time(int
*tp);` giving `conflicting types for 'time'`. Nothing in it ever mentioned `time_T`, and
phase 115's `m2` is the proof: **this phase's output with `typedef long time_T;` changed
to `int` and the prototype left untouched compiles in SILENCE.** What checked
`time_T == time_t` here was `_Generic((time_T)0, time_t: 1, default: 0)`, one of the
sixteen `static_assert`s above — a **control in the check**, never in the product —
which is exactly why phase 115 had to
put `static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");`
into `zero-vim.c` when it took the prototype away.

### The tool changed, and that is the tool working

`tools/zhostonly.py` refused, as predicted — its two `struct timeval` exceptions read
*"the clock's, and not this phase's … somebody else's later phase"*, and this is that
phase; `kill` acquires one for the core's own prototype. But a swapped exception list was
not enough. The tool runs in the checks of phases **20, 104, 108 and 109, each on its own
output**, and the core's vocabulary shrinks between them — so an exception's count is now
the **tuple of values it takes**, each written with the phase that made it true. What it
refuses is still a count nobody has written down, so *an exception that stops being true
is a fact this tool is meant to notice* holds, and the phase that ends one comes here and
says so. Measured with `tools/implhash.sh`: 107 whim and slim keys identical either side,
six zero keys move — the units and edits whose programs name the tool.

### A control can be right and still be a bad check

The must-differ control — `musl_gettimeofday` writing its two fields the wrong way round
— **stalls `tools/zpty.py`**. An editor whose clock runs backwards has timeouts that
never expire, so the harness waits out its deadline, writes `stalled` and exits 1 two
minutes later, which is correct behaviour on a deliberately broken binary and a flaky
check. The control is `zcases.py` alone for that reason. **A check should not depend on
how long a harness takes to give up.**

### The declared delta is nothing at all, and it is a sixth kind

Not code that could not run (92, 100), not code the instrument cannot see (12), not a
possibility removed (13), not a `cmp` of the binary (99, 106, 107), and not phase 108's *the
code runs and the instrument sees it do the same thing* either. It is **six of seven
changes that are a `cmp` and one that is a recording**, and the phase separates them
rather than taking the weaker evidence for all of it.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,173 | **80,197 (+24)** |
| `time_t` / `time_T` | 6 / 5 | **0 / 10** |
| `sig_atomic_t` | 5 | **3**, all three the host block's |
| `uintptr_t` | 1 | **0** |
| `offsetof` / `__builtin_offsetof` | 9 / 0 | **0 / 9** |
| `MIN(` + `MAX(` | 7 + 16, on 19 lines | **0**, expanded in place |
| `struct timeval` | 6 — 4 core, 2 host | **3**, all host |
| `gettimeofday` | 5 | **1**, inside `musl_gettimeofday` |
| libc prototypes in the core | 0 | **9**, none `static` |
| functions | 1,756 | **1,757** (`musl_gettimeofday`) |
| type definitions / DWARF enumerators | 905 / 1,177 | 905 / 1,177 |
| `nm -u` with zero's own flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488** — and `cmp`-identical with the clock alone reverted |
| records that moved | | **0 of 106**; the wrong-way-round clock moves 6 of 102 cases |
| sweep | | takes nothing, canon a no-op |
| phase | | **27 s** |

### Its placement

`stage 109`, `package boundary` beside 106, 108 and 110. Its `uses` are
`boundary:109 seed:83 mechanical` and `boundary:109 harness:86 mechanical` for the recording;
`boundary:109 host:103 mechanical`, because `musl_gettimeofday` is **defined inside the
host block phase 103 created**, immediately above `musl_delay` — `tools/zhostonly.py`
reads the host region as the lines from `host_winch_pending` to `musl_suspend`'s last
brace, so a definition below `musl_suspend` would put a `struct timeval` outside it and
the tool would refuse; and `boundary:109 host:102 rationale`, because the two
`sig_atomic_t` this phase respells are the core's and the three it leaves alone are the
host block's — the split that makes that sentence meaningful is the launcher phases 101
and 102 put at the bottom of the file.

**`apart 108 109` is measured, not predicted.** `tools/phaserun.sh whim 108-109` on q107 runs
both edits, one sweep and then **phase 108's** check, which stops at *"the file is 80197
lines and the input was 80178 (80178 recorded) — expected exactly five fewer"*: this
phase adds twenty-four lines to the text before that check reads it. **`need 109 swept` is
measured NOT to be required**, in the same run — both edits came from the edit cache,
which is keyed on the digest of the tree each is handed, and a direct `cmp` confirms it:
phase 108's edit applied to q107 gives a `zero-vim.c` byte-identical to q108's, so the sweep
between them is a complete no-op.

### What zero-vim is after twenty-six phases

```
zero-vim.c        80,197 lines          from whim-vim.c's 86,614  (-6,417, 7.4%)
functions         1,757
type definitions  905
DWARF enumerators 1,177
header names left above the host block   the twelve *_MAX, PATH_MAX, EXIT_FAILURE,
                                         SIGHUP and SIGTERM -- and nothing else
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
```

**Recorded as a follow-up rather than done here.** Every core use of the clock is *stamp
now, then ask how many milliseconds have passed*: `elapsed()` is one `gettimeofday` and a
subtraction, and its four callers all compare the result against a millisecond count. So
the core never needs the **layout**, only a scalar, and a `long musl_now_ms(void)` would
take `struct timeval`, `gettimeofday` and the tagless-struct question out of the core
together. The tagless struct is an interim shape, kept because it is what was surveyed
and what the evidence above was measured against; the scalar clock is a phase of its own,
if it is asked for.

**Its product landed separately too**, like phase 108's: the branch and the merge hold the
programs, and `zero-vim.c` came in the commit after.

## Phase 110 (zero 27) — the move: the first `#include` becomes the boundary

`pipes/whim110-edit.sh` and `pipes/whim110-check.sh`, `stage 110`, `package boundary`.
This is what the pipeline had been clearing the ground for. **The eleven `#include`s
move from the first eleven lines to line 78,360, and above them there is not one
preprocessor directive.** `zero-vim.c` is now one translation unit with a core editor on
top, written in plain C with no directives at all, and a host below that begins with the
includes. **The first `#include` IS the boundary**, marked by nothing else — no comment,
no banner, no name — and `make editor.c` writes the **78,358 lines** above it.

That target existed before this phase, writing an empty file on purpose, so that the
phase that fills it would change the source and not the makefile. It did.

### The order was forced, and it is why 109 and 110 are two phases

```c
    enum : int { INT_MAX = (int)(~0u >> 1) };       placed AFTER <limits.h> is
    enum : int { 0x7fffffff = (int)(~0u >> 1) };    a syntax error
```

So the derived constants can only be written **once the includes have moved**, and phase
109's sixteen `static_assert`s against the headers can only be written **while they are
still above**. Two phases, in that order, and neither could have held the other's work.
The check proves the first half on the product rather than arguing it: with `<limits.h>`
put back at line 1 the build stops on that line with *expected identifier before numeric
constant*.

### The constants are asked for, not remembered

The edit performs the move, compiles the cut alone, and **collects every name gcc says is
undeclared: 23 errors naming exactly twelve**. A thirteenth would mean phase 109 did not
finish; one this program declared and gcc did not ask for would be a declaration nobody
needs. Eight are derived from the type system and four asserted:

```
    INT_MAX 14   INT_MIN 2   LONG_MAX 51   LONG_MIN 1        derived: (int)(~0u >> 1) and kin
    LLONG_MAX 3  LLONG_MIN 1 ULLONG_MAX 10 SIZE_MAX 1
    PATH_MAX 12  EXIT_FAILURE 1  SIGHUP 2  SIGTERM 2         asserted against the header below
```

(the counts are the core's mentions at q109). **All twelve are enumerators**, including
the four asserted ones, because `PATH_MAX` is an **array bound** — and a `static const
int` cannot appear in an array bound, a case label or an enumerator initialiser
(`CLAUDE.md`, *Add a constant*).

Re-measured from the product: delete the twenty enumerator lines from `editor.c` and gcc
gives the same **23 errors over exactly those twelve names**, `INT_MAX` eight times,
`PATH_MAX` five and the other ten once each.

### Below the includes, `INT_MAX` IS the macro, so each assert restates the derivation

This is the design point the brief could not have foreseen, and it is what the move costs.
Phase 109 could compare every core-owned spelling against the header still above it. Here
the headers are **below**, and below them the name `INT_MAX` is `<limits.h>`'s macro — so

```c
    static_assert(INT_MAX == INT_MAX, "INT_MAX");        a tautology about the header
    static_assert((int)(~0u >> 1) == INT_MAX, "INT_MAX");  what the phase writes
```

The twelve asserts the phase puts **into the product** compare the **deriving
expression** against the header, and the left-hand side is not typed twice: it is emitted
from the same table as the enumerator's own initialiser, and the check reads both back
out of the source and requires them equal as text. Measured both ways — the wrong
derivation in both places is `static assertion failed`, and **the same wrong enumerator
with the assert written the naive way builds in silence**.

### What moves below is computed to a fixpoint, not listed

The four functions that hold a `va_list` are named, because `va_list` is `<stdarg.h>`'s
and the core cannot declare it: `vim_snprintf`, `vim_vsnprintf`, `vim_vsnprintf_typval`
and `skip_to_arg` — the fourth being the one `WHIM-PLAN.md` §II.4c missed and phase 105
counted. **Everything else follows from compiling the cut**: move what gcc calls unused,
compile again, repeat.

The brief's single round is only the first of **five**. The fixpoint takes **15
functions, 18 objects and 3 enum blocks** — the formatter's whole private island, down to
`musl_strchr`, `format_typeof` and the eleven `typename_*` strings — where one round
takes seven things. The stopping rule is `vim_main` and `deathtrap`, the two the **host**
calls, which are unused above the cut by construction and stay there. Enum blocks are
found by **counting**, not by compiling: no warning gcc has can see a dead enumerator
(`CLAUDE.md`).

That is **78,358 lines** in the cut where one round gives 79,079, and it is the right
answer because **the cut is the deliverable**: shipping 15 dead functions inside it would
be a defect, and *nothing above the boundary is dead* is now a checkable sentence.
Measured on the product, the moved island is lines 78,385–79,952 — **1,568 lines** — and
1,874 lines sit below the cut in all, of which the 280-line host block was already at the
bottom and did not move.

### There is no `cmp` to be had, so tier 1 moved up a level, onto the source

Every address below the first moved definition moves with it, so the binary cannot be the
evidence and the phase does not pretend otherwise. What replaces it is `CLAUDE.md`'s tier
1 **one level up**, stated and checked as a **multiset**:

> not one of the input's 80,197 lines is missing from the output, and the only lines the
> output adds are the **32** this phase writes — twenty enumerator lines and twelve
> `static_assert`s — plus three blanks where an emptied paragraph left two.

**A phase that moved code and altered a character of it on the way could not say that.**
Re-measured independently by sorting both files: 0 lines missing, 35 added, and the 35
are exactly the 32 and three blanks. 80,197 → 80,232 lines.

The recording answers for the thirty-two: two full recordings, of the binary the phase
was handed and of its own, **byte-identical across all 106 records**. `nm -u` is the same
17 names in both directions — **moving a definition inside ONE translation unit frees
nothing and needs nothing**, because a symbol leaves when its last caller leaves the
*file* — and `nm --extern-only --defined-only` is still exactly `main`. The claim of this
phase is structural and not a symbol count.

### The cut's own check is four parts, and three of them are silent in an ordinary build

`awk '/^ *# *include / { exit }'` — one clause, no judgement — gives the prefix, and then:

```
    0 lines beginning with #                 a #define above the cut gives 1
    a floor of 70,000 lines                  an #include back at line 1 gives a cut of 0
    0 errors under -fsyntax-only
    a warning set EQUAL to the declared boundary
```

The fourth is the interesting one. The cut's warnings **are** the core → host interface:
**thirteen names, every one `used but never defined`** — `vim_snprintf`, `host_exit`,
`host_message` and the ten `musl_*` — computed a second way from the text, as the names
defined below the cut and mentioned above it, and required to match. Verified here
independently: 0 errors, 13 warnings, and the cut an exact byte prefix of `zero-vim.c`.

Each of the three mistakes is built both ways in the check, and **each is silent in the
ordinary build**: a `#define` above the cut, an `#include` back at line 1, and one core
function — `elapsed` — quietly moved below the boundary, which changes the thirteen by
exactly its name. Nothing else in this pipeline can see any of them.

### Two corrections, and one of them was in the makefile

**The brief's `static` trap is wrong.** It says a `static` libc prototype above the
boundary makes the link fail. It does not: gcc gives `<stdlib.h>`'s own declaration
internal linkage too, warns on **that** line — *'malloc' declared 'static' but never
defined* — links against libc regardless, and produces a binary `cmp`-identical to the
product's. So the trap phase 109 caught with a hard error is now a warning, and what
stands between the core and it is the sweep's rule that the build print **nothing**, plus
this check's assertion that none of the nine prototypes is `static`.

**And `whim.mk`'s `editor.c` guard was wrong, in a way it could only be once the cut was
not empty.** It refused if the cut held a `#` of **any** kind — but `#` is an ordinary
character, and the editor is full of it:

```c
    enum { CPO_HASH = '#' };
    if (ptr[0] == '#')
    "E1281: Atom '\%%#=%c' must be at the start of the pattern"
    the two latin1 case tables
```

Measured on the first cut this rule ever produced, **63 lines** hold one and none is a
directive — so the guard would have refused every valid cut for ever. It is `^ *#` now,
which is what the rule's own paragraph already said it meant, and it is 0 on the cut and
11 on the whole file. The agent was told not to touch that file and changed it anyway,
with the measurement and a flag saying so, which was the right call. **The phase's merge
commit says 54 lines and the tree says 63**; 63 is the number `whim.mk` and the phase
commit carry, and the number reproduced here.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,197 | **80,232 (+35)** — 32 written, 3 blanks |
| input lines missing from the output | | **0**, as a multiset |
| first `#include` | line 1 | **line 78,360** |
| directives above the first `#include` | 11 | **0** |
| `make editor.c` | an empty file | **78,358 lines**, 0 directives, 0 errors, 13 warnings |
| moved below | | 4 va_list functions + 15 functions, 18 objects, 3 enum blocks, 5 rounds |
| the island's extent | | lines 78,385–79,952, **1,568 lines** |
| constants written above | 0 | **12 enumerators**, 8 derived and 4 asserted |
| `static_assert` in the product | 1 | **13** — the `cmdnames[]` one and this phase's twelve |
| functions | 1,757 | 1,757 |
| type definitions | 905 | **909** |
| DWARF enumerators | 1,177 | **1,189 (+12)** |
| `nm -u` with zero's own flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 bytes, and not the same bytes** |
| records that moved | | **0 of 106** |
| sweep | | takes nothing, canon a no-op |
| phase | | **64 s**, the longest any zero phase has taken |

### Its placement

`stage 110`, `package boundary`, the last of the four. Its `uses` are
`boundary:110 seed:83 mechanical` and `boundary:110 harness:86 mechanical`, the evidence
being a recording; `boundary:110 format:105 mechanical`, because what moves below is **four**
functions and not eleven only because phase 105 collapsed the seven `va_list` wrappers into
their call sites; `boundary:110 host:103 mechanical`, the includes landing immediately above
`host_winch_pending`, the first line of the block phase 103 created and where
`tools/zhostonly.py` starts reading; and `boundary:110 host:104 mechanical`, the eleven
directives on the first eleven lines being what phase 104 left, which this edit asserts
before lifting the block.

`tools/zhostonly.py` gained two named exceptions here and nothing else:
`<file scope>` now says `SIGHUP` and `SIGTERM` twice each — the core's own
`enum { SIGHUP = 1 };` and the `static_assert` below the includes that checks it — both
outside the host region because that region begins at `host_winch_pending` and the
includes are above it. Measured: the tool passes on the phase's input and on its output,
and the edit moves **four zero unit keys (103, 104, 108, 109**, the phases whose checks name
it**) and no whim, whim edit or slim key at all**.

**`apart 109 110` is measured, not predicted**, by running phase 109's check on the tree
phase 110 leaves: it stops with *"the output is 80232 lines and the input was 80173, a
difference of 59 where 24 was expected"* and *"the output does not have exactly eleven
directives on its first eleven lines"* — and behind those, its sixteen `static_assert`s
and four mismatched declarations **cannot compile at all**, because every one of them
needs the headers above the core. One direction only: phase 110's own check passes on that
stage. **There is no `need 110`**, also measured — the edit applied to phase 109's unswept
output gives a `zero-vim.c` `cmp`-identical to the swept path's, and it asserts no count
of its input that a sweep could move.

### What zero-vim is after twenty-seven phases

```
zero-vim.c        80,232 lines          from whim-vim.c's 86,614  (-6,382, 7.4%)
                  78,358 above the boundary, 1,874 below it
functions         1,757
type definitions  909
DWARF enumerators 1,189
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, at line 78,360, and NOT ONE DIRECTIVE above them
core -> host      13 names: vim_snprintf, host_exit, host_message, ten musl_*
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
make editor.c     78,358 lines: 0 directives, 0 errors, 13 warnings, all of them
                  `used but never defined` and all of them the interface
```

**`WHIM-PLAN.md` §II.4c is built out.** Its design was one file with two parts and the first
`#include` as the boundary, and the four phases that draw it are done: 23 replaced the two
names a header supplied with two the language does, 108 made the two host calls plain, 26
gave the core its own types and macros, and 27 moved the includes. **Seven phases in a row
have declared nothing** — 104 through 110 — and among them are five distinct kinds of
evidence: the code runs and the instrument sees it (104, 108, 109's clock), the instrument is
nearly blind and 263 probes stand in (22), the binary is the same bytes (106, 107), six of
seven changes are a `cmp` and one is a recording (26), and the source is the same lines
rearranged (27). The last is new, and it is the one a pipeline needs the day it starts
moving code rather than deleting it.

## Phase 111 (zero 28) — the scalar clock

`pipes/whim111-edit.sh` and `pipes/whim111-check.sh`, `stage 111`, `package boundary`.
The core's whole use of time is *stamp now, then ask how many milliseconds have
passed*. That is four places — `do_sleep`'s `done < msec` loop, `vim_beep`'s 500 ms
rate limit, `handle_osc`'s `>= p_ost` timeout and `inchar_loop`'s deadline — and **not
one of them reads a field, prints a reading or compares two stamps**. So the core never
needed the *layout* of a clock, only a scalar, and this phase gives it one:

```c
    long musl_now_ms(void)          replaces    void musl_gettimeofday(long *, long *)
    X = musl_now_ms();                          a stamp
    musl_now_ms() - X                           a reading
```

Three things phase 109 created go together and are at **0** afterwards: `elapsed_T`, its
tagless mirror of `struct timeval`, whose layout that phase had to `static_assert`
equal; `elapsed()`, whose whole body was one clock read and one subtraction; and
`musl_gettimeofday`, **whose out-parameter pair existed only because a struct could not
cross the boundary**. 80,232 → 80,222 lines.

### What it earns, and the check is careful not to claim more

Every one of the thirteen core → host signatures takes **scalars and byte buffers
only** — `void`, `int`, `long`, `usize`, `char *` and `int *`, with `vim_snprintf`'s
`...` held to printf arguments by `format(printf, 3, 4)` on a `-Wall -Wextra` clean
build. **That was already true at q110**, phase 109 having chosen `long *, long *`
precisely so that `struct timeval` would not cross, and the check computes the property
on the *input* as well as the output for exactly that reason. What is new is that the
**workaround** is gone: no host call's shape is decided any more by a type the core
cannot name.

`nm -u` is the same 17 names and **`gettimeofday` is still one of them**, stated as an
equality because a reader expects a clock phase to free a clock symbol. It cannot: the
host still calls it to implement `musl_now_ms`, and a symbol leaves when its last
*caller* leaves the **file**, which is the split and not this phase.

### Two decisions, both measured

**The origin is the whole second of the first call, not 1970.** Every core use is a
difference, so the origin is free — and on a target where `long` is 32 bits `tv_sec *
1000` is signed overflow on the **first** call and every call after it, measured with
`-fsanitize=signed-integer-overflow` as *`1789797927 * 1000 cannot be represented in
type 'int'`*, where `(tv_sec - base) * 1000` is exact for 2^31 ms, **24.86 days** of
uptime. The base is taken **lazily** rather than in `musl_host_init()`, because an
ordering dependency between two host functions is what a host rewrite breaks silently.
The origin being a whole second is what makes it behaviourally invisible, and the
`epoch` variant's probes and full recording are the product's.

**Precision is not lost and the rounding point moves.** `elapsed()` subtracted and
*then* divided; `musl_now_ms` divides at each reading and the caller subtracts.
Microseconds were already discarded either way — but the two roundings are not the same
function, and the check compiles a probe and runs it over **20,000,000 random pairs**:
the difference is exactly ±1 ms and never more, 24.95 % one lower, 50.08 % equal,
24.97 % one higher, with neither formula closer to the truth. The control is the `ceil`
variant, which rounds every reading **up** — twice the perturbation this change can
cause — and whose probes and full recording are also the product's.

### The declared delta is nothing at all, and it is phase 85's kind

The code runs and the instrument cannot see it, **measured rather than inferred**: of
the 102 screen cases, 95 ring the bell once and 7 not at all, and **not one rings it
twice**, so `vim_beep`'s 500 ms limit — the only clock reading a screen case can reach
— is never asked to suppress anything. Three controls say it from the other side: a
clock that never advances, one that runs backwards and one that runs 1000× fast each
move **0 of the 102**.

So the phase owes probes, and they are built on **`gs`** — `nv_g_cmd`'s `s` arm is
`do_sleep(count * 1000)`, the one call site a keystroke file can drive and the only way
real time passes inside the editor. `1gs` is 1,009 ms on the binary the phase was
handed and 1,008 on its own, `2gs` 2,005 and 2,004, and `hgshh` rings **2** bells on
both — `vim_beep`'s threshold in both directions in one probe. Each half fails on a
control aimed at it: with the clock 1000× fast `2gs` returns after one wait; with a
clock that never advances `1gs` **never returns**; with `vim_beep`'s 500 written
500000 `hgshh` rings 1 bell and with it written −1 it rings 3.

**The sleep assertion was wrong once and the fix is the interesting part** (commit
`6ef24b7`, after the phase landed). It asked for `2gs - 1gs >= 900`, and **both numbers
are wall-clock times taken from outside, around whole editor runs**, so each carries its
own startup jitter: phase 113's verify caught it on a loaded machine with `1gs` inflated
to 1,133 ms against `2gs` at 2,006, and a correct phase failed. Widening the threshold
would move the boundary rather than remove it. **A lower bound on a sleep cannot flake
in that direction** — a sleep takes at least as long as it asks for and load can only
make it longer — so the assertion is `2gs >= 1900`, and the `fast` control still breaks
it at 1,008 ms.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 80,232 | **80,222 (−10)** |
| `elapsed_T` / `elapsed()` / `musl_gettimeofday` | 3 things | **0** |
| `make editor.c` | 78,358 lines | **78,342**, 0 directives, 13 boundary names |
| the boundary | 13 names | **13**, `musl_gettimeofday` → `musl_now_ms`, asserted as a *set* |
| `nm -u` | 17 | **17, the same set**, `gettimeofday` among them |
| external symbols | `main` | `main` |
| binary | 788,488 | **788,488 bytes, and not the same bytes** |
| records that moved | | **0 of 106**, on four recordings — input, output, `ceil`, `epoch` |

### Its placement

`stage 111`, `package boundary` — **not `host`**, and the phase argues it: `boundary` is
not only where the line falls but what the core may **name** at it, which is what 106, 108
and 109 each did, and this is 26's clock item finished; `host` would be wrong more
plainly, since 100 to 104 move code into the launcher and this phase moves none. Four
`uses`: `seed:83` and `harness:86` for the recording — where the check says outright that
the recording is the **weakest** part of the evidence — `host:103`, because `musl_now_ms`
is defined inside the block that phase created and `tools/zhostonly.py` reads the host
region from `host_winch_pending`, and `host:101`, because the probes rest on the editor
being an ordinary program a keystroke file can drive to exit.

`apart 110 111` is measured by running phase 110's check on the tree this phase leaves: it
stops at its first act with *"`elapsed` is not defined exactly once above the boundary,
so the control that moves one core function below it would not be a control"* — **phase
110's boundary argument rests on moving one core function below the cut, and the function
it picked is the one this phase deletes**. One direction only. `need 111 swept` is
measured *not* to be required: run on the unswept tree from phase 110's edit cache, this
edit gives a byte-identical `zero-vim.c` and the sweep after it is a complete no-op.

`tools/zhostonly.py` gains `gettimeofday` as a host word, which this phase is what makes
permanently true, with the five core call sites phase 109 moved named as exceptions at
the counts they had at q103, q104 and q108. Measured: exactly ten keys move — zero units
and edits 103, 104, 108, 109 and 110 — and **not one slim or whim key of the 107**.

## Phase 112 (zero 29) — the case tables become one, and it is the union

`pipes/whim112-edit.sh` and `pipes/whim112-check.sh`, `stage 112`, `package casemap`.
`zero-vim.c` carried **two complete Unicode simple-case maps** and they did the same
job: vim's own `toUpper[]`/`toLower[]`, there since whim, and musl's, which phase 98
added as `musl_toUpper[]`/`musl_toLower[]` range-compressed into the same
`convertStruct` shape so that `towupper` and `towlower` could leave `nm -u`. Which one
the editor consults is decided by `'casemap'`. **A core with no C library has nothing
to choose between**, so this phase makes it one table — and the table is the **union**.

### The survey said the two "differ on 2 of 5 probes", and five characters cannot see 193 codepoints

Expanded over the whole of `0..0x10FFFF`, from the file and again from this machine's
libc through `ctypes`, the two disagree at **97 upper and 96 lower** codepoints, at
none of which both map to different characters, and the split is lopsided:

* **vim maps and musl does not, 96 and 96**: all of Vithkuqi, all of Garay, the enclosed
  Latin letters `U+24B6..U+24CF` and `U+24D0..U+24E9`, Glagolitic `U+2C2F`/`U+2C5F`, the
  recent Latin Extended-D additions, `U+019B`, `U+0264`, `U+1C89`, `U+1C8A`. **vim's
  table is simply newer** — it knows Unicode 14's Vithkuqi and Unicode 16's Garay, and
  musl's `casemap.h` predates both.
* **musl maps and vim does not, exactly one**: `U+00DF → U+1E9E`, the sharp s.

So *use vim's* loses the sharp s and *use musl's* loses ninety-six. **Each table knew
something the other did not, and the union is the only answer that keeps both.** It is
**computed, not written down**: the edit expands both tables, refuses on a codepoint
they map differently, requires that no existing row covers one it is about to insert —
`utf_convert()` binary-searches on `rangeEnd`, so a row inside another row is
unreachable — inserts `{0xdf,0xdf,-1,7615}` at its sorted place, and re-expands and
requires the result to be exactly the union. It also parses and re-emits all four
tables **before** changing anything and refuses unless the re-emission is byte-identical
to the text it came from, so the row it writes is in `tools/canon.sh`'s shape by
construction.

### The one row is a deliberate divergence from Unicode, taken knowingly

Unicode's **simple** uppercase of `U+00DF` is `U+00DF`; `U+1E9E` is musl's tailoring,
and putting it into vim's own table changes the **default** `'casemap'`. What it buys is
that the file stops contradicting itself: `swapchar()` has hard-coded `ß → ẞ` for `gU`,
`g~` and `~` all along, so before this phase the table and the keystroke gave different
answers for the same character.

### The delta runs on both arms, and that is what a reader gets wrong

On the **non-internal** arm — `:set casemap=` or `casemap=keepascii`, which read musl's
table and now read the union — 96 upper and 96 lower codepoints **gain a mapping they
never had** and `ß` **keeps** the one it had. On the **default** arm the single row
arrives. Six probe sessions move and six must not, and the six that must not are the
ninety-six proving they did not regress on the arm that always had them, the sharp s
keeping what it had on the arm that always had it, `g~g~` on `ß`, and `:set isk=@` then
`dw` on `café naïve`.

**Two traps the probes had to get right, both measured.** `gU`, `g~` and `~` **cannot
show the sharp s at all**, `swapchar()` hard-coding the mapping before it consults any
table — so the row is reachable only through `\u`/`\U` in a substitution, which goes
`do_upper` → `vim_toupper` → `utf_toupper` and hits the table directly. And the chartab
that the 892 startup calls of `towupper`/`towlower` build **does not move**, although
those calls run with `cmp_flags` still 0 and therefore take the non-internal arm: the
union equals musl's table at every one of `128..255`, the two having disagreed below
`U+0100` at `U+00DF` alone.

### The declared delta is nothing at all, and that is the harness and not the phase

The corpus cannot see any of this — all 102 screen cases seed themselves by typing
ASCII and none touches `'casemap'`, the Ex sweep reads the message a command prints, the
argv records are command lines, the pty scenarios are the window size and raw mode. Two
full recordings are byte-identical in all 106 records, so `pipes/zero.delta` gains no
line. **That is phase 85's situation — a blind harness rather than a static phase — and a
phase in it owes probes of its own.** Two controls, each computed from the two sources
rather than spelled out: `vimonly` is the output with musl's contribution taken back
out, and the default-arm probe then records exactly what the **input** recorded;
`vimless` is the output with `toUpper[]` replaced by the input's `musl_toUpper[]` — the
merge done the careless way round — and the circled letter goes, which is the regression
no record could report.

**The check's strongest assertion is not a row count.** The produced tables are expanded
over all 1,114,112 codepoints and required to be exactly the union in three directions,
with the musl half **re-derived from libc** rather than from the bytes the phase
deleted, and a perturbed row proving the comparison can fail. Beside it is a rule rather
than a number: what the phase changes on the default arm, and what it stops mapping, are
both **computed** from the two input tables, and every member of both must appear in the
probe text.

### Measured

| | input | after |
| --- | --- | --- |
| `toUpper[]` | 198 rows, 1,477 codepoints | **199 rows, 1,478** |
| `toLower[]` | 183 rows, 1,460 codepoints | **183, 1,460** — musl's lower table added nothing |
| `musl_toUpper[]` / `musl_toLower[]` | present | **gone** |
| lines | 80,222 | **79,857 (−365)** — 358 sixteen-byte rows out and one in |
| `make editor.c` | 78,342 | **77,977**, the same −365 |
| binary | 788,488 | **782,760 (−5,728)** |
| `nm -u` | 17 | **17, the same set** — changing *data* frees no symbol and needs none |
| DWARF enumerators | 1,189 | **1,189**, none gone, arrived or renumbered |
| `options[]` / `cmdnames[]` | 107 / 98 | 107 / 98 |
| records that moved | | **0 of 106**, against twelve probes that carry the phase |

### Its placement

A package of one, `casemap`, deliberately **not** `vendor`: `vendor` is *nothing is
brought in*, and this phase brings nothing in and frees no symbol — what it decides is
what the core's case map **is**, which is phase 95's argument and whim's Phase 18's
applied to data instead of to an option row. Three `uses`: `seed:83` and `harness:86`,
and `vendor:98`, because without phase 98 there is one case table already and no union
to take.

`apart 111 112` is measured with `tools/phaserun.sh whim 111-112` on q110: phase 111 states
its arithmetic as a line count of **the core** and stops at *"the core is 77978 lines
and was 78359, a difference of −381 where −16 was expected"* — its own −16 less this
phase's 365. One direction only, measured too: phase 112's check was then run on the tree
that stage leaves and every part of it passed. **No `need 112 swept`**, measured in the
same run.

**Two things about the pipeline this phase ran into, recorded and not acted on.** `make
whim-verify` cannot run while the phase list has a gap — `tools/verifypass.sh` takes the
previous boundary as `r$((first - 1))`, so a reserved-but-unlanded number makes it die
on a missing tar. And `make whim-tip` in a fresh worktree re-runs every phase, because
`git worktree add` gives `whim-vim.c` a new mtime and `$(ZEROBUILD)/input.sha256`
depends on it.

## Phase 113 (zero 30) — the message fold: `msg_puts_printf()` and the branch that reaches it

`pipes/whim113-edit.sh` and `pipes/whim113-check.sh`, `stage 113`, `package host`.
`msg_puts_attr_len()` ends in a two-armed test: the true arm handed the message to
`msg_puts_printf()`, 75 lines that reach the terminal **without a screen**, and the
false arm draws it. The true arm is never taken, and this phase folds it to two lines
that say the same thing to the host:

```c
    host_message((char *)str, maxlen, !info_message);
    msg_didout = TRUE;
```

`msg_puts_printf()`, its prototype, and `vim_strlen_maxlen()` and its prototype — which
the sweep finds, that function's only call being inside it — go with it. **Two
functions, not one**: 1,756 definitions → 1,754, and 79,857 → 79,766 lines, the edit
adding one and the sweep taking 92.

### Which kind of dead, and it is not phase 92's

Phase 92 removed code that **could not run**. This removes code that **can** run and
never does, which is phase 95's kind, and the difference decides what evidence is owed.
`msg_use_printf()` is a live predicate: instrumented on this phase's own output it
answers TRUE **23 times**, every one at `msg_clr_eos_force()`, every one in
`ref-argv.txt`, one per `mainerr` row — with `full_screen` FALSE in all 23, so the body
it guards is a no-op. The phase therefore leaves the predicate at six mentions and
claims only that **one of its four call sites is dead**. The evidence is phase 95's
shape: the input source built twice with the identical `write(2, "PP-ENTERED\n", 11)`,
first in `msg_puts_printf()` — **0 of 106 records** — and then in `msg_puts_display()` —
**103 of 106, 5,749 occurrences**.

### Why the message is kept rather than dropped

Deleting the arm's body outright is five lines smaller and records identically. It was
rejected: **a phase about removing dead *code* must not quietly remove a
*capability*.** `host_message(msg, len, err)` takes `len < 0` as `strlen` and `len >= 0`
as an exact count, which **is** `msg_puts_printf`'s own `maxlen` contract, measured by
reading both. The arm is never executed, so equivalence is not claimed: what the two
lines do not reproduce is the CR-before-NL insertion and the `msg_col` bookkeeping, and
no recording or probe in this pipeline can reach either.

### That the recording did not move is not the check, and this is where that matters most

**The two folds this phase declines also record byte-identically, and one of them is
wrong.** So the check is 36 probes and an instrumented pair, and it **builds the
rejected folds and requires each to move a named probe**:

* **`msg_clr_eos_force()`'s test cannot be folded safely.** Phase 104 said folding it
  "would run `screen_fill()` with no valid screen". That is right, and the number behind
  it is the interesting part: `screen_fill()` returns early on `ScreenLines == nullptr`,
  and `ScreenLines` **is** null in all 23 `mainerr` cases, which are the only 23 places
  the predicate is TRUE in a recording — **so the fold leaves the whole 106-record
  recording byte-identical and a phase checked only against the corpus would ship it**.
  Two probes see it: `t_ti_stopterm` 2,266 → 2,280 bytes and `hup_clean` 2,124 → 2,142,
  the extra eighteen being `\x1b[24;63H\x1b[K\x1b[24;1H` **after** `Vim: Finished.` —
  the editor erasing the last line of a screen it has just declared unusable, on its way
  out. Guarding with `msg_check_screen()` instead is **not** a cheaper spelling of the
  same thing: it drops the `swapping_screen() && !termcap_active` disjunct, which is
  exactly what `t_ti_stopterm` reaches.
* **`exit_scroll()`'s printf arm is ALIVE, and phase 104 was wrong to name it a follow-up
  beside `msg_puts_printf()`.** `pipes/whim104-check.sh` says the two "fire in ZERO of
  106 records"; that is true of the **corpus** and true of the editor only for the
  first. With **no signal at all** the arm fires in **three of this phase's 32 stream
  probes** — `t_ti_more`, `debug_more`, `term_ti_then_ti` — and in **three of its four
  deadly-signal probes**. Folding it to `out_char('\n')` is not a crash risk:
  `out_char('\n')` emits `\r` first, so the bytes on the wire are the same two. It moves
  them **from fd 2 to fd 1**, and on a pty where both descriptors are the same device
  the combined stream is byte-identical — which is why `tools/zpty.py` could never see
  it and why folding it here would be **undeclarable**. It belongs to whichever phase
  decides the core writes nothing to fd 2 at all. The check builds that fold too and
  requires it to move exactly those three stream probes and those three signal probes,
  so *"this phase did not disturb it"* is measured rather than asserted — and that is
  what the four deadly-signal probes are for, and why they run **with fd 2 on a pipe of
  its own**.

### A counting trap that cost a first attempt at the anchor

`    if (msg_use_printf())` at four spaces is a **substring** of the same line at eight,
so `str.count()` says 3 where `grep -c '^    if (msg_use_printf())$'` says 2 — the third
match being `exit_scroll`'s. And there are **four** call sites, not three: the fourth is
written `if (!msg_use_printf())` in `hit_return_msg()`, and an edit that greps for the
positive spelling misses it. The anchor is the four-line block, whose count is 1, and
all three untouched sites are asserted verbatim before and after.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,857 | **79,766** — the edit adds 1, the sweep takes 92 |
| function definitions | 1,756 | **1,754** |
| `make editor.c` | 77,977 | **77,886**, 0 directives, 0 errors, the boundary unchanged |
| `nm -u` | 17 | **17, the same set** |
| binary | 782,760 | **782,760** |
| records that moved | | **0 of 106**, with 32 stream probes and 4 signal probes identical |

### Its placement

`stage 113`, `package host 17 18 19 20 21 30`, because this is phase 104's own follow-up
and not a tidy-up. Three `uses`: `seed:83` and `harness:86` for the recording, and
`boundary:108`, because `host_message()` is a name the `editor.c` cut enumerates only
since phase 108 turned the function pointer into a declaration. There is deliberately
**no `uses host:113 host:104`** — `uses` records a dependency *across* packages and
`tools/packages.sh --check` refuses one inside a package — so that relation is written
as a comment on the `package host` line instead.

`apart 104 113`, measured rather than assumed: phase 104's check pins thirteen names, of
which **seven are already broken by phases 105–112**, four are untouched here, and exactly
**two** move at this phase — `msg_puts_printf` 3 → 0 and `info_message` 9 → 7.
`msg_use_printf` stays at 6, which is the other half of phase 104's assertion and
survives intact. **No `need 113`**: the edit's one anchor is a four-line block of exact
text whose count is 1, and no count a sweep can move.

**Phases 111 and 112 landed while this one was being written**, and the independence was
measured rather than assumed: every counted anchor has the same value on q110, on q111 and
on q112. One thing did move — phase 111 renames `musl_gettimeofday` to `musl_now_ms`, and
that name is one of the boundary names the `editor.c` cut prints. This check never
writes that set out: it computes it from the input and from the output and requires the
two to be equal, so the rename cost it nothing. **That is the whole argument for
counting a set as a rule rather than as a table of constants.**

## Phase 114 (zero 31) — `abs` and `labs`, the two the core took on trust

`pipes/whim114-edit.sh` and `pipes/whim114-check.sh`, `stage 114`, `package vendor`.
**The core is optimised for transpilation, not for performance, and so it may not depend
on latent compiler behaviour** (`WHIM-PLAN.md` §II.4c, the user's rule). This is the first
application of it, and by every number this pipeline usually reports it does nothing:
`nm -u` is the same 17 names either side, as a `comm` empty in both directions.

**That is the phase.** `abs` and `labs` were **called** by the core, at three sites, and
were in the undefined set **zero times** — measured here, the input's whole assembly
(`gcc -S -O0`) mentions neither name, because gcc lowers both to inline arithmetic.
Nothing in the language promises that. A compiler that emitted the calls the source
literally asks for would have added two libc symbols to a file whose whole claim is the
shortness of that list, **and nothing in the pipeline would have said so until it
happened**. So the phase frees nothing and says so as an equality; what it removes is a
dependence on behaviour nothing states.

Two prototypes leave the core's libc declaration block, which phase 109 wrote and which
goes **9 entries to 7** — found by its *shape*, a contiguous run of top-level
declarations above the first `static`, and never by line number. The three call sites
become `musl_abs` and `musl_labs`, by the literal-aware single pass `CLAUDE.md` asks
for. And two definitions land in the `musl_` block phases 97 and 98 built, immediately
above `musl_bsearch`, so the four `<stdlib.h>` functions the core owns — `musl_atoi`,
`musl_atol`, `musl_abs`, `musl_labs` — sit together and above every use.

### musl's spelling is copied and not improved, and the undefined behaviour with it

`/root/musl/src/stdlib/abs.c` and `labs.c` are one line each, `a>0 ? a : -a`, and the
check proves that choosing it **costs nothing** rather than arguing it: `a > 0 ? a : -a`
and `a < 0 ? -a : a`, both taken out of the output, compile to byte-identical machine
code at `-O0` and at `-O2`, and agree at all 4,294,967,296 `int` values and at
20,000,006 `long` ones including `LONG_MIN` and `LONG_MAX`.

`-a` overflows at `INT_MIN` and at `LONG_MIN`, so both vendored functions are undefined
there — **and so are libc's, by the same expression, and so is musl's own source**. The
pair is *faithful rather than safer*: a phase that quietly made the core's arithmetic
differ from the libc it replaces would be a behaviour change wearing a vendoring phase's
clothes. What is measured instead is whether the three sites can be driven there, and
they cannot — `last_status_rec`'s two operands are window heights, which
`limit_screen_size()` clamps at 1,000 rows, and the two `labs` arguments are differences
of line numbers, so `LONG_MIN` needs a buffer of 2^63 lines. Instrumented, the largest
magnitude any of the three is ever handed over 51 probe calls is **22**.

### What the image may do is a rule and not a coincidence

There is no `cmp` to be had — at `-O0` a call to a static function is a call and inline
arithmetic is not — so what is asserted is that the difference is **accounted for
instruction by instruction**: the object's `.text` grows by exactly **42 bytes**, which
is `musl_abs` (19) plus `musl_labs` (24) plus what the three callers gained or lost
(−2, +1, 0) **and nothing else**, and the linked image is 782,760 bytes either side with
604,650 of them different, which is what putting a definition near the front of a file
does.

**Only `.text` and `.eh_frame` change size** — no data section moves a byte, which is
the *this phase changes code, not data* claim — and neither changes by more than one
alignment unit. Which of the two happens is a property of the **input**, measured both
ways for the identical edit: 0 on the q112 tree and 64 here. Written as *"every section
but `.eh_frame` keeps its size and its address"* — true on q112 — the check **refused
this rebase**, naming `.text` and `.fini`, and that is the assertion working and the
phase being fine.

### The corpus cannot see this phase at all

Measured rather than assumed: the output built with a probe on each of the three
arguments enters **none** of them in 106 records, and its recording is byte-identical to
the product's. So the phase owes probes, and runs three, one per site — `+set rnu` with
sixty lines (46 calls, arguments −21 to 22), sixty long wrapped lines then CTRL-F CTRL-F
CTRL-B CTRL-B (3 calls), and `:set laststatus=2` then `=0` (2 calls). **Two of the three
are proven able to fail**, by a control whose `musl_abs` and `musl_labs` return their
argument unchanged. **`stl` does not, and the check reports it rather than hiding it**:
its site is reached twice and its answer guards only `w_prev_height = w_height`, which
`win_new_height()` already assigns on every path that changes a height. That site is
proven **reached** and not proven **observable**, and the equivalence above is its
evidence.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,766 | **79,776 (+10)**, all of it core |
| `make editor.c` | 77,886 | **77,896** |
| the libc prototype block | 9 entries | **7** |
| `abs` / `labs` in `nm -u` | 0 | **0** — they were never there, and that is the point |
| `nm -u` | 17 | **17, the same set** |
| object `.text` | | **+42 bytes**, accounted for instruction by instruction |
| binary | 782,760 | **782,760**, 604,650 bytes different |
| records that moved | | **0 of 106**, against three probes the corpus cannot reach |

### Its placement

`stage 114`, `package vendor 14 15 31`. Four `uses`: `seed:83` and `harness:86`, the corpus
reaching none of the three call sites so a probe here is a screen recorded from a
keystroke file; `boundary:109`, the two prototypes it deletes being that phase's; and
`boundary:110`, the check asserting that both definitions land **above** the first
`#include`, which is the boundary only because of the move.

**Both schedule declarations are measured, in one run.** `tools/phaserun.sh whim 113-114`
on q112 runs both edits, one sweep and both checks and stops in phase 113's — *"the output
is 79776 lines and the input was 79857, a difference of 81 where 91 was expected"* —
because the ten lines this edit adds land in the same swept text. That is `apart 100 101`'s
shape exactly, and it is one direction only. The same run measures that **`need 114
swept` is not required**, this edit applying unchanged to phase 113's unswept output with
all five anchors holding.

## Phase 115 (zero 32) — the clock crosses the boundary

`pipes/whim115-edit.sh` and `pipes/whim115-check.sh`, `stage 115`, `package host`.
The core read **two** clocks and only one of them had crossed. Phase 111 gave the
elapsed-milliseconds clock to the host as `long musl_now_ms(void)`; the wall clock
stayed behind as `static time_T vim_time(void) { return time(nullptr); }`, with five
call sites, `long time(long *tp);` in the core's own libc prototype block, and **two
more reads that bypassed the wrapper altogether** inside `ui_focus_change()`. Two steps,
in order: those two become `vim_time()`, and the wrapper then moves below the first
`#include` as `host_time()`, declared in the core's host block beside `host_exit` and
`host_message`.

Afterwards **the core does not name `time` at all** — four mentions in the input's core
to none, counted on the **literal-stripped** text because two `NGETTEXT` strings in
`op_shift()` say the English word and a count that read those would be counting English.
`host_time` is 8 above the boundary (the declaration and seven call sites, the input's
five plus the two that bypassed the wrapper) and 1 below. The libc prototype block goes
**7 entries to 6**, losing `time` and nothing else — `malloc realloc free getpid kill
write` — phase 114 having vendored `abs` and `labs` out of it immediately before. The
block is found by its **shape**, a run of non-blank lines around a line already required
to be unique, so phase 114 landing under this phase cost it no edit at all. The file is
**79,776 lines either side**: the core loses 7 and the host gains exactly 7.

### The prototype never pinned `time_T`, and that corrects something written down twice

Phase 109's commit and the brief for this one both say that `typedef long time_T;` is
correct because `long time(long *tp);` sits above `<time.h>`'s declaration of the same
function, where gcc compares the two. **Half of that is true and the important half is
not**, and the `m2` compile is what says so: the input with `time_T` changed to `int`
and the prototype **left alone** compiles in **silence**. The prototype pinned
`long == time_t`; nothing ever checked `time_T == long`. So this phase does not preserve
a guarantee, it **replaces a weaker one with a stronger one**:

```c
    static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");
```

beside the twelve constants phase 110 put below the includes, **which is the only place
in the file where a core name and a header name are both in scope**. Four compiles, all
in the check. `m1`: the input with the prototype written `int time(int *tp);` is
`conflicting types for 'time'` — that *was* the guarantee, and it is a real one. `m2`,
as above: silent. `p1`: the output with the assert deleted and `time_T` perturbed
compiles in silence too — **the regression this phase would otherwise have shipped, and
the reason the prototype could not simply be deleted**. `p2` and `p3`: with the assert
present, `int` and `long long` both give `static assertion failed: "time_T is time_t"`.
The one line names `time_T` itself, which the prototype could not.

### `host_time()` returns `long` and not `time_T`, which is a decision

Its definition is below the boundary and `time_T` is a core typedef above it, so the
host half could not name it once the file is cut at the first `#include`.
`musl_now_ms()` returns `long` for that reason and this is its sibling — the two halves
of the clock now cross in the same shape, and the boundary's stated property that every
core → host signature takes scalars and byte buffers only survives a **fourteenth**
name. Nothing is converted at any call site, `time_T` being `long` on the page, and the
check asserts the typedef line itself.

### The recording is the weakest part of the evidence, and the check says so

Two full recordings are byte-identical across all 106 records — but **not one of the 102
screen cases reaches `ui_focus_change()`**, which is the only function whose reads this
phase respells in place. So the phase owes an instrument, and it has two.

An **instrumented pair**: `write(2, "TICK\n", 5)` at every clock read on each side —
three sites on the input (the wrapper, and `ui_focus_change`'s two, as comma expressions
so that the tick is exactly where the read is), one on the output, because afterwards
there is only one — with the two instrumented 102-case recordings required to be
byte-identical. They are, and the instrument is not silent: it marks **100 of 102 cases
with 410 reads** in all, the two it misses being `ctrl_c_clean` and `ctrl_c_changed`,
which exit before a key is looked up.

And **focus probes**, because a keystroke file *can* reach `ui_focus_change()`: `\033[I`
and `\033[O` are `KE_FOCUSGAINED` and `KE_FOCUSLOST`, and `set_termname()` registers
both unconditionally — **`WHIM-PLAN.md` §II.2l's hazard, that a typed Escape followed by
`[` is read as a key code, used deliberately**. `\033[O \033[I` reads the clock 3 times
and `\033[O \033[I \033[O \033[I` reads it 4, identically on both binaries; the
arithmetic is 0 + 2 + 0 + 1 at the four calls plus one for the `:q!`, `focus_state`
starting MAYBE so that the first FocusLost reads nothing and the first FocusGained finds
`last_time` at 0 and takes both reads. **The two reads are still two reads**, in the
same two statements and the same order, so they straddle a second neither more nor less
often than before — and the control is that question made into a program: `hoist` is the
output with the two reads collapsed into one local, and it gives 5 on the second probe
where the product gives 4. `focus` does **not** separate them (3 either way, by a
different route), which is why there are two probes and the check says which one is
load-bearing.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,776 | **79,776** — the core loses 7 and the host gains 7 |
| `time` named in the core | 4 | **0**, on the literal-stripped text |
| the libc prototype block | 7 entries | **6** |
| `make editor.c` | 77,896 | **77,889**, 0 directives, 0 errors |
| the boundary | 13 names | **14**, `host_time` arriving and nothing gone |
| `nm -u` | 17 | **17, the same set** — `time` does not leave, and that is said as an equality |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, and not the same bytes** |
| records that moved | | **0 of 106**, with an instrumented pair and two focus probes behind it |

### A hazard this phase found in the shared recording, recorded and deliberately not worked around

Comparing two **full** recordings is load-sensitive in exactly three records, and the
clock phase is the one that would notice. `tools/zrec.py` scrubs the undo message's
elapsed time to `<ago>` **padded to the width it replaces**, so the *screen* is
protected — but the record also carries `--- stream <len> sha=<…>`, taken over the
**raw** byte stream, where `0 seconds ago` and `1 second ago` are 13 bytes and 12.
Measured with a control built for it — `add_time()` reporting one second more — exactly
three records move, `undo_after_ins`, `undo_block` and `undo_redo`, which are exactly
the three whose screen carries `<ago>`, and in each exactly **one line** moves, the
`--- stream` line, with all 24 screen lines byte-identical. One 33-way concurrent `make
whim-verify` failed here on `undo_after_ins` alone. **It is not this phase's to fix** —
hashing the scrubbed stream in `tools/zrec.py` would re-key all 33
phases and require `.reference/zero-baselines` to be recorded again — **and not this
phase's to paper over either**, a private exclusion being a check narrowed to fit what
it saw. The comparison stays an exact `diff -rq` and the hazard is written into the
check's header. What matters for this phase is that the exposure is **unchanged** by it,
which is what the instrumented pair measures.

**AND HASHING THE SCRUBBED STREAM WOULD NOT CLOSE IT, which phase 123 measured
afterwards and which this paragraph got wrong.** The scrub rewrites the age's TEXT,
padded; the leak is *arithmetic on that text's width*. An undo reports its age and the
editor then positions the cursor to clear the line, so `0 seconds ago` emits
`\033[24;40H\033[K` and `1 second ago` emits `\033[24;39H` — a column derived from a
scrubbed string's length, in a byte sequence the scrub never touches. It failed zero
phase 99, whose binary is byte-identical either side, which is the only reason it was
catchable at all. The fix that does work is phase 123's: record `stream N redraws`, a
count of `\x1b[?25h`, instead of a digest, and let a clock control carry the evidence —
both readable clocks replaced by runaway counters move 0 of 16 memline records against
9 of 102 screen cases, which says the record does not depend on the clock AT ALL, and a
digest never could. `tools/zmemline.py` does this; `tools/zcases.py` still digests the
raw stream, and the change is expensive rather than hard: the `--- stream` line is named
in eighty files, forty-seven times in phase 95's check alone.

### Its placement

`stage 115`, `package host 17 18 19 20 21 30 32`, because this is what that package is:
a thing the core did for itself becomes a thing it asks the host to do, declared in the
one host block and defined below the boundary — `host_exit` (19), `host_message` (21),
`host_time`. It is deliberately **not** `boundary`, which *draws* the line (106, 108, 109,
110, 111); this phase moves one function across a line already drawn. Five `uses`:
`seed:83` and `harness:86`; `boundary:109` for the prototype and the typedef it replaces;
`boundary:110` for the only place the `static_assert` can be written; and `boundary:111`
for `musl_now_ms`, whose shape and whose `nm -u` sentence this phase takes.

**Four `apart` lines.** Three are one fact measured three ways, phase 113's method of
applying each check's own assertion directly to the tree this phase leaves: 109 and 110
both carry the nine-entry `PROTOS` list and now find **three** of the nine at 0 — `labs`
and `abs`, already phase 114's, and `time`, which is this phase's — and 110 and 111 both
**write out** the boundary as thirteen names where this phase makes it fourteen, the
symmetric difference being exactly `{host_time}`. The fourth was measured with
`tools/phaserun.sh whim 114-115` on q113: `apart 114 115`, one direction only, where phase
114's check stops on three messages — the block losing `time` as well as `labs`/`abs`,
the core at a difference of 3 where 10 was expected, and *"the host changed size, and
this phase does not touch it"*. **No `need 115`**, measured in the same run.

## Phase 116 (zero 33) — the terminal table is asked with `+set term=`, not `$TERM`

`pipes/whim116.sh`, one whole program, `stage 116`, `package harness`. The second phase
in the pipeline that changes **no source at all** — phase 86 is the other — and it is
there for the same reason: the pipeline was about to measure itself with a question
that could not see the answer.

**The nineteen rows of `.reference/zero-baselines/ref-term.txt` were content-free, and
had been since phase 83.** Every one of them read

```
TERM='vt100'              -> term=xterm-256color t_Co=256
```

because whim phase 19 removed the `getenv("TERM")` from `termcapinit()` — *the terminal
is what the build says* — and left a compiled `"xterm-256color"` in its place. Nineteen
ways of recording that the environment does nothing.

**The measurement is the reason the phase exists rather than an argument for it.** A
prototype that DELETED eight of the ten built-in terminal names and three of the nine
capability tables — 118 lines of terminal description — passed `tools/zcompare.py`
against the real baselines **declaring nothing at all**. The only thing that moved in a
five-part recording was two lines of stderr, which `2 stderr-moved` already absorbs. A
phase may declare nothing only when the instrument could have seen it; here it could
not.

`tools/ztermcheck.py` now asks `+set term={name}` on the command line, which reaches
`did_set_term()` rather than `termcapinit()`'s compiled default, and records the `E5NN`
beside the answer where one is given. **Ten of the nineteen names resolve to
themselves** — `term=screen t_Co=8`, `term=debug t_Co=` — and **nine are refused**,
recorded as `E522 term=xterm-256color t_Co=256`: the error *and* the terminal the editor
stayed on, which is what makes a refusal distinguishable from the old vacuous row. It
goes straight to `+set term=` and never to `-T`, measured: a `-T` harness run against a
binary with no `-T` records nineteen `(none)` rows, and `+{command}` is `WHIM-PLAN.md`
II decision 8, the one facility promised to survive every phase.

Two things the tool had to get right, both measured. The error line **echoes the
assignment** — `E522: Not found in termcap: term=vt320` — so a naive `find('term=')`
reports the *requested* name as the result; any line carrying an `E<digits>:` has the
code taken off it and is then skipped. And the row label is `:set term=` and not
`TERM=`, or the record would say `TERM='vt320'` about something that is not the
environment at all, so `termcheck.one` is overridden as well as `termcheck.ask`.
`tools/termcheck.py` itself is untouched: it is named by `tools/whimdelta.sh` and
`tools/verify.sh` and its bytes are in every whim stage's key (core rule 9).

### The re-record is the delicate part, and it is not `CLAUDE.md`'s mistake

`CLAUDE.md`'s rule is *never regenerate it from the current binary, which would make the
comparison self-fulfilling*, and the mistake it names is a pipeline re-recording from
its **own output**. `pipes/whim83.sh` does the opposite and enforces it: the baselines
come from `whim-vim.c`, the pipeline's immutable input, built with **whim's** compile
line, recorded three times and required identical. Nothing zero produces is on the
recording side. The incantation is

```sh
rm -rf .reference/zero-baselines .cache/r0 && make whim-phase-83
```

and **both paths are needed**: measured, with only `.cache/r0` removed `pipes/whim83.sh`
refuses — *"baselines DIFFER from the recorded `.reference/zero-baselines` … a harness
changed, or the frozen `whim-vim.c` did. Name which before removing it"* — and exits 1
naming `ref-term.txt`. It is right to refuse. `whim.mk`'s `whim-baselines-check` said
only `rm -rf .cache/r0`, which is correct for the MISSING case and wrong for the
changed-harness case a reader will actually hit, so its message now names both paths and
says why; `whim.mk` is in no implementation digest.

### What makes the re-record safe is measured and not cited

The baseline and every phase's recording move **together**, and `pipes/whim116.sh`
measures that: `./zero-vim` extracted from every recorded boundary tar — all 33 of them —
plus `whim-vim.c` built with whim's own line records the same table, **one digest across
every one of them** under the new question, exactly as the old question gave one digest
across every one of them. So `term-moved` stays undeclared at every phase before this
one and after it, and `tools/zcompare.py` agrees: the declared delta at every boundary is
the cumulative list through phase 94 and nothing new, checked at 89, 98, 105 and 115 by hand
as well as at all 34 by `make`. **That check is also what covers a stale tier-3 replay**:
nineteen other zero units keep their keys and would replay with a `ref-term.txt` recorded
under the old question, so section 4 re-derives, for every recorded boundary binary, the
thing such a replay would carry over.

### The instrument is proven able to fail, and the one it replaces proven not to be

With **one** row deleted from `builtin_terminals[]` — the last named row, chosen by the
program and not written into it — the new table moves exactly **one** of its nineteen
rows, `term=debug t_Co=` → `E522 term=xterm-256color t_Co=256`, and the question this
replaces, asked of the same two binaries, moves **0 of 19**: its nineteen rows carry one
distinct answer between them. That pair is the whole phase in one measurement.

**Nothing the check asserts is a number that was observed.** The table has as many rows
as `tools/termcheck.py` has names; which of them resolve is read out of
`builtin_terminals[]` in the source the phase was handed; and what a refused name leaves
the terminal as is measured from the binary, by asking it with no `+set term=` at all,
rather than written down as `xterm-256color`. So the rules stay true of the phase that
deletes eight of those names. The undefined symbol count is **reported and not pinned**
for the same reason: a number this phase cannot move is not a check, it is a thing to go
stale.

### One second change to the tool, and it is not cosmetic

Every session made a scratch directory in `/tmp` and left it there. Measured while this
phase was being written: **182,319** of them were lying about — 100,280 `termcheck-*` and
82,039 `ztermcheck-*` — and an ext4 directory that full answers `mkdir` with `ENOSPC` on
a disk with 70 GB free. That failed phase 92, a phase with nothing to do with
terminals, in the middle of a run of this one. `ztermcheck.py` now removes its own
directory; `termcheck.py` is whim's and slim's and is left alone.

### Measured

| | input | after |
| --- | --- | --- |
| `zero-vim.c` | 79,776 lines | **79,776, byte for byte** — `cmp`-identical |
| the boundary digest | `d2a14122ccf7` | **`d2a14122ccf7`**, its input's |
| `make editor.c` | 77,889 | **77,889**, 11 `#include`s with none above them |
| binary | 782,760 | **782,760**, `EXEC`, no `INTERP`, no dynamic section, no relocation |
| `nm -u` | 17 | **17**, reported and not pinned |
| the nineteen rows | one answer between them | **ten resolve to themselves, nine are refused with `E522`** |
| a deleted `builtin_terminals[]` row | moves **0 of 19** | moves **1 of 19** |
| records that moved | | **0 of 106** — there is no source to move them |

### Its placement

`stage 116`, `package harness 3 33` — the package that changes no source at all. Two
`uses`: `seed:83`, because the nineteen rows it re-records are phase 83's and phase 83
**refuses** to overwrite a set that differs; and `streams:88`, because the question is
`+set term={name}` on a command line with **no file on it**, which phase 88 made an
unknown option — and which is also what left the old question asking `$TERM` with an
empty buffer.

**Phase 116 can share a stage with nothing and needs no `apart` to say so**, exactly as
phase 86 does not. A stage of more than one phase is made of **split** programs and
`pipes/whim116.sh` is one file, so the schedule is refused before any check runs:
measured, `stage 116-117` gives `phase 116 is in stage 116-117 but is not an edit and a check`
from `tools/stages.sh`, and `tools/phaserun.sh` refuses the same unit with `phase 116
has no edit and check to run`. Nor is there a `need`: a whole-phase program is handed the
previous boundary's tree and has no edit part for a sweep to precede.

**Key movement, measured over all 170 implementation keys of the three pipelines** — 12
slim phases, 13 whim stages, 82 whim edits, 33 zero units, 30 zero edits — in a scratch
copy of `tools/` and `pipes/`, one change at a time: editing `tools/ztermcheck.py` moves
**16, every one of them zero's** (units 83, 86, 88, 92, 96, 104, 108, 109, 110, 111, 112, 113, 114,
115 and edits 88 and 108), and **adding the phase moves 0 of 170** and adds one unit — which
is what zero's phase list living in `pipes/whim.stages` rather than in
`tools/pipeline.sh` buys.

## Phase 117 (zero 34) — the core stops reallocating

`pipes/whim117-edit.sh` and `pipes/whim117-check.sh`, `stage 117`, `package boundary`.

**`realloc` cannot be implemented from `malloc` and `free`**, and that is why this phase
is a rewrite of two call sites rather than a seventeenth vendored function beside phase
97's sixteen. To move the old contents it has to know how many bytes the old block held,
and its interface — `void *realloc(void *p, usize n)` — does not carry that number: musl
reads it back out of the **chunk header below the pointer**, which is a fact about musl's
heap and not about C. There is no `musl_realloc` that can be written at all, because
there is nothing to give the copy for a length. The only route left is each call site
with the size **it** knows, and the phase exists because both core sites know it.

`ga_grow_inner()` already computes its own — `old_len = (usize)gap->ga_itemsize *
gap->ga_maxlen;`, on the line after the call, to zero the new tail; the rewrite hoists
that line above the allocation and copies exactly it. `get_keystroke()`'s is `buflen`
before the `buflen += 100;` immediately above the call, and the rewrite saves it as
`t_buflen` beside the `t_buf` the input already saves, so the two halves of what
`realloc`'s interface does not carry — the old pointer and the old size — sit on adjacent
lines. `void *realloc(void *p, usize n);` leaves the core's libc prototype block, **six
lines to five** — a number the check COUNTS from its input rather than states, because
phases 114 and 115 shrank the same block just before this one.

**The symbol does not leave, and that is stated as an equality because a reader will
expect otherwise.** `nm -u` is 17 names before and 17 after, the same set as a `comm`
empty in both directions, with `realloc` still among them: the third call site is
`adjust_types()`, in the formatter island phase 110 moved below the first `#include`,
which is the host's and keeps it. Phases 97, 98 and 104 each require `realloc` to be
undefined and all three still pass. Phase 111 said the same of `gettimeofday` and phase 113
of the four stdio names.

### The four traps are memory bugs and not differences, so the phase owes a harness

A recording cannot see a leak, a double free, a premature free, or an overread whose
bytes are overwritten before anything reads them. `pipes/whim117-check.sh` extracts
`ga_grow_inner()`, `musl_memcpy()`, `musl_memset()`, `garray_T` and `get_keystroke`'s
extension block **from the input source and from the output at run time**, drops both
into the same AddressSanitizer driver, and drives eight doublings from an empty
growarray, six independent first grows, a failed allocation and the 100-byte extension.
The two transcripts are identical, 32 lines, and neither reports a finding.
`B.fail r=0 same=1` is trap 2: the allocation failed, `ga_data` is the block it was, and
the grow after it reads that block back intact.

**Six of seven controls move**, each with its own named finding rather than one bucket:
copying `new_len` instead of `old_len` is a heap-buffer-overflow **read** — and it is
invisible to any recording, the overread bytes landing where the `musl_memset` that
follows overwrites them; freeing the old block on the failure path is a
heap-use-after-free at the grow that follows; `get_keystroke` copying `buflen` is a
heap-buffer-overflow; not freeing the old buffer is a LeakSanitizer report; freeing it on
both paths is a heap-use-after-free. Copying **nothing** gives no sanitizer finding at
all and is caught by the transcript, 84 bytes lost.

**The seventh moves nothing and is reported rather than hidden.** Dropping the
`if (gap->ga_data != nullptr)` guard gives a byte-identical unit transcript and no
finding, because a null `ga_data` implies `ga_maxlen == 0` implies `old_len == 0`,
`musl_memcpy` is a plain `for (; n; n--)` loop that never dereferences, and
`free(nullptr)` is a no-op. The guard is kept for what `WHIM-PLAN.md` §II.4c asks of the
core — that its meaning be on the page, not in what a compiler or a libc happens to
tolerate — and the check proves what it buys with a driver whose `musl_memcpy` announces
a null source: **0** from the output, **8** from the unguarded control.

**Shrinking was measured, not assumed**, because copying the OLD size is wrong if either
site can ask for less than it has. Neither can: `ga_grow_inner`'s only caller enters it
when `ga_maxlen - ga_len < n` and the three statements above the allocation only raise
`n`, `get_keystroke` adds 100 immediately above the call, and an instrumented build marks
a shrink at **0 of 106 records**. The input's own
`musl_memset(pp + old_len, 0, new_len - old_len)` already relied on it, the length being
unsigned.

### The declared delta is nothing at all, and here that is the STRONG kind

Not phase 92's (code that could not run), not 12's or 13's or 30's (code the instrument
cannot see), not 16's or 23's (a byte-identical binary), and not 29's (different answers
no record holds). `ga_grow_inner()` is on the path of every growarray in the editor: the
same instrument inserted at a line both sources have counts **4,289** calls per recording
on the input and **4,289** on the output, **2,739** of them with `ga_data == nullptr` —
trap 1 is the majority case and not an edge — in 104 of the 106 records, the two that do
not mark being `ref-pty.txt` and `ref-term.txt`. Two full recordings are byte-identical,
and the control that keeps the rewrite and copies nothing moves **102 of the 102** screen
cases.

`get_keystroke`'s extension is the opposite and the check says so: it is **unreachable**
in a recording. An instrumented build of the input marks each of the five `continue`
paths inside its loop at 0 of 106 records, so `len` never exceeds one `ui_inchar()` and
`maxlen` never falls below 10, and a pty session feeding a partial escape sequence sixty
times does not reach it either. The unit harness is the only instrument that can drive
it, and it drives both versions.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,776 | **79,786 (+10)** — +5 at `ga_grow_inner`, +6 at `get_keystroke`, −1 for the prototype |
| `make editor.c` | 77,889 | **77,899**, same fourteen boundary names, compared at run time |
| the libc prototype block | 6 entries | **5** — counted from the input, not stated |
| `nm -u` | 17 | **17, the same set**, `realloc` still among them |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, and not `cmp`-identical** |
| the ASan unit transcript | 32 lines, no finding | **32 lines, identical, no finding** |
| controls that move | | **6 of 7**, each with its own named finding |
| records that moved | | **0 of 106**, with 4,289 calls per recording behind it |

### Its placement

`stage 117`, `package boundary 23 25 26 27 28 34` — because the phase's product is one
line fewer in the block phase 109 wrote and phase 110 carried. **Not `vendor`**: nothing is
vendored here, this being the case where the core stops needing a libc function that
*cannot* be vendored. Three `uses`: `seed:83` and `harness:86`, and `vendor:97`, because the
copy both rewrites make is `musl_memcpy`, phase 97's static definition — which is also
why the null guard buys nothing measurable.

**Three `apart` lines, each measured rather than predicted.** `apart 109 117` and
`apart 110 117`: both of those checks assert `void *realloc(void *p, usize n);` is on a
line of its own exactly once, and it is not any more — run against this phase's output
each reports that one prototype and only that one. `apart 113 117`, both directions,
measured when the two were adjacent: on a shared stage phase 113's check stops with *"the
output is 79776 lines and the input was 79857, a difference of 81 where 91 was
expected"*, exactly the ten lines this phase adds, and this phase's check stops with the
mirror image, *"a difference of −82 where 10 was expected"*.

**The pair that would normally need measuring — 116 and 117 — cannot have an `apart` at
all, and that is itself a measurement.** A stage of more than one phase is made of split
programs and `pipes/whim116.sh` is ONE file, so the schedule is refused before any check
runs: `stage 116-117` gives `phase 116 is in stage 116-117 but is not an edit and a check`
from `tools/stages.sh`, and `tools/phaserun.sh` refuses the same unit with `phase 116
has no edit and check to run`. **`need 117` is measured not to be required** — the edit was
run on phase 113's unswept output, 79,858 lines against the swept 79,776, and every anchor
and every count held — and it could not be exercised anyway, a phase whose predecessor can
never share its stage being handed a boundary either way.

## Phase 118 (zero 35) — the core calls nothing but the host

`pipes/whim118-edit.sh` and `pipes/whim118-check.sh`, `stage 118`, `package host`. The three
libc functions the core still **called** for itself go to the host: `malloc`, called by
`lalloc()`; `free`, called by `vim_free()` and `update_wincolor()`; and `write`, called by
`mch_write()` — and, since phase 117 rewrote `realloc` as a malloc, a copy and a free, by
`ga_grow_inner()` and `get_keystroke()` as well.

Above the first `#include`, which since phase 110 **is** the boundary, `malloc` was 4
mentions, `free` 5 and `write` 2: each a declaration in the core's one run of ordinary,
non-`static` declarations, plus its call sites. All three are **0** now, and below the
boundary they are 1, 2 and 2 where they were 0, 1 and 1. Three `static` prototypes go in
the block phase 108 made one run of eleven, so the core → host boundary is still **one**
block of declarations and not a block plus three:

```c
    static void *host_alloc(usize n);
    static void host_free(void *p);
    static int  host_write(const char *s, int len);
```

and three definitions below the boundary make the same libc calls with the same
arguments.

### Not one of those counts is written down, and the reason is that they were once

This phase first asserted `malloc` 2, `free` 3 and `write` 2, the counts measured on the
boundary it was written against. **Phase 117 then rewrote `realloc` at two core sites and
took them to 4 and 5, and the anchors refused** — which is what counted anchors are for,
and better than the alternative. But a count is a fact about a tree that was measured and
a **partition** is a fact about the tree that arrives, so both programs now assert the
SHAPE: every mention of each name above the boundary is its own declarator or a call of
it; the declaration goes, every call is rewritten, and how many there are is read off the
text. A mention that is neither — an address taken, a variable of the name — **refuses**
rather than surviving into a file whose declaration is gone. That is `CLAUDE.md`'s rule
under *Rename a name across the whole file*, and it is what made phase 117 cost this phase
a re-run rather than an edit.

### The wrappers are faithful and not improved

This is the trap the phase could have fallen into with no recording seeing it.
`mch_write()` is `vim_ignored = (int)write(1, (char *)s, len);` — **one** `write(2)`, no
loop, the count assigned to the variable this tree keeps for results it means to ignore. A
short write LOSES those bytes today and `host_write()` loses them too: a wrapper that
looped would be a behaviour change in a phase that declares none, and output that silently
truncated under load is the worst outcome available here. `host_alloc()` returns what
`malloc()` returned, `nullptr` included, so `lalloc()`'s `clear_sb_text()` /
`do_outofmem_msg()` failure path is reached exactly as before; `host_free()` calls
`free()`, so it is null-safe for `free()`'s own reason — the core does not rely on that
(`vim_free()` tests `x != nullptr`, `update_wincolor()` frees only the arm it allocated,
and **0 of the corpus's 22,417 frees are null**) but the wrapper inherits it rather than
adding a test. `host_write()` **drops the descriptor** because its two neighbours on this
boundary already have: phase 103's `musl_read_input(char *, int)` reads fd 0 inside the
host and phase 104's `host_message(msg, len, err)` chooses its stream from a flag. A
descriptor is the host's idea of where the screen is; `host_write(s, len)` is the core's.

### Nothing is freed and the phase says so as an equality

`nm -u` is the same set in and out, a `comm` empty in both directions — 17 names with
zero's own flags, 18 as `tools/symbols.sh` counts — with `malloc`, `free` and `write`
still in it. That is this pipeline's own rule read back: **a symbol leaves when its last
CALLER leaves the file**, and inside one translation unit moving a call from the core into
the host moves no caller out. Phase 111 is the contrast, freeing `gettimeofday` because the
last caller went with it.

**What does move is the thing `nm -u` cannot show.** `make editor.c`'s cut — 77,899 lines
either side, a byte prefix of the file, 0 errors under `-fsyntax-only` — has a warning set
that IS the core → host interface, every name `used but never defined`, and it goes from
**14 names to 17**. `host_alloc`, `host_free` and `host_write` arrive and nothing leaves.
An implicit libc dependency hidden in a bare declaration becoming an explicit named call
is the point of the boundary, and an interface growing by exactly three is what that looks
like. The input's set is computed in the check at run time and never written down: a
written list has gone stale twice in this pipeline already.

### The declared delta is nothing at all, and here the recording is STRONG evidence

Two full `tools/zrecord.sh` recordings, `diff -r` empty across all 106 records. `lalloc()`
and `vim_free()` are on the path of essentially everything the editor does and
`mch_write()` is every byte it draws, so the corpus hammers all three. Measured on an
instrumented build of this phase's own output, over the 102 screen cases and marking every
one of them: **53,848** `host_alloc` calls with the largest 319,968 bytes; **22,417**
`host_free` calls with 0 null; **1,012** `host_write` calls with the largest 2,063 bytes
and 0 short. A by-hand session, `+normal 200000ax`, makes 400,475 `host_alloc` calls
against 479 for `ihello world<Esc>`, and frees 400,188 of them.

**And it can fail, once for each function — with two controls that move nothing, reported
and not hidden.** `host_alloc` returning `nullptr` always moves 106 of 106 records;
refusing only allocations above 200,000 bytes — in this editor exactly ONE, the screen —
moves 100 of 102 screen cases, the survivors being `ctrl_c_clean` and `ctrl_c_changed`,
which exit before a key is looked up. `host_write` writing HALF THE BYTES moves 102 of
102, and `host_write` writing every byte and REPORTING half moves **0 of 102**, which is
the phase's claim about the return value measured rather than argued. `host_free` doing
nothing at all moves 0 of 102, because a leak is invisible to a 106-record corpus; the
instrument is what says `host_free` is called, and the check says so in those words
instead of presenting a silent control as evidence. `host_free(nullptr)` on every draw
moves 0 of 102 and writes nothing.

**The `static` trap is built both ways.** `nm --extern-only --defined-only` is still
exactly `main`: with the keyword off the three prototypes gcc refuses — *"static
declaration of 'host_alloc' follows non-static declaration"* — and with it off the
prototypes AND the definitions the build is silent and the object defines `host_alloc`,
`host_free`, `host_write` and `main`. Deleting the three prototype lines gives 9 errors
naming all three, which is what makes them load-bearing rather than decorative.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,786 | **79,804 (+18)** — three prototypes and three five-line definitions with their blanks |
| `malloc` / `free` / `write` above the boundary | 4 / 5 / 2 | **0 / 0 / 0** |
| the same three below it | 0 / 1 / 1 | **1 / 2 / 2** |
| the core's plain libc prototype block | 5 lines | **2** — `getpid` and `kill` |
| `make editor.c` | 77,899 | **77,899**, 0 directives, 0 errors |
| the boundary | 14 names | **17**, computed at run time on both sides |
| `nm -u` | 17 | **17, the same set** — `malloc`, `free` and `write` all still in it |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, 206,588 of them different** |
| `cmdnames[]` / `options[]` rows | 98 / 107 | 98 / 107 |
| records that moved | | **0 of 106**, against 53,848 + 22,417 + 1,012 instrumented calls |

### Its placement

`stage 118`, `package host 17 18 19 20 21 30 32 35` — a thing the core did for itself
becomes a thing it asks the host to do, declared in the one host block and defined below
the boundary. Four `uses`: `seed:83` and `harness:86`; `boundary:108`, a `static` forward
declaration above and a `static` definition below being all a direct call to the host
needs, which is that phase's finding; `boundary:110`, because *above the boundary* and
*below it* are a LINE NUMBER in this check and phase 110 is what made that line the split;
and `boundary:106`, because `host_alloc(usize n)` is spelled in the core's own type names
and a signature in `size_t` would name something no header above the boundary declares.

**Six `apart` lines, every one a measurement.** 109 and 110 carry a nine-entry `PROTOS` list
and require each `\n<prototype>\n` exactly once: on this output **two** are at 1 — `getpid`,
`kill` — and **seven** at 0, the three phases 114 and 115 took, `realloc` which 117 took, and
this phase's three; and 110 builds a control by putting `static ` in front of the `malloc`
prototype, which is no longer there to put it in front of. 110 and 111 write the boundary out
as a list of names and require the cut's warning set to be exactly it, and this phase makes
it seventeen where 115 made it fourteen.

**`apart 117 118` was measured as a REAL SHARED STAGE** — q116 restored, both edits, one sweep,
both checks — and phase 117's check stops four ways. The first two are its prototype block:
*the core's plain libc prototype block is 2 lines and the input's was 6, a difference of 4
where 1 was expected*, and *the prototype block lost [long write(…) / void \*malloc(…) /
void \*realloc(…) / void free(…)] and not realloc's line alone*. The other two are
`apart 105 106`'s lesson landing on the phase that had just taught it: *ga_grow_inner's
rewrite is not in the output exactly once*, and the same for `get_keystroke`'s, because
phase 117's check matches the code it wrote VERBATIM — `pp = malloc(new_len);`,
`free(gap->ga_data);` — and this phase renames exactly those calls. **A check that quotes C
is a dependency on the spelling**, and here the quoting phase and the respelling phase are
adjacent. One direction only: phase 118's check was then run on that same tree and every
part of it held.

`apart 113 118` is the same rule one level deeper, and it is kept from an earlier base
because it is the clearest instance of it: phase 113's check writes
`write(2, "T-cleos\n", 8);` INTO THE CORE to build an instrumented control, and relied on
the core's own declaration of `write` — so run as a pair it does not compile at all.
**A check that CALLS libc from above the boundary is a dependency on the core still
declaring it.** There is **no `need 118`**, measured three times — on phase 113's unswept
output, on phase 115's and on phase 117's — the partition holding identically each time.

### What this phase does not yet claim

The sentence this arc has been building to — that the core calls no libc function at all,
every outward call a `musl_` or a `host_` — is **not** stated, because it is not true of
this tree. `getpid` and `kill` remain, two lines of prototype and three call sites:
`mch_get_pid()`'s `getpid()`, and `vim_handle_signal()`'s `kill(getpid(), got_signal)`,
which re-raises a deferred deadly signal. The phase is written so that the claim becomes
true without another edit — the block is FOUND rather than assumed, the lines this phase
owns are taken out of whatever run holds them, the residue is printed, and when the residue
is empty the edit drops the block's trailing blank line with it — but **the phase that
takes those two is the one that gets to write it down.**

### What zero-vim is after phase 118

```
zero-vim.c        79,804 lines          from whim-vim.c's 86,614  (-6,810, 7.9%)
                  77,899 above the boundary, 1,905 below it
functions         1,759
type definitions  908
DWARF enumerators 1,189
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, at line 77,901, and NOT ONE DIRECTIVE above them
core -> host      17 names: vim_snprintf, host_exit, host_message, host_time,
                  host_alloc, host_free, host_write, ten musl_*
libc prototypes   2 in the core: getpid kill
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            782,760 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
make editor.c     77,899 lines: 0 directives, 0 errors, 17 warnings, all of them
                  `used but never defined` and all of them the interface
```

**Fifteen phases in a row have declared nothing** — 104 through 118 — and the kinds of
evidence that stand in for a recording are named rather than counted here; the one
numbered list is `CLAUDE.md`'s, and the taxonomy is written out at the end of phase 122.
**Phase 116's is a kind new with it, and it is phase 86's**: the phase changes no source at
all, so nothing about the editor's behaviour *can* have moved, and what has to be argued
instead is that the **comparison** moved safely. 117 and 118 are the strongest instance of a kind the pipeline already had —
two byte-identical recordings — because what they touch is on the path of everything:
34's `ga_grow_inner` at 4,289 calls a recording, 35's `lalloc`/`vim_free`/`mch_write` at
53,848, 22,417 and 1,012 over the screen cases, each with a control that moves 102 of 102
to say so. **Fifteen of the seventeen libc symbols are now
called from the host and from nowhere else**: `malloc`, `free` and `write` joined them
here, `time` at 32 and `gettimeofday` at 28, and `__errno_location` is gcc's own for the
host's `errno`. **The two that are not are `getpid` and `kill`.** What the core still does
for itself is one re-raise of a deadly signal and one `getpid()` that fills a `b0_pid`
nothing reads.

## Phase 119 (zero 36) — the core names no libc function at all

`pipes/whim119-edit.sh` and `pipes/whim119-check.sh`, `stage 119`, `package host`. Phase 118
ended with a sentence it would not write down, and this is the phase that gets to write
it. The core's block of ordinary, non-`static` declarations — the libc the editor spells
out by hand since phase 110 put the headers below it — was two lines, and both go:

```c
    int getpid(void);
    int kill(int pid, int sig);
```

**They are reached two different ways and only one of them needed a host call**, which is
the whole shape of the phase. `getpid` is **avoidable outright**. `mch_get_pid()` is
`return (long)getpid();` and had exactly one caller, `long_to_char(mch_get_pid(),
b0p->b0_pid)` in `ml_open()`; `b0_pid` is the process id written into block zero of a swap
file and had exactly two mentions in the whole file, its own declaration and that write.
It is **write-only, and not by anything zero did** — `whim-vim.c`, this pipeline's
immutable input, already has exactly those two, whim having taken the readers with
recovery. So the write goes, `mch_get_pid()` goes with it because the declaration it needs
is going, the sweep takes the forward declaration and the field, and `getpid` leaves the
core with nobody calling a host at all. Phase 103 saw it coming and said so in as many
words: *"`b0_pid` is written and never read, so one line frees it whenever block zero is
somebody's phase"*.

`kill` is **moved**. Its one core site is `vim_handle_signal()`'s `kill(getpid(),
got_signal);`, re-raising a deadly signal that arrived while the editor was not reading,
and it becomes `host_raise(got_signal);` — `static void host_raise(int sig);` at the end
of the single run of core → host prototypes phase 108 made one block of eleven, and a
definition inside the host region that calls `kill(getpid(), sig)`. **The deferral is not
dropped**: the blocked/`got_signal` mechanism is exactly what it was, so the core still
decides *when* the signal acts and only the raising crosses the line. **It takes no pid**,
which is phase 118's rule for `host_write` and phase 103's for `musl_read_input` applied
here: a core that can no longer ask for its own process id must not be handed one. And it
is spelled `kill(getpid(), sig)` and not libc's `raise()`, because `raise` has not been an
undefined symbol of this file since phase 103 and a wrapper reaching for it would **add** a
libc symbol in the phase whose subject is the core's last two.

### The claim is measured from the cut and not from the block, and those are two assertions

An empty block is a fact about a paragraph. *The core names no libc function at all* is a
fact about everything above the first `#include`, and the two are not the same sentence,
because **a bare `extern` declaration is invisible to the cut's ordinary check**: gcc
warns `'X' used but never defined` for a `static` function and says nothing whatever about
an ordinary one. That is exactly how two libc names sat above the boundary for nine phases
without the interface set noticing them.

So the check compiles `make editor.c`'s cut to an **object** and takes `nm -u` of it,
which is the set of names the core needs from outside *itself*: **19 in, 18 out**, and
every one of the 18 defined below the boundary in this same file, computed from the text
and listed nowhere. The identical computation on the input finds two that are not —
`getpid` and `kill` — and **that control is what keeps the emptiness from being two
numbers agreeing**. Neither cut defines an external symbol either: `main` is below the
boundary and is the host's.

### The phase frees no symbol and says so as an equality

`nm -u` is the same set in and out, a `cmp` in both directions, and `getpid` and `kill`
are both still in it: `host_raise()` calls both where `vim_handle_signal()` did,
`musl_suspend()` calls `kill` as well, and **a symbol leaves when its last caller leaves
the file**. What moves is the thing `nm -u` cannot show — the cut's warning set, the
core → host interface, from **17 names to 18**, `host_raise` arriving and nothing leaving.

And the thing no tool but `tools/zhostonly.py` can show: **above the boundary the core's
whole remaining vocabulary of the host is two words**, `SIGHUP` three times and `SIGTERM`
three, the two deadly signals the editor names because it **prints** them. The input said
`getpid` three times and `kill` twice as well, and those were the only mentions of any
host word in the core that were not a message. The tool reports 6 of its 18 named
exceptions live, and all six are that message.

### The fold this phase was asked to take does not exist, and the check says so

Phase 100 removed `deathtrap()`'s `entered >= 3` ladder as code no build of zero-vim could
reach, which leaves `entered` able to reach 2 and no further, and the question put to this
phase was whether that makes anything around the `if (entered == 2)` arm foldable.
Measured, it does not: **`entered` has exactly three reachable values and every one of
them is read.** 0 is read by the entry guard `if (entered == 0 && ...)`, which
distinguishes a first entry from a nested one; 1 and 2 are told apart **twice** — by the
double-signal arm, which calls `getout(1)` and never returns, and by `v_dying = entered;`,
whose value reaches `getout()`'s two `if (v_dying <= 1)` tests and selects the buffer
cleanup there. No two of the three states are interchangeable, so there is nothing to
fold. The non-fold is asserted as a byte comparison: `deathtrap()` is identical in and out
at 38 lines either side, and `vim_handle_signal()` differs in exactly one line.

### The declared delta is nothing at all, and the two halves are blind for opposite reasons

It is phase 85's kind and phase 95's — the code **runs** and the instrument cannot see it —
so the phase owes probes and builds six binaries of its own. That the corpus cannot see
either half is measured. An instrumented build of the input, over all 102 screen cases and
marking every one: the `b0_pid` write runs in **102 of 102** cases, 102 times in all, being
on the path of every buffer the editor opens, and the recording still does not move,
because nothing reads the field. The re-raise fires in **0 of 102**: nothing in the corpus
sends the editor a deadly signal, so `got_signal` is never set and the deferral has
nothing to re-raise. `vim_handle_signal()` itself is entered 4 times in 2 cases, which is
**reported and not pinned**, this phase being unable to move it — `ui_inchar()` calls it
only around a wait longer than 100 ms, and a corpus whose stdin is a file of keystrokes
almost never waits.

**So the two controls are on the one line the phase deletes**, and they are the phase
itself rather than a borrowed anti-vacuity check. That line replaced by `return FAIL;` —
the same line, in the same place — moves **106 of 106** records, so the corpus really does
execute it and the empty `diff -r` is not the corpus missing the code. The same line
writing a **constant** instead of the pid moves **0 of 106**, which is *`b0_pid` is
write-only* measured rather than argued, and a control that moves nothing is reported here
and not quietly dropped.

**The probe the other half owes is a forced deferral**, driven identically into both
binaries: `(void)vim_handle_signal(SIGTERM);` with `blocked` still TRUE and then
`(void)vim_handle_signal(-2);`, appended to `mch_init()` — exactly the path
`kill(getpid(), got_signal)` was on and `host_raise(got_signal)` is on now. Input and
output agree in every byte of stdout (48), stderr (0) and status (1), and the screen
carries `Vim: Caught deadly signal TERM` and `Vim: Finished.`. It can fail twice, and
identically on both binaries: with the re-raise **deleted** the deferred signal is simply
lost and the editor runs on to end of input (157 bytes), which says the probe goes through
the line this phase rewrites; and with the re-raise given `SIGHUP` instead of the signal
that was deferred the screen says `Caught deadly signal HUP` (47 bytes), which says the
**argument** crosses the boundary and not merely the call.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,804 | **79,799 (−5)** |
| the core's plain libc prototype block | 2 lines | **0 — the block is gone** |
| `nm -u` of the **cut**, compiled to an object | 19, two of them libc | **18, every one defined below the boundary** |
| `make editor.c` | 77,899 | **77,888**, 0 directives, 0 errors |
| the boundary's warning set | 17 names | **18**, computed at run time on both sides |
| `nm -u` of the whole file | 17 | **17, the same set** — `getpid` and `kill` still in it |
| external symbols | `main` | `main` |
| host words in the core (`zhostonly.py`) | `SIGHUP` 3, `SIGTERM` 3, `getpid` 3, `kill` 2 | **`SIGHUP` 3, `SIGTERM` 3** |
| the `#include`s | line 77,901 | line **77,890**, the same eleven consecutive lines |
| binary | 782,760 | **782,760 bytes, 397,978 of them different** |
| `cmdnames[]` / `options[]` rows | 98 / 107 | 98 / 107 |
| records that moved | | **0 of 106**, against a write executed 102 times |

### Its placement

`stage 119`, `package host` — a thing the core did for itself becomes a thing it asks the
host to do, or stops doing. It belongs beside 113, 115 and 118 rather than in `boundary`,
which is about drawing the line and about the core's own spelling of types and constants.
Four `uses`: `seed:83` and `harness:86`; `boundary:108`, the one prototype going at the end
of the single run of core → host declarations that phase made of eleven; and
`boundary:110`, because the whole claim is stated as a property of the **cut**, and the
first `#include` is the line between core and host only because 27 moved the eleven
directives below the core.

**One `apart` line, measured as a real shared stage**, q117 restored with both edits, one
sweep and both checks: phase 118's check stops three ways, the first being the block this
phase exists to empty — *the ordinary declarations above the boundary are none and the
input's block minus the three is `int getpid(void); / int kill(int pid, int sig);`* — then
*the file is 79799 lines and the input was 79786 — expected 18 more*, and *the boundary
moved from line 77901 to line 77890, and it must not*, which is phase 118's own statement
that it moves no line above the first `#include`. One direction only: phase 119's check was
then run on that same tree and every part held. **Six further `apart` lines are not
written, with the reason.** Phases 109, 110, 111, 113, 115 and 117 all have checks this output
breaks — 109's and 110's nine-entry `PROTOS` list reaches **zero of nine** on it, which is
this arc read from the losing side — but every one of them is already broken by phase 118
and recorded against it, and any stage holding 36 and one of the six would have to hold 35
too. **A redundant `apart` nobody measured is worse than none.** No `need 119`, measured in
the same run on phase 118's unswept output.

`tools/zhostonly.py` gains `getpid` as a host word and three named exceptions, and the
`vim_handle_signal:kill` exception gains a 0 beside its 1 — a count there is a **tuple**
of the values it takes, one per phase that runs the tool, because the core's vocabulary
shrinks between them. Measured, the cost of that tool edit is **20 zero keys and no whim
or slim key**: the units and the edits of phases 103, 104, 108, 109, 110, 111, 113, 115, 117 and
118, with all 107 whim unit, whim edit and slim phase keys byte-identical either side.
`make whim-verify` (13 of 13) and `make slim-verify` (12 of 12) are the gate core rule 9 asks
for, and both were green.

## Phase 120 (zero 37) — the degenerate unions go

`pipes/whim120-edit.sh` and `pipes/whim120-check.sh`, `stage 120`, `package tidy`. Thirteen
`union` keywords in `zero-vim.c`, and **six of them union nothing with anything**. Five
are single-member — `u_header`'s `uh_next`, `uh_prev`, `uh_alt_next` and `uh_alt_prev`,
each `union { u_header_T *ptr; }`, and `typval_S.vval`, `union { varnumber_T v_number;
}` — and the sixth is **empty**, `union { } es_info;`, with one mention in the whole file
and no use at all.

Every one is a leftover of a cut already made. The `u_header` unions had an arm that named
a **swapfile block number** and it went with the swapfile; `vval` had nine arms and has had
one since the eval layer went; `estack_T.es_info` was a `ufunc_T *` beside an `sctx_T *`
and both went the same way. **A variant type with one variant is a value with a longer
spelling, and a variant type with no variants is a GNU C extension ISO C forbids** —
`WHIM-GOAL.md`'s core is what a transpiler reads, so both cost a reader something and
neither buys anything.

### Which six is computed, not listed

The edit scans for `union`, matches the braces, counts the member declarations at depth 1
and takes every union with fewer than two as degenerate — and it must find **both kinds**,
so a scanner that stopped matching cannot pass by finding nothing. Measured on q119: 1
member for `uh_next`, `uh_prev`, `uh_alt_next`, `uh_alt_prev` and `vval`, **0** for
`es_info`, and 2 or 3 for `ae_u`, `lv_u`, `os_oldval`, `os_newval`, `rs_u`, `se_u` and
`rs_un`, which stay byte for byte and whose text is required to occur once in both files.
Thirteen keywords become **seven**.

A single-member union becomes its member **carrying the union's name** — the replacement
text is the member's own declaration, so the type, the stars and the spacing are the
input's — and every `uh_next.ptr` becomes `uh_next`. The empty one is deleted outright.

### A partition and not a count

For each of the six, every mention outside a string literal must classify as **its own
declaration** or a **`.member` access on it**, and a mention that is neither refuses: a
whole-union assignment, a `sizeof`, a designated initialiser, or another struct with a
field of the same name and a different member would each stop the phase. Measured on q119:
`uh_next` 1 + 27, `uh_prev` 1 + 23, `uh_alt_next` 1 + 28, `uh_alt_prev` 1 + 26, `vval`
1 + 19, `es_info` 1 + 0, with nothing left over — **identical to what the same computation
gives on q118**, so phase 119 moved nothing of this phase's. Those numbers are read off the
text by both the edit and the check and written into neither, which is the lesson phase 118
was taught when phase 117 moved its counted anchors.

The rewrite is literal-aware and **single-pass**: 129 spans computed against the original
text and applied together, over 6,707 literals none of which holds any of the six names,
because a second pass would index spans computed on the first pass's output — phase 106
measured five of 437 `size_t` left behind that way.

### The evidence is that the binary is byte-identical, and the control is what makes it evidence

A union of one member has the size, the alignment and the offset of that member, and an
empty union contributes no storage, so no layout moves and `uh_next.ptr` and `uh_next`
name the same object at the same address. **782,760 bytes either side**, both built with
`SOURCE_DATE_EPOCH=0` and the boundary's own flags — tier 1 of `CLAUDE.md`'s verification
table, which subsumes every screen case, Ex-command row, command line and pty scenario at
once, because the program that would be run is literally the same program.

Two numbers agreeing prove nothing on their own, so **`c1` is built from the input's own
text**: this phase's output with the two fields it *promotes*, `uh_next` and `uh_prev`,
**exchanged** — a pure layout permutation of the very struct it rewrites. It builds and
differs in **31,056 bytes**, so the binary is demonstrably sensitive to that struct's
layout. Two further controls move nothing and are **reported rather than dropped**: `c2`
puts the empty union back and `c3` runs the phase backwards on `vval` with all 19
`.v_number` accesses, and both are byte-identical. That is the claim stated in the only
direction a control can state it in. All three are built from the input's own text and
never from C quoted in the check, because **a check that quotes C is a dependency on
spelling** (`apart 105 106`).

### The empty union's dialect argument is measured, not asserted

gcc reports `union has no members [-Wpedantic]` **once** on the input and not at all on
the output, and the rest of the pedantic diagnostic set does not move — 199 → 198, a
difference of exactly one. A minimal probe confirms the construct is a hard **error**
under `-pedantic-errors` and that the identical struct with a one-member union is silent:
the probe proving it can pass, in the same run.

### The declared delta is nothing at all, and it is the strongest kind and not the weakest

`pipes/zero.delta` gets a comment and no line. This is phase 99's and phase 106's kind — a
`cmp` of the binary — so the phase owes no probes, unlike 103 and 105, and asks nobody to
believe a replacement does what an original did, unlike 97 and 98. Two full
`tools/zrecord.sh` recordings are identical across all 106 records, and the check says in
those words that **with a byte-identical binary that is a check on the harness and not on
the phase**; it is not offered as the evidence.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,799 | **79,786 (−13)**, every one above the boundary |
| `union` keywords | 13 | **7**, computed |
| `make editor.c` | 77,888 | **77,875**, 0 directives, 0 errors |
| the boundary | 18 names | **18**, computed from the input at run time |
| `nm -u` | 17 | **17**, a `cmp` in both directions |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, and the same bytes** |
| `cmdnames[]` / `options[]` rows | 98 / 107 | 98 / 107 |
| records that moved | | 0 of 106 — the harness, not the evidence |

### Its placement

`stage 120`, `package tidy 13 37 42` and **not `dialect`**: five of the six are leftovers of
earlier cuts, which is phase 96's kind, and only the sixth has a dialect argument.

**It was for a while the one phase after the seed with no `uses` line at all**, and that
looked defensible — its evidence is a `cmp` and not the recording, so it does not rest on
phase 83's baselines the way every other empty declaration does. **It was still wrong.**
`pipes/whim120-check.sh` names `tools/zerodelta.sh` twice, the delta check runs at its stage
end like every other phase's, and the two **other** `cmp`-evidenced phases, 99 and 106, both
declare the dependency — five `uses` lines and six respectively. So the line is written now,
with the reason it was missing recorded in it. It was found by a documentation pass and not
by anything failing, which is the property of `pipes/whim.stages` worth saying plainly: the
file is read by `tools/stages.sh` and `tools/packages.sh` and **named by no phase program**,
so `tools/implhash.sh` never hashes it. **A manifest edit is free, which is why package and
`uses` data can be kept honest without paying for a repass — and it is also why a wrong one
is never caught by anything running. The only guard on that file is a reader.** Measured
either side of the correction: slim 9, whim 13-41, zero 37 and zero 44 are byte-identical,
and both manifest checks pass.

**`apart 119 120` is written and was measured in both directions**, as a real shared stage on
q118, which 119 and 120 make possible by both being split. Phase 119's check stops first, with
*the file is 79786 lines and the input was 79804 (79804 recorded) — expected 79799* and
*the boundary moved from line 77901 to line 77877*: 119 states its arithmetic as a line
count and 120 takes 13 more. The other direction was **observed rather than reasoned**, by
running phase 120's check on exactly the tree and state directory the driver would have
handed it next: *the file is 79786 lines and the input was 79801 — the six declarations are
13 lines shorter between them*, then *the boundary moved by 15 lines and the file by 13*.
**The two extra lines are phase 119's residue** — `static long mch_get_pid(void);` and the
`b0_pid` member, which 36 deliberately leaves for the sweep — landing inside phase 120's
arithmetic because a stage sweeps **once**, at the end. The general form is now in the
manifest: *a phase that states its line count against its own edit cannot share a sweep
with a phase that leaves work for it.*

No `need 120`, measured in the same run: the edit was handed phase 119's **unswept** output
and every part held, at exactly the counts it gets on swept text. It asserts no count a
sweep can move. Adding the phase moved no existing implementation key — 144 whim, slim and
zero keys identical either side, with only z37 new.

## Phase 121 (zero 38) — the eight terminal names go, leaving two

`pipes/whim121-edit.sh` and `pipes/whim121-check.sh`, `stage 121`, `package terminal`.
`builtin_terminals[]` is the whole of what the core knows how to draw on: a name and a
capability table, ten times. **An embeddable core has no business carrying ten terminal
descriptions** — the host decides what it is attached to — so this phase keeps **two**:
`xterm-256color`, which is already the name `termcapinit()` substitutes when it is given
none, and `debug`, which draws its capabilities as text and is the only one readable
without a terminal at all. Which rows go is **the table minus the two**, computed from the
source; the two are the only names the phase writes down.

### This is the first zero phase to declare `term-moved`

The token has been in the delta grammar since phase 83 and no phase had used it, because
the terminal table was nineteen ways of recording that the environment decided nothing —
until phase 116 changed the question to `+set term={name}`. And **phase 116 exists
because a prototype of this cut passed `tools/zcompare.py` declaring nothing at all**. So
the declaration is a real delta and not a harness artefact, which is the distinction rule
2 and `CLAUDE.md`'s *A phase can break a harness rather than change behaviour* exist to
keep apart, and this phase is on the other side of it from phase 88's.

`pipes/zero.delta` states which **eight** of the nineteen rows move and to what, because
the token itself is whole-file: `xterm`, `screen`, `screen-256color`, `tmux`,
`tmux-256color`, `vt100`, `ansi` and `dumb`, each from `term=<itself>` to `E522
term=xterm-256color t_Co=256` — the answer the eight names that never had a row already
gave. The other eleven are byte-identical: `xterm-256color` and `debug` still resolve, the
eight unknown names were refusals already, and `''` is still `E529`, which is `'term'`
refusing an empty string before any table is consulted.

### Three things the cut drags with it, and none of them is in the row list

**Three capability tables lose their last row** — `builtin_ansi`, `builtin_vt100`,
`builtin_dumb` — and the sweep takes them, 103 lines. That set is **computed**, *a
`builtin_*` table whose only mention left is its own definition*, and the count is taken
on text whose string literals are blanked: `builtin_xterm` is also a string literal inside
`vim_is_xterm()`, and a count that read that as a reference would report a live table
dead.

**`find_builtin_term()`'s xterm-family special case can never fire again.** It is
`strcmp(name, "xterm") == 0 && vim_is_xterm(term)` — a test on the **row's** name — so once
no row carries that name its first conjunct is false for every row. gcc has no warning for
a condition false at run time and nothing in `tools/sweep.sh` reads one, so it goes in the
**edit**. It is dead twice over, measured: the output with the clause restored records byte
for byte what the output records, and the same marker inside it, written through
`host_message()`, is in **102 of 102** screen records on the input and **0** here. And the
finding a reader would not predict is that **every startup of the input resolved through
that clause** — the compiled default is `xterm-256color`, the `xterm` row sorts before it,
and `vim_is_xterm()` says yes — so the surviving row was never reached until now, and it
gives the same table.

**And `set_termname()`'s no-screen fallback named one of the eight.** An unknown name is
refused at run time (E522, the terminal left alone) but *before there is a screen* — the
`-T {term}` path — it is replaced by a name written in the source, and that name was
`"xterm"`. Left alone the cut would have left it dangling, and that was measured rather
than reasoned: with the repair left out, `-T xterm` and `-T no-such-term-9x` print `E437:
Terminal capability "cm" required` and draw **2,045 bytes where the baseline draws
2,117** — an editor with no cursor motion. The phase retargets the fallback onto
`termcapinit()`'s compiled default, computed from that function and required to be a
surviving row, and rewrites the message in the same step, because **nothing in the build
checks that a message tells the truth**. With the repair those two rows are the baseline's
again and `term-moved` is the whole difference; the check builds the repair-left-out form
as a control and requires it to move exactly those two records and no others.

### The edit is a partition and not a count, and it has to be literal-aware

A terminal name here is a string literal, and the same words appear as identifiers
(`builtin_xterm`), as prefixes (`musl_strncasecmp(name, "xterm", 5)`) and inside other
literals (`"screen.xterm"`). Every literal whose **content equals** a removed name must
fall in one of four classes — its row, the family clause, the fallback, or a counted
prefix test in `vim_is_xterm()` — and a leftover refuses. Measured on the input: 11
literals, 8 + 1 + 1 + 1. The prefix class is the only one kept, and keeping it is honest
because each must be an argument of a comparison with **its length written out**, so a
name compared in full could not hide there. All the text edits are applied in one pass
over the original offsets.

### The instrument is shown able to fail in both directions

On this phase's own output, with both rows derived from the tables and neither written
here: deleting the last row the output names moves exactly **1 of 19**, resolving →
refused, and putting the first removed name back moves exactly **1 of 19**, refused →
resolving. A cut of eight that moved seven or nine would have been seen.

### The phase owes probes the nineteen rows cannot give

Because the `xterm` row was the whole xterm **family** and not one name. Six prefixes read
out of `vim_is_xterm()` in the source the phase was handed — `xterm nxterm kterm mlterm
rxvt screen.xterm` — every one resolved before and every one is `E522` now, and **not one
of them is among the nineteen**. `xterm-kitty`, which that function already excluded, was
E522 either way. `:set term=builtin_xterm` is the same finding through `term_is_builtin()`,
which strips the prefix and leaves `xterm`.

### Nothing is freed, and the check says why rather than presenting it as a disappointment

`nm -u` is the same 17 symbols, a `comm` empty in both directions: **this phase deletes
data** — three static arrays and eight rows — **and data calls nothing**.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,786 | **79,668 (−118)** |
| `builtin_terminals[]` rows | 10 | **2**, computed as the table minus the two kept |
| `builtin_*` capability tables | 9 | **6** — three taken by the sweep, 103 lines |
| `make editor.c` | 77,876 | **77,758**, 0 directives, 0 errors |
| the boundary | 18 names | **18**, computed from the input at run time |
| `nm -u` | 17 | **17**, a `comm` empty both ways |
| binary | 782,760 | **781,096** |
| terminal rows that moved | | **8 of 19**, each `term=<itself>` → `E522 term=xterm-256color t_Co=256` |
| screen cases / Ex rows / command lines / pty scenarios | | **0 of 102, 0 of 98, 0 of 30, 0 of 4** |

The cut was stated here as its own check counted it, and that was **one more than phase
120's 77,875 on the same file**: this check took the naive `awk` prefix and phase 120's
takes `whim.mk`'s rule, which drops the cut's trailing blank line. Both are the same text.

**That has since been repaired in both programs, and the repair is worth stating because
the defect was a comment.** `pipes/whim121-check.sh`'s header said the cut was *"whim.mk's
own rule"* while the `awk` beside it was not — and 122 had the same line, and is where the
next phase would have copied it from. **A comment that claims to be the rule and is not is
the thing that propagates.** Both now carry the rule entire, `{ a[NR] = $0; if (NF) last =
NR }` and then print up to `last`, with the reason written beside it; so consecutive phase
commits no longer report cut figures that differ by one **on the same files**, where a
reader comparing them saw an off-by-one that was in neither phase. The figures above are
the numbers those checks printed at the time; the product rule makes them 77,875 and
77,757. The boundaries cannot move and did not — a check produces no tree — and both
phases were **verified to re-run at tier 2** rather than assumed to, because a tier-3
replay copies the recorded digest and would have agreed whatever the change did: q121 at
`9f2b37f26bef` and q122 at `68e450fd6912`. **On this tree a control run through `make` has
to print `tier 2` to be a control at all.**

### Its placement

`stage 121`, `package terminal`, which is *what the core still assumes about the terminal
it is attached to*. Phase 85 removed the **question** — the two "not to a terminal"
warnings, the pause and `--ttyfail`; this phase removes the **vocabulary**. The three
packages it is not in each say something: not `harness`, which changes no source, where
this changes what the editor does and declares a delta for it; not `host`, because nothing
crosses the boundary and the cut's warning set is unchanged either side; and not `tidy`,
because the rows removed are live code a `:set term=` still reaches. Four `uses`:
`seed:83`; `harness:116`, the phase that made those rows say something real; `streams:88`,
whose `-T {term}` is the only way into the fallback this phase repairs; and `host:104`,
whose `host_message()` is the only declared way the core reaches stderr and so the only
way to write an instrument into it.

**`apart 120 121`, one direction and both halves measured.** Phase 120's check states its own
size as a line count of the whole file and of the core, and this phase takes 118 more lines
out of the same swept text: `tools/phaserun.sh whim 120-121` on q119 runs both edits and one
sweep and then stops in phase 120's check with *the file is 79668 lines and the input was
79799* and *the boundary moved by 131 lines and the file by 13*. That is `apart 100 101`'s
shape exactly. The other half was **run** and not reasoned: phase 121's check, given that
stage's swept tree and its own state directory, passes every part, because everything it
asserts is against `$state/old.c`, which its own edit writes. There is **no `need 121`** and
the measurement is reported as vacuous — phase 120's sweep removes nothing, so its unswept
edit output is q120 byte for byte and there is no unswept text here that differs from a
swept one.

**`.reference/zero-baselines` is deliberately not re-recorded, and re-recording it would be
wrong.** `term-moved` is cumulative like `2 stderr-moved`: declared here and inherited by
every later phase, so `ref-term.txt` disagreeing with the baseline is exactly what the
declaration says. Phase 116 needed a re-record because the **harness** changed shape; this
phase changes the **editor**.

**And it broke another phase's program, which it measured and declined to repair.**
`pipes/whim116.sh`'s section 4 extracted `./zero-vim` from **every** `.build-whim/r*.tar`
and required one terminal table across all of them — a glob that reaches boundaries which
did not exist when the phase ran, so the first later phase to move the table on purpose
makes phase 116's check fail. Measured, with this phase's tar present: *q121 records a
different table:*, exit 1. Repairing another phase's program is a decision and not this
phase's to take, so it was reported; the fix bounds the loop by the phase's own number,
and the rule it states is **a phase may assert anything it likes about the past; it may
not assert that the future will not change what it measured.**

## Phase 122 (zero 39) — `-T {term}` goes, and the command line is `+{command}`

`pipes/whim122-edit.sh` and `pipes/whim122-check.sh`, `stage 122`, `package terminal`. Zero
phase 88 left argv as exactly two options: `+{command}`, which is how a **host** tells the
editor what to do, and `-T {term}`, which is how a **shell** told it what terminal it was
attached to. A core is told that by its host or not at all, and `-T` has had a replacement
inside the editor since before this pipeline began — `+set term=` reaches
`did_set_term()` and does everything `-T` did, which is why phase 116 rebuilt the terminal
harness on it rather than on `-T`. So the option goes, and after this phase
`command_line_scan()` is one `if (argv[0][0] == '+')` and one `else` answering
`mainerr(ME_UNKNOWN_OPTION)` — which is what every other word already answered.

### Six cuts, and five of them are dead code no sweep can see

gcc has no warning for a variable that is only ever FALSE, for a switch that has lost its
cases, for a statement after a `return`, or for a struct field whose name another struct
also uses — so all five go in the **edit**, which is `CLAUDE.md`'s rule and phase 121's
shape.

The option letters are **read out of** `command_line_scan()`'s first `switch (c)` rather
than written down. With no case left to set it, `want_argument` is FALSE for ever, so its
block goes and takes the argument switch, `parmp->term = argv[0]`, `ME_GARBAGE` and
`mainerr_arg_missing()` with it. The letter switch is then one `default:` and **is** its
body, which is a rewrite and not a change only because `mainerr()` does not return — read
off `mainerr()`, not assumed. The `-` arm and the last arm are then the same statement,
compared as **text** and collapsed into one else.

**The two `main_errors[]` rows go with their enumerators**, because the table is indexed
by them. `deadenums.py` would take the enumerator and leave the row, and the rows are
positional — `CLAUDE.md`'s `deadfields` lesson in a table — so the edit takes both,
renumbers `ME_EXTRA_CMD` 3 → 1, and deletes `mainerr_arg_missing()`, which cannot be left
for the sweep because it is `ME_ARG_MISSING`'s only other mention. Which enumerators die is
**computed** as those whose only remaining mention is their own `enum` line, and the check
is **DWARF and not the build**, in phase 88's shape: 1,189 enumerator values in, **1,187**
out, the two gone, `ME_EXTRA_CMD` lower by exactly the number of rows that went, and 1,186
unmoved.

**`mparm_T.term` goes in the edit for a reason phase 121 did not have.**
`tools/deadfields.py` matches by **name**, and this file holds 32 mentions of another
struct's `.term` member — `attr_entry`'s `ae_u.term` — so that tool can never see this one
dead. The edit **computes the partition**, every `.term` left belonging to `ae_u`, rather
than asserting it. `termcapinit()` then takes no name at all, and the compiled default it
substituted when given none, read out of the function, becomes its initialiser — which is
`pipes/whim96-edit.sh`'s `ui_write(console)` again.

### And then `set_termname()`'s no-screen arm cannot run, which is what phase 121 predicted

`set_termname()` has two call sites, partitioned by the edit: `termcapinit()`'s, before
there is a screen, which can no longer fail because the compiled default **is** a row of
`builtin_terminals[]`; and `did_set_term()`'s, at run time, where `starting` is
`NO_BUFFERS` or 0 — the edit reads **every assignment to `starting`** and requires none to
be `NO_SCREEN`. So the test folds always, the arm returns FAIL, and the three statements
after it — the fallback phase 121 repaired, `report_default_term()` and the option write
that recorded it — are unreachable and go, with `report_default_term()` falling to the
sweep as the phase's only sweep find.

**Measured, and the instrument is phase 121's**: with the same marker in the same place,
reached through `host_message()`, the input enters the fallback in exactly **2 of its 30
command rows** — `-T xterm` and `-T no-such-term-9x`, the only way in — and a control built
from this phase's **output** with the whole arm restored enters it in **none**, while
recording byte for byte what the output records.

### Phase 121's prediction was half right, and the other half is stated rather than quietly dropped

`report_term_error()` does **not** fall to the sweep. It is called **before** the
`starting != NO_SCREEN` test, so the run-time refusal still prints it — measured, `:set
term=vt320` on a pty prints it and then E522 on both binaries. What it said was `' not
known, defaulting to 'xterm-256color'`, and with no fallback left **that is a promise
nothing keeps**, so the clause naming it is cut in the same step, for phase 121's own
reason: nothing in the build checks that a message tells the truth. The name is read out
of the assignment the edit deletes.

### What rides along is `requested`, and not the 256-colour test

`set_termname()` kept the name it was **given** for one test, `musl_strstr(requested,
"256color")`, because the unknown-terminal path reassigned `term`; that path is what this
phase removes, so the only rewrite of `term` left is the `term += 8` that strips a
`builtin_` prefix, and a strstr cannot match inside that prefix because the needle begins
with a character the prefix does not contain — which the edit **checks** rather than
asserts. **The test itself does not fold and this phase declines to pretend it does**: the
two surviving terminal names disagree on it and `:set term=` still names either at run
time. Measured in both directions and kept in the check — with the name test forced TRUE
exactly **1 of the 19** terminal rows moves (`debug` gains `t_Co=256`), and with it forced
FALSE **18** do.

### The declared delta is four of thirty command lines, and the set is computed

A row must move **if and only if** one of its words is an option spelling a letter the
input's switch accepted and the output's does not:

```
  -T xterm             started and drew 2,117 bytes      -> exit 1, Unknown option
  -T no-such-term-9x   started and drew 2,117 bytes      -> exit 1, Unknown option
  -T                   Argument missing after: "-T"      -> exit 1, Unknown option
  -Txterm              Garbage after option argument     -> exit 1, Unknown option
```

The last two are the interesting half: **they were already errors and they moved anyway**,
because the two messages they gave were `ME_ARG_MISSING` and `ME_GARBAGE`, enumerators
whose last use was the `-T` argument block. A row that was already `Unknown option
argument` — every one of phase 88's leftovers — must **not** move, and the other 26 do not.
The 102 screen cases, the 98 Ex-command rows, the four pty scenarios and the 19 terminal
rows are the input's byte for byte: `term-moved` is phase 121's and cumulative, and this
phase's recording of the table is the input's.

### And the phase decomposes, which is also the instrument shown able to fail

The **input** with only the `case` arm deleted — `want_argument`, the argument switch,
both `ME_*` rows, the field, the unreachable fallback and `requested` all left in place —
records byte for byte what this phase's output records, and differs from the input. **So
the option letter is the whole of what moves behaviour**, and the other five cuts are
invisible to every part of a recording.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,668 | **79,589 (−79)** |
| the command line | `+{command}` and `-T {term}` | **`+{command}`**, every other word `ME_UNKNOWN_OPTION` |
| `main_errors[]` rows / DWARF enumerators | | two rows and two enumerators go; 1,189 → **1,187**, 1,186 unmoved |
| the cut at the first `#include` | 77,758 | **77,679**, eleven directives with none above them |
| the boundary | 18 names | **18**, unchanged |
| `nm -u` | 17 | **17**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| binary | 781,096 | **781,064** |
| records that moved | | **4 of 30 command lines**, and nothing else |

### Its placement

`stage 122`, `package terminal`, and `streams` had a real claim on it that the manifest
answers: the phase removes the half of the command line phase 88 kept, in the same parser —
but of the six cuts, one is that parser arm and **the other five are all terminal code**.
The sentence the package makes reads straight on: phase 85 removed the **question** the core
asked about the terminal, 38 removed the **vocabulary** it could describe one with, and 39
removes the **telling** — after it nothing outside the process can say what terminal this
is, and `+set term=` inside the editor is the only way. Four `uses`: `seed:83`;
`harness:86`, which put an argv record in a zero recording at all; `streams:88`, whose
leftover this phase's whole subject is, and whose renumbered `main_errors[]` table it
checks against DWARF in that phase's own shape; and `host:104`, whose `host_message()` is
the instrument again.

**`apart 121 122`, one direction, both halves measured in the same run.** Phase 121's check
builds its first control by finding the fallback in `set_termname()`, and phase 122 deletes
the fallback outright, so `tools/phaserun.sh whim 121-122` on q120 stops in phase 121's check
at its **first act** with *set_termname() names 0 of the surviving rows and this check
needs one — the fallback*. **That is a check depending on the code it repaired still being
there**, which is the sharpest form of `apart 105 106`'s lesson: phase 121's repair is phase
122's dead code. The other direction was run rather than reasoned — phase 122's check passes
on that stage's tree, which is byte-identical to the sequential one. No `need 122`, measured
in the same run on phase 121's unswept output.

And `pipes/whim116.sh`, the one phase program that reads other boundaries, was run in the
repository root with this phase's tar present: *all 34 recorded boundary binaries up to q116
record the SAME table*, because its scan has been bounded by its own number since the fix
phase 121 asked for, and this phase changes no terminal row at all.

### What zero-vim is after phase 122

```
zero-vim.c        79,589 lines          from whim-vim.c's 86,614  (-7,025, 8.1%)
                  77,678 above the boundary, 1,911 below it
functions         1,757
type definitions  906
DWARF enumerators 1,187
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
built-in terminals 2 of whim's 10: xterm-256color and debug
#include          11, at line 77,680, and NOT ONE DIRECTIVE above them
core -> host      18 names: vim_snprintf, host_exit, host_message, host_time,
                  host_alloc, host_free, host_write, host_raise, ten musl_*
libc prototypes   0 -- the core names no libc function at all
libc symbols      17 with zero's flags, 18 as tools/symbols.sh counts
binary            781,064 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    term-moved at 38 and four command lines at 39, the first zero has
                  declared since phase 94 -- with 2 stderr-moved and the records of
                  87 to 94 before them
make editor.c     77,678 lines: 0 directives, 0 errors, 18 warnings, all of them
                  `used but never defined` and all of them the interface
```

**Seventeen phases in a row had declared nothing — 104 through 120 — and phase 121 ended the
run.** What stood in for a recording through those seventeen is a taxonomy and not a
tally, and this document **names** the kinds rather than counting them, because an ordinal
written into a phase section is frozen on the day it is written while the list keeps
growing: phase 101's section says *a sixth kind* over one merge of the list and phase 109's
says *a sixth kind* over another, and both were true when written. **`CLAUDE.md` carries
the one numbered list**, keyed to the order the kinds first appear in `pipes/zero.delta`,
and where an ordinal here and an ordinal there disagree, that one is the current one. The
kinds, by name:

* **code that could not run** (92, 100) — an instrumented pair reaching it 0 times;
* **code that runs and the instrument cannot see** (85, 95, 103, 105, 111, 113, 119) — the phase
  owes probes of its own;
* **a possibility removed** (13) — nothing had ever opened those `FILE *` in any build;
* **the binary is the same bytes** (99, 106, 107, 120) — tier 1, and the strongest, with a
  control that moves it to keep a `cmp` from being two numbers agreeing;
* **replacement code that computes the same answers** (97, 98) — the weakest, and their own
  checks argue it;
* **the code runs and the instrument sees it do the same thing** (101, 102, 104, 108, 115, 117,
  118) — strong exactly when what moved is on the path of everything;
* **part `cmp` and part recording, separated rather than averaged** (26);
* **the source is the same lines rearranged** (27) — a multiset equality, tier 1 one level
  up;
* **the behaviour really moved and the corpus cannot reach it** (29);
* **an accounting** (31) — 42 bytes of `.text`, for a phase that frees no symbol because
  there was none to free;
* **no source changed at all** (86, 116) — what has to be argued is that the **comparison**
  moved safely.

**Fifteen of the seventeen libc symbols are called from the host and from nowhere else**,
and since phase 119 **so are the other two**: `getpid` and `kill` are `host_raise()`'s and
`musl_suspend()`'s, and the core's whole vocabulary of the seventeen is three English
words inside string literals — the two `NGETTEXT` strings in `op_shift()` that say *time*,
and `E222`'s *"already read from"*, measured on this file.

## Phase 123 (zero 40) — the instrument could not see the text layer

`pipes/whim123.sh` — one file, like phases 83, 84, 86 and 116 — `stage 123`, `package harness
3 33 40`. It changes no source at all: q123's `zero-vim.c` is q122's byte for byte and its
boundary digest is its input's, `68e450fd6912` either side. What it adds is the **sixth
part of a recording**, `tools/zmemline.py`, and it is here for the reason phase 86 and
phase 116 were: **a harness that cannot see a phase must be fixed before the phase, never
after.** Phases 124 to 128 rewrite the memline, and until this phase ran nothing in the
pipeline could have told a working one from a broken one.

**One thing about every number from here on.** Between phase 122 and this phase the
`keymodel=startsel` repair landed in slim Phase 1, three lines in `set_init_1()`, and all
three products were re-passed from it — so every zero boundary gained three lines and
`q122` is **79,592** where the phase 122 section above says 79,589. Every figure in this
section and the five below it is post-repair, and every figure above it is pre-repair;
the difference is those three lines and nothing else. `CLAUDE.md` records what the repair
was and what it falsified.

### The blindness is measured and not suspected

A `zero-vim` with one line deleted from `ml_find_line()`'s `ML_DELETE` arm —
`pp->pb_pointer[idx].pe_line_count--;`, the statement that keeps a pointer entry's idea
of how many lines hang under it — records **all 102 screen cases byte for byte**. That
binary was built and both corpora run on it before this phase was written: 0 of 102
differ. **Forty phases had been verified by an instrument that could not see a corrupted
text layer at all.**

The reason, confirmed with an instrumented q122 rather than reasoned: every one of the
102 cases allocates **exactly one data block**. The root pointer block holds one entry
for the whole session, so `idx` is 0 every time, `ml_find_line()` never chooses among
entries, and `pe_line_count` is never the number that decides anything. A 4,096-byte
page holds 78 lines of the width these cases type and `pb_count_max` was 127, so a root
cannot split before **9,984 lines** — and the largest case in the 102 is nowhere near
it. All seven of the phase's markers fire in **0 of 102**.

### The corpus is sixteen cases, and its depth is measured rather than intended

`tools/zmemline.py` is sixteen cases building buffers of **200 to 25,000 lines**, and
they are built **in the editor**: there is no file argument (phase 88), no `:edit` (phase
91) and no `:read` (phase 90), so a case types one line under `'paste'` and replays a
three-key macro with a count — five keystrokes and about two seconds for 25,000 lines.
Every line begins with its own number, so a screen drawn with `'number'` shows the
tree's answer beside the question; each case churns the **middle** of the buffer and
then reads the whole of it back with a substitute count, and jumps to named lines and to
both ends.

**The block arithmetic is derived and not written down**, by compiling the struct
definitions out of the source the phase was handed — page 4,096, data header 24, index
entry 4, `PTR_EN` 32, so a 47-byte line packs 78 to a block and `pb_count_max` is 127 —
and the corpus's own sizes are then required to clear those thresholds. **A corpus that
means to reach a root split and does not is exactly the defect this phase exists to
end**, so the depth is a measurement:

| marker | 16 memline cases | 102 screen cases |
| --- | --- | --- |
| a data block splits | **16** | 0 |
| a pointer entry chosen at `idx > 0` | **16** | 0 |
| a data block of more than one page | **3** | 0 |
| a pointer block full | **4** | 0 |
| **the root splits** | **4** | 0 |
| a non-root pointer block splits | **4** | 0 |
| `ml_find_line()` descends through a second pointer block | **4** | 0 |

### Five controls, and the one that moves nothing is what makes the others mean something

Four are corruptions and each moves **0 of the 102**, which is the premise restated as a
measurement. Deleting `pe_line_count--` from the descent moves **7 of 16**; deleting
`ml_lineadd()`'s **deferred** adjustment — the other place a pointer entry's count is
maintained, and a different shape of mistake — moves **16 of 16**; widening
`ml_find_line()`'s `ML_FIND` stack by one line moves **4 of 16**, and they are exactly
the four the probe measured descending past one pointer block, which the check states as
a **rule** and not as a list of names.

The fifth quarters `pb_count_max` and moves **0 of 16** while the probe shows the root
split going from 4 cases to 13. That is not a failure: **the corpus records behaviour
and not tree shape**, and the control proves it is not a no-op by the probe rather than
by assertion. It is the same finding phase 127 meets again from the other side when it
has to choose `DB_LINE_MAX`, and phase 128 turns into a `static_assert`.

**A discarded control is reported rather than dropped.** Corrupting the root entry at the
split site moves nothing, because the next iteration overwrites it — and a control the
code repairs is not a control.

### It found a real defect, and it is not the one the scrub was written for

One record's stream digest moved in **1 of 48** whole recordings with every screen
identical. Phase 115's section above has the corrected account: the cause is not the
timestamp's text, which `tools/zrec.py` already rewrites padded, but **arithmetic on its
length**. An undo in a buffer this size reports its age and the editor then positions the
cursor to clear the rest of the line, so `0 seconds ago` emits `\033[24;40H\033[K` and
`1 second ago` emits `\033[24;39H\033[K`. A column derived from a scrubbed string's width
leaks the thing the scrub exists to hide, and it failed phase 99, whose binary is
byte-identical either side — which is the only reason it was catchable.

So **a memline record carries `stream N redraws` and no digest**, N being the count of
`\x1b[?25h`. What replaces the digest as evidence is the fifth control above's sibling:
both clocks the core can read replaced by counters that run away from the wall move **0
of 16 memline records against 9 of the 102 screen cases**. That is stronger than a
digest, because it says the record does not depend on the clock **at all** rather than
that two runs of it happened to agree. `tools/zcases.py` still digests the raw stream,
and closing that is expensive rather than difficult — see *What is still open* below.

### Four phases pinned the size of a recording, and had to stop

A new **part** of a recording is a new file whatever shape it takes, so `zpty.py`'s
precedent — one record however many scenarios it holds — could not be followed. Measured:
phase 92 stops with *a recording is 122 files, not the 106 this phase counted*. Phases 92,
96, 113 and 117 each tested the count as an **equality**; phases 108, 118, 119 and 120 tested
it as a floor of 100 and were unaffected. The four equalities are now **computed** —
phase 92's control must mark *total − 2*, quiet only in `ref-pty.txt` and `ref-term.txt`,
and the other three take the floor of 100 their siblings already use — which is
`CLAUDE.md`'s own rule that **a number a phase cannot move is reported and not pinned**.

### Measured

| | before | after |
| --- | --- | --- |
| `zero-vim.c` | 79,592 | **79,592**, byte for byte |
| the boundary digest | `68e450fd6912` | **`68e450fd6912`** |
| a recording | 106 records, five parts | **122 records, six parts** |
| memline cases | — | **16**, 200 to 25,000 lines |
| the seven markers, memline | — | 16 / 16 / 3 / 4 / **4** / 4 / 4 |
| the seven markers, screen | 0 of 102 | **0 of 102** |
| `make whim-verify` | | **41 of 41**, 2,771 s of phases in 129 s |
| keys moved | | **58 zero**, and not one whim or slim key |

A cold `make whim-repass` from an empty cache reproduces all 40 recorded boundaries and
adds q123. The 58 are all 40 zero units and 18 zero edits; `tools/zmemline.py`,
`tools/zrecord.sh` and `tools/zcompare.py` are named by no whim or slim phase, and that
is the measurement rather than the claim, so core rule 9's gate does not apply.

### Its placement

`stage 123`, `package harness 3 33 40`, which is *the phase changed no source and moved
the instrument instead*. **There is no `apart 122 123` and no `need 123`**, and both are
refusals rather than omissions: `pipes/whim123.sh` is a whole-phase program, so
`stage 122-123` is answered by `tools/stages.sh` with *phase 123 is in stage 122-123 but is
not an edit and a check* and by `tools/phaserun.sh` with *phase 123 has no edit and
check to run in stage 122-123*, both measured — and `need` is a statement about an edit
part, which this phase has none of. That is `apart 116 117`'s shape exactly.

The declared delta is **nothing at all**, and it is phase 86's and phase 116's kind: the
phase changes no source, so nothing about the editor's behaviour *can* have moved, and
what it has to argue is that the **comparison** moved safely. It does that by requiring
every recorded boundary back.

### What is still open, stated as two things and not one

**The corpus can be made to reach further and the fix cannot land yet.** Branch
`zmemline-fix`, commit `4e9fb9c`: `tools/zmemline.py` derives its case sizes from the
block arithmetic instead of carrying them as constants, and chunks the buffer build.
Measured, it reaches a **root split in 4 of 16 cases** both at the real fanout and at a
forced `PB_COUNT_MAX = 511` — the value an 8-byte `PTR_EN` would give — where the corpus
as committed reaches **1 and 0**. It is blocked because **two merged checks assert the
corpus's insufficiency as a requirement**: `pipes/whim125-check.sh:686` requires
root-split coverage to *decrease* under phase 125's wider pointer block, and
`pipes/whim126-check.sh:557` requires identical tree-event tuples across a fanout change.
Both pass today **only because the instrument is too small to see otherwise**, so
landing the better corpus means rewriting two checks that were correct when they were
written. That is a phase's worth of work and is recorded here rather than done quietly.

**And the `ago` leak is still open in `tools/zcases.py`.** The fix that works is this
phase's — count redraws, and let a clock control carry the evidence — and applying it to
the other 106 records is expensive rather than hard: the `--- stream` line is named in
**eighty files**, forty-seven times in phase 95's check alone. It deserves a pass of
its own. Phase 115's section states the hazard and phase 99 is where it struck.

## Phase 124 (zero 41) — freeing is free, and the arena is measured

`pipes/whim124-edit.sh` and `pipes/whim124-check.sh`, `stage 124`, `package host`.
`host_alloc()` becomes a **bump allocator** into a fixed 1 GiB arena and `host_free()`
returns without doing anything. That is the charter bullet *A GARBAGE COLLECTOR IS
ASSUMED FROM HERE ON* built, and it is what makes the four phases after it cheap rather
than clever: the core may now allocate a record per line and simply not free it.

**The claim is "freeing is now free" and not "the core stopped freeing".** Every
`host_free()` call the core makes is still there and still made; a later phase may delete
them, which is the reason for doing this one first. Phase 118 moved `malloc` and `free`
across the line and wrote the two wrappers; this changes what is behind the two names and
nothing else.

### The strongest thing it says is a `cmp`, and it is a `cmp` of the core

The phase is host-only from end to end, so `make editor.c` — **77,681 lines, 2,064,232
bytes** — is **byte-identical in and out**. That subsumes every screen case, every
memline case, every Ex command, every command line and every pty scenario at once *for
the part of the file the project is for*, because the program a port would be handed is
literally the same text. It is the cleanest proof a phase is host-only that this pipeline
has, and it is a tier-1 check one level in from the whole binary. The recording is then
what says the **host** still answers the same way.

### The size could not have been measured one boundary earlier

The input built with a counter on `host_alloc()` that totals every request as the
allocator rounds it, dumped from `host_exit()`, over the whole of `tools/zrecord.sh`:
**268 sessions in 122 records, largest 200,458,672 bytes**, two recordings and the same
number. It is **one case**, phase 123's `mem_deep_jumps`, a 25,000-line buffer churned in
the middle; the next three memline cases are near 52 million, and the heaviest of the
**102 screen cases is 1,722,512**, which is **115 times less**.

So an arena sized from the 102 alone would have been sized from a corpus provably unable
to reach the text layer — the defect phase 123 exists to have ended, arriving one phase
later in a shape nobody predicted. **This phase found that out the hard way and the
record says so**: 64 MiB was written first and the recording refused it, *THE RECORDING
MOVED, in 1 of 122 records: memline/mem_deep_jumps*, the case dying with
`host arena exhausted: 67108864 bytes, 67058640 used, request 60263`, with 0 of 102
screen cases and 0 of the four sweeps moving.

1 GiB is **5.36 times** the measured high-water and 18.67 % used at its worst. The check
re-measures the high-water on its **own** output over both corpora every run and refuses
an arena less than four times it — and refuses as well if the heaviest memline case is
not heavier than the heaviest screen case, which is the lesson above written down as an
assertion rather than as a paragraph. So the size is a checked property and not a
remembered one, and a later phase that makes the memline allocate more is told by its own
check instead of by a crash.

### The phase overturns its own earlier reasoning, and the correction is worth more than the number

It first justified 64 MiB by *the abort path costs the arena times the harness's
concurrency* — "64 MiB across 64 threads is 4 GiB on a 62 GiB machine". **That is false
except for a runaway session**: an ordinary session's resident memory is its traffic,
which the arena does not change, and an untouched arena page costs nothing. Measured, the
same source at 64 MiB and at 1 GiB gives a **byte-identical image**, 772,872 either way,
because `.bss` is `NOBITS`. The size buys one thing, how far a runaway goes before it
dies loudly, and costs one thing, address space. The wrong argument and the measurement
that killed it are both in `pipes/zero.delta`, so the correction survives outside the git
log.

### And one measurement says why no number is safe

With nothing freed an arena holds a session's whole allocation **traffic** and not its
live data, and this editor's traffic is **quadratic in the length of a single line being
typed**: `+normal 200000ax` asks for 20,013,114,624 bytes across 400,475 calls, and
`+normal 500000ax` for 44,075,360,179. Phase 118's own by-hand probe was that command, so
a later phase that writes one like it will hit the wall. Buffers are linear and cheap by
comparison — 100,000 lines cost 12,862,224 bytes and 300,000 lines 35,157,264, about 112
a line. **It is churn and not size that fills an arena.**

### The real cost is not the arena, it is the resident memory

A bump allocator makes a session's peak RSS equal to its traffic. Measured with
`getrusage(RUSAGE_CHILDREN)` over a whole `tools/zmemline.py` run: the worst child peaks
at **13.6 MiB on the input and 191.8 MiB here**, fourteen times more, at either arena
size. Across a whole `make whim-verify` — 42 units at once — the peak is **13.7 GiB of a
62 GiB machine against 13.1 GiB** measured the same way on the boundary before it: 591
MiB and 4.4 % more, with 41 GiB still available. That is the charter's trade under
harness concurrency, on the record for the phases behind it, and it is the number to
watch as phases 125 to 128 change how much the memline allocates.

### Two of the four rewrites are not in the allocator, and without them the phase is wrong

The formatter's private island — the functions phase 110 moved below the includes because
they need `va_list` — still called libc's `free()` and libc's `realloc()` directly, on
pointers that came from `alloc_clear()`, which is to say from `host_alloc`. Phase 118 named
one in its own program (*"and `format_overflow_error()` below the boundary"*) and phase 117
named the other (*"`realloc` call is the host's and is not this phase's"*). **Both were
right while `host_alloc` WAS `malloc`**: the two allocators were one allocator. From this
phase a `free()` or a `realloc()` of an arena pointer is undefined, so they move — the
`realloc` by phase 117's own allocate-copy-free with phase 117's three traps read off this
site — and the byte-identical cut is what proves the phase did it without touching a core
line.

Neither is reachable by any recording and one cannot run at all, so the check owes a
probe and runs one: `adjust_types()` grows `*ap_types` only for a format string carrying
a **positional** spec, and not one string literal in this file has one, so the same
driver built into the input and the output runs six ascending positional formats through
it, enters the grow arm **17 times in each**, and the two binaries print the same bytes.
`format_overflow_error()` cannot be probed because it cannot run — its guard is
`overflow_err`, which is `tvs != nullptr`, and `vim_vsnprintf_typval()` has one caller in
this file passing `nullptr`. That is phase 92's and phase 100's kind, and it is kept correct
rather than left to rot.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,592 | **79,660 (+68)**, every one below the first `#include` |
| `make editor.c` | 77,681 | **77,681**, byte-identical, 2,064,232 bytes |
| `nm -u` | 17 | **14**, the set moving by exactly `free malloc realloc`, a `comm` empty the other way |
| `.bss` | 24,600 | **1,073,766,424** |
| binary | 781,064 | **772,872 — smaller**, because `.bss` is `NOBITS` and musl's allocator left the link |
| arena high-water | | **200,458,672 bytes**, one case, 5.36x under the ceiling |
| worst child RSS | 13.6 MiB | **191.8 MiB** |
| records that moved | | **0 of 122**, two full recordings byte-identical |

It is the **first zero phase since 28 to free a symbol, and it frees three**. The `.bss`
growth is the arena less 960 bytes, and the 960 is musl's own `__malloc_context` and five
smaller objects leaving with it. `EXEC`, no `INTERP`, no dynamic section and no
relocation are all still true of a gigabyte object.

**The controls, over both corpora.** `host_alloc` returning `nullptr` moves **122 of
122**. **The offset never advancing** moves 102 of 102 screen cases and 16 of 16 memline
cases, and it is the only one that tests the *allocator* rather than the wrapper: a
`host_alloc` that returned the arena's base for ever would pass the symbol check, the cut
and the size assertion. `host_free` doing nothing moves **0 of 102 and 0 of 16** — phase
118's own `cf` control re-run on this phase's input rather than a new claim, on a corpus
phase 118 did not have, and reported rather than hidden, because a leak is invisible to
this corpus too and it is `free` leaving `nm -u` that says the freeing changed. And the
guard, which no recording can take: a 256 KiB arena aborts with
`host arena exhausted: 262144 bytes, 113024 used, request 319968` and exits 1 — the
request being the screen, this editor's single largest allocation — while the identical
session on the real output is silent and exits 0.

**`<stdlib.h>` is now dead and it stays**, measured rather than argued: the output built
with the directive deleted is byte-identical, 772,872 either way. Eleven stays eleven,
on phase 96's precedent for declining — a phase that changes two things cannot say which
one a difference came from, and the removal is free for whoever asks for it.

### Its placement

`stage 124`, `package host 17 18 19 20 21 30 32 35 36 41`, because that is what the
package is: a thing the core did for itself becomes a thing it asks the host to do. Here
it is one step further — the core already asked, and what changes is the answer.
`tools/zhostonly.py` needed no new word and no new exception: `host_alloc` and `host_free`
have sat below `musl_suspend()`'s brace since phase 118, and none of `malloc`, `free`,
`realloc`, `max_align_t` or `alignof` is in its vocabulary.

**`apart 124 125` is measured and is not the mechanism the phase predicted.** It expected
`apart 97 98`'s shape, an undefined-set equality against a stage's one snapshot. What
actually fires is **this phase's own promise**: `tools/phaserun.sh whim 124-125` on q123
stops with *the text above the first `#include` is not byte-identical in and out, and
this phase is entirely below it*, and again on the directives' line numbers. **A phase
that promises to touch no core line cannot share a stage with one that deletes 366 of
them.** The other direction is reasoning: phase 125's check compares the undefined set of
the text its own edit was handed with its output, and on a shared stage phase 124's edit
has already taken the three symbols by then, so it would pass.

**There is no `need 125`, for a reason stronger than one stage's measurement**: phase 124's
**sweep is a no-op** — its edit's output on q123 is byte-identical to q124 — so there is no
unswept text for phase 125 to be handed at all. `make whim-verify` is 42 of 42.

## Phase 125 (zero 42) — the swap file's residue, and what no sweep could find

`pipes/whim125-edit.sh` and `pipes/whim125-check.sh`, `stage 125`, `package tidy 13 37 42`.
The filesystem went at phases 89 to 93 and the swap file's **bookkeeping** did not:
memline and memfile still kept a header block carrying the editor's version and the
buffer's name, a translation table for blocks not yet written out, a three-valued
dirtiness state, and a record of where each block's lines used to be. None of it can be
reached, none of it is read — and **not one of the four is visible to any tool in
`tools/`, because every one of them is written.**

`tools/deadfields.py` takes a field named nowhere outside its own type, and each of these
is named; run against the input it reports **0 fields**. gcc has no warning for a struct
member nothing reads, for an enumerator only ever OR-ed into a word nothing tests, or for
a file-scope object read twice and assigned nowhere;
`-Wunused-but-set-variable` does not reach a file-scope object and
`tools/deadsweep.py` does not act on it at all. So this is an **edit** and not a sweep,
and the check states the division rather than assuming it: **22 names leave in the edit
and 39 more in the sweep**, both sets named with the reason each is in the half it is in.

### The four, each proved as a partition over every mention

* **`struct block0`, the header.** **Eight** fields, not the survey's nine — phase 119
  already took `b0_pid`, so the edit reads the list **out of the struct** rather than
  from a list typed into it, which was already a phase out of date. 8 declarations, 12
  writes, **0 reads**. `ml_open()`'s thirty-two-line preamble goes with
  `set_b0_fname()`, `long_to_char()`, `ml_setflags()` and its two call sites, and the two
  surviving blocks move down by one: the pointer block is block nr 0 and the data block
  block nr 1.
* **Negative block numbers.** `mf_trans_add()` returns before doing anything unless a
  block number is negative, and the chain that could make one is **computed** in the
  edit: `mf_new()`'s callers pass `FALSE` or `ml_new_data()`'s own parameter,
  `ml_new_data()`'s pass `FALSE` or `flags & ML_APPEND_NEW`, `ML_APPEND_NEW` comes only
  from `ml_append()`'s `newfile`, and `newfile` is `FALSE` at **all eleven call sites**.
  `mf_trans_add`, `mf_trans_del`, three memfile fields, the parameter and two flags.
* **The dirtiness, write-only in all three layers.** `mf_dirty` has six writes and two
  reads and **each read is the condition of an `if` whose only statement writes the field
  again**, which the edit checks structurally; `bh_flags` is read in exactly one place
  and that read tests `BH_LOCKED`, so `BH_DIRTY` is set three times and **tested
  nowhere**; and `ML_LOCKED_DIRTY` and `ML_LOCKED_POS` are read at one place between
  them, the two arguments `mf_put()` stops taking. `mf_put()` is now `mf_put(bhdr_T *hp)`.
* **`pe_old_lnum`, 7 writes and 0 reads — and the three locals that go with it.** Taking
  the field leaves `lnum_left`, `lnum_right` and `ml_find_line()`'s `dirty` written and
  never read, which is `-Wunused-but-set-variable`, which `tools/deadsweep.py` does not
  act on, so they are the edit's for the same reason the fields are. And
  `mf_dont_release`, `static int mf_dont_release = FALSE;`, read twice and **assigned
  nowhere in the file**.

**A fifth the survey missed: `ML_LOCKED_DIRTY`'s ml-level twin.** `ML_LOCKED_DIRTY` is
set 8 times, cleared once and tested nowhere once `mf_put()` loses its state arguments —
and no warning covers a bit in a struct field. Leaving it would have created exactly the
invisible write-only state this phase exists to remove.

### And `BH_LOCKED` is not dead, which is the distinction worth keeping

It looks like `BH_DIRTY`'s twin and it **is** read, by `mf_put()`'s own
`e_block_was_not_locked` test. Measured: a binary whose `mf_put()` **sets** the bit
instead of clearing it draws all 102 screen cases identically, because the only reader is
an internal-error test that then never fires. **That is unreachable evidence, not
unreachable code**, and the phase leaves it alone and says so.

### The declared delta is nothing at all, and it is two kinds at once

The negative-block half is **phase 92's kind**, code that could not run; the block-zero
half is **phase 95's**, code that runs everywhere and the instrument cannot see. One
instrumented build of the input says both, over **252 records** — 102 screen, 16 memline,
126 from the two sweeps and eight stress sessions: the four markers on the negative-block
island fire in **0 of 252**, against a control of identical shape in `ml_new_data()`
firing in **227 of them, 2,141 times**, and the three markers on the header writes fire
in **227 / 214 / 227**, so that code runs nearly everywhere and the two full recordings
are byte-identical anyway.

**The check caught itself failing, and that is reported rather than smoothed away.** An
earlier draft computed each marker's indent from its anchor, which put two counters
**outside** the `if` they belonged in, and it refused with *`neg_new neg_find` fired in a
recorded session* — reporting the unreachable island as reachable. Every marker's
placement is now written out in full, with that measurement as the reason.

### The fanout changes, and that is what phase 123 is for

`pe_old_lnum` is a member of `PTR_EN`, so every pointer-block entry gets smaller and more
of them fit in a page: `sizeof(PTR_EN)` 32 → 24 and `pb_count_max` **127 → 170**, and the
root pointer block overflows **later**. A binary with `ml_append_int()`'s root test left
at the old block number — a real bug, the root not kept where `ml_find_line()` starts —
draws all 102 screen cases and all four of the survey's own deep cases **identically**;
what moves it is `mem_deep_jumps`, and that corpus is the only recorded thing that can.

**It also narrows what phase 123 reaches, measured 4 cases to 1** — `mem_root_split` among
them, the case named for the thing it no longer does — because phase 123 derived its
buffer sizes from `sizeof(PTR_EN)`. **A case named for the root split is a case sized for
a fanout.** The check asserts that as an *inequality* rather than a count, because this
phase can only make a pointer block hold more children. The corpus fix that would undo
the narrowing exists and cannot land; phase 123's section says why.

Section 8 of the check is the direct proof with an instrument: at sixty thousand lines
the output preserves the root once and never overflows, the control preserves it never
and reaches `e_updated_too_many_blocks`, and the two draw different screens. Four sessions
of sixty thousand lines and more then agree between the input and the output. Undo's
message carries a wall clock — four of five runs said `0 seconds ago` and one said
`1 second ago`, a difference between a binary and **itself** — so that phrase is folded to
a constant, and the check requires it present so the folding cannot hide anything.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,660 | **79,294 (−366)** — the edit takes 280 and the sweep 86 |
| names that leave | | **22 in the edit, 39 in the sweep**, both sets named |
| `sizeof(PTR_EN)` / `pb_count_max` | 32 / 127 | **24 / 170** |
| `make editor.c` | 77,681 | **77,315**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, an equality: a phase that deletes core code and crosses no boundary can free nothing |
| binary | 772,872 | **768,744** |
| arena high-water | 200,458,672 | **200,449,792 — lower**, every memline case allocating less |
| records that moved | | **0 of 122** |

Every one of the sixteen memline cases allocates less, smaller structs outweighing the
free-list reuse that goes; phase 124's instrument reproduces its own published figure to
the byte on q124, which is what makes the q125 number trustworthy. `tools/canon.sh` is a
no-op on the output, and `zhostonly`, `orphanopts`, `nvidxcheck` and `phasecheck` all
pass unchanged.

### Its placement

`stage 125`, `package tidy 13 37 42`, which is *leftovers of cuts already made* — phase
96's `FILE *` that had never been opened and phase 120's unions that unite nothing are the
same shape as a swap file's header in an editor that has had no swap file since phase 89.
It is not `memline`, which is phases 126 to 128 and is about **representation**.

`apart 124 125` is phase 124's, above. **There is no `need 125`** — phase 124's sweep is a
no-op, so there is no unswept text to be handed — and the 41-42 run confirms it end to
end anyway: the stage's one sweep leaves a `zero-vim.c` byte-identical to q125.

The check is proven able to fail three ways, two of them while it was being written: the
indent draft above; the output with `ml_find_line()`'s descent put back to block nr 1
refuses at the block numbers; and the edit run on its own output refuses at
`struct block0`. `make whim-verify` is 43 of 43.

## Phase 126 (zero 43) — a block number becomes a reference

`pipes/whim126-edit.sh` and `pipes/whim126-check.sh`, `stage 126`, `package memline 43 44
45`. `pe_bnum` and `ip_bnum` become `bhdr_T *`, `memline_T` gains `ml_root`, and
`mf_get(mfp, nr, page_count)` becomes `mf_get(mfp, hp)`. The hash table that turned an
integer block number into a page then has nothing left to look up, so it goes — with the
free list it was keyed alongside, with `mf_blocknr_max` that handed the numbers out, and
with `pe_page_count`, whose one reader in the whole file was the argument `mf_get()` no
longer takes. `blocknr_T` 13 → 0, `mf_hashitem_T` 18 → 0, `mf_hashtab_T` 14 → 0, eleven
functions, 180 lines.

**This is the phase that buys the port the most, and it is worth being precise about what
it buys.** An integer key into a side hash table becomes an **object reference**, which is
the one thing a JVM has and C does not make you say. `WHIM-PLAN.md` §II.4d lists what a port
would have to be told about rather than translate, and the memline page is the whole of
that list; this removes the **outer** half of it, the indirection *between* pages. It does
**not** remove the inner half, and the phase says so rather than letting the headline
stand: the page is still a byte array, `db_index[1]` is still declared length 1 and
indexed to the block's line count, the fourteen `(char_u *)dp + start` interior pointers
are untouched, the top bit of an offset is still a flag, and the arena and its interior
pointers survive until phase 127. **A reference to a block whose innards are still a byte
array is halfway.**

### Why the lookup could not miss, which is the whole argument that a pointer is the same answer

Read off the text by the edit rather than asserted: the hash is inserted into by
`mf_new()` and `mf_get()` and removed from by `mf_free()` and `mf_get()`, which removes
and re-inserts in one breath to move a block to the head of the used list — so **every
live block has been in the hash since it was made**. Nothing has been written to a disk
since phase 89 and nothing could be read from one since phase 92, so there has never
been a block number in this build that named a page not already in memory.

No invariant broke, and both were looked for rather than assumed. **No block number is
stored anywhere else** — every mention of `pe_bnum`, `ip_bnum`, `mhi_key` and
`bh_hashitem` is partitioned by its owning function. **The hash provided no ordering
anything reads**: `mf_used_last` is write-only, there is no release path left, and one
`ml_root` suffices because a root split **preserves the root block's identity**.

### Four write-only fields go in the edit and not the sweep, and that is the rule rather than a choice

`tools/deadfields.py` takes a field named nowhere outside its own type and reports **0
fields** in this region, because every one of `mf_used_last`, `bh_page_count`,
`pe_page_count` and `pe_bnum` is **written**; gcc has no warning for a struct member in
either direction; and phase 103's trap is the other half — remove a member and leave its
initialiser and the compile says `excess elements in struct initializer`, which is a
correct phase failing. `mf_used_last` had been write-only since phase 125 took
`ml_setflags()`, its last reader. `bh_page_count` and `pe_page_count` become write-only
**here**, and that cascade was measured rather than predicted: with `pe_page_count`'s one
read gone, gcc reports `page_count_left` and `page_count_right` as
`-Wunused-but-set-variable`, which `tools/deadsweep.py` does not act on, so those two
locals are the edit's as well.

**Every assertion is a partition and not a count.** Phase 118 had to repair phase 117's
counted anchors, and phase 125 is this phase's direct predecessor and removes four fields
from the same two structs. So the edit asserts the **set of functions** that says each
name — `mhi_key` in eight places, `pe_bnum` in four, `ip_bnum` in five — and a name said
somewhere the phase does not account for refuses. The check states the difference the same
way: the names that leave in the edit (**47**), the names that leave in the sweep (**2**)
and the names that **arrive** (**6**) are three computed sets compared against three
written ones, over identifiers with string and character literals masked out first.

### It caught a lie the prototype would have shipped

`E323: Line count wrong in block %ld` is the only message in the file that printed a
block number, and there are none left. The survey's prototype passed `(long)0`, which
would have printed a falsehood for ever; the message becomes **`E323: Line count wrong in
block`**. The evidence is phase 92's shape: the input built with a latching marker on that
arm carries it in **0 of 122 records** and the identical marker one line above — the
descent into a pointer block — in **120 of 122**; then both sources are forced to take the
arm and both do draw it, `...in block 0` against `...in block`.

**How the arm is forced was arrived at by measurement, and two wrong ways are recorded
because each looks right.** Emptying the scan loop leaves `idx` at 0, `0 >= pb_count` is
false and the descent simply goes round again. Forcing the arm alone is not enough either:
`ml_find_line()` then returns `nullptr`, the editor dies of it — SIGSEGV, measured — and
the message never reaches the stream. What works is reading every entry's line count as 0,
which leaves `idx` at `pb_count`, and then making the arm draw and stop with `out_flush()`
and `host_exit(0)` right after the `iemsg`. That last edit is found by the **shape** of the
call and not its text, which is what lets one rule serve two sources that spell it
differently: the input formats a block number into `IObuff` and the output does not.

`E298: Didn't get block nr 0?` and `E298: Didn't get block nr 1?` are not changed but
**deleted**, with the two `ml_open()` tests that were the only thing that could raise them,
and both objects are left standing for `tools/sweep.sh` — which is the whole of what the
sweep does here, because the eleven functions that go all name a type or a field the edit
removes and leaving them would hand the sweep a file that does not compile.

### The declared delta is nothing at all, and it is the strongest instance of the sixth kind any zero phase has had

The code runs and the instrument sees it do the same thing — and it is strongest here for
a reason about **where** the phase is rather than how careful it was: **every keystroke
this editor draws reaches its text through `ml_find_line()`**. Two whole recordings are
byte-identical across all 122 records.

**And it is only the sixth kind because phase 123 exists.** Before it a recording was 102
screen cases that allocate exactly one data block each, and a binary with one line deleted
from `ml_find_line()`'s pointer bookkeeping recorded every one of them byte for byte; a
phase that rewrites the descent, measured against that, would have been the **second**
kind and would have owed probes for the whole text layer. So the check does not merely
diff the recording: it plants five counters — root splits, pointer-block splits,
data-block splits, deepest descent, data blocks made — in **both** sources and requires the
sixteen memline cases to agree event for event, which they do.

**Five controls, and the fifth moves nothing and is reported.** `c_root` (the root test
made a test nothing passes) and `c_stack` (every stack entry remembers the root) move a
65,149-line session and leave a two-hundred-line one alone — and that two-hundred-line
session is not an easy target: 200 lines of 20 bytes already fill two data blocks, so it
descends through a real pointer block and still cannot see either control. `c_descend`
(every descent takes the first child) moves both, and `c_mlroot` (`ml_open()` never writes
`ml_root`) moves all three sessions, which keeps the finding from being a session nothing
could fail. `c_pages` (`mf_alloc_bhdr()` sizing every block one page) moves **nothing**,
and the reason is worth having rather than hiding: since phase 124 `host_alloc` is a bump
allocator with no redzone and no free, so a block written past its end scribbles on arena
bytes nothing has handed out yet. **A short allocation there is a memory bug and not a
difference** — the last row of `CLAUDE.md`'s verification table, the one that needs a
sanitizer and not an instrument — and it is built, run and reported, which is phase 117's
seventh control exactly.

### The pointer entry shrinks and the tree gets wider, which is the thing a later phase has to know

Derived by compiling the structs out of both sources rather than written down:
`sizeof(PTR_EN)` **24 → 16** bytes, so `pb_count_max` — children per pointer block — goes
**170 → 255**, and a root split needs more than that many live data blocks. It was 127
before phase 125. The corpus's largest case makes **321**, so it still splits the root,
with 66 blocks of margin. Because of this the check's own deep session states the data
layer as an equality and the pointer layer as an **inequality**: at 65,149 lines — derived
from `pb_count_max` and the lines a data block holds, not typed in — both binaries make
386 data blocks, descend 2 deep and split the root once and draw the same stream, while
the output splits a pointer block **once** against the input's twice, because a wider
block splits no more often than a narrower one.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 79,294 | **78,977 (−317)** — the edit takes 315 and the sweep 2 |
| names | | **47 leave in the edit, 2 in the sweep, 6 arrive**, three computed sets |
| `blocknr_T` / `mf_hashitem_T` / `mf_hashtab_T` | 13 / 18 / 14 | **0 / 0 / 0** |
| `sizeof(PTR_EN)` / `pb_count_max` | 24 / 170 | **16 / 255** |
| `make editor.c` | 77,315 | **76,998**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, an equality with its reason |
| binary | 768,744 | **760,456** |
| records that moved | | **0 of 122**, with five tree counters agreeing case for case |

`nm -u` does not move, and the reason is stated rather than the number: the hash, the free
list and the block numbers were **pure computation inside the file**, reaching the outside
only through `alloc()` and `vim_free()`, which have been `host_alloc` and `host_free`
since phase 118. `tools/zerodelta.sh --phase 126` reports *exactly as declared* — screen
102/102, memline 16/16, `ref-excmds.txt` 111/111 and `ref-argv.txt` 30/30 — which is a
different comparison from the check's own `diff -r` of two recordings: one asks whether
the output matches its input, the other whether it matches whim. The synthetic input the
phase was designed against is **byte-identical** to the real q125.

### Its placement

`stage 126`, `package memline 43 44 45`, which the charter names as the end of the road —
*the text later held as a tree*. It is not `buffers`, which is phase 94 and an Ex-level
refusal to quit, and not `tidy`, which is leftovers of cuts already made.

**`need 126 swept` is required and what breaks without it is not the anchor a reader would
guess.** Measured on exactly the text phase 125's edit leaves, the edit runs to its last
act and refuses with *names this phase removes are still said: `blocknr_T` 1,
`mf_hashitem_T` 1, `mf_hashtab_T` 1* — phase 125 leaves `mf_hash_free_all` standing for the
sweep and its **forward declaration** names all three of the types this phase deletes.
**The cut applied cleanly and the partition refused, which is what a partition is for.**

**There is deliberately no `apart 125 126`, and both halves are measured rather than
argued.** Phase 125's check quotes verbatim the two lines this phase rewrites —
`pp->pb_pointer[0].pe_bnum = 1;` 1 → 0 and `if (hp-> bh_hashitem.mhi_key != 0)` 2 → 0 — so
it would stop. But with `stage 125-126` in the manifest `tools/stages.sh` answers *43 needs
swept input and does not start a stage (42-43)* and exits 1 **before any check runs**:
`need 126 swept` already forbids the only stage that could hold both, and an `apart`
nobody can measure is phase 119's rule. Both halves are written down so the next reader
knows it was checked and not assumed.

## Phase 127 (zero 44) — de-page the leaf

`pipes/whim127-edit.sh` and `pipes/whim127-check.sh`, `stage 127`, `package memline`. A data
block stops being a **page of bytes** and becomes an **array of line records**. Until this
phase a leaf is a header, an index of byte offsets growing up from it and a text arena
growing down from the end of the page, with `db_free` bytes of gap where they meet: a
line's text lives inside the block, so inserting a line in the middle memmoves the arena
and rewrites every index below it, a line that grows past the gap is appended-and-deleted
into another block, and a line longer than a page makes the block two pages. After it the
leaf is

```c
struct { char_u *dl_text; colnr_T dl_len; char dl_marked; } db_line[DB_LINE_MAX];
```

and a line's text is **its own allocation**: inserting shifts records, not bytes, and
replacing stores a pointer.

**Why it is cheap now.** The arena exists for exactly one reason, to avoid a `malloc` per
line, and the charter has retired it — *A GARBAGE COLLECTOR IS ASSUMED FROM HERE ON*. This
phase spends what phase 124 bought. And the target representation is **today's dirty-line
path made permanent**, which is why the rewrite is 184 lines out and 71 in and not a
thousand: `ml_line_ptr` under `ML_LINE_DIRTY` is already a separately allocated `char_u *`
with `ml_line_len` beside it, so `ml_flush_line()`'s sixty-line *does the new text still
fit* branch — the memmove, the index fixup and the append-then-delete fallback — has
nothing left to decide and becomes one store.

### What goes, every count measured on the input and asserted as a partition

`db_free` 14 mentions, `db_txt_start` 29, `db_txt_end` 6 and `db_index` **34** all to
**zero**; the fourteen interior pointers of the shape `(char_u *)dp + start` to zero;
`DB_MARKED`'s stolen top bit, 17 expressions, to a real field; `ML_APPEND_MARK` 5; and
**both `offsetof(DATA_BL, db_index)` — by having nothing left to measure**, which is a
stronger removal than respelling them as `sizeof`, and which a prior survey verified was
byte-identical and recommended against for exactly this reason. The edit classifies every
mention of every one of them by its enclosing function and refuses on one that is not in
the struct, the enumerator or one of the eight memline functions it rewrites; the check
re-reads the input's counts **off the input** rather than trusting the numbers. **The
third memline `offsetof` stays and is named as not this phase's**: `ml_new_ptr()`'s
measures a *pointer* block, which is the branch and not the leaf.

### `DB_LINE_MAX` is a free parameter now, and it is chosen by instrument reachability

A leaf used to hold whatever fitted in a page and nothing decides it any more. **The
corpus cannot see the value at all**: 32, 64, 128 and even 1 record all 118 cases byte for
byte, so any argument from *the recording agrees* would have been vacuous. What it does
decide is how much of the tree the corpus **reaches**, measured with phase 123's markers on
q126, this phase's actual input:

| `DB_LINE_MAX` | SPLITDATA | SPLITPTR | SPLITROOT | IDXNZ | DEEP |
| --- | --- | --- | --- | --- | --- |
| 32 | 16 | 5 | 5 | 16 | 5 |
| **64** | **16** | **1** | **1** | **16** | **1** |
| 128 | 16 | 0 | 0 | 16 | 0 |
| 255 | 14 | 0 | 0 | 14 | 0 |
| q126, the input | 16 | 1 | 1 | 16 | 1 |

64 reaches exactly what the input reaches. **255 is the value that would fill the page and
it is the one that must not be chosen**: the natural-looking pick, the one that wastes
nothing, reaches no pointer-block split at all and would have blinded the instrument on
the very phase that rewrites the tree. That is phase 123's lesson applied to a parameter
instead of a corpus.

**And the margin is one case, which has narrowed under this phase.** The same table taken
on q123, where this was prototyped, read 6 / 5 / 1 / 0 in the SPLITROOT column: **128 was a
live choice then and reaches zero now.** Phase 125 took `pe_old_lnum` out of `PTR_EN` and
phase 126 took the block number and the page count, so `pb_count_max` has gone 127 → 170 →
255 while the corpus's buffer sizes have not moved. Measured directly with a counter on
`ml_new_data()`: `mem_deep_jumps` builds **321 data blocks on the input and 391 here**
against a `pb_count_max` of 255, and no other case comes near it on either side (204 / 155
/ 154 and 248 / 192 / 188). **A `PTR_EN` of 8 bytes would put `pb_count_max` at 511 and
take even 64 to zero** — at which point the corpus needs resizing or `DB_LINE_MAX` needs
lowering. The measurement stands; its margin does not, and that is the finding phase 128
turns into a compile error. The number, and that 32 reaches five, are written into the
edit, the delta and the commit, so lowering `DB_LINE_MAX` stays available if it is ever
preferred to resizing the corpus.

**This phase does not move `sizeof(PTR_EN)`**: 16 bytes either side, `struct
pointer_entry` byte-identical in and out, and the check pins `offsetof(PTR_BL,
pb_pointer)` at 1 → 1.

### The lifetime rule is pinned as a partition and not as prose

341 call sites depend on what `ml_get()` returns. It used to be a pointer **into the
page**, invalidated by any insert or delete in the same block, any flush of any line in it
and any split — the arena memmoves. It is now the record's own allocation and nothing
frees it, so **a pointer returned by `ml_get*()` is valid for the lifetime of the
process**. Stated as a partition — a record's text is written in exactly **five** places,
`ml_open` once, `ml_append_int` three times and `ml_flush_line` once, and freed in
**none** — and probed: a build that poisons the text a record stops owning moves **0 of
118**, which is the rule measured and not asserted.

**`ml_line_alloced()` is deliberately not simplified and the check enforces that.**
`del_bytes()` shortens `ml_line_len` in place under it and nothing would write that length
back, so `ML_LINE_DIRTY` must keep meaning *a replacement is pending* and not *the text is
allocated*. **It looks like an invitation and is a trap.**

### What it spends is measured, and it is the one cost no recording can see

The heaviest memline session asks the host for **201,927,792 bytes where the input asks
200,438,864**, +0.7 %, `mem_deep_jumps` either side. With nothing freed that is a
session's **traffic** and not its live data, which is why it is two hundred megabytes and
why phase 124's arena is a gigabyte. The counter is calibrated against a known answer
before it is believed — on the 233 non-memline sessions it reproduces phase 124's own
published high-water to the byte, **1,734,544** — and phase 124, rebased onto phase 123,
reached the same two numbers independently with an instrument written apart from this one.
The bound is **proven able to fail** rather than chosen: `ml_alloc_line()` over-allocating
by one page a line — the blunder the section is for — asks **304,354,000**, 1.52 times the
input, against the real output's 1.007.

### The leaf is still allocated as one memfile page, and that is deliberate scope with a measured cost

1,040 bytes of 4,096, asserted by a `static_assert` rather than left to be discovered. The
cost is reported and not hidden: the `cap` control, the capacity bound off by one, moves
**0 of 118**, because the 65th record lands in the page's spare room. Allocating a block
at its own size means giving memfile a **byte size where it has a page count**, which is
block *numbering* as well as block size — the machinery phase 126 has just rewritten — and a
phase that replaced the leaf's representation and changed how blocks are allocated in one
act would have two claims and one set of evidence. Phase 128 is that phase, and it measures
this prediction rather than repeating it. One prediction of this phase's had already come
true from the other side: `pe_page_count` and `bh_page_count` would be constant 1 after
it, and phase 126 removed both before it.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 78,977 | **78,859 (−118)** — 184 out and 71 in, and the sweep finds exactly one thing |
| `db_free` / `db_txt_start` / `db_txt_end` / `db_index` | 14 / 29 / 6 / 34 | **0 / 0 / 0 / 0** |
| interior pointers `(char_u *)dp + start` | 14 | **0** |
| `offsetof(DATA_BL, db_index)` | 2 | **0**, by having nothing left to measure |
| `make editor.c` | 76,998 | **76,880**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, a `comm` empty both ways |
| binary | 760,456 | **760,456** — the same size, different bytes, absorbed by alignment padding |
| arena high-water | 200,438,864 | **201,927,792 (+0.7 %)** |
| records that moved | | **0 of 118** |

The declared delta is **nothing at all** and it is the **weakest** kind on the list, phases
97 and 98's: the code changes, the binary moves, and the claim is that a replacement does
what the original did. There is no `cmp` to be had, so the two byte-identical recordings
are the **floor** and the **eleven controls** are the evidence. Eight must move and do —
the text not copied 2 of 118, the stored length dropped 36, a mark never set 4, the delete
shifting one record too few 6, the insert opening its gap the wrong way 8, the split
moving one record too few 4, every read taking the block's first record 52, every length
short by one 38 — and three must not, each with the reason it cannot be seen: `poison`,
`cap` and `DB_LINE_MAX = 1`. **The split control is the one that says why phase 123 was not
optional: 0 of the 102 screen cases and 4 of the 16 memline cases.** The corpus per case is
the input's own numbers — MLSPLITDATA 16, MLSPLITPTR 1, MLSPLITROOT 1, MLIDXNZ 16, MLDEEP
1 — and one marker goes down because the code is gone: phase 123's MLBIGLINE, a data block
of more than one page, fires in 3 of 16 on the input and has **no anchor in the output at
all**.

### The rebase cost exactly two anchors and both refused loudly

Which is the design working. The edit is written so that every region is located by a
function name and its own first and last line, every call whose arity changes is rewritten
by **place** and not by argument text, and every `ml_flags |=` inside a replaced region is
**carried forward as found** — and that last one paid for itself exactly as intended, phase
125 having deleted `ML_LOCKED_DIRTY` and `ML_LOCKED_POS`, so `ml_flush_line()` now carries
nothing and the edit needed no change. What did move: `ml_alloc_line`'s insertion point was
anchored on `long_to_char()`, which phase 125 deleted with block zero, and now goes
immediately above `ml_open()`, its first caller, so it depends on the function it is about;
and the probe's depth counter was declared at `ml_find_line()`'s `bnum = 1;`, which phase
126 deleted with block numbers themselves, and is now at `low = 1;` beside it — the line
that says the same thing about the **search** rather than about the representation.

### Its placement

`stage 127`, `package memline`, which phase 126 opened and whose comment says *phase 127
replaces the leaf, so the package grows*.

**`apart 126 127` is `apart 119 120` in its sharper form.** `tools/phaserun.sh whim 126-127` on
q125 runs both edits, one sweep and phase 126's check and stops with *the sweep: the names
that leave are ['ML_APPEND_MARK', 'ML_DEL_NOPROP', 'data_moved', 'db_free', 'db_index',
'db_txt_end', 'db_txt_start', 'e_didnt_get_block_nr_one', 'e_didnt_get_block_nr_zero',
'line_start', 'space_needed', 'text_start'] and this phase accounts for
['e_didnt_get_block_nr_one', 'e_didnt_get_block_nr_zero']*. Phase 126 states the division
between its edit and its sweep as a **partition over names**, a stage sweeps **once** at
the end, and the ten names this edit orphans land in phase 126's sweep set. **36-37 was that
lesson in a line count; this is the same lesson in a set, and a set is what a later phase
is more likely to state.** One direction only: phase 127's check was then run on exactly the
tree the shared stage produced and every part passes.

**And there is no `need 127`, measured as an equality and not as a run that did not
refuse.** The same 43-44 stage hands this edit phase 126's **unswept** output, and the file
the one sweep leaves is byte-identical to the sequential run's, 78,859 lines either way.
**The edit does not merely survive unswept text; it cannot tell the difference.**

`tools/phaserun.sh whim 127` exits 0 in 70 s and `make whim-tip` records q127 as
`5677d3f826f4`. The sweep finds exactly one thing in the whole phase, `ML_DEL_NOPROP`, and
the check states that division: **`ML_APPEND_MARK` is reachable code that can never be
true once the fallback goes**, so no sweep can see it and the edit takes it.
`make whim-verify` is 45 of 45.

## Phase 128 (zero 45) — fold the node types

`pipes/whim128-edit.sh` and `pipes/whim128-check.sh`, `stage 128`, `package memline`. The
memfile goes, and with it the last thing between the tree and its nodes. Until this phase
a memline node is **two** allocations: a `bhdr_T` of four members — two used-list
pointers, a `char_u *bh_data` and a lock flag — and, hanging off it, a 4,096-byte page cast
to `PTR_BL *` or `DATA_BL *` by the two-byte id at its front, with a `memfile_T` of two
members owning the list head and the page size. After it

```c
struct block_hdr     { short_u bh_id; };
struct pointer_block { bhdr_T pb_hdr; short_u pb_count; PTR_EN pb_pointer[PB_COUNT_MAX]; };
struct data_block    { bhdr_T db_hdr; linenr_T db_line_count; DATA_LN db_line[DB_LINE_MAX]; };
```

**There are no pages, no blocks and no memfile left — just a counted tree of nodes holding
line records.** A node is **one allocation at its own size**, 1,040 bytes for a leaf and
4,088 for a branch against 4,128 for either of them before. `bhdr_T` is the node's **tag**
and the first member of both, so `(PTR_BL *)hp` and `(bhdr_T *)pp` are the same address
and the file needs no union; `memfile_T` has nothing left to hold and is gone; and
`ml_root` answers *does this buffer have a memline* where `ml_mfp` did, taking `memline_T`
from 104 bytes to 96. `mf_open/close/new/get/put/free/ins_used/rem_used/alloc_bhdr/
free_bhdr` all go, and `mf_close()`'s teardown becomes `ml_free_tree()` walking the tree,
which is the same set of nodes.

`bhdr_T` was **not** the wrapper around one pointer the brief hedged for, and the phase
says so: it was four members and 32 bytes, a doubly-linked used list, a `char_u *bh_data`
pointing at a separate page, and a `BH_LOCKED` flag. It is now two bytes.

### This is phase 127's own named next step, and both halves are measured rather than repeated

Quoted in that phase's program: *"Allocating a block at its own size means giving memfile a
byte size where it has a page count ... it would take the leaf from 112 bytes a line to 64,
and it would MAKE AN OFF-BY-ONE IN THE CAPACITY BOUND VISIBLE, which today it is not."*

Phase 127's own `cap` control — the leaf capacity test widened by one — is built from **this
phase's input and from its output in the same run**: **0 of 118 records on the input**,
which is the 0 of 118 phase 127 published, and **4 of 118 here**. The 65th record used to
land in the page's spare room and now lands past the end of a 1,040-byte allocation. Phase
127's other half does **not** reproduce, and the phase reports what it measured rather than
what was predicted: the leaf's node cost falls from **64.5 bytes a line to 16.25**, not
"112 to 64".

### What the phase could have destroyed, and the measurement that says it did not

`pb_count_max` was computed per block as `(4096 - 8) / sizeof(PTR_EN)` = **255**, and it is
the tree's **fanout**. Phase 123's corpus reaches a root split in exactly one of its sixteen
cases, `mem_deep_jumps`, which builds **391** data blocks; the other fifteen and all 102
screen cases reach none of it. A phase that took `sizeof(PTR_EN)` to 8 would put the fanout
at **511**, and 391 < 511 would take root-split coverage to **zero — silently**, because
the instrument would still run and still pass, which is the defect phase 123 exists to have
ended.

So `PTR_EN` is not touched, and the new struct has the same offset **by construction**: a
two-byte tag and a two-byte count where three shorts were, so `pb_pointer` starts at 8 and
`PB_COUNT_MAX = 255` is the number the input computes rather than a number chosen.
`static_assert(sizeof(PTR_EN) == 16, ...)` is appended to the `make editor.c` cut of both
sides and both compile, with `== 8` required to **fail** against both so the assertion is
an assertion. The five markers are then measured case by case on both binaries and are
identical: MLSPLITDATA 16, MLSPLITPTR 1, MLSPLITROOT 1, MLIDXNZ 16, MLDEEP 1, and 0 of 102
screen cases.

**And the file now carries**

```c
static_assert(PB_COUNT_MAX == (4096 - 8) / sizeof(PTR_EN), ...)
```

**which fails to compile if a later phase narrows the entry.** The whole memline arc has
been shadowed by the risk that shrinking `PTR_EN` to 8 would take `pb_count_max` to 511 and
root-split coverage to zero without anything noticing. **It is now a build error rather
than a thing to remember**, which is the arc's standing hazard ended in the only way that
survives a reader who has not read this document.

### The hazard is demonstrated and not argued

The `fanout` control sets `PB_COUNT_MAX = 511`, what an 8-byte entry would give. It moves
**0 of 118 records** — and that **is** the point: the same binary takes MLSPLITPTR,
MLSPLITROOT and MLDEEP **from 1 to 0**, so narrowing the entry would take the root split
out of the corpus **without moving one record**. It is run under phase 123's instrument as
well as under the recording, **because the recording is exactly what cannot see it**.

Two other controls move nothing and are reported with their reasons. `noclear` —
`alloc()` for `alloc_clear()` — moves nothing because the host's arena is a bump pointer
over fresh pages, so the memory is already zero: **a fact about this host and not a promise
the core may rest on**, though `ml_open()`'s error path does rest on it, and that zeroing
is kept and is load-bearing exactly once, the error path walking a root whose single entry
has not been filled in. `nofree` — `ml_free_tree()` freeing nothing — moves nothing because
`host_free()` has returned without doing anything since phase 124, so **what a core gives
back is unobservable by construction**.

### The partition is over the file's whole vocabulary

And not over a list of names the edit happens to know. String literals excluded —
`zero-vim.c` has no comments and no preprocessor, so the scan is exact — **exactly 30
identifiers leave and exactly 5 arrive**: 26 the edit takes (236 mentions of `bh_next`,
`bh_prev`, `bh_data`, `bh_flags`, `mf_used_first`, `mf_page_size`, `memfile`, `memfile_T`,
`ml_mfp`, `pb_id`, `db_id`, `pb_count_max`, `MEMFILE_PAGE_SIZE`, the ten `mf_*` functions,
`mfp`, `page_count` and `page_size`), 2 the sweep takes (`BH_LOCKED`, which
`deadenums.py` finds, and `e_block_was_not_locked`, which `deadsweep.py` does — the whole
of what the sweep finds in this phase), 2 that go with a function the edit deletes
(`mf_close`'s `nextp` and `ml_find_line`'s `error_noblock` label), and 5 written
(`PB_COUNT_MAX`, `bh_id`, `pb_hdr`, `db_hdr`, `ml_free_tree`).

The open-buffer predicate is stated the same way: `ml_root` is compared with `nullptr` in
**no** function in the input and `ml_mfp` in **fifteen**, and in the output `ml_root` is
compared in those fifteen **plus `ml_delete_int`**, which asked the same question through
a local copy.

**Nothing is freed that was not freed before.** `mf_close()` walked the used list at
`ml_close()` and the used list was exactly the set of live nodes, so `ml_free_tree()` walks
the **tree** and frees the same set; `mf_free()`'s two call sites become `vim_free(hp)`,
one allocation where there were two.

### Measured

| | input | after |
| --- | --- | --- |
| lines | 78,859 | **78,666 (−193)** |
| a node | 2 allocations, 4,128 bytes either kind | **1 allocation**, 1,040 leaf / 4,088 branch |
| `bhdr_T` / `memline_T` | 32 / 104 bytes | **2 / 96 bytes**; `memfile_T` gone |
| `sizeof(PTR_EN)` / `PB_COUNT_MAX` | 16 / 255 | **16 / 255**, by construction and asserted from the cut |
| identifiers | | **30 leave, 5 arrive**, a partition over the whole vocabulary |
| `make editor.c` | 76,880 | **76,687**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, a `comm` empty both ways |
| binary | 760,456 | **760,424** |
| arena, heaviest case | 201,927,792 | **200,720,256 (−0.6 %)**, the biggest fall `mem_join_split` at −9.2 % |
| records that moved | | **0 of 118** |

**Every one of the sixteen memline sessions asks the host for less, and the difference is
arithmetic**: 391 data blocks times (4,128 − 1,040) is 1,207,408 of the 1,207,536 bytes
that go.

The declared delta is **nothing at all**, phases 97, 98 and 127's weakest kind — the code
changes, the binary moves, and the claim is that a replacement does what the thing it
replaces did. Two full recordings are byte-identical to the input's across all 118 cases
and four tables, so the recordings are the floor and not the evidence. **Twelve controls
carry the phase**, nine of which must move a recording and do: the leaf tagged wrong 118 of
118, the branch tagged wrong 118, the leaf test inverted 118, the root split forgetting its
count 1, the root split copying no entries 1, the branch capacity bound off by one 1, a
branch allocated at a leaf's size 6, a leaf allocated at half its size 16, the leaf
capacity bound off by one 4. Three must not, and each is named above with its reason.

### Its placement

`stage 128`, `package memline 43 44 45`. **The 45-on-44 dependency is the strongest in the
arc and is deliberately not a `uses` line**, both phases being in one package, so it is
written into the package comment instead; eight `uses` lines record the cross-package ones.
`tools/stages.sh zero --check` and `tools/packages.sh zero --check` both pass.

**`apart 127 128` is phase 127's own scope statement read from the other end, and it needed no
reasoning.** That phase wrote that `offsetof(PTR_BL, pb_pointer)` *"measures a POINTER
block, which is still a page of entries and is not the leaf ... De-paging the branch is a
phase of its own"*, and its check says it as a count of 1. Measured by running
`pipes/whim127-check.sh` on the q128 tree with phase 127's own state directory: *`ml_new_ptr`'s
offsetof moved, and a POINTER block is still a page and is not this phase's*. One direction
only, because there is nothing to observe in the other — the stage cannot run at all.

**`need 128 swept` is the first in this pipeline that is about blank lines**, and it is
`CLAUDE.md`'s *a pass that touches those needs a count of them as its own check* meeting a
shared sweep. The edit deletes whole functions and single statements out of the middle of
others, so it asserts that it leaves **no run of two blank lines anywhere** — which is a
statement about this edit only if the text it was handed had none. It had one: phase 127's
edit leaves a run of two at line 33,815 of its own unswept output, which `canon.py` removes
in phase 127's sweep. So the edit asks the **input** first, and `tools/phaserun.sh zero
44-45` on q126 stops with *the input already has a run of two blank lines, so this edit
cannot say it left none: it needs swept text*. **The order of those two tests is the whole
of it** — asked the other way round the refusal would have blamed this phase for the
previous one's residue.

`make whim-tip` records q128 = `698924a46bfa`, and `make whim-verify` reproduces it from q127
in a scratch root of its own. Two things not this phase's, both stated: q122 failed that
verify run on `ref-pty.txt` under a 64-way load and passes alone, which is the pty
flakiness `CLAUDE.md` already records; and **`del_file` is still an unread parameter of
`ml_close()`**, as it was of `mf_close()` before it — removing it reaches into
`'cpoptions'` through `CPO_PRESERVE`, which is not this phase's.

### What zero-vim is after phase 128

```
zero-vim.c        78,681 lines          from whim-vim.c's 86,583  (-7,902, 9.1%)
                  76,716 above the boundary, 1,965 below it
functions         1,737
type definitions  881
DWARF enumerators 1,168
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    109 that are not a t_ capability, 95 distinct globals
                        (orphanopts floor 80; 15 of margin)
built-in terminals 2 of whim's 10: xterm-256color and debug
#include          11, at line 76,718, and NOT ONE DIRECTIVE above them
core -> host      18 names: vim_snprintf, host_exit, host_message, host_time,
                  host_alloc, host_free, host_write, host_raise, ten musl_*
libc prototypes   0 -- the core names no libc function at all
libc symbols      14 with zero's flags, 15 as tools/symbols.sh counts
the memline       a tree of nodes, one allocation each: a leaf is 1,040 bytes
                  holding 64 line records, a branch 4,088 holding 255 children,
                  and a line's text is its own allocation nothing frees
binary            760,424 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    term-moved at 38 and four command lines at 39; 123 to 128 declare
                  nothing at all -- with 2 stderr-moved and the records of 87 to 94
                  before them
make editor.c     76,716 lines: 0 directives, 0 errors, 18 warnings, all of them
                  `used but never defined` and all of them the interface
```

**The `options[] rows` figure is stated here as the count that can be reproduced**: rows of
`options[]` whose name is not a `t_` terminal capability, 109, measured on this file. The
blocks above this one carry **107**, which no phase between phase 103 and here removed a row
to justify; the number that the floor actually reads, and that has tracked every removal
exactly, is the 95 distinct globals — `whim-vim.c`'s 116 and 102, less phase 95's six rows
and phase 103's `'termresize'`.

**Phases 123 to 128 are one arc and the documents carry it as one.** The instrument had to
exist before the work was checkable, which is 40; the bump allocator made per-line
allocation free, which is 41; 42 cleared the swap file's bookkeeping out of the way; 43
turned a block number into a reference; 44 let the leaf stop being a byte arena; and 45
ends it by folding the node types and turning the arc's standing hazard — that shrinking
`PTR_EN` would silently take the root split out of the corpus — into a `static_assert` that
fails to compile. Every one of 42 to 45 rests on a
measurement the corpus the pipeline had at phase 122 could not have taken — the fanout
narrowing, the tree events agreeing case for case, `DB_LINE_MAX`'s reachability table and
the `fanout` control — which is the whole argument for doing 40 first.

**The six phases declare nothing between them, and they are four different kinds.** 123 is
phase 86's and 116's — no source changed at all, so what has to be argued is that the
*comparison* moved safely. 124 is the sixth — the code runs and the instrument sees it do
the same thing — with a `cmp` of the **core** underneath it that no earlier phase could
offer. 125 is phase 92's and phase 95's **at once**, one instrumented build carrying both
halves. 126 is the sixth again and the strongest instance of it this pipeline has, because
every keystroke reaches its text through the function it rewrites. And 127 and 128 are the
**weakest** kind, phases 97 and 98's: the code changes, the binary moves, and eleven and
twelve controls carry each of them because nothing else can.

## Phase 129 — `p_emoji` is an `int`

The first phase that comes of transpiling `editor.c` to Go (`tx/FINDINGS.md`,
finding 1). `'emoji'` is a boolean option, and the options table writes and
reads every boolean option through an `int *` — `set_option_default()` stores
`*(int *)varp`, as `do_set_option_bool()` and `set_bool_option()` do — but its
variable was declared `char_u *`. The C got away with it: the static starts
zeroed and the `int` lands in the pointer's low bytes, so
`utf_char2cells()`'s `if (p_emoji && ...)` tests the right thing. The Go
transpilation cannot say that, and its first run panicked in
`set_option_default()`. The phase declares the variable what every writer and
the one reader already take it to be; one line, no line added or removed.

**Declared delta: nothing.** No recorded case types an emoji. The check's
evidence is a partition and a probe:

- **every `P_BOOL` row of `options[]` with a global variable names an `int`**
  on the output; on the input exactly one did not, `'emoji'`. The check fails
  if another appears or this one comes back.
- `p_emoji` is its declaration, its row and its reader, and nothing else.
- the libc surface is the set the stage was handed.
- **the probe**: `iab<U+1F600>cd<Esc>:q!` is written in the same bytes by the
  binary the phase was handed and the one it made; and **the control**,
  `+set noemoji`, writes different bytes, so the probe sees the option at all.

## Phase 130 — the `(pos_T *)-1` tests go

`get_address()`, `nv_gomark()` and `nv_pcmark()` each compared a mark lookup
with `(pos_T *)-1`, the value vim once returned for "a mark in another file".
Nothing in this tree returns it: `getmark()` is `getmark_buf_fnum()`, which
returns a pointer into the buffer or NULL, and `movechangelist()` returns NULL
or an element of `b_changelist`. So each test was an `if` never taken, and
`cutil.FoldNever` folds the three away, keeping the branch that runs. The Go
transpilation had written each as `if false` (`tx/FINDINGS.md`, finding 10).

**Declared delta: nothing.** The check proves the tests were dead from the
input — every mention of `(pos_T *)-1` is one of the three tests, and no
`return` of the four functions can produce it — and that the file lost
exactly the tests, their bodies and their `else` lines, counted from the input.
Its probe drives every way the three sites are reached, `'a`, `` `a ``, `:'a`
and `g;`, on both binaries; each control (an unset mark, an empty change list)
must write different bytes.

## Phase 131 — the saved input buffer is a `garray_T *`

`save_typeahead()` keeps what `inbuf[]` held in `tasave_T.save_inputbuf`,
made by `get_input_buf()`: a `garray_T`, allocated and filled — then cast to
`char_u *` to be stored, and cast back by `set_input_buf()`. Nothing read it
as characters. The Go transpilation could not carry a growarray in a string
pointer and registered it under a one-byte key (finding 6). The field, both
prototypes, the definition and the return now say `garray_T *`, and
`set_input_buf()` takes the growarray it always cast its argument to.

**Declared delta: nothing.** The check's partition: every line naming the
saved input buffer is one of seven, each saying `garray_T` where the input
said `char_u`; the two casts are gone and no other appeared; and the silent
compile is itself the proof that nothing passed a string where the growarray
goes.

## Phase 132 — nothing frees

Since phase 124 `host_free()` has an empty body — the host's arena is a bump
allocator, and a garbage collector is assumed — so `vim_free()`, a NULL test
around it, does nothing observable, and neither does any call to either. The
Go transpilation dropped every one (finding 9); this is the C catching up. All
273 calls in the core go: a call whose argument has no side effect goes
entirely, and the two whose argument decrements a counter
(`termcodes[--tc_len].code`, `uep->ue_array[--n].ul_line`) become that
decrement. `vim_free()` is then called by nothing and the sweep takes it; the
host keeps `host_free()`, which its formatter still calls.

A local whose only reader was a free is then only ever given values, which gcc
calls *set but not used* and the sweep leaves: the statements that set it are,
to gcc, its uses. So the phase also applies `edit.DeadStores`: a local declared
alone on its line in a function body, every other mention of which is a
statement `name = E;` with `E` only reading, goes with its stores. On q128 it
finds nothing, on this phase's edit exactly one (`taep` in
`clear_hl_tables()`) -- the same set gcc warns about, measured.

**Declared delta: nothing.** The check proves the premise from the input —
`host_free()`'s body is empty, `vim_free()` only calls it — and then **computes
the whole output**: the input with every call replaced by the same rule
(`edit.W132Rule`), and `vim_free()`'s definition and prototype removed, must be
the output byte for byte, the dead stores taken by the same function. It also
reports the blocks the calls leave empty,
which a later phase can fold once each condition is shown to have no side
effect.

## Phase 133 — one buffer needs no hash table

Whim has one buffer: `buflist_new()` is called once, at startup, and `curbuf`
is that buffer or NULL. So `buf_hashtab` held one entry, and
`buflist_findnr()` — its one reader, called only by `setmark_pos()` — found
the buffer by subtracting the key's offset from the key inside it, the
`container_of` the Go transpilation had to keep an owner registry for
(finding 3). `buflist_findnr()` becomes "the current buffer, if its number is
`nr`"; the table's init, add and remove go, and the sweep takes the two
helpers, the table and `b_key`.

**Declared delta: nothing.** The check proves from the input, as partitions
of assignments, that `curbuf` is only ever `nullptr`, `curwin->w_buffer` or
`buflist_new()`'s result and `w_buffer` only `nullptr` or `curbuf`, so the
current buffer is the one buffer whenever a mark can be set. Its probe sets
and jumps to the marks that go through `buflist_findnr()` (`m[`, `m]`,
`m"`); the control jumps to a mark never set.

## Phase 134 — the empty blocks fold

Phase 132 left 33 blocks with nothing in them: `if (allocated) { vim_free(p); }`
became `if (allocated) { }`; with those earlier phases left, 49 fold. An empty block guarded by a condition that only
reads (no call, no assignment) does nothing, and goes; so does an empty
`else`, and an empty `else if` that ends its chain. Loops are kept. A flag
that only such a condition read is then only given values, and goes with its
stores (`edit.DeadStores`, phase 132's): `did_intro`,
`event_cmdlineleavepre_triggered`, `mustfree` and `free_str`. The two rules
alternate until neither changes anything (`edit.W134Rule`).

**Declared delta: nothing.** The check **computes the whole output**: the
input's core with `edit.W134Rule` applied, run through the real sweep, must
be the output byte for byte. It names what the sweep took beyond the blocks,
and requires every empty block left to be one the rule must keep.

## Phase 135 — one regexp program type

vim had two regexp engines, and `regprog_T` was the header both programs
began with: `bt_regprog_T` repeated its five fields and added its own, and
the code cast between the two. Whim kept one engine, so every `regprog_T` is
a `bt_regprog_T` and every cast is to itself; the Go transpilation, which
cannot cast a struct to the larger one it heads, kept a registry
(finding 4). `regprog_T` takes the backtracking fields, the casts go, and
`bt_regprog_T` is not a name any more.

**Declared delta: nothing**, and more: every field keeps its offset, so the
check requires the input and the output, built with the boundary's flags and
`SOURCE_DATE_EPOCH=0`, to be **the same bytes**.

## Phase 136 — the engine is called directly

With one engine, `bt_regengine`'s four function pointers always hold
`bt_regcomp`, `bt_regfree`, `bt_regexec_nl` and `bt_regexec_multi`, and every
program's `engine` points at it — `bt_regcomp()`, the only function that
makes a program, sets it. So the four calls through the table call known
functions. They name them, `bt_regcomp()` stops recording an engine, and the
sweep takes the table, the field and `regengine_T`.

**Declared delta: nothing.** The check proves the premise from the input —
the table's initialiser in field order, one assignment of `->engine` — and
requires the three names gone. Its probe runs a search, a single-line
substitution with back-references and a multi-line one on both binaries;
each control, a pattern matching nothing, must write different bytes.

## Phase 137 — the changedtick is a number

vim kept `b:changedtick` as a dictionary item inside `buf_T`, so a buffer's
variables could hold it without a copy: `CHANGEDTICK(buf)` expanded to
`((buf)->b_ct_di.di_tv.vval)`, the number inside a `typval_T` inside a
`dictitem16_T`. Whim has no buffer variables, and that one field was what
kept `typval_T` — and through it lists, dicts, `type_T` and `class_T` — in
`buf_T`. The field becomes `varnumber_T b_changedtick`, its 18 reads and
writes name it, and `init_changedtick()` sets it to 0 and no longer sets a
type, a lock and flags that nothing reads.

**Declared delta: nothing.** The check's partition: on the input every mention
of `b_ct_di` is the field, `init_changedtick()`'s cast or a use of the
number; on the output each use is the input's, rewritten in place by the
same rule (`edit.W137Tick`). Every insert, undo and search in the recording
reads the tick.

## Phase 138 — no parameter carries an eval value

Five functions still took an eval value that every call passed as `nullptr`:
`vim_regsub_both()`'s `typval_T *expr` (substitute() with a funcref),
`match_add()`'s `list_T *pos_list` (matchaddpos(), never even read),
`cursor_pos_info()`'s `dict_T *dict` (wordcount()), the host formatter's
`typval_T *tvs` (printf()'s argument list, also handed to
`parse_fmt_types()`), and `find_ex_command()`'s Vim9 lookup and compile
context, never read. Each goes with its `nullptr`, and each test of it
becomes what it always was: `cursor_pos_info()` always gives its message, the
formatter always clamps an overlong width. The unused `cfunc_T` and
`cfunc_free_T` typedefs go too, since the sweep doesn't take a
function-pointer typedef. Then nothing names the eval layer's value types,
and the sweep takes all of them: `typval_T`, `list_T`, `dict_T`, their items
and watchers, `type_T`, `class_T` and the rest. Phase 137 made this possible, and
`vim_regsub_both()`'s second argument being always `nullptr` was the lead.

**Declared delta: nothing.** The check's partition: in each function every
line naming the parameter is its head, a test against `nullptr`, or its
being handed on, and each function's one call passes `nullptr`; on the output
none of the 16 eval types is named. Its probes cover the paths the tests sat
on — `g CTRL-G`, a `\=` substitution and `:match` — and each control moves.

## Phase 139 — the core sorts and searches typed arrays

The core sorted one array and searched four through the vendored
`musl_qsort()` and `musl_bsearch()`. These see an array as a `void *` stepped
by a byte width, and hand each element to the comparator as a
`const void *`. The Go transpilation could not follow a pointer through
`void *`: `sort_strings()`'s first Go signature was wrong, and each search
was typed by hand (finding 7).

The four searches — highlight attributes, colour names, key names and
character classes — now call `keyvalue_bsearch()` or `key_name_bsearch()`.
Each is `musl_bsearch()` line for line on a typed pointer, so it probes the
same entries in the same order, and a comparator that matches a prefix finds
the entry it found before. The comparators take the type they always cast
to. `:undolist`'s one sort is an insertion sort by `strcmp()`: two strings
that compare equal are equal byte for byte, so any order of them prints the
same. The sweep takes `musl_qsort()`, `musl_bsearch()` and `sort_compare()`.

**Declared delta: nothing.** The check proves each typed search's body is
the input's `musl_bsearch()` with its two byte steps made element steps. It
requires each search to use the table and comparator it had. Its probes are
`:hi` attributes, a colour name, a key name in a mapping, a character class
in a pattern, and `:undolist` over two branches; each control moves.

## Phase 140 — highlight groups are found in their array

Every highlight group's upper-cased name was kept twice: as `sg_name_u` in its
`hl_group_T`, and as the key of an entry in `highlight_ht`, allocated inside
an `hlname_T` beside the group's id. `syn_name2id_len()` found the key in the
table and got the id back by subtracting the key's offset in `hlname_T`. That
is the `container_of` the Go transpilation kept an owner registry for
(finding 3, its second half; phase 133 was the first).

A group is only added after the lookup of its name failed, so names are
unique. The group whose `sg_name_u` is the name is therefore the one the
table found, and its id is its index + 1, which is what `hn_id` held. So:
- the lookup scans the array;
- the name is saved and upper-cased on its own;
- `syn_unadd_group()` just decrements the length.

`highlight_ht` was the last hash table. The sweep takes `hashtab_T`,
`hashitem_T` and all of `hash_*`: the file loses 344 lines. The scalar
`hash_T` typedef stays, because the sweep doesn't follow scalar typedefs.

**Declared delta: nothing.** The check proves the premise from the input:
- `syn_add_group()` has one call, taken only when the lookup failed;
- the key is the group's `sg_name_u`;
- the id is `ga_len + 1` just before the length goes up;
- `highlight_ht` is the only table.

Its probes are a new group looked up in another case, a group a failed
`:hi` added and took back, and a link; each control moves.

## Phase 141 — `regrepeat()` does not jump into a case

The character classes `\s \S \d \D \x \X \o \O \w \W \h \H \a \A \l \L \u \U`
followed by a multi are eighteen pairs of case labels in `regrepeat()`:
- `\s` set its mask and `testval` and ran into the class loop, labelled
  `do_class`;
- each of the other seventeen set its own and jumped to that label from
  further down the switch.

Go cannot jump into a case, and the transpilation restructured it by hand
(finding 11). Now all thirty-six labels lead to one case. It sets `mask`, and
`testval` for the classes that match rather than exclude, in a switch on the
same opcode, then runs the loop. Every opcode makes the same assignments, the
loop is unchanged, and no `goto` is left in the function. The other jumps
into a case (`edit()`, `regatom()`, `check_termcode()`) are not yet phases.

**Declared delta: nothing.** The check reads an opcode → assignment table from
the input's jumps and another from the output's switch, each with its own
parse, and requires them equal with 18 entries. It requires the loop
unchanged byte for byte. Its probe substitutes with all eighteen classes, one
per line; the control swaps `\s` for `\S` and moves.

## Phase 142 — the version names no build date or time

`init_longVersion()` put `__DATE__ " " __TIME__` into the version line that
`mainerr()` prints above a command-line error: `VIM - Vi IMproved 9.2 (2026
Feb 14, compiled <date> <time>)`. So the core's text depended on when it was
compiled, and two builds of the same source differed unless
`SOURCE_DATE_EPOCH` pinned them. The Go transpilation had no such clock and
wrote in the constant the pinned build produces (finding 13). The version is
now its name and its release date, `VIM - Vi IMproved 9.2 (2026 Feb 14)`, and
the binary depends on the source alone.

**Declared delta: nothing**, and not because nothing changed. The version
line is written to stderr, and phase 85's `stderr-moved` excludes stderr from
every comparison, so the recording cannot see this change. The check
therefore measures it itself. It requires `__DATE__` and `__TIME__` gone. It builds the output at two `SOURCE_DATE_EPOCH`s and requires
the same bytes; the input built the same two ways, the control, differs. It
runs `-T` without its argument and reads the version line from both
binaries: the new one, and the input's with `, compiled Jan  1 1970 00:00:00`.

## Phase 143 — `regatom()` has no goto

`regatom()` jumped into the middle of other cases three ways (finding 11):
- `\_%)` jumped to the delimiter atom inside the `\%` case's own switch, and
  so did `\%>` when no digit follows it;
- `\_[` jumped to the collection that starts the `[` case;
- `.` followed by a composing character jumped to the multibyte node inside
  the default case.

Go cannot jump into a case or a block, and none of these does now:
- **the delimiter atom:** its block becomes `regatom_delim()`, moved verbatim,
  and its case and both jumps call it. `regnode()` never returns NULL, so a
  NULL can only be the block's own error return, and it is passed on;
- **the multibyte node:** its three statements are written where the jump
  was;
- **the collection:** the switch dispatches on `sw`, not `c`, in a loop that
  runs once. The `\_[` path sets `sw` to the `[` case and continues, while `c`
  stays `'['` as the jump left it.

The last jumps into a case are in `edit()`. `check_termcode()`'s jump goes into
an `if` body.

**Declared delta: nothing.** The check requires `regatom_delim()`'s body to be
the input's block line for line. It diffs `regatom()` from input to output and
requires every lost and gained line to be one the phase accounts for. Its
probes cover `\%)`, `\%t)`, `\%f]`, `\%>`, `\_%)`, `\_[` and `.` before a
composing character; each control moves.

## Phase 144 — `edit()` has no goto

Insert mode's loop jumped to three labels (finding 11):
- `doESCkey`, the second half of the Esc case, from nine places, some of them
  before the switch;
- `normalchar`, the default case's insertion, from seven cases;
- `do_intr`, the start of the Esc case, from the default case when the key is
  the interrupt character.

The two labelled blocks become functions: `edit_esc()`, which says whether
Insert mode ends, and `edit_normalchar()`. The locals they wrote (`count`,
`o_lnum`, `inserted_space`) are passed by pointer. Each jump becomes a call:
- **`doESCkey`:** `if (edit_esc(...)) return (c == Ctrl_O); continue;`. A
  `continue` means "the next key" only where the innermost loop is the main
  one, and the edit checks that at every site. The one jump inside a do-while
  sets `esc_now`, breaks out, and the flag is tested (and reset) right after
  the loop;
- **`normalchar`:** a call and a `break`, checked to leave the main switch;
- **`do_intr`:** its `goto_im()` test written out, then the call.

`check_termcode()`'s jump into an `if` body is phase 145's.

**Declared delta: nothing.** The check requires the helpers to be the input's
blocks, line for line apart from the pointers. It requires every `continue`
and `break` at a former jump to bind where the label's own did. It diffs
`edit()` and requires every change to be accounted for. It probes every key
whose case jumped that a terminal can deliver: Esc, CTRL-O, Tab, CTRL-K,
CTRL-], CTRL-F, CTRL-S, CTRL-L, CTRL-Z, CTRL-A and Enter. Three paths are out
of the harness's reach:
- CTRL-C: on its pty it is SIGINT, and both binaries exit before drawing;
- the do-while's site, which needs `stop_insert_mode`;
- `do_intr`'s, which needs an interrupt character other than CTRL-C.

For those three, where their `continue` and `break` bind is the evidence.

## Phase 145 — `check_termcode()` has no goto

While an OSC response was arriving over several reads, `check_termcode()`
jumped from the top of its loop to `handle_osc`, a label inside the OSC branch
of the if-chain in `if (key_name[0] == NUL)`, skipping everything in between.
That was the last `goto` of finding 11 that the transpilation had to
restructure. Nothing follows that chain inside its block, so the jump ran
exactly the OSC handling and then the code after the block. Now the jump's
`if` does the handling itself, and everything the jump skipped, from the key's
first byte through the end of the block, becomes its `else`. A `continue` or
`break` in that code binds to the same loop as before, because an `if` catches
neither.

**Declared delta: nothing.** The check proves from the input that the label's
chain is the last thing in its block. It requires the `else` to be the
skipped code byte for byte, indented four spaces further. Its probe sends an
OSC response in two writes and then types, and requires the same output
from both binaries. Two controls move: typing other text, and sending no
response. Whether the pty delivers the two writes as two reads is up to the
kernel, so for that path the byte-for-byte `else` is the evidence.

## Phase 146 — a memline node names its block

A node of the text's tree was either a `PTR_BL` or a `DATA_BL`, and each began
with a `bhdr_T` holding a tag. The tree held `bhdr_T` pointers and cast each
to the block its tag named: struct prefix inheritance. Phase 135 took the
regexp half of this out; the Go transpilation kept a registry of blocks for
this half (finding 4).

Now the header is the node. It holds its tag and a pointer to its block, of
the block's own type, set when the two are allocated. Each of the 19 casts
becomes a field read, so `(DATA_BL *)(hp)` is `hp->bh_data`, and the blocks no
longer begin with a header. The `static_assert` on a leaf's size loses the
header's 8 bytes: a leaf is its count and its records.

Casting a NULL gave NULL, but reading a field of one crashes, so this is only
safe because every cast was of a node known to exist. The check proves it: it
classifies every read of `bh_ptr` or `bh_data` by what shows its node is
there. That is a NULL test or a dereference of its tag earlier in the
function, the tree's stack, or an assignment from such a node.

**Declared delta: nothing.** The memline corpus in the recording exercises the
tree. The check's probe makes 300 lines, deletes 100 and undoes the deletion,
splitting leaves and making pointer blocks; one line fewer moves the output.

## Phase 147 — `deathtrap()` runs at the host's next wait

SIGHUP and SIGTERM ran `deathtrap()` as their handler. So the core's whole
way out (preserving, restoring the terminal, writing its message, exiting)
ran inside a signal handler, at whatever point the signal found the core.
That is undefined behaviour in C, since none of it is async-signal-safe, and
Go can't express it: Go's runtime takes the signal and hands it to a
goroutine. The Go host queued the signal and ran `deathtrap()` at its next
wait (finding 12).

The C host now does the same, with a self-pipe:
- **the handler** records the signal and writes a byte to a non-blocking
  pipe;
- **the wait** runs a pending `deathtrap()` first, then selects on the pipe
  beside the input. The read runs a pending one first too;
- **no race:** a signal that lands after the flag is tested and before
  `select()` leaves a byte, which ends the `select()` at once.

Timing doesn't change either. The core already blocks deadly signals
everywhere except the wait in `ui_inchar()`. `vim_handle_signal()` turns a
signal that arrives while the core is busy into an interrupt, and raises it
again when that wait unblocks. So `deathtrap()` only ever ran in that window,
and it still does, at the wait or the read inside it.

It maps line for line onto `editor/host.go`, and from there onto a Go
`select` over an input channel and a `signal.Notify` channel.

**Declared delta: nothing**, and the libc surface grows by `pipe2`, which the
check requires exactly. The host now has twelve `#include`s, `<fcntl.h>` for
the pipe's flags. The check's probes run on a real pty with stderr on its own
pipe: SIGTERM and SIGHUP while the editor waits for a key, and SIGTERM during
a substitution that backtracks for longer than the probe runs. Each gives the
same output, stderr and exit status on both binaries. The controls:
- SIGTERM and SIGHUP differ, because the message names the signal;
- the busy run is interrupted and never finishes its substitution, so the
  signal did land mid-computation. It says `Interrupted`, as it did before.

# What comes next

Not yet done, and each one only when it is asked for:

- **Optimisation**, last and deliberately: `-O0` is right for a tree rebuilt more
  often than it is run, and wrong for a binary shipped to a device. It has been
  last since Part I was written, and it still is.
- **The text as a tree.** Phases 123 to 128 prepared the memline for it — no pages,
  no blocks, no memfile, a line record per line — and the move itself belongs to
  the repository that receives the transpiled core, not to this one (the core's
  charter).
- **Whatever the host still provides** that the core could own, measured each
  time as the core's libc surface (`make score`) and its boundary (`make
  editor.c`), and never assumed.

When Part I ended at phase 82 this list also held the file-lookup layer, state
on disk and the build-time dependencies. Phases 89 to 96 took every way the
editor reaches a file, and 97 to 119 every libc function the core names, so
those items are Part II's history rather than its future.
