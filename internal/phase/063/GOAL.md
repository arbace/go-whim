# Phase 63 — no jump list

The per-window jump list goes: `w_jumplist`, `w_jumplistlen` and
`w_jumplistidx`; `setpcmark()` appending to it; CTRL-O and CTRL-I walking it
through `movemark()`; `:jumps` and `:clearjumps`, now `ex_ni`; `cleanup_jumplist()`;
copying it into a new window and freeing it with one; and the loops in
`mark_adjust_internal()`, `mark_col_adjust()`, `mark_forget_file()` and
`fmarks_check_names()` that kept its marks right when lines moved or a file was
forgotten.

**What stays, because it is not the jump list.** The previous-context mark behind
`''` and `` ` ` `` — `w_pcmark`, still set by `setpcmark()`. The change list and
`g;`/`g,`: `nv_pcmark()` served both, and keeps that half. `:keepjumps`, which
guards the pcmark and the change list too. And `JUMPLISTSIZE`, which sizes the
change list. CTRL-O in Select mode still runs one Visual command; elsewhere CTRL-O
and CTRL-I beep, and `<Tab>` is mapped to `%` in this build, so losing CTRL-I's
meaning costs nothing typed. The phase greps afterwards that `w_pcmark` and
`movechangelist` survived, since either going would mean it took more than the
jump list.

It was tried first on the Phase 61 boundary, before Phase 62 was recorded. That
trial failed only on the `'jumpoptions'` block Phase 62 removes — so it checked
this phase's own script and nothing else.

The phase checks `:jumps` is refused, CTRL-O after `3G` leaves the cursor on line
3, and `''` after `3G` still returns to line 1.

## The delta

`:jumps` and `:clearjumps`, now `ex_ni`. Measured: 100,643 → **100,354 lines**.
