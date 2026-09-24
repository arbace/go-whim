# Phase 49 — one set of options

Every buffer and window option has two copies inside the editor, a global and a
local one. **The storage stays**: collapsing it would touch every option's reader
for nothing a user can see. What goes is every way to make the two copies differ,
so that `:set` — which writes both — is the only way an option is given a value,
and there is one set of options as far as anything outside can tell.
`tools/oneoptset.py` removes the four things that made them differ:

- **`:setlocal` and `:setglobal`** wrote one copy each. Their rows go to `ex_ni`,
  `ex_set()` stops choosing a flag for them, and their completion arms go.
- **`:set opt<`** copied the global copy into the local one. `<` is no longer an
  accepted suffix, and its three branches — boolean, number, string — fold, so
  `:set ts<` is an error like any other malformed `:set`.
- **Modelines** set a file's local copy from a `vim: set ...:` line, and the user
  was asked and chose to drop them. The four calls of `do_modelines()` go and the
  sweep takes it and `chk_modeline()`; every test of `OPT_MODELINE`, a flag
  nothing passes after that, folds; and `'modeline'`'s save and restore around
  `'binary'` in `set_options_bin()` goes. Then the rows of `'modeline'`,
  `'modelines'`, `'modelineexpr'` and `'modelinestrict'` go, `droplocal.py` takes
  `b_p_ml`, and `b_p_ml_nobin` — not an option, so with no `get_varp()` case that
  tool knows — goes by hand. A first run found that.

**Left alone:** a value detected from the file being read. `'fileformat'`, and
`'binary'` from `-b`, are the current file's state, and with one buffer only ever
one file's.

The phase checks that `:set ts<` is refused against a `:set ts=3` control, and
that `>>` on a file whose modeline says `sw=2` indents by the compiled-in four.
**The first version of that check proved nothing.** It used `ff=dos`, and
`'modelinestrict'` let a modeline set only whitelisted options, which
`'fileformat'` is not, so it passed against binaries that still read modelines.
`'shiftwidth'` is on the whitelist: measured, the Phase 48 binary indents by two
and this one by four.

**Measuring it showed something else.** `slim-vim` indents by four too, and so do
whim Phases 0 to 24, with `:set modeline?` answering `nomodeline`; Phases 25 to 48
answer `modeline`. The harnesses run as root, and upstream forces `'modeline'` off
for root — the check Phase 25 removed, as its section says. So a modeline was
read, as root, from Phase 25 until this phase, and no harness case has a modeline
to notice. Diffing `:set all` between Phases 24 and 25 shows that it is the only
value that moved besides the backup options that phase removed on purpose.

## The delta

**`:setlocal` and `:setglobal`**, which succeeded run bare. Measured: 115,568 →
**115,246 lines**.
