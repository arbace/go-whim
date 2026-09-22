# Phase 61 — no window title

`'title'`, `'titlelen'`, `'titleold'`, `'titlestring'`, `'icon'` and `'iconstring'`
go, and with them everything that set or restored the terminal's title:
`maketitle()` and its thirteen callers, `need_maketitle` and the six places that
asked for an update, `resettitle()`, `mch_settitle()`, `mch_restore_title()` in
`:stop`, exit, a terminal change and `value_changed()`, `set_title_defaults()`,
`term_settitle()`, the X11 title and icon probes, and the title-stack push at
startup and pop at exit. The editor no longer writes to the terminal's title at
all. The `t_ts`, `t_fs`, `t_ST` and `t_RT` codes stay, with the other terminal codes.

**Two of the phase's own checks were wrong first, and both failed loudly.** One
guarded `do_exedit()`, where `n` held the argument index only to decide whether to
update the title, by requiring no other mention of `n` — but `n` also saves and
restores `readonlymode` around `:view`. It now requires exactly those three
mentions. The other counted five `need_maketitle = TRUE` assignments where there
are six: `maketitle()` sets it itself before an early return. Each was tried first on
a copy of the Phase 59 boundary, since this phase touches nothing Phase 60 does.

## The delta

**None the harnesses record.** Measured: 101,826 → **101,188 lines**.
