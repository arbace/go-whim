# Phase 105 — the variadic collapse

`phase/105/edit.go` and `phase/105/check.go`, `stage 105`, `package format`.
C cannot forward `...` — which is why `vsnprintf` exists beside `snprintf` — so a
function that takes `...`, opens a `va_list` and hands it to `vim_vsnprintf` cannot
survive a split unless the formatter goes with it. There are eight such functions.
**Seven are wrappers over the eighth**, and this phase expands every one of their 129
call sites into `vim_snprintf(…)` plus the tail the wrapper ran afterwards.

**`va_start` goes from eight functions to one**, and that is the whole product: no libc
symbol falls, no Ex command goes, no option goes, no message changes, and the binary
gets *bigger*. This is the half of the `va_list` decision that needs only one file, and
it exists separately for the reason `GOALS.md` §II.4c gives — its declared delta must
be nothing at all, provable as a byte-identical recording, which is a far stronger
position from which to make 129 mechanical edits than making them while everything else
is moving.

| wrapper | mentions | protos | own def | **call sites** |
| --- | --- | --- | --- | --- |
| `smsg` | 12 | 1 | 1 | **10** |
| `smsg_attr` | 4 | 1 | 1 | **2** |
| `smsg_attr_keep` | 2 | **0** | 1 | **1** |
| `semsg` | 96 | 1 | 1 | **94** |
| `siemsg` | 12 | 1 | 1 | **10** |
| `vim_snprintf_add` | 3 | 1 | 1 | **1** |
| `vim_snprintf_safelen` | 13 | 1 | 1 | **11** |
| | | | | **129** |

`smsg_attr_keep` has no prototype and `vim_snprintf` has **two**, so a phase that
deletes "the prototype and the definition" for each of seven names fails on the first
and leaves one behind on the second.

## No message logic was written, because the tails already existed

Read each wrapper beside its non-variadic twin and the wrapper *is* the twin with a
format in front of it. `semsg`'s tail is `emsg()`, `siemsg`'s is `iemsg()`, `smsg`'s is
`msg()`, `smsg_attr`'s `msg_attr()`, `smsg_attr_keep`'s `msg_attr_keep(…, TRUE)`. All
five already existed and all five were already called from elsewhere. What is left over
is the two guards, and those become helpers: `iobuff_room()`, `emsg_iobuff_room()`,
`iobuff_or()`, `safelen_result()` and `append_room()` — **five helpers against seven
deleted definitions, which is the whole of 1,758 → 1,756.**

**The size-zero trick is what makes the expansion exactly faithful, and it was measured
rather than assumed.** `vim_vsnprintf_typval` guards every write with
`if (str_l < str_m)` and terminates with `if (str_m > 0)`, so `vim_snprintf(buf, 0, …)`
**writes nothing and does not fault** — measured with a build whose first act is
`vim_snprintf(canary, 0, …)` and `vim_snprintf(NULL, 0, …)`: all eight canary bytes
untouched, no fault on the null destination. So a helper returning 0 reproduces **both**
of the wrapper's guards — `emsg_off > 0` and `IObuff == NULL` — with **no conditional at
any site**. That is why there are five helpers and not an `if`/`else` written out 117
times.

## The sites come in three shapes, and an edit that emits two statements always gets two of them wrong

| shape | sites | what the expansion does |
| --- | --- | --- |
| a plain statement alone on its line | **92** | two lines at the same indentation |
| a **whole block on one line** inside `parse_fmt_types` | **7** | inline on the same line — two lines would put a statement in front of the closing brace |
| **value position** | **30** | a comma expression, the format call then the tail |

The 30 are the 18 `return (semsg(…), rc_did_emsg = TRUE, (void *)NULL);` comma
expressions in the regexp engine, all eleven `vim_snprintf_safelen`s — whose value is
consumed at every site, five of them `+=` — and `vim_snprintf_add`'s one. A statement is
told from an operand by the character after the closing paren. **The comma shape already
existed in the file**, as `return (iemsg(e_internal_error_in_regexp), rc_did_emsg =
TRUE, (void *)NULL);`, so the expansion invents no idiom.

The survey split them 93 / 29 / 7 and put `vim_snprintf_add`'s site in the plain column;
it is in value position, and the implementation's 92 / 30 / 7 is the count that makes
the edit correct.

## All 129 formats are non-literals, which is why the warning list is the check that matters

The thing a reader expects to be a problem is not one, and the measurement is the
opposite of the expected answer: **every one of `semsg`'s 94 formats is `_(e_name)` or
`(const char *)(_(e_name))`**, where `e_name` is a `static char e_name[] = "E123: …";`
array. Whim's constant fold turned upstream's string macros into arrays, so **there is
no string literal at a `semsg` site anywhere in the file.** It changes nothing about the
edit, which copies the format *expression* verbatim into `vim_snprintf`'s third
argument — but it means the build cannot catch a mis-expanded argument list.

What can is `-Wformat=2`. The wrappers carry `format(printf, 1, 2)` / `(2, 3)` / `(3,
4)` and `vim_snprintf` carries `format(printf, 3, 4)`, and every expansion puts the
format expression at `vim_snprintf`'s third parameter — so gcc checks exactly what it
checked before. **115 `-Wformat-nonliteral` warnings in 53 functions before, and the
identical 115 in the identical 53 after**, compared as an exact list equality and not as
two numbers. `_()` and `NGETTEXT()` are `static inline __attribute__((format_arg(1)))`,
so gcc sees through them either side.

**And the invariant fired for real on its first run**, which is the part worth keeping.
It reported `115 warnings in 0 distinct functions`: gcc quotes identifiers as `'x'`
under the phase's locale and as curly quotes under the author's, so the function-name
regex matched nothing and an empty list compared equal to an empty list. The regex
matches both quotings now, and **a zero-function list can no longer pass for an
equality** — a list comparison that can be satisfied by two empty lists is the same
mistake as a test that cannot fail.

The other thing that could have gone wrong was measured too. The expansion mentions the
format **twice** — once in `vim_snprintf`, once in the tail's `iobuff_or(F)` — and over
all 129 sites every format expression is side-effect-free: 118 are `_(e_name)`, a bare
`e_name` or a literal, 8 are `NGETTEXT(a, b, n)` (a pure inline `return`), 2 are a `? :`
over two `_()`s, and 1 is a parameter.

## `nm -u` cannot move for a restructure inside one translation unit, and the check asserts that as an equality

```
nm -u  17 → 17, THE SAME SET
gone: nothing        arrived: nothing
```

A reader meeting a 129-site phase expects a symbol to fall, and none can: a symbol
leaves when its last **caller** leaves the file, and nothing left. `vim_vsnprintf_typval`
still does every conversion in the same file, and `<stdarg.h>`'s three names are macros
and a compiler builtin type, which are no symbol at all. **The binary GROWS — 784,392 →
788,488 — and the check requires it to**, because at `-O0` 129 sites that carried one
call now carry a format call and a tail call. It is the same fact wearing its other
face, and the check reports the number rather than letting it look like a mistake.

What the phase moves is not code across a boundary but the **possibility of drawing
one**: eight functions calling `va_start` cannot be split, one can.

## Two controls move nothing, and they are kept

The recording is nearly blind to this phase, and that is measured rather than asserted.
The input source built again with `write(2, "ZW|<wrapper>|<format>\n", …)` at the entry
to each of the seven, run over the 102 screen cases: `vim_snprintf_safelen` is entered
**617** times, `smsg_attr_keep` **6**, `vim_snprintf_add` **2**, and `smsg`, `smsg_attr`,
`semsg` and `siemsg` **not once**. `semsg` is 94 of the 129 sites and the screen corpus
enters it zero times. So the phase owes probes, and the check runs **263** on both
binaries — the Ex-command errors, 132 regexp errors over both engines and three magic
settings (which are where the 18 comma-expression sites live), the report messages, the
substitute-confirm prompt, undo, CTRL-G and the ruler, and four incsearch probes with a
bad pattern, which are the only way to reach a `semsg` under `emsg_off > 0`. The same
instrument says they enter `semsg` **232 times over 34 distinct formats** and reach **67
of the 129 sites**.

**263 probes, 0 differ. Four deliberate breaks, and two of them move nothing on
purpose:**

| break | records that differ |
| --- | --- |
| both room helpers return 20 instead of `IOSIZE` | **129 of 263** |
| every `semsg` site given `msg()` for a tail instead of `emsg()` | **155 of 263** |
| `safelen_result`'s clamp reduced to `return str_l;` | **0 of 263** |
| all three guards removed | **0 of 263** |

The two zeroes are reported rather than dropped, in the program, the delta file and
here, because **they are the honest statement of what this evidence cannot reach**: the
clamp needs a message longer than 1,025 bytes out of `fileinfo`, and the guards need
`IObuff == NULL`, which is an out-of-memory failure of the first two allocations the
process makes. Reporting them as 0 is the difference between *"the probes prove the
guards are load-bearing"*, which would be false, and *"the guards are correct by
construction and the probes say so about the other two"*.

The other 62 sites are covered by the edit being **one rule applied uniformly** and by
the whole-file equalities above. `semsg`'s unreached sites are out-of-memory reports and
the twenty inside the formatter itself — which fire only on a format string the editor
would have to have got wrong, and every format in this file is one of its own — and
`siemsg`'s ten are the memfile detecting its own corruption.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 80,173 | **80,176 (+3)** |
| functions | 1,758 | **1,756** — seven wrappers out, five helpers in |
| type definitions | 904 | 904 |
| DWARF enumerators | 1,177 | **1,177**, and not one went, arrived or renumbered |
| `va_start` / `va_list` / `va_end` | 8 / 15 / 10 | **1 / 8 / 3** |
| `vim_snprintf` mentions | 73 | **201** — one per site less the redundant second prototype |
| `-Wformat-nonliteral` | 115 in 53 functions | **the identical 115 in the identical 53** |
| `nm -u` with the core's flags | 17 | **17, the same set**, a `comm` empty both ways |
| `nm -u` as `tools/symbols.sh` counts it | 18 | **18** |
| external symbols | `main` | `main` |
| `#include` | 11 | 11 |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 784,392 | **788,488 (+4,096)** |
| sweep | | takes nothing, 0 warnings |
| phase | | **37 s** |

**Four functions still hold a `va_list`** — `vim_snprintf`, `vim_vsnprintf`,
`vim_vsnprintf_typval` and `skip_to_arg`, the positional-argument walker, which
`GOALS.md` II.4c named as three. All four belong below the first `#include` when the
reorganisation comes.

## Its placement

`stage 105`, `package format`. **`format` is a new package and it is deliberately not
`vendor`**: nothing is brought in. A layer is *flattened* — seven wrappers over one
formatter become 129 call sites and five helpers, so that `va_start` appears once and
the formatter becomes movable. Its `uses` are `format:105 seed:83` and
`format:105 harness:86` mechanical, `format:105 vendor:97 rationale` — `musl_strlen` is
what `append_room()` measures the appended string with, and phase 97 is where the core
got its own string functions — and `format:105 host:104 mechanical`, because phase 104
routed `mainerr` and `report_term_error` through `vim_snprintf`, so the mention count
this phase's arithmetic starts from is phase 104's. **The check therefore asserts
`vim_snprintf`'s count only AFTER**, as the transformer's own arithmetic against
whatever it was handed.

**`need 105 swept` is not required, and it was measured** in the same run that measured
`apart 104 105`: this edit finds its sites by word boundary and balanced parens over the
whole file and asserts no counted anchor a sweep can move, and on phase 104's **unswept**
output it finds the same 129 sites in the same 92 / 7 / 30 shapes, 80,174 → 80,182
lines.

**`apart 104 105`, measured, and the first complaint is not the predicted one.**
`tools/phaserun.sh 104-105` on q103 stops in phase 104's check with **``printf` has 4
mentions, expected 10``** — phase 104's own documented counting trap read from the other
end. None of the ten is a call; nine are `format(printf, …)` attributes, and **six of
those nine sit on the wrapper prototypes this phase deletes**. The other three
complaints are ordinary: ``vim_snprintf` has 201 mentions, expected 73``, ``musl_strlen`
has 135 mentions, expected 134`` — `append_room()`'s — and *the file is 80176 lines and
the input was 80148, expected exactly 25 more*. **One direction only**, and it is not
observable in that run because 104's check refuses first.

## What whim-vim is after twenty-two phases

```
whim-vim.c        80,176 lines          from whim-vim.c's 86,614  (-6,438, 7.4%)
functions         1,756
type definitions  904
DWARF enumerators 1,177
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, every one a system header; no #define, no conditional
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
va_start          1, in vim_snprintf
```

**The 17 are phase 104's 17, unchanged**, and that is this phase's claim rather than an
omission. What it produced is not a symbol, a line count or a row but a *shape*: one
`va_start` in the file, which is what the split needs and what nothing before it could
have asserted.
