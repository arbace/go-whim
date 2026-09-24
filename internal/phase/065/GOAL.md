# Phase 65 — no rot13, no operator function, no empty key handler

Three cuts, and only the first changes what the editor can do.

- **rot13.** `g?` is the one operator here that encodes rather than edits. It goes
  whole: `nv_g_cmd()`'s case, the `OP_ROT13` dispatch label, `nv_search()`'s
  redirect — which is how `g?` reaches the operator while a search is pending —
  and `swapchar()`'s three arms, after which `swapchar()` is the case-changing
  function it always really was.
- **The operator function.** `g@` has had nothing to call since the eval feature
  went: `op_function()` was one `emsg()`, and `'operatorfunc'` does not exist to
  name a function anyway. The dispatch, the `OP_FUNCTION` term in the
  `motion_force` test and `op_function()` itself go, and
  `e_eval_feature_not_available` falls to the sweep with its only reader.
- **An empty call.** `ins_ctrl_x()` had an empty body — CTRL-X in Insert mode
  began a completion, and completion went in phase 32. The key stays inert, but
  it no longer calls a function in order to do nothing.

**Three things are kept deliberately, because "does nothing" and "should be
deleted" are different claims.**

- **CTRL-P and CTRL-N in Insert mode are `break;`** — they do nothing *on purpose*.
  Deleting the labels would drop them into `normalchar`, which **inserts the
  control character**, so removing dead-looking code would add behaviour. The
  phase greps that `case Ctrl_P:` survives.
- **`zy`, `zp` and `zP` are live.** It looks as though `zy` must reach
  `internal_error("get_op_type()")`, since `opchars[]` has no `{'z','y'}` row —
  but `get_op_type()` special-cases `'z'`+`'y'` to `OP_YANK` before it consults
  the table. Measured on the q64 binary before cutting: no error, no message.
  This is why the "dead weight" list was checked key by key rather than read off
  the table.
- **The `opchars[]` rows for `g?` and `g@` stay**, for the reason phase 64
  records: the table is positional, so a removed row renumbers every operator
  after it. Nothing reaches them once `nv_g_cmd()` has no case.

The `'?'` and `'@'` case labels are edited **scoped to `nv_g_cmd()`**: another
switch entirely has `'?'` and `'@'` adjacent, and an unscoped edit would have had
two places to choose between — the same trap as `if (inindent(0))` in phase 64.

## The delta

**None.** No Ex command moves, and no behaviour case covers rot13 — the harness
never encodes anything. The probes check `g?g?` and `g??` no longer encode, that
`g@g@` is refused, that `gUU`, `guu` and `g~~` still change case (they share
`swapchar()` with the arms that went), and that `zyy` still yanks.

Measured: 97,734 → **97,684 lines**.
