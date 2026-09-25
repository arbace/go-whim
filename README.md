# go-whim

One pipeline that takes vim apart on purpose, written in Go.

```
slim-vim.c  ──────── 0-82 ────────▶  q82  ──────── 83-162 ────────▶  whim-vim.c
(input)              an editor with no           an embeddable editor core:
                     runtime to install          no filesystem, no libc it did
                                                 not vendor, the text a tree
```

**The input is one file.** `slim-vim.c` is vim 9.2 as a single translation unit —
no preprocessor, no comments, every body braced — produced by
[arbace/slim-vim](https://github.com/arbace/slim-vim), whose own pipeline changes
nothing about what the editor does. `make` asks that repository for its head,
fetches `slim-vim.c` and vim's `LICENSE` at exactly that commit, and records the
commit in `src/upstream.sha`.

**whim** (the `Makefile`, 169 phases) removes capability on purpose. **Phases 0-82** remove the runtime files, the eval layer,
windows beyond one, buffers beyond one, the command-line arguments and 489 Ex
commands. **Phases 83-128** turn what is
left into an embeddable core: the filesystem goes, the signals and the terminal
cross to a host block at the bottom of the same file, the libc that is pure
computation is vendored, the core names no libc function at all, and the memline
stops being pages and becomes a tree, measured with an instrument that reads the
screen. **Phases 129-162** take out of
the core what transpiling it to Go (`editor/`, `internal/gen/FINDINGS.md`) had to work
around; all but one change nothing the editor does, and 142 drops the build
date from the version line.

**A phase is a function of the tree it is handed**, so the pipeline is 169 of
them in order, and it runs as one program: `make whim-build` applies them in
memory and produces `whim-vim.c` in about seventeen minutes. Every phase is its steps, then
the sweep -- one reachability closure over the parsed text, cutting what `main`
cannot reach -- then the one canonical print. What answers for it
is that the product is tracked — `make whim-build-check` requires the committed
bytes back from the committed input.

**The test suite is minimal**, `make whim-test`: 44 key sessions, each required
to print the same screens from the working tree's build as from HEAD's, and from
the Go editor as from the C, with a control that must move them; about five
seconds. The suite each phase was verified with while it was written -- check
programs, and deltas declared in advance against recorded baselines -- was
removed after `448e9a8`, the last commit that has it.

## Documents

- **`internal/phase/NNN/`** — one directory per phase, numbered in three digits:
  **`GOAL.md`** — what it removes, why, and what was measured — and, where the
  cut is a program, **`edit.go`**, which makes it a Go package of its own.
- **`doc/GOALS.md`** — what holds for every phase. **Part I** is phases 0-82, **Part
  II** phases 83 onwards with the core's own charter and rules; each has an index
  of its phases, Part II's *Adding a phase* is the process for the next one, and
  its appendix is the plan phases 83 onwards were built from (sections II.1-II.6).
- **`internal/gen/FINDINGS.md`** — what transpiling the core to Go found, and which phase
  took each finding out of the C; `internal/gen/CONVENTIONS.md` is how the C is written in
  Go.
- **`CLAUDE.md`** — the working guide: the build, the pipeline, and what to know
  before changing anything shared.

## Use

```sh
make                 # fetch slim-vim.c if upstream moved, then bin/whim, the editor
make whim-vim        # the C product's binary
make slim-vim        # the input's binary, with the same one line
make whim-build      # the 169 phases in one process: slim-vim.c -> whim-vim.c
make whim-build-check  # the same build, required to give the committed bytes back
make whim-editor-check # refuse an editor/editor.go that is not what internal/gen writes
make help            # every target, with a line each
make editor.c        # whim-vim.c's core, cut at the line between core and host
make editor/editor.go  # the core in Go, generated from editor.c
make score           # bytes to store and libc symbols to provide, input and product
```

`go tool whim build --to N --work D` leaves the tree after phase N for a phase
program run by hand, and `--keep D` every boundary.

## The Go editor

`editor/` is the core, `editor.c`, in Go:
- **`editor.go`**: **generated** by `go tool whim gen` (`internal/gen` on `modernc.org/cc/v4`):
  every C function a Go function with the same name and control flow, so the
  two read line for line;
- **`crt.go`**: the C runtime it is written against, `Ptr[T]` for a C pointer
  that walks;
- **`host.go`**: the host, from the Go runtime and standard library only.

It follows the current core. `make whim-build` writes `editor.go` again after it
produces `whim-vim.c`, and `make editor/editor.go` does it on its own.
`make whim-editor-check` refuses a committed file that is not what the program
writes, and `make` builds it into `bin/whim`.

The Go is faithful, not yet idiomatic. The phases from 129 on removed from the C
what it had to work around; making the Go idiomatic comes next -- `doc/GO-IDIOMS.md`
is the measured survey of how, and `make whim-test` checks the Go editor
against the C on every run.

## Requirements

Linux, **Go 1.27**, **gcc** that links a static binary against **musl** (every
binary is built `-O0 -fno-stack-protector -static -no-pie -s`), `git`,
`curl`, and binutils (`readelf`, `nm`, `objcopy`, `strings`). Measured on Alpine
Linux with gcc 15.2 and musl. The C front end is a fork of `modernc.org/cc/v4`
carried as source in `internal/cc`, so nothing is downloaded to build it; `make`
fetches `slim-vim.c`, and after that needs no network.

There is no Python in this repository and no agent: every phase is a program,
and a phase that refuses stops the pass with its own report.

## Layout

```
cmd/whim/        the toolset, every tool a subcommand: go tool whim <subcommand>
internal/        the cutters, the sweep, the canonical printer (cemit), the plan and its
                 driver (internal/build), the dead-code reporter (reach), ccx:
                 what the core's C leaves a translation to decide -- pointer
                 casts, evaluation order -- partitioned, and the phases:
                 internal/phase/NNN/ (GOAL.md, and edit.go where its cut is a
                 program) and internal/phase/STAGES.md, the record of the
                 stages there were
editor/          the core transpiled into Go, with its runtime and host
internal/gen/    the generator of editor/editor.go (go tool whim gen), splice/
                 (whim splice: the emitted bodies measured in a copy of
                 editor/), pre/ (whim pre: ccx's reports on an editor.c),
                 sigs.md, CONVENTIONS.md and FINDINGS.md
Makefile         the whole build: the input, the pipeline, the binaries, the editor
doc/             GOALS.md (what holds for every phase), AGENDA.md (what is not
                 done, in order), and two surveys: AST-EDITING.md (the phases
                 editing the AST: not taken, to revisit) and GO-IDIOMS.md (how the Go
                 editor could be idiomatic, measured and ranked)
src/             the input and the product, their binaries and digests:
                 whim-vim.c (the product, tracked; make editor.c cuts the core
                 out of it), slim-vim.c (the input, fetched, not tracked),
                 upstream.sha (the arbace/slim-vim commit slim-vim.c was
                 fetched from), slim.sha (slim-vim.c's digest, from which
                 whim-vim.c was produced), and the binaries slim-vim, whim-vim
```

## License

The product is modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
