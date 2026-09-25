# Phase 46 — no `:wall`, `:qall`, `:quitall`, `:wqall` or `:xall`

With one window and one buffer these were `:w`, `:q`, `:wq` and `:x` under longer
names. `:quitall` is `:qall`'s long spelling, the same handler, and goes with it.
`do_wqall()` and `ex_quit_all()` go with their rows.

**`tools/exsweep.py` quit every run with `:qall!`**, which is what made `:new`,
`:split` and the other window commands deterministic in `slim-vim`. It now runs
the binary once with `+qall!` and quits with `:q!` wherever that fails — which is
the same thing from Phase 39 on, where there is one window. Against `slim-vim`
it still quits with `:qall!`, and `tools/verify.sh` is all clear. The phase
checks that `:q!` still quits and `:qa!` is not a command.

## The delta

**The five rows**, which succeeded run bare. Measured: 115,744 → **115,646
lines**.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 48, as the first of the group 44-48, which was one idea split for history's sake. There is no boundary q046 of its own any more; everything above still says what the steps do and why.
