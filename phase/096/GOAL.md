# Phase 96 — no `FILE *` that is never opened

`phase/096/edit.go` and `phase/096/check.go`, `stage 96`, `package tidy`. Two
`static FILE *` survive in this editor and **nothing has ever opened either of them in
any build of `whim-vim`**: `scriptin[NSCRIPT]`, which `-s {scriptfile}` filled and for
which whim removed the option, and `redir_fd`, which `:redir > file` filled and for
which whim removed the command. So this phase removes the **possibility** rather than
a behaviour — phase 92's situation and phase 92's answer.

## The counts are the argument, and each is computed before anything is folded

* **`scriptin[]` is assigned in exactly one place in the whole file**, and that place
  is `scriptin[curscript] = NULL;` inside `closescript()`. So it is NULL for ever.
* **`redir_fd`'s only assignment is its own declaration**, `= NULL`.
* **`ui_write()` has three mentions** — a prototype, a definition and one call — and
  that call passes `FALSE` for `console`.

The edit asserts all three as exact text before it folds anything, because every fold
rests on them; a fourth assignment anywhere would make every one a guess.

## Six anchors, in three groups

**A — `scriptin[]` is NULL for ever.** `may_sync_undo()` and `is_safe_now()` each lose
one conjunct and **survive**: `u_sync()` still runs on the same condition, and
`is_safe_now()` is still `stuff_empty() && typebuf.tb_len == 0 && !global_busy`.
`using_script()` is FALSE at both call sites — a `&& !using_script()` conjunct and a
`|| using_script()` disjunct — and the sweep then takes it. And `inchar()`'s script
reader goes as text with its local, after which `if (script_char < 0)` is always true
and folds; **that fold is what takes `closescript()`'s only caller**, and `fclose` and
`getc` with it.

**B — `redir_fd` is NULL for ever**, so `redirecting()` is FALSE always and folds at
both call sites, in `undo_cmdmod` and inside `redir_write()`. **Their indentation
differs**, which is what makes two separate one-count patterns honest rather than a
count of two over one pattern. The second fold takes the whole `fputs`/`putc` block.

**C — `ui_write()`'s `console`** is FALSE at its one call site, so the `vim_fsync(1)`
it guards can never be entered. **The parameter goes too**, and that is what makes the
cut honest: leaving it would leave `__attribute__((unused))` on something that will
never be read again — phase 85's argument for `check_tty(void)` — and `tools/sweep.sh`
compiles with `-Wno-unused-parameter`, so an unused parameter is invisible where an
unused local is not. `vim_fsync()` is then uncalled and `fsync` goes.

## Two locals are folded by hand and no tool covers either

**`retesc`** is written only inside the loop anchor A4 deletes and read once.
Afterwards it is a local that is **read and never written**: gcc has no warning for
that, `deadsweep.py` acts on warnings, and leaving it would mean `inchar()` returns an
uninitialised value on a path the compiler thinks exists. `return retesc;` becomes
`return FALSE;` and the declaration goes. It is folded **after** the two declarations
and **before** `fold_always`, because the fold dedents the body it keeps and a rewrite
counted against the original indentation refuses afterwards — which it did, the first
time this was run.

**`did_return`** is the same shape one level down: the `if (!did_return)` block the
`redir_write` extra removes is its only reader, and an `if` with an empty body is not
something any tool here removes either, so the block goes whole with `cutil.drop_if`
and the variable's two lines with it.

## The recommended extra is taken, and a second is declined

After B, `redir_write()` is `{ char_u *s = str; static int cur_col = 0; if (redir_off)
return; }` — the sweep takes the two variables and leaves a function with five callers
that cannot do anything. Leaving it is the "concept the table has and the code does
not" that phase 18 argued against, so it goes with its five call sites, and
`redir_off` — then written **five** times, not four, and read never, a file-scope
static that no warning covers — goes with them. `msg_puts_attr_len()`'s call was every
message the editor prints, and that is the one to notice: nothing is printed
differently, because `redir_write()` returned without doing anything at every one of
them.

**A second extra is declined and is a question for the user, not an oversight.** After
this phase `typedef struct stat stat_T;` has no user and `#include <sys/stat.h>` and
`#include <fcntl.h>` are needed by nothing. Removing all three is free and was
measured — same binary, byte-identical recording, four fewer lines — but it would be
**the first time any Part II phase changes the directive count**, and the charter above
says `whim-vim.c` "inherits 18 directives from `whim-vim.c`". That sentence is a
statement about the pipeline, so the change belongs to whoever decides it, either here
or as an includes phase of its own. The count stays **18**.

**The user took it, as phase 99, and that phase found the paragraph above short by a
header and by a line.** There is a third that supplies nothing — `<iconv.h>`, whose
only occurrence in `whim-vim.c` is its own `#include` line, whim having removed the
conversion layer and left it — and the cut is five lines and not four, because the
typedef sits between two blank lines and one of them has to go with it. So the answer
here was **15 directives**, not 16, and phase 99 takes six because phases 97 and 98
emptied three more. The decision to decline was right for its reason: the charter now
says in as many words that a phase may remove a directive and may not add one.

## The honest problem, and the probe that answers it

Nothing this phase removes is reachable, so there is **no behavioural must-differ
probe** and no dishonest one is offered instead. The check builds the source the phase
was handed, twice:

- **probe** — `(void)write(2, "FILESTAR-ENTERED\n", 17);` at **five** places: the top
  of `closescript()`, inside `inchar()`'s `getc(scriptin[curscript])` loop, inside
  `redir_write()`'s `redirecting()` block, inside `undo_cmdmod`'s, and the top of
  `vim_fsync()`. **0 of the 106 records** carry the marker.
- **ctl** — the *identical* instrument at the top of `ui_write()`, which every byte
  the editor draws goes through. **105 of the same 106** carry it.

The zero is the claim; the 105 is what makes it a probe that can fail. **Proven able
to fail, by measurement**: with the instrument moved to `ui_write()` in the probe
build, the recording carries the marker in 105 of 106 records and the check reports
*105 of 106 records ENTERED one of the five sites on the binary this phase was handed*
and exits 1.

**Eighteen adversarial sessions** run on both instrumented binaries, and every one of
them is a way of making the editor **print**, which is where `redir_write()` sat —
`msg_puts_attr_len()` called it for every message. `:messages`, `:verbose set ai?`,
`:silent echo`, `:history`, `:registers`, `:display`, `ga`, an unknown command, `:set
all`, `:marks`, `:undolist`, `:changes`, `:map`, `:highlight`, `:normal ihi`,
`:g/a/p`, a recorded-and-replayed register and `:set verbose=9`. **Each reached
`ui_write()` and not one reached any of the five** — and the first half is checked
too, because a session that draws nothing is not an adversary.

## The declared delta is nothing at all, measured twice over

`diff -rq` over two full recordings — the binary the phase was handed against the one
it made — is **empty**: all 102 screen cases, all 111 Ex-command rows, all 30 command
lines, the four pty scenarios and the nineteen terminal rows. `tools/coredelta.sh
--phase 96` then finds the same against whim-vim's frozen baselines, with the nine
lines phases 85 to 94 declared and nothing new. `phase/096/delta.md` gets a comment and
no line. Nine ordinary sessions run directly between the two binaries and each is
required to be identical **and** to be doing something.

## Measured

*Measured before canonical seeding (`d8365fb`), and kept as the record of that run; a row re-measured since says so. What the pipeline measures now is in `phase/boundaries.md`.*

| | input | after |
| --- | --- | --- |
| lines | 79,757 | **79,603** (−154) |
| functions | 1,724 | 1,719 (−5) |
| type definitions | 909 | 908 |
| enumerators (DWARF) | 1,182 | 1,181 (`NSCRIPT`, nothing renumbers) |
| `FILE` mentions | 2 | **0** |
| `#include` | 18 | 18 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 66 | **62** |
| `nm -u` with the core's flags | 65 | **61** |
| binary | 803,912 | **799,816** |

**Four symbols go and the check names the set, not the count**: `fclose` and `getc`
were `closescript()`'s and `inchar()`'s script loop's, `putc` was `redir_write()`'s,
and `fsync` was `vim_fsync()`'s — whose only caller was `ui_write()`'s `console`
branch, which is why **`fsync` is this phase's and not the buffer-name phase's**.

**`fputs` does not go, and `GOALS.md` II.3b row 12 says it does.** After this phase the
source names it nowhere and `nm -u` still lists it: gcc lowers `fprintf(stderr, "…")`
to it, exactly as it lowers `printf` to `fputc`, `fwrite` and `putchar`. The check
asserts the freed set as exactly `fclose fsync getc putc`, with `fputs fputc fwrite
putchar __errno_location` named as gcc's own and required to be **still** undefined.

**`GOALS.md` §II.4b's invariant is assertable in its strongest form now**, and the
check states it: `open creat openat stat access fcntl getcwd strerror fopen fdopen
opendir` are absent from **both** the source and the undefined set. The core has no
`open`, no `stat`, no stdio stream and no fourth descriptor — it can read, write,
close and dup fds 0, 1 and 2 and nothing else.

The sweep is **3 rounds** and the phase **37 s**. Its boundary is `f995296f2536`.

## Its placement

`stage 96`, `package tidy`, and two `uses` lines: `tidy:96 seed:83 mechanical`, because
the "none" is checked against phase 83's baselines, and `tidy:96 terminal:85 rationale`,
because `ui_write()`'s `console` argument is FALSE at its one call site either way and
phase 85 is where the terminal stopped being asked anything — so dropping the parameter
rather than leaving `__attribute__((unused))` on it is that phase's argument for
`check_tty(void)`.

**`need 96 swept` is not required, and it was measured**: phase 96's edit applies
unchanged to the *unswept* text phase 95's edit leaves, every counted anchor at the
same number. The run fails only on the edit's build of its input binary, which is true
of every Part II edit that builds one.

**`apart 95 96`, measured.** Phase 95's check pins `scriptin` at 8 mentions, `redir_fd`
at 6 and `vim_fsync` at 3 and names all three as the `FILE *` phase's; it also requires
`fclose`, `getc`, `putc` and `fsync` to be **still** undefined. Run on the tree this
phase leaves it gives three complaints — `redir_fd has 0 mentions, expected 6`,
`scriptin has 0 mentions, expected 8`, `vim_fsync has 0 mentions, expected 3` — and
exits 1. Its symbol check would fail too, being a `cmp` of the whole undefined set
against a phase that frees four, but the source assertions come first. Phase 94's check
pins the same three and would fail as well, but a stage holding 94 and 96 holds 95 and
`apart 94 95` forbids that already.
