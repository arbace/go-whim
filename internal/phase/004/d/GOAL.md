# Phase 4d — one buffer, structurally

*Formerly phase 71, now part d of phase 4. The other phase numbers in this file are the old
numbering, as it was written: `doc/PHASES.md` maps them.*

**A record now.** Phase 6 calls `whim71` by name, after `whim68`, on the text phase 3
swept (`doc/PIPELINE-REFORM.md` §7, the drops brought to the front): the
program is here, and the phase has no plan entry. It applied there as
written. Its body for `buflist_findpat()` supersedes phase 62's edit of the `'buflisted'` test there. What follows is the account of the cut as it was made here.

**The invariant was already true; this phase removes the machinery that pretended
otherwise.** Phase 69 allowed at most one file argument, phase 70 made `:e` reuse the
one buffer, and every buffer Ex command had been retired long before that — all 24
rows (`:buffer`, `:buffers`/`:ls`/`:files`, `:bnext`, `:bprevious`, `:bNext`,
`:bfirst`, `:blast`, `:brewind`, `:bmodified`, `:bdelete`, `:bunload`, `:bwipeout`,
`:bufdo`, `:ball`, `:badd`, `:balt`) already read `ex_ni`, and `do_buffer`,
`do_bufdel`, `ex_buffer`, `ex_bufdo` and `ex_listdo` do not exist. So nothing can
make a second buffer: `win_alloc_first()` makes the one buffer at startup, *before*
`command_line_scan()`, and `buflist_add()` then names that same buffer through
`BLN_CURBUF`.

`buf_valid()` becoming `return buf == curbuf;` is the keystone — it makes
`set_curbuf()`'s `enter_buffer(lastbuf)` fallback unreachable, and the rest of that
function's other-buffer handling with it.

## Three things named b_next are not the buffer list

A regex over the name would gut the editor, so every edit is scoped by function:

- `buffblock_T.b_next` — the typeahead and redo chain: `bh_first`, `redobuff`,
  `old_redobuff`, `readbuf1`, `readbuf2`. About thirty sites.
- `free_buffer()` — `buf->b_next = au_pending_free_buf`, a free list.
- `buf_T.b_next`/`b_prev` — **this** is the buffer list, and only this.

`au_pending_free_buf` turned out to be written in two places and **read in none**:
nothing ever drained that chain, so the `autocmd_busy` branch leaked the buffer and
always had. It goes with the field it linked through, and `free_buffer()` now always
frees immediately.

## break binds to the loop, not to the braces

Folding `for ((buf) = firstbuf; …)` into `buf = curbuf;` rebinds any `break` or
`continue` in the body to whatever loop encloses it next — and brace depth has
nothing to do with which statements those are. `getout()`'s `break` sits two `if`s
deep and still bound to the walk; `buflist_findpat()`'s body has a `break` **and** a
`continue` that bind to the walk while a third `break` correctly belongs to an inner
window loop.

The first version folded both anyway. `getout()` failed to compile, which is the
cheap outcome. `buflist_findpat()` **compiled fine and changed behaviour** — its two
statements silently rebound to the enclosing `for (;;)` retry loop — and sat
undetected through three dry runs. So `fold_walk()` now refuses a body whose
`break`/`continue` is not inside a nested loop or switch of its own, and the two
functions are rewritten rather than folded. An earlier version of that guard tested
brace depth and would have passed `getout()`; depth is the wrong question.

With one buffer `buflist_findpat()` has nothing to retry — one candidate, so the
"more than one match" (`-2`) arm is unreachable by construction.

## The boundary compiles (2026-10-03)

Nothing compiled the boundaries until `whim build --check` was made to, and
q004-q019 had **two errors** of this part's making, which gcc 15's defaults
make errors and not warnings (C23 has no implicit declaration):

- the body written for `buflist_findpat()` reported a failed match with
  `semsg()`, which the front had already taken: by phase 4 every `semsg` call
  in the text is `vim_snprintf()` into `IObuff` and `emsg(iobuff_or(...))`.
  The body now says it that way, as the text around it does. The error lasted
  to q022: phase 23 takes `buflist_findpat()` whole;
- `autowrite_all()` kept its walk over the list, `for ((buf) = firstbuf; ...;
  (buf) = (buf)->b_next)`, while this part deleted `b_next` -- the walk was
  missed, not kept. Its body is now the walk folded: `curbuf` written when it
  is changed. The walk's restart at `firstbuf` when a write wiped the buffer
  it was on ends it with one buffer, so the restart and its `bufref` go, and
  `firstbuf` goes with its last reader. Phase 20 takes `autowrite_all()`'s
  callers, and the sweep it.

Measured, on a build in order that gave `whim-vim.c` byte for byte: q004
96,340 -> **96,329 lines**, and it compiles and links with the one line, as
every boundary after it does; q030 onwards are what they were, byte for byte
(`internal/phase/boundaries.md`).

## What stays

`buf_hashtab` and `buflist_findnr()`, because five live callers still look a buffer
up by number: `eval_vars`, `setmark_pos`, `check_changed_any`, `buflist_nr2name` and
`buflist_getfile`. Collapsing that to a `curbuf` test is a separate step.
`DOBUF_WIPE_REUSE` keeps its enum and the two tests that name it — no caller ever
passes it, which is what made `close_buffer()`'s wipe splice unreachable; that splice
was guarded by `(b_prev != NULL || b_next != NULL)`, already false with one buffer.

## The delta

**None**, and `whimdelta.sh` confirmed it. The buffer commands were already `ex_ni`,
so no `exsweep` row can move.

**A probe that cannot fail proves nothing, again.** The buffer-local mapping probe
was written `+normal! Q` — and `normal!` suppresses mappings *by definition*, so it
could never fire on any build. Calibrated against q70: `x` with the bang, `x!`
without it, and the phase-71 build gives `x!` too. The bang is gone and the comment
says not to put it back. It also corrected a belief: that walk is
`check_map_keycodes()`, which feeds `add_termcap_entry()`, **not** mapping lookup —
a mapping is found through `curbuf->b_maphash[]`, which never touches the list.

Measured: 93,127 → **92,749 lines**.

**On the graph** (B3a, `doc/GRAPH-MIGRATION.md`): this part's program is a graph step of phase 4, on `crefactor/graph`'s verbs, its report the text version's line for line and the phase's boundary byte for byte (`whim-build-check`).
