# editor.c → editor.go: what the transpilation found

`editor/editor.go` is `editor.c` transpiled by hand, faithfully: 1,705 C
functions as 1,705 Go functions with the same names and control flow, 61,883
lines, plus `editor/crt.go` (the C runtime it is written against, 323 lines)
and `editor/host.go` (the host, from the Go runtime, 1,704 lines).

**It works.** `go build ./editor` gives an editor that records the same 122
files as the C binary built from `whim-vim.c` — 102 screen cases, 111 Ex rows,
30 command lines, the pty scenarios, 19 terminals, the memline corpus — byte
for byte, and `tools/zerodelta.sh … --phase 128` accepts it as exactly phase
128's declared delta. The control: the same Go build with one message word
changed moves 95 of the 122 files and the delta check refuses it.

## How it was made

1. `tx/skel` (Go, on `modernc.org/cc/v4`) parsed `editor.c` and decided, by a
   union-find over every pointer's flows, which pointers must walk (`Ptr[T]`,
   an allocation and an offset) and which stay Go pointers (`*T`); from that it
   generated the Go types, the globals and all 1,723 signatures.
2. Seventeen agents transpiled one chunk of functions each, one the global
   initial values, one the host, in parallel, to `tx/CONVENTIONS.md`, each
   file checked alone against the skeleton.
3. The pieces compiled together at the first attempt. One runtime bug was
   found by running it (finding 1); the recording then matched.

`unsafe` is in two places only: `crt.go`, so a C pointer can be a comparable
Go value, and `host.go`, to hand pointers to `ioctl` and `rt_sigaction`.

## What C relied on that Go cannot say — each a candidate phase

Every item below is something the C does that the transpilation had to work
around. Removing it in C, as a pipeline phase before the transpilation, makes
the Go simpler and the patch smaller.

1. **`p_emoji` is `char_u *` used as a boolean option.** The options table
   writes `*(int *)varp` into the pointer's storage and `utf_char2cells` tests
   the pointer for non-NULL. The one runtime bug of the first run. Fix in C:
   declare it `int`.
2. **Option variables through `char_u *`.** `vimoption_T.var`, `def_val`,
   `optset_T.os_varp`, `get_varp()` and every `varp` hold the address of an
   `int`, a `long` or a `char_u *`, or a number cast to a pointer (`def_val`,
   `VAR_WIN = (char_u *)-1`). In Go they are `any`. A phase could give the
   table a typed slot per kind.
3. **`container_of`.** `buflist_findnr` and the highlight-name table recover a
   struct from a hash key inside it by subtracting the key's offset. Go keeps
   an owner registry (`SetOwner`/`Owner`). A phase could store the struct
   pointer beside the key.
4. **Struct prefix inheritance.** `bhdr_T` is cast to the `PTR_BL`/`DATA_BL` it
   heads, and `regprog_T` to `bt_regprog_T`. Go keeps registries
   (`f07_blocks`, `bt_regprog_of`). After phases 126-128 only the memline
   header cast remains of the first kind; `regprog_T` has one engine left,
   so the two structs can be one.
5. **A garray of mixed types.** The backtracking engine's `regstack` is one
   byte array holding `regitem_T`, `regstar_T` and `regbehind_T` back to back,
   limited by `'maxmempattern'` in bytes. Go keeps the objects in a side table
   and the byte accounting with x86-64 C sizes (40, 32, 376).
6. **A garray smuggled as `char_u *`.** `get_input_buf()` returns a
   `garray_T *` cast to `char_u *`, stored in `tasave_T.save_inputbuf`.
7. **`void *` walked.** `sort_strings` qsorts a `char_u **` it receives
   through `void *`; the flow analysis cannot see a walk through `void *`,
   and its first signature was wrong (fixed by hand). `ga_data` is `void *`
   everywhere; Go types it at the first cast (`GaData[T]`), and `ga_itemsize`
   means nothing.
8. **`sizeof` of structs as byte counts**: `exestack.ga_itemsize`, the
   highlight tables, the regstack limit. Go has no layout to take the size of.
9. **Every `vim_free` is dropped** — the garbage collector is the allocator —
   and every allocation-failure branch is unreachable. A phase could remove
   both from the C.
10. **Sentinels.** `(pos_T *)-1` is compared three times and produced nowhere:
    dead code in C already.
11. **`goto` into `switch` cases** (`edit`, `regatom`, `regrepeat`,
    `check_termcode`) had to be restructured; Go cannot jump into a case.
12. **Signals.** The C host runs `deathtrap` in a signal handler; the Go host
    receives signals on a goroutine and runs their effect at the next wait or
    read of input, which the recording cannot tell apart.
13. **`__DATE__ " " __TIME__`** in the version string became the constant the
    pinned build (`SOURCE_DATE_EPOCH=0`) produces.

## Which findings are phases now

Phases 129 to 143 (`WHIM-GOAL.md`, Part II) remove from the C what the
transpilation worked around. Each declares no behavioural delta; 142's change
is to stderr, which the recording excludes, and its check measures it:

| Finding | Phase |
| --- | --- |
| 1, `p_emoji` | 129, `p_emoji` is an `int` |
| 10, `(pos_T *)-1` | 130, the tests go |
| 6, the input buffer as `char_u *` | 131, it is a `garray_T *` |
| 9, the frees | 132, nothing frees; 134, the blocks that held only a free fold |
| 3, `container_of` | 133, one buffer needs no hash table; 140, highlight groups are found in their array |
| 4, `regprog_T` / `bt_regprog_T` | 135, one program type; 136, the engine called directly |
| 7, `void *` walked (`qsort`, `bsearch`) | 139, the core sorts and searches typed arrays |
| 11, `goto` into `switch` (`regrepeat`, `regatom`) | 141, `regrepeat()` does not jump into a case; 143, `regatom()` has no goto |
| 13, `__DATE__ " " __TIME__` | 142, the version names no build date or time |
| the eval value types the transpilation carried (`typval_T`, lists, dicts, classes) | 137, the changedtick is a number; 138, no parameter carries an eval value |

Not yet phases:
- 2, option `varp`;
- 4's memline header;
- 5 and 8, the regstack and `sizeof` accounting;
- 7's `ga_data`;
- 9's allocation-failure branches. These are dead only where the size is not
  0: `host_alloc()` never returns NULL, but `lalloc(0)` does, after an
  internal error, so each branch needs its size proved non-zero;
- 11's `edit` and `check_termcode` (the last jumps into an `if`
  body, not a case);
- 12, signals.

`editor/editor.go` is still the transpilation of phase 128's core.

## Not done here

The Go is faithful, not idiomatic: `Ptr[T]` everywhere a C pointer walked,
C's integer widths, `goto`, `B2i`. Idiomatic Go is the phases that follow,
each measurable the same way: `go build ./editor` and the recording.
