# Phase 58 — no language mappings

`'iminsert'` and `'imsearch'` are 0 from here on, so language mappings are never
active and nothing can make them so. `:lmap`, `:lnoremap`, `:lunmap` and
`:lmapclear` point at `ex_ni` and lose their completion. CTRL-^ in Insert mode and
on the command line is still consumed — its `case` stays, so it does not start
inserting itself — and toggles nothing. `MODE_LANGMAP` is never set, so every test
of it folds: in `edit()`, `ex_append()`, `ins_insert()`,
`normal_cmd_get_more_chars()`'s lookup for `r`, `f` and `t`, `getcmdline_int()`
for `/`, `?` and `@`, `handle_mapping()`, `vgetorpeek()`, `get_map_mode()` and
`map_mode_to_chars()`. The status line's `<lang>` goes with `get_keymap_str()`,
which only ever printed it.

**The declared delta was wrong once, and the harness said so.** It named all four
commands; `:lunmap`'s row did not move, because bare `:lunmap` already failed for
want of an argument and `ex_ni` fails too. The declaration was corrected rather
than the check widened. The phase checks the two options are unknown, `:lmap` is
refused, and CTRL-^ in Insert mode inserts nothing.

## The delta

`:lmap`, `:lnoremap` and `:lmapclear`, now `ex_ni`. Measured: 108,651 →
**108,374 lines**.
