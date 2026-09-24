# Phase 94 — `:q` quits, and `ZZ` is `ZQ`

`internal/phase/094/edit.go` and `internal/phase/094/check.go`, `stage 94`, `package buffers`.
Phases 89 to 93 took every way to reach a file. What was left of the filesystem in
this editor was a **refusal**: `:q` on a modified buffer answered `E37: No write
since last change (add ! to override)` and stayed. The protection has no remedy once
nothing can be written — there is no `:w` to answer it with and no file the text
could have come from — so it is a door onto nothing, and this phase takes it. `:q`,
`:q!`, `ZZ` and `ZQ` are one thing afterwards.

## One anchor, and eleven of the sixteen functions are a surprise

`ex_quit()` is `if ((check_changed(…)) || (check_changed_any(…))) { not_exiting(…); }
else { getout(0); … }`, and the test *is* the refusal. Folding it **never** keeps
the `else` — quit — and is the last reference `check_changed()` has. That single
fold is the phase; the edit names not one function.

**Fifteen functions follow by reachability and eleven of them are not the refusal at
all.** `check_changed_any()`'s tail is *"go to the buffer that refused"* — it calls
`set_curbuf()`, which calls `enter_buffer()` and `win_enter_ext()` — and after whim
removed the buffer list and the window commands, **that tail was the last caller of
the whole switch-buffer/switch-window island**: `add_bufnum`, `set_curbuf`,
`enter_buffer`, `win_enter`, `win_enter_ext`, `goto_tabpage_win`, `goto_tabpage_tp`,
`get_winopts`, `find_wininfo`, `buflist_findfpos` and `buflist_getfpos`. After this
phase the editor has no code for entering a different buffer or a different window.

**The island is a graph and not a fan, and the edit computes that before it folds
anything.** Only `add_bufnum`, `set_curbuf` and `goto_tabpage_win` are called by
`check_changed_any` itself; the other eight hang off those. So what is required is
that *every* call to any of the eleven is inside `check_changed_any` or inside
another of the eleven — and a check that asked for the simpler shape would fail on a
correct phase. Three of the fifteen also have **no prototype**, being defined above
their first call (`check_changed_any` and `no_write_message_nobang` at two mentions,
`add_bufnum` at three for having two calls), so a loop that wanted three for all of
them refuses. Both facts were discovered by the counted anchors refusing.

## Two extras, each measured byte-identical in the recording

**A — two struct fields that become write-only, which no tool can see.** This is
phase 90's `usefilter` judgement in a smaller shape: `deadfields.py` removes a field
nothing *names*, and gcc has no warning for a member that is only written.
`win_T.w_topline_was_set`'s only reader was in `enter_buffer()` and
`wininfo_S.wi_changelistidx`'s only reader was in `get_winopts()`. The declaration
and the one surviving write of each go by hand, and **the text the edit leaves does
not compile** — both readers are still there, inside functions the sweep is about to
take — which is said in the program rather than discovered, as `internal/phase/090/edit.go`
says of its own.

**B — the tail that cannot run.** After the fold `ex_quit()` ended `int save_exiting
= exiting; exiting = TRUE; getout(0); not_exiting(save_exiting);`. `getout()` sets
`exiting = TRUE` **itself** and ends in `mch_exit()`, which never returns, so the
first, second and fourth statements are dead and gcc cannot prove it. Replacing the
four with `getout(0);` orphans `not_exiting()`, and **`not_exiting()` is the refusal
machinery** — `exiting = save_exiting; settmode(TMODE_RAW);`, the "we changed our
mind, put the terminal back" — so it is this phase's and not tidy. The fold is right
only because `getout()` sets `exiting` for itself; check that before making it on
another tree.

## What survives, and why a check copied from phases 89 to 93 fails here

* **The buffer still knows it is modified.** `bufIsChanged` goes 10 → 7 and
  `curbufIsChanged` does not move at all: CTRL-G still prints `[Modified]`, the
  status line still draws `[+]`, `:set modified?` still answers. What went is the
  refusal, not the state.
* **`:q` can still decline.** `text_locked()`, `curbuf_locked()` and
  `before_quit_autocmds()` all return early **above** the anchor and are untouched.
* **Phases 89, 90, 91, 92 and 93 each assert `E37: No write since last change` survives**
  and name `check_changed` as the `:q` phase's. This is the `:q` phase, so the check
  asserts the opposite in both directions: E37 must be absent here and must have been
  present in the input. `apart 93 94` records it.
* **`open_buffer` goes 5 → 4** — `enter_buffer()` was one of its four callers — where
  phases 92 and 93 both pin it at 5. `buf_spname` goes 5 → 4 and `exiting` 17 → 13.
* **`p_wh` looks write-only and is not.** It goes 4 → 2, the two reads inside the
  island having gone, and what is left is its declaration, which carries the
  initialiser, and one real reader in the frame layer. A naive "uses − writes − 1 ≤ 0"
  scan reports it; the check's scan excludes the declaration **by position** and then
  reports nothing but `vim_ignored`, upstream's sink for an ignored return value,
  which is write-only in the input too. Running it on both texts and requiring the
  same set is what stops an empty answer being a broken scan rather than a clean phase.
* **`SHM_FILEINFO` leaves**, and it is the `'shortmess'` `F` letter: its only reader
  was inside `enter_buffer()`. The letter is accepted and inert afterwards. That is
  the options phase's and no flag string is touched here.
* **`ZZ` is already `ZQ` and stays so.** `nv_Zet` has run `do_cmdline_cmd("q!")` for
  `case 'Z'` and for `case 'Q'` since phase 89. The strings are **not** rewritten to
  `"q"`: it would move `zz_key` and `zq_key` for no gain, and `case:zz_key` is phase
  89's declaration.
* **There are no `'confirm'`-style prompts to worry about**: `grep -cw confirm` on the
  input is 0, whim having removed the dialog layer. Said out loud so that the next
  reader does not go looking.

## Twelve enumerators go and nothing renumbers

`typereach.py` takes twelve as whole anonymous definitions — the four `CCGD_`, the
two `DOBUF_`, `SHM_FILEINFO` and the five `WEE_` — and a whole definition leaving
takes no survivor's value with it. The check dumps DWARF either side and requires
exactly that: **1,197 → 1,185, not one survivor renumbered and none arriving.** That
is the opposite of phase 93, where 85 moved, and it is worth the four seconds either
side to say rather than assume. **No `cmdnames[]` row and no `nv_cmds[]` row moves**:
98 rows, `names()` reads 98, the `static_assert` is in place and `nvidxcheck` reports
194.

## The declared delta: one case and one row, and the row is a third kind

```
11    case:quit_modified
      quit
```

`quit_modified` types text and then `:q`, so what the baselines hold is an editor
that refused: the E37 line goes, the one bell with it, and the record loses a
snapshot — 2,342 → 2,213 bytes. **The exit status does not move there, and that is
the corpus's limit rather than the phase's**: every `zcases.py` case ends with a
trailing `:q!`, which quits the old binary too.

The `ref-excmds.txt` row `quit` **changes message and does not cease to exist**,
unlike every row phases 89 to 93 declared: `:quit` is still a command with its row, so
`tools/zexcmds.py` enumerates the same 98 names and compares the block, whose `msgs`
go from `:set nopaste / E37… / :q!` to `:set nopaste / :quit`. The `cquit` row does
not move. Measured with `tools/zcompare.py`: the other 101 screen cases, the other 97
command rows, all 30 command lines, the four pty scenarios and the terminal table are
identical.

## The probes, which are the only evidence the refusal went

Seventeen, on both binaries — the one the phase was handed, built by the edit part
from the boundary's own makefile flags, and the one it made — six required to move
and eleven not.

* **`q_alone` is the probe.** `ihello<Esc>`, `:set nopaste`, `:q` **and nothing after
  it**. The old binary draws E37, runs out of stdin, prints `Vim: Finished.` and exits
  **1**; this one quits on the `:q` and exits **0**. That difference is the whole
  phase measured from outside and no recording can see it.
* **Five more spellings of the same refusal** — `:q` with the trailing `:q!` (the
  declared case), `:1q`, `:qu`, `:quit`, and `x` then `:q` — each required to have
  refused **before** and not to now, and each to lose the E37 snapshot. `q_modified`'s
  bells must go 1 → 0.
* **Eleven that must not move and are required to be *doing* something**: `q_clean`
  (`:q` on an **unmodified** buffer, status 0 either side and no E37 anywhere — it
  took the else arm before this phase and takes it now, which makes it `q_alone`'s
  pair), `q_bang`, `zz_key` and `zq_key` — which must also be identical **to each
  other** — `cquit` (exit 1 either side), `ctrl_g` (must say `[Modified]`),
  `cmd_set_ro`, `cmd_set_mod` (`:set modified?` must answer), `reg_list`, `cmd_undo`
  and an ordinary editing session.
* **A real terminal**, because every probe above went through a pipe: `ityped<Esc>`,
  `:q`, `:q!`. On the old binary the `:q` draws E37 and leaves the editor running, so
  the `:q!` is what ends the session; here the `:q` quits and the `:q!` reaches
  nothing. An ordinary pty editing session beside it is identical either side.

**Proven able to fail in both directions**: with the new binary on both sides all six
report *was to move and did not* and add *the input binary did not refuse, so this
proves nothing about a refusal being removed* and *exited 0, expected 1*; with the old
binary on both sides they add *this binary still refuses* and *exited 1, expected 0*.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 80,387 | **79,866** (−521) |
| functions | 1,742 | **1,726** (−16) |
| type definitions | 922 | 910 |
| enumerators (DWARF) | 1,197 | **1,185** |
| struct fields | | **−2**, by hand |
| `cmdnames[]` rows | 98 | 98 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 66 | **66** |
| binary | 812,744 | **804,360** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set, so a symbol *arriving* fails too. Sixteen functions go and not one was
libc's last caller: the refusal printed through `emsg()` and the island moved windows,
neither of which reaches the C library on its own. `fclose`, `getc`, `putc` and
`fsync` are required to be **still** undefined and are the `FILE *` phase's; `open`,
`access`, `fcntl`, `stat`, `getcwd` and `strerror` to be still **absent**.

The sweep is **3 rounds** and the phase **97 s**. Its boundary is `b81ce6372fc4`, and
`make whim-verify` recomputes all twelve in 107 s of wall time over 629 s of phases.

## Its placement

`stage 94`, `package buffers` — the first phase of a package of its own — and two
`uses` lines: `buffers:94 seed:83 mechanical`, because the two declared records are
compared with the baselines phase 83 records, and `buffers:94 files:89 rationale`,
because the refusal has no remedy once nothing can be written: phase 89 took every
`:write`, so E37 asked for a save the editor no longer had any way to perform.

**`need 94 swept`, measured, and the brief that specified this phase said there was
none.** The invariant the whole phase rests on is that every call to any of the eleven
island functions is inside `check_changed_any` or inside another of the eleven. On the
text phase 93's *edit* leaves that is **false**: `buflist_findlnum()` is still there to
make `return buflist_findfpos(buf)->lnum;`, a call from outside the island, and phase
93's sweep is what takes it — so `buflist_findfpos` has four mentions where the anchor
wants three. `SHM_FILEINFO` refuses first, at 3 where it wants 2, `ex_file()` still
being there to read the `'shortmess'` `F` letter: `tools/phaserun.sh 93-94` says
`SHM_FILEINFO has 3 mentions, expected 2`. Unlike 90, 91 and 92 the text before it
**compiles** — phase 93's edit left valid C — so the refusal is the counted anchors
alone.

**`apart 93 94`, measured.** Phase 93's check pins `check_changed` at 4,
`no_write_message` at 3, `buf_spname` at 5 and `open_buffer` at 5, and requires `E37:
No write since last change` to survive. Run on the tree this phase leaves it gives
five complaints — `check_changed has 0 mentions, expected 4` among them, and `E37
went, and check_changed() is the :q phase's` — and exits 1. **Only `apart 93 94` is
written**, and the four before it are implied: a stage holding 89 and 94 holds 93, so
that line forbids it already. It is the shape of the missing `apart 85 89`, `apart 89 91`
and `apart 91 93`.
