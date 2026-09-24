# Phase 68 — one window, structurally

**This phase establishes an invariant and then spends it**, which is why it is the
largest cut here since the early ones.

A window is created in exactly two places: `win_alloc_firstwin()`, once at startup,
and `win_split_ins()`, whose **only** caller is `aucmd_prepbuf()`. `win_split()`,
`make_windows()` and `win_new_tabpage()` have no mentions at all. A tabpage is
created once, by `alloc_tabpage()` in `win_alloc_first()`. So removing the
autocommand window means nothing can ever add a window or a tabpage again:

```
    firstwin == lastwin        first_tabpage->tp_next == NULL
```

`one_window()`, `last_window()` and `only_one_window()` are then constant TRUE —
not as an observation about the harness, but as a consequence of the two creation
sites — and every caller folds.

**Why the autocommand window can go.** `aucmd_prepbuf()` splits one open only when
no window shows the buffer, in order to run autocommands in it — and
`apply_autocmds_group()` has been `return FALSE` since autocommands went. The
window was built to run nothing.

**What was already a no-op**, which is why this removes capability from the source
and none from the editor:

- `win_close()` tests `last_window()` first and answers *cannot close last window*,
  so the calls in `ex_quit()`, `ex_exit()` and `do_exedit()` could never close
  anything — and the first two reach `getout(0)` before them regardless.
- `do_exedit()`'s call is guarded by `old_curwin != NULL`, and its one caller
  passes `NULL`.
- `close_windows()` loops `wp != NULL && !(firstwin == lastwin)`, false at once,
  then over tabpages other than `curtab`, of which there are none.

Gone with them: `win_split_ins`, `win_close`, `close_windows`, `win_close_othertab`,
`close_last_window_tabpage`, `close_tabpage`, `free_tabpage`, `winframe_remove`,
`win_equal`, `win_equal_rec`, `frame2win`, `win_altframe`, `is_aucmd_win`, the four
snapshot functions, `win_alloc_popup_win`, `win_init_popup_win` and the `aucmd_win[]`
table.

**What stays, and is checked rather than assumed.** `win_comp_pos()`,
`frame_comp_pos()`, `last_status()` and `last_status_rec()` are reached from
`shell_new_rows()` and `did_set_laststatus()`, so a terminal resize and
`:set laststatus` still compute the one window's geometry. The frame code does not
vanish wholesale.

**Four failures, and two of them were the kind that ship.**

1. **A splice that would have compiled.** The first version cut everything from the
   `aucmd_win[]` search through `curbuf = buf;` — which also swallowed
   `aco->save_curwin_id` and `aco->save_prevwin_id`, the two fields
   `aucmd_restbuf()`'s surviving branch reads back through `win_find_by_id()`. It
   would have built cleanly and restored from uninitialised stack. A count check on
   an unrelated line is what stopped it; the cut is now two narrow splices.
2. **`drop_if` refused an `else`, correctly.** `aucmd_restbuf()`'s
   `if (aco->use_aucmd_win_idx >= 0)` has one, and deleting the `if` alone would
   orphan it. `fold_never` is the helper for that shape.
3. **Three places managed the table without reading it** — `autocmd_init()`, whose
   whole body was a `memset` of it, and two loops in `screenalloc()` freeing and
   reallocating line sizes for windows that can no longer exist. The `can_cindent`
   shape from phase 64, found by the post-condition grep rather than by any warning.
4. **The must-go list contradicted the phase's own header.** It demanded
   `last_status_rec` reach zero mentions while the header said `last_status()`
   stays. The check was wrong, not the tree.

## The delta

**None.** No key, command or option changes. Measured: 96,636 → **94,122 lines**,
the largest single phase since the early cuts.
