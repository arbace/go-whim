# Phase 170 — a goto whose label marks a short tail is that tail

vim leaves a function early with `goto theend;`, and `theend:` marks a few
statements of clean-up and then the return: `*ptr = cmd; return lnum;`,
`--bt_reg_parse_depth; return ret;`, or in a void function the closing brace.
Where the tail is that short and that plain, the jump is the tail: at the
`goto`, running the tail's statements and returning does what the jump leads to,
in the same state, because the tail is straight-line code that returns. This
phase copies the tail over every such `goto`, and drops a label no `goto`
reaches any more. It is phase 168's rule widened from no statement before the
return to a few, and the first of the steps `doc/AGENDA.md` item 1 lists toward
no `goto` in the C: a Java backend has none, and the Go loses these too --
a function left with no `goto` has its locals declared where C declares them,
since `internal/gen` hoists them only in a function that jumps.

**The rule is general, and the tree only locates.** It is `crefactor/xform`'s
`GotoTail`, the `gototail` step, and names nothing in vim; whim tells it one
knob, the bound (`internal/whim/xform.go`: at most **3** statements before the
return, the longest straight tail a `goto` of the core reaches --
`cmdline_browse_history`'s three stores -- and each copy costs its length).
`cc.Parse` finds each label that is a block item, followed in its block by at
most three expression statements (an empty one neither counted nor copied) and
then `return x;` -- or, in a function whose type is `void`, by the end of its
body, which is `return;`. The loops and switches between a `goto` and its label
do not matter: the copy ends in a return. The copy is written in braces where
the `goto` was an if's or a case's statement, and as block items where it was
one.

**What it holds.** A `goto` is left, and with it its label, when the copy could
mean something else or could not be written twice:

- a name the tail reads -- a typedef name too -- is declared in any inner
  block of the function, where it might shadow, or at the function's top after
  the `goto` or after the label (phase 168's rule);
- the tail holds another label, a `case` or a `default` (another way in, and a
  copy of it would be a second one), a declaration, or a statement expression
  (a block inside an expression, which could declare, be labeled or break).

A tail with any other statement -- an `if`, a loop, a jump that is not the
return -- or with more than three is no tail, and its gotos stay: they are for
the phases after this one. A label no `goto` reaches any more goes, unless its
address is taken (`&&L`). When the statement before it always jumps, nothing
reaches its tail either, and the tail goes with the label (phase 164's rule); a
label on an empty statement goes with the statement.

**It changes no behaviour, and the binary differs**: gcc at `-O0` compiles the
tail at each place where there was one jump to a shared one.

**Measured**, the phase run alone on q169 (`go tool whim build --from 170 --to
170`, 3 s): **79 `goto`s take their tail, 0 are held**, and 106 to a label that
marks no such tail stay. In the core, 52 in 12 functions -- `get_address` 12,
`parse_winhighlight` 8, `do_set_option_value` 6, `reg` 6, `do_set_option` 5,
`cmdline_browse_history` 3, `gotchars_add_byte` 3, `do_set_option_numeric` 3,
`open_line` 2, `op_yank` 2, `showmap` 1 (a void end: `--map_locked;
return;`), `utf_find_illegal` 1 (`theend: ;` at a void end: `return;`) -- which
is exactly class A of the goto survey, every function of it freed; in the host,
27 in 2 more (`parse_fmt_types` 22, `vim_vsnprintf_typval` 5), the host's
every `goto`. 14 labels go; 3 of them -- `parse_winhighlight`'s `fail`,
`op_yank`'s `fail`, `parse_fmt_types`' `error` -- with their tail, which only the
gotos reached.

| | before | after |
|---|---:|---:|
| `goto` in `whim-vim.c` (core + host) | 185 (158 + 27) | 106 (106 + 0) |
| labels in `whim-vim.c` (core + host) | 32 (30 + 2) | 18 (18 + 0) |
| functions with a `goto` (core + host) | 34 (32 + 2) | 20 (20 + 0) |
| lines of `whim-vim.c` | 75,396 | 75,507 |
| `goto` in `editor/editor.go` | 163 | 111 |
| labels in `editor/editor.go` | 30 | 18 |
| functions with a `goto` in `editor/editor.go` | 34 | 22 |
| lines of `editor/editor.go` | 58,141 | 58,113 |

The Go's numbers are `internal/gen` run out of tree on the core of the phase's
output; its 5 gotos more than the C's are the generator's own (a `continue` in
a `do`-while or a `for` with a complex increment). The C grows 111 lines by
its copies, and the Go shrinks 28 though it carries the same copies: the twelve
freed functions no longer declare their locals at the top.

`gcc -fsyntax-only -Wall -Wextra -Wno-unused-parameter` prints nothing on the
output, as on the input. `whim test` on the output: 45/45 cases as HEAD does,
and the Go editor answers all 45 as the C does; `whim test --wide`: 240 cases
as HEAD does. Both again with the `editor.go` generated from the output in
place of the committed one: the same.

`crefactor/xform`'s tests hold the rule on a foreign program (`gototail_test.go`):
every hold as a case, the tails that are none, a kept `&&` label; gcc compiles
the input and the output silently and they print the same; and, as the control,
the two rewrites the name rule forbids (an inner `r` and an inner typedef
`T` shadowing at the `goto`), made anyway, compile as silently and print
something else.
