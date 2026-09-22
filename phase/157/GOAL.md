# Phase 157 — get_register() and put_register() carry a yankreg_T *, not a void *

`get_register()` returns a copy of a yank register as `void *`, and
`put_register()` takes it back and casts it to `yankreg_T *`. It was never
anything else. The register is typed `yankreg_T *` from one to the other, with
the callers' locals, and the cast goes.

That was the last pointer cast in the core outside the classes `internal/ccx`'s
`Casts` names: allocations, growarray data, the byte functions, bytes, nulls
and conversions to `void *`. Each has a rule a translation can apply without
reading the code around it.

**Declared delta: nothing.** The check requires the typed prototypes, no cast
outside a class, and the binary byte-identical. Its probe is a Visual-mode put,
which saves and restores a register, and the control moves.
