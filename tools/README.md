# tools/

What a check or a delta runs, and the three wrappers that find the Go binary.
**The driver is gone**: the pipeline is `internal/build` (the plan and the
phases) and `internal/verify` (the same plan with every check and delta), both
reached as `tools/st.sh build` and `tools/st.sh verify`, and there is no
memoize, no boundary and no oracle between them.

| file | what it is |
| --- | --- |
| `st.sh`, `sweep.sh`, `canon.sh` | run `whimtools`: they build it if they must (`gobuild.sh`) and exec it. A phase program, a check, a makefile rule and a person at a prompt all reach the toolset the same way |
| `gobuild.sh` | builds `whimtools`, content-keyed, with the patched `modernc.org/cc/v4` (`patches/cc-v4-c23.patch`) |
| `whimdelta.sh`, `coredelta.sh` | the declared delta at a phase, read from the `phase/NNN/delta` files through `declared.sh`: against `.reference/baselines` for phases 0-82, and from `CORE_FROM` (83) on `whimdelta.sh` hands the phase to `coredelta.sh`, against `.reference/core-baselines` |
| `declared.sh` | the declarations of a run of phases, in the one grammar both checkers read |
| `zrecord.sh` | the instrument from phase 86 on: six parts, 122 records |
| `phasecheck.sh`, `phasebuild.sh`, `symbols.sh` | what a check runs: the sweep's silence, linkage, the libc surface, the build |
| `enumvals.sh` | every enumerator's value from DWARF, before and after |
| `score.sh` | bytes to store and symbols to provide, the input beside the product |
| `templates/whim.mk`, `templates/core.mk` | the makefile phase 0 starts from, and the one phase 83 writes over it |
| `musl-case.txt`, `musl-ctype.txt`, `nolibm_check.c` | data and a probe a phase reads |

Every one of them runs from the repository root and writes its temporaries in
`.tmp/`.
