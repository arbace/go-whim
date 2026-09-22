# Phase 112 — the case tables become one, and it is the union

`phase/112/edit.sh` and `phase/112/check.sh`, `stage 112`, `package casemap`.
`whim-vim.c` carried **two complete Unicode simple-case maps** and they did the same
job: vim's own `toUpper[]`/`toLower[]`, there since whim, and musl's, which phase 98
added as `musl_toUpper[]`/`musl_toLower[]` range-compressed into the same
`convertStruct` shape so that `towupper` and `towlower` could leave `nm -u`. Which one
the editor consults is decided by `'casemap'`. **A core with no C library has nothing
to choose between**, so this phase makes it one table — and the table is the **union**.

## The survey said the two "differ on 2 of 5 probes", and five characters cannot see 193 codepoints

Expanded over the whole of `0..0x10FFFF`, from the file and again from this machine's
libc through `ctypes`, the two disagree at **97 upper and 96 lower** codepoints, at
none of which both map to different characters, and the split is lopsided:

* **vim maps and musl does not, 96 and 96**: all of Vithkuqi, all of Garay, the enclosed
  Latin letters `U+24B6..U+24CF` and `U+24D0..U+24E9`, Glagolitic `U+2C2F`/`U+2C5F`, the
  recent Latin Extended-D additions, `U+019B`, `U+0264`, `U+1C89`, `U+1C8A`. **vim's
  table is simply newer** — it knows Unicode 14's Vithkuqi and Unicode 16's Garay, and
  musl's `casemap.h` predates both.
* **musl maps and vim does not, exactly one**: `U+00DF → U+1E9E`, the sharp s.

So *use vim's* loses the sharp s and *use musl's* loses ninety-six. **Each table knew
something the other did not, and the union is the only answer that keeps both.** It is
**computed, not written down**: the edit expands both tables, refuses on a codepoint
they map differently, requires that no existing row covers one it is about to insert —
`utf_convert()` binary-searches on `rangeEnd`, so a row inside another row is
unreachable — inserts `{0xdf,0xdf,-1,7615}` at its sorted place, and re-expands and
requires the result to be exactly the union. It also parses and re-emits all four
tables **before** changing anything and refuses unless the re-emission is byte-identical
to the text it came from, so the row it writes is in `tools/canon.sh`'s shape by
construction.

## The one row is a deliberate divergence from Unicode, taken knowingly

Unicode's **simple** uppercase of `U+00DF` is `U+00DF`; `U+1E9E` is musl's tailoring,
and putting it into vim's own table changes the **default** `'casemap'`. What it buys is
that the file stops contradicting itself: `swapchar()` has hard-coded `ß → ẞ` for `gU`,
`g~` and `~` all along, so before this phase the table and the keystroke gave different
answers for the same character.

## The delta runs on both arms, and that is what a reader gets wrong

On the **non-internal** arm — `:set casemap=` or `casemap=keepascii`, which read musl's
table and now read the union — 96 upper and 96 lower codepoints **gain a mapping they
never had** and `ß` **keeps** the one it had. On the **default** arm the single row
arrives. Six probe sessions move and six must not, and the six that must not are the
ninety-six proving they did not regress on the arm that always had them, the sharp s
keeping what it had on the arm that always had it, `g~g~` on `ß`, and `:set isk=@` then
`dw` on `café naïve`.

**Two traps the probes had to get right, both measured.** `gU`, `g~` and `~` **cannot
show the sharp s at all**, `swapchar()` hard-coding the mapping before it consults any
table — so the row is reachable only through `\u`/`\U` in a substitution, which goes
`do_upper` → `vim_toupper` → `utf_toupper` and hits the table directly. And the chartab
that the 892 startup calls of `towupper`/`towlower` build **does not move**, although
those calls run with `cmp_flags` still 0 and therefore take the non-internal arm: the
union equals musl's table at every one of `128..255`, the two having disagreed below
`U+0100` at `U+00DF` alone.

## The declared delta is nothing at all, and that is the harness and not the phase

The corpus cannot see any of this — all 102 screen cases seed themselves by typing
ASCII and none touches `'casemap'`, the Ex sweep reads the message a command prints, the
argv records are command lines, the pty scenarios are the window size and raw mode. Two
full recordings are byte-identical in all 106 records, so `phase/112/delta` gains no
line. **That is phase 85's situation — a blind harness rather than a static phase — and a
phase in it owes probes of its own.** Two controls, each computed from the two sources
rather than spelled out: `vimonly` is the output with musl's contribution taken back
out, and the default-arm probe then records exactly what the **input** recorded;
`vimless` is the output with `toUpper[]` replaced by the input's `musl_toUpper[]` — the
merge done the careless way round — and the circled letter goes, which is the regression
no record could report.

**The check's strongest assertion is not a row count.** The produced tables are expanded
over all 1,114,112 codepoints and required to be exactly the union in three directions,
with the musl half **re-derived from libc** rather than from the bytes the phase
deleted, and a perturbed row proving the comparison can fail. Beside it is a rule rather
than a number: what the phase changes on the default arm, and what it stops mapping, are
both **computed** from the two input tables, and every member of both must appear in the
probe text.

## Measured

| | input | after |
| --- | --- | --- |
| `toUpper[]` | 198 rows, 1,477 codepoints | **199 rows, 1,478** |
| `toLower[]` | 183 rows, 1,460 codepoints | **183, 1,460** — musl's lower table added nothing |
| `musl_toUpper[]` / `musl_toLower[]` | present | **gone** |
| lines | 80,222 | **79,857 (−365)** — 358 sixteen-byte rows out and one in |
| `make editor.c` | 78,342 | **77,977**, the same −365 |
| binary | 788,488 | **782,760 (−5,728)** |
| `nm -u` | 17 | **17, the same set** — changing *data* frees no symbol and needs none |
| DWARF enumerators | 1,189 | **1,189**, none gone, arrived or renumbered |
| `options[]` / `cmdnames[]` | 107 / 98 | 107 / 98 |
| records that moved | | **0 of 106**, against twelve probes that carry the phase |

## Its placement

A package of one, `casemap`, deliberately **not** `vendor`: `vendor` is *nothing is
brought in*, and this phase brings nothing in and frees no symbol — what it decides is
what the core's case map **is**, which is phase 95's argument and phase 18's
applied to data instead of to an option row. Three `uses`: `seed:83` and `harness:86`,
and `vendor:98`, because without phase 98 there is one case table already and no union
to take.

`apart 111 112` is measured with `tools/phaserun.sh whim 111-112` on q110: phase 111 states
its arithmetic as a line count of **the core** and stops at *"the core is 77978 lines
and was 78359, a difference of −381 where −16 was expected"* — its own −16 less this
phase's 365. One direction only, measured too: phase 112's check was then run on the tree
that stage leaves and every part of it passed. **No `need 112 swept`**, measured in the
same run.

**Two things about the pipeline this phase ran into, recorded and not acted on.** `make
whim-verify` cannot run while the phase list has a gap — `tools/verifypass.sh` takes the
previous boundary as `r$((first - 1))`, so a reserved-but-unlanded number makes it die
on a missing tar. And `make whim-tip` in a fresh worktree re-runs every phase, because
`git worktree add` gives `whim-vim.c` a new mtime and `$(ZEROBUILD)/input.sha256`
depends on it.
