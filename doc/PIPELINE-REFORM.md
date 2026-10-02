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
   - **D4, done (branch `reform-d4`): the commands that name a file,
     deleted at the front.**
     - **The cut.** `filefront`, phase 1's fourth step, deletes the rows of
       the 13 commands of the 111 left that the product has not. Their
       enumerators move after `CMD_SIZE`, as `extable` did for the stubs'.
       Phase 80 now deletes only the enumerators past `CMD_SIZE` that
       nothing names; the others go with their last uses in 89-93, or with
       the sweep.
     - **What dies with them.** The handlers, and with them the write path
       (`buf_write()`, `check_overwrite()`), `:read`'s, `:edit`'s
       (`do_exedit()`), `:file`'s (`rename_buffer()`, `setfname()`) and
       `do_bang()`'s filters.
     - **What it replaced.** Every edit inside them:
       - in `noshellout`, `nocmdopts`, `nomemfile`, `nosession`,
         `nowindows`, `nobuflist`, `onebuffer` and `oneoptset`;
       - in phases 56, 61, 62, 64, 68, 75, 78, 79, 80 and 87.

       Phases 89, 90, 91 and 93 lose their row deletions and row counts:
       their enumerators are taken where still named, pinned or not. Their
       code edits stay. Phase 93's writers of the name fields are two now
       (`buflist_new()`, `shorten_buf_fname()`), its readers four.
     - **What was not superseded.** The surgery the survey called
       superseded (50, 51, 53, 61, 62, 69, 70, 74) mostly still found its
       anchors: that code is live until 89-93 or 92.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       - In order: 149 phases, 986 s (962 after D3b). No phase goes, since
         89-93 keep real edits.
       - The parallel check: 148 links, 116 s, bound by phase 1, which now
         runs four closures.
   - **The closure's speed, done (branch `reform-speed`).** Phase 1 took
     71 s with four closures, one per front cut, each with two full-file
     analyses and 0-6 rounds of about 2.3 s.
     - **One closure.** It now runs once, after all five front cuts
       (`front`).
     - **A fixed point over the seeds as well.** After the rounds settle,
       what is unwritten is asked again. What a fold took the last write of
       becomes a seed too: `if (params.no_swap_file) p_uc = 0;` goes with
       argv's flag, and `p_uc` falls out. One closure over everything
       missed that until this was added.
     - **The seed check reuses the last round's parse.** The values stay
       as their enumerators until the very end.
     - **The objects' values are computed once a pass**, not once a round.
     - **A seed's read counts as its value** in the rules' own evaluator,
       before any marker is written: comparisons, `!`, `&&`, `||`, `?:`,
       bit operators on non-negative values. It answers only where C's own
       answer cannot differ. So one round can take the `if` that reads a
       seed, where it took two. The rules now also visit every function
       that reads a seed.

     The result: phase 1 takes 38 s, with two passes of 5 and 1 rounds,
     and the same q001. The whole build in order takes 949 s (986); the
     parallel check, no longer bound by phase 1, 79 s (116).

   - **The moves of 94, 92 and 81 (item 4).** Three phases whose one cut
     is a drop the product makes, moved to the front one at a time.
     - **94, done (branch `reform-quit`): `:q` quits.** `quitfront`, phase
       1's fifth cut, folds `ex_quit()`'s refusal never on the seed (its
       test there also asks `buf_hide()`, `check_more()` and
       `only_one_window()`). `check_changed()`, `check_changed_any()` and
       the switch-buffer/switch-window island its tail was the last caller
       of (`set_curbuf()`, `enter_buffer()`, `win_enter_ext()`,
       `get_winopts()` and seven more) go with it.
       - **What it replaced.** Every edit inside them: `onebuffer`'s
         `set_curbuf` folds, and edits in 62, 68, 71, 75, 78, 79 and 93
         (the refusal's E162 and E37 literals, `check_changed_any`'s walk,
         `win_enter_ext`'s block). The counts of `open_buffer` in 90-92 fall
         by one, `enter_buffer()`'s call.
       - **Phase 94 keeps its two extras**, whose text is still there: the
         tail that cannot run (`getout(0);` alone), and the two fields
         whose readers went with the island. Its counts are restated on
         q093, and its island check goes.
       - **Result.** The chain gives the committed `whim-vim.c` byte for
         byte. In order: 149 phases, 948 s. The parallel check: 148 links,
         77 s; phase 1 37 s.
     - **92, done (same branch): nothing reads a byte.** `readfront`, phase
       1's sixth cut, folds `open_buffer()`'s two read arms (the named file
       and stdin) never. `fix_help_buffer()` and `read_buffer()` go at once.
       `readfile()` (787 lines) goes once its other callers have, by phase
       49, and the swap-file check, the format and encoding detection, and
       the read autocommands go with it.
       - **What it replaced.** Every edit inside them:
         - `lfonly`'s 15 readfile edits and its fifo and stdin `'binary'`;
         - `keepbytes`' `++bad`;
         - `noconv`'s conversions;
         - `nogetenv`'s local-additions scan;
         - folds in 62, 70, 75 and 87.

         The counts in 75 (bare dispatches 36 -> 25), 87, 90 and 91 are
         restated.
       - **Phase 92 keeps anchors 2-4**: `read_fifo`'s last test, and
         `open_buffer()`'s parameters, which every caller passed as
         `FALSE, NULL, 0`.
       - **Result.** The chain gives the committed `whim-vim.c` byte for
         byte. In order: 149 phases, 934 s. The parallel check: 148 links,
         85 s (the machine's load; phase 1 is still 37 s).
     - **81, done (same branch): one line, one command.** `onecmdfront`,
       phase 1's seventh cut, makes phase 81's edits on the seed's
       spelling:
       - `separate_nextcmd()` splits at a newline only;
       - `ends_excmd()` and `ends_excmd2()` answer the end of the line,
         their Vim9 `#` going with the rest;
       - `find_nextcmd()`, `check_nextcmd()`, `do_one_cmd()`'s trailing
         check and comment line, `:|`, `:substitute`'s tail and
         `:append`'s bar stop knowing `|` and `"`.

       Nothing falls out (no object loses its last write), so the order
       among the front cuts does not matter.
       - **What it replaced.** Phase 81 entirely: it is a record now. Also
         phase 79's two literals on `separate_nextcmd()`'s condition and
         phase 80's `:redir @` exception. Phase 79's Vim9 counts are
         restated (`if (vim9script)` 2 -> 1, `if (in_vim9script())` 8 ->
         5).
       - **Result.** The chain gives the committed `whim-vim.c` byte for
         byte. In order: 148 phases, 928 s. The parallel check: 147
         links, 77 s; phase 1 37 s.
   - **D5, done (branch `reform-d5`): swap and recovery.** Phase 1 runs
     `noswap`, `norecover` and `nomemfile` (phases 11 and 21) after
     `optfront`, under the same closure. 11 and 21 are records now.
     - **What moved earlier with them.** `ml_recover()` was `readfile()`'s
       last caller but phase 13's two, so `readfile()` dies at phase 13
       rather than 49. Three things fall out at phase 1:
       - `mparm_T.edit_type`;
       - `mf_dont_release`, assigned only in `mf_close_file()`;
       - the swap file's timestamps (`get_ctime()`, `add_time()`,
         `vim_localtime()`).
     - **What it replaced.**
       - `nofencs`' and `nofenc`'s readfile and block-zero entries;
       - `nogetenv`'s `vim_localtime` cache;
       - phase 125's `mf_dont_release` section.

       `nocmdopts`' tagname anchor is scoped by its new neighbour.
       `nomemfile`'s two directory-list edits now refuse rather than
       silently doing nothing: on the seed `p_path`'s term follows
       `p_dir`'s, and the term alone is removed.
     - **What stays.** Phase 125's residue: its proofs rest on text that
       only exists late, such as `newfile` FALSE at all eleven
       `ml_append()` calls. The swap halves of 48, 70 and 119 also stay;
       each still finds its anchors where it is.
     - **Result.** The chain gives the committed `whim-vim.c` byte for
       byte. In order: 146 phases, 923 s. The parallel check: 145 links,
       79 s; phase 1 40 s.

   - **D6, done (branch `reform-d6`): startup and runtime.** Phase 1 runs
     `nolocale` (9), `nostartup` and `nocmdopts` (18), `nosession` (35) and
     `whim56` after D5's cuts. 9, 18 and 35 are records now. 56 keeps
     `'keywordprg'`'s cut and `droplocal b_p_kp`, because on the seed that
     field still has readers (K's `nv_ident`, `get_varp_scope`) that earlier
     phases take.
     - **Written for the seed.**
       - `nocmdopts` finds `mparm_T`'s `tagname` by the struct it closes.
       - `nosession`'s filetype fold is scoped to `set_rw_fname()`, since
         `do_write()`'s copy is still in the unswept text inside phase 1.
         Its `p_lpl` postcondition, which only holds after the sweep, is
         gone.
       - `whim56` folds the whole directory-list test, since every option
         it names is dropped. That supersedes `nobackup`'s and
         `noinertopts`' term edits and 55's `'cdpath'` literal, whose
         program is gone; 55 keeps its `droplocal`.
     - **What falls out earlier.** `close_disallowed` and `autocmd_busy`,
       so `notabs`' anchor and 71's `free_buffer` edit change.
       `sticky_cmdmod_flags` and `aucmd_cmdline_changed_count` lose their
       writers too, but the product keeps them, so the closure holds them.
     - **Result.** The chain gives the committed `whim-vim.c` byte for
       byte. In order: 143 phases, 899 s. The parallel check: 142 links,
       85 s; phase 1 47 s.

   - **D7, done (branch `reform-d7`): encoding.** Phase 1 runs `noenc` (12),
     `nofencs` (15), `nofenc` (17) and `utf8only` (53) after D6's cuts. 12
     and 15 are records; 17 keeps its `droplocal`. `keepbytes` and
     `noconv` stay at 53: their `getargopt` chain starts after the `++ff`
     arm that phase 50's `lfonly` takes. `lfonly` cannot move, because at
     the front `readfile()` is live until phase 13 and it would need every
     readfile edit back.
     - **Written for the front.**
       - `nofenc` counts the BOM clears in readfile and the recovery too
         (3).
       - `nofencs` no longer asserts that `p_fencs` is unread, since
         readfile still names it.
       - `noabbr`'s literals and `noident`'s body are in UTF-8's spelling,
         because `utf8only` now folds `has_mbyte` before them.
     - **Result.** The chain gives the committed `whim-vim.c` byte for
       byte. In order: 141 phases, 912 s. The parallel check: 140 links,
       111 s. It is bound by phase 1 again, now 61 s, so the front's cost
       is the next thing to measure.

   - **D8, done (branch `reform-d8`): the terminal.** Phase 1 runs `noterm`
     (19), `nomouse` (24) and `whim61` (61, the window title) after D7's
     cuts. 19, 24 and 61 are records, and 61's program stays for the front
     to call.
     - **What stays.** 67 (mouse and spell plumbing, write-only flags) and
       85 (tty diagnosis) count what phases 2-66 and 2-84 took, so they
       stay. So do 103's drop half and 121, which are Part II rewires.
     - **Counted on the seed.** `nomouse`'s `setmouse()` calls 30 -> 33,
       and 61's `need_maketitle` writes 5 -> 7.
     - **What it replaced.** `noconv`'s `mb_tail_off()` edit:
       `mb_tail_off()` goes with `maketitle()` at phase 1.
     - **Tried and not kept.** A sweep inside the front before these cuts
       took only one of the three extra calls, because the rest are in live
       code that phases 2-23 take. It was removed.
     - **Result.** The chain gives the committed `whim-vim.c` byte for
       byte. In order: 138 phases, 864 s. The parallel check: 137 links,
       106 s; phase 1 64 s.

   - **D9, done (branch `reform-d9`): one of each.** Phase 1 runs
     `noinert` (37), `notabs` (36), `noarglist` (38), `nowindows` and
     `nowinsizes` (40), and `nobuflist` (41) after D8's cuts. 36-38, 40 and
     41 are records. `onebuffer` (42) stays where it is: on the seed,
     `buf_hide()` has 24 mentions, which phases 2-41 take. So do 68, 71 and
     72-73, the structural folds, which count late text.
     - **Written for the front.**
       - `noinert` comes first, so that `:browse` is gone from
         `case 'b'`.
       - `notabs` folds `window_layout_locked()`'s test as the seed spells
         it, before the closure.
       - `nowindows` counts 18 `'scrollbind'` writes, not 14.
       - The closing assertions written for swept text are gone: `cmod_tab`,
         `CMD_windo`, `DOBUF_SPLIT`, the `ECMD_` flags, and the
         argument-list names.
     - **What it replaced.** In phase 3, `optreaders`' three `-p` edits;
       in phase 22, `nochdir`'s `edit_buffers()` edits, keeping
       `start_dir`'s free; in phase 30, `noident`'s two CTRL-W cuts in
       `do_window()`. 87's vgetorpeek fold and 93's open_buffer fold go,
       because `pending_exmode_active` and `readonlymode` now fall out at
       phase 1. Their counts are restated.
     - **Held.** `skip_win_fix_cursor` loses its writers, and the product
       keeps it.
     - **Result.** The chain gives the committed `whim-vim.c` byte for
       byte. In order: 133 phases, 821 s. The parallel check: 132 links,
       101 s; phase 1 69 s.

   - **D10-D12, done (branch `reform-d10`): editing features, the regexp
     engine, the process.** Phase 1 runs these after D9's cuts. They
     applied to the seed as written.
     - **The cuts.** `nonfa` (5), `noshellout` (8), `notags` (10),
       `nosignals` (26), `noequiclass` (27), `nocindent` (28), `noucmd`
       (29), `noident` (30), `nofnamemod` (31), `nocompl` and
       `nocomplkeys` (32), and `noabbr` (34).
     - **Records.** 5, 8, 10, 26, 27, 29-31 and 34. 28 and 32 keep their
       sweep and `droplocal`.
     - **What stays.** The editing-feature programs of 57-59, 63-66, 74
       and 75, the engine's 76, 135 and 136, and the shell halves of 33,
       44 and 100. They count text that only exists late.
     - **What it replaced.** `nogetenv`'s `-complete=environment` row,
       which goes with `:command`.
     - **Held.** `nocompl` takes the writers of the popup menu's blend
       state (`screen_pum_blend`, `pum_bg_*`) and of the completion
       submode's message (`edit_submode*`), which the product keeps.
       Folding them would have left phase 110 an unused label,
       `next_col:`.
     - **Result.** The chain gives the committed `whim-vim.c` byte for
       byte. In order: 124 phases, 785 s. The parallel check: 123 links,
       120 s. Phase 1 is 92 s and bounds it.

6. **D6-D12 and R.**
   - Group the remaining drops and rewires by family.
   - Respell R to slim's spelling while `NullptrUsize` moves to G.
   - **Done as far as the bytes allow.** D5-D12 are above. What stays of
     the drops is written against text only the phases before it leave:
     `onebuffer`, `lfonly`, `keepbytes`, `noconv`, 57-59, 63-68, 71-76 and
     85. Step 10 brings what the bytes later allowed to the front. R keeps its order, because of what step 7 measured: its phases are
     written for the generic steps between them.
7. **G.**
   - Merge the reruns: `BoolRet` once, and possibly 168 into `GotoTail`.
   - Move each generic step to the end.
   - **Measured, and not done: none of the moves holds the bytes.** Each was
     tried on the chain from the step's own position to the product.
     - **120's unions, moved to the generic tail.** Phase 137 refuses: it
       names `b_ct_di.di_tv.v_number`, the field as 120 leaves it, 18
       times.
     - **134's empty blocks.** Phase 145 refuses: the labelled block it
       takes is the one 134's fold leaves.
     - **149's never-null calls.** The chain completes, but 34 lines
       longer. The phases between 149 and 165 are written for its folds.
     - **`BoolRet` once.** 166 run beside 183 and 184 leaves functions
       `int` that the three runs make `bool`. The lines are the same, but
       120 of them differ: 166's text is the one 167-182 were measured on.
     - **The attributes (107) at the end.** Not tried: the steps they
       would have to follow (120, 134, 149) cannot move. They went to the
       seed instead (below).
   - **`NullptrUsize` (106) to the seed instead, done.** Phase 0 runs
     `whim106 --casts 1` on the canonical input, so every phase is written
     in C23's spelling, the product's. At the seed it renames 7,617 `NULL`
     and 778 `size_t` (346 casts, 432 declarations) and drops 32
     `(void *)` casts, where q107 held 2,211 `nullptr` and 416 `usize`
     after 106; the three literals holding `NULL` are the same, and phase
     0's sweep and print take 219 lines. The respelling: 282 lines -- 109 in `internal/cut` (33 files), 165 in the
     programs of 26 phases (56-105), 6 of phase 98's `musl-ctype.md` and
     vimtext's two buffer walks; every one a literal, an anchor or text
     inserted. One anchor moved rather than respelled: phase 97's
     definitions follow the `usize` typedef, not the last `#include`. The
     fall-out closure names neither spelling, and no count moved. The chain
     gives the product byte for byte. In order: 124 phases, 815 s (the seed
     10 s, phases 1-3 40, 43 and 45 s); the parallel check: 123 links, 83 s.
   - **The attributes (107) to the seed too, done.** Phase 0 runs
     `whim107` after `whim106`. At the seed `Attrs` finds 358 attributes
     (q106 held 139): 306 `unused`, 37 `fallthrough`, 9 `format`, 3
     `format_arg` and 3 `cold`. It refused there twice, and learned: `[[`
     occurs twice inside one shell-command literal, so it is counted
     outside the literals; 3 `unused` are on locals of `buf_write`, each
     declaration a statement of its own, and go too; and the 3 `cold`, on
     105's wrappers' prototypes beside a `format`, are kept with it. It
     deletes 306 `unused` (303 on 265 definitions' parameters), respells 37
     `fallthrough`, keeps 15, and changes 305 lines within themselves. The
     canonical print writes `[[fallthrough]];` as `;`, so the phases read an
     empty statement where they read the GNU spelling. The respelling: 14
     lines -- 3 cutters of `internal/cut` (`nomouse`, `nobuflist`,
     `nocomplkeys`: the fallthrough as `;`) and the programs of 64, 67, 96,
     100 and 103 (2 fallthroughs, and 11 `unused` in anchors and in the
     host functions 103 inserts). No count moved. 105 is a block of its own,
     `g01-variadics`, under its own number. The chain gives the product byte
     for byte. In order: 124 phases, 820 s (the seed 11 s, phases 1-3 40,
     43 and 45 s); the parallel check: 123 links, 84 s.
   - **The variadic collapse (105) to the seed, done.** Phase 0 runs
     `whim105` between `whim106` and `whim107`, and 105 has no plan entry:
     the 105-107 group is gone, and with it the block `g01` (38 blocks).
     At the seed the seven wrappers have 297 call sites where q104 had 129
     (smsg 40, smsg_attr 2, smsg_attr_keep 1, semsg 212, siemsg 13,
     vim_snprintf_add 2, vim_snprintf_safelen 27): 244 statements, 53 in
     value position; `va_start` goes 8 -> 1 there too, and phase 0 is 3 s
     longer. Two changes to the program: 7 sites pass no variadic argument,
     each a literal format without `%`, which `vim_snprintf` copies as it
     is, and the rule now takes them (it refused any); and the append's
     length is `strlen`, which phase 97 renames with every other. Not only
     spelling: the expansion writes each message format twice, and the
     phases before 105 were written for one call. Yet what moved is small,
     measured on the chain: 6 lines respelled -- phase 74's inserted body
     (two `semsg` calls, as 105 expands them), 93's two literals and 95's
     anchor on `fileinfo`'s safelen, now closed by one more `)` -- and 4 of
     phase 104's counts re-measured: `printf` 13 -> 7 before and 10 -> 4
     after (the six wrapper prototypes' `format(printf, ...)` are gone),
     `vim_snprintf` 73 -> 201 and `musl_strlen` 134 -> 135 after it. The
     chain gives the product byte for byte. In order: 123 phases, 815 s
     (the seed 14 s, phases 1-3 40, 43 and 44 s); the parallel check: 122
     links, 88 s.
   - **The key names (167) to the seed, measured and not done.** Run on
     the whole seed (there is no core/host line yet), `whim167` names 155
     codes at 959 sites (683 at 167), every one by the name the product
     gives it: the table, `K_X` and the mechanical names agree. But the
     definitions go before the declaration that holds the first use, and at
     the seed that is `wildmenu_translate_key`, which a later phase cuts,
     not `edit()`; and they come in the order of first use, which puts
     `K_UP`, `K_LEFT`, `K_DOWN`, `K_RIGHT` and `K_KENTER` first and moves
     `K_BS`. The sweep cuts but never reorders, so the product's 153
     definitions would come in another order and place. The anchors that
     spell a code as arithmetic are few -- 6 lines: `nomouse`'s `ke()` (20
     uses) and its mouse pattern, `nocomplkeys`, `nowildmenu`, 59's `ke()`
     and 67 -- but the move is not one of spelling.
   - **Plain C (182) to the seed, measured and not done.** Its three steps
     are the core's: at the seed the `#include`s are the first 41 lines
     and there is no core yet (phase 110 draws the line), so a seed step
     would see the whole file. `Identity` there would unwrap the host's
     `_()` calls too, and the product's host keeps 42 of them; `ConstBranch`
     takes branches whole, which is not spelling. `AsciiClass`, the one
     spelling part, was tried: on the whole seed it names 247 tests (119
     digit, 62 lower, 66 upper; 136 at 182) and puts the three functions
     below the `#include`s, where after 110 they are the core's top as in
     the product. With anchors and inserted texts respelled in 6 files (74's
     two, 79, 97's and 98's musl functions, `extable`) and `LibcOwn` (114) told that
     static functions above the library declarations are not their end,
     the chain completes, but 19 lines differ from the product: 12 tests in
     the host's formatter, which 182 never touches because it runs on the
     core alone, are named; and 166's `BoolRet` leaves 5 functions `int`
     that it made `bool` -- `musl_isdigit`, `_isupper`, `_islower`,
     `vim_islower`, `vim_isupper` -- because their return is now a call,
     not a comparison. So 182 stays where it is.
   - **168 into `GotoTail`, done.** With 168's program removed, 170 takes
     its 19 gotos as tails of no statements, and the chain gives the product
     byte for byte. `GotoTail` now takes 98 gotos and drops 21 labels. 168
     is a record. `crefactor/xform`'s `GotoReturn` is gone too: its test runs
     `GotoTail` with a tail of no statements and gives the same text. In order: 124 phases, 789 s; the parallel check:
     123 links, 119 s.
   - So the generic steps stay where the rewires need them. The tail
     164-184 is generic but for 174-180, the regex engine's rewire (R10),
     which rests on 175 and 176 as §5 foresaw.
8. **Renumber and archive.**
   - **Done, in the shape the measurements left.** The phases kept their
     numbers: the snapshots (`qNNN.c`), `--from`, `--to` and some 4,200
     citations name them. The numbering the review asked for is a label on
     each block instead:
     - `crefactor/pipeline`'s `Phase` has a `Block`, the block a phase
       opens, which the log prints as a heading;
     - the plan names 39 blocks, from `s00-seed` and `d01-front` to
       `g11-bool`;
     - `doc/GOALS.md`, *The pipeline as it runs*, indexes them.
   - **Why the kinds interleave.** D1-D12 are phase 1's front. The drops
     that stay (`d02`-`d13`) count text only their predecessors leave, and
     the generic steps that stay in the middle (`g01`-`g04`) are what the
     rewires after them were written for.
   - **The archive.** The 39 phases that edit nothing are records, in
     `internal/phase/archive/NNN/`, with their old numbers, and the 122
     citations of their paths point there. A program the front calls (56,
     61) keeps its directory.
   - **The front in three phases (after the reform, to speed the check).**
     Phase 1 had grown to 89 s and bound the parallel check. Profiled, the
     cuts took 48 s, spread across them as regexp scans of a 4 MB text not
     yet swept, and the closure 31 s, with GC 28% of all CPU. No single
     cut dominates, and `GOGC=800` saves 10% at 3 GB a phase.
     - **The split.** Phase 1 runs D1-D5 and its closure, phase 2 D6-D8
       and phase 3 D9-D12, each part with its own closure, ahead of the
       steps those phases had. They take 40, 43 and 45 s.
     - **Re-counted.** Each part sees the text the previous phase swept, so
       the counts restated for the unswept front move again: `setmouse()`
       32, `need_maketitle` 6, `'scrollbind'` 14 again. `notabs` folds the
       test the closure left.
     - **The same product.** The chain gives it byte for byte. In order:
       124 phases, 815 s (36 more, for two more closures). The parallel
       check: 123 links, 75 s (122 before), bound by the machine's load
       again; the parts take 40, 43 and 47 s.
   - **Result.** The chain gives the committed `whim-vim.c` byte for byte,
     under its 39 block headings. In order: 124 phases, 779 s. The parallel
     check: 123 links, 122 s; phase 1 89 s.
   - The new phases go in a fresh directory and a fresh numbering (see
     question 1).
   - The old `internal/phase/NNN/` records go to an archive.
   - `GOALS.md` indexes both.
   - CLAUDE.md is rewritten to the new shape.

9. **The in-order build, profiled and made faster (after the reform).**
   `make whim-build-check`'s first run -- `whim build --check` from an empty
   `.cache/boundaries` -- runs the 123 phases in order, and it is what a
   moved upstream costs. `whim build --cpuprofile F` profiles a run.
   - **The profile, at 000dbf0.** 815 s wall; 1,335 s of CPU (1,210 user,
     125 system), 1,274 s sampled. The categories overlap (a parse inside the
     sweep is both):
     - the collector's mark workers 430 s, a third of all CPU;
     - the steps 307 s: the edits (`runEdit`) 124, the front's closures
       (`FallOutOf`) 91 (`fallOutMarked` 35, `cc.Translate` 30), the plain
       cutters 50, `droplocal` 44;
     - the sweep 217 s: its parse 103, `collect` 78, `unusedLocals` 30;
     - the canonical print 133 s: its parse 67, the expansions walk 65;
     - `cc.Parse` 178 s across the sweep and the print, and the parser 204
       counting `Translate`'s;
     - the tree walks by reflection 169 s (`sweep.walk` 115, `walkTok` 54),
       the print's own reflective walk inside its 65;
     - regexp 194 s, the largest users `DropLocal` 44 and `MentionCount` 31;
     - gcc: phase 110's eight compiles of the cut (about 3.4 s each, wall,
       outside the profile) and phase 169's 33 one-by-one header trials (11
       of its 13 s); process and temp files under 2 s.
   - **Done**, each a commit, each measured A/B/A/B on a run of phases from
     its snapshot, by CPU, with the boundary byte for byte:
     - the sweep's walks by a generated type switch (`crefactor/ccwalk`,
       `go generate`), reflection kept for a type it was not generated for:
       phases 120-140, CPU -9.4%;
     - the sweep in memory, and its last parse handed to the canonical print
       when that round cut nothing (`sweep.PruneParsed`,
       `cemit.CanonicalParsed`): -9.5%;
     - a run in order at GOGC 400 with a 3 GiB soft limit (`Run`; the
       parallel check is untouched, and GOGC or GOMEMLIMIT in the
       environment wins): phases 1-3, CPU 201-209 -> 131 s, peak resident
       0.9 -> 2.0-2.2 GB; phases 120-140, -35%;
     - `DropLocal`'s regexps on the lines that hold the field and not the
       whole text: phases 16-32, CPU 61-63 -> 41-42 s;
     - `MentionCount` by `bytes.Index` for a name of word characters, and
       `FindDefinition` counting depth from candidate to candidate instead of
       an int for every byte: phases 80-104, CPU 81 -> 61 s;
     - the canonical print's token pass by the generated walk: phases
       120-140, -7.5%;
     - phase 169's one-by-one fold 16 trials at a time, speculatively, the
       answers taken in order to the first header that stays: 13.5 -> 4.4-5.3
       s wall, for 5-11 s more CPU.
   - **The result.** In order: 123 phases, 506 and 516 s wall (load 5-6), 651
     and 670 s of CPU, peak resident 2.0-2.3 GB (0.8 before); the seed 8-9 s,
     phases 1-3 31-32, 34 and 36-38 s, 110 28 s, 169 4 s. The parallel check
     on the same snapshots, A/B/A/B under a load of 17-22: 97 and 101 s wall
     before, 77 and 71 s after; CPU 2,059 and 1,961 s before, 1,523 and 1,492
     after. Both give `whim-vim.c` byte for byte.
   - **Declined.**
     - A sweep inside the front before its closures: the cuts assert counts
       on the unswept text, and it moved them before (step 8); the closures
       are now 75 s of CPU across the run, and their parses 25 s.
     - Reusing a parse across steps: an edit's text moves between any two
       parses but the sweep's last round and the print's, which is the pair
       now shared.
     - Phase 110's `gcc -c` as `-fsyntax-only` (3.4 -> 0.3 s a compile):
       measured, the diagnostics differ (`-Wimplicit-fallthrough` and the
       other flow warnings need the middle end), and the phase reads them.
     - `cc`'s own allocation (`tokens2CppTokens`, `fset.Position`, 18 s):
       the front end is a fork kept diffable against upstream.
     - The rest of regexp (123 s): spread across dozens of edits and cutters,
       the largest 15 s (`ReplacePattern`).

10. **The drops that stayed, brought to the front (after the reform).** Step
   6 left `onebuffer`, `lfonly`, `keepbytes`, `noconv` and the programs of
   57-59, 63-68, 71-76 and 85 where they were. Each was tried again on the
   front's text, from the snapshot before it to the product, with the chain
   harness of D1-D12 (resume, next, diag).
   - **The editing features, done (branch `drops-front`).** Phase 3's
     part calls the programs of 57 (lisp), 58 (language mappings), 63 (the
     jump list), 65 (rot13 and the operator function), 66 (sentences and
     paragraphs) and 74 (file marks) by name after `noabbr`, under its
     closure. They applied to the front's text as written. Phase 6 runs
     two steps first, on the text phase 3 swept: 76's program, whose proof
     asks that `nfa_regengine` is named by nothing (it refused inside the
     front, where `nonfa`'s engine is not yet swept), and the `droplocal`
     of 57's and 58's four fields, whose readers phase 3 took. 57, 58, 63,
     65, 66, 74 and 76 are records, their programs kept where they are.
     - **What it replaced.** Phase 62's `getfile()` and `cleanup_jumplist()`
       edits and phase 70's `fname2fnum()` body: those functions go at phase
       3 with the file-mark jump and `:jumps`.
     - **Held.** `whim66` takes the last writers of `listcmd_busy` (the `'{`
       and `'(` addresses saved and set it); the product keeps it, and
       without the hold the closure folds it out of `setpcmark()`, two lines
       short.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       In order: 116 phases, 490 s (634 s of CPU); phases 1-3 33, 34 and 37
       s, phase 6 7 s. The parallel check: 115 links, 74 s.
   - **One buffer, one window, completion, and the terminal's diagnosis,
     done (same branch).** Phase 6 runs first, on the text phase 3 swept:
     42's `onebuffer`, a sweep and the `droplocal` of `b_p_bh`, then the
     programs of 67 (mouse and spell plumbing), 68 (one window,
     structurally), 71 (one buffer, structurally) and 85 (the tty
     diagnosis); after its own two cuts, 59's (command-line completion),
     written against `nowildmenu`'s `getcmdline_int()`. 42 is a record in
     the archive; 59, 67, 68, 71 and 85 keep their programs.
     - **Why phase 6 and not the front.** Inside phase 3's part
       `onebuffer` refused: `do_exedit()` and the rest of what the front's
       cuts leave for its sweep still name `buf_hide()` (5 mentions, not 2),
       `w_alt_fnum` (8, not 2) and the two `CMOD_` flags (7, not 2). On swept
       text it and the four programs apply as written, so nothing is
       restated.
     - **What it replaced.** Every edit inside what they now take first:
       - 68 takes the autocommand window's switch, and with it `nochdir`'s
         (22) `w_localdir` restore and `globaldir`'s save and restore in
         `aucmd_prepbuf()` and `aucmd_restbuf()`;
       - 71's body for `buflist_findpat()` supersedes 62's `'buflisted'`
         literal there;
       - 59 takes every completion context but files, and with them
         `noinertopts`' (16) `'tags'` backslash rule, `nohome`'s (20)
         `~user` row and context, `nogetenv`'s (20) `$PATH` in
         `expand_shellcmd()` and `$VAR` row and context, `nofloat`'s (23)
         fuzzy-matcher rounding, the completion of `:retab` (44),
         `:noswapfile` (48), `:setglobal` and `:setlocal` (`oneoptset`, 49),
         and `noconv`'s (53) `++` argument completion.
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       In order: 110 phases, 473 s (614 s of CPU); phases 1-3 31, 33 and 39
       s, phase 6 15 s. The parallel check: 109 links, 78 s.
   - **The encoding, the formatting, one window and one frame, and the
     autocommands, done (same branch).** Phase 7 runs first, on the text
     phase 6 swept: 25's `nobackup`, 50's `lfonly`, 51-53's `keepbytes` and
     `noconv`, 64's program, 72's and 73's, a sweep, the `droplocal` of the
     fields of 50, 53 and 64, and 75's program. Phase 6 runs 13's `nostat`
     first. 13 and 50-53 are records in the archive; 64, 72, 73 and 75 keep
     their programs; 25 keeps `noowner` and its `droplocal`. The blocks
     label it: `d01-front` runs to phase 7, `d02-outside` opens at 14.
     - **Why `lfonly` could move after all.** It counts and anchors on text
       where `readfile()` is gone; at the front that was phase 13's
       `nostat`, so it seemed every readfile edit would have to come back.
       Bringing `nostat` to phase 6 instead, and `lfonly` to phase 7, gives it
       that text: `readfile()` and `mch_call_shell_fork()` (phase 6's
       `nowild`) are swept by then, and it applies as written. `keepbytes`
       and `noconv` follow it; `noconv` needs `nobackup` first (its
       `if (!converted || dobackup)`), so that comes too.
     - **Restated, with a comment each.**
       - 64 cuts the format operator's case and the filter and indent
         dispatch with phase 60's `'formatprg'` and `'equalprg'` tests still
         in them; 60's two folds there go.
       - 75's `open_buffer()` literal keeps `do_modelines(0)`, which
         `oneoptset` (49) takes after it, at the new depth; its bare
         dispatches are 43, not 25: buf_write()'s 8, set_rw_fname()'s 4,
         set_buflisted()'s and enter_buffer()'s 2 each, do_ecmd()'s third
         and do_filetype_autocmd()'s, which phases 8-74 took first.
       - `noowner` (25) finds its mode mask by its condition alone, since
         `nobackup`'s block around it is printed one level shallower.
       - 62 counts one `set_buflisted()` (the help buffer's), 69 three
         `check_arg_idx()` calls, and 78 twelve empty functions:
         `create_windows()`'s body is 72's from phase 7, and with its
         `setfname()` gone, `buf_name_changed()`, `ml_setname()` and
         `set_b0_dir_flag()` go before 62.
     - **What it replaced.** `nogetenv`'s `'backupskip'` loop (20, with
       `nobackup`), `nochdir`'s `win_fix_current_dir()` call (22, with
       `win_enter_ext()`), `oneoptset`'s `set_options_bin()` edit (49, with
       `lfonly`), 69's `win_init_some()`, `create_windows()` and
       `win_alloc_firstwin()` edits and 70's `create_windows()` literal (with
       72's bodies), and 60's `match_file_pat()` and `verbose_enter()` and
       `verbose_leave()` edits (with 75).
     - **Result.** The chain gives the committed `whim-vim.c` byte for byte.
       Every drop the reform left is at the front now. In order: 104
       phases, 439 s (572 s of CPU); phases 1-3 30, 33 and 37 s, phase 6 14
       s, phase 7 10 s. The parallel check: 103 links, 76-77 s.

11. **The in-order build, profiled again: regexp and phase 110.** Step 9
   left two named targets, the regexps spread across the edits and phase
   110's gcc.
   - **The profile, at db4f488.** `whim build --check` from an empty
     `.cache/boundaries`: 518 s wall, 667 s of CPU (613 user, 54 system),
     602 s sampled. Regexp 128 s, attributed by the outermost regexp frame's
     caller and by the phase program or cutter above it:
     - by caller: `ReplacePattern` 14.8 (every `E.Sub`, `E.Cut` and the
       cutters' `subCount`), the sweep's identifier scan 14.6 (`identRe` on
       every definition, every round), the folds 5.7, `EmptyBlocksFold` 4.6,
       `CountIs` 3.3, `cutCounted` 2.7, the fall-out's index 2.6 and its
       declaration 2.0, `Query` 2.5, `replaceFirst` 2.1, `CallsNotAfterWord`
       2.1, `DeadStores` 1.7, and the phases' own mention counters (128 2.8,
       125 2.2, 94, 96 and 126 2.0 each, 104 1.3) and the cutters' (NoTags,
       NoFenc 2.4 each, NoLocale, Utf8Only 1.9 each);
     - by program: 79 5.7, 78 4.7, 128 4.7, 125 3.5, 126 3.3, 80 3.2, 67
       3.1, Utf8Only 2.7, NoMouse, NoFenc, NoTags 2.5 each; 32 s in no
       program at all (the sweep and the generic steps);
     - by pattern, three shapes: a line or two anchored at a line's start,
       `(?m)^[ \t]*if \(p_xyz\)\n`, which Go's regexp cannot skip ahead
       through -- it skips only by a literal PREFIX -- so it ran its machine
       over the whole 2-7 MB text; `\bname\b` compiled for one name and run
       over the whole text to count it (the `\b` hides the prefix too); and
       FindAll to count followed by ReplaceAll to rewrite, two passes.
   - **Done**, each a commit, each measured A/B/A/B on a run of phases from
     its snapshot, by CPU, with the boundary byte for byte:
     - the sweep's identifier scan by hand (`identSpans`): phases 120-140
       and 1-3, a percent or two, inside the noise;
     - `crefactor/edit/lines.go`: a pattern's syntax tree gives a literal
       every match holds and the most newlines a match spans, k, and the
       counted acts run the pattern only on WINDOWS -- each occurrence's line
       and k+1 lines either side, merged, the extra line being the context
       its `^`, `$` and `\b` see -- with `ReplacePattern` finding its matches
       once; no literal of two bytes, a newline under a star, `\A`, `\z`:
       the whole text, as before. Phases 77-104, CPU 81.6 and 82.9 -> 70.9
       and 68.7 s;
     - phase 110's eight compiles of the cut -- see below: 30.0 and 29.8 ->
       12.7 and 12.4 s of CPU, 29.8 -> 11 s wall;
     - the generic steps: `EmptyBlocksFold` on the lines around empty
       blocks (`AllSubmatchIndexAround`), the fall-out's enumerators asked
       `bytes.Contains` first, `DeadStores` counting a name by
       `strings.Index` (`WordCount`), `NeverNullRule` and `unions` windowed:
       phase 134 9.6 and 9.0 -> 3.6 and 3.2 s, phases 1-3 -3%;
     - the cutters and the phase programs: the mention counters as
       `edit.WordPatternCount`, the FindAll-then-ReplaceAll pairs as
       `ReplaceAllCounted`/`ReplaceAllLiteralCounted`: phases 1-3 114.6 and
       112.4 -> 99.9 and 101.3 s (-11%), phases 93-128 100.9 and 103.5 ->
       89.2 and 88.0 s (-13%);
     - `Blank` copies the text and jumps between quotes and slashes by
       `bytes.IndexByte` (9.1 -> 1.75 ms on slim-vim.c): a percent or two;
     - `CallsNotAfterWord` by `bytes.Index`, and a pattern with no window
       and no empty match replaced in one pass: inside the noise (the
       profile's 4 s).
   - **How each was proved the same.** A test holds each new path to the
     regexp it replaced on slim-vim.c and whim-vim.c (`TestIdentSpans`,
     `TestScoped`, `TestScopeOf`, `TestEmptyBraces`, `TestWordCount`,
     `TestDeadStoresSame` -- the old `DeadStores` kept in the test, on both
     files and on them with every line that returns or calls taken out, 57
     and 512 locals taken --, `TestBlankSame`, `TestCallsNotAfterWord`), and
     `internal/build`'s `TestScopedPatterns` runs the 837 distinct patterns
     the edits handed the windowed matchers in a whole check
     (`testdata/edit-patterns.md`) on both files both ways. And twice a
     temporary hook, not kept, ran every call of a parallel check both ways
     on the real boundaries -- 933,424 calls the first time -- and found no
     difference.
   - **Phase 110.** Its eight compiles: the move alone (`cut0.c`: the
     twelve names the cut asks for), six rounds of the fixpoint (each moves
     what the last called unused) and the finished cut, which is the last
     round's text. What it reads of gcc is the errors, `undeclared`,
     `unknown type name`, `defined but not used` and `used but never
     defined`; the last two are the call graph's, which `-fsyntax-only`
     never builds (it gives none of them, measured again). `-flto
     -fno-fat-lto-objects` stops right after the call graph and writes no
     code: on the eight cuts of a run gcc's whole stderr is byte for byte a
     plain `-c`'s, flow warnings and all, 1.4 s a compile against 3.5. The
     finished cut's compile is the last round's answer, kept by its text;
     the first round's is started beside the move's, which it does not
     need. The phase's report is line for line what it was. 28-29 s -> 10-11
     s, in order.
   - **The result.** Regexp 128 -> 41 s at 3086efd's parent, the largest
     left 2.3 s. In order, A/B/A/B under a load of 52-57 (other agents'):
     676 and 685 s wall before, 503 and 525 after; CPU 787 and 799 before,
     617 and 642 after; peak resident 2.0-2.3 GB either way. Phases 1-3 39-41,
     51-54 and 43-47 s before, 32-33, 22-29 and 30-34 after; 110 34-35 -> 13-14.
     Profiled at a lighter load, before the last commit: 405 s wall, 553 s
     of CPU, against the 518 and 667 the step began from. The proof: `rm -rf
     .cache/boundaries`, then `whim build --check` in order, 525 s wall and
     642 s of CPU (load 55), and again on its snapshots in parallel, 106 s;
     both end `whim-vim.c byte for byte`. The parallel check A/B/A/B under a
     load of 72-75: 115 and 124 s wall before, 109 and 108 after; CPU 1,264
     and 1,327 before, 1,064 and 1,025 after.
   - **Declined.**
     - A literal for a pattern with a newline under a star (`\s*`,
       `[^;]*`, `(?:[^\n]*\n)*?`): 26 of the 382 patterns run on a whole
       text, each about once. Their matches can start anywhere above the
       literal; a window would have to run to the text's end.
     - The rarest of several literals, or a set of them for an alternation
       (`\b(MIN|MAX)\(`): phase 109's 0.6 s, and every other a few tenths.
     - Rewriting a pattern to make it windowable (`\s` as `[ \t]`): it
       would change what the pattern says, and so what it could match.
     - Phase 110's rounds side by side: each is the previous one's answer.
     - The rest, about 37 s of regexp, nothing over 2 s: and in the files
       the front's own move was rewriting at the time (`lfonly`'s `\bname\(`
       per name, 1.0 s; `noconv`'s `\bptr\b`, 0.6; phase 66's rows written
       `[^}]*`, which no window can take, 0.5).
     - The parser (`cc.Parse`, 145 s across the sweep and the print) and the
       collector are most of what is left; the parser is the fork kept
       diffable against upstream (step 9).

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
