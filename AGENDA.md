# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

- **The closure replacing typereach/deadfields/deadenums' analysis** --
  `internal/reach` is the analysis, as a reporter (`tools/st.sh reach FILE`).
  Run on all 44 texts in `.build/` (the input, every boundary tar; residue-era,
  from before canonicalisation): it parses every one, gcc's unused set and the
  planted copy agree with it on every one (the positional trial declines one
  member at q12 and q41, whose struct is written on one line), and it reproduces the survey's 119 at q82 and 192 at q41. It moves
  the product (119 entities at q82), and TWO of its 16 findings on the product
  are wrong in a way gcc reports as a warning with exit 0, not one:
  `termrequest_T.tr_start` and `vimoption.opt_expand_cb`, whose `options[]`
  table places 555 elements by position -- deleting it gives 185 `excess
  elements in scalar initializer` and exit 0. The reporter carries both as
  findings of their own class. `sweep ∖ closure` over the tars, which is what
  the survey measured on five texts, was not measured: that needs a sweep of
  each, and two pipelines cannot share `.cache`.

## Known stale, not yet scoped

- **Most phases' `GOAL.md` `Measured` tables are residue-era** -- line counts and
  binary sizes from before canonicalisation. Only the rows this work re-derived
  were changed, and each such row says so in the row itself. Re-measure, never
  adjust by reasoning: `CLAUDE.md` says the numbers are measurements.
- **Inserted text is not canonical at the boundaries.** Phase 163 prints the
  PRODUCT canonically (1,080 lines moved at q162, 226 beyond whitespace), so
  the product is held to the form; every intermediate still carries what the
  phases wrote. Fixing that is per phase, and optional: phase 163 would then
  change nothing and its check would say the input was already canonical.

## Small

- `internal/check/phases0NN.go` naming -- needs code moved into the phase
  packages, so not mechanical.
- The `z*` identifier family (`zmemline`, `ztc`, `z33Rule`, `z41*`).

## Declined, with the reason recorded

**In-AST editing.** `surveys/AST-EDITING.md`, and `GOALS.md`'s *What comes next*.
Not on cost: `internal/cemit` joins the AST to the source text by byte offset, so
a mutation moves the tree while the text stands still, and deleting a table row
gives BYTE-IDENTICAL output -- an edit that did nothing, which `whim-build-check`
cannot see. 10 of 10 deletions did this. What survives the assessment is the
cheap half: **the AST as a locator, with the text still doing the editing**.
