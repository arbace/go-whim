# Phase 64 — the engine is called directly

*Formerly phase 136. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

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

Since step 5 (`doc/GRAPH-MIGRATION.md`, B1c) the phase runs on the graph: each of its five literals is the same statement found by its C-lisp form on `crefactor/graph`'s verbs -- four rebuilt from a template naming the callee, one cut -- its report the text version's, and the collection takes the table, the field and `regengine_T`; `whim-build-check` holds q064 to the bytes the text version made (which is in history, `4c8b3a7` and before).
