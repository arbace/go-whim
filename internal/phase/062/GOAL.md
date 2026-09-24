# Phase 62 — no buffer-type, file-type, listing, jump, update-time or autowrite options

Seven options, each checked by what its readers still did:

- **`'buflisted'`** — every reader chose which autocommand event to fire, and
  `apply_autocmds_group()` has been `return FALSE` since autocommands went, or
  searched a buffer list of one. The `set_buflisted()` calls go with it.
- **`'filetype'`** — every reader fed the FileType event, which cannot fire, or
  `fix_help_buffer()`, and `:help` is `ex_ni`.
- **`'buftype'`** — the one that was live, but only through `:set bt=`: `nofile`,
  `nowrite`, `acwrite` and `prompt` refused `:w` and skipped reading, and `help` set
  `b_help`. Nothing inside the editor ever set it. `bt_dontwrite()`,
  `bt_nofilename()`, `bt_nofileread()` and `bt_prompt()` fold as false at every
  caller — including two inside one `snprintf` line in `fileinfo()`, the
  `[Not edited]` and `[New]` notes, and `buf_write()`'s `nofile_err`, set in three
  branches and read in two tests and one condition.
- **`'jumpoptions'`** — empty, so the "stack" behaviour of the jump list folds.
- **`'updatetime'`** — **dead, though it looked live.** After that long idle,
  `inchar_loop()` asked `trigger_cursorhold()`, which is `return FALSE`, and called
  `before_blocking()`. Its swap sync, `updatescript(0)`, reaches an `ml_sync_all()`
  whose body is empty; its terminal flush only writes while `sync_output_state` is
  above zero, which is inside `update_screen()` or `redraw_after_callback()` — both
  close it before returning, with no `return` or `goto` in between — so never at
  idle. The idle wait therefore goes: a wait with no timeout blocks at once, and
  `before_blocking()`, `updatescript()`, `ml_sync_all()` and the `scriptout`
  save and restore in `wait_return()` go with it. A first version of this phase
  kept the timeout at its 4000 ms default, on the strength of the call alone; it
  was corrected in place when the chain was read to the end.
- **`'autowrite'`** and **`'autowriteall'`** — off, so `autowrite()` always failed and
  `autowrite_all()` returned at once. Their callers fold, and so does the `CCGD_AW`
  flag — including the two places, `:next` and `do_argfile()`, that passed it
  unconditionally.

**The phase took six runs to get right, and every failure was the post-condition
grep or a tool refusing, never the build.** `droplocal.py` refused `b_p_bl` with
two plumbing sites where it requires three — the folds had already taken its
initialiser, so its field and `get_varp()` case go by hand — and then refused
`b_p_bt` because `fileinfo()` still called `bt_dontwrite()` a second time. The
grep then found `CCGD_AW` and `nofile_err` alive, and — once the idle wait was
removed — `did_start_blocking`, still read by the loop's exit test. That one needed
thought rather than deletion: blocking now starts on the first wait with no
timeout, so the flag was always TRUE where it was tested, and the term goes so that
an interrupted wait still returns instead of blocking again. Each was a reader the
first reading had missed, not a reader the check invented.

## The delta

**None the harnesses record.** Measured: 101,188 → **100,643 lines**.
