# Phase 98 — the character classes, the numbers and the sort

`phase/098/edit.sh` and `phase/098/check.sh`, `stage 98`, `package vendor`. Phase
97 took the strings; this takes everything else in `whim-vim.c` that is **pure
computation** — a function of its arguments that asks the operating system nothing —
and defines it in the file, as plain C, with no preprocessor and no comment. Eleven
undefined symbols go: `tolower toupper towlower towupper isalnum iscntrl ispunct`, the
character classes; `atoi atol`, the numbers; and `qsort bsearch`.

## `nm -u` is not the scope, and that is why the phase is bigger than that list

musl spells six more classifiers as **function-like macros** in `include/ctype.h` —
`isalpha isdigit isgraph islower isupper`, and `isspace` through `__isspace` — so a
source that calls them produces **no undefined symbol at all**. Five of them are live
here, at **seventeen sites**: `isdigit` 7, `isupper` 5, `isalpha` 3, `islower` 1,
`isgraph` 1. Until they go, `<ctype.h>` cannot, and a phase scoped by the symbol list
would have looked finished and left phase 99 unable to move.

So this phase asserts **phase 99's contract itself**, in both directions: a copy of the
produced source with `#include <ctype.h>` and `#include <wctype.h>` deleted must
compile **silently**, and the same deletion on the source the phase was handed must
fail — measured, **13 errors**. That pair, and not a grep, is what says phase 99 can
move. Neither copy goes near the tree; this phase changes no directive and the count
stays 18.

**The sign-extension question is moot at every one of the seventeen sites, twice
over.** Fifteen already cast to `(unsigned char)`, and the two that do not are
`regatom()`'s, where `int cu` runs `1..127`. And the replacements are **total over
`int` and bit-for-bit equal to musl's for every argument**: EOF, `INT_MIN` and
everything outside 0..255 give false for every classifier and the identity for both
mappings, exactly as musl's do. Measured once, over **all 4,294,967,296 `int` values:
zero disagreements**, in 66 seconds.

## The obvious reading of `towupper` and `towlower` is wrong

Their one live arm each is `if (!(cmp_flags & CMP_INTERNAL)) return towupper(a);`
inside `utf_toupper()` and `utf_tolower()`, and `'casemap'` defaults to
`"internal,keepascii"` — so they look unreachable without `:set casemap=`. **An
instrumented build marks 104 of the 106 records of a full recording, 892 times in one
trivial session.** The caller is `utf_islower()`/`utf_isupper()` from
`buf_init_chartab()`, running **before `'casemap'` has been applied**, with `cmp_flags`
still its static zero and arguments in **exactly 128..255**. They classify the whole
Latin-1 range at startup, and nothing in the source says so.

The other two mentions, in `vim_toupper()` and `vim_tolower()`, sit on the line after
an unconditional `return`, and gcc drops the unreachable block even at `-O0` — which is
why `iswupper`, whose **only** mention has that shape, is a name in the source and not
a symbol in the object.

## Four answers were built, recorded and probed

| | lines added | binary | corpus | the case probes |
| --- | --- | --- | --- | --- |
| musl's `casemap.h` verbatim | +548 | unchanged | identical | identical |
| **range-compressed (taken)** | +557 | +1,632 | identical | **identical** |
| vim's own table instead | +166 | −4,096 | identical | 2 of 5 differ |
| ASCII only | +188 | −4,096 | identical | 2 of 5 differ |

musl packs the mapping into a two-level base-6 table, 297 lines and forty lines of
`(v*mt[y]>>11)%6` bit arithmetic: exact, and the opposite of obvious idiomatic C.
Routing the calls to vim's own table instead costs **97 upper and 96 lower codepoints,
96 and 96 of them above U+00FF** — the one below, U+00DF, is neutralised twice in the
source, by `utf_islower()`'s `|| a == 0xdf` and by `swapchar()`'s hard-coded ß→ẞ.

**The ASCII fallback is rejected with a number, not an opinion.** It misclassifies
**62 of 128** Latin-1 bytes at startup — every accented letter and µ — and *every
harness here says it is fine*, because `'isprint'` (`"@,161-255"`) and `'iskeyword'`
(`"@,48-57,_,192-255"`) re-cover the same bytes by range: `g_chartab` and `b_chartab`
come out byte-identical under all four variants, and stay identical under `:set isk=@`
and `:set isp=@`. A variant whose only defence is that two option defaults happen to
paper over it is not one to ship.

What runs is the fourth: **musl's mapping range-compressed into the shape this file
already has**, `187 + 171` `convertStruct` rows read by `utf_convert()` — the same size
as vim's own 198 + 183, and read by the same function. Exact, and in the file's own
idiom. **Dropping `'casemap'`'s non-internal arm is a phase of its own and deliberately
not this one**: it would delete those 358 rows and take the binary down 5,728 bytes,
and deciding what `'casemap'` means in a core is a different idea from vendoring.

## The two data files are not remembered constants

`tools/musl-case.txt` is generated by `tools/muslcase.py --generate`, which reads
**this machine's libc** through `ctypes`; `--verify` re-derives every one of the
**1,114,112** codepoints from the same authority and refuses on one disagreement, in
1.4 s. A musl upgrade that moved one codepoint fails the phase rather than passing it
quietly. `tools/musl-ctype.txt` is the seventeen functions, and `tools/muslctype.py
--verify` slices them **out of the source the phase produced** — not out of the data
file, which would only prove the copy was faithful — compiles them with `-Wall
-Wextra` and runs them beside libc's, in 0.35 s. Both are proven able to fail:
perturbing one `convertStruct` offset, one `& 0x5f`, one comparator direction and one
`return` in the binary search each makes the matching tool refuse.

**`muslctype.py`'s domain is bounded on purpose and its docstring says so.** The 2³²
sweep costs 66 seconds, which is more than the rest of the phase, and a check that
doubles a phase's time stops being run. What runs is every int in [−1024, 1024], every
threshold in the definitions and its two neighbours, EOF, `INT_MIN`, `INT_MAX` and a
**fixed** pseudo-random sample of 1,000,000 — 1,002,140 values. Every one of the eleven
is a closed form in `(unsigned)c`, so a disagreement anywhere is a disagreement at a
threshold, and every threshold is in the bounded set.

## Three anchors and sixteen counted rewrites

**A — the seventeen functions and two prototypes**, at the **end of the block phase 97
started**, between the last `#include` and the enum wall, so the file has one vendored
block and not two. The anchor is the junction itself, the end of `musl_fmtptr()` and
the first enum. The prototypes are needed only because the two **dead** `return
towupper(c);` mentions are rewritten too and sit below.

**They use the file's two-line definition style, and that is mechanical rather than
cosmetic.** `tools/funcreach.py` finds a definition with `^([A-Za-z_]\w*)\(…\)$` —
the *name* at column 0, which is what `    static int` on its own line gives. Phase
97 wrote one-line headers, and **measured, `funcreach.py` saw 1,719 definitions on
this phase's input, exactly what it saw before phase 97 ran**: not one of its
nineteen was in the reachability graph. Nothing was wrong — the block calls nothing
`funcreach.py` tracks, and `-Wunused-function` still covers a dead one — but a
function written that way is outside the sweep, so these seventeen are written the
way the other 1,719 are.

**B — the two tables**, after vim's own `toUpper[]`, so musl's sit beside vim's and
`utf_convert()` is already declared above them.

**C — `return iswupper(c);`, deleted** rather than vendored, and that is measured
rather than argued: same file name, `SOURCE_DATE_EPOCH=0`, the binary is
**byte-identical either side**.

**D — sixteen counted rewrites, 44 call sites**, applied only to the text *after* the
inserted block, so that no vendored body rewrites itself — `musl_ispunct()` calls
`musl_isalnum()`, and a file-wide regex would have made `musl_tolower()`'s body read
`musl_musl_tolower`.

**The check counts them as a rule and not as a table of constants**, and writing it
that way is what found that they are **44 and not 42**: for each name it requires
`calls(new, "musl_N") == calls(old, "N") + calls(vendored text, "musl_N")`, both halves
read at run time, so it says *every site moved and none was invented* on whatever input
it is handed rather than on the one it was written against.

## Nothing in `tools/sweep.sh` covers an unreachable statement

gcc's `-Wunreachable-code` has been a no-op since gcc 4.5 and is not in `-Wall
-Wextra`; `deadsweep.py` acts on warnings; and `funcreach.py` and `typereach.py` read
*definitions*, so a libc name with no definition here is invisible to them. An
unreachable statement is a **sixth kind of dead thing** beside the five the sweep
knows. Measured: `whim-vim.c` holds **eighteen** of them and exactly **one** names a
libc symbol. This phase takes that one. The other seventeen are a phase of their own —
three are the folded Latin-1 arms of these same four functions, and deleting those
would orphan `latin1flags`, `latin1upper` and `latin1lower`, which would turn a
vendoring phase into a cut. The check requires all three to **survive**, because a
check that expected them at zero would fail on a correct phase.

## `qsort` is stable on purpose, and it cannot matter

`sort_strings()` has one caller, `ex_undolist()`, and the keys begin `"%6ld"` of
`uh_seq`, which is `++curbuf->b_u_seq_last` — **one assignment in the whole file** —
so `strcmp` can never return 0 there. The replacement is an insertion sort, **stable**
where musl's smoothsort is not: where they could differ it returns the input order,
which is a function of the input alone, and that is what a memoized pipeline wants.

Measured three ways. The same `:undolist` row order as the input binary over five runs;
an **anti-stable** build (`>=` for `>`) giving the same order too, because there are no
ties; and a **reversed** build giving `5 4 3 2` where the others give `2 3 4 5`, which
is the control that proves the probe can see the sort at all. `tools/muslctype.py`
proves the converse in C, where the editor cannot: on three equal keys the two sorts
place **2 of 5** pointers differently while sorting to the same strings.

**`:undolist` is not visible in the screen snapshots** — the Press-ENTER redraw wipes
it before the `\x1b[?25h` that ends a step — so the probe reads the row order out of
the raw stdout stream, and compares the order rather than a digest, because the rows
carry `"0 seconds ago"`.

## `bsearch` is a binary search, and one of its four tables has a duplicate row

All four were checked by running their real comparators over their real data:
`highlight_tab` 13 rows, `color_name_tab` 28 and `char_class_tab` 19 are strictly
increasing; **`key_names_table` has two adjacent rows both spelled `"Tab"`**, one
carrying `TAB` and one `K_TAB`. It is still sorted, and it is not a hazard, because
`get_special_key_code()` ends `return key == K_TAB ? TAB : key`. Measured: both
binaries resolve `<Tab>` to row 100.

musl's algorithm is kept line for line rather than a linear scan, for two reasons that
are both about failure modes: a scan would silently **repair** a future unsorted table,
and the binary search resolves that tie identically. Against libc: **1,845 lookups**
over sizes 0..40 and every key, **the same pointer every time** — which is stronger
than "it found something".

## `atoi` and `atol`

musl routes both through `strtol`-shaped code; the replacement is the plain loop — skip
`isspace`, an optional sign, then accumulate **negatively** so that `INT_MIN` and
`LONG_MIN` do not overflow on the way in. **Overflow is undefined in the real ones
too**, and these are undefined in the same place and no other; all ten call sites can
already be handed arbitrary digits by a user or a terminal. `getdigits()` does not skip
the whitespace `atol` skips — upstream's, unchanged, and a tidier `atol` would have
changed it. Measured against libc: **137,560 strings**, every one of length 1..4 over an
alphabet holding whitespace, both signs, digits, letters and the two high bytes `\x80`
and `\xff` — **zero disagreements**.

## The declared delta is nothing at all, and it is a third kind

Phase 92's was code that **could not run**; phase 96's was a **possibility that had
never existed**; this one is an **equality**. The code it replaces runs constantly, so
the evidence is equivalence rather than unreachability and **every probe is a
must-not-differ**.

And the corpus reaches none of it: every one of the 102 screen cases seeds itself by
typing ASCII, so nothing in it touches Unicode case folding, `'casemap'`, the four
`bsearch` tables or the sort. **Fourteen probe sessions run on both binaries beside the
recording** — six over an ASCII, a Latin-1, a Greek, a Cyrillic, a circled Latin and a
Coptic letter under every `'casemap'` the option can hold, four for the four `bsearch`
tables, three for `atoi` and `atol`, one for the chartab those 892 startup calls build,
and `:undolist`. **The probe text holds two groups on purpose**: the first three
letters separate an ASCII fallback and the last two separate vim's own table, and a
text with only one group would pass one of the two wrong answers. Validated against
both: with the vim-table build `case_empty` and `case_keepascii` move; with the ASCII
build they move **and** stop showing the Greek capital.

Two full recordings, the binary the phase was handed against the one it made, are
byte-identical — all 102 screen cases, all 111 Ex-command rows, all 30 command lines,
the four pty scenarios and the nineteen terminal rows — and `tools/coredelta.sh --phase
98` finds the same against whim-vim's frozen baselines. `phase/098/delta` gets a
comment and no line.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,884 | **80,440** (+556) |
| functions | 1,738 | **1,755** (+17) |
| type definitions | 908 | 908 |
| enumerators (DWARF) | 1,181 | 1,181 — none gone, none arrived, none renumbered |
| `#include` | 18 | 18 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 45 | **34** |
| `nm -u` with the core's flags | 44 | **33** |
| binary | 803,912 | **805,544** |

**The binary grows, and that is the measurement rather than a disappointment.** The 358
rows are data the image did not carry, and the musl objects they replace were smaller
because musl packs the same mapping into 16,998 bytes. Phase 97 was the first phase
in this pipeline to make the file longer and the image bigger; this is the second.

**Eleven symbols go and the check names the set, not the count** — `atoi atol bsearch
isalnum iscntrl ispunct qsort tolower toupper towlower towupper`, with nothing
arriving — and **five more identifiers leave the source with no symbol to show for it**,
`isalpha isdigit isgraph islower isupper` being macros in musl. The check asserts the
terminal, the memory, the message layer, the clock and the signals still undefined
beside them, and `GOALS.md` §II.4b's invariant again.

The functions row is `funcreach.py`'s, and the **+17 is this phase's seventeen**. Both
absolute numbers here are 19 higher than they first read, because phase 97 emitted its
nineteen definitions with the header on one line, where `funcreach.py` cannot see them;
this phase's agent found that, phase 97 was restyled into the file's own shape, and the
difference the row records did not move.

The sweep is **1 round** and removes **nothing** — every one of the seventeen new
functions is called — and `tools/canon.sh` is a **no-op** on the output, so the
inserted text is already in the file's canonical form. The phase is **41 s**: 7 s the
edit and its build of the input binary, 6 s the sweep, 24 s the check and 4 s
`tools/coredelta.sh`. `make whim-tip` is 48 s. Its boundary is `b1b12e506ceb`.

## Its placement

`stage 98`, `package vendor` with phase 97, and two `uses` lines: `vendor:98 seed:83
mechanical`, because the "none" is checked against phase 83's baselines, and `vendor:98
harness:86 mechanical`, because "nothing moved" is a statement about a zero recording
and a zero recording is what phase 86 made one. Phase 97 is in the same package, so the
ordering between them is the package's and not a `uses`.

**`need 98 swept` is not required, and it was measured**: every anchor is exact text,
every count was taken on unswept input, and the sweep removes nothing here.

**`apart 97 98`, measured.** Both checks state that the undefined set moved by exactly
**their** symbols — phase 97's seventeen strings, phase 98's eleven — and a stage
holding both takes one symbol snapshot at the stage's start. Run with a snapshot from
before phase 97, phase 98's check reports **28 symbols gone instead of 11** and exits
1. Phase 97's check fails the other way for a reason of its own: it pins `tolower` at
**2 mentions** and names it as the character-class phase's, and on the tree phase 98
leaves it is **0**.

`make whim-verify` reproduces every boundary in **110 s** of wall time over 863 s of
phases -- sixteen, q83 to q98, when this phase was written, and seventeen since -- and
all 107 whim and slim implementation keys are unchanged.
