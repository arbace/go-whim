# Phase 179 — no mark is cleared when none was set

`:g/pat/cmd` marks the lines `pat` matches (`ml_setmarked`), runs `cmd` on
each, and clears the marks (`ml_clearmarked`), which walks the buffer from
`lowest_marked`, the first marked line. When nothing matched, that is 0, and
the walk starts at line 0: `ml_find_line` finds line 1's block, and the loop
reads the slot before its first line -- `db_line[-1]`. Upstream vim does the
same (`db_index[-1]`), where it reads a neighbouring field and, the flag
being clear there by chance, writes nothing. In whim-vim.c it is undefined
behaviour the C product survived; the Go editor panicked on it ("index out
of range [-1]") and the Java and Clojure editors failed alike, on
`:g/nomatch/d` in a one-line buffer as in a large one. No case had a `:g`
that matched nothing until phase 178's `par_gnomatch`.

The function now returns when `lowest_marked` is 0: no line is marked then
(`ml_setmarked` sets it to the first line it marks), and line 0 never is,
so nothing the walk cleared is left uncleared.

**Measured:** one line of `ml_clearmarked`; the C answers every case as the
commit before, and `par_gnomatch` -- `:g/nomatch/d` and `:v/./d` over 3,000
lines -- now answers on the Go, Java and Clojure editors as on the C.
