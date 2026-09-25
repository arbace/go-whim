# Phase 165 — no store nothing reads

Six stores to locals that nothing reads before they are overwritten or leave
scope. staticcheck found them in the Go (SA4006, SA4009), and all six are the
C's:
- `open_line()`'s `ptr = ml_get_curline()`, fetched right after the indent and
  never looked at;
- `edit()`'s `i = showmode()`. The call draws the mode and stays; the store
  goes;
- `cmdline_handle_ctrl_bsl()`'s parameter `c`, which is overwritten by the key
  read before any use. It becomes a local, and the one caller stops passing it;
- `next_search_hl()`'s `++matchcol` just before the loop is left;
- `adjust_skipcol()`'s `col = col % width2`, which nothing after it asks for;
- `do_put()`'s `lnum--` after the last use of `lnum`.

They are named edits, each found exactly once in its own function. Finding
such stores in general is dataflow through `goto`s, and there are six.

**Measured:** staticcheck's default checks on `./editor` go to 0. Together
with phase 164, the generator's own fixes and phase 149's widened rule, `go
vet`, staticcheck and `gofmt -s` are all clean. `whim-test`: 45/45, C and Go.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 164-165 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).
