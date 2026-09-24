# Phase 113 — the message fold: `msg_puts_printf()` and the branch that reaches it

`phase/113/edit.go` and `phase/113/check.go`, `stage 113`, `package host`.
`msg_puts_attr_len()` ends in a two-armed test: the true arm handed the message to
`msg_puts_printf()`, 75 lines that reach the terminal **without a screen**, and the
false arm draws it. The true arm is never taken, and this phase folds it to two lines
that say the same thing to the host:

```c
    host_message((char *)str, maxlen, !info_message);
    msg_didout = TRUE;
```

`msg_puts_printf()`, its prototype, and `vim_strlen_maxlen()` and its prototype — which
the sweep finds, that function's only call being inside it — go with it. **Two
functions, not one**: 1,756 definitions → 1,754, and 79,857 → 79,766 lines, the edit
adding one and the sweep taking 92.

## Which kind of dead, and it is not phase 92's

Phase 92 removed code that **could not run**. This removes code that **can** run and
never does, which is phase 95's kind, and the difference decides what evidence is owed.
`msg_use_printf()` is a live predicate: instrumented on this phase's own output it
answers TRUE **23 times**, every one at `msg_clr_eos_force()`, every one in
`ref-argv.txt`, one per `mainerr` row — with `full_screen` FALSE in all 23, so the body
it guards is a no-op. The phase therefore leaves the predicate at six mentions and
claims only that **one of its four call sites is dead**. The evidence is phase 95's
shape: the input source built twice with the identical `write(2, "PP-ENTERED\n", 11)`,
first in `msg_puts_printf()` — **0 of 106 records** — and then in `msg_puts_display()` —
**103 of 106, 5,749 occurrences**.

## Why the message is kept rather than dropped

Deleting the arm's body outright is five lines smaller and records identically. It was
rejected: **a phase about removing dead *code* must not quietly remove a
*capability*.** `host_message(msg, len, err)` takes `len < 0` as `strlen` and `len >= 0`
as an exact count, which **is** `msg_puts_printf`'s own `maxlen` contract, measured by
reading both. The arm is never executed, so equivalence is not claimed: what the two
lines do not reproduce is the CR-before-NL insertion and the `msg_col` bookkeeping, and
no recording or probe in this pipeline can reach either.

## That the recording did not move is not the check, and this is where that matters most

**The two folds this phase declines also record byte-identically, and one of them is
wrong.** So the check is 36 probes and an instrumented pair, and it **builds the
rejected folds and requires each to move a named probe**:

* **`msg_clr_eos_force()`'s test cannot be folded safely.** Phase 104 said folding it
  "would run `screen_fill()` with no valid screen". That is right, and the number behind
  it is the interesting part: `screen_fill()` returns early on `ScreenLines == nullptr`,
  and `ScreenLines` **is** null in all 23 `mainerr` cases, which are the only 23 places
  the predicate is TRUE in a recording — **so the fold leaves the whole 106-record
  recording byte-identical and a phase checked only against the corpus would ship it**.
  Two probes see it: `t_ti_stopterm` 2,266 → 2,280 bytes and `hup_clean` 2,124 → 2,142,
  the extra eighteen being `\x1b[24;63H\x1b[K\x1b[24;1H` **after** `Vim: Finished.` —
  the editor erasing the last line of a screen it has just declared unusable, on its way
  out. Guarding with `msg_check_screen()` instead is **not** a cheaper spelling of the
  same thing: it drops the `swapping_screen() && !termcap_active` disjunct, which is
  exactly what `t_ti_stopterm` reaches.
* **`exit_scroll()`'s printf arm is ALIVE, and phase 104 was wrong to name it a follow-up
  beside `msg_puts_printf()`.** `phase/104/check.go` says the two "fire in ZERO of
  106 records"; that is true of the **corpus** and true of the editor only for the
  first. With **no signal at all** the arm fires in **three of this phase's 32 stream
  probes** — `t_ti_more`, `debug_more`, `term_ti_then_ti` — and in **three of its four
  deadly-signal probes**. Folding it to `out_char('\n')` is not a crash risk:
  `out_char('\n')` emits `\r` first, so the bytes on the wire are the same two. It moves
  them **from fd 2 to fd 1**, and on a pty where both descriptors are the same device
  the combined stream is byte-identical — which is why `tools/zpty.py` could never see
  it and why folding it here would be **undeclarable**. It belongs to whichever phase
  decides the core writes nothing to fd 2 at all. The check builds that fold too and
  requires it to move exactly those three stream probes and those three signal probes,
  so *"this phase did not disturb it"* is measured rather than asserted — and that is
  what the four deadly-signal probes are for, and why they run **with fd 2 on a pipe of
  its own**.

## A counting trap that cost a first attempt at the anchor

`    if (msg_use_printf())` at four spaces is a **substring** of the same line at eight,
so `str.count()` says 3 where `grep -c '^    if (msg_use_printf())$'` says 2 — the third
match being `exit_scroll`'s. And there are **four** call sites, not three: the fourth is
written `if (!msg_use_printf())` in `hit_return_msg()`, and an edit that greps for the
positive spelling misses it. The anchor is the four-line block, whose count is 1, and
all three untouched sites are asserted verbatim before and after.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 77,779 | **77,693** — the edit adds 1, the sweep takes 87. Re-measured on the canonical text; the rest of this table is not |
| function definitions | 1,756 | **1,754** |
| `make editor.c` | 77,977 | **77,886**, 0 directives, 0 errors, the boundary unchanged |
| `nm -u` | 17 | **17, the same set** |
| binary | 782,760 | **782,760** |
| records that moved | | **0 of 106**, with 32 stream probes and 4 signal probes identical |

## Its placement

`stage 113`, `package host 100 101 102 103 104 113`, because this is phase 104's own follow-up
and not a tidy-up. Three `uses`: `seed:83` and `harness:86` for the recording, and
`boundary:108`, because `host_message()` is a name the `editor.c` cut enumerates only
since phase 108 turned the function pointer into a declaration. There is deliberately
**no `uses host:113 host:104`** — `uses` records a dependency *across* packages and
`tools/packages.sh --check` refuses one inside a package — so that relation is written
as a comment on the `package host` line instead.

`apart 104 113`, measured rather than assumed: phase 104's check pins thirteen names, of
which **seven are already broken by phases 105–112**, four are untouched here, and exactly
**two** move at this phase — `msg_puts_printf` 3 → 0 and `info_message` 9 → 7.
`msg_use_printf` stays at 6, which is the other half of phase 104's assertion and
survives intact. **No `need 113`**: the edit's one anchor is a four-line block of exact
text whose count is 1, and no count a sweep can move.

**Phases 111 and 112 landed while this one was being written**, and the independence was
measured rather than assumed: every counted anchor has the same value on q110, on q111 and
on q112. One thing did move — phase 111 renames `musl_gettimeofday` to `musl_now_ms`, and
that name is one of the boundary names the `editor.c` cut prints. This check never
writes that set out: it computes it from the input and from the output and requires the
two to be equal, so the rename cost it nothing. **That is the whole argument for
counting a set as a rule rather than as a table of constants.**
