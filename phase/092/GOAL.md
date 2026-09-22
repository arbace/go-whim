# Phase 92 — nothing reads a byte

`phase/092/edit.go` and `phase/092/check.go`, `stage 92`, `package files`. Phases
89, 90 and 91 took every way to *ask* for a file. This one takes the machinery those
commands used: `readfile()`, 787 lines, `read_buffer()`, the four functions of the
message layer that reported what had been read, and eleven more the sweep finds
under them. The file loses 1,183 lines and the core loses `access`, `fcntl` and
`open` — the first libc symbols a Part II phase has freed since phase 89.

## What makes this phase different from every one before it

**`readfile()` was already unreachable when the phase was handed the tree**, and
that is the whole of what makes it delicate rather than difficult. Its three call
sites are one in `read_buffer()` and two in `open_buffer()`, and `read_buffer`'s
only callers are those same two arms. The outer arm needs `curbuf->b_ffname !=
NULL` and the inner one a `read_stdin` argument that all four callers pass as
`FALSE`; phase 88 took the file argument and the bare `-`, and phases 89, 90 and 91 took
every command that could name a file. Nothing the editor can be given reaches it.
gcc keeps the code only because it cannot prove `b_ffname != NULL` never holds.

So **the difference this phase makes is between code that cannot run and code that
is not there**, and no behavioural probe can see it. A recording that *moved* would
mean the cut was wrong. That is why the declared delta is nothing at all, and why
the evidence is something else.

## Four anchors, all inside `open_buffer()`

1. the `if (curbuf->b_ffname != NULL) {…} else if (read_stdin) {…}` pair, as exact
   text with the blank line after it — 36 lines holding all three calls into the
   read path;
2. `int read_fifo = FALSE;`, whose only writer was anchor 1;
3. `else if (retval == OK && !read_stdin && !read_fifo)` → `else if (retval == OK)`,
   where anchor 2's second reader was;
4. the signature — `open_buffer(int read_stdin, exarg_T *eap, int flags_arg)` →
   `open_buffer(void)` — the `int flags = flags_arg;` local, and the four call sites
   in `enter_buffer`, `ml_append_flags`, `ml_replace_len` and `create_windows`,
   every one of which already passed `FALSE, NULL, 0`.

**There is no prototype for `open_buffer`.** It is defined above its first call, so
the `static int open_buffer(…);` line the proto block would hold does not exist, and
a phase that edits one fails loudly. Anchor 4 edits the definition alone.

**Anchor 4 is what takes `read_stdin` to zero, and it is measured rather than
argued.** Without it `open_buffer` keeps three parameters nothing reads, and **the
sweep cannot see them**: `tools/sweep.sh` compiles with `-Wno-unused-parameter`, so
an unused parameter is invisible where an unused local is not. Measured both ways
on this input: anchors 1–3 alone leave the sweep deleting the `int flags =
flags_arg;` local by its own unused-variable pass and `read_stdin` alive at exactly
**one** mention, the parameter. Both swept files are **82,572 lines** and differ in
exactly **five** — the signature and the four calls — the two binaries are the same
830,440 bytes, and **the two recordings are byte-identical**. The fold costs
nothing, says what is true, and is taken.

**One further fold is declined, deliberately.** After anchor 1, `retval` in
`open_buffer` is `OK` from its initialiser to its return and nothing between can
change it, so `if (retval != OK) return retval;` is dead, the function could be
`void`, and the two `open_buffer() == FAIL` guards in the `ml_*` layer can never
hold. That is memline tidy, not the read path. The edit asserts `retval` at its **5
mentions with one assignment** and says it is constant, and the 75-line function is
left for a later phase.

**The text this edit leaves compiles**, where phases 89, 90 and 91 each left theirs
broken until the sweep had run: nothing it removed was named from outside what it
removed, so there is no dangling enumerator and no handler without a row. `readfile` goes 5 → 3
and `read_buffer` 17 → 15, and the survivors are not calls — a prototype, two
definitions, and **fourteen mentions of `readfile`'s own local `int read_buffer =
(flags & READ_BUFFER);`**. The edit asserts exactly that, and the two entry points
the sweep starts from are exactly the two `-Wunused-function` warnings the text
produces: `read_buffer` and `fix_help_buffer`.

## The evidence, which is an instrumented pair and nothing else

There is no behavioural must-differ probe and no dishonest one is offered instead.
What the check does is build **the source the phase was handed, twice**:

- **probe** — `old.c` with `(void)write(2, "READFILE-ENTERED\n", 17);` as
  `readfile()`'s first statement. Recorded with `tools/zrecord.sh`: **0 of the 106
  records** carry the marker.
- **ctl** — the *identical* instrument in `open_buffer()`, which **is** reached.
  **104 of the same 106** carry it.

The zero is the claim; the 104 is what makes it a probe that can fail. The two
records that stay quiet under `ctl` are `ref-pty.txt` and `ref-term.txt`, and the
reason is the instrument and not the editor — both drive a real pty and keep what
was *drawn*, where the other three keep stderr separately. They are named in the
check so that a third going quiet is a failure rather than a shrug.

**Proven able to fail, by measurement**: with `readfile` replaced by `open_buffer`
in the probe build, the check reports *104 of 106 records ENTERED readfile() on the
binary this phase was handed* and exits 1.

**Eight adversarial sessions** run on both instrumented binaries, and they are the
part that asks whether anything could still get in. Naming a buffer after a real
file that exists and then making the editor want its contents is the shape of every
way back into `readfile()` there was: `:file /etc/hostname` and then `G`, an insert
and an undo, `:bdelete`, the `%` register, and then `:new`, `:ball`, `:buffer 1` and
the `#` register. **Each reached `open_buffer()` and not one reached `readfile()`** —
and the first half of that is checked too, because a session that gets nowhere is
not an adversary.

## The declared delta is nothing at all, and it is measured twice

`phase/092/delta` gets a comment for phase 92 and no line, as phases 83, 84 and 86 do.
**`diff -rq` over two full `tools/zrecord.sh` recordings — the binary the phase was
handed against the one it made — is empty**: all 102 screen cases, all 111
Ex-command rows, all 30 command lines, the four pty scenarios and the nineteen
terminal rows. `tools/coredelta.sh --phase 92` then finds the same against whim-vim's
frozen baselines, with the eight lines phases 85 to 91 declared and nothing new.

The check also runs eight sessions directly between the two binaries and requires
each to be identical **and to be doing something**: an ordinary editing session,
`:file` and CTRL-G (still `[No Name]`), `:registers` with its table in the stream,
the `%` and `#` registers, `:q` on a modified buffer (still E37) and `:q!`.

## The traps, which make the obvious check the wrong one

- **`check_readonly` reaches zero here, not at phase 89.** It was `readfile()`'s
  *local*, four mentions since phase 89, and a check copied from that phase fails on
  a correct phase 92.
- **`readonlymode` goes 5 → 3**, where phase 91's check asserts 5: `readfile` held
  two of them. It is still write-only and `FALSE`, and still the options phase's.
- **`msg_scrolled_ign` becomes read-only, and nothing sees it.** Four writers, all
  inside `filemess()` and `readfile()`; after this phase it is `FALSE` for ever with
  one reader left, in `msg_puts_attr_len()`. gcc has no warning for a variable that
  is only read, `deadsweep.py` removes what is unused rather than what is constant,
  and `deadfields.py` is about struct members. It is asserted at **2 mentions,
  read-only**, and handed on rather than folded.
- **Four struct fields become write-only and `deadfields.py` cannot see them**,
  because they are still *named* — by the writes in `buf_store_time()` and
  `set_b0_fname()`: `b_mtime_read`, `b_mtime_read_ns`, `b_orig_size`, `b_orig_mode`,
  three mentions each, of which exactly one is not a write. They go with the
  buffer's name in phase 93. (`b_mtime` and `b_mtime_ns` are read, but only to feed
  `b_mtime_read`, so the whole six-field cluster is dead from outside.)
- **`"[RO]"` goes 3 → 2 and `"[readonly]"` 2 → 1**, each losing `readfile`'s copy
  and keeping the rest, and **CTRL-G's counter survives** — `"%ld line --%d%%--"` is
  `fileinfo()`'s and was never `readfile`'s. All three are asserted at their counts,
  which is the opposite direction from the 24 strings that go.
- **`read_cmd_fd` does not move at all**: 12 mentions on 11 lines, and every one of
  them the terminal's — `fill_input_buf()` and `mch_settmode()`.
- **`setfname` goes 3 → 2**, because `set_rw_fname` was its second caller. That is
  what makes phase 93 possible.

## Seventeen enumerators go and nothing renumbers

`typereach.py` deletes **seventeen whole anonymous enum definitions** — the eight
`READ_*` flags, and `BF_NEW_W`, `CONV_RESTLEN`, `CPO_FNAMER`, `NOTDONE`, `O_EXTRA`,
`SHM_LAST`, `SHM_LINES`, `SHM_OVER` and `SHM_OVERALL` — and a whole definition
leaving takes no survivor's value with it. The check dumps DWARF either side and
requires exactly that: **1,286 → 1,269, not one survivor renumbered and none
arriving**, so no parallel table can have shifted. That is the opposite of phase 91,
where 87 renumbered, and it is worth the four seconds either side to say.

**No `cmdnames[]` row and no `nv_cmds[]` row is touched**: this phase removes no
command. The table is the same 99 rows phase 91 left, `names()` reads 99, the
`static_assert` is in place, and the floor phase 91 lowered to 80 is asserted **by
using it** — the checked parser is called and must not refuse — rather than by
grepping for the number. `tools/nvidxcheck.py` still reports 194 rows indexed once
each. `E32: No file name` survives with `check_fname` at 3 mentions, and E37 with
`check_changed` at 4.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 83,755 | **82,572** (−1,183) |
| functions | 1,818 | 1,802 (−16) |
| type definitions | 998 | 981 (−17) |
| enumerators (DWARF) | 1,286 | **1,269** |
| `cmdnames[]` rows | 99 | 99 — untouched |
| `nm -u`, as `phasecheck.sh` counts it | 72 | **69** |
| `.text` / `.data` / `.rodata` of the object | 633,907 / 38,571 / 17,225 | 623,651 / 38,379 / 16,921 |
| binary | 838,856 | **830,440** |

**Three symbols go and the check names the set, not the count**: `access`, `fcntl`
and `open` were `readfile()`'s and nothing else's. `read`, `close` and `dup` **stay**
and are the terminal's alone — `fill_input_buf()` and `mch_settmode()` — so a check
that read "the file symbols went" would be wrong here; `stat`, `getcwd` and
`strerror` are phase 93's and `fsync` the `FILE *` phase's, and all seven are
required to be **still** undefined.

Sixteen functions go, none of them named by the edit: `readfile` (787 lines),
`read_buffer`, `read_eintr`, `readfile_linenr`, `filemess`, `msg_add_fname`,
`msg_add_lines`, `msg_add_eol`, `after_pathsep`, `dir_of_file_exists`,
`fix_help_buffer`, `gettail_sep`, `mch_isdir`, `set_rw_fname`,
`u_find_first_changed` and `utf_ptr2len_len` — with fifteen prototypes, three
file-scope error strings, seventeen enumerators and **24 string literals**, which
are the whole of the message layer: `"%s%ldL, %lldB"`, `"[noeol]"`,
`"[READ ERRORS]"`, `"[New DIRECTORY]"`, `"Vim: Reading from stdin...\n"`, E200, E201,
E812 and sixteen more. Nothing in the instrument loses a message: the last thing
that could print `"keys" 1L, 30B` was `:read`/`:edit`. The sweep is **3 rounds** and
the phase **42 s**. Its boundary is `6755bb567bea`, and `make whim-verify`
recomputes all ten in 80 s of wall time over 448 s of phases.

## Its placement

`stage 92`, `package files`, and two `uses` lines: `files:92 seed:83 mechanical`,
because `tools/coredelta.sh` compares the recording with the baselines phase 83
records and this phase's whole declaration is that nothing in them moved, and
`files:92 streams:88 mechanical`, because the `read_stdin` *argument* anchor 4 removes
is `FALSE` at all four call sites only since phase 88 took the bare `-` and
`EDIT_STDIN` with it. **There is no `files:92 files:90` or `files:92 files:91` line**,
for phase 90's reason — `tools/packages.sh --check` refuses a `uses` inside one
package — and both dependencies are real and are stated here and in the program's
head instead: `:read` held two of `readfile`'s seven mentions before phase 90, and
`do_ecmd` passed `eap` and flags to `open_buffer` until phase 91, which is what lets
the signature fold happen at all.

**`need 92 swept`, measured.** The edit's anchor is `open_buffer` at exactly 5
mentions — the definition and four callers, every one of them `open_buffer(FALSE,
NULL, 0)`, which is what lets anchor 4 rewrite all four by text. On the text phase
91's *edit* leaves there are **six**: `do_ecmd` is still there to make
`(void)open_buffer(FALSE, eap, readfile_flags);`, the one call site the rewrite
would **not** match and one the sweep would then delete, hiding the mistake.
`tools/phaserun.sh 91-92` says `open_buffer has 6 mentions, expected 5`. The same
run shows the edit's build of the input binary failing on phase 91's non-compiling
intermediate, which is true of every Part II edit that builds one and is not declared
for that reason.

**`apart 91 92`, measured.** Phase 91's check draws its line against this phase as
counts — `readfile` 5, `read_buffer` 17, `readonlymode` 5, `b_ffname` 43,
`b_fname` 37 — so that reaching into the read path would fail rather than widen
quietly. Run on the tree this phase leaves it gives five complaints, `readfile has 0
mentions, expected 5` among them, and exits 1. Its symbol check would fail too,
being a `cmp` of the whole undefined set against a phase that frees three, but the
source assertions come first.
