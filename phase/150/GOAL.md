# Phase 150 — the regexp stack is three typed stacks

The backtracking engine's stack, `regstack`, was one growarray of bytes:
- the `regitem_T` record of each state;
- below the record of a star or a look-behind state, the `regstar_T` or
  `regbehind_T` it carries. That data was pushed first, found again as
  `((regstar_T *)rp) - 1`, and popped with `ga_len -= sizeof(...)`.

`'maxmempattern'` limits the array's bytes (E363). Go can't overlay structs on
bytes, so the transpilation kept the objects in a side table and the byte
accounting in x86-64 sizes (findings 5 and 8).

Now the records, the stars and the look-behinds are three growarrays of their
own types. The extra data is still pushed just before its record and popped
just after it, so the three stay in step, and two accessors return the top of
the star and look-behind stacks. `regstack_bytes` adds and subtracts exactly
the sizes the byte array did, so E363 comes at the same moment. The `sizeof`s
that remain are those three, where the byte count is kept.

**Declared delta: nothing.** The check requires the output's nine byte
adjustments to be the input's nine, each beside a push or pop of its own
stack, and nothing to read the stack as bytes. Its probes are a star, a lazy
count, a look-behind, a negative look-behind and a complex star, each against
a control. Then it finds by bisection, on the input's binary, the exact
`'maxmempattern'` at which a complex star over 600 characters stops failing
with E363. The output's binary must fail at one less and succeed at that
value, drawing the same bytes at both.
