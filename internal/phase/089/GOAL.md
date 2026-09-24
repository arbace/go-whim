# Phase 89 — no write

`internal/phase/089/edit.go` and `internal/phase/089/check.go`, `stage 89`, `package files`. A
core does not own a disk: reading and writing files is the host's business, and
this is the first half of taking the filesystem away. The six Ex commands that
put bytes on one go — `:write :wq :xit :exit :update :saveas` — and with them
everything only they reached.

## Four anchors, and not one fold

The phase is four edits, and every removal after them is the sweep's
(core rule 1). The `cmdnames[]` row is the only reference a command handler has, so
taking the row is what makes the handler unreachable:

1. the six enumerators of `enum CMD_index`, one line each;
2. the six `cmdnames[]` rows, one physical line each, designated `[CMD_x] = {`;
3. `nv_Zet`'s `ZZ`, which runs the command *string* `"x"` → `"q!"`;
4. `do_one_cmd`'s `:w>>` / `:w!` parse — an `if (ea.cmdidx == CMD_write ||
   ea.cmdidx == CMD_update) {…}` with no else — deleted as **text** rather than
   folded, because its condition names two of the enumerators that are going. It
   has to go in the same edit as anchor 1 or nothing declares what it reads.

**The alternative was measured.** An edit that also deletes `ex_write`,
`ex_update`, `ex_exit`, `do_write`, `check_writable`, `check_overwrite`,
`not_writing` and `check_readonly` by name produces a **byte-identical swept
file**, in 3 sweep rounds against 4 and 17 seconds against 22. Four seconds is
not a reason to write eight names into a phase program, so the minimal edit is
what runs.

**The text the edit leaves does not compile, and the program says so.** Six
mentions of the six enumerators survive it — `CMD_saveas` five times and
`CMD_wq` once — every one inside `ex_write`, `do_write` or `ex_exit`. The edit
asserts exactly that, as a computation rather than a list: each survivor is
inside a function definition, and no surviving `cmdnames[]` row names that
function, which is the whole argument that `funcreach.py` takes them in the
sweep's first round. `tools/phasecheck.sh` in the check is where *it compiles*
is asserted.

## `ZZ` is `ZQ`, and that is a decision

`nv_Zet` runs a command **string**, so nothing here breaks at compile time:
left alone, `ZZ` would type `:x` at a command that no longer exists and answer
`E492`. `case:zz_key` therefore moves whatever is done — `E32` today, `E492` if
the string is left, nothing at all with `"q!"` — so this phase owns it rather
than leaving a dead command named in the source. It is the user's settled
decision that ZZ is ZQ. `GOALS.md` II.3b gives it to the `:q` phase; that row is
annotated as built.

## No DWARF dump, and phase 88's reason for one does not apply

Deleting the six renumbers **89** survivors, and every one is a `CMD_*`.
Measured with `tools/enumvals.sh`: **1,323 enumerator values in, 1,303 out** —
the six, plus fourteen single-constant explicit-value enums the sweep takes with
their types (`CPO_FNAMEAPP CPO_FNAMEW CPO_FWRITE CPO_KEEPRO CPO_OVERNEW
CPO_PLUS NODE_NORMAL NODE_OTHER NODE_WRITABLE SHM_WRI SHM_WRITE SMALLBUFSIZE
TRUNC_ON_OPEN WRITEBUFSIZE`) — 89 moved and nothing else touched, nothing
arriving.

Phase 88 compared DWARF either side because `main_errors[]` was a table written
in its enumerators' order, where a wrong index was invisible to the build.
`cmdnames[]` is **designated**: a row lands at its own enumerator whatever the
numbering is, the `static_assert` on the row count catches a dropped pair, and
all 105 surviving names are dispatched by `tools/zexcmds.py` inside the declared
delta. Three checks the build cannot dodge, and none of them needs the values.

**The row floor now has five rows of margin.** `cmdnames[]` goes 111 → 105 and
`tools/create_cmdidxs.py`'s `names()` refuses a table of fewer than 100 — a
regex that stops matching otherwise yields a plausible all-zero index, so the
floor is deliberate. `tools/zexcmds.py` enumerates the table through it, so
crossing it would stop Part II's command sweep rather than give a wrong answer.
`GOALS.md` II.3a: the `:edit` phase spends the margin, and it is the phase that
must lower the floor.

## Two traps, and both make the obvious check the wrong one

- **`check_readonly` is also a local**, in `readfile()`: `int check_readonly;`
  and three uses. After the phase `grep -cw` is **4, not 0**, so a phase-4-style
  "every name at zero mentions" loop fails on a correct phase. What must be gone
  is the definition, `^check_readonly(`, and the four survivors are required to
  be inside `readfile()`.
- **`"write"` survives**, as the `'write'` option's name, and `E32: No file
  name` with it — still reachable through `check_fname()` from `do_ecmd()` and
  `ex_bang()`. A "no mention of write anywhere" check fails on a correct phase
  just as surely.

## The declared delta, and what the corpus cannot see

`6   case:cmd_write case:zz_key` and the six rows `write wq xit exit update
saveas`. The two kinds of movement are different: `cmd_write` goes from `E32: No
file name` to `E492: Not an editor command: write`, `zz_key` loses its bell,
its `E32` and two snapshots — and the six command rows **cease to exist**,
because `tools/zexcmds.py` enumerates 105 names where it enumerated 111.
Measured with `tools/zcompare.py`: the other 100 screen cases, the other 105
command rows, all 30 command lines, the four pty scenarios and the terminal
table are identical.

**And that is the whole of what any recording here can see.** Every one of the
102 screen cases types its own text and names no file, so `cmd_write` types
`:write` with no file name and what the baselines hold is an editor that
**failed** to write. "cmd_write and zz_key moved" is equally consistent with a
phase that changed one error message and left `buf_write()` reachable.

## The probes, which are the only evidence writing went

**26, on both binaries** — the one the phase was handed, built by the edit part
from the boundary's own makefile flags, and the one it made — **in a directory
they keep**. `tools/zstream.py`'s `session()` throws its run directory away,
which is the one thing a phase about files cannot do, so the runner is in the
check and adds one section to the record: what the run left on the disk.

- **`write_roundtrip`** types `WROTEME`, writes it to `out.txt`, empties the
  buffer and reads the file back. The old binary answers `"out.txt" 1L, 8B`; the
  new one `E484: Can't open file out.txt` and an empty buffer. It leans on
  `:read`, which the next phase removes — harmless, because `make whim-verify`
  runs every check on its own boundary.
- **`:w :sav :update :wq :x` with a file name**: `{'out.txt': 6}` on the old
  binary and `{}` on the new, for all five, with `:wq` and `:x` going exit 0 → 1.
- **Eight spellings** — `:w :x :wq :up :sav a :w! :w >>f :w !cat` — each E492
  now and none before, which is the inheritance check `CLAUDE.md`'s `:help` →
  `:helpclose` trap asks for.
- **Ten that must not move and do not**: `:q` on a modified buffer (still E37 —
  `check_changed` stays and is the `:q` phase's), `:q!`, `:read` (still E32, the
  read phase's), `:edit`, `:file`, `:%!sort`, `:s/x/y/`, `u`, CTRL-G and an
  ordinary edit.
- **A pty session**, because every probe above went through a pipe: `:wq
  out.txt` writes the file and quits on the old binary and answers E492 here,
  and an editing session is identical either side.

**Proven able to fail in both directions**: with the old binary on both sides
all sixteen report *was to move and did not*; with the new binary on both sides
they add *the input binary left {}, not a 6-byte out.txt, so this proves nothing
about writing*.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 85,734 | **84,675** (−1,059) |
| functions | 1,860 | 1,841 (−19) |
| enumerators (DWARF) | 1,323 | 1,303 |
| `cmdnames[]` rows | 111 | **105** |
| `nm -u`, the core's flags | 77 | **71** |
| `nm -u`, as `phasecheck.sh` counts it | 78 | **72** |
| `.text` / `.rodata` of the plain object | 648,496 / 17,609 | 640,449 / 17,289 |
| binary | 861,288 | **847,656** |

**The six that go are `chmod fchmod fstat ftruncate lstat unlink`**, and they
are the first any Part II phase has freed. The check states the set rather than a
count, and requires `stat`, `open`, `access`, `fsync` and `getcwd` to be
**still** undefined, so a cut reaching into a later phase fails here rather than
widening quietly: `stat` is down from 14 calls to 8 and belongs to the phase
that gives up the buffer's name, `open` and `access` to the one that stops
reading a byte, `fsync` to the options.

Nineteen functions go, none of them named by the edit — `ex_write ex_update
ex_exit do_write check_writable check_overwrite not_writing check_readonly
check_file_readonly buf_write buf_write_bytes check_mtime time_differs
write_eintr vim_fexists mch_setperm mch_fsetperm mch_nodetype u_update_save_nr`
— with one struct field, `exarg_T.append`, and 39 string literals. The sweep is
**4 rounds, 22 s**, and the phase **46–49 s**. Its boundary is `8ce685cf2592`,
and `make whim-verify` recomputes all seven in 80 s of wall time over 299 s of
phases.

**What no instrument here sees, said out loud.** `p_fs`, `p_write` and `p_wa`
lose their last readers and keep their rows and their `:set` answers — removing
a row is the options phase's, and `tools/orphanopts.py` refuses a global whose
row has gone, so they are asserted at exactly two mentions each. Six
`'cpoptions'` and two `'shortmess'` letters lose their readers with no
observable change, the validity lists being string literals. `'readonly'` keeps
its `W10` warning and its `[RO]` indicator. And the row floor now has five rows
of margin rather than eleven.

## Its placement

`stage 89`, `package files`, and two `uses` lines: `files:89 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records,
and `files:89 harness:86 mechanical`, because the old file-based sweep recorded an
exit status and `:write` went from E32 to E492 without changing it — phase 86's
message-level record is what can see this phase at all.

**`apart 88 89` is measured rather than predicted.** Phase 88's check states that
*it* frees no libc symbol, as a `cmp` against the stage's starting undefined
set, and inside a stage every check compares with the **stage's** start. Run as
one stage — `tools/phaserun.sh 88-89` — phase 88's check fails with *the libc
surface moved, and this phase frees nothing* and names all six.

**There is deliberately no `apart 85 89`.** Phase 85's check drives a pty with
`:wq` and reads the file back, which this phase would break — but measured, it
already fails identically on a phase **5** tree (status 256, the file
unchanged), because the session opens `f.txt` as a file *argument* and never
reaches the `:wq`. So the failure at 89 is phase 88's, a stage holding 85 and 89
holds 87, and `apart 85 87` forbids it already. Phase 87's check fails on a phase 89
tree for the reasons `apart 87 88` records.
