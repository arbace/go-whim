# Phase 143 — `regatom()` has no goto

`regatom()` jumped into the middle of other cases three ways (finding 11):
- `\_%)` jumped to the delimiter atom inside the `\%` case's own switch, and
  so did `\%>` when no digit follows it;
- `\_[` jumped to the collection that starts the `[` case;
- `.` followed by a composing character jumped to the multibyte node inside
  the default case.

Go cannot jump into a case or a block, and none of these does now:
- **the delimiter atom:** its block becomes `regatom_delim()`, moved verbatim,
  and its case and both jumps call it. `regnode()` never returns NULL, so a
  NULL can only be the block's own error return, and it is passed on;
- **the multibyte node:** its three statements are written where the jump
  was;
- **the collection:** the switch dispatches on `sw`, not `c`, in a loop that
  runs once. The `\_[` path sets `sw` to the `[` case and continues, while `c`
  stays `'['` as the jump left it.

The last jumps into a case are in `edit()`. `check_termcode()`'s jump goes into
an `if` body.

**Declared delta: nothing.** The check requires `regatom_delim()`'s body to be
the input's block line for line. It diffs `regatom()` from input to output and
requires every lost and gained line to be one the phase accounts for. Its
probes cover `\%)`, `\%t)`, `\%f]`, `\%>`, `\_%)`, `\_[` and `.` before a
composing character; each control moves.
