# Compacting the pipeline: a survey of its 170 phases

2026-09-25. This survey measures and changes nothing: the only file it adds is
this one. It was measured on commit `fd1eff2` ("edit: PureCond no longer takes a
one-argument call for a cast"), against that commit's sealed boundaries
`q000..q169` (a copy of `.cache/boundaries/`, `q169` equal to the committed
`src/whim-vim.c` byte for byte). Every "SAME" below means an experiment's output
was compared with the snapshot byte for byte; every time is wall-clock on the
64-core build machine while other work was running, so compare times inside one
table rather than across tables.

**The answer in one paragraph.** Of the 170 phases, **8 do nothing** (82, 83, 84,
86, 99, 116, 123, 163). They are `NoSource`, and each leaves its boundary equal
to the one before it byte for byte. They cost **0 s**, because the driver skips
them. Taking them out of the plan and keeping every number passes
`whim-build-check`. The time is in the finish that follows each of the other
161 phases: the sweep and the canonical print, which parse the text twice. That
finish costs **~3 s a phase in Part II and ~4 s in Part I**, 407 s of sweeps
alone out of a 1,100 s serial run. Merging phases saves that finish. As an
experiment, the fewest merged runs that give the product back with no sweep
between phases is **21**. That plan builds the committed `whim-vim.c` in **533 s
against 1,094 s** under the same load. It is not recommended. It hides two
*silent* dependencies (a merged run gives other bytes and does not refuse), and
on a moved upstream nothing would catch them. Its longest link, 54-71 at 111 s,
would make `whim-build-check` slower, not faster. A modest set of
**14 same-purpose merges** (23 phases fewer) also passes `whim-build-check` byte
for byte, and saves about 7 % of a serial build. That is the ceiling of what
compaction is worth while each phase stays a unit a reader can name. Renumbering
is not worth doing: the driver does not care about gaps, and 4,200 citations
do. §7 is the ranked plan.

## 0. What was measured, and how

- **Every boundary against the one before it.** `cmp` and `diff` of `q(N-1).c`
  and `qN.c` for N = 1..169: lines added and removed per phase.
- **Every phase alone.** For each N, `whim build --from N --to N --src
  q(N-1).c -v`, 8 at a time. The measurements are its wall time, the
  sweep's own milliseconds (the sweep prints them), the lines of its report
  (one per act) and whether the output is `qN` (every one is).
- **A throwaway harness**, `.tmp/mergex/` in the survey's worktree (not
  tracked). It links `internal/build` and every phase, and calls
  `build.Advance`, the same function `whim-build-check` runs per link, on
  phases it builds itself. It has four modes:
  - `none A B`: phases A..B as ONE phase, their steps concatenated with no
    sweep between, run on `q(A-1)` and compared with `qB`. This is a merge.
  - `sweep A B`: the same with an inner `sweep` step between the phases.
    `full` adds the canonical print as well (`cemit`), which is identical to
    today by construction.
  - `reorder LIST`: phases in a given order, each its own phase with its full
    finish, `+` joining phases merged without one. It starts from `q(min-1)`
    and is compared with `q(max)`.
  - `split N K`: phase N's steps `[:K]` and `[K:]` as two phases, compared with
    `qN`.
- **A greedy search** for the fewest merged runs. From each start it galloped
  forward and bisected to the longest run that `none` still reproduces, then
  started the next run there.
- **Two modified plans run through the real check.** In the worktree only,
  `internal/build/plan.go` was edited (then restored), `cmd/whim` built, and
  `whim build --check --jobs 24` run against the snapshots. The two plans:
  1. the 8 `NoSource` phases dropped;
  2. the same plus the 14 merges of §3d.

## 1. Where the time goes

| what | measured |
|---|---|
| phases that run (not `NoSource`) | 161 |
| sum of their solo wall times | 1,100 s (the serial `make whim-build` is 1,070 s) |
| sweeps run | 173 (161 finishing sweeps + 12 inner `sweep` steps), **407 s** |
| sweep, one run | 1.2 s on a 76k-line text, 1.9-2.5 s on a 110-170k one |
| canonical print, one run | 1.3 s (q128) to 2.0 s (q044) |
| parse alone (`whim parse`) | 0.7 s (q128) to 1.0 s (q044), paid by the sweep AND the print |
| the finish a merge saves, per phase (merged vs separate, same load) | 44-48: 4.3 s; 129-141: 3.0 s; 143-162: 2.9 s; 164-168: 2.8 s |
| the slowest single phases | 54: 44 s (dropoptions on 174 names); 110: 31 s (gcc fixpoint); 16: 25 s; 32: 21 s |
| mean phase, 0-81 / 129-162 | 8.9 s / 3.2 s |

Two consequences shape everything below:

- **A merge saves only the finish**, 3-4 s of each phase merged away. The steps
  still run. Merging 23 phases saves about 75 s of 1,094 s.
- **`whim-build-check` is bound by its slowest link, not by the number of
  links.** With `--jobs 32` it takes 72 s, and phase 54 alone is 44 s of that. A
  merge never shortens the longest link and can lengthen it: merging 54-55
  gives a 58 s link, 54-71 a 111 s one.

## 2. Empty phases (question 1)

Exactly the 8 `NoSource` phases leave their boundary byte-identical, and no
other phase does:

| phase | name | why it is empty | mentions of its number (files) |
|---|---|---|---|
| 82 | every comment | its comments went to the canonical print, its headers to 169 | 41 (11) |
| 83 | the core's compile line, and the baselines | the compile line became the line for all; the baselines went with the suite at `448e9a8` | 146 (25) |
| 84 | the stack protector goes | the same, for `-fno-stack-protector` | 14 (3) |
| 86 | the instrument becomes the screen | a harness change, never a source change | 67 (12) |
| 99 | the includes nothing names | its headers went to 169, its typedef to the sweep | 57 (16) |
| 116 | the terminal table is asked with `+set term=` | a harness change | 47 (9) |
| 123 | the instrument could not see the text layer | a harness change | 66 (13) |
| 163 | the product is in the one canonical spelling | every boundary is printed canonically now | 7 (4) |

- **Cost today: 0 s.** `Run` and `Advance` return the text unchanged for a
  `NoSource` phase, with no sweep, print or scratch directory. Removing them
  buys a plan 8 entries shorter, not time.
- **Removing them keeps the product, measured.** Plan 1 of §0 (161 links)
  passes `whim-build-check`: "every one reproduces the next", `whim-vim.c` byte
  for byte. Check pairs `Plan[i-1]` with `Plan[i]` and names snapshots by `N`,
  so the link 81 → 85 compares `q081` with `q085`. A sequential run across the
  gap also works: `whim build --from 80 --to 88` from `q079` gives `q088`.
- **What would still name them**:
  - their directories, which stay: each `GOAL.md` already opens as a record;
  - `internal/phase/boundaries.md` rows 082-084, 086, 099, 116, 123 and 163,
    which a new `measure` would simply not produce;
  - the count "170 phases" in `CLAUDE.md`, the `Makefile`'s help text and
    `README.md`;
  - `GOALS.md`'s two indexes;
  - `internal/phase/STAGES.md`, whose phase list is a record anyway.

  No Go reads any of their numbers.

## 3. Merges (question 2)

### 3a. Every adjacent pair, merged with no sweep between

The 160 adjacent pairs of phases that run (82-84, 86, 99, 116, 123 and 163
skipped) were each merged with `none`. **148 give the next boundary
byte for byte.** 12 do not:

| pair | how it fails | what it needs between |
|---|---|---|
| 41 \| 42 | `onebuffer`: `buf_hide` has 3 mentions, 41's dead callers hold one | a sweep (`sweep 41 42`: SAME) |
| **53 \| 54** | **silent: `'arabic'`'s row survives, one line more, no refusal** | a sweep (SAME), as `STAGES.md` recorded in e10/e11 |
| 80 \| 81 | none: a harness artefact (the merged phase must be numbered 80 for its `delta.md`); with that, SAME | nothing |
| 87 \| 88 | `noargv`: `read_cmd_fd` 14 mentions, not 13 | sweep AND canonical print (`sweep` alone still refuses) |
| 89 \| 90, 90 \| 91, 91 \| 92, 92 \| 93, 93 \| 94 | each anchor counts mentions its predecessor's dead code still holds (`usefilter`, `readfile`, `open_buffer`, `b_ffname`, `SHM_FILEINFO`) | a sweep (each SAME with one) |
| 97 \| 98 | `vendor`: the end of phase 97's block is spelled otherwise | sweep and canonical print |
| 103 \| 104 | `message`: the file does not end with 102's launcher as printed | sweep and canonical print |
| 125 \| 126 | `refblocks`: the memfile's shape | a sweep (SAME) |

The whole-phase finish is the sweep AND the print. Three pairs (87|88, 97|98,
103|104) need the print too: their anchors are written against canonical text.

### 3b. Pairs do not compose

A run that merges three phases can fail although both of its pairs pass: the
leftovers of several phases add up. The greedy search found these further cuts:

| run that fails | first refusal |
|---|---|
| 1-13 | 13 `nostat`: `buf_check_timestamp` twice in `enter_buffer`, expected once |
| 13-17 | 17 `nofenc`: "the empty test Phase 15 left", 0 matches |
| 54-72 | 72 `onewin`: `ONE_WINDOW` expanded 9 times, expected 3 |
| 72-75 | 75 `noautocmd`: 54 bare dispatches, expected 49 |
| 75-87 | 87 `noexmode`: `exmode_active` 50 mentions, expected 49 |
| 104-110 | 110 `boundary`: `vim_host_exit` declared 0 times |
| 110-125 | 125 `swapres`: `ml_open()`'s preamble does not mention `b0_pid` |
| 126-128 | 128 `node`: the page header is not the four members it folds |
| **128-166** | **silent: two `int`s that should be `bool` (`reg_line_lbr`, a `bt_regexec_nl` parameter)** |

The last one was narrowed by bisection. `N-166` is SAME for every N from 137
up, and DIFF for 136 and below. Phase 136 calls the regexp engine directly, and
its unswept output still holds the `regengine_T` table with `bt_regexec_nl`'s
address in it. Phase 166's computed set leaves a function whose address is
taken with its signature, so it types two answers `int`. Nothing refuses. It is
the second silent dependency found, after 53|54, and neither `STAGES.md` nor any
`GOAL.md` records it: phase 166 was written after the stages were retired, and it
has only ever been run on swept text.

### 3c. The fewest runs: 21, measured

The greedy partition, every run merged with no sweep between its phases:

```
1-12  13-16  17-41  42-53  54-71  72-74  75-85  87  88-89  90  91  92  93
94-97  98-103  104-109  110-124  125  126-127  128-165  166-169
```

Run as one plan from `q000`, **it gives `q169`, the committed `whim-vim.c`,
byte for byte, in 533 s**. The 161 phases run one at a time in the same way,
at the same time and on the same machine, took 1,094 s. The longest runs are
54-71 (111 s), 17-41 (93 s), 110-124 (43 s) and 42-53 (41 s).

It is not a plan to adopt:

1. **It is fitted to this input.** Every run was found by trying. The two silent
   cases show that a merged computed set shrinks without a word. On a moved
   `slim-vim.c`, `whim-build` has no snapshot to compare with, and a sweep after
   every phase is what keeps each phase's premise true by construction.
2. **The check gets slower.** Its longest link would be 111 s against today's
   44 s.
3. **Finding a failure gets coarser.** A link that breaks names 18 phases
   (54-71) instead of one, and `whim build --to N` cannot stop inside a run.
4. **A phase stops being the unit a reader can name.** One `GOAL.md` would
   stand for 25 phases (17-41).

### 3d. Same-purpose merges: 14 groups, 23 phases, measured

These were chosen for a common purpose first, then kept only if `none`
reproduced the group's last boundary. They were then run TOGETHER, as one
139-phase plan: **it passes `whim-build-check` (138 links, "every one
reproduces the next", `whim-vim.c` byte for byte)**. Its longest merged link
is 39-40 at about 20 s, so the check's bound stays phase 54.

| group | what they share | merged time | phases saved |
|---|---|---|---|
| 39-40 | one window, then no window sizes | 19.5 s | 1 |
| 44-48 | Ex commands retired one by one (`:!`/sort, `:drop`, `:wall`..., `:startinsert`..., `:noswapfile`) | 5.8 s | 4 |
| 51-53 | UTF-8: keep bytes, UTF-8 only, no conversion | 18.1 s | 2 |
| 72-73 | one window/tabpage structurally, then one frame | 8.4 s | 1 |
| 100-102 | the deadly ladder, `vim_main`, the core cannot stop the process | 6.6 s | 2 |
| 105-107 | C23 spelling: the variadic collapse, `nullptr`/`usize`, the attributes | 8.6 s (105-108) | 2 |
| 117-119 | the core calls nothing but the host | 7.2 s | 2 |
| 121-122 | the terminal names, then `-T` | 8.8 s | 1 |
| 143-145 | `regatom()`, `edit()`, `check_termcode()` lose their `goto` | 3.5 s | 2 |
| 148-149 | allocation cannot fail, and its branches fold | 7.1 s | 1 |
| 151-152 | the option table's defaults, then its variables, typed | 5.5 s | 1 |
| 153-154 | `free_one_termoption()`: the cast, then the NULL write | 3.5 s | 1 |
| 164-165 | what the Go's linters found dead | 4.4 s | 1 |
| 166-168 | `bool`, key names, `goto` as `return` | 14.4 s | 2 |

These also reproduce but were left out of the plan: 20-22, 29-31, 33-37 and
55-62. They have no common purpose, and 55-62 would be a 56 s link, slower
than phase 54. Also left out: 68-71, 85-87, 88-89, 111-115, 129-131, 132-134,
135-136 and 155-162. They reproduce too, but most of 129-162 map one to one
onto a finding in `internal/gen/FINDINGS.md`.

**Saving:** 23 finishes at 3-4 s, about 75 s. Measured: the 139-phase plan run
in order from `q000` gives `q169` byte for byte in **1,013 s**, against 1,094 s for
the 161 phases (a later run, so on a somewhat different load), **7 %**. The
check does not change. **Cost:** 14 `GOAL.md` merges (or 23 `GOAL.md`s that
say "runs in phase B"), 23 `internal/phase/boundaries.md` rows, and
`GOALS.md`'s indexes.

## 4. Splits (question 3)

The largest phases, by the size of their program and of their report:

| phase | Go lines | report lines | shape |
|---|---|---|---|
| 128 fold the node types | 1,029 | - | one `Edit` |
| 125 the swap file's residue, and what no sweep could find | 1,021 | - | one `Edit`, two themes in its name |
| 127 de-page the leaf | 891 | - | one `Edit` |
| 122 `-T {term}` goes | 823 | - | one `Edit` |
| 110 the move: the first `#include` becomes the boundary | 759 | 15 | one `Edit`, a gcc fixpoint of 6 rounds, 31 s |
| 80 the Ex command table | 717 | 58 (54 acts) | one step; about 14 acts cut the table and the lookup, about 40 fold what no row can raise any more |
| 64, 39 | (steps) | 88 each | 7 and 6 steps |

Phases whose names already join several purposes: 3, 6, 18, 60, 65, 67, 78 and 125.

**Splitting a multi-step phase into two phases at a step boundary** was tried
18 times. 15 give the phase's boundary byte for byte: 3@1, 3@2, 6@2, 9@1,
10@1, 11@1, 18@2, 18@3, 21@1, 25@2, 32@4, 39@2, 56@3, 60@3 and 62@3. Three
refuse: 35@1 (`nosession`: `has_autocmd` not at file scope), 95@1 and 95@6
(`b_did_warn` 0 mentions, expected 1). The later half counts text that the
sweep between them removes. Each such split costs one finish, 3-4 s.

**The inner sweeps are all forced.** The pattern `dropoptions --local ...;
sweep; droplocal b_p_...` appears in 12 phases: 16, 24, 28, 32, 42, 49, 50, 57,
58, 60, 62 and 64. Taking the inner `sweep` out of any one of them makes it
refuse, 12 of 12 (`droplocal: b_p_X still has N mentions ... those are readers`;
in 24, `dropoptions: 'mouse' still has readers`).

**What a split should be, if one is wanted: steps within the phase, not a new
phase.** The plan already splits four programs this way: `whim56` then
`whim56kp`, `whim60` then `whim60ep`, `whim62` then `whim62bl`, `whim95` then
`whim95rows`. Each is a second registered edit inside the same phase, with no
finish between. Cutting `whim80` into the table and the folds, or `whim125`
into its two themes, at a point where the first half hands the second exactly
the text it hands itself today, is byte-identical by construction and costs
0 s. Its value is a report and a program a reader can take in halves. It is low
priority: the `GOAL.md`s already give these phases sections.

## 5. Reordering (question 4)

Measured moves, each against the snapshot after the moved group:

| move | result |
|---|---|
| 22 (`nochdir`) right after 7 (which retires `:cd` and its kin), then 8-21 | SAME |
| 115 (the clock crosses the boundary) right after 111 (the scalar clock) | SAME |
| 142 (no build date) before 129 | SAME |
| 169 (the headers nothing needs) before 164-168 | SAME |
| 44-48 as one phase | SAME |
| 12, 15, 17 (the encoding phases) together, then 13, 14, 16 | refuses at 17: `b_p_fenc` still has 2 readers |
| 12+15+17 merged | refuses at 15: `'fileencodings'` reached by name |
| 43 (no `-c`, `--cmd` ...) right after 18 (the command line decides nothing) | refuses: 43's anchor, `-c` in the option switch, matches 0 times until 19-42 have run |
| 156 (the regex size node) right after 150 (the regexp stack) | refuses: 26 uses, expected 14 |

- **Part I's order is mostly forced.** Its anchors count what their
  predecessors left, as the failures above show.
- **Part II's later phases commute more freely** (142, 169, 111+115), but a
  free move gains nothing: no time, and the numbers would stop being the order.
- **Phase 169 is last by choice, not by force.** It holds anywhere after the
  last phase that removes a header's use; before 164 was measured.

**Cleanups done piecemeal, and what is left of them:**

- **The headers**: already one step, phase 169. 82 and 99 are empty for that reason.
- **The canonical print**: already every boundary's. 163 is empty for that reason.
- **`cmdidxs --check`** runs in 15 phases (58, 63, 66, 68-79). It is an
  assertion that changes no byte and costs 0.02 s. Its value is naming the
  phase that breaks the derived index. Phase 80 deletes the index, and nothing
  checks it after. Each call carries a stray argument, `">/dev/null"`, a relic
  of the shell program it was ported from; `cmdIdxs` ignores it.
- **`query-empty whim2`** (phase 2) is likewise assertion-only.
- **Phase 64 calls `droplocal` three times** with one field each. It takes a
  list and loops field by field (`steps.dropLocal`), so one call with three
  fields is the same bytes and the same report.
- **`@state`** is handed to 8 edits (110, 118, 119, 124-128) and 80 gets
  `@state/words`. They write files there for checks that no longer exist, and
  the plan's own comment says no edit reads them back. Removing it touches
  those 9 programs, not the plan alone.

## 6. Renumbering (question 5)

- **The driver does not need dense numbers** (measured, §2): check and run pair
  `Plan` entries by position and name snapshots by `N`.
- **The numbers are cited everywhere**:
  - 2,824 phase-number mentions in the tracked Markdown, 894 of them in
    `GOALS.md` and 516 in `STAGES.md`;
  - 1,403 in hand-written Go, including 118 in `plan.go`, 111 in
    `cmd/whim/phases.go`, and the registry names `whimNN` and `pNNN`;
  - 31 in `internal/gen/FINDINGS.md`;
  - 472 in the messages of the repository's 285 commits, which cannot be
    edited.
- **Renumbering buys contiguous numbers and nothing else.** No run gets faster.
  In exchange, every history-based citation would be wrong or doubly mapped
  ("phase 88, formerly 95").
- **Keep the numbers, keep the gaps.** A merged phase should keep the LAST
  number of its group. Its boundary is then `qB`, the same bytes as today, so
  the existing snapshots and the `boundaries.md` rows for the numbers that
  survive stay valid. That is how §3d was checked without a rebuild.

**A structural alternative to merging**, not built: a per-phase flag, say
`Join`. It would mean: run this phase's steps on the previous phase's text
without its finish. That is §3c/§3d's merge while every `GOAL.md`, number and
report line stays where it is. What it gives up is the same as a merge's: the
joined boundary is not swept or canonical, so `qN` would hold unfinished text
(`boundaries.md`'s "every row parses" would need qualifying). The silent cases
of §3b apply to it unchanged. It is worth having only if the build time comes
to matter.

## 7. Recommended plan, ranked by value against risk

Each step keeps the product: `make whim-build-check` must give `whim-vim.c`
byte for byte, and `make whim-test` must pass, because no step moves a byte.

1. **Plan hygiene (zero risk, small value).** Drop `">/dev/null"` from the 15
   `cmdidxs` steps, and fold phase 64's three `droplocal` steps into one. Both
   are byte-identical by construction; check them with `whim-build-check`.
2. **Take the 8 `NoSource` phases out of `Plan`, keeping every number (zero
   risk, clarity).** Measured passing (§2). Their directories and `GOAL.md`s
   stay as records. `CLAUDE.md`, the `Makefile`'s help text and `README.md`
   then say "170 phases, 162 of which run" (or the plan's length). Update
   `GOALS.md`'s indexes and `boundaries.md` in the same commit.
3. **If build time matters, make the finish cheaper rather than rarer (medium
   value, code risk contained in `crefactor`).**
   - The sweep and the print parse the same text one after the other. One
     parse shared between them would save about 0.7-1.0 s a phase, 110-160 s a
     build, more than all of §3d.
   - `cut.DropOptions` scans the whole file with fresh regexps several times
     per option name. Phase 54 drops 174 names, and 44 s of the check's 72 s
     is that one link.

   Neither touches the plan, and both are held to the same byte check.
4. **Optionally, the same-purpose merges of §3d (low risk, about 7 % of a
   serial build).** Take them one group at a time, each merged group numbered
   after its last phase, its `GOAL.md`s merged or pointing at it. Each group is
   measured SAME; together they are measured passing. Take the retire series
   44-48, the goto series 143-145 and the two `free_one_termoption()` phases
   153-154 first: those were one idea split for history's sake. Leave the rest
   unless a document is being rewritten anyway.
5. **Split programs into steps, not phases (zero risk, readability).** If 80 or
   125 is ever reworked, cut it into two registered edits inside the phase, as
   56, 60, 62 and 95 already are (§4).

## 8. What is not worth doing

- **The 21-run minimum** (§3c). Half the serial build, but it rests on
  measurement against this input alone. It has two silent dependencies that
  nothing would catch on the next input. The check would slow to about 111 s,
  a failure would name 18 phases, and 25 `GOAL.md`s would share one boundary.
- **Renumbering** (§6). 4,200 citations, 472 of them in unchangeable history,
  for no run-time gain.
- **Splitting phases into more phases** (§4). 15 of 18 splits work, but each
  costs a finish and a boundary. It buys nothing a split into steps does not.
- **Removing the inner sweeps** (§4). All 12 are forced.
- **Reordering for tidiness** (§5). Where a move is free it gains nothing.
  Where it would group a theme (the encodings 12/15/17, the command line
  18/43), it refuses.
- **Dropping the `cmdidxs --check` assertions.** They cost 0.3 s in total and
  name the phase that breaks the index on a moved upstream.

## Side findings (not acted on)

- **Two silent merge dependencies**, not recorded anywhere. **53 → 54** is
  known (`STAGES.md`, e10/e11). **136 → 166** is new: phase 166's `bool` set
  depends on phase 136's table having been swept away. Today's plan satisfies
  both by construction. They matter only to someone who merges phases or adds
  a `Join`.
- **`CLAUDE.md` says "84,111 lines at q82".** The snapshot and
  `internal/phase/boundaries.md` both say **84,025**, the same as q81, since
  phase 82 is empty. The sentence predates a printer change and is stale.
- **The check's floor is phase 54.** Its 44 s is dropoptions' per-name regexp
  compilation and whole-file scans over a 3 MB text, not the sweep. Phase 110's
  31 s is its own gcc fixpoint. Together they are why `--jobs 32` cannot go
  below about 45 s.
