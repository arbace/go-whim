# Phase 78 — empty functions, write-only counters, and the window id

Three unrelated kinds of leftover, all invisible to the compiler and so to every
sweep this pipeline runs. A fourth — the constant-return predicates — was **split
out into its own phase** once the survey showed it is not one shape but several:
only about twenty of the twenty-nine sit in a foldable `if`, the rest needing
term-level or expression edits, seven fold sites carry an `else`, one needs a rewrite
for an escaping `break`, and one is a function pointer in an option table row that
must not be touched. Bundling them here would have repeated the shape that cost
phase 75 eight iterations.

**Fifteen empty functions, 49 call sites.** Each was emptied by an earlier phase and
left with its callers in place. **Every one of the 49 is a bare statement** — checked,
not assumed: none appears in an `if`, an assignment or any larger expression, so a
line removal cannot corrupt a condition. That was the trap in phase 75, where
`ins_apply_autocmds` calls were invisible to a regex anchored on `apply_autocmds`.
The phase re-checks each body is still empty before removing its calls, and counts
mentions against bare-calls-plus-prototype-plus-definition so a hidden use fails the
phase rather than the compiler.

**`nv_nop` is not among them.** It is empty by design — the `nv_cmds` row for
`KE_NOP` — and `nvidxcheck.py` requires `nv_cmd_idx[]` to stay a permutation.

**Six write-only statics**: `autocmd_blocked` (its reader went in 75),
`autocmd_no_enter`, `autocmd_no_leave`, `redrawing_for_callback`, `prevwin`,
`last_win_id`. `block_autocmds()`/`unblock_autocmds()` become empty and **stay** —
eight live call sites, one deliberately unpaired in `deathtrap()` where the process
is dying and never unblocks.

**Two that the same scan flags and that must not be touched**: `breakcheck_count` is
*read* by `if (++breakcheck_count >= BREAKCHECK_SKIP)`, and `vim_ignored` is the
deliberate sink for discarded return values kept in phase 67. A scanner counting
`++x` as a write, unable to see the read in `x = call()`, reports both as write-only.

**The window id**, one chain: with one window `curwin->w_id` is constant, so both
`if (is_state.winid != curwin->w_id)` guards in `getcmdline_int` can never fire —
matched by regex, not a literal, because they sit at different indents. Folding them
makes `winid` write-only, then `w_id`, then `last_win_id` and `LOWEST_WIN_ID`.

## The field that was not dead

`termrequest_T.tr_start` was on the Tier-1 list with **one** identifier mention — its
declaration — and removing it produced three *"excess elements in struct
initializer"* errors. `termrequest_T` is positionally initialised three times as
`{STATUS_GET, -1}`, and **that `-1` is `tr_start`**. A positional initialiser names
nothing, so counting identifiers cannot see the use — which is exactly why
`deadfields.py` exempts every field of a type that has one. The exemption was
recorded during the audit and then ignored.

`cmdarg_T.prechar` is the contrasting case and was removed safely: its only
positional initialiser is `cmdarg_T ca = { 0 };`, which supplies one value and
zero-fills the rest, so it cannot overflow. The distinction is whether the initialiser
supplies enough values to reach the field being removed.

Two assertions in this phase also had to be repaired before it would run: one still
demanded `tr_start` reach zero while another demanded it survive, and an initialiser
count used `[a-z_]+_status`, which cannot match the digit in `u7_status`.

## The delta

**None**, and `whimdelta.sh` confirms it. An empty function called or not called does
the same nothing, a counter nobody reads has no effect, and the two `winid` guards
could never fire. The inventory was produced by two independent scanners that agree
exactly on all 16 empty functions and 32 stubs.

Measured: 89,407 → **89,233 lines**.
