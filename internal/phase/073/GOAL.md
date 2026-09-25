# Phase 73 — one frame

**The strongest invariant of this run, and it is proved by absence.** Grepping the
whole file for a write to `fr_child`, `fr_next`, `fr_prev` or `fr_parent` returns
**nothing at all**. The frame tree is never linked:

- `alloc_clear(sizeof(frame_T))` appears exactly once, in `new_frame()`, whose only
  caller is `win_alloc_firstwin()` — itself called once, from `win_alloc_first()`;
- `new_frame()` writes `fr_layout = FR_LEAF` and `fr_win = wp`, and nothing else ever
  writes `fr_layout`;
- `win_alloc_firstwin()` sets `topframe = curwin->w_frame`;
- there is no `frame_insert`, `frame_append`, `frame_remove`, `win_split` or
  `win_split_ins` anywhere — they went with the window layout in phases 68 and 72.

So `topframe == curwin->w_frame`, `fr_layout` is `FR_LEAF` forever, and the four tree
pointers are permanently NULL. Every `FR_ROW`/`FR_COL` branch is dead, every
`fr_child` walk iterates zero times, and every `fr_parent` walk stops on its first
test.

That makes this phase a set of **body replacements** rather than a fold campaign —
fifteen of them. Each function keeps the arm that runs and loses the arms that
cannot: `frame_fixed_height`/`frame_fixed_width` → `FALSE`; the minima keep their
leaf arm; `frame_check_height`/`frame_check_width` compare one frame;
`frame_comp_pos` keeps the `fr_win != NULL` arm; `frame_new_height` keeps the
cmdheight adjustment and `win_new_height`; `frame_new_width` clears `w_vsep_width`
and calls `win_new_width`; `frame_setheight` keeps the root arm and `frame_setwidth`
returns; `frame_add_height` loses the parent walk; `last_status_rec` keeps `FR_LEAF`;
`command_height` loses the walk to the widest ancestor; `stl_connected` → `FALSE`.

**The invariant is asserted in the phase, not just in this document.** The script
greps for a write to any tree pointer and dies if it finds one, so if an upstream
ever links a frame again this fails loudly instead of producing an editor that
silently mis-sizes its one window.

**The break hazard was handled by construction.** The audit named four loops whose
`break` binds to the loop being removed — `stl_connected`, `frame_new_height`,
`frame_new_width` (twice) and `command_height` — and all four were already in the
replace-whole-body set, so nothing was folded out from under a `break`. This is the
phase 71 lesson applied ahead of time, and it is why **this phase passed its first
dry run**, the only one in this run that did.

## What is not a constant

`frame_minheight()` reads `p_wh`, `p_wmh` and `w_status_height`, and `min_rows()` and
`did_set_cmdheight()`'s clamp both depend on the number it returns — so the leaf arm
keeps its arithmetic exactly and only the recursion goes. Replacing it with a literal
would silently change what `:set cmdheight=` accepts, which no probe here would have
caught. A post-condition asserts `p_wh` and `p_wmh` still appear in its body.

`fr_width` and `fr_height` stay: they are live layout state, read by `win_do_lines`,
`screen_ins_lines`, `screen_del_lines`, `redraw_block`, `screen_line`, `win_line` and
`did_set_cmdheight`. The **fields** stay; only the tree goes.

`frame_fixed_height`, `frame_fixed_width`, `frame_minwidth` and `frame_fix_height`
fall out by cascade once their callers' `wfh`/`wfw` loops vanish, and `FR_ROW` and
`FR_COL` lose every reader. Nothing deletes those by name.

## The delta

**None**, and `whimdelta.sh` confirmed it. Every splitting and resizing Ex command is
already `ex_ni`, and `:set cmdheight=` keeps its accepted range because
`frame_minheight` kept its arithmetic.

Two new probes drive the rewritten bodies — `:set cmdheight=2` through
`did_set_cmdheight` → `command_height` → `frame_add_height`, and `:set laststatus=2`
through `last_status` → `last_status_rec` — and **both were calibrated against q72
before being trusted**, which is the rule three earlier probes in this run were
written in violation of.

Measured: 92,110 → **91,329 lines**.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 72-73 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).
