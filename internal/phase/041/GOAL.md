# Phase 41 — the buffer list is walked by `:bnext` and `:bprevious` alone

**The list stays.** Every file edited is a buffer on it, `:bnext` and
`:bprevious` move through it, `:e #` reaches the alternate one, and
quitting still refuses while a hidden buffer is changed. Every other command on
it goes — fifteen rows: `:buffer`, `:buffers`, `:files`, `:ls`, `:badd`, `:balt`,
`:bdelete`, `:bunload`, `:bwipeout`, `:bfirst`, `:brewind`, `:blast`,
`:bmodified`, `:bNext` and `:bufdo`.

**`:bNext` is a row of its own**, spelled apart from `:bprevious` though it shares
the handler, so `:bN` goes with it; `:bp` still reaches `:bprevious`.

`tools/nobuflist.py` removes what a row cannot:

- **`:badd` and `:balt`** went through `ex_edit()` and `do_exedit()` with `:edit`,
  so only their terms go — and `do_ecmd()`'s `ECMD_ADDBUF` and `ECMD_ALTBUF`
  paths, which nothing else passed.
- **`:bdelete`, `:bwipeout` and `:bunload`** were `do_bufdel()` and `do_buffer()`,
  which the sweep takes. That leaves `do_buffer_ext()` one caller, `goto_buffer()`,
  and one action, `DOBUF_GOTO`, so its `unload` is always false and every branch
  that unloaded, deleted or wiped folds. **That fold is true only after the
  sweep**, so the phase asks after it: exactly one call, and that one.
  `set_curbuf()` and `empty_curbuf()` keep their unload paths, because
  `check_changed_any()` still passes one. `do_one_cmd()` stops asking whether a
  buffer-name argument belongs to one of the three.
- **Completion** for the retired names. `ex_listdo()` had `:bufdo` as its last
  user and goes whole.

A first run counted the retired names before the sweep, and found four in
`ex_listdo()` and `ex_bunload()` — handlers with no row, which the sweep then
took. The count is asked after the sweep now.

## The delta

**The ten rows that succeeded run bare** — `:buffer`, `:bNext`, `:bdelete`,
`:bfirst`, `:blast`, `:brewind`, `:buffers`, `:bwipeout`, `:files` and `:ls`, read
from the slim baseline. `:badd`, `:balt`, `:bmodified`, `:bufdo` and `:bunload`
already failed with no argument. Measured: 118,130 → **117,506 lines**, libc
symbols 88 → 88.
