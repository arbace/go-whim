# Phase 60 — no suffix, case, delay, verbose-file, debug or filter-program options

Seven options whose default is the only value anything could still act on.
`'suffixes'` ordered wildcard matches, and wildcards have not expanded since
Phase 7 removed globbing, so `match_suffix()` and its two reordering blocks go.
`'fileignorecase'` is off, and its five tests fold as false. `'autocompletedelay'`
is 0, so `inchar_loop()`'s delay was never pending. `'verbosefile'` is empty, so
the file is never opened: `redir_write()`, `redirecting()` and the
`verbose_enter`/`verbose_leave` family fold, and `fopen` leaves the libc symbols.
`'debug'` is empty, and its tests in `emsg_not_now()`, `emsg_core()` and
`vim_beep()` fold.

**`'formatprg'` and `'equalprg'` were suspicious, and dead.** Their only effect was
to make `gq` and `=` build a `:{range}!prg` line, and `:!` has been `ex_ni` since
Phase 44 — so a non-empty value turned a working operator into an error. `gq` and
`=` take the internal path unconditionally now, and `op_colon()` loses its
indent and format branches. `get_varp()`'s `'equalprg'` case has the same
missing parentheses as `'keywordprg'`'s in Phase 56 and is removed by hand.
`'formatoptions'` and `'formatlistpat'`, suspected with them, are live —
auto-wrap, comment leaders, `gq`, `J` and numbered-list indent read them — and stay.

The phase checks the seven are unknown and that `gqq` with `tw=4` still breaks
`aaa bbb` into two lines.

## The delta

**None the harnesses record.** Measured: 102,166 → **101,826 lines**; libc symbols
81 → 80.
