# Phase 56 — no shell, runtime or keyword-program options

Six options whose readers survived only in machinery with nothing left to serve.
**`'shell'`, `'shellquote'` and `'shellredir'`**: no shell is ever run — `call_shell()`
and `mch_call_shell()` went long before — so `'shell'` only chose the default of
`'shellredir'` in `set_init_3()` and whether filename escaping doubled a `!` for csh,
and `'shellquote'` only wrapped `do_bang()`'s command line. **`'runtimepath'` and
`'packpath'`**: there is no runtime to find. Their readers were the completion of
`:colorscheme`, `:compiler`, `:ownsyntax`, `:setfiletype`, `:packadd` and
`:runtime` — every one of them `ex_ni` — and of `:set ft=`, which listed runtime
syntax, indent and ftplugin names. **`'keywordprg'`**: `K` is gone; only `:set kp=`
defaulting to `:help` read it. Each reader is folded before the rows go, and the
phase greps afterwards for every variable and helper.

**One plumbing site had a shape `droplocal.py` did not know.** `get_varp()`'s "local
if set" case for `'keywordprg'` reads `&curbuf->b_p_kp`, without the parentheses
every other such case has, so its two mentions counted as readers and the tool
refused. The phase removes that case by hand first; the shared tool is unchanged,
so no other phase's key moved.

## The delta

**None the harnesses record.** Measured: 109,655 → **109,039 lines**.
