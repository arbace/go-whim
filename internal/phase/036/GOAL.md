# Phase 36 — one tab page, always

A tab page is a set of windows the editor can switch between whole. The
tab-page list is also the container every window lives in — `curtab` and
`first_tabpage` are read in hundreds of places — so **it stays, with exactly one
entry**, and every way to make or reach a second one goes.

The fifteen rows go to `ex_ni`: `:tab`, `:tabnew`, `:tabedit`, `:tabclose`,
`:tabonly`, `:tabnext`, `:tabNext`, `:tabprevious`, `:tabfirst`, `:tabrewind`,
`:tablast`, `:tabmove`, `:tabs`, `:tabdo` and `:redrawtabline`.
`tools/notabs.py` removes what a row cannot:

- **The `:tab` modifier** is matched by name in `parse_command_modifiers()`, and
  it was the only thing that set `cmdmod.cmod_tab`. With it gone every test of
  `cmod_tab` is decided: the tab branches of `:all`, `:ball`, `:drop`,
  `:argedit`, `:wincmd` and the command-line window fold.
- **The handlers the tab commands shared** keep their other users. `:tabnew` and
  `:tabedit` went through `ex_splitview()` with `:split` and `:new`, and `:tabdo`
  through `ex_listdo()` with `:windo`, so only their terms and branches go.
- **Each key keeps the answer it already gave with one tab page.**
  `goto_tabpage(n)` with a single tab page beeps when `n > 1` and otherwise does
  nothing, and there is never a last-used tab page. So `gt`, CTRL-PageDown and
  CTRL-W gt beep for a count above 1; `gT`, CTRL-PageUp and CTRL-W gT do
  nothing; `g<Tab>`, CTRL-Tab and CTRL-W g`<Tab>` beep; insert mode's
  CTRL-PageUp and CTRL-PageDown stay no-ops. The keys are not given a new
  meaning — they lose a function nothing could reach.
- **CTRL-W T and CTRL-W gf/gF open a tab page and nothing else**, so they beep
  now, as an unknown window command does. CTRL-W T with one window used to say
  "Already only one window"; that message goes with the command.
- **The tab line** is 0 lines and `draw_tabline()` draws nothing — what both
  answered for one tab page under the default `'showtabline'`. `win_split()`
  no longer asks `may_open_tabpage()` whether a `:tab`-modified split became a
  tab page — a stub would have answered, and left the caller and the function
  alive, which is how the first run of this phase failed. The sweep then takes `'showtabline'`, `'tabline'`
  and `'tabpagemax'`'s readers, and `dropoptions.py --strict` their rows.
  `'tabclose'` is the Phase 35 trap again: its own callback, `did_set_tabclose()`,
  reads `p_tcl`, so while the row stands the reader is live and no order of sweep
  and `--strict` works. Its row goes before the sweep, and the post-condition —
  nothing names `p_tcl` or `tcl_flags` afterwards — is the check.

Left alone, as every earlier phase left them: the completion arms of
`set_context_by_cmdname()` for these names.

## The delta

**The thirteen rows that succeeded run bare** — `:tab`, `:tabedit`, `:tabfirst`,
`:tabmove`, `:tablast`, `:tabnext`, `:tabnew`, `:tabonly`, `:tabprevious`,
`:tabNext`, `:tabrewind`, `:tabs` and `:redrawtabline` — measured before the cut.
`:tabclose` and `:tabdo` already failed with no argument. No harness opens a tab
page. Measured: 123,384 → **122,290 lines**, libc symbols 88 → 88.
