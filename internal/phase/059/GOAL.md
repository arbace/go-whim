# Phase 59 — the saved input buffer is a `garray_T *`

*Formerly phase 131. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

`save_typeahead()` keeps what `inbuf[]` held in `tasave_T.save_inputbuf`,
made by `get_input_buf()`: a `garray_T`, allocated and filled — then cast to
`char_u *` to be stored, and cast back by `set_input_buf()`. Nothing read it
as characters. The Go transpilation could not carry a growarray in a string
pointer and registered it under a one-byte key (finding 6). The field, both
prototypes, the definition and the return now say `garray_T *`, and
`set_input_buf()` takes the growarray it always cast its argument to.

**Declared delta: nothing.** The check's partition: every line naming the
saved input buffer is one of seven, each saying `garray_T` where the input
said `char_u`; the two casts are gone and no other appeared; and the silent
compile is itself the proof that nothing passed a string where the growarray
goes.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the member, the result and the parameter are retyped (RETYPE), and the parameter takes the local's uses and name (RENAME), in its prototype too, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
