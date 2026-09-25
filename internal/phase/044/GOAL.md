# Phase 44 — no filters, sorting or alignment

Seven rows go to `ex_ni`: `:!` (with `:{range}!`), `:sort`, `:uniq`, `:retab`,
`:left`, `:center` and `:right`. **`:!` was kept in Phase 8 on purpose**, as the
sentence it printed instead of starting a process; it is dropped here on
request. The `!` operator key built nothing but a `:{range}!` command line, so
its row in `nv_cmds[]` points at `nv_error` — pointed, not deleted, as every row
there is. Completion for `:retab` goes with its row.

**`:r !cmd` and `:w !cmd` stay as they were.** They reach `do_bang()` through
`:read` and `:write`, not through the `:!` row, and keep Phase 8's refusal:
without their `!` being special, `:w !cmd` would write a file of that name.

## The delta

**The six rows that succeeded run bare** — `:sort`, `:uniq`, `:retab`, `:left`,
`:center` and `:right` — and **three behaviour cases**, `retab`, `sort_u` and
`sort_n`, which used them. `:!` already differed from Phase 8. Measured: 116,892
→ **115,798 lines**.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 48, as the first of the group 44-48, which was one idea split for history's sake. There is no boundary q044 of its own any more; everything above still says what the steps do and why.
