# Phase 67 — no mouse, no spell plumbing, no write-only flags

Three cuts, none of which changes what the editor can do, because none of it
could happen in the first place. This is the first phase driven by
`tools/coverage.sh` and by a scan for **write-only statics**, rather than by a
capability to remove.

- **The mouse, which cannot arrive.** There is no `'mouse'` option row, and
  `setmouse()`, `mch_setmouse()`, `mouse_has()` and `p_mouse` are all gone, so
  nothing ever asks a terminal to report mouse events. What served them goes:
  `is_mouse_key()` and the term in the input loop that called it,
  `reset_dragwin()`/`reset_held_button()` with `dragwin` and `held_button`,
  `mouse_row`/`mouse_col` and `old_mouse_row`/`old_mouse_col` — a save-and-restore
  pair nothing else reads — the 18 mouse rows of `key_names_table`, the `[MOUSE]`
  entry of the terminal string table, and `check_termcode()`'s mouse matching.
  **The 26 `nv_cmds` rows stay at `nv_error`**: that table's index is a permutation
  of its rows, so a removed row renumbers the keys after it.
- **The spell plumbing.** `spellvars_T` was one field, `win_line()`'s `spv`
  parameter was already `__attribute__((unused))`, and `win_update()` declared one
  on the stack only to pass its address twice.
- **Fourteen write-only statics.** `did_check_timestamps`, `was_safe`,
  `did_emsg_syntax`, `typebuf_was_empty`, `in_mch_delay`, `mr_patternlen`,
  `frame_locked`, `swap_exists_did_quit`, `did_swapwrite_msg`, `autocmd_nested`,
  `dragwin`, `held_button`, `oldtitle_outdated`, `deadly_signal`. Two were a whole
  function body, so `state_no_longer_safe()` and its two calls go with `was_safe`.

**`vim_ignored` is not one of them, though it looks identical to the detector.**
Its five sites are `vim_ignored = ftruncate(...)`, `= dup(2)` and
`= write(1, ...)`: it exists to swallow `warn_unused_result`, and removing it
*adds* warnings — a `(void)` cast does not silence that attribute in gcc. The
phase greps that it survives.

**One real change of behaviour is buried in the mouse cut.**
`looks_like_mouse_start` is not mouse-specific despite its name: it is set for any
two-byte `ESC [` termcode whose third byte is not a digit, and it *defers* the
match so a longer code — a mouse one — can win instead. With no mouse code able to
arrive, deferring can only lose, so the fold makes such a code match at once.
`tools/arrowcheck.py`, which drove a real pty, was what would have caught that
going wrong, until it was retired after Phase 82.

**Two failures, both in the phase's own counting, and both caught by a guard
rather than by the build.**

1. **A probe that could not fail.** It asserted `:map <LeftMouse> x` is refused
   once the name is gone. Measured on both binaries: **an unrecognised `<...>` is
   taken as a literal string, not refused** — `<Foo>` and `<ZZnotakey>` are
   accepted too. The evidence that the names are gone is the grep; what the probe
   checks now is that a name which *does* exist still maps.
2. **Thirteen of eighteen rows.** Five mouse rows — `DecMouse`, `JsbMouse`,
   `NetMouse`, `PtermMouse`, `UrxvtMouse` — are written across **three** lines
   (`{`, `FALSE,`, then code and name), so a single-line pattern could not see
   them. This is phase 54's wrapped-option-row trap again. Both patterns are
   anchored on the *name*, which is what keeps them off the sixth three-line row,
   `SNR`.

## The delta

**None.** No key, command or option changes — every cut is code nothing could
reach. Measured: 96,848 → **96,636 lines**.
