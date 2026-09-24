# Phase 166 — a question returns bool

A core function declared `int` whose every `return` answers yes or no is
declared `bool`: its definition and every prototype, and nothing else. It
qualifies when each return is `TRUE`, `FALSE`, a comparison, `&&`, `||`, `!`,
a `?:` of those, or a call to another such function. The set is found to a
fixpoint, since one question often returns another's answer. A function whose
name is used as anything but a callee keeps `int`, because a table or a
pointer wants the type it has. So does one the host names, and so does `main`.
`MAYBE`, which is 2, disqualifies a function, because it is not a form the rule
accepts.

**It changes no value in C.** Every return is 0 or 1 already, and a `bool`
converts to exactly that wherever an `int` is wanted: `count += f()`,
`x = f()`, `f() == TRUE` and varargs all read the same numbers. What changes is
what the type says, and what the Go transpilation can then say: `internal/gen`
already turns a C `bool` into a Go `bool`. So `if f() != 0` becomes `if f()`,
`return B2i(a < b)` becomes `return a < b`, and `x != 0` stops being needed
where x holds an answer.

It is the survey's third recommendation (`doc/GO-IDIOMS.md`, item 3), and the
C phase it asked for: the generator follows the C's types, so the fix belongs
in the types.

**Measured:** 151 functions and 233 declarations. In `editor/editor.go`,
functions returning `bool` go from 7 to 158, `f(...) != 0` comparisons from 390
to 158, and `B2i` conversions from 359 to 292. vet, staticcheck and
`gofmt -s` stay at 0. `whim-test`: 45/45 as the commit before, and the Go
editor answers all 45 as the C does.

Not done here: the 65 functions that return `OK` or `FAIL`. Those are yes/no
too, but their callers say `== FAIL` and `!= OK`, and a `bool` would lose the
names. That is a phase of its own.
