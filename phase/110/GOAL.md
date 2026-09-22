# Phase 110 — the move: the first `#include` becomes the boundary

`phase/110/edit.sh` and `phase/110/check.sh`, `stage 110`, `package boundary`.
This is what the pipeline had been clearing the ground for. **The eleven `#include`s
move from the first eleven lines to line 78,360, and above them there is not one
preprocessor directive.** `whim-vim.c` is now one translation unit with a core editor on
top, written in plain C with no directives at all, and a host below that begins with the
includes. **The first `#include` IS the boundary**, marked by nothing else — no comment,
no banner, no name — and `make editor.c` writes the **78,358 lines** above it.

That target existed before this phase, writing an empty file on purpose, so that the
phase that fills it would change the source and not the makefile. It did.

## The order was forced, and it is why 109 and 110 are two phases

```c
    enum : int { INT_MAX = (int)(~0u >> 1) };       placed AFTER <limits.h> is
    enum : int { 0x7fffffff = (int)(~0u >> 1) };    a syntax error
```

So the derived constants can only be written **once the includes have moved**, and phase
109's sixteen `static_assert`s against the headers can only be written **while they are
still above**. Two phases, in that order, and neither could have held the other's work.
The check proves the first half on the product rather than arguing it: with `<limits.h>`
put back at line 1 the build stops on that line with *expected identifier before numeric
constant*.

## The constants are asked for, not remembered

The edit performs the move, compiles the cut alone, and **collects every name gcc says is
undeclared: 23 errors naming exactly twelve**. A thirteenth would mean phase 109 did not
finish; one this program declared and gcc did not ask for would be a declaration nobody
needs. Eight are derived from the type system and four asserted:

```
    INT_MAX 14   INT_MIN 2   LONG_MAX 51   LONG_MIN 1        derived: (int)(~0u >> 1) and kin
    LLONG_MAX 3  LLONG_MIN 1 ULLONG_MAX 10 SIZE_MAX 1
    PATH_MAX 12  EXIT_FAILURE 1  SIGHUP 2  SIGTERM 2         asserted against the header below
```

(the counts are the core's mentions at q109). **All twelve are enumerators**, including
the four asserted ones, because `PATH_MAX` is an **array bound** — and a `static const
int` cannot appear in an array bound, a case label or an enumerator initialiser
(`CLAUDE.md`, *Add a constant*).

Re-measured from the product: delete the twenty enumerator lines from `editor.c` and gcc
gives the same **23 errors over exactly those twelve names**, `INT_MAX` eight times,
`PATH_MAX` five and the other ten once each.

## Below the includes, `INT_MAX` IS the macro, so each assert restates the derivation

This is the design point the brief could not have foreseen, and it is what the move costs.
Phase 109 could compare every core-owned spelling against the header still above it. Here
the headers are **below**, and below them the name `INT_MAX` is `<limits.h>`'s macro — so

```c
    static_assert(INT_MAX == INT_MAX, "INT_MAX");        a tautology about the header
    static_assert((int)(~0u >> 1) == INT_MAX, "INT_MAX");  what the phase writes
```

The twelve asserts the phase puts **into the product** compare the **deriving
expression** against the header, and the left-hand side is not typed twice: it is emitted
from the same table as the enumerator's own initialiser, and the check reads both back
out of the source and requires them equal as text. Measured both ways — the wrong
derivation in both places is `static assertion failed`, and **the same wrong enumerator
with the assert written the naive way builds in silence**.

## What moves below is computed to a fixpoint, not listed

The four functions that hold a `va_list` are named, because `va_list` is `<stdarg.h>`'s
and the core cannot declare it: `vim_snprintf`, `vim_vsnprintf`, `vim_vsnprintf_typval`
and `skip_to_arg` — the fourth being the one `GOALS.md` §II.4c missed and phase 105
counted. **Everything else follows from compiling the cut**: move what gcc calls unused,
compile again, repeat.

The brief's single round is only the first of **five**. The fixpoint takes **15
functions, 18 objects and 3 enum blocks** — the formatter's whole private island, down to
`musl_strchr`, `format_typeof` and the eleven `typename_*` strings — where one round
takes seven things. The stopping rule is `vim_main` and `deathtrap`, the two the **host**
calls, which are unused above the cut by construction and stay there. Enum blocks are
found by **counting**, not by compiling: no warning gcc has can see a dead enumerator
(`CLAUDE.md`).

That is **78,358 lines** in the cut where one round gives 79,079, and it is the right
answer because **the cut is the deliverable**: shipping 15 dead functions inside it would
be a defect, and *nothing above the boundary is dead* is now a checkable sentence.
Measured on the product, the moved island is lines 78,385–79,952 — **1,568 lines** — and
1,874 lines sit below the cut in all, of which the 280-line host block was already at the
bottom and did not move.

## There is no `cmp` to be had, so tier 1 moved up a level, onto the source

Every address below the first moved definition moves with it, so the binary cannot be the
evidence and the phase does not pretend otherwise. What replaces it is `CLAUDE.md`'s tier
1 **one level up**, stated and checked as a **multiset**:

> not one of the input's 80,197 lines is missing from the output, and the only lines the
> output adds are the **32** this phase writes — twenty enumerator lines and twelve
> `static_assert`s — plus three blanks where an emptied paragraph left two.

**A phase that moved code and altered a character of it on the way could not say that.**
Re-measured independently by sorting both files: 0 lines missing, 35 added, and the 35
are exactly the 32 and three blanks. 80,197 → 80,232 lines.

The recording answers for the thirty-two: two full recordings, of the binary the phase
was handed and of its own, **byte-identical across all 106 records**. `nm -u` is the same
17 names in both directions — **moving a definition inside ONE translation unit frees
nothing and needs nothing**, because a symbol leaves when its last caller leaves the
*file* — and `nm --extern-only --defined-only` is still exactly `main`. The claim of this
phase is structural and not a symbol count.

## The cut's own check is four parts, and three of them are silent in an ordinary build

`awk '/^ *# *include / { exit }'` — one clause, no judgement — gives the prefix, and then:

```
    0 lines beginning with #                 a #define above the cut gives 1
    a floor of 70,000 lines                  an #include back at line 1 gives a cut of 0
    0 errors under -fsyntax-only
    a warning set EQUAL to the declared boundary
```

The fourth is the interesting one. The cut's warnings **are** the core → host interface:
**thirteen names, every one `used but never defined`** — `vim_snprintf`, `host_exit`,
`host_message` and the ten `musl_*` — computed a second way from the text, as the names
defined below the cut and mentioned above it, and required to match. Verified here
independently: 0 errors, 13 warnings, and the cut an exact byte prefix of `whim-vim.c`.

Each of the three mistakes is built both ways in the check, and **each is silent in the
ordinary build**: a `#define` above the cut, an `#include` back at line 1, and one core
function — `elapsed` — quietly moved below the boundary, which changes the thirteen by
exactly its name. Nothing else in this pipeline can see any of them.

## Two corrections, and one of them was in the makefile

**The brief's `static` trap is wrong.** It says a `static` libc prototype above the
boundary makes the link fail. It does not: gcc gives `<stdlib.h>`'s own declaration
internal linkage too, warns on **that** line — *'malloc' declared 'static' but never
defined* — links against libc regardless, and produces a binary `cmp`-identical to the
product's. So the trap phase 109 caught with a hard error is now a warning, and what
stands between the core and it is the sweep's rule that the build print **nothing**, plus
this check's assertion that none of the nine prototypes is `static`.

**And `whim.mk`'s `editor.c` guard was wrong, in a way it could only be once the cut was
not empty.** It refused if the cut held a `#` of **any** kind — but `#` is an ordinary
character, and the editor is full of it:

```c
    enum { CPO_HASH = '#' };
    if (ptr[0] == '#')
    "E1281: Atom '\%%#=%c' must be at the start of the pattern"
    the two latin1 case tables
```

Measured on the first cut this rule ever produced, **63 lines** hold one and none is a
directive — so the guard would have refused every valid cut for ever. It is `^ *#` now,
which is what the rule's own paragraph already said it meant, and it is 0 on the cut and
11 on the whole file. The agent was told not to touch that file and changed it anyway,
with the measurement and a flag saying so, which was the right call. **The phase's merge
commit says 54 lines and the tree says 63**; 63 is the number `whim.mk` and the phase
commit carry, and the number reproduced here.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 80,197 | **80,232 (+35)** — 32 written, 3 blanks |
| input lines missing from the output | | **0**, as a multiset |
| first `#include` | line 1 | **line 78,360** |
| directives above the first `#include` | 11 | **0** |
| `make editor.c` | an empty file | **78,358 lines**, 0 directives, 0 errors, 13 warnings |
| moved below | | 4 va_list functions + 15 functions, 18 objects, 3 enum blocks, 5 rounds |
| the island's extent | | lines 78,385–79,952, **1,568 lines** |
| constants written above | 0 | **12 enumerators**, 8 derived and 4 asserted |
| `static_assert` in the product | 1 | **13** — the `cmdnames[]` one and this phase's twelve |
| functions | 1,757 | 1,757 |
| type definitions | 905 | **909** |
| DWARF enumerators | 1,177 | **1,189 (+12)** |
| `nm -u` with the core's flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 bytes, and not the same bytes** |
| records that moved | | **0 of 106** |
| sweep | | takes nothing, canon a no-op |
| phase | | **64 s**, the longest any Part II phase has taken |

## Its placement

`stage 110`, `package boundary`, the last of the four. Its `uses` are
`boundary:110 seed:83 mechanical` and `boundary:110 harness:86 mechanical`, the evidence
being a recording; `boundary:110 format:105 mechanical`, because what moves below is **four**
functions and not eleven only because phase 105 collapsed the seven `va_list` wrappers into
their call sites; `boundary:110 host:103 mechanical`, the includes landing immediately above
`host_winch_pending`, the first line of the block phase 103 created and where
`tools/zhostonly.py` starts reading; and `boundary:110 host:104 mechanical`, the eleven
directives on the first eleven lines being what phase 104 left, which this edit asserts
before lifting the block.

`tools/zhostonly.py` gained two named exceptions here and nothing else:
`<file scope>` now says `SIGHUP` and `SIGTERM` twice each — the core's own
`enum { SIGHUP = 1 };` and the `static_assert` below the includes that checks it — both
outside the host region because that region begins at `host_winch_pending` and the
includes are above it. Measured: the tool passes on the phase's input and on its output,
and the edit moves **four Part II unit keys (103, 104, 108, 109**, the phases whose checks name
it**) and no whim, whim edit or slim key at all**.

**`apart 109 110` is measured, not predicted**, by running phase 109's check on the tree
phase 110 leaves: it stops with *"the output is 80232 lines and the input was 80173, a
difference of 59 where 24 was expected"* and *"the output does not have exactly eleven
directives on its first eleven lines"* — and behind those, its sixteen `static_assert`s
and four mismatched declarations **cannot compile at all**, because every one of them
needs the headers above the core. One direction only: phase 110's own check passes on that
stage. **There is no `need 110`**, also measured — the edit applied to phase 109's unswept
output gives a `whim-vim.c` `cmp`-identical to the swept path's, and it asserts no count
of its input that a sweep could move.

## What whim-vim is after twenty-seven phases

```
whim-vim.c        80,232 lines          from whim-vim.c's 86,614  (-6,382, 7.4%)
                  78,358 above the boundary, 1,874 below it
functions         1,757
type definitions  909
DWARF enumerators 1,189
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, at line 78,360, and NOT ONE DIRECTIVE above them
core -> host      13 names: vim_snprintf, host_exit, host_message, ten musl_*
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
make editor.c     78,358 lines: 0 directives, 0 errors, 13 warnings, all of them
                  `used but never defined` and all of them the interface
```

**`GOALS.md` §II.4c is built out.** Its design was one file with two parts and the first
`#include` as the boundary, and the four phases that draw it are done: 23 replaced the two
names a header supplied with two the language does, 108 made the two host calls plain, 26
gave the core its own types and macros, and 27 moved the includes. **Seven phases in a row
have declared nothing** — 104 through 110 — and among them are five distinct kinds of
evidence: the code runs and the instrument sees it (104, 108, 109's clock), the instrument is
nearly blind and 263 probes stand in (22), the binary is the same bytes (106, 107), six of
seven changes are a `cmp` and one is a recording (26), and the source is the same lines
rearranged (27). The last is new, and it is the one a pipeline needs the day it starts
moving code rather than deleting it.
