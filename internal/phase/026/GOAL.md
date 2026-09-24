# Phase 26 — five signals, not twenty-one

`signal_info[]` had twenty-one entries and five handlers. Reviewed one at a
time, four earn their keep.

| kept | why |
| --- | --- |
| `SIGWINCH` | `sig_winch()` sets `do_resize`, read in nine places. Without it the editor never learns the terminal changed size. |
| `SIGINT` | `catch_sigint()` sets `got_int` — **read in 222 places**. That number is the argument: `got_int` is how every long operation is interruptible. Without the handler, CTRL-C reverts to its default action, which kills the process and loses the buffer, turning "stop that" into "lose your work". |
| `SIGTSTP` | CTRL-Z and `:suspend`, through `sig_tstp()` and `got_tstp`. The only caller of `raise()`. |
| `SIGHUP`, `SIGTERM` | reaching `deathtrap()`, so that a killed editor **puts the terminal back**. |

Sixteen entries and three handlers go: `SIGPWR`, whose handler called
`ml_sync_all()` — **an empty function** since Phase 11; `SIGUSR1`, whose flag
**nothing reads** (assigned and never examined, so `-Wunused-variable` never
fires and the sweep would never have found it); and `SIGQUIT`, `SIGILL`,
`SIGTRAP`, `SIGABRT`, `SIGFPE`, `SIGBUS`, `SIGSEGV`, `SIGSYS`, `SIGALRM`,
`SIGVTALRM`, `SIGPROF`, `SIGXCPU`, `SIGXFSZ`, `SIGUSR2`, `SIGPIPE`. With them go
`sigaltstack` and its stack — which existed so a SIGSEGV from stack overflow
could still run a handler, and SEGV no longer reaches one — and
`may_core_dump()`, which re-raises to produce a core there is nobody to read.

Eight signal names are left in the file: the five kept, plus `SIGCONT`,
`SIGALRM` and `SIGPIPE`, which `mch_suspend()` sets around the stop. The phase
asserts exactly that list.

**The cost, decided deliberately: a crash no longer restores the terminal.**
`SIGSEGV` and `SIGBUS` take their default action. The alternative is keeping a
handler for conditions this editor should not have, to tidy up after a bug that
should not exist.

## The reason to keep `SIGHUP` and `SIGTERM` was not true until this phase

This is the part worth recording, because the phase was written on a claim that
turned out to be false and the check is what caught it.

`deathtrap()` reaches `preserve_exit()` → `prepare_to_exit()`, which calls
`settmode(TMODE_COOK)` to put the terminal back. And:

```c
settmode(tmode_T tmode)
{
    if (!full_screen)
        return;
```

— while `deathtrap()` sets `full_screen = FALSE` several lines before it gets
there. So the editor printed `Vim: Caught deadly signal TERM`, emitted
`stoptermcap()`'s escapes, exited, and **left the terminal with `ICANON` and
`ECHO` off**. The shell that got it back was unusable; the user had to type
`reset` blind. Measured on the slave side of a pty, before and after the cut:
identical, and wrong both times. Upstream has the same hole.

The fix is additive, so the ordinary exit path is untouched: `full_screen` is
lent for the length of the call. The guard exists to avoid drawing on a screen
that is not there, and putting the terminal back is not drawing.

`mch_settmode()` would have been the more direct call and is not available —
it is defined 89,000 lines further down and `SLIM-GOAL.md` Phase 10 removed the
forward declaration nothing needed.

## The check

`tools/termrestore.py` opens a pty, starts the editor on it, **verifies it
really entered raw mode** — otherwise the test would pass for the wrong reason,
on an editor that never changed anything — sends `SIGTERM`, and requires
`ICANON` and `ECHO` back on the slave side. No harness here kills an editor
halfway: the behaviour cases and the Ex sweep run it to completion and the pty
harness quits cleanly. This is the one thing the kept signals are for, so it is
checked in the phase.

## Where the symbol count moves

**90 → 88**: `sigaltstack`, `sysconf`. `raise` stays — `sig_tstp()` needs it —
and so does `kill`, whose three sites are `mch_suspend()`, the signal-blocking
helper, and `may_core_dump()`; only the last goes.

## The delta

**None the harness records.** The Ex sweep records `:suspend` and `:stop` as
*skipped* — they hand over the terminal — and `SIGTSTP` stays regardless.
