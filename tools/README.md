# tools/

The shell half of the pipeline: the memoize driver, and the three wrappers
through which every phase runs Go. Everything else is `cmd/whimtools` and
`internal/`.

| file | what it is |
| --- | --- |
| `memo.sh` | one unit of the pipeline: tier 3 (the recorded result), else tier 2 (the program) |
| `phaserun.sh` | one stage. **shared** (`stage A-B`): the symbol snapshot, every edit in order, one sweep, every check, the delta. **each** (`stage A-B each`): every edit followed by its own sweep, then every check and its delta at once, each in a root of its own on exactly the tree it had as a stage of one |
| `stages.sh`, `packages.sh` | read and check `phase/stages`: the stages and their mode, `need` and `apart`; the packages and `uses` |
| `implhash.sh` | the implementation half of a key: a phase's programs, every path they name, the declared delta and the stage's mode |
| `snapshot.sh`, `restore.sh` | a boundary: a tar and a content digest of a tree |
| `oracle.sh` | compare a boundary with the recorded one in `.reference/whim-phases/` |
| `verifypass.sh` | every unit at once on the recorded boundary before it (`make whim-verify`) |
| `specpass.sh` | every unit at once on the previous pass's boundaries, into the cache (`make whim-specpass`) |
| `pipeline.sh` | the pipeline's parameters: tag, work directory, source, delta checker, `ZERO_FROM` (the phase where the second baselines begin), `phasedir` (a phase's directory, `phase/NNN`), and the phase list, read from `phase/stages` |
| `st.sh`, `sweep.sh`, `canon.sh` | the wrappers that run `whimtools`; they name `cmd/`, `internal/`, `go.mod` and `go.sum` so every Go file is in every key that runs Go |
| `gobuild.sh` | builds `whimtools`, content-keyed, with the patched `modernc.org/cc/v4` (`patches/cc-v4-c23.patch`) |
| `phasecheck.sh`, `phasebuild.sh`, `symbols.sh` | what every check runs: the sweep's silence, linkage, the libc surface, the build |
| `whimdelta.sh`, `zerodelta.sh` | a stage's declared delta, every phase's `delta` read through `declared.sh`: against `.reference/baselines` for phases 0-82, and from `ZERO_FROM` on `whimdelta.sh` hands the phase to `zerodelta.sh`, against `.reference/zero-baselines` |
| `declared.sh` | the declarations of a run of phases, from their `phase/NNN/delta` files, in the one grammar both checkers read |
| `zrecord.sh` | the instrument from phase 86 on: six parts, 122 records |
| `enumvals.sh` | every enumerator's value from DWARF, before and after |
| `score.sh`, `residue.sh`, `phasename.sh` | reporting; `phasename.sh` reads a phase's name from the heading of its `GOAL.md`, and `residue.sh` reports phase patches, of which there are none here |
| `templates/whim.mk`, `templates/zero.mk` | the makefile phase 0 starts from, and the one phase 83 writes over it |
| `musl-case.txt`, `musl-ctype.txt`, `nolibm_check.c` | data and a probe a phase reads |
