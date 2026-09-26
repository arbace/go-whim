# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

1. **vijure: the printer's noise** (`CLOJURE-IDIOMS.md`). clj-kondo: 2,993
   warnings; 11,240 `_` bindings, `if ... nil` for `when`, and kin. The
   generator splits functions by text length, so its guess is
   recalibrated with it (`load()` is at 54,382 of 65,535 bytes).
2. **vijure: the four nesting rules.** Better lowering makes 154 of 334
   state machines structured `let`/`loop` without duplicating code; the
   first rule (joins) was built by the survey and passes both suites.
3. **vijure: kebab-case names and `?` predicates**, then address-taken
   members and constants as shared data, in the survey's order.

## Known stale, not yet scoped


## Declined, with the reason recorded
