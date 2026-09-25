# Phase 40 — no window sizes to set

With one window, `'winheight'`, `'winminheight'`, `'winwidth'`, `'winminwidth'`,
`'helpheight'`, `'splitbelow'`, `'splitright'`, `'splitkeep'`, `'equalalways'`,
`'eadirection'`, `'winfixheight'` and `'winfixwidth'` have nothing to decide. The
rows go. **The values do not**, and that is the whole difficulty of the phase.

The frame arithmetic still runs — `aucmd_prepbuf()` inserts its hidden window
with `win_split_ins()` and `win_close()` takes it out — and it reads the size
globals as it goes. A row is what writes a default into its global (see
`tools/orphanopts.py`), so dropping one alone would leave `p_wmh` at 0 and
`p_spk` NULL. **`tools/nowinsizes.py` gives each global its default as an
initialiser of its own** before the rows go: `FALSE`, `FALSE`, `"cursor"`, `TRUE`,
`"both"`, `1`, `1`, `20`, `1`. The value is exactly what the defaults gave, and
nothing can change it; the arithmetic keeps its own temporary writes, which a
variable allows as well as an option. `orphanopts.py` accepts an initialised
global by construction.

The two window-local fields are never set now, so their tests fold instead:
`win_split_ins()` keeping a fixed size, `winframe_remove()` passing over one,
`frame_setheight()`/`frame_setwidth()` reserving room for one, `win_enter_ext()`
sparing one, and `command_height()`'s loop over fixed-height frames, which never
runs. `frame_fixed_height()` and `frame_fixed_width()` answer `FALSE` for a window
and keep their recursive callers. `'helpheight'` had no reader but the callback
it shared with `'winheight'`, and the sweep takes both.

## The delta

**None the Ex sweep records beyond Phase 39's** — no row is retired. Each of the
twelve names is refused by `:set` now, which the phase probes against a
`:set ignorecase` control. Measured: 118,516 → **118,130 lines**, libc symbols
88 → 88.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 39-40 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).
