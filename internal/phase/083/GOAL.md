# Phase 83 — the core's compile line, and the baselines it is measured against

**Since the merge this phase is not a seed.** It is handed q82's tree — `whim-vim.c`
and the makefile whim's phases carry, `tools/templates/whim.mk` — and there is no
committed file for it to `cmp` with. So step 1 below is now *build the tree it was
handed with the compile line it carries*: that binary is the one the core baselines
are recorded from (step 3), and the phase then writes `tools/templates/core.mk` over
the makefile and builds again (step 2). The baselines are therefore recorded from the
pipeline's own output at q82 — the one place it does so, and legitimate for the
reason recording from an input is: nothing from 83 on can reach q82. What it costs is
that a change to what phases 0-82 produce moves the recording, and this phase refuses
rather than overwrite it. `internal/phase/083/check.go` carries the argument. What follows is the
phase as it was written.

`whim-vim.c` starts as a byte-for-byte copy of the committed `whim-vim.c`, and the
phase is `internal/phase/083/check.go`, one whole program. Four things, each depending on the one
before:

1. **The seed is the input.** `cmp` against `whim-vim.c`; the boundary digest is the
   same file's.
2. **It builds, absolutely static.** `make -C zero` with `tools/templates/core.mk`,
   `gcc -O0 -static -no-pie -s`, and then `readelf`: the type is `EXEC`, there is no
   `INTERP`, no dynamic section and no relocation. Measured on this input: 894,088
   bytes, where whim's static-PIE of the same source is 955,976 bytes, `DYN`, with a
   dynamic section and 1,986 relative relocations.
3. **The core baselines are whim-vim's.** `whim-vim.c` is built in a scratch
   directory with `tools/templates/whim.mk` — whim's compile line — and the three
   harnesses whim's delta reads, `behaviour.py`, `exsweep.py` and `termcheck.py`,
   record it three times; the runs must be identical. If `.reference/core-baselines`
   exists the recording must equal it, and it is never overwritten: a difference
   means a harness or the input changed, and has to be named. If it does not exist,
   it is written.
4. **whim-vim does exactly what whim-vim does.** `tools/coredelta.sh zero/whim-vim
   zero/whim-vim.c --phase 83`, with `internal/phase/083/delta.md` empty, requires no behaviour
   case, no Ex command and not the terminal table to move — so `-no-pie` changed
   nothing a harness sees. And `tools/whimdelta.sh --phase 82` on the same binary,
   against slim-vim's baselines, shows whim's whole declared delta still holds: the
   frozen whim behaviour is intact under the new compile line.

A tier-3 hit on this phase records nothing, because the phase does not run. So
`whim.mk` checks afterwards: `whim-pass`, and `zero-phase-N` — and through them
`whim-repass`, `whim-specpass`, `whim-tip` and the `whim-vim.c` rule — run
`whim-baselines-check` once the chain has reached its boundary, and it refuses unless
`.reference/core-baselines` holds a non-empty `behaviour/`, `ref-exsweep.txt` and
`ref-term.txt`, naming the fix: `rm -rf .cache/r0 && make whim-phase-83`. The check
lives in `whim.mk`, which no implementation digest reads, so it moves no key.

## The one compile line

This phase changed the compile line, and that change is now the line for every
boundary, the input and the product: `gcc -O0 -fno-stack-protector -static
-no-pie -s` (`internal/build/compile.go`). So the phase changes nothing today;
it is kept in the plan, empty, so the numbering does not move.
