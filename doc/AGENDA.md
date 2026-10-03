# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

Each moves method sizes: `whim test --java --clojure`'s heavy case, which
times every editor, is judged with the suites.

Nothing queued.

## Known stale, not yet scoped

- **The `SIZE_MAX` static_assert checks nothing** (found 2026-10-03 by
  B2e's include rule, doc/GRAPH-MIGRATION.md *B2e as built*; confirmed on
  whim-vim.c). Phase 43 defined the header limits as core enumerators
  (`enum : usize { SIZE_MAX = (usize)-1 };`) and asserted them against the
  headers' macros below the line; phase 88 (old 169) dropped `<stdint.h>`,
  since the file still compiles without it, so `static_assert((usize)-1 ==
  SIZE_MAX, "SIZE_MAX")` now compares the core's enumerator with its own
  definition. `EXIT_FAILURE`'s assert was the same from phase 88 until phase
  99 brought `<stdlib.h>` back. A fix keeps `<stdint.h>` (phase 88 judging a
  header needed when an assert's name would bind to the core without it --
  the graph's `Collisions`/`Missing` say exactly that); the product gains
  one include line.


## Declined, with the reason recorded

- **The generic steps moved to the end, and `BoolRet` run once** (2026-10-02;
  the pipeline reform's G, `PIPELINE-REFORM.md` §7). Each move was run from
  the step's position to the product.
  - Phase 50's unions: phase 65 names the field as 50 leaves it.
  - Phase 62's empty blocks: phase 71 takes the block 62 leaves.
  - Phase 74's never-null folds: the product is 34 lines longer.
  - `BoolRet` once: 120 lines differ.
  - The attributes (part 0c), not tried at the end: they must precede 50, 62
    and 74, which cannot move. They moved the other way, to the seed, after
    `NullptrUsize` (part 0a) (§7, G), and the variadic collapse (part 0b)
    with them: 282 lines of the programs, cutters and vendored C that ran
    before them respelled for 0a, 14 for 0c, 6 lines and 4 counts for 0b,
    the chain byte for byte.

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
  parallel body of `match_lines` (phase 96) was written five ways and
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
