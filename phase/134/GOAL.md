# Phase 134 — the empty blocks fold

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
