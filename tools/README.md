# tools/

What a check or a delta runs, and the three wrappers that find the Go binary.
**The driver is gone**: the pipeline is `internal/build` (the plan and the
phases) and `internal/verify` (the same plan with every check and delta), and
there is no memoize, no boundary and no oracle between them.

Everything is reached the same way, as a subcommand of the one binary:

| | |
| --- | --- |
| `tools/st.sh build` | the 163 phases in one process; `--check` requires the committed product back, `--canonical` prints the input in canonical form at phase 0 first, `--keep-going` records a phase that refuses instead of stopping |
| `tools/st.sh verify` | every phase's check and every stage's declared delta |
| `tools/st.sh record` | the two baseline sets every delta is measured against |
| `tools/st.sh delta` | the declared delta at a phase, as a check: `BIN SRC --phase N`; `--declared N` prints what phase N itself declares, `--list FROM TO` a run of phases' declarations |
| `tools/st.sh zrecord` | one core recording: six harnesses at once, 122 records |
| `tools/st.sh phasecheck` | the standard gate on a work tree: `WORK SRC BEFORE` -- one compile, no warning but the fall-throughs, `main` the only external symbol, `nvidx`, and the libc surface against the stage's snapshot in `BEFORE` (which it removes), answers left in `.cache/symbols/last` (`internal/check`'s `PhaseCheck`) |
| `tools/st.sh phasebuild` | the work tree's binary: `WORK LINES-BEFORE`, linked from the sweep's object when it is of exactly this text, with the flags the work makefile resolves to, and `make` otherwise (`PhaseBuild`) |
| `tools/st.sh symbols` | a source's libc surface and external names, `FILE OUTDIR`, cached by content in `.cache/symbols` (`check.Symbols`; the verification writes every stage's snapshot with it) |
| `tools/st.sh score` | bytes to store and symbols to provide, slim-vim beside whim-vim; `make score` passes `WHIMCFLAGS` and `WHIMLDFLAGS` (`internal/verify`'s `Score`) |
| `tools/st.sh cemit` | one file in the canonical C23 form (`internal/cemit`), `--check` to ask whether it already is |
| `tools/st.sh parse` | the front end's smoke test, and the proof that the PATCHED `internal/cc` is what got linked |

| file | what it is |
| --- | --- |
| `st.sh`, `sweep.sh`, `canon.sh` | run `whimtools`: they build it if they must (`gobuild.sh`) and exec it. A phase program, a check, a makefile rule and a person at a prompt all reach the toolset the same way |
| `gobuild.sh` | builds `whimtools`, content-keyed on go.mod, go.sum and every .go under cmd/, internal/ and phase/ |
| `enumvals.sh` | every enumerator's value from DWARF, before and after -- kept as shell on purpose: twelve phase checks use it as the control that is independent of the Go |
| `templates/whim.mk`, `templates/core.mk` | the makefile phase 0 starts from, and the one phase 83 writes over it |
| `musl-case.txt`, `musl-ctype.txt`, `nolibm_check.c` | data and a probe a phase reads |

Every one of them runs from the repository root and writes its temporaries in
`.tmp/`.

## Retired, and what became of them

The prose names about ninety scripts that are not here. Almost every mention is
a **record** -- what was run when a phase was measured, and what it printed --
and a record keeps the name it was measured with; renaming it to today's tool
would claim a measurement nobody made. This table is where such a name leads.
The Python was never tracked in this repository: it is arbace/slim-vim's
history from before the split (`8ba7c9d`), ported to Go there or here, one
subcommand per script.

| retired | what it is now | gone in |
| --- | --- | --- |
| `phaserun.sh` (a stage from its boundary), `verifypass.sh` | `tools/st.sh verify --from N --to M --src BOUNDARY` (`internal/verify`) | `4496964` |
| `stages.sh`, `packages.sh` | the stages are `internal/build/plan.go`; `need`, `apart`, `package` and `uses` are prose in `phase/STAGES.md`, and nothing checks them | `4496964` |
| `pipeline.sh` (`PHASE_LIST`, `CORE_FROM`) | `internal/build`'s `Plan` and `CoreFrom` | `4496964` |
| `memo.sh`, `implhash.sh`, `oracle.sh`, `snapshot.sh`, `restore.sh`, `specpass.sh`, `residue.sh`, `phasename.sh` | nothing: the memoize and its keys went, and the tracked product is what answers for the pipeline (`make whim-build-check`) | `4496964` |
| `whimdelta.sh`, `coredelta.sh` (earlier `zerodelta.sh`), `declared.sh` | `tools/st.sh delta BIN SRC --phase N`, `--declared N`, `--list FROM TO` (`internal/verify`'s `Delta`, `CoreDelta`, `Declarations`) | `d8b3c29` (`zerodelta.sh` renamed in `d3ac925`) |
| `zrecord.sh` | `tools/st.sh zrecord` (`internal/harness`'s `ZRecord`; `check.RecZ` from a check) | `d8b3c29` |
| `phasecheck.sh`, `phasebuild.sh`, `symbols.sh` | `tools/st.sh phasecheck`, `phasebuild`, `symbols` (`internal/check`'s `PhaseCheck`, `PhaseBuild`, `Symbols`, the first two called in process by every check, the third by the verification for each stage's snapshot) | COMMIT |
| `score.sh` | `tools/st.sh score` (`internal/verify`'s `Score`), which `make score` runs | COMMIT |
| `phase/NNN/make.sh`, `edit.sh`, `check.sh` | `phase/NNN/edit.go` and `check.go`, package `pNNN` | `f0a58b9` |
| `deadsweep.py`, `deadprotos.py`, `typereach.py`, `funcreach.py`, `deadfields.py`, `deadenums.py`, `orphanopts.py`, `nvidxcheck.py` | `whimtools` of the same name (`nvidx` for the last), in `internal/dead` | before the split |
| `canon.py`, `cutil.py` and the canonicaliser's pieces (`brace.py`, `onestmt.py`, `onedecl.py`, `forcomma.py`) | `internal/canon`, `internal/cutil` | before the split |
| `behaviour.py`, `exsweep.py`, `termcheck.py`, `clicheck.py`, `starcheck.py`, `complcheck.py`, `termrestore.py`, `muslcase.py`, `muslctype.py`, `create_cmdidxs.py` | `whimtools` of the same name (`cmdidxs` for the last), in `internal/harness` | before the split |
| `zscreen.py`, `zstream.py`, `zrec.py`, `zcases.py`, `zexcmds.py`, `zargv.py`, `zpty.py`, `zmemline.py`, `ztermcheck.py`, `zhostonly.py`, `zcompare.py` | `internal/harness`'s `z*.go`, each also a `whimtools` subcommand where it has a command line | before the split |
| a phase's cutter (`noswap.py`, `nosession.py`, `retire.py`, `dropoptions.py`, …) | the step of that name in `internal/steps`, most of them also a `whimtools` subcommand | before the split |
| `ptyrun.py`, `ptycheck.py` | the pty driver in `internal/harness` (`pty.go`, `ptyprobes.go`, `ptysplit.go`) | before the split |
| `arrowcheck.py`, `coverage.sh` | not ported; nothing runs them, and the text naming them is the record of what they measured | before the split |
| `graph.py`, `sim.py`, `corpus.py`, `run2.py`, `argvcheck.py`, `allstatic.py`, `exsweep_stream.py`, `.tmp/…/seq.sh` and the like | throwaway probes under `/tmp` or `.tmp/`, never tracked; the text naming them says what they measured | -- |
