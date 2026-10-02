# Phase 45 — no `:drop`

`:drop` edited a file by making it the argument list and going to its first
entry: with one window and one buffer it was `:args` plus `:first`, both gone.
`ex_drop()` was the last caller of `set_arglist()` and `ex_rewind()`, and the
sweep takes all three.

## The delta

**None.** `:drop` already failed with no argument. Measured: 115,798 → **115,744
lines**.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 48, as the first of the group 44-48, which was one idea split for history's sake. There is no boundary q045 of its own any more; everything above still says what the steps do and why.
