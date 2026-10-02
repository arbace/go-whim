# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

- **The drops that stayed, brought to the front** (the pipeline reform,
  `PIPELINE-REFORM.md` §7).
  - `onebuffer` (42), `lfonly` (50), `keepbytes` and `noconv` (53), the
    programs of 57-59, 63-68, 71-76 and 85.
  - Each counts text that only the phases before it leave. So each needs
    the edits it depends on brought to the front with it: for `lfonly`,
    every readfile edit, since readfile is live at the front until phase 13.
- **A fresh numbering of the phases**, beyond the block labels: some 4,200
  citations, the snapshots and `--from`/`--to` name the numbers (§7, step
  8).

## Known stale, not yet scoped

Nothing known.


## Declined, with the reason recorded

- **The generic steps moved to the end, and `BoolRet` run once** (2026-10-02;
  the pipeline reform's G, `PIPELINE-REFORM.md` §7). Each move was run from
  the step's position to the product.
  - 120's unions: phase 137 names the field as 120 leaves it.
  - 134's empty blocks: phase 145 takes the block 134 leaves.
  - 149's never-null folds: the product is 34 lines longer.
  - `BoolRet` once: 120 lines differ.
  - `NullptrUsize` and the attributes (106-107): they would need about 280
    anchors in 34 phases respelled, and must precede 120, 134 and 149,
    which cannot move.

  Declined: the rewires are written for these steps' text.

- **The messages as data the Clojure editors share** (2026-09-29;
  `CLOJURE-IDIOMS.md` item 4). A sound proof that nothing writes vim's 185
  message arrays, not even through a pointer, reaches 16-19 of them.
  Everything `emsg` prints goes through `:filter`'s regex engine, which
  keeps its subject in memory and saves it through a `char_u **`. Excusing
  that engine alone would give 177. Proving it needs a field- and
  type-precise heap model, which no suite could check, to share byte arrays.
  Declined.
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
