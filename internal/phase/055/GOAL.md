# Phase 55 — no option nothing reads

Phase 54 took the options with no variable. These have one, and nothing but the
option machinery reads it — the declaration and the row, `get_varp()` and the
buffer copy for a local one, `set_context_in_set_cmd()`'s completion, and a
`did_set_*` callback that only validates the value or fills a flag set nothing
reads. Setting any of them changed nothing:

    autocompletetimeout  cdhome  cdpath  completetimeout  imcmdline  secure
    shellcmdflag  shelltemp  shellxescape  shellxquote  shortname  ttybuiltin
    warn  xtermcodes  commentstring  completefuzzycollect  completeitemalign
    helpfile  lispoptions  operatorfunc

and sixteen terminal codes the built-in tables and `:set` store and the editor
never sends — `t_8b t_8f t_EC t_EI t_GP t_RB t_RC t_RF t_RS t_SC t_SH t_SI t_SR
t_WP t_XM t_u7`. Their `KS_` enumerators stay, since the built-in terminal tables
still name them.

**Found by reachability, not by name.** Each option's variable was mapped from its
row — `p_xx`, `b_p_xx` from `BV_XX`, `wo_xx` from `WV_XX` (not `w_p_xx`, which a first
count used and so found `'list'` and `'number'` unread), `KS_XX` for a terminal
code — and every mention attributed to its function. Mentions in the plumbing did
not count. **A callback counted only through what it touched:**
`'belloff'`, `'casemap'`, `'display'`, `'jumpoptions'` and `'keymodel'` looked
unused until their callbacks' flag sets were followed to `vim_beep()`, the case
mappers, screen drawing, the jump list and selection; `'modified'`, `'terse'` and
`'wincolor'` act in the callback itself. Those stay. `cfc_flags`, `cia_flags` and
`opfunc_cb` were set and never read, so their options go. No option of the 36 is
named by string anywhere outside the table.

One real reader had to go first: `'cdpath'` was completed as a directory list,
the only use of `p_cdpath`. The phase greps afterwards for every variable, flag set
and callback of the 36, so a reader that appears later fails it. It checks `:set
sw` still works and that `'shelltemp'`, `'commentstring'` and `t_EI` are unknown.

## The delta

**None the harnesses record** — no case sets one. Measured: 110,025 →
**109,655 lines**.
