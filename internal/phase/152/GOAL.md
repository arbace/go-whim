# Phase 152 — the option variables are typed

`vimoption_T.var`, `optset_T.os_varp`, `get_varp()`, `get_varp_scope()` and
every `varp` held the address of an option's variable (an `int`, a `long` or a
`char_u *`) as a `char_u *`, cast back at every read: `*(int *)varp`. Two more
tricks rode on that pointer:
- a window-local option with no global variable held `(char_u *)-1`;
- its global value was reached as `(char *)get_varp(p) + sizeof(winopt_T)`,
  the same field one `winopt_T` further on, in `w_allbuf_opt`.

The Go transpilation held all of these as `any` (finding 2, its second half).

Now they are an `optvar_T`: a pointer of each kind, one of them set, and a
flag for the window-local sentinel.
- **reads** name their kind: `*varp.ov_int`, `*varp.ov_long`, `*varp.ov_str`;
- **table rows** name their variable in the slot the row's `P_BOOL`, `P_NUM`
  or `P_STRING` gives;
- **`get_varp()`'s returns** are built by constructors typed by the field they
  name, so the compiler checks every one;
- **the window-local global value** is `get_varp_allbuf()`, the
  `w_allbuf_opt` field by name. That covers both `get_varp_scope()` and
  `set_string_option_global()`, whose two callers hand it curwin's
  `w_onebuf_opt` field, the one the byte offset stepped from.

`free_one_termoption()` compares a terminal option's variable address with the
string passed to it. The address of a `term_strings` slot is never equal to a
string, so the comparison is never true, as it was before. It is kept, cast
for cast: a latent bug of vim's, not this phase's to fix.

**Declared delta: nothing.** The check computes every row's typed variable
from the input's row, and requires no option variable punned through
`char_u *`. Its probes cover:
- a boolean, a number and a string option, set and read back;
- window-local, buffer-local and global-local options through `:set`,
  `:setlocal` and `:setglobal`;
- a terminal option, and `:set all`.

Each control moves.

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 151-152 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).
