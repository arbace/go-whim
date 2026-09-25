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
answer (no `&`, no `++`, no compound assignment) is `bool` too. So is a
struct member of that kind -- by name, so every member so named, and never
one a designator names or a brace initializer fills by position, since those
give values no assignment shows (`internal/sweep`'s `PositionalMembers` says
which) -- and a parameter every call gives an answer, of a function called
only by name from the core. A comparison with 0 becomes the answer or its
negation too. Anything compared with a code, a constant other than 0, 1,
`TRUE`, `FALSE`, `OK` or `FAIL` (`redraw != -1`, `flags == ATC_FROM_TERM`),
stays `int`: what it holds is a code that happens to be 0 or 1, and as a
`bool` gcc would rightly call the comparison constant.

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

**Measured:** 279 functions, 331 locals, 26 members and 279 parameters, 1,276
declarations retyped, and 189 comparisons rewritten. In `editor/editor.go`:
- functions returning `bool`: 7 -> 285;
- `f(...) != 0`: 390 -> 142;
- `B2i`: 359 -> 179.

vet, staticcheck and `gofmt -s` stay at 0. `whim-test`: 45/45 as the commit
before, and the Go editor answers all 45 as the C does.

Not done: what the rule refuses by design keeps its `B2i` -- a variable also
updated with `|=` (`area_highlighting`), a member some table fills by position,
a parameter of a function taken by address.
