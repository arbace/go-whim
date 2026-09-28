# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

1. **vijure's `regmatch` structured, in methods C1 compiles.** The nesting
   structures it now (2026-09-28: its loops' tails, a region's `break`s
   and `continue`s, tuples in the frame), but its main loop, in expression
   position, is a closure Clojure writes of 26,974 bytes, which the
   launchers' `-XX:TieredStopAtLevel=1` leaves interpreted ("out of virtual
   registers"); so a function past the split bound is not structured, and
   `regmatch` stays split. Measured on a 100,000-line `:%s`, substitution
   only: structured 7.5 s (`[ab]\+c`), 26.3 s (`\v(a|b)+c`), 0.76 s
   (literal) under C1; 2.6, 4.9 and 1.1 s with every tier (C2); split
   3.1, 9.8 and 0.60 s; the Java 0.66, 1.8 and 0.18. What would take it:
   the large regions written as functions of their own (the variables in
   the frame, as the split's groups have them), each small enough for C1.
2. **`:g` on `match_lines`.** `ex_global`'s marking pass is a second loop
   that matches every line of a range on its own, and phases 176-177 made
   that a primitive with a parallel body; `:g/pat/cmd` over a large buffer
   would share the gain `:%s` has (`doc/PARALLEL-SUBSTITUTE.md`). Not
   measured.

## Known stale, not yet scoped

- **The `insert` case is timed for the editors on the JVM.** Its keys end in
  an undo, and vim's message says how long ago the change was: "0 seconds
  ago" -- "1 second ago" when an editor takes more than a second between
  the two. Under a load of 20-30 on this machine, 1-2 of 48 simultaneous
  runs of `bin/vijure` say so (measured on HEAD's build and on the build
  after the nesting rules alike), and `whim test --clojure` failed once on
  it (2026-09-27). The C and the Go are far from the second. A fix belongs
  in the case (no undo message) or the host (a clock the suite pins).
  The wide suite's `keys` group failed once each for the Java and the
  Clojure editors the same day, under a load of 60 or more from another
  project, and not in eight reruns: the same kind, not identified.
  `undo_redo` is the same (its keys end in undo and redo, "0 seconds
  ago"): the Java editor's quick suite and the Clojure editor's wide one
  each failed on it once on 2026-09-28, beside a pipeline check, and not
  in 32 runs 16 at a time.


## Declined, with the reason recorded
