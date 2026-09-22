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
- **whim** (`whim.mk`) removes capability on purpose, 140 phases from 180,870
  lines to 77,907, and every phase **declares its delta in advance**; the harness
  proves it changed that and nothing else. It is two arcs and a coda:
  - **phases 0-82** (`WHIM-GOAL.md` Part I) leave an editor with no runtime to
    install, 86,583 lines at q82; their delta is `pipes/whim.delta`, against
    `.reference/baselines`;
  - **phases 83-128** (`WHIM-GOAL.md` Part II) turn it into an embeddable core:
    no filesystem, the host behind a line in the file, no libc the core names, the
    text a tree. Their delta is `pipes/zero.delta`, against
    `.reference/zero-baselines`, with another instrument. They were a pipeline of
    their own, **zero**, numbered from 0, and every document and program here now
    numbers them as this one does: zero phase N is phase N+83, and a Part II
    heading gives both. `WHIM-GOAL.md` Part II, *Phases 83 to 128 as they
    stand*, is the phase-by-phase account and belongs there, not here;
  - **phases 129-139** (the last sections of Part II) remove from the core what
    transpiling it to Go (`editor/editor.go`, `tx/FINDINGS.md`) had to work
    around, and change nothing the editor does: each declares nothing in
    `pipes/zero.delta`, and its check carries a probe or a byte-identical
    binary for what the recording cannot see. `tx/FINDINGS.md` maps each
    finding to its phase.

  `ZERO_FROM=83` in `tools/pipeline.sh` is the line between the two arcs, stated
  once.

The two documents follow the two arcs. **`WHIM-GOAL.md`** is Part I (phases 0-82:
charter, rules, the sweep, the concept index, one section per phase), **Part II**
(phases 83-128: the core's charter, what is measured from 83 on, the core's rules
-- cited as *core rule N*, and holding beside Part I's -- *Phases 83 to 128 as they
stand*, *Adding a phase*, one section per phase) and *What comes next*. Every
phase's section is `## Phase N — …`, which is where `tools/phasename.sh` reads its
name. **`WHIM-PLAN.md`** is Part I, the plan that grouped phases 0-82 into stages
and packages, and Part II, the plan phases 83 onwards were built from, its
sections numbered II.1-II.6 (cited `WHIM-PLAN.md II.4c`).

`whim-vim.c` is **produced, not edited**. `slim.sha` records the digest of the
`slim-vim.c` the committed `whim-vim.c` came from.

**There is no Python and no agent here.** Every phase is a program; a phase that
refuses stops the pass with its own report, and there is no tier below it to fall
through to. arbace/slim-vim keeps both, for its own pipeline.

## Layout

```
cmd/whimtools/     one binary, every tool a subcommand: whimtools <subcommand>
internal/          the Go: sweep, canon, dead, cut/cutil (the cutters), edit (the
                   phases' edit programs), harness (every recorder), check
                   (one check per phase), memo and pipeline (dormant Go ports of
                   the shell driver -- the shell is what runs)
pipes/             the phases, whimN.sh or whimN-edit.sh + whimN-check.sh;
                   whim.stages the schedule, whim.delta and zero.delta the deltas
tools/             the driver (memo, phaserun, stages, implhash, verifypass,
                   specpass, oracle, snapshot, restore) and the three wrappers
                   that run Go: st.sh, sweep.sh, canon.sh
tools/templates/   whim.mk, the makefile phase 0 starts from, and zero.mk, the one
                   phase 83 writes over it
tools/patches/     cc-v4-c23.patch, two C23 productions modernc.org/cc/v4 lacks
Makefile           fetches the input and includes whim.mk
```

`tools/gobuild.sh` builds `cmd/whimtools` into `.cache/gobin/<key>/`, keyed on
`go.mod`, `go.sum`, the patch and every `.go` under `cmd/` and `internal/`; it
composes the pinned `modernc.org/cc/v4` with the patch under `.cache/gofork/`
rather than vendoring it, because a patched `vendor/` fails `go mod verify`.

## Build

```sh
make                 # all: whim-vim, producing whim-vim.c only when slim-vim.c moved
```

- **The compile line is the boundary's**, in the work tree's `Makefile`. Up to
  q82 it is `gcc -O0 -static -s` (a static-PIE); phase 83 writes
  `tools/templates/zero.mk` over it, `-static -no-pie -s`, and phase 84 adds
  `-fno-stack-protector` (`EXEC`, no `INTERP`, no dynamic section, no relocation
  -- 83 and 84 require all four). `whim.mk` states the product's flags once more
  as `WHIMCFLAGS`/`WHIMLDFLAGS` and `whim-pass` refuses to copy the product out
  when they differ from the last boundary's makefile.
- **No `-g`**, so a formatting change leaves the binary byte-identical -- the
  cheapest verification there is. `SOURCE_DATE_EPOCH=0` pins `__DATE__`/`__TIME__`
  when two builds are compared.
- **`gcc` exits 0 with warnings**; a check on the dead-code sweep is that it
  prints *nothing*, never that it succeeded.

## Temporaries

Every temporary goes in `.tmp/` (gitignored), never the shared `/tmp`: the
`Makefile` exports `TMPDIR` there, so `mktemp`, Go's `os.MkdirTemp`, the
harnesses' scratch homes and the verify and specpass scratch roots all land in
it. Worktrees go in `.tmp/worktrees/`. Outside `make`, run with
`TMPDIR=$PWD/.tmp`.

## The memoize

A phase is a function of the tree it is handed, so a pipeline is `p_N = f_N(p_{N-1})`
memoized by content. A **boundary** is a tar and a content digest of a stage's
end; the tier-3 cache key is the input boundary's digest **and**
`tools/implhash.sh` of the implementation together.

- **`implhash.sh` finds a phase's dependencies by grepping its program for
  paths** -- `tools/…`, `pipes/…`, `cmd/…`, `internal/…`, and the root `go.mod`
  and `go.sum` -- one level through what those name, and a bare directory
  mention (`internal/`) expands to every file under it. So **a phase names a
  tool as a PATH**, in a comment if it must, or an edit to that tool moves no key
  and a warm pass replays a stale boundary. The three wrappers name `cmd/`,
  `internal/`, `go.mod` and `go.sum`, which is how every Go file is in every key
  that runs Go. The rule cannot tell *I depend on X* from *I mention X*; it
  over-includes, which costs CPU and never a wrong boundary.
- **Editing an existing phase's program does not re-run it under `make
  whim-pass`**: the prerequisite is the previous boundary *file*. `make
  whim-phase-N` (runs the stage holding N) and `make whim-tip` force it; after
  editing a phase that is not in the last stage, `make whim-repass`.
- **Stages** (`pipes/whim.stages`): a run of phases whose end is the only
  boundary, in one of two modes. A **shared** stage (`stage A-B`, phases 1-82)
  runs every edit, ONE sweep, every check on the one swept text -- a sweep was
  most of a whim phase. `need P swept` and `apart P K` are declared, measured
  facts, and `tools/stages.sh` refuses a schedule that breaks them. An **each**
  stage (`stage A-B each`, phases 87-139) sweeps after every edit, then runs every
  check and delta at once (`CHECK_JOBS`, default 8), each in a root of its own
  under `.cache/state/` on exactly the tree, state and symbol snapshot it had as a
  stage of one -- there the checks are most of a phase, and each was written
  against its own boundary, so no `need` or `apart` can break inside one. A
  single-file program (`whimN.sh`) is always a stage of its own. Edits are cached
  by their input (`.cache/edit/`); in an each stage, with their sweep. The mode is
  in `tools/implhash.sh`'s key.
- **A tier-3 replay copies the recorded digest rather than recomputing it**, so a
  warm pass agrees with the oracle whatever the oracle says. Only a run that
  recomputes can falsify a boundary: **`make whim-verify`** runs every stage at once on the recorded boundary before it, in scratch roots
  that snapshot `tools/`, `pipes/`, `cmd/`, `internal/`, `go.mod` and `go.sum`,
  and requires each recorded boundary back. Run it before a push and whenever a
  shared tool changes.
- **`make whim-specpass`** speculates every stage at once on the previous pass's
  boundaries, then runs the sequential pass, which hits the cache wherever the
  guess was right.
- **A phase list with a gap refuses** (`memo: whim unit 123 wants q122…`): the
  input half of a key must be a function of the input.
- The phase list is the `phases` line of `pipes/whim.stages` and not
  `tools/pipeline.sh`, because `pipeline.sh` is in every key.

## Checks

Every split phase's check part is a **dispatcher**: the shell header -- which is
where a check's argument lives -- then `exec tools/st.sh check <phase> "$work"
"$state"`, whose body is `internal/check/`. The dispatcher names, in a comment,
every `tools/` path its Go runs, for `implhash.sh`. The whole-phase programs
`whim86.sh`, `whim116.sh` and `whim123.sh` dispatch the same way.

- A check reads nothing from its edit's shell: the state directory
  (`.cache/state/q<N>`) holds the input's line count, the stage's symbol snapshot
  and whatever the edit names for it (`old.c`, `old`, `words`, `keep`).
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
  commands). `tools/whimdelta.sh` checks whim's declared delta against them.
- **`.reference/zero-baselines/`** -- `screen/`, `memline/`, `ref-excmds.txt`,
  `ref-argv.txt`, `ref-pty.txt`, `ref-term.txt` (`tools/zrecord.sh`) -- is
  recorded by **phase 83 from q82**, the tree it is handed, built with the compile
  line that tree carries; `tools/whimdelta.sh` hands every phase from 83 on to
  `tools/zerodelta.sh`, which checks `pipes/zero.delta` against it.
- **Never regenerate a baseline from a binary a later phase can reach**, which
  would agree by construction. `slim-vim.c` is the pipeline's immutable input, and
  nothing from 83 on can reach q82. The cost of the second: a change to what
  phases 0-82 produce moves it, and phase 83 **refuses** rather than overwrite it
  -- name what moved before removing the set.
- A tier-3 hit on phase 0 or 83 records nothing, so every target that can end a
  pass ends with `whim-baselines-check`, which names the fix:
  `rm -rf .reference/baselines .cache/q0 && make whim-phase-0`, and
  `rm -rf .reference/zero-baselines .cache/q83 && make whim-phase-83`.

## Harness rules that were each learned the hard way

- **A vim binary's own name changes what it does** (`parse_command_name()` reads
  `argv[0]`: a leading `r` is restricted mode). Every harness stages the binary
  under test as `vim`, once per binary, under a lock, in a child process -- a
  `fork` in another thread otherwise inherits the copy's write fd and the exec
  dies with `Text file busy`.
- **A pty harness waits on content, never on a clock**: for the next `\x1b[?25h`,
  where a redraw ends. The window size is set on the slave before the child
  execs.
- **`set -e` changes what `( … ) &` means**, and a bare `wait` with no argument
  returns 0 whatever it waited for.
- Under heavy parallel load a pty scenario can stall; a unit that fails a verify
  and passes alone in less time is the load. Re-run it by itself:
  `sh tools/verifypass.sh --one whim <unit> <scratch>` after removing that unit's
  directory and result.
- `-e -s` never initialises the terminal; anything about terminals, mappings or
  `:set` reporting needs a real pty. And silent Ex mode exits 0 on almost
  anything: check what a run *did*, never its status.

## The core and the host

`whim-vim.c`'s eleven `#include`s are not at the top: **the first one is the line
between the editor core and its host**, marked by nothing else. `make editor.c`
cuts there: a complete translation unit with 0 preprocessor lines, 0 errors under
`-fsyntax-only`, and an interface of exactly the names the host defines --
computed, never listed. The core names no libc function at all, holds no file
descriptor of its own, and uses no floating point. `WHIM-PLAN.md` §II.4 is the
design.

## Adding a phase

`WHIM-GOAL.md` Part II, *Adding a phase*, has the process; the next phase is 140. In
short: write `pipes/whimN-edit.sh` (it calls `tools/st.sh edit whimN`, whose body
is `internal/edit/`) and `pipes/whimN-check.sh`, declare its delta in
`pipes/zero.delta`, add N to the `phases` line, a stage and a package in
`pipes/whim.stages`, and `make whim-tip`, then `make whim-pass` to copy the
product out. A split phase joins the last `each` stage (its edit is swept on its
own and its check sees its own tree, so no `need` or `apart` can bind there); a
single-file program is a stage of its own.

## Commit style

A `type: summary` subject, then prose explaining *why*, what was measured and how
it was verified. State deliberate omissions.
