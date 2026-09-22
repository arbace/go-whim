# Phase 100 — the deadly ladder that cannot run

`phase/100/edit.go` and `phase/100/check.go`, `stage 100`, `package host`. Nine
lines, one libc symbol, and the smallest Part II phase so far. `deathtrap()` — the handler
for the deadly signals — opens with a ladder that counts how often it has been entered:

```c
    if (entered >= 3)
    {
        reset_signals();
        if (entered >= 4)
        {
            _exit(8);
        }
        exit(7);
    }
```

**`entered` cannot reach 3 in any build of whim-vim**, so this removes a *possibility*
and not a behaviour. That is phase 96's kind of cut rather than phase 92's: phase 92 took
code that had *become* unreachable, when phases 88 to 91 removed every way to name a file,
and this — like the two `FILE *` — was never reachable in anything the pipeline has ever
produced.

## Why it cannot run, and why the obvious reason is the wrong one

Two facts, and **neither is Part II's doing**:

* `catch_signals()` installs the deadly handler with `sigemptyset(&sa.sa_mask)` and
  **`sa.sa_flags = 0`**. No `SA_NODEFER`, so the signal being handled is blocked for the
  duration of its own handler.
* `signal_info[]` carries exactly **two** rows with `deadly = TRUE`, `SIGHUP` and
  `SIGTERM`. `SIGSEGV`, `SIGBUS`, `SIGILL` and `SIGFPE` are at **zero mentions** in
  `whim-vim.c` — whim removed all four — so there is no third deadly signal to arrive.

Two signals, each blocked inside its own handler, lets `entered` reach **2** — TERM
nested inside HUP's handler or the reverse, which is the `Vim: Double signal, exiting`
arm, and that arm calls `getout(1)` and never returns. It cannot reach 3: by then both
are blocked and nothing else is caught.

**The wrong reason is available and would be wrong elsewhere.** A phase that removed the
ladder because *"`reset_signals()` makes it unreachable"* would have the right answer for
the wrong reason — `reset_signals()` is **inside** the ladder and is never reached — and
would be wrong on any tree with three deadly signals. What the edit asserts, character for
character, is the two-row table and the `sa_flags = 0` arm, on the **output**, so that a
later phase cannot quietly falsify either without this check saying so.

## The argument is two binaries differing in one field

`phase/100/check.go` instruments the source the phase was **handed** —
`write(2, "DTn\n", 4)` immediately after `++entered;`, and `write(2, "DTLADDER\n", 9)` as
the first statement inside the ladder — and builds it five ways:

| | what it is | what it must report |
| --- | --- | --- |
| `in_mark` | the input, marked | **bombarded**: 8 concurrent sessions, 60 alternating SIGTERM/SIGHUP each at full speed. Every session reaches the handler, **the maximum `entered` ever observed is 2**, `DTLADDER` never appears |
| `in_forced` | plus `raise(SIGHUP)` in `preserve_exit()` and `raise(SIGTERM)` in the `entered == 2` arm | exactly `DT1 DT2`, no ladder, **exit 1** |
| `in_nodefer3` | the same source with **one field changed**, `sa.sa_flags = SA_NODEFER` | `DT1 DT2 DT3`, `DTLADDER`, **exit 7** — `exit(7)` running |
| `in_nodefer4` | plus one more forced signal at `entered == 3` | `DT1..DT4`, `DTLADDER`, **exit 8** — `_exit(8)` running |
| `out_forced` | **the output**, instrumented and forced identically | `DT1 DT2`, exit 1, and the same screen `in_forced` drew |

**`in_forced` and `in_nodefer3` are the whole phase in two binaries.** They differ in one
`sigaction` field and nothing else, and the ladder runs in one and not the other — so
both statements this phase deletes are live code that only the signal mask keeps out of
reach, and the bombardment's zero is a probe that is *proven* able to report otherwise.
That is phase 96's `ui_write()` control in this phase's shape, and it is stronger: the
control is not a different place in the program, it is the same place with the reason
removed.

The bombardment's assertion is deliberately **one-sided** — no session may report 3 or
more — so machine load can only ever weaken it, never make it fail spuriously. What load
*could* do is stop a session reaching the handler at all, which would make the zero
meaningless, so every session is required to have reported a depth. Measured over eight
runs: 8 of 8 sessions always reach it and between 2 and 8 of them reach depth 2.

## The stream is not comparable, and the screen is

The uninstrumented pair — the binary the phase was handed and the one it made — is sent a
single SIGTERM and a single SIGHUP, and the two must agree. **They cannot be compared as
byte streams.** The editor emits `\x1b[?4m`, a private mode that draws nothing, at a point
that depends on when its own flush happened: measured, `in_forced` and `out_forced` came
back 2,136 bytes each, byte-for-byte equal in three runs out of six and differing at
offset 2,003 in the other three, the whole difference being those five bytes sitting
before `\x1b[?25l` rather than after `\x1b[?25h`. The plain pair flaked the same way.

So the comparison is **what the editor drew** — `tools/zscreen.py`, Part II's instrument:
every snapshot, the final screen and the bell count, none of which a private mode touches
— beside the exit status and stderr, which are bytes and are compared as such. And the
screen is required to *carry* `Vim: Caught deadly signal TERM`/`HUP` and `Vim: Finished.`,
so the equality is not two blank screens agreeing. Six consecutive runs of the whole
phase were green.

**The harness waits on content, not on a clock**, for the same reason. A fixed sleep
signals the editor wherever its redraw had got to, and with sixteen sessions running at
once that is a recording of the machine's load; a quiet-for-300 ms drain was still wrong
once, on a startup that took longer than that to produce its first byte. The wait is for
the typed text to be on the screen *and then* for quiet, and a session that never gets
there is reported rather than compared.

## The counting trap

`exit` is a word this file uses for four things that are not a call. In the input:

```
    "Type  :qa!  and press <Enter> to abandon all changes and exit Vim"   a string
    "Type  :qa  and press <Enter> to exit Vim"                            a string
            exit(7);                                              this phase's
        exit(r);                                                  mch_exit's
                        goto exit;                                a goto, and
exit:                                                             its label, both
                                                                  in vim_regsub_both()
```

So **`assert exit at 0 mentions` fails on a correct phase**, and `assert 'exit(' at 0`
fails on `mch_exit(`, `preserve_exit(` and `getout(`. The assertion that works is
**`nm -u`** — the gone set is exactly `{_exit}`, nothing arrives, and `exit` is required
to be *still* undefined, being `mch_exit`'s and a later phase's. `_exit` as a word is
unambiguous, the label being `exit`, and it is asserted as well.

## Nothing is orphaned, and `reset_signals()` is the one that looks as though it should be

The ladder held one of `reset_signals()`'s four mentions; `mainerr()` holds another, so
the function stays and the check pins it at three. The sweep after the edit is a complete
no-op — 0 prototypes, 0 functions, 0 variables, 0 types, 0 fields, 0 enumerators, `canon
settled` — and `funcreach.py` reports 1,755 of 1,755 definitions reachable either side.

## The declared delta is nothing at all

`phase/100/delta` gets a comment and no line, and the reason is the statement. Five
phases now declare nothing and each for a different reason: **9** removed code that could
not run, **12** removed code that can run and that the instrument cannot see, **13**
removed the possibility, **16** changed no code at all, and **14** and **15** replaced
code with code that computes the same answers. **This one is 96's**: the ladder has never
been reachable in any build of whim-vim, so a recording that *moved* would mean the cut
was wrong. `tools/coredelta.sh --phase 100` finds the corpus unmoved, as it must.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 80,432 | **80,423** (−9) |
| functions | 1,755 | 1,755 — untouched |
| type definitions | 907 | 907 |
| DWARF enumerators | 1,181 | 1,181 |
| `nm -u`, as `phasecheck.sh` counts it | 34 | **33**, gone set exactly `{_exit}` |
| `nm -u` with the core's flags | 33 | **32** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| binary | 805,544 | **805,544 — the same size, different bytes** |
| sweep | | **1 round, a complete no-op** |
| phase | | **17 s** |

**Nine lines of code that never ran cost no bytes at all**, which is worth saying because
it is the opposite of what the line count suggests: the image is the same 805,544 and the
two binaries are not the same bytes, so what the ladder occupied was absorbed by alignment
padding. This phase's measure is the symbol, not the size.

## Its placement

`stage 100`, `package host` — new, and it is where `main()`'s demotion and `mch_exit`'s
one remaining `exit(r)` will go — with two `uses` lines, `exit:17 seed:83 mechanical` and
`exit:17 harness:86 mechanical`, which is what every phase declaring "nothing moved" owes.

**`need 100 swept` is not required**, and the anchors say why: every one is exact text at a
counted occurrence — the nine-line ladder, the six `exit` lines one by one, `signal_info[]`
and `catch_signals()`'s deadly arm — and none of it is text any sweep has ever touched.

**`apart 99 100`, measured.** Phase 99's check requires the file to have lost **exactly
eight** lines and this phase takes nine more: `tools/phaserun.sh 99-100` reports *"the
file lost 17 lines, expected 8"* and exits 1. Its libc check would fail as well — phase 99
states as a `cmp` that it frees **nothing**, inside a stage every check compares with the
*stage's* start, and this phase frees `_exit` — but the source assertions come first. It
is `apart 98 99`'s shape in **one** direction only: phase 100's own check passes on a 16-17
stage, phase 99 having freed nothing for it to be blamed for. And **no `apart 98 100`** is
written, although phase 98's check does require `_exit` to be still undefined: a stage
holding 98 and 100 holds 99, and `apart 98 99` forbids that already.

## What whim-vim is after seventeen phases

```
whim-vim.c        80,423 lines          from whim-vim.c's 86,614  (-6,191, 7.1%)
functions         1,755
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      32 with the core's flags, 33 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 32, attributed.** One row of the table moved and it is the last row of the host
boundary that is not a device:

| why | symbols |
| --- | --- |
| **the terminal** | `read` `write` `close` `dup` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` `isatty` (10) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals and exit** | `sigaction` `sigaddset` `sigemptyset` `sigismember` `sigprocmask` `kill` `raise` `getpid` `exit` (9) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

**`exit` is now a single call site**, `mch_exit`'s `exit(r);`, and that is what this phase
was for as much as the symbol: the next phase in this package has one line to replace in
one function rather than three in two.
