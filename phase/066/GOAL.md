# Phase 66 — no sentences, paragraphs, sections, methods, #if blocks or comment blocks

One idea, cut at all three places it was reachable from. A sentence you cannot
move over is not one you can select, or address a line range with.

- **The motions.** `(` and `)` by sentence, `{` and `}` by paragraph — these four
  `nv_cmds` rows point at `nv_error` — and from `nv_brackets()`/`nv_bracket_block()`:
  `[[` `]]` `[]` `][` by section, `[m` `]m` `[M` `]M` to a method's braces, `[#` `]#`
  to the enclosing `#if`/`#endif`, and `[/` `]/` `[*` `]*` to the enclosing C comment.
  The dispatch sets shrink from `"{(*/#mM"`/`"})*/#mM"` to `"{("`/`"})"`.
- **The text objects.** `is`, `as`, `ip` and `ap` — `current_sent()` and
  `current_par()`.
- **The Ex addresses.** `'{`, `'}`, `'(` and `')` as line addresses, which
  `get_address()` answered by calling `findpar()` and `findsent()`.

After which `findsent()`, `findpar()` and `startPS()` have no callers at all, and
the concept is gone from the editor rather than merely unbound.

**What stays, and is checked rather than assumed**: `%` and the enclosing-bracket
motions `[{` `]}` `[(` `])`, which are `findmatchlimit()` and never had anything to
do with paragraphs; the `(` `)` `{` `}` `[` `]` `<` `>` **text objects** (`i{`, `a(`
…), which are `current_block()`; `iw`/`aw`; and the `'[` `']` `'<` `'>` marks, which
`get_address()` answers from stored positions.

**Four failures, all in the phase's own machinery rather than the tree.**

1. **The method test matched twice.** `if (cap->nchar == 'm' || cap->nchar == 'M')`
   is both the head that picks the character to match and the half that walks out
   to the method. The counted helpers cannot express "the second of two" — they
   die on any count but the one given — so the walk-out is cut from a slice that
   starts at it, and only then is the head the single match the counted fold wants.
2. **A dead assignment in the script itself**, left over from the first attempt at
   that ordering.
3. **`prev_pos` was set and not used** once the walk-out went. gcc reports
   `-Wunused-but-set-variable`, which `deadsweep.py` does not handle: it deletes
   *unused* variables, not written ones. The declaration and both writes go by
   hand. `c` is a plain unused variable after the same cut, and the sweep takes it.
4. **`lines()` was never defined in this script** — the three `prev_pos` removals
   were carried over from phase 64 without its helper.

## The delta

**None.** No behaviour case moves over a sentence or a paragraph, and no Ex
command changes. The probes check that each cut key leaves the file untouched —
a beep abandons the rest of a `:normal!` sequence, so the `ix` after it never
runs, with a bare `ix` as the control — that `[{` still walks out to the enclosing
`{`, `%` still matches, `di{` still deletes a block's contents without its braces,
and `'{,'}d` is refused.

Measured: 97,684 → **96,848 lines**.
