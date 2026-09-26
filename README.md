# go-whim

A pipeline, written in Go, that takes vim apart on purpose -- and the editor it
leaves, four times over: in C, in Go, in Java and in Clojure, each required
to answer every test case as the others do.

```
slim-vim.c ──── whim: phases, in order ────▶ whim-vim.c   an embeddable editor core
                                                  │
                  crefactor/togo ─────────────────┼──▶ editor/    the core in Go
                                                  ├──▶ jeditor/   the core in Java
                                                  └──▶ cljeditor/ the core in Clojure
```

**The input** is `slim-vim.c`: vim 9.2 as one C translation unit, without
preprocessor or comments, from [arbace/slim-vim](https://github.com/arbace/slim-vim).
`make` fetches it, and vim's `LICENSE`, at that repository's head.

**The pipeline** removes capability on purpose, one phase at a time: first
whatever needs a runtime installed; then the filesystem, the libc the core
names and the host, which crosses to a block at the bottom of the file; then
what translating the core to Go and Java had to work around. Each phase is its
steps, then a reachability sweep, then one canonical print, and
`make whim-build` runs them all in one process in about fifteen minutes. The
product, `src/whim-vim.c`, is tracked, and `make whim-build-check` requires it
back byte for byte.

**The translations** are written by `crefactor/togo` from the core's C, not
from each other: `editor/editor.go`, `jeditor/Editor.java` and
`cljeditor/src/whim/editor.clj` are generated, tracked, and refused by
`make whim-editor-check` when stale.

**The tests**: `make whim-test` runs 45 key sessions on the C and the Go editor
and requires the same screens, with a control that must move them;
`make whim-test-wide` does the same with 240 cases; `make whim-test-java` and
`make whim-test-clj` hold the Java and the Clojure editors to them too. `make go-test` runs the Go packages' tests.

## Use

```sh
make                   # fetch the input if it moved, build the Go editor, bin/whim
make whim-build        # the pipeline: slim-vim.c -> whim-vim.c and the three translations
make whim-build-check  # the same, required to give the committed bytes back
make whim-editor-check # refuse a stale editor.go, Editor.java or editor.clj
make whim-test         # the quick suite; whim-test-wide, whim-test-java
make go-test           # the Go packages' tests
make whim-vim          # the C editor's binary
make bin/whim-java     # the Java editor, and a launcher: bin/whim-java [args]
make jeditor.jar       # the same as one jar: java -jar jeditor.jar [args]
make bin/whim-clj      # the Clojure editor (doc/CLOJURE.md), and a launcher: bin/whim-clj [args]
make cljeditor.jar     # the same as one jar: java -jar cljeditor.jar [args]
make whim-test-clj     # the quick suite with the Clojure editor too
make editor.lgo        # the Go editor as one go-lisp file (doc/GO-LISP.md)
make help              # every target
```

## The editors

- **C**, `src/whim-vim.c`: the core, then the host from its first `#include` on.
- **Go**, `editor/`: `package editor`, a library. An `Editor` is one editor on a
  `Host` (the terminal, the clock, input, signals, output); a process holds as
  many as it makes. `editor/term` is the terminal host, `editor/cmd/whim` the
  launcher behind `bin/whim`.
- **Java**, `jeditor/`: the same shape -- `Editor.java` on a `Host`, a runtime
  (`rt/`, a C pointer as an array and an offset), and a terminal host through
  the Foreign Function & Memory API.
- **Clojure**, `cljeditor/`: the namespace `whim.editor`, generated from basic
  blocks (structured `let`/`loop` where the C's flow nests, a `loop`/`case`
  state machine where it does not), on the Java editor's runtime and host
  through interop; `whim.cljhost` the glue, `whim.cljmain` the launcher.

## Requirements

Linux, **Go 1.27**, **gcc** linking statically against **musl**, `git`, `curl`,
and binutils; measured on Alpine Linux. The Java editor also needs a **JDK 22
or later**; the Clojure editor the **`clojure`** command too (Clojure 1.12, whose jars
it copies), and starts fastest on a JDK 25 or later (an AOT cache). The C front end is a fork of `modernc.org/cc/v4` carried as source,
so after fetching the input nothing needs the network.

## Layout

```
cmd/whim/        the toolset: go tool whim <subcommand>
internal/        the plan, the phases (internal/phase/NNN/: GOAL.md, edit.go)
                 and what the generic library is told about vim (internal/whim)
crefactor/       the generic C refactoring library, a Go module of its own:
                 the C front end, the canonical printer, the sweep, the driver,
                 the transforms, the analyses, and togo, the C-to-Go-and-Java
                 translator
editor/          the editor in Go          jeditor/   the editor in Java
cljeditor/       the editor in Clojure: the glue, the launcher, the build
src/             the input (fetched) and the product (tracked)
doc/             GOALS.md (what holds for every phase), AGENDA.md (what is
                 not done), JAVA.md, GO-IDIOMS.md, PIPELINE-COMPACTION.md,
                 GO-LISP.md, CLOJURE.md, HASKELL.md, RUST.md (preliminary plans, not scheduled)
CLAUDE.md        the working guide: the build, the pipeline, what to know
                 before changing anything shared
```

## License

The product is modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
