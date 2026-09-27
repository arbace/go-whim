# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

1. **vijure's regex matcher, unboxed** (`doc/PARALLEL-SUBSTITUTE.md`): on a
   500,000-line `:%s` it is 22 times the C on `[ab]\+c` and 15 on
   `\v(a|b)+c`, mostly boxing (`Numbers.num`) in `regmatch`'s split groups; a
   matcher as fast as the Java's would gain about 13 times on one core -- what
   64 cores would give. Measure with the survey's workloads.
2. **The parallel `:%s`** (`doc/IR.md`, `doc/PARALLEL-SUBSTITUTE.md`): worth it
   only for regex-heavy patterns on large buffers (7.8-13 times at 16 cores;
   literal and dense patterns 1.3-4.3). First cases that cross chunk
   boundaries and one per condition that forces the sequential path (`c`,
   multi-line patterns, `\%#`, `\%V` ...); then the two C phases (the regex
   state a parameter: `rex` and kin, 40 functions; `ex_substitute`'s loop split,
   431 of its 805 lines, `match_lines` about 60) and the per-target bodies.

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


## Declined, with the reason recorded
