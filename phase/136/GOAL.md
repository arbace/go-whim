# Phase 136 — the engine is called directly

With one engine, `bt_regengine`'s four function pointers always hold
`bt_regcomp`, `bt_regfree`, `bt_regexec_nl` and `bt_regexec_multi`, and every
program's `engine` points at it — `bt_regcomp()`, the only function that
makes a program, sets it. So the four calls through the table call known
functions. They name them, `bt_regcomp()` stops recording an engine, and the
sweep takes the table, the field and `regengine_T`.

**Declared delta: nothing.** The check proves the premise from the input —
the table's initialiser in field order, one assignment of `->engine` — and
requires the three names gone. Its probe runs a search, a single-line
substitution with back-references and a multi-line one on both binaries;
each control, a pattern matching nothing, must write different bytes.
