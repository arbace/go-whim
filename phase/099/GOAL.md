# Phase 99 — the includes nothing names

`phase/099/edit.go` and `phase/099/check.go`, `stage 99`, `package includes`.
`whim-vim.c` inherited **eighteen** preprocessor directives from `whim-vim.c`, every
one an `#include` of a system header, and fifteen phases removed none of them. Six are
now needed by nothing, and this phase takes them — **the first Part II phase to change
that count**, and the only one whose evidence is a byte comparison rather than a
recording.

## Six headers, and three of them have been dead all along

| | why it is unused | since |
| --- | --- | --- |
| `<sys/stat.h>` | supplies **nothing**: its one user is `typedef struct stat stat_T;`, and nothing uses `stat_T` | before the pipeline |
| `<fcntl.h>` | supplies **nothing at all** — `fcntl`, `creat`, `openat` and every `O_*` at zero mentions | phase 92 |
| `<iconv.h>` | supplies **nothing at all**, and nobody had noticed: `iconv` occurs exactly once in `whim-vim.c` and that once is its own `#include` line | whim |
| `<string.h>` | the sixteen `mem*`/`str*` functions | phase 97 |
| `<ctype.h>` | **ten** identifiers | phase 98 |
| `<wctype.h>` | `towlower`, `towupper` — and `iswupper` | phase 98 |

**`<iconv.h>` is the find, and it was missed twice.** `GOALS.md` §II.4 measures this
cut as "79,599 lines and **16 directives**", and `phase/096/edit.go` names two
headers where there are three. Both were counting `<sys/stat.h>` and `<fcntl.h>` and
neither looked at the rest; the answer is **15 directives** at that point and twelve
here.

**`<ctype.h>` is ten identifiers and not five, and the five that are invisible are the
interesting half.** `isalnum`, `iscntrl`, `ispunct`, `tolower` and `toupper` are real
calls and appear in `nm -u`. `isalpha`, `isdigit`, `isgraph`, `islower` and `isupper`
are musl **macros** — `#define isalpha(a) (0 ? isalpha(a) : (((unsigned)(a)|32)-'a') <
26)`, where the `0 ?` arm keeps the prototype visible and is never emitted — so they
are in no undefined set at all and a survey driven by the symbol list cannot see them.
Seventeen occurrences of real source, and `<ctype.h>` does not go until all ten are
handled.

**`iswupper` is the same shape one step further on**: a name the header had to supply
that was never a symbol either. Its one occurrence sits directly after
`return utf_isupper(c);` inside `vim_isupper()`, so gcc never emitted the call. Phase 98
deleted the statement rather than vendoring a function nothing calls.

## The typedef, which is the one silent drop in this file

`typedef struct stat stat_T;` goes in the same edit as its header, and that is not
tidiness. **Removing `<sys/stat.h>` alone compiles cleanly** — the typedef simply
declares a new, *incomplete* `struct stat` at file scope — and what is left is a lie
that only `sizeof(stat_T)` would ever expose. Measured both ways: `-Wall -Wextra
-Wno-unused-parameter` is silent on that file, and a two-line probe using `stat_T` by
value gives *"invalid application of `sizeof` to incomplete type `stat_T` {aka `struct
stat`}"*. The check's loop could not have caught it, because the loop asks the compiler
and the compiler is content.

**And no sweep could ever have taken it, for two textual reasons, both measured.**
`tools/typereach.py` takes as roots every identifier mentioned outside a type
definition, and this definition's name set is `{stat, stat_T}`. It is kept alive by
`update_search_stat()`'s local variable `searchstat_T stat;` **and by the `#include
<sys/stat.h>` line itself**, whose text contains the token `stat`. Measured:
`typereach.py` reports `0 unreachable` on the committed file, `0 unreachable` with only
the include gone, `0 unreachable` with only the local renamed, and **`1 unreachable —
stat,stat_T`** only when both are gone. Fifteen phases of sweeps had left it.

**One blank line goes with it.** The typedef sits between two blank lines, so deleting
the line alone leaves a run of two, which `CLAUDE.md` states this tree does not have —
and which neither verification tier can see. `tools/canon.sh` would collapse it inside
the sweep; the edit does it, so the text the sweep is handed is already right. Six
includes, the typedef and one blank is **eight lines**.

## The argument is a computation and not a list

A phase that deleted six named headers would prove only that six named headers were
deletable. `phase/099/check.go` proves something else, and it is the whole phase:

* **on the output**, each of the twelve surviving `#include`s is removed in turn and
  the compile **must fail**. A dead include that survived this phase would be a compile
  that succeeded.
* **on the source the phase was handed**, the identical loop over eighteen must find
  **exactly the six this phase removes** droppable and the other twelve not. That is
  the same loop proving it can fail, in the same run and on the same code path — phase
  96's `ui_write()` control in this phase's shape.

Thirty compiles, run at once, about five seconds. **gcc 15 defaults to C23, where an
implicit function declaration is a hard error**, so a header that still supplies a
function, a type, a macro constant or an enum constant cannot be dropped quietly:
there is no `-Wimplicit-*` to look for because there is nothing left to warn about.
Every one of the twelve refusals is recorded in the phase's output, so the check is a
statement of *why* each survivor is held as well as a test that it is.

Proven able to fail three ways, each measured on a scratch tree: a live `offsetof`
rewritten to `__builtin_offsetof` leaves `<stddef.h>` dead and the loop names it; the
typedef put back with its header gone is caught by the assertion above and not by the
loop; and one character changed in one string literal is caught by the `cmp`.

## `<sys/param.h>` is not touched, and the reason is recorded rather than repaired

Its **own** contribution to this file is `MIN` and `MAX`. Everything else it supplies
arrives through three levels of musl-internal inclusion — measured with `gcc -E -H`:

```
sys/param.h -> sys/resource.h -> sys/time.h -> sys/select.h
```

and `select`, `gettimeofday`, `fd_set`, `FD_SET`, `FD_ZERO`, `FD_ISSET`, `struct
timeval` and every `*_MAX` are supplied by **no other header in this file** — measured,
one probe per identifier against each of the eighteen. `whim-vim.c` has no
`<limits.h>`, no `<sys/time.h>` and no `<sys/select.h>`. That is a real fragility, and
it is written down instead of being fixed: fixing it means **adding** three directives,
and the charter says no phase adds one. If musl ever reorganises those headers the
build breaks outright, which is the loud failure and the acceptable one.

**Two of the twelve are held by almost nothing**, and the edit counts those exactly,
because the count is the statement. `<stddef.h>` is held by `offsetof` **alone**, nine
mentions — `size_t` and `NULL` come from six of the twelve, so nothing else there is at
risk. `<stdint.h>` is held by exactly **two** identifiers at one mention each:
`SIZE_MAX`, which has held it all along, and `uintptr_t`, **which phase 97 brought** —
the first thing in this pipeline's history to make a header *more* held rather than
less, and the reason the number is two. A later phase that took them would find this
check's loop reporting a dead include.

## The declared delta is nothing at all, and it is a fourth kind

Three phases before this one declared nothing, each for a different reason, and the
reason is the statement: **9** removed code that could not run, **12** removed code that
can run and that the instrument cannot see, **13** removed the possibility. **This phase
changes no code at all**, and its evidence is not that the recording did not move but
that **the binary is the same bytes** — the input's and the output's, both built with
`SOURCE_DATE_EPOCH=0` and the boundary's own flags, compared with `cmp`.

That is tier 1 of `CLAUDE.md`'s verification table, and it subsumes every screen case,
every Ex-command row, every command line and every pty scenario at once, because the
program that would be run is literally the same program. `tools/coredelta.sh --phase 99`
runs and finds nothing moved, as it must; here it corroborates rather than proves.

**`SOURCE_DATE_EPOCH` is required and the file name is not.** `version.c`'s
`__DATE__ " " __TIME__` is the only thing in `whim-vim.c` that a build can vary — there
is no `__FILE__` and no `__LINE__` anywhere — so two ordinary builds of the same bytes
differ, measured, while the same source built under two different names and from two
different directories is identical. The `cmp` is also as sensitive as a comparison can
be: gcc writes a GNU build-id note near the front of the image and it is a hash of the
whole output, so any difference anywhere moves it and the first difference `cmp` reports
is always that note.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 80,440 | **80,432** (−8) |
| `#include` | **18** | **12** |
| other directives | 0 | 0 |
| functions | 1,755 | 1,755 — untouched |
| type definitions | 908 | **907** (−1, `stat_T`) |
| DWARF enumerators | 1,181 | 1,181 |
| `nm -u`, as `phasecheck.sh` counts it | 34 | **34 — the same set, as a `cmp`** |
| `nm -u` with the core's flags | 33 | **33** |
| external symbols | `main` | `main` |
| binary | 805,544 | **805,544, byte-identical** |
| sweep | | **1 round, a complete no-op** |

**Nothing is freed and nothing arrives**, and the check states it as a `cmp` of the
whole undefined set rather than as a count: a header is not code, so a symbol moving in
either direction would mean the phase had done something it does not claim to do. The
sweep finds nothing at all — 0 prototypes, 0 functions, 0 variables, 0 types, 0 fields,
0 enumerators, `canon settled` — and the file it hands back is the one the edit wrote.
`main` is still the only external symbol, which is also the cheapest re-assertion that
phases 97 and 98 did not forget a `static`.

## Its placement

`stage 99`, `package includes`, and four `uses` lines: `includes:99 seed:83 mechanical`,
because the "none" is checked against phase 83's baselines; `includes:99 tidy:96
rationale`, because `phase/096/edit.go` names `<sys/stat.h>` and `<fcntl.h>`,
measures that removing them is free and **declines** — *"the count stays 18"* — so this
phase is that decision reversed; and one line each to `vendor:97` and `vendor:98`, whose vendoring
is the only reason `<string.h>`, `<ctype.h>` and `<wctype.h>` are unused.

**`need 99 swept` is not required, and it was measured.** The anchors are six exact
`#include <...>` lines at one occurrence each and one exact typedef line — text no
sweep has ever touched — and the computed part is a *compile* rather than a count, so
it cannot shrink silently on unswept text the way a counted cut can. Measured: the edit
applies unchanged to unswept text, and the sweep that follows it removes nothing.

**`apart 98 99`, and it is measured in both directions.** Phase 98's check requires the
three headers it emptied to be still present — "it is the includes phase's to take" —
and this phase removes them. And phase 99's check states that it frees **nothing**, as
a `cmp` of the stage's starting undefined set against the one it made; inside a stage
every check compares with the *stage's* start, and phase 98 frees eleven symbols, so in
a shared stage phase 99 says *"the libc surface moved, and REMOVING AN `#include`
CANNOT MOVE IT"* and names them. It is `apart 88 89`'s shape exactly.

**The edit refuses loudly on a tree the vendoring has not reached**, which is what
makes the six a requirement rather than a wish: run on the phase-13 boundary it says
*"`<string.h>` is NOT unused: memchr (1), memcmp (2), memcpy (7), memmove (159) …"* and
exits 1 with the file untouched.

## What whim-vim is after sixteen phases

```
whim-vim.c        80,432 lines          from whim-vim.c's 86,614  (-6,182, 7.1%)
functions         1,755
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      33 with the core's flags, 34 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**Two of those numbers went the other way and that is the trade.** The file is 829
lines longer than it was after phase 96 and the binary 5,728 bytes larger, because
phases 97 and 98 move code *in*: twenty-eight libc functions are `static` definitions
here now instead of names a host has to answer. `GOALS.md` measures bytes to store
**and** libc symbols to provide, and this is the first place the two disagree.

**The 33, attributed.** The whole host boundary that is left is a terminal, a message
line, memory, a clock and the process:

| why | symbols |
| --- | --- |
| **the terminal** | `read` `write` `close` `dup` `ioctl` `select` `tcgetattr` `tcsetattr` `nanosleep` `isatty` (10) |
| **messages before and after the screen** | `printf` `fflush` `stderr` (3) |
| **memory** | `malloc` `free` `realloc` (3) |
| **time** | `time` `gettimeofday` (2) |
| **signals and exit** | `sigaction` `sigaddset` `sigemptyset` `sigismember` `sigprocmask` `kill` `raise` `getpid` `exit` `_exit` (10) |
| **gcc's own**, named nowhere in the source | `__errno_location` `fputc` `fputs` `fwrite` `putchar` (5) |

**Four rows are gone and they were the pure computation**: strings and memory blocks
(17, with `sprintf`), character classes (7), numbers (2), sorting and searching (2).
Phases 97 and 98 put all 28 inside `whim-vim.c` as `static` definitions, `sprintf`
going onto the editor's own `vim_snprintf` rather than being copied. Nothing in the
list above is a function of its arguments alone: every one of the 33 asks the
operating system something.

`tools/symbols.sh` counts 34 because it compiles plain `-O0` and so adds
`__stack_chk_fail`, which the core's `-fno-stack-protector` removes. **`isatty` survives
with three call sites** and belongs to the terminal, not the filesystem —
`mch_check_win`'s `isatty(1)`, `mch_get_shellsize`'s `!isatty(fd) &&
isatty(read_cmd_fd)` and `fill_input_buf`'s `!did_read_something &&
!isatty(read_cmd_fd)`.

**The filesystem work is finished.** Phases 89 to 96 took, in order: every way to write
a file, every way to read one, every way to name another one to edit, the machinery
that read the bytes, the buffer's own name with the last three questions the core
asked a disk on its own initiative, the refusal that asked whether the text had been
saved, the option rows that reported settings nothing read, and the two `FILE *` that
were never opened.

**And the pure computation is finished too.** Phases 97, 98 and 99 took the other half
of *no musl dependencies*: the twenty-eight functions a host should never have been
asked for, and then the six headers that had nothing left to supply. What remains of
the plan (`GOALS.md` II.4c) is the host boundary itself: `main()` demoted to a launcher, the
terminal and the signal set moved out of the core, and the text representation changed
from lines to a tree. **Its §4c expected strings, memory and arithmetic to be what was
left in the core after that move; they are already gone.** Phases 101 and 102 are the
demotion itself: the launcher exists, at the bottom of the same file, and the core
asks it to end the process rather than ending it.
