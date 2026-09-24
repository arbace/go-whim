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

- **The reachability closure as a REPORTER** -- `surveys/REACHABILITY.md`.
  `sweep \ closure = empty` over 1,059 removals on five texts; ships as an
  `internal/ccx`-shaped partition refusing on a leftover, with gcc as the control
  that lets it fail. It already answers *what is still dead in the product*: 16
  things, named. Costs nothing and moves no bytes.
- **The closure replacing typereach/deadfields/deadenums' analysis** -- not until
  the reporter has run against the boundary tars. It moves the product (119
  entities at q82), and one of its 16 findings is WRONG in a way gcc reports as a
  warning with exit 0.

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
- **Retired tools in the prose: what is left.** The prose names 93 scripts not on
  disk, about 900 times, nearly all as records of what was run; those keep their
  names, and `tools/README.md`'s *Retired* table says what each became. The
  present-tense claims and instructions were corrected. Left: `CLAUDE.md`'s
  `CORE_FROM=83` in `tools/pipeline.sh` (it is `internal/build`'s `CoreFrom`),
  and Go comments that still describe the old machinery as current --
  `phase/{033,054,080,081,085}/check.go` "as tools/phaserun.sh describes",
  `phase/{096,097,099,106}/check.go` on `coredelta.sh` and `phaserun.sh`.
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
