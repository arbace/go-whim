# Phase 3a — no lisp

*Formerly phase 57, now part a of phase 3. The other phase numbers in this file are the old
numbering, as it was written: `doc/PHASES.md` maps them.*

**A record now.** Phase 3's front calls `whim57` by name, after D12's cuts
(`doc/PIPELINE-REFORM.md` §7, the drops brought to the front), and phase 6
drops the two fields with phase 58's, on the text phase 3 swept: the program
is here, and the phase has no plan entry. It applied to the front's text as
written. What follows is the account of the cut as it was made here.

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

**On the graph** (B4, `doc/GRAPH-MIGRATION.md`): this part runs on `crefactor/graph` inside its phase's front, registered with `phase.RegisterGraph`, its report the text version's and its boundary byte for byte.
