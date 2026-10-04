# Phase 79 — the regex size pass's node is a static byte, not (char_u *) -1

*Formerly phase 156. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

The backtracking regex compiler runs twice: once to measure the program and
once to write it. During the first pass `regcode` is `(char_u *) -1`, and every
node-making function tests for it and writes nothing. That is an integer made a
pointer: fourteen places, each a comparison or the one assignment in
`bt_regcomp()`, none a dereference. A translation into a language with no
integer-to-pointer conversion has no value to give it.

It becomes the address of a static byte, `reg_calc_size_node`, which is never
read or written; only its address is compared, as the sentinel's was.

**Declared delta: nothing.** The check requires every use of the sentinel,
before and after, to be a comparison or that assignment. It requires no
integer cast to a pointer left anywhere in the core (`internal/ccx`'s `Casts`).
Its probes substitute with a branch, counted and lazy repeats, a look-behind, a
collection and a back-reference on both binaries, and each control moves.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the byte and its fourteen uses are written by FRAG in one unit, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
