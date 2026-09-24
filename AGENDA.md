# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Landed: canonicalisation at phase 0

Merged 2026-09-24 (`d8365fb`). Kept here only as the pointer a later reader will
want: the account is in that merge commit, and the two migrations behind it are
in the branch's 22.

What it left behind for this file: the stale `Measured` tables below, and the
two "literals that
ate an indent" still in the product -- a `return;` at 30 spaces
(`phase/074/editlit.go`, `whim-vim.c:28014`) and a `got_int = TRUE;` at 29
(`internal/cut/nosignals.go`, `whim-vim.c:52439`). Neither fails a check. They
are the same class as the phase 145 and 104 defects the merge names, and they
belong with the inserted-text item.

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
- **Inserted text is not canonical.** Phases write residue-spelled blocks into the
  tree, so the product carries them.
- **44 whole-line comments inside braces, from four phases**, are what now stop an
  intermediate text canonicalising. The fix is where those phases put their
  comments, not the printer -- the finished product has zero.
- **Eleven retired tools are still named in the prose**, every one gone from disk:
  `phaserun.sh` in 30 `.md` files, `coredelta.sh` in 22, `zcompare.py` in 20,
  `zrecord.sh` in 14, `stages.sh` and `whimdelta.sh` in 8 each, plus
  `implhash.sh`, `ztermcheck.py`, `pipeline.sh`, `memo.sh`, `verifypass.sh`. A
  `GOAL.md` that says to run a file that does not exist is a dead end. The fix
  differs per file: a doc should be corrected, a phase's record may deserve
  "(now `internal/verify`)" rather than a rename.
- **`zero-vim` in prose**: 20 in `GOALS.md`, 7 in `phase/STAGES.md`, 6 across five
  `delta.md`. `GOALS.md`'s appendix is the plan AS WRITTEN, so renaming inside it
  falsifies a record -- a judgement, not a sweep.

## Small

- `tools/` shell still to become Go: `phasecheck`, `phasebuild`, `symbols`,
  `enumvals`, `score`. (`enumvals.sh` STAYS while twelve phase checks use it as
  the independent DWARF control.)
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
