# Phase 106 — `nullptr` and `usize`

`phase/106/edit.go` and `phase/106/check.go`, `stage 106`, `package boundary`.
`GOALS.md` §II.4c settled the design: **there is no split into two files, there is one
file with two parts, and the first `#include` is the boundary.** The core is the prefix
above it and must name nothing a header supplies. Four phases draw that line; this is
the first, and it is deliberately the smallest **because it is the one that can be
checked by `cmp`**.

Two names the core takes from a header are replaced by two the **language** supplies:

```c
    NULL    →  nullptr                            a C23 keyword; nothing is declared
    size_t  →  usize                              typedef typeof(sizeof(0)) usize;
```

Neither is a new dependency. gcc here defaults to C23 — `__STDC_VERSION__` is
`202311L` — and this file already depends on it for `enum : long`, `static_assert` and
the lowercase `bool`/`true`/`false` it uses throughout. The check states the dependency
as a measurement rather than leaving it implicit: it lifts the typedef line **out of the
output** and compiles it four ways, where gcc's default and `-std=c23` must accept it
and `-std=c11` and `-std=c99` must refuse.

**Both spellings came from the user and both beat what had been proposed.** An
enumerator with the value 0 is a null pointer constant everywhere except a variadic
argument, where it passes four bytes to a callee reading eight **with no warning from
gcc**; `nullptr` is typed, so the hazard does not exist and the rule the phase would
have had to assert for ever is not needed. And `typeof(sizeof(0))` **is** `size_t` on
any target, because `sizeof(0)` has that type by definition — proved in the same
translation unit as the real `<stddef.h>` with `_Generic((usize)0, size_t: 1, default:
0)`, which is the same *type* and not merely the same width. `typedef unsigned long
size_t;` is correct here and silently wrong elsewhere, and silent when it is right, so
nothing in this repository could have told the two apart.

The `#include`s stay at the top. Moving them is phase 110 — 109 when this was written,
before the attributes took the number 24. Eleven directives sit on the
first eleven lines and the typedef on line 13, which is the whole of **+2 lines**.

## The binary is byte-identical, and that is the whole of the evidence

`cmp` of the input's binary against the output's, both built with
`SOURCE_DATE_EPOCH=0` and the boundary's own flags: **788,488 bytes either side, no
difference at all**. That is tier 1 of `CLAUDE.md`'s verification table, and it subsumes
every screen case, every Ex-command row, every command line and every pty scenario at
once, **because the program that would be run is the same program**. `nm -u` holds still
as a `comm` empty both ways, `main` is still the only external symbol, the sweep took
nothing and canon settled in one round. `tools/coredelta.sh --phase 106` still runs and
corroborates; it is not the evidence. It is phase 99's shape exactly, on three thousand
edits instead of seven.

## Every `size_t` was partitioned before any was renamed

`NULL` 2,555 → 3 and `size_t` 437 → 0, and the second number is a **partition and not a
count**. The edit classifies all 437 into

| class | sites |
| --- | --- |
| casts — `(size_t)` and `((size_t)` | **202** |
| declarations — parameter, local, struct field, return type | **235** |
| anything else | **0** |

and **refuses on a leftover**. A leftover would be a use a typedef does not serve — a
case label, an array bound, a `sizeof(size_t)` — and there are none. The classification
is computed from the text, so it stays true of a file this phase has never seen; the
edit asserts no *count* of its input at all, because one rule applied to every
occurrence is correct for any number of them, and pinning the count would make the phase
refuse on a tree that is merely bigger without making a wrong substitution any more
visible.

**The eleven vendored signatures change with everything else, and that is not an
interface change.** `musl_memcpy musl_memmove musl_memset musl_memcmp musl_memchr
musl_strncpy musl_strncmp musl_strncasecmp musl_bsearch musl_qsort` take `usize`
parameters and `musl_strlen` returns one. They have been the core's own `static`
definitions since phases 97 and 98 — nothing outside this file calls them — so renaming
their parameter type changes no contract with anybody.

## Three `NULL`s survive, and the control is what proves they matter

Three string literals in this file contain `NULL`:

```
"E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"
"[NULL]"          the printf layer's stand-in for a null %s argument
"NULL"            what ga_print writes for an empty growarray
```

and no literal contains `size_t`. So the substitution is not a `sed`: it scans the file
for string and character literals first — cheap and exact here, this file having no
preprocessor and no comments — and rewrites only outside them.

**The check builds the literal-unaware form as a control and requires it to differ.**
Measured: a plain line-wise `\bNULL\b` → `nullptr` gives a binary **1,598 bytes
different — 50 in `.text`, 174 in `.data` and 1,354 in `.rodata`** — and `strings` finds
`[nullptr]`, `nullptr` and an E1507 message that names a C keyword at the user. That is
`CLAUDE.md`'s rule that *what must not change is data, and the check for that is the
strings*, arriving on a phase nobody expected it on. Without the control the `cmp` above
is a pair of numbers agreeing, and a test that cannot fail is not evidence.

The survey measured the same control at **1,597 bytes and 49 in `.text`**; it ran it on
q104, and on q105 — the text this phase was actually handed — it is 1,598 and 50. The
`.rodata` and `.data` figures are the same either side, which is what says the extra
byte is code motion and not another string.

## The one-pass rule, which is worth more than the phase

**Both names must be rewritten in ONE pass over the original text**, and that is not
tidiness. A second pass indexes literal spans computed on the **first pass's output**,
and every span after the first replacement is shifted. Measured: the two-pass form
leaves **five of the 437 `size_t` behind** — and leaves a file that still **compiles**,
whose binary is still **byte-identical**, because `<stddef.h>` is still above every line
of it. Every check this phase has passes on that file except the count.

It would have surfaced at phase 110, as five unexplained errors in a move that had
nothing to do with them and nothing pointing back here. **A whole-file substitution is
literal-aware and single-pass**, and `CLAUDE.md` records it as a pattern now rather than
as this phase's incident.

## The thirty `(void *)NULL` become plain `nullptr`, and the survey said eighteen

That is a decision and not a mechanical consequence: a mechanical `\bNULL\b` → `nullptr`
leaves them as `(void *)nullptr`, which compiles and is byte-identical. The cast exists
for exactly one hazard — an untyped null constant in a variadic argument position
passing a four-byte `int` where the callee reads an eight-byte pointer — and `nullptr`
is typed, `sizeof(nullptr) == sizeof(void *)`, so the cast now says nothing a reader
needs. Doing it here rather than later is what keeps those sites from being touched
twice.

**The survey counted eighteen of them and it was simply wrong**, at q105 and at q104
alike. Re-measured, there are **thirty**: 28 comma expressions in the regexp parser,
`return (emsg(…), rc_did_emsg = TRUE, (void *)NULL);`, where the cast was carrying the
comma expression's type, and 2 returns in `get_register`. `nullptr_t` converts to any
pointer type on return, so they are the same program — which the `cmp` says. The
survey's occurrence counts were q104's as well, and phase 105 moved both.

## And one control that moves nothing, reported rather than dropped

Reverting one `usize` to `size_t` compiles cleanly and gives a byte-identical binary,
because the `#include`s are still at the **top** of the file and `size_t` is therefore
still declared above every line of it. That is the honest statement of what this phase's
evidence cannot reach: **the rename is not yet load-bearing**, and it becomes so at
phase 110, where the same control is three hard errors — measured there and exactly
three: reverting one `usize` in `musl_bsearch`'s signature gives two `unknown type name
'size_t'` and one implicit declaration at its call site. It is phase 105's b3/b4 in this
phase's shape.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 80,176 | **80,178 (+2)** — the typedef and its blank |
| functions | 1,756 | 1,756 |
| type definitions | 904 | **905** (`usize`) |
| DWARF enumerators | 1,177 | 1,177 |
| `NULL` | 2,555 | **3**, all three inside string literals |
| `nullptr` | 0 | **2,552** |
| `size_t` | 437 | **0** — 202 casts and 235 declarations, nothing left over |
| `usize` | 0 | **438** — the 437 and its own typedef |
| `(void *)NULL` | 30 | **0** |
| `nm -u` with the core's flags | 17 | **17, the same set**, a `comm` empty both ways |
| `nm -u` as `tools/symbols.sh` counts it | 18 | **18** |
| external symbols | `main` | `main` |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 — `cmp`-identical** |
| sweep | | takes nothing, 0 warnings, canon settles in one round |
| phase | | **18 s** |

## Its placement

`stage 106`, `package boundary`. **The package is `boundary` and not `language`**,
although `language` is what this phase and the plain-host-call phase do: the four are one
idea — 23, the two names the language supplies instead of a header; then the two host
calls that become plain ones; then the header types and macros the core can own; then the
move itself — and `language` would have named the first and the second of those while
leaving the other two in a package that did not describe them. **They were numbered 23 to
26 when this was written and they are 106, 108, 109 and 110**: the attributes were asked for
in between and took the number 24, which is `package dialect` and not this idea at all. Its `uses` are `boundary:106 seed:83 mechanical`,
`boundary:106 vendor:97 rationale` and `boundary:106 vendor:98 rationale` for the eleven
vendored signatures above, and `boundary:106 host:104 mechanical` — this edit puts the
typedef directly below the **last** `#include` and asserts eleven directives on the
first eleven lines, and the eleventh and the count are phase 104's, which took
`<stdio.h>` with the seven symbols it freed.

**`need 106 swept` is not required, and it was measured** in the same run that measured
`apart 105 106`. This edit asserts **no count of its input**, so there is no counted anchor
that could shrink silently; what it asserts is structural — eleven directives on the
first eleven lines, `usize` and `nullptr` at zero, the three literals holding `NULL`, and
the partition — and every one of those held on the unswept text phase 105's edit leaves,
giving the same 2,552, 437 and 30. The one number that differs is the blank-line runs it
preserves, 5 on unswept text against 0 on swept, and it **preserves whatever it is
handed** rather than requiring a value.

**`apart 105 106`, measured, and the refusal is a phase that renamed nothing breaking on a
phase that renamed two type names.** `tools/phaserun.sh 105-106` on q104 runs both
edits and two sweeps and stops at phase 105's check's **first act** — ``iobuff_room` is
not in the output exactly once, so the controls below would not be controls`. Phase 105
writes its four controls by matching the helpers' text **verbatim**, and two of the three
hold `if (IObuff == NULL)`, which this phase spells `nullptr`. Behind that refusal sit
every other literal text it names: `safelen_result`'s clamp is `((size_t)str_l >= str_m)
? …` and all six declarations it requires are `static size_t …`. **One direction
observed**, because 105's check refuses first; what the run does show is phase 106's edit
applying unchanged to phase 105's unswept output, and its own input binary building to the
same 788,488 bytes.

## What whim-vim is after twenty-three phases

```
whim-vim.c        80,178 lines          from whim-vim.c's 86,614  (-6,436, 7.4%)
functions         1,756
type definitions  905
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
va_start          1, in vim_snprintf
NULL / size_t     3 (all in string literals) / 0
```

**Three phases in a row have declared nothing**, and each is a different kind of nothing:
21 the code runs and the instrument sees it do the same thing; 22 the code runs and the
instrument is nearly blind to it, so 263 probes stand in; 23 **the binary is the same
bytes**, which is the strongest kind this pipeline has — phase 99's, and the reason this
phase was made the smallest of the four rather than the first convenient one.
