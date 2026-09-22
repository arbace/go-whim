# Phase 123 — the instrument could not see the text layer

`phase/123/make.sh` — one file, like phases 83, 84, 86 and 116 — `stage 123`, `package harness
3 33 40`. It changes no source at all: q123's `whim-vim.c` is q122's byte for byte and its
boundary digest is its input's, `68e450fd6912` either side. What it adds is the **sixth
part of a recording**, `tools/zmemline.py`, and it is here for the reason phase 86 and
phase 116 were: **a harness that cannot see a phase must be fixed before the phase, never
after.** Phases 124 to 128 rewrite the memline, and until this phase ran nothing in the
pipeline could have told a working one from a broken one.

**One thing about every number from here on.** Between phase 122 and this phase the
`keymodel=startsel` repair landed in slim Phase 1, three lines in `set_init_1()`, and all
three products were re-passed from it — so every Part II boundary gained three lines and
`q122` is **79,592** where the phase 122 section above says 79,589. Every figure in this
section and the five below it is post-repair, and every figure above it is pre-repair;
the difference is those three lines and nothing else. `CLAUDE.md` records what the repair
was and what it falsified.

## The blindness is measured and not suspected

A `whim-vim` with one line deleted from `ml_find_line()`'s `ML_DELETE` arm —
`pp->pb_pointer[idx].pe_line_count--;`, the statement that keeps a pointer entry's idea
of how many lines hang under it — records **all 102 screen cases byte for byte**. That
binary was built and both corpora run on it before this phase was written: 0 of 102
differ. **Forty phases had been verified by an instrument that could not see a corrupted
text layer at all.**

The reason, confirmed with an instrumented q122 rather than reasoned: every one of the
102 cases allocates **exactly one data block**. The root pointer block holds one entry
for the whole session, so `idx` is 0 every time, `ml_find_line()` never chooses among
entries, and `pe_line_count` is never the number that decides anything. A 4,096-byte
page holds 78 lines of the width these cases type and `pb_count_max` was 127, so a root
cannot split before **9,984 lines** — and the largest case in the 102 is nowhere near
it. All seven of the phase's markers fire in **0 of 102**.

## The corpus is sixteen cases, and its depth is measured rather than intended

`tools/zmemline.py` is sixteen cases building buffers of **200 to 25,000 lines**, and
they are built **in the editor**: there is no file argument (phase 88), no `:edit` (phase
91) and no `:read` (phase 90), so a case types one line under `'paste'` and replays a
three-key macro with a count — five keystrokes and about two seconds for 25,000 lines.
Every line begins with its own number, so a screen drawn with `'number'` shows the
tree's answer beside the question; each case churns the **middle** of the buffer and
then reads the whole of it back with a substitute count, and jumps to named lines and to
both ends.

**The block arithmetic is derived and not written down**, by compiling the struct
definitions out of the source the phase was handed — page 4,096, data header 24, index
entry 4, `PTR_EN` 32, so a 47-byte line packs 78 to a block and `pb_count_max` is 127 —
and the corpus's own sizes are then required to clear those thresholds. **A corpus that
means to reach a root split and does not is exactly the defect this phase exists to
end**, so the depth is a measurement:

| marker | 16 memline cases | 102 screen cases |
| --- | --- | --- |
| a data block splits | **16** | 0 |
| a pointer entry chosen at `idx > 0` | **16** | 0 |
| a data block of more than one page | **3** | 0 |
| a pointer block full | **4** | 0 |
| **the root splits** | **4** | 0 |
| a non-root pointer block splits | **4** | 0 |
| `ml_find_line()` descends through a second pointer block | **4** | 0 |

## Five controls, and the one that moves nothing is what makes the others mean something

Four are corruptions and each moves **0 of the 102**, which is the premise restated as a
measurement. Deleting `pe_line_count--` from the descent moves **7 of 16**; deleting
`ml_lineadd()`'s **deferred** adjustment — the other place a pointer entry's count is
maintained, and a different shape of mistake — moves **16 of 16**; widening
`ml_find_line()`'s `ML_FIND` stack by one line moves **4 of 16**, and they are exactly
the four the probe measured descending past one pointer block, which the check states as
a **rule** and not as a list of names.

The fifth quarters `pb_count_max` and moves **0 of 16** while the probe shows the root
split going from 4 cases to 13. That is not a failure: **the corpus records behaviour
and not tree shape**, and the control proves it is not a no-op by the probe rather than
by assertion. It is the same finding phase 127 meets again from the other side when it
has to choose `DB_LINE_MAX`, and phase 128 turns into a `static_assert`.

**A discarded control is reported rather than dropped.** Corrupting the root entry at the
split site moves nothing, because the next iteration overwrites it — and a control the
code repairs is not a control.

## It found a real defect, and it is not the one the scrub was written for

One record's stream digest moved in **1 of 48** whole recordings with every screen
identical. Phase 115's section above has the corrected account: the cause is not the
timestamp's text, which `tools/zrec.py` already rewrites padded, but **arithmetic on its
length**. An undo in a buffer this size reports its age and the editor then positions the
cursor to clear the rest of the line, so `0 seconds ago` emits `\033[24;40H\033[K` and
`1 second ago` emits `\033[24;39H\033[K`. A column derived from a scrubbed string's width
leaks the thing the scrub exists to hide, and it failed phase 99, whose binary is
byte-identical either side — which is the only reason it was catchable.

So **a memline record carries `stream N redraws` and no digest**, N being the count of
`\x1b[?25h`. What replaces the digest as evidence is the fifth control above's sibling:
both clocks the core can read replaced by counters that run away from the wall move **0
of 16 memline records against 9 of the 102 screen cases**. That is stronger than a
digest, because it says the record does not depend on the clock **at all** rather than
that two runs of it happened to agree. `tools/zcases.py` still digests the raw stream,
and closing that is expensive rather than difficult — see *What is still open* below.

## Four phases pinned the size of a recording, and had to stop

A new **part** of a recording is a new file whatever shape it takes, so `zpty.py`'s
precedent — one record however many scenarios it holds — could not be followed. Measured:
phase 92 stops with *a recording is 122 files, not the 106 this phase counted*. Phases 92,
96, 113 and 117 each tested the count as an **equality**; phases 108, 118, 119 and 120 tested
it as a floor of 100 and were unaffected. The four equalities are now **computed** —
phase 92's control must mark *total − 2*, quiet only in `ref-pty.txt` and `ref-term.txt`,
and the other three take the floor of 100 their siblings already use — which is
`CLAUDE.md`'s own rule that **a number a phase cannot move is reported and not pinned**.

## Measured

| | before | after |
| --- | --- | --- |
| `whim-vim.c` | 79,592 | **79,592**, byte for byte |
| the boundary digest | `68e450fd6912` | **`68e450fd6912`** |
| a recording | 106 records, five parts | **122 records, six parts** |
| memline cases | — | **16**, 200 to 25,000 lines |
| the seven markers, memline | — | 16 / 16 / 3 / 4 / **4** / 4 / 4 |
| the seven markers, screen | 0 of 102 | **0 of 102** |
| `make whim-verify` | | **41 of 41**, 2,771 s of phases in 129 s |
| keys moved | | **58 zero**, and not one whim or slim key |

A cold `make whim-repass` from an empty cache reproduces all 40 recorded boundaries and
adds q123. The 58 are all 40 Part II units and 18 Part II edits; `tools/zmemline.py`,
`tools/zrecord.sh` and `tools/zcompare.py` are named by no whim or slim phase, and that
is the measurement rather than the claim, so core rule 9's gate does not apply.

## Its placement

`stage 123`, `package harness 86 116 123`, which is *the phase changed no source and moved
the instrument instead*. **There is no `apart 122 123` and no `need 123`**, and both are
refusals rather than omissions: `phase/123/make.sh` is a whole-phase program, so
`stage 122-123` is answered by `tools/stages.sh` with *phase 123 is in stage 122-123 but is
not an edit and a check* and by `tools/phaserun.sh` with *phase 123 has no edit and
check to run in stage 122-123*, both measured — and `need` is a statement about an edit
part, which this phase has none of. That is `apart 116 117`'s shape exactly.

The declared delta is **nothing at all**, and it is phase 86's and phase 116's kind: the
phase changes no source, so nothing about the editor's behaviour *can* have moved, and
what it has to argue is that the **comparison** moved safely. It does that by requiring
every recorded boundary back.

## What is still open, stated as two things and not one

**The corpus can be made to reach further and the fix cannot land yet.** Branch
`zmemline-fix`, commit `4e9fb9c`: `tools/zmemline.py` derives its case sizes from the
block arithmetic instead of carrying them as constants, and chunks the buffer build.
Measured, it reaches a **root split in 4 of 16 cases** both at the real fanout and at a
forced `PB_COUNT_MAX = 511` — the value an 8-byte `PTR_EN` would give — where the corpus
as committed reaches **1 and 0**. It is blocked because **two merged checks assert the
corpus's insufficiency as a requirement**: `phase/125/check.sh:686` requires
root-split coverage to *decrease* under phase 125's wider pointer block, and
`phase/126/check.sh:557` requires identical tree-event tuples across a fanout change.
Both pass today **only because the instrument is too small to see otherwise**, so
landing the better corpus means rewriting two checks that were correct when they were
written. That is a phase's worth of work and is recorded here rather than done quietly.

**And the `ago` leak is still open in `tools/zcases.py`.** The fix that works is this
phase's — count redraws, and let a clock control carry the evidence — and applying it to
the other 106 records is expensive rather than hard: the `--- stream` line is named in
**eighty files**, forty-seven times in phase 95's check alone. It deserves a pass of
its own. Phase 115's section states the hazard and phase 99 is where it struck.
