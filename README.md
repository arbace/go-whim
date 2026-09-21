# go-whim

One pipeline that takes vim apart on purpose, written in Go.

```
slim-vim.c  ──────── 0-82 ────────▶  q82  ──────── 83-128 ────────▶  whim-vim.c
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

**whim** (`whim.mk`, 129 phases) removes capability on purpose, and every phase
declares in advance what it changes and a harness proves it changed exactly that
and nothing else. **Phases 0-82** (`WHIM-GOAL.md` and `WHIM-PLAN.md`, Part I of each) remove the
runtime files, the eval layer, windows beyond one, buffers beyond one, the
command-line arguments and 489 Ex commands; their deltas are `pipes/whim.delta`.
**Phases 83-128** (Part II of each) turn what is left into an
embeddable core: the filesystem goes, the signals and the terminal cross to a
host block at the bottom of the same file, the libc that is pure computation is
vendored, the core names no libc function at all, and the memline stops being
pages and becomes a tree; their deltas are `pipes/zero.delta`, against an
instrument that reads the screen. Those were a second pipeline, zero, numbered
from 0 — zero phase N is phase N+83.

**A phase is a function of the tree it is handed**, memoized by content — the
input boundary's digest and the implementation's — so re-running a pass is free,
and editing one phase re-runs that phase and the ones after it.

## Use

```sh
make                 # fetch slim-vim.c if upstream moved, then whim-vim
make whim-verify     # reproduce every recorded boundary, all stages at once
make editor.c        # whim-vim.c's core, cut at the line between core and host
make score           # bytes to store and libc symbols to provide, input and product
```

`make whim-repass` recomputes the pipeline from nothing; `make whim-specpass` does
it with every stage speculated at once on the previous pass's boundaries first.

## Requirements

Linux, **Go 1.27**, **gcc** that links a static binary against **musl** (the
product is built `-static -no-pie`, and phases before 83 `-static`), `git`, `curl`, and
binutils (`readelf`, `nm`, `objcopy`, `strings`). Measured on Alpine Linux with
gcc 15.2 and musl. The first build downloads `modernc.org/cc/v4`, pinned in
`go.mod` and patched by `tools/patches/cc-v4-c23.patch` for two C23 productions;
after that `make clean-cache` needs no network.

There is no Python in this repository and no agent: every phase is a program,
and a phase that refuses stops the pass with its own report.

## Layout

```
cmd/whimtools/   the one binary every tool runs as: whimtools <subcommand>
internal/        the cutters, the sweep, the canonicalisers, the harnesses,
                 and one check per phase (internal/check/)
pipes/           the phases: whimN.sh or whimN-edit.sh + whimN-check.sh;
                 whim.stages is the schedule and the packages, whim.delta
                 (phases 0-82) and zero.delta (83 on) the declared deltas
tools/           the memoize driver and the shell wrappers around whimtools
whim.mk          the pipeline as make targets
whim-vim.c       the product, tracked
upstream.sha     the arbace/slim-vim commit slim-vim.c was fetched from
slim.sha         slim-vim.c's digest, from which whim-vim.c was produced
```

`CLAUDE.md` is the working guide: how the memoize keys a phase, how a check is
verified, and what to know before changing anything shared.

## License

The products are modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
