# Phase 177 — a line's match on its own

The second C phase of the parallel `:%s` (`doc/PARALLEL-SUBSTITUTE.md`): the
regex engine can match one line handed to it and nothing else, and
`ex_substitute`, before its loop, makes each line's searches that way -- a
question whose lines are independent, so that a target may answer it on many
cores at once. The C answers it one line after another; that is its meaning,
and the Go, Java and Clojure editors' parallel answers are held to it.

**The engine alone.** Phase 176 made the engine's state a parameter; two
members join it, `string_T alone` -- the line a match reads, when set -- and
`bool failed`. With a line set, the engine reads that line and no other,
polls no host, gives no message and writes nothing into the compiled
pattern, and whatever would have needed more **fails the match alone**
instead of doing it:

| what a match would do | where | alone |
| --- | --- | --- |
| read another line: a multi-line pattern (`\n`, `\_x`), look-behind at a line's start | `reg_getline_common`, `reg_nextline` | fails |
| read the cursor, a mark, the Visual area, the line's number, the file's first or last line, a virtual column (`\%#`, `\%'m`, `\%V`, `\%23l`, `\%^`, `\%$`, `\%23v`) | seven atoms of `regmatch` | fails |
| a message: `'maxmempattern'` exceeded, a corrupt program, a null argument | five functions | fails |
| look for the must-have string where case is ignored (its length may shrink, and later searches read it) | `bt_regexec_both` | fails |
| poll the host for an interrupt | `reg_nextline`, `regmatch` | not done |
| write the must-have string's length back into the program | `bt_regexec_both` | not done: it is unchanged where case matters |

**What is found.** `match_chunk(re, rmp, do_all, buf, lines, line1, from,
to, found)` runs one engine over lines `[from, to)` of a snapshot and makes
each line's searches where `ex_substitute`'s loop will make them -- from
column 0, then, with `g`, from where each match ended, one character on
after an empty match where the last one ended, until the line's end -- and
records each: the column it started at, what it returned, `rmm_matchcol`,
and the positions it left, up to the last one set (the engine leaves every
later one at -1). `match_lines` runs a new engine over all the lines and says
whether none failed. That is the function whose body a target writes in
parallel (`internal/whim/gen.go`: chunks on goroutines in Go, on the common
fork-join pool in Java and Clojure, each chunk on an engine of its own).

**ex_substitute.** Unless it asks for confirmation (`c`) or its range is one
line, it snapshots the range's lines -- a pointer and a length each,
`ml_get`'s, before anything changes -- and calls `match_lines`. If that
succeeded, each of the loop's two searches of a line's original text -- the
first, from column 0, and the next one, before the line is replaced --
goes through `search_found`: when the line's next recorded search started at
the column asked for, its answer is the search's -- the return value, and on
a match the positions and `rmm_matchcol` put back into `regmatch` -- and
otherwise the search is made, and the line's records are done with.
Everything else stays in the loop, in order: the replacement, undo, marks,
line splits and joins, the counts and the messages are what they were. A
line's index in the snapshot is its number less the range's first, less the
lines the loop has added or removed so far, since it adds and removes them
only at or before the line it is on.

It is exact whatever the pattern: a search is answered from the records
only when the engine made that very search -- the same line's text, from the
same column, with the same program and flags -- and needed nothing but the
line to do it; and a pattern that needed more fails the snapshot, and the
loop searches as before. When the loop's next search is not the recorded
one (a search that went beyond the line, taken anew), the line's records
are not read again. After a search that found nothing the loop reads no
position, so none is put back.

**Not done while matching alone:** an interrupt (`CTRL-C`) is not polled, so
a long `:%s` cannot be interrupted before the snapshot's matching ends; and
the snapshot and the records are allocated in the C's arena, which never
frees -- as every line copy `:s` makes already is.

**Measured.** 75,522 -> 75,642 lines (phase 176's product). Every search of
every eligible `:%s` in the suite's cases comes from the records, none is
made anew (an instrumented build counted them: `par_literal` 5,000 of 5,000,
`par_empty` 156,786 empty matches); the six cases whose pattern needs more
(`par_join`, `par_cursor`, `par_visual`, `par_mark`, `par_lnum`,
`par_behind`) fail the snapshot and search as before, and `par_confirm` and
`par_global` (one line at a time) never take it. The quick and the wide
suites answer as the commit before on the C, and the Go, Java and Clojure
editors as the C -- the Go's also built with `-race`, which reports nothing.

A `:%s` on the survey's buffer (`doc/PARALLEL-SUBSTITUTE.md`: eight varied
lines built into 500,000 by keys, the substitution's time the median of
three runs less the buffer's), 64 cores, every output the same bytes:

| seconds | lit | dense | bt | cls | sel |
| --- | --- | --- | --- | --- | --- |
| C, phase 176 | 1.17 | 2.52 | 24.6 | 3.68 | .243 |
| C, phase 177 | 1.62 | 4.18 | 24.8 | 4.08 | .395 |
| Go, phase 176 | .640 | 1.16 | 10.2 | 2.90 | .210 |
| Go, phase 177 | **.372** | **.587** | **.376** | **.176** | **.006** |
| Go, phase 177, `GOMAXPROCS=1` | .796 | 1.33 | 9.89 | 2.74 | .250 |
| Java, phase 176 | 2.24 | 3.45 | 18.3 | 5.93 | .445 |
| Java, phase 177 | **1.42** | **2.59** | **1.58** | **.952** | **.208** |
| Clojure, phase 176 | 7.57 | 11.7 | 123 | 40.2 | 2.41 |
| Clojure, phase 177 | **3.31** | **6.73** | **4.52** | **2.20** | **.208** |

`lit` is `:%s/the/THE/g`, `dense` `:%s/e/E/g`, `bt` `:%s/\v(a|b)+c/X/g`, `cls`
`:%s/[ab]\+c/X/g`, `sel` `:%s/needle/pin/g`. The Go is 1.7 to 35 times as
fast, the Java 1.3 to 11.5, the Clojure 1.7 to 27; run on one CPU the Go
is about as fast as before, so the gain is the cores. The C, which runs the
matching and then the loop on one thread and records every search in
between, is slower on the patterns that match most: it is the meaning, not
the product.

Two things the targets needed besides the bodies. The JVM launchers ran the
serial collector, which stopped every thread while one collected: the Java's
`bt` took 3.1 s at 100,000 lines with it and 0.98 s with `-XX:+UseParallelGC`,
which starts as fast, and is the launchers' now (`braaam.JVMFlags`). And
the snapshot's lines were first written inside `ex_substitute`, which grew
its Clojure method to 26,715 bytes, past what C1 compiles ("out of virtual
registers"): interpreted, the Clojure's `dense` took 6.0 s at 100,000 lines
against 1.75 once `match_range` and `search_found` took the code out. The
heavy case moved with it: the Java 2.4 -> 1.9 times the C, the Clojure 8-10
-> 4.5.
