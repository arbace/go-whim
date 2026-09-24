# Phase 38 — the argument list is walked by `:next` and `:previous` alone

**The list stays.** `vim a b c` fills it, `:next` and `:previous` move through
it, `:next x y` replaces it, `:drop` sets it, and quitting with files not yet
edited is still refused. Every other command on it goes — twenty-five rows:
`:args`, `:argglobal`, `:arglocal`, `:argadd`, `:argdelete`, `:argdedupe`,
`:argedit`, `:argument`, `:sargument`, `:first`, `:sfirst`, `:rewind`,
`:srewind`, `:last`, `:slast`, `:snext`, `:wnext`, `:Next`, `:sNext`,
`:sprevious`, `:wNext`, `:wprevious`, `:all`, `:sall` and `:argdo`.

**`:Next` is a row of its own**, spelled apart from `:previous` though it shares
the handler, so `:N` goes with it; `:prev` still reaches `:previous`.

`tools/noarglist.py` removes what a row cannot:

- **The shared handlers keep their other users.** `:snext` went through
  `ex_next()`, and `:argdo` through `ex_listdo()` with `:bufdo` and `:windo`, so
  only their terms go; `do_argfile()` no longer spares `:argdo` the `'` mark.
  **`ex_rewind()` stays**, because `:drop` ends in it — the `:first` row going
  does not make its handler dead, and the phase checks that it survives.
- **Completion** for `:argdo` and `:argdelete`, and the argument-list expansion
  only `:argdelete` asked for, so the sweep takes `get_arglist_name()`.

The sweep takes the handlers, `do_arg_all()` and its helpers, `alist_new()` —
only `:arglocal` gave a window a list of its own — and `list_in_columns()`,
which only `:args` printed with.

## The delta

**The sixteen rows that succeeded run bare** — `:all`, `:args`, `:argadd`,
`:argdelete`, `:argdedupe`, `:argglobal`, `:arglocal`, `:argument`, `:first`,
`:last`, `:rewind`, `:sargument`, `:sall`, `:sfirst`, `:slast` and `:srewind`,
read from the slim baseline. The other nine already failed with no argument.
Measured: 122,145 → **121,368 lines**, libc symbols 88 → 88.
