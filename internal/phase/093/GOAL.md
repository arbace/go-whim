# Phase 93 — the buffer has no name

`internal/phase/093/edit.go` and `internal/phase/093/check.go`, `stage 93`, `package files`.
Phases 89, 90 and 91 took every way to *ask* for a file and phase 92 took the machinery
that read one. What was left of the filesystem in this editor is a **name**: three
`char_u *` fields on every buffer — `b_ffname`, `b_sfname`, `b_fname` — and the one
command that could still set them, `:file`. This phase takes the command, stops
`buflist_new()` naming the buffer it makes, folds the sixteen places that ask what
the name is, and hands the sweep **sixty functions**, the largest number any zero
phase has. It is also where the core stops asking the filesystem questions of its
own initiative: `stat`, `getcwd` and `strerror` go, and with `access`, `fcntl` and
`open` already gone at phase 92 **the process has no way left to acquire a fourth
file descriptor**.

## Seven parts, and every removal that is not one of them is the sweep's

**A — `:file` goes.** The enumerator, the `cmdnames[]` row, and **both** of
`do_one_cmd`'s `CMD_file` tests: the `curbuf_locked()` conjunct phase 91 deliberately
kept, saying this phase would take it, and the second test below it. All in the same
edit as the enumerator or the text does not compile. The row is the only reference a
handler has, so taking it is what makes `ex_file` unreachable, and `rename_buffer`
and `setfname` follow — `set_rw_fname` having been `setfname`'s second caller until
phase 92 took it. **`fileinfo()` survives**, with three callers: CTRL-G, `g CTRL-G`
and the startup message.

**B — `buflist_new()` never names.** Its one call site is `create_windows`', and it
has passed `NULL, NULL` since phase 88 took the file argument. So both parameters go
with the `fname_expand`/`stat`/`buflist_findname_stat` prologue, the
`if (ffname != NULL)` assignment that *was* the naming, the failure arm's frees and
the `st.st_dev` block — nine counted replacements inside one definition, plus the
prototype and the call.

**C — sixteen folds, one per site, each with its constant written out in the
program.** The three fields are NULL for ever, so `== NULL` is TRUE and folds always
and `!= NULL` is FALSE and folds never. **The invariant is computed before anything
is folded**: every write to the three fields is enumerated and required to be inside
`buflist_new`, `setfname`, `rename_buffer` or `shorten_buf_fname` — the four this
phase accounts for — and a write anywhere else would make every fold a guess.

**D — `EX_XFILE` reaches zero rows, which is the largest part of the phase and is
computed.** `:read` was one of its six rows and phase 90 took it; four more went with
the `:edit` family at phase 91; `:file` was the last. So `do_one_cmd`'s
`if ((ea.argt & EX_XFILE) && expand_filename(…) == FAIL)` can never be entered, and
folding it never hands the sweep **32 of the sixty functions** — `expand_filename`,
`eval_vars`, `find_cmdline_var`, the whole `ExpandOne`/`ExpandFromContext`/
`gen_expand_wildcards` layer, `vim_FullName`, `mch_FullName`, `FullName_save`,
`shorten_fname`, `home_replace_save`, `backslash_halve` and the `ff_*` remnants. It
is phase 91's `EX_ARGOPT` in exactly the same shape. One more edit goes with it:
`separate_nextcmd`'s `eap->argt & (EX_CTRLV | EX_XFILE)` loses the second disjunct,
which is 0 for every row, and that is what takes the enumerator itself to zero.

**E — two write-only leftovers nothing can see.** `readonlymode` had one reader,
inside `open_buffer`, and part C folds it; a file-scope static that is assigned and
never read draws no warning and `deadsweep.py` acts on warnings, so it goes by hand
with the `if` around its write — an `if` with an empty body being something no tool
here removes either. `b_dev_valid`'s one surviving assignment is part B's fold of
the device block, and `deadfields.py` removes a field nothing *names*, not one that
is only written, so it goes by hand too and the three fields it guards sweep.

**F — `shorten_fnames()` stops asking where it is.** `shorten_buf_fname()` is empty
after part C, so the `mch_dirname()` cwd fetched for it is fetched for nothing. The
signature folds to `void` in the same edit, for phase 92's measured reason:
`tools/sweep.sh` compiles with `-Wno-unused-parameter`, so an unused parameter is
invisible where an unused local is not.

**G — nothing looks a name up on a disk.** `find_file_name_in_path()`'s
`if (options & FNAME_EXP)` arm searched `'path'` and `mch_getperm()`ed each
candidate; the other arm returns the word itself. Folding the arm never is the
charter reading of *no filesystem access*: the editor extracts text and asks
nothing. `find_file_in_path()` and `mch_getperm()` are then the sweep's, and `stat`
goes with them. **The cost is that CTRL-F and CTRL-P become indistinguishable**, and
that is the one piece of behaviour this phase gives up beyond `:file`.

## Three things about the folds that a tool would have got wrong

**`buflist_name_nr` is folded at its callers and never in place, and the agent that
surveyed this phase made the mistake first.** Its body is `buf =
buflist_findnr(fnum); if (buf == NULL || buf->b_fname == NULL) return FAIL; *fname =
buf->b_fname; … return OK;`. Folding the whole `if` away gives a function that
returns OK with `*fname` never written — a silent behaviour change in the direction
that crashes. What is true is that it returns **FAIL always**, so the fold belongs
at `getaltfname()`, which becomes `emsg(E23); return NULL;`, and at `ex_display()`,
whose `"#` block then does nothing and goes whole. Only then is it uncalled.

**Three sites have an `else` and `cutil.fold_always` refuses them, by design.**
Keeping a body and dropping an else is not what it does, so `fileinfo()`,
`set_b0_fname()` and `get_trans_bufname()` use a local `fold_always_else()` that
keeps the if body dedented four columns — right only because each of the three was
**read** first, phase 91's anchor 5 being what a wrong dedent costs. The helper
refuses a body that is not written one level in.

**Four more are a function whose whole body is the `if`.** `buf_spname()`,
`buf_get_fname()`, `check_fname()` and `getaltfname()` each end in a second
`return`, and `fold_always` there leaves it behind **unreachable and alive** —
measured: `return buf->b_fname;` would have kept `b_fname` referenced for ever and
no sweep tool removes it. Those four are exact-text rewrites of the body.

**And two folds the brief asked for are not made.** Both of `eval_vars()`'s
`if (b_fname == NULL)` arms are inside a function part D makes unreachable —
`expand_filename()` and `expand_wildcards_eval()` are its only callers and both go —
so folding inside text the sweep deletes changes no output and states nothing. Rule
1 applies, and the check requires `eval_vars` at 0 mentions instead.

## What the screen shows afterwards

`[No Name]` survives and is now **the only thing `buf_spname()` can return**, not
one of two answers. CTRL-G prints `"[No Name]" [Modified] 1 line --100%--`
byte-identically either side, and so do `g CTRL-G`, the status line and `:ls`.
`:registers` does not move either: its `"%` and `"#` lines were never printed,
`b_fname` having been NULL since phase 88.

**Three flags are left read-only and named rather than folded.** `BF_NOTEDITED` can
never be set — `setfname()` was its only writer — and `BF_NEW` never could; both are
still read by `fileinfo()`, so CTRL-G asks two questions whose answer is fixed.
`b_shortname` has the same shape and was **already** write-only before this phase.
Folding any of the three changes the string set for no gain, so all three are
asserted where they are. `msg_scrolled_ign` is phase 92's leftover and does not move.

**`"file"` reaches zero and `E32: No file name` does not.** `check_fname()` survives
folded to an unconditional `emsg`, because `get_spec_reg()`'s `%` still calls it.
**`E447: Can't find file "%s" in path` does reach zero here**, and phase 91's check
asserts it *survives* — part G takes its last speaker. The two checks disagree on
purpose, and `apart 91 93` is unnecessary only because `apart 91 92` and `apart 92 93`
already forbid the stage.

## The declared delta: one case and one row

```
10    case:cmd_file
      file
```

`cmd_file` types `:file` with no argument, so what the baselines hold is the CTRL-G
line for `[No Name]` — an editor that answered with the buffer's name. It is
`E492: Not an editor command: file` now, with the **same exit, the same bells and
the same snapshot count**, the stream going 2,310 → 2,327 bytes. The `ref-excmds.txt`
row `file` does not change message: it **ceases to exist**, `tools/zexcmds.py`
enumerating 98 names where it enumerated 99.

**Nothing else can move, and the reason is stronger than a measurement**: every fold
takes the branch the code already took at run time. Measured with
`tools/zcompare.py`: the other 101 screen cases, the other 97 command rows, all 30
command lines, the four pty scenarios and the terminal table are identical —
`:filter` and `:fixdel` among them, and `:q` on a modified buffer (still E37).

## The probes, which are the only evidence a buffer could be named

**21, on both binaries** — the one the phase was handed, built by the edit part from
the boundary's own makefile flags, and the one it made — eight required to move and
thirteen not.

- **`file_rename` is the probe.** `ihello<Esc>:file NEWNAME<CR>` and then CTRL-G:
  the old binary answers `"NEWNAME" [Modified][Not edited] 1 line --100%--` and this
  one `"[No Name]" [Modified] 1 line --100%--`. That line, on the *old* binary, is
  the whole evidence that a buffer could be named, and **the corpus cannot see it**:
  every case types its own text and names nothing. `[Not edited]` is `BF_NOTEDITED`,
  which `rename_buffer()` set and which nothing can set now. `file_bang` is the same
  with `:file!`.
- **`cp_missing` is the only probe that shows the old binary asking the disk.** With
  `nosuchfile` under the cursor, `: CTRL-R CTRL-P <CR>` was **silent** before —
  `find_file_in_path()` stat()ed the name, found nothing and yielded NULL, so nothing
  reached the command line — and answers `E492: Not an editor command: nosuchfile`
  now. Its pair **`cp_existing` must not move and does not**, which is what says part
  G removed the lookup and not the extraction: with `keys` under the cursor — the
  keystroke file `tools/zstream.py` always leaves in the run directory — both
  binaries answer `E488: Trailing characters: eys`, `:k` being a command of its own.
  `cf_existing` is the same session with CTRL-F, which never expanded.
- **Five spellings** — `:f :fi :fil :file :file!` — each E492 now and none before,
  `:file`'s row having given its shortest abbreviation as one character. `:filter`
  and `:fixdel` are the neighbours required not to move, which is the inheritance
  check `CLAUDE.md`'s `:help` → `:helpclose` trap asks for.
- **Thirteen that must not move and do not**: CTRL-G, `g CTRL-G`, `:registers` with
  its table **and without a `"%` or `"#` line**, the `%` and `#` registers,
  `cp_existing`, `cf_existing`, `:filter`, `:fixdel`, `:ls`, `:q` on a modified
  buffer (still E37), `:q!` and an ordinary editing session. Each is required to be
  *doing* something.
- **Two pty sessions**, because every probe above went through a pipe: `:file
  NEWNAME` then CTRL-G, which renames on the old binary and answers E492 and
  `[No Name]` here, and an editing session identical either side.

**Proven able to fail in both directions**: with the new binary on both sides all
eight report *was to move and did not* and add *the input binary did not name the
buffer NEWNAME, so this proves nothing*; with the old binary on both sides they add
*the new binary named the buffer anyway* and *a removed name has been inherited*.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 82,572 | **80,387** (−2,185) |
| functions | 1,803 | **1,743** (−60) |
| type definitions | 981 | 922 |
| enumerators (DWARF) | 1,269 | **1,197** |
| `buf_T` fields | | **−12** |
| `cmdnames[]` rows | 99 | **98** |
| `nm -u`, as `phasecheck.sh` counts it | 69 | **66** |
| binary | 830,440 | **812,744** |

**Three symbols go and the check names the set, not the count**: `stat` was
`mch_getperm()`'s and nothing else's, `getcwd` and `strerror` were `mch_dirname()`'s.
`read`, `close` and `dup` **stay** and are the terminal's alone, and `fsync` is
`ui_write`'s and the `FILE *` phase's; all four are required to be still undefined,
and `open access fcntl chmod fchmod fstat lstat unlink` to be still **absent**, which
is the file-descriptor invariant stated as a check.

**Seventy-two enumerators go and eighty-five renumber, every one of the 85 a
`CMD_`** — the `EXPAND_*`, `WILD_*`, `EW_*`, `XP_BS_*`, `SPEC_*`, `BLOCK0_*`, `BLN_*`,
`ESTACK_*`, `VSE_*` and `VALID_*` families leave as whole anonymous definitions, which
takes no survivor's value with them, and `CMD_file`'s row is what moves the rest.
`cmdnames[]` is designated, so a row lands at its own enumerator whatever the
numbering is — but 85 movers from one family is exactly the case `CLAUDE.md` says a
build is happy to get wrong, so the check dumps DWARF either side and requires it.

The sweep is **4 rounds** and the phase **67 s**. Its boundary is `2829849cb53a`, and
`make whim-verify` recomputes all eleven in 80 s of wall time over 523 s of phases.

## Its placement

`stage 93`, `package files`, and two `uses` lines: `files:93 seed:83 mechanical`,
because the declared records are compared with the baselines phase 83 records, and
`files:93 harness:86 mechanical`, because `:file` went from the CTRL-G line to E492
with the **same exit status** — the old file-based sweep recorded an exit status, so
phase 86's message-level record is what can see this phase at all. **There is no
`files:93 files:90`, `files:93 files:91` or `files:93 files:92` line**, for phase 90's
reason — `tools/packages.sh --check` refuses a `uses` inside one package — and all
three dependencies are real and are stated in the program's head instead: `:read` was
one of the six `EX_XFILE` rows and phase 90 took it, four more went with the `:edit`
family at phase 91, which is what makes `:file` the last and part D possible, and
`set_rw_fname` was `setfname`'s second caller and went with `readfile` at phase 92.

**`need 93 swept`, measured.** The edit's anchor is `b_ffname` at exactly 32
mentions, with `b_sfname` at 26 and `b_fname` at 29. On the text phase 92's *edit*
leaves there are **forty**, `readfile()` still being there to make eight of them, and
the counted anchor refuses: `tools/phaserun.sh 92-93` says `b_ffname has 40
mentions, expected 32`. Unlike 90, 91 and 92, the text before it **compiles** — phase
92's edit left valid C — so the refusal is the counted anchor alone.

**`apart 92 93`, measured.** Phase 92's check draws its line against this phase as
counts — `b_ffname` 32, `b_sfname` 26, `b_fname` 29, `setfname` 2, `readonlymode` 3,
`eval_vars` 4, `mch_dirname` 5 and the four write-only fields at 3 each — and names
the four as things phase 93 takes. This phase takes every one to 0. Run on the tree
it leaves, phase 92's check gives eighteen count complaints, `b_ffname has 0 mentions,
expected 32` among them, plus `b_mtime_read is no longer a field of buf_T`, and exits
1. **There is deliberately no `apart 91 93`**, although phase 91's check does fail here
— it requires E447 to survive — because a stage holding 91 and 93 holds 92, and `apart
91 92` forbids that already. It is the shape of the missing `apart 85 89` and `apart 89 91`.
