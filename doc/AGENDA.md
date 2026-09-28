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

- **core.async for the Clojure editor's parallel `:%s`** (2026-09-28). The
  parallel body of `match_lines` (phase 177) was written five ways and
  measured, each built in place with its start-up cache, all answering the
  quick suite's 73 cases as the C: braaam's `Rt/chunks` (the Java's
  fork-join pool, through a `reify`: what is committed), Clojure `future`s,
  and core.async 1.8.741's `thread` per chunk, `pipeline-blocking` and
  `pipeline` (its compute pool at the default size and at 64), the chunks
  the same in each (about four a CPU, 64 lines at least). A `:%s` at
  500,000 lines, seconds (median of three, the buffer's build taken off),
  64 cores:

  | body | start-up | lit | dense | bt | cls | sel |
  | --- | --- | --- | --- | --- | --- | --- |
  | `Rt/chunks` | 0.34 | 4.03 | 7.06 | 4.32 | 2.32 | .44 |
  | `future` | 0.34 | 4.14 | 7.37 | 4.41 | 2.36 | .42 |
  | `a/thread` | 0.51 | 4.01 | 7.32 | 4.37 | 2.36 | .44 |
  | `a/pipeline-blocking` | 0.49 | 4.00 | 7.32 | 4.96 | 2.34 | .38 |
  | `a/pipeline` | 0.51 | 4.16 | 6.98 | 4.73 | 2.38 | .45 |
  | `a/pipeline`, pool 64 | 0.51 | 3.96 | 7.21 | 4.86 | 2.45 | .50 |

  The work is the same within the noise: 256 chunks of milliseconds each
  keep 64 cores busy whatever hands them out, and the rest of the time is
  the loop's, on one thread (one CPU: `bt` 64 s, `cls` 21.6 s, against 4.6
  and 2.4). What core.async costs is everything else: requiring
  `clojure.core.async` loads its analyzer (`tools.analyzer.jvm`,
  `core.cache`), so start-up goes 0.34 -> 0.50 s, the jar 9.5 -> 12.3 MB,
  the start-up cache 43 -> 51 MB; and the build, which compiles with
  unchecked, warn-on-boxed math and refuses a warning, must compile the
  library first with Clojure's defaults (compiled with ours it warns, and
  its arithmetic would change). Channels and back-pressure buy nothing for
  a fork-join over independent chunks. Kept: `Rt/chunks`, one runtime for
  the Java and the Clojure; `future`s are the dependency-free idiom if the
  Clojure should not lean on braaam's runtime, at no measured cost.
