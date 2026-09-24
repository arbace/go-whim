# Phase 37 — no command that does nothing

What was left in the table after Phase 36, read handler by handler, had ten
rows that either did nothing or did something this editor does not want:

| rows | what they did |
| --- | --- |
| `:browse`, `:confirm` | modifiers whose flags went with the file browser and the dialogs; each skipped its own name and ran the rest |
| `:tmap`, `:tnoremap`, `:tunmap`, `:tmapclear` | stored mappings for terminal-job mode, which nothing enters — no assignment puts `MODE_TERMINAL` in `State` |
| `:winpos` | "not implemented" bare; with two numbers, checked them and did nothing |
| `:behave` | set `'selection'`, `'selectmode'` and `'keymodel'` to another editor's habits — `mswin`'s naming the mouse, which Phase 24 removed |
| `:mode` | a screen-mode switch no terminal here has; bare, a redraw |
| `:open` | vi's open mode, which here was a cursor move followed by `:visual` |

All ten go to `ex_ni`. `tools/noinert.py` removes what a row cannot:

- **`:browse` and `:confirm` are matched by name in
  `parse_command_modifiers()`**, before the table, so their branches go; the
  name then reaches the table and is not implemented. `:browse set ic` no longer
  sets anything.
- **Terminal-job mappings.** `get_map_mode()` loses its `'t'` and
  `map_mode_to_chars()` the letter it printed. The `MODE_TERMINAL` enumerator
  and the masks that test it stay — constants, costing nothing.
- **Completion.** Unlike the phases before it, this one takes the
  `set_context_by_cmdname()` arms for its names, because `:behave`'s is what
  kept `get_behave_arg()` alive.

**`:highlight` stays.** It sets the colours of highlight groups, and `Search` is
the one `'hlsearch'` draws with; the defaults are applied through
`do_highlight()` whether or not the command exists, but changing them needs it.
Syntax highlighting is not in this build — the `:syntax` row already points
at `ex_ni`.

## The delta

**The seven rows that succeeded run bare** — `:browse`, `:confirm`, `:mode`,
`:open`, `:tmap`, `:tmapclear` and `:tnoremap`, read from the slim baseline.
`:behave`, `:tunmap` and `:winpos` already failed with no argument. Measured:
122,290 → **122,145 lines**, libc symbols 88 → 88.
