# Phase 160 — no line getter takes a cookie

`do_cmdline()` takes a function that gets the next line, and a `void *` cookie
it hands to that function. Vim's script and user-function readers used the
cookie; here every one of the six calls of `do_cmdline()` passes `nullptr`.
`do_one_cmd()` and `:append`'s reader only pass it on, and neither getter left,
`getexline()` or `getcmdkeycmd()`, reads it. So it was a parameter that was
always `nullptr` and read by nothing. It goes from the getter's type, from the
five functions that declare it, and from `exarg_T`. With it goes `find_func_t`,
a typedef nothing names.

What is left of `void *` in the core is memory, and
`internal/ccx`'s `VoidPtrs` partitions every declaration that names one:
- the functions of bytes;
- the allocators and `host_free()`;
- a growarray's `ga_data`.

`GrowArrays` shows that every growarray object has one element type. That
covers the globals, the locals, the fields of each struct type, the one
allocated, and every object a pointer can be given through a parameter, an
assignment, an initializer or a return. So a translation can give each
growarray's storage its type.

**Declared delta: nothing.** The check requires every cookie passed on the
input to have been `nullptr`. That nothing read it is the output compiling
without it. It requires `VoidPtrs` and `GrowArrays` to leave nothing. It probes
a command typed at `:`, a `<Cmd>` mapping, `:append` and `:@`; each control
moves.

**Already gone is a class of its own.** The cut of `find_func_t` is a partition, not a
count: the typedef is here and this phase removes it, or nothing at all names
it and the phase says so and cuts nothing (`edit.Ph.LiteralOrGone`). The
second class exists for the sweep's closure (`crefactor/sweep`,
the sweep itself since the six deleters went), whose closure takes a typedef nothing names in
an earlier sweep. Anything else refuses as it always did.
