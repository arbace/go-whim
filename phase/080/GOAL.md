# Phase 80 — the Ex command table, cut to the commands that exist

600 rows in `enum CMD_index` and `cmdnames[]`, and **489 were `ex_ni` or
`ex_script_ni`**. Every phase that removed a command had pointed its row at the stub
and left it, under rule 3 as it then read, because a row still did one job: its
*name* decided what every abbreviation of every other name meant. The rows, and the
two-level prefix index generated from them, were kept for that alone.

**The rows go and what they were for stays.** The old lookup took the first row, in
index order, whose name started with the typed word, so a command's shortest
abbreviation was implied by every row above it. Measured on q79: deleting the 489
rows in place hands **15 prefixes** that used to reach a stub to a live command —
`:n` to `nmap`, `:o` to `omap`, `:h` to `highlight`, `:sa` to `saveas`, `:la` to
`later`, `:en` to `enew`, `:ve` to `verbose`. No live command would lose an
abbreviation or gain another's; an error would just quietly become a mapping
listing.

So each surviving row **carries its shortest abbreviation**, computed from the
600-row table before a row is touched, in the field that held the name's length —
whose one reader was the Vim9 whole-name check, dead since Phase 79. A word names a
row when it is a prefix of the name and at least that long. That makes a match
**unique**, which makes row order irrelevant, which makes the index pointless:
`cmdidxs1`, `cmdidxs2`, `command_count` and E943 go, and the lookup is a scan of 111
rows. `tools/create_cmdidxs.py --check`, which fifteen phases between 58 and 79 ran, has no
block to check in `whim-vim.c` any more and is not called from here on. Its `names()`
still reads the table for `exsweep.py`, and refuses fewer than 100 rows; a phase that
takes the table below that has to lower the floor.

**Proved rather than argued, twice.** The program models the old lookup — the index
read out of the file, its start points and all — and the new one, over all 2,538
prefixes of the 600 names. They must agree wherever the old answer survives, find
nothing wherever it did not, and no word may match two rows. Then every one of those
words, plus 94 command lines covering every address form the surviving commands
take, goes through **both binaries** — the input's, built in the background while
the edits run, and the output — comparing exit status, stderr, the file afterwards
and anything left in the directory. Words the old table sent to `:stop` or
`:suspend` are left out, as the command sweep leaves those commands out.

## What went with the rows

- **26 `CMD_` tests** of commands that no longer exist: `:wincmd`'s address type, the
  filename-escaping exceptions for `:grep`, `:make` and `:terminal`, `:new`/`:split`/
  `:sview` in `do_exedit`, `:try`, the Vim9 `:final` and `:horizontal` quirks, and
  the index's two start points `CMD_Next` and `CMD_bang`.
- **The `ni` flag** in `do_one_cmd`, which exempted a stub from the range, bang,
  count and argument checks. No row can raise it.
- **The user-command test `(int)cmdidx < 0`**: nothing assigns a negative index.
- **The `py3` and `vim9` digit rules** in `find_ex_command`: no row left starts with
  `py` or `vim`.
- **Seven address types** only stub rows used — argument list, buffers, loaded
  buffers, two for tab pages, two for quickfix — 49 case labels, 35 whole arms, and
  the buffer-offset arithmetic behind them. The program refuses to delete an arm
  that the arm above it can fall into.
- **`:if`, and with it `ea.skip`.** `:if` was a stub row that `do_one_cmd`
  special-cased to raise `if_level`, which made later commands skipped. But `:if`
  takes the rest of its line, `if_level` is reset at the end of every `do_cmdline`,
  and nothing passes `DOCMD_REPEAT`, so no command could ever run with it raised.
  `ea.skip` was already constantly false, and its nineteen readers fold.

## The delta

A removed name gives **E492 "Not an editor command"** instead of E319, with the same
exit status, and the command sweep cannot see the text. Two things can see a
difference, and both were agreed before the program was written:

- **`:if`** was accepted silently (exit 0) and is an error now (exit 1).
- **`stub|cmd`** used to run `cmd` after the stub's error, because a stub row with
  `EX_TRLBAR` split its line at the bar; an unknown name takes the whole line.
  `:buffer|%s/a/X/|w` wrote the substitution before and writes nothing now.
  `:h|…` is the control: `:help`'s row never had `EX_TRLBAR`, and it comes out the
  same.

And every removed row leaves the command sweep, which dispatches the names in the
table — so the declared list is Phase 79's plus all 489, and the program requires
that list to be exactly the stub rows.

## What the dry runs caught

All five were in the checks or my arithmetic, not the edits: a `vim9` word check
that matched the string literal `"vim9"` (which is how the digit rules were found); a
lookup span counted as 31 lines that is 33; `cutil.delete_definition` returning a
pair, not the text; a "no `sizeof(\"` left" check that matched unrelated string
lengths elsewhere in the file; and `:h|…` expected to differ, because I assumed every
stub row split at the bar without reading `:help`'s flags.

Measured: 88,636 → **87,142 lines**, the binary 1,008,424 → 955,976 bytes.
