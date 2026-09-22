# Phase 109 — the header types and macros the core can own

`phase/109/edit.go` and `phase/109/check.go`, `stage 109`, `package boundary`.
Eight things the core took from a header stop coming from one:

```
    time_t          →  typedef long time_T;  plus  long time(long *tp);
    sig_atomic_t    →  volatile int                       2 in the core; the host's 3 stay
    uintptr_t       →  usize                              1 site
    struct timeval  →  a TAGLESS struct of two longs, and musl_gettimeofday(long *, long *)
    MIN / MAX       →  the text the preprocessor gives, at 19 lines
    offsetof        →  __builtin_offsetof                 9 sites
```

and the nine libc functions the core still calls — `malloc realloc free time getpid kill
write labs abs` — get plain prototypes, none of them `static`. 80,173 → 80,197 lines,
exactly the 24 the edit adds.

## It comes BEFORE the move, and the ordering is the whole of its evidence

Every declaration here replaces something a header **still supplies from above it**, so
the ordinary build cross-checks each one for free. After the move there is nothing left
to check against, and the check that can be written then is only *it compiles*.

**Sixteen `static_assert`s**, all silent, compare every core-owned spelling against the
header type it replaces:

```c
    sizeof(elapsed_T) == sizeof(struct timeval)              and both offsets, both widths
    _Generic((usize)0,  uintptr_t: 1, default: 0)            and again against size_t
    _Generic((time_T)0, time_t:    1, default: 0)
    _Generic((int)0,    sig_atomic_t: 1, default: 0)
    _Generic(time, long (*)(long *): 1, default: 0)
    __builtin_offsetof(T, m) == offsetof(T, m)               at all six types the 9 sites use
```

Every one of them **names a header type**, so not one can be written once phase 110 moves
the includes below. They are a control in the check and never in the product, and a
seventeenth with one comparison deliberately wrong must fail — so they are compiled and
not merely present.

**Four deliberately wrong declarations give four diagnostics**, which is the same
argument from the other side: `int time(int *tp);`, `void *malloc(int n);` and
`long getpid(void);` are each `conflicting types`, and `static void *malloc(usize n);` —
the trap the survey names — is `error: static declaration of 'malloc' follows non-static
declaration`. After the move the first three become **nothing at all** and the fourth
changes shape entirely. One more argument for doing this first, and phase 110 measured
where the fourth went.

## Six of the seven changes are tier 1, and the check says so as an equality

Six of them are a rename or an expansion the preprocessor was already performing, so
none can generate a different instruction. That is stated rather than claimed: **with the
clock ALONE reverted, the binary is `cmp`-identical** to the 788,488-byte one the phase
was handed. So `time_T`, `volatile int`, `usize`, the 23 `MIN`/`MAX` expansions,
`__builtin_offsetof` and the nine prototypes are `CLAUDE.md`'s tier 1 — literally the
same program — and only the clock has anything to answer for.

**The seventh is the clock, and it is the only libc *type* the core could not rename
away.** `struct timeval` is a **layout**, so `elapsed_T` becomes the core's own tagless
`struct { long tv_sec; long tv_usec; }` and the five `gettimeofday(&X, nullptr)` calls go
through a new `musl_gettimeofday(long *, long *)` in the host block. That is a call and
two stores where there was a syscall wrapper, so the binary moves, and **two full
recordings, `diff -r` empty across all 106 records**, are what answers for it.

The recording is not blind to it: `musl_gettimeofday` writing its two fields the wrong
way round moves **six of the 102 screen cases** — `key_Q`, `key_gQ`, `key_gf`, `macro_q`,
`reg_percent` and `ruler_move`.

## `MIN` and `MAX` are read from the header, not written into the program

The edit sends `MIN(ZZA,ZZB)` and `MAX(ZZA,ZZB)` through the preprocessor with
`<sys/param.h>` included and turns what comes back — `(((ZZA)<(ZZB))?(ZZA):(ZZB))` —
into its template. That is the only honest meaning of *the exact text the header gives*,
and the check re-derives the same two shapes and requires 7 more `MIN` expansions and 16
more `MAX` ones, with the repeated argument really repeated. **Two of the 19 lines pass a
call as an argument and so evaluate it twice** — exactly as the macro did, which is what
the `cmp` proves and what writing `<` by hand would have quietly fixed into a different
program.

## Two corrections to the brief, both measured, and both make the rule stronger

**A core-defined `struct timeval` tag is NOT a hard error.** The brief says it is
`error: redefinition`. Measured: **C23 permits a struct to be redeclared with the same
members**, so gcc 15's default dialect is *silent* both ways round, and only `-std=c11`
and `-std=c17` refuse it. The tagless struct is still mandatory, for a better reason than
a diagnostic — **the core must not define a libc tag at all**, and the layout equality
has to be **asserted**, which is what three of the sixteen `static_assert`s do. A rule
that rests on a diagnostic the standard has since removed is a rule with a shelf life.

**There is no `musl_time`.** The brief has the core calling its own
`long musl_time(long *)`. `time` keeps its name, so that `long time(long *tp);` sits
above `<time.h>`'s own declaration of the same function and gcc compares the two, where
a wrapper would have cast any mismatch away at its own boundary.

**AND WHAT THAT PROTOTYPE PINNED IS NOT WHAT THIS SECTION SAID IT WAS.** It read
*"precisely what pins `time_T`'s width"*, and so does this phase's commit; **phase 115
measured it and both are wrong**. The prototype pinned `long == time_t` — real, and
phase 115's `m1` compile re-ran this phase's own control to confirm it, `int time(int
*tp);` giving `conflicting types for 'time'`. Nothing in it ever mentioned `time_T`, and
phase 115's `m2` is the proof: **this phase's output with `typedef long time_T;` changed
to `int` and the prototype left untouched compiles in SILENCE.** What checked
`time_T == time_t` here was `_Generic((time_T)0, time_t: 1, default: 0)`, one of the
sixteen `static_assert`s above — a **control in the check**, never in the product —
which is exactly why phase 115 had to
put `static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");`
into `whim-vim.c` when it took the prototype away.

## The tool changed, and that is the tool working

`tools/zhostonly.py` refused, as predicted — its two `struct timeval` exceptions read
*"the clock's, and not this phase's … somebody else's later phase"*, and this is that
phase; `kill` acquires one for the core's own prototype. But a swapped exception list was
not enough. The tool runs in the checks of phases **20, 104, 108 and 109, each on its own
output**, and the core's vocabulary shrinks between them — so an exception's count is now
the **tuple of values it takes**, each written with the phase that made it true. What it
refuses is still a count nobody has written down, so *an exception that stops being true
is a fact this tool is meant to notice* holds, and the phase that ends one comes here and
says so. Measured with `tools/implhash.sh`: 107 whim and slim keys identical either side,
six Part II keys move — the units and edits whose programs name the tool.

## A control can be right and still be a bad check

The must-differ control — `musl_gettimeofday` writing its two fields the wrong way round
— **stalls `tools/zpty.py`**. An editor whose clock runs backwards has timeouts that
never expire, so the harness waits out its deadline, writes `stalled` and exits 1 two
minutes later, which is correct behaviour on a deliberately broken binary and a flaky
check. The control is `zcases.py` alone for that reason. **A check should not depend on
how long a harness takes to give up.**

## The declared delta is nothing at all, and it is a sixth kind

Not code that could not run (92, 100), not code the instrument cannot see (12), not a
possibility removed (13), not a `cmp` of the binary (99, 106, 107), and not phase 108's *the
code runs and the instrument sees it do the same thing* either. It is **six of seven
changes that are a `cmp` and one that is a recording**, and the phase separates them
rather than taking the weaker evidence for all of it.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 80,173 | **80,197 (+24)** |
| `time_t` / `time_T` | 6 / 5 | **0 / 10** |
| `sig_atomic_t` | 5 | **3**, all three the host block's |
| `uintptr_t` | 1 | **0** |
| `offsetof` / `__builtin_offsetof` | 9 / 0 | **0 / 9** |
| `MIN(` + `MAX(` | 7 + 16, on 19 lines | **0**, expanded in place |
| `struct timeval` | 6 — 4 core, 2 host | **3**, all host |
| `gettimeofday` | 5 | **1**, inside `musl_gettimeofday` |
| libc prototypes in the core | 0 | **9**, none `static` |
| functions | 1,756 | **1,757** (`musl_gettimeofday`) |
| type definitions / DWARF enumerators | 905 / 1,177 | 905 / 1,177 |
| `nm -u` with the core's flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488** — and `cmp`-identical with the clock alone reverted |
| records that moved | | **0 of 106**; the wrong-way-round clock moves 6 of 102 cases |
| sweep | | takes nothing, canon a no-op |
| phase | | **27 s** |

## Its placement

`stage 109`, `package boundary` beside 106, 108 and 110. Its `uses` are
`boundary:109 seed:83 mechanical` and `boundary:109 harness:86 mechanical` for the recording;
`boundary:109 host:103 mechanical`, because `musl_gettimeofday` is **defined inside the
host block phase 103 created**, immediately above `musl_delay` — `tools/zhostonly.py`
reads the host region as the lines from `host_winch_pending` to `musl_suspend`'s last
brace, so a definition below `musl_suspend` would put a `struct timeval` outside it and
the tool would refuse; and `boundary:109 host:102 rationale`, because the two
`sig_atomic_t` this phase respells are the core's and the three it leaves alone are the
host block's — the split that makes that sentence meaningful is the launcher phases 101
and 102 put at the bottom of the file.

**`apart 108 109` is measured, not predicted.** `tools/phaserun.sh 108-109` on q107 runs
both edits, one sweep and then **phase 108's** check, which stops at *"the file is 80197
lines and the input was 80178 (80178 recorded) — expected exactly five fewer"*: this
phase adds twenty-four lines to the text before that check reads it. **`need 109 swept` is
measured NOT to be required**, in the same run — both edits came from the edit cache,
which is keyed on the digest of the tree each is handed, and a direct `cmp` confirms it:
phase 108's edit applied to q107 gives a `whim-vim.c` byte-identical to q108's, so the sweep
between them is a complete no-op.

## What whim-vim is after twenty-six phases

```
whim-vim.c        80,197 lines          from whim-vim.c's 86,614  (-6,417, 7.4%)
functions         1,757
type definitions  905
DWARF enumerators 1,177
header names left above the host block   the twelve *_MAX, PATH_MAX, EXIT_FAILURE,
                                         SIGHUP and SIGTERM -- and nothing else
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
```

**Recorded as a follow-up rather than done here.** Every core use of the clock is *stamp
now, then ask how many milliseconds have passed*: `elapsed()` is one `gettimeofday` and a
subtraction, and its four callers all compare the result against a millisecond count. So
the core never needs the **layout**, only a scalar, and a `long musl_now_ms(void)` would
take `struct timeval`, `gettimeofday` and the tagless-struct question out of the core
together. The tagless struct is an interim shape, kept because it is what was surveyed
and what the evidence above was measured against; the scalar clock is a phase of its own,
if it is asked for.

**Its product landed separately too**, like phase 108's: the branch and the merge hold the
programs, and `whim-vim.c` came in the commit after.
