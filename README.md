# go-whim

A pipeline, written in Go, that takes vim apart on purpose -- and the editor it
leaves, nine times over: in C, in Go, in Java, in Clojure, in Haskell, in
Rust, in Scheme, in OCaml and in C++, each required to answer every test case as the
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
                                                      ├──▶ whiml/    the core in OCaml
                                                      └──▶ wpp/      the core in C++ (whim++)
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
`whimsy/src/editor.rs`, `whimsical/whimsical/editor.ss`, `whiml/editor.ml` (with its `.mli`) and `wpp/src/editor.cpp` (with its header) are generated, tracked, and refused by
`make whim-editor-check` when stale.

**The tests.**
- `make whim-test` runs 80 key sessions on the C editor, built from the tree
  and from HEAD, and on the Go editor. It requires the same screens and exits,
  with a control that must move them.
- `make whim-test-wide` does the same with 240 cases: keys, every Ex command,
  command lines, and a real terminal.
- `make whim-test-java`, `-clj`, `-hs`, `-rs`, `-scm`, `-ml` and `-cpp` hold
  the Java, Clojure, Haskell, Rust, Scheme, OCaml and C++ editors to them too.
- `make whim-test-guest` holds the C core run as a virtual machine on KVM
  (`guest/`, `vmm/`, `hv/`, `doc/GUEST.md`) to them as well.
- `make whim-test-guest-go TAMAGO_ROOT=...` holds the Go editor run as one,
  built with TamaGo (`guest/tamago/`), to them too.
- `make go-test` runs the Go packages' tests.

## Use

```sh
make                   # fetch the input if it moved, then every editor and jar
make help              # every target, grouped as below

# the pipeline
make whim-build        # slim-vim.c -> whim-vim.c and the translations
make whim-build-check  # the same, required to give the committed bytes back
make whim-editor-check # refuse a stale generated translation
make bin/whim-vim      # the C editor's binary

# the editors (each with a whim-test-* target: the quick suite, held to the C)
make bin/whim          # Go           (editor/)
make bin/braaam        # Java         (doc/JAVA.md); braaam.jar: java -jar braaam.jar
make bin/vijure        # Clojure      (doc/CLOJURE.md); vijure.jar likewise
make bin/caprice       # Haskell      (doc/HASKELL.md), by GHC
make bin/whimsy        # Rust         (doc/RUST.md), by cargo, offline
make bin/whimsical     # Scheme       (doc/SCHEME.md), by Chez Scheme
make bin/whiml         # OCaml        (doc/OCAML.md), by ocamlopt
make bin/whim++        # C++          (doc/CPP.md), by g++ -std=c++23

# the editor as a virtual machine (doc/GUEST.md)
make bin/whim-guest    # the C core as a guest on KVM: bin/whim-guest [args]
make bin/whim-guest-go TAMAGO_ROOT=/path/to/tamago-go  # the Go editor as one, built with TamaGo
make mac-images TAMAGO_ROOT=/path/to/tamago-go         # the Mac's three arm64 images, in lib/whim-guest/ (guest/mac/mac.sh)

# the tests
make whim-test         # the quick suite; whim-test-wide for the wide one
make whim-test-java    # ... and -clj, -hs, -rs, -scm, -ml, -cpp, -guest, -guest-go
make go-test           # the Go packages' tests

# the Lisp spellings
make editor.lgo        # the Go editor as one go-lisp file (doc/GO-LISP.md)
make caprice.hsl       # the Haskell core as one ghc-lisp module (doc/GHC-LISP.md)
make whim-vim.lc       # the C product as s-expressions, back byte for byte (doc/C-LISP.md)
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
- **C++**, **whim++** (`wpp/`): the core's own C printed as C++23, the
  editor a `class Editor` -- its file-scope objects the fields, its
  functions the members (`static` where they reach nothing of it), a
  function pointer a pointer to a member -- with what C++ says otherwise
  written as C++ says it: named casts, `using`, `enum class` where an
  enumeration is a type, references where a pointer is never null,
  `[[nodiscard]]`. A `Host` interface, a terminal host on the C host's own
  calls, the C host's printf, the parallel `:%s` on `std::jthread`;
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
  ocamlfind or opam;
- **g++ 15** (`-std=c++23`) and its libstdc++.

The core as a virtual machine (`make bin/whim-guest`) needs **clang** (for
x86-64 and AArch64), GNU **ld** (and `aarch64-none-elf-ld` for arm64), and
`/dev/kvm` to run; `make mac-images` builds the three arm64 images the Mac
runs beside its monitor (`guest/mac/mac.sh`). The Go editor as one
(`make bin/whim-guest-go`) needs
**TamaGo** (github.com/usbarmory/tamago-go, the release matching `go.mod`'s
Go), named by `TAMAGO_ROOT`; it fetches the module
`github.com/usbarmory/tamago` once.

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
                 Java, Clojure, Haskell, Rust, Scheme, OCaml and C++ translator
editor/          the editor in Go          braaam/   the editor in Java
vijure/          the editor in Clojure     caprice/  the editor in Haskell
whimsy/          the editor in Rust        whimsical/ the editor in Scheme
whiml/           the editor in OCaml       wpp/      the editor in C++ (whim++)
whiml/           the editor in OCaml
hv/              a virtual machine as Hypervisor.framework shapes one, on KVM
vmm/ guest/      the monitor that runs the core as a guest, and the guest's builder
                 (guest/tamago/: the Go editor as a guest, a module built by TamaGo)
src/             the input (fetched) and the product (tracked)
doc/             GOALS.md (what holds for every phase, and the blocks),
                 AGENDA.md (what is not done), PIPELINE-REFORM.md (the
                 pipeline reordered: the front, the blocks, what was measured),
                 PIPELINE-COMPACTION.md, JAVA.md, CLOJURE.md, CLOJURE-PROFILE.md,
                 HASKELL.md, RUST.md, SCHEME.md, OCAML.md, CPP.md, the *-IDIOMS.md surveys, GO-LISP.md,
                 GHC-LISP.md, C-LISP.md, C-LISP-TREE.md, C23.md (the
                 front end against C23: the conformance test, its score),
                 GRAPH.md (the program as a graph, its views as Lisp: the
                 design, built), GRAPH-MIGRATION.md (every phase moved onto
                 it, and how to write one there), GUEST.md (the core as a
                 bare-metal guest: on KVM amd64 and arm64, built; on
                 Hypervisor.framework, designed; and the Go editor as one,
                 on TamaGo),
                 PARALLEL-SUBSTITUTE.md (how much of a :%s is matching),
                 IR.md and IR-SCHEMA.md (an intermediate representation)
CLAUDE.md        the working guide: the build, the pipeline, what to know
                 before changing anything shared
```

## License

The product is modified vim, under vim's licence: `LICENSE`, fetched with
`slim-vim.c`, unmodified, as its clause II.1 requires.
