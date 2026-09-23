# Phase 104 — the messages are the editor's, the writing is the host's

`phase/104/edit.go` and `phase/104/check.go`, `stage 104`, `package host`.
`GOALS.md` §II.4c's second step, and the half of it that is not the screen: *"`printf`
for the messages that appear before there is a screen, which is itself a question for
the host"*. Every byte this file has ever put on a **stream** instead of a screen now
goes through one callback the launcher installs, in exactly phase 102's shape:

```c
static void (*vim_host_message)(const char *msg, int len, int err);

    static void
host_message(const char *msg, int len, int err)
{
    ...
    int w = (int)write(err ? 2 : 1, msg + off, (size_t)(n - off));
    ...
}
```

`vim_main()` takes it as a fourth parameter and installs it beside `vim_host_exit`; the
two become `musl_` prototypes at the split, together. `<stdio.h>` goes with the symbols
— **twelve directives become eleven**, the second time a Part II phase has removed one and
the same argument phase 99 made.

**Formatting stays in the core, and that is what turns twenty statements into eight call
sites.** `vim_snprintf` has been the only formatter in the file since phase 97, so the
two multi-part speakers assemble into a 1024-byte local and hand over one string, and
the other six sites are one call each with the text unchanged. Measured with a
`SOCK_SEQPACKET` socketpair as fd 2, which preserves write boundaries exactly: `-Q` was
**6 writes of 96 bytes and is 1 write of 96**; `-T no-such-term-9x` was **5 of 54 and is
1 of 54**. The same bytes, one syscall — and the latent hazard goes with them, stdout's
buffered `printf` arm arriving after everything the editor drew.

## Four of the seven symbols are named nowhere in the source

Unlike phase 103 this one really frees symbols, and the reason is the rule phase 103
stated: a symbol leaves when its last **caller** leaves the file. `printf` and `fprintf`
were being *called* here, not merely mentioned.

```
nm -u  24 → 17, gone exactly
    fflush  fputc  fputs  fwrite  printf  putchar  stderr
arrived: nothing
```

**`fputc`, `fputs`, `fwrite` and `putchar` have never appeared in `whim-vim.c` at all.**
They are what gcc emits for `printf("%s", x)` and `fprintf(stderr, "%s", x)`, and no
grep of the source could have found them. So they were **predicted** to leave with the
construct and then **verified by building** — which is the whole reason the check states
the claim as one `comm` with an *empty* `arrived` side rather than as a count. A
prediction about a symbol the source does not name has nothing but the linker to
confirm it.

**`__errno_location` is not this phase's and the check requires it PRESENT.** `errno` is
3 → 3, its two uses being `host_tty_set`'s and `musl_wait_for_input`'s `EINTR` tests,
both inside phase 103's host block. It leaves at the split, not here — said out loud,
because a reader who watches seven symbols go will look for the eighth.

**And `printf` is not at 0.** It is 13 → 10, and none of the ten is a call: nine are
`__attribute__((format(printf, …)))` and one is the string `"E767: Too many arguments
for printf()"`. `assert printf at 0` fails on a correct phase. The assertions that work
are `fprintf` 16 → 0, `stderr` 17 → 0, `fflush` 1 → 0, `printf` 13 → 10, and `nm -u`.

## The inventory is twenty statements in five functions, and the brief said twenty-one in six

The survey this phase was written from counted twenty-one output statements in six
functions, and the sixth was `nv_esc`'s `Type :qa! and press <Enter> to abandon all
changes`. **Phase 103 had already taken it**, with `stdout_isatty` and the `out_redir`
arm it sat in. So `fprintf` is sixteen here and not seventeen, `stderr` is seventeen and
not eighteen, and there are **eight call sites and not nine**. The edit counts its input
rather than trusting the survey, which is the only reason the arithmetic closed:

| function | statements | what it says |
| --- | --- | --- |
| `mainerr` | 6 `fprintf` | the version banner and the argv refusal |
| `report_term_error` | 7 `fprintf` | `'<term>' not known, defaulting to 'xterm'` |
| `set_termname` | 1 `fflush` | eleven lines *below* the call above, not inside it |
| `msg_puts_printf` | 2 `printf`, 2 `fprintf` | whatever message was being printed |
| `exit_scroll` | 1 `printf`, 1 `fprintf` | `"\n"` / `"\r\n"` on the way out |

The instrument goes on **nineteen** of the twenty, `fflush` taking no message.

## `msg_puts_printf()` and `exit_scroll`'s printf arm are deliberately KEPT

All 75 lines of the first and the else arm of the second stay, and that is a decision
rather than an oversight. **`msg_use_printf()` is not dead: it returns TRUE 23 times in
106 records** — once in each `mainerr` record, from `mch_exit` → `exit_scroll()`'s else
arm → `msg_clr_eos_force()`, where `full_screen` is FALSE and the body it guards
therefore does nothing. It is never true at `msg_puts_attr()`'s call site, so
`msg_puts_printf()` is entered **0 of 106 records** against a control that marks 100 of
102 screens. That is phase 95's kind of dead and not phase 92's: the branch *can* be
taken and never is.

Folding either would run `screen_fill()` on a screen the test has just called unusable —
`msg_clr_eos_force()` with no valid screen, or `msg_puts_display()` on a screen
`msg_use_printf()` has just said is not there. Removing them is a separate phase with a
separate question — *"the screen is always usable in this build"* — and phase 95's kind
of evidence to gather, and **it would free nothing, because the symbols are gone here**.

**PHASE 113 CORRECTED THIS, AND THE PART THAT IS WRONG IS THE PART ABOUT
`exit_scroll`.** This phase's check says the two speakers "fire in ZERO of 106
records", which is true of the **corpus** and true of the **editor** only for
`msg_puts_printf()`. `exit_scroll()`'s printf arm is **alive**, with no signal at all:
measured in phase 113's check, it moves **three of that phase's 32 stream probes**
(`t_ti_more`, `debug_more`, `term_ti_then_ti` — each `:set t_ti=X` or `-T debug`, a
paged `:set all`, exit) and **three of its four deadly-signal probes**. It is invisible
here because a recording drives one pty on which fd 1 and fd 2 are the same device and
the two bytes are the same two bytes either way — `out_char('\n')` emits `\r` first —
so the fold moves them from **fd 2 to fd 1** and nothing in this pipeline's instrument
can see that. It is therefore not merely undone but **undeclarable**, and it belongs to
whichever phase decides the core writes nothing to fd 2 at all. The other half of the
sentence survives intact: `msg_clr_eos_force()`'s test cannot be folded safely, and
phase 113 measured *why* — `screen_fill()` returns early on `ScreenLines == nullptr`, so
the fold leaves the whole 106-record recording byte-identical and two probes see the
eighteen extra bytes it emits after `Vim: Finished.`

## The bound that comes with the buffer, stated rather than declared

`mainerr`'s `str` and `report_term_error`'s `term` are both argv, so assembling into a
1024-byte buffer caps a message that used to be unbounded. That is a real behaviour
change and **no instrument in this pipeline can see it**, because nothing in the corpus
comes within 800 characters of the bound and `phase/104/delta.md` is a list of records
that moved. So it is not declared; it is pinned as probes, in both directions and in
both speakers, so it cannot drift:

| probe | the input | this |
| --- | --- | --- |
| an unknown option of **900** characters | 995 B | **995 B — byte-identical** |
| an unknown option of **930** characters | 1,025 B | **1,023 B — the turn** |
| an unknown option of **2,000** characters | 2,095 B | **exactly 1,023 B** |
| `-T` of **900** characters | 939 B | **939 B — byte-identical** |
| `-T` of **2,000** characters | 2,039 B | **exactly 1,023 B** |

1024 is `IOSIZE`, which is what every other message in this editor is built in. The
counts are **raw and not scrubbed** except at 900: the version banner's
`__DATE__`/`__TIME__` differ between two builds and their *length* does not, so only the
equality needs scrubbing and the caps do not.

## `coredelta.sh` is the second opinion here and not the first

The declared delta is nothing at all, and two full recordings either side are
**byte-identical** — `diff -r` reports 0 lines across 102 screen cases, `ref-excmds.txt`,
`ref-argv.txt`, `ref-pty.txt` and `ref-term.txt`. The control proves that table can
fail: `write(err ? 2 : 1, …)` made `write(err ? 1 : 1, …)`, **one character**, moves 24
records and **217 lines** of `diff -r`.

**And it confirms a trap the survey named.** Run on the same control,
`tools/coredelta.sh` names only **fourteen** of those twenty-four, because ten of them
are argv rows phases 87 and 88 already declared (`-`, `--`, `-e`, `-E`, `-e -s`, `-v`,
`f.txt`, `f.txt g.txt`, `+q! f.txt`, `-- +q!`) and `tools/zcompare.py` therefore accepts
any *further* movement in them silently. A declared row is not compared again. So this
check diffs the two recordings itself and keeps `coredelta.sh` as the second opinion —
run on the control, where it must refuse.

**The instrumented pair is what makes the empty declaration mean something**, and it is
phase 92's shape: the input built with `write(2, "MESSAGE-OUT\n", 12)` at all nineteen
output statements and the output with the identical instrument inside `host_message()`
mark **exactly the same 24 of the 30 argv rows, by name**, and 0 of 102 screens, 0 of
`ref-excmds.txt`, 0 of `ref-pty.txt` and 0 of `ref-term.txt`. Same places, same times,
different primitive. The 24 are 23 `mainerr` and one `report_term_error` — which is also
what says the other four speakers fire in zero of 106 records.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 80,148 | **80,173 (+25)** |
| functions | 1,757 | **1,758** (`host_message`) |
| type definitions | 904 | 904 |
| DWARF enumerators | 1,177 | 1,177 |
| `nm -u` with the core's flags | 24 | **17**, the gone set as one `comm` |
| `nm -u` as `tools/symbols.sh` counts it | 25 | **18** |
| external symbols | `main` | `main` |
| `#include` | 12 | **11** (`<stdio.h>`) |
| bare `write()` call sites | 1 | **2** — `mch_write`'s and `host_message`'s |
| `options[]` rows | 107 | 107 |
| `cmdnames[]` rows | 98 | 98 |
| `nv_cmds[]` rows | 194 | 194 |
| binary | 797,192 | **784,392 (−12,800)** |
| sweep | | 0 warnings |
| phase | | **28 s** |

## Its placement

`stage 104`, `package host` beside 100, 101, 102 and 103, with `uses host:104 seed:83` and
`uses host:104 harness:86` mechanical, `uses host:104 vendor:97 mechanical` — `mainerr` and
`report_term_error` assemble with `vim_snprintf`, which is the only formatter left in
the file because phase 97 put `sprintf` onto it rather than vendoring one — and
`uses host:104 includes:99 rationale`, because a phase may remove a directive at all only
since phase 99 replaced the charter's old reading of the directive count as a property
the pipeline preserves.

**`need 104 swept` is not required, and it was measured** in the same run that measured
`apart 103 104`: phase 104's edit applied to the **unswept** text phase 103's edit leaves
gives 80,181 → 80,206, every one of its anchors holding — the twelve input counts, the
twenty statements, the eleven output counts and the whole launcher tail. Its cuts are
exact text in functions no sweep touches and its computed parts are counts of words a
sweep cannot create, so there is nothing that could shrink silently.

**`apart 103 104`, measured, one direction only.** `tools/phaserun.sh 103-104` on q102
runs both edits and two sweeps and stops in phase 103's check on one message — *"the
output does not have exactly the twelve `#include` directives phase 99 left"*. Phase 103
states the twelve as a property it preserves and this phase takes `<stdio.h>`. There is
a second reason the run never reaches, and it is `apart 97 98`'s and `apart 102 103`'s
shape: phase 103 states its gone set as one `comm` against the **stage's** symbol
snapshot, a stage takes one snapshot at its start, so on a 103-104 stage its gone set
would be its seven plus these seven.

## What whim-vim is after twenty-one phases

```
whim-vim.c        80,173 lines          from whim-vim.c's 86,614  (-6,441, 7.4%)
functions         1,758
type definitions  904
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, every one a system header; no #define, no conditional
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            784,392 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**The 17, attributed — and a whole row of the table is gone.**

| why | symbols |
| --- | --- |
| **the terminal**, every one of them in the host block | `read` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` (6) |
| **the two writes** — `mch_write`'s in the core, `host_message`'s in the launcher | `write` (1) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals** — `sigaction` and `sigemptyset` in the host block, `kill` and `getpid` in both | `sigaction` `sigemptyset` `kill` `getpid` (4) |
| **gcc's own**, named nowhere in the source | `__errno_location` (1) |

**There is no row for "messages before there is a screen" any more**, and the five-strong
"gcc's own" row is down to one. `write` has a row of its own because it is the only one
here the **core** still does for itself as well as the host: `mch_write`'s
`write(1, …)`, which with `musl_read_input`'s `read(0, …)` is all of `GOALS.md`
§II.4c's remaining step.
