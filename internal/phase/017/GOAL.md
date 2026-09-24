# Phase 17 — the last two per-buffer encoding options

`'fileencoding'` names the encoding a buffer was read in and will be written
back in, and `'bomb'` whether it had a byte-order mark. With one encoding and no
BOM, both have had one possible value since Phase 12 — but **unlike the six
Phase 16 took, these are not plumbing.** Eight functions read them, and each had
to be looked at:

| | what it wanted them for |
| --- | --- |
| `buf_write()`, `readfile()` | the conversion target, and whether to write a BOM |
| `bomb_size()` | how many bytes of the file are a BOM, for `g CTRL-G` |
| `save_file_ff()`, `file_ff_differs()` | remembering the pair, so `:w` can warn they changed |
| `utf_find_illegal()` (`g8`) | converting to the buffer's encoding to find a byte illegal in it |
| `add_b0_fenc()` | writing the name into a swap file's block zero |

None of those questions has more than one answer now, and the last has no swap
file to write into.

**What the options leave behind is a pair of remembered copies in `buf_T`** —
`b_start_fenc` and `b_start_bomb`, written on every read and looked at by
nobody. A struct field is not a variable, so no warning reports it and the
sweep cannot see it, which is why those are listed in the tool rather than left
to fall out. The same is true of `gvarp`, the local that asked *which* encoding
option was being set: there is one.

`'fileformat'`, `'endofline'` and `'endoffile'` can still change under a buffer,
so `file_ff_differs()` keeps those and loses only the two that cannot.

## The delta

**None.** Both report `E518` instead of a value with one possible setting. What
is left is `'encoding'`, alone, reporting `utf-8`.
