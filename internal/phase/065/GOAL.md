# Phase 65 — the changedtick is a number

*Formerly phase 137. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

vim kept `b:changedtick` as a dictionary item inside `buf_T`, so a buffer's
variables could hold it without a copy: `CHANGEDTICK(buf)` expanded to
`((buf)->b_ct_di.di_tv.vval)`, the number inside a `typval_T` inside a
`dictitem16_T`. Whim has no buffer variables, and that one field was what
kept `typval_T` — and through it lists, dicts, `type_T` and `class_T` — in
`buf_T`. The field becomes `varnumber_T b_changedtick`, its 18 reads and
writes name it, and `init_changedtick()` sets it to 0 and no longer sets a
type, a lock and flags that nothing reads.

**Declared delta: nothing.** The check's partition: on the input every mention
of `b_ct_di` is the field, `init_changedtick()`'s cast or a use of the
number; on the output each use is the input's, rewritten in place by the
same rule (`edit.W137Tick`). Every insert, undo and search in the recording
reads the tick.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the member is retyped and renamed (RETYPE, RENAME), the body written by FRAG and the 18 expansions rewritten by form, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
