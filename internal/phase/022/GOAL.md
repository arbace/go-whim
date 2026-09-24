# Phase 22 — the working directory is where it started

`:cd`, `:lcd` and `:tcd` are `ex_ni`, `:!` no longer forks, and nothing else in
this editor moves the process. So **the directory it starts in is the one it
dies in**, and three pieces of machinery that exist because that was not true
stop being needed.

  * `mch_FullName()` chdir'd into the leading directory of a relative name,
    asked `getcwd()` where that had landed, and chdir'd back — via `fchdir()`
    on a descriptor it held open, falling back to `chdir()`. That dance is what
    resolved `..` and a symlinked directory on the way to a full name.
  * `win_fix_current_dir()` restores a window's or tab's local directory, and
    runs only when `w_localdir`, `tp_localdir` or `globaldir` is set. The first
    two come only from `:lcd` and `:tcd`; `globaldir` is assigned only inside
    this function. Unreachable.
  * `edit_buffers()` takes a `cwd` to return to between `-o` windows, and is
    passed `start_dir` — `static char_u *start_dir = NULL;`, which nothing
    assigns. The `-o` local-directory handling that set it is already gone.

## What it costs

A full name is now the working directory with the name appended, so `../x/y`
becomes `/cwd/../x/y` instead of `/real/x/y`. It opens the same file. What it
loses is that **two spellings of one path no longer compare equal**, so
`:e ../x/y` and `:e /real/x/y` are two buffers rather than one.

## The trap, and the harness that caught it

The first version of this dropped the dance and kept the rest of the function,
which reads `if ((force || !mch_isFullName(fname)) && ...)`. That condition is
true for an *absolute* name when `force` is set — harmless while the dance
existed, because the dance chdir'd to the name's own directory and `getcwd()`
came back with it. Without the dance, the working directory was prepended to a
name that already had one: `/tmp/x` became `/cwd//tmp/x`.

The delta check named it in one line — `:read`, `:write` and `:wq` moved, and
nothing else — which is the whole argument for declaring a delta in advance
rather than reading a diff afterwards. The fix is that `force` has nothing left
to re-resolve, so an absolute name is its own answer.

## `getcwd` stays, and is asked once

It has five callers through `mch_dirname()`: `shorten_fname1()` and
`shorten_fnames()` shorten every displayed name against it, `buf_modname()`
builds names from it, `modify_fname()` implements `%:p`, `fname2fnum()` resolves
a mark's file, and `mch_FullName()` is how a relative name becomes absolute at
all. Dropping it would mean `b_ffname` could not be a full path — a capability
cut rather than plumbing, and a different decision.

Since nothing can move the process, though, the answer cannot change. It is read
into a static on the first call and every later call is a copy: one syscall for
the life of the editor, where there used to be one per path operation.

## Where the symbol count moves

**103 → 101**: `chdir`, `fchdir`.

## The delta

**None the harness records** — and the phase adds a check of its own, because
none of them walks the path this changes: every harness edits a file in the
directory it is standing in. So it writes `sub/f.txt` from above and then
`../sub/f.txt` from inside `sub`, and requires the file to come back correct
both times.
