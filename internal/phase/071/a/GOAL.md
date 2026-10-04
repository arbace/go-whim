# Phase 71a — `regatom()` has no goto

*Formerly phase 143, now part a of phase 71. The other phase numbers in this file are the old
numbering, as it was written: `doc/PHASES.md` maps them.*

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

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 145, as the first of the group 143-145, which was one idea split for history's sake. There is no boundary q143 of its own any more; everything above still says what the steps do and why.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the labelled block is outlined and the jumps replaced by FRAG in one unit, and the switch moved whole into the loop BUILD makes, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
