# Phase 2a — no window title

*Formerly phase 61, now part a of phase 2. The other phase numbers in this file are the old
numbering, as it was written: `doc/PHASES.md` maps them.*

**Its program runs at phase 1 now.** Phase 1's front calls `whim61` by name
(the pipeline reform's D8, `doc/PIPELINE-REFORM.md` §7), on the seed, after
the options are dropped: the program is here, and the phase has no plan
entry. There `need_maketitle = TRUE;` is written seven times, since what phases
2-60 took is still in the text. What follows is the account of the cut as it
was made here.

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
