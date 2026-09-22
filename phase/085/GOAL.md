# Phase 85 — the core stops diagnosing its own terminal

`phase/085/edit.sh` and `phase/085/check.sh`, `stage 85`, `package terminal`. The
first phase that cuts source, and the first piece of *a component, not a program*: a
host hands the core its input and output, and whether either is a terminal is the
host's business. Upstream's answer is to complain and then wait —

```
Vim: Warning: Output is not to a terminal
Vim: Warning: Input is not from a terminal
```

— on stderr, `out_flush()`, `exit(1)` if `--ttyfail` was given, and then
`ui_delay(2005L, TRUE)` so that a person can read them. All of it is
`check_tty()`'s second branch, and all of it goes, with the `--ttyfail` flag: its
`case '-'` test, the `parmp->tty_fail = TRUE` it set, and the `tty_fail` field of
`mparm_T`. `--ttyfail` becomes what any other unknown word is, `ME_UNKNOWN_OPTION`
through `mainerr()`.

**The pause was looked for, not assumed.** There are nine `ui_delay()` call sites in
`whim-vim.c` and exactly one is the warnings': 2005 ms, inside the branch that
printed them, under upstream's `scriptin[0] == NULL` ("do not pause while a script
is being read", which has nothing left to condition). The other eight are each a
different pause — 3001 ms for `W14: List of file names overflow`, 1002 ms for the
`'readonly'` warning, the 1000 ms slices of `ui_delay`'s own wait loop, 1003 and
3003 in `wait_return()`, 1006 in `check_for_delay()`, and `p_mat`'s two in
`showmatch()` — and none is reached from `check_tty()`. **Measured: the pause is
2,005 ms once, for both warnings together, not one per warning.**

**What stays, and the reader that forces each.** The `exmode_active` branch —
`if (!input_isatty) silent_mode = TRUE` — because Ex mode is a later phase, and it is
also what keeps `mch_input_isatty()` called. `stdout_isatty`, read outside `main` by
`out_redir = !stdout_isatty` in the message layer, which keeps its one assignment and
so `mch_check_win()` and that function's `isatty(1)`. `want_full_screen`, whose
second reader, `params.want_full_screen && !silent_mode`, survives the branch that
went. **So all five `isatty()` calls remain and the libc surface does not move:
79 undefined symbols before and after, the same set.** Folding an `isatty` caller is
a phase of its own if it is ever one; this phase is the warnings, the pause and the
flag. `check_tty()` then reads nothing from its argument, so it takes `void` and its
one caller drops the `&params` — the alternative being
`__attribute__((unused))` on a parameter nothing will read again.

**The delta is none, and every harness here is blind to it — so the phase's own
probes are the check.** `behaviour.py` and `exsweep.py` run the editor `-e -s`:
`exmode_active` is set before `check_tty()`, the first branch takes it, and the
warnings were never on any recorded stderr (grepped: the only baseline line
mentioning a terminal is `exsweep`'s `SKIPPED (hands over the terminal)`).
`termcheck.py` drives a real pty, where both streams *are* terminals. A delta of
"none" from a harness that cannot see the code proves nothing, so
`phase/085/check.sh` measures the removed behaviour directly, in both directions:
the edit part first builds the binary the phase was **handed**, from the boundary's
own makefile flags, and every probe requires the old binary to do the thing and the
new one not to.

What they measured, `TERM=xterm`, stdin a file of `ihello world<Esc>:q!`, stdout a
file:

| | input binary | after |
| --- | --- | --- |
| stderr | 85 bytes, both warnings | **0 bytes** |
| elapsed | 2,010 ms | **5 ms** |
| stdout, the escape stream | 2,108 bytes | 2,108 bytes, **byte-identical** |
| exit | 0 | 0 |

`--ttyfail` exits 1 under both — the old binary because the flag asked it to, the new
one because the flag is gone — so the status is not the check and what `mainerr()`
prints is: `Unknown option argument: "--ttyfail"`, present after and absent before.
A bare `--` still ends the options, `+cmd` and `-T dumb` still work, each with the
same result from both binaries. And a real terminal is untouched: one `ptyrun`
session that types text, asks `:set term?` and `:wq` gives the same status, the same
file and the same answer either side — as do the 19 pty sessions `termcheck.py` runs
inside the declared delta.

**Measured, and what did not move.** 86,614 → 86,586 lines. The sweep found nothing
at all — no function, prototype, type, variable, field or enumerator — so the cut
orphaned nothing, which is what the kept readers above predicted. In the plain
object `.text` goes 654,846 → 654,576 bytes and `.rodata` 17,785 → 17,689, the two
strings and the branch; the stripped static binary is **869,512 bytes either side**,
the shrinkage absorbed by alignment padding. The phase runs in 28 seconds, 3.6 of
them the extra compile of the input binary its probes need. Its boundary is
`74ca3e1ffeb8`, and `make whim-verify` recomputes all three.

One `uses` line: `terminal:85 seed:83 mechanical`, for phase 84's reason —
`tools/coredelta.sh` refuses without the `.reference/core-baselines` phase 83 records,
and "none" is checked against them.

It does not run `tools/create_cmdidxs.py --check`, which every whim edit of the
command table ends with: the derived first-two-letters index went with the table whim
reduced, there are no `ex_cmdidxs.h` banners left in `whim-vim.c`, and the tool
raises rather than reporting nothing.
