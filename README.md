# go-whim

Two pipelines that take vim apart on purpose, written in Go.

```
slim-vim.c  ──G──▶  whim-vim.c  ──H──▶  zero-vim.c
(input)             an editor with       an embeddable editor core:
                    no runtime to        no filesystem, no libc it did
                    install              not vendor, the text a tree
```

**The input is one file.** `slim-vim.c` is vim 9.2 as a single translation unit —
no preprocessor, no comments, every body braced — produced by
[arbace/slim-vim](https://github.com/arbace/slim-vim), whose own pipeline changes
nothing about what the editor does. `make` asks that repository for its head,
fetches `slim-vim.c` and vim's `LICENSE` at exactly that commit, and records the
commit in `upstream.sha`.

**whim** (`whim.mk`, 83 phases in 13 stages) removes capability on purpose: the
runtime files, the eval layer, windows beyond one, buffers beyond one, the
command-line arguments, 489 Ex commands. Every phase declares in advance what it
changes (`pipes/whim.delta`) and a harness proves it changed exactly that and
nothing else. `WHIM-GOAL.md` is the process; `WHIM-PLAN.md` the plan it followed.

**zero** (`zero.mk`, 46 phases) turns `whim-vim.c` into an embeddable core: the
filesystem goes, the signals and the terminal cross to a host block at the
bottom of the same file, the libc that is pure computation is vendored, the core
names no libc function at all, and the memline stops being pages and becomes a
tree. `ZERO-GOAL.md` is the process and the phase-by-phase account.

Both are the same construct: **a phase is a function of the tree it is handed**,
memoized by content — the input boundary's digest and the implementation's —
so re-running a pass is free, and editing one phase re-runs that phase and the
ones after it.

## Use

```sh
make                 # fetch slim-vim.c if upstream moved, then whim-vim and zero-vim
make whim-verify     # reproduce every recorded whim boundary, all stages at once
make zero-verify     # the same for zero
make editor.c        # zero-vim.c's core, cut at the line between core and host
make score           # bytes to store and libc symbols to provide, all three
```

`make whim-repass` and `make zero-repass` recompute a pipeline from nothing;
`make whim-specpass` and `make zero-specpass` do it with every stage speculated at
once on the previous pass's boundaries first.

## Requirements

Linux, **Go 1.27**, **gcc** that links a static binary against **musl** (the
products are built `-static`, and zero's `-static -no-pie`), `git`, `curl`, and
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
pipes/           the phases: whimN.sh or whimN-edit.sh + whimN-check.sh, the same
                 for zero; whim.stages and zero.stages are the schedules,
                 whim.delta and zero.delta the declared deltas
tools/           the memoize driver and the shell wrappers around whimtools
whim.mk zero.mk  the pipelines as make targets
whim-vim.c       whim's product, tracked
zero-vim.c       zero's product, tracked
upstream.sha     the arbace/slim-vim commit slim-vim.c was fetched from
slim.sha         slim-vim.c's digest, from which whim-vim.c was produced
whim.sha         whim-vim.c's digest, from which zero-vim.c was produced
```

`CLAUDE.md` is the working guide: how the memoize keys a phase, how a check is
verified, and what to know before changing anything shared.

## License

The products are modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
