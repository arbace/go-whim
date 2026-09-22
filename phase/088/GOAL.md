# Phase 88 — argv is `+{command}` and `-T {term}`

`phase/088/edit.go` and `phase/088/check.go`, `stage 88`, `package streams`. A
core is handed its buffer by a host, not by a shell. What phases 85 and 87 left of
`command_line_scan()` is five things — `+cmd`, `-T`, a bare `-`, `--` and a file
argument — and this phase takes the last three, which are exactly the three that
name a **file** or a **stream** to edit. Everything the parser does not recognise
is now what every other unknown word already was, `mainerr(ME_UNKNOWN_OPTION)`.

**The file-argument arm is replaced, not deleted**, and that is not tidiness: with
no `else` at all a bare word matches neither `+` nor `-`, `argv[0][argv_idx]` is
not NUL for any word of more than one character, and the `while` never advances.
Deleting it gives an infinite loop, not an error. What goes with it is
`parmp->edit_type = EDIT_FILE`, the `vim_strsave()` and the `buflist_add()` that
put the name in the buffer list. `case NUL` — the bare `-`, with `EDIT_STDIN` and
`read_cmd_fd = 2` — and `case '-'` — where `--` set `had_minmin` and made every
later word a file name — fall to `default:` instead. `--foo` reached
ME_UNKNOWN_OPTION from *inside* `case '-'` and reaches it from `default:` now, so
only `--` itself changes.

**`ME_TOO_MANY_ARGS` had exactly those two call sites**, so it goes with its row in
`main_errors[]` — and **the enumerator is the row index**, so the three after it
move down by one. That is the renumbering `CLAUDE.md` warns about, done on purpose:
the table and the enum are rewritten from **one parse of both**, which is what
makes the mapping a fact rather than two edits that agree, and the check compares
the DWARF enumerator values of the binary the phase was handed with the ones it
made. Measured: **1,327 in, 1,323 out** — `ME_TOO_MANY_ARGS` and the three `EDIT_*`
gone, `ME_ARG_MISSING` 2→1, `ME_GARBAGE` 3→2, `ME_EXTRA_CMD` 4→3, and **1,320
unmoved**. A wrong index here shows up nowhere else: the build is perfectly happy
with it, and `-Txterm` would simply start printing another message.

`main_errors[]` keeps a **sixth** row, `"Invalid argument for"`, which no
enumerator named before this phase either. It is whim's leftover, and this phase
removes the row an enumerator it removes points at and nothing else.

**What `params.edit_type` then is: EDIT_NONE, for ever**, because nothing assigns
it. Its two readers are in `vim_main2()` and their polarity is opposite — `==
EDIT_STDIN` folds never, taking `read_stdin()`'s only call with it, and `!=
EDIT_STDIN` folds always, leaving `newline_on_exit` under the two conditions that
were already there. The field, the three `EDIT_*` enumerators, `read_stdin()` and
`buflist_add()` are then what the **sweep** takes: `deadfields.py` for the field —
there is no `ml_recover()` in this file, so a struct is no longer a disk format —
`deadenums.py` for the enumerators, and `deadsweep.py` for the two functions. Two
rounds, 79 lines.

## Where the line is against the later phases

Three things this phase could have taken and did not, each asserted by a **count**
so that taking them would fail here rather than widen quietly:

* **`readfile()`'s stdin half** is the "nothing reads a byte" phase's
  (`GOALS.md` II.3b P8). What goes here is the *function* `read_stdin()`, argv's
  entry point into that code; the 23 remaining mentions of the name are the
  **parameter** of `readfile()`, `read_buffer()` and `open_buffer()`, and the check
  requires exactly 23.
* **`read_cmd_fd`** keeps its definition and its twelve remaining mentions. Only
  the assignment was argv's; nothing writes it now, so it is 0 for ever and folding
  it belongs with stdin. A file-scope static that is read and never written draws
  no warning, so the sweep would not have touched it either way.
* **The buffer's name** is P9's. Nothing here touches `b_ffname`, `b_sfname` or
  `b_fname`: what goes is the one call that ever gave the startup buffer a name
  from argv. `create_windows()` already opens an unnamed buffer when argv named
  none — that is `tools/zargv.py`'s `(none)` row — so the startup path is the one
  that was always there.

**There is no `usage()` to leave alone.** A phase that removes options usually owes
the help text an apology; `grep -i usage whim-vim.c` finds nothing at all, whim
having removed it, and `--help` is already `Unknown option argument: "--help"` in
the baselines.

## The declared delta: six command lines, and nothing else

`argv:-`, `argv:--`, `argv:f.txt`, `argv:f.txt_g.txt`, `argv:+q!_f.txt` and
`argv:--_+q!` — six of `tools/zargv.py`'s 30 rows, every one a way of naming a file
or a stream:

* **`-` was the row that blocked.** The editor read the keystroke file itself as
  buffer text, closed fd 0, duped stderr and waited there for keys that never came;
  the baseline record is `blocked` and it cost the harness its timeout on every
  run. It is `Unknown option argument: "-"`, exit 1, now.
* **`f.txt` and `+q! f.txt`** opened a buffer and drew a screen; both are exit 1
  with an empty stream.
* **`--` and `-- +q!` disagreed with each other** in the baselines, because `+q!`
  after `--` was a *file name* rather than a command. They agree now, and that
  difference is the whole of what `--` did.
* **`f.txt g.txt` is declared although `stderr-moved` would absorb it**: it exited
  1 with an empty stream for `Too many edit arguments: "g.txt"` and exits 1 with an
  empty stream for `Unknown option argument: "f.txt"`. It is the two-file-argument
  row and this is the phase that removes file arguments, so the list says so rather
  than letting a dimension cover it — phase 87's reason for naming `-e`.

**Nothing else moves, and the corpus is the reason it cannot**: every one of the
102 screen cases seeds itself by *typing* under `'paste'`, so not one passes a file
argument. Measured: 102/102 cases, 111/111 Ex-command rows, the four pty scenarios
and all 24 other command lines identical — including every `+{command}` form and
all three `-T` spellings.

## The instrument this phase broke, and the one that replaced it

**`tools/termcheck.py` asks its question with a file argument.** It is whim's and
slim's, the one harness zero kept (phase 86), and it opens a three-line `f.txt` so
that the screen has something on it before `:set term? t_Co?`. From this boundary
that argument is an unknown option, the editor exits 1 before drawing, and **all
nineteen rows read `(none)`** — measured. That is the harness failing, not the
terminal table moving, and declaring `term-moved` for it would switch the terminal
table off for every phase after this one, which is the one thing a phase must not
buy its way out with.

So Part II's recording now uses **`tools/ztermcheck.py`**: `termcheck.py` imported
with its `ask()` replaced and nothing else, so the terminal list, the environment
isolation, the settle ladder and the output format stay in one place and cannot
drift from whim's. Editing `termcheck.py` itself is what core rule 9 forbids — it is
named by `tools/whimdelta.sh` and `tools/verify.sh`, so its bytes are in every whim
stage's key. **The swap is proven, not asserted**, in three places: the new tool
records `.reference/core-baselines/ref-term.txt` byte for byte from the binary this
phase was *handed* (which still accepts a file argument, so both forms work on it),
the old tool records nineteen `(none)` rows from the one it *made*, and phase 83 —
which records from `whim-vim.c` three times and compares with the baselines —
reproduces `q83` unchanged under the new instrument. Only `tools/zrecord.sh` changed
to name it, which re-keyed the five Part II phases before it and **no whim or slim key**:
all 107 are identical to `main`'s.

## The probes, and why the delta is not enough

The baselines are one recording of one binary, so `tools/coredelta.sh` can say
"exactly these six moved" and cannot say "the old binary opened the file". The
check says it, by running both — the binary the phase was **handed**, built by the
edit part from the boundary's own makefile flags, and the one it made. **22 probes,
six required to move and 16 required not to**, and each of the six is also required
to show the old behaviour on the *old* binary: a buffer drawn and exit 0 for
`f.txt`, `Too many edit arguments` for two of them, `blocked` for the bare `-`, and
`--` reading differently from `-- +q!`. Proven able to fail in both directions: run
with the old binary on both sides all six report *was to move and did not*, and
with the new binary on both sides they add *the input binary already refused it, so
this proves nothing*.

The 16 are `+`, `+q!`, `+set nu`, two `+{command}`s at once, **`+set paste`** with
typing under it — `'paste'` and `+cmd` are what the whole corpus is seeded with and
this is the phase that could have lost both — the three `-T` spellings and an
unknown terminal name, the four options that were already unknown, and an ordinary
keystroke edit. Two pty sessions beside them: `vim f.txt` on a real terminal, which
the old binary edits and the new one refuses with wait status 256, and an editing
session with no arguments, identical either side.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 85,813 | **85,734** (−79) |
| functions | 1,862 | 1,860 |
| enumerators (DWARF) | 1,327 | **1,323** |
| `nm -u`, the core's flags | 77 | **77**, the same set |
| `nm -u`, as `phasecheck.sh` counts it | 78 | 78 (the extra is `__stack_chk_fail`) |
| `.text` / `.rodata` of the plain object | 648,968 / 17,625 | 648,496 / 17,609 |
| binary | 861,288 | **861,288** |

**Nothing is freed, and that is the measurement rather than a disappointment**:
`read_stdin()`'s `close()` and `dup()` have other callers and `buflist_add()` names
no libc directly, so the undefined set is equal — stated as an equality, so a
symbol *arriving* would fail. The binary does not move either: 472 bytes of `.text`
and 16 of `.rodata` go, and alignment padding absorbs them, exactly as in phase 85.

The phase is **52 s** cold and 57 s under `make whim-verify`, which recomputes all
six boundaries in 80 s of wall time over 247 s of phases. A `make whim-repass` from
an empty zero cache ran the first five in **152 s** — 30, 12, 28, 31, 51 — and took
phase 88 from tier 3, so the whole pipeline cold is 204 s; every boundary matched
its recording. Its own is `d466c3b9245b`.

Its placement carries one new constraint, and it is measured rather than predicted:
**`apart 87 88`**. Phase 87's check names `EDIT_STDIN`, `read_cmd_fd = 2`,
`had_minmin`, `buflist_add` and `ME_TOO_MANY_ARGS` one by one and requires each to
be *there* — "it is the argv phase's to take" — and this phase takes all five. Run
on a phase 88 tree it says `'EDIT_STDIN' went, and it is the argv phase's to take`
and exits 1. Phase 85's check would fail on a phase 88 tree too, but a stage holding
85 and 88 holds 87, and `apart 85 87` already forbids that.

Two `uses` lines: `streams:88 seed:83 mechanical`, because the six declared records
are compared with the baselines phase 83 records, and `streams:88 harness:86
mechanical`, because an argv record is something a zero recording only has from
phase 86. Phase 87 is in the same package, so the ordering between them is the
package's and not a `uses`.
