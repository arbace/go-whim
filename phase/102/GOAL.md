# Phase 102 — the core can no longer stop the process

`phase/102/edit.go` and `phase/102/check.go`, `stage 102`, `package host`.
Eighteen lines, one libc symbol, and `GOALS.md` §II.4c's second step. `mch_exit()`'s
last statement stops being `exit(r);`:

```c
static void (*vim_host_exit)(int);

    static void
mch_exit(int r)
{
    ...
    ml_close_all(TRUE);

    vim_host_exit(r);
}
```

`vim_main()` takes the callback as a third parameter and installs it as its first
statement, and phase 101's six-line launcher becomes twenty:

```c
static void *host_jump[5];
static int host_code;

    static void
host_exit(int r)
{
    host_code = r;
    __builtin_longjmp(host_jump, 1);
}

    int
main(int argc, char **argv)
{
    if (__builtin_setjmp(host_jump) != 0)
    {
        return host_code;
    }
    return vim_main(argc, argv, host_exit);
}
```

The editor no longer ends the process. It hands the process back, with a number.
**`nm -u` 32 → 31, the gone set exactly `{exit}`, and nothing arrives** — an indirect
call through a pointer names no symbol, and *returning* from `main()` ends the process
without naming one either.

## Why a function pointer, and why the other two routes are not available

* **Thread a status up through every caller.** Not expensive — *not writable*.
  `cmdnames[].cmd_func` is `void (*)(exarg_T *)` for all 98 rows and
  `nv_cmds[].cmd_func` is `void (*)(cmdarg_T *)` for all 194, each dispatched through
  one indirect call, so every handler would have to change signature together; and
  `deathtrap` is `void (*)(int)` by the kernel's contract and cannot participate at
  all. Say it plainly, because "thread the value up" is the first thing a reader
  proposes.
* **A `setjmp` in the core.** It puts the mechanism in the file that is meant to stop
  naming mechanisms, and it costs symbols.
* **The core calls out and does not come back.** `vim_host_exit(r);` is four words of C
  that say exactly that; the host decides *how*. It is the one route whose C text
  already says what a JVM host would have to do — an interface call whose
  implementation throws.

**The indirection is temporary and `GOALS.md` §II.4c says so.** It exists because
everything is still one translation unit and *nothing is global but `main()`* is still
the invariant: a pointer the launcher installs through a parameter adds no external
symbol, where a `musl_exit(int)` the host defines would. Once the file is split there
*is* a declared boundary, `vim_host_exit` becomes a plain `musl_exit(int)` prototype at
the top of the editor file, and the parameter and the pointer both go.

## The mechanism in the launcher is measured, and the review that proposed this phase got it wrong

Returning from `main()` is what ends the process without naming `exit`, and getting
back to `main()` from inside `deathtrap` needs a non-local jump. All four spellings,
measured on this tree:

| the launcher jumps with | `nm -u` | what it costs |
| --- | --- | --- |
| **`__builtin_setjmp`/`__builtin_longjmp`** | **32 → 31** | nothing arrives; no header |
| `sigsetjmp`/`siglongjmp` | 32 → **33** | `+sigsetjmp` `+siglongjmp`, `+<setjmp.h>` |
| `setjmp`/`longjmp` | 32 → **33** | `+setjmp` `+longjmp`, `+<setjmp.h>` |
| the launcher calls `exit(r)` | 32 → 32 | nothing moves; the phase achieves nothing |

**The two library spellings are net worse than not doing the phase**: `exit` leaves and
two symbols arrive in its place, plus a thirteenth `#include` in a file whose last
phase but two removed six. The review this phase comes from recommended `sigsetjmp`,
having counted the *core* at 42 with the launcher's cost attributed to a host file that
does not exist yet; in one translation unit there is no separate. So
`phase/102/check.go` **builds the `sigsetjmp` variant on every run** and requires
`nm -u` to show 33 against the output's 32, with `sigsetjmp` and `siglongjmp` present
and `exit` gone from both — the road not taken as a number rather than a memory. It is
compiled to an object; it is an answer, not a program.

## The one thing `sigsetjmp` would have bought, and the measurement that says it is not needed

`__builtin_longjmp` does not restore the process signal mask and `siglongjmp` does, so
after a jump out of `deathtrap` on SIGTERM the landing site still has SIGTERM blocked.
**That is exactly the state the process already died in.** Measured on the source this
phase was handed, with a `sigprocmask`/`sigismember` probe immediately before
`exit(r);`, and on the output with the identical probe immediately before the launcher
returns:

| | before `exit(r);` (input) | before `return host_code;` (output) |
| --- | --- | --- |
| SIGTERM | `TERM-MASKED HUP-CLEAR` | `TERM-MASKED HUP-CLEAR` |
| SIGHUP | `TERM-CLEAR HUP-CLEAR` | `TERM-CLEAR HUP-CLEAR` |

`exit()` was always being called from inside the handler with the handled signal
blocked. SIGHUP is clear only because `prepare_to_exit()` calls
`mch_signal(SIGHUP, SIG_IGN)`, which unblocks it on the way past. So the builtin
**preserves** the mask the process ends with and `siglongjmp` would have **changed**
it. The check asserts the pair every run, and requires the input's half to be non-empty
so the equality is not two silences agreeing.

A host that keeps running rather than returning is where the mask would matter, and
there is none: `main()` lands and returns four lines later. When the file is split that
host writes `musl_exit(int)` for itself and owns the question along with `sigprocmask`,
which is a symbol the *host* is allowed to name.

**One thing is deliberate and is said here rather than discovered later.** A non-local
jump out of a signal handler is undefined by the letter of C11 when the signal
interrupted a function that is not async-signal-safe, which here it always does —
`deathtrap` already calls `out_str`, `sprintf`, `ml_close_all` and `free`, and upstream
has always done that and got away with it because the process was about to die. The
design that removes it is the signal handlers becoming the host's — `sig_winch`'s
`do_resize = TRUE; return;` applied to the deadly two — which is a later phase and the
first of these with a real declared delta.

## The evidence

The six routes again, on both binaries, every one of which now leaves `mch_exit`
through the pointer, lands in `main()` and comes back as a **return value**:

```
quit=0   cquit3=3   eof=1   badopt=1   sigterm=1   sighup=1
```

**Proven able to fail**: the output built again with `host_exit`'s `host_code = r;`
made `host_code = r + 1;` — one character — moves all six, to 1, 4, 2, 2, 2, 2. That is
the value travelling from `mch_exit` through a function pointer, into a jump buffer and
out of `main()`, and the control is what says the table measures it.

**One finding the harness cost, and the program now records it.** The EOF row's status
is a statement about the harness's fd 2 as much as about the editor.
`fill_input_buf()` answers a read of nothing on a non-tty fd 0 with `close(0);
vim_ignored = dup(2);` and tries again — so the EOF route's *second* read is a read of
whatever stderr is. Measured on the binary this phase was handed: with stderr at
`/dev/null` the second read is another end of file, `read_error_exit` runs and the
status is 1; with stderr a **pipe the harness holds open**, the editor waits there for
keys that never come and the row times out on every binary. Both probes use
`/dev/null`. `tools/zstream.py`'s docstring has the same finding from the other end,
about `vim -`.

## The counting trap, one phase further on

`exit` as a word is **5** in the input and **4** in the output, and the number of
statements beginning with `exit(` is **1 → 0**. The four that stay are two string
literals (`"Type  :qa!  and press <Enter> to abandon all changes and exit Vim"` and its
shorter twin) and a `goto exit;` with its `exit:` label inside `vim_regsub_both()`. So
`assert exit at 0 mentions` fails on a correct phase, and `assert 'exit(' at 0` fails
on `mch_exit(`, `preserve_exit(`, `prepare_to_exit(`, `read_error_exit(`, `getout(` and
now `vim_host_exit(` and `host_exit(` as well. The assertion that works is `nm -u`, and
it is asserted in both directions: `exit` `_exit` `abort` `_Exit` `quick_exit` `atexit`
and every spelling of a jump must be **absent**.

## The declared delta is nothing at all

`phase/102/delta.md` gets a comment and no line, and it is phase 101's kind. Everything
`mch_exit` does before the changed line is untouched — the terminal restored, the
screen scrolled, the memfile closed — so what the editor *draws* on its way out cannot
move, and the recording is of what the editor draws. **18 and 102 are the first two
phases in this pipeline whose empty declaration means neither "nothing ran" nor "the
instrument cannot see it": the code runs, the instrument sees it, and it does the same
thing.** `tools/coredelta.sh --phase 102` finds the corpus unmoved: 102 of 102 screen
cases, 111 of 111 Ex-command rows, 30 of 30 command lines.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 80,428 | **80,446** (+18) |
| functions | 1,757 | **1,758** (`host_exit`) |
| type definitions | 907 | 907 |
| `nm -u`, as `phasecheck.sh` counts it | 33 | **32**, gone set exactly `{exit}` |
| `nm -u` with the core's flags | 32 | **31** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| binary | 805,544 | **805,544 — the same size, different bytes** |
| sweep | | **1 round, a complete no-op** |
| phase | | **24 s** |

## Its placement

`stage 102`, `package host` beside 100 and 101, with `uses host:102 seed:83 mechanical` and
`uses host:102 harness:86 mechanical`.

**`need 102 swept` is not required**: every anchor is exact text at a counted occurrence
— `mch_exit()`'s definition and its tail, `vim_main()`'s head, and the file's last six
lines — and none of it is text a sweep has ever touched.

**`apart 101 102`, measured, and the measurement found a better reason than the four that
were predicted.** Phase 101's check fails at its **first act**: it builds its own control
by rewriting `mch_exit`'s `exit(r);` to `exit(r + 1);` with `sed`, and refuses when
that changes nothing — and this phase has replaced that line. `tools/phaserun.sh zero
18-19` on q100 reports *"the control edit changed nothing — mch_exit's `exit(r);` is not
where this phase expects it"* and exits 1. Three more of its assertions would have
failed after it — the `cmp` on an undefined set that a stage snapshots once at its
start, the five-line gain where a 101-102 stage gains 23, and the six-line launcher it
requires the file to end with. **One direction only**: phase 102's own check compares
against the text *its* edit was handed and its gone set from q100 is still exactly
`exit`, 101 having freed nothing for it to be blamed for, so it passes on a 101-102 stage.

## What whim-vim is after nineteen phases

```
whim-vim.c        80,446 lines          from whim-vim.c's 86,614  (-6,168, 7.1%)
functions         1,758
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      31 with the core's flags, 32 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 31, attributed.** One row of the table lost its last member:

| why | symbols |
| --- | --- |
| **the terminal** | `read` `write` `close` `dup` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` `isatty` (10) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals** | `sigaction` `sigaddset` `sigemptyset` `sigismember` `sigprocmask` `kill` `raise` `getpid` (8) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

**The row used to be "signals and exit" and it is now "signals".** What is left of the
host boundary is a terminal, a clock, three allocations and eight signal calls — and
§4c's remaining step is the one that takes the first and the last of those out
together.
