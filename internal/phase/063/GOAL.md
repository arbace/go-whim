# Phase 63 — one regexp program type

*Formerly phase 135. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

vim had two regexp engines, and `regprog_T` was the header both programs
began with: `bt_regprog_T` repeated its five fields and added its own, and
the code cast between the two. Whim kept one engine, so every `regprog_T` is
a `bt_regprog_T` and every cast is to itself; the Go transpilation, which
cannot cast a struct to the larger one it heads, kept a registry
(finding 4). `regprog_T` takes the backtracking fields, the casts go, and
`bt_regprog_T` is not a name any more.

**Declared delta: nothing**, and more: every field keeps its offset, so the
check requires the input and the output, built with the boundary's flags and
`SOURCE_DATE_EPOCH=0`, to be **the same bytes**.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the casts are rewritten by form and the two typedefs written as one by FRAG, which resolves every use of the struct again, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
