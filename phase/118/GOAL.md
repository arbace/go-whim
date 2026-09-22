# Phase 118 — the core calls nothing but the host

`phase/118/edit.sh` and `phase/118/check.sh`, `stage 118`, `package host`. The three
libc functions the core still **called** for itself go to the host: `malloc`, called by
`lalloc()`; `free`, called by `vim_free()` and `update_wincolor()`; and `write`, called by
`mch_write()` — and, since phase 117 rewrote `realloc` as a malloc, a copy and a free, by
`ga_grow_inner()` and `get_keystroke()` as well.

Above the first `#include`, which since phase 110 **is** the boundary, `malloc` was 4
mentions, `free` 5 and `write` 2: each a declaration in the core's one run of ordinary,
non-`static` declarations, plus its call sites. All three are **0** now, and below the
boundary they are 1, 2 and 2 where they were 0, 1 and 1. Three `static` prototypes go in
the block phase 108 made one run of eleven, so the core → host boundary is still **one**
block of declarations and not a block plus three:

```c
    static void *host_alloc(usize n);
    static void host_free(void *p);
    static int  host_write(const char *s, int len);
```

and three definitions below the boundary make the same libc calls with the same
arguments.

## Not one of those counts is written down, and the reason is that they were once

This phase first asserted `malloc` 2, `free` 3 and `write` 2, the counts measured on the
boundary it was written against. **Phase 117 then rewrote `realloc` at two core sites and
took them to 4 and 5, and the anchors refused** — which is what counted anchors are for,
and better than the alternative. But a count is a fact about a tree that was measured and
a **partition** is a fact about the tree that arrives, so both programs now assert the
SHAPE: every mention of each name above the boundary is its own declarator or a call of
it; the declaration goes, every call is rewritten, and how many there are is read off the
text. A mention that is neither — an address taken, a variable of the name — **refuses**
rather than surviving into a file whose declaration is gone. That is `CLAUDE.md`'s rule
under *Rename a name across the whole file*, and it is what made phase 117 cost this phase
a re-run rather than an edit.

## The wrappers are faithful and not improved

This is the trap the phase could have fallen into with no recording seeing it.
`mch_write()` is `vim_ignored = (int)write(1, (char *)s, len);` — **one** `write(2)`, no
loop, the count assigned to the variable this tree keeps for results it means to ignore. A
short write LOSES those bytes today and `host_write()` loses them too: a wrapper that
looped would be a behaviour change in a phase that declares none, and output that silently
truncated under load is the worst outcome available here. `host_alloc()` returns what
`malloc()` returned, `nullptr` included, so `lalloc()`'s `clear_sb_text()` /
`do_outofmem_msg()` failure path is reached exactly as before; `host_free()` calls
`free()`, so it is null-safe for `free()`'s own reason — the core does not rely on that
(`vim_free()` tests `x != nullptr`, `update_wincolor()` frees only the arm it allocated,
and **0 of the corpus's 22,417 frees are null**) but the wrapper inherits it rather than
adding a test. `host_write()` **drops the descriptor** because its two neighbours on this
boundary already have: phase 103's `musl_read_input(char *, int)` reads fd 0 inside the
host and phase 104's `host_message(msg, len, err)` chooses its stream from a flag. A
descriptor is the host's idea of where the screen is; `host_write(s, len)` is the core's.

## Nothing is freed and the phase says so as an equality

`nm -u` is the same set in and out, a `comm` empty in both directions — 17 names with
the core's flags, 18 as `tools/symbols.sh` counts — with `malloc`, `free` and `write`
still in it. That is this pipeline's own rule read back: **a symbol leaves when its last
CALLER leaves the file**, and inside one translation unit moving a call from the core into
the host moves no caller out. Phase 111 is the contrast, freeing `gettimeofday` because the
last caller went with it.

**What does move is the thing `nm -u` cannot show.** `make editor.c`'s cut — 77,899 lines
either side, a byte prefix of the file, 0 errors under `-fsyntax-only` — has a warning set
that IS the core → host interface, every name `used but never defined`, and it goes from
**14 names to 17**. `host_alloc`, `host_free` and `host_write` arrive and nothing leaves.
An implicit libc dependency hidden in a bare declaration becoming an explicit named call
is the point of the boundary, and an interface growing by exactly three is what that looks
like. The input's set is computed in the check at run time and never written down: a
written list has gone stale twice in this pipeline already.

## The declared delta is nothing at all, and here the recording is STRONG evidence

Two full `tools/zrecord.sh` recordings, `diff -r` empty across all 106 records. `lalloc()`
and `vim_free()` are on the path of essentially everything the editor does and
`mch_write()` is every byte it draws, so the corpus hammers all three. Measured on an
instrumented build of this phase's own output, over the 102 screen cases and marking every
one of them: **53,848** `host_alloc` calls with the largest 319,968 bytes; **22,417**
`host_free` calls with 0 null; **1,012** `host_write` calls with the largest 2,063 bytes
and 0 short. A by-hand session, `+normal 200000ax`, makes 400,475 `host_alloc` calls
against 479 for `ihello world<Esc>`, and frees 400,188 of them.

**And it can fail, once for each function — with two controls that move nothing, reported
and not hidden.** `host_alloc` returning `nullptr` always moves 106 of 106 records;
refusing only allocations above 200,000 bytes — in this editor exactly ONE, the screen —
moves 100 of 102 screen cases, the survivors being `ctrl_c_clean` and `ctrl_c_changed`,
which exit before a key is looked up. `host_write` writing HALF THE BYTES moves 102 of
102, and `host_write` writing every byte and REPORTING half moves **0 of 102**, which is
the phase's claim about the return value measured rather than argued. `host_free` doing
nothing at all moves 0 of 102, because a leak is invisible to a 106-record corpus; the
instrument is what says `host_free` is called, and the check says so in those words
instead of presenting a silent control as evidence. `host_free(nullptr)` on every draw
moves 0 of 102 and writes nothing.

**The `static` trap is built both ways.** `nm --extern-only --defined-only` is still
exactly `main`: with the keyword off the three prototypes gcc refuses — *"static
declaration of 'host_alloc' follows non-static declaration"* — and with it off the
prototypes AND the definitions the build is silent and the object defines `host_alloc`,
`host_free`, `host_write` and `main`. Deleting the three prototype lines gives 9 errors
naming all three, which is what makes them load-bearing rather than decorative.

## Measured

| | input | after |
| --- | --- | --- |
| lines | 79,786 | **79,804 (+18)** — three prototypes and three five-line definitions with their blanks |
| `malloc` / `free` / `write` above the boundary | 4 / 5 / 2 | **0 / 0 / 0** |
| the same three below it | 0 / 1 / 1 | **1 / 2 / 2** |
| the core's plain libc prototype block | 5 lines | **2** — `getpid` and `kill` |
| `make editor.c` | 77,899 | **77,899**, 0 directives, 0 errors |
| the boundary | 14 names | **17**, computed at run time on both sides |
| `nm -u` | 17 | **17, the same set** — `malloc`, `free` and `write` all still in it |
| external symbols | `main` | `main` |
| binary | 782,760 | **782,760 bytes, 206,588 of them different** |
| `cmdnames[]` / `options[]` rows | 98 / 107 | 98 / 107 |
| records that moved | | **0 of 106**, against 53,848 + 22,417 + 1,012 instrumented calls |

## Its placement

`stage 118`, `package host 100 101 102 103 104 113 115 118` — a thing the core did for itself
becomes a thing it asks the host to do, declared in the one host block and defined below
the boundary. Four `uses`: `seed:83` and `harness:86`; `boundary:108`, a `static` forward
declaration above and a `static` definition below being all a direct call to the host
needs, which is that phase's finding; `boundary:110`, because *above the boundary* and
*below it* are a LINE NUMBER in this check and phase 110 is what made that line the split;
and `boundary:106`, because `host_alloc(usize n)` is spelled in the core's own type names
and a signature in `size_t` would name something no header above the boundary declares.

**Six `apart` lines, every one a measurement.** 109 and 110 carry a nine-entry `PROTOS` list
and require each `\n<prototype>\n` exactly once: on this output **two** are at 1 — `getpid`,
`kill` — and **seven** at 0, the three phases 114 and 115 took, `realloc` which 117 took, and
this phase's three; and 110 builds a control by putting `static ` in front of the `malloc`
prototype, which is no longer there to put it in front of. 110 and 111 write the boundary out
as a list of names and require the cut's warning set to be exactly it, and this phase makes
it seventeen where 115 made it fourteen.

**`apart 117 118` was measured as a REAL SHARED STAGE** — q116 restored, both edits, one sweep,
both checks — and phase 117's check stops four ways. The first two are its prototype block:
*the core's plain libc prototype block is 2 lines and the input's was 6, a difference of 4
where 1 was expected*, and *the prototype block lost [long write(…) / void \*malloc(…) /
void \*realloc(…) / void free(…)] and not realloc's line alone*. The other two are
`apart 105 106`'s lesson landing on the phase that had just taught it: *ga_grow_inner's
rewrite is not in the output exactly once*, and the same for `get_keystroke`'s, because
phase 117's check matches the code it wrote VERBATIM — `pp = malloc(new_len);`,
`free(gap->ga_data);` — and this phase renames exactly those calls. **A check that quotes C
is a dependency on the spelling**, and here the quoting phase and the respelling phase are
adjacent. One direction only: phase 118's check was then run on that same tree and every
part of it held.

`apart 113 118` is the same rule one level deeper, and it is kept from an earlier base
because it is the clearest instance of it: phase 113's check writes
`write(2, "T-cleos\n", 8);` INTO THE CORE to build an instrumented control, and relied on
the core's own declaration of `write` — so run as a pair it does not compile at all.
**A check that CALLS libc from above the boundary is a dependency on the core still
declaring it.** There is **no `need 118`**, measured three times — on phase 113's unswept
output, on phase 115's and on phase 117's — the partition holding identically each time.

## What this phase does not yet claim

The sentence this arc has been building to — that the core calls no libc function at all,
every outward call a `musl_` or a `host_` — is **not** stated, because it is not true of
this tree. `getpid` and `kill` remain, two lines of prototype and three call sites:
`mch_get_pid()`'s `getpid()`, and `vim_handle_signal()`'s `kill(getpid(), got_signal)`,
which re-raises a deferred deadly signal. The phase is written so that the claim becomes
true without another edit — the block is FOUND rather than assumed, the lines this phase
owns are taken out of whatever run holds them, the residue is printed, and when the residue
is empty the edit drops the block's trailing blank line with it — but **the phase that
takes those two is the one that gets to write it down.**

## What whim-vim is after phase 118

```
whim-vim.c        79,804 lines          from whim-vim.c's 86,614  (-6,810, 7.9%)
                  77,899 above the boundary, 1,905 below it
functions         1,759
type definitions  908
DWARF enumerators 1,189
cmdnames[] rows   98    (create_cmdidxs floor 80; 18 rows of margin)
nv_cmds[] rows    194   (nvidxcheck: a permutation)
options[] rows    107, 95 distinct globals  (orphanopts floor 80; 15 of margin)
#include          11, at line 77,901, and NOT ONE DIRECTIVE above them
core -> host      17 names: vim_snprintf, host_exit, host_message, host_time,
                  host_alloc, host_free, host_write, ten musl_*
libc prototypes   2 in the core: getpid kill
libc symbols      17 with the core's flags, 18 as tools/symbols.sh counts
binary            782,760 bytes, EXEC, no INTERP, no dynamic section, no relocation
declared delta    nothing since phase 94 -- 2 stderr-moved and the records of 87 to 94
make editor.c     77,899 lines: 0 directives, 0 errors, 17 warnings, all of them
                  `used but never defined` and all of them the interface
```

**Fifteen phases in a row have declared nothing** — 104 through 118 — and the kinds of
evidence that stand in for a recording are named rather than counted here; the one
numbered list is `CLAUDE.md`'s, and the taxonomy is written out at the end of phase 122.
**Phase 116's is a kind new with it, and it is phase 86's**: the phase changes no source at
all, so nothing about the editor's behaviour *can* have moved, and what has to be argued
instead is that the **comparison** moved safely. 117 and 118 are the strongest instance of a kind the pipeline already had —
two byte-identical recordings — because what they touch is on the path of everything:
117's `ga_grow_inner` at 4,289 calls a recording, 118's `lalloc`/`vim_free`/`mch_write` at
53,848, 22,417 and 1,012 over the screen cases, each with a control that moves 102 of 102
to say so. **Fifteen of the seventeen libc symbols are now
called from the host and from nowhere else**: `malloc`, `free` and `write` joined them
here, `time` at 32 and `gettimeofday` at 28, and `__errno_location` is gcc's own for the
host's `errno`. **The two that are not are `getpid` and `kill`.** What the core still does
for itself is one re-raise of a deadly signal and one `getpid()` that fills a `b0_pid`
nothing reads.
