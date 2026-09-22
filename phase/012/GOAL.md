# Phase 12 — UTF-8, and no other encoding, ever

Phase 9 made `'encoding'` a property of the build rather than of the machine.
This makes it **not a setting at all**: `mb_init()` accepts `utf-8` and returns
"invalid argument" for anything else, so `:set enc=latin1` fails the way a
misspelt value fails, and the latin1 and DBCS character paths lose their only
caller and are swept.

## The conversion layer is cut at its entry points, not unpicked from its callers

This is the shape of the phase and the reason it is small. `readfile()` is 1,758
lines with conversion woven through a retry loop, partial-character carry-over
and a `goto retry`; `buf_write()` is much the same. Excising that by hand is the
kind of surgery that compiles, passes a symbol check, and corrupts a file on
some path nobody tested.

Instead **six functions answer differently**, and every one of those answers is
a case the callers already branch on:

| | now answers | which is what happens when |
| --- | --- | --- |
| `my_iconv_open()` | failure | the system has no iconv — a case upstream supports |
| `convert_setup()` | `CONV_NONE` | source and target encodings are the same |
| `string_convert()` | `NULL` | there is nothing to convert |
| `check_for_bom()` | no BOM found | the file has none |
| `make_bom()` | writes nothing | `'bomb'` is off |
| `convert_input_safe()` | the input unchanged | no input conversion is set up |

Nothing is restructured. The sweep then takes `convert_setup_ext`,
`string_convert_ext` and `iconv_string`, because nothing reaches them.

**Only then** are the seven branches that can no longer be taken deleted — and
only because the calls inside them are what keep `iconv`, `iconv_open` and
`iconv_close` in the symbol table. A dependency that is linked in and never
reached is exactly what this pipeline exists to remove. Each is a brace-matched
`if` with no `else`, and **every anchor names a line of the body, not just the
condition**: `if (fio_flags == 0)` occurs twice in `readfile()` and the first has
an `else` after it, so the obvious anchor deleted the wrong block and left an
orphaned `else` — which gcc reported as *"expected `}` before `else`"* and then
as two undefined labels six hundred lines away. The same shape as
`funcreach.py`'s two regex bugs: a span that ended in the wrong place.

## Two checks no build can make

An editor that silently stopped being a UTF-8 editor passes the build, the
linkage check and the symbol check. So the phase runs the binary: `'encoding'`
must report `utf-8`, and `gUU` over `à é` must produce `À É` — byte for byte,
`c3 80 c3 89 0a`. It also asserts that **no symbol beginning `iconv` is linked**,
asked of the object and after the sweep, because asking the source beforehand
gets the wrong answer: `iconv_string()` is still there at that point and it is
the sweep that removes it.

## What cannot go, and the rule that finally states why

**Of the six encoding options, only `'charconvert'` can actually be removed.**
The other five each fail for what turns out to be the same reason, arrived at
three times by three different routes:

| | why it stays |
| --- | --- |
| `'encoding'` | `PV_NONE`, but `p_enc` is read in twenty-nine places |
| `'fileencodings'` | `PV_NONE`, not reached by name — and `readfile()` dereferences `p_fencs` |
| `'termencoding'` | `PV_NONE` — and `did_set_encoding()` dereferences `p_tenc` |
| `'fileencoding'`, `'bomb'` | `PV_BUF`, the trap Phase 10 recorded |

**A row is what initialises its global.** Phase 10 found that for a
buffer-local option and guarded on `PV_`; this phase found it for a `PV_NONE`
option reached by *name* (`set_string_option_direct((char_u *)"fencs", …)`,
which answers `E685` and then segfaults) and then again for one reached only
through its variable. The `PV_` test and the name test are both special cases of
the real invariant: **an option is inert only when nothing reads its global any
more.** One whose feature has truly gone has an unread global and the sweep
deletes it a moment later; one that is still read is not inert, it is live code
with its initialiser removed.

**The `PV_` test is unconditional; the other two are `--strict`, and that
distinction is not tidiness.** `PV_BUF` is a property of the row, true whenever
you look. "Nothing reads this global" is only true *after the sweep*, and most
phases drop their options before it — so asking then names the readers the sweep
is about to delete. Phase 2 (`'spell'`) and Phase 5 (`'regexpengine'`) both
fail that question and are both correct. This phase drops after sweeping and so
asks in strict mode. It is the same mistake this phase made twice more — a check
placed one step too early — and it is worth naming because it looks like
rigour.

`'fileencodings'` keeps its row and loses its content — empty is the branch
`readfile()` already takes when a user empties it.

## The delta

A byte-order mark becomes three ordinary bytes at the top of the buffer, which
is what ignoring it means, and the `bomb_on` behaviour case moves because of it.
`'charconvert'` stops existing. **No Ex command moves.**
