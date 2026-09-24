# Phase 13 — the editor stops re-reading a file it has already read

vim watches the files it holds. `check_timestamps()` walks every buffer and
stats its file — from the main loop, from insert mode, from the `Press ENTER`
prompt, and whenever the terminal regains focus — and `buf_check_timestamp()`
does the same for one buffer on entering it. If the file moved underneath it
prompts, and with `'autoread'` it reloads.

That is the editor initiating filesystem traffic on its own account, which is
the boundary this fork narrows. **Phase 11 retired `:checktime`, which removed
the command; this removes the polling, which is what actually reached the
disk.** What is left is an editor that reads a file when told to and writes it
when told to.

`check_timestamps()` returns 0 without looking at anything, and its four callers
are left calling it. Stubbing rather than unpicking them is deliberate: each
sits in a different control structure and each already handles that answer. The
three direct `buf_check_timestamp()` calls — in `do_ecmd()`, `enter_buffer()`
and `ex_drop()` — are deleted, because with the poll gone they are the only
thing keeping 339 lines of checking and reloading alive.

**`check_mtime()` stays.** `buf_write()` calls it before overwriting a file that
changed since it was read, and that is not polling: it happens only when the
user asks to write, and it is what stops a write silently clobbering someone
else's edit. `b_mtime_read` is still recorded on read, so it still works.

`'autoread'` cannot go — `PV_BOTH`, and its row is what initialises the global.
It stays, and now decides nothing.

## The delta

**None the harness records.** Nothing it does changes a file behind the editor's
back, so nothing it does reaches this code — which is worth stating rather than
glossing, because a phase with no delta is either well-chosen or untested, and
the only way to tell them apart is to say which you think it is.
