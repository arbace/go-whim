# Where the Clojure editor's time goes: the heavy case, profiled

2026-09-29, at `6777dc1`. The heavy case (`internal/suite/heavy.go`: 5,000
lines, three `:s` and a `:g` with `:normal`) takes the Clojure editor 4.4
times the C's time. This is where that time is, measured, so that the next
change to the Clojure editor is chosen from it.

## How it was measured

- **The Clojure editor**: `bin/vijure`'s JVM and flags (C1 only,
  `-XX:-DontCompileHugeMethods`, the AOT cache), the heavy keys from a file on
  stdin, Java Flight Recorder's `profile` settings (`jdk.ExecutionSample`
  every 10 ms); eight runs, 1,292 samples. Each sample is grouped by the C
  function it is in: the generated function, its outlined pieces (`__rN`),
  its split groups (`__N`) and its closures (`$fn__N`) folded together. A
  frame's `type` says whether it ran interpreted or compiled.
- **The C**: `whim-vim.c` built as `bin/whim-vim` is (`-O0`) but with `-g`,
  under `perf record -F 4000`, eight runs.
- **A stale build misleads.** The first profile, of a `bin/vijure` built
  before phase 181, showed `utfc_ptr2len` building a lazy sequence and an
  `int-array` on every call (an out-parameter boxed in a one-element array,
  which `Numbers.int_array` fills through `RT.seq`): 8.7% of the samples.
  Phase 181 had already removed it; `make bin/vijure` first, always.

## What was found

Wall time, median of three: the C 0.44 s; the Clojure editor 1.93 s. The
Clojure's samples: 79% in the generated code, 8% in Clojure's runtime (the
boxing of `Numbers.num` and `Long.valueOf` 2.4%, lazy sequences under 1%),
9% the JDK, 4% `whim.rt`. A tenth of the run (about 0.2 s) has no editor
frame at all: starting up.

Self time by C function, in milliseconds a run, the Clojure's beside the
C's:

| function | C | Clojure | times |
| --- | ---: | ---: | ---: |
| `ex_substitute` | 1.7 | 198 | 115 |
| `win_line` | 0.1 | 82 | ~800 |
| `match_chunk` | 3.3 | 78 | 23 |
| `vim_regsub_multi` | 0.6 | 16 | 26 |
| `bt_regexec_both` | 2.8 | 38 | 13 |
| `musl_strncpy` | 3.3 | 44 | 13 |
| `regmatch` | 36.6 | 116 | 3.2 |
| `win_linetabsize_cts` | 23.8 | 107 | 4.5 |
| `win_nolbr_chartabsize` | 27.9 | 78 | 2.8 |
| `ptr2cells` | 13.5 | 99 | 7.3 |
| `musl_memset` | 215 | 0 | -- |

Two kinds of cost:

1. **Big functions run interpreted.** `ex_substitute` ran interpreted in
   119 of its 125 samples, `match_chunk` in 46 of 48, `win_line` in 36 of
   53, `bt_regexec_both` 16 of 22, `regmatch` 40 of 75. These are called a
   handful of times -- three `:s`, a chunk each -- and loop for thousands of
   lines inside. HotSpot compiles a method after 200 calls or, mid-loop,
   after 60,000 back edges (`Tier3BackEdgeThreshold`, with C1 only), so a
   5,000-line loop never reaches it; and a state machine the backend writes
   as one method (`ex_substitute`: 270 kB of C1's code when it is compiled)
   costs 70 ms or more to compile when it is. This is the 115 times.
2. **The rest is spread thin**: the per-character work of the redraw's line
   counting (`plines_win_nofold` 22% of the run on the stack,
   `win_linetabsize_cts`, `win_lbr_chartabsize`, `ptr2cells`) and the regex
   engine, 3-7 times the C's -- the JVM's C1 against gcc's `-O0` on the same
   loops, byte arrays behind `BytePtr` and boxing in places. No single
   hot spot.

The C, for its part, spends 42% of its run in its own `musl_memset` (a byte
loop at `-O0`, `cleanup_subexpr` zeroing the regex engine's arrays), which
the JVM editors do with `Arrays.fill`.

## What was tried

Each built from the same core, the heavy case timed three times and a short
session (`ihello world<Esc>:q!`) three times; every variant printed the
same screens:

| variant | heavy (s) | short (s) |
| --- | ---: | ---: |
| as built (split at 110,000, C1) | 1.92-1.97 | 0.34-0.36 |
| `-XX:CompileThresholdScaling=0.1` | 1.75-1.81 | 0.40-0.42 |
| `-XX:Tier3BackEdgeThreshold=6000` | 1.78-1.89 | 0.33-0.34 |
| a state machine split at 50,000 (`CljSplit`) | 1.73-1.80 | 0.34-0.40 |
| **split at 50,000, and `Tier3BackEdgeThreshold=6000`** | **1.64-1.74** | 0.34-0.36 |
| split at 50,000, and `CompileThresholdScaling=0.3` | 1.64-1.72 | 0.36-0.40 |
| C2 on (no `TieredStopAtLevel`), split at 110,000 | 2.87-3.03 | 0.34-0.37 |
| C2 on, split at 50,000 | 2.43-2.46 | 0.35-0.36 |
| split at 25,000 | does not build (a Clojure compile error) | |
| outlining functions that fit one method, at 20,000 or 40,000 | 1.84-2.38 | 0.33-0.39 |

With the split at 50,000, `ex_substitute` falls from 10.4% of the samples
to 4.1%, half of them compiled; `match_chunk`, which is small but called a
few dozen times around its loop, stays interpreted until the back-edge
threshold is lowered. C2 compiles too slowly to pay back on a run this
short, as `doc/CLOJURE-IDIOMS.md` found.

## The change this chose, done

**Done** (2026-09-29): `CljSplit: 50000` and `-XX:Tier3BackEdgeThreshold=6000`.
The heavy case at three sizes, before and after, the same screens:

| lines | before (s) | after (s) | the C (s) |
| ---: | ---: | ---: | ---: |
| 500 | 0.83-0.90 | 0.77-0.82 | 0.04 |
| 5,000 | 1.85-2.01 | 1.72-1.76 | 0.44 |
| 50,000 | 10.9-12.1 | 10.7-11.1 | 4.31 |

The suite's heavy case: the Clojure editor 3.4-3.9 times the C, from 4.4.
The Java editor, which shares the launcher's flags, as before (0.73-0.88 s
against 0.77-0.85). The smaller split found a bug the backend had carried:
a split group's return out of a region that can return (`ex_substitute`,
no previous pattern) was the region's value -- a void function's `nil` --
where the group must yield -1; the wide suite's `:substitute`, `:&`, `:~`,
`:smagic` and `:snomagic` ended in a NullPointerException, and now answer
as the C does (`retOf`, `clj_shape.go`).

## The change this chooses (as it was chosen)

**A state machine split at 50,000, and a lower back-edge threshold for C1**
(`CljSplit` in `internal/whim/gen.go`, `-XX:Tier3BackEdgeThreshold=6000` in
the launcher's flags): the heavy case 1.93 -> 1.69 s, about 13%, short
sessions as they were. Both are one line each; the split needs the suites,
since it changes the generated namespace, and the flag the heavy case at
other sizes, since it makes C1 compile more.

What it does not reach: the per-character loops at 3-7 times the C (the
redraw's line counting above all) and the 0.2 s of start-up. Those would
take the backend's code for a byte pointer's walk (`BytePtr` and its
boxing), not the JIT's thresholds -- a larger change, and the next
measurement's to choose.

## The per-character loops: bytes boxed (2026-09-30)

The next measurement, at `0876ad6` after the change above: the redraw's line
counting (`plines_win_nofold`, `win_linetabsize_cts`,
`win_lbr_chartabsize`, `ptr2cells`) was still a quarter of the samples.
`ptr2cells`'s bytecode showed why it was 7.3 times the C: each unsigned byte
read, `(bit-and (.get p) 0xff)`, boxed the byte (`Byte.valueOf`, a
checkcast, `RT.uncheckedLongCast`), since Clojure's arithmetic has no
`byte` overload. `:warn-on-boxed` does not report it. There were 2,200 such
reads. They are now `(.ub p)` / `(.ub p k)` (`BytePtr.ub()`, the unsigned
read as a `long`: an `int`, `u()`, the Java's, made a `let` or `loop` local an
Object and the build refused its reflection). Every other byte source is
`(bit-and (long x) 0xff)`.

Timed without the AOT cache, before and after, three runs each, the same
screens:

| lines | before (s) | after (s) |
| ---: | ---: | ---: |
| 500 | 1.57-1.64 | 1.60-1.65 |
| 5,000 | 2.55-2.57 | 2.39-2.56 |
| 50,000 | 10.9-11.2 | 10.2-10.6 |

`ptr2cells` fell from 6.9% of the samples to 2.9%. The suite's heavy case
reads 3.7 times the C, inside the 3.4-3.9 it read before: a few percent, and
under the heavy case's noise at 5,000 lines.

What is left, not done:

- **Boxing at calls** (`Numbers.num`, 3.5% of the samples, most of it in
  `ex_substitute`). Clojure gives a function a primitive signature only up to
  4 parameters, so `vim_regsub_multi` (7), `search_found` and
  `vim_regexec_multi` take their longs boxed on every line. Removing it
  would take another calling convention for the backend, not a rule.
- **Start-up** (about 10% of the samples, 0.2 s), which the AOT cache
  already cuts.
- **A `BytePtr` allocated a step**: `(.add p n)` in the walks, and
  `(.lt p (.add line len))` each turn of `win_linetabsize_cts`. The C does
  the same arithmetic; a mutable cursor would be a different runtime.

