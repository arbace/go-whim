# Phase 117 — the core stops reallocating

`phase/117/edit.sh` and `phase/117/check.sh`, `stage 117`, `package boundary`.

**`realloc` cannot be implemented from `malloc` and `free`**, and that is why this phase
is a rewrite of two call sites rather than a seventeenth vendored function beside phase
97's sixteen. To move the old contents it has to know how many bytes the old block held,
and its interface — `void *realloc(void *p, usize n)` — does not carry that number: musl
reads it back out of the **chunk header below the pointer**, which is a fact about musl's
heap and not about C. There is no `musl_realloc` that can be written at all, because
there is nothing to give the copy for a length. The only route left is each call site
with the size **it** knows, and the phase exists because both core sites know it.

`ga_grow_inner()` already computes its own — `old_len = (usize)gap->ga_itemsize *
gap->ga_maxlen;`, on the line after the call, to zero the new tail; the rewrite hoists
that line above the allocation and copies exactly it. `get_keystroke()`'s is `buflen`
before the `buflen += 100;` immediately above the call, and the rewrite saves it as
`t_buflen` beside the `t_buf` the input already saves, so the two halves of what
`realloc`'s interface does not carry — the old pointer and the old size — sit on adjacent
lines. `void *realloc(void *p, usize n);` leaves the core's libc prototype block, **six
lines to five** — a number the check COUNTS from its input rather than states, because
phases 114 and 115 shrank the same block just before this one.

**The symbol does not leave, and that is stated as an equality because a reader will
expect otherwise.** `nm -u` is 17 names before and 17 after, the same set as a `comm`
empty in both directions, with `realloc` still among them: the third call site is
`adjust_types()`, in the formatter island phase 110 moved below the first `#include`,
which is the host's and keeps it. Phases 97, 98 and 104 each require `realloc` to be
undefined and all three still pass. Phase 111 said the same of `gettimeofday` and phase 113
of the four stdio names.

## The four traps are memory bugs and not differences, so the phase owes a harness

A recording cannot see a leak, a double free, a premature free, or an overread whose
bytes are overwritten before anything reads them. `phase/117/check.sh` extracts
`ga_grow_inner()`, `musl_memcpy()`, `musl_memset()`, `garray_T` and `get_keystroke`'s
extension block **from the input source and from the output at run time**, drops both
into the same AddressSanitizer driver, and drives eight doublings from an empty
growarray, six independent first grows, a failed allocation and the 100-byte extension.
The two transcripts are identical, 32 lines, and neither reports a finding.
`B.fail r=0 same=1` is trap 2: the allocation failed, `ga_data` is the block it was, and
the grow after it reads that block back intact.

**Six of seven controls move**, each with its own named finding rather than one bucket:
copying `new_len` instead of `old_len` is a heap-buffer-overflow **read** — and it is
invisible to any recording, the overread bytes landing where the `musl_memset` that
follows overwrites them; freeing the old block on the failure path is a
heap-use-after-free at the grow that follows; `get_keystroke` copying `buflen` is a
heap-buffer-overflow; not freeing the old buffer is a LeakSanitizer report; freeing it on
both paths is a heap-use-after-free. Copying **nothing** gives no sanitizer finding at
all and is caught by the transcript, 84 bytes lost.

**The seventh moves nothing and is reported rather than hidden.** Dropping the
`if (gap->ga_data != nullptr)` guard gives a byte-identical unit transcript and no
finding, because a null `ga_data` implies `ga_maxlen == 0` implies `old_len == 0`,
`musl_memcpy` is a plain `for (; n; n--)` loop that never dereferences, and
`free(nullptr)` is a no-op. The guard is kept for what `GOALS.md` §II.4c asks of the
core — that its meaning be on the page, not in what a compiler or a libc happens to
tolerate — and the check proves what it buys with a driver whose `musl_memcpy` announces
a null source: **0** from the output, **8** from the unguarded control.

**Shrinking was measured, not assumed**, because copying the OLD size is wrong if either
site can ask for less than it has. Neither can: `ga_grow_inner`'s only caller enters it
when `ga_maxlen - ga_len < n` and the three statements above the allocation only raise
`n`, `get_keystroke` adds 100 immediately above the call, and an instrumented build marks
a shrink at **0 of 106 records**. The input's own
`musl_memset(pp + old_len, 0, new_len - old_len)` already relied on it, the length being
unsigned.

## The declared delta is nothing at all, and here that is the STRONG kind

Not phase 92's (code that could not run), not 95's or 96's or 113's (code the instrument
cannot see), not 99's or 106's (a byte-identical binary), and not 112's (different answers
no record holds). `ga_grow_inner()` is on the path of every growarray in the editor: the
same instrument inserted at a line both sources have counts **4,289** calls per recording
on the input and **4,289** on the output, **2,739** of them with `ga_data == nullptr` —
trap 1 is the majority case and not an edge — in 104 of the 106 records, the two that do
not mark being `ref-pty.txt` and `ref-term.txt`. Two full recordings are byte-identical,
and the control that keeps the rewrite and copies nothing moves **102 of the 102** screen
cases.

`get_keystroke`'s extension is the opposite and the check says so: it is **unreachable**
in a recording. An instrumented build of the input marks each of the five `continue`
paths inside its loop at 0 of 106 records, so `len` never exceeds one `ui_inchar()` and
`maxlen` never falls below 10, and a pty session feeding a partial escape sequence sixty
times does not reach it either. The unit harness is the only instrument that can drive
it, and it drives both versions.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,776 | **79,786 (+10)** — +5 at `ga_grow_inner`, +6 at `get_keystroke`, −1 for the prototype |
| `make editor.c` | 77,889 | **77,899**, same fourteen boundary names, compared at run time |
| the libc prototype block | 6 entries | **5** — counted from the input, not stated |
| `nm -u` | 17 | **17, the same set**, `realloc` still among them |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, and not `cmp`-identical** |
| the ASan unit transcript | 32 lines, no finding | **32 lines, identical, no finding** |
| controls that move | | **6 of 7**, each with its own named finding |
| records that moved | | **0 of 106**, with 4,289 calls per recording behind it |

## Its placement

`stage 117`, `package boundary 106 108 109 110 111 117` — because the phase's product is one
line fewer in the block phase 109 wrote and phase 110 carried. **Not `vendor`**: nothing is
vendored here, this being the case where the core stops needing a libc function that
*cannot* be vendored. Three `uses`: `seed:83` and `harness:86`, and `vendor:97`, because the
copy both rewrites make is `musl_memcpy`, phase 97's static definition — which is also
why the null guard buys nothing measurable.

**Three `apart` lines, each measured rather than predicted.** `apart 109 117` and
`apart 110 117`: both of those checks assert `void *realloc(void *p, usize n);` is on a
line of its own exactly once, and it is not any more — run against this phase's output
each reports that one prototype and only that one. `apart 113 117`, both directions,
measured when the two were adjacent: on a shared stage phase 113's check stops with *"the
output is 79776 lines and the input was 79857, a difference of 81 where 91 was
expected"*, exactly the ten lines this phase adds, and this phase's check stops with the
mirror image, *"a difference of −82 where 10 was expected"*.

**The pair that would normally need measuring — 116 and 117 — cannot have an `apart` at
all, and that is itself a measurement.** A stage of more than one phase is made of split
programs and `phase/116/make.sh` is ONE file, so the schedule is refused before any check
runs: `stage 116-117` gives `phase 116 is in stage 116-117 but is not an edit and a check`
from `tools/stages.sh`, and `tools/phaserun.sh` refuses the same unit with `phase 116
has no edit and check to run`. **`need 117` is measured not to be required** — the edit was
run on phase 113's unswept output, 79,858 lines against the swept 79,776, and every anchor
and every count held — and it could not be exercised anyway, a phase whose predecessor can
never share its stage being handed a boundary either way.
