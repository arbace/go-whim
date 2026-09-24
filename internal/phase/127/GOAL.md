# Phase 127 — de-page the leaf

`internal/phase/127/edit.go` and `internal/phase/127/check.go`, `stage 127`, `package memline`. A data
block stops being a **page of bytes** and becomes an **array of line records**. Until this
phase a leaf is a header, an index of byte offsets growing up from it and a text arena
growing down from the end of the page, with `db_free` bytes of gap where they meet: a
line's text lives inside the block, so inserting a line in the middle memmoves the arena
and rewrites every index below it, a line that grows past the gap is appended-and-deleted
into another block, and a line longer than a page makes the block two pages. After it the
leaf is

```c
struct { char_u *dl_text; colnr_T dl_len; char dl_marked; } db_line[DB_LINE_MAX];
```

and a line's text is **its own allocation**: inserting shifts records, not bytes, and
replacing stores a pointer.

**Why it is cheap now.** The arena exists for exactly one reason, to avoid a `malloc` per
line, and the charter has retired it — *A GARBAGE COLLECTOR IS ASSUMED FROM HERE ON*. This
phase spends what phase 124 bought. And the target representation is **today's dirty-line
path made permanent**, which is why the rewrite is 184 lines out and 71 in and not a
thousand: `ml_line_ptr` under `ML_LINE_DIRTY` is already a separately allocated `char_u *`
with `ml_line_len` beside it, so `ml_flush_line()`'s sixty-line *does the new text still
fit* branch — the memmove, the index fixup and the append-then-delete fallback — has
nothing left to decide and becomes one store.

## What goes, every count measured on the input and asserted as a partition

`db_free` 14 mentions, `db_txt_start` 29, `db_txt_end` 6 and `db_index` **34** all to
**zero**; the fourteen interior pointers of the shape `(char_u *)dp + start` to zero;
`DB_MARKED`'s stolen top bit, 17 expressions, to a real field; `ML_APPEND_MARK` 5; and
**both `offsetof(DATA_BL, db_index)` — by having nothing left to measure**, which is a
stronger removal than respelling them as `sizeof`, and which a prior survey verified was
byte-identical and recommended against for exactly this reason. The edit classifies every
mention of every one of them by its enclosing function and refuses on one that is not in
the struct, the enumerator or one of the eight memline functions it rewrites; the check
re-reads the input's counts **off the input** rather than trusting the numbers. **The
third memline `offsetof` stays and is named as not this phase's**: `ml_new_ptr()`'s
measures a *pointer* block, which is the branch and not the leaf.

## `DB_LINE_MAX` is a free parameter now, and it is chosen by instrument reachability

A leaf used to hold whatever fitted in a page and nothing decides it any more. **The
corpus cannot see the value at all**: 32, 64, 128 and even 1 record all 118 cases byte for
byte, so any argument from *the recording agrees* would have been vacuous. What it does
decide is how much of the tree the corpus **reaches**, measured with phase 123's markers on
q126, this phase's actual input:

| `DB_LINE_MAX` | SPLITDATA | SPLITPTR | SPLITROOT | IDXNZ | DEEP |
| --- | --- | --- | --- | --- | --- |
| 32 | 16 | 5 | 5 | 16 | 5 |
| **64** | **16** | **1** | **1** | **16** | **1** |
| 128 | 16 | 0 | 0 | 16 | 0 |
| 255 | 14 | 0 | 0 | 14 | 0 |
| q126, the input | 16 | 1 | 1 | 16 | 1 |

64 reaches exactly what the input reaches. **255 is the value that would fill the page and
it is the one that must not be chosen**: the natural-looking pick, the one that wastes
nothing, reaches no pointer-block split at all and would have blinded the instrument on
the very phase that rewrites the tree. That is phase 123's lesson applied to a parameter
instead of a corpus.

**And the margin is one case, which has narrowed under this phase.** The same table taken
on q123, where this was prototyped, read 6 / 5 / 1 / 0 in the SPLITROOT column: **128 was a
live choice then and reaches zero now.** Phase 125 took `pe_old_lnum` out of `PTR_EN` and
phase 126 took the block number and the page count, so `pb_count_max` has gone 127 → 170 →
255 while the corpus's buffer sizes have not moved. Measured directly with a counter on
`ml_new_data()`: `mem_deep_jumps` builds **321 data blocks on the input and 391 here**
against a `pb_count_max` of 255, and no other case comes near it on either side (204 / 155
/ 154 and 248 / 192 / 188). **A `PTR_EN` of 8 bytes would put `pb_count_max` at 511 and
take even 64 to zero** — at which point the corpus needs resizing or `DB_LINE_MAX` needs
lowering. The measurement stands; its margin does not, and that is the finding phase 128
turns into a compile error. The number, and that 32 reaches five, are written into the
edit, the delta and the commit, so lowering `DB_LINE_MAX` stays available if it is ever
preferred to resizing the corpus.

**This phase does not move `sizeof(PTR_EN)`**: 16 bytes either side, `struct
pointer_entry` byte-identical in and out, and the check pins `offsetof(PTR_BL,
pb_pointer)` at 1 → 1.

## The lifetime rule is pinned as a partition and not as prose

341 call sites depend on what `ml_get()` returns. It used to be a pointer **into the
page**, invalidated by any insert or delete in the same block, any flush of any line in it
and any split — the arena memmoves. It is now the record's own allocation and nothing
frees it, so **a pointer returned by `ml_get*()` is valid for the lifetime of the
process**. Stated as a partition — a record's text is written in exactly **five** places,
`ml_open` once, `ml_append_int` three times and `ml_flush_line` once, and freed in
**none** — and probed: a build that poisons the text a record stops owning moves **0 of
118**, which is the rule measured and not asserted.

**`ml_line_alloced()` is deliberately not simplified and the check enforces that.**
`del_bytes()` shortens `ml_line_len` in place under it and nothing would write that length
back, so `ML_LINE_DIRTY` must keep meaning *a replacement is pending* and not *the text is
allocated*. **It looks like an invitation and is a trap.**

## What it spends is measured, and it is the one cost no recording can see

The heaviest memline session asks the host for **201,927,792 bytes where the input asks
200,438,864**, +0.7 %, `mem_deep_jumps` either side. With nothing freed that is a
session's **traffic** and not its live data, which is why it is two hundred megabytes and
why phase 124's arena is a gigabyte. The counter is calibrated against a known answer
before it is believed — on the 233 non-memline sessions it reproduces phase 124's own
published high-water to the byte, **1,734,544** — and phase 124, rebased onto phase 123,
reached the same two numbers independently with an instrument written apart from this one.
The bound is **proven able to fail** rather than chosen: `ml_alloc_line()` over-allocating
by one page a line — the blunder the section is for — asks **304,354,000**, 1.52 times the
input, against the real output's 1.007.

## The leaf is still allocated as one memfile page, and that is deliberate scope with a measured cost

1,040 bytes of 4,096, asserted by a `static_assert` rather than left to be discovered. The
cost is reported and not hidden: the `cap` control, the capacity bound off by one, moves
**0 of 118**, because the 65th record lands in the page's spare room. Allocating a block
at its own size means giving memfile a **byte size where it has a page count**, which is
block *numbering* as well as block size — the machinery phase 126 has just rewritten — and a
phase that replaced the leaf's representation and changed how blocks are allocated in one
act would have two claims and one set of evidence. Phase 128 is that phase, and it measures
this prediction rather than repeating it. One prediction of this phase's had already come
true from the other side: `pe_page_count` and `bh_page_count` would be constant 1 after
it, and phase 126 removed both before it.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 78,977 | **78,859 (−118)** — 184 out and 71 in, and the sweep finds exactly one thing |
| `db_free` / `db_txt_start` / `db_txt_end` / `db_index` | 14 / 29 / 6 / 34 | **0 / 0 / 0 / 0** |
| interior pointers `(char_u *)dp + start` | 14 | **0** |
| `offsetof(DATA_BL, db_index)` | 2 | **0**, by having nothing left to measure |
| `make editor.c` | 76,998 | **76,880**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, a `comm` empty both ways |
| binary | 760,456 | **760,456** — the same size, different bytes, absorbed by alignment padding |
| arena high-water | 200,438,864 | **201,927,792 (+0.7 %)** |
| records that moved | | **0 of 118** |

The declared delta is **nothing at all** and it is the **weakest** kind on the list, phases
97 and 98's: the code changes, the binary moves, and the claim is that a replacement does
what the original did. There is no `cmp` to be had, so the two byte-identical recordings
are the **floor** and the **eleven controls** are the evidence. Eight must move and do —
the text not copied 2 of 118, the stored length dropped 36, a mark never set 4, the delete
shifting one record too few 6, the insert opening its gap the wrong way 8, the split
moving one record too few 4, every read taking the block's first record 52, every length
short by one 38 — and three must not, each with the reason it cannot be seen: `poison`,
`cap` and `DB_LINE_MAX = 1`. **The split control is the one that says why phase 123 was not
optional: 0 of the 102 screen cases and 4 of the 16 memline cases.** The corpus per case is
the input's own numbers — MLSPLITDATA 16, MLSPLITPTR 1, MLSPLITROOT 1, MLIDXNZ 16, MLDEEP
1 — and one marker goes down because the code is gone: phase 123's MLBIGLINE, a data block
of more than one page, fires in 3 of 16 on the input and has **no anchor in the output at
all**.

## The rebase cost exactly two anchors and both refused loudly

Which is the design working. The edit is written so that every region is located by a
function name and its own first and last line, every call whose arity changes is rewritten
by **place** and not by argument text, and every `ml_flags |=` inside a replaced region is
**carried forward as found** — and that last one paid for itself exactly as intended, phase
125 having deleted `ML_LOCKED_DIRTY` and `ML_LOCKED_POS`, so `ml_flush_line()` now carries
nothing and the edit needed no change. What did move: `ml_alloc_line`'s insertion point was
anchored on `long_to_char()`, which phase 125 deleted with block zero, and now goes
immediately above `ml_open()`, its first caller, so it depends on the function it is about;
and the probe's depth counter was declared at `ml_find_line()`'s `bnum = 1;`, which phase
126 deleted with block numbers themselves, and is now at `low = 1;` beside it — the line
that says the same thing about the **search** rather than about the representation.

## Its placement

`stage 127`, `package memline`, which phase 126 opened and whose comment says *phase 127
replaces the leaf, so the package grows*.

**`apart 126 127` is `apart 119 120` in its sharper form.** `tools/phaserun.sh 126-127` on
q125 runs both edits, one sweep and phase 126's check and stops with *the sweep: the names
that leave are ['ML_APPEND_MARK', 'ML_DEL_NOPROP', 'data_moved', 'db_free', 'db_index',
'db_txt_end', 'db_txt_start', 'e_didnt_get_block_nr_one', 'e_didnt_get_block_nr_zero',
'line_start', 'space_needed', 'text_start'] and this phase accounts for
['e_didnt_get_block_nr_one', 'e_didnt_get_block_nr_zero']*. Phase 126 states the division
between its edit and its sweep as a **partition over names**, a stage sweeps **once** at
the end, and the ten names this edit orphans land in phase 126's sweep set. **36-37 was that
lesson in a line count; this is the same lesson in a set, and a set is what a later phase
is more likely to state.** One direction only: phase 127's check was then run on exactly the
tree the shared stage produced and every part passes.

**And there is no `need 127`, measured as an equality and not as a run that did not
refuse.** The same 126-127 stage hands this edit phase 126's **unswept** output, and the file
the one sweep leaves is byte-identical to the sequential run's, 78,859 lines either way.
**The edit does not merely survive unswept text; it cannot tell the difference.**

`tools/phaserun.sh 127` exits 0 in 70 s and `make whim-tip` records q127 as
`5677d3f826f4`. The sweep finds exactly one thing in the whole phase, `ML_DEL_NOPROP`, and
the check states that division: **`ML_APPEND_MARK` is reachable code that can never be
true once the fallback goes**, so no sweep can see it and the edit takes it.
`make whim-verify` is 45 of 45.
