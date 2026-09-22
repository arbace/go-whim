# Phase 131 — the saved input buffer is a `garray_T *`

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
