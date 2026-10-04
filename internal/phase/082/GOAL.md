# Phase 82 — a struct's text is a pointer to an allocation of its own

*Formerly phase 159. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

Three structs ended in a one-element array that was really as long as its
allocation: `buffblock_T`'s `b_str`, `msgchunk_T`'s `sb_text` and `regprog_T`'s
`program`. Each was allocated as `offsetof(T, x) + n` bytes at once. The
array's length was not its type's, which a translation into a language whose
arrays know their length cannot write.

Each is now a `char_u *`, and the struct and its bytes are two allocations.
That changes nothing, because the rest of the core never depended on the array
being inside:
- a `msgchunk_T` or a `regprog_T` is never copied, sized or declared;
- a `buffblock_T` is declared only as the head of each of the five buffer
  lists.

A head's one byte is written only with NUL, by `delete_buff_tail()` of nothing
while the head is current, and it is never read. Each head now points at a
byte of its own, a compound literal with static storage.

**Declared delta: nothing.** The check partitions every mention of the three
types on the input into the typedef, pointers, the one allocation and the
heads. It requires the same partition on the output, with `sizeof(T)` where the
`offsetof()` was. It requires that a head becomes current only with
`bh_create_newblock` set, and that only a new block, made current, clears it.
It probes a repeated change, a recorded register and an empty one, a long
message redisplayed by `g<`, and a pattern. Each control moves.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the members are retyped by RETYPE and the allocations and heads written by FRAG in one unit, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
