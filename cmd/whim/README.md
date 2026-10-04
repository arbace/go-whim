# cmd/whim -- the toolset

The toolset is `cmd/whim`, declared as a tool in `go.mod`, so it runs as
`go tool whim <subcommand>`: Go builds it, caches it, and rebuilds it when any
`.go` moves. The `Makefile` calls it the same way. The test suite is
`whim test` (`internal/suite`), minimal; the checks, the deltas, the recorders
and the baselines of the one before it were removed after `448e9a8`, the last
commit that has them.

| | |
| --- | --- |
| `go tool whim build` | the 104 phases, numbered 0-103, in one process; `--check` requires the committed product back and compiles and links every boundary, `--canonical` prints the input in canonical form at phase 0 first, `--keep-going` records a phase that refuses instead of stopping |
| `go tool whim test` | the quick suite: the editor on the key sessions in `internal/suite/cases.md`, built from the working tree and from `--ref REV` (HEAD), required to print the same, and the Go editor as the C; a control must move one. `--wide` runs the optional wide suite instead: keystrokes with arguments (`wide-cases.md`), every Ex command, command lines (`wide-argv.md`) and a real terminal (`wide-pty.md`) (`internal/suite`). `--java`, with either, adds the Java editor, built from FILE, held to the C with a control of its own; `--clojure` the Clojure editor the same way (`--clojure-editor F`: with the namespace in F, not generated); `--haskell` the Haskell editor, caprice (`--haskell-bin P`: the program P, built already, its control P with its one `" INSERT"` changed to `" INSERX"`, and the heavy case's bound not applied to it); `--rust` the Rust editor, whimsy; `--scheme` the Scheme editor, whimsical (`--scheme-debug`: its debugging build, `whim whimsical --debug`'s). `--limit D` (10s) is how long one run of a case may take on the editors these add and their controls; the C builds and the Go editor keep 10s |
| `go tool whim java` | the editor in Java (`braaam/`, doc/JAVA.md): the core cut from `src/whim-vim.c` (or FILE), written as the package `whim.editor` (`Editor.java` and its classes' files), compiled with braaam's runtime, host and glue into `lib/braaam/classes` (`--out DIR`), and `bin/braaam`, a launcher script (`DIR/braaam` with `--out`); `make braaam.jar` packs the classes as `./braaam.jar` |
| `go tool whim clj` | the editor in Clojure (`vijure/`, doc/CLOJURE.md): the core cut from `src/whim-vim.c` (or FILE) and written as the namespace `whim.editor` by the Clojure backend (`--editor F`: the namespace in F instead), AOT-compiled with vijure's glue and launcher on braaam's runtime and host, merged with Clojure's jars into `lib/vijure/vijure.jar` (`--out DIR`) with its AOT cache, and `bin/vijure`, a launcher script; `--jar F` writes the same jar at F (`make vijure.jar`) |
| `go tool whim sweep` | the text's sweep on one file: one reachability closure over its parse, and everything it did not reach cut out (`crefactor/sweep`); the pipeline collects on the graph instead, by the same rules (`graph --collect`) |
| `go tool whim cemit` | one file in the canonical C23 form (`crefactor/cemit`), `--check` to ask whether it already is |
| `go tool whim c2lisp`, `lisp2c` | C as s-expressions and back (`crefactor/clisp`, C-lisp: `crefactor/clisp/SPEC.md`, doc/C-LISP.md): `c2lisp [-o OUT] [FILE]` writes the `.lc`, `lisp2c [-o OUT] [FILE.lc]` the C in cemit's canonical spelling; `c2lisp --check FILE...` converts each there and back and says OK only when the bytes are the input's (the canonical text's, for an input that is not canonical, and it says so), `lisp2c --check F.lc F.c` that F.lc prints as F.c and F.c converts to F.lc, byte for byte. `make whim-vim.lc` |
| `go tool whim graph` | the program as one resolved, typed graph (`crefactor/graph`, doc/GRAPH.md): `graph [-o OUT] FILE` writes it as Lisp -- the forms with their ids and edges, the type nodes, the externs; `graph --check FILE...` holds each file to steps 1 and 2 (its C view the file byte for byte, written and read back the same graph) and says what the import resolved; `graph --collect [-o OUT] FILE` is the sweep as garbage collection, vim's roots and guard, the C view of what is left. Step 3's gate on every phase is `internal/graphcheck`'s test (`GRAPH_SNAPS`) |
| `go tool whim view` | a read-only VIEW of the graph, a tree printed as Lisp (`crefactor/graph/view`, doc/GRAPH.md, *Views, read-only*): `view [--ids] [--depth N] [--show node\|stmt\|fn\|none] [--stop HEADS] VIEW ARG [FILE]`, VIEW one of `callers F`, `callees F` (each with the calling statements, to a depth, a function the view holds already a link), `uses NAME` (every use, grouped by the top-level form holding it, each statement labelled `:read`, `:write`, `:update`, `:addr`, `:call` ...), `member S.M` (the same for a member, resolved by type), `type T` (a struct: its members and their uses, the functions taking and returning it, the members and objects of it), `def NAME` (the definition, C-lisp with ids; `--c` its C) or `follow 'STEPS' ROOT` (ad hoc: `refers<`, `typed`, `inside`, `^fn`, `call` ...); a root is a name, `S.M`, `struct T`, `F/local` or `#ID`. FILE is `src/whim-vim.c` by default, or a graph's Lisp; a C file's graph is imported once (1.6 s) and kept in `.cache/graph/`, keyed by its digest and the binary's, so later views start in 0.07 s |
| `go tool whim parse` | the front end's smoke test, and the proof that the PATCHED `crefactor/cc` is what got linked |
| `go tool whim reach` | what nothing reaches in a text, as a partition with gcc as its control (`crefactor/reach`); it deletes nothing |
| `go tool whim measure` | one row per boundary a `build --keep D` left: lines, entity counts, binary, undefined symbols (`internal/phase/boundaries.md`) |
| `go tool whim cdiff [-n N] A.c B.c` | the first difference between two C files, entity by entity: each top-level function, object, type, tag and assertion keyed by what it declares; the ones that differ (their first differing lines), the ones only in either, and whether the order is the same. Exit 0 only when they are the same (`doc/PIPELINE-REFORM.md` §7) |
| `go tool whim score` | bytes to store and symbols to provide, slim-vim beside whim-vim, both built with the one line (`internal/score`) |
| `go tool whim cmdnames` | the Ex command table's names (`internal/cmdtab`); the ex_cmdidxs block they derived went at phase 1 (the reform's D2b) |
| `go tool whim cut` | the core of src/whim-vim.c (or FILE) on stdout: everything before the first `#include` (internal/whim's `Cut`), what every translation is written from |
| `go tool whim gen` | editor/editor.go (and internal/gen/sigs.md) from whim-vim.c's core, which it cuts itself, written only when it differs; `--check` refuses a stale one (`internal/gen`) |
| `go tool whim caprice` | the editor in Haskell (`caprice/`, doc/HASKELL.md): the core cut from `src/whim-vim.c` (or FILE), written as the module `Caprice.Editor` (and its hs-boot) by the Haskell backend, compiled by GHC with caprice's runtime, host and launcher in `lib/caprice` (`--out DIR`), and the program `bin/caprice` (`DIR/caprice` with `--out`); a build whose core has not moved skips its three minutes |
| `go tool whim hscat` | the Haskell core as ONE module (doc/GHC-LISP.md): `Caprice.Editor` written unsplit (`HsParts` 1) on stdout; with `--ghc GHC` (ghc-lisp's), converted by its `--hs2lisp`, its round trip held by `--lisp-check`, and compiled by that ghc at `-O0` with caprice's runtime, host and launcher, the program run on the quick suite as `whim test --haskell-bin PROG --limit 2m` runs it (`--no-test` skips it), the `.hsl` to `--hsl FILE` (`--out DIR` keeps the work and the program). `make caprice.hsl` |
| `go tool whim whimsical` | the editor in Scheme (`whimsical/`, doc/SCHEME.md): the core cut from `src/whim-vim.c` (or FILE), written as the library `(whimsical editor)` by the Scheme backend, compiled by Chez Scheme with whimsical's runtime, host and launcher into one boot file in `lib/whimsical` (`--out DIR`), linked with Chez's kernel into the program `bin/whimsical` (`DIR/whimsical` with `--out`); a build whose core has not moved skips its half minute. `--debug` builds the debugging build instead, beside the release one: optimize-level 2 (safe: an access out of bounds is an error, not corrupted memory), the inspector's information kept, in `lib/whimsical-debug` and `bin/whimsical-debug` (doc/SCHEME.md §14) |
| `go tool whim skel`, `pre` | the generator by hand (`-bodies` for the bodies alone, `-java` for the Java class: doc/JAVA.md, `-clj` the Clojure namespace, `-hs` the Haskell module, `-scm` the Scheme library), and crefactor/ccx's partitions on an editor.c |

`go tool whim` with no argument lists the rest: `funcreach` and `fold` (text
tools on a file), `edit whimN FILE` (a phase's program), `query whim2 FILE`,
and the cutters a phase names. A phase's program, `droplocal` and a cutter of
the plan's graph table run on the file imported and write its C view back
(`internal/steps`' `OnText`); the front's cutters (phases 1-3: `notags`,
`noterm`, `utf8only` and 28 more) are listed but refuse, *no step named*,
since they run only inside the front's graph steps. Every one runs from the
repository root and writes its temporaries in `.tmp/`.

## Retired, and what became of them

The prose names about ninety scripts that are not here. Almost every mention is
a **record** -- what was run when a phase was measured, and what it printed --
and a record keeps the name it was measured with; renaming it to today's tool
would claim a measurement nobody made. This table is where such a name leads.
The Python was never tracked in this repository: it is arbace/slim-vim's
history from before the split (`8ba7c9d`), ported to Go there or here, one
subcommand per script.

Where the middle column names a verification tool -- `tools/st.sh verify`,
`record`, `delta`, `zrecord`, `phasecheck`, `phasebuild`, `symbols`, anything in
`internal/check`, `internal/verify` or `internal/harness` -- that successor went
too, with the test suite; `448e9a8` is the last commit that has it.

| retired | what it is now | gone in |
| --- | --- | --- |
| `phaserun.sh` (a stage from its boundary), `verifypass.sh` | `tools/st.sh verify --from N --to M --src BOUNDARY` (`internal/verify`) | `4496964` |
| `stages.sh`, `packages.sh` | the stages are `internal/build/plan.go`; `need`, `apart`, `package` and `uses` are prose in `internal/phase/STAGES.md`, and nothing checks them | `4496964` |
| `pipeline.sh` (`PHASE_LIST`, `CORE_FROM`) | `internal/build`'s `Plan` and `CoreFrom` | `4496964` |
| `memo.sh`, `implhash.sh`, `oracle.sh`, `snapshot.sh`, `restore.sh`, `specpass.sh`, `residue.sh`, `phasename.sh` | nothing: the memoize and its keys went, and the tracked product is what answers for the pipeline (`make whim-build-check`) | `4496964` |
| `whimdelta.sh`, `coredelta.sh` (earlier `zerodelta.sh`), `declared.sh` | `tools/st.sh delta BIN SRC --phase N`, `--declared N`, `--list FROM TO` (`internal/verify`'s `Delta`, `CoreDelta`, `Declarations`) | `d8b3c29` (`zerodelta.sh` renamed in `d3ac925`) |
| `zrecord.sh` | `tools/st.sh zrecord` (`internal/harness`'s `CoreRecord`; `check.RecCore` from a check) | `d8b3c29` |
| `phasecheck.sh`, `phasebuild.sh`, `symbols.sh` | `tools/st.sh phasecheck`, `phasebuild`, `symbols` (`internal/check`'s `PhaseCheck`, `PhaseBuild`, `Symbols`, the first two called in process by every check, the third by the verification for each stage's snapshot) | `016ce45` |
| `score.sh` | `tools/st.sh score` (`internal/verify`'s `Score`), which `make score` runs | `016ce45` |
| `canon.sh` | `tools/st.sh canon FILE [--once]` (`internal/canon`'s `Run`; `check.Canon` from a check) | `942db23` |
| `internal/phase/NNN/make.sh`, `edit.sh`, `check.sh` | `internal/phase/NNN/edit.go` and `check.go`, package `pNNN` | `f0a58b9` |
| `deadsweep.py`, `deadprotos.py`, `typereach.py`, `funcreach.py`, `deadfields.py`, `deadenums.py`, `orphanopts.py`, `nvidxcheck.py` | `whimtools` of the same name (`nvidx` for the last), in `internal/dead` (now `crefactor/dead`) | before the split |
| `canon.py`, `cutil.py` and the canonicaliser's pieces (`brace.py`, `onestmt.py`, `onedecl.py`, `forcomma.py`) | `internal/canon`, `internal/cutil` (now `crefactor/edit`) | before the split |
| `behaviour.py`, `exsweep.py`, `termcheck.py`, `clicheck.py`, `starcheck.py`, `complcheck.py`, `termrestore.py`, `muslcase.py`, `muslctype.py`, `create_cmdidxs.py` | `whimtools` of the same name (`cmdidxs` for the last), in `internal/harness` | before the split |
| `zscreen.py`, `zstream.py`, `zrec.py`, `zcases.py`, `zexcmds.py`, `zargv.py`, `zpty.py`, `zmemline.py`, `ztermcheck.py`, `zhostonly.py`, `zcompare.py` | `internal/harness`'s `z*.go`, each also a `whimtools` subcommand where it has a command line | before the split |
| a phase's cutter (`noswap.py`, `nosession.py`, `retire.py`, `dropoptions.py`, …) | the cutter of that name in `internal/cut`, on the graph, which the plan names as a step | before the split |
| `ptyrun.py`, `ptycheck.py` | the pty driver in `internal/harness` (`pty.go`, `ptyprobes.go`, `ptysplit.go`) | before the split |
| `arrowcheck.py`, `coverage.sh` | not ported; nothing runs them, and the text naming them is the record of what they measured | before the split |
| `graph.py`, `sim.py`, `corpus.py`, `run2.py`, `argvcheck.py`, `allstatic.py`, `exsweep_stream.py`, `.tmp/…/seq.sh` and the like | throwaway probes under `/tmp` or `.tmp/`, never tracked; the text naming them says what they measured | -- |
| `enumvals.sh`, `sweep.sh`, `nolibm_check.c`; the `whimtools` subcommands `verify`, `record`, `delta`, `check`, `phasecheck`, `phasebuild`, `symbols`, `nvidx`, `orphanopts`, `behaviour`, `termcheck`, `exsweep`, `starcheck`, `termrestore`, `complcheck`, `clicheck`, `muslctype`, `muslcase` and the ten `z*` | nothing: they were the test suite (the DWARF control, the checks' sweep, a phase-23 probe, the verifier, the recorders) | with the test suite, after `448e9a8` |
| `st.sh`, `gobuild.sh` | `go tool whim <subcommand>`: `go.mod` declares `cmd/whim` (renamed from `cmd/whimtools`) as a tool, and Go builds and caches it | the commit that made the toolset a go tool |
| `tools/musl-ctype.txt`, `tools/musl-case.txt`, and `tools/` itself | `internal/phase/037/musl-ctype.md` and `musl-case.md`, embedded in phase 37's edit (the fenced block, byte for byte); this file moved to `cmd/whim/README.md` | the commit that embedded them |
| `internal/canon` and the subcommands `canon`, `blankruns`, `joinparens`, `splitheads`, `brace`, `onestmt`, `onedecl`, `forcomma` | nothing: `crefactor/cemit` is the one canonical printer, whose text the graph's C view prints at the end of every phase (`go tool whim cemit` by hand); the sweep stopped running these | the commit that removed them, after 38be391 |
| the subcommands `deadsweep`, `deadprotos`, `typereach`, `deadfields`, `deadenums`, and `crefactor/sweep`'s `WHIM_CLOSURE` switch | `crefactor/sweep`'s closure (`Prune`), which replaced the six-tool loop and is now, rule for rule, the graph's collection; `funcreach` stays, a tool no phase runs now, and `crefactor/dead` keeps it, `FuncDefinitions` and the gcc warnings `reach` uses as its control | the commit that replaced the sweep |
