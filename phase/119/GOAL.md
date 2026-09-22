# Phase 119 — the core names no libc function at all

`phase/119/edit.go` and `phase/119/check.go`, `stage 119`, `package host`. Phase 118
ended with a sentence it would not write down, and this is the phase that gets to write
it. The core's block of ordinary, non-`static` declarations — the libc the editor spells
out by hand since phase 110 put the headers below it — was two lines, and both go:

```c
    int getpid(void);
    int kill(int pid, int sig);
```

**They are reached two different ways and only one of them needed a host call**, which is
the whole shape of the phase. `getpid` is **avoidable outright**. `mch_get_pid()` is
`return (long)getpid();` and had exactly one caller, `long_to_char(mch_get_pid(),
b0p->b0_pid)` in `ml_open()`; `b0_pid` is the process id written into block zero of a swap
file and had exactly two mentions in the whole file, its own declaration and that write.
It is **write-only, and not by anything zero did** — `whim-vim.c`, this pipeline's
immutable input, already has exactly those two, whim having taken the readers with
recovery. So the write goes, `mch_get_pid()` goes with it because the declaration it needs
is going, the sweep takes the forward declaration and the field, and `getpid` leaves the
core with nobody calling a host at all. Phase 103 saw it coming and said so in as many
words: *"`b0_pid` is written and never read, so one line frees it whenever block zero is
somebody's phase"*.

`kill` is **moved**. Its one core site is `vim_handle_signal()`'s `kill(getpid(),
got_signal);`, re-raising a deadly signal that arrived while the editor was not reading,
and it becomes `host_raise(got_signal);` — `static void host_raise(int sig);` at the end
of the single run of core → host prototypes phase 108 made one block of eleven, and a
definition inside the host region that calls `kill(getpid(), sig)`. **The deferral is not
dropped**: the blocked/`got_signal` mechanism is exactly what it was, so the core still
decides *when* the signal acts and only the raising crosses the line. **It takes no pid**,
which is phase 118's rule for `host_write` and phase 103's for `musl_read_input` applied
here: a core that can no longer ask for its own process id must not be handed one. And it
is spelled `kill(getpid(), sig)` and not libc's `raise()`, because `raise` has not been an
undefined symbol of this file since phase 103 and a wrapper reaching for it would **add** a
libc symbol in the phase whose subject is the core's last two.

## The claim is measured from the cut and not from the block, and those are two assertions

An empty block is a fact about a paragraph. *The core names no libc function at all* is a
fact about everything above the first `#include`, and the two are not the same sentence,
because **a bare `extern` declaration is invisible to the cut's ordinary check**: gcc
warns `'X' used but never defined` for a `static` function and says nothing whatever about
an ordinary one. That is exactly how two libc names sat above the boundary for nine phases
without the interface set noticing them.

So the check compiles `make editor.c`'s cut to an **object** and takes `nm -u` of it,
which is the set of names the core needs from outside *itself*: **19 in, 18 out**, and
every one of the 18 defined below the boundary in this same file, computed from the text
and listed nowhere. The identical computation on the input finds two that are not —
`getpid` and `kill` — and **that control is what keeps the emptiness from being two
numbers agreeing**. Neither cut defines an external symbol either: `main` is below the
boundary and is the host's.

## The phase frees no symbol and says so as an equality

`nm -u` is the same set in and out, a `cmp` in both directions, and `getpid` and `kill`
are both still in it: `host_raise()` calls both where `vim_handle_signal()` did,
`musl_suspend()` calls `kill` as well, and **a symbol leaves when its last caller leaves
the file**. What moves is the thing `nm -u` cannot show — the cut's warning set, the
core → host interface, from **17 names to 18**, `host_raise` arriving and nothing leaving.

And the thing no tool but `tools/zhostonly.py` can show: **above the boundary the core's
whole remaining vocabulary of the host is two words**, `SIGHUP` three times and `SIGTERM`
three, the two deadly signals the editor names because it **prints** them. The input said
`getpid` three times and `kill` twice as well, and those were the only mentions of any
host word in the core that were not a message. The tool reports 6 of its 18 named
exceptions live, and all six are that message.

## The fold this phase was asked to take does not exist, and the check says so

Phase 100 removed `deathtrap()`'s `entered >= 3` ladder as code no build of whim-vim could
reach, which leaves `entered` able to reach 2 and no further, and the question put to this
phase was whether that makes anything around the `if (entered == 2)` arm foldable.
Measured, it does not: **`entered` has exactly three reachable values and every one of
them is read.** 0 is read by the entry guard `if (entered == 0 && ...)`, which
distinguishes a first entry from a nested one; 1 and 2 are told apart **twice** — by the
double-signal arm, which calls `getout(1)` and never returns, and by `v_dying = entered;`,
whose value reaches `getout()`'s two `if (v_dying <= 1)` tests and selects the buffer
cleanup there. No two of the three states are interchangeable, so there is nothing to
fold. The non-fold is asserted as a byte comparison: `deathtrap()` is identical in and out
at 38 lines either side, and `vim_handle_signal()` differs in exactly one line.

## The declared delta is nothing at all, and the two halves are blind for opposite reasons

It is phase 85's kind and phase 95's — the code **runs** and the instrument cannot see it —
so the phase owes probes and builds six binaries of its own. That the corpus cannot see
either half is measured. An instrumented build of the input, over all 102 screen cases and
marking every one: the `b0_pid` write runs in **102 of 102** cases, 102 times in all, being
on the path of every buffer the editor opens, and the recording still does not move,
because nothing reads the field. The re-raise fires in **0 of 102**: nothing in the corpus
sends the editor a deadly signal, so `got_signal` is never set and the deferral has
nothing to re-raise. `vim_handle_signal()` itself is entered 4 times in 2 cases, which is
**reported and not pinned**, this phase being unable to move it — `ui_inchar()` calls it
only around a wait longer than 100 ms, and a corpus whose stdin is a file of keystrokes
almost never waits.

**So the two controls are on the one line the phase deletes**, and they are the phase
itself rather than a borrowed anti-vacuity check. That line replaced by `return FAIL;` —
the same line, in the same place — moves **106 of 106** records, so the corpus really does
execute it and the empty `diff -r` is not the corpus missing the code. The same line
writing a **constant** instead of the pid moves **0 of 106**, which is *`b0_pid` is
write-only* measured rather than argued, and a control that moves nothing is reported here
and not quietly dropped.

**The probe the other half owes is a forced deferral**, driven identically into both
binaries: `(void)vim_handle_signal(SIGTERM);` with `blocked` still TRUE and then
`(void)vim_handle_signal(-2);`, appended to `mch_init()` — exactly the path
`kill(getpid(), got_signal)` was on and `host_raise(got_signal)` is on now. Input and
output agree in every byte of stdout (48), stderr (0) and status (1), and the screen
carries `Vim: Caught deadly signal TERM` and `Vim: Finished.`. It can fail twice, and
identically on both binaries: with the re-raise **deleted** the deferred signal is simply
lost and the editor runs on to end of input (157 bytes), which says the probe goes through
the line this phase rewrites; and with the re-raise given `SIGHUP` instead of the signal
that was deferred the screen says `Caught deadly signal HUP` (47 bytes), which says the
**argument** crosses the boundary and not merely the call.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,804 | **79,799 (−5)** |
| the core's plain libc prototype block | 2 lines | **0 — the block is gone** |
| `nm -u` of the **cut**, compiled to an object | 19, two of them libc | **18, every one defined below the boundary** |
| `make editor.c` | 77,899 | **77,888**, 0 directives, 0 errors |
| the boundary's warning set | 17 names | **18**, computed at run time on both sides |
| `nm -u` of the whole file | 17 | **17, the same set** — `getpid` and `kill` still in it |
| external symbols | `main` | `main` |
| host words in the core (`zhostonly.py`) | `SIGHUP` 3, `SIGTERM` 3, `getpid` 3, `kill` 2 | **`SIGHUP` 3, `SIGTERM` 3** |
| the `#include`s | line 77,901 | line **77,890**, the same eleven consecutive lines |
| binary | 782,760 | **782,760 bytes, 397,978 of them different** |
| `cmdnames[]` / `options[]` rows | 98 / 107 | 98 / 107 |
| records that moved | | **0 of 106**, against a write executed 102 times |

## Its placement

`stage 119`, `package host` — a thing the core did for itself becomes a thing it asks the
host to do, or stops doing. It belongs beside 113, 115 and 118 rather than in `boundary`,
which is about drawing the line and about the core's own spelling of types and constants.
Four `uses`: `seed:83` and `harness:86`; `boundary:108`, the one prototype going at the end
of the single run of core → host declarations that phase made of eleven; and
`boundary:110`, because the whole claim is stated as a property of the **cut**, and the
first `#include` is the line between core and host only because 27 moved the eleven
directives below the core.

**One `apart` line, measured as a real shared stage**, q117 restored with both edits, one
sweep and both checks: phase 118's check stops three ways, the first being the block this
phase exists to empty — *the ordinary declarations above the boundary are none and the
input's block minus the three is `int getpid(void); / int kill(int pid, int sig);`* — then
*the file is 79799 lines and the input was 79786 — expected 18 more*, and *the boundary
moved from line 77901 to line 77890, and it must not*, which is phase 118's own statement
that it moves no line above the first `#include`. One direction only: phase 119's check was
then run on that same tree and every part held. **Six further `apart` lines are not
written, with the reason.** Phases 109, 110, 111, 113, 115 and 117 all have checks this output
breaks — 109's and 110's nine-entry `PROTOS` list reaches **zero of nine** on it, which is
this arc read from the losing side — but every one of them is already broken by phase 118
and recorded against it, and any stage holding 36 and one of the six would have to hold 35
too. **A redundant `apart` nobody measured is worse than none.** No `need 119`, measured in
the same run on phase 118's unswept output.

`tools/zhostonly.py` gains `getpid` as a host word and three named exceptions, and the
`vim_handle_signal:kill` exception gains a 0 beside its 1 — a count there is a **tuple**
of the values it takes, one per phase that runs the tool, because the core's vocabulary
shrinks between them. Measured, the cost of that tool edit is **20 Part II keys and no whim
or slim key**: the units and the edits of phases 103, 104, 108, 109, 110, 111, 113, 115, 117 and
118, with all 107 whim unit, whim edit and slim phase keys byte-identical either side.
`make whim-verify` (13 of 13) and `make slim-verify` (12 of 12) are the gate core rule 9 asks
for, and both were green.
