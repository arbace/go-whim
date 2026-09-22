# Phase 21 — there is nothing to recover, and the memfile is memory

## there is nothing to recover

Phase 11 made the swap file memory-only: the block structure is still built,
still paged, still where every line of the buffer lives, but it never reaches a
disk. What that left behind is the other half of the feature — the code that
reads *someone else's* swap file back, which is code for reading a file this
editor cannot have written.

`-r` and `-L` are the only two things that ever set `recoverymode`, so the
global folds to FALSE and its seven readers each collapse to the branch they
were already taking. Three of them are in `readfile()`, which had to know
whether it was filling a buffer from a swap file rather than from the file
itself; the other four are the two `-r`-with-no-file arms, the stdin arm, and
the recovery arm of `create_windows()`. `ml_recover()` (559 lines),
`recover_names()` (216) and `swapfile_info()` (103) go with them.

**This is where `getpwuid` goes** — the fifth of the five password-database
symbols, and the one Phase 20 said would need a phase of its own.
`swapfile_info()` called `mch_get_uname()` to say who owned a swap file.

`:recover` was pointed at `ex_ni` earlier and does not move. It already failed,
needing a swap file to read — which is Rule 3's other half: retiring a command
only shows in the Ex sweep if it used to *succeed*.

### Time, which is the part that is a decision rather than a consequence

`swapfile_info()` was the only caller of `get_ctime()`, which left
`vim_localtime()` with exactly one user: `add_time()`, the timestamp in
`:undolist` and in `1 change; before #3`. It is dropped too, and **not because
it is unreachable**. `localtime_r()` asks libc what the local zone is, and
Phase 20 took away every way this editor could be told; a wall-clock time
without a zone is a wrong answer rather than a partial one. Undo history does
not outlive the process either — `:wundo` and `:rundo` have been `ex_ni` since
Phase 11 — so every time `add_time()` formats is within one session, and the
relative form it already used below 100 seconds is the true one. `strftime` and
both format strings go with it, and `:undolist` now reads `1 second ago` where
it used to read `14:23:07`.

### Where the symbol count moves

**110 → 107**: `getpwuid`, `localtime_r`, `strftime`.

### The delta

**None.** Verified by hand: `-r` is now `Unknown option argument: "-r"`,
`:undolist` prints `1 second ago`, and editing is untouched.

## the memfile is memory, and only memory

Phase 11 stopped the editor creating a swap file and Phase 21 stopped it reading
one back. What was left is a **file back-end with no file**: `memfile_T` still
carried a descriptor, still knew how to page a block out and read it in, and
still sized an LRU cache against how much memory the machine has — all of it
behind `if (mfp->mf_fd >= 0)`, and `mf_fd` could no longer be anything but −1.

The proof is short. `mf_open()` has two callers: `ml_open()` passes `(NULL, 0)`,
and `ml_recover()` passed a name — Phase 21 deleted it. Phase 11 stubbed
`ml_open_file()` to `b_may_swap = FALSE`. So nothing can hand the memfile a
name, `mf_do_open()` is unreachable, and `mf_write()` and `mf_read()` return
FAIL on their first lines.

Which makes **`'maxmem'` and `'maxmemtot'` options that decide nothing**:

```c
need_release = (mfp->mf_used_count >= mfp->mf_used_count_max
                || (total_mem_used >> 10) >= (long_u)p_mmt);
...
if (mfp->mf_fd < 0 || !need_release) { return NULL; }
```

`need_release` is the only place either is read, and the test in front of it is
always true — so the answer is computed and discarded. `mch_total_mem()` went to
real trouble to size that cache, through `sysinfo`, `sysconf` and `getrlimit`,
for a cache that never evicts.

Three more things fall out: `mch_get_host_name()`, which wrote the machine's
name into block zero so a recovering vim could say the swap file came from
elsewhere (**`uname`**); `lalloc()`'s retry loop, whose whole point was that
`mf_release_all()` might have freed memory by paging blocks to disk; and
`check_overwrite()`'s "swap file exists" warning.

**What does not change is the block structure.** Lines still live in blocks,
blocks still have numbers, `mf_trans` still maps them. This removes the ability
to *evict* a block, which was already impossible — not the ability to have one.

### A bug this phase fixes, and where it came from

`check_overwrite()` is the last reader of `p_dir`, so **`'directory'` can
finally go**. Phase 11 dropped its row while this still read it, and a row is
what initialises its global — so `p_dir` was NULL for ever, and

```
:w! <an existing other file>      ->      Segmentation fault
```

shipped for twelve phases. Nothing saw it. The build is clean; an orphaned
global is *used*, so no unused-variable warning names it; the linkage and symbol
checks pass; and neither the Ex sweep nor the 67 behaviour cases write over an
existing file under a different name with `!`.

`dropoptions.py --strict` refuses exactly this and had not been written when
Phase 11 was. The repair is in three parts: Phase 11 keeps `'directory'` and
drops it here instead; `tools/orphanopts.py` checks the invariant in **every**
whim phase, and is type-aware — a `long` orphan reads as 0 and is reported, a
`char_u *` orphan is fatal; and `--strict` learned that `varp == (char_u *)&p_x`
takes an address rather than reading a value, which is what made it refuse a row
that was genuinely inert.

### Where the symbol count moves

**107 → 104**: `sysinfo`, `getrlimit`, `uname`. `sysconf` stays — its other
caller is `_SC_SIGSTKSZ`, for `sigaltstack`.

### The delta

**None.** `:w!` over an existing other file stops crashing and writes it, which
is what it should always have done, and the phase asserts that directly — no
harness does.
