# Phase 116 — the terminal table is asked with `+set term=`, not `$TERM`

`phase/116/check.go`, one whole program, `stage 116`, `package harness`. The second phase
in the pipeline that changes **no source at all** — phase 86 is the other — and it is
there for the same reason: the pipeline was about to measure itself with a question
that could not see the answer.

**The nineteen rows of `.reference/core-baselines/ref-term.txt` were content-free, and
had been since phase 83.** Every one of them read

```
TERM='vt100'              -> term=xterm-256color t_Co=256
```

because phase 19 removed the `getenv("TERM")` from `termcapinit()` — *the terminal
is what the build says* — and left a compiled `"xterm-256color"` in its place. Nineteen
ways of recording that the environment does nothing.

**The measurement is the reason the phase exists rather than an argument for it.** A
prototype that DELETED eight of the ten built-in terminal names and three of the nine
capability tables — 118 lines of terminal description — passed `tools/zcompare.py`
against the real baselines **declaring nothing at all**. The only thing that moved in a
five-part recording was two lines of stderr, which `2 stderr-moved` already absorbs. A
phase may declare nothing only when the instrument could have seen it; here it could
not.

`tools/ztermcheck.py` now asks `+set term={name}` on the command line, which reaches
`did_set_term()` rather than `termcapinit()`'s compiled default, and records the `E5NN`
beside the answer where one is given. **Ten of the nineteen names resolve to
themselves** — `term=screen t_Co=8`, `term=debug t_Co=` — and **nine are refused**,
recorded as `E522 term=xterm-256color t_Co=256`: the error *and* the terminal the editor
stayed on, which is what makes a refusal distinguishable from the old vacuous row. It
goes straight to `+set term=` and never to `-T`, measured: a `-T` harness run against a
binary with no `-T` records nineteen `(none)` rows, and `+{command}` is `GOALS.md`
II.5 decision 8, the one facility promised to survive every phase.

Two things the tool had to get right, both measured. The error line **echoes the
assignment** — `E522: Not found in termcap: term=vt320` — so a naive `find('term=')`
reports the *requested* name as the result; any line carrying an `E<digits>:` has the
code taken off it and is then skipped. And the row label is `:set term=` and not
`TERM=`, or the record would say `TERM='vt320'` about something that is not the
environment at all, so `termcheck.one` is overridden as well as `termcheck.ask`.
`tools/termcheck.py` itself is untouched: it was named by `tools/whimdelta.sh` and
`tools/verify.sh` and its bytes were in every whim stage's key (core rule 9).

## The re-record is the delicate part, and it is not `CLAUDE.md`'s mistake

`CLAUDE.md`'s rule is *never regenerate it from the current binary, which would make the
comparison self-fulfilling*, and the mistake it names is a pipeline re-recording from
its **own output**. `phase/083/check.go` does the opposite and enforces it: the baselines
come from `whim-vim.c`, the pipeline's immutable input, built with **whim's** compile
line, recorded three times and required identical. Nothing the core produces is on the
recording side. The incantation is

```sh
rm -rf .reference/core-baselines .cache/r0 && make whim-phase-83
```

and **both paths are needed**: measured, with only `.cache/r0` removed `phase/083/check.go`
refuses — *"baselines DIFFER from the recorded `.reference/core-baselines` … a harness
changed, or the frozen `whim-vim.c` did. Name which before removing it"* — and exits 1
naming `ref-term.txt`. It is right to refuse. `whim.mk`'s `whim-baselines-check` said
only `rm -rf .cache/r0`, which is correct for the MISSING case and wrong for the
changed-harness case a reader will actually hit, so its message now names both paths and
says why; `whim.mk` is in no implementation digest.

## What makes the re-record safe is measured and not cited

The baseline and every phase's recording move **together**, and `phase/116/check.go`
measures that: `./whim-vim` extracted from every recorded boundary tar — all 33 of them —
plus `whim-vim.c` built with whim's own line records the same table, **one digest across
every one of them** under the new question, exactly as the old question gave one digest
across every one of them. So `term-moved` stays undeclared at every phase before this
one and after it, and `tools/zcompare.py` agrees: the declared delta at every boundary is
the cumulative list through phase 94 and nothing new, checked at 89, 98, 105 and 115 by hand
as well as at all 34 by `make`. **That check is also what covers a stale tier-3 replay**:
nineteen other Part II units keep their keys and would replay with a `ref-term.txt` recorded
under the old question, so section 4 re-derives, for every recorded boundary binary, the
thing such a replay would carry over.

**Section 4 now refuses when the tars are absent, where it used to say so and pass.**
Nothing produces `.build` any more — `.gitignore` states that — so this arm's evidence
rests on the set kept from the pass that recorded it, and a checkout without that set
was getting `no .build here` followed by a pass. That is a check that cannot fail, which
by this repository's rule is not evidence, so all three arms return instead: no
`.build/q82.tar`, a q82 that will not build, and a `.build` holding no boundary binary
from q83 on. Re-measured today rather than reasoned about: **30** tars from q83 on and
`q82.tar` present, where the paragraph above was written against 33. The count is not
asserted — the arm compares whatever boundaries are there and refuses only on none — but
the shrinkage is recorded here because nothing can rebuild what goes missing.

## The instrument is proven able to fail, and the one it replaces proven not to be

With **one** row deleted from `builtin_terminals[]` — the last named row, chosen by the
program and not written into it — the new table moves exactly **one** of its nineteen
rows, `term=debug t_Co=` → `E522 term=xterm-256color t_Co=256`, and the question this
replaces, asked of the same two binaries, moves **0 of 19**: its nineteen rows carry one
distinct answer between them. That pair is the whole phase in one measurement.

**Nothing the check asserts is a number that was observed.** The table has as many rows
as `tools/termcheck.py` has names; which of them resolve is read out of
`builtin_terminals[]` in the source the phase was handed; and what a refused name leaves
the terminal as is measured from the binary, by asking it with no `+set term=` at all,
rather than written down as `xterm-256color`. So the rules stay true of the phase that
deletes eight of those names. The undefined symbol count is **reported and not pinned**
for the same reason: a number this phase cannot move is not a check, it is a thing to go
stale.

## One second change to the tool, and it is not cosmetic

Every session made a scratch directory in `/tmp` and left it there. Measured while this
phase was being written: **182,319** of them were lying about — 100,280 `termcheck-*` and
82,039 `ztermcheck-*` — and an ext4 directory that full answers `mkdir` with `ENOSPC` on
a disk with 70 GB free. That failed phase 92, a phase with nothing to do with
terminals, in the middle of a run of this one. `ztermcheck.py` now removes its own
directory; `termcheck.py` is whim's and slim's and is left alone.

## Measured

| | input | after |
| --- | --- | --- |
| `whim-vim.c` | 79,776 lines | **79,776, byte for byte** — `cmp`-identical |
| the boundary digest | `d2a14122ccf7` | **`d2a14122ccf7`**, its input's |
| `make editor.c` | 77,889 | **77,889**, 11 `#include`s with none above them |
| binary | 782,760 | **782,760**, `EXEC`, no `INTERP`, no dynamic section, no relocation |
| `nm -u` | 17 | **17**, reported and not pinned |
| the nineteen rows | one answer between them | **ten resolve to themselves, nine are refused with `E522`** |
| a deleted `builtin_terminals[]` row | moves **0 of 19** | moves **1 of 19** |
| records that moved | | **0 of 106** — there is no source to move them |

## Its placement

`stage 116`, `package harness 86 116` — the package that changes no source at all. Two
`uses`: `seed:83`, because the nineteen rows it re-records are phase 83's and phase 83
**refuses** to overwrite a set that differs; and `streams:88`, because the question is
`+set term={name}` on a command line with **no file on it**, which phase 88 made an
unknown option — and which is also what left the old question asking `$TERM` with an
empty buffer.

**Phase 116 can share a stage with nothing and needs no `apart` to say so**, exactly as
phase 86 does not. A stage of more than one phase is made of **split** programs and
`phase/116/check.go` is one file, so the schedule is refused before any check runs:
measured, `stage 116-117` gives `phase 116 is in stage 116-117 but is not an edit and a check`
from `tools/stages.sh`, and `tools/phaserun.sh` refuses the same unit with `phase 116
has no edit and check to run`. Nor is there a `need`: a whole-phase program is handed the
previous boundary's tree and has no edit part for a sweep to precede.

**Key movement, measured over all 170 implementation keys of the three pipelines** — 12
slim phases, 13 whim stages, 82 whim edits, 33 Part II units, 30 Part II edits — in a scratch
copy of the tools and the phase programs, one change at a time: editing `tools/ztermcheck.py` moves
**16, every one of them Part II's** (units 83, 86, 88, 92, 96, 104, 108, 109, 110, 111, 112, 113, 114,
115 and edits 88 and 108), and **adding the phase moves 0 of 170** and adds one unit — which
is what the phase list living in `phase/STAGES.md` rather than in
`tools/pipeline.sh` buys.
