# Phase 137 — the changedtick is a number

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
