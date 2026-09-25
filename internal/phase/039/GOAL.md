# Phase 39 — one window, always

The window list is the container the editor draws into, and `aucmd_prepbuf()`
still slots its hidden autocommand window into the frame tree beside the user's
with `win_split_ins()`. So **the list stays, with one user window in it**, and
every way to make, reach, resize, close or bind a second one goes.

Thirty-two rows go to `ex_ni`: `:split`, `:vsplit`, `:new`, `:vnew`, `:sview`,
`:close`, `:only`, `:resize`, `:wincmd`, `:windo`, `:syncbind`, `:hide`, `:sbuffer`,
`:sbNext`, `:sball`, `:sbfirst`, `:sblast`, `:sbmodified`, `:sbnext`,
`:sbprevious`, `:sbrewind`, `:ball`, `:unhide`, `:sunhide`, and the eight split
modifiers `:aboveleft`, `:leftabove`, `:belowright`, `:rightbelow`, `:topleft`,
`:botright`, `:vertical` and `:horizontal`. `tools/nowindows.py` removes what a row
cannot:

- **The modifiers** are matched by name in `parse_command_modifiers()` and were
  all that set `cmdmod.cmod_split`. **`:hide {cmd}` is a modifier too, and stays**
  — only bare `:hide`, which closed the window, is a row.
- **CTRL-W** points at `nv_error`, and `do_window()` goes with every window command
  behind it.
- **The command-line window is a split**, so it goes: `q:`, `q/` and `q?` are the
  recordings they would be without it, CTRL-F on the command line is an ordinary
  key, and every test of `cmdwin_type`, `cmdwin_win`, `cmdwin_buf` and
  `cmdwin_result` folds. `vgetorpeek()`'s `tc` remembered the previous key for one
  of those tests alone, and goes with it — the warning check caught it.
- **`-o` and `-O`** are unknown options, and startup opens no window per file:
  `create_windows()` loses its count and `edit_buffers()` its call.
  `tools/clicheck.py` still lists them as accepted, because it runs at Phase 3
  where they are; the phase checks they are refused, in its terms.
- **The paths that still split.** `:drop` split when the buffer could not be
  abandoned, and now does what `:first` does — refuses. `do_argfile()`'s `s`
  commands, `goto_buffer()`'s `:sb` family and `buflist_getfile()`'s
  `'switchbuf'` block fold.
- **`'scrollbind'`, `'cursorbind'`** bind one window to another, and
  **`'winfixbuf'`** is answered by splitting: their tests fold, their assignments
  and `get_varp()`/`copy_winopt()` plumbing go, and the rows go before the sweep.
  `'switchbuf'`, `'scrollopt'`, `'cmdwinheight'` and `'cedit'` lose their last
  reader here too — `'cedit'` via `didset_options()`, which a first run missed —
  and `'previewheight'` and `'previewwindow'` had none.

Left alone: `check_can_set_curbuf_disabled()` and `_forceit()` now always answer
yes and keep their nine callers, and `z{height}<CR>` still resizes the one window.

Checked in a terminal against `slim-vim`: after CTRL-W s, CTRL-W v or `q:`, `:q`
leaves the editor, where slim-vim stays open with the second window.

## The delta

**The twenty-seven rows that succeeded run bare**, read from the slim baseline.
`:close`, `:hide`, `:sbmodified`, `:wincmd` and `:windo` already failed.
Measured: 121,368 → **118,516 lines**, libc symbols 88 → 88.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 40, in the group 39-40, whose phases share one purpose. There is no boundary q039 of its own any more; everything above still says what the steps do and why.
