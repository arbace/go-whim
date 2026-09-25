# Phase 171 — a goto that is a break is break

Some of vim's `goto`s say what a structured statement already says. In
`ml_find_line` three `goto error_block;` leave the `for (;;)` that the label
follows: that is `break;`. In `getcmdline_int` one `goto returncmd;` sits
directly in the command-line loop, and `returncmd:` is the statement after it:
`break;` again. In `screenalloc`, `outofmem = TRUE; goto give_up;` is the last
thing in an `if` whose next statement is `give_up:` -- the jump goes where
control would fall anyway. This phase writes the first kind as `break;`,
deletes the second, and drops a label no `goto` reaches any more.

**Why.** A Java backend has no `goto`, and the Go editor pays for every one:
Go refuses a `goto` that jumps over a declaration, so `internal/gen` hoists
every local of a function that jumps to its top. A `goto` that is a `break`
needs none of that, and a function left with no `goto` has its locals declared
where C declares them. It is the first of the steps the goto survey (`GOTOS.md`,
classes B and C) found generic; phase 172 (backward gotos as loops) follows it,
because `screenalloc`'s `give_up` sits inside the region of its `retry` loop.

**The rule is general, and the tree only locates** (`crefactor/xform`'s
`GotoBreak`, the step `gotobreak`). A walk of each function's statements keeps,
for each `goto`, the chain of statements that holds it. *Where control goes
next* from a statement is found by climbing that chain: out of a block's last
item, out of an if's branch, out of a label's statement and out of a switch's
body, to the next item of a block; a loop's body stops the climb (its end is
the loop's test).

- If control goes next from the `goto` to an item its label marks, the `goto`
  goes (`;` where it is an if's lone statement).
- Else, if control goes next from the innermost loop or switch around it to an
  item its label marks, the `goto` is `break;`, which goes exactly there,
  skipping the loop's test and increment as the jump did.
- A `goto` whose label follows an OUTER loop or switch is held and counted: C
  has no break for more than one level.
- A function with a computed goto, a label's address (`&&L`), a local label, a
  nested function, or a jump or label inside an expression is left as it is.

**It changes no behaviour, and the binary may differ.** `--at-least 5` is the
floor.

**Measured** on q169 (the product before it, 75,396 lines): 4 `goto`s become
`break;` (`ml_find_line` 3, `getcmdline_int` 1) and 1 goes (`screenalloc`'s
`give_up`); 18 are held, each leaving more than one loop or switch; 2 labels
go (`error_block`, `give_up`); 0 functions left as odd. The product is 75,393
lines. `goto`s: 185 → 180 in `whim-vim.c`, 158 → 153 in the core, labels 37 →
35. In `editor/editor.go` (generated from the result, not committed here):
`goto` 163 → 158, labels 40 → 38, functions with a `goto` 34 → 33
(`ml_find_line` freed), 58,141 → 58,137 lines.

**Verified** with phase 172 on top: `whim test` on the result, 45/45 as HEAD
and the Go editor answering all 45 as the C does; `whim test --wide`, 240 cases
in 4 groups as HEAD; the same two with `editor/editor.go` regenerated from the
result. The step's own tests (`crefactor/xform/goto_test.go`): a break out of
a `for`, out of a `while` in an `if`, out of a `switch`; a goto that falls to
its label, braced and alone under an `else if`; a two-level goto held; a label
kept while one of its gotos is held; a computed goto's function left; each
program compiled silently by gcc under `-Wall -Wextra` and printing what the
original prints; and a control: `break` written for the held two-level goto
is caught by that comparison.
