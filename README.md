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
commit in `upstream.sha`.

**whim** (`whim.mk`, 155 phases) removes capability on purpose, and every phase
declares in advance what it changes and a harness proves it changed exactly that
and nothing else. **Phases 0-82** remove the runtime files, the eval layer,
windows beyond one, buffers beyond one, the command-line arguments and 489 Ex
commands; their deltas are `pipes/whim.delta`. **Phases 83-128** turn what is
left into an embeddable core: the filesystem goes, the signals and the terminal
cross to a host block at the bottom of the same file, the libc that is pure
computation is vendored, the core names no libc function at all, and the memline
stops being pages and becomes a tree; their deltas are `pipes/zero.delta`,
against an instrument that reads the screen. Those were a second pipeline, zero,
numbered from 0 — zero phase N is phase N+83. **Phases 129-162** take out of
the core what transpiling it to Go (`editor/`, `tx/FINDINGS.md`) had to work
around; all but one change nothing the editor does, and 142 drops the build
date from the version line.

**A phase is a function of the tree it is handed**, memoized by content — the
input boundary's digest and the implementation's — so re-running a pass is free,
and editing one phase re-runs that phase and the ones after it. Phases run in
**stages**, and only a stage's end is a boundary: phases 1-82 share one sweep per
stage, because there the sweep is most of a phase; from 87 each phase is swept on
its own and every check in a stage runs at once, because there the checks are.

## Documents

- **`WHIM-GOAL.md`** — what the pipeline removes and why. **Part I** is phases
  0-82, **Part II** phases 83 onwards with the core's own charter and rules; each
  has a section per phase, and Part II's *Adding a phase* is the process for the
  next one.
- **`tx/FINDINGS.md`** — what transpiling the core to Go found, and which phase
  took each finding out of the C; `tx/CONVENTIONS.md` is how the C is written in
  Go.
- **`WHIM-PLAN.md`** — the plans the phases were built from: **Part I** grouped
  phases 0-82 into stages and packages, **Part II** (sections II.1-II.6) planned
  phases 83 onwards.
- **`CLAUDE.md`** — the working guide: how the memoize keys a phase, how a check is
  verified, and what to know before changing anything shared.

## Use

```sh
make                 # fetch slim-vim.c if upstream moved, then whim-vim
make whim-verify     # reproduce every recorded boundary, all stages at once
make editor.c        # whim-vim.c's core, cut at the line between core and host
make score           # bytes to store and libc symbols to provide, input and product
```

`make whim-repass` recomputes the pipeline from nothing; `make whim-specpass` does
it with every stage speculated at once on the previous pass's boundaries first.

## The Go editor

`editor/` is the core, `editor.c`, transpiled by hand into Go:
- **`editor.go`**: every C function a Go function with the same name and
  control flow, so the two read line for line;
- **`crt.go`**: the C runtime it is written against, `Ptr[T]` for a C pointer
  that walks;
- **`host.go`**: the host, from the Go runtime and standard library only.

It follows the current core. When a phase changes the C, the functions it
changed are re-transpiled from their new C, and the rest carried over.

```sh
go build -o editor.bin ./editor
tools/zerodelta.sh editor.bin whim-vim.c --phase N   # N: the last phase
```

It is measured the way the C is: the Go build must record exactly the declared
delta of the last phase, byte for byte across the screen cases, the Ex rows, the
command lines, the pty scenarios, the terminals and the memline corpus.

The Go is faithful, not yet idiomatic. The phases from 129 on removed from the C
what it had to work around; making the Go idiomatic comes next, measured by the
same recording.

## Requirements

Linux, **Go 1.27**, **gcc** that links a static binary against **musl** (the
product is built `-static -no-pie`, and phases before 83 `-static`), `git`,
`curl`, and binutils (`readelf`, `nm`, `objcopy`, `strings`). Measured on Alpine
Linux with gcc 15.2 and musl. The first build downloads `modernc.org/cc/v4`,
pinned in `go.mod` and patched by `tools/patches/cc-v4-c23.patch` for two C23
productions; after that `make clean-cache` needs no network.

There is no Python in this repository and no agent: every phase is a program,
and a phase that refuses stops the pass with its own report.

## Layout

```
cmd/whimtools/   the one binary every tool runs as: whimtools <subcommand>
internal/        the cutters, the sweep, the canonicalisers, the harnesses,
                 one check per phase (internal/check/), and ccx: what the
                 core's C leaves a translation to decide -- pointer casts,
                 evaluation order -- partitioned
pipes/           the phases: whimN.sh or whimN-edit.sh + whimN-check.sh;
                 whim.stages is the schedule and the packages, whim.delta
                 (phases 0-82) and zero.delta (83 on) the declared deltas
tools/           the memoize driver and the shell wrappers around whimtools
editor/          the core transpiled into Go, with its runtime and host
tx/              tx/skel (the skeleton generator), tx/pre (ccx's reports on an
                 editor.c), sigs.txt, CONVENTIONS.md and FINDINGS.md
whim.mk          the pipeline as make targets
whim-vim.c       the product, tracked; make editor.c cuts the core out of it
upstream.sha     the arbace/slim-vim commit slim-vim.c was fetched from
slim.sha         slim-vim.c's digest, from which whim-vim.c was produced
```

## License

The product is modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
