# Phase 42 — one buffer, always

The buffer list is the container the editor edits in — `firstbuf`, `curbuf` and
the buffer hash table are read everywhere — so **it stays, with exactly one
buffer on it between commands**. Two decisions were the user's, not the
process's, and were asked: editing another file **reuses the one buffer**, and
**there is no alternate file**.

**The mechanism is `'bufhidden=wipe'`, made unconditional.** `do_ecmd()` still
makes the new buffer and then closes the old one; it closes it with `DOBUF_WIPE`
instead of `DOBUF_UNLOAD`, or not at all under `ECMD_HIDE`. So `:e`, `:enew`,
`:next`, `:previous`, `:drop` and `gf` work as before, and a file left behind
takes its undo history, marks and local options with it. Changes cannot be lost
by it: `do_ecmd()` has already refused a changed buffer unless it was written or
`!` was given, exactly as under `'nohidden'`.

Three rows go to `ex_ni` — `:bnext`, `:bprevious` and `:keepalt` — and
`tools/onebuffer.py` removes what a row cannot:

- **Nothing is hidden.** `buf_hide()` answered from `'hidden'`, the `:hide`
  modifier and `'bufhidden'`, and all three go, so each of its sixteen call sites
  folds as if it said no and the sweep takes it. `close_buffer()` stops reading
  `'bufhidden'`, and `tools/droplocal.py` takes the field.
- **No alternate file**, which is itself a second buffer. Nothing writes
  `w_alt_fnum`: `do_ecmd()`, `set_curbuf()`, `do_exedit()` and `win_init()` stop,
  `:file`, `:read` and `:write` stop making an alternate buffer for a name, and
  `buflist_findnr(0)` and a `#` pattern find nothing — which is what they did when
  there was no alternate. CTRL-^ points at `nv_error`; `:e #` fails.
- **`:saveas` renamed the buffer by swapping names with an alternate buffer**
  made for the new name. With no alternate it would have written the file and
  kept the old name, so it renames the one buffer with `setfname()`. It is the one
  addition in the phase, and the reason is that the mechanism, not the behaviour,
  needed a second buffer.
- **The argument list stops making buffers.** `alist_add()` put every file
  argument on the buffer list, unloaded, when it was named. An entry is a name now,
  buffer number 0 — `alist_name()` and `editing_arg_idx()` already fall back to the
  name — and the one buffer is named for the first file only while it is still the
  empty buffer startup made.

**Stays:** `:qall`, `:wall`, `:wqall` and `:xall`, which are `:q` and `:w` with one
buffer, and which every harness here quits with. `'buflisted'`, whose field is
internal state the buffer code reads. No command-line option opened more than one
buffer, so none goes.

The phase checks the behaviour it changes against what it replaces: under
`'nohidden'` an unloaded buffer keeps its marks, so marking a line, editing
another file and coming back finds the mark; with one buffer it is gone, and
deleting to it changes nothing. It checks that `:e #` is refused and that
`:saveas` renames.

Two first runs failed usefully: the leftover counts included `setaltfname()` and
`buf_hide()`, which have no caller and which the sweep takes, and
`rename_buffer()`'s `xfname` had held the old short name for the alternate alone.

## The delta

**`:bnext`, `:bprevious` and `:keepalt`**, which succeeded run bare. Measured:
117,506 → **117,013 lines**, libc symbols 88 → 88.
