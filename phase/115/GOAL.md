# Phase 115 — the clock crosses the boundary

`phase/115/edit.go` and `phase/115/check.go`, `stage 115`, `package host`.
The core read **two** clocks and only one of them had crossed. Phase 111 gave the
elapsed-milliseconds clock to the host as `long musl_now_ms(void)`; the wall clock
stayed behind as `static time_T vim_time(void) { return time(nullptr); }`, with five
call sites, `long time(long *tp);` in the core's own libc prototype block, and **two
more reads that bypassed the wrapper altogether** inside `ui_focus_change()`. Two steps,
in order: those two become `vim_time()`, and the wrapper then moves below the first
`#include` as `host_time()`, declared in the core's host block beside `host_exit` and
`host_message`.

Afterwards **the core does not name `time` at all** — four mentions in the input's core
to none, counted on the **literal-stripped** text because two `NGETTEXT` strings in
`op_shift()` say the English word and a count that read those would be counting English.
`host_time` is 8 above the boundary (the declaration and seven call sites, the input's
five plus the two that bypassed the wrapper) and 1 below. The libc prototype block goes
**7 entries to 6**, losing `time` and nothing else — `malloc realloc free getpid kill
write` — phase 114 having vendored `abs` and `labs` out of it immediately before. The
block is found by its **shape**, a run of non-blank lines around a line already required
to be unique, so phase 114 landing under this phase cost it no edit at all. The file loses
**one line**: the core loses 8 and the host gains 7, measured 77,703 → 77,702. The two
halves used to cancel exactly. What moved is the forward declaration `static time_T
vim_time(void);`, which now costs TWO lines and not one — the canonical text writes a
blank line between forward declarations where the residue wrote them consecutively, so the
one that goes takes its blank with it, and the declaration that arrives joins the host
block, which already had its own separators.

## The prototype never pinned `time_T`, and that corrects something written down twice

Phase 109's commit and the brief for this one both say that `typedef long time_T;` is
correct because `long time(long *tp);` sits above `<time.h>`'s declaration of the same
function, where gcc compares the two. **Half of that is true and the important half is
not**, and the `m2` compile is what says so: the input with `time_T` changed to `int`
and the prototype **left alone** compiles in **silence**. The prototype pinned
`long == time_t`; nothing ever checked `time_T == long`. So this phase does not preserve
a guarantee, it **replaces a weaker one with a stronger one**:

```c
    static_assert(_Generic((time_T)0, time_t: 1, default: 0), "time_T is time_t");
```

beside the twelve constants phase 110 put below the includes, **which is the only place
in the file where a core name and a header name are both in scope**. Four compiles, all
in the check. `m1`: the input with the prototype written `int time(int *tp);` is
`conflicting types for 'time'` — that *was* the guarantee, and it is a real one. `m2`,
as above: silent. `p1`: the output with the assert deleted and `time_T` perturbed
compiles in silence too — **the regression this phase would otherwise have shipped, and
the reason the prototype could not simply be deleted**. `p2` and `p3`: with the assert
present, `int` and `long long` both give `static assertion failed: "time_T is time_t"`.
The one line names `time_T` itself, which the prototype could not.

## `host_time()` returns `long` and not `time_T`, which is a decision

Its definition is below the boundary and `time_T` is a core typedef above it, so the
host half could not name it once the file is cut at the first `#include`.
`musl_now_ms()` returns `long` for that reason and this is its sibling — the two halves
of the clock now cross in the same shape, and the boundary's stated property that every
core → host signature takes scalars and byte buffers only survives a **fourteenth**
name. Nothing is converted at any call site, `time_T` being `long` on the page, and the
check asserts the typedef line itself.

## The recording is the weakest part of the evidence, and the check says so

Two full recordings are byte-identical across all 106 records — but **not one of the 102
screen cases reaches `ui_focus_change()`**, which is the only function whose reads this
phase respells in place. So the phase owes an instrument, and it has two.

An **instrumented pair**: `write(2, "TICK\n", 5)` at every clock read on each side —
three sites on the input (the wrapper, and `ui_focus_change`'s two, as comma expressions
so that the tick is exactly where the read is), one on the output, because afterwards
there is only one — with the two instrumented 102-case recordings required to be
byte-identical. They are, and the instrument is not silent: it marks **100 of 102 cases
with 410 reads** in all, the two it misses being `ctrl_c_clean` and `ctrl_c_changed`,
which exit before a key is looked up.

And **focus probes**, because a keystroke file *can* reach `ui_focus_change()`: `\033[I`
and `\033[O` are `KE_FOCUSGAINED` and `KE_FOCUSLOST`, and `set_termname()` registers
both unconditionally — **`GOALS.md` §II.2l's hazard, that a typed Escape followed by
`[` is read as a key code, used deliberately**. `\033[O \033[I` reads the clock 3 times
and `\033[O \033[I \033[O \033[I` reads it 4, identically on both binaries; the
arithmetic is 0 + 2 + 0 + 1 at the four calls plus one for the `:q!`, `focus_state`
starting MAYBE so that the first FocusLost reads nothing and the first FocusGained finds
`last_time` at 0 and takes both reads. **The two reads are still two reads**, in the
same two statements and the same order, so they straddle a second neither more nor less
often than before — and the control is that question made into a program: `hoist` is the
output with the two reads collapsed into one local, and it gives 5 on the second probe
where the product gives 4. `focus` does **not** separate them (3 either way, by a
different route), which is why there are two probes and the check says which one is
load-bearing.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 77,703 | **77,702** — the core loses 8 and the host gains 7. Re-measured on the canonical text; the rest of this table is not |
| `time` named in the core | 4 | **0**, on the literal-stripped text |
| the libc prototype block | 7 entries | **6** |
| `make editor.c` | 77,896 | **77,889**, 0 directives, 0 errors |
| the boundary | 13 names | **14**, `host_time` arriving and nothing gone |
| `nm -u` | 17 | **17, the same set** — `time` does not leave, and that is said as an equality |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, and not the same bytes** |
| records that moved | | **0 of 106**, with an instrumented pair and two focus probes behind it |

## A hazard this phase found in the shared recording, recorded and deliberately not worked around

Comparing two **full** recordings is load-sensitive in exactly three records, and the
clock phase is the one that would notice. `tools/zrec.py` scrubs the undo message's
elapsed time to `<ago>` **padded to the width it replaces**, so the *screen* is
protected — but the record also carries `--- stream <len> sha=<…>`, taken over the
**raw** byte stream, where `0 seconds ago` and `1 second ago` are 13 bytes and 12.
Measured with a control built for it — `add_time()` reporting one second more — exactly
three records move, `undo_after_ins`, `undo_block` and `undo_redo`, which are exactly
the three whose screen carries `<ago>`, and in each exactly **one line** moves, the
`--- stream` line, with all 24 screen lines byte-identical. One 33-way concurrent `make
whim-verify` failed here on `undo_after_ins` alone. **It is not this phase's to fix** —
hashing the scrubbed stream in `tools/zrec.py` would re-key all 33
phases and require `.reference/core-baselines` to be recorded again — **and not this
phase's to paper over either**, a private exclusion being a check narrowed to fit what
it saw. The comparison stays an exact `diff -rq` and the hazard is written into the
check's header. What matters for this phase is that the exposure is **unchanged** by it,
which is what the instrumented pair measures.

**AND HASHING THE SCRUBBED STREAM WOULD NOT CLOSE IT, which phase 123 measured
afterwards and which this paragraph got wrong.** The scrub rewrites the age's TEXT,
padded; the leak is *arithmetic on that text's width*. An undo reports its age and the
editor then positions the cursor to clear the line, so `0 seconds ago` emits
`\033[24;40H\033[K` and `1 second ago` emits `\033[24;39H` — a column derived from a
scrubbed string's length, in a byte sequence the scrub never touches. It failed zero
phase 99, whose binary is byte-identical either side, which is the only reason it was
catchable at all. The fix that does work is phase 123's: record `stream N redraws`, a
count of `\x1b[?25h`, instead of a digest, and let a clock control carry the evidence —
both readable clocks replaced by runaway counters move 0 of 16 memline records against
9 of 102 screen cases, which says the record does not depend on the clock AT ALL, and a
digest never could. `tools/zmemline.py` does this; `tools/zcases.py` still digests the
raw stream, and the change is expensive rather than hard: the `--- stream` line is named
in eighty files, forty-seven times in phase 95's check alone.

## Its placement

`stage 115`, `package host 100 101 102 103 104 113 115`, because this is what that package is:
a thing the core did for itself becomes a thing it asks the host to do, declared in the
one host block and defined below the boundary — `host_exit` (19), `host_message` (21),
`host_time`. It is deliberately **not** `boundary`, which *draws* the line (106, 108, 109,
110, 111); this phase moves one function across a line already drawn. Five `uses`:
`seed:83` and `harness:86`; `boundary:109` for the prototype and the typedef it replaces;
`boundary:110` for the only place the `static_assert` can be written; and `boundary:111`
for `musl_now_ms`, whose shape and whose `nm -u` sentence this phase takes.

**Four `apart` lines.** Three are one fact measured three ways, phase 113's method of
applying each check's own assertion directly to the tree this phase leaves: 109 and 110
both carry the nine-entry `PROTOS` list and now find **three** of the nine at 0 — `labs`
and `abs`, already phase 114's, and `time`, which is this phase's — and 110 and 111 both
**write out** the boundary as thirteen names where this phase makes it fourteen, the
symmetric difference being exactly `{host_time}`. The fourth was measured with
`tools/phaserun.sh 114-115` on q113: `apart 114 115`, one direction only, where phase
114's check stops on three messages — the block losing `time` as well as `labs`/`abs`,
the core at a difference of 3 where 10 was expected, and *"the host changed size, and
this phase does not touch it"*. **No `need 115`**, measured in the same run.
