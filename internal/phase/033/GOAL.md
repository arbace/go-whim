# Phase 33 — commands whose machinery has already gone

Every one of these still had a handler, and every one refused or did nothing
when run with a sensible argument. That was measured one at a time, in Ex mode,
reading the message each left behind:

| command | what it said |
| --- | --- |
| `:shell` | `E319`, no processes since Phase 8 |
| `:gui`, `:gvim` | `E25`, no GUI in this build |
| `:cdo` `:cfdo` `:ldo` `:lfdo` | `E319`, no quickfix lists |
| `:vim9cmd` | `E319`, no eval layer |
| `:endclass` `:endinterface` `:endenum` `:public` `:static` `:this` | Vim9 class keywords, invalid without the eval layer |
| `:digraphs` | `E196`, no digraphs in this build |
| `:redrawtabpanel` | `E1547`, no tab panel |
| `:colorscheme` | `E185`, no colour scheme to find — nothing is installed |

**A command that only says no is a row pointing at a handler that exists to say
no.** So the rows go to `ex_ni` — rule 3, the table keeps its shape — and the
sweep takes `ex_shell`, `ex_nogui`, `ex_digraphs`, `ex_redrawtabpanel`,
`ex_colorscheme` and `load_colors()`, which nothing else called. `ex_listdo`
stays, because `:argdo`, `:bufdo`, `:windo` and `:tabdo` use it, and its two tests
for the quickfix commands are folded rather than left asking a question that can
no longer be true. `ex_wrongmodifier` stays for the modifiers that still work.

**`:!` is the one refusal kept, and on purpose.** `:!cmd`, `:r !cmd` and `:w !cmd`
are how a user reaches for a process, and the answer Phase 8 gave them is the
sentence it prints. `:filetype` and `:vim9script` are not here either: they run
without an error, and nothing measured shows them refusing.

Left alone, as every earlier phase left them: the arms of
`set_context_by_cmdname()` that set up command-line completion for these names.
A retired row still parses, so its completion context still fires, and it
completes nothing.

## A tool bug this found

`tools/retire.py` matched a row with exactly one space before the handler, and
the `:gui` and `:gvim` rows are spelled `- 1,  ex_nogui ,` — macro expansion's
spacing. It refused, loudly, which is what it is for; the rule is the row, not
the spacing, so it takes any whitespace now. The six earlier phases that retire
rows with it were re-verified and all reproduce their boundaries.

## The delta

**`:colorscheme`**, which run bare reported the current scheme and succeeded in
`slim-vim`, and now reports that it is not implemented. Every other row already
failed, or is one the sweep skips because it hands over the terminal. Measured:
127,131 → **127,037 lines**, libc symbols 88 → 88.
