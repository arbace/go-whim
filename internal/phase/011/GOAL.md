# Phase 11 — nothing is written that was not asked for

A swap file is not a recovery add-on bolted to the side of the editor. It is
**memline's backing store**: created beside every file you open, written to as
you type, deleted on a clean exit. For an embedded editor it is the last thing
writing a file nobody asked for, and it is why `'directory'` is searched for a
free `.swp` name and why a 576-line recovery reader exists.

**What goes is the file, not the memline.** `mf_open()` already supports a
memfile with no name — that is what `:set noswapfile` has always produced — so
the buffer keeps its block structure and never acquires a fd. The cost is real
and was agreed before any of it was written: **no crash recovery**, and a buffer
larger than memory can no longer page out to disk.

Five entry points, because `ml_open_file()` has seven callers and no-oping them
one at a time would be seven chances to miss one. `ml_open_file()` returns
having set `b_may_swap = FALSE`, so the callers that retry stop retrying — a
body that merely returned would search `'directory'` again on the next
keystroke. `ml_preserve()`, `ml_sync_all()` and `ml_setname()` become no-ops:
flushing, syncing and renaming a file that does not exist. And the `SEA_RECOVER`
arm of the ATTENTION prompt goes, which is the only way into `ml_recover()` once
`:recover` is retired.

With it go the two other things that wrote without being asked: `:mkvimrc`,
`:mkexrc`, `:mksession` and `:mkview`, which drop a script into the current
directory, and `:checktime`.

## The check this phase exists for

No build can make it, so the phase runs the binary: **edit a file in an empty
directory and nothing may be left beside it.** `ls -A` must show exactly the
file that was edited.

## Not done here

**The automatic timestamp check remains**, for now. `check_timestamps()` is
still called from `main_loop()`, `edit()` and `wait_return()`, so the editor
still notices a file changing underneath it — retiring `:checktime` removed the
command, not the polling. That is a separate cut with a separate delta, and
**Phase 13 is where it happens**.

## The delta, and three things the harness knew better than the author

Eight command names report that they are not available; `'updatecount'` and
`'swapsync'` stop existing. `'swapfile'` cannot go — it is
`PV_BUF` and its row is what initialises the global, the trap Phase 10 records —
so it stays and is now always effectively off.

## `'directory'` was dropped here once, and that was a bug

It is in the list above no longer, and the correction is worth more than the
line it takes. A row is also what **initialises** its global, so a row can only
go once nothing reads the global — and `recover_names()` scans every directory
in `p_dir` looking for swap files, right up until Phase 21 deletes it. Dropping
the row here left `p_dir` NULL for ever, with a live dereference in
`check_overwrite()`, which asks whether *another* vim has a swap file beside the
file you are about to overwrite. So this shipped for twelve phases:

```
:w! <an existing other file>      ->      Segmentation fault
```

**Nothing saw it, and each reason is worth knowing.** The build is clean. The
dead-code sweep is silent, because an orphaned global is *used* — no
unused-variable warning names it. The linkage and symbol checks pass. The Ex
sweep runs every command from its own scratch directory, where the target does
not exist; the 67 behaviour cases write to the file they opened; and neither
writes over an existing file *under a different name* with `!`, which is the one
shape that reaches it.

`dropoptions.py --strict` refuses exactly this and did not exist when this phase
was written. The repair has three parts, and only the first is about this bug:

1. `'directory'` moves to Phase 21, where its last reader goes. Phase 11 keeps
   the row, so `p_dir` is initialised for every phase in between.
2. `tools/orphanopts.py` runs in **every** whim phase, out of `whimdelta.sh`. It
   parses `options[]`, collects every `&p_xx` it names, and compares that with
   every `p_xx` declared at file scope. It is type-aware, which is the whole
   trick: a `long` orphan reads as 0 and is reported, a `char_u *` orphan is
   fatal. `'updatecount'` is genuinely safe to drop here for that reason —
   `p_uc` reading 0 *is* "never create a swap file".
3. `--strict` learned that `varp == (char_u *)&p_x` takes an address rather than
   reading a value. Counting those made it refuse `'directory'` in Phase 21,
   where the row genuinely was inert — a guard that cries wolf gets turned off,
   which would have cost more than the bug did.

`mf_sync()`'s `MFS_FLUSH` tail goes here too, as the last reader of `p_sws`, and
takes `sync()` with it. It sat behind `if (mfp->mf_fd < 0) return FAIL;` and so
was never reached — latent rather than live, and removed for the same reason.

`:mksession` and `:mkview` **do not move**: they already failed. And `:recover`
**leaves** the cumulative list it joined in Phase 7 — removing globbing had
made it fail differently from the slim baseline, and `ex_ni` makes it fail the
same way again, so it stops being a difference. A cumulative delta can shrink,
which is not something a list maintained by hand would ever discover.
