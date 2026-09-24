# Phase 77 — no buffer-name argument matching

`do_one_cmd()` computes

```c
ni = (!(cmdidx < 0) && (cmd_func == ex_ni || cmd_func == ex_script_ni))
```

— "this command is not implemented" — and **seven** later checks consult it before
doing work. One does not: the `EX_BUFNAME` pre-dispatch block, guarded only by
`!(cmdidx < 0)`, which compiles a regexp and matches it against the buffer to turn
`:buffer foo` into a line number.

**Every command carrying `EX_BUFNAME` is `ex_ni`** — `:buffer`, `:bdelete`,
`:bunload`, `:bwipeout`, `:checktime`, `:sbuffer` and `:pbuffer`. That last one is
worth noting: an earlier hand grep found only six because `:pbuffer`'s row spells the
handler with surrounding spaces (` ex_ni `). The phase counts the rows **dynamically**
and asserts every handler is `ex_ni`, so it is right regardless of how many there are.

**The edit is a fold, not a guard.** Adding `&& !ni` would leave a block that can
still never run — dead weight wearing a condition. The condition is false for every
command that reaches it, so `fold_never` removes it outright and `buflist_findpat`
loses its only caller.

## A goto statement, not a label

The block contains `goto doend;`, and that is safe: `doend` is `do_one_cmd`'s shared
exit label with 27 gotos targeting it, so this removes a goto **statement**. The
distinction is the one that mattered for `readfile`'s `theend` in phase 75, and for
`close_buffer`'s `aucmd_abort`, where the label itself sat inside the fold and three
gotos would have been orphaned.

## What went by cascade

`buflist_findpat` (71 lines), `file_pat_to_reg_pat` (167), `buflist_match` (13) and
`fname_match` — the last reachable only through the pattern matcher and not
predicted. 306 lines against an estimate of 251. Nothing is deleted by name here;
removing the one call site orphans them all and the sweep takes them.

## The delta

**None**, and `whimdelta.sh` confirmed it. `:buffer foo` already exited 1 with nothing
on stderr — `ex_ni` sets `eap->errmsg` rather than printing, and an `exsweep` row is
`exit= left= err=`. Measured on q76: exit 1, empty stderr, file written either way.
So the gain is code, not behaviour, and the phase says so rather than claiming a
user-visible fix.

`:buffer nosuchname` is therefore **not used as a discriminator** — only as a
does-not-crash check. The probes that can actually fail exercise what survives: the
load, a write, `:e` naming a file (the argument path *next to* the one removed), and
`:g` taking a pattern. All were calibrated against q76 first.

It passed its first dry run.

Measured: 89,713 → **89,407 lines**.
