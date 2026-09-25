# Phase 172 — a goto back is a loop

Two of vim's `goto`s jump backward, and both are retry loops written with a
label. `screenalloc` begins `retry:` after its declarations and ends
`if (starting == 0 && ++retry_count <= 3) goto retry;`.
`normal_cmd_get_count`'s first statement is `getcount: if (...) { ... }`, and
the `goto getcount;` is the last statement of an `if` inside it. This phase
writes each as the loop it is and drops the label: the first as
`do { ... } while (starting == 0 && ++retry_count <= 3);`, the second as
`for (;;) { if (...) { ...; continue; } break; }`.

**Why.** A Java backend has no `goto` and its labeled `break` goes only
forward; a backward jump is a loop or nothing. The Go editor loses the two
`goto`s and the two functions are freed of them, so `internal/gen` declares
their locals where C does. It is class D of the goto survey (`GOTOS.md`). It
follows phase 171, which deletes `screenalloc`'s `goto give_up;` -- a jump
inside `retry`'s region, which would otherwise be a forward goto inside the new
loop.

**The rule is general, and the tree only locates** (`crefactor/xform`'s
`GotoLoop`, the step `gotoloop`). The label L marks item k of a block, and
every `goto L;` is under an item i >= k of the same block. Items k..i -- i the
last goto's -- are the region, and become

```c
for (;;) { <items k..i>; break; }      /* each goto L; -> continue; */
```

with no `break;` where item i always jumps; or, where the one `goto` is item i
itself as `if (c) goto L;`, `do { <items k..i-1> } while (c);`. The loop runs
the region again exactly where the jump went back, and leaves it where control
ran off its end. A label is held, with its gotos, when the loop would mean
something else, and the report names why:

- a `goto` to it comes from before it (it would jump into the loop);
- a loop or switch lies between a `goto` and the region's block (the
  `continue` would bind to it);
- a `break` or `continue` in the region leaves it (the new loop would take it);
- a `goto` from outside the region reaches a label inside it, or a `case` of a
  switch around the region is in it (control would enter the loop from the side);
- a declaration among items k..i is named after the region (the loop's braces
  would end its scope);
- another label marks item k, or the label is not a block item;
- its region overlaps one taken before it in the same function (a second run
  takes it).

The `do` form is written only where `c` names nothing the region declares. A
function with a computed goto, a label's address, a local label, a nested
function, or a jump or label inside an expression is left as it is.

**It changes no behaviour, and the binary may differ.** `--at-least 2` is the
floor.

**Measured** on q169 with phase 171 applied: 2 backward `goto`s become loops,
1 as `do`-while (`screenalloc`) and 1 as `for (;;)` (`normal_cmd_get_count`);
0 held; 2 labels go. The product is 75,395 lines (75,393 before: the `do`
is one line fewer, the `for (;;)` with its braces and `break;` three more). `goto`s: 180 →
178 in `whim-vim.c`, 153 → 151 in the core, labels 35 → 33. In
`editor/editor.go` (generated from the result, not committed here): `goto`
158 → 156, labels 38 → 36, functions with a `goto` 33 → 31 (`screenalloc`
and `normal_cmd_get_count` freed), 58,137 → 58,127 lines. With phase 171,
from q169: `goto`s 185 → 178 (core 158 → 151), and in the Go 163 → 156 in 34 → 31
functions.

**Verified**: `whim test` on the result, 45/45 as HEAD and the Go editor
answering all 45 as the C does; `whim test --wide`, 240 cases in 4 groups as
HEAD; the same two with `editor/editor.go` regenerated from the result;
`gcc -fsyntax-only` silent. The step's own tests
(`crefactor/xform/goto_test.go`): a two-goto retry as `for (;;)` with its
`break;`, a guarded one as `do`-while, one whose region always jumps with no
`break;`; each hold above as a case that the report names and the text keeps;
two nested retries, the inner held for overlap and taken by a second run;
each program compiled silently by gcc under `-Wall -Wextra` and printing what
the original prints; and a control: a retry wrapped although a `while` inside
it takes the `continue` -- the program then loops for ever, which the
comparison catches.
