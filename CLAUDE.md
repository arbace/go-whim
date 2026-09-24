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
  that repository's `main` points to, and records the commit in `upstream.sha`.
  It is not tracked here. **Never edit it**; a change to the input belongs in
  arbace/slim-vim.
- **whim** (`whim.mk`) removes capability on purpose, 164 phases from 180,870
  lines to 75,225, and every phase **declares its delta in advance**; the harness
  proves it changed that and nothing else. It is two arcs, a coda and a last print:
  - **phases 0-82** (`GOALS.md` Part I) leave an editor with no runtime to
    install, 84,111 lines at q82; their deltas are measured against
    `.reference/baselines`;
  - **phases 83-128** (`GOALS.md` Part II) turn it into an embeddable core:
    no filesystem, the host behind a line in the file, no libc the core names, the
    text a tree. Their deltas are measured against `.reference/core-baselines`,
    with another instrument. `GOALS.md` Part II, *Phases 83 to 128 as they
    stand*, is the account read across and belongs there, not here;
  - **phases 129-162** (Part II too) remove from the core what transpiling it to
    Go (`editor/editor.go`, `tx/FINDINGS.md`) had to work around. Each declares
    nothing, and its check carries a probe or a byte-identical binary for what
    the recording cannot see. All but 142 change nothing the editor does; 142
    drops the build date from the version line, which only stderr shows.
    `tx/FINDINGS.md` maps each finding to its phase.
  - **phase 163** prints the product in the one canonical spelling phase 0
    seeded with, and the binary is byte-identical.

  `CoreFrom = 83` in `internal/build/plan.go` is the line between the two arcs,
  stated once.

## What a file is called

**Code is `.go`, data is `lowercase.md`, prose is `UPPERCASE.md`**, and the three
mix freely in one directory: `phase/099/` holds `edit.go`, `check.go`,
`GOAL.md` and `delta.md`. A data file in Markdown puts its data in a FENCED
BLOCK and its notes around it, and the reader takes the fence and ignores the
rest -- `internal/verify`'s `Declarations` and `internal/build`'s `declared()` read
`phase/NNN/delta.md` that way, and a phase that declares nothing simply has no
block. What is left outside the rule is what Markdown would only obscure:
`slim.sha` and `upstream.sha` (one digest each, read by `make`), and the C that
is C -- `tools/musl-case.txt` and `tools/musl-ctype.txt`, the definitions phase
98 splices into the tree, and `tools/nolibm_check.c`, which a phase compiles.

**A phase is a directory, `phase/NNN/`**, its number in three digits so that they
sort, and **a Go package of its own**, `pNNN`: `edit.go` (its cut) and
`check.go` (its evidence), `GOAL.md`, which opens `# Phase N — …` and says what
the phase removes, why and what was measured, and `delta.md`, the tokens it
declares with `#` notes, or `# declares nothing`. A phase with more to say
splits it: `editlit.go`, `checkprobes.go`, `checkevidence.go`. Each registers
itself with `internal/edit` and `internal/check` in an `init()`, and
`phase/registry.go` is what links them in -- `cmd/whimtools` imports it blank.
`internal/verify`'s `Declarations` reads a run of deltas in the one grammar both
delta checkers take, and `tools/st.sh delta --list FROM TO` prints it.

**`GOALS.md`** is what holds for every phase: Part I (phases 0-82: the charter,
what is measured and the declared delta, the rules, the sweep, the concept index,
an index of the phases) and Part II (phases 83 on: the core's charter, what is
measured from 83 on, the core's rules -- cited as *core rule N*, and holding beside
Part I's -- *Phases 83 to 128 as they stand*, *Adding a phase*, an index of the
phases, and an appendix: the plan phases 83 onwards were built from, its sections
numbered II.1-II.6 and cited `GOALS.md II.4c`), then *What comes next*.

`whim-vim.c` is **produced, not edited**. `slim.sha` records the digest of the
`slim-vim.c` the committed `whim-vim.c` came from.

**There is no Python and no agent here.** Every phase is a program; a phase that
refuses stops the pass with its own report, and there is no tier below it to fall
through to. arbace/slim-vim keeps both, for its own pipeline.

## Layout

```
cmd/whimtools/     one binary, every tool a subcommand: whimtools <subcommand>
internal/          the Go: cc (the forked C front end), sweep, canon, dead,
                   cut/cutil (the cutters), edit and
                   check (what the phases' edits and checks are written against --
                   the drivers, the reporters, in shared.go what more than one
                   phase uses, and for Part I's checks shell.go's Wsh and
                   partone.go), steps (every transformation a phase names, as one
                   table), build (the plan: what each phase does to the source,
                   and the driver that runs it), verify (the same plan with every
                   check and delta, and the baseline recorder), harness (every
                   recorder), ccx (the core's pointer casts and evaluation order),
                   reach (what nothing reaches, as a partition with gcc as its
                   control -- a reporter, `tools/st.sh reach FILE`; it deletes
                   nothing)
phase/NNN/         a phase, and a package: edit.go, check.go, GOAL.md, delta.md
phase/registry.go  every phase package, blank-imported so they register
phase/STAGES.md       the record the plan was read from: the stages, need and apart,
                   the packages.  Prose now, not a manifest a program reads
phase/boundaries.md every boundary's lines, entity counts, binary and nm -u, as
                   `tools/st.sh build --keep D` and `measure D` give them
tools/             what a check still shells out to -- enumvals, the DWARF
                   control kept as shell on purpose -- and the two wrappers
                   that find the Go binary: st.sh and sweep.sh.  The gate
                   (phasecheck, phasebuild, symbols) is internal/check, canon
                   is internal/canon, the delta, the declarations and score are
                   internal/verify, a recording is internal/harness: each is
                   `tools/st.sh <name>` at a prompt, and a check calls the Go
                   in process
tools/templates/   whim.mk, the makefile phase 0 starts from, and core.mk, the one
                   phase 83 writes over it
editor/            the core in Go: editor.go GENERATED (make editor/editor.go; never edit it),
                   its runtime crt.go and host host.go by hand; `tools/st.sh
                   delta` measures a build of it
tx/                skel (types, globals, signatures and, with -bodies, the bodies),
                   splice (emitted bodies measured in a copy of editor/), pre
                   (internal/ccx's partitions on an editor.c), and the conventions
Makefile           fetches the input and includes whim.mk
```

`tools/gobuild.sh` builds `cmd/whimtools` into `.cache/gobin/<key>/`, keyed on
`go.mod`, `go.sum` and every `.go` under `cmd/`, `internal/` and `phase/`.
**The C front end is a fork**, `internal/cc`: modernc.org/cc/v4 v4.29.7 with two
C23 productions added, tracked as ordinary source (`internal/cc/README.md`,
which says how to diff it against upstream). It was composed at build time under `.cache/gofork/`
before, because a patched `vendor/` fails `go mod verify`; a fork under its own
import path has neither problem. Measured: with the patch reversed, `whimtools
parse whim-vim.c` says *unexpected `<EOF>`, expected `}`*, and `tx/skel` writes
the same `editor/editor.go` byte for byte either way.

## Build

```sh
make                 # all: bin/whim, the editor (editor/ built), through whim-vim.c
                     # (produced only when slim-vim.c moved) and editor/editor.go
make whim-build      # the 164 phases in one process: slim-vim.c -> whim-vim.c
make whim-build-check  # the same, required to give the committed bytes back
make whim-verify     # every phase's check and every declared delta (hours)
make whim-vim        # the C product's binary
make slim-vim        # the input's binary, gcc -O0 -static -s (a static-PIE)
make help            # every target, with a line each
```

- **The build and the verification are two paths.** `make whim-build` is what a
  moved upstream runs: `internal/build`'s plan -- each phase's steps
  (`internal/steps`) and the sweep where the schedule put one -- applied in one
  process, in memory, with no boundaries, digests or cache. Measured: 164
  phases, **1,044 s**, 75,225 lines. It checks nothing about the editor; the
  checks, recordings and declared deltas are `make whim-verify`, which costs
  hours. What holds the plan to the phase programs is
  the product: `whim-build-check` requires the committed `whim-vim.c` back, byte
  for byte, from the committed `slim-vim.c`.

- **`editor/editor.go` is generated** (`tx/gen.sh`, `tx/skel` on the cut
  `editor.c`) and tracked. `whim-build` writes it after producing `whim-vim.c`;
  `make editor/editor.go` writes it on its own; `whim-verify` ends with
  `whim-editor-check`, which refuses a tracked file that is not what the
  program writes. `tx/gen.sh` writes only when the content differs and never
  runs make: through `whim-vim.c`'s rule a check could start a build. The binary
  is `bin/whim`.

- **The compile line is the boundary's**, in the work tree's `Makefile`. Up to
  q82 it is `gcc -O0 -static -s` (a static-PIE); phase 83 writes
  `tools/templates/core.mk` over it, `-static -no-pie -s`, and phase 84 adds
  `-fno-stack-protector` (`EXEC`, no `INTERP`, no dynamic section, no relocation
  -- 83 and 84 require all four). `whim.mk` states the product's flags once more
  as `WHIMCFLAGS`/`WHIMLDFLAGS`, for a checkout that only compiles the committed
  `whim-vim.c` and has no work tree to read.
- **No `-g`**, so a formatting change leaves the binary byte-identical -- the
  cheapest verification there is. `SOURCE_DATE_EPOCH=0` pins `__DATE__`/`__TIME__`
  when two builds are compared.
- **`gcc` exits 0 with warnings**; a check on the dead-code sweep is that it
  prints *nothing*, never that it succeeded.

## Temporaries

Every temporary goes in `.tmp/` (gitignored), never the shared `/tmp`: the
`Makefile` exports `TMPDIR` there, so `mktemp`, Go's `os.MkdirTemp`, the
harnesses' scratch homes and a verification's work trees and state directories
all land in it. **Worktrees go in `.tmp/worktrees/`** -- a subagent's too:
`git worktree add .tmp/worktrees/<name>` before launching it, rather than an
isolation flag that chooses its own place, because nothing of ours belongs
inside `.claude/`. Outside `make`, run with
`TMPDIR=$PWD/.tmp`.

## The two paths

A phase is a function of the tree it is handed, so the pipeline is
`p_N = f_N(p_{N-1})` -- 164 of them, in order. **There is no memoize.** Its key
was the input boundary's digest and the implementation's together, so a moved
`slim-vim.c` missed every entry by construction; it paid only while the phases
were being written, and that is over.

- **`make whim-build`** applies the plan (`internal/build`) in one process, in
  memory: no work trees, tars, digests or cache. Measured: 164 phases, **1,044
  s**, 75,225 lines. **`make whim-build-check`** requires the committed
  `whim-vim.c` back, byte for byte, from the committed `slim-vim.c` -- that is
  what holds the plan to the phase programs it was read from, and it is total: a
  step in the wrong order, a dropped argument or a missing sweep moves the bytes.
- **`make whim-verify`** applies the same plan and, at every phase, writes what
  that phase's program wrote for its check -- the source it was handed, that
  source compiled, its enumerator values -- then runs the check
  (`internal/check`) and the stage's declared delta. It costs hours; nothing on
  the build path waits for it. It seeds through `internal/build`'s `Seed`, the
  same function the build path seeds with, so both run the canonical spelling.
  **A check that refuses does not cost the stages after it their input**: an
  edit makes the text and a check only reads it, so the stage hands its tree on
  and every refusal in a run is reported. Only an edit or a sweep failing leaves
  no tree, and then the run stops there.
- **The stage is semantics, not scheduling.** A shared stage runs every edit, ONE
  sweep, then every check on that one swept text, with the symbol snapshot of the
  text its FIRST edit was handed; an `each` stage sweeps after every edit and
  gives every check its own tree. A check in the middle of a shared stage was
  written against what that arrangement hands it, so `internal/verify` keeps it.
  `need P swept` and `apart P K` were the facts that shaped it (`phase/STAGES.md`).
- **`tools/st.sh verify --from N --src BOUNDARY`** verifies from a boundary you
  already have, and `--to N` stops after the stage holding N. `tools/st.sh build
  --to N --work D` leaves that tree, its makefile included, for a phase program
  run by hand.
- **The compile line is the boundary's**, and the work tree's makefile carries
  it: phase 0 starts from `tools/templates/whim.mk`, phase 83 writes
  `tools/templates/core.mk` over it, phase 84 adds `-fno-stack-protector`
  (`internal/build`'s `Makefile` field). A binary built with any other line is
  not the binary a check or a delta means.


## Checks

A check is `phase/NNN/check.go`, one per phase, taking the work tree and a state
directory. `internal/verify` looks it up in `internal/check`'s registry and runs
it.

- A check reads nothing from an edit's shell: the state directory holds the
  input's line count, the stage's symbol snapshot and whatever the phase leaves
  for it (`old.c`, `old`, `enums-before`, `words`, `keep`). What writes those is
  `internal/build`'s plan -- the `OldSource`, `OldBinary`, `EnumVals` and
  `OldDir` fields are the whole of what a phase program built for its check.
- **A recording that fails says why** (`internal/check/recjob.go`): which
  recording, its exit status, its last lines. And **a dead recording is not a
  moved one**: a control whose record set is short refuses as incomplete rather
  than counting the missing records as moved.
- **A check that cannot fail is not evidence.** Every behavioural claim here has
  a control that moves it; a claim the corpus cannot see is stated as such, with
  the reason, and never widened to fit.
- **Assert a partition, not a count**: classify every occurrence into the classes
  a rule serves and refuse on a leftover. A count is a fact about a tree that was
  measured; a partition is a fact about the tree that arrives.

## Baselines

`.reference/` is untracked, produced, and the one thing worth keeping across
passes: every delta is measured against it.

- **`.reference/baselines/`** -- `behaviour/`, `ref-term.txt`, `ref-exsweep.txt`
  -- is recorded by **whim phase 0 from `slim-vim.c`**: three runs that must be
  identical, an existing set compared and never overwritten. Measured when it
  moved here: the Go harnesses on `slim-vim.c`'s binary reproduce arbace/slim-vim's
  own baselines byte for byte (67 behaviour cases, 19 terminals, 600 Ex
  commands). `internal/verify`'s `Delta` checks whim's declared delta against them.
- **`.reference/core-baselines/`** -- `screen/`, `memline/`, `ref-excmds.txt`,
  `ref-argv.txt`, `ref-pty.txt`, `ref-term.txt` (`harness.CoreRecord`) -- is
  recorded by **phase 83 from q82**, the tree it is handed, built with the compile
  line that tree carries; `Delta` hands every phase from 83 on to `CoreDelta`,
  which checks the declarations from 83 on against it.
- **Never regenerate a baseline from a binary a later phase can reach**, which
  would agree by construction. `slim-vim.c` is the pipeline's immutable input, and
  nothing from 83 on can reach q82. The cost of the second: a change to what
  phases 0-82 produce moves it, and phase 83 **refuses** rather than overwrite it
  -- name what moved before removing the set.
- `make whim-verify` asks for both sets before it starts (`whim-baselines-check`)
  and names the fix: **`make whim-baselines`**, which is `whimtools record`
  (`internal/verify`) -- the recording phase 0's program and phase 83's did,
  building q82 to make the second. Each set is recorded three times and required
  identical, and an existing set is COMPARED, never overwritten: a set that
  DIFFERS is not the missing case, and what moved must be named before the
  recording is thrown away.

## Harness rules that were each learned the hard way

- **A vim binary's own name changes what it does** (`parse_command_name()` reads
  `argv[0]`: a leading `r` is restricted mode). Every harness stages the binary
  under test as `vim`, once per binary, under a lock, in a child process -- a
  `fork` in another thread otherwise inherits the copy's write fd and the exec
  dies with `Text file busy`. **Once per binary means once per DIGEST**, not per
  path: a whole-pipeline verification builds every stage's binary at the same
  `work/whim-vim`, so a path key handed every delta after the first the phase 0
  copy -- which is `slim-vim.c`'s own binary -- and each of them then measured
  slim against slim's baselines and reported that nothing had moved.
- **Two pipelines cannot run at once in one checkout.** `.cache/compile` and
  `.cache/symbols` are shared state keyed on the text, so a `build` and a
  `verify` side by side race over them. Run one at a time.
- **A pty harness waits on content, never on a clock**: for the next `\x1b[?25h`,
  where a redraw ends. The window size is set on the slave before the child
  execs.
- **`set -e` changes what `( … ) &` means**, and a bare `wait` with no argument
  returns 0 whatever it waited for.
- Under heavy parallel load a pty scenario can stall; a stage that fails a verify
  and passes alone in less time is the load. Re-run it by itself:
  `tools/st.sh verify --from <first phase of the stage> --to <it> --src <the
  boundary before it>`.
- `-e -s` never initialises the terminal; anything about terminals, mappings or
  `:set` reporting needs a real pty. And silent Ex mode exits 0 on almost
  anything: check what a run *did*, never its status.

## The core and the host

`whim-vim.c`'s twelve `#include`s are not at the top: **the first one is the line
between the editor core and its host**, marked by nothing else. `make editor.c`
cuts there: a complete translation unit with 0 preprocessor lines, 0 errors under
`-fsyntax-only`, and an interface of exactly the names the host defines --
computed, never listed. The core names no libc function at all, holds no file
descriptor of its own, and uses no floating point. `GOALS.md` §II.4 is the
design.

## Adding a phase

`GOALS.md` Part II, *Adding a phase*, has the process; the next phase is 164,
and no more are expected -- the pipeline's goal is met. What a new one takes:
`phase/NNN/` with `GOAL.md`, `delta`, `edit.go` and `check.go` in package
`pNNN`, registering themselves; a line in `phase/registry.go`; and an entry at
the end of `internal/build`'s `Plan` naming its steps, its stage and whether a
sweep follows. Then `make whim-build-check` (the product moves, so the tracked
`whim-vim.c` and `editor/editor.go` are rewritten by `make whim-build`) and
`make whim-verify`.

## Commit style

A `type: summary` subject, then prose explaining *why*, what was measured and how
it was verified. State deliberate omissions.
