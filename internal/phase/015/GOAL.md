# Phase 15 — no `:noswapfile`

*Formerly phase 48. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

There has been no swap file since Phase 21: the memfile is memory. The modifier
set `CMOD_NOSWAPFILE`, whose two readers in `ml_open()` and `buf_copy_options()`
were already empty blocks. It is matched by name in `parse_command_modifiers()`
before the table, so its branch goes as well as its row, and so does its line in
the completion arm for modifiers.

## The delta

**The row**, which succeeded run bare. Measured: 115,588 → **115,568 lines**.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 44-48 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).

**On the graph** (B1a, `doc/GRAPH-MIGRATION.md`): `whim15a` and `whim15`
run on the program's graph -- the `!` row's handler replaced in place, the
`:noswapfile` case dropped with its run, the two readers folded, the
mentions counted on the C view and the enumerator's uses asked of its
edges -- their report the text versions', line for line; phase 14 hands
the phase the graph. Handed the graph it costs 0.30 s (main's text 2.4 s; `TestMeasureGraphPhases`, 5 runs, medians, beside main at a load of 20-30).
