# tools/

The shell half of the two pipelines: the memoize driver, and the three wrappers
through which every phase runs Go. Everything else is `cmd/whimtools` and
`internal/`.

| file | what it is |
| --- | --- |
| `memo.sh` | one unit of a pipeline: tier 3 (the recorded result), else tier 2 (the program) |
| `phaserun.sh` | one stage: the symbol snapshot, every edit in order, one sweep, every check, the delta |
| `stages.sh`, `packages.sh` | read and check `pipes/<p>.stages` |
| `implhash.sh` | the implementation half of a key: a phase's programs and every path they name |
| `snapshot.sh`, `restore.sh` | a boundary: a tar and a content digest of a tree |
| `oracle.sh` | compare a boundary with the recorded one in `.reference/<p>-phases/` |
| `verifypass.sh` | every unit at once on the recorded boundary before it (`make <p>-verify`) |
| `specpass.sh` | every unit at once on the previous pass's boundaries, into the cache (`make <p>-specpass`) |
| `pipeline.sh` | the only place the two pipelines differ: tag, work directory, source, delta checker |
| `st.sh`, `sweep.sh`, `canon.sh` | the wrappers that run `whimtools`; they name `cmd/`, `internal/`, `go.mod` and `go.sum` so every Go file is in every key that runs Go |
| `gobuild.sh` | builds `whimtools`, content-keyed, with the patched `modernc.org/cc/v4` |
| `phasecheck.sh`, `phasebuild.sh`, `symbols.sh` | what every check runs: the sweep's silence, linkage, the libc surface, the build |
| `whimdelta.sh`, `zerodelta.sh` | a stage's declared delta against `.reference/baselines` and `.reference/zero-baselines` |
| `zrecord.sh` | zero's instrument: six parts, 122 records |
| `enumvals.sh` | every enumerator's value from DWARF, before and after |
| `score.sh`, `residue.sh`, `phasename.sh` | reporting |
| `musl-case.txt`, `musl-ctype.txt`, `nolibm_check.c` | data and a probe a phase reads |
