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

**whim** (`whim.mk`, 163 phases) removes capability on purpose, and every phase
declares in advance what it changes and a harness proves it changed exactly that
and nothing else. **Phases 0-82** remove the runtime files, the eval layer,
windows beyond one, buffers beyond one, the command-line arguments and 489 Ex
commands. **Phases 83-128** turn what is
left into an embeddable core: the filesystem goes, the signals and the terminal
cross to a host block at the bottom of the same file, the libc that is pure
computation is vendored, the core names no libc function at all, and the memline
stops being pages and becomes a tree, measured with an instrument that reads the
screen. **Phases 129-162** take out of
the core what transpiling it to Go (`editor/`, `tx/FINDINGS.md`) had to work
around; all but one change nothing the editor does, and 142 drops the build
date from the version line.

**A phase is a function of the tree it is handed**, so the pipeline is 163 of
them in order, and it runs as one program: `make whim-build` applies them in
memory and produces `whim-vim.c` in twenty minutes. What answers for it is that
the product is tracked — `make whim-build-check` requires the committed bytes
back from the committed input. `make whim-verify` is the other path: the same
phases with every check and every declared delta, which costs hours and is not
on the way to the editor. Phases are grouped in **stages**, and a stage decides
what text a check sees: phases 1-82 share one sweep per stage, and from 87 each
phase is swept on its own.

## Documents

- **`phase/NNN/`** — one directory per phase, numbered in three digits, and a Go
  package of its own: **`edit.go`** (its cut), **`check.go`** (its evidence),
  **`GOAL.md`** — what it removes, why, and what was measured — and its declared
  **`delta`**.
- **`GOALS.md`** — what holds for every phase. **Part I** is phases 0-82, **Part
  II** phases 83 onwards with the core's own charter and rules; each has an index
  of its phases, Part II's *Adding a phase* is the process for the next one, and
  its appendix is the plan phases 83 onwards were built from (sections II.1-II.6).
- **`tx/FINDINGS.md`** — what transpiling the core to Go found, and which phase
  took each finding out of the C; `tx/CONVENTIONS.md` is how the C is written in
  Go.
- **`CLAUDE.md`** — the working guide: the two paths, how a check is verified,
  and what to know before changing anything shared.

## Use

```sh
make                 # fetch slim-vim.c if upstream moved, then bin/whim, the editor
make whim-vim        # the C product's binary
make whim-build      # the 163 phases in one process: slim-vim.c -> whim-vim.c
make whim-build-check  # the same build, required to give the committed bytes back
make whim-verify     # every phase's check and every declared delta, and refuse
                     # an editor/editor.go that is not what tx/skel writes
make help            # every target, with a line each
make editor.c        # whim-vim.c's core, cut at the line between core and host
make editor/editor.go  # the core in Go, generated from editor.c
make score           # bytes to store and libc symbols to provide, input and product
```

`tools/st.sh verify --from N --src BOUNDARY` verifies one stage from a boundary
you already have, and `tools/st.sh build --to N --work D` leaves that tree for a
phase program run by hand.

## The Go editor

`editor/` is the core, `editor.c`, in Go:
- **`editor.go`**: **generated** by `tx/gen.sh` (`tx/skel` on `modernc.org/cc/v4`):
  every C function a Go function with the same name and control flow, so the
  two read line for line;
- **`crt.go`**: the C runtime it is written against, `Ptr[T]` for a C pointer
  that walks;
- **`host.go`**: the host, from the Go runtime and standard library only.

It follows the current core. `make whim-build` writes `editor.go` again after it
produces `whim-vim.c`, and `make editor/editor.go` does it on its own.
`make whim-verify` refuses a committed file that is not what the program
writes (`make whim-editor-check` asks just that).

```sh
make                                                 # bin/whim
tools/st.sh delta bin/whim whim-vim.c --phase N       # N: the last phase
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
Linux with gcc 15.2 and musl. The C front end is a fork of `modernc.org/cc/v4`
carried as source in `internal/cc`, so nothing is downloaded to build it; `make`
fetches `slim-vim.c`, and after that needs no network.

There is no Python in this repository and no agent: every phase is a program,
and a phase that refuses stops the pass with its own report.

## Layout

```
cmd/whimtools/   the one binary every tool runs as: whimtools <subcommand>
internal/        the cutters, the sweep, the canonicalisers, the harnesses,
                 one check per phase (internal/check/), and ccx: what the
                 core's C leaves a translation to decide -- pointer casts,
                 evaluation order -- partitioned
phase/NNN/       a phase, and a Go package: edit.go, check.go, GOAL.md, delta.md
phase/STAGES.md     the record the stages were read from: need, apart, the packages
tools/           the instruments a check runs, and the wrappers around whimtools
editor/          the core transpiled into Go, with its runtime and host
tx/              tx/skel (the skeleton generator and, with -bodies, the body
                 emitter), tx/splice (measures the emitted bodies in a copy of
                 editor/), tx/pre (ccx's reports on an editor.c), sigs.md,
                 CONVENTIONS.md and FINDINGS.md
whim.mk          the pipeline as make targets
whim-vim.c       the product, tracked; make editor.c cuts the core out of it
upstream.sha     the arbace/slim-vim commit slim-vim.c was fetched from
slim.sha         slim-vim.c's digest, from which whim-vim.c was produced
```

## License

The product is modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
