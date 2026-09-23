# Phase 124 — freeing is free, and the arena is measured

`phase/124/edit.go` and `phase/124/check.go`, `stage 124`, `package host`.
`host_alloc()` becomes a **bump allocator** into a fixed 1 GiB arena and `host_free()`
returns without doing anything. That is the charter bullet *A GARBAGE COLLECTOR IS
ASSUMED FROM HERE ON* built, and it is what makes the four phases after it cheap rather
than clever: the core may now allocate a record per line and simply not free it.

**The claim is "freeing is now free" and not "the core stopped freeing".** Every
`host_free()` call the core makes is still there and still made; a later phase may delete
them, which is the reason for doing this one first. Phase 118 moved `malloc` and `free`
across the line and wrote the two wrappers; this changes what is behind the two names and
nothing else.

## The strongest thing it says is a `cmp`, and it is a `cmp` of the core

The phase is host-only from end to end, so `make editor.c` — **77,681 lines, 2,064,232
bytes** — is **byte-identical in and out**. That subsumes every screen case, every
memline case, every Ex command, every command line and every pty scenario at once *for
the part of the file the project is for*, because the program a port would be handed is
literally the same text. It is the cleanest proof a phase is host-only that this pipeline
has, and it is a tier-1 check one level in from the whole binary. The recording is then
what says the **host** still answers the same way.

## The size could not have been measured one boundary earlier

The input built with a counter on `host_alloc()` that totals every request as the
allocator rounds it, dumped from `host_exit()`, over the whole of `tools/zrecord.sh`:
**268 sessions in 122 records, largest 200,458,672 bytes**, two recordings and the same
number. It is **one case**, phase 123's `mem_deep_jumps`, a 25,000-line buffer churned in
the middle; the next three memline cases are near 52 million, and the heaviest of the
**102 screen cases is 1,722,512**, which is **115 times less**.

So an arena sized from the 102 alone would have been sized from a corpus provably unable
to reach the text layer — the defect phase 123 exists to have ended, arriving one phase
later in a shape nobody predicted. **This phase found that out the hard way and the
record says so**: 64 MiB was written first and the recording refused it, *THE RECORDING
MOVED, in 1 of 122 records: memline/mem_deep_jumps*, the case dying with
`host arena exhausted: 67108864 bytes, 67058640 used, request 60263`, with 0 of 102
screen cases and 0 of the four sweeps moving.

1 GiB is **5.36 times** the measured high-water and 18.67 % used at its worst. The check
re-measures the high-water on its **own** output over both corpora every run and refuses
an arena less than four times it — and refuses as well if the heaviest memline case is
not heavier than the heaviest screen case, which is the lesson above written down as an
assertion rather than as a paragraph. So the size is a checked property and not a
remembered one, and a later phase that makes the memline allocate more is told by its own
check instead of by a crash.

## The phase overturns its own earlier reasoning, and the correction is worth more than the number

It first justified 64 MiB by *the abort path costs the arena times the harness's
concurrency* — "64 MiB across 64 threads is 4 GiB on a 62 GiB machine". **That is false
except for a runaway session**: an ordinary session's resident memory is its traffic,
which the arena does not change, and an untouched arena page costs nothing. Measured, the
same source at 64 MiB and at 1 GiB gives a **byte-identical image**, 772,872 either way,
because `.bss` is `NOBITS`. The size buys one thing, how far a runaway goes before it
dies loudly, and costs one thing, address space. The wrong argument and the measurement
that killed it are both in `phase/124/delta.md`, so the correction survives outside the git
log.

## And one measurement says why no number is safe

With nothing freed an arena holds a session's whole allocation **traffic** and not its
live data, and this editor's traffic is **quadratic in the length of a single line being
typed**: `+normal 200000ax` asks for 20,013,114,624 bytes across 400,475 calls, and
`+normal 500000ax` for 44,075,360,179. Phase 118's own by-hand probe was that command, so
a later phase that writes one like it will hit the wall. Buffers are linear and cheap by
comparison — 100,000 lines cost 12,862,224 bytes and 300,000 lines 35,157,264, about 112
a line. **It is churn and not size that fills an arena.**

## The real cost is not the arena, it is the resident memory

A bump allocator makes a session's peak RSS equal to its traffic. Measured with
`getrusage(RUSAGE_CHILDREN)` over a whole `tools/zmemline.py` run: the worst child peaks
at **13.6 MiB on the input and 191.8 MiB here**, fourteen times more, at either arena
size. Across a whole `make whim-verify` — 42 units at once — the peak is **13.7 GiB of a
62 GiB machine against 13.1 GiB** measured the same way on the boundary before it: 591
MiB and 4.4 % more, with 41 GiB still available. That is the charter's trade under
harness concurrency, on the record for the phases behind it, and it is the number to
watch as phases 125 to 128 change how much the memline allocates.

## Two of the four rewrites are not in the allocator, and without them the phase is wrong

The formatter's private island — the functions phase 110 moved below the includes because
they need `va_list` — still called libc's `free()` and libc's `realloc()` directly, on
pointers that came from `alloc_clear()`, which is to say from `host_alloc`. Phase 118 named
one in its own program (*"and `format_overflow_error()` below the boundary"*) and phase 117
named the other (*"`realloc` call is the host's and is not this phase's"*). **Both were
right while `host_alloc` WAS `malloc`**: the two allocators were one allocator. From this
phase a `free()` or a `realloc()` of an arena pointer is undefined, so they move — the
`realloc` by phase 117's own allocate-copy-free with phase 117's three traps read off this
site — and the byte-identical cut is what proves the phase did it without touching a core
line.

Neither is reachable by any recording and one cannot run at all, so the check owes a
probe and runs one: `adjust_types()` grows `*ap_types` only for a format string carrying
a **positional** spec, and not one string literal in this file has one, so the same
driver built into the input and the output runs six ascending positional formats through
it, enters the grow arm **17 times in each**, and the two binaries print the same bytes.
`format_overflow_error()` cannot be probed because it cannot run — its guard is
`overflow_err`, which is `tvs != nullptr`, and `vim_vsnprintf_typval()` has one caller in
this file passing `nullptr`. That is phase 92's and phase 100's kind, and it is kept correct
rather than left to rot.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,592 | **79,660 (+68)**, every one below the first `#include` |
| `make editor.c` | 77,681 | **77,681**, byte-identical, 2,064,232 bytes |
| `nm -u` | 17 | **14**, the set moving by exactly `free malloc realloc`, a `comm` empty the other way |
| `.bss` | 24,600 | **1,073,766,424** |
| binary | 781,064 | **772,872 — smaller**, because `.bss` is `NOBITS` and musl's allocator left the link |
| arena high-water | | **200,458,672 bytes**, one case, 5.36x under the ceiling |
| worst child RSS | 13.6 MiB | **191.8 MiB** |
| records that moved | | **0 of 122**, two full recordings byte-identical |

It is the **first Part II phase since 28 to free a symbol, and it frees three**. The `.bss`
growth is the arena less 960 bytes, and the 960 is musl's own `__malloc_context` and five
smaller objects leaving with it. `EXEC`, no `INTERP`, no dynamic section and no
relocation are all still true of a gigabyte object.

**The controls, over both corpora.** `host_alloc` returning `nullptr` moves **122 of
122**. **The offset never advancing** moves 102 of 102 screen cases and 16 of 16 memline
cases, and it is the only one that tests the *allocator* rather than the wrapper: a
`host_alloc` that returned the arena's base for ever would pass the symbol check, the cut
and the size assertion. `host_free` doing nothing moves **0 of 102 and 0 of 16** — phase
118's own `cf` control re-run on this phase's input rather than a new claim, on a corpus
phase 118 did not have, and reported rather than hidden, because a leak is invisible to
this corpus too and it is `free` leaving `nm -u` that says the freeing changed. And the
guard, which no recording can take: a 256 KiB arena aborts with
`host arena exhausted: 262144 bytes, 113024 used, request 319968` and exits 1 — the
request being the screen, this editor's single largest allocation — while the identical
session on the real output is silent and exits 0.

**`<stdlib.h>` is now dead and it stays**, measured rather than argued: the output built
with the directive deleted is byte-identical, 772,872 either way. Eleven stays eleven,
on phase 96's precedent for declining — a phase that changes two things cannot say which
one a difference came from, and the removal is free for whoever asks for it.

## Its placement

`stage 124`, `package host 100 101 102 103 104 113 115 118 119 124`, because that is what the
package is: a thing the core did for itself becomes a thing it asks the host to do. Here
it is one step further — the core already asked, and what changes is the answer.
`tools/zhostonly.py` needed no new word and no new exception: `host_alloc` and `host_free`
have sat below `musl_suspend()`'s brace since phase 118, and none of `malloc`, `free`,
`realloc`, `max_align_t` or `alignof` is in its vocabulary.

**`apart 124 125` is measured and is not the mechanism the phase predicted.** It expected
`apart 97 98`'s shape, an undefined-set equality against a stage's one snapshot. What
actually fires is **this phase's own promise**: `tools/phaserun.sh 124-125` on q123
stops with *the text above the first `#include` is not byte-identical in and out, and
this phase is entirely below it*, and again on the directives' line numbers. **A phase
that promises to touch no core line cannot share a stage with one that deletes 366 of
them.** The other direction is reasoning: phase 125's check compares the undefined set of
the text its own edit was handed with its output, and on a shared stage phase 124's edit
has already taken the three symbols by then, so it would pass.

**There is no `need 125`, for a reason stronger than one stage's measurement**: phase 124's
**sweep is a no-op** — its edit's output on q123 is byte-identical to q124 — so there is no
unswept text for phase 125 to be handed at all. `make whim-verify` is 42 of 42.
