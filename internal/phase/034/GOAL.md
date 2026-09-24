# Phase 34 — no abbreviations

An abbreviation is a word the editor rewrites as you type it. Nothing reads a
vimrc here, so the only way to have one was to type `:abbreviate` in the session
that wanted it — and the twelve rows that did that, `:abbreviate`, `:noreabbrev`,
`:unabbreviate`, `:abclear` and their `i` and `c` forms, go to `ex_ni`.

**Retiring the rows removes the answer, not the question.** Insert mode asked
`echeck_abbr()` on ESC, CTRL-O, CTRL-L, Tab, Enter and every non-word character,
and the command line asked `ccheck_abbr()` twice, and each would go on asking
for ever and being told no. `tools/noabbr.py` removes the questions, each a fold
whose answer is now known:

- `if (echeck_abbr(...)) { ... }` and `if (ccheck_abbr(...)) { ... }` guard what
  happens when an abbreviation fired, so the blocks go;
- `!echeck_abbr(x) && c != Ctrl_RSB` is `c != Ctrl_RSB`;
- `(ccheck_abbr(x) || c == Ctrl_RSB)` is `c == Ctrl_RSB` — CTRL-] on the command
  line still triggers "an abbreviation", which is to say nothing, and still does
  not insert itself.

The sweep then takes `check_abbr()` — 195 lines — its two wrappers,
`ex_abbreviate` and `ex_abclear`. **What stays** is the mapping code's `abbr`
parameters and list, which mappings share; nothing can put an entry on that list
any more, and nothing here pretends that makes the shared code smaller.

The tool's own check failed twice before the phase ran, both times on itself:
it counted `check_abbr()` calls inside the two wrappers the sweep removes, and
then prototypes it matched with one space where the file has two. **A check that
cannot tell a caller from a definition is measuring the wrong thing**, and it
now asks only about calls outside the definitions going away.

## The delta

**The nine rows that succeeded run bare** — `:abbreviate`, `:noreabbrev` and
`:abclear` with their `i` and `c` forms, which listed or cleared nothing and
exited 0 — measured before the cut. The three `:unabbreviate` rows already failed
with no argument. Measured: 127,037 → **126,756 lines**, libc symbols 88 → 88.
