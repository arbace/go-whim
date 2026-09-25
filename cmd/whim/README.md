# cmd/whim -- the toolset

The toolset is `cmd/whim`, declared as a tool in `go.mod`, so it runs as
`go tool whim <subcommand>`: Go builds it, caches it, and rebuilds it when any
`.go` moves. The `Makefile` calls it the same way. The test suite is
`whim test` (`internal/suite`), minimal; the checks, the deltas, the recorders
and the baselines of the one before it were removed after `448e9a8`, the last
commit that has them.

| | |
| --- | --- |
| `go tool whim build` | the 164 phases in one process; `--check` requires the committed product back, `--canonical` prints the input in canonical form at phase 0 first, `--keep-going` records a phase that refuses instead of stopping |
| `go tool whim test` | the quick suite: the editor on the key sessions in `internal/suite/cases.md`, built from the working tree and from `--ref REV` (HEAD), required to print the same, and the Go editor as the C; a control must move one. `--wide` runs the optional wide suite instead: keystrokes with arguments (`wide-cases.md`), every Ex command, command lines (`wide-argv.md`) and a real terminal (`wide-pty.md`) (`internal/suite`) |
| `go tool whim sweep` | the sweep on one file: one reachability closure over its parse, and everything it did not reach cut out (`crefactor/sweep`) |
| `go tool whim cemit` | one file in the canonical C23 form (`crefactor/cemit`), `--check` to ask whether it already is |
| `go tool whim parse` | the front end's smoke test, and the proof that the PATCHED `crefactor/cc` is what got linked |
| `go tool whim reach` | what nothing reaches in a text, as a partition with gcc as its control (`crefactor/reach`); it deletes nothing |
| `go tool whim measure` | one row per boundary a `build --keep D` left: lines, entity counts, binary, undefined symbols (`internal/phase/boundaries.md`) |
| `go tool whim score` | bytes to store and symbols to provide, slim-vim beside whim-vim, both built with the one line (`internal/score`) |
| `go tool whim cmdidxs`, `cmdnames` | the Ex command table: its names, and the ex_cmdidxs block derived from them (`internal/cmdtab`) |
| `go tool whim gen` | editor/editor.go (and internal/gen/sigs.md) from editor.c, written only when it differs; `--check` refuses a stale one (`internal/gen`) |
| `go tool whim skel`, `pre` | the generator by hand (`-bodies` for the bodies alone), and crefactor/ccx's partitions on an editor.c |

`go tool whim` with no argument lists the rest: the dead-code tools one at a
time and every cutter a phase names, each runnable on a file by hand. Every one
runs from the repository root and writes its temporaries in `.tmp/`.

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
| a phase's cutter (`noswap.py`, `nosession.py`, `retire.py`, `dropoptions.py`, …) | the step of that name in `internal/steps`, most of them also a `whimtools` subcommand | before the split |
| `ptyrun.py`, `ptycheck.py` | the pty driver in `internal/harness` (`pty.go`, `ptyprobes.go`, `ptysplit.go`) | before the split |
| `arrowcheck.py`, `coverage.sh` | not ported; nothing runs them, and the text naming them is the record of what they measured | before the split |
| `graph.py`, `sim.py`, `corpus.py`, `run2.py`, `argvcheck.py`, `allstatic.py`, `exsweep_stream.py`, `.tmp/…/seq.sh` and the like | throwaway probes under `/tmp` or `.tmp/`, never tracked; the text naming them says what they measured | -- |
| `enumvals.sh`, `sweep.sh`, `nolibm_check.c`; the `whimtools` subcommands `verify`, `record`, `delta`, `check`, `phasecheck`, `phasebuild`, `symbols`, `nvidx`, `orphanopts`, `behaviour`, `termcheck`, `exsweep`, `starcheck`, `termrestore`, `complcheck`, `clicheck`, `muslctype`, `muslcase` and the ten `z*` | nothing: they were the test suite (the DWARF control, the checks' sweep, a phase-23 probe, the verifier, the recorders) | with the test suite, after `448e9a8` |
| `st.sh`, `gobuild.sh` | `go tool whim <subcommand>`: `go.mod` declares `cmd/whim` (renamed from `cmd/whimtools`) as a tool, and Go builds and caches it | the commit that made the toolset a go tool |
| `tools/musl-ctype.txt`, `tools/musl-case.txt`, and `tools/` itself | `internal/phase/098/musl-ctype.md` and `musl-case.md`, embedded in phase 98's edit (the fenced block, byte for byte); this file moved to `cmd/whim/README.md` | the commit that embedded them |
| `internal/canon` and the subcommands `canon`, `blankruns`, `joinparens`, `splitheads`, `brace`, `onestmt`, `onedecl`, `forcomma` | nothing: `crefactor/cemit` is the one canonical printer, run at the end of every phase (`go tool whim cemit` by hand); the sweep stopped running these | the commit that removed them, after 38be391 |
| the subcommands `deadsweep`, `deadprotos`, `typereach`, `deadfields`, `deadenums`, and `crefactor/sweep`'s `WHIM_CLOSURE` switch | `crefactor/sweep`'s closure (`Prune`), which replaced the six-tool loop; `funcreach` stays, a step some phases run mid-edit, and `crefactor/dead` keeps it, `FuncDefinitions` and the gcc warnings `reach` uses as its control | the commit that replaced the sweep |
