# Phase 122 — `-T {term}` goes, and the command line is `+{command}`

`phase/122/edit.go` and `phase/122/check.go`, `stage 122`, `package terminal`. Zero
phase 88 left argv as exactly two options: `+{command}`, which is how a **host** tells the
editor what to do, and `-T {term}`, which is how a **shell** told it what terminal it was
attached to. A core is told that by its host or not at all, and `-T` has had a replacement
inside the editor since before this pipeline began — `+set term=` reaches
`did_set_term()` and does everything `-T` did, which is why phase 116 rebuilt the terminal
harness on it rather than on `-T`. So the option goes, and after this phase
`command_line_scan()` is one `if (argv[0][0] == '+')` and one `else` answering
`mainerr(ME_UNKNOWN_OPTION)` — which is what every other word already answered.

## Six cuts, and five of them are dead code no sweep can see

gcc has no warning for a variable that is only ever FALSE, for a switch that has lost its
cases, for a statement after a `return`, or for a struct field whose name another struct
also uses — so all five go in the **edit**, which is `CLAUDE.md`'s rule and phase 121's
shape.

The option letters are **read out of** `command_line_scan()`'s first `switch (c)` rather
than written down. With no case left to set it, `want_argument` is FALSE for ever, so its
block goes and takes the argument switch, `parmp->term = argv[0]`, `ME_GARBAGE` and
`mainerr_arg_missing()` with it. The letter switch is then one `default:` and **is** its
body, which is a rewrite and not a change only because `mainerr()` does not return — read
off `mainerr()`, not assumed. The `-` arm and the last arm are then the same statement,
compared as **text** and collapsed into one else.

**The two `main_errors[]` rows go with their enumerators**, because the table is indexed
by them. `deadenums.py` would take the enumerator and leave the row, and the rows are
positional — `CLAUDE.md`'s `deadfields` lesson in a table — so the edit takes both,
renumbers `ME_EXTRA_CMD` 3 → 1, and deletes `mainerr_arg_missing()`, which cannot be left
for the sweep because it is `ME_ARG_MISSING`'s only other mention. Which enumerators die is
**computed** as those whose only remaining mention is their own `enum` line, and the check
is **DWARF and not the build**, in phase 88's shape: 1,189 enumerator values in, **1,187**
out, the two gone, `ME_EXTRA_CMD` lower by exactly the number of rows that went, and 1,186
unmoved.

**`mparm_T.term` goes in the edit for a reason phase 121 did not have.**
`tools/deadfields.py` matches by **name**, and this file holds 32 mentions of another
struct's `.term` member — `attr_entry`'s `ae_u.term` — so that tool can never see this one
dead. The edit **computes the partition**, every `.term` left belonging to `ae_u`, rather
than asserting it. `termcapinit()` then takes no name at all, and the compiled default it
substituted when given none, read out of the function, becomes its initialiser — which is
`phase/096/edit.go`'s `ui_write(console)` again.

## And then `set_termname()`'s no-screen arm cannot run, which is what phase 121 predicted

`set_termname()` has two call sites, partitioned by the edit: `termcapinit()`'s, before
there is a screen, which can no longer fail because the compiled default **is** a row of
`builtin_terminals[]`; and `did_set_term()`'s, at run time, where `starting` is
`NO_BUFFERS` or 0 — the edit reads **every assignment to `starting`** and requires none to
be `NO_SCREEN`. So the test folds always, the arm returns FAIL, and the three statements
after it — the fallback phase 121 repaired, `report_default_term()` and the option write
that recorded it — are unreachable and go, with `report_default_term()` falling to the
sweep as the phase's only sweep find.

**Measured, and the instrument is phase 121's**: with the same marker in the same place,
reached through `host_message()`, the input enters the fallback in exactly **2 of its 30
command rows** — `-T xterm` and `-T no-such-term-9x`, the only way in — and a control built
from this phase's **output** with the whole arm restored enters it in **none**, while
recording byte for byte what the output records.

## Phase 121's prediction was half right, and the other half is stated rather than quietly dropped

`report_term_error()` does **not** fall to the sweep. It is called **before** the
`starting != NO_SCREEN` test, so the run-time refusal still prints it — measured, `:set
term=vt320` on a pty prints it and then E522 on both binaries. What it said was `' not
known, defaulting to 'xterm-256color'`, and with no fallback left **that is a promise
nothing keeps**, so the clause naming it is cut in the same step, for phase 121's own
reason: nothing in the build checks that a message tells the truth. The name is read out
of the assignment the edit deletes.

## What rides along is `requested`, and not the 256-colour test

`set_termname()` kept the name it was **given** for one test, `musl_strstr(requested,
"256color")`, because the unknown-terminal path reassigned `term`; that path is what this
phase removes, so the only rewrite of `term` left is the `term += 8` that strips a
`builtin_` prefix, and a strstr cannot match inside that prefix because the needle begins
with a character the prefix does not contain — which the edit **checks** rather than
asserts. **The test itself does not fold and this phase declines to pretend it does**: the
two surviving terminal names disagree on it and `:set term=` still names either at run
time. Measured in both directions and kept in the check — with the name test forced TRUE
exactly **1 of the 19** terminal rows moves (`debug` gains `t_Co=256`), and with it forced
FALSE **18** do.

## The declared delta is four of thirty command lines, and the set is computed

A row must move **if and only if** one of its words is an option spelling a letter the
input's switch accepted and the output's does not:

```
  -T xterm             started and drew 2,117 bytes      -> exit 1, Unknown option
  -T no-such-term-9x   started and drew 2,117 bytes      -> exit 1, Unknown option
  -T                   Argument missing after: "-T"      -> exit 1, Unknown option
  -Txterm              Garbage after option argument     -> exit 1, Unknown option
```

The last two are the interesting half: **they were already errors and they moved anyway**,
because the two messages they gave were `ME_ARG_MISSING` and `ME_GARBAGE`, enumerators
whose last use was the `-T` argument block. A row that was already `Unknown option
argument` — every one of phase 88's leftovers — must **not** move, and the other 26 do not.
The 102 screen cases, the 98 Ex-command rows, the four pty scenarios and the 19 terminal
rows are the input's byte for byte: `term-moved` is phase 121's and cumulative, and this
phase's recording of the table is the input's.

## And the phase decomposes, which is also the instrument shown able to fail

The **input** with only the `case` arm deleted — `want_argument`, the argument switch,
both `ME_*` rows, the field, the unreachable fallback and `requested` all left in place —
records byte for byte what this phase's output records, and differs from the input. **So
the option letter is the whole of what moves behaviour**, and the other five cuts are
invisible to every part of a recording.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 79,668 | **79,589 (−79)** |
| the command line | `+{command}` and `-T {term}` | **`+{command}`**, every other word `ME_UNKNOWN_OPTION` |
| `main_errors[]` rows / DWARF enumerators | | two rows and two enumerators go; 1,189 → **1,187**, 1,186 unmoved |
| the cut at the first `#include` | 77,758 | **77,679**, eleven directives with none above them |
| the boundary | 18 names | **18**, unchanged |
| `nm -u` | 17 | **17**, a `comm` empty both ways |
| external symbols | `main` | `main` |
| binary | 781,096 | **781,064** |
| records that moved | | **4 of 30 command lines**, and nothing else |

## Its placement

`stage 122`, `package terminal`, and `streams` had a real claim on it that the manifest
answers: the phase removes the half of the command line phase 88 kept, in the same parser —
but of the six cuts, one is that parser arm and **the other five are all terminal code**.
The sentence the package makes reads straight on: phase 85 removed the **question** the core
asked about the terminal, 38 removed the **vocabulary** it could describe one with, and 39
removes the **telling** — after it nothing outside the process can say what terminal this
is, and `+set term=` inside the editor is the only way. Four `uses`: `seed:83`;
`harness:86`, which put an argv record in a zero recording at all; `streams:88`, whose
leftover this phase's whole subject is, and whose renumbered `main_errors[]` table it
checks against DWARF in that phase's own shape; and `host:104`, whose `host_message()` is
the instrument again.

**`apart 121 122`, one direction, both halves measured in the same run.** Phase 121's check
builds its first control by finding the fallback in `set_termname()`, and phase 122 deletes
the fallback outright, so `tools/phaserun.sh 121-122` on q120 stops in phase 121's check
at its **first act** with *set_termname() names 0 of the surviving rows and this check
needs one — the fallback*. **That is a check depending on the code it repaired still being
there**, which is the sharpest form of `apart 105 106`'s lesson: phase 121's repair is phase
122's dead code. The other direction was run rather than reasoned — phase 122's check passes
on that stage's tree, which is byte-identical to the sequential one. No `need 122`, measured
in the same run on phase 121's unswept output.

And `phase/116/check.go`, the one phase program that reads other boundaries, was run in the
repository root with this phase's tar present: *all 34 recorded boundary binaries up to q116
record the SAME table*, because its scan has been bounded by its own number since the fix
phase 121 asked for, and this phase changes no terminal row at all.

## What whim-vim is after phase 122

```
whim-vim.c        79,589 lines          from whim-vim.c's 86,614  (-7,025, 8.1%)
                  77,678 above the boundary, 1,911 below it
functions         1,757
type definitions  906
DWARF enumerators 1,187
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
built-in terminals 2 of whim's 10: xterm-256color and debug
#include          11, at line 77,680, and NOT ONE DIRECTIVE above them
core -> host      18 names: vim_snprintf, host_exit, host_message, host_time,
                  host_alloc, host_free, host_write, host_raise, ten musl_*
libc prototypes   0 -- the core names no libc function at all
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            781,064 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    term-moved at 38 and four command lines at 39, the first zero has
                  declared since phase 94 -- with 2 stderr-moved and the records of
                  87 to 94 before them
make editor.c     77,678 lines: 0 directives, 0 errors, 18 warnings, all of them
                  `used but never defined` and all of them the interface
```

**Seventeen phases in a row had declared nothing — 104 through 120 — and phase 121 ended the
run.** What stood in for a recording through those seventeen is a taxonomy and not a
tally, and this document **names** the kinds rather than counting them, because an ordinal
written into a phase section is frozen on the day it is written while the list keeps
growing: phase 101's section says *a sixth kind* over one merge of the list and phase 109's
says *a sixth kind* over another, and both were true when written. **`CLAUDE.md` carries
the one numbered list**, keyed to the order the kinds first appear in `phase/122/delta.md`,
and where an ordinal here and an ordinal there disagree, that one is the current one. The
kinds, by name:

* **code that could not run** (92, 100) — an instrumented pair reaching it 0 times;
* **code that runs and the instrument cannot see** (85, 95, 103, 105, 111, 113, 119) — the phase
  owes probes of its own;
* **a possibility removed** (13) — nothing had ever opened those `FILE *` in any build;
* **the binary is the same bytes** (99, 106, 107, 120) — tier 1, and the strongest, with a
  control that moves it to keep a `cmp` from being two numbers agreeing;
* **replacement code that computes the same answers** (97, 98) — the weakest, and their own
  checks argue it;
* **the code runs and the instrument sees it do the same thing** (101, 102, 104, 108, 115, 117,
  118) — strong exactly when what moved is on the path of everything;
* **part `cmp` and part recording, separated rather than averaged** (26);
* **the source is the same lines rearranged** (27) — a multiset equality, tier 1 one level
  up;
* **the behaviour really moved and the corpus cannot reach it** (29);
* **an accounting** (31) — 42 bytes of `.text`, for a phase that frees no symbol because
  there was none to free;
* **no source changed at all** (86, 116) — what has to be argued is that the **comparison**
  moved safely.

**Fifteen of the seventeen libc symbols are called from the host and from nowhere else**,
and since phase 119 **so are the other two**: `getpid` and `kill` are `host_raise()`'s and
`musl_suspend()`'s, and the core's whole vocabulary of the seventeen is three English
words inside string literals — the two `NGETTEXT` strings in `op_shift()` that say *time*,
and `E222`'s *"already read from"*, measured on this file.
