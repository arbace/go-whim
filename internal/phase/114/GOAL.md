# Phase 114 — `abs` and `labs`, the two the core took on trust

`internal/phase/114/edit.go` and `internal/phase/114/check.go`, `stage 114`, `package vendor`.
**The core is optimised for transpilation, not for performance, and so it may not depend
on latent compiler behaviour** (`GOALS.md` §II.4c, the user's rule). This is the first
application of it, and by every number this pipeline usually reports it does nothing:
`nm -u` is the same 17 names either side, as a `comm` empty in both directions.

**That is the phase.** `abs` and `labs` were **called** by the core, at three sites, and
were in the undefined set **zero times** — measured here, the input's whole assembly
(`gcc -S -O0`) mentions neither name, because gcc lowers both to inline arithmetic.
Nothing in the language promises that. A compiler that emitted the calls the source
literally asks for would have added two libc symbols to a file whose whole claim is the
shortness of that list, **and nothing in the pipeline would have said so until it
happened**. So the phase frees nothing and says so as an equality; what it removes is a
dependence on behaviour nothing states.

Two prototypes leave the core's libc declaration block, which phase 109 wrote and which
goes **9 entries to 7** — found by its *shape*, a contiguous run of top-level
declarations above the first `static`, and never by line number. The three call sites
become `musl_abs` and `musl_labs`, by the literal-aware single pass `CLAUDE.md` asks
for. And two definitions land in the `musl_` block phases 97 and 98 built, immediately
above `musl_bsearch`, so the four `<stdlib.h>` functions the core owns — `musl_atoi`,
`musl_atol`, `musl_abs`, `musl_labs` — sit together and above every use.

## musl's spelling is copied and not improved, and the undefined behaviour with it

`/root/musl/src/stdlib/abs.c` and `labs.c` are one line each, `a>0 ? a : -a`, and the
check proves that choosing it **costs nothing** rather than arguing it: `a > 0 ? a : -a`
and `a < 0 ? -a : a`, both taken out of the output, compile to byte-identical machine
code at `-O0` and at `-O2`, and agree at all 4,294,967,296 `int` values and at
20,000,006 `long` ones including `LONG_MIN` and `LONG_MAX`.

`-a` overflows at `INT_MIN` and at `LONG_MIN`, so both vendored functions are undefined
there — **and so are libc's, by the same expression, and so is musl's own source**. The
pair is *faithful rather than safer*: a phase that quietly made the core's arithmetic
differ from the libc it replaces would be a behaviour change wearing a vendoring phase's
clothes. What is measured instead is whether the three sites can be driven there, and
they cannot — `last_status_rec`'s two operands are window heights, which
`limit_screen_size()` clamps at 1,000 rows, and the two `labs` arguments are differences
of line numbers, so `LONG_MIN` needs a buffer of 2^63 lines. Instrumented, the largest
magnitude any of the three is ever handed over 51 probe calls is **22**.

## What the image may do is a rule and not a coincidence

There is no `cmp` to be had — at `-O0` a call to a static function is a call and inline
arithmetic is not — so what is asserted is that the difference is **accounted for
instruction by instruction**: the object's `.text` grows by exactly **42 bytes**, which
is `musl_abs` (19) plus `musl_labs` (24) plus what the three callers gained or lost
(−2, +1, 0) **and nothing else**, and the linked image is 782,760 bytes either side with
604,650 of them different, which is what putting a definition near the front of a file
does.

**Only `.text` and `.eh_frame` change size** — no data section moves a byte, which is
the *this phase changes code, not data* claim — and neither changes by more than one
alignment unit. Which of the two happens is a property of the **input**, measured both
ways for the identical edit: 0 on the q112 tree and 64 here. Written as *"every section
but `.eh_frame` keeps its size and its address"* — true on q112 — the check **refused
this rebase**, naming `.text` and `.fini`, and that is the assertion working and the
phase being fine.

## The corpus cannot see this phase at all

Measured rather than assumed: the output built with a probe on each of the three
arguments enters **none** of them in 106 records, and its recording is byte-identical to
the product's. So the phase owes probes, and runs three, one per site — `+set rnu` with
sixty lines (46 calls, arguments −21 to 22), sixty long wrapped lines then CTRL-F CTRL-F
CTRL-B CTRL-B (3 calls), and `:set laststatus=2` then `=0` (2 calls). **Two of the three
are proven able to fail**, by a control whose `musl_abs` and `musl_labs` return their
argument unchanged. **`stl` does not, and the check reports it rather than hiding it**:
its site is reached twice and its answer guards only `w_prev_height = w_height`, which
`win_new_height()` already assigns on every path that changes a height. That site is
proven **reached** and not proven **observable**, and the equivalence above is its
evidence.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 79,766 | **79,776 (+10)**, all of it core |
| `make editor.c` | 77,886 | **77,896** |
| the libc prototype block | 9 entries | **7** |
| `abs` / `labs` in `nm -u` | 0 | **0** — they were never there, and that is the point |
| `nm -u` | 17 | **17, the same set** |
| object `.text` | | **+42 bytes**, accounted for instruction by instruction |
| binary | 782,760 | **782,760**, 604,650 bytes different |
| records that moved | | **0 of 106**, against three probes the corpus cannot reach |

## Its placement

`stage 114`, `package vendor 97 98 114`. Four `uses`: `seed:83` and `harness:86`, the corpus
reaching none of the three call sites so a probe here is a screen recorded from a
keystroke file; `boundary:109`, the two prototypes it deletes being that phase's; and
`boundary:110`, the check asserting that both definitions land **above** the first
`#include`, which is the boundary only because of the move.

**Both schedule declarations are measured, in one run.** `tools/phaserun.sh 113-114`
on q112 runs both edits, one sweep and both checks and stops in phase 113's — *"the output
is 79776 lines and the input was 79857, a difference of 81 where 91 was expected"* —
because the ten lines this edit adds land in the same swept text. That is `apart 100 101`'s
shape exactly, and it is one direction only. The same run measures that **`need 114
swept` is not required**, this edit applying unchanged to phase 113's unswept output with
all five anchors holding.
