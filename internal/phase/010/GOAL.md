# Phase 10 — no tag stack

A tag jump is the editor discovering, on its own, that a file it was never told
about exists. `get_tagfname()` walks `'tags'` upward from the current file,
opens whatever it finds and binary-searches it — filesystem-layout knowledge of
exactly the kind Phase 7 removed from `'path'`, and the largest single item
left in the tree at 2,364 lines.

## Four entry points that are not commands

Retiring the fifteen rows is most of it, and would have removed almost nothing
on its own, because each of these keeps the whole subtree alive by itself:

- **`nv_help()` — the `<Help>` key — calls `ex_help()`, which calls `do_tag()`.**
  `:help` has been `ex_ni` since Phase 1, but the *key* was never cut, so the
  entire help-tag search survived a phase that believed it had removed it. This
  is the clearest case in this tree for the rule that entry points are cut, not
  commands.
- `nv_tagpop()` — CTRL-T — calls `do_tag()` straight out of `nv_cmds[]`.
- `ExpandFromContext()` dispatches `EXPAND_TAGS` to `expand_tags()` and
  `EXPAND_HELP` to `find_help_tags()`. Completion is a caller like any other.
- `get_next_completion_match()` dispatches CTRL-X CTRL-] to
  `get_next_tag_completion()`.

**CTRL-`]` is not on that list and does not need to be.** `nv_ident()` builds
the string `":ta "` and runs it as an Ex command, so retiring the row is enough
and the key reports what `:tag` reports — which is also the honest answer.

## What stays

`vim_findfile()`. `'tags'` searching and `'path'` searching share it, and
`find_file_in_path_option()` still serves `:find` and `gf`. Cutting that is a
separate decision from this one, because `gf` is a normal-mode command a user
would miss, and it deserves to be made on its own.

## Two options that cannot go, and the trap they exposed

Six of the eight tag options are dropped. **`'tags'` and `'tagcase'` are
`PV_BOTH` — buffer-local — and their rows are also what initialise their
globals**, because `set_init_1()` sets `p_tags` and `p_tc` by walking
`options[]`. Remove the row and the global stays NULL, and any reader the sweep
does not reach dereferences it at startup.

`'tagcase'` is the one that taught this. Dropping it **built cleanly, swept to
silence, passed the linkage and symbol checks, and segfaulted before the first
keystroke.** From outside, the harness reported it as *every* behaviour case,
the terminal table and *every* Ex command moving at once — which is what a crash
looks like through a delta check. Every option any phase had dropped until then
was `PV_NONE`, so nothing had ever exercised this path.

`tools/dropoptions.py` now refuses a row whose `indir` is not `PV_NONE`, and
says why. Refusing is the right answer rather than handling it: removing the
buffer-local field, its initialiser, its copy, its free and its readers is real
surgery, and it should be a phase that says so rather than a side effect of a
call that looks like the six beside it. The two options stay, inert, until then.

## The delta

**One row moves, not fifteen**, and the difference is worth keeping. Retiring a
command only shows up in the Ex sweep if the command used to *succeed*: `:tag`,
`:tjump` and the rest already failed for want of a tags file to read, and
`ex_ni` fails too, so their recorded exit is unchanged. `:tags` listed an empty
tag stack and exited 0, and now reports instead. CTRL-`]` and CTRL-T report what
`:tag` reports. The declared list is what moved, not what was cut.
