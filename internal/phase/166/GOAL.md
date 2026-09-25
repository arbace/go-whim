# Phase 166 — a question returns bool

A core function declared `int` whose every `return` answers yes or no is
declared `bool`: its definition and every prototype. It qualifies when each
return is:
- `TRUE`, `FALSE`, `OK` or `FAIL`;
- a comparison, `&&`, `||`, `!`, or a `?:` of those;
- a call to another such function;
- a local that only ever holds one of those.

The set is found to a fixpoint, since one question often returns another's
answer. A function whose name is used as anything but a callee keeps `int`,
because a table or a pointer wants the type it has; so does one the host
names, and so does `main`. `MAYBE`, which is 2, disqualifies a function: the
rule does not accept it as an answer.

**OK is success, and it is `true`.** `OK` is 1 and `FAIL` 0 -- the values of
`true` and `false` -- so a function that returns `OK` or `FAIL` returns a
`bool` whose `true` means it worked, which is Go's own idiom. Its callers
follow. A comparison of an answer with `OK`, `FAIL`, `TRUE` or `FALSE` becomes
the answer itself (`f() == OK`, `f() != FAIL`) or its negation (`f() == FAIL`,
`f() != OK`), so `if (ga_grow(gap, 1) == FAIL)` reads `if (!ga_grow(gap, 1))`.
A local declared `int` alone, with one declarator, that is only ever given an
answer (no `&`, no `++`, no compound assignment) is `bool` too.

**It changes no value.** Every return and every such local holds 0 or 1
already, and a `bool` converts to exactly that wherever an `int` is wanted.
The code does change, so the binary differs; the evidence is `whim-test`.
The Go follows the C's types: `internal/gen` turns a C `bool` into a Go `bool`,
so `if f() != 0` becomes `if f()` and `retval = B2i(f())` becomes
`retval = f()`. The generator learned two things for it: `ga_grow_inner`'s
fixed Go body takes its result type from the C, and a `bool` used as an offset
converts with `B2i`.

It is the survey's third recommendation (`doc/GO-IDIOMS.md`, item 3): the C
phase the survey asked for, since the generator follows the C's types.

**Measured:** 278 functions and 327 locals, 774 declarations retyped, and
188 comparisons rewritten. In `editor/editor.go`:
- functions returning `bool`: 7 -> 285;
- `f(...) != 0`: 390 -> 152;
- `B2i`: 359 -> 242.

vet, staticcheck and `gofmt -s` stay at 0. `whim-test`: 45/45 as the commit
before, and the Go editor answers all 45 as the C does.

Not done: struct fields that hold an answer (`typebuf_valid`), and locals the
rule refuses because they are also updated with `|=` (`area_highlighting`).
They are `int` still, so storing an answer in one needs `B2i` in the Go.
