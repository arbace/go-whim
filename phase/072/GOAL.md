# Phase 72 — one window, one tabpage, structurally

**The invariant is provable, not imposed** — the phase 68 shape rather than the phase
70 one. Windows are created in exactly one place: `win_alloc(NULL, FALSE)` from
`win_alloc_firstwin()`, whose only caller is `win_alloc_first()` at startup.
`alloc_tabpage()` is called exactly once, from the same function, and `curtab` is set
to it there. `:tabnew`, `:tabedit`, `:split`, `:new` and the rest are already
`ex_ni`, and `aucmd_win` went in phase 68. So `firstwin == lastwin == curwin` and
`first_tabpage == curtab`, always.

## Two layers, folded as a pair

Nearly every tabpage walk immediately contains

```c
for ((wp) = ((tp) == curtab) ? firstwin : (tp)->tp_firstwin; (wp); (wp) = (wp)->w_next)
```

so folding the outer walk to `tp = curtab` makes that ternary constant-fold to
`firstwin`, and folding the inner one then gives `curwin`. Cutting one layer without
the other would leave half a traversal at 15 sites. 43 walks folded: 13 nested, 14
over the tabpage list, 16 over the window list.

**The whole tabpage-switching group hangs from one gate.** `goto_tabpage_tp()`'s body
is `if (tp != curtab && leave_tabpage(…) == OK)`, never true with one tabpage.
Folding that gate orphans `leave_tabpage`, `enter_tabpage`, `valid_tabpage`,
`use_tabpage`, `win_init`, `win_copy_options` and `win_init_some`, and the sweep
removes them — taking the last readers of `tp_firstwin`, `tp_lastwin` and
`tp_prevwin` with them, 13 dead fields in all. Nothing here deletes those by name.

## The break audit, run before writing the script

Phase 71 learned that folding a walk rebinds any `break`/`continue` in its body, and
that the failure can be silent. So this phase's walk audit ran **first**, and named
its seven exceptions in advance: `aucmd_prepbuf`, `can_unload_buffer`,
`borrow_stl_vsep_hl` (two walks), `current_win_nr`, `current_tab_nr`, `getout` and
`create_windows`. Each is rewritten rather than folded, and `fold_walks()` **dies**
rather than skipping if it ever meets an eighth. It did not.

`borrow_stl_vsep_hl` lends a status line's highlight to the separator beside it; with
one window there is no beside, so the function and its two calls go.

## What the walk audit could not see

`w_next` survived the fold with six readers, because `win_ins_lines`,
`win_del_lines` and `win_do_lines` ask `wp->w_next` as a **layout question** — "is
there anything below this window on the screen" — and never traverse. No `for` head
mentions them. The zero-mention assertion is what found them.

One of those is not cosmetic: `win_rest_invalid()` no longer walks, so it
dereferences its argument unconditionally, and the two surviving
`win_rest_invalid(wp->w_next)` calls would have passed NULL and crashed.

## A blind spot the sweep does not cover

Folding `for ((tp) = first_tabpage; …)` into `tp = curtab;` leaves a variable nothing
reads, which gcc reports as `-Wunused-but-set-variable` — and `deadsweep.py` handles
`unused-variable` and `unused-function` and **nothing else**. Eight functions were
left that way, which is what the sweep's persistent "left alone 8" meant.
`check_changed_any` and `min_rows_for_all_tabpages` still read their `tp`, so theirs
stay; `changed_common` had three assignments, not one.

The first count of five came from reading a gcc list I had truncated at twenty lines.
The full list is eight.

## What stays, deliberately

The **frame layer** — `topframe`, `frame_T`, `fr_next`, `fr_child`, `fr_parent`. One
window still has one frame and sizing needs it; cutting frames is its own phase.
**`b_nwindows`**: tracing every write, it is 1 at creation, balanced `++`/`--` in
`enter_buffer` and `aucmd_restbuf`, and `--` in `close_buffer` — genuinely 0 once the
window drops the buffer, so `<= 0` and `== 0` are live "not displayed" tests. An
earlier plan folded all 20 sites to a constant 1; that would have broken buffer
release silently. **`prevwin` and `w_id`**, which `aucmd_prepbuf`/`aucmd_restbuf` and
the incsearch state use, are not list state.

## The delta

**None**, and `whimdelta.sh` confirms it. Every window and tabpage Ex command was
already `ex_ni`.

**A third probe that could not fail.** The autocommand probe was written
`+autocmd BufWritePre * normal! A-au` — and `:autocmd`, `:augroup`, `:doautocmd` and
`:doautoall` are all `ex_ni` here, so it registered nothing and fired on no build.
Calibrated against q71: the same answer as this phase gives. After `<LeftMouse>` and
`normal!`, the rule that catches all three is now written into the script:
**calibrate a new probe against the previous boundary before trusting it**, which
costs one build. The replacements — a write that completes through `getout()`'s
rewritten BUFWINLEAVE block, and an `O` that exercises the folded scroll path — were
calibrated that way and pass on q71.

Measured: 92,749 → **92,110 lines**.
