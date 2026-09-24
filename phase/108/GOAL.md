# Phase 108 — the plain host calls

`phase/108/edit.go` and `phase/108/check.go`, `stage 108`, `package boundary`.
The core reached the host through two function pointers:

```c
    static void (*vim_host_exit)(int);                          phase 102's, one call site
    static void (*vim_host_message)(const char *, int, int);    phase 104's, eight
```

each a file-scope object installed through a parameter of `vim_main()` that `main()`
passed. This phase makes both of them ordinary calls to `static` functions declared
above and defined below. **Two objects, two parameters, two assignments and two
arguments go; two prototypes arrive**, and `vim_main(int argc, char **argv)` is exactly
the signature phase 101 wrote when it demoted `main()`. 80,178 → 80,173 lines.

## The indirection had one reason, and the design changed underneath it

`phase/102/edit.go` states it in as many words: *a pointer the launcher installs
through a parameter adds no external symbol, where a `musl_exit(int)` the host defines
would.* The invariant it was protecting is `nm --extern-only --defined-only` printing
exactly `main`, and under **two translation units** the sentence is true — the host's
definition of a function the core calls has external linkage by construction.

`GOALS.md` §II.4c is now one file with two parts and the first `#include` as the
boundary, so the host's definitions sit below the core in the **same** translation unit.
A `static` forward declaration above and a `static` definition below is all a direct call
needs, and the global the indirection existed to avoid does not appear at all. The
machinery outlived its argument by six phases, which is the ordinary way a design change
leaves debris.

## The control that matters is the one that would have passed unnoticed

This is the phase that makes the core **name** the host's two functions, so
`nm --extern-only --defined-only` is the assertion it could have broken, and both halves
of the `static` trap are built on the phase's own output:

```
    static off the two PROTOTYPES                gcc refuses --
                                                 "static declaration of 'host_exit'
                                                  follows non-static declaration"
    static off the prototypes AND the            the build is SILENT and the object
    definitions                                  defines host_exit, host_message, main
```

The second is the mistake nothing else here would have caught: it compiles, it links, it
runs, every recording matches, and the core has quietly acquired two external symbols.
The check runs it every time. A third control deletes the two prototype lines and
requires the file **not** to compile, which is what makes *load-bearing* a measurement
rather than a description.

Each prototype is built out of its definition's own lines rather than retyped, so the two
cannot disagree. Measured on the output: `host_exit` declared at line 3533, called at
55872, defined at 80137; `host_message` declared at 3534, called at eight sites from
37649 to 79847, defined at 80144 — declared above every use and defined below every one,
which is the shape that keeps the declaration serving when a later phase moves the
definitions further down. It did: phase 110 moved them, and these two lines did not change.

## The evidence is the recording, because the binary moves

788,488 bytes either side and **347,279 of them differing**. An indirect call through a
pointer loads the pointer and calls a register where a direct call is relative to a known
address, and removing two file-scope objects moves what follows them; at `-O0` that is
simply different code. So this phase cannot use tier 1, does not pretend to, and falls
back on what every phase before 106 used: **two full recordings, `diff -r` empty
across all 106 records** — the 102 screen cases, every Ex command typed at `:`, every
command line the parser may see, the four pty scenarios and the terminal table.

And it can fail, **once for each name the phase makes direct**, which is what keeps an
empty `diff -r` from being a harness that recorded nothing. `host_exit`'s own
`host_code = r;` changed to `r + 1` moves **105 of the 106** records — every one but the
terminal table, which records no exit status. `host_message`'s `write(err ? 2 : 1, …)`
with the two streams swapped moves **`ref-argv.txt` and nothing else**, which is phase
104's own finding read back: everything reaching that function is a message printed before
there is a screen.

## The two prototypes join phase 103's nine, and that is the point of doing it here

The core → host boundary is now **one block of eleven declarations** rather than nine in
a block and two wherever an object happened to sit. `tools/zhostonly.py` — phase 103's
structural check — is run rather than assumed: its vocabulary is libc's terminal, signal
and descriptor names, `host_exit` and `host_message` are not in it, and it passes
unchanged at 60 mentions of 43 words, all inside the host block, with the same seven
named exceptions.

## It answers phase 105's open question rather than leaving it

Phase 105's survey flagged that 20 of its 129 new call sites sat inside the functions a
**split** would have moved, so host code would be calling back into the core's `emsg()`
and reading the core's `IObuff` — a genuine boundary question with three unattractive
answers. Under one file there is no question, and the measurement is here rather than in
a note somebody has to find: the four functions that still hold a `va_list` —
`vim_snprintf`, `vim_vsnprintf`, `vim_vsnprintf_typval` and `skip_to_arg` — call
**twenty distinct core functions at forty-one sites** and read `IObuff` once, and not one
of those costs a declaration. **Everything above the cut is visible below it**; only the
core → host direction ever needs a name declared, which is why this phase costs two
prototypes and not four.

## The declared delta is nothing at all, and it is phase 104's kind

The code runs, the instrument sees it, and it does the same thing. Not a `cmp` — that
kind belongs to 99, 106 and 107 — and not a blindness either: the recording is shown able
to see both names change.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 78,116 | **78,112 (−4)** — re-measured on the canonical text; the rest of this table is not |
| `vim_host_exit` / `vim_host_message` | 3 / 10 | **0 / 0** |
| `host_exit` / `host_message` | 2 / 2 | **3 / 10** |
| `exit_fn` / `message_fn` | 2 / 2 | **0 / 0** |
| `vim_main`'s signature | four parameters | `(int argc, char **argv)` — phase 101's, back again |
| core → host prototypes | 9 | **11**, one block |
| functions / type definitions / DWARF enumerators | 1,756 / 905 / 1,177 | 1,756 / 905 / 1,177 |
| `nm -u` with the core's flags | 17 | **17, the same set**, a `comm` empty both ways |
| external symbols | `main` | **`main`**, with both halves of the trap built |
| `#include` | 11 | 11, on the first eleven lines |
| `options[]` / `cmdnames[]` / `nv_cmds[]` rows | 107 / 98 / 194 | 107 / 98 / 194 |
| binary | 788,488 | **788,488 bytes, 347,279 of them differing** |
| records that moved | | **0 of 106** |
| sweep | | takes nothing, one round, canon a no-op |
| phase | | **25 s** |

## Its placement

`stage 108`, `package boundary` with 106, 109 and 110 — it writes no C23 and removes no GNU
extension, so `dialect` would be wrong; what it changes is how the core names what is on
the other side of the line. Its `uses` are `boundary:108 seed:83 mechanical`;
`boundary:108 harness:86 mechanical`, its evidence being a recording and not a `cmp`;
`boundary:108 host:101 rationale`, the signature it gives back being the one phase 101
wrote; `boundary:108 host:102 mechanical`, for the pointer, the parameter, the assignment
and the reason they were a pointer — and for the launcher it must leave untouched, `exit`
staying out of `nm -u` because `host_exit()` still records a status and jumps; and
`boundary:108 host:103 mechanical` and `boundary:108 host:104 mechanical` for the block the
two prototypes join and the eight call sites and the control that reads back phase 104's
finding.

**`apart 107 108` is measured, and its first complaint is one nobody would predict.**
`tools/phaserun.sh 107-108` on q106 stops inside **phase 107's** check with *"a line
carrying a kept attribute is not the line it was, byte for byte: lines 847 852 3081
3092"* — phase 107 records the six `format`/`format_arg` lines it keeps **by line
number**, and this phase deletes the `vim_host_message` object with its blank line at
495, so everything below moves up by two. A check that pins a line number is a dependency
on every line above it, exactly as a check that quotes C is a dependency on spelling
(`apart 105 106`). One direction only, measured: phase 108's check passes on the tree that
stage leaves.

**`need 108 swept` is measured NOT to be required**: this edit applies unchanged to the
unswept text phase 107's edit leaves, giving the same 80,178 → 80,173 and the same line
numbers.

## What whim-vim is after twenty-five phases

```
whim-vim.c        80,173 lines          from whim-vim.c's 86,614  (-6,441, 7.4%)
functions         1,756
type definitions  905
DWARF enumerators 1,177
core -> host      11 prototypes in one block; no function pointer left between them
#include          11, on the first eleven lines; no #define, no conditional
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            788,488 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
```

**The product was not in this phase's branch.** `whim-vim.c` landed in a commit of its
own after the merge, as phase 109's did, where phases 107 and 110 carried theirs in the
branch. Nothing was lost — the file is q108's boundary either way, and `make whim-verify`
reproduces it — but a merge whose diff holds the programs and not the thing they produce
is easy to read as a phase that changed no source, and it is worth knowing that two of
these four look like that in `git log`.
