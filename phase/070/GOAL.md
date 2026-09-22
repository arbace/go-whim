# Phase 70 — :e reloads in place, and there is no swap file

**This invariant is imposed, not proved**, and that is the difference between it and
phase 68. One window fell out of the two places a window could be created. A second
*buffer* is genuinely reachable: `curbuf_reusable()` wants an unnamed, empty buffer,
so once the first file is named, `:e other` allocates a new `buf_T` and switches to
it. Measured on q69: `:e h2.txt` then `+wq h1.txt` writes **h2**.

So `do_ecmd()` is made to reuse the one buffer:

- the `other_file` branch renames `curbuf` with `setfname()` instead of calling
  `buflist_new()`, sets `oldbuf = FALSE`, and falls through;
- the reload path below it — `u_sync()`, `u_savecommon()`,
  `buf_freeall(curbuf, BFA_KEEP_UNDO)`, then `open_buffer(… READ_KEEP_UNDO)` —
  **already is** "wipe and re-read in place". Its gate widens from
  `!other_file && !oldbuf` to `!oldbuf`;
- the whole `if (buf != curbuf)` block goes: BufLeave, `buf_copy_options`, `u_sync`,
  `close_buffer(DOBUF_WIPE)`, the `auto_buf` dance, the `curwin->w_buffer` swap and
  `get_winopts`. It is removed by **brace matching**, not by matching its body —
  the body is long and macro-expanded, and three runs of an abandoned phase died on
  patterns transcribed from truncated views of exactly such lines.

**Order: `fname2fnum()` first.** It called `buflist_new(name, p, 1, 0)` to give a
file mark's file a buffer, and once reuse is unconditional that call would wipe the
buffer being edited. It is **folded to an empty body, not removed** —
`getmark_buf_fnum()` still calls it, and the file marks are a separate cut. An empty
shell with a live caller is the fold, not a leftover, so the check asserts that its
body can no longer reach `buflist_new()` rather than that the symbol is gone. A
first version demanded zero mentions and failed on its own terms.

**What is lost:** the state of the file you leave — its undo history and its marks.
`:e`, `:e!` and `:wq` keep working, on one buffer.

## No swap file, ever — not even one left from another age

Swap files are already never *written* here: `findswapname`, `p_swf`,
`swapfile_info`, `swapfile_unchanged`, `ml_recover` and `ml_sync_all` went with the
recovery phase, `mf_open()` is the in-memory memfile, and `ml_open_file()` had been
reduced to a single `b_may_swap = FALSE`.

What survived was the **detection** half — the prompt for a swap file somebody else
left behind — and it was already unreachable. Measured on q69 with a `.swp` sitting
beside the file: **no prompt at all**, no stderr, the edit and the write going
through in silence. This is the `can_cindent` shape again, a flag written in three
places and never once true, and the compiler cannot say so because assigning to a
static counts as using it.

So the whole surface goes together: `swap_exists_action`, the three `SEA_*` actions,
`handle_swap_exists()`, `check_swap_exists_action()`, `check_need_swap()`,
`ml_open_file()` and the `b_may_swap` field, with the `SEA_DIALOG` setters in
`do_ecmd`, `read_stdin` and `create_windows` and the three `SEA_QUIT` tests that
could never fire.

One of those tests is worth naming because it is not where it looks like it should
be: the *first change to a buffer* used to open its swap file, and that test lives in
`changed()`, not in `buf_write()`. Writing the host function from memory got it
wrong, and reading line 8756 got it right.

The three enums the sweep reports as "every constant is dead and the type is in use,
which cannot be expressed" are **inherited, not made here** — pristine q69 reports
the same three.

## The delta

**None.** `:e` prints nothing to stderr, and an `exsweep` row is `exit= left= err=`
— the same reason `:next` did not move in phase 69. Removing the swap surface moves
nothing either, for the same reason and one more: the prompt it removes was already
never reached. The probes are load-first and **quote-free**: `+$` then `+s/^/LAST /`
proves the buffer holds the file; then `:e h2.txt` leaving h1 untouched and writing
h2; plain editing; and `:e!` discarding an unwritten change. No probe key contains
`'`, which is what made an abandoned phase's mark probes measure nothing three times
over.

Measured: 93,393 → **93,127 lines**.
