# Phase 90 — no read

`phase/090/edit.sh` and `phase/090/check.sh`, `stage 90`, `package files`. The
other half of taking the filesystem away. Phase 89 removed the six commands that put
bytes on a disk; this one removes the command that takes them off it on request —
`:read` — and with it the `:r !cmd` arm, which was the last caller of the filter and
shell plumbing whim left as stubs. What is left of reading a file is `readfile()`
itself, which the startup path still uses, and this phase asserts by count that it
is untouched.

## Three anchors, and one fold that is a judgement

1. the `CMD_read` enumerator of `enum CMD_index`, one line;
2. the `cmdnames[]` row, one physical line, designated `[CMD_read] = {`;
3. `do_one_cmd`'s `if (ea.cmdidx == CMD_read) {…}` — the parse that turns `:r!` and
   `:r !cmd` into a filter — deleted as **text** rather than folded, because its
   condition names the enumerator that is going, and in the same edit as anchor 1.

**No handler is named.** The row is the only reference a command handler has, so
taking the row is what makes `ex_read` unreachable, and `do_bang`, `do_shell`,
`do_filter`, `check_secure` and `prevcmd_is_set` follow it — `:!` has not existed
since whim, and phase 89 swept `ex_write`, which held `do_bang`'s other call.

**The fold is `exarg_T.usefilter`, and it is the one thing here no tool could have
found.** Phase 89 removed one of its two writers (`:w >>`, `:w !cmd`) and anchor 3
removes the other, so after the edit the field is **written nowhere** — and
`do_one_cmd` memsets the struct, so all seven readers are constantly FALSE.
`tools/deadfields.py` removes a field nothing *names*, and gcc has no warning for a
member that is only read, so neither would ever have reported it. The six
surviving tests are folded with their polarity stated — two `&& !ea.usefilter`
conjuncts and one `|| ea.usefilter` disjunct in `do_one_cmd`, a
`!eap->usefilter &&` and the whole `if (eap->usefilter && strpbrk(repl, "!"))` arm
and one more conjunct in `expand_filename` — and the field goes with them.
**Measured both ways**: the fold costs 13 lines and gives a **byte-identical
recording**, because it removes tests whose answer was already fixed.

**The text the edit leaves does not compile**, and the edit says so as a
computation rather than a list: one mention of `usefilter` survives, in `ex_read`,
whose only reference was the row that just went, and no surviving `cmdnames[]` row
names that function — which is the whole argument that `funcreach.py` takes it in
the sweep's first round. `tools/phasecheck.sh` in the check is where *it compiles*
is asserted. It is phase 89's shape exactly, one name instead of six.

## The traps, which make the obvious check the wrong one

- **`secure` is not `check_secure`.** The function goes; the variable keeps
  **eleven** mentions, being the vimrc and tag-search flag half the editor tests.
  Only the two inside `check_secure()` went. A copied "every name at zero" loop
  fails on a correct phase.
- **The bare word `read` survives three times** — two `read(fd, …)` calls and an
  E222 string — and `readfile` 5, `read_buffer` 17, `open_buffer` 6, `read_edit` 2,
  `readonly` 4, `shell` 1 and `filter` 2. The eight names that genuinely reach zero
  are `ex_read do_bang do_shell do_filter check_secure prevcmd_is_set prevcmd
  CMD_read`.
- **`E32: No file name` survives and `E484: Can't open file` does not.** E32 is
  still reachable through `check_fname()` from `do_ecmd()`; `ex_read` was E484's
  last speaker, and after this phase nothing in the file says it. Both are asserted,
  in opposite directions, with `"read"`, E12, E34 and E319 — the four strings the
  five swept functions were the last to say.

**No DWARF dump, for phase 89's reason.** Deleting one enumerator renumbers 46
survivors and every one is a `CMD_*`; `cmdnames[]` is designated, the
`static_assert` on the row count catches a dropped pair, and all 104 names are
dispatched by `tools/zexcmds.py` inside the declared delta. Measured with
`tools/enumvals.sh` anyway, once, for this document: **1,303 values in, 1,302 out**,
`CMD_read` the only one gone, 46 moved, nothing arriving.

**The row floor now has four rows of margin.** `cmdnames[]` goes 105 → 104 and
create_cmdidxs's `names()` refuses a table of fewer than 100. `GOALS.md` II.3a: the
`:edit` phase spends the rest, and it is the phase that must lower the floor.

## The declared delta, and what the corpus cannot see

`7   case:cmd_read case:read_cmd_gone` and the sweep row `read`. `cmd_read` types
`:read` **with no file name**, so what the baselines hold is an editor that *failed*
to read, `E32: No file name`; it is `E492: Not an editor command: read` now.
`read_cmd_gone` types `:r !echo piped`, which whim's stub answered with
`E319: Sorry, the command is not available in this version`; it is E492 now, one
snapshot fewer and the same single bell either side. The `read` row does not change
message — it **ceases to exist**, `tools/zexcmds.py` enumerating 104 names where it
enumerated 105.

**`filter_gone` is not declared, and `GOALS.md` II.3b's P6 row over-declares it.**
`:!` has not existed since whim, so `:%!sort` already answered E492 on the input
binary and its record is byte-identical. What this phase removes is the code behind
a command that was already gone — which the check asserts, by requiring E492 on
*both* binaries.

Measured with `tools/zcompare.py`: the other 100 screen cases, the other 103 command
rows, all 30 command lines, the four pty scenarios and the terminal table are
identical.

## The probes, which are the only evidence reading went

**25, on both binaries** — the one the phase was handed, built by the edit part from
the boundary's own makefile flags, and the one it made — eleven required to move and
fourteen required not to.

**The file they read is `keys` itself.** `tools/zstream.py` writes a session's
keystrokes into a file called `keys` in the run directory and feeds it on stdin, so
there is always one file there and no runner has to plant one: `:r keys` reads it
back. **What proves the bytes arrived is the Escape in them** — the keystroke file
holds `…\x1b:q!\r`, which `tools/zscreen.py` draws as `^[:q!^M`, and an Escape can
only be in the buffer if the file was read. The *message* is not the check: `:1r
keys` reads the file and leaves the message line blank, measured, so only `:r keys`
is asked for `"keys" [noeol] 1L, 33B`.

- **`r_keys` and `r_range`** (`:r keys`, `:1r keys`): the keystrokes in the buffer
  on the old binary, E492 and nothing read on this one.
- **`r_missing`** (`:1r nosuch`): `E484: Can't open file nosuch` before, E492 now —
  the probe that pairs with E484 having no speaker left in the source.
- **`r_bang`** (`:r !echo piped`): **E319 in the stream** on the old binary and not
  here. It is in the stream and never in a snapshot, because the message is drawn, a
  `Press ENTER` prompt follows and the next redraw wipes the line before the cursor
  comes back, which is where `tools/zscreen.py` takes its picture. Its presence on
  the old binary is also the proof that no shell ever ran: the stub refused before
  one could.
- **Five spellings** — `:r :re :rea :r! :r !cat` — each E492 now and none before,
  which is the inheritance check `CLAUDE.md`'s `:help` → `:helpclose` trap asks for,
  with `:redo`, `:redraw`, `:registers` and `:reg` required not to move at all.
- **Fourteen that must not move and do not**: `:%!sort`, `:redo`, `:redraw`,
  `:registers`, `:reg`, `:undo`, `:edit`, `:print`, `:append`/`:insert`/`:change`,
  `:q` on a modified buffer (still E37), `:q!` and an ordinary editing session. The
  ones that must not move are required to be *doing* something — `:append` shows its
  added line, `:registers` prints its table **in the stream**, for E319's reason.
- **Two pty sessions**, because every probe above went through a pipe: `:r
  planted.txt`, where **the runner writes the file** — the editor has had no way to
  write one since phase 89 — which the old binary reads into the buffer and this one
  answers E492; and an editing session identical either side.

**Proven able to fail in both directions**: with the old binary on both sides all
eleven report *was to move and did not*, and with the new binary on both sides they
add *the keystroke file did not reach the buffer on the input binary, so this proves
nothing about reading*. Both pty sessions fail the same way round.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 84,675 | **84,453** (−222) |
| functions | 1,841 | 1,835 (−6) |
| enumerators (DWARF) | 1,303 | 1,302 |
| `cmdnames[]` rows | 105 | **104** |
| `nm -u`, the core's flags | 71 | **71, the same set** |
| `nm -u`, as `phasecheck.sh` counts it | 72 | 72 |
| `.text` / `.data` of the plain object | 640,449 / 38,923 | 638,960 / 38,699 |
| binary | 847,656 | **847,368** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set — so a symbol *arriving* would fail too. `:read` reached `readfile()`,
which the startup path still uses, and the shell stubs never called a shell: this
phase removes two commands and not the read path, and `open`, `read`, `close` and
`stat` are required to be **still** undefined, `GOALS.md` II.3b P8 being the phase
that frees them. `.rodata` does not move at all and `.data` loses 224 bytes,
because the five strings are `static char e_…[]` arrays and not `const`.

Six functions go, none of them named by the edit — `ex_read do_bang do_shell
do_filter check_secure prevcmd_is_set` — with three prototypes and five file-scope
variables: `prevcmd` and the four error strings, `"read"` having gone with the row.
The `usefilter` field is the edit's, and the only removal this phase names. The sweep is
**3 rounds, 19 s**, and the phase **43–47 s**. Its boundary is `8f1a98bef913`, and
`make whim-verify` recomputes all eight in 80 s of wall time over 344 s of phases.

## Its placement

`stage 90`, `package files`, and two `uses` lines: `files:90 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records, and
`files:90 harness:86 mechanical`, because the old file-based sweep recorded an exit
status and `:read` goes from E32 to E492 without changing it — phase 86's
message-level record is what can see this phase at all. **There is no `files:90
files:89` line**: `tools/packages.sh --check` refuses a `uses` inside one package,
the ordering between two phases of the same package being the package's.

**`apart 89 90` is measured rather than predicted.** Phase 89's check names `do_bang`
among the things a later phase takes — "it is a later phase's" — and requires
`cmdnames[]` to hold 105 rows. Run on the tree this phase leaves it answers
`do_bang went, and it is a later phase's` and `cmdnames[] has 104 rows and names()
reads 104; both must be 105`, and exits 1. Its `write_roundtrip` probe reads the
file back with `:r out.txt` and would fail too; the source assertions come first.

**And `need 90 swept`, which is the first `need` Part II's schedule has.** The edit's
anchor is `usefilter` at exactly 10 mentions — the field, the two writes anchor 3
removes and seven reads. On the text phase 89's *edit* leaves there are **eleven**,
`ex_write` still being there to read one, and the counted anchor refuses: measured
with `tools/phaserun.sh whim 89-90`, which says `usefilter has 11 mentions, expected
10`. The same run shows the edit's build of the input binary failing on phase 89's
non-compiling intermediate, which is true of every Part II edit that builds one and is
not declared for that reason.
