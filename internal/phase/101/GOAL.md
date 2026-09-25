# Phase 101 — `main()` is demoted to `vim_main()`

`internal/phase/101/edit.go` and `internal/phase/101/check.go`, `stage 101`, `package host`. Five
lines, no libc symbol, and `GOALS.md` §II.4c's first step. What was

```c
    int
main
(int argc, char **argv)
{
    ...
    return vim_main2();
}
```

becomes `static int vim_main(int argc, char **argv)` with the **same body, byte for
byte**, and a six-line launcher is appended below it:

```c
    int
main(int argc, char **argv)
{
    return vim_main(argc, argv);
}
```

The editor runs one call frame deeper and does exactly what it did. Nothing else moves;
this is deliberately the only thing the phase does.

## Both stay in `whim-vim.c`, and that is the point rather than a compromise

Two tools hard-code today's invariant: `tools/phasecheck.sh`'s `grep -v '^main$'` and
`tools/funcreach.py`'s `{'main'}` root. Splitting the launcher into a second translation
unit is what breaks both, and the cost was measured while `exit` was being reviewed —
**one appended line to `tools/phasecheck.sh` moves 118 implementation keys**: 12 whim
stages, 82 whim edits, 12 Part II units, 12 Part II edits, and no slim key. So every demotion
that *can* be done inside one file is done inside one file, and the split happens once,
late, when there is nothing left to do before it.

**`vim_main` is `static` for the same reason.** Nothing outside this file calls it, and
a non-static one would be the first external symbol any Part II phase has ever added.
`nm --extern-only --defined-only` on the object still prints exactly `main`.

## The name was checked for a collision rather than assumed

`vim_main2()` already exists — it is upstream's, the second half of the old `main()`
split at the point where the screen is up — and `vim_main` had **zero** mentions as a
whole word. C has no prefix collision, but a reader greps, so the edit and the check pin
all three words separately: `main` 1 (the launcher's head, and the only bare `main` in
80,000 lines), `vim_main` 2 (its definition and the one call — a third would be a
prototype, and a function defined above its only call needs none), `vim_main2` 2
(untouched). `\b` does not match inside `main_loop`, `main_errors` or `vim_main2`, and a
substring grep does.

## A fossil went before this phase, and it turns out not to have been cosmetic

`main()`'s head used to be spelled over **three** lines because upstream had an `#ifdef`
between the name and the argument list, giving MS-Windows a different signature; slim's
phase 5 took the conditional and left the line break. **Phase 0's canonical text writes
one shape per construct**, so the break is gone before this phase is handed anything:
the head arrives in the two-line shape every other function here has, `vim_main` keeps
it and the new `main` gets it too.

What the fossil cost is now simply absent. `tools/funcreach.py`'s definition finder
never matched a three-line head, so `main` was never one of the definitions it counted —
its `{'main'}` root was a name added by hand to a set that did not contain it. It
matches every head in the canonical text, so this phase adds **two** definitions to what
it counts and one of them is `main` itself, seen for the first time. All are reachable.

## The evidence is every way the editor can end

The binary is **not** byte-identical and is not asserted to be: at `-O0` an extra call
frame is real code. Measured, both built `SOURCE_DATE_EPOCH=0` with the boundary's own
flags: **805,544 bytes either side — the same size, different bytes**, the frame
absorbed by alignment padding. So phase 99's tier-1 argument is not available here and
something else has to stand in its place.

What stands in its place is the six routes `GOALS.md` II.3b maps that a probe can reach
from outside, run on **both** binaries:

| | how it starts | path | status |
| --- | --- | --- | --- |
| `quit` | `+q!` | `ex_quit` → `getout(0)` | **0** |
| `cquit3` | `+cq 3` | `ex_cquit` → `getout(3)` | **3** |
| `eof` | stdin at `/dev/null` | `read_error_exit` → `preserve_exit` → `getout(1)` | **1** |
| `badopt` | `-Z` | `mainerr` → `mch_exit(1)` | **1** |
| `sigterm` | SIGTERM | `deathtrap` → `preserve_exit` → `getout(1)` | **1** |
| `sighup` | SIGHUP | the same | **1** |

`return vim_main(argc, argv);` puts a value in the program's path that was not there
before, and that table is what says it arrives. **The binary the phase was handed is
required to give the same six**, so an agreement cannot be two wrong answers agreeing.

**And the table is proven able to fail.** The output is built a second time with
`mch_exit`'s `exit(r)` changed to `exit(r + 1)` — one character — and all six move:
1, 4, 2, 2, 2, 2. Six statuses that agree prove nothing unless a wrong one would have
been caught, which is phase 100's `SA_NODEFER` control in this phase's shape.

**`/dev/null` and not a pipe, and that is a measurement.** With stdin a pipe the harness
closes, the EOF row came back as a twenty-second timeout on four binaries out of five
and as a clean 1 on the fifth — a race in the *harness*, not in the editor. A file that
is already at end of file has no race in it, and the four non-signal rows are then
deterministic over repeated runs.

## The declared delta is nothing at all

`internal/phase/101/delta.md` gets a comment and no line, and this is a **sixth** kind of empty
declaration. The five before it each removed *something*: **9** code that could not run,
**12** code that can run and that the instrument cannot see, **13** a possibility, **16**
no code at all with the binary the same bytes, **14** and **15** code replaced by code
that computes the same answers. **This one adds a call frame and removes nothing**, so
there is nothing to declare and nothing for a recording to show. `tools/coredelta.sh
--phase 101` finds the corpus unmoved, as it must: 102 of 102 screen cases, 111 of 111
Ex-command rows, 30 of 30 command lines.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | (the canonical text's) | **+6** — the launcher, the head being the shape it already had |
| functions `funcreach` counts | 1,755 | **1,757** — one new, and `main` seen at last |
| type definitions | 907 | 907 |
| `nm -u`, as `phasecheck.sh` counts it | 33 | **33, identical as a `cmp`** |
| `nm -u` with the core's flags | 32 | **32** |
| external symbols | `main` | `main` |
| `#include` | 12 | 12 |
| binary | 805,544 | **805,544 — the same size, different bytes** |
| sweep | | **1 round, a complete no-op** |
| phase | | **22 s** |

## Its placement

`stage 101`, `package host` beside phase 100, with `uses host:101 seed:83 mechanical` and
`uses host:101 harness:86 mechanical` — the two every phase declaring "nothing moved"
owes.

**`need 101 swept` is not required.** Both anchors are exact text at a counted
occurrence — `main()`'s head, and the file's last two lines — and neither is text a
sweep has ever touched.

**`apart 100 101`, measured.** Phase 100's check requires the file to have lost **exactly
nine** lines and this phase adds six, so on a shared stage the one swept text 17's
check is handed is shorter by the difference rather than by nine, and the check refuses
with *"the file lost N lines, expected 9"*. It is `apart 99 100`'s shape in **one** direction only — phase 101's own check
compares against the text *its* edit was handed, which is 100's output either way, so it
passes on a 100-101 stage.

## What whim-vim is after eighteen phases

```
whim-vim.c        80,428 lines          from whim-vim.c's 86,614  (-6,186, 7.1%)
functions         1,757  (1,755 + vim_main, + main itself, now a shape funcreach sees)
type definitions  907
DWARF enumerators 1,181
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    108, 96 distinct globals  (orphanopts floor 80; 16 of margin)
#include          12, every one a system header; no #define, no conditional
libc symbols      32 with the core's flags, 33 as tools/symbols.sh counts
binary            805,544 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    20 records + stderr-moved, from whim-vim
```

**Nothing in the table moved but the line count and the function count**, which is what
a phase that renames one function and adds another is entitled to move. `exit` is still
`mch_exit`'s single call site; the next phase in this package is the one that takes it.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase's steps run in phase 102, in the group 100-102, whose phases share one purpose. There is no boundary q101 of its own any more; everything above still says what the steps do and why.
