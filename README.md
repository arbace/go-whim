# go-whim

A pipeline, written in Go, that takes vim apart on purpose -- and the editor it
leaves, eight times over: in C, in Go, in Java, in Clojure, in Haskell, in
Rust, in Scheme and in OCaml, each required to answer every test case as the
C does.

```
slim-vim.c ──── whim: 104 phases, in order ────▶ whim-vim.c   an embeddable editor core
                                                      │
                  crefactor/togo ─────────────────────┼──▶ editor/   the core in Go
                                                      ├──▶ braaam/   the core in Java
                                                      ├──▶ vijure/   the core in Clojure
                                                      ├──▶ caprice/  the core in Haskell
                                                      ├──▶ whimsy/   the core in Rust
                                                      ├──▶ whimsical/ the core in Scheme
                                                      └──▶ whiml/    the core in OCaml
```

**The input** is `slim-vim.c`: vim 9.2 as one C translation unit, without
preprocessor or comments, from [arbace/slim-vim](https://github.com/arbace/slim-vim).
`make` fetches it, and vim's `LICENSE`, at that repository's head.

**The pipeline** removes capability on purpose.
- **The front, phases 1-5, cuts first.** Phases 1-3 cut every interface and
  feature the product has not, as twelve packages: the command line, the Ex
  commands, the files, the options, the swap file, startup, the encoding, the
  terminal, one window and buffer, the editing features, one regexp engine,
  the process. Then a generic fall-out closure folds what each cut left
  unwritten, and phases 4 and 5 cut what counts on the text it leaves.
- **The phases after it, in order,**
  - drop what can only be cut late;
  - rewire the core for a host: no filesystem, no libc it names, and the host
    across a line in the file;
  - make generic C rewrites for the translations.

Every phase edits the program as one resolved, typed graph
(`crefactor/graph`, `doc/GRAPH.md`), imported once from the input and handed
from phase to phase: its steps, then a collection of what nothing reaches,
then the C, printed in one canonical spelling. The phases are numbered 0 to
103 in the order they run (`doc/PHASES.md` maps the numbers they were written
under), and named in 34 blocks of three kinds: `d` drops, `r` rewires, `g`
generic steps. `make whim-build` runs them all in one process in about two
minutes. The product, `src/whim-vim.c`, is tracked, and `make
whim-build-check` requires it back byte for byte: every phase checked from its
own snapshot, side by side, and every boundary compiled, in about 35 seconds.

**The translations** are written by `crefactor/togo` from the core's C, not
from each other. `editor/editor.go`, `braaam/editor/`,
`vijure/src/whim/editor.clj`, `caprice/Caprice/Editor.hs` and
`whimsy/src/editor.rs`, `whimsical/whimsical/editor.ss` and `whiml/editor.ml` (with its `.mli`) are generated, tracked, and refused by
`make whim-editor-check` when stale.

**The tests.**
- `make whim-test` runs 80 key sessions on the C editor, built from the tree
  and from HEAD, and on the Go editor. It requires the same screens and exits,
  with a control that must move them.
- `make whim-test-wide` does the same with 240 cases: keys, every Ex command,
  command lines, and a real terminal.
- `make whim-test-java`, `make whim-test-clj`, `make whim-test-hs`,
  `make whim-test-rs`, `make whim-test-scm` and `make whim-test-ml` hold the
  Java, Clojure, Haskell, Rust, Scheme and OCaml editors to them too.
- `make go-test` runs the Go packages' tests.

## Use

```sh
make                   # fetch the input if it moved, then every editor: bin/whim,
                       # bin/whim-vim, bin/slim-vim, bin/braaam and braaam.jar,
                       # bin/vijure and vijure.jar, bin/caprice, bin/whimsy,
                       # bin/whimsical, bin/whiml
make whim-build        # the pipeline: slim-vim.c -> whim-vim.c and the translations
make whim-build-check  # the same, required to give the committed bytes back
make whim-editor-check # refuse a stale editor.go, braaam/editor/, editor.clj, Editor.hs, editor.rs, editor.ss or editor.ml(i)
make whim-test         # the quick suite; whim-test-wide for the wide one
make go-test           # the Go packages' tests
make bin/whim-vim      # the C editor's binary
make bin/braaam        # the Java editor, and a launcher: bin/braaam [args]
make braaam.jar        # the same as one jar: java -jar braaam.jar [args]
make bin/vijure        # the Clojure editor (doc/CLOJURE.md), and a launcher: bin/vijure [args]
make vijure.jar        # the same as one jar: java -jar vijure.jar [args]
make bin/caprice       # the Haskell editor (doc/HASKELL.md), compiled by GHC
make bin/whimsy        # the Rust editor (doc/RUST.md), compiled by cargo, offline
make bin/whimsical     # the Scheme editor (doc/SCHEME.md), compiled by Chez Scheme
make bin/whiml         # the OCaml editor (doc/OCAML.md), compiled by ocamlopt
make whim-test-java    # the quick suite with the Java editor too
make whim-test-clj     # ... with the Clojure editor
make whim-test-hs      # ... with the Haskell editor
make whim-test-rs      # ... with the Rust editor
make whim-test-scm     # ... with the Scheme editor
make whim-test-ml      # ... with the OCaml editor
make editor.lgo        # the Go editor as one go-lisp file (doc/GO-LISP.md)
make caprice.hsl       # the Haskell core as one ghc-lisp module (doc/GHC-LISP.md)
make whim-vim.lc       # the C product as s-expressions, back byte for byte (doc/C-LISP.md)
make help              # every target
```

## The editors

- **C**, `src/whim-vim.c`: the core, then the host from its first `#include` on.
- **Go**, `editor/`: `package editor`, a library. An `Editor` is one editor on a
  `Host` (the terminal, the clock, input, signals, output); a process holds as
  many as it makes. `editor/term` is the terminal host, `editor/cmd/whim` the
  launcher behind `bin/whim`.
- **Java**, **braaam** (`braaam/`): the same shape. `Editor.java` runs on a
  `Host`, with a runtime (`rt/`, a C pointer as an array and an offset) and a
  terminal host through the Foreign Function & Memory API.
- **Clojure**, **vijure** (`vijure/`): the namespace `whim.editor`, generated
  from basic blocks. It nests structured `let`/`loop` where the C's flow nests,
  and uses a `loop`/`case` state machine where it does not. It runs on the
  Java editor's runtime and host through interop; `whim.cljhost` is the glue
  and `whim.cljmain` the launcher.
- **Haskell**, **caprice** (`caprice/`): the module `Caprice.Editor`, written on
  C's own memory, raw. Its runtime is `Caprice.Rt`, its host a record of
  functions, its terminal host termios and poll; several editors run at once.
- **Rust**, **whimsy** (`whimsy/`): the crate `whimsy`, its module `editor`
  C's memory natively -- `#[repr(C)]` types, raw pointers, a reference only
  where its promise is proved -- and C's control flow, a goto a labeled
  block; safe functions where the C allows, signed arithmetic as Rust's plain
  operators (`doc/RUST-IDIOMS.md`). A `Host`
  trait, a terminal host over libc with real signal handlers, a hand port of
  vim's printf; several editors run at once.
- **Scheme**, **whimsical** (`whimsical/`): the R6RS library `(whimsical
  editor)` for Chez Scheme, C's memory as one bytevector per editor, a
  pointer an offset into it, read and written by the C's names (`curwin`,
  `(win_T.w_cursor.lnum wp)`); its functions joins and loops as local
  procedures and named lets, every jump a tail call, no `set!`. A host
  record, a terminal host through Chez's foreign procedures (signals by
  `signalfd`), a hand port of vim's printf; one executable with its boot
  files linked in; several editors run at once.
- **OCaml**, **whiml** (`whiml/`): the module `Editor` for OCaml 5, printed
  from the Scheme backend's forms with OCaml's types: C's memory as one
  `Bytes` per editor, the C's names its accessors (`curwin ed`,
  `Win_T.w_cursor_lnum ed wp`), joins and loops local functions, each
  cycle of calls one `let rec`, a function pointer an index into a table
  per arity. A host record, a terminal host on `Unix` and the C host's own
  signal handlers, a port of vim's printf, the parallel `:%s` on domains;
  several editors run at once.

## Requirements

Linux, **Go 1.27**, **gcc** linking statically against **musl**, `git`, `curl`,
and binutils; measured on Alpine Linux. `make` builds the other editors too,
so it also needs:
- a **JDK 22 or later**, which starts the editors fastest at JDK 25 or later
  (an AOT cache);
- the **`clojure`** command (Clojure 1.12, whose jars it copies);
- **GHC 9.14** with its boot packages, and no cabal;
- **rustc and cargo 1.98**, std only: no crate is fetched, the build is
  offline;
- **Chez Scheme 10.3** (`chez`, with its kernel, `libkernel.a` and
  `scheme.h`, as Alpine's `chez-scheme` installs them), and the shared lz4,
  zlib and ncursesw it links against;
- **OCaml 5.5** (`ocamlopt`, and the `unix` library it ships), no dune,
  ocamlfind or opam.

The C front end is a fork of `modernc.org/cc/v4` carried as source, so after
fetching the input nothing needs the network. The fork parses C23, which
upstream's parser does not wholly: `crefactor/c23conf` holds it, the
canonical printer and C-lisp to gcc 15, a file per language feature
(`doc/C23.md`).

## Layout

```
cmd/whim/        the toolset: go tool whim <subcommand>
internal/        the plan (internal/build), the cuts and steps, the phases
                 (internal/phase/NNN/: GOAL.md and edit.go; the records of
                 phases that edit nothing now in internal/phase/archive/), and
                 what the generic library is told about vim (internal/whim)
crefactor/       the generic C refactoring library, a Go module of its own:
                 the C front end, the canonical printer, C-lisp (C as
                 s-expressions and back), the program graph (its editor,
                 the verbs the phases are written on, the collection, the
                 fall-out closure and the transforms) and its read-only
                 views, the driver, the analyses, and togo, the C-to-Go,
                 Java, Clojure, Haskell, Rust, Scheme and OCaml translator
editor/          the editor in Go          braaam/   the editor in Java
vijure/          the editor in Clojure     caprice/  the editor in Haskell
whimsy/          the editor in Rust        whimsical/ the editor in Scheme
whiml/           the editor in OCaml
src/             the input (fetched) and the product (tracked)
doc/             GOALS.md (what holds for every phase, and the blocks),
                 AGENDA.md (what is not done), PIPELINE-REFORM.md (the
                 pipeline reordered: the front, the blocks, what was measured),
                 PIPELINE-COMPACTION.md, JAVA.md, CLOJURE.md, CLOJURE-PROFILE.md,
                 HASKELL.md, RUST.md, SCHEME.md, OCAML.md, the *-IDIOMS.md surveys, GO-LISP.md,
                 GHC-LISP.md, C-LISP.md, C-LISP-TREE.md, C23.md (the
                 front end against C23: the conformance test, its score),
                 GRAPH.md (the program as a graph, its views as Lisp: the
                 design, built), GRAPH-MIGRATION.md (every phase moved onto
                 it, and how to write one there), GUEST.md (the core as a
                 bare-metal guest on KVM and Hypervisor.framework: a design),
                 PARALLEL-SUBSTITUTE.md (how much of a :%s is matching),
                 IR.md and IR-SCHEMA.md (an intermediate representation)
CLAUDE.md        the working guide: the build, the pipeline, what to know
                 before changing anything shared
```

## License

The product is modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
