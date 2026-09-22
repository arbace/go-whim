# Phase 121 — the eight terminal names go, leaving two

`phase/121/edit.sh` and `phase/121/check.sh`, `stage 121`, `package terminal`.
`builtin_terminals[]` is the whole of what the core knows how to draw on: a name and a
capability table, ten times. **An embeddable core has no business carrying ten terminal
descriptions** — the host decides what it is attached to — so this phase keeps **two**:
`xterm-256color`, which is already the name `termcapinit()` substitutes when it is given
none, and `debug`, which draws its capabilities as text and is the only one readable
without a terminal at all. Which rows go is **the table minus the two**, computed from the
source; the two are the only names the phase writes down.

## This is the first Part II phase to declare `term-moved`

The token has been in the delta grammar since phase 83 and no phase had used it, because
the terminal table was nineteen ways of recording that the environment decided nothing —
until phase 116 changed the question to `+set term={name}`. And **phase 116 exists
because a prototype of this cut passed `tools/zcompare.py` declaring nothing at all**. So
the declaration is a real delta and not a harness artefact, which is the distinction rule
2 and `CLAUDE.md`'s *A phase can break a harness rather than change behaviour* exist to
keep apart, and this phase is on the other side of it from phase 88's.

`phase/121/delta` states which **eight** of the nineteen rows move and to what, because
the token itself is whole-file: `xterm`, `screen`, `screen-256color`, `tmux`,
`tmux-256color`, `vt100`, `ansi` and `dumb`, each from `term=<itself>` to `E522
term=xterm-256color t_Co=256` — the answer the eight names that never had a row already
gave. The other eleven are byte-identical: `xterm-256color` and `debug` still resolve, the
eight unknown names were refusals already, and `''` is still `E529`, which is `'term'`
refusing an empty string before any table is consulted.

## Three things the cut drags with it, and none of them is in the row list

**Three capability tables lose their last row** — `builtin_ansi`, `builtin_vt100`,
`builtin_dumb` — and the sweep takes them, 103 lines. That set is **computed**, *a
`builtin_*` table whose only mention left is its own definition*, and the count is taken
on text whose string literals are blanked: `builtin_xterm` is also a string literal inside
`vim_is_xterm()`, and a count that read that as a reference would report a live table
dead.

**`find_builtin_term()`'s xterm-family special case can never fire again.** It is
`strcmp(name, "xterm") == 0 && vim_is_xterm(term)` — a test on the **row's** name — so once
no row carries that name its first conjunct is false for every row. gcc has no warning for
a condition false at run time and nothing in `tools/sweep.sh` reads one, so it goes in the
**edit**. It is dead twice over, measured: the output with the clause restored records byte
for byte what the output records, and the same marker inside it, written through
`host_message()`, is in **102 of 102** screen records on the input and **0** here. And the
finding a reader would not predict is that **every startup of the input resolved through
that clause** — the compiled default is `xterm-256color`, the `xterm` row sorts before it,
and `vim_is_xterm()` says yes — so the surviving row was never reached until now, and it
gives the same table.

**And `set_termname()`'s no-screen fallback named one of the eight.** An unknown name is
refused at run time (E522, the terminal left alone) but *before there is a screen* — the
`-T {term}` path — it is replaced by a name written in the source, and that name was
`"xterm"`. Left alone the cut would have left it dangling, and that was measured rather
than reasoned: with the repair left out, `-T xterm` and `-T no-such-term-9x` print `E437:
Terminal capability "cm" required` and draw **2,045 bytes where the baseline draws
2,117** — an editor with no cursor motion. The phase retargets the fallback onto
`termcapinit()`'s compiled default, computed from that function and required to be a
surviving row, and rewrites the message in the same step, because **nothing in the build
checks that a message tells the truth**. With the repair those two rows are the baseline's
again and `term-moved` is the whole difference; the check builds the repair-left-out form
as a control and requires it to move exactly those two records and no others.

## The edit is a partition and not a count, and it has to be literal-aware

A terminal name here is a string literal, and the same words appear as identifiers
(`builtin_xterm`), as prefixes (`musl_strncasecmp(name, "xterm", 5)`) and inside other
literals (`"screen.xterm"`). Every literal whose **content equals** a removed name must
fall in one of four classes — its row, the family clause, the fallback, or a counted
prefix test in `vim_is_xterm()` — and a leftover refuses. Measured on the input: 11
literals, 8 + 1 + 1 + 1. The prefix class is the only one kept, and keeping it is honest
because each must be an argument of a comparison with **its length written out**, so a
name compared in full could not hide there. All the text edits are applied in one pass
over the original offsets.

## The instrument is shown able to fail in both directions

On this phase's own output, with both rows derived from the tables and neither written
here: deleting the last row the output names moves exactly **1 of 19**, resolving →
refused, and putting the first removed name back moves exactly **1 of 19**, refused →
resolving. A cut of eight that moved seven or nine would have been seen.

## The phase owes probes the nineteen rows cannot give

Because the `xterm` row was the whole xterm **family** and not one name. Six prefixes read
out of `vim_is_xterm()` in the source the phase was handed — `xterm nxterm kterm mlterm
rxvt screen.xterm` — every one resolved before and every one is `E522` now, and **not one
of them is among the nineteen**. `xterm-kitty`, which that function already excluded, was
E522 either way. `:set term=builtin_xterm` is the same finding through `term_is_builtin()`,
which strips the prefix and leaves `xterm`.

## Nothing is freed, and the check says why rather than presenting it as a disappointment

`nm -u` is the same 17 symbols, a `comm` empty in both directions: **this phase deletes
data** — three static arrays and eight rows — **and data calls nothing**.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,786 | **79,668 (−118)** |
| `builtin_terminals[]` rows | 10 | **2**, computed as the table minus the two kept |
| `builtin_*` capability tables | 9 | **6** — three taken by the sweep, 103 lines |
| `make editor.c` | 77,876 | **77,758**, 0 directives, 0 errors |
| the boundary | 18 names | **18**, computed from the input at run time |
| `nm -u` | 17 | **17**, a `comm` empty both ways |
| binary | 782,760 | **781,096** |
| terminal rows that moved | | **8 of 19**, each `term=<itself>` → `E522 term=xterm-256color t_Co=256` |
| screen cases / Ex rows / command lines / pty scenarios | | **0 of 102, 0 of 98, 0 of 30, 0 of 4** |

The cut was stated here as its own check counted it, and that was **one more than phase
120's 77,875 on the same file**: this check took the naive `awk` prefix and phase 120's
takes `whim.mk`'s rule, which drops the cut's trailing blank line. Both are the same text.

**That has since been repaired in both programs, and the repair is worth stating because
the defect was a comment.** `phase/121/check.sh`'s header said the cut was *"whim.mk's
own rule"* while the `awk` beside it was not — and 122 had the same line, and is where the
next phase would have copied it from. **A comment that claims to be the rule and is not is
the thing that propagates.** Both now carry the rule entire, `{ a[NR] = $0; if (NF) last =
NR }` and then print up to `last`, with the reason written beside it; so consecutive phase
commits no longer report cut figures that differ by one **on the same files**, where a
reader comparing them saw an off-by-one that was in neither phase. The figures above are
the numbers those checks printed at the time; the product rule makes them 77,875 and
77,757. The boundaries cannot move and did not — a check produces no tree — and both
phases were **verified to re-run at tier 2** rather than assumed to, because a tier-3
replay copies the recorded digest and would have agreed whatever the change did: q121 at
`9f2b37f26bef` and q122 at `68e450fd6912`. **On this tree a control run through `make` has
to print `tier 2` to be a control at all.**

## Its placement

`stage 121`, `package terminal`, which is *what the core still assumes about the terminal
it is attached to*. Phase 85 removed the **question** — the two "not to a terminal"
warnings, the pause and `--ttyfail`; this phase removes the **vocabulary**. The three
packages it is not in each say something: not `harness`, which changes no source, where
this changes what the editor does and declares a delta for it; not `host`, because nothing
crosses the boundary and the cut's warning set is unchanged either side; and not `tidy`,
because the rows removed are live code a `:set term=` still reaches. Four `uses`:
`seed:83`; `harness:116`, the phase that made those rows say something real; `streams:88`,
whose `-T {term}` is the only way into the fallback this phase repairs; and `host:104`,
whose `host_message()` is the only declared way the core reaches stderr and so the only
way to write an instrument into it.

**`apart 120 121`, one direction and both halves measured.** Phase 120's check states its own
size as a line count of the whole file and of the core, and this phase takes 118 more lines
out of the same swept text: `tools/phaserun.sh 120-121` on q119 runs both edits and one
sweep and then stops in phase 120's check with *the file is 79668 lines and the input was
79799* and *the boundary moved by 131 lines and the file by 13*. That is `apart 100 101`'s
shape exactly. The other half was **run** and not reasoned: phase 121's check, given that
stage's swept tree and its own state directory, passes every part, because everything it
asserts is against `$state/old.c`, which its own edit writes. There is **no `need 121`** and
the measurement is reported as vacuous — phase 120's sweep removes nothing, so its unswept
edit output is q120 byte for byte and there is no unswept text here that differs from a
swept one.

**`.reference/core-baselines` is deliberately not re-recorded, and re-recording it would be
wrong.** `term-moved` is cumulative like `2 stderr-moved`: declared here and inherited by
every later phase, so `ref-term.txt` disagreeing with the baseline is exactly what the
declaration says. Phase 116 needed a re-record because the **harness** changed shape; this
phase changes the **editor**.

**And it broke another phase's program, which it measured and declined to repair.**
`phase/116/make.sh`'s section 4 extracted `./whim-vim` from **every** `.build/r*.tar`
and required one terminal table across all of them — a glob that reaches boundaries which
did not exist when the phase ran, so the first later phase to move the table on purpose
makes phase 116's check fail. Measured, with this phase's tar present: *q121 records a
different table:*, exit 1. Repairing another phase's program is a decision and not this
phase's to take, so it was reported; the fix bounds the loop by the phase's own number,
and the rule it states is **a phase may assert anything it likes about the past; it may
not assert that the future will not change what it measured.**
