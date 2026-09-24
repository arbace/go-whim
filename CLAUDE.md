# CLAUDE.md

Guidance for Claude Code working in this repository. **This file describes the
tree as it now is**; when the two disagree, this file is wrong -- fix it, by
editing the sentence that became wrong rather than appending one that disagrees
with it. The numbers here are measurements: re-measure rather than adjust them by
reasoning.

## What this is

One pipeline, and the Go toolset it runs:

```
slim-vim.c  --whim-->  whim-vim.c
```

- **The input is one file**, `slim-vim.c`: vim 9.2 as a single C23 translation
  unit, produced by [arbace/slim-vim](https://github.com/arbace/slim-vim), whose
  pipeline removes the preprocessor, the comments and dead code and **changes
  nothing the editor does**. `make` fetches it and vim's `LICENSE` at the commit
  that repository's `main` points to, and records the commit in `src/upstream.sha`.
  It is not tracked here. **Never edit it**; a change to the input belongs in
  arbace/slim-vim.
- **whim** (the `Makefile`) removes capability on purpose, 166 phases from 180,870
  lines to 75,099. It is two arcs, a coda, an empty phase and a lint pair:
  - **phases 0-82** (`GOALS.md` Part I) leave an editor with no runtime to
    install, 84,111 lines at q82;
  - **phases 83-128** (`GOALS.md` Part II) turn it into an embeddable core:
    no filesystem, the host behind a line in the file, no libc the core names, the
    text a tree. `GOALS.md` Part II, *Phases 83 to 128 as they
    stand*, is the account read across and belongs there, not here;
  - **phases 129-162** (Part II too) remove from the core what transpiling it to
    Go (`editor/editor.go`, `internal/gen/FINDINGS.md`) had to work around. All but 142
    were meant to change nothing the editor does; 142 drops the build date from
    the version line.
    `internal/gen/FINDINGS.md` maps each finding to its phase.
  - **phase 163** printed the product in the one canonical spelling phase 0
    seeds with; it is empty now that every boundary is printed that way.
  - **phases 164-165** take what the Go's linters found dead in the C: the
    statements after a jump (164, a general rule) and six stores nothing reads
    (165, named). With the generator's own fixes, `go vet`, staticcheck and
    `gofmt -s` are clean on `editor/`.

  Phase 83 is the line between the two arcs.

  **The test suite is minimal.** Each phase was verified, while it was written, by a
  check program of its own and a delta declared in advance against recorded
  baselines; that whole suite (`internal/check`, `internal/verify`,
  `internal/harness`, every `check.go`, every `delta.md` but phase 80's, the
  baselines and `make whim-verify`) was removed after `448e9a8`, the last commit
  that has it. What proves a change now is two things: `make whim-build-check`,
  the committed product back byte for byte, which sees the text; and `make
  whim-test` (`internal/suite`), which sees the editor -- 44 key sessions
  (`internal/suite/cases.md`) fed on stdin to a build of the working tree's
  `src/whim-vim.c` and to one of HEAD's, required to print the same screens and
  exit the same way, with a control (`" INSERT"` spelled `" INSERX"`) that must
  move at least one of them, and a case that runs out of keys before its `:q!`
  refused. Then the same cases on the GO editor (`editor/` built), required to
  answer exactly as the C candidate does -- the one check that `editor.go` is
  the editor and not only what `internal/gen` writes (the same control moves 41
  of the 44 there). About 5 s, the four builds side by side.

## What a file is called

**Code is `.go`, data is `lowercase.md`, prose is `UPPERCASE.md`**, and the three
mix freely in one directory: `internal/phase/099/` holds `edit.go` and `GOAL.md`. A
data file in Markdown puts its data in a FENCED BLOCK and its notes around it,
and the reader takes the fence and ignores the rest -- `internal/build`'s
`declared()` reads `internal/phase/080/delta.md` that way (the command rows phase 80's
edit cuts), and phase 98's edit embeds `internal/phase/098/musl-ctype.md` and
`musl-case.md` and splices their fenced C in byte for byte. What is left outside
the rule is what Markdown would only obscure: `src/slim.sha` and `src/upstream.sha`, one
digest each, read by `make`.

**A phase is a directory, `internal/phase/NNN/`**, its number in three digits so that they
sort: `GOAL.md`, which opens `# Phase N — …` and says what the phase removes,
why and what was measured, and -- for the 108 phases whose cut is a program of
its own -- `edit.go`, which makes that directory **a Go package**, `pNNN`
(`editlit.go` beside it where the literals are long). The other phases are plan
steps only (`internal/steps`). An edit registers itself with `internal/edit` in
an `init()`, and `internal/phase/registry.go` is what links them in -- `cmd/whim`
imports it blank.

**`doc/GOALS.md`** is what holds for every phase: Part I (phases 0-82: the charter,
what was measured and the declared delta -- a record now, see its opening -- the rules, the sweep, the concept index,
an index of the phases) and Part II (phases 83 on: the core's charter, what is
measured from 83 on, the core's rules -- cited as *core rule N*, and holding beside
Part I's -- *Phases 83 to 128 as they stand*, *Adding a phase*, an index of the
phases, and an appendix: the plan phases 83 onwards were built from, its sections
numbered II.1-II.6 and cited `GOALS.md II.4c`), then *What comes next*.

`src/whim-vim.c` is **produced, not edited**. `src/slim.sha` records the digest of the
`slim-vim.c` the committed `whim-vim.c` came from.

**There is no Python and no agent here.** Every phase is a program; a phase that
refuses stops the pass with its own report, and there is no tier below it to fall
through to. arbace/slim-vim keeps both, for its own pipeline.

## Layout

```
cmd/whim/         the toolset, every tool a subcommand: go tool whim <subcommand>
                   (README.md: each tool, and what each retired script became)
internal/          the Go: cc (the forked C front end), cemit (the canonical printer), sweep, dead,
                   cut/cutil (the cutters), edit (what the phases' edits are
                   written against: the driver, and in shared.go what more than
                   one phase uses), steps (every transformation a phase names, as
                   one table), build (the plan: what each phase does to the
                   source, and the driver that runs it), cmdtab (the Ex command
                   table: its names and the ex_cmdidxs block), score (bytes and
                   symbols, the input beside the product), ccx (the core's pointer casts and evaluation order),
                   reach (what nothing reaches, as a partition with gcc as its
                   control -- a reporter, `go tool whim reach FILE`; it deletes
                   nothing)
internal/phase/    the phases: NNN/ (GOAL.md, and edit.go where its cut is a
                   program), registry.go (every phase with an edit.go,
                   blank-imported so it registers), STAGES.md (the record of
                   the stages there were, and of the measurement that retired them --
                   prose, not a manifest a program reads) and boundaries.md
                   (every boundary's lines, entity counts, binary and nm -u, as
                   `go tool whim build --keep D` and `measure D` give them)
editor/            the core in Go: editor.go GENERATED (make editor/editor.go; never edit it),
                   its runtime crt.go and host host.go by hand
internal/gen/      the generator of editor/editor.go (`go tool whim gen`; `whim
                   skel` runs it by hand, with -bodies for the bodies alone),
                   splice/ (`whim splice`: emitted bodies measured in a copy of
                   editor/), pre/ (`whim pre`: internal/ccx's partitions on an
                   editor.c), sigs.md, CONVENTIONS.md and FINDINGS.md
Makefile           the whole build: fetches the input, runs the pipeline, builds the
                   binaries and the editor
src/               the input and the product: slim-vim.c (fetched, not tracked),
                   whim-vim.c (produced, tracked), their binaries slim-vim and
                   whim-vim, upstream.sha and slim.sha
doc/               GOALS.md (what holds for every phase), AGENDA.md (what is not
                   done, in order), and two surveys: AST-EDITING.md (the phases
                   editing the AST: not taken, to revisit) and GO-IDIOMS.md (how the Go
                   editor could be idiomatic, measured and ranked)
```

**The toolset is `go tool whim`**: `go.mod` declares `cmd/whim` as a tool, so Go
builds it, caches it and rebuilds it when any `.go` moves; every tool is a
subcommand, `go tool whim <name>`, and the `Makefile` calls it the same way.
**The C front end is a fork**, `internal/cc`: modernc.org/cc/v4 v4.29.7 with two
C23 productions added, tracked as ordinary source (`internal/cc/README.md`,
which says how to diff it against upstream). It was composed at build time under `.cache/gofork/`
before, because a patched `vendor/` fails `go mod verify`; a fork under its own
import path has neither problem. Measured: with the patch reversed, `whim
parse whim-vim.c` says *unexpected `<EOF>`, expected `}`*, and `internal/gen` writes
the same `editor/editor.go` byte for byte either way.

## Build

```sh
make                 # all: bin/whim, the editor (editor/ built), through whim-vim.c
                     # (produced only when slim-vim.c moved) and editor/editor.go
make whim-build      # the 166 phases in one process: slim-vim.c -> whim-vim.c
make whim-build-check  # the same, required to give the committed bytes back
make whim-editor-check # refuse a tracked editor.go that is not what internal/gen writes
make whim-test        # the editor on 44 key sessions, required to behave as HEAD's does
make whim-vim        # the C product's binary
make slim-vim        # the input's binary, with the same one line
make score           # bytes to store and symbols to provide, input beside product
make help            # every target, with a line each
```

- **One path.** `make whim-build` is what a moved upstream runs:
  `internal/build`'s plan -- each phase's steps (`internal/steps`), **the sweep,
  after every phase** (there are no stages), and **the canonical print of what is left**
  (`internal/cemit`: one spelling per construct, and NO COMMENTS, of any kind),
  so every boundary that is C is in the one spelling phase 0 seeds with -- applied
  in one process, in memory. Measured: 166 phases, **1,045 s**, 75,099 lines. A
  whole run keeps every boundary in `.cache/boundaries/` (qNNN.c) and seals the
  set with the input's digest (`manifest`).
- **The sweep is one closure** (`internal/sweep`'s `Prune`): the text parsed
  (`cc.Parse`, no type-checking, no gcc), everything reachable from `main` and
  the static_asserts found by name in C's three name spaces, and everything else
  cut -- functions, objects, prototypes, typedefs, tags, members, enumerators, and
  the locals nothing reads (resolved by the parser's scopes). Its guards: no
  member goes while `ml_recover` is defined; a struct filled by position keeps
  every member; nothing is emptied; an enumerator's deletion pins the survivor
  after it to its value. About 3 s a phase. It replaced six deleters looped around
  gcc, and keeps nothing they cut (measured on the product: 5,657 entities
  against 5,662, the five it adds all unused).
  It needs every text it is handed to PARSE, and every one does.
- **`whim-build-check` runs phase by phase, in parallel.** With a sealed set of
  snapshots for the input on disk, it checks that phase 0 seeds the input into
  q000 and that EVERY phase N, run on q(N-1), gives qN -- all phases at once,
  `--jobs N` at a time (default: every core) -- and that the last snapshot is the
  committed `whim-vim.c`. Measured: **79 s** on 64 cores, against 1,045 s in
  order; and a phase whose program was changed on purpose
  (a control) is named and fails the check. That is
  the induction a run in order walks, so it proves the same thing; a phase whose
  program changed breaks its own link and is named. With no snapshots of this
  input it runs the pipeline in order, which writes them. It proves the text,
  not the editor; `make whim-test` is what sees the editor (see *What this is*).

- **`editor/editor.go` is generated** (`go tool whim gen`, `internal/gen` on the cut
  `editor.c`) and tracked. `whim-build` writes it after producing `whim-vim.c`;
  `make editor/editor.go` writes it on its own; `whim-editor-check` refuses a
  tracked file that is not what the program writes. `go tool whim gen` writes only
  when the content differs and never runs make: through `whim-vim.c`'s rule it
  could start a build. The binary is `bin/whim`.

- **The compile line is one line**, `gcc -O0 -fno-stack-protector -static -no-pie
  -s`, for the input, the product and every boundary: an ordinary static
  executable, no stack protector. `internal/build/compile.go` states it for the
  tools (`FlagsFor`, `score`, `measure`), and the `Makefile` as `CFLAGS`/`LDFLAGS`
  for its two binary rules. It moved at phases 83 (`-no-pie`) and 84
  (`-fno-stack-protector`) until those became the line for all; the two phases
  change nothing now.
- **No `-g`**, so a formatting change leaves the binary byte-identical -- the
  cheapest comparison there is. `SOURCE_DATE_EPOCH=0` pins `__DATE__`/`__TIME__`
  when two builds are compared.
- **`gcc` exits 0 with warnings**; the dead-code sweep's compile is judged by
  whether it prints *nothing*, never by its status.

## Temporaries

Every temporary goes in `.tmp/` (gitignored), never the shared `/tmp`: the
`Makefile` exports `TMPDIR` there, so `mktemp`, Go's `os.MkdirTemp` and a
build's work tree all land in it. **Worktrees go in `.tmp/worktrees/`** -- a
subagent's too: `git worktree add .tmp/worktrees/<name>` before launching it,
rather than an isolation flag that chooses its own place, because nothing of
ours belongs inside `.claude/`. Outside `make`, run with `TMPDIR=$PWD/.tmp`.

## The pipeline

A phase is a function of the tree it is handed, so the pipeline is
`p_N = f_N(p_{N-1})` -- 166 of them, in order. **There is no memoize**: its key
was the input boundary's digest and the implementation's together, so a moved
`slim-vim.c` missed every entry by construction.

- **There are no stages.** Every phase is its steps, the sweep, and the
  canonical print; a `sweep` step inside a phase's steps is for an edit that
  reads its own earlier steps' text swept. `internal/phase/STAGES.md` is the
  record of the schedule there was, and of the measurement that retired it.
- `go tool whim build --to N --work D` leaves the tree after phase N; `--keep D` writes every boundary, and `go tool whim measure
  D` counts them (`internal/phase/boundaries.md`).
- **A binary is only ever the build of its source as it stands.** `slim-vim` and
  `whim-vim` stamp the digest of the `.c` they were built from
  (`.cache/stamps/`); as make starts, and whenever a rule rewrites a source, a
  binary whose source no longer matches its stamp is deleted and not rebuilt --
  `make slim-vim` or `make whim-vim` builds it again.
- **A phase touches no shared state**: its scratch and its sweep's file are
  temporary directories of its own, which is what lets the check run phases side
  by side. Two WHOLE builds in one checkout would still both write
  `.cache/boundaries/`; a second one goes in a worktree.
- **The analysis tools report, they do not cut**: `go tool whim reach FILE` is
  what nothing reaches in a text, typed, with gcc as its control and struct
  casts held -- the survey instrument the sweep's closure grew from.

## The core and the host

`whim-vim.c`'s twelve `#include`s are not at the top: **the first one is the line
between the editor core and its host**, marked by nothing else. `make editor.c`
cuts there: a complete translation unit with 0 preprocessor lines, 0 errors under
`-fsyntax-only`, and an interface of exactly the names the host defines --
computed, never listed. The core names no libc function at all, holds no file
descriptor of its own, and uses no floating point. `GOALS.md` §II.4 is the
design.

## Adding a phase

`GOALS.md` Part II, *Adding a phase*, has the process; the next phase is 166.
The pipeline's goal is met; what comes now is the Go editor idiomatic
(`doc/GO-IDIOMS.md`), by the generator where it can and by a phase where the C
is the cause. What a new one takes:
`internal/phase/NNN/` with `GOAL.md`, and `edit.go` in package `pNNN` registering
itself if its cut is a program (a line in `internal/phase/registry.go`); and an entry at
the end of `internal/build`'s `Plan` naming its steps (the sweep follows
every phase). Then `make whim-build` (the product moves, so the tracked `whim-vim.c`
and `editor/editor.go` are rewritten), `make whim-test` against the commit
before it (a phase that removes capability moves cases on purpose: name them),
and whatever further evidence the phase needs, stated in its `GOAL.md`.

## Commit style

A `type: summary` subject, then prose explaining *why*, what was measured and how
it was verified. State deliberate omissions.
