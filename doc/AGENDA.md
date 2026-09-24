# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## Queued, measured, not started

- **A test suite derived from upstream.** The one the phases were verified with
  -- every `check.go`, the declared deltas, the recorders (`internal/harness`),
  `internal/check`, `internal/verify`, the baselines and `make whim-verify` --
  was removed after `448e9a8`, the last commit that has it, because verifying
  was a pipeline of its own that cost more than it returned. What proves a change
  now is `make whim-build-check` alone, which sees the text and not the editor.
  Re-deriving a suite from the behaviour of upstream slim-vim is the plan; the
  archived one is where its corpus, its recorders and its pty harness can be
  read.

## Known stale, not yet scoped

- **Inserted text is not canonical at the boundaries.** Phase 163 prints the
  PRODUCT canonically (1,080 lines moved at q162, 226 beyond whitespace), so
  the product is held to the form; every intermediate still carries what the
  phases wrote. Fixing that is per phase, and optional: phase 163 would then
  change nothing and its check would say the input was already canonical.

## Declined, with the reason recorded

**The closure as the sweep's analysis, switched on.** Built and measured, and merged
OFF (`271ea7d`, `WHIM_CLOSURE=1`): with it on, no phase refuses (phases 78, 99,
138 and 160 accept "already gone" when the closure took what they cut), q82
records identical to the core baselines, and it buys the product 5 entities,
7 lines (75,218 against 75,225) -- the other ~114 dead things it finds at q82
later phases remove anyway -- for a build ~80 % slower (1,874 s against 1,044
s). Not worth the re-derivation and a full whim-verify today; one env var
away if the balance changes. The reporter (`go tool whim reach`, cast-guarded)
stays the standing answer to what is still dead: 16 on the product.

**In-AST editing.** `doc/AST-EDITING.md`, and `GOALS.md`'s *What comes next*.
Not on cost: `internal/cemit` joins the AST to the source text by byte offset, so
a mutation moves the tree while the text stands still, and deleting a table row
gives BYTE-IDENTICAL output -- an edit that did nothing, which `whim-build-check`
cannot see. 10 of 10 deletions did this. What survives the assessment is the
cheap half: **the AST as a locator, with the text still doing the editing**.
