# AGENDA.md

What is not done, in the order it has to happen, with what was measured rather
than what is hoped. Written 2026-09-23. **When an item lands, delete it** --
this file is a queue, not a record; the record is the commit and the `GOAL.md`.

## In flight: canonicalisation at phase 0

Phase 0 seeds the input through `internal/cemit`, so every later phase reads one
C23 spelling per construct instead of whatever the input happened to write. It
lives on **`worktree-agent-a8c7188a3477181d0`** (pushed; worktree at
`.tmp/worktrees/canon`), **20 commits ahead of main**. The `--canonical` flag is
gone: it was scaffolding while 163 phases' anchors were migrated, and a flag that
can only be on is a lie about what the pipeline does.

Measured on the branch: **163 of 163 phases build clean**, 75,273 lines, ~1,220 s.
`build --check` returns the committed product. **`whim-verify --to 82` passes 13
of 13 Part I stages**, 892 s. `whim-vim.c` and `editor/editor.go` are
regenerated. `internal/cemit` prints from `cc.Parse`, not `cc.Translate`.

It has already paid for itself. Reading the canonical product against the old one
token by token found **three latent bugs** no check could see: phase 152 emitted
an empty `get_varp_allbuf()` (the global value of every window-local option,
gone), phase 150 left three pops on the byte stack, and phase 127 deleted
`ml_flush_line`'s re-entrancy clear. Each compiled, swept, and passed every count
its phase took. A fourth was in the harness: `harness.Stage` keyed the staged
binary on its PATH, so in a whole-pipeline run every delta after the first
measured phase 0's binary -- slim against slim's own baselines -- and reported
that nothing had moved. **A delta that could not fail.**

### Before it merges, in order

1. **Name what moved at q82, then decide about `.reference/core-baselines`.**
   q82 went 86,583 -> 84,111 lines, so phase 83 REFUSES to overwrite the recorded
   set, and it is right to: what moved must be named before a recording is thrown
   away. The Part II checks are what would name it. **Do not re-record to make a
   run pass** -- a set regenerated from a binary a later phase can reach agrees by
   construction and proves nothing.
2. **Verify Part II** (phases 83-162), which needs step 1. Expect it to refuse
   things it used to pass, and expect those refusals to be REAL: an `each` stage
   runs a delta per phase in one process, so every delta after the first in
   stages 87-115 was vacuous until `cab3d36`.
3. **Merge to main and push.** Nothing lands before step 2.
4. **Re-measure what the merge makes stale.** Most phases' `GOAL.md` numbers are
   residue-era (line counts, binary sizes); only 054 and 101 were updated, where
   a *statement* became wrong rather than a number stale. Re-measure, never adjust
   by reasoning.

## Queued, measured, not started

- **The enumerator values from the front end** -- committed on main (`566a6bd`)
  and NOT yet proved end to end. Per-text agreement is measured: byte-identical
  on `whim-vim.c` (1,155 both ways), 37 one-directional differences on
  `slim-vim.c` (names the tree declares and DWARF omits, zero value
  disagreements), and 1,141 on `editor.c`, which `tools/enumvals.sh` cannot
  answer for at all. The gate is a build with each path required byte-identical.
  Note `whim-build-check` on main already fails for a separate deliberate
  reason: `c975619` changed the arena message without regenerating the product.
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
