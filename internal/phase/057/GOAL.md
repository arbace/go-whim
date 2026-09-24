# Phase 57 — no lisp

`'lisp'` and `'lispwords'` go, and with them everything they switched on:
`get_lisp_indent()` for autoindent, `=`, `gq` and new lines; `lisp_match()` over
`'lispwords'`; `-` as a keyword character; `;` line comments in
`check_linecomment()`; and `findmatchlimit()`'s lisp mode, which stopped `%` at a
`;` comment and skipped `#\(` character literals. `'lispoptions'` went in Phase 55.

`b_p_lisp` is folded as false at every reader rather than stubbed — nine places
in `findmatchlimit()` alone — so each branch it guarded is gone or taken
unconditionally. One test inverts: `op_reindent()` skipped the last line of a
range only when re-indenting with `get_lisp_indent()`, so its `how !=
get_lisp_indent` is always true and the branch is kept. The phase checks the two
options are unknown and that `%` on `(a ; b)` now matches across the `;`.

## The delta

**None the harnesses record** — no case sets `'lisp'`. Measured: 109,039 →
**108,651 lines**.
