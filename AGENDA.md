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

  **Step 1 measured: NO-GO as it stands** (branch `closure-swap`, `492640f`,
  behind `WHIM_CLOSURE=1`, off by default and `cmp`-identical to main off).
  With it on, `build --keep-going`: 45 phases refuse, 78,209 lines against
  75,225. And the core's BEHAVIOUR moves at q82: every record of a recording
  differs from `.reference/core-baselines` (the switch-off q82 matches them),
  because the closure deleted `re_engine` and `re_flags` from `bt_regprog_T`,
  which is read only through `regprog_T`, the shared opening sequence the code
  casts between -- `re_in_use` moved and every pattern gives `E956`. So the
  closure's MEMBER class is unsound under struct casts, and so is the
  reporter's: its 119 at q82 include those two. Before a second attempt: (1)
  a guard, or a refusal class in the reporter, for members of structs used
  through a common initial sequence (`internal/ccx`'s casts are the start);
  (2) phases 78 (`cmdarg_T.prechar`) and 99 (`stat_T`) assert counts the
  closure changes, and 99's refusal cascades through every phase after it.
  **(1) is done** (`3873ec0`, the reporter's `ClassCast`: every member of both
  sides of a cast between two struct pointer types is held; 7 punned pairs at
  q82, 3 of its 119 held, 0 on the product). With it, the switch-on q82 records
  IDENTICAL to the core baselines. What is left is (2): 44 phases still refuse,
  all from 78 and 99 -- decide whether those phases change or the closure
  leaves alone what a later edit expects to find.

## Known stale, not yet scoped

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
