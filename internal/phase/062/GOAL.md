# Phase 62 — the empty blocks fold

*Formerly phase 134. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

Phase 132 left 33 blocks with nothing in them: `if (allocated) { vim_free(p); }`
became `if (allocated) { }`; with those earlier phases left, 49 fold. An empty block guarded by a condition that only
reads (no call, no assignment) does nothing, and goes; so does an empty
`else`, and an empty `else if` that ends its chain. Loops are kept. A flag
that only such a condition read is then only given values, and goes with its
stores (`edit.DeadStores`, phase 132's): `did_intro`,
`event_cmdlineleavepre_triggered`, `mustfree` and `free_str`. The two rules
alternate until neither changes anything (`edit.W134Rule`).

**Declared delta: nothing.** The check **computes the whole output**: the
input's core with `edit.W134Rule` applied, run through the real sweep, must
be the output byte for byte. It names what the sweep took beyond the blocks,
and requires every empty block left to be one the rule must keep.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3g) the phase runs on the graph: `crefactor/graph`'s `Editor.EmptyBlocks` (B2d's) on the core's forms, with `edit.PureCond` as the text's own test of a condition, its report the text version's; `whim-build-check` holds q062 to the bytes `crefactor/xform`'s `EmptyBlocks` made (which is in history, `16717ab` and before).
