# Phase 128 — fold the node types

`phase/128/edit.go` and `phase/128/check.go`, `stage 128`, `package memline`. The
memfile goes, and with it the last thing between the tree and its nodes. Until this phase
a memline node is **two** allocations: a `bhdr_T` of four members — two used-list
pointers, a `char_u *bh_data` and a lock flag — and, hanging off it, a 4,096-byte page cast
to `PTR_BL *` or `DATA_BL *` by the two-byte id at its front, with a `memfile_T` of two
members owning the list head and the page size. After it

```c
struct block_hdr     { short_u bh_id; };
struct pointer_block { bhdr_T pb_hdr; short_u pb_count; PTR_EN pb_pointer[PB_COUNT_MAX]; };
struct data_block    { bhdr_T db_hdr; linenr_T db_line_count; DATA_LN db_line[DB_LINE_MAX]; };
```

**There are no pages, no blocks and no memfile left — just a counted tree of nodes holding
line records.** A node is **one allocation at its own size**, 1,040 bytes for a leaf and
4,088 for a branch against 4,128 for either of them before. `bhdr_T` is the node's **tag**
and the first member of both, so `(PTR_BL *)hp` and `(bhdr_T *)pp` are the same address
and the file needs no union; `memfile_T` has nothing left to hold and is gone; and
`ml_root` answers *does this buffer have a memline* where `ml_mfp` did, taking `memline_T`
from 104 bytes to 96. `mf_open/close/new/get/put/free/ins_used/rem_used/alloc_bhdr/
free_bhdr` all go, and `mf_close()`'s teardown becomes `ml_free_tree()` walking the tree,
which is the same set of nodes.

`bhdr_T` was **not** the wrapper around one pointer the brief hedged for, and the phase
says so: it was four members and 32 bytes, a doubly-linked used list, a `char_u *bh_data`
pointing at a separate page, and a `BH_LOCKED` flag. It is now two bytes.

## This is phase 127's own named next step, and both halves are measured rather than repeated

Quoted in that phase's program: *"Allocating a block at its own size means giving memfile a
byte size where it has a page count ... it would take the leaf from 112 bytes a line to 64,
and it would MAKE AN OFF-BY-ONE IN THE CAPACITY BOUND VISIBLE, which today it is not."*

Phase 127's own `cap` control — the leaf capacity test widened by one — is built from **this
phase's input and from its output in the same run**: **0 of 118 records on the input**,
which is the 0 of 118 phase 127 published, and **4 of 118 here**. The 65th record used to
land in the page's spare room and now lands past the end of a 1,040-byte allocation. Phase
127's other half does **not** reproduce, and the phase reports what it measured rather than
what was predicted: the leaf's node cost falls from **64.5 bytes a line to 16.25**, not
"112 to 64".

## What the phase could have destroyed, and the measurement that says it did not

`pb_count_max` was computed per block as `(4096 - 8) / sizeof(PTR_EN)` = **255**, and it is
the tree's **fanout**. Phase 123's corpus reaches a root split in exactly one of its sixteen
cases, `mem_deep_jumps`, which builds **391** data blocks; the other fifteen and all 102
screen cases reach none of it. A phase that took `sizeof(PTR_EN)` to 8 would put the fanout
at **511**, and 391 < 511 would take root-split coverage to **zero — silently**, because
the instrument would still run and still pass, which is the defect phase 123 exists to have
ended.

So `PTR_EN` is not touched, and the new struct has the same offset **by construction**: a
two-byte tag and a two-byte count where three shorts were, so `pb_pointer` starts at 8 and
`PB_COUNT_MAX = 255` is the number the input computes rather than a number chosen.
`static_assert(sizeof(PTR_EN) == 16, ...)` is appended to the `make editor.c` cut of both
sides and both compile, with `== 8` required to **fail** against both so the assertion is
an assertion. The five markers are then measured case by case on both binaries and are
identical: MLSPLITDATA 16, MLSPLITPTR 1, MLSPLITROOT 1, MLIDXNZ 16, MLDEEP 1, and 0 of 102
screen cases.

**And the file now carries**

```c
static_assert(PB_COUNT_MAX == (4096 - 8) / sizeof(PTR_EN), ...)
```

**which fails to compile if a later phase narrows the entry.** The whole memline arc has
been shadowed by the risk that shrinking `PTR_EN` to 8 would take `pb_count_max` to 511 and
root-split coverage to zero without anything noticing. **It is now a build error rather
than a thing to remember**, which is the arc's standing hazard ended in the only way that
survives a reader who has not read this document.

## The hazard is demonstrated and not argued

The `fanout` control sets `PB_COUNT_MAX = 511`, what an 8-byte entry would give. It moves
**0 of 118 records** — and that **is** the point: the same binary takes MLSPLITPTR,
MLSPLITROOT and MLDEEP **from 1 to 0**, so narrowing the entry would take the root split
out of the corpus **without moving one record**. It is run under phase 123's instrument as
well as under the recording, **because the recording is exactly what cannot see it**.

Two other controls move nothing and are reported with their reasons. `noclear` —
`alloc()` for `alloc_clear()` — moves nothing because the host's arena is a bump pointer
over fresh pages, so the memory is already zero: **a fact about this host and not a promise
the core may rest on**, though `ml_open()`'s error path does rest on it, and that zeroing
is kept and is load-bearing exactly once, the error path walking a root whose single entry
has not been filled in. `nofree` — `ml_free_tree()` freeing nothing — moves nothing because
`host_free()` has returned without doing anything since phase 124, so **what a core gives
back is unobservable by construction**.

## The partition is over the file's whole vocabulary

And not over a list of names the edit happens to know. String literals excluded —
`whim-vim.c` has no comments and no preprocessor, so the scan is exact — **exactly 30
identifiers leave and exactly 5 arrive**: 26 the edit takes (236 mentions of `bh_next`,
`bh_prev`, `bh_data`, `bh_flags`, `mf_used_first`, `mf_page_size`, `memfile`, `memfile_T`,
`ml_mfp`, `pb_id`, `db_id`, `pb_count_max`, `MEMFILE_PAGE_SIZE`, the ten `mf_*` functions,
`mfp`, `page_count` and `page_size`), 2 the sweep takes (`BH_LOCKED`, which
`deadenums.py` finds, and `e_block_was_not_locked`, which `deadsweep.py` does — the whole
of what the sweep finds in this phase), 2 that go with a function the edit deletes
(`mf_close`'s `nextp` and `ml_find_line`'s `error_noblock` label), and 5 written
(`PB_COUNT_MAX`, `bh_id`, `pb_hdr`, `db_hdr`, `ml_free_tree`).

The open-buffer predicate is stated the same way: `ml_root` is compared with `nullptr` in
**no** function in the input and `ml_mfp` in **fifteen**, and in the output `ml_root` is
compared in those fifteen **plus `ml_delete_int`**, which asked the same question through
a local copy.

**Nothing is freed that was not freed before.** `mf_close()` walked the used list at
`ml_close()` and the used list was exactly the set of live nodes, so `ml_free_tree()` walks
the **tree** and frees the same set; `mf_free()`'s two call sites become `vim_free(hp)`,
one allocation where there were two.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 78,859 | **78,666 (−193)** |
| a node | 2 allocations, 4,128 bytes either kind | **1 allocation**, 1,040 leaf / 4,088 branch |
| `bhdr_T` / `memline_T` | 32 / 104 bytes | **2 / 96 bytes**; `memfile_T` gone |
| `sizeof(PTR_EN)` / `PB_COUNT_MAX` | 16 / 255 | **16 / 255**, by construction and asserted from the cut |
| identifiers | | **30 leave, 5 arrive**, a partition over the whole vocabulary |
| `make editor.c` | 76,880 | **76,687**, 0 directives, the same 18 interface names |
| `nm -u` | 14 | **14**, a `comm` empty both ways |
| binary | 760,456 | **760,424** |
| arena, heaviest case | 201,927,792 | **200,720,256 (−0.6 %)**, the biggest fall `mem_join_split` at −9.2 % |
| records that moved | | **0 of 118** |

**Every one of the sixteen memline sessions asks the host for less, and the difference is
arithmetic**: 391 data blocks times (4,128 − 1,040) is 1,207,408 of the 1,207,536 bytes
that go.

The declared delta is **nothing at all**, phases 97, 98 and 127's weakest kind — the code
changes, the binary moves, and the claim is that a replacement does what the thing it
replaces did. Two full recordings are byte-identical to the input's across all 118 cases
and four tables, so the recordings are the floor and not the evidence. **Twelve controls
carry the phase**, nine of which must move a recording and do: the leaf tagged wrong 118 of
118, the branch tagged wrong 118, the leaf test inverted 118, the root split forgetting its
count 1, the root split copying no entries 1, the branch capacity bound off by one 1, a
branch allocated at a leaf's size 6, a leaf allocated at half its size 16, the leaf
capacity bound off by one 4. Three must not, and each is named above with its reason.

## Its placement

`stage 128`, `package memline 126 127 128`. **The 128-on-127 dependency is the strongest in the
arc and is deliberately not a `uses` line**, both phases being in one package, so it is
written into the package comment instead; eight `uses` lines record the cross-package ones.
`tools/stages.sh zero --check` and `tools/packages.sh zero --check` both pass.

**`apart 127 128` is phase 127's own scope statement read from the other end, and it needed no
reasoning.** That phase wrote that `offsetof(PTR_BL, pb_pointer)` *"measures a POINTER
block, which is still a page of entries and is not the leaf ... De-paging the branch is a
phase of its own"*, and its check says it as a count of 1. Measured by running
`phase/127/check.go` on the q128 tree with phase 127's own state directory: *`ml_new_ptr`'s
offsetof moved, and a POINTER block is still a page and is not this phase's*. One direction
only, because there is nothing to observe in the other — the stage cannot run at all.

**`need 128 swept` is the first in this pipeline that is about blank lines**, and it is
`CLAUDE.md`'s *a pass that touches those needs a count of them as its own check* meeting a
shared sweep. The edit deletes whole functions and single statements out of the middle of
others, so it asserts that it leaves **no run of two blank lines anywhere** — which is a
statement about this edit only if the text it was handed had none. It had one: phase 127's
edit leaves a run of two at line 33,815 of its own unswept output, which `canon.py` removes
in phase 127's sweep. So the edit asks the **input** first, and `tools/phaserun.sh zero
44-45` on q126 stops with *the input already has a run of two blank lines, so this edit
cannot say it left none: it needs swept text*. **The order of those two tests is the whole
of it** — asked the other way round the refusal would have blamed this phase for the
previous one's residue.

`make whim-tip` records q128 = `698924a46bfa`, and `make whim-verify` reproduces it from q127
in a scratch root of its own. Two things not this phase's, both stated: q122 failed that
verify run on `ref-pty.txt` under a 64-way load and passes alone, which is the pty
flakiness `CLAUDE.md` already records; and **`del_file` is still an unread parameter of
`ml_close()`**, as it was of `mf_close()` before it — removing it reaches into
`'cpoptions'` through `CPO_PRESERVE`, which is not this phase's.

## What whim-vim is after phase 128

```
whim-vim.c        78,681 lines          from whim-vim.c's 86,583  (-7,902, 9.1%)
                  76,716 above the boundary, 1,965 below it
functions         1,737
type definitions  881
DWARF enumerators 1,168
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    109 that are not a t_ capability, 95 distinct globals
                        (orphanopts floor 80; 15 of margin)
built-in terminals 2 of whim's 10: xterm-256color and debug
#include          11, at line 76,718, and NOT ONE DIRECTIVE above them
core -> host      18 names: vim_snprintf, host_exit, host_message, host_time,
                  host_alloc, host_free, host_write, host_raise, ten musl_*
libc prototypes   0 -- the core names no libc function at all
libc symbols      14 with the core's flags, 15 as tools/symbols.sh counts
the memline       a tree of nodes, one allocation each: a leaf is 1,040 bytes
                  holding 64 line records, a branch 4,088 holding 255 children,
                  and a line's text is its own allocation nothing frees
binary            760,424 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    term-moved at 38 and four command lines at 39; 123 to 128 declare
                  nothing at all -- with 2 stderr-moved and the records of 87 to 94
                  before them
make editor.c     76,716 lines: 0 directives, 0 errors, 18 warnings, all of them
                  `used but never defined` and all of them the interface
```

**The `options[] rows` figure is stated here as the count that can be reproduced**: rows of
`options[]` whose name is not a `t_` terminal capability, 109, measured on this file. The
blocks above this one carry **107**, which no phase between phase 103 and here removed a row
to justify; the number that the floor actually reads, and that has tracked every removal
exactly, is the 95 distinct globals — `whim-vim.c`'s 116 and 102, less phase 95's six rows
and phase 103's `'termresize'`.

**Phases 123 to 128 are one arc and the documents carry it as one.** The instrument had to
exist before the work was checkable, which is 40; the bump allocator made per-line
allocation free, which is 41; 42 cleared the swap file's bookkeeping out of the way; 43
turned a block number into a reference; 44 let the leaf stop being a byte arena; and 45
ends it by folding the node types and turning the arc's standing hazard — that shrinking
`PTR_EN` would silently take the root split out of the corpus — into a `static_assert` that
fails to compile. Every one of 42 to 45 rests on a
measurement the corpus the pipeline had at phase 122 could not have taken — the fanout
narrowing, the tree events agreeing case for case, `DB_LINE_MAX`'s reachability table and
the `fanout` control — which is the whole argument for doing 40 first.

**The six phases declare nothing between them, and they are four different kinds.** 123 is
phase 86's and 116's — no source changed at all, so what has to be argued is that the
*comparison* moved safely. 124 is the sixth — the code runs and the instrument sees it do
the same thing — with a `cmp` of the **core** underneath it that no earlier phase could
offer. 125 is phase 92's and phase 95's **at once**, one instrumented build carrying both
halves. 126 is the sixth again and the strongest instance of it this pipeline has, because
every keystroke reaches its text through the function it rewrites. And 127 and 128 are the
**weakest** kind, phases 97 and 98's: the code changes, the binary moves, and eleven and
twelve controls carry each of them because nothing else can.
