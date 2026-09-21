# CLAUDE.md

Guidance for Claude Code working in this repository. **This file describes the
tree as it now is**; when the two disagree, this file is wrong -- fix it, by
editing the sentence that became wrong rather than appending one that disagrees
with it. The numbers here are measurements: re-measure rather than adjust them by
reasoning.

## What this is

Two pipelines, and the Go toolset they run:

```
slim-vim.c  --whim-->  whim-vim.c  --zero-->  zero-vim.c
```

- **The input is one file**, `slim-vim.c`: vim 9.2 as a single C23 translation
  unit, produced by [arbace/slim-vim](https://github.com/arbace/slim-vim), whose
  pipeline removes the preprocessor, the comments and dead code and **changes
  nothing the editor does**. `make` fetches it and vim's `LICENSE` at the commit
  that repository's `main` points to, and records the commit in `upstream.sha`.
  It is not tracked here. **Never edit it**; a change to the input belongs in
  arbace/slim-vim.
- **whim** (`whim.mk`, `WHIM-GOAL.md`) removes capability on purpose -- 83 phases
  in 13 stages, from 180,847 lines to 86,617 -- and every phase **declares its
  delta in advance** in `pipes/whim.delta`; the harness proves it changed that
  and nothing else.
- **zero** (`zero.mk`, `ZERO-GOAL.md`) turns `whim-vim.c` into an embeddable core,
  46 phases to 78,666 lines: no filesystem, the host behind a line in the file,
  no libc the core names, the text a tree. `ZERO-GOAL.md` *The pipeline as it
  stands* is the phase-by-phase account and belongs there, not here.

`whim-vim.c` and `zero-vim.c` are **produced, not edited**. `slim.sha` records the
digest of the `slim-vim.c` the committed `whim-vim.c` came from, `whim.sha` the
`whim-vim.c` the committed `zero-vim.c` came from.

**There is no Python and no agent here.** Every phase is a program; a phase that
refuses stops the pass with its own report, and there is no tier below it to fall
through to. arbace/slim-vim keeps both, for its own pipeline.

## Layout

```
cmd/whimtools/     one binary, every tool a subcommand: whimtools <subcommand>
internal/          the Go: sweep, canon, dead, cut/cutil (the cutters), edit (the
                   whim and zero edit programs), harness (every recorder), check
                   (one check per phase), memo and pipeline (dormant Go ports of
                   the shell driver -- the shell is what runs)
pipes/             the phases, whimN.sh or whimN-edit.sh + whimN-check.sh, the
                   same for zero; *.stages the schedules, *.delta the deltas
tools/             the driver (memo, phaserun, stages, implhash, verifypass,
                   specpass, oracle, snapshot, restore) and the three wrappers
                   that run Go: st.sh, sweep.sh, canon.sh
tools/templates/   the makefile each pipeline's work tree is built with
tools/patches/     cc-v4-c23.patch, two C23 productions modernc.org/cc/v4 lacks
Makefile           fetches the input and includes whim.mk and zero.mk
```

`tools/gobuild.sh` builds `cmd/whimtools` into `.cache/gobin/<key>/`, keyed on
`go.mod`, `go.sum`, the patch and every `.go` under `cmd/` and `internal/`; it
composes the pinned `modernc.org/cc/v4` with the patch under `.cache/gofork/`
rather than vendoring it, because a patched `vendor/` fails `go mod verify`.

## Build

```sh
make                 # all: whim-vim and zero-vim, producing either only when its input moved
```

- whim's compile line is `gcc -O0 -static -s` (a static-PIE), zero's is
  `gcc -O0 -fno-stack-protector -static -no-pie -s` (`EXEC`, no `INTERP`, no
  dynamic section, no relocation -- zero phase 0 and 1 require all four).
  `zero.mk` states zero's flags once more as `ZEROCFLAGS`/`ZEROLDFLAGS` and
  `zero-pass` refuses to copy the product out when they differ from the last
  boundary's makefile.
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
  editing a phase that is not in the last stage, `make whim-repass`. The same for
  zero.
- **Stages** (`pipes/whim.stages`): a run of phases whose edits share one sweep;
  only a stage's end is a boundary. `need P swept` and `apart P K` are declared,
  measured facts, and `tools/stages.sh` refuses a schedule that breaks them.
  Inside a stage each edit is cached by its input (`.cache/edit/`).
- **A tier-3 replay copies the recorded digest rather than recomputing it**, so a
  warm pass agrees with the oracle whatever the oracle says. Only a run that
  recomputes can falsify a boundary: **`make whim-verify` / `make zero-verify`**
  run every stage at once on the recorded boundary before it, in scratch roots
  that snapshot `tools/`, `pipes/`, `cmd/`, `internal/`, `go.mod` and `go.sum`,
  and require each recorded boundary back. Run one before a push and whenever a
  shared tool changes.
- **`make whim-specpass`** speculates every stage at once on the previous pass's
  boundaries, then runs the sequential pass, which hits the cache wherever the
  guess was right.
- **A phase list with a gap refuses** (`memo: zero unit 40 wants r39…`): the
  input half of a key must be a function of the input.
- Zero's phase list is the `phases` line of `pipes/zero.stages` and not
  `tools/pipeline.sh`, because `pipeline.sh` is in every whim key.

## Checks

Every split phase's check part is a **dispatcher**: the shell header -- which is
where a check's argument lives -- then `exec tools/st.sh check <phase> "$work"
"$state"`, whose body is `internal/check/`. The dispatcher names, in a comment,
every `tools/` path its Go runs, for `implhash.sh`. The whole-phase programs
`zero3.sh`, `zero33.sh` and `zero40.sh` dispatch the same way.

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
- **`.reference/zero-baselines/`** is recorded by **zero phase 0 from
  `whim-vim.c`**, built with whim's own compile line; `tools/zerodelta.sh` checks
  zero's delta against it.
- **Never regenerate a baseline from the pipeline's own output**, which would
  agree by construction. Both are recorded from the pipeline's immutable input.
- A tier-3 hit on phase 0 records nothing, so `whim-pass` ends with
  `whim-baselines-check` and zero's targets with `zero-baselines-check`, each
  naming the fix: `rm -rf .reference/baselines .cache/q0 && make whim-phase-0`,
  and `rm -rf .reference/zero-baselines .cache/r0 && make zero-phase-0`.

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
  `sh tools/verifypass.sh --one zero <N> <scratch>` after removing that unit's
  directory and result.
- `-e -s` never initialises the terminal; anything about terminals, mappings or
  `:set` reporting needs a real pty. And silent Ex mode exits 0 on almost
  anything: check what a run *did*, never its status.

## The core and the host

`zero-vim.c`'s eleven `#include`s are not at the top: **the first one is the line
between the editor core and its host**, marked by nothing else. `make editor.c`
cuts there: a complete translation unit with 0 preprocessor lines, 0 errors under
`-fsyntax-only`, and an interface of exactly the names the host defines --
computed, never listed. The core names no libc function at all, holds no file
descriptor of its own, and uses no floating point. `ZERO-PLAN.md` §4 is the
design.

## Adding a phase

`WHIM-GOAL.md` and `ZERO-GOAL.md` have the process. In short: write
`pipes/<p>N-edit.sh` (it calls `tools/st.sh edit <p>N`, whose body is
`internal/edit/`) and `pipes/<p>N-check.sh`, declare its delta in
`pipes/<p>.delta`, add N to the schedule and a package in `pipes/<p>.stages`
(and, for whim, to `tools/pipeline.sh`), and `make <p>-tip`. A new phase starts a
stage of its own if its edit needs swept input.

## Commit style

A `type: summary` subject, then prose explaining *why*, what was measured and how
it was verified. State deliberate omissions.
