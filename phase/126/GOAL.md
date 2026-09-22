# Phase 126 — a block number becomes a reference

`phase/126/edit.sh` and `phase/126/check.sh`, `stage 126`, `package memline 126 127
128`. `pe_bnum` and `ip_bnum` become `bhdr_T *`, `memline_T` gains `ml_root`, and
`mf_get(mfp, nr, page_count)` becomes `mf_get(mfp, hp)`. The hash table that turned an
integer block number into a page then has nothing left to look up, so it goes — with the
free list it was keyed alongside, with `mf_blocknr_max` that handed the numbers out, and
with `pe_page_count`, whose one reader in the whole file was the argument `mf_get()` no
longer takes. `blocknr_T` 13 → 0, `mf_hashitem_T` 18 → 0, `mf_hashtab_T` 14 → 0, eleven
functions, 180 lines.

**This is the phase that buys the port the most, and it is worth being precise about what
it buys.** An integer key into a side hash table becomes an **object reference**, which is
the one thing a JVM has and C does not make you say. `GOALS.md` §II.4d lists what a port
would have to be told about rather than translate, and the memline page is the whole of
that list; this removes the **outer** half of it, the indirection *between* pages. It does
**not** remove the inner half, and the phase says so rather than letting the headline
stand: the page is still a byte array, `db_index[1]` is still declared length 1 and
indexed to the block's line count, the fourteen `(char_u *)dp + start` interior pointers
are untouched, the top bit of an offset is still a flag, and the arena and its interior
pointers survive until phase 127. **A reference to a block whose innards are still a byte
array is halfway.**

## Why the lookup could not miss, which is the whole argument that a pointer is the same answer

Read off the text by the edit rather than asserted: the hash is inserted into by
`mf_new()` and `mf_get()` and removed from by `mf_free()` and `mf_get()`, which removes
and re-inserts in one breath to move a block to the head of the used list — so **every
live block has been in the hash since it was made**. Nothing has been written to a disk
since phase 89 and nothing could be read from one since phase 92, so there has never
been a block number in this build that named a page not already in memory.

No invariant broke, and both were looked for rather than assumed. **No block number is
stored anywhere else** — every mention of `pe_bnum`, `ip_bnum`, `mhi_key` and
`bh_hashitem` is partitioned by its owning function. **The hash provided no ordering
anything reads**: `mf_used_last` is write-only, there is no release path left, and one
`ml_root` suffices because a root split **preserves the root block's identity**.

## Four write-only fields go in the edit and not the sweep, and that is the rule rather than a choice

`tools/deadfields.py` takes a field named nowhere outside its own type and reports **0
fields** in this region, because every one of `mf_used_last`, `bh_page_count`,
`pe_page_count` and `pe_bnum` is **written**; gcc has no warning for a struct member in
either direction; and phase 103's trap is the other half — remove a member and leave its
initialiser and the compile says `excess elements in struct initializer`, which is a
correct phase failing. `mf_used_last` had been write-only since phase 125 took
`ml_setflags()`, its last reader. `bh_page_count` and `pe_page_count` become write-only
**here**, and that cascade was measured rather than predicted: with `pe_page_count`'s one
read gone, gcc reports `page_count_left` and `page_count_right` as
`-Wunused-but-set-variable`, which `tools/deadsweep.py` does not act on, so those two
locals are the edit's as well.

**Every assertion is a partition and not a count.** Phase 118 had to repair phase 117's
counted anchors, and phase 125 is this phase's direct predecessor and removes four fields
from the same two structs. So the edit asserts the **set of functions** that says each
name — `mhi_key` in eight places, `pe_bnum` in four, `ip_bnum` in five — and a name said
somewhere the phase does not account for refuses. The check states the difference the same
way: the names that leave in the edit (**47**), the names that leave in the sweep (**2**)
and the names that **arrive** (**6**) are three computed sets compared against three
written ones, over identifiers with string and character literals masked out first.

## It caught a lie the prototype would have shipped

`E323: Line count wrong in block %ld` is the only message in the file that printed a
block number, and there are none left. The survey's prototype passed `(long)0`, which
would have printed a falsehood for ever; the message becomes **`E323: Line count wrong in
block`**. The evidence is phase 92's shape: the input built with a latching marker on that
arm carries it in **0 of 122 records** and the identical marker one line above — the
descent into a pointer block — in **120 of 122**; then both sources are forced to take the
arm and both do draw it, `...in block 0` against `...in block`.

**How the arm is forced was arrived at by measurement, and two wrong ways are recorded
because each looks right.** Emptying the scan loop leaves `idx` at 0, `0 >= pb_count` is
false and the descent simply goes round again. Forcing the arm alone is not enough either:
`ml_find_line()` then returns `nullptr`, the editor dies of it — SIGSEGV, measured — and
the message never reaches the stream. What works is reading every entry's line count as 0,
which leaves `idx` at `pb_count`, and then making the arm draw and stop with `out_flush()`
and `host_exit(0)` right after the `iemsg`. That last edit is found by the **shape** of the
call and not its text, which is what lets one rule serve two sources that spell it
differently: the input formats a block number into `IObuff` and the output does not.

`E298: Didn't get block nr 0?` and `E298: Didn't get block nr 1?` are not changed but
**deleted**, with the two `ml_open()` tests that were the only thing that could raise them,
and both objects are left standing for `tools/sweep.sh` — which is the whole of what the
sweep does here, because the eleven functions that go all name a type or a field the edit
removes and leaving them would hand the sweep a file that does not compile.

## The declared delta is nothing at all, and it is the strongest instance of the sixth kind any Part II phase has had

The code runs and the instrument sees it do the same thing — and it is strongest here for
a reason about **where** the phase is rather than how careful it was: **every keystroke
this editor draws reaches its text through `ml_find_line()`**. Two whole recordings are
byte-identical across all 122 records.

**And it is only the sixth kind because phase 123 exists.** Before it a recording was 102
screen cases that allocate exactly one data block each, and a binary with one line deleted
from `ml_find_line()`'s pointer bookkeeping recorded every one of them byte for byte; a
phase that rewrites the descent, measured against that, would have been the **second**
kind and would have owed probes for the whole text layer. So the check does not merely
diff the recording: it plants five counters — root splits, pointer-block splits,
data-block splits, deepest descent, data blocks made — in **both** sources and requires the
sixteen memline cases to agree event for event, which they do.

**Five controls, and the fifth moves nothing and is reported.** `c_root` (the root test
made a test nothing passes) and `c_stack` (every stack entry remembers the root) move a
65,149-line session and leave a two-hundred-line one alone — and that two-hundred-line
session is not an easy target: 200 lines of 20 bytes already fill two data blocks, so it
descends through a real pointer block and still cannot see either control. `c_descend`
(every descent takes the first child) moves both, and `c_mlroot` (`ml_open()` never writes
`ml_root`) moves all three sessions, which keeps the finding from being a session nothing
could fail. `c_pages` (`mf_alloc_bhdr()` sizing every block one page) moves **nothing**,
and the reason is worth having rather than hiding: since phase 124 `host_alloc` is a bump
allocator with no redzone and no free, so a block written past its end scribbles on arena
bytes nothing has handed out yet. **A short allocation there is a memory bug and not a
difference** — the last row of `CLAUDE.md`'s verification table, the one that needs a
sanitizer and not an instrument — and it is built, run and reported, which is phase 117's
seventh control exactly.

## The pointer entry shrinks and the tree gets wider, which is the thing a later phase has to know

Derived by compiling the structs out of both sources rather than written down:
`sizeof(PTR_EN)` **24 → 16** bytes, so `pb_count_max` — children per pointer block — goes
**170 → 255**, and a root split needs more than that many live data blocks. It was 127
before phase 125. The corpus's largest case makes **321**, so it still splits the root,
with 66 blocks of margin. Because of this the check's own deep session states the data
layer as an equality and the pointer layer as an **inequality**: at 65,149 lines — derived
from `pb_count_max` and the lines a data block holds, not typed in — both binaries make
386 data blocks, descend 2 deep and split the root once and draw the same stream, while
the output splits a pointer block **once** against the input's twice, because a wider
block splits no more often than a narrower one.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,294 | **78,977 (−317)** — the edit takes 315 and the sweep 2 |
| names | | **47 leave in the edit, 2 in the sweep, 6 arrive**, three computed sets |
| `blocknr_T` / `mf_hashitem_T` / `mf_hashtab_T` | 13 / 18 / 14 | **0 / 0 / 0** |
| `sizeof(PTR_EN)` / `pb_count_max` | 24 / 170 | **16 / 255** |
| `make editor.c` | 77,315 | **76,998**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, an equality with its reason |
| binary | 768,744 | **760,456** |
| records that moved | | **0 of 122**, with five tree counters agreeing case for case |

`nm -u` does not move, and the reason is stated rather than the number: the hash, the free
list and the block numbers were **pure computation inside the file**, reaching the outside
only through `alloc()` and `vim_free()`, which have been `host_alloc` and `host_free`
since phase 118. `tools/zerodelta.sh --phase 126` reports *exactly as declared* — screen
102/102, memline 16/16, `ref-excmds.txt` 111/111 and `ref-argv.txt` 30/30 — which is a
different comparison from the check's own `diff -r` of two recordings: one asks whether
the output matches its input, the other whether it matches whim. The synthetic input the
phase was designed against is **byte-identical** to the real q125.

## Its placement

`stage 126`, `package memline 126 127 128`, which the charter names as the end of the road —
*the text later held as a tree*. It is not `buffers`, which is phase 94 and an Ex-level
refusal to quit, and not `tidy`, which is leftovers of cuts already made.

**`need 126 swept` is required and what breaks without it is not the anchor a reader would
guess.** Measured on exactly the text phase 125's edit leaves, the edit runs to its last
act and refuses with *names this phase removes are still said: `blocknr_T` 1,
`mf_hashitem_T` 1, `mf_hashtab_T` 1* — phase 125 leaves `mf_hash_free_all` standing for the
sweep and its **forward declaration** names all three of the types this phase deletes.
**The cut applied cleanly and the partition refused, which is what a partition is for.**

**There is deliberately no `apart 125 126`, and both halves are measured rather than
argued.** Phase 125's check quotes verbatim the two lines this phase rewrites —
`pp->pb_pointer[0].pe_bnum = 1;` 1 → 0 and `if (hp-> bh_hashitem.mhi_key != 0)` 2 → 0 — so
it would stop. But with `stage 125-126` in the manifest `tools/stages.sh` answers *43 needs
swept input and does not start a stage (125-126)* and exits 1 **before any check runs**:
`need 126 swept` already forbids the only stage that could hold both, and an `apart`
nobody can measure is phase 119's rule. Both halves are written down so the next reader
knows it was checked and not assumed.
