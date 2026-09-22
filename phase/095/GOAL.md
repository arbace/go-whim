# Phase 95 — the options nothing reads

`phase/095/edit.sh` and `phase/095/check.sh`, `stage 95`, `package options`.
Phases 89 to 94 took every way to reach a file and then the refusal that guarded the
text. What they left behind is a set of **settings**: `options[]` rows whose global
nothing reads any more, so that `:set fsync?` answers a question about machinery that
is not there. An option that cannot do anything is a lie, and the same argument that
removed `:write` removes `'write'`.

## Which rows go is computed, not listed

The edit walks `options[]`, finds each row's `(char_u *)&p_xx` and counts readers of
that global outside the row, with `tools/dropoptions.py --strict`'s own exclusions —
another row, the row's `var` field, the variable's declaration (which is what the row
initialises), and taking the address, which asks which option a pointer refers to and
never touches the value. **Exactly seven of the 114 rows have no reader**, and the
program requires that set rather than naming six of them:

| row | var | indir | verdict |
| --- | --- | --- | --- |
| `fsync` | `p_fs` | `PV_BOTH` | goes, after `droplocal.py b_p_fs` |
| `modified` | `p_mod` | `PV_BUF` | **stays** |
| `prompt` | `p_prompt` | `PV_NONE` | goes |
| `readonly` | `p_ro` | `PV_BUF` | goes, by `GOALS.md` II.5 decision 5 |
| `undoreload` | `p_ur` | `PV_NONE` | goes |
| `write` | `p_write` | `PV_NONE` | goes |
| `writeany` | `p_wa` | `PV_NONE` | goes |

**`'modified'` has no reader of `p_mod` either and must not go.** Decision 5 keeps it:
the state it reports lives in `b_changed`, not in `p_mod`, so `:set modified?` answers
correctly and the row is not a lie. A computation that took "no reader" as the
criterion would delete it, which is why the seven are computed and the six are
*chosen*. `dropoptions.py` refuses it anyway, on the `PV_` guard. It is now the only
option row with no reader of its own global.

**`'prompt'` is the find, and `GOALS.md` II.3b's row 11 computed four.** Its only reader
was `getexmodeline()`'s `if (p_prompt) msg_putchar(':');`, so it is **phase 87's
orphan**, collected here — which is a `uses` line the plan does not have.

**`'paste'` is exempt for ever**, and that is asserted rather than only written down:
`p_paste` has 12 mentions before and after, its five save slots `p_ai_nopaste`
`p_et_nopaste` `p_sts_nopaste` `p_tw_nopaste` `p_wm_nopaste` four each, and the edit
refuses outright if the computation ever offers `'paste'`. `+{command}` is likewise
untouched. That is the user's standing promise (`GOALS.md` II.2d and II.5 decision 8), and
the next person to widen the computation meets the assertion and not just a comment.

## Four parts, and only one of them is live code

**A — four clean rows**, `dropoptions.py --strict prompt undoreload write writeany`.
The sweep then takes the four globals as `-Wunused-variable`.

**B — `'fsync'`, which `--strict` alone refuses**, and not on a reader: the row is
`PV_BOTH + PV_BUF + BV_FS`, so the tool stops on the `PV_` guard, because *the row is
what initialises the global* (`'tagcase'` taught that by segfaulting before the first
keystroke). `droplocal.py b_p_fs` is the other half and goes first — six plumbing
sites, including `get_varp()`'s two-line "local if set" form — then `--strict --local
fsync`.

**C — `'readonly'`, which is live code and not an inert row.** `p_ro` the global has
had no reader since whim; what survives is the buffer-local `b_p_ro`, at ten mentions,
and since phase 89 nothing but `:set ro` can set it, which is decision 5's premise.
Five edits, in this order and for this reason:

1. **the W10 warning.** `change_warning()` and its six call sites, each one statement
   on a line of its own. There is **no prototype** — it is defined above its first
   call — so a program that removes one fails loudly. This also takes the
   `ui_delay(1002L, TRUE)` that phase 85 named as one of the eight other pauses, and
   the `static char *w_readonly` inside the function.
2. **the `[RO]` in `fileinfo()`.** The format string and the argument move
   **together**, `%s%s%s%s%s%s` to `%s%s%s%s%s`, and nothing in the build checks a
   `vim_snprintf_safelen` count.
3. **the `[RO]` on the status line**, in `win_redr_status()`: the name-padding
   disjunct and the block that appends the indicator.
4. **`did_set_readonly()`, by name and with the reason.** It is the row's callback and
   the row is its only other reference, so the sweep would take it — but `droplocal.py`
   runs in the *same edit* and would find it still reading `b_p_ro`. Measured without
   it: `droplocal: b_p_ro still has 1 mentions after the plumbing went`, which is the
   tool working. The alternative is an inner sweep; this is cheaper and honest.
5. the row, then `droplocal.py b_p_ro` — three plumbing sites.

**D — what the sweep then finds**: `SHM_RO`, `BV_FS`, `BV_RO`, `w_readonly`, the six
globals, and the `b_did_warn` field — which becomes dead **only after both**
`change_warning` and `did_set_readonly` have gone. Remove one and it is a field with
one reader and one writer, which no tool reports.

## The flag letters are not touched, and that is a decision

`'cpoptions'` and `'shortmess'` each have a **validity list that is a separate string
literal from the value**, so removing a letter from a list cannot move `:set cpo?` or
`:set shm?`. But it *would* turn `:set shm=F`, accepted silently, into `E539: Illegal
character`, and no corpus case, Ex row, argv row or pty scenario types `:set shm=` —
which is exactly the kind of change core rule 2 exists to prevent. Accepting a letter that
does nothing is what upstream does for every feature a build lacks.

Measured: **23 of `'cpoptions'` 60 letters and 14 of `'shortmess'` 23 are inert** — in
a validity list with no enumerator of that value — and **this phase makes exactly one
more so, `'shortmess'`'s `r`**, whose `SHM_RO` goes with the `[RO]` indicator. Both
literals are asserted character for character, the inert sets are computed either side
and required to differ by exactly `{r}`, and four probes require `:set shm=F` and
`:set cpo=g` to be accepted silently on **both** binaries and `:set shm=y` and `:set
cpo=h` to answer E539 on both.

## The row floor, which this phase crosses

`tools/orphanopts.py` refused a table it parsed fewer than 100 distinct `&p_xx` out
of; this phase takes the count **102 → 96**, and its first call crosses it.
`tools/zerodelta.sh` runs that tool beside its harnesses, so crossing the floor does
not fail *this* phase — it fails the delta check of **every Part II phase after it**, with
a message about a table that moved. It is the same failure shape as
`create_cmdidxs.py`'s 100-row floor at phase 91, arriving from a different table.

**The floor is 80 now, lowered in this phase's own commit**, with the reason in the
tool's docstring — the same number and the same argument as `create_cmdidxs.py`'s, so
that the two floors stay one idea. 80 leaves 16 globals of margin below 96 and the
plan removes no further rows. The check proves it **by using it**, not by grepping for
the number: the tool must not refuse, and its output on this source must be
byte-identical to its output on the input — five non-pointer orphans, which are
`'paste'`'s save slots, and every option pointer still with the row that sets it.

**It cost implementation keys, and that is stated rather than hidden.** `tools/whimdelta.sh`
names `orphanopts.py` and `tools/implhash.sh` hashes what a delta checker names, so
lowering the number re-keys whim and zero. Measured, before and after, over every
whim stage, every whim phase-as-unit, every whim edit, every slim phase and every zero
unit and edit: **12 whim stage keys, 4 whim edit keys, 82 whim phase-as-unit keys, 12
Part II unit keys and 3 Part II edit keys move, and not one slim key.** The tool's *verdict*
is unchanged everywhere — `slim-vim.c`, `whim-vim.c` and every Part II boundary are far
above either floor, and the output is byte-identical — so no boundary can move; the
cost is CPU in a repass. `make whim-verify` and `make slim-verify` are the gate
`GOALS.md` core rule 9 asks for, and both were run.

## The declared delta is nothing at all, and the reason is not phase 92's

Phase 92 removed code that **could not run**. This phase removes code that **can run
and that the instrument cannot see**. Measured record by record:

* `:set <name>?` goes from an answer to `E518: Unknown option`, and **no recorded case
  or row asks any of the six.**
* **bare `:set` does not move**, because none of the six differs from its default, and
  its listing is wiped by the Press-ENTER redraw before `zscreen.py` takes its picture.
* **`:set all` does move** — `readonly`, `fsync`, `prompt` and `undoreload` are in the
  old stream and absent from the new — and `:set all` is in no harness. It is a probe.
* **the W10 warning and the two indicators move**, and no recorded case sets
  `'readonly'`: they need `:set ro`, which nothing types.
* `tools/zexcmds.py` records `exit`, `bells`, `stderr`, `text` and `msgs` for the `set`
  row and **no stream digest**, so even a change to what `:set` prints in the stream
  would be invisible there.

So a phase that did nothing and a phase that did everything have the same recording.
`diff -rq` over two full recordings — the binary the phase was handed against the one
it made — is **empty**, and `tools/zerodelta.sh --phase 95` finds the nine lines phases
85 to 94 declared and nothing new. `phase/095/delta` gets a comment and no line.

## The probes, which are not a supplement but the check

**Twenty-seven, on both binaries**, thirteen required to move and fourteen not.

* **`ro_w10` is the one that shows behaviour going rather than a row.** `:set ro` on an
  **unmodified** buffer, then an insert: the old binary prints `W10: Warning: Changing
  a readonly file` and **pauses a second** — 1,006 ms measured against 2 ms here, the
  same shape as phase 85's 2,010 ms → 5 ms. The message is never in a snapshot: it is
  drawn, a Press-ENTER follows and the redraw wipes it, exactly as `whim90-check`'s
  E319, so the assertion is on the *stream* and on the elapsed time. The buffer must be
  unmodified when `:set ro` runs — `change_warning()` returned early on `b_did_warn ||
  curbufIsChanged()` — so a probe that types its seed first shows nothing on either
  binary.
* **`ro_ctrlg`** (`[readonly]`, not `[RO]`, because `'shortmess'`'s default has no `r`),
  **`ro_shm_r`** (`:set shm=r` first, the only probe that reaches `SHM_RO`) and
  **`ro_statusline`** (`+set laststatus=2`, which `win_redr_status` is reached by
  nothing else here).
* **`set_all`**, and the six `:set <name>?` spellings plus `:set readonly` and `:set
  ro`, each an answer before and `E518: Unknown option` after.
* **Fourteen that must not move and are required to be *doing* something**:
  `paste_roundtrip` (the exempt option, which the whole corpus depends on),
  `mod_query`, `shm_query`, `cpo_query`, `shm_F`, `shm_bad`, `cpo_g`, `cpo_bad`,
  `nu_query`, `bare_set`, `set_listing`, `ctrl_g` (still `[Modified]`), `undo_case`
  and an ordinary editing session.

**Proven able to fail in both directions**: with the new binary on both sides all
thirteen report *was to move and did not* and add *the input binary did not warn* and
*took 10 ms … under half a second means it never drew it*; with the old binary on both
sides they add *this binary still warns*, *took 1,006 ms, so something is still
pausing* and *`:set ro` does not answer E518 now, so something can still mark a buffer
read only*.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,866 | **79,757** (−109) |
| functions | 1,726 | 1,724 (−2) |
| type definitions | 910 | 909 |
| enumerators (DWARF) | 1,185 | **1,182** (−3, nothing renumbers) |
| struct fields | | **−3** (`b_p_fs`, `b_p_ro`, `b_did_warn`) |
| `options[]` rows | 114 | **108** |
| distinct `&p_xx` in `options[]` | 102 | **96** |
| `cmdnames[]` rows | 98 | 98 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 66 | **66** |
| binary | 804,360 | **803,912** |

**Nothing is freed, and the check states it as an equality** — a `cmp` of the whole
undefined set. An option row is not a libc call, and `fsync` is still reached from
`ui_write()` and is the `FILE *` phase's.

**Three enumerators go and nothing renumbers, and `BV_RO` is why it needs saying**: it
is *unpinned*, so `BV_SI`, the next survivor, would follow it down. `tools/deadenums.py`
pins `BV_SI = 53` in the sweep and `enumvals.sh --verify` reports it there; the check's
independent dump either side requires `BV_SI` to hold its value and no other survivor
to move. `BV_FS` had an explicit value and so does its successor.

The sweep is **2 rounds** and the phase **37 s**. Its boundary is `fbaa6d80884b`.

## Its placement

`stage 95`, `package options`, and four `uses` lines: `options:95 seed:83 mechanical`,
because the declaration is "none" and "none" is checked against phase 83's baselines;
`options:95 files:89 mechanical` (`'write'`, `'writeany'` and `'fsync'` were
`do_write`'s, `not_writing`'s, `check_overwrite`'s and `buf_write`'s, and phase 89 left
`:set ro` as the only thing that could mark a buffer read only); `options:95 files:91
mechanical` (`'undoreload'` was read by `do_ecmd`); and **`options:95 streams:87
mechanical`, which the plan does not have** — `'prompt'`'s only reader was
`getexmodeline()`.

**`need 95 swept` is not required, and it was measured rather than assumed.**
`tools/phaserun.sh whim 94-95` runs phase 95's edit on the unswept text phase 94's edit
leaves, and every counted anchor matches: the same seven rows come back from the
computation and the cut ends at the same 108 rows and 96 globals. The run fails only
on the edit's build of its input binary, which is true of every Part II edit that builds
one and is not declared for that reason.

**`apart 94 95`, measured.** Phase 94's check pins `p_ro` and `p_ur` at 2 mentions
**with their option rows** and says in as many words that removing one is the options
phase's. Run on the tree this phase leaves it gives six complaints — `p_ro has 0
mentions, expected 2`, `'readonly' lost its option row, and that is the options
phase's`, `curbufIsChanged has 6 mentions, expected 7` (`change_warning`'s early return
read it) and `the function count went 1742 -> 1724, expected 1742 -> 1726` among them —
and exits 1. Phase 93's check pins the same two rows and would fail too, but a stage
holding 93 and 95 holds 94 and `apart 93 94` forbids that already.
