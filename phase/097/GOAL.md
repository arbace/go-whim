# Phase 97 — the strings are the editor's own

`phase/097/edit.go` and `phase/097/check.go`, `stage 97`, `package vendor`.
Seventeen of the 61 libc symbols whim-vim still asked for are string and memory work,
and every one of them is **pure computation**: no descriptor, no clock, no signal,
nothing the host owns. So they are not a boundary to move, they are code the file can
simply contain. This phase brings sixteen of them in as `static musl_*` functions
written from `/root/musl/src/string/`, and moves the seventeenth — `sprintf` — onto
the printf this editor already carries. `nm -u` goes **61 → 44** and the recording
does not move at all.

## The measurement the phase turned on, and it was the open question

**gcc emits calls to `memcpy` and `memset` for itself**, for aggregate assignments and
large zero initialisers, whatever the source calls. So renaming every call site might
have left both symbols undefined and forced a definition under the **real** name — and
a definition of `memcpy` cannot be `static` without the question of whether gcc's own
emitted call still binds to it, which is the "nothing is global but `main()`"
invariant at stake. Measured on this file and it does not happen: after the rename
`gcc -S` contains **not one call to any of the seventeen**, and all seventeen leave
`nm -u`.

Two things make that a bound rather than luck, both measured. gcc's `-O0` inline-copy
threshold is **between 8 KiB and 16 KiB** — an 8,192-byte struct assignment is
inlined, a 16,384-byte one calls `memcpy`. And `-Wlarger-than=8192` on `whim-vim.c`
reports **exactly one** object above 8 KiB, `options[]` at 13,536 bytes, which is a
table nothing assigns whole, while `-Wframe-larger-than=8192` reports none. The check
asserts the absence from the **assembly** as well as from `nm -u`, so a later phase
that adds a big aggregate and assigns it whole fails loudly rather than quietly
reacquiring a libc symbol.

**Measured and not taken:** `-fno-builtin` and `-ffreestanding` each *add* `abs
fprintf labs` and remove `fputc fputs fwrite putchar` — a different set, not a smaller
problem, and none of it this phase's. `ZEROCFLAGS` is untouched, so this phase edits
no makefile and `whim.mk` needs no change. **And, for the record, because it was the
question asked:** a `static` definition *does* satisfy gcc's own emitted call — a
64 KiB struct assignment beside `static void *memcpy(…)` compiles to `call
memcpy@PLT` and the object has no undefined symbols at all. The escape hatch existed
and was not needed.

## `sprintf` is two populations, and the split is the whole story

Of its **22** occurrences, **thirteen** are ordinary call sites and **nine are inside
`vim_vsnprintf_typval` itself** — which is what `vim_snprintf` calls, so using the
in-house printf for those nine would be circular. The user's decision was to use the
in-house one; it applies to thirteen of the twenty-two and cannot apply to the rest.

The thirteen become `vim_snprintf(dest, size, …)`, each a statement whose return value
was already discarded. **Every size argument is knowable and none is invented**: eight
are `sizeof()` of a visible array or the constant the buffer was allocated with —
`IObuff` is `alloc((1024+1))` and `NameBuff` is `alloc(PATH_MAX)` — three repeat the
`alloc()` expression from three lines above, and one is a pointer **parameter** where
`sizeof(buf)` would be 8 and wrong. `highlight_arg_to_string`'s bound is
`MAX_ATTR_LEN`, and **that is sound only because the function has exactly one
caller**, `highlight_list_arg`, whose local is `char_u buf[MAX_ATTR_LEN]`. Both
programs assert that caller count and pin it at two mentions for ever: a second caller
with a smaller buffer would silently invalidate the bound and nothing else here would
see it.

The nine are narrower than they look. `f` is built twenty lines above the call and is
`%`, an optional `h`/`l`/`ll`, and one of `p d o u x X` — **no flags, no width, no
precision**, because vim does all of those itself in `tmp[]` before and after. So the
nine are "write this integer in this base", and they become `musl_fmtnum()` and
`musl_fmtptr()`, which have no format string and are not a printf. `musl_fmtptr()`
reproduces musl's `%p` exactly — musl's `vfprintf` does `p = MAX(p, 2*sizeof(void*));
t = 'x'; fl |= ALT_FORM`, so a null pointer is `0x0000000000000000` and not glibc's
`(nil)`. The `char f[6]` block goes with them: leaving it would draw
`-Wunused-but-set-variable`, which is in `-Wall`.

## One thing changes on one reachable input, and it is a bug fix

`t_CF` is a **user-settable option** that `term_font()` uses as a **format string**
into `char buf[20]`, and `sprintf` has no bound. Measured: `:set t_CF=` followed by
forty `X` and `%d`, then `:highlight Search ctermfont=3` and a search, exits **−11
(SIGSEGV)** on the binary this phase was handed, and exits **0** here with the output
truncated to nineteen characters. It is the only reachable input on which this phase
changes what the editor does, and the check requires **both halves** — the old one
must die and the new one must not.

It is **not** a declared delta, and the reason is the one phase 95 established:
nothing in the instrument sets `t_CF`, and the only built-in `t_CF` is the `debug`
terminal's `"[CF%d]"`. The same probe records the rest of the difference rather than
hiding it: a user-set `%f`, `%b`, `%*d` or `%z` now renders as vim's own printf spells
it rather than as musl's — `[0.000000]` → `[f]`, nothing → `[1101]`, a garbage int →
the argument, nothing → `[z]` — while `%d` and `%1$d` are identical. **`%s` segfaults
on both binaries** and is pre-existing, not this phase's, and the check says so.

## The case fold is inlined, so that the next phase stays independent

`musl_strcasecmp` and `musl_strncasecmp` **do not call `tolower`**, and musl's do.
musl's `tolower()` in the C locale is `(unsigned)c - 'A' < 26 ? c | 32 : c` and
nothing else — `tolower.c` is `if (isupper(c)) return c | 32; return c;` and
`isupper.c` is `(unsigned)c-'A' < 26` — so the arithmetic is written out. The cast is
what keeps a byte over 127 out of the range test, and the check reads both bodies to
confirm it is there. Inlining costs nothing and it is what keeps this phase and phase
98 orderable either way: phase 98 counts `tolower` mentions, and four new ones here
would have tripped it. **`tolower` is at 2 mentions before and after.**

## Four functions are vendored for code that cannot run, and the check says so

Breaking each moves nothing in the 106-record corpus or in the probes, and the phase
states that rather than offering a probe that cannot fail:

* **`musl_strpbrk`** — its one site needs `P_NFNAME` or `P_NDNAME`, and each of those
  has exactly **two** mentions in the file, its own enumerator and that one test. No
  `options[]` row carries either, so the condition is false always.
* **`musl_memchr`** — its one site is `vim_vsnprintf_typval`'s `%.*s`, and the only
  `%.*s` format string in the file is the OSC-timeout message.
* **`musl_strchr`'s NUL arm** — both call sites pass `'%'`.
* **`musl_fmtptr`** — nothing formats a pointer.

Their correctness rests on musl's source and on a standalone comparison against libc,
not on the recording. `musl_strstr` and `musl_strpbrk` are **naive loops** rather than
musl's two-way and bitset versions, which is the "performance is not a concern"
licence being used deliberately.

## The declared delta is nothing at all, for a third reason

Phase 92's "none" was code that could not run. Phase 95's was code the instrument
cannot see. **This phase removes no code and changes no behaviour**, and a recording
that moved would mean a vendored function was wrong. `diff -rq` over two full
recordings is empty and `tools/coredelta.sh --phase 97` finds exactly the lines phases
85 to 94 declared and nothing new — screen 102/102, `ref-excmds.txt` 111/111,
`ref-argv.txt` 30/30.

**Thirty-three probes run on both binaries and are byte-identical**, and they exist
because the corpus reaches only part of this: every one of the thirteen external
`sprintf` sites (`:highlight`, `:marks`, `:changes`, `ga`, `:set sw?`/`all`, a
recording register, `:set term? t_Co?`, `t_CF` used properly), the numbers the nine
internal ones formatted (CTRL-G, the search count, a `:%s` count, the `Ndd`/`N>>`/undo
line reports, the ruler and its percentage), and the three things nothing else
reaches — `:history SEARCH` and `:history ALL` for the case fold, `:highlight Search
ctermfg=1` twice for `memcmp`, and `:set winhighlight=` for `memcpy`.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,603 | **79,884** (+281) |
| functions | 1,719 | **1,738** (+19) |
| type definitions | 908 | 908 |
| enumerators (DWARF) | 1,181 | 1,181 — **not one value moved** |
| `cmdnames[]` / `options[]` | 98 / 108 | 98 / 108 |
| `sprintf` mentions | 22 | **0** |
| `vim_snprintf` mentions | 55 | 68 |
| `#include` | 18 | 18 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 62 | **45** |
| `nm -u` with the core's flags | 61 | **44** |
| binary | 799,816 | **803,912** |

**The freed set is named and not counted**: `memchr memcmp memcpy memmove memset
sprintf strcasecmp strcat strchr strcmp strcpy strlen strncasecmp strncmp strncpy
strpbrk strstr`, and **nothing arrives**. The binary grows by exactly **4,096 bytes**,
one page — the seventeen libc objects that stop being linked in roughly pay for the C
added. The sweep removes **nothing**, in one round, and `canon.sh` settles on the
first: the vendored text is already in the file's shape. The phase is **29 s** and its
boundary is `17a649164516`.

**The nineteen definitions are written the way the rest of the file writes one** —
the return type indented four spaces on its own line, the name at **column 0** — and
that is not cosmetic. `tools/funcreach.py` reads a definition as
`^([A-Za-z_]\w*)\([^;\n]*\)[ \t]*$`, so a one-line header is invisible to it. This
phase first emitted them on one line and the reachability sweep counted 1,719
definitions afterwards, exactly what it counted before the phase ran; phase 98's agent
found it. Nothing was broken by it — the block calls no vim helper, so nothing could
be orphaned, and `-Wunused-function` still covered a dead one — but it was a trap for
the first phase to make one of these call into the editor, and it was repaired while
it was cheap. The repair is pure formatting and its check is **tier 1**: both sources
built with `SOURCE_DATE_EPOCH=0` give a binary of 805,544 bytes and `cmp` says
byte-identical.

## Its placement

`stage 97`, `package vendor`, and two `uses` lines: `vendor:97 seed:83 mechanical`,
because the "none" is checked against phase 83's baselines, and `vendor:97 harness:86
mechanical`, because the one must-differ probe is a **screen** record of a binary that
segfaults — the old file-based sweep recorded an exit status and could not tell a
crash from a quit.

**`need 97 swept` is not required**: every anchor is exact text or a counted
identifier, and the phase is a stage of one, so its input is a boundary and is swept
by construction. **`apart 96 97` is real and is not written**: phase 96's check
asserts the libc surface moves by exactly `fclose fsync getc putc` and names `fputs
fputc fwrite putchar` as still undefined, and this phase moves seventeen more — but a
stage holding 96 and 97 holds them adjacent and every Part II phase is its own stage, so
it is the shape of the missing `apart 85 89`. **`apart 97 98` is phase 98's**, and it is
the one that matters: this check pins `tolower` and `toupper` and requires both still
undefined, and phase 98 takes them.
