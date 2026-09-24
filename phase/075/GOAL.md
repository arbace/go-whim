# Phase 75 — no autocommands

**Proved by absence, not inferred from the command table.**
`first_autopat[NUM_EVENTS] = { NULL }` is the **only** write to that array in the
whole file; every other mention reads it. No autocommand pattern can ever be
registered. `:autocmd`, `:augroup`, `:doautocmd`, `:doautoall` and `:noautocmd` were
already `ex_ni`, but that is the weaker argument — the array being write-once-to-NULL
is the strong one.

It follows that `apply_autocmds_group()` was already `return FALSE;`, that its three
one-line wrappers made **all ~74 dispatch sites no-ops**, that
`has_cursormovedI`/`has_textchangedI`/`has_textchangedP` were always FALSE, and that
`au_cleanup`, `au_remove_pat`, `au_del_cmd` and `aubuflocal_remove` walked a
permanently empty list.

**The invariant is asserted in the phase**, and the assertion was *proved able to
fail*: injecting a synthetic `first_autopat[0] = NULL;` makes it fire. A first
version matched `first_autopat[^\n;]*=` and reported three writes that were the `!=`
of the `has_*` predicates — it spanned the subscript and landed on the comparison. An
assertion that cries wolf is worse than none, because the temptation is to loosen it
until it passes.

## Two sites rewritten, not folded

`close_buffer()` — the label `aucmd_abort:` sat **inside** the block guarded by
`apply_autocmds(EVENT_BUFWINLEAVE, …)`, with three `goto`s targeting it, two from
outside. Folding would have orphaned them: the phase-71 break-rebinding hazard
wearing a label instead of a loop. All three gotos fold away with their conditions,
so the label goes too and `if (abort_if_last)` carries the abort directly.

`buf_write()` — the 132-line block **looks** like pure scaffolding (`aco_save_T`,
`aucmd_prepbuf`/`restbuf`, `set_bufref`, `did_cmd`) but computes `buf_ffname`,
`buf_sfname`, `buf_fname_f` and `buf_fname_s`, which are **read a hundred lines
later** to restore `ffname`/`sfname`/`fname`. Deleting it wholesale would have broken
`:w` on a renamed buffer. The scaffolding goes; the flags stay, and a `:w dst.txt`
probe guards it.

## Classification had to be done by hand

A scan got two sites **backwards**. `7712` and `7741` read
`if (!(did_cmd = apply_autocmds_exarg(...)))` — negated *with an embedded assignment*
— so they are always **TRUE** (`fold_always`), not always false. A
`startswith("if (!apply_autocmds")` test cannot see the `!(var = …)` shape, and
folding them the other way deletes the branch that runs.

`did_cmd` then needed its **two readers folded before its declaration was removed**;
doing it the other way round leaves them undeclared, which is precisely the compile
error a first version produced.

## Ordering is part of the edit

Three edits depend on an *earlier* edit having created their target, and must run
after it: `buf_write`'s scaffold and `set_termname`'s husk only take their final
shape once the blanket dispatch removal has emptied them. Pre-flighting those against
an already-swept tree confirms the shape while saying nothing about when it becomes
valid — the fix is to **replay the script's own steps** and read the result.

`set_termname`'s husk is removed by brace matching keyed on `buf = curbuf;`, not by a
literal: the literal was transcribed twice from a post-sweep tree where `deadsweep`
had already dropped the now-unused `aco_save_T aco;`. The helper **refuses** a block
that does any real work, and that refusal was demonstrated before being relied on.

## What survives, deliberately

`block_autocmds`/`unblock_autocmds` keep four caller pairs that are not autocommand
code — `set_string_option_direct_in_win`, `u_undoredo`, `win_alloc` — so both stay,
and `autocmd_blocked` stays with them as a **write-only counter** belonging to the
combined phase. Asserting any of the three reached zero would fail the phase on its
own terms.

## The delta

**None**, and `whimdelta.sh` confirmed it. Nothing could fire an autocommand, so
removing the dispatch cannot change what the editor does.

A `:%!sort` probe was written and **discarded**: `!` went in phase 64, so it fails on
the baseline too and would have measured nothing. The eight that remain — load,
write, `:w name`, `:e`, `:g`, `:s`, `:m`, undo, insert — were each calibrated against
q74 first.

Gone: the `EVENT_` enum (123 enumerators), `event_tab` (127 rows), `AutoPat`,
`AutoCmd`, `AutoPatCmd_T`, `active_apc_list`, `first_autopat`, `last_autopat`,
`aucmd_prepbuf`, `aucmd_restbuf`, `aco_save_T`, `getnextac`, `auto_next_pat`,
`event_nr2name` and all four dispatch wrappers.

Measured: 90,972 → **89,804 lines**.
