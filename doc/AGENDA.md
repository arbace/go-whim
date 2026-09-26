# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

1. **braaam: constants and case labels by name** (`JAVA-IDIOMS.md`). 961 of
   961 `case` labels are numbers and about 9,000 other constants are
   folded: the printer goes through the constant's value
   (`java_stmt.go`, `java_expr.go`). Print the enumerator or macro's name.
   Spelling only: the classes stay byte-identical under `javac -g:none`,
   which is the check (`--same-classes`, to be written with it).
2. **braaam: parentheses by precedence, masks where needed.** 11,127
   redundant parentheses; 1,200 `& 0xff` where the byte is only compared.
   Same check as 1 for the parentheses.
3. **braaam: locals where the C declares them.** 3,449 are hoisted to the
   top of the method with a throwaway zero.
4. **A C phase for `pos_T`'s address-taken members** (both surveys). A
   vim macro takes `&pos.lnum` / `&pos.col` (`one_adjust` in
   `mark_adjust_internal`, `cursor_pos_info`), so both fields are
   one-element arrays in Java and Clojure: 2,124 of braaam's 6,509 `[0]`
   reads. Phase 174: write those uses without the address.
5. **vijure: the printer's noise** (`CLOJURE-IDIOMS.md`). clj-kondo: 2,993
   warnings; 11,240 `_` bindings, `if ... nil` for `when`, and kin. The
   generator splits functions by text length, so its guess is
   recalibrated with it (`load()` is at 54,382 of 65,535 bytes).
6. **vijure: the four nesting rules.** Better lowering makes 154 of 334
   state machines structured `let`/`loop` without duplicating code; the
   first rule (joins) was built by the survey and passes both suites.
7. **vijure: kebab-case names and `?` predicates**, then address-taken
   members and constants as shared data, in the survey's order.

## Known stale, not yet scoped


## Declined, with the reason recorded
