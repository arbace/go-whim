# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

1. **vijure: kebab-case names and `?` predicates**, then address-taken
   members and constants as shared data, in the survey's order.

## Known stale, not yet scoped

- **The `insert` case is timed for the editors on the JVM.** Its keys end in
  an undo, and vim's message says how long ago the change was: "0 seconds
  ago" -- "1 second ago" when an editor takes more than a second between
  the two. Under a load of 20-30 on this machine, 1-2 of 48 simultaneous
  runs of `bin/vijure` say so (measured on HEAD's build and on the build
  after the nesting rules alike), and `whim test --clojure` failed once on
  it (2026-09-27). The C and the Go are far from the second. A fix belongs
  in the case (no undo message) or the host (a clock the suite pins).


## Declined, with the reason recorded
