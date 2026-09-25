# Phase 168 — a goto whose label returns is that return

vim leaves a function early with `goto theend;`, and `theend:` marks
`return ret;`. Where the label marks nothing but the return, the jump is the
return: at the `goto`, `return ret;` returns what the label would, in the same
state, because nothing runs in between. This phase writes the return at every
such `goto`, and drops a label no `goto` reaches any more. It is
`doc/GO-IDIOMS.md` item 10: the Go says `return retval` where it said
`goto theend`, and a function left with no `goto` has its locals declared where
C declares them, since `internal/gen` hoists them only in a function that
jumps.

**The rule is general, and the tree only locates.** `cc.Parse` finds each
label whose statement -- through any further labels -- is a `return`, and each
`goto` to it; the text of the return is copied over the text of the `goto`.
One thing could make the copy mean something else: a name in the returned
expression that is another object at the `goto` than at the label. So a `goto`
is held when a name in the expression is declared in any inner block of the
function, where it might shadow, or at the function's top after the `goto` or
after the label. A label that still has a `goto` stays. When a label goes and
the statement before it always jumps, nothing reaches its return any more, and
the return goes with it (phase 164's rule).

**It changes no behaviour, and the binary may differ**: gcc at `-O0` compiles
a return in each place where there was one jump to a shared one.

**Measured:** on the product before it, 19 `goto`s become returns in 7
functions (`do_map`'s eight `goto theend`, `match_keyprotocol`'s four,
`ml_append_int`'s three, and one each in `ml_delete_int`, `do_join`, `do_set`
and `vim_regsub_both`, whose label marks `return (int)((dst - dest) + 1)`),
0 are held, and
7 labels go; 0 returns become unreachable. `whim-vim.c`'s `goto`s go from 204
to 185. In `editor/editor.go`: `goto` 182 → 163, labels 37 → 30, functions with
a `goto` 41 → 34, and 74 lines fewer, the locals of the seven functions no
longer hoisted. `whim-test`: 45/45 as the commit before, and the Go editor
answers all 45 as the C does.

The transformation now lives in `crefactor/xform` (`GotoReturn`).

**Since merged** (2026-09-25, `doc/PIPELINE-COMPACTION.md` §3d): this phase carries the group 166-168 -- the steps of each, in order, then one sweep and one print. Each phase's own `GOAL.md` still says what its steps do; the merge moved no byte of the product (`whim-build-check`).
