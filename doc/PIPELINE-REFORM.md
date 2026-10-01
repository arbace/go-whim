# Reforming the pipeline: drops first, the C last

2026-09-30. A survey for review. It changes nothing but adds this file. It
was measured at `b63f0df`: 154 plan entries numbered 0-184, 77,634 lines, and
a full build of 1,005 s. The snapshots it used are the sealed boundaries in
`.cache/boundaries/` (q000..q184) that build left.

## The request, and the bar

The request was to reorder the pipeline so that every vim-specific feature
removal comes first and every generic C refactoring last:

- **Drop packages.** The scattered drops merge into a few large packages.
  Examples are all command-line arguments but `+` in one cut, all the Ex
  commands that go in one cut, and all the options that go in one cut. The
  dead weight each package leaves then falls out.
- **Order.** After the last vim-specific phase comes only generic C.
- **Priorities.** First separate the vim-specific from the generic. Then
  fewer phases, then a simpler and faster pipeline. A bonus is phases that can
  run in parallel.
- **The bar.** It was agreed on 2026-09-30. `slim-vim.c` goes in, and **today's
  `whim-vim.c` must come out byte for byte**. The old phase records are
  archived, and new ones are written for the new pipeline.

## The answer in one paragraph

The reform is possible and worth doing. The price is a rewrite of most
Part I programs, not a reshuffle of the plan.

- **The mix today.** Of the 154 plan entries, about 90 are vim-specific
  drops and about 45 vim-specific rewires or feature changes. About 22 are
  generic C.
  - They are interleaved to the end: generic steps sit at 106, 107, 120, 132,
    134 and 149, among the vim phases, and vim-specific ones at 174 and
    177-180, among the generic.
  - The three families you named are scattered. **Command-line arguments are
    cut in 13 phases** between 3 and 122, **Ex commands in 34**, and **options
    in 35**.
- **What the order depends on.** It is forced by *anchors*: each program
  asserts the text and the counts its predecessors left. It is not forced by
  what the code needs.
  - **Measured:** 53 of the 153 programs run unchanged on the seed q000, with
    no predecessor at all (§3).
  - The charter split made much work *superseded*. Part I kept an editor with
    files, so phases 50, 51, 53, 61, 62, 69, 70 and 74 did careful surgery in
    `readfile()`, `buf_write()` and `do_ecmd()`. Part II then deleted those
    functions whole, at 89-93.
- **The proposed pipeline** is three blocks (§5).
  - **D, drops.** About 12 packages, each an interface or a feature family cut
    once on the seed text, followed by a *fall-out* closure: the sweep plus
    generic constant folding.
  - **R, rewires.** About 20 vim-specific phases that keep capability: the
    host boundary, the memline tree, the regex engine, the option types, and
    the feature changes.
  - **G, generic.** About 12 steps from `crefactor/xform`, each run once.
  - **In all** that is about 45 phases for 154, and an estimated **400-500 s**
    for a full build against 1,005 s.
- **The risk is the byte-identity bar.** Hand-written folds and a generic
  fall-out must reach the same final text. So the migration (§7) moves one
  package at a time, each proven against the committed product before the
  next.

## 1. What was measured, and how

- **The classification.** Every plan entry and record, 0-184, was read and
  classified:
  - by its `GOAL.md` and its program (the `internal/cut/*.go` cutters for
    most of Part I, `internal/phase/NNN/edit.go` otherwise);
  - by its steps (`internal/build/plan.go`);
  - by the anchors each program asserts;
  - by greps of the snapshots, to see which earlier phase changes an asserted
    count.
  Four readers did this in parallel over the ranges 0-45, 46-95, 96-140 and
  141-184. Their tables are condensed in §2 and §4.
- **Every program alone on the seed.** Each of the 153 plan entries after the
  seed was run alone on q000 (`whim build --from N --to N --src q000`), 32 at
  a time, in 37 s wall-clock. The result is PASS if the program's anchors held
  and REFUSE otherwise. A PASS says only that nothing it needs from a
  predecessor is missing; it does not say it did the same cut. The results are
  in §3.
- **Times.** They are the per-phase times of the phase-184 build, under the
  machine's load:
  - 0-45: 336 s;
  - 46-95: 277 s;
  - 96-140: 168 s;
  - 141-184: 139 s;
  - the seed, and the regeneration of the editors, for the rest.
  About 3-4 s of every phase is its finish, the sweep and the canonical print
  (`doc/PIPELINE-COMPACTION.md` §1). The biggest single cuts are:
  - 5, the NFA engine: -13,694 lines;
  - 32, completion: -7,864;
  - 59, command-line completion: -6,040;
  - 53, conversion: -3,709;
  - 28, cindent: -3,675.
- **The endpoints of the three families.** In the product:
  - `command_line_scan()` accepts only `+{cmd}` (bare `+` is `$`). Anything
    else, file names included, is `ME_UNKNOWN_OPTION`.
  - The Ex command table has 98 rows of slim-vim.c's 600.
  - The option table has 184 rows of 559.

## 2. The kinds, phase by phase

- **Kinds.** Every entry is one of five:
  - VIM-DROP: removes capability.
  - VIM-REWIRE: vim-specific restructuring that keeps capability.
  - VIM-ADD: changes behaviour on purpose.
  - C-GENERIC: a step that would apply to any C program.
  - RECORD: no plan entry, or edits nothing.
- **Bundles.** A phase that bundles two kinds is listed under both.

| range | VIM-DROP | VIM-REWIRE | VIM-ADD | C-GENERIC | RECORD |
|---|---|---|---|---|---|
| 0-45 | 1-18, 20, 21, 23-43, 44-45 (in 48) | 22 | parts of 19, 21, 26, 42 | 0 (the seed) | 39, 44-47 run inside groups |
| 46-95 | 46-50, 53-66, 69, 74, 81, 85, 87-91, 93, 95 | 52, 67, 68, 71-73, 75-80, 92 | 51, 70, 94 | (78, 79 are generic in kind) | 82, 83, 84, 86 |
| 96-140 | 96, 100, 113, 121, 122, 125; halves of 103, 119 | 97, 98, 101-105, 108-112, 115, 117-119, 124, 126-131, 133, 135-140 | 112 (U+00DF) | 106, 107, 114, 120, 132, 134 | 99, 116, 123 |
| 141-184 | (154 is a dead call) | 141, 143-148, 150-153, 155-162, 165, 174, 176 | 142, 177, 178, 179, 180 | 149, 164, 166, 168-173, 175, 181-184 (167 generic in kind) | 163 |

- **Generic in kind but written with vim's names.** These are candidates for
  `crefactor/xform`, which would make the generic tail larger and the vim
  block smaller:
  - 78: empty functions and write-only statics;
  - 79: functions that return a constant;
  - 101: `main` demoted behind a launcher;
  - 105: variadic wrappers inlined;
  - 109: the header types owned;
  - 110: the includes moved below the core;
  - 118: libc routed through host shims;
  - 97 and 98: libc functions vendored (114 already uses `xform.Own`);
  - 117: realloc to malloc, copy and free;
  - 131: a type hidden by a cast round-trip;
  - 135: one-subtype struct inheritance;
  - 136: a constant function table devirtualized;
  - 138: a parameter always passed null;
  - 139: bsearch and qsort monomorphized;
  - 125: write-only fields;
  - 96 and 100: objects only ever NULL.

## 3. What the order really depends on

**53 programs run on the seed unchanged** (PASS on q000):

```
1 2 3 4 5 6 7 8 9 10 11 12 14 19 20 22 23 26 27 28 29 30 31 33 34 37 38 41
54 58 63 65 66 71 74 129 130 131 133 135 141 158 165
169 170 171 172 173 175 181 182 183 184          (the generic tail)
```

Most of Part I's first half is on that list. Its cutters anchor on text that
is in slim-vim.c already, and their counts hold there, so they can form the
drop block as they are. The 100 that refuse fall into three groups:

- **Counts that earlier cuts moved**, which is the common case.
  - Examples:
    - 24 wants 31 `setmouse()` calls, which is 33 minus the 2 that phase 10
      removed.
    - 35 wants `p_lpl` mentioned 2 times, after 3 and 18.
    - 43's `-c` arm matches once only after 3, 18 and 35.
    - 72 wants `ONE_WINDOW` 3 times.
    - 75 wants 49 dispatches.
    - 87 wants `exmode_active` 49 times.
    - 88-95 carry maps of before and after counts.
  - These are properties of the *order*, not of the cut. A package cut on the
    seed states its counts on the seed, once.
- **Spellings that a rewire or a generic step made.**
  - The later phases anchor on `nullptr` and `usize` (106), `musl_*` (97, 98),
    `__builtin_offsetof` (109), `bool` and `!f()` (166), and the canonical
    print.
  - Moving a vim phase before one of these means respelling its anchors.
  - Moving a generic step after it means the same for everything between.
- **Real prerequisites**, where one phase consumes another's product. Most of
  these are chains inside one subsystem, which stay together as one package:
  - host: 100 → 101 → 102 → 103 → 104 → 108 → 109 → 110;
  - libc: 117 → 118 → 119;
  - memline: 124 → 125 → 126 → 127 → 128 → 146;
  - regex: 150 → 176 → 177 → 178;
  - options typed: 151 → 152 → 153 → 154;
  - line getter: 160 → 162;
  - gotos: 170 → 171 → 172 → 173.

**The reverse constraint.** There is one more order that matters, and it
forbids the obvious move.

- **The constraint.** A phase that edits inside another command's handler
  must run before the phase that retires that command. After the retire, the
  sweep deletes the handler and the anchor matches nothing. Examples:
  - 13 and 16 edit `ex_drop()`, which dies at 45;
  - 33, 36, 38 and 40 edit `ex_listdo()`, which dies at 41;
  - 36 edits `:all`, `:ball` and `:argedit`, which die at 38 and 40.
- **What the Ex package does to it.** The edits it protects are to code that
  dies. Cut the Ex table first, and there is nothing to edit.

**Silent dependencies.** Two are known (`PIPELINE-COMPACTION.md` §3b).

- **The two.** A merged run gives other bytes without refusing:
  - 53 must be swept before 54, or 'arabic' survives;
  - 136 must be swept before 166, or two answers stay `int`.
- **What catches them.** Only the byte-identity bar does: the product is
  compared at the end, and a package boundary is compared where one can be
  (§7).

## 4. The families, as they are cut today

This is where the scattered drops that belong together are today.

**Command-line arguments: 13 phases.**

| phase | what goes |
|---|---|
| 3 | -h -? -A -F -H -g -f -X -Y -d -U -l -C -N -n -p -V -nb --help --version --clean --literal --nofork --noplugin --not-a-term --gui-dialog-file --startuptime --log |
| 4 | the program's name choosing a mode (rvim, view, ex, evim) |
| 18 | -y -Z -u -t -i |
| 21 | -r -L |
| 35 | -S, -s{file}, -w{file}, -W{file} |
| 40 | -o -O |
| 43 | -c --cmd -R -m -M -w{N} |
| 50 | -b |
| 69 | a second file argument |
| 85 | --ttyfail |
| 87 | -e -E -s -v |
| 88 | the file argument, `-`, `--` |
| 122 | -T {term} |

- **What survives.** Only `+{cmd}` survives, on purpose: the suites seed
  themselves with `+set paste`.
- **Residue.** Each phase also folded the flags its arguments set:
  `restricted`, `recoverymode`, `exmode_active`, `silent_mode`, `read_stdin`
  and others.
- **What is cut twice.** 69's argument rule is superseded by 88. 96 folds the
  residue of `-s` (`scriptin`).

**Ex commands: 34 phases, by two mechanisms.**

- **Retire, up to 79.** The row is pointed at `ex_ni`. Phases:
  1, 3, 7, 9, 10, 11, 14, 29, 33, 34, 35, 36, 37, 38, 40, 41, 42, 44-48,
  49, 58, 63, 69.
- **Delete, from 80.** 80 deletes all 489 retired rows, as declared in
  `080/delta.md`; the sweep takes their handlers. After it:
  - 89: write, wq, xit, exit, update, saveas;
  - 90: read;
  - 91: edit, enew, ex, visual, view;
  - 93: file.
- **Also in the family:**
  - 81 drops the `|` and `"` syntax;
  - 96 folds the residue of `:redir`;
  - 77 is moot once 80 has deleted the rows.
- **Split over several phases:**
  - the `:!` family: 8 stubs the process, 33 retires `:shell`, 44 retires
    `:!`;
  - the arglist, window and buffer commands: 38, 40, 41, 42 and 69.

**Options: 35 phases.**

- **The phases:** 2, 5, 6, 9, 10, 11, 12, 15, 16, 17, 18, 21, 24, 25, 28,
  32, 35, 36, 40, 42, 49, 50, 53, 54 (174 rows, computed), 55 (36, listed),
  56, 57, 58, 59, 60, 61, 62, 64, 95 (6, computed), and 103 ('termresize').
  Then 129 and 151-152 retype what is left.
- **The same idea three times.** 54, 55 and 95 are all "an option nothing
  reads".
- **What delays them.** Several drops wait on purpose (the `--strict` guard):
  a row initialises its global, so the row cannot go while the global has
  readers. The waits are:
  - 'tags', 'path' and 'autoread' until 16;
  - 'fileencodings' until 15;
  - 'fileencoding' until 17;
  - 'directory' until 21;
  - 'mousemodel' until 24.
  That guard is what keeps the option drops apart today.

**The other families**, by phase. Each group could be one package.

| family | phases |
|---|---|
| files and the filesystem (read, write, edit, names, glob, cd, home, env, timestamps, backup, ownership) | 1, 7, 13, 14, 20, 22, 25, 50\*, 61\*, 62\*, 70, 74, 89, 90, 91, 92, 93, 94, 96 |
| swap, recovery, memfile | 11, 21, 48 (noswapfile), 70 (swap half), 119 (b0_pid), 125 |
| encoding and locale | 9, 12, 15, 17, 51, 52, 53, 112 |
| runtime and startup | 1, 18, 35 (scripts), 56 |
| terminal | 19, 24 (mouse), 26 (signals), 61 (title), 67 (mouse residue), 85, 103 (half), 121, 122 |
| windows, tabs, buffers, arglist | 36, 38, 40, 41, 42, 68, 69, 71, 72, 73 |
| editing features (tags, completion, abbreviations, user commands, autocommands, jumps, marks, formatting, motions, lisp, cindent, rot13, language maps) | 10, 28, 29, 30, 31, 32, 34, 35, 57, 58, 59, 63, 64, 65, 66, 74, 75 |
| regexp (NFA, equivalence classes, one engine) | 5, 27, 76 |
| shell and process | 8, 26, 33, 44, 100, 102 |

\* surgery on code that 89-93 later delete whole: superseded.

## 5. The proposed pipeline

### D: drops, on the seed text

Each package cuts an *interface* or a *feature* once, on the text the
previous package left. It then lets the dead weight fall out, by the
**fall-out closure** below. Order matters only where one package's interface
is another's reader, so the outermost interfaces come first.

| package | what it does | replaces |
|---|---|---|
| D1 argv | `command_line_scan()` to its final shape: `+{cmd}` only, `parse_command_name()` gone, `mparm_T` to its final fields | the argument halves of 3, 4, 18, 21, 35, 40, 43, 50, 69, 85, 87, 88, 122 |
| D2 Ex table | `cmdnames[]` cut to the product's 98 rows at once. Rows are deleted, never retired: no `ex_ni` stage, and no reverse constraint. It also does 80's remaining work (each row's shortest abbreviation, the index gone, `:if`) and 81's syntax. | every `retire` step, 80, 81, 77, and the command halves of 89-91 and 93 |
| D3 options | `options[]` cut to the product's 184 rows at once. A row's global keeps its default as its own initialiser, as `nowinsizes` already does, so the `--strict` guard is not needed. | the row halves of about 35 phases, and 54, 55, 95 |
| D4 files | no read, write, edit, file name, glob, cd, home, environment, timestamps, backup or ownership | 7, 13, 14, 20, 22, 25, 89-94, 96, and the superseded surgery in 50, 51, 53, 61, 62, 69, 70, 74 |
| D5 swap and recovery | no swap file, recovery or disk memfile | 11, 21, the swap halves of 48, 70 and 119, and 125 |
| D6 startup and runtime | no runtime path, startup files, scripts, locale or viminfo | 1, 9, 18, 35, 56 |
| D7 encoding | UTF-8 only, the bytes kept, no conversion | 12, 15, 17, 51, 52, 53 |
| D8 terminal | one terminal name; no mouse, title, tty diagnosis or `$TERM` | 19, 24, 61, 67, 85, the drop half of 103, 121 |
| D9 one of each | one window, tab page, buffer and frame; no arglist | 36, 38, 40-42, 68, 71-73 |
| D10 editing features | tags, completion, abbreviations, user commands, autocommands, jumps, marks, formatting, motions, lisp, cindent, rot13, language maps | 10, 28-32, 34, 57-59, 63-66, 74, 75 |
| D11 regexp | one backtracking engine, no equivalence classes | 5, 27, 76, 135, 136 |
| D12 process | no shell-out, five signals, no deadly ladder | 8, 26, 100, the shell halves of 33 and 44 |

- **The fall-out closure.** A generic step in `crefactor/xform`, run after
  every package and knowing no code base. It repeats the following until
  nothing moves:
  - the sweep;
  - an object that is never written becomes its initial value at every read
    (52's technique, generalised);
  - `ConstBranch` (182's step);
  - a store nothing reads goes (165's, generalised);
  - a function that returns a constant is inlined (79);
  - an empty function's calls go (78).
- **What it replaces.** Most of today's hand-written folds are those steps,
  applied by name:
  - `restricted`, `recoverymode`, `exmode_active`, `silent_mode`;
  - `enc_utf8`, `has_mbyte`;
  - `ONE_WINDOW`, `firstwin == lastwin`.
- **What stays by hand.** Where the closure does not reproduce a fold byte
  for byte (§6), the package keeps a named *shape* edit for that function
  only.
- **Your first package, D1.** Its dead weight is mostly flags, and flags are
  exactly what the closure folds. So D1 is the right first test of the
  closure.

### R: rewires, vim-specific, keeping capability

| package | today's phases |
|---|---|
| R1 the core stops the process no more (the host chain) | 101-104, 108 |
| R2 the core owns its libc | 97, 98, 111, 112, 114, 115, 117-119, 139 |
| R3 the boundary | 109, 110 |
| R4 memory is the host's | 124, 132 (its knobs), 148 |
| R5 the memline tree | 125-128, 146 |
| R6 the regex engine for translation | 141, 143, 150, 156, 176 |
| R7 option types | 129, 151-154 |
| R8 dead parameters and types | 130, 131, 133, 137, 138, 140, 157, 160, 162 |
| R9 goto shapes no generic step takes | 141, 143-145, 161, 174 |
| R10 behaviour | 142 (no build date), 158, 179 (fixes), 180 (`WHIM_TIME`), 177-178 (the parallel `:%s`, `:g`) |

- **Size.** About 20 phases, where there are now 55.
- **R10's `:%s` is the one knot.** It rests on 175 (`MemberOut`) and 176
  (`StateParam`), which are generic steps with vim's knobs.
  - One way is to keep 175 and 176 inside R as vim-configured generic steps.
  - The other is to respell 177's anchors on the text before 175.
  - I recommend the first.

### G: generic C, each step once, at the end

In their forced order:

1. `NullptrUsize`, `Attrs` (106, 107)
2. `Unions` (120)
3. `DropCalls` and `EmptyBlocks` (132, 134)
4. `NeverNull` (149)
5. `DeadStmt` (164)
6. key names (167)
7. `GotoReturn`, `GotoTail`, `GotoBreak`, `GotoLoop`, `GotoBlock` (168,
   170-173; 168 may fold into `GotoTail` with a tail of 0)
8. `LocalOut` and `StructScalar` (181)
9. `Identity`, `AsciiClass`, `ConstBranch` (182)
10. `BoolRet` with `Globals` and `Relax` (166, 183, 184 as one run)
11. `Includes` (169, last)

- **Moving `NullptrUsize` and the attributes to the end** means every
  vim phase in D and R is written in slim's spelling (`NULL`, `size_t`), once.
  That is the respelling cost §3 names, paid once.
- **The candidates of §2** (78, 79, 101, 105, 109, 110, 118, 136, 138, 139
  and others) can join G as their generic versions are written. Each one
  moved shrinks R.

### Parallel

- **What is parallel now.**
  - `whim-build-check` already runs every link side by side (83 s against
    1,005 s in order).
  - Whole-pipeline parallelism is limited by what the text makes sequential.
    One file, and each phase reads its predecessor.
- **What the new shape adds.**
  - **Fewer, larger links.** About 45 links make the check shorter, provided
    no single package becomes a 100 s link. The earlier study's 21-run
    experiment had a 111 s link.
  - **Independent packages cut side by side.**
    - The families are nearly disjoint regions of the file. D5-D12 could be
      computed against one input, each as a set of edits, then applied
      together and swept once.
    - That turns eight sweeps into one. It is also exactly what the silent
      dependencies punish, so it needs the per-package guard of §7.
  - **Per-function transforms.** They could run per function, concurrently:
    `GotoTail` and the rest, `LocalOut`, `StructScalar`, `DeadStmt`,
    `ConstBranch`.
  - **Host and core.** After R3 the core and the host are separate texts, so
    R's host-only work (124, 147, 180) runs beside the core's.

### Estimate

| | phases | serial build |
|---|---|---|
| today | 154 (plus 8 records) | 1,005 s |
| proposed | D ≈ 12, R ≈ 20, G ≈ 11, seed 1: about 45 | about 400-500 s |

- **Why it is faster.** About 110 fewer finishes at 3-4 s each, and about
  55 superseded cuts that no longer run.
- **What it is not.** It is an estimate; nothing here was built.

## 6. The risk: byte for byte

The bar is the committed `whim-vim.c`. What could keep a reordered pipeline
from reaching it:

1. **Hand folds against the generic fall-out.**
   - **The concern.** A hand-written fold may shape a surviving function
     differently from a generic closure. For example, a `?:` folded to one
     arm versus a branch deleted, or a flag's last read kept.
   - **What contains it.** The canonical print removes spelling differences,
     but not shape ones. Each package that uses the closure must be checked
     function by function against the product. Where it differs, it keeps a
     named shape edit for that function. The cost is unknown until D1 is
     tried.
2. **Superseded surgery is not always dead.**
   - **The concern.** A Part I edit inside `readfile()` or `do_ecmd()` dies
     with them, but its *side effects* elsewhere may survive into the
     product:
     - an option dropped;
     - a field folded;
     - a helper swept.
   - **What contains it.** D4 must still do those. The per-phase tables in
     the agents' reports list what each did.
3. **Declaration order and insertions.**
   - **The concern.** Some phases insert code:
     - the musl tables (98);
     - the launcher (101);
     - the helpers at the core's top (182);
     - the includes moved below the core (110).
     Where they land depends on anchors that the reorder moves.
   - **What contains it.** The final bytes check.
**Measured on the product, 2026-09-30, and corrected 2026-10-01.** The
first measurement said the product had no object that is read but never
written. That was wrong. The closure's own analysis (`crefactor/xform`'s
`FallOut`, which reads the type-checker's write counts) finds 32 in the
product:
- `pum_bg_*`, the popup-blend state;
- `edit_submode*`;
- `curbuf_lock`, `allbuf_lock` and `ex_normal_lock`;
- `in_assert_fails`, `read_cmd_fd` and `redraw_not_allowed`;
- `p_wh`, `p_wmh` and `p_wmw`;
- and others.

Run on the product, the closure would still:
- write 68 reads as their values;
- take out 13 parameters that every call passes as one constant;
- take 14 constant ifs and 16 constant operands of `&&` and `||`.

So **the product is not a fixed point of the closure**: the hand pipeline
left that dead weight in it. A closure seeded with *every* never-written
object therefore cannot give today's bytes, whatever order it runs in.

The seed has 15 such objects of its own: `in_assert_fails`,
`redraw_not_allowed`, `mouse_vert_step`, `nv_max_linear`, … D1a's cut adds
`read_cmd_fd` and `has_dash_c_arg`, and 11 members of `mparm_T`. Of those
the product still keeps `read_cmd_fd`: phase 88 left it to "the stdin
phase", which never folded it.

- **Empty functions.** The product has none.
- **Functions returning a constant.** The product has 2 of its 1,668:
  - `did_set_number_relativenumber` is used as a value, which the rule
    leaves alone.
  - `ctrl_x_mode_scroll()` is still called in `if (ctrl_x_mode_scroll())`.
  A closure that inlines every constant-return function would fold that
  call and miss the bar. The closure as written only follows values it
  wrote itself, so it does not see this one.

4. **Enumerators pinned by the sweep.** A deleted enumerator pins its
   successor's value. The values are the originals, whatever the order, so
   this should be safe. It is not measured here.
5. **The silent dependencies of §3.** Caught at the end, or earlier by the
   guard of §7.

## 7. The migration, one package at a time

Each step leaves a pipeline that gives today's product byte for byte, proven
by `whim-build` and `whim-build-check`, with the four editors untouched.

1. **Tooling.**
   - A *first difference* tool: the first function whose text differs
     between a candidate product and the committed one.
   - A *package guard*: after a D package, every function, object, command,
     option and argument that is in the product must still be there. That
     catches an over-cut at the package, not 17 minutes later.
   - The fall-out closure, as an `xform` step with its own gcc
     before-and-after tests.
2. **D1 argv.**
   - One cut on the seed.
   - Remove the argument halves from 3, 4, 18, 21, 35, 40, 43, 50, 69, 85,
     87, 88 and 122, and restate their counts.
   - This is the probe for the fall-out closure: how much of the flags'
     residue it takes, and how many shape edits remain.
   - **D1a, done (branch `reform-d1`).** The argument parser was moved to
     the front with no closure, as a first step.
     - **The cut.** `internal/cut/argvfront.go`, the first step of phase 1:
       - `command_line_scan()`'s body is the product's;
       - the calls of `parse_command_name()` and `early_arg_scan()` go;
       - main's `--clean` prescan goes;
       - the ME_* enumerators and `main_errors[]` rows are rewritten to
         the product's two and three, in one place. Phases 88 and 122 had
         each renumbered part of the way.
     - **What it replaced.** Each argument half of 3, 4, 18, 21, 35, 40,
       43, 50, 69, 85, 87, 88 and 122 is deleted:
       - all three `dropopts` steps;
       - phase 4's plan entry, which is a record only now;
       - phase 88's parser and table sections;
       - phase 122's letter switch and renumbering.
       The counts those phases assert were restated on the new text.
     - **Size.** The Go lost 588 lines net: 170 added, 758 deleted.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       - In order: 153 phases, 999 s (1,005 before).
       - The parallel check: 152 links, 70 s.
       - The time did not move. The halves cut were small, and the phases
         they sat in still run. What D1a buys is fewer places, not seconds.
     - **What stays.** The flags the options set (`params.*`,
       `exmode_active`, `silent_mode`, `read_cmd_fd`, `edit_type`) are
       still folded by the phases that folded them. Each is now never
       written from phase 1 on. D1b replaces those folds with the closure,
       and that is where §6 item 1 gets tested.
   - **D1b, done (branch `reform-d1`).** The closure replaces D1's folds,
     and the product is still byte for byte the same.
     - **The closure.** `crefactor/xform/fallout.go`, which knows no code
       base. `FallOutOf(cut, hold...)` runs a cut, then seeds the closure
       with what the cut left unwritten:
       - an object or struct member that code reachable from `main`
         wrote before the cut and nothing writes after it;
       - minus the names held.

       Each read of a seed becomes its value, as an enumerator
       `fallout_V` declared while the step runs. The rules after the seeds
       fire only on text that holds such a value, or on a branch they took,
       which is marked by a comment. So code the cut didn't touch stays
       exactly as it is.

       The rules:
       - constant expressions;
       - a constant operand of `&&` and `||`;
       - `if`, `while` and `?:` of a constant condition;
       - the statements after a jump that a taken branch revealed;
       - calls of a function whose body became `return K;`;
       - a parameter that every call passes as one constant;
       - calls of a function left empty.

       They run in rounds to a fixed point. A member is seeded only when
       its struct's instances are all static with no initializer, and are
       reached only through pointers to themselves or `memset` to zero.
     - **The bar held only by seeding per package** (§6, corrected): the
       product keeps 32 objects that nothing writes. So the seeds are the
       cut's own, and `read_cmd_fd`, which the product keeps, is held.
     - **What D1 left.** 12 seeds: `has_dash_c_arg` and 11 members of
       `mparm_T`. Five rounds give:
       - 14 ifs taken;
       - 7 calls of constant functions;
       - 3 parameters;
       - 1 empty function's call.

       Then `read_stdin()`, `is_not_a_term()`, `exe_pre_commands()` and
       `set_init_clean_rtp()` are the sweep's.
     - **What it replaced.**
       - `optreaders`' `--clean`, `--not-a-term` and `-n` folds (phase 3);
       - `nostartup`'s `-u NONE` test (18);
       - `norecover`'s stdin arm (21);
       - all of phase 43 and all of phase 88, which are records now;
       - restated counts in 62, 70, 85 and 132.

       **The shapes came out the same**: no fold needed a named shape edit
       to meet the bar.
     - **Cost.** Phase 1 takes 28 s instead of 9 s:
       - two analyses of about 4 s each, before and after the cut;
       - five rounds of about 2 s, mostly type-checking.

       Making it that fast took:
       - a type's name computed once;
       - only the functions a rule can fire in indexed;
       - the first round's parse reused.

       The whole build: 151 phases in order, byte for byte, 1,004 s (999
       after D1a). The parallel check: 150 links, 76 s. Two phases fewer
       paid for the closure's 19 s.
3. **D2 Ex table.** Delete rows at once, remove every `retire` step, and
   restate the named edits that survive.
   - **D2a, done (branch `reform-d2`).** The rows are retired at the front;
     phase 80 still deletes them.
     - **The cut.** `exfront`, phase 1's second step, behind `FallOutOf`.
       It points every row declared in `internal/phase/001/delta.md` at
       `ex_ni`. The list moved there from 080; phase 80 now asserts it
       finds 489 stubs. Of the 489, 271 are stubs in the seed already
       (ten as `ex_script_ni`, left so), and 218 are retired here.
     - **What falls out.** 21 objects only the retired handlers wrote: the
       `:sort` state, `redir_fd`, `filetype_*`, `last_event`,
       `ucmd_locked`, … They fold in two rounds: 5 ifs and 6 reads.
     - **What it replaced.**
       - all 21 `retire` steps, the `retire` op and its subcommand;
       - the row rewrites in `noruntime`, `nointro` and phases 58, 63
         and 69;
       - every edit inside a handler that now dies at phase 1: in 9, 13,
         16, 20, 24, 35, 36, 38, 40, 41, 42 and 62. That is the reverse
         constraint of §3, gone as predicted;
       - all of phase 33, which is a record now;
       - counts restated in 40, 60, 69, 93, 94, 95 and 96, where
         `redir_fd`, folded at phase 1, had been counted.

       Phase 60 now writes `redirecting()` as `return FALSE;`, since
       `redir_fd`'s half was already gone. Phase 96 asserts that, instead
       of `redir_fd`'s single write.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       No shape edit was needed.
       - In order: 150 phases, 989 s (1,004 after D1b).
       - The parallel check: 149 links, 82 s.
   - **D2b, done (branch `reform-d2b`): the rows deleted at the front.**
     - **The design.** A generic rule ("a comparison with a deleted
       enumerator is false") is unsound on the seed:
       `window_layout_locked(CMD_close)` and `ea.cmdidx = CMD_tabnew` pass
       those values through live code until phases 36 and 40 remove them.
       So the rows go, and their enumerators stay, moved after `CMD_SIZE`.
       Every comparison still compiles and means what it meant; no parsed
       command can reach them.
     - **The cut.** `internal/cut/extable.go`, phase 1's third step, is
       phase 80's table half on the seed:
       - the stub rows go;
       - each of the 111 left carries its shortest abbreviation, from the
         600-row table, proved over all 2,538 prefixes;
       - the prefix index goes, and the lookup is a scan;
       - the one-character set shrinks to what exists.
     - **What it replaced.**
       - phase 80's first half: phase 80 now deletes the enumerators past
         `CMD_SIZE` and keeps its code half;
       - all 15 `cmdidxs --check` steps, the `cmdidxs` op, subcommand and
         generator;
       - `nocmdopts`' `EX_RESTRICT` row edit: all 24 such rows were stubs;
       - phase 77 now asserts no row carries `EX_BUFNAME`, instead of every
         one being a stub, and keeps its fold.

       Phase 77 was first made a record by mistake. Its fold still removes
       buffer-name matching, and phase 93's counts caught the divergence.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte:
       150 phases in order, 982 s; the parallel check, 149 links, 84 s.
       Phase 81's syntax and the deletions of 89-91 and 93 stay where they
       are: they delete live commands, and belong with their packages
       (D4, files).
4. **D3 options.** Default-as-initialiser, then remove 54, 55 and 95 and the
   row halves.
   - **D3a, done (branch `reform-d3`): the rows dropped at the front, no
     closure.**
     - **Why no closure, and no default-as-initialiser.** A row's default
       reaches its global through `set_init_1()`, under `'compatible'`, and
       through what startup does after. Modelling that to give each global
       its "right" initialiser, and then folding with it, would put
       shapes at risk that the hand folds decided with that knowledge.
       Instead each global stays at its static zero until the phase that
       removes its readers. Every drop without `--strict` already did that,
       and nothing reads the intermediate texts' behaviour.
     - **The cut.** `internal/cut/optfront.go`, phase 1's fourth step. It
       drops the 375 rows listed in `optfront.md`.
       - A row is found inside `options[]` only. Over the whole file,
         `'arabic'` first matched the encoding table, and the row
         survived to phase 54.
       - Each name's line goes from every list, as `dropoptions` always
         did. The product keeps that: `"key"` is out of `'selectmode'`'s
         values and `"debug"` out of the history names. A whitelist-only
         version differed from the product in exactly four tables.
     - **What it replaced.**
       - all 53 `dropoptions` steps, the op, `query-dropoptions` and the
         subcommand; `DropRow` keeps no guard;
       - all of phase 54, a record now. Phase 95's computed set is now an
         assertion that its six rows are gone; its code edits stay;
       - the edits of rows and of handlers that died with them:
         - `noruntime`, `nowildmenu`, `nolocale`, `noenc`, `nofencs`, `nofenc`;
         - `nogetenv`, `nobackup`, `nomouse`;
         - phases 93 (`did_set_readonly`) and 103 (`'termresize'`);
       - counts restated in 24, 35, 61, 91, 93, 94 and 95.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       - In order: 149 phases, 948 s (982 after D2b).
       - The parallel check: 148 links, 86 s.
   - **D3b, done (branch `reform-d3b`): the closure over the dropped
     options.** `optfront` now runs behind `FallOutOf`, with a hold list.
     - **The seeds.** 135 objects are left unwritten by the drop: the
       options' globals, and `pum_border`'s members. Their real value is
       zero now, since no row initializes them.
     - **The fold.** 113 are folded, in 6 rounds: about 200 reads, 44
       ifs, 20 `?:`, a few emptied functions.
     - **Held: 22**, where the closure's shape and the hand fold's
       differ, and the product has the hand fold's:
       - `p_wmnu`: `nowildmenu` writes `a && b && c` flat, where the
         closure keeps the parentheses around the operand it keeps;
       - the backup family: `dobackup`'s assignment dereferences `p_pm`;
       - the window sizes: `nowinsizes` gives them their real defaults
         (`p_ea` TRUE), and later phases fold with those values; the
         product keeps three;
       - `p_fic` (`= FALSE`, where the closure writes 0) and `p_acl`;
       - the autowrite family: phase 62 cuts whole blocks around
         `autowrite_all()`.
     - **A bug the probe found.** A node of a macro's expansion carries
       the invocation's token positions (`MAX(...)`). A rewrite's span
       must now be balanced, or it is refused.
     - **What it replaced.** Hand folds in 16 (`ml_open`'s swap test), 59
       (`p_wic`) and 91 (`p_ur`'s count). Every other hand fold either
       still finds its anchor or folds a held variable.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       - In order: 149 phases, 962 s, against 948 for D3a: the closure costs
         about 14 s.
       - The parallel check: 148 links, 107 s against 86. It is bound now
         by its longest link, phase 1, whose three closures take most of its
         ~75 s. That is the case for making the closure faster (agenda).

5. **D4 files, and D5.**
   - Delete the superseded surgery.
   - This is the largest simplification, and the largest risk (§6, item 2).
6. **D6-D12 and R.**
   - Group the remaining drops and rewires by family.
   - Respell R to slim's spelling while `NullptrUsize` moves to G.
7. **G.**
   - Merge the reruns: `BoolRet` once, and possibly 168 into `GotoTail`.
   - Move each generic step to the end.
8. **Renumber and archive.**
   - The new phases go in a fresh directory and a fresh numbering (see
     question 1).
   - The old `internal/phase/NNN/` records go to an archive.
   - `GOALS.md` indexes both.
   - CLAUDE.md is rewritten to the new shape.

- **Effort.** Steps 2-5 are the bulk. They rewrite about 90 cutters and edit
  programs as about 12 packages. The cutters that pass on q000 (§3) move with
  little change; the rest have their counts restated on the seed.
- **Timing.** The first probe, D1, would say within a day whether the
  fall-out closure carries the bar.

## 8. Questions for the review

**Answered 2026-09-30:**
1. **Block names**: `d01-argv`, `r03-boundary`, `g05-gotos`. The archive
   keeps the old numbers.
2. **Build the fall-out closure**, and probe it on D1 before relying on it.
3. **Behaviour changes stay with their package**. 177-178 go in the regex
   rewire.
4. **The generic-in-kind phases are a later reform.** They stay in R, named,
   for now.
5. **Start with D1 argv.**

The questions as they were asked:

1. **Numbering.** Today's 184 numbers are cited about 4,200 times in docs,
   comments and commit messages (`PIPELINE-COMPACTION.md` §6).
   - A new pipeline numbered 0-45 would collide with the archived 0-184.
   - I would name the new phases by block and number: `d01-argv`,
     `r03-boundary`, `g05-gotos`. The directory would sort in pipeline order.
   - The archive would keep the old numbers as they are.
   - Is that acceptable, or do you want plain numbers?
2. **The fall-out closure.**
   - It is the clean way to "let the dead weight fall out", and new generic
     code with the byte-identity bar on it.
   - The alternative keeps every hand fold as a named edit inside its
     package: safer, and less simple.
   - I recommend building the closure and measuring it on D1 before
     deciding.
3. **Where behaviour changes go.** 142, 177-180, and the smaller changes
   inside drops (for example 21's "N seconds ago" and 42's `:saveas`
   rename) could be a block of their own, B, between R and G. Or they could
   stay with the package whose code they touch. I recommend staying with the
   package; 177-178 would be R10.
4. **Generic-in-kind phases (§2).** Writing them as `xform` steps moves them
   to G and makes R small, at the cost of new generic code for each. Should
   they be part of this reform, or a later one?
5. **Order of work.** Start with D1 argv, as proposed? Or with D4 files,
   which removes the most superseded work but carries the most risk?
