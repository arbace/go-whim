# Phase 97 — :g marks the lines match_lines finds

*Formerly phase 178. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

`:g/pat/cmd` and `:v/pat/cmd` first mark the lines of their range that `pat`
matches (or does not): `ex_global`'s marking pass searches each line from
column 0 and changes nothing while it runs -- `ml_setmarked` sets a flag and
moves no text. That is the question phase 177 made a primitive with a
parallel body, so the pass asks `match_range` first, as `ex_substitute`
does: one search a line (`do_all` false), made on each line alone and
recorded, and each of the pass's searches is taken from the record through
`search_found`. A pattern that needs more than a line's text fails the
snapshot and the pass searches as before; the pass's range is one line inside
a `:g` that runs `:g` (`global_busy`), which is left as it was. Then `cmd`
runs on the marked lines one after another, as before.

It is exact for the reason phase 177 is: a search is answered from the
record only when the engine made that very search on the same line's text
and needed nothing else, and the pass changes no line while it asks.

The cases (`internal/suite/cases.md`, 7 `par_g*`/`par_v*` beside phase
177's): `:g/abc/d`, `:g/\v(a|b)+c/normal A!`, `:v/7/s/a/A/g`, a range, a
multi-line pattern and `\%#` (both falling back), and a `:g` and a `:v` that
mark nothing -- which found an old failure, phase 179's.

**Measured.** 75,642 -> 75,644 lines. An instrumented build answered all of
the marking pass's searches from the records (3,000 of 3,000 on the cases'
buffer, 1,901 of 1,901 on the range), and the two that need more fell back.
A `:g` at 500,000 lines of phase 177's buffer (seconds, median of three less
the build, 64 cores, every output the same bytes), before and after:

| seconds | `:g/needle/d` | `:g/[ab]\+c/d` | `:g/\v(a\|b)+c/d` | `:v/the/d` |
| --- | --- | --- | --- | --- |
| C | .222 -> .387 | 3.69 -> 3.95 | 22.6 -> 23.9 | 1.28 -> 1.53 |
| Go | .184 -> **.058** | 2.81 -> **.620** | 9.06 -> **.799** | .940 -> **.798** |
| Go, `GOMAXPROCS=1` | .243 | 2.88 | 8.92 | 1.10 |
| Java | .517 -> **.306** | 5.39 -> **1.94** | 14.3 -> **2.77** | 1.95 -> **1.69** |
| Clojure | 2.56 -> **.440** | 36.2 -> **3.77** | 106 -> **5.56** | 5.75 -> **3.90** |

What is left of each is the command's: the two middle ones delete 62,500
lines one after another, and `:v/the/d` 187,500.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the loop is found by its form and its locals and first statement written by FRAG, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
