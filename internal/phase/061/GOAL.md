# Phase 61 — one buffer needs no hash table

*Formerly phase 133. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

Whim has one buffer: `buflist_new()` is called once, at startup, and `curbuf`
is that buffer or NULL. So `buf_hashtab` held one entry, and
`buflist_findnr()` — its one reader, called only by `setmark_pos()` — found
the buffer by subtracting the key's offset from the key inside it, the
`container_of` the Go transpilation had to keep an owner registry for
(finding 3). `buflist_findnr()` becomes "the current buffer, if its number is
`nr`"; the table's init, add and remove go, and the sweep takes the two
helpers, the table and `b_key`.

**Declared delta: nothing.** The check proves from the input, as partitions
of assignments, that `curbuf` is only ever `nullptr`, `curwin->w_buffer` or
`buflist_new()`'s result and `w_buffer` only `nullptr` or `curbuf`, so the
current buffer is the one buffer whenever a mark can be set. Its probe sets
and jumps to the marks that go through `buflist_findnr()` (`m[`, `m]`,
`m"`); the control jumps to a mark never set.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the body by FRAG, the three statements cut by their forms, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
