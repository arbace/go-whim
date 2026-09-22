# Phase 35 — no scripts, no session, no autocommands

Three things that are one question: can the editor be told to do something
later, or somewhere else, by a file? A script is commands read from a file, a
session is a script the editor wrote about itself, and an autocommand is a
command registered now to run when an event happens. None of them has anywhere
to come from: nothing is installed, no vimrc is searched for, and since Phase 18
nothing at all is read at startup.

| | what goes |
| --- | --- |
| scripts | `:source` `:scriptencoding` `:scriptversion` `:vim9script` `:legacy`, the `vim9cmd` modifier, `'loadplugins'` |
| the session | `:redir` `:sleep` `:smile` `:sandbox`, `-S`, `-s file`, `-w`/`-W file`, `'sessionoptions'` `'viewoptions'` `'viewdir'` |
| autocommands | `:autocmd` `:augroup` `:doautocmd` `:doautoall` `:noautocmd` `:filetype` `:setfiletype`, the engine, `'eventignore'` `'eventignorewin'` |

The sixteen rows go to `ex_ni`. `tools/nosession.py` removes what a row cannot:

- **The modifiers are parsed by name.** `parse_command_modifiers()` matches
  `legacy`, `noautocmd`, `sandbox` and `vim9cmd` before the table is consulted,
  so retiring a row changes nothing about `:noautocmd w`. The four blocks go,
  with the save and restore of `'eventignore'` that `:noautocmd` did.
- **The engine is answered at its doors.** `apply_autocmds_group()`,
  `has_autocmd()` and the per-event `has_*()` say no, and the `trigger_*()`
  helpers and `may_trigger_win_scrolled_resized()` do nothing — which is what
  each already did with no autocommand defined. The sweep takes the engine
  behind the doors. The calls that fire events stay: each is a call to a
  constant now, and removing them is a phase of its own.
- **Filetype detection after a rename** ran only when the `filetypedetect` group
  existed, which only `:augroup` or `:autocmd` could make; both tests fold, and
  `do_doautocmd()` goes with its last callers.
- **`in_vim9script()` is FALSE**: it was true only after `:vim9script` or under
  `vim9cmd`.
- **Suspending stays.** CTRL-Z, `:stop` and `:suspend` still hand the terminal
  back to the shell. The first version of this phase took them as part of the
  session and they were put back on request: suspending is job control, and
  nothing about it is read from or written to a file.
- **The command line loses its scripts.** `-S`, `-s file` outside Ex mode, and
  `-w file`/`-W file` are unknown options. `-s` keeps silent Ex mode after `-e`,
  `-wN` still sets `'window'`, and `-u file` stays.

## Two options that have to go before the sweep

`dropoptions.py --strict` refused `'eventignore'` after the first sweep: the
readers `event_ignored()` and `check_ei()` were still live. Two things held them.
`did_set_eventignore()` is the callback of **both** `'eventignore'` and
`'eventignorewin'`, and calls `check_ei()` — so while either row stands, the
reader is reachable from the option table and no sweep can take it, and
`--strict` cannot be satisfied in either order. And the WinScrolled/WinResized
scan read `'eventignorewin'` from every window before learning that neither
event had an autocommand.

So both rows go **before** the sweep and without `--strict`, and the
post-condition is the check: after the sweep nothing names `p_ei`, `wo_eiw`,
`check_ei`, `event_ignored` or `check_window_scroll_resize`. The enumerator
`WV_EIW` stays, named only by its own declaration: the `WV_` and `BV_` index
enums are anonymous, `enum { WV_LIST = 0, ... }`, and the definition finder
`deadenums.py` walks sees only tagged and typedef'd enums, so it never examines
them. That is a gap in a shared tool, measured here and not yet closed — closing
it changes every phase's implementation digest.
`'eventignorewin'` is window-local and `tools/droplocal.py` knows only buffer
fields, so `nosession.py` removes its field and the four places that maintain
it — `get_varp()`, `copy_winopt()`, `check_winopt()`, `clear_winopt()` — itself.

## The delta

**The ten rows that succeeded run bare** — `:sleep`, `:smile`, `:vim9script`,
`:autocmd`, `:augroup`, `:doautocmd`, `:doautoall`, `:noautocmd`, `:sandbox` and
`:filetype` — measured before the cut. `:source`, `:redir`, `:scriptencoding`,
`:scriptversion`, `:legacy` and `:setfiletype` already failed with no argument.
No harness sources, redirects,
suspends or defines an autocommand, and each passes `-s` only after `-e`.
Measured: 126,756 → **123,384 lines**, libc symbols 88 → 88.
