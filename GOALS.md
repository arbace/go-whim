# GOALS.md — reduce slim-vim to an embedded editor, and that to an embeddable core

`slim-vim.c` is vim as one translation unit, with every feature upstream's
`tiny` configuration has. **`whim-vim.c` is what is left when the editor stops
expecting a filesystem to have been installed for it** (Part I, phases 0 to 82),
**and then stops being a program at all and becomes a component a host program
runs** (Part II, phases 83 on).

```
slim-vim.c = F(upstream@sha)          SLIM-GOAL.md, twelve phases
whim-vim.c = G(slim-vim.c)            this document: phases 0-82, then 83 on
```

**The test suite this document describes is gone.** *What is measured*, the
declared deltas, the rules about them, the checks and the baselines are how
phases 0 to 163 were verified while they were written; the suite was removed
after `448e9a8`, the last commit that has it, and those sections are kept as
that record. What holds now is `CLAUDE.md`'s *Build*: one path, and the product
reproduced byte for byte.

The two pipelines are the same construct — a phase is a function of the tree it
is handed, memoized in three tiers — and differ only in what they remove.
`SLIM-GOAL.md` removes *files and preprocessor* and changes nothing about what
the editor can do. **This one removes capability, on purpose**, and every phase
has to say which and prove it removed nothing else.

**A phase is a directory, `internal/phase/NNN/`**, its number in three digits, and a Go
package of its own: `edit.go` and `check.go`, its `GOAL.md` — what it removes and why,
and what was measured — and its declared `delta.md`. This document is what holds for
all of them: the charters, the rules, what is measured, and an index of the
phases in each part.

The two parts are one pipeline with one numbering and one product. What changes
at phase 83 is what a phase is measured against: Part I's phases declare their
deltas against slim-vim's behaviour, Part II's against q82's, with an instrument
an editor with no file to write can still be measured by. `CoreFrom` in
`internal/build` is that line, stated once.

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

### The declared delta, phases 0 to 82

Each phase declares what it changes, in advance, in its own `internal/phase/NNN/delta.md`: a
token per way the binary moves, and `#` notes saying why. A phase with no token
declares nothing new. `tools/st.sh delta BIN SRC --phase N` (`internal/verify`)
reads every declaration up to
phase N and checks the binary moved in exactly that
way — these Ex commands, these behaviour cases, the terminal table or not — and
nothing else (rule 2 below, the rule that separates this document from
`SLIM-GOAL.md`). The tokens:

| token | means |
| --- | --- |
| `word` | an Ex command whose exit, or what it leaves behind, now differs |
| `case:NAME` | a behaviour case whose recording now differs |
| `term-moved` | the terminal table now differs |
| `drop:word` | a command declared earlier that no longer differs |

The declarations up to a phase are the whole difference from slim-vim at that
phase, which is why a stage checks only its last phase's: every earlier phase's is
inside it. They are measured against slim-vim's own recorded baselines, never
against the previous phase: each phase states the whole difference from slim, so a
phase that quietly undid an earlier one shows up.

From phase 83 (`CoreFrom`, `internal/build`) on, a phase is measured against
other baselines with another instrument, and `Delta` hands it to `CoreDelta`
(Part II, *What is measured from phase 83 on*). The list to 82
is not retired there: phase 83 checks the whole of it once more against its
`-no-pie` binary and slim-vim's baselines. A phase's declaration is read from
`internal/phase/NNN/delta.md` (`Declarations`; `tools/st.sh delta --list FROM TO` prints a
run of them) — the tokens inside the fenced block, not the notes.

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
| functions | `deadsweep` (gcc) and `funcreach` | yes — reachability |
| prototypes | `deadprotos` | n/a |
| types | `typereach` | yes — reachability |
| variables | `deadsweep`, `-Wunused-variable` | no — reference counting |
| struct fields | `deadfields` | no — a mention outside every type definition |
| enumerators | `deadenums` | no — a mention anywhere |

Each is `internal/dead`, and a `go tool whim` subcommand of the same name; the
phase accounts call them by the Python they were ported from (`cmd/whim/README.md`).

**The sweep is not written into a phase program; the plan runs it, once per
stage.** Every phase is two programs: `internal/phase/NNN/edit.go` makes the cut (and
may sweep part way through, where a second cut needs the first one swept), and
`internal/phase/NNN/check.go` asserts, builds and probes. A **stage** is a run of phases
whose edits share one sweep: every edit in order on text no sweep has touched
since the stage began, one sweep, every check in order on the swept text and its
binary, and then the declared delta once (`internal/verify`). The check shares
nothing with the edit but the work tree and a state directory — the line count of
the text its edit was handed, the stage's symbol snapshot, and whatever file the
edit names for it. Only a stage's end is a boundary.

The schedule — 0 | 1-12 | 13-41 | 42-63 | 64-65 | 66-71 | 72 | 73-77 | 78 | 79 |
80 | 81 | 82 — is in `internal/build/plan.go`, and `internal/phase/STAGES.md` keeps the two
kinds of fact that decided it, both measured:

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

Each phase's `delta` is its part of rule 2's list, **written once**: the Ex
commands, behaviour cases (`case:`) and terminal table (`term-moved`) it changes,
and `drop:` for a command that stops differing. The declarations up to a phase are
the whole difference from slim at that phase; `tools/st.sh delta BIN SRC --phase N`
checks a binary against exactly that, and a stage checks its last phase's.

### Adding a phase

A new phase is the next one after the last, and Part II's *Adding a phase* is the
current process for it; what follows is how the mechanics work, and holds for both
parts. A phase is a directory, `internal/phase/NNN/`, its number in three digits.

1. Write the cut in `internal/phase/NNN/edit.go`, or as steps in the plan. It does not
   sweep at its end, and write its `GOAL.md`, which opens `# Phase N — what it
   does`.
2. **Place it in the plan** (`internal/build/plan.go`): its steps, and whether a
   sweep follows. A sweep must precede it if its edit counts anchors against, or
   computes its cut from, swept text; record such facts in `internal/phase/STAGES.md`
   (`need N swept`), where the placement of every sweep was measured.
3. `make whim-build` says whether the product moved as the phase intends; there
   is no test suite, so the evidence the phase needs goes in its `GOAL.md`.

**The last two are covered by no warning at all**, and for a while they were
covered by no sweep either. A phase of its own asserted them, part way through
the pipeline and then again at the tip, because the first assertion had been
followed by nine phases that orphaned 40 more fields and 14 more enumerators and
nothing in their own sweeps noticed. **An invariant asserted in one place is a
cleanup.** Asserted in every sweep, it holds at every boundary, and no phase is
ever handed dead code by the one before it.

**A struct field is not a variable.** `deadfields` calls a field live if its
name appears outside every type definition, since a mention inside another struct
is a different field with the same name. It refuses what it cannot be sure of,
because being wrong here is silent:

- a bitfield or anonymous member, whose declaration does not say plainly what
  it declares;
- the last field of a struct, since an empty struct is not C and whole types
  are `typereach`'s;
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
implicit one after it, and several enums index a parallel table. `deadenums`
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

### The inner sweeps — and the failure the anchors cannot see

Each of the 23 phases with an inner sweep was run alone with it removed: **10 do not
need it** (12, 15, 17, 25, 35, 36, 39, 53, 55, 56) and 13 refuse without it.

**Then all ten were removed together, and the pass produced a different file.** Stage
`42–71` swept to text one option row longer than q71: `'arabic'` survived. Isolated
(`e11`): phase 53 without its inner sweep is fine alone, but phase 54 computes the
rows to cut *from the option table as it finds it*, found one row fewer, **and did not
refuse** — nothing about an under-cut looks wrong to the program doing it, and no
later phase takes the row.

This is the one real danger in regrouping, and it is a kind, not an instance:

> **A counted anchor fails loudly when its input is less swept than it expects. A
> computed set shrinks silently.**

Phases whose cut is computed from the text — "rows nothing reads", "options without a
variable", "functions whose whole body is a constant" — must see their input swept,
and that is a property to *declare* per phase, not to discover. It also makes the
recorded boundaries non-negotiable: without the byte comparison at every stage end,
`e10` would have shipped an editor with one more option.

With 53's inner sweep kept and the other nine removed (`e12`): 7 stages — `1–12`, `13–41`, `42–71`, `72–78`, `79`, `80–81`, `82` — **1,635 s**, every stage boundary identical. Phase 12 without its inner sweep costs a stage boundary before 13, and saves more than it costs.

## Concept index: the phases as packages

The phase sections below are in the order the work was done, and it shows: windows
are cut in six places, buffers in nine, options in eight. This index reads the same
83 phases **by concept** — eighteen *packages* — so that "everything this editor
lost about windows" is one list. It is placed here, after the rules and the sweep
that every phase shares and before the first phase section, because it is a table of
contents for those sections: it introduces nothing a phase depends on, and every
line of it points down into one of them.

**A package is a view, and nothing runs it.** No phase moved and no schedule
moved: a package's phases are spread across stages. The data is two more kinds of
line in `internal/phase/STAGES.md` — `package NAME P...`, and `uses A:P B:Q KIND why` for a
phase that relies on a phase of another package having run. They were checked by
a tool of their own while one existed: no phase in no package or in two, no
unknown phase or package, no `uses` inside one package and none whose dependency
runs later.

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
mechanical and 13 are rationale. The four classes are the record, and nothing
runs them.

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
| 0 | seed, in the one spelling every later phase reads | `0` |

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

## Phases 0 to 82

Each phase is a directory, `internal/phase/NNN/`, and a Go package: `edit.go` is its cut,
`check.go` its evidence, `GOAL.md` what it removes and why, and `delta.md` what it
declares.

**THE ACCOUNTS BELOW NAME THE TOOLS THAT RAN AT THE TIME**, and a few of those
are gone: `tools/phaserun.sh` running a stage, `tools/memo.sh` keying it,
`tools/implhash.sh` deciding what a change re-keyed, `make whim-tip` and `make
whim-pass`. What runs now is `internal/build` (the plan, and `make whim-build`), and
nothing verifies (the suite is at `448e9a8`); `CLAUDE.md` is where the current
shape is stated. A phase's account is a record of how it was measured, and it is
left as it was measured -- rewriting the history to name today's tools would
make it a worse record and no truer.

- [Phase 0 — seed, in the one spelling every later phase reads](internal/phase/000/GOAL.md)
- [Phase 1 — no `$VIMRUNTIME`](internal/phase/001/GOAL.md)
- [Phase 2 — the options for features that are not here](internal/phase/002/GOAL.md)
- [Phase 3 — no introduction, and the command line says only what the editor still decides](internal/phase/003/GOAL.md)
- [Phase 4 — the binary's name stops choosing what it does](internal/phase/004/GOAL.md)
- [Phase 5 — one regexp engine, not two](internal/phase/005/GOAL.md)
- [Phase 6 — the editor stops writing shell scripts, and stops drawing a menu](internal/phase/006/GOAL.md)
- [Phase 7 — the editor stops looking for files it was not given](internal/phase/007/GOAL.md)
- [Phase 8 — `:!` keeps its name and loses its process](internal/phase/008/GOAL.md)
- [Phase 9 — the editor stops asking the environment what language it is in](internal/phase/009/GOAL.md)
- [Phase 10 — no tag stack](internal/phase/010/GOAL.md)
- [Phase 11 — nothing is written that was not asked for](internal/phase/011/GOAL.md)
- [Phase 12 — UTF-8, and no other encoding, ever](internal/phase/012/GOAL.md)
- [Phase 13 — the editor stops re-reading a file it has already read](internal/phase/013/GOAL.md)
- [Phase 14 — a file name means the file of that name](internal/phase/014/GOAL.md)
- [Phase 15 — the last two encoding options](internal/phase/015/GOAL.md)
- [Phase 16 — six options that no longer decide anything](internal/phase/016/GOAL.md)
- [Phase 17 — the last two per-buffer encoding options](internal/phase/017/GOAL.md)
- [Phase 18 — nothing is read at startup, and nothing on the command line decides anything](internal/phase/018/GOAL.md)
- [Phase 19 — the terminal is what the build says](internal/phase/019/GOAL.md)
- [Phase 20 — nothing outside the process is consulted](internal/phase/020/GOAL.md)
- [Phase 21 — there is nothing to recover, and the memfile is memory](internal/phase/021/GOAL.md)
- [Phase 22 — the working directory is where it started](internal/phase/022/GOAL.md)
- [Phase 23 — no floating-point library](internal/phase/023/GOAL.md)
- [Phase 24 — there is no mouse](internal/phase/024/GOAL.md)
- [Phase 25 — a write is a write, and nobody owns it](internal/phase/025/GOAL.md)
- [Phase 26 — five signals, not twenty-one](internal/phase/026/GOAL.md)
- [Phase 27 — `[[=a=]]` stops meaning "a with any accent"](internal/phase/027/GOAL.md)
- [Phase 28 — C indenting](internal/phase/028/GOAL.md)
- [Phase 29 — `:command`, user-defined commands](internal/phase/029/GOAL.md)
- [Phase 30 — `K` and the tag jumps, keeping `*` and `#`](internal/phase/030/GOAL.md)
- [Phase 31 — file-name modifiers](internal/phase/031/GOAL.md)
- [Phase 32 — insert completion, the popup menu, and the keys that reached them](internal/phase/032/GOAL.md)
- [Phase 33 — commands whose machinery has already gone](internal/phase/033/GOAL.md)
- [Phase 34 — no abbreviations](internal/phase/034/GOAL.md)
- [Phase 35 — no scripts, no session, no autocommands](internal/phase/035/GOAL.md)
- [Phase 36 — one tab page, always](internal/phase/036/GOAL.md)
- [Phase 37 — no command that does nothing](internal/phase/037/GOAL.md)
- [Phase 38 — the argument list is walked by `:next` and `:previous` alone](internal/phase/038/GOAL.md)
- [Phase 39 — one window, always](internal/phase/039/GOAL.md)
- [Phase 40 — no window sizes to set](internal/phase/040/GOAL.md)
- [Phase 41 — the buffer list is walked by `:bnext` and `:bprevious` alone](internal/phase/041/GOAL.md)
- [Phase 42 — one buffer, always](internal/phase/042/GOAL.md)
- [Phase 43 — no -c, --cmd, -R, -m, -M or -w](internal/phase/043/GOAL.md)
- [Phase 44 — no filters, sorting or alignment](internal/phase/044/GOAL.md)
- [Phase 45 — no `:drop`](internal/phase/045/GOAL.md)
- [Phase 46 — no `:wall`, `:qall`, `:quitall`, `:wqall` or `:xall`](internal/phase/046/GOAL.md)
- [Phase 47 — no `:startinsert`, `:startreplace`, `:startgreplace` or `:stopinsert`](internal/phase/047/GOAL.md)
- [Phase 48 — no `:noswapfile`](internal/phase/048/GOAL.md)
- [Phase 49 — one set of options](internal/phase/049/GOAL.md)
- [Phase 50 — only LF text files](internal/phase/050/GOAL.md)
- [Phase 51 — a byte that is not UTF-8 is kept as it is](internal/phase/051/GOAL.md)
- [Phase 52 — UTF-8 is not a question](internal/phase/052/GOAL.md)
- [Phase 53 — no conversion layer, no 'encoding'](internal/phase/053/GOAL.md)
- [Phase 54 — no option without a variable](internal/phase/054/GOAL.md)
- [Phase 55 — no option nothing reads](internal/phase/055/GOAL.md)
- [Phase 56 — no shell, runtime or keyword-program options](internal/phase/056/GOAL.md)
- [Phase 57 — no lisp](internal/phase/057/GOAL.md)
- [Phase 58 — no language mappings](internal/phase/058/GOAL.md)
- [Phase 59 — no command-line completion](internal/phase/059/GOAL.md)
- [Phase 60 — no suffix, case, delay, verbose-file, debug or filter-program options](internal/phase/060/GOAL.md)
- [Phase 61 — no window title](internal/phase/061/GOAL.md)
- [Phase 62 — no buffer-type, file-type, listing, jump, update-time or autowrite options](internal/phase/062/GOAL.md)
- [Phase 63 — no jump list](internal/phase/063/GOAL.md)
- [Phase 64 — no formatting, comment or nroff-macro options](internal/phase/064/GOAL.md)
- [Phase 65 — no rot13, no operator function, no empty key handler](internal/phase/065/GOAL.md)
- [Phase 66 — no sentences, paragraphs, sections, methods, #if blocks or comment blocks](internal/phase/066/GOAL.md)
- [Phase 67 — no mouse, no spell plumbing, no write-only flags](internal/phase/067/GOAL.md)
- [Phase 68 — one window, structurally](internal/phase/068/GOAL.md)
- [Phase 69 — one file argument, and no argument list](internal/phase/069/GOAL.md)
- [Phase 70 — :e reloads in place, and there is no swap file](internal/phase/070/GOAL.md)
- [Phase 71 — one buffer, structurally](internal/phase/071/GOAL.md)
- [Phase 72 — one window, one tabpage, structurally](internal/phase/072/GOAL.md)
- [Phase 73 — one frame](internal/phase/073/GOAL.md)
- [Phase 74 — no file marks](internal/phase/074/GOAL.md)
- [Phase 75 — no autocommands](internal/phase/075/GOAL.md)
- [Phase 76 — one regexp engine, so no retry](internal/phase/076/GOAL.md)
- [Phase 77 — no buffer-name argument matching](internal/phase/077/GOAL.md)
- [Phase 78 — empty functions, write-only counters, and the window id](internal/phase/078/GOAL.md)
- [Phase 79 — the constant-return predicates](internal/phase/079/GOAL.md)
- [Phase 80 — the Ex command table, cut to the commands that exist](internal/phase/080/GOAL.md)
- [Phase 81 — one line, one command](internal/phase/081/GOAL.md)
- [Phase 82 — the system headers nothing needs, and every comment](internal/phase/082/GOAL.md)

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

# Part II — phases 83 on: an embeddable core

`whim-vim.c` at q82 is an embedded editor: one static binary that expects nothing to
have been installed for it. **Phases 83 onwards take it to what is left when the editor
stops being a program at all, and becomes a component a host program runs.**

What differs
from Part I is what the phases remove and what their behaviour is measured against
(core rules 2 and 3).

This part was once a pipeline of its own, **zero**, and `d3ac925` retired the name
after the two were one: a record written before then -- a phase's `delta.md` or `GOAL.md`, `internal/phase/STAGES.md`, the
appendix -- calls the product `zero-vim.c` and the executable `zero-vim`, and means
`whim-vim.c` and its binary as they stood at that phase. The records keep the name
they were measured under.

**This part is iterative.** Its first forty-six phases, 83 to 128, build the core. Phase 83 is where
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
the editor could ever have reached — two deadly signals, each blocked inside its own
handler — and with it `_exit`, leaving exactly one `exit()` call in the whole file.
Phases 101, 102 and 103 are *a component, not a program* on request, and they are
§II.4c's three steps: `main()` becomes `static int vim_main(...)` with a
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
the host. `make editor.c` writes the lines above it — **76,716** of the file's
78,681 at phase 128 — and they are a complete translation unit whose warnings are the
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
application of §II.4c's rule that the core is optimised for transpilation
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
And 121 and 122 are the terminal, which is the first thing Part II has taken that the
recording can see since phase 94: 121 keeps **two** of the ten built-in terminal names,
`xterm-256color` and `debug`, and declares `term-moved` — the first use of a token that
had been in the grammar since phase 83 — and 122 removes `-T {term}`, so that
`+{command}` is the whole command line and nothing outside the process can say what
terminal this is. **And 123 to 128 are the memline, which is one arc and not six phases**,
the charter's *the text later held as a tree* begun: 123 changes no source and gives the
pipeline a corpus that can see the text layer at all, because a build with one line
deleted from `ml_find_line()`'s descent recorded all 102 screen cases byte for byte; 124
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
Phases 129 to 162 then remove from the core what translating it to Go had to work
around, `internal/gen/FINDINGS.md` mapping each finding to its phase. Phase 163 prints the product in
the canonical spelling phase 0 seeds with. Phases are added one at a
time, each on the user's own request, and each is written into its own `internal/phase/NNN/`
and into `internal/phase/STAGES.md` when it is added — never in advance.

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
  unit. §II.4c is where that was decided and `.claude/briefs/zero-split.md`
  is the survey of the two-file design it replaced — abandoned, and worth reading only
  for what it measured the split would have cost.
- **No musl dependencies** in `whim-vim.c` itself. Whatever the core still needs
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
  24 of a 32-byte struct and the page-count arithmetic uses it. §II.4d
  states it and names the one other construct of the same shape. **And nothing
  constrains what replaces it**, which is worth knowing before the move is designed:
  the memfile is purely in memory — `mf_open()` takes no name, sets `mf_page_size` from
  a constant and hands out pages `alloc()` returns, `mf_sync()` clears the dirty flag
  and returns FAIL, and `mf_write`, `mf_read`, `mf_release`, `mf_fd` and `ml_recover`
  have **0 mentions** in `whim-vim.c` — so `DATA_BL` is an internal layout with no
  compatibility constraint on it and not a disk format any more.
- **`whim-vim.c` stays pure C without a preprocessor** — it inherited 18 directives
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

### The declared delta, phases 83 on

Each phase from 83 declares what it changes in its own `internal/phase/NNN/delta.md`, as Part
I's phases do, and `tools/st.sh delta BIN SRC --phase N` reads every declaration
from 83 to N and checks the binary moved in exactly that way — and nothing else (core rule
2). The tokens:

| token | means |
| --- | --- |
| `word` | an Ex command name: its block in `ref-excmds.txt` now differs |
| `case:NAME` | a screen case whose record now differs |
| `argv:NAME` | an invocation; spaces are written as `_` |
| `term-moved` | the terminal table now differs |
| `pty-moved` | the pty scenarios now differ |
| `screen-moved` | what the editor **draws** differs everywhere — the snapshots and the stream digest of every case, the text and message lines of every command row. Exit status, bells and stderr must still match, case by case |
| `stderr-moved` | what the editor writes to **stderr** differs everywhere, and nothing else does |
| `drop:X` | X was declared earlier and no longer differs |

The two `-moved` dimensions are not a way of saying "some things changed". Each
excludes **one** dimension from every comparison and must itself have moved
somewhere: a phase that declares `screen-moved` and draws the same screens fails
(proven, by declaring it and watching the comparison refuse). Everything outside
the declared dimension is still checked record by record.

**The difference is from q82, not from slim-vim.** The baselines are
`.reference/core-baselines`, which phase 83 records from the tree it is handed,
built with the compile line that tree carries. So everything phases 0 to 82 removed
is already in them, and the declarations start empty at 83: each phase from 83
declares only what it changes relative to q82.

## The core's rules

Cited as *core rule N*; Part I's rules still hold.

1. **Removal is computed, not listed.** Cut the entry points and let the sweep find
   what becomes unreachable (Part I, *The sweep*). The same six kinds, the
   same tools.
2. **Every phase states its delta, in advance, as a check.** Its `delta` is the
   list — an Ex command by name, `case:` for a screen case, `argv:` for a
   command line, `term-moved`, `pty-moved`, and the two dimensions `screen-moved`
   and `stderr-moved` — in Part I's grammar, and
   `tools/st.sh delta BIN SRC --phase N` shows exactly that set moved and no more. "Some
   cases differ" is not a check, and neither is a dimension declared that nothing
   touched: `internal/harness`'s `CoreCompare` refuses a `-moved` token whose
   dimension did not move.
3. **The delta is from q82, not from slim-vim, and it is cumulative.** From phase 83
   behaviour is compared with `.reference/core-baselines`, which phase 83 records from
   the tree it is handed — q82's `whim-vim.c` built with the compile line q82 carries.
   Everything phases 0 to 82 removed is therefore already in them, the declarations
   start empty at 83, and those up to phase N are the whole difference from q82 at
   N, as phases 0 to 82's are from slim. A phase declares what *it* changes, and
   every phase after it is held to that line too. **This is the one place the pipeline
   records baselines from its own output**, and it is not the mistake `CLAUDE.md` warns
   about, a pipeline re-recording from its own current binary and agreeing by
   construction: nothing from phase 83 on can reach q82, and phase 83 also proves the
   recording is q82's (see *Phase 83*). The instrument is another one because an editor
   with no file to write cannot be measured by Part I's (`whimtools zrecord`, phase 86).
4. **The product is `whim-vim.c`, produced from the committed `slim-vim.c`** by one pass
   of every phase, as Part I's rule 4 says.
5. **Phases are programs, not agents.** A phase is an edit and a check,
   `internal/phase/NNN/edit.go` and `internal/phase/NNN/check.go`, each a package of its own, run
   by `internal/build` and `internal/verify` in stages. From phase 87 the phases run
   in **each** stages: every edit swept on its own, and every check on exactly its
   own tree.
6. **Stages and packages are declared.** `internal/build/plan.go` holds what runs
   — the phase list, each phase's stage and where the sweeps fall — and
   `internal/phase/STAGES.md` keeps what measured it: `need`, `apart`, and the concept view
   (`package`, `uses`). What checks the schedule now is the product:
   `make whim-build-check`.
7. **The product carries no comments.** Phase 82 removed the last, and no phase from 83
   on writes one.
8. **The compile line is `gcc -O0 -fno-stack-protector -static -no-pie -s`, for every
   boundary, the input and the product alike.** An ordinary static executable —
   `readelf -h` says `EXEC`, with no `INTERP`, no dynamic section and no relocations —
   because a core with nothing left to relocate is one a host can place without a
   loader. It was introduced by phase 83 (`-no-pie`) and phase 84
   (`-fno-stack-protector`), with phases 0 to 82 on `-O0 -static -s`, a static-PIE;
   it is one line now (`internal/build/compile.go`, and the `Makefile`'s `CFLAGS`
   and `LDFLAGS`), and those two phases change nothing.
9. **The toolset is shared, and gated.** A change to a tool the build runs must
   leave `make whim-build-check` passing (the product is unmoved). There is no key
   to move any more: what was `tools/implhash.sh`'s account of which units a tool
   reached went with the memoize.

## Phases 83 to 128 as they stand

**This section is the whole of phases 83 to 128 read across, and it lives here
rather than in `CLAUDE.md` because it is 910 lines of one pipeline's history in a
file that is loaded into every session.** As in Part I, it names the tools that
ran when each phase was built -- `tools/phaserun.sh`, the memoize, `make
whim-tip` -- and those are gone; `CLAUDE.md` has what runs now. What `CLAUDE.md` keeps is the summary
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
`GOALS.md` states what it is for — an embeddable editor core that keeps the
screen and all visual editing and loses the filesystem, with `main()` demoted to a
host launcher and the text later held as a tree — and has forty-six phases:
`internal/phase/083/check.go`, the seed; `internal/phase/084/check.go`, which adds `-fno-stack-protector`;
`internal/phase/085/edit.go` with `internal/phase/085/check.go`, the first source cut — the two
"not to a terminal" warnings, the two-second pause after them and `--ttyfail`;
`internal/phase/086/check.go`, which changes no source at all and replaces the instrument
(below), so q86's tree and q85's have the same digest; `internal/phase/087/edit.go` with
`internal/phase/087/check.go`, which removes Ex mode, silent mode and the `-e -E -s -v`
options; `internal/phase/088/edit.go` with `internal/phase/088/check.go`, which leaves the
command line as `+{command}` and `-T {term}` — the file argument, the bare `-` and
`--` all become `mainerr(ME_UNKNOWN_OPTION)`; and `internal/phase/089/edit.go` with
`internal/phase/089/check.go`, which takes every way to write a file — the six Ex commands
`:write :wq :xit :exit :update :saveas`, four anchors and nineteen functions the
sweep finds, `ZZ` becoming `q!`; and `internal/phase/090/edit.go` with
`internal/phase/090/check.go`, which takes the way to read one — `:read` and its `:r !cmd`
arm, three anchors, six functions the sweep finds and one fold no tool could make,
the `exarg_T.usefilter` field that nothing writes once both `:w !` and `:r !` are
gone; and `internal/phase/091/edit.go` with `internal/phase/091/check.go`, which takes every way to
name another file to edit — the five Ex commands `:edit :enew :ex :visual :view`,
which are one handler, and the `gf gF [f ]f` keys, which are **arms** inside two
surviving handlers and not `nv_cmds[]` rows, six anchors and seventeen functions the
sweep finds; and `internal/phase/092/edit.go` with `internal/phase/092/check.go`, which takes the
machinery under all of those — `readfile()`, `read_buffer()` and the message layer
that reported what had been read, four anchors all inside `open_buffer()` and sixteen
functions the sweep finds; and `internal/phase/093/edit.go` with `internal/phase/093/check.go`,
which takes the buffer's **name** — `:file`, `buflist_new()`'s two name parameters,
sixteen folds of `b_ffname`/`b_sfname`/`b_fname`, and three further folds that free
the last three questions the core asked the filesystem — seven parts and **sixty**
functions the sweep finds, the most any Part II phase has handed it; and
`internal/phase/094/edit.go` with `internal/phase/094/check.go`, which takes the last thing the
filesystem left behind — the **refusal**, `E37: No write since last change`, which has
had no remedy to offer since phase 89 took every `:write` — as ONE fold of `ex_quit`'s
test, and sixteen functions the sweep finds, eleven of them the whole
switch-buffer/switch-window island that hung off `check_changed_any()`'s tail and that
no plan foresaw; and `internal/phase/095/edit.go` with `internal/phase/095/check.go`, **the
options nothing reads** — six `options[]` rows of a *computed* seven whose global has
no reader left, `'fsync' 'prompt' 'readonly' 'undoreload' 'write' 'writeany'`, with
`'readonly'`'s `W10` warning, its one-second pause and its two `[RO]` indicators;
and `internal/phase/096/edit.go` with `internal/phase/096/check.go`, **no `FILE *` that is never
opened** — `scriptin[NSCRIPT]` and `redir_fd`, two `static FILE *` that nothing has
ever opened in any build of whim-vim, `ui_write()`'s `console` parameter, and the
five functions the sweep finds under them; `internal/phase/097/edit.go` with
`internal/phase/097/check.go` and `internal/phase/098/edit.go` with `internal/phase/098/check.go`, **the
libc that is pure computation**, defined in the file as local `static musl_*`
functions — the sixteen `mem*`/`str*` of `<string.h>`, with `sprintf` moved onto the
editor's own `vim_snprintf` instead, then the character classes, the two `ato*`,
`qsort` and `bsearch`; and `internal/phase/099/edit.go` with
`internal/phase/099/check.go`, **the includes nothing names** — six of eighteen,
`<sys/stat.h>` `<fcntl.h>` `<iconv.h>` dead since before the pipeline and `<string.h>`
`<ctype.h>` `<wctype.h>` dead since phase 98, with the `stat_T` typedef no sweep could
take; and `internal/phase/100/edit.go` with `internal/phase/100/check.go`, **the deadly ladder
that cannot run** — the nine lines of `deathtrap()`'s `entered >= 3` arm,
`reset_signals()`, `_exit(8)` and `exit(7)`, which no build of whim-vim could ever
reach; and `internal/phase/101/edit.go` with `internal/phase/101/check.go`, **`main()` demoted to
`vim_main()`** — `static`, with a six-line launcher appended below it, both still in the
one file; and `internal/phase/102/edit.go` with `internal/phase/102/check.go`, **the core can no
longer stop the process** — `mch_exit()`'s `exit(r);` becomes `vim_host_exit(r);`
through a pointer the launcher installs, and the launcher lands on `__builtin_setjmp`
and returns the status; and `internal/phase/103/edit.go` with `internal/phase/103/check.go`, **the
signals and the terminal are the host's** — the five signal handlers, `mch_settmode`'s
three-valued mode, `mch_delay`'s sleep, `RealWaitForChar`'s `select`, `mch_get_shellsize`
and all three `isatty()` calls move into a 229-line `host_*`/`musl_*` block at the
bottom of the same file, the resize and the external stop and the interrupt arrive as
**bytes** in the input stream, and `fill_input_buf`'s `close(0); dup(2)` arm goes; and
`internal/phase/104/edit.go` with `internal/phase/104/check.go`, **the messages are the editor's and
the writing is the host's** — twenty output statements in five functions become eight
calls through one `vim_host_message(msg, len, err)` the launcher installs, with
`<stdio.h>` and seven symbols going with them; and `internal/phase/105/edit.go` with
`internal/phase/105/check.go`, **the variadic collapse** — the seven wrappers that walk a
`va_list` expanded at their 129 call sites into `vim_snprintf` plus the tail each
already had, five helpers against seven deleted definitions, so `va_start` appears
**once**; and `internal/phase/106/edit.go` with `internal/phase/106/check.go`, **`nullptr` and
`usize`** — `NULL` 2,555 → 3 and `size_t` 437 → 0, two names the language supplies
instead of a header, on a **byte-identical binary**; `internal/phase/107/edit.go` with
`internal/phase/107/check.go`, **the attributes** — 139 GNU `__attribute__` to six, the 113
`unused` deleted (**21 of them false**, marking a parameter the code reads), the 20
`fallthrough` respelled as the C23 `[[fallthrough]]`, and the three `format` and three
`format_arg` kept because they *are* the check `-Wformat` performs; `internal/phase/108/edit.go`
with `internal/phase/108/check.go`, **the plain host calls** — the two function pointers the
launcher installed become a forward declaration and a direct call, so `vim_main(int argc,
char **argv)` is phase 101's signature again; `internal/phase/109/edit.go` with
`internal/phase/109/check.go`, **the header types and macros the core can own** — `time_t`,
`sig_atomic_t`, `uintptr_t`, `struct timeval`, `MIN`, `MAX` and `offsetof` become the
core's own and nine libc prototypes are written out, **while the headers are still above
them to be cross-checked against**; and `internal/phase/110/edit.go` with
`internal/phase/110/check.go`, **the move** — the eleven `#include`s go below the core, twelve
header-supplied constants become enumerators asserted from below, and the formatter's
private island follows the four `va_list` functions down; and `internal/phase/111/edit.go`
with `internal/phase/111/check.go`, **the scalar clock** — `long musl_now_ms(void)` replaces
`void musl_gettimeofday(long *, long *)` and takes `elapsed_T`, `elapsed()` and the
out-parameter pair with it, **so no host call's shape is decided any more by a type the
core cannot name**; and `internal/phase/112/edit.go` with `internal/phase/112/check.go`, **the case
tables become one, and it is the union** — vim's `toUpper[]`/`toLower[]` and the
`musl_to*[]` phase 98 vendored disagreed at 97 upper and 96 lower codepoints, vim's
newer by ninety-six and musl's knowing `ß → ẞ` alone, and a core with no C library has
nothing for `'casemap'` to choose between; and `internal/phase/113/edit.go` with
`internal/phase/113/check.go`, **the message fold** — `msg_puts_attr_len()`'s never-taken arm
becomes one `host_message()` call, with `msg_puts_printf()` and `vim_strlen_maxlen()`
going, and **two folds measured and declined**; and `internal/phase/114/edit.go` with
`internal/phase/114/check.go`, **`abs` and `labs`** — called by the core, never in `nm -u`
because gcc lowers both to inline arithmetic, and vendored so that the core stops
depending on behaviour nothing states; and `internal/phase/115/edit.go` with
`internal/phase/115/check.go`, **the wall clock crosses too** — `vim_time()` becomes
`host_time()` below the boundary, `long time(long *tp);` leaves the core's prototype
block and a `static_assert` stronger than it replaces it; and `internal/phase/116/check.go`,
**the terminal table is asked a question it can answer** — the second phase that changes
no source at all, replacing `$TERM`, which phase 19 stopped the editor reading, with
`+set term={name}`, so nineteen rows that carried one answer between them carried ten
resolutions and nine refusals — two and seventeen since phase 121; and
`internal/phase/117/edit.go` with
`internal/phase/117/check.go`, **the core stops reallocating** — `realloc` rewritten at its two
core sites as a `host` allocation, a `musl_memcpy` of the **old** size and a free, because
`realloc` is the one libc function that cannot be vendored at all: to move the old
contents it needs a length its interface does not carry; and `internal/phase/118/edit.go` with
`internal/phase/118/check.go`, **the core calls nothing but the host** — `malloc`, `free` and
`write` become `host_alloc`, `host_free` and `host_write`, three prototypes above the
boundary and three definitions below it; and `internal/phase/119/edit.go` with
`internal/phase/119/check.go`, **the core names no libc function at all** — the last two
declarations go, and by different routes: `getpid` is **avoidable outright**, its one
caller `mch_get_pid()` feeding a `b0_pid` that whim's removal of recovery had already
left write-only, so the write, the function and the field all go and nothing calls a
host; `kill` is **moved**, `vim_handle_signal()`'s `kill(getpid(), got_signal)` becoming
`host_raise(got_signal)`, which takes no pid because a core that cannot ask for its own
process id must not be handed one; and `internal/phase/120/edit.go` with
`internal/phase/120/check.go`, **the degenerate unions** — six of the thirteen `union`
keywords unite nothing with anything, five single-member (`uh_next`, `uh_prev`,
`uh_alt_next`, `uh_alt_prev`, `vval`) and one **empty** (`es_info`), every one a leftover
of a cut already made and the last of them a GNU extension ISO C forbids, on a
**byte-identical binary**; and `internal/phase/121/edit.go` with `internal/phase/121/check.go`, **the
eight terminal names** — `builtin_terminals[]` goes from ten rows to **two**,
`xterm-256color`, which is already the compiled default, and `debug`, with three
capability tables, `find_builtin_term()`'s xterm-family clause and a repair to
`set_termname()`'s no-screen fallback that is not optional; and `internal/phase/122/edit.go`
with `internal/phase/122/check.go`, **`-T {term}` goes** — `command_line_scan()` becomes one
`if (argv[0][0] == '+')` and one `else` answering `mainerr(ME_UNKNOWN_OPTION)`, with two
`main_errors[]` rows and their enumerators, `mparm_T.term`, and the no-screen arm of
`set_termname()` that only `-T` could reach; and then the **memline arc**, which is one
arc and not six phases — `internal/phase/123/check.go`, **the instrument could not see the text
layer**, a whole-phase program that changes no source and adds the sixth part of a
recording, because a `whim-vim` with `pp->pb_pointer[idx].pe_line_count--` deleted from
`ml_find_line()`'s descent recorded **all 102 screen cases byte for byte** and forty
phases had been verified by a corpus that allocates exactly one data block a case;
`internal/phase/124/edit.go` with `internal/phase/124/check.go`, **freeing is free** — `host_alloc()`
a bump allocator into a 1 GiB arena and `host_free()` a function that returns, the
charter's *a garbage collector is assumed from here on* built entirely below the
boundary, with `free malloc realloc` leaving `nm -u`; `internal/phase/125/edit.go` with
`internal/phase/125/check.go`, **the swap file's residue** — `struct block0` with eight fields
written and none read, the negative block numbers `ml_append()`'s `newfile` could never
make, a three-layer dirtiness nothing tests and `pe_old_lnum`, none of which any tool in
`tools/` can see because **every one of them is written**; `internal/phase/126/edit.go` with
`internal/phase/126/check.go`, **a block number becomes a reference** — `pe_bnum` and `ip_bnum`
become `bhdr_T *`, `memline_T` gains `ml_root`, and the hash table that turned an integer
into a page goes with the free list and `mf_blocknr_max`, eleven functions and three
types; `internal/phase/127/edit.go` with `internal/phase/127/check.go`, **de-page the leaf** — a data
block stops being an index of byte offsets over a text arena and becomes
`DATA_LN db_line[DB_LINE_MAX]`, so a line's text is its own allocation valid for the
lifetime of the process, taking `db_index`'s 34 mentions, the fourteen interior pointers
and both `offsetof(DATA_BL, db_index)` **by having nothing left to measure**; and
`internal/phase/128/edit.go` with `internal/phase/128/check.go`, **fold the node types** — `bhdr_T`
becomes `struct block_hdr { short_u bh_id; }`, `memfile_T` goes entirely, a node is one
allocation at its own size (1,040 bytes for a leaf and 4,088 for a branch against 4,128
for either before), and the file gains a `static_assert` that **fails to compile** if a
later phase narrows `PTR_EN`, so
`whim-vim.c` is
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
Part II phase since 28 to free a symbol at all. **Phase 126's equality is the one worth
reading twice**: it deletes a hash table, a free list, three types and eleven functions
and frees nothing, because all of it was **pure computation inside the file**, reaching
the outside only through `alloc()` and `vim_free()`.

**After phase 96 the core cannot acquire a file descriptor and holds no stdio
stream, and that is an invariant rather than a count.** `open`, `access` and `fcntl` went at phase 92 and
`stat`, `getcwd` and `strerror` at phase 93, with `chmod fchmod fstat ftruncate
lstat unlink` already gone at phase 89 — so nothing in `whim-vim.c` can name anything
on a disk, and `read`, `write`, `close` and `dup` work on fds 0, 1 and 2 alone.
Phase 93's check asserts it in both directions: the undefined set must move by
exactly `getcwd stat strerror`, none of the eleven may be back, and `read close dup
fsync` must still be there — `fsync` being `ui_write()`'s and the `FILE *` phase's.
Phases 94 and 95 assert it again while freeing nothing themselves, and **phase 96
finishes it**: `FILE` is not named in `whim-vim.c` at all (2 → 0), `fclose getc putc
fsync` are gone, and the check requires `open creat openat stat access fcntl getcwd
strerror fopen fdopen opendir` absent from **both** the source and `nm -u`. The core
can read, write, close and dup fds 0, 1 and 2 and nothing else. §II.4b
states the invariant and it is assertable in that strongest form from here on.

**After forty-six phases whim-vim is 78,681 lines and 14 libc symbols, and what is
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
source at all, as phase 86 did**, so q116's `whim-vim.c` is q115's byte for byte and q123's
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
`host_time()` there, `write` since phase 118, and `getpid kill` since 119 — with
`__errno_location`, which no line of either
half names and which gcc emits for the host's three `errno` mentions — **measured by
splitting
the file at the first `#include`**, which since phase 110 is an exact line and not a
region anybody has to identify. Measured on the committed `whim-vim.c` that way: **the
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
the committed `whim-vim.c` it reports **6 of
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

**§II.4c is built out, and phases 118 and 119 finished it.** Its three steps
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
filesystem's** — three call sites, §II.4a had it going, and §II.1's decision 7
(*"do not ask whether stdin or stdout is a terminal"*) was only two thirds kept until
phase 103 took all three. **§II.4c's boundary is now drawn, and it is the `boundary`
package: phases 106, 108, 109, 110, 111 and 117** — one file with two parts rather than two
files,
the `#include`s moved to line 76,689 and **the first one the line between the core and
the host**, marked by nothing else. 106 was first and deliberately the smallest, because
it is the one a `cmp` can check; 108 made the two host calls plain; 109 gave the core its
own types and macros *while the headers were still above them to check against*; 110
performed the move; **111 is the one that finished what the line is for** — it
deleted the tagless `struct timeval` mirror 109 had to invent, so that no core → host
signature's shape is decided any more by a type the core cannot name, all eighteen of
them now taking scalars and byte buffers only; and **117 is in the package because its
product is one line fewer in the block 109 wrote**, not because anything is vendored —
`realloc` is the one libc function that *cannot* be vendored, needing an old size its
interface does not carry, so the core loses it by each call site supplying the length it
already knows. Phase 107, the attributes, sits between
them in a package of its own, `dialect`, and moves no line at all; phases 113, 115, 118, 119
and 124 are
`host`, because they move a thing the core did for itself **across** a line already
drawn — 124 being the one that moves nothing and changes what is *behind* a name the
core already asked through, which is why its evidence is that `make editor.c` is
**byte-identical in and out**, 77,681 lines and 2,064,232 bytes; and 120 and 125 are
`tidy` with phase 96, because five of 120's six unions and all four of 125's groups are
leftovers of cuts already made. See *The core and the host are one file with a line in it* in `CLAUDE.md`.

**What phases 121 and 122 take is not the boundary but the terminal, and that is the
`terminal` package: 85, 121 and 122.** Phase 85 removed the **question** the core asked about
a terminal it had not been told about — the two "not to a terminal" warnings, the pause
and `--ttyfail`; phase 121 removed the **vocabulary**, eight of the ten built-in terminal
names with three capability tables and `find_builtin_term()`'s xterm-family clause,
leaving `xterm-256color` and `debug`; and phase 122 removed the **telling**, `-T {term}`,
so nothing outside the process can say what terminal this is and `+set term=` inside the
editor is the only way. **121 is the first Part II phase to declare anything since phase
94**, and the first ever to use `term-moved`, a token that had been in the grammar since
phase 83 with nothing to say.

**And what the last six take is the text layer, which is one arc and not six phases —
`harness:123`, `host:124`, `tidy:125` and the `memline` package, 126, 127 and 128.** The order
is the argument. **123 had to be first**, because until it ran nothing in the pipeline
could tell a working memline from a broken one: a build with
`pp->pb_pointer[idx].pe_line_count--` deleted from `ml_find_line()`'s descent records
**all 102 screen cases byte for byte**, every one of them allocating exactly one data
block, so `idx` is 0 every time and a pointer entry's line count never decides anything.
`tools/zmemline.py` is sixteen cases of 200 to 25,000 lines built **in the editor** —
there is no file argument, no `:edit` and no `:read` — and its depth is measured and not
intended: a data block splits in 16 of 16 and the **root** splits in 4, against 0 of 102
for all seven markers. **124 made the work cheap**, `host_alloc()` becoming a bump
allocator and `host_free()` a function that returns, which is the charter's *a garbage
collector is assumed from here on*: the core may allocate a record per line and simply
not free it. **125 cleared the way**, four groups of swap-file bookkeeping that is
**written and never read** and that no tool in `tools/` can see for exactly that reason.
Then **126 turned a block number into a reference** — `pe_bnum` and `ip_bnum` become
`bhdr_T *` and the hash table that resolved integers to pages has nothing left to look
up — **127 let the leaf stop being a byte arena**, so a line's text is its own allocation
valid for the lifetime of the process, and **128 folded the node types**, so a node is one
allocation at its own size and `memfile_T` is gone. **128 also ends the arc's standing
hazard the only way that survives a reader who has not read the documents**: the file
carries `static_assert(PB_COUNT_MAX == (4096 - 8) / sizeof(PTR_EN), …)`, which **fails to
compile** if a later phase narrows the entry — because an 8-byte `PTR_EN` would put the
fanout at 511, the corpus's deepest case builds 391 data blocks, and root-split coverage
would go to zero **without moving one record**. That last is demonstrated and not argued:
the `fanout` control moves 0 of 118 and takes three markers from 1 to 0.

**Phase 95 is the other Part II phase that declares nothing, and for the opposite
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
whim-vim could reach, because `catch_signals()` installs with `sa_flags = 0` and
`signal_info[]` has exactly two deadly rows, so `entered` can reach 2 and never 3. Its
evidence is the same source built five ways, of which two differ in **one `sigaction`
field**: with `sa_flags = 0` a forced double signal stops at depth 2 and exits 1, and
with `SA_NODEFER` it reaches depth 3, runs the ladder and exits **7** — which is
`exit(7)` executing — and one forced signal further exits **8**, which is `_exit(8)`.
Phase 96's own argument was textual: `scriptin[]` is assigned once in the whole file — to NULL, inside the
function the phase removes — and `redir_fd` only by its own declaration, so neither
`FILE *` has ever been opened in any build of whim-vim and the phase removes the
*possibility*. Its evidence is the source it was handed, built with
`write(2, "FILESTAR-ENTERED\n", 17)` at **five** places (**0 of 106 records**) and
then with the identical instrument on `ui_write()` (**105 of 106**), plus eighteen
adversarial sessions that are each a way of making the editor *print* — which is
where `redir_write()` sat.

**Phase 92 is the one Part II phase no recording can see, and it says so.** `readfile()`
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
it does by measurement rather than by citation, `./whim-vim` extracted from all 33
recorded boundary tars plus `whim-vim.c` built with whim's own line giving **one digest
across every one of them** under the new question, as the old question gave one across
every one of them.
**Phases 117 and 118 are the strongest instance of a kind already on this list and not a
new one**, and it is worth saying which: their evidence is two byte-identical recordings,
which is usually the weak answer, and here it is not, because what they touch is on the
path of everything. `ga_grow_inner()` is driven **4,289 times per recording** on both
sides, 2,739 of those with `ga_data == nullptr`, and the control that keeps 117's rewrite
and copies nothing moves **102 of 102** screen cases; `lalloc()`, `vim_free()` and
`mch_write()` are entered **53,848, 22,417 and 1,012 times** over the 102 cases, and
`host_write` writing half the bytes moves 102 of 102 while `host_write` reporting half
moves 0 — which is 118's claim about the return value measured rather than argued. 117 also
owes a harness rather than probes, because **its four traps are memory bugs and not
differences**: an AddressSanitizer driver built at run time from both sources, where six
of seven controls each produce their own named finding.

Phases are added one at a time, on request, and these were born staged:
`internal/phase/STAGES.md` (a stage per phase and seventeen packages: `seed 83`, `build 84`,
`terminal 85 121 122`, `harness 86 116 123`, `streams 87 88`, `files 89 90 91 92 93`,
`buffers 94`, `options 95`, `tidy 96 120 125`, `vendor 97 98 114`, `includes 99`,
`host 100 101 102 103 104 113 115 118 119 124`,
`format 105`, `boundary 106 108 109 110 111 117`, `casemap 112`, `dialect 107` and
`memline 126 127 128`, with `apart 85 87` — phase 85's
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
holding 89 and 94 holding 93 already — and `apart 94 95`, because phase 94's check pins
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
and phase 96's anchors all holding there) and the declarations
(`2 stderr-moved`, phase 87's six records, phase 88's six command lines, phase 89's
two cases and six command rows, phase 90's two cases and one, phase 91's two cases
and five, **nothing at all for phase 92**, phase 93's one case and one row, and phase
94's one case and one row — where the row `quit` is the first Part II has declared that
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
by `tools/stages.sh` and `tools/packages.sh`. The last two
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
and its measured message is the one a reader would not predict: `tools/phaserun.sh
102-103` on q101 stops in phase 102's check with ``deathtrap` as a whole word has 4
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
where `tools/phaserun.sh 106-107` on q105 runs both edits, one sweep and both checks
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
direct `cmp`, phase 108's edit on q107 giving a `whim-vim.c` byte-identical to q108's.

**Phases 111 to 115 add eight more `apart` lines and no `need` at all** — each of the five
was measured to apply unchanged to the unswept text before it. `apart 110 111` is the
neatest of all of them: phase 110's boundary argument rests
on moving **one core function below the cut** as a control, the function it picked is
`elapsed`, and that is the function phase 111 deletes — so the check stops at its first
act with *"`elapsed` is not defined exactly once above the boundary, so the control that
moves one core function below it would not be a control"*. `apart 111 112` is arithmetic:
phase 111 states its own as a line count **of the core**, and a 111-112 stage gives it
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

**Phases 116 to 119 add ten more, and one pair is forbidden rather than declared.** 117
carries three — `apart 109 117` and `apart 110 117`, because both those checks assert
`void *realloc(void *p, usize n);` on a line of its own exactly once and it is not there
any more, and `apart 113 117` in **both** directions, phase 113's line count meeting the ten
lines 117 adds and 117's meeting the eighty-one 113 removes. 118 carries six: 109 and 110's
`PROTOS` list again, now with **seven** of its nine at 0 and only `getpid` and `kill` at
1; 110 and 111's written-out boundary, seventeen names where 115 made it fourteen;
`apart 113 118`, which is the sharpest instance this pipeline has of *a check that CALLS
libc from above the boundary is a dependency on the core still declaring it* — phase 113's
check writes `write(2, "T-cleos\n", 8);` **into the core** to build an instrumented
control, so run as a pair it does not compile at all; and `apart 117 118`, measured as a
real shared stage, where phase 117's check stops four ways and two of them are
`apart 105 106`'s lesson landing on the phase that had just taught it — 117's check matches
the code it wrote **verbatim**, `pp = malloc(new_len);` and `free(gap->ga_data);`, and 118
renames exactly those calls. **There cannot be an `apart 116 117` at all, and that is a
measurement and not an omission**: a stage of more than one phase is made of split
programs and `internal/phase/116/check.go` is ONE file, so `stage 116-117` is refused before any check
runs — `phase 116 is in stage 116-117 but is not an edit and a check` from
`tools/stages.sh`, and `phase 116 has no edit and check to run` from
`tools/phaserun.sh`. **No `need 116`, `need 117`, `need 118` or `need 119`** — 118's measured
three times, on phases 113's, 115's and 117's unswept output, and 119's on 118's, with the
prototype it is about to orphan still there and `b0_pid` still a field.

**Phase 119 carries exactly one `apart` line and the reason it carries only one is worth
copying.** `apart 118 119` was measured as a real shared stage on q117, and phase 118's check
stops three ways, the first being the block the phase exists to empty: *the ordinary
declarations above the boundary are none and the input's block minus the three is
`int getpid(void); / int kill(int pid, int sig);`*. Six further lines — 109, 110, 111, 113, 115
and 117, every one of whose checks this output also breaks, 109's and 110's nine-entry
`PROTOS` list reaching **zero of nine** on it — are **deliberately not written**, because
every one of them is already broken by phase 118 and recorded against it, and any stage
holding 119 and one of the six would have to hold 118 too. **A redundant `apart` nobody
measured is worse than none.**

**The last three are one each, and two of them are arithmetic.** `apart 119 120` is the
only one of the three measured in **both** directions, both phases being split: 119's
check stops on its own line count — *the file is 79786 lines and the input was 79804
(79804 recorded) — expected 79799* — and, run the other way on exactly the tree the
driver would hand it next, **120's check stops too**, with *the six declarations are 13
lines shorter between them* and *the boundary moved by 15 lines and the file by 13*. The
two extra lines are **phase 119's residue**, `mch_get_pid`'s forward declaration and the
`b0_pid` member, which 119 leaves for the sweep on purpose, arriving inside 120's
arithmetic because a stage sweeps **once**, at the end: *a phase that states its line
count against its own edit cannot share a sweep with a phase that leaves work for it.*
`apart 120 121` is `apart 100 101`'s shape — 120's line count meeting the 118 lines 121 takes
out of the same swept text — and `apart 121 122` is the sharpest form of `apart 105 106`'s
lesson this pipeline has: phase 121's check builds its first control by finding **the
fallback it repaired**, and phase 122 deletes that fallback, so a 121-122 stage stops at 121's
**first act** with *set_termname() names 0 of the surviving rows and this check needs
one — the fallback*. **A check that depends on the code the phase repaired is a
dependency on the next phase not needing it.** No `need 120`, `need 121` or `need 122` —
120's and 122's measured on the unswept output before them, 121's reported as **vacuous**,
phase 120's sweep removing nothing at all.

**The memline arc adds three `apart` lines and two `need`s, and one `apart` is forbidden
rather than declared.** There is **no `apart 122 123` and no `need 123`**, for the reason
there is no `apart 116 117`: `internal/phase/123/check.go` is one file, so `stage 122-123` is refused
before any check runs — *phase 123 is in stage 122-123 but is not an edit and a check* from
`tools/stages.sh` and *phase 123 has no edit and check to run* from
`tools/phaserun.sh`, both measured — and `need` is a statement about an **edit part**,
which a whole-phase program has none of. **`apart 124 125` is measured and is not the
mechanism the phase predicted**: it expected `apart 97 98`'s shape, an undefined-set
equality against a stage's one snapshot, and what actually fires is phase 124's **own
promise** — `tools/phaserun.sh 124-125` on q123 stops with *the text above the first
`#include` is not byte-identical in and out, and this phase is entirely below it*. **A
phase that promises to touch no core line cannot share a stage with one that deletes 366
of them.** **There is no `apart 125 126`, deliberately**, although phase 125's check quotes
verbatim two lines phase 126 rewrites and would stop: `need 126 swept` already forbids the
only stage that could hold both, `tools/stages.sh` answering *126 needs swept input and
does not start a stage (125-126)* and exiting 1 before any check runs, and **an `apart`
nobody can measure is phase 119's rule**. **`apart 126 127` is `apart 119 120` in its sharper
form**: phase 126 states the division between its edit and its sweep as a **partition over
names**, a stage sweeps once at the end, and the ten names phase 127's edit orphans land
in phase 126's sweep set, so the check stops naming all twelve — *119-120 was that lesson in
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
`tools/zrecord.sh` names the new tool, and that re-keyed the five Part II phases before it
and nothing else.

**And a harness can be blind for a reason that has nothing to do with the phase it is
run for.** That is what phase 116 found and fixed. `ztermcheck.py` still asked
`$TERM`, and **phase 19 had removed the `getenv("TERM")` the editor read it with** —
*the terminal is what the build says* — so all nineteen rows of
`.reference/core-baselines/ref-term.txt` said `term=xterm-256color t_Co=256`: nineteen
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
together**, and `internal/phase/116/check.go` measures that rather than citing it: `./whim-vim` out
of every recorded boundary tar **up to its own number** plus `whim-vim.c` built with
whim's own line gives one digest across every one of them, so `term-moved` stays
undeclared at every phase before it. The incantation, and **both halves are needed** —
measured, with only
`.cache/r0` removed `internal/phase/083/check.go` refuses and names `ref-term.txt`, which is right —
is `rm -rf .reference/core-baselines .cache/r0 && make whim-phase-83`.

**That bound is a repair, and the rule behind it is general.** The loop globbed
`.build/r*.tar` and required one terminal table across **all** boundaries, which was
true when it was written and reaches boundaries that did not exist then — so phase 121
moving the table on purpose made **phase 116's** check fail, measured with 121's tar
present: *q121 records a different table:*, exit 1. **A phase may assert anything it likes
about the past; it may not assert that the future will not change what it measured.** And
the place it would have struck is worth knowing: `make whim-verify` could never have
caught it, because a verify scratch root links the tools, the phase programs and the baselines and
has **no `.build`**, so section 4 takes its "no boundary binary" arm there. It would
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

**Part II's instrument is the screen, and it is Part II's own** (phase 86, §II.2).
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

**Three things differ from Part I, and each was decided rather than inherited:**

- **The compile line is `gcc -O0 -fno-stack-protector -static -no-pie -s`.** The
  seed adds `-no-pie` (`tools/templates/core.mk`), which is why `readelf -h whim-vim`
  says `EXEC` where phases 0 to 82 and slim-vim say `DYN`: see *The binary is standalone* in `CLAUDE.md`.
  Phase 84 adds `-fno-stack-protector` by editing the boundary's `Makefile` —
  never the template, which is the pipeline's input and in q83's digest. The product
  rule cannot read the work tree, so `whim.mk` states the flags once more, as
  `WHIMCFLAGS` and `WHIMLDFLAGS`, which is what a checkout with no work tree
  compiles the committed `whim-vim.c` with.
  `make score` passes both to `tools/st.sh score`, so the symbol count is taken with
  the product's flags.
- **Part II's behaviour is measured against its own baselines**,
  `.reference/core-baselines`, which phase 83 records from `whim-vim.c` built
  with whim's compile line. So the declarations start empty at 83 and each Part II phase
  declares only what it changes relative to whim, checked by `tools/coredelta.sh`
  — `whimdelta.sh`'s rule and grammar against those baselines. Phase 83 also runs
  the harnesses on the `-no-pie` binary and requires no difference at all, and
  runs `tools/whimdelta.sh --phase 82` on it against slim-vim's baselines, which
  still holds: 489 commands and 11 cases, exactly whim's declared delta.
- **The phase list was the `phases` line of `internal/phase/STAGES.md`**, and not a
  `PHASE_LIST` written into a tool, because a tool was in every key and a phase
  added there would have re-keyed the whole pipeline. There are no keys now:
  `internal/build`'s plan is the list, and `internal/phase/STAGES.md` is the record of
  how the stages around it were decided.


## Adding a phase

Only on request, and one at a time — and the pipeline's goal is met, so the
expected number of new phases is none. A phase is a directory, `internal/phase/NNN/`, its
number in three digits, and this is how one would join now.

1. **Write it in Go** if its cut is a program: `internal/phase/NNN/` is then a package of
   its own, `pNNN`, whose `edit.go` registers itself in an `init()`, and
   `internal/phase/registry.go` gains a line so it is linked in.
2. **Add it to the plan**, `internal/build/plan.go`: its steps in order and
   whether a sweep follows.
3. Write its `internal/phase/NNN/GOAL.md`, which opens `# Phase N — ...`, and add it to
   the index below.
4. `make whim-build` produces `whim-vim.c` and `editor/editor.go` again — **both
   are tracked, and a new phase moves them**, so the diff is the phase's product
   and is reviewed as such. There is no test suite (the one these phases were
   verified with is at `448e9a8`); what shows the phase does what it says is
   stated in its `GOAL.md`.

What used to be here — the boundary to record, `whim-tip`, and the warning that
the tracked product can lag the pipeline — went with the memoize. The product
cannot lag now: it is what `make whim-build` writes, and `make whim-build-check`
requires the committed bytes back from the committed input.

## Phases 83 to 163

Each phase is a directory, `internal/phase/NNN/`: its program, its `GOAL.md` and its
declared `delta.md`.

- [Phase 83 — the core's compile line, and the baselines it is measured against](internal/phase/083/GOAL.md)
- [Phase 84 — the stack protector goes](internal/phase/084/GOAL.md)
- [Phase 85 — the core stops diagnosing its own terminal](internal/phase/085/GOAL.md)
- [Phase 86 — the instrument becomes the screen](internal/phase/086/GOAL.md)
- [Phase 87 — no streaming Ex](internal/phase/087/GOAL.md)
- [Phase 88 — argv is `+{command}` and `-T {term}`](internal/phase/088/GOAL.md)
- [Phase 89 — no write](internal/phase/089/GOAL.md)
- [Phase 90 — no read](internal/phase/090/GOAL.md)
- [Phase 91 — no `:edit`, and no `gf`](internal/phase/091/GOAL.md)
- [Phase 92 — nothing reads a byte](internal/phase/092/GOAL.md)
- [Phase 93 — the buffer has no name](internal/phase/093/GOAL.md)
- [Phase 94 — `:q` quits, and `ZZ` is `ZQ`](internal/phase/094/GOAL.md)
- [Phase 95 — the options nothing reads](internal/phase/095/GOAL.md)
- [Phase 96 — no `FILE *` that is never opened](internal/phase/096/GOAL.md)
- [Phase 97 — the strings are the editor's own](internal/phase/097/GOAL.md)
- [Phase 98 — the character classes, the numbers and the sort](internal/phase/098/GOAL.md)
- [Phase 99 — the includes nothing names](internal/phase/099/GOAL.md)
- [Phase 100 — the deadly ladder that cannot run](internal/phase/100/GOAL.md)
- [Phase 101 — `main()` is demoted to `vim_main()`](internal/phase/101/GOAL.md)
- [Phase 102 — the core can no longer stop the process](internal/phase/102/GOAL.md)
- [Phase 103 — the signals and the terminal are the host's](internal/phase/103/GOAL.md)
- [Phase 104 — the messages are the editor's, the writing is the host's](internal/phase/104/GOAL.md)
- [Phase 105 — the variadic collapse](internal/phase/105/GOAL.md)
- [Phase 106 — `nullptr` and `usize`](internal/phase/106/GOAL.md)
- [Phase 107 — the attributes](internal/phase/107/GOAL.md)
- [Phase 108 — the plain host calls](internal/phase/108/GOAL.md)
- [Phase 109 — the header types and macros the core can own](internal/phase/109/GOAL.md)
- [Phase 110 — the move: the first `#include` becomes the boundary](internal/phase/110/GOAL.md)
- [Phase 111 — the scalar clock](internal/phase/111/GOAL.md)
- [Phase 112 — the case tables become one, and it is the union](internal/phase/112/GOAL.md)
- [Phase 113 — the message fold: `msg_puts_printf()` and the branch that reaches it](internal/phase/113/GOAL.md)
- [Phase 114 — `abs` and `labs`, the two the core took on trust](internal/phase/114/GOAL.md)
- [Phase 115 — the clock crosses the boundary](internal/phase/115/GOAL.md)
- [Phase 116 — the terminal table is asked with `+set term=`, not `$TERM`](internal/phase/116/GOAL.md)
- [Phase 117 — the core stops reallocating](internal/phase/117/GOAL.md)
- [Phase 118 — the core calls nothing but the host](internal/phase/118/GOAL.md)
- [Phase 119 — the core names no libc function at all](internal/phase/119/GOAL.md)
- [Phase 120 — the degenerate unions go](internal/phase/120/GOAL.md)
- [Phase 121 — the eight terminal names go, leaving two](internal/phase/121/GOAL.md)
- [Phase 122 — `-T {term}` goes, and the command line is `+{command}`](internal/phase/122/GOAL.md)
- [Phase 123 — the instrument could not see the text layer](internal/phase/123/GOAL.md)
- [Phase 124 — freeing is free, and the arena is measured](internal/phase/124/GOAL.md)
- [Phase 125 — the swap file's residue, and what no sweep could find](internal/phase/125/GOAL.md)
- [Phase 126 — a block number becomes a reference](internal/phase/126/GOAL.md)
- [Phase 127 — de-page the leaf](internal/phase/127/GOAL.md)
- [Phase 128 — fold the node types](internal/phase/128/GOAL.md)
- [Phase 129 — `p_emoji` is an `int`](internal/phase/129/GOAL.md)
- [Phase 130 — the `(pos_T *)-1` tests go](internal/phase/130/GOAL.md)
- [Phase 131 — the saved input buffer is a `garray_T *`](internal/phase/131/GOAL.md)
- [Phase 132 — nothing frees](internal/phase/132/GOAL.md)
- [Phase 133 — one buffer needs no hash table](internal/phase/133/GOAL.md)
- [Phase 134 — the empty blocks fold](internal/phase/134/GOAL.md)
- [Phase 135 — one regexp program type](internal/phase/135/GOAL.md)
- [Phase 136 — the engine is called directly](internal/phase/136/GOAL.md)
- [Phase 137 — the changedtick is a number](internal/phase/137/GOAL.md)
- [Phase 138 — no parameter carries an eval value](internal/phase/138/GOAL.md)
- [Phase 139 — the core sorts and searches typed arrays](internal/phase/139/GOAL.md)
- [Phase 140 — highlight groups are found in their array](internal/phase/140/GOAL.md)
- [Phase 141 — `regrepeat()` does not jump into a case](internal/phase/141/GOAL.md)
- [Phase 142 — the version names no build date or time](internal/phase/142/GOAL.md)
- [Phase 143 — `regatom()` has no goto](internal/phase/143/GOAL.md)
- [Phase 144 — `edit()` has no goto](internal/phase/144/GOAL.md)
- [Phase 145 — `check_termcode()` has no goto](internal/phase/145/GOAL.md)
- [Phase 146 — a memline node names its block](internal/phase/146/GOAL.md)
- [Phase 147 — `deathtrap()` runs at the host's next wait](internal/phase/147/GOAL.md)
- [Phase 148 — allocation cannot fail](internal/phase/148/GOAL.md)
- [Phase 149 — the allocation-failure branches fold](internal/phase/149/GOAL.md)
- [Phase 150 — the regexp stack is three typed stacks](internal/phase/150/GOAL.md)
- [Phase 151 — the option table's defaults are typed](internal/phase/151/GOAL.md)
- [Phase 152 — the option variables are typed](internal/phase/152/GOAL.md)
- [Phase 153 — `free_one_termoption()` compares without a cast](internal/phase/153/GOAL.md)
- [Phase 154 — the NULL write in `free_one_termoption()` is gone](internal/phase/154/GOAL.md)
- [Phase 155 — call arguments with effects are evaluated in gcc's order](internal/phase/155/GOAL.md)
- [Phase 156 — the regex size pass's node is a static byte, not (char_u *) -1](internal/phase/156/GOAL.md)
- [Phase 157 — get_register() and put_register() carry a yankreg_T *, not a void *](internal/phase/157/GOAL.md)
- [Phase 158 — a highlight's terminal font is read only from a colour entry](internal/phase/158/GOAL.md)
- [Phase 159 — a struct's text is a pointer to an allocation of its own](internal/phase/159/GOAL.md)
- [Phase 160 — no line getter takes a cookie](internal/phase/160/GOAL.md)
- [Phase 161 — no goto jumps into a block](internal/phase/161/GOAL.md)
- [Phase 162 — no two function pointers are compared](internal/phase/162/GOAL.md)
- [Phase 163 — the product is in the one canonical spelling](internal/phase/163/GOAL.md)

## Appendix to Part II — the plan phases 83 onwards were built from

A plan, kept as written, and cited by the phases as §II.1 to §II.6. Where it names the
product `zero-vim.c`, that is `whim-vim.c` as it stood when the plan was written. The
`P2`…`P12` labels are the plan's own names for the phases it proposed, which §II.3b maps
to what was built. What was built is in each phase's `GOAL.md`.

`GOALS.md` Part II was the charter, and it had two phases in it, 83 and 84, when
this was written. This is the plan for the next eleven: **the editor stops being something you pipe text through and becomes
something you type at, on a screen, and nothing else.** It was written without
touching a single pipeline file, tool or phase program — every number below was
measured in this worktree and in a scratch directory (`/tmp/zpty`, `/tmp/zfs`; see
the appendix), against the committed `zero-vim.c`, which is `whim-vim.c` byte for
byte and builds with phase 84's compile line into 869,512 bytes and 79 undefined
symbols.

**The fixed point.** `whim-vim.c` goes in. What comes out has no way to read or
write a file, no stdin to take text from, no stdout to give it back on, no Ex
mode, no silent mode, and no opinion about whether it is talking to a terminal. It
still draws a screen and still edits text.

### II. Summary

| | today | after the eleven phases |
| --- | --- | --- |
| lines of `zero-vim.c` | 86,614 | **≈ 80,300** (−5,636 measured by reachability, −≈700 estimated from folds) |
| functions | 1,874 | **1,719** (−155, measured) |
| undefined symbols (`nm -u`) | 79 | **≈ 60** (measured by simulation) |
| binary | 869,512 bytes | not estimated |
| what argv accepts | `+cmd`, `-T`, `-e`, `-E`, `-s`, `-v`, `-`, `--`, `--ttyfail`, one file | **`+{command}` and `-T {term}`** |
| how it is tested | 67 file-based cases, 111 Ex commands by exit status, 5 pty scenarios | **102 keystroke cases, 111 Ex commands by the message they print, 27 invocations, 5 pty scenarios** |
| what a test run costs | 1.2 s (file) + 20 s (pty) | **0.53 s** for 102 cases (measured on a patched binary; 6.4 s until the tty warnings go) |

**This table is the forecast it was written as.** What actually happened is in each
phase's `GOAL.md`; at phase 128 the file was 78,666 lines, 1,734 functions, 14
undefined symbols and a 760,424-byte binary, and argv was `+{command}` alone. §II.4 below is
where this document says which of its own rows landed and which were wrong.

**The harness is the hard part and it is solved.** Not with a pty: with a
keystroke file on stdin, the escape-sequence stream on stdout, and a screen
rebuilt from that stream. 102 cases, **eight consecutive runs byte-identical
including the sha256 of the byte stream**. Of the six deliberate breaks it was
tested against, four move exactly 11, 2, 1 and 1 case of 102, and two — a deleted
`nv_cmds[]` row and a changed default — move 100, which is itself a property to
declare.

### II.1. The decisions this plan is built on

These came from the user and are not open questions. Each is recorded with what it
costs, because several of them are the reason a phase exists.

1. **No load path.** Text does not come in on stdin; `vim -` and `EDIT_STDIN` go
   with the file argument. Nothing outside the process can put text in the buffer.
2. **No save path.** Text does not go out — no stdout dump, no `:w` anywhere, and
   "not even `editor.c` will have save/load". The embedding host owns both.
3. **No streaming Ex.** With neither stream there is nothing for `-e`, `-E`, `-s`,
   Ex mode (`Q`, `gQ`, `do_exmode`) or silent mode to do. `zero-vim` is "just a
   visual editor accepting input and emitting screen output through terminal
   connection".
4. **Testing is through that connection only** — keystrokes in, screen out. §II.2.
5. **`:q` always quits**, and `ZZ` becomes `ZQ`: the protection that says "no write
   since last change" has no remedy left to offer.
6. **The buffer keeps no file name.** The window will later be a view over an
   edn-like tree that simulates lines; that is not this plan.
7. **Do not ask whether stdin or stdout is a terminal.** The check goes entirely —
   not `--ttyfail`'s refusal made default, but no check at all.
8. **`+{command}` and `'paste'` survive every phase**, as harness infrastructure.
   No phase may retire either; a phase that would (a command-line cut, an options
   tidy that drops rows whose readers went) must exempt them and say so.
9. **argv accepts `+{command}` and nothing else** — **reversed by the user on
   2026-09-19**, and the decision it replaces is kept here because a reversed
   decision is worth more than an absent one: it read *"argv accepts
   `+{command}` and `-T {term}`, and nothing else"*, `--` going too, since with
   no file argument it only means "treat the next `+cmd` as a file", which then
   errors. That half stands. What changed is `-T`: which terminal the core
   drives is the **host's** business by the same argument as decision 7, and a
   core that keeps a command-line option for it is keeping a program's facility
   inside a component. **The option buys nothing that `+{command}` — which
   decision 8 promises to keep for ever — does not already buy**, measured here
   on the q115 binary: `-T ansi` and `+set term=ansi` both answer `term=ansi`,
   `t_ti=`, `t_te=`, and `-T debug` and `+set term=debug` both answer
   `t_ti=[TI]`. **Two things it costs, measured in the same run and not hidden.**
   The fallback goes: `-T no-such-term-9x` resolves to `xterm` and runs, where
   `+set term=no-such-term-9x` is `E522: Not found in termcap` — arguably the
   better answer, a core that does not claim to be a terminal it is not, but the
   fallback's own code becomes dead. And a probe that wants a non-default
   terminal table **before the editor's first screen** cannot be written
   afterwards: `-T debug` writes `[24CWS80][TI][KS]…` from byte 0, where
   `+set term=debug` writes 111 bytes of xterm prologue first. The phase that
   removes it is not landed; `-T {term}` is what phases 88 to 115 left, and four
   existing checks type it, which costs nothing — a phase check runs only against
   the two binaries of its own phase. `.claude/briefs/zero-terminals.md` §6d and
   §10 are the survey the reversal came from.

### II.2. The harness

#### II.2a. The pty is not the instrument, and here is why

`tools/ptyrun.py` and `tools/ptycheck.py` already drive a pty, and
`tools/ptycheck.py` says in its own comment what the problem is: *"The screen dump
itself is too timing-dependent to compare directly"* — so it records only the lines
that answer a `:set` query. That is not a behaviour harness; it is a spot check.
The pty adds four hazards that `CLAUDE.md` names — the Press-ENTER prompt that
swallows keys, the read loop that hangs for ever without a hard timeout, ANSI
escapes that have to be stripped before anything can be compared, and keystroke
timing that decides whether an Escape is an Escape or the start of a key code.

It also adds one that is not in `CLAUDE.md` and that this work measured: a case
whose command exits the editor (`:cquit`) races with the keystrokes still to be
typed, which the pty then **echoes onto a screen nothing is drawing any more**. One
run in four of a 111-command pty sweep recorded those echoes. The fix is to stop
typing when the child is gone, and the general lesson is that a pty harness has
state the editor does not control.

I built the pty harness anyway, because it was the obvious reading of the
constraint, and it worked: 92 cases, eight identical runs, 1.9 s. Then the
requirement changed to no-tty-check, which makes the simpler instrument possible,
and the pty corpus shrinks to the handful of things only a real terminal shows
(§II.2j). **The pty work is not wasted**: the screen emulator it needed is the same
one the stream harness uses, and the two instruments were compared against each
other (§II.2c).

#### II.2b. Keystroke file in, escape-sequence stream out

```
    keys        a file:      ":set paste\r" "ialpha\rbeta\x1b" ":set nopaste\r" "..."
    invocation  ./vim +cmd… < keys > stream 2> errors
    record      the screen, rebuilt from `stream`; plus exit status, stderr,
                the stream's length and sha256
```

There is no pty, no `select` loop, no settle time, no ANSI stripping and no
Press-ENTER hazard: a hit-enter prompt is simply what the screen says at that
point. The terminal is **80×24 by construction** — the window-size ioctl fails on a
pipe and the built-in fallback applies — and `$LINES`/`$COLUMNS` cannot override it
(whim's Phase 19 took that away).

Measured on the committed binary, `TERM=xterm`, stdin a file and stdout a file:
`ihello world<Esc>:q!<CR>` exits 0 and writes a 2,294-byte stream carrying the
typed text; three runs are byte-identical. The whole of it is 30 lines of Python
around `subprocess.run(..., start_new_session=True)`.

**Two things it must get right, both measured.**

* **A session of its own.** `:stop` and `:suspend` signal the process *group*
  with SIGTSTP. Without `start_new_session=True` the command sweep stopped the
  shell that ran it — exit 148, which looks like the harness dying at command 100.
  They are also on the sweep's skip list, as they are in the file-based sweep.
* **A timeout, and a recording for what hits it.** `vim -` in this harness reads
  the *keystroke file* as buffer text, then `close(0); dup(2)` and waits on stderr
  for keys that never come: measured, 30 s per invocation. An invocation that
  hangs is recorded as `TIMEOUT`, not treated as a crash.

#### II.2c. The screen, rebuilt from the stream

The record is not the byte stream (which is unreadable and moves whenever a redraw
is re-ordered) but the screen that stream produces. The editor emits a small set
of sequences under `xterm-256color`: CUP, CUU/CUD/CUF/CUB, EL, ED, IL, DL, ICH,
DCH, DECSTBM, SGR, private modes, CR, LF, BS, TAB, BEL. 180 lines of Python
replays them into a 24×80 matrix.

**One screen per redraw, from the stream alone.** The editor hides the cursor while
it draws and shows it when the screen is settled and it is about to wait for a key.
`\x1b[?25h` is therefore a step boundary *visible in the bytes*, and a record can
hold a screen per redraw without a pty and without timing:

```python
if params == '?25' and final == 'h':
    snap = (self.dump(), self.y, self.x, self.bells)
    if not self.snaps or self.snaps[-1][0] != snap[0]:
        self.snaps.append(snap)
```

That is what makes the messages recordable. With only the final screen, every row
of the command sweep read `msg : ~`, because the keys that quit the editor wipe the
message line; with per-redraw snapshots, `:write` leaves `E32: No file name` in
snapshot 4 and it stays in the record.

**A terminal is columns, not characters.** The first emulator counted characters,
and 2 of 88 cases disagreed with `pyte` — both CJK: `日` is two cells wide and a
combining mark is none. `unicodedata.east_asian_width` and `unicodedata.combining`
are enough, and are in the standard library:

```python
def cellwidth(ch):
    if unicodedata.combining(ch) or unicodedata.category(ch) in ('Mn', 'Me', 'Cf'):
        return 0
    return 2 if unicodedata.east_asian_width(ch) in ('W', 'F') else 1
```

**Cross-checked against a real emulator.** `pyte` 0.8.2 happens to be installed
here, so every case was rendered twice — once by the in-house emulator, once by
`pyte` — and after the width fix **88 of 88 screens and 88 of 88 cursor positions
agree**. `pyte` is not a dependency of the plan (it chokes on vim's private SGR
`\x1b[>4;2m` and has to be fed a filtered stream); it is the oracle the in-house
one was proved against, and the repository's tools stay standard-library-only.

**The two instruments agree on the outcome and not on the intermediate states.**
For all 92 cases of the pty corpus, **the final text area is identical** in pty and
stream mode. Only 66 of 92 *intermediate* screens match verbatim, and the reason is
not a bug: when input is already pending the editor skips redraws, so the stream
contains fewer screens than a pty session where every keystroke group is followed
by a pause. The insert-mode and listing cases are the ones that differ. **The
stream harness records what the editor draws when it has nothing left to read**,
which is the honest thing for it to record.

#### II.2d. How a case gets its text: `:set paste`

Nothing can load text, so a case types it — and typing is subject to the
compiled-in `'ai' 'si' 'et' 'sts=4' 'ts=4' 'sw=4'` and to the four compiled-in
mappings. `:set paste` turns off exactly that interference. Measured on the
committed binary: `O    indented`, `o<Tab>TAB`, `oplain` gives
`"    indented" / "    <Tab>TAB" / "    plain"` without it and
`"    indented" / "<Tab>TAB" / "plain"` with it.

So the shape of a case is:

```
    args = ['+set paste', '+set <whatever the case needs>']
    keys = [ 'i<seed text>\x1b', ':set nopaste\r', <the case's real editing>, '\x1b:q!\r' ]
```

and a case that is *about* `'ai'` or `'et'` simply does its typing after
`:set nopaste`. Both forms were measured: `+set paste` on the command line works in
this harness (`+{command}` runs before the first screen is drawn), and so does a
typed `:set paste\r`.

**`:set nopaste` restores everything, including from non-default values** —
measured, which matters because the five global save slots `p_ai_nopaste`,
`p_et_nopaste`, `p_sts_nopaste`, `p_tw_nopaste`, `p_wm_nopaste` (56420–56424) are
the shape `tools/orphanopts.py` warns about and the buffer-local
`b_p_*_nopaste` fields do the real work:

| | `ai` | `et` | `sts` | `tw` | `wm` | `sm` |
| --- | --- | --- | --- | --- | --- | --- |
| defaults | on | on | 4 | 0 | 0 | off |
| after `:set paste` | off | off | 0 | 0 | 0 | off |
| after `:set nopaste` | on | on | 4 | 0 | 0 | off |
| from `noai sts=0 tw=9`, after paste/nopaste | off | on | 0 | 9 | 0 | off |

`'paste'` also takes the mapping layer out of the way: `vgetorpeek`'s condition at
28207 excludes mapping in Insert and Command-line mode while it is on, so a seed
cannot be rewritten by `map! <char-0xa7> <C-_>`. That is a second reason the option
is harness infrastructure and survives every phase (decision 8), and the corpus has
a case (`map_in_paste`) that records exactly it.

#### II.2e. What is recorded

One file per case:

```
=== incr_hex   exit=0 bells=1 stream=2371B sha=dc2753fe11c84d69 snaps=7
--- stderr: Vim: Warning: Output is not to a terminal / Vim: Warning: Input is not …
--- snap 0 cursor=23,79 bells=0
<24 lines of screen, right-stripped>
--- snap 1 cursor=0,0 bells=0
…
```

* **the screen per redraw** — what the editor drew, in columns;
* **the cursor** — where it left the cursor, which the text does not show;
* **the bell count** — `\x07` is how the editor says a key did nothing, and a
  refusal that beeps is a recording rather than an absence;
* **exit status and stderr** — the process's own answer;
* **the stream's length and sha256** — a tripwire under the screen: it moves when
  the *drawing* changes even though the result does not. It is recorded because it
  is free; a phase that legitimately re-orders drawing declares it.

**One normalisation, and it is padded.** Undo reports how long ago a change was
made ("1 second ago", from `time()`), and that is the only nondeterminism measured
in the corpus. It is replaced by `<ago>` **padded to the width it replaced**,
because the screen is columns: a shorter token moved the ruler into a different
one, and three of eight runs disagreed until the padding went in.

A second normalisation is needed for the argv record and not for the corpus:
`mainerr()` prints the version banner, which carries `__DATE__`/`__TIME__`
("compiled Sep 17 2026 18:19:28"). Three runs of one binary agree; a rebuild would
not. The record scrubs `compiled <date> <time>`.

#### II.2f. Determinism, measured

| | runs | result |
| --- | --- | --- |
| 92-case pty corpus | 8 | identical |
| 92-case pty corpus, all 92 at once under eight concurrent `gcc` | 2 | identical, and identical to the sequential runs |
| 102-case stream corpus | 8 (92 cases) + 3 (102 cases) | identical, **including the sha256 of every byte stream** |
| 111-command stream sweep | 4 | identical |
| 27-invocation argv record | 3 | identical |

Wall times: 102 cases in **6.4 s** against the committed binary, and **0.53 s**
against a binary with the tty warnings and their `ui_delay(2005)` removed (the
phase-2 shape — §II.3, P2). The sweep is 6.3 s, the argv record 5.0 s (of which 5.0 s
is the one `TIMEOUT`). The pty corpus was 1.9 s.

#### II.2g. Sensitivity, measured — six deliberate breaks

A corpus that cannot fail is not evidence (`CLAUDE.md`). Six patched copies of
`zero-vim.c` were built in `/tmp` and run through the stream corpus:

| break | what it patches | cases that move (of 102) |
| --- | --- | --- |
| `do_addsub` returns `FAIL` — `CLAUDE.md`'s canonical break | one line | **11**: the ten CTRL-A/CTRL-X cases and `mb_incr`, and nothing else |
| `CTRL-G` prints nothing (`fileinfo()` call removed) | one line | **1**: `ctrl_g`. **The file-based harness could not see this at all** — no file was written differently |
| `check_changed()` returns `FALSE` (planned phase P10) | one line | **2**: `quit_modified` and `cmd_edit` (`:edit` answered E37 too); and 6 of 111 sweep rows — `edit enew ex quit view visual`, E37 → E32 or success. **Built as phase 94 and it is 1 and 1**: five of the six rows went with `:edit` at phase 91 and `cmd_edit` with them, and `quit` changes message rather than ceasing to exist |
| `ZZ` runs `q!` instead of `x` (planned phase P10) | one line | **1**: `zz_key` |
| one `nv_cmds[]` row deleted under the precomputed index — the real twelve-phase arrow-key bug | one line | **100**, several with a non-zero exit: the editor cannot even quit. The two that do *not* move are `ctrl_c_clean` and `ctrl_c_changed`, which exit before a key is looked up |
| `'ruler'` default off | one row | **100**, each by the same single line, and the same two exceptions |

The first four are the shape a phase check wants: a small, named set — and the
two-case answer for `check_changed()` is the corpus being more exact than the
author, who expected one. The last two say the corpus is *maximally* sensitive to
anything that changes every screen, which is a property to declare rather than to
fix (§II.5, decision 3).

#### II.2h. The corpus: 102 cases

`tools/behaviour.py`'s 67 cases are the model. Of those:

* **50 translate directly** — the CTRL-A/CTRL-X family over every `'nrformats'`,
  autoindent and formatting, insert-mode CTRL-V/CTRL-W/backspace, multibyte
  motions and case changes, substitution flavours, operators and text objects,
  macros, undo/redo, registers, marks, `:move`/`:copy`, `:normal`, and the four
  `+extra_search` cases. The seed is typed under `'paste'` instead of being loaded
  from a file, and the assertion is the screen instead of the written file.
* **7 become "the feature is gone" witnesses** — `retab_gone`, `sort_gone`,
  `filter_gone`, `read_cmd_gone`, `put_expr_gone`, `ff_gone`, `bomb_gone`. whim
  already removed the commands and the options; what the case records now is the
  refusal message, which is more than the file harness could see (it saw only a
  non-zero exit).
* **3 cannot survive at all** — the three that only existed to check what was
  *written*: `'fileformat'`, `'bomb'` and `'binary'` change bytes on the way to a
  file, and there is no file. Two of them remain as option witnesses above; what is
  genuinely lost is any coverage of write-time transformation, and nothing replaces
  it because nothing performs it.
* **7 gain a companion** — `ins_arrows`, `nav_arrows`, `nav_home_end`, `key_Q`,
  `key_gQ`, `map_tab_percent`, `map_e_acute_undo`: keys that were untestable
  without a terminal.

Then 35 cases exist only because there is a screen: `startup` (the empty buffer,
the `~` filler and the ruler), `ruler_move`, `showmode_ins` (`-- INSERT --`),
`term_report`, `scroll_ctrl_f`, `scroll_zz`, `ctrl_g`, `ctrl_g_count`,
`visual_show`, `set_listing`, `hit_enter` (the Press-ENTER prompt as a *recording*
rather than a hazard), `unknown_cmd`, `search_count`, `search_wrap`, `reg_list`,
`mark_list`, `map_list`, `map_in_paste`, `undo_U`, `percent_match`,
`ctrl_c_clean`, `ctrl_c_changed`, and one per feature a phase removes —
`quit_modified`, `zz_key`, `cmd_write`, `cmd_read`, `cmd_edit`, `cmd_file`,
`key_gf`, `reg_percent`. **A phase whose delta no case can see is a phase with no
check**, so each planned removal has a case before the phase is written.

#### II.2i. The Ex-command sweep survives, and gets stronger

The file-based sweep recorded `exit=`, the files left in the cwd, and stderr. None
of the three exists here. What is left is what the editor **says**, which the
per-redraw snapshots make recordable. 111 commands, each typed at `:` after a
three-line seed, in a session of its own:

```
=== write          exit=0   bells=0 snaps=6
    text: alpha | beta | gamma
    msgs: … ;; E32: No file name … ;; :q!
=== quit           exit=0   …  msgs: … ;; E37: No write since last change (add ! to override)
=== file           exit=0   …  msgs: … ;; "[No Name]" [Modified] 3 lines --100%--
=== highlight      exit=1   …  msgs: … ;; -- More --
=== cquit          exit=1   …
```

That is a *message-level* record where the old one was an exit status: retiring
`:write` moves `E32` to `E492`, which the file sweep could not have seen (both are
exit 1). 111 commands in 6.3 s, four identical runs, 27 KB.

Two things it needs, both measured. A listing that pages (`:highlight`) does not
stop for `ESC`, and the session then ran to the timeout; `q` ends the `--More--`
listing, and in Normal mode `q` followed by `ESC` is an aborted recording that
changes nothing (`.` would also work, but it repeats the last change and polluted
the row). And `:stop`/`:suspend` are skipped, as they are today.

#### II.2j. The argv record, and the pty corpus that stays

**argv is not keystrokes**, so it needs its own instrument — `tools/clicheck.py`'s
shape in stream form. 27 invocations, each recorded by exit status, stderr and
whether a screen was drawn at all; three runs identical; 5.0 s. It is the only
thing that can check phases P2, P3 and P4, and today it reads, among others:

```
'-T' 'xterm'    exit=0  stream=2117
'-T'            exit=1  stream=0     Argument missing after: "-T"
'-Txterm'       exit=1  stream=0     Garbage after option argument: "-Txterm"
'-e'            exit=1  stream=0
'-E'            exit=0  stream=0
'-'             TIMEOUT (took over the input and never returned)
'--ttyfail'     exit=1  stream=0
'f.txt' 'g.txt' exit=1  stream=0     Too many edit arguments: "g.txt"
'-R' '-c' '-u' '-i' '-m' '-Z' '--help' '--version'   exit=1   Unknown option argument
```

**A small pty corpus stays**, five scenarios, for the two things a pipe cannot
show: that the window size comes from the terminal (`TIOCGWINSZ` on a real pty
versus the 80×24 fallback) and that raw mode is entered and restored
(`tcgetattr`/`tcsetattr` succeed on a tty and fail harmlessly on a pipe). Those are
`termcheck.py`'s and `ptycheck.py`'s territory and they already exist; what goes is
the *behaviour* corpus's dependence on a pty.

#### II.2k. What `.reference/core-baselines` becomes

Three recordings, replacing today's `behaviour/`, `ref-exsweep.txt` and
`ref-term.txt`:

```
.reference/core-baselines/
    screen/            102 keystroke cases, one file each     (166 KB)
    ref-excmds.txt     111 Ex commands, by the message        (27 KB)
    ref-argv.txt       27 invocations                         (2 KB)
    ref-term.txt       the terminal table                     (pty)
    ptycheck.txt       the five pty scenarios that stay       (pty)
```

**As built it is `ref-pty.txt` rather than `ptycheck.txt`, and it is six directories and
files rather than five**: phase 123 added `memline/`, sixteen cases that build
buffers of 200 to 25,000 lines in the editor, because everything above records a corpus
that allocates **exactly one data block per case** and therefore cannot see the text
layer as a tree at all. `whim.mk`'s `whim-baselines-check` names the shape and not merely
the directory, which is why it noticed. **Adding a part cost four phase checks their
arithmetic** — 92, 96, 113 and 117 pinned the record count as an equality and 108, 118, 119 and
120 as a floor, and only the equalities broke; all four are computed now.

Recorded, as today, **from the frozen `whim-vim.c` input** built with whim's own
compile line, three times, requiring the three runs to be identical — `GOALS.md`
core rule 3, unchanged. Recording them from the pipeline's input is legitimate where
recording them from its current output would not be.

The programs are zero-only files, named by `internal/phase/083/check.go` and
`tools/coredelta.sh` and by nothing whim or slim runs — `tools/zscreen.py` (the
emulator), `tools/zstream.py` (the driver), `tools/zcases.py` (the corpus),
`tools/zexcmds.py` (the command sweep) and `tools/zargv.py` (the invocations) —
which is `GOALS.md` core rule 9's requirement and the precedent `coredelta.sh` set.
The shared `behaviour.py`, `exsweep.py` and `termcheck.py` are not touched, so no
whim or slim key moves. (`termcheck.py` is still untouched, and from phase 88 it is
no longer *run*: zero records the terminal table with `tools/ztermcheck.py`, which
imports it and replaces the one call that passes a file argument.)

**This is a change to phase 83's program**, not a new phase: `internal/phase/083/check.go` is
what records the baselines and what refuses when they differ. It moves phase 83's
implementation digest, so phases 83 and 84 re-run (9 s and a build), and
`.reference/core-baselines` is rewritten once, with the difference named — which is
what the existing refusal text already asks for. `tools/coredelta.sh` reads the new
recordings, and the old file-based harnesses stay exactly where they are, untouched,
for slim and whim (`GOALS.md` core rule 9: a zero tool is a new file, never an edit
to a hashed one).

**There is no case-by-case equivalence with the old corpus, and this plan does not
pretend otherwise.** A file-based case asserted the bytes of a written file; a
keystroke case asserts a screen. The evidence that the new corpus is worth
trusting is §II.2f (it is deterministic, including under load), §II.2c (it agrees with
`pyte`, and with a pty on every case's outcome) and §II.2g (it catches five
deliberate breaks, three of them in exactly one named case, and one of them
invisible to the old corpus).

#### II.2l. Hazards this instrument has

Measured, all of them:

1. **A typed Escape followed by `[`** would be read as a key code, because with the
   whole keystroke file available there is no pause to tell them apart. No case in
   the corpus does it; one that needs to must put the Escape in its own read, which
   the stream harness cannot do — that is a case for the pty corpus.
2. **A command that takes over the input** (`:append` and friends) must be closed
   by the case itself (`.` on a line of its own). Measured: `:append`,
   `inserted`, `.` works, and `:a/:i/:c` use `getexline`, not the Ex-mode line
   reader, so they survive P3.
3. **EOF is an exit, not a quit.** A keystroke file that does not quit ends with
   the editor exiting 1 and clearing the screen. Every case quits.
4. **`:stop`/`:suspend`** need the session of their own (§II.2b).
5. **The version banner carries a build timestamp** and must be scrubbed from the
   argv record (§II.2e).
6. **The bell is part of the record** and the corpus's own quit keystroke
   (`ESC` in Normal mode) rings it. That is deterministic and left in.

#### II.2m. As implemented, in phase 86

The instrument in this section is built: `tools/zscreen.py`, `zstream.py`,
`zrec.py`, `zcases.py`, `zexcmds.py`, `zargv.py`, `zpty.py`, `zrecord.sh` and
`zcompare.py`, 1,144 lines, named by nothing whim or slim runs. `GOALS.md`'s
*Phase 86* is what it does and what it proved. **Five things differ from the design
above**, each because building it said so:

1. **A case's options and `'paste'` go on the command line**, as `+set paste`
   rather than a typed `:set paste` — §II.5.11's recommendation, taken. It removes two
   snapshots per case, and it exercises `+{command}` in all 102.
2. **The corpus is 102 cases, not 92**: the eight "one case per thing a later phase
   removes" (`key_Q`, `cmd_write`, `key_gf`, …) and the two CTRL-C cases were
   written in, so every planned phase has something that can see it.
3. **The record is sectioned** — `--- exit`, `--- bells`, `--- stream`,
   `--- stderr`, `--- snap N` — because `screen-moved` and `stderr-moved` have to
   drop a *dimension* of every record and compare the rest. The design said
   "record both"; the sections are how a comparator is told which is which.
4. **A `-moved` token is itself checked.** Declaring a dimension that did not move
   fails, which the design did not ask for and which the first version accepted:
   with both tokens declared, a difference explained by one of them counted as
   evidence for both.
5. **The pty corpus is four scenarios, not five**, and none of them records a
   screen: `tools/ptycheck.py`'s lesson is that a pty screen dump is a recording of
   the machine's load, so each scenario is an extraction — the size, the term name,
   the line the editing left.

One number in §II.2f moved: the corpus is **0.5 s** here against the phase-2 binary
(the 6.4 s measurement was the phase-1 binary, which still had the two-second
pause), and a whole recording — four corpora and the terminal table, run at once —
is **5.1 s**.

### II.3. The phases

#### II.3a. First, a thing the sweep can no longer do: `ex_ni` is gone

whim's rule 3 was *a command is never deleted from the table*: the row's name still
decided what every abbreviation of every other name meant, so a retired command
pointed at `ex_ni`. **Phase 80 removed `ex_ni` with the 489 stub rows** — measured:
zero mentions in `zero-vim.c`, and a patch that pointed `:write` at `ex_ni` does not
compile.

So a Part II phase that removes a command **deletes its row and its enumerator**. That
is safe now and was not before, for three reasons, each of which has to hold:

* whim 80 gave every surviving row its shortest abbreviation and made a match
  require at least that many characters, so **row order is irrelevant** and no
  removed name can be inherited by the next row (`SLIM-GOAL.md`'s `:help` →
  `:helpclose` trap cannot fire);
* `cmdnames[]` is **designated** (`[CMD_write] = {…}`), so deleting the enumerator
  and the row together cannot misalign them, and
  `static_assert(sizeof(cmdnames)/sizeof(cmdnames[0]) == CMD_SIZE)` catches a
  dropped one either way — measured: deleting six rows without their enumerators
  fails the assertion at compile time;
* `tools/create_cmdidxs.py`'s `names()` **refused a table with fewer than 100
  rows**, and it is what the command sweep enumerates. 111 − 12 = 99. **The floor
  had to be lowered deliberately, in the phase that crosses it**, or the sweep
  stops working with a message about a parse that cannot be right. *Done, as
  phase 91*, which took the table 104 → 99: the floor is **80** and the
  message the old one gave was not the one this line predicts — `no command table
  found in either shape`, because `names()` tries both parsers with `check=False`.
  Twenty-eight implementation keys moved with it, gated on `slim-verify` and
  `whim-verify`.

`nv_cmds[]` rows are the opposite: they are **pointed at `nv_error`, never
deleted**, and `tools/nvidxcheck.py` requires the precomputed index to stay a
permutation — §II.2g's fifth break is what happens otherwise.

#### II.3b. The eleven phases

| # | phase | removes | frees | delta in the new corpus |
| --- | --- | --- | --- | --- |
| 2 | nothing asks whether this is a terminal | the two warnings, `ui_delay(2005)`, `tty_fail`/`--ttyfail`, `stdout_isatty`, `mch_check_win`, `mch_input_isatty`, the four `isatty()` calls, `check_tty` | **`isatty`** | every record's stderr line (`stderr-moved`); measured on a patched build: 102 of 102 records move, each by one line, and the corpus goes 6.4 s → **0.53 s** |
| 3 | no streaming Ex | `-e -E -s -v`, `Q`, `gQ`, `do_exmode` (95 lines), `getexmodeline` (262), `silent_mode` (23 mentions), `exmode_active` (49 mentions, constant `FALSE`), `pending_exmode_active`, `s_vbuf`, `main_loop`'s `noexmode` | **`setvbuf`** (and the `stdout` reference) | `key_Q`, `key_gQ`; argv rows `-e -E -s -v` |
| 4 | argv is `+{command}` and `-T {term}` — **built, as phase 88** | the file argument and `buflist_add`, bare `-`/`EDIT_STDIN`, `--`, `ME_TOO_MANY_ARGS`, `had_minmin`, `read_cmd_fd`'s reassignment, and `params.edit_type` with `read_stdin()` | — | **as listed, plus `+q! f.txt`**: 79 lines, six argv rows, nothing else |
| 5 | no write — **built, as phase 89** | the six rows and their enumerators, `nv_Zet`'s `ZZ`, `do_one_cmd`'s `:w>>`/`:w!` parse; the sweep then takes `do_write`, `buf_write`, `buf_write_bytes`, `check_overwrite`, `check_writable`, `check_mtime`, `not_writing`, `write_eintr`, `mch_setperm`, `mch_fsetperm`, `mch_nodetype`, `vim_fexists` and seven more — **19 functions; the file 85,734 → 84,675** | `chmod fchmod fstat ftruncate lstat unlink`, exactly | **`cmd_write` and `zz_key`; the six sweep rows CEASE TO EXIST, they do not change message** |
| 6 | no read — **built, as phase 90** | the `CMD_read` row and enumerator and `do_one_cmd`'s `:r!`/`:r !cmd` parse; the sweep then takes `ex_read`, `do_bang`, `do_shell`, `do_filter`, `check_secure` and `prevcmd_is_set` — **6 functions exact; the file 84,675 → 84,453**, which is 222 lines and not 194, the extra being the `usefilter` fold below | — | **`cmd_read` and `read_cmd_gone`; the sweep row `read` CEASES TO EXIST — but NOT `filter_gone`**, which was E492 on the input binary already |
| 7 | no `:edit`, and no `gf` — **built, as phase 91** | the five rows and enumerators, `do_one_cmd`'s `curbuf_locked()` exemption for `:edit` and its `++opt` parse, and the `gf`/`gF` and `[f`/`]f` **arms** — there are no `nv_cmds[]` rows for them; the sweep then takes `do_ecmd` (328), `do_exedit`, `ex_edit`, `grab_file_name`, `otherfile`, `nv_gotofile`, `text_or_buf_locked`, `check_lnums*`, `prepare_help_buffer`, `getargopt` and six more — **17 functions; the file 84,453 → 83,755** | — | **`cmd_edit` and `key_gf`; the five sweep rows CEASE TO EXIST** |
| 8 | nothing reads a byte — **built, as phase 92** | `open_buffer`'s two read arms, its `read_fifo` local and its signature; the sweep then takes `readfile` (787), `read_buffer`, `read_eintr`, `readfile_linenr`, `filemess`, `msg_add_fname`, `msg_add_lines`, `msg_add_eol` and eight more — **16 functions; the file 83,755 → 82,572**. `read_stdin` here is the ARGUMENT, not the function: `read_stdin()` was argv's and went with phase 88. `mch_isdir` and `set_rw_fname` land here and not in row 9 | `open access fcntl`, exactly | **nothing at all**: two full recordings byte-identical. The evidence is an instrumented build, not a record — see below |
| 9 | the buffer has no name — **built, as phase 93** | `:file`'s row, enumerator and BOTH of `do_one_cmd`'s `CMD_file` tests; `buflist_new`'s two name parameters and everything it did with them; sixteen folds of `b_ffname`/`b_sfname`/`b_fname` (**32/26/29 mentions, not 58/30/42** — phase 92 had already taken eleven of them); `do_one_cmd`'s `EX_XFILE` call, which reaches zero rows here; `readonlymode` and `b_dev_valid`'s last write; `shorten_fnames`' cwd; and `find_file_name_in_path`'s `FNAME_EXP` arm. The sweep then takes **60 functions** — `setfname`, `ex_file`, `rename_buffer`, `otherfile_buf`, `buf_setino`, `fix_fname`, `ml_upd_block0`, `ml_timestamp`, `eval_vars`, `expand_filename`, `find_cmdline_var`, the whole `ExpandOne`/`gen_expand_wildcards` layer, `vim_FullName`, `mch_FullName`, `mch_dirname`, `mch_getperm`, `shorten_*`, `home_replace_save` and the `ff_*` remnants — with twelve `buf_T` fields and 43 string literals; **the file 82,572 → 80,387**. `buf_spname` and `get_spec_reg` are NOT removed: both survive folded, `[No Name]` being the only answer left. **`mch_isdir` and `set_rw_fname` are not here — phase 92 took both**, and `setfname` had one caller left because of it | `stat getcwd strerror`, exactly — and **only with the `shorten_fnames` and `FNAME_EXP` folds**: `stat`'s last caller is `mch_getperm`, `getcwd`'s and `strerror`'s is `mch_dirname`. **Not `fsync`**, whose only caller is `ui_write` and which is row 12's | **`cmd_file` and the sweep row `file`, and nothing else**: `reg_percent` and `ctrl_g` are byte-identical, `b_fname` having been NULL since row 4 and `[No Name]` already what they printed |
| 10 | `:q` quits, `ZZ` is `ZQ` — **built, as phase 94** | ONE fold, of `ex_quit`'s refusal. This row names three functions and **sixteen** go: the five of the refusal — `check_changed`, `check_changed_any`, `no_write_message`, `no_write_message_nobang` and `not_exiting` — and then **eleven nobody foresaw**, `check_changed_any()`'s tail being the last caller of the whole switch-buffer/switch-window island: `add_bufnum`, `set_curbuf`, `enter_buffer`, `win_enter`, `win_enter_ext`, `goto_tabpage_win`, `goto_tabpage_tp`, `get_winopts`, `find_wininfo`, `buflist_findfpos` and `buflist_getfpos`. Two struct fields go by hand — `w_topline_was_set` and `wi_changelistidx`, write-only afterwards and invisible to `deadfields.py` — and `ex_quit`'s dead tail with them, since `getout()` sets `exiting` itself and never returns. **The file 80,387 → 79,866**; twelve enumerators go and nothing renumbers. **`nv_Zet`'s `:x` is not here — phase 89 took it** | — | **`quit_modified` and the sweep row `quit`, and nothing else**: the measurement below was taken before phase 91, which has since moved `cmd_edit` and removed the rows `edit enew ex view visual`. **`quit` CHANGES MESSAGE rather than ceasing to exist**, unlike every sweep row rows 5 to 9 declared, and the **exit status does not move in the corpus** — every case ends with a trailing `:q!`, so the 1 → 0 is a probe (`q_alone`) and not a record |
| 11 | the options nothing reads — **built, as phase 95** | The set is COMPUTED and it is **seven**, not four: `'fsync'` `'modified'` `'prompt'` `'readonly'` `'undoreload'` `'write'` `'writeany'` have no reader of their own global, and SIX are dropped. **`'prompt'` is missing from this row** — its only reader was `getexmodeline()`'s `if (p_prompt) msg_putchar(':');`, so it is phase 87's orphan. **`'modified'` stays** (decision 5, and `dropoptions.py` refuses a PV_BUF row). `'readonly'` is live code and not an inert row: `change_warning()` and its six calls, the `[RO]` in `fileinfo()` with its format string, the `[RO]` on the status line, and `did_set_readonly()`. **The flag letters are NOT touched** — see below. The sweep then takes `SHM_RO`, `BV_FS`, `BV_RO`, `w_readonly`, `b_did_warn` and the six globals; **the file 79,866 → 79,757**, and `options[]` 114 → 108 rows and 102 → 96 globals | — | none, measured: two full recordings byte-identical. `tools/dropoptions.py --strict` is the check for the four `PV_NONE` rows ONLY; `'fsync'` (PV_BOTH) and `'readonly'` (PV_BUF) are refused on the PV_ guard before the reader test is reached, and `tools/droplocal.py` is the tool and the check there |
| 12 | no `FILE *` that is never opened — **built, as phase 96** | `scriptin[]`, `redir_fd` and `ui_write`'s `console`, as stated — and with them `closescript()`, `using_script()`, `redirecting()`, `vim_fsync()` and **`redir_write()`**, which this row omits, plus `curscript`, `NSCRIPT`, `saved_typebuf[]`, `redir_off` and the two hand-folded locals `script_char` and `retesc`. `may_sync_undo()` and `is_safe_now()` SURVIVE one conjunct shorter. **The file 79,757 → 79,603**, and `FILE` is not named in `zero-vim.c` at all afterwards | **`fclose getc putc fsync`**, and this row is wrong twice: **`fputs` does NOT go** — the source names it nowhere and `nm -u` still lists it, gcc lowering `fprintf(stderr, ...)` to it — and **`fsync` is here, not in row 9**, its only caller being `vim_fsync()` and that function's only caller `ui_write()`'s `console` branch | none, measured: two full recordings byte-identical. The evidence is an instrumented build at FIVE places, 0 of 106, with the `ui_write()` control at 105 of 106, and eighteen adversarial sessions |

**EVERY ROW IS BUILT, and the numbering is not the table's.** Row 2 ran as zero
phase 85, row 3 as zero **phase 87**, row 4 as zero **phase 88**, row 5 as zero
**phase 89**, row 6 as zero **phase 90**, row 7 as zero **phase 91**, row 8 as zero
**phase 92**, row 9 as zero **phase 93**, row 10 as zero **phase 94**, row 11 as zero
**phase 95** and row 12 as zero **phase 96**, because the harness switch of §II.2 landed
between rows 2 and 3 as phase 86. This plan is done; what is left of it is §II.4c. `GOALS.md` is what each one did; where this
table turned out to be wrong is said at the row.

##### P2 — nothing asks whether this is a terminal

Decision 7. `check_tty()` (86432) is the whole of it: it prints
`"Vim: Warning: Output is not to a terminal"` and
`"Vim: Warning: Input is not from a terminal"` to stderr and then, unless
`--ttyfail` was given, sleeps 2,005 ms. Measured on the committed binary with both
fds piped: the two lines, `ui_delay` costing 2.007 s, and exit 0; with
`--ttyfail`, the same two lines, no delay, exit 1.

Measured by the user on the same binary: the phase takes the object's undefined
symbols from **79 to 78**, the one that goes being `isatty`; and it makes the
102-case corpus cost **0.53 s instead of 6.4 s**, because the delay was two
seconds of every case.

Both go, along with `stdout_isatty` (3592, 6 mentions), `mch_check_win` (its
`isatty(1)` is the only thing it does, and `common_init_2`'s assignment at 86008
goes with it), `mch_input_isatty` (61535), `mch_get_shellsize`'s
`if (!isatty(1) && isatty(read_cmd_fd)) fd = read_cmd_fd;` (61888 — `fd` stays 1),
and `fill_input_buf`'s `!did_read_something && !isatty(read_cmd_fd)` arm (82807),
which reopened fd 0 from stderr when the first read came back empty. `tcgetattr`
and `tcsetattr` stay: a real terminal still needs raw mode, and on a pipe they fail
harmlessly.

**One behaviour question, and it had to be measured rather than reasoned.**
`nv_esc` (52449) computes `int out_redir = !stdout_isatty;` and, on a non-tty,
either prints `"Type  :qa!  and press <Enter> to abandon all changes and exit Vim"`
to stderr or runs `do_cmdline_cmd("qa")` — a command whim removed in Phase 46, so
it would answer E492. Folding `stdout_isatty` to `TRUE` sends both to the screen
instead. Measured: **neither is reachable in this harness.** That branch is behind
`cap->arg`, which is `TRUE` only for the `CTRL-C` row, and CTRL-C on a stream
exits through `preserve_exit()` — recorded screen `"Vim: Finished."`, exit 1 — before
`out_redir` matters. The corpus has `ctrl_c_clean` and `ctrl_c_changed` to pin
exactly that, and the declared delta for the fold is **none**.

##### P3 — no streaming Ex

Decision 3. `do_exmode` (95 lines) and `getexmodeline` (262) go; `Q`'s `nv_cmds[]`
row is pointed at `nv_error` and `gQ`'s arm of `nv_g_cmd` goes; `exmode_active`
becomes constantly `FALSE` and its 49 mentions fold, as `silent_mode`'s 23 do; and
`-e`, `-E`, `-s` and `-v` leave `command_line_scan`.

**The hazard: `exe_commands()` must not go with them.** The `+{command}` list is run
from `vim_main2()` (85903) by `exe_commands()` (86502), which touches neither Ex
mode nor silent mode — but it sits two lines from the `if (exmode_active)` that sets
the cursor to the last line, and `+cmd` is decision 8 infrastructure that the whole
harness depends on. The phase check runs the argv record, where five rows exercise
`+cmd`, and refuses if any of them stops working.

`getexline` (23843) **stays**: `:append`, `:insert` and `:change` read their lines
through it, not through the Ex-mode reader, and measured they still work
interactively. `print_line`'s `silent_mode` save/restore (16326) folds, and
`msg_puts_printf` stays — `msg_use_printf()` asks whether the *screen* is usable,
not whether the editor is silent, so `printf` survives this phase.

**As built, in phase 87**, and three things in the row above are wrong.
`-s` **alone does not move**: `case 's'` set silent mode only `if (exmode_active)`
and called `mainerr()` otherwise, so a bare `-s` was already an unknown option and
its record is byte-identical — the declared delta is `key_Q`, `key_gQ` and the argv
rows `-e`, `-E`, `-e -s`, `-v`. **`isatty` is freed here, not by P2**: phase 85 kept
`check_tty()`'s `if (exmode_active)` branch deliberately, so all five calls survived
it, and folding that branch here empties the function — which the sweep then cannot
take, because what is left is a local that is set and never read, a warning
`tools/deadsweep.py` does not act on. `check_tty()` and its call go by name and
`mch_input_isatty()` with the fifth `isatty()` follows. And `exe_commands()` did not
need special handling beyond folding its last statement. Measured: 86,586 → 85,813
lines, five functions, `nm -u` 80 → 78 (`setvbuf`, `stdout`), the binary 869,512 →
861,288 bytes, the phase 81 s.

##### P4 — argv is `+{command}` and `-T {term}`

Decision 9, **as it read when this was written** — §II.1 records the reversal, and
`-T {term}` is to go in a later phase, leaving `+{command}` as the whole of argv.
What is left of `command_line_scan` after P2 and P3 is `+cmd`, `-T`,
bare `-`, `--` and the file argument; this phase takes the last three. A bare word
becomes `ME_UNKNOWN_OPTION` — *"Unknown option argument: \"foo\""* — and
`ME_TOO_MANY_ARGS` loses both call sites.

**Deleting that enumerator renumbers a parallel table.** `main_errors[]` (85888) is
indexed by `ME_*`, so the row and the enumerator go together, in the order
`deadenums.py` requires: pin the survivors, dump DWARF before and after
(`tools/enumvals.sh`), and require no survivor to have moved.

**As built (phase 88), three things this section did not foresee.** *No
survivor can be pinned*: the enumerator **is** the row index, so the three after
the deleted one must move, and the check is that exactly they moved, by one, out of
1,327 — not that nothing moved. *The file-argument arm is replaced by
`mainerr(ME_UNKNOWN_OPTION)` and not deleted*, because a bare word that matches no
arm never advances the `while` and the parser loops for ever. And *`+q! f.txt`
moves too*, which this row's delta column missed: the option scan reads the whole
command line before anything runs, so a file argument after a `+cmd` is refused
before the `+cmd` is executed.

**And it breaks `tools/termcheck.py`**, which is whim's and asks its question with
a file argument: from this boundary all nineteen of its rows read `(none)`.
§II.2e/§II.2k's "the terminal table, unchanged" was wrong by one argument.
`tools/ztermcheck.py` — `termcheck.py` with its `ask()` replaced and nothing else —
records the same nineteen rows, proven from the binary the phase was handed and
from `whim-vim.c` in phase 83; `tools/termcheck.py` itself is untouched, so no whim
or slim key moves.

##### P5–P9 — the filesystem

The order is forced and the reason is `b_ffname`: `do_write`, `check_readonly` and
`do_ecmd` are its largest readers, so the **name goes last** (P9). Within that, the
write side goes before the read side because `:w !cmd` and `:r !cmd` share
`do_bang`, and `:edit` goes before `readfile` because `do_ecmd` is a caller of
`open_buffer`, which is `readfile`'s caller — one link further out than this line
said, measured as phase 91.

**As built (phase 89), four things this row did not foresee.** *`ZZ` moves
here, not at P10*: `nv_Zet` runs the command string `"x"`, so `case:zz_key` moves
whichever way it is left, and the phase that removes `:x` is the phase that owns
it — it is `"q!"` from phase 89, which is decision 5 arriving early. *The six
sweep rows do not change message, they cease to exist*: whim's Phase 80 removed
`ex_ni`, so a Part II phase deletes the row and the enumerator (§II.3a) and
`tools/zexcmds.py` enumerates 105 names where it enumerated 111 — the column's
`E32/E471 → E492` describes the two screen cases and not the sweep. *The count was 17
functions and is **19***: `check_file_readonly` and `u_update_save_nr` are the two
the reachability simulation of §II.3d missed. Its 905 lines are function bodies and
are not comparable with the 1,059 the file actually lost, which includes the
prototypes, enumerators, blank lines and one struct field the sweep took with
them. And *no
handler is deleted by name*: the four anchors alone produce a byte-identical swept
file, measured against an edit that also deletes the eight handlers, so the phase
program names none of them.

**And the row floor is now live.** §II.3a's warning — `create_cmdidxs.names()` refuses
a table of fewer than 100 rows, and it is what the command sweep enumerates — had
five rows of margin after P5 and **four** after P6, not eleven. P7 spent the rest
(104 → 99) and lowered the floor to **80** in its own commit, as decision 8 says.
The margin is 19 rows and P9's `:file` spent one of them, 99 → 98: 18 left.

**As built (phase 90), four things the P6 row did not foresee.** *`filter_gone`
is not this phase's delta*: `:!` has not existed since whim, so `:%!sort` already
answered E492 on the input binary and its record is byte-identical — what the phase
removes is the code behind a command that was already gone, and the check asserts
E492 on both binaries rather than declaring the case. *The line count is 222, not
194*: the function count is exact at six, and the difference is the prototypes, the
five file-scope variables, the blank lines and the `usefilter` fold. *`do_bang`'s
other caller was `ex_write`'s `:w !cmd`*, which phase 89 swept — so the
ordering this section states (the write side before the read side, because the two
share `do_bang`) is what made the read side a three-anchor phase. And *one fold is
a judgement no tool could make*: `exarg_T.usefilter` is written by nothing once
both `:w !` and `:r !` are gone, and a struct member that is only read draws no
warning and is not what `tools/deadfields.py` removes, so the six surviving tests
and the field go by hand — measured byte-identical in the recording.

**As built (phase 91), five things the P7 row did not foresee.** *There are no
`gf`/`gF`/`[f`/`]f` rows to remove*: the four keys are **arms** inside `nv_g_cmd()`
and `nv_brackets()`, whose `g`, `[` and `]` rows dispatch dozens of other keys, so
this phase deletes no `nv_cmds[]` row at all and the usual hazard does not apply —
fifty of those keys were pressed on both binaries and exactly four moved. *There is
a sixth anchor the row does not list and it pays for itself*: `EX_ARGOPT` reaches
zero rows here, so `do_one_cmd`'s `++opt` block, `getargopt()` and
`exarg_T.read_edit` all go, for 30 lines and a **byte-identical** recording. *There
is one anchor outside the table and the keys*, `do_one_cmd`'s `curbuf_locked()`
exemption, which names `CMD_edit` in a conjunct and keeps `CMD_file`; an edit shaped
like the table forgets it and the build catches that. *The function count is 17, not
16, and the line count 698, not 618* — the seventeenth is `getargopt` and the
difference is prototypes, enumerators, blank lines and two struct fields. And *the
`:edit` row is `:ex` and `:visual` too*: `do_exedit` is thirty lines, so `:ex! f` and
`:visual! f` load a file exactly as `:e! f` does, `:view! f` loads it with
`'readonly'` and `:enew!` empties the buffer — measured on the input binary, because
no recording here can see a file being opened.

**As built (phase 92), five things the P8 row did not foresee.** *`read_stdin`
in the row is the ARGUMENT and not the function*: `read_stdin()` was argv's and went
with phase 88, and what this phase removes is the parameter of `open_buffer()`
and `read_buffer()` — which needs the signature fold, because `-Wno-unused-parameter`
makes an unused parameter invisible to the sweep where an unused local is not.
*`mch_isdir` and `set_rw_fname` land here, not in the name row*, and `set_rw_fname`
being `setfname`'s second caller is what makes P9 possible at all. *`fsync` is not
row 9's*: its only caller is `ui_write`, so it belongs to row 12 — the row above is
corrected. *The count is 16 functions and 1,183 lines*, against the nine names the
row listed; the difference is eleven the sweep found under them, fifteen prototypes,
three file-scope strings, seventeen enumerators and 24 string literals — the whole
message layer that reported what had been read. And *the delta is nothing at all,
which is a statement rather than an omission*: the read path stopped being reachable
at phase 91, so the phase removes code that could not run, two full recordings
are byte-identical, and the evidence is an instrumented build — the input source
compiled twice, with `write(2, …)` first in `readfile()` (0 of 106 records marked)
and then in `open_buffer()` (104 of 106, the identical instrument). The row's
"probed by the argv record and by `startup`" could not have worked: those records do
not move.

**As built (phase 93), six things the P9 row did not foresee.** *The three
name fields are 32/26/29 mentions, not 58/30/42*: phase 92 took eleven of them
with `readfile()`, and the count the row carried was whim-vim's. *`buf_spname()`,
`buf_get_fname()`, `get_spec_reg()`, `get_trans_bufname()`, `fileinfo()` and
`check_fname()` are NOT removed* — the row lists them among what goes, and every one
survives folded, because `[No Name]` is what they answer and `fileinfo()` still has
three callers. *There is an anchor the row does not list and it is the largest part
of the phase*: `EX_XFILE` reaches zero rows once `:file`'s goes — `:read` was one of
its six and phase 90 took it, four more went with the `:edit` family at phase 91 — so
`do_one_cmd`'s `expand_filename()` call can never be entered, and folding it hands
the sweep 32 of the sixty functions. It is phase 91's `EX_ARGOPT` exactly. *The freed
set is right only with two further folds*: `getcwd` and `strerror` need
`shorten_fnames()` to stop fetching a cwd for a now-empty `shorten_buf_fname()`, and
`stat` needs `find_file_name_in_path`'s `FNAME_EXP` arm folded away — which costs
CTRL-F and CTRL-P their difference, both becoming pure text extraction, and is the
charter reading. *The delta is one case and one row, not four*: `reg_percent` and
`ctrl_g` are byte-identical, `b_fname` having been NULL since phase 88 and
`[No Name]` already what they printed, and `:registers` never printed its `"%` and
`"#` lines at all. And *`buflist_name_nr` must be folded at its callers and never in
place*: folding `buf == NULL || b_fname == NULL` away inside it returns OK with
`*fname` never written, a silent behaviour change in the direction that crashes,
where the truth is that it returns FAIL always.

Two things P9 must decide rather than compute, both already measured — and both
came out as written:

* **`[No Name]` is already the answer.** `buf_get_fname()` (5617) returns
  `_("[No Name]")` when `b_fname` is `NULL`, and `win_redr_status` (11277) already
  draws it through `get_trans_bufname`. Nothing has to be written to give the
  buffer a display name; the name simply never gets set. The status line and
  `CTRL-G` change, and `startup`, `ruler_move`, `ctrl_g` and `cmd_file` are the
  cases that see it.
* **`'isfname'` is not a file option.** Its readers are `buf_init_chartab`,
  `parse_isopt` — and, through `vim_isfilec`, `regatom`, `regrepeat` and
  `regmatch`: the `\f` pattern atom. It stays.

##### P10 — `:q` quits, `ZZ` is `ZQ`

Decision 5. §II.2g measured this delta as 2 corpus cases and 6 sweep rows, and **five
sixths of it has already happened**: `:edit`, `:enew`, `:ex`, `:view` and `:visual`
all answered E37 before reaching their own refusal, and phase 91 removed all
five — so `cmd_edit` moved there, and their sweep rows do not exist to move. What is
left for this phase is **`quit_modified` and the sweep row `quit`**; `zz_key` moved
at phase 89. `check_changed()` is folded away rather than deleted first, because
`ex_quit` and `check_changed_any` call it — `do_ecmd` was the third caller and went
with phase 91.

**Built as phase 94, and three things here were wrong.** *(1)* The removal is
not three functions but **sixteen**, and the eleven this row does not name are the
larger half: `check_changed_any()`'s tail is "go to the buffer that refused", and
after whim removed the buffer list and the window commands that tail was the last
caller of the whole switch-buffer/switch-window island. The editor has no code for
entering a different buffer or window afterwards. *(2)* The row `quit` **changes
message rather than ceasing to exist** — `:quit` still has its row, so
`tools/zexcmds.py` enumerates the same 98 names and compares the block — which makes
it the first sweep row zero has declared that survives. *(3)* `need 94 swept` **is
required**, where the brief that specified the phase said there was none: on the
unswept text `buflist_findlnum()` still calls `buflist_findfpos()` from outside the
island, so the invariant every fold rests on is false and the counted anchor refuses.
Two extras the row does not mention were taken and each is byte-identical in the
recording: the two struct fields that become write-only, and `ex_quit`'s dead tail.

##### P11 — the options nothing reads

Computed, not listed: for each of the 116 non-`t_` rows, the set of functions that
mention its variable, minus the option-table plumbing. Four rows have no reader
outside the phases above — `'fsync'` (`buf_write`), `'write'`
(`not_writing`→`do_write`), `'writeany'` (`do_write`, `check_overwrite`),
`'undoreload'` (`do_ecmd`) — and `'readonly'` keeps only the `W10` warning and the
`[RO]` indicator, which decision 8's exemption does not cover and which this plan
recommends dropping (§II.5, decision 5).

`tools/dropoptions.py --strict` refuses a row while anything still reads its
global, and `tools/orphanopts.py` refuses a global whose initialising row has gone —
whim's Phase 11 left `p_dir` NULL and dereferenced before those guards existed.
**`'paste'` is exempt by decision 8**, and the phase program says so in a comment
that names this plan, so the next person to compute the set does not "fix" it.

**Built as phase 95, and four things here were wrong.** *(1)* The computed set is
**seven, not four**: `'prompt'` is missing from this section, and its only reader was
`getexmodeline()` — so it is phase 87's orphan, and the phase needs a `uses
options:95 streams:87` line this plan does not have. `'modified'` is the seventh and
**stays**, by decision 5. *(2)* *"dropoptions.py --strict refuses while a reader exists,
which is the check"* is true only for the four `PV_NONE` rows. `'fsync'` (PV_BOTH) and
`'readonly'` (PV_BUF) are refused on the **PV_ guard**, before the reader test is
reached and with a message about a segfault at startup rather than about readers;
`--local` plus `tools/droplocal.py` is the pair, and `droplocal.py` is what refuses
while a real reader survives — it did, on `did_set_readonly`, which is why that one
function is removed by name. *(3)* **The `'shortmess'` and `'cpoptions'` letters are
NOT dropped, deliberately.** Each list is a separate string literal from the value, so
removing a letter could not move `:set shm?` or `:set cpo?` — but it turns `:set shm=F`
from silently accepted into `E539`, and nothing in the instrument types `:set shm=`.
That is the change rule 2 exists to prevent. 23 of `'cpoptions'` 60 letters and 14 of
`'shortmess'` 23 are inert afterwards, and the phase makes exactly one more so,
`'shortmess'`'s `r`. *(4)* **`tools/orphanopts.py` has a 100-row floor of its own that
this phase crosses on its first drop** — 102 → 98 → 96 — which does not fail the phase
but fails `tools/coredelta.sh` for every later phase, the same shape as decision 8's
floor arriving from a different table. It was lowered to 80 in the phase's own commit,
the same number and the same argument as `create_cmdidxs.py`'s.

##### P12 — no `FILE *` that is never opened

`scriptin[NSCRIPT]` and `redir_fd` are `static FILE *` that **nothing ever
assigns**: `closescript` calls `fclose` on one, `inchar` calls `getc` on it,
`redir_write` calls `fputs` and `putc` on the other, and all of it is unreachable
in the `can_cindent` sense — a static written nowhere and read everywhere, which no
warning can see. `ui_write`'s `vim_fsync(1)` is the same shape: its `console`
parameter is `FALSE` at the only call site (78780).

**Built as phase 96, and three things here were wrong or short.** *(1)*
`scriptin[]` IS assigned, once, in `closescript()` — to NULL — and `redir_fd` by its
own declaration; "nothing ever assigns" is what the phase must *prove* rather than
assume, and it proves it by requiring exactly those two assignments as exact text
before it folds anything. *(2)* **`fputs` does not go and `fsync` does**: the source
names `fputs` nowhere afterwards and `nm -u` still lists it, because gcc lowers
`fprintf(stderr, "…")` to it, exactly as it lowers `printf` to `fputc`, `fwrite` and
`putchar`; `fsync`'s only caller was `vim_fsync()`, so it belongs here and not to row
9. *(3)* The row omits `redir_write()` itself, which is a no-op after the fold and
goes with its five call sites, and with it `redir_off` — **five** writes and no
reader — and the two locals `retesc` and `did_return`, each read-or-written once and
covered by no warning and no tool. A fourth thing the row could not have known: the
`#include <sys/stat.h>` and `#include <fcntl.h>` that nothing needs afterwards were
**left alone**, because removing them would be the first change to the directive
count and that is the charter's to decide.

#### II.3c. Stages, packages, `need`, `apart`, `uses`

```
phases      83 1 2 3 4 5 6 7 8 9 10 11 12

stage       0
stage       1
stage       2-4
stage       5-7
stage       8
stage       9
stage       10
stage       11
stage       12

need  8  swept      readfile's arms are folded by counting what is left of them
need  9  swept      the b_fname reader set is COMPUTED; on unswept text it shrinks silently
need 11  swept      the option set is computed from the readers that remain
need 12  swept      "nothing assigns this" is a count, and dead code assigns things

apart 4  9          P4's check asserts the one buffer is still named by buflist_add
                   -- WRONG, as built: P4 (phase 88) removes buflist_add itself, and
                   what was measured instead is `apart 87 88`, i.e.
                   the Ex-mode phase's check against the argv phase, which names
                   EDIT_STDIN, had_minmin, buflist_add and ME_TOO_MANY_ARGS as things
                   the argv phase is still to take
apart 7  9          P7's check asserts `:file` still reports a name
apart 9  10         P9's check asserts `:q` still refuses on a modified buffer
apart 4  5          NOT PREDICTED, and measured as built: `apart 88 89`.
                   P4's check (phase 88) states that IT frees no
                   libc symbol, as a cmp against the stage's starting undefined
                   set, and P5 (phase 89) frees six -- so the two cannot share
                   a sweep.  A check that asserts a NEGATIVE about the libc
                   surface is apart from every later phase that frees anything,
                   which is a shape the three predictions above all missed.

package  seed        0
package  build       1
package  terminal    2
package  streams     3 4
package  files       5 6 7 8 9
package  buffers     10
package  options     11
package  tidy        12

uses  streams:86   terminal:85   mechanical  silent_mode's last readers are the warnings 2 removes
uses  streams:87   streams:86    mechanical  -e/-E/-s leave the parser with 3; 4 removes what is left
uses  files:91     files:90      mechanical  open_buffer loses one of its four callers with do_ecmd, removed by 7
                                 -- WRONG as written, and corrected by measurement: it said
                                 readfile's last caller is do_ecmd.  do_ecmd called
                                 open_buffer, not readfile; after phase 91 readfile
                                 still has three call sites and open_buffer three callers.
                                 The two phases are in one package, so the line is a
                                 statement rather than a `uses` -- tools/packages.sh
                                 refuses a `uses` inside one package
uses  files:91     streams:87    mechanical  read_stdin's entry point is the bare `-`, removed by 4
uses  files:92     files:88      mechanical  b_ffname's largest readers are do_write and check_readonly
uses  files:92     files:90      mechanical  and do_ecmd, which 7 removes
uses  buffers:93  files:88      rationale   the protection has no remedy once nothing can be written
uses  options:94  files:88      mechanical  dropoptions --strict refuses 'fsync'/'write'/'writeany' before 5
uses  options:94  files:91      mechanical  'undoreload' is read by do_ecmd, removed by 8 (the core's
                                 numbering); asserted there at exactly 2 mentions with its row
uses  tidy:95     terminal:85   rationale   ui_write's console is FALSE at its one call site either way
```

The three `apart` lines are predictions, not measurements: whim's were found by
running each check against each candidate boundary, and the core's must be found the
same way once the programs exist.

#### II.3d. The cumulative measurement

Reachability over the call graph of all 1,874 functions (roots: `main` plus every
name mentioned outside every function body, as `tools/funcreach.py` computes them),
with each phase's entry points removed:

| cut, cumulatively | functions gone | lines gone | libc freed by reachability |
| --- | --- | --- | --- |
| P5, the write side | 17 | 905 | `chmod fchmod fstat ftruncate lstat unlink` |
| + P6, `:read` | 23 | 1,099 | — |
| + P7, `:edit` and `gf` | 39 | 1,717 | — |
| + P9, the name and everything that reads it | 140 | 4,305 | `+ fsync getcwd strerror` |
| + P8, `readfile` and the stdin reader | 152 | 5,268 | `+ access fcntl open` |
| + P3, Ex mode | **155** | **5,636** | — |

The rows are the order the *measurement* was taken in, not the order the phases
run in: P8's own cut is what is left of `readfile` once P7 has taken a caller of
`open_buffer` — **not of `readfile`**, which is the same correction the `uses` line
above carries, measured as phase 91 — and P9's is the largest of them whichever
side of P8 it falls. **`stat` is P9's, and not for the reason this said**: the last
call site is not `buflist_new`'s naming branch but `mch_getperm()`, reached from
`find_file_in_path()`, so it goes only because P9 also folds
`find_file_name_in_path`'s `FNAME_EXP` arm — measured as phase 93, where the
undefined set moves by exactly `getcwd stat strerror`.

5,636 lines is **6.5 %** of 86,614. The folds add to it and are **estimated**, not
measured: `exmode_active` (49 mentions), `silent_mode` (23), `stdout_isatty` (6),
`readfile`'s surviving arms, the five option rows, the two `FILE *`s — call it 700
lines, for ≈ 80,300.

Symbols: of the 79 the object needs today, **19 go** — `isatty setvbuf open access
fcntl stat lstat fstat chmod fchmod ftruncate unlink fsync getcwd strerror fclose
getc fputs putc` — leaving **60**, of which four (`__errno_location`, `fputc`,
`fwrite`, `putchar`) are gcc's lowering of `printf`/`fprintf` and not written
anywhere in the source. Measured by simulation over the same graph, not by building
the result.

**BUILT, ALL OF IT, AND THIS ESTIMATE IS WRONG THREE WAYS.** The thirteen phases are
done and `zero-vim.c` is **79,603 lines**, not ≈ 80,300 — **7,011 lines, 8.1 %**, where
this said 6.5 % plus about 700. **Seventeen symbols go, not nineteen, leaving 62** as
`tools/symbols.sh` counts and **61** with the core's own `-fno-stack-protector`. Two of
the nineteen are wrong: **`isatty` does not go** — phase 87 removed one of its five
call sites and three survive, all of them the terminal's — and **`fputs` does not go**,
the source naming it nowhere while gcc lowers `fprintf(stderr, "…")` to it. The
eighteen that went, in phase order, are `__stack_chk_fail` (1), `setvbuf` and `stdout`
(4), `chmod fchmod fstat ftruncate lstat unlink` (6), `access fcntl open` (9), `getcwd
stat strerror` (10) and `fclose getc putc fsync` (13). And **`fsync` belongs to row 12
and not row 9**, its only caller being `vim_fsync()`.

**AND THE NUMBER MOVED AGAIN, FOR A REASON THIS SECTION NEVER CONSIDERED.** This whole
estimate is about what *removal* frees. Phases 97 and 98 free 28 more symbols by
**moving code in** rather than out — the strings and memory blocks, the character
classes, the two `ato*`, `qsort` and `bsearch`, defined in `zero-vim.c` as `static`
functions — so `nm -u` is **33** with the core's flags, 34 as `tools/symbols.sh` counts,
and the file is **80,413 lines**, longer than the 79,603 thirteen phases left. Forty-six
of whim's 79 symbols have gone and the eighteen listed above are only the first of
them. The 8.1 % this section was corrected to is 7.2 % now, and a line count is no
longer the measure it was.

### II.4. What remains

**THE PLAN IS BUILT AND WHAT REMAINS IS NOT WHAT THIS SECTION WAS WRITTEN FOR.**
Every row of §II.3b has landed, as phases 85 and 87 to 96; §II.4a and §II.4b below are
corrected from measurement in place. **And §II.4c has landed too**: `main()` is a launcher
(101, 102), the terminal and the signal set are the host's (20), both stream calls went
(104, 118), and the line between the core and the host is the file's first `#include`
(106, 108, 109, 110). **Phase 119 finished the sentence the whole of §II.4 was reaching for**:
the core's own block of plain libc declarations is empty and gone, so it names no libc
function at all — asserted on the cut compiled to an **object**, because a bare `extern`
is invisible to the warning check the boundary otherwise relies on. What the charter
still asks for after that is the **text
representation**, from lines to a tree — and §II.4d, added afterwards, is that same move
measured from the porter's end. **That move is now largely made**, as phases 123 to
128, and §II.4d is rewritten below from measurement rather than left as a forecast: the
memline is a counted tree of nodes holding line records, with no pages, no blocks and no
memfile, and the three constructs §II.4d called untranslatable are gone. **What is left of
the charter's line is the *outer* shape** — the tree is still an array-of-children B-tree
in one flat node type rather than the recursive structure a port would write, and a port
still has to be told what `bhdr_T`'s tag means. **Two phases have landed that are in no
part of this
plan**, and they are named here so the plan is not read as the whole account: 38 cut
`builtin_terminals[]` from ten names to `xterm-256color` and `debug`, and 39 removed
`-T {term}`, so `+{command}` is the whole command line and nothing outside the process
can say what terminal this is. Two smaller things are named here
rather than planned, because each is a decision and not a computation — **and the
first of the two has since been built, which is why its bullet is struck through**:

* ~~**the includes.**~~ **BUILT, AS PART II PHASE 16, AND THIS BULLET IS WRONG TWICE.**
  It says two headers and there are **three** — `<iconv.h>` supplies nothing either,
  and its only occurrence in `zero-vim.c` was its own `#include` line, whim having
  removed the conversion layer and left it. And "79,599 lines and 16 directives" is
  **79,598 lines and 15**: the typedef sits between two blank lines and one of them
  goes with it, which is five lines and not four, and the header count was short by
  the one it missed. As built the phase takes **six**, because phases 97 and 98
  emptied `<string.h>`, `<ctype.h>` and `<wctype.h>` first — **eighteen directives to
  twelve** — and its evidence is a `cmp`: 805,544 bytes either side, both built with
  `SOURCE_DATE_EPOCH=0`. `GOALS.md`'s charter now says a phase may remove a
  directive and may not add one.
* ~~**the clock.**~~ **BUILT, AS PART II PHASES 28 AND 32, AND THE LAST CLAUSE IS WRONG.**
  It took two phases and not one, the elapsed-milliseconds clock crossing as
  `long musl_now_ms(void)` and the wall clock as `long host_time(void)` — and neither
  **frees** anything. `gettimeofday` and `time` are both still in `nm -u`, because a
  symbol leaves when its last *caller* leaves the **file** and both callers moved to the
  other side of a boundary inside one translation unit. What the core no longer does is
  *name* either one; the bullet predicted a symbol count and what it bought was a place.
  The undo message is still the one nondeterminism the corpus has to scrub (§II.2e) — **and
  the scrub does not close it**, which phase 123 measured: the leak is arithmetic on
  the scrubbed text's *width*, an undo reporting its age and the editor then positioning
  the cursor to clear the line, so `0 seconds ago` emits `\033[24;40H\033[K` and
  `1 second ago` emits `\033[24;39H`. Phase 123's own records count `\x1b[?25h` redraws
  instead of digesting the stream, and carry a clock control as the evidence; the other
  106 records still digest, and closing that is expensive rather than hard — the
  `--- stream` line is named in **eighty files**, forty-seven times in phase 95's
  check alone.

#### II.4a. The surviving libc, by reason

**MEASURED after phase 96, and this table was wrong twice and short by one.**
**`isatty` is missing from it and survives**, with three call sites —
`mch_check_win`'s `isatty(1)`, `mch_get_shellsize`'s `!isatty(fd) &&
isatty(read_cmd_fd)` and `fill_input_buf`'s `!did_read_something &&
!isatty(read_cmd_fd)` — so it belongs in the terminal row, making that row ten.
**`fputs` is missing from "gcc's own"**: the source names it nowhere and `nm -u`
still lists it. The predicted total of **60 is 61** (62 as `tools/symbols.sh`
counts, which compiles plain `-O0` and so adds `__stack_chk_fail` that the core's
`-fno-stack-protector` removes). Every other row is confirmed symbol for symbol.
Both corrections are in the table below.

**AND THEN FOUR OF ITS ROWS STOPPED EXISTING.** Phases 97 and 98 are *no musl
dependencies* for the half of the charter that is pure computation: the four rows
struck through below — strings and memory blocks (17, `sprintf` among them),
character classes (7), numbers (2), sorting and searching (2) — are **28 symbols that
are now `static` definitions inside `zero-vim.c`**, and `sprintf` went onto the
editor's own `vim_snprintf` rather than being copied. `nm -u` is **33** with the core's
flags (34 as `tools/symbols.sh` counts). What is left is six rows and not one of them
is a function of its arguments alone: every surviving symbol asks the operating system
something, which is the line this table was always trying to draw.

| why | symbols |
| --- | --- |
| **the terminal** (the whole of the host boundary that is left) | `read` `write` `close` `dup` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` **`isatty`** |
| **messages before and after the screen** | `printf` `fflush` `stderr` |
| **memory** | `malloc` `free` `realloc` |
| ~~**strings and memory blocks**~~ *(phase 97)* | ~~`memchr` `memcmp` `memcpy` `memmove` `memset` `strcasecmp` `strcat` `strchr` `strcmp` `strcpy` `strlen` `strncasecmp` `strncmp` `strncpy` `strpbrk` `strstr` `sprintf`~~ |
| ~~**character classes**~~ *(phase 98)* | ~~`isalnum` `iscntrl` `ispunct` `tolower` `toupper` `towlower` `towupper`~~ |
| ~~**numbers**~~ *(phase 98)* | ~~`atoi` `atol`~~ |
| ~~**sorting and searching**~~ *(phase 98)* | ~~`qsort` `bsearch`~~ |
| **time** | `time` `gettimeofday` |
| **signals and exit** | `sigaction` `sigaddset` `sigemptyset` `sigismember` `sigprocmask` `kill` `raise` `getpid` `exit` `_exit` |
| **gcc's own** | `__errno_location` `fputc` **`fputs`** `fwrite` `putchar` |

#### II.4b. What is still file-shaped afterwards

Almost nothing, and the residue is nameable:

* **`read`/`write`/`close`/`dup` on fds 0, 1 and 2.** After P8 there is no `open`,
  so the process cannot acquire a fourth descriptor — an invariant a phase check can
  assert mechanically (no `open`/`creat`/`openat`/`mkdir`/`rename`/`unlink`/
  `readlink`/`opendir` in the source, and none in `nm -u`). **It is asserted from
  phase 93 on**, which is where the last of them went: `access fcntl open` at
  phase 92 and `getcwd stat strerror` at phase 93, and phase 93's check requires all
  eleven to be absent from `nm -u` while `read close dup fsync` stay.
  **Phase 96 makes it stronger and it is right rather than merely holding**:
  there is no stdio stream either. `FILE` is not named in `zero-vim.c` at all (2 → 0)
  and `fclose getc putc fsync` are gone, so the invariant names `fopen fdopen fclose
  getc putc fsync` beside `open creat openat stat access fcntl getcwd strerror
  opendir` — and phase 96's check asserts every one of them absent from **both** the
  source and `nm -u`. The core can read, write, close and dup fds 0, 1 and 2 and
  nothing else.
* **Nothing phases 97 to 99 did touches this.** Those three free 28 symbols and
  remove six headers, and not one of the 28 and not one of the six is file-shaped:
  their checks state the surviving set as a `cmp`, so a file symbol *arriving* would
  fail as loudly as one leaving. Two of the six headers are the ones a file-opening
  core would have needed — `<sys/stat.h>` and `<fcntl.h>` — so from phase 99 the
  invariant is visible in the directive list as well as in `nm -u`.
* **`time()` and `gettimeofday()`.** `vim_time()` feeds undo's *"1 second ago"* and
  the command-line history's timestamps; `gettimeofday` times `:sleep`, the bell,
  key timeouts and OSC replies. The undo message is the one nondeterminism the
  corpus has to scrub (§II.2e), which is a hint: *"the editor stops asking what time
  it is"* is a phase, and it frees `time`. Not in this plan.
* **`exit`/`_exit` and the signal set.** whim's Phase 26 argued five signals are
  the minimum; a core that does not own the process wants none of them, and that is
  the host's business rather than a filesystem question.
* **The swap file's BOOKKEEPING outlived the swap file by twenty-nine phases**, and
  that is the residue this section did not name because no tool could see it. Every
  syscall went at 89 to 96 and the memline went on keeping a header block with the
  editor's version and the buffer's name, a translation table for blocks not yet
  written out, a three-valued dirtiness and a record of where each block's lines used
  to be — **all of it written and none of it read**, which is exactly the shape
  `tools/deadfields.py` cannot report (it matches a field named nowhere outside its
  own type) and gcc has no warning for. Phase 125 took it, naming every field
  itself, and measured the negative half as phase 92's kind — four markers on the
  negative-block island fire in **0 of 252 records** against a control of identical
  shape firing 2,141 times — and the block-zero half as phase 95's, three markers
  firing in 227, 214 and 227 records with two full recordings byte-identical anyway.
  **The lesson generalises past this phase: an invariant asserted over `nm -u` and
  over the source's *calls* says nothing about state that is merely maintained.**

#### II.4c. Toward the host boundary — a sketch, not a plan

Once argv is two options and nothing is loaded or saved, `main()` is a few lines of
terminal setup, a size query, and `vim_main2()`.

**THERE IS NO SPLIT INTO TWO FILES. IT IS ONE FILE WITH TWO PARTS, AND THE FIRST
`#include` IS THE BOUNDARY** — the user's design, 2026-09-18, replacing both earlier
readings of this section:

```
zero-vim.c   upper part   the core editor.  NO PREPROCESSOR SYNTAX AT ALL.  At its
                          top, the musl_-prefixed prototypes: its calls to the host.
             ----------   the first #include IS the boundary, and nothing else marks it
             lower part   the host.  The #includes, then the musl_ definitions,
                          host_exit, host_message, and main().
```

Everything stays `static` except `main`, which lives in the lower part. **When the
project concludes the product is the upper part**, taken up to and not including the
first `#include` — so the file that goes to the next repository is a *prefix* of this
one, extracted by a rule with no judgement in it.

This dissolves the problems the two-file version created rather than solving them,
and each was measured before the design was taken:

- **The boundary needs no marker.** It is a directive that has to be there anyway.
  No comment — which the no-comments rule forbids — no sentinel declaration, no data
  file naming the boundary functions.
- **"Nothing is global but `main()`" survives untouched**, because there is still one
  translation unit. `tools/phasecheck.sh`'s `grep -v '^main$'` and
  `tools/funcreach.py`'s `{'main'}` root need no change, and the 130 implementation
  keys that changing `phasecheck.sh` would have moved are not spent.
- **The dead-code sweep survives intact.** One translation unit means
  `-Wunused-function` stays a boolean and `funcreach.py`, `typereach.py` and
  `deadsweep.py` work exactly as they do today. MEASURED that the two-file version
  would have broken this: `funcreach.py` on `editor.c` alone deletes **49 functions
  and 1,080 lines** — the editor's whole startup — because its root is `{'main'}` and
  `main` leaves; and `-Wunused-function` is blind to a dead *external* function, so
  none of `-flto`, `-fwhole-program` or a plain `cat` recovers it.
- **No tool changes at all.** `PSOURCE`, `whim.mk`'s single `tar -xO`, `score.sh`,
  `symbols.sh`, `sweep.sh`, `coredelta.sh` and `zrecord.sh` all keep working on one
  file, and the eleven files a split would have had to teach about two products stay
  as they are. Two planned phases — a tooling phase and a join/split tool pair — drop
  out entirely.
- **The build stays one `gcc` invocation.**

**The check that falls out of it is stronger than anything the two-file design had:
cut the file at the first `#include` and compile the upper part with
`-fsyntax-only`.** It must pass with no diagnostics, which proves the core is
header-free and self-contained — and it cannot pass vacuously, because a core still
needing a header fails loudly.

**`NULL` becomes `nullptr`, and there is nothing to declare.** Settled by the user,
2026-09-18, in place of an earlier `enum { nil = 0 };` which this supersedes
entirely. `nullptr` is a **C23 keyword**, and gcc here defaults to C23 — MEASURED,
`__STDC_VERSION__` is `202311L` — so the core needs no declaration, no enumerator and
no line at all for it. That is consistent with what the file already relies on: `enum
: long` and `static_assert` are both C23 and both already load-bearing here.

**It also disposes of the varargs hazard rather than managing it.** An enumerator
with the value 0 is a null pointer constant everywhere *except* a variadic argument,
where it would pass a four-byte `int` to a callee reading an eight-byte pointer with
no warning from gcc — measured. `nullptr` is typed: MEASURED,
`sizeof(nullptr) == sizeof(void *)`, so a variadic argument is pointer-sized and the
rule the phase would otherwise have had to prove and then assert for ever — *no bare
null constant inside a variadic argument list* — is not needed at all.

**`size_t` becomes `usize`, derived rather than asserted:
`typedef typeof(sizeof(0)) usize;`.** Settled by the user with the same message.
`typeof` is C23 and `sizeof(0)` has type `size_t` by definition, so this *is* `size_t`
on any target — MEASURED with `_Generic((usize)0, size_t: 1, default: 0)`, which holds,
so the two are the same type and not merely the same width. The alternative that was
on the table, `typedef unsigned long size_t;`, is correct on this target and silently
wrong on one where `size_t` is not `unsigned long`; this spelling cannot be. The
distinct name keeps the core from shadowing a name libc typedefs again below the
boundary, and nothing outside forces libc's spelling any more, the vendored
`musl_mem*`/`musl_str*` signatures being the core's own since phases 97 and 98.

**THE CORE IS OPTIMISED FOR TRANSPILATION, NOT FOR PERFORMANCE, AND SO IT MAY NOT
DEPEND ON LATENT COMPILER BEHAVIOUR.** The user's rule, 2026-09-19, and it governs
the phases still to come. The core is a text that another runtime will read; what
matters is that its meaning is on the page, not that gcc happens to compile it well.
Where the two conflict, the page wins — `-O0` was already that trade made once, and
this is the same trade made about *semantics* rather than speed.

It has teeth. `abs` and `labs` were called by the core and **never appeared in
`nm -u`**, because gcc lowers both to inline arithmetic — nothing in the language
promises that, and a compiler that emitted calls would silently have added two libc
symbols to a file whose whole claim is the shortness of that list. Vendoring them as
`musl_abs`/`musl_labs` removes the dependence, and MEASURED it frees no symbol at all:
the phase's value is that the core stops relying on something no standard states.

**It does not follow that every compiler-specific spelling must go, and one measured
case argues the other way.** All nine `__builtin_offsetof` uses in the core are
runtime expressions — `alloc(offsetof(T, tail) + len)`, pointer arithmetic, one
division — and MEASURED, none sits where an integer constant expression is required,
so the plain-C form compiles and agrees:

```c
(usize)&(((struct T *)0)->b)      /* "standard" C, and a null-pointer trick */
__builtin_offsetof(struct T, b)   /* a gcc token, and unmistakable */
```

The first only *looks* portable: it is a null dereference by the letter of the
standard, and a transpiler must pattern-match it out of ordinary pointer arithmetic —
failing silently into plausible nonsense if it does not. The second is one token with
two arguments that a reader of the text can special-case by name. For a target that
has no struct offsets at all, the explicit spelling is the safer one, so the builtin
stays and this paragraph is the reason. The rule is *do not depend on behaviour
nothing states*, not *avoid every extension*.

By that reading the core is clean. `typeof`, `nullptr`, `static_assert`, `enum : T`
and `[[fallthrough]]` are C23 and stated. The only `__attribute__` above the boundary
are `format_arg` on `_()` and `NGETTEXT` and `format(printf, 3, 4)` on `vim_snprintf`
— pure diagnostics that generate no code and that a transpiler drops. And
`__builtin_setjmp`/`__builtin_longjmp` are **below** the boundary, in the host, which
is where platform-specific code is allowed to live and the reason phase 102's choice
was acceptable.

**MEASURED, the size of what is left to do.** Moving the eleven `#include`s down to
just above the host block leaves a core of **79,928 lines** and a host of **235**,
and the compile gives **660 errors, every one of them a libc type or macro the core
takes from a header**: `size_t` 263, `va_list` 11 — which phase 105 has since
taken to 8, in four functions that all go below the boundary, so the core declares
nothing for it — `INT_MAX` 8, `time_t` 5, `sig_atomic_t` 5, and `NULL`. Giving the
core its own declarations for those is the whole of the remaining work, and it is the
same work the two-file version needed — the difference is everything else it was
going to cost.

The executable is still `zero-vim` and `argv[0]` is unchanged, so no harness moves.

The three steps that get there, in the order their measurements suggest: demote
`main()` to `vim_main(void)` and give the launcher the four calls `common_init_1`,
`common_init_2`, `termcapinit`, `screenalloc` now make; replace `mch_write`/
`fill_input_buf` with two function pointers the launcher installs (that is the
whole of `read`/`write`/`dup`/`close`); and move `mch_get_shellsize`,
`mch_settmode` and the signal handlers across, which takes `ioctl`, `tcgetattr`,
`tcsetattr`, `select`, `nanosleep` and the nine signal symbols with them. What is
left in the core after that is the fifth and sixth rows of §II.4a's table — strings,
memory and arithmetic — plus `printf` for the messages that appear before there is a
screen, which is itself a question for the host.

**THAT LAST SENTENCE CAME TRUE EARLY AND FROM THE OTHER DIRECTION.** The rows it
expected to be left in the core — strings, character classes, numbers and sorting —
are **gone from `nm -u` already**, taken by phases 97 and 98 *before* the
launcher was written: they are `static` definitions in `zero-vim.c` now, which is
where this section wanted them, reached by moving code in rather than by moving
`main()` out. So the arithmetic in the destination above is done and what is left of
§II.4c is exactly the three steps: `main()`, the two stream calls, and the terminal with
its signal set. After them the core's undefined set would be `malloc free realloc`,
`time gettimeofday`, `exit _exit` and gcc's five — and `printf`, still the open
question this section named.

**THE FIRST OF THE THREE IS DONE, AND THE ARITHMETIC ABOVE IS ONE SYMBOL OUT BECAUSE
OF IT.** Phase 101 demoted `main()` to `static int vim_main(int argc, char **argv)`
and put a launcher of its own at the bottom of the same file, and phase 102 gave
the core a `static void (*vim_host_exit)(int)` that `vim_main()` installs and
`mch_exit()` calls where `exit(r);` used to be. So `exit` and `_exit` are BOTH already
out of the list this section expected to be left with — `_exit` at phase 100, `exit`
here — and the launcher jumps with `__builtin_setjmp`/`__builtin_longjmp` rather than
`sigsetjmp`, because measured on this tree the library spelling costs two undefined
symbols and a thirteenth `#include` where the builtin costs nothing, and because the
signal mask the process ends with is UNCHANGED by the builtin and would have been
changed by `siglongjmp` (`exit()` was always called from inside the handler with the
handled signal blocked). What is left of §II.4c is the two stream calls and the terminal
with its signal set.

**One fact for whoever writes `editor.c`, measured at phase 99 and recorded nowhere
else.** The twelve `#include`s that survive are not twelve independent dependencies.
`select`, `gettimeofday`, `fd_set`, `FD_SET`, `FD_ZERO`, `FD_ISSET`, `struct timeval`
and every `*_MAX` reach `zero-vim.c` through **no header it names**: they arrive
transitively, from `<sys/param.h>` — whose own contribution is `MIN` and `MAX` —
through musl's `sys/resource.h` → `sys/time.h` → `sys/select.h`, measured with `gcc
-E -H` and with one probe per identifier against each of the eighteen. A host that is
not musl needs `<limits.h>`, `<sys/time.h>` and `<sys/select.h>` written in. Phase 99
recorded it rather than repairing it, because repairing it means adding directives and
the charter forbids that; the failure it leaves is loud — the build stops — which is
the acceptable one.

##### What "no preprocessor syntax at all" costs, measured on the file as it stands

The split is not "move `main()` out". A file with no directives has no libc **types**
and no libc **constants** either, and `zero-vim.c` names a great many of both. Counted
with `grep -owc` on the 80,413-line file:

| what it is | from | mentions |
| --- | --- | --- |
| `va_list` `va_start` `va_arg` `va_end` | `<stdarg.h>`, three of them **macros** | 15, 8, 21, 10 |
| `offsetof` | `<stddef.h>`, a **macro**, and the only thing holding that header | 9 |
| `size_t` / `NULL` | `<stddef.h>` | 430 / 2,564 |
| `MIN` / `MAX` | `<sys/param.h>`, **macros** | 7 / 16 |
| `SIZE_MAX` / `uintptr_t` | `<stdint.h>` | 1 / 1 |
| `errno` | `<errno.h>`, a **macro** for `*__errno_location()` | 3 |
| `EXIT_FAILURE` | `<stdlib.h>` | 1 |
| `struct termios` `struct winsize` `struct timeval` `struct timespec` | the terminal and the clock | 4, 1, 6, 1 |
| `sigset_t` `struct sigaction` `sighandler_T` `SIGHUP`… | `<signal.h>` | 1, 3, 5, … |
| `fd_set` `FD_SET` `FD_ZERO` `FD_ISSET` `TIOCGWINSZ` | `select` and `ioctl`, four of them **macros** | 6, 2, 3, 1, 1 |

So the rule forces a stronger interface than it first appears to ask for: **no libc
type may cross the boundary, only scalars.** `musl_get_winsize(int *rows, int *cols)`
rather than `ioctl` with a `struct winsize`; `musl_set_raw(int on)` rather than
`tcgetattr`/`tcsetattr` with a `struct termios`; `musl_wait_for_input(int ms)` rather
than `select` with an `fd_set` and a `struct timeval`. That is more work than a rename
and it is the right shape anyway — **it is also exactly what makes the file
transpile**, since a JVM target has no `struct termios` either. `size_t` and `NULL`
become a typedef and a constant `editor.c` declares for itself; `offsetof` becomes
`__builtin_offsetof`, which phase 99 already measured leaves `<stddef.h>` dead.

**`va_list` was the one with no scalar workaround, and it is settled: the variadic
layer goes to the host.** Decided by the user on 2026-09-18, against the alternative
of `__builtin_va_list` in `editor.c` — which keeps the letter of the rule, since it
needs no header, but puts a gcc extension in the one file whose point is to be plain
C, and which is the hardest thing here to transpile: a `va_list` walk is type-driven
at run time by the format string, where Java varargs hand you an `Object[]`. The
option that looks worse by this repository's own vendoring principle — a formatter is
pure computation, and phases 97 and 98 established that what the file can compute it
computes — is the one that actually transpiles, and that is what the split exists to
serve. The inconsistency is real and is recorded here rather than glossed.

**From the core side it costs one prototype**, because a *caller* of a variadic
function needs no header at all: `...` is C syntax and only the callee that walks the
list needs `va_list`. MEASURED — `int musl_snprintf(char *, unsigned long, const char
*, ...);` followed by a call compiles `-Wall -Wextra` clean with no directive of any
kind.

**What it costs the core is that the eight wrappers cannot survive**, because C cannot
forward `...` — which is exactly why `vsnprintf` exists beside `snprintf`. MEASURED,
the functions that call `va_start`: `smsg`, `smsg_attr`, `smsg_attr_keep`, `semsg`,
`siemsg`, `vim_snprintf_add`, `vim_snprintf`, `vim_snprintf_safelen`, with
`vim_vsnprintf` and `vim_vsnprintf_typval` taking a list, the latter **734 lines**.
Each wrapper's call sites become two statements — `semsg("E123: %s", x)` into
`musl_snprintf(IObuff, IOSIZE, "E123: %s", x); emsg(IObuff);`, the same buffer they
already use. Counts: `semsg` ~94, `vim_snprintf_safelen` ~11, `smsg` ~10, `siemsg`
~10, the rest ~3, and `vim_snprintf`'s ~66 sites are a pure rename. About **130
sites**.

**It splits into two steps, and the first of them is DONE — phase 105.** The
wrapper expansion ran in one translation unit against the core's own `vim_snprintf`,
taking `va_start` from eight functions to one; the reorganisation then puts the
remaining variadic functions below the first `#include`, and above it there is only
the prototype. The first step's declared delta was nothing at all and was checked as
a byte-identical recording, which is a much stronger position to make the mechanical
edits from than making them while the file is being rearranged.

**As built, and two corrections the phase forced on the paragraph above.** It was
**129** sites, not ~130, and the split by wrapper is `semsg` 94, `vim_snprintf_safelen`
11, `smsg` 10, `siemsg` 10, `smsg_attr` 2, `smsg_attr_keep` 1, `vim_snprintf_add` 1.
And **four functions still hold a `va_list`, not three**: `vim_snprintf`,
`vim_vsnprintf`, `vim_vsnprintf_typval` and **`skip_to_arg`**, the positional-argument
walker, which this section had missed. All four go below the boundary.

**The counts in the table above are now the INPUT figures, not the tree's.** After
phase 105: `va_start` 1, `va_list` 8, `va_end` 3, `va_arg` 21 unchanged. And under the
one-file design nothing "moves to the host" in the sense of leaving the file — the
four functions end up below the first `#include`, in the same translation unit, which
is why the phase freed no symbol and asserted that as an equality.

**The split necessarily ends "nothing is global but `main()`" — and replaces it with
a named list rather than with nothing.** The user's constraint, 2026-09-18: *"of
course we need to introduce some global functions for the interaction between host
and editor, but keep it minimal as necessary."* So the invariant generalises instead
of lapsing. Today's check is `nm --extern-only --defined-only` giving exactly `main`;
afterwards it is **exactly the enumerated boundary and nothing else** — `vim_main`
out of `editor.c`, and the `musl_` set out of `zero-vim.c` — with any name not on the
list a failure. That is a stronger check than the one it replaces, because the list
has to be *written down and argued for* rather than being the single name a linker
happens to need, and every future phase that wants to add to it has to say so.

Two consequences of keeping it minimal, both worth settling before the split rather
than during it. **The boundary should be counted, not just listed**: the 33 symbols
`zero-vim.c` has now are not the same as the number of `musl_` prototypes it will
need, because several collapse — `tcgetattr` and `tcsetattr` become one `musl_set_raw`,
`select` with its `fd_set` becomes one `musl_wait_for_input`, `sigaction`/`sigprocmask`/
`sigemptyset`/`sigaddset`/`sigismember` become a flag the host sets and the editor
reads. Fewer, wider calls in plain scalars is both the minimal set and the
transpilable one. And **the `exit` decision simplifies**: the host callback chosen
while everything was still one file — a `static void (*vim_host_exit)(int)` installed
through a parameter, which named no symbol at all — can become a plain `musl_exit(int)`
prototype once there are two translation units. The function-pointer indirection was
there to avoid a global; with a declared boundary the global is the honest spelling.
Two translation
units mean `vim_main()` and every `musl_` the host provides have external linkage by
construction. Two tools hard-code the old invariant — `tools/phasecheck.sh:64`'s
`grep -v '^main$'` and `tools/funcreach.py:127`'s `{'main'}` root — and the cost of
touching the first was measured while reviewing `exit`: **one appended line to
`phasecheck.sh` moves 118 implementation keys**, 12 whim stages, 82 whim edits, 12
zero units and 12 Part II edits, and no slim key. That is the argument for doing
everything that can be done *while the launcher still lives at the bottom of the
single file*, and splitting once, late.

One smaller consequence: each pipeline's product rule extracts **one** file from the
last boundary's tar (`whim.mk`, and the same shape in `slim.mk` and `whim.mk`). After
the split zero has two, and `make score`, the file census and `whim-pass`'s flags
check all assume one product per pipeline.

#### II.4d. What a JVM target cannot express, measured on the core as it stands

§II.4c's rule is *the core is optimised for transpilation, not for performance*, and the
question that rule implies has been asked of the cut `make editor.c` writes: **which
constructs in the 76,687 lines would a JVM port have to be told about, rather than
translate?** Three answers, and they are of very different sizes.

**THE LARGEST OF THE THREE IS GONE, AS PHASES 126, 127 AND 128.** When this section
was written the memline page was 34 + 45 + 14 mentions of three constructs a JVM cannot
express at all, and it dominated every other finding by an order of magnitude. Measured
on the phase 128 core, each of those counts is now **zero**:

| | phase 122 core | phase 128 core |
| --- | --- | --- |
| `db_index[1]`, declared length 1 and indexed to the line count | 34 | **0** |
| `pb_pointer[1]`, the same idiom | 45 | **0** |
| interior pointers of the shape `(char_u *)dp + start` | 14 | **0** |
| `DB_MARKED`, the top bit of an offset stolen as a flag | 17 expressions | **0**, a real field |
| `memfile` / `mf_*` | the whole layer | **0** |
| `__builtin_offsetof` above the boundary | 9 | **6** |
| right-shift operators | 65 | **59** |

**The rewrite was allowed at all because `DATA_BL` had stopped being a disk format**,
which is the correction this bullet most needed before it could be acted on. There was no
file to write a page to — phases 89 to 96 took every one — so the layout carried no
compatibility constraint and the lines→tree move was free to choose any representation it
liked. That is the **opposite** of `slim-vim.c`, where `tools/deadfields.py` refuses while
`ml_recover()` exists precisely because block zero and the memfile's pages *are* a disk
format there.

What replaced them is five struct definitions a port can read straight off:

```c
struct block_hdr     { short_u bh_id; };
struct pointer_entry { bhdr_T *pe_block; linenr_T pe_line_count; };
struct pointer_block { bhdr_T pb_hdr; short_u pb_count; PTR_EN pb_pointer[PB_COUNT_MAX]; };
struct data_line     { char_u *dl_text; colnr_T dl_len; char dl_marked; };
struct data_block    { bhdr_T db_hdr; linenr_T db_line_count; DATA_LN db_line[DB_LINE_MAX]; };
```

Every array has the length it was given, `PB_COUNT_MAX` is 255 and `DB_LINE_MAX` 64, a
child is a **reference** and not an integer key into a side hash table, and a line's text
is its own allocation whose lifetime is the process's. **Three `static_assert`s hold the
arithmetic in place**, including `PB_COUNT_MAX == (4096 - 8) / sizeof(PTR_EN)`, which
fails to compile if anything narrows the entry.

**What a port still has to be told about the memline is one thing and it is small**: the
node is **tagged, not subclassed**. `bhdr_T` is the first member of both node types, so
`(PTR_BL *)hp` and `(bhdr_T *)pp` are the same address and `bh_id` says which it is, at
**23** cast sites in the core. On a JVM that is two classes and a common supertype, or a
sealed pair, and the cast becomes a type test — a translation the porter has to *choose*
rather than one the C forces. That is the difference from what was here before, where
there was no translation to choose at all.

**Four smaller instances of the `[1]` idiom survive, and they are a different and much
easier shape.** `bt_regprog_T.program[1]`, `buffblock_T.b_str[1]`, `hlname_T.hn_key[1]`
and `msgchunk_T.sb_text[1]` are each a trailing `char_u` array allocated with
`alloc(__builtin_offsetof(T, member) + len + 1)` — six `offsetof` sites between them.
**None of them reads an offset back as an interior pointer and none steals a bit**, so
each is a plain "struct plus a variable-length byte string", which on a JVM is a field
holding a `byte[]`. **One of the four does do a `container_of`**, and that is the piece
to name rather than let a reader discover: `hlname_T` is recovered from a pointer to its
`hn_key` member by subtracting the `offsetof`, at two sites — `hi_key -
__builtin_offsetof(hlname_T, hn_key)` — which a JVM has no expression for at all and
which a port turns into a back-reference or an index.

**And one construct is unchanged and is now the largest of its kind left**: the regexp
backtracking stack builds typed pointers into a byte buffer at a computed byte offset,
`rp = (regitem_T *)((char *)regstack.ga_data + regstack.ga_len);`, at **three** sites
that build one and one more that compares against it. It is a growarray used as a stack
of **variable-sized** records, and a port has to give it a representation of its own
exactly as the memline page needed one. Three sites is a different order of work from the
memline's 93, and nothing in the pipeline has touched it.

**Two porter notes that are the opposite finding**, recorded because each looks like a
hazard and measures as nearly none:

* **`>>` is almost a non-issue.** The cut has **65** right-shift operators, 63 `>>` and
  two `>>=`. **45 carry an explicit `(unsigned)` cast at the shift itself** —
  `(unsigned)(c) >> 3`, `((unsigned)(-(key)) >> 8) & 0xff` — six more are the `~0u`,
  `~0ul` and `~0ull` of the limit enumerators, and the rest are on `long_u`, `hash_T` or
  `uvarnumber_T`. **Exactly three are on a signed operand**, and all three mask
  immediately afterwards: two in `blend_cterm_colors()` on `int default_rgb`
  (`(default_rgb >> 16) & 0xFF`) and one in `mf_hash_grow()` on `blocknr_T mhi_key`,
  which is `long`. So Java's `>>` versus `>>>` decides three expressions in the whole
  core, and in each the `& 0xFF` or `& (MHT_GROWTH_FACTOR - 1)` makes the two agree
  anyway.

  **Re-measured on the phase 128 core it is 59 and TWO**, and the one signed shift that
  went is the memline's: phase 126 deleted `mf_hash_grow()` with the hash table, so
  the only signed shifts left are `blend_cterm_colors()`'s pair, both masked with
  `& 0xFF`. **All six of the shifts that went belong to the swap file and the memfile** —
  three in the memfile hash, taken by phase 126, and three in `long_to_char()`, taken with
  block zero by phase 125 — which is the shape of every finding in this section: the
  constructs a port has to be told about were concentrated in the layer the pipeline was
  always going to rewrite.
* **`%` is NOT**, and a Clojure port must use `quot` and `rem` rather than `/` and
  `mod`. C's `%` takes the sign of its left operand; Clojure's `mod` is the floored
  modulus and is never negative for a positive divisor. `ex_history()` relies on the C
  rule and compensates for it by hand, at two adjacent sites:
  `hist[(hislen + j + idx + 1) % hislen].hisnum` **adds the modulus first** precisely
  because `j` is negative there. Under `mod` the bias is applied twice; under `rem` the
  line means what it means today.

**The core uses no floating point at all**, which is the third answer and the one that
costs a port nothing: it is why nothing above needs to say anything about rounding
modes, `strtod` or a soft-float library. It is met **by construction** and not by any
phase — the configuration is `tiny`, which has no `+float`, and whim removed the eval
layer `float_T` belonged to — and `CLAUDE.md` states it as a tree invariant with what
was measured.

### II.5. Decision points

**Four are settled** (the user, 2026-09-17), and they are marked SETTLED below: 1 and 2
(record the per-redraw screens *and* the stream digest), 3 (add the `screen-moved` and
`stderr-moved` tokens), 5 and 6 (drop `'readonly'` with `W10` and `[RO]`, and delete the
`:file` row, keeping `CTRL-G`), and 8 (lower the row floor deliberately, in the phase
that crosses it). The rest stand as recommendations.

1. **SETTLED — what the corpus records.** Both the per-redraw screens, for reading and diffing, and
   the stream's sha256 as a tripwire beside them. The screens are what a human
   reads in a failure; the digest catches a redraw that draws the same result
   differently, which a phase should have to declare.
2. **SETTLED — per-redraw snapshots**, not only the last screen. It is what makes the message line recordable at all (§II.2c), it costs
   166 KB for 102 cases, and it localises a failure to a keystroke.
3. **SETTLED — the declared delta gains tokens for "everything moved"**: `screen-moved` and `stderr-moved`, in the shape of
   whim's `term-moved`. Measured: P2 moves all 102 records by one stderr line, and
   a `'ruler'` change would move all 102 screens. Listing 102 case names to say
   "the status line changed" is a list, not a check.
4. **Is the Ex-command sweep worth keeping in pty-less form?** Recommendation:
   **yes, and it is now better** — 111 commands, 6.3 s, four identical runs, and it
   records the *message* where the file sweep recorded only an exit status (§II.2i).
   It is also the only instrument that would notice a command silently changing
   which error it gives.
5. **SETTLED — `'readonly'` goes.** Drop the row in P11 with its `W10`
   warning and `[RO]` indicator: after P5 nothing can be written, and nothing but
   `:set ro` can set it. `'modifiable'` (a real protection) and `'modified'` (state
   a host wants) stay. **Done at phase 95 and confirmed by measurement**: the
   premise held — `b_p_ro` had ten mentions and `:set ro` was the only thing that
   could set it — and `'modified'` stays for a second reason this decision does not
   give, that `tools/dropoptions.py` refuses a `PV_BUF` row. `'readonly'` turned out
   to have **two** `[RO]` indicators, `fileinfo()`'s and `win_redr_status()`'s, and
   the W10 warning ended in a one-second `ui_delay`, which is the probe that shows
   the phase removing behaviour rather than a row. After it **nothing anywhere can
   mark a buffer read only**, and `'modifiable'` is the protection that remains.
6. **SETTLED — `:file` goes, `CTRL-G` stays.** Keep `CTRL-G` as buffer info
   (`[No Name]`, the line count, the percentage) and **delete the `:file` row**,
   which exists to rename. `fileinfo()` shrinks rather than going.
7. **`--More--` and the Press-ENTER prompt.** Recommendation: **keep both** and
   record them (`hit_enter`, `:highlight`). They are how the editor behaves when a
   message does not fit, and a phase that removed them would be removing screen
   behaviour, which decision 3 protects.
8. **SETTLED, AND DONE — row deletion crosses `create_cmdidxs.py`'s 100-row floor** (111 − 12 = 99), and the floor is lowered **to 80 in the phase that crosses it**, in the
   same commit, with the reason in the tool's docstring — and never silently.
   *Done as phase 91*, which took the table 104 → 99. Measured: the old floor
   failed with `no command table found in either shape` rather than a count, and the
   edit moved **28** implementation keys — 6 whim stages, 15 whim edits, 2 slim
   phases and 5 Part II phases — every one of which still reproduces its boundary under
   `make whim-verify` and `make slim-verify`.
9. **`ME_TOO_MANY_ARGS` and the parallel `main_errors[]` table.**
   Recommendation: delete the row and the enumerator together in P4, with the
   DWARF before/after check (§P4). The alternative — leaving an unreachable row —
   is the "concept the table has and the code does not" that whim's Phase 18
   argued against.
10. **The five pty scenarios.** Recommendation: **keep them**, and reduce them to
    what only a terminal shows: the window size from `TIOCGWINSZ`, raw mode
    entered and restored, and one arrow-key scenario as a second opinion on
    `nvidxcheck.py`. **As built it is four and now five**, and the fifth is the
    counter-example to the reduction: `sel_arrows` presses a **shifted** arrow, which no
    harness in any of the three pipelines had ever done, and it is the only thing that
    can see `keymodel=startsel` — a compiled-in default that had never worked in any
    build, because `set_options_default()` runs no callback and `km_startsel` exists
    nowhere but `did_set_keymodel()`. **A scenario set reduced to what a terminal shows
    is still only as strong as the keys it presses.**
11. **Do the corpus's seeds use `+set paste` or a typed `:set paste`?**
    Recommendation: **`+set paste` on the command line** for the option setup and a
    typed `:set nopaste` before the real editing — it works (measured), it keeps
    the cmdline echo out of the early snapshots, and it exercises decision 8's
    `+cmd` on every single case.
12. **Whether P2's `out_redir` fold needs its own case.** Recommendation: it has
    two (`ctrl_c_clean`, `ctrl_c_changed`), and they record an exit rather than the
    branch — which is honest, and better than a case that cannot fail.

### II.6. Measured versus estimated

**Measured** (in `/tmp/zpty` and `/tmp/zfs`, against the committed source and
binaries built from it): every line number and function extent quoted; the 79
symbols and every call site of each; the 155-function/5,636-line reachability
result and its five intermediate steps; the freed-symbol sets; the 19-symbol
projection; the whole of §II.2 — eight identical corpus runs, four identical sweep
runs, three identical argv runs, the run under load, the `pyte` cross-check (88 of
88 screens and cursors), the pty-versus-stream comparison (92 of 92 final text
areas; 66 of 92 intermediate screens), the five deliberate breaks, the
`:set paste`/`nopaste` restoration table, the tty warnings and their 2.007 s delay,
the 6.4 s → 0.53 s corpus cost, `ex_ni`'s absence and the `static_assert` failure,
`:append` working through `getexline`, `vim -` hanging for 30 s, `:stop` stopping
the harness's shell, the version banner's build timestamp, and the `<ago>` padding
failure.

**Estimated**: the ≈700 lines the constant folds remove and the ≈80,300-line total;
the binary size after the cut (not attempted); the `apart` lines in §II.3c; and the
per-phase order costs, which nothing has run yet.

### II. Appendix: how the measurements were taken

Everything ran in this worktree (`.claude/worktrees/zeroplan`, branch `zero-plan`)
and in two scratch directories, and **no pipeline file, tool or phase program was
touched**. The binary under test is the committed `zero-vim.c` compiled with the core's
phase-1 line, `gcc -O0 -fno-stack-protector -static -no-pie -s` — 869,512 bytes, 79
undefined symbols, which is what `GOALS.md`'s Phase 84 records.

* `/tmp/zfs` — the call graph. `graph.py` collects every function definition at
  file scope (the brace-at-column-0 shape, plus the `main` header split across two
  lines that `tools/funcreach.py`'s regex misses), what each body mentions, and
  which libc names it names; `sim.py` removes a set of entry points, recomputes
  reachability from the same roots `funcreach.py` uses, and reports functions,
  lines and freed symbols. 1,874 definitions, 186 roots.
* `/tmp/zpty` — the harness. `screen.py` (the emulator and the `?25h` snapshot
  rule), `zpty.py` (the pty driver, kept for the comparison), `zstream.py` (the
  stream driver), `corpus.py` (102 cases), `run2.py`, `exsweep_stream.py`,
  `argvcheck.py`, and the patched sources `b1.c`–`b8.c` for the deliberate breaks.
* The five breaks: `do_addsub` returning `FAIL`; one `nv_cmds[]` row deleted;
  `CTRL-G`'s `fileinfo()` call removed; `check_changed()` returning `FALSE`;
  `nv_Zet`'s `"x"` replaced by `"q!"`; `'ruler'`'s default flipped; and the
  warnings-and-delay-free build used for the 0.53 s timing.

Both scratch directories are temporary by design. What is durable is this document
and, once the phases are written, the programs and their recorded boundaries.

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
- **In-AST editing, revisited.** The phases locate and change constructs by text,
  and a regex anchor can silently match the wrong thing — phase 54's missed two
  `options[]` rows for the pipeline's whole life. Editing the tree instead was
  surveyed and declined, for a reason worth re-reading before it is proposed
  again: `internal/cemit` joins the AST to the source text by byte offset, so a
  mutation that moves the tree leaves the text standing, and deleting a table row
  yields BYTE-IDENTICAL output — an edit that did nothing, which the product gate
  cannot see. `doc/AST-EDITING.md` has the measurements and what would
  unblock it. The cheap half of the idea stands: the AST as a LOCATOR, with the
  text still doing the editing.

When Part I ended at phase 82 this list also held the file-lookup layer, state
on disk and the build-time dependencies. Phases 89 to 96 took every way the
editor reaches a file, and 97 to 119 every libc function the core names, so
those items are Part II's history rather than its future.
