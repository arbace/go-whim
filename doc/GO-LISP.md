# The Go editor in go-lisp: an experiment

2026-09-25. [arbace/go-lisp](https://github.com/arbace/go-lisp) is a fork of the
Go toolchain (`go1.28-devel`) with a second front end: the whole Go language
written as s-expressions, in `.lgo` files. Its parser produces the same
`*syntax.File` as Go's, so type checking, SSA and the linker are Go's
unchanged; `go tool golisp go2lisp` / `lisp2go` convert between the two
syntaxes, and `go build` compiles `.lgo` packages. The question: can it be
used here, and what does it give?

## What was done, and measured

At go-lisp `70366138` ("golisp/upstream: add fixes for two upstream Go bugs"),
against go-whim `2b35fb0`:

1. **The toolchain builds, with one fix.** `src/make.bash` fails:
   `cmd/go/internal/imports/lisp.go: bootstrap-copied source file cannot
   import internal/golisp` -- the package is not in `cmd/dist`'s
   `bootstrapDirs`. With `"internal/golisp"` added to that list
   (`src/cmd/dist/buildtool.go`) it builds, in 38 s, bootstrapped from Go
   1.27.1. `make.bash` does not install the `golisp` tool, although its own
   documentation says `go tool golisp`; `go build -o bin/golisp
   ./cmd/compile/golisp` in the fork's `src/` does. Both are fixed upstream:
   [arbace/go-lisp#1](https://github.com/arbace/go-lisp/pull/1), merged
   2026-09-26 (`b335a27a`) and tidied after (`ee0852c8`: the bootstrap entry
   sorted, the `go tool` hook one line), so `cmd/dist` bootstraps
   `internal/golisp` and `go tool golisp` builds the tool on first use, as
   `go tool` does the other tools make.bash leaves unbuilt. Verified on a
   fresh clone of the `go-lisp` branch, with no patch: make.bash in 38 s,
   `go tool golisp run`, and `make editor.lgo GOLISP_ROOT=...`.
2. **go-whim builds and tests under it, unchanged**: both modules build and
   vet (`GOTOOLCHAIN=local`; `go.mod`'s `go 1.27.1` is accepted), and `whim
   test` passes.
3. **The Go editor converts, and runs.** `go2lisp` converts `editor/`'s five
   source files in 0.26 s (`editor.go` becomes 2.2 MB of s-expressions). With
   no `.go` source left in `editor/`, go-lisp's `go build` compiles the
   package in 4.8 s, its tests pass (the in-process editors, four at once,
   under `-race` too), and `whim test` holds it to the C: 45 of 45, and 240 of
   240 with `--wide`, the pseudo-terminal cases among them.
4. **The round trip is the same program.** `lisp2go` of each `.lgo` against
   the `.go` it came from, both parsed and printed by the standard library
   with three normalisations: comments dropped (`lisp2go` keeps only `;go:`
   directives), `a, b T` split into `a T, b T` (go-lisp's field pairs, as its
   SPEC says), and redundant parentheses unwrapped (`(v >> x) & 0xf` comes
   back `v >> x & 0xf`, the same expression: prefix forms carry the grouping,
   and the printer re-derives parentheses from precedence). All five files are
   then the same program -- 1.7 MB normalised for `editor.go` -- and a control,
   one `+ 1` made `+ 2`, is caught.
5. **The whole editor is one file.** A package may be one file, and `go2lisp`
   converts a file at a time, each `.lgo` opening with its own package clause
   and imports; so `go tool whim gocat editor` merges the package's source
   files into one Go file first -- one package clause, one import block, every
   file's declarations with their comments (59,810 lines) -- which alone in
   `editor/` passes `whim test` too. Converted, it is **one `editor.lgo`,
   2.27 MB, 52,657 lines**, which alone in `editor/` compiles under go-lisp and
   passes the package's tests, `whim test` (45/45) and `--wide` (240/240).

## What it is and is not good for here

- **It is not a front end for this pipeline.** go-whim parses C (`crefactor/cc`,
  modernc's C front end, forked); go-lisp's parser reads Go, and lives in
  `cmd/compile/internal/syntax`, which Go's `internal` rule keeps from being
  imported outside `cmd/compile`.
- **It is not a third translation.** `.lgo` is Go's AST in another spelling:
  the go-lisp editor is the Go editor, which is why it answers every case as
  the Go does. A `togo` backend writing `.lgo` would be `go2lisp` of its Go
  output, which it already is.
- **It is a good test of go-lisp.** A generated 58,000-line Go file, and a
  package of 60,000 lines as one file, converted, compiled and run against an
  independent oracle -- the C editor -- is a corpus the fork's own round-trip
  test (Go's source tree) does not have.

## Reproduce

```sh
git clone --depth 1 --branch go-lisp https://github.com/arbace/go-lisp.git .tmp/go-lisp
(cd .tmp/go-lisp/src && GOROOT_BOOTSTRAP=$(go env GOROOT) ./make.bash)
make editor.lgo GOLISP_ROOT=$PWD/.tmp/go-lisp
```

`make editor.lgo` writes `./editor.lgo` (untracked), converting with the
toolchain's `go tool golisp` and compiling with its `go`, and refuses a file
that does not compile; `GOLISP=path/to/golisp` works too, with the `go`
beside it. Without either it says what it needs. To run the suites on it, as
in (5): in a worktree, replace `editor/`'s non-test `.go` files by
`editor.lgo`, put the go-lisp `bin/` first on `PATH` with `GOTOOLCHAIN=local`,
and run `go tool whim test` (and `--wide`).
