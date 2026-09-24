# Phase 103 — the signals and the terminal are the host's

`phase/103/edit.go` and `phase/103/check.go`, `stage 103`, `package host`.
`GOALS.md` §II.4c's third step, and it is **one** phase where the plan and two
surveys had two. The signals half on its own leaves the terminal in **raw mode** after
`kill -TERM`, because the only way to delete the core's signal handlers is to delete
`deathtrap`, and `deathtrap` is what restores it. Keeping `deathtrap` and having the
host merely *install* it costs nothing and keeps all of it:

```c
    static void
musl_host_init(void)
{
    host_catch(SIGHUP, deathtrap);
    host_catch(SIGTERM, deathtrap);
    host_catch(SIGWINCH, host_on_winch);
    host_catch(SIGCONT, host_on_winch);
    host_catch(SIGTSTP, host_on_tstp);
    host_catch(SIGINT, host_on_int);
    host_catch(SIGPIPE, SIG_IGN);
    host_catch(SIGALRM, SIG_IGN);
}
```

`deathtrap → preserve_exit → prepare_to_exit → term_leave()` still runs, and phase
102's `vim_host_exit` → `__builtin_longjmp` turns `mch_exit(1)` into `return 1`, so the
handler needs nothing of its own. Measured, identical on both binaries: `kill -TERM`
exits 1, restores the terminal to `ICANON=1 ECHO=1 ISIG=1 ONLCR=1 ICRNL=1`, and draws
`Vim: Caught deadly signal TERM` and `Vim: Finished.` in **2,241 bytes** — SIGHUP the
same in **2,240**. There is no phase 104.

## Within one translation unit, moving code frees nothing — and the phase says so

A symbol leaves `nm -u` when its last **caller** leaves the file, and that is the
split. `sigaction`, `sigemptyset`, `kill`, `ioctl`, `tcgetattr`, `tcsetattr`,
`nanosleep`, `select` and `__errno_location` all survive this phase; every one of them
is now called from a 229-line host block at the bottom of the same file and from
nowhere else. **What leaves is what the phase DELETES**, stated as one `comm` because
the two halves are one phase:

```
nm -u  31 → 24, gone exactly
    close  dup  isatty  raise  sigaddset  sigismember  sigprocmask
arrived: nothing
```

`raise` was `sig_tstp`'s re-raise. `sigaddset sigismember sigprocmask` were
`mch_signal()`'s fifty lines emulating `sigset()`, where the host's `host_catch()` is
four lines of `sigaction`. `isatty` is all three surviving calls. `close` and `dup` are
`mch_tcgetattr`'s dead `close(tty_fd)` and `fill_input_buf`'s `close(0); dup(2)` arm.

**So the phase's real claim is structural, and it ships the assertion for it.**
`tools/zhostonly.py` is new, Part II-only, and named by `phase/103/check.go` alone: 43
host words — `sigaction`, `kill`, `ioctl`, `tcsetattr`, `select`, `nanosleep`, every
`SIG*`, `struct termios`, `fd_set`, `ICANON`, `VMIN` — and **every mention of every one
of them must be inside the host block**. It reports 60 mentions of 43 words, all of
them there. Seven exceptions are named with their reason and their exact count, and
they are the whole of what the core still says: `signal_info[]` and `deathtrap` name
`SIGHUP` and `SIGTERM` because that is the **message** the editor prints,
`vim_handle_signal` re-raises a deferred deadly signal with `kill`, and `elapsed_T` is
`struct timeval` because it is the clock. It ignores string literals — the file
contains `hash_remove(&buf_hashtab, hi, "close buffer")`, and a tool that read that as
a `close()` would report the buffer layer as filesystem code — and `#` lines, because
`#include <errno.h>` and `#include <sys/ioctl.h>` name two of the words. It refuses to
pass vacuously: the host region must be found, must define all fourteen of its
functions, and must itself mention `sigaction ioctl tcsetattr nanosleep select kill`.
Without it the strongest available form of *the core names none of this* is
`sigaction` at 2 mentions, which says nothing about **where**.

## The in-band protocol was already in the file, and it could never run

This is the finding that reframed the phase. `handle_csi()` has always parsed DEC
private mode 2048's notification — `CSI 48 ; rows ; cols ; hpx ; wpx t`, specified by
Tim Culverhouse in 2024 — and carried the whole negotiation around it:
`win_resize_setting`, `win_resize_enabled`, `term_set_win_resize()`, the `'termresize'`
option with its `inband`/`sigwinch` values, and the `CSI ? 2048 h` / `l` it writes.

**And none of it could ever run.** `win_resize_setting` is written in exactly one place
in `whim-vim.c` — inside the parser for the DECRQM *response*, `CSI ? 2048 ; Ps $ y` —
and `\033[?2048$p`, the DECRQM **query** that is the only thing a terminal answers with
that response, appears **nowhere in `slim-vim.c`, `whim-vim.c` or `whim-vim.c`**. The
editor never asks, so it is never told, so `win_resize_setting` is 0 for ever,
so `term_set_win_resize(true)` always takes its first branch and sets
`win_resize_enabled = false`, so the notification arm is unreachable. Proven by running
it: typing `\x1b[48;30;100t` at the committed binary inserts the escape **as buffer
text**. So the phase deletes the negotiation, the option that drove it and the two
state variables, and switches the parser on — and the **host** writes the
notification, from its own SIGWINCH handler and its own `ioctl`.

That is the cheapest half of the phase, and it is half a deletion of code that could
not run rather than half a rewrite.

## Why the host keeps its `ioctl`, which is the decision the whole design turns on

Mode 2048 is a good protocol that almost nothing implements. Source-verified:

| implements 2048 | does **not** |
| --- | --- |
| ghostty, kitty, foot, iTerm2, contour, Bobcat | **xterm, tmux, GNU screen, WezTerm, Alacritty, VTE/gnome-terminal, Konsole, Windows Terminal, rxvt-unicode, st, Rio, zutty, xterm.js, Zellij, mintty, Terminator, mlterm, Warp, Hyper** |

**tmux and GNU screen terminate it**: their private-DECSET whitelists stop short of
2048, their DECRQM arms answer `CSI ? 2048 ; 0 $ y` — the correct *"not recognised"* —
and neither forwards `CSI ? 2048 h` outward. A core that relied on the terminal would
be an editor that never learns it was resized under tmux. The alternative query,
XTWINOPS `CSI 18 t`, is a **round trip** whose answer races real keystrokes in the same
stream, is unsupported by `st`, and can be switched off by xterm's `allowWindowOps` —
and it answers "what size am I", never "did it change".

So the host keeps `SIGWINCH` and `TIOCGWINSZ` and **injects** the notification the
spec defines. Deleting the host's `ioctl` instead would free `ioctl` and an eleventh
`#include` and cost exactly that list of terminals: a number bought with the terminal.

## The four messages become bytes, and the one that cannot

Four of the five handlers existed to *tell the editor something*, and a host cannot
deliver a signal to a core it is linked into — there is no such thing. So the four
become bytes on the channel that already exists:

| the host catches | the core reads |
| --- | --- |
| `SIGWINCH`, `SIGCONT` | `CSI 48 ; rows ; cols ; 0 ; 0 t` — the mode-2048 notification |
| `SIGTSTP` | `\033[?1z` |
| `SIGINT` | the byte `0x03` |
| `SIGHUP`, `SIGTERM` | nothing — it is not a message, it is the process ending |

**`\033[?1z` is ours and is in no spec**, and that is said here rather than discovered.
`first == '?' && argc == 1 && arg[0] == 1 && trail == 'z'` reaches no existing arm —
the `?`-prefixed arms end in `c`, `y` and `u` — and `z` is a letter, so the trail scan
stops on it. Doing real work from inside the termcode parser has precedent in the file:
the resize arm calls `set_shellsize()`, which redraws the whole screen.

**The interrupt travels as the byte it already is, and it is not optional.**
`catch_sigint` set `got_int` directly. The host instead hands the core a `0x03`, which
`fill_input_buf`'s own CTRL-C scan turns into `got_int` — the only way `got_int` is
ever set in raw mode anyway, because raw mode clears `ISIG`. **With no handler at all,
`SIG_DFL` KILLS the editor**: measured, `kill -INT` mid-session leaves the binary this
phase was handed editing and kills a no-handler build with SIG2. The `10gs` probe is
what found it, because the sleep mode deliberately leaves `ISIG` on (below).

**Keyboard CTRL-Z is the one thing that cannot go in band**, and `musl_suspend()` is
why there is one call out and not zero. In raw mode `ISIG` is clear, so CTRL-Z arrives
as the byte `0x1a`, reaches `nv_cmds[]`'s real `{Ctrl_Z, nv_suspend, 0, 0}` row, and
**the core decides** to suspend. A one-way host→core channel has no way to carry that
decision back. The external `kill -TSTP` is the other direction and fits the in-band
path exactly, which is the asymmetry stated once:

```c
    static void
musl_suspend(void)
{
    host_catch(SIGTSTP, SIG_DFL);
    kill(0, SIGTSTP);
    host_catch(SIGTSTP, host_on_tstp);
}
```

`mch_suspend()` goes from 27 lines to eight, and `in_mch_suspend`, `sigcont_received`
and the four-iteration `mch_delay` back-off loop go with it. The first existed only so
the core's own `sig_tstp` could tell *my* stop from *someone else's*, and the second
only to drive the loop: **the call returning is the handshake.**

## The terminal is two operations, not a mode setter

`settmode(tmode_T)` had nine call sites and a three-valued mode, and every site is
attached to an **operation** — the editor takes the terminal, gives it back, suspends,
resumes, sleeps, or re-asserts that it still holds it. Exposing `musl_set_raw(int)`
would move the syscall without moving the responsibility. So `settmode()` splits at the
one line that was ever the host's:

```c
    if (termcap_active && tmode != TMODE_SLEEP && cur_tmode != TMODE_SLEEP)
        ... out_str(t_CBD); out_str_t_TE();     /* leaving  -- screen work, stays */
        ... out_str_t_BE(); out_str_t_TI();     /* entering -- screen work, stays */
    out_flush();
    mch_settmode(tmode);                        /* <- the only host part */
```

into `term_enter()` and `term_leave()`, which keep the escape sequences because those
are screen work. `cur_tmode` becomes a boolean `term_entered`, `mch_cur_tmode` goes —
the two were equal at every one of the eleven call sites, over 521 recorded
observations — and `tmode_T` with `TMODE_COOK`, `TMODE_RAW` and `TMODE_SLEEP` goes to
the sweep, along with `MCH_DELAY_SETTMODE`, which had no caller.

**The two sites that look like exceptions are re-assertions, and a re-assertion is an
idempotent operation.** `getcmdline_int`'s `settmode(TMODE_RAW)` found the terminal
already raw on all 521 observations, and `ask_yesno`'s runs only `if (exiting)`, after
`prepare_to_exit` cooked it. Both are `term_enter()`.

## `TMODE_SLEEP` leaves `ISIG` on deliberately, and the `gs` probe is what proves it

The parameter is `interruptible`, not `discard_input`, and the code says so. Measured
from `mch_settmode`: the sleep mode is the **saved** termios with `ICANON` and `ECHO`
cleared and `VMIN=1 VTIME=0`, applied **`TCSANOW`, not `TCSAFLUSH`** — nothing is
flushed and nothing is discarded. What is different from raw mode is that **`ISIG` is
left on**, so the terminal's INTR character raises `SIGINT` and cuts the `nanosleep`
short instead of sitting in the input queue.

That is load-bearing, and a phase that "simplified" `musl_delay()` into a plain
`nanosleep` would break CTRL-C during `gs` with no recorded harness able to see it —
`gs` is in no case. So the check builds **this phase's own output** with `musl_delay()`'s
two `host_tty_set()` calls deleted and nothing else, and runs `10gs` followed by CTRL-C
1.5 s later on all three:

| binary | comes back |
| --- | --- |
| the one this phase was handed | **1.81 s** |
| this phase's output | **1.81 s** |
| this phase's output without the sleep mode | **never** — killed at 99 s |

Two numbers agreeing prove nothing if a wrong one is not caught.

## Nothing asks whether this is a terminal, which is decision 7 finally kept

`GOALS.md` §II.1's decision 7 is *"do not ask whether stdin or stdout is a terminal.
The check goes entirely."* Phases 85 and 87 left three `isatty()` calls alive and this
takes all three: `mch_check_win`'s, `mch_get_shellsize`'s (with the whole function),
and `fill_input_buf`'s (with the arm below). `stdout_isatty` folds to TRUE and
`nv_esc`'s `out_redir` folds to the terminal arm — **both arms together**, because
folding one leaves an `if (out_redir)` with no definition. **`musl_is_terminal()` is
deliberately NOT written**: the two questions had different subjects — fd 1 for the
size, fd 0 for the input — and neither survives its caller, so a host call to answer a
question nobody asks afterwards is boundary surface for nothing.

## The core cannot acquire a descriptor at all any more

`fill_input_buf`'s `close(0); vim_ignored = dup(2);` arm reopened the editor's stdin
from its stderr when stdin hit end of file and was not a terminal. **That is the core
second-guessing the host about where input comes from**, and once the host owns the
terminal it is the host's business. `GOALS.md` §II.4b becomes absolute: the core is
handed fds 0, 1 and 2 and that is the whole of it — it cannot open, close or duplicate
anything.

The behaviour that goes is real and probe-only. With stdin at EOF and a **terminal on
fd 2** the old binary reopens fd 0 and carries on editing (2,016 bytes of drawn
screen); this one prints `Vim: Finished.` and exits 1 (168 bytes). It fired **0 times
across all 253 recorded rows**, which is why the declaration is empty and the probe is
the evidence.

## `ui_get_shellsize()` stays a query, and this is a design the survey got wrong

The survey proposed making it *"do I know my size?"* — a `shell_size_known` flag set by
the host and by every notification. **That breaks resizing, and the measurement is
exact.** At a `Press ENTER` prompt `set_shellsize()` does

```c
    if (State == (0x2000 | MODE_NORMAL) || State == MODE_SETWSIZE)
    {
        State = MODE_SETWSIZE;
        return;
    }
```

and **discards the width and height it was given**. `wait_return()` then calls
`shell_resized()`, which is `set_shellsize(0, 0, FALSE)`, which reaches
`mustset || (ui_get_shellsize() == FAIL && height != 0)` — and learns the new size only
from `ui_get_shellsize()`'s **side effect**. With the flag, a pty resized while the
editor sits at that prompt stays 24x80 for ever; measured twice, and the in-band
notification is delivered and parsed and still lost.

So `mch_get_shellsize()`'s body moves to the host as `musl_get_winsize(int *, int *)` —
`GOALS.md` §II.4c's own name for it — and `ui_get_shellsize()` keeps its shape. That
also disposes of the trap the survey spent an hour on: on a pipe the host's `ioctl`
fails, `ui_get_shellsize()` returns FAIL exactly as `mch_get_shellsize()` did,
`set_termname()` still emits `t_CWS`, and the recording does not move by one byte. The
flag version cost 10 bytes of stream in every one of the 102 cases.

## `vim_handle_signal` stays, and it is the one core mention that is not a message

It is the deferral machine: `ui_inchar` calls it with `-2` before a long wait and `-1`
after, and `deathtrap` consults it to *defer* a deadly signal arriving while the editor
is not reading, re-raising it later with `kill(getpid(), got_signal)`. Deleting it
would make a deadly signal act in the middle of a screen update — a behaviour change no
recording can see. So it stays, and `tools/zhostonly.py` names it as an exception with
that reason rather than loosening its pattern.

## The wait has two answers, not three

`musl_wait_for_input(long ms)` returns 1 (something to read) or 0 (the time elapsed);
`ms < 0` waits for ever. There is no "interrupted", and the reason is measured:
**nothing reads the one that exists today.** `RealWaitForChar` writes `*interrupted` in
two places, `WaitForChar` passes it through, and `inchar_loop` declares
`int interrupted = FALSE;`, passes `&interrupted` and **never reads it** — eleven
mentions, not one of them a read. The whole `efds` set exists only to produce it, and
it goes.

**The `EINTR` retry does not disappear; it moves inside the host**, and so does the
pending-flag check, which must happen on **both** sides of the `select`: only after an
`EINTR`, and a signal arriving while the editor is not inside `select` is lost until
the next keystroke; only before it, and one arriving during it is lost until it
returns. That is why the signals half's proposed `musl_input_pending()` and the
terminal half's `musl_wait_for_input()` are **one function** and the phase declares one:

```c
    static int
musl_wait_for_input(long ms)
{
    ...
    for (;;)
    {
        if (host_winch_pending || host_tstp_pending || host_int_pending) { return 1; }
        FD_ZERO(&rfds);
        FD_SET(0, &rfds);
        ret = select(1, &rfds, NULL, NULL, tvp);
        if (ret == -1 && errno == EINTR) { continue; }
        return ret > 0 && FD_ISSET(0, &rfds);
    }
}
```

`tvp` is computed once, before the loop, because Linux decrements it in place and the
`goto select_eintr` it replaces did the same.

## Two findings that are patterns, not incidents

* **A struct field whose only reader the phase deletes must go in the EDIT, not the
  sweep.** `signal_info[]`'s `deadly` was read only by `catch_signals()`.
  `tools/deadfields.py` duly removed the **member** — and left the three initialisers
  behind: `warning: excess elements in struct initializer`, three times, which
  `tools/phasecheck.sh` then fails on. A field the edit knows is dead is the edit's to
  take, **with its data**.
* **`-Wunused-but-set-variable` does not reach an address-taken or file-scope object**,
  and this phase met it twice in one edit: `did_read_something`, left set and never read
  by the reopen arm going, and `*interrupted`, write-only through three functions
  because `inchar_loop` takes its address. `CLAUDE.md` records that `deadsweep.py` does
  not act on that warning; both had to be removed by hand.

## The declared delta is nothing at all, and it is phase 85's kind

`phase/103/delta.md` gets a comment block and no line. Two full recordings either side
are **byte-identical** — 102 screen cases, `ref-excmds.txt`, `ref-argv.txt`,
`ref-pty.txt`, `ref-term.txt` — and `tools/coredelta.sh --phase 103` finds the corpus
unmoved: 102 of 102, 111 of 111, 30 of 30.

But it is **phase 85's** kind and not phase 101's: the code runs and the instrument
*cannot see it*. On a pipe `tcgetattr`/`tcsetattr` fail and change nothing; no recorded
case sends a signal, resizes a window, types `gs`, or reaches EOF with a terminal on
fd 2. So the phase owes probes, and the check runs **fifteen** on both binaries.

**MUST DIFFER (7)**

| probe | the input | this |
| --- | --- | --- |
| `resize_inband` — `ESC[48;30;100t` then `:set columns?` | the escape lands as buffer text, 2,248 B | `columns=100`, 5,279 B |
| `trz_query` — `:set trz?` | `termresize=` | `E518: Unknown option: trz?` |
| `trz_set` — `:set trz=sigwinch` | accepted, no message | `E518` |
| `inband_stop` — `ESC[?1z` | not a command; the session ends at EOF, rc 1 | runs `:stop`, comes back, `:q!` quits, rc 0 |
| `eof_on_tty2` — stdin `/dev/null`, a pty on fd 2 | **still editing**, 2,016 B | `Vim: Finished.`, exit 1, 168 B |
| `ctrl_c_redir` — CTRL-C, stdout a pipe | `do_cmdline_cmd("qa")` → `E492` | `Type :qa and press <Enter> to exit Vim` |
| `tstp_external` — `kill -TSTP` | 4,379 B | 4,356 B — the in-band path |

**MUST NOT DIFFER (8)**

| probe | both binaries |
| --- | --- |
| `sigterm_restores` | exit 1, tty back to `ICANON=1 ECHO=1 ISIG=1 ONLCR=1 ICRNL=1`, both messages, **2,241 B** |
| `sighup_restores` | the same, **2,240 B** |
| `sigint_external` | survives and keeps editing, 2,327 B |
| `ctrl_z_key`, `stop_cmd` | 4,401 B and 4,379 B, editing afterwards |
| `raw_mode_live` | `ICANON=0 ECHO=0 ISIG=0 ONLCR=0 ICRNL=0` while editing |
| `pty_resize` | 24x80 asked, resized to 30x100, asked again: `lines=30 columns=100` |
| `stopcont` | a whole screen redrawn on CONT — 1,986 B and 2,008 B |
| `gs_interrupt` | 1.81 s, with its control at 99 s |

**A pty byte count is not an assertion, and the check says which are which.** The pty
probes drive a real terminal with real waits, so what they assert is structural — exit
status, terminal mode, what text was drawn — and the counts are reported. The two
places a count **is** the evidence are `tstp_external`, where it must differ, and
`stopcont`, where both must draw a whole screen. The pipe probes, which are
`tools/zstream.py` and deterministic, assert exactly.

**One correction to a number the survey recorded.** It reported the committed binary
drawing 2,008 bytes on `kill -STOP; kill -CONT` and a variant without the fix drawing
63. Re-measured: the committed binary draws **69 B** in one harness shape and
**1,986 B** in another, because `mch_signal()` installs with `SA_RESTART` and the
`select` simply restarts — `sigcont_handler`'s `redraw_later(UPD_CLEAR)` is deferred to
the next keystroke. Catching `SIGCONT` with the same handler as `SIGWINCH`, and
dropping the notification arm's `if (height != Rows || width != Columns)` guard so that
a notification is always a full redraw, is still right; the reason is *the screen is
correct at once instead of at the next keystroke*, not *a regression is avoided*.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 80,446 | **80,148 (−298)** |
| functions | 1,758 | **1,757** |
| type definitions | 907 | **904** (`tmode_T`, and the two the sweep found with it) |
| DWARF enumerators | 1,181 | **1,177** (`TMODE_COOK` `TMODE_RAW` `TMODE_SLEEP` `MCH_DELAY_SETTMODE`) |
| `nm -u` with the core's flags | 31 | **24**, the gone set as one `comm` |
| `nm -u` as `tools/symbols.sh` counts it | 32 | **25** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| `options[]` rows | 108 | **107** (`'termresize'`) |
| `cmdnames[]` rows | 98 | 98 — **no Ex command is touched** |
| `nv_cmds[]` rows | 194 | 194 |
| binary | 805,544 | **797,192 (−8,352)** |
| sweep | | 2 rounds, 0 warnings |
| phase | | **56 s**, of which the probes are most |

## Its placement

`stage 103`, `package host` beside 100, 101 and 102, with `uses host:103 seed:83`,
`uses host:103 harness:86` and `uses host:103 terminal:85 rationale` — phase 85 removed the
two "not to a terminal" warnings, and this is where the core stops asking the kernel
about a terminal at all. The two dependencies a reader expects, on phases 101 and 102,
**cannot be written**: `tools/packages.sh --check` refuses a `uses` inside one package,
and 17–20 are one. Their reasons are in the phase program's header instead — the host
block goes inside the launcher region phase 101 created, and the deadly-signal restore
ends in phase 102's `vim_host_exit` → `__builtin_longjmp` → `return 1`.

**`need 103 swept` is not required, and it was measured.** The edit applied to the
unswept text phase 102's edit leaves gives the identical 80,447 → 80,181; every anchor
is exact text at a counted occurrence.

**`apart 102 103`, measured, and the first of its three messages is the one a reader
would not predict.** `tools/phaserun.sh 102-103` on q101 stops in phase 102's check
with `` `deathtrap` as a whole word has 4 mentions, expected 3 `` — a phase about
*removing* signal handling leaves one **more** mention of a handler, because the host
installs the core's rather than replacing it. The other two are ordinary: `the file
gained -280 lines, expected 18`, and `options[] is not the 108 rows phase 95 left`.
**This one is both directions**, unlike 99–100, 100–101 and 101–102: phase 103's own check
states its gone set as one `comm` against the stage's symbol snapshot, and a stage
takes one snapshot at its start — so on a 102-103 stage it is handed q101's set and the
gone set is its seven **plus `exit`**. That is `apart 97 98`'s shape, and it is
reasoning from the two programs rather than a second run, because 102's check refuses
first and there is nothing after it to observe.

## What whim-vim is after twenty phases

```
whim-vim.c        80,148 lines          from whim-vim.c's 86,614  (-6,466, 7.5%)
functions         1,757
type definitions  904
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      24 with the core's flags, 25 as tools/symbols.sh counts
binary            797,192 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 24, attributed — and two rows are now the host block's alone.**

| why | symbols |
| --- | --- |
| **the terminal**, every one of them in the host block | `read` `write` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` (7) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals**, all four in the host block | `sigaction` `sigemptyset` `kill` `getpid` (4) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

`getpid` is the odd one: it is `mch_get_pid()`'s, called once to write `b0_pid` into
block zero, and `vim_handle_signal()`'s re-raise — neither is this phase's, and `b0_pid`
is written and never read, so one line frees it whenever block zero is somebody's
phase. `__errno_location` stays too, and the phase says so rather than implying
otherwise: `errno` has exactly three mentions and does not move at all — the
`#include <errno.h>`, the `tcsetattr` retry and the `select` test — and the last two are
host-side now, so **`<errno.h>` leaves the CORE and the symbol leaves the process at the
split**.

**`GOALS.md` §II.4c is built out.** Its three steps were `main()`, the two stream
calls, and the terminal with its signal set; 101 and 102 did the first and this does the
third, and the second is what is left — `mch_write`'s `write(1, …)` and
`musl_read_input`'s `read(0, …)`, the last two syscalls the core still makes for
itself. What remains after that is the file split, and `tools/zhostonly.py` is the
check that survives into it: when `editor.c` and `whim-vim.c` become two files, the
host block becomes the second file and the tool becomes `grep` over the first.
