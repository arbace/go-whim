# Phase 163 — the product is in the one canonical spelling

Phase 0 hands the pipeline its input in the one canonical C23 form
(`internal/cemit`), and every phase after it reads that form. What the phases
WRITE is not held to it: a cut leaves a one-enumerator `typedef enum` spread
over four lines, a Part II phase inserts `enum :\n    int { INT_MAX … }`, an
inserted initialiser keeps the residue era's column alignment. Measured on
the product this phase is handed (q162, 75,273 lines): the canonical printer
changes **1,080 lines**, 226 of them in more than whitespace and blank lines,
and gives 75,225.

So the last phase prints the product canonically, and nothing else. It is one
step, `cemit`, the same step phase 0 seeds with. Fixing every phase that writes
a non-canonical shape was the alternative: it would hold every boundary to the
form and not only the last, and it touches many phases where this touches one.
It was not chosen, and nothing here prevents it later -- this phase would then
change nothing and say so.

**Declared delta: nothing**, and more: there is no `-g`, so a change of layout
alone leaves the binary as it was. The check requires

1. **the fixed point**: the printer, run on the output, gives the output back;
   and the input was not already canonical, or the phase did nothing;
2. **the same bytes**: the input and the output, built with the boundary's
   flags and `SOURCE_DATE_EPOCH=0`, are one binary;
3. **the control**: the output with one string it prints (`add_time`'s
   `"%ld seconds ago"`) changed builds to different bytes, so 2 can fail;
4. the libc surface unchanged.

Measured, `tools/st.sh verify --from 163 --src` the committed q162: 75,273 →
75,225 lines, the output its own canonical form, **759,496 bytes either
side**, the control different, the declared delta exactly as declared, 32 s.
`editor/editor.go` does not move: `internal/gen` reads the AST, and the
AST is the same.

## Empty now

Every boundary is printed canonically since the build did it at the end of
every phase (`internal/build`'s `finish`), so the text this phase is handed is
already its own canonical form and the `cemit` step had nothing to do. The
phase is kept, empty, so the numbering does not move.
