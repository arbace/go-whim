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
| `tools/st.sh cemit` | one file in the canonical C23 form (`internal/cemit`), `--check` to ask whether it already is |
| `tools/st.sh parse` | the front end's smoke test, and the proof that the PATCHED `internal/cc` is what got linked |

| file | what it is |
| --- | --- |
| `st.sh`, `sweep.sh`, `canon.sh` | run `whimtools`: they build it if they must (`gobuild.sh`) and exec it. A phase program, a check, a makefile rule and a person at a prompt all reach the toolset the same way |
| `gobuild.sh` | builds `whimtools`, content-keyed on go.mod, go.sum and every .go under cmd/, internal/ and phase/ |
| `phasecheck.sh`, `phasebuild.sh`, `symbols.sh` | what a check runs: the sweep's silence, linkage, the libc surface, the build |
| `enumvals.sh` | every enumerator's value from DWARF, before and after |
| `score.sh` | bytes to store and symbols to provide, the input beside the product |
| `templates/whim.mk`, `templates/core.mk` | the makefile phase 0 starts from, and the one phase 83 writes over it |
| `musl-case.txt`, `musl-ctype.txt`, `nolibm_check.c` | data and a probe a phase reads |

Every one of them runs from the repository root and writes its temporaries in
`.tmp/`.
