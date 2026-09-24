# Phase 87 — no streaming Ex

`internal/phase/087/edit.go` and `internal/phase/087/check.go`, `stage 87`, `package streams`. The
second cut, and the first that removes a *mode*. Ex mode is the arrangement a core
does not have: the editor takes stdin over, prints its own prompt, reads a line at a
time and writes the result back on stdout. Silent mode comes with it — the message
layer redirected into `printf()` and stdout buffered through `setvbuf()` — and both
are entered from the command line (`-e`, `-E`, `-s`, `-v`) or from the keyboard
(`Q`, `gQ`).

`do_exmode()` (96 lines), `getexmodeline()` (263) and `nv_exmode()` (12) go, the
four option cases go, and then `exmode_active` (49 mentions) and `silent_mode` (23)
are constantly FALSE and fold at every reader. **The two counts are the whole
argument and are asserted both ways**: 49 and 23 before, every use gone after, with
the nineteen identifiers that go with them — counted by `\b`, because
`pending_exmode_active` contains `exmode_active` and a plain substring count says
53.

**Every fold is counted and scoped to one function, because the polarity is not the
same at every site.** `if (exmode_active)` folds never; `if (!exmode_active)` folds
always; and `msg_start()`'s `if (exmode_active != EXMODE_NORMAL)` folds **always**,
because `0 != 1` is TRUE. That one sits among its opposites and reads exactly like
them, and getting it backwards would have given every message Ex mode's newline,
with nothing in the build to say so. Two other shapes needed care: `fold_never` on
an `if` rewrites the `else if` after it into an `if`, so `main_loop`'s next anchor
is written without the `else` it had a moment earlier; and `command_line_scan` has
two `if (exmode_active)`, so the four option cases go first or the counted fold
refuses — loudly, which is the point.

**Three functions are deleted by name rather than left to the sweep.** A function
whose address is taken is reachable as far as gcc is concerned: `getexmodeline` is
passed to `do_cmdline()` and compared with `getline_equal()`. Its last live
reference is one disjunct of `do_cmdline`'s 200-column `while` condition — miss it
and 263 lines survive silently. `nv_exmode` goes the same way, because an
`nv_cmds[]` row is a reference: **the `'Q'` row is repointed at `nv_error` and never
deleted**, a hole being what moves every key past it onto another key's handler.

**Six write-only leftovers go by hand**, because nothing sees them:
`ex_pressedreturn`, `ex_no_reprint` (seven writes), `ex_exitval`,
`previous_got_int`, `use_plus_cmd` and `exmode_was`. A file-scope static that is
assigned and never read draws no warning at all, and the locals draw
`-Wunused-but-set-variable`, which `tools/deadsweep.py` does not act on.
`main_loop`'s `noexmode` parameter and the `theend:` label it jumped to go with
them — an unused label *is* a warning, and `tools/phasecheck.sh` fails on it.

**`check_tty()` is deleted here, and that is a gap in the sweep worth naming.**
Folding its one remaining branch leaves `int input_isatty; input_isatty =
mch_input_isatty();` — set and never read, which is exactly the kind
`deadsweep.py` does not act on. Measured: the sweep reports `left alone 1` and
settles with the warning still there. So the function and its call in `main()` go by
name, and `mch_input_isatty()` — whose only caller it was — is what the sweep takes,
with the fifth `isatty()` call. **It is phase 85's cut as much as this one's**: phase
85 kept that branch deliberately, saying Ex mode was a later phase's, which is why
the manifest carries `uses streams:87 terminal:85 mechanical`. `GOALS.md` II.3b's table
gives `isatty` to P2; it arrives here.

**What stays, and the reader that forces each.** `getexline()`, because `:append`,
`:insert` and `:change` read their lines through it and not through the Ex-mode
reader. `exe_commands()`, because it runs the `+{command}` list every harness here
drives the editor with — only its last statement folds. And everything the argv
phase owns: `case NUL`'s `EDIT_STDIN` arm, `read_cmd_fd = 2`, `had_minmin`, the file
argument, `ME_TOO_MANY_ARGS` and its `main_errors[]` row, `case 'T'`. The check
names each of them.

## The declared delta, and the one the plan over-declared

Six records move, and every one is a way *in* to Ex mode: `case:key_Q`,
`case:key_gQ`, `argv:-e`, `argv:-E`, `argv:-e_-s`, `argv:-v`. The two keys drew
`Entering Ex mode.  Type "visual" to go to Normal mode.` and now beep once, from
`nv_error` and from `nv_g_cmd`'s `default: clearopbeep`; the command lines are
`mainerr(ME_UNKNOWN_OPTION)` like any other unknown letter.

**`-s` alone is not declared.** `case 's'` set silent mode only `if (exmode_active)`
and called `mainerr()` otherwise, so a bare `-s` was an unknown option *before* this
phase; measured, its record is byte-identical. `GOALS.md` II.3b's P3 row lists it.
Two of the four that are declared — `-e` and `-e -s` — differ only in stderr, which
`stderr-moved` already excuses everywhere; they are named anyway, because they are
ways into Ex mode and this is the phase that closes them.

## The probes, and why a delta is not enough here

The baselines are one recording of one binary, so `tools/coredelta.sh` can say
"exactly these six moved" and cannot say "the old binary entered Ex mode". The check
says it, by running both: the binary the phase was **handed**, built by the edit
part from the boundary's own makefile flags, and the one it made. **29 probes, in
two halves** — six required to move and 23 required not to — each of the six also
required to show Ex mode, or an option the old parser accepted, on the *old* binary.
A probe that only looks at the new binary passes on a phase that did nothing.

The 23 are `-s` alone, six `+{command}` forms, `:append`/`:insert`/`:change`,
`:visual`/`:vi`/`:view`/`:ex` from Normal mode (whose Ex-mode escape this phase
folded away), bare `-`, `--`, `-- +q!`, one and two file arguments, the three `-T`
spellings and an ordinary edit. Each record is built the way `tools/zcases.py`
builds one and scrubbed the same way, because `mainerr()` prints the version banner
and two binaries built a minute apart disagree on `__DATE__` for a reason that is
not the editor's behaviour — which is precisely why `-s` reads as unchanged and
must.

Two pty sessions beside them: `Q`, `visual<CR>`, `<Esc>:q!` — Ex mode entered by the
old binary and by nothing now — and an editing session identical either side. **The
`<Esc>` is not decoration.** Without Ex mode those six letters are Normal-mode keys
that end in Insert mode, `:q!` is typed into the buffer, and the session runs to the
timeout and is killed: status 9, measured, and it looked like a broken harness.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `internal/phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 86,586 | **85,813** (−773) |
| functions | 1,867 | 1,862 |
| `nm -u` | 80 | **78** — `setvbuf`, `stdout`, nothing else |
| `isatty(` calls | 5 | **4**, and the symbol stays |
| binary | 869,512 | **861,288** |

Five functions: `do_exmode`, `getexmodeline`, `nv_exmode` and `check_tty` by name,
`mch_input_isatty` by the sweep — which is clean in two rounds and also takes the
three single-constant enums `EXMODE_NORMAL`, `EXMODE_VIM` and `BO_EX`, each with an
explicit value, so nothing renumbers. In the plain object `.text` goes 654,576 →
648,968 and `.rodata` 17,689 → 17,625. `stdout` was the file's only mention, and it
was `setvbuf`'s argument.

The phase is **81 s** cold and 51 s with its edit cached; its boundary is
`97a2ab4895e7`, and `make whim-verify` recomputes all five in 81 s of wall time over
191 s of phases. No whim or slim cache key moved: all 107 are identical to `main`'s.

Its placement carries one thing the schedule does not need yet and will:
`apart 85 87`. Phase 85's check runs both of its binaries with `-e -s` and requires
exit 0, which is how it proves `--`, `+cmd` and `-T` still work — and this phase
removes `-e`. Every Part II phase is a stage of its own today, so the line is a
statement; it becomes a constraint the moment two of them share a sweep.
