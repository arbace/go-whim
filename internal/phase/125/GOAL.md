# Phase 125 — the swap file's residue, and what no sweep could find

`internal/phase/125/edit.go` and `internal/phase/125/check.go`, `stage 125`, `package tidy 96 120 125`.
The filesystem went at phases 89 to 93 and the swap file's **bookkeeping** did not:
memline and memfile still kept a header block carrying the editor's version and the
buffer's name, a translation table for blocks not yet written out, a three-valued
dirtiness state, and a record of where each block's lines used to be. None of it can be
reached, none of it is read — and **not one of the four is visible to any tool in
`tools/`, because every one of them is written.**

`tools/deadfields.py` takes a field named nowhere outside its own type, and each of these
is named; run against the input it reports **0 fields**. gcc has no warning for a struct
member nothing reads, for an enumerator only ever OR-ed into a word nothing tests, or for
a file-scope object read twice and assigned nowhere;
`-Wunused-but-set-variable` does not reach a file-scope object and
`tools/deadsweep.py` does not act on it at all. So this is an **edit** and not a sweep,
and the check states the division rather than assuming it: **22 names leave in the edit
and 39 more in the sweep**, both sets named with the reason each is in the half it is in.

## The four, each proved as a partition over every mention

* **`struct block0`, the header.** **Eight** fields, not the survey's nine — phase 119
  already took `b0_pid`, so the edit reads the list **out of the struct** rather than
  from a list typed into it, which was already a phase out of date. 8 declarations, 12
  writes, **0 reads**. `ml_open()`'s thirty-two-line preamble goes with
  `set_b0_fname()`, `long_to_char()`, `ml_setflags()` and its two call sites, and the two
  surviving blocks move down by one: the pointer block is block nr 0 and the data block
  block nr 1.
* **Negative block numbers.** `mf_trans_add()` returns before doing anything unless a
  block number is negative, and the chain that could make one is **computed** in the
  edit: `mf_new()`'s callers pass `FALSE` or `ml_new_data()`'s own parameter,
  `ml_new_data()`'s pass `FALSE` or `flags & ML_APPEND_NEW`, `ML_APPEND_NEW` comes only
  from `ml_append()`'s `newfile`, and `newfile` is `FALSE` at **all eleven call sites**.
  `mf_trans_add`, `mf_trans_del`, three memfile fields, the parameter and two flags.
* **The dirtiness, write-only in all three layers.** `mf_dirty` has six writes and two
  reads and **each read is the condition of an `if` whose only statement writes the field
  again**, which the edit checks structurally; `bh_flags` is read in exactly one place
  and that read tests `BH_LOCKED`, so `BH_DIRTY` is set three times and **tested
  nowhere**; and `ML_LOCKED_DIRTY` and `ML_LOCKED_POS` are read at one place between
  them, the two arguments `mf_put()` stops taking. `mf_put()` is now `mf_put(bhdr_T *hp)`.
* **`pe_old_lnum`, 7 writes and 0 reads — and the three locals that go with it.** Taking
  the field leaves `lnum_left`, `lnum_right` and `ml_find_line()`'s `dirty` written and
  never read, which is `-Wunused-but-set-variable`, which `tools/deadsweep.py` does not
  act on, so they are the edit's for the same reason the fields are. And
  `mf_dont_release`, `static int mf_dont_release = FALSE;`, read twice and **assigned
  nowhere in the file**.

**A fifth the survey missed: `ML_LOCKED_DIRTY`'s ml-level twin.** `ML_LOCKED_DIRTY` is
set 8 times, cleared once and tested nowhere once `mf_put()` loses its state arguments —
and no warning covers a bit in a struct field. Leaving it would have created exactly the
invisible write-only state this phase exists to remove.

## And `BH_LOCKED` is not dead, which is the distinction worth keeping

It looks like `BH_DIRTY`'s twin and it **is** read, by `mf_put()`'s own
`e_block_was_not_locked` test. Measured: a binary whose `mf_put()` **sets** the bit
instead of clearing it draws all 102 screen cases identically, because the only reader is
an internal-error test that then never fires. **That is unreachable evidence, not
unreachable code**, and the phase leaves it alone and says so.

## The declared delta is nothing at all, and it is two kinds at once

The negative-block half is **phase 92's kind**, code that could not run; the block-zero
half is **phase 95's**, code that runs everywhere and the instrument cannot see. One
instrumented build of the input says both, over **252 records** — 102 screen, 16 memline,
126 from the two sweeps and eight stress sessions: the four markers on the negative-block
island fire in **0 of 252**, against a control of identical shape in `ml_new_data()`
firing in **227 of them, 2,141 times**, and the three markers on the header writes fire
in **227 / 214 / 227**, so that code runs nearly everywhere and the two full recordings
are byte-identical anyway.

**The check caught itself failing, and that is reported rather than smoothed away.** An
earlier draft computed each marker's indent from its anchor, which put two counters
**outside** the `if` they belonged in, and it refused with *`neg_new neg_find` fired in a
recorded session* — reporting the unreachable island as reachable. Every marker's
placement is now written out in full, with that measurement as the reason.

## The fanout changes, and that is what phase 123 is for

`pe_old_lnum` is a member of `PTR_EN`, so every pointer-block entry gets smaller and more
of them fit in a page: `sizeof(PTR_EN)` 32 → 24 and `pb_count_max` **127 → 170**, and the
root pointer block overflows **later**. A binary with `ml_append_int()`'s root test left
at the old block number — a real bug, the root not kept where `ml_find_line()` starts —
draws all 102 screen cases and all four of the survey's own deep cases **identically**;
what moves it is `mem_deep_jumps`, and that corpus is the only recorded thing that can.

**It also narrows what phase 123 reaches, measured 4 cases to 1** — `mem_root_split` among
them, the case named for the thing it no longer does — because phase 123 derived its
buffer sizes from `sizeof(PTR_EN)`. **A case named for the root split is a case sized for
a fanout.** The check asserts that as an *inequality* rather than a count, because this
phase can only make a pointer block hold more children. The corpus fix that would undo
the narrowing exists and cannot land; phase 123's section says why.

Section 8 of the check is the direct proof with an instrument: at sixty thousand lines
the output preserves the root once and never overflows, the control preserves it never
and reaches `e_updated_too_many_blocks`, and the two draw different screens. Four sessions
of sixty thousand lines and more then agree between the input and the output. Undo's
message carries a wall clock — four of five runs said `0 seconds ago` and one said
`1 second ago`, a difference between a binary and **itself** — so that phrase is folded to
a constant, and the check requires it present so the folding cannot hide anything.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 79,660 | **79,294 (−366)** — the edit takes 280 and the sweep 86 |
| names that leave | | **22 in the edit, 39 in the sweep**, both sets named |
| `sizeof(PTR_EN)` / `pb_count_max` | 32 / 127 | **24 / 170** |
| `make editor.c` | 77,681 | **77,315**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, an equality: a phase that deletes core code and crosses no boundary can free nothing |
| binary | 772,872 | **768,744** |
| arena high-water | 200,458,672 | **200,449,792 — lower**, every memline case allocating less |
| records that moved | | **0 of 122** |

Every one of the sixteen memline cases allocates less, smaller structs outweighing the
free-list reuse that goes; phase 124's instrument reproduces its own published figure to
the byte on q124, which is what makes the q125 number trustworthy. `tools/canon.sh` is a
no-op on the output, and `zhostonly`, `orphanopts`, `nvidxcheck` and `phasecheck` all
pass unchanged.

## Its placement

`stage 125`, `package tidy 96 120 125`, which is *leftovers of cuts already made* — phase
96's `FILE *` that had never been opened and phase 120's unions that unite nothing are the
same shape as a swap file's header in an editor that has had no swap file since phase 89.
It is not `memline`, which is phases 126 to 128 and is about **representation**.

`apart 124 125` is phase 124's, above. **There is no `need 125`** — phase 124's sweep is a
no-op, so there is no unswept text to be handed — and the 41-42 run confirms it end to
end anyway: the stage's one sweep leaves a `whim-vim.c` byte-identical to q125.

The check is proven able to fail three ways, two of them while it was being written: the
indent draft above; the output with `ml_find_line()`'s descent put back to block nr 1
refuses at the block numbers; and the edit run on its own output refuses at
`struct block0`. `make whim-verify` is 43 of 43.
