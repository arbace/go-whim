# Phase 93 — no address of a position's line or column

*Formerly phase 174. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

A C position, `pos_T`, is a line and a column, `lnum` and `col`. Two
functions took the address of one of them, and nothing else in the core
does; but a member whose address is taken anywhere is a one-element array in
the Java and the Clojure editors (neither has an address of a variable), so
every cursor read in those files was `w_cursor.lnum[0]` because of them
(`doc/JAVA-IDIOMS.md`, item 6.1; `doc/CLOJURE-IDIOMS.md`, item 5):

- **`mark_adjust_internal()`** holds 13 expansions of vim's `one_adjust()`
  and `one_adjust_nodel()` macros: `lp = &(curbuf->b_namedm[i].lnum);`, then
  `*lp` compared and moved with the lines. They become two functions of the
  value, defined before their caller, and each expansion a call:
  `curbuf->b_namedm[i].lnum = one_adjust(curbuf->b_namedm[i].lnum, line1,
  line2, amount, amount_after);`. Six are `one_adjust` (a deleted line's mark
  becomes 0), seven `one_adjust_nodel` (it becomes `line1`), as vim wrote
  them. The lvalues are plain members, so reading one twice is reading it
  once. `lp` goes with them (the sweep).
- **`cursor_pos_info()`** called `getvcols(curwin, &min_pos, &max_pos,
  &min_pos.col, &max_pos.col, 0)`; the two columns now come back through two
  locals, copied into the positions after the call. `getvcols()` reads both
  positions whole before it writes either column, so it sees what it saw.

Two more members were boxed by the same macro, `w_old_cursor_lnum` and
`w_old_visual_lnum`; they are plain now too.

**Measured:** 75,539 -> 75,346 lines (-192 edited, -1 swept: `lp`). No
member of `pos_T` has its address taken. The Java editor's `[0]` reads 7,032
-> 4,874, none of them on `lnum` or `col` (1,119 and 1,003 before); the
Clojure editor's `(aget ... 0)` 10,786 -> 8,997; the Go's `editor.go` moves
only in the two functions and their callers (176 lines). The phase changes
nothing the editor does: `whim test` 45/45 as HEAD's, and the Go, Java and
Clojure editors all 45 and all 240 (`--wide`) as the C; the heavy case's
times as before; `whim-build-check` gives the product back.
