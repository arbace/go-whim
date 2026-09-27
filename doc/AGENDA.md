# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

1. **The Go's plain `char` is unsigned; the C's and the Java's are signed**
   (`doc/IR-SCHEMA.md`, step 0). gcc on x86-64 makes `char` signed, and the
   Java and Clojure backends follow it; the Go printer writes `byte`. A probe:
   `char x = *s; return x < 0;` is `int32(x) < 0` in the Go -- never true --
   and `(int) x < 0` in the Java. Editor.java has 44 sign-extending reads in 16
   functions; the ones the survey read are equality or ASCII comparisons,
   where the two agree, and no suite sees a difference -- so a case must be
   found or written that would (a byte of 128 or more where a plain char is
   compared or widened), then the Go printer made to agree (about 20 lines),
   the suites and whim-editor-check the proof.

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
