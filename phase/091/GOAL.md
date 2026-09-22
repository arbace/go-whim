# Phase 91 — no `:edit`, and no `gf`

`phase/091/edit.go` and `phase/091/check.go`, `stage 91`, `package files`. Phases
89 and 90 took the commands that put bytes on a disk and the one that takes them off
it. This one takes the commands that point the editor **at** a file — `:edit :enew
:ex :visual :view` — and the four Normal-mode keys that do the same thing from the
buffer's own text, `gf gF [f ]f`. What is left of opening anything is `readfile()`
and `open_buffer()`, which the startup path still uses, and this phase asserts by
count that it has not reached them.

**What the five commands were, measured from outside rather than read.**
`do_exedit` is thirty lines after phase 87 — a lock guard, a `readonlymode`
save/set/restore testing `CMD_view` and `CMD_enew`, `setpcmark()` and one
`do_ecmd()` call — so on the input binary, in a directory holding a file called
`keys`: `:e! keys` loads it (`"keys" [noeol] 1L, 30B`), `:ex! keys` and `:visual!
keys` do exactly the same, `:view! keys` loads it **and makes `:set ro?` answer
`readonly`**, and `:enew!` empties the buffer. One handler, `ex_edit`, is all five
rows, which is why the five go together.

## Six anchors, and the sixth is measured

1. five enumerators of `enum CMD_index`, one line each;
2. five `cmdnames[]` rows, one physical line each, designated `[CMD_x] = {`;
3. `do_one_cmd`'s `curbuf_locked()` exemption: the `ea.cmdidx != CMD_edit` conjunct
   goes and **`CMD_file` stays**, `:file` being the buffer-name phase's. It must go
   in the same edit as anchor 1, and it is **the one anchor outside the table and
   the keys** — an edit shaped like the table forgets it, and the build is what
   catches that;
4. `nv_g_cmd`'s `case 'f': case 'F': nv_gotofile(cap); break;` arm. `case 'f':`
   alone occurs six times in that function and the four-line block once;
5. the same call in `nv_brackets`;
6. `do_one_cmd`'s `if (ea.argt & EX_ARGOPT) { while (… getargopt(&ea) …) }`.

**The sixth is worth its lines, and that is a measurement.** `EX_ARGOPT` — `++ff=`,
`++enc=`, `++bin`, `++edit` — was on five rows: `:read`, which phase 90 took, and
these four. After anchor 2 it is on **none**, so the block can never be entered,
`getargopt()` can never run, and `exarg_T.read_edit` is written by nothing and read
by nothing. Deleting it hands all three to the sweep: 30 lines, a seventeenth
function, and a recording **byte-identical** to the one the five anchors alone
produce — `++edit` was only ever accepted by the commands this phase removes, so
there is nothing to declare. `EX_CMDARG` reaches zero rows too and is deliberately
left: `do_ecmd_lnum` is written through `eval_vars()`, which is the buffer-name
phase's, so the fold round it belongs there.

**Anchor 5 is not a `cutil.fold_never`, and the reason is indentation.**
`fold_never` keeps an `else` body by dedenting it four columns, which is right when
the body was written one level in. This one was not: upstream's `else` here has no
braces at all — the `if` is inside `#ifdef FEAT_SEARCHPATH` — so slim's bracing pass
put a `{`/`}` round the rest of the function and left every line at the function's
own four columns. Dedenting would have put forty lines at **column zero**, and
`CLAUDE.md`'s *Verification tiers* says no tier can see indentation. So the head and
its matching closer are deleted as counted text, found by brace matching and
required to be a line of its own, and the body keeps what it had.

**No handler is named.** The row is the only reference a command handler has, so
taking the five rows is what makes `ex_edit` unreachable and sixteen more follow it.

**The text the edit leaves does not compile**, as phases 89 and 90 leave theirs: three
mentions of `CMD_enew` and `CMD_view` survive, all inside `do_exedit`, and the edit
asserts that as a computation — every survivor is inside a function definition and
no surviving `cmdnames[]` row names that function, which is the whole argument that
`funcreach.py` takes it in the sweep's first round.

## The hazard this phase does not have, asserted anyway

**No `nv_cmds[]` row is deleted or repointed.** There is no row for `gf`, `gF`, `[f`
or `]f`: they are arms inside two handlers whose `g`, `[` and `]` rows dispatch
dozens of other keys. That is an argument, and `CLAUDE.md`'s twelve-phase arrow-key
bug is what an argument costs when it is wrong, so the check measures it: **fifty
`g*`, `[` and `]` keys pressed on both binaries, and exactly four moved** — `gf gF
[f ]f` — the other 46 identical in exit, bells, snapshots and stream digest.
`tools/nvidxcheck.py` still reports 194 rows indexed once each.

## The row floor is crossed here, and the floor moves in the same commit

`cmdnames[]` goes **104 → 99**, and create_cmdidxs's `names()` refused a table of
fewer than 100. **The failure is not the one the name suggests**: measured, it is
`no command table found in either shape`, because `names()` tries both parsers with
`check=False` and neither answer clears the bar. `tools/zexcmds.py` enumerates
Part II's whole Ex sweep through it, so the old floor would have stopped the sweep,
`tools/coredelta.sh`, the recording and every later phase's check rather than giving
a wrong answer. `GOALS.md` II.5 decision 8 is settled: **lowered to 80, deliberately,
in the phase that crosses it, with the reason in the tool's own docstring.** The
margin is 19 rows and the next row the plan removes is `:file`'s.

**What the floor edit costs, measured over all 115 implementation keys**
(`tools/implhash.sh` for every whim stage, every whim edit, every slim phase and
every Part II phase): **28 move** — 6 whim stages (42-63, 66-71, 72, 73-77, 78, 79), 15
whim edits (58, 63, 66, 68–79), 2 slim phases (6, 7) and 5 of the phases from 83 (85, 87, 88, 89,
90). The last five are there only because their programs name the tool's path in a
comment; `implhash.sh` greps for paths and does not know what a comment is. Every
one of those phases calls the tool with a 489- or 600-row table, so `--check` passes
identically and every boundary reproduces; the cost is CPU in a repass. **Gated on
both**: `make slim-verify` 12 of 12 (394 s of phases in 114 s of wall time) and
`make whim-verify` 13 of 13 (1,638 s in 595 s), green after the edit. This is core rule 9
being paid rather than avoided — a Part II-only tool was not an option, because the
floor is inside the tool the sweep reads the table with.

## The traps, which make the obvious check the wrong one

- **Five pairs where one name is a prefix of another and only one goes**:
  `check_lnums`/`check_lnums_both`, `reset_VIsual`/`reset_VIsual_and_resel`,
  `u_unchanged`/`u_unch_branch`, `do_ecmd`/`do_ecmd_cmd`, `otherfile`/`otherfile_buf`.
  Every count in both programs is `\b`-anchored for that reason.
- **`"edit"` reaches zero and `"ex"` does not.** `getargopt()`'s `++edit` strncmp
  was the last speaker of `"edit"` once the row went, and anchor 6 takes it; `"ex"`
  survives as one word of `'belloff'`'s value list. A check that wanted both to
  survive fails on this phase, and one that wanted both gone fails on a correct one.
- **`E447: Can't find file "%s" in path` survives.** `nv_gotofile()` was not its only
  speaker, so the message the key probes look for on the old binary is still in the
  source afterwards. It is the **key** that went, not the string.
- **Three things are left write-only rather than removed**, and are asserted at
  their counts so that a later widening has to move them: `readonlymode` (5
  mentions, one write, and that write `FALSE`), `do_ecmd_cmd` (6) and `do_ecmd_lnum`
  (2).

**`'undoreload'` is not this phase's, and it earns the plan's `uses` line.** `p_ur`'s
only reader was inside `do_ecmd`, so it is now a global with an option row and
nothing that reads it. Removing the row would change what `:set ur?` answers, which
nothing this pipeline records sweeps, so the delta could not be checked — and
`tools/orphanopts.py` refuses the opposite direction, a global whose row has gone.
It is asserted at exactly 2 mentions **with** its row, and
`uses options:94 files:91 mechanical  'undoreload' is read by do_ecmd` is the line
the options phase carries.

## The line against the byte-reader phase, and one correction to the plan

`readfile` keeps exactly **5** mentions — its prototype, its definition and the
three calls in `read_buffer()` and `open_buffer()` — and `read_buffer` 17.
`open_buffer` goes **6 → 5**, and that is the number `GOALS.md` II.3c got
backwards: it says `readfile`'s last caller is `do_ecmd`, and `do_ecmd` called
`open_buffer`. What this phase costs the read path is one call site. The plan's
`uses files:91 files:90` line is corrected there.

## The declared delta, and what the corpus cannot see

`8   case:cmd_edit case:key_gf` and the five rows `edit enew ex view visual`. The
two kinds of movement are different:

- **`cmd_edit`** types `:edit` with no file name, so the baselines hold an editor
  that got as far as `check_changed()` and refused — `E37: No write since last
  change (add ! to override)`. It is `E492: Not an editor command: edit` now.
- **`key_gf`** presses `gf` on a word naming nothing, so the baselines hold
  `E447: Can't find file "nosuchfile" in path` — an editor that **looked**. The key
  beeps from `nv_g_cmd`'s `default: clearopbeep` now, where every other unused `g`
  key does: the record loses one snapshot and keeps **one bell either side**, so the
  check is the screen and the snapshot count and not the bell.
- the five `ref-excmds.txt` rows do not change message, they **cease to exist**:
  `tools/zexcmds.py` enumerates 99 names where it enumerated 104.

Measured with `tools/zcompare.py`: the other 100 screen cases, the other 94 command
rows, all 30 command lines, the four pty scenarios and the terminal table are
identical.

## The probes, which are the only evidence

**34, on both binaries** — the one the phase was handed, built by the edit part from
the boundary's own makefile flags, and the one it made — twenty required to move and
fourteen not, plus the fifty-key sweep and two pty sessions.

**The file they open is `keys` itself.** `tools/zstream.py` writes a session's
keystrokes into a file called `keys` in the run directory and feeds it on stdin, so
there is always one file there and no runner has to plant one. **What proves the
bytes arrived is the Escape in them** — the keystroke file holds `…\x1b:q!\r`, which
`tools/zscreen.py` draws as `^[:q!^M`, and nothing typed at `:` can put an Escape in
the buffer.

- **`edit_keys`, `ex_keys`, `visual_keys`, `view_keys`**: the keystrokes in the
  buffer and the file named on the old binary, E492 and nothing opened on this one.
  `view_keys` then asks `:set ro?` and requires `readonly` before and `noreadonly`
  after, which is the whole of what `do_exedit` did with `CMD_view`.
- **`enew_bang`**: the old binary throws the text away and this one does not.
- **`key_gf gF [f ]f`**: E447 on the old binary — the proof that the key reached
  `nv_gotofile()` and looked — and **one snapshot fewer** afterwards, the bell
  unchanged.
- **Ten spellings** — `:e :ed :edit :enew :ex :vi :vis :vie :view :visual` — each
  answering E37 before and E492 now, which is the inheritance check `CLAUDE.md`'s
  `:help` → `:helpclose` trap asks for. **`:en` is the eleventh and is not one of
  them**: `enew`'s shortest abbreviation is three characters, so `:en` matched
  nothing before this phase either, and it is asserted as E492 on *both* sides —
  whim's Phase 80 rule, measured rather than argued.
- **Fourteen that must not move**: `:en`, `:earlier`, `:verbose set ro?`,
  `:vglobal/a/d`, `:vmap`, `:file` (still `[No Name]`), `:read keys` (E492 on both,
  phase 90 having taken it), `:print`, `:append`, `:registers`, CTRL-G, `:q` on a
  modified buffer (still E37), `:q!` and an ordinary editing session. The ones that
  must not move are required to be *doing* something.
- **Two pty sessions**: `:e! planted.txt`, where the runner writes the file — the
  editor has had no way to write one since phase 89 — which the old binary loads and
  this one answers E492; and an editing session identical either side.

**Proven able to fail in both directions**: with the new binary on both sides all
twenty report *was to move and did not* and add *the keystroke file did not reach
the buffer on the input binary, so this proves nothing about opening a file*; with
the old binary on both sides they add *the new binary opened the file anyway*.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 84,453 | **83,755** (−698) |
| functions | 1,835 | 1,818 (−17) |
| type definitions | 1,010 | 998 |
| enumerators (DWARF) | 1,302 | **1,286** |
| `cmdnames[]` rows | 104 | **99** |
| `nm -u`, as `phasecheck.sh` counts it | 72 | **72, the same set** |
| `.text` / `.data` of the plain object | 638,960 / 38,699 | 633,907 / 38,571 |
| binary | 847,368 | **838,856** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set, so a symbol *arriving* would fail too. `:edit` reached `do_ecmd()`,
which reached `open_buffer()` and `readfile()`, and both are the startup path's, so
`open`, `read`, `close` and `stat` are required to be **still** undefined,
`GOALS.md` II.3b P8 being the phase that frees them.

Seventeen functions go, none of them named by the edit — `do_ecmd` (328 lines),
`get_visual_text`, `check_lnums_both`, `do_exedit`, `nv_gotofile`, `grab_file_name`,
`prepare_help_buffer`, `u_unch_branch`, `text_or_buf_locked`, `reset_VIsual`,
`reset_VIsual_and_resel`, `delbuf_msg`, `ex_edit`, `u_unchanged`, `otherfile`,
`check_lnums` and `getargopt` — with two struct fields, sixteen enumerators and
seven string literals (`"edit" "enew" "view" "visual"`, E143, E1546 and the help
buffer's `'iskeyword'`). **Sixteen enumerators go and 87 renumber, every one of the
87 a `CMD_`**, and the check dumps DWARF either side and requires it: the sixteen
are the five `CMD_`, `EX_ARGOPT`, and ten single-constant enums the sweep takes with
their types (`CPO_GOTO1 DOCMD_RANGEOK ECMD_FORCEIT ECMD_HIDE ECMD_NOWINENTER
ECMD_OLDBUF ECMD_SET_HELP FNAME_REL FNAME_UNESC READ_NOWINENTER`). The sweep is **3
rounds** and the phase **50 s**. Its boundary is `eeb4031a31b4`, and `make
whim-verify` recomputes all nine in 79 s of wall time over 394 s of phases.

## Its placement

`stage 91`, `package files`, and two `uses` lines: `files:91 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records, and
`files:91 harness:86 mechanical`, because the old file-based sweep recorded an exit
status and `:edit` goes from E37 to E492 without changing it. **There is no `files:91
files:90` line**, for phase 90's reason: `tools/packages.sh --check` refuses a `uses`
inside one package.

**`need 91 swept`, measured.** The edit's anchor is `readfile` at exactly 5 mentions.
On the text phase 90's *edit* leaves there are **seven**, `ex_read` still being there
to make two of them, and the counted anchor refuses: `tools/phaserun.sh 90-91`
says `readfile has 7 mentions, expected 5`. The same run shows the edit's build of
the input binary failing on phase 90's non-compiling intermediate, which is true of
every Part II edit that builds one and is not declared for that reason.

**`apart 90 91`, measured.** Phase 90's check requires `check_fname` at 4 mentions,
`open_buffer` at 6 and `read_edit` at 2, names `do_ecmd` and `otherfile` as a later
phase's, and requires `cmdnames[]` to hold 104 rows with `:edit` among them. Run on
the tree this phase leaves it gives seven complaints — `:edit went, and it is not
this phase's` and `do_ecmd went, and it is a later phase's` among them — and exits 1.

**And deliberately no `apart 89 91`**, which is the shape of the missing `apart 85 89`.
Phase 89's check *does* fail on a phase 91 tree — measured: `do_bang went`, `otherfile
went`, `cmdnames[] has 99 rows and names() reads 99; both must be 105`, exit 1 — but
a stage holding 89 and 91 holds 90, and `apart 89 90` forbids that already.
